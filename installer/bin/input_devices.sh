#!/usr/bin/env bash
set -euo pipefail

if ! command -v python3 >/dev/null 2>&1; then
  exit 0
fi

python3 <<'PY'
import os
import pathlib
import re
import shutil
import subprocess
import sys

records = []

def sanitize(value: str) -> str:
    if not value:
        return ""
    value = value.strip()
    return re.sub(r"[^A-Za-z0-9_.-]", "_", value)

def emit_line(tags, fields):
    tag_parts = []
    for key, val in tags.items():
        if not val:
            continue
        tag_parts.append("{}={}".format(key, val))
    if not fields:
        return
    field_parts = []
    for key, val in fields.items():
        if isinstance(val, int):
            field_parts.append("{}={}i".format(key, val))
        else:
            field_parts.append("{}={}".format(key, val))
    if not field_parts:
        return
    line = "kiosk_input"
    if tag_parts:
        line += "," + ",".join(tag_parts)
    line += " " + ",".join(field_parts)
    print(line)

def collect_usb():
    if shutil.which("lsusb") is None:
        return
    try:
        raw = subprocess.check_output(
            ["lsusb"],
            universal_newlines=True,
            stderr=subprocess.STDOUT,
        )
    except Exception:
        return
    pattern = re.compile(r"Bus (\d+)\s+Device (\d+):\s+ID ([0-9A-Fa-f]{4}):([0-9A-Fa-f]{4})\s*(.*)")
    for line in raw.splitlines():
        line = line.strip()
        if not line:
            continue
        match = pattern.match(line)
        if not match:
            continue
        bus, device, vendor, product, name = match.groups()
        tags = {
            "source": "usb",
            "bus": sanitize(bus),
            "device": sanitize(device),
            "vendor": sanitize(vendor.lower()),
            "product": sanitize(product.lower()),
            "name": sanitize(name) or "unknown",
        }
        records.append((tags, {"present": 1}))

def collect_input_links():
    base = pathlib.Path("/dev/input/by-id")
    if not base.exists():
        return
    for entry in sorted(base.iterdir()):
        if not entry.is_symlink():
            continue
        entry_path = str(entry)
        try:
            resolved = os.path.realpath(entry_path)
            exists = os.path.exists(resolved)
        except OSError:
            resolved = ""
            exists = False
        tags = {
            "source": "dev_input",
            "id": sanitize(entry.name),
            "target": sanitize(os.path.basename(resolved)) if resolved else "",
        }
        fields = {
            "link_present": 1,
            "event_present": 1 if exists else 0,
        }
        records.append((tags, fields))

collect_usb()
collect_input_links()

for tags, fields in records:
    emit_line(tags, fields)
PY
