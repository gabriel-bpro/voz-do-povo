# Prototype a plain-language report

Type: prototype
Status: resolved
Blocked by: 03

## Question

What does a plain-language report actually look and read like? Build a cheap, rough prototype of the politician profile and one home-dashboard section using real-shaped data (from the API research), so the tone, granularity, and "explained for everyone" style can be reacted to and locked. The prototype is throwaway; the locked style is the deliverable.

## Answer

_Resolved 2026-09-02 with the dev driving the map, reacting to the prototype._

**Locked style: variant B's dashboard structure + variant A's plain-language explainers.**

- **Structure from B**: stat-card dashboard — big numbers up front (year total, % pago, attendance rate, patrimony), compact tables with bars, house toggle + search bar, per-dataset source + sync-date lines. Numbers should be as self-explanatory as possible.
- **Explanations from A**: plain-language disclaimers/tooltips layered onto the dashboard — the short concept explainers ("o que é cota parlamentar", "emenda empenhada vs paga", "emenda PIX é a mais difícil de rastrear") and the human-scale anchors A used (e.g. "≈ 268 salários mínimos", "mediana da bancada de SP"). Delivered as inline disclaimers/tooltips, not long prose sections.
- Rejected: A's article-length prose as the primary structure (too slow to scan); C's progressive disclosure (an extra click between the citizen and the numbers).

Deliverable locked = this tone/granularity/style. Prototype (all 3 variants) lives at [prototype/plain-language-report.html](../prototype/plain-language-report.html) as the primary source; throwaway.

## Comments

_2026-09-02 — session working the map._ Prototype built at [prototype/plain-language-report.html](../prototype/plain-language-report.html) — open directly in a browser (no server needed). 3 radically different variants of the politician profile + one home-dashboard section (the rankings block), switchable via the floating bottom bar or ←/→ keys; `?variant=A|B|C&tab=home|perfil` is shareable. Fictional politician, real-shaped data per the API research (CEAP categories, empenhado/pago emendas, TSE patrimony cycles, donor detail). Awaiting the dev's reaction to lock tone/granularity/style.
