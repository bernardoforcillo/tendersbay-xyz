package postgres

import (
	"context"
	"strings"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/core/ingestion"
)

// TenderRepo implements ingestion.Sink against the `tenders` schema.
type TenderRepo struct {
	db *pg.DB
}

// NewTenderRepo builds a TenderRepo over db.
func NewTenderRepo(db *pg.DB) *TenderRepo {
	return &TenderRepo{db: db}
}

const upsertTenderSQL = `
INSERT INTO tenders.ingested_tenders (
	source, source_ref, title, buyer_name, buyer_id, status, procedure_type,
	language, country, nuts, cpv, cpv_secondary, value, currency,
	published_at, deadline, raw, version, history, first_seen_at, last_seen_at
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::text[], $13, $14, $15, $16, $17::jsonb,
	1, '[]'::jsonb, now(), now()
)
ON CONFLICT (source, source_ref) DO UPDATE SET
	title = EXCLUDED.title,
	buyer_name = EXCLUDED.buyer_name,
	buyer_id = EXCLUDED.buyer_id,
	procedure_type = EXCLUDED.procedure_type,
	language = EXCLUDED.language,
	country = EXCLUDED.country,
	nuts = EXCLUDED.nuts,
	cpv = EXCLUDED.cpv,
	cpv_secondary = EXCLUDED.cpv_secondary,
	value = EXCLUDED.value,
	currency = EXCLUDED.currency,
	published_at = EXCLUDED.published_at,
	deadline = EXCLUDED.deadline,
	raw = EXCLUDED.raw,
	version = CASE WHEN tenders.ingested_tenders.status IS DISTINCT FROM EXCLUDED.status
	               THEN tenders.ingested_tenders.version + 1
	               ELSE tenders.ingested_tenders.version END,
	history = CASE WHEN tenders.ingested_tenders.status IS DISTINCT FROM EXCLUDED.status
	               THEN tenders.ingested_tenders.history || jsonb_build_object(
	                      'event', 'status_changed',
	                      'from', tenders.ingested_tenders.status,
	                      'to', EXCLUDED.status,
	                      'at', now()
	                    )
	               ELSE tenders.ingested_tenders.history END,
	status = EXCLUDED.status,
	last_seen_at = now()
RETURNING id, (xmax = 0) AS inserted
`

const upsertDocumentSQL = `
INSERT INTO tenders.ingested_tender_documents (tender_id, url, type)
VALUES ($1, $2, $3)
ON CONFLICT (tender_id, url) DO UPDATE SET type = EXCLUDED.type
`

const upsertLotSQL = `
INSERT INTO tenders.ingested_tender_lots (tender_id, ref, title, cpv, value, currency, deadline)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (tender_id, ref) DO UPDATE SET
	title = EXCLUDED.title,
	cpv = EXCLUDED.cpv,
	value = EXCLUDED.value,
	currency = EXCLUDED.currency,
	deadline = EXCLUDED.deadline
`

const insertRunSQL = `
INSERT INTO tenders.ingestion_runs (source, started_at, finished_at, fetched, inserted, updated, error)
VALUES ($1, $2, $3, $4, $5, $6, $7)
`

// pgTextArray renders a Go string slice as a PostgreSQL array literal, e.g.
// []string{"a", "b"} -> `{"a","b"}`. CPVSecondary crosses the database/sql
// boundary as a single text parameter, cast to text[] in SQL (see
// upsertTenderSQL's $12::text[]) rather than relying on driver-specific
// slice encoding.
func pgTextArray(vals []string) string {
	quoted := make([]string, len(vals))
	for i, v := range vals {
		escaped := strings.ReplaceAll(v, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		quoted[i] = `"` + escaped + `"`
	}
	return "{" + strings.Join(quoted, ",") + "}"
}

// Save upserts each tender (and its documents/lots) in its own transaction,
// so a failure on one tender doesn't roll back its siblings' rows.
func (r *TenderRepo) Save(ctx context.Context, tenders []tender.Tender) (ingestion.SaveResult, error) {
	var result ingestion.SaveResult
	for _, t := range tenders {
		inserted, err := r.saveOne(ctx, t)
		if err != nil {
			return result, err
		}
		if inserted {
			result.Inserted++
		} else {
			result.Updated++
		}
	}
	return result, nil
}

func (r *TenderRepo) saveOne(ctx context.Context, t tender.Tender) (inserted bool, err error) {
	err = r.db.InTx(ctx, func(tx *pg.DB) error {
		rows, qErr := tx.Query(ctx, upsertTenderSQL,
			t.Source, t.SourceRef, t.Title, t.Buyer.Name, t.Buyer.ID, string(t.Status),
			t.ProcedureType, t.Language, t.Country, t.NUTS, t.CPV,
			pgTextArray(t.CPVSecondary), t.Value, t.Currency,
			t.PublishedAt, t.Deadline, []byte(t.Raw),
		)
		if qErr != nil {
			return qErr
		}
		var tenderID int64
		if rows.Next() {
			if scanErr := rows.Scan(&tenderID, &inserted); scanErr != nil {
				rows.Close()
				return scanErr
			}
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			rows.Close()
			return rowsErr
		}
		rows.Close()

		for _, d := range t.Documents {
			if _, execErr := tx.Exec(ctx, upsertDocumentSQL, tenderID, d.URL, d.Type); execErr != nil {
				return execErr
			}
		}
		for _, l := range t.Lots {
			if _, execErr := tx.Exec(ctx, upsertLotSQL, tenderID, l.Ref, l.Title, l.CPV, l.Value, l.Currency, l.Deadline); execErr != nil {
				return execErr
			}
		}
		return nil
	})
	return inserted, err
}

// RecordRun writes one audit row for a provider's outcome in one cycle.
func (r *TenderRepo) RecordRun(ctx context.Context, rec ingestion.RunRecord) error {
	var errMsg *string
	if rec.Err != nil {
		msg := rec.Err.Error()
		errMsg = &msg
	}
	_, err := r.db.Exec(ctx, insertRunSQL,
		rec.Source, rec.StartedAt, rec.FinishedAt, rec.Fetched, rec.Inserted, rec.Updated, errMsg,
	)
	return err
}
