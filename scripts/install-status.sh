#!/usr/bin/env bash
# Print Barn install status on VPS (run from /opt/barn).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

COMPOSE=docker-compose.barn.yml
[[ -f docker-compose.barn.yml ]] || COMPOSE=docker-compose.barn-full.yml
[[ -f "$COMPOSE" ]] || COMPOSE=docker-compose.full.yml
[[ -f "$COMPOSE" ]] || COMPOSE=docker-compose.dock-pilot.yml

echo "=== Barn status ==="
echo "Directory: ${ROOT}"
echo

if [[ -f .env ]]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
  echo "PANEL_DOMAIN=${PANEL_DOMAIN:-(IP access)}"
  echo "PANEL_HTTP_PORT=${PANEL_HTTP_PORT:-8888}"
  echo "API_PORT=${API_PORT:-8080}  FRONTEND_PORT=${FRONTEND_PORT:-3000}"
else
  echo "MISSING .env"
fi
echo

echo "--- docker compose ps ---"
docker compose -f "$COMPOSE" ps -a 2>&1 || true
echo

API_PORT="${API_PORT:-8080}"
echo "--- API health (127.0.0.1:${API_PORT}) ---"
curl -sS "http://127.0.0.1:${API_PORT}/health" 2>&1 || echo "FAIL"
echo

FE="${FRONTEND_PORT:-3000}"
echo "--- Frontend (127.0.0.1:${FE}) ---"
curl -sS -o /dev/null -w "HTTP %{http_code}\n" "http://127.0.0.1:${FE}/" 2>&1 || echo "FAIL"
echo

echo "--- nginx panel config ---"
ls -la /etc/nginx/sites-enabled/barn-panel*.conf /etc/nginx/sites-enabled/dockpilot-panel*.conf 2>&1 || echo "not enabled"
grep -E 'server_name|proxy_pass|listen' /etc/nginx/sites-available/barn-panel*.conf /etc/nginx/sites-available/dockpilot-panel*.conf 2>/dev/null || echo "no config file"
echo

if [[ -n "${PANEL_DOMAIN:-}" ]]; then
  echo "--- HTTP panel (${PANEL_DOMAIN}) ---"
  curl -sS -o /dev/null -w "HTTP %{http_code}\n" -H "Host: ${PANEL_DOMAIN}" "http://127.0.0.1/" 2>&1 || true
  echo "--- HTTPS panel ---"
  curl -sSI "https://${PANEL_DOMAIN}/" 2>&1 | head -5 || true
else
  PANEL_HTTP_PORT="${PANEL_HTTP_PORT:-8888}"
  echo "--- HTTP panel (IP:${PANEL_HTTP_PORT}) ---"
  curl -sS -o /dev/null -w "HTTP %{http_code}\n" "http://127.0.0.1:${PANEL_HTTP_PORT}/" 2>&1 || true
fi
echo

echo "--- certbot ---"
certbot certificates 2>/dev/null | grep -A3 "${PANEL_DOMAIN:-panel}" || certbot certificates 2>/dev/null | tail -5 || true
echo

if [[ -f credentials.txt ]]; then
  echo "--- credentials.txt ---"
  cat credentials.txt
else
  echo "No credentials.txt (install did not finish)"
  [[ -f .env ]] && echo "API_TOKEN from .env: $(grep ^API_TOKEN= .env | cut -d= -f2-)"
fi
