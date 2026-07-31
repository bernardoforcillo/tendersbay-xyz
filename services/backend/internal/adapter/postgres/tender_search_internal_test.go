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

// cpvPrefixRanking backs FindByCPVPrefixes' ordering fix (Task 14/15): a
// tender's rank must follow the CALLER'S code order, not just recency. These
// two tests pin the SQL shape down directly, without a live database — the
// behaviour itself (does the database actually order rows this way) is
// checked against a real one in TestFindByCPVPrefixes_* in tender_repo_test.go.
func TestCPVPrefixRanking_MatchesBothPrimaryAndSecondaryCPV(t *testing.T) {
	b := &argBuilder{}
	predicate, rank := cpvPrefixRanking([]string{"9091", "45"}, b)

	if !strings.Contains(predicate, "t.cpv LIKE") {
		t.Errorf("predicate = %q, want a primary-CPV prefix match", predicate)
	}
	if !strings.Contains(predicate, "unnest(t.cpv_secondary)") {
		t.Errorf("predicate = %q, want a secondary-CPV prefix match too", predicate)
	}
	if len(b.args) != 2 || b.args[0] != "9091%" || b.args[1] != "45%" {
		t.Errorf("args = %v, want [9091%% 45%%] — one LIKE pattern per code, shared by its primary and secondary arm", b.args)
	}
	if !strings.HasPrefix(rank, "CASE ") || !strings.HasSuffix(rank, " END") {
		t.Errorf("rank = %q, want a CASE…END expression", rank)
	}
}

// The FIRST code the caller passed is the one the lexicon ranked highest
// (cpvCandidates preserves MatchCodes' order), so it must resolve to the
// LOWEST — i.e. best — CASE rank. Getting this backwards would silently
// invert the fix: a tender matching only the query's weakest resolved code
// would outrank one matching its best.
func TestCPVPrefixRanking_RanksTheCallersFirstCodeBest(t *testing.T) {
	b := &argBuilder{}
	_, rank := cpvPrefixRanking([]string{"9091", "45", "72"}, b)

	posBest := strings.Index(rank, "THEN 0")
	posMid := strings.Index(rank, "THEN 1")
	posWorst := strings.Index(rank, "THEN 2")
	if posBest == -1 || posMid == -1 || posWorst == -1 {
		t.Fatalf("rank = %q, want THEN 0, THEN 1 and THEN 2 (one per code)", rank)
	}
	if !(posBest < posMid && posMid < posWorst) {
		t.Errorf("rank = %q, want THEN 0 before THEN 1 before THEN 2 — CASE evaluates WHENs in order, so this is what makes the caller's best-ranked code win", rank)
	}
}

func TestCPVPrefixRanking_NoCodesProducesNoPredicate(t *testing.T) {
	b := &argBuilder{}
	predicate, rank := cpvPrefixRanking(nil, b)
	if predicate != "" || rank != "" {
		t.Errorf("predicate=%q rank=%q, want both empty for no codes — a non-empty predicate here would risk matching every tender", predicate, rank)
	}
	if len(b.args) != 0 {
		t.Errorf("args = %v, want none bound when there are no codes to match", b.args)
	}
}

// lexicalSQL backs LexicalSearch (Task 16): whether the CPV-label weight
// class (migration 0008) is part of the lexical match is a toggle,
// tender.LexicalQuery.ExpandCPVLabels, and these tests pin its SQL shape down
// directly, without a live database — the same way cpvPrefixRanking's tests
// above do, and matchCodesSQL's in cpv_lexicon_internal_test.go.

func TestLexicalSQL_ExpansionOnMatchesTheWholeVector(t *testing.T) {
	sql, _ := lexicalSQL(tender.LexicalQuery{Text: "pulizie", ExpandCPVLabels: true}, tender.Filters{}, 20)
	if strings.Contains(sql, "ts_filter") {
		t.Errorf("SQL narrows the vector with expansion ON:\n%s", sql)
	}
	if !strings.Contains(sql, "t.search_vector @@ websearch_to_tsquery('simple'") {
		t.Errorf("SQL missing the full-vector match:\n%s", sql)
	}
}

