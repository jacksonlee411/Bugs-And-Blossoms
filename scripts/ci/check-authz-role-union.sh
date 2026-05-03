#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

prefix="[authz-role-union]"

echo "$prefix scan: block ordinary tenant authz fallback to single role or policy CSV"

failures=()

require_file() {
  local file="${1:?}"
  if [[ ! -f "$file" ]]; then
    failures+=("missing expected file: $file")
  fi
}

check_no_hits() {
  local desc="${1:?}"
  local pattern="${2:?}"
  shift 2
  local hits=""
  if command -v rg >/dev/null 2>&1; then
    hits="$(rg -n --pcre2 "$pattern" "$@" || true)"
  else
    hits="$(grep -RInE -- "$pattern" "$@" || true)"
  fi
  if [[ -n "$hits" ]]; then
    failures+=("$desc"$'\n'"$hits")
  fi
}

require_file "internal/server/authz_middleware.go"
require_file "internal/server/authz_runtime_store.go"
require_file "internal/server/session_capabilities_api.go"
require_file "internal/server/cubebox_api.go"
require_file "config/access/policy.csv"

mapfile -t source_files < <(
  find apps/web/src internal/server modules pkg config scripts \
    -type f \( -name '*.go' -o -name '*.ts' -o -name '*.tsx' -o -name '*.js' -o -name '*.yaml' -o -name '*.yml' -o -name '*.json' -o -name '*.sh' \) \
    ! -name '*_templ.go' \
    ! -name '*_test.go' \
    ! -name '*.test.ts' \
    ! -name '*.test.tsx' \
    ! -path 'internal/server/assets/web/assets/*' \
    ! -path 'modules/*/infrastructure/sqlc/gen/*' \
    ! -path 'config/capability/contract-freeze*.json' \
    -print
)

if [[ ${#source_files[@]} -gt 0 ]]; then
  check_no_hits \
    "found current-role marker; 489A forbids current/primary/default/active role selection" \
    '\b(current|primary|default|active)_?role_?slug\b|\b(current|primary|default|active)RoleSlug\b' \
    "${source_files[@]}"

  check_no_hits \
    "found first-role indexing; 489A requires full principal role union" \
    '\broles\s*\[\s*0\s*\]|\broleSlugs\s*\[\s*0\s*\]|\bassignedRoleSlugs\s*\[\s*0\s*\]|\brole_slugs\s*\[\s*0\s*\]' \
    "${source_files[@]}"
fi

mapfile -t policy_files < <(
  {
    find config/access/policies -type f -name '*.csv' -print 2>/dev/null || true
    printf '%s\n' "config/access/policy.csv"
  } | sort -u
)

if [[ ${#policy_files[@]} -gt 0 ]]; then
  check_no_hits \
    "found ordinary tenant role grant in policy CSV; role capability grants must come from DB SoT" \
    '^p,\s*role:tenant-(admin|viewer)\s*,' \
    "${policy_files[@]}"
fi

python3 - <<'PY'
import pathlib
import re
import sys

failures = []

def read(path: str) -> str:
    p = pathlib.Path(path)
    if not p.exists():
        failures.append(f"missing expected file: {path}")
        return ""
    return p.read_text(encoding="utf-8")

middleware = read("internal/server/authz_middleware.go")
if middleware:
    if "runtime.AuthorizePrincipal" not in middleware:
        failures.append("withAuthz must authorize ordinary tenant routes through runtime.AuthorizePrincipal")
    authorize_calls = re.findall(r"\ba\.Authorize\s*\(", middleware)
    if len(authorize_calls) != 1:
        failures.append(f"withAuthz must keep exactly one Casbin call for bootstrap routes; found {len(authorize_calls)} a.Authorize calls")
    if "a.Authorize(authz.SubjectFromRoleSlug(authz.RoleAnonymous)" not in middleware:
        failures.append("the remaining withAuthz Casbin call must be anonymous session bootstrap only")
    tenant_role_subject = re.search(r"SubjectFromRoleSlug\(authz\.RoleTenant(Admin|Viewer)\)", middleware)
    if tenant_role_subject:
        failures.append("withAuthz must not authorize ordinary tenant routes via tenant-admin/viewer policy subjects")

for path in ["internal/server/session_capabilities_api.go", "internal/server/cubebox_api.go"]:
    text = read(path)
    if not text:
        continue
    if "CapabilitiesForPrincipal" not in text:
        failures.append(f"{path} must derive exposed capabilities from authzRuntimeStore.CapabilitiesForPrincipal")
    if re.search(r"\.Authorize\s*\(", text) or "SubjectFromRoleSlug" in text:
        failures.append(f"{path} must not derive ordinary tenant capabilities from policy/Casbin subjects")

runtime = read("internal/server/authz_runtime_store.go")
if runtime:
    def function_body(name: str) -> str:
        start = runtime.find(f"func {name}(")
        if start == -1:
            return ""
        end = runtime.find("\nfunc ", start + 1)
        if end == -1:
            end = len(runtime)
        return runtime[start:end]

    body = function_body("capabilityKeysForPrincipalTx")
    if not body:
        failures.append("missing capabilityKeysForPrincipalTx runtime union function")
    else:
        if "principal_role_assignments" not in body:
            failures.append("capabilityKeysForPrincipalTx must read principal_role_assignments")
        if "SELECT DISTINCT" not in body:
            failures.append("capabilityKeysForPrincipalTx must DISTINCT-union capabilities across all assigned roles")
        if re.search(r"\bLIMIT\s+1\b", body, flags=re.I):
            failures.append("capabilityKeysForPrincipalTx must not select a single assigned role")
        if "iam.principals" in body:
            failures.append("capabilityKeysForPrincipalTx must not read iam.principals.role_slug")

    start = runtime.find("func (s *pgAuthzRuntimeStore) OrgScopesForPrincipal(")
    if start == -1:
        method_body = ""
    else:
        end = runtime.find("\nfunc ", start + 1)
        if end == -1:
            end = len(runtime)
        method_body = runtime[start:end]
    helper_body = function_body("orgScopesForPrincipalTx")
    scope_body = method_body + "\n" + helper_body
    if not method_body or not helper_body:
        failures.append("missing OrgScopesForPrincipal runtime scope function")
    else:
        if "principal_org_scope_bindings" not in scope_body:
            failures.append("OrgScopesForPrincipal must read principal_org_scope_bindings")
        if re.search(r"\bLIMIT\s+1\b", scope_body, flags=re.I):
            failures.append("OrgScopesForPrincipal must not select a single org scope")

if failures:
    print("[authz-role-union] FAIL", file=sys.stderr)
    for failure in failures:
        print(f"  - {failure}", file=sys.stderr)
    raise SystemExit(1)
PY

if [[ ${#failures[@]} -gt 0 ]]; then
  echo "$prefix FAIL" >&2
  printf '  - %s\n' "${failures[@]}" >&2
  exit 1
fi

echo "$prefix OK"
