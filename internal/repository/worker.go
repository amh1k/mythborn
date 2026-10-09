package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/amh1k/mythborn/internal/contracts"
	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/workflow"
)

var ErrStateConflict = errors.New("state conflict")

func (s *Store) LoadRoundContext(ctx context.Context, gameID, roundID domain.ID) (workflow.RoundContext, error) {
	var out workflow.RoundContext
	game, err := scanGame(s.DB.QueryRow(ctx, `SELECT `+gameColumns+` FROM games WHERE id=$1`, string(gameID)))
	if err != nil {
		return out, normalizeNotFound(err)
	}
	out.Game = game
	var rid, gid, kind, status, stage string
	var sequence int64
	var observationID *string
	var started, completed *time.Time
	var errorCode *string
	var created, updated time.Time
	err = s.DB.QueryRow(ctx, `SELECT id::text,game_id::text,sequence_number,kind,observation_id::text,status,stage,error_code,started_at,completed_at,created_at,updated_at FROM rounds WHERE game_id=$1 AND id=$2`, string(gameID), string(roundID)).Scan(&rid, &gid, &sequence, &kind, &observationID, &status, &stage, &errorCode, &started, &completed, &created, &updated)
	if err != nil {
		return out, normalizeNotFound(err)
	}
	out.Round = domain.Round{ID: domain.ID(rid), GameID: domain.ID(gid), SequenceNumber: sequence, Kind: domain.RoundKind(kind), Status: domain.RoundStatus(status), Stage: domain.RoundStage(stage), ErrorCode: errorCode, StartedAt: started, CompletedAt: completed, CreatedAt: created, UpdatedAt: updated}
	if observationID != nil {
		var o workflow.ObservationContext
		var correction, statement, description *string
		var descStatus string
		err = s.DB.QueryRow(ctx, `SELECT id::text,game_id::text,photo_object_path,mime_type,visual_description,player_correction,player_statement,description_status FROM observations WHERE game_id=$1 AND id=$2`, string(gameID), *observationID).Scan(&rid, &gid, &o.PhotoObjectPath, &o.MIMEType, &description, &correction, &statement, &descStatus)
		if err != nil {
			return out, normalizeNotFound(err)
		}
		o.ID, o.GameID, o.DescriptionStatus = domain.ID(rid), domain.ID(gid), descStatus
		if description != nil {
			o.VisualDescription = *description
		}
		if correction != nil {
			o.PlayerCorrection = *correction
		}
		if statement != nil {
			o.PlayerStatement = *statement
		}
		out.Observation = &o
	}
	agentRows, err := s.DB.Query(ctx, `SELECT id::text,agent_type,display_name,personality_prompt,source_template_id::text,created_at FROM agents WHERE game_id=$1 ORDER BY agent_type`, string(gameID))
	if err != nil {
		return out, err
	}
	defer agentRows.Close()
	for agentRows.Next() {
		var agent domain.Agent
		var aid string
		var typ string
		var source *string
		if err = agentRows.Scan(&aid, &typ, &agent.DisplayName, &agent.PersonalityPrompt, &source, &agent.CreatedAt); err != nil {
			return out, err
		}
		agent.ID, agent.GameID, agent.Type = domain.ID(aid), gameID, domain.AgentType(typ)
		if source != nil {
			id := domain.ID(*source)
			agent.SourceTemplateID = &id
		}
		beliefs, e := s.loadBeliefs(ctx, gameID, agent.ID)
		if e != nil {
			return out, e
		}
		out.Agents = append(out.Agents, workflow.AgentContext{Agent: agent, Beliefs: beliefs})
		if agent.Type == domain.AgentTypeHistorian {
			out.HistorianAgentID = agent.ID
		}
	}
	if err = agentRows.Err(); err != nil {
		return out, err
	}
	if len(out.Agents) != 4 || out.HistorianAgentID == "" {
		return out, fmt.Errorf("game founding agents are incomplete")
	}
	tradRows, err := s.DB.Query(ctx, `SELECT id::text,type,title,description,state,version,created_at,updated_at FROM traditions WHERE game_id=$1 ORDER BY created_at`, string(gameID))
	if err != nil {
		return out, err
	}
	defer tradRows.Close()
	for tradRows.Next() {
		var t domain.Tradition
		var tid, typ, state string
		if err = tradRows.Scan(&tid, &typ, &t.Title, &t.Description, &state, &t.Version, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return out, err
		}
		t.ID, t.GameID, t.Type, t.State = domain.ID(tid), gameID, domain.TraditionType(typ), domain.TraditionState(state)
		supporters, e := s.loadSupporters(ctx, gameID, t.ID)
		if e != nil {
			return out, e
		}
		out.Traditions = append(out.Traditions, workflow.TraditionContext{Tradition: t, SupporterIDs: supporters})
	}
	return out, tradRows.Err()
}

func (s *Store) loadBeliefs(ctx context.Context, gameID, agentID domain.ID) ([]domain.Belief, error) {
	rows, err := s.DB.Query(ctx, `SELECT id::text,claim,state,version,created_at,updated_at FROM beliefs WHERE game_id=$1 AND agent_id=$2 ORDER BY created_at`, string(gameID), string(agentID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Belief, 0)
	for rows.Next() {
		var b domain.Belief
		var id, state string
		if err = rows.Scan(&id, &b.Claim, &state, &b.Version, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		b.ID, b.GameID, b.AgentID, b.State = domain.ID(id), gameID, agentID, domain.BeliefState(state)
		items = append(items, b)
	}
	return items, rows.Err()
}

func (s *Store) loadSupporters(ctx context.Context, gameID, traditionID domain.ID) ([]domain.ID, error) {
	rows, err := s.DB.Query(ctx, `SELECT agent_id::text FROM tradition_supporters WHERE game_id=$1 AND tradition_id=$2 ORDER BY agent_id`, string(gameID), string(traditionID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]domain.ID, 0, 4)
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, domain.ID(id))
	}
	return ids, rows.Err()
}

