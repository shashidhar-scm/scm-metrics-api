#!/usr/bin/env bash
set -euo pipefail

BASE="/sys/class/power_supply"
if [[ ! -d "$BASE" ]]; then
  exit 0
fi

sanitize_tag() {
  echo "$1" | tr '[:upper:]' '[:lower:]' | tr ' /' '__'
}

for supply in "$BASE"/*; do
  [[ -d "$supply" ]] || continue
  name="$(basename "$supply")"
  type="$(<"$supply/type" 2>/dev/null || true)"
  type_lc="$(sanitize_tag "$type")"
  case "$type" in
    "" )
      continue
      ;;
  esac

  status="$(<"$supply/status" 2>/dev/null || echo "unknown")"
  status_tag="$(sanitize_tag "$status")"

  if [[ "$type" == "Battery" ]]; then
    capacity="$(<"$supply/capacity" 2>/dev/null || echo "0")"
    present="$(<"$supply/present" 2>/dev/null || echo "0")"
    voltage_raw="$(<"$supply/voltage_now" 2>/dev/null || echo "0")"
    current_raw="$(<"$supply/current_now" 2>/dev/null || echo "0")"
    voltage_mv=$((voltage_raw / 1000))
    current_ma=$((current_raw / 1000))
    printf 'kiosk_power,supply=%s,type=%s,status=%s charge_percent=%di,voltage_mv=%di,current_ma=%di,present=%di\n' \
      "$name" "$type_lc" "$status_tag" "${capacity:-0}" "${voltage_mv:-0}" "${current_ma:-0}" "${present:-0}"
  else
    online="$(<"$supply/online" 2>/dev/null || echo "0")"
    printf 'kiosk_power,supply=%s,type=%s,status=%s online=%di\n' \
      "$name" "$type_lc" "$status_tag" "${online:-0}"
  fi
done
