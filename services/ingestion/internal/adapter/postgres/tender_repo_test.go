package postgres_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/postgres"
	coreingestion "github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/core/ingestion"
)

func testRepo(t *testing.T) (*postgres.TenderRepo, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, sqlDB, err := postgres.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("postgres.New: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return postgres.NewTenderRepo(db), sqlDB
}

func cleanupTender(t *testing.T, sqlDB *sql.DB, source, sourceRef string) {
	t.Helper()
	t.Cleanup(func() {
		_, _ = sqlDB.Exec(`DELETE FROM tenders.ingested_tenders WHERE source = $1 AND source_ref = $2`, source, sourceRef)
	})
}

func TestSave_UpsertIsIdempotentAndTracksStatus(t *testing.T) {
	repo, sqlDB := testRepo(t)
	ctx := context.Background()
	source, ref := "test-repo", "upsert-1"
	cleanupTender(t, sqlDB, source, ref)

	open := tender.Tender{Source: source, SourceRef: ref, Title: "First", Status: tender.StatusOpen}
	result, err := repo.Save(ctx, []tender.Tender{open})
	if err != nil {
		t.Fatalf("Save (insert): %v", err)
	}
	if result.Inserted != 1 || result.Updated != 0 {
		t.Fatalf("first Save result = %+v, want Inserted=1 Updated=0", result)
	}

	var version int
	var lastSeen1 time.Time
	row := sqlDB.QueryRowContext(ctx,
		`SELECT version, last_seen_at FROM tenders.ingested_tenders WHERE source = $1 AND source_ref = $2`,
		source, ref)
	if err := row.Scan(&version, &lastSeen1); err != nil {
		t.Fatalf("query after insert: %v", err)
	}
	if version != 1 {
		t.Fatalf("version after insert = %d, want 1", version)
	}

	// Re-upsert with the SAME status: version/history untouched, last_seen_at bumped.
	result, err = repo.Save(ctx, []tender.Tender{open})
	if err != nil {
		t.Fatalf("Save (same status): %v", err)
	}
	if result.Inserted != 0 || result.Updated != 1 {
		t.Fatalf("same-status Save result = %+v, want Inserted=0 Updated=1", result)
	}

	var count int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT count(*) FROM tenders.ingested_tenders WHERE source = $1 AND source_ref = $2`,
		source, ref).Scan(&count); err != nil {
		t.Fatalf("count after re-upsert: %v", err)
	}
	if count != 1 {
		t.Fatalf("row count = %d, want 1 (no duplicate)", count)
	}

	var lastSeen2 time.Time
	var history []byte
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT version, last_seen_at, history FROM tenders.ingested_tenders WHERE source = $1 AND source_ref = $2`,
		source, ref).Scan(&version, &lastSeen2, &history); err != nil {
		t.Fatalf("query after same-status upsert: %v", err)
	}
	if version != 1 {
		t.Fatalf("version after same-status upsert = %d, want unchanged 1", version)
	}
	if !lastSeen2.After(lastSeen1) {
		t.Fatalf("last_seen_at did not advance: %v -> %v", lastSeen1, lastSeen2)
	}
	var historyEvents []map[string]any
	if err := json.Unmarshal(history, &historyEvents); err != nil {
		t.Fatalf("unmarshal history: %v", err)
	}
	if len(historyEvents) != 0 {
		t.Fatalf("history = %v, want empty after same-status upsert", historyEvents)
	}

	// Re-upsert with a CHANGED status: version bumps, one history event appended.
	awarded := open
	awarded.Status = tender.StatusAwarded
	if _, err := repo.Save(ctx, []tender.Tender{awarded}); err != nil {
		t.Fatalf("Save (status change): %v", err)
	}

	if err := sqlDB.QueryRowContext(ctx,
		`SELECT version, history FROM tenders.ingested_tenders WHERE source = $1 AND source_ref = $2`,
		source, ref).Scan(&version, &history); err != nil {
		t.Fatalf("query after status change: %v", err)
	}
	if version != 2 {
		t.Fatalf("version after status change = %d, want 2", version)
	}
	if err := json.Unmarshal(history, &historyEvents); err != nil {
		t.Fatalf("unmarshal history after status change: %v", err)
	}
	if len(historyEvents) != 1 || historyEvents[0]["event"] != "status_changed" ||
		historyEvents[0]["from"] != "open" || historyEvents[0]["to"] != "awarded" {
		t.Fatalf("history after status change = %v, want one status_changed open->awarded event", historyEvents)
	}
}

