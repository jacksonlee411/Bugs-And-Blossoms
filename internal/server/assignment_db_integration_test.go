package server

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestStaffingAssignmentDB_RerunnableUpsert(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	adminConn, adminDSN, ok := connectTestPostgres(ctx, t)
	if !ok {
		return
	}
	t.Cleanup(func() { _ = adminConn.Close(context.Background()) })

	if err := ensureStaffingAssignmentSchemaForTest(ctx, adminConn); err != nil {
		t.Fatal(err)
	}

	runtimeDSN, err := withUserPassword(adminDSN, "bb_test_runtime", "bb_test_runtime")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := pgx.Connect(ctx, runtimeDSN)
	if err != nil {
		t.Fatalf("connect runtime role: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })

	// RLS fail-closed smoke (No Tx, No RLS).
	func() {
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()

		var n int
		err = tx.QueryRow(ctx, `SELECT count(*) FROM staffing.assignment_events;`).Scan(&n)
		if err == nil {
			t.Fatal("expected RLS fail-closed error when app.current_tenant is missing")
		}
	}()

	tenantA := "00000000-0000-0000-0000-0000000000a1"
	personUUID := "00000000-0000-0000-0000-0000000000c1"
	position1 := "00000000-0000-0000-0000-000000000011"
	position2 := "00000000-0000-0000-0000-000000000022"
	effectiveDate := "2026-01-01"

	func() {
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()

		if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantA); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO staffing.positions (tenant_id, id) VALUES ($1::uuid, $2::uuid) ON CONFLICT DO NOTHING;`, tenantA, position1); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO staffing.positions (tenant_id, id) VALUES ($1::uuid, $2::uuid) ON CONFLICT DO NOTHING;`, tenantA, position2); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM staffing.assignment_versions WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM staffing.assignment_event_rescinds WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM staffing.assignment_event_corrections WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM staffing.assignment_events WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM staffing.assignments WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}()

	store := newStaffingPGStore(conn)

	a1, err := store.UpsertPrimaryAssignmentForPerson(ctx, tenantA, effectiveDate, personUUID, position1, "", "", "")
	if err != nil {
		t.Fatalf("upsert-1: %v", err)
	}
	a2, err := store.UpsertPrimaryAssignmentForPerson(ctx, tenantA, effectiveDate, personUUID, position1, "", "", "")
	if err != nil {
		t.Fatalf("upsert-2 (rerun): %v", err)
	}
	if a1.AssignmentID == "" || a1.AssignmentID != a2.AssignmentID {
		t.Fatalf("expected same assignment_id on rerun, got %q vs %q", a1.AssignmentID, a2.AssignmentID)
	}

	func() {
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()

		if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantA); err != nil {
			t.Fatal(err)
		}
		var n int
		if err := tx.QueryRow(ctx, `
	SELECT count(*)
	FROM staffing.assignment_events
	WHERE tenant_id = $1::uuid AND assignment_id = $2::uuid AND effective_date = $3::date
	`, tenantA, a1.AssignmentID, effectiveDate).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("expected exactly 1 assignment_event after rerun, got %d", n)
		}
	}()

	_, err = store.UpsertPrimaryAssignmentForPerson(ctx, tenantA, effectiveDate, personUUID, position2, "", "", "")
	if err == nil {
		t.Fatal("expected error when reusing effective_date with different payload")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Message != "STAFFING_IDEMPOTENCY_REUSED" {
		t.Fatalf("expected STAFFING_IDEMPOTENCY_REUSED, got err=%v", err)
	}
}

