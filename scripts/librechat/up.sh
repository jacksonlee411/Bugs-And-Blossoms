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
librechat_require_env_nonempty OPENAI_API_KEY "set OPENAI_API_KEY in deploy/librechat/.env (or export it) before runtime-up"

"${LIBRECHAT_COMPOSE_CMD[@]}" up -d
librechat_require_container_env_nonempty api OPENAI_API_KEY "container env drift detected; check deploy/librechat/.env and rerun make assistant-runtime-down && make assistant-runtime-up"
"${LIBRECHAT_RUNTIME_DIR}/healthcheck.sh"

echo "${prefix} OK"
