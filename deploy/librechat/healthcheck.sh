#!/usr/bin/env bash
set -euo pipefail

prefix="[librechat-runtime]"
repo_root="$(git rev-parse --show-toplevel)"
# shellcheck disable=SC1091
source "${repo_root}/scripts/librechat/common.sh"
librechat_init "${prefix}"

status_file="${ASSISTANT_RUNTIME_STATUS_FILE:-${LIBRECHAT_RUNTIME_DIR}/runtime-status.json}"
librechat_require_cmd docker
librechat_require_cmd jq
librechat_require_cmd curl

upstream="${LIBRECHAT_UPSTREAM:-http://127.0.0.1:${LIBRECHAT_PORT:-3080}}"
checked_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
running_services="$(librechat_running_services)"
api_probe_timeout_seconds="${LIBRECHAT_API_PROBE_TIMEOUT_SECONDS:-60}"
api_probe_interval_seconds="${LIBRECHAT_API_PROBE_INTERVAL_SECONDS:-2}"

status="healthy"
services_json="[]"
for service in "${LIBRECHAT_SERVICES[@]}"; do
  healthy="healthy"
  reason=""
  if [[ -z "${running_services}" ]] || ! grep -qx "${service}" <<<"${running_services}"; then
    healthy="unavailable"
    data_dir="$(librechat_service_data_dir "${service}")"
    if [[ ! -d "${data_dir}" ]]; then
      reason="mount_source_missing"
    else
      reason="container_not_running"
    fi
  fi

  if [[ "${service}" == "api" && "${healthy}" == "healthy" ]]; then
    deadline=$((SECONDS + api_probe_timeout_seconds))
    api_reachable="0"
    while (( SECONDS < deadline )); do
      if curl -fsS --max-time 3 "${upstream%/}/" >/dev/null 2>&1; then
        api_reachable="1"
        break
      fi
      sleep "${api_probe_interval_seconds}"
    done
    if [[ "${api_reachable}" != "1" ]]; then
      healthy="unavailable"
      reason="upstream_unreachable"
    fi
  fi

  if [[ "${healthy}" != "healthy" ]]; then
    status="unavailable"
  fi

  row="$(jq -cn \
    --arg name "${service}" \
    --arg healthy "${healthy}" \
    --arg reason "${reason}" \
    '{name:$name,required:true,healthy:$healthy,reason:$reason}')"
  services_json="$(jq -cn --argjson current "${services_json}" --argjson row "${row}" '$current + [$row]')"
done

payload="$(jq -cn \
  --arg status "${status}" \
  --arg checked_at "${checked_at}" \
  --arg upstream "${upstream}" \
  --argjson services "${services_json}" \
  '{status:$status,checked_at:$checked_at,upstream:{url:$upstream},services:$services}')"

mkdir -p "$(dirname "${status_file}")"
printf '%s\n' "${payload}" >"${status_file}"

if [[ "${status}" == "healthy" ]]; then
  echo "${prefix} OK: status=${status} file=${status_file}"
  exit 0
fi

echo "${prefix} FAIL: status=${status} file=${status_file}" >&2
exit 1
