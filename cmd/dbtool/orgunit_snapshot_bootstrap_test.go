package main

import (
	"strings"
	"testing"
)

func TestOrgunitSnapshotBootstrapFiles(t *testing.T) {
	want := []string{
		"00023_orgunit_org_node_key_schema.sql",
		"00024_orgunit_org_node_key_allocator.sql",
		"00025_orgunit_org_node_key_kernel_privileges.sql",
	}
	if len(orgunitSnapshotBootstrapFiles) != len(want) {
		t.Fatalf("unexpected bootstrap file count: got %d want %d", len(orgunitSnapshotBootstrapFiles), len(want))
	}
	for i := range want {
		if orgunitSnapshotBootstrapFiles[i] != want[i] {
			t.Fatalf("unexpected bootstrap file at %d: got %q want %q", i, orgunitSnapshotBootstrapFiles[i], want[i])
		}
	}
}

func TestOrgunitSnapshotBootstrapPaths(t *testing.T) {
	t.Run("org-only bootstrap", func(t *testing.T) {
		paths := orgunitSnapshotBootstrapPaths("/tmp/org-bootstrap", false)
		want := []string{
			"/tmp/org-bootstrap/00023_orgunit_org_node_key_schema.sql",
			"/tmp/org-bootstrap/00024_orgunit_org_node_key_allocator.sql",
			"/tmp/org-bootstrap/00025_orgunit_org_node_key_kernel_privileges.sql",
		}
		if len(paths) != len(want) {
			t.Fatalf("unexpected path count: got %d want %d", len(paths), len(want))
		}
		for i := range want {
			if paths[i] != want[i] {
				t.Fatalf("unexpected path at %d: got %q want %q", i, paths[i], want[i])
			}
		}
	})

	t.Run("org plus setid bootstrap", func(t *testing.T) {
		paths := orgunitSnapshotBootstrapPaths("/tmp/org-bootstrap", true)
		want := []string{
			"/tmp/org-bootstrap/00023_orgunit_org_node_key_schema.sql",
			"/tmp/org-bootstrap/00024_orgunit_org_node_key_allocator.sql",
			"/tmp/org-bootstrap/00025_orgunit_org_node_key_kernel_privileges.sql",
			defaultSetIDStrategyRegistrySchemaDir + "/00020_orgunit_setid_strategy_registry_schema.sql",
			defaultSetIDStrategyRegistrySchemaDir + "/00021_orgunit_setid_strategy_registry_fields.sql",
			defaultSetIDStrategyRegistrySchemaDir + "/00022_orgunit_setid_strategy_registry_modes.sql",
		}
		if len(paths) != len(want) {
			t.Fatalf("unexpected path count: got %d want %d", len(paths), len(want))
		}
		for i := range want {
			if paths[i] != want[i] {
				t.Fatalf("unexpected path at %d: got %q want %q", i, paths[i], want[i])
			}
		}
	})
}

func TestOrgunitSnapshotBootstrapPrelude(t *testing.T) {
	required := []string{
		"CREATE EXTENSION IF NOT EXISTS ltree;",
		"CREATE EXTENSION IF NOT EXISTS btree_gist;",
		"CREATE SCHEMA IF NOT EXISTS orgunit;",
		"CREATE OR REPLACE FUNCTION orgunit.assert_current_tenant",
		"MESSAGE = 'RLS_TENANT_CONTEXT_MISSING'",
		"MESSAGE = 'RLS_TENANT_CONTEXT_MISMATCH'",
	}
	for _, snippet := range required {
		if !strings.Contains(orgunitSnapshotBootstrapPrelude, snippet) {
			t.Fatalf("bootstrap prelude missing %q", snippet)
		}
	}
}
