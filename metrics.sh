#!/usr/bin/env bash
set -e

API_URL="https://scm-metrics-api.citypost.us/api/metrics"
CONF="/etc/telegraf/telegraf.conf"
CONF_DIR="/etc/telegraf/telegraf.d"
KIOSK_JSON="/opt/scmkiosk/db_data/data_files/kiosk.json"
HOSTNAME="$(hostname)"

OS_VERSION="$(lsb_release -rs 2>/dev/null || echo unknown)"
echo "▶️ Detected Ubuntu version: $OS_VERSION"

# ------------------------------------------------------------
# Install Telegraf (safe for 16 / 18 / 20 / 22)
# ------------------------------------------------------------
install_telegraf() {
  if command -v telegraf >/dev/null; then
    echo "✔ Telegraf already installed"
    return
  fi

  case "$OS_VERSION" in
    16.04|18.04)
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
# Restart Telegraf
# ------------------------------------------------------------
echo "▶️ Restarting Telegraf..."
sudo systemctl reset-failed telegraf || true
sudo systemctl restart telegraf
sudo systemctl enable telegraf

echo "✅ Telegraf running (60s interval, clean tags)"
