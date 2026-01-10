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
