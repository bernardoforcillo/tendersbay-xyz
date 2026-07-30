package postgres

import (
	"regexp"
	"strings"
	"testing"
)

// langRestrictionPattern catches a per-language filter regardless of spacing
// or case — "lang = 'it'", "lang='it'", "lang IN (...)" and "lang in(...)"
// all match. A literal Contains(sql, "lang =") check would miss "lang='en'"
// (no space), silently letting a language restriction back in unnoticed.
var langRestrictionPattern = regexp.MustCompile(`(?i)\blang\s*(=|in\b)`)

// groupByClausePattern captures exactly the column list a GROUP BY names, so
// a test can assert on it rather than merely on GROUP BY's presence.
var groupByClausePattern = regexp.MustCompile(`(?i)GROUP BY\s+([^\n]+)`)

func TestMatchCodesSQL_MatchesAllLanguagesAtOnce(t *testing.T) {
	// Restricting to the caller's UI language would defeat the purpose: people
	// type in a language other than their interface, and technical terms
	// ("software", "catering", "hosting") travel unchanged across languages.
	sql, _ := matchCodesSQL("pulizie uffici", 8)
	if m := langRestrictionPattern.FindString(sql); m != "" {
		t.Errorf("sql contains %q, want no per-language restriction — narrowing to one language breaks exactly the cross-language queries this bridge exists for:\n%s", m, sql)
	}
}

func TestMatchCodesSQL_UsesTheLexemeIndexAndTheTrigramArm(t *testing.T) {
	sql, args := matchCodesSQL("pulizie", 8)
	if !strings.Contains(sql, "label_vector @@ websearch_to_tsquery('simple'") {
		t.Errorf("sql = %q, want the lexeme match present — without it, exact and near-exact lexical hits (matching CPV vocabulary, not just fuzzy spelling) are lost entirely", sql)
	}
	if !strings.Contains(sql, "label %") {
		t.Errorf("sql = %q, want the trigram arm present — without it an inflected query like \"pulizie\" (plural) never matches the stored \"pulizia\" (singular) lexeme at all, since the 'simple' text-search config does not stem", sql)
	}
	// The two arms must be OR-ed, not AND-ed: AND would require both the
	// lexeme AND the trigram condition to hold at once, which is a severe
	// recall regression — the whole point of the second arm is to catch what
	// the first missed, not to co-require it. Both substrings above would
	// still be present under an AND, so only checking for their presence
	// would not catch this.
	if !strings.Contains(sql, ") OR t.label %") {
		t.Errorf("sql = %q, want the lexeme and trigram arms joined by OR, not AND — AND would require both conditions to hold simultaneously, undoing the whole reason the trigram arm exists", sql)
	}
	// One placeholder per distinct value: query (twice is fine as one arg reused
	// is NOT the house style — argBuilder appends), plus the limit.
	if len(args) == 0 {
		t.Error("args is empty, want the query and the limit recorded — with no positional args, matchCodesSQL's placeholders ($1, $2, ...) are unbound and Query would fail against the database")
	}
	if got := args[len(args)-1]; got != 8 {
		t.Errorf("last arg = %v, want the limit 8 — LIMIT must bind to the caller's requested value or a search can return an unbounded, un-capped result set", got)
	}
}

func TestMatchCodesSQL_CollapsesToOneRowPerCode(t *testing.T) {
	// cpv_terms holds 24 rows per code. Grouping by anything wider than t.code
	// alone — e.g. "GROUP BY t.code, t.lang" — reopens exactly the hole this
	// guards: a query matching a label in six languages would return the same
	// code six times, each consuming one of the arm's limited candidate slots
	// with a copy of itself.
	sql, _ := matchCodesSQL("pulizie", 8)
	m := groupByClausePattern.FindStringSubmatch(sql)
	if m == nil {
		t.Fatalf("sql has no GROUP BY clause, want \"GROUP BY t.code\" — without one, a code matching several languages returns once per matching language instead of once, so it consumes several of the arm's limited candidate slots with copies of itself:\n%s", sql)
	}
	if got := strings.TrimSpace(m[1]); got != "t.code" {
		t.Errorf("GROUP BY clause = %q, want exactly \"t.code\" — grouping by anything wider (e.g. adding t.lang back in) reopens the duplicate-code hole this test exists to catch", got)
	}
}

func TestMatchCodesSQL_OrdersDeterministically(t *testing.T) {
	// Ties broken by code so the same query always resolves the same codes —
	// otherwise the CPV arm's contribution would drift between identical
	// searches and the harness would report phantom regressions.
	sql, _ := matchCodesSQL("pulizie", 8)
	if !strings.Contains(sql, "ORDER BY score DESC, code") {
		t.Errorf("sql = %q, want \"ORDER BY score DESC, code\" — without the code tiebreak, two codes with an equal score can swap places between runs of the identical query, and the eval harness would report the swap as a phantom regression", sql)
	}
}

func TestMatchCodesSQL_SkipsAShortTrigramProbeForLongInput(t *testing.T) {
	// Same reasoning as LexicalSearch's trigramQueryMaxLen: on a long sentence
	// trigram similarity matches nothing useful and only costs an index probe.
	long := strings.Repeat("parola ", 20)
	sql, _ := matchCodesSQL(long, 8)
	if strings.Contains(sql, "label %") {
		t.Errorf("sql added a trigram arm for %d-rune input, want it omitted — pg_trgm similarity is normalised by the combined trigram count of query and label, so a sentence this long cannot score above the 0.3 cutoff against a short CPV label and the probe only spends an index lookup for nothing", len([]rune(long)))
	}
}

func TestMatchCodesSQL_TrigramArmPresentAtTheBoundaryAbsentOneRuneOver(t *testing.T) {
	// The long-input test above only proves SOME bound exists below 140 runes
	// — it would pass unchanged if cpvTrigramMaxLen were silently reverted to
	// 40, or set to 1. This test pins BOTH the specific chosen value (see
	// cpvTrigramMaxLen's doc comment in cpv_lexicon.go for why 60) AND that
	// the cutoff is applied inclusively ("<=") rather than off by one.
	if cpvTrigramMaxLen != 60 {
		t.Fatalf("cpvTrigramMaxLen = %d, want 60 — pinning the value the task chose; change it deliberately (update this test and the doc comment's justification together), not silently", cpvTrigramMaxLen)
	}

	atBound := strings.Repeat("a", cpvTrigramMaxLen)
	sql, _ := matchCodesSQL(atBound, 8)
	if !strings.Contains(sql, "label %") {
		t.Errorf("trigram arm absent at exactly cpvTrigramMaxLen (%d) runes, want it present — the bound is inclusive, so a query at the cutoff must still get the fuzzy arm", cpvTrigramMaxLen)
	}

	overBound := strings.Repeat("a", cpvTrigramMaxLen+1)
	sql, _ = matchCodesSQL(overBound, 8)
	if strings.Contains(sql, "label %") {
		t.Errorf("trigram arm present at cpvTrigramMaxLen+1 (%d) runes, want it absent — a query one rune past the bound must fall back to lexeme-only, or the constant is not actually being enforced", cpvTrigramMaxLen+1)
	}
}
