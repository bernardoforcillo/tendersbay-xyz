package postgres

import (
	"strings"
	"testing"
)

func TestMatchCodesSQL_MatchesAllLanguagesAtOnce(t *testing.T) {
	// Restricting to the caller's UI language would defeat the purpose: people
	// type in a language other than their interface, and technical terms
	// ("software", "catering", "hosting") travel unchanged across languages.
	sql, _ := matchCodesSQL("pulizie uffici", 8)
	if strings.Contains(sql, "lang =") || strings.Contains(sql, "lang IN") {
		t.Errorf("SQL restricts by language, want all 24 searched:\n%s", sql)
	}
}

func TestMatchCodesSQL_UsesTheLexemeIndexAndTheTrigramArm(t *testing.T) {
	sql, args := matchCodesSQL("pulizie", 8)
	if !strings.Contains(sql, "label_vector @@ websearch_to_tsquery('simple'") {
		t.Errorf("SQL missing the lexeme match:\n%s", sql)
	}
	if !strings.Contains(sql, "label %") {
		t.Errorf("SQL missing the trigram arm:\n%s", sql)
	}
	// One placeholder per distinct value: query (twice is fine as one arg reused
	// is NOT the house style — argBuilder appends), plus the limit.
	if len(args) == 0 {
		t.Error("args is empty, want the query and the limit recorded")
	}
	if args[len(args)-1] != 8 {
		t.Errorf("last arg = %v, want the limit 8", args[len(args)-1])
	}
}

func TestMatchCodesSQL_CollapsesToOneRowPerCode(t *testing.T) {
	// cpv_terms holds 24 rows per code. Without a GROUP BY, a query matching a
	// label in six languages would return the same code six times and consume
	// six of the arm's candidate slots with copies of itself.
	sql, _ := matchCodesSQL("pulizie", 8)
	if !strings.Contains(sql, "GROUP BY") {
		t.Errorf("SQL does not group by code:\n%s", sql)
	}
}

func TestMatchCodesSQL_OrdersDeterministically(t *testing.T) {
	// Ties broken by code so the same query always resolves the same codes —
	// otherwise the CPV arm's contribution would drift between identical
	// searches and the harness would report phantom regressions.
	sql, _ := matchCodesSQL("pulizie", 8)
	if !strings.Contains(sql, "ORDER BY score DESC, code") {
		t.Errorf("SQL ordering is not total:\n%s", sql)
	}
}

func TestMatchCodesSQL_SkipsAShortTrigramProbeForLongInput(t *testing.T) {
	// Same reasoning as LexicalSearch's trigramQueryMaxLen: on a long sentence
	// trigram similarity matches nothing useful and only costs an index probe.
	long := strings.Repeat("parola ", 20)
	sql, _ := matchCodesSQL(long, 8)
	if strings.Contains(sql, "label %") {
		t.Errorf("SQL added a trigram arm for %d-rune input:\n%s", len([]rune(long)), sql)
	}
}
