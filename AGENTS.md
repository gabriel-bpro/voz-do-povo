# voz-do-povo

A web app to help people understand, investigate, and do diligence on the politicians they elected — built on Brazilian government open data, explained in plain language.

## Layout

- `cmd/api`, `cmd/worker` — Go services (API serves the curated layer; worker runs syncs)
- `internal/` — `store` (Postgres + embedded migrations in `internal/store/migrations/`), `httpapi`, `sync` (raw landing + promotion to curated, identity linkage), `camara` (Câmara source adapter: fetch + parse only, no DB), `testharness`
- `web/` — Next.js app (App Router); note `web/AGENTS.md` (Next-version-specific rules)
- `.hosting_provider/hosting_provider.ts` — hosting provider infrastructure as code (4 units: web, api, worker, Postgres); change via `hosting_provider config plan` / `apply`

## Commands

- Go: `go build ./...`, `go vet ./...`, `go test ./...`
- Web: `cd web && npm run lint`, `npm run build`
- Deploy: `hosting_provider up --service api|worker --detach` from repo root; `hosting_provider up ./web --path-as-root --service web --detach` for web

## Tests — two agreed seams (see `.scratch/voz-do-povo-mvp/spec.md`)

1. **HTTP API seam** — drive the API over HTTP against a real Postgres seeded at the curated layer (`testharness.API`).
2. **Sync seam** — feed recorded source fixtures into sync entry points, assert raw + curated table state (`testharness.TestDB` + `testharness.Fixture`).

Both spin a throwaway Postgres via Docker — tests require a running Docker daemon. The frontend has no test layer (type-check + lint only).

## Agent skills

### Issue tracker

Issues live as local markdown files under `.scratch/<feature-slug>/`. See `docs/agents/issue-tracker.md`.

### Triage labels

Default vocabulary (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: `CONTEXT.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.
