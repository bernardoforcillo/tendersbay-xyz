// Package esplacsp registers Spain's PLACSP (Plataforma de Contratación del
// Sector Público) as an ingestion.Source. It wires placspapi (ATOM transport)
// and codice (CODICE/UBL protocol + mapping) together; neither of those
// packages knows about the other's caller.
package esplacsp

import (
	"context"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/es/codice"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/es/placspapi"
)

// fetchWindow is how far back each cycle looks. PLACSP's ATOM feed is a
// rolling, newest-first stream filtered client-side by publication time, so a
// 24h window comfortably covers the hourly ingestion CronJob (and tolerates
// several missed runs) while keeping each fetch to a few feed pages.
const fetchWindow = 24 * time.Hour

// Source is the registered ingestion.Source for PLACSP.
type Source struct {
	api *placspapi.Client
}

// New returns a Source wired to the real PLACSP syndication feed.
func New() *Source {
	return &Source{api: placspapi.New()}
}

// Name returns "es-placsp" — stored as tender.Tender.Source on every tender
// this provider produces.
func (s *Source) Name() string { return "es-placsp" }

// Fetch pulls the CODICE documents published in the last fetchWindow and maps
// each into a tender.Tender.
func (s *Source) Fetch(ctx context.Context) ([]tender.Tender, error) {
	docs, err := s.api.FetchSince(ctx, time.Now().UTC().Add(-fetchWindow))
	if err != nil {
		return nil, err
	}
	tenders := make([]tender.Tender, len(docs))
	for i, d := range docs {
		tenders[i] = codice.Map(d, s.Name())
	}
	return tenders, nil
}
