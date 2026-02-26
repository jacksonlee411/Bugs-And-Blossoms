#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

echo "[policy-baseline-dup] verify baseline/override resolution contract"
go test ./internal/server -run '^TestHandleSetIDStrategyRegistryAPI_RedundantIntentOverride|TestResolveFieldDecisionFromItems_IntentBucketPrecedence$' -count=1

echo "[policy-baseline-dup] OK"
