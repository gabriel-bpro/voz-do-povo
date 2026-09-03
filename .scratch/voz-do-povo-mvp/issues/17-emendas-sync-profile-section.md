# 17 — Emendas sync + profile emendas section

**What to build:** the emendas half of the diligence story. The worker syncs emendas parlamentares from Portal da Transparência weekly bulk spreadsheets (no API key) through raw into a curated emendas table linked to canonical politicians. The profile gains the emendas section: yearly totals empenhado vs pago, breakdown by policy area and by state/municipality, top 5 largest emendas with beneficiary names, emendas PIX flagged distinctly wherever they appear, and the plain-language line explaining what an emenda is and why the empenhado/pago gap matters.

**Blocked by:** 13 — Search + profile expenses section.

**Status:** ready-for-agent

- [ ] Weekly bulk spreadsheet sync promotes emendas raw → curated, linked to canonical politicians
- [ ] Profile shows yearly empenhado vs pago totals with the plain-language emenda/gap explainer
- [ ] Breakdowns by policy area and by state/municipality render in the locked style
- [ ] Top 5 largest emendas show beneficiary names
- [ ] Emendas PIX are flagged distinctly as a permanent type marker (not an attention badge)
- [ ] Sync-seam tests cover fixture spreadsheet → curated state, including PIX classification
- [ ] HTTP-API-seam tests cover the emendas section payload incl. source + sync date
