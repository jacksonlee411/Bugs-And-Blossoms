#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

echo "[dict-tenant-only] scan: disallow runtime global fallback in dict read paths"

go_targets=(
  "internal/server/dicts_store.go"
)

sql_targets=(
  "modules/iam/infrastructure/persistence/schema/00007_iam_dict_config.sql"
  "modules/iam/infrastructure/persistence/schema/00008_iam_dict_registry.sql"
  "internal/sqlc/schema.sql"
)

for file in "${go_targets[@]}" "${sql_targets[@]}"; do
  if [[ ! -f "$file" ]]; then
    echo "[dict-tenant-only] FAIL: missing expected file: $file" >&2
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
    echo "[dict-tenant-only] FAIL (${kind}): ${desc}" >&2
    printf '%s\n' "$hits" >&2
  fi
}

check_pattern "go" "dict 读路径使用 tenant+global 合并查询" 'tenant_uuid\s+IN\s*\(\$1::uuid,\s*\$3::uuid\)' "${go_targets[@]}"
check_pattern "go" "dict 读路径使用 tenant 优先 global 排序" 'CASE\s+WHEN\s+[^\n]*tenant_uuid\s*=\s*\$1::uuid\s+THEN\s+0\s+ELSE\s+1\s+END' "${go_targets[@]}"
check_pattern "sql" "dict RLS 策略允许 global_tenant 回退" "OR\s+tenant_uuid\s*=\s*'00000000-0000-0000-0000-000000000000'::uuid" "${sql_targets[@]}"

if [[ "$violations" -ne 0 ]]; then
  echo "[dict-tenant-only] FAIL: tenant-only guard violated" >&2
  exit 1
fi

echo "[dict-tenant-only] OK"
