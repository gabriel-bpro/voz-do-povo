-- Curated core schema: canonical politician entity, per-source id map,
-- normalized party dimension, unified expense table (house as a column), and
-- the identity-linkage manual review queue.

CREATE TABLE curated.party (
    id    BIGSERIAL PRIMARY KEY,
    sigla TEXT NOT NULL UNIQUE,  -- party acronym as published by the source
    name  TEXT                   -- full name, when a source provides it
);

CREATE TABLE curated.politician (
    id                 BIGSERIAL PRIMARY KEY,
    civil_name         TEXT NOT NULL,
    parliamentary_name TEXT NOT NULL,
    name_norm          TEXT NOT NULL,  -- uppercased, unaccented, space-collapsed; for matching
    uf                 TEXT,           -- electoral UF at registry time (fuzzy-match key)
    cpf                TEXT,           -- digits only, when the source publishes it
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- CPF is the auto-link key across sources: at most one politician per CPF.
CREATE UNIQUE INDEX politician_cpf_key ON curated.politician (cpf) WHERE cpf IS NOT NULL;
CREATE INDEX politician_name_norm_uf_idx ON curated.politician (name_norm, uf);

-- Per-source identifiers for a canonical politician (Câmara URI id, Senado
-- code, TSE ids). All cross-source joins go through this map.
CREATE TABLE curated.politician_source_id (
    politician_id BIGINT NOT NULL REFERENCES curated.politician (id),
    source        TEXT NOT NULL,  -- e.g. 'camara'
    source_id     TEXT NOT NULL,
    PRIMARY KEY (source, source_id)
);

-- Identity linkage: fuzzy matches land here for a human to decide. A queued
-- record is NEVER linked until a human resolves it.
CREATE TABLE curated.identity_link_review (
    id                      BIGSERIAL PRIMARY KEY,
    source                  TEXT NOT NULL,            -- source the record came from
    identity                JSONB NOT NULL,           -- the identifiers we tried to link
    candidate_politician_id BIGINT REFERENCES curated.politician (id),
    reason                  TEXT NOT NULL,            -- e.g. 'fuzzy-name-uf-match'
    status                  TEXT NOT NULL DEFAULT 'pending',  -- pending | linked | rejected
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Unified expense table (CEAP + CEAPS; house as a column, quotas never mixed
-- in comparisons). One row per source expense register.
CREATE TABLE curated.expense (
    id                 BIGSERIAL PRIMARY KEY,
    politician_id      BIGINT REFERENCES curated.politician (id),  -- null = unlinked
    party_id           BIGINT REFERENCES curated.party (id),
    house              TEXT NOT NULL,             -- 'camara' | 'senado'
    category           TEXT NOT NULL,             -- e.g. CEAP tipo de despesa
    supplier_name      TEXT,
    supplier_tax_id    TEXT,                      -- CNPJ/CPF, digits only
    document_number    TEXT,
    issued_at          DATE,
    year               INT NOT NULL,
    month              INT NOT NULL,
    amount_document    NUMERIC(14, 2),
    amount_glosa       NUMERIC(14, 2),            -- disallowed; negative net = glosa/ressarcimento
    amount_net         NUMERIC(14, 2) NOT NULL,
    document_url       TEXT,
    source_document_id TEXT,                      -- CEAP ideDocumento: NOT unique per register
                                                  -- (installments share a document); grouping key only
    source_row_key     TEXT NOT NULL,             -- hash of the full normalized row: the dedup key
    raw_id             BIGINT,                    -- provenance pointer into raw.payload (not an FK:
                                                  -- raw rows are replaced on re-ingest)
    synced_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (house, source_row_key)
);

CREATE INDEX expense_politician_year_idx ON curated.expense (politician_id, year);
