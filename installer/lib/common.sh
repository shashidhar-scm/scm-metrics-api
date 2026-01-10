#!/usr/bin/env bash
set -euo pipefail

API_URL="${API_URL:-https://scm-metrics-api.citypost.us/api/metrics}"
CONF="${CONF:-/etc/telegraf/telegraf.conf}"
CONF_DIR="${CONF_DIR:-/etc/telegraf/telegraf.d}"
KIOSK_JSON="${KIOSK_JSON:-/opt/scmkiosk/db_data/data_files/kiosk.json}"
HOSTNAME="${HOSTNAME:-$(hostname)}"
NET_IFACE="${NET_IFACE:-$(ip route | awk '/default/ {print $5; exit}')}"
NET_IFACE="${NET_IFACE:-enp1s0}"
