# Spec: Voz do Povo — MVP

Status: ready-for-agent

Synthesized 2026-09-03 from the wayfinder map ([map.md](map.md)) and its ten resolved tickets. Every decision below traces to a ticket; the map is the index, this document is the buildable contract.

## Problem Statement

A Brazilian citizen who wants to understand or investigate the federal politicians they elected faces scattered, jargon-heavy, hard-to-trace data: expenses live in two different house portals (CEAP/CEAPS), emendas parlamentares in the Portal da Transparência, patrimony and campaign financing in TSE dumps that resist programmatic access. Nothing ties it together per politician, nothing explains it in plain language, and the least traceable money (emendas PIX) is effectively invisible. The result: diligence on an elected representative is expert work, not citizen work.

## Solution

A federal-only, citizen-first web app — **Voz do Povo** — with two surfaces:

1. **Home dashboard**: politician search plus five national rankings (per-house expense totals, per-house biggest single expenses, maiores emendas PIX, attendance laggards, largest declared patrimonies) under a Deputados/Senadores toggle.
2. **Politician profile**: one page assembling the **diligence quartet** — expenses, emendas, votes & attendance, declared patrimony, campaign financing — rendered in plain language from templates and structured components (stat cards, big numbers, bars), with inline tooltips explaining concepts (cota, empenhado vs pago, emenda PIX) and human-scale anchors ("≈ 268 salários mínimos").

An **attention badge** ("chama atenção") marks items that trip one of three fixed, transparent rules — shown as neutral context (the bare median, percentage, or list membership + sync date), never editorialized. Zero LLM calls, zero user accounts. All data comes from our own **mirror** of government open data; the app never calls gov APIs at request time.

## User Stories

### Home dashboard

1. As a citizen, I want to search for a federal politician by name, so that I can go directly to the person I elected.
2. As a citizen, I want a one-line explainer of the project on the home page, so that I understand what I'm looking at.
3. As a citizen, I want to see the top spenders of the current calendar year per house, so that I know who spends the most quota money.
4. As a citizen, I want to see the biggest single expenses of the month per house, so that I catch striking individual items that totals hide.
5. As a citizen, I want a unified ranking of the maiores emendas PIX do ano, so that the least traceable federal money is visible.
6. As a citizen, I want a ranking of attendance laggards, so that I can see who isn't showing up to work.
7. As a citizen, I want a ranking of the largest declared patrimonies, so that I can see who the wealthiest federal politicians are.
8. As a citizen, I want a single Deputados/Senadores toggle atop the rankings block, so that I can switch the per-house rankings without leaving the page.
9. As a citizen, I want every ranking row to link to that politician's profile, so that any striking row is one click from the full picture.
10. As a citizen, I want each ranking to show about 10 rows, so that the home page stays scannable.

### Politician profile

