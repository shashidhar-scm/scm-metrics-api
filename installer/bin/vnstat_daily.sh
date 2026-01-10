#!/usr/bin/env bash
set -euo pipefail
trap '' PIPE

DEFAULT_IFACE="$(ip route | awk '/default/ {print $5; exit}')"
IFACE="${1:-${DEFAULT_IFACE:-enp1s0}}"

JSON="$(vnstat --json -i "$IFACE" 2>/dev/null || true)"

emit_daily() {
  local json="$1"
  local fields
  fields="$(jq -r '
    .interfaces[0].traffic.days
    | sort_by(.date.year, .date.month, .date.day)
    | last?
    | "\(.date.year) \(.date.month) \(.date.day) \(.rx // 0) \(.tx // 0)"
  ' <<<"$json" 2>/dev/null || true)"

  if [[ -z "$fields" ]]; then
    printf 'vnstat_daily,interface=%s,day=%s rx_mib=0,tx_mib=0\n' "$IFACE" "$(date +%F)"
    return
  fi

  local year month day rx_kib tx_kib
  read -r year month day rx_kib tx_kib <<<"$fields"
  local day_fmt
  day_fmt="$(printf "%04d-%02d-%02d" "$year" "$month" "$day")"
  local rx_mib tx_mib
  rx_mib="$(awk -v v="$rx_kib" 'BEGIN {printf "%.3f", v / 1024}')"
  tx_mib="$(awk -v v="$tx_kib" 'BEGIN {printf "%.3f", v / 1024}')"
  printf 'vnstat_daily,interface=%s,day=%s rx_mib=%s,tx_mib=%s\n' "$IFACE" "$day_fmt" "$rx_mib" "$tx_mib"
}

emit_monthly() {
  local json="$1"
  local fields
  fields="$(jq -r '
    .interfaces[0].traffic.months
    | sort_by(.date.year, .date.month)
    | last?
    | "\(.date.year) \(.date.month) \(.rx // 0) \(.tx // 0)"
  ' <<<"$json" 2>/dev/null || true)"

  if [[ -z "$fields" ]]; then
    printf 'vnstat_monthly,interface=%s,month=%s rx_mib=0,tx_mib=0\n' "$IFACE" "$(date +%Y-%m)"
    return
  fi

  local year month rx_kib tx_kib
  read -r year month rx_kib tx_kib <<<"$fields"
  local month_fmt
  month_fmt="$(printf "%04d-%02d" "$year" "$month")"
  local rx_mib tx_mib
  rx_mib="$(awk -v v="$rx_kib" 'BEGIN {printf "%.3f", v / 1024}')"
  tx_mib="$(awk -v v="$tx_kib" 'BEGIN {printf "%.3f", v / 1024}')"
  printf 'vnstat_monthly,interface=%s,month=%s rx_mib=%s,tx_mib=%s\n' "$IFACE" "$month_fmt" "$rx_mib" "$tx_mib"
}

if [[ -n "$JSON" ]]; then
  emit_daily "$JSON"
  emit_monthly "$JSON"
else
  printf 'vnstat_daily,interface=%s,day=%s rx_mib=0,tx_mib=0\n' "$IFACE" "$(date +%F)"
  printf 'vnstat_monthly,interface=%s,month=%s rx_mib=0,tx_mib=0\n' "$IFACE" "$(date +%Y-%m)"
fi
