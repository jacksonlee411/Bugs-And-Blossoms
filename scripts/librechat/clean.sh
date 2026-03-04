#!/usr/bin/env bash
set -euo pipefail

prefix="[librechat-runtime-clean]"
repo_root="$(git rev-parse --show-toplevel)"
# shellcheck disable=SC1091
source "${repo_root}/scripts/librechat/common.sh"
librechat_init "${prefix}"

remove_service_data_dir() {
  local dir="${1:?}"
  if rm -rf "${dir}" 2>/dev/null; then
    return 0
  fi

  librechat_require_cmd docker
  if ! docker run --rm -v "${dir}:/target" alpine:3.20 sh -lc 'rm -rf /target/* /target/.[!.]* /target/..?*' >/dev/null; then
    echo "${prefix} failed to clean ${dir} via docker helper" >&2
    echo "${prefix} fix ownership and retry: sudo chown -R \"$(id -u):$(id -g)\" \"${dir}\"" >&2
    return 1
  fi
  if ! rm -rf "${dir}" 2>/dev/null; then
    echo "${prefix} failed to remove ${dir} after docker helper cleanup" >&2
    echo "${prefix} fix ownership and retry: sudo chown -R \"$(id -u):$(id -g)\" \"${dir}\"" >&2
    return 1
  fi
  return 0
}

for service in "${LIBRECHAT_SERVICES[@]}"; do
  dir="$(librechat_service_data_dir "${service}")"
  if [[ -d "${dir}" ]]; then
    remove_service_data_dir "${dir}"
    echo "${prefix} removed ${dir}"
  fi
done

mkdir -p "${LIBRECHAT_DATA_ROOT_ABS}"
echo "${prefix} OK"
