# 17 — Emendas sync + profile emendas section

**What to build:** the emendas half of the diligence story. The worker syncs emendas parlamentares from Portal da Transparência weekly bulk spreadsheets (no API key) through raw into a curated emendas table linked to canonical politicians. The profile gains the emendas section: yearly totals empenhado vs pago, breakdown by policy area and by state/municipality, top 5 largest emendas with beneficiary names, emendas PIX flagged distinctly wherever they appear, and the plain-language line explaining what an emenda is and why the empenhado/pago gap matters.

**Blocked by:** 13 — Search + profile expenses section.

**Status:** ready-for-agent

- [x] Weekly bulk spreadsheet sync promotes emendas raw → curated, linked to canonical politicians
- [x] Profile shows yearly empenhado vs pago totals with the plain-language emenda/gap explainer
- [x] Breakdowns by policy area and by state/municipality render in the locked style
- [x] Top 5 largest emendas show beneficiary names
- [x] Emendas PIX are flagged distinctly as a permanent type marker (not an attention badge)
- [x] Sync-seam tests cover fixture spreadsheet → curated state, including PIX classification
- [x] HTTP-API-seam tests cover the emendas section payload incl. source + sync date

**Done 2026-09-21.** Decisions (user, this session): the bulk CSV has no favorecido column, so top-5 "beneficiary" renders the source's `Localidade do gasto`; identity linkage is name-only (the file carries no CPF/UF/source id) — exact unique normalized-name auto-link, else pg_trgm similarity ≥ 0.90 to the single closest match, ties/collisions to the review queue. PIX = tipo "Transferências Especiais" (plural, verified live). Cadence is weekly (Sunday-gated job); `worker bootstrap-emendas` backfills 2023+. Breakdowns are capped at the top 8 rows per grouping; the year selector is shared with the expenses section (same `?year=`).

**Amended 2026-09-22:** the bulk spreadsheet is Akamai-blocked from datacenters (500, verified — same wall as TSE), so the worker syncs via the keyed REST API (`TRANSPARENCIA_API_KEY` on the worker service; ~420 pages/year at 15 rows/page, paced under the 400 req/min limit); the bulk CSV path remains as the keyless fallback. Live data also forced two parser fixes: floating-sign negatives (`"- 17.088,41"`) and the plural PIX tipo. The API key itself is NOT in the repo.
