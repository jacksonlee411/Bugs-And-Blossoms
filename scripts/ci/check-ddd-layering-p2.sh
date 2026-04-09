#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

echo "[ddd-layering-p2] scan: require composition root when module layering expands"

mapfile -t changed_files < <(git diff --name-only --diff-filter=ACMR)

if [[ ${#changed_files[@]} -eq 0 ]]; then
  echo "[ddd-layering-p2] no changed files; skip"
  exit 0
fi

is_package_only_file() {
  local file="${1:?}"

  if [[ ! -f "$file" ]]; then
    return 0
  fi

  mapfile -t meaningful_lines < <(grep -Ev '^[[:space:]]*(//.*)?$' "$file" || true)

  if [[ ${#meaningful_lines[@]} -ne 1 ]]; then
    return 1
  fi

  [[ "${meaningful_lines[0]}" =~ ^package[[:space:]]+[A-Za-z0-9_]+$ ]]
}

violations=()
checked_roots=()

record_violation_once() {
  local key="${1:?}"
  local message="${2:?}"
  local seen

  for seen in "${checked_roots[@]}"; do
    if [[ "$seen" == "$key" ]]; then
      return 0
    fi
  done

  checked_roots+=("$key")
  violations+=("$message")
}

for file in "${changed_files[@]}"; do
  [[ -f "$file" ]] || continue

  case "$file" in
    *_test.go)
      continue
      ;;
  esac

  if [[ "$file" =~ ^modules/([^/]+)/(domain|services|infrastructure|presentation)/.+\.go$ ]]; then
    module_name="${BASH_REMATCH[1]}"
    layer_name="${BASH_REMATCH[2]}"

    root_file="modules/${module_name}/module.go"
    root_label="module.go"
    root_purpose="默认装配"

    if [[ "$layer_name" == "presentation" ]]; then
      root_file="modules/${module_name}/links.go"
      root_label="links.go"
      root_purpose="路由挂载"
    fi

    if [[ ! -f "$root_file" ]]; then
      record_violation_once \
        "$root_file" \
        "$file: module layering changed but missing composition root ${root_file}"
      continue
    fi

    if is_package_only_file "$root_file"; then
      record_violation_once \
        "$root_file" \
        "$file: module layering changed but ${root_file} is still package-only; expand ${root_label} to carry ${root_purpose} intent"
    fi
  fi
done

if [[ ${#violations[@]} -gt 0 ]]; then
  echo "[ddd-layering-p2] FAIL: found module changes without effective composition root" >&2
  printf '%s\n' "${violations[@]}" >&2
  exit 1
fi

echo "[ddd-layering-p2] OK"
