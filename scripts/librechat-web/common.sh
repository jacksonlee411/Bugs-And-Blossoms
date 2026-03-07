#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
librechat_web_root="${repo_root}/third_party/librechat-web"
librechat_web_source_dir="${librechat_web_root}/source"
librechat_web_patches_dir="${librechat_web_root}/patches"
librechat_web_metadata_file="${librechat_web_root}/UPSTREAM.yaml"
librechat_web_output_dir="${repo_root}/internal/server/assets/librechat-web"
librechat_web_public_base="/assets/librechat-web/"

require_librechat_web_scaffold() {
	local missing=()
	[[ -d "${librechat_web_root}" ]] || missing+=("third_party/librechat-web")
	[[ -f "${librechat_web_metadata_file}" ]] || missing+=("third_party/librechat-web/UPSTREAM.yaml")
	[[ -d "${librechat_web_patches_dir}" ]] || missing+=("third_party/librechat-web/patches")
	[[ -f "${librechat_web_patches_dir}/series" ]] || missing+=("third_party/librechat-web/patches/series")
	[[ -d "${librechat_web_source_dir}" ]] || missing+=("third_party/librechat-web/source")
	if (( ${#missing[@]} > 0 )); then
		printf '[librechat-web] scaffold missing: %s\n' "${missing[*]}" >&2
		exit 2
	fi
}

metadata_value() {
	local key="${1:?key required}"
	python3 - "${librechat_web_metadata_file}" "${key}" <<'PY'
from pathlib import Path
import re
import sys

path = Path(sys.argv[1])
key = sys.argv[2]
text = path.read_text(encoding="utf-8")
pattern = re.compile(rf"(?m)^\s*{re.escape(key)}\s*:\s*(.+?)\s*$")
match = pattern.search(text)
if not match:
    sys.exit(1)
value = match.group(1).strip()
if value.startswith(('"', "'")) and value.endswith(('"', "'")) and len(value) >= 2:
    value = value[1:-1]
print(value)
PY
}

source_imported() {
	[[ -f "${librechat_web_source_dir}/package.json" ]] && [[ -d "${librechat_web_source_dir}/client" ]]
}

prepare_build_dir() {
	local build_dir="${1:?build_dir required}"
	rm -rf "${build_dir}"
	mkdir -p "${build_dir}"
	cp -a "${librechat_web_source_dir}/." "${build_dir}/"
	find "${build_dir}" -type d \( -name node_modules -o -name dist \) -prune -exec rm -rf {} +
}

apply_patch_stack() {
	local build_dir="${1:?build_dir required}"
	while IFS= read -r patch_name || [[ -n "$patch_name" ]]; do
		patch_name="${patch_name%%#*}"
		patch_name="$(printf '%s' "$patch_name" | xargs)"
		[[ -n "$patch_name" ]] || continue
		local patch_path="${librechat_web_patches_dir}/${patch_name}"
		if [[ ! -f "$patch_path" ]]; then
			printf '[librechat-web] patch missing: %s\n' "$patch_path" >&2
			exit 2
		fi
		printf '[librechat-web] apply_patch=%s\n' "$patch_name"
		patch -d "$build_dir" -p1 <"$patch_path"
	done <"${librechat_web_patches_dir}/series"
}
