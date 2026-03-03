#!/usr/bin/env bash
set -euo pipefail

prefix="[librechat-runtime-up]"
repo_root="$(git rev-parse --show-toplevel)"
runtime_dir="${repo_root}/deploy/librechat"
compose_project="${LIBRECHAT_COMPOSE_PROJECT:-bugs-and-blossoms-librechat}"
env_file="${LIBRECHAT_ENV_FILE:-${runtime_dir}/.env}"
if [[ ! -f "${env_file}" ]]; then
  env_file="${runtime_dir}/.env.example"
fi

if [[ ! -f "${runtime_dir}/versions.lock.yaml" ]]; then
  echo "${prefix} missing versions.lock.yaml" >&2
  exit 2
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "${prefix} missing docker" >&2
  exit 2
fi

compose_cmd=(docker compose -p "${compose_project}" --env-file "${env_file}" -f "${runtime_dir}/docker-compose.upstream.yaml" -f "${runtime_dir}/docker-compose.overlay.yaml")

"${compose_cmd[@]}" up -d
"${runtime_dir}/healthcheck.sh"

echo "${prefix} OK"
