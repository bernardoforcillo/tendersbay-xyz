// Package plbzp registers Poland's BZP (Biuletyn Zamówień Publicznych) as an
// ingestion.Source. It wires bzpapi (transport) and bzp (protocol/mapping)
// together, mirroring how the ted package wires tedapi and eforms; neither
// half knows about the other's caller.
package plbzp

import (
	"context"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/pl/bzp"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/pl/bzpapi"
)

// fetchWindow is how far back each cycle looks. Matching ted's rationale: the
// ingestion CronJob fires hourly and 3h (3x the interval) tolerates one missed
// run without a gap. BZP has no server-side date filter, so the window is
// applied client-side by bzpapi on each notice's publicationDate.
const fetchWindow = 3 * time.Hour

// Source is the registered ingestion.Source for Poland's BZP.
type Source struct {
	api    *bzpapi.Client
	window time.Duration
}

// New returns a Source wired to the real BZP board search API.
func New() *Source {
	return &Source{api: bzpapi.New(), window: fetchWindow}
}

// Name returns "pl-bzp" — stored as tender.Tender.Source on every tender this
// provider produces.
func (s *Source) Name() string { return "pl-bzp" }

// Fetch queries BZP for notices published in the last window and maps each
// into a tender.Tender.
func (s *Source) Fetch(ctx context.Context) ([]tender.Tender, error) {
	notices, err := s.api.FetchSince(ctx, time.Now().UTC().Add(-s.window))
	if err != nil {
		return nil, err
	}
	tenders := make([]tender.Tender, len(notices))
	for i, n := range notices {
		tenders[i] = bzp.Map(n, s.Name())
	}
	return tenders, nil
}
