# 19 — Votes & attendance sync + profile section

**What to build:** the third leg of the quartet. The worker syncs nominal votes for both houses (API delta covering the days since the previous run) and attendance — Câmara via the `eventosPresencaDeputados` bulk (updated daily upstream), Senado derived by walking recent plenary sessions with polite polling (no bulk dataset exists). All jobs run inside the single weekly cron run (Monday 06:00 UTC). The profile gains the votes & attendance section: the ~20 most recent nominal votes with party-orientation comparison (how the politician voted vs their party's orientation) and the attendance rate for the current legislature, in the locked style with source + sync dates.

**Blocked by:** 13 — Search + profile expenses section. _(done 2026-09-13 — unblocked)_

**Status:** ready-for-agent

- [x] Nominal votes for both houses sync into curated on the weekly cron run, linked to canonical politicians
- [x] Câmara attendance syncs from the bulk; Senado attendance derives from the sessions walk (both weekly)
- [x] Profile shows ~20 most recent nominal votes with party-orientation comparison
- [x] Profile shows the current-legislature attendance rate
- [x] Sync-seam tests cover vote fixtures and both attendance paths (bulk + derived)
- [x] HTTP-API-seam tests cover the section payload incl. source + sync dates

**Implemented 2026-09-29.** Schema (`0005`): `vote` (denormalized: description/matter/result on the row), `vote_orientation` (blocs Governo/Oposição/Minoria dropped at sync; empty orientations dropped), `plenary_session` (the attendance denominator: Câmara = PLEN vote events behind nominal votes, Senado = CodigoSessao), `attendance`. Câmara: three yearly bulk files (`votacoes`, `votacoesVotos`, `votacoesOrientacoes`) + `eventosPresencaDeputados` (presences only — absences derived at query time); linking via the deputy's API id. Senado: `/plenario/votacao/nominal/{ano}` returns the full senator roster per vote with presence codes (Sim/Não/Votou/P-NRV/AP/MIS/LS/LP/LAP/NCom/NA), so **one call per year covers votes AND attendance — the planned per-session walk was unnecessary**; orientations from `orientacaoBancada` (60-day windows, polite pacing, join key `sequencialVotacao == CodigoVotacaoSve`, verified live 2026-09-28). Attendance status per session = best code (present > justified > absent). Profile payload: attendance rate (Câmara denominator = all legislature sessions; Senado = sessions with a record; justified absences counted separately, Senado only), alignment rollup (Sim/Não vs party Sim/Não only; Liberado never compares), ~20 most recent position votes with three-state `aligned`. Worker: `camara-votes`, `camara-attendance` (after votes — needs the sessions), `senado-votes` inside the weekly cron; `worker bootstrap-votes` backfills 2023+.
