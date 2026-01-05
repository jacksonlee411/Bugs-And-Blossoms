#!/usr/bin/env bash
set -euo pipefail

cfg=".gocleanarch.yml"
if [[ ! -f "$cfg" ]]; then
  echo "[cleanarch] missing config: $cfg" >&2
  exit 1
fi

get() {
  local key="${1:?}"
  awk -F': ' -v k="$key" '$1==k{print $2}' "$cfg" | head -n1
}

domain="$(get domain)"
application="$(get application)"
infrastructure="$(get infrastructure)"
interfaces="$(get interfaces)"
ignore_tests="$(get ignore_tests)"

args=(
  "-domain" "$domain"
  "-application" "$application"
  "-infrastructure" "$infrastructure"
  "-interfaces" "$interfaces"
)
if [[ "$ignore_tests" == "true" ]]; then
  args+=("-ignore-tests")
fi

go run github.com/roblaszczak/go-cleanarch@v1.2.1 "${args[@]}"

