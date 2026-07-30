// Command eval-load populates an evaluation database and Qdrant collection
// from the committed corpus snapshot, so the offline harness
// (internal/core/tender/eval) has something to search.
//
// It is a command rather than test setup because embedding ~3.000 tenders
// through Ollama takes minutes: paying that on every harness run would make
// the harness something people skip by habit rather than by intent.
//
// The target database must ALREADY have services/ingestion's migrations
// applied. This command deliberately does not migrate: services/backend does
// not own the tenders schema (services/ingestion does), and importing the
// ingestion module here would invert that ownership for the sake of a
// fixture.
//
// # Why idempotence is NOT "is the Qdrant collection non-empty"
//
// go-services/knowledge hardcodes its Qdrant collection name ("tenders") as a
// package constant — there is no way to point this command at a
// dedicated eval collection. It is the SAME collection services/ingestion's
// real pipeline writes to. A "does a vector search return any hit" probe
// would therefore report "already loaded" on the very first run, before a
// single corpus tender had been embedded, simply because the shared
// collection already holds tens of thousands of real points. Idempotence is
// tracked instead with tendersbay_eval's own indexed_at column — the exact
// mechanism services/ingestion's real indexer already uses (indexed_at IS
// NULL means "needs indexing", see services/ingestion's markIndexedSQL) —
// which is per-tender, resumes correctly after a partial failure, and needs
// no knowledge of what else lives in the shared collection.
//
// Because the collection is shared, every corpus document is also keyed in
// Qdrant by its own stable identity (eval.CorpusTender.Key(),
// "source:source_ref") rather than by the eval database's row id. A real
// tender's Qdrant document is keyed by its decimal Postgres id (see
// services/ingestion/internal/adapter/index/indexer.go's indexOne), and
// tendersbay_eval's bigserial sequence starts fresh at 1 — so an eval row's
// id would very likely collide with a REAL tender's id already indexed in
// that same shared collection, silently overwriting its embedding.
// "source:source_ref" always contains a colon, which a bare decimal id
// string never does, so the collision is structurally impossible. It is
// also the identity the golden judged set (Task 5) already keys judgements
// on, so a later harness needs no extra lookup to match a hit back to a
// judgement.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/buildwithgo/berrygem/rag"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/knowledge"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender/eval"
)

// insertCorpusTenderSQL mirrors the columns the search paths read. It is a
// deliberate subset of services/ingestion's upsertTenderSQL: the history,
// version and raw bookkeeping has no bearing on retrieval, and duplicating it
// here would couple a fixture loader to an ingestion concern.
//
// cpv_labels is NOT written: it is derived from cpv_terms by seed-cpv (see
// the Phase 1 tasks), and writing a stale value here would make the
// index-side expansion look broken.
//
// indexed_at is only ever set to NULL here, and the ON CONFLICT branch never
// touches it — that is what lets a re-run tell a freshly-upserted tender
// apart from one this command already embedded on a previous run (see the
// package doc).
const insertCorpusTenderSQL = `
INSERT INTO tenders.ingested_tenders (
	source, source_ref, title, description, buyer_name, status, procedure_type,
	language, country, nuts, cpv, cpv_secondary, value, currency,
	published_at, deadline, raw, indexed_at
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::text[], $13, $14, $15, $16, '{}'::jsonb, NULL
)
ON CONFLICT (source, source_ref) DO UPDATE SET
	title = EXCLUDED.title,
	description = EXCLUDED.description,
	buyer_name = EXCLUDED.buyer_name,
	status = EXCLUDED.status,
	procedure_type = EXCLUDED.procedure_type,
	language = EXCLUDED.language,
	country = EXCLUDED.country,
	nuts = EXCLUDED.nuts,
	cpv = EXCLUDED.cpv,
	cpv_secondary = EXCLUDED.cpv_secondary,
	value = EXCLUDED.value,
	currency = EXCLUDED.currency,
	published_at = EXCLUDED.published_at,
	deadline = EXCLUDED.deadline
RETURNING id, indexed_at
`

// markIndexedSQL records that a tender's embedding was written to Qdrant,
// mirroring services/ingestion's own markIndexedSQL/MarkIndexed.
const markIndexedSQL = `UPDATE tenders.ingested_tenders SET indexed_at = now() WHERE id = $1`

func main() {
	var (
		corpusPath = flag.String("corpus", "internal/core/tender/eval/testdata/corpus.jsonl.gz", "corpus snapshot")
		force      = flag.Bool("force-embed", false, "re-embed every tender even if already indexed")
	)
	flag.Parse()

	if err := run(*corpusPath, *force); err != nil {
		log.Fatal(err)
	}
}

