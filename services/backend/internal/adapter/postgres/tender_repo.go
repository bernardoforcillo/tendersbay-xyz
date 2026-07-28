// Package postgres — this file adds a READ-ONLY reference into
// tenders.ingested_tenders, a table owned and migrated exclusively by
// services/ingestion. TenderRepo never writes to it and this service's
// migrator (db.go) never manages its schema.
//
// The search queries themselves live in tender_search.go, written as raw
// parameterised SQL because lexical retrieval needs constructs the drops
// builder has no DSL for. This file holds the repo type and the thin
// adapters that satisfy core/tender's ports.
package postgres

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

// TenderResultRow is one row of tenders.ingested_tenders, projected down
// to the columns this service's search API needs.
type TenderResultRow struct {
	ID            int64
	Title         string
	BuyerName     string
	Status        string
	ProcedureType string
	Country       string
	CPV           string
	Value         *int64
	Currency      string
	PublishedAt   *time.Time
	Deadline      *time.Time
	Source        string
	SourceRef     string
	NUTS          string
	SourceURL     *string
}

type TenderRepo struct{ db *pg.DB }

// NewTenderRepo builds a TenderRepo over db.
func NewTenderRepo(db *pg.DB) *TenderRepo { return &TenderRepo{db: db} }

// tenderIDFromString parses a Qdrant tender_id payload value (a decimal
// string, e.g. "42") into the int64 this table's id column actually is.
// A malformed value returns ok=false rather than an error — one bad
// payload entry shouldn't fail an entire search.
func tenderIDFromString(s string) (id int64, ok bool) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

var _ tender.Repo = (*TenderRepo)(nil)

// SearchTenders satisfies tender.Repo. Pass limit+1 from the caller to
// compute has_more without a separate COUNT(*).
func (r *TenderRepo) SearchTenders(ctx context.Context, filters tender.Filters, limit, offset int) ([]tender.Tender, error) {
	scored, err := r.SearchByFiltersRanked(ctx, filters, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]tender.Tender, len(scored))
	for i, s := range scored {
		out[i] = s.Tender
	}
	return out, nil
}

// EnrichTenders satisfies tender.Repo. Malformed or unparseable entries in
// ids are silently skipped (see tenderIDFromString) rather than failing the
// whole search, since ids typically originates from an external system's
// (Qdrant's) payload.
func (r *TenderRepo) EnrichTenders(ctx context.Context, ids []string, filters tender.Filters) ([]tender.Tender, error) {
	return r.FindByIDsFiltered(ctx, ids, filters)
}

// DistinctCountries returns each distinct non-empty alpha-2 country with at
// least one ingested tender. Cheap and cacheable; the landing coverage
// marquee is the only caller.
func (r *TenderRepo) DistinctCountries(ctx context.Context) ([]string, error) {
	var rows []struct {
		Country string `drop:"country"`
	}
	if err := r.db.Select(TenderCountry).From(Tenders).
		GroupBy(TenderCountry).All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("postgres: distinct countries: %w", err)
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Country != "" {
			out = append(out, row.Country)
		}
	}
	return out, nil
}
