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
	requireIngestionSchema(t, sqlDB)
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

// itoa renders a seeded row's bigserial id the way the domain types carry it.
func itoa(id int64) string { return strconv.FormatInt(id, 10) }

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
func withCPV(c string) func(*testTenderRow)  { return func(r *testTenderRow) { r.cpv = c } }

func TestSearchByFiltersRanked_FiltersByCountryAndOrdersByPublishedAtDesc(t *testing.T) {
	repo, sqlDB := testTenderRepo(t)
	ctx := context.Background()

	older := time.Now().Add(-48 * time.Hour)
	newer := time.Now().Add(-1 * time.Hour)
	idIT1 := insertTestTender(t, sqlDB, "search-1", withCountry("ITA"), withPublishedAt(older))
	idIT2 := insertTestTender(t, sqlDB, "search-2", withCountry("ITA"), withPublishedAt(newer))
	_ = insertTestTender(t, sqlDB, "search-3", withCountry("FRA"), withPublishedAt(newer))

	rows, err := repo.SearchByFiltersRanked(ctx, tender.Filters{Countries: []string{"ITA"}}, tender.SortPublished, 10, 0)
	if err != nil {
		t.Fatalf("SearchByFiltersRanked: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2 (only ITA tenders)", len(rows))
	}
	if rows[0].ID != itoa(idIT2) || rows[1].ID != itoa(idIT1) {
		t.Errorf("rows = [%s, %s], want [%d, %d] (newest published_at first)", rows[0].ID, rows[1].ID, idIT2, idIT1)
	}
}

