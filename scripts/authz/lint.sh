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

bad=0
while IFS= read -r line; do
  [[ -z "$line" ]] && continue
  case "$line" in
    p,*) ;;
    g,*) ;;
    *)
      echo "[authz-lint] invalid line (must start with p, or g,): $line" >&2
      bad=1
      ;;
  esac
done <"$policy"

if [[ "$bad" -ne 0 ]]; then
  exit 1
fi

echo "[authz-lint] OK"

