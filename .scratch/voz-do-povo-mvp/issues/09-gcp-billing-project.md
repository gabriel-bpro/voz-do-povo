# Set up GCP billing project for Base dos Dados BigQuery

Type: task
Status: open
Blocked by: —

## Question

Manual HITL setup the agent cannot do: create a GCP project with billing enabled (BigQuery stays within the free 1 TB queries/month tier for our ~few-GB TSE slices), create a service account with BigQuery read access, and store its JSON key for the Nest worker (per the data-architecture decision: `@google-cloud/bigquery` in the worker pulls from `basedosdados.br_tse_eleicoes`). Resolution records where the credentials live (e.g. Railway env var name) so the sync implementation can reference them.
