# Research: Railway monthly cost for the MVP topology

_Resolved 2026-09-02 via research subagent against Railway's official pricing docs. For the topology decided in ticket 04: Next web service, Nest API service, Nest worker service, Postgres._

## Billing model (facts)

Railway bills a **plan subscription + metered per-second usage**; no per-service minimums; stopped services cost nothing.

- RAM: $10/GB/mo · CPU: $20/vCPU/mo · volume storage: $0.15/GB/mo (used space) · egress: $0.05/GB (outbound only; inbound free) · object storage $0.015/GB-mo with free egress.
- **Postgres is not a managed line item** — it's a template container billed by its actual RAM/CPU + volume. Use **private networking** between services (public TCP proxy egress is billed).
- Backups: incremental, billed at the $0.15/GB volume rate for incremental data only.

## Plans

- **Free Trial**: $0, $5 one-time credit, 30 days, 500MB volumes — spin-up only.
- **Free**: $1/mo credit but **3 services max**, 500MB volumes — doesn't fit (4 units).
- **Hobby $5/mo** (includes $5 usage): fits everything, hard limit of **5GB volume** per service.
- **Pro $20/mo**: 1TB volumes.

## Estimate for our topology (4 units, low traffic)

| Item | Lean | Comfortable |
|---|---|---|
| Next.js web | $3.40 | $5.50 |
| Nest API | $1.80 | $3.50 |
| Worker (cron-triggered, per-second billing) | $0.60 | $2.50 |
| Postgres + 3GB volume | $1.50 | $3.30 |
| **Total** | **~$7.40** | **~$15.10** |

**Hobby plan ($5/mo, includes $5 usage) → ~$7–15/mo ≈ R$45–90/month all-in** (R$5.60/US$, +IOF). Within the R$0–100 envelope with headroom. Watch: the 5GB Postgres volume cap on Hobby — crossing 5GB forces Pro ($20/mo). LLM cost (precomputed walkthroughs on Kimi K3) is negligible.

Detail: [issues/10-railway-cost-estimate.md](../issues/10-railway-cost-estimate.md) (research ticket) — full rate table there.