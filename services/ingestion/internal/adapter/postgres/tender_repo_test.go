package postgres_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/postgres"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/core/enrich"
	coreingestion "github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/core/ingestion"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/core/retrieve"
)

// The enrichment pass wires this repository in through the port the core owns,
// so the assignment below is the only place that check exists at compile time —
// and unlike every test in this file, it holds whether or not a database is
// reachable.
var _ enrich.Repo = (*postgres.TenderRepo)(nil)

// The portal-document retrieval pass wires the same repository in through its
// own port, for the same reason and with the same guarantee. Both assertions
// are also the only mechanical statement of the house dependency rule at this
// boundary: the interface is declared in core and satisfied here, never the
// other way round.
var _ retrieve.Repo = (*postgres.TenderRepo)(nil)

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

	if err := repo.SaveDocumentParts(ctx, documentID, []tender.DocumentPart{
		{Text: "part one", PageStart: 1, PageEnd: 2, SectionTitle: "Oggetto"},
		{Text: "part two", PageStart: 3, PageEnd: 3},
	}); err != nil {
		t.Fatalf("SaveDocumentParts (insert): %v", err)
	}
	got, err := repo.DocumentParts(ctx, documentID)
	if err != nil {
		t.Fatalf("DocumentParts: %v", err)
	}
	if len(got) != 2 || got[0].Text != "part one" || got[1].Text != "part two" {
		t.Fatalf("DocumentParts = %+v, want [part one, part two]", got)
	}
	if got[0].PageStart != 1 || got[0].PageEnd != 2 || got[0].SectionTitle != "Oggetto" {
		t.Fatalf("DocumentParts[0] = %+v, want pages 1-2 under \"Oggetto\"", got[0])
	}

	// Re-saving the same parts is an idempotent update, not a duplicate — and
	// the provenance is updated with the text, not left behind pointing at
	// pages the new text never came from.
	if err := repo.SaveDocumentParts(ctx, documentID, []tender.DocumentPart{
		{Text: "part one updated", PageStart: 5, PageEnd: 6, SectionTitle: "Criteri", HasTable: true},
		{Text: "part two", PageStart: 3, PageEnd: 3},
	}); err != nil {
		t.Fatalf("SaveDocumentParts (update): %v", err)
	}
	got, err = repo.DocumentParts(ctx, documentID)
	if err != nil {
		t.Fatalf("DocumentParts after update: %v", err)
	}
	if len(got) != 2 || got[0].Text != "part one updated" {
		t.Fatalf("DocumentParts after update = %+v, want [part one updated, part two]", got)
	}
	if got[0].PageStart != 5 || got[0].PageEnd != 6 || got[0].SectionTitle != "Criteri" || !got[0].HasTable {
		t.Fatalf("DocumentParts[0] after update = %+v, want the refreshed page/section metadata", got[0])
	}
}

