#!/usr/bin/env bash
set -euo pipefail

module="${1:-}"
if [[ -z "$module" ]]; then
  echo "usage: plan.sh <module>" >&2
  exit 2
fi

if [[ ! -d "modules/$module" ]]; then
  echo "[db-plan] unknown module: $module (expected modules/$module)" >&2
  exit 2
fi

echo "[db-plan] no-op (placeholder): module=$module"

