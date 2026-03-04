#!/usr/bin/env bash

librechat_abspath() {
  local raw_path="${1:?}"
  local base_path="${2:?}"
  python3 - "$raw_path" "$base_path" <<'PY'
import os
import sys

raw = os.path.expanduser(sys.argv[1].strip())
base = sys.argv[2].strip()
if os.path.isabs(raw):
    print(os.path.abspath(raw))
else:
    print(os.path.abspath(os.path.join(base, raw)))
PY
}

librechat_service_data_subdir() {
  case "${1:?}" in
    api) printf '%s\n' "api" ;;
    mongodb) printf '%s\n' "mongodb" ;;
    meilisearch) printf '%s\n' "meilisearch" ;;
    rag_api) printf '%s\n' "rag_api" ;;
    vectordb) printf '%s\n' "vectordb" ;;
    *)
      return 1
      ;;
  esac
}

librechat_service_mount_target() {
  case "${1:?}" in
    api) printf '%s\n' "/app/data" ;;
    mongodb) printf '%s\n' "/data/db" ;;
    meilisearch) printf '%s\n' "/meili_data" ;;
    rag_api) printf '%s\n' "/app/data" ;;
    vectordb) printf '%s\n' "/qdrant/storage" ;;
    *)
      return 1
      ;;
  esac
}

librechat_service_data_dir() {
  local service="${1:?}"
  local subdir
  subdir="$(librechat_service_data_subdir "${service}")"
  printf '%s\n' "${LIBRECHAT_DATA_ROOT_ABS}/${subdir}"
}

librechat_init() {
  LIBRECHAT_PREFIX="${1:-[librechat-runtime]}"
  LIBRECHAT_REPO_ROOT="$(git rev-parse --show-toplevel)"
  LIBRECHAT_RUNTIME_DIR="${LIBRECHAT_REPO_ROOT}/deploy/librechat"
  LIBRECHAT_SERVICES=(api mongodb meilisearch rag_api vectordb)
  LIBRECHAT_ENV_FILE_PATH="${LIBRECHAT_ENV_FILE:-${LIBRECHAT_RUNTIME_DIR}/.env}"
  if [[ ! -f "${LIBRECHAT_ENV_FILE_PATH}" ]]; then
    LIBRECHAT_ENV_FILE_PATH="${LIBRECHAT_RUNTIME_DIR}/.env.example"
  fi
  if [[ ! -f "${LIBRECHAT_ENV_FILE_PATH}" ]]; then
    echo "${LIBRECHAT_PREFIX} missing env file: ${LIBRECHAT_ENV_FILE_PATH}" >&2
    return 2
  fi

  set -a
  # shellcheck disable=SC1090
  . "${LIBRECHAT_ENV_FILE_PATH}"
  set +a

  LIBRECHAT_COMPOSE_PROJECT="${LIBRECHAT_COMPOSE_PROJECT:-bugs-and-blossoms-librechat}"
  LIBRECHAT_DATA_ROOT_RAW="${LIBRECHAT_DATA_ROOT:-.local/librechat}"
  LIBRECHAT_DATA_ROOT_ABS="$(librechat_abspath "${LIBRECHAT_DATA_ROOT_RAW}" "${LIBRECHAT_REPO_ROOT}")"
  if [[ "${LIBRECHAT_DATA_ROOT_ABS}" != "${LIBRECHAT_REPO_ROOT}/"* ]]; then
    echo "${LIBRECHAT_PREFIX} LIBRECHAT_DATA_ROOT must stay under repo root: ${LIBRECHAT_DATA_ROOT_ABS}" >&2
    return 2
  fi
  export LIBRECHAT_DATA_ROOT="${LIBRECHAT_DATA_ROOT_ABS}"
  export LIBRECHAT_COMPOSE_PROJECT

  LIBRECHAT_COMPOSE_CMD=(
    docker compose
    -p "${LIBRECHAT_COMPOSE_PROJECT}"
    --env-file "${LIBRECHAT_ENV_FILE_PATH}"
    -f "${LIBRECHAT_RUNTIME_DIR}/docker-compose.upstream.yaml"
    -f "${LIBRECHAT_RUNTIME_DIR}/docker-compose.overlay.yaml"
  )
}

librechat_require_cmd() {
  local name="${1:?}"
  if ! command -v "${name}" >/dev/null 2>&1; then
    echo "${LIBRECHAT_PREFIX} missing ${name}" >&2
    return 2
  fi
}

librechat_ensure_data_dirs() {
  mkdir -p "${LIBRECHAT_DATA_ROOT_ABS}"
  for service in "${LIBRECHAT_SERVICES[@]}"; do
    local dir
    dir="$(librechat_service_data_dir "${service}")"
    mkdir -p "${dir}"
    if [[ ! -d "${dir}" || ! -w "${dir}" ]]; then
      echo "${LIBRECHAT_PREFIX} data dir unavailable: ${dir}" >&2
      return 1
    fi
  done
}

librechat_compose_config_json() {
  "${LIBRECHAT_COMPOSE_CMD[@]}" config --format json
}

librechat_assert_mount_sources() {
  local config_json
  if ! config_json="$(librechat_compose_config_json 2>/dev/null)"; then
    echo "${LIBRECHAT_PREFIX} docker compose config failed (env/compose invalid)" >&2
    return 2
  fi

  for service in "${LIBRECHAT_SERVICES[@]}"; do
    local target expected actual
    target="$(librechat_service_mount_target "${service}")"
    expected="$(librechat_service_data_dir "${service}")"
    actual="$(
      printf '%s' "${config_json}" |
        jq -r --arg service "${service}" --arg target "${target}" '
          .services[$service].volumes[]?
          | select(.type == "bind" and .target == $target)
          | .source
        ' |
        head -n 1
    )"
    if [[ -z "${actual}" || "${actual}" == "null" ]]; then
      echo "${LIBRECHAT_PREFIX} missing bind mount for ${service} (${target})" >&2
      return 1
    fi
    if [[ "${actual}" != "${expected}" ]]; then
      echo "${LIBRECHAT_PREFIX} mount source drift for ${service}: expected=${expected} actual=${actual}" >&2
      return 1
    fi
    if [[ ! -d "${actual}" ]]; then
      echo "${LIBRECHAT_PREFIX} mount source missing for ${service}: ${actual}" >&2
      return 1
    fi
  done
}

librechat_running_services() {
  "${LIBRECHAT_COMPOSE_CMD[@]}" ps --services --status running 2>/dev/null || true
}
