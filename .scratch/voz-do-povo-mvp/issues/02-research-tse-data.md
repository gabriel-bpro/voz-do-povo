# Research TSE open data: patrimony and campaign financing

Type: research
Status: resolved
Blocked by: —

## Question

What does the TSE (Tribunal Superior Eleitoral) expose for the TSE half of the diligence quartet — declared patrimony (declaração de bens) and campaign financing (prestação de contas) — plus the candidate registry that ties them to a politician? For each: access mechanism (bulk CSV dumps vs API), formats, historical depth (which election years), update cadence across the election cycle, data volume, and known data-quality gotchas (e.g. CPF/party inconsistencies across years).

Primary source: https://dadosabertos.tse.jus.br/

Findings captured in `.scratch/voz-do-povo-mvp/research/tse-data.md`.

## Answer

Full findings: [research/tse-data.md](../research/tse-data.md).

Gist: the TSE half of the quartet is fully available **programmatically via the Base dos Dados mirror** (BigQuery dataset `basedosdados.br_tse_eleicoes`): `candidatos` (3.4M rows, 1994–2026), `bens_candidato` (5.1M rows, 2006–2026), `receitas_candidato` (15.3M rows, 2002–2024), `despesas_candidato` (32.4M rows, 2002–2024) — all free, refreshed 2026-08-12 (mid-2026-campaign), declared cadence 2/year, ~21 GB total (inside BigQuery's 1 TB/month free tier). Direct TSE sync is **not** automatable from a datacenter: all TSE properties return Akamai-edge HTTP 403 (IP-reputation blocking, re-verified with browser headers). Official path is manual/residential-network download of stable bulk-zip URLs (`consulta_cand_<ano>.zip`, `prestacao_contas...zip`), kept as a freshness fallback. Join keys: `sequencial`+ano+UF internally, plus `cpf`/`titulo_eleitoral`; donor/supplier CPF/CNPJ columns bridge directly to CGU sanctions checks. BD's cleaning code documents the official files' gotchas (per-cycle schema drift, per-UF splits, `#NULO#` sentinels, decimal commas, unpadded CPFs) and absorbs them. Open edges carried forward: exact official catalog names/cadence unverifiable while TSE 403s (verify from a non-blocked network); BD's party-level financing tables lag (`receitas_comite` ends 2014, `receitas_orgao_partidario` ends 2022) — relevant only if party-level money is in MVP scope.

## Comments

**2026-08-27 — access blocked, ticket returned to the frontier.** All TSE properties (dadosabertos.tse.jus.br, cdn.tse.jus.br, www.tse.jus.br/eleicoes/estatisticas) return HTTP 403 to automated fetches — bot protection. This is itself a decision-relevant fact for the data-architecture ticket: TSE syncing likely cannot be naive HTTP from a datacenter; alternatives to evaluate when this ticket is re-taken: (a) a human-assisted/manual bulk download step, (b) fetching via a mirror such as Base dos Dados (basedosdados.org mirrors TSE datasets — verify freshness), (c) retry from a residential network. Next session should attempt (b)/(c) or escalate to HITL (a).
