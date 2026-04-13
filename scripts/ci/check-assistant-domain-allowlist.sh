#!/usr/bin/env bash
set -euo pipefail

prefix="[assistant-domain-allowlist]"
repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

policy_file="config/assistant/domain-allowlist.yaml"
if [[ ! -f "$policy_file" ]]; then
  echo "${prefix} FAIL R1 ${policy_file}: domain allowlist file is missing" >&2
  exit 1
fi

if ! grep -q '^default:[[:space:]]*deny[[:space:]]*$' "$policy_file"; then
  echo "${prefix} FAIL R1 ${policy_file}: default must be deny" >&2
  exit 1
fi

echo "${prefix} go test ./internal/server -run TestAssistantDomainPolicy"
go test ./internal/server -run 'TestAssistantDomainPolicyRepoConfigIsValid|TestValidateAssistantDomainPolicy' -count=1

if [[ ! -f "Makefile" ]]; then
  echo "${prefix} FAIL R3 Makefile: missing Makefile" >&2
  exit 1
fi
if ! grep -q '^assistant-domain-allowlist:' Makefile; then
  echo "${prefix} FAIL R3 Makefile: missing assistant-domain-allowlist target" >&2
  exit 1
fi
if ! grep -q '\$(MAKE) check assistant-domain-allowlist' Makefile; then
  echo "${prefix} FAIL R3 Makefile: missing preflight hook for assistant-domain-allowlist" >&2
  exit 1
fi

workflow_file=".github/workflows/quality-gates.yml"
if [[ ! -f "$workflow_file" ]]; then
  echo "${prefix} FAIL R3 ${workflow_file}: workflow is missing" >&2
  exit 1
fi
if ! grep -q 'make check assistant-domain-allowlist' "$workflow_file"; then
  echo "${prefix} FAIL R3 ${workflow_file}: missing CI step for assistant-domain-allowlist gate" >&2
  exit 1
fi

docs_with_gate=0
for doc in \
  docs/dev-plans/012-ci-quality-gates.md \
  docs/archive/dev-plans/230-librechat-project-level-integration-plan.md \
  docs/archive/dev-plans/234-librechat-open-source-capabilities-reuse-plan.md; do
  if [[ -f "$doc" ]] && grep -q 'assistant-domain-allowlist' "$doc"; then
    docs_with_gate=$((docs_with_gate + 1))
  fi
done
if ((docs_with_gate < 2)); then
  echo "${prefix} FAIL R3 docs/dev-plans: assistant-domain-allowlist must be documented in at least two plan docs (012/archive-230/archive-234)" >&2
  exit 1
fi

echo "${prefix} OK"
