#!/usr/bin/env bash
# Build images and save to dist/*.tar.gz for copying to a VPS (docker load).
# Barn naming with legacy compat.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [[ -f .env ]]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

# Default to barn naming
API_IMAGE="${API_IMAGE:-barn-api:latest}"
FRONTEND_IMAGE="${FRONTEND_IMAGE:-barn-frontend:latest}"
MIGRATE_IMAGE="${MIGRATE_IMAGE:-barn-migrate:latest}"
POSTGRES_IMAGE="${POSTGRES_IMAGE:-barn-postgres:latest}"
POSTGRES_BASE="${POSTGRES_BASE:-postgres:16-alpine}"
DOCKER_PLATFORM="${DOCKER_PLATFORM:-linux/amd64}"
export DOCKER_PLATFORM
OUTPUT_DIR="${OUTPUT_DIR:-dist}"
BUNDLE="${BUNDLE:-${OUTPUT_DIR}/barn-images.tar.gz}"

mkdir -p "$OUTPUT_DIR"

"$ROOT/scripts/docker-build.sh"

echo "Building migrate image (${DOCKER_PLATFORM})..."
docker build --platform "$DOCKER_PLATFORM" -t "$MIGRATE_IMAGE" -f backend/Dockerfile.migrate backend

echo "Pulling PostgreSQL (${POSTGRES_BASE}) for ${DOCKER_PLATFORM}..."
docker pull --platform "$DOCKER_PLATFORM" "$POSTGRES_BASE"
docker tag "$POSTGRES_BASE" "$POSTGRES_IMAGE"

# Also tag as dock-pilot-* for backward compatibility
echo "Tagging barn images with legacy dock-pilot names for backward compat..."
docker tag "$API_IMAGE" "dock-pilot-api:latest"
docker tag "$FRONTEND_IMAGE" "dock-pilot-frontend:latest"
docker tag "$MIGRATE_IMAGE" "dock-pilot-migrate:latest"
docker tag "$POSTGRES_IMAGE" "dock-pilot-postgres:latest"

echo "Saving images to ${BUNDLE}..."
docker save \
  "$API_IMAGE" \
  "$FRONTEND_IMAGE" \
  "$MIGRATE_IMAGE" \
  "$POSTGRES_IMAGE" \
  dock-pilot-api:latest \
  dock-pilot-frontend:latest \
  dock-pilot-migrate:latest \
  dock-pilot-postgres:latest \
  | gzip > "$BUNDLE"

ls -lh "$BUNDLE"

cat <<EOF

Export ready (${POSTGRES_IMAGE} included — no separate Postgres install needed).
Includes both barn-* and dock-pilot-* tags for compatibility.

  scp ${BUNDLE} docker-compose.barn.yml .env.barn.example scripts/barn-*.sh user@your-vps:/opt/barn/

On VPS (new installs):

  cd /opt/barn
  cp .env.barn.example .env && chmod 600 .env
  gunzip -c barn-images.tar.gz | docker load
  chmod +x scripts/barn-*.sh
  ./scripts/barn-up.sh

Legacy VPS (using dock-pilot paths):

  cd /opt/dock-pilot
  cp .env.dock-pilot.example .env && chmod 600 .env
  gunzip -c barn-images.tar.gz | docker load
  chmod +x scripts/dock-pilot-*.sh
  ./scripts/dock-pilot-up.sh

Or use the one-line installer: scripts/install.sh

EOF
