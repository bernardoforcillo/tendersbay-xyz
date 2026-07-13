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

func TestSaveDocumentParts_UpsertIsIdempotent(t *testing.T) {
	repo, sqlDB := testRepo(t)
	ctx := context.Background()
	source, ref := "test-repo", "doc-parts-1"
	cleanupTender(t, sqlDB, source, ref)

	tenderResult, err := repo.Save(ctx, []tender.Tender{{
		Source: source, SourceRef: ref, Title: "Doc parts test", Status: tender.StatusOpen,
		Documents: []tender.Document{{URL: "https://example.org/notice.pdf", Type: "notice"}},
	}})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if tenderResult.Inserted != 1 {
		t.Fatalf("Save result = %+v, want Inserted=1", tenderResult)
	}

	var documentID int64
	row := sqlDB.QueryRowContext(ctx,
		`SELECT d.id FROM tenders.ingested_tender_documents d
		 JOIN tenders.ingested_tenders t ON t.id = d.tender_id
		 WHERE t.source = $1 AND t.source_ref = $2`,
		source, ref)
	if err := row.Scan(&documentID); err != nil {
		t.Fatalf("find document id: %v", err)
	}

	if err := repo.SaveDocumentParts(ctx, documentID, []string{"part one", "part two"}); err != nil {
		t.Fatalf("SaveDocumentParts (insert): %v", err)
	}
	got, err := repo.DocumentParts(ctx, documentID)
	if err != nil {
		t.Fatalf("DocumentParts: %v", err)
	}
	if len(got) != 2 || got[0] != "part one" || got[1] != "part two" {
		t.Fatalf("DocumentParts = %v, want [part one, part two]", got)
	}

	// Re-saving the same parts is an idempotent update, not a duplicate.
	if err := repo.SaveDocumentParts(ctx, documentID, []string{"part one updated", "part two"}); err != nil {
		t.Fatalf("SaveDocumentParts (update): %v", err)
	}
	got, err = repo.DocumentParts(ctx, documentID)
	if err != nil {
		t.Fatalf("DocumentParts after update: %v", err)
	}
	if len(got) != 2 || got[0] != "part one updated" {
		t.Fatalf("DocumentParts after update = %v, want [part one updated, part two]", got)
	}
}

