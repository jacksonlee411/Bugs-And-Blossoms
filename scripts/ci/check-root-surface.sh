#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

echo "[root-surface] scan root files and directories"

allowed_root_names=(
  ".env"
  ".env.example"
  ".env.local"
  ".env.local.example"
  ".git"
  ".github"
  ".gitignore"
  ".gocleanarch.yml"
  ".local"
  ".nvmrc"
  ".tool-versions"
  ".vscode"
  "AGENTS.md"
  "Makefile"
  "apps"
  "atlas.hcl"
  "bin"
  "cmd"
  "compose.dev.yml"
  "config"
  "coverage"
  "deploy"
  "designs"
  "docs"
  "e2e"
  "go.mod"
  "go.sum"
  "internal"
  "migrations"
  "modules"
  "pkg"
  "scripts"
  "sqlc.yaml"
  "third_party"
  "tools"
)

allowed_tracked_roots=(
  ".env.example"
  ".env.local.example"
  ".github"
  ".gitignore"
  ".gocleanarch.yml"
  ".nvmrc"
  ".tool-versions"
  "AGENTS.md"
  "Makefile"
  "apps"
  "atlas.hcl"
  "cmd"
  "compose.dev.yml"
  "config"
  "deploy"
  "designs"
  "docs"
  "e2e"
  "go.mod"
  "go.sum"
  "internal"
  "migrations"
  "modules"
  "pkg"
  "scripts"
  "sqlc.yaml"
  "third_party"
  "tools"
)

list_contains() {
  local name="${1:?}"
  shift
  local allowed
  for allowed in "$@"; do
    if [[ "$name" == "$allowed" ]]; then
      return 0
    fi
  done
  return 1
}

is_allowed_root_name() {
  local name="${1:?}"
  list_contains "$name" "${allowed_root_names[@]}"
}

is_allowed_tracked_root() {
  local name="${1:?}"
  list_contains "$name" "${allowed_tracked_roots[@]}"
}

has_bad_name_bytes() {
  local name="${1:?}"
  [[ "$name" == *$'\n'* || "$name" == *$'\r'* || "$name" == *$'\t'* ]]
}

failures=()

while IFS= read -r -d '' entry; do
  name="${entry#./}"
  if has_bad_name_bytes "$name"; then
    failures+=("root entry has newline/tab/control-like whitespace: $(printf '%q' "$name")")
    continue
  fi
  if ! is_allowed_root_name "$name"; then
    failures+=("unexpected root entry: ${name}")
  fi
done < <(find . -mindepth 1 -maxdepth 1 -printf '%p\0')

while IFS= read -r -d '' tracked; do
  name="${tracked#./}"
  root="${name%%/*}"
  if has_bad_name_bytes "$name"; then
    failures+=("tracked path has newline/tab/control-like whitespace: $(printf '%q' "$name")")
  fi
  if ! is_allowed_tracked_root "$root"; then
    failures+=("tracked path under unexpected root: ${name}")
  fi
done < <(git ls-files -z)

if ((${#failures[@]} > 0)); then
  echo "[root-surface] FAIL: root surface drift detected" >&2
  printf '  - %s\n' "${failures[@]}" >&2
  echo "[root-surface] Put generated/runtime artifacts under .local/, e2e/_artifacts/, coverage/, or the owning module directory." >&2
  exit 1
fi

echo "[root-surface] OK"
