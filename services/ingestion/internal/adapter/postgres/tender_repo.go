package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/core/enrich"
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
	Description   string
	BuyerName     string
	CPV           string
	ProcedureType string
	Country       string
	Status        string
	Source        string
	SourceRef     string
	Documents     []UnindexedDocument

	// Facets below are not embedded as text — they become the vector store's
	// filterable point payload. Nil means the column is NULL, which a range
	// filter must treat as "excluded", not as zero.
	CPVSecondary []string
	NUTS         string
	Value        *int64
	PublishedAt  *time.Time
	Deadline     *time.Time
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
	source, source_ref, title, description, buyer_name, buyer_id, status, procedure_type,
	language, country, nuts, cpv, cpv_secondary, value, currency,
	published_at, deadline, raw, publication_number, xml_url,
	version, history, first_seen_at, last_seen_at,
	cpv_labels
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::text[], $14, $15, $16, $17, $18::jsonb,
	$19, $20,
	1, '[]'::jsonb, now(), now(),
	-- Derived from the vocabulary rather than passed in: it consumes no
	-- placeholder, so nothing after it renumbers, and it cannot drift out of step
	-- with cpv/cpv_secondary the way a Go-side lookup could. ORDER BY (code,
	-- lang) inside string_agg matters here for the same reason it matters in
	-- RecomputeLabels: without it, the label order is unspecified and two
	-- upserts of the identical cpv/cpv_secondary pair could legally produce a
	-- different cpv_labels string, making an otherwise-no-op re-save look like
	-- a change.
	coalesce((SELECT string_agg(ct.label, ' ' ORDER BY ct.code, ct.lang) FROM tenders.cpv_terms ct
	          WHERE ct.code = $12 OR ct.code = ANY($13::text[])), '')
)
ON CONFLICT (source, source_ref) DO UPDATE SET
	title = EXCLUDED.title,
	description = EXCLUDED.description,
	buyer_name = EXCLUDED.buyer_name,
	buyer_id = EXCLUDED.buyer_id,
	procedure_type = EXCLUDED.procedure_type,
	language = EXCLUDED.language,
	country = EXCLUDED.country,
	nuts = EXCLUDED.nuts,
	cpv = EXCLUDED.cpv,
	cpv_secondary = EXCLUDED.cpv_secondary,
	cpv_labels = coalesce((SELECT string_agg(ct.label, ' ' ORDER BY ct.code, ct.lang) FROM tenders.cpv_terms ct
	                       WHERE ct.code = EXCLUDED.cpv OR ct.code = ANY(EXCLUDED.cpv_secondary)), ''),
	value = EXCLUDED.value,
	currency = EXCLUDED.currency,
	published_at = EXCLUDED.published_at,
	deadline = EXCLUDED.deadline,
	raw = EXCLUDED.raw,
	-- The notice-document enrichment queue is xml_fetched_at IS NULL, and this
	-- CASE is the only thing that ever clears it again. TED republishes a
	-- corrected notice under a NEW publication number rather than editing the
	-- old one, so an unchanged number is proof the document we already read
	-- cannot have changed — which is what makes "an unchanged notice is never
	-- re-fetched" true, without an expiry heuristic and without a conditional
	-- request per notice.
	--
	-- It still re-fetches exactly when it should. source_ref is deliberately the
	-- procedure identifier, so a contract notice and its later award notice land
	-- on THIS SAME ROW (see eforms.Map) — and those two carry different
	-- publication numbers, so the CN→CAN transition clears xml_fetched_at and
	-- the award notice's detail is read over the call's. Steady-state re-ingest
	-- of an unchanged notice moves neither column.
	--
	-- xml_attempts resets on the same condition: a row parked after exhausting
	-- its attempt budget against a struggling upstream must get a fresh budget
	-- when a NEW document appears, rather than inheriting a count earned by a
	-- document that no longer exists. xml_status is deliberately left alone —
	-- it describes the last look that was actually taken, and it is overwritten
	-- by the next one; clearing it here would erase why the previous notice
	-- failed while its replacement is still unread.
	--
	-- Every right-hand side in a SET list reads the PRE-UPDATE row, so assigning
	-- publication_number below does not disturb the two comparisons above it,
	-- despite how the order reads.
	--
	-- Sources that publish no notice document write '' into both columns. Their
	-- first upsert after these columns existed compares NULL against '', finds
	-- them distinct, and clears an xml_fetched_at that was never set — harmless,
	-- and those rows never reach the queue regardless, which filters on
	-- source = 'ted'.
	xml_fetched_at = CASE WHEN tenders.ingested_tenders.publication_number
	                        IS DISTINCT FROM EXCLUDED.publication_number
	                      THEN NULL
	                      ELSE tenders.ingested_tenders.xml_fetched_at END,
	xml_attempts = CASE WHEN tenders.ingested_tenders.publication_number
	                      IS DISTINCT FROM EXCLUDED.publication_number
	                    THEN 0
	                    ELSE tenders.ingested_tenders.xml_attempts END,
	publication_number = EXCLUDED.publication_number,
	xml_url = EXCLUDED.xml_url,
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
	                    OR tenders.ingested_tenders.description IS DISTINCT FROM EXCLUDED.description
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

