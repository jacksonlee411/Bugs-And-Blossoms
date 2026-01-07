#!/usr/bin/env bash
set -euo pipefail

policy="config/coverage/policy.yaml"
if [[ ! -f "$policy" ]]; then
  echo "[coverage] missing policy: $policy" >&2
  exit 1
fi

threshold="$(grep -E '^threshold_percent:' "$policy" | head -n1 | awk '{print $2}')"
if [[ -z "${threshold:-}" ]]; then
  echo "[coverage] missing threshold_percent in $policy" >&2
  exit 1
fi

mapfile -t excludes < <(awk '/^exclude_package_prefixes:/{flag=1;next} flag && $1=="-"{print $2}' "$policy")

all_pkgs="$(go list -buildvcs=false ./...)"
cover_pkgs="$all_pkgs"
for pfx in "${excludes[@]:-}"; do
  cover_pkgs="$(echo "$cover_pkgs" | grep -vE "^${pfx//\//\\/}" || true)"
done

coverpkg_csv="$(echo "$cover_pkgs" | paste -sd, -)"
mkdir -p coverage

echo "[coverage] running go test with coverpkg policy"
go test -count=1 -buildvcs=false -covermode=atomic -coverpkg="$coverpkg_csv" -coverprofile=coverage/coverage.out ./...

total="$(go tool cover -func=coverage/coverage.out | awk '/^total:/{gsub(/%/,"",$3);print $3}')"
if [[ -z "${total:-}" ]]; then
  echo "[coverage] failed to read total coverage" >&2
  exit 1
fi

python3 - <<PY
threshold=float("${threshold}")
total=float("${total}")
if total + 1e-9 < threshold:
    raise SystemExit(f"[coverage] FAIL: total {total:.2f}% < threshold {threshold:.2f}%")
print(f"[coverage] OK: total {total:.2f}% >= threshold {threshold:.2f}%")
PY
