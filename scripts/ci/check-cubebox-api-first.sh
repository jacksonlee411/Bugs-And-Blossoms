#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

prefix="[cubebox-api-first]"
pattern='READ_PLAN|ReadPlan|ReadAPICatalog|ExecutionRegistry(\.ExecutePlan)?|executor_key|source_executor_key'

targets=(
  internal/server
  modules/cubebox
  modules/orgunit/presentation/cubebox
)

ignore_globs=(
  --glob '!**/*_templ.go'
  --glob '!internal/server/assets/**'
)

is_allowed_hit() {
  local path="$1"
  local line="$2"

  case "$path" in
    internal/server/cubebox_query_flow.go)
      [[ "$line" == *"queryNarrationForbiddenPatterns"* ]] && return 0
      [[ "$line" == *'api_key|executor_key|result_focus|payload|results'* ]] && return 0
      [[ "$line" == *"不得暴露实现细节"* ]] && return 0
      [[ "$line" == *"不得提到内部术语"* ]] && return 0
      [[ "$line" == *"forbidden := range []string"* ]] && return 0
      ;;
    internal/server/cubebox_query_flow_test.go)
      [[ "$line" == *'"executor_key"'* ]] && return 0
      ;;
    modules/cubebox/planner_outcome_test.go)
      [[ "$line" == *"TestDecodePlannerOutcomeRejectsLegacyReadPlanShapes"* ]] && return 0
      [[ "$line" == *"executor_key"* ]] && return 0
      [[ "$line" == *"READ_PLAN"* ]] && return 0
      ;;
    modules/cubebox/knowledge_pack_test.go)
      [[ "$line" == *"TestValidateKnowledgePackRejectsLegacyExampleShape"* ]] && return 0
      [[ "$line" == *"executor_key"* ]] && return 0
      ;;
    modules/cubebox/api_execution_test.go)
      [[ "$line" == *"forbidden"* && "$line" == *"executor_key"* ]] && return 0
      ;;
  esac

  return 1
}

echo "${prefix} scan: active CubeBox API-first runtime, knowledge packs, and tests"

hits="$(rg -n -S "${ignore_globs[@]}" "$pattern" "${targets[@]}" || true)"
violations=()

while IFS= read -r hit; do
  [[ -z "$hit" ]] && continue
  path="${hit%%:*}"
  rest="${hit#*:}"
  line_no="${rest%%:*}"
  line="${rest#*:}"
  if is_allowed_hit "$path" "$line"; then
    continue
  fi
  violations+=("$path:$line_no:$line")
done <<< "$hits"

if (( ${#violations[@]} > 0 )); then
  echo "${prefix} FAIL: found legacy CubeBox execution contract residue" >&2
  printf '%s\n' "${violations[@]}" >&2
  exit 1
fi

echo "${prefix} OK"
