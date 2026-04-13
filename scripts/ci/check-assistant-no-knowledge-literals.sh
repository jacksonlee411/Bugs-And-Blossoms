#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

prefix="[assistant-no-knowledge-literals]"
targets=(
  internal/server/assistant_api.go
  internal/server/assistant_model_gateway.go
  internal/server/assistant_semantic_state.go
  internal/server/assistant_context_assembler.go
  internal/server/assistant_reply_nlg.go
)

pattern='org\.orgunit_|knowledge\.general_qa|chat\.greeting|route\.uncertain|当前轮属于知识问答|当前轮属于闲聊响应|当前轮语义仍不确定'
hits="$(rg -n -S "$pattern" "${targets[@]}" || true)"

if [[ -n "$hits" ]]; then
  echo "${prefix} FAIL: found prompt/runtime-adjacent knowledge literals" >&2
  printf '%s\n' "$hits" >&2
  exit 1
fi

echo "${prefix} OK"