func TestSave_DocumentAndLotUpsertAreIdempotent(t *testing.T) {
	repo, sqlDB := testRepo(t)
	ctx := context.Background()
	source, ref := "test-repo", "docs-lots-1"
	cleanupTender(t, sqlDB, source, ref)

	value := int64(5000)
	t1 := tender.Tender{
		Source: source, SourceRef: ref, Title: "With docs and lots", Status: tender.StatusOpen,
		Documents: []tender.Document{{URL: "https://example.org/notice.pdf", Type: "notice"}},
		Lots:      []tender.Lot{{Ref: "LOT-1", Title: "Lot one", Value: &value, Currency: "EUR"}},
	}
	if _, err := repo.Save(ctx, []tender.Tender{t1}); err != nil {
		t.Fatalf("Save (first): %v", err)
	}
	// Re-save the same tender with the same document URL and lot ref.
	if _, err := repo.Save(ctx, []tender.Tender{t1}); err != nil {
		t.Fatalf("Save (second): %v", err)
	}

	var tenderID int64
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT id FROM tenders.ingested_tenders WHERE source = $1 AND source_ref = $2`,
		source, ref).Scan(&tenderID); err != nil {
		t.Fatalf("query tender id: %v", err)
	}

	var docCount, lotCount int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT count(*) FROM tenders.ingested_tender_documents WHERE tender_id = $1`, tenderID).Scan(&docCount); err != nil {
		t.Fatalf("count documents: %v", err)
	}
	if docCount != 1 {
		t.Fatalf("document count = %d, want 1 (no duplicate)", docCount)
	}
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT count(*) FROM tenders.ingested_tender_lots WHERE tender_id = $1`, tenderID).Scan(&lotCount); err != nil {
		t.Fatalf("count lots: %v", err)
	}
	if lotCount != 1 {
		t.Fatalf("lot count = %d, want 1 (no duplicate)", lotCount)
	}
}

func TestSave_CPVSecondaryRoundTripsBackslashesAndQuotes(t *testing.T) {
	repo, sqlDB := testRepo(t)
	ctx := context.Background()
	source, ref := "test-repo", "cpv-secondary-escaping-1"
	cleanupTender(t, sqlDB, source, ref)

	want := []string{`a\b`, `c"d`, `e\"f`}
	t1 := tender.Tender{
		Source: source, SourceRef: ref, Title: "CPV escaping", Status: tender.StatusOpen,
		CPVSecondary: want,
	}
	if _, err := repo.Save(ctx, []tender.Tender{t1}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Index into the array directly rather than parsing Postgres's array-literal
	// text format in Go — sidesteps needing a second array parser to test the
	// first one.
	var got1, got2, got3 string
	row := sqlDB.QueryRowContext(ctx,
		`SELECT cpv_secondary[1], cpv_secondary[2], cpv_secondary[3]
		 FROM tenders.ingested_tenders WHERE source = $1 AND source_ref = $2`,
		source, ref)
	if err := row.Scan(&got1, &got2, &got3); err != nil {
		t.Fatalf("scan cpv_secondary elements: %v", err)
	}

	got := []string{got1, got2, got3}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cpv_secondary[%d] = %q, want %q (backslash or quote was mangled)", i+1, got[i], want[i])
		}
	}
}

func TestRecordRun_WritesAuditRow(t *testing.T) {
	repo, sqlDB := testRepo(t)
	ctx := context.Background()

	rec := coreingestion.RunRecord{
		Source: "test-provider-record-run", Fetched: 3, Inserted: 2, Updated: 1,
		StartedAt: time.Now().UTC(), FinishedAt: time.Now().UTC(),
	}
	t.Cleanup(func() {
		_, _ = sqlDB.Exec(`DELETE FROM tenders.ingestion_runs WHERE source = $1`, rec.Source)
	})
	if err := repo.RecordRun(ctx, rec); err != nil {
		t.Fatalf("RecordRun: %v", err)
	}

	var fetched, inserted, updated int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT fetched, inserted, updated FROM tenders.ingestion_runs WHERE source = $1`,
		rec.Source).Scan(&fetched, &inserted, &updated); err != nil {
		t.Fatalf("query ingestion_runs: %v", err)
	}
	if fetched != 3 || inserted != 2 || updated != 1 {
		t.Fatalf("audit row = fetched=%d inserted=%d updated=%d, want 3/2/1", fetched, inserted, updated)
	}
}
