package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/amh1k/mythborn/internal/contracts"
	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/workflow"
)

func (s *Store) GetCommand(ctx context.Context, id, accountID domain.ID, admin bool) (domain.WorkflowCommand, error) {
	_, err := s.DB.Exec(ctx, `UPDATE workflow_outbox SET result_payload=NULL,result_expires_at=NULL WHERE id=$1 AND result_expires_at<=now()`, string(id))
	if err != nil {
		return domain.WorkflowCommand{}, err
	}
	query := `SELECT id::text,account_id::text,game_id::text,round_id::text,command_type,idempotency_key,payload,result_payload,result_expires_at,status,attempt_count,created_at FROM workflow_outbox WHERE id=$1`
	args := []any{string(id)}
	if !admin {
		query += ` AND account_id=$2`
		args = append(args, string(accountID))
	}
	var c domain.WorkflowCommand
	var cid, aid, typ, status string
	var gameID, roundID *string
	var result []byte
	var expires *time.Time
	err = s.DB.QueryRow(ctx, query, args...).Scan(&cid, &aid, &gameID, &roundID, &typ, &c.IdempotencyKey, &c.Payload, &result, &expires, &status, &c.AttemptCount, &c.CreatedAt)
	if err != nil {
		return c, normalizeNotFound(err)
	}
	c.ID, c.AccountID, c.Type, c.Status = domain.ID(cid), domain.ID(aid), typ, domain.CommandStatus(status)
	if gameID != nil {
		x := domain.ID(*gameID)
		c.GameID = &x
	}
	if roundID != nil {
		x := domain.ID(*roundID)
		c.RoundID = &x
	}
	c.ResultPayload = result
	c.ResultExpiresAt = expires
	if c.ResultExpiresAt != nil && c.ResultExpiresAt.Before(time.Now()) {
		c.ResultPayload = nil
		c.ResultExpiresAt = nil
	}
	return c, nil
}

