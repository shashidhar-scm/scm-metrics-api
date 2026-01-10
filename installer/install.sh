#!/usr/bin/env bash
set -e

API_URL="${API_URL:-https://scm-metrics-api.citypost.us/api/metrics}"
CONF="${CONF:-/etc/telegraf/telegraf.conf}"
CONF_DIR="${CONF_DIR:-/etc/telegraf/telegraf.d}"
KIOSK_JSON="${KIOSK_JSON:-/opt/scmkiosk/db_data/data_files/kiosk.json}"
HOSTNAME="$(hostname)"
NET_IFACE="$(ip route | awk '/default/ {print $5; exit}')"
NET_IFACE="${NET_IFACE:-enp1s0}"
export API_URL CONF CONF_DIR KIOSK_JSON HOSTNAME NET_IFACE
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ASSET_ROOT="$SCRIPT_DIR"
BIN_DIR="$ASSET_ROOT/bin"
CONFIG_DIR="$ASSET_ROOT/configs"

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
# Ensure audio tooling for volume readings
# ------------------------------------------------------------
ensure_audio_utils() {
  if dpkg -s alsa-utils >/dev/null 2>&1; then
    echo "✔ alsa-utils already installed"
  else
    echo "▶ Installing alsa-utils (for amixer)..."
    sudo apt install -y alsa-utils
  fi
}

ensure_audio_utils

# ------------------------------------------------------------
# Ensure display + input + network tooling
# ------------------------------------------------------------
ensure_display_utils() {
  local packages=()
  if ! dpkg -s x11-xserver-utils >/dev/null 2>&1; then
    packages+=("x11-xserver-utils")
  fi
  if ! dpkg -s usbutils >/dev/null 2>&1; then
    packages+=("usbutils")
  fi
  if ! dpkg -s ethtool >/dev/null 2>&1; then
    packages+=("ethtool")
  fi
  if ! dpkg -s iw >/dev/null 2>&1; then
    packages+=("iw")
  fi
  if ((${#packages[@]})); then
    echo "▶ Installing ${packages[*]} for display/input helpers..."
    sudo apt install -y "${packages[@]}"
  else
    echo "✔ Display/input tooling already installed"
  fi
}

ensure_display_utils

# ------------------------------------------------------------
# Ensure envsubst (gettext-base) is available
# ------------------------------------------------------------
ensure_envsubst() {
  if command -v envsubst >/dev/null 2>&1; then
    return
  fi
  echo "▶ Installing gettext-base for envsubst..."
  sudo apt install -y gettext-base
}

ensure_envsubst

# ------------------------------------------------------------
# Helper deployment utility
# ------------------------------------------------------------
install_helper_script() {
  local name="$1"
  local src="$BIN_DIR/$name"
  local dest="/usr/local/bin/$name"
  if [[ ! -f "$src" ]]; then
    echo "❌ Helper $name not found at $src"
    return 1
  fi
  echo "▶️ Installing $name helper script..."
  sudo install -m 0755 "$src" "$dest"
}

install_config_file() {
  local name="$1"
  local src="$CONFIG_DIR/$name"
  local dest="$CONF_DIR/$name"
  if [[ ! -f "$src" ]]; then
    echo "❌ Config $name not found at $src"
    return 1
  fi
  echo "▶️ Installing config $name..."
  sudo install -m 0644 "$src" "$dest"
}

install_template_config() {
  local template_name="$1"
  local dest="$2"
  local src="$CONFIG_DIR/$template_name"
  if [[ ! -f "$src" ]]; then
    echo "❌ Template $template_name not found at $src"
    return 1
  fi
  echo "▶️ Rendering template $template_name..."
  envsubst < "$src" | sudo tee "$dest" >/dev/null
}

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
install_helper_script "vnstat_daily.sh"

# ------------------------------------------------------------
# Helper script for audio volume
# ------------------------------------------------------------
install_helper_script "volume_level.sh"

# ------------------------------------------------------------
# Helper script for fan speed + chassis thermals
# ------------------------------------------------------------
install_helper_script "chassis_health.sh"

# ------------------------------------------------------------
# Helper script for power / battery status
# ------------------------------------------------------------
install_helper_script "power_status.sh"

# ------------------------------------------------------------
# Helper script for display status
# ------------------------------------------------------------
install_helper_script "display_status.sh"

# ------------------------------------------------------------
# Helper script for input device health
# ------------------------------------------------------------
install_helper_script "input_devices.sh"

# ------------------------------------------------------------
# Helper script for network link quality
# ------------------------------------------------------------
install_helper_script "network_link.sh"

# ------------------------------------------------------------
# Helper script for process watchdog
# ------------------------------------------------------------
install_helper_script "process_watchdog.sh"

# ------------------------------------------------------------
# Ensure telegraf user can access ALSA devices
# ------------------------------------------------------------
if id telegraf >/dev/null 2>&1; then
  echo "▶️ Adding telegraf user to audio group (safe if already a member)"
  sudo usermod -a -G audio telegraf || true
else
  echo "⚠ telegraf user not found, skipping audio group assignment"
fi

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

install_template_config "http-output.conf.tmpl" "$CONF_DIR/http-output.conf"

# ------------------------------------------------------------
# Sensors + Net inputs
# ------------------------------------------------------------
echo "▶️ Configuring sensors + net inputs..."

install_config_file "inputs-sensors.conf"

install_template_config "inputs-net.conf.tmpl" "$CONF_DIR/inputs-net.conf"

install_config_file "inputs-vnstat.conf"
install_config_file "inputs-volume.conf"
install_config_file "inputs-chassis.conf"
install_config_file "inputs-power.conf"
install_config_file "inputs-display.conf"
install_config_file "inputs-input.conf"
install_config_file "inputs-link.conf"
install_config_file "inputs-watchdog.conf"

# ------------------------------------------------------------
# Global Tags (server_id)
# ------------------------------------------------------------
install_template_config "global-tags.conf.tmpl" "$CONF_DIR/global-tags.conf"

# ------------------------------------------------------------
# City + Region Tags from kiosk.json (SAFE VERSION)
# ------------------------------------------------------------
if [[ -f "$KIOSK_JSON" ]]; then
  echo "▶️ Found kiosk.json – configuring city & region tags"
  install_config_file "kiosk-tags.conf"
else
  echo "⚠ kiosk.json not found, skipping city/region tags"
  sudo rm -f "$CONF_DIR/kiosk-tags.conf"
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
