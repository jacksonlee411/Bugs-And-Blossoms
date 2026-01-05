#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

if [[ ! -f "$root/sqlc.yaml" ]]; then
  echo "[sqlc] no sqlc.yaml; no-op"
  exit 0
fi

echo "[sqlc] export schema"
"$root/scripts/sqlc/export-schema.sh"

echo "[sqlc] install sqlc/goimports"
"$root/scripts/sqlc/install_sqlc.sh"
"$root/scripts/sqlc/install_goimports.sh"

echo "[sqlc] generate"
"$root/bin/sqlc" generate -f "$root/sqlc.yaml"

echo "[sqlc] format gen"
gofmt -w "$root/modules"/*/infrastructure/sqlc/gen 2>/dev/null || true
"$root/bin/goimports" -w "$root/modules"/*/infrastructure/sqlc/gen 2>/dev/null || true