11. As a citizen, I want an identity header with a one-paragraph plain-language summary rendered from templates, so that I immediately know who this person is and what their numbers look like.
12. As a citizen, I want the politician's expense total for the current calendar year with a year selector covering the current term, so that I can compare across years.
13. As a citizen, I want a monthly expense trend, so that I can spot unusual months.
14. As a citizen, I want the largest individual expense items listed, so that I can inspect the most significant spending.
15. As a citizen, I want expenses above the category median to carry an attention badge showing the median itself, so that I can judge for myself how unusual the amount is.
16. As a citizen, I want an expense with a sanctioned supplier to carry an attention badge saying so in plain language, so that I know the counterparty is on a government sanctions list.
17. As a citizen, I want a tooltip on the sanctions badge explaining what CEIS/CNEP are, so that I understand the badge without insider knowledge.
18. As a citizen, I want emenda totals by year split empenhado vs pago, so that I can see the gap between announced and actually-delivered money.
19. As a citizen, I want emendas broken down by policy area and by state/municipality, so that I can see where the politician directs federal money.
20. As a citizen, I want the top 5 largest emendas with beneficiary names, so that the biggest transfers are individually inspectable.
21. As a citizen, I want emendas PIX flagged distinctly wherever emendas appear, so that the hardest-to-trace money is always recognizable.
22. As a citizen, I want a plain-language line explaining what an emenda is and why the empenhado/pago gap matters, so that I can interpret the section without prior knowledge.
23. As a citizen, I want an attention badge when most of a politician's empenhado is emenda PIX, showing the actual percentage, so that concentration in untraceable transfers is visible.
24. As a citizen, I want the ~20 most recent nominal votes with party-orientation comparison, so that I can see how the politician votes relative to their party.
25. As a citizen, I want the attendance rate for the current legislature, so that I can judge presence at a glance.
26. As a citizen, I want the declared patrimony total plus its evolution across the 2022 and 2026 election cycles, so that I can see declared wealth and its trajectory.
27. As a citizen, I want the most recent campaign financing cycle — total raised and top donors — so that I know who funded the campaign.
28. As a citizen, I want tooltips/inline disclaimers explaining concepts like cota parlamentar and emenda PIX, so that the page teaches me as I read.
29. As a citizen, I want human-scale anchors next to big numbers (e.g. "≈ 268 salários mínimos"), so that abstract amounts become graspable.
30. As a citizen, I want a rollup count on the profile header of how many items "chamam atenção", linking to the expenses section, so that I know there's flagged content before scrolling.
31. As a citizen, I want each dataset section to show its source and last sync date, so that I can judge freshness and verify at the origin.
32. As a citizen, I want an honest "not linked" state when a source couldn't be confidently matched to the politician, so that missing data is never silently hidden or falsely merged.

### Trust & language

33. As a citizen, I want every badge to be explainable in one sentence with the exact rule behind it, so that I can trust the flag as fact, not opinion.
34. As a citizen, I want the whole product in pt-BR plain language, so that no insider jargon stands between me and the data.
35. As a citizen, I want the site to keep rendering the last synced data even when a sync fails, so that the app degrades gracefully instead of breaking.

## Implementation Decisions

### Topology & stack

- **Railway, 4 units**: Next.js web service, **Go API service**, **Go worker service** (same Go codebase as the API, deployed separately, one-shot sync run on a Railway cron schedule), Railway Postgres. (Backend switched from Nest to Go on 2026-09-03.)
- **Private networking between services is mandatory** — public TCP proxy egress is billed.
- Hobby plan assumed; **5GB Postgres volume cap is the known Pro-trigger** (crossing forces the $20/mo plan, not higher usage pricing). Shallow history keeps the store in the low GB range.

### Mirror & sync

- **Two layers**: raw staging (source data landed verbatim, with provenance: source + pulled-at; never read by the app) and curated (cleaned, unified tables promoted from raw; the only schema the app queries). Cleaning bugs are re-runnable; bad parses never reach what the app serves.
- **Per-source cadence**:
  - CEAP/CEAPS expenses: daily bulk re-ingest of the current-year file (updated daily upstream; doubles as freshness + ground-truth reconciliation). _Amended at ticket 12 implementation (2026-09-08): the Câmara API's per-deputy despesas endpoint returns empty for every deputy — verified live — so the planned "daily API delta" doesn't exist; the daily bulk covers it._
  - Votes (both houses): daily API delta.
  - Attendance: Câmara via daily bulk (`eventosPresencaDeputados`); Senado derived daily by walking recent plenary sessions (polite polling — no bulk dataset exists).
  - TSE (candidatos, bens, receitas, despesas) via **Base dos Dados BigQuery** (`basedosdados.br_tse_eleicoes`): monthly, weekly during campaign season, gated on BD's `updates.latest` metadata. Go BigQuery client (`cloud.google.com/go/bigquery`) in the worker.
  - Emendas (Portal da Transparência): weekly bulk spreadsheet — no API key.
  - Sanctions (CEIS/CNEP): monthly bulk spreadsheet — no API key.
- **Sync failures: logs only** (stdout on Railway). The app always renders the last good sync, with sync dates visible per dataset.
- **Historical depth**: legislative data 2023+ (current term); TSE 2022 and 2026 cycles only. Expansion later is a resync of older years into the same pipeline, not a migration.

### Curated schema & identity

