#!/usr/bin/env bash
# One-command VPS install: Docker stack + nginx (+ Let's Encrypt when --domain is set).
# Barn (Амбар) - Docker deployment panel
#
#   sudo bash /tmp/barn-install.sh --domain deploy.example.com --email you@example.com
#   sudo bash /tmp/barn-install.sh --version latest
#
set -euo pipefail

INSTALL_DIR="${BARN_INSTALL_DIR:-${DOCK_PILOT_INSTALL_DIR:-/opt/barn}}"
GITHUB_REPO="${BARN_GITHUB_REPO:-${DOCK_PILOT_GITHUB_REPO:-ebasht/barn}}"
VERSION="${BARN_VERSION:-${DOCK_PILOT_VERSION:-latest}}"
DOMAIN=""
EMAIL=""
API_TOKEN=""
FROM_DIR=""
SKIP_CERT=0
SKIP_PACKAGES=0
RESET_DB=0

usage() {
  cat <<EOF
Usage: install.sh [options]

Panel access:
  --domain DOMAIN     Public hostname for the panel (DNS A/AAAA → this VPS, HTTPS via Let's Encrypt)
  --email EMAIL       Email for Let's Encrypt (required with --domain)

  Without --domain: panel on http://SERVER_IP:PANEL_HTTP_PORT (default 8888), no TLS for the panel.

Options:
  --token TOKEN       API token (generated if omitted)
  --install-dir DIR   Install path (default: /opt/barn, or /opt/dock-pilot if detected)
  --repo OWNER/REPO   GitHub repo (default: ebasht/barn)
  --version TAG       Release tag v0.1.0 or latest
  --from-dir DIR      Use an unpacked release directory
  --skip-cert         With --domain: HTTP only, no Let's Encrypt for the panel
  --skip-packages     Skip apt install of docker/nginx/certbot
  --reset-db          Wipe bundled Postgres volume and regenerate DB password
EOF
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --domain) DOMAIN="$2"; shift 2 ;;
      --email) EMAIL="$2"; shift 2 ;;
      --token) API_TOKEN="$2"; shift 2 ;;
      --install-dir) INSTALL_DIR="$2"; shift 2 ;;
      --repo) GITHUB_REPO="$2"; shift 2 ;;
      --version) VERSION="$2"; shift 2 ;;
      --from-dir) FROM_DIR="$2"; shift 2 ;;
      --skip-cert) SKIP_CERT=1; shift ;;
      --skip-packages) SKIP_PACKAGES=1; shift ;;
      --reset-db) RESET_DB=1; shift ;;
      -h|--help) usage; exit 0 ;;
      *) echo "Unknown option: $1" >&2; usage; exit 1 ;;
    esac
  done
}

parse_args "$@"

download_with_progress() {
  local url="$1" dest="$2"
  local name cl size_human="" show_progress=0

  name="$(basename "$dest")"
  cl="$(curl -fsSLI -L "$url" 2>/dev/null | awk 'tolower($1)=="content-length:" {print $2; exit}' | tr -d '\r' || true)"
  if [[ -n "$cl" && "$cl" =~ ^[0-9]+$ ]]; then
    size_human="$(numfmt --to=iec-i --suffix=B "$cl" 2>/dev/null || echo "${cl} B")"
  fi

  if [[ -t 1 || -t 2 || -n "${DOCK_PILOT_FORCE_PROGRESS:-}" ]]; then
    show_progress=1
  fi

  if [[ -n "$size_human" ]]; then
    echo "[dock-pilot] Downloading ${name} (~${size_human})..."
  else
    echo "[dock-pilot] Downloading ${name}..."
  fi

  if [[ "$show_progress" -eq 1 ]]; then
    if ! curl -fL --progress-bar --stderr - "$url" -o "$dest"; then
      return 1
    fi
    echo ""
    echo "[dock-pilot] Download complete: ${name}"
    return 0
  fi

  echo "[dock-pilot] No TTY — showing progress every 5s..."
  curl -fsSL "$url" -o "$dest.part" &
  local pid=$!
  while kill -0 "$pid" 2>/dev/null; do
    if [[ -f "$dest.part" ]]; then
      local got
      got="$(stat -c%s "$dest.part" 2>/dev/null || stat -f%z "$dest.part" 2>/dev/null || echo 0)"
      if [[ -n "$cl" && "$cl" =~ ^[0-9]+$ && "$cl" -gt 0 ]]; then
        local pct=$((got * 100 / cl))
        echo "[dock-pilot]   ${got} / ${cl} bytes (${pct}%)"
      else
        echo "[dock-pilot]   ${got} bytes downloaded..."
      fi
    else
      echo "[dock-pilot]   connecting..."
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
  echo "[dock-pilot] Download complete: ${name}"
}

