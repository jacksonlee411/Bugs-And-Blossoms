#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 4 || $# -gt 5 ]]; then
  cat <<'USAGE' >&2
Usage:
  dict-baseline-reconcile.sh <db_url> <source_tenant_uuid> <target_tenant_uuid> <as_of:YYYY-MM-DD> [sample_limit]

Example:
  ./scripts/db/dict-baseline-reconcile.sh \
    "$DATABASE_URL" \
    00000000-0000-0000-0000-000000000000 \
    00000000-0000-0000-0000-000000000001 \
    2026-01-01 \
    50
USAGE
  exit 1
fi

db_url="$1"
source_tenant="$2"
target_tenant="$3"
as_of="$4"
sample_limit="${5:-50}"

echo "[dict-reconcile] source=${source_tenant} target=${target_tenant} as_of=${as_of} sample_limit=${sample_limit}"

psql "${db_url}" \
  -v ON_ERROR_STOP=1 \
  -v source_tenant="${source_tenant}" \
  -v target_tenant="${target_tenant}" \
  -v as_of="${as_of}" \
  -v sample_limit="${sample_limit}" <<'SQL'
\pset pager off

WITH
src_dicts AS (
  SELECT dict_code, name
  FROM iam.dicts
  WHERE tenant_uuid = :'source_tenant'::uuid
    AND enabled_on <= :'as_of'::date
    AND (disabled_on IS NULL OR :'as_of'::date < disabled_on)
),
tgt_dicts AS (
  SELECT dict_code, name
  FROM iam.dicts
  WHERE tenant_uuid = :'target_tenant'::uuid
    AND enabled_on <= :'as_of'::date
    AND (disabled_on IS NULL OR :'as_of'::date < disabled_on)
),
src_values AS (
  SELECT dict_code, code, label
  FROM iam.dict_value_segments
  WHERE tenant_uuid = :'source_tenant'::uuid
    AND enabled_on <= :'as_of'::date
    AND (disabled_on IS NULL OR :'as_of'::date < disabled_on)
),
tgt_values AS (
  SELECT dict_code, code, label
  FROM iam.dict_value_segments
  WHERE tenant_uuid = :'target_tenant'::uuid
    AND enabled_on <= :'as_of'::date
    AND (disabled_on IS NULL OR :'as_of'::date < disabled_on)
)
SELECT
  (SELECT count(*) FROM src_dicts) AS source_dict_count,
  (SELECT count(*) FROM tgt_dicts) AS target_dict_count,
  (SELECT count(*) FROM src_values) AS source_value_count,
  (SELECT count(*) FROM tgt_values) AS target_value_count,
  (SELECT count(*) FROM src_dicts s LEFT JOIN tgt_dicts t USING(dict_code) WHERE t.dict_code IS NULL) AS missing_dict_count,
  (SELECT count(*) FROM src_dicts s JOIN tgt_dicts t USING(dict_code) WHERE s.name <> t.name) AS dict_name_mismatch_count,
  (SELECT count(*) FROM src_values s LEFT JOIN tgt_values t USING(dict_code, code) WHERE t.code IS NULL) AS missing_value_count,
  (SELECT count(*) FROM src_values s JOIN tgt_values t USING(dict_code, code) WHERE s.label <> t.label) AS value_label_mismatch_count;

SELECT 'dict_missing' AS kind, s.dict_code, '' AS code, s.name AS source_value, '' AS target_value
FROM src_dicts s
LEFT JOIN tgt_dicts t USING(dict_code)
WHERE t.dict_code IS NULL
ORDER BY s.dict_code
LIMIT :'sample_limit'::int;

SELECT 'dict_name_mismatch' AS kind, s.dict_code, '' AS code, s.name AS source_value, t.name AS target_value
FROM src_dicts s
JOIN tgt_dicts t USING(dict_code)
WHERE s.name <> t.name
ORDER BY s.dict_code
LIMIT :'sample_limit'::int;

SELECT 'value_missing' AS kind, s.dict_code, s.code, s.label AS source_value, '' AS target_value
FROM src_values s
LEFT JOIN tgt_values t USING(dict_code, code)
WHERE t.code IS NULL
ORDER BY s.dict_code, s.code
LIMIT :'sample_limit'::int;

SELECT 'value_label_mismatch' AS kind, s.dict_code, s.code, s.label AS source_value, t.label AS target_value
FROM src_values s
JOIN tgt_values t USING(dict_code, code)
WHERE s.label <> t.label
ORDER BY s.dict_code, s.code
LIMIT :'sample_limit'::int;
SQL

echo "[dict-reconcile] done"
