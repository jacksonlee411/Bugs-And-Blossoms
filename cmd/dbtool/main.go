package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func main() {
	if len(os.Args) < 2 {
		fatalf("usage: dbtool <rls-smoke|orgunit-smoke|jobcatalog-smoke> [args]")
	}

	switch os.Args[1] {
	case "rls-smoke":
		rlsSmoke(os.Args[2:])
	case "orgunit-smoke":
		orgunitSmoke(os.Args[2:])
	case "jobcatalog-smoke":
		jobcatalogSmoke(os.Args[2:])
	default:
		fatalf("unknown subcommand: %s", os.Args[1])
	}
}

func rlsSmoke(args []string) {
	fs := flag.NewFlagSet("rls-smoke", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var url string
	fs.StringVar(&url, "url", "", "postgres connection string")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	if url == "" {
		fatalf("missing --url")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		fatal(err)
	}
	defer conn.Close(context.Background())

	_ = tryEnsureRole(ctx, conn, "app_nobypassrls")

	tx, err := conn.Begin(ctx)
	if err != nil {
		fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	_ = trySetRole(ctx, tx, "app_nobypassrls")
	if _, err := tx.Exec(ctx, `CREATE TEMP TABLE rls_smoke (tenant_id uuid NOT NULL, val text NOT NULL);`); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `ALTER TABLE rls_smoke ENABLE ROW LEVEL SECURITY;`); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `ALTER TABLE rls_smoke FORCE ROW LEVEL SECURITY;`); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `
CREATE POLICY tenant_isolation ON rls_smoke
USING (tenant_id = public.current_tenant_id())
WITH CHECK (tenant_id = public.current_tenant_id());`); err != nil {
		fatal(err)
	}

	if _, err := tx.Exec(ctx, `SAVEPOINT sp_failclosed;`); err != nil {
		fatal(err)
	}
	_, err = tx.Exec(ctx, `SELECT count(*) FROM rls_smoke;`)
	if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_failclosed;`); rbErr != nil {
		fatal(rbErr)
	}
	if err == nil {
		fatalf("expected fail-closed error when app.current_tenant is missing")
	}

	tenantA := "00000000-0000-0000-0000-00000000000a"
	tenantB := "00000000-0000-0000-0000-00000000000b"
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantA); err != nil {
		fatal(err)
	}

	if _, err := tx.Exec(ctx, `INSERT INTO rls_smoke (tenant_id, val) VALUES ($1, 'a');`, tenantA); err != nil {
		fatal(err)
	}

	if _, err := tx.Exec(ctx, `SAVEPOINT sp_cross_insert;`); err != nil {
		fatal(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO rls_smoke (tenant_id, val) VALUES ($1, 'b');`, tenantB)
	if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_cross_insert;`); rbErr != nil {
		fatal(rbErr)
	}
	if err == nil {
		fatalf("expected RLS rejection on cross-tenant insert")
	}

	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM rls_smoke;`).Scan(&count); err != nil {
		fatal(err)
	}
	if count != 1 {
		fatalf("expected count=1 under tenant A, got %d", count)
	}

	if err := tx.Commit(ctx); err != nil {
		fatal(err)
	}

	tx2, err := conn.Begin(ctx)
	if err != nil {
		fatal(err)
	}
	defer func() { _ = tx2.Rollback(context.Background()) }()

	_ = trySetRole(ctx, tx2, "app_nobypassrls")
	if _, err := tx2.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantB); err != nil {
		fatal(err)
	}
	if err := tx2.QueryRow(ctx, `SELECT count(*) FROM rls_smoke;`).Scan(&count); err != nil {
		fatal(err)
	}
	if count != 0 {
		fatalf("expected count=0 under tenant B, got %d", count)
	}
	if _, err := tx2.Exec(ctx, `INSERT INTO rls_smoke (tenant_id, val) VALUES ($1, 'b');`, tenantB); err != nil {
		fatal(err)
	}
	if err := tx2.QueryRow(ctx, `SELECT count(*) FROM rls_smoke;`).Scan(&count); err != nil {
		fatal(err)
	}
	if count != 1 {
		fatalf("expected count=1 after insert under tenant B, got %d", count)
	}

	if err := tx2.Commit(ctx); err != nil {
		fatal(err)
	}

	fmt.Println("[rls-smoke] OK")
}

