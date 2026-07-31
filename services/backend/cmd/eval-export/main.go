// Command eval-export writes a fixed tender corpus snapshot for the offline
// relevance harness (internal/core/tender/eval).
//
// Run it once against a populated database, commit the output, and leave it
// alone: the harness's numbers are only comparable within one snapshot, so
// regenerating it invalidates the committed baseline.
//
// Balance across source countries is explicit rather than incidental. A
// snapshot drawn purely by recency would be dominated by whichever connector
// ran last, and the cross-language cells of the report — the ones this whole
// effort is about — would be empty.
package main

import (
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender/eval"
)

// selectCorpusSQL caps every source country at $2 rows before the outer
// LIMIT $3 applies.
//
// The outer LIMIT still binds on the ORDER BY country, rn result, not on the
// per-country windowing itself — so if $3 is smaller than
// (distinct countries in the table) * $2, the cut lands mid-alphabet: every
// country up to some letter gets its full per-country share, and every
// country after it gets none, silently. Nothing about that failure is
// visible in a row count or a query error; the query looks balanced (the
// PARTITION BY country window ran correctly) right up until the final LIMIT
// throws away the tail. This cost one export run — see -limit's default and
// checkCountryCoverage below, which now catches it if it happens again.
const selectCorpusSQL = `
SELECT source, source_ref, title, left(description, $1) AS description, buyer_name,
       status, procedure_type, language, country, nuts, cpv,
       coalesce(array_to_json(cpv_secondary)::text, '[]'), value, currency, published_at, deadline
FROM (
	SELECT t.*, row_number() OVER (PARTITION BY t.country ORDER BY t.published_at DESC NULLS LAST, t.id DESC) AS rn
	FROM tenders.ingested_tenders t
	WHERE t.title <> ''
) ranked
WHERE rn <= $2
ORDER BY country, rn
LIMIT $3
`