func (s *Store) SetRoundStage(ctx context.Context, gameID, roundID domain.ID, status domain.RoundStatus, stage domain.RoundStage) error {
	n, err := s.DB.Exec(ctx, `UPDATE rounds SET status=$3,stage=$4,started_at=CASE WHEN $3='running' THEN COALESCE(started_at,now()) ELSE started_at END,completed_at=CASE WHEN $3='complete' THEN now() WHEN $3 IN ('failed','abandoned','needs_attention') THEN completed_at ELSE NULL END,updated_at=now() WHERE game_id=$1 AND id=$2`, string(gameID), string(roundID), string(status), string(stage))
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SavePhotoDescription atomically persists the model's neutral evidence and
// moves the round to the matching review, clarity, or reaction stage.
func (s *Store) SavePhotoDescription(ctx context.Context, gameID, roundID, observationID domain.ID, description, status string) error {
	if strings.TrimSpace(description) == "" || len(description) > 3000 {
		return fmt.Errorf("invalid photo description")
	}
	var stage domain.RoundStage
	switch status {
	case "accepted":
		stage = domain.RoundStageReacting
	case "awaiting_review":
		stage = domain.RoundStageAwaitingReview
	case "uncertain":
		stage = domain.RoundStageAwaitingClarityChoice
	default:
		return fmt.Errorf("invalid description status")
	}
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var roundStatus, roundStage string
	var currentObservation *string
	if err = tx.QueryRow(ctx, `SELECT status,stage,observation_id::text FROM rounds WHERE game_id=$1 AND id=$2 FOR UPDATE`, string(gameID), string(roundID)).Scan(&roundStatus, &roundStage, &currentObservation); err != nil {
		return normalizeNotFound(err)
	}
	if currentObservation == nil || *currentObservation != string(observationID) || roundStatus != "running" {
		return ErrStateConflict
	}
	var currentDescription *string
	var currentStatus string
	if err = tx.QueryRow(ctx, `SELECT visual_description,description_status FROM observations WHERE game_id=$1 AND id=$2 FOR UPDATE`, string(gameID), string(observationID)).Scan(&currentDescription, &currentStatus); err != nil {
		return normalizeNotFound(err)
	}
	if currentStatus != "pending" {
		if currentStatus == status && currentDescription != nil && *currentDescription == description {
			return tx.Commit(ctx)
		}
		return ErrStateConflict
	}
	if _, err = tx.Exec(ctx, `UPDATE observations SET visual_description=$3,description_status=$4,accepted_at=CASE WHEN $4='accepted' THEN now() ELSE NULL END WHERE game_id=$1 AND id=$2`, string(gameID), string(observationID), description, status); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE rounds SET stage=$3,updated_at=now() WHERE game_id=$1 AND id=$2`, string(gameID), string(roundID), string(stage)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ApplyDescriptionDecision acknowledges one durable decision command. The API
// may have already written the same decision; this method validates it and
// makes the command's processed marker atomic with any remaining row update.
func (s *Store) ApplyDescriptionDecision(ctx context.Context, eventID, gameID, roundID, observationID domain.ID, decision, correction string) (bool, error) {
	if decision != "accept" && decision != "correct" && decision != "continue_uncertain" && decision != "try_another_photo" {
		return false, fmt.Errorf("invalid description decision")
	}
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var commandType, status string
	var commandGame, commandRound *string
	if err = tx.QueryRow(ctx, `SELECT command_type,status,game_id::text,round_id::text FROM workflow_outbox WHERE id=$1 FOR UPDATE`, string(eventID)).Scan(&commandType, &status, &commandGame, &commandRound); err != nil {
		return false, normalizeNotFound(err)
	}
	if commandType != "description.decision" || commandGame == nil || *commandGame != string(gameID) || commandRound == nil || *commandRound != string(roundID) {
		return false, ErrStateConflict
	}
	if status == "processed" {
		return decision == "try_another_photo", tx.Commit(ctx)
	}
	var roundStatus string
	var actualObservation *string
	if err = tx.QueryRow(ctx, `SELECT status,observation_id::text FROM rounds WHERE game_id=$1 AND id=$2 FOR UPDATE`, string(gameID), string(roundID)).Scan(&roundStatus, &actualObservation); err != nil {
		return false, normalizeNotFound(err)
	}
	if actualObservation == nil || *actualObservation != string(observationID) {
		return false, ErrStateConflict
	}
	if decision == "try_another_photo" {
		if roundStatus != "abandoned" {
			if _, err = tx.Exec(ctx, `UPDATE rounds SET status='abandoned',updated_at=now() WHERE game_id=$1 AND id=$2 AND status='running'`, string(gameID), string(roundID)); err != nil {
				return false, err
			}
		}
	} else {
		if roundStatus != "running" {
			return false, ErrStateConflict
		}
		if decision == "correct" && strings.TrimSpace(correction) == "" {
			return false, fmt.Errorf("correction is required")
		}
		if _, err = tx.Exec(ctx, `UPDATE observations SET description_status='accepted',accepted_at=COALESCE(accepted_at,now()),player_correction=CASE WHEN $3<>'' THEN $3 ELSE player_correction END WHERE game_id=$1 AND id=$2`, string(gameID), string(observationID), strings.TrimSpace(correction)); err != nil {
			return false, err
		}
		if _, err = tx.Exec(ctx, `UPDATE rounds SET stage='reacting',updated_at=now() WHERE game_id=$1 AND id=$2`, string(gameID), string(roundID)); err != nil {
			return false, err
		}
	}
	if decision == "try_another_photo" {
		if _, err = tx.Exec(ctx, `UPDATE workflow_outbox SET status='processed',processed_at=COALESCE(processed_at,now()),payload='{}'::jsonb,lease_until=NULL,error_code=NULL WHERE game_id=$1 AND round_id=$2 AND command_type IN ('round.process','round.retry','council.run','description.decision','game.end') AND status <> 'processed'`, string(gameID), string(roundID)); err != nil {
			return false, err
		}
	} else if _, err = tx.Exec(ctx, `UPDATE workflow_outbox SET status='processed',processed_at=COALESCE(processed_at,now()),payload='{}'::jsonb,lease_until=NULL,error_code=NULL WHERE id=$1`, string(eventID)); err != nil {
		return false, err
	}
	return decision == "try_another_photo", tx.Commit(ctx)
}

func (s *Store) CommitRound(ctx context.Context, commit workflow.RoundCommit) error {
	if commit.ProcessedCommandID == "" || commit.EventID == "" || commit.Chronicle.HistorianAgentID == "" {
		return fmt.Errorf("round commit requires event, processed command, and historian IDs")
	}
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	gameID, roundID := commit.Chronicle.GameID, commit.Chronicle.RoundID
	var gameStatus string
	if err = tx.QueryRow(ctx, `SELECT status FROM games WHERE id=$1 FOR UPDATE`, string(gameID)).Scan(&gameStatus); err != nil {
		return normalizeNotFound(err)
	}
	var status string
	if err = tx.QueryRow(ctx, `SELECT status FROM rounds WHERE game_id=$1 AND id=$2 FOR UPDATE`, string(gameID), string(roundID)).Scan(&status); err != nil {
		return normalizeNotFound(err)
	}
	if status == "complete" {
		if _, err = tx.Exec(ctx, `UPDATE workflow_outbox SET status='processed',processed_at=COALESCE(processed_at,now()),payload='{}'::jsonb WHERE game_id=$1 AND round_id=$2 AND command_type IN ('round.process','round.retry','council.run','description.decision','game.end') AND status <> 'processed'`, string(gameID), string(roundID)); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if status != "running" {
		return fmt.Errorf("%w: round cannot complete from %s", ErrStateConflict, status)
	}
	var kind string
	if err = tx.QueryRow(ctx, `SELECT kind FROM rounds WHERE game_id=$1 AND id=$2`, string(gameID), string(roundID)).Scan(&kind); err != nil {
		return err
	}
	if kind != string(domain.RoundKindDiscovery) && kind != string(domain.RoundKindCouncil) && kind != string(domain.RoundKindClosing) {
		return fmt.Errorf("unknown round kind %q", kind)
	}
	if kind == string(domain.RoundKindClosing) && gameStatus != "ending" {
		return fmt.Errorf("%w: game is not ending", ErrStateConflict)
	}
	if kind != string(domain.RoundKindClosing) && gameStatus != "active" && gameStatus != "ending_requested" {
		return fmt.Errorf("%w: game is not processing rounds", ErrStateConflict)
	}
	if kind == string(domain.RoundKindClosing) && (len(commit.BeliefProposals) != 0 || len(commit.TraditionChanges) != 0) {
		return fmt.Errorf("closing round cannot revise beliefs or traditions")
	}
	for _, proposal := range commit.BeliefProposals {
		if err = applyBeliefProposal(ctx, tx, gameID, roundID, commit.EventID, kind, nil, proposal); err != nil {
			return err
		}
	}
	for _, change := range commit.TraditionChanges {
		if err = applyTraditionChange(ctx, tx, gameID, roundID, commit.EventID, change); err != nil {
			return err
		}
	}
	chron := commit.Chronicle
	if chron.ID == "" {
		chron.ID, err = newID()
		if err != nil {
			return err
		}
	}
	metadata, marshalErr := json.Marshal(chron.Metadata)
	if marshalErr != nil {
		return marshalErr
	}
	if len(metadata) > 8192 {
		return fmt.Errorf("chronicle metadata exceeds limit")
	}
	if len(metadata) == 0 || string(metadata) == "null" {
		metadata = []byte(`{}`)
	}
	var outcome any
	if chron.Outcome != nil {
		outcome = string(*chron.Outcome)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO chronicles(id,game_id,round_id,historian_agent_id,title,outcome,verdict,body,suggestion,metadata) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb)`, string(chron.ID), string(gameID), string(roundID), string(chron.HistorianAgentID), chron.Title, outcome, chron.Verdict, chron.Body, chron.Suggestion, string(metadata)); err != nil {
		return err
	}
	for _, doc := range commit.SearchDocuments {
		if err = upsertSearchDocument(ctx, tx, gameID, doc); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE rounds SET status='complete',stage='complete',completed_at=now(),updated_at=now() WHERE game_id=$1 AND id=$2`, string(gameID), string(roundID)); err != nil {
		return err
	}
	if kind == "closing" {
		if _, err = tx.Exec(ctx, `UPDATE games SET status='archived',archived_at=now(),updated_at=now() WHERE id=$1`, string(gameID)); err != nil {
			return err
		}
	} else if kind == "council" {
		if _, err = tx.Exec(ctx, `UPDATE games SET last_council_at=now(),updated_at=now() WHERE id=$1`, string(gameID)); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE workflow_outbox SET status='processed',processed_at=COALESCE(processed_at,now()),payload='{}'::jsonb,lease_until=NULL,error_code=NULL WHERE game_id=$1 AND round_id=$2 AND command_type IN ('round.process','round.retry','council.run','description.decision','game.end') AND status <> 'processed'`, string(gameID), string(roundID)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func applyBeliefProposal(ctx context.Context, tx contracts.Tx, gameID, roundID, eventID domain.ID, cause string, summaryID *domain.ID, p workflow.BeliefProposal) error {
	if strings.TrimSpace(p.NewClaim) == "" || strings.TrimSpace(p.Reason) == "" {
		return fmt.Errorf("invalid belief proposal")
	}
	if p.BeliefID == nil {
		id := p.NewBeliefID
		if id == "" {
			var err error
			id, err = newID()
			if err != nil {
				return err
			}
		}
		revision, err := newID()
		if err != nil {
			return err
		}
		if p.ChangeType != domain.BeliefChangeFormed {
			return fmt.Errorf("new belief must use formed change type")
		}
		if _, err = tx.Exec(ctx, `INSERT INTO beliefs(id,game_id,agent_id,claim,state,version) VALUES($1,$2,$3,$4,$5,1)`, string(id), string(gameID), string(p.AgentID), p.NewClaim, string(p.NewState)); err != nil {
			return err
		}
		return insertBeliefRevision(ctx, tx, revision, gameID, id, 1, eventID, cause, p, nil, nil, roundID, summaryID)
	}
	var agent, claim, state string
	var version int64
	err := tx.QueryRow(ctx, `SELECT agent_id::text,claim,state,version FROM beliefs WHERE game_id=$1 AND id=$2 FOR UPDATE`, string(gameID), string(*p.BeliefID)).Scan(&agent, &claim, &state, &version)
	if err != nil {
		return normalizeNotFound(err)
	}
	if domain.ID(agent) != p.AgentID {
		return fmt.Errorf("belief proposal agent mismatch")
	}
	if claim == p.NewClaim && state == string(p.NewState) {
		return nil
	}
	prevClaim, prevState := claim, domain.BeliefState(state)
	next := version + 1
	if _, err = tx.Exec(ctx, `UPDATE beliefs SET claim=$3,state=$4,version=$5,updated_at=now() WHERE game_id=$1 AND id=$2`, string(gameID), string(*p.BeliefID), p.NewClaim, string(p.NewState), next); err != nil {
		return err
	}
	revision, err := newID()
	if err != nil {
		return err
	}
	return insertBeliefRevision(ctx, tx, revision, gameID, *p.BeliefID, next, eventID, cause, p, &prevClaim, &prevState, roundID, summaryID)
}

func insertBeliefRevision(ctx context.Context, tx contracts.Tx, id, gameID, beliefID domain.ID, version int64, eventID domain.ID, cause string, p workflow.BeliefProposal, previousClaim *string, previousState *domain.BeliefState, roundID domain.ID, summaryID *domain.ID) error {
	var sourceRound, sourceSummary any
	if cause == "messenger" {
		if summaryID != nil {
			sourceSummary = string(*summaryID)
		}
	} else {
		sourceRound = string(roundID)
	}
	changeType := string(p.ChangeType)
	returnExec, err := tx.Exec(ctx, `INSERT INTO belief_revisions(id,game_id,belief_id,version,event_id,cause,change_type,previous_claim,previous_state,new_claim,new_state,reason,source_round_id,source_conversation_summary_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, string(id), string(gameID), string(beliefID), version, string(eventID), cause, changeType, previousClaim, previousState, p.NewClaim, string(p.NewState), p.Reason, sourceRound, sourceSummary)
	_ = returnExec
	return err
}

func applyTraditionChange(ctx context.Context, tx contracts.Tx, gameID, roundID, eventID domain.ID, c workflow.TraditionChange) error {
	if c.TraditionID == "" || strings.TrimSpace(c.Title) == "" || strings.TrimSpace(c.Description) == "" || strings.TrimSpace(c.Reason) == "" || len(c.SupporterIDs) > 4 {
		return fmt.Errorf("invalid tradition change")
	}
	for i := range c.SupporterIDs {
		for j := 0; j < i; j++ {
			if c.SupporterIDs[i] == c.SupporterIDs[j] {
				return fmt.Errorf("duplicate tradition supporter")
			}
		}
	}
	var title, description, state string
	var version int64
	err := tx.QueryRow(ctx, `SELECT title,description,state,version FROM traditions WHERE game_id=$1 AND id=$2 FOR UPDATE`, string(gameID), string(c.TraditionID)).Scan(&title, &description, &state, &version)
	newRecord := errors.Is(err, contracts.ErrNoRows)
	if err != nil && !newRecord {
		return err
	}
	var previousTitle, previousDescription, previousState any
	previousIDs := make([]string, 0, 4)
	if !newRecord {
		previousTitle, previousDescription, previousState = title, description, state
		rows, qerr := tx.Query(ctx, `SELECT agent_id::text FROM tradition_supporters WHERE game_id=$1 AND tradition_id=$2 ORDER BY agent_id`, string(gameID), string(c.TraditionID))
		if qerr != nil {
			return qerr
		}
		for rows.Next() {
			var id string
			if qerr = rows.Scan(&id); qerr != nil {
				rows.Close()
				return qerr
			}
			previousIDs = append(previousIDs, id)
		}
		if qerr = rows.Err(); qerr != nil {
			rows.Close()
			return qerr
		}
		rows.Close()
		version++
	} else {
		version = 1
	}
	if newRecord {
		if _, err = tx.Exec(ctx, `INSERT INTO traditions(id,game_id,type,title,description,state,version) VALUES($1,$2,$3,$4,$5,$6,1)`, string(c.TraditionID), string(gameID), string(c.Type), c.Title, c.Description, string(c.State)); err != nil {
			return err
		}
	} else {
		if _, err = tx.Exec(ctx, `UPDATE traditions SET title=$3,description=$4,state=$5,version=$6,updated_at=now() WHERE game_id=$1 AND id=$2`, string(gameID), string(c.TraditionID), c.Title, c.Description, string(c.State), version); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `DELETE FROM tradition_supporters WHERE game_id=$1 AND tradition_id=$2`, string(gameID), string(c.TraditionID)); err != nil {
		return err
	}
	for _, supporter := range c.SupporterIDs {
		if _, err = tx.Exec(ctx, `INSERT INTO tradition_supporters(game_id,tradition_id,agent_id) VALUES($1,$2,$3)`, string(gameID), string(c.TraditionID), string(supporter)); err != nil {
			return err
		}
	}
	newIDs := make([]string, len(c.SupporterIDs))
	for i, id := range c.SupporterIDs {
		newIDs[i] = string(id)
	}
	revision, err := newID()
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO tradition_revisions(id,game_id,tradition_id,version,event_id,source_round_id,previous_title,previous_description,previous_state,new_title,new_description,new_state,previous_supporter_ids,new_supporter_ids,reason) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::uuid[],$14::uuid[],$15)`, string(revision), string(gameID), string(c.TraditionID), version, string(eventID), string(roundID), previousTitle, previousDescription, previousState, c.Title, c.Description, string(c.State), pgUUIDArray(previousIDs), pgUUIDArray(newIDs), c.Reason)
	return err
}

func (s *Store) LoadMessengerContext(ctx context.Context, gameID, roundID, agentID domain.ID) (workflow.MessengerContext, error) {
	var out workflow.MessengerContext
	var err error
	out.Game, err = scanGame(s.DB.QueryRow(ctx, `SELECT `+gameColumns+` FROM games WHERE id=$1`, string(gameID)))
	if err != nil {
		return out, normalizeNotFound(err)
	}
	var rid, gid, kind, status, stage string
	var seq int64
	var obs *string
	var errorCode *string
	var started, completed *time.Time
	var created, updated time.Time
	err = s.DB.QueryRow(ctx, `SELECT id::text,game_id::text,sequence_number,kind,observation_id::text,status,stage,error_code,started_at,completed_at,created_at,updated_at FROM rounds WHERE game_id=$1 AND id=$2`, string(gameID), string(roundID)).Scan(&rid, &gid, &seq, &kind, &obs, &status, &stage, &errorCode, &started, &completed, &created, &updated)
	if err != nil {
		return out, normalizeNotFound(err)
	}
	out.Round = domain.Round{ID: domain.ID(rid), GameID: domain.ID(gid), SequenceNumber: seq, Kind: domain.RoundKind(kind), Status: domain.RoundStatus(status), Stage: domain.RoundStage(stage), ErrorCode: errorCode, StartedAt: started, CompletedAt: completed, CreatedAt: created, UpdatedAt: updated}
	if out.Game.PlayerRole != domain.PlayerRoleMessenger || out.Round.Kind != domain.RoundKindDiscovery || out.Round.Status != domain.RoundStatusComplete {
		return out, ErrStateConflict
	}
	var aid, typ string
	var source *string
	err = s.DB.QueryRow(ctx, `SELECT id::text,agent_type,display_name,personality_prompt,source_template_id::text,created_at FROM agents WHERE game_id=$1 AND id=$2`, string(gameID), string(agentID)).Scan(&aid, &typ, &out.Agent.DisplayName, &out.Agent.PersonalityPrompt, &source, &out.Agent.CreatedAt)
	if err != nil {
		return out, normalizeNotFound(err)
	}
	out.Agent.ID, out.Agent.GameID, out.Agent.Type = domain.ID(aid), gameID, domain.AgentType(typ)
	if source != nil {
		id := domain.ID(*source)
		out.Agent.SourceTemplateID = &id
	}
	out.Beliefs, err = s.loadBeliefs(ctx, gameID, agentID)
	if err != nil {
		return out, err
	}
	var summary domain.ConversationSummary
	var sid, lastEvent string
	var summaryText string
	err = s.DB.QueryRow(ctx, `SELECT id::text,summary,version,last_event_id::text,created_at,updated_at FROM conversation_summaries WHERE game_id=$1 AND round_id=$2 AND agent_id=$3`, string(gameID), string(roundID), string(agentID)).Scan(&sid, &summaryText, &summary.Version, &lastEvent, &summary.CreatedAt, &summary.UpdatedAt)
	if err == nil {
		summary.ID, summary.GameID, summary.RoundID, summary.AgentID, summary.Summary, summary.LastEventID = domain.ID(sid), gameID, roundID, agentID, summaryText, domain.ID(lastEvent)
		out.Summary = &summary
	} else if !errors.Is(err, contracts.ErrNoRows) {
		return out, err
	}
	return out, nil
}

func (s *Store) CommitMessenger(ctx context.Context, c workflow.MessengerCommit) error {
	if c.EventID == "" || c.ProcessedCommandID == "" || c.GameID == "" || c.RoundID == "" || c.AgentID == "" || len(c.ReplyPayload) > 8192 || len(c.NewSummary) > 12000 {
		return fmt.Errorf("messenger result exceeds storage limit")
	}
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var commandStatus string
	if err = tx.QueryRow(ctx, `SELECT status FROM workflow_outbox WHERE id=$1 FOR UPDATE`, string(c.ProcessedCommandID)).Scan(&commandStatus); err != nil {
		return normalizeNotFound(err)
	}
	if commandStatus == "processed" {
		return tx.Commit(ctx)
	}
	var role, status string
	if err = tx.QueryRow(ctx, `SELECT g.player_role,g.status FROM games g WHERE g.id=$1 FOR UPDATE`, string(c.GameID)).Scan(&role, &status); err != nil {
		return normalizeNotFound(err)
	}
	if role != "messenger" || status != "active" {
		return ErrStateConflict
	}
	var roundKind, roundStatus string
	if err = tx.QueryRow(ctx, `SELECT kind,status FROM rounds WHERE game_id=$1 AND id=$2 FOR SHARE`, string(c.GameID), string(c.RoundID)).Scan(&roundKind, &roundStatus); err != nil {
		return normalizeNotFound(err)
	}
	if roundKind != "discovery" || roundStatus != "complete" {
		return ErrStateConflict
	}
	var agentType string
	if err = tx.QueryRow(ctx, `SELECT agent_type FROM agents WHERE game_id=$1 AND id=$2`, string(c.GameID), string(c.AgentID)).Scan(&agentType); err != nil {
		return normalizeNotFound(err)
	}
	_ = agentType
	var currentVersion int64
	var currentSummaryID string
	err = tx.QueryRow(ctx, `SELECT id::text,version FROM conversation_summaries WHERE game_id=$1 AND round_id=$2 AND agent_id=$3 FOR UPDATE`, string(c.GameID), string(c.RoundID), string(c.AgentID)).Scan(&currentSummaryID, &currentVersion)
	newSummary := errors.Is(err, contracts.ErrNoRows)
	if err != nil && !newSummary {
		return err
	}
	if (newSummary && c.ExpectedVersion != 0) || (!newSummary && currentVersion != c.ExpectedVersion) {
		return ErrStateConflict
	}
	summaryID := c.SummaryID
	if summaryID == "" {
		if newSummary {
			summaryID, err = newID()
			if err != nil {
				return err
			}
		} else {
			summaryID = domain.ID(currentSummaryID)
		}
	} else if !newSummary && summaryID != domain.ID(currentSummaryID) {
		return ErrStateConflict
	}
	if newSummary {
		if _, err = tx.Exec(ctx, `INSERT INTO conversation_summaries(id,game_id,round_id,agent_id,summary,version,last_event_id) VALUES($1,$2,$3,$4,$5,1,$6)`, string(summaryID), string(c.GameID), string(c.RoundID), string(c.AgentID), c.NewSummary, string(c.EventID)); err != nil {
			return err
		}
	} else {
		if _, err = tx.Exec(ctx, `UPDATE conversation_summaries SET summary=$4,version=version+1,last_event_id=$5,updated_at=now() WHERE game_id=$1 AND round_id=$2 AND agent_id=$3`, string(c.GameID), string(c.RoundID), string(c.AgentID), c.NewSummary, string(c.EventID)); err != nil {
			return err
		}
	}
	for _, proposal := range c.BeliefProposals {
		if err = applyBeliefProposal(ctx, tx, c.GameID, c.RoundID, c.EventID, "messenger", &summaryID, proposal); err != nil {
			return err
		}
	}
	for _, doc := range c.SearchDocuments {
		if err = upsertSearchDocument(ctx, tx, c.GameID, doc); err != nil {
			return err
		}
	}
	markerPayload, _ := json.Marshal(map[string]string{"agent_id": string(c.AgentID), "summary_id": string(summaryID)})
	if _, err = tx.Exec(ctx, `UPDATE workflow_outbox SET status='processed',processed_at=now(),payload=$4::jsonb,result_payload=$2::jsonb,result_expires_at=$3,error_code=NULL WHERE id=$1`, string(c.ProcessedCommandID), string(c.ReplyPayload), c.ResultExpiresAt, string(markerPayload)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func upsertSearchDocument(ctx context.Context, tx contracts.Tx, gameID domain.ID, doc workflow.SearchDocument) error {
	if doc.GameID != gameID || strings.TrimSpace(doc.Content) == "" || len(doc.Content) > 12000 {
		return fmt.Errorf("invalid search document content or game scope")
	}
	if doc.SourceVersion < 1 {
		doc.SourceVersion = 1
	}
	sourceCount := 0
	sourceType := ""
	sourceID := domain.ID("")
	for _, source := range []struct {
		name string
		id   *domain.ID
	}{{"observation", doc.ObservationID}, {"chronicle", doc.ChronicleID}, {"belief", doc.BeliefID}, {"conversation_summary", doc.ConversationSummaryID}} {
		if source.id != nil {
			sourceCount++
			sourceType = source.name
			sourceID = *source.id
		}
	}
	if sourceCount != 1 {
		return fmt.Errorf("search document must have exactly one source")
	}
	if (sourceType == "belief" || sourceType == "conversation_summary") != (doc.AgentID != nil) {
		return fmt.Errorf("search document visibility does not match source type")
	}
	if doc.AgentID != nil {
		if sourceType == "belief" {
			var owner string
			if err := tx.QueryRow(ctx, `SELECT agent_id::text FROM beliefs WHERE game_id=$1 AND id=$2`, string(gameID), string(sourceID)).Scan(&owner); err != nil {
				return normalizeNotFound(err)
			}
			if owner != string(*doc.AgentID) {
				return fmt.Errorf("belief search document agent scope mismatch")
			}
		}
		if sourceType == "conversation_summary" {
			var owner string
			if err := tx.QueryRow(ctx, `SELECT agent_id::text FROM conversation_summaries WHERE game_id=$1 AND id=$2`, string(gameID), string(sourceID)).Scan(&owner); err != nil {
				return normalizeNotFound(err)
			}
			if owner != string(*doc.AgentID) {
				return fmt.Errorf("conversation search document agent scope mismatch")
			}
		}
	}
	if len(doc.Embedding) != 0 && len(doc.Embedding) != 768 {
		return fmt.Errorf("search document embedding must have 768 dimensions")
	}
	var vector any
	if len(doc.Embedding) == 768 {
		vector = pgFloatVector(doc.Embedding)
		if vector == nil {
			return fmt.Errorf("search document embedding contains non-finite values")
		}
	}
	var observation, chronicle, belief, summary, agent any
	if doc.ObservationID != nil {
		observation = string(*doc.ObservationID)
	}
	if doc.ChronicleID != nil {
		chronicle = string(*doc.ChronicleID)
	}
	if doc.BeliefID != nil {
		belief = string(*doc.BeliefID)
	}
	if doc.ConversationSummaryID != nil {
		summary = string(*doc.ConversationSummaryID)
	}
	if doc.AgentID != nil {
		agent = string(*doc.AgentID)
	}
	var id domain.ID
	var err error
	if id, err = newID(); err != nil {
		return err
	}
	column := map[string]string{"observation": "observation_id", "chronicle": "chronicle_id", "belief": "belief_id", "conversation_summary": "conversation_summary_id"}[sourceType]
	query := `INSERT INTO search_documents(id,game_id,agent_id,observation_id,chronicle_id,belief_id,conversation_summary_id,content,embedding,source_version) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9::vector,$10) ON CONFLICT (` + column + `) WHERE ` + column + ` IS NOT NULL DO UPDATE SET agent_id=EXCLUDED.agent_id,content=EXCLUDED.content,embedding=EXCLUDED.embedding,source_version=EXCLUDED.source_version,updated_at=now() WHERE EXCLUDED.source_version >= search_documents.source_version`
	_, err = tx.Exec(ctx, query, string(id), string(gameID), agent, observation, chronicle, belief, summary, doc.Content, vector, doc.SourceVersion)
	return err
}

func pgFloatVector(values []float32) any {
	parts := make([]string, len(values))
	for i, v := range values {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return nil
		}
		parts[i] = strconv.FormatFloat(float64(v), 'f', 6, 32)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func (s *Store) ClaimOutbox(ctx context.Context, limit int, lease time.Duration) ([]domain.WorkflowCommand, error) {
	if limit < 1 || limit > 100 {
		limit = 25
	}
	if lease < time.Second {
		lease = 30 * time.Second
	}
	rows, err := s.DB.Query(ctx, `WITH picked AS (SELECT id FROM workflow_outbox WHERE ((status IN ('pending','failed') AND COALESCE(retry_at,created_at)<=now()) OR (status='dispatched' AND lease_until<now())) ORDER BY COALESCE(retry_at,created_at),created_at FOR UPDATE SKIP LOCKED LIMIT $1) UPDATE workflow_outbox o SET status='dispatched',attempt_count=attempt_count+1,lease_until=now()+$2::interval FROM picked WHERE o.id=picked.id RETURNING o.id::text,o.account_id::text,o.game_id::text,o.round_id::text,o.command_type,o.idempotency_key,o.payload,o.result_payload,o.result_expires_at,o.status,o.attempt_count,o.created_at`, limit, lease.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	commands := make([]domain.WorkflowCommand, 0)
	for rows.Next() {
		var c domain.WorkflowCommand
		var id, account, typ, status string
		var gameID, roundID *string
		var result []byte
		var expires *time.Time
		if err = rows.Scan(&id, &account, &gameID, &roundID, &typ, &c.IdempotencyKey, &c.Payload, &result, &expires, &status, &c.AttemptCount, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.ID, c.AccountID, c.Type, c.Status = domain.ID(id), domain.ID(account), typ, domain.CommandStatus(status)
		if gameID != nil {
			v := domain.ID(*gameID)
			c.GameID = &v
		}
		if roundID != nil {
			v := domain.ID(*roundID)
			c.RoundID = &v
		}
		c.ResultPayload = result
		c.ResultExpiresAt = expires
		commands = append(commands, c)
	}
	return commands, rows.Err()
}

func (s *Store) MarkOutboxDelivered(ctx context.Context, id domain.ID) error {
	n, err := s.DB.Exec(ctx, `UPDATE workflow_outbox SET status='dispatched',delivered_at=COALESCE(delivered_at,now()),lease_until=NULL,error_code=NULL WHERE id=$1 AND status='dispatched'`, string(id))
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
func (s *Store) MarkOutboxFailed(ctx context.Context, id domain.ID, errorCode string, retryAt time.Time) error {
	if len(errorCode) > 100 {
		errorCode = errorCode[:100]
	}
	n, err := s.DB.Exec(ctx, `UPDATE workflow_outbox SET status='failed',error_code=$2,retry_at=$3,lease_until=NULL WHERE id=$1 AND status<>'processed'`, string(id), errorCode, retryAt)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// LoadGamePhotoPaths is only valid after the game has been marked deleting.
// External photo deletion happens before CompleteGameDeletion is called.
func (s *Store) LoadGamePhotoPaths(ctx context.Context, gameID domain.ID) ([]string, error) {
	var status string
	if err := s.DB.QueryRow(ctx, `SELECT status FROM games WHERE id=$1`, string(gameID)).Scan(&status); err != nil {
		return nil, normalizeNotFound(err)
	}
	if status != "deleting" {
		return nil, ErrStateConflict
	}
	rows, err := s.DB.Query(ctx, `SELECT photo_object_path FROM observations WHERE game_id=$1 ORDER BY id`, string(gameID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	paths := make([]string, 0)
	for rows.Next() {
		var path string
		if err = rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, rows.Err()
}

// CompleteGameDeletion removes all relational game data only after its worker
// has stopped durable workflows and deleted every external photo object.
func (s *Store) CompleteGameDeletion(ctx context.Context, gameID, commandID domain.ID) error {
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status string
	if err = tx.QueryRow(ctx, `SELECT status FROM games WHERE id=$1 FOR UPDATE`, string(gameID)).Scan(&status); err != nil {
		return normalizeNotFound(err)
	}
	if status != "deleting" {
		return ErrStateConflict
	}
	var commandType string
	var commandGame *string
	if err = tx.QueryRow(ctx, `SELECT command_type,game_id::text FROM workflow_outbox WHERE id=$1 FOR UPDATE`, string(commandID)).Scan(&commandType, &commandGame); err != nil {
		return normalizeNotFound(err)
	}
	if commandType != "game.delete" || commandGame == nil || *commandGame != string(gameID) {
		return ErrStateConflict
	}
	if _, err = tx.Exec(ctx, `UPDATE workflow_outbox SET status='processed',processed_at=COALESCE(processed_at,now()),payload='{}'::jsonb,lease_until=NULL WHERE id=$1`, string(commandID)); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM games WHERE id=$1`, string(gameID)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func pgUUIDArray(ids []string) string {
	quoted := make([]string, len(ids))
	for i, id := range ids {
		quoted[i] = `"` + strings.ReplaceAll(id, `"`, `\"`) + `"`
	}
	return "{" + strings.Join(quoted, ",") + "}"
}

func (s *Store) FindCouncilCandidates(ctx context.Context, now time.Time, minimumInterval time.Duration) ([]domain.ID, error) {
	rows, err := s.DB.Query(ctx, `SELECT g.id::text FROM games g WHERE g.status='active' AND COALESCE(g.last_council_at,'-infinity'::timestamptz)<$1-$2::interval AND NOT EXISTS(SELECT 1 FROM rounds r WHERE r.game_id=g.id AND r.status IN ('queued','running','needs_attention')) AND (EXISTS(SELECT 1 FROM chronicles c JOIN rounds r ON r.id=c.round_id WHERE c.game_id=g.id AND r.kind IN ('discovery','council') AND c.outcome='unresolved' AND c.created_at>COALESCE(g.last_council_at,'-infinity'::timestamptz)) OR EXISTS(SELECT 1 FROM traditions t WHERE t.game_id=g.id AND t.state='contested') OR EXISTS(SELECT 1 FROM conversation_summaries cs WHERE cs.game_id=g.id AND cs.updated_at>COALESCE(g.last_council_at,'-infinity'::timestamptz))) ORDER BY COALESCE(g.last_council_at,g.created_at)`, now, minimumInterval.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]domain.ID, 0)
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, domain.ID(id))
	}
	return ids, rows.Err()
}

func (s *Store) EnqueueCouncilIfEligible(ctx context.Context, gameID domain.ID, now time.Time) (domain.WorkflowCommand, bool, error) {
	tx, err := s.DB.BeginTx(ctx)
	if err != nil {
		return domain.WorkflowCommand{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var owner, status string
	var role string
	var last *time.Time
	if err = tx.QueryRow(ctx, `SELECT owner_account_id::text,status,player_role,last_council_at FROM games WHERE id=$1 FOR UPDATE`, string(gameID)).Scan(&owner, &status, &role, &last); err != nil {
		return domain.WorkflowCommand{}, false, normalizeNotFound(err)
	}
	if status != "active" {
		return domain.WorkflowCommand{}, false, nil
	}
	if last != nil && now.Sub(*last) < 24*time.Hour {
		return domain.WorkflowCommand{}, false, nil
	}
	var open bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM rounds WHERE game_id=$1 AND status IN ('queued','running','needs_attention'))`, string(gameID)).Scan(&open); err != nil {
		return domain.WorkflowCommand{}, false, err
	}
	if open {
		return domain.WorkflowCommand{}, false, nil
	}
	var eligible bool
	if err = tx.QueryRow(ctx, `SELECT (EXISTS(SELECT 1 FROM chronicles c JOIN rounds r ON r.id=c.round_id WHERE c.game_id=$1 AND r.kind IN ('discovery','council') AND c.outcome='unresolved' AND c.created_at>COALESCE((SELECT last_council_at FROM games WHERE id=$1),'-infinity'::timestamptz)) OR EXISTS(SELECT 1 FROM traditions WHERE game_id=$1 AND state='contested') OR EXISTS(SELECT 1 FROM conversation_summaries WHERE game_id=$1 AND updated_at>COALESCE((SELECT last_council_at FROM games WHERE id=$1),'-infinity'::timestamptz)))`, string(gameID)).Scan(&eligible); err != nil {
		return domain.WorkflowCommand{}, false, err
	}
	if !eligible {
		return domain.WorkflowCommand{}, false, nil
	}
	var sequence int64
	if err = tx.QueryRow(ctx, `SELECT COALESCE(max(sequence_number),0)+1 FROM rounds WHERE game_id=$1`, string(gameID)).Scan(&sequence); err != nil {
		return domain.WorkflowCommand{}, false, err
	}
	roundID, err := newID()
	if err != nil {
		return domain.WorkflowCommand{}, false, err
	}
	commandID, err := newID()
	if err != nil {
		return domain.WorkflowCommand{}, false, err
	}
	key := fmt.Sprintf("scheduled-council-%s", now.UTC().Format("20060102T15"))
	payload, _ := json.Marshal(map[string]string{"round_id": string(roundID)})
	if _, err = tx.Exec(ctx, `INSERT INTO rounds(id,game_id,sequence_number,kind,status,stage,idempotency_key) VALUES($1,$2,$3,'council','queued','reacting',$4)`, string(roundID), string(gameID), sequence, key); err != nil {
		return domain.WorkflowCommand{}, false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO workflow_outbox(id,account_id,game_id,round_id,command_type,idempotency_key,payload) VALUES($1,$2,$3,$4,'council.run',$5,$6::jsonb)`, string(commandID), owner, string(gameID), string(roundID), key, string(payload)); err != nil {
		return domain.WorkflowCommand{}, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.WorkflowCommand{}, false, err
	}
	return domain.WorkflowCommand{ID: commandID, AccountID: domain.ID(owner), GameID: &gameID, RoundID: &roundID, Type: string(workflow.CommandRunCouncil), IdempotencyKey: key, Payload: payload, Status: domain.CommandStatusPending, CreatedAt: now}, true, nil
}

func normalizeNotFound(err error) error {
	if errors.Is(err, contracts.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
