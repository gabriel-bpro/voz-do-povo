# Estimate the real Railway monthly cost

Type: research
Status: resolved
Blocked by: —

## Question

What does the decided topology actually cost per month? Facts needed from Railway's current pricing (https://railway.com/pricing): their plan model (usage-based vs flat), per-service costs for 4 units — Next web service, Nest API service, Nest worker service, Postgres (with a few GB volume), low citizen traffic. Output: a concrete R$/month estimate against the ~R$0–100 envelope, including what happens as Postgres storage grows. Sources: Railway's official pricing docs.

## Answer

_Resolved 2026-09-02 via research subagent against Railway's official pricing docs._

**Hobby plan ($5/mo, $5 usage credit included) → estimated $7–15/mo ≈ R$45–90/month all-in** (R$5.60/US$, +IOF), with a $5 floor. Inside the R$0–100/month envelope with headroom. Two hard constraints to remember: 5GB Postgres volume cap on the Hobby plan (crossing it forces Pro at $20/mo, not higher usage pricing) and private networking between services (public TCP proxy egress is billed). Full rate table and BRL conversion: [research/railway-cost.md](../research/railway-cost.md).
