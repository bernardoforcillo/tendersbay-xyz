package eval

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
)

// GoldenQuery is one hand-judged search: the text a person would type, the
// language they typed it in, and which tenders in the corpus snapshot are
// relevant to it.
//
// Note is free text explaining what the query is meant to exercise. It is not
// used by any metric — it exists so a future reader can tell a deliberate
// cross-language probe from an ordinary same-language check without
// reverse-engineering the judgements.
type GoldenQuery struct {
	ID         string     `json:"id"`
	Query      string     `json:"query"`
	Language   string     `json:"language"`
	Note       string     `json:"note,omitempty"`
	Judgements Judgements `json:"judgements"`
}

// GoldenSet is the whole judged set, in file order.
type GoldenSet []GoldenQuery

// LoadGolden reads and validates a golden set.
//
// Validation is strict and fails the load rather than skipping a bad entry: a
// silently-dropped query changes every aggregate in the report while looking
// exactly like a query that simply scored badly.
func LoadGolden(fsys fs.FS, name string) (GoldenSet, error) {
	raw, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, fmt.Errorf("eval: read golden set %s: %w", name, err)
	}
	var set GoldenSet
	if err := json.Unmarshal(raw, &set); err != nil {
		return nil, fmt.Errorf("eval: parse golden set %s: %w", name, err)
	}
	if len(set) == 0 {
		return nil, fmt.Errorf("eval: golden set %s is empty", name)
	}

	seen := make(map[string]bool, len(set))
	for _, q := range set {
		switch {
		case q.ID == "":
			return nil, fmt.Errorf("eval: golden set %s: a query has no id", name)
		case seen[q.ID]:
			return nil, fmt.Errorf("eval: golden set %s: duplicate query id %q", name, q.ID)
		case strings.TrimSpace(q.Query) == "":
			return nil, fmt.Errorf("eval: golden query %q has empty text; an empty query takes the browse path, where relevance is undefined", q.ID)
		case q.Language == "":
			return nil, fmt.Errorf("eval: golden query %q has no language; the report groups by it", q.ID)
		}
		seen[q.ID] = true

		relevant := 0
		for key, grade := range q.Judgements {
			if grade < 0 || grade > 2 {
				return nil, fmt.Errorf("eval: golden query %q grades %s as %d; grades are 0, 1 or 2", q.ID, key, grade)
			}
			if grade > 0 {
				relevant++
			}
		}
		if relevant == 0 {
			return nil, fmt.Errorf("eval: golden query %q has no relevant tenders; it would score 0 whatever the engine does", q.ID)
		}
	}
	return set, nil
}
