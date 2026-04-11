#!/usr/bin/env bash
set -euo pipefail

echo "[test] go test ./... (with configured coverage policy)"
./scripts/ci/coverage.sh
