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
    rag_api) printf '%s\n' "/app/uploads" ;;
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
  LIBRECHAT_SERVICES=(api)
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

librechat_require_env_nonempty() {
  local name="${1:?}"
  local hint="${2:-}"
  local value="${!name:-}"
  if [[ -n "${value//[[:space:]]/}" ]]; then
    return 0
  fi
  echo "${LIBRECHAT_PREFIX} missing required env: ${name} (file=${LIBRECHAT_ENV_FILE_PATH})" >&2
  if [[ -n "${hint}" ]]; then
    echo "${LIBRECHAT_PREFIX} ${hint}" >&2
  fi
  return 2
}

librechat_env_trimmed() {
  local name="${1:?}"
  local value="${!name:-}"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s\n' "${value}"
}

librechat_has_retired_dependency_env() {
  local retired_vars=(
    MEILI_HOST
    MEILI_MASTER_KEY
    RAG_API_URL
    VECTOR_DB_PROVIDER
    QDRANT_URL
    RAG_API_VECTOR_DB_TYPE
    RAG_API_ATLAS_MONGO_DB_URI
    RAG_API_ATLAS_SEARCH_INDEX
    RAG_API_COLLECTION_NAME
  )
  local name
  for name in "${retired_vars[@]}"; do
    if [[ -n "$(librechat_env_trimmed "${name}")" ]]; then
      return 0
    fi
  done
  return 1
}

librechat_retired_dependency_env_report() {
  local retired_vars=(
    MEILI_HOST
    MEILI_MASTER_KEY
    RAG_API_URL
    VECTOR_DB_PROVIDER
    QDRANT_URL
    RAG_API_VECTOR_DB_TYPE
    RAG_API_ATLAS_MONGO_DB_URI
    RAG_API_ATLAS_SEARCH_INDEX
    RAG_API_COLLECTION_NAME
  )
  local first="1"
  local name value
  for name in "${retired_vars[@]}"; do
    value="$(librechat_env_trimmed "${name}")"
    if [[ -z "${value}" ]]; then
      continue
    fi
    if [[ "${first}" == "1" ]]; then
      first="0"
    else
      printf ', '
    fi
    printf '%s=%q' "${name}" "${value}"
  done
  printf '\n'
}

librechat_mongo_uri_targets_removed_service() {
  local mongo_uri
  mongo_uri="$(librechat_env_trimmed MONGO_URI)"
  if [[ -z "${mongo_uri}" ]]; then
    return 1
  fi
  [[ "${mongo_uri}" == *"://mongodb:"* || "${mongo_uri}" == *"://mongodb/"* || "${mongo_uri}" == *"@mongodb:"* || "${mongo_uri}" == *"@mongodb/"* ]]
}

librechat_require_container_env_nonempty() {
  local service="${1:?}"
  local name="${2:?}"
  local hint="${3:-}"
  if "${LIBRECHAT_COMPOSE_CMD[@]}" exec -T "${service}" sh -lc "test -n \"\${${name}:-}\"" >/dev/null 2>&1; then
    return 0
  fi
  echo "${LIBRECHAT_PREFIX} ${service} missing required env in container: ${name}" >&2
  if [[ -n "${hint}" ]]; then
    echo "${LIBRECHAT_PREFIX} ${hint}" >&2
  fi
  return 1
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
