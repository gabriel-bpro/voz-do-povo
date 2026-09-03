# 14 — Median attention badge (rule 1) + badge system

**What to build:** the attention-badge system ships with its first rule. API responses carry a badge field on expense items (absent = no badge); the profile renders the badge inline on flagged items with the locked neutral copy — `Chama atenção · Acima da mediana da categoria "<categoria>" (mediana: R$ <valor>)` — showing the bare median, never a multiplier. Rule: an expense item badges when it is above the median of its bucket (house × expense category, trailing 12 months) and ≥ R$ 1.000; buckets with < 30 items never badge; negative values (glosas/ressarcimentos) are excluded from both median and badge. Medians are computed on demand over curated tables. The profile header gains the rollup count (`N despesas chamam atenção`) linking to the expenses section.

**Blocked by:** 13 — Search + profile expenses section.

**Status:** ready-for-agent

- [ ] Expense items above their house × category trailing-12-month median and ≥ R$ 1.000 carry the badge field
- [ ] Buckets with < 30 items produce no badges; negative-value items are excluded from medians and badges
- [ ] Badge renders inline with the exact neutral copy (category name + median value), no editorializing
- [ ] Profile header shows the flagged-expenses rollup count linking to the expenses section
- [ ] HTTP-API-seam tests cover: badge tripping, the R$ 1.000 floor, the 30-item bucket minimum, and negative-value exclusion
