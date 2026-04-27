#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

build_ui=0
setup_cubebox=1
verify_cubebox=0
with_superadmin=0

usage() {
  cat <<'EOF' >&2
usage:
  start_dev_runtime.sh [--build-ui] [--skip-cubebox-model] [--verify-cubebox] [--with-superadmin]

starts:
  - docker infra via make dev-up
  - iam/orgunit migrations
  - authz policy pack
  - kratos stub
  - main Go server
  - optional superadmin
  - default KratosStub identities
  - optional CubeBox model settings baseline

env:
  DEV_INFRA_ENV_FILE       default: .env.example
  DEV_SERVER_ENV_FILE      default: .env.local, env.local, .env, .env.example
  DEV_SERVER_BASE_URL      default: http://localhost:8080
  DEV_SUPERADMIN_BASE_URL  default: http://localhost:8081
  DEV_RUNTIME_DIR          default: .local/runtime
  CUBEBOX_OPENAI_API_KEY   required by CubeBox at runtime for the default DeepSeek baseline when secret_ref=env://CUBEBOX_OPENAI_API_KEY
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --build-ui) build_ui=1; shift ;;
    --skip-cubebox-model) setup_cubebox=0; shift ;;
    --verify-cubebox) verify_cubebox=1; shift ;;
    --with-superadmin) with_superadmin=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *)
      echo "[dev-runtime] unknown arg: $1" >&2
      usage
      exit 2
      ;;
  esac
done

log() {
  printf '[dev-runtime] %s\n' "$*"
}

warn() {
  printf '[dev-runtime] warning: %s\n' "$*" >&2
}

require_cmd() {
  local name="${1:?}"
  if ! command -v "$name" >/dev/null 2>&1; then
    echo "[dev-runtime] missing required command: ${name}" >&2
    exit 2
  fi
}

load_env_file() {
  local file="${1:?}"
  if [[ -f "$file" ]]; then
    set -a
    # shellcheck disable=SC1090
    . "$file"
    set +a
  fi
}

first_existing_server_env_file() {
  if [[ -n "${DEV_SERVER_ENV_FILE:-}" ]]; then
    printf '%s' "$DEV_SERVER_ENV_FILE"
    return
  fi
  for candidate in .env.local env.local .env .env.example; do
    if [[ -f "$candidate" ]]; then
      printf '%s' "$candidate"
      return
    fi
  done
}

file_defines_env_key() {
  local file="${1:?}"
  local key="${2:?}"
  [[ -f "$file" ]] && grep -Eq "^[[:space:]]*(export[[:space:]]+)?${key}=" "$file"
}

extract_port_from_url() {
  local target_url="${1:?}"
  local fallback_port="${2:?}"
  python3 - "$target_url" "$fallback_port" <<'PY'
import sys
from urllib.parse import urlparse

parsed = urlparse(sys.argv[1])
fallback = int(sys.argv[2])
print(parsed.port or fallback)
PY
}

wait_http_ready() {
  local name="${1:?}"
  local url="${2:?}"
  local attempts="${3:-60}"
  log "wait ${name}: ${url}"
  for _ in $(seq 1 "$attempts"); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.5
  done
  echo "[dev-runtime] ${name} did not become ready: ${url}" >&2
  return 1
}

stop_stale_pid_file() {
  local pid_file="${1:?}"
  if [[ ! -f "$pid_file" ]]; then
    return 0
  fi
  local pid
  pid="$(tr -d '[:space:]' <"$pid_file" || true)"
  if [[ -z "$pid" ]]; then
    rm -f "$pid_file"
    return 0
  fi
  if kill -0 "$pid" >/dev/null 2>&1; then
    return 0
  fi
  rm -f "$pid_file"
}

start_if_needed() {
  local name="${1:?}"
  local pid_file="${2:?}"
  local log_file="${3:?}"
  shift 3

  stop_stale_pid_file "$pid_file"
  if [[ -f "$pid_file" ]]; then
    local pid
    pid="$(tr -d '[:space:]' <"$pid_file" || true)"
    if [[ -n "$pid" ]] && kill -0 "$pid" >/dev/null 2>&1; then
      log "${name} already running: pid=${pid}"
      return 0
    fi
  fi

  log "start ${name}: log=${log_file}"
  setsid nohup "$@" >"$log_file" 2>&1 </dev/null &
  local pid="$!"
  printf '%s\n' "$pid" >"$pid_file"
  disown "$pid" >/dev/null 2>&1 || true
  sleep 0.2
  if ! kill -0 "$pid" >/dev/null 2>&1; then
    echo "[dev-runtime] ${name} failed to start; see ${log_file}" >&2
    exit 1
  fi
}

