package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
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

	if _, err := tx.Exec(ctx, `SAVEPOINT sp_failclosed_payroll;`); err != nil {
		fatal(err)
	}
	_, err = tx.Exec(ctx, `SELECT count(*) FROM staffing.pay_periods;`)
	if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_failclosed_payroll;`); rbErr != nil {
		fatal(rbErr)
	}
	if err == nil {
		fatalf("expected fail-closed error when app.current_tenant is missing")
	}

	if _, err := tx.Exec(ctx, `SAVEPOINT sp_failclosed_payroll_runs;`); err != nil {
		fatal(err)
	}
	_, err = tx.Exec(ctx, `SELECT count(*) FROM staffing.payroll_runs;`)
	if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_failclosed_payroll_runs;`); rbErr != nil {
		fatal(rbErr)
	}
	if err == nil {
		fatalf("expected fail-closed error when app.current_tenant is missing")
	}

	if _, err := tx.Exec(ctx, `SAVEPOINT sp_failclosed_payslip_items;`); err != nil {
		fatal(err)
	}
	_, err = tx.Exec(ctx, `SELECT count(*) FROM staffing.payslip_items;`)
	if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_failclosed_payslip_items;`); rbErr != nil {
		fatal(rbErr)
	}
	if err == nil {
		fatalf("expected fail-closed error when app.current_tenant is missing")
	}

	if _, err := tx.Exec(ctx, `SAVEPOINT sp_failclosed_si_policies;`); err != nil {
		fatal(err)
	}
	_, err = tx.Exec(ctx, `SELECT count(*) FROM staffing.social_insurance_policies;`)
	if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_failclosed_si_policies;`); rbErr != nil {
		fatal(rbErr)
	}
	if err == nil {
		fatalf("expected fail-closed error when app.current_tenant is missing")
	}

	if _, err := tx.Exec(ctx, `SAVEPOINT sp_failclosed_si_events;`); err != nil {
		fatal(err)
	}
	_, err = tx.Exec(ctx, `SELECT count(*) FROM staffing.social_insurance_policy_events;`)
	if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_failclosed_si_events;`); rbErr != nil {
		fatal(rbErr)
	}
	if err == nil {
		fatalf("expected fail-closed error when app.current_tenant is missing")
	}

	if _, err := tx.Exec(ctx, `SAVEPOINT sp_failclosed_si_versions;`); err != nil {
		fatal(err)
	}
	_, err = tx.Exec(ctx, `SELECT count(*) FROM staffing.social_insurance_policy_versions;`)
	if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_failclosed_si_versions;`); rbErr != nil {
		fatal(rbErr)
	}
	if err == nil {
		fatalf("expected fail-closed error when app.current_tenant is missing")
	}

	if _, err := tx.Exec(ctx, `SAVEPOINT sp_failclosed_payslip_si_items;`); err != nil {
		fatal(err)
	}
	_, err = tx.Exec(ctx, `SELECT count(*) FROM staffing.payslip_social_insurance_items;`)
	if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_failclosed_payslip_si_items;`); rbErr != nil {
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

	if _, err := tx.Exec(ctx, `DELETE FROM staffing.payslip_social_insurance_items WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM staffing.social_insurance_policy_versions WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM staffing.social_insurance_policy_events WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM staffing.social_insurance_policies WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM staffing.payslips WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM staffing.payroll_runs WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM staffing.payroll_run_events WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM staffing.pay_periods WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM staffing.pay_period_events WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
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
			    'base_salary', '30000.00',
			    'allocated_fte', '1.0',
			    'currency', 'CNY',
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

	payGroup := "monthly"
	ppStart := "2030-01-01"
	ppEndExcl := "2030-02-01"
	{
		seedHex := strings.ReplaceAll(requestIDPrefix, "-", "")
		if len(seedHex) >= 4 {
			if seed, err := strconv.ParseInt(seedHex[:4], 16, 64); err == nil {
				base := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
				start := base.AddDate(0, int(seed%600), 0)
				start = time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC)
				end := start.AddDate(0, 1, 0)
				ppStart = start.Format("2006-01-02")
				ppEndExcl = end.Format("2006-01-02")
			}
		}
	}

	ppStartTime, err := time.Parse("2006-01-02", ppStart)
	if err != nil {
		fatal(err)
	}
	ppEndExclTime, err := time.Parse("2006-01-02", ppEndExcl)
	if err != nil {
		fatal(err)
	}
	periodDays := int64(ppEndExclTime.Sub(ppStartTime) / (24 * time.Hour))
	if periodDays <= 0 {
		fatalf("expected pay period days > 0, got %d", periodDays)
	}

	formatMoney := func(cents int64) string {
		whole := cents / 100
		frac := cents % 100
		if frac < 0 {
			frac = -frac
		}
		return fmt.Sprintf("%d.%02d", whole, frac)
	}
	prorateCents := func(baseSalaryCents int64, fteNum int64, fteDen int64, overlapDays int64) int64 {
		num := baseSalaryCents * fteNum * overlapDays
		den := fteDen * periodDays
		if den <= 0 {
			fatalf("invalid denominator=%d", den)
		}
		q := num / den
		r := num % den
		if r < 0 {
			r = -r
		}
		if r*2 >= den {
			q++
		}
		return q
	}

	withSavepoint := func(name string, fn func()) {
		if _, err := tx.Exec(ctx, fmt.Sprintf("SAVEPOINT %s;", name)); err != nil {
			fatal(err)
		}
		fn()
		if _, err := tx.Exec(ctx, fmt.Sprintf("ROLLBACK TO SAVEPOINT %s;", name)); err != nil {
			fatal(err)
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf("RELEASE SAVEPOINT %s;", name)); err != nil {
			fatal(err)
		}
	}

	resetPayrollAndAssignments := func() {
		if _, err := tx.Exec(ctx, `DELETE FROM staffing.payslips WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
			fatal(err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM staffing.payroll_runs WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
			fatal(err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM staffing.payroll_run_events WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
			fatal(err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM staffing.pay_periods WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
			fatal(err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM staffing.pay_period_events WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
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
	}

	mustUUID := func() string {
		var id string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&id); err != nil {
			fatal(err)
		}
		return id
	}

	{
		var gotHalfUp string
		if err := tx.QueryRow(ctx, `SELECT staffing.round_by_rule('1.005'::numeric, 'HALF_UP', 2::smallint)::text;`).Scan(&gotHalfUp); err != nil {
			fatal(err)
		}
		if gotHalfUp != "1.01" {
			fatalf("unexpected round_by_rule HALF_UP: %s", gotHalfUp)
		}

		var gotCeil string
		if err := tx.QueryRow(ctx, `SELECT trim_scale(staffing.round_by_rule('1.01'::numeric, 'CEIL', 1::smallint))::text;`).Scan(&gotCeil); err != nil {
			fatal(err)
		}
		if gotCeil != "1.1" {
			fatalf("unexpected round_by_rule CEIL: %s", gotCeil)
		}
	}

	cityCode := "CN-310000"
	hukouType := "default"
	policyEffectiveDate := effectiveDate
	policyBaseFloor := "0.00"
	policyBaseCeiling := "999999.00"
	policyRoundingRule := "HALF_UP"
	policyPrecision := "2"

	type siSeed struct {
		insuranceType string
		employerRate  string
		employeeRate  string
	}
	seeds := []siSeed{
		{insuranceType: "PENSION", employerRate: "0.16", employeeRate: "0.08"},
		{insuranceType: "MEDICAL", employerRate: "0.10", employeeRate: "0.02"},
		{insuranceType: "UNEMPLOYMENT", employerRate: "0.01", employeeRate: "0.005"},
		{insuranceType: "INJURY", employerRate: "0.002", employeeRate: "0"},
		{insuranceType: "MATERNITY", employerRate: "0.01", employeeRate: "0"},
		{insuranceType: "HOUSING_FUND", employerRate: "0.07", employeeRate: "0.07"},
	}

	policyIDByType := map[string]string{}
	for _, seed := range seeds {
		policyID := mustUUID()
		eventID := mustUUID()
		var eventDBID int64
		if err := tx.QueryRow(ctx, `
SELECT staffing.submit_social_insurance_policy_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::text,
  $5::text,
  $6::text,
  'CREATE',
  $7::date,
  jsonb_build_object(
    'employer_rate', $8::text,
    'employee_rate', $9::text,
    'base_floor', $10::text,
    'base_ceiling', $11::text,
    'rounding_rule', $12::text,
    'precision', $13::text,
    'rules_config', '{}'::jsonb
  ),
  $14::text,
  $15::uuid
);
`, eventID, tenantA, policyID, cityCode, hukouType, seed.insuranceType, policyEffectiveDate, seed.employerRate, seed.employeeRate, policyBaseFloor, policyBaseCeiling, policyRoundingRule, policyPrecision, eventID, initiatorID).Scan(&eventDBID); err != nil {
			fatal(err)
		}
		if eventDBID <= 0 {
			fatalf("expected social_insurance_policy_event db id > 0, got %d", eventDBID)
		}
		policyIDByType[seed.insuranceType] = policyID
	}

	withSavepoint("sp_si_multi_city", func() {
		eventID := mustUUID()
		policyID := mustUUID()
		_, err := tx.Exec(ctx, `
SELECT staffing.submit_social_insurance_policy_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  'CN-110000',
  $4::text,
  'PENSION',
  'CREATE',
  $5::date,
  jsonb_build_object(
    'employer_rate', '0.16',
    'employee_rate', '0.08',
    'base_floor', '0.00',
    'base_ceiling', '999999.00',
    'rounding_rule', 'HALF_UP',
    'precision', '2',
    'rules_config', '{}'::jsonb
  ),
  $6::text,
  $7::uuid
);
`, eventID, tenantA, policyID, hukouType, policyEffectiveDate, eventID, initiatorID)
		if err == nil {
			fatalf("expected multi-city to fail")
		}
		if msg, ok := pgErrorMessage(err); !ok || msg != "STAFFING_PAYROLL_SI_MULTI_CITY_NOT_SUPPORTED" {
			fatalf("expected pg error message=STAFFING_PAYROLL_SI_MULTI_CITY_NOT_SUPPORTED, got ok=%v message=%q err=%v", ok, msg, err)
		}
	})

	withSavepoint("sp_si_one_per_day", func() {
		eventID := mustUUID()
		_, err := tx.Exec(ctx, `
