# Railway provisioning checklist (ticket 11)

Human-driven setup for the walking skeleton's infrastructure. Run top to bottom; checkboxes are confirmation gates. Expected cost posture: Hobby plan, $5/mo floor, ~$7–15/mo estimated; the 5GB Postgres volume cap is the Pro-plan trigger.

## 1. Install the Railway CLI

```bash
brew install railway        # macOS
# or: npm i -g @railway/cli
# or: bash <(curl -fsSL railway.com/install.sh)
```

- [x] `railway --version` prints a version

## 2. Login

```bash
railway login               # opens the browser
railway whoami              # verify
```

- [x] `railway whoami` shows your Railway account

## 3. Create and link the project

```bash
railway init                # name it: voz-do-povo
railway status              # verify the directory is linked
```

- [x] `railway status` shows the linked project

## 4. Provision Postgres

```bash
railway add --database postgres
```

- [x] Postgres appears in the project (`railway status` or dashboard)
- [x] Note the exact Postgres service name (default: `Postgres`) — needed in stage 6

## 5. Create the three app services

Create three **empty services** named `web`, `api`, `worker`:

```bash
railway add                 # interactive — choose "Empty Service", repeat 3×
# or dashboard: railway open → + Create → Empty Service
```

They stay idle until the skeleton code deploys into them.

- [x] `web`, `api`, `worker` all exist in the project

## 6. Set variables

The only secret: the GCP service-account JSON key (BigQuery `dataViewer` + `jobUser`), downloaded during ticket 09.

```bash
# Validate + compact the key (adjust the path):
jq -c . /path/to/service-account.json

# Worker: the BigQuery credential (ticket 09's locked convention)
railway variable set -s worker "GCP_SERVICE_ACCOUNT_JSON=$(jq -c . /path/to/service-account.json)"

# Private-network database references — never use the public TCP proxy between services
railway variable set -s api    'DATABASE_URL=${{Postgres.DATABASE_URL}}'
railway variable set -s worker 'DATABASE_URL=${{Postgres.DATABASE_URL}}'
```

(If the Postgres service isn't named `Postgres`, use its actual name in the reference.)

For local dev, also add the compacted JSON to the repo-local `.env` as `GCP_SERVICE_ACCOUNT_JSON`.

- [x] `railway variable list -s worker` shows `GCP_SERVICE_ACCOUNT_JSON` and `DATABASE_URL`
- [x] `railway variable list -s api` shows `DATABASE_URL`
- [x] `.env` has `GCP_SERVICE_ACCOUNT_JSON` for local dev

## 7. Verify

```bash
railway status
railway open                # eyeball the dashboard: 1 database + 3 empty services
```

- [x] Project shows Postgres + `web` + `api` + `worker`

## Reminders that bite later

- **Private networking only** between services — public TCP proxy egress is billed.
- **5GB Postgres volume cap** on Hobby; crossing it forces Pro ($20/mo), not higher usage pricing.
- Not done here (part of ticket 11's build work): deploying code via `railway up` (no GitHub remote yet), generating public domains, wiring the web → api URL.
