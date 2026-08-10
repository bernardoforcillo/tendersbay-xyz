-- Persists the detail that only TED's eForms XML carries: per-lot value, CPV,
-- documents/submission URLs and deadline, the award-criteria grid, and the
-- notice's organizations. The search API this service ingests today returns
-- ~19 of the 40+ fields a notice actually publishes; everything below is
-- reachable only by fetching the notice's XML document, which has never been
-- fetched at all. See docs/superpowers/specs/2026-08-06-phase-1a-eforms-xml-ingest.md.
--
-- COST: this migration REWRITES NOTHING. Every change here is either a new
-- table, or an ADD COLUMN that is nullable or carries a non-volatile constant
-- default — both metadata-only in modern Postgres. That is the deliberate
-- contrast with 0008/0009, which dropped and re-added a STORED generated
-- column and therefore rewrote the whole table plus its GIN index inside one
-- transaction. The only non-trivial work here is the partial index and the
-- one-shot backfill at the bottom; both are seconds at the 13,036-row dev
-- scale 0009 measured. Production row count is a different number and is
-- unknown — check it before merging, per
-- infrastructure/kubernetes/tendersbay-xyz/ingestion/readme.md. If the backfill
-- is uncomfortable at that scale, split it out into a batched command
-- mirroring cmd/backfill-descriptions rather than widening this migration's
-- transaction: migrations here apply unattended at the start of a
-- deadline-bounded CronJob, so "run it during a quiet window" is not an
-- instruction anyone can follow.
--
-- EVERY NEW COLUMN IS NULLABLE (or defaulted), on purpose. ingested_tenders,
-- ingested_tender_lots and ingested_tender_document_parts are shared with
-- pl-bzp, fr-boamp and es-placsp (internal/adapter/source/registry.go); none of
-- those sources publish eForms XML and none of them will ever populate an
-- eForms column. A NOT NULL here would make three other sources unable to
-- insert a row.
--
--
-- WHY award_criteria.weight IS NULLABLE, AND WHY NULL IS THE COMMON CASE
--
-- A published award criterion usually has no weight. Measured on Italian
-- cn-standard notices, n=50:
--
--     criteria published at all           38 / 50  (76%)
--     at least one criterion with weight  18 / 50  (36%)   <- the real number
--     criterion entries carrying a weight 63 / 87
--
-- The twenty notices in the gap typically carry a single entry reading
-- "Offerta economicamente piu vantaggiosa" — that is the award METHOD (MEAT vs
-- lowest price), not a scoring grid. It tells you nothing about where the
-- points are won. Corroborated on 545620-2026: a EUR 402M, 13-lot notice whose
-- one criterion is exactly that string, with no weight.
--
-- So "has criteria" overstates usable coverage by roughly 2x, and NULL weight
-- is not an ingestion failure to be cleaned up later — it is the majority of
-- what TED publishes. `weight numeric` NULL and `weight_raw text` are both
-- stored: published weights are inconsistently formatted ("30", "30%", "30,5"),
-- and a parse failure must degrade to an auditable string rather than lose the
-- datum. Anything that coalesces weight to 0 reintroduces the 2x overstatement
-- silently, one layer below where anyone will look for it.
--
-- The same trap applies to reporting. The PostHog event this feature emits
-- (notice_xml_ingested) deliberately has NO has_criteria property at all — the
-- only reliable way to stop someone quoting 76% is to make 76% unavailable.
-- criteria_count and weighted_criteria_count are both present so the gap shows
-- up in every chart by construction. Do not add has_criteria back.
--
--
-- WHY grid_usable IS THREE-VALUED, AND WHY NULL MUST NOT COLLAPSE INTO false
--
--     NULL  = not yet enriched, or not a TED row at all (we have not looked)
--     false = enriched, and no criterion carries a weight (we looked; no grid)
--     true  = enriched, at least one weighted criterion (we looked; grid exists)
--
-- Collapsing NULL into false makes "we haven't looked" indistinguishable from
-- "we looked and there is nothing there" — the coverage-vs-cause conflation
-- this whole phase exists to remove. The larger deliverable of Phase 1a is not
-- the 36% it answers; it is that it tells you which bucket every other notice
-- is in, which converts a later phase's scope from a guess into a query
-- (SELECT ... WHERE grid_usable = false AND country = 'IT'). That query is
-- meaningless if false also means "unfetched". Keep the column nullable and
-- keep the three-way read.
--
--
-- WHY grid_usable IS PERSISTED RATHER THAN COMPUTED ON READ
--
-- It is strictly derivable from ingested_tender_award_criteria, so persisting
-- it is a denormalisation — and normally that argument would win. It does not
-- win here, because grid_usable is a FILTER PREDICATE, not a display field: it
-- is what builds the backlog query above, over the whole corpus.
--
-- Computed on read, that predicate becomes a correlated EXISTS against a child
-- table, evaluated per row — a non-indexable expression to the left of the
-- comparison, which is STRUCTURALLY THE SAME DEFECT MIGRATION 0009 REVERTED.
-- There, ts_filter(search_vector, ...) sat to the left of @@ and so was not the
-- indexed expression, and the planner fell back to a sequential scan: 88 ms
-- became 2,987 ms on a 13,036-row table, linear in corpus size, with no
-- statement_timeout anywhere to bound it. The house has already paid for this
-- lesson once at production expense; a boolean column costs one byte and is
-- indexable.
--
-- Drift is the real cost of persisting, and it is mitigated rather than
-- ignored: grid_usable is written in the SAME TRANSACTION as the criteria rows
-- it summarises, and "recompute grid_usable from the child rows and assert zero
-- mismatches" is a standing acceptance gate, answerable in plain SQL.
--
-- Deliberately NOT persisted: criteria_count and weighted_criteria_count.
-- Nothing filters on them, so they are cheap aggregates that would only add
-- drift surface. grid_usable earns its column because it is a predicate; a
-- count is not.
--
--
-- version / history MUST NOT BE EXTENDED TO COVER XML ENRICHMENT
--
-- ingested_tenders.version and .history track STATUS TRANSITIONS ONLY, and that
-- is deliberate (internal/adapter/postgres/tender_repo.go). Do not make the
-- enricher bump version or append an entry per enrichment. Three reasons:
--
--   1. history is an unbounded jsonb on a hot row that is read on every fetch.
--      One entry per enrichment puts corpus-sized growth into that column, and
--      unlike a side table it can never be pruned without rewriting the row.
--   2. A CN and its later CAN land on ONE row (source_ref is the
--      procedure-identifier, not the publication-number), and their criteria
--      differ. "The criteria changed between the call and the award" is the
--      EXPECTED outcome of that design, not a notable event — logging it turns
--      a log whose every current entry is meaningful into one that is mostly
--      noise.
--   3. The facts a reader would actually want are already columns:
--      xml_fetched_at (when we looked), xml_status (what we got), and
--      publication_number (which notice it came from). Nothing is lost.
--
-- publication_number is also the idempotency key: TED republishes a corrected
-- notice under a NEW publication number, so an unchanged notice never needs
-- re-fetching, and a changed one is detected by comparison rather than by an
-- expiry heuristic. The upsert clears xml_fetched_at (and resets xml_attempts)
-- exactly when publication_number changes.
--
-- xml_status vocabulary: 'ok' | 'absent' | 'parse_error' | 'unavailable'.
-- 'absent' is TERMINAL — a legacy pre-eForms notice has no XML and never will,
-- and treating its 404 as retryable is the shortest path to an infinite loop
-- against TED. xml_attempts exists so a transient failure (429/5xx) can be
-- retried a bounded number of times and then parked as 'unavailable', instead
-- of the drain re-listing the same permanently-failing batch for a whole run.
--
--
-- WHAT THIS MIGRATION DOES NOT BACKFILL, STATED SO NOBODY ASSUMES IT DID
--
-- The tabula page/section columns on ingested_tender_document_parts
-- (page_start, page_end, section_path, section_title, has_table) are populated
-- GOING FORWARD ONLY. Backfilling them means re-fetching and re-extracting
-- every PDF already in the corpus — precisely the cost that justified adding
-- the columns now rather than later. Existing rows keep NULL, and NULL there
-- means "extracted before these columns existed", not "the chunk has no page".
--
-- The XML itself is likewise not backfilled here, because it cannot be: unlike
-- cmd/backfill-descriptions, which re-derived a mis-parsed field from a payload
-- that was already on disk, the XML was never fetched. Backfill is network
-- work, one request per notice. It therefore has no command of its own — the
-- pre-existing corpus IS the enricher's queue at t=0 (xml_fetched_at IS NULL),
-- drained by the same code at the same paced rate with the same per-row failure
-- isolation as steady state.

