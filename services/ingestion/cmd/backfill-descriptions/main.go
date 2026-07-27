// Command backfill-descriptions re-derives the Description (BT-24 for ted,
// the EFORMS scope-of-work text for fr-boamp) and, for es-placsp, attached
// document references that ingestion didn't capture before those fixes
// shipped, for tenders already saved in tenders.ingested_tenders. It
// re-parses each row's untouched raw payload (already stored in the raw
// column) through the fixed ted/fr-boamp/es-placsp mappers — no re-fetch
// from TED/BOAMP/PLACSP needed — and re-saves through the same
// TenderRepo.Save upsert path ordinary ingestion uses, so version/history
// stay consistent with a normal re-ingest. It then force-resets indexed_at
// on every row it touches: adding an es-placsp Document doesn't change any
// column tender_repo.go's indexed_at dirty-check compares, so that CASE
// alone would never re-queue those rows for the indexer.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/postgres"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/eforms"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/es/codice"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/fr/boamp"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/fr/boampapi"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/config"
)

func main() { os.Exit(run()) }

func run() int {
	dryRun := flag.Bool("dry-run", false, "log what would change without writing to the database")
	flag.Parse()

	cfg := config.FromEnv()
	if cfg.DatabaseURL == "" {
		slog.Error("DATABASE_URL is required")
		return 1
	}

	ctx := context.Background()
	db, sqlDB, err := postgres.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		return 1
	}
	defer sqlDB.Close()

	rows, err := fetchRawRows(ctx, sqlDB)
	if err != nil {
		slog.Error("failed to list rows to backfill", "error", err)
		return 1
	}
	slog.Info("backfilling descriptions", "candidate_rows", len(rows), "dry_run", *dryRun)

	repo := postgres.NewTenderRepo(db)
	var updated, skipped, failed int
	for _, r := range rows {
		t, err := rebuild(r)
		if err != nil {
			slog.Error("failed to rebuild tender from raw payload", "id", r.id, "source", r.source, "error", err)
			failed++
			continue
		}
		if t == nil {
			skipped++
			continue
		}
		if *dryRun {
			slog.Info("would backfill", "id", r.id, "source", r.source, "source_ref", t.SourceRef,
				"description_preview", preview(t.Description), "documents", len(t.Documents))
			updated++
			continue
		}
		if _, err := repo.Save(ctx, []tender.Tender{*t}); err != nil {
			slog.Error("failed to save backfilled tender", "id", r.id, "error", err)
			failed++
			continue
		}
		if err := resetIndexedAt(ctx, sqlDB, r.id); err != nil {
			slog.Error("failed to reset indexed_at after backfill", "id", r.id, "error", err)
			failed++
			continue
		}
		slog.Info("backfilled", "id", r.id, "source", r.source, "source_ref", t.SourceRef)
		updated++
	}
	slog.Info("backfill complete", "updated", updated, "skipped_no_new_content", skipped, "failed", failed, "dry_run", *dryRun)
	if failed > 0 {
		return 1
	}
	return 0
}

// preview trims a description to a log-friendly length so a dry run stays
// scannable across hundreds of rows.
func preview(s string) string {
	const maxLen = 80
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		return s[:maxLen] + "…"
	}
	return s
}

type rawRow struct {
	id     int64
	source string
	raw    []byte
}

const selectRawRowsSQL = `
SELECT id, source, raw
FROM tenders.ingested_tenders
WHERE source IN ('ted', 'fr-boamp', 'es-placsp') AND raw IS NOT NULL
ORDER BY id
`

func fetchRawRows(ctx context.Context, sqlDB *sql.DB) ([]rawRow, error) {
	dbRows, err := sqlDB.QueryContext(ctx, selectRawRowsSQL)
	if err != nil {
		return nil, err
	}
	defer dbRows.Close()

	var rows []rawRow
	for dbRows.Next() {
		var r rawRow
		if err := dbRows.Scan(&r.id, &r.source, &r.raw); err != nil {
			return nil, err
		}
		rows = append(rows, r)
	}
	return rows, dbRows.Err()
}

const resetIndexedAtSQL = `UPDATE tenders.ingested_tenders SET indexed_at = NULL WHERE id = $1`

func resetIndexedAt(ctx context.Context, sqlDB *sql.DB, id int64) error {
	_, err := sqlDB.ExecContext(ctx, resetIndexedAtSQL, id)
	return err
}

// rebuild re-derives a tender from one row's stored raw payload using the
// fixed mapper for its source. It returns (nil, nil) when the source has no
// backfill (nothing beyond fr-boamp/es-placsp is handled) or when the
// payload yields no new Description/Documents — Save is skipped so an
// untouched row's last_seen_at/version aren't bumped for no reason.
func rebuild(r rawRow) (*tender.Tender, error) {
	switch r.source {
	case "ted":
		n, err := eforms.Decode(r.raw)
		if err != nil {
			return nil, fmt.Errorf("decode ted notice: %w", err)
		}
		t := eforms.Map(n, r.source)
		if t.Description == "" {
			return nil, nil
		}
		return &t, nil
	case "fr-boamp":
		rec, err := boampapi.Decode(json.RawMessage(r.raw))
		if err != nil {
			return nil, fmt.Errorf("decode boamp record: %w", err)
		}
		t := boamp.Map(rec, r.source)
		if t.Description == "" {
			return nil, nil
		}
		return &t, nil
	case "es-placsp":
		var xmlPayload string
		if err := json.Unmarshal(r.raw, &xmlPayload); err != nil {
			return nil, fmt.Errorf("decode codice raw wrapper: %w", err)
		}
		d, err := codice.Parse([]byte(xmlPayload))
		if err != nil {
			return nil, fmt.Errorf("parse codice payload: %w", err)
		}
		if len(d.Documents) == 0 {
			return nil, nil
		}
		t := codice.Map(d, r.source)
		return &t, nil
	default:
		return nil, nil
	}
}
