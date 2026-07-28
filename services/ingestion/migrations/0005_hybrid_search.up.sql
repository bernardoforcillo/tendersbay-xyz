-- Foundations for hybrid (lexical + dense) tender search.
--
-- The tsvector uses the 'simple' text search configuration on purpose: this
-- one table holds notices in 24 EU languages, so any single stemmer
-- ('english', 'italian', …) would be wrong for most rows. 'simple' folds case
-- and splits on word boundaries without stemming, which is exactly right for
-- the identifier-shaped tokens that matter most here — CPV codes, notice
-- references, buyer names. Language-aware stemming, if it ever earns its
-- keep, belongs in a per-row regconfig derived from ingested_tenders.language,
-- not in a single global guess.
--
-- Column weights follow how discriminating each field is: title (A) beats
-- buyer_name (B) beats cpv (C) beats description (D). cpv_secondary stays out
-- of the vector and gets its own GIN array index below — set containment is a
-- better fit for it than text search.

CREATE EXTENSION IF NOT EXISTS pg_trgm;

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

-- Trigram indexes back the fuzzy/substring fallback the tsvector can't serve:
-- a misspelled or truncated buyer ("comune bergam") produces no lexeme match
-- but a high trigram similarity.
CREATE INDEX IF NOT EXISTS idx_ingested_tenders_title_trgm
    ON tenders.ingested_tenders USING GIN (title gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_ingested_tenders_buyer_name_trgm
    ON tenders.ingested_tenders USING GIN (buyer_name gin_trgm_ops);

-- Filter-supporting indexes. Each one backs a predicate the search API
-- already issues today with nothing behind it but a sequential scan.
CREATE INDEX IF NOT EXISTS idx_ingested_tenders_status_deadline
    ON tenders.ingested_tenders (status, deadline);

-- text_pattern_ops is what makes the CPV prefix filter (cpv LIKE '45%')
-- indexable regardless of the database's collation.
CREATE INDEX IF NOT EXISTS idx_ingested_tenders_cpv_prefix
    ON tenders.ingested_tenders (cpv text_pattern_ops);

CREATE INDEX IF NOT EXISTS idx_ingested_tenders_nuts_prefix
    ON tenders.ingested_tenders (nuts text_pattern_ops);

-- The filters-only path orders by published_at DESC on every request.
CREATE INDEX IF NOT EXISTS idx_ingested_tenders_published_at
    ON tenders.ingested_tenders (published_at DESC);

CREATE INDEX IF NOT EXISTS idx_ingested_tenders_value
    ON tenders.ingested_tenders (value);

CREATE INDEX IF NOT EXISTS idx_ingested_tenders_cpv_secondary
    ON tenders.ingested_tenders USING GIN (cpv_secondary);

-- Partial index over the indexing backlog — the indexer polls
-- "indexed_at IS NULL" every cycle, and 0006 is about to make that the whole
-- table.
CREATE INDEX IF NOT EXISTS idx_ingested_tenders_unindexed
    ON tenders.ingested_tenders (id) WHERE indexed_at IS NULL;
