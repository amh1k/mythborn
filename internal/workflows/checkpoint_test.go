package workflows

import (
	"fmt"
	"testing"

	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/workflow"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	tw "go.temporal.io/sdk/workflow"
)

func TestManualRetryOnlyRequestsMissingAgent(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	calls := map[domain.ID]int{}
	env.RegisterWorkflowWithOptions(func(_ tw.Context, req workflow.AgentRequest) (workflow.AgentResult, error) {
		id := req.Agent.Agent.ID
		calls[id]++
		if id == "fourth" && calls[id] == 1 {
			return workflow.AgentResult{}, temporal.NewNonRetryableApplicationError("invalid output", "invalid_output", nil)
		}
		return workflow.AgentResult{RequestID: req.RequestID, Reaction: &workflow.AgentReaction{AgentID: id, EventID: req.RequestID, Interpretation: "A visible stone"}}, nil
	}, tw.RegisterOptions{Name: workflow.AgentWorkflowName})
	env.ExecuteWorkflow(func(ctx tw.Context) error {
		current := workflow.RoundContext{}
		for _, id := range []domain.ID{"first", "second", "third", "fourth"} {
			current.Agents = append(current.Agents, workflow.AgentContext{Agent: domain.Agent{ID: id, GameID: "game"}})
		}
		command := workflow.Command{GameID: "game", RoundID: "round", EventID: "first-attempt"}
		saved := &RoundProgress{}
		saved.Preview = newDebatePreview(command, current, saved)
		first, _, err := runAgentPhase(ctx, command, current, "reaction", nil, nil, saved.Preview, saved)
		if err == nil || len(first) != 3 || len(saved.Reactions) != 3 {
			return fmt.Errorf("successful siblings were not checkpointed: %d/%d, %v", len(first), len(saved.Reactions), err)
		}
		command.EventID = "manual-retry"
		saved.Preview = newDebatePreview(command, current, saved)
		responses, _, err := runAgentPhase(ctx, command, current, "reaction", nil, nil, saved.Preview, saved)
		if err != nil || len(responses) != 4 {
			return fmt.Errorf("retry failed: %v", err)
		}
		return nil
	})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	for _, id := range []domain.ID{"first", "second", "third", "fourth"} {
		want := 1
		if id == "fourth" {
			want = 2
		}
		if calls[id] != want {
			t.Errorf("%s generated %d times, want %d", id, calls[id], want)
		}
	}
}

func TestCheckpointInvalidatesWhenEvidenceChanges(t *testing.T) {
	current := workflow.RoundContext{Observation: &workflow.ObservationContext{VisualDescription: "A stone"}}
	before := evidenceFingerprint(current)
	current.Observation.PlayerCorrection = "A seed, not a stone"
	if before == evidenceFingerprint(current) {
		t.Fatal("correction did not invalidate cached contributions")
	}
}