json_string() {
  local value="${1:-}"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\n'/\\n}"
  value="${value//$'\r'/\\r}"
  printf '"%s"' "$value"
}

post_json() {
  local cookie_file="${1:?}"
  local url="${2:?}"
  local payload="${3:?}"
  local tmp
  tmp="$(mktemp)"
  local code
  code="$(curl -sS -b "$cookie_file" -o "$tmp" -w "%{http_code}" \
    -H "Content-Type: application/json" \
    -X POST "$url" \
    -d "$payload" || true)"
  if [[ "$code" =~ ^2[0-9][0-9]$ ]]; then
    rm -f "$tmp"
    return 0
  fi
  echo "[dev-runtime] POST failed: status=${code} url=${url}" >&2
  cat "$tmp" >&2
  rm -f "$tmp"
  return 1
}

require_cmd docker
require_cmd go
require_cmd curl
require_cmd python3
require_cmd setsid

if ! command -v pnpm >/dev/null 2>&1 && command -v corepack >/dev/null 2>&1; then
  corepack enable >/dev/null 2>&1 || true
  corepack prepare pnpm@10.24.0 --activate >/dev/null 2>&1 || true
fi

infra_env_file="${DEV_INFRA_ENV_FILE:-.env.example}"
if [[ ! -f "$infra_env_file" ]]; then
  echo "[dev-runtime] missing DEV_INFRA_ENV_FILE: ${infra_env_file}" >&2
  exit 2
fi

server_env_file="$(first_existing_server_env_file)"
if [[ -z "$server_env_file" ]]; then
  echo "[dev-runtime] no server env file found; expected .env.local/env.local/.env/.env.example or DEV_SERVER_ENV_FILE" >&2
  exit 2
fi

load_env_file "$infra_env_file"
load_env_file ".env"
load_env_file ".env.local"
load_env_file "env.local"

if file_defines_env_key ".env" "CUBEBOX_OPENAI_API_KEY" && file_defines_env_key ".env.local" "CUBEBOX_OPENAI_API_KEY"; then
  warn "CUBEBOX_OPENAI_API_KEY is present in both .env and .env.local; keep the real key in .env.local to avoid drift."
elif file_defines_env_key ".env" "CUBEBOX_OPENAI_API_KEY" && ! file_defines_env_key ".env.local" "CUBEBOX_OPENAI_API_KEY"; then
  warn "CUBEBOX_OPENAI_API_KEY is present in .env but not .env.local; CubeBox still works, but project policy prefers .env.local for local secrets."
fi

if [[ "$build_ui" == "1" || ! -f internal/server/assets/web/index.html ]]; then
  log "build embedded web assets"
  make css
fi

runtime_dir="${DEV_RUNTIME_DIR:-.local/runtime}"
mkdir -p "$runtime_dir"

log "start infra: DEV_INFRA_ENV_FILE=${infra_env_file}"
DEV_INFRA_ENV_FILE="$infra_env_file" make dev-up

compose_project="${DEV_COMPOSE_PROJECT:-bugs-and-blossoms-dev}"
postgres_cid="$(docker compose -p "$compose_project" --env-file "$infra_env_file" -f compose.dev.yml ps -q postgres)"
if [[ -z "$postgres_cid" ]]; then
  echo "[dev-runtime] postgres container not found (project=${compose_project})" >&2
  exit 2
fi

log "wait postgres"
for i in $(seq 1 120); do
  if docker exec -i "$postgres_cid" pg_isready -h localhost -U app -d postgres >/dev/null 2>&1; then
    break
  fi
  if [[ "$i" == "120" ]]; then
    echo "[dev-runtime] postgres did not become ready; see docker compose logs" >&2
    exit 1
  fi
  sleep 0.5
done

