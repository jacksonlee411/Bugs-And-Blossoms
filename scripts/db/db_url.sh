#!/usr/bin/env bash
set -euo pipefail

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

if [[ -n "${DATABASE_URL:-}" ]]; then
  echo "$DATABASE_URL"
  exit 0
fi

host="${DB_HOST:-localhost}"
port="${DB_PORT:-5438}"
user="${DB_USER:-app}"
password="${DB_PASSWORD:-app}"
name="${DB_NAME:-bugs_and_blossoms}"
sslmode="${DB_SSLMODE:-disable}"

printf "postgres://%s:%s@%s:%s/%s?sslmode=%s" "$user" "$password" "$host" "$port" "$name" "$sslmode"
