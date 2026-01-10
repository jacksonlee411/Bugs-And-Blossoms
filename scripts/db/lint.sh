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

migrations_dir="migrations/$module"
if [[ ! -d "$migrations_dir" ]]; then
  echo "[db-lint] missing migrations dir: $migrations_dir" >&2
  exit 2
fi

dev_url="${ATLAS_DEV_URL:-docker://postgres/17/dev}"

echo "[db-lint] atlas migrate validate: $module"
ATLAS_NO_UPDATE_NOTIFIER=true \
  ./scripts/db/run_atlas.sh migrate validate \
  --dir "file://${migrations_dir}" \
  --dir-format goose \
  --dev-url "$dev_url"