load_docker_images() {
  local images="$1"
  log "Loading Docker images from $(basename "$images")..."
  if [[ -t 1 || -t 2 || -n "${DOCK_PILOT_FORCE_PROGRESS:-}" ]] && command -v pv >/dev/null 2>&1; then
    pv -f -pte "$images" | gunzip -c | docker load
  else
    if [[ -t 1 || -t 2 || -n "${DOCK_PILOT_FORCE_PROGRESS:-}" ]] && ! command -v pv >/dev/null 2>&1; then
      log "Tip: apt install pv for load progress (percent bar)"
    fi
    gunzip -c "$images" | docker load
  fi
}

# curl | bash: docker compose run reads stdin and consumes the rest of this script.
if [[ -p /dev/stdin && -z "${BARN_INSTALL_REEXECED:-${DOCK_PILOT_INSTALL_REEXECED:-}}" ]]; then
  export BARN_INSTALL_REEXECED=1
  _install_copy="$(mktemp)"
  cat >"$_install_copy"
  chmod 700 "$_install_copy"
  exec bash "$_install_copy" "$@"
fi

# --- Bootstrap: download release when not yet on disk (curl | bash) ---
# Check for barn-full or dock-pilot-full compose file
if [[ -z "$FROM_DIR" && ! -f "${INSTALL_DIR}/docker-compose.barn-full.yml" && ! -f "${INSTALL_DIR}/docker-compose.full.yml" ]]; then
  if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
    echo "[barn] ERROR: Run as root: sudo bash -s -- ..." >&2
    exit 1
  fi
  [[ -n "$DOMAIN" && -z "$EMAIL" ]] && { usage; echo "[barn] ERROR: --email is required with --domain" >&2; exit 1; }

  if [[ "$VERSION" == "latest" ]]; then
    VERSION="$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" \
      | grep -o '"tag_name": *"[^"]*"' | head -1 | cut -d'"' -f4)"
    [[ -n "$VERSION" ]] || { echo "No GitHub release found for ${GITHUB_REPO}" >&2; exit 1; }
  fi
  FILE_TAG="${VERSION#v}"
  # Try barn artifact first, fall back to dock-pilot for older releases
  URL="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/barn-${FILE_TAG}.tar.gz"
  echo "[barn] Downloading ${URL} ..."
  mkdir -p "$INSTALL_DIR"
  if ! download_with_progress "$URL" "${INSTALL_DIR}/.barn-release.tar.gz" 2>/dev/null; then
    URL="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/dock-pilot-${FILE_TAG}.tar.gz"
    echo "[barn] Trying legacy artifact ${URL} ..."
    download_with_progress "$URL" "${INSTALL_DIR}/.barn-release.tar.gz"
  fi
  tar -xzf "${INSTALL_DIR}/.barn-release.tar.gz" -C "$INSTALL_DIR" --strip-components=1
  rm -f "${INSTALL_DIR}/.barn-release.tar.gz"
  FROM_DIR="$INSTALL_DIR"
fi

SCRIPT_ROOT="${FROM_DIR:-$INSTALL_DIR}"
# Check for barn or legacy compose files
if [[ ! -f "${SCRIPT_ROOT}/docker-compose.barn-full.yml" && ! -f "${SCRIPT_ROOT}/docker-compose.full.yml" ]]; then
  echo "[barn] ERROR: docker-compose.barn-full.yml or docker-compose.full.yml not found in ${SCRIPT_ROOT}" >&2
  echo "Publish a GitHub release first, or use --from-dir with a local bundle." >&2
  exit 1
fi

