# voz-do-povo

A web app to help people understand, investigate, and do diligence on the politicians they elected — built on Brazilian government open data, explained in plain language (pt-BR product, English codebase).

The app maintains its own **mirror** of government open data, synced per source cadence; it never calls government APIs at request time. Data lands in a **raw layer** (staging, verbatim, with provenance) and is promoted to a **curated layer** — the only schema the app queries.

## Repository layout

| Path | Description |
|---|---|
| `cmd/api` | API service — serves the curated layer over HTTP |
| `cmd/worker` | Worker service — runs the data syncs |
| `internal/store` | Postgres access + embedded migrations (`internal/store/migrations/`) |
| `internal/httpapi` | HTTP handlers |
| `internal/sync` | Raw landing + promotion to curated, identity linkage |
| `internal/camara` | Câmara dos Deputados source adapter (fetch + parse only, no DB) |
| `internal/testharness` | Test fixtures and throwaway-Postgres helpers |
| `web/` | Next.js app (App Router) — see `web/AGENTS.md` |
| `.hosting_provider/hosting_provider.ts` | hosting provider infrastructure as code (4 units: web, api, worker, Postgres) |

## Prerequisites

- Go 1.25+
- Node.js + npm
- Docker (required for tests — both test seams spin up a throwaway Postgres)

## Development

```bash
# Go services
go build ./...
go vet ./...
go test ./...        # requires a running Docker daemon

# Web
cd web && npm install
npm run lint
npm run build
```

Configuration via `.env` (e.g. `GCP_SERVICE_ACCOUNT_JSON`, database connection settings).

## Tests

Two agreed seams (see `.scratch/voz-do-povo-mvp/spec.md`):

1. **HTTP API seam** — drive the API over HTTP against a real Postgres seeded at the curated layer (`testharness.API`).
2. **Sync seam** — feed recorded source fixtures into sync entry points, assert raw + curated table state (`testharness.TestDB` + `testharness.Fixture`).

Both spin a throwaway Postgres via Docker. The frontend has no test layer (type-check + lint only).

## Deploy

```bash
hosting_provider up --service api --detach        # from repo root
hosting_provider up --service worker --detach     # from repo root
hosting_provider up ./web --path-as-root --service web --detach
```

Infrastructure changes go through `hosting_provider config plan` / `hosting_provider config apply` (`.hosting_provider/hosting_provider.ts`).

## Domain docs

- `CONTEXT.md` — glossary and domain language
- `docs/adr/` — architectural decisions
- `AGENTS.md` — agent-oriented project guide
