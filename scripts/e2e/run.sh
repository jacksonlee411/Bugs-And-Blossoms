#!/usr/bin/env bash
set -euo pipefail

base_url="${E2E_BASE_URL:-http://localhost:8080}"
server_log="${E2E_SERVER_LOG:-./e2e/_artifacts/server.log}"
superadmin_log="${E2E_SUPERADMIN_LOG:-./e2e/_artifacts/superadmin.log}"
kratos_log="${E2E_KRATOS_LOG:-./e2e/_artifacts/kratosstub.log}"

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
export TRUST_PROXY="${TRUST_PROXY:-1}"

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

assert_port_free() {
  local host="${1:?}"
  local port="${2:?}"
  local service_name="${3:?}"
  if ! python3 - "$host" "$port" <<'PY'
import socket
import sys

host = sys.argv[1]
port = int(sys.argv[2])
sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
sock.settimeout(0.25)
try:
    sock.connect((host, port))
except OSError:
    sys.exit(0)
finally:
    sock.close()
sys.exit(1)
PY
  then
    echo "[e2e] ${service_name} port already in use: ${host}:${port}" >&2
    echo "[e2e] stop the existing service or override ports (E2E_BASE_URL / E2E_SUPERADMIN_BASE_URL / KRATOS_PUBLIC_URL / E2E_KRATOS_ADMIN_URL)" >&2
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

require_cmd() {
  local name="${1:?}"
  if ! command -v "$name" >/dev/null 2>&1; then
    echo "[e2e] missing required command: ${name}" >&2
    exit 2
  fi
}

require_cmd docker
require_cmd go
require_cmd python3
require_cmd curl

if ! command -v pnpm >/dev/null 2>&1; then
  if command -v corepack >/dev/null 2>&1; then
    corepack enable >/dev/null 2>&1 || true
    corepack prepare pnpm@10.24.0 --activate >/dev/null 2>&1 || true
  fi
fi
require_cmd pnpm

infra_env_file="${DEV_INFRA_ENV_FILE:-.env.example}"
load_env_file "$infra_env_file"
if [[ "$infra_env_file" != ".env" && -f .env ]]; then
  load_env_file .env
fi
export DEV_INFRA_ENV_FILE="$infra_env_file"

db_host="${DB_HOST:-127.0.0.1}"
db_port="${DB_PORT:-5438}"
db_name="${DB_NAME:-bugs_and_blossoms}"
db_pass="${DB_PASSWORD:-app}"
db_sslmode="${DB_SSLMODE:-disable}"

admin_db_user="${E2E_DB_ADMIN_USER:-app}"
runtime_db_user="app_runtime"
admin_db_url="postgres://${admin_db_user}:${db_pass}@${db_host}:${db_port}/${db_name}?sslmode=${db_sslmode}"
runtime_db_url="postgres://${runtime_db_user}:${db_pass}@${db_host}:${db_port}/${db_name}?sslmode=${db_sslmode}"

export DATABASE_URL="$runtime_db_url"

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
  docker exec -i "$postgres_cid" psql -h localhost -U app -d postgres -tAc \
    "SELECT (CASE WHEN rolsuper THEN 't' ELSE 'f' END) || '|' || (CASE WHEN rolbypassrls THEN 't' ELSE 'f' END) || '|' || (CASE WHEN rolcanlogin THEN 't' ELSE 'f' END) FROM pg_roles WHERE rolname='app_runtime';" \
    2>/dev/null || true
}

for i in $(seq 1 120); do
  if ! docker exec -i "$postgres_cid" pg_isready -h localhost -U app -d postgres >/dev/null 2>&1; then
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

echo "[e2e] ensure superadmin_runtime role exists (dev-only)"
docker exec -i "$postgres_cid" psql -h localhost -U app -d postgres -v ON_ERROR_STOP=1 <<'SQL' >/dev/null
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'superadmin_runtime') THEN
    CREATE ROLE superadmin_runtime
      LOGIN
      PASSWORD 'app'
      NOSUPERUSER
      NOCREATEDB
      NOCREATEROLE
      NOREPLICATION
      NOBYPASSRLS;
  END IF;
