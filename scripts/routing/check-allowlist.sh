#!/usr/bin/env bash
set -euo pipefail

echo "[routing] running routing gates"
go test ./internal/routing -run '^TestGate' -count=1
