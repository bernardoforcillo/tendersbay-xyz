// Command indexer drains the search-indexing backlog and exits: it embeds
// unindexed tenders into the vector store and extracts the text of their
// attached documents.
//
// This runs as its own Kubernetes CronJob, separate from the ingestion job.
// The two phases used to share one process, ingestion first — and because
// fetching every provider regularly outran the job's activeDeadlineSeconds,
// the indexing phase was killed before it ever started. The symptom was
// silent: ingestion kept succeeding at its own job, while indexed_at stayed
// NULL for every tender and document extraction never advanced past the
// backlog it had built up earlier. Separate jobs mean a slow fetch can no
// longer starve indexing.
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
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/config"
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

	// Connect, not New: the ingestion job owns the schema and applies
	// migrations. This job fires four times an hour and must not race it.
	db, sqlDB, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		return 1
	}
	defer sqlDB.Close()

	// Unlike the old in-process indexing pass, an unreachable knowledge base
	// is fatal here. This job exists only to index, so it has no useful work
	// to fall back on, and a non-zero exit surfaces the outage as a failed
	// Job instead of a green run that silently indexed nothing.
	kb, err := knowledge.NewKnowledgeBase(ctx, cfg.QdrantURL, cfg.OllamaBaseURL, cfg.EmbeddingModel)
	if err != nil {
		slog.ErrorContext(ctx, "failed to connect to knowledge base", "error", err)
		return 1
	}

	idx := index.New(postgres.NewTenderRepo(db), kb, document.NewClient())
	if err := idx.Drain(ctx); err != nil {
		slog.ErrorContext(ctx, "indexing pass failed", "error", err)
		return 1
	}
	return 0
}
