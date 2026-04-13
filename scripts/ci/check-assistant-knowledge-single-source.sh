#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

prefix="[assistant-knowledge-single-source]"
root="internal/server/assistant_knowledge_md"
required_dirs=(intent actions replies tools wiki)

if [[ ! -d "$root" ]]; then
  echo "${prefix} FAIL: missing ${root}" >&2
  exit 1
fi

for dir in "${required_dirs[@]}"; do
  if [[ ! -d "${root}/${dir}" ]]; then
    echo "${prefix} FAIL: missing ${root}/${dir}" >&2
    exit 1
  fi
done

extra_dirs="$(find "$root" -mindepth 1 -maxdepth 1 -type d | sed "s#^${root}/##" | grep -Ev '^(intent|actions|replies|tools|wiki)$' || true)"
if [[ -n "$extra_dirs" ]]; then
  echo "${prefix} FAIL: unexpected knowledge directory" >&2
  printf '%s\n' "$extra_dirs" >&2
  exit 1
fi

bad_files="$(find "$root" -type f ! -name '*.zh.md' ! -name '*.en.md' | sort || true)"
if [[ -n "$bad_files" ]]; then
  echo "${prefix} FAIL: only <id>.zh.md / <id>.en.md are allowed" >&2
  printf '%s\n' "$bad_files" >&2
  exit 1
fi

top_level_files="$(find "$root" -mindepth 1 -maxdepth 1 -type f | sort || true)"
if [[ -n "$top_level_files" ]]; then
  echo "${prefix} FAIL: top-level files are not allowed under ${root}" >&2
  printf '%s\n' "$top_level_files" >&2
  exit 1
fi

if find internal/server/assistant_knowledge -type f | grep -q . 2>/dev/null; then
  echo "${prefix} FAIL: retired JSON knowledge directory must stay empty" >&2
  find internal/server/assistant_knowledge -type f | sort >&2
  exit 1
fi

echo "${prefix} OK"
