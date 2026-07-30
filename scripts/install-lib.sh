#!/usr/bin/env bash
# Shared helpers for install.sh (Barn)
set -euo pipefail

log() { echo "[barn] $*"; }
die() { echo "[barn] ERROR: $*" >&2; exit 1; }

write_credentials_file() {
  local install_dir="$1" panel_url="$2" api_token="$3"
  local cred="${install_dir}/credentials.txt"
  cat >"$cred" <<EOF
Barn (Амбар) — $(date -u +%Y-%m-%dT%H:%M:%SZ)

Panel URL:  ${panel_url}
API token:  ${api_token}
EOF
  chmod 600 "$cred"
  log "Saved credentials to ${cred}"
}

need_root() {
  if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
    die "Run as root: sudo $0 $*"
  fi
}

rand_secret() {
  local n="${1:-32}" raw
  if command -v openssl >/dev/null 2>&1; then
    raw="$(openssl rand -base64 64 | tr -dc 'A-Za-z0-9!@#%^&*-_=+')"
    echo -n "${raw:0:n}"
  else
  (
    set +o pipefail
    tr -dc 'A-Za-z0-9' </dev/urandom | head -c "$n"
  )
  fi
}

# Postgres password is embedded in DATABASE_URL — must be URL-safe (no @ : / # etc).
rand_postgres_password() {
  local n="${1:-24}" hex
  if command -v openssl >/dev/null 2>&1; then
    hex="$(openssl rand -hex "$(( (n + 1) / 2 ))")"
    echo -n "${hex:0:n}"
  else
  (
    set +o pipefail
    tr -dc 'A-Za-z0-9' </dev/urandom | head -c "$n"
  )
  fi
}

detect_os() {
  if [[ -f /etc/os-release ]]; then
    # shellcheck disable=SC1091
    source /etc/os-release
    echo "${ID:-unknown}"
  else
    echo unknown
  fi
}

# True when the host can run Barn without apt (docker compose + nginx + certbot).
host_prereqs_met() {
  command -v docker >/dev/null 2>&1 \
    && docker compose version >/dev/null 2>&1 \
    && command -v nginx >/dev/null 2>&1 \
    && command -v certbot >/dev/null 2>&1
}

ensure_docker_nginx_running() {
  systemctl enable docker nginx 2>/dev/null || true
  systemctl start docker nginx 2>/dev/null || true
}

apt_install() {
  # shellcheck disable=SC2068
  apt-get install -y -qq "$@" || {
    log "apt install failed for: $*"
    log "Try: apt --fix-broken install && apt-get update"
    log "Held packages: apt-mark showhold"
    log "If docker/nginx/certbot are already installed, re-run with --skip-packages"
    return 1
  }
}

# Many VPS have no IPv6 — default Ubuntu nginx listens on [::]:80 and fails to start.
ipv6_available() {
  [[ -f /proc/net/if_inet6 ]] || return 1
  [[ "$(cat /proc/sys/net/ipv6/conf/all/disable_ipv6 2>/dev/null)" == "1" ]] && return 1
  return 0
}

fix_nginx_no_ipv6() {
  if ipv6_available; then
    return 0
  fi
  local f
  log "IPv6 not available on this host — commenting out listen [::] in nginx configs"
  while IFS= read -r f; do
    [[ -f "$f" ]] || continue
    [[ "$f" == *".dpkg-"* ]] && continue
    if grep -qE '^\s*listen\s+\[::\]:' "$f" 2>/dev/null; then
      sed -i -E 's/^\s*listen\s+\[::\]:/# listen [::]:/' "$f"
      log "  patched: $f"
    fi
  done < <(find /etc/nginx -type f 2>/dev/null || true)
}

log_nginx_ipv6_listeners() {
  local hits
  hits="$(grep -rnE '^\s*listen\s+\[::\]:' /etc/nginx 2>/dev/null | head -5 || true)"
  if [[ -n "$hits" ]]; then
    log "Remaining listen [::] directives:"
    while IFS= read -r line; do
      [[ -n "$line" ]] && log "  $line"
    done <<<"$hits"
  fi
}

