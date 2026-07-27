ALTER TABLE tenders.ingested_tenders ADD COLUMN IF NOT EXISTS description text NOT NULL DEFAULT '';
