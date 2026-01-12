#!/usr/bin/env bash
set -euo pipefail

tenant_id=""
email=""
password=""
role_slug="tenant-admin"
kratos_admin_base_url="${KRATOS_STUB_ADMIN_BASE_URL:-http://127.0.0.1:4434}"

usage() {
  cat <<'EOF' >&2
usage:
  seed_kratosstub_identity.sh --tenant-id <uuid> --email <email> --password <pw> [--role-slug <slug>] [--kratos-admin-base-url <url>]

notes:
  - Creates an identity in KratosStub via POST /admin/identities
  - Identifier is always "tenant_id:email" (matches server-side login behavior)
  - Idempotent: HTTP 409 (already exists) is treated as success
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --tenant-id) tenant_id="${2:-}"; shift 2 ;;
    --email) email="${2:-}"; shift 2 ;;
    --password) password="${2:-}"; shift 2 ;;
    --role-slug) role_slug="${2:-}"; shift 2 ;;
    --kratos-admin-base-url) kratos_admin_base_url="${2:-}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *)
      echo "unknown arg: $1" >&2
      usage
      exit 2
      ;;
  esac
done

email="$(printf "%s" "$email" | tr '[:upper:]' '[:lower:]' | xargs)"
role_slug="$(printf "%s" "$role_slug" | tr '[:upper:]' '[:lower:]' | xargs)"

if [[ -z "$tenant_id" || -z "$email" || -z "$password" ]]; then
  usage
  exit 2
fi

identifier="${tenant_id}:${email}"

payload="$(cat <<JSON
{
  "schema_id": "default",
  "traits": {
    "tenant_id": "${tenant_id}",
    "email": "${email}",
    "role_slug": "${role_slug}"
  },
  "credentials": {
    "password": {
      "identifiers": ["${identifier}"],
      "config": { "password": "${password}" }
    }
  }
}
JSON
)"

tmp="$(mktemp)"
code="$(curl -sS -o "$tmp" -w "%{http_code}" \
  -H "Content-Type: application/json" \
  -X POST "${kratos_admin_base_url}/admin/identities" \
  -d "$payload" || true)"

case "$code" in
  200)
    echo "[kratosstub] created identity: identifier=${identifier}"
    cat "$tmp"
    ;;
  409)
    echo "[kratosstub] identity already exists (409): identifier=${identifier}"
    ;;
  *)
    echo "[kratosstub] create identity failed: http_status=${code} url=${kratos_admin_base_url}/admin/identities" >&2
    cat "$tmp" >&2
    exit 1
    ;;
esac

