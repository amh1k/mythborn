package workflows

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/workflow"
	"go.temporal.io/sdk/client"
)

type OutboxStore interface {
	ClaimOutbox(context.Context, int, time.Duration) ([]domain.WorkflowCommand, error)
	MarkOutboxDelivered(context.Context, domain.ID) error
	MarkOutboxFailed(context.Context, domain.ID, string, time.Time) error
}

type OutboxDispatcher struct {
	Client       client.Client
	Store        OutboxStore
	Logger       *slog.Logger
	BatchSize    int
	PollInterval time.Duration
	Lease        time.Duration
}

func (d *OutboxDispatcher) Run(ctx context.Context) error {
	if d.Client == nil || d.Store == nil {
		return fmt.Errorf("outbox dispatcher requires Temporal client and store")
	}
	batch := d.BatchSize
	if batch < 1 || batch > 100 {
		batch = 25
	}
	poll := d.PollInterval
	if poll < 100*time.Millisecond {
		poll = time.Second
	}
	lease := d.Lease
	if lease < 5*time.Second {
		lease = 45 * time.Second
	}
	logger := d.Logger
	if logger == nil {
		logger = slog.Default()
	}
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	for {
		commands, err := d.Store.ClaimOutbox(ctx, batch, lease)
		if err != nil {
			logger.Error("claim workflow outbox", "error", err)
		} else {
			for _, row := range commands {
				if err := d.Dispatch(ctx, row); err != nil {
					logger.Warn("dispatch workflow command", "event_id", row.ID, "command_type", row.Type, "error", err)
					delay := retryDelay(row.AttemptCount)
					if markErr := d.Store.MarkOutboxFailed(ctx, row.ID, "dispatch_failed", time.Now().Add(delay)); markErr != nil {
						logger.Error("mark workflow command failed", "event_id", row.ID, "error", markErr)
					}
					continue
				}
				if err := d.Store.MarkOutboxDelivered(ctx, row.ID); err != nil {
					logger.Error("mark workflow command delivered", "event_id", row.ID, "error", err)
				}
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (d *OutboxDispatcher) Dispatch(ctx context.Context, row domain.WorkflowCommand) error {
	cmd, err := workflow.DecodeCommand(row)
	if err != nil {
		return fmt.Errorf("decode outbox command: %w", err)
	}
	if cmd.Type == workflow.CommandDeleteAccount {
		_, err = d.Client.SignalWithStartWorkflow(ctx,
			workflow.AccountDeletionWorkflowID(cmd.AccountID), workflow.AccountDeletionSignalName, cmd,
			client.StartWorkflowOptions{ID: workflow.AccountDeletionWorkflowID(cmd.AccountID), TaskQueue: workflow.WorkflowTaskQueue},
			workflow.AccountDeletionWorkflowName, cmd.AccountID, cmd,
		)
		return err
	}
	if cmd.GameID == "" {
		return fmt.Errorf("game command is missing game_id")
	}
	_, err = d.Client.SignalWithStartWorkflow(ctx,
		workflow.WorkflowID(cmd.GameID), workflow.CoordinatorSignalName, cmd,
		client.StartWorkflowOptions{ID: workflow.WorkflowID(cmd.GameID), TaskQueue: workflow.WorkflowTaskQueue},
		workflow.CoordinatorWorkflowName, cmd.GameID, nil,
	)
	return err
}

func retryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 7 {
		attempt = 7
	}
	delay := time.Second * time.Duration(1<<uint(attempt-1))
	if delay > 2*time.Minute {
		return 2 * time.Minute
	}
	return delay
}