# Release tarballs ship frozen scripts; pull latest installer helpers from main when online.
source_install_lib() {
  local bundled="${SCRIPT_ROOT}/scripts/install-lib.sh"
  local api_url="https://api.github.com/repos/${GITHUB_REPO}/contents/scripts/install-lib.sh?ref=main"
  local raw_url="https://raw.githubusercontent.com/${GITHUB_REPO}/main/scripts/install-lib.sh?$(date +%s)"
  local tmp
  tmp="$(mktemp)"
  if curl -fsSL -H "Accept: application/vnd.github.raw+json" "$api_url" -o "$tmp" 2>/dev/null && [[ -s "$tmp" ]]; then
    # shellcheck source=/dev/null
    source "$tmp"
    rm -f "$tmp"
    log "Installer helpers: ${GITHUB_REPO}@main (GitHub API)"
  elif curl -fsSL "$raw_url" -o "$tmp" 2>/dev/null && [[ -s "$tmp" ]]; then
    # shellcheck source=/dev/null
    source "$tmp"
    rm -f "$tmp"
    log "Installer helpers: ${GITHUB_REPO}@main"
  elif [[ -f "$bundled" ]]; then
    # shellcheck source=/dev/null
    source "$bundled"
    log "Installer helpers: bundled in ${SCRIPT_ROOT} (offline or fetch failed)"
  else
    die "install-lib.sh not found"
  fi
}

source_install_lib

# GitHub raw CDN can lag; never use `docker compose exec` here (hangs on some hosts).
wait_for_postgres() {
  local tries=30 health running
  for ((tries=30; tries>0; tries--)); do
    health="$(docker inspect --format='{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' dock-pilot-postgres 2>/dev/null || echo missing)"
    if [[ "$health" == "healthy" ]]; then
      return 0
    fi
    running="$(docker inspect --format='{{.State.Running}}' dock-pilot-postgres 2>/dev/null || echo false)"
    if [[ "$running" == "true" && "$health" != "missing" && "$health" != "unhealthy" ]]; then
      return 0
    fi
    sleep 1
  done
  return 1
}

refresh_install_files() {
  local base="https://raw.githubusercontent.com/${GITHUB_REPO}/main"
  local cache="?$(date +%s)"
  local tmp
  tmp="$(mktemp)"
  # Try barn-full first, then fall back to full
  if curl -fsSL "${base}/docker-compose.barn-full.yml${cache}" -o "$tmp" 2>/dev/null && [[ -s "$tmp" ]]; then
    cp "$tmp" "${INSTALL_DIR}/docker-compose.barn-full.yml"
    log "Updated docker-compose.barn-full.yml from ${GITHUB_REPO}@main"
  elif curl -fsSL "${base}/docker-compose.full.yml${cache}" -o "$tmp" 2>/dev/null && [[ -s "$tmp" ]]; then
    cp "$tmp" "${INSTALL_DIR}/docker-compose.full.yml"
    log "Updated docker-compose.full.yml from ${GITHUB_REPO}@main"
  fi
  rm -f "$tmp"
  tmp="$(mktemp)"
  if curl -fsSL "${base}/install/nginx-panel.conf.template${cache}" -o "$tmp" 2>/dev/null && [[ -s "$tmp" ]]; then
    mkdir -p "${INSTALL_DIR}/install"
    cp "$tmp" "${INSTALL_DIR}/install/nginx-panel.conf.template"
  fi
  if curl -fsSL "${base}/install/nginx-panel-ip.conf.template${cache}" -o "$tmp" 2>/dev/null && [[ -s "$tmp" ]]; then
    mkdir -p "${INSTALL_DIR}/install"
    cp "$tmp" "${INSTALL_DIR}/install/nginx-panel-ip.conf.template"
  fi
  rm -f "$tmp"
}

[[ -n "$DOMAIN" && -z "$EMAIL" ]] && { usage; die "--email is required with --domain"; }
need_root

if [[ "$SKIP_PACKAGES" -eq 0 ]]; then
  log "Installing system packages (docker, nginx, certbot)..."
  if ! install_packages; then
    if host_prereqs_met; then
      log "WARN: apt failed but docker, compose, nginx, and certbot are already installed — continuing"
    else
      die "apt install failed. Fix conflicts (apt --fix-broken install; apt-mark showhold) or install prerequisites manually, then re-run with --skip-packages"
    fi
  fi
fi

command -v docker >/dev/null || die "docker not found"
docker compose version >/dev/null 2>&1 || die "docker compose plugin not found"
command -v nginx >/dev/null || die "nginx not found (install nginx or fix apt, then re-run)"
command -v certbot >/dev/null || die "certbot not found (install certbot or re-run with working apt)"

