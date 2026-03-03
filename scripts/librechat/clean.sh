#!/usr/bin/env bash
set -euo pipefail

prefix="[librechat-runtime-clean]"
repo_root="$(git rev-parse --show-toplevel)"

allowed_dirs=(
  "${repo_root}/.local/librechat/api"
  "${repo_root}/.local/librechat/mongodb"
  "${repo_root}/.local/librechat/meilisearch"
  "${repo_root}/.local/librechat/rag_api"
  "${repo_root}/.local/librechat/vectordb"
)

for dir in "${allowed_dirs[@]}"; do
  if [[ -d "${dir}" ]]; then
    rm -rf "${dir}"
    echo "${prefix} removed ${dir}"
  fi
done

echo "${prefix} OK"