func orgunitSmoke(args []string) {
	fs := flag.NewFlagSet("orgunit-smoke", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var url string
	fs.StringVar(&url, "url", "", "postgres connection string")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	if url == "" {
		fatalf("missing --url")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		fatal(err)
	}
	defer conn.Close(context.Background())

	_ = tryEnsureRole(ctx, conn, "app_nobypassrls")

	tx, err := conn.Begin(ctx)
	if err != nil {
		fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	_ = trySetRole(ctx, tx, "app_nobypassrls")

	if _, err := tx.Exec(ctx, `SAVEPOINT sp_failclosed;`); err != nil {
		fatal(err)
	}
	_, err = tx.Exec(ctx, `SELECT count(*) FROM orgunit.org_events;`)
	if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_failclosed;`); rbErr != nil {
		fatal(rbErr)
	}
	if err == nil {
		fatalf("expected fail-closed error when app.current_tenant is missing")
	}

	tenantA := "00000000-0000-0000-0000-00000000000a"
	tenantB := "00000000-0000-0000-0000-00000000000b"
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantA); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM orgunit.org_unit_versions WHERE tenant_id = $1::uuid AND hierarchy_type = 'OrgUnit';`, tenantA); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM orgunit.org_trees WHERE tenant_id = $1::uuid AND hierarchy_type = 'OrgUnit';`, tenantA); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM orgunit.org_events WHERE tenant_id = $1::uuid AND hierarchy_type = 'OrgUnit';`, tenantA); err != nil {
		fatal(err)
	}

	var countA0 int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM orgunit.org_events WHERE hierarchy_type = 'OrgUnit';`).Scan(&countA0); err != nil {
		fatal(err)
	}

	initiatorID := "00000000-0000-0000-0000-00000000f001"
	requestID := "dbtool-orgunit-smoke-a"
	eventIDA := "00000000-0000-0000-0000-00000000a101"
	orgIDA := "00000000-0000-0000-0000-0000000000a1"

	var dbIDA int64
	if err := tx.QueryRow(ctx, `
	SELECT orgunit.submit_org_event(
	  $1::uuid,
	  $2::uuid,
	  'OrgUnit',
	  $3::uuid,
	  'CREATE',
	  $4::date,
	  jsonb_build_object('name', 'A1'),
	  $5::text,
	  $6::uuid
	)
	`, eventIDA, tenantA, orgIDA, "2026-01-01", requestID, initiatorID).Scan(&dbIDA); err != nil {
		fatal(err)
	}

	var countA1 int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM orgunit.org_events WHERE hierarchy_type = 'OrgUnit';`).Scan(&countA1); err != nil {
		fatal(err)
	}
	if countA1 != countA0+1 {
		fatalf("expected count under tenant A to increase by 1, got before=%d after=%d", countA0, countA1)
	}

	if _, err := tx.Exec(ctx, `SAVEPOINT sp_cross_event;`); err != nil {
		fatal(err)
	}
	_, err = tx.Exec(ctx, `
	SELECT orgunit.submit_org_event(
	  gen_random_uuid(),
	  $1::uuid,
	  'OrgUnit',
	  gen_random_uuid(),
	  'CREATE',
	  '2026-01-01'::date,
	  jsonb_build_object('name', 'B1'),
	  'dbtool-orgunit-smoke-b',
	  gen_random_uuid()
	)
	`, tenantB)
	if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_cross_event;`); rbErr != nil {
		fatal(rbErr)
	}
	if err == nil {
		fatalf("expected tenant mismatch on cross-tenant event")
	}

	if err := tx.Commit(ctx); err != nil {
		fatal(err)
	}

	tx2, err := conn.Begin(ctx)
	if err != nil {
		fatal(err)
	}
	defer func() { _ = tx2.Rollback(context.Background()) }()
	_ = trySetRole(ctx, tx2, "app_nobypassrls")
	if _, err := tx2.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantB); err != nil {
		fatal(err)
	}

	var visible int
	if err := tx2.QueryRow(ctx, `SELECT count(*) FROM orgunit.org_events WHERE event_id = $1::uuid;`, eventIDA).Scan(&visible); err != nil {
		fatal(err)
	}
	if visible != 0 {
		fatalf("expected event created under tenant A to be invisible under tenant B, got visible=%d event_id=%s", visible, eventIDA)
	}
	if err := tx2.Commit(ctx); err != nil {
		fatal(err)
	}

	tx3, err := conn.Begin(ctx)
	if err != nil {
		fatal(err)
	}
	defer func() { _ = tx3.Rollback(context.Background()) }()
	_ = trySetRole(ctx, tx3, "app_nobypassrls")

	tenantC := "00000000-0000-0000-0000-0000000000c1"

	if _, err := tx3.Exec(ctx, `SAVEPOINT sp_missing_ctx;`); err != nil {
		fatal(err)
	}
	_, err = tx3.Exec(ctx, `SELECT orgunit.assert_current_tenant($1::uuid);`, tenantC)
	if _, rbErr := tx3.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_missing_ctx;`); rbErr != nil {
		fatal(rbErr)
	}
	if err == nil {
		fatalf("expected orgunit.assert_current_tenant to fail when app.current_tenant is missing")
	}
	if msg, ok := pgErrorMessage(err); !ok || msg != "RLS_TENANT_CONTEXT_MISSING" {
		fatalf("expected pg error message=RLS_TENANT_CONTEXT_MISSING, got ok=%v message=%q err=%v", ok, msg, err)
	}

	if _, err := tx3.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantC); err != nil {
		fatal(err)
	}

	if _, err := tx3.Exec(ctx, `DELETE FROM orgunit.org_unit_versions WHERE tenant_id = $1::uuid AND hierarchy_type = 'OrgUnit';`, tenantC); err != nil {
		fatal(err)
	}
	if _, err := tx3.Exec(ctx, `DELETE FROM orgunit.org_trees WHERE tenant_id = $1::uuid AND hierarchy_type = 'OrgUnit';`, tenantC); err != nil {
		fatal(err)
	}
	if _, err := tx3.Exec(ctx, `DELETE FROM orgunit.org_events WHERE tenant_id = $1::uuid AND hierarchy_type = 'OrgUnit';`, tenantC); err != nil {
		fatal(err)
	}

	requestID = "dbtool-orgunit-smoke"

	orgRootID := "00000000-0000-0000-0000-000000000101"
	orgChildID := "00000000-0000-0000-0000-000000000102"
	orgParent2ID := "00000000-0000-0000-0000-000000000103"

	eventCreateRoot := "00000000-0000-0000-0000-00000000e101"
	eventCreateChild := "00000000-0000-0000-0000-00000000e102"
	eventCreateParent2 := "00000000-0000-0000-0000-00000000e103"
	eventRenameChild := "00000000-0000-0000-0000-00000000e104"
	eventMoveChild := "00000000-0000-0000-0000-00000000e105"
	eventDisableChild := "00000000-0000-0000-0000-00000000e106"

	var createRootDBID int64
	if err := tx3.QueryRow(ctx, `
SELECT orgunit.submit_org_event(
  $1::uuid,
  $2::uuid,
  'OrgUnit',
  $3::uuid,
  'CREATE',
  $4::date,
  jsonb_build_object('name', 'Root'),
  $5::text,
  $6::uuid
)
`, eventCreateRoot, tenantC, orgRootID, "2026-01-01", requestID, initiatorID).Scan(&createRootDBID); err != nil {
		fatal(err)
	}

	var createRootDBID2 int64
	if err := tx3.QueryRow(ctx, `
SELECT orgunit.submit_org_event(
  $1::uuid,
  $2::uuid,
  'OrgUnit',
  $3::uuid,
  'CREATE',
  $4::date,
  jsonb_build_object('name', 'Root'),
  $5::text,
  $6::uuid
)
`, eventCreateRoot, tenantC, orgRootID, "2026-01-01", requestID, initiatorID).Scan(&createRootDBID2); err != nil {
		fatal(err)
	}
	if createRootDBID2 != createRootDBID {
		fatalf("expected idempotent submit_org_event to return same db id, got %d then %d", createRootDBID, createRootDBID2)
	}

	if _, err := tx3.Exec(ctx, `
SELECT orgunit.submit_org_event(
  $1::uuid,
  $2::uuid,
  'OrgUnit',
  $3::uuid,
  'CREATE',
  $4::date,
  jsonb_build_object('parent_id', $5::uuid, 'name', 'Child'),
  $6::text,
  $7::uuid
)
`, eventCreateChild, tenantC, orgChildID, "2026-01-01", orgRootID, requestID, initiatorID); err != nil {
		fatal(err)
	}

	if _, err := tx3.Exec(ctx, `
SELECT orgunit.submit_org_event(
  $1::uuid,
  $2::uuid,
  'OrgUnit',
  $3::uuid,
  'CREATE',
  $4::date,
  jsonb_build_object('parent_id', $5::uuid, 'name', 'Parent2'),
  $6::text,
  $7::uuid
)
`, eventCreateParent2, tenantC, orgParent2ID, "2026-01-03", orgRootID, requestID, initiatorID); err != nil {
		fatal(err)
	}

	if _, err := tx3.Exec(ctx, `
SELECT orgunit.submit_org_event(
  $1::uuid,
  $2::uuid,
  'OrgUnit',
  $3::uuid,
  'RENAME',
  $4::date,
  jsonb_build_object('new_name', 'Child2'),
  $5::text,
  $6::uuid
)
`, eventRenameChild, tenantC, orgChildID, "2026-01-02", requestID, initiatorID); err != nil {
		fatal(err)
	}

	if _, err := tx3.Exec(ctx, `
SELECT orgunit.submit_org_event(
  $1::uuid,
  $2::uuid,
  'OrgUnit',
  $3::uuid,
  'MOVE',
  $4::date,
  jsonb_build_object('new_parent_id', $5::uuid),
  $6::text,
  $7::uuid
)
`, eventMoveChild, tenantC, orgChildID, "2026-01-04", orgParent2ID, requestID, initiatorID); err != nil {
		fatal(err)
	}

	if _, err := tx3.Exec(ctx, `
SELECT orgunit.submit_org_event(
  $1::uuid,
  $2::uuid,
  'OrgUnit',
  $3::uuid,
  'DISABLE',
  $4::date,
  '{}'::jsonb,
  $5::text,
  $6::uuid
)
`, eventDisableChild, tenantC, orgChildID, "2026-01-06", requestID, initiatorID); err != nil {
		fatal(err)
	}

	var childSlices int
	if err := tx3.QueryRow(ctx, `
SELECT count(*)
FROM orgunit.org_unit_versions
WHERE tenant_id = $1::uuid AND hierarchy_type = 'OrgUnit' AND org_id = $2::uuid
`, tenantC, orgChildID).Scan(&childSlices); err != nil {
		fatal(err)
	}
	if childSlices != 4 {
		fatalf("expected org child to have 4 version slices, got %d", childSlices)
	}

	var rootLabel, childLabel, parent2Label string
	if err := tx3.QueryRow(ctx, `SELECT orgunit.org_ltree_label($1::uuid);`, orgRootID).Scan(&rootLabel); err != nil {
		fatal(err)
	}
	if err := tx3.QueryRow(ctx, `SELECT orgunit.org_ltree_label($1::uuid);`, orgChildID).Scan(&childLabel); err != nil {
		fatal(err)
	}
	if err := tx3.QueryRow(ctx, `SELECT orgunit.org_ltree_label($1::uuid);`, orgParent2ID).Scan(&parent2Label); err != nil {
		fatal(err)
	}
	expectedPathBeforeMove := rootLabel + "." + childLabel
	expectedPathAfterMove := rootLabel + "." + parent2Label + "." + childLabel

	var name0301, status0301, parent0301, path0301 string
	if err := tx3.QueryRow(ctx, `
SELECT name, status, parent_id::text, node_path::text
FROM orgunit.org_unit_versions
WHERE tenant_id = $1::uuid
  AND hierarchy_type = 'OrgUnit'
  AND org_id = $2::uuid
  AND validity @> $3::date
`, tenantC, orgChildID, "2026-01-03").Scan(&name0301, &status0301, &parent0301, &path0301); err != nil {
		fatal(err)
	}
	if name0301 != "Child2" || status0301 != "active" || parent0301 != orgRootID || path0301 != expectedPathBeforeMove {
		fatalf("unexpected snapshot on 2026-01-03: name=%q status=%q parent=%q path=%q", name0301, status0301, parent0301, path0301)
	}

	var name0501, status0501, parent0501, path0501 string
	if err := tx3.QueryRow(ctx, `
SELECT name, status, parent_id::text, node_path::text
FROM orgunit.org_unit_versions
WHERE tenant_id = $1::uuid
  AND hierarchy_type = 'OrgUnit'
  AND org_id = $2::uuid
  AND validity @> $3::date
`, tenantC, orgChildID, "2026-01-05").Scan(&name0501, &status0501, &parent0501, &path0501); err != nil {
		fatal(err)
	}
	if name0501 != "Child2" || status0501 != "active" || parent0501 != orgParent2ID || path0501 != expectedPathAfterMove {
		fatalf("unexpected snapshot on 2026-01-05: name=%q status=%q parent=%q path=%q", name0501, status0501, parent0501, path0501)
	}

	var status0701 string
	if err := tx3.QueryRow(ctx, `
SELECT status
FROM orgunit.org_unit_versions
WHERE tenant_id = $1::uuid
  AND hierarchy_type = 'OrgUnit'
  AND org_id = $2::uuid
  AND validity @> $3::date
`, tenantC, orgChildID, "2026-01-07").Scan(&status0701); err != nil {
		fatal(err)
	}
	if status0701 != "disabled" {
		fatalf("expected snapshot status on 2026-01-07 to be disabled, got %q", status0701)
	}

	if _, err := tx3.Exec(ctx, `SAVEPOINT sp_tenant_mismatch;`); err != nil {
		fatal(err)
	}
	_, err = tx3.Exec(ctx, `
SELECT orgunit.submit_org_event(
  $1::uuid,
  $2::uuid,
  'OrgUnit',
  $3::uuid,
  'CREATE',
  $4::date,
  jsonb_build_object('name', 'X'),
  $5::text,
  $6::uuid
)
`, "00000000-0000-0000-0000-00000000e1ff", tenantB, "00000000-0000-0000-0000-0000000001ff", "2026-01-01", requestID, initiatorID)
	if _, rbErr := tx3.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_tenant_mismatch;`); rbErr != nil {
		fatal(rbErr)
	}
	if err == nil {
		fatalf("expected tenant mismatch when orgunit.submit_org_event tenant_id differs from app.current_tenant")
	}
	if msg, ok := pgErrorMessage(err); !ok || msg != "RLS_TENANT_MISMATCH" {
		fatalf("expected pg error message=RLS_TENANT_MISMATCH, got ok=%v message=%q err=%v", ok, msg, err)
	}

	fmt.Println("[orgunit-smoke] OK")
}

func jobcatalogSmoke(args []string) {
	fs := flag.NewFlagSet("jobcatalog-smoke", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var url string
	fs.StringVar(&url, "url", "", "postgres connection string")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	if url == "" {
		fatalf("missing --url")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		fatal(err)
	}
	defer conn.Close(context.Background())

	_ = tryEnsureRole(ctx, conn, "app_nobypassrls")

	tx, err := conn.Begin(ctx)
	if err != nil {
		fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	_ = trySetRole(ctx, tx, "app_nobypassrls")

	if _, err := tx.Exec(ctx, `SAVEPOINT sp_failclosed;`); err != nil {
		fatal(err)
	}
	_, err = tx.Exec(ctx, `SELECT count(*) FROM jobcatalog.job_family_group_events;`)
	if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_failclosed;`); rbErr != nil {
		fatal(rbErr)
	}
	if err == nil {
		fatalf("expected fail-closed error when app.current_tenant is missing")
	}

	tenantA := "00000000-0000-0000-0000-00000000000a"
	tenantB := "00000000-0000-0000-0000-00000000000b"
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantA); err != nil {
		fatal(err)
	}

	// SetID bootstrap is part of 009M1 dependency chain; smoke uses it to avoid hidden coupling.
	_, _ = tx.Exec(ctx, `SELECT orgunit.ensure_setid_bootstrap($1::uuid, $1::uuid);`, tenantA)

	if _, err := tx.Exec(ctx, `DELETE FROM jobcatalog.job_family_group_versions WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM jobcatalog.job_family_group_events WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM jobcatalog.job_family_groups WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}

	groupID := "00000000-0000-0000-0000-00000000c101"
	eventID := "00000000-0000-0000-0000-00000000c102"
	requestID := "dbtool-jobcatalog-smoke-create"
	initiatorID := "00000000-0000-0000-0000-00000000f001"

	if _, err := tx.Exec(ctx, `
SELECT jobcatalog.submit_job_family_group_event(
  $1::uuid,
  $2::uuid,
  'SHARE',
  $3::uuid,
  'CREATE',
  $4::date,
  jsonb_build_object('code', 'JC1', 'name', 'Job Family Group 1', 'description', null),
  $5::text,
  $6::uuid
);
`, eventID, tenantA, groupID, "2026-01-01", requestID, initiatorID); err != nil {
		fatal(err)
	}

	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM jobcatalog.job_family_group_versions WHERE validity @> '2026-01-01'::date;`).Scan(&count); err != nil {
		fatal(err)
	}
	if count != 1 {
		fatalf("expected versions count=1 under tenant A, got %d", count)
	}

	if err := tx.Commit(ctx); err != nil {
		fatal(err)
	}

	tx2, err := conn.Begin(ctx)
	if err != nil {
		fatal(err)
	}
	defer func() { _ = tx2.Rollback(context.Background()) }()

	_ = trySetRole(ctx, tx2, "app_nobypassrls")
	if _, err := tx2.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantB); err != nil {
		fatal(err)
	}
	if err := tx2.QueryRow(ctx, `SELECT count(*) FROM jobcatalog.job_family_group_versions;`).Scan(&count); err != nil {
		fatal(err)
	}
	if count != 0 {
		fatalf("expected versions count=0 under tenant B, got %d", count)
	}
	if err := tx2.Commit(ctx); err != nil {
		fatal(err)
	}

	fmt.Println("[jobcatalog-smoke] OK")
}