- **Canonical `politician` entity** with a `politician_source_id` map (Câmara URI id, Senado code, TSE sequencial/CPF/título eleitoral). All cross-source joins go through it.
- **Unified per-dataset tables**: one `expense` table with house as a column (CEAP/CEAPS quotas are never mixed in comparisons), one `vote`, one `attendance_event`, emendas, patrimony declarations, campaign receitas/despesas. A normalized `party` dimension absorbs sigla drift across years.
- **Identity linkage: CPF-first, deterministic fallback, confidence-gated.** Exact identifier match (CPF where populated; título eleitoral secondary) → auto-link. Fuzzy name+UF+cargo → manual review queue (~594 current members, tiny volume). Non-matches stay unlinked and profiles render per-source data with the honest "not linked" state. A false link is a defamation-grade bug; a missed link is just missing data.

### API contract (serving layer)

- The app queries curated tables **on demand** (no materialized rollups in the MVP; optimize when perf hurts).
- Endpoints: politician search (Postgres trigram/full-text over names), home rankings (per-house parameter where applicable), politician profile (per-section data). Every response carrying source data includes the **source name and last sync date** for that dataset.
- Expense items, emendas, and the emendas section carry a **badge field**: which rule tripped plus the values needed to render the neutral copy (median + category for rule 1; list + sync date for rule 2; percentage + year for rule 3). Absent field = no badge.
- The profile header payload includes the **rollup count** of flagged expenses.

### Attention badge — the fixed rule set (exactly 3)

1. **Expense above category median**: single expense item above the median of its bucket (house × expense category, trailing 12 months) **and ≥ R$ 1.000**. Buckets with < 30 items never badge. Negative values (glosas/ressarcimentos) excluded from median and badge. Copy: `Chama atenção · Acima da mediana da categoria "<categoria>" (mediana: R$ <valor>)`.
2. **Sanctioned counterparty**: expense supplier or emenda beneficiary matches CEIS or CNEP by **exact CNPJ/CPF** (never fuzzy), per the latest monthly sync. No value floor. Copy: `Chama atenção · Fornecedor consta em cadastro oficial de empresas sancionadas (consulta de <mês/ano>)` (emenda variant: `Beneficiário consta em …`). Tooltip: `CEIS e CNEP são listas do Governo Federal de empresas punidas por irregularidades como fraude em licitações ou corrupção. Consultamos essas listas mensalmente.`
3. **Emenda PIX concentration**: ≥ 50% of the politician's empenhado in the current calendar year is emenda PIX; badges the emendas section, showing the actual percentage. Copy: `Chama atenção · <X>% das emendas empenhadas em <ano> são emendas PIX`. The per-item emenda PIX flag is a separate, permanent type marker, not a badge.

Badge character: **neutral context, not editorial judgment** — the bare number, no multipliers, no adjectives. Every rule must be explainable in one pt-BR sentence and computable as a cheap SQL aggregate over curated tables. No badge on home ranking rows.

### Plain language & presentation

- **Template-only**: all numbers and prose come from templates/structured components traceable to source data. **Zero LLM calls.**
- **Locked style** (prototype, ticket 06): stat-card dashboard — big numbers up front, compact tables with bars, house toggle + search bar, per-dataset source + sync-date lines — plus short inline disclaimers/tooltips for concepts and human-scale anchors. Article-length prose and progressive disclosure are rejected. Primary style source: [prototype/plain-language-report.html](prototype/plain-language-report.html) (throwaway artifact; the *style* is the deliverable).
- Product language **pt-BR**; repo and code language English.

### Home dashboard content

Search bar front and center; five rankings (~10 rows each, every row linking to a profile): top spenders of the current calendar year (per house), biggest single expenses of the month (per house), maiores emendas PIX do ano (unified), attendance laggards, largest declared patrimonies (unified). One Deputados/Senadores toggle switches the per-house rankings. One-line project explainer. No editorial content, no charts beyond simple bars.

### Profile content (in order)

1. Identity header + templated plain-language summary + flagged-items rollup count.
2. Expenses — current calendar year total (year selector within the current term), monthly trend, largest items, badges per rules 1–2.
3. Emendas — totals by year empenhado vs pago; breakdown by policy area and state/municipality; top 5 largest with beneficiary; emendas PIX flagged distinctly; PIX-concentration badge (rule 3); plain-language emenda/empenhado-vs-pago explainer.
4. Votes & attendance — ~20 most recent nominal votes with party-orientation comparison; attendance rate for the current legislature.
5. Declared patrimony — total plus evolution across the 2022/2026 cycles (max 2 points; accepted as thin).
6. Campaign financing — most recent cycle: total raised, top donors.

