-- Citizen-facing name search: trigram similarity over the normalized name
-- (already uppercased + accent-folded at sync time), per the data-architecture
-- decision (Postgres-only search, no Elasticsearch).

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX politician_name_norm_trgm_idx
    ON curated.politician USING gin (name_norm gin_trgm_ops);
