package postgres_test

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/postgres"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

func testTenderRepo(t *testing.T) (*postgres.TenderRepo, *sql.DB) {
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

// insertTestTender writes directly into tenders.ingested_tenders (bypassing
// services/ingestion's own repo, which this test module doesn't import) so
// the search repo has real rows to query. Cleans itself up via t.Cleanup.
func insertTestTender(t *testing.T, sqlDB *sql.DB, sourceRef string, opts ...func(*testTenderRow)) int64 {
	t.Helper()
	row := testTenderRow{
		source: "test-repo", sourceRef: sourceRef, title: "Test tender " + sourceRef,
		buyerName: "Test Buyer", status: "open", procedureType: "open",
		country: "ITA", cpv: "45000000", currency: "EUR",
	}
	for _, o := range opts {
		o(&row)
	}
	var id int64
	err := sqlDB.QueryRow(
		`INSERT INTO tenders.ingested_tenders
		 (source, source_ref, title, buyer_name, status, procedure_type, country, cpv, value, currency, published_at, deadline, nuts)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		 RETURNING id`,
		row.source, row.sourceRef, row.title, row.buyerName, row.status, row.procedureType,
		row.country, row.cpv, row.value, row.currency, row.publishedAt, row.deadline, row.nuts,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insertTestTender: %v", err)
	}
	t.Cleanup(func() {
		_, _ = sqlDB.Exec(`DELETE FROM tenders.ingested_tenders WHERE id = $1`, id)
	})
	return id
}

type testTenderRow struct {
	source, sourceRef, title, buyerName, status, procedureType, country, cpv, currency, nuts string
	value                                                                                    *int64
	publishedAt, deadline                                                                    *time.Time
}

func withCountry(c string) func(*testTenderRow) { return func(r *testTenderRow) { r.country = c } }
func withStatus(s string) func(*testTenderRow)  { return func(r *testTenderRow) { r.status = s } }
func withPublishedAt(ts time.Time) func(*testTenderRow) {
	return func(r *testTenderRow) { r.publishedAt = &ts }
}
func withNUTS(n string) func(*testTenderRow) { return func(r *testTenderRow) { r.nuts = n } }

func TestSearchByFilters_FiltersByCountryAndOrdersByPublishedAtDesc(t *testing.T) {
	repo, sqlDB := testTenderRepo(t)
	ctx := context.Background()

	older := time.Now().Add(-48 * time.Hour)
	newer := time.Now().Add(-1 * time.Hour)
	idIT1 := insertTestTender(t, sqlDB, "search-1", withCountry("ITA"), withPublishedAt(older))
	idIT2 := insertTestTender(t, sqlDB, "search-2", withCountry("ITA"), withPublishedAt(newer))
	_ = insertTestTender(t, sqlDB, "search-3", withCountry("FRA"), withPublishedAt(newer))

	rows, err := repo.SearchByFilters(ctx, postgres.TenderFilters{Country: "ITA"}, 10, 0)
	if err != nil {
		t.Fatalf("SearchByFilters: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2 (only ITA tenders)", len(rows))
	}
	if rows[0].ID != idIT2 || rows[1].ID != idIT1 {
		t.Errorf("rows = [%d, %d], want [%d, %d] (newest published_at first)", rows[0].ID, rows[1].ID, idIT2, idIT1)
	}
}

func TestSearchByFilters_RespectsLimitAndOffset(t *testing.T) {
	repo, sqlDB := testTenderRepo(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		insertTestTender(t, sqlDB, "page-"+string(rune('a'+i)), withCountry("DEU"), withPublishedAt(time.Now().Add(-time.Duration(i)*time.Hour)))
	}

	page1, err := repo.SearchByFilters(ctx, postgres.TenderFilters{Country: "DEU"}, 2, 0)
	if err != nil {
		t.Fatalf("SearchByFilters page1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("len(page1) = %d, want 2", len(page1))
	}
	page2, err := repo.SearchByFilters(ctx, postgres.TenderFilters{Country: "DEU"}, 2, 2)
	if err != nil {
		t.Fatalf("SearchByFilters page2: %v", err)
	}
	if len(page2) != 1 {
		t.Fatalf("len(page2) = %d, want 1 (3 total, page size 2, offset 2)", len(page2))
	}
}

func TestFindByIDs_ReturnsOnlyMatchingIDsAndFilters(t *testing.T) {
	repo, sqlDB := testTenderRepo(t)
	ctx := context.Background()

	idMatch := insertTestTender(t, sqlDB, "ids-1", withStatus("open"))
	idWrongStatus := insertTestTender(t, sqlDB, "ids-2", withStatus("awarded"))
	_ = idWrongStatus

	rows, err := repo.FindByIDs(ctx, []int64{idMatch, idWrongStatus, 999999}, postgres.TenderFilters{Status: "open"})
	if err != nil {
		t.Fatalf("FindByIDs: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != idMatch {
		t.Errorf("rows = %+v, want exactly [id=%d] (status filter excludes idWrongStatus, 999999 doesn't exist)", rows, idMatch)
	}
}

func TestFindByIDs_EmptyIDsReturnsEmptyNoQuery(t *testing.T) {
	repo, _ := testTenderRepo(t)
	rows, err := repo.FindByIDs(context.Background(), nil, postgres.TenderFilters{})
	if err != nil {
		t.Fatalf("FindByIDs: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("len(rows) = %d, want 0", len(rows))
	}
}

func TestSearchTenders_RoundTripsStringIDs(t *testing.T) {
	repo, sqlDB := testTenderRepo(t)
	ctx := context.Background()
	insertTestTender(t, sqlDB, "domain-1", withCountry("ITA"), withPublishedAt(time.Now()))

	rows, err := repo.SearchTenders(ctx, tender.Filters{Country: "ITA"}, 10, 0)
	if err != nil {
		t.Fatalf("SearchTenders: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	if _, err := strconv.ParseInt(rows[0].ID, 10, 64); err != nil {
		t.Errorf("rows[0].ID = %q, want a valid decimal string", rows[0].ID)
	}
}

func TestSearchByFilters_ReturnsNUTS(t *testing.T) {
	repo, sqlDB := testTenderRepo(t)
	id := insertTestTender(t, sqlDB, "nuts-row", withCountry("ITA"), withNUTS("ITC4"))
	rows, err := repo.SearchByFilters(context.Background(), postgres.TenderFilters{Country: "ITA"}, 10, 0)
	if err != nil {
		t.Fatalf("SearchByFilters: %v", err)
	}
	var got *postgres.TenderResultRow
	for i := range rows {
		if rows[i].ID == id {
			got = &rows[i]
		}
	}
	if got == nil {
		t.Fatal("seeded row not returned")
	}
	if got.NUTS != "ITC4" {
		t.Fatalf("NUTS = %q, want ITC4", got.NUTS)
	}
}

// insertTestDocument writes a row into tenders.ingested_tender_documents for
// tenderID, mirroring insertTestTender's direct-INSERT bypass pattern.
func insertTestDocument(t *testing.T, sqlDB *sql.DB, tenderID int64, docType, url string) {
	t.Helper()
	var id int64
	err := sqlDB.QueryRow(
		`INSERT INTO tenders.ingested_tender_documents (tender_id, url, type) VALUES ($1, $2, $3) RETURNING id`,
		tenderID, url, docType,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insertTestDocument: %v", err)
	}
	t.Cleanup(func() {
		_, _ = sqlDB.Exec(`DELETE FROM tenders.ingested_tender_documents WHERE id = $1`, id)
	})
}

func TestSearchByFilters_JoinsNoticeDocumentURL(t *testing.T) {
	repo, sqlDB := testTenderRepo(t)
	ctx := context.Background()

	withDoc := insertTestTender(t, sqlDB, "doc-with-notice", withCountry("ITA"))
	insertTestDocument(t, sqlDB, withDoc, "notice", "https://ted.europa.eu/example/notice")
	insertTestDocument(t, sqlDB, withDoc, "spec", "https://ted.europa.eu/example/spec") // must NOT be picked
	withoutDoc := insertTestTender(t, sqlDB, "doc-without-notice", withCountry("ITA"))

	rows, err := repo.SearchByFilters(ctx, postgres.TenderFilters{Country: "ITA"}, 10, 0)
	if err != nil {
		t.Fatalf("SearchByFilters: %v", err)
	}

	var gotWith, gotWithout *postgres.TenderResultRow
	for i := range rows {
		switch rows[i].ID {
		case withDoc:
			gotWith = &rows[i]
		case withoutDoc:
			gotWithout = &rows[i]
		}
	}
	if gotWith == nil || gotWithout == nil {
		t.Fatalf("expected both seeded rows in results, got %d rows", len(rows))
	}
	if gotWith.SourceURL == nil || *gotWith.SourceURL != "https://ted.europa.eu/example/notice" {
		t.Fatalf("gotWith.SourceURL = %v, want the notice-type URL (not the spec one)", gotWith.SourceURL)
	}
	if gotWithout.SourceURL != nil {
		t.Fatalf("gotWithout.SourceURL = %v, want nil (no document ingested)", *gotWithout.SourceURL)
	}
}

func TestDistinctCountries_ReturnsDedupedNonEmpty(t *testing.T) {
	repo, sqlDB := testTenderRepo(t)
	insertTestTender(t, sqlDB, "it-1", withCountry("IT"))
	insertTestTender(t, sqlDB, "it-2", withCountry("IT")) // duplicate country
	insertTestTender(t, sqlDB, "pl-1", withCountry("PL"))
	got, err := repo.DistinctCountries(context.Background())
	if err != nil {
		t.Fatalf("DistinctCountries: %v", err)
	}
	set := map[string]bool{}
	for _, c := range got {
		set[c] = true
	}
	if !set["IT"] || !set["PL"] {
		t.Fatalf("want IT and PL, got %v", got)
	}
	// dedup: IT appears once
	n := 0
	for _, c := range got {
		if c == "IT" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("IT appears %d times, want 1 (deduped)", n)
	}
}

func TestEnrichTenders_RoundTripsStringIDs(t *testing.T) {
	repo, sqlDB := testTenderRepo(t)
	ctx := context.Background()
	id := insertTestTender(t, sqlDB, "domain-2", withStatus("open"))

	rows, err := repo.EnrichTenders(ctx, []string{strconv.FormatInt(id, 10)}, tender.Filters{Status: "open"})
	if err != nil {
		t.Fatalf("EnrichTenders: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != strconv.FormatInt(id, 10) {
		t.Errorf("rows = %+v, want exactly one tender with ID %d", rows, id)
	}
}
