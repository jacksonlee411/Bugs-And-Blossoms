#!/usr/bin/env bash
set -euo pipefail

prefix="[librechat-runtime-down]"
repo_root="$(git rev-parse --show-toplevel)"
# shellcheck disable=SC1091
source "${repo_root}/scripts/librechat/common.sh"
librechat_init "${prefix}"

librechat_require_cmd docker

"${LIBRECHAT_COMPOSE_CMD[@]}" down

echo "${prefix} OK"
