// Package repository contains explicit SQL persistence adapters.
package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/amh1k/mythborn/internal/contracts"
	"github.com/amh1k/mythborn/internal/domain"
)

var ErrNotFound = errors.New("resource not found")
var ErrMissingTemplates = errors.New("all four active agent templates are required")

type Store struct{ DB contracts.Database }

func New(db contracts.Database) *Store { return &Store{DB: db} }

func newID() (domain.ID, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	var dst [36]byte
	hex.Encode(dst[0:8], raw[0:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], raw[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], raw[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], raw[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:36], raw[10:16])
	return domain.ID(dst[:]), nil
}

func NewID() (domain.ID, error) { return newID() }

func (s *Store) EnsureAccount(ctx context.Context, id domain.ID) error {
	_, err := s.DB.Exec(ctx, `INSERT INTO accounts(id) VALUES ($1) ON CONFLICT (id) DO NOTHING`, string(id))
	return err
}

func scanGame(row contracts.Row) (domain.Game, error) {
	var g domain.Game
	var id, owner, role, status string
	var council, archived *time.Time
	err := row.Scan(&id, &owner, &g.Name, &role, &status, &g.ReviewPhotoDescription, &council, &g.CreatedAt, &g.UpdatedAt, &archived)
	if err != nil {
		return g, err
	}
	g.ID, g.OwnerAccountID, g.PlayerRole, g.Status = domain.ID(id), domain.ID(owner), domain.PlayerRole(role), domain.GameStatus(status)
	g.LastCouncilAt, g.ArchivedAt = council, archived
	return g, nil
}

const gameColumns = `id, owner_account_id, name, player_role, status, review_photo_description, last_council_at, created_at, updated_at, archived_at`

func (s *Store) GetGame(ctx context.Context, id domain.ID) (domain.Game, error) {
	g, err := scanGame(s.DB.QueryRow(ctx, `SELECT `+gameColumns+` FROM games WHERE id=$1`, string(id)))
	return g, normalizeNotFound(err)
}

func (s *Store) GetOwnedGame(ctx context.Context, id, accountID domain.ID) (domain.Game, error) {
	g, err := scanGame(s.DB.QueryRow(ctx, `SELECT `+gameColumns+` FROM games WHERE id=$1 AND owner_account_id=$2`, string(id), string(accountID)))
	return g, normalizeNotFound(err)
}

