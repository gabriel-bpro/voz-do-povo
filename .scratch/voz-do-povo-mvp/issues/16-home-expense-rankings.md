# 16 — Home: expense rankings + house toggle

**What to build:** the home dashboard's rankings block debuts with the two expense rankings: top spenders of the current calendar year and biggest single expenses of the month, each ~10 rows, every row linking to a profile. A single Deputados/Senadores toggle atop the block switches both rankings between houses (quotas are never compared cross-house). Rendered in the locked prototype style: compact tables with bars, source + sync-date line, no editorial content.

**Blocked by:** 13 — Search + profile expenses section; 15 — Senado expenses sync.

**Status:** ready-for-agent

- [x] Home shows top-spenders-of-the-year and biggest-single-expenses-of-the-month rankings, ~10 rows each
- [x] The Deputados/Senadores toggle switches both rankings; the toggle state is coherent across the block
- [x] Every ranking row links to the politician's profile
- [x] Rankings never mix CEAP and CEAPS values in one comparison
- [x] HTTP-API-seam tests cover both rankings, both house selections, and sync-date presence

**Done 2026-09-16.** API: `GET /api/rankings?house=camara|senado` (`internal/httpapi/rankings.go`) — top spenders sum the current calendar year; "of the month" uses the latest month the sync has landed for that house (graceful degradation, never "now"), returned as `month`/`monthYear` so the title states it honestly. Unlinked expenses (`politician_id NULL`) never rank — a row must link to a profile. Row party = most recent expense's party in that house. Web: `web/components/Rankings.tsx`, toggle is plain links (`?casa=senado`, preserves an active `q`) — zero client JS; bars reuse the prototype's `barRows` pattern (`.bar` + `.ranking-grid` in globals.css). No badges on ranking rows (per ticket 08).
