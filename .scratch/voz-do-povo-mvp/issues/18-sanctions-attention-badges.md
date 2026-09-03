# 18 — Sanctions sync + attention badges (rules 2–3)

**What to build:** the badge system completes. The worker syncs CEIS/CNEP monthly bulk spreadsheets (no API key) into curated. Rule 2 badges: an expense supplier or emenda beneficiary matching CEIS or CNEP by exact CNPJ/CPF (never fuzzy) carries the badge with the locked copy — `Chama atenção · Fornecedor consta em cadastro oficial de empresas sancionadas (consulta de <mês/ano>)` (emenda variant: `Beneficiário consta em …`) — plus the tooltip explaining CEIS/CNEP in plain language and noting the monthly cadence. No value floor. Rule 3 badges the profile's emendas section when ≥ 50% of the politician's empenhado in the current calendar year is emenda PIX, showing the actual percentage: `Chama atenção · <X>% das emendas empenhadas em <ano> são emendas PIX`. The header rollup count extends to cover all tripped rules.

**Blocked by:** 14 — Median attention badge; 17 — Emendas sync + profile emendas section.

**Status:** ready-for-agent

- [ ] CEIS/CNEP monthly bulk sync lands in curated with sync-date provenance
- [ ] Expense suppliers and emenda beneficiaries matching by exact CNPJ/CPF carry the badge; fuzzy matches never badge
- [ ] Badge copy and CEIS/CNEP tooltip render exactly as locked (plain language, consultation date)
- [ ] Sanctioned counterparties badge at any value (no floor)
- [ ] Emendas section badges when ≥ 50% of current-year empenhado is PIX, showing the actual percentage
- [ ] Header rollup count reflects all tripped rules
- [ ] HTTP-API-seam tests cover rule 2 (both counterparties), exact-match-only behavior, and the rule 3 threshold boundary
