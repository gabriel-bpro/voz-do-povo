# 16 — Home: expense rankings + house toggle

**What to build:** the home dashboard's rankings block debuts with the two expense rankings: top spenders of the current calendar year and biggest single expenses of the month, each ~10 rows, every row linking to a profile. A single Deputados/Senadores toggle atop the block switches both rankings between houses (quotas are never compared cross-house). Rendered in the locked prototype style: compact tables with bars, source + sync-date line, no editorial content.

**Blocked by:** 13 — Search + profile expenses section; 15 — Senado expenses sync.

**Status:** ready-for-agent

- [ ] Home shows top-spenders-of-the-year and biggest-single-expenses-of-the-month rankings, ~10 rows each
- [ ] The Deputados/Senadores toggle switches both rankings; the toggle state is coherent across the block
- [ ] Every ranking row links to the politician's profile
- [ ] Rankings never mix CEAP and CEAPS values in one comparison
- [ ] HTTP-API-seam tests cover both rankings, both house selections, and sync-date presence
