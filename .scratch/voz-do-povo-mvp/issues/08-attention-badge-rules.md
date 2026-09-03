# Define the attention-badge rules

Type: grilling
Status: resolved
Blocked by: 04

## Question

What exactly is the small fixed set of transparent rules that trips the "chama atenção" badge on expenses, emendas, and other items? Candidates surfaced in the surface spec: 3+× the category median; supplier or emenda beneficiary flagged in CGU sanctions (CEIS/CNEP); emenda PIX concentration. For each rule: the precise threshold, the population it's computed over (per house? per expense category? per state?), and the plain-language explanation shown when it trips. Rules must be fixed, explainable, and cheap to compute over the mirrored store — blocked on the data architecture so we know what aggregations are available.

## Answer

_Resolved 2026-09-03 via grilling (3 rounds) with the dev driving the map._

**Badge character: neutral context, not editorial judgment.** The badge shows the underlying number (median, percentage, list membership + sync date) and lets the citizen draw the conclusion — no multipliers, no adjectives. Consistent with the plain-language, no-editorializing stance locked in the report prototype.

**Scope: expenses and emendas only, item-level.** Rankings already highlight politician-level extremes; campaign-donor sanctions and patrimony rules are deferred (TSE depth is only 2022/2026, so patrimony-change rules would rarely fire).

**The fixed rule set — exactly 3 rules:**

1. **Expense above category median.** Trips when a single expense item is **above the median** of its bucket (house × expense category, trailing 12 months) **and ≥ R$ 1.000** (absolute floor — trivial values never badge). Buckets with **< 30 items** have no median and never badge (too unstable). Negative values (glosas/ressarcimentos) are excluded from both the median and the badge. Medians are computed on demand over curated tables — no precomputation in the MVP.
   - Copy: `Chama atenção · Acima da mediana da categoria "Passagens aéreas" (mediana: R$ 2.140)`
2. **Sanctioned counterparty.** Trips when an expense supplier or emenda beneficiary matches **CEIS or CNEP by exact CNPJ/CPF** (never fuzzy — same defamation-grade standard as identity linkage), per the latest monthly sanctions sync. **No value floor** — a sanctioned counterparty matters at any amount. Campaign donors deferred.
   - Copy: `Chama atenção · Fornecedor consta em cadastro oficial de empresas sancionadas (consulta de ago/2026)` (emenda variant: `Beneficiário consta em …`)
   - Tooltip: `CEIS e CNEP são listas do Governo Federal de empresas punidas por irregularidades como fraude em licitações ou corrupção. Consultamos essas listas mensalmente.`
3. **Emenda PIX concentration.** Trips on the profile's emendas section when **≥ 50%** of the politician's empenhado in the current calendar year is emenda PIX. The per-item PIX flag (ticket 03) stays a separate, permanent type marker.
   - Copy: `Chama atenção · 63% das emendas empenhadas em 2026 são emendas PIX` (shows the actual percentage)

**Rendering.** Badge renders inline on the item (or section, for PIX concentration), following the locked prototype style: plain factual line on the surface, concepts explained in tooltips. Profile header shows a rollup count (`N despesas chamam atenção`) linking to the expenses section. **No badge on home ranking rows** — rankings are already the highlight.

**Test every rule passes:** explainable in one pt-BR sentence, fixed threshold, cheap SQL aggregate over curated tables.
