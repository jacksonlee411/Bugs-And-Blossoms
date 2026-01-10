#!/usr/bin/env bash
set -euo pipefail

allowed_pattern='^(wt-dev-main|wt-dev-a|wt-dev-b)$'

fail() {
  local got="${1:-}"
  echo "[pr-branch] FAIL: PR 源分支必须为固定 worktree 分支：wt-dev-main / wt-dev-a / wt-dev-b；当前为：${got}" >&2
  echo "[pr-branch] 处理方式：请在对应固定 worktree 分支提交并 push，然后从该分支发起 PR（禁止临时分支）" >&2
  exit 1
}

ok_skip() {
  echo "[pr-branch] OK (skip): ${1:?}"
  exit 0
}

if [[ "${GITHUB_ACTIONS:-}" == "true" ]]; then
  case "${GITHUB_EVENT_NAME:-}" in
    pull_request|pull_request_target)
      head_ref="${GITHUB_HEAD_REF:-}"
      if [[ -z "$head_ref" ]]; then
        echo "[pr-branch] FAIL: pull_request 事件缺少 GITHUB_HEAD_REF" >&2
        exit 1
      fi
      if [[ "$head_ref" =~ $allowed_pattern ]]; then
        echo "[pr-branch] OK: ${head_ref}"
        exit 0
      fi
      fail "$head_ref"
      ;;
    *)
      ok_skip "非 pull_request 事件（GITHUB_EVENT_NAME=${GITHUB_EVENT_NAME:-<empty>}）"
      ;;
  esac
fi

if ! command -v git >/dev/null 2>&1; then
  ok_skip "git 未安装"
fi

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
if [[ -z "$repo_root" ]]; then
  ok_skip "不在 git 仓库内"
fi
cd "$repo_root"

branch="$(git rev-parse --abbrev-ref HEAD)"
if [[ "$branch" == "HEAD" ]]; then
  ok_skip "detached HEAD"
fi

if [[ "$branch" =~ $allowed_pattern ]]; then
  echo "[pr-branch] OK: ${branch}"
  exit 0
fi

fail "$branch"