// ActivateGame makes a newly initialized game available only after the four
// founding agents have been persisted. It is safe for repeated start commands.
func (s *Store) ActivateGame(ctx context.Context, gameID domain.ID) error {
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status string
	if err = tx.QueryRow(ctx, `SELECT status FROM games WHERE id=$1 FOR UPDATE`, string(gameID)).Scan(&status); err != nil {
		return normalizeNotFound(err)
	}
	if status == "active" {
		return tx.Commit(ctx)
	}
	if status != "starting" {
		return ErrStateConflict
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM agents WHERE game_id=$1`, string(gameID)).Scan(&count); err != nil {
		return err
	}
	if count != 4 {
		return fmt.Errorf("game cannot activate without four founding agents")
	}
	if _, err = tx.Exec(ctx, `UPDATE games SET status='active',updated_at=now() WHERE id=$1`, string(gameID)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) ListGames(ctx context.Context, owner domain.ID) ([]domain.Game, error) {
	rows, err := s.DB.Query(ctx, `SELECT `+gameColumns+` FROM games WHERE owner_account_id=$1 ORDER BY created_at DESC`, string(owner))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Game, 0)
	for rows.Next() {
		g, scanErr := scanGame(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, g)
	}
	return items, rows.Err()
}

type startingTemplate struct {
	id                            domain.ID
	typeName, displayName, prompt string
	beliefs                       []startingBelief
}
type startingBelief struct{ claim, state string }

// CreateGame atomically snapshots all active templates, starts the four agents,
// copies each initial belief and creates the Temporal outbox command.
func (s *Store) CreateGame(ctx context.Context, owner domain.ID, name string, role domain.PlayerRole, idem string) (domain.Game, domain.ID, error) {
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return domain.Game{}, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `INSERT INTO accounts(id) VALUES ($1) ON CONFLICT (id) DO NOTHING`, string(owner)); err != nil {
		return domain.Game{}, "", err
	}
	var existingCommand string
	err = tx.QueryRow(ctx, `SELECT id::text FROM workflow_outbox WHERE account_id=$1 AND command_type='game.start' AND idempotency_key=$2`, string(owner), idem).Scan(&existingCommand)
	if err == nil {
		game, gameErr := scanGame(tx.QueryRow(ctx, `SELECT `+gameColumns+` FROM games WHERE id=(SELECT game_id FROM workflow_outbox WHERE id=$1)`, existingCommand))
		if gameErr != nil {
			return domain.Game{}, "", gameErr
		}
		if err = tx.Commit(ctx); err != nil {
			return domain.Game{}, "", err
		}
		return game, domain.ID(existingCommand), nil
	}
	if !errors.Is(err, contracts.ErrNoRows) {
		return domain.Game{}, "", err
	}
	rows, err := tx.Query(ctx, `SELECT id::text, agent_type, display_name, personality_prompt FROM agent_templates WHERE is_active ORDER BY agent_type FOR SHARE`)
	if err != nil {
		return domain.Game{}, "", err
	}
	templates := make([]startingTemplate, 0, 4)
	for rows.Next() {
		var t startingTemplate
		var id string
		if err = rows.Scan(&id, &t.typeName, &t.displayName, &t.prompt); err != nil {
			rows.Close()
			return domain.Game{}, "", err
		}
		t.id = domain.ID(id)
		templates = append(templates, t)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return domain.Game{}, "", err
	}
	rows.Close()
	if len(templates) != 4 {
		return domain.Game{}, "", ErrMissingTemplates
	}
	for i := range templates {
		beliefRows, qerr := tx.Query(ctx, `SELECT claim, initial_state FROM initial_belief_templates WHERE agent_template_id=$1 ORDER BY sort_order`, string(templates[i].id))
		if qerr != nil {
			return domain.Game{}, "", qerr
		}
		for beliefRows.Next() {
			var b startingBelief
			if qerr = beliefRows.Scan(&b.claim, &b.state); qerr != nil {
				beliefRows.Close()
				return domain.Game{}, "", qerr
			}
			templates[i].beliefs = append(templates[i].beliefs, b)
		}
		if qerr = beliefRows.Err(); qerr != nil {
			beliefRows.Close()
			return domain.Game{}, "", qerr
		}
		beliefRows.Close()
	}
	gameID, err := newID()
	if err != nil {
		return domain.Game{}, "", err
	}
	commandID, err := newID()
	if err != nil {
		return domain.Game{}, "", err
	}
	now := time.Now().UTC()
	if _, err = tx.Exec(ctx, `INSERT INTO games(id,owner_account_id,name,player_role,status) VALUES($1,$2,$3,$4,'starting')`, string(gameID), string(owner), name, string(role)); err != nil {
		return domain.Game{}, "", err
	}
	for _, t := range templates {
		agentID, idErr := newID()
		if idErr != nil {
			return domain.Game{}, "", idErr
		}
		if _, err = tx.Exec(ctx, `INSERT INTO agents(id,game_id,source_template_id,agent_type,display_name,personality_prompt) VALUES($1,$2,$3,$4,$5,$6)`, string(agentID), string(gameID), string(t.id), t.typeName, t.displayName, t.prompt); err != nil {
			return domain.Game{}, "", err
		}
		for _, b := range t.beliefs {
			beliefID, idErr := newID()
			if idErr != nil {
				return domain.Game{}, "", idErr
			}
			eventID, idErr := newID()
			if idErr != nil {
				return domain.Game{}, "", idErr
			}
			revisionID, idErr := newID()
			if idErr != nil {
				return domain.Game{}, "", idErr
			}
			if _, err = tx.Exec(ctx, `INSERT INTO beliefs(id,game_id,agent_id,claim,state,version) VALUES($1,$2,$3,$4,$5,1)`, string(beliefID), string(gameID), string(agentID), b.claim, b.state); err != nil {
				return domain.Game{}, "", err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO belief_revisions(id,game_id,belief_id,version,event_id,cause,change_type,new_claim,new_state,reason) VALUES($1,$2,$3,1,$4,'initial','formed',$5,$6,'Initial belief copied from the active agent template.')`, string(revisionID), string(gameID), string(beliefID), string(eventID), b.claim, b.state); err != nil {
				return domain.Game{}, "", err
			}
		}
	}
	payload, _ := json.Marshal(map[string]string{"game_id": string(gameID)})
	if _, err = tx.Exec(ctx, `INSERT INTO workflow_outbox(id,account_id,game_id,command_type,idempotency_key,payload) VALUES($1,$2,$3,'game.start',$4,$5::jsonb)`, string(commandID), string(owner), string(gameID), idem, string(payload)); err != nil {
		return domain.Game{}, "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Game{}, "", err
	}
	return domain.Game{ID: gameID, OwnerAccountID: owner, Name: name, PlayerRole: role, Status: domain.GameStatusStarting, CreatedAt: now, UpdatedAt: now}, commandID, nil
}

