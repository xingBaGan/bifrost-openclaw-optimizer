#!/usr/bin/env bash
set -euo pipefail

ROOT="/Users/jzj/bifrost"
SRC="$ROOT/config.json"
DST="$ROOT/data/config.json"

while true; do
  if [[ -f "$SRC" ]]; then
    if [[ ! -f "$DST" || "$SRC" -nt "$DST" ]]; then
      cp "$SRC" "$DST"
      (cd "$ROOT" && docker compose restart bifrost)
    fi
  fi
  sleep 1
 done
