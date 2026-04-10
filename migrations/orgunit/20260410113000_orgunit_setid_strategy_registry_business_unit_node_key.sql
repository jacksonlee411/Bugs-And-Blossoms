-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION orgunit.is_valid_org_node_key(p_value text)
RETURNS boolean
LANGUAGE sql
IMMUTABLE
AS $$
  SELECT btrim(COALESCE(p_value, '')) ~ '^[ABCDEFGHJKLMNPQRSTUVWXYZ][ABCDEFGHJKLMNPQRSTUVWXYZ23456789]{7}$';
$$;

ALTER TABLE orgunit.setid_strategy_registry
  DROP CONSTRAINT IF EXISTS setid_strategy_registry_business_unit_applicability_check;

DROP INDEX IF EXISTS orgunit.setid_strategy_registry_key_unique_idx;

ALTER TABLE orgunit.setid_strategy_registry
  RENAME COLUMN business_unit_id TO business_unit_node_key;

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
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM orgunit.setid_strategy_registry
    WHERE org_applicability = 'business_unit'
      AND btrim(business_unit_node_key) <> ''
      AND business_unit_node_key !~ '^[0-9]{8}$'
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'SETID_STRATEGY_REGISTRY_DOWNGRADE_BLOCKED',
      DETAIL = 'business_unit_node_key contains non-legacy values and cannot be losslessly downgraded to business_unit_id';
  END IF;
END
$$;

ALTER TABLE orgunit.setid_strategy_registry
  DROP CONSTRAINT IF EXISTS setid_strategy_registry_business_unit_node_key_applicability_check;

DROP INDEX IF EXISTS orgunit.setid_strategy_registry_key_unique_idx;

ALTER TABLE orgunit.setid_strategy_registry
  RENAME COLUMN business_unit_node_key TO business_unit_id;

ALTER TABLE orgunit.setid_strategy_registry
  ADD CONSTRAINT setid_strategy_registry_business_unit_applicability_check CHECK (
    (org_applicability = 'tenant' AND business_unit_id = '')
    OR
    (org_applicability = 'business_unit' AND business_unit_id ~ '^[0-9]{8}$')
  );

CREATE UNIQUE INDEX setid_strategy_registry_key_unique_idx
  ON orgunit.setid_strategy_registry (
    tenant_uuid,
    capability_key,
    field_key,
    org_applicability,
    business_unit_id,
    effective_date
  );

DROP FUNCTION IF EXISTS orgunit.is_valid_org_node_key(text);
-- +goose StatementEnd
