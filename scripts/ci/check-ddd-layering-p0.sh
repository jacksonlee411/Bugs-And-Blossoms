#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

echo "[ddd-layering-p0] scan: block new DDD layering drift in added lines"

mapfile -t changed_files < <(git diff --name-only --diff-filter=ACMR)

if [[ ${#changed_files[@]} -eq 0 ]]; then
  echo "[ddd-layering-p0] no changed files; skip"
  exit 0
fi

violations=()

is_allowed_internal_server_infra_exception() {
  local file="${1:?}"
  local content="${2:?}"

  case "$file" in
    internal/server/handler.go|internal/server/person.go)
      if [[ "$content" == *'github.com/jacksonlee411/Bugs-And-Blossoms/modules/person/infrastructure/persistence'* ]]; then
        return 0
      fi
      ;;
  esac

  return 1
}

for file in "${changed_files[@]}"; do
  [[ -f "$file" ]] || continue

  case "$file" in
    docs/*|*.md|*.svg|*.png|*.jpg|*.jpeg|*.gif|*.lock)
      continue
      ;;
    internal/server/assets/*)
      continue
      ;;
    scripts/ci/check-ddd-layering-p0.sh)
      continue
      ;;
    *_test.go|*.test.ts|*.spec.ts|*.spec.tsx)
      continue
      ;;
  esac

  case "$file" in
    *.go)
      ;;
    *)
      continue
      ;;
  esac

  module_name=""
  if [[ "$file" =~ ^modules/([^/]+)/infrastructure/.*\.go$ ]]; then
    module_name="${BASH_REMATCH[1]}"
  fi

  while IFS= read -r line; do
    [[ "$line" == +++* ]] && continue
    [[ "$line" == +* ]] || continue
    content="${line:1}"

    if [[ "$file" =~ ^internal/server/.*\.go$ ]]; then
      if [[ "$content" =~ github\.com/.*/Bugs-And-Blossoms/modules/.*/infrastructure(/|\"|$) ]]; then
        if is_allowed_internal_server_infra_exception "$file" "$content"; then
          continue
        fi
        violations+=("$file: internal/server must not add module infrastructure import -> $content")
        continue
      fi

      if [[ "$content" =~ github\.com/.*/Bugs-And-Blossoms/modules/.*/presentation(/|\"|$) ]]; then
        violations+=("$file: internal/server must not add module presentation import -> $content")
        continue
      fi

      if [[ "$content" =~ ^[[:space:]]*type[[:space:]]+[A-Za-z0-9_]*PGStore[[:space:]]+struct ]]; then
        violations+=("$file: internal/server must not add new module-level PG store type -> $content")
        continue
      fi

      if [[ "$content" =~ ^[[:space:]]*func[[:space:]]+new[A-Za-z0-9_]*PGStore[[:space:]]*\( ]]; then
        violations+=("$file: internal/server must not add new module-level PG store constructor -> $content")
        continue
      fi

      if [[ "$content" =~ submit_[a-z0-9_]+_event|apply_[a-z0-9_]+_logic ]]; then
        violations+=("$file: internal/server must not add new direct kernel marker -> $content")
        continue
      fi
    fi

    if [[ -n "$module_name" ]]; then
      service_import="github.com/jacksonlee411/Bugs-And-Blossoms/modules/${module_name}/services"
      if [[ "$content" == *"$service_import"* ]]; then
        violations+=("$file: infrastructure must not add reverse dependency on same-module services -> $content")
        continue
      fi
    fi
  done < <(git diff -U0 -- "$file" || true)
done

if [[ ${#violations[@]} -gt 0 ]]; then
  echo "[ddd-layering-p0] FAIL: found forbidden new layering drift" >&2
  printf '%s\n' "${violations[@]}" >&2
  exit 1
fi

echo "[ddd-layering-p0] OK"
