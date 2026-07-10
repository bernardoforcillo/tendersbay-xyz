CREATE SCHEMA IF NOT EXISTS tenders;

CREATE TABLE IF NOT EXISTS tenders.ingested_tenders (
    id              bigserial PRIMARY KEY,
    source          text NOT NULL,
    source_ref      text NOT NULL,
    title           text NOT NULL,
    buyer_name      text NOT NULL DEFAULT '',
    buyer_id        text NOT NULL DEFAULT '',
    status          text NOT NULL DEFAULT 'unknown'
                        CHECK (status IN ('open', 'awarded', 'cancelled', 'closed', 'unknown')),
    procedure_type  text NOT NULL DEFAULT '',
    language        text NOT NULL DEFAULT '',
    country         text NOT NULL DEFAULT '',
    nuts            text NOT NULL DEFAULT '',
    cpv             text NOT NULL DEFAULT '',
    cpv_secondary   text[] NOT NULL DEFAULT '{}',
    value           bigint,
    currency        text NOT NULL DEFAULT '',
    published_at    timestamptz,
    deadline        timestamptz,
    raw             jsonb,
    version         integer NOT NULL DEFAULT 1,
    history         jsonb NOT NULL DEFAULT '[]'::jsonb,
    first_seen_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (source, source_ref)
);

CREATE TABLE IF NOT EXISTS tenders.ingested_tender_documents (
    id          bigserial PRIMARY KEY,
    tender_id   bigint NOT NULL REFERENCES tenders.ingested_tenders(id) ON DELETE CASCADE,
    url         text NOT NULL,
    type        text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tender_id, url)
);

CREATE TABLE IF NOT EXISTS tenders.ingested_tender_lots (
    id          bigserial PRIMARY KEY,
    tender_id   bigint NOT NULL REFERENCES tenders.ingested_tenders(id) ON DELETE CASCADE,
    ref         text NOT NULL,
    title       text NOT NULL DEFAULT '',
    cpv         text NOT NULL DEFAULT '',
    value       bigint,
    currency    text NOT NULL DEFAULT '',
    deadline    timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tender_id, ref)
);

CREATE TABLE IF NOT EXISTS tenders.ingestion_runs (
    id           bigserial PRIMARY KEY,
    source       text NOT NULL,
    started_at   timestamptz NOT NULL,
    finished_at  timestamptz NOT NULL,
    fetched      integer NOT NULL DEFAULT 0,
    inserted     integer NOT NULL DEFAULT 0,
    updated      integer NOT NULL DEFAULT 0,
    error        text
);
