#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

echo "[no-legacy] scan: disallow legacy branches/fallbacks in runtime sources"

exclude_globs=(
  --glob '!.git/**'
  --glob '!.github/**'
  --glob '!docs/**'
  --glob '!**/*_templ.go'
  --glob '!**/pnpm-lock.yaml'
  --glob '!**/package-lock.json'
  --glob '!**/yarn.lock'
  --glob '!scripts/ci/**'
  --glob '!scripts/ci/check-no-legacy.sh'
  --glob '!internal/server/assets/shoelace/vendor/**'
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
pattern='(?i)(\bread=legacy\b|\buse_legacy\b|\blegacy_mode\b|\blegacy_\w+|\w+_legacy\b|\blegacy\b|ReadLegacy|orgunit\.nodes|submit_orgunit_event)'

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

echo "[no-legacy] OK"
