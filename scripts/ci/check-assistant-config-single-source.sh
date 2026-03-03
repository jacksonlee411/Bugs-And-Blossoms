#!/usr/bin/env bash
set -euo pipefail

prefix="[assistant-config-single-source]"

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

allowlist_file="config/assistant/single-source-gate-allowlist.yaml"
today_utc="$(date -u +%F)"

declare -a allow_rules=()
declare -a allow_paths=()
declare -a allow_expires=()
declare -a violations=()

trim() {
  local value="${1:-}"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "$value"
}

strip_yaml_scalar() {
  local value
  value="$(trim "${1:-}")"
  if [[ "${value:0:1}" == '"' && "${value: -1}" == '"' ]]; then
    value="${value:1:${#value}-2}"
  elif [[ "${value:0:1}" == "'" && "${value: -1}" == "'" ]]; then
    value="${value:1:${#value}-2}"
  fi
  printf '%s' "$value"
}

add_violation() {
  local rule="${1:?}"
  local file="${2:?}"
  local error_code="${3:?}"
  local reason="${4:?}"
  violations+=("${rule}|${file}|${error_code}|${reason}")
}

flush_allowlist_entry() {
  local rule="${1:-}"
  local path="${2:-}"
  local expires_at="${3:-}"
  if [[ -z "$rule" && -z "$path" && -z "$expires_at" ]]; then
    return
  fi
  if [[ -z "$rule" || -z "$path" || -z "$expires_at" ]]; then
    echo "${prefix} FAIL R0 ${allowlist_file}: invalid allowlist entry (rule/path/expires_at required)" >&2
    exit 2
  fi
  if [[ ! "$expires_at" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]]; then
    echo "${prefix} FAIL R0 ${allowlist_file}: invalid expires_at format for ${rule}:${path} (${expires_at})" >&2
    exit 2
  fi
  allow_rules+=("$rule")
  allow_paths+=("$path")
  allow_expires+=("$expires_at")
}

load_allowlist() {
  if [[ ! -f "$allowlist_file" ]]; then
    return
  fi

  local current_rule=""
  local current_path=""
  local current_expires=""
  local line=""
  local trimmed=""
  local key=""
  local value=""

  while IFS= read -r line || [[ -n "$line" ]]; do
    trimmed="$(trim "$line")"
    [[ -z "$trimmed" || "${trimmed:0:1}" == "#" ]] && continue

    if [[ "$trimmed" == "- "* ]]; then
      flush_allowlist_entry "$current_rule" "$current_path" "$current_expires"
      current_rule=""
      current_path=""
      current_expires=""
      trimmed="$(trim "${trimmed#- }")"
      [[ -z "$trimmed" ]] && continue
    fi

    if [[ "$trimmed" != *:* ]]; then
      continue
    fi

    key="$(trim "${trimmed%%:*}")"
    value="$(strip_yaml_scalar "${trimmed#*:}")"
    case "$key" in
      rule)
        current_rule="$value"
        ;;
      path)
        current_path="$value"
        ;;
      expires_at)
        current_expires="$value"
        ;;
    esac
  done <"$allowlist_file"

  flush_allowlist_entry "$current_rule" "$current_path" "$current_expires"
}

allowlist_decision() {
  local rule="${1:?}"
  local file="${2:?}"
  local idx=""
  for idx in "${!allow_rules[@]}"; do
    if [[ "${allow_rules[$idx]}" != "$rule" || "${allow_paths[$idx]}" != "$file" ]]; then
      continue
    fi
    local expires_at="${allow_expires[$idx]}"
    if [[ "$expires_at" < "$today_utc" ]]; then
      printf 'expired:%s' "$expires_at"
      return
    fi
    printf 'allow:%s' "$expires_at"
    return
  done
  printf 'none'
}

validate_allowlist_expiry() {
  local idx=""
  for idx in "${!allow_rules[@]}"; do
    if [[ "${allow_expires[$idx]}" < "$today_utc" ]]; then
      add_violation "R0" "$allowlist_file" "assistant_config_allowlist_expired" \
        "allowlist entry expired: rule=${allow_rules[$idx]} path=${allow_paths[$idx]} expires_at=${allow_expires[$idx]}"
    fi
  done
}

