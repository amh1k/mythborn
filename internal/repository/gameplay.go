package repository

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/amh1k/mythborn/internal/contracts"
	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/gameplay"
)

// Queries return typed JSON records, avoiding a second set of persistence DTOs.
func readJSON[T any](ctx context.Context, db interface {
	Query(context.Context, string, ...any) (contracts.Rows, error)
}, query string, args ...any) ([]T, error) {
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]T, 0)
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item T
		if err = json.Unmarshal(raw, &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *Store) ListMysteries(ctx context.Context, game domain.ID, state, cursor string) ([]gameplay.Mystery, error) {
	return readJSON[gameplay.Mystery](ctx, s.DB, `SELECT to_jsonb(m) FROM mysteries m WHERE game_id=$1 AND ($2='' OR state=$2) AND ($3='' OR (state,id)>(SELECT state,id FROM mysteries WHERE game_id=$1 AND id=NULLIF($3,'')::uuid)) ORDER BY state,id LIMIT 26`, string(game), state, cursor)
}
func (s *Store) Mystery(ctx context.Context, game, id domain.ID) (gameplay.Mystery, error) {
	var raw []byte
	err := s.DB.QueryRow(ctx, `SELECT to_jsonb(m) FROM mysteries m WHERE game_id=$1 AND id=$2`, string(game), string(id)).Scan(&raw)
	var m gameplay.Mystery
	if err != nil {
		return m, normalizeNotFound(err)
	}
	err = json.Unmarshal(raw, &m)
	return m, err
}
func (s *Store) Relationships(ctx context.Context, game, agent domain.ID) ([]gameplay.Relationship, error) {
	return readJSON[gameplay.Relationship](ctx, s.DB, `SELECT to_jsonb(r) FROM agent_relationships r WHERE game_id=$1 AND ($2='' OR from_agent_id=NULLIF($2,'')::uuid) ORDER BY from_agent_id,to_agent_id`, string(game), string(agent))
}
func (s *Store) GameplayRevisions(ctx context.Context, game, id domain.ID, kind string, after int64) ([]gameplay.Revision, error) {
	table, column := "mystery_revisions", "mystery_id"
	label := "kind"
	if kind == "relationship" {
		table, column = "agent_relationship_revisions", "relationship_id"
		label = "'relationship'"
	}
	return readJSON[gameplay.Revision](ctx, s.DB, `SELECT jsonb_build_object('id',id,'source_round_id',source_round_id,'version',version,'kind',`+label+`,'before',before_snapshot,'after',after_snapshot,'reason',reason,'created_at',created_at) FROM `+table+` WHERE game_id=$1 AND `+column+`=$2 AND version>$3 ORDER BY version LIMIT 26`, string(game), string(id), after)
}
func (s *Store) TraditionInfluences(ctx context.Context, game, tradition domain.ID, cursor string) ([]gameplay.Influence, error) {
	return readJSON[gameplay.Influence](ctx, s.DB, `SELECT to_jsonb(i)||jsonb_build_object('round_id',source_round_id,'title',t.title) FROM tradition_influences i JOIN traditions t ON (t.game_id,t.id)=(i.game_id,i.tradition_id) WHERE i.game_id=$1 AND i.tradition_id=$2 AND ($3='' OR (i.created_at,i.id)<(SELECT created_at,id FROM tradition_influences WHERE game_id=$1 AND id=NULLIF($3,'')::uuid)) ORDER BY i.created_at DESC,i.id DESC LIMIT 26`, string(game), string(tradition), cursor)
}
func (s *Store) LoadGameplayContext(ctx context.Context, game domain.ID, query string) (gameplay.Context, error) {
	out := gameplay.Context{ContractVersion: 1}
	if err := s.DB.QueryRow(ctx, `SELECT count(*) FROM mysteries WHERE game_id=$1 AND state='open'`, string(game)).Scan(&out.OpenMysteryCount); err != nil {
		return out, err
	}
	// OR-search retains older relevant questions even when the evidence has many unrelated words.
	search := gameplay.SearchQuery(query)
	var err error
	out.Mysteries, err = readJSON[gameplay.Mystery](ctx, s.DB, `SELECT to_jsonb(m) FROM mysteries m WHERE game_id=$1 AND (state='open' OR $2='' OR ($2<>'' AND to_tsvector('english',question||' '||evidence_summary) @@ websearch_to_tsquery('english',$2))) ORDER BY (state='open') DESC,ts_rank_cd(to_tsvector('english',question||' '||evidence_summary),websearch_to_tsquery('english',$2)) DESC,updated_at DESC,id LIMIT 12`, string(game), search)
	if err != nil {
		return out, err
	}
	out.Relationships, err = s.Relationships(ctx, game, "")
	return out, err
}
func initializeRelationships(ctx context.Context, tx contracts.Tx, game domain.ID) error {
	_, err := tx.Exec(ctx, `INSERT INTO agent_relationships(id,game_id,from_agent_id,to_agent_id) SELECT gen_random_uuid(),a.game_id,a.id,b.id FROM agents a JOIN agents b ON a.game_id=b.game_id AND a.id<>b.id WHERE a.game_id=$1 ON CONFLICT(game_id,from_agent_id,to_agent_id) DO NOTHING`, string(game))
	return err
}
func gameplayID(game, round domain.ID, key string) domain.ID {
	b := sha256.Sum256([]byte(string(game) + "/" + string(round) + "/" + key))
	b[6] = (b[6] & 15) | 80
	b[8] = (b[8] & 63) | 128
	return domain.ID(fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:16]))
}
func jsonValue(v any) string { b, _ := json.Marshal(v); return string(b) }

