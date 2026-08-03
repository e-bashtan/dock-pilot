#!/usr/bin/env bash
# Upgrade Barn on VPS: download release, load images, migrate, recreate containers.
#
#   sudo bash scripts/barn-upgrade.sh v0.1.7
#   sudo bash scripts/barn-upgrade.sh latest
#   sudo bash scripts/barn-upgrade.sh latest --domain panel.example.com --email you@example.com
#
set -euo pipefail

ROOT="${BARN_INSTALL_DIR:-${DOCK_PILOT_INSTALL_DIR:-/opt/barn}}"
GITHUB_REPO="${BARN_GITHUB_REPO:-${DOCK_PILOT_GITHUB_REPO:-ebasht/barn}}"
VERSION="${1:-latest}"
DOMAIN=""
EMAIL=""
SKIP_CERT=0

shift $(( $# > 0 ? 1 : 0 )) || true
while [[ $# -gt 0 ]]; do
  case "$1" in
    --domain) DOMAIN="$2"; shift 2 ;;
    --email) EMAIL="$2"; shift 2 ;;
    --skip-cert) SKIP_CERT=1; shift ;;
    -h|--help)
      cat <<EOF
Usage: barn-upgrade.sh [VERSION] [options]

  sudo bash barn-upgrade.sh latest
  sudo bash barn-upgrade.sh latest --domain panel.example.com --email you@example.com

Options:
  --domain DOMAIN   Configure panel HTTPS (DNS must point to this VPS)
  --email EMAIL     Let's Encrypt email (required with --domain)
  --skip-cert       With --domain: HTTP only, no TLS for the panel
EOF
      exit 0
      ;;
    *) die "Unknown option: $1 (try --help)" ;;
  esac
done

log() { echo "[barn] $*"; }
die() { echo "[barn] ERROR: $*" >&2; exit 1; }

download_with_progress() {
  local url="$1" dest="$2"
  local name cl size_human="" show_progress=0

  name="$(basename "$dest")"
  cl="$(curl -fsSLI -L "$url" 2>/dev/null | awk 'tolower($1)=="content-length:" {print $2; exit}' | tr -d '\r' || true)"
  if [[ -n "$cl" && "$cl" =~ ^[0-9]+$ ]]; then
    size_human="$(numfmt --to=iec-i --suffix=B "$cl" 2>/dev/null || echo "${cl} B")"
  fi

  if [[ -t 1 || -t 2 || -n "${BARN_FORCE_PROGRESS:-${DOCK_PILOT_FORCE_PROGRESS:-}}" ]]; then
    show_progress=1
  fi

  if [[ -n "$size_human" ]]; then
    log "Downloading ${name} (~${size_human})..."
  else
    log "Downloading ${name}..."
  fi

  if [[ "$show_progress" -eq 1 ]]; then
    if ! curl -fL --progress-bar --stderr - "$url" -o "$dest"; then
      return 1
    fi
    echo ""
    log "Download complete: ${name}"
    return 0
  fi

  log "No TTY — showing progress every 5s (set BARN_FORCE_PROGRESS=1 to force bar)..."
  curl -fsSL "$url" -o "$dest.part" &
  local pid=$!
  while kill -0 "$pid" 2>/dev/null; do
    if [[ -f "$dest.part" ]]; then
      local got
      got="$(stat -c%s "$dest.part" 2>/dev/null || stat -f%z "$dest.part" 2>/dev/null || echo 0)"
      if [[ -n "$cl" && "$cl" =~ ^[0-9]+$ && "$cl" -gt 0 ]]; then
        local pct=$((got * 100 / cl))
        log "  ${got} / ${cl} bytes (${pct}%)"
      else
        log "  ${got} bytes downloaded..."
      fi
    else
      log "  connecting..."
    fi
    sleep 5
  done
  wait "$pid"
  local rc=$?
  if [[ "$rc" -ne 0 ]]; then
    rm -f "$dest.part"
    return "$rc"
  fi
  mv -f "$dest.part" "$dest"
  log "Download complete: ${name}"
}

load_docker_images() {
  local images="$1"
  log "Loading Docker images from $(basename "$images")..."
  if [[ -t 1 || -t 2 || -n "${BARN_FORCE_PROGRESS:-${DOCK_PILOT_FORCE_PROGRESS:-}}" ]] && command -v pv >/dev/null 2>&1; then
    pv -f -pte "$images" | gunzip -c | docker load
  else
    if [[ -t 1 || -t 2 || -n "${BARN_FORCE_PROGRESS:-${DOCK_PILOT_FORCE_PROGRESS:-}}" ]] && ! command -v pv >/dev/null 2>&1; then
      log "Tip: apt install pv for load progress (percent bar)"
    fi
    gunzip -c "$images" | docker load
  fi
}

if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
  die "Run as root: sudo $0 [VERSION]"
fi

[[ -d "$ROOT" ]] || die "Install dir not found: ${ROOT}"
cd "$ROOT"
[[ -f .env ]] || die "Missing ${ROOT}/.env"

if [[ "$VERSION" == "latest" ]]; then
  VERSION="$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" \
    | grep -o '"tag_name": *"[^"]*"' | head -1 | cut -d'"' -f4)"
