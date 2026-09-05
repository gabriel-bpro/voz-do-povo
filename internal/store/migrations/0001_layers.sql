-- Mirror layers: raw staging (verbatim source data with provenance, never read
-- by the app) and curated (cleaned, unified tables; the only schema the app
-- queries). Domain tables arrive with the sync tickets (12+).

CREATE SCHEMA IF NOT EXISTS raw;
CREATE SCHEMA IF NOT EXISTS curated;

CREATE TABLE IF NOT EXISTS raw.payload (
    id        BIGSERIAL PRIMARY KEY,
    source    TEXT NOT NULL,
    payload   JSONB NOT NULL,
    pulled_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