ALTER TABLE tenders.ingested_tenders
    ADD COLUMN IF NOT EXISTS publication_number text,
    ADD COLUMN IF NOT EXISTS xml_url            text,
    ADD COLUMN IF NOT EXISTS xml_fetched_at     timestamptz,
    ADD COLUMN IF NOT EXISTS xml_status         text,
    ADD COLUMN IF NOT EXISTS xml_attempts       smallint NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS documents_url      text,
    ADD COLUMN IF NOT EXISTS submission_url     text,
    ADD COLUMN IF NOT EXISTS grid_usable        boolean;

ALTER TABLE tenders.ingested_tender_lots
    ADD COLUMN IF NOT EXISTS cpv_secondary  text[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS documents_url  text,
    ADD COLUMN IF NOT EXISTS submission_url text;

ALTER TABLE tenders.ingested_tender_document_parts
    ADD COLUMN IF NOT EXISTS page_start    integer,
    ADD COLUMN IF NOT EXISTS page_end      integer,
    ADD COLUMN IF NOT EXISTS section_path  text[],
    ADD COLUMN IF NOT EXISTS section_title text,
    ADD COLUMN IF NOT EXISTS has_table     boolean;

-- The enricher's queue. Partial, not a plain index on xml_fetched_at: the
-- pending set shrinks toward empty as the corpus drains, so the index that
-- serves it should shrink with it rather than carry an entry per enriched row
-- forever. Indexing (id) and nothing else keeps it a covering lookup for the
-- batch-listing query, which selects the oldest pending ids and then joins back
-- for the payload. Contrast indexed_at IS NULL, which has no supporting index
-- at all today — fine at current scale, and noted as debt rather than fixed
-- here because it belongs to the indexer, not to this change.
CREATE INDEX IF NOT EXISTS idx_ingested_tenders_xml_pending
    ON tenders.ingested_tenders (id)
    WHERE xml_fetched_at IS NULL;

-- One row per published criterion entry, notice-level or lot-level.
--
-- lot_ref is a plain text column, NOT a foreign key onto
-- ingested_tender_lots(id). It holds the notice's OWN lot identifier — the same
-- value ingested_tender_lots.ref carries — so a notice's criteria can be
-- written in a single statement from the parsed XML with no round-trip to
-- resolve surrogate keys first. The trade-off is real and is accepted
-- knowingly: there is no referential integrity between the two child tables, so
-- a criterion can name a lot that was never inserted. The acceptance eval
-- checks for that rather than the database enforcing it.
--
-- '' (not NULL) means notice-level, so that the UNIQUE constraint actually
-- constrains: in Postgres, NULLs in a unique key do not conflict with each
-- other, and a nullable lot_ref would silently permit unlimited duplicate
-- notice-level criteria at the same ordinal.
--
-- ordinal is the position as published, kept because it is the only stable sort
-- key a criteria set has — the entries are frequently untitled and often
-- identical in type, so nothing else orders them reproducibly.
--
-- A criteria set is a SET, not an accumulating log: the writer replaces the
-- whole set (DELETE by tender_id, then INSERT, in one transaction) rather than
-- upserting on (tender_id, lot_ref, ordinal). Upserting leaves stale rows
-- whenever a corrected notice publishes FEWER criteria than the one it
-- replaces, so the count could only ever grow and a withdrawn criterion would
-- survive forever. (ingested_tender_documents and ingested_tender_lots have
-- exactly this latent bug today; it is pre-existing and out of scope, because
-- fixing it changes three other sources' behaviour.)
CREATE TABLE IF NOT EXISTS tenders.ingested_tender_award_criteria (
    id          bigserial PRIMARY KEY,
    tender_id   bigint NOT NULL REFERENCES tenders.ingested_tenders(id) ON DELETE CASCADE,
    lot_ref     text NOT NULL DEFAULT '',   -- '' = notice-level
    ordinal     integer NOT NULL,
    type        text NOT NULL DEFAULT '',
    name        text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    -- NULL is the COMMON case, not a defect — see the header. weight_raw keeps
    -- the published string verbatim so "30" vs "30%" vs "30,5" stays auditable
    -- when the numeric parse gives up.
    weight      numeric,
    weight_raw  text NOT NULL DEFAULT '',
    -- The XML is MUL: every free-text element is published once per language.
    -- lang records which one this row's text actually came from, because the
    -- selection (notice language, else ENG, else first available) is a choice
    -- the parser makes, and a later reader has no way to reconstruct it.
    lang        text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tender_id, lot_ref, ordinal)
);

-- The notice's organizations: the buyer, plus the appeal / appeal-information /
-- mediation bodies a bidder needs in order to challenge an award.
--
-- roles is text[] rather than a join table because it is a closed, tiny set of
-- values with no attributes of their own — the same judgement ingested_tenders
-- .cpv_secondary already makes, with an established encode/decode pair in the
-- repository to match.
--
-- These rows can contain personal data: buyer contacts are usually
-- institutional (a PEC address such as protocollo@pec.regione.lazio.it), but
-- some notices name an individual. Persisting is defensible — this is published
-- in the EU's own register — but name, email and phone MUST NEVER reach a
-- PostHog event. That is enforced by construction rather than by review: the
-- notice_xml_ingested payload carries counts and booleans only, so there is no
-- field for a contact to leak through.
CREATE TABLE IF NOT EXISTS tenders.ingested_tender_organizations (
    id          bigserial PRIMARY KEY,
    tender_id   bigint NOT NULL REFERENCES tenders.ingested_tenders(id) ON DELETE CASCADE,
    org_ref     text NOT NULL,
    name        text NOT NULL DEFAULT '',
    company_id  text NOT NULL DEFAULT '',
    roles       text[] NOT NULL DEFAULT '{}',
    email       text NOT NULL DEFAULT '',
    phone       text NOT NULL DEFAULT '',
    website     text NOT NULL DEFAULT '',
    city        text NOT NULL DEFAULT '',
    country     text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tender_id, org_ref)
);