SELECT staffing.submit_social_insurance_policy_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::text,
  $5::text,
  'PENSION',
  'UPDATE',
  $6::date,
  jsonb_build_object(
    'employer_rate', '0.16',
    'employee_rate', '0.08',
    'base_floor', '0.00',
    'base_ceiling', '999999.00',
    'rounding_rule', 'HALF_UP',
    'precision', '2',
    'rules_config', '{}'::jsonb
  ),
  $7::text,
  $8::uuid
);
`, eventID, tenantA, policyIDByType["PENSION"], cityCode, hukouType, policyEffectiveDate, eventID, initiatorID)
		if err == nil {
			fatalf("expected one-per-day conflict to fail")
		}
		if msg, ok := pgErrorMessage(err); !ok || msg != "STAFFING_PAYROLL_SI_POLICY_EVENT_ONE_PER_DAY_CONFLICT" {
			fatalf("expected pg error message=STAFFING_PAYROLL_SI_POLICY_EVENT_ONE_PER_DAY_CONFLICT, got ok=%v message=%q err=%v", ok, msg, err)
		}
	})

	withSavepoint("sp_si_missing_policy_asof", func() {
		if _, err := tx.Exec(ctx, `
DELETE FROM staffing.social_insurance_policy_versions
WHERE tenant_id = $1::uuid AND insurance_type = 'PENSION';
`, tenantA); err != nil {
			fatal(err)
		}

		ppID := mustUUID()
		ppEventID := mustUUID()
		if _, err := tx.Exec(ctx, `
