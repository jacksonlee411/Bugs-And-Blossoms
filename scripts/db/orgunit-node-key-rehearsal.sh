#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF' >&2
usage: orgunit-node-key-rehearsal.sh --source-url URL --target-url URL --as-of YYYY-MM-DD [options]

options:
  --snapshot PATH       snapshot json output path
  --schema-dir PATH     target schema dir (default: modules/orgunit/infrastructure/persistence/schema)
  --import-mode MODE    commit | dry-run (default: commit)
  --skip-bootstrap      skip target bootstrap step

notes:
  source-url must be an owner/bypass-RLS connection able to export all tenant snapshots.
  target-url must point to a dedicated rehearsal database with DDL privileges.
EOF
  exit 2
}

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

source_url=""
target_url=""
as_of=""
snapshot=""
schema_dir="modules/orgunit/infrastructure/persistence/schema"
import_mode="commit"
skip_bootstrap="0"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --source-url)
      source_url="${2:-}"
      shift 2
      ;;
    --target-url)
      target_url="${2:-}"
      shift 2
      ;;
    --as-of)
      as_of="${2:-}"
      shift 2
      ;;
    --snapshot)
      snapshot="${2:-}"
      shift 2
      ;;
    --schema-dir)
      schema_dir="${2:-}"
      shift 2
      ;;
    --import-mode)
      import_mode="${2:-}"
      shift 2
      ;;
    --skip-bootstrap)
      skip_bootstrap="1"
      shift
      ;;
    -h|--help)
      usage
      ;;
    *)
      echo "[orgunit-node-key-rehearsal] unknown argument: $1" >&2
      usage
      ;;
  esac
done

if [[ -z "$source_url" || -z "$target_url" || -z "$as_of" ]]; then
  usage
fi

if [[ "$source_url" == "$target_url" ]]; then
  echo "[orgunit-node-key-rehearsal] source and target URLs must differ" >&2
  exit 2
fi

case "$import_mode" in
  commit|dry-run) ;;
  *)
    echo "[orgunit-node-key-rehearsal] invalid --import-mode: $import_mode" >&2
    usage
    ;;
esac

if [[ -z "$snapshot" ]]; then
  timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
  snapshot=".local/orgunit-node-key-rehearsal/orgunit-snapshot-${timestamp}.json"
fi

mkdir -p "$(dirname "$snapshot")"

dbtool=(go run ./cmd/dbtool)

echo "[orgunit-node-key-rehearsal] export source snapshot"
"${dbtool[@]}" orgunit-snapshot-export \
  --url "$source_url" \
  --as-of "$as_of" \
  --output "$snapshot"

echo "[orgunit-node-key-rehearsal] check source snapshot"
"${dbtool[@]}" orgunit-snapshot-check \
  --input "$snapshot"

if [[ "$skip_bootstrap" != "1" ]]; then
  echo "[orgunit-node-key-rehearsal] bootstrap target schema"
  "${dbtool[@]}" orgunit-snapshot-bootstrap-target \
    --url "$target_url" \
    --schema-dir "$schema_dir"
fi

echo "[orgunit-node-key-rehearsal] import target snapshot mode=$import_mode"
if [[ "$import_mode" == "dry-run" ]]; then
  "${dbtool[@]}" orgunit-snapshot-import \
    --url "$target_url" \
    --input "$snapshot" \
    --dry-run
  echo "[orgunit-node-key-rehearsal] dry-run import already executed in-transaction verify; skip standalone verify"
else
  "${dbtool[@]}" orgunit-snapshot-import \
    --url "$target_url" \
    --input "$snapshot"
  echo "[orgunit-node-key-rehearsal] verify committed target snapshot"
  "${dbtool[@]}" orgunit-snapshot-verify \
    --url "$target_url" \
    --input "$snapshot"
fi

echo "[orgunit-node-key-rehearsal] OK snapshot=$snapshot import_mode=$import_mode"