-- The one thing here that IS backfillable without touching the network, and the
-- thing that makes the enricher's queue resolvable at all.
--
-- Every TED row already stores its API payload in `raw`, and that payload
-- already contains both the publication number and the XML document's URL —
-- the ingestion client has always requested the `links` field; the decoder
-- simply threw away every format except `pdf`. So for the existing corpus these
-- two columns are a re-read of data we already hold, exactly like
-- cmd/backfill-descriptions' re-derivation, and unlike the XML content itself
-- (which was never fetched and therefore has no offline backfill).
--
-- Without this UPDATE the enricher would list every pre-existing TED row as
-- pending and then have no URL to fetch for any of them, so the entire existing
-- corpus would be permanently unreachable until each row happened to be
-- re-ingested. That is why it belongs inside the migration rather than in a
-- follow-up command.
--
-- `links` is shaped {format: {lang: url}} and the XML entry is published under
-- the pseudo-language MUL, hence the two-step traversal. A TED row whose raw
-- carries no such link simply assigns NULL over NULL — harmless, and correctly
-- leaves that row unresolvable rather than inventing a URL for it.
--
-- The WHERE clause is doing real work in both halves: `source = 'ted'` keeps
-- the statement off the pl-bzp / fr-boamp / es-placsp rows, whose raw payloads
-- have entirely different shapes and no publication number at all, and
-- `raw IS NOT NULL` keeps it off rows ingested before raw was retained.
UPDATE tenders.ingested_tenders
   SET publication_number = raw->>'publication-number',
       xml_url            = raw->'links'->'xml'->>'MUL'
 WHERE source = 'ted'
   AND raw IS NOT NULL;