func (s *Store) SetReviewPhotoDescription(ctx context.Context, gameID domain.ID, enabled bool) error {
	n, err := s.DB.Exec(ctx, `UPDATE games SET review_photo_description=$2, updated_at=now() WHERE id=$1 AND status IN ('starting','active')`, string(gameID), enabled)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) RequestGameEnd(ctx context.Context, game domain.Game, accountID domain.ID, idem string) (domain.ID, error) {
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var existing string
	err = tx.QueryRow(ctx, `SELECT id::text FROM workflow_outbox WHERE account_id=$1 AND command_type='game.end' AND idempotency_key=$2`, string(accountID), idem).Scan(&existing)
	if err == nil {
		if err = tx.Commit(ctx); err != nil {
			return "", err
		}
		return domain.ID(existing), nil
	}
	if !errors.Is(err, contracts.ErrNoRows) {
		return "", err
	}
	var status string
	if err = tx.QueryRow(ctx, `SELECT status FROM games WHERE id=$1 FOR UPDATE`, string(game.ID)).Scan(&status); err != nil {
		return "", err
	}
	if status != "active" {
		return "", fmt.Errorf("game cannot be ended from status %s", status)
	}
	id, err := newID()
	if err != nil {
		return "", err
	}
	payload, _ := json.Marshal(map[string]string{"game_id": string(game.ID)})
	if _, err = tx.Exec(ctx, `UPDATE games SET status='ending_requested',updated_at=now() WHERE id=$1`, string(game.ID)); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO workflow_outbox(id,account_id,game_id,command_type,idempotency_key,payload) VALUES($1,$2,$3,'game.end',$4,$5::jsonb) ON CONFLICT(account_id,command_type,idempotency_key) DO NOTHING`, string(id), string(accountID), string(game.ID), idem, string(payload)); err != nil {
		return "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) MarkGameDeleting(ctx context.Context, id, accountID domain.ID) (domain.ID, error) {
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	key := "delete-game-" + string(id)
	var existing string
	err = tx.QueryRow(ctx, `SELECT id::text FROM workflow_outbox WHERE account_id=$1 AND command_type='game.delete' AND idempotency_key=$2`, string(accountID), key).Scan(&existing)
	if err == nil {
		if err = tx.Commit(ctx); err != nil {
			return "", err
		}
		return domain.ID(existing), nil
	}
	if !errors.Is(err, contracts.ErrNoRows) {
		return "", err
	}
	n, err := tx.Exec(ctx, `UPDATE games SET status='deleting',updated_at=now() WHERE id=$1 AND status <> 'deleting'`, string(id))
	if err != nil {
		return "", err
	}
	if n == 0 {
		return "", ErrNotFound
	}
	commandID, err := newID()
	if err != nil {
		return "", err
	}
	payload, _ := json.Marshal(map[string]string{"game_id": string(id)})
	if _, err = tx.Exec(ctx, `INSERT INTO workflow_outbox(id,account_id,game_id,command_type,idempotency_key,payload) VALUES($1,$2,$3,'game.delete',$4,$5::jsonb)`, string(commandID), string(accountID), string(id), key, string(payload)); err != nil {
		return "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return commandID, nil
}
