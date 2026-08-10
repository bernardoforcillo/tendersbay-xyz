package webdoc

import "github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/core/retrieve"

// The three ports this package exists to satisfy, asserted at compile time.
//
// The retrieval orchestration wires all three in, so a signature drift would
// eventually fail there too — but it would fail in cmd/retriever, several
// packages away from the code that broke, and only for whoever built the
// binary. Declaring it here puts the failure in this package's own build, and
// it is also the only mechanical statement of the dependency direction the
// house rules require: every one of these interfaces is DECLARED in core and
// SATISFIED here, never the other way round.
var (
	_ retrieve.PortalFetcher     = (*Client)(nil)
	_ retrieve.ListingEnumerator = (*Registry)(nil)
	_ retrieve.TextExtractor     = Extractor{}
)
