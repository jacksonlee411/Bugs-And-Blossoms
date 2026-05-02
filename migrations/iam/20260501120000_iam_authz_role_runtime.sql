-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS iam.role_definitions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_uuid uuid NOT NULL REFERENCES iam.tenants(id) ON DELETE CASCADE,
  role_slug text NOT NULL,
  name text NOT NULL,
  description text NOT NULL DEFAULT '',
  system_managed boolean NOT NULL DEFAULT false,
  revision bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT role_definitions_slug_nonempty_check CHECK (btrim(role_slug) <> ''),
  CONSTRAINT role_definitions_slug_lower_check CHECK (role_slug = lower(role_slug)),
  CONSTRAINT role_definitions_slug_trim_check CHECK (role_slug = btrim(role_slug)),
  CONSTRAINT role_definitions_name_nonempty_check CHECK (btrim(name) <> ''),
  CONSTRAINT role_definitions_revision_positive_check CHECK (revision >= 1),
  UNIQUE (tenant_uuid, role_slug)
);

CREATE INDEX IF NOT EXISTS role_definitions_tenant_updated_idx
  ON iam.role_definitions (tenant_uuid, updated_at DESC, role_slug ASC);

CREATE TABLE IF NOT EXISTS iam.role_authz_capabilities (
  role_id uuid NOT NULL REFERENCES iam.role_definitions(id) ON DELETE CASCADE,
  authz_capability_key text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT role_authz_capabilities_key_nonempty_check CHECK (btrim(authz_capability_key) <> ''),
  CONSTRAINT role_authz_capabilities_key_lower_check CHECK (authz_capability_key = lower(authz_capability_key)),
  CONSTRAINT role_authz_capabilities_key_trim_check CHECK (authz_capability_key = btrim(authz_capability_key)),
  PRIMARY KEY (role_id, authz_capability_key)
);

CREATE TABLE IF NOT EXISTS iam.principal_role_assignments (
  tenant_uuid uuid NOT NULL REFERENCES iam.tenants(id) ON DELETE CASCADE,
  principal_id uuid NOT NULL REFERENCES iam.principals(id) ON DELETE CASCADE,
  role_slug text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT principal_role_assignments_slug_nonempty_check CHECK (btrim(role_slug) <> ''),
  CONSTRAINT principal_role_assignments_slug_lower_check CHECK (role_slug = lower(role_slug)),
  CONSTRAINT principal_role_assignments_slug_trim_check CHECK (role_slug = btrim(role_slug)),
  CONSTRAINT principal_role_assignments_role_fk FOREIGN KEY (tenant_uuid, role_slug)
    REFERENCES iam.role_definitions (tenant_uuid, role_slug) ON DELETE RESTRICT,
  PRIMARY KEY (tenant_uuid, principal_id, role_slug)
);

CREATE INDEX IF NOT EXISTS principal_role_assignments_principal_idx
  ON iam.principal_role_assignments (tenant_uuid, principal_id);

CREATE TABLE IF NOT EXISTS iam.principal_authz_assignment_revisions (
  tenant_uuid uuid NOT NULL REFERENCES iam.tenants(id) ON DELETE CASCADE,
  principal_id uuid NOT NULL REFERENCES iam.principals(id) ON DELETE CASCADE,
  revision bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT principal_authz_assignment_revisions_revision_positive_check CHECK (revision >= 1),
  PRIMARY KEY (tenant_uuid, principal_id)
);

CREATE TABLE IF NOT EXISTS iam.principal_org_scope_bindings (
  tenant_uuid uuid NOT NULL REFERENCES iam.tenants(id) ON DELETE CASCADE,
  principal_id uuid NOT NULL REFERENCES iam.principals(id) ON DELETE CASCADE,
  org_node_key text NOT NULL,
  include_descendants boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT principal_org_scope_bindings_node_key_nonempty_check CHECK (btrim(org_node_key) <> ''),
  CONSTRAINT principal_org_scope_bindings_node_key_trim_check CHECK (org_node_key = btrim(org_node_key)),
  PRIMARY KEY (tenant_uuid, principal_id, org_node_key)
);

CREATE INDEX IF NOT EXISTS principal_org_scope_bindings_principal_idx
  ON iam.principal_org_scope_bindings (tenant_uuid, principal_id);