SELECT staffing.submit_payroll_pay_period_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::text,
  daterange($5::date, $6::date, '[)'),
  $7::text,
  $8::uuid
);
`, ppEventID, tenantA, ppID, payGroup, ppStart, ppEndExcl, ppEventID, initiatorID); err != nil {
			fatal(err)
		}

		runID := mustUUID()
		createEventID := mustUUID()
		if _, err := tx.Exec(ctx, `
SELECT staffing.submit_payroll_run_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::uuid,
  'CREATE',
  '{}'::jsonb,
  $5::text,
  $6::uuid
);
`, createEventID, tenantA, runID, ppID, createEventID, initiatorID); err != nil {
			fatal(err)
		}

		startEventID := mustUUID()
		if _, err := tx.Exec(ctx, `
SELECT staffing.submit_payroll_run_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::uuid,
  'CALC_START',
  '{}'::jsonb,
  $5::text,
  $6::uuid
);
`, startEventID, tenantA, runID, ppID, startEventID, initiatorID); err != nil {
			fatal(err)
		}

		finishEventID := mustUUID()
		_, err := tx.Exec(ctx, `
SELECT staffing.submit_payroll_run_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::uuid,
  'CALC_FINISH',
  '{}'::jsonb,
  $5::text,
  $6::uuid
);
`, finishEventID, tenantA, runID, ppID, finishEventID, initiatorID)
		if err == nil {
			fatalf("expected CALC_FINISH to fail when policy is missing")
		}
		if msg, ok := pgErrorMessage(err); !ok || msg != "STAFFING_PAYROLL_SI_POLICY_NOT_FOUND_AS_OF" {
			fatalf("expected pg error message=STAFFING_PAYROLL_SI_POLICY_NOT_FOUND_AS_OF, got ok=%v message=%q err=%v", ok, msg, err)
		}
	})

	withSavepoint("sp_si_changed_within_period", func() {
		mid := ppStartTime.AddDate(0, 0, 1).Format("2006-01-02")
		eventID := mustUUID()
		if _, err := tx.Exec(ctx, `
SELECT staffing.submit_social_insurance_policy_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::text,
  $5::text,
  'PENSION',
  'UPDATE',
  $6::date,
  jsonb_build_object(
    'employer_rate', '0.16',
    'employee_rate', '0.08',
    'base_floor', '0.00',
    'base_ceiling', '999999.00',
    'rounding_rule', 'HALF_UP',
    'precision', '2',
    'rules_config', '{}'::jsonb
  ),
  $7::text,
  $8::uuid
);
`, eventID, tenantA, policyIDByType["PENSION"], cityCode, hukouType, mid, eventID, initiatorID); err != nil {
			fatal(err)
		}

		ppID := mustUUID()
		ppEventID := mustUUID()
		if _, err := tx.Exec(ctx, `
SELECT staffing.submit_payroll_pay_period_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::text,
  daterange($5::date, $6::date, '[)'),
  $7::text,
  $8::uuid
);
`, ppEventID, tenantA, ppID, payGroup, ppStart, ppEndExcl, ppEventID, initiatorID); err != nil {
			fatal(err)
		}

		runID := mustUUID()
		createEventID := mustUUID()
		if _, err := tx.Exec(ctx, `
SELECT staffing.submit_payroll_run_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::uuid,
  'CREATE',
  '{}'::jsonb,
  $5::text,
  $6::uuid
);
`, createEventID, tenantA, runID, ppID, createEventID, initiatorID); err != nil {
			fatal(err)
		}

		startEventID := mustUUID()
		if _, err := tx.Exec(ctx, `
SELECT staffing.submit_payroll_run_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::uuid,
  'CALC_START',
  '{}'::jsonb,
  $5::text,
  $6::uuid
);
`, startEventID, tenantA, runID, ppID, startEventID, initiatorID); err != nil {
			fatal(err)
		}

		finishEventID := mustUUID()
		_, err := tx.Exec(ctx, `
SELECT staffing.submit_payroll_run_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::uuid,
  'CALC_FINISH',
  '{}'::jsonb,
  $5::text,
  $6::uuid
);
`, finishEventID, tenantA, runID, ppID, finishEventID, initiatorID)
		if err == nil {
			fatalf("expected CALC_FINISH to fail when policy changed within period")
		}
		if msg, ok := pgErrorMessage(err); !ok || msg != "STAFFING_PAYROLL_SI_POLICY_CHANGED_WITHIN_PERIOD" {
			fatalf("expected pg error message=STAFFING_PAYROLL_SI_POLICY_CHANGED_WITHIN_PERIOD, got ok=%v message=%q err=%v", ok, msg, err)
		}
	})

	var payPeriodID string
	var payPeriodEventID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&payPeriodID); err != nil {
		fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&payPeriodEventID); err != nil {
		fatal(err)
	}

	var payPeriodEventDBID int64
	if err := tx.QueryRow(ctx, `
SELECT staffing.submit_payroll_pay_period_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::text,
  daterange($5::date, $6::date, '[)'),
  $7::text,
  $8::uuid
);
`, payPeriodEventID, tenantA, payPeriodID, payGroup, ppStart, ppEndExcl, payPeriodEventID, initiatorID).Scan(&payPeriodEventDBID); err != nil {
		fatal(err)
	}
	if payPeriodEventDBID <= 0 {
		fatalf("expected pay period event db id > 0, got %d", payPeriodEventDBID)
	}

	var payPeriodEventDBID2 int64
	if err := tx.QueryRow(ctx, `
SELECT staffing.submit_payroll_pay_period_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::text,
  daterange($5::date, $6::date, '[)'),
  $7::text,
  $8::uuid
);
`, payPeriodEventID, tenantA, payPeriodID, payGroup, ppStart, ppEndExcl, payPeriodEventID, initiatorID).Scan(&payPeriodEventDBID2); err != nil {
		fatal(err)
	}
	if payPeriodEventDBID2 != payPeriodEventDBID {
		fatalf("expected idempotent pay period event id=%d, got %d", payPeriodEventDBID, payPeriodEventDBID2)
	}

	var periodStatus string
	if err := tx.QueryRow(ctx, `
