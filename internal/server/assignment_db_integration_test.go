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

	a1, err := store.UpsertPrimaryAssignmentForPerson(ctx, tenantA, effectiveDate, personUUID, position1, "", "")
	if err != nil {
		t.Fatalf("upsert-1: %v", err)
	}
	a2, err := store.UpsertPrimaryAssignmentForPerson(ctx, tenantA, effectiveDate, personUUID, position1, "", "")
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

	_, err = store.UpsertPrimaryAssignmentForPerson(ctx, tenantA, effectiveDate, personUUID, position2, "", "")
	if err == nil {
		t.Fatal("expected error when reusing effective_date with different payload")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Message != "STAFFING_IDEMPOTENCY_REUSED" {
		t.Fatalf("expected STAFFING_IDEMPOTENCY_REUSED, got err=%v", err)
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
		`
CREATE OR REPLACE FUNCTION staffing.replay_assignment_versions(p_tenant_id uuid, p_assignment_id uuid)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);
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