// section_path is stored as a real text[] and read back as JSON rather than a
// comma-joined string, because headings routinely contain the delimiter. The
// round-trip is the only place that can prove one heading did not become two.
func TestSaveDocumentParts_SectionPathSurvivesDelimitersInHeadings(t *testing.T) {
	repo, sqlDB := testRepo(t)
	ctx := context.Background()
	source, ref := "test-repo", "doc-parts-section-path"
	cleanupTender(t, sqlDB, source, ref)

	if _, err := repo.Save(ctx, []tender.Tender{{
		Source: source, SourceRef: ref, Title: "Section path test", Status: tender.StatusOpen,
		Documents: []tender.Document{{URL: "https://example.org/capitolato.pdf", Type: "spec"}},
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

	path := []string{`Capo 1, Disposizioni generali`, `Art. 4 "Criteri" \ sub-a`}
	if err := repo.SaveDocumentParts(ctx, documentID, []tender.DocumentPart{
		{Text: "criteri di aggiudicazione", PageStart: 17, PageEnd: 17, SectionPath: path, SectionTitle: path[1]},
	}); err != nil {
		t.Fatalf("SaveDocumentParts: %v", err)
	}

	got, err := repo.DocumentParts(ctx, documentID)
	if err != nil {
		t.Fatalf("DocumentParts: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(DocumentParts) = %d, want 1", len(got))
	}
	if !reflect.DeepEqual(got[0].SectionPath, path) {
		t.Errorf("SectionPath = %q, want %q — commas, quotes and backslashes must round-trip", got[0].SectionPath, path)
	}
}

// A part saved without any page or section metadata — every part written
// before migration 0010, none of which are backfilled — must read back as the
// zero value, not as an error or a one-element path holding "".
func TestDocumentParts_MissingMetadataReadsAsZero(t *testing.T) {
	repo, sqlDB := testRepo(t)
	ctx := context.Background()
	source, ref := "test-repo", "doc-parts-no-metadata"
	cleanupTender(t, sqlDB, source, ref)

	if _, err := repo.Save(ctx, []tender.Tender{{
		Source: source, SourceRef: ref, Title: "Legacy part test", Status: tender.StatusOpen,
		Documents: []tender.Document{{URL: "https://example.org/legacy.pdf", Type: "notice"}},
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

	// Written the way a pre-0010 extraction did: content only, every new
	// column left NULL.
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO tenders.ingested_tender_document_parts (document_id, index, content) VALUES ($1, 0, $2)`,
		documentID, "legacy text"); err != nil {
		t.Fatalf("insert legacy part: %v", err)
	}

	got, err := repo.DocumentParts(ctx, documentID)
	if err != nil {
		t.Fatalf("DocumentParts: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(DocumentParts) = %d, want 1", len(got))
	}
	want := tender.DocumentPart{Text: "legacy text"}
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("DocumentParts[0] = %+v, want %+v (NULL metadata reads as unknown, not as page zero)", got[0], want)
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
			Description: "Resurfacing of municipal roads in the historic center.",
			Buyer:       tender.Buyer{Name: "Comune di Roma"}, CPV: "45233220",
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
	if found.Description != "Resurfacing of municipal roads in the historic center." {
		t.Errorf("found.Description = %q, want the saved free-text description", found.Description)
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

// TestSave_ResetsIndexedAtWhenDescriptionChanges covers the same CASE for
// description specifically: backfilling a richer description onto a tender
// that was already indexed with none (description = ”) must re-queue it
// for indexing, the same way a title/status change already does.
func TestSave_ResetsIndexedAtWhenDescriptionChanges(t *testing.T) {
	repo, sqlDB := testRepo(t)
	ctx := context.Background()
	source, ref := "test-repo", "reindex-on-description-1"
	cleanupTender(t, sqlDB, source, ref)

	open := tender.Tender{Source: source, SourceRef: ref, Title: "Needs reindex", Status: tender.StatusOpen}
	if _, err := repo.Save(ctx, []tender.Tender{open}); err != nil {
		t.Fatalf("Save (insert): %v", err)
	}
	if _, err := sqlDB.ExecContext(ctx,
		`UPDATE tenders.ingested_tenders SET indexed_at = now() WHERE source = $1 AND source_ref = $2`,
		source, ref); err != nil {
		t.Fatalf("mark indexed: %v", err)
	}

	withDescription := open
	withDescription.Description = "Backfilled scope of work extracted from the raw payload."
	if _, err := repo.Save(ctx, []tender.Tender{withDescription}); err != nil {
		t.Fatalf("Save (description backfill): %v", err)
	}

	var got sql.NullTime
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT indexed_at FROM tenders.ingested_tenders WHERE source = $1 AND source_ref = $2`,
		source, ref).Scan(&got); err != nil {
		t.Fatalf("query indexed_at: %v", err)
	}
	if got.Valid {
		t.Fatalf("indexed_at = %v, want NULL after description changed from empty to non-empty", got.Time)
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

// tenderID resolves a saved tender's surrogate key, which every enrichment
// method takes as its handle.
func savedTenderID(t *testing.T, sqlDB *sql.DB, source, ref string) int64 {
	t.Helper()
	var id int64
	if err := sqlDB.QueryRow(
		`SELECT id FROM tenders.ingested_tenders WHERE source = $1 AND source_ref = $2`,
		source, ref).Scan(&id); err != nil {
		t.Fatalf("find tender id for %s/%s: %v", source, ref, err)
	}
	return id
}

// queueDepth counts the enrichment queue exactly as ListPendingXML defines it.
// Tests list with this as their limit rather than an arbitrary large number:
// the dev database holds real TED rows whose ids sort before the ones a test
// just inserted, so any smaller limit would truncate the test's own rows out of
// the result and fail for a reason that has nothing to do with the code.
func queueDepth(t *testing.T, sqlDB *sql.DB) int {
	t.Helper()
	var n int
	if err := sqlDB.QueryRow(
		`SELECT count(*) FROM tenders.ingested_tenders
		 WHERE source = 'ted' AND xml_fetched_at IS NULL AND xml_url <> ''`).Scan(&n); err != nil {
		t.Fatalf("count pending xml rows: %v", err)
	}
	return n
}

// The queue is defined by three predicates and each one excludes a row that
// would otherwise be listed forever with no way to advance: a non-TED row has
// no notice document to fetch, an already-read row has nothing left to read,
// and a row with no URL has no document to ask for.
func TestListPendingXML_ExcludesNonTEDReadAndURLlessRows(t *testing.T) {
	repo, sqlDB := testRepo(t)
	ctx := context.Background()

	const (
		pendingRef   = "test-enrich-pending"
		fetchedRef   = "test-enrich-fetched"
		noURLRef     = "test-enrich-no-url"
		otherSrcRef  = "test-enrich-other-source"
		otherSrcName = "test-repo"
	)
	cleanupTender(t, sqlDB, "ted", pendingRef)
	cleanupTender(t, sqlDB, "ted", fetchedRef)
	cleanupTender(t, sqlDB, "ted", noURLRef)
	cleanupTender(t, sqlDB, otherSrcName, otherSrcRef)

	if _, err := repo.Save(ctx, []tender.Tender{
		{
			Source: "ted", SourceRef: pendingRef, Title: "Queued", Status: tender.StatusOpen,
			PublicationNumber: "545620-2026", XMLURL: "https://ted.europa.eu/notice/545620-2026.xml",
		},
		{
			Source: "ted", SourceRef: fetchedRef, Title: "Already read", Status: tender.StatusOpen,
			PublicationNumber: "545621-2026", XMLURL: "https://ted.europa.eu/notice/545621-2026.xml",
		},
		{
			Source: "ted", SourceRef: noURLRef, Title: "No document to fetch", Status: tender.StatusOpen,
			PublicationNumber: "545622-2026",
		},
		{
			Source: otherSrcName, SourceRef: otherSrcRef, Title: "Different source", Status: tender.StatusOpen,
			XMLURL: "https://example.org/not-ted.xml",
		},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Take the "already read" row out of the queue the way a successful
	// enrichment does.
	if _, err := sqlDB.ExecContext(ctx,
		`UPDATE tenders.ingested_tenders SET xml_fetched_at = now(), xml_status = 'ok'
		 WHERE source = 'ted' AND source_ref = $1`, fetchedRef); err != nil {
		t.Fatalf("mark fetched: %v", err)
	}

	pending, err := repo.ListPendingXML(ctx, queueDepth(t, sqlDB))
	if err != nil {
		t.Fatalf("ListPendingXML: %v", err)
	}

	byID := make(map[int64]enrich.PendingNotice, len(pending))
	for _, n := range pending {
		byID[n.ID] = n
	}
	got, ok := byID[savedTenderID(t, sqlDB, "ted", pendingRef)]
	if !ok {
		t.Fatal("ListPendingXML did not return the queued TED notice")
	}
	if got.PublicationNumber != "545620-2026" || got.XMLURL != "https://ted.europa.eu/notice/545620-2026.xml" {
		t.Errorf("queued row = %+v, want the saved publication number and XML URL", got)
	}
	if got.Attempts != 0 {
		t.Errorf("Attempts = %d, want 0 on a row that has never been attempted", got.Attempts)
	}

	for _, excluded := range []struct {
		source, ref, why string
	}{
		{"ted", fetchedRef, "its notice document has already been read"},
		{"ted", noURLRef, "it has no notice document URL to fetch"},
		{otherSrcName, otherSrcRef, "it is not a TED row"},
	} {
		if _, listed := byID[savedTenderID(t, sqlDB, excluded.source, excluded.ref)]; listed {
			t.Errorf("ListPendingXML returned %s/%s, which must be excluded because %s",
				excluded.source, excluded.ref, excluded.why)
		}
	}
}

// ListPendingXML must expose the STORED attempt counter unmodified: the
// orchestrator adds one to it to decide whether the failure it is about to
// record is the one that exhausts the budget, so a repository that
// pre-incremented would park every row an attempt early.
func TestListPendingXML_ReportsStoredAttemptCount(t *testing.T) {
	repo, sqlDB := testRepo(t)
	ctx := context.Background()
	ref := "test-enrich-attempts"
	cleanupTender(t, sqlDB, "ted", ref)

	if _, err := repo.Save(ctx, []tender.Tender{{
		Source: "ted", SourceRef: ref, Title: "Retried", Status: tender.StatusOpen,
		PublicationNumber: "545623-2026", XMLURL: "https://ted.europa.eu/notice/545623-2026.xml",
	}}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	id := savedTenderID(t, sqlDB, "ted", ref)

	for i := 1; i <= 3; i++ {
		if err := repo.MarkXMLFailed(ctx, id, enrich.StatusUnavailable, false); err != nil {
			t.Fatalf("MarkXMLFailed (attempt %d): %v", i, err)
		}
	}

	pending, err := repo.ListPendingXML(ctx, queueDepth(t, sqlDB))
	if err != nil {
		t.Fatalf("ListPendingXML: %v", err)
	}
	for _, n := range pending {
		if n.ID != id {
			continue
		}
		if n.Attempts != 3 {
			t.Fatalf("Attempts = %d, want 3 (the stored count, not a pre-incremented one)", n.Attempts)
		}
		return
	}
	t.Fatal("ListPendingXML did not return the transiently-failed row, which must stay queued")
}

func TestMarkXMLFailed_TransientKeepsRowQueuedPermanentParksIt(t *testing.T) {
	repo, sqlDB := testRepo(t)
	ctx := context.Background()
	ref := "test-enrich-mark-failed"
	cleanupTender(t, sqlDB, "ted", ref)

	if _, err := repo.Save(ctx, []tender.Tender{{
		Source: "ted", SourceRef: ref, Title: "Fails", Status: tender.StatusOpen,
		PublicationNumber: "545624-2026", XMLURL: "https://ted.europa.eu/notice/545624-2026.xml",
	}}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	id := savedTenderID(t, sqlDB, "ted", ref)

	state := func() (sql.NullTime, string, int) {
		t.Helper()
		var (
			fetchedAt sql.NullTime
			status    sql.NullString
			attempts  int
		)
		if err := sqlDB.QueryRowContext(ctx,
			`SELECT xml_fetched_at, xml_status, xml_attempts FROM tenders.ingested_tenders WHERE id = $1`,
			id).Scan(&fetchedAt, &status, &attempts); err != nil {
			t.Fatalf("query xml state: %v", err)
		}
		return fetchedAt, status.String, attempts
	}

	// A transient failure spends an attempt but leaves the row in the queue —
	// that is what makes the queue self-healing without a retry scheduler.
	if err := repo.MarkXMLFailed(ctx, id, enrich.StatusUnavailable, false); err != nil {
		t.Fatalf("MarkXMLFailed (transient): %v", err)
	}
	fetchedAt, status, attempts := state()
	if fetchedAt.Valid {
		t.Fatalf("xml_fetched_at = %v after a transient failure, want NULL so the row stays queued", fetchedAt.Time)
	}
	if status != enrich.StatusUnavailable || attempts != 1 {
		t.Fatalf("after transient failure: status=%q attempts=%d, want %q and 1", status, attempts, enrich.StatusUnavailable)
	}

	// A permanent one stamps xml_fetched_at, which is the only thing that takes
	// a never-successful row out of the queue for good.
	if err := repo.MarkXMLFailed(ctx, id, enrich.StatusAbsent, true); err != nil {
		t.Fatalf("MarkXMLFailed (permanent): %v", err)
	}
	fetchedAt, status, attempts = state()
	if !fetchedAt.Valid {
		t.Fatal("xml_fetched_at is NULL after a permanent failure, so the row would be re-listed forever")
	}
	if status != enrich.StatusAbsent || attempts != 2 {
		t.Fatalf("after permanent failure: status=%q attempts=%d, want %q and 2", status, attempts, enrich.StatusAbsent)
	}
}

// enrichedDetail is a notice carrying one weightless notice-level criterion (the
// award *method*, the common real-world shape), one lot with a weighted grid,
// and the parties a bidder would need in order to challenge an award.
func enrichedDetail() tender.NoticeDetail {
	quality := 60.0
	lotValue := int64(5010851464)
	lotDeadline := time.Date(2026, 9, 10, 12, 30, 0, 0, time.FixedZone("CEST", 2*60*60))
	return tender.NoticeDetail{
		Criteria: []tender.AwardCriterion{
			{Ordinal: 1, Type: "quality", Name: "Offerta economicamente piu vantaggiosa", Lang: "ITA"},
		},
		Lots: []tender.Lot{{
			Ref: "LOT-0001", Title: "Lotto uno",
			// Per-lot value is the headline finding of reading the notice
			// document — the search payload cannot supply it — so it is part of
			// the fixture rather than an afterthought.
			CPV: "45233200", Value: &lotValue, Currency: "EUR", Deadline: &lotDeadline,
			CPVSecondary:  []string{"45233220", `quoted"code`},
			DocumentsURL:  "https://buyer.example/lot-1/documenti",
			SubmissionURL: "https://buyer.example/lot-1/offerte",
			Criteria: []tender.AwardCriterion{
				{LotRef: "LOT-0001", Ordinal: 1, Type: "quality", Name: "Qualita", Weight: &quality, WeightRaw: "60%", Lang: "ITA"},
				{LotRef: "LOT-0001", Ordinal: 2, Type: "price", Name: "Prezzo", Lang: "ITA"},
			},
		}},
		Organizations: []tender.Organization{{
			Ref: "ORG-0001", Name: "Comune di Roma", CompanyID: "02438750586",
			Roles: []string{"buyer", "appeal-information"},
			Email: "protocollo@pec.example.it", City: "Roma", Country: "ITA",
		}},
		DocumentsURL:  "https://buyer.example/documenti",
		SubmissionURL: "https://buyer.example/offerte",
	}
}

// TestSaveDetail_LotValueSurvivesTheNextSearchIngest is the regression test for
// the reason per-lot value was unstorable before this pass.
//
// Two writers share ingested_tender_lots: the hourly search-payload upsert and
// this enrichment. Only the second one ever learns a lot's value — buildLots
// leaves it nil deliberately, because the search payload's CPV array does not
// align with its lot array — so an unconditional `value = EXCLUDED.value` on
// the search side did not refresh the value, it erased it, within the hour,
// every hour. The column would have spent most of its life NULL and the
// headline finding of reading the notice document would have been invisible in
// production while passing every test that only ever wrote once.
//
// So the assertion that matters is the SECOND Save, not the first.
func TestSaveDetail_LotValueSurvivesTheNextSearchIngest(t *testing.T) {
	repo, sqlDB := testRepo(t)
	ctx := context.Background()
	source, ref := "test-repo", "enrich-lot-value-flap"
	cleanupTender(t, sqlDB, source, ref)

	searchPayload := []tender.Tender{{
		Source: source, SourceRef: ref, Title: "To enrich", Status: tender.StatusOpen,
		// Exactly what buildLots produces: a ref and a title, nothing else.
		Lots: []tender.Lot{{Ref: "LOT-0001", Title: "Lotto uno"}},
	}}
	if _, err := repo.Save(ctx, searchPayload); err != nil {
		t.Fatalf("Save: %v", err)
	}
	id := savedTenderID(t, sqlDB, source, ref)

	if err := repo.SaveDetail(ctx, id, enrichedDetail()); err != nil {
		t.Fatalf("SaveDetail: %v", err)
	}

	// The next hourly cycle, with the notice still returned by search.
	if _, err := repo.Save(ctx, searchPayload); err != nil {
		t.Fatalf("re-Save: %v", err)
	}

	var (
		value    sql.NullInt64
		currency string
		cpv      string
		deadline sql.NullTime
		title    string
	)
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT value, currency, cpv, deadline, title FROM tenders.ingested_tender_lots
		 WHERE tender_id = $1 AND ref = 'LOT-0001'`, id).
		Scan(&value, &currency, &cpv, &deadline, &title); err != nil {
		t.Fatalf("query lot after re-ingest: %v", err)
	}
	if !value.Valid || value.Int64 != 5010851464 {
		t.Errorf("value = %v after re-ingest, want the enriched figure preserved", value)
	}
	if currency != "EUR" || cpv != "45233200" || !deadline.Valid {
		t.Errorf("currency=%q cpv=%q deadline=%v after re-ingest, want the enriched values preserved",
			currency, cpv, deadline)
	}
	// The search payload stays authoritative for the one field it does carry,
	// so this is not "the enricher wins", it is "neither writer clears a field
	// it has no value for".
	if title != "Lotto uno" {
		t.Errorf("title = %q, want the search payload's title", title)
	}
}

func TestSaveDetail_WritesChildRowsAndTakesTheRowOutOfTheQueue(t *testing.T) {
	repo, sqlDB := testRepo(t)
	ctx := context.Background()
	source, ref := "test-repo", "enrich-save-detail"
	cleanupTender(t, sqlDB, source, ref)

	if _, err := repo.Save(ctx, []tender.Tender{{
		Source: source, SourceRef: ref, Title: "To enrich", Status: tender.StatusOpen,
		Lots: []tender.Lot{{Ref: "LOT-0001", Title: "Lotto uno"}},
	}}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	id := savedTenderID(t, sqlDB, source, ref)

	// Index it first, so the reset below is observable as a change rather than
	// as a column that was never set.
	if err := repo.MarkIndexed(ctx, id); err != nil {
		t.Fatalf("MarkIndexed: %v", err)
	}

	if err := repo.SaveDetail(ctx, id, enrichedDetail()); err != nil {
		t.Fatalf("SaveDetail: %v", err)
	}

	var (
		documentsURL, submissionURL string
		gridUsable                  sql.NullBool
		xmlStatus                   sql.NullString
		xmlFetchedAt, indexedAt     sql.NullTime
	)
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT documents_url, submission_url, grid_usable, xml_status, xml_fetched_at, indexed_at
		 FROM tenders.ingested_tenders WHERE id = $1`, id).
		Scan(&documentsURL, &submissionURL, &gridUsable, &xmlStatus, &xmlFetchedAt, &indexedAt); err != nil {
		t.Fatalf("query enriched tender: %v", err)
	}
	if documentsURL != "https://buyer.example/documenti" || submissionURL != "https://buyer.example/offerte" {
		t.Errorf("documents_url=%q submission_url=%q, want the notice-level portal URLs", documentsURL, submissionURL)
	}
	if !gridUsable.Valid || !gridUsable.Bool {
		t.Errorf("grid_usable = %v, want true — one lot criterion carries a weight", gridUsable)
	}
	if xmlStatus.String != enrich.StatusOK || !xmlFetchedAt.Valid {
		t.Errorf("xml_status=%q xml_fetched_at=%v, want %q and a timestamp", xmlStatus.String, xmlFetchedAt, enrich.StatusOK)
	}
	// Enrichment changes the text the tender is searched by, and the indexer's
	// own dirty-check cannot see that — without this reset the tender would keep
	// its pre-enrichment embedding forever.
	if indexedAt.Valid {
		t.Errorf("indexed_at = %v, want NULL so the enriched tender is re-embedded", indexedAt.Time)
	}

	// Notice-level and lot-level criteria partition the set; all three entries
	// are persisted exactly once, and the weightless ones keep a NULL weight
	// rather than being flattened to zero.
	var criteriaCount, weightedCount int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT count(*), count(weight) FROM tenders.ingested_tender_award_criteria WHERE tender_id = $1`,
		id).Scan(&criteriaCount, &weightedCount); err != nil {
		t.Fatalf("count criteria: %v", err)
	}
	if criteriaCount != 3 || weightedCount != 1 {
		t.Errorf("criteria = %d (%d weighted), want 3 and 1", criteriaCount, weightedCount)
	}

	var (
		critType, critName, weightRaw, lang string
		weight                              sql.NullFloat64
	)
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT type, name, weight, weight_raw, lang FROM tenders.ingested_tender_award_criteria
		 WHERE tender_id = $1 AND lot_ref = 'LOT-0001' AND ordinal = 1`, id).
		Scan(&critType, &critName, &weight, &weightRaw, &lang); err != nil {
		t.Fatalf("query weighted criterion: %v", err)
	}
	if critType != "quality" || critName != "Qualita" || !weight.Valid || weight.Float64 != 60 ||
		weightRaw != "60%" || lang != "ITA" {
		t.Errorf("weighted criterion = type=%q name=%q weight=%v raw=%q lang=%q, want the published values",
			critType, critName, weight, weightRaw, lang)
	}

	if err := sqlDB.QueryRowContext(ctx,
		`SELECT weight FROM tenders.ingested_tender_award_criteria
		 WHERE tender_id = $1 AND lot_ref = '' AND ordinal = 1`, id).Scan(&weight); err != nil {
		t.Fatalf("query notice-level criterion: %v", err)
	}
	if weight.Valid {
		t.Errorf("notice-level weight = %v, want NULL — an absent weight is not a zero weight", weight.Float64)
	}

	// roles reuses the same array encoder as cpv_secondary; index into the
	// column rather than parsing Postgres's array literal back in Go.
	var orgName, companyID, role1, role2, email string
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT name, company_id, roles[1], roles[2], email FROM tenders.ingested_tender_organizations
		 WHERE tender_id = $1 AND org_ref = 'ORG-0001'`, id).
		Scan(&orgName, &companyID, &role1, &role2, &email); err != nil {
		t.Fatalf("query organization: %v", err)
	}
	if orgName != "Comune di Roma" || companyID != "02438750586" ||
		role1 != "buyer" || role2 != "appeal-information" || email != "protocollo@pec.example.it" {
		t.Errorf("organization = name=%q company_id=%q roles=[%q %q] email=%q, want the published party",
			orgName, companyID, role1, role2, email)
	}

	// The lot keeps the title the ingest path wrote and gains every column only
	// the notice document carries — including value, which is the single field
	// this whole pass exists to recover.
	var lotTitle, lotDocs, lotSubmission, lotCPV, lotCPV1, lotCPV2 string
	var lotValue int64
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT title, documents_url, submission_url, cpv, value, cpv_secondary[1], cpv_secondary[2]
		 FROM tenders.ingested_tender_lots WHERE tender_id = $1 AND ref = 'LOT-0001'`, id).
		Scan(&lotTitle, &lotDocs, &lotSubmission, &lotCPV, &lotValue, &lotCPV1, &lotCPV2); err != nil {
		t.Fatalf("query enriched lot: %v", err)
	}
	if lotTitle != "Lotto uno" || lotDocs != "https://buyer.example/lot-1/documenti" ||
		lotSubmission != "https://buyer.example/lot-1/offerte" ||
		lotCPV != "45233200" || lotValue != 5010851464 ||
		lotCPV1 != "45233220" || lotCPV2 != `quoted"code` {
		t.Errorf("enriched lot = title=%q docs=%q submission=%q cpv=%q value=%d cpv_secondary=[%q %q], want the notice's lot detail",
			lotTitle, lotDocs, lotSubmission, lotCPV, lotValue, lotCPV1, lotCPV2)
	}

	var lotCount int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT count(*) FROM tenders.ingested_tender_lots WHERE tender_id = $1`, id).Scan(&lotCount); err != nil {
		t.Fatalf("count lots: %v", err)
	}
	if lotCount != 1 {
		t.Errorf("lot count = %d, want 1 — enrichment updates the ingested lot rather than duplicating it", lotCount)
	}
}

// The reason child rows are replaced instead of upserted. A corrected notice
// that publishes FEWER criteria than the one it supersedes must not leave the
// withdrawn ones behind: upserting on (tender_id, lot_ref, ordinal) would make
// the stored count only ever grow, and nothing about a leftover row looks wrong
// on its own.
func TestSaveDetail_ReplacesChildRowsRatherThanAccumulatingThem(t *testing.T) {
	repo, sqlDB := testRepo(t)
	ctx := context.Background()
	source, ref := "test-repo", "enrich-replace-children"
	cleanupTender(t, sqlDB, source, ref)

	if _, err := repo.Save(ctx, []tender.Tender{{
		Source: source, SourceRef: ref, Title: "Corrected notice", Status: tender.StatusOpen,
		Lots: []tender.Lot{{Ref: "LOT-0001", Title: "Lotto uno"}},
	}}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	id := savedTenderID(t, sqlDB, source, ref)

	if err := repo.SaveDetail(ctx, id, enrichedDetail()); err != nil {
		t.Fatalf("SaveDetail (first notice): %v", err)
	}

	// The republished notice drops the whole award grid and every party but the
	// buyer — the shape that exposes an accumulating writer.
	corrected := tender.NoticeDetail{
		Criteria: []tender.AwardCriterion{
			{Ordinal: 1, Type: "price", Name: "Prezzo piu basso", Lang: "ITA"},
		},
		Organizations: []tender.Organization{{Ref: "ORG-0001", Name: "Comune di Roma", Roles: []string{"buyer"}}},
		DocumentsURL:  "https://buyer.example/documenti-v2",
	}
	if err := repo.SaveDetail(ctx, id, corrected); err != nil {
		t.Fatalf("SaveDetail (corrected notice): %v", err)
	}

	var criteriaCount, orgCount int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT (SELECT count(*) FROM tenders.ingested_tender_award_criteria WHERE tender_id = $1),
		        (SELECT count(*) FROM tenders.ingested_tender_organizations WHERE tender_id = $1)`,
		id).Scan(&criteriaCount, &orgCount); err != nil {
		t.Fatalf("count child rows: %v", err)
	}
	if criteriaCount != 1 {
		t.Errorf("criteria count = %d, want 1 — the withdrawn criteria must not survive the correction", criteriaCount)
	}
	if orgCount != 1 {
		t.Errorf("organization count = %d, want 1 — the dropped parties must not survive the correction", orgCount)
	}

	// grid_usable is written in the same transaction as the rows it summarises,
	// so losing the weighted criterion must flip it, not leave a stale true
	// behind. NULL would be wrong too: the notice HAS been read.
	var gridUsable sql.NullBool
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT grid_usable FROM tenders.ingested_tenders WHERE id = $1`, id).Scan(&gridUsable); err != nil {
		t.Fatalf("query grid_usable: %v", err)
	}
	if !gridUsable.Valid {
		t.Fatal("grid_usable = NULL after enrichment, which means \"not looked at\" and is a different fact")
	}
	if gridUsable.Bool {
		t.Error("grid_usable = true after the weighted criteria were withdrawn, want false")
	}
}

// A notice publishing criteria with no weights at all is the trap this whole
// feature exists to make visible: it must read as false (we looked; there is no
// grid), never as true and never as NULL.
func TestSaveDetail_GridUsableFalseWhenNoCriterionCarriesAWeight(t *testing.T) {
	repo, sqlDB := testRepo(t)
	ctx := context.Background()
	source, ref := "test-repo", "enrich-grid-unusable"
	cleanupTender(t, sqlDB, source, ref)

	if _, err := repo.Save(ctx, []tender.Tender{{
		Source: source, SourceRef: ref, Title: "Method, not a grid", Status: tender.StatusOpen,
	}}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	id := savedTenderID(t, sqlDB, source, ref)

	if err := repo.SaveDetail(ctx, id, tender.NoticeDetail{
		Criteria: []tender.AwardCriterion{
			{Ordinal: 1, Type: "quality", Name: "Offerta economicamente piu vantaggiosa", Lang: "ITA"},
		},
	}); err != nil {
		t.Fatalf("SaveDetail: %v", err)
	}

	var gridUsable sql.NullBool
	var criteriaCount int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT grid_usable, (SELECT count(*) FROM tenders.ingested_tender_award_criteria WHERE tender_id = $1)
		 FROM tenders.ingested_tenders WHERE id = $1`, id).Scan(&gridUsable, &criteriaCount); err != nil {
		t.Fatalf("query grid_usable: %v", err)
	}
	if criteriaCount != 1 {
		t.Fatalf("criteria count = %d, want 1", criteriaCount)
	}
	if !gridUsable.Valid || gridUsable.Bool {
		t.Errorf("grid_usable = %v, want false — a criterion naming the award method is not a scoring grid", gridUsable)
	}
}

// The idempotency gate. TED republishes a corrected notice under a NEW
// publication number, so an unchanged number is proof the document we already
// read cannot have changed — which is what stops the enricher re-fetching the
// entire corpus every time a notice is re-ingested.
func TestSave_ClearsXMLQueueStateOnlyWhenThePublicationNumberChanges(t *testing.T) {
	repo, sqlDB := testRepo(t)
	ctx := context.Background()
	source, ref := "test-repo", "enrich-publication-number"
	cleanupTender(t, sqlDB, source, ref)

	notice := tender.Tender{
		Source: source, SourceRef: ref, Title: "Contract notice", Status: tender.StatusOpen,
		PublicationNumber: "545620-2026", XMLURL: "https://ted.europa.eu/notice/545620-2026.xml",
	}
	if _, err := repo.Save(ctx, []tender.Tender{notice}); err != nil {
		t.Fatalf("Save (insert): %v", err)
	}

	xmlState := func() (sql.NullTime, int, string, string) {
		t.Helper()
		var (
			fetchedAt         sql.NullTime
			attempts          int
			pubNumber, xmlURL sql.NullString
		)
		if err := sqlDB.QueryRowContext(ctx,
			`SELECT xml_fetched_at, xml_attempts, publication_number, xml_url
			 FROM tenders.ingested_tenders WHERE source = $1 AND source_ref = $2`,
			source, ref).Scan(&fetchedAt, &attempts, &pubNumber, &xmlURL); err != nil {
			t.Fatalf("query xml state: %v", err)
		}
		return fetchedAt, attempts, pubNumber.String, xmlURL.String
	}

	_, _, pubNumber, xmlURL := xmlState()
	if pubNumber != "545620-2026" || xmlURL != "https://ted.europa.eu/notice/545620-2026.xml" {
		t.Fatalf("after insert: publication_number=%q xml_url=%q, want the mapped values", pubNumber, xmlURL)
	}

	// Simulate an enrichment that succeeded after two transient failures.
	markEnriched := func() {
		t.Helper()
		if _, err := sqlDB.ExecContext(ctx,
			`UPDATE tenders.ingested_tenders SET xml_fetched_at = now(), xml_status = 'ok', xml_attempts = 2
			 WHERE source = $1 AND source_ref = $2`, source, ref); err != nil {
			t.Fatalf("simulate enrichment: %v", err)
		}
	}
	markEnriched()

	// Steady-state re-ingest of the same notice: nothing about the document
	// changed, so nothing is re-fetched.
	if _, err := repo.Save(ctx, []tender.Tender{notice}); err != nil {
		t.Fatalf("Save (unchanged re-ingest): %v", err)
	}
	fetchedAt, attempts, _, _ := xmlState()
	if !fetchedAt.Valid {
		t.Fatal("xml_fetched_at was cleared by an unchanged re-ingest — the whole corpus would be re-fetched every cycle")
	}
	if attempts != 2 {
		t.Errorf("xml_attempts = %d after an unchanged re-ingest, want the stored 2", attempts)
	}

	// The CN→CAN transition: source_ref is the procedure identifier, so the
	// award notice lands on this same row under a DIFFERENT publication number
	// and its document must be read over the call's.
	awarded := notice
	awarded.Status = tender.StatusAwarded
	awarded.PublicationNumber = "612345-2026"
	awarded.XMLURL = "https://ted.europa.eu/notice/612345-2026.xml"
	if _, err := repo.Save(ctx, []tender.Tender{awarded}); err != nil {
		t.Fatalf("Save (award notice): %v", err)
	}
	fetchedAt, attempts, pubNumber, xmlURL = xmlState()
	if fetchedAt.Valid {
		t.Errorf("xml_fetched_at = %v after the publication number changed, want NULL so the new notice is read",
			fetchedAt.Time)
	}
	if attempts != 0 {
		t.Errorf("xml_attempts = %d after the publication number changed, want 0 — a new document gets a fresh budget",
			attempts)
	}
	if pubNumber != "612345-2026" || xmlURL != "https://ted.europa.eu/notice/612345-2026.xml" {
		t.Errorf("after the award notice: publication_number=%q xml_url=%q, want the new notice's values",
			pubNumber, xmlURL)
	}
}

// TestSaveSelectionCriteria_ReplacesAndLeavesEnrichmentAlone pins the two
// properties that made this a separate method from SaveDetail: the set is
// REPLACED wholesale (so a corrected feed publishing fewer requirements does not
// leave the withdrawn ones behind), and the enrichment bookkeeping is UNTOUCHED
// (so a provider that publishes criteria in its search feed never claims the
// notice document was read).
func TestSaveSelectionCriteria_ReplacesAndLeavesEnrichmentAlone(t *testing.T) {
	repo, sqlDB := testRepo(t)
	ctx := context.Background()
	source, ref := "test-repo", "selection-1"
	cleanupTender(t, sqlDB, source, ref)

	if _, err := repo.Save(ctx, []tender.Tender{{Source: source, SourceRef: ref, Title: "ES folder", Status: tender.StatusOpen}}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	var id int64
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT id FROM tenders.ingested_tenders WHERE source = $1 AND source_ref = $2`, source, ref,
	).Scan(&id); err != nil {
		t.Fatalf("resolve tender id: %v", err)
	}

	three := []tender.SelectionCriterion{
		{Ordinal: 0, Category: "technical", Type: "OSR-TECH", Description: "Equipo minimo", Origin: "es-placsp", Lang: "SPA"},
		{Ordinal: 1, Category: "financial", Type: "5", Description: "Volumen anual de negocios", Origin: "es-placsp", Lang: "SPA"},
		{Ordinal: 2, Category: "declaration", Type: "1", Description: "Capacidad de obrar", Origin: "es-placsp", Lang: "SPA"},
	}
	if err := repo.SaveSelectionCriteria(ctx, source, map[string][]tender.SelectionCriterion{ref: three}); err != nil {
		t.Fatalf("SaveSelectionCriteria (first): %v", err)
	}

	var got int
	countRow := func() {
		t.Helper()
		if err := sqlDB.QueryRowContext(ctx,
			`SELECT count(*) FROM tenders.ingested_tender_selection_criteria WHERE tender_id = $1`, id,
		).Scan(&got); err != nil {
			t.Fatalf("count: %v", err)
		}
	}
	countRow()
	if got != 3 {
		t.Fatalf("after first write: %d rows, want 3", got)
	}

	// A corrected feed publishing FEWER requirements must shrink the set, which
	// is the whole reason this replaces instead of upserting.
	if err := repo.SaveSelectionCriteria(ctx, source, map[string][]tender.SelectionCriterion{ref: three[:1]}); err != nil {
		t.Fatalf("SaveSelectionCriteria (replace): %v", err)
	}
	countRow()
	if got != 1 {
		t.Errorf("after replacing with one criterion: %d rows, want 1 — the set is not being replaced", got)
	}

	var category, origin string
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT category, origin FROM tenders.ingested_tender_selection_criteria WHERE tender_id = $1`, id,
	).Scan(&category, &origin); err != nil {
		t.Fatalf("read surviving row: %v", err)
	}
	if category != "technical" || origin != "es-placsp" {
		t.Errorf("surviving row = (%q, %q), want (technical, es-placsp)", category, origin)
	}

	// The enrichment queue must not have moved: this tender has no notice
	// document and none was read.
	var fetchedAt sql.NullTime
	var status string
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT xml_fetched_at, coalesce(xml_status, '') FROM tenders.ingested_tenders WHERE id = $1`, id,
	).Scan(&fetchedAt, &status); err != nil {
		t.Fatalf("read enrichment bookkeeping: %v", err)
	}
	if fetchedAt.Valid || status != "" {
		t.Errorf("enrichment bookkeeping moved: xml_fetched_at=%v xml_status=%q — writing selection criteria must not claim the notice was read", fetchedAt, status)
	}
}
