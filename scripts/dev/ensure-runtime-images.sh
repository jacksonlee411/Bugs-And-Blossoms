#!/usr/bin/env bash
set -euo pipefail

infra_env_file="${DEV_INFRA_ENV_FILE:-.env.example}"
if [[ -f "$infra_env_file" ]]; then
  set -a
  # shellcheck disable=SC1090
  . "$infra_env_file"
  set +a
fi

mirror_prefix="${DEV_RUNTIME_IMAGE_MIRROR_PREFIX:-docker.m.daocloud.io/library}"
pull_timeout_seconds="${DEV_RUNTIME_IMAGE_PULL_TIMEOUT_SECONDS:-90}"
postgres_image="${POSTGRES_IMAGE:-postgres:17}"
redis_image="${REDIS_IMAGE:-redis:latest}"

log() {
  echo "[dev-images] $*"
}

run_pull() {
  local image="${1:?}"
  if command -v timeout >/dev/null 2>&1; then
    timeout "${pull_timeout_seconds}s" docker pull "$image" >/dev/null
    return
  fi
  docker pull "$image" >/dev/null
}

mirror_candidate_for() {
  local image="${1:?}"
  case "$image" in
    postgres:17)
      echo "${mirror_prefix}/postgres:17"
      ;;
    redis:latest)
      echo "${mirror_prefix}/redis:latest"
      ;;
    *)
      return 1
      ;;
  esac
}

pull_candidate() {
  local target_image="${1:?}"
  local source_image="${2:?}"

  log "pull: $source_image"
  if ! run_pull "$source_image"; then
    return 1
  fi

  if [[ "$source_image" != "$target_image" ]]; then
    docker tag "$source_image" "$target_image"
  fi
  log "ready: $target_image"
}

ensure_image() {
  local image="${1:?}"
  local mirror_image=""
  local -a candidates=()

  if docker image inspect "$image" >/dev/null 2>&1; then
    log "cached: $image"
    return 0
  fi

  if mirror_image="$(mirror_candidate_for "$image")"; then
    if [[ "${CI:-}" == "true" || "${GITHUB_ACTIONS:-}" == "true" ]]; then
      candidates=("$image" "$mirror_image")
    else
      candidates=("$mirror_image" "$image")
    fi
  else
    candidates=("$image")
  fi

  for candidate in "${candidates[@]}"; do
    if pull_candidate "$image" "$candidate"; then
      return 0
    fi
    log "pull failed: $candidate"
  done

  echo "[dev-images] failed to prepare image: $image" >&2
  exit 1
}

if [[ "$#" -eq 0 ]]; then
  set -- "$postgres_image" "$redis_image"
fi

for image in "$@"; do
  ensure_image "$image"
done
