-- Index-side half of the CPV language bridge: each tender carries the labels of
-- its CPV codes, in all 24 EU languages, as searchable text.
--
-- Why denormalise at all: search_vector is a generated column, and a generated
-- column may only reference its OWN row — it cannot join tenders.cpv_terms. So
-- the labels have to live on the tender.
--
-- Why this is worth doing when the CPV retrieval arm already exists: they reach
-- the same tenders by different routes, and the two fail differently. The arm
-- needs the query to resolve to a code; the index needs only a word from any
-- label to appear in the query. Running both counts the CPV signal twice, which
-- is a real tuning hazard — see Ranking.CPVIndexExpanded — so both are
-- independently switchable and the evaluation harness measures the combinations.
--
-- Weight class C, together with the code itself. setweight has exactly four
-- classes; keeping the labels with cpv rather than mixing them into D with the
-- description leaves them separately weightable, and ts_rank_cd's default weight
-- for C is 0.2 — a category label must never outrank a title match.
--
-- cpv_labels is populated by SQL, not by Go:
--   * the upsert (tender_repo.go) recomputes it from cpv_terms on every write,
--     as a scalar subquery — so it costs no placeholder and cannot drift;
--   * `go run ./cmd/seed-cpv` recomputes it for the whole table, which is how a
--     vocabulary revision reaches existing rows.
--
-- COST: dropping and re-adding search_vector REWRITES the table and rebuilds its
-- GIN index, inside one transaction (drops runs every migration that way, which
-- is also why CREATE INDEX CONCURRENTLY is unavailable here). On a large
-- ingested_tenders this holds an ACCESS EXCLUSIVE lock for the duration. Run it
-- during a quiet window.
--
-- NOTE: this does NOT reset indexed_at. cpv_labels feeds only the Postgres
-- lexical index, never the Qdrant payload (knowledge.Attributes carries the
-- codes, not the labels), so no re-embedding is needed — unlike 0006.

ALTER TABLE tenders.ingested_tenders
    ADD COLUMN IF NOT EXISTS cpv_labels text NOT NULL DEFAULT '';

-- Backfill from the vocabulary. Empty when cpv_terms has not been seeded yet,
-- which is a valid state: the column defaults to '' and seed-cpv fills it.
--
-- string_agg's row order is unspecified unless the aggregate itself carries an
-- ORDER BY — without one, two runs over the identical vocabulary can legally
-- concatenate the same labels in a different order, which would make this
-- column (and everything downstream, including RecomputeLabels' dirty-check)
-- flap on every re-run even though nothing actually changed. ORDER BY (code,
-- lang) makes the result byte-identical run over run.
UPDATE tenders.ingested_tenders t
SET cpv_labels = coalesce((
        SELECT string_agg(ct.label, ' ' ORDER BY ct.code, ct.lang)
        FROM tenders.cpv_terms ct
        WHERE ct.code = t.cpv OR ct.code = ANY(t.cpv_secondary)
    ), '')
WHERE t.cpv <> '' OR cardinality(t.cpv_secondary) > 0;

-- Postgres cannot change a generated column's expression portably, so the column
-- is dropped and re-added. Dropping it also drops its index, hence the CREATE
-- INDEX below — without it every lexical search silently becomes a sequential
-- scan.
ALTER TABLE tenders.ingested_tenders DROP COLUMN IF EXISTS search_vector;

ALTER TABLE tenders.ingested_tenders
    ADD COLUMN IF NOT EXISTS search_vector tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('simple', coalesce(buyer_name, '')), 'B') ||
        setweight(to_tsvector('simple', coalesce(cpv, '') || ' ' || coalesce(cpv_labels, '')), 'C') ||
        setweight(to_tsvector('simple', coalesce(description, '')), 'D')
    ) STORED;

CREATE INDEX IF NOT EXISTS idx_ingested_tenders_search_vector
    ON tenders.ingested_tenders USING GIN (search_vector);
