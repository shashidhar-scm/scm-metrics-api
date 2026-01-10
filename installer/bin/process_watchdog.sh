#!/usr/bin/env bash
set -euo pipefail

declare -A PATTERNS=(
  ["kiosk"]="\/opt\/scmkiosk\/cmd\/cmd"
  ["chisel"]="\/opt\/scmkiosk\/rtty\/chisel"
  ["rtty"]="\/opt\/scmkiosk\/rtty\/rtty"
  ["ui"]="\/opt\/scmkiosk\/ui\/release\/KioskUI-linux-x64"
  ["syncer"]="\/opt\/scmkiosk\/syncer\/syncer"
)

ORDER=("kiosk" "chisel" "rtty" "ui" "syncer")

for name in "${ORDER[@]}"; do
  pattern="${PATTERNS[$name]}"
  count=$(pgrep -fc -- "$pattern" || true)
  running=0
  if [[ "${count:-0}" -gt 0 ]]; then
    running=1
  fi
  printf 'kiosk_service,name=%s running=%si,process_count=%si\n' "$name" "$running" "${count:-0}"
done
