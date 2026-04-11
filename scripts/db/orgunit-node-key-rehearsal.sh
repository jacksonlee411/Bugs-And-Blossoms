#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF' >&2
usage: orgunit-node-key-rehearsal.sh --source-url URL --target-url URL --as-of YYYY-MM-DD [options]

options:
  --snapshot PATH       snapshot json output path
  --setid-registry-snapshot PATH
                      setid strategy registry snapshot json output path
  --schema-dir PATH     target schema dir (default: modules/orgunit/infrastructure/persistence/org-node-key-bootstrap)
  --import-mode MODE    commit | dry-run (default: commit)
  --rehearse-setid-strategy-registry
                      after committed target org import/verify, run source export -> check -> target import -> verify for setid strategy registry
  --validate-setid-strategy-registry
                        after committed target setid strategy registry verify, run stopline validation
  --setid-registry-as-of YYYY-MM-DD
                        effective day passed to setid strategy registry validation (default: same as --as-of)
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
setid_registry_snapshot=""
schema_dir="modules/orgunit/infrastructure/persistence/org-node-key-bootstrap"
import_mode="commit"
skip_bootstrap="0"
rehearse_setid_strategy_registry="0"
validate_setid_strategy_registry="0"
setid_registry_as_of=""

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
    --setid-registry-snapshot)
      setid_registry_snapshot="${2:-}"
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
    --rehearse-setid-strategy-registry)
      rehearse_setid_strategy_registry="1"
      shift
      ;;
    --validate-setid-strategy-registry)
      validate_setid_strategy_registry="1"
      shift
      ;;
    --setid-registry-as-of)
      setid_registry_as_of="${2:-}"
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

if [[ -z "$setid_registry_as_of" ]]; then
  setid_registry_as_of="$as_of"
fi

if [[ "$rehearse_setid_strategy_registry" == "1" && "$import_mode" != "commit" ]]; then
  echo "[orgunit-node-key-rehearsal] --rehearse-setid-strategy-registry requires --import-mode commit" >&2
  exit 2
fi

if [[ "$validate_setid_strategy_registry" == "1" && "$import_mode" != "commit" ]]; then
  echo "[orgunit-node-key-rehearsal] --validate-setid-strategy-registry requires --import-mode commit" >&2
  exit 2
fi

if [[ -z "$snapshot" ]]; then
  timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
  snapshot=".local/orgunit-node-key-rehearsal/orgunit-snapshot-${timestamp}.json"
fi

if [[ -z "$setid_registry_snapshot" ]]; then
  timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
  setid_registry_snapshot=".local/orgunit-node-key-rehearsal/setid-strategy-registry-${timestamp}.json"
fi

mkdir -p "$(dirname "$snapshot")"
mkdir -p "$(dirname "$setid_registry_snapshot")"

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
  bootstrap_args=()
  if [[ "$rehearse_setid_strategy_registry" == "1" || "$validate_setid_strategy_registry" == "1" ]]; then
    bootstrap_args+=(--include-setid-strategy-registry)
  fi
  "${dbtool[@]}" orgunit-snapshot-bootstrap-target \
    --url "$target_url" \
    --schema-dir "$schema_dir" \
    "${bootstrap_args[@]}"
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

  if [[ "$rehearse_setid_strategy_registry" == "1" ]]; then
    echo "[orgunit-node-key-rehearsal] export source setid strategy registry snapshot"
    "${dbtool[@]}" orgunit-setid-strategy-registry-export \
      --url "$source_url" \
      --as-of "$setid_registry_as_of" \
      --output "$setid_registry_snapshot"

    echo "[orgunit-node-key-rehearsal] check source setid strategy registry snapshot"
    "${dbtool[@]}" orgunit-setid-strategy-registry-check \
      --input "$setid_registry_snapshot"

    echo "[orgunit-node-key-rehearsal] import target setid strategy registry snapshot"
    "${dbtool[@]}" orgunit-setid-strategy-registry-import \
      --url "$target_url" \
      --input "$setid_registry_snapshot"

    echo "[orgunit-node-key-rehearsal] verify committed target setid strategy registry snapshot"
    "${dbtool[@]}" orgunit-setid-strategy-registry-verify \
      --url "$target_url" \
      --input "$setid_registry_snapshot"
  fi

  if [[ "$validate_setid_strategy_registry" == "1" ]]; then
    echo "[orgunit-node-key-rehearsal] validate target setid strategy registry"
    ./scripts/db/orgunit-setid-strategy-registry-validate.sh \
      --url "$target_url" \
      --as-of "$setid_registry_as_of"
  fi
fi

echo "[orgunit-node-key-rehearsal] OK snapshot=$snapshot import_mode=$import_mode"