// The search-payload lot upsert. Its conflict branch never overwrites a value
// it does not itself have, and that single rule is what makes per-lot detail
// storable at all.
//
// Two writers share this table: this one, from the hourly search pass, and the
// enricher's upsertLotEnrichmentSQL, from the notice document. Only the second
// one ever learns a lot's value, CPV, currency or deadline — buildLots leaves
// all four at their zero value on purpose, because the search payload's
// classification-cpv array does not align 1:1 with its identifier-lot array and
// an index match would assign the wrong CPV to the wrong lot. An unconditional
// `value = EXCLUDED.value` therefore did not mean "refresh the value", it meant
// "erase whatever the notice said, within the hour, every hour", leaving the
// column flapping between the XML's figure and NULL depending on which writer
// ran last. Per-lot value is the headline finding of reading the notice
// document (13 lots, 13 distinct values on one verified €402M notice); it was
// unstorable until this coalesce.
//
// This changes no other source's observable behaviour: eforms.buildLots is the
// only producer of lot rows in the service, and es/fr/pl publish no lots at all,
// so nothing has ever written a non-NULL value here to be preserved. cpv is
// coalesced through nullif because the column is text NOT NULL with an
// empty-string default rather than nullable, so "" is its absent value.
//
// title stays unconditional: it is the one field the search payload does carry
// for every lot, so it is authoritative there.
const upsertLotSQL = `
INSERT INTO tenders.ingested_tender_lots (tender_id, ref, title, cpv, value, currency, deadline)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (tender_id, ref) DO UPDATE SET
	title = EXCLUDED.title,
	cpv = coalesce(nullif(EXCLUDED.cpv, ''), tenders.ingested_tender_lots.cpv),
	value = coalesce(EXCLUDED.value, tenders.ingested_tender_lots.value),
	currency = coalesce(nullif(EXCLUDED.currency, ''), tenders.ingested_tender_lots.currency),
	deadline = coalesce(EXCLUDED.deadline, tenders.ingested_tender_lots.deadline)
`

const insertRunSQL = `
INSERT INTO tenders.ingestion_runs (source, started_at, finished_at, fetched, inserted, updated, error)
VALUES ($1, $2, $3, $4, $5, $6, $7)
`

// The page/section columns land alongside the text rather than in a side
// table because they are properties of the chunk itself: a part is only ever
// read together with them, and a citation that cannot name its page is not a
// citation. They are written on every upsert, including the conflict branch —
// a re-extraction that produced different text almost certainly produced a
// different page span too, so refreshing content while leaving stale
// provenance behind would be worse than not storing it at all.
const upsertDocumentPartSQL = `
INSERT INTO tenders.ingested_tender_document_parts (
	document_id, index, content, page_start, page_end, section_path, section_title, has_table
)
VALUES ($1, $2, $3, $4, $5, $6::text[], $7, $8)
ON CONFLICT (document_id, index) DO UPDATE SET
	content = EXCLUDED.content,
	page_start = EXCLUDED.page_start,
	page_end = EXCLUDED.page_end,
	section_path = EXCLUDED.section_path,
	section_title = EXCLUDED.section_title,
	has_table = EXCLUDED.has_table
`

