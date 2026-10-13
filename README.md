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

Configuration via `.env` — see `.env.example`.

## Tests

Two agreed seams:

1. **HTTP API seam** — drive the API over HTTP against a real Postgres seeded at the curated layer (`testharness.API`).
2. **Sync seam** — feed recorded source fixtures into sync entry points, assert raw + curated table state (`testharness.TestDB` + `testharness.Fixture`).

Both spin a throwaway Postgres via Docker. The frontend has no test layer (type-check + lint only).

## Running locally

```bash
docker compose up
```

Brings up Postgres, the API, the worker, and the web app. Configuration via `.env` — see `.env.example`.

## Deploy

Provider-agnostic: the repo-root `Dockerfile` builds both Go binaries (`/bin/api`, `/bin/worker`) and `web/Dockerfile` builds the Next.js standalone server. Deploy the three services plus a Postgres instance on any container platform, wiring the environment variables from `.env.example`.

## Domain docs

- `CONTEXT.md` — glossary and domain language
- `docs/adr/` — architectural decisions
- `AGENTS.md` — agent-oriented project guide
