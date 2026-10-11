package workflows

import (
	"context"
	"testing"
	"time"

	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/gameplay"
	"github.com/amh1k/mythborn/internal/workflow"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	tw "go.temporal.io/sdk/workflow"
)

func TestDeletionInterruptsExtendedQuotaWait(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivityWithOptions(func(context.Context, workflow.RoundContext) (gameplay.Context, error) { return gameplay.Context{}, nil }, activity.RegisterOptions{Name: "LoadGameplay"})
	current := workflow.RoundContext{Game: domain.Game{ID: "game"}, Round: domain.Round{ID: "round", GameID: "game", Kind: domain.RoundKindDiscovery, Status: domain.RoundStatusRunning, Stage: domain.RoundStageReacting}, Observation: &workflow.ObservationContext{DescriptionStatus: "accepted"}}
	for _, id := range []domain.ID{"first", "second", "third", "fourth"} {
		current.Agents = append(current.Agents, workflow.AgentContext{Agent: domain.Agent{ID: id, GameID: "game"}})
	}
	env.RegisterActivityWithOptions(func(context.Context, domain.ID, domain.ID) (workflow.RoundContext, error) { return current, nil }, activity.RegisterOptions{Name: activityLoadRound})
	env.RegisterActivityWithOptions(func(context.Context, domain.ID, domain.ID, string) error { return nil }, activity.RegisterOptions{Name: activityPauseRound})
	deleted := false
	env.RegisterActivityWithOptions(func(context.Context, struct {
		GameID    domain.ID `json:"game_id"`
		CommandID domain.ID `json:"command_id"`
	}) error {
		deleted = true
		return nil
	}, activity.RegisterOptions{Name: activityDeleteGame})
	env.RegisterWorkflowWithOptions(func(_ tw.Context, req workflow.AgentRequest) (workflow.AgentResult, error) {
		if req.Agent.Agent.ID == "fourth" {
			return workflow.AgentResult{}, temporal.NewNonRetryableApplicationError("quota", workflow.ModelRateLimitErrorType, nil, time.Minute)
		}
		return workflow.AgentResult{Reaction: &workflow.AgentReaction{AgentID: req.Agent.Agent.ID, EventID: req.RequestID, Interpretation: "Actual response"}}, nil
	}, tw.RegisterOptions{Name: workflow.AgentWorkflowName})
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(workflow.CoordinatorSignalName, workflow.Command{EventID: "discover", GameID: "game", RoundID: "round", Type: workflow.CommandProcessRound})
	}, 0)
	env.RegisterDelayedCallback(func() {
		value, err := env.QueryWorkflow(workflow.DebatePreviewQueryName, domain.ID("round"))
		if err != nil {
			t.Error(err)
		} else {
			var preview *workflow.DebatePreview
			_ = value.Get(&preview)
			if preview == nil || len(preview.Entries) != 3 {
				t.Error("successful contributions lost during quota outage")
			}
		}
		command := workflow.Command{EventID: "delete", GameID: "game", Type: workflow.CommandDeleteGame}
		env.SignalWorkflow(workflow.CoordinatorSignalName, command)
		env.SignalWorkflow(workflow.CoordinatorInterruptSignalName, command)
	}, 5*time.Second)
	env.ExecuteWorkflow(Coordinator, domain.ID("game"), map[domain.ID]*RoundProgress{})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	if !deleted {
		t.Fatal("deletion stayed queued behind quota wait")
	}
}

func TestExtendedQuotaHasFiniteAttemptAndTimeBudget(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	calls := 0
	env.RegisterActivityWithOptions(func(context.Context, string) error {
		calls++
		return temporal.NewNonRetryableApplicationError("quota", workflow.ModelRateLimitErrorType, nil, time.Minute)
	}, activity.RegisterOptions{Name: activityDescribePhoto})
	start := env.Now()
	env.ExecuteWorkflow(func(ctx tw.Context) error {
		return modelActivity(activityContext(ctx), workflow.Command{EventID: "quota"}, nil, "", "describing", activityDescribePhoto, "input", nil)
	})
	if env.GetWorkflowError() == nil || calls != maxQuotaAttempts || env.Now().Sub(start) > maxQuotaWindow {
		t.Fatalf("unbounded quota recovery: calls=%d duration=%s error=%v", calls, env.Now().Sub(start), env.GetWorkflowError())
	}
}
