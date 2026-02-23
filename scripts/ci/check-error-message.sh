#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$root"

echo "[error-message] go test ./internal/routing"
go test ./internal/routing -run "TestWriteError_(RewritesGenericMessageFromCode|HumanizesUnknownGenericCode|KeepExplicitMessage)" -count=1

resolver_file="apps/web/src/errors/presentApiError.ts"
catalog_file="config/errors/catalog.yaml"
backend_file="internal/routing/responder.go"
if [[ ! -f "$resolver_file" ]]; then
  echo "[error-message] missing resolver file: $resolver_file" >&2
  exit 1
fi
if [[ ! -f "$catalog_file" ]]; then
  echo "[error-message] missing catalog file: $catalog_file" >&2
  exit 1
fi
if [[ ! -f "$backend_file" ]]; then
  echo "[error-message] missing backend resolver file: $backend_file" >&2
  exit 1
fi

if command -v rg >/dev/null 2>&1; then
  has_resolver() { rg -q "export function resolveApiErrorMessage" "$resolver_file"; }
  has_empty_i18n() { rg -n "en: ''|zh: ''" "$resolver_file" >/dev/null; }
else
  has_resolver() { grep -q "export function resolveApiErrorMessage" "$resolver_file"; }
  has_empty_i18n() { grep -nE "en: ''|zh: ''" "$resolver_file" >/dev/null; }
fi

if ! has_resolver; then
  echo "[error-message] resolveApiErrorMessage not found in $resolver_file" >&2
  exit 1
fi

if has_empty_i18n; then
  echo "[error-message] localized message contains empty en/zh value" >&2
  exit 1
fi

python3 - <<'PY'
import pathlib
import re

catalog_path = pathlib.Path("config/errors/catalog.yaml")
resolver_path = pathlib.Path("apps/web/src/errors/presentApiError.ts")
backend_path = pathlib.Path("internal/routing/responder.go")

records = []
current = None
for raw in catalog_path.read_text(encoding="utf-8").splitlines():
    line = raw.strip()
    if not line or line.startswith("#") or line in {"errors:", "version: 1"}:
        continue
    if line.startswith("- code:"):
        if current is not None:
            records.append(current)
        current = {"code": line.split(":", 1)[1].strip()}
        continue
    if current is not None and ":" in line:
        key, value = line.split(":", 1)
        current[key.strip()] = value.strip()
if current is not None:
    records.append(current)

if not records:
    raise SystemExit("[error-message] catalog has no records")

required_fields = {"code", "module", "http_status", "severity", "user_message_key", "backend_policy", "frontend_policy"}
backend_policies = {"mapped", "passthrough"}
frontend_policies = {"mapped", "passthrough"}
codes = []
for rec in records:
    missing = [field for field in required_fields if not rec.get(field)]
    if missing:
        raise SystemExit(f"[error-message] catalog record missing fields: code={rec.get('code')} missing={missing}")
    code = rec["code"]
    codes.append(code)
    if rec["backend_policy"] not in backend_policies:
        raise SystemExit(f"[error-message] invalid backend_policy: code={code} value={rec['backend_policy']}")
    if rec["frontend_policy"] not in frontend_policies:
        raise SystemExit(f"[error-message] invalid frontend_policy: code={code} value={rec['frontend_policy']}")
    if rec["backend_policy"] == "passthrough" and rec["frontend_policy"] == "passthrough":
        raise SystemExit(f"[error-message] code={code} cannot bypass both backend and frontend mapping")

if len(codes) != len(set(codes)):
    raise SystemExit("[error-message] duplicated code in catalog")

resolver_text = resolver_path.read_text(encoding="utf-8")
entry_pattern = re.compile(r"^\s*([A-Za-z0-9_]+):\s*\{\s*en:\s*'([^']*)',\s*zh:\s*'([^']*)'\s*\},?\s*$")
frontend_entries = {}
for line in resolver_text.splitlines():
    match = entry_pattern.match(line)
    if match:
        frontend_entries[match.group(1)] = {"en": match.group(2), "zh": match.group(3)}

if not frontend_entries:
    raise SystemExit("[error-message] frontend localized messages not found")

backend_text = backend_path.read_text(encoding="utf-8")
known_start = backend_text.find("func knownErrorMessage(code string) string {")
if known_start < 0:
    raise SystemExit("[error-message] knownErrorMessage not found")
humanize_start = backend_text.find("func humanizeErrorCode(code string) string {", known_start)
if humanize_start < 0:
    raise SystemExit("[error-message] humanizeErrorCode not found")
known_body = backend_text[known_start:humanize_start]
backend_codes = set(re.findall(r'case\s+"([^"]+)":', known_body))

catalog_frontend_mapped = {rec["code"] for rec in records if rec["frontend_policy"] == "mapped"}
catalog_backend_mapped = {rec["code"] for rec in records if rec["backend_policy"] == "mapped"}

missing_frontend = sorted(code for code in catalog_frontend_mapped if code not in frontend_entries)
if missing_frontend:
    raise SystemExit(f"[error-message] catalog frontend mapped codes missing in resolver: {missing_frontend}")

missing_backend = sorted(code for code in catalog_backend_mapped if code not in backend_codes)
if missing_backend:
    raise SystemExit(f"[error-message] catalog backend mapped codes missing in knownErrorMessage: {missing_backend}")

extra_frontend = sorted(code for code in frontend_entries if code not in catalog_frontend_mapped)
if extra_frontend:
    raise SystemExit(f"[error-message] resolver has unmapped codes not in catalog(frontend mapped): {extra_frontend}")

extra_backend = sorted(code for code in backend_codes if code not in catalog_backend_mapped)
if extra_backend:
    raise SystemExit(f"[error-message] knownErrorMessage has unmapped codes not in catalog(backend mapped): {extra_backend}")

for code in sorted(catalog_frontend_mapped):
    entry = frontend_entries[code]
    en = entry["en"].strip()
    zh = entry["zh"].strip()
    if not en or not zh:
        raise SystemExit(f"[error-message] empty en/zh message for code={code}")
    if "_failed" in en.lower() or "_failed" in zh.lower():
        raise SystemExit(f"[error-message] generic failed token is not allowed in explicit message: code={code}")

print("[error-message] catalog consistency OK")
PY

pnpm_cmd=()
if command -v pnpm >/dev/null 2>&1; then
  pnpm_cmd=(pnpm)
elif command -v corepack >/dev/null 2>&1; then
  pnpm_cmd=(corepack pnpm)
else
  echo "[error-message] pnpm is required (please enable corepack or install pnpm)." >&2
  exit 1
fi

echo "[error-message] ${pnpm_cmd[*]} -C apps/web test -- src/errors/presentApiError.test.ts"
if [[ ! -x "apps/web/node_modules/.bin/vitest" ]]; then
  echo "[error-message] ${pnpm_cmd[*]} -C apps/web install --frozen-lockfile"
  "${pnpm_cmd[@]}" -C apps/web install --frozen-lockfile
fi

"${pnpm_cmd[@]}" -C apps/web test -- src/errors/presentApiError.test.ts

echo "[error-message] OK"
