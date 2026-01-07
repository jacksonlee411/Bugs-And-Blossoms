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

schema_dir="modules/$module/infrastructure/persistence/schema"
migrations_dir="migrations/$module"

if [[ ! -d "$schema_dir" ]]; then
  echo "[db-plan] missing schema dir: $schema_dir" >&2
  exit 2
fi
if [[ ! -d "$migrations_dir" ]]; then
  echo "[db-plan] missing migrations dir: $migrations_dir" >&2
  exit 2
fi

dev_url="${ATLAS_DEV_URL:-docker://postgres/17/dev?search_path=public}"

echo "[db-plan] atlas schema diff: $module (migrations -> schema)"
if ! diff_out="$(
  ATLAS_NO_UPDATE_NOTIFIER=true \
    ./scripts/db/run_atlas.sh schema diff \
    --from "file://${migrations_dir}?format=goose" \
    --to "file://${schema_dir}" \
    --dev-url "$dev_url" \
    --format '{{ sql . }}'
)"; then
  echo "[db-plan] FAIL: atlas schema diff failed"
  echo "$diff_out"
  exit 1
fi

if [[ -n "${diff_out:-}" ]]; then
  echo "[db-plan] FAIL: migrations drift from schema SSOT (update migrations + atlas.sum and commit)"
  echo "$diff_out"
  exit 1
fi

echo "[db-plan] OK: no drift"
