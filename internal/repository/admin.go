package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/amh1k/mythborn/internal/contracts"
	"github.com/amh1k/mythborn/internal/domain"
)

type BeliefTemplate struct {
	ID        domain.ID          `json:"id"`
	Claim     string             `json:"claim"`
	State     domain.BeliefState `json:"initial_state"`
	SortOrder int                `json:"sort_order"`
}
type AgentTemplate struct {
	ID                domain.ID        `json:"id"`
	AgentType         domain.AgentType `json:"agent_type"`
	Version           int              `json:"version"`
	DisplayName       string           `json:"display_name"`
	PersonalityPrompt string           `json:"personality_prompt"`
	Active            bool             `json:"is_active"`
	Beliefs           []BeliefTemplate `json:"beliefs"`
}
type AdminAccount struct {
	ID         domain.ID         `json:"id"`
	SystemRole domain.SystemRole `json:"system_role"`
	Status     string            `json:"status"`
	CreatedAt  string            `json:"created_at"`
}

func (s *Store) ListAllGames(ctx context.Context) ([]domain.Game, error) {
	rows, err := s.DB.Query(ctx, `SELECT `+gameColumns+` FROM games ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Game, 0)
	for rows.Next() {
		g, e := scanGame(rows)
		if e != nil {
			return nil, e
		}
		items = append(items, g)
	}
	return items, rows.Err()
}
func (s *Store) ListAccounts(ctx context.Context) ([]AdminAccount, error) {
	rows, err := s.DB.Query(ctx, `SELECT id::text,system_role,status,created_at::text FROM accounts ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]AdminAccount, 0)
	for rows.Next() {
		var a AdminAccount
		var id, role string
		if err = rows.Scan(&id, &role, &a.Status, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.ID, a.SystemRole = domain.ID(id), domain.SystemRole(role)
		items = append(items, a)
	}
	return items, rows.Err()
}
func (s *Store) SetAccountRole(ctx context.Context, id domain.ID, role domain.SystemRole) error {
	if role != domain.SystemRoleUser && role != domain.SystemRoleAdmin {
		return fmt.Errorf("invalid role")
	}
	n, err := s.DB.Exec(ctx, `UPDATE accounts SET system_role=$2,updated_at=now() WHERE id=$1 AND status='active'`, string(id), string(role))
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
func (s *Store) ListAgentTemplates(ctx context.Context) ([]AgentTemplate, error) {
	rows, err := s.DB.Query(ctx, `SELECT id::text,agent_type,version,display_name,personality_prompt,is_active FROM agent_templates ORDER BY agent_type,version DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]AgentTemplate, 0)
	for rows.Next() {
		var t AgentTemplate
		var id, typ string
		if err = rows.Scan(&id, &typ, &t.Version, &t.DisplayName, &t.PersonalityPrompt, &t.Active); err != nil {
			return nil, err
		}
		t.ID, t.AgentType = domain.ID(id), domain.AgentType(typ)
		items = append(items, t)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	for i := range items {
		beliefs, e := s.DB.Query(ctx, `SELECT id::text,claim,initial_state,sort_order FROM initial_belief_templates WHERE agent_template_id=$1 ORDER BY sort_order`, string(items[i].ID))
		if e != nil {
			return nil, e
		}
		for beliefs.Next() {
			var b BeliefTemplate
			var id, state string
			if e = beliefs.Scan(&id, &b.Claim, &state, &b.SortOrder); e != nil {
				beliefs.Close()
				return nil, e
			}
			b.ID, b.State = domain.ID(id), domain.BeliefState(state)
			items[i].Beliefs = append(items[i].Beliefs, b)
		}
		e = beliefs.Err()
		beliefs.Close()
		if e != nil {
			return nil, e
		}
	}
	return items, nil
}

func (s *Store) CreateAgentTemplate(ctx context.Context, adminID domain.ID, t AgentTemplate) (AgentTemplate, error) {
	if t.AgentType != domain.AgentTypePriest && t.AgentType != domain.AgentTypeScientist && t.AgentType != domain.AgentTypeSoldier && t.AgentType != domain.AgentTypeHistorian {
		return t, fmt.Errorf("invalid agent type")
	}
	if t.DisplayName == "" || len(t.DisplayName) > 120 || len(t.PersonalityPrompt) == 0 || len(t.PersonalityPrompt) > 12000 || len(t.Beliefs) > 100 {
		return t, fmt.Errorf("invalid template")
	}
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return t, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var version int
	if err = tx.QueryRow(ctx, `SELECT COALESCE(max(version),0)+1 FROM agent_templates WHERE agent_type=$1`, string(t.AgentType)).Scan(&version); err != nil {
		return t, err
	}
	if t.Active {
		if _, err = tx.Exec(ctx, `UPDATE agent_templates SET is_active=false,updated_at=now() WHERE agent_type=$1 AND is_active`, string(t.AgentType)); err != nil {
			return t, err
		}
	}
	id, err := newID()
	if err != nil {
		return t, err
	}
	t.ID, t.Version = id, version
	if _, err = tx.Exec(ctx, `INSERT INTO agent_templates(id,agent_type,version,display_name,personality_prompt,is_active,updated_by_account_id) VALUES($1,$2,$3,$4,$5,$6,$7)`, string(id), string(t.AgentType), version, t.DisplayName, t.PersonalityPrompt, t.Active, string(adminID)); err != nil {
		return t, err
	}
	for i, b := range t.Beliefs {
		if b.Claim == "" || len(b.Claim) > 2000 {
			return t, fmt.Errorf("invalid belief template")
		}
		if b.State != "forming" && b.State != "held" && b.State != "questioned" && b.State != "abandoned" {
			return t, fmt.Errorf("invalid belief state")
		}
		beliefID, e := newID()
		if e != nil {
			return t, e
		}
		if _, err = tx.Exec(ctx, `INSERT INTO initial_belief_templates(id,agent_template_id,claim,initial_state,sort_order) VALUES($1,$2,$3,$4,$5)`, string(beliefID), string(id), b.Claim, string(b.State), i); err != nil {
			return t, err
		}
		t.Beliefs[i].ID = beliefID
		t.Beliefs[i].SortOrder = i
	}
	if err = tx.Commit(ctx); err != nil {
		return t, err
	}
	return t, nil
}

func (s *Store) ActivateAgentTemplate(ctx context.Context, id domain.ID) error {
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var typ string
	if err = tx.QueryRow(ctx, `SELECT agent_type FROM agent_templates WHERE id=$1 FOR UPDATE`, string(id)).Scan(&typ); err != nil {
		if errors.Is(err, contracts.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE agent_templates SET is_active=false,updated_at=now() WHERE agent_type=$1 AND is_active`, typ); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE agent_templates SET is_active=true,updated_at=now() WHERE id=$1`, string(id)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) RequestAccountDeletion(ctx context.Context, accountID domain.ID) (domain.ID, domain.ID, error) {
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status string
	if err = tx.QueryRow(ctx, `SELECT status FROM accounts WHERE id=$1 FOR UPDATE`, string(accountID)).Scan(&status); err != nil {
		return "", "", normalizeNotFound(err)
	}
	var existingJob string
	err = tx.QueryRow(ctx, `SELECT id::text FROM account_deletion_jobs WHERE external_account_id=$1 AND status IN ('pending','running','failed')`, string(accountID)).Scan(&existingJob)
	if err == nil {
		var command string
		if err = tx.QueryRow(ctx, `SELECT id::text FROM workflow_outbox WHERE account_id=$1 AND command_type='account.delete' AND idempotency_key=$2`, string(accountID), "delete-account-"+string(accountID)).Scan(&command); err != nil {
			return "", "", err
		}
		if err = tx.Commit(ctx); err != nil {
			return "", "", err
		}
		return domain.ID(existingJob), domain.ID(command), nil
	}
	if !errors.Is(err, contracts.ErrNoRows) {
		return "", "", err
	}
	jobID, err := newID()
	if err != nil {
		return "", "", err
	}
	commandID, err := newID()
	if err != nil {
		return "", "", err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO account_deletion_jobs(id,external_account_id,status,cleanup_stage) VALUES($1,$2,'pending','requested')`, string(jobID), string(accountID)); err != nil {
		return "", "", err
	}
	if _, err = tx.Exec(ctx, `UPDATE accounts SET status='deleting',updated_at=now() WHERE id=$1`, string(accountID)); err != nil {
		return "", "", err
	}
	gameRows, err := tx.Query(ctx, `SELECT id::text FROM games WHERE owner_account_id=$1 FOR UPDATE`, string(accountID))
	if err != nil {
		return "", "", err
	}
	gameIDs := make([]string, 0)
	for gameRows.Next() {
		var id string
		if err = gameRows.Scan(&id); err != nil {
			gameRows.Close()
			return "", "", err
		}
		gameIDs = append(gameIDs, id)
	}
	if err = gameRows.Err(); err != nil {
		gameRows.Close()
		return "", "", err
	}
	gameRows.Close()
	for _, gameID := range gameIDs {
		if _, err = tx.Exec(ctx, `UPDATE games SET status='deleting',updated_at=now() WHERE id=$1`, gameID); err != nil {
			return "", "", err
		}
		id, e := newID()
		if e != nil {
			return "", "", e
		}
		payload, _ := json.Marshal(map[string]string{"game_id": gameID, "deletion_job_id": string(jobID)})
		if _, err = tx.Exec(ctx, `INSERT INTO workflow_outbox(id,account_id,game_id,command_type,idempotency_key,payload) VALUES($1,$2,$3,'game.delete',$4,$5::jsonb)`, string(id), string(accountID), gameID, "delete-game-"+gameID, string(payload)); err != nil {
			return "", "", err
		}
	}
	payload, _ := json.Marshal(map[string]string{"account_id": string(accountID), "deletion_job_id": string(jobID)})
	if _, err = tx.Exec(ctx, `INSERT INTO workflow_outbox(id,account_id,command_type,idempotency_key,payload) VALUES($1,$2,'account.delete',$3,$4::jsonb)`, string(commandID), string(accountID), "delete-account-"+string(accountID), string(payload)); err != nil {
		return "", "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", "", err
	}
	return jobID, commandID, nil
}