block_service_starts() {
  printf '%s\n' '#!/bin/sh' 'exit 101' > /usr/sbin/policy-rc.d
  chmod +x /usr/sbin/policy-rc.d
}

unblock_service_starts() {
  rm -f /usr/sbin/policy-rc.d
}

repair_apt_if_needed() {
  fix_nginx_no_ipv6
  if dpkg --audit 2>/dev/null | grep -q .; then
    log "Repairing interrupted apt/dpkg state..."
    block_service_starts
    apt-get install -y -f -qq 2>/dev/null || apt --fix-broken install -y -qq || true
    dpkg --configure -a 2>/dev/null || true
    unblock_service_starts
    fix_nginx_no_ipv6
  fi
}

install_nginx_package() {
  if command -v nginx >/dev/null 2>&1 \
    && nginx -t >/dev/null 2>&1; then
    fix_nginx_no_ipv6
    return 0
  fi

  if ipv6_available; then
    apt_install nginx
    return $?
  fi

  log "Installing nginx (deferring service start until [::] listeners are disabled)..."
  repair_apt_if_needed
  block_service_starts
  if ! apt_install nginx; then
    unblock_service_starts
    fix_nginx_no_ipv6
    block_service_starts
    apt-get install -y -f -qq 2>/dev/null || apt --fix-broken install -y -qq || true
    dpkg --configure -a 2>/dev/null || true
    apt_install nginx || {
      unblock_service_starts
      return 1
    }
  fi
  unblock_service_starts
  fix_nginx_no_ipv6
  if ! nginx -t; then
    log "nginx -t failed after IPv6 fix — check /etc/nginx"
    log_nginx_ipv6_listeners
    return 1
  fi
  systemctl enable nginx 2>/dev/null || true
  systemctl start nginx 2>/dev/null || true
  return 0
}

install_packages() {
  if host_prereqs_met; then
    log "docker, compose, nginx, and certbot already present — skipping apt"
    ensure_docker_nginx_running
    return 0
  fi

  local os
  os="$(detect_os)"
  case "$os" in
    ubuntu|debian)
      export DEBIAN_FRONTEND=noninteractive
      apt-get update -qq

      apt_install ca-certificates curl gnupg lsb-release || return 1

      repair_apt_if_needed

      if ! command -v nginx >/dev/null 2>&1 || ! nginx -t >/dev/null 2>&1; then
        install_nginx_package || return 1
      else
        log "nginx already installed — skipping"
        fix_nginx_no_ipv6
      fi

      if ! command -v certbot >/dev/null 2>&1; then
        apt_install certbot python3-certbot-nginx || return 1
      else
        log "certbot already installed — skipping"
      fi

      if ! command -v docker >/dev/null 2>&1; then
        apt_install docker.io || return 1
      else
        log "docker already installed — skipping"
      fi

      if ! docker compose version >/dev/null 2>&1; then
        apt_install docker-compose-plugin || apt_install docker-compose-v2 || return 1
      else
        log "docker compose already available — skipping"
      fi

      systemctl enable docker nginx 2>/dev/null || true
      ensure_docker_nginx_running
      ;;
    *)
      die "Unsupported OS: $os (need Ubuntu/Debian). Install docker, nginx, certbot manually."
      ;;
  esac
}

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

# Load barn-images.tar.gz or dock-pilot-images.tar.gz; show byte progress via pv when available.
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

github_latest_tag() {
  local repo="$1"
  curl -fsSL "https://api.github.com/repos/${repo}/releases/latest" \
    | grep -o '"tag_name": *"[^"]*"' | head -1 | cut -d'"' -f4
}

download_release() {
  local repo="$1" version="$2" dest="$3"
  local url
  if [[ "$version" == "latest" ]]; then
    version="$(github_latest_tag "$repo")"
    [[ -n "$version" ]] || die "Could not resolve latest release for ${repo}"
  fi
  # Try barn artifact first, fall back to dock-pilot for older releases
  url="https://github.com/${repo}/releases/download/${version}/barn-${version#v}.tar.gz"
  if ! download_with_progress "$url" "$dest" 2>/dev/null; then
    url="https://github.com/${repo}/releases/download/${version}/dock-pilot-${version#v}.tar.gz"
    download_with_progress "$url" "$dest"
  fi
}

