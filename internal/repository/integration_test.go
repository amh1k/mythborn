package repository

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/amh1k/mythborn/internal/database"
	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/workflow"
)

// Run only against a disposable migrated database, never DATABASE_URL.
func TestPostgresBeliefSourcesAndAbandonment(t *testing.T) {
	url := os.Getenv("MYTHBORN_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set MYTHBORN_TEST_DATABASE_URL to a migrated disposable database")
	}
	ctx := context.Background()
	db, err := database.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := New(db)
	account, _ := NewID()
	game, _ := NewID()
	round, _ := NewID()
	agent, _ := NewID()
	belief, _ := NewID()
	event, _ := NewID()
	fixture,err:=db.BeginTx(ctx);if err!=nil {t.Fatal(err)}
	defer func(){_=fixture.Rollback(ctx)}()
	initializing:=true
	exec := func(query string, args ...any) {
		t.Helper()
		var err error
		if initializing { _,err=fixture.Exec(ctx,query,args...) } else { _,err=db.Exec(ctx,query,args...) }
		if err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO accounts(id) VALUES($1)`, string(account))
	defer func() { _, _ = db.Exec(ctx, `DELETE FROM accounts WHERE id=$1`, string(account)) }()
	exec(`INSERT INTO games(id,owner_account_id,name,player_role,status) VALUES($1,$2,'Test only','observer','starting')`, string(game), string(account))
	exec(`INSERT INTO agents(id,game_id,agent_type,display_name,personality_prompt) VALUES($1,$2,'scientist','A test scientist','Test')`, string(agent), string(game))
	for _, kind := range []string{"priest", "soldier", "historian"} {
		id,_:=NewID()
		exec(`INSERT INTO agents(id,game_id,agent_type,display_name,personality_prompt) VALUES($1,$2,$3,$3,'Test')`,string(id),string(game),kind)
	}
	exec(`UPDATE games SET status='active' WHERE id=$1`,string(game))
	if err=fixture.Commit(ctx);err!=nil {t.Fatal(err)}
	initializing=false
	exec(`INSERT INTO rounds(id,game_id,sequence_number,kind,status,stage,idempotency_key) VALUES($1,$2,1,'council','running','reacting','test-council')`, string(round), string(game))
	tx, err := db.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	proposal := workflow.BeliefProposal{NewBeliefID: belief, AgentID: agent, ChangeType: domain.BeliefChangeFormed, NewClaim: "The evidence remains uncertain.", NewState: domain.BeliefStateForming, Reason: "One observation is insufficient."}
	if err = applyBeliefProposal(ctx, tx, game, round, event, "council", nil, proposal); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	changes, err := store.ListRoundChanges(ctx, game, round)
	if err != nil || len(changes) != 1 || changes[0].Before != nil {
		t.Fatalf("committed revision missing: %+v %v", changes, err)
	}
	// A non-initial revision must still have a real source.
	invalidID, _ := NewID()
	if _, err = db.Exec(ctx, `INSERT INTO belief_revisions(id,game_id,belief_id,version,event_id,cause,change_type,new_claim,new_state,reason) VALUES($1,$2,$3,1,$4,'council','formed','x','forming','x')`, string(invalidID), string(game), string(belief), string(event)); err == nil {
		t.Fatal("unattributed council revision accepted")
	}
	command, err := store.RequestRoundAbandon(ctx, account, game, round, "abandon-test-key")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.AbandonRound(ctx, game, round, command); err != nil {
		t.Fatal(err)
	}
	if err = store.SetRoundStage(ctx, game, round, domain.RoundStatusRunning, domain.RoundStageWriting); !errors.Is(err, ErrNotFound) {
		t.Fatalf("late activity resurrected abandoned round: %v", err)
	}
	current, err := store.GetRound(ctx, round)
	if err != nil || current.Status != domain.RoundStatusAbandoned {
		t.Fatalf("abandon failed: %+v %v", current, err)
	}
	var pending int
	if err = db.QueryRow(ctx, `SELECT count(*) FROM workflow_outbox WHERE round_id=$1 AND status <> 'processed'`, string(round)).Scan(&pending); err != nil || pending != 0 {
		t.Fatalf("abandon command not settled: %d %v", pending, err)
	}
	// A quota-paused closing round remains retryable while ending the game.
	closing, _ := NewID()
	exec(`UPDATE games SET status='ending' WHERE id=$1`, string(game))
	exec(`INSERT INTO rounds(id,game_id,sequence_number,kind,status,stage,idempotency_key) VALUES($1,$2,2,'closing','needs_attention','writing','test-closing')`, string(closing), string(game))
	if _, err = store.RetryRound(ctx, account, game, closing, "closing-retry-test"); err != nil {
		t.Fatal(err)
	}
}
