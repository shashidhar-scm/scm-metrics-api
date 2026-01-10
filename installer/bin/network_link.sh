#!/usr/bin/env bash
set -euo pipefail
trap '' PIPE

DEFAULT_IFACE="$(ip route | awk '/default/ {print $5; exit}')"
IFACE="${1:-${DEFAULT_IFACE:-enp1s0}}"
if [[ -z "$IFACE" ]]; then
  exit 0
fi

sanitize() {
  echo "$1" | tr -cs '[:alnum:]_.-' '_'
}

STAT_BASE="/sys/class/net/$IFACE/statistics"
stat_read() {
  local file="$STAT_BASE/$1"
  if [[ -r "$file" ]]; then
    cat "$file"
  else
    echo 0
  fi
}

rx_errors="$(stat_read rx_errors)"
tx_errors="$(stat_read tx_errors)"
rx_dropped="$(stat_read rx_dropped)"
tx_dropped="$(stat_read tx_dropped)"

link_type="wired"
if iw dev "$IFACE" info >/dev/null 2>&1; then
  link_type="wifi"
fi

emit_fields() {
  local tags="$1"
  local fields="$2"
  printf 'kiosk_link,%s %s\n' "$tags" "$fields"
}

if [[ "$link_type" == "wired" ]]; then
  if ! command -v ethtool >/dev/null 2>&1; then
    exit 0
  fi
  ethtool_out="$(ethtool "$IFACE" 2>/dev/null || true)"
  if [[ -z "$ethtool_out" ]]; then
    exit 0
  fi
  link_detected="$(grep -i 'Link detected' <<<"$ethtool_out" | awk '{print tolower($3)}')"
  link_up=0
  if [[ "$link_detected" == "yes" ]]; then
    link_up=1
  fi
  speed_val="$(grep -i 'Speed:' <<<"$ethtool_out" | awk '{print $2}' | tr -d '[:alpha:]/')"
  if [[ -z "$speed_val" ]]; then
    speed_val=0
  fi
  duplex="$(grep -i 'Duplex:' <<<"$ethtool_out" | awk '{print tolower($2)}')"
  duplex_full=0
  if [[ "$duplex" == "full" ]]; then
    duplex_full=1
  fi
  autoneg="$(grep -i 'Auto-negotiation:' <<<"$ethtool_out" | awk '{print tolower($2)}')"
  autoneg_flag=0
  if [[ "$autoneg" == "on" ]]; then
    autoneg_flag=1
  fi
  emit_fields "interface=$(sanitize "$IFACE"),type=wired" \
    "link_up=${link_up}i,speed_mbps=${speed_val}i,duplex_full=${duplex_full}i,autoneg=${autoneg_flag}i,rx_errors=${rx_errors}i,tx_errors=${tx_errors}i,rx_dropped=${rx_dropped}i,tx_dropped=${tx_dropped}i"
else
  if ! command -v iw >/dev/null 2>&1; then
    exit 0
  fi
  link_info="$(iw dev "$IFACE" link 2>/dev/null || true)"
  link_up=1
  signal_dbm=0
  tx_bitrate=0
  rx_bitrate=0
  if grep -qi 'Not connected' <<<"$link_info"; then
    link_up=0
  else
    sig="$(grep -i 'signal:' <<<"$link_info" | awk '{print $2}')"
    if [[ -n "$sig" ]]; then
      signal_dbm="${sig}"
    fi
    tx_line="$(grep -i 'tx bitrate:' <<<"$link_info")"
    if [[ -n "$tx_line" ]]; then
      tx_bitrate="$(awk '{print $3}' <<<"$tx_line" | awk -F'.' '{print $1}')"
    fi
    rx_line="$(grep -i 'rx bitrate:' <<<"$link_info")"
    if [[ -n "$rx_line" ]]; then
      rx_bitrate="$(awk '{print $3}' <<<"$rx_line" | awk -F'.' '{print $1}')"
    fi
  fi
  emit_fields "interface=$(sanitize "$IFACE"),type=wifi" \
    "link_up=${link_up}i,signal_dbm=${signal_dbm}i,tx_bitrate_mbps=${tx_bitrate}i,rx_bitrate_mbps=${rx_bitrate}i,rx_errors=${rx_errors}i,tx_errors=${tx_errors}i,rx_dropped=${rx_dropped}i,tx_dropped=${tx_dropped}i"
fi
