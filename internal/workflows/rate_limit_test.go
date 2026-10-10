package workflows

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/amh1k/mythborn/internal/activities"
	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/workflow"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	tw "go.temporal.io/sdk/workflow"
)

func TestRoundResumesRateLimitedCallsWithoutRepeatingSuccessfulAgents(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflowWithOptions(Agent, tw.RegisterOptions{Name: workflow.AgentWorkflowName})
	current := workflow.RoundContext{
		Game:             domain.Game{ID: "game"},
		Round:            domain.Round{ID: "round", GameID: "game", Kind: domain.RoundKindDiscovery, Status: domain.RoundStatusRunning, Stage: domain.RoundStageQueued},
		Observation:      &workflow.ObservationContext{ID: "photo", DescriptionStatus: "pending"},
		HistorianAgentID: "historian",
	}
	for _, id := range []domain.ID{"priest", "scientist", "soldier", "historian"} {
		current.Agents = append(current.Agents, workflow.AgentContext{Agent: domain.Agent{ID: id, GameID: "game"}})
	}
	var mu sync.Mutex
	calls := map[string]int{}
	count := func(key string) int {
		mu.Lock()
		defer mu.Unlock()
		calls[key]++
		return calls[key]
	}
	limited := func(delay time.Duration) error {
		return temporal.NewNonRetryableApplicationError("model rate limit reached", workflow.ModelRateLimitErrorType, nil, delay)
	}
	env.RegisterActivityWithOptions(func(context.Context, domain.ID, domain.ID) (workflow.RoundContext, error) {
		return current, nil
	}, activity.RegisterOptions{Name: activityLoadRound})
	env.RegisterActivityWithOptions(func(_ context.Context, _, _ domain.ID, status domain.RoundStatus, _ domain.RoundStage) error {
		if status == domain.RoundStatusNeedsAttention || status == domain.RoundStatusFailed {
			t.Error("rate limit stopped the discovery")
		}
		return nil
	}, activity.RegisterOptions{Name: activitySetStage})
	env.RegisterActivityWithOptions(func(context.Context, activities.DescriptionInput) (activitiesDescriptionResult, error) {
		if count("description") == 1 {
			return activitiesDescriptionResult{}, limited(time.Minute)
		}
		return activitiesDescriptionResult{Description: "A visible stone", Usable: true}, nil
	}, activity.RegisterOptions{Name: activityDescribePhoto})
	env.RegisterActivityWithOptions(func(_ context.Context, input activities.AgentPhaseInput) (workflow.AgentResult, error) {
		req := input.Request
		n := count(string(req.Agent.Agent.ID) + "/" + req.Phase)
		if req.Agent.Agent.ID == "priest" && ((req.Phase == "reaction" && n <= 2) || (req.Phase == "rebuttal" && n == 1)) {
			return workflow.AgentResult{}, limited(time.Minute)
		}
		result := workflow.AgentResult{RequestID: req.RequestID}
		if req.Phase == "reaction" {
			result.Reaction = &workflow.AgentReaction{EventID: req.RequestID, AgentID: req.Agent.Agent.ID, Interpretation: "A real interpretation"}
		} else {
			if len(req.Reactions) != 4 {
				t.Error("rebuttal began before all four reactions completed")
			}
			result.Rebuttal = &workflow.AgentRebuttal{EventID: req.RequestID, AgentID: req.Agent.Agent.ID, Response: "A real rebuttal"}
		}
		return result, nil
	}, activity.RegisterOptions{Name: activityGenerateAgent})
	env.RegisterActivityWithOptions(func(_ context.Context, input activitiesHistorianInput) (workflow.HistorianRecord, error) {
		if len(input.Reactions) != 4 || len(input.Rebuttals) != 4 {
			t.Error("historian wrote with missing agent responses")
		}
		if count("historian") == 1 {
			return workflow.HistorianRecord{}, limited(90 * time.Second)
		}
		return workflow.HistorianRecord{Title: "Complete discovery", Outcome: domain.OutcomeConsensus, Verdict: "Full debate", Body: "Only real contributions"}, nil
	}, activity.RegisterOptions{Name: activityHistorian})
	env.RegisterActivityWithOptions(func(_ context.Context, input activities.RoundCommitInput) error {
		count("commit")
		if input.Commit.Chronicle.Outcome == nil || *input.Commit.Chronicle.Outcome != domain.OutcomeConsensus {
			t.Error("rate limit recorded as an unresolved chronicle")
		}
		return nil
	}, activity.RegisterOptions{Name: activityCommitRound})
	start := env.Now()
	for _, check := range []struct {
		after            time.Duration
		phase            string
		entries, attempt int
		agent            domain.ID
		retryAt          time.Duration
	}{
		{10 * time.Second, "describing", 0, 1, "", time.Minute},
		{70 * time.Second, "reaction", 3, 1, "priest", 2 * time.Minute},
		{130 * time.Second, "reaction", 3, 2, "priest", 3 * time.Minute},
		{190 * time.Second, "rebuttal", 7, 1, "priest", 4 * time.Minute},
		{250 * time.Second, "writing", 8, 1, "historian", 330 * time.Second},
	} {
		env.RegisterDelayedCallback(func() {
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
			if preview == nil || preview.Phase != check.phase || len(preview.Entries) != check.entries || len(preview.Retries) != 1 {
				t.Errorf("incorrect progress while waiting at %v: %+v", check.after, preview)
				return
			}
			retry := preview.Retries[0]
			if retry.AgentID != check.agent || retry.Attempt != check.attempt || !retry.RetryAt.Equal(start.Add(check.retryAt)) {
				t.Errorf("wrong retry schedule: %+v", retry)
			}
			mu.Lock()
			defer mu.Unlock()
			if calls["commit"] != 0 {
				t.Error("chronicle committed while a model call was still rate limited")
			}
		}, check.after)
	}
	env.ExecuteWorkflow(func(ctx tw.Context) error {
		progress := map[domain.ID]*RoundProgress{}
		if err := registerDebateQuery(ctx, progress); err != nil {
			return err
		}
		if err := processRound(activityContext(ctx), workflow.Command{GameID: "game", RoundID: "round", EventID: "event"}, progress); err != nil {
			return err
		}
		if len(progress) != 0 {
			return fmt.Errorf("completed round retained transient retry state")
		}
		return nil
	})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	if env.Now().Sub(start) != 330*time.Second {
		t.Fatalf("retry timers were skipped or duplicated: %v", env.Now().Sub(start))
	}
	for key, want := range map[string]int{"description": 2, "historian": 2, "commit": 1, "priest/reaction": 3, "priest/rebuttal": 2, "scientist/reaction": 1, "scientist/rebuttal": 1, "soldier/reaction": 1, "soldier/rebuttal": 1, "historian/reaction": 1, "historian/rebuttal": 1} {
		if calls[key] != want {
			t.Errorf("%s called %d times, want %d", key, calls[key], want)
		}
	}
}

func TestOrdinaryModelFailureKeepsBoundedRetries(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	calls := 0
	env.RegisterActivityWithOptions(func(context.Context, string) error {
		calls++
		return fmt.Errorf("invalid model JSON")
	}, activity.RegisterOptions{Name: activityDescribePhoto})
	env.ExecuteWorkflow(func(ctx tw.Context) error {
		return modelActivity(activityContext(ctx), workflow.Command{EventID: "ordinary-error"}, nil, "", "describing", activityDescribePhoto, "input", nil)
	})
	if env.GetWorkflowError() == nil || calls != 5 {
		t.Fatalf("ordinary failure escaped the five-attempt limit: calls=%d, err=%v", calls, env.GetWorkflowError())
	}
}
