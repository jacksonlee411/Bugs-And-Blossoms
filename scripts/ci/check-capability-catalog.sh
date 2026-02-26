#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

route_map_file="config/capability/route-capability-map.v1.json"

if [[ ! -f "$route_map_file" ]]; then
  echo "[capability-catalog] FAIL: missing $route_map_file" >&2
  exit 1
fi

python3 - "$route_map_file" <<'PY'
import json
import re
import sys

route_map_path = sys.argv[1]
with open(route_map_path, "r", encoding="utf-8") as f:
    route_map = json.load(f)


def fail(msg: str) -> None:
    raise SystemExit(f"[capability-catalog] FAIL: {msg}")


capability_pattern = re.compile(r"^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$")
capabilities = route_map.get("capabilities", [])
if not isinstance(capabilities, list) or len(capabilities) == 0:
    fail("capabilities must be a non-empty list")

keys = set()
for item in capabilities:
    key = str(item.get("capability_key", "")).strip()
    owner_module = str(item.get("owner_module", "")).strip()
    if not capability_pattern.match(key):
        fail(f"invalid capability_key: {key!r}")
    if key in keys:
        fail(f"duplicate capability_key: {key}")
    keys.add(key)
    if owner_module == "":
        fail(f"owner_module required for capability_key={key}")

print("[capability-catalog] route-capability map shape OK")
PY

echo "[capability-catalog] verify server catalog contract"
go test ./internal/server -run '^TestCapabilityCatalog|TestCapabilityRouteRegistryContract$' -count=1

echo "[capability-catalog] OK"
