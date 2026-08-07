package document

import (
	"context"
	"fmt"
	"strings"
)

// MaxExcerpts and MaxExcerptRunes are the retrieval token budget, enforced
// HERE rather than in the agent tool that reads it, so that a second consumer
// — core/bid's requirement extraction — cannot obtain an unbounded read simply
// by not knowing the bound exists.
//
// The arithmetic they exist for: a 50-150 page disciplinare is 35k-100k
// tokens. Against a 2,000,000-token default monthly workspace allowance
// (core/credits) and an 8-turn agent loop (core/agent's registry), one
// whole-document read resent on every turn costs 500k+ tokens — a quarter of a
// workspace-month for a single question. 6 x 1200 runes is ~7.2k characters;
// dense Italian legal text tokenizes worse than the 4-chars-per-token the
// usage estimator assumes, so budgeting ~3 chars/token gives ~2.4k tokens per
// call and ~19k across a fully saturated turn. That is ~1% of a
// workspace-month, not 25%.
//
// The metering path systematically under-bills retrieved text (it estimates
// from character length rather than reading the provider's usage), so these
// bounds are also what keeps that estimation error small in absolute terms.
const (
	MaxExcerpts     = 6
	MaxExcerptRunes = 1200
)

// Service answers document questions about one tender at a time.
type Service struct{ reader Reader }

// NewService builds a Service over r.
func NewService(r Reader) *Service { return &Service{reader: r} }

// Excerpts answers one question against one tender's extracted text.
//
// It always loads Availability, even when parts were found, because the two
// halves of the answer are only useful together: "we found no passage about
// penali" and "we hold nothing but the notice PDF" are different answers, and
// a caller that sees only an empty slice will conflate them — which is the
// exact failure this package exists to prevent.
func (s *Service) Excerpts(ctx context.Context, q ExcerptQuery) (ExcerptResult, error) {
	av, err := s.reader.Availability(ctx, q.TenderID)
	if err != nil {
		return ExcerptResult{}, fmt.Errorf("document: availability: %w", err)
	}

	// A blank question cannot match anything — websearch_to_tsquery over an
	// empty string yields an empty query — so the round trip is skipped rather
	// than paid for. The port still contracts to tolerate it, because this is
	// not the only possible caller of a Reader.
	question := strings.TrimSpace(q.Question)
	if question == "" {
		return ExcerptResult{Availability: av}, nil
	}

	limit := q.Limit
	if limit <= 0 || limit > MaxExcerpts {
		limit = MaxExcerpts
	}
	excerpts, err := s.reader.SearchParts(ctx, q.TenderID, question, limit)
	if err != nil {
		return ExcerptResult{}, fmt.Errorf("document: search parts: %w", err)
	}
	// Clamping happens here, not in the adapter's SQL and not in the tool that
	// renders the result: an adapter can be swapped and a tool can be added,
	// but neither can widen a bound it does not own.
	for i := range excerpts {
		excerpts[i].Text, excerpts[i].Truncated = clampRunes(excerpts[i].Text, MaxExcerptRunes)
	}
	return ExcerptResult{Availability: av, Excerpts: excerpts}, nil
}

// Availability is the coverage answer alone, for callers that need to decide
// whether reading is worth it before paying for retrieval.
func (s *Service) Availability(ctx context.Context, tenderID int64) (Availability, error) {
	av, err := s.reader.Availability(ctx, tenderID)
	if err != nil {
		return Availability{}, fmt.Errorf("document: availability: %w", err)
	}
	return av, nil
}

// clampRunes truncates s to at most max runes, reporting whether it had to.
//
// Runes, not bytes: this corpus is 24 languages, and cutting a multi-byte
// character in half produces a replacement glyph in the middle of a quoted
// legal passage. The byte-length pre-check is not just an optimisation — for
// the overwhelmingly common short part it avoids allocating a rune slice as
// wide as the text.
func clampRunes(s string, max int) (string, bool) {
	if len(s) <= max {
		return s, false
	}
	r := []rune(s)
	if len(r) <= max {
		return s, false
	}
	return string(r[:max]), true
}
