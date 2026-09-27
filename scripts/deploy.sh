#!/usr/bin/env bash
# Deploy voz-do-povo services to Railway.
#
# Usage:
#   scripts/deploy.sh api|worker|web   deploy one service
#   scripts/deploy.sh all              deploy api, worker, and web
#
# api/worker build from the repo root Dockerfile; web builds from ./web with
# --path-as-root (Next app, separate build context).

set -euo pipefail

cd "$(dirname "$0")/.."

if ! command -v railway >/dev/null 2>&1; then
  echo "railway CLI not found — install: https://docs.railway.com/guides/cli" >&2
  exit 1
fi

# RAILWAY_TOKEN (project token, used by CI) replaces interactive login.
if [ -z "${RAILWAY_TOKEN:-}" ] && ! railway whoami >/dev/null 2>&1; then
  echo "not logged in — run: railway login" >&2
  exit 1
fi

deploy_go_service() {
  echo "==> deploying $1 (repo root Dockerfile)"
  railway up --service "$1" --detach
}

deploy_web() {
  echo "==> deploying web (./web, path-as-root)"
  railway up ./web --path-as-root --service web --detach
}

case "${1:-}" in
  api|worker) deploy_go_service "$1" ;;
  web)        deploy_web ;;
  all)        deploy_go_service api; deploy_go_service worker; deploy_web ;;
  *)
    echo "usage: $0 api|worker|web|all" >&2
    exit 1
    ;;
esac