SELECT status
FROM staffing.pay_periods
WHERE tenant_id = $1::uuid AND id = $2::uuid;
`, tenantA, payPeriodID).Scan(&periodStatus); err != nil {
		fatal(err)
	}
	if periodStatus != "open" {
		fatalf("expected pay period status=open, got %s", periodStatus)
	}

	var payPeriodID2 string
	var payPeriodEventID3 string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&payPeriodID2); err != nil {
		fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&payPeriodEventID3); err != nil {
		fatal(err)
	}

	if _, err := tx.Exec(ctx, `SAVEPOINT sp_pp_overlap;`); err != nil {
		fatal(err)
	}
	_, err = tx.Exec(ctx, `
	SELECT staffing.submit_payroll_pay_period_event(
	  $1::uuid,
	  $2::uuid,
	  $3::uuid,
	  $4::text,
	  daterange(($5::date + 14), ($6::date + 14), '[)'),
	  $7::text,
	  $8::uuid
	);
	`, payPeriodEventID3, tenantA, payPeriodID2, payGroup, ppStart, ppEndExcl, payPeriodEventID3, initiatorID)
	if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_pp_overlap;`); rbErr != nil {
		fatal(rbErr)
	}
	if err == nil {
		fatalf("expected submit_payroll_pay_period_event to fail on overlap")
	}
	if msg, ok := pgErrorMessage(err); !ok || msg != "STAFFING_PAYROLL_PAY_PERIOD_OVERLAP" {
		fatalf("expected pg error message=STAFFING_PAYROLL_PAY_PERIOD_OVERLAP, got ok=%v message=%q err=%v", ok, msg, err)
	}

	var runID string
	var runCreateEventID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&runID); err != nil {
		fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&runCreateEventID); err != nil {
		fatal(err)
	}

	var runCreateEventDBID int64
	if err := tx.QueryRow(ctx, `
SELECT staffing.submit_payroll_run_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::uuid,
  'CREATE',
  '{}'::jsonb,
  $5::text,
  $6::uuid
);
`, runCreateEventID, tenantA, runID, payPeriodID, runCreateEventID, initiatorID).Scan(&runCreateEventDBID); err != nil {
		fatal(err)
	}
	if runCreateEventDBID <= 0 {
		fatalf("expected run create event db id > 0, got %d", runCreateEventDBID)
	}

	var illegalFinalizeEventID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&illegalFinalizeEventID); err != nil {
		fatal(err)
	}

	if _, err := tx.Exec(ctx, `SAVEPOINT sp_run_illegal_finalize;`); err != nil {
		fatal(err)
	}
	_, err = tx.Exec(ctx, `
SELECT staffing.submit_payroll_run_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::uuid,
  'FINALIZE',
  '{}'::jsonb,
  $5::text,
  $6::uuid
);
`, illegalFinalizeEventID, tenantA, runID, payPeriodID, illegalFinalizeEventID, initiatorID)
	if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_run_illegal_finalize;`); rbErr != nil {
		fatal(rbErr)
	}
	if err == nil {
		fatalf("expected FINALIZE to fail when run_state=draft")
	}
	if msg, ok := pgErrorMessage(err); !ok || msg != "STAFFING_PAYROLL_RUN_INVALID_TRANSITION" {
		fatalf("expected pg error message=STAFFING_PAYROLL_RUN_INVALID_TRANSITION, got ok=%v message=%q err=%v", ok, msg, err)
	}

	var calcStartEventID string
	var calcFinishEventID string
	var finalizeEventID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&calcStartEventID); err != nil {
		fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&calcFinishEventID); err != nil {
		fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&finalizeEventID); err != nil {
		fatal(err)
	}

	if _, err := tx.Exec(ctx, `
	SELECT staffing.submit_payroll_run_event(
	  $1::uuid,
	  $2::uuid,
	  $3::uuid,
  $4::uuid,
  'CALC_START',
  '{}'::jsonb,
  $5::text,
  $6::uuid
);
`, calcStartEventID, tenantA, runID, payPeriodID, calcStartEventID, initiatorID); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `
SELECT staffing.submit_payroll_run_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::uuid,
  'CALC_FINISH',
  '{}'::jsonb,
  $5::text,
  $6::uuid
);
	`, calcFinishEventID, tenantA, runID, payPeriodID, calcFinishEventID, initiatorID); err != nil {
		fatal(err)
	}

	var payslipCount int
	if err := tx.QueryRow(ctx, `
	SELECT count(*)
	FROM staffing.payslips
	WHERE tenant_id = $1::uuid AND run_id = $2::uuid;
	`, tenantA, runID).Scan(&payslipCount); err != nil {
		fatal(err)
	}
	if payslipCount != 1 {
		fatalf("expected payslips=1, got %d", payslipCount)
	}

	var payslipItemCount int
	if err := tx.QueryRow(ctx, `
	SELECT count(*)
	FROM staffing.payslip_items i
	JOIN staffing.payslips p ON p.tenant_id = i.tenant_id AND p.id = i.payslip_id
	WHERE p.tenant_id = $1::uuid AND p.run_id = $2::uuid;
	`, tenantA, runID).Scan(&payslipItemCount); err != nil {
		fatal(err)
	}
	if payslipItemCount < 1 {
		fatalf("expected payslip_items>=1, got %d", payslipItemCount)
	}

	var grossPay string
	var netPay string
	var employerTotal string
	if err := tx.QueryRow(ctx, `
		SELECT
		  gross_pay::text,
		  net_pay::text,
		  employer_total::text
		FROM staffing.payslips
		WHERE tenant_id = $1::uuid AND run_id = $2::uuid
		LIMIT 1;
		`, tenantA, runID).Scan(&grossPay, &netPay, &employerTotal); err != nil {
		fatal(err)
	}
	if grossPay != "30000.00" || netPay != "24750.00" || employerTotal != "10560.00" {
		fatalf("unexpected payslip totals gross=%s net=%s employer_total=%s", grossPay, netPay, employerTotal)
	}

	var sumItems string
	if err := tx.QueryRow(ctx, `
	SELECT COALESCE(sum(i.amount), 0)::text
	FROM staffing.payslip_items i
	JOIN staffing.payslips p ON p.tenant_id = i.tenant_id AND p.id = i.payslip_id
	WHERE p.tenant_id = $1::uuid AND p.run_id = $2::uuid;
		`, tenantA, runID).Scan(&sumItems); err != nil {
		fatal(err)
	}
	if sumItems != grossPay {
		fatalf("unexpected items sum=%s gross=%s", sumItems, grossPay)
	}

	var baseItemAmount string
	var baseOverlapDays int64
	var basePeriodDays int64
	if err := tx.QueryRow(ctx, `
	SELECT
	  i.amount::text,
	  (i.meta->>'overlap_days')::bigint,
	  (i.meta->>'period_days')::bigint
	FROM staffing.payslip_items i
	JOIN staffing.payslips p ON p.tenant_id = i.tenant_id AND p.id = i.payslip_id
	WHERE p.tenant_id = $1::uuid AND p.run_id = $2::uuid AND i.item_code = 'EARNING_BASE_SALARY'
	LIMIT 1;
		`, tenantA, runID).Scan(&baseItemAmount, &baseOverlapDays, &basePeriodDays); err != nil {
		fatal(err)
	}
	if basePeriodDays != periodDays {
		fatalf("unexpected base item period_days=%d expected=%d", basePeriodDays, periodDays)
	}
	if baseOverlapDays != periodDays {
		fatalf("unexpected base item overlap_days=%d expected=%d", baseOverlapDays, periodDays)
	}
	if expected := formatMoney(prorateCents(3_000_000, 1, 1, baseOverlapDays)); baseItemAmount != expected {
		fatalf("unexpected base item amount=%s expected=%s", baseItemAmount, expected)
	}

	var finalizeEventDBID int64
	if err := tx.QueryRow(ctx, `
