package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/postgres"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/document"
)

// These tests are DB-gated in the same way tender_repo_test.go's are. They
// exist to check the things a SQL-string test structurally cannot: that the
// statements are valid against the real ingestion schema, and that the
// nullable columns actually round-trip as pointers through the driver. The
// mapping logic itself is covered without a database in
// document_repo_internal_test.go, because CI has no Postgres.
func testDocumentRepo(t *testing.T) (*postgres.DocumentRepo, *sql.DB) {
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
	return postgres.NewDocumentRepo(db), sqlDB
}

// insertTestDocumentReturningID mirrors insertTestDocument but hands back the
// row id, which the parts table needs as its foreign key.
func insertTestDocumentReturningID(t *testing.T, sqlDB *sql.DB, tenderID int64, docType, url string) int64 {
	t.Helper()
	var id int64
	err := sqlDB.QueryRow(
		`INSERT INTO tenders.ingested_tender_documents (tender_id, url, type) VALUES ($1, $2, $3) RETURNING id`,
		tenderID, url, docType,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insertTestDocumentReturningID: %v", err)
	}
	t.Cleanup(func() {
		_, _ = sqlDB.Exec(`DELETE FROM tenders.ingested_tender_documents WHERE id = $1`, id)
	})
	return id
}

// insertTestPart writes one extracted part. Provenance is left NULL, which is
// the state of every document extracted before those columns existed.
func insertTestPart(t *testing.T, sqlDB *sql.DB, documentID int64, index int, content string) {
	t.Helper()
	var id int64
	err := sqlDB.QueryRow(
		`INSERT INTO tenders.ingested_tender_document_parts (document_id, index, content)
		 VALUES ($1, $2, $3) RETURNING id`,
		documentID, index, content,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insertTestPart: %v", err)
	}
	t.Cleanup(func() {
		_, _ = sqlDB.Exec(`DELETE FROM tenders.ingested_tender_document_parts WHERE id = $1`, id)
	})
}

// insertTestPartWithProvenance writes a part carrying the page/section
// metadata the ingestion pass populates going forward.
func insertTestPartWithProvenance(t *testing.T, sqlDB *sql.DB, documentID int64, index int, content string,
	pageStart, pageEnd int, sectionPath []string, sectionTitle string, hasTable bool,
) {
	t.Helper()
	var id int64
	// section_path crosses as a text literal for the same reason the ingestion
	// repo writes it that way: database/sql has no portable array binding.
	pathLiteral := "{" + joinQuoted(sectionPath) + "}"
	err := sqlDB.QueryRow(
		`INSERT INTO tenders.ingested_tender_document_parts
		 (document_id, index, content, page_start, page_end, section_path, section_title, has_table)
		 VALUES ($1,$2,$3,$4,$5,$6::text[],$7,$8) RETURNING id`,
		documentID, index, content, pageStart, pageEnd, pathLiteral, sectionTitle, hasTable,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insertTestPartWithProvenance: %v", err)
	}
	t.Cleanup(func() {
		_, _ = sqlDB.Exec(`DELETE FROM tenders.ingested_tender_document_parts WHERE id = $1`, id)
	})
}

func joinQuoted(vals []string) string {
	out := ""
	for i, v := range vals {
		if i > 0 {
			out += ","
		}
		out += `"` + v + `"`
	}
	return out
}

// markNoticeRead stamps the enrichment columns the way a successful enricher
// run would, so the coverage answer has real state to derive from.
func markNoticeRead(t *testing.T, sqlDB *sql.DB, tenderID int64, status, documentsURL string) {
	t.Helper()
	if _, err := sqlDB.Exec(
		`UPDATE tenders.ingested_tenders
		    SET xml_fetched_at = now(), xml_status = $2, documents_url = nullif($3, '')
		  WHERE id = $1`,
		tenderID, status, documentsURL,
	); err != nil {
		t.Fatalf("markNoticeRead: %v", err)
	}
}

func markIndexed(t *testing.T, sqlDB *sql.DB, tenderID int64) {
	t.Helper()
	if _, err := sqlDB.Exec(`UPDATE tenders.ingested_tenders SET indexed_at = now() WHERE id = $1`, tenderID); err != nil {
		t.Fatalf("markIndexed: %v", err)
	}
}

func TestDocumentAvailability_UnknownTender(t *testing.T) {
	repo, _ := testDocumentRepo(t)
	if _, err := repo.Availability(context.Background(), 999999999); !errors.Is(err, document.ErrTenderNotFound) {
		t.Errorf("Availability(unknown) = %v, want ErrTenderNotFound", err)
	}
}

