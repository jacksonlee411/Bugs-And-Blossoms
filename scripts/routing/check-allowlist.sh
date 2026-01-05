#!/usr/bin/env bash
set -euo pipefail

allowlist="config/routing/allowlist.yaml"

if [[ ! -f "$allowlist" ]]; then
  echo "[routing] missing allowlist: $allowlist" >&2
  exit 1
fi

missing=0
if ! grep -Eq '^[[:space:]]*server:[[:space:]]*$' "$allowlist"; then
  echo "[routing] allowlist missing entrypoint key: server" >&2
  missing=1
fi
if ! grep -Eq '^[[:space:]]*superadmin:[[:space:]]*$' "$allowlist"; then
  echo "[routing] allowlist missing entrypoint key: superadmin" >&2
  missing=1
fi

if [[ "$missing" -ne 0 ]]; then
  exit 1
fi

echo "[routing] allowlist OK ($allowlist)"