func TestSearchByFiltersRanked_RespectsLimitAndOffset(t *testing.T) {
	repo, sqlDB := testTenderRepo(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		insertTestTender(t, sqlDB, "page-"+string(rune('a'+i)), withCountry("DEU"), withPublishedAt(time.Now().Add(-time.Duration(i)*time.Hour)))
	}

	page1, err := repo.SearchByFiltersRanked(ctx, tender.Filters{Countries: []string{"DEU"}}, tender.SortPublished, 2, 0)
	if err != nil {
		t.Fatalf("SearchByFiltersRanked page1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("len(page1) = %d, want 2", len(page1))
	}
	page2, err := repo.SearchByFiltersRanked(ctx, tender.Filters{Countries: []string{"DEU"}}, tender.SortPublished, 2, 2)
	if err != nil {
		t.Fatalf("SearchByFiltersRanked page2: %v", err)
	}
	if len(page2) != 1 {
		t.Fatalf("len(page2) = %d, want 1 (3 total, page size 2, offset 2)", len(page2))
	}
}

func TestFindByIDsFiltered_ReturnsOnlyMatchingIDsAndFilters(t *testing.T) {
	repo, sqlDB := testTenderRepo(t)
	ctx := context.Background()

	idMatch := insertTestTender(t, sqlDB, "ids-1", withStatus("open"))
	idWrongStatus := insertTestTender(t, sqlDB, "ids-2", withStatus("awarded"))
	_ = idWrongStatus

	rows, err := repo.FindByIDsFiltered(ctx, []string{itoa(idMatch), itoa(idWrongStatus), "999999"}, tender.Filters{Statuses: []string{"open"}})
	if err != nil {
		t.Fatalf("FindByIDsFiltered: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != itoa(idMatch) {
		t.Errorf("rows = %+v, want exactly [id=%d] (status filter excludes idWrongStatus, 999999 doesn't exist)", rows, idMatch)
	}
}

func TestFindByIDsFiltered_EmptyIDsReturnsEmptyNoQuery(t *testing.T) {
	repo, _ := testTenderRepo(t)
	rows, err := repo.FindByIDsFiltered(context.Background(), nil, tender.Filters{})
	if err != nil {
		t.Fatalf("FindByIDsFiltered: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("len(rows) = %d, want 0", len(rows))
	}
}

func TestSearchTenders_RoundTripsStringIDs(t *testing.T) {
	repo, sqlDB := testTenderRepo(t)
	ctx := context.Background()
	insertTestTender(t, sqlDB, "domain-1", withCountry("ITA"), withPublishedAt(time.Now()))

	rows, err := repo.SearchTenders(ctx, tender.Filters{Countries: []string{"ITA"}}, tender.SortPublished, 10, 0)
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

func TestSearchByFiltersRanked_ReturnsNUTS(t *testing.T) {
	repo, sqlDB := testTenderRepo(t)
	id := insertTestTender(t, sqlDB, "nuts-row", withCountry("ITA"), withNUTS("ITC4"))
	rows, err := repo.SearchByFiltersRanked(context.Background(), tender.Filters{Countries: []string{"ITA"}}, tender.SortPublished, 10, 0)
	if err != nil {
		t.Fatalf("SearchByFiltersRanked: %v", err)
	}
	var got *tender.ScoredTender
	for i := range rows {
		if rows[i].ID == itoa(id) {
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

func TestSearchByFiltersRanked_JoinsNoticeDocumentURL(t *testing.T) {
	repo, sqlDB := testTenderRepo(t)
	ctx := context.Background()

	withDoc := insertTestTender(t, sqlDB, "doc-with-notice", withCountry("ITA"))
	insertTestDocument(t, sqlDB, withDoc, "notice", "https://ted.europa.eu/example/notice")
	insertTestDocument(t, sqlDB, withDoc, "spec", "https://ted.europa.eu/example/spec") // must NOT be picked
	withoutDoc := insertTestTender(t, sqlDB, "doc-without-notice", withCountry("ITA"))

	rows, err := repo.SearchByFiltersRanked(ctx, tender.Filters{Countries: []string{"ITA"}}, tender.SortPublished, 10, 0)
	if err != nil {
		t.Fatalf("SearchByFiltersRanked: %v", err)
	}

	var gotWith, gotWithout *tender.ScoredTender
	for i := range rows {
		switch rows[i].ID {
		case itoa(withDoc):
			gotWith = &rows[i]
		case itoa(withoutDoc):
			gotWithout = &rows[i]
		}
	}
	if gotWith == nil || gotWithout == nil {
		t.Fatalf("expected both seeded rows in results, got %d rows", len(rows))
	}
	if gotWith.SourceURL != "https://ted.europa.eu/example/notice" {
		t.Fatalf("gotWith.SourceURL = %q, want the notice-type URL (not the spec one)", gotWith.SourceURL)
	}
	if gotWithout.SourceURL != "" {
		t.Fatalf("gotWithout.SourceURL = %q, want empty (no document ingested)", gotWithout.SourceURL)
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

// LexicalSearch adds a trigram similarity arm (the `%` operator) whenever the
// query is short enough to be name-shaped (see trigramQueryMaxLen). pg_trgm is
// installed in the `tenders` schema, not `public` — if the connection's
// search_path doesn't include `tenders`, Postgres can't resolve the `%`
// operator at all and every short query errors, silently degrading hybrid
// search to dense-only for essentially every real user query. This is the
// regression test for that: before the search_path fix it fails with
// "could not choose a best candidate operator for `%` operator"; after, it
// must return without error.
func TestLexicalSearch_ShortQueryDoesNotErrorOnTrigramOperator(t *testing.T) {
	repo, sqlDB := testTenderRepo(t)
	ctx := context.Background()
	insertTestTender(t, sqlDB, "lexical-trigram-1", withCountry("ITA"))

	// "comune bergam" is 13 runes — well under trigramQueryMaxLen (60) — so
	// LexicalSearch adds the `t.title % $n` / `t.buyer_name % $n` arm.
	_, err := repo.LexicalSearch(ctx, tender.LexicalQuery{Text: "comune bergam"}, tender.Filters{}, 10)
	if err != nil {
		t.Fatalf("LexicalSearch(short query) error = %v, want nil — the trigram `%%` operator must resolve via search_path", err)
	}
}

func TestEnrichTenders_RoundTripsStringIDs(t *testing.T) {
	repo, sqlDB := testTenderRepo(t)
	ctx := context.Background()
	id := insertTestTender(t, sqlDB, "domain-2", withStatus("open"))

	rows, err := repo.EnrichTenders(ctx, []string{strconv.FormatInt(id, 10)}, tender.Filters{Statuses: []string{"open"}})
	if err != nil {
		t.Fatalf("EnrichTenders: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != strconv.FormatInt(id, 10) {
		t.Errorf("rows = %+v, want exactly one tender with ID %d", rows, id)
	}
}

// FindByCPVPrefixes used to order strictly by recency; Task 14/15 found that
// this discarded the actually-relevant tender whenever its own CPV category
// held enough rows to push it past the retrieval arm's small candidate
// window, however precisely the resolved code was truncated. The fix ranks by
// the CALLER'S code order first (cpvPrefixRanking — see tender_search.go and
// its SQL-shape unit tests) and recency only breaks a tie within one code.
// This is the end-to-end check that the database actually orders rows that
// way, not just that the generated SQL looks right.
func TestFindByCPVPrefixes_RanksTheCallersBestCodeAboveNewerWeakerMatches(t *testing.T) {
	repo, sqlDB := testTenderRepo(t)
	ctx := context.Background()

	// A country code no real ingested data uses, so this test's result set is
	// isolated from anything already in the database regardless of what other
	// tenders happen to carry these CPV prefixes.
	const country = "ZZ"
	older := time.Now().Add(-72 * time.Hour)
	newer := time.Now().Add(-1 * time.Hour)
	newest := time.Now()

	// Both share codes[0] ("9091"), the caller's BEST-ranked resolved code.
	idOldBest := insertTestTender(t, sqlDB, "cpv-rank-old-best", withCountry(country), withCPV("90911200"), withPublishedAt(older))
	idNewBest := insertTestTender(t, sqlDB, "cpv-rank-new-best", withCountry(country), withCPV("90912000"), withPublishedAt(newer))
	// Matches only codes[1] ("4521"), the caller's WEAKER, second-ranked code —
	// but it is the newest row of the three. Recency-only ordering (the bug
	// this guards against) would put it first; correct ordering must not.
	idNewestWeak := insertTestTender(t, sqlDB, "cpv-rank-newest-weak", withCountry(country), withCPV("45210000"), withPublishedAt(newest))

	got, err := repo.FindByCPVPrefixes(ctx, []string{"9091", "4521"}, tender.Filters{Countries: []string{country}}, 10)
	if err != nil {
		t.Fatalf("FindByCPVPrefixes: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3", len(got))
	}

	gotIDs := make([]string, len(got))
	for i, r := range got {
		gotIDs[i] = r.ID
	}
	want := []string{itoa(idNewBest), itoa(idOldBest), itoa(idNewestWeak)}
	for i := range want {
		if gotIDs[i] != want[i] {
			t.Fatalf("order = %v, want %v — the caller's best code must rank above its weaker code regardless of recency, and recency must still break the tie WITHIN the best code", gotIDs, want)
		}
	}
}
