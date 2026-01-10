#!/usr/bin/env bash
set -euo pipefail

CHANNEL="${1:-Master}"

LINE="$(amixer get "$CHANNEL" 2>/dev/null | awk -F'[][]' '/Mono: Playback|Front Left: Playback|Front Right: Playback/ {printf "%s\t%s\n", $2, $6; exit}')"
if [[ -n "$LINE" ]]; then
  IFS=$'\t' read -r RAW STATE <<<"$LINE"
else
  RAW="0%"
  STATE=""
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
