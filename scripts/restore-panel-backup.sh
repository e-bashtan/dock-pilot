#!/usr/bin/env bash
# Restore a DockPilot full snapshot (tar.gz from S3) onto a VPS after install.sh.
#
# Bundle layout (created by panel backup):
#   manifest.json
#   secrets.env
#   panel/dockpilot.sql.gz
#   managed/<db>.sql.gz
#
# Typical flow on a new VPS:
#   1. sudo bash scripts/install.sh --domain ... --email ...
#   2. sudo bash scripts/restore-panel-backup.sh --s3-uri s3://bucket/prefix/full/dock-pilot-….tar.gz
#   3. Redeploy sites from git in the panel (volumes are not in the backup).
#
# Requires: docker, gunzip, tar. Download via aws CLI, yc, or --file.
set -euo pipefail

INSTALL_DIR="${DOCK_PILOT_INSTALL_DIR:-/opt/dock-pilot}"
COMPOSE_FILE="${DOCK_PILOT_COMPOSE:-docker-compose.dock-pilot.yml}"
PANEL_PG_CONTAINER="${PANEL_POSTGRES_CONTAINER:-dock-pilot-postgres}"
MANAGED_PG_CONTAINER="${MANAGED_POSTGRES_CONTAINER:-dockpilot-postgres}"

S3_URI=""
LOCAL_FILE=""
S3_ENDPOINT=""
AWS_PROFILE_OPT=()
SKIP_SECRETS=0
SKIP_PANEL=0
SKIP_MANAGED=0
SKIP_MIGRATE=0
WORK_DIR=""

