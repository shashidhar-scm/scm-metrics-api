#!/usr/bin/env bash
set -euo pipefail

CHANNEL="${1:-Master}"

if ! command -v amixer >/dev/null 2>&1; then
  printf 'kiosk_volume,channel=%s level_percent=0i,muted=1i\n' "$CHANNEL"
  exit 0
fi

set +o pipefail
LINE="$(amixer get "$CHANNEL" 2>/dev/null | awk -F'[][]' '/Mono: Playback|Front Left: Playback|Front Right: Playback/ {printf "%s\t%s\n", $2, $6; exit}')"
exit_code=$?
set -o pipefail

if [[ $exit_code -ne 0 || -z "${LINE:-}" ]]; then
  RAW="0%"
  STATE="off"
else
  IFS=$'\t' read -r RAW STATE <<<"$LINE"
fi
VALUE="${RAW%%%}"

if [[ -z "$VALUE" ]]; then
  VALUE="0"
fi

STATE_LOWER="$(echo "${STATE:-on}" | tr '[:upper:]' '[:lower:]')"
if [[ "$STATE_LOWER" == "off" || "$STATE_LOWER" == "mute" || "$STATE_LOWER" == "muted" ]]; then
  MUTED=1
else
  MUTED=0
fi

printf 'kiosk_volume,channel=%s level_percent=%di,muted=%di\n' "$CHANNEL" "$VALUE" "$MUTED"
