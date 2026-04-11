-- +goose Up
-- +goose StatementBegin
ALTER TABLE orgunit.setid_strategy_registry
  ADD COLUMN IF NOT EXISTS resolved_setid text NOT NULL DEFAULT '';

UPDATE orgunit.setid_strategy_registry
SET resolved_setid = ''
WHERE org_applicability = 'tenant';

WITH business_unit_resolution AS (
  SELECT
    r.id,
    count(DISTINCT b.setid)::int AS resolved_setid_count,
    min(b.setid) AS resolved_setid
  FROM orgunit.setid_strategy_registry r
  LEFT JOIN orgunit.setid_binding_versions b
    ON b.tenant_uuid = r.tenant_uuid
   AND b.org_id = orgunit.decode_org_node_key(NULLIF(btrim(r.business_unit_node_key), '')::char(8))::int
   AND b.validity @> r.effective_date
  WHERE r.org_applicability = 'business_unit'
  GROUP BY r.id
)
UPDATE orgunit.setid_strategy_registry r
SET resolved_setid = upper(btrim(resolved.resolved_setid))
FROM business_unit_resolution resolved
WHERE r.id = resolved.id
  AND resolved.resolved_setid_count = 1
  AND resolved.resolved_setid IS NOT NULL;

DO $$
DECLARE
  v_issue record;
BEGIN
  SELECT
    r.tenant_uuid::text AS tenant_uuid,
    r.capability_key,
    r.field_key,
    r.business_unit_node_key,
    r.effective_date::text AS effective_date,
    count(DISTINCT b.setid)::int AS resolved_setid_count
  INTO v_issue
  FROM orgunit.setid_strategy_registry r
  LEFT JOIN orgunit.setid_binding_versions b
    ON b.tenant_uuid = r.tenant_uuid
   AND b.org_id = orgunit.decode_org_node_key(NULLIF(btrim(r.business_unit_node_key), '')::char(8))::int
   AND b.validity @> r.effective_date
  WHERE r.org_applicability = 'business_unit'
  GROUP BY r.id, r.tenant_uuid, r.capability_key, r.field_key, r.business_unit_node_key, r.effective_date
  HAVING count(DISTINCT b.setid) <> 1
  ORDER BY r.tenant_uuid, r.capability_key, r.field_key, r.business_unit_node_key, r.effective_date
  LIMIT 1;

  IF FOUND THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_STRATEGY_RESOLVED_SETID_BACKFILL_BLOCKED',
      DETAIL = format(
        'tenant_uuid=%s capability_key=%s field_key=%s business_unit_node_key=%s effective_date=%s resolved_setid_count=%s',
        v_issue.tenant_uuid,
        v_issue.capability_key,
        v_issue.field_key,
        v_issue.business_unit_node_key,
        v_issue.effective_date,
        v_issue.resolved_setid_count
      );
  END IF;
END
$$;

ALTER TABLE orgunit.setid_strategy_registry
  DROP CONSTRAINT IF EXISTS setid_strategy_registry_business_unit_node_key_applicability_check;

ALTER TABLE orgunit.setid_strategy_registry
  DROP CONSTRAINT IF EXISTS setid_strategy_registry_resolved_setid_format_check,
  DROP CONSTRAINT IF EXISTS setid_strategy_registry_scope_shape_check;

ALTER TABLE orgunit.setid_strategy_registry
  ADD CONSTRAINT setid_strategy_registry_resolved_setid_format_check CHECK (
    btrim(resolved_setid) = ''
    OR btrim(resolved_setid) ~ '^[A-Z0-9]{5}$'
  ),
  ADD CONSTRAINT setid_strategy_registry_scope_shape_check CHECK (
    (
      org_applicability = 'tenant'
      AND business_unit_node_key = ''
      AND (
        btrim(resolved_setid) = ''
        OR btrim(resolved_setid) ~ '^[A-Z0-9]{5}$'
      )
    )
    OR
    (
      org_applicability = 'business_unit'
      AND orgunit.is_valid_org_node_key(btrim(business_unit_node_key))
      AND btrim(resolved_setid) ~ '^[A-Z0-9]{5}$'
    )
  );

DROP INDEX IF EXISTS orgunit.setid_strategy_registry_key_unique_idx;
CREATE UNIQUE INDEX setid_strategy_registry_key_unique_idx
  ON orgunit.setid_strategy_registry (
    tenant_uuid,
    capability_key,
    field_key,
    org_applicability,
    resolved_setid,
    business_unit_node_key,
    effective_date
  );

DROP INDEX IF EXISTS orgunit.setid_strategy_registry_lookup_idx;
CREATE INDEX setid_strategy_registry_lookup_idx
  ON orgunit.setid_strategy_registry (
    tenant_uuid,
    capability_key,
    field_key,
    resolved_setid,
    business_unit_node_key,
    effective_date DESC
  );
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM orgunit.setid_strategy_registry
    GROUP BY tenant_uuid, capability_key, field_key, org_applicability, business_unit_node_key, effective_date
    HAVING count(*) > 1
  ) THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_STRATEGY_REGISTRY_DOWNGRADE_BLOCKED',
      DETAIL = 'resolved_setid differentiated rows cannot be losslessly collapsed to the pre-PR-3 unique key';
  END IF;
END
$$;

ALTER TABLE orgunit.setid_strategy_registry
  DROP CONSTRAINT IF EXISTS setid_strategy_registry_scope_shape_check,
  DROP CONSTRAINT IF EXISTS setid_strategy_registry_resolved_setid_format_check;

DROP INDEX IF EXISTS orgunit.setid_strategy_registry_lookup_idx;
DROP INDEX IF EXISTS orgunit.setid_strategy_registry_key_unique_idx;

ALTER TABLE orgunit.setid_strategy_registry
  DROP COLUMN IF EXISTS resolved_setid;

ALTER TABLE orgunit.setid_strategy_registry
  ADD CONSTRAINT setid_strategy_registry_business_unit_node_key_applicability_check CHECK (
    (org_applicability = 'tenant' AND business_unit_node_key = '')
    OR
    (
      org_applicability = 'business_unit'
      AND orgunit.is_valid_org_node_key(btrim(business_unit_node_key))
    )
  );

CREATE UNIQUE INDEX setid_strategy_registry_key_unique_idx
  ON orgunit.setid_strategy_registry (
    tenant_uuid,
    capability_key,
    field_key,
    org_applicability,
    business_unit_node_key,
    effective_date
  );

CREATE INDEX setid_strategy_registry_lookup_idx
  ON orgunit.setid_strategy_registry (
    tenant_uuid,
    capability_key,
    field_key,
    effective_date DESC
  );
-- +goose StatementEnd
