-- The CPV 2008 vocabulary: every Common Procurement Vocabulary code's label in
-- all 24 official EU languages.
--
-- This table is what makes cross-language tender search possible. A CPV code is
-- identical in every EU language, so resolving a query onto codes bridges
-- languages without translating anything: "pulizie uffici" resolves to 90919200
-- and that code is on the German notice too. Neither the lexical arm (which
-- matches lexemes) nor a stemmer (which does not translate) can do that.
--
-- 'simple' is the right text-search configuration here for the same reason it is
-- right on ingested_tenders.search_vector: one table holds 24 languages, so any
-- single stemmer would be wrong for most rows. Matching is by lexeme plus
-- trigram, which is enough for a short label.
--
-- The generated column is deliberately built from to_tsvector('simple', label)
-- and nothing fancier. A diacritic-folding step would widen recall (e.g.
-- folding "Getranke" and "Getränke" to the same lexeme), but PostgreSQL's
-- built-in extension for that is STABLE, not IMMUTABLE — Postgres rejects a
-- STABLE function in a STORED generated column outright at CREATE TABLE time
-- (it would not even apply), and the common workaround of wrapping it in a
-- same-named function declared IMMUTABLE is worse: it lies to the planner, and
-- the stored vectors silently go stale the moment the folding dictionary
-- changes underneath them, with no error to notice it happened. If diacritic
-- folding is ever needed, it belongs in the query side (normalize the search
-- term before matching), not baked into this column.
--
-- pg_trgm already exists (created by 0005) — no CREATE EXTENSION needed here.
--
-- The table is seeded by `go run ./cmd/seed-cpv` (services/ingestion), NOT by
-- this migration: ~9,454 codes x 24 languages (~227k rows) cannot be embedded
-- as INSERT statements in a migration that is compiled into every binary, and
-- Migrator.Up runs each migration inside one transaction.

CREATE TABLE IF NOT EXISTS tenders.cpv_terms (
    code  text NOT NULL,
    lang  text NOT NULL,
    label text NOT NULL,
    -- Generated, so a re-seed can never leave the vector out of step with the
    -- label it was built from.
    label_vector tsvector GENERATED ALWAYS AS (to_tsvector('simple', label)) STORED,
    -- (code, lang) rather than a surrogate key: it is the natural identity, and
    -- it is what makes the seeder's ON CONFLICT DO UPDATE idempotent.
    PRIMARY KEY (code, lang)
);

CREATE INDEX IF NOT EXISTS idx_cpv_terms_label_vector
    ON tenders.cpv_terms USING GIN (label_vector);

-- Backs the fuzzy arm of the query -> code match, for the same reason the
-- tenders table has trigram indexes: a truncated or misspelled term produces no
-- lexeme match but a high trigram similarity.
CREATE INDEX IF NOT EXISTS idx_cpv_terms_label_trgm
    ON tenders.cpv_terms USING GIN (label gin_trgm_ops);

-- The label-expansion path (a later task) looks up every label for one code,
-- and the primary key's leading column already serves that. This index instead
-- backs the reverse lookup the seeder does when recomputing a single language.
CREATE INDEX IF NOT EXISTS idx_cpv_terms_lang
    ON tenders.cpv_terms (lang);
