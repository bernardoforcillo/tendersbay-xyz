// Package postgres — this file adds a READ-ONLY reference into
// tenders.ingested_tenders, a table owned and migrated exclusively by
// services/ingestion. TenderRepo never writes to it and this service's
// migrator (db.go) never manages its schema.
package postgres

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/bernardoforcillo/drops"
	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

// TenderFilters narrows a tender search. Zero-value fields are ignored
// (not turned into "= ”" predicates).
type TenderFilters struct {
	Country      string
	CPV          string
	Status       string
	DeadlineFrom *time.Time
	DeadlineTo   *time.Time
}

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

var tenderResultColumns = []drops.Expression{
	TenderID, TenderTitle, TenderBuyerName, TenderStatus, TenderProcedureType,
	TenderCountry, TenderCPV, TenderValue, TenderCurrency, TenderPublishedAt,
	TenderDeadline, TenderSource, TenderSourceRef, TenderNUTS, TDocURL,
}

// SearchByFilters returns up to limit tenders matching filters, ordered by
// published_at descending starting at offset. Pass limit+1 from the
// caller to compute has_more without a separate COUNT(*).
func (r *TenderRepo) SearchByFilters(ctx context.Context, filters TenderFilters, limit, offset int) ([]TenderResultRow, error) {
	q := r.db.Select(tenderResultColumns...).From(Tenders).
		LeftJoin(TenderDocuments, pg.And(TDocTenderID.EqCol(TenderID), TDocType.Eq("notice"))).
		OrderBy(TenderPublishedAt.Desc()).Limit(int64(limit)).Offset(int64(offset))
	if preds := filterPredicates(filters); len(preds) > 0 {
		q = q.Where(preds...)
	}
	var rows []DBTender
	if err := q.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("postgres: search tenders by filters: %w", err)
	}
	return dbTendersToRows(rows), nil
}

// FindByIDs returns the tenders among ids that also match filters, in no
// particular order (callers needing a specific order — e.g. by relevance
// score — re-sort in Go). Malformed entries in ids are silently skipped
// rather than failing the whole query, since ids typically originates
// from an external system's (Qdrant's) payload.
func (r *TenderRepo) FindByIDs(ctx context.Context, ids []int64, filters TenderFilters) ([]TenderResultRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	preds := append([]drops.Expression{TenderID.In(ids...)}, filterPredicates(filters)...)
	var rows []DBTender
	if err := r.db.Select(tenderResultColumns...).From(Tenders).
		LeftJoin(TenderDocuments, pg.And(TDocTenderID.EqCol(TenderID), TDocType.Eq("notice"))).
		Where(preds...).All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("postgres: find tenders by ids: %w", err)
	}
	return dbTendersToRows(rows), nil
}

func filterPredicates(f TenderFilters) []drops.Expression {
	var preds []drops.Expression
	if f.Country != "" {
		preds = append(preds, TenderCountry.Eq(f.Country))
	}
	if f.CPV != "" {
		preds = append(preds, TenderCPV.Like(f.CPV+"%"))
	}
	if f.Status != "" {
		preds = append(preds, TenderStatus.Eq(f.Status))
	}
	if f.DeadlineFrom != nil {
		preds = append(preds, TenderDeadline.Gte(*f.DeadlineFrom))
	}
	if f.DeadlineTo != nil {
		preds = append(preds, TenderDeadline.Lte(*f.DeadlineTo))
	}
	return preds
}

func dbTendersToRows(rows []DBTender) []TenderResultRow {
	out := make([]TenderResultRow, len(rows))
	for i, row := range rows {
		out[i] = TenderResultRow{
			ID: row.ID, Title: row.Title, BuyerName: row.BuyerName, Status: row.Status,
			ProcedureType: row.ProcedureType, Country: row.Country, CPV: row.CPV,
			Value: row.Value, Currency: row.Currency, PublishedAt: row.PublishedAt,
			Deadline: row.Deadline, Source: row.Source, SourceRef: row.SourceRef,
			NUTS: row.NUTS, SourceURL: row.SourceURL,
		}
	}
	return out
}

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

// SearchTenders satisfies tender.Repo, adapting this file's
// Postgres-shaped SearchByFilters (int64 ids, TenderFilters) to
// core/tender's domain-shaped types (string ids, tender.Filters).
func (r *TenderRepo) SearchTenders(ctx context.Context, filters tender.Filters, limit, offset int) ([]tender.Tender, error) {
	rows, err := r.SearchByFilters(ctx, toTenderFilters(filters), limit, offset)
	if err != nil {
		return nil, err
	}
	return rowsToTenders(rows), nil
}

// EnrichTenders satisfies tender.Repo, adapting FindByIDs the same way.
// Malformed or unparseable entries in ids are silently skipped (see
// tenderIDFromString) rather than failing the whole search.
func (r *TenderRepo) EnrichTenders(ctx context.Context, ids []string, filters tender.Filters) ([]tender.Tender, error) {
	numeric := make([]int64, 0, len(ids))
	for _, id := range ids {
		if n, ok := tenderIDFromString(id); ok {
			numeric = append(numeric, n)
		}
	}
	rows, err := r.FindByIDs(ctx, numeric, toTenderFilters(filters))
	if err != nil {
		return nil, err
	}
	return rowsToTenders(rows), nil
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

func toTenderFilters(f tender.Filters) TenderFilters {
	return TenderFilters{
		Country: f.Country, CPV: f.CPV, Status: f.Status,
		DeadlineFrom: f.DeadlineFrom, DeadlineTo: f.DeadlineTo,
	}
}

func rowsToTenders(rows []TenderResultRow) []tender.Tender {
	out := make([]tender.Tender, len(rows))
	for i, row := range rows {
		var sourceURL string
		if row.SourceURL != nil {
			sourceURL = *row.SourceURL
		}
		out[i] = tender.Tender{
			ID: strconv.FormatInt(row.ID, 10), Title: row.Title, BuyerName: row.BuyerName,
			Status: row.Status, ProcedureType: row.ProcedureType, Country: row.Country,
			CPV: row.CPV, Value: row.Value, Currency: row.Currency,
			PublishedAt: row.PublishedAt, Deadline: row.Deadline,
			Source: row.Source, SourceRef: row.SourceRef,
			NUTS: row.NUTS, SourceURL: sourceURL,
		}
	}
	return out
}
