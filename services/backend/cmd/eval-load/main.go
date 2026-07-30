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
// # Idempotence
//
// A re-run costs seconds, not minutes: idempotence is tracked with
// tendersbay_eval's own indexed_at column, the exact mechanism
// services/ingestion's real indexer already uses (indexed_at IS NULL means
// "needs indexing" — see its markIndexedSQL/MarkIndexed). Every upsert
// leaves indexed_at untouched, and embedAll only sets it once a tender's own
// Qdrant write has actually succeeded — so a tender is never marked done on
// a failure, and a crash partway through a run leaves only the true
// remainder to embed next time. This is deliberately NOT "does a vector
// search against the collection return any hit": that check cannot tell
// "this corpus is loaded" apart from "something else happens to already be
// in this collection", which matters a great deal given the next point.
//
// # Document identity: the eval row's own id, and why that is now required
//
// Every corpus document is keyed in Qdrant by the eval database's own row id
// (the id returned from the upsert) — exactly like a real tender
// (services/ingestion/internal/adapter/index/indexer.go's indexOne keys the
// same way), not by the corpus's own "source:source_ref" identity, even
// though the latter is arguably the more meaningful key. That is because the
// only place a Qdrant hit's DocID goes downstream is
// internal/adapter/postgres/tender_search.go's FindByIDsFiltered, which
// parses it with a bare strconv.ParseInt: a non-numeric id parses as
// nothing, is silently dropped, and once every id in a batch is dropped that
// way, FindByIDsFiltered returns (nil, nil) — no error, no degraded-mode
// signal, just an empty dense candidate set folded into a fusion that still
// reports RetrievalMode "hybrid". Keying by "source:source_ref" therefore
// does not merely lose a convenience, it makes every dense hit silently
// unresolvable while the search path insists nothing went wrong. Keying by
// id is a hard requirement of the current search path, not a style choice.
//
// This id-based keying is only safe because EVAL_QDRANT_URL must point at a
// Qdrant instance DEDICATED to this eval corpus (this repo's eval stack runs
// one on :6533, isolated from the shared dev instance on :6333, which real
// ingestion also writes to). go-services/knowledge hardcodes its collection
// name ("tenders") as a package constant, so a caller cannot select a
// different collection on a shared instance — only a separate instance
// provides real isolation. tendersbay_eval's bigserial sequence starts fresh
// at 1, and a real tender's Qdrant document is keyed by its own decimal
// Postgres id the same way — so on a SHARED instance this exact keying would
// silently overwrite a real tender's embedding the moment their ids
// coincide, which they very likely would. Do not point EVAL_QDRANT_URL at a
// shared instance to "simplify the setup": if collection isolation is ever
// removed, this keying must go back to something collision-proof
// (source:source_ref was used for exactly that reason, briefly, before the
// dedicated instance existed), and FindByIDsFiltered's id-only assumption
// would need to change first.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
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
	if err := evalDatabaseGuard(dsn); err != nil {
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
// the Postgres row id upsertCorpus resolved for it. That id does double
// duty: it is both the mark-indexed key and the Qdrant document id (see the
// package doc for why by-id keying is required, and why that is only safe
// against a dedicated eval Qdrant instance).
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
			// strconv.FormatInt of the eval row's own id, mirroring
			// services/ingestion's indexOne exactly — see the package doc
			// for why this must be the numeric id and not c.Key().
			ID:      strconv.FormatInt(p.id, 10),
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

// evalDatabaseGuard refuses to run unless dsn's database name plainly marks
// it as an evaluation target (contains "eval", case-insensitive).
//
// The corpus snapshot carries REAL production (source, source_ref) keys —
// eval-export drew it straight from tenders.ingested_tenders — and every
// upsert here is an ON CONFLICT (source, source_ref) DO UPDATE. Pointed at
// the production database by a copy-paste or env-var mistake, this command
// would not just add rows: it would silently overwrite live tenders with
// stale, 4000-character-truncated snapshot data. This is a cheap guard
// against exactly that mistake, not a full authorization system —
// "contains eval" is deliberately permissive about tendersbay_eval,
// tendersbay-eval-2, and similar, while blocking the one name that actually
// matters: "tendersbay" itself.
func evalDatabaseGuard(dsn string) error {
	name, err := dsnDatabaseName(dsn)
	if err != nil {
		return fmt.Errorf("eval-load: could not determine a database name from EVAL_DATABASE_URL (%w); refusing to run without confirming it targets an evaluation database", err)
	}
	if !strings.Contains(strings.ToLower(name), "eval") {
		return fmt.Errorf("eval-load: refusing to run against database %q (parsed from EVAL_DATABASE_URL): its name does not contain \"eval\", and this command's ON CONFLICT DO UPDATE would silently overwrite real tenders if this is a production database; point EVAL_DATABASE_URL at an evaluation-only database instead", name)
	}
	return nil
}

// dsnDatabaseName extracts the database name from a Postgres DSN, supporting
// both URL form (postgres://user:pass@host:port/dbname?opts...) and libpq
// keyword form (space-separated key=value pairs including dbname=... or
// database=...).
func dsnDatabaseName(dsn string) (string, error) {
	if u, err := url.Parse(dsn); err == nil && (u.Scheme == "postgres" || u.Scheme == "postgresql") {
		if name := strings.TrimPrefix(u.Path, "/"); name != "" {
			return name, nil
		}
	}
	for _, field := range strings.Fields(dsn) {
		for _, key := range []string{"dbname=", "database="} {
			if rest, ok := strings.CutPrefix(field, key); ok {
				return strings.Trim(rest, `'"`), nil
			}
		}
	}
	return "", fmt.Errorf("no database name found in DSN")
}