resolve_diff_mode() {
  if [[ -n "${GITHUB_EVENT_PATH:-}" && -f "${GITHUB_EVENT_PATH:-}" && -n "${GITHUB_EVENT_NAME:-}" ]]; then
    local base_sha=""
    local head_sha=""
    case "${GITHUB_EVENT_NAME}" in
      pull_request|pull_request_target)
        base_sha="$(jq -r '.pull_request.base.sha' "$GITHUB_EVENT_PATH")"
        head_sha="$(jq -r '.pull_request.head.sha' "$GITHUB_EVENT_PATH")"
        ;;
      push)
        base_sha="$(jq -r '.before' "$GITHUB_EVENT_PATH")"
        head_sha="$(jq -r '.after' "$GITHUB_EVENT_PATH")"
        ;;
    esac
    if [[ -n "$base_sha" && -n "$head_sha" && "$base_sha" != "null" && "$head_sha" != "null" ]]; then
      if [[ "$base_sha" =~ ^0+$ ]]; then
        printf 'show %s\n' "$head_sha"
        return
      fi
      printf 'range %s %s\n' "$base_sha" "$head_sha"
      return
    fi
  fi

  if ! git diff --quiet HEAD --; then
    printf 'worktree\n'
    return
  fi

  if git rev-parse --verify HEAD~1 >/dev/null 2>&1; then
    printf 'range %s %s\n' 'HEAD~1' 'HEAD'
    return
  fi

  printf 'show %s\n' 'HEAD'
}

collect_patch_for_file() {
  local mode="${1:?}"
  local ref_a="${2:-}"
  local ref_b="${3:-}"
  local file="${4:?}"
  case "$mode" in
    worktree)
      git diff --unified=0 --no-color HEAD -- "$file"
      ;;
    range)
      git diff --unified=0 --no-color "$ref_a" "$ref_b" -- "$file"
      ;;
    show)
      git show --unified=0 --no-color "$ref_a" -- "$file"
      ;;
    *)
      return 2
      ;;
  esac
}

is_runtime_file() {
  local file="${1:?}"
  [[ "$file" =~ ^docs/ ]] && return 1
  [[ "$file" =~ ^internal/server/assets/web/ ]] && return 1
  [[ "$file" =~ ^scripts/ci/ ]] && return 1
  [[ "$file" =~ ^\.github/ ]] && return 1
  [[ "$file" =~ (_test\.go|\.test\.(ts|tsx|js|jsx)$) ]] && return 1

  [[ "$file" =~ ^internal/.*\.go$ ]] ||
    [[ "$file" =~ ^modules/.*\.(go|sql)$ ]] ||
    [[ "$file" =~ ^cmd/.*\.go$ ]] ||
    [[ "$file" =~ ^apps/web/src/.*\.(ts|tsx|js|jsx)$ ]] ||
    [[ "$file" == "config/routing/allowlist.yaml" ]] ||
    [[ "$file" == "config/capability/route-capability-map.v1.json" ]]
}

is_config_layer_file() {
  local file="${1:?}"
  [[ "$file" =~ ^internal/server/assistant_model_gateway.*\.go$ ]] ||
    [[ "$file" =~ ^internal/server/assistant_model_providers.*\.go$ ]]
}

load_allowlist
validate_allowlist_expiry

if [[ ! -f "Makefile" ]]; then
  add_violation "R3" "Makefile" "assistant_config_ssot_drift_detected" "Makefile is missing"
else
  grep -q '^assistant-config-single-source:' Makefile ||
    add_violation "R3" "Makefile" "assistant_config_ssot_drift_detected" "missing assistant-config-single-source target"
  grep -q 'check-assistant-config-single-source\.sh' Makefile ||
    add_violation "R3" "Makefile" "assistant_config_ssot_drift_detected" "missing check-assistant-config-single-source.sh wiring"
  grep -q '\$(MAKE) check assistant-config-single-source' Makefile ||
    add_violation "R3" "Makefile" "assistant_config_ssot_drift_detected" "missing preflight hook for assistant-config-single-source"
