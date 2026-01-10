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
# Ensure display tooling for xrandr
# ------------------------------------------------------------
ensure_display_utils() {
  if dpkg -s x11-xserver-utils >/dev/null 2>&1; then
    echo "✔ x11-xserver-utils already installed"
  else
    echo "▶ Installing x11-xserver-utils (for xrandr)..."
    sudo apt install -y x11-xserver-utils
  fi
}

ensure_display_utils

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
# Helper script for audio volume
# ------------------------------------------------------------
echo "▶️ Installing volume helper script..."
sudo tee /usr/local/bin/volume_level.sh >/dev/null <<'EOF'
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
EOF
sudo chmod +x /usr/local/bin/volume_level.sh

# ------------------------------------------------------------
# Helper script for fan speed + chassis thermals
# ------------------------------------------------------------
echo "▶️ Installing chassis health helper script..."
sudo tee /usr/local/bin/chassis_health.sh >/dev/null <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

emit_defaults() {
  printf 'kiosk_chassis temp_c=0\n'
  printf 'kiosk_hotspot temp_c=0\n'
}

if ! command -v python3 >/dev/null; then
  emit_defaults
  exit 0
fi

python3 <<'PY'
import os
import shutil
import subprocess
import sys

def emit(fan=0, chassis=0.0, hotspot=0.0):
    if fan > 0:
        print("kiosk_fan rpm=%di" % fan)
    print("kiosk_chassis temp_c=%.2f" % chassis)
    print("kiosk_hotspot temp_c=%.2f" % hotspot)

sensors_bin = shutil.which("sensors") or "/usr/bin/sensors"
if not os.path.exists(sensors_bin):
    emit()
    sys.exit(0)

try:
    raw = subprocess.check_output(
        [sensors_bin, "-u"], stderr=subprocess.STDOUT, universal_newlines=True
    )
except Exception:
    emit()
    sys.exit(0)

fan_values = []
temps = []
labels = {}
current_label = ""

def record_temp(base, value):
    label = labels.get(base, current_label or base).lower()
    temps.append((label, value))

for raw_line in raw.splitlines():
    line = raw_line.strip()
    if not line or line.startswith("Adapter:"):
        continue
    if line.endswith(":") and ":" not in line[:-1]:
        current_label = line[:-1]
        continue
    if ":" not in line:
        continue
    key, val = [part.strip() for part in line.split(":", 1)]
    if not key:
        continue
    if key.endswith("_label"):
        labels[key[:-6]] = val.strip('"')
        continue
    try:
        value = float(val.split()[0])
    except ValueError:
        continue
    if key.startswith("fan") and key.endswith("_input"):
        fan_values.append(value)
    elif key.startswith("temp") and key.endswith("_input"):
        record_temp(key[:-6], value)

priority = ["chassis", "system", "board", "ambient", "enclosure", "case", "systin"]

def pick_chassis():
    if not temps:
        return 0.0
    for word in priority:
        matches = [val for label, val in temps if word in label]
        if matches:
            return max(matches)
    return max(val for _, val in temps)

hotspot = max((val for _, val in temps), default=0.0)
chassis = pick_chassis()
fan = int(max(fan_values)) if fan_values else 0

emit(fan=fan, chassis=chassis, hotspot=hotspot)
PY
EOF
sudo chmod +x /usr/local/bin/chassis_health.sh

# ------------------------------------------------------------
# Helper script for power / battery status
# ------------------------------------------------------------
echo "▶️ Installing power status helper script..."
sudo tee /usr/local/bin/power_status.sh >/dev/null <<'EOF'
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
EOF
sudo chmod +x /usr/local/bin/power_status.sh

# ------------------------------------------------------------
# Helper script for display status
# ------------------------------------------------------------
echo "▶️ Installing display status helper script..."
sudo tee /usr/local/bin/display_status.sh >/dev/null <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

export DISPLAY="${DISPLAY:-:0}"

if ! command -v xrandr >/dev/null 2>&1; then
  exit 0
fi

python3 <<'PY'
import os
import re
import shutil
import subprocess
import sys

display = os.environ.get("DISPLAY") or ":0"
env = dict(os.environ)
env["DISPLAY"] = display
xrandr = shutil.which("xrandr")
if not xrandr:
    sys.exit(0)