db_host="${DB_HOST:-127.0.0.1}"
db_port="${DB_PORT:-5438}"
db_name="${DB_NAME:-bugs_and_blossoms}"
db_pass="${DB_MIGRATION_PASSWORD:-${DB_ADMIN_PASSWORD:-${DB_PASSWORD:-app}}}"
db_sslmode="${DB_SSLMODE:-disable}"
admin_db_user="${DB_MIGRATION_USER:-${DB_ADMIN_USER:-app}}"
admin_db_url="postgres://${admin_db_user}:${db_pass}@${db_host}:${db_port}/${db_name}?sslmode=${db_sslmode}"

log "migrate iam"
DATABASE_URL="$admin_db_url" make iam migrate up
log "migrate orgunit"
DATABASE_URL="$admin_db_url" make orgunit migrate up

log "authz pack"
make authz-pack >/dev/null

server_base_url="${DEV_SERVER_BASE_URL:-http://localhost:8080}"
superadmin_base_url="${DEV_SUPERADMIN_BASE_URL:-http://localhost:8081}"
server_port="$(extract_port_from_url "$server_base_url" 8080)"
superadmin_port="$(extract_port_from_url "$superadmin_base_url" 8081)"

kratos_public_url="${KRATOS_PUBLIC_URL:-http://127.0.0.1:4433}"
kratos_admin_url="${KRATOS_STUB_ADMIN_BASE_URL:-${E2E_KRATOS_ADMIN_URL:-http://127.0.0.1:4434}}"
kratos_public_port="$(extract_port_from_url "$kratos_public_url" 4433)"
kratos_admin_port="$(extract_port_from_url "$kratos_admin_url" 4434)"
export KRATOS_STUB_PUBLIC_ADDR="${KRATOS_STUB_PUBLIC_ADDR:-127.0.0.1:${kratos_public_port}}"
export KRATOS_STUB_ADMIN_ADDR="${KRATOS_STUB_ADMIN_ADDR:-127.0.0.1:${kratos_admin_port}}"

kratos_pid_file="${runtime_dir}/dev-kratosstub.pid"
kratos_log_file="${runtime_dir}/dev-kratosstub.log"
server_pid_file="${runtime_dir}/dev-server.pid"
server_log_file="${runtime_dir}/dev-server.log"
superadmin_pid_file="${runtime_dir}/dev-superadmin.pid"
superadmin_log_file="${runtime_dir}/dev-superadmin.log"

start_if_needed "kratos stub" "$kratos_pid_file" "$kratos_log_file" make dev-kratos-stub
wait_http_ready "kratos public" "${kratos_public_url}/health/ready" 60
wait_http_ready "kratos admin" "${kratos_admin_url}/health/ready" 60

start_if_needed "server" "$server_pid_file" "$server_log_file" env DEV_SERVER_ENV_FILE="$server_env_file" DEV_SERVER_HTTP_ADDR=":${server_port}" make dev-server
wait_http_ready "server" "http://127.0.0.1:${server_port}/health" 60

if [[ "$with_superadmin" == "1" ]]; then
  superadmin_db_url="postgres://superadmin_runtime:${DB_PASSWORD:-app}@${db_host}:${db_port}/${db_name}?sslmode=${db_sslmode}"
  start_if_needed "superadmin" "$superadmin_pid_file" "$superadmin_log_file" env \
    DEV_SUPERADMIN_ENV_FILE="$server_env_file" \
    DEV_SUPERADMIN_HTTP_ADDR=":${superadmin_port}" \
    SUPERADMIN_DATABASE_URL="$superadmin_db_url" \
    SUPERADMIN_WRITE_MODE="${SUPERADMIN_WRITE_MODE:-enabled}" \
    make dev-superadmin
  wait_http_ready "superadmin" "http://127.0.0.1:${superadmin_port}/health" 60
fi

seed_script="${DEV_KRATOS_SEED_SCRIPT:-${E2E_KRATOS_SEED_SCRIPT:-./tools/codex/skills/bugs-and-blossoms-dev-login/scripts/seed_kratosstub_identity.sh}}"
if [[ ! -x "$seed_script" ]]; then
  echo "[dev-runtime] missing executable seed script: ${seed_script}" >&2
  exit 2
fi

