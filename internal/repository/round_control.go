package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/amh1k/mythborn/internal/contracts"
	"github.com/amh1k/mythborn/internal/domain"
)

func (s *Store) PauseRound(ctx context.Context, gameID, roundID domain.ID, code string) error {
	_, err := s.DB.Exec(ctx, `UPDATE rounds SET status='needs_attention',error_code=$3,updated_at=now() WHERE game_id=$1 AND id=$2 AND status NOT IN ('complete','abandoned')`, string(gameID), string(roundID), code)
	return err
}

// RequestRoundAbandon locks the game before the round, matching other commands.
// Only the coordinator marks the round abandoned after it has canceled work.
func (s *Store) RequestRoundAbandon(ctx context.Context, accountID, gameID, roundID domain.ID, key string) (domain.ID, error) {
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var owner, gameStatus string
	if err = tx.QueryRow(ctx, `SELECT owner_account_id::text,status FROM games WHERE id=$1 FOR UPDATE`, string(gameID)).Scan(&owner, &gameStatus); err != nil {
		return "", normalizeNotFound(err)
	}
	if owner != string(accountID) {
		return "", ErrNotFound
	}
	var previous string
	err = tx.QueryRow(ctx, `SELECT id::text FROM workflow_outbox WHERE account_id=$1 AND command_type='round.abandon' AND idempotency_key=$2`, string(accountID), key).Scan(&previous)
	if err == nil {
		return domain.ID(previous), tx.Commit(ctx)
	}
	if !errors.Is(err, contracts.ErrNoRows) {
		return "", err
	}
	if gameStatus != "active" {
		return "", ErrStateConflict
	}
	var status, kind string
	if err = tx.QueryRow(ctx, `SELECT status,kind FROM rounds WHERE game_id=$1 AND id=$2 FOR UPDATE`, string(gameID), string(roundID)).Scan(&status, &kind); err != nil {
		return "", normalizeNotFound(err)
	}
	if status == "complete" || status == "abandoned" || kind == "closing" {
		return "", ErrStateConflict
	}
	id, err := newID()
	if err != nil {
		return "", err
	}
	payload, _ := json.Marshal(map[string]string{"round_id": string(roundID)})
	if _, err = tx.Exec(ctx, `INSERT INTO workflow_outbox(id,account_id,game_id,round_id,command_type,idempotency_key,payload) VALUES($1,$2,$3,$4,'round.abandon',$5,$6::jsonb)`, string(id), string(accountID), string(gameID), string(roundID), key, string(payload)); err != nil {
		return "", err
	}
	return id, tx.Commit(ctx)
}

func (s *Store) AbandonRound(ctx context.Context, gameID, roundID, commandID domain.ID) error {
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var gameStatus string
	if err = tx.QueryRow(ctx, `SELECT status FROM games WHERE id=$1 FOR UPDATE`, string(gameID)).Scan(&gameStatus); err != nil {
		return normalizeNotFound(err)
	}
	var status string
	if err = tx.QueryRow(ctx, `SELECT status FROM rounds WHERE game_id=$1 AND id=$2 FOR UPDATE`, string(gameID), string(roundID)).Scan(&status); err != nil {
		return normalizeNotFound(err)
	}
	if status != "complete" {
		if _, err = tx.Exec(ctx, `UPDATE rounds SET status='abandoned',error_code=NULL,completed_at=now(),updated_at=now() WHERE game_id=$1 AND id=$2 AND status <> 'abandoned'`, string(gameID), string(roundID)); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE workflow_outbox SET status='processed',processed_at=COALESCE(processed_at,now()),payload='{}'::jsonb,lease_until=NULL,error_code=NULL WHERE game_id=$1 AND round_id=$2 AND command_type IN ('round.process','round.retry','round.abandon','description.decision','council.run') AND status <> 'processed'`, string(gameID), string(roundID)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) AbandonOpenRounds(ctx context.Context, gameID domain.ID) error {
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status string
	if err = tx.QueryRow(ctx, `SELECT status FROM games WHERE id=$1 FOR UPDATE`, string(gameID)).Scan(&status); err != nil {
		return normalizeNotFound(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE rounds SET status='abandoned',error_code=NULL,completed_at=now(),updated_at=now() WHERE game_id=$1 AND status IN ('queued','running','needs_attention','failed') AND kind <> 'closing'`, string(gameID)); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE workflow_outbox SET status='processed',processed_at=COALESCE(processed_at,now()),payload='{}'::jsonb,lease_until=NULL,error_code=NULL WHERE game_id=$1 AND round_id IN (SELECT id FROM rounds WHERE game_id=$1 AND status='abandoned') AND command_type IN ('round.process','round.retry','round.abandon','description.decision','council.run') AND status <> 'processed'`, string(gameID)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
