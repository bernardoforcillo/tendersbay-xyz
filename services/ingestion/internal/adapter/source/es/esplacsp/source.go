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
// each into a tender.Tender. It discards the selection criteria those documents
// carry; FetchWithDetail is the entry point that keeps them, and the ingestion
// service prefers it. Fetch stays because ingestion.Source requires it and
// because "the tenders, without the extras" is a coherent thing to ask for.
func (s *Source) Fetch(ctx context.Context) ([]tender.Tender, error) {
	tenders, _, err := s.FetchWithDetail(ctx)
	return tenders, err
}

// FetchWithDetail implements ingestion.DetailedSource: it returns the same
// tenders as Fetch, plus the selection criteria keyed by the SourceRef each set
// belongs to.
//
// PLACSP is the reason that interface exists. Its ATOM entries embed the whole
// CODICE document rather than linking one, so the admissibility conditions —
// technical and financial capability, required declarations — are already in
// hand at fetch time, where TED publishes a link to a notice document a
// separate pass has to go and read.
//
// The map is keyed on ContractFolderID because that is exactly what Map writes
// to Tender.SourceRef; the sink joins on it, so the two must not drift apart.
// A document publishing no criteria contributes no entry rather than an empty
// one — "the buyer published none" and "we have nothing for this tender" are
// the same statement to the sink, and the shorter map is the cheaper write.
func (s *Source) FetchWithDetail(ctx context.Context) ([]tender.Tender, map[string][]tender.SelectionCriterion, error) {
	docs, err := s.api.FetchSince(ctx, time.Now().UTC().Add(-fetchWindow))
	if err != nil {
		return nil, nil, err
	}

	tenders := make([]tender.Tender, len(docs))
	var criteria map[string][]tender.SelectionCriterion
	for i, d := range docs {
		tenders[i] = codice.Map(d, s.Name())
		if set := codice.MapSelectionCriteria(d, s.Name()); len(set) > 0 {
			if criteria == nil {
				criteria = make(map[string][]tender.SelectionCriterion, len(docs))
			}
			criteria[d.ContractFolderID] = set
		}
	}
	return tenders, criteria, nil
}
