#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "$0")/common.sh"

require_librechat_web_scaffold

if ! command -v node >/dev/null 2>&1; then
	echo '[librechat-web] missing node; install Node before building vendored UI' >&2
	exit 2
fi

if ! command -v npm >/dev/null 2>&1; then
	echo '[librechat-web] missing npm; install npm before building vendored UI' >&2
	exit 2
fi

if ! source_imported; then
	echo '[librechat-web] vendored source has not been imported yet' >&2
	echo '[librechat-web] expected third_party/librechat-web/source/package.json and client/ to exist' >&2
	exit 2
fi

build_dir="$(mktemp -d)"
trap 'rm -rf "$build_dir"' EXIT

prepare_build_dir "$build_dir"
apply_patch_stack "$build_dir"

cd "$build_dir"
export MONGOMS_DISABLE_POSTINSTALL=1
npm ci --ignore-scripts --no-audit --no-fund
npm run build:data-provider
npm run build:client-package
npm --workspace client run build -- --base "$librechat_web_public_base"

rm -rf "$librechat_web_output_dir"
mkdir -p "$librechat_web_output_dir"
cp -a "$build_dir/client/dist/." "$librechat_web_output_dir/"

printf '[librechat-web] built=%s\n' "$librechat_web_output_dir/index.html"
