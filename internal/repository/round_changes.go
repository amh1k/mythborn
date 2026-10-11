package repository

import (
	"context"
	"github.com/amh1k/mythborn/internal/domain"
)

// These are committed revisions, never a model's unaccepted proposals.
type SavedChange struct {
	ID          string  `json:"id"`
	Kind        string  `json:"kind"`
	Name        string  `json:"name"`
	Before      *string `json:"before,omitempty"`
	After       string  `json:"after"`
	BeforeState *string `json:"before_state,omitempty"`
	AfterState  string  `json:"after_state"`
	Reason      string  `json:"reason"`
}

func (s *Store) ListRoundChanges(ctx context.Context, gameID, roundID domain.ID) ([]SavedChange, error) {
	rows, err := s.DB.Query(ctx, `SELECT r.id::text,'belief',a.display_name,r.previous_claim,r.new_claim,r.previous_state,r.new_state,r.reason FROM belief_revisions r JOIN beliefs b ON (b.game_id,b.id)=(r.game_id,r.belief_id) JOIN agents a ON (a.game_id,a.id)=(b.game_id,b.agent_id) WHERE r.game_id=$1 AND r.source_round_id=$2
    UNION ALL SELECT r.id::text,'tradition',r.new_title,COALESCE(r.previous_title || ' — ' || r.previous_description,r.new_title),r.new_title || ' — ' || r.new_description,r.previous_state,r.new_state,r.reason FROM tradition_revisions r WHERE r.game_id=$1 AND r.source_round_id=$2
    UNION ALL SELECT r.id::text,'mystery',r.after_snapshot->>'title',r.before_snapshot->>'evidence_summary',r.after_snapshot->>'evidence_summary',r.before_snapshot->>'state',r.after_snapshot->>'state',r.reason FROM mystery_revisions r WHERE r.game_id=$1 AND r.source_round_id=$2
    UNION ALL SELECT r.id::text,'relationship',a.display_name || ' → ' || b.display_name,r.before_snapshot->>'summary',r.after_snapshot->>'summary',(r.before_snapshot->>'trust') || ' / ' || (r.before_snapshot->>'rapport'),(r.after_snapshot->>'trust') || ' / ' || (r.after_snapshot->>'rapport'),r.reason FROM agent_relationship_revisions r JOIN agent_relationships rel ON (rel.game_id,rel.id)=(r.game_id,r.relationship_id) JOIN agents a ON (a.game_id,a.id)=(rel.game_id,rel.from_agent_id) JOIN agents b ON (b.game_id,b.id)=(rel.game_id,rel.to_agent_id) WHERE r.game_id=$1 AND r.source_round_id=$2
    UNION ALL SELECT i.id::text,'culture_influence',t.title || ' · ' || a.display_name,NULL,i.explanation,NULL,i.category,'Influence on this discovery; does not imply a support vote or state change.' FROM tradition_influences i JOIN traditions t ON (t.game_id,t.id)=(i.game_id,i.tradition_id) JOIN agents a ON (a.game_id,a.id)=(i.game_id,i.agent_id) WHERE i.game_id=$1 AND i.source_round_id=$2 ORDER BY 2,3,1`, string(gameID), string(roundID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	changes := make([]SavedChange, 0)
	for rows.Next() {
		var change SavedChange
		if err = rows.Scan(&change.ID, &change.Kind, &change.Name, &change.Before, &change.After, &change.BeforeState, &change.AfterState, &change.Reason); err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	return changes, rows.Err()
}

// One batch read for the player's worlds; no per-card history queries.
func (s *Store) LatestChronicles(ctx context.Context, owner domain.ID) (map[domain.ID]*domain.GameChronicle, error) {
	rows, err := s.DB.Query(ctx, `SELECT DISTINCT ON (c.game_id) c.game_id::text,c.id::text,c.round_id::text,c.title,c.verdict,c.created_at FROM chronicles c JOIN games g ON g.id=c.game_id WHERE g.owner_account_id=$1 ORDER BY c.game_id,c.created_at DESC,c.id DESC`, string(owner))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[domain.ID]*domain.GameChronicle{}
	for rows.Next() {
		var c domain.GameChronicle
		var game, id, round string
		if err = rows.Scan(&game, &id, &round, &c.Title, &c.Verdict, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.ID, c.RoundID = domain.ID(id), domain.ID(round)
		gameID := domain.ID(game)
		result[gameID] = &c
	}
	return result, rows.Err()
}
