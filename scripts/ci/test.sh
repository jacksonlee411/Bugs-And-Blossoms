#!/usr/bin/env bash
set -euo pipefail

echo "[test] go test ./... (with 100% coverage policy)"
./scripts/ci/coverage.sh

