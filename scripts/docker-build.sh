#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [[ -f .env ]]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

API_IMAGE="${API_IMAGE:-barn-api:latest}"
FRONTEND_IMAGE="${FRONTEND_IMAGE:-barn-frontend:latest}"
DOCKER_PLATFORM="${DOCKER_PLATFORM:-linux/amd64}"

export API_IMAGE FRONTEND_IMAGE DOCKER_PLATFORM
export NEXT_PUBLIC_API_URL="${NEXT_PUBLIC_API_URL:-auto}"
export NEXT_PUBLIC_API_TOKEN="${NEXT_PUBLIC_API_TOKEN:-}"
if [[ -z "${NEXT_PUBLIC_APP_VERSION:-}" ]]; then
  NEXT_PUBLIC_APP_VERSION="$(git describe --tags --always 2>/dev/null || echo dev)"
fi
export NEXT_PUBLIC_APP_VERSION

echo "Building images for platform: ${DOCKER_PLATFORM}"
echo "  API:      ${API_IMAGE}"
echo "  Frontend: ${FRONTEND_IMAGE}"
echo "  NEXT_PUBLIC_API_URL=${NEXT_PUBLIC_API_URL}"
echo "  NEXT_PUBLIC_APP_VERSION=${NEXT_PUBLIC_APP_VERSION}"

# Ensure buildx is available for cross-platform builds (e.g. Mac ARM → VPS AMD64)
if ! docker buildx inspect dock-pilot-builder >/dev/null 2>&1; then
  docker buildx create --name dock-pilot-builder --use >/dev/null 2>&1 \
    || docker buildx use default
else
  docker buildx use dock-pilot-builder >/dev/null 2>&1 || true
fi

DOCKER_DEFAULT_PLATFORM="${DOCKER_PLATFORM}" \
  docker compose -f docker-compose.build.yml build

echo ""
echo "Built (${DOCKER_PLATFORM}):"
docker images --format '  {{.Repository}}:{{.Tag}}  {{.Size}}' \
  | grep -E 'dock-pilot-(api|frontend|migrate)' || true
