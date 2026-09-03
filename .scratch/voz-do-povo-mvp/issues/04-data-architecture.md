# Design the data architecture (mirror, sync, search)

Type: grilling
Status: resolved
Blocked by: 01, 02

## Question

How is the mirror built?

- Store topology: roles of Postgres and Elasticsearch (confirm or revise the Next + Nest + Elasticsearch sketch).
- Sync pipeline: per-source cadence and cache TTL derived from each source's real churn (daily expenses vs per-election patrimony).
- Schema strategy enabling cross-source joins (expenses ↔ patrimony ↔ campaign financing on one politician).
- How the indexed data serves both the home dashboard and the NL-query engine.

Must fit Railway hosting on a side-project budget (~R$0–100/month, flexible if value is shown).

## Answer

_Decided 2026-08-28 (grilling session, 3 rounds)._

**Store topology: Postgres only.** The Next + Nest + Elasticsearch sketch is revised — Elasticsearch is dropped. All real workloads (rankings, profile sections, badge rules) are SQL aggregations; politician-name search uses Postgres trigram/full-text indexes. Nothing in the architecture forecloses adding a search engine later.

**Sync runner: dedicated Nest worker service on Railway**, same codebase as the API, deployed separately, with per-source internal schedulers. Railway topology: Next web service + Nest API service + Nest worker service + Railway Postgres.

**Historical depth: current term + current election only.** Legislative data (CEAP/CEAPS, votes, attendance) from 2023+; TSE data (candidatos, bens, receitas, despesas) for the 2022 and 2026 cycles only. Patrimony evolution is thin (max 2 points) — accepted. Expansion is a resync of older years into the same pipeline, not a migration.

**Layers: raw staging + curated.** Syncs land files/rows verbatim in raw tables with provenance (source, pulled-at); a separate cleaning step promotes into curated app-facing tables. Cleaning bugs are re-runnable; bad parses never touch what the app serves.

**Curated schema: unified per-dataset tables.** One `expense` table (house as a column), one `vote`, one `attendance_event`, etc., around a canonical `politician` entity with a `politician_source_id` map (Câmara URI id, Senado code, TSE sequencial/CPF/título). Cross-house rankings are plain GROUP BYs. A normalized `party` dimension absorbs sigla drift across years.

**Identity linkage: CPF-first, deterministic fallback, confidence-gated.** Exact identifier match (CPF where populated — population in recent dumps still unverified, checked at implementation; título eleitoral as secondary) → auto-link. Fuzzy name+UF+cargo matches → manual review queue (~594 current members, tiny volume). Non-matches stay unlinked: profiles render per-source data with an honest "not linked" state. A false link is a defamation-grade bug; a missed link is just missing data.

**Per-source sync cadence:**
- CEAP/CEAPS expenses: **daily API delta** (last few days) + **weekly bulk re-ingest** of the current-year zip/CSVs into raw as ground-truth reconciliation.
- Votes (both houses): daily API delta.
- Attendance: Câmara via daily bulk (`eventosPresencaDeputados`); **Senado derived from the sessions API** (daily job walks recent plenary sessions, polite polling).
- TSE quartet via Base dos Dados: **monthly** pull (weekly during campaign season), gated on BD's `updates.latest` metadata via their GraphQL API; worker uses `@google-cloud/bigquery` with service-account credentials (GCP billing project setup is a separate manual task — new ticket).
- Emendas (Portal da Transparência): **weekly bulk spreadsheet** download — no API key needed.
- Sanctions (CEIS/CNEP): **monthly bulk spreadsheet** download — no API key needed.

**Serving the app: compute on demand, optimize later.** Rankings, badge medians, and profile totals query the curated tables directly with proper indexes; materialized rollups are a later optimization when perf hurts. (The attention-badge ticket can rely on arbitrary SQL aggregates over curated tables.)

**NL-query engine interface: LLM-generated SQL with strict validation.** The LLM emits SQL; execution goes through a read-only DB role after mechanical validation — SELECT-only, curated-tables-only allowlist, row caps and timeouts. No fixed intent templates.

**Sync failure handling: logs only (MVP).** Stdout logs checked on Railway; the fuller ops story (sync_runs table, freshness API, stale-source detection, data-quality monitoring) stays in the map's fog.

**Cost posture:** four Railway units (web, API, worker, Postgres) with shallow history keeps the store in the low GB range; expected to fit the ~R$0–100/month envelope. Precise monthly cost still open — graduates with the LLM-strategy ticket.

## Comments

_Amendment 2026-09-02 (pivot in [ticket 05](05-llm-strategy.md)):_ the "NL-query engine interface: LLM-generated SQL with strict validation" decision above is **moot** — free-form NL querying left the MVP. Everything else stands: the app queries curated tables on demand, and the LLM never touches the store directly (walkthroughs are precomputed post-sync from the curated layer).
