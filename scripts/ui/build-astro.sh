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

dist_dir="apps/web/dist"
out_dir="internal/server/assets/astro"

if [[ ! -f "${dist_dir}/index.html" ]]; then
  echo "[ui] missing ${dist_dir}/index.html; build output unexpected" >&2
  exit 2
fi

rm -rf "$out_dir"
mkdir -p "$out_dir"

cp -a "${dist_dir}/." "$out_dir/"
rm -f "${out_dir}/index.html"
cp "${dist_dir}/index.html" "${out_dir}/app.html"

shoelace_src="apps/web/node_modules/@shoelace-style/shoelace/dist"
shoelace_out="internal/server/assets/shoelace"

if [[ ! -d "${shoelace_src}" ]]; then
  echo "[ui] missing ${shoelace_src}; Shoelace assets not found" >&2
  exit 2
fi

rm -rf "$shoelace_out"
mkdir -p "$shoelace_out"
cp -a "${shoelace_src}/." "$shoelace_out/"

echo "[ui] OK: ${out_dir}/app.html"
echo "[ui] OK: ${shoelace_out}"
