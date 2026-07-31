package eval

import (
	"os"
	"strings"
	"testing"
)

// minGoldenQueries is the floor below which the report's per-language cells
// become single-query anecdotes. It is a smell test, not statistics — see the
// design spec's Risks section.
const minGoldenQueries = 40

// minCrossLanguageQueries is how many queries must probe a document language
// other than their own. Without them the matrix's off-diagonal cells are empty
// and the harness silently stops measuring the requirement it exists for.
const minCrossLanguageQueries = 12

func loadCommittedGolden(t *testing.T) (GoldenSet, map[string]CorpusTender) {
	t.Helper()
	fsys := os.DirFS("testdata")

	set, err := LoadGolden(fsys, "golden.json")
	if err != nil {
		t.Fatalf("LoadGolden: %v", err)
	}
	corpus, err := LoadCorpus(fsys, "corpus.jsonl.gz")
	if err != nil {
		t.Fatalf("LoadCorpus: %v", err)
	}
	byKey := make(map[string]CorpusTender, len(corpus))
	for _, c := range corpus {
		byKey[c.Key()] = c
	}
	return set, byKey
}

func TestGolden_EveryJudgedTenderExistsInTheCorpus(t *testing.T) {
	// A judgement pointing at a tender the snapshot does not contain is
	// unreachable: it can never be retrieved, so it silently caps recall below
	// 1.0 forever and looks like a ranking failure.
	set, corpus := loadCommittedGolden(t)
	for _, q := range set {
		for key := range q.Judgements {
			if _, ok := corpus[key]; !ok {
				t.Errorf("corpus[%s] ok = %v, want true — golden query %q judges this tender, and a judgement pointing outside the corpus snapshot is unreachable, silently capping recall below 1.0 forever and looking exactly like a ranking failure", key, ok, q.ID)
			}
		}
	}
}

func TestGolden_HasEnoughQueriesToBeWorthReading(t *testing.T) {
	set, _ := loadCommittedGolden(t)
	if len(set) < minGoldenQueries {
		t.Errorf("len(set) = %d, want at least %d — below this floor the report's per-language cells become single-query anecdotes, not a measurement", len(set), minGoldenQueries)
	}
}

func TestGolden_ProbesCrossLanguageRetrieval(t *testing.T) {
	set, corpus := loadCommittedGolden(t)
	cross := 0
	for _, q := range set {
		for key := range q.Judgements {
			if q.Judgements[key] > 0 && corpus[key].Language != "" && corpus[key].Language != q.Language {
				cross++
				break
			}
		}
	}
	if cross < minCrossLanguageQueries {
		t.Errorf("only %d golden queries judge a document in another language, want at least %d — "+
			"without them the report's off-diagonal cells are empty and measure nothing",
			cross, minCrossLanguageQueries)
	}
}

func TestGolden_CoversMoreThanOneQueryLanguage(t *testing.T) {
	// A set written entirely in Italian would report a healthy overall number
	// while saying nothing about the other 23 locales we serve.
	set, _ := loadCommittedGolden(t)
	langs := map[string]bool{}
	for _, q := range set {
		langs[q.Language] = true
	}
	if len(langs) < 4 {
		t.Errorf("len(langs) = %d %v, want at least 4 — fewer would report a healthy overall number while saying nothing about the other locales this project serves", len(langs), sortedKeys(langs))
	}
}

func TestGolden_NoteExplainsEveryCrossLanguageQuery(t *testing.T) {
	// A cross-language judgement is the non-obvious kind: six months from now
	// nobody will remember why an Italian query should match a German notice
	// unless the query says so.
	set, corpus := loadCommittedGolden(t)
	for _, q := range set {
		crossLang := false
		for key, grade := range q.Judgements {
			if grade > 0 && corpus[key].Language != "" && corpus[key].Language != q.Language {
				crossLang = true
				break
			}
		}
		if crossLang && strings.TrimSpace(q.Note) == "" {
			t.Errorf("query %q note = %q, want non-empty — a cross-language judgement is the non-obvious kind: six months from now nobody will remember why this query should match a foreign-language document unless the note says so", q.ID, q.Note)
		}
	}
}
