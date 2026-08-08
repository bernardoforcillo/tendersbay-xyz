// Command enricher drains the notice-enrichment backlog and exits: for every
// ingested TED tender whose eForms XML document has not been read yet, it
// fetches that document, parses the structure only the XML carries — per-lot
// values, the award grid, the parties a bidder needs in order to challenge an
// award — and persists it.
//
// This is the third binary in the ingestion image and the third CronJob, for
// the reason the indexer became the second one. Its pace is set by an external
// host: TED rate-limits aggressively, so notice documents are fetched at four
// a second and a backlog measured in thousands of notices is measured in tens
// of minutes. Folded into either existing job, that work would consume the
// whole deadline and starve the other phase — exactly the failure that made
// indexing stop advancing for months while the job it shared kept reporting
// success. Separate jobs mean a slow upstream can only starve itself.
//
// The queue is xml_fetched_at IS NULL, which is why there is no backfill
// command: the pre-existing corpus is simply the queue's state at t=0, drained
// by this same code at the same paced rate with the same per-row failure
// isolation. A run cut short by its deadline is not data loss — every batch
// commits independently and whatever is still queued is re-listed by the next
// fire.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/telemetry"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/document"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/postgres"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/eforms"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/config"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/core/enrich"
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
	// migrations, and it must stay the ONLY binary that does. This job fires
	// four times an hour against an hourly migrating job, so calling New here
	// would put two schedules in a race to apply the same migration — and this
	// pass depends on a migration (0010) that the ingestion job is responsible
	// for having applied. Keeping exactly one migrating entrypoint is the
	// documented invariant; see postgres.Connect's doc comment.
	db, sqlDB, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		return 1
	}
	defer sqlDB.Close()

	// No knowledge base is wired here, unlike the indexer: enrichment writes
	// structure to Postgres and resets indexed_at on every row it touches, so
	// the embedding work it creates is done by the indexer CronJob. That is
	// also why this job needs neither Qdrant nor Ollama to be reachable to do
	// its own work correctly.
	enricher := enrich.New(
		postgres.NewTenderRepo(db),
		document.NewNoticeXMLClient(),
		eforms.DetailParser{},
	)
	if err := enricher.Drain(ctx); err != nil {
		slog.ErrorContext(ctx, "enrichment pass failed", "error", err)
		return 1
	}
	return 0
}
