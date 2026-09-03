# Research the legislative/executive open-data APIs

Type: research
Status: resolved
Blocked by: —

## Question

What do the Câmara dos Deputados, Senado Federal, and Portal da Transparência open-data APIs actually offer for the diligence quartet (expenses, votes, attendance, politician registry)? For each source: relevant endpoints/resources, auth requirements, rate limits, data formats, historical depth, update cadence, and known data-quality gotchas. The data-architecture ticket needs these facts to design the mirror pipeline and per-source sync cadence.

Primary sources:
- https://dadosabertos.camara.leg.br/ (Câmara — CEAP expenses, votes, attendance, deputies)
- https://www12.senado.leg.br/dados-abertos (Senado — quota expenses, votes, senators)
- https://api.portaldatransparencia.gov.br/swagger-ui/index.html#/ (executive; requires API key)

Findings captured in `.scratch/voz-do-povo-mvp/research/legislative-executive-apis.md`.

## Answer

Full findings: [research/legislative-executive-apis.md](../research/legislative-executive-apis.md).

Gist: all four diligence-quartet datasets are covered for both houses, bulk-first. Câmara CEAP expenses go back to 2008, updated daily, as yearly bulk files (which deviate from the API's identifier conventions — a gotcha the Câmara itself documents). Nominal votes (per-deputy and per-senator) are structured and daily. Senado CEAPS is daily via CSV + API. Portal da Transparência is email-token auth with documented rate limits (400/min daytime, 700/min overnight) and adds emendas parlamentares + sanctions registries (CEIS/CNEP/CEAF) for supplier cross-referencing. Sync tiers: expenses/votes daily, sanctions monthly. A canonical politician entity with per-source id mapping is required. Open edges carried forward: Câmara/Senado API rate limits undocumented (measure empirically); Senado attendance has no bulk dataset (derive per-session).
