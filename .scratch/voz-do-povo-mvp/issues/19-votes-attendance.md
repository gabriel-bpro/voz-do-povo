# 19 — Votes & attendance sync + profile section

**What to build:** the third leg of the quartet. The worker syncs nominal votes for both houses (daily API delta) and attendance — Câmara via the daily `eventosPresencaDeputados` bulk, Senado derived daily by walking recent plenary sessions with polite polling (no bulk dataset exists). The profile gains the votes & attendance section: the ~20 most recent nominal votes with party-orientation comparison (how the politician voted vs their party's orientation) and the attendance rate for the current legislature, in the locked style with source + sync dates.

**Blocked by:** 13 — Search + profile expenses section.

**Status:** ready-for-agent

- [ ] Nominal votes for both houses sync daily into curated, linked to canonical politicians
- [ ] Câmara attendance syncs from the daily bulk; Senado attendance derives from the sessions walk
- [ ] Profile shows ~20 most recent nominal votes with party-orientation comparison
- [ ] Profile shows the current-legislature attendance rate
- [ ] Sync-seam tests cover vote fixtures and both attendance paths (bulk + derived)
- [ ] HTTP-API-seam tests cover the section payload incl. source + sync dates
