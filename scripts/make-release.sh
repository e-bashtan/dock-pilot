#!/usr/bin/env bash
# Build release tarball for GitHub Releases: images + compose + install files (Barn).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

VERSION="${1:-}"
if [[ -z "$VERSION" ]]; then
  VERSION="v$(date +%Y.%m.%d)"
fi
TAG="${VERSION#v}"
OUTPUT_DIR="${OUTPUT_DIR:-dist}"
BARN_RELEASE_DIR="${OUTPUT_DIR}/barn-${TAG}"
BARN_BUNDLE="${OUTPUT_DIR}/barn-${TAG}.tar.gz"
DOCK_PILOT_RELEASE_DIR="${OUTPUT_DIR}/dock-pilot-${TAG}"
DOCK_PILOT_BUNDLE="${OUTPUT_DIR}/dock-pilot-${TAG}.tar.gz"

log() { echo "[release] $*"; }

log "Building Docker images (NEXT_PUBLIC_API_URL=auto)..."
export NEXT_PUBLIC_API_URL=auto
export NEXT_PUBLIC_APP_VERSION="${VERSION}"
export DOCKER_PLATFORM="${DOCKER_PLATFORM:-linux/amd64}"
"$ROOT/scripts/docker-export.sh"

# Build barn artifact
log "Creating barn-${TAG} release bundle..."
mkdir -p "$BARN_RELEASE_DIR"
cp "${OUTPUT_DIR}/barn-images.tar.gz" "$BARN_RELEASE_DIR/" || cp "${OUTPUT_DIR}/dock-pilot-images.tar.gz" "$BARN_RELEASE_DIR/barn-images.tar.gz"
cp docker-compose.barn-full.yml docker-compose.barn.yml docker-compose.barn-migrate.yml "$BARN_RELEASE_DIR/"
cp -r install scripts "$BARN_RELEASE_DIR/"
cp .env.barn.example "$BARN_RELEASE_DIR/"
[[ -f .env.dock-pilot.example ]] && cp .env.dock-pilot.example "$BARN_RELEASE_DIR/"
echo "$VERSION" > "$BARN_RELEASE_DIR/VERSION"

tar -czf "$BARN_BUNDLE" -C "$OUTPUT_DIR" "barn-${TAG}"
ls -lh "$BARN_BUNDLE"

# Build dock-pilot artifact for backward compatibility
log "Creating dock-pilot-${TAG} legacy release bundle..."
mkdir -p "$DOCK_PILOT_RELEASE_DIR"
cp "${OUTPUT_DIR}/barn-images.tar.gz" "$DOCK_PILOT_RELEASE_DIR/dock-pilot-images.tar.gz" || cp "${OUTPUT_DIR}/dock-pilot-images.tar.gz" "$DOCK_PILOT_RELEASE_DIR/"
# Include both barn and legacy compose files for smooth transition
cp docker-compose.barn-full.yml "$DOCK_PILOT_RELEASE_DIR/"
cp docker-compose.full.yml docker-compose.dock-pilot.yml docker-compose.dock-pilot-migrate.yml "$DOCK_PILOT_RELEASE_DIR/"
cp -r install scripts "$DOCK_PILOT_RELEASE_DIR/"
cp .env.dock-pilot.example "$DOCK_PILOT_RELEASE_DIR/"
[[ -f .env.barn.example ]] && cp .env.barn.example "$DOCK_PILOT_RELEASE_DIR/"
echo "$VERSION" > "$DOCK_PILOT_RELEASE_DIR/VERSION"

tar -czf "$DOCK_PILOT_BUNDLE" -C "$OUTPUT_DIR" "dock-pilot-${TAG}"
ls -lh "$DOCK_PILOT_BUNDLE"

cat <<EOF

Release bundles:
  - ${BARN_BUNDLE} (primary, barn naming)
  - ${DOCK_PILOT_BUNDLE} (legacy compatibility)

Upload both to GitHub Release as:
  - barn-${TAG}.tar.gz
  - dock-pilot-${TAG}.tar.gz

Tag: ${VERSION}

One-line install on VPS (after release is published):

  # New installs (barn):
  curl -fsSL https://raw.githubusercontent.com/ebasht/barn/main/scripts/install.sh | sudo bash -s -- \\
    --domain deploy.example.com --email you@example.com --version ${VERSION}

  # Legacy (still works):
  curl -fsSL https://raw.githubusercontent.com/ebasht/barn/main/scripts/install.sh | sudo bash -s -- \\
    --domain deploy.example.com --email you@example.com --version ${VERSION}

EOF
