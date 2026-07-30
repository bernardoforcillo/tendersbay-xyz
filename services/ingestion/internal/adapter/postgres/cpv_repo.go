// Package postgres — this file holds the CPV vocabulary writes. They are raw
// parameterised SQL for the same reason the tender writes are: a multi-row
// VALUES list with ON CONFLICT DO UPDATE has no DSL in the drops builder.
package postgres

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/bernardoforcillo/drops/pg"

	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/cpvdata"
)

// cpvBatchSize is how many vocabulary rows go in one INSERT.
//
// 500 rows × 3 placeholders = 1500 parameters, comfortably inside Postgres'
// 65535 limit while keeping the round-trip count for ~227k rows in the
// hundreds rather than the hundreds of thousands.
const cpvBatchSize = 500

const countCPVTermsSQL = `SELECT count(*)::int FROM tenders.cpv_terms`

// CPVRepo writes the CPV vocabulary.
type CPVRepo struct {
	db *pg.DB
}

// NewCPVRepo builds a CPVRepo over db.
func NewCPVRepo(db *pg.DB) *CPVRepo { return &CPVRepo{db: db} }

// UpsertTerms writes rows and returns how many were sent.
//
// ON CONFLICT DO UPDATE rather than DO NOTHING so a vocabulary revision
// actually lands: a label that changed must overwrite the old one, and
// label_vector is generated so it follows automatically — it must never
// appear in the column list or the SET clause, or Postgres rejects the
// statement with "cannot insert a non-DEFAULT value into column". This is
// what makes seeding safe to re-run.
//
// Batches share one transaction per batch, not one for the whole vocabulary: a
// single 227k-row transaction would hold locks for minutes on a table other
// queries read.
func (r *CPVRepo) UpsertTerms(ctx context.Context, rows []cpvdata.Row) (int, error) {
	sent := 0
	for start := 0; start < len(rows); start += cpvBatchSize {
		end := start + cpvBatchSize
		if end > len(rows) {
			end = len(rows)
		}
		batch := rows[start:end]

		placeholders, args := buildUpsertValues(batch)
		sql := `INSERT INTO tenders.cpv_terms (code, lang, label) VALUES ` +
			placeholders +
			` ON CONFLICT (code, lang) DO UPDATE SET label = EXCLUDED.label`

		if _, err := r.db.Exec(ctx, sql, args...); err != nil {
			return sent, fmt.Errorf("postgres: upsert cpv terms [%d:%d]: %w", start, end, err)
		}
		sent += len(batch)
	}
	return sent, nil
}

// buildUpsertValues renders batch as a parenthesised, comma-joined VALUES
// placeholder list ("($1, $2, $3), ($4, $5, $6)") alongside the args slice it
// references, each row contributing (code, lang, label) in that order.
//
// Pulled out of UpsertTerms as a pure function so this arithmetic — the
// riskiest part of the whole task — can be pinned by a plain unit test
// instead of only ever running through a live database. code, lang and label
// are all `text`: a misaligned bind would not error, it would silently seed
// a label under the wrong language, and every downstream consumer of the CPV
// bridge would trust the corrupted row.
func buildUpsertValues(batch []cpvdata.Row) (string, []any) {
	var (
		placeholders = make([]string, len(batch))
		args         = make([]any, 0, len(batch)*3)
	)
	for i, row := range batch {
		n := i * 3
		placeholders[i] = "($" + strconv.Itoa(n+1) + ", $" + strconv.Itoa(n+2) + ", $" + strconv.Itoa(n+3) + ")"
		args = append(args, row.Code, row.Lang, row.Label)
	}
	return strings.Join(placeholders, ", "), args
}

// CountTerms is how many vocabulary rows the table holds. Used by the seeder to
// report progress and by an operator to confirm a seed landed.
func (r *CPVRepo) CountTerms(ctx context.Context) (int, error) {
	rows, err := r.db.Query(ctx, countCPVTermsSQL)
	if err != nil {
		return 0, fmt.Errorf("postgres: count cpv terms: %w", err)
	}
	defer rows.Close()
	var n int
	if rows.Next() {
		if err := rows.Scan(&n); err != nil {
			return 0, fmt.Errorf("postgres: scan cpv term count: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("postgres: count cpv terms: %w", err)
	}
	return n, nil
}