NGINX_CONF="/etc/nginx/nginx.conf"
NGINX_HASH_BEGIN="# BEGIN barn nginx hash"
NGINX_HASH_END="# END barn nginx hash"
# Legacy markers for backward compat
NGINX_HASH_BEGIN_LEGACY="# BEGIN dock-pilot nginx hash"
NGINX_HASH_END_LEGACY="# END dock-pilot nginx hash"

remove_conf_d_hash_snippets() {
  rm -f /etc/nginx/conf.d/00-barn-global.conf /etc/nginx/conf.d/00-dockpilot-global.conf /etc/nginx/conf.d/00-vpsdeploy-global.conf 2>/dev/null || true
}

remove_nginx_hash_markers() {
  sed -i "/${NGINX_HASH_BEGIN}/,/${NGINX_HASH_END}/d" "$NGINX_CONF" 2>/dev/null || true
  sed -i "/${NGINX_HASH_BEGIN_LEGACY}/,/${NGINX_HASH_END_LEGACY}/d" "$NGINX_CONF" 2>/dev/null || true
}

remove_dockpilot_hash_snippets() {
  remove_conf_d_hash_snippets
  remove_nginx_hash_markers
}

comment_all_nginx_hash_directives() {
  local f
  for f in /etc/nginx/conf.d/*.conf; do
    [[ -f "$f" ]] || continue
    sed -i -E '/^[[:space:]]*#/! s/^[[:space:]]*(server_names_hash_(bucket_size|max_size)[^;]*;)/# \1/' "$f" 2>/dev/null || true
  done

  [[ -f "$NGINX_CONF" ]] || return 0
  # Check for barn or legacy dock-pilot markers
  if grep -qF "$NGINX_HASH_BEGIN" "$NGINX_CONF" 2>/dev/null || grep -qF "$NGINX_HASH_BEGIN_LEGACY" "$NGINX_CONF" 2>/dev/null; then
    awk -v b="$NGINX_HASH_BEGIN" -v e="$NGINX_HASH_END" -v bl="$NGINX_HASH_BEGIN_LEGACY" -v el="$NGINX_HASH_END_LEGACY" '
      index($0, b) || index($0, bl) { inb=1 }
      inb { print; if (index($0, e) || index($0, el)) inb=0; next }
      /^[ \t]*server_names_hash_(bucket_size|max_size)/ && $0 !~ /^[ \t]*#/ {
        match($0, /^[ \t]*/); print substr($0, 1, RSTART - 1) "# " substr($0, RSTART)
        next
      }
      { print }
    ' "$NGINX_CONF" > "${NGINX_CONF}.barn-tmp" && mv "${NGINX_CONF}.barn-tmp" "$NGINX_CONF"
  else
    sed -i -E '/^[[:space:]]*#/! s/^[[:space:]]*(server_names_hash_(bucket_size|max_size)[^;]*;)/# \1/' "$NGINX_CONF" 2>/dev/null || true
  fi
}

apply_nginx_hash_tuning() {
  local bucket="${1:-128}"
  local max_size="${2:-2048}"

  remove_dockpilot_hash_snippets
  comment_all_nginx_hash_directives

  sed -i "/^[[:space:]]*http[[:space:]]*{/a\\
    ${NGINX_HASH_BEGIN}\\
    server_names_hash_bucket_size ${bucket};\\
    server_names_hash_max_size ${max_size};\\
    ${NGINX_HASH_END}" "$NGINX_CONF"
}

