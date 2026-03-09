#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "$0")/common.sh"

require_librechat_web_scaffold

if ! source_imported; then
	printf '[librechat-web][284-prep] source not imported, skip scan\n'
	exit 0
fi

client_src="${librechat_web_source_dir}/client/src"

print_section() {
	local title="${1:?title required}"
	shift
	printf '\n[librechat-web][284-prep] %s\n' "$title"
	printf '[librechat-web][284-prep] ----------------------------------------\n'
	local output
	output="$(rg -n --glob '!**/*.spec.ts' --glob '!**/*.test.ts' --glob '!**/*.spec.tsx' --glob '!**/*.test.tsx' "$@" "$client_src" || true)"
	if [[ -z "${output}" ]]; then
		printf '[librechat-web][284-prep] no hits\n'
		return
	fi
	printf '%s\n' "$output" | awk 'NR<=120 { print }'
	local total
	total="$(printf '%s\n' "$output" | awk 'END { print NR }')"
	if [[ "${total}" -gt 120 ]]; then
		printf '[librechat-web][284-prep] ... truncated (%s total hits)\n' "$total"
	fi
}

printf '[librechat-web][284-prep] client_src=%s\n' "$client_src"

print_section "send-control-points" \
	"onSubmit=\\{methods\\.handleSubmit\\(submitMessage\\)\\}|useSubmitMessage\\(|submitMessage\\(|ask\\("

print_section "sse-store-points" \
	"useSSE\\(|useEventHandlers\\(|setMessages\\(|setQueryData<.*TMessage|QueryKeys\\.messages"

print_section "render-points" \
	"MessageRender|MessageContent|PlaceholderRow|HoverButtons|queryClient\\.setQueryData<.*TMessage|setMessages\\("
