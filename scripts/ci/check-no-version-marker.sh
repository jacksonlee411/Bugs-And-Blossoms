#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

if ! command -v rg >/dev/null 2>&1; then
  echo "[naming] missing dependency: rg (ripgrep)" >&2
  exit 1
fi

prefix_lower="v"
prefix_upper="V"
digit="4"

pattern_lower="$(printf '%s%s' "$prefix_lower" "$digit")"
pattern_upper="$(printf '%s%s' "$prefix_upper" "$digit")"

exclude_globs=(
  --glob '!.git/**'
  --glob '!**/*_templ.go'
)

echo "[naming] scan: disallow '$prefix_lower/$prefix_upper' adjacent '$digit'"

content_hits="$(rg -n "(?i)(${pattern_lower}|${pattern_upper})" -S --hidden "${exclude_globs[@]}" || true)"
filename_hits="$(rg --files --hidden "${exclude_globs[@]}" | rg -n -S "(?i)(${pattern_lower}|${pattern_upper})" || true)"

if [[ -n "$filename_hits" || -n "$content_hits" ]]; then
  echo "[naming] FAIL: found disallowed version marker" >&2

  if [[ -n "$filename_hits" ]]; then
    echo "[naming] filename hits:" >&2
    printf '%s\n' "$filename_hits" >&2
  fi

  if [[ -n "$content_hits" ]]; then
    echo "[naming] content hits:" >&2
    printf '%s\n' "$content_hits" >&2
  fi

  exit 1
fi

echo "[naming] OK"
