#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

echo "[no-scope-package] scan: block newly added scope/package runtime markers"

mapfile -t changed_files < <(git diff --name-only --diff-filter=ACMR)

if [[ ${#changed_files[@]} -eq 0 ]]; then
  echo "[no-scope-package] no changed files; skip"
  exit 0
fi

violations=()

for file in "${changed_files[@]}"; do
  [[ -f "$file" ]] || continue
  case "$file" in
    docs/*|*.md|*.svg|*.png|*.jpg|*.jpeg|*.gif|*.lock)
      continue
      ;;
    config/capability/contract-freeze.v1.json)
      continue
      ;;
    internal/server/assets/web/assets/*)
      continue
      ;;
    scripts/ci/check-no-scope-package.sh)
      continue
      ;;
  esac

  case "$file" in
    *.go|*.sql|*.ts|*.tsx|*.js|*.json|*.yaml|*.yml|*.sh)
      ;;
    *)
      continue
      ;;
  esac

  while IFS= read -r line; do
    [[ "$line" == +++* ]] && continue
    [[ "$line" == +* ]] || continue
    content="${line:1}"

    if [[ "$content" =~ (^|[^a-zA-Z0-9_])(scope_code|scope_package|scope_subscription|package_id)([^a-zA-Z0-9_]|$) ]]; then
      violations+=("$file: $content")
      continue
    fi

    if [[ "$content" =~ /org/api/(scope-packages|owned-scope-packages|scope-subscriptions|global-scope-packages) ]]; then
      violations+=("$file: $content")
      continue
    fi
  done < <(git diff -U0 -- "$file" || true)
done

if [[ ${#violations[@]} -gt 0 ]]; then
  echo "[no-scope-package] FAIL: found newly added scope/package markers" >&2
  printf '%s\n' "${violations[@]}" >&2
  exit 1
fi

echo "[no-scope-package] OK"
