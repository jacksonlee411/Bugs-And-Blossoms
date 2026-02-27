#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
schema_file="$root/internal/sqlc/schema.sql"

if [[ ! -f "$schema_file" ]]; then
  echo "[sqlc-verify] missing schema file: $schema_file" >&2
  echo "[sqlc-verify] run make sqlc-generate first" >&2
  exit 1
fi

list_modules() {
  find "$root/modules" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' | LC_ALL=C sort
}

host="${DB_HOST:-localhost}"
port="${DB_PORT:-5438}"
user="${DB_USER:-app}"
password="${DB_PASSWORD:-app}"
sslmode="${DB_SSLMODE:-disable}"
admin_db="${DB_ADMIN_DB:-postgres}"

export PGPASSWORD="$password"

if ! pg_isready -h "$host" -p "$port" -U "$user" >/dev/null 2>&1; then
  echo "[sqlc-verify] database is not ready: host=$host port=$port user=$user" >&2
  echo "[sqlc-verify] set DB_HOST/DB_PORT/DB_USER/DB_PASSWORD and ensure postgres is reachable" >&2
  exit 1
fi

suffix="$(date -u +%s)_$$"
db_from_migrations="sqlc_verify_m_${suffix}"
db_from_export="sqlc_verify_e_${suffix}"

cleanup() {
  psql "postgres://${user}:${password}@${host}:${port}/${admin_db}?sslmode=${sslmode}" \
    -v ON_ERROR_STOP=1 \
    -c "DROP DATABASE IF EXISTS ${db_from_migrations};" >/dev/null 2>&1 || true
  psql "postgres://${user}:${password}@${host}:${port}/${admin_db}?sslmode=${sslmode}" \
    -v ON_ERROR_STOP=1 \
    -c "DROP DATABASE IF EXISTS ${db_from_export};" >/dev/null 2>&1 || true
}
trap cleanup EXIT

admin_url="postgres://${user}:${password}@${host}:${port}/${admin_db}?sslmode=${sslmode}"
migrations_db_url="postgres://${user}:${password}@${host}:${port}/${db_from_migrations}?sslmode=${sslmode}"
export_db_url="postgres://${user}:${password}@${host}:${port}/${db_from_export}?sslmode=${sslmode}"
atlas_dev_url="${ATLAS_DEV_URL:-$admin_url}"

echo "[sqlc-verify] create temp databases"
psql "$admin_url" -v ON_ERROR_STOP=1 -c "CREATE DATABASE ${db_from_migrations};" >/dev/null
psql "$admin_url" -v ON_ERROR_STOP=1 -c "CREATE DATABASE ${db_from_export};" >/dev/null

echo "[sqlc-verify] apply migrations to ${db_from_migrations}"
while IFS= read -r module; do
  schema_dir="$root/modules/$module/infrastructure/persistence/schema"
  migrations_dir="$root/migrations/$module"
  if [[ ! -d "$schema_dir" ]]; then
    continue
  fi
  if [[ ! -d "$migrations_dir" ]]; then
    echo "[sqlc-verify] missing migrations dir for module: $module ($migrations_dir)" >&2
    exit 1
  fi
  "$root/scripts/db/run_goose.sh" \
    -dir "$migrations_dir" \
    -table "goose_db_version_${module}" \
    postgres \
    "$migrations_db_url" \
    up >/dev/null
done < <(list_modules)

echo "[sqlc-verify] apply exported schema to ${db_from_export}"
psql "$export_db_url" -v ON_ERROR_STOP=1 -f "$schema_file" >/dev/null

echo "[sqlc-verify] compare schemas with atlas diff"
if ! drift_sql="$(
  ATLAS_NO_UPDATE_NOTIFIER=true \
    "$root/scripts/db/run_atlas.sh" schema diff \
    --from "$migrations_db_url" \
    --to "$export_db_url" \
    --exclude "public.goose_db_version_*" \
    --exclude "goose_db_version_*" \
    --dev-url "$atlas_dev_url" \
    --format '{{ sql . }}'
)"; then
  echo "[sqlc-verify] atlas diff failed (migrations -> exported schema)" >&2
  exit 1
fi

if [[ -n "${drift_sql:-}" ]]; then
  echo "[sqlc-verify] schema mismatch: migrations-applied DB != internal/sqlc/schema.sql" >&2
  echo "$drift_sql" >&2
  exit 1
fi

echo "[sqlc-verify] OK: schema is consistent"
