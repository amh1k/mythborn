package repository

import (
	"context"
	"encoding/json"
	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/workflow"
	"time"
)

func (s *Store) ListAgents(ctx context.Context, gameID domain.ID) ([]domain.Agent, error) {
	rows, err := s.DB.Query(ctx, `SELECT id::text,agent_type,display_name,personality_prompt,source_template_id::text,created_at FROM agents WHERE game_id=$1 ORDER BY agent_type`, string(gameID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Agent, 0, 4)
	for rows.Next() {
		var a domain.Agent
		var id, typ string
		var source *string
		if err = rows.Scan(&id, &typ, &a.DisplayName, &a.PersonalityPrompt, &source, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.ID, a.GameID, a.Type = domain.ID(id), gameID, domain.AgentType(typ)
		if source != nil {
			x := domain.ID(*source)
			a.SourceTemplateID = &x
		}
		items = append(items, a)
	}
	return items, rows.Err()
}

func (s *Store) ListBeliefs(ctx context.Context, gameID, agentID domain.ID) ([]domain.Belief, error) {
	var exists bool
	if err := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM agents WHERE game_id=$1 AND id=$2)`, string(gameID), string(agentID)).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}
	return s.loadBeliefs(ctx, gameID, agentID)
}

type BeliefHistoryItem struct {
	Revision domain.BeliefRevision `json:"revision"`
}

func (s *Store) ListBeliefRevisions(ctx context.Context, gameID, agentID domain.ID) ([]domain.BeliefRevision, error) {
	rows, err := s.DB.Query(ctx, `SELECT r.id::text,r.belief_id::text,r.version,r.event_id::text,r.cause,r.change_type,r.previous_claim,r.previous_state,r.new_claim,r.new_state,r.reason,r.source_round_id::text,r.source_conversation_summary_id::text,r.created_at FROM belief_revisions r JOIN beliefs b ON b.game_id=r.game_id AND b.id=r.belief_id WHERE r.game_id=$1 AND b.agent_id=$2 ORDER BY r.created_at,r.version`, string(gameID), string(agentID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.BeliefRevision, 0)
	for rows.Next() {
		var v domain.BeliefRevision
		var id, belief, event, cause, change, newState string
		var previousState *string
		var sourceRound, sourceSummary *string
		if err = rows.Scan(&id, &belief, &v.Version, &event, &cause, &change, &v.PreviousClaim, &previousState, &v.NewClaim, &newState, &v.Reason, &sourceRound, &sourceSummary, &v.CreatedAt); err != nil {
			return nil, err
		}
		v.ID, v.GameID, v.BeliefID, v.EventID = domain.ID(id), gameID, domain.ID(belief), domain.ID(event)
		v.Cause = domain.BeliefCause(cause)
		v.ChangeType = domain.BeliefChangeType(change)
		v.NewState = domain.BeliefState(newState)
		if previousState != nil {
			x := domain.BeliefState(*previousState)
			v.PreviousState = &x
		}
		if sourceRound != nil {
			x := domain.ID(*sourceRound)
			v.SourceRoundID = &x
		}
		if sourceSummary != nil {
			x := domain.ID(*sourceSummary)
			v.SourceConversationSummaryID = &x
		}
		items = append(items, v)
	}
	return items, rows.Err()
}

func (s *Store) ListTraditions(ctx context.Context, gameID domain.ID) ([]workflow.TraditionContext, error) {
	rows, err := s.DB.Query(ctx, `SELECT id::text,type,title,description,state,version,created_at,updated_at FROM traditions WHERE game_id=$1 ORDER BY created_at DESC`, string(gameID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]workflow.TraditionContext, 0)
	for rows.Next() {
		var t domain.Tradition
		var id, typ, state string
		if err = rows.Scan(&id, &typ, &t.Title, &t.Description, &state, &t.Version, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.ID, t.GameID, t.Type, t.State = domain.ID(id), gameID, domain.TraditionType(typ), domain.TraditionState(state)
		supporters, e := s.loadSupporters(ctx, gameID, t.ID)
		if e != nil {
			return nil, e
		}
		items = append(items, workflow.TraditionContext{Tradition: t, SupporterIDs: supporters})
	}
	return items, rows.Err()
}

func (s *Store) ListChronicles(ctx context.Context, gameID domain.ID) ([]domain.Chronicle, error) {
	rows, err := s.DB.Query(ctx, `SELECT id::text,round_id::text,historian_agent_id::text,title,outcome,verdict,body,suggestion,metadata,created_at FROM chronicles WHERE game_id=$1 ORDER BY created_at DESC`, string(gameID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Chronicle, 0)
	for rows.Next() {
		var c domain.Chronicle
		var id, rid, historian string
		var outcome, suggestion *string
		var metadata []byte
		if err = rows.Scan(&id, &rid, &historian, &c.Title, &outcome, &c.Verdict, &c.Body, &suggestion, &metadata, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.ID, c.GameID, c.RoundID, c.HistorianAgentID = domain.ID(id), gameID, domain.ID(rid), domain.ID(historian)
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
		items = append(items, c)
	}
	return items, rows.Err()
}

func (s *Store) LastCouncilAt(ctx context.Context, gameID domain.ID) (*time.Time, error) {
	var at *time.Time
	err := s.DB.QueryRow(ctx, `SELECT last_council_at FROM games WHERE id=$1`, string(gameID)).Scan(&at)
	if err != nil {
		return nil, normalizeNotFound(err)
	}
	return at, nil
}
