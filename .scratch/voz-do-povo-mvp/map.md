# Map: Voz do Povo — MVP spec

## Destination

A spec for the MVP of Voz do Povo: a federal-only, citizen-first web app that explains Brazilian federal politicians in plain language — a national dashboard home (top expenses, rankings), politician profiles, all in plain language via templated components (no LLM prose in the MVP). Done when someone can start building with no open decisions.

## Notes

- Domain: Brazilian federal open data (Câmara dos Deputados, Senado Federal, Portal da Transparência, TSE). Product language is pt-BR; repo and code language is English.
- Every session: `/grilling` + `/domain-modeling` by default; `/research` for research tickets; `/prototype` for the report prototype. Glossary lives in `CONTEXT.md`.
- Standing preferences (settled while charting; revisit only with cause):
  - Plain language is **template-only in the MVP**: numbers and prose both come from templates/structured components traceable to source data; no LLM prose in the MVP (walkthroughs dropped 2026-09-02; LLM debuts post-MVP in "Follow your politician").
  - We **mirror** gov data into our own store; per-source sync cadence and cache TTL per source churn.
  - MVP datasets = the **diligence quartet**: expenses, votes/attendance, declared patrimony, campaign financing.
  - Budget posture: side-project (~R$0–100/month), flexible upward if value is shown. MVP hosting target: **Railway**.
  - Stack: Next + **Go (API + worker; switched from Nest 2026-09-03)**, Postgres, Railway (Hobby plan).

## Decisions so far

- [Research the legislative/executive open-data APIs](issues/01-research-legislative-executive-apis.md) — the quartet is fully covered for both houses, bulk-first (Câmara CEAP since 2008, daily; Senado CEAPS daily; nominal votes structured for both); Portal da Transparência adds emendas + sanctions with documented rate limits; a canonical politician entity with per-source id mapping is required. Detail: [research/legislative-executive-apis.md](research/legislative-executive-apis.md).
- [Research TSE open data: patrimony and campaign financing](issues/02-research-tse-data.md) — TSE is not directly syncable (Akamai 403 from datacenters), but the Base dos Dados mirror covers the full TSE half of the quartet programmatically (BigQuery, free tier, refreshed mid-2026-campaign); official manual bulk download is the fallback. Detail: [research/tse-data.md](research/tse-data.md).
- [Define the MVP surfaces](issues/03-mvp-surface-spec.md) — home = search + 5 rankings (per-house expense totals/single items, emendas PIX, attendance laggards, patrimonies) under a house toggle; profile = identity, expenses, emendas (PIX flagged), votes & attendance, patrimony evolution, campaign financing; the NL box was dropped in the pivot (ticket 05); attention badge is in the MVP with fixed transparent rules.
- [Design the data architecture (mirror, sync, search)](issues/04-data-architecture.md) — Postgres only (Elasticsearch dropped); worker service on Railway runs syncs (**Go**, switched from Nest 2026-09-03); raw+curated layers with unified per-dataset tables around a canonical `politician` entity; history = current term + 2022/2026 TSE cycles; identity links CPF-first with confidence-gated auto-linking; cadence = daily API delta + weekly bulk reconcile for expenses, monthly BD/BigQuery for TSE, weekly bulk emendas, monthly bulk sanctions; app queries curated tables on demand; sync failures = logs only. (The "NL engine = LLM SQL" decision was voided by the pivot in ticket 05.)
- [Choose the LLM strategy](issues/05-llm-strategy.md) — **two pivots**: free-form NL querying dropped; then walkthroughs dropped too — **the MVP has zero LLM calls** (data + templates only). LLM strategy (Kimi K3, dev-paid key, precompute, figure guardrails) parks with the **post-MVP "Follow your politician" feature** (LLM-written e-mail alerts on data changes).
- [Estimate the real Railway monthly cost](issues/10-railway-cost-estimate.md) — Hobby plan ($5/mo, usage credit included) → **~$7–15/mo ≈ R$45–90/month all-in**; Hobby's 5GB Postgres volume cap is the Pro-trigger; private networking required to avoid egress billing. Detail: [research/railway-cost.md](research/railway-cost.md).
- [Prototype a plain-language report](issues/06-plain-language-report-prototype.md) — **style locked: dashboard structure + plain-language explainers** — stat cards/big numbers/bars (self-explanatory numbers) with short inline disclaimers/tooltips for concepts (cota, empenhado vs pago, emenda PIX) and human-scale anchors ("≈ 268 salários mínimos", bancada median); article-length prose and progressive disclosure rejected. Primary source: [prototype/plain-language-report.html](prototype/plain-language-report.html).
- [Define the attention-badge rules](issues/08-attention-badge-rules.md) — **badge = neutral context, not editorial judgment**: 3 fixed rules, item-level, expenses + emendas only — (1) expense above category median (house × category, trailing 12 months, ≥ R$ 1.000 floor, ≥ 30-item buckets, glosas excluded); (2) counterparty in CEIS/CNEP by exact CNPJ/CPF, no floor; (3) ≥ 50% of the year's empenhado in emenda PIX. Copy shows the bare number + sync date; concepts explained in tooltips.
- [Set up GCP billing project for Base dos Dados BigQuery](issues/09-gcp-billing-project.md) — GCP project + billing + service account (`bigquery.dataViewer` + `bigquery.jobUser`) done, JSON key downloaded; credentials convention locked: full key in Railway env var **`GCP_SERVICE_ACCOUNT_JSON`** on the worker service, set at provisioning time (Railway not yet provisioned).
- [Câmara expenses sync + core schema](issues/12-camara-expenses-sync.md) — curated core landed (politician, source-id map, party, unified expense, identity review queue); CEAP syncs from the **daily bulk file only** (API despesas endpoint dead upstream), dedup by full-row hash (CEAP has no row-unique id), identity links CPF-first with fuzzy → review queue; raw layer is replace-per-source-key staging; `worker bootstrap-camara` backfills 2023+.

## Not yet specified

_Empty — the frontier is closed (2026-09-03)._

## Out of scope

- **Ops beyond logs** (sync_runs tracking, stale-data detection, data-quality monitoring) — post-MVP: [Design the data architecture](issues/04-data-architecture.md) decided logs-only for the MVP; moved here from the fog when the map closed.

- **Free-form NL querying** — pivoted out on 2026-09-02 via [Choose the LLM strategy](issues/05-llm-strategy.md).
- **"Follow your politician" (citizen e-mail alerts on data changes, LLM-written body)** — dev-designed on 2026-09-02 ([ticket 05](issues/05-llm-strategy.md)) but ruled **post-MVP**; chart as a fresh effort after the MVP ships. This consciously re-admits what "Monitoring/alerts for citizens" below ruled out — as the next effort, not this one.
- [Obtain a Portal da Transparência API key](issues/07-portal-transparencia-api-key.md) — ruled out by the data-architecture decision: emendas and sanctions both sync via bulk spreadsheet downloads, which need no API token.
- State and municipal coverage — the destination is federal-only; architecture must not foreclose expansion, but no tickets plan for it.
- Monitoring/alerts for citizens and journalist/researcher tooling (exports, deep dives) — citizen-first MVP only.
- Authored-proposal texts and sanctions/TCU datasets — deferred when the diligence quartet was chosen.
- User accounts and personalization — no alerts in the MVP, so no accounts.
- Party-level campaign financing view — candidate-level receitas already capture party donations and BD's party-level tables lag; ruled out when the MVP surfaces were defined.