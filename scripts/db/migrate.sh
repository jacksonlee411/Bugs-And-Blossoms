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

echo "[db-migrate] goose $direction: module=$module table=$goose_table"
case "$direction" in
  up)
    ./scripts/db/run_goose.sh -dir "$migrations_dir" -table "$goose_table" postgres "$db_url" up
    if [[ "$module" == "iam" ]]; then
      echo "[db-migrate] rls smoke: module=$module"
      go run ./cmd/dbtool rls-smoke --url "$db_url"
    fi
    ;;
  down)
    steps="${GOOSE_STEPS:-1}"
    if [[ "$steps" -lt 1 ]]; then
      echo "[db-migrate] invalid GOOSE_STEPS=$steps" >&2
      exit 2
    fi
    for _ in $(seq 1 "$steps"); do
      ./scripts/db/run_goose.sh -dir "$migrations_dir" -table "$goose_table" postgres "$db_url" down
    done
    ;;
esac
