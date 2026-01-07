#!/usr/bin/env bash
set -euo pipefail

base_ref="${1:-}"
head_ref="${2:-}"

is_all_zero_sha() {
  local sha="${1:?}"
  [[ "$sha" =~ ^0+$ ]]
}

try_diff_names() {
  local base="${1:?}"
  local head="${2:?}"
  git diff --name-only "$base" "$head"
}

try_show_names() {
  local rev="${1:?}"
  git show --pretty=format: --name-only "$rev"
}

if [[ -n "${GITHUB_EVENT_PATH:-}" && -f "${GITHUB_EVENT_PATH:-}" && -n "${GITHUB_EVENT_NAME:-}" ]]; then
  case "${GITHUB_EVENT_NAME}" in
    pull_request|pull_request_target)
      base_sha="$(jq -r '.pull_request.base.sha' "$GITHUB_EVENT_PATH")"
      head_sha="$(jq -r '.pull_request.head.sha' "$GITHUB_EVENT_PATH")"
      ;;
    push)
      base_sha="$(jq -r '.before' "$GITHUB_EVENT_PATH")"
      head_sha="$(jq -r '.after' "$GITHUB_EVENT_PATH")"
      ;;
    *)
      base_sha=""
      head_sha=""
      ;;
  esac

  if [[ -n "${base_sha:-}" && -n "${head_sha:-}" && "${base_sha}" != "null" && "${head_sha}" != "null" ]]; then
    if is_all_zero_sha "${base_sha}"; then
      if try_show_names "${head_sha}"; then
        exit 0
      fi
    else
      if try_diff_names "${base_sha}" "${head_sha}"; then
        exit 0
      fi
    fi
  fi
fi

if [[ -n "$base_ref" && -n "$head_ref" ]]; then
  if try_diff_names "$base_ref" "$head_ref"; then
    exit 0
  fi
fi

if try_diff_names HEAD~1 HEAD; then
  exit 0
fi

try_show_names HEAD