mkdir -p "$INSTALL_DIR"
if [[ -n "$FROM_DIR" && "$FROM_DIR" != "$INSTALL_DIR" ]]; then
  cp -a "${FROM_DIR}/." "$INSTALL_DIR/"
fi
cd "$INSTALL_DIR"
refresh_install_files

IMAGES=""
# Try barn images first, then fall back to dock-pilot
for f in barn-images.tar.gz dist/barn-images.tar.gz dock-pilot-images.tar.gz dist/dock-pilot-images.tar.gz; do
  [[ -f "$f" ]] && IMAGES="$f" && break
done
[[ -n "$IMAGES" ]] || die "barn-images.tar.gz or dock-pilot-images.tar.gz not found"

# Check for barn or legacy images
if docker image inspect barn-api:latest >/dev/null 2>&1 \
  && docker image inspect barn-frontend:latest >/dev/null 2>&1 \
  && docker image inspect barn-migrate:latest >/dev/null 2>&1 \
  && docker image inspect barn-postgres:latest >/dev/null 2>&1; then
  log "Barn Docker images already loaded — skipping docker load"
elif docker image inspect dock-pilot-api:latest >/dev/null 2>&1 \
  && docker image inspect dock-pilot-frontend:latest >/dev/null 2>&1 \
  && docker image inspect dock-pilot-migrate:latest >/dev/null 2>&1 \
  && docker image inspect dock-pilot-postgres:latest >/dev/null 2>&1; then
  log "Legacy Docker images already loaded — skipping docker load"
else
  load_docker_images "$IMAGES"
fi
log "Docker images ready"

# Re-run must not rotate Postgres password: the data volume keeps the first password.
if [[ -f .env ]]; then
  log "Reusing secrets from existing .env (Postgres volume keeps its password)"
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

if [[ "$RESET_DB" -eq 1 ]]; then
  log "Resetting Postgres volume (--reset-db) ..."
  # Try both barn and legacy compose files
  docker compose -f docker-compose.barn-full.yml down 2>/dev/null || docker compose -f docker-compose.full.yml down 2>/dev/null || true
  for vol in barn_pg barn_barn_pg dock-pilot_dock_pilot_pg dock_pilot_pg; do
    docker volume rm "$vol" 2>/dev/null || true
  done
  unset POSTGRES_PASSWORD DATABASE_URL
fi

if postgres_volume_exists && [[ -z "${POSTGRES_PASSWORD:-}" ]]; then
  die "Postgres volume exists but POSTGRES_PASSWORD missing in .env — re-run with --reset-db or restore .env"
fi

log "Generating secrets and writing .env ..."
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-$(rand_postgres_password 24)}"
SECRETS_KEY="${SECRETS_ENCRYPTION_KEY:-$(rand_secret 32)}"
API_TOKEN="${API_TOKEN:-$(rand_secret 32)}"

# Use barn naming for new installs, detect legacy for upgrades
POSTGRES_USER="barn"
POSTGRES_DB="barn"
POSTGRES_IMAGE="barn-postgres:latest"
API_IMAGE="barn-api:latest"
FRONTEND_IMAGE="barn-frontend:latest"
MIGRATE_IMAGE="barn-migrate:latest"
DEPLOY_WORK_DIR="/var/lib/barn"

# Detect legacy install - if dock-pilot images exist, keep using them
if docker volume inspect dock-pilot_dock_pilot_pg >/dev/null 2>&1 || docker volume inspect dock_pilot_pg >/dev/null 2>&1; then
  log "Detected legacy dock-pilot installation - preserving naming"
  POSTGRES_USER="dockpilot"
  POSTGRES_DB="dockpilot"
  POSTGRES_IMAGE="dock-pilot-postgres:latest"
  API_IMAGE="dock-pilot-api:latest"
  FRONTEND_IMAGE="dock-pilot-frontend:latest"
  MIGRATE_IMAGE="dock-pilot-migrate:latest"
  DEPLOY_WORK_DIR="/var/lib/dock-pilot"
fi

# Pick free localhost ports (8080/3000 are often taken on busy VPS hosts).
if port_in_use "${API_PORT:-8080}"; then
  NEW_API="$(pick_free_port "${API_PORT:-8080}")"
  log "Host port ${API_PORT:-8080} busy — using ${NEW_API} for API"
  API_PORT="$NEW_API"
