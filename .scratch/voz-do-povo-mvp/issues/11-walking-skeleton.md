# 11 — Walking skeleton

**What to build:** the deployable end-to-end thread every later ticket hangs on: Next web service, Go API service, Go worker service (same Go codebase, separate deployable) and Postgres running on Railway with private networking; raw and curated layers exist in the schema; CI is green; both test-seam harnesses are established (HTTP-API-seam: drive the API over HTTP against a test Postgres seeded at the curated layer; sync-seam: feed a recorded source payload into a sync entry point and assert table state). A citizen can load a page that renders a value served by the API from the database. Railway provisioning happens here, including setting `GCP_SERVICE_ACCOUNT_JSON` on the worker service.

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

- [ ] All four Railway units (web, API, worker, Postgres) deployed and communicating over private networking
- [ ] `GCP_SERVICE_ACCOUNT_JSON` env var set on the worker service
- [ ] Raw + curated layer structure exists in Postgres (provenance columns: source, pulled-at on raw tables)
- [ ] HTTP-API test seam harness working: a test drives the API over HTTP against a seeded test Postgres
- [ ] Sync test seam harness working: a test feeds a fixture payload into a sync entry point and asserts table state
- [ ] A citizen-facing page renders a value that traveled DB → API → web
- [ ] CI runs both test seams green

## Comments

_2026-09-03 —_ Railway provisioning checklist for the human-driven infra steps: [railway-provisioning.md](../railway-provisioning.md).
