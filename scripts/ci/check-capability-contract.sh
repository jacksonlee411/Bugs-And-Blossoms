#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

contract_file="config/capability/contract-freeze.v1.json"
workflow_file=".github/workflows/quality-gates.yml"

if [[ ! -f "$contract_file" ]]; then
  echo "[capability-contract] FAIL: missing $contract_file" >&2
  exit 1
fi

python3 - "$contract_file" <<'PY'
import json
import re
import sys

path = sys.argv[1]
with open(path, "r", encoding="utf-8") as f:
    data = json.load(f)


def fail(msg: str) -> None:
    raise SystemExit(f"[capability-contract] FAIL: {msg}")


def require_subset(actual, required, name: str) -> None:
    missing = sorted(required - set(actual))
    if missing:
        fail(f"{name} missing required entries: {', '.join(missing)}")


if data.get("replacement_matrix") != {"scope_code": "capability_key", "package_id": "setid"}:
    fail("replacement_matrix must be exactly scope_code->capability_key and package_id->setid")

capability_types = set(data.get("capability_types", []))
if capability_types != {"domain_capability", "process_capability"}:
    fail("capability_types must be exactly domain_capability/process_capability")

context = data.get("context_contract", {})
for ctx_name in ("StaticContext", "ProcessContext"):
    fields = context.get(ctx_name)
    if not isinstance(fields, list) or len(fields) == 0:
        fail(f"{ctx_name} must be a non-empty list")

areas = data.get("functional_areas", [])
if not isinstance(areas, list) or len(areas) == 0:
    fail("functional_areas must be a non-empty list")

allowed_status = {"active", "reserved", "deprecated"}
allowed_status_list = set(data.get("lifecycle_statuses", []))
if not allowed_status.issubset(allowed_status_list):
    fail("lifecycle_statuses must include active/reserved/deprecated")

area_keys = []
for area in areas:
    key = area.get("functional_area_key", "")
    status = area.get("lifecycle_status", "")
    if not re.match(r"^[a-z][a-z0-9_]*$", key):
        fail(f"invalid functional_area_key: {key!r}")
    if status not in allowed_status:
        fail(f"invalid lifecycle_status {status!r} for functional_area_key={key!r}")
    area_keys.append(key)

if len(set(area_keys)) != len(area_keys):
    fail("functional_areas contains duplicate functional_area_key")

require_subset(
    area_keys,
    {"org_foundation", "staffing", "jobcatalog", "person", "iam_platform", "compensation", "benefits"},
    "functional_areas",
)

reserved_keys = {a["functional_area_key"] for a in areas if a.get("lifecycle_status") == "reserved"}
require_subset(reserved_keys, {"compensation", "benefits"}, "reserved functional_areas")

require_subset(
    data.get("reason_codes", []),
    {
        "CAPABILITY_CONTEXT_REQUIRED",
        "CAPABILITY_CONTEXT_MISMATCH",
        "FUNCTIONAL_AREA_MISSING",
        "FUNCTIONAL_AREA_DISABLED",
        "FUNCTIONAL_AREA_NOT_ACTIVE",
    },
    "reason_codes",
)

require_subset(
    data.get("explain_min_fields", []),
    {
        "trace_id",
        "request_id",
        "capability_key",
        "setid",
        "functional_area_key",
        "policy_version",
        "decision",
        "reason_code",
    },
    "explain_min_fields",
)

baseline = data.get("gate_baseline", {})
require_subset(
    baseline.get("make_checks", []),
    {
        "check no-legacy",
        "check no-scope-package",
        "check capability-key",
        "check capability-contract",
        "check request-code",
        "check go-version",
        "check error-message",
        "check doc",
        "check routing",
    },
    "gate_baseline.make_checks",
)
require_subset(
    baseline.get("required_checks", []),
    {
        "Code Quality & Formatting",
        "Unit & Integration Tests",
        "Routing Gates",
        "E2E Tests",
    },
    "gate_baseline.required_checks",
)
print("[capability-contract] contract file OK")
PY

required_workflow_cmds=(
  "make check no-scope-package"
  "make check capability-key"
  "make check capability-contract"
)

for cmd in "${required_workflow_cmds[@]}"; do
  if ! grep -Fq "$cmd" "$workflow_file"; then
    echo "[capability-contract] FAIL: $workflow_file missing '$cmd'" >&2
    exit 1
  fi
done

echo "[capability-contract] workflow baseline OK"