else
  API_PORT="${API_PORT:-8080}"
fi
if port_in_use "${FRONTEND_PORT:-3000}"; then
  NEW_FE="$(pick_free_port "${FRONTEND_PORT:-3000}")"
  log "Host port ${FRONTEND_PORT:-3000} busy — using ${NEW_FE} for frontend"
  FRONTEND_PORT="$NEW_FE"
else
  FRONTEND_PORT="${FRONTEND_PORT:-3000}"
fi

PANEL_HTTP_PORT="${PANEL_HTTP_PORT:-8888}"
if [[ -z "$DOMAIN" ]]; then
  PANEL_HTTP_PORT="$(pick_panel_http_port "$PANEL_HTTP_PORT")"
  PANEL_URL="$(panel_url_for_env "" "$PANEL_HTTP_PORT" 1)"
  CORS_ORIGINS="$(panel_cors_origins_ip "$PANEL_HTTP_PORT")"
else
  PANEL_URL="$(panel_url_for_env "$DOMAIN" "$PANEL_HTTP_PORT" "$SKIP_CERT")"
  CORS_ORIGINS="$PANEL_URL"
fi

cat > .env <<EOF
# Generated by install.sh on $(date -u +%Y-%m-%dT%H:%M:%SZ)
POSTGRES_USER=${POSTGRES_USER}
POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
POSTGRES_DB=${POSTGRES_DB}
POSTGRES_IMAGE=${POSTGRES_IMAGE}
DATABASE_URL=postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@postgres:5432/${POSTGRES_DB}?sslmode=disable
POSTGRES_HOST_PORT=5433

HTTP_ADDR=:8080
SECRETS_ENCRYPTION_KEY=${SECRETS_KEY}
API_TOKEN=${API_TOKEN}

PANEL_DOMAIN=${DOMAIN}
PANEL_HTTP_PORT=${PANEL_HTTP_PORT}
CORS_ALLOWED_ORIGINS=${CORS_ORIGINS}

DEPLOY_MODE=real
DEPLOY_WORK_DIR=${DEPLOY_WORK_DIR}
HOST_ROOT=/host
NGINX_SITES_AVAILABLE=/host/etc/nginx/sites-available
NGINX_SITES_ENABLED=/host/etc/nginx/sites-enabled
CERTBOT_EMAIL=${EMAIL}

API_PORT=${API_PORT}
FRONTEND_PORT=${FRONTEND_PORT}

API_IMAGE=${API_IMAGE}
FRONTEND_IMAGE=${FRONTEND_IMAGE}
MIGRATE_IMAGE=${MIGRATE_IMAGE}
EOF
chmod 600 .env
log "Wrote ${INSTALL_DIR}/.env (api=${API_PORT}, ui=${FRONTEND_PORT})"
write_credentials_file "$INSTALL_DIR" "$PANEL_URL" "$API_TOKEN"

run_migrate() {
  log "Applying migrations..."
  local compose_file="docker-compose.barn-full.yml"
  [[ -f "$compose_file" ]] || compose_file="docker-compose.full.yml"
  set +e
  docker compose -f "$compose_file" run --rm -T migrate
  local rc=$?
  set -e
  if [[ $rc -eq 0 ]]; then
    log "Migrations applied"
    return 0
  fi
  # goose/docker compose often exit non-zero when schema is already current
  log "Migrate exited with code ${rc} — continuing (schema may already be up to date)"
  return 0
}

start_api_frontend() {
  local compose_file="docker-compose.barn-full.yml"
  [[ -f "$compose_file" ]] || compose_file="docker-compose.full.yml"
  local attempt
  for attempt in 1 2 3 4 5; do
    log "Starting API and frontend (attempt ${attempt}, api=${API_PORT}, ui=${FRONTEND_PORT}) ..."
    docker rm -f barn-api barn-frontend dock-pilot-api dock-pilot-frontend 2>/dev/null || true
    if docker compose -f "$compose_file" up -d api frontend --no-deps 2>&1; then
      return 0
    fi
    if port_in_use "$API_PORT"; then
      API_PORT="$(pick_free_port "$((API_PORT + 1))")"
      sed -i "s/^API_PORT=.*/API_PORT=${API_PORT}/" .env
      log "Port busy — retrying API on ${API_PORT}"
      continue
    fi
    if port_in_use "$FRONTEND_PORT"; then
      FRONTEND_PORT="$(pick_free_port "$((FRONTEND_PORT + 1))")"
      sed -i "s/^FRONTEND_PORT=.*/FRONTEND_PORT=${FRONTEND_PORT}/" .env
      log "Port busy — retrying frontend on ${FRONTEND_PORT}"
      continue
    fi
    log "docker compose up failed:"
    docker compose -f "$compose_file" logs api --tail 30 2>&1 || true
    return 1
  done
  die "Could not start API/frontend after 5 attempts (check: docker compose logs api)"
}

