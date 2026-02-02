-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS orgunit.org_unit_codes (
  tenant_uuid uuid NOT NULL,
  org_id int NOT NULL CHECK (org_id BETWEEN 10000000 AND 99999999),
  org_code text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_uuid, org_id),
  CONSTRAINT org_unit_codes_org_code_format CHECK (
    length(org_code) BETWEEN 1 AND 16
    AND org_code = upper(btrim(org_code))
    AND org_code ~ '^[A-Z0-9_-]{1,16}$'
  ),
  CONSTRAINT org_unit_codes_org_code_unique UNIQUE (tenant_uuid, org_code)
);

ALTER TABLE orgunit.org_unit_codes ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.org_unit_codes FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.org_unit_codes;
CREATE POLICY tenant_isolation ON orgunit.org_unit_codes
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP POLICY IF EXISTS tenant_isolation ON orgunit.org_unit_codes;
ALTER TABLE IF EXISTS orgunit.org_unit_codes DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS orgunit.org_unit_codes;
-- +goose StatementEnd
