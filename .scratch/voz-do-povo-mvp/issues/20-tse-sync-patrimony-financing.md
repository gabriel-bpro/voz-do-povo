# 20 — TSE sync + patrimony & campaign financing sections

**What to build:** the TSE half of the quartet via Base dos Dados. The worker pulls `candidatos`, `bens_candidato`, `receitas_candidato`, and `despesas_candidato` from `basedosdados.br_tse_eleicoes` (BigQuery, Go client, `GCP_SERVICE_ACCOUNT_JSON`), monthly and weekly during campaign season, gated on BD's `updates.latest` metadata. 2022 and 2026 cycles only. Identity linking uses CPF/título eleitoral through the existing confidence-gated mechanism. The profile gains its final two sections: declared patrimony (total plus evolution across the 2022/2026 cycles — thin at max 2 points, accepted) and campaign financing (most recent cycle: total raised, top donors), in the locked style with source + sync dates.

**Blocked by:** 13 — Search + profile expenses section.

**Status:** ready-for-agent

- [ ] BigQuery pulls run on the metadata-gated cadence and land raw → curated
- [ ] TSE records link to canonical politicians via CPF/título through the confidence-gated mechanism (unmatched stay honestly unlinked)
- [ ] Profile shows declared patrimony total + 2022/2026 evolution
- [ ] Profile shows most recent campaign cycle: total raised and top donors
- [ ] Sync-seam tests cover fixture BigQuery result sets → curated state, including a TSE identity-link case
- [ ] HTTP-API-seam tests cover both section payloads incl. source + sync dates
