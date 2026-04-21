package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"
)

const defaultOrgunitSnapshotSchemaDir = "modules/orgunit/infrastructure/persistence/org-node-key-bootstrap"

var orgunitSnapshotBootstrapFiles = []string{
	"00023_orgunit_org_node_key_schema.sql",
	"00024_orgunit_org_node_key_allocator.sql",
	"00025_orgunit_org_node_key_kernel_privileges.sql",
}

const orgunitSnapshotBootstrapPrelude = `
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS ltree;
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE SCHEMA IF NOT EXISTS orgunit;

CREATE OR REPLACE FUNCTION orgunit.assert_current_tenant(p_tenant_uuid uuid)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_ctx_raw text;
  v_ctx_tenant uuid;
BEGIN
  IF p_tenant_uuid IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'ORG_INVALID_ARGUMENT',
      DETAIL = 'tenant_uuid is required';
  END IF;

  v_ctx_raw := current_setting('app.current_tenant', true);
  IF v_ctx_raw IS NULL OR btrim(v_ctx_raw) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'RLS_TENANT_CONTEXT_MISSING',
      DETAIL = 'app.current_tenant is required';
  END IF;

  BEGIN
    v_ctx_tenant := v_ctx_raw::uuid;
  EXCEPTION
    WHEN invalid_text_representation THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'RLS_TENANT_CONTEXT_INVALID',
        DETAIL = format('app.current_tenant=%s', v_ctx_raw);
  END;

  IF v_ctx_tenant <> p_tenant_uuid THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'RLS_TENANT_CONTEXT_MISMATCH',
      DETAIL = format('ctx=%s arg=%s', v_ctx_tenant, p_tenant_uuid);
  END IF;
END;
$$;
`

func orgunitSnapshotBootstrapTarget(args []string) {
	fs := flag.NewFlagSet("orgunit-snapshot-bootstrap-target", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var url string
	var schemaDir string
	fs.StringVar(&url, "url", "", "postgres connection string for the dedicated target database")
	fs.StringVar(&schemaDir, "schema-dir", defaultOrgunitSnapshotSchemaDir, "directory containing 00023-00025 target schema files")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	if url == "" {
		fatalf("missing --url")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	config, err := pgx.ParseConfig(url)
	if err != nil {
		fatal(err)
	}
	config.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		fatal(err)
	}
	defer conn.Close(context.Background())

	tx, err := conn.Begin(ctx)
	if err != nil {
		fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, orgunitSnapshotBootstrapPrelude); err != nil {
		fatal(err)
	}

	paths := orgunitSnapshotBootstrapPaths(schemaDir)
	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			fatal(readErr)
		}
		if _, execErr := tx.Exec(ctx, string(data)); execErr != nil {
			fatalf("apply %s: %v", path, execErr)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		fatal(err)
	}

	fmt.Printf("[orgunit-snapshot-bootstrap-target] OK applied_files=%d schema_dir=%s\n", len(paths), schemaDir)
}

func orgunitSnapshotBootstrapPaths(schemaDir string) []string {
	paths := make([]string, 0, len(orgunitSnapshotBootstrapFiles))
	for _, name := range orgunitSnapshotBootstrapFiles {
		paths = append(paths, filepath.Join(schemaDir, name))
	}
	return paths
}
