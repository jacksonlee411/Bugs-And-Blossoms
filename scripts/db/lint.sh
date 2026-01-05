#!/usr/bin/env bash
set -euo pipefail

module="${1:-}"
if [[ -z "$module" ]]; then
  echo "usage: lint.sh <module>" >&2
  exit 2
fi

if [[ ! -d "modules/$module" ]]; then
  echo "[db-lint] unknown module: $module (expected modules/$module)" >&2
  exit 2
fi

echo "[db-lint] no-op (placeholder): module=$module"