try:
    raw = subprocess.check_output(
        [xrandr, "--query"],
        env=env,
        stderr=subprocess.STDOUT,
        universal_newlines=True,
    )
except Exception:
    sys.exit(0)

if not raw.strip():
    sys.exit(0)

lines = raw.splitlines()
dpms_enabled = None

def sanitize_tag(value: str) -> str:
    return re.sub(r"[^\w\-.]", "_", value)

def parse_resolution(token: str):
    base = token.split("+", 1)[0]
    if "x" not in base:
        return None
    parts = base.split("x", 1)
    if len(parts) != 2:
        return None
    try:
        width = int(parts[0])
        height_match = re.match(r"\d+", parts[1])
        if not height_match:
            return None
        height = int(height_match.group(0))
        return width, height
    except ValueError:
        return None

def parse_mode_line(line: str):
    tokens = line.strip().split()
    if not tokens:
        return None
    dims = parse_resolution(tokens[0])
    if not dims:
        return None
    refresh = 0
    for token in tokens[1:]:
        token_clean = token.replace("*", "").replace("+", "")
        try:
            refresh = float(token_clean)
            break
        except ValueError:
            continue
    return dims[0], dims[1], int(round(refresh)) if refresh else 0

results = []
i = 0
while i < len(lines):
    line = lines[i]
    if "DPMS is" in line:
        dpms_enabled = 1 if "Enabled" in line else 0
        i += 1
        continue
    if not line or line[0].isspace():
        i += 1
        continue
    parts = line.split()
    if len(parts) < 2 or parts[1] not in ("connected", "disconnected"):
        i += 1
        continue
    name = parts[0]
    status = parts[1]
    primary = 1 if "primary" in parts else 0
    connected = 1 if status == "connected" else 0
    width = 0
    height = 0
    refresh = 0

    j = i + 1
    mode_lines = []
    while j < len(lines) and lines[j].startswith(" "):
        mode_lines.append(lines[j])
        j += 1

    header_dims = None
    for token in parts[2:]:
        dims = parse_resolution(token)
        if dims:
            header_dims = dims
            break

    if connected:
        if header_dims:
            width, height = header_dims
        for mode_line in mode_lines:
            parsed = parse_mode_line(mode_line)
            if not parsed:
                continue
            mw, mh, mr = parsed
            if mw:
                width = mw
            if mh:
                height = mh
            if mr:
                refresh = mr
                break

    dpms_flag = 0 if dpms_enabled is None else dpms_enabled
    results.append(
        (
            sanitize_tag(name),
            connected,
            width,
            height,
            refresh,
            primary,
            dpms_flag,
        )
    )
    i = j

if not results:
    sys.exit(0)

for name, connected, width, height, refresh, primary, dpms_flag in results:
    fields = [
        "connected={}i".format(connected),
        "width={}i".format(width),
        "height={}i".format(height),
        "refresh_hz={}i".format(refresh),
        "primary={}i".format(primary),
        "dpms_enabled={}i".format(dpms_flag),
    ]
    print("kiosk_display,output={} ".format(name) + ",".join(fields))
PY
EOF
sudo chmod +x /usr/local/bin/display_status.sh

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

sudo tee "$CONF_DIR/inputs-volume.conf" >/dev/null <<'EOF'
[[inputs.exec]]
  commands = ["/usr/local/bin/volume_level.sh"]
  timeout = "5s"
  data_format = "influx"
EOF

sudo tee "$CONF_DIR/inputs-chassis.conf" >/dev/null <<'EOF'
[[inputs.exec]]
  commands = ["/usr/local/bin/chassis_health.sh"]
  timeout = "5s"
  data_format = "influx"
EOF

sudo tee "$CONF_DIR/inputs-power.conf" >/dev/null <<'EOF'
[[inputs.exec]]
  commands = ["/usr/local/bin/power_status.sh"]
  timeout = "5s"
  data_format = "influx"
EOF

sudo tee "$CONF_DIR/inputs-display.conf" >/dev/null <<'EOF'
[[inputs.exec]]
  commands = ["/usr/local/bin/display_status.sh"]
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
