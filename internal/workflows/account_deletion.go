package workflows

import (
	"fmt"
	"time"

	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/workflow"
	"go.temporal.io/sdk/temporal"
	tw "go.temporal.io/sdk/workflow"
)

// AccountDeletionCoordinator waits for the per-game photo/database cleanup to
// finish before removing the Supabase Auth identity and local account.
func AccountDeletionCoordinator(ctx tw.Context, accountID domain.ID, command workflow.Command) error {
	if command.AccountID != accountID || command.Type != workflow.CommandDeleteAccount {
		return fmt.Errorf("invalid account deletion command")
	}
	ctx = tw.WithActivityOptions(ctx, tw.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{InitialInterval: 10 * time.Second, BackoffCoefficient: 2, MaximumInterval: 5 * time.Minute, MaximumAttempts: 0},
	})
	iterations := 0
	for {
		var done bool
		if err := tw.ExecuteActivity(ctx, "DeleteAccount", struct {
			AccountID domain.ID `json:"account_id"`
			CommandID domain.ID `json:"command_id"`
		}{accountID, command.EventID}).Get(ctx, &done); err != nil {
			return err
		}
		if done {
			return nil
		}
		iterations++
		if iterations >= 100 {
			return tw.NewContinueAsNewError(ctx, workflow.AccountDeletionWorkflowName, accountID, command)
		}
		if err := tw.NewTimer(ctx, time.Minute).Get(ctx, nil); err != nil {
			return err
		}
	}
}
