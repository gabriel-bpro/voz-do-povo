-- Nominal votes + attendance (ticket 19), both houses unified.
--
-- vote: one politician's position in one nominal vote. Denormalized
-- (description, matter ref, result on every row) like expense — the profile
-- reads never join back to a vote-event table. The dedup key is the source's
-- own identifiers: (house, source_vote_id, member_source_id). A source-side
-- correction upserts in place; the weekly re-ingest reconciles per year
-- against ground truth.
--
-- vote_orientation: one party's leadership orientation in one nominal vote.
-- Only real party siglas land here (Governo/Oposição/Minoria blocs are not
-- comparable to a politician's party and are dropped at sync). party_sigla is
-- normalized uppercase; the Senado publishes some parties by full name
-- ("Progressistas"), normalized to the sigla at sync.
--
-- plenary_session: one plenary session that held at least one nominal vote —
-- the attendance-rate denominator ("sessões com votação desta legislatura").
--
-- attendance: one politician's status in one plenary session. The Câmara
-- source (eventosPresencaDeputados) publishes presences only, so Câmara rows
-- are always status='present' and the API computes absences against
-- plenary_session. The Senado source lists every senator per vote, so Senado
-- rows carry present/justified/absent derived from the vote codes.

CREATE TABLE curated.vote (
    id               BIGSERIAL PRIMARY KEY,
    politician_id    BIGINT REFERENCES curated.politician (id),  -- null = unlinked
    house            TEXT NOT NULL,
    source_vote_id   TEXT NOT NULL,   -- camara idVotacao / senado CodigoSessaoVotacao
    member_source_id TEXT NOT NULL,   -- deputy API id / codigoParlamentar at vote time
    voted_at         TIMESTAMPTZ NOT NULL,
    year             INT NOT NULL,
    description      TEXT NOT NULL,   -- what was voted (tema), as published
    matter_ref       TEXT,            -- "PDL 995/2026" when the source identifies it
    approved         BOOLEAN,         -- null = result unknown
    vote             TEXT NOT NULL,   -- Sim / Não / Abstenção / Obstrução / Votou (secreta) / presence codes
    party_sigla      TEXT,            -- voter's party at vote time (uppercase)
    raw_id           BIGINT,          -- provenance pointer into raw.payload
    synced_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (house, source_vote_id, member_source_id)
);

CREATE INDEX vote_politician_recency_idx ON curated.vote (politician_id, voted_at DESC);

CREATE TABLE curated.vote_orientation (
    id             BIGSERIAL PRIMARY KEY,
    house          TEXT NOT NULL,
    source_vote_id TEXT NOT NULL,
    party_sigla    TEXT NOT NULL,   -- normalized uppercase
    orientation    TEXT NOT NULL,   -- Sim / Não / Liberado / ...
    year           INT NOT NULL,
    raw_id         BIGINT,
    synced_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (house, source_vote_id, party_sigla)
);

CREATE TABLE curated.plenary_session (
    id          BIGSERIAL PRIMARY KEY,
    house       TEXT NOT NULL,
    session_key TEXT NOT NULL,      -- camara idEvento / senado CodigoSessao
    session_date DATE NOT NULL,
    year        INT NOT NULL,
    raw_id      BIGINT,
    synced_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (house, session_key)
);

CREATE TABLE curated.attendance (
    id               BIGSERIAL PRIMARY KEY,
    politician_id    BIGINT REFERENCES curated.politician (id),  -- null = unlinked
    house            TEXT NOT NULL,
    session_key      TEXT NOT NULL,
    member_source_id TEXT NOT NULL,
    status           TEXT NOT NULL,   -- present | justified | absent
    status_detail    TEXT,            -- the source's raw code (AP, P-NRV, …); null for câmara presences
    year             INT NOT NULL,
    raw_id           BIGINT,
    synced_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (house, session_key, member_source_id)
);

CREATE INDEX attendance_politician_idx ON curated.attendance (politician_id, year);
