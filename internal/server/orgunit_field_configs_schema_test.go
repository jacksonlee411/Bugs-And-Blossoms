package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOrgunitFieldConfigSchema_UsesFrozenDataSourceConfigShape(t *testing.T) {
	root := repoRootFromCurrentFile(t)
	p := filepath.Join(root, "modules/orgunit/infrastructure/persistence/schema/00016_orgunit_field_configs_schema.sql")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	s := string(b)

	for _, token := range []string{
		"CONSTRAINT tenant_field_configs_data_source_type_check CHECK (data_source_type IN ('PLAIN','DICT','ENTITY'))",
		"display_label text NULL",
		"CONSTRAINT tenant_field_configs_plain_config_check CHECK",
		"data_source_config = jsonb_build_object('dict_code', data_source_config->'dict_code')",
		"data_source_config = jsonb_build_object(",
	} {
		if !strings.Contains(s, token) {
			t.Fatalf("missing %q in %s", token, p)
		}
	}

	if strings.Contains(s, "jsonb_object_length(") {
		t.Fatalf("unexpected jsonb_object_length() in %s", p)
	}
}

func TestOrgunitFieldConfigSchema_HasRLSGuardAndImmutability(t *testing.T) {
	root := repoRootFromCurrentFile(t)
	p := filepath.Join(root, "modules/orgunit/infrastructure/persistence/schema/00016_orgunit_field_configs_schema.sql")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	s := string(b)

	for _, token := range []string{
		"ALTER TABLE orgunit.tenant_field_config_events ENABLE ROW LEVEL SECURITY;",
		"ALTER TABLE orgunit.tenant_field_config_events FORCE ROW LEVEL SECURITY;",
		"ALTER TABLE orgunit.tenant_field_configs ENABLE ROW LEVEL SECURITY;",
		"ALTER TABLE orgunit.tenant_field_configs FORCE ROW LEVEL SECURITY;",
		"CREATE OR REPLACE FUNCTION orgunit.guard_tenant_field_configs_write()",
		"MESSAGE = 'ORGUNIT_FIELD_CONFIGS_WRITE_FORBIDDEN'",
		"CREATE OR REPLACE FUNCTION orgunit.assert_tenant_field_configs_update_allowed()",
		"MESSAGE = 'ORG_FIELD_CONFIG_MAPPING_IMMUTABLE'",
		"MESSAGE = 'ORG_FIELD_CONFIG_DISABLED_ON_INVALID'",
	} {
		if !strings.Contains(s, token) {
			t.Fatalf("missing %q in %s", token, p)
		}
	}
}

func TestOrgunitFieldConfigSchema_HasKernelOneDoorEntryPoints(t *testing.T) {
	root := repoRootFromCurrentFile(t)
	schemaPath := filepath.Join(root, "modules/orgunit/infrastructure/persistence/schema/00016_orgunit_field_configs_schema.sql")
	privilegePath := filepath.Join(root, "modules/orgunit/infrastructure/persistence/schema/00017_orgunit_field_configs_kernel_privileges.sql")

	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read %s: %v", schemaPath, err)
	}
	s := string(schemaBytes)

	for _, token := range []string{
		"CREATE OR REPLACE FUNCTION orgunit.enable_tenant_field_config(",
		"CREATE OR REPLACE FUNCTION orgunit.disable_tenant_field_config(",
		"CREATE OR REPLACE FUNCTION orgunit.rekey_tenant_field_config(",
		"pg_advisory_xact_lock(hashtextextended(v_lock_key, 0))",
		"MESSAGE = 'ORG_REQUEST_ID_CONFLICT'",
		"MESSAGE = 'ORG_FIELD_CONFIG_SLOT_EXHAUSTED'",
		"MESSAGE = 'ORG_FIELD_CONFIG_NOT_FOUND'",
	} {
		if !strings.Contains(s, token) {
			t.Fatalf("missing %q in %s", token, schemaPath)
		}
	}

	privilegeBytes, err := os.ReadFile(privilegePath)
	if err != nil {
		t.Fatalf("read %s: %v", privilegePath, err)
	}
	p := string(privilegeBytes)

	for _, token := range []string{
		"ALTER FUNCTION orgunit.enable_tenant_field_config(uuid, text, text, date, text, jsonb, text, text, uuid)\n  SECURITY DEFINER;",
		"ALTER FUNCTION orgunit.disable_tenant_field_config(uuid, text, date, text, uuid)\n  SECURITY DEFINER;",
		"ALTER FUNCTION orgunit.rekey_tenant_field_config(uuid, text, text, text, uuid)\n  SECURITY DEFINER;",
		"REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON TABLE ",
	} {
		if !strings.Contains(p, token) {
			t.Fatalf("missing %q in %s", token, privilegePath)
		}
	}
}
