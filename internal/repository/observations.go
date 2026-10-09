package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/amh1k/mythborn/internal/contracts"
	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/workflow"
)

type DiscoveryInput struct {
	ObservationID   domain.ID
	RoundID         domain.ID
	CommandID       domain.ID
	GameID          domain.ID
	AccountID       domain.ID
	PhotoPath       string
	SourceSHA256    string
	MIMEType        string
	ByteSize        int64
	PlayerStatement string
	IdempotencyKey  string
	AllowReuse      bool
}

var ErrDuplicateObservation = errors.New("duplicate observation")

// CreateDiscovery stores the observation, queued round, and process command in
// one transaction. Photo bytes are written by the application before this call.
func (s *Store) CreateDiscovery(ctx context.Context, in DiscoveryInput) (domain.ID, domain.ID, domain.ID, error) {
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return "", "", "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var owner, gameStatus string
	if err = tx.QueryRow(ctx, `SELECT owner_account_id::text,status FROM games WHERE id=$1 FOR UPDATE`, string(in.GameID)).Scan(&owner, &gameStatus); err != nil {
		return "", "", "", normalizeNotFound(err)
	}
	if domain.ID(owner) != in.AccountID {
		return "", "", "", ErrNotFound
	}
	if gameStatus != "active" {
		return "", "", "", ErrStateConflict
	}
	var previousObservation, previousRound, previousCommand string
	err = tx.QueryRow(ctx, `SELECT o.id::text,r.id::text,w.id::text FROM workflow_outbox w JOIN rounds r ON r.id=w.round_id JOIN observations o ON o.id=r.observation_id WHERE w.account_id=$1 AND w.game_id=$2 AND w.command_type='round.process' AND w.idempotency_key=$3`, string(in.AccountID), string(in.GameID), in.IdempotencyKey).Scan(&previousObservation, &previousRound, &previousCommand)
	if err == nil {
		if err = tx.Commit(ctx); err != nil {
			return "", "", "", err
		}
		return domain.ID(previousObservation), domain.ID(previousRound), domain.ID(previousCommand), nil
	}
	if !errors.Is(err, contracts.ErrNoRows) {
		return "", "", "", err
	}
	var priorRound string
	err = tx.QueryRow(ctx, `SELECT r.id::text FROM observations o JOIN rounds r ON r.game_id=o.game_id AND r.observation_id=o.id WHERE o.game_id=$1 AND o.source_image_sha256=$2 ORDER BY o.created_at LIMIT 1`, string(in.GameID), in.SourceSHA256).Scan(&priorRound)
	if err == nil && !in.AllowReuse {
		return "", domain.ID(priorRound), "", ErrDuplicateObservation
	}
	if err != nil && !isNoRows(err) {
		return "", "", "", err
	}
	var open bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM rounds WHERE game_id=$1 AND status IN ('queued','running','needs_attention'))`, string(in.GameID)).Scan(&open); err != nil {
		return "", "", "", err
	}
	if open {
		return "", "", "", ErrStateConflict
	}
	var sequence int64
	if err = tx.QueryRow(ctx, `SELECT COALESCE(max(sequence_number),0)+1 FROM rounds WHERE game_id=$1`, string(in.GameID)).Scan(&sequence); err != nil {
		return "", "", "", err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO observations(id,game_id,photo_object_path,source_image_sha256,mime_type,byte_size,player_statement,description_status) VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,''),'pending')`, string(in.ObservationID), string(in.GameID), in.PhotoPath, in.SourceSHA256, in.MIMEType, in.ByteSize, in.PlayerStatement); err != nil {
		return "", "", "", err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO rounds(id,game_id,sequence_number,kind,observation_id,status,stage,idempotency_key) VALUES($1,$2,$3,'discovery',$4,'queued','queued',$5)`, string(in.RoundID), string(in.GameID), sequence, string(in.ObservationID), in.IdempotencyKey); err != nil {
		return "", "", "", err
	}
	payload, _ := json.Marshal(map[string]string{"observation_id": string(in.ObservationID), "round_id": string(in.RoundID)})
	if _, err = tx.Exec(ctx, `INSERT INTO workflow_outbox(id,account_id,game_id,round_id,command_type,idempotency_key,payload) VALUES($1,$2,$3,$4,'round.process',$5,$6::jsonb)`, string(in.CommandID), string(in.AccountID), string(in.GameID), string(in.RoundID), in.IdempotencyKey, string(payload)); err != nil {
		return "", "", "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", "", "", err
	}
	return in.ObservationID, in.RoundID, in.CommandID, nil
}

func isNoRows(err error) bool { return errors.Is(err, contracts.ErrNoRows) }

func (s *Store) GetRound(ctx context.Context, roundID domain.ID) (domain.Round, error) {
	var r domain.Round
	var id, gid, kind, status, stage string
	var sequence int64
	var observationID *string
	var errorCode *string
	var started, completed *time.Time
	var created, updated time.Time
	err := s.DB.QueryRow(ctx, `SELECT id::text,game_id::text,sequence_number,kind,observation_id::text,status,stage,error_code,started_at,completed_at,created_at,updated_at FROM rounds WHERE id=$1`, string(roundID)).Scan(&id, &gid, &sequence, &kind, &observationID, &status, &stage, &errorCode, &started, &completed, &created, &updated)
	if err != nil {
		return r, normalizeNotFound(err)
	}
	r.ID, r.GameID, r.SequenceNumber, r.Kind, r.Status, r.Stage = domain.ID(id), domain.ID(gid), sequence, domain.RoundKind(kind), domain.RoundStatus(status), domain.RoundStage(stage)
	r.ErrorCode, r.StartedAt, r.CompletedAt, r.CreatedAt, r.UpdatedAt = errorCode, started, completed, created, updated
	if observationID != nil {
		v := domain.ID(*observationID)
		r.ObservationID = &v
	}
	return r, nil
}

func (s *Store) GetChronicle(ctx context.Context, roundID domain.ID) (*domain.Chronicle, error) {
	var c domain.Chronicle
	var id, gid, rid, historian string
	var outcome *string
	var suggestion *string
	var metadata []byte
	err := s.DB.QueryRow(ctx, `SELECT id::text,game_id::text,round_id::text,historian_agent_id::text,title,outcome,verdict,body,suggestion,metadata,created_at FROM chronicles WHERE round_id=$1`, string(roundID)).Scan(&id, &gid, &rid, &historian, &c.Title, &outcome, &c.Verdict, &c.Body, &suggestion, &metadata, &c.CreatedAt)
	if isNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.ID, c.GameID, c.RoundID, c.HistorianAgentID = domain.ID(id), domain.ID(gid), domain.ID(rid), domain.ID(historian)
	if outcome != nil {
		x := domain.ChronicleOutcome(*outcome)
		c.Outcome = &x
	}
	c.Suggestion = suggestion
	if len(metadata) > 0 {
		if err = json.Unmarshal(metadata, &c.Metadata); err != nil {
			return nil, err
		}
	}
	return &c, nil
}

func (s *Store) GetObservationByID(ctx context.Context, id domain.ID) (workflow.ObservationContext, error) {
	var o workflow.ObservationContext
	var observationID, gameID string
	var description, correction, statement *string
	err := s.DB.QueryRow(ctx, `SELECT id::text,game_id::text,photo_object_path,mime_type,visual_description,player_correction,player_statement,description_status FROM observations WHERE id=$1`, string(id)).Scan(&observationID, &gameID, &o.PhotoObjectPath, &o.MIMEType, &description, &correction, &statement, &o.DescriptionStatus)
	if err != nil {
		return o, normalizeNotFound(err)
	}
	o.ID, o.GameID = domain.ID(observationID), domain.ID(gameID)
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
