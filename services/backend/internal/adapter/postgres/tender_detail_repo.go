package postgres

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bernardoforcillo/drops"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

var tenderDetailColumns = []drops.Expression{
	TenderID, TenderSource, TenderSourceRef, TenderTitle, TenderBuyerName, TenderBuyerID,
	TenderStatus, TenderProcedureType, TenderCountry, TenderNUTS, TenderLanguage,
	TenderCPV, TenderValue, TenderCurrency, TenderPublishedAt, TenderDeadline,
}

// FindDetailByID returns the full detail row for id, or tender.ErrTenderNotFound.
func (r *TenderRepo) FindDetailByID(ctx context.Context, id int64) (*tender.TenderDetail, error) {
	var rows []DBTenderDetail
	if err := r.db.Select(tenderDetailColumns...).From(Tenders).Where(TenderID.Eq(id)).All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("postgres: find tender detail: %w", err)
	}
	if len(rows) == 0 {
		return nil, tender.ErrTenderNotFound
	}
	cpvSecondary, err := r.cpvSecondary(ctx, id)
	if err != nil {
		return nil, err
	}
	docs, err := r.DocumentsByTenderID(ctx, id)
	if err != nil {
		return nil, err
	}
	lots, err := r.LotsByTenderID(ctx, id)
	if err != nil {
		return nil, err
	}
	d := detailRowToDomain(rows[0])
	d.CPVSecondary = cpvSecondary
	d.Documents = docs
	d.Lots = lots
	return &d, nil
}

// cpvSecondary reads the text[] column via array_to_string (drops' typed
// builder can't scan a Postgres array), returning nil for an empty array.
func (r *TenderRepo) cpvSecondary(ctx context.Context, id int64) ([]string, error) {
	rows, err := r.db.Query(ctx,
		"SELECT array_to_string(cpv_secondary, ',') FROM tenders.ingested_tenders WHERE id = $1", id)
	if err != nil {
		return nil, fmt.Errorf("postgres: cpv_secondary: %w", err)
	}
	defer rows.Close()
	var csv string
	if rows.Next() {
		if err := rows.Scan(&csv); err != nil {
			return nil, fmt.Errorf("postgres: scan cpv_secondary: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: cpv_secondary rows: %w", err)
	}
	if csv == "" {
		return nil, nil
	}
	return strings.Split(csv, ","), nil
}

func (r *TenderRepo) DocumentsByTenderID(ctx context.Context, id int64) ([]tender.Document, error) {
	var rows []DBTenderDocument
	if err := r.db.Select(TenderDocURL, TenderDocType).From(TenderDocuments).
		Where(TenderDocTenderID.Eq(id)).OrderBy(TenderDocURL.Asc()).All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("postgres: tender documents: %w", err)
	}
	out := make([]tender.Document, len(rows))
	for i, row := range rows {
		out[i] = tender.Document{URL: row.URL, Type: row.Type}
	}
	return out, nil
}

func (r *TenderRepo) LotsByTenderID(ctx context.Context, id int64) ([]tender.Lot, error) {
	var rows []DBTenderLot
	if err := r.db.Select(TenderLotRef, TenderLotTitle, TenderLotCPV, TenderLotValue, TenderLotCurrency, TenderLotDeadline).
		From(TenderLots).Where(TenderLotTenderID.Eq(id)).OrderBy(TenderLotRef.Asc()).All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("postgres: tender lots: %w", err)
	}
	out := make([]tender.Lot, len(rows))
	for i, row := range rows {
		out[i] = tender.Lot{Ref: row.Ref, Title: row.Title, CPV: row.CPV, Value: row.Value, Currency: row.Currency, Deadline: row.Deadline}
	}
	return out, nil
}

// RecentTenderRefs returns up to limit tenders (id + published_at) newest first,
// for the dynamic sitemap. limit is clamped to [1, 50000].
func (r *TenderRepo) RecentTenderRefs(ctx context.Context, limit int) ([]tender.TenderRef, error) {
	if limit <= 0 || limit > 50000 {
		limit = 50000
	}
	type refRow struct {
		ID          int64      `drop:"id"`
		PublishedAt *time.Time `drop:"published_at"`
	}
	var rows []refRow
	if err := r.db.Select(TenderID, TenderPublishedAt).From(Tenders).
		OrderBy(TenderPublishedAt.Desc()).Limit(int64(limit)).All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("postgres: recent tender refs: %w", err)
	}
	out := make([]tender.TenderRef, len(rows))
	for i, row := range rows {
		lastmod := ""
		if row.PublishedAt != nil {
			lastmod = row.PublishedAt.Format(time.RFC3339)
		}
		out[i] = tender.TenderRef{ID: strconv.FormatInt(row.ID, 10), Lastmod: lastmod}
	}
	return out, nil
}

func detailRowToDomain(row DBTenderDetail) tender.TenderDetail {
	return tender.TenderDetail{
		ID: strconv.FormatInt(row.ID, 10), Source: row.Source, SourceRef: row.SourceRef,
		Title: row.Title, BuyerName: row.BuyerName, BuyerID: row.BuyerID, Status: row.Status,
		ProcedureType: row.ProcedureType, Country: row.Country, NUTS: row.NUTS, Language: row.Language,
		CPV: row.CPV, Value: row.Value, Currency: row.Currency,
		PublishedAt: row.PublishedAt, Deadline: row.Deadline,
	}
}