func TestLexicalSQL_ExpansionOffDropsTheLabelWeightClass(t *testing.T) {
	// The labels live inside search_vector, so an unmodified match picks them up
	// unconditionally — the toggle only means something if it narrows.
	sql, _ := lexicalSQL(tender.LexicalQuery{Text: "pulizie", ExpandCPVLabels: false}, tender.Filters{}, 20)
	if !strings.Contains(sql, "ts_filter(t.search_vector, '{a,b,d}')") {
		t.Errorf("SQL does not drop weight class C with expansion OFF:\n%s", sql)
	}
}

func TestLexicalSQL_KeepsTheBareCodeMatchableWhenExpansionIsOff(t *testing.T) {
	// cpv and cpv_labels share weight class C, so dropping C would also stop the
	// bare 8-digit code from matching — and an exact code is exactly what the
	// lexical arm exists to answer.
	sql, _ := lexicalSQL(tender.LexicalQuery{Text: "45233120", ExpandCPVLabels: false}, tender.Filters{}, 20)
	if !strings.Contains(sql, "t.cpv = ") && !strings.Contains(sql, "t.cpv LIKE ") {
		t.Errorf("SQL has no direct cpv arm, so an exact code stops matching with expansion off:\n%s", sql)
	}
}

// The direct t.cpv arm must be scoped to expansion-off ONLY. With expansion
// on, search_vector's weight class C already carries the code (migration 0008
// sets weight C from the cpv column together with cpv_labels), so an
// unconditional extra arm here would not be wrong, exactly, but would make the
// OFF path's SQL indistinguishable from a bug that always adds it — this pins
// the arm to the branch that actually needs it.
func TestLexicalSQL_ExpansionOnHasNoRedundantCPVArm(t *testing.T) {
	sql, _ := lexicalSQL(tender.LexicalQuery{Text: "45233120", ExpandCPVLabels: true}, tender.Filters{}, 20)
	if strings.Contains(sql, "t.cpv = ") || strings.Contains(sql, "t.cpv LIKE ") {
		t.Errorf("SQL has a direct t.cpv arm with expansion ON, want none — search_vector already covers the code:\n%s", sql)
	}
}

// A row matching ONLY via the direct t.cpv arm (no text overlap in
// title/buyer/description) must not sort behind every other candidate: with
// expansion off, ts_rank_cd against the narrowed vector is 0 for such a row
// (the code lexeme lived in the now-dropped weight class C), so without an
// explicit rank contribution an exact-code match — the one case this
// retriever exists to answer — ties at the bottom with every non-match and is
// ordered only by recency. Mirrors the trigram arm's own
// GREATEST(rank, similarity(...)) pattern a few lines below in lexicalSQL.
func TestLexicalSQL_BareCodeMatchEarnsARankContribution(t *testing.T) {
	sql, _ := lexicalSQL(tender.LexicalQuery{Text: "45233120", ExpandCPVLabels: false}, tender.Filters{}, 20)
	if !strings.Contains(sql, "GREATEST(") {
		t.Fatalf("SQL has no GREATEST rank combinator:\n%s", sql)
	}
	// The RANK clause (before "AS relevance") must itself reference the cpv
	// equality, not just the WHERE clause — widening WHERE without widening
	// rank leaves the row matchable but effectively unranked.
	rankClause := sql[:strings.Index(sql, "AS relevance")]
	if !strings.Contains(rankClause, "t.cpv = ") {
		t.Errorf("rank clause = %q, want it to reference t.cpv so an exact-code-only match outranks a zero-relevance tie", rankClause)
	}
}

func TestLexicalSQL_EmptyTextProducesNoStatement(t *testing.T) {
	sql, args := lexicalSQL(tender.LexicalQuery{Text: "   "}, tender.Filters{}, 20)
	if sql != "" || len(args) != 0 {
		t.Errorf("lexicalSQL = %q / %v, want empty for blank text", sql, args)
	}
}

func TestLexicalSQL_StillAppliesTheFilters(t *testing.T) {
	// Both retrievers must pass through the same authoritative Postgres filter;
	// a narrowing that skipped it would return rows the caller excluded.
	sql, args := lexicalSQL(tender.LexicalQuery{Text: "pulizie"}, tender.Filters{Countries: []string{"IT"}}, 20)
	if !strings.Contains(sql, "t.country IN (") {
		t.Errorf("SQL lost the country filter:\n%s", sql)
	}
	var sawIT bool
	for _, a := range args {
		if a == "IT" {
			sawIT = true
		}
	}
	if !sawIT {
		t.Errorf("args = %v, want IT recorded", args)
	}
}
