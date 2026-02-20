#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

if [[ ! -f "$root/sqlc.yaml" ]]; then
  echo "[sqlc] no sqlc.yaml; no-op"
  exit 0
fi

echo "[sqlc] export schema"
"$root/scripts/sqlc/export-schema.sh"

echo "[sqlc] verify sqlc/goimports tools"
"$root/scripts/go/verify-tools.sh" sqlc
"$root/scripts/go/verify-tools.sh" goimports

echo "[sqlc] generate"
go tool sqlc generate -f "$root/sqlc.yaml"

echo "[sqlc] format gen"
gofmt -w "$root/modules"/*/infrastructure/sqlc/gen 2>/dev/null || true
go tool goimports -w "$root/modules"/*/infrastructure/sqlc/gen 2>/dev/null || true
