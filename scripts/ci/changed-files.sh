#!/usr/bin/env bash
set -euo pipefail

base_ref="${1:-}"
head_ref="${2:-}"

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
    git diff --name-only "${base_sha}" "${head_sha}"
    exit 0
  fi
fi

if [[ -n "$base_ref" && -n "$head_ref" ]]; then
  git diff --name-only "$base_ref" "$head_ref"
  exit 0
fi

git diff --name-only HEAD~1 HEAD