END
$$;
GRANT app_nobypassrls TO superadmin_runtime;
GRANT ALL PRIVILEGES ON DATABASE bugs_and_blossoms TO superadmin_runtime;
SQL

echo "[e2e] assert superadmin_runtime role exists"
sa_role_line="$(
  docker exec -i "$postgres_cid" psql -h localhost -U app -d postgres -tAc \
    "SELECT (CASE WHEN rolsuper THEN 't' ELSE 'f' END) || '|' || (CASE WHEN rolbypassrls THEN 't' ELSE 'f' END) || '|' || (CASE WHEN rolcanlogin THEN 't' ELSE 'f' END) FROM pg_roles WHERE rolname='superadmin_runtime';" \
    2>/dev/null || true
)"
sa_role_line="$(echo "$sa_role_line" | tr -d '[:space:]')"
if [[ -z "$sa_role_line" ]]; then
  echo "[e2e] missing role superadmin_runtime; run \`make dev-reset\` once to re-init the dev database" >&2
  exit 2
fi
if [[ "$sa_role_line" != "f|f|t" ]]; then
  echo "[e2e] invalid superadmin_runtime role flags (expected rolsuper=false, rolbypassrls=false, rolcanlogin=true), got: $sa_role_line" >&2
  exit 2
fi

echo "[e2e] migrate: iam/orgunit/jobcatalog/person/staffing"
DATABASE_URL="$admin_db_url" make iam migrate up
DATABASE_URL="$admin_db_url" make orgunit migrate up
DATABASE_URL="$admin_db_url" make jobcatalog migrate up
DATABASE_URL="$admin_db_url" make person migrate up
DATABASE_URL="$admin_db_url" make staffing migrate up

mkdir -p "$(dirname "$server_log")"
mkdir -p "$(dirname "$superadmin_log")"
mkdir -p "$(dirname "$kratos_log")"

echo "[e2e] authz pack (policy.csv must be up-to-date)"
make authz-pack >/dev/null

export KRATOS_PUBLIC_URL="${KRATOS_PUBLIC_URL:-http://127.0.0.1:4433}"
export E2E_KRATOS_ADMIN_URL="${E2E_KRATOS_ADMIN_URL:-http://127.0.0.1:4434}"
superadmin_base_url="${E2E_SUPERADMIN_BASE_URL:-http://localhost:8081}"
e2e_server_env_file="${E2E_SERVER_ENV_FILE:-$infra_env_file}"
e2e_superadmin_env_file="${E2E_SUPERADMIN_ENV_FILE:-$infra_env_file}"
if [[ -z "${ASSISTANT_MODEL_CONFIG_JSON:-}" ]]; then
  export ASSISTANT_MODEL_CONFIG_JSON='{"provider_routing":{"strategy":"priority_failover","fallback_enabled":true},"providers":[{"name":"openai","enabled":true,"model":"gpt-5-codex","endpoint":"https://api.openai.com/v1","timeout_ms":8000,"retries":1,"priority":10,"key_ref":"OPENAI_API_KEY"}]}'
fi

server_port="$(extract_port_from_url "$base_url" 8080)"
superadmin_port="$(extract_port_from_url "$superadmin_base_url" 8081)"
kratos_public_port="$(extract_port_from_url "$KRATOS_PUBLIC_URL" 4433)"
kratos_admin_port="$(extract_port_from_url "$E2E_KRATOS_ADMIN_URL" 4434)"
export KRATOS_STUB_PUBLIC_ADDR="${KRATOS_STUB_PUBLIC_ADDR:-127.0.0.1:${kratos_public_port}}"
export KRATOS_STUB_ADMIN_ADDR="${KRATOS_STUB_ADMIN_ADDR:-127.0.0.1:${kratos_admin_port}}"

server_health_url="http://127.0.0.1:${server_port}/health"
superadmin_health_url="http://127.0.0.1:${superadmin_port}/health"

