// Command seed-cpv loads the embedded CPV 2008 vocabulary into
// tenders.cpv_terms.
//
// It is a command rather than a migration because the vocabulary is ~227k
// rows: embedding that many INSERT statements in a migration would bloat
// every binary that imports the migrations package, and drops runs each
// migration inside one transaction, which would hold locks for minutes.
//
// It is idempotent (ON CONFLICT DO UPDATE on the (code, lang) primary key),
// so re-running it after a vocabulary revision is the intended way to apply
// one.
//
// Run it once per environment, after migrations, before the search relies on
// the CPV bridge. In Kubernetes that is a Job — out of scope for this
// command, which only needs DATABASE_URL.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/postgres"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/config"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/cpvdata"
)

func main() { os.Exit(run()) }

func run() int {
	dryRun := flag.Bool("dry-run", false, "decode and report the vocabulary without writing")
	flag.Parse()

	rows, err := cpvdata.Rows()
	if err != nil {
		slog.Error("failed to decode vocabulary", "error", err)
		return 1
	}

	codes, langs := map[string]struct{}{}, map[string]struct{}{}
	for _, r := range rows {
		codes[r.Code] = struct{}{}
		langs[r.Lang] = struct{}{}
	}
	fmt.Printf("vocabulary: %d rows, %d codes, %d languages\n", len(rows), len(codes), len(langs))

	if *dryRun {
		fmt.Println("dry run: nothing written")
		return 0
	}

	cfg := config.FromEnv()
	if cfg.DatabaseURL == "" {
		slog.Error("DATABASE_URL is required")
		return 1
	}

	ctx := context.Background()
	// postgres.New runs the migrations too, so a fresh database is usable in
	// one step — the same behaviour every other entry point in this service
	// relies on.
	db, sqlDB, err := postgres.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		return 1
	}
	defer sqlDB.Close()

	repo := postgres.NewCPVRepo(db)
	n, err := repo.UpsertTerms(ctx, rows)
	if err != nil {
		slog.Error("failed to upsert cpv terms", "rows_sent", n, "error", err)
		return 1
	}

	total, err := repo.CountTerms(ctx)
	if err != nil {
		slog.Error("failed to count cpv terms", "error", err)
		return 1
	}
	fmt.Printf("seeded %d rows; cpv_terms now holds %d\n", n, total)

	// Existing tenders carry whatever labels the previous vocabulary produced —
	// the upsert only recomputes a row it is writing. Without this step a
	// re-seed would change nothing for anything already ingested.
	changed, err := repo.RecomputeLabels(ctx)
	if err != nil {
		slog.Error("failed to recompute tender labels", "error", err)
		return 1
	}
	fmt.Printf("recomputed cpv_labels on %d tenders\n", changed)
	return 0
}
