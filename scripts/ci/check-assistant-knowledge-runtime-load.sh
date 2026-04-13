#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

echo "[assistant-knowledge-runtime-load] go test runtime loader/parity subset"
go test ./internal/server -run 'TestAssistantKnowledgeRuntime_LoadAndRoute|TestAssistantKnowledgeRuntime_LoadersErrorPaths|TestAssistant268SyntheticSemanticHelperCoverage'
echo "[assistant-knowledge-runtime-load] OK"
