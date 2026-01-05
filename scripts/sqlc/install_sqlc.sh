#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
mkdir -p "$root/bin"

if [[ -x "$root/bin/sqlc" ]]; then
  exit 0
fi

version="${SQLC_VERSION:-v1.28.0}"
GOBIN="$root/bin" go install "github.com/sqlc-dev/sqlc/cmd/sqlc@${version}"

