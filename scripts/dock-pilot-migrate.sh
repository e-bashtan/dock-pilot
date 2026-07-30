#!/usr/bin/env bash
# LEGACY WRAPPER: Redirects to barn-migrate.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec "$SCRIPT_DIR/barn-migrate.sh" "$@"
