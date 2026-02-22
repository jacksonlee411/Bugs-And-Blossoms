#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

echo "[as-of-explicit] scan: disallow implicit today/default date fallback in 070/071 runtime scope"

go_targets=(
  "internal/server/setid_api.go"
  "internal/server/setid_scope_api.go"
  "internal/server/jobcatalog_api.go"
  "internal/server/staffing_handlers.go"
  "modules/staffing/presentation/controllers/assignments_api.go"
)

sql_targets=(
  "modules/orgunit/infrastructure/persistence/schema/00006_orgunit_setid_engine.sql"
  "internal/sqlc/schema.sql"
)

for file in "${go_targets[@]}" "${sql_targets[@]}"; do
  if [[ ! -f "$file" ]]; then
    echo "[as-of-explicit] FAIL: missing expected file: $file" >&2
    exit 1
  fi
done

violations=0

check_pattern() {
  local kind="${1:?}"
  local desc="${2:?}"
  local pattern="${3:?}"
  shift 3
  local -a files=("$@")

  local hits=""
  if command -v rg >/dev/null 2>&1; then
    hits="$(rg -nU --pcre2 "$pattern" "${files[@]}" || true)"
  else
    hits="$(grep -RInE -- "$pattern" "${files[@]}" || true)"
  fi

  if [[ -n "$hits" ]]; then
    violations=1
    echo "[as-of-explicit] FAIL (${kind}): ${desc}" >&2
    printf '%s\n' "$hits" >&2
  fi
}

check_pattern "go" "as_of 回填为系统当天" '(\\basOf\\s*(:=|=)\\s*[^\\n]*(time\\.Now\\(\\)|NowUTC\\(\\))[^\\n]*\\.Format\\("2006-01-02"\\))' "${go_targets[@]}"
check_pattern "go" "effective_date 从 as_of/系统时间隐式回填" '(\\b(req\\.)?EffectiveDate\\s*=\\s*(asOf|[^\\n]*time\\.Now\\(\\)\\.UTC\\(\\)\\.Format\\("2006-01-02"\\)))' "${go_targets[@]}"
check_pattern "go" "as_of 缺失分支内出现赋值回填" '(if\\s+[^\\n{]*\\basOf\\b[^\\n{]*==\\s*""\\s*\\{(?:(?!\\n\\s*\\}).)*\\basOf\\s*=)' "${go_targets[@]}"
check_pattern "go" "effective_date 缺失分支内出现赋值回填" '(if\\s+[^\\n{]*\\bEffectiveDate\\b[^\\n{]*==\\s*""\\s*\\{(?:(?!\\n\\s*\\}).)*\\bEffectiveDate\\s*=)' "${go_targets[@]}"

check_pattern "sql" "SQL 逻辑使用 current_date 作为业务生效日兜底" '(v_(effective_date|as_of_date)\\s*:=\\s*current_date)' "${sql_targets[@]}"
check_pattern "sql" "SQL 使用 validity @> current_date（应使用显式日期参数）" '(validity\\s*@>\\s*current_date)' "${sql_targets[@]}"

if [[ "$violations" -ne 0 ]]; then
  echo "[as-of-explicit] FAIL: implicit date fallback detected" >&2
  exit 1
fi

echo "[as-of-explicit] OK"
