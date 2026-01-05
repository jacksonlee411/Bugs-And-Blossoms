#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
mkdir -p "$root/bin"

if [[ -x "$root/bin/goimports" ]]; then
  exit 0
fi

version="${GOIMPORTS_VERSION:-v0.26.0}"
GOBIN="$root/bin" go install "golang.org/x/tools/cmd/goimports@${version}"

