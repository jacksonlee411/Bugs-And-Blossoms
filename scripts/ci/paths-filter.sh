#!/usr/bin/env bash
set -euo pipefail

files="$(./scripts/ci/changed-files.sh)"

has() {
  local pattern="${1:?}"
  if [[ -z "$files" ]]; then
    return 1
  fi
  echo "$files" | grep -Eq "$pattern"
}

set_out() {
  local key="${1:?}"
  local val="${2:?}"
  if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
    echo "${key}=${val}" >>"$GITHUB_OUTPUT"
  else
    echo "${key}=${val}"
  fi
}

set_out "docs" "$(has '^(AGENTS\\.md|docs/)' && echo true || echo false)"
set_out "go" "$(has '(\\.go$|^go\\.(mod|sum)$|^cmd/|^internal/|^modules/|^pkg/)' && echo true || echo false)"
set_out "routing" "$(has '^(config/routing/|scripts/routing/|docs/dev-plans/017-)' && echo true || echo false)"
set_out "ui" "$(has '^(Makefile|scripts/ui/|apps/web/|internal/server/assets/web/)' && echo true || echo false)"
set_out "i18n" "$(has '^(i18n/|config/i18n/)' && echo true || echo false)"
set_out "db" "$(has '^(atlas\\.hcl|migrations/|compose\\.dev\\.yml|compose\\.yml|scripts/db/|modules/.+/infrastructure/)' && echo true || echo false)"
set_out "sqlc" "$(has '^(sqlc\\.ya?ml|internal/sqlc/|modules/.+/infrastructure/sqlc/|modules/.+/infrastructure/persistence/schema/)' && echo true || echo false)"
set_out "authz" "$(has '^(config/access/|policies/|scripts/authz/)' && echo true || echo false)"
set_out "e2e" "$(has '^(e2e/|apps/web/|playwright\\.)' && echo true || echo false)"

if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
  {
    echo "files<<EOF"
    echo "${files}"
    echo "EOF"
  } >>"$GITHUB_OUTPUT"
fi
