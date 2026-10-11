package workflows

import (
	"context"
	"fmt"
	"testing"

	"github.com/amh1k/mythborn/internal/activities"
	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/gameplay"
	"github.com/amh1k/mythborn/internal/workflow"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	tw "go.temporal.io/sdk/workflow"
)

func TestGameplayContextFrozenAcrossManualRetryAndLegacyPath(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		name := "continuity"
		if legacy {
			name = "legacy"
		}
		t.Run(name, func(t *testing.T) {
			var suite testsuite.WorkflowTestSuite
			env := suite.NewTestWorkflowEnvironment()
			loads := 0
			calls := map[domain.ID]int{}
			commits := 0
			current := workflow.RoundContext{Game: domain.Game{ID: "game"}, Round: domain.Round{ID: "round", GameID: "game", Kind: domain.RoundKindDiscovery, Status: domain.RoundStatusRunning, Stage: domain.RoundStageReacting}, Observation: &workflow.ObservationContext{DescriptionStatus: "accepted", VisualDescription: "A mossy stone"}, HistorianAgentID: "fourth"}
			for _, id := range []domain.ID{"first", "second", "third", "fourth"} {
				current.Agents = append(current.Agents, workflow.AgentContext{Agent: domain.Agent{ID: id, GameID: "game"}})
			}
			env.RegisterActivityWithOptions(func(context.Context, domain.ID, domain.ID) (workflow.RoundContext, error) { return current, nil }, activity.RegisterOptions{Name: activityLoadRound})
			env.RegisterActivityWithOptions(func(context.Context, workflow.RoundContext) (gameplay.Context, error) {
				loads++
				return gameplay.Context{ContractVersion: 1, Features: gameplay.Enabled(), Mysteries: []gameplay.Mystery{{ID: "question", Version: 7}}}, nil
			}, activity.RegisterOptions{Name: "LoadGameplay"})
			env.RegisterActivityWithOptions(func(context.Context, domain.ID, domain.ID, domain.RoundStatus, domain.RoundStage) error { return nil }, activity.RegisterOptions{Name: activitySetStage})
			env.RegisterActivityWithOptions(func(_ context.Context, in activitiesHistorianInput) (workflow.HistorianRecord, error) {
				if !legacy && (in.Context.Gameplay == nil || in.Context.Gameplay.Mysteries[0].Version != 7) {
					return workflow.HistorianRecord{}, fmt.Errorf("historian lost frozen context")
				}
				return workflow.HistorianRecord{Title: "Stone", Body: "An open question", Verdict: "Uncertain", Outcome: domain.OutcomeUnresolved}, nil
			}, activity.RegisterOptions{Name: activityHistorian})
			env.RegisterActivityWithOptions(func(context.Context, activities.RoundCommitInput) error { commits++; return nil }, activity.RegisterOptions{Name: activityCommitRound})
			env.RegisterWorkflowWithOptions(func(_ tw.Context, req workflow.AgentRequest) (workflow.AgentResult, error) {
				if !legacy && (req.Gameplay == nil || req.Gameplay.Mysteries[0].Version != 7) {
					return workflow.AgentResult{}, fmt.Errorf("agent lost frozen context")
				}
				if legacy && req.Gameplay != nil {
					return workflow.AgentResult{}, fmt.Errorf("legacy request changed contract")
				}
				if req.Phase == "reaction" {
					calls[req.Agent.Agent.ID]++
					if req.Agent.Agent.ID == "fourth" && calls["fourth"] == 1 {
						return workflow.AgentResult{}, temporal.NewNonRetryableApplicationError("invalid response", "invalid_output", nil)
					}
					return workflow.AgentResult{Reaction: &workflow.AgentReaction{AgentID: req.Agent.Agent.ID, Interpretation: "Stone"}}, nil
				}
				return workflow.AgentResult{Rebuttal: &workflow.AgentRebuttal{AgentID: req.Agent.Agent.ID, Response: "Uncertain"}}, nil
			}, tw.RegisterOptions{Name: workflow.AgentWorkflowName})
			if legacy {
				env.OnGetVersion("gameplay-continuity/round", tw.DefaultVersion, 1).Return(tw.DefaultVersion)
			}
			env.ExecuteWorkflow(func(ctx tw.Context) error {
				progress := map[domain.ID]*RoundProgress{}
				command := workflow.Command{GameID: "game", RoundID: "round", EventID: "first-attempt"}
				if err := processRound(activityContext(ctx), command, progress); err == nil {
					return fmt.Errorf("first attempt should pause")
				}
				command.EventID = "retry"
				return processRound(activityContext(ctx), command, progress)
			})
			if err := env.GetWorkflowError(); err != nil {
				t.Fatal(err)
			}
			want := 1
			if legacy {
				want = 0
			}
			if loads != want || commits != 1 {
				t.Fatalf("context loads/commits %d/%d, want %d/1", loads, commits, want)
			}
			for id, count := range calls {
				want := 1
				if id == "fourth" {
					want = 2
				}
				if count != want {
					t.Fatalf("repeated successful reaction %s: %d", id, count)
				}
			}
		})
	}
}
