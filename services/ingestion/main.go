// Command ingestion runs one tender-ingestion cycle and exits. Scheduling is
// external — a Kubernetes CronJob fires this on the hour (see the design
// doc at docs/superpowers/specs/2026-07-06-ingestion-service-design.md).
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/knowledge"
	"github.com/bernardoforcillo/tendersbay-xyz/go-services/telemetry"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/document"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/index"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/postgres"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/config"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/core/ingestion"
)

func main() { os.Exit(run()) }

func run() int {
	cfg := config.FromEnv()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdown, err := telemetry.Setup(ctx, telemetry.Config{
		APIKey:      cfg.PostHogAPIKey,
		Host:        cfg.PostHogHost,
		ServiceName: cfg.ServiceName,
	})
	if err != nil {
		slog.Error("failed to set up telemetry", "error", err)
		return 1
	}
	defer func() { _ = shutdown(context.Background()) }()

	if cfg.DatabaseURL == "" {
		slog.Error("DATABASE_URL is required")
		return 1
	}

	db, sqlDB, err := postgres.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		return 1
	}
	defer sqlDB.Close()

	sink := postgres.NewTenderRepo(db)
	sources := source.NewRegistry()
	svc := ingestion.NewService(sources, sink)

	report := svc.RunOnce(ctx)
	slog.Info("ingestion complete", "providers", len(sources), "summary", report.Summary())

	// Indexing failures never affect the process exit code — Postgres is
	// the source of truth for ingestion; a Qdrant/Ollama outage delays
	// search indexing, it doesn't fail the ingestion run.
	kb, kbErr := knowledge.NewKnowledgeBase(ctx, cfg.QdrantURL, cfg.OllamaBaseURL, cfg.EmbeddingModel)
	if kbErr != nil {
		slog.ErrorContext(ctx, "failed to connect to knowledge base, skipping indexing this cycle", "error", kbErr)
	} else {
		idx := index.New(sink, kb, document.NewClient())
		if idxErr := idx.RunOnce(ctx); idxErr != nil {
			slog.ErrorContext(ctx, "indexing pass failed", "error", idxErr)
		}
	}

	if report.Failed() {
		return 1
	}
	return 0
}
