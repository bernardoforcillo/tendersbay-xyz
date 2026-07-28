package postgres

import (
	"strings"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

func int64p(v int64) *int64 { return &v }

// An unset facet must produce no predicate at all. The failure mode this
// guards is not cosmetic: a `= ”` test excludes every row, so a blank filter
// control would silently empty the results instead of widening them.
func TestFilterClauses_UnsetFacetsProduceNoPredicates(t *testing.T) {
	b := &argBuilder{}
	got := filterClauses(tender.Filters{}, b)
	if len(got) != 0 {
		t.Errorf("clauses = %v, want none for zero-value filters", got)
	}
	if len(b.args) != 0 {
		t.Errorf("args = %v, want none", b.args)
	}
}

// Whitespace-only and empty entries come from UI controls whose "any" option
// is the empty string; they must be dropped rather than matched literally.
func TestFilterClauses_DropsBlankListEntries(t *testing.T) {
	b := &argBuilder{}
	got := filterClauses(tender.Filters{Countries: []string{"IT", "", "  ", "DE"}}, b)
	if len(got) != 1 {
		t.Fatalf("clauses = %v, want exactly one country clause", got)
	}
	if len(b.args) != 2 || b.args[0] != "IT" || b.args[1] != "DE" {
		t.Errorf("args = %v, want only [IT DE]", b.args)
	}
}

func TestFilterClauses_NumbersPlaceholdersSequentially(t *testing.T) {
	b := &argBuilder{}
	got := filterClauses(tender.Filters{
		Countries: []string{"IT", "DE"},
		Statuses:  []string{"open"},
	}, b)
	joined := strings.Join(got, " AND ")
	for _, want := range []string{"$1", "$2", "$3"} {
		if !strings.Contains(joined, want) {
			t.Errorf("clauses %q missing placeholder %s", joined, want)
		}
	}
	if len(b.args) != 3 {
		t.Errorf("args = %v, want 3", b.args)
	}
}

// The placeholder numbering has to keep counting from wherever the caller
// left off — LexicalSearch already consumed $1 for the tsquery before any
// filter clause is built.
func TestArgBuilder_ContinuesFromExistingArgs(t *testing.T) {
	b := &argBuilder{}
	if p := b.next("query text"); p != "$1" {
		t.Fatalf("first placeholder = %s, want $1", p)
	}
	got := filterClauses(tender.Filters{Statuses: []string{"open"}}, b)
	if !strings.Contains(got[0], "$2") {
		t.Errorf("clause = %q, want it to use $2, not restart at $1", got[0])
	}
}

// A CPV prefix must match secondary codes too — a tender only incidentally in
// a sector is still a tender in that sector, and the vector payload indexes
// both, so the two retrieval paths would otherwise disagree about what the
// same filter means.
func TestFilterClauses_CPVPrefixMatchesSecondaryCodes(t *testing.T) {
	b := &argBuilder{}
	got := filterClauses(tender.Filters{CPVPrefixes: []string{"45"}}, b)
	if len(got) != 1 {
		t.Fatalf("clauses = %v, want one", got)
	}
	if !strings.Contains(got[0], "t.cpv LIKE") {
		t.Errorf("clause = %q, want a primary-CPV prefix match", got[0])
	}
	if !strings.Contains(got[0], "unnest(t.cpv_secondary)") {
		t.Errorf("clause = %q, want a secondary-CPV prefix match too", got[0])
	}
	if len(b.args) != 1 || b.args[0] != "45%" {
		t.Errorf("args = %v, want a single [45%%] shared by both arms", b.args)
	}
}

func TestFilterClauses_MultipleCPVPrefixesAreORed(t *testing.T) {
	b := &argBuilder{}
	got := filterClauses(tender.Filters{CPVPrefixes: []string{"45", "72"}}, b)
	if n := strings.Count(got[0], " OR "); n != 3 {
		t.Errorf("clause = %q, want 3 ORs joining 2 prefixes x 2 arms", got[0])
	}
	if len(b.args) != 2 {
		t.Errorf("args = %v, want one per prefix (each shared by its two arms)", b.args)
	}
}

// A buyer name containing a LIKE wildcard must search for that character, not
// match everything — "50%% financing authority" is a plausible buyer name.
func TestEscapeLike_NeutralisesWildcards(t *testing.T) {
	got := escapeLike(`50% _ \ authority`)
	if !strings.Contains(got, `\%`) || !strings.Contains(got, `\_`) {
		t.Errorf("escapeLike = %q, want %% and _ escaped", got)
	}
	if !strings.Contains(got, `\\`) {
		t.Errorf("escapeLike = %q, want the backslash itself escaped first", got)
	}
}

func TestFilterClauses_BuyerIsASubstringMatch(t *testing.T) {
	b := &argBuilder{}
	got := filterClauses(tender.Filters{Buyer: "Comune di Roma"}, b)
	if len(got) != 1 || !strings.Contains(got[0], "ILIKE") {
		t.Fatalf("clauses = %v, want one case-insensitive match", got)
	}
	// Buyers are written inconsistently across sources, so the match is
	// anchored at neither end.
	if b.args[0] != "%Comune di Roma%" {
		t.Errorf("arg = %v, want the term wrapped in wildcards on both sides", b.args[0])
	}
}

func TestFilterClauses_RangesAndDates(t *testing.T) {
	from := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	b := &argBuilder{}
	got := filterClauses(tender.Filters{
		ValueMin:     int64p(1000),
		ValueMax:     int64p(500000),
		DeadlineFrom: &from,
	}, b)
	joined := strings.Join(got, " AND ")
	for _, want := range []string{"t.value >=", "t.value <=", "t.deadline >="} {
		if !strings.Contains(joined, want) {
			t.Errorf("clauses %q missing %q", joined, want)
		}
	}
	if len(b.args) != 3 {
		t.Fatalf("args = %v, want 3", b.args)
	}
	if b.args[0] != int64(1000) || b.args[2] != from {
		t.Errorf("args = %v, want the values bound as-is, not stringified", b.args)
	}
}

func TestWhereClause_OmittedWhenThereAreNoPredicates(t *testing.T) {
	if got := whereClause(nil); got != "" {
		t.Errorf("whereClause(nil) = %q, want empty — a bare WHERE is a syntax error", got)
	}
	if got := whereClause([]string{"a = 1", "b = 2"}); !strings.Contains(got, "AND") {
		t.Errorf("whereClause = %q, want the predicates ANDed", got)
	}
}

func TestNonEmpty_TrimsAndDropsBlanks(t *testing.T) {
	got := nonEmpty([]string{" IT ", "", "\t", "DE"})
	if len(got) != 2 || got[0] != "IT" || got[1] != "DE" {
		t.Errorf("nonEmpty = %v, want [IT DE] trimmed", got)
	}
}
