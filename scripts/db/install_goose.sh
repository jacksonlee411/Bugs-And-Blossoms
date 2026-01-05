#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
mkdir -p "$root/bin"

if [[ -x "$root/bin/goose" ]]; then
  exit 0
fi

version="${GOOSE_VERSION:-v3.26.0}"
GOBIN="$root/bin" go install "github.com/pressly/goose/v3/cmd/goose@${version}"