// Called only under CommitRound's game/round lock; no changes escape its transaction.
func applyGameplay(ctx context.Context, tx contracts.Tx, game, round domain.ID, kind string, changes gameplay.Changes) error {
	if len(changes.Mysteries)+len(changes.Relationships)+len(changes.Influences) == 0 {
		return nil
	}
	if kind == "closing" && (len(changes.Mysteries)+len(changes.Relationships)+len(changes.Influences) > 0) {
		return fmt.Errorf("closing round cannot mutate gameplay")
	}
	if kind != "discovery" && len(changes.Mysteries) > 0 {
		return fmt.Errorf("only discoveries update mysteries")
	}
	if len(changes.Mysteries) > 4 || len(changes.Relationships) > 8 || len(changes.Influences) > 4 {
		return fmt.Errorf("gameplay change budget exceeded")
	}
	current := gameplay.Context{}
	var err error
	current.Mysteries, err = readJSON[gameplay.Mystery](ctx, tx, `SELECT to_jsonb(m) FROM mysteries m WHERE game_id=$1`, string(game))
	if err != nil {
		return err
	}
	for _, m := range current.Mysteries {
		if m.State == "open" {
			current.OpenMysteryCount++
		}
	}
	current.Relationships, err = readJSON[gameplay.Relationship](ctx, tx, `SELECT to_jsonb(r) FROM agent_relationships r WHERE game_id=$1`, string(game))
	if err != nil {
		return err
	}
	ids, err := readJSON[domain.ID](ctx, tx, `SELECT to_jsonb(id) FROM agents WHERE game_id=$1`, string(game))
	if err != nil {
		return err
	}
	if err = gameplay.ValidateMysteries(changes.Mysteries, current, ids); err != nil {
		return err
	}
	for _, p := range changes.Mysteries {
		var before *gameplay.Mystery
		for _, m := range current.Mysteries {
			if m.ID == p.MysteryID {
				x := m
				before = &x
			}
		}
		after := gameplay.Mystery{ID: p.MysteryID, GameID: game, Title: p.Title, Question: p.Question, State: p.State, Explanations: p.Explanations, EvidenceSummary: p.EvidenceSummary, NextObservation: p.NextObservation, OriginRoundID: round, LatestRoundID: round, Version: 1}
		changeKind := "opened"
		if before == nil {
			after.ID = gameplayID(game, round, "mystery/"+gameplay.Normalize(p.Question))
		} else {
			after.OriginRoundID = before.OriginRoundID
			after.Version = before.Version + 1
			changeKind = "advanced"
			if before.State == "open" && after.State == "resolved" {
				changeKind = "resolved"
			}
			if before.State == "resolved" && after.State == "open" {
				changeKind = "reopened"
			}
			if before.Title == after.Title && before.State == after.State && reflect.DeepEqual(before.Explanations, after.Explanations) && before.EvidenceSummary == after.EvidenceSummary && before.NextObservation == after.NextObservation {
				continue
			}
		}
		if before == nil {
			_, err = tx.Exec(ctx, `INSERT INTO mysteries(id,game_id,title,question,state,explanations,evidence_summary,next_observation,origin_round_id,latest_round_id,version) VALUES($1,$2,$3,$4,$5,$6::jsonb,$7,$8,$9,$9,1)`, string(after.ID), string(game), after.Title, after.Question, after.State, jsonValue(after.Explanations), after.EvidenceSummary, after.NextObservation, string(round))
		} else {
			_, err = tx.Exec(ctx, `UPDATE mysteries SET title=$3,state=$4,explanations=$5::jsonb,evidence_summary=$6,next_observation=$7,latest_round_id=$8,version=$9,updated_at=now() WHERE game_id=$1 AND id=$2`, string(game), string(after.ID), after.Title, after.State, jsonValue(after.Explanations), after.EvidenceSummary, after.NextObservation, string(round), after.Version)
		}
		if err != nil {
			return err
		}
		var prev any
		if before != nil {
			prev = jsonValue(before)
		}
		_, err = tx.Exec(ctx, `INSERT INTO mystery_revisions(id,game_id,mystery_id,source_round_id,version,kind,before_snapshot,after_snapshot,reason) VALUES($1,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb,$9)`, string(gameplayID(game, round, "mystery-revision/"+string(after.ID))), string(game), string(after.ID), string(round), after.Version, changeKind, prev, jsonValue(after), p.Reason)
		if err != nil {
			return err
		}
	}
	grouped := map[domain.ID][]gameplay.RelationshipProposal{}
	for _, p := range changes.Relationships {
		grouped[p.FromAgentID] = append(grouped[p.FromAgentID], p)
	}
	for actor, proposals := range grouped {
		if err = gameplay.ValidateRelationships(proposals, current, actor); err != nil {
			return err
		}
	}
	seen := map[string]bool{}
	for _, p := range changes.Relationships {
		key := string(p.FromAgentID) + "/" + string(p.ToAgentID)
		if seen[key] {
			return fmt.Errorf("duplicate relationship revision")
		}
		seen[key] = true
		if err = gameplay.ValidateRelationships([]gameplay.RelationshipProposal{p}, current, p.FromAgentID); err != nil {
			return err
		}
		var before gameplay.Relationship
		for _, r := range current.Relationships {
			if r.FromAgentID == p.FromAgentID && r.ToAgentID == p.ToAgentID {
				before = r
			}
		}
		if before.Trust == p.Trust && before.Rapport == p.Rapport && before.Summary == p.Summary {
			continue
		}
		after := before
		after.Trust = p.Trust
		after.Rapport = p.Rapport
		after.Summary = p.Summary
		after.Version++
		after.LatestRoundID = &round
		_, err = tx.Exec(ctx, `UPDATE agent_relationships SET trust=$3,rapport=$4,summary=$5,version=$6,latest_round_id=$7,updated_at=now() WHERE game_id=$1 AND id=$2`, string(game), string(before.ID), after.Trust, after.Rapport, after.Summary, after.Version, string(round))
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO agent_relationship_revisions(id,game_id,relationship_id,source_round_id,version,before_snapshot,after_snapshot,reason) VALUES($1,$2,$3,$4,$5,$6::jsonb,$7::jsonb,$8)`, string(gameplayID(game, round, "relationship/"+key)), string(game), string(before.ID), string(round), after.Version, jsonValue(before), jsonValue(after), p.Reason)
		if err != nil {
			return err
		}
	}
	traditionIDs := make([]domain.ID, 0)
	for _, i := range changes.Influences {
		traditionIDs = append(traditionIDs, i.TraditionID)
	}
	if err = gameplay.ValidateInfluences(changes.Influences, traditionIDs, "", 4); err != nil {
		return err
	}
	for _, i := range changes.Influences {
		found := false
		for _, id := range ids {
			if id == i.AgentID {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("influence actor not in game")
		}
		// Snapshot supplied to agents is retained even if the same round revises this tradition.
		var version int64
		var state string
		if err = tx.QueryRow(ctx, `SELECT version,state FROM traditions WHERE game_id=$1 AND id=$2`, string(game), string(i.TraditionID)).Scan(&version, &state); err != nil {
			return err
		}
		if version != i.TraditionVersion || state != i.TraditionState {
			return fmt.Errorf("%w: tradition influence context changed", ErrStateConflict)
		}
		_, err = tx.Exec(ctx, `INSERT INTO tradition_influences(id,game_id,tradition_id,source_round_id,agent_id,tradition_version,tradition_state,category,explanation) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, string(gameplayID(game, round, "influence/"+string(i.TraditionID)+"/"+string(i.AgentID)+"/"+i.Category)), string(game), string(i.TraditionID), string(round), string(i.AgentID), i.TraditionVersion, i.TraditionState, i.Category, i.Explanation)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) TraditionRevisions(ctx context.Context, game, id domain.ID, after int64) ([]gameplay.TraditionRevision, error) {
	return readJSON[gameplay.TraditionRevision](ctx, s.DB, `SELECT to_jsonb(r) FROM tradition_revisions r WHERE game_id=$1 AND tradition_id=$2 AND version>$3 ORDER BY version LIMIT 26`, string(game), string(id), after)
}
