package tender

import "context"

// CPVMatch is one CPV code a query resolved to, with the label that matched it.
//
// Lang and Label are carried all the way to the UI, not just used internally:
// a user must be able to see that "pulizie uffici" was read as
// "90919200 — Servizi di pulizia di uffici" and remove it. A signal the user
// cannot see is a signal they cannot correct — the same principle
// SearchOutput.AppliedFilters already exists to honour.
type CPVMatch struct {
	Code  string
	Lang  string // language of the label that matched, lowercase ISO 639-1
	Label string
	Score float64 // relative match strength; only the ordering is meaningful
}

// CPVLexicon resolves free text onto CPV codes.
//
// This is the bridge that makes cross-language search work. A CPV code is
// identical in all 24 EU languages, so a query resolved to codes reaches notices
// written in languages the query is not — something the lexical arm cannot do
// (it matches lexemes) and a stemmer cannot do either (it does not translate).
//
// Defined here, in the consumer, and implemented by the postgres adapter —
// mirroring KnowledgeBase and Repo.
type CPVLexicon interface {
	// MatchCodes returns the best-matching codes, best first, at most limit of
	// them. An unmatched query returns an empty slice and a nil error: "no code
	// describes this" is a normal answer, not a failure.
	MatchCodes(ctx context.Context, query string, limit int) ([]CPVMatch, error)
}
