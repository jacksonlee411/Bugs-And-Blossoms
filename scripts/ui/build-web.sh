#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

if ! command -v node >/dev/null 2>&1; then
  echo "[ui] missing node; please install Node 20.x (SSOT: DEV-PLAN-011)" >&2
  exit 2
fi

if ! command -v corepack >/dev/null 2>&1; then
  echo "[ui] missing corepack; please ensure Node ships corepack or install it" >&2
  exit 2
fi

corepack enable >/dev/null 2>&1 || true
corepack prepare pnpm@10.24.0 --activate >/dev/null

pnpm -C apps/web install --frozen-lockfile
pnpm -C apps/web build

mui_dist_dir="apps/web/dist"
out_dir="internal/server/assets/web"

if [[ ! -f "${mui_dist_dir}/index.html" ]]; then
  echo "[ui] missing ${mui_dist_dir}/index.html; build output unexpected" >&2
  exit 2
fi

rm -rf "$out_dir"
mkdir -p "$out_dir"
cp -a "${mui_dist_dir}/." "$out_dir/"

echo "[ui] OK: ${out_dir}/index.html"

