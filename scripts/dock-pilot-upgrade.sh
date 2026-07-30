#!/usr/bin/env bash
# LEGACY WRAPPER: Redirects to barn-upgrade.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Preserve legacy DOCK_PILOT_* env vars for old installs
export BARN_INSTALL_DIR="${BARN_INSTALL_DIR:-${DOCK_PILOT_INSTALL_DIR:-/opt/dock-pilot}}"
export BARN_GITHUB_REPO="${BARN_GITHUB_REPO:-${DOCK_PILOT_GITHUB_REPO:-ebasht/barn}}"
exec "$SCRIPT_DIR/barn-upgrade.sh" "$@"
