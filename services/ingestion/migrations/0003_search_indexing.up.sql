ALTER TABLE tenders.ingested_tenders ADD COLUMN IF NOT EXISTS indexed_at timestamptz;

CREATE TABLE IF NOT EXISTS tenders.ingested_tender_document_parts (
    id           bigserial PRIMARY KEY,
    document_id  bigint NOT NULL REFERENCES tenders.ingested_tender_documents(id) ON DELETE CASCADE,
    index        integer NOT NULL,
    content      text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (document_id, index)
);
