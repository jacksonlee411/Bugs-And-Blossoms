#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

echo "[authz-test] pack + lint"
"$root/scripts/authz/pack.sh"
"$root/scripts/authz/lint.sh"

echo "[authz-test] OK"

