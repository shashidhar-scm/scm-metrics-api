#!/usr/bin/env bash
set -e

API_URL="https://scm-metrics-api.citypost.us/api/metrics"
CONF="/etc/telegraf/telegraf.conf"
CONF_DIR="/etc/telegraf/telegraf.d"
KIOSK_JSON="/opt/scmkiosk/db_data/data_files/kiosk.json"
HOSTNAME="$(hostname)"
NET_IFACE="$(ip route | awk '/default/ {print $5; exit}')"
NET_IFACE="${NET_IFACE:-enp1s0}"

OS_VERSION="$(lsb_release -rs 2>/dev/null || echo unknown)"
echo "▶️ Detected Ubuntu version: $OS_VERSION"
echo "▶️ Primary network interface: $NET_IFACE"

# ------------------------------------------------------------
# Install Telegraf (safe for 16 / 18 / 20 / 22)
# ------------------------------------------------------------
install_telegraf() {
  if command -v telegraf >/dev/null; then
    echo "✔ Telegraf already installed"
    return
  fi

  case "$OS_VERSION" in
    14.04|16.04|18.04)
      echo "⚠ Legacy Ubuntu – installing Telegraf via .deb"
      wget -q https://dl.influxdata.com/telegraf/releases/telegraf_1.37.0-1_amd64.deb
      sudo dpkg -i telegraf_1.37.0-1_amd64.deb || true
      ;;
    20.04|22.04)
      echo "▶ Installing Telegraf via Influx repo"
      curl -fsSL https://repos.influxdata.com/influxdata-archive.key \
        | gpg --dearmor | sudo tee /usr/share/keyrings/influxdata-archive.gpg >/dev/null

      echo "deb [signed-by=/usr/share/keyrings/influxdata-archive.gpg] https://repos.influxdata.com/ubuntu jammy stable" \
        | sudo tee /etc/apt/sources.list.d/influxdata.list

      sudo apt update
      sudo apt install -y telegraf
      ;;
    *)
      echo "❌ Unsupported OS"
      exit 1
      ;;
  esac
}

install_telegraf

# ------------------------------------------------------------
# Install lm-sensors for temperature readings
# ------------------------------------------------------------
install_sensors_prereqs() {
  if dpkg -s lm-sensors >/dev/null 2>&1; then
    echo "✔ lm-sensors already installed"
  else
    echo "▶ Installing lm-sensors..."
    sudo apt install -y lm-sensors
  fi

  echo "▶ Running sensors-detect (auto)..."
  sudo sensors-detect --auto || true
}

install_sensors_prereqs

