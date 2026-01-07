#!/usr/bin/env bash
set -euo pipefail

base_url="${E2E_BASE_URL:-http://localhost:8080}"
server_log="${E2E_SERVER_LOG:-./e2e/test-results/server.log}"

if [[ ! "$base_url" =~ ^https?://localhost(:[0-9]+)?(/|$) ]]; then
  echo "[e2e] invalid E2E_BASE_URL=$base_url (must use localhost for Host->tenant fail-closed)" >&2
  exit 2
fi

if [[ "${AUTHZ_UNSAFE_ALLOW_DISABLED:-}" == "1" ]]; then
  echo "[e2e] AUTHZ_UNSAFE_ALLOW_DISABLED=1 is forbidden for E2E" >&2
  exit 2
fi

export AUTHZ_MODE="${AUTHZ_MODE:-enforce}"
export RLS_ENFORCE="${RLS_ENFORCE:-enforce}"

load_env_file() {
  local file="${1:?}"
  if [[ -f "$file" ]]; then
    set -a
    # shellcheck disable=SC1090
    . "$file"
    set +a
  fi
}

require_cmd() {
  local name="${1:?}"
  if ! command -v "$name" >/dev/null 2>&1; then
    echo "[e2e] missing required command: ${name}" >&2
    exit 2
  fi
}

require_cmd docker
require_cmd go

if ! command -v pnpm >/dev/null 2>&1; then
  if command -v corepack >/dev/null 2>&1; then
    corepack enable >/dev/null 2>&1 || true
    corepack prepare pnpm@10.24.0 --activate >/dev/null 2>&1 || true
  fi
fi
require_cmd pnpm

infra_env_file="${DEV_INFRA_ENV_FILE:-.env.example}"
load_env_file "$infra_env_file"

db_host="${DB_HOST:-127.0.0.1}"
db_port="${DB_PORT:-5438}"
db_name="${DB_NAME:-bugs_and_blossoms}"
db_pass="${DB_PASSWORD:-app}"
db_sslmode="${DB_SSLMODE:-disable}"

export DATABASE_URL="postgres://app_runtime:${db_pass}@${db_host}:${db_port}/${db_name}?sslmode=${db_sslmode}"

echo "[e2e] start infra: docker compose"
make dev-up

echo "[e2e] assert runtime db role (non-superuser, NOBYPASSRLS)"
compose_project="${DEV_COMPOSE_PROJECT:-bugs-and-blossoms-dev}"
compose_env_file="$infra_env_file"
postgres_cid="$(
  docker compose -p "$compose_project" --env-file "$compose_env_file" -f compose.dev.yml ps -q postgres
)"
if [[ -z "$postgres_cid" ]]; then
  echo "[e2e] postgres container not found (project=$compose_project)" >&2
  exit 2
fi

echo "[e2e] wait postgres ready"
role_line=""
get_role_line() {
  docker exec -i "$postgres_cid" psql -U app -d postgres -tAc \
    "SELECT (CASE WHEN rolsuper THEN 't' ELSE 'f' END) || '|' || (CASE WHEN rolbypassrls THEN 't' ELSE 'f' END) || '|' || (CASE WHEN rolcanlogin THEN 't' ELSE 'f' END) FROM pg_roles WHERE rolname='app_runtime';" \
    2>/dev/null || true
}

for i in $(seq 1 120); do
  if ! docker exec -i "$postgres_cid" pg_isready -U app -d postgres >/dev/null 2>&1; then
    sleep 0.5
    continue
  fi

  role_line="$(get_role_line)"
  role_line="$(echo "$role_line" | tr -d '[:space:]')"
  if [[ -n "$role_line" ]]; then
    break
  fi

  if [[ "$i" == "120" ]]; then
    echo "[e2e] missing role app_runtime after waiting; run \`make dev-reset\` once to re-init the dev database" >&2
    exit 2
  fi
  sleep 0.5
done

if [[ "$role_line" != "f|f|t" ]]; then
  echo "[e2e] invalid app_runtime role flags (expected rolsuper=false, rolbypassrls=false, rolcanlogin=true), got: $role_line" >&2
  exit 2
fi

echo "[e2e] migrate: iam/orgunit/jobcatalog/person/staffing"
make iam migrate up
make orgunit migrate up
make jobcatalog migrate up
make person migrate up
make staffing migrate up

mkdir -p "$(dirname "$server_log")"

echo "[e2e] start server: log=$server_log"
make dev-server >"$server_log" 2>&1 &
server_pid="$!"

cleanup() {
  if kill -0 "$server_pid" >/dev/null 2>&1; then
    kill "$server_pid" >/dev/null 2>&1 || true
    wait "$server_pid" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

echo "[e2e] wait server ready: http://127.0.0.1:8080/health"
for i in $(seq 1 60); do
  if command -v curl >/dev/null 2>&1; then
    if curl -fsS "http://127.0.0.1:8080/health" >/dev/null 2>&1; then
      break
    fi
  else
    if wget -q -O- "http://127.0.0.1:8080/health" >/dev/null 2>&1; then
      break
    fi
  fi
  if [[ "$i" == "60" ]]; then
    echo "[e2e] server did not become ready; see: $server_log" >&2
    exit 1
  fi
  sleep 0.5
done

echo "[e2e] install e2e deps (pnpm --frozen-lockfile)"
(cd e2e && pnpm install --frozen-lockfile)

echo "[e2e] assert tests exist (fail-fast on 0 tests)"
list_out="$(cd e2e && pnpm exec playwright test --list)"
printf "%s\n" "$list_out"
if ! echo "$list_out" | grep -Eq 'Total: [1-9][0-9]* test'; then
  echo "[e2e] no tests discovered; refusing to pass required check" >&2
  exit 1
fi

if [[ "${CI:-}" == "true" || "${CI:-}" == "1" ]]; then
  echo "[e2e] install playwright browsers (CI): chromium + deps"
  (cd e2e && pnpm exec playwright install --with-deps chromium)
else
  echo "[e2e] install playwright browsers: chromium"
  (cd e2e && pnpm exec playwright install chromium)
fi

echo "[e2e] run playwright: baseURL=$base_url"
if ! (cd e2e && E2E_BASE_URL="$base_url" pnpm exec playwright test); then
  echo "[e2e] reproduce locally: make e2e" >&2
  echo "[e2e] artifacts: e2e/test-results/ e2e/playwright-report/ (and server log: $server_log)" >&2
  exit 1
fi
