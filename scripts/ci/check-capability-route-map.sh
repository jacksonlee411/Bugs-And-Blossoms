#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

route_map_file="config/capability/route-capability-map.v1.json"
contract_file="config/capability/contract-freeze.v1.json"
allowlist_file="config/routing/allowlist.yaml"

if [[ ! -f "$route_map_file" ]]; then
  echo "[capability-route-map] FAIL: missing $route_map_file" >&2
  exit 1
fi

python3 - "$route_map_file" "$contract_file" "$allowlist_file" <<'PY'
import json
import re
import sys

route_map_path, contract_path, allowlist_path = sys.argv[1], sys.argv[2], sys.argv[3]

with open(route_map_path, "r", encoding="utf-8") as f:
    route_map = json.load(f)
with open(contract_path, "r", encoding="utf-8") as f:
    contract = json.load(f)
with open(allowlist_path, "r", encoding="utf-8") as f:
    allowlist_text = f.read()


def fail(msg: str) -> None:
    raise SystemExit(f"[capability-route-map] FAIL: {msg}")


capability_pattern = re.compile(r"^[a-z][a-z0-9_]*(\.[a-z0-9_]+)+$")
allowed_methods = {"GET", "POST", "PUT", "PATCH", "DELETE"}
allowed_actions = {"read", "admin"}
allowed_status = {"active", "reserved", "deprecated"}

areas = {}
for area in contract.get("functional_areas", []):
    key = area.get("functional_area_key", "")
    areas[key] = area.get("lifecycle_status", "")

capabilities = route_map.get("capabilities", [])
if not isinstance(capabilities, list) or len(capabilities) == 0:
    fail("capabilities must be a non-empty list")

capability_keys = set()
for item in capabilities:
    key = item.get("capability_key", "")
    area = item.get("functional_area_key", "")
    cap_type = item.get("capability_type", "")
    status = item.get("status", "")
    if not capability_pattern.match(key):
        fail(f"invalid capability_key: {key!r}")
    if key in capability_keys:
        fail(f"duplicate capability_key: {key}")
    capability_keys.add(key)
    if area not in areas:
        fail(f"functional_area_key not found in contract-freeze: {area!r}")
    if areas[area] != "active":
        fail(f"functional_area_key must be active for route mapping: {area!r}")
    if cap_type not in {"domain_capability", "process_capability"}:
        fail(f"invalid capability_type for {key}: {cap_type!r}")
    if status not in allowed_status:
        fail(f"invalid capability status for {key}: {status!r}")

routes = route_map.get("routes", [])
if not isinstance(routes, list) or len(routes) == 0:
    fail("routes must be a non-empty list")

allowlist_routes = set()
current = None
for line in allowlist_text.splitlines():
    path_match = re.match(r"^\s*-\s+path:\s*(\S+)\s*$", line)
    if path_match:
        if current and current.get("route_class") == "internal_api":
            for method in current.get("methods", []):
                allowlist_routes.add((method, current["path"]))
        current = {"path": path_match.group(1), "methods": [], "route_class": ""}
        continue
    if current is None:
        continue
    methods_match = re.match(r"^\s+methods:\s*\[(.*)\]\s*$", line)
    if methods_match:
        current["methods"] = [m.strip() for m in methods_match.group(1).split(",") if m.strip()]
        continue
    class_match = re.match(r"^\s+route_class:\s*(\S+)\s*$", line)
    if class_match:
        current["route_class"] = class_match.group(1)
        continue
if current and current.get("route_class") == "internal_api":
    for method in current.get("methods", []):
        allowlist_routes.add((method, current["path"]))

route_keys = set()
for route in routes:
    method = route.get("method", "").upper()
    path = route.get("path", "")
    route_class = route.get("route_class", "")
    action = route.get("action", "")
    capability_key = route.get("capability_key", "")
    status = route.get("status", "")
    if method not in allowed_methods:
        fail(f"invalid method for route {path!r}: {method!r}")
    if not path.startswith("/"):
        fail(f"invalid path for route mapping: {path!r}")
    if route_class != "internal_api":
        fail(f"route_class must be internal_api for {method} {path}")
    if action not in allowed_actions:
        fail(f"invalid action for {method} {path}: {action!r}")
    if capability_key not in capability_keys:
        fail(f"route {method} {path} references unknown capability_key: {capability_key!r}")
    if status != "active":
        fail(f"route status must be active for {method} {path}")
    key = (method, path)
    if key in route_keys:
        fail(f"duplicate route mapping: {method} {path}")
    route_keys.add(key)
    if key not in allowlist_routes:
        fail(f"route mapping not found in allowlist internal_api entries: {method} {path}")

print("[capability-route-map] contract file OK")
PY

echo "[capability-route-map] verify go registry sync"
go test ./internal/server -run '^TestCapabilityRouteRegistryContract$' -count=1

echo "[capability-route-map] OK"