// A freshly ingested tender nobody has enriched: no notice read, no links, no
// documents. The only honest answer is "we have not looked".
func TestDocumentAvailability_UnenrichedTenderReportsIgnorance(t *testing.T) {
	repo, sqlDB := testDocumentRepo(t)
	id := insertTestTender(t, sqlDB, "avail-unenriched")

	got, err := repo.Availability(context.Background(), id)
	if err != nil {
		t.Fatalf("Availability: %v", err)
	}
	if got.Coverage != document.CoverageNone || got.Reason != document.ReasonNoticeNotRead {
		t.Errorf("(coverage, reason) = (%q, %q), want (none, notice_not_read)", got.Coverage, got.Reason)
	}
	if !got.Consistent() {
		t.Errorf("Consistent() = false for %+v", got)
	}
}

// The bucket most Italian tenders land in: we read the notice, it named a
// documents page, and we hold only the notice PDF's text. This must never
// report as "there are no documents".
func TestDocumentAvailability_NoticeTextWithAnUnfollowedLink(t *testing.T) {
	repo, sqlDB := testDocumentRepo(t)
	id := insertTestTender(t, sqlDB, "avail-notice-only")
	markNoticeRead(t, sqlDB, id, "ok", "https://buyer.example/gara/documenti")
	docID := insertTestDocumentReturningID(t, sqlDB, id, "notice", "https://ted.europa.eu/avail/notice.pdf")
	insertTestPart(t, sqlDB, docID, 0, "Oggetto dell'appalto: servizi di pulizia.")
	markIndexed(t, sqlDB, id)

	got, err := repo.Availability(context.Background(), id)
	if err != nil {
		t.Fatalf("Availability: %v", err)
	}
	if got.Coverage != document.CoverageNotice || got.Reason != document.ReasonBodyNotRetrieved {
		t.Errorf("(coverage, reason) = (%q, %q), want (notice_only, body_not_retrieved)", got.Coverage, got.Reason)
	}
	if got.KnownDocumentLinks != 1 || got.ExtractedDocuments != 1 || got.ExtractedParts != 1 {
		t.Errorf("evidence = %+v, want 1 link / 1 document / 1 part", got)
	}
	if !got.Consistent() {
		t.Errorf("Consistent() = false for %+v", got)
	}
}

// A parse_error stamps xml_fetched_at but proves nothing about what the buyer
// published — so it must not reach the absence claim.
func TestDocumentAvailability_ParseErrorIsNotAReadNotice(t *testing.T) {
	repo, sqlDB := testDocumentRepo(t)
	id := insertTestTender(t, sqlDB, "avail-parse-error")
	markNoticeRead(t, sqlDB, id, "parse_error", "")

	got, err := repo.Availability(context.Background(), id)
	if err != nil {
		t.Fatalf("Availability: %v", err)
	}
	if got.NoticeRead {
		t.Error("NoticeRead = true for a parse_error row")
	}
	if got.Reason != document.ReasonNoticeNotRead {
		t.Errorf("reason = %q, want notice_not_read — a parse_error may never assert absence", got.Reason)
	}
}

// The one state that IS allowed to assert absence: read successfully, no link.
func TestDocumentAvailability_ReadNoticeWithNoLinkMayAssertAbsence(t *testing.T) {
	repo, sqlDB := testDocumentRepo(t)
	id := insertTestTender(t, sqlDB, "avail-nothing-published")
	markNoticeRead(t, sqlDB, id, "ok", "")

	got, err := repo.Availability(context.Background(), id)
	if err != nil {
		t.Fatalf("Availability: %v", err)
	}
	if got.Reason != document.ReasonNoDocumentsPublished {
		t.Errorf("reason = %q, want no_documents_published", got.Reason)
	}
	if !got.Consistent() {
		t.Errorf("Consistent() = false for %+v", got)
	}
}

