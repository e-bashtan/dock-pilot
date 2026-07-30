#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

MAX_ATTEMPTS="${MAX_ATTEMPTS:-30}"
SLEEP_SEC="${SLEEP_SEC:-2}"
PG_USER="${POSTGRES_USER:-barn}"
PG_DB="${POSTGRES_DB:-barn}"

echo "Waiting for PostgreSQL to be ready..."

for i in $(seq 1 "$MAX_ATTEMPTS"); do
  if docker compose exec -T postgres pg_isready -U "$PG_USER" -d "$PG_DB" >/dev/null 2>&1; then
    echo "PostgreSQL is ready."
    exit 0
  fi
  # Legacy defaults
  if docker compose exec -T postgres pg_isready -U dockpilot -d dockpilot >/dev/null 2>&1; then
    echo "PostgreSQL is ready."
    exit 0
  fi
  echo "  attempt $i/$MAX_ATTEMPTS — not ready yet"
  sleep "$SLEEP_SEC"
done

echo "PostgreSQL did not become ready in time." >&2
exit 1