assert_port_free "127.0.0.1" "$server_port" "server"
assert_port_free "127.0.0.1" "$superadmin_port" "superadmin"
assert_port_free "127.0.0.1" "$kratos_public_port" "kratos public"
assert_port_free "127.0.0.1" "$kratos_admin_port" "kratos admin"

echo "[e2e] start kratos stub: log=$kratos_log"
go run ./cmd/kratosstub >"$kratos_log" 2>&1 &
kratos_pid="$!"
sleep 0.2
if ! kill -0 "$kratos_pid" >/dev/null 2>&1; then
  echo "[e2e] kratos stub failed to start; see: $kratos_log" >&2
  exit 1
fi

echo "[e2e] start server: log=$server_log"
DEV_SERVER_ENV_FILE="$e2e_server_env_file" DEV_SERVER_HTTP_ADDR=":${server_port}" make dev-server >"$server_log" 2>&1 &
server_pid="$!"
sleep 0.2
if ! kill -0 "$server_pid" >/dev/null 2>&1; then
  echo "[e2e] server failed to start; see: $server_log" >&2
  exit 1
fi

superadmin_db_url="postgres://superadmin_runtime:${db_pass}@${db_host}:${db_port}/${db_name}?sslmode=${db_sslmode}"
export SUPERADMIN_DATABASE_URL="$superadmin_db_url"
export SUPERADMIN_BASIC_AUTH_USER="${E2E_SUPERADMIN_USER:-admin}"
export SUPERADMIN_BASIC_AUTH_PASS="${E2E_SUPERADMIN_PASS:-admin}"
export SUPERADMIN_WRITE_MODE="${SUPERADMIN_WRITE_MODE:-enabled}"

echo "[e2e] start superadmin: log=$superadmin_log"
DEV_SUPERADMIN_ENV_FILE="$e2e_superadmin_env_file" DEV_SUPERADMIN_HTTP_ADDR=":${superadmin_port}" make dev-superadmin >"$superadmin_log" 2>&1 &
superadmin_pid="$!"
sleep 0.2
if ! kill -0 "$superadmin_pid" >/dev/null 2>&1; then
  echo "[e2e] superadmin failed to start; see: $superadmin_log" >&2
  exit 1
fi

