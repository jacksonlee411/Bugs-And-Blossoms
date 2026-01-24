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
		fatalf("usage: dbtool <rls-smoke|orgunit-smoke|jobcatalog-smoke|person-smoke|staffing-smoke> [args]")
	}

	switch os.Args[1] {
	case "rls-smoke":
		rlsSmoke(os.Args[2:])
	case "orgunit-smoke":
		orgunitSmoke(os.Args[2:])
	case "jobcatalog-smoke":
		jobcatalogSmoke(os.Args[2:])
	case "person-smoke":
		personSmoke(os.Args[2:])
	case "staffing-smoke":
		staffingSmoke(os.Args[2:])
	default:
		fatalf("unknown subcommand: %s", os.Args[1])
	}
}

func personSmoke(args []string) {
	fs := flag.NewFlagSet("person-smoke", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var url string
	fs.StringVar(&url, "url", "", "postgres connection string")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	if url == "" {
		fatalf("missing --url")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	_, err = tx.Exec(ctx, `SELECT count(*) FROM person.persons;`)
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

	if _, err := tx.Exec(ctx, `DELETE FROM person.persons WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}

	var personUUID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&personUUID); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO person.persons (tenant_id, person_uuid, pernr, display_name, status)
		VALUES ($1::uuid, $2::uuid, '1', 'Smoke Person', 'active');
	`, tenantA, personUUID); err != nil {
		fatal(err)
	}

	if _, err := tx.Exec(ctx, `SAVEPOINT sp_cross_insert;`); err != nil {
		fatal(err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO person.persons (tenant_id, pernr, display_name, status)
		VALUES ($1::uuid, '2', 'Cross Tenant', 'active');
	`, tenantB)
	if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_cross_insert;`); rbErr != nil {
		fatal(rbErr)
	}
	if err == nil {
		fatalf("expected RLS rejection on cross-tenant insert")
	}

	var countA int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM person.persons WHERE tenant_id = $1::uuid;`, tenantA).Scan(&countA); err != nil {
		fatal(err)
	}
	if countA != 1 {
		fatalf("expected count=1 under tenant A, got %d", countA)
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
	var countB int
	if err := tx2.QueryRow(ctx, `SELECT count(*) FROM person.persons;`).Scan(&countB); err != nil {
		fatal(err)
	}
	if countB != 0 {
		fatalf("expected count=0 under tenant B, got %d", countB)
	}
	if err := tx2.Commit(ctx); err != nil {
		fatal(err)
	}

	fmt.Println("[person-smoke] OK")
}

func staffingSmoke(args []string) {
	fs := flag.NewFlagSet("staffing-smoke", flag.ContinueOnError)
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

	tenantA := "00000000-0000-0000-0000-00000000000a"
	tenantB := "00000000-0000-0000-0000-00000000000b"

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantA); err != nil {
		fatal(err)
	}
	var seedPositionID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&seedPositionID); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO staffing.positions (tenant_id, id)
		VALUES ($1::uuid, $2::uuid);
	`, tenantA, seedPositionID); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO staffing.position_events (tenant_id, position_id, event_type, effective_date, payload, request_id, initiator_id)
		VALUES ($1::uuid, $2::uuid, 'CREATE', '2026-01-01'::date, '{}'::jsonb, $3, $2::uuid);
	`, tenantA, seedPositionID, "dbtool-seed-"+seedPositionID); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `RESET app.current_tenant;`); err != nil {
		fatal(err)
	}

	if _, err := tx.Exec(ctx, `SAVEPOINT sp_failclosed;`); err != nil {
		fatal(err)
	}
	_, err = tx.Exec(ctx, `SELECT count(*) FROM staffing.position_events;`)
	if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_failclosed;`); rbErr != nil {
		fatal(rbErr)
	}
	if err == nil {
		fatalf("expected fail-closed error when app.current_tenant is missing")
	}

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantA); err != nil {
		fatal(err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM staffing.assignment_versions WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM staffing.assignment_events WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM staffing.assignments WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM staffing.position_versions WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM staffing.position_events WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM staffing.positions WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}

	initiatorID := "00000000-0000-0000-0000-00000000f001"
	var requestIDPrefix string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&requestIDPrefix); err != nil {
		fatal(err)
	}
	requestID := "dbtool-staffing-smoke-" + requestIDPrefix
	effectiveDate := "2026-01-01"

	var positionID string
	var positionEventID string
	var orgEventID string
	var missingOrgUnitID string
	var orgUnitID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&positionID); err != nil {
		fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&positionEventID); err != nil {
		fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&orgEventID); err != nil {
		fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&missingOrgUnitID); err != nil {
		fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&orgUnitID); err != nil {
		fatal(err)
	}

	var existingRootOrgID string
	err = tx.QueryRow(ctx, `
		SELECT root_org_id::text
		FROM orgunit.org_trees
		WHERE tenant_id = $1::uuid AND hierarchy_type = 'OrgUnit';
	`, tenantA).Scan(&existingRootOrgID)
	if err != nil && err != pgx.ErrNoRows {
		fatal(err)
	}
	if err == nil {
		orgUnitID = existingRootOrgID
		if err := tx.QueryRow(ctx, `
			SELECT lower(validity)::text
			FROM orgunit.org_unit_versions
			WHERE tenant_id = $1::uuid
			  AND hierarchy_type = 'OrgUnit'
			  AND org_id = $2::uuid
			  AND status = 'active'
			ORDER BY lower(validity) DESC
			LIMIT 1;
		`, tenantA, orgUnitID).Scan(&effectiveDate); err != nil {
			fatal(err)
		}
	}

	if _, err := tx.Exec(ctx, `SAVEPOINT sp_missing_org;`); err != nil {
		fatal(err)
	}
	_, err = tx.Exec(ctx, `
			SELECT staffing.submit_position_event(
			  $1::uuid,
			  $2::uuid,
			  $3::uuid,
			  'CREATE',
			  $4::date,
			  jsonb_build_object('org_unit_id', $5::text, 'name', 'Smoke Position'),
			  $6::text,
			  $7::uuid
			);
		`, positionEventID, tenantA, positionID, effectiveDate, missingOrgUnitID, requestID, initiatorID)
	if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_missing_org;`); rbErr != nil {
		fatal(rbErr)
	}
	if err == nil {
		fatalf("expected submit_position_event to fail when org_unit_id is missing as-of")
	}
	if msg, ok := pgErrorMessage(err); !ok || msg != "STAFFING_ORG_UNIT_NOT_FOUND_AS_OF" {
		fatalf("expected pg error message=STAFFING_ORG_UNIT_NOT_FOUND_AS_OF, got ok=%v message=%q err=%v", ok, msg, err)
	}

	if existingRootOrgID == "" {
		if _, err := tx.Exec(ctx, `
			SELECT orgunit.submit_org_event(
			  $1::uuid,
			  $2::uuid,
			  'OrgUnit',
			  $3::uuid,
			  'CREATE',
			  $4::date,
			  jsonb_build_object('name', 'Smoke Org'),
			  $5::text,
			  $6::uuid
			);
		`, orgEventID, tenantA, orgUnitID, effectiveDate, requestID+"-org", initiatorID); err != nil {
			fatal(err)
		}
	}

	var positionEventDBID int64
	if err := tx.QueryRow(ctx, `
			SELECT staffing.submit_position_event(
			  $1::uuid,
			  $2::uuid,
			  $3::uuid,
			  'CREATE',
			  $4::date,
			  jsonb_build_object('org_unit_id', $5::text, 'name', 'Smoke Position'),
			  $6::text,
			  $7::uuid
			);
		`, positionEventID, tenantA, positionID, effectiveDate, orgUnitID, requestID, initiatorID).Scan(&positionEventDBID); err != nil {
		fatal(err)
	}
	if positionEventDBID <= 0 {
		fatalf("expected position event db id > 0, got %d", positionEventDBID)
	}

	var positionVersions int
	if err := tx.QueryRow(ctx, `
		SELECT count(*)
		FROM staffing.position_versions
		WHERE tenant_id = $1::uuid AND position_id = $2::uuid AND validity @> $3::date;
	`, tenantA, positionID, effectiveDate).Scan(&positionVersions); err != nil {
		fatal(err)
	}
	if positionVersions != 1 {
		fatalf("expected position_versions=1, got %d", positionVersions)
	}

	var assignmentID string
	var assignmentEventID string
	var personUUID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&assignmentID); err != nil {
		fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&assignmentEventID); err != nil {
		fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&personUUID); err != nil {
		fatal(err)
	}

	var assignmentEventDBID int64
	if err := tx.QueryRow(ctx, `
			SELECT staffing.submit_assignment_event(
			  $1::uuid,
			  $2::uuid,
			  $3::uuid,
			  $4::uuid,
			  'primary',
			  'CREATE',
			  $5::date,
			  jsonb_build_object(
			    'position_id', $6::text,
			    'allocated_fte', '1.0',
			    'profile', '{}'::jsonb
			  ),
			  $7::text,
			  $8::uuid
			);
		`, assignmentEventID, tenantA, assignmentID, personUUID, effectiveDate, positionID, requestID+"-a2", initiatorID).Scan(&assignmentEventDBID); err != nil {
		fatal(err)
	}
	if assignmentEventDBID <= 0 {
		fatalf("expected assignment event db id > 0, got %d", assignmentEventDBID)
	}

	var assignmentVersions int
	if err := tx.QueryRow(ctx, `
		SELECT count(*)
		FROM staffing.assignment_versions
		WHERE tenant_id = $1::uuid AND assignment_id = $2::uuid AND validity @> $3::date;
	`, tenantA, assignmentID, effectiveDate).Scan(&assignmentVersions); err != nil {
		fatal(err)
	}
	if assignmentVersions != 1 {
		fatalf("expected assignment_versions=1, got %d", assignmentVersions)
	}

	{
		if _, err := tx.Exec(ctx, `SAVEPOINT sp_assignment_one_per_day;`); err != nil {
			fatal(err)
		}

		var secondEventID string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&secondEventID); err != nil {
			fatal(err)
		}

		_, err = tx.Exec(ctx, `
				SELECT staffing.submit_assignment_event(
				  $1::uuid,
				  $2::uuid,
				  $3::uuid,
				  $4::uuid,
				  'primary',
				  'UPDATE',
				  $5::date,
				  jsonb_build_object('position_id', $6::text),
				  $7::text,
				  $8::uuid
				);
			`, secondEventID, tenantA, assignmentID, personUUID, effectiveDate, positionID, requestID+"-as-one-per-day", initiatorID)
		if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_assignment_one_per_day;`); rbErr != nil {
			fatal(rbErr)
		}
		if err == nil {
			fatalf("expected submit_assignment_event to fail when (tenant_id, assignment_id, effective_date) is reused with a different event_id")
		}
		constraint, hasConstraint := pgErrorConstraintName(err)
		if hasConstraint && constraint == "assignment_events_one_per_day_unique" {
			// OK: can locate the exact constraint (DEV-PLAN-031 M3-C).
		} else {
			if msg, ok := pgErrorMessage(err); !ok || msg != "STAFFING_ASSIGNMENT_ONE_PER_DAY" {
				fatalf("expected constraint=assignment_events_one_per_day_unique (or message=STAFFING_ASSIGNMENT_ONE_PER_DAY), got has_constraint=%v constraint=%q err=%v", hasConstraint, constraint, err)
			}
		}
	}

	{
		if _, err := tx.Exec(ctx, `SAVEPOINT sp_assignment_missing_position;`); err != nil {
			fatal(err)
		}

		var missingPositionID string
		var aID string
		var eID string
		var pUUID string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&missingPositionID); err != nil {
			fatal(err)
		}
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&aID); err != nil {
			fatal(err)
		}
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&eID); err != nil {
			fatal(err)
		}
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&pUUID); err != nil {
			fatal(err)
		}

		_, err = tx.Exec(ctx, `
				SELECT staffing.submit_assignment_event(
				  $1::uuid,
				  $2::uuid,
				  $3::uuid,
				  $4::uuid,
				  'primary',
				  'CREATE',
				  $5::date,
				  jsonb_build_object(
				    'position_id', $6::text,
				    'allocated_fte', '1.0',
				    'profile', '{}'::jsonb
				  ),
				  $7::text,
				  $8::uuid
				);
			`, eID, tenantA, aID, pUUID, effectiveDate, missingPositionID, requestID+"-as-missing-position", initiatorID)
		if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_assignment_missing_position;`); rbErr != nil {
			fatal(rbErr)
		}
		if err == nil {
			fatalf("expected submit_assignment_event to fail when position is missing as-of")
		}
		if msg, ok := pgErrorMessage(err); !ok || msg != "STAFFING_POSITION_NOT_FOUND_AS_OF" {
			fatalf("expected pg error message=STAFFING_POSITION_NOT_FOUND_AS_OF, got ok=%v message=%q err=%v", ok, msg, err)
		}
	}

	// DEV-PLAN-031 M4: Correct / Rescind / Delete-slice（附属 SoT + replay 统一解释）.
	{
		// Correct CREATE: replace payload interpretation (allocated_fte 1.0 -> 0.75).
		var correctionEventID string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&correctionEventID); err != nil {
			fatal(err)
		}
		if _, err := tx.Exec(ctx, `
				SELECT staffing.submit_assignment_event_correction(
				  $1::uuid,
				  $2::uuid,
				  $3::uuid,
				  $4::date,
				  jsonb_build_object(
				    'position_id', $5::text,
				    'allocated_fte', '0.75',
				    'profile', '{}'::jsonb
				  ),
				  $6::text,
				  $7::uuid
				);
			`, correctionEventID, tenantA, assignmentID, effectiveDate, positionID, requestID+"-a2-correct-create", initiatorID); err != nil {
			fatal(err)
		}

		var allocatedFte string
		if err := tx.QueryRow(ctx, `
				SELECT COALESCE(allocated_fte::text, '')
				FROM staffing.assignment_versions
				WHERE tenant_id = $1::uuid
				  AND assignment_id = $2::uuid
				  AND validity @> $3::date
				LIMIT 1;
			`, tenantA, assignmentID, effectiveDate).Scan(&allocatedFte); err != nil {
			fatal(err)
		}
		if allocatedFte != "0.75" {
			fatalf("expected allocated_fte=0.75 after correct, got %q", allocatedFte)
		}
	}

	updateDate := "2026-02-01"
	{
		// Prepare a second position to update into.
		var positionID2 string
		var positionEventID2 string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&positionID2); err != nil {
			fatal(err)
		}
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&positionEventID2); err != nil {
			fatal(err)
		}
		if _, err := tx.Exec(ctx, `
					SELECT staffing.submit_position_event(
					  $1::uuid,
					  $2::uuid,
					  $3::uuid,
					  'CREATE',
					  $4::date,
					  jsonb_build_object('org_unit_id', $5::text, 'name', 'Smoke Position 2'),
					  $6::text,
					  $7::uuid
					);
				`, positionEventID2, tenantA, positionID2, effectiveDate, orgUnitID, requestID+"-pos2", initiatorID); err != nil {
			fatal(err)
		}

		// UPDATE assignment to position 2.
		var updateEventID string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&updateEventID); err != nil {
			fatal(err)
		}
		if _, err := tx.Exec(ctx, `
				SELECT staffing.submit_assignment_event(
				  $1::uuid,
				  $2::uuid,
				  $3::uuid,
				  $4::uuid,
				  'primary',
				  'UPDATE',
				  $5::date,
				  jsonb_build_object('position_id', $6::text),
				  $7::text,
				  $8::uuid
				);
			`, updateEventID, tenantA, assignmentID, personUUID, updateDate, positionID2, requestID+"-a2-update", initiatorID); err != nil {
			fatal(err)
		}

		// Rescind UPDATE: delete-slice + stitch should leave only the CREATE slice as-of later date.
		var rescindEventID string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&rescindEventID); err != nil {
			fatal(err)
		}
		if _, err := tx.Exec(ctx, `
				SELECT staffing.submit_assignment_event_rescind(
				  $1::uuid,
				  $2::uuid,
				  $3::uuid,
				  $4::date,
				  '{}'::jsonb,
				  $5::text,
				  $6::uuid
				);
			`, rescindEventID, tenantA, assignmentID, updateDate, requestID+"-a2-rescind-update", initiatorID); err != nil {
			fatal(err)
		}

		asOfAfterUpdate := "2026-02-15"
		var posAfter string
		if err := tx.QueryRow(ctx, `
				SELECT position_id::text
				FROM staffing.assignment_versions
				WHERE tenant_id = $1::uuid
				  AND assignment_id = $2::uuid
				  AND validity @> $3::date
				LIMIT 1;
			`, tenantA, assignmentID, asOfAfterUpdate).Scan(&posAfter); err != nil {
			fatal(err)
		}
		if posAfter != positionID {
			fatalf("expected stitched position_id=%s after rescind, got %s", positionID, posAfter)
		}

		// Not allowed to rescind CREATE (when later events exist).
		if _, err := tx.Exec(ctx, `SAVEPOINT sp_rescind_create;`); err != nil {
			fatal(err)
		}
		var rescindCreateEventID string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&rescindCreateEventID); err != nil {
			fatal(err)
		}
		_, err := tx.Exec(ctx, `
				SELECT staffing.submit_assignment_event_rescind(
				  $1::uuid,
				  $2::uuid,
				  $3::uuid,
				  $4::date,
				  '{}'::jsonb,
				  $5::text,
				  $6::uuid
				);
			`, rescindCreateEventID, tenantA, assignmentID, effectiveDate, requestID+"-a2-rescind-create", initiatorID)
		if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_rescind_create;`); rbErr != nil {
			fatal(rbErr)
		}
		if err == nil {
			fatalf("expected rescind CREATE to fail")
		}
		if msg, ok := pgErrorMessage(err); !ok || msg != "STAFFING_ASSIGNMENT_CREATE_CANNOT_RESCIND" {
			fatalf("expected pg error message=STAFFING_ASSIGNMENT_CREATE_CANNOT_RESCIND, got ok=%v message=%q err=%v", ok, msg, err)
		}

		// Correcting a rescinded target must fail-closed.
		if _, err := tx.Exec(ctx, `SAVEPOINT sp_correct_rescinded;`); err != nil {
			fatal(err)
		}
		var correctRescindedID string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&correctRescindedID); err != nil {
			fatal(err)
		}
		_, err = tx.Exec(ctx, `
				SELECT staffing.submit_assignment_event_correction(
				  $1::uuid,
				  $2::uuid,
				  $3::uuid,
				  $4::date,
				  jsonb_build_object('position_id', $5::text),
				  $6::text,
				  $7::uuid
				);
			`, correctRescindedID, tenantA, assignmentID, updateDate, positionID2, requestID+"-a2-correct-rescinded", initiatorID)
		if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_correct_rescinded;`); rbErr != nil {
			fatal(rbErr)
		}
		if err == nil {
			fatalf("expected correct on rescinded target to fail")
		}
		if msg, ok := pgErrorMessage(err); !ok || msg != "STAFFING_ASSIGNMENT_EVENT_ALREADY_RESCINDED" {
			fatalf("expected pg error message=STAFFING_ASSIGNMENT_EVENT_ALREADY_RESCINDED, got ok=%v message=%q err=%v", ok, msg, err)
		}
	}

	{
		// Target not found should be stable.
		if _, err := tx.Exec(ctx, `SAVEPOINT sp_correct_target_not_found;`); err != nil {
			fatal(err)
		}
		var eID string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&eID); err != nil {
			fatal(err)
		}
		_, err := tx.Exec(ctx, `
				SELECT staffing.submit_assignment_event_correction(
				  $1::uuid,
				  $2::uuid,
				  $3::uuid,
				  '2099-01-01'::date,
				  jsonb_build_object('position_id', $4::text),
				  $5::text,
				  $6::uuid
				);
			`, eID, tenantA, assignmentID, positionID, requestID+"-target-not-found", initiatorID)
		if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_correct_target_not_found;`); rbErr != nil {
			fatal(rbErr)
		}
		if err == nil {
			fatalf("expected correct target not found to fail")
		}
		if msg, ok := pgErrorMessage(err); !ok || msg != "STAFFING_ASSIGNMENT_EVENT_NOT_FOUND" {
			fatalf("expected pg error message=STAFFING_ASSIGNMENT_EVENT_NOT_FOUND, got ok=%v message=%q err=%v", ok, msg, err)
		}
	}

	disableDateTime, err := time.Parse("2006-01-02", effectiveDate)
	if err != nil {
		fatal(err)
	}
	disableDate := disableDateTime.AddDate(0, 0, 1).Format("2006-01-02")

	var disablePositionID string
	var disablePositionEventID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&disablePositionID); err != nil {
		fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&disablePositionEventID); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `
				SELECT staffing.submit_position_event(
				  $1::uuid,
				  $2::uuid,
				  $3::uuid,
				  'CREATE',
				  $4::date,
				  jsonb_build_object('org_unit_id', $5::text, 'name', 'Smoke Disable Test Position'),
				  $6::text,
				  $7::uuid
				);
			`, disablePositionEventID, tenantA, disablePositionID, effectiveDate, orgUnitID, requestID+"-pos-disable-test-create", initiatorID); err != nil {
		fatal(err)
	}

	var disableAssignmentID string
	var disableAssignmentEventID string
	var disablePersonUUID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&disableAssignmentID); err != nil {
		fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&disableAssignmentEventID); err != nil {
		fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&disablePersonUUID); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `
			SELECT staffing.submit_assignment_event(
			  $1::uuid,
			  $2::uuid,
			  $3::uuid,
			  $4::uuid,
			  'primary',
			  'CREATE',
			  $5::date,
			  jsonb_build_object(
			    'position_id', $6::text,
			    'allocated_fte', '1.0',
			    'profile', '{}'::jsonb
			  ),
			  $7::text,
			  $8::uuid
			);
		`, disableAssignmentEventID, tenantA, disableAssignmentID, disablePersonUUID, effectiveDate, disablePositionID, requestID+"-as-disable-test-create", initiatorID); err != nil {
		fatal(err)
	}

	{
		if _, err := tx.Exec(ctx, `SAVEPOINT sp_disable_position_with_active_assignment;`); err != nil {
			fatal(err)
		}
		var disableEventID string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&disableEventID); err != nil {
			fatal(err)
		}

		_, err = tx.Exec(ctx, `
				SELECT staffing.submit_position_event(
				  $1::uuid,
				  $2::uuid,
				  $3::uuid,
				  'UPDATE',
				  $4::date,
				  jsonb_build_object('lifecycle_status', 'disabled'),
				  $5::text,
				  $6::uuid
				);
			`, disableEventID, tenantA, disablePositionID, disableDate, requestID+"-pos-disable-should-fail", initiatorID)
		if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_disable_position_with_active_assignment;`); rbErr != nil {
			fatal(rbErr)
		}
		if err == nil {
			fatalf("expected disabling position to fail when it still has active assignments")
		}
		if msg, ok := pgErrorMessage(err); !ok || msg != "STAFFING_POSITION_HAS_ACTIVE_ASSIGNMENT_AS_OF" {
			fatalf("expected pg error message=STAFFING_POSITION_HAS_ACTIVE_ASSIGNMENT_AS_OF, got ok=%v message=%q err=%v", ok, msg, err)
		}
	}

	{
		var assignmentDeactivateEventID string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&assignmentDeactivateEventID); err != nil {
			fatal(err)
		}
		if _, err := tx.Exec(ctx, `
				SELECT staffing.submit_assignment_event(
				  $1::uuid,
				  $2::uuid,
				  $3::uuid,
				  $4::uuid,
				  'primary',
				  'UPDATE',
				  $5::date,
				  jsonb_build_object('status', 'inactive'),
				  $6::text,
				  $7::uuid
				);
			`, assignmentDeactivateEventID, tenantA, disableAssignmentID, disablePersonUUID, disableDate, requestID+"-as-deactivate", initiatorID); err != nil {
			fatal(err)
		}

		var disableEventID string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&disableEventID); err != nil {
			fatal(err)
		}
		if _, err := tx.Exec(ctx, `
				SELECT staffing.submit_position_event(
				  $1::uuid,
				  $2::uuid,
				  $3::uuid,
				  'UPDATE',
				  $4::date,
				  jsonb_build_object('lifecycle_status', 'disabled'),
				  $5::text,
				  $6::uuid
				);
			`, disableEventID, tenantA, disablePositionID, disableDate, requestID+"-pos-disable", initiatorID); err != nil {
			fatal(err)
		}
	}

	{
		if _, err := tx.Exec(ctx, `SAVEPOINT sp_assignment_to_disabled_position;`); err != nil {
			fatal(err)
		}

		var otherAssignmentID string
		var otherAssignmentEventID string
		var otherPersonUUID string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&otherAssignmentID); err != nil {
			fatal(err)
		}
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&otherAssignmentEventID); err != nil {
			fatal(err)
		}
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&otherPersonUUID); err != nil {
			fatal(err)
		}

		_, err = tx.Exec(ctx, `
				SELECT staffing.submit_assignment_event(
				  $1::uuid,
				  $2::uuid,
				  $3::uuid,
				  $4::uuid,
				  'primary',
				  'CREATE',
				  $5::date,
				  jsonb_build_object(
				    'position_id', $6::text,
				    'allocated_fte', '1.0',
				    'profile', '{}'::jsonb
				  ),
				  $7::text,
				  $8::uuid
				);
			`, otherAssignmentEventID, tenantA, otherAssignmentID, otherPersonUUID, disableDate, disablePositionID, requestID+"-as-disabled", initiatorID)
		if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_assignment_to_disabled_position;`); rbErr != nil {
			fatal(rbErr)
		}
		if err == nil {
			fatalf("expected assignment create to disabled position to fail")
		}
		if msg, ok := pgErrorMessage(err); !ok || msg != "STAFFING_POSITION_DISABLED_AS_OF" {
			fatalf("expected pg error message=STAFFING_POSITION_DISABLED_AS_OF, got ok=%v message=%q err=%v", ok, msg, err)
		}
	}

	reportsToDate := disableDateTime.AddDate(0, 0, 2).Format("2006-01-02")
	reportsToRetroDate := disableDate

	{
		var posAID, posBID, posCID string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&posAID); err != nil {
			fatal(err)
		}
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&posBID); err != nil {
			fatal(err)
		}
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&posCID); err != nil {
			fatal(err)
		}

		createPosition := func(positionID string, name string) {
			var eventID string
			if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&eventID); err != nil {
				fatal(err)
			}
			if _, err := tx.Exec(ctx, `
					SELECT staffing.submit_position_event(
					  $1::uuid,
					  $2::uuid,
					  $3::uuid,
					  'CREATE',
					  $4::date,
					  jsonb_build_object('org_unit_id', $5::text, 'name', $6::text),
					  $7::text,
					  $8::uuid
					);
				`, eventID, tenantA, positionID, effectiveDate, orgUnitID, name, requestID+"-pos-reports-to-create-"+positionID, initiatorID); err != nil {
				fatal(err)
			}
		}

		updateReportsTo := func(positionID string, asOf string, reportsToPositionID string) error {
			var eventID string
			if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&eventID); err != nil {
				fatal(err)
			}
			_, err := tx.Exec(ctx, `
				SELECT staffing.submit_position_event(
				  $1::uuid,
				  $2::uuid,
				  $3::uuid,
				  'UPDATE',
				  $4::date,
				  jsonb_build_object('reports_to_position_id', $5::text),
				  $6::text,
				  $7::uuid
				);
			`, eventID, tenantA, positionID, asOf, reportsToPositionID, requestID+"-pos-reports-to-update-"+positionID+"-"+asOf, initiatorID)
			return err
		}

		createPosition(posAID, "Smoke ReportsTo A")
		createPosition(posBID, "Smoke ReportsTo B")
		createPosition(posCID, "Smoke ReportsTo C")

		if err := updateReportsTo(posBID, reportsToDate, posAID); err != nil {
			fatalf("expected reports_to update to succeed, err=%v", err)
		}
		if err := updateReportsTo(posCID, reportsToDate, posBID); err != nil {
			fatalf("expected reports_to update to succeed, err=%v", err)
		}

		{
			if _, err := tx.Exec(ctx, `SAVEPOINT sp_reports_to_self;`); err != nil {
				fatal(err)
			}
			err := updateReportsTo(posAID, reportsToDate, posAID)
			if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_reports_to_self;`); rbErr != nil {
				fatal(rbErr)
			}
			if err == nil {
				fatalf("expected reports_to self to fail")
			}
			if msg, ok := pgErrorMessage(err); !ok || msg != "STAFFING_POSITION_REPORTS_TO_SELF" {
				fatalf("expected pg error message=STAFFING_POSITION_REPORTS_TO_SELF, got ok=%v message=%q err=%v", ok, msg, err)
			}
		}

		{
			if _, err := tx.Exec(ctx, `SAVEPOINT sp_reports_to_cycle;`); err != nil {
				fatal(err)
			}
			err := updateReportsTo(posAID, reportsToDate, posCID)
			if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_reports_to_cycle;`); rbErr != nil {
				fatal(rbErr)
			}
			if err == nil {
				fatalf("expected reports_to cycle to fail")
			}
			if msg, ok := pgErrorMessage(err); !ok || msg != "STAFFING_POSITION_REPORTS_TO_CYCLE" {
				fatalf("expected pg error message=STAFFING_POSITION_REPORTS_TO_CYCLE, got ok=%v message=%q err=%v", ok, msg, err)
			}
		}

		{
			if _, err := tx.Exec(ctx, `SAVEPOINT sp_reports_to_retro;`); err != nil {
				fatal(err)
			}
			err := updateReportsTo(posBID, reportsToRetroDate, posAID)
			if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_reports_to_retro;`); rbErr != nil {
				fatal(rbErr)
			}
			if err == nil {
				fatalf("expected retro reports_to update to fail")
			}
			if msg, ok := pgErrorMessage(err); !ok || msg != "STAFFING_INVALID_ARGUMENT" {
				fatalf("expected pg error message=STAFFING_INVALID_ARGUMENT, got ok=%v message=%q err=%v", ok, msg, err)
			}
		}
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
	var crossCount int
	if err := tx2.QueryRow(ctx, `SELECT count(*) FROM staffing.position_versions;`).Scan(&crossCount); err != nil {
		fatal(err)
	}
	if crossCount != 0 {
		fatalf("expected position_versions count=0 under tenant B, got %d", crossCount)
	}

	if err := tx2.Commit(ctx); err != nil {
		fatal(err)
	}

	fmt.Println("[staffing-smoke] OK")
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

	if _, err := tx.Exec(ctx, `DELETE FROM jobcatalog.job_profile_version_job_families WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM jobcatalog.job_profile_versions WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM jobcatalog.job_profile_events WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM jobcatalog.job_profiles WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM jobcatalog.job_level_versions WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM jobcatalog.job_level_events WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM jobcatalog.job_levels WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM jobcatalog.job_family_versions WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM jobcatalog.job_family_events WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM jobcatalog.job_families WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}

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

	var createdEventDBID int64
	if err := tx.QueryRow(ctx, `
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
`, eventID, tenantA, groupID, "2026-01-01", requestID, initiatorID).Scan(&createdEventDBID); err != nil {
		fatal(err)
	}

	var retriedEventDBID int64
	if err := tx.QueryRow(ctx, `
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
`, eventID, tenantA, groupID, "2026-01-01", requestID, initiatorID).Scan(&retriedEventDBID); err != nil {
		fatal(err)
	}
	if retriedEventDBID != createdEventDBID {
		fatalf("expected idempotent retry to return same event id: got %d want %d", retriedEventDBID, createdEventDBID)
	}

	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM jobcatalog.job_family_group_events WHERE tenant_id = $1::uuid;`, tenantA).Scan(&count); err != nil {
		fatal(err)
	}
	if count != 1 {
		fatalf("expected events count=1 under tenant A, got %d", count)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM jobcatalog.job_family_group_versions WHERE validity @> '2026-01-01'::date;`).Scan(&count); err != nil {
		fatal(err)
	}
	if count != 1 {
		fatalf("expected versions count=1 under tenant A, got %d", count)
	}

	groupID2 := "00000000-0000-0000-0000-00000000c111"
	eventID2 := "00000000-0000-0000-0000-00000000c112"
	requestID2 := "dbtool-jobcatalog-smoke-create2"

	if _, err := tx.Exec(ctx, `
SELECT jobcatalog.submit_job_family_group_event(
  $1::uuid,
  $2::uuid,
  'SHARE',
  $3::uuid,
  'CREATE',
  $4::date,
  jsonb_build_object('code', 'JC2', 'name', 'Job Family Group 2', 'description', null),
  $5::text,
  $6::uuid
);
`, eventID2, tenantA, groupID2, "2026-01-01", requestID2, initiatorID); err != nil {
		fatal(err)
	}

	familyID := "00000000-0000-0000-0000-00000000c201"
	familyCreateEventID := "00000000-0000-0000-0000-00000000c202"
	familyCreateRequestID := "dbtool-jobcatalog-smoke-family-create"

	var familyCreatedEventDBID int64
	if err := tx.QueryRow(ctx, `
SELECT jobcatalog.submit_job_family_event(
  $1::uuid,
  $2::uuid,
  'SHARE',
  $3::uuid,
  'CREATE',
  $4::date,
  jsonb_build_object('code', 'JF1', 'name', 'Job Family 1', 'description', null, 'job_family_group_id', $5::uuid),
  $6::text,
  $7::uuid
);
`, familyCreateEventID, tenantA, familyID, "2026-01-01", groupID, familyCreateRequestID, initiatorID).Scan(&familyCreatedEventDBID); err != nil {
		fatal(err)
	}

	var familyRetriedEventDBID int64
	if err := tx.QueryRow(ctx, `
SELECT jobcatalog.submit_job_family_event(
  $1::uuid,
  $2::uuid,
  'SHARE',
  $3::uuid,
  'CREATE',
  $4::date,
  jsonb_build_object('code', 'JF1', 'name', 'Job Family 1', 'description', null, 'job_family_group_id', $5::uuid),
  $6::text,
  $7::uuid
);
`, familyCreateEventID, tenantA, familyID, "2026-01-01", groupID, familyCreateRequestID, initiatorID).Scan(&familyRetriedEventDBID); err != nil {
		fatal(err)
	}
	if familyRetriedEventDBID != familyCreatedEventDBID {
		fatalf("expected idempotent retry to return same family event id: got %d want %d", familyRetriedEventDBID, familyCreatedEventDBID)
	}

	familyUpdateEventID := "00000000-0000-0000-0000-00000000c203"
	familyUpdateRequestID := "dbtool-jobcatalog-smoke-family-reparent"

	if _, err := tx.Exec(ctx, `
SELECT jobcatalog.submit_job_family_event(
  $1::uuid,
  $2::uuid,
  'SHARE',
  $3::uuid,
  'UPDATE',
  $4::date,
  jsonb_build_object('job_family_group_id', $5::uuid),
  $6::text,
  $7::uuid
);
`, familyUpdateEventID, tenantA, familyID, "2026-02-01", groupID2, familyUpdateRequestID, initiatorID); err != nil {
		fatal(err)
	}

	familyDisableEventID := "00000000-0000-0000-0000-00000000c204"
	familyDisableRequestID := "dbtool-jobcatalog-smoke-family-disable"

	if _, err := tx.Exec(ctx, `
SELECT jobcatalog.submit_job_family_event(
  $1::uuid,
  $2::uuid,
  'SHARE',
  $3::uuid,
  'DISABLE',
  $4::date,
  '{}'::jsonb,
  $5::text,
  $6::uuid
);
`, familyDisableEventID, tenantA, familyID, "2026-03-01", familyDisableRequestID, initiatorID); err != nil {
		fatal(err)
	}

	var familyGroupAtJan string
	if err := tx.QueryRow(ctx, `
SELECT job_family_group_id::text
FROM jobcatalog.job_family_versions
WHERE tenant_id = $1::uuid
  AND setid = 'SHARE'
  AND job_family_id = $2::uuid
  AND validity @> $3::date
LIMIT 1
`, tenantA, familyID, "2026-01-15").Scan(&familyGroupAtJan); err != nil {
		fatal(err)
	}
	if familyGroupAtJan != groupID {
		fatalf("expected family group at 2026-01-15 to be %s, got %s", groupID, familyGroupAtJan)
	}

	var familyGroupAtFeb string
	if err := tx.QueryRow(ctx, `
SELECT job_family_group_id::text
FROM jobcatalog.job_family_versions
WHERE tenant_id = $1::uuid
  AND setid = 'SHARE'
  AND job_family_id = $2::uuid
  AND validity @> $3::date
LIMIT 1
`, tenantA, familyID, "2026-02-15").Scan(&familyGroupAtFeb); err != nil {
		fatal(err)
	}
	if familyGroupAtFeb != groupID2 {
		fatalf("expected family group at 2026-02-15 to be %s, got %s", groupID2, familyGroupAtFeb)
	}

	var familyIsActiveAtMar bool
	if err := tx.QueryRow(ctx, `
SELECT is_active
FROM jobcatalog.job_family_versions
WHERE tenant_id = $1::uuid
  AND setid = 'SHARE'
  AND job_family_id = $2::uuid
  AND validity @> $3::date
LIMIT 1
`, tenantA, familyID, "2026-03-15").Scan(&familyIsActiveAtMar); err != nil {
		fatal(err)
	}
	if familyIsActiveAtMar {
		fatalf("expected family to be disabled at 2026-03-15")
	}

	familyID2 := "00000000-0000-0000-0000-00000000c211"
	family2CreateEventID := "00000000-0000-0000-0000-00000000c212"
	family2CreateRequestID := "dbtool-jobcatalog-smoke-family2-create"

	if _, err := tx.Exec(ctx, `
	SELECT jobcatalog.submit_job_family_event(
	  $1::uuid,
	  $2::uuid,
	  'SHARE',
	  $3::uuid,
	  'CREATE',
	  $4::date,
	  jsonb_build_object('code', 'JF2', 'name', 'Job Family 2', 'description', null, 'job_family_group_id', $5::uuid),
	  $6::text,
	  $7::uuid
	);
	`, family2CreateEventID, tenantA, familyID2, "2026-01-01", groupID, family2CreateRequestID, initiatorID); err != nil {
		fatal(err)
	}

	profileID := "00000000-0000-0000-0000-00000000c401"
	profileCreateEventID := "00000000-0000-0000-0000-00000000c402"
	profileCreateRequestID := "dbtool-jobcatalog-smoke-profile-create"

	var profileCreatedEventDBID int64
	if err := tx.QueryRow(ctx, `
	SELECT jobcatalog.submit_job_profile_event(
	  $1::uuid,
	  $2::uuid,
	  'SHARE',
	  $3::uuid,
	  'CREATE',
	  $4::date,
	  jsonb_build_object(
	    'code', 'JP1',
	    'name', 'Job Profile 1',
	    'description', null,
	    'job_family_ids', jsonb_build_array($5::uuid, $6::uuid),
	    'primary_job_family_id', $5::uuid
	  ),
	  $7::text,
	  $8::uuid
	);
	`, profileCreateEventID, tenantA, profileID, "2026-01-01", familyID, familyID2, profileCreateRequestID, initiatorID).Scan(&profileCreatedEventDBID); err != nil {
		fatal(err)
	}
	if profileCreatedEventDBID <= 0 {
		fatalf("expected profile event db id > 0, got %d", profileCreatedEventDBID)
	}

	profileUpdateEventID := "00000000-0000-0000-0000-00000000c403"
	profileUpdateRequestID := "dbtool-jobcatalog-smoke-profile-update"
	if _, err := tx.Exec(ctx, `
	SELECT jobcatalog.submit_job_profile_event(
	  $1::uuid,
	  $2::uuid,
	  'SHARE',
	  $3::uuid,
	  'UPDATE',
	  $4::date,
	  jsonb_build_object(
	    'job_family_ids', jsonb_build_array($5::uuid),
	    'primary_job_family_id', $5::uuid
	  ),
	  $6::text,
	  $7::uuid
	);
	`, profileUpdateEventID, tenantA, profileID, "2026-02-01", familyID2, profileUpdateRequestID, initiatorID); err != nil {
		fatal(err)
	}

	profileDisableEventID := "00000000-0000-0000-0000-00000000c404"
	profileDisableRequestID := "dbtool-jobcatalog-smoke-profile-disable"
	if _, err := tx.Exec(ctx, `
	SELECT jobcatalog.submit_job_profile_event(
	  $1::uuid,
	  $2::uuid,
	  'SHARE',
	  $3::uuid,
	  'DISABLE',
	  $4::date,
	  '{}'::jsonb,
	  $5::text,
	  $6::uuid
	);
	`, profileDisableEventID, tenantA, profileID, "2026-03-01", profileDisableRequestID, initiatorID); err != nil {
		fatal(err)
	}

	var profileFamiliesAtJan int
	var profilePrimaryCountAtJan int
	if err := tx.QueryRow(ctx, `
	SELECT
	  count(*)::int,
	  sum(CASE WHEN f.is_primary THEN 1 ELSE 0 END)::int
	FROM jobcatalog.job_profile_versions v
	JOIN jobcatalog.job_profile_version_job_families f
	  ON f.job_profile_version_id = v.id
	WHERE v.tenant_id = $1::uuid
	  AND v.setid = 'SHARE'
	  AND v.job_profile_id = $2::uuid
	  AND v.validity @> $3::date
	`, tenantA, profileID, "2026-01-15").Scan(&profileFamiliesAtJan, &profilePrimaryCountAtJan); err != nil {
		fatal(err)
	}
	if profileFamiliesAtJan != 2 {
		fatalf("expected profile families count at 2026-01-15 to be 2, got %d", profileFamiliesAtJan)
	}
	if profilePrimaryCountAtJan != 1 {
		fatalf("expected profile primary count at 2026-01-15 to be 1, got %d", profilePrimaryCountAtJan)
	}

	var profilePrimaryFamilyAtJan string
	if err := tx.QueryRow(ctx, `
	SELECT f.job_family_id::text
	FROM jobcatalog.job_profile_versions v
	JOIN jobcatalog.job_profile_version_job_families f
	  ON f.job_profile_version_id = v.id
	WHERE v.tenant_id = $1::uuid
	  AND v.setid = 'SHARE'
	  AND v.job_profile_id = $2::uuid
	  AND v.validity @> $3::date
	  AND f.is_primary = true
	LIMIT 1
	`, tenantA, profileID, "2026-01-15").Scan(&profilePrimaryFamilyAtJan); err != nil {
		fatal(err)
	}
	if profilePrimaryFamilyAtJan != familyID {
		fatalf("expected primary family at 2026-01-15 to be %s, got %s", familyID, profilePrimaryFamilyAtJan)
	}

	var profileFamiliesAtFeb int
	var profilePrimaryCountAtFeb int
	if err := tx.QueryRow(ctx, `
	SELECT
	  count(*)::int,
	  sum(CASE WHEN f.is_primary THEN 1 ELSE 0 END)::int
	FROM jobcatalog.job_profile_versions v
	JOIN jobcatalog.job_profile_version_job_families f
	  ON f.job_profile_version_id = v.id
	WHERE v.tenant_id = $1::uuid
	  AND v.setid = 'SHARE'
	  AND v.job_profile_id = $2::uuid
	  AND v.validity @> $3::date
	`, tenantA, profileID, "2026-02-15").Scan(&profileFamiliesAtFeb, &profilePrimaryCountAtFeb); err != nil {
		fatal(err)
	}
	if profileFamiliesAtFeb != 1 {
		fatalf("expected profile families count at 2026-02-15 to be 1, got %d", profileFamiliesAtFeb)
	}
	if profilePrimaryCountAtFeb != 1 {
		fatalf("expected profile primary count at 2026-02-15 to be 1, got %d", profilePrimaryCountAtFeb)
	}

	var profileIsActiveAtMar bool
	if err := tx.QueryRow(ctx, `
	SELECT is_active
	FROM jobcatalog.job_profile_versions
	WHERE tenant_id = $1::uuid
	  AND setid = 'SHARE'
	  AND job_profile_id = $2::uuid
	  AND validity @> $3::date
	LIMIT 1
	`, tenantA, profileID, "2026-03-15").Scan(&profileIsActiveAtMar); err != nil {
		fatal(err)
	}
	if profileIsActiveAtMar {
		fatalf("expected profile to be disabled at 2026-03-15")
	}

	levelID := "00000000-0000-0000-0000-00000000c301"
	levelCreateEventID := "00000000-0000-0000-0000-00000000c302"
	levelCreateRequestID := "dbtool-jobcatalog-smoke-level-create"

	var levelCreatedEventDBID int64
	if err := tx.QueryRow(ctx, `
SELECT jobcatalog.submit_job_level_event(
  $1::uuid,
  $2::uuid,
  'SHARE',
  $3::uuid,
  'CREATE',
  $4::date,
  jsonb_build_object('code', 'JL1', 'name', 'Job Level 1', 'description', null),
  $5::text,
  $6::uuid
);
`, levelCreateEventID, tenantA, levelID, "2026-01-01", levelCreateRequestID, initiatorID).Scan(&levelCreatedEventDBID); err != nil {
		fatal(err)
	}

	var levelRetriedEventDBID int64
	if err := tx.QueryRow(ctx, `
SELECT jobcatalog.submit_job_level_event(
  $1::uuid,
  $2::uuid,
  'SHARE',
  $3::uuid,
  'CREATE',
  $4::date,
  jsonb_build_object('code', 'JL1', 'name', 'Job Level 1', 'description', null),
  $5::text,
  $6::uuid
);
`, levelCreateEventID, tenantA, levelID, "2026-01-01", levelCreateRequestID, initiatorID).Scan(&levelRetriedEventDBID); err != nil {
		fatal(err)
	}
	if levelRetriedEventDBID != levelCreatedEventDBID {
		fatalf("expected idempotent retry to return same level event id: got %d want %d", levelRetriedEventDBID, levelCreatedEventDBID)
	}

	levelUpdateEventID := "00000000-0000-0000-0000-00000000c303"
	levelUpdateRequestID := "dbtool-jobcatalog-smoke-level-update"

	if _, err := tx.Exec(ctx, `
SELECT jobcatalog.submit_job_level_event(
  $1::uuid,
  $2::uuid,
  'SHARE',
  $3::uuid,
  'UPDATE',
  $4::date,
  jsonb_build_object('name', 'Job Level 1 Updated'),
  $5::text,
  $6::uuid
);
`, levelUpdateEventID, tenantA, levelID, "2026-02-01", levelUpdateRequestID, initiatorID); err != nil {
		fatal(err)
	}

	levelDisableEventID := "00000000-0000-0000-0000-00000000c304"
	levelDisableRequestID := "dbtool-jobcatalog-smoke-level-disable"

	if _, err := tx.Exec(ctx, `
SELECT jobcatalog.submit_job_level_event(
  $1::uuid,
  $2::uuid,
  'SHARE',
  $3::uuid,
  'DISABLE',
  $4::date,
  '{}'::jsonb,
  $5::text,
  $6::uuid
);
`, levelDisableEventID, tenantA, levelID, "2026-03-01", levelDisableRequestID, initiatorID); err != nil {
		fatal(err)
	}

	var levelNameAtFeb string
	if err := tx.QueryRow(ctx, `
SELECT name
FROM jobcatalog.job_level_versions
WHERE tenant_id = $1::uuid
  AND setid = 'SHARE'
  AND job_level_id = $2::uuid
  AND validity @> $3::date
LIMIT 1
`, tenantA, levelID, "2026-02-15").Scan(&levelNameAtFeb); err != nil {
		fatal(err)
	}
	if levelNameAtFeb != "Job Level 1 Updated" {
		fatalf("expected level name at 2026-02-15 to be updated, got %q", levelNameAtFeb)
	}

	var levelIsActiveAtMar bool
	if err := tx.QueryRow(ctx, `
SELECT is_active
FROM jobcatalog.job_level_versions
WHERE tenant_id = $1::uuid
  AND setid = 'SHARE'
  AND job_level_id = $2::uuid
  AND validity @> $3::date
LIMIT 1
`, tenantA, levelID, "2026-03-15").Scan(&levelIsActiveAtMar); err != nil {
		fatal(err)
	}
	if levelIsActiveAtMar {
		fatalf("expected level to be disabled at 2026-03-15")
	}

	if _, err := tx.Exec(ctx, `
	SELECT *
	FROM jobcatalog.get_job_catalog_snapshot($1::uuid, 'SHARE', $2::date);
	`, tenantA, "2026-02-15"); err != nil {
		fatal(err)
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

func pgErrorConstraintName(err error) (string, bool) {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
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
	_, _ = conn.Exec(ctx, `GRANT USAGE ON SCHEMA jobcatalog TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT USAGE ON SCHEMA person TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT USAGE ON SCHEMA staffing TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA iam TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA orgunit TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA jobcatalog TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA person TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA staffing TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA iam TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA orgunit TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA jobcatalog TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA person TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA staffing TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA iam TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA orgunit TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA jobcatalog TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA person TO `+role+`;`)
	_, _ = conn.Exec(ctx, `GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA staffing TO `+role+`;`)
	_, _ = conn.Exec(ctx, `ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO `+role+`;`)
	_, _ = conn.Exec(ctx, `ALTER DEFAULT PRIVILEGES IN SCHEMA iam GRANT USAGE, SELECT ON SEQUENCES TO `+role+`;`)
	_, _ = conn.Exec(ctx, `ALTER DEFAULT PRIVILEGES IN SCHEMA orgunit GRANT USAGE, SELECT ON SEQUENCES TO `+role+`;`)
	_, _ = conn.Exec(ctx, `ALTER DEFAULT PRIVILEGES IN SCHEMA jobcatalog GRANT USAGE, SELECT ON SEQUENCES TO `+role+`;`)
	_, _ = conn.Exec(ctx, `ALTER DEFAULT PRIVILEGES IN SCHEMA person GRANT USAGE, SELECT ON SEQUENCES TO `+role+`;`)
	_, _ = conn.Exec(ctx, `ALTER DEFAULT PRIVILEGES IN SCHEMA staffing GRANT USAGE, SELECT ON SEQUENCES TO `+role+`;`)
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
