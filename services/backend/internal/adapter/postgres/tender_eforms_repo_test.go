package postgres_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

// The eForms detail read paths, DB-gated the same way tender_detail_repo_test.go
// and tender_repo_test.go are (skipped without TEST_DATABASE_URL).
//
// These exist to check what a SQL-string test structurally cannot: that
// `weight numeric` really does round-trip as a *float64 through the ::float8
// projection with NULL preserved, that `grid_usable boolean` really does
// round-trip as a *bool — it is the first nullable boolean this service reads,
// and its three values are the whole point — and that the text[] columns
// survive array_to_string. Verified against PostgreSQL 16.

type testLotRow struct {
	title, cpv, currency        string
	value                       *int64
	cpvSecondary                []string
	documentsURL, submissionURL string
}

// insertTestLot writes one lot, mirroring insertTestTender's direct-INSERT
// bypass. cpv_secondary crosses as a text literal because database/sql has no
// portable array binding — the same reason the repo reads it back joined.
func insertTestLot(t *testing.T, sqlDB *sql.DB, tenderID int64, ref string, opts ...func(*testLotRow)) {
	t.Helper()
	row := testLotRow{title: "Lotto " + ref, cpv: "45000000", currency: "EUR"}
	for _, o := range opts {
		o(&row)
	}
	var id int64
	err := sqlDB.QueryRow(
		`INSERT INTO tenders.ingested_tender_lots
		 (tender_id, ref, title, cpv, value, currency, cpv_secondary, documents_url, submission_url)
		 VALUES ($1,$2,$3,$4,$5,$6,$7::text[],nullif($8,''),nullif($9,''))
		 RETURNING id`,
		tenderID, ref, row.title, row.cpv, row.value, row.currency,
		"{"+joinQuoted(row.cpvSecondary)+"}", row.documentsURL, row.submissionURL,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insertTestLot: %v", err)
	}
	t.Cleanup(func() {
		_, _ = sqlDB.Exec(`DELETE FROM tenders.ingested_tender_lots WHERE id = $1`, id)
	})
}

func insertTestCriterion(t *testing.T, sqlDB *sql.DB, tenderID int64, lotRef string, ordinal int,
	critType, name string, weight *float64, weightRaw, lang string,
) {
	t.Helper()
	var id int64
	err := sqlDB.QueryRow(
		`INSERT INTO tenders.ingested_tender_award_criteria
		 (tender_id, lot_ref, ordinal, type, name, description, weight, weight_raw, lang)
		 VALUES ($1,$2,$3,$4,$5,'',$6,$7,$8) RETURNING id`,
		tenderID, lotRef, ordinal, critType, name, weight, weightRaw, lang,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insertTestCriterion: %v", err)
	}
	t.Cleanup(func() {
		_, _ = sqlDB.Exec(`DELETE FROM tenders.ingested_tender_award_criteria WHERE id = $1`, id)
	})
}

func insertTestOrganization(t *testing.T, sqlDB *sql.DB, tenderID int64, ref, name string, roles []string) {
	t.Helper()
	var id int64
	err := sqlDB.QueryRow(
		`INSERT INTO tenders.ingested_tender_organizations
		 (tender_id, org_ref, name, company_id, roles, email, phone, website, city, country)
		 VALUES ($1,$2,$3,'01234567890',$4::text[],'protocollo@pec.example.it','','','Roma','ITA')
		 RETURNING id`,
		tenderID, ref, name, "{"+joinQuoted(roles)+"}",
	).Scan(&id)
	if err != nil {
		t.Fatalf("insertTestOrganization: %v", err)
	}
	t.Cleanup(func() {
		_, _ = sqlDB.Exec(`DELETE FROM tenders.ingested_tender_organizations WHERE id = $1`, id)
	})
}

// setTenderEForms writes the notice-level scalars the enricher fills in.
// gridUsable is a *bool on purpose: passing nil must leave the column NULL,
// which is the "not yet read" state the whole three-valued design rests on.
func setTenderEForms(t *testing.T, sqlDB *sql.DB, tenderID int64, description, pubNumber, docsURL, subURL string, gridUsable *bool) {
	t.Helper()
	if _, err := sqlDB.Exec(
		`UPDATE tenders.ingested_tenders
		    SET description = $2, publication_number = nullif($3,''),
		        documents_url = nullif($4,''), submission_url = nullif($5,''),
		        grid_usable = $6
		  WHERE id = $1`,
		tenderID, description, pubNumber, docsURL, subURL, gridUsable,
	); err != nil {
		t.Fatalf("setTenderEForms: %v", err)
	}
}

