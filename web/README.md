# web

Next.js app (App Router) — the citizen-facing frontend for voz-do-povo.

## Development

```bash
npm install
npm run dev
```

Open [http://localhost:3000](http://localhost:3000). The app reads the curated layer through the Go API — set `API_HOST`/`API_PORT` (default `localhost:8080`).

## Build

```bash
npm run lint
npm run build
```

`web/Dockerfile` produces a standalone production server.