func (s *Store) CreateMessengerCommand(ctx context.Context, accountID, gameID, roundID, agentID domain.ID, message, key string, expectedVersion int64) (domain.ID, error) {
	if len(message) == 0 || len(message) > 4000 || len(key) < 8 || len(key) > 180 {
		return "", fmt.Errorf("invalid messenger input")
	}
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var owner, role, status string
	if err = tx.QueryRow(ctx, `SELECT owner_account_id::text,player_role,status FROM games WHERE id=$1 FOR UPDATE`, string(gameID)).Scan(&owner, &role, &status); err != nil {
		return "", normalizeNotFound(err)
	}
	if domain.ID(owner) != accountID || role != "messenger" || status != "active" {
		return "", ErrNotFound
	}
	var old string
	err = tx.QueryRow(ctx, `SELECT id::text FROM workflow_outbox WHERE account_id=$1 AND command_type='messenger.exchange' AND idempotency_key=$2`, string(accountID), key).Scan(&old)
	if err == nil {
		if err = tx.Commit(ctx); err != nil {
			return "", err
		}
		return domain.ID(old), nil
	}
	if !errors.Is(err, contracts.ErrNoRows) {
		return "", err
	}
	var roundKind, roundStatus string
	if err = tx.QueryRow(ctx, `SELECT kind,status FROM rounds WHERE game_id=$1 AND id=$2`, string(gameID), string(roundID)).Scan(&roundKind, &roundStatus); err != nil {
		return "", normalizeNotFound(err)
	}
	if roundKind != "discovery" || roundStatus != "complete" {
		return "", ErrStateConflict
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM agents WHERE game_id=$1 AND id=$2)`, string(gameID), string(agentID)).Scan(&exists); err != nil {
		return "", err
	}
	if !exists {
		return "", ErrNotFound
	}
	var actual int64
	err = tx.QueryRow(ctx, `SELECT version FROM conversation_summaries WHERE game_id=$1 AND round_id=$2 AND agent_id=$3`, string(gameID), string(roundID), string(agentID)).Scan(&actual)
	if errors.Is(err, contracts.ErrNoRows) {
		actual = 0
	} else if err != nil {
		return "", err
	}
	if actual != expectedVersion {
		return "", ErrStateConflict
	}
	id, err := newID()
	if err != nil {
		return "", err
	}
	payload, _ := json.Marshal(workflow.CommandPayload{AgentID: agentID, Message: message, ExpectedSummaryVersion: expectedVersion})
	if _, err = tx.Exec(ctx, `INSERT INTO workflow_outbox(id,account_id,game_id,round_id,command_type,idempotency_key,payload) VALUES($1,$2,$3,$4,'messenger.exchange',$5,$6::jsonb)`, string(id), string(accountID), string(gameID), string(roundID), key, string(payload)); err != nil {
		return "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) GetObservation(ctx context.Context, gameID, observationID domain.ID) (workflow.ObservationContext, error) {
	var o workflow.ObservationContext
	var id, gid string
	var description, correction, statement *string
	err := s.DB.QueryRow(ctx, `SELECT id::text,game_id::text,photo_object_path,mime_type,visual_description,player_correction,player_statement,description_status FROM observations WHERE game_id=$1 AND id=$2`, string(gameID), string(observationID)).Scan(&id, &gid, &o.PhotoObjectPath, &o.MIMEType, &description, &correction, &statement, &o.DescriptionStatus)
	if err != nil {
		return o, normalizeNotFound(err)
	}
	o.ID, o.GameID = domain.ID(id), domain.ID(gid)
	if description != nil {
		o.VisualDescription = *description
	}
	if correction != nil {
		o.PlayerCorrection = *correction
	}
	if statement != nil {
		o.PlayerStatement = *statement
	}
	return o, nil
}

func (s *Store) QueueGameCommand(ctx context.Context, accountID, gameID, roundID domain.ID, commandType, key string, payload any) (domain.ID, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	if len(data) > 32768 {
		return "", fmt.Errorf("command payload exceeds limit")
	}
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var existing string
	err = tx.QueryRow(ctx, `SELECT id::text FROM workflow_outbox WHERE account_id=$1 AND command_type=$2 AND idempotency_key=$3`, string(accountID), commandType, key).Scan(&existing)
	if err == nil {
		if err = tx.Commit(ctx); err != nil {
			return "", err
		}
		return domain.ID(existing), nil
	}
	if !errors.Is(err, contracts.ErrNoRows) {
		return "", err
	}
	id, err := newID()
	if err != nil {
		return "", err
	}
	var round any
	if roundID != "" {
		round = string(roundID)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO workflow_outbox(id,account_id,game_id,round_id,command_type,idempotency_key,payload) VALUES($1,$2,$3,$4,$5,$6,$7::jsonb)`, string(id), string(accountID), string(gameID), round, commandType, key, string(data)); err != nil {
		return "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) SubmitDescriptionDecision(ctx context.Context, accountID, gameID, roundID domain.ID, decision, correction, clarityChoice, key string) (domain.ID, error) {
	if len(key) < 8 || len(key) > 180 {
		return "", fmt.Errorf("invalid idempotency key")
	}
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
	var prior string
	err = tx.QueryRow(ctx, `SELECT id::text FROM workflow_outbox WHERE account_id=$1 AND command_type='description.decision' AND idempotency_key=$2`, string(accountID), key).Scan(&prior)
	if err == nil {
		if err = tx.Commit(ctx); err != nil {
			return "", err
		}
		return domain.ID(prior), nil
	}
	if !errors.Is(err, contracts.ErrNoRows) {
		return "", err
	}
	if gameStatus != "active" {
		return "", ErrStateConflict
	}
	var obsID, roundStatus, stage string
	if err = tx.QueryRow(ctx, `SELECT observation_id::text,status,stage FROM rounds WHERE game_id=$1 AND id=$2 FOR UPDATE`, string(gameID), string(roundID)).Scan(&obsID, &roundStatus, &stage); err != nil {
		return "", normalizeNotFound(err)
	}
	if roundStatus != "running" || (stage != "awaiting_review" && stage != "awaiting_clarity_choice") {
		return "", ErrStateConflict
	}
	if decision == "choose_clarity" {
		if clarityChoice != "continue_uncertain" && clarityChoice != "try_another_photo" {
			return "", fmt.Errorf("invalid clarity choice")
		}
		decision = clarityChoice
	} else if decision != "accept" && decision != "correct" {
		return "", fmt.Errorf("invalid description decision")
	}
	if decision == "correct" && strings.TrimSpace(correction) == "" {
		return "", fmt.Errorf("correction is required")
	}
	if (decision == "continue_uncertain" || decision == "try_another_photo") && stage != "awaiting_clarity_choice" {
		return "", ErrStateConflict
	}
	if decision == "accept" || decision == "correct" || decision == "continue_uncertain" {
		if _, err = tx.Exec(ctx, `UPDATE observations SET description_status='accepted',accepted_at=COALESCE(accepted_at,now()),player_correction=CASE WHEN $3<>'' THEN $3 ELSE player_correction END WHERE game_id=$1 AND id=$2`, string(gameID), obsID, strings.TrimSpace(correction)); err != nil {
			return "", err
		}
	}
	if decision == "try_another_photo" {
		if _, err = tx.Exec(ctx, `UPDATE rounds SET status='abandoned',updated_at=now() WHERE game_id=$1 AND id=$2`, string(gameID), string(roundID)); err != nil {
			return "", err
		}
	} else {
		if _, err = tx.Exec(ctx, `UPDATE rounds SET stage='reacting',updated_at=now() WHERE game_id=$1 AND id=$2`, string(gameID), string(roundID)); err != nil {
			return "", err
		}
	}
	id, err := newID()
	if err != nil {
		return "", err
	}
	payload, _ := json.Marshal(workflow.CommandPayload{DescriptionDecision: decision, Correction: strings.TrimSpace(correction)})
	if _, err = tx.Exec(ctx, `INSERT INTO workflow_outbox(id,account_id,game_id,round_id,command_type,idempotency_key,payload) VALUES($1,$2,$3,$4,'description.decision',$5,$6::jsonb)`, string(id), string(accountID), string(gameID), string(roundID), key, string(payload)); err != nil {
		return "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) RetryRound(ctx context.Context, accountID, gameID, roundID domain.ID, key string) (domain.ID, error) {
	if len(key) < 8 || len(key) > 180 {
		return "", fmt.Errorf("invalid idempotency key")
	}
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
	var prior string
	err = tx.QueryRow(ctx, `SELECT id::text FROM workflow_outbox WHERE account_id=$1 AND command_type='round.retry' AND idempotency_key=$2`, string(accountID), key).Scan(&prior)
	if err == nil {
		if err = tx.Commit(ctx); err != nil {
			return "", err
		}
		return domain.ID(prior), nil
	}
	if !errors.Is(err, contracts.ErrNoRows) {
		return "", err
	}
	if gameStatus != "active" {
		return "", ErrStateConflict
	}
	var roundStatus string
	if err = tx.QueryRow(ctx, `SELECT status FROM rounds WHERE game_id=$1 AND id=$2 FOR UPDATE`, string(gameID), string(roundID)).Scan(&roundStatus); err != nil {
		return "", normalizeNotFound(err)
	}
	if roundStatus != "failed" && roundStatus != "needs_attention" {
		return "", ErrStateConflict
	}
	if _, err = tx.Exec(ctx, `UPDATE rounds SET status='queued',error_code=NULL,updated_at=now() WHERE game_id=$1 AND id=$2`, string(gameID), string(roundID)); err != nil {
		return "", err
	}
	id, err := newID()
	if err != nil {
		return "", err
	}
	payload, _ := json.Marshal(map[string]string{"round_id": string(roundID)})
	if _, err = tx.Exec(ctx, `INSERT INTO workflow_outbox(id,account_id,game_id,round_id,command_type,idempotency_key,payload) VALUES($1,$2,$3,$4,'round.retry',$5,$6::jsonb)`, string(id), string(accountID), string(gameID), string(roundID), key, string(payload)); err != nil {
		return "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}
