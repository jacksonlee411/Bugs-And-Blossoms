#!/usr/bin/env bash
set -euo pipefail

# 规则：仓库根目录禁止新增 Markdown 文档（白名单：AGENTS.md）
root_mds="$(find . -maxdepth 1 -type f -iname '*.md' -print)"
if [[ -n "$root_mds" ]]; then
  bad="$(printf "%s\n" "$root_mds" | grep -vE '^./AGENTS\.md$' || true)"
  if [[ -n "$bad" ]]; then
    echo "[doc] root Markdown document is not allowed (except AGENTS.md):" >&2
    printf "%s\n" "$bad" >&2
    exit 1
  fi
fi

echo "[doc] OK"
