-- Reverts search_vector to its pre-0008 shape: title (A), buyer_name (B), cpv
-- (C), description (D) — cpv_labels is no longer part of it.
--
-- WHY THIS IS A REVERT, NOT A RETUNE: migration 0008 put cpv_labels in weight
-- class C alongside cpv, and 0008's own toggle (LexicalQuery.ExpandCPVLabels /
-- Ranking.CPVIndexExpanded, tender_search.go / hybrid.go) had to be DISABLED
-- by production shipping `ts_filter(t.search_vector, '{a,b,d}')` on the
-- default (off) path. `ts_filter(...)` is a function call to the LEFT of the
-- `@@` operator, so it is not the indexed expression
-- (idx_ingested_tenders_search_vector indexes search_vector itself), and
-- because the predicate is an OR chain, Postgres could not use ANY index for
-- it — every lexical search, on the shipped default, degraded to a
-- sequential scan of the whole table. Measured against the live 13,036-row
-- dev table: 88 ms (BitmapOr over 3 GIN indexes, pre-0008) versus 2,987 ms
-- (Seq Scan, 42,330 buffers, Rows Removed by Filter: 13,035) on the shipped
-- default — linear in corpus size, with no statement_timeout anywhere in
-- services/backend to bound it, hitting every authenticated and anonymous
-- search that carries query text.
--
-- The label expansion itself was ALSO measured, independently, to be worth
-- nothing: task-16-report.md (local SDD record) ran the harness with
-- expansion on and off — on found slightly MORE candidates (recall@20 0.6389
-- vs 0.6315) but ranked them WORSE (ndcg 0.4764 vs 0.5004, mrr 0.4538 vs
-- 0.4983), and moved NONE of the four target cross-language cells
-- (it→de, fr→de, nl→de, pl→de) off 0.0000 in any of the four measured
-- {CPVWeight, CPVIndexExpanded} combinations. Root cause: websearch_to_tsquery
-- ANDs every query word, 'simple' does no stemming or diacritic folding, and
-- the official CPV taxonomy wording ("Building-cleaning services") is not
-- worded like a natural-language query ("pulizie uffici") — so putting the
-- labels in the vector made the text present without making it MATCHABLE by
-- the query strategy that reads it.
--
-- Given both of those — a proven production correctness/performance defect on
-- the toggle's off path, and a proven zero-benefit-with-real-cost result on
-- its on path — the honest fix is not to retune the toggle but to remove the
-- reason it exists: cpv_labels never belonged in the indexed vector. See
-- Ranking.CPVWeight's doc comment (hybrid.go) for the full history.
--
-- cpv_labels the COLUMN is deliberately kept, not dropped: it is cheap
-- (bounded text, already populated on every row via cpv_terms) and a future
-- attempt at cross-language lexical matching — e.g. per-word OR instead of
-- websearch_to_tsquery's implicit AND, or a stemming/synonym bridge — would
-- want the denormalised label text already sitting on the row rather than
-- re-deriving it. It is simply no longer part of search_vector, so it no
-- longer costs an index rebuild's worth of read amplification on every
-- lexical search.
--
-- COST: dropping and re-adding search_vector REWRITES the table and rebuilds
-- its GIN index inside one transaction (this repo's migrations run inside a
-- transaction, so CREATE INDEX CONCURRENTLY is unavailable, same constraint
-- 0008 recorded). See infrastructure/kubernetes/tendersbay-xyz/ingestion/readme.md
-- for why "run during a quiet window" (0008's phrasing) is not an achievable
-- instruction here — migrations apply unattended at the start of an hourly,
-- deadline-bounded CronJob — and for the check this migration's cost needs
-- against the current production row count before merge.

ALTER TABLE tenders.ingested_tenders DROP COLUMN IF EXISTS search_vector;

ALTER TABLE tenders.ingested_tenders
    ADD COLUMN IF NOT EXISTS search_vector tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('simple', coalesce(buyer_name, '')), 'B') ||
        setweight(to_tsvector('simple', coalesce(cpv, '')), 'C') ||
        setweight(to_tsvector('simple', coalesce(description, '')), 'D')
    ) STORED;

CREATE INDEX IF NOT EXISTS idx_ingested_tenders_search_vector
    ON tenders.ingested_tenders USING GIN (search_vector);
