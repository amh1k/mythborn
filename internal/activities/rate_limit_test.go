package activities

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/models"
	"github.com/amh1k/mythborn/internal/workflow"
	"go.temporal.io/sdk/temporal"
)

type rateLimitedModel struct{}

func (rateLimitedModel) GenerateJSON(context.Context, string, []byte, string, any) error {
	return &models.RateLimitError{RetryAfter: 90 * time.Second}
}

func TestAgentRateLimitNeverBecomesAContribution(t *testing.T) {
	activity := Activities{Models: rateLimitedModel{}}
	for _, phase := range []string{"reaction", "rebuttal"} {
		result, err := activity.GenerateAgentPhase(context.Background(), AgentPhaseInput{Request: workflow.AgentRequest{RequestID: "request", GameID: "game", Phase: phase, Agent: workflow.AgentContext{Agent: domain.Agent{ID: "agent", GameID: "game"}}}})
		var failure *temporal.ApplicationError
		if !errors.As(err, &failure) || failure.Type() != workflow.ModelRateLimitErrorType || !failure.NonRetryable() {
			t.Fatalf("missing workflow scheduling error: %v", err)
		}
		var delay time.Duration
		if err := failure.Details(&delay); err != nil || delay != 90*time.Second {
			t.Fatalf("provider delay was lost: %v %v", delay, err)
		}
		if result.Reaction != nil || result.Rebuttal != nil || result.ErrorCode != "" {
			t.Fatal("rate limit converted into debate content")
		}
	}
	ordinary := errors.New("invalid model JSON")
	if modelActivityError(ordinary) != ordinary {
		t.Fatal("ordinary errors must retain bounded activity retries")
	}
}