fi
[[ -n "$VERSION" ]] || die "Could not resolve release version"

FILE_TAG="${VERSION#v}"
BUNDLE="/tmp/barn-${FILE_TAG}.tar.gz"
# Try barn artifact first, fall back to dock-pilot for older releases
URL="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/barn-${FILE_TAG}.tar.gz"
if ! download_with_progress "$URL" "$BUNDLE" 2>/dev/null; then
  log "barn-${FILE_TAG}.tar.gz not found, trying dock-pilot-${FILE_TAG}.tar.gz..."
  URL="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/dock-pilot-${FILE_TAG}.tar.gz"
  BUNDLE="/tmp/dock-pilot-${FILE_TAG}.tar.gz"
  download_with_progress "$URL" "$BUNDLE"
fi

EXTRACT="$(mktemp -d)"
trap 'rm -rf "$EXTRACT"' EXIT
tar -xzf "$BUNDLE" -C "$EXTRACT" --strip-components=1

# Try barn images first, fall back to dock-pilot
IMAGES="${EXTRACT}/barn-images.tar.gz"
if [[ ! -f "$IMAGES" ]]; then
  IMAGES="${EXTRACT}/dock-pilot-images.tar.gz"
fi
[[ -f "$IMAGES" ]] || die "barn-images.tar.gz / dock-pilot-images.tar.gz missing in ${VERSION} release"

log "Loading Docker images (replaces :latest tags)..."
load_docker_images "$IMAGES"

# Copy barn compose files if available, fall back to dock-pilot
if [[ -f "${EXTRACT}/docker-compose.barn-full.yml" ]]; then
  cp "${EXTRACT}/docker-compose.barn-full.yml" "${ROOT}/docker-compose.barn-full.yml"
  log "Updated docker-compose.barn-full.yml"
elif [[ -f "${EXTRACT}/docker-compose.full.yml" ]]; then
  cp "${EXTRACT}/docker-compose.full.yml" "${ROOT}/docker-compose.full.yml"
  log "Updated docker-compose.full.yml"
fi
if [[ -d "${EXTRACT}/scripts" ]]; then
  cp -a "${EXTRACT}/scripts/." "${ROOT}/scripts/"
  chmod +x "${ROOT}/scripts/"*.sh 2>/dev/null || true
  log "Updated scripts/"
fi
if [[ -d "${EXTRACT}/install" ]]; then
  mkdir -p "${ROOT}/install"
  cp -a "${EXTRACT}/install/." "${ROOT}/install/"
  log "Updated install/ (nginx templates)"
fi
if [[ -f "${EXTRACT}/VERSION" ]]; then
  cp "${EXTRACT}/VERSION" "${ROOT}/VERSION"
  log "Updated VERSION → $(tr -d '[:space:]' < "${ROOT}/VERSION")"
fi

COMPOSE="docker-compose.barn-full.yml"
[[ -f "$COMPOSE" ]] || COMPOSE="docker-compose.full.yml"
[[ -f "$COMPOSE" ]] || COMPOSE="docker-compose.barn.yml"
[[ -f "$COMPOSE" ]] || COMPOSE="docker-compose.dock-pilot.yml"

log "Running migrations..."
if ! docker compose -f "$COMPOSE" run --rm -T migrate; then
  die "Migrations failed — check migrate image includes latest SQL (00016_fleet / 00017_*) and DATABASE_URL"
fi

log "Recreating postgres + api + frontend (picks up new images and compose)..."
# Clean up legacy and current containers
docker rm -f dock-pilot-telegram-socks-relay barn-telegram-socks-relay 2>/dev/null || true
docker compose -f "$COMPOSE" up -d --force-recreate postgres api frontend

if [[ -x "${ROOT}/scripts/configure-panel-nginx.sh" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "${ROOT}/.env"
  set +a
  if [[ -n "$DOMAIN" ]]; then
    [[ -n "$EMAIL" ]] || die "--email is required with --domain"
    log "Configuring panel domain and SSL..."
    NGINX_ARGS=(--domain "$DOMAIN" --email "$EMAIL")
    [[ "$SKIP_CERT" -eq 1 ]] && NGINX_ARGS+=(--skip-cert)
    bash "${ROOT}/scripts/configure-panel-nginx.sh" "${NGINX_ARGS[@]}"
  elif [[ -n "${PANEL_DOMAIN:-}" ]]; then
    log "Refreshing nginx panel config (domain from .env)..."
    bash "${ROOT}/scripts/configure-panel-nginx.sh" || log "WARN: configure-panel-nginx failed — check nginx manually"
  else
    # IP:port panel — still rewrite vhost so template fixes (e.g. client_max_body_size) apply.
    log "Refreshing nginx panel config (IP access)..."
    bash "${ROOT}/scripts/configure-panel-nginx.sh" || log "WARN: configure-panel-nginx failed — check nginx manually"
  fi
fi

log "Upgrade complete → ${VERSION}"
docker compose -f "$COMPOSE" ps
log "Check version in panel header (e.g. ${VERSION}) or: docker inspect barn-frontend --format '{{.Image}}' (or dock-pilot-frontend for legacy)"
