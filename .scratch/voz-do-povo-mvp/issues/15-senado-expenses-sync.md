# 15 — Senado expenses sync

**What to build:** the Senado joins the mirror. Senators registry and CEAPS expenses (daily via CSV + API) sync into the same unified curated tables — house as a column, CEAP and CEAPS quotas never mixed in comparisons. The `politician_source_id` map extends with Senado codes; cross-source identity linking gets its first real exercise (a sitting senator who was previously a deputado links to one canonical politician via CPF, or queues for manual review). Expense surfaces built so far (profile section, median badge) serve Senado data with no per-house special-casing beyond the house-aware comparisons.

**Blocked by:** 12 — Câmara expenses sync + core schema.

**Status:** ready-for-agent

- [x] Senators registry syncs into curated with Senado source ids
- [x] CEAPS expenses sync daily into the unified expense table
- [x] A politician present in both houses resolves to one canonical politician (exact-CPF auto-link or review queue)
- [x] Profile expenses section and median badge serve Senado data correctly, per-house buckets intact
- [x] Sync-seam tests cover CEAPS fixture → curated state, including a cross-house identity-link case

**Implemented 2026-09-16.** Adapter: `internal/senado` (registry via legis API `senador/lista/atual.json`; CEAPS via admin API `/api/v1/senadores/despesas_ceaps/{year}` JSON — the CSV variant is latin-1 with decimal commas). Key facts verified live: CEAPS rows are uniquely keyed and carry `codSenador` (auto-link via the source-id map); the Senado publishes no senator CPF, so cross-house linkage resolves via the name+UF review queue (the senator still gets her own politician record pending human resolution — never auto-merged). No migration needed (`expense.house` was already house-scoped). Worker: `senado-registry` + `senado-ceaps` daily jobs (registry first) and `bootstrap-senado` for 2023+ backfill.

**Known edges (from code review, accepted for now):**
- A merged both-house politician (deputada → senadora) shows only the active house's expenses on the profile (senado wins); past-mandate CEAP rows are hidden until profiles render per-house sections (rankings/profile-split work).
- If the Senado ever publishes senator CPFs, thread them through `Senator` → `LinkIdentity` (path exists, field doesn't).
- `internal/senado` imports `camara.DigitsOnly`; fine at two houses, wants a shared util if a third adapter appears.
