#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF' >&2
usage: orgunit-setid-strategy-registry-validate.sh --as-of YYYY-MM-DD [--url URL]

options:
  --as-of YYYY-MM-DD   current-state effective day used to resolve business_unit_node_key
  --url URL            target database url (default: scripts/db/db_url.sh migration)
EOF
  exit 2
}

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

url=""
as_of=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --url)
      url="${2:-}"
      shift 2
      ;;
    --as-of)
      as_of="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      ;;
    *)
      echo "[orgunit-setid-strategy-registry-validate] unknown argument: $1" >&2
      usage
      ;;
  esac
done

if [[ -z "$as_of" ]]; then
  usage
fi

if [[ -z "$url" ]]; then
  url="$(./scripts/db/db_url.sh migration)"
fi

go run ./cmd/dbtool orgunit-setid-strategy-registry-validate \
  --url "$url" \
  --as-of "$as_of"