SELECT staffing.submit_payroll_run_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::uuid,
  'FINALIZE',
  '{}'::jsonb,
  $5::text,
  $6::uuid
);
`, finalizeEventID, tenantA, runID, payPeriodID, finalizeEventID, initiatorID).Scan(&finalizeEventDBID); err != nil {
		fatal(err)
	}
	if finalizeEventDBID <= 0 {
		fatalf("expected finalize event db id > 0, got %d", finalizeEventDBID)
	}

	var finalizeEventDBID2 int64
	if err := tx.QueryRow(ctx, `
SELECT staffing.submit_payroll_run_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::uuid,
  'FINALIZE',
  '{}'::jsonb,
  $5::text,
  $6::uuid
);
`, finalizeEventID, tenantA, runID, payPeriodID, finalizeEventID, initiatorID).Scan(&finalizeEventDBID2); err != nil {
		fatal(err)
	}
	if finalizeEventDBID2 != finalizeEventDBID {
		fatalf("expected idempotent finalize event id=%d, got %d", finalizeEventDBID, finalizeEventDBID2)
	}

	if err := tx.QueryRow(ctx, `
SELECT status
FROM staffing.pay_periods
WHERE tenant_id = $1::uuid AND id = $2::uuid;
`, tenantA, payPeriodID).Scan(&periodStatus); err != nil {
		fatal(err)
	}
	if periodStatus != "closed" {
		fatalf("expected pay period status=closed after finalize, got %s", periodStatus)
	}

	var postFinalizeCalcEventID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&postFinalizeCalcEventID); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `SAVEPOINT sp_post_finalize_calc;`); err != nil {
		fatal(err)
	}
	_, err = tx.Exec(ctx, `
