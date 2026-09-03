# Research: Câmara, Senado, and Portal da Transparência open data

_Resolved from primary sources on 2026-08-27 (this session; the planned research subagents could not be spawned, so the research was done directly). All facts below come from the official portals unless marked otherwise._

## Câmara dos Deputados (dadosabertos.camara.leg.br)

Two access mechanisms, both official:

- **REST API** — documented at `dadosabertos.camara.leg.br/swagger/api.html`, no auth required, JSON/XML. Covers deputados, proposições, votações, eventos, frentes, órgãos, legislaturas.
- **Bulk files** — full datasets as yearly files in CSV/XLSX/ODS/JSON/XML under `dadosabertos.camara.leg.br/arquivos/<dataset>/<formato>/<dataset>-{ano}.{formato}`.

Diligence-quartet coverage:

- **Expenses (CEAP)** — bulk files per year at `www.camara.leg.br/cotas/Ano-{ano}.{csv|json|xml}.zip`, **history back to 2008**, **updated daily**. Field documentation at `dadosabertos.camara.leg.br/howtouse/2023-12-26-dados-ceap.html`. Caveat called out by the Câmara itself: CEAP files **do not follow** the naming/identifier conventions of the other datasets.
- **Votes** — `votacoes-{ano}`, `votacoesVotos-{ano}` (per-deputy vote), `votacoesOrientacoes-{ano}` (party orientation), all daily-updated bulk + API. Gotcha: individual votes only exist for **nominal** votes; symbolic votes have sparse records. Also `votacoesObjetos` lists only *possible* objects of a vote — the actually-voted proposition is often not identified.
- **Attendance** — `eventosPresencaDeputados-{ano}` (per-event presence), daily.
- **Politician registry** — `deputados` bulk file (all deputies in history, civil + parliamentary names, URI identifiers) + `/deputados` API endpoints, daily.

Cadence: everything relevant is **daily**.

## Senado Federal (www12.senado.leg.br/dados-abertos)

Catalog at `/dados-abertos/conjuntos`, split Administrativo/Legislativo. Two API families, both with Swagger:

- **Administrative API** — `adm.senado.gov.br/adm-dadosabertos` (Swagger at `/adm-dadosabertos/swagger-ui/index.html`): CEAPS expenses (`buscarDespesasCeapsPorAno`, also CSV), support offices, housing/per diem benefits. No documented auth.
- **Legislative API** — `legis.senado.leg.br/dadosabertos` (Swagger at `/dadosabertos/api-docs/swagger-ui/index.html`): nominal votes by year/senator/matéria, party-orientation + per-senator votes (`orientacaoBancadaData`), sessions, speeches. Also plain CSV/JSON/XML file endpoints (e.g. `legis.senado.leg.br/dadosabertos/votacao.csv` — last 12 months of a senator's votes).

Diligence-quartet coverage:

- **Expenses (CEAPS)** — CSV + web service, **daily** (last update shown: today, 27/08/2026). Extra datasets worth having: health-assistance expenses (annual 2014–2025 + monthly 2015–2026 CSVs), air tickets outside the quota (monthly), housing allowance/functional properties (daily).
- **Votes** — nominal votes per year/senator/matéria + party orientations, online (real-time).
- **Attendance** — via plenary sessions APIs (per-session data; no single bulk attendance dataset observed).
- **Politician registry** — senators in exercise / by legislature via the Legislative API and `www25.senado.leg.br/web/senadores`.

Cadence: CEAPS daily; legislative data effectively real-time ("online"); secondary benefits monthly/annual.

## Portal da Transparência / CGU (api.portaldatransparencia.gov.br)

- **Auth**: token obtained by **registering an e-mail** at `portaldatransparencia.gov.br/api-de-dados/cadastrar-email`. (Note: this corrects the ticket text — the signup is e-mail-based, not CPF-based.)
- **Rate limits (documented)**: **400 req/min from 06:00–23:59, 700 req/min from 00:00–05:59** (Brasília time).
- **Format**: REST, JSON; Swagger at `api.portaldatransparencia.gov.br`.
- The portal itself recommends **bulk spreadsheet downloads** (`/download-de-dados`) over the API for large volumes.

Relevance to the diligence quartet — this source is the **executive branch**, so it is not the primary source for legislators' own expenses. But it is highly valuable for cross-referencing a politician:

- **Emendas parlamentares** — which amendments a parliamentarian authored and where the money went (endpoint: consulta de emendas + documentos relacionados). A natural "diligence" signal beyond the quartet.
- **Sanções** — CEIS/CNEP/CEAF/CEPIM lookups by CPF/CNPJ: check whether expense suppliers (from CEAP/CEAPS CNPJs) are sanctioned companies.
- **CPGF** (cartão corporativo), **viagens a serviço**, **servidores**, contratos, licitações — executive-side context.

## Implications for the mirror pipeline

- **Bulk-first, API-second**: Câmara bulk files (daily) and Senado CSVs cover the quartet without touching rate limits; APIs fill gaps and do incremental refresh. Portal da Transparência's own guidance says bulk for volume; its 400–700 req/min cap makes API-only mirroring of emendas feasible but slow.
- **Sync cadence tiers confirmed by the sources**: CEAP/CEAPS → daily; votes/sessions → daily (both houses); TSE patrimony/financing → per-election-cycle (see the TSE ticket — access blocked in this session); sanctions registries → monthly is plenty.
- **Identifier strategy needed**: Câmara uses URI ids, Senado uses its own codes, CEAP files deviate from Câmara conventions; a canonical `politician` entity with per-source id mapping is required. (TSE linkage via CPF may be redacted — unresolved, see TSE ticket.)
- **Nice surprise**: vote data from both houses is structured and complete for nominal votes — good enough to power "como votou" profile sections without scraping.

## Known limitations of this research

- Rate limits for the Câmara and Senado APIs are not documented on the pages fetched; assume conservative politeness (or measure empirically once the pipeline exists).
- Senado attendance: no bulk attendance dataset found; likely derivable per-session via API — verify during the data-architecture ticket.
- TSE could not be reached at all this session (see ticket 02).