// Every page/section column is coalesced to its Go zero value here because
// rows extracted before migration 0010 hold NULL in all of them and are NOT
// backfilled — recovering the metadata for an already-extracted document means
// re-fetching and re-extracting its PDF, a corpus-wide network cost that is a
// separate, explicitly-costed decision. Going forward only. A caller reading 0
// pages or an empty section path is therefore reading "extracted before this
// existed", not "the chunk has no page"; tender.DocumentPart documents that
// same distinction on the type.
//
// section_path is read as JSON, not via array_to_string like cpv_secondary:
// a CPV code cannot contain a comma but a document heading routinely can
// ("Capo 1, Disposizioni generali"), and comma-joining would silently split
// one heading into two path elements.
const selectDocumentPartsSQL = `
SELECT content,
       coalesce(page_start, 0),
       coalesce(page_end, 0),
       coalesce(to_jsonb(section_path)::text, ''),
       coalesce(section_title, ''),
       coalesce(has_table, false)
FROM tenders.ingested_tender_document_parts
WHERE document_id = $1 ORDER BY index
`

// The trailing columns feed the vector store's payload rather than the
// embedded text, so a filtered search can constrain the vector search itself
// instead of over-fetching and discarding (see knowledge.Attributes).
// cpv_secondary crosses the database/sql boundary as a comma-joined string
// for the same reason it's written as a text literal in upsertTenderSQL —
// database/sql has no native text[] support without a driver-specific wrapper.
const selectUnindexedTendersSQL = `
SELECT id, title, description, buyer_name, cpv, procedure_type, country, status, source, source_ref,
       array_to_string(cpv_secondary, ','), nuts, value, published_at, deadline
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

// The notice-enrichment queue. Three predicates, each load-bearing:
//
//   - source = 'ted' because it is the only source that publishes a
//     machine-readable notice document; the other three (pl-bzp, fr-boamp,
//     es-placsp) share these tables and would otherwise be listed forever with
//     nothing to fetch.
//   - xml_fetched_at IS NULL is the queue itself, and is what makes backfill and
//     steady state the same mechanism: the pre-existing corpus is simply the
//     queue's state at t=0. It is served by the partial index
//     idx_ingested_tenders_xml_pending, which shrinks as the corpus drains.
//   - the empty-string test on xml_url excludes rows with nothing to fetch. Note
//     that it ALSO excludes NULL, since comparing NULL to anything yields NULL
//     rather than true — deliberate, and the reason no explicit IS NOT NULL is
//     written here.
//
// That last predicate is a real decision, not a tidy-up. A row with no document
// URL can never advance, so listing it would burn a batch slot every pass to
// re-derive the same answer; filtering it out costs nothing and keeps the batch
// full of rows that can actually make progress. The consequence is that such
// rows stay at xml_fetched_at IS NULL forever, so the "nothing is stuck" check
// has to repeat the same empty-string test on xml_url, or it will count rows
// that are not stuck, merely unfetchable. The orchestrator's own empty-URL guard
// stays as defence in depth for any Repo implementation that does list them.
//
// ORDER BY id is stable and starvation-free: ids only ascend, and a row leaves
// the queue permanently once it is enriched or parked, so the head of the queue
// always moves.
//
// country and notice-type are selected for the observation the pass emits, not
// for any decision it makes. The notice type comes out of `raw` because it has
// never been promoted to a column: Map derives the tender's status from it and
// keeps the code itself only in the stored payload, and a whole migration to
// denormalise a value read 200 rows at a time is not worth it. Both are
// low-cardinality categoricals; nothing identifying is read here.
const selectPendingXMLSQL = `
SELECT id, coalesce(publication_number, ''), xml_url, xml_attempts,
       country, coalesce(raw->>'notice-type', '')
FROM tenders.ingested_tenders
WHERE source = 'ted'
  AND xml_fetched_at IS NULL
  AND xml_url <> ''
