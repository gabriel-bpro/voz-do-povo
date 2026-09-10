# 12 — Câmara expenses sync + core schema

**What to build:** the mirror's foundation, landed with its first dataset. Curated schema: canonical `politician` entity, `politician_source_id` map, normalized `party` dimension, and the unified `expense` table (house as a column). The worker syncs the Câmara deputies registry and CEAP expenses — daily API delta plus weekly bulk re-ingest of the current-year files as ground-truth reconciliation — through the raw layer into curated. The identity-linkage mechanism is built here: CPF-first exact-match auto-linking, fuzzy matches to a manual review queue, non-matches left unlinked. Historical depth: 2023+. Verifiable by inspection: curated tables fill with real Câmara data.

**Blocked by:** 11 — Walking skeleton.

**Status:** resolved

- [x] Canonical `politician`, `politician_source_id`, `party`, and unified `expense` curated tables exist
- [x] Câmara deputies registry syncs into curated, creating canonical politicians with Câmara source ids
- [x] CEAP daily delta sync promotes expenses raw → curated with provenance intact _(amended: the API delta doesn't exist upstream — daily bulk re-ingest covers it; see Answer)_
- [x] CEAP weekly bulk re-ingest reconciles the current year (bulk-file identifier deviations from the API are handled)
- [x] Identity linkage auto-links on exact CPF, routes fuzzy matches to a review queue, and never force-links
- [x] Sync failures surface as logs only; a failed sync leaves previously curated data untouched
- [x] Sync-seam tests cover: fixture CEAP payload → expected raw + curated state, including a review-queue case

## Answer

_2026-09-10 — resolved._

**Schema** (`internal/store/migrations/0002_curated_core.sql`): `curated.party` (sigla unique), `curated.politician` (civil + parliamentary names, `name_norm` unaccented match key, `cpf` unique-when-present), `curated.politician_source_id` ((source, source_id) PK), `curated.identity_link_review` (pending fuzzy/conflict matches, deduped), `curated.expense` (house as column, provenance `raw_id` pointer, `UNIQUE (house, source_row_key)`).

**Source findings** (verified live against the real endpoints, 2026-09-08/10):

- The REST API's `/deputados/{id}/despesas` endpoint **returns empty for every deputy** — the planned daily API delta doesn't exist. The bulk year files are updated daily, so the current-year bulk re-ingest runs **daily** and doubles as the delta (spec.md amended).
- The bulk **JSON** file deviates hard from the API conventions: camelCase fields (`nomeParlamentar`, `valorLiquido`…), **no `ideCadastro`** (only `numeroDeputadoID`, the portal's id — not the API id), amounts as strings, and the parliamentarian **CPF populated on ~99.5% of rows** — CPF is the primary expense→politician link key.
- CEAP registers have **no unique row id** (`idDocumento` groups installments — official field doc). Dedup key = SHA-256 of the full normalized row (`source_row_key`): a source-side correction lands as a new register and the reconcile pass (`DELETE … synced_at < runStart`, bulk = ground truth) removes the stale one.
- The deputy **registry** API works fine (`/deputados` list + detail with CPF). Registry sync fetches details only for deputies lacking a CPF in the mirror.

**Entry points** (sync seam): `sync.SyncCamaraDeputies`, `sync.SyncCamaraExpenses` — both take already-fetched data, all-or-nothing in one transaction (failed sync = rollback + log). `internal/camara` fetches/parses only, never touches the DB. Raw layer is **replace-per-source-key** staging (Hobby 5GB volume cap), provenance via source + pulled_at + `raw_id` pointer on curated rows.

**Identity linkage** (`sync.LinkIdentity`): source-id → CPF exact (auto-link + source-id backfill; a source id already mapped to a *different* politician is queued as `conflicting-identifiers`, never silently kept) → normalized name+UF → `identity_link_review` (never linked) → unlinked.

**Worker** (`cmd/worker`): jobs run at startup then on interval — `camara-registry` daily, `camara-expenses-bulk` (current year) daily. Failures are logs-only. History backfill: `worker bootstrap-camara` re-ingests 2023→current once (manual, post-deploy).

**Tests:** sync seam — registry (raw provenance, canonical creation, idempotency), bulk (full linkage matrix incl. review-queue case, promoted fields, provenance), reconcile (correction + stale-row deletion + queue dedup), failure-rollback. Parse of the real 2026 file verified manually (101,047 rows); the unit-level parse test was removed per the two-seams rule.

**Deviations recorded:** API delta dead upstream (daily bulk instead); fuzzy match is name+UF only — CEAP rows carry no cargo; 2023+ history lands via the manual bootstrap command rather than a recurring job.

**Deploy note:** `railway up --service worker --detach`, then run `bootstrap-camara` once to fill 2023+; curated tables then fill by inspection.
