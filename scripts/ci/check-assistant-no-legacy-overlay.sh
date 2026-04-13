#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

prefix="[assistant-no-legacy-overlay]"
pattern='overlay|pass-through|passthrough|mixed-source|mixed_source|partial ownership|json snapshot|snapshot export'

hits="$(rg -n -i -S --hidden \
  --glob '*.go' \
  --glob '!**/*_test.go' \
  "$pattern" \
  internal/server || true)"

if [[ -n "$hits" ]]; then
  echo "${prefix} FAIL: found legacy overlay / mixed-source residue" >&2
  printf '%s\n' "$hits" >&2
  exit 1
fi

echo "${prefix} OK"
