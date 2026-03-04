#!/usr/bin/env bash
set -euo pipefail

prefix="[librechat-runtime-up]"
repo_root="$(git rev-parse --show-toplevel)"
# shellcheck disable=SC1091
source "${repo_root}/scripts/librechat/common.sh"
librechat_init "${prefix}"

if [[ ! -f "${LIBRECHAT_RUNTIME_DIR}/versions.lock.yaml" ]]; then
  echo "${prefix} missing versions.lock.yaml" >&2
  exit 2
fi

librechat_require_cmd docker
librechat_require_cmd jq

librechat_ensure_data_dirs
librechat_assert_mount_sources

"${LIBRECHAT_COMPOSE_CMD[@]}" up -d
"${LIBRECHAT_RUNTIME_DIR}/healthcheck.sh"

echo "${prefix} OK"