usage() {
  cat <<EOF
Usage: restore-panel-backup.sh [options]

Download / input:
  --s3-uri URI          s3://bucket/key/to/dock-pilot-….tar.gz
  --file PATH           Local .tar.gz (skip download)
  --s3-endpoint URL     Custom S3 endpoint (e.g. https://storage.yandexcloud.net)
  --aws-profile NAME    AWS CLI profile

Install target:
  --install-dir DIR     DockPilot install path (default: ${INSTALL_DIR})
  --compose-file NAME   Compose file name under install dir (default: ${COMPOSE_FILE})

Steps (all on by default):
  --skip-secrets        Do not merge secrets.env into .env
  --skip-panel          Do not restore panel/dockpilot.sql.gz
  --skip-migrate        Do not run panel migrations after panel restore
  --skip-managed        Do not restore managed/*.sql.gz

Env overrides:
  AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY / AWS_DEFAULT_REGION
  PANEL_POSTGRES_CONTAINER / MANAGED_POSTGRES_CONTAINER
EOF
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --s3-uri) S3_URI="$2"; shift 2 ;;
      --file) LOCAL_FILE="$2"; shift 2 ;;
      --s3-endpoint) S3_ENDPOINT="$2"; shift 2 ;;
      --aws-profile) AWS_PROFILE_OPT=(--profile "$2"); shift 2 ;;
      --install-dir) INSTALL_DIR="$2"; shift 2 ;;
      --compose-file) COMPOSE_FILE="$2"; shift 2 ;;
      --skip-secrets) SKIP_SECRETS=1; shift ;;
      --skip-panel) SKIP_PANEL=1; shift ;;
      --skip-migrate) SKIP_MIGRATE=1; shift ;;
      --skip-managed) SKIP_MANAGED=1; shift ;;
      -h|--help) usage; exit 0 ;;
      *) echo "Unknown option: $1" >&2; usage; exit 1 ;;
    esac
  done
}

log() { echo "[dock-pilot-restore] $*"; }
die() { echo "[dock-pilot-restore] ERROR: $*" >&2; exit 1; }

download_bundle() {
  local dest="$1"
  if [[ -n "$LOCAL_FILE" ]]; then
    [[ -f "$LOCAL_FILE" ]] || die "file not found: $LOCAL_FILE"
    cp "$LOCAL_FILE" "$dest"
    return
  fi
  [[ -n "$S3_URI" ]] || die "provide --s3-uri or --file"

  if command -v aws >/dev/null 2>&1; then
    local args=(s3 cp "$S3_URI" "$dest")
    if [[ -n "$S3_ENDPOINT" ]]; then
      args+=(--endpoint-url "$S3_ENDPOINT")
    fi
    log "Downloading via aws: $S3_URI"
    aws "${AWS_PROFILE_OPT[@]}" "${args[@]}"
    return
  fi

  if command -v yc >/dev/null 2>&1 && [[ "$S3_URI" =~ ^s3://([^/]+)/(.+)$ ]]; then
    local bucket="${BASH_REMATCH[1]}"
    local key="${BASH_REMATCH[2]}"
    log "Downloading via yc: $bucket / $key"
    yc storage s3api get-object --bucket "$bucket" --key "$key" "$dest" >/dev/null
    return
  fi

  die "need aws CLI or yc (or pass --file)"
}

merge_secrets_env() {
  local secrets_file="$1"
  local env_file="$2"
  [[ -f "$secrets_file" ]] || die "secrets.env missing in bundle"
  [[ -f "$env_file" ]] || die ".env not found at $env_file (run install.sh first)"

  log "Merging secrets.env into $env_file"
  local tmp
  tmp="$(mktemp)"
  cp "$env_file" "$tmp"
  chmod 600 "$tmp"

  while IFS= read -r line || [[ -n "$line" ]]; do
    [[ -z "$line" || "$line" =~ ^[[:space:]]*# ]] && continue
    local key="${line%%=*}"
    local val="${line#*=}"
    [[ -n "$key" && "$key" != "$line" ]] || continue
    if grep -qE "^${key}=" "$tmp"; then
      # Replace existing assignment (value may contain =)
      awk -v k="$key" -v v="$val" '
        BEGIN { done=0 }
        index($0, k "=") == 1 && !done { print k "=" v; done=1; next }
        { print }
        END { if (!done) print k "=" v }
      ' "$tmp" > "${tmp}.new"
      mv "${tmp}.new" "$tmp"
    else
      printf '%s=%s\n' "$key" "$val" >> "$tmp"
    fi
  done < "$secrets_file"

  mv "$tmp" "$env_file"
  chmod 600 "$env_file"
}

wait_pg() {
  local container="$1"
  local user="$2"
  log "Waiting for $container…"
  for _ in $(seq 1 60); do
    if docker exec "$container" pg_isready -U "$user" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  die "postgres not ready: $container"
}

restore_panel_sql() {
  local sql_gz="$1"
  local user="$2"
  local password="$3"
  local dbname="$4"

  [[ -f "$sql_gz" ]] || die "panel dump missing: $sql_gz"
  wait_pg "$PANEL_PG_CONTAINER" "$user"

  log "Restoring panel database into $PANEL_PG_CONTAINER ($dbname)"
  gunzip -c "$sql_gz" | docker exec -i \
    -e "PGPASSWORD=$password" \
    "$PANEL_PG_CONTAINER" \
    psql -v ON_ERROR_STOP=1 -U "$user" -d "$dbname" >/dev/null
}

run_migrations() {
  log "Running panel migrations"
  (
    cd "$INSTALL_DIR"
    docker compose -f "$COMPOSE_FILE" run --rm migrate
  )
}

restore_managed_sql() {
  local managed_dir="$1"
  local admin_user="${2:-postgres}"
  local admin_pass="${3:-}"

  [[ -d "$managed_dir" ]] || {
    log "No managed/ dumps in bundle — skipping"
    return 0
  }

  if ! docker inspect "$MANAGED_PG_CONTAINER" >/dev/null 2>&1; then
    log "Managed container $MANAGED_PG_CONTAINER not found."
    log "Start managed Postgres from the panel (Databases → Deploy), then re-run with --skip-panel --skip-secrets --file <bundle>."
    return 0
  fi

  wait_pg "$MANAGED_PG_CONTAINER" "$admin_user"

  local f dbname
  for f in "$managed_dir"/*.sql.gz; do
    [[ -e "$f" ]] || continue
    dbname="$(basename "$f" .sql.gz)"
    log "Restoring managed database: $dbname"
    docker exec -i \
      -e "PGPASSWORD=${admin_pass}" \
      "$MANAGED_PG_CONTAINER" \
      psql -v ON_ERROR_STOP=1 -U "$admin_user" -d postgres \
      -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '${dbname}' AND pid <> pg_backend_pid();" \
      >/dev/null 2>&1 || true
    docker exec -i \
      -e "PGPASSWORD=${admin_pass}" \
      "$MANAGED_PG_CONTAINER" \
      psql -v ON_ERROR_STOP=1 -U "$admin_user" -d postgres \
      -c "DROP DATABASE IF EXISTS \"${dbname}\";" >/dev/null
    docker exec -i \
      -e "PGPASSWORD=${admin_pass}" \
      "$MANAGED_PG_CONTAINER" \
      psql -v ON_ERROR_STOP=1 -U "$admin_user" -d postgres \
      -c "CREATE DATABASE \"${dbname}\";" >/dev/null
    gunzip -c "$f" | docker exec -i \
      -e "PGPASSWORD=${admin_pass}" \
      "$MANAGED_PG_CONTAINER" \
      psql -v ON_ERROR_STOP=1 -U "$admin_user" -d "$dbname" >/dev/null
  done
}

parse_args "$@"

[[ -d "$INSTALL_DIR" ]] || die "install dir not found: $INSTALL_DIR (run install.sh first)"
[[ -f "$INSTALL_DIR/.env" ]] || die "missing $INSTALL_DIR/.env"

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/dock-pilot-restore.XXXXXX")"
cleanup() { rm -rf "$WORK_DIR"; }
trap cleanup EXIT

BUNDLE="$WORK_DIR/bundle.tar.gz"
EXTRACT="$WORK_DIR/extract"
mkdir -p "$EXTRACT"

download_bundle "$BUNDLE"
log "Extracting bundle"
tar -xzf "$BUNDLE" -C "$EXTRACT"

# Support both flat layout and a single top-level directory
ROOT="$EXTRACT"
if [[ ! -f "$ROOT/manifest.json" && ! -f "$ROOT/panel/dockpilot.sql.gz" ]]; then
  for d in "$EXTRACT"/*; do
    if [[ -d "$d" && ( -f "$d/manifest.json" || -f "$d/panel/dockpilot.sql.gz" ) ]]; then
      ROOT="$d"
      break
    fi
  done
fi

if [[ -f "$ROOT/manifest.json" ]]; then
  log "Manifest:"
  sed 's/^/  /' "$ROOT/manifest.json" || true
fi

set -a
# shellcheck disable=SC1091
source "$INSTALL_DIR/.env"
set +a

if [[ "$SKIP_SECRETS" -eq 0 ]]; then
  merge_secrets_env "$ROOT/secrets.env" "$INSTALL_DIR/.env"
  set -a
  # shellcheck disable=SC1091
  source "$INSTALL_DIR/.env"
  set +a
  log "Restarting panel stack so API picks up secrets"
  (
    cd "$INSTALL_DIR"
    docker compose -f "$COMPOSE_FILE" up -d
  )
fi

PANEL_USER="${POSTGRES_USER:-dockpilot}"
PANEL_PASS="${POSTGRES_PASSWORD:-}"
PANEL_DB="${POSTGRES_DB:-dockpilot}"
[[ -n "$PANEL_PASS" ]] || die "POSTGRES_PASSWORD empty after secrets merge"

if [[ "$SKIP_PANEL" -eq 0 ]]; then
  restore_panel_sql "$ROOT/panel/dockpilot.sql.gz" "$PANEL_USER" "$PANEL_PASS" "$PANEL_DB"
fi

if [[ "$SKIP_MIGRATE" -eq 0 ]]; then
  run_migrations || log "WARNING: migrate returned non-zero (panel dump may already include schema)"
fi

if [[ "$SKIP_MANAGED" -eq 0 ]]; then
  # Admin credentials for managed PG live in the restored panel DB; try common defaults.
  # Prefer env if the operator set them for offline restore.
  MANAGED_USER="${MANAGED_POSTGRES_USER:-postgres}"
  MANAGED_PASS="${MANAGED_POSTGRES_PASSWORD:-}"
  restore_managed_sql "$ROOT/managed" "$MANAGED_USER" "$MANAGED_PASS"
fi

log "Done."
log "Next: open the panel, verify Databases, then redeploy each site from git (site volumes are not in the backup)."
