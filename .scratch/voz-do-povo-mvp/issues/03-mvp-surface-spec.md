# Define the MVP surfaces

Type: grilling
Status: resolved
Blocked by: 01, 02

## Question

What exactly are the three MVP surfaces?

- **Home dashboard** — which rankings/highlights exist (top spenders? biggest single expenses? by which period?), and what else earns a place on the landing page.
- **Politician profile** — which sections, in what order, assembling the diligence quartet.
- **NL query experience** — entry points, answer shape, empty/failure states.

Produces the surface-by-surface content list that the spec and the report prototype build on. Blocked on the API research because the surfaces can only promise what the data actually delivers.

## Answer

_Resolved 2026-08-27 via grilling with the dev driving the map._

### Home dashboard

Five elements, in order:

1. **NL query box**, front and center.
2. **Politician search bar** (by name).
3. **Rankings block** — five rankings, ~10 rows each, every row linking to a profile:
   - Top spenders of the current calendar year (per house).
   - Biggest single expenses of the month (per house).
   - Maiores emendas PIX do ano (unified — the least traceable money; motivated by the "farra das emendas" concern).
   - Attendance laggards.
   - Largest declared patrimonies.
   - A single Deputados/Senadores toggle atop the block switches the per-house rankings; patrimony stays unified.
4. **One-line explainer** of the project.

Ranking definitions: period = current calendar year (daily sync keeps it live); expense rankings split per house (CEAP/CEAPS quotas are not comparable); both "total spent" and "biggest single expense" are shown since they tell different stories. No editorial content, no charts beyond simple bars.

### Politician profile

Sections, in order:

1. **Identity header** + one-paragraph plain-language summary — _amended 2026-09-02 ([ticket 05](05-llm-strategy.md)): the MVP has zero LLM calls; this summary is rendered from **templates**, not generated prose._
2. **Expenses** — current calendar year total by default (year selector back to 2008), monthly trend, largest items.
3. **Emendas** (MVP, not optional): totals by year empenhado vs pago; breakdown by policy area and by state/municipality; top 5 largest emendas with beneficiary name; **emendas PIX flagged distinctly**; a plain-language line explaining what an emenda is and why the empenhado/pago gap matters.
4. **Votes & attendance** — ~20 most recent nominal votes with party-orientation comparison; attendance rate for the current legislature.
5. **Declared patrimony** — total plus evolution across elections since 2006.
6. **Campaign financing** — most recent election cycle: total raised, top donors.

### NL query experience

> **Amendment 2026-09-02 (pivot in [Choose the LLM strategy](05-llm-strategy.md)):** free-form NL querying is **out of the MVP**. The NL query box leaves the home dashboard (the politician search bar moves to the front), the `/perguntar` page is dropped, and there is no NL answer surface. The LLM's only job is plain-language walkthroughs of available data, precomputed post-sync — see ticket 05. The failure-state principles below survive as the spirit of the product (silence with guidance over a wrong number).

- **Entry points**: ~~home box + dedicated page (`/perguntar`)~~ _dropped — see amendment above._
- **Answer shape**: short LLM-written lead sentence, then a templated table/list with source links and profile links. Every answer cites the source dataset and its sync date.
- **Failure states**: honest refusal in plain language ("ainda não tenho dados sobre X") with 2–3 suggested rephrasings or example questions; ambiguous politician names get a disambiguation list. Never a hallucinated answer — silence with guidance over a wrong number.

### Attention badge

The MVP **does** include the "chama atenção" badge, driven by a small fixed set of transparent rules (e.g. 3+× the category median, sanctioned supplier). The rules themselves are defined in [Define the attention-badge rules](08-attention-badge-rules.md).

### Party-level campaign financing

No party-level view in the MVP — candidate-level receitas already capture party donations, and Base dos Dados' party-level tables lag. Ruled out of scope on the map.
