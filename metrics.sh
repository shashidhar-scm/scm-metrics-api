#!/usr/bin/env bash
set -e

API_URL="https://scm-metrics-api.citypost.us/api/metrics"
HOSTNAME="$(hostname)"
CONF="/etc/telegraf/telegraf.conf"
CONF_DIR="/etc/telegraf/telegraf.d"

OS_VERSION="$(lsb_release -rs 2>/dev/null || echo unknown)"

echo "▶ Detected Ubuntu version: $OS_VERSION"

install_telegraf() {
  if command -v telegraf >/dev/null; then
    echo "✔ Telegraf already installed"
    return
  fi

  case "$OS_VERSION" in
    16.04|18.04)
      echo "⚠ Legacy Ubuntu ($OS_VERSION) – installing Telegraf via .deb (NO APT FIX)"

      wget -q https://dl.influxdata.com/telegraf/releases/telegraf_1.37.0-1_amd64.deb
      sudo dpkg -i telegraf_1.37.0-1_amd64.deb || true

      echo "✔ Telegraf installed (dependencies intentionally not fixed)"
      ;;
    20.04|22.04)
      echo "▶ Supported Ubuntu ($OS_VERSION) – installing via apt repo"

      curl -fsSL https://repos.influxdata.com/influxdata-archive.key \
        | gpg --dearmor | sudo tee /usr/share/keyrings/influxdata-archive.gpg >/dev/null

      echo "deb [signed-by=/usr/share/keyrings/influxdata-archive.gpg] https://repos.influxdata.com/ubuntu jammy stable" \
        | sudo tee /etc/apt/sources.list.d/influxdata.list

      sudo apt update
      sudo apt install -y telegraf
      ;;
    *)
      echo "❌ Unsupported OS version: $OS_VERSION"
      exit 1
      ;;
  esac
}

install_telegraf

echo "▶ Enforcing 60s interval..."

sudo sed -i \
  -e 's/^\(\s*interval\s*=\s*\)"[^"]*"/\1"60s"/' \
  -e 's/^\(\s*flush_interval\s*=\s*\)"[^"]*"/\1"60s"/' \
  "$CONF"

grep -q 'flush_interval' "$CONF" || \
sudo sed -i '/interval\s*=\s*"60s"/a\  flush_interval = "60s"' "$CONF"

grep -q 'collection_jitter' "$CONF" || \
sudo sed -i '/round_interval\s*=\s*true/a\
  collection_jitter = "5s"\n  flush_jitter = "5s"' "$CONF"

echo "▶ Configuring HTTP output..."

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

echo "▶ Setting global tags..."

sudo tee "$CONF_DIR/tags.conf" >/dev/null <<EOF
[global_tags]
  server_id = "$HOSTNAME"
EOF

echo "▶ Restarting Telegraf..."
sudo systemctl restart telegraf
sudo systemctl enable telegraf

echo "▶ Agent config:"
grep -n "\[agent\]" -A15 "$CONF"

echo "✅ Telegraf running (60s interval)"
