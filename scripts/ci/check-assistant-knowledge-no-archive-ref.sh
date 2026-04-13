#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

prefix="[assistant-knowledge-no-archive-ref]"
hits="$(rg -n '^\s*-\s*docs/archive/' internal/server/assistant_knowledge_md || true)"

if [[ -n "$hits" ]]; then
  echo "${prefix} FAIL: archive refs are not allowed in markdown knowledge source_refs" >&2
  printf '%s\n' "$hits" >&2
  exit 1
fi

echo "${prefix} OK"
