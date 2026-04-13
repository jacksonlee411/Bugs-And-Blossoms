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
librechat_require_env_nonempty MONGO_URI "current LibreChat upstream backend still hard-requires Mongo; point MONGO_URI at an externally managed Mongo instance before runtime-up"

if librechat_has_retired_dependency_env; then
  echo "${prefix} retired dependency env drift detected in ${LIBRECHAT_ENV_FILE_PATH}" >&2
  echo "${prefix} default runtime baseline no longer allows MEILI/RAG/QDRANT env wiring: $(librechat_retired_dependency_env_report)" >&2
  echo "${prefix} clear the retired env vars before rerunning make assistant-runtime-up" >&2
  exit 2
fi

if librechat_mongo_uri_targets_removed_service; then
  echo "${prefix} MONGO_URI still points at removed compose service 'mongodb': $(librechat_env_trimmed MONGO_URI)" >&2
  echo "${prefix} current upstream backend hard-requires Mongo, but default compose no longer provisions mongodb; use an externally managed Mongo endpoint or intentionally restore legacy deps outside the default baseline" >&2
  exit 2
fi

"${LIBRECHAT_COMPOSE_CMD[@]}" up -d
librechat_require_container_env_nonempty api OPENAI_API_KEY "container env drift detected; check deploy/librechat/.env and rerun make assistant-runtime-down && make assistant-runtime-up"
"${LIBRECHAT_RUNTIME_DIR}/healthcheck.sh"

echo "${prefix} OK"
