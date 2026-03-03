#!/usr/bin/env bash
set -euo pipefail

prefix="[librechat-runtime]"
repo_root="$(git rev-parse --show-toplevel)"
runtime_dir="${repo_root}/deploy/librechat"
status_file="${ASSISTANT_RUNTIME_STATUS_FILE:-${runtime_dir}/runtime-status.json}"
compose_project="${LIBRECHAT_COMPOSE_PROJECT:-bugs-and-blossoms-librechat}"
env_file="${LIBRECHAT_ENV_FILE:-${runtime_dir}/.env}"
if [[ ! -f "${env_file}" ]]; then
  env_file="${runtime_dir}/.env.example"
fi

upstream="${LIBRECHAT_UPSTREAM:-http://127.0.0.1:${LIBRECHAT_PORT:-3080}}"
checked_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
services=(api mongodb meilisearch rag_api vectordb)

running_services=""
if command -v docker >/dev/null 2>&1; then
  compose_cmd=(docker compose -p "${compose_project}" --env-file "${env_file}" -f "${runtime_dir}/docker-compose.upstream.yaml" -f "${runtime_dir}/docker-compose.overlay.yaml")
  running_services="$("${compose_cmd[@]}" ps --services --status running 2>/dev/null || true)"
fi

status="healthy"
services_json="[]"
for service in "${services[@]}"; do
  healthy="healthy"
  reason=""
  if [[ -z "${running_services}" ]] || ! grep -qx "${service}" <<<"${running_services}"; then
    healthy="unavailable"
    reason="container_not_running"
  fi

  if [[ "${service}" == "api" && "${healthy}" == "healthy" ]]; then
    if ! curl -fsS --max-time 3 "${upstream%/}/" >/dev/null 2>&1; then
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
