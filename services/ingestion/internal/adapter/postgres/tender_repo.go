package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/core/ingestion"
)

// TenderRepo implements ingestion.Sink against the `tenders` schema.
type TenderRepo struct {
	db *pg.DB
}

// UnindexedTender is one row from ListUnindexed — a tender that hasn't
// been successfully indexed into the vector search collection yet, either
// because it was just ingested or a prior indexing attempt failed. Both
// cases look identical (indexed_at IS NULL), so ListUnindexed serves both.
type UnindexedTender struct {
	ID            int64
	Title         string
	BuyerName     string
	CPV           string
	ProcedureType string
	Country       string
	Status        string
	Source        string
	SourceRef     string
	Documents     []UnindexedDocument
}

// UnindexedDocument is one document attached to an UnindexedTender.
type UnindexedDocument struct {
	ID  int64
	URL string
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
	last_seen_at = now(),
	indexed_at = CASE WHEN tenders.ingested_tenders.title IS DISTINCT FROM EXCLUDED.title
	                    OR tenders.ingested_tenders.buyer_name IS DISTINCT FROM EXCLUDED.buyer_name
	                    OR tenders.ingested_tenders.cpv IS DISTINCT FROM EXCLUDED.cpv
	                    OR tenders.ingested_tenders.procedure_type IS DISTINCT FROM EXCLUDED.procedure_type
	                    OR tenders.ingested_tenders.country IS DISTINCT FROM EXCLUDED.country
	                    OR tenders.ingested_tenders.status IS DISTINCT FROM EXCLUDED.status
	               THEN NULL
	               ELSE tenders.ingested_tenders.indexed_at END
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

const upsertDocumentPartSQL = `
INSERT INTO tenders.ingested_tender_document_parts (document_id, index, content)
VALUES ($1, $2, $3)
ON CONFLICT (document_id, index) DO UPDATE SET content = EXCLUDED.content
`

const selectDocumentPartsSQL = `
SELECT content FROM tenders.ingested_tender_document_parts
WHERE document_id = $1 ORDER BY index
`

const selectUnindexedTendersSQL = `
SELECT id, title, buyer_name, cpv, procedure_type, country, status, source, source_ref
FROM tenders.ingested_tenders
WHERE indexed_at IS NULL
ORDER BY id
LIMIT $1
`

const selectDocumentsForTenderSQL = `
SELECT id, url FROM tenders.ingested_tender_documents WHERE tender_id = $1 ORDER BY id
`

const markIndexedSQL = `
UPDATE tenders.ingested_tenders SET indexed_at = now() WHERE id = $1
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

// SaveDocumentParts upserts the extracted text parts of one document,
// keyed by (document_id, index) — idempotent, so re-extracting the same
// document updates content in place rather than duplicating rows. All
// parts are saved in a single transaction so a mid-loop failure can't
// commit a partial set of parts: a partial commit would look
// indistinguishable from a complete one to indexOne's len(parts) == 0
// reuse-gate check on the next cycle, causing it to skip re-fetching and
// index the tender with incomplete text.
func (r *TenderRepo) SaveDocumentParts(ctx context.Context, documentID int64, parts []string) error {
	err := r.db.InTx(ctx, func(tx *pg.DB) error {
		for i, p := range parts {
			if _, err := tx.Exec(ctx, upsertDocumentPartSQL, documentID, i, p); err != nil {
				return fmt.Errorf("postgres: save document part %d for document %d: %w", i, documentID, err)
			}
		}
		return nil
	})
	return err
}

// DocumentParts returns the previously-extracted text parts of one
// document, in order — an empty slice if none have been saved yet.
func (r *TenderRepo) DocumentParts(ctx context.Context, documentID int64) ([]string, error) {
	rows, err := r.db.Query(ctx, selectDocumentPartsSQL, documentID)
	if err != nil {
		return nil, fmt.Errorf("postgres: document parts for document %d: %w", documentID, err)
	}
	defer rows.Close()

	var parts []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("postgres: scan document part for document %d: %w", documentID, err)
		}
		parts = append(parts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: document parts for document %d: %w", documentID, err)
	}
	return parts, nil
}

// ListUnindexed returns up to limit tenders that haven't been indexed
// yet, each with its attached documents.
func (r *TenderRepo) ListUnindexed(ctx context.Context, limit int) ([]UnindexedTender, error) {
	rows, err := r.db.Query(ctx, selectUnindexedTendersSQL, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: list unindexed tenders: %w", err)
	}
	var tenders []UnindexedTender
	for rows.Next() {
		var t UnindexedTender
		if err := rows.Scan(&t.ID, &t.Title, &t.BuyerName, &t.CPV, &t.ProcedureType,
			&t.Country, &t.Status, &t.Source, &t.SourceRef); err != nil {
			rows.Close()
			return nil, fmt.Errorf("postgres: scan unindexed tender: %w", err)
		}
		tenders = append(tenders, t)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("postgres: list unindexed tenders: %w", err)
	}
	rows.Close()

	for i := range tenders {
		docRows, err := r.db.Query(ctx, selectDocumentsForTenderSQL, tenders[i].ID)
		if err != nil {
			return nil, fmt.Errorf("postgres: documents for tender %d: %w", tenders[i].ID, err)
		}
		for docRows.Next() {
			var d UnindexedDocument
			if err := docRows.Scan(&d.ID, &d.URL); err != nil {
				docRows.Close()
				return nil, fmt.Errorf("postgres: scan document for tender %d: %w", tenders[i].ID, err)
			}
			tenders[i].Documents = append(tenders[i].Documents, d)
		}
		if err := docRows.Err(); err != nil {
			docRows.Close()
			return nil, fmt.Errorf("postgres: documents for tender %d: %w", tenders[i].ID, err)
		}
		docRows.Close()
	}
	return tenders, nil
}

// MarkIndexed records that a tender was successfully indexed into the
// vector search collection.
func (r *TenderRepo) MarkIndexed(ctx context.Context, tenderID int64) error {
	if _, err := r.db.Exec(ctx, markIndexedSQL, tenderID); err != nil {
		return fmt.Errorf("postgres: mark tender %d indexed: %w", tenderID, err)
	}
	return nil
}