COMPOSE_FILE="docker-compose.barn-full.yml"
[[ -f "$COMPOSE_FILE" ]] || COMPOSE_FILE="docker-compose.full.yml"

log "Starting stack (postgres → migrate → api → frontend)..."
docker compose -f "$COMPOSE_FILE" up -d postgres
log "Waiting for PostgreSQL..."
if wait_for_postgres; then
  log "PostgreSQL is ready"
else
  docker compose -f "$COMPOSE_FILE" ps -a 2>&1 || true
  docker compose -f "$COMPOSE_FILE" logs postgres --tail 30 2>&1 || true
  die "PostgreSQL did not become ready"
fi
run_migrate
start_api_frontend || die "API/frontend failed to start"

log "Waiting for API on 127.0.0.1:${API_PORT}/health ..."
if ! wait_for_api "$API_PORT"; then
  log "API not healthy — last logs:"
  docker compose -f docker-compose.full.yml logs api --tail 40 2>&1 || true
  die "API not healthy on 127.0.0.1:${API_PORT}"
fi

if [[ -n "$DOMAIN" ]]; then
  configure_panel_nginx "$INSTALL_DIR" "$DOMAIN" "$EMAIL" "$API_PORT" "$FRONTEND_PORT" "$SKIP_CERT"
  PANEL_URL="$(panel_url_for_env "$DOMAIN" "$PANEL_HTTP_PORT" "$SKIP_CERT")"
  sed -i "s|^CORS_ALLOWED_ORIGINS=.*|CORS_ALLOWED_ORIGINS=${PANEL_URL}|" .env
  docker compose -f "$COMPOSE_FILE" up -d api

  if [[ "$SKIP_CERT" -eq 0 ]]; then
    if verify_panel_https "$DOMAIN"; then
      log "HTTPS verified: https://${DOMAIN}/"
    else
      log "WARN: HTTPS probe failed for ${DOMAIN} — panel may still work; check: certbot certificates"
    fi
  fi
else
  configure_panel_nginx_ip "$INSTALL_DIR" "$API_PORT" "$FRONTEND_PORT" "$PANEL_HTTP_PORT"
  PANEL_URL="$(panel_url_for_env "" "$PANEL_HTTP_PORT" 1)"
  CORS_ORIGINS="$(panel_cors_origins_ip "$PANEL_HTTP_PORT")"
  sed -i "s|^CORS_ALLOWED_ORIGINS=.*|CORS_ALLOWED_ORIGINS=${CORS_ORIGINS}|" .env
  docker compose -f "$COMPOSE_FILE" up -d api
  log "Panel URL: ${PANEL_URL} (open port ${PANEL_HTTP_PORT} in firewall if needed)"
fi

CREDENTIALS="${INSTALL_DIR}/credentials.txt"
write_credentials_file "$INSTALL_DIR" "$PANEL_URL" "$API_TOKEN"

cat <<EOF

================================================================================
  Barn (Амбар) is ready.

  Panel:     ${PANEL_URL}
  API token: ${API_TOKEN}

  Saved to:  ${CREDENTIALS}

  1. Open the panel and enter the API token.
  2. Add a site with your app domain (DNS must point here).
  3. Deploy — SSL for app domains is issued automatically.
$(if [[ -z "$DOMAIN" ]]; then
  echo ""
  echo "  To add HTTPS for the panel later:"
  echo "    sudo bash ${INSTALL_DIR}/scripts/barn-upgrade.sh latest \\"
  echo "      --domain panel.example.com --email you@example.com"
fi)

  cd ${INSTALL_DIR} && docker compose -f ${COMPOSE_FILE} ps
================================================================================

EOF
