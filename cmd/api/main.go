package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/amh1k/mythborn/internal/api"
	"github.com/amh1k/mythborn/internal/auth"
	"github.com/amh1k/mythborn/internal/config"
	"github.com/amh1k/mythborn/internal/database"
	"github.com/amh1k/mythborn/internal/httpapi/handlers"
	"github.com/amh1k/mythborn/internal/storage"
	"github.com/amh1k/mythborn/internal/workflow"
	"github.com/amh1k/mythborn/internal/workflows"
	"go.temporal.io/sdk/client"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load configuration", "error", err)
		os.Exit(1)
	}
	if cfg.DatabaseURL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	db, err := database.Open(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	verifier, err := auth.NewSupabaseAuthenticator(db, cfg.SupabaseURL)
	if err != nil {
		slog.Error("configure Supabase authenticator", "error", err)
		os.Exit(1)
	}
	photos, err := storage.NewSupabasePhotoStorage(cfg.SupabaseURL, cfg.SupabaseServiceKey, cfg.SupabasePhotoBucket)
	if err != nil {
		slog.Error("configure Supabase photo storage", "error", err)
		os.Exit(1)
	}

	var debates workflow.DebatePreviewReader
	if cfg.TemporalAddress != "" {
		options := client.Options{HostPort: cfg.TemporalAddress, Namespace: cfg.TemporalNamespace}
		if cfg.TemporalAPIKey != "" {
			options.Credentials = client.NewAPIKeyStaticCredentials(cfg.TemporalAPIKey)
		}
		// Connect on the first preview request; a Temporal outage must not stop
		// sign-in, durable episode reads, or other API operations.
		temporalClient, err := client.NewLazyClient(options)
		if err != nil {
			slog.Warn("live debate previews unavailable")
		} else {
			defer temporalClient.Close()
			debates = workflows.DebateReader{Client: temporalClient}
		}
	}

	server := &http.Server{
		Addr: cfg.HTTPAddress,
		Handler: api.NewHandler(api.Dependencies{
			DB: db, Auth: verifier, Photos: photos, Debates: debates, Logger: slog.Default(), CORSAllowedOrigins: cfg.CORSAllowedOrigins,
		}, handlers.Registrar{}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	slog.Info("api listening", "address", cfg.HTTPAddress)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("api server stopped", "error", err)
		os.Exit(1)
	}
}
