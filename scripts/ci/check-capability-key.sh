#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

echo "[capability-key] scan: block context-leaking literals and dynamic key concatenation"

mapfile -t changed_files < <(git diff --name-only --diff-filter=ACMR)

if [[ ${#changed_files[@]} -eq 0 ]]; then
  echo "[capability-key] no changed files; skip"
  exit 0
fi

violations=()

for file in "${changed_files[@]}"; do
  [[ -f "$file" ]] || continue
  case "$file" in
    docs/*|*.md|*.svg|*.png|*.jpg|*.jpeg|*.gif|*.lock)
      continue
      ;;
    internal/server/assets/web/assets/*)
      continue
      ;;
    *_test.go|*.test.ts|*.spec.ts|*.spec.tsx)
      continue
      ;;
    scripts/ci/check-capability-key.sh)
      continue
      ;;
  esac

  case "$file" in
    *.go|*.ts|*.tsx|*.js|*.json|*.yaml|*.yml)
      ;;
    *)
      continue
      ;;
  esac

  while IFS= read -r line; do
    [[ "$line" == +++* ]] && continue
    [[ "$line" == +* ]] || continue
    content="${line:1}"

    if printf '%s\n' "$content" | grep -Eq '(capability_key|capabilityKey|CapabilityKey)[^"'"'"']*["'"'"'][^"'"'"']*(setid|scope|tenant|bu(_|$|[0-9]))[^"'"'"']*["'"'"']'; then
      violations+=("$file: context token in capability_key literal -> $content")
      continue
    fi

    if printf '%s\n' "$content" | grep -Eq '((capability_key|capabilityKey|CapabilityKey).*[+]|[+].*(capability_key|capabilityKey|CapabilityKey))'; then
      violations+=("$file: dynamic capability_key concatenation -> $content")
      continue
    fi
  done < <(git diff -U0 -- "$file" || true)
done

if [[ ${#violations[@]} -gt 0 ]]; then
  echo "[capability-key] FAIL: found invalid capability_key additions" >&2
  printf '%s\n' "${violations[@]}" >&2
  exit 1
fi

echo "[capability-key] OK"