fi

if [[ ! -f ".github/workflows/quality-gates.yml" ]]; then
  add_violation "R3" ".github/workflows/quality-gates.yml" "assistant_config_ssot_drift_detected" "quality-gates workflow is missing"
else
  grep -q 'make check assistant-config-single-source' .github/workflows/quality-gates.yml ||
    add_violation "R3" ".github/workflows/quality-gates.yml" "assistant_config_ssot_drift_detected" "missing CI step for assistant-config-single-source gate"
fi

docs_with_gate=0
for doc in \
  docs/dev-plans/012-ci-quality-gates.md \
  docs/dev-plans/230-librechat-project-level-integration-plan.md \
  docs/dev-plans/231-librechat-prerequisites-contract-and-gates-plan.md; do
  if [[ -f "$doc" ]] && grep -q 'assistant-config-single-source' "$doc"; then
    docs_with_gate=$((docs_with_gate + 1))
  fi
done
if ((docs_with_gate < 2)); then
  add_violation "R3" "docs/dev-plans" "assistant_config_ssot_drift_detected" \
    "assistant-config-single-source must be documented in at least two plan docs (012/230/231)"
fi

resolve_mode_value=""
ref_a=""
ref_b=""
read -r resolve_mode_value ref_a ref_b < <(resolve_diff_mode)

mapfile -t changed_files < <(./scripts/ci/changed-files.sh | awk 'NF' | sort -u)
for file in "${changed_files[@]}"; do
  [[ -z "$file" || ! -f "$file" ]] && continue
  if ! is_runtime_file "$file"; then
    continue
  fi

  patch="$(collect_patch_for_file "$resolve_mode_value" "$ref_a" "$ref_b" "$file" || true)"
  added_lines="$(printf '%s\n' "$patch" | grep -E '^\+[^+]' || true)"
  [[ -z "$added_lines" ]] && continue

  r1_hits="$(printf '%s\n' "$added_lines" | grep -nE '/internal/assistant/model-providers:apply|handleAssistantModelProvidersApply|applyAssistantModelProviders\(' || true)"
  if [[ -n "$r1_hits" ]]; then
    decision="$(allowlist_decision "R1" "$file")"
    case "$decision" in
      allow:*)
        echo "${prefix} allowlist R1 ${file} (${decision#allow:})"
        ;;
      expired:*)
        add_violation "R1" "$file" "assistant_config_allowlist_expired" \
          "allowlist expired for R1 at ${decision#expired:}"
        ;;
      *)
        add_violation "R1" "$file" "assistant_config_secondary_write_path_detected" \
          "secondary write path marker detected in added lines"
        ;;
    esac
  fi

  if is_config_layer_file "$file"; then
    r2_hits="$(printf '%s\n' "$added_lines" | grep -nE 'intent_hash|plan_hash|context_hash|contract_snapshot' || true)"
    if [[ -n "$r2_hits" ]]; then
      decision="$(allowlist_decision "R2" "$file")"
      case "$decision" in
        allow:*)
          echo "${prefix} allowlist R2 ${file} (${decision#allow:})"
          ;;
        expired:*)
          add_violation "R2" "$file" "assistant_config_allowlist_expired" \
            "allowlist expired for R2 at ${decision#expired:}"
          ;;
        *)
          add_violation "R2" "$file" "assistant_config_deterministic_artifact_backwrite_detected" \
            "deterministic artifact token detected in config-layer added lines"
          ;;
      esac
    fi
  fi
done

if ((${#violations[@]} > 0)); then
  printf '%s FAIL: %d violation(s)\n' "$prefix" "${#violations[@]}" >&2
  for entry in "${violations[@]}"; do
    IFS='|' read -r rule file error_code reason <<<"$entry"
    printf '%s FAIL %s %s: [%s] %s\n' "$prefix" "$rule" "$file" "$error_code" "$reason" >&2
  done
  exit 1
fi

echo "${prefix} OK"