cleanup() {
  if kill -0 "$server_pid" >/dev/null 2>&1; then
    kill "$server_pid" >/dev/null 2>&1 || true
    wait "$server_pid" >/dev/null 2>&1 || true
  fi
  if kill -0 "$kratos_pid" >/dev/null 2>&1; then
    kill "$kratos_pid" >/dev/null 2>&1 || true
    wait "$kratos_pid" >/dev/null 2>&1 || true
  fi
  if kill -0 "$superadmin_pid" >/dev/null 2>&1; then
    kill "$superadmin_pid" >/dev/null 2>&1 || true
    wait "$superadmin_pid" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

echo "[e2e] wait server ready: $server_health_url"
for i in $(seq 1 60); do
  if ! kill -0 "$server_pid" >/dev/null 2>&1; then
    echo "[e2e] server exited before becoming ready; see: $server_log" >&2
    exit 1
  fi
  if command -v curl >/dev/null 2>&1; then
    if curl -fsS "$server_health_url" >/dev/null 2>&1; then
      break
    fi
  else
    if wget -q -O- "$server_health_url" >/dev/null 2>&1; then
      break
    fi
  fi
  if [[ "$i" == "60" ]]; then
    echo "[e2e] server did not become ready; see: $server_log" >&2
    exit 1
  fi
  sleep 0.5
done

echo "[e2e] wait superadmin ready: $superadmin_health_url"
for i in $(seq 1 60); do
  if ! kill -0 "$superadmin_pid" >/dev/null 2>&1; then
    echo "[e2e] superadmin exited before becoming ready; see: $superadmin_log" >&2
    exit 1
  fi
  if command -v curl >/dev/null 2>&1; then
    if curl -fsS "$superadmin_health_url" >/dev/null 2>&1; then
      break
    fi
  else
    if wget -q -O- "$superadmin_health_url" >/dev/null 2>&1; then
      break
    fi
  fi
  if [[ "$i" == "60" ]]; then
    echo "[e2e] superadmin did not become ready; see: $superadmin_log" >&2
    exit 1
  fi
  sleep 0.5
done

echo "[e2e] wait kratos ready: $KRATOS_PUBLIC_URL/health/ready"
for i in $(seq 1 60); do
  if ! kill -0 "$kratos_pid" >/dev/null 2>&1; then
    echo "[e2e] kratos exited before becoming ready; see: $kratos_log" >&2
    exit 1
  fi
  if command -v curl >/dev/null 2>&1; then
    if curl -fsS "$KRATOS_PUBLIC_URL/health/ready" >/dev/null 2>&1; then
      break
    fi
  else
    if wget -q -O- "$KRATOS_PUBLIC_URL/health/ready" >/dev/null 2>&1; then
      break
    fi
  fi
  if [[ "$i" == "60" ]]; then
    echo "[e2e] kratos did not become ready; see: $kratos_log" >&2
    exit 1
  fi
  sleep 0.5
done

echo "[e2e] wait kratos admin ready: $E2E_KRATOS_ADMIN_URL/health/ready"
for i in $(seq 1 60); do
  if ! kill -0 "$kratos_pid" >/dev/null 2>&1; then
    echo "[e2e] kratos exited before becoming ready; see: $kratos_log" >&2
    exit 1
  fi
  if command -v curl >/dev/null 2>&1; then
    if curl -fsS "$E2E_KRATOS_ADMIN_URL/health/ready" >/dev/null 2>&1; then
      break
    fi
  else
    if wget -q -O- "$E2E_KRATOS_ADMIN_URL/health/ready" >/dev/null 2>&1; then
      break
    fi
  fi
  if [[ "$i" == "60" ]]; then
    echo "[e2e] kratos admin did not become ready; see: $kratos_log" >&2
    exit 1
  fi
  sleep 0.5
done

kratos_seed_script="./tools/codex/skills/bugs-and-blossoms-dev-login/scripts/seed_kratosstub_identity.sh"
if [[ ! -x "$kratos_seed_script" ]]; then
  echo "[e2e] missing kratos seed script: $kratos_seed_script" >&2
  exit 1
fi

echo "[e2e] seed default kratosstub identities"
"$kratos_seed_script" --tenant-id 00000000-0000-0000-0000-000000000000 --email admin0@localhost --password admin123 --role-slug tenant-admin --kratos-admin-base-url "$E2E_KRATOS_ADMIN_URL"
"$kratos_seed_script" --tenant-id 00000000-0000-0000-0000-000000000001 --email admin@localhost --password admin123 --role-slug tenant-admin --kratos-admin-base-url "$E2E_KRATOS_ADMIN_URL"
"$kratos_seed_script" --tenant-id 00000000-0000-0000-0000-000000000002 --email admin2@localhost --password admin123 --role-slug tenant-admin --kratos-admin-base-url "$E2E_KRATOS_ADMIN_URL"

echo "[e2e] install e2e deps (pnpm --frozen-lockfile)"
(cd e2e && pnpm install --frozen-lockfile)

echo "[e2e] assert tests exist (fail-fast on 0 tests)"
list_out="$(cd e2e && pnpm exec playwright test --list "$@")"
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
if ! (cd e2e && E2E_BASE_URL="$base_url" E2E_SUPERADMIN_BASE_URL="${E2E_SUPERADMIN_BASE_URL:-http://localhost:8081}" E2E_SUPERADMIN_USER="${E2E_SUPERADMIN_USER:-admin}" E2E_SUPERADMIN_PASS="${E2E_SUPERADMIN_PASS:-admin}" pnpm exec playwright test "$@"); then
  echo "[e2e] reproduce locally: make e2e" >&2
  echo "[e2e] artifacts: e2e/test-results/ e2e/playwright-report/ (server log: $server_log; superadmin log: $superadmin_log)" >&2
  exit 1
fi
