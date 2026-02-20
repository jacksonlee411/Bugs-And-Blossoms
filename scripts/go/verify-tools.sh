#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

tool="${1:-all}"

mod_version() {
  local module="$1"
  (cd "$root" && go list -m -f '{{.Version}}' "$module" 2>/dev/null || true)
}

verify_sqlc() {
  local expected actual
  expected="$(mod_version github.com/sqlc-dev/sqlc)"
  if [[ -z "$expected" ]]; then
    echo "[go-tools] missing module github.com/sqlc-dev/sqlc in go.mod tool directives" >&2
    exit 1
  fi
  actual="$(cd "$root" && go tool sqlc version 2>/dev/null | awk 'NR==1 {print $1}')"
  if [[ -z "$actual" ]]; then
    echo "[go-tools] failed to execute go tool sqlc" >&2
    exit 1
  fi
  if [[ "$actual" != "$expected" ]]; then
    echo "[go-tools] sqlc version mismatch: expected=$expected actual=$actual" >&2
    exit 1
  fi
}

verify_goose() {
  local expected actual
  expected="$(mod_version github.com/pressly/goose/v3)"
  if [[ -z "$expected" ]]; then
    echo "[go-tools] missing module github.com/pressly/goose/v3 in go.mod tool directives" >&2
    exit 1
  fi
  actual="$(cd "$root" && go tool goose -version 2>/dev/null | awk '{print $3}')"
  if [[ -z "$actual" ]]; then
    echo "[go-tools] failed to execute go tool goose" >&2
    exit 1
  fi
  if [[ "$actual" != "$expected" ]]; then
    echo "[go-tools] goose version mismatch: expected=$expected actual=$actual" >&2
    exit 1
  fi
}

verify_goimports() {
  local expected
  expected="$(mod_version golang.org/x/tools)"
  if [[ -z "$expected" ]]; then
    echo "[go-tools] missing module golang.org/x/tools in go.mod tool directives" >&2
    exit 1
  fi
  if ! (cd "$root" && printf 'package p\n' | go tool goimports >/dev/null 2>&1); then
    echo "[go-tools] failed to execute go tool goimports" >&2
    exit 1
  fi
}

case "$tool" in
  sqlc)
    verify_sqlc
    ;;
  goose)
    verify_goose
    ;;
  goimports)
    verify_goimports
    ;;
  all)
    verify_sqlc
    verify_goose
    verify_goimports
    ;;
  *)
    echo "usage: verify-tools.sh [all|sqlc|goose|goimports]" >&2
    exit 2
    ;;
esac

echo "[go-tools] OK: ${tool}"
