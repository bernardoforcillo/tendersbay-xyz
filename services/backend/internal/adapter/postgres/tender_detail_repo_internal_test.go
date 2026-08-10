package postgres

import (
	"strings"
	"testing"
	"time"
)

func strp(s string) *string { return &s }

// xml_fetched_at is stamped on permanent FAILURE as well as on success, so it
// cannot stand alone for "we read this notice". Reporting a parse_error row as
// enriched would tell a reader we have read a notice we demonstrably could
// not — the inaccessible-versus-absent conflation in its smallest form.
func TestEnrichedAt_OnlyASuccessfulReadCounts(t *testing.T) {
	ts := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)

	cases := []struct {
		name      string
		fetchedAt *time.Time
		status    *string
		want      *time.Time
	}{
		{"never attempted", nil, nil, nil},
		{"read successfully", &ts, strp("ok"), &ts},
		// The three terminal failures. Each carries a timestamp, and none of
		// them means the notice was read.
		{"no XML exists for this notice", &ts, strp("absent"), nil},
		{"the XML would not parse", &ts, strp("parse_error"), nil},
		{"the fetch gave up after retries", &ts, strp("unavailable"), nil},
		// Defensive: a status this service has never heard of is not "ok".
		{"an unknown future status", &ts, strp("something_new"), nil},
		// A status with no timestamp, and a timestamp with no status, are both
		// incoherent rows; neither may be reported as an enrichment.
		{"status without a timestamp", nil, strp("ok"), nil},
		{"timestamp without a status", &ts, nil, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := enrichedAt(tc.fetchedAt, tc.status)
			switch {
			case tc.want == nil && got != nil:
				t.Errorf("enrichedAt = %v, want nil", *got)
			case tc.want != nil && got == nil:
				t.Errorf("enrichedAt = nil, want %v", *tc.want)
			case tc.want != nil && !got.Equal(*tc.want):
				t.Errorf("enrichedAt = %v, want %v", *got, *tc.want)
			}
		})
	}
}

func TestDerefString(t *testing.T) {
	if got := derefString(nil); got != "" {
		t.Errorf("derefString(nil) = %q, want empty", got)
	}
	if got := derefString(strp("https://buyer.example/docs")); got != "https://buyer.example/docs" {
		t.Errorf("derefString = %q, want the value through", got)
	}
}

// ── SQL shape ──────────────────────────────────────────────────────────────

// The whole reason criteria are a flat list rather than nested inside Lot is
// that a query per lot would be an N+1 against a 13-lot notice. One statement,
// ordered so the caller can group in a single pass.
func TestCriteriaSQL_IsOneOrderedStatementPerTender(t *testing.T) {
	if strings.Count(criteriaSQL, "SELECT") != 1 {
		t.Error("criteriaSQL is not a single statement")
	}
	if !strings.Contains(criteriaSQL, "WHERE tender_id = $1") {
		t.Error("criteriaSQL is not scoped to one tender")
	}
	if !strings.Contains(criteriaSQL, "ORDER BY lot_ref ASC, ordinal ASC") {
		t.Error("criteriaSQL must order by (lot_ref, ordinal) so grouping is a single pass in published order")
	}
}

// `weight numeric` will not scan into a *float64 without the cast, and a
// coalesce would be far worse than a scan error: it would turn the COMMON
// no-weight case into a published zero.
func TestCriteriaSQL_CastsWeightAndNeverDefaultsIt(t *testing.T) {
	if !strings.Contains(criteriaSQL, "weight::float8") {
		t.Error("criteriaSQL does not cast the numeric weight; it will not scan into *float64")
	}
	if strings.Contains(criteriaSQL, "coalesce(weight") {
		t.Fatal("criteriaSQL coalesces weight — an absent weight would become a published zero")
	}
}

// text[] has no drops column type and no scan path, so both array columns
// cross the boundary comma-joined, exactly as cpv_secondary already does.
func TestArrayColumnsCrossTheBoundaryJoined(t *testing.T) {
	if !strings.Contains(organizationsSQL, "array_to_string(roles, ',')") {
		t.Error("organizationsSQL does not project roles through array_to_string")
	}
	if !strings.Contains(lotsSQL, "array_to_string(cpv_secondary, ',')") {
		t.Error("lotsSQL does not project cpv_secondary through array_to_string")
	}
}

// ── The search projection ──────────────────────────────────────────────────

// tenderSelectColumns and queryScoredTenders' single Scan are one contract in
// two places, and a mismatch is a runtime error on every search rather than a
// compile-time one. Pinning the count is what makes adding a column here fail
// loudly if the Scan is not updated with it.
func TestTenderSelectColumns_ProjectsExactlyWhatIsScanned(t *testing.T) {
	const wantColumns = 16 // + relevance + snippet, appended by each query
	if got := strings.Count(tenderSelectColumns, ",") + 1; got != wantColumns {
		t.Fatalf("tenderSelectColumns projects %d columns, want %d — queryScoredTenders' Scan must match", got, wantColumns)
	}
	for _, col := range []string{"t.description"} {
		if !strings.Contains(tenderSelectColumns, col) {
			t.Errorf("tenderSelectColumns is missing %s", col)
		}
	}
}

// TestTenderSelectColumns_OmitsIngestion0010Columns pins the production
// incident that removed them. publication_number, documents_url,
// submission_url and grid_usable are created by services/ingestion's migration
// 0010. The backend does not migrate the tenders schema and deploys on its own
// image tag, so naming them here took every search down with SQLSTATE 42703
// in the window before ingestion rolled out — both arms at once, because the
// lexical and semantic paths share this projection.
//
// Nothing downstream ever read them from a search result: the proto carries
// them on TenderDetail alone. They belong on the per-tender fetch, and this
// test is what stops a future change from moving them back onto the hot path.
func TestTenderSelectColumns_OmitsIngestion0010Columns(t *testing.T) {
	for _, col := range []string{
		"publication_number", "documents_url", "submission_url", "grid_usable",
	} {
		if strings.Contains(tenderSelectColumns, col) {
			t.Errorf("tenderSelectColumns names %s — that column is owned by services/ingestion's "+
				"migration 0010 and is not guaranteed to exist when this service deploys; "+
				"detail-only fields belong on TenderDetail", col)
		}
	}
}

// Every child table is absent from the search projection on purpose: it is
// materialised across a candidate window of hundreds of rows, so a join here
// would be an N+1 on the hot path for data no result card shows.
func TestTenderSelectColumns_JoinsNoChildTable(t *testing.T) {
	for _, table := range []string{
		"ingested_tender_lots", "ingested_tender_award_criteria", "ingested_tender_organizations",
	} {
		if strings.Contains(tenderFromClause, table) {
			t.Errorf("the search FROM clause joins %s — that is an N+1 across the candidate window", table)
		}
	}
}
