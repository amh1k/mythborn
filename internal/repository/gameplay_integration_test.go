package repository

import (
	"context"
	"os"
	"testing"

	"github.com/amh1k/mythborn/internal/database"
	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/gameplay"
	"github.com/amh1k/mythborn/internal/workflow"
)

func TestPostgresGameplayContinuityAtomicityAndReopening(t *testing.T) {
	url := os.Getenv("MYTHBORN_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requires disposable migrated database")
	}
	ctx := context.Background()
	db, err := database.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := New(db)
	account, _ := NewID()
	game, _ := NewID()
	agents := map[string]domain.ID{}
	for _, kind := range []string{"priest", "scientist", "soldier", "historian"} {
		id, _ := NewID()
		agents[kind] = id
	}
	tx, err := db.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO accounts(id) VALUES($1)`, string(account))
	exec(`INSERT INTO games(id,owner_account_id,name,player_role,status) VALUES($1,$2,'Continuity test','observer','starting')`, string(game), string(account))
	for kind, id := range agents {
		exec(`INSERT INTO agents(id,game_id,agent_type,display_name,personality_prompt) VALUES($1,$2,$3,$3,'Test')`, string(id), string(game), kind)
	}
	exec(`UPDATE games SET status='active' WHERE id=$1`, string(game))
	if err = initializeRelationships(ctx, tx, game); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM games WHERE id=$1`, string(game))
		_, _ = db.Exec(ctx, `DELETE FROM accounts WHERE id=$1`, string(account))
	}()
	rels, err := s.Relationships(ctx, game, agents["soldier"])
	if err != nil || len(rels) != 3 {
		t.Fatalf("neutral pairs: %+v %v", rels, err)
	}
	var rel gameplay.Relationship
	for _, r := range rels {
		if r.ToAgentID == agents["scientist"] {
			rel = r
		}
	}
	tradition, _ := NewID()
	if _, err = db.Exec(ctx, `INSERT INTO traditions(id,game_id,type,title,description,state) VALUES($1,$2,'ritual','Marked stones','Compare moss and moisture','adopted')`, string(tradition), string(game)); err != nil {
		t.Fatal(err)
	}
	sequence := 0
	round := func() domain.ID {
		t.Helper()
		sequence++
		id, _ := NewID()
		photo, _ := NewID()
		fixture, e := db.BeginTx(ctx)
		if e != nil {
			t.Fatal(e)
		}
		defer fixture.Rollback(ctx)
		_, e = fixture.Exec(ctx, `INSERT INTO observations(id,game_id,photo_object_path,mime_type,byte_size,visual_description,description_status) VALUES($1,$2,$3,'image/jpeg',100,'A moss-covered stone','accepted')`, string(photo), string(game), "test/"+string(photo))
		if e != nil {
			t.Fatal(e)
		}
		_, e = fixture.Exec(ctx, `INSERT INTO rounds(id,game_id,sequence_number,kind,observation_id,status,stage,idempotency_key) VALUES($1,$2,$3,'discovery',$4,'running','writing',$5)`, string(id), string(game), sequence, string(photo), string(id))
		if e != nil {
			t.Fatal(e)
		}
		if e = fixture.Commit(ctx); e != nil {
			t.Fatal(e)
		}
		return id
	}
	makeCommit := func(id domain.ID, changes gameplay.Changes) workflow.RoundCommit {
		chronicle, _ := NewID()
		event, _ := NewID()
		outcome := domain.OutcomeUnresolved
		return workflow.RoundCommit{EventID: event, ProcessedCommandID: event, Chronicle: domain.Chronicle{ID: chronicle, GameID: game, RoundID: id, HistorianAgentID: agents["historian"], Title: "Marked stone", Verdict: "Still uncertain", Body: "Visible moss leaves a question.", Outcome: &outcome}, Gameplay: changes}
	}
	p := gameplay.MysteryProposal{Title: "Marked stones", Question: "Does moss follow moisture?", State: "open", Explanations: []gameplay.Explanation{{Key: "moisture", Claim: "Moss follows damp conditions.", AgentIDs: []domain.ID{agents["scientist"]}}}, EvidenceSummary: "A damp stone carries moss.", Reason: "The observed pattern needs a comparison."}
	r1 := round()
	commit := makeCommit(r1, gameplay.Changes{Mysteries: []gameplay.MysteryProposal{p}, Relationships: []gameplay.RelationshipProposal{{FromAgentID: agents["soldier"], ToAgentID: agents["scientist"], ExpectedVersion: rel.Version, Trust: "trusting", Rapport: "neutral", Summary: "Respects a grounded comparison.", Reason: "The Scientist separated the observation from the claim."}}, Influences: []gameplay.Influence{{TraditionID: tradition, AgentID: agents["soldier"], Category: "challenge", Explanation: "The moss challenges the old practice.", TraditionVersion: 1, TraditionState: "adopted"}}})
	if err = s.CommitRound(ctx, commit); err != nil {
		t.Fatal(err)
	}
	if err = s.CommitRound(ctx, commit); err != nil {
		t.Fatal("repeat commit", err)
	}
	mysteries, err := s.ListMysteries(ctx, game, "", "")
	if err != nil || len(mysteries) != 1 {
		t.Fatalf("mystery commit: %+v %v", mysteries, err)
	}
	m := mysteries[0]
	revisions, err := s.GameplayRevisions(ctx, game, m.ID, "mystery", 0)
	if err != nil || len(revisions) != 1 {
		t.Fatalf("duplicate mystery revision: %+v %v", revisions, err)
	}
	reverse, err := s.Relationships(ctx, game, agents["scientist"])
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range reverse {
		if r.ToAgentID == agents["soldier"] && r.Trust != "neutral" {
			t.Fatal("relationship wrongly symmetric")
		}
	}
	changes, err := s.ListRoundChanges(ctx, game, r1)
	if err != nil || len(changes) != 3 {
		t.Fatalf("saved consequences: %+v %v", changes, err)
	}
	influences, err := s.TraditionInfluences(ctx, game, tradition, "")
	if err != nil || len(influences) != 1 || influences[0].TraditionVersion != 1 {
		t.Fatalf("culture influence missing: %+v %v", influences, err)
	}
	// A stale relationship following a valid mystery proposal rolls back the entire round.
	r2 := round()
	p.MysteryID = m.ID
	p.ExpectedVersion = 1
	p.EvidenceSummary = "A second stone differs."
	bad := makeCommit(r2, gameplay.Changes{Mysteries: []gameplay.MysteryProposal{p}, Relationships: commit.Gameplay.Relationships})
	if s.CommitRound(ctx, bad) == nil {
		t.Fatal("stale relationship overwrite accepted")
	}
	unchanged, err := s.Mystery(ctx, game, m.ID)
	if err != nil || unchanged.Version != 1 {
		t.Fatalf("partial mystery change escaped rollback: %+v %v", unchanged, err)
	}
	var count int
	if err = db.QueryRow(ctx, `SELECT count(*) FROM chronicles WHERE round_id=$1`, string(r2)).Scan(&count); err != nil || count != 0 {
		t.Fatal("partial chronicle committed", err)
	}
	p.State = "resolved"
	if err = s.CommitRound(ctx, makeCommit(r2, gameplay.Changes{Mysteries: []gameplay.MysteryProposal{p}})); err != nil {
		t.Fatal(err)
	}
	r3 := round()
	p.ExpectedVersion = 2
	p.State = "open"
	p.EvidenceSummary = "A contradiction reopens the comparison."
	if err = s.CommitRound(ctx, makeCommit(r3, gameplay.Changes{Mysteries: []gameplay.MysteryProposal{p}})); err != nil {
		t.Fatal(err)
	}
	revisions, err = s.GameplayRevisions(ctx, game, m.ID, "mystery", 0)
	if err != nil || len(revisions) != 3 || revisions[2].Kind != "reopened" {
		t.Fatalf("resolution/reopening: %+v %v", revisions, err)
	}
	// An unrelated discovery does not require changes to any ledger.
	r4 := round()
	if err = s.CommitRound(ctx, makeCommit(r4, gameplay.Changes{})); err != nil {
		t.Fatal(err)
	}
	current, err := s.Mystery(ctx, game, m.ID)
	if err != nil || current.Version != 3 {
		t.Fatal("unrelated round changed mystery", err)
	}
	// SQL constraints independently reject a cross-game agent reference.
	foreign, _ := NewID()
	if _, err = db.Exec(ctx, `INSERT INTO agent_relationships(id,game_id,from_agent_id,to_agent_id) VALUES(gen_random_uuid(),$1,$2,$3)`, string(game), string(agents["soldier"]), string(foreign)); err == nil {
		t.Fatal("foreign relationship accepted")
	}
	if _, err = db.Exec(ctx, `DELETE FROM games WHERE id=$1`, string(game)); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(ctx, `SELECT count(*) FROM mysteries WHERE game_id=$1`, string(game)).Scan(&count); err != nil || count != 0 {
		t.Fatal("gameplay not deleted with game", err)
	}
}