// Text from a non-notice document is what raises coverage to full — that is
// the distinction the spec_part_rows subquery exists to draw.
func TestDocumentAvailability_SpecificationTextReachesFullCoverage(t *testing.T) {
	repo, sqlDB := testDocumentRepo(t)
	id := insertTestTender(t, sqlDB, "avail-full")
	markNoticeRead(t, sqlDB, id, "ok", "https://buyer.example/gara/documenti")
	noticeDoc := insertTestDocumentReturningID(t, sqlDB, id, "notice", "https://ted.europa.eu/avail-full/notice.pdf")
	insertTestPart(t, sqlDB, noticeDoc, 0, "Bando di gara.")
	specDoc := insertTestDocumentReturningID(t, sqlDB, id, "spec", "https://buyer.example/disciplinare.pdf")
	insertTestPart(t, sqlDB, specDoc, 0, "Requisiti di capacità tecnica e professionale.")

	got, err := repo.Availability(context.Background(), id)
	if err != nil {
		t.Fatalf("Availability: %v", err)
	}
	if got.Coverage != document.CoverageFull || got.Reason != document.ReasonNone {
		t.Errorf("(coverage, reason) = (%q, %q), want (full, \"\")", got.Coverage, got.Reason)
	}
	if got.ExtractedParts != 2 || got.ExtractedDocuments != 2 {
		t.Errorf("evidence = %+v, want 2 documents / 2 parts", got)
	}
}

// A document row with no parts and no indexing pass yet is transient, and must
// say so rather than reporting the tender as unreadable.
func TestDocumentAvailability_PendingExtractionIsTransient(t *testing.T) {
	repo, sqlDB := testDocumentRepo(t)
	id := insertTestTender(t, sqlDB, "avail-pending")
	markNoticeRead(t, sqlDB, id, "ok", "")
	insertTestDocumentReturningID(t, sqlDB, id, "notice", "https://ted.europa.eu/avail-pending/notice.pdf")

	got, err := repo.Availability(context.Background(), id)
	if err != nil {
		t.Fatalf("Availability: %v", err)
	}
	if got.Reason != document.ReasonNotYetExtracted {
		t.Errorf("reason = %q, want not_yet_extracted", got.Reason)
	}

	markIndexed(t, sqlDB, id)
	got, err = repo.Availability(context.Background(), id)
	if err != nil {
		t.Fatalf("Availability after indexing: %v", err)
	}
	if got.Reason != document.ReasonExtractionFailed {
		t.Errorf("reason = %q after the indexer ran, want extraction_failed", got.Reason)
	}
}

// A per-lot documents_url counts as a held link too: the buyer published it,
// so absence is refuted whether or not the notice carries one of its own.
func TestDocumentAvailability_CountsPerLotLinks(t *testing.T) {
	repo, sqlDB := testDocumentRepo(t)
	id := insertTestTender(t, sqlDB, "avail-lot-links")
	markNoticeRead(t, sqlDB, id, "ok", "")
	insertTestLot(t, sqlDB, id, "LOT-0001", func(l *testLotRow) {
		l.documentsURL = "https://buyer.example/lotto-1"
	})

	got, err := repo.Availability(context.Background(), id)
	if err != nil {
		t.Fatalf("Availability: %v", err)
	}
	if got.KnownDocumentLinks != 1 {
		t.Errorf("KnownDocumentLinks = %d, want 1 (the lot's link)", got.KnownDocumentLinks)
	}
	if got.Reason != document.ReasonBodyNotRetrieved {
		t.Errorf("reason = %q, want body_not_retrieved", got.Reason)
	}
}

