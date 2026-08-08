// Command retriever drains the portal-document backlog and exits: for every
// enriched tender whose own notice carried no usable award grid, it follows the
// buyer's documents link, enumerates the files that page publishes, extracts
// their text, and records both how much of it we got and — when we got none —
// why not.
//
// This is the fourth binary in the ingestion image and the fourth CronJob, and
// the split is load-bearing rather than tidy.
//
// # Why not a stage inside the enricher
//
// Every constant in the enrichment pass is calibrated for ONE upstream serving
// one 20-120 KB document per row: its batch size is sized as "roughly fifty
// seconds of TED traffic", its pace interval is TED's measured tolerance, and
// its circuit breaker is a health signal about that single host. This pass's
// unit of work is one robots fetch plus one listing fetch plus up to fifteen
// file downloads plus fifteen extractions, against a DIFFERENT host per row, at
// whatever rate a comune's server tolerates. Dropping that into the same drain
// would make every one of those constants wrong at once.
//
// That is the recorded incident restaged (see ../../../../infrastructure/
// kubernetes/tendersbay-xyz/ingestion/readme.md): fetching once outran the
// shared deadline and indexing silently stopped advancing for months while the
// job kept reporting success. A slow third-party portal sharing the enricher's
// window would starve the very pass this one depends on.
//
// # Why not the indexer
//
// The indexer's fetcher is internal/adapter/document, which has no robots
// handling, no per-host pacing and no dial guard, and whose own package comment
// forbids exactly this. Folding portal retrieval in there would point the
// unguarded fetcher at buyer portals. The rule runs the other way too and is
// this feature's sharpest constraint: THE INDEXER MUST NEVER TOUCH A BUYER
// PORTAL, which is why this binary extracts text itself rather than recording
// URLs for the indexer to fetch later.
//
// # The queue
//
// grid_usable = false AND docs_fetched_at IS NULL, which is why there is no
// backfill command: the pre-existing corpus is simply the queue's state at t=0,
// drained by this same code at the same paced rate with the same per-row failure
// isolation. And because grid_usable is three-valued — NULL means "not enriched
// yet" — a row the enricher has not read cannot enter this queue at all, so the
// ordering between the two jobs is enforced by the data rather than by their
// schedules.
//
// A run cut short by its deadline is not data loss: every file commits
// independently, and a tender whose docs_fetched_at is still NULL is simply
// re-listed by the next fire.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/telemetry"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/postgres"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/webdoc"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/config"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/core/retrieve"
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
	// twice an hour against an hourly migrating job, so calling New here would
	// put two schedules in a race to apply the same migration — and this pass
	// depends on migrations (0010 and 0011) that the ingestion job is
	// responsible for having applied. Keeping exactly one migrating entrypoint
	// is the documented invariant; see postgres.Connect's doc comment.
	db, sqlDB, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		return 1
	}
	defer sqlDB.Close()

	// One webdoc.Client for the whole run, shared by every worker. The per-host
	// state it carries — the robots cache, the single request slot, the pacer —
	// is what makes those bounds a property of this process's egress rather than
	// of one goroutine; a client per worker would multiply the outbound rate by
	// the number of workers and defeat the whole politeness story.
	//
	// No knowledge base is wired here, for the same reason the enricher wires
	// none: this pass writes text to Postgres and resets indexed_at on every
	// tender whose corpus changed, so the embedding work it creates is done by
	// the `indexer` CronJob. It therefore needs neither Qdrant nor Ollama to be
	// reachable to do its own work correctly.
	retriever := retrieve.New(
		postgres.NewTenderRepo(db),
		webdoc.NewClient(),
		webdoc.NewRegistry(),
		webdoc.Extractor{},
	)
	if err := retriever.Drain(ctx); err != nil {
		slog.ErrorContext(ctx, "portal retrieval pass failed", "error", err)
		return 1
	}
	return 0
}
