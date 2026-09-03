# 12 — Câmara expenses sync + core schema

**What to build:** the mirror's foundation, landed with its first dataset. Curated schema: canonical `politician` entity, `politician_source_id` map, normalized `party` dimension, and the unified `expense` table (house as a column). The worker syncs the Câmara deputies registry and CEAP expenses — daily API delta plus weekly bulk re-ingest of the current-year files as ground-truth reconciliation — through the raw layer into curated. The identity-linkage mechanism is built here: CPF-first exact-match auto-linking, fuzzy matches to a manual review queue, non-matches left unlinked. Historical depth: 2023+. Verifiable by inspection: curated tables fill with real Câmara data.

**Blocked by:** 11 — Walking skeleton.

**Status:** ready-for-agent

- [ ] Canonical `politician`, `politician_source_id`, `party`, and unified `expense` curated tables exist
- [ ] Câmara deputies registry syncs into curated, creating canonical politicians with Câmara source ids
- [ ] CEAP daily delta sync promotes expenses raw → curated with provenance intact
- [ ] CEAP weekly bulk re-ingest reconciles the current year (bulk-file identifier deviations from the API are handled)
- [ ] Identity linkage auto-links on exact CPF, routes fuzzy matches to a review queue, and never force-links
- [ ] Sync failures surface as logs only; a failed sync leaves previously curated data untouched
- [ ] Sync-seam tests cover: fixture CEAP payload → expected raw + curated state, including a review-queue case