func TestSearchParts_RanksAndScopesToOneTender(t *testing.T) {
	repo, sqlDB := testDocumentRepo(t)
	ctx := context.Background()

	mine := insertTestTender(t, sqlDB, "parts-mine")
	other := insertTestTender(t, sqlDB, "parts-other")
	mineDoc := insertTestDocumentReturningID(t, sqlDB, mine, "spec", "https://buyer.example/mine.pdf")
	otherDoc := insertTestDocumentReturningID(t, sqlDB, other, "spec", "https://buyer.example/other.pdf")

	insertTestPart(t, sqlDB, mineDoc, 0, "Le penali per ritardo sono applicate giornalmente. Penali massime dieci per cento.")
	insertTestPart(t, sqlDB, mineDoc, 1, "Modalità di fatturazione elettronica e termini di pagamento.")
	insertTestPart(t, sqlDB, otherDoc, 0, "Penali per ritardo di un altro appalto, che non deve comparire.")

	got, err := repo.SearchParts(ctx, mine, "penali ritardo", 6)
	if err != nil {
		t.Fatalf("SearchParts: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("excerpts = %d, want exactly the one matching part of THIS tender", len(got))
	}
	if got[0].Citation.DocumentURL != "https://buyer.example/mine.pdf" {
		t.Errorf("citation URL = %q, want this tender's document", got[0].Citation.DocumentURL)
	}
	if got[0].Citation.DocumentType != "spec" || got[0].Citation.PartIndex != 0 {
		t.Errorf("citation = %+v, want type=spec index=0", got[0].Citation)
	}
	if got[0].RelevanceScore <= 0 || got[0].RelevanceScore >= 1 {
		t.Errorf("relevance = %v, want a rank normalised into (0,1)", got[0].RelevanceScore)
	}
}

// The corpus is full of parts extracted before the provenance columns existed.
// A citation must degrade to the document URL rather than inventing page 0.
func TestSearchParts_DegradesWhenProvenanceIsNull(t *testing.T) {
	repo, sqlDB := testDocumentRepo(t)
	id := insertTestTender(t, sqlDB, "parts-no-provenance")
	docID := insertTestDocumentReturningID(t, sqlDB, id, "notice", "https://ted.europa.eu/no-prov/notice.pdf")
	insertTestPart(t, sqlDB, docID, 0, "Requisiti di capacità economica e finanziaria richiesti al concorrente.")

	got, err := repo.SearchParts(context.Background(), id, "capacità economica", 6)
	if err != nil {
		t.Fatalf("SearchParts: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("excerpts = %d, want 1", len(got))
	}
	c := got[0].Citation
	if c.PageStart != nil || c.PageEnd != nil {
		t.Errorf("pages = (%v, %v), want nil — an unknown page is not page zero", c.PageStart, c.PageEnd)
	}
	if c.SectionPath != nil || c.SectionTitle != "" || c.HasTable {
		t.Errorf("section provenance = %+v, want empty", c)
	}
	if c.DocumentURL == "" {
		t.Error("the document URL must survive even when everything else is unknown")
	}
}

func TestSearchParts_ReadsProvenanceWhenPresent(t *testing.T) {
	repo, sqlDB := testDocumentRepo(t)
	id := insertTestTender(t, sqlDB, "parts-provenance")
	docID := insertTestDocumentReturningID(t, sqlDB, id, "spec", "https://buyer.example/disciplinare.pdf")
	insertTestPartWithProvenance(t, sqlDB, docID, 3,
		"Tabella dei requisiti di capacità tecnica richiesti.",
		14, 16, []string{"7", "7.2"}, "Requisiti di capacità tecnica", true)

	got, err := repo.SearchParts(context.Background(), id, "requisiti capacità tecnica", 6)
	if err != nil {
		t.Fatalf("SearchParts: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("excerpts = %d, want 1", len(got))
	}
	c := got[0].Citation
	if c.PageStart == nil || *c.PageStart != 14 || c.PageEnd == nil || *c.PageEnd != 16 {
		t.Errorf("pages = (%v, %v), want (14, 16)", c.PageStart, c.PageEnd)
	}
	if len(c.SectionPath) != 2 || c.SectionPath[0] != "7" || c.SectionPath[1] != "7.2" {
		t.Errorf("section path = %v, want [7 7.2]", c.SectionPath)
	}
	if c.SectionTitle != "Requisiti di capacità tecnica" || !c.HasTable || c.PartIndex != 3 {
		t.Errorf("citation = %+v, want the seeded provenance", c)
	}
}

// The limit is a hard bound on what reaches a model's context; the adapter
// must honour it even though the service already clamps it.
func TestSearchParts_HonoursTheLimit(t *testing.T) {
	repo, sqlDB := testDocumentRepo(t)
	id := insertTestTender(t, sqlDB, "parts-limit")
	docID := insertTestDocumentReturningID(t, sqlDB, id, "spec", "https://buyer.example/limit.pdf")
	for i := 0; i < 5; i++ {
		insertTestPart(t, sqlDB, docID, i, "Penali applicabili al concorrente aggiudicatario, comma successivo.")
	}

	got, err := repo.SearchParts(context.Background(), id, "penali", 2)
	if err != nil {
		t.Fatalf("SearchParts: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("excerpts = %d, want 2", len(got))
	}
}

// No match is not an error — it is a different answer, and the coverage half
// of the result is what explains it.
func TestSearchParts_NoMatchIsNotAnError(t *testing.T) {
	repo, sqlDB := testDocumentRepo(t)
	id := insertTestTender(t, sqlDB, "parts-no-match")
	docID := insertTestDocumentReturningID(t, sqlDB, id, "spec", "https://buyer.example/nomatch.pdf")
	insertTestPart(t, sqlDB, docID, 0, "Oggetto: fornitura di arredi scolastici.")

	got, err := repo.SearchParts(context.Background(), id, "zzzznonesistente", 6)
	if err != nil {
		t.Fatalf("SearchParts: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("excerpts = %d, want 0", len(got))
	}
}
