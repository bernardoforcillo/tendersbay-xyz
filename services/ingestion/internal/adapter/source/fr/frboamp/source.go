// Package frboamp registers France's BOAMP as an ingestion.Source. It wires
// boampapi (transport) and boamp (protocol/mapping) together; neither of those
// packages knows about the other's caller.
package frboamp

import (
	"context"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/fr/boamp"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/fr/boampapi"
)

// fetchWindow is how far back each cycle looks. BOAMP publishes on a daily
// cadence and dates notices by calendar day (dateparution has no time), so a
// 24h window captures a day's publications while keeping each fetch small.
const fetchWindow = 24 * time.Hour

// Source is the registered ingestion.Source for BOAMP.
type Source struct {
	api *boampapi.Client
}

// New returns a Source wired to the real BOAMP dataset.
func New() *Source {
	return &Source{api: boampapi.New()}
}

// Name returns "fr-boamp" — stored as tender.Tender.Source on every tender this
// provider produces.
func (s *Source) Name() string { return "fr-boamp" }

// Fetch queries BOAMP for records published in the last fetchWindow and maps
// each into a tender.Tender.
func (s *Source) Fetch(ctx context.Context) ([]tender.Tender, error) {
	records, err := s.api.FetchSince(ctx, time.Now().UTC().Add(-fetchWindow))
	if err != nil {
		return nil, err
	}
	tenders := make([]tender.Tender, len(records))
	for i, r := range records {
		tenders[i] = boamp.Map(r, s.Name())
	}
	return tenders, nil
}
