package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/espd"
)

// EspdStore persists the buyer's ESPD request per bid and the export audit
// trail. It satisfies espd.RequestStore and espd.ExportLog.
//
// The request XML is stored as text on purpose: it is a small document (tens
// of KB) that must be re-readable to recompose the response, and a row keyed
// by bid_id is the cheapest place that guarantees it disappears with the bid.
// The export row carries a content hash and never the bytes.
type EspdStore struct{ db *pg.DB }

func NewEspdStore(db *pg.DB) *EspdStore { return &EspdStore{db: db} }

var (
	_ espd.RequestStore = (*EspdStore)(nil)
	_ espd.ExportLog    = (*EspdStore)(nil)
)

// Get returns the stored request, re-parsed from its XML so the Criteria and
// Lots reflect the CURRENT taxonomy rather than the one in force at import.
// ImportedBy/ImportedAt come from the row.
func (s *EspdStore) Get(ctx context.Context, bidID string) (espd.Request, error) {
	var row DBEspdRequest
	err := s.db.Select().From(BidEspdRequests).Where(ERBidID.Eq(bidID)).One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return espd.Request{}, espd.ErrRequestNotFound
	}
	if err != nil {
		return espd.Request{}, err
	}
	req, err := espd.ParseRequest([]byte(row.XML))
	if err != nil {
		return espd.Request{}, err
	}
	req.ImportedBy = row.ImportedBy
	req.ImportedAt = row.ImportedAt
	return req, nil
}

// Put upserts the request for a bid. raw is the XML the Request was parsed
// from, and it is what gets stored: the parsed struct is derived, the bytes are
// the source of truth, and Get re-parses them so a request imported before we
// learned a criterion code is re-read with the newer mapping.
func (s *EspdStore) Put(ctx context.Context, bidID string, r espd.Request, raw []byte) error {
	importedAt := r.ImportedAt
	if importedAt.IsZero() {
		importedAt = time.Now()
	}
	set := []pg.ColumnValue{
		ERVersion.Val(string(r.Version)),
		ERXML.Val(string(raw)),
		ERSHA256.Val(r.SHA256),
		ERImportedBy.Val(r.ImportedBy),
		ERImportedAt.Val(importedAt),
	}
	_, err := s.db.Insert(BidEspdRequests).
		Row(append([]pg.ColumnValue{ERBidID.Val(bidID)}, set...)...).
		OnConflictUpdate(ERBidID).
		Set(set...).
		Done().
		Exec(ctx)
	return err
}

// Record appends one export to the audit trail.
func (s *EspdStore) Record(ctx context.Context, e espd.Export) error {
	exportedAt := e.ExportedAt
	if exportedAt.IsZero() {
		exportedAt = time.Now()
	}
	_, err := s.db.Insert(BidEspdExports).
		Row(
			EXBidID.Val(e.BidID),
			EXUserID.Val(e.UserID),
			EXVersion.Val(string(e.Version)),
			EXFormat.Val(string(e.Format)),
			EXContentSHA256.Val(e.ContentSHA256),
			EXDeclarationsConfirmedAt.Val(e.DeclarationsConfirmedAt),
			EXExportedAt.Val(exportedAt),
		).
		Exec(ctx)
	return err
}

// List returns a bid's exports, newest first.
func (s *EspdStore) List(ctx context.Context, bidID string) ([]espd.Export, error) {
	var rows []DBEspdExport
	err := s.db.Select().From(BidEspdExports).Where(EXBidID.Eq(bidID)).
		OrderBy(EXExportedAt.Desc(), EXID.Asc()).All(ctx, &rows)
	if err != nil {
		return nil, err
	}
	out := make([]espd.Export, len(rows))
	for i, row := range rows {
		out[i] = espd.Export{
			ID: row.ID, BidID: row.BidID, UserID: row.ExportedBy,
			Version: espd.Version(row.Version), Format: espd.Format(row.Format),
			ContentSHA256: row.ContentSHA256, DeclarationsConfirmedAt: row.DeclarationsConfirmedAt, ExportedAt: row.ExportedAt,
		}
	}
	return out, nil
}
