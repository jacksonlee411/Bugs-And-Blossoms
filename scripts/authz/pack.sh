#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

policies_dir="$root/config/access/policies"
out_csv="$root/config/access/policy.csv"
out_rev="$root/config/access/policy.csv.rev"

if [[ ! -d "$policies_dir" ]]; then
  echo "[authz-pack] missing policies dir: ${policies_dir#"$root/"}" >&2
  exit 2
fi

tmp="$(mktemp)"

{
  while IFS= read -r f; do
    cat "$f"
    echo
  done < <(find "$policies_dir" -type f -name "*.csv" | LC_ALL=C sort)
} >"$tmp"

mkdir -p "$(dirname "$out_csv")"
mv "$tmp" "$out_csv"
chmod 0644 "$out_csv"

sum() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256
  else
    echo "missing sha256sum/shasum" >&2
    exit 1
  fi
}

{
  cat "$root/config/access/model.conf"
  echo
  cat "$out_csv"
} | sum | awk '{print $1}' >"$out_rev"
chmod 0644 "$out_rev"

echo "[authz-pack] OK"