ORDER BY id
LIMIT $1
`

// A criteria set is a SET, not an accumulating log, so it is REPLACED rather
// than upserted on (tender_id, lot_ref, ordinal). Upserting would leave stale
// rows behind whenever a corrected notice publishes FEWER criteria than the one
// it supersedes: the stored count could only ever grow, and a criterion the
// buyer withdrew would outlive the notice that withdrew it — silently, since
// nothing about a leftover row looks wrong on its own. The same argument, and
// the same delete-then-insert, applies to the organizations.
//
// (ingested_tender_documents and ingested_tender_lots have exactly this bug
// today, upserting with no delete. It is pre-existing and deliberately not
// fixed here: those tables are written by four sources, so changing their
// replace semantics changes three other providers' behaviour in a change that
// is about a fifth one.)
const deleteAwardCriteriaSQL = `
DELETE FROM tenders.ingested_tender_award_criteria WHERE tender_id = $1
`

const insertAwardCriterionSQL = `
INSERT INTO tenders.ingested_tender_award_criteria (
	tender_id, lot_ref, ordinal, type, name, description, weight, weight_raw, lang
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
`

const deleteOrganizationsSQL = `
DELETE FROM tenders.ingested_tender_organizations WHERE tender_id = $1
`

const insertOrganizationSQL = `
INSERT INTO tenders.ingested_tender_organizations (
	tender_id, org_ref, name, company_id, roles, email, phone, website, city, country
) VALUES ($1, $2, $3, $4, $5::text[], $6, $7, $8, $9, $10)
`

// Lot enrichment: everything the notice document says about one lot that the
// search payload cannot say.
//
// It writes value/currency/cpv/deadline as well as the three columns migration
// 0010 added, which is only sound because upsertLotSQL's conflict branch was
// made coalescing — see the long note there for why an enricher-written value
// used to be erased within the hour, and why per-lot value is the whole point
// of reading the XML at all. On a real 13-lot notice the values are 13 distinct
// figures spanning €22M to €50M while every other lot-scoped field is identical
// across the lots, so this column carries essentially all of the per-lot
// information there is.
//
// The conflict branch is coalescing in the same direction and for the same
// reason, mirrored: a notice that omits a lot's value must not erase one the
// search payload happened to supply. Neither writer can clear a field, which is
// the deliberate trade — a value withdrawn by a corrected notice outlives the
// withdrawal, and that is the lesser of the two failure modes because the
// alternative is a column that is NULL most of the hour.
//
// title is carried on the INSERT branch only, because that branch fires only
// for a lot the search payload never listed (nothing else will ever populate
// that row) while on conflict the search payload's title is authoritative.
const upsertLotEnrichmentSQL = `
INSERT INTO tenders.ingested_tender_lots (
	tender_id, ref, title, cpv, cpv_secondary, value, currency, deadline,
	documents_url, submission_url
) VALUES ($1, $2, $3, $4, $5::text[], $6, $7, $8, $9, $10)
ON CONFLICT (tender_id, ref) DO UPDATE SET
	cpv = coalesce(nullif(EXCLUDED.cpv, ''), tenders.ingested_tender_lots.cpv),
	cpv_secondary = EXCLUDED.cpv_secondary,
	value = coalesce(EXCLUDED.value, tenders.ingested_tender_lots.value),
	currency = coalesce(nullif(EXCLUDED.currency, ''), tenders.ingested_tender_lots.currency),
	deadline = coalesce(EXCLUDED.deadline, tenders.ingested_tender_lots.deadline),
	documents_url = EXCLUDED.documents_url,
	submission_url = EXCLUDED.submission_url
`

// The statement that takes an enriched row out of the queue. It runs in the same
// transaction as the child rows above, which is what makes grid_usable safe to
// store at all: the flag summarises the criteria rows, and no reader may ever
// observe one without the other. Recomputing the flag from the child rows and
// asserting zero mismatches is a standing acceptance check, and it only holds
// because these writes commit together.
//
// indexed_at = NULL is not bookkeeping — it is the handoff to the indexer.
// Enrichment changes the text a tender is searched by, but the indexer's
// dirty-check (see upsertTenderSQL) compares only the fields the search payload
// carries and cannot see that the detail changed underneath it. Without this
// reset an enriched tender keeps the embedding it was indexed with before the
// notice was ever read, forever. The indexer's backlog spikes once as the
// enricher drains the corpus; that is expected and absorbed by its own
// batch-and-commit loop.
//
// The notice-level deadline and language are deliberately NOT written, even
// though the parse produces both. Unlike per-lot value — which only the notice
// document knows, and which the coalescing lot upsert now protects — the tender
// row's deadline and language are fields the search payload already carries for
// every TED notice on every cycle. Writing a second real value into a column
// another writer also writes a real value into is the flap this file spends the
// upsertLotSQL note avoiding, in the one shape coalesce cannot fix: with both
// sides populated there is no NULL to fall back to, so the column would simply
// alternate between two sources of truth. The parsed values are not wasted —
// the deadline is what lets a notice with no notice-level period inherit one
// from its lots, and the language drives every text selection in the parse —
// they are just parse-time inputs rather than persisted output.
//
// version and history are deliberately NOT touched. They track status
// transitions only: history is an unbounded jsonb on a row read on every fetch,
// and "the criteria changed between a call for tenders and its award notice" is
// the expected consequence of collapsing both onto one row, not an event worth
// recording. xml_fetched_at, xml_status and publication_number already say when
// we looked, what we got and which notice it came from.
const markXMLEnrichedSQL = `
UPDATE tenders.ingested_tenders
SET documents_url = $2,
    submission_url = $3,
    grid_usable = $4,
    xml_status = $5,
    xml_fetched_at = now(),
    indexed_at = NULL
WHERE id = $1
`

// A failed attempt. The counter always advances; xml_fetched_at advances only
// when the failure is permanent, and that asymmetry is the whole retry policy.
//
// Leaving xml_fetched_at NULL is what makes the queue self-healing — the row is
// simply listed again next pass, with no separate retry table and no scheduler —
// and incrementing xml_attempts on every attempt is what stops "again next pass"
// from meaning "forever". Without the counter, a throttling upstream would have
// the drain spin against the same batch for a whole run.
const markXMLFailedSQL = `
UPDATE tenders.ingested_tenders
SET xml_attempts = xml_attempts + 1,
    xml_status = $2,
    xml_fetched_at = CASE WHEN $3::boolean THEN now() ELSE xml_fetched_at END
WHERE id = $1
`

// pgTextArray renders a Go string slice as a PostgreSQL array literal, e.g.
// []string{"a", "b"} -> `{"a","b"}`. CPVSecondary crosses the database/sql
// boundary as a single text parameter, cast to text[] in SQL (see
// upsertTenderSQL's $13::text[]) rather than relying on driver-specific
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

// splitTextArray is pgTextArray's read-side counterpart: it turns the
// comma-joined text[] selectUnindexedTendersSQL produces via array_to_string
// back into a slice, dropping empties so an empty array yields nil rather than
// a one-element slice holding "".
func splitTextArray(joined string) []string {
	if joined == "" {
		return nil
	}
	parts := strings.Split(joined, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// jsonTextArray decodes the JSON rendering of a text[] column (produced by
// to_jsonb, see selectDocumentPartsSQL) back into a slice, mapping both a NULL
// column and an empty array onto nil so callers get one representation of
// "nothing here". Unlike splitTextArray it is lossless for values containing
// the delimiter, which is why section_path uses it.
func jsonTextArray(encoded string) ([]string, error) {
	if encoded == "" || encoded == "null" {
		return nil, nil
	}
	var out []string
	if err := json.Unmarshal([]byte(encoded), &out); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
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
			t.Source, t.SourceRef, t.Title, t.Description, t.Buyer.Name, t.Buyer.ID, string(t.Status),
			t.ProcedureType, t.Language, t.Country, t.NUTS, t.CPV,
			pgTextArray(t.CPVSecondary), t.Value, t.Currency,
			t.PublishedAt, t.Deadline, []byte(t.Raw),
			t.PublicationNumber, t.XMLURL,
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

// SaveDocumentParts upserts the extracted parts of one document — their text
// plus the page/section provenance the extractor reported — keyed by
// (document_id, index). Idempotent, so re-extracting the same document updates
// each row in place rather than duplicating it. All parts are saved in a
// single transaction so a mid-loop failure can't commit a partial set of
// parts: a partial commit would look indistinguishable from a complete one to
// indexOne's len(parts) == 0 reuse-gate check on the next cycle, causing it to
// skip re-fetching and index the tender with incomplete text. Widening the row
// does not weaken that — the extra columns ride along inside the same
// transaction, and a failure on any one of them still rolls the whole document
// back to "nothing saved", which is the only state the reuse gate reads
// correctly.
func (r *TenderRepo) SaveDocumentParts(ctx context.Context, documentID int64, parts []tender.DocumentPart) error {
	err := r.db.InTx(ctx, func(tx *pg.DB) error {
		for i, p := range parts {
			if _, err := tx.Exec(ctx, upsertDocumentPartSQL,
				documentID, i, p.Text,
				p.PageStart, p.PageEnd, pgTextArray(p.SectionPath), p.SectionTitle, p.HasTable,
			); err != nil {
				return fmt.Errorf("postgres: save document part %d for document %d: %w", i, documentID, err)
			}
		}
		return nil
	})
	return err
}

// DocumentParts returns the previously-extracted parts of one document, in
// order — an empty slice if none have been saved yet, which is what tells the
// indexer to fetch and extract the document rather than reuse it.
//
// Parts written before migration 0010 carry no page/section metadata and are
// not backfilled (see selectDocumentPartsSQL); they come back with zero pages,
// a nil section path and HasTable false.
func (r *TenderRepo) DocumentParts(ctx context.Context, documentID int64) ([]tender.DocumentPart, error) {
	rows, err := r.db.Query(ctx, selectDocumentPartsSQL, documentID)
	if err != nil {
		return nil, fmt.Errorf("postgres: document parts for document %d: %w", documentID, err)
	}
	defer rows.Close()

	var parts []tender.DocumentPart
	for rows.Next() {
		var (
			p           tender.DocumentPart
			sectionPath string
		)
		if err := rows.Scan(&p.Text, &p.PageStart, &p.PageEnd, &sectionPath, &p.SectionTitle, &p.HasTable); err != nil {
			return nil, fmt.Errorf("postgres: scan document part for document %d: %w", documentID, err)
		}
		if p.SectionPath, err = jsonTextArray(sectionPath); err != nil {
			return nil, fmt.Errorf("postgres: decode section path for document %d: %w", documentID, err)
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
		var (
			t            UnindexedTender
			cpvSecondary string
		)
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.BuyerName, &t.CPV, &t.ProcedureType,
			&t.Country, &t.Status, &t.Source, &t.SourceRef,
			&cpvSecondary, &t.NUTS, &t.Value, &t.PublishedAt, &t.Deadline); err != nil {
			rows.Close()
			return nil, fmt.Errorf("postgres: scan unindexed tender: %w", err)
		}
		t.CPVSecondary = splitTextArray(cpvSecondary)
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

// ListPendingXML returns up to limit tenders whose notice document has not been
// read yet and that have a document to read — see selectPendingXMLSQL for what
// each part of that queue definition is doing and why rows with no URL are left
// out rather than listed and parked.
func (r *TenderRepo) ListPendingXML(ctx context.Context, limit int) ([]enrich.PendingNotice, error) {
	rows, err := r.db.Query(ctx, selectPendingXMLSQL, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: list pending xml: %w", err)
	}
	defer rows.Close()

	var pending []enrich.PendingNotice
	for rows.Next() {
		var n enrich.PendingNotice
		if err := rows.Scan(&n.ID, &n.PublicationNumber, &n.XMLURL, &n.Attempts,
			&n.Country, &n.NoticeType); err != nil {
			return nil, fmt.Errorf("postgres: scan pending xml row: %w", err)
		}
		pending = append(pending, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list pending xml: %w", err)
	}
	return pending, nil
}

// SaveDetail persists one notice's parsed detail and takes its row out of the
// enrichment queue, in a single transaction.
//
// Everything here commits together on purpose. grid_usable is a stored
// denormalisation of the criteria rows written a few statements earlier, so a
// partial commit would leave the flag disagreeing with the rows it summarises —
// and since the flag is the predicate a later phase's backlog query is built on,
// a disagreement is not a cosmetic drift but a wrong answer to "which notices
// still need work". The indexed_at reset rides in the same transaction for the
// mirror-image reason: a committed detail change with no re-index queued is an
// enriched tender that is searched by its pre-enrichment text forever.
//
// The criteria and organizations are replaced wholesale rather than upserted;
// deleteAwardCriteriaSQL explains why that is a correctness requirement and not
// a simplification.
func (r *TenderRepo) SaveDetail(ctx context.Context, tenderID int64, d tender.NoticeDetail) error {
	err := r.db.InTx(ctx, func(tx *pg.DB) error {
		if _, err := tx.Exec(ctx, deleteAwardCriteriaSQL, tenderID); err != nil {
			return fmt.Errorf("postgres: delete award criteria for tender %d: %w", tenderID, err)
		}
		// AllCriteria is the single definition of "every criterion in this
		// notice" — the notice-level entries plus each lot's, which partition
		// the set and never overlap. Deriving both these rows and the
		// grid_usable flag below from that one method is what keeps the stored
		// aggregate and the child rows in agreement by construction rather than
		// by two call sites happening to apply the same rule.
		//
		// Weight is passed as the *float64 it is: database/sql maps a nil
		// pointer onto NULL, which is the point. A weightless criterion is the
		// common case, and flattening it to 0 here would make "criteria
		// published" read as "grid recoverable" — an overstatement of roughly 2x,
		// reintroduced one layer below where anyone would look for it.
		for _, c := range d.AllCriteria() {
			if _, err := tx.Exec(ctx, insertAwardCriterionSQL,
				tenderID, c.LotRef, c.Ordinal, c.Type, c.Name, c.Description,
				c.Weight, c.WeightRaw, c.Lang,
			); err != nil {
				return fmt.Errorf("postgres: insert award criterion %s/%d for tender %d: %w",
					c.LotRef, c.Ordinal, tenderID, err)
			}
		}

		if _, err := tx.Exec(ctx, deleteOrganizationsSQL, tenderID); err != nil {
			return fmt.Errorf("postgres: delete organizations for tender %d: %w", tenderID, err)
		}
		for _, o := range d.Organizations {
			if _, err := tx.Exec(ctx, insertOrganizationSQL,
				tenderID, o.Ref, o.Name, o.CompanyID, pgTextArray(o.Roles),
				o.Email, o.Phone, o.Website, o.City, o.Country,
			); err != nil {
				return fmt.Errorf("postgres: insert organization %s for tender %d: %w", o.Ref, tenderID, err)
			}
		}

		for _, l := range d.Lots {
			if _, err := tx.Exec(ctx, upsertLotEnrichmentSQL,
				tenderID, l.Ref, l.Title, l.CPV, pgTextArray(l.CPVSecondary),
				l.Value, l.Currency, l.Deadline, l.DocumentsURL, l.SubmissionURL,
			); err != nil {
				return fmt.Errorf("postgres: enrich lot %s for tender %d: %w", l.Ref, tenderID, err)
			}
		}

		if _, err := tx.Exec(ctx, markXMLEnrichedSQL,
			tenderID, d.DocumentsURL, d.SubmissionURL, d.GridUsable(), enrich.StatusOK,
		); err != nil {
			return fmt.Errorf("postgres: mark tender %d xml-enriched: %w", tenderID, err)
		}
		return nil
	})
	return err
}

// MarkXMLFailed records one failed enrichment attempt against a tender. A
// permanent failure additionally stamps xml_fetched_at, which is what removes
// the row from the queue for good; a transient one leaves it NULL so the row is
// attempted again next pass, having spent one of its attempts.
//
// status is one of the enrich package's four values. It is passed through rather
// than derived here because which failure is permanent is a policy the core
// owns — the adapter only knows how to write the two columns that encode it.
func (r *TenderRepo) MarkXMLFailed(ctx context.Context, tenderID int64, status string, permanent bool) error {
	if _, err := r.db.Exec(ctx, markXMLFailedSQL, tenderID, status, permanent); err != nil {
		return fmt.Errorf("postgres: mark tender %d xml failed (%s): %w", tenderID, status, err)
	}
	return nil
}
