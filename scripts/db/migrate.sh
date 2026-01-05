#!/usr/bin/env bash
set -euo pipefail

module="${1:-}"
direction="${2:-}"

if [[ -z "$module" || -z "$direction" ]]; then
  echo "usage: migrate.sh <module> <up|down>" >&2
  exit 2
fi

if [[ ! -d "modules/$module" ]]; then
  echo "[db-migrate] unknown module: $module (expected modules/$module)" >&2
  exit 2
fi

case "$direction" in
  up|down) ;;
  *)
    echo "[db-migrate] invalid direction: $direction (expected up|down)" >&2
    exit 2
    ;;
esac

echo "[db-migrate] no-op (placeholder): module=$module direction=$direction"

