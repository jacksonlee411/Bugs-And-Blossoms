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

if ! rg -q "export function resolveApiErrorMessage" "$resolver_file"; then
  echo "[error-message] resolveApiErrorMessage not found in $resolver_file" >&2
  exit 1
fi

if rg -n "en: ''|zh: ''" "$resolver_file" >/dev/null; then
  echo "[error-message] localized message contains empty en/zh value" >&2
  exit 1
fi

if ! command -v pnpm >/dev/null 2>&1; then
  echo "[error-message] pnpm is required (please enable corepack or install pnpm)." >&2
  exit 1
fi

echo "[error-message] pnpm -C apps/web test -- src/errors/presentApiError.test.ts"
pnpm -C apps/web test -- src/errors/presentApiError.test.ts

echo "[error-message] OK"
