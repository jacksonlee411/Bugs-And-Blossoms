#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

runtime_dir="${DEV_RUNTIME_DIR:-.local/runtime}"

stop_pid_file() {
  local name="${1:?}"
  local pid_file="${2:?}"
  if [[ ! -f "$pid_file" ]]; then
    printf '[dev-runtime] %s not started by pid file\n' "$name"
    return 0
  fi
  local pid
  pid="$(tr -d '[:space:]' <"$pid_file" || true)"
  rm -f "$pid_file"
  if [[ -z "$pid" ]]; then
    printf '[dev-runtime] %s pid file was empty\n' "$name"
    return 0
  fi
  if kill -0 "$pid" >/dev/null 2>&1; then
    printf '[dev-runtime] stop %s: pid=%s\n' "$name" "$pid"
    kill "$pid" >/dev/null 2>&1 || true
    wait "$pid" >/dev/null 2>&1 || true
  else
    printf '[dev-runtime] %s already stopped: pid=%s\n' "$name" "$pid"
  fi
}

stop_pid_file "server" "${runtime_dir}/dev-server.pid"
stop_pid_file "kratos stub" "${runtime_dir}/dev-kratosstub.pid"
stop_pid_file "superadmin" "${runtime_dir}/dev-superadmin.pid"

for legacy_pid_file in .dev-server.pid .dev-kratosstub.pid .dev-superadmin.pid .dev-web.pid; do
  [[ -f "$legacy_pid_file" ]] || continue
  stop_pid_file "legacy ${legacy_pid_file}" "$legacy_pid_file"
done

if [[ "${1:-}" == "--down" ]]; then
  DEV_INFRA_ENV_FILE="${DEV_INFRA_ENV_FILE:-.env.example}" make dev-down
fi
