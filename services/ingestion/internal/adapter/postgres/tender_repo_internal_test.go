package postgres

import (
	"strings"
	"testing"
)

// TestMarkDocumentsFailedClearsTheLastLooksMeasurement pins the three NULLs in
// markDocumentsFailedSQL, and it needs no database because the defect it
// guards is not one a database would report.
//
// A failure reaches that statement without an Outcome, so it has no coverage
// measurement and no enumerator to attribute — but the columns are not empty.
// Rows are re-queued IN PLACE (upsertTenderSQL resets docs_fetched_at and
// docs_attempts on a re-ingested notice and deliberately leaves docs_status,
// docs_platform and the two counts alone), so without these NULLs a tender that
// once read cleanly as tuttogare with 5 files found and 3 extracted, and whose
// corrected documents_url is then robots-denied, persists as robots_denied WITH
// 5/3 and tuttogare still on it. Every one of those three facts describes a URL
// nobody looked at again.
//
// That row is exactly what migration 0011's header forbids — "a robots-denied
// tender has no file count because we never saw the listing" — and it is the
// unreadable-versus-absent conflation surviving inside a single row instead of
// across two. Nothing errors when it happens: the platform backlog query just
// files a refusal under a parser that never ran on that address, and reports a
// coverage number it invented.
func TestMarkDocumentsFailedClearsTheLastLooksMeasurement(t *testing.T) {
	set := setClause(t, markDocumentsFailedSQL)

	for _, column := range []string{"docs_platform", "docs_files_found", "docs_files_extracted"} {
		assignment, ok := set[column]
		if !ok {
			t.Errorf("markDocumentsFailedSQL never assigns %s — a stale value from the last look survives the failure", column)
			continue
		}
		if !strings.EqualFold(assignment, "NULL") {
			t.Errorf("markDocumentsFailedSQL sets %s = %q, want NULL — this pass read no listing, so it has no measurement to write", column, assignment)
		}
	}

	// The retry policy itself, asserted alongside so a future edit cannot buy
	// the NULLs by breaking it: the counter always advances, and only a
	// permanent failure stamps docs_fetched_at and takes the row out.
	if got := set["docs_attempts"]; !strings.Contains(got, "docs_attempts + 1") {
		t.Errorf("docs_attempts = %q, want it incremented on every attempt", got)
	}
	if got := set["docs_fetched_at"]; !strings.Contains(strings.ToUpper(got), "CASE WHEN") {
		t.Errorf("docs_fetched_at = %q, want it stamped only when the failure is permanent", got)
	}
}

// TestMarkDocumentsRetrievedWritesEveryCoverageColumn is the other half of the
// pair. Between them the two terminal statements must write the same column
// set, or a column one of them clears and the other never touches drifts back
// into the stale-value state above by a different route.
func TestMarkDocumentsRetrievedWritesEveryCoverageColumn(t *testing.T) {
	retrieved := setClause(t, markDocumentsRetrievedSQL)
	failed := setClause(t, markDocumentsFailedSQL)

	for _, column := range []string{"docs_status", "docs_platform", "docs_files_found", "docs_files_extracted"} {
		if _, ok := retrieved[column]; !ok {
			t.Errorf("markDocumentsRetrievedSQL never assigns %s", column)
		}
		if _, ok := failed[column]; !ok {
			t.Errorf("markDocumentsFailedSQL never assigns %s", column)
		}
	}
}

// setClause parses an UPDATE's SET list into column -> assigned expression.
// Deliberately simple: these two statements are hand-written constants in this
// file's package, one assignment per line, and a parser any more general than
// that would be the thing under test.
func setClause(t *testing.T, stmt string) map[string]string {
	t.Helper()

	_, after, ok := strings.Cut(stmt, "SET ")
	if !ok {
		t.Fatalf("statement has no SET clause: %s", stmt)
	}
	body, _, ok := strings.Cut(after, "WHERE")
	if !ok {
		t.Fatalf("statement has no WHERE clause: %s", stmt)
	}

	out := map[string]string{}
	for _, assignment := range strings.Split(body, ",\n") {
		column, expr, ok := strings.Cut(assignment, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(column)] = strings.TrimSpace(expr)
	}
	return out
}
