package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/amh1k/mythborn/internal/contracts"
	"github.com/amh1k/mythborn/internal/domain"
)

// EnsureClosingRound turns an end-game command into the single closing round.
// It is safe to retry after a worker restart; the existing round/attachment is
// returned when already created.
func (s *Store) EnsureClosingRound(ctx context.Context, gameID, commandID domain.ID) (domain.ID, error) {
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var gameStatus string
	if err = tx.QueryRow(ctx, `SELECT status FROM games WHERE id=$1 FOR UPDATE`, string(gameID)).Scan(&gameStatus); err != nil {
		return "", normalizeNotFound(err)
	}
	if gameStatus != "ending_requested" && gameStatus != "ending" && gameStatus != "archived" {
		return "", ErrStateConflict
	}
	var commandType, commandStatus string
	var commandGame *string
	var currentRound *string
	if err = tx.QueryRow(ctx, `SELECT command_type,status,game_id::text,round_id::text FROM workflow_outbox WHERE id=$1 FOR UPDATE`, string(commandID)).Scan(&commandType, &commandStatus, &commandGame, &currentRound); err != nil {
		return "", normalizeNotFound(err)
	}
	if commandType != "game.end" || commandGame == nil || *commandGame != string(gameID) {
		return "", ErrStateConflict
	}
	if currentRound != nil {
		if err = tx.Commit(ctx); err != nil {
			return "", err
		}
		return domain.ID(*currentRound), nil
	}
	var roundID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM rounds WHERE game_id=$1 AND kind='closing'`, string(gameID)).Scan(&roundID)
	if errors.Is(err, contracts.ErrNoRows) {
		var sequence int64
		if err = tx.QueryRow(ctx, `SELECT COALESCE(max(sequence_number),0)+1 FROM rounds WHERE game_id=$1`, string(gameID)).Scan(&sequence); err != nil {
			return "", err
		}
		newRound, genErr := newID()
		if genErr != nil {
			return "", genErr
		}
		roundID = string(newRound)
		if _, err = tx.Exec(ctx, `INSERT INTO rounds(id,game_id,sequence_number,kind,status,stage,idempotency_key,started_at) VALUES($1,$2,$3,'closing','running','writing',$4,now())`, roundID, string(gameID), sequence, "closing-"+string(gameID)); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	} else {
		if _, err = tx.Exec(ctx, `UPDATE rounds SET status='running',stage='writing',error_code=NULL,started_at=COALESCE(started_at,now()),completed_at=NULL,updated_at=now() WHERE game_id=$1 AND id=$2 AND status IN ('failed','needs_attention','queued')`, string(gameID), roundID); err != nil {
			return "", err
		}
	}
	if gameStatus != "ending" {
		if _, err = tx.Exec(ctx, `UPDATE games SET status='ending',updated_at=now() WHERE id=$1`, string(gameID)); err != nil {
			return "", err
		}
	}
	payload, _ := json.Marshal(map[string]string{"game_id": string(gameID), "round_id": roundID})
	if _, err = tx.Exec(ctx, `UPDATE workflow_outbox SET round_id=$2,payload=$3::jsonb WHERE id=$1`, string(commandID), roundID, string(payload)); err != nil {
		return "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return domain.ID(roundID), nil
}

func (s *Store) AccountDeletionReady(ctx context.Context, accountID, commandID domain.ID) (bool, error) {
	var accountStatus string
	if err := s.DB.QueryRow(ctx, `SELECT status FROM accounts WHERE id=$1`, string(accountID)).Scan(&accountStatus); err != nil {
		return false, normalizeNotFound(err)
	}
	if accountStatus != "deleting" {
		return false, ErrStateConflict
	}
	var commandType, commandAccount string
	if err := s.DB.QueryRow(ctx, `SELECT command_type,account_id::text FROM workflow_outbox WHERE id=$1`, string(commandID)).Scan(&commandType, &commandAccount); err != nil {
		return false, normalizeNotFound(err)
	}
	if commandType != "account.delete" || commandAccount != string(accountID) {
		return false, ErrStateConflict
	}
	var jobID string
	if err := s.DB.QueryRow(ctx, `SELECT id::text FROM account_deletion_jobs WHERE external_account_id=$1 AND status IN ('pending','running') ORDER BY created_at DESC LIMIT 1`, string(accountID)).Scan(&jobID); err != nil {
		return false, normalizeNotFound(err)
	}
	var hasGames bool
	if err := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM games WHERE owner_account_id=$1)`, string(accountID)).Scan(&hasGames); err != nil {
		return false, err
	}
	return !hasGames, nil
}

// CompleteAccountDeletion is called only after Supabase Auth deletion succeeds.
// It preserves the independent job record, removes account-owned outbox markers,
// and then deletes the application account in one transaction.
func (s *Store) CompleteAccountDeletion(ctx context.Context, accountID, commandID domain.ID) error {
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status string
	if err = tx.QueryRow(ctx, `SELECT status FROM accounts WHERE id=$1 FOR UPDATE`, string(accountID)).Scan(&status); err != nil {
		return normalizeNotFound(err)
	}
	if status != "deleting" {
		return ErrStateConflict
	}
	var commandType, commandAccount string
	if err = tx.QueryRow(ctx, `SELECT command_type,account_id::text FROM workflow_outbox WHERE id=$1 FOR UPDATE`, string(commandID)).Scan(&commandType, &commandAccount); err != nil {
		return normalizeNotFound(err)
	}
	if commandType != "account.delete" || commandAccount != string(accountID) {
		return ErrStateConflict
	}
	var hasGames bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM games WHERE owner_account_id=$1)`, string(accountID)).Scan(&hasGames); err != nil {
		return err
	}
	if hasGames {
		return ErrStateConflict
	}
	var jobID string
	if err = tx.QueryRow(ctx, `SELECT id::text FROM account_deletion_jobs WHERE external_account_id=$1 AND status IN ('pending','running') ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, string(accountID)).Scan(&jobID); err != nil {
		return normalizeNotFound(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE account_deletion_jobs SET status='complete',cleanup_stage='complete',completed_at=now(),updated_at=now(),error_code=NULL WHERE id=$1`, jobID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM workflow_outbox WHERE account_id=$1`, string(accountID)); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM accounts WHERE id=$1`, string(accountID)); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit account deletion: %w", err)
	}
	return nil
}
