#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

echo "[org-node-key-backflow] scan: block legacy org_id/org_node_key DTO backflow and resolver regressions"

mapfile -t changed_files < <(git diff --name-only --diff-filter=ACMR)

if [[ ${#changed_files[@]} -eq 0 ]]; then
  echo "[org-node-key-backflow] no changed files; skip"
  exit 0
fi

violations=()

is_runtime_source_file() {
  local file="${1:?}"

  case "$file" in
    docs/*|*.md|*.svg|*.png|*.jpg|*.jpeg|*.gif|*.lock)
      return 1
      ;;
    scripts/ci/check-org-node-key-backflow.sh)
      return 1
      ;;
    internal/server/assets/*)
      return 1
      ;;
    *_test.go|*.test.ts|*.test.tsx|*.spec.ts|*.spec.tsx)
      return 1
      ;;
    modules/*/infrastructure/persistence/schema/*|modules/*/infrastructure/sqlc/gen/*)
      return 1
      ;;
    cmd/*)
      return 1
      ;;
  esac

  if [[ "$file" =~ ^(internal/server/.*\.go|modules/.*\.go|pkg/.*\.go|apps/web/.*\.(ts|tsx|js|jsx|json)|config/routing/.*\.(yaml|yml|json))$ ]]; then
    return 0
  fi

  return 1
}

is_boundary_contract_file() {
  local file="${1:?}"

  if [[ "$file" =~ ^(internal/server/.*\.go|modules/.*/presentation/.*\.go|apps/web/.*\.(ts|tsx|js|jsx|json)|config/routing/.*\.(yaml|yml|json))$ ]]; then
    return 0
  fi

  return 1
}

for file in "${changed_files[@]}"; do
  [[ -f "$file" ]] || continue
  if ! is_runtime_source_file "$file"; then
    continue
  fi

  while IFS= read -r line; do
    [[ "$line" == +++* ]] && continue
    [[ "$line" == +* ]] || continue
    content="${line:1}"

    if [[ "$content" =~ (^|[^A-Za-z0-9_])ResolveOrgID[[:space:]]*\( ]]; then
      violations+=("$file: legacy resolver marker -> $content")
      continue
    fi

    if [[ "$content" =~ (^|[^A-Za-z0-9_])ResolveOrgCode[[:space:]]*\( ]]; then
      violations+=("$file: legacy resolver marker -> $content")
      continue
    fi

    if [[ "$content" =~ (^|[^A-Za-z0-9_])ResolveOrgCodes[[:space:]]*\( ]]; then
      violations+=("$file: legacy resolver marker -> $content")
      continue
    fi

    if is_boundary_contract_file "$file"; then
      if [[ "$content" =~ (json|form|query|schema|url|uri|path):\"(org_id|org_node_key)\" ]]; then
        violations+=("$file: external DTO must not expose org_id/org_node_key -> $content")
        continue
      fi

      if [[ "$content" =~ [\"\'](org_id|org_node_key)[\"\'] ]] || [[ "$content" =~ (^|[^A-Za-z0-9_])(org_id|org_node_key)(\??:)[[:space:]] ]]; then
        violations+=("$file: boundary contract must not add org_id/org_node_key marker -> $content")
        continue
      fi
    fi

    if [[ "$content" =~ [\"\'](parent_id|new_parent_id)[\"\'] ]]; then
      violations+=("$file: legacy org payload key is forbidden in runtime path -> $content")
      continue
    fi
  done < <(git diff -U0 -- "$file" || true)
done

if [[ ${#violations[@]} -gt 0 ]]; then
  echo "[org-node-key-backflow] FAIL: found forbidden org-node-key backflow marker(s)" >&2
  printf '%s\n' "${violations[@]}" >&2
  exit 1
fi

echo "[org-node-key-backflow] OK"
