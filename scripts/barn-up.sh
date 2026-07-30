#!/usr/bin/env bash
# VPS: start bundled PostgreSQL, migrate, then API + frontend (Barn).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
COMPOSE_FILE="${BARN_COMPOSE:-${DOCK_PILOT_COMPOSE:-}}"
if [[ -z "$COMPOSE_FILE" ]]; then
  if [[ -f docker-compose.barn-full.yml ]]; then
    COMPOSE_FILE=docker-compose.barn-full.yml
  elif [[ -f docker-compose.full.yml ]]; then
    COMPOSE_FILE=docker-compose.full.yml
  elif [[ -f docker-compose.barn.yml ]]; then
    COMPOSE_FILE=docker-compose.barn.yml
  else
    COMPOSE_FILE=docker-compose.dock-pilot.yml
  fi
fi

if [[ ! -f .env ]]; then
  echo "[barn] Create .env from .env.barn.example first." >&2
  exit 1
fi

set -a
# shellcheck disable=SC1091
source .env
set +a

for var in POSTGRES_PASSWORD DATABASE_URL SECRETS_ENCRYPTION_KEY API_TOKEN CORS_ALLOWED_ORIGINS CERTBOT_EMAIL; do
  if [[ -z "${!var:-}" ]]; then
    echo "[barn] Missing required variable in .env: ${var}" >&2
    exit 1
  fi
done

echo "[barn] Starting PostgreSQL..."
docker compose -f "$COMPOSE_FILE" up -d postgres

echo "[barn] Applying migrations..."
docker compose -f "$COMPOSE_FILE" run --rm -T migrate

echo "[barn] Starting API and frontend..."
docker compose -f "$COMPOSE_FILE" up -d api frontend

echo ""
docker compose -f "$COMPOSE_FILE" ps -a
