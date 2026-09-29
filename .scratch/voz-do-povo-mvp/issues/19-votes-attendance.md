# 19 — Votes & attendance sync + profile section

**What to build:** the third leg of the quartet. The worker syncs nominal votes for both houses (API delta covering the days since the previous run) and attendance — Câmara via the `eventosPresencaDeputados` bulk (updated daily upstream), Senado derived by walking recent plenary sessions with polite polling (no bulk dataset exists). All jobs run inside the single weekly cron run (Monday 06:00 UTC). The profile gains the votes & attendance section: the ~20 most recent nominal votes with party-orientation comparison (how the politician voted vs their party's orientation) and the attendance rate for the current legislature, in the locked style with source + sync dates.

**Blocked by:** 13 — Search + profile expenses section. _(done 2026-09-13 — unblocked)_

**Status:** ready-for-agent

- [ ] Nominal votes for both houses sync into curated on the weekly cron run, linked to canonical politicians
- [ ] Câmara attendance syncs from the bulk; Senado attendance derives from the sessions walk (both weekly)
- [ ] Profile shows ~20 most recent nominal votes with party-orientation comparison
- [ ] Profile shows the current-legislature attendance rate
- [ ] Sync-seam tests cover vote fixtures and both attendance paths (bulk + derived)
- [ ] HTTP-API-seam tests cover the section payload incl. source + sync dates
