# Research: TSE open data — patrimony, campaign financing, candidate registry

_Resolved on 2026-08-27. TSE properties are still bot-blocked from this network (re-verified, details below), so the primary evidence for dataset contents comes from the **Base dos Dados mirror**, whose metadata API was queried live this session and whose public cleaning code documents the quirks of the official TSE files. Claims that could not be verified against TSE directly are marked._

## Direct TSE access — still blocked (verified 2026-08-27)

All TSE properties return **HTTP 403** from this environment, including with a realistic browser User-Agent, Accept and Accept-Language headers and a Google referer:

- `https://dadosabertos.tse.jus.br/` (catalog homepage) — 403
- `https://cdn.tse.jus.br/estatistica/sead/odsele/consulta_cand/consulta_cand_2024.zip` (known bulk-file pattern) — 403
- `https://cdn.tse.jus.br/estatistica/sead/odsele/prestacao_contas/prestacao_de_contas_eleitorais_candidatos_2024.zip` — 403
- `https://www.tse.jus.br/eleicoes/estatisticas` and `https://www.tse.jus.br/` — 403
- `https://divulgacandcontas.tse.jus.br/divulga/` and its REST endpoint `.../divulga/rest/v1/eleicao/listar` — 403

The 403 page is an **Akamai edge deny** (`errors.edgesuite.net` reference; the hosts resolve to `*.edgesuite.net`), i.e. IP-reputation/bot blocking at the CDN layer, not application auth. Faking browser headers is not enough; the block targets the datacenter egress IP. Only exception observed: `https://resultados.tse.jus.br/` returned a 302 redirect rather than 403 (election-night results site — not useful for the diligence quartet anyway).

**Consequence**: naive HTTP sync of TSE data from a datacenter will not work. The official bulk URLs follow stable, predictable patterns (e.g. `consulta_cand_<ano>.zip`), so a **manual or residential-network one-shot download per election cycle** is viable, but cannot be automated from typical cloud/server IPs.

## What TSE officially exposes (structure known; contents verified indirectly)

The official portal is `https://dadosabertos.tse.jus.br/` (a CKAN-style catalog). Base dos Dados registers it as the single raw source for every TSE table they mirror (verified via BD's metadata API, see below). Known characteristics of the official bulk dumps (from Base dos Dados' cleaning code, which parses them — see "Data-quality gotchas"):

- **Access mechanism**: bulk downloads only — per-year ZIP files containing **one CSV/TXT per UF** (plus a `BR` file for national-level offices in presidential years). There is **no official REST/GraphQL API** for the open-data bulk datasets.
- **Formats**: semicolon-delimited text files, Portuguese header rows, decimal **comma** for money, dates as `DD/MM/YYYY`, sentinel nulls like `#NULO#` / `#NE#` / `-1` / `NÃO DIVULGÁVEL`.
- **Datasets relevant to the quartet** (names as mirrored by BD, official catalog equivalents in parentheses):
  - `candidatos` (Repositório de Dados Eleitorais → "Candidatos" / `consulta_cand_<ano>`) — 1994–2026
  - `bens_candidato` ("Bens de candidatos" / `bem_candidato_<ano>`) — 2006–2026
  - `receitas_candidato` and `despesas_candidato` ("Prestação de contas eleitorais" / `prestacao_contas_<ano>`) — 2002–2024
- **DivulgaCand / DivulgaCandContas** (`divulgacandcontas.tse.jus.br`): interactive web app backed by an **undocumented** internal JSON REST API (`/divulga/rest/v1/...`). Also 403 from this network. Not a documented programmatic interface; treat as out of scope for mirroring.
- **Update cadence across the election cycle (official)** — could not be verified from TSE pages this session (403). Base dos Dados' catalog metadata states an update frequency of **2/year** for `candidatos`, `bens_candidato`, `receitas_candidato` and `despesas_candidato` (see next section); their latest refresh was 2026-08-12, i.e. mid-campaign of the 2026 cycle.

## Base dos Dados mirror — verified live (2026-08-27)