ALTER TABLE iam.role_definitions ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam.role_definitions FORCE ROW LEVEL SECURITY;
ALTER TABLE iam.role_authz_capabilities ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam.role_authz_capabilities FORCE ROW LEVEL SECURITY;
ALTER TABLE iam.principal_role_assignments ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam.principal_role_assignments FORCE ROW LEVEL SECURITY;
ALTER TABLE iam.principal_authz_assignment_revisions ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam.principal_authz_assignment_revisions FORCE ROW LEVEL SECURITY;
ALTER TABLE iam.principal_org_scope_bindings ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam.principal_org_scope_bindings FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON iam.role_definitions
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

CREATE POLICY tenant_isolation ON iam.role_authz_capabilities
USING (
  EXISTS (
    SELECT 1
    FROM iam.role_definitions rd
    WHERE rd.id = role_authz_capabilities.role_id
      AND rd.tenant_uuid = current_setting('app.current_tenant')::uuid
  )
)
WITH CHECK (
  EXISTS (
    SELECT 1
    FROM iam.role_definitions rd
    WHERE rd.id = role_authz_capabilities.role_id
      AND rd.tenant_uuid = current_setting('app.current_tenant')::uuid
  )
);

CREATE POLICY tenant_isolation ON iam.principal_role_assignments
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

CREATE POLICY tenant_isolation ON iam.principal_authz_assignment_revisions
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

CREATE POLICY tenant_isolation ON iam.principal_org_scope_bindings
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

CREATE OR REPLACE FUNCTION iam.seed_builtin_authz_roles(p_tenant_uuid uuid)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
DECLARE
  v_admin_role_id uuid;
  v_viewer_role_id uuid;
