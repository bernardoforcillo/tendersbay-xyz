// Package ted registers TED (Tenders Electronic Daily) as an
// ingestion.Source. It wires tedapi (transport) and eforms
// (protocol/mapping) together; neither of those packages knows about the
// other's caller.
package ted

import (
	"context"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/eforms"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/tedapi"
)

// fetchWindow is how far back each cycle looks. The ingestion CronJob
// fires hourly; 3h (3x the interval) tolerates exactly one missed run
// without a gap, while keeping each fetch to roughly one Search API page.
// Source deliberately has no persistence access to compute a precise
// high-water mark instead — see the design doc's "Incremental fetch
// strategy" section for the full trade-off.
const fetchWindow = 3 * time.Hour

// Source is the registered ingestion.Source for TED.
type Source struct {
	api *tedapi.Client
}

// New returns a Source wired to the real TED Search API.
func New() *Source {
	return &Source{api: tedapi.New()}
}

// Name returns "ted" — stored as tender.Tender.Source on every tender this
// provider produces.
func (s *Source) Name() string { return "ted" }

// Fetch queries TED for notices published in the last fetchWindow and maps
// each into a tender.Tender.
func (s *Source) Fetch(ctx context.Context) ([]tender.Tender, error) {
	notices, err := s.api.FetchSince(ctx, time.Now().UTC().Add(-fetchWindow))
	if err != nil {
		return nil, err
	}
	tenders := make([]tender.Tender, len(notices))
	for i, n := range notices {
		tenders[i] = eforms.Map(n, s.Name())
	}
	return tenders, nil
}
