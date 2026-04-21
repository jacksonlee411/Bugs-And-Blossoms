#!/usr/bin/env bash
set -euo pipefail

prefix="[chat-surface-clean]"

patterns=(
  '/app/assistant'
  '/internal/assistant'
  '/assistant-ui'
  '/assets/librechat-web'
  '/librechat'
  'compat window'
  'redirect alias'
  '410 Gone'
  'retired semantics'
  'assistant-config-single-source'
  'assistant-domain-allowlist'
  'assistant-knowledge-single-source'
  'assistant-knowledge-runtime-load'
  'assistant-knowledge-no-json-runtime'
  'assistant-no-legacy-overlay'
  'assistant-no-knowledge-literals'
  'assistant-knowledge-no-archive-ref'
  'assistant-knowledge-contract-separation'
  'assistant-no-knowledge-db'
)

doc_record_patterns=(
  '/app/assistant'
  '/internal/assistant'
  '/assistant-ui'
  '/assets/librechat-web'
  '/librechat'
  'compat window'
  'redirect alias'
  '410 Gone'
  'retired semantics'
  'assistant-config-single-source'
  'assistant-domain-allowlist'
  'assistant-knowledge-single-source'
  'assistant-knowledge-runtime-load'
  'assistant-knowledge-no-json-runtime'
  'assistant-no-legacy-overlay'
  'assistant-no-knowledge-literals'
  'assistant-knowledge-no-archive-ref'
  'assistant-knowledge-contract-separation'
  'assistant-no-knowledge-db'
)

globs=(
  'config'
  'internal'
  'apps'
  'e2e'
  'modules'
  'cmd'
  'migrations'
  'scripts'
  'tools'
  'docs/dev-plans'
  'AGENTS.md'
  'Makefile'
)

ignore_globs=(
  '--glob' '!docs/archive/**'
  '--glob' '!docs/dev-records/DEV-PLAN-436-READINESS.md'
  '--glob' '!docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md'
  '--glob' '!docs/dev-plans/431-codex-ui-protocol-and-shell-reuse-plan.md'
  '--glob' '!docs/dev-plans/432-codex-session-persistence-reuse-plan.md'
  '--glob' '!docs/dev-plans/433-bifrost-centric-ai-gateway-reuse-and-reconstruction-plan.md'
  '--glob' '!docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md'
  '--glob' '!docs/dev-plans/435-bifrost-centric-model-config-ui-and-admin-governance-plan.md'
  '--glob' '!docs/dev-plans/436-cubebox-historical-surface-hard-delete-plan.md'
  '--glob' '!scripts/ci/check-chat-surface-clean.sh'
)

args=()
for pattern in "${patterns[@]}"; do
  args+=(-e "$pattern")
done
for glob in "${ignore_globs[@]}"; do
  args+=("$glob")
done

if rg -n -i "${args[@]}" "${globs[@]}"; then
  echo "${prefix} FAIL: detected legacy chat surface residue" >&2
  exit 1
fi

doc_record_args=()
for pattern in "${doc_record_patterns[@]}"; do
  doc_record_args+=(-e "$pattern")
done
doc_record_args+=(
  '--glob' '!docs/archive/**'
  '--glob' '!docs/dev-records/DEV-PLAN-436-READINESS.md'
)

if rg -n -i "${doc_record_args[@]}" docs/dev-records; then
  echo "${prefix} FAIL: detected legacy chat surface residue in active dev-records" >&2
  exit 1
fi

echo "${prefix} OK"