Dataset page: `https://basedosdados.org/dataset/eleicoes` — "Eleições Brasileiras", source organization registered as "Tribunal Superior Eleitoral (TSE)", description "Dados de eleições brasileiras desde 1945 fornecidos pelo TSE". All facts below come from BD's public GraphQL metadata API (`https://backend.basedosdados.org/api/v1/graphql`), queried this session.

### Diligence-quartet tables (all free — `isClosed: false` — all in BigQuery dataset `basedosdados.br_tse_eleicoes`)

| BD table | Rows | Size (uncompressed) | Coverage | Declared update freq | Last BD refresh |
|---|---|---|---|---|---|
| `candidatos` | 3,375,303 (28 cols) | ~0.99 GB | 1994–2026 | 2/year | 2026-08-12 |
| `bens_candidato` | 5,107,538 (10 cols) | ~0.72 GB | 2006–2026 | 2/year | 2026-08-12 |
| `receitas_candidato` | 15,311,065 (55 cols) | ~5.5 GB | 2002–2024 | 2/year | 2026-08-12 |
| `despesas_candidato` | 32,411,723 (45 cols) | ~14 GB | 2002–2024 | 2/year | 2026-08-12 |

Supporting tables also mirrored: `resultados_candidato` (1945–2024), `partidos` (1990–2026), `vagas` (1994–2026), `receitas_comite` (2002–2014, stale), `receitas_orgao_partidario` (2010–2022, lags), `dicionario` (data dictionary), plus voter/results-by-section tables (large, not needed for the quartet). A `certidoes_criminais` table exists in the catalog but is **empty** (no rows).

### Join keys (verified from BD column metadata)

- `candidatos.sequencial` ←→ `bens_candidato.sequencial_candidato`, `receitas_candidato.sequencial_candidato`, `despesas_candidato.sequencial_candidato` (valid **within** ano+UF; also `numero`/`numero_candidato` and `titulo_eleitoral` on all four).
- `candidatos` carries `cpf`, `titulo_eleitoral`, `nome`, `nome_urna`, `sigla_partido`, `cargo`, `situacao`, `id_eleicao`, `id_municipio` (IBGE 7-digit — bridge to other BD datasets).
- `receitas_candidato` carries donor detail: `cpf_cnpj_doador`, `nome_doador`, `nome_doador_rf` (Receita Federal name), `valor_receita`, `data_receita`, `especie/fonte/origem_receita`; `despesas_candidato` mirrors this for suppliers (`cpf_cnpj_fornecedor`, `valor_despesa`, ...). The CNPJ columns are a ready-made bridge to CGU sanctions checks from the Portal da Transparência research.

### Access mechanisms (BD docs)

- **BigQuery (SQL)** against `basedosdados.br_tse_eleicoes.*` — requires a GCP **billing project**; BigQuery's free tier gives **1 TB of queries/month** (`https://basedosdados.org/docs/access_data_bq`, `https://basedosdados.org/docs/home`).
- **Python / R packages** — `read_sql`, `read_table`, `download()` (to CSV, GZIP) (`https://basedosdados.org/docs/api_reference_python`, `https://basedosdados.org/docs/access_data_packages`).
- **Direct CSV download** from the dataset page — capped at **200k rows**, so only useful for small tables, not for the quartet tables (`https://basedosdados.org/docs/home`).
- **GraphQL metadata API** (`https://backend.basedosdados.org/api/v1/graphql`, no auth observed) — freshness (`updates.latest`), row counts, coverage, columns; ideal for a cheap "is there new data?" check before re-pulling.

### Freshness verdict

Metadata shows the four quartet tables were re-built on **2026-08-12** (15 days before this research) with coverage already including the **2026** election year for `candidatos`/`bens_candidato` — i.e. BD is actively refreshing during the current campaign. Declared cadence is 2/year; observed practice in an election year is better.

## Data-quality gotchas

Documented by Base dos Dados' own cleaning code (`https://github.com/basedosdados/sdk/tree/master/bases/br_tse_eleicoes/code` — the `dataCleaningCodeUrl` registered for these tables), which is the best surviving primary evidence of the official files' quirks:

