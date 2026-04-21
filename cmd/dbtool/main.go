package main

import (
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/uuidv7"
)

func main() {
	if len(os.Args) < 2 {
		fatalf("usage: dbtool <rls-smoke|orgunit-smoke|orgunit-code-validate|orgunit-snapshot-export|orgunit-snapshot-check|orgunit-snapshot-bootstrap-target|orgunit-snapshot-import|orgunit-snapshot-verify> [args]")
	}

	switch os.Args[1] {
	case "rls-smoke":
		rlsSmoke(os.Args[2:])
	case "orgunit-smoke":
		orgunitSmoke(os.Args[2:])
	case "orgunit-code-validate":
		orgunitCodeValidate(os.Args[2:])
	case "orgunit-snapshot-export":
		orgunitSnapshotExport(os.Args[2:])
	case "orgunit-snapshot-check":
		orgunitSnapshotCheck(os.Args[2:])
	case "orgunit-snapshot-bootstrap-target":
		orgunitSnapshotBootstrapTarget(os.Args[2:])
	case "orgunit-snapshot-import":
		orgunitSnapshotImport(os.Args[2:])
	case "orgunit-snapshot-verify":
		orgunitSnapshotVerify(os.Args[2:])
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
	if _, err := tx.Exec(ctx, `CREATE TEMP TABLE rls_smoke (tenant_uuid uuid NOT NULL, val text NOT NULL);`); err != nil {
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
USING (tenant_uuid = public.current_tenant_id())
WITH CHECK (tenant_uuid = public.current_tenant_id());`); err != nil {
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

	if _, err := tx.Exec(ctx, `INSERT INTO rls_smoke (tenant_uuid, val) VALUES ($1, 'a');`, tenantA); err != nil {
		fatal(err)
	}

	if _, err := tx.Exec(ctx, `SAVEPOINT sp_cross_insert;`); err != nil {
		fatal(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO rls_smoke (tenant_uuid, val) VALUES ($1, 'b');`, tenantB)
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
	if _, err := tx2.Exec(ctx, `INSERT INTO rls_smoke (tenant_uuid, val) VALUES ($1, 'b');`, tenantB); err != nil {
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
	if _, err := tx.Exec(ctx, `DELETE FROM orgunit.org_unit_versions WHERE tenant_uuid = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM orgunit.org_trees WHERE tenant_uuid = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM orgunit.org_events WHERE tenant_uuid = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}

	var countA0 int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM orgunit.org_events;`).Scan(&countA0); err != nil {
		fatal(err)
	}

	initiatorID := "00000000-0000-0000-0000-00000000f001"
	requestID := "dbtool-orgunit-smoke-a"
	eventIDA := mustUUIDv7()
	var orgIDA string

	var dbIDA int64
	if err := tx.QueryRow(ctx, `
	SELECT orgunit.submit_org_event(
	  $1::uuid,
	  $2::uuid,
	  $3::char(8),
	  'CREATE',
	  $4::date,
	  jsonb_build_object('name', 'A1'),
		  $5::text,
	  $6::uuid
	)
	`, eventIDA, tenantA, nil, "2026-01-01", requestID, initiatorID).Scan(&dbIDA); err != nil {
		fatal(err)
	}
	if err := tx.QueryRow(ctx, `
		SELECT org_node_key::text
		FROM orgunit.org_events
		WHERE tenant_uuid = $1::uuid AND event_uuid = $2::uuid
	`, tenantA, eventIDA).Scan(&orgIDA); err != nil {
		fatal(err)
	}

	var countA1 int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM orgunit.org_events;`).Scan(&countA1); err != nil {
		fatal(err)
	}
	if countA1 != countA0+1 {
		fatalf("expected count under tenant A to increase by 1, got before=%d after=%d", countA0, countA1)
	}

	if _, err := tx.Exec(ctx, `SAVEPOINT sp_cross_event;`); err != nil {
		fatal(err)
	}
	crossEventID := mustUUIDv7()
	_, err = tx.Exec(ctx, `
	SELECT orgunit.submit_org_event(
	  $1::uuid,
	  $2::uuid,
	  $3::char(8),
	  'CREATE',
	  '2026-01-01'::date,
	  jsonb_build_object('name', 'B1'),
	  'dbtool-orgunit-smoke-b',
	  $4::uuid
	)
	`, crossEventID, tenantB, orgIDA, initiatorID)
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
	if err := tx2.QueryRow(ctx, `SELECT count(*) FROM orgunit.org_events WHERE event_uuid = $1::uuid;`, eventIDA).Scan(&visible); err != nil {
		fatal(err)
	}
	if visible != 0 {
		fatalf("expected event created under tenant A to be invisible under tenant B, got visible=%d event_uuid=%s", visible, eventIDA)
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

	if _, err := tx3.Exec(ctx, `DELETE FROM orgunit.org_unit_versions WHERE tenant_uuid = $1::uuid;`, tenantC); err != nil {
		fatal(err)
	}
	if _, err := tx3.Exec(ctx, `DELETE FROM orgunit.org_trees WHERE tenant_uuid = $1::uuid;`, tenantC); err != nil {
		fatal(err)
	}
	if _, err := tx3.Exec(ctx, `DELETE FROM orgunit.org_events WHERE tenant_uuid = $1::uuid;`, tenantC); err != nil {
		fatal(err)
	}

	requestID = "dbtool-orgunit-smoke-root"

	var orgRootID string
	var orgChildID string
	var orgParent2ID string

	eventCreateRoot := mustUUIDv7()
	eventCreateChild := mustUUIDv7()
	eventCreateParent2 := mustUUIDv7()
	eventRenameChild := mustUUIDv7()
	eventMoveChild := mustUUIDv7()
	eventDisableChild := mustUUIDv7()

	var createRootDBID int64
	if err := tx3.QueryRow(ctx, `
	SELECT orgunit.submit_org_event(
	  $1::uuid,
	  $2::uuid,
	  $3::char(8),
	  'CREATE',
	  $4::date,
	  jsonb_build_object('name', 'Root'),
	  $5::text,
  $6::uuid
)
`, eventCreateRoot, tenantC, nil, "2026-01-01", requestID, initiatorID).Scan(&createRootDBID); err != nil {
		fatal(err)
	}
	if err := tx3.QueryRow(ctx, `
		SELECT org_node_key::text
		FROM orgunit.org_events
		WHERE tenant_uuid = $1::uuid AND event_uuid = $2::uuid
	`, tenantC, eventCreateRoot).Scan(&orgRootID); err != nil {
		fatal(err)
	}

	var createRootDBID2 int64
	if err := tx3.QueryRow(ctx, `
	SELECT orgunit.submit_org_event(
	  $1::uuid,
	  $2::uuid,
	  $3::char(8),
	  'CREATE',
	  $4::date,
	  jsonb_build_object('name', 'Root'),
	  $5::text,
  $6::uuid
)
`, eventCreateRoot, tenantC, nil, "2026-01-01", requestID, initiatorID).Scan(&createRootDBID2); err != nil {
		fatal(err)
	}
	if createRootDBID2 != createRootDBID {
		fatalf("expected idempotent submit_org_event to return same db id, got %d then %d", createRootDBID, createRootDBID2)
	}

	requestID = "dbtool-orgunit-smoke-child"
	if _, err := tx3.Exec(ctx, `
	SELECT orgunit.submit_org_event(
	  $1::uuid,
	  $2::uuid,
	  $3::char(8),
	  'CREATE',
	  $4::date,
	  jsonb_build_object('parent_org_node_key', $5::char(8), 'name', 'Child'),
	  $6::text,
  $7::uuid
)
`, eventCreateChild, tenantC, nil, "2026-01-01", orgRootID, requestID, initiatorID); err != nil {
		fatal(err)
	}
	if err := tx3.QueryRow(ctx, `
		SELECT org_node_key::text
		FROM orgunit.org_events
		WHERE tenant_uuid = $1::uuid AND event_uuid = $2::uuid
	`, tenantC, eventCreateChild).Scan(&orgChildID); err != nil {
		fatal(err)
	}

	requestID = "dbtool-orgunit-smoke-parent2"
	if _, err := tx3.Exec(ctx, `
	SELECT orgunit.submit_org_event(
	  $1::uuid,
	  $2::uuid,
	  $3::char(8),
	  'CREATE',
	  $4::date,
	  jsonb_build_object('parent_org_node_key', $5::char(8), 'name', 'Parent2'),
	  $6::text,
  $7::uuid
)
`, eventCreateParent2, tenantC, nil, "2026-01-03", orgRootID, requestID, initiatorID); err != nil {
		fatal(err)
	}
	if err := tx3.QueryRow(ctx, `
		SELECT org_node_key::text
		FROM orgunit.org_events
		WHERE tenant_uuid = $1::uuid AND event_uuid = $2::uuid
	`, tenantC, eventCreateParent2).Scan(&orgParent2ID); err != nil {
		fatal(err)
	}

	requestID = "dbtool-orgunit-smoke-rename"
	if _, err := tx3.Exec(ctx, `
	SELECT orgunit.submit_org_event(
	  $1::uuid,
	  $2::uuid,
	  $3::char(8),
	  'RENAME',
	  $4::date,
	  jsonb_build_object('new_name', 'Child2'),
	  $5::text,
  $6::uuid
)
`, eventRenameChild, tenantC, orgChildID, "2026-01-02", requestID, initiatorID); err != nil {
		fatal(err)
	}

	requestID = "dbtool-orgunit-smoke-move"
	if _, err := tx3.Exec(ctx, `
	SELECT orgunit.submit_org_event(
	  $1::uuid,
	  $2::uuid,
	  $3::char(8),
	  'MOVE',
	  $4::date,
	  jsonb_build_object('new_parent_org_node_key', $5::char(8)),
	  $6::text,
  $7::uuid
)
`, eventMoveChild, tenantC, orgChildID, "2026-01-04", orgParent2ID, requestID, initiatorID); err != nil {
		fatal(err)
	}

	requestID = "dbtool-orgunit-smoke-disable"
	if _, err := tx3.Exec(ctx, `
	SELECT orgunit.submit_org_event(
	  $1::uuid,
	  $2::uuid,
	  $3::char(8),
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
WHERE tenant_uuid = $1::uuid AND org_node_key = $2::char(8)
`, tenantC, orgChildID).Scan(&childSlices); err != nil {
		fatal(err)
	}
	if childSlices != 4 {
		fatalf("expected org child to have 4 version slices, got %d", childSlices)
	}

	var rootLabel, childLabel, parent2Label string
	if err := tx3.QueryRow(ctx, `SELECT orgunit.org_ltree_label($1::text);`, orgRootID).Scan(&rootLabel); err != nil {
		fatal(err)
	}
	if err := tx3.QueryRow(ctx, `SELECT orgunit.org_ltree_label($1::text);`, orgChildID).Scan(&childLabel); err != nil {
		fatal(err)
	}
	if err := tx3.QueryRow(ctx, `SELECT orgunit.org_ltree_label($1::text);`, orgParent2ID).Scan(&parent2Label); err != nil {
		fatal(err)
	}
	expectedPathBeforeMove := rootLabel + "." + childLabel
	expectedPathAfterMove := rootLabel + "." + parent2Label + "." + childLabel

	var name0301, status0301, parent0301, path0301 string
	if err := tx3.QueryRow(ctx, `
SELECT name, status, parent_org_node_key::text, node_path::text
FROM orgunit.org_unit_versions
WHERE tenant_uuid = $1::uuid
  AND org_node_key = $2::char(8)
  AND validity @> $3::date
`, tenantC, orgChildID, "2026-01-03").Scan(&name0301, &status0301, &parent0301, &path0301); err != nil {
		fatal(err)
	}
	if name0301 != "Child2" || status0301 != "active" || parent0301 != orgRootID || path0301 != expectedPathBeforeMove {
		fatalf("unexpected snapshot on 2026-01-03: name=%q status=%q parent=%q path=%q", name0301, status0301, parent0301, path0301)
	}

	var name0501, status0501, parent0501, path0501 string
	if err := tx3.QueryRow(ctx, `
SELECT name, status, parent_org_node_key::text, node_path::text
FROM orgunit.org_unit_versions
WHERE tenant_uuid = $1::uuid
  AND org_node_key = $2::char(8)
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
WHERE tenant_uuid = $1::uuid
  AND org_node_key = $2::char(8)
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
	tenantMismatchEventID := mustUUIDv7()
	requestID = "dbtool-orgunit-smoke-tenant-mismatch"
	_, err = tx3.Exec(ctx, `
	SELECT orgunit.submit_org_event(
	  $1::uuid,
	  $2::uuid,
	  $3::char(8),
	  'CREATE',
	  $4::date,
	  jsonb_build_object('name', 'X'),
	  $5::text,
  $6::uuid
)
`, tenantMismatchEventID, tenantB, orgRootID, "2026-01-01", requestID, initiatorID)
	if _, rbErr := tx3.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_tenant_mismatch;`); rbErr != nil {
		fatal(rbErr)
	}
	if err == nil {
		fatalf("expected tenant mismatch when orgunit.submit_org_event tenant_uuid differs from app.current_tenant")
	}
	if msg, ok := pgErrorMessage(err); !ok || msg != "RLS_TENANT_MISMATCH" {
		fatalf("expected pg error message=RLS_TENANT_MISMATCH, got ok=%v message=%q err=%v", ok, msg, err)
	}

	fmt.Println("[orgunit-smoke] OK")
}

type orgunitCodeRow struct {
	line          int
	orgID         int
	rawCode       string
	normalized    string
	alreadyMapped bool
}

type orgunitCodeConflict struct {
	line       int
	orgID      int
	rawCode    string
	normalized string
	reason     string
	detail     string
}

func orgunitCodeValidate(args []string) {
	fs := flag.NewFlagSet("orgunit-code-validate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var url string
	var tenantUUID string
	var inputPath string
	var conflictOutPath string
	var normalizedOutPath string
	fs.StringVar(&url, "url", "", "postgres connection string")
	fs.StringVar(&tenantUUID, "tenant", "", "tenant uuid")
	fs.StringVar(&inputPath, "input", "", "csv path (org_id,org_code)")
	fs.StringVar(&conflictOutPath, "out", "", "conflict csv output (default stdout)")
	fs.StringVar(&normalizedOutPath, "normalized-out", "", "normalized mapping csv output when no conflicts")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	if url == "" {
		fatalf("missing --url")
	}
	if tenantUUID == "" {
		fatalf("missing --tenant")
	}
	if inputPath == "" {
		fatalf("missing --input")
	}

	rows, conflicts := readOrgunitCodeCSV(inputPath)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantUUID); err != nil {
		fatal(err)
	}

	existingCodes := make(map[string]int)
	existingOrgIDs := make(map[int]string)
	rowsResult, err := tx.Query(ctx, `
SELECT org_id, org_code
FROM orgunit.org_unit_codes
WHERE tenant_uuid = $1::uuid
`, tenantUUID)
	if err != nil {
		fatal(err)
	}
	for rowsResult.Next() {
		var orgID int
		var orgCode string
		if err := rowsResult.Scan(&orgID, &orgCode); err != nil {
			fatal(err)
		}
		existingCodes[orgCode] = orgID
		existingOrgIDs[orgID] = orgCode
	}
	rowsResult.Close()
	if rowsResult.Err() != nil {
		fatal(rowsResult.Err())
	}

	existingOrgSet := make(map[int]struct{})
	orgRows, err := tx.Query(ctx, `
SELECT DISTINCT org_id
FROM orgunit.org_unit_versions
WHERE tenant_uuid = $1::uuid
`, tenantUUID)
	if err != nil {
		fatal(err)
	}
	for orgRows.Next() {
		var orgID int
		if err := orgRows.Scan(&orgID); err != nil {
			fatal(err)
		}
		existingOrgSet[orgID] = struct{}{}
	}
	orgRows.Close()
	if orgRows.Err() != nil {
		fatal(orgRows.Err())
	}

	validRows, rowConflicts := validateOrgunitCodeRows(rows, existingOrgSet, existingCodes, existingOrgIDs)
	conflicts = append(conflicts, rowConflicts...)

	var conflictOut io.Writer = os.Stdout
	var conflictFile *os.File
	if conflictOutPath != "" {
		file, err := os.Create(conflictOutPath)
		if err != nil {
			fatal(err)
		}
		conflictFile = file
		conflictOut = file
	}
	if conflictFile != nil {
		defer conflictFile.Close()
	}

	if len(conflicts) > 0 {
		if err := writeOrgunitCodeConflicts(conflictOut, conflicts); err != nil {
			fatal(err)
		}
		fatalf("orgunit-code-validate: conflicts=%d (see output)", len(conflicts))
	}
	if conflictOutPath != "" {
		if err := writeOrgunitCodeConflicts(conflictOut, nil); err != nil {
			fatal(err)
		}
	}
	if normalizedOutPath != "" {
		if err := writeOrgunitCodeNormalized(normalizedOutPath, validRows); err != nil {
			fatal(err)
		}
	}

	fmt.Fprintf(os.Stderr, "orgunit-code-validate: ok rows=%d already_mapped=%d\n", len(validRows), countAlreadyMapped(validRows))
}

func readOrgunitCodeCSV(path string) ([]orgunitCodeRow, []orgunitCodeConflict) {
	file, err := os.Open(path)
	if err != nil {
		fatal(err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = false

	var rows []orgunitCodeRow
	var conflicts []orgunitCodeConflict
	line := 0
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		line++
		if err != nil {
			conflicts = append(conflicts, orgunitCodeConflict{
				line:   line,
				reason: "csv_read_error",
				detail: err.Error(),
			})
			continue
		}
		if len(record) == 0 {
			continue
		}
		if line == 1 && isOrgunitCodeHeader(record) {
			continue
		}
		if len(record) < 2 {
			conflicts = append(conflicts, orgunitCodeConflict{
				line:   line,
				reason: "csv_columns",
				detail: "expected org_id,org_code",
			})
			continue
		}

		orgIDRaw := strings.TrimSpace(record[0])
		orgCodeRaw := record[1]
		orgID, err := strconv.Atoi(orgIDRaw)
		if err != nil {
			conflicts = append(conflicts, orgunitCodeConflict{
				line:    line,
				rawCode: orgCodeRaw,
				reason:  "org_id_invalid",
				detail:  fmt.Sprintf("org_id=%s", orgIDRaw),
			})
			rows = append(rows, orgunitCodeRow{line: line, rawCode: orgCodeRaw})
			continue
		}
		if orgID < 10000000 || orgID > 99999999 {
			conflicts = append(conflicts, orgunitCodeConflict{
				line:    line,
				orgID:   orgID,
				rawCode: orgCodeRaw,
				reason:  "org_id_out_of_range",
				detail:  fmt.Sprintf("org_id=%d", orgID),
			})
		}
		normalized, err := orgunit.NormalizeOrgCode(orgCodeRaw)
		if err != nil {
			conflicts = append(conflicts, orgunitCodeConflict{
				line:    line,
				orgID:   orgID,
				rawCode: orgCodeRaw,
				reason:  "org_code_invalid",
				detail:  err.Error(),
			})
			rows = append(rows, orgunitCodeRow{line: line, orgID: orgID, rawCode: orgCodeRaw})
			continue
		}

		rows = append(rows, orgunitCodeRow{
			line:       line,
			orgID:      orgID,
			rawCode:    orgCodeRaw,
			normalized: normalized,
		})
	}
	return rows, conflicts
}

func validateOrgunitCodeRows(
	rows []orgunitCodeRow,
	existingOrgSet map[int]struct{},
	existingCodes map[string]int,
	existingOrgIDs map[int]string,
) ([]orgunitCodeRow, []orgunitCodeConflict) {
	conflictKey := make(map[string]struct{})
	conflicts := make([]orgunitCodeConflict, 0)
	addConflict := func(conflict orgunitCodeConflict) {
		key := fmt.Sprintf("%d:%s", conflict.line, conflict.reason)
		if _, ok := conflictKey[key]; ok {
			return
		}
		conflicts = append(conflicts, conflict)
		conflictKey[key] = struct{}{}
	}

	codeSeen := make(map[string]orgunitCodeRow)
	orgSeen := make(map[int]orgunitCodeRow)
	for _, row := range rows {
		if row.normalized == "" || row.orgID == 0 {
			continue
		}
		if prev, ok := codeSeen[row.normalized]; ok {
			addConflict(orgunitCodeConflict{
				line:       row.line,
				orgID:      row.orgID,
				rawCode:    row.rawCode,
				normalized: row.normalized,
				reason:     "org_code_duplicate_input",
				detail:     fmt.Sprintf("first_line=%d", prev.line),
			})
			addConflict(orgunitCodeConflict{
				line:       prev.line,
				orgID:      prev.orgID,
				rawCode:    prev.rawCode,
				normalized: prev.normalized,
				reason:     "org_code_duplicate_input",
				detail:     fmt.Sprintf("duplicate_line=%d", row.line),
			})
		} else {
			codeSeen[row.normalized] = row
		}
		if prev, ok := orgSeen[row.orgID]; ok {
			addConflict(orgunitCodeConflict{
				line:       row.line,
				orgID:      row.orgID,
				rawCode:    row.rawCode,
				normalized: row.normalized,
				reason:     "org_id_duplicate_input",
				detail:     fmt.Sprintf("first_line=%d", prev.line),
			})
			addConflict(orgunitCodeConflict{
				line:       prev.line,
				orgID:      prev.orgID,
				rawCode:    prev.rawCode,
				normalized: prev.normalized,
				reason:     "org_id_duplicate_input",
				detail:     fmt.Sprintf("duplicate_line=%d", row.line),
			})
		} else {
			orgSeen[row.orgID] = row
		}
	}

	validRows := make([]orgunitCodeRow, 0, len(rows))
	for _, row := range rows {
		if row.normalized == "" || row.orgID == 0 {
			continue
		}
		if _, ok := existingOrgSet[row.orgID]; !ok {
			addConflict(orgunitCodeConflict{
				line:       row.line,
				orgID:      row.orgID,
				rawCode:    row.rawCode,
				normalized: row.normalized,
				reason:     "org_id_missing_db",
				detail:     "org_id not found in org_unit_versions",
			})
			continue
		}
		if existingOrgCode, ok := existingOrgIDs[row.orgID]; ok && existingOrgCode != row.normalized {
			addConflict(orgunitCodeConflict{
				line:       row.line,
				orgID:      row.orgID,
				rawCode:    row.rawCode,
				normalized: row.normalized,
				reason:     "org_id_conflict_db",
				detail:     fmt.Sprintf("existing_org_code=%s", existingOrgCode),
			})
		}
		if existingOrgID, ok := existingCodes[row.normalized]; ok && existingOrgID != row.orgID {
			addConflict(orgunitCodeConflict{
				line:       row.line,
				orgID:      row.orgID,
				rawCode:    row.rawCode,
				normalized: row.normalized,
				reason:     "org_code_conflict_db",
				detail:     fmt.Sprintf("existing_org_id=%d", existingOrgID),
			})
		}
		if existingOrgCode, ok := existingOrgIDs[row.orgID]; ok && existingOrgCode == row.normalized {
			row.alreadyMapped = true
		}
		if existingOrgID, ok := existingCodes[row.normalized]; ok && existingOrgID == row.orgID {
			row.alreadyMapped = true
		}
		validRows = append(validRows, row)
	}
	return validRows, conflicts
}

func isOrgunitCodeHeader(record []string) bool {
	if len(record) < 2 {
		return false
	}
	first := strings.TrimSpace(strings.ToLower(record[0]))
	second := strings.TrimSpace(strings.ToLower(record[1]))
	return first == "org_id" && second == "org_code"
}

func writeOrgunitCodeConflicts(out io.Writer, conflicts []orgunitCodeConflict) error {
	writer := csv.NewWriter(out)
	if err := writer.Write([]string{"line", "org_id", "org_code", "org_code_normalized", "reason", "detail"}); err != nil {
		return err
	}
	for _, conflict := range conflicts {
		row := []string{
			strconv.Itoa(conflict.line),
			strconv.Itoa(conflict.orgID),
			conflict.rawCode,
			conflict.normalized,
			conflict.reason,
			conflict.detail,
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func writeOrgunitCodeNormalized(path string, rows []orgunitCodeRow) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	if err := writer.Write([]string{"org_id", "org_code"}); err != nil {
		return err
	}
	for _, row := range rows {
		if row.alreadyMapped {
			continue
		}
		if err := writer.Write([]string{strconv.Itoa(row.orgID), row.normalized}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func countAlreadyMapped(rows []orgunitCodeRow) int {
	count := 0
	for _, row := range rows {
		if row.alreadyMapped {
			count++
		}
	}
	return count
}

func pgErrorMessage(err error) (string, bool) {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	if !ok || pgErr == nil {
		return "", false
	}
	return pgErr.Message, true
}

func pgErrorConstraintName(err error) (string, bool) {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	if !ok || pgErr == nil {
		return "", false
	}
	if pgErr.ConstraintName == "" {
		return "", false
	}
	return pgErr.ConstraintName, true
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
	_, _ = conn.Exec(ctx, `GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA iam TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA orgunit TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA iam TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA orgunit TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA iam TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA orgunit TO `+role+`;`)
	_, _ = conn.Exec(ctx, `ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO `+role+`;`)
	_, _ = conn.Exec(ctx, `ALTER DEFAULT PRIVILEGES IN SCHEMA iam GRANT USAGE, SELECT ON SEQUENCES TO `+role+`;`)
	_, _ = conn.Exec(ctx, `ALTER DEFAULT PRIVILEGES IN SCHEMA orgunit GRANT USAGE, SELECT ON SEQUENCES TO `+role+`;`)
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

func mustUUIDv7() string {
	id, err := uuidv7.NewString()
	if err != nil {
		fatal(err)
	}
	return id
}

func fatal(err error) {
	if err == nil {
		os.Exit(1)
	}
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		if pgErr.Detail != "" {
			_, _ = fmt.Fprintf(os.Stderr, "DETAIL: %s\n", pgErr.Detail)
		}
		if pgErr.Where != "" {
			_, _ = fmt.Fprintf(os.Stderr, "WHERE: %s\n", pgErr.Where)
		}
		os.Exit(1)
	}
	fatalf("%v", err)
}

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
