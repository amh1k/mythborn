package activities

import (
	"github.com/amh1k/mythborn/internal/models"
	"github.com/amh1k/mythborn/internal/workflow"
	"go.temporal.io/sdk/temporal"
)

func modelActivityError(err error) error {
	if delay, limited := models.RateLimitDelay(err); limited {
		// Return immediately to the workflow. Its durable timer handles 429s
		// separately from the bounded retries for ordinary activity failures.
		return temporal.NewNonRetryableApplicationError("model rate limit reached", workflow.ModelRateLimitErrorType, nil, delay)
	}
	return err
}
