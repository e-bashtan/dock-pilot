#!/usr/bin/env bash
# Apply SQL migrations to bundled PostgreSQL from DATABASE_URL in .env (Barn).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
COMPOSE_FILE="${BARN_COMPOSE:-${DOCK_PILOT_COMPOSE:-docker-compose.barn.yml}}"
[[ -f "$COMPOSE_FILE" ]] || COMPOSE_FILE="docker-compose.dock-pilot.yml"

if [[ ! -f .env ]]; then
  echo "[barn] Create .env from .env.barn.example first." >&2
  exit 1
fi

set -a
# shellcheck disable=SC1091
source .env
set +a

if [[ -z "${DATABASE_URL:-}" ]]; then
  echo "[barn] DATABASE_URL is not set in .env" >&2
  exit 1
fi

echo "[barn] Ensuring PostgreSQL is running..."
docker compose -f "$COMPOSE_FILE" up -d postgres

echo "[barn] Running migrations (goose up)..."
docker compose -f "$COMPOSE_FILE" run --rm migrate

echo ""
echo "[barn] Migration status:"
docker compose -f "$COMPOSE_FILE" run --rm migrate status

echo ""
echo "[barn] Done."
