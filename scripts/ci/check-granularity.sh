#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

echo "[granularity] scan: block org_level and scope_type/scope_key additions"

mapfile -t changed_files < <(git diff --name-only --diff-filter=ACMR)

if [[ ${#changed_files[@]} -eq 0 ]]; then
  echo "[granularity] no changed files; skip"
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
    internal/sqlc/schema.sql)
      continue
      ;;
    modules/*/infrastructure/sqlc/gen/*)
      continue
      ;;
    *_test.go|*.test.ts|*.spec.ts|*.spec.tsx)
      continue
      ;;
    scripts/ci/check-granularity.sh)
      continue
      ;;
  esac

  case "$file" in
    *.go|*.ts|*.tsx|*.js|*.json|*.yaml|*.yml|*.sh)
      ;;
    *)
      continue
      ;;
  esac

  while IFS= read -r line; do
    [[ "$line" == +++* ]] && continue
    [[ "$line" == +* ]] || continue
    content="${line:1}"

    if [[ "$content" =~ (^|[^a-zA-Z0-9_])(scope_type|scope_key|scopeType|scopeKey|ScopeType|ScopeKey)([^a-zA-Z0-9_]|$) ]]; then
      violations+=("$file: scope_type/scope_key added -> $content")
      continue
    fi

    if [[ "$content" =~ (^|[^a-zA-Z0-9_])(org_level|orgLevel|OrgLevel)([^a-zA-Z0-9_]|$) ]]; then
      violations+=("$file: org_level added (use org_applicability) -> $content")
      continue
    fi
  done < <(git diff -U0 -- "$file" || true)
done

if [[ ${#violations[@]} -gt 0 ]]; then
  echo "[granularity] FAIL: found forbidden granularity markers" >&2
  printf "%s\n" "${violations[@]}" >&2
  exit 1
fi

echo "[granularity] OK"
