package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/amh1k/mythborn/internal/activities"
	"github.com/amh1k/mythborn/internal/config"
	"github.com/amh1k/mythborn/internal/database"
	"github.com/amh1k/mythborn/internal/models"
	"github.com/amh1k/mythborn/internal/repository"
	"github.com/amh1k/mythborn/internal/retrieval"
	"github.com/amh1k/mythborn/internal/storage"
	"github.com/amh1k/mythborn/internal/workflow"
	"github.com/amh1k/mythborn/internal/workflows"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	tw "go.temporal.io/sdk/workflow"
)

func main() {
	if err := run(); err != nil {
		slog.Error("worker stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.DatabaseURL == "" || cfg.TemporalAddress == "" || cfg.TemporalNamespace == "" || cfg.GeminiAPIKey == "" {
		return contextError("DATABASE_URL, TEMPORAL_ADDRESS, TEMPORAL_NAMESPACE, and GEMINI_API_KEY are required")
	}
	if strings.TrimSpace(cfg.SupabaseURL) == "" || strings.TrimSpace(cfg.SupabaseServiceKey) == "" {
		return contextError("SUPABASE_URL and SUPABASE_SERVICE_ROLE_KEY are required by worker photo activities")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	photos, err := storage.NewSupabasePhotoStorage(cfg.SupabaseURL, cfg.SupabaseServiceKey, cfg.SupabasePhotoBucket)
	if err != nil {
		return err
	}
	model, err := models.NewGemini(ctx, cfg.GeminiAPIKey)
	if err != nil {
		return err
	}
	store := repository.New(db)
	clientOptions := client.Options{HostPort: cfg.TemporalAddress, Namespace: cfg.TemporalNamespace}
	if cfg.TemporalAPIKey != "" {
		clientOptions.Credentials = client.NewAPIKeyStaticCredentials(cfg.TemporalAPIKey)
	}
	temporalClient, err := client.Dial(clientOptions)
	if err != nil {
		return err
	}
	defer temporalClient.Close()

	workerActivities := &activities.Activities{
		Store:     store,
		Photos:    photos,
		Models:    model,
		Retriever: retrieval.Retriever{Embedder: model, Index: store, Limit: 5},
	}
	authAdmin, err := activities.NewSupabaseAuthAdmin(cfg.SupabaseURL, cfg.SupabaseServiceKey)
	if err != nil {
		return err
	}
	workerActivities.Auth = authAdmin
	temporalWorker := worker.New(temporalClient, workflow.WorkflowTaskQueue, worker.Options{})
	temporalWorker.RegisterWorkflowWithOptions(workflows.Coordinator, tw.RegisterOptions{Name: workflow.CoordinatorWorkflowName})
	temporalWorker.RegisterWorkflowWithOptions(workflows.Agent, tw.RegisterOptions{Name: workflow.AgentWorkflowName})
	temporalWorker.RegisterWorkflowWithOptions(workflows.AccountDeletionCoordinator, tw.RegisterOptions{Name: workflow.AccountDeletionWorkflowName})
	temporalWorker.RegisterActivity(workerActivities)

	dispatcher := &workflows.OutboxDispatcher{Client: temporalClient, Store: store, Logger: slog.Default(), BatchSize: 25, PollInterval: time.Second, Lease: 45 * time.Second}
	dispatcherDone := make(chan struct{})
	go func() {
		defer close(dispatcherDone)
		if err := dispatcher.Run(ctx); err != nil && ctx.Err() == nil {
			slog.Error("outbox dispatcher stopped", "error", err)
			stop()
		}
	}()

	interrupt := make(chan interface{})
	go func() { <-ctx.Done(); close(interrupt) }()
	slog.Info("Temporal worker started", "task_queue", workflow.WorkflowTaskQueue, "temporal_address", cfg.TemporalAddress)
	workerErr := temporalWorker.Run(interrupt)
	stop()
	<-dispatcherDone
	if workerErr != nil {
		return workerErr
	}
	return nil
}

type configError string

func (e configError) Error() string     { return string(e) }
func contextError(message string) error { return configError(message) }