func TestStaffingAssignmentDB_CorrectRescind(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	adminConn, adminDSN, ok := connectTestPostgres(ctx, t)
	if !ok {
		return
	}
	t.Cleanup(func() { _ = adminConn.Close(context.Background()) })

	if err := ensureStaffingAssignmentSchemaForTest(ctx, adminConn); err != nil {
		t.Fatal(err)
	}

	runtimeDSN, err := withUserPassword(adminDSN, "bb_test_runtime", "bb_test_runtime")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := pgx.Connect(ctx, runtimeDSN)
	if err != nil {
		t.Fatalf("connect runtime role: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })

	tenantA := "00000000-0000-0000-0000-0000000000a1"
	personUUID := "00000000-0000-0000-0000-0000000000c1"
	position1 := "00000000-0000-0000-0000-000000000011"
	position2 := "00000000-0000-0000-0000-000000000022"

	func() {
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()

		if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantA); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO staffing.positions (tenant_id, id) VALUES ($1::uuid, $2::uuid) ON CONFLICT DO NOTHING;`, tenantA, position1); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO staffing.positions (tenant_id, id) VALUES ($1::uuid, $2::uuid) ON CONFLICT DO NOTHING;`, tenantA, position2); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM staffing.assignment_versions WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM staffing.assignment_event_rescinds WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM staffing.assignment_event_corrections WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM staffing.assignment_events WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM staffing.assignments WHERE tenant_id = $1::uuid;`, tenantA); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}()

	store := newStaffingPGStore(conn)

	a1, err := store.UpsertPrimaryAssignmentForPerson(ctx, tenantA, "2026-01-01", personUUID, position1, "", "20000.00", "1.0")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = store.CorrectAssignmentEvent(ctx, tenantA, a1.AssignmentID, "2026-01-01", []byte(`{"position_id":"`+position1+`","base_salary":"30000.00"}`))
	if err != nil {
		t.Fatalf("correct: %v", err)
	}

	func() {
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()
		if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantA); err != nil {
			t.Fatal(err)
		}
		var baseSalary string
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(base_salary::text,'')
			FROM staffing.assignment_versions
			WHERE tenant_id = $1::uuid AND assignment_id = $2::uuid AND validity @> '2026-01-15'::date
			LIMIT 1;
		`, tenantA, a1.AssignmentID).Scan(&baseSalary); err != nil {
			t.Fatal(err)
		}
		if baseSalary != "30000.00" {
			t.Fatalf("expected base_salary=30000.00, got %q", baseSalary)
		}
	}()

	_, err = store.UpsertPrimaryAssignmentForPerson(ctx, tenantA, "2026-02-01", personUUID, position2, "", "", "")
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	_, err = store.RescindAssignmentEvent(ctx, tenantA, a1.AssignmentID, "2026-02-01", []byte(`{}`))
	if err != nil {
		t.Fatalf("rescind: %v", err)
	}

	func() {
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()
		if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantA); err != nil {
			t.Fatal(err)
		}
		var pos string
		if err := tx.QueryRow(ctx, `
			SELECT position_id::text
			FROM staffing.assignment_versions
			WHERE tenant_id = $1::uuid AND assignment_id = $2::uuid AND validity @> '2026-02-15'::date
			LIMIT 1;
		`, tenantA, a1.AssignmentID).Scan(&pos); err != nil {
			t.Fatal(err)
		}
		if pos != position1 {
			t.Fatalf("expected stitched position_id=%s, got %s", position1, pos)
		}
	}()

	_, err = store.RescindAssignmentEvent(ctx, tenantA, a1.AssignmentID, "2026-01-01", []byte(`{}`))
	if err == nil {
		t.Fatal("expected rescind CREATE to fail")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Message != "STAFFING_ASSIGNMENT_CREATE_CANNOT_RESCIND" {
		t.Fatalf("expected STAFFING_ASSIGNMENT_CREATE_CANNOT_RESCIND, got err=%v", err)
	}

	_, err = store.CorrectAssignmentEvent(ctx, tenantA, a1.AssignmentID, "2026-02-01", []byte(`{"position_id":"`+position2+`"}`))
	if err == nil {
		t.Fatal("expected correct rescinded to fail")
	}
	if !errors.As(err, &pgErr) || pgErr.Message != "STAFFING_ASSIGNMENT_EVENT_ALREADY_RESCINDED" {
		t.Fatalf("expected STAFFING_ASSIGNMENT_EVENT_ALREADY_RESCINDED, got err=%v", err)
	}
}

