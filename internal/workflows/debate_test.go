package workflows

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/workflow"
	"go.temporal.io/sdk/testsuite"
	tw "go.temporal.io/sdk/workflow"
)

func TestDebatePublishesEachPhaseBeforeSlowAgentFinishes(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		name := "live"
		if legacy {
			name = "legacy-wait-order"
		}
		t.Run(name, func(t *testing.T) {
			var suite testsuite.WorkflowTestSuite
			env := suite.NewTestWorkflowEnvironment()
			env.RegisterWorkflowWithOptions(func(ctx tw.Context, request workflow.AgentRequest) (workflow.AgentResult, error) {
				delay := 10 * time.Second
				if request.Agent.Agent.ID == "fast" {
					delay = time.Second
				}
				if err := tw.Sleep(ctx, delay); err != nil {
					return workflow.AgentResult{}, err
				}
				result := workflow.AgentResult{RequestID: request.RequestID}
				if request.Phase == "reaction" {
					result.Reaction = &workflow.AgentReaction{EventID: request.RequestID, AgentID: request.Agent.Agent.ID, Interpretation: "A public interpretation", Reasoning: "Visible evidence"}
				} else {
					result.Rebuttal = &workflow.AgentRebuttal{EventID: request.RequestID, AgentID: request.Agent.Agent.ID, Response: "A response to the other voices"}
				}
				return result, nil
			}, tw.RegisterOptions{Name: workflow.AgentWorkflowName})
			if legacy {
				for _, phase := range []string{"reaction", "rebuttal"} {
					env.OnGetVersion("agent-phase-live-preview/event/"+phase, tw.DefaultVersion, 1).Return(tw.DefaultVersion)
				}
			}
			query := func(expected int) {
				value, err := env.QueryWorkflow(workflow.DebatePreviewQueryName, domain.ID("round"))
				if err != nil {
					t.Error(err)
					return
				}
				var preview *workflow.DebatePreview
				if err := value.Get(&preview); err != nil {
					t.Error(err)
					return
				}
				if preview == nil || len(preview.Entries) != expected {
					t.Errorf("expected %d live contributions, got %+v", expected, preview)
				}
				if !legacy && expected == 1 && preview != nil && preview.Entries[0].AgentID != "fast" {
					t.Error("slow first agent blocked the fast response")
				}
			}
			firstCount, secondCount := 1, 5
			if legacy {
				firstCount, secondCount = 0, 4
			}
			env.RegisterDelayedCallback(func() { query(firstCount) }, 1500*time.Millisecond)
			env.RegisterDelayedCallback(func() { query(secondCount) }, 11500*time.Millisecond)
			env.RegisterDelayedCallback(func() { query(8) }, 20500*time.Millisecond)
			env.RegisterDelayedCallback(func() {
				value, err := env.QueryWorkflow(workflow.DebatePreviewQueryName, domain.ID("round"))
				if err != nil {
					t.Error(err)
					return
				}
				var preview *workflow.DebatePreview
				_ = value.Get(&preview)
				if preview != nil {
					t.Error("completed preview was not cleared")
				}
			}, 22500*time.Millisecond)
			if legacy {
				env.RegisterDelayedCallback(func() {
					value, err := env.QueryWorkflow(workflow.DebatePreviewQueryName, domain.ID("next-round"))
					if err != nil {
						t.Error(err)
						return
					}
					var preview *workflow.DebatePreview
					if err := value.Get(&preview); err != nil {
						t.Error(err)
						return
					}
					if preview == nil || len(preview.Entries) != 1 || preview.Entries[0].AgentID != "fast" {
						t.Error("replayed legacy phase pinned a future command to the old wait order")
					}
				}, 24500*time.Millisecond)
			}
			env.ExecuteWorkflow(func(ctx tw.Context) error {
				command := workflow.Command{GameID: "game", RoundID: "round", EventID: "event"}
				current := workflow.RoundContext{Agents: []workflow.AgentContext{}}
				for _, id := range []domain.ID{"slow", "fast", "third", "fourth"} {
					current.Agents = append(current.Agents, workflow.AgentContext{Agent: domain.Agent{ID: id, GameID: "game", Type: domain.AgentTypePriest, DisplayName: string(id)}})
				}
				progress := &RoundProgress{}
				progress.Preview = newDebatePreview(command, current, progress)
				byRound := map[domain.ID]*RoundProgress{"round": progress}
				if err := registerDebateQuery(ctx, byRound); err != nil {
					return err
				}
				var err error
				progress.Reactions, _, err = runAgentPhase(ctx, command, current, "reaction", nil, nil, progress.Preview)
				if err != nil {
					return err
				}
				progress.Preview.Phase = "rebuttal"
				_, progress.Rebuttals, err = runAgentPhase(ctx, command, current, "rebuttal", progress.Reactions, nil, progress.Preview)
				if err != nil {
					return err
				}
				if len(progress.Reactions) != 4 || len(progress.Rebuttals) != 4 {
					t.Error("historian inputs lost a contribution")
				}
				for i := 1; i < len(progress.Reactions); i++ {
					if progress.Reactions[i-1].AgentID > progress.Reactions[i].AgentID {
						t.Error("historian input order changed")
					}
				}
				progress.Preview.Phase = "writing"
				if err := tw.Sleep(ctx, 2*time.Second); err != nil {
					return err
				}
				delete(byRound, "round")
				if err := tw.Sleep(ctx, time.Second); err != nil {
					return err
				}
				if legacy {
					command.RoundID, command.EventID = "next-round", "next-event"
					next := &RoundProgress{}
					next.Preview = newDebatePreview(command, current, next)
					byRound[command.RoundID] = next
					_, _, err = runAgentPhase(ctx, command, current, "reaction", nil, nil, next.Preview)
					return err
				}
				return nil
			})
			if err := env.GetWorkflowError(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPreviewIsNotCarriedIntoContinueAsNew(t *testing.T) {
	data, err := json.Marshal(RoundProgress{Preview: &workflow.DebatePreview{Entries: []workflow.DebateEntry{{Text: "transient debate"}}}})
	if err != nil {
		t.Fatal(err)
	}
	var carried RoundProgress
	if err := json.Unmarshal(data, &carried); err != nil {
		t.Fatal(err)
	}
	if carried.Preview != nil {
		t.Fatal("live preview was included in carried state")
	}
}
