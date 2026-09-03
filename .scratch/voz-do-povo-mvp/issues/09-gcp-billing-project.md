# Set up GCP billing project for Base dos Dados BigQuery

Type: task
Status: resolved
Blocked by: —

## Question

Manual HITL setup the agent cannot do: create a GCP project with billing enabled (BigQuery stays within the free 1 TB queries/month tier for our ~few-GB TSE slices), create a service account with BigQuery read access, and store its JSON key for the Nest worker (per the data-architecture decision: `@google-cloud/bigquery` in the worker pulls from `basedosdados.br_tse_eleicoes`). Resolution records where the credentials live (e.g. Railway env var name) so the sync implementation can reference them.

## Answer

_Done 2026-09-03 by the dev driving the map._

- **GCP project**: created, billing enabled. BigQuery usage stays within the free 1 TB queries/month tier (our TSE slices are a few GB, pulled monthly).
- **Service account**: created with BigQuery read access (`roles/bigquery.dataViewer` + `roles/bigquery.jobUser`); JSON key downloaded.
- **Credentials convention**: the full JSON key contents go in a Railway environment variable named **`GCP_SERVICE_ACCOUNT_JSON`** on the **worker service**; the worker does `JSON.parse(process.env.GCP_SERVICE_ACCOUNT_JSON)` and reads the project ID from the key itself. Railway is not provisioned yet — the key is held locally by the dev and gets set as this env var at provisioning time (provisioning is execution work for the build phase, not a map decision).
