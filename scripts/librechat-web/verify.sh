#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "$0")/common.sh"

require_librechat_web_scaffold

repo="$(metadata_value repo)"
ref="$(metadata_value ref)"
commit="$(metadata_value commit)"
rollback_ref="$(metadata_value rollback_ref)"
imported_at="$(metadata_value imported_at)"

printf '[librechat-web] repo=%s\n' "$repo"
printf '[librechat-web] ref=%s\n' "$ref"
printf '[librechat-web] commit=%s\n' "$commit"
printf '[librechat-web] rollback_ref=%s\n' "$rollback_ref"
printf '[librechat-web] imported_at=%s\n' "$imported_at"
printf '[librechat-web] source_dir=%s\n' "$librechat_web_source_dir"
printf '[librechat-web] patches_dir=%s\n' "$librechat_web_patches_dir"
printf '[librechat-web] output_dir=%s\n' "$librechat_web_output_dir"
printf '[librechat-web] public_base=%s\n' "$librechat_web_public_base"

if source_imported; then
	printf '[librechat-web] state=source_imported\n'
	if [[ ! -f "${librechat_web_source_dir}/client/package.json" ]]; then
		echo '[librechat-web] client/package.json missing under imported source' >&2
		exit 2
	fi
	if [[ ! -f "${librechat_web_source_dir}/package.json" ]]; then
		echo '[librechat-web] package.json missing under imported source' >&2
		exit 2
	fi
	if [[ ! -f "${librechat_web_source_dir}/package-lock.json" ]]; then
		echo '[librechat-web] package-lock.json missing under imported source' >&2
		exit 2
	fi
	printf '[librechat-web] source_check=ok\n'
	exit 0
fi

printf '[librechat-web] state=scaffold_only\n'
printf '[librechat-web] next_step=import upstream source into third_party/librechat-web/source\n'