func ensureStaffingAssignmentSchemaForTest(ctx context.Context, conn *pgx.Conn) error {
	const runtimeRole = "bb_test_runtime"

	ddl := []string{
		`CREATE EXTENSION IF NOT EXISTS pgcrypto;`,
		`CREATE SCHEMA IF NOT EXISTS staffing;`,
		`
CREATE OR REPLACE FUNCTION staffing.assert_current_tenant(p_tenant_id uuid)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  IF current_setting('app.current_tenant', true) IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_TENANT_MISSING';
  END IF;
  IF current_setting('app.current_tenant')::uuid <> p_tenant_id THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_TENANT_MISMATCH';
  END IF;
END;
	$$;
	`,
		`
	CREATE TABLE IF NOT EXISTS staffing.positions (
	  tenant_id uuid NOT NULL,
	  id uuid NOT NULL,
	  created_at timestamptz NOT NULL DEFAULT now(),
	  updated_at timestamptz NOT NULL DEFAULT now(),
	  PRIMARY KEY (tenant_id, id)
	);
	`,
		`
	DO $$
	BEGIN
	  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '` + runtimeRole + `') THEN
    CREATE ROLE ` + runtimeRole + ` LOGIN PASSWORD '` + runtimeRole + `' NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
  END IF;
END
$$;
`,
		`
CREATE TABLE IF NOT EXISTS staffing.assignments (
  tenant_id uuid NOT NULL,
  id uuid PRIMARY KEY,
  person_uuid uuid NOT NULL,
  assignment_type text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT assignments_person_assignment_type_unique UNIQUE (tenant_id, person_uuid, assignment_type),
  CONSTRAINT assignments_assignment_type_check CHECK (assignment_type IN ('primary'))
);
`,
		`CREATE UNIQUE INDEX IF NOT EXISTS assignments_identity_unique ON staffing.assignments (tenant_id, person_uuid, assignment_type);`,
		`
CREATE TABLE IF NOT EXISTS staffing.assignment_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  assignment_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  assignment_type text NOT NULL,
  event_type text NOT NULL,
  effective_date date NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT assignment_events_event_type_check CHECK (event_type IN ('CREATE','UPDATE')),
  CONSTRAINT assignment_events_payload_is_object_check CHECK (jsonb_typeof(payload) = 'object'),
  CONSTRAINT assignment_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT assignment_events_one_per_day_unique UNIQUE (tenant_id, assignment_id, effective_date),
  CONSTRAINT assignment_events_request_id_unique UNIQUE (tenant_id, request_id)
);
`,
		`CREATE UNIQUE INDEX IF NOT EXISTS assignment_events_event_id_unique_idx ON staffing.assignment_events (event_id);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS assignment_events_one_per_day_unique_idx ON staffing.assignment_events (tenant_id, assignment_id, effective_date);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS assignment_events_request_id_unique_idx ON staffing.assignment_events (tenant_id, request_id);`,
		`
	CREATE TABLE IF NOT EXISTS staffing.assignment_event_corrections (
	  id bigserial PRIMARY KEY,
	  event_id uuid NOT NULL,
	  tenant_id uuid NOT NULL,
	  assignment_id uuid NOT NULL,
	  target_effective_date date NOT NULL,
	  replacement_payload jsonb NOT NULL,
	  request_id text NOT NULL,
	  initiator_id uuid NOT NULL,
	  transaction_time timestamptz NOT NULL DEFAULT now(),
	  created_at timestamptz NOT NULL DEFAULT now(),
	  CONSTRAINT assignment_event_corrections_payload_is_object_check CHECK (jsonb_typeof(replacement_payload) = 'object'),
	  CONSTRAINT assignment_event_corrections_event_id_unique UNIQUE (event_id),
	  CONSTRAINT assignment_event_corrections_target_unique UNIQUE (tenant_id, assignment_id, target_effective_date),
	  CONSTRAINT assignment_event_corrections_request_id_unique UNIQUE (tenant_id, request_id)
	);
	`,
		`
	CREATE TABLE IF NOT EXISTS staffing.assignment_event_rescinds (
	  id bigserial PRIMARY KEY,
	  event_id uuid NOT NULL,
	  tenant_id uuid NOT NULL,
	  assignment_id uuid NOT NULL,
	  target_effective_date date NOT NULL,
	  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
	  request_id text NOT NULL,
	  initiator_id uuid NOT NULL,
	  transaction_time timestamptz NOT NULL DEFAULT now(),
	  created_at timestamptz NOT NULL DEFAULT now(),
	  CONSTRAINT assignment_event_rescinds_payload_is_object_check CHECK (jsonb_typeof(payload) = 'object'),
	  CONSTRAINT assignment_event_rescinds_event_id_unique UNIQUE (event_id),
	  CONSTRAINT assignment_event_rescinds_target_unique UNIQUE (tenant_id, assignment_id, target_effective_date),
	  CONSTRAINT assignment_event_rescinds_request_id_unique UNIQUE (tenant_id, request_id)
	);
	`,
		`
	CREATE TABLE IF NOT EXISTS staffing.assignment_versions (
	  id bigserial PRIMARY KEY,
	  tenant_id uuid NOT NULL,
	  assignment_id uuid NOT NULL,
	  person_uuid uuid NOT NULL,
	  position_id uuid NOT NULL,
	  assignment_type text NOT NULL,
	  status text NOT NULL DEFAULT 'active',
	  allocated_fte numeric(9,2) NOT NULL DEFAULT 1.0,
	  base_salary numeric(15,2) NULL,
	  validity daterange NOT NULL,
	  last_event_id bigint NOT NULL
	);
	`,
		`ALTER TABLE staffing.positions ENABLE ROW LEVEL SECURITY;`,
		`ALTER TABLE staffing.positions FORCE ROW LEVEL SECURITY;`,
		`DROP POLICY IF EXISTS tenant_isolation ON staffing.positions;`,
		`
	CREATE POLICY tenant_isolation ON staffing.positions
	USING (tenant_id = current_setting('app.current_tenant')::uuid)
	WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
	`,
		`ALTER TABLE staffing.assignments ENABLE ROW LEVEL SECURITY;`,
		`ALTER TABLE staffing.assignments FORCE ROW LEVEL SECURITY;`,
		`DROP POLICY IF EXISTS tenant_isolation ON staffing.assignments;`,
		`
CREATE POLICY tenant_isolation ON staffing.assignments
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
`,
		`ALTER TABLE staffing.assignment_events ENABLE ROW LEVEL SECURITY;`,
		`ALTER TABLE staffing.assignment_events FORCE ROW LEVEL SECURITY;`,
		`DROP POLICY IF EXISTS tenant_isolation ON staffing.assignment_events;`,
		`
	CREATE POLICY tenant_isolation ON staffing.assignment_events
	USING (tenant_id = current_setting('app.current_tenant')::uuid)
	WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
	`,
		`ALTER TABLE staffing.assignment_event_corrections ENABLE ROW LEVEL SECURITY;`,
		`ALTER TABLE staffing.assignment_event_corrections FORCE ROW LEVEL SECURITY;`,
		`DROP POLICY IF EXISTS tenant_isolation ON staffing.assignment_event_corrections;`,
		`
	CREATE POLICY tenant_isolation ON staffing.assignment_event_corrections
	USING (tenant_id = current_setting('app.current_tenant')::uuid)
	WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
	`,
		`ALTER TABLE staffing.assignment_event_rescinds ENABLE ROW LEVEL SECURITY;`,
		`ALTER TABLE staffing.assignment_event_rescinds FORCE ROW LEVEL SECURITY;`,
		`DROP POLICY IF EXISTS tenant_isolation ON staffing.assignment_event_rescinds;`,
		`
	CREATE POLICY tenant_isolation ON staffing.assignment_event_rescinds
	USING (tenant_id = current_setting('app.current_tenant')::uuid)
	WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
	`,
		`ALTER TABLE staffing.assignment_versions ENABLE ROW LEVEL SECURITY;`,
		`ALTER TABLE staffing.assignment_versions FORCE ROW LEVEL SECURITY;`,
		`DROP POLICY IF EXISTS tenant_isolation ON staffing.assignment_versions;`,
		`
	CREATE POLICY tenant_isolation ON staffing.assignment_versions
	USING (tenant_id = current_setting('app.current_tenant')::uuid)
	WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
	`,
		`
	CREATE OR REPLACE FUNCTION staffing.replay_assignment_versions(p_tenant_id uuid, p_assignment_id uuid)
	RETURNS void
	LANGUAGE plpgsql
	AS $$
	DECLARE
	  v_lock_key text;
	  v_prev_effective date;
	  v_person_uuid uuid;
	  v_assignment_type text;
	  v_position_id uuid;
	  v_status text;
	  v_allocated_fte numeric(9,2);
	  v_base_salary numeric(15,2);
	  v_tmp_text text;
	  v_row RECORD;
	  v_validity daterange;
	BEGIN
	  PERFORM staffing.assert_current_tenant(p_tenant_id);
	
	  IF p_assignment_id IS NULL THEN
	    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'assignment_id is required';
	  END IF;
	
	  v_lock_key := format('staffing:assignment:%s:%s', p_tenant_id, p_assignment_id);
	  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));
	
	  DELETE FROM staffing.assignment_versions
	  WHERE tenant_id = p_tenant_id AND assignment_id = p_assignment_id;
	
	  v_person_uuid := NULL;
	  v_assignment_type := NULL;
	  v_position_id := NULL;
	  v_status := 'active';
	  v_allocated_fte := 1.0;
	  v_base_salary := NULL;
	  v_prev_effective := NULL;
	
	  FOR v_row IN
	    WITH base AS (
	      SELECT
	        e.id AS event_db_id,
	        e.event_type,
	        e.effective_date,
	        e.person_uuid,
	        e.assignment_type,
	        COALESCE(c.replacement_payload, e.payload) AS payload,
	        (r.id IS NOT NULL) AS is_rescinded
	      FROM staffing.assignment_events e
	      LEFT JOIN staffing.assignment_event_corrections c
	        ON c.tenant_id = e.tenant_id
	       AND c.assignment_id = e.assignment_id
	       AND c.target_effective_date = e.effective_date
	      LEFT JOIN staffing.assignment_event_rescinds r
	        ON r.tenant_id = e.tenant_id
	       AND r.assignment_id = e.assignment_id
	       AND r.target_effective_date = e.effective_date
	      WHERE e.tenant_id = p_tenant_id
	        AND e.assignment_id = p_assignment_id
	    ),
	    filtered AS (
	      SELECT *
	      FROM base
	      WHERE NOT is_rescinded
	    ),
	    ordered AS (
	      SELECT
	        event_db_id,
	        event_type,
	        effective_date,
	        person_uuid,
	        assignment_type,
	        payload,
	        lead(effective_date) OVER (ORDER BY effective_date ASC, event_db_id ASC) AS next_effective
	      FROM filtered
	    )
	    SELECT *
	    FROM ordered
	    ORDER BY effective_date ASC, event_db_id ASC
	  LOOP
	    IF v_row.event_type = 'CREATE' THEN
	      IF v_prev_effective IS NOT NULL THEN
	        RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_EVENT', DETAIL = 'CREATE must be the first event';
	      END IF;
	
	      v_person_uuid := v_row.person_uuid;
	      v_assignment_type := v_row.assignment_type;
	
	      v_position_id := NULLIF(v_row.payload->>'position_id', '')::uuid;
	      IF v_position_id IS NULL THEN
	        RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'position_id is required';
	      END IF;
	      v_status := 'active';
	    ELSIF v_row.event_type = 'UPDATE' THEN
	      IF v_prev_effective IS NULL THEN
	        RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_EVENT', DETAIL = 'UPDATE requires prior state';
	      END IF;
	
	      IF v_row.payload ? 'position_id' THEN
	        v_position_id := NULLIF(v_row.payload->>'position_id', '')::uuid;
	        IF v_position_id IS NULL THEN
	          RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'position_id is required';
	        END IF;
	      END IF;
	
	      IF v_row.payload ? 'status' THEN
	        v_status := NULLIF(btrim(v_row.payload->>'status'), '');
	        IF v_status IS NULL OR v_status NOT IN ('active','inactive') THEN
	          RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('invalid status: %s', v_row.payload->>'status');
	        END IF;
	      END IF;
	    ELSE
	      RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('unexpected event_type: %s', v_row.event_type);
	    END IF;
	
	    IF v_row.payload ? 'base_salary' THEN
	      v_tmp_text := NULLIF(btrim(v_row.payload->>'base_salary'), '');
	      IF v_tmp_text IS NULL THEN
	        v_base_salary := NULL;
	      ELSE
	        BEGIN
	          v_base_salary := v_tmp_text::numeric;
	        EXCEPTION
	          WHEN others THEN
	            RAISE EXCEPTION USING MESSAGE = 'STAFFING_ASSIGNMENT_BASE_SALARY_INVALID';
	        END;
	      END IF;
	    END IF;
	
	    IF v_row.payload ? 'allocated_fte' THEN
	      v_tmp_text := NULLIF(btrim(v_row.payload->>'allocated_fte'), '');
	      IF v_tmp_text IS NULL THEN
	        RAISE EXCEPTION USING MESSAGE = 'STAFFING_ASSIGNMENT_ALLOCATED_FTE_INVALID';
	      END IF;
	      BEGIN
	        v_allocated_fte := v_tmp_text::numeric;
	      EXCEPTION
	        WHEN others THEN
	          RAISE EXCEPTION USING MESSAGE = 'STAFFING_ASSIGNMENT_ALLOCATED_FTE_INVALID';
	      END;
	    END IF;
	
	    IF v_row.next_effective IS NULL THEN
	      v_validity := daterange(v_row.effective_date, NULL, '[)');
	    ELSE
	      v_validity := daterange(v_row.effective_date, v_row.next_effective, '[)');
	    END IF;
	
	    INSERT INTO staffing.assignment_versions (
	      tenant_id,
	      assignment_id,
	      person_uuid,
	      position_id,
	      assignment_type,
	      status,
	      allocated_fte,
	      base_salary,
	      validity,
	      last_event_id
	    )
	    VALUES (
	      p_tenant_id,
	      p_assignment_id,
	      v_person_uuid,
	      v_position_id,
	      v_assignment_type,
	      v_status,
	      v_allocated_fte,
	      v_base_salary,
	      v_validity,
	      v_row.event_db_id
	    );
	
	    v_prev_effective := v_row.effective_date;
	  END LOOP;
	END;
	$$;
	`,
		`
CREATE TABLE IF NOT EXISTS staffing.positions (
  tenant_id uuid NOT NULL,
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, id)
);
`,
		`
	CREATE OR REPLACE FUNCTION staffing.submit_assignment_event_correction(
	  p_event_id uuid,
	  p_tenant_id uuid,
	  p_assignment_id uuid,
	  p_target_effective_date date,
	  p_replacement_payload jsonb,
	  p_request_id text,
	  p_initiator_id uuid
	)
	RETURNS bigint
	LANGUAGE plpgsql
	AS $$
	DECLARE
	  v_lock_key text;
	  v_target staffing.assignment_events%ROWTYPE;
	  v_payload jsonb;
	  v_correction_db_id bigint;
	  v_existing_by_event staffing.assignment_event_corrections%ROWTYPE;
	  v_existing_by_target staffing.assignment_event_corrections%ROWTYPE;
	BEGIN
	  PERFORM staffing.assert_current_tenant(p_tenant_id);
	
	  IF p_event_id IS NULL OR p_assignment_id IS NULL OR p_target_effective_date IS NULL OR p_request_id IS NULL OR p_initiator_id IS NULL THEN
	    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT';
	  END IF;
	  IF p_replacement_payload IS NULL OR jsonb_typeof(p_replacement_payload) <> 'object' THEN
	    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT';
	  END IF;
	
	  v_lock_key := format('staffing:assignment:%s:%s', p_tenant_id, p_assignment_id);
	  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));
	
	  SELECT * INTO v_target
	  FROM staffing.assignment_events
	  WHERE tenant_id = p_tenant_id
	    AND assignment_id = p_assignment_id
	    AND effective_date = p_target_effective_date
	  LIMIT 1;
	
	  IF NOT FOUND THEN
	    RAISE EXCEPTION USING MESSAGE = 'STAFFING_ASSIGNMENT_EVENT_NOT_FOUND';
	  END IF;
	
	  IF EXISTS (
	    SELECT 1
	    FROM staffing.assignment_event_rescinds r
	    WHERE r.tenant_id = p_tenant_id
	      AND r.assignment_id = p_assignment_id
	      AND r.target_effective_date = p_target_effective_date
	    LIMIT 1
	  ) THEN
	    RAISE EXCEPTION USING MESSAGE = 'STAFFING_ASSIGNMENT_EVENT_ALREADY_RESCINDED';
	  END IF;
	
	  v_payload := p_replacement_payload;
	
	  INSERT INTO staffing.assignment_event_corrections (
	    event_id,
	    tenant_id,
	    assignment_id,
	    target_effective_date,
	    replacement_payload,
	    request_id,
	    initiator_id
	  )
	  VALUES (
	    p_event_id,
	    p_tenant_id,
	    p_assignment_id,
	    p_target_effective_date,
	    v_payload,
	    p_request_id,
	    p_initiator_id
	  )
	  ON CONFLICT DO NOTHING
	  RETURNING id INTO v_correction_db_id;
	
	  IF v_correction_db_id IS NULL THEN
	    SELECT * INTO v_existing_by_event
	    FROM staffing.assignment_event_corrections
	    WHERE event_id = p_event_id;
	
	    IF FOUND THEN
	      IF v_existing_by_event.replacement_payload <> v_payload THEN
	        RAISE EXCEPTION USING MESSAGE = 'STAFFING_IDEMPOTENCY_REUSED';
	      END IF;
	      v_correction_db_id := v_existing_by_event.id;
	    ELSE
	      SELECT * INTO v_existing_by_target
	      FROM staffing.assignment_event_corrections
	      WHERE tenant_id = p_tenant_id
	        AND assignment_id = p_assignment_id
	        AND target_effective_date = p_target_effective_date
	      LIMIT 1;
	
	      IF FOUND THEN
	        IF v_existing_by_target.replacement_payload = v_payload THEN
	          v_correction_db_id := v_existing_by_target.id;
	        ELSE
	          RAISE EXCEPTION USING MESSAGE = 'STAFFING_ASSIGNMENT_EVENT_ALREADY_CORRECTED';
	        END IF;
	      ELSE
	        RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT';
	      END IF;
	    END IF;
	  END IF;
	
	  PERFORM staffing.replay_assignment_versions(p_tenant_id, p_assignment_id);
	  RETURN v_correction_db_id;
	END;
	$$;
	`,
		`
	CREATE OR REPLACE FUNCTION staffing.submit_assignment_event_rescind(
	  p_event_id uuid,
	  p_tenant_id uuid,
	  p_assignment_id uuid,
	  p_target_effective_date date,
	  p_payload jsonb,
	  p_request_id text,
	  p_initiator_id uuid
	)
	RETURNS bigint
	LANGUAGE plpgsql
	AS $$
	DECLARE
	  v_lock_key text;
	  v_target staffing.assignment_events%ROWTYPE;
	  v_payload jsonb;
	  v_rescind_db_id bigint;
	  v_existing_by_event staffing.assignment_event_rescinds%ROWTYPE;
	  v_existing_by_target staffing.assignment_event_rescinds%ROWTYPE;
	BEGIN
	  PERFORM staffing.assert_current_tenant(p_tenant_id);
	
	  IF p_event_id IS NULL OR p_assignment_id IS NULL OR p_target_effective_date IS NULL OR p_request_id IS NULL OR p_initiator_id IS NULL THEN
	    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT';
	  END IF;
	
	  v_payload := COALESCE(p_payload, '{}'::jsonb);
	  IF jsonb_typeof(v_payload) <> 'object' THEN
	    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT';
	  END IF;
	
	  v_lock_key := format('staffing:assignment:%s:%s', p_tenant_id, p_assignment_id);
	  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));
	
	  SELECT * INTO v_target
	  FROM staffing.assignment_events
	  WHERE tenant_id = p_tenant_id
	    AND assignment_id = p_assignment_id
	    AND effective_date = p_target_effective_date
	  LIMIT 1;
	
	  IF NOT FOUND THEN
	    RAISE EXCEPTION USING MESSAGE = 'STAFFING_ASSIGNMENT_EVENT_NOT_FOUND';
	  END IF;
	  IF v_target.event_type = 'CREATE' THEN
	    RAISE EXCEPTION USING MESSAGE = 'STAFFING_ASSIGNMENT_CREATE_CANNOT_RESCIND';
	  END IF;
	
	  INSERT INTO staffing.assignment_event_rescinds (
	    event_id,
	    tenant_id,
	    assignment_id,
	    target_effective_date,
	    payload,
	    request_id,
	    initiator_id
	  )
	  VALUES (
	    p_event_id,
	    p_tenant_id,
	    p_assignment_id,
	    p_target_effective_date,
	    v_payload,
	    p_request_id,
	    p_initiator_id
	  )
	  ON CONFLICT DO NOTHING
	  RETURNING id INTO v_rescind_db_id;
	
	  IF v_rescind_db_id IS NULL THEN
	    SELECT * INTO v_existing_by_event
	    FROM staffing.assignment_event_rescinds
	    WHERE event_id = p_event_id;
	
	    IF FOUND THEN
	      IF v_existing_by_event.payload <> v_payload THEN
	        RAISE EXCEPTION USING MESSAGE = 'STAFFING_IDEMPOTENCY_REUSED';
	      END IF;
	      v_rescind_db_id := v_existing_by_event.id;
	    ELSE
	      SELECT * INTO v_existing_by_target
	      FROM staffing.assignment_event_rescinds
	      WHERE tenant_id = p_tenant_id
	        AND assignment_id = p_assignment_id
	        AND target_effective_date = p_target_effective_date
	      LIMIT 1;
	
	      IF FOUND THEN
	        v_rescind_db_id := v_existing_by_target.id;
	      ELSE
	        RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT';
	      END IF;
	    END IF;
	  END IF;
	
	  PERFORM staffing.replay_assignment_versions(p_tenant_id, p_assignment_id);
	  RETURN v_rescind_db_id;
	END;
	$$;
	`,
		`
CREATE OR REPLACE FUNCTION staffing.submit_assignment_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_assignment_id uuid,
  p_person_uuid uuid,
  p_assignment_type text,
  p_event_type text,
  p_effective_date date,
  p_payload jsonb,
  p_request_id text,
  p_initiator_id uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_event_db_id bigint;
  v_existing staffing.assignment_events%ROWTYPE;
  v_payload jsonb;
  v_existing_assignment_id uuid;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'event_id is required';
  END IF;
  IF p_assignment_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'assignment_id is required';
  END IF;
  IF p_person_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'person_uuid is required';
  END IF;
  IF p_assignment_type IS NULL OR btrim(p_assignment_type) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'assignment_type is required';
  END IF;
  IF p_assignment_type <> 'primary' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('unsupported assignment_type: %s', p_assignment_type);
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'effective_date is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'initiator_id is required';
  END IF;
  IF p_event_type NOT IN ('CREATE','UPDATE') THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('unsupported event_type: %s', p_event_type);
  END IF;

  v_lock_key := format('staffing:assignment:%s:%s', p_tenant_id, p_assignment_id);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  INSERT INTO staffing.assignments (tenant_id, id, person_uuid, assignment_type)
  VALUES (p_tenant_id, p_assignment_id, p_person_uuid, p_assignment_type)
  ON CONFLICT (tenant_id, person_uuid, assignment_type) DO NOTHING;

  SELECT id INTO v_existing_assignment_id
  FROM staffing.assignments
  WHERE tenant_id = p_tenant_id AND person_uuid = p_person_uuid AND assignment_type = p_assignment_type;

  IF v_existing_assignment_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'assignment identity missing';
  END IF;
  IF v_existing_assignment_id <> p_assignment_id THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_ASSIGNMENT_ID_MISMATCH', DETAIL = format('assignment_id=%s existing_id=%s', p_assignment_id, v_existing_assignment_id);
  END IF;

  v_payload := COALESCE(p_payload, '{}'::jsonb);

  INSERT INTO staffing.assignment_events (
    event_id,
    tenant_id,
    assignment_id,
    person_uuid,
    assignment_type,
    event_type,
    effective_date,
    payload,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    p_assignment_id,
    p_person_uuid,
    p_assignment_type,
    p_event_type,
    p_effective_date,
    v_payload,
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT (event_id) DO NOTHING
  RETURNING id INTO v_event_db_id;

  IF v_event_db_id IS NULL THEN
    SELECT * INTO v_existing
    FROM staffing.assignment_events
    WHERE event_id = p_event_id;

    IF v_existing.tenant_id <> p_tenant_id
      OR v_existing.assignment_id <> p_assignment_id
      OR v_existing.person_uuid <> p_person_uuid
      OR v_existing.assignment_type <> p_assignment_type
      OR v_existing.event_type <> p_event_type
      OR v_existing.effective_date <> p_effective_date
      OR v_existing.payload <> v_payload
      OR v_existing.request_id <> p_request_id
      OR v_existing.initiator_id <> p_initiator_id
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_IDEMPOTENCY_REUSED',
        DETAIL = format('event_id=%s existing_id=%s', p_event_id, v_existing.id);
    END IF;

    RETURN v_existing.id;
  END IF;

  PERFORM staffing.replay_assignment_versions(p_tenant_id, p_assignment_id);

  RETURN v_event_db_id;
END;
$$;
`,
		`GRANT USAGE ON SCHEMA staffing TO ` + runtimeRole + `;`,
		`GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA staffing TO ` + runtimeRole + `;`,
		`GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA staffing TO ` + runtimeRole + `;`,
		`GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA staffing TO ` + runtimeRole + `;`,
	}

	for _, stmt := range ddl {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if _, err := conn.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
