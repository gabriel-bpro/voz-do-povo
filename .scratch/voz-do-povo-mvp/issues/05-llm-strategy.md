# Choose the LLM strategy

Type: grilling
Status: resolved
Blocked by: 04

## Question

Which LLM(s), at what cost envelope, doing exactly which jobs?

- NL query → structured query translation against the mirrored store.
- Plain-language explanations around templated numbers (the LLM never generates figures).
- Answer rendering for NL queries.

Includes guardrails (source citation, refusal when data is missing), evaluation approach for translation accuracy, and the fallback when the monthly budget cap is hit. Blocked on the data architecture because the translation target (schemas, query interface) must exist first.

## Answer

_Decided 2026-09-02 (grilling, 2 rounds; the session surfaced a product pivot that rewrites this ticket's premise — recorded below.)_

**⚠ Product pivot: free-form NL query is out of the MVP.** Mid-session the dev ruled "the free-form chatbot will not exist anymore — we're pivoting" (confirmed: free-form NL dropped entirely from the MVP). The LLM's only job is **plain-language walkthroughs of available data**, never a free-form chatbot. This redraws the map's destination and amends tickets 03 and 04.

**Model: Kimi K3** via the Kimi (Moonshot) platform API key the dev already pays for. Facts (verified 2026-09-02 on `platform.moonshot.ai/docs/pricing/chat-k3`): $0.30/M cache-hit input, $3.00/M input, $15.00/M output, 1M-token context, automatic context caching. The NLP stack calls it through an OpenAI-compatible client pointed at the Moonshot base URL; the API key lives in Railway env vars.

**The LLM's only job: plain-language walkthroughs of available data.** The LLM explains what is on the page — expenses, votes, patrimony, emendas — walking the citizen through the available figures. It never answers questions the data wasn't asked for.

**Generation timing: precompute post-sync, cache everything.** The worker regenerates walkthroughs only for politicians whose data changed in the last sync; the app never calls the LLM at request time. NL-as-request-time-surface died with the pivot, so the app has no live LLM path at all — which is also the budget cap answer: **when the cap is hit, pages render data-only** (templates + structured components) and nothing breaks.

**Guardrails**: every figure in generated prose is verified against the fact set passed in; unverifiable figure → regenerate once → fall back to the templated version. No figures ever enter LLM output without source-traceable backing. (This operationalizes the dev's "let the LLM only walk through the available data".)

**Cost posture**: with precompute-only and ~594 current federal profiles, the entire explanation corpus regenerates for pocket change per sync — K3's 1M-token context is massively oversized for this job; per-call cost is dominated by output tokens and trivially cheap. Budget cap behavior above is mostly a belt-and-suspenders.

**Eval: none** (dev's call). Manual spot checks on tone/accuracy in staging before deploys.

**Failure UX**: figure-verification failure → regenerate once → template fallback. Profile pages always have a pure-template render path, so LLM absence never blanks a page.

**⚠ Second pivot (same session, later round): walkthroughs are gone from the website entirely.** The site renders data + templated components only — **zero LLM calls in the MVP**. The LLM debuts in a **post-MVP** feature: **"Follow your politician"** — citizens subscribe per politician and receive e-mail notifications when data changes; the e-mail body is LLM-generated, one LLM call per change event, fanned out to N followers (dev's design, 2026-09-02). The follow feature was ruled **post-MVP** and lives in the map's Out of scope until a fresh effort charts it. Consequence for this ticket: the MVP's LLM strategy is "no LLM" — the decisions above (Kimi K3, precompute, figure guardrails) park with the follow feature, not the MVP.

## Comments
