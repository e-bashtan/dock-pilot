#!/usr/bin/env bash
# Configure host nginx for the Barn panel (install, upgrade, repair).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GITHUB_REPO="${BARN_GITHUB_REPO:-${DOCK_PILOT_GITHUB_REPO:-ebasht/barn}}"
SKIP_CERT=0
DOMAIN=""
EMAIL=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-cert) SKIP_CERT=1; shift ;;
    --domain) DOMAIN="$2"; shift 2 ;;
    --email) EMAIL="$2"; shift 2 ;;
    -h|--help)
      cat <<EOF
Usage: configure-panel-nginx.sh [options]

Reads PANEL_DOMAIN, PANEL_HTTP_PORT, API_PORT, FRONTEND_PORT, CERTBOT_EMAIL from .env.
Without PANEL_DOMAIN: HTTP on http://SERVER_IP:PANEL_HTTP_PORT (no TLS for the panel).

Options:
  --domain DOMAIN   Set panel domain (enables HTTPS via Let's Encrypt unless --skip-cert)
  --email EMAIL     Let's Encrypt email (required with --domain for TLS)
  --skip-cert       With --domain: HTTP only, no certificate for the panel
EOF
      exit 0
      ;;
    *) echo "Unknown option: $1" >&2; exit 1 ;;
  esac
done

if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
  echo "Run as root: sudo $0" >&2
  exit 1
fi

[[ -f .env ]] || { echo "Missing .env in ${ROOT}" >&2; exit 1; }
set -a
# shellcheck disable=SC1091
source .env
set +a

[[ -n "$DOMAIN" ]] && PANEL_DOMAIN="$DOMAIN"
[[ -n "$EMAIL" ]] && CERTBOT_EMAIL="$EMAIL"

API_PORT="${API_PORT:-8080}"
FRONTEND_PORT="${FRONTEND_PORT:-3000}"
PANEL_HTTP_PORT="${PANEL_HTTP_PORT:-8888}"

if [[ -n "$DOMAIN" ]]; then
  PANEL_DOMAIN="$DOMAIN"
  [[ -n "$CERTBOT_EMAIL" ]] || { echo "Set CERTBOT_EMAIL in .env or pass --email" >&2; exit 1; }
  grep -q '^PANEL_DOMAIN=' .env \
    && sed -i "s|^PANEL_DOMAIN=.*|PANEL_DOMAIN=${PANEL_DOMAIN}|" .env \
    || echo "PANEL_DOMAIN=${PANEL_DOMAIN}" >> .env
  grep -q '^CERTBOT_EMAIL=' .env \
    && sed -i "s|^CERTBOT_EMAIL=.*|CERTBOT_EMAIL=${CERTBOT_EMAIL}|" .env \
    || echo "CERTBOT_EMAIL=${CERTBOT_EMAIL}" >> .env
fi

tmp="$(mktemp)"
if [[ -f "${ROOT}/scripts/install-lib.sh" ]]; then
  # shellcheck source=scripts/install-lib.sh
  source "${ROOT}/scripts/install-lib.sh"
elif curl -fsSL "https://raw.githubusercontent.com/${GITHUB_REPO}/main/scripts/install-lib.sh" -o "$tmp" 2>/dev/null; then
  # shellcheck source=/dev/null
  source "$tmp"
else
  echo "install-lib.sh not found in ${ROOT}/scripts" >&2
  exit 1
fi
rm -f "$tmp"

if [[ -n "${PANEL_DOMAIN:-}" ]]; then
  configure_panel_nginx "$ROOT" "$PANEL_DOMAIN" "$CERTBOT_EMAIL" "$API_PORT" "$FRONTEND_PORT" "$SKIP_CERT"
  PANEL_URL="$(panel_url_for_env "$PANEL_DOMAIN" "$PANEL_HTTP_PORT" "$SKIP_CERT")"
  CORS_ORIGINS="$PANEL_URL"
else
  PANEL_HTTP_PORT="$(pick_panel_http_port "$PANEL_HTTP_PORT")"
  configure_panel_nginx_ip "$ROOT" "$API_PORT" "$FRONTEND_PORT" "$PANEL_HTTP_PORT"
  PANEL_URL="$(panel_url_for_env "" "$PANEL_HTTP_PORT" 1)"
  CORS_ORIGINS="$(panel_cors_origins_ip "$PANEL_HTTP_PORT")"
  grep -q '^PANEL_HTTP_PORT=' .env \
    && sed -i "s|^PANEL_HTTP_PORT=.*|PANEL_HTTP_PORT=${PANEL_HTTP_PORT}|" .env \
    || echo "PANEL_HTTP_PORT=${PANEL_HTTP_PORT}" >> .env
fi

sed -i "s|^CORS_ALLOWED_ORIGINS=.*|CORS_ALLOWED_ORIGINS=${CORS_ORIGINS}|" .env
docker compose -f docker-compose.full.yml up -d api 2>/dev/null \
  || docker compose -f docker-compose.dock-pilot.yml up -d api
log "Panel: ${PANEL_URL}"
[[ -n "${API_TOKEN:-}" ]] && write_credentials_file "$ROOT" "$PANEL_URL" "$API_TOKEN"