SELECT staffing.submit_payroll_run_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::uuid,
  'CALC_START',
  '{}'::jsonb,
  $5::text,
  $6::uuid
);
`, postFinalizeCalcEventID, tenantA, runID, payPeriodID, postFinalizeCalcEventID, initiatorID)
	if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_post_finalize_calc;`); rbErr != nil {
		fatal(rbErr)
	}
	if err == nil {
		fatalf("expected CALC_START to fail after finalize")
	}
	if msg, ok := pgErrorMessage(err); !ok || msg != "STAFFING_PAYROLL_RUN_FINALIZED_READONLY" {
		fatalf("expected pg error message=STAFFING_PAYROLL_RUN_FINALIZED_READONLY, got ok=%v message=%q err=%v", ok, msg, err)
	}

	var postFinalizeFinalizeEventID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&postFinalizeFinalizeEventID); err != nil {
		fatal(err)
	}
	if _, err := tx.Exec(ctx, `SAVEPOINT sp_post_finalize_finalize;`); err != nil {
		fatal(err)
	}
	_, err = tx.Exec(ctx, `
SELECT staffing.submit_payroll_run_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::uuid,
  'FINALIZE',
  '{}'::jsonb,
  $5::text,
  $6::uuid
);
`, postFinalizeFinalizeEventID, tenantA, runID, payPeriodID, postFinalizeFinalizeEventID, initiatorID)
	if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_post_finalize_finalize;`); rbErr != nil {
		fatal(rbErr)
	}
	if err == nil {
		fatalf("expected FINALIZE to fail after finalize")
	}
	if msg, ok := pgErrorMessage(err); !ok || msg != "STAFFING_PAYROLL_RUN_FINALIZED_READONLY" {
		fatalf("expected pg error message=STAFFING_PAYROLL_RUN_FINALIZED_READONLY, got ok=%v message=%q err=%v", ok, msg, err)
	}

	baseSalaryCents := int64(3_000_000)

	submitAssignmentCreate := func(effective string, baseSalaryText any, allocatedFTE string, currency string) (assignmentID string, personUUID string) {
		assignmentID = mustUUID()
		personUUID = mustUUID()
		assignmentEventID := mustUUID()
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
			    'base_salary', $7::text,
			    'allocated_fte', $8::text,
			    'currency', $9::text,
			    'profile', '{}'::jsonb
			  ),
			  $10::text,
			  $11::uuid
			);
			`, assignmentEventID, tenantA, assignmentID, personUUID, effective, positionID, baseSalaryText, allocatedFTE, currency, assignmentEventID, initiatorID); err != nil {
			fatal(err)
		}
		return assignmentID, personUUID
	}

	submitAssignmentUpdateBaseSalary := func(assignmentID string, personUUID string, effective string, baseSalary string) {
		assignmentEventID := mustUUID()
		if _, err := tx.Exec(ctx, `
			SELECT staffing.submit_assignment_event(
			  $1::uuid,
			  $2::uuid,
			  $3::uuid,
			  $4::uuid,
			  'primary',
			  'UPDATE',
			  $5::date,
			  jsonb_build_object('base_salary', $6::text),
			  $7::text,
			  $8::uuid
			);
			`, assignmentEventID, tenantA, assignmentID, personUUID, effective, baseSalary, assignmentEventID, initiatorID); err != nil {
			fatal(err)
		}
	}

	submitPayPeriod := func(payGroup string, start string, endExcl string) (payPeriodID string) {
		payPeriodID = mustUUID()
		payPeriodEventID := mustUUID()
		if _, err := tx.Exec(ctx, `
			SELECT staffing.submit_payroll_pay_period_event(
			  $1::uuid,
			  $2::uuid,
			  $3::uuid,
			  $4::text,
			  daterange($5::date, $6::date, '[)'),
			  $7::text,
			  $8::uuid
			);
			`, payPeriodEventID, tenantA, payPeriodID, payGroup, start, endExcl, payPeriodEventID, initiatorID); err != nil {
			fatal(err)
		}
		return payPeriodID
	}

	submitPayrollRunEvent := func(runID string, payPeriodID string, eventType string) error {
		eventID := mustUUID()
		_, err := tx.Exec(ctx, `
			SELECT staffing.submit_payroll_run_event(
			  $1::uuid,
			  $2::uuid,
			  $3::uuid,
			  $4::uuid,
			  $5::text,
			  '{}'::jsonb,
			  $6::text,
			  $7::uuid
			);
			`, eventID, tenantA, runID, payPeriodID, eventType, eventID, initiatorID)
		return err
	}

	createRun := func(payPeriodID string) (runID string) {
		runID = mustUUID()
		if err := submitPayrollRunEvent(runID, payPeriodID, "CREATE"); err != nil {
			fatal(err)
		}
		return runID
	}

	calcRun := func(payPeriodID string) (runID string) {
		runID = createRun(payPeriodID)
		if err := submitPayrollRunEvent(runID, payPeriodID, "CALC_START"); err != nil {
			fatal(err)
		}
		if err := submitPayrollRunEvent(runID, payPeriodID, "CALC_FINISH"); err != nil {
			fatal(err)
		}
		return runID
	}

	getBaseSalaryItem := func(runID string) (amount string, overlapDays int64) {
		if err := tx.QueryRow(ctx, `
			SELECT
			  i.amount::text,
			  (i.meta->>'overlap_days')::bigint
			FROM staffing.payslip_items i
			JOIN staffing.payslips p ON p.tenant_id = i.tenant_id AND p.id = i.payslip_id
			WHERE p.tenant_id = $1::uuid AND p.run_id = $2::uuid AND i.item_code = 'EARNING_BASE_SALARY'
			LIMIT 1;
			`, tenantA, runID).Scan(&amount, &overlapDays); err != nil {
			fatal(err)
		}
		return amount, overlapDays
	}

	withSavepoint("sp_prorate_midmonth", func() {
		resetPayrollAndAssignments()

		startOffsetDays := int64(15)
		if startOffsetDays >= periodDays {
			fatalf("expected startOffsetDays < periodDays, got %d >= %d", startOffsetDays, periodDays)
		}
		start := ppStartTime.AddDate(0, 0, int(startOffsetDays)).Format("2006-01-02")
		submitAssignmentCreate(start, "30000.00", "1.0", "CNY")

		payPeriodID := submitPayPeriod(payGroup, ppStart, ppEndExcl)
		runID := calcRun(payPeriodID)

		itemAmount, overlapDays := getBaseSalaryItem(runID)
		if overlapDays != periodDays-startOffsetDays {
			fatalf("unexpected overlap_days=%d expected=%d", overlapDays, periodDays-startOffsetDays)
		}
		if expected := formatMoney(prorateCents(baseSalaryCents, 1, 1, overlapDays)); itemAmount != expected {
			fatalf("unexpected prorate midmonth amount=%s expected=%s", itemAmount, expected)
		}
	})

	withSavepoint("sp_prorate_last_day_repeating", func() {
		resetPayrollAndAssignments()

		start := ppEndExclTime.AddDate(0, 0, -1).Format("2006-01-02")
		submitAssignmentCreate(start, "30000.00", "1.0", "CNY")

		payPeriodID := submitPayPeriod(payGroup, ppStart, ppEndExcl)
		runID := calcRun(payPeriodID)

		itemAmount, overlapDays := getBaseSalaryItem(runID)
		if overlapDays != 1 {
			fatalf("unexpected overlap_days=%d expected=1", overlapDays)
		}
		if expected := formatMoney(prorateCents(baseSalaryCents, 1, 1, overlapDays)); itemAmount != expected {
			fatalf("unexpected prorate last-day amount=%s expected=%s", itemAmount, expected)
		}
	})

	withSavepoint("sp_fte_half_full", func() {
		resetPayrollAndAssignments()

		submitAssignmentCreate(effectiveDate, "30000.00", "0.5", "CNY")

		payPeriodID := submitPayPeriod(payGroup, ppStart, ppEndExcl)
		runID := calcRun(payPeriodID)

		itemAmount, overlapDays := getBaseSalaryItem(runID)
		if overlapDays != periodDays {
			fatalf("unexpected overlap_days=%d expected=%d", overlapDays, periodDays)
		}
		if expected := formatMoney(prorateCents(baseSalaryCents, 1, 2, overlapDays)); itemAmount != expected {
			fatalf("unexpected fte-half amount=%s expected=%s", itemAmount, expected)
		}
	})

	withSavepoint("sp_salary_change", func() {
		resetPayrollAndAssignments()

		changeDays := int64(15)
		if changeDays >= periodDays {
			fatalf("expected changeDays < periodDays, got %d >= %d", changeDays, periodDays)
		}
		changeDate := ppStartTime.AddDate(0, 0, int(changeDays)).Format("2006-01-02")

		assignmentID, personUUID := submitAssignmentCreate(ppStart, "30000.00", "1.0", "CNY")
		submitAssignmentUpdateBaseSalary(assignmentID, personUUID, changeDate, "31000.00")

		payPeriodID := submitPayPeriod(payGroup, ppStart, ppEndExcl)
		runID := calcRun(payPeriodID)

		type row struct {
			amount      string
			segmentDate string
			overlapDays int64
		}
		rows, err := tx.Query(ctx, `
			SELECT
			  i.amount::text,
			  (i.meta->>'segment_start')::text,
			  (i.meta->>'overlap_days')::bigint
			FROM staffing.payslip_items i
			JOIN staffing.payslips p ON p.tenant_id = i.tenant_id AND p.id = i.payslip_id
			WHERE p.tenant_id = $1::uuid AND p.run_id = $2::uuid AND i.item_code = 'EARNING_BASE_SALARY'
			ORDER BY (i.meta->>'segment_start')::date ASC;
			`, tenantA, runID)
		if err != nil {
			fatal(err)
		}
		defer rows.Close()
		var got []row
		for rows.Next() {
			var r row
			if err := rows.Scan(&r.amount, &r.segmentDate, &r.overlapDays); err != nil {
				fatal(err)
			}
			got = append(got, r)
		}
		if err := rows.Err(); err != nil {
			fatal(err)
		}
		if len(got) != 2 {
			fatalf("expected 2 base salary items for mid-period change, got %d", len(got))
		}

		if got[0].segmentDate != ppStart || got[0].overlapDays != changeDays {
			fatalf("unexpected first segment_start=%s overlap_days=%d expected_start=%s expected_days=%d", got[0].segmentDate, got[0].overlapDays, ppStart, changeDays)
		}
		if expected := formatMoney(prorateCents(baseSalaryCents, 1, 1, got[0].overlapDays)); got[0].amount != expected {
			fatalf("unexpected first segment amount=%s expected=%s", got[0].amount, expected)
		}

		remainDays := periodDays - changeDays
		if got[1].segmentDate != changeDate || got[1].overlapDays != remainDays {
			fatalf("unexpected second segment_start=%s overlap_days=%d expected_start=%s expected_days=%d", got[1].segmentDate, got[1].overlapDays, changeDate, remainDays)
		}
		if expected := formatMoney(prorateCents(3_100_000, 1, 1, got[1].overlapDays)); got[1].amount != expected {
			fatalf("unexpected second segment amount=%s expected=%s", got[1].amount, expected)
		}

		var gross string
		var sum string
		if err := tx.QueryRow(ctx, `
			SELECT p.gross_pay::text
			FROM staffing.payslips p
			WHERE p.tenant_id = $1::uuid AND p.run_id = $2::uuid
			LIMIT 1;
			`, tenantA, runID).Scan(&gross); err != nil {
			fatal(err)
		}
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(sum(i.amount), 0)::text
			FROM staffing.payslip_items i
			JOIN staffing.payslips p ON p.tenant_id = i.tenant_id AND p.id = i.payslip_id
			WHERE p.tenant_id = $1::uuid AND p.run_id = $2::uuid;
			`, tenantA, runID).Scan(&sum); err != nil {
			fatal(err)
		}
		if gross != sum {
			fatalf("unexpected salary-change gross=%s sum=%s", gross, sum)
		}
	})

	withSavepoint("sp_round_half_up_boundary", func() {
		resetPayrollAndAssignments()

		start := ppEndExclTime.AddDate(0, 0, -1).Format("2006-01-02")
		baseSalaryBoundaryCents := periodDays
		baseSalaryBoundary := formatMoney(baseSalaryBoundaryCents)
		submitAssignmentCreate(start, baseSalaryBoundary, "0.5", "CNY")

		payPeriodID := submitPayPeriod(payGroup, ppStart, ppEndExcl)
		runID := calcRun(payPeriodID)

		itemAmount, overlapDays := getBaseSalaryItem(runID)
		if overlapDays != 1 {
			fatalf("unexpected overlap_days=%d expected=1", overlapDays)
		}
		if expected := formatMoney(prorateCents(baseSalaryBoundaryCents, 1, 2, overlapDays)); itemAmount != expected {
			fatalf("unexpected rounding boundary amount=%s expected=%s", itemAmount, expected)
		}
		if itemAmount != "0.01" {
			fatalf("expected rounding boundary amount=0.01, got %s (period_days=%d base_salary=%s)", itemAmount, periodDays, baseSalaryBoundary)
		}
	})

	withSavepoint("sp_fail_missing_base_salary", func() {
		resetPayrollAndAssignments()

		submitAssignmentCreate(effectiveDate, nil, "1.0", "CNY")

		payPeriodID := submitPayPeriod(payGroup, ppStart, ppEndExcl)
		runID := createRun(payPeriodID)
		if err := submitPayrollRunEvent(runID, payPeriodID, "CALC_START"); err != nil {
			fatal(err)
		}

		if _, err := tx.Exec(ctx, `SAVEPOINT sp_calc_finish_missing_salary;`); err != nil {
			fatal(err)
		}
		err := submitPayrollRunEvent(runID, payPeriodID, "CALC_FINISH")
		if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_calc_finish_missing_salary;`); rbErr != nil {
			fatal(rbErr)
		}
		if err == nil {
			fatalf("expected CALC_FINISH to fail when base_salary is missing")
		}
		if msg, ok := pgErrorMessage(err); !ok || msg != "STAFFING_PAYROLL_MISSING_BASE_SALARY" {
			fatalf("expected pg error message=STAFFING_PAYROLL_MISSING_BASE_SALARY, got ok=%v message=%q err=%v", ok, msg, err)
		}

		if err := submitPayrollRunEvent(runID, payPeriodID, "CALC_FAIL"); err != nil {
			fatal(err)
		}
		var runState string
		if err := tx.QueryRow(ctx, `
			SELECT run_state
			FROM staffing.payroll_runs
			WHERE tenant_id = $1::uuid AND id = $2::uuid;
			`, tenantA, runID).Scan(&runState); err != nil {
			fatal(err)
		}
		if runState != "failed" {
			fatalf("expected run_state=failed after CALC_FAIL, got %s", runState)
		}
	})

	withSavepoint("sp_fail_unsupported_pay_group", func() {
		resetPayrollAndAssignments()

		submitAssignmentCreate(effectiveDate, "30000.00", "1.0", "CNY")

		payPeriodID := submitPayPeriod("weekly", ppStart, ppEndExcl)
		runID := createRun(payPeriodID)
		if err := submitPayrollRunEvent(runID, payPeriodID, "CALC_START"); err != nil {
			fatal(err)
		}

		if _, err := tx.Exec(ctx, `SAVEPOINT sp_calc_finish_bad_group;`); err != nil {
			fatal(err)
		}
		err := submitPayrollRunEvent(runID, payPeriodID, "CALC_FINISH")
		if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_calc_finish_bad_group;`); rbErr != nil {
			fatal(rbErr)
		}
		if err == nil {
			fatalf("expected CALC_FINISH to fail when pay_group is not monthly")
		}
		if msg, ok := pgErrorMessage(err); !ok || msg != "STAFFING_PAYROLL_PAY_GROUP_NOT_SUPPORTED" {
			fatalf("expected pg error message=STAFFING_PAYROLL_PAY_GROUP_NOT_SUPPORTED, got ok=%v message=%q err=%v", ok, msg, err)
		}

		if err := submitPayrollRunEvent(runID, payPeriodID, "CALC_FAIL"); err != nil {
			fatal(err)
		}
		var runState string
		if err := tx.QueryRow(ctx, `
			SELECT run_state
			FROM staffing.payroll_runs
			WHERE tenant_id = $1::uuid AND id = $2::uuid;
			`, tenantA, runID).Scan(&runState); err != nil {
			fatal(err)
		}
		if runState != "failed" {
			fatalf("expected run_state=failed after CALC_FAIL, got %s", runState)
		}
	})

	withSavepoint("sp_fail_unsupported_pay_period", func() {
		resetPayrollAndAssignments()

		submitAssignmentCreate(effectiveDate, "30000.00", "1.0", "CNY")

		unsupportedStart := ppStartTime.AddDate(0, 0, 1).Format("2006-01-02")
		unsupportedEnd := ppEndExclTime.AddDate(0, 0, 1).Format("2006-01-02")
		payPeriodID := submitPayPeriod(payGroup, unsupportedStart, unsupportedEnd)
		runID := createRun(payPeriodID)
		if err := submitPayrollRunEvent(runID, payPeriodID, "CALC_START"); err != nil {
			fatal(err)
		}

		if _, err := tx.Exec(ctx, `SAVEPOINT sp_calc_finish_bad_period;`); err != nil {
			fatal(err)
		}
		err := submitPayrollRunEvent(runID, payPeriodID, "CALC_FINISH")
		if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_calc_finish_bad_period;`); rbErr != nil {
			fatal(rbErr)
		}
		if err == nil {
			fatalf("expected CALC_FINISH to fail when period is not a natural month")
		}
		if msg, ok := pgErrorMessage(err); !ok || msg != "STAFFING_PAYROLL_PERIOD_NOT_NATURAL_MONTH" {
			fatalf("expected pg error message=STAFFING_PAYROLL_PERIOD_NOT_NATURAL_MONTH, got ok=%v message=%q err=%v", ok, msg, err)
		}

		if err := submitPayrollRunEvent(runID, payPeriodID, "CALC_FAIL"); err != nil {
			fatal(err)
		}
		var runState string
		if err := tx.QueryRow(ctx, `
			SELECT run_state
			FROM staffing.payroll_runs
			WHERE tenant_id = $1::uuid AND id = $2::uuid;
			`, tenantA, runID).Scan(&runState); err != nil {
			fatal(err)
		}
		if runState != "failed" {
			fatalf("expected run_state=failed after CALC_FAIL, got %s", runState)
		}
	})

	withSavepoint("sp_assignment_currency_unsupported", func() {
		resetPayrollAndAssignments()

		assignmentID := mustUUID()
		personUUID := mustUUID()
		assignmentEventID := mustUUID()

		if _, err := tx.Exec(ctx, `SAVEPOINT sp_bad_currency;`); err != nil {
			fatal(err)
		}
		_, err := tx.Exec(ctx, `
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
			    'base_salary', '30000.00',
			    'allocated_fte', '1.0',
			    'currency', 'USD',
			    'profile', '{}'::jsonb
			  ),
			  $7::text,
			  $8::uuid
			);
			`, assignmentEventID, tenantA, assignmentID, personUUID, effectiveDate, positionID, assignmentEventID, initiatorID)
		if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_bad_currency;`); rbErr != nil {
			fatal(rbErr)
		}
		if err == nil {
			fatalf("expected submit_assignment_event to fail when currency is non-CNY")
		}
		if msg, ok := pgErrorMessage(err); !ok || msg != "STAFFING_ASSIGNMENT_CURRENCY_UNSUPPORTED" {
			fatalf("expected pg error message=STAFFING_ASSIGNMENT_CURRENCY_UNSUPPORTED, got ok=%v message=%q err=%v", ok, msg, err)
		}
	})

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

	if err := tx2.QueryRow(ctx, `SELECT count(*) FROM staffing.pay_periods;`).Scan(&crossCount); err != nil {
		fatal(err)
	}
	if crossCount != 0 {
		fatalf("expected pay_periods count=0 under tenant B, got %d", crossCount)
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
