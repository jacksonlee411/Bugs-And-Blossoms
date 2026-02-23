-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS orgunit.setid_strategy_registry (
  id bigserial PRIMARY KEY,
  tenant_uuid uuid NOT NULL,
  capability_key text NOT NULL,
  owner_module text NOT NULL,
  field_key text NOT NULL,
  personalization_mode text NOT NULL,
  org_level text NOT NULL,
  business_unit_id text NOT NULL DEFAULT '',
  required boolean NOT NULL DEFAULT false,
  visible boolean NOT NULL DEFAULT true,
  default_rule_ref text NULL,
  default_value text NULL,
  priority integer NOT NULL DEFAULT 100,
  explain_required boolean NOT NULL DEFAULT false,
  is_stable boolean NOT NULL DEFAULT false,
  change_policy text NOT NULL DEFAULT 'plan_required',
  effective_date date NOT NULL,
  end_date date NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT setid_strategy_registry_capability_key_format_check CHECK (
    capability_key ~ '^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$'
  ),
  CONSTRAINT setid_strategy_registry_capability_key_context_check CHECK (
    capability_key !~ '(^|\.)(setid|scope|tenant|bu)(\.|$)'
    AND capability_key !~ '(^|\.)(bu_|setid_|tenant_|scope_)'
  ),
  CONSTRAINT setid_strategy_registry_owner_module_format_check CHECK (
    owner_module ~ '^[a-z][a-z0-9_]{0,62}$'
  ),
  CONSTRAINT setid_strategy_registry_field_key_format_check CHECK (
    field_key ~ '^[a-z][a-z0-9_]{0,62}$'
  ),
  CONSTRAINT setid_strategy_registry_personalization_mode_check CHECK (
    personalization_mode IN ('tenant_only', 'setid')
  ),
  CONSTRAINT setid_strategy_registry_org_level_check CHECK (
    org_level IN ('tenant', 'business_unit')
  ),
  CONSTRAINT setid_strategy_registry_business_unit_check CHECK (
    (org_level = 'tenant' AND business_unit_id = '')
    OR
    (org_level = 'business_unit' AND business_unit_id ~ '^[0-9]{8}$')
  ),
  CONSTRAINT setid_strategy_registry_priority_check CHECK (priority > 0),
  CONSTRAINT setid_strategy_registry_required_visible_check CHECK (
    NOT (required = true AND visible = false)
  ),
  CONSTRAINT setid_strategy_registry_effective_end_check CHECK (
    end_date IS NULL OR end_date > effective_date
  )
);

CREATE UNIQUE INDEX IF NOT EXISTS setid_strategy_registry_key_unique_idx
  ON orgunit.setid_strategy_registry (
    tenant_uuid,
    capability_key,
    field_key,
    org_level,
    business_unit_id,
    effective_date
  );

CREATE INDEX IF NOT EXISTS setid_strategy_registry_lookup_idx
  ON orgunit.setid_strategy_registry (
    tenant_uuid,
    capability_key,
    field_key,
    effective_date DESC
  );

ALTER TABLE orgunit.setid_strategy_registry ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.setid_strategy_registry FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.setid_strategy_registry;
CREATE POLICY tenant_isolation ON orgunit.setid_strategy_registry
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_runtime') THEN
    EXECUTE 'GRANT SELECT, INSERT, UPDATE ON TABLE orgunit.setid_strategy_registry TO app_runtime';
    EXECUTE 'GRANT USAGE, SELECT ON SEQUENCE orgunit.setid_strategy_registry_id_seq TO app_runtime';
  END IF;

  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'superadmin_runtime') THEN
    EXECUTE 'GRANT SELECT, INSERT, UPDATE ON TABLE orgunit.setid_strategy_registry TO superadmin_runtime';
    EXECUTE 'GRANT USAGE, SELECT ON SEQUENCE orgunit.setid_strategy_registry_id_seq TO superadmin_runtime';
  END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP POLICY IF EXISTS tenant_isolation ON orgunit.setid_strategy_registry;
ALTER TABLE IF EXISTS orgunit.setid_strategy_registry DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS orgunit.setid_strategy_registry;
-- +goose StatementEnd
