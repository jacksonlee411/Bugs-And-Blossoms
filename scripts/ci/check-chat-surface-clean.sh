#!/usr/bin/env bash
set -euo pipefail

prefix="[chat-surface-clean]"

patterns=(
  '(^|/)(assistant|cubebox|librechat)(/|_|-)'
  '/app/assistant'
  '/app/cubebox'
  '/internal/assistant'
  '/internal/cubebox'
  '/assistant-ui'
  '/assets/librechat-web'
  'ASSISTANT_MODEL_CONFIG_JSON'
  'assistant\.workspace'
  'cubebox\.workspace'
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
  '--glob' '!docs/dev-records/**'
  '--glob' '!docs/dev-plans/392-remove-assistant-cubebox-and-librechat-rebuild-plan.md'
  '--glob' '!docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md'
  '--glob' '!docs/dev-plans/431-codex-ui-protocol-and-shell-reuse-plan.md'
  '--glob' '!docs/dev-plans/432-codex-session-persistence-reuse-plan.md'
  '--glob' '!docs/dev-plans/433-bifrost-centric-ai-gateway-reuse-and-reconstruction-plan.md'
  '--glob' '!docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md'
  '--glob' '!docs/dev-plans/435-bifrost-centric-model-config-ui-and-admin-governance-plan.md'
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

echo "${prefix} OK"
