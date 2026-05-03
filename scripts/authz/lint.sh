#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

model="$root/config/access/model.conf"
policy="$root/config/access/policy.csv"

if [[ ! -s "$model" ]]; then
  echo "[authz-lint] missing/empty model: ${model#"$root/"}" >&2
  exit 1
fi
if [[ ! -s "$policy" ]]; then
  echo "[authz-lint] missing/empty policy.csv (run make authz-pack): ${policy#"$root/"}" >&2
  exit 1
fi

go run ./cmd/authz-lint

echo "[authz-lint] OK"
