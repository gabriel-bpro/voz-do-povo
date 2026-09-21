-- Emendas parlamentares (Portal da Transparência weekly bulk spreadsheet).
-- One row per emenda register; the source's código da emenda is the natural
-- unique key (an emenda's values are restated as execution advances, and the
-- weekly re-ingest upserts + reconciles per year against ground truth).

CREATE TABLE curated.emenda (
    id                BIGSERIAL PRIMARY KEY,
    politician_id     BIGINT REFERENCES curated.politician (id),  -- null = unlinked
    author_name       TEXT NOT NULL,         -- name as published (no CPF/UF in the source)
    code              TEXT NOT NULL,         -- código da emenda: the source's unique id
    year              INT NOT NULL,
    number            TEXT,                  -- número da emenda (display)
    tipo              TEXT NOT NULL,         -- tipo de emenda as published
    is_pix            BOOLEAN NOT NULL DEFAULT FALSE,  -- transferência especial:
                                                       -- permanent type marker, not a badge
    localidade        TEXT,                  -- where the money goes (município/UF);
                                             -- stands in for the beneficiary, which the
                                             -- bulk spreadsheet does not carry
    funcao            TEXT,                  -- policy area
    subfuncao         TEXT,
    amount_empenhado  NUMERIC(16, 2) NOT NULL DEFAULT 0,  -- reserved
    amount_liquidado  NUMERIC(16, 2) NOT NULL DEFAULT 0,
    amount_pago       NUMERIC(16, 2) NOT NULL DEFAULT 0,  -- actually paid
    raw_id            BIGINT,                -- provenance pointer into raw.payload
    synced_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (code)
);

CREATE INDEX emenda_politician_year_idx ON curated.emenda (politician_id, year);
