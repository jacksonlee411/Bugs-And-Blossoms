#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
mkdir -p "$root/bin"

if [[ -x "$root/bin/atlas" ]]; then
  exit 0
fi

export CI=true
export ATLAS_NO_UPDATE_NOTIFIER=true
export ATLAS_VERSION="${ATLAS_VERSION:-v0.38.0}"

sh -c "$(curl -sSfL https://atlasgo.sh)" -- -y -o "$root/bin/atlas"