// selectDistinctCountriesSQL mirrors selectCorpusSQL's non-empty-title filter
// so checkCountryCoverage compares against the same population the export
// draws from, not the whole table.
const selectDistinctCountriesSQL = `
SELECT DISTINCT country FROM tenders.ingested_tenders WHERE title <> ''
`

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	var (
		out = flag.String("out", "internal/core/tender/eval/testdata/corpus.jsonl.gz", "output path")
		// 10000 is deliberately far above any current per-country total
		// (28 known source countries * 120 default = 3360): the previous
		// default of 2000 bound before every country's cap did, and silently
		// dropped nine countries — PL, PT, RO, SE, SI, SK, NL, MT, LV — with
		// no error. See the comment on selectCorpusSQL. checkCountryCoverage
		// below is the backstop if this default is ever undersized again;
		// this default is the first line of defense.
		limit      = flag.Int("limit", 10000, "maximum tenders to export — keep well above (distinct source countries) * -per-country, or ORDER BY country will truncate the alphabet's tail before every country reaches its cap")
		perCountry = flag.Int("per-country", 120, "maximum tenders per source country")
		descMax    = flag.Int("description-max", 4000, "characters of description to keep")
	)
	flag.Parse()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("eval-export: DATABASE_URL is not set")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("eval-export: open database: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	rows, err := db.QueryContext(ctx, selectCorpusSQL, *descMax, *perCountry, *limit)
	if err != nil {
		return fmt.Errorf("eval-export: query corpus: %w", err)
	}
	defer rows.Close()

	// Write to a temp file in *out's own directory and only os.Rename it onto
	// *out once the gzip writer has closed cleanly and the country-coverage
	// guardrail has passed. *out defaults to the committed fixture, and
	// os.Create truncates its target the instant it opens — so writing
	// in place meant any failure after that point (a scan error, an encode
	// error, a bad gzip trailer) replaced a valid committed baseline with an
	// unreadable stub, with no way back short of git checkout. Renaming
	// within the same directory is atomic on the filesystems this runs on:
	// *out is either the old valid file or the new valid one, never a
	// partial write.
	dir := filepath.Dir(*out)
	tmp, err := os.CreateTemp(dir, filepath.Base(*out)+".tmp-*")
	if err != nil {
		return fmt.Errorf("eval-export: create temp file in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	// Unconditional cleanup: if we return before the rename below, the temp
	// file is removed. After a successful rename tmpPath no longer exists
	// (it was moved to *out), so this Remove is a harmless no-op error that
	// is intentionally discarded.
	defer func() { _ = os.Remove(tmpPath) }()

	zw := gzip.NewWriter(tmp)
	enc := json.NewEncoder(zw)

	byCountry := map[string]int{}
	n := 0
	for rows.Next() {
		var (
			t             eval.CorpusTender
			secondaryJSON string
		)
		if err := rows.Scan(&t.Source, &t.SourceRef, &t.Title, &t.Description, &t.BuyerName,
			&t.Status, &t.ProcedureType, &t.Language, &t.Country, &t.NUTS, &t.CPV,
			&secondaryJSON, &t.Value, &t.Currency, &t.PublishedAt, &t.Deadline); err != nil {
			_ = tmp.Close()
			return fmt.Errorf("eval-export: scan row %d: %w", n+1, err)
		}
		// cpv_secondary crosses database/sql as text (no driver-level
		// []string support here), so it has to be re-encoded on the SQL side
		// and decoded here. JSON rather than a comma join is what makes that
		// unambiguous: array_to_json quotes and escapes each element, so a
		// value containing a comma, a quote, or a backslash — real CPV
		// secondary values are hyphenated ("45230000-8"), the column has no
		// CHECK constraint, and tender_repo_test.go in services/ingestion
		// exists specifically to prove backslashes/quotes survive the round
		// trip — still decodes to exactly one array element, never splits.
		if err := json.Unmarshal([]byte(secondaryJSON), &t.CPVSecondary); err != nil {
			_ = tmp.Close()
			return fmt.Errorf("eval-export: decode cpv_secondary for row %d: %w", n+1, err)
		}
		if err := enc.Encode(t); err != nil {
			_ = tmp.Close()
			return fmt.Errorf("eval-export: encode row %d: %w", n+1, err)
		}
		byCountry[t.Country]++
		n++
	}
	if err := rows.Err(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("eval-export: read corpus: %w", err)
	}
	if err := zw.Close(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("eval-export: finish gzip: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("eval-export: close temp file %s: %w", tmpPath, err)
	}
	if n == 0 {
		return fmt.Errorf("eval-export: exported 0 tenders — the database has no ingested tenders")
	}

	// Guard the export itself, not just the default: even with a generous
	// -limit, a caller who lowers it back down (or a table whose country
	// count grows past today's 28) can reproduce the exact silent gap this
	// command already produced once. Comparing against every distinct
	// country actually in the table catches that before the temp file ever
	// touches the committed fixture.
	if err := checkCountryCoverage(ctx, db, byCountry); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, *out); err != nil {
		return fmt.Errorf("eval-export: rename %s to %s: %w", tmpPath, *out, err)
	}

	fmt.Printf("wrote %d tenders to %s\n", n, *out)
	for _, country := range sortedCountries(byCountry) {
		fmt.Printf("  %-4s %d\n", country, byCountry[country])
	}
	return nil
}

// checkCountryCoverage fails loudly, naming names, if any country present in
// the source table (under the same non-empty-title filter the export uses)
// has zero rows in the export. A country silently missing from the snapshot
// means an empty cross-language cell in the report — the harness's whole
// reason for existing — with no error anywhere to point at it.
func checkCountryCoverage(ctx context.Context, db *sql.DB, exported map[string]int) error {
	rows, err := db.QueryContext(ctx, selectDistinctCountriesSQL)
	if err != nil {
		return fmt.Errorf("eval-export: list distinct countries: %w", err)
	}
	defer rows.Close()

	var missing []string
	for rows.Next() {
		var country string
		if err := rows.Scan(&country); err != nil {
			return fmt.Errorf("eval-export: scan country: %w", err)
		}
		if exported[country] == 0 {
			missing = append(missing, country)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("eval-export: read distinct countries: %w", err)
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("eval-export: %d source countries are missing from the export entirely: %v — raise -limit (must be at least (distinct countries) * -per-country) and re-run",
			len(missing), missing)
	}
	return nil
}

func sortedCountries(counts map[string]int) []string {
	out := make([]string, 0, len(counts))
	for c := range counts {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}