# ------------------------------------------------------------
# Install vnStat + jq for daily network totals
# ------------------------------------------------------------
install_vnstat_stack() {
  local packages=()
  if ! command -v vnstat >/dev/null 2>&1; then
    packages+=("vnstat")
  fi
  if ! command -v jq >/dev/null 2>&1; then
    packages+=("jq")
  fi

  if ((${#packages[@]})); then
    echo "▶ Installing ${packages[*]}..."
    sudo apt install -y "${packages[@]}"
  else
    echo "✔ vnStat + jq already installed"
  fi

  echo "▶ Ensuring vnStat tracks $NET_IFACE"
  sudo vnstat --add -i "$NET_IFACE" >/dev/null 2>&1 || true
  sudo vnstat -u -i "$NET_IFACE" >/dev/null 2>&1 || true

  if command -v systemctl >/dev/null; then
    sudo systemctl enable vnstat >/dev/null 2>&1 || true
    sudo systemctl restart vnstat >/dev/null 2>&1 || sudo systemctl start vnstat >/dev/null 2>&1 || true
  else
    sudo service vnstat restart >/dev/null 2>&1 || sudo service vnstat start >/dev/null 2>&1 || true
  fi
}

install_vnstat_stack

# ------------------------------------------------------------
# Helper script for vnStat daily totals
# ------------------------------------------------------------
echo "▶️ Installing vnStat helper script..."
sudo tee /usr/local/bin/vnstat_daily.sh >/dev/null <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

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
EOF
sudo chmod +x /usr/local/bin/vnstat_daily.sh

# ------------------------------------------------------------
# Enforce 60s collection + flush
# ------------------------------------------------------------
echo "▶️ Enforcing 60s agent interval..."

sudo sed -i \
  -e 's/^\s*interval\s*=.*/  interval = "60s"/' \
  -e 's/^\s*flush_interval\s*=.*/  flush_interval = "60s"/' \
  "$CONF"

grep -q 'flush_interval' "$CONF" || \
sudo sed -i '/interval = "60s"/a\  flush_interval = "60s"' "$CONF"

# ------------------------------------------------------------
# HTTP Output
# ------------------------------------------------------------
echo "▶️ Configuring HTTP output..."

sudo mkdir -p "$CONF_DIR"

sudo tee "$CONF_DIR/http-output.conf" >/dev/null <<EOF
[[outputs.http]]
  url = "$API_URL"
  method = "POST"
  timeout = "10s"
  data_format = "json"

  [outputs.http.headers]
    Content-Type = "application/json"
EOF

# ------------------------------------------------------------
# Sensors + Net inputs
# ------------------------------------------------------------
echo "▶️ Configuring sensors + net inputs..."

sudo tee "$CONF_DIR/inputs-sensors.conf" >/dev/null <<'EOF'
[[inputs.sensors]]
  name_override = "kiosk_temperature"
  [inputs.sensors.tags]
    sensor_source = "lm"
EOF

sudo tee "$CONF_DIR/inputs-net.conf" >/dev/null <<EOF
[[inputs.net]]
  interfaces = ["$NET_IFACE"]
  ## Collect all interfaces; loopback filtering happens server-side.
EOF

sudo tee "$CONF_DIR/inputs-vnstat.conf" >/dev/null <<'EOF'
[[inputs.exec]]
  commands = ["/usr/local/bin/vnstat_daily.sh"]
  timeout = "5s"
  data_format = "influx"
EOF

# ------------------------------------------------------------
# Global Tags (server_id)
# ------------------------------------------------------------
sudo tee "$CONF_DIR/global-tags.conf" >/dev/null <<EOF
[global_tags]
  server_id = "$HOSTNAME"
EOF

# ------------------------------------------------------------
# City + Region Tags from kiosk.json (SAFE VERSION)
# ------------------------------------------------------------
if [[ -f "$KIOSK_JSON" ]]; then
  echo "▶️ Found kiosk.json – configuring city & region tags"

  sudo tee "$CONF_DIR/kiosk-tags.conf" >/dev/null <<'EOF'
# -----------------------------
# City + City Name (TAGS ONLY)
# -----------------------------
[[inputs.file]]
  files = ["/opt/scmkiosk/db_data/data_files/kiosk.json"]
  data_format = "json_v2"
  name_override = "kiosk_city"

  [[inputs.file.json_v2]]
    measurement_name = "kiosk_city"

    [[inputs.file.json_v2.object]]
      path = "@this"
      tags = ["city", "city_full_name"]
      disable_prepend_keys = true

# -----------------------------
# Region (TAGS ONLY)
# -----------------------------
[[inputs.file]]
  files = ["/opt/scmkiosk/db_data/data_files/kiosk.json"]
  data_format = "json_v2"
  name_override = "kiosk_region"

  [[inputs.file.json_v2]]
    measurement_name = "kiosk_region"

    [[inputs.file.json_v2.object]]
      path = "region"
      tags = ["code", "name"]
      disable_prepend_keys = true
EOF
else
  echo "⚠ kiosk.json not found, skipping city/region tags"
fi

# ------------------------------------------------------------
# Validate config BEFORE restart
# ------------------------------------------------------------
echo "▶️ Validating Telegraf config..."
sudo telegraf \
  --config "$CONF" \
  --config-directory "$CONF_DIR" \
  --test >/dev/null

# ------------------------------------------------------------
# Restart Telegraf (handles both systemd and upstart)
# ------------------------------------------------------------
echo "▶️ Restarting Telegraf..."
if command -v systemctl >/dev/null; then
  sudo systemctl restart telegraf || sudo systemctl start telegraf
  sudo systemctl enable telegraf
else
  # For Ubuntu 14.04 (upstart)
  sudo service telegraf stop 2>/dev/null || true
  sudo service telegraf start
  sudo update-rc.d telegraf defaults
fi

echo "✅ Telegraf running (60s interval, clean tags)"