ensure_nginx_config_valid() {
  local err
  remove_conf_d_hash_snippets
  comment_all_nginx_hash_directives
  err="$(mktemp)"
  if nginx -t 2>"$err"; then
    rm -f "$err"
    return 0
  fi
  if grep -q 'duplicate' "$err" 2>/dev/null && grep -q 'server_names_hash' "$err" 2>/dev/null; then
    log "Fixing duplicate server_names_hash directives..."
    remove_dockpilot_hash_snippets
    comment_all_nginx_hash_directives
    rm -f "$err"
    nginx -t
    return $?
  fi
  cat "$err" >&2
  rm -f "$err"
  return 1
}

write_nginx_global_tuning() {
  local domain="$1"
  local bucket=128
  local max_size=2048
  local len=${#domain}

  while (( bucket < len )); do
    bucket=$((bucket * 2))
  done

  apply_nginx_hash_tuning "$bucket" "$max_size"
  log "nginx hash tuning (bucket=${bucket}, max_size=${max_size}, domain=${domain})"
}

test_and_reload_nginx() {
  local domain="$1"
  local bucket=128
  local max_size=2048
  local len=${#domain}
  local attempt err

  while (( bucket < len )); do
    bucket=$((bucket * 2))
  done

  for attempt in 1 2 3 4 5; do
    apply_nginx_hash_tuning "$bucket" "$max_size"
    err="$(mktemp)"
    if nginx -t 2>"$err"; then
      rm -f "$err"
      systemctl reload nginx
      return 0
    fi
    if grep -q 'duplicate' "$err" 2>/dev/null && grep -q 'server_names_hash' "$err" 2>/dev/null; then
      log "Fixing duplicate server_names_hash (single block in nginx.conf) ..."
      remove_dockpilot_hash_snippets
      comment_all_nginx_hash_directives
      apply_nginx_hash_tuning "$bucket" "$max_size"
      rm -f "$err"
      continue
    fi
    if grep -q 'server_names_hash' "$err" 2>/dev/null; then
      log "nginx -t failed (server_names_hash bucket=${bucket}) — retrying with larger hash table ..."
      bucket=$((bucket * 2))
      max_size=$((max_size * 2))
      rm -f "$err"
      continue
    fi
    cat "$err" >&2
    rm -f "$err"
    return 1
  done
  die "nginx -t failed after tuning server_names_hash (last bucket=${bucket})"
}

write_panel_nginx() {
  local template="$1" domain="$2" api_port="$3" frontend_port="$4" out="$5"
  sed \
    -e "s/{{DOMAIN}}/${domain}/g" \
    -e "s/{{API_PORT}}/${api_port}/g" \
    -e "s/{{FRONTEND_PORT}}/${frontend_port}/g" \
    "$template" >"$out"
}

write_panel_nginx_ip() {
  local template="$1" panel_port="$2" api_port="$3" frontend_port="$4" out="$5"
  sed \
    -e "s/{{PANEL_HTTP_PORT}}/${panel_port}/g" \
    -e "s/{{API_PORT}}/${api_port}/g" \
    -e "s/{{FRONTEND_PORT}}/${frontend_port}/g" \
    "$template" >"$out"
}

detect_primary_ip() {
  local ip
  ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
  if [[ -z "$ip" ]]; then
    ip="$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for (i = 1; i <= NF; i++) if ($i == "src") { print $(i + 1); exit }}')"
  fi
  echo "$ip"
}

panel_cors_origins_ip() {
  local panel_port="$1"
  local ip origins=""
  ip="$(detect_primary_ip)"
  origins="http://127.0.0.1:${panel_port}"
  if [[ -n "$ip" ]]; then
    origins="http://${ip}:${panel_port},${origins}"
  fi
  echo "$origins"
}

panel_url_for_env() {
  local domain="$1" panel_port="$2" skip_cert="$3"
  if [[ -n "$domain" ]]; then
    if [[ "$skip_cert" -eq 1 ]]; then
      echo "http://${domain}"
    else
      echo "https://${domain}"
    fi
    return 0
  fi
  local ip
  ip="$(detect_primary_ip)"
  if [[ -n "$ip" ]]; then
    echo "http://${ip}:${panel_port}"
  else
    echo "http://127.0.0.1:${panel_port}"
  fi
}

disable_panel_nginx_site() {
  local enabled="$1" name="$2"
  rm -f "${enabled}/${name}" 2>/dev/null || true
}

enable_panel_nginx() {
  local available="$1" enabled="$2" domain="$3" name="barn-panel.conf"
  rm -f /etc/nginx/conf.d/00-vpsdeploy-global.conf /etc/nginx/conf.d/00-dockpilot-global.conf 2>/dev/null || true
  rm -f "${enabled}/default" "${enabled}/default.conf" 2>/dev/null || true
  disable_panel_nginx_site "$enabled" "barn-panel-ip.conf"
  disable_panel_nginx_site "$enabled" "dockpilot-panel-ip.conf"
  disable_panel_nginx_site "$enabled" "dockpilot-panel.conf"
  fix_nginx_no_ipv6
  # Try barn first, fall back to legacy
  if [[ -f "$available/$name" ]]; then
    ln -sf "$available/$name" "$enabled/$name"
  elif [[ -f "$available/dockpilot-panel.conf" ]]; then
    ln -sf "$available/dockpilot-panel.conf" "$enabled/dockpilot-panel.conf"
  fi
  test_and_reload_nginx "$domain"
}

enable_panel_nginx_ip() {
  local available="$1" enabled="$2" name="barn-panel-ip.conf"
  rm -f /etc/nginx/conf.d/00-vpsdeploy-global.conf /etc/nginx/conf.d/00-dockpilot-global.conf 2>/dev/null || true
  disable_panel_nginx_site "$enabled" "barn-panel.conf"
  disable_panel_nginx_site "$enabled" "dockpilot-panel.conf"
  disable_panel_nginx_site "$enabled" "dockpilot-panel-ip.conf"
  fix_nginx_no_ipv6
  # Try barn first, fall back to legacy
  if [[ -f "$available/$name" ]]; then
    ln -sf "$available/$name" "$enabled/$name"
  elif [[ -f "$available/dockpilot-panel-ip.conf" ]]; then
    ln -sf "$available/dockpilot-panel-ip.conf" "$enabled/dockpilot-panel-ip.conf"
  fi
  nginx -t
  systemctl reload nginx
}

issue_panel_cert() {
  local domain="$1" email="$2"
  remove_conf_d_hash_snippets
  comment_all_nginx_hash_directives
  ensure_nginx_config_valid || die "nginx -t failed before certbot — fix /etc/nginx manually"
  certbot --nginx -d "$domain" --non-interactive --agree-tos -m "$email" --redirect --no-eff-email
}

wait_for_api() {
  local port="$1" tries=60
  while ((tries-- > 0)); do
    if curl -fsS "http://127.0.0.1:${port}/health" >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  return 1
}

postgres_container_health() {
  # Try barn container first, then legacy
  docker inspect --format='{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' barn-postgres 2>/dev/null \
    || docker inspect --format='{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' dock-pilot-postgres 2>/dev/null \
    || echo "missing"
}

wait_for_postgres() {
  local tries=45
  local health container_name="barn-postgres"
  
  # Detect which container we're using
  if ! docker inspect barn-postgres >/dev/null 2>&1; then
    container_name="dock-pilot-postgres"
  fi
  
  health="$(postgres_container_health)"
  if [[ "$health" == "healthy" ]]; then
    return 0
  fi

  while ((tries-- > 0)); do
    health="$(postgres_container_health)"
    if [[ "$health" == "healthy" ]]; then
      return 0
    fi
    if [[ "$health" == "none" ]] && docker inspect --format='{{.State.Running}}' "$container_name" 2>/dev/null | grep -q true; then
      return 0
    fi
    if (( tries % 10 == 9 )); then
      log "Still waiting for PostgreSQL (${tries} checks left, health=${health})..."
    fi
    sleep 2
  done
  return 1
}

# True if something listens on this TCP port (any interface).
port_in_use() {
  local port="$1"
  ss -tln 2>/dev/null | awk '{print $4}' | grep -qE ":${port}$"
}

# Return the first free port starting at preferred (checks up to 100 candidates).
pick_free_port() {
  local preferred="$1"
  local p="$preferred"
  local limit=$((preferred + 100))
  while ((p < limit)); do
    if ! port_in_use "$p"; then
      echo "$p"
      return 0
    fi
    ((p++))
  done
  die "No free host port near ${preferred} (needed for Barn)"
}

postgres_volume_exists() {
  # Check barn volumes first, then legacy dock-pilot
  docker volume inspect barn_pg >/dev/null 2>&1 \
    || docker volume inspect dock-pilot_dock_pilot_pg >/dev/null 2>&1 \
    || docker volume inspect dock_pilot_pg >/dev/null 2>&1
}

# Write HTTP vhost, reload nginx, optionally issue Let's Encrypt for the panel domain.
configure_panel_nginx() {
  local install_dir="$1" domain="$2" email="$3" api_port="$4" frontend_port="$5" skip_cert="$6"
  local panel_port="${7:-8888}"

  if [[ -z "$domain" ]]; then
    configure_panel_nginx_ip "$install_dir" "$api_port" "$frontend_port" "$panel_port"
    return 0
  fi

  [[ -n "$email" ]] || die "CERTBOT_EMAIL is required when PANEL_DOMAIN is set"

  local template="${install_dir}/install/nginx-panel.conf.template"
  local available="/etc/nginx/sites-available/barn-panel.conf"
  
  # Fall back to legacy if barn template doesn't exist
  [[ -f "$template" ]] || die "Missing ${template}"

  log "Writing panel nginx config for ${domain} (api=${api_port}, ui=${frontend_port}) ..."
  write_nginx_global_tuning "$domain"
  write_panel_nginx "$template" "$domain" "$api_port" "$frontend_port" "$available"
  log "Enabling nginx site and reloading ..."
  enable_panel_nginx /etc/nginx/sites-available /etc/nginx/sites-enabled "$domain"

  if ! curl -fsS -H "Host: ${domain}" "http://127.0.0.1/" >/dev/null 2>&1; then
    log "WARN: HTTP probe for Host: ${domain} failed — check DNS and nginx"
  fi

  if [[ "$skip_cert" -eq 1 ]]; then
    log "Skipping TLS (--skip-cert)"
    return 0
  fi

  log "Issuing Let's Encrypt certificate for ${domain} ..."
  if ! issue_panel_cert "$domain" "$email"; then
    die "certbot failed for ${domain}. Check DNS → this VPS, ports 80/443 open, and: certbot --nginx -d ${domain}"
  fi

  log "TLS certificate installed for ${domain}"
}

configure_panel_nginx_ip() {
  local install_dir="$1" api_port="$2" frontend_port="$3" panel_port="$4"
  local template="${install_dir}/install/nginx-panel-ip.conf.template"
  local available="/etc/nginx/sites-available/barn-panel-ip.conf"
  
  # Fall back to legacy if barn template doesn't exist
  [[ -f "$template" ]] || die "Missing ${template}"

  log "Writing panel nginx config for IP access on port ${panel_port} (api=${api_port}, ui=${frontend_port}) ..."
  write_panel_nginx_ip "$template" "$panel_port" "$api_port" "$frontend_port" "$available"
  log "Enabling nginx site and reloading ..."
  enable_panel_nginx_ip /etc/nginx/sites-available /etc/nginx/sites-enabled

  if ! curl -fsS "http://127.0.0.1:${panel_port}/" >/dev/null 2>&1; then
    log "WARN: HTTP probe on 127.0.0.1:${panel_port} failed — check nginx"
  fi

  log "Panel available at $(panel_url_for_env "" "$panel_port" 1) (no TLS)"
}

pick_panel_http_port() {
  local port="${1:-8888}"
  if port_in_use "$port"; then
    port="$(pick_free_port "$((port + 1))")"
    log "Panel port busy — using ${port}"
  fi
  echo "$port"
}

verify_panel_https() {
  local domain="$1"
  curl -fsS "https://${domain}/" >/dev/null 2>&1
}
