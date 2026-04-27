#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

runtime_dir="${DEV_RUNTIME_DIR:-.local/runtime}"

process_group_has_processes() {
  local pgid="${1:?}"
  ps -eo pgid= | awk -v pgid="$pgid" '$1 == pgid { found=1; exit } END { exit found ? 0 : 1 }'
}

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

  # start_dev_runtime.sh uses setsid, so the recorded pid is also the process-group id.
  local pgid="$pid"
  if kill -0 "$pid" >/dev/null 2>&1 || process_group_has_processes "$pgid"; then
    printf '[dev-runtime] stop %s: pid=%s pgid=%s\n' "$name" "$pid" "$pgid"
    kill -TERM -- "-${pgid}" >/dev/null 2>&1 || kill "$pid" >/dev/null 2>&1 || true
    for _ in $(seq 1 40); do
      if ! process_group_has_processes "$pgid"; then
        return 0
      fi
      sleep 0.25
    done
    printf '[dev-runtime] force stop %s: pgid=%s\n' "$name" "$pgid"
    kill -KILL -- "-${pgid}" >/dev/null 2>&1 || kill -KILL "$pid" >/dev/null 2>&1 || true
    return 0
  else
    printf '[dev-runtime] %s already stopped: pid=%s\n' "$name" "$pid"
  fi
}

stop_pid_file "server" "${runtime_dir}/dev-server.pid"
stop_pid_file "kratos stub" "${runtime_dir}/dev-kratosstub.pid"
stop_pid_file "superadmin" "${runtime_dir}/dev-superadmin.pid"

for root_pid_file in .dev-server.pid .dev-kratosstub.pid .dev-superadmin.pid .dev-web.pid; do
  [[ -f "$root_pid_file" ]] || continue
  stop_pid_file "root pid file ${root_pid_file}" "$root_pid_file"
done

if [[ "${1:-}" == "--down" ]]; then
  DEV_INFRA_ENV_FILE="${DEV_INFRA_ENV_FILE:-.env.example}" make dev-down
fi
