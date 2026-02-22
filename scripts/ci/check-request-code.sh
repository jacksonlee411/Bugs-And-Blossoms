#!/usr/bin/env bash
set -euo pipefail

business_token_pattern='(^|[^A-Za-z0-9_])(request_code|RequestCode|requestCode)([^A-Za-z0-9_]|$)'
tracing_token_pattern='(^|[^A-Za-z0-9_])(request_id|RequestID|requestId|X-Request-ID)([^A-Za-z0-9_]|$)'
mode="incremental"
dry_run=0

usage() {
  cat <<'USAGE'
Usage: scripts/ci/check-request-code.sh [--full|--incremental] [--dry-run]

Options:
  --full         全量扫描（Gate-B）
  --incremental  仅扫描增量新增行（Gate-A 兼容模式）
  --dry-run      只打印违规，不阻断（退出 0）
  -h, --help     显示帮助
USAGE
}

while (($# > 0)); do
  case "${1}" in
    --full)
      mode="full"
      ;;
    --incremental)
      mode="incremental"
      ;;
    --dry-run)
      dry_run=1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "[request-code] unknown argument: ${1}" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

is_all_zero_sha() {
  local sha="${1:?}"
  [[ "$sha" =~ ^0+$ ]]
}

resolve_mode() {
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
      if is_all_zero_sha "$base_sha"; then
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

is_scoped_file() {
  local file="${1:?}"
  [[ "$file" =~ ^internal/.*\.go$ ]] ||
    [[ "$file" =~ ^modules/.*\.(go|sql)$ ]] ||
    [[ "$file" =~ ^apps/web/src/.*\.(ts|tsx)$ ]] ||
    [[ "$file" =~ ^cmd/.*\.go$ ]] ||
    [[ "$file" =~ ^e2e/tests/.*\.js$ ]]
}

is_tracing_file() {
  local file="${1:?}"
  [[ "$file" == "internal/routing/responder.go" ]] ||
    [[ "$file" == "modules/staffing/presentation/controllers/assignments_api.go" ]] ||
    [[ "$file" == "apps/web/src/api/errors.ts" ]] ||
    [[ "$file" == "apps/web/src/api/httpClient.ts" ]] ||
    [[ "$file" == "apps/web/src/api/httpClient.test.ts" ]]
}

is_excluded_file() {
  local file="${1:?}"
  [[ "$file" =~ ^internal/server/assets/web/ ]]
}

collect_changed_files() {
  local mode="${1:?}"
  local ref_a="${2:-}"
  local ref_b="${3:-}"
  case "$mode" in
    worktree)
      git diff --name-only HEAD --
      ;;
    range)
      git diff --name-only "$ref_a" "$ref_b"
      ;;
    show)
      git show --pretty=format: --name-only "$ref_a"
      ;;
    *)
      return 2
      ;;
  esac
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

collect_full_scan_files() {
  {
    find internal -type f -name '*.go' ! -path 'internal/server/assets/*'
    find modules -type f \( -name '*.go' -o -name '*.sql' \)
    find apps/web/src -type f \( -name '*.ts' -o -name '*.tsx' \)
    find cmd -type f -name '*.go'
    find e2e/tests -type f -name '*.js'
  } | sort -u
}

scan_incremental() {
  local resolve_mode_value=""
  local ref_a=""
  local ref_b=""
  read -r resolve_mode_value ref_a ref_b < <(resolve_mode)

  mapfile -t changed_files < <(collect_changed_files "$resolve_mode_value" "$ref_a" "$ref_b" | awk 'NF' | sort -u)
  if [[ ${#changed_files[@]} -eq 0 ]]; then
    echo "[request-code] no changed files; skip"
    return 0
  fi

  local violations=0
  for file in "${changed_files[@]}"; do
    if ! is_scoped_file "$file" && ! is_tracing_file "$file"; then
      continue
    fi
    if is_excluded_file "$file"; then
      continue
    fi

    patch="$(collect_patch_for_file "$resolve_mode_value" "$ref_a" "$ref_b" "$file" || true)"
    added_lines="$(printf '%s\n' "$patch" | grep -E '^\+[^+]' || true)"

    if is_scoped_file "$file"; then
      bad_business_lines="$(printf '%s\n' "$added_lines" | grep -E "$business_token_pattern" || true)"
      if [[ -n "$bad_business_lines" ]]; then
        violations=1
        echo "[request-code] forbidden business token detected in added lines: $file" >&2
        printf '%s\n' "$bad_business_lines" >&2
      fi
    fi

    if is_tracing_file "$file"; then
      bad_tracing_lines="$(printf '%s\n' "$added_lines" | grep -E "$tracing_token_pattern" || true)"
      if [[ -n "$bad_tracing_lines" ]]; then
        violations=1
        echo "[request-code] forbidden tracing token detected in added lines: $file" >&2
        printf '%s\n' "$bad_tracing_lines" >&2
      fi
    fi
  done

  if [[ $violations -ne 0 ]]; then
    if [[ $dry_run -eq 1 ]]; then
      echo "[request-code] dry-run mode: violations detected (non-blocking)" >&2
      return 0
    fi
    echo "[request-code] fail: business idempotency must use request_id; tracing must use trace_id + traceparent (no request_id/X-Request-ID in tracing files)" >&2
    return 1
  fi

  if [[ $dry_run -eq 1 ]]; then
    echo "[request-code] dry-run mode: clean (incremental)"
    return 0
  fi
  echo "[request-code] OK (incremental)"
}

scan_full() {
  local violations=0
  local file=""
  while IFS= read -r file; do
    if ! is_scoped_file "$file" && ! is_tracing_file "$file"; then
      continue
    fi
    if is_excluded_file "$file"; then
      continue
    fi

    if is_scoped_file "$file"; then
      bad_business_lines="$(grep -nE "$business_token_pattern" "$file" || true)"
      if [[ -n "$bad_business_lines" ]]; then
        violations=1
        echo "[request-code] forbidden business token detected: $file" >&2
        printf '%s\n' "$bad_business_lines" >&2
      fi
    fi

    if is_tracing_file "$file"; then
      bad_tracing_lines="$(grep -nE "$tracing_token_pattern" "$file" || true)"
      if [[ -n "$bad_tracing_lines" ]]; then
        violations=1
        echo "[request-code] forbidden tracing token detected: $file" >&2
        printf '%s\n' "$bad_tracing_lines" >&2
      fi
    fi
  done < <(collect_full_scan_files)

  if [[ $violations -ne 0 ]]; then
    if [[ $dry_run -eq 1 ]]; then
      echo "[request-code] dry-run mode: violations detected (non-blocking)" >&2
      return 0
    fi
    echo "[request-code] fail: business idempotency must use request_id; tracing must use trace_id + traceparent (no request_id/X-Request-ID in tracing files)" >&2
    return 1
  fi

  if [[ $dry_run -eq 1 ]]; then
    echo "[request-code] dry-run mode: clean"
    return 0
  fi
  echo "[request-code] OK (full)"
}

if [[ "$mode" == "incremental" ]]; then
  scan_incremental
  exit 0
fi

scan_full
