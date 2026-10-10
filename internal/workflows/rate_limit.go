package workflows

import (
	"errors"
	"fmt"
	"time"

	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/workflow"
	"go.temporal.io/sdk/temporal"
	tw "go.temporal.io/sdk/workflow"
)

func rateLimitDelay(err error) (time.Duration, bool) {
	var applicationError *temporal.ApplicationError
	if !errors.As(err, &applicationError) || applicationError.Type() != workflow.ModelRateLimitErrorType {
		return 0, false
	}
	delay := time.Minute
	var requested time.Duration
	if applicationError.HasDetails() && applicationError.Details(&requested) == nil {
		delay = max(delay, requested)
	}
	return delay, true
}

func clearModelRetry(preview *workflow.DebatePreview, agentID domain.ID, phase string) {
	if preview == nil {
		return
	}
	for i, retry := range preview.Retries {
		if retry.AgentID == agentID && retry.Phase == phase {
			preview.Retries = append(preview.Retries[:i], preview.Retries[i+1:]...)
			return
		}
	}
}

func waitForModelRetry(ctx tw.Context, preview *workflow.DebatePreview, agentID domain.ID, phase string, attempt int, delay time.Duration) error {
	clearModelRetry(preview, agentID, phase)
	if preview != nil {
		preview.Retries = append(preview.Retries, workflow.ModelRetry{AgentID: agentID, Phase: phase, RetryAt: tw.Now(ctx).Add(delay), Attempt: attempt})
	}
	// A Temporal timer releases the worker and survives server/worker restarts.
	err := tw.Sleep(ctx, delay)
	clearModelRetry(preview, agentID, phase)
	return err
}

func modelActivity(ctx tw.Context, command workflow.Command, preview *workflow.DebatePreview, agentID domain.ID, phase, name string, input, output any) error {
	version := tw.GetVersion(ctx, "model-rate-limit/"+string(command.EventID)+"/"+name, tw.DefaultVersion, 1)
	for attempt := 1; ; attempt++ {
		err := tw.ExecuteActivity(ctx, name, input).Get(ctx, output)
		delay, limited := rateLimitDelay(err)
		if version == tw.DefaultVersion || !limited {
			return err
		}
		if err := waitForModelRetry(ctx, preview, agentID, phase, attempt, delay); err != nil {
			return err
		}
	}
}

// Each agent waits independently. Successful siblings are never rerun when
// another agent hits a quota, and their contributions remain visible.
func retryAgentFuture(ctx tw.Context, first tw.Future, request workflow.AgentRequest, childID string, preview *workflow.DebatePreview) tw.Future {
	future, result := tw.NewFuture(ctx)
	tw.Go(ctx, func(ctx tw.Context) {
		pending := first
		for attempt := 1; ; attempt++ {
			var output workflow.AgentResult
			err := pending.Get(ctx, &output)
			delay, limited := rateLimitDelay(err)
			if !limited {
				result.Set(output, err)
				return
			}
			if err := waitForModelRetry(ctx, preview, request.Agent.Agent.ID, request.Phase, attempt, delay); err != nil {
				result.Set(nil, err)
				return
			}
			childCtx := tw.WithChildOptions(ctx, tw.ChildWorkflowOptions{WorkflowID: fmt.Sprintf("%s/rate-limit-retry/%d", childID, attempt)})
			pending = tw.ExecuteChildWorkflow(childCtx, workflow.AgentWorkflowName, request)
		}
	})
	return future
}
