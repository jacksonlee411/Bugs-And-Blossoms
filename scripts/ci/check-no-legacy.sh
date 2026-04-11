#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

echo "[no-legacy] scan: disallow legacy branches/fallbacks in runtime sources"

exclude_globs=(
  --glob '!.git/**'
  --glob '!.github/**'
  --glob '!docs/**'
  --glob '!third_party/**'
  --glob '!**/*_templ.go'
  --glob '!**/pnpm-lock.yaml'
  --glob '!**/package-lock.json'
  --glob '!**/yarn.lock'
  --glob '!scripts/ci/**'
  --glob '!scripts/ci/check-no-legacy.sh'
  --glob '!internal/server/assets/shoelace/vendor/**'
  --glob '!internal/server/assets/**'
  --glob '!config/capability/contract-freeze*.json'
)

include_globs=(
  --glob '*.go'
  --glob '*.sql'
  --glob '*.sh'
  --glob '*.yaml'
  --glob '*.yml'
  --glob '*.json'
)

# NOTE: docs 允许讨论 legacy；本门禁只约束运行时代码/脚本/迁移与配置。
pattern='(?i)(\bread=legacy\b|\buse_legacy\b|\blegacy_mode\b|\blegacy_\w+|\w+_legacy\b|ReadLegacy|orgunit\.nodes|submit_orgunit_event)'

hits=""
if command -v rg >/dev/null 2>&1; then
  hits="$(rg -n -S --hidden "${include_globs[@]}" "${exclude_globs[@]}" "$pattern" . || true)"
else
  hits="$(
    grep -RIn -E \
      --exclude-dir='.git' \
      --exclude-dir='.github' \
      --exclude-dir='docs' \
      --exclude-dir='scripts/ci' \
      --exclude='*_templ.go' \
      --include='*.go' \
      --include='*.sql' \
      --include='*.sh' \
      --include='*.yaml' \
      --include='*.yml' \
      --include='*.json' \
      -- "$pattern" . || true
  )"
fi

if [[ -n "$hits" ]]; then
  echo "[no-legacy] FAIL: found disallowed legacy marker(s) in runtime sources" >&2
  printf '%s\n' "$hits" >&2
  exit 1
fi

echo "[no-legacy] scan: retired orgunit field-policy runtime/public entrypoints"

retired_include_globs=(
  --glob '*.go'
  --glob '*.ts'
  --glob '*.tsx'
  --glob '*.sql'
  --glob '*.sh'
  --glob '*.yaml'
  --glob '*.yml'
  --glob '*.json'
)

retired_exclude_globs=(
  "${exclude_globs[@]}"
  --glob '!**/*_test.go'
  --glob '!**/*.test.ts'
  --glob '!**/*.test.tsx'
  --glob '!modules/orgunit/infrastructure/persistence/schema/**'
)

retired_pattern='(/org/api/org-units/field-policies(?::disable|:resolve-preview)?\b|upsertOrgUnitFieldPolicy\b|disableOrgUnitFieldPolicy\b|resolveOrgUnitFieldPolicyPreview\b|handleOrgUnitFieldPoliciesAPI\b|handleOrgUnitFieldPoliciesDisableAPI\b|handleOrgUnitFieldPoliciesResolvePreviewAPI\b|ResolveTenantFieldPolicy\b|ListTenantFieldPolicies\b|UpsertTenantFieldPolicy\b|DisableTenantFieldPolicy\b)'

retired_hits=""
if command -v rg >/dev/null 2>&1; then
  retired_hits="$(rg -n -S --hidden "${retired_include_globs[@]}" "${retired_exclude_globs[@]}" "$retired_pattern" . || true)"
else
  retired_hits="$(
    grep -RIn -E \
      --exclude-dir='.git' \
      --exclude-dir='.github' \
      --exclude-dir='docs' \
      --exclude-dir='scripts/ci' \
      --exclude-dir='internal/server/assets' \
      --exclude-dir='modules/orgunit/infrastructure/persistence/schema' \
      --exclude='*_templ.go' \
      --exclude='*_test.go' \
      --exclude='*.test.ts' \
      --exclude='*.test.tsx' \
      --include='*.go' \
      --include='*.ts' \
      --include='*.tsx' \
      --include='*.sql' \
      --include='*.sh' \
      --include='*.yaml' \
      --include='*.yml' \
      --include='*.json' \
      -- "$retired_pattern" . || true
  )"
fi

if [[ -n "$retired_hits" ]]; then
  echo "[no-legacy] FAIL: found retired orgunit field-policy runtime/public symbol(s)" >&2
  printf '%s\n' "$retired_hits" >&2
  exit 1
fi

echo "[no-legacy] OK"
