#!/usr/bin/env bash
set -euo pipefail

root="$(git rev-parse --show-toplevel)"
codex_home="${CODEX_HOME:-$HOME/.codex}"

mkdir -p "${codex_home}/skills"

install_one() {
  local name="${1:?}"
  local src="${root}/tools/codex/skills/${name}"
  local dst="${codex_home}/skills/${name}"

  if [[ ! -d "$src" ]]; then
    echo "[codex-skill] missing source: $src" >&2
    return 1
  fi

  if [[ -L "$dst" ]]; then
    ln -sfn "$src" "$dst"
  elif [[ -d "$dst" ]]; then
    echo "[codex-skill] destination is a directory; updating in-place (non-destructive): $dst" >&2
    cp -f "$src/SKILL.md" "$dst/SKILL.md"
    mkdir -p "$dst/scripts"
    if [[ -f "$src/scripts/seed_kratosstub_identity.sh" ]]; then
      cp -f "$src/scripts/seed_kratosstub_identity.sh" "$dst/scripts/seed_kratosstub_identity.sh"
    fi
  else
    ln -sfn "$src" "$dst"
  fi

  if [[ -f "$src/scripts/seed_kratosstub_identity.sh" ]]; then
    chmod +x "$src/scripts/seed_kratosstub_identity.sh" || true
  fi

  echo "[codex-skill] installed: ${name} -> ${dst}"
}

install_one "bugs-and-blossoms-dev-login"
