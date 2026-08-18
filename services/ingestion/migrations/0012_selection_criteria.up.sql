-- Selection criteria: the conditions a bidder must satisfy to be ADMITTED to a
-- procedure, as opposed to the award criteria in 0010 that score bids already
-- admitted. Two different questions the same notice answers side by side, so
-- two tables rather than one with a discriminator — a single table invites
-- averaging a weight across both, and the result would mean nothing.
--
--
-- WHY THERE IS NO weight COLUMN
--
-- Selection is pass/fail. Nothing scores a selection criterion, so a weight
-- here would be NULL on every row ever written. That is structurally unlike
-- award_criteria.weight, where NULL is the common case but is a FACT about a
-- particular notice (the buyer named an award method and attached no weights);
-- here it would be a fact about the concept. A column that can never be
-- populated is not a nullable column, it is a mistake with a default.
--
--
-- WHY THERE IS NO THRESHOLD COLUMN, WHICH IS THE POINT OF THIS TABLE
--
-- The obvious shape is threshold_amount / threshold_currency / threshold_kind,
-- so "do we clear €309.552,00 of annual turnover" is a comparison rather than a
-- reading exercise. Reject it, for the same reason 0011 rejected docs_coverage:
-- it would be a claim wearing the clothes of a measurement.
--
-- NO SOURCE OBSERVED PUBLISHES A COMPARABLE THRESHOLD.
--
--   Spain (PLACSP/CODICE) publishes the requirement as PROSE, and the prose is
--   genuinely rich — "un volumen anual de negocios referido al año de mayor
--   volumen de los tres últimos concluidos igual o superior a 309.552,00 €,
--   equivalente a una vez y media el VEC (artículo 87.3.a) LCSP)". The number
--   is in there. It is in there as Spanish, next to a legal basis, a reference
--   period and a derivation, none of which survive extraction to a single
--   numeric column.
--
--   TED (eForms) mostly publishes a POINTER. Where the qualification block
--   appears at all, its content is selection-criteria-source = epo-sub-espd,
--   which means "the criteria are in the ESPD document" — no name, no amount,
--   no category. There is nothing to put in a threshold column at all.
--
-- So a threshold column would be NULL on every row this table can currently be
-- built from, while reading — to anyone writing a query — as though a populated
-- one meant "we checked". Recovering a comparable threshold is a document
-- extraction problem with an accuracy bar and an eval to clear, and the column
-- belongs to whichever pass earns the right to claim one, together with the
-- provenance that lets a reader disbelieve it.
--
--
-- WHY category AND type ARE TWO COLUMNS
--
-- type alone is ambiguous, and silently so. PLACSP publishes the bare code "5"
-- under a FinancialCapabilityTypeCode list and the bare code "1" under a
-- DeclarationTypeCode list; store type without category and both are integers
-- that collide, describing unrelated requirements. category is the normalized
-- family (technical | financial | declaration) — every source observed splits
-- selection the same three ways — and type is the source's own code within it,
-- kept verbatim because mapping it onto a domain vocabulary is a decision for
-- the reader that owns that vocabulary, not for the writer.
--
--
-- WHY origin IS STORED
--
-- Prose scraped from a search feed and a structured code parsed from a notice
-- document are not equally strong evidence, and the difference has to survive
-- persistence or every reader re-derives it wrongly. origin names the reading
-- that produced the row ('es-placsp'), so the domain that owns admissibility
-- can grade trust per row. The authority ceiling itself is NOT encoded here:
-- this table records what was published and who read it, and stays out of the
-- question of what may block a bid.
--
--
-- REPLACE, NOT UPSERT — the same rule as award criteria, for the same reason.
-- A criteria set is a SET: the writer deletes by tender_id and re-inserts in one
-- transaction. Upserting on (tender_id, lot_ref, ordinal) leaves stale rows
-- whenever a corrected notice publishes FEWER criteria than the one it replaces,
-- so the count could only grow and a withdrawn requirement would outlive its
-- withdrawal.
--
-- Lot-scoped entries live in this same flat table, told apart by lot_ref, rather
-- than being split the way award criteria are across notice level and lot rows.
-- That split exists so the stored grid_usable denormalisation can be verified
-- against its child rows; selection criteria carry no such aggregate, so a
-- second partition invariant would be cost with no reader to serve.
CREATE TABLE IF NOT EXISTS tenders.ingested_tender_selection_criteria (
    id          bigserial PRIMARY KEY,
    tender_id   bigint NOT NULL REFERENCES tenders.ingested_tenders(id) ON DELETE CASCADE,
    lot_ref     text NOT NULL DEFAULT '',   -- '' = notice-level
    ordinal     integer NOT NULL,
    category    text NOT NULL DEFAULT '',   -- technical | financial | declaration; '' = source filed it under none
    type        text NOT NULL DEFAULT '',   -- the source's own code, meaningful only within category
    name        text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',   -- the requirement in prose, as published
    origin      text NOT NULL DEFAULT '',   -- which reading produced this row, e.g. 'es-placsp'
    lang        text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tender_id, lot_ref, ordinal)
);

-- The read is always "every selection criterion for this tender", in published
-- order — there is no query that wants one criterion by id, and none that wants
-- criteria across tenders. One index serves the whole access pattern.
CREATE INDEX IF NOT EXISTS ingested_tender_selection_criteria_tender_idx
    ON tenders.ingested_tender_selection_criteria (tender_id, lot_ref, ordinal);