log "seed KratosStub identities"
"$seed_script" --tenant-id 00000000-0000-0000-0000-000000000000 --email admin0@localhost --password admin123 --role-slug tenant-admin --kratos-admin-base-url "$kratos_admin_url" >/dev/null
"$seed_script" --tenant-id 00000000-0000-0000-0000-000000000001 --email admin@localhost --password admin123 --role-slug tenant-admin --kratos-admin-base-url "$kratos_admin_url" >/dev/null
"$seed_script" --tenant-id 00000000-0000-0000-0000-000000000002 --email admin2@localhost --password admin123 --role-slug tenant-admin --kratos-admin-base-url "$kratos_admin_url" >/dev/null

mkdir -p .local/codex
cookie_file=".local/codex/dev-runtime-cookies.txt"
login_payload="{\"email\":$(json_string "${DEV_LOGIN_EMAIL:-admin@localhost}"),\"password\":$(json_string "${DEV_LOGIN_PASSWORD:-admin123}")}"
login_tmp="$(mktemp)"
login_code="$(curl -sS -c "$cookie_file" -o "$login_tmp" -w "%{http_code}" \
  -H "Content-Type: application/json" \
  -X POST "${server_base_url}/iam/api/sessions" \
  -d "$login_payload" || true)"
if [[ "$login_code" != "204" ]]; then
  echo "[dev-runtime] login failed: status=${login_code} url=${server_base_url}/iam/api/sessions" >&2
  cat "$login_tmp" >&2
  rm -f "$login_tmp"
  exit 1
fi
rm -f "$login_tmp"
log "login OK: ${DEV_LOGIN_EMAIL:-admin@localhost}"

if [[ "$setup_cubebox" == "1" ]]; then
  if [[ -z "${CUBEBOX_OPENAI_API_KEY:-}" ]]; then
    warn "CUBEBOX_OPENAI_API_KEY is not visible to this shell after loading env files. CubeBox settings will be saved, but the default DeepSeek baseline will fail real turns until the server starts with CUBEBOX_OPENAI_API_KEY."
  fi

  provider_id="${CUBEBOX_PROVIDER_ID:-deepseek}"
  provider_type="${CUBEBOX_PROVIDER_TYPE:-openai-compatible}"
  provider_display_name="${CUBEBOX_PROVIDER_DISPLAY_NAME:-DeepSeek}"
  provider_base_url="${CUBEBOX_BASE_URL:-https://api.deepseek.com}"
  secret_ref="${CUBEBOX_SECRET_REF:-env://CUBEBOX_OPENAI_API_KEY}"
  masked_secret="${CUBEBOX_MASKED_SECRET:-env://CUBEBOX_OPENAI_API_KEY}"
  model_slug="${CUBEBOX_MODEL_SLUG:-deepseek-v4-flash}"
  capability_summary_json="${CUBEBOX_CAPABILITY_SUMMARY_JSON:-{}}"

  log "configure CubeBox provider: ${provider_id} / ${model_slug}"
  provider_payload="{\"provider_id\":$(json_string "$provider_id"),\"provider_type\":$(json_string "$provider_type"),\"display_name\":$(json_string "$provider_display_name"),\"base_url\":$(json_string "$provider_base_url"),\"enabled\":true}"
  post_json "$cookie_file" "${server_base_url}/internal/cubebox/settings/providers" "$provider_payload"

  credential_payload="{\"provider_id\":$(json_string "$provider_id"),\"secret_ref\":$(json_string "$secret_ref"),\"masked_secret\":$(json_string "$masked_secret")}"
  post_json "$cookie_file" "${server_base_url}/internal/cubebox/settings/credentials" "$credential_payload"

  selection_payload="{\"provider_id\":$(json_string "$provider_id"),\"model_slug\":$(json_string "$model_slug"),\"capability_summary\":${capability_summary_json}}"
  post_json "$cookie_file" "${server_base_url}/internal/cubebox/settings/selection" "$selection_payload"

  if [[ "$verify_cubebox" == "1" ]]; then
    log "verify CubeBox active model"
    post_json "$cookie_file" "${server_base_url}/internal/cubebox/settings/verify" "{}"
  fi
fi

log "ready: ${server_base_url}/app"
log "logs: ${server_log_file} ${kratos_log_file}"
if [[ "$with_superadmin" == "1" ]]; then
  log "superadmin: ${superadmin_base_url}"
fi
