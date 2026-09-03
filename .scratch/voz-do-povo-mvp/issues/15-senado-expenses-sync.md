# 15 — Senado expenses sync

**What to build:** the Senado joins the mirror. Senators registry and CEAPS expenses (daily via CSV + API) sync into the same unified curated tables — house as a column, CEAP and CEAPS quotas never mixed in comparisons. The `politician_source_id` map extends with Senado codes; cross-source identity linking gets its first real exercise (a sitting senator who was previously a deputado links to one canonical politician via CPF, or queues for manual review). Expense surfaces built so far (profile section, median badge) serve Senado data with no per-house special-casing beyond the house-aware comparisons.

**Blocked by:** 12 — Câmara expenses sync + core schema.

**Status:** ready-for-agent

- [ ] Senators registry syncs into curated with Senado source ids
- [ ] CEAPS expenses sync daily into the unified expense table
- [ ] A politician present in both houses resolves to one canonical politician (exact-CPF auto-link or review queue)
- [ ] Profile expenses section and median badge serve Senado data correctly, per-house buckets intact
- [ ] Sync-seam tests cover CEAPS fixture → curated state, including a cross-house identity-link case