func float64p(v float64) *float64 { return &v }

// The nil weight is the COMMON case, and the one a numeric column will most
// easily corrupt: a scan into a plain float64 (or a coalesce in SQL) turns
// "no weight published" into "weighted zero".
func TestCriteriaByTenderID_PreservesAbsentWeightsAndOrder(t *testing.T) {
	repo, sqlDB := testTenderRepo(t)
	ctx := context.Background()

	id := insertTestTender(t, sqlDB, "criteria-weights")
	// Deliberately inserted out of order, so the ORDER BY is what fixes it.
	insertTestCriterion(t, sqlDB, id, "LOT-0001", 0, "quality", "Offerta tecnica", float64p(70), "70", "ITA")
	insertTestCriterion(t, sqlDB, id, "", 1, "cost", "Offerta economica", float64p(30.5), "30,5", "ITA")
	insertTestCriterion(t, sqlDB, id, "", 0, "quality", "Offerta economicamente più vantaggiosa", nil, "", "ITA")

	got, err := repo.CriteriaByTenderID(ctx, id)
	if err != nil {
		t.Fatalf("CriteriaByTenderID: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("criteria = %d, want 3", len(got))
	}
	// (lot_ref, ordinal): '' sorts first, so the notice-level grid leads.
	if got[0].LotRef != "" || got[0].Ordinal != 0 || got[1].LotRef != "" || got[1].Ordinal != 1 || got[2].LotRef != "LOT-0001" {
		t.Fatalf("order = %+v, want (lot_ref, ordinal) with notice-level first", got)
	}
	if got[0].HasWeight() {
		t.Errorf("the weightless award-method entry came back with weight %v — nil must survive", *got[0].Weight)
	}
	if got[1].Weight == nil || *got[1].Weight != 30.5 {
		t.Errorf("weight = %v, want 30.5 through the ::float8 cast", got[1].Weight)
	}
	if got[1].WeightRaw != "30,5" {
		t.Errorf("weight_raw = %q, want the published string verbatim", got[1].WeightRaw)
	}
	if got[2].Weight == nil || *got[2].Weight != 70 || got[2].Lang != "ITA" {
		t.Errorf("lot criterion = %+v, want weight 70 and lang ITA", got[2])
	}
}

func TestCriteriaByTenderID_NoCriteriaIsEmptyNotAnError(t *testing.T) {
	repo, sqlDB := testTenderRepo(t)
	id := insertTestTender(t, sqlDB, "criteria-none")
	got, err := repo.CriteriaByTenderID(context.Background(), id)
	if err != nil {
		t.Fatalf("CriteriaByTenderID: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("criteria = %+v, want none", got)
	}
}

func TestOrganizationsByTenderID_ReadsTheRolesArray(t *testing.T) {
	repo, sqlDB := testTenderRepo(t)
	id := insertTestTender(t, sqlDB, "orgs-roles")
	insertTestOrganization(t, sqlDB, id, "ORG-0001", "Comune di Roma", []string{"buyer"})
	insertTestOrganization(t, sqlDB, id, "ORG-0002", "TAR Lazio", []string{"appeal-body", "appeal-information"})

	got, err := repo.OrganizationsByTenderID(context.Background(), id)
	if err != nil {
		t.Fatalf("OrganizationsByTenderID: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("organizations = %d, want 2", len(got))
	}
	if got[0].Ref != "ORG-0001" || len(got[0].Roles) != 1 || got[0].Roles[0] != "buyer" {
		t.Errorf("buyer = %+v, want ORG-0001 with role buyer", got[0])
	}
	if len(got[1].Roles) != 2 || got[1].Roles[0] != "appeal-body" || got[1].Roles[1] != "appeal-information" {
		t.Errorf("appeal body roles = %v, want both roles through array_to_string", got[1].Roles)
	}
	if got[0].CompanyID != "01234567890" || got[0].City != "Roma" {
		t.Errorf("organization scalars = %+v, want the seeded values", got[0])
	}
}

func TestLotsByTenderID_ReadsSecondaryCPVAndBuyerLinks(t *testing.T) {
	repo, sqlDB := testTenderRepo(t)
	id := insertTestTender(t, sqlDB, "lots-eforms")
	insertTestLot(t, sqlDB, id, "LOT-0001", func(l *testLotRow) {
		l.cpvSecondary = []string{"45111000", "45112000"}
		l.documentsURL = "https://buyer.example/lotto-1/documenti"
		l.submissionURL = "https://buyer.example/lotto-1/offerte"
	})
	insertTestLot(t, sqlDB, id, "LOT-0002")

	got, err := repo.LotsByTenderID(context.Background(), id)
	if err != nil {
		t.Fatalf("LotsByTenderID: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("lots = %d, want 2", len(got))
	}
	if len(got[0].CPVSecondary) != 2 || got[0].CPVSecondary[0] != "45111000" {
		t.Errorf("cpv_secondary = %v, want both codes", got[0].CPVSecondary)
	}
	if got[0].DocumentsURL != "https://buyer.example/lotto-1/documenti" ||
		got[0].SubmissionURL != "https://buyer.example/lotto-1/offerte" {
		t.Errorf("lot links = (%q, %q), want the seeded URLs", got[0].DocumentsURL, got[0].SubmissionURL)
	}
	// A lot with no eForms detail must come back empty, not with a NULL scan
	// error — the columns are nullable because three other sources never
	// populate them.
	if got[1].CPVSecondary != nil || got[1].DocumentsURL != "" || got[1].SubmissionURL != "" {
		t.Errorf("unenriched lot = %+v, want empty eForms fields", got[1])
	}
}

// grid_usable is the first nullable boolean this service reads, and its three
// values are load-bearing: NULL must arrive as nil, not as false.
func TestFindDetailByID_GridUsableIsThreeValued(t *testing.T) {
	repo, sqlDB := testTenderRepo(t)
	ctx := context.Background()

	unread := insertTestTender(t, sqlDB, "grid-null")
	noGrid := insertTestTender(t, sqlDB, "grid-false")
	hasGrid := insertTestTender(t, sqlDB, "grid-true")
	no, yes := false, true
	setTenderEForms(t, sqlDB, unread, "", "", "", "", nil)
	setTenderEForms(t, sqlDB, noGrid, "", "545620-2026", "", "", &no)
	setTenderEForms(t, sqlDB, hasGrid, "", "548186-2026", "", "", &yes)

	for _, tc := range []struct {
		name string
		id   int64
		want *bool
	}{
		{"never read", unread, nil},
		{"read, no weighted criterion", noGrid, &no},
		{"read, weighted criteria present", hasGrid, &yes},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, err := repo.FindDetailByID(ctx, tc.id)
			if err != nil {
				t.Fatalf("FindDetailByID: %v", err)
			}
			switch {
			case tc.want == nil && d.GridUsable != nil:
				t.Errorf("GridUsable = %v, want nil — NULL means we have not looked", *d.GridUsable)
			case tc.want != nil && d.GridUsable == nil:
				t.Errorf("GridUsable = nil, want %v", *tc.want)
			case tc.want != nil && *d.GridUsable != *tc.want:
				t.Errorf("GridUsable = %v, want %v", *d.GridUsable, *tc.want)
			}
		})
	}
}

func TestFindDetailByID_HydratesTheEFormsDetail(t *testing.T) {
	repo, sqlDB := testTenderRepo(t)
	ctx := context.Background()

	id := insertTestTender(t, sqlDB, "detail-eforms")
	yes := true
	setTenderEForms(t, sqlDB, id, "Servizi di pulizia degli uffici comunali.", "545620-2026",
		"https://buyer.example/gara/documenti", "https://buyer.example/gara/offerte", &yes)
	insertTestLot(t, sqlDB, id, "LOT-0001")
	insertTestCriterion(t, sqlDB, id, "", 0, "quality", "Offerta tecnica", float64p(70), "70", "ITA")
	insertTestOrganization(t, sqlDB, id, "ORG-0001", "Comune di Roma", []string{"buyer"})

	d, err := repo.FindDetailByID(ctx, id)
	if err != nil {
		t.Fatalf("FindDetailByID: %v", err)
	}
	if d.Description != "Servizi di pulizia degli uffici comunali." {
		t.Errorf("Description = %q, want the seeded text in full", d.Description)
	}
	if d.PublicationNumber != "545620-2026" {
		t.Errorf("PublicationNumber = %q, want 545620-2026", d.PublicationNumber)
	}
	if d.DocumentsURL == "" || d.SubmissionURL == "" {
		t.Errorf("buyer links = (%q, %q), want both", d.DocumentsURL, d.SubmissionURL)
	}
	if len(d.Criteria) != 1 || len(d.Organizations) != 1 || len(d.Lots) != 1 {
		t.Fatalf("children = %d criteria / %d organizations / %d lots, want 1 each",
			len(d.Criteria), len(d.Organizations), len(d.Lots))
	}
	// The lot declares no criteria of its own, so it inherits the notice-level
	// grid — the publication pattern CriteriaForLot exists for.
	if got := d.CriteriaForLot("LOT-0001"); len(got) != 1 || got[0].Name != "Offerta tecnica" {
		t.Errorf("CriteriaForLot = %+v, want the inherited notice-level entry", got)
	}
}

// EnrichedAt must reflect a SUCCESSFUL read only — a terminal failure also
// stamps xml_fetched_at, and reporting that as an enrichment would say we read
// a notice we could not.
func TestFindDetailByID_EnrichedAtIgnoresTerminalFailures(t *testing.T) {
	repo, sqlDB := testTenderRepo(t)
	ctx := context.Background()

	ok := insertTestTender(t, sqlDB, "enriched-ok")
	failed := insertTestTender(t, sqlDB, "enriched-parse-error")
	never := insertTestTender(t, sqlDB, "enriched-never")
	markNoticeRead(t, sqlDB, ok, "ok", "")
	markNoticeRead(t, sqlDB, failed, "parse_error", "")

	d, err := repo.FindDetailByID(ctx, ok)
	if err != nil {
		t.Fatalf("FindDetailByID(ok): %v", err)
	}
	if d.EnrichedAt == nil {
		t.Error("EnrichedAt = nil for a successfully read notice")
	}
	if d, err = repo.FindDetailByID(ctx, failed); err != nil {
		t.Fatalf("FindDetailByID(parse_error): %v", err)
	} else if d.EnrichedAt != nil {
		t.Errorf("EnrichedAt = %v for a parse_error row, want nil", *d.EnrichedAt)
	}
	if d, err = repo.FindDetailByID(ctx, never); err != nil {
		t.Fatalf("FindDetailByID(never): %v", err)
	} else if d.EnrichedAt != nil {
		t.Errorf("EnrichedAt = %v for an unenriched row, want nil", *d.EnrichedAt)
	}
}

// The eForms scalars deliberately do NOT ride the search projection: they are
// created by services/ingestion's migration 0010, and naming them in the
// projection this service shares across all four search queries took every
// search down with SQLSTATE 42703 during the window before ingestion deployed.
//
// What search still carries is description (migration 0004, long settled). The
// eForms scalars are read per-tender by TenderDetail, which is what the proto
// exposes them on and what the tender-detail page and the agent tools use.
//
// This test now pins the ABSENCE, because the failure it guards against is a
// well-meaning future change moving them back onto the hot path for one fewer
// round trip. Search must survive a backend that is ahead of ingestion.
func TestSearchProjection_OmitsTheEFormsScalarsButKeepsDescription(t *testing.T) {
	repo, sqlDB := testTenderRepo(t)
	ctx := context.Background()

	id := insertTestTender(t, sqlDB, "search-eforms", withCountry("PRT"))
	no := false
	setTenderEForms(t, sqlDB, id, "Descrizione della gara.", "545620-2026",
		"https://buyer.example/documenti", "https://buyer.example/offerte", &no)

	rows, err := repo.SearchByFiltersRanked(ctx, tender.Filters{Countries: []string{"PRT"}}, tender.SortPublished, 10, 0)
	if err != nil {
		t.Fatalf("SearchByFiltersRanked: %v", err)
	}
	var enriched *tender.ScoredTender
	for i := range rows {
		if rows[i].ID == itoa(id) {
			enriched = &rows[i]
		}
	}
	if enriched == nil {
		t.Fatalf("expected the seeded row in results, got %d rows", len(rows))
	}
	if enriched.Description != "Descrizione della gara." {
		t.Errorf("Description = %q, want the seeded value — description DOES ride the projection",
			enriched.Description)
	}

	// The eForms scalars reach a caller through the detail fetch, not search.
	d, err := repo.FindDetailByID(ctx, id)
	if err != nil {
		t.Fatalf("FindDetailByID: %v", err)
	}
	if d.PublicationNumber != "545620-2026" {
		t.Errorf("detail PublicationNumber = %q, want the seeded value", d.PublicationNumber)
	}
	if d.DocumentsURL == "" || d.SubmissionURL == "" {
		t.Errorf("detail buyer links = (%q, %q), want both", d.DocumentsURL, d.SubmissionURL)
	}
	if d.GridUsable == nil || *d.GridUsable {
		t.Errorf("detail GridUsable = %v, want a non-nil false", d.GridUsable)
	}
}
