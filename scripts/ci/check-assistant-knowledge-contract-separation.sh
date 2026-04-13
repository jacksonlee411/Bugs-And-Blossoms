#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

prefix="[assistant-knowledge-contract-separation]"
pattern='PolicyContextContractVersion|PrecheckProjectionContractVersion|CommitAdapterKey|DryRunKey|CapabilityBucketKey|submit_[a-z_]+_event|ActionSchema|PolicyContext|PrecheckProjection'

hits="$(rg -n -S "$pattern" internal/server/assistant_knowledge_md || true)"

if [[ -n "$hits" ]]; then
  echo "${prefix} FAIL: markdown knowledge must not own contract internals" >&2
  printf '%s\n' "$hits" >&2
  exit 1
fi

echo "${prefix} OK"
