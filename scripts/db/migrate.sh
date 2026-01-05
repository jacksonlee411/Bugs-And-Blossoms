#!/usr/bin/env bash
set -euo pipefail

module="${1:-}"
direction="${2:-}"

if [[ -z "$module" || -z "$direction" ]]; then
  echo "usage: migrate.sh <module> <up|down>" >&2
  exit 2
fi

if [[ ! -d "modules/$module" ]]; then
  echo "[db-migrate] unknown module: $module (expected modules/$module)" >&2
  exit 2
fi

case "$direction" in
  up|down) ;;
  *)
    echo "[db-migrate] invalid direction: $direction (expected up|down)" >&2
    exit 2
    ;;
esac

migrations_dir="migrations/$module"
if [[ ! -d "$migrations_dir" ]]; then
  echo "[db-migrate] missing migrations dir: $migrations_dir" >&2
  exit 2
fi

db_url="$("./scripts/db/db_url.sh")"
goose_table="goose_db_version_${module}"

run_goose_with_retry() {
  local max="${GOOSE_CONNECT_RETRY_MAX:-20}"
  local delay="${GOOSE_CONNECT_RETRY_DELAY_SECONDS:-1}"
  local attempt=1

  while true; do
    local output=""
    if output="$("$@" 2>&1)"; then
      printf "%s\n" "$output"
      return 0
    fi

    printf "%s\n" "$output" >&2

    if echo "$output" | grep -Eiq 'failed to connect|connection refused|connection reset by peer|server closed the connection unexpectedly'; then
      if [[ "$attempt" -lt "$max" ]]; then
        echo "[db-migrate] connection not ready; retry ${attempt}/${max} after ${delay}s" >&2
        attempt=$((attempt + 1))
        sleep "$delay"
        continue
      fi
    fi

    return 1
  done
}

echo "[db-migrate] goose $direction: module=$module table=$goose_table"
case "$direction" in
  up)
    run_goose_with_retry ./scripts/db/run_goose.sh -dir "$migrations_dir" -table "$goose_table" postgres "$db_url" up
    if [[ "$module" == "iam" ]]; then
      echo "[db-migrate] rls smoke: module=$module"
      go run ./cmd/dbtool rls-smoke --url "$db_url"
    fi
    if [[ "$module" == "orgunit" ]]; then
      echo "[db-migrate] orgunit smoke: module=$module"
      go run ./cmd/dbtool orgunit-smoke --url "$db_url"
    fi
    ;;
  down)
    steps="${GOOSE_STEPS:-1}"
    if [[ "$steps" -lt 1 ]]; then
      echo "[db-migrate] invalid GOOSE_STEPS=$steps" >&2
      exit 2
    fi
    for _ in $(seq 1 "$steps"); do
      run_goose_with_retry ./scripts/db/run_goose.sh -dir "$migrations_dir" -table "$goose_table" postgres "$db_url" down
    done
    ;;
esac
