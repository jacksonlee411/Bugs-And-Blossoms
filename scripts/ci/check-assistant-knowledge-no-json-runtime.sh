#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

prefix="[assistant-knowledge-no-json-runtime]"

if find internal/server/assistant_knowledge -type f | grep -q . 2>/dev/null; then
  echo "${prefix} FAIL: retired JSON knowledge assets must not exist" >&2
  find internal/server/assistant_knowledge -type f | sort >&2
  exit 1
fi

hits="$(rg -n -S --hidden \
  --glob '!**/*_test.go' \
  --glob '!internal/server/assistant_knowledge_md/**' \
  'assistant_knowledge/|assistantLoadInterpretationPacks|assistantLoadActionViewPacks|assistantLoadReplyGuidancePacks|go:embed assistant_knowledge/\*\.json' \
  internal/server cmd modules pkg || true)"

if [[ -n "$hits" ]]; then
  echo "${prefix} FAIL: found JSON-runtime residue" >&2
  printf '%s\n' "$hits" >&2
  exit 1
fi

echo "${prefix} OK"