## Testing Decisions

- **What makes a good test**: it drives external behavior only — HTTP requests against the API, source payloads into a sync — and asserts observable outcomes (responses, table state). No tests of internal functions, ORM mappings, or rendering internals.
- **Seam 1 — the HTTP API** (Go API ↔ Postgres): tests run the API over real HTTP against a real test Postgres **seeded directly into the curated layer** with fixtures. This single seam covers home rankings, profile sections, search, the attention-badge fields (all three rules are SQL over curated tables), pt-BR template strings and numbers in responses, and source/sync-date presence.
- **Seam 2 — the sync entry points** (worker ↔ source fixtures): tests feed recorded source payloads — Câmara/Senado CSVs and API responses, Portal da Transparência spreadsheets, BigQuery result rows — into each sync's entry point and assert resulting raw + curated table state, including provenance, identity-linking outcomes (auto-link vs review queue vs unlinked), and emenda PIX flagging. Source adapters never touch the network in tests.
- **No frontend test layer**: the Next app only renders API responses in the locked style; type-checking plus the API contract is its safety net (dev's call, 2026-09-03).
- **Prior art**: none — greenfield repo. These two seams establish the testing conventions for the codebase; keep them as the only seams.

## Out of Scope

- **Free-form NL querying** and **all LLM usage** — pivoted out 2026-09-02; the MVP has zero LLM calls.
- **"Follow your politician"** (citizen e-mail alerts, LLM-written) — designed but ruled **post-MVP**; chart as a fresh effort after the MVP ships.
- **State and municipal coverage** — federal-only; the architecture must not foreclose expansion but nothing builds toward it.
- **Monitoring/alerts for citizens** and **journalist/researcher tooling** (exports, deep dives) — citizen-first MVP only.
- **User accounts and personalization** — no alerts, so no accounts.
- **Authored-proposal texts and sanctions/TCU datasets beyond CEIS/CNEP** (CEAF, CEPIM) — deferred.
- **Party-level campaign financing view** — candidate-level receitas already capture party donations; BD's party-level tables lag.
- **Ops beyond logs** (sync_runs tracking, stale-data detection, data-quality monitoring) — logs-only for the MVP.
- **Portal da Transparência API key** — emendas and sanctions sync via bulk spreadsheets, no token needed.
- **Legislative history before 2023 and TSE cycles before 2022** — expansion is a later resync, not part of this spec.
- **Campaign-donor sanction badges and patrimony-change badges** — deferred (TSE depth makes the latter near-dead).
- **Frontend tests** — see Testing Decisions.

## Further Notes

- **Reconciliation**: the surface spec (ticket 03) originally described an expense year selector "back to 2008" and patrimony "since 2006"; the data architecture (ticket 04, decided a day later) set history to the current term + 2022/2026 TSE cycles. This spec follows ticket 04.
- **Credentials**: GCP service-account JSON key (BigQuery `dataViewer` + `jobUser`) is downloaded and held locally by the dev; it becomes the Railway env var **`GCP_SERVICE_ACCOUNT_JSON`** on the worker service at provisioning time. Railway is not yet provisioned.
- **Budget**: estimated **~R$45–90/month all-in** on the Hobby plan ([research/railway-cost.md](research/railway-cost.md)).
- **TSE direct access is blocked** (Akamai 403 from datacenters); Base dos Dados is the sync path, with official manual bulk download as the freshness fallback ([research/tse-data.md](research/tse-data.md)).
- **Known data gotchas absorbed by design**: Câmara bulk files deviate from the API's identifier conventions; TSE per-cycle schema drift, `#NULO#` sentinels, decimal commas, and unpadded CPFs are absorbed by using BD's cleaned tables; Câmara/Senado API rate limits are undocumented — measure empirically during implementation.
- **BigQuery cost**: TSE slices are a few GB against the 1 TB/month free tier; effectively free.