func pgErrorMessage(err error) (string, bool) {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return "", false
	}
	return pgErr.Message, true
}

func tryEnsureRole(ctx context.Context, conn *pgx.Conn, role string) error {
	if !validSQLIdent(role) {
		return fmt.Errorf("invalid role: %s", role)
	}

	stmt := fmt.Sprintf(`DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '%s') THEN
    EXECUTE 'CREATE ROLE %s NOBYPASSRLS';
  END IF;
END
$$;`, role, role)
	if _, err := conn.Exec(ctx, stmt); err != nil {
		return err
	}
	_, _ = conn.Exec(ctx, `GRANT USAGE ON SCHEMA public TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT USAGE ON SCHEMA iam TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT USAGE ON SCHEMA orgunit TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT USAGE ON SCHEMA jobcatalog TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA iam TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA orgunit TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA jobcatalog TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA iam TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA orgunit TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA jobcatalog TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA iam TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA orgunit TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA jobcatalog TO `+role+`;`)
	_, _ = conn.Exec(ctx, `ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO `+role+`;`)
	_, _ = conn.Exec(ctx, `ALTER DEFAULT PRIVILEGES IN SCHEMA iam GRANT USAGE, SELECT ON SEQUENCES TO `+role+`;`)
	_, _ = conn.Exec(ctx, `ALTER DEFAULT PRIVILEGES IN SCHEMA orgunit GRANT USAGE, SELECT ON SEQUENCES TO `+role+`;`)
	_, _ = conn.Exec(ctx, `ALTER DEFAULT PRIVILEGES IN SCHEMA jobcatalog GRANT USAGE, SELECT ON SEQUENCES TO `+role+`;`)
	return nil
}

func trySetRole(ctx context.Context, tx pgx.Tx, role string) bool {
	if _, err := tx.Exec(ctx, `SET ROLE `+role+`;`); err != nil {
		return false
	}
	return true
}

var reSQLIdent = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func validSQLIdent(s string) bool {
	return reSQLIdent.MatchString(s)
}

func fatal(err error) {
	if err == nil {
		os.Exit(1)
	}
	fatalf("%v", err)
}

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
