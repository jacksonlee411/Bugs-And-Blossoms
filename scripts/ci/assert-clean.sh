#!/usr/bin/env bash
set -euo pipefail

if [[ ! -d .git ]]; then
  echo "[assert-clean] not a git repo; skip"
  exit 0
fi

if [[ -n "$(git status --porcelain=v1)" ]]; then
  echo "[assert-clean] working tree dirty; generated artifacts likely not committed" >&2
  git status --porcelain=v1 >&2
  exit 1
fi

echo "[assert-clean] OK"

