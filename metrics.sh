#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALLER="$SCRIPT_DIR/installer/install.sh"

if [[ ! -x "$INSTALLER" ]]; then
  echo "❌ Installer entrypoint missing or not executable: $INSTALLER"
  exit 1
fi

exec "$INSTALLER" "$@"
