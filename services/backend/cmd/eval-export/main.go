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
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender/eval"
)

const selectCorpusSQL = `
SELECT source, source_ref, title, left(description, $1) AS description, buyer_name,
       status, procedure_type, language, country, nuts, cpv,
       array_to_string(cpv_secondary, ','), value, currency, published_at, deadline
FROM (
	SELECT t.*, row_number() OVER (PARTITION BY t.country ORDER BY t.published_at DESC NULLS LAST, t.id DESC) AS rn
	FROM tenders.ingested_tenders t
	WHERE t.title <> ''
) ranked
WHERE rn <= $2
ORDER BY country, rn
LIMIT $3
`

func main() {
	var (
		out        = flag.String("out", "internal/core/tender/eval/testdata/corpus.jsonl.gz", "output path")
		limit      = flag.Int("limit", 2000, "maximum tenders to export")
		perCountry = flag.Int("per-country", 120, "maximum tenders per source country")
		descMax    = flag.Int("description-max", 4000, "characters of description to keep")
	)
	flag.Parse()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	rows, err := db.QueryContext(ctx, selectCorpusSQL, *descMax, *perCountry, *limit)
	if err != nil {
		log.Fatalf("query corpus: %v", err)
	}
	defer rows.Close()

	f, err := os.Create(*out)
	if err != nil {
		log.Fatalf("create %s: %v", *out, err)
	}
	defer f.Close()

	zw := gzip.NewWriter(f)
	enc := json.NewEncoder(zw)

	byCountry := map[string]int{}
	n := 0
	for rows.Next() {
		var (
			t         eval.CorpusTender
			secondary string
		)
		if err := rows.Scan(&t.Source, &t.SourceRef, &t.Title, &t.Description, &t.BuyerName,
			&t.Status, &t.ProcedureType, &t.Language, &t.Country, &t.NUTS, &t.CPV,
			&secondary, &t.Value, &t.Currency, &t.PublishedAt, &t.Deadline); err != nil {
			log.Fatalf("scan row %d: %v", n+1, err)
		}
		t.CPVSecondary = splitCommaList(secondary)
		if err := enc.Encode(t); err != nil {
			log.Fatalf("encode row %d: %v", n+1, err)
		}
		byCountry[t.Country]++
		n++
	}
	if err := rows.Err(); err != nil {
		log.Fatalf("read corpus: %v", err)
	}
	if err := zw.Close(); err != nil {
		log.Fatalf("finish gzip: %v", err)
	}

	fmt.Printf("wrote %d tenders to %s\n", n, *out)
	for _, country := range sortedCountries(byCountry) {
		fmt.Printf("  %-4s %d\n", country, byCountry[country])
	}
	if n == 0 {
		log.Fatal("exported 0 tenders — the database has no ingested tenders")
	}
}

// splitCommaList mirrors the ingestion repo's array_to_string round trip. CPV
// codes are digits, so a comma inside a value is impossible here.
func splitCommaList(joined string) []string {
	if joined == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i <= len(joined); i++ {
		if i == len(joined) || joined[i] == ',' {
			if i > start {
				out = append(out, joined[start:i])
			}
			start = i + 1
		}
	}
	return out
}

func sortedCountries(counts map[string]int) []string {
	out := make([]string, 0, len(counts))
	for c := range counts {
		out = append(out, c)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