func TestListUnindexed_ReturnsOnlyNullIndexedAtWithDocuments(t *testing.T) {
	repo, sqlDB := testRepo(t)
	ctx := context.Background()
	source := "test-repo"
	unindexedRef, indexedRef := "unindexed-1", "already-indexed-1"
	cleanupTender(t, sqlDB, source, unindexedRef)
	cleanupTender(t, sqlDB, source, indexedRef)

	if _, err := repo.Save(ctx, []tender.Tender{
		{
			Source: source, SourceRef: unindexedRef, Title: "Needs indexing",
			Buyer: tender.Buyer{Name: "Comune di Roma"}, CPV: "45233220",
			ProcedureType: "open", Country: "IT", Status: tender.StatusOpen,
			Documents: []tender.Document{{URL: "https://example.org/a.pdf", Type: "notice"}},
		},
		{
			Source: source, SourceRef: indexedRef, Title: "Already indexed",
			Status: tender.StatusOpen,
		},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var indexedID int64
	row := sqlDB.QueryRowContext(ctx,
		`SELECT id FROM tenders.ingested_tenders WHERE source = $1 AND source_ref = $2`,
		source, indexedRef)
	if err := row.Scan(&indexedID); err != nil {
		t.Fatalf("find indexed tender id: %v", err)
	}
	if err := repo.MarkIndexed(ctx, indexedID); err != nil {
		t.Fatalf("MarkIndexed: %v", err)
	}

	unindexed, err := repo.ListUnindexed(ctx, 100)
	if err != nil {
		t.Fatalf("ListUnindexed: %v", err)
	}

	var found *postgres.UnindexedTender
	for i := range unindexed {
		if unindexed[i].SourceRef == unindexedRef {
			found = &unindexed[i]
		}
		if unindexed[i].SourceRef == indexedRef {
			t.Fatalf("ListUnindexed returned the already-indexed tender %q", indexedRef)
		}
	}
	if found == nil {
		t.Fatal("ListUnindexed did not return the unindexed tender")
	}
	if found.Title != "Needs indexing" || found.BuyerName != "Comune di Roma" ||
		found.CPV != "45233220" || found.ProcedureType != "open" || found.Country != "IT" ||
		found.Status != "open" || found.Source != source || found.SourceRef != unindexedRef {
		t.Errorf("found tender = %+v, want matching structured fields", found)
	}
	if len(found.Documents) != 1 || found.Documents[0].URL != "https://example.org/a.pdf" {
		t.Errorf("found.Documents = %+v, want one document with the saved URL", found.Documents)
	}
}

func TestListUnindexed_RespectsLimit(t *testing.T) {
	repo, sqlDB := testRepo(t)
	ctx := context.Background()
	source := "test-repo"
	refs := []string{"limit-1", "limit-2", "limit-3"}
	for _, ref := range refs {
		cleanupTender(t, sqlDB, source, ref)
	}

	batch := make([]tender.Tender, len(refs))
	for i, ref := range refs {
		batch[i] = tender.Tender{Source: source, SourceRef: ref, Title: "Limit test", Status: tender.StatusOpen}
	}
	if _, err := repo.Save(ctx, batch); err != nil {
		t.Fatalf("Save: %v", err)
	}

	unindexed, err := repo.ListUnindexed(ctx, 2)
	if err != nil {
		t.Fatalf("ListUnindexed: %v", err)
	}
	count := 0
	for _, u := range unindexed {
		if u.Source == source {
			for _, ref := range refs {
				if u.SourceRef == ref {
					count++
				}
			}
		}
	}
	if count > 2 {
		t.Errorf("ListUnindexed(limit=2) returned %d of our 3 test tenders, want at most 2", count)
	}
}

func TestSave_ResetsIndexedAtWhenSummaryFieldsChange(t *testing.T) {
	repo, sqlDB := testRepo(t)
	ctx := context.Background()
	source, ref := "test-repo", "reindex-on-change-1"
	cleanupTender(t, sqlDB, source, ref)

	open := tender.Tender{Source: source, SourceRef: ref, Title: "Needs reindex", Status: tender.StatusOpen}
	if _, err := repo.Save(ctx, []tender.Tender{open}); err != nil {
		t.Fatalf("Save (insert): %v", err)
	}

	markIndexed := func() {
		t.Helper()
		if _, err := sqlDB.ExecContext(ctx,
			`UPDATE tenders.ingested_tenders SET indexed_at = now() WHERE source = $1 AND source_ref = $2`,
			source, ref); err != nil {
			t.Fatalf("mark indexed: %v", err)
		}
	}
	indexedAt := func() sql.NullTime {
		t.Helper()
		var got sql.NullTime
		if err := sqlDB.QueryRowContext(ctx,
			`SELECT indexed_at FROM tenders.ingested_tenders WHERE source = $1 AND source_ref = $2`,
			source, ref).Scan(&got); err != nil {
			t.Fatalf("query indexed_at: %v", err)
		}
		return got
	}

	// A changed summary-affecting field (Status) must reset indexed_at to
	// NULL so ListUnindexed's indexed_at IS NULL filter picks the tender
	// back up.
	markIndexed()
	if got := indexedAt(); !got.Valid {
		t.Fatal("indexed_at not set after MarkIndexed")
	}

	changed := open
	changed.Status = tender.StatusAwarded
	if _, err := repo.Save(ctx, []tender.Tender{changed}); err != nil {
		t.Fatalf("Save (status change): %v", err)
	}
	if got := indexedAt(); got.Valid {
		t.Fatalf("indexed_at = %v, want NULL after a summary-affecting field changed", got.Time)
	}

	// An unchanged re-save of an already-indexed tender must NOT reset
	// indexed_at — the CASE must not blindly reset on every save.
	markIndexed()
	if got := indexedAt(); !got.Valid {
		t.Fatal("indexed_at not set after second MarkIndexed")
	}
	if _, err := repo.Save(ctx, []tender.Tender{changed}); err != nil {
		t.Fatalf("Save (no change): %v", err)
	}
	if got := indexedAt(); !got.Valid {
		t.Fatal("indexed_at was reset to NULL despite no summary-affecting field changing")
	}
}

func TestDocumentParts_EmptyWhenNoneSaved(t *testing.T) {
	repo, sqlDB := testRepo(t)
	ctx := context.Background()
	source, ref := "test-repo", "doc-parts-empty"
	cleanupTender(t, sqlDB, source, ref)

	if _, err := repo.Save(ctx, []tender.Tender{{
		Source: source, SourceRef: ref, Title: "No parts yet", Status: tender.StatusOpen,
		Documents: []tender.Document{{URL: "https://example.org/notice2.pdf", Type: "notice"}},
	}}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var documentID int64
	row := sqlDB.QueryRowContext(ctx,
		`SELECT d.id FROM tenders.ingested_tender_documents d
		 JOIN tenders.ingested_tenders t ON t.id = d.tender_id
		 WHERE t.source = $1 AND t.source_ref = $2`,
		source, ref)
	if err := row.Scan(&documentID); err != nil {
		t.Fatalf("find document id: %v", err)
	}

	got, err := repo.DocumentParts(ctx, documentID)
	if err != nil {
		t.Fatalf("DocumentParts: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("DocumentParts = %v, want empty", got)
	}
}
