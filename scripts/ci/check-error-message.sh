#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$root"

echo "[error-message] go test ./internal/routing"
go test ./internal/routing -run "TestWriteError_(RewritesGenericMessageFromCode|HumanizesUnknownGenericCode|KeepExplicitMessage)" -count=1

resolver_file="apps/web/src/errors/presentApiError.ts"
if [[ ! -f "$resolver_file" ]]; then
  echo "[error-message] missing resolver file: $resolver_file" >&2
  exit 1
fi

if command -v rg >/dev/null 2>&1; then
  has_resolver() { rg -q "export function resolveApiErrorMessage" "$resolver_file"; }
  has_empty_i18n() { rg -n "en: ''|zh: ''" "$resolver_file" >/dev/null; }
else
  has_resolver() { grep -q "export function resolveApiErrorMessage" "$resolver_file"; }
  has_empty_i18n() { grep -nE "en: ''|zh: ''" "$resolver_file" >/dev/null; }
fi

if ! has_resolver; then
  echo "[error-message] resolveApiErrorMessage not found in $resolver_file" >&2
  exit 1
fi

if has_empty_i18n; then
  echo "[error-message] localized message contains empty en/zh value" >&2
  exit 1
fi

pnpm_cmd=()
if command -v pnpm >/dev/null 2>&1; then
  pnpm_cmd=(pnpm)
elif command -v corepack >/dev/null 2>&1; then
  pnpm_cmd=(corepack pnpm)
else
  echo "[error-message] pnpm is required (please enable corepack or install pnpm)." >&2
  exit 1
fi

echo "[error-message] ${pnpm_cmd[*]} -C apps/web test -- src/errors/presentApiError.test.ts"
"${pnpm_cmd[@]}" -C apps/web test -- src/errors/presentApiError.test.ts

echo "[error-message] OK"