BEGIN
  PERFORM set_config('app.current_tenant', p_tenant_uuid::text, true);

  INSERT INTO iam.role_definitions (tenant_uuid, role_slug, name, description, system_managed)
  VALUES (
    p_tenant_uuid,
    'tenant-admin',
    'Tenant Admin',
    'Built-in tenant administrator role',
    true
  )
  ON CONFLICT (tenant_uuid, role_slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    system_managed = true,
    updated_at = now()
  RETURNING id INTO v_admin_role_id;

  INSERT INTO iam.role_definitions (tenant_uuid, role_slug, name, description, system_managed)
  VALUES (
    p_tenant_uuid,
    'tenant-viewer',
    'Tenant Viewer',
    'Built-in tenant viewer role',
    true
  )
  ON CONFLICT (tenant_uuid, role_slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    system_managed = true,
    updated_at = now()
  RETURNING id INTO v_viewer_role_id;

  DELETE FROM iam.role_authz_capabilities
  WHERE role_id IN (v_admin_role_id, v_viewer_role_id);

  INSERT INTO iam.role_authz_capabilities (role_id, authz_capability_key)
  SELECT v_admin_role_id, key
  FROM unnest(ARRAY[
    'iam.authz:read',
    'iam.authz:admin',
    'iam.dicts:read',
    'iam.dicts:admin',
    'iam.dict_release:admin',
    'cubebox.conversations:read',
    'cubebox.conversations:use',
    'cubebox.model_provider:update',
    'cubebox.model_credential:read',
    'cubebox.model_credential:rotate',
    'cubebox.model_credential:deactivate',
    'cubebox.model_selection:select',
    'cubebox.model_selection:verify',
    'orgunit.orgunits:read',
    'orgunit.orgunits:admin'
  ]::text[]) AS key
  ON CONFLICT DO NOTHING;

  INSERT INTO iam.role_authz_capabilities (role_id, authz_capability_key)
  SELECT v_viewer_role_id, key
  FROM unnest(ARRAY[
    'iam.dicts:read',
    'cubebox.conversations:read',
    'cubebox.conversations:use',
    'orgunit.orgunits:read'
  ]::text[]) AS key
  ON CONFLICT DO NOTHING;
END
$$;

CREATE OR REPLACE FUNCTION iam.seed_builtin_authz_roles_for_tenant()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
BEGIN
  PERFORM iam.seed_builtin_authz_roles(NEW.id);
  RETURN NEW;
END
$$;

DROP TRIGGER IF EXISTS tenants_seed_builtin_authz_roles ON iam.tenants;
CREATE TRIGGER tenants_seed_builtin_authz_roles
AFTER INSERT ON iam.tenants
FOR EACH ROW
EXECUTE FUNCTION iam.seed_builtin_authz_roles_for_tenant();

DO $$
DECLARE
  v_tenant uuid;
  v_root_org_node_key text;
BEGIN
  FOR v_tenant IN SELECT id FROM iam.tenants LOOP
    PERFORM iam.seed_builtin_authz_roles(v_tenant);
    PERFORM set_config('app.current_tenant', v_tenant::text, true);

    INSERT INTO iam.principal_role_assignments (tenant_uuid, principal_id, role_slug)
    SELECT p.tenant_uuid, p.id, p.role_slug
    FROM iam.principals p
    JOIN iam.role_definitions rd
      ON rd.tenant_uuid = p.tenant_uuid
     AND rd.role_slug = p.role_slug
    WHERE p.tenant_uuid = v_tenant
    ON CONFLICT DO NOTHING;

    INSERT INTO iam.principal_authz_assignment_revisions (tenant_uuid, principal_id)
    SELECT DISTINCT pra.tenant_uuid, pra.principal_id
    FROM iam.principal_role_assignments pra
    WHERE pra.tenant_uuid = v_tenant
    ON CONFLICT DO NOTHING;

    IF to_regclass('orgunit.org_trees') IS NOT NULL THEN
      SELECT btrim(COALESCE(to_jsonb(t)->>'root_org_node_key', ''))
      INTO v_root_org_node_key
      FROM orgunit.org_trees t
      WHERE t.tenant_uuid = v_tenant;

      IF btrim(COALESCE(v_root_org_node_key, '')) <> '' THEN
        INSERT INTO iam.principal_org_scope_bindings (tenant_uuid, principal_id, org_node_key, include_descendants)
        SELECT DISTINCT pra.tenant_uuid, pra.principal_id, btrim(v_root_org_node_key), true
        FROM iam.principal_role_assignments pra
        JOIN iam.role_definitions rd
          ON rd.tenant_uuid = pra.tenant_uuid
         AND rd.role_slug = pra.role_slug
        JOIN iam.role_authz_capabilities rac
          ON rac.role_id = rd.id
        WHERE pra.tenant_uuid = v_tenant
          AND rac.authz_capability_key IN ('orgunit.orgunits:read', 'orgunit.orgunits:admin')
        ON CONFLICT DO NOTHING;
      END IF;
    END IF;
  END LOOP;
END
$$;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_runtime') THEN
    EXECUTE 'GRANT USAGE ON SCHEMA iam TO app_runtime';
    EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON iam.role_definitions TO app_runtime';
    EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON iam.role_authz_capabilities TO app_runtime';
    EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON iam.principal_role_assignments TO app_runtime';
    EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON iam.principal_authz_assignment_revisions TO app_runtime';
    EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON iam.principal_org_scope_bindings TO app_runtime';
    EXECUTE 'GRANT EXECUTE ON FUNCTION iam.seed_builtin_authz_roles(uuid) TO app_runtime';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_nobypassrls') THEN
    EXECUTE 'GRANT USAGE ON SCHEMA iam TO app_nobypassrls';
    EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON iam.role_definitions TO app_nobypassrls';
    EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON iam.role_authz_capabilities TO app_nobypassrls';
    EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON iam.principal_role_assignments TO app_nobypassrls';
    EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON iam.principal_authz_assignment_revisions TO app_nobypassrls';
    EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON iam.principal_org_scope_bindings TO app_nobypassrls';
    EXECUTE 'GRANT EXECUTE ON FUNCTION iam.seed_builtin_authz_roles(uuid) TO app_nobypassrls';
  END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS tenants_seed_builtin_authz_roles ON iam.tenants;
DROP FUNCTION IF EXISTS iam.seed_builtin_authz_roles_for_tenant();
DROP FUNCTION IF EXISTS iam.seed_builtin_authz_roles(uuid);
DROP INDEX IF EXISTS iam.principal_org_scope_bindings_principal_idx;
DROP TABLE IF EXISTS iam.principal_org_scope_bindings;
DROP TABLE IF EXISTS iam.principal_authz_assignment_revisions;
DROP INDEX IF EXISTS iam.principal_role_assignments_principal_idx;
DROP TABLE IF EXISTS iam.principal_role_assignments;
DROP TABLE IF EXISTS iam.role_authz_capabilities;
DROP INDEX IF EXISTS iam.role_definitions_tenant_updated_idx;
DROP TABLE IF EXISTS iam.role_definitions;
-- +goose StatementEnd
