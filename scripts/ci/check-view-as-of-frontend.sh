#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

echo "[view-as-of-frontend] scan: disallow page-local date fallback and read/write date coupling"

allowlist_file="scripts/ci/view-as-of-frontend-allowlist.txt"
if [[ ! -f "$allowlist_file" ]]; then
  echo "[view-as-of-frontend] FAIL: missing allowlist file: $allowlist_file" >&2
  exit 1
fi

mapfile -t raw_as_of_allowlist < <(grep -v '^\s*#' "$allowlist_file" | sed '/^\s*$/d')

violations=0

check_pattern() {
  local desc="${1:?}"
  local pattern="${2:?}"
  local hits
  hits="$(rg -n --glob '!**/*.test.*' --glob '!**/dist/**' --glob '!**/coverage/**' "$pattern" apps/web/src/pages apps/web/src/components || true)"
  if [[ -n "$hits" ]]; then
    violations=1
    echo "[view-as-of-frontend] FAIL: ${desc}" >&2
    printf '%s\n' "$hits" >&2
  fi
}

check_raw_as_of_label() {
  local hits
  hits="$(rg -n --glob '!**/*.test.*' "label='as_of'" apps/web/src/pages apps/web/src/components || true)"
  [[ -z "$hits" ]] && return 0

  local unexpected=()
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    local file="${line%%:*}"
    local allowed=0
    for path in "${raw_as_of_allowlist[@]}"; do
      if [[ "$file" == "$path" ]]; then
        allowed=1
        break
      fi
    done
    if [[ "$allowed" -eq 0 ]]; then
      unexpected+=("$line")
    fi
  done <<< "$hits"

  if [[ "${#unexpected[@]}" -gt 0 ]]; then
    violations=1
    echo "[view-as-of-frontend] FAIL: raw label='as_of' is only allowed in explicit tooling allowlist" >&2
    printf '%s\n' "${unexpected[@]}" >&2
  fi
}

check_pattern "新增 page-local todayISO()，请改用 shared readViewState helper" 'function\s+todayISO\s*\('
check_pattern "新增 parseDateOrDefault()，请改用 parseRequestedAsOf()/resolveReadViewState()" 'function\s+parseDateOrDefault\s*\('
check_pattern "新增 fallbackAsOf，说明页面正在重新引入 today fallback" '\bfallbackAsOf\b'
check_pattern "新增 effectiveDate: asOf，请按动作语义初始化写态日期" 'effectiveDate\s*:\s*asOf\b'
check_pattern "新增 effectiveDate: toDateValue(asOf)，请不要用浏览态 as_of 预填 dialog" 'effectiveDate\s*:\s*toDateValue\(asOf\)'
check_pattern "新增 setEffectiveDate(asOf)，请不要让读态覆盖写态日期" 'setEffectiveDate\(asOf\)'
check_raw_as_of_label

if [[ "$violations" -ne 0 ]]; then
  echo "[view-as-of-frontend] FAIL: anti-regression scan detected forbidden patterns" >&2
  exit 1
fi

echo "[view-as-of-frontend] OK"
