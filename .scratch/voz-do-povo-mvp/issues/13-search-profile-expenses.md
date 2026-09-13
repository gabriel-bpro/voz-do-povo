# 13 — Search + profile expenses section

**What to build:** the first citizen-visible slice. Home page with the politician search bar front and center plus the one-line project explainer; name search via Postgres trigram/full-text. Politician profile page with the identity header (templated plain-language summary — zero LLM) and the complete expenses section in the locked prototype style: current-calendar-year total with a term-year selector (2023+), monthly trend, largest items, per-dataset source + sync date, stat cards / big numbers / bars with inline concept tooltips (e.g. cota parlamentar) and human-scale anchors. When a source can't be confidently linked, the section renders the honest "not linked" state.

**Blocked by:** 12 — Câmara expenses sync + core schema.

**Status:** ready-for-agent

- [x] Citizen can search politicians by name from the home page and reach a profile
- [x] Profile identity header renders a templated plain-language summary (no LLM, no free prose)
- [x] Expenses section shows current-year total, term-year selector back to 2023, monthly trend, and largest items
- [x] Every dataset section displays its source name and last sync date
- [x] An unlinked source renders the explicit "not linked" state rather than fabricating or hiding
- [x] Style matches the locked prototype: stat cards, big numbers, bars, inline tooltips, human-scale anchors
- [x] API responses for search and profile carry source + sync-date metadata
- [x] HTTP-API-seam tests cover search, profile expenses payload, and the unlinked-state response

**Done 2026-09-13.** Deferred: the profile header's flagged-items rollup count (spec) waits on the badge rules (issues 14/18) — no flags exist to count yet. The year selector covers the full term (2023+), even years with zero expenses. Registry freshness is derived from curated `politician` timestamps (the app never reads raw).
