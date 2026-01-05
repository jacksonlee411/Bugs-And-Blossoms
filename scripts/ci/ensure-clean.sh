#!/usr/bin/env bash
set -euo pipefail

if ! command -v git >/dev/null 2>&1; then
  echo "[ensure-clean] git not found; skip"
  exit 0
fi

if [[ ! -d .git ]]; then
  echo "[ensure-clean] not a git repo; skip"
  exit 0
fi

if [[ -n "$(git status --porcelain=v1)" ]]; then
  echo "[ensure-clean] working tree dirty after generation/formatting" >&2
  git status --porcelain=v1 >&2
  exit 1
fi

