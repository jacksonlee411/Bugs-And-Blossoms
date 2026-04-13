#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

prefix="[assistant-no-knowledge-db]"
pattern='database/sql|github\.com/jackc/pgx|gorm\.io|redis|pinecone|weaviate|milvus|qdrant|chroma|embedding|vector'

hits="$(rg -n -i -S "$pattern" \
  internal/server/assistant_knowledge_md \
  internal/server/assistant_knowledge_markdown_runtime.go \
  internal/server/assistant_knowledge_runtime.go || true)"

if [[ -n "$hits" ]]; then
  echo "${prefix} FAIL: knowledge/runtime layer must not depend on DB/vector/RAG stacks" >&2
  printf '%s\n' "$hits" >&2
  exit 1
fi

echo "${prefix} OK"
