#!/usr/bin/env bash
set -euo pipefail

mode="${1:-runtime}"

case "$mode" in
  runtime|migration) ;;
  *)
    echo "usage: db_url.sh [runtime|migration]" >&2
    exit 2
    ;;
esac

load_env() {
  local file="${1:?}"
  if [[ -f "$file" ]]; then
    set -a
    # shellcheck disable=SC1090
    . "$file"
    set +a
  fi
}

load_env ".env"
load_env ".env.local"
load_env "env.local"

if [[ "$mode" == "migration" ]]; then
  if [[ -n "${MIGRATION_DATABASE_URL:-}" ]]; then
    echo "$MIGRATION_DATABASE_URL"
    exit 0
  fi
  if [[ -n "${DATABASE_URL:-}" ]]; then
    echo "$DATABASE_URL"
    exit 0
  fi
else
  if [[ -n "${DATABASE_URL:-}" ]]; then
    echo "$DATABASE_URL"
    exit 0
  fi
fi

if [[ "$mode" == "migration" ]]; then
  host="${DB_MIGRATION_HOST:-${DB_HOST:-localhost}}"
  port="${DB_MIGRATION_PORT:-${DB_PORT:-5438}}"
  user="${DB_MIGRATION_USER:-${DB_ADMIN_USER:-app}}"
  password="${DB_MIGRATION_PASSWORD:-${DB_ADMIN_PASSWORD:-${DB_PASSWORD:-app}}}"
  name="${DB_MIGRATION_NAME:-${DB_NAME:-bugs_and_blossoms}}"
  sslmode="${DB_MIGRATION_SSLMODE:-${DB_SSLMODE:-disable}}"
else
  host="${DB_HOST:-localhost}"
  port="${DB_PORT:-5438}"
  user="${DB_USER:-app}"
  password="${DB_PASSWORD:-app}"
  name="${DB_NAME:-bugs_and_blossoms}"
  sslmode="${DB_SSLMODE:-disable}"
fi

printf "postgres://%s:%s@%s:%s/%s?sslmode=%s" "$user" "$password" "$host" "$port" "$name" "$sslmode"