// run does the actual work and returns errors instead of exiting, so every
// deferred cleanup (closing the database connection) always runs — main is
// the only place allowed to turn an error into a process exit.
func run(corpusPath string, force bool) error {
	dsn, err := mustEnv("EVAL_DATABASE_URL")
	if err != nil {
		return err
	}
	qdrantURL, err := mustEnv("EVAL_QDRANT_URL")
	if err != nil {
		return err
	}
	ollamaURL, err := mustEnv("EVAL_OLLAMA_URL")
	if err != nil {
		return err
	}
	model := os.Getenv("EVAL_EMBEDDING_MODEL")
	if model == "" {
		model = "embeddinggemma:latest"
	}

	corpus, err := eval.LoadCorpus(os.DirFS("."), corpusPath)
	if err != nil {
		return fmt.Errorf("eval-load: load corpus: %w", err)
	}
	fmt.Printf("corpus: %d tenders\n", len(corpus))

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("eval-load: open database: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	toEmbed, err := upsertCorpus(ctx, db, corpus, force)
	if err != nil {
		return err
	}
	fmt.Printf("postgres: %d tenders upserted\n", len(corpus))

	if len(toEmbed) == 0 {
		fmt.Printf("qdrant: all %d tenders already indexed; nothing to embed (pass -force-embed to rebuild)\n", len(corpus))
		return nil
	}
	fmt.Printf("qdrant: %d/%d tenders already indexed, %d to embed\n",
		len(corpus)-len(toEmbed), len(corpus), len(toEmbed))

	kb, err := knowledge.NewKnowledgeBase(ctx, qdrantURL, ollamaURL, model)
	if err != nil {
		return fmt.Errorf("eval-load: knowledge base: %w", err)
	}

	if err := embedAll(ctx, db, kb, toEmbed); err != nil {
		return err
	}
	fmt.Printf("qdrant: %d/%d embedded\n", len(toEmbed), len(toEmbed))
	return nil
}

// pendingTender is a corpus tender that still needs embedding, paired with
// the Postgres row id upsertCorpus resolved for it (needed only to mark it
// indexed afterwards — Qdrant itself is keyed by c.Key(), never by id; see
// the package doc).
type pendingTender struct {
	id int64
	c  eval.CorpusTender
}

// upsertCorpus writes every corpus tender to Postgres and returns the subset
// that still needs embedding: everything, when force is set, otherwise only
// the rows whose indexed_at came back NULL.
func upsertCorpus(ctx context.Context, db *sql.DB, corpus []eval.CorpusTender, force bool) ([]pendingTender, error) {
	var toEmbed []pendingTender
	for _, c := range corpus {
		var (
			id        int64
			indexedAt sql.NullTime
		)
		if err := db.QueryRowContext(ctx, insertCorpusTenderSQL,
			c.Source, c.SourceRef, c.Title, c.Description, c.BuyerName, statusOr(c.Status),
			c.ProcedureType, c.Language, c.Country, c.NUTS, c.CPV, pgTextArray(c.CPVSecondary),
			c.Value, c.Currency, c.PublishedAt, c.Deadline,
		).Scan(&id, &indexedAt); err != nil {
			return nil, fmt.Errorf("eval-load: upsert %s: %w", c.Key(), err)
		}
		if force || !indexedAt.Valid {
			toEmbed = append(toEmbed, pendingTender{id: id, c: c})
		}
	}
	return toEmbed, nil
}

// embedAll embeds and ingests every pending tender, marking each one indexed
// in Postgres as soon as its own Qdrant write succeeds — so a failure partway
// through leaves already-embedded tenders recorded as done and only the
// remainder to retry on the next run, rather than losing that progress.
func embedAll(ctx context.Context, db *sql.DB, kb *knowledge.KnowledgeBase, toEmbed []pendingTender) error {
	for i, p := range toEmbed {
		c := p.c
		doc := &rag.Document{
			ID:      c.Key(),
			Content: summaryText(c),
			Metadata: map[string]string{
				"source":     c.Source,
				"source_ref": c.SourceRef,
			},
		}
		if err := kb.IngestWithAttributes(ctx, doc, knowledge.Attributes{
			Title:        c.Title,
			Country:      c.Country,
			Status:       statusOr(c.Status),
			CPV:          c.CPV,
			CPVSecondary: c.CPVSecondary,
			NUTS:         c.NUTS,
			Value:        c.Value,
			PublishedAt:  c.PublishedAt,
			Deadline:     c.Deadline,
		}); err != nil {
			return fmt.Errorf("eval-load: index %s: %w", c.Key(), err)
		}
		if _, err := db.ExecContext(ctx, markIndexedSQL, p.id); err != nil {
			return fmt.Errorf("eval-load: mark %s indexed: %w", c.Key(), err)
		}
		if (i+1)%100 == 0 {
			fmt.Printf("qdrant: %d/%d embedded\n", i+1, len(toEmbed))
		}
	}
	return nil
}

// summaryText is the chunk the dense arm searches: title, buyer and
// description together, mirroring what services/ingestion's indexer builds
// for a tender's summary chunk. The corpus snapshot's Description is always
// empty (the source database has none), so in practice this reduces to
// title + buyer — expected, not a bug (see the plan brief).
func summaryText(c eval.CorpusTender) string {
	parts := []string{c.Title}
	if c.BuyerName != "" {
		parts = append(parts, c.BuyerName)
	}
	if c.Description != "" {
		parts = append(parts, c.Description)
	}
	return strings.Join(parts, "\n")
}

// statusOr defaults an unset status to 'unknown'. The column has a CHECK
// constraint listing the five allowed values, and an empty string is not
// one of them.
func statusOr(status string) string {
	if status == "" {
		return "unknown"
	}
	return status
}

// pgTextArray renders a []string as a Postgres text[] literal. database/sql
// has no portable slice binding, so the value crosses as text and the SQL
// casts it — the same round trip services/ingestion's tender_repo.go
// performs.
func pgTextArray(vals []string) string {
	if len(vals) == 0 {
		return "{}"
	}
	quoted := make([]string, len(vals))
	for i, v := range vals {
		v = strings.ReplaceAll(v, `\`, `\\`)
		v = strings.ReplaceAll(v, `"`, `\"`)
		quoted[i] = `"` + v + `"`
	}
	return "{" + strings.Join(quoted, ",") + "}"
}

func mustEnv(name string) (string, error) {
	v := os.Getenv(name)
	if v == "" {
		return "", fmt.Errorf("eval-load: %s is not set", name)
	}
	return v, nil
}
