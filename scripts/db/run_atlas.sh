#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
"$root/scripts/db/install_atlas.sh"

exec "$root/bin/atlas" "$@"

