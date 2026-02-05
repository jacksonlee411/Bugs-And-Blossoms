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
cp -a "${shoelace_src}/shoelace.js" "$shoelace_out/"

for dir in assets chunks components internal styles themes translations utilities; do
  if [[ -d "${shoelace_src}/${dir}" ]]; then
    cp -a "${shoelace_src}/${dir}" "$shoelace_out/"
  fi
done

vendor_out="${shoelace_out}/vendor"
pnpm_store="apps/web/node_modules/.pnpm"

copy_vendor_pkg() {
  local pkg="$1"
  local pattern="$2"
  local match=""
  local glob="${pnpm_store}/${pattern}"

  match=$(ls -d ${glob} 2>/dev/null | head -n 1 || true)
  if [[ -z "$match" ]]; then
    echo "[ui] missing ${pkg} (pattern: ${pattern})" >&2
    exit 2
  fi

  local src="${match}/node_modules/${pkg}"
  if [[ ! -d "$src" ]]; then
    echo "[ui] missing ${pkg} at ${src}" >&2
    exit 2
  fi

  local dest="${vendor_out}/${pkg}"
  mkdir -p "$dest"
  cp -a "$src/." "$dest/"
}

rm -rf "$vendor_out"
mkdir -p "$vendor_out"
copy_vendor_pkg "lit" "lit@*"
copy_vendor_pkg "lit-html" "lit-html@*"
copy_vendor_pkg "lit-element" "lit-element@*"
copy_vendor_pkg "@lit/reactive-element" "@lit+reactive-element@*"
copy_vendor_pkg "@lit/react" "@lit+react@*"
copy_vendor_pkg "@shoelace-style/localize" "@shoelace-style+localize@*"
copy_vendor_pkg "@shoelace-style/animations" "@shoelace-style+animations@*"
copy_vendor_pkg "@floating-ui/dom" "@floating-ui+dom@*"
copy_vendor_pkg "@floating-ui/core" "@floating-ui+core@*"
copy_vendor_pkg "@floating-ui/utils" "@floating-ui+utils@*"
copy_vendor_pkg "@ctrl/tinycolor" "@ctrl+tinycolor@*"
copy_vendor_pkg "composed-offset-position" "composed-offset-position@*"
copy_vendor_pkg "qr-creator" "qr-creator@*"

echo "[ui] OK: ${out_dir}/app.html"
echo "[ui] OK: ${shoelace_out}"
