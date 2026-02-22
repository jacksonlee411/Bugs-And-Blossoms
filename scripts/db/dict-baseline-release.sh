#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 6 || $# -gt 7 ]]; then
  cat <<'USAGE' >&2
Usage:
  dict-baseline-release.sh <db_url> <source_tenant_uuid> <target_tenant_uuid> <as_of:YYYY-MM-DD> <release_id> <request_id> [initiator_uuid]

Example:
  ./scripts/db/dict-baseline-release.sh \
    "$DATABASE_URL" \
    00000000-0000-0000-0000-000000000000 \
    00000000-0000-0000-0000-000000000001 \
    2026-01-01 \
    rel-20260222 \
    req-20260222 \
    00000000-0000-0000-0000-000000000001
USAGE
  exit 1
fi

db_url="$1"
source_tenant="$2"
target_tenant="$3"
as_of="$4"
release_id="$5"
request_id="$6"
initiator="${7:-$target_tenant}"

echo "[dict-release] source=${source_tenant} target=${target_tenant} as_of=${as_of} release_id=${release_id}"

psql "${db_url}" \
  -v ON_ERROR_STOP=1 \
  -v source_tenant="${source_tenant}" \
  -v target_tenant="${target_tenant}" \
  -v as_of="${as_of}" \
  -v release_id="${release_id}" \
  -v request_id="${request_id}" \
  -v initiator="${initiator}" <<'SQL'
BEGIN;

CREATE TEMP TABLE _tmp_dict_events ON COMMIT DROP AS
SELECT id, dict_code, event_type, effective_day, request_id, payload
FROM iam.dict_events
WHERE tenant_uuid = :'source_tenant'::uuid
  AND effective_day <= :'as_of'::date
ORDER BY id ASC;

CREATE TEMP TABLE _tmp_dict_value_events ON COMMIT DROP AS
SELECT id, dict_code, code, event_type, effective_day, request_id, payload
FROM iam.dict_value_events
WHERE tenant_uuid = :'source_tenant'::uuid
  AND effective_day <= :'as_of'::date
ORDER BY id ASC;

SELECT set_config('app.current_tenant', :'target_tenant', true);

SELECT iam.submit_dict_event(
  :'target_tenant'::uuid,
  rec.dict_code,
  rec.event_type,
  rec.effective_day,
  rec.payload || jsonb_build_object(
    'release',
    jsonb_build_object(
      'release_id', :'release_id',
      'source_tenant_id', :'source_tenant',
      'target_tenant_id', :'target_tenant',
      'source_event_id', rec.id,
      'source_request_id', rec.request_id,
      'operator', :'initiator',
      'as_of', :'as_of'
    )
  ),
  format('%s#dict#%s', :'request_id', rec.id),
  :'initiator'::uuid
)
FROM _tmp_dict_events AS rec
ORDER BY rec.id;

SELECT iam.submit_dict_value_event(
  :'target_tenant'::uuid,
  rec.dict_code,
  rec.code,
  rec.event_type,
  rec.effective_day,
  rec.payload || jsonb_build_object(
    'release',
    jsonb_build_object(
      'release_id', :'release_id',
      'source_tenant_id', :'source_tenant',
      'target_tenant_id', :'target_tenant',
      'source_event_id', rec.id,
      'source_request_id', rec.request_id,
      'operator', :'initiator',
      'as_of', :'as_of'
    )
  ),
  format('%s#value#%s', :'request_id', rec.id),
  :'initiator'::uuid
)
FROM _tmp_dict_value_events AS rec
ORDER BY rec.id;

COMMIT;
SQL

echo "[dict-release] done"
