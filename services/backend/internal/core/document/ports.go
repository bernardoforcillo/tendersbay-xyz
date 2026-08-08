package document

import "context"

// Reader is the port this package defines and an adapter implements.
//
// It is deliberately narrow and deliberately expressed in this package's own
// types only — no *sql.DB, no HTTP status, no vendor vocabulary — so that the
// Phase 3 buyer-portal fetcher can be composed behind the SAME port without
// any core type changing. Satisfied today by *postgres.DocumentRepo.
//
// The asymmetry between the two methods is intentional: Availability is a
// question about state that must always have an answer, while SearchParts is a
// retrieval that is allowed to come back empty. Making "nothing matched" an
// error would force every caller to distinguish it from a real failure, and
// the whole point of returning both together (ExcerptResult) is that the
// caller never has to guess which one it got.
type Reader interface {
	// Availability reports coverage and cause for one tender. It must return a
	// value satisfying Availability.Consistent for every input, and
	// ErrTenderNotFound for an id no tender has.
	Availability(ctx context.Context, tenderID int64) (Availability, error)
	// SearchParts returns the parts of tenderID best matching question,
	// best-first, at most limit of them. An empty question, a non-positive
	// limit, or a tender with no parts returns (nil, nil) — not an error.
	SearchParts(ctx context.Context, tenderID int64, question string, limit int) ([]Excerpt, error)
}