- **Schema drift every cycle**: `candidatos` needs **three different column layouts** (≤2014/2018, 2016/2020–2022, 2024); `bens_candidato` changes layout at 2014; `prestacao_contas.do` is ~50 KB of **per-year bespoke parsing** (2002 and 2004 each have unique layouts).
- **Per-UF file split**: one file per state per year; presidential years add a `BR` pseudo-UF file; municipal years have no `DF`/`BR` file. Mixed `.txt` vs `.csv` extensions across years.
- **Sentinel nulls**: `#NULO#`, `#NULO`, `#NE#`, `#NE`, `#NI#`, `-1`, `-4`, `NÃO DIVULGÁVEL`, `##VERIFICAR BASE 1994##` — all must be normalized to null.
- **CPF inconsistencies**: CPFs arrive with variable digit counts and need zero-left-padding (BD pads lengths 5–10 up to 11). Verify empirically whether `cpf` is populated for recent years (TSE has progressively restricted personal identifiers in public dumps) — **unverified**.
- **Título eleitoral corruption**: values with embedded spaces requiring positional repair; `"000000000000"` used as null.
- **Money and dates**: decimal comma (`1.234,56`), dates `DD/MM/YYYY` with occasional invalid years (BD drops dates < 1900).
- **Malformed rows**: BD explicitly drops rows that fail CSV parsing (`drop if ano == . — "casos de leitura problematica do csv"`).
- **Party inconsistencies across years**: a dedicated per-year `limpa_partido` routine exists (renames/mergers, e.g. PTB→PRTB-era changes) — cross-year party analysis needs a normalized party dimension, not raw sigla.
- **Encoding**: TSE files are historically **ISO-8859-1/Latin-1** (BD's code only specifies UTF-8 for its own auxiliary file, never for TSE inputs); newer cycles are reported as UTF-8. **Not verified against TSE this session** — confirm on first manual download.
- Using BD's cleaned tables removes essentially all of the above (they also add `id_municipio` IBGE codes and Receita-Federal-name columns for donors/suppliers).

## Implications for the mirror pipeline

- **Sync TSE via Base dos Dados, not TSE directly.** BD mirrors everything the quartet needs, refreshed mid-campaign, with a free 1 TB/month BigQuery allowance — far more than the ~21 GB quartet footprint. A monthly (weekly in election season) `download()`/`COPY` from `br_tse_eleicoes.{candidatos,bens_candidato,receitas_candidato,despesas_candidato}` into our own store, gated on BD's `updates.latest` via their GraphQL API, is a complete pipeline.
- **Fallback for freshness-critical moments** (e.g. prestacao parcial during campaign weeks, if BD lags): manual or residential-network one-shot download of the official zips — URL patterns are stable (`cdn.tse.jus.br/estatistica/sead/odsele/...`), but expect the schema-drift and cleaning gotchas above.
- **Identity linkage**: use `sequencial`+ano+UF for TSE-internal joins; `cpf` (where populated) and `titulo_eleitoral` for cross-source identity; donor/supplier CNPJs bridge to CGU sanctions. A canonical politician entity still needs a per-source id map, as flagged in the legislative research.
- **Cost**: well inside BigQuery free tier; BD Python/R packages require only a GCP billing project, no BD account, for open tables.

## Known limitations of this research

- Nothing was verified against TSE's own pages or files (403); the official catalog structure, cadence and encoding notes are second-hand via BD's metadata and cleaning code. Human verification from a non-blocked network is needed for: exact official dataset names/URLs, official cadence statements, file encodings per year, and whether CPF is still published.
- BD's `receitas_comite` (ends 2014) and `receitas_orgao_partidario` (ends 2022) lag — party-level financing may need a direct TSE pull.
- BD GraphQL `lastUpdatedAt` resolver returned HTTP 500; freshness was taken from the `updates.latest` field instead.
- Whether BD will keep refreshing at campaign pace (beyond the declared 2/year) is an assumption based on the observed 2026-08-12 refresh.
