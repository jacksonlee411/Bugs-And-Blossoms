#!/usr/bin/env bash
set -euo pipefail

prefix="[librechat-runtime-clean]"
repo_root="$(git rev-parse --show-toplevel)"
# shellcheck disable=SC1091
source "${repo_root}/scripts/librechat/common.sh"
librechat_init "${prefix}"

for service in "${LIBRECHAT_SERVICES[@]}"; do
  dir="$(librechat_service_data_dir "${service}")"
  if [[ -d "${dir}" ]]; then
    if ! rm -rf "${dir}" 2>/dev/null; then
      librechat_require_cmd docker
      docker run --rm -v "${dir}:/target" alpine:3.20 sh -lc 'rm -rf /target/* /target/.[!.]* /target/..?*' >/dev/null
      rm -rf "${dir}"
    fi
    echo "${prefix} removed ${dir}"
  fi
done

mkdir -p "${LIBRECHAT_DATA_ROOT_ABS}"
echo "${prefix} OK"
