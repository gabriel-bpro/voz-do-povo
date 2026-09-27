# 18 — Attention badges (rule 3)

**What to build:** the badge system completes. Rule 3 badges the profile's emendas section when ≥ 50% of the politician's empenhado in the current calendar year is emenda PIX, showing the actual percentage: `Chama atenção · <X>% das emendas empenhadas em <ano> são emendas PIX`. The header rollup count extends to cover all tripped rules.

Sanctions (CEIS/CNEP sync, rule 2 badges) are out of scope for now.

**Blocked by:** 14 — Median attention badge; 17 — Emendas sync + profile emendas section.

**Status:** done

- [x] Emendas section badges when ≥ 50% of current-year empenhado is PIX, showing the actual percentage
- [x] Header rollup count reflects all tripped rules
- [x] HTTP-API-seam tests cover the rule 3 threshold boundary (just below 50%, exactly 50%, above)

**Done 2026-09-27.** Decisions (user, this session): sanctions (CEIS/CNEP, rule 2) descoped — ticket covers rule 3 only. Rollup copy generalized to `N itens chamam atenção` since the count now mixes expenses + the section-level PIX badge; anchor points at #gastos when expenses are flagged, else #emendas. The badge is computed on the current calendar year regardless of `?year=` (copy carries the year), and the payload field is `emendas.badge = {pixPercent, year} | null`.
