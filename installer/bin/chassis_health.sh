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
