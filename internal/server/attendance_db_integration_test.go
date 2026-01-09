package server

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestAttendanceDB_RLSIsolationAndIdempotency(t *testing.T) {
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

	if err := ensureAttendanceSchemaForTest(ctx, adminConn); err != nil {
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

	func() {
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()

		var n int
		err = tx.QueryRow(ctx, `SELECT count(*) FROM staffing.time_punch_events;`).Scan(&n)
		if err == nil {
			t.Fatal("expected RLS fail-closed error when app.current_tenant is missing")
		}
	}()

	tenantA := "00000000-0000-0000-0000-0000000000a1"
	tenantB := "00000000-0000-0000-0000-0000000000b1"
	personUUID := "00000000-0000-0000-0000-0000000000c1"
	initiatorID := "00000000-0000-0000-0000-0000000000d1"

	func() {
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()

		if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantA); err != nil {
			t.Fatal(err)
		}

		var id int64
		err = tx.QueryRow(ctx, `
SELECT staffing.submit_time_punch_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::timestamptz,
  $5::text,
  $6::text,
  $7::jsonb,
  $8::jsonb,
  $9::jsonb,
  $10::text,
  $11::uuid
)
`, "00000000-0000-0000-0000-0000000000e1", tenantA, personUUID, time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC), "IN", "MANUAL", []byte(`{}`), []byte(`{}`), []byte(`{}`), "req-1", initiatorID).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}

		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}()

	func() {
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()

		if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantB); err != nil {
			t.Fatal(err)
		}

		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM staffing.time_punch_events WHERE person_uuid = $1::uuid;`, personUUID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatalf("expected tenant isolation; got count=%d", n)
		}
	}()

	func() {
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()

		if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantA); err != nil {
			t.Fatal(err)
		}

		eventID := "00000000-0000-0000-0000-0000000000f1"
		requestID := eventID
		punchTime := time.Date(2026, 1, 2, 1, 0, 0, 0, time.UTC)

		var firstID int64
		if err := tx.QueryRow(ctx, `
SELECT staffing.submit_time_punch_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::timestamptz,
  $5::text,
  $6::text,
  $7::jsonb,
  $8::jsonb,
  $9::jsonb,
  $10::text,
  $11::uuid
)
`, eventID, tenantA, personUUID, punchTime, "IN", "MANUAL", []byte(`{"a":1}`), []byte(`{}`), []byte(`{}`), requestID, initiatorID).Scan(&firstID); err != nil {
			t.Fatal(err)
		}

		var secondID int64
		err = tx.QueryRow(ctx, `
SELECT staffing.submit_time_punch_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::timestamptz,
  $5::text,
  $6::text,
  $7::jsonb,
  $8::jsonb,
  $9::jsonb,
  $10::text,
  $11::uuid
)
`, eventID, tenantA, personUUID, punchTime, "IN", "MANUAL", []byte(`{"a":2}`), []byte(`{}`), []byte(`{}`), requestID, initiatorID).Scan(&secondID)
		if err == nil {
			t.Fatal("expected idempotency reused error")
		}
		if !strings.Contains(err.Error(), "STAFFING_IDEMPOTENCY_REUSED") {
			t.Fatalf("unexpected err=%v", err)
		}
	}()
}

func TestAttendanceDailyResultsDB_StandardShift(t *testing.T) {
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

	if err := ensureAttendanceDailyResultsSchemaForTest(ctx, adminConn); err != nil {
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
	tenantB := "00000000-0000-0000-0000-0000000000b1"
	initiatorID := "00000000-0000-0000-0000-0000000000d1"

	testUUID := func(n int) string {
		return fmt.Sprintf("00000000-0000-0000-0000-%012x", n)
	}

	submitPunch := func(t *testing.T, tenantID string, personUUID string, punchTime time.Time, punchType string, n int) {
		t.Helper()

		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()

		if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
			t.Fatal(err)
		}

		eventID := testUUID(n)
		requestID := eventID
		var id int64
		err = tx.QueryRow(ctx, `
SELECT staffing.submit_time_punch_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::timestamptz,
  $5::text,
  $6::text,
  $7::jsonb,
  $8::jsonb,
  $9::jsonb,
  $10::text,
  $11::uuid
)
`, eventID, tenantID, personUUID, punchTime, punchType, "MANUAL", []byte(`{}`), []byte(`{}`), []byte(`{}`), requestID, initiatorID).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}

	readResult := func(t *testing.T, tenantID string, personUUID string, workDate string) (status string, flags []string, worked int, late int, early int, firstIn *time.Time, lastOut *time.Time) {
		t.Helper()

		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()

		if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
			t.Fatal(err)
		}

		err = tx.QueryRow(ctx, `
SELECT status, flags, worked_minutes, late_minutes, early_leave_minutes, first_in_time, last_out_time
FROM staffing.daily_attendance_results
WHERE tenant_id = $1::uuid AND person_uuid = $2::uuid AND work_date = $3::date
`, tenantID, personUUID, workDate).Scan(&status, &flags, &worked, &late, &early, &firstIn, &lastOut)
		if err != nil {
			t.Fatal(err)
		}
		return status, flags, worked, late, early, firstIn, lastOut
	}

	seedTimeProfile := func(t *testing.T, tenantID string) {
		t.Helper()

		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()

		if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
			t.Fatal(err)
		}

		eventID := testUUID(9001)
		requestID := eventID
		payload := []byte(`{"shift_start_local":"09:00","shift_end_local":"18:00","late_tolerance_minutes":5,"early_leave_tolerance_minutes":5,"overtime_min_minutes":0,"overtime_rounding_mode":"NONE","overtime_rounding_unit_minutes":0}`)

		var eventDBID int64
		if err := tx.QueryRow(ctx, `
INSERT INTO staffing.time_profile_events (
  event_id,
  tenant_id,
  event_type,
  effective_date,
  payload,
  request_id,
  initiator_id
)
VALUES (
  $1::uuid,
  $2::uuid,
  'CREATE',
  $3::date,
  $4::jsonb,
  $5::text,
  $6::uuid
)
RETURNING id
`, eventID, tenantID, "2025-01-01", payload, requestID, initiatorID).Scan(&eventDBID); err != nil {
			t.Fatal(err)
		}

		if _, err := tx.Exec(ctx, `
INSERT INTO staffing.time_profile_versions (
  tenant_id,
  name,
  lifecycle_status,
  shift_start_local,
  shift_end_local,
  late_tolerance_minutes,
  early_leave_tolerance_minutes,
  overtime_min_minutes,
  overtime_rounding_mode,
  overtime_rounding_unit_minutes,
  validity,
  last_event_id
)
VALUES (
  $1::uuid,
  NULL,
  'active',
  '09:00'::time,
  '18:00'::time,
  5,
  5,
  0,
  'NONE',
  0,
  daterange($2::date, NULL, '[)'),
  $3::bigint
)
`, tenantID, "2025-01-01", eventDBID); err != nil {
			t.Fatal(err)
		}

		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}

	seedTimeProfile(t, tenantA)

	t.Run("fail-closed (no tenant)", func(t *testing.T) {
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()

		var n int
		err = tx.QueryRow(ctx, `SELECT count(*) FROM staffing.daily_attendance_results;`).Scan(&n)
		if err == nil {
			t.Fatal("expected RLS fail-closed error when app.current_tenant is missing")
		}
	})

	t.Run("present (09:00-18:00)", func(t *testing.T) {
		person := testUUID(1)
		submitPunch(t, tenantA, person, time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC), "IN", 101)
		submitPunch(t, tenantA, person, time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC), "OUT", 102)

		status, flags, worked, late, early, firstIn, lastOut := readResult(t, tenantA, person, "2026-01-01")
		if status != "PRESENT" {
			t.Fatalf("status=%s", status)
		}
		if len(flags) != 0 {
			t.Fatalf("flags=%v", flags)
		}
		if worked != 540 || late != 0 || early != 0 {
			t.Fatalf("worked=%d late=%d early=%d", worked, late, early)
		}
		if firstIn == nil || lastOut == nil {
			t.Fatalf("firstIn=%v lastOut=%v", firstIn, lastOut)
		}
	})

	t.Run("tenant isolation (no cross-tenant leakage)", func(t *testing.T) {
		person := testUUID(2)
		submitPunch(t, tenantA, person, time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC), "IN", 111)

		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()

		if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantB); err != nil {
			t.Fatal(err)
		}

		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM staffing.daily_attendance_results WHERE person_uuid = $1::uuid;`, person).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatalf("expected tenant isolation; got count=%d", n)
		}
	})

	t.Run("missing out => EXCEPTION + MISSING_OUT", func(t *testing.T) {
		person := testUUID(3)
		submitPunch(t, tenantA, person, time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC), "IN", 121)

		status, flags, worked, _, _, firstIn, lastOut := readResult(t, tenantA, person, "2026-01-01")
		if status != "EXCEPTION" {
			t.Fatalf("status=%s flags=%v", status, flags)
		}
		if worked != 0 {
			t.Fatalf("worked=%d", worked)
		}
		if firstIn == nil || lastOut != nil {
			t.Fatalf("firstIn=%v lastOut=%v", firstIn, lastOut)
		}
		found := false
		for _, f := range flags {
			if f == "MISSING_OUT" {
				found = true
			}
		}
		if !found {
			t.Fatalf("flags=%v", flags)
		}
	})

	t.Run("tolerance boundary (09:05 not late, 09:06 late=1)", func(t *testing.T) {
		person1 := testUUID(4)
		submitPunch(t, tenantA, person1, time.Date(2026, 1, 1, 1, 5, 0, 0, time.UTC), "IN", 131)
		submitPunch(t, tenantA, person1, time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC), "OUT", 132)

		status, flags, _, late, _, _, _ := readResult(t, tenantA, person1, "2026-01-01")
		if status != "PRESENT" || late != 0 || len(flags) != 0 {
			t.Fatalf("status=%s late=%d flags=%v", status, late, flags)
		}

		person2 := testUUID(5)
		submitPunch(t, tenantA, person2, time.Date(2026, 1, 1, 1, 6, 0, 0, time.UTC), "IN", 141)
		submitPunch(t, tenantA, person2, time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC), "OUT", 142)

		status2, flags2, _, late2, _, _, _ := readResult(t, tenantA, person2, "2026-01-01")
		if status2 != "EXCEPTION" || late2 != 1 {
			t.Fatalf("status=%s late=%d flags=%v", status2, late2, flags2)
		}
	})

	t.Run("cross-day OUT (23:00->02:00 counts for D last_out)", func(t *testing.T) {
		person := testUUID(6)
		in := time.Date(2026, 1, 1, 15, 0, 0, 0, time.UTC)  // 23:00 +08
		out := time.Date(2026, 1, 1, 18, 0, 0, 0, time.UTC) // 02:00 +08 (next day)
		submitPunch(t, tenantA, person, in, "IN", 151)
		submitPunch(t, tenantA, person, out, "OUT", 152)

		_, _, _, _, _, firstIn, lastOut := readResult(t, tenantA, person, "2026-01-01")
		if firstIn == nil || lastOut == nil {
			t.Fatalf("firstIn=%v lastOut=%v", firstIn, lastOut)
		}
		if !firstIn.UTC().Equal(in) || !lastOut.UTC().Equal(out) {
			t.Fatalf("firstIn=%s lastOut=%s", firstIn.UTC().Format(time.RFC3339), lastOut.UTC().Format(time.RFC3339))
		}
	})
}

func connectTestPostgres(ctx context.Context, t *testing.T) (*pgx.Conn, string, bool) {
	t.Helper()

	if v := strings.TrimSpace(os.Getenv("DATABASE_URL")); v != "" {
		conn, err := pgx.Connect(ctx, v)
		if err != nil {
			t.Skipf("postgres unavailable: %v", err)
			return nil, "", false
		}
		return conn, v, true
	}

	candidates := []string{
		"postgres://app:app@localhost:5432/bugs_and_blossoms?sslmode=disable",
		"postgres://app:app@localhost:5438/bugs_and_blossoms?sslmode=disable",
	}
	for _, dsn := range candidates {
		conn, err := pgx.Connect(ctx, dsn)
		if err == nil {
			return conn, dsn, true
		}
	}
	t.Skip("postgres unavailable (tried localhost:5432 and localhost:5438); skipping integration test")
	return nil, "", false
}

func ensureAttendanceSchemaForTest(ctx context.Context, conn *pgx.Conn) error {
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
END;
$$;
`,
		`
CREATE TABLE IF NOT EXISTS staffing.time_punch_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  punch_time timestamptz NOT NULL,
  punch_type text NOT NULL,
  source_provider text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  source_raw_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  device_info jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT time_punch_events_punch_type_check CHECK (punch_type IN ('IN','OUT')),
  CONSTRAINT time_punch_events_source_provider_check CHECK (source_provider IN ('MANUAL','IMPORT')),
  CONSTRAINT time_punch_events_payload_is_object_check CHECK (jsonb_typeof(payload) = 'object'),
  CONSTRAINT time_punch_events_source_raw_is_object_check CHECK (jsonb_typeof(source_raw_payload) = 'object'),
  CONSTRAINT time_punch_events_device_info_is_object_check CHECK (jsonb_typeof(device_info) = 'object'),
  CONSTRAINT time_punch_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT time_punch_events_request_id_unique UNIQUE (tenant_id, request_id)
);
`,
		`
CREATE INDEX IF NOT EXISTS time_punch_events_lookup_idx
  ON staffing.time_punch_events (tenant_id, person_uuid, punch_time DESC, id DESC);
`,
		`ALTER TABLE staffing.time_punch_events ENABLE ROW LEVEL SECURITY;`,
		`ALTER TABLE staffing.time_punch_events FORCE ROW LEVEL SECURITY;`,
		`DROP POLICY IF EXISTS tenant_isolation ON staffing.time_punch_events;`,
		`
CREATE POLICY tenant_isolation ON staffing.time_punch_events
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
`,
		`
CREATE OR REPLACE FUNCTION staffing.submit_time_punch_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_person_uuid uuid,
  p_punch_time timestamptz,
  p_punch_type text,
  p_source_provider text,
  p_payload jsonb,
  p_source_raw_payload jsonb,
  p_device_info jsonb,
  p_request_id text,
  p_initiator_id uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_event_db_id bigint;
  v_existing staffing.time_punch_events%ROWTYPE;
  v_payload jsonb;
  v_source_raw jsonb;
  v_device jsonb;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'event_id is required';
  END IF;
  IF p_person_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'person_uuid is required';
  END IF;
  IF p_punch_time IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'punch_time is required';
  END IF;
  IF p_punch_type NOT IN ('IN','OUT') THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('unsupported punch_type: %s', p_punch_type);
  END IF;
  IF p_source_provider NOT IN ('MANUAL','IMPORT') THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('unsupported source_provider: %s', p_source_provider);
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'initiator_id is required';
  END IF;

  v_payload := COALESCE(p_payload, '{}'::jsonb);
  v_source_raw := COALESCE(p_source_raw_payload, '{}'::jsonb);
  v_device := COALESCE(p_device_info, '{}'::jsonb);
  IF jsonb_typeof(v_payload) <> 'object' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'payload must be an object';
  END IF;
  IF jsonb_typeof(v_source_raw) <> 'object' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'source_raw_payload must be an object';
  END IF;
  IF jsonb_typeof(v_device) <> 'object' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'device_info must be an object';
  END IF;

  INSERT INTO staffing.time_punch_events (
    event_id,
    tenant_id,
    person_uuid,
    punch_time,
    punch_type,
    source_provider,
    payload,
    source_raw_payload,
    device_info,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    p_person_uuid,
    p_punch_time,
    p_punch_type,
    p_source_provider,
    v_payload,
    v_source_raw,
    v_device,
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT (event_id) DO NOTHING
  RETURNING id INTO v_event_db_id;

  IF v_event_db_id IS NULL THEN
    SELECT * INTO v_existing
    FROM staffing.time_punch_events
    WHERE event_id = p_event_id;

    IF v_existing.tenant_id <> p_tenant_id
      OR v_existing.person_uuid <> p_person_uuid
      OR v_existing.punch_time <> p_punch_time
      OR v_existing.punch_type <> p_punch_type
      OR v_existing.source_provider <> p_source_provider
      OR v_existing.payload <> v_payload
      OR v_existing.source_raw_payload <> v_source_raw
      OR v_existing.device_info <> v_device
      OR v_existing.request_id <> p_request_id
      OR v_existing.initiator_id <> p_initiator_id
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_IDEMPOTENCY_REUSED',
        DETAIL = format('event_id=%s existing_id=%s', p_event_id, v_existing.id);
    END IF;

    RETURN v_existing.id;
  END IF;

  RETURN v_event_db_id;
END;
$$;
`,
		`GRANT USAGE ON SCHEMA staffing TO ` + runtimeRole + `;`,
		`GRANT SELECT, INSERT ON staffing.time_punch_events TO ` + runtimeRole + `;`,
		`GRANT USAGE, SELECT ON SEQUENCE staffing.time_punch_events_id_seq TO ` + runtimeRole + `;`,
		`GRANT EXECUTE ON FUNCTION staffing.assert_current_tenant(uuid) TO ` + runtimeRole + `;`,
		`GRANT EXECUTE ON FUNCTION staffing.submit_time_punch_event(uuid, uuid, uuid, timestamptz, text, text, jsonb, jsonb, jsonb, text, uuid) TO ` + runtimeRole + `;`,
		`TRUNCATE staffing.time_punch_events;`,
	}

	for _, s := range ddl {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, err := conn.Exec(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

func ensureAttendanceDailyResultsSchemaForTest(ctx context.Context, conn *pgx.Conn) error {
	const runtimeRole = "bb_test_runtime"

	if err := ensureAttendanceSchemaForTest(ctx, conn); err != nil {
		return err
	}

	ddl := []string{
		`CREATE EXTENSION IF NOT EXISTS btree_gist;`,
		`
CREATE TABLE IF NOT EXISTS staffing.daily_attendance_results (
  tenant_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  work_date date NOT NULL,

  ruleset_version text NOT NULL,
  day_type text NULL,
  status text NOT NULL,
  flags text[] NOT NULL DEFAULT '{}'::text[],

  first_in_time timestamptz NULL,
  last_out_time timestamptz NULL,
  scheduled_minutes int NOT NULL DEFAULT 0,
  worked_minutes int NOT NULL DEFAULT 0,
  overtime_minutes_150 int NOT NULL DEFAULT 0,
  overtime_minutes_200 int NOT NULL DEFAULT 0,
  overtime_minutes_300 int NOT NULL DEFAULT 0,
  late_minutes int NOT NULL DEFAULT 0,
  early_leave_minutes int NOT NULL DEFAULT 0,

  input_punch_count int NOT NULL DEFAULT 0,
  input_max_punch_event_db_id bigint NULL,
  input_max_punch_time timestamptz NULL,

  time_profile_last_event_id bigint NULL,
  holiday_day_last_event_id bigint NULL,

  computed_at timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),

  PRIMARY KEY (tenant_id, person_uuid, work_date),

  CONSTRAINT daily_attendance_results_status_check
    CHECK (status IN ('PRESENT','ABSENT','EXCEPTION','OFF')),
  CONSTRAINT daily_attendance_results_day_type_check
    CHECK (day_type IS NULL OR day_type IN ('WORKDAY','RESTDAY','LEGAL_HOLIDAY')),
  CONSTRAINT daily_attendance_results_minutes_nonneg_check
    CHECK (scheduled_minutes >= 0 AND worked_minutes >= 0 AND late_minutes >= 0 AND early_leave_minutes >= 0),
  CONSTRAINT daily_attendance_results_overtime_nonneg_check
    CHECK (overtime_minutes_150 >= 0 AND overtime_minutes_200 >= 0 AND overtime_minutes_300 >= 0),
  CONSTRAINT daily_attendance_results_flags_allowlist_check
    CHECK (flags <@ ARRAY['ABSENT','MISSING_IN','MISSING_OUT','LATE','EARLY_LEAVE']::text[])
);
`,
		`
CREATE INDEX IF NOT EXISTS daily_attendance_results_lookup_idx
  ON staffing.daily_attendance_results (tenant_id, person_uuid, work_date DESC);
`,
		`ALTER TABLE staffing.daily_attendance_results ENABLE ROW LEVEL SECURITY;`,
		`ALTER TABLE staffing.daily_attendance_results FORCE ROW LEVEL SECURITY;`,
		`DROP POLICY IF EXISTS tenant_isolation ON staffing.daily_attendance_results;`,
		`
CREATE POLICY tenant_isolation ON staffing.daily_attendance_results
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
`,
		`
CREATE TABLE IF NOT EXISTS staffing.time_profile_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  event_type text NOT NULL,
  effective_date date NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT time_profile_events_event_type_check CHECK (event_type IN ('CREATE','UPDATE')),
  CONSTRAINT time_profile_events_payload_is_object_check CHECK (jsonb_typeof(payload) = 'object'),
  CONSTRAINT time_profile_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT time_profile_events_one_per_day_unique UNIQUE (tenant_id, effective_date),
  CONSTRAINT time_profile_events_request_id_unique UNIQUE (tenant_id, request_id)
);
`,
		`
CREATE INDEX IF NOT EXISTS time_profile_events_lookup_idx
  ON staffing.time_profile_events (tenant_id, effective_date, id);
`,
		`ALTER TABLE staffing.time_profile_events ENABLE ROW LEVEL SECURITY;`,
		`ALTER TABLE staffing.time_profile_events FORCE ROW LEVEL SECURITY;`,
		`DROP POLICY IF EXISTS tenant_isolation ON staffing.time_profile_events;`,
		`
CREATE POLICY tenant_isolation ON staffing.time_profile_events
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
`,
		`
CREATE TABLE IF NOT EXISTS staffing.time_profile_versions (
  id bigserial PRIMARY KEY,
  tenant_id uuid NOT NULL,
  name text NULL,
  lifecycle_status text NOT NULL DEFAULT 'active',

  shift_start_local time NOT NULL,
  shift_end_local time NOT NULL,
  late_tolerance_minutes int NOT NULL DEFAULT 0,
  early_leave_tolerance_minutes int NOT NULL DEFAULT 0,

  overtime_min_minutes int NOT NULL DEFAULT 0,
  overtime_rounding_mode text NOT NULL DEFAULT 'NONE',
  overtime_rounding_unit_minutes int NOT NULL DEFAULT 0,

  validity daterange NOT NULL,
  last_event_id bigint NOT NULL REFERENCES staffing.time_profile_events(id),

  CONSTRAINT time_profile_versions_validity_check CHECK (NOT isempty(validity)),
  CONSTRAINT time_profile_versions_validity_bounds_check CHECK (lower_inc(validity) AND NOT upper_inc(validity)),
  CONSTRAINT time_profile_versions_lifecycle_status_check CHECK (lifecycle_status IN ('active','disabled')),
  CONSTRAINT time_profile_versions_shift_time_order_check CHECK (shift_end_local > shift_start_local),
  CONSTRAINT time_profile_versions_tolerance_minutes_check CHECK (late_tolerance_minutes >= 0 AND early_leave_tolerance_minutes >= 0),
  CONSTRAINT time_profile_versions_overtime_min_check CHECK (overtime_min_minutes >= 0),
  CONSTRAINT time_profile_versions_overtime_rounding_mode_check CHECK (overtime_rounding_mode IN ('NONE','FLOOR','CEIL','NEAREST')),
  CONSTRAINT time_profile_versions_overtime_rounding_unit_check CHECK (overtime_rounding_unit_minutes >= 0),
  CONSTRAINT time_profile_versions_no_overlap
    EXCLUDE USING gist (
      tenant_id gist_uuid_ops WITH =,
      validity WITH &&
    )
);
`,
		`
CREATE INDEX IF NOT EXISTS time_profile_versions_lookup_idx
  ON staffing.time_profile_versions (tenant_id, lower(validity));
`,
		`ALTER TABLE staffing.time_profile_versions ENABLE ROW LEVEL SECURITY;`,
		`ALTER TABLE staffing.time_profile_versions FORCE ROW LEVEL SECURITY;`,
		`DROP POLICY IF EXISTS tenant_isolation ON staffing.time_profile_versions;`,
		`
CREATE POLICY tenant_isolation ON staffing.time_profile_versions
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
`,
		`
CREATE TABLE IF NOT EXISTS staffing.holiday_day_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  day_date date NOT NULL,
  event_type text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT holiday_day_events_event_type_check CHECK (event_type IN ('SET','CLEAR')),
  CONSTRAINT holiday_day_events_payload_is_object_check CHECK (jsonb_typeof(payload) = 'object'),
  CONSTRAINT holiday_day_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT holiday_day_events_request_id_unique UNIQUE (tenant_id, request_id)
);
`,
		`
CREATE INDEX IF NOT EXISTS holiday_day_events_lookup_idx
  ON staffing.holiday_day_events (tenant_id, day_date, id);
`,
		`ALTER TABLE staffing.holiday_day_events ENABLE ROW LEVEL SECURITY;`,
		`ALTER TABLE staffing.holiday_day_events FORCE ROW LEVEL SECURITY;`,
		`DROP POLICY IF EXISTS tenant_isolation ON staffing.holiday_day_events;`,
		`
CREATE POLICY tenant_isolation ON staffing.holiday_day_events
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
`,
		`
CREATE TABLE IF NOT EXISTS staffing.holiday_days (
  tenant_id uuid NOT NULL,
  day_date date NOT NULL,
  day_type text NOT NULL,
  holiday_code text NULL,
  note text NULL,
  last_event_id bigint NOT NULL REFERENCES staffing.holiday_day_events(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, day_date),
  CONSTRAINT holiday_days_day_type_check CHECK (day_type IN ('WORKDAY','RESTDAY','LEGAL_HOLIDAY'))
);
`,
		`
CREATE INDEX IF NOT EXISTS holiday_days_lookup_idx
  ON staffing.holiday_days (tenant_id, day_date DESC);
`,
		`ALTER TABLE staffing.holiday_days ENABLE ROW LEVEL SECURITY;`,
		`ALTER TABLE staffing.holiday_days FORCE ROW LEVEL SECURITY;`,
		`DROP POLICY IF EXISTS tenant_isolation ON staffing.holiday_days;`,
		`
CREATE POLICY tenant_isolation ON staffing.holiday_days
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
`,
		`
CREATE OR REPLACE FUNCTION staffing.recompute_daily_attendance_result(
  p_tenant_id uuid,
  p_person_uuid uuid,
  p_work_date date
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_tz text := 'Asia/Shanghai';
  v_ruleset_version text := 'TIME_PROFILE_V1';

  v_shift_start_local time := NULL;
  v_shift_end_local time := NULL;
  v_late_tolerance_min int := 0;
  v_early_tolerance_min int := 0;

  v_overtime_min_minutes int := 0;
  v_overtime_rounding_mode text := 'NONE';
  v_overtime_rounding_unit_minutes int := 0;

  v_window_before interval := interval '6 hours';
  v_window_after interval := interval '12 hours';

  v_shift_start timestamptz;
  v_shift_end timestamptz;
  v_window_start timestamptz;
  v_window_end timestamptz;

  v_punch_count int := 0;
  v_input_max_id bigint := NULL;
  v_input_max_punch_time timestamptz := NULL;

  v_expect text := 'IN';
  v_open_in_time timestamptz := NULL;

  v_first_in_time timestamptz := NULL;
  v_last_out_time timestamptz := NULL;

  v_day_type text := NULL;
  v_holiday_day_last_event_id bigint := NULL;

  v_scheduled_minutes int := 0;
  v_worked_minutes int := 0;
  v_overtime_minutes_150 int := 0;
  v_overtime_minutes_200 int := 0;
  v_overtime_minutes_300 int := 0;
  v_late_minutes int := 0;
  v_early_leave_minutes int := 0;

  v_time_profile_last_event_id bigint := NULL;

  v_status text := 'ABSENT';
  v_flags text[] := '{}'::text[];

  r record;
  v_delta_min int;
  v_raw_ot int := 0;
  v_rounded_ot int := 0;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_person_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'person_uuid is required';
  END IF;
  IF p_work_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'work_date is required';
  END IF;

  PERFORM pg_advisory_xact_lock(
    hashtext(p_tenant_id::text),
    hashtext(p_person_uuid::text || ':' || p_work_date::text)
  );

  SELECT
    shift_start_local,
    shift_end_local,
    late_tolerance_minutes,
    early_leave_tolerance_minutes,
    overtime_min_minutes,
    overtime_rounding_mode,
    overtime_rounding_unit_minutes,
    last_event_id
  INTO
    v_shift_start_local,
    v_shift_end_local,
    v_late_tolerance_min,
    v_early_tolerance_min,
    v_overtime_min_minutes,
    v_overtime_rounding_mode,
    v_overtime_rounding_unit_minutes,
    v_time_profile_last_event_id
  FROM staffing.time_profile_versions
  WHERE tenant_id = p_tenant_id
    AND lifecycle_status = 'active'
    AND validity @> p_work_date
  LIMIT 1;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_TIME_PROFILE_NOT_CONFIGURED_AS_OF',
      DETAIL = format('tenant_id=%s as_of=%s', p_tenant_id, p_work_date);
  END IF;

  v_scheduled_minutes := floor(extract(epoch FROM (v_shift_end_local - v_shift_start_local)) / 60.0)::int;
  IF v_scheduled_minutes < 0 THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = 'scheduled_minutes must be non-negative';
  END IF;

  SELECT day_type, last_event_id
  INTO v_day_type, v_holiday_day_last_event_id
  FROM staffing.holiday_days
  WHERE tenant_id = p_tenant_id
    AND day_date = p_work_date;

  IF NOT FOUND THEN
    IF extract(isodow FROM p_work_date) IN (6, 7) THEN
      v_day_type := 'RESTDAY';
    ELSE
      v_day_type := 'WORKDAY';
    END IF;
    v_holiday_day_last_event_id := NULL;
  END IF;

  v_shift_start := (p_work_date + v_shift_start_local) AT TIME ZONE v_tz;
  v_shift_end := (p_work_date + v_shift_end_local) AT TIME ZONE v_tz;
  v_window_start := v_shift_start - v_window_before;
  v_window_end := v_shift_end + v_window_after;

  FOR r IN
    SELECT id, punch_time, punch_type
    FROM staffing.time_punch_events
    WHERE tenant_id = p_tenant_id
      AND person_uuid = p_person_uuid
      AND punch_time >= v_window_start
      AND punch_time < v_window_end
    ORDER BY punch_time ASC, id ASC
  LOOP
    v_punch_count := v_punch_count + 1;
    v_input_max_id := COALESCE(v_input_max_id, r.id);
    v_input_max_id := GREATEST(v_input_max_id, r.id);
    v_input_max_punch_time := COALESCE(v_input_max_punch_time, r.punch_time);
    v_input_max_punch_time := GREATEST(v_input_max_punch_time, r.punch_time);

    IF r.punch_type = 'IN' THEN
      IF v_expect = 'IN' THEN
        v_open_in_time := r.punch_time;
        v_expect := 'OUT';
        IF v_first_in_time IS NULL THEN
          v_first_in_time := r.punch_time;
        END IF;
      ELSE
        v_flags := array_append(v_flags, 'MISSING_OUT');
        v_open_in_time := r.punch_time;
        v_expect := 'OUT';
        IF v_first_in_time IS NULL THEN
          v_first_in_time := r.punch_time;
        END IF;
      END IF;
    ELSE
      IF v_expect = 'OUT' AND v_open_in_time IS NOT NULL THEN
        v_delta_min := floor(extract(epoch FROM (r.punch_time - v_open_in_time)) / 60.0)::int;
        IF v_delta_min > 0 THEN
          v_worked_minutes := v_worked_minutes + v_delta_min;
        END IF;
        v_last_out_time := r.punch_time;
        v_open_in_time := NULL;
        v_expect := 'IN';
      ELSE
        v_flags := array_append(v_flags, 'MISSING_IN');
      END IF;
    END IF;
  END LOOP;

  IF v_punch_count = 0 THEN
    IF v_day_type = 'WORKDAY' THEN
      v_status := 'ABSENT';
      v_flags := array_append(v_flags, 'ABSENT');
    ELSE
      v_status := 'OFF';
      v_flags := '{}'::text[];
    END IF;
  ELSE
    IF v_first_in_time IS NULL THEN
      v_flags := array_append(v_flags, 'MISSING_IN');
    END IF;
    IF v_expect = 'OUT' THEN
      v_flags := array_append(v_flags, 'MISSING_OUT');
    END IF;

    IF v_first_in_time IS NOT NULL THEN
      v_delta_min := floor(extract(epoch FROM (v_first_in_time - v_shift_start)) / 60.0)::int;
      IF v_delta_min > v_late_tolerance_min THEN
        v_late_minutes := v_delta_min - v_late_tolerance_min;
        v_flags := array_append(v_flags, 'LATE');
      END IF;
    END IF;

    IF v_last_out_time IS NOT NULL THEN
      v_delta_min := floor(extract(epoch FROM (v_shift_end - v_last_out_time)) / 60.0)::int;
      IF v_delta_min > v_early_tolerance_min THEN
        v_early_leave_minutes := v_delta_min - v_early_tolerance_min;
        v_flags := array_append(v_flags, 'EARLY_LEAVE');
      END IF;
    END IF;

    IF array_length(v_flags, 1) IS NULL THEN
      v_status := 'PRESENT';
    ELSE
      SELECT COALESCE(array_agg(DISTINCT f ORDER BY f), '{}'::text[]) INTO v_flags
      FROM unnest(v_flags) AS f;
      v_status := 'EXCEPTION';
    END IF;
  END IF;

  IF v_day_type = 'WORKDAY' THEN
    v_raw_ot := GREATEST(0, v_worked_minutes - v_scheduled_minutes);
  ELSE
    v_raw_ot := v_worked_minutes;
  END IF;

  IF v_raw_ot < v_overtime_min_minutes THEN
    v_raw_ot := 0;
  END IF;

  v_rounded_ot := v_raw_ot;
  IF v_rounded_ot > 0 AND v_overtime_rounding_unit_minutes > 0 AND v_overtime_rounding_mode <> 'NONE' THEN
    IF v_overtime_rounding_mode = 'FLOOR' THEN
      v_rounded_ot := floor(v_rounded_ot::numeric / v_overtime_rounding_unit_minutes::numeric) * v_overtime_rounding_unit_minutes;
    ELSIF v_overtime_rounding_mode = 'CEIL' THEN
      v_rounded_ot := ceiling(v_rounded_ot::numeric / v_overtime_rounding_unit_minutes::numeric) * v_overtime_rounding_unit_minutes;
    ELSIF v_overtime_rounding_mode = 'NEAREST' THEN
      v_rounded_ot := round(v_rounded_ot::numeric / v_overtime_rounding_unit_minutes::numeric) * v_overtime_rounding_unit_minutes;
    END IF;
  END IF;

  v_overtime_minutes_150 := 0;
  v_overtime_minutes_200 := 0;
  v_overtime_minutes_300 := 0;
  IF v_rounded_ot > 0 THEN
    IF v_day_type = 'WORKDAY' THEN
      v_overtime_minutes_150 := v_rounded_ot;
    ELSIF v_day_type = 'RESTDAY' THEN
      v_overtime_minutes_200 := v_rounded_ot;
    ELSIF v_day_type = 'LEGAL_HOLIDAY' THEN
      v_overtime_minutes_300 := v_rounded_ot;
    END IF;
  END IF;

  INSERT INTO staffing.daily_attendance_results (
    tenant_id,
    person_uuid,
    work_date,
    ruleset_version,
    day_type,
    status,
    flags,
    first_in_time,
    last_out_time,
    scheduled_minutes,
    worked_minutes,
    overtime_minutes_150,
    overtime_minutes_200,
    overtime_minutes_300,
    late_minutes,
    early_leave_minutes,
    input_punch_count,
    input_max_punch_event_db_id,
    input_max_punch_time,
    time_profile_last_event_id,
    holiday_day_last_event_id,
    computed_at,
    created_at,
    updated_at
  )
  VALUES (
    p_tenant_id,
    p_person_uuid,
    p_work_date,
    v_ruleset_version,
    v_day_type,
    v_status,
    v_flags,
    v_first_in_time,
    v_last_out_time,
    v_scheduled_minutes,
    v_worked_minutes,
    v_overtime_minutes_150,
    v_overtime_minutes_200,
    v_overtime_minutes_300,
    v_late_minutes,
    v_early_leave_minutes,
    v_punch_count,
    v_input_max_id,
    v_input_max_punch_time,
    v_time_profile_last_event_id,
    v_holiday_day_last_event_id,
    now(),
    now(),
    now()
  )
  ON CONFLICT (tenant_id, person_uuid, work_date)
  DO UPDATE SET
    ruleset_version = EXCLUDED.ruleset_version,
    day_type = EXCLUDED.day_type,
    status = EXCLUDED.status,
    flags = EXCLUDED.flags,
    first_in_time = EXCLUDED.first_in_time,
    last_out_time = EXCLUDED.last_out_time,
    scheduled_minutes = EXCLUDED.scheduled_minutes,
    worked_minutes = EXCLUDED.worked_minutes,
    overtime_minutes_150 = EXCLUDED.overtime_minutes_150,
    overtime_minutes_200 = EXCLUDED.overtime_minutes_200,
    overtime_minutes_300 = EXCLUDED.overtime_minutes_300,
    late_minutes = EXCLUDED.late_minutes,
    early_leave_minutes = EXCLUDED.early_leave_minutes,
    input_punch_count = EXCLUDED.input_punch_count,
    input_max_punch_event_db_id = EXCLUDED.input_max_punch_event_db_id,
    input_max_punch_time = EXCLUDED.input_max_punch_time,
    time_profile_last_event_id = EXCLUDED.time_profile_last_event_id,
    holiday_day_last_event_id = EXCLUDED.holiday_day_last_event_id,
    computed_at = EXCLUDED.computed_at,
    updated_at = EXCLUDED.updated_at;
END;
$$;
`,
		`
CREATE OR REPLACE FUNCTION staffing.recompute_daily_attendance_results_for_punch(
  p_tenant_id uuid,
  p_person_uuid uuid,
  p_punch_time timestamptz
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_tz text := 'Asia/Shanghai';
  v_local_date date;
  v_d1 date;
  v_d2 date;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_person_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'person_uuid is required';
  END IF;
  IF p_punch_time IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'punch_time is required';
  END IF;

  v_local_date := (p_punch_time AT TIME ZONE v_tz)::date;
  v_d1 := v_local_date - 1;
  v_d2 := v_local_date;

  PERFORM staffing.recompute_daily_attendance_result(p_tenant_id, p_person_uuid, v_d1);
  PERFORM staffing.recompute_daily_attendance_result(p_tenant_id, p_person_uuid, v_d2);
END;
$$;
`,
		`
CREATE OR REPLACE FUNCTION staffing.submit_time_punch_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_person_uuid uuid,
  p_punch_time timestamptz,
  p_punch_type text,
  p_source_provider text,
  p_payload jsonb,
  p_source_raw_payload jsonb,
  p_device_info jsonb,
  p_request_id text,
  p_initiator_id uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_event_db_id bigint;
  v_existing staffing.time_punch_events%ROWTYPE;
  v_payload jsonb;
  v_source_raw jsonb;
  v_device jsonb;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'event_id is required';
  END IF;
  IF p_person_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'person_uuid is required';
  END IF;
  IF p_punch_time IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'punch_time is required';
  END IF;
  IF p_punch_type NOT IN ('IN','OUT') THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('unsupported punch_type: %s', p_punch_type);
  END IF;
  IF p_source_provider NOT IN ('MANUAL','IMPORT') THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('unsupported source_provider: %s', p_source_provider);
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'initiator_id is required';
  END IF;

  v_payload := COALESCE(p_payload, '{}'::jsonb);
  v_source_raw := COALESCE(p_source_raw_payload, '{}'::jsonb);
  v_device := COALESCE(p_device_info, '{}'::jsonb);
  IF jsonb_typeof(v_payload) <> 'object' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'payload must be an object';
  END IF;
  IF jsonb_typeof(v_source_raw) <> 'object' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'source_raw_payload must be an object';
  END IF;
  IF jsonb_typeof(v_device) <> 'object' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'device_info must be an object';
  END IF;

  INSERT INTO staffing.time_punch_events (
    event_id,
    tenant_id,
    person_uuid,
    punch_time,
    punch_type,
    source_provider,
    payload,
    source_raw_payload,
    device_info,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    p_person_uuid,
    p_punch_time,
    p_punch_type,
    p_source_provider,
    v_payload,
    v_source_raw,
    v_device,
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT (event_id) DO NOTHING
  RETURNING id INTO v_event_db_id;

  IF v_event_db_id IS NULL THEN
    SELECT * INTO v_existing
    FROM staffing.time_punch_events
    WHERE event_id = p_event_id;

    IF v_existing.tenant_id <> p_tenant_id
      OR v_existing.person_uuid <> p_person_uuid
      OR v_existing.punch_time <> p_punch_time
      OR v_existing.punch_type <> p_punch_type
      OR v_existing.source_provider <> p_source_provider
      OR v_existing.payload <> v_payload
      OR v_existing.source_raw_payload <> v_source_raw
      OR v_existing.device_info <> v_device
      OR v_existing.request_id <> p_request_id
      OR v_existing.initiator_id <> p_initiator_id
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_IDEMPOTENCY_REUSED',
        DETAIL = format('event_id=%s existing_id=%s', p_event_id, v_existing.id);
    END IF;

    RETURN v_existing.id;
  END IF;

  PERFORM staffing.recompute_daily_attendance_results_for_punch(p_tenant_id, p_person_uuid, p_punch_time);

  RETURN v_event_db_id;
END;
$$;
`,
		`GRANT EXECUTE ON FUNCTION staffing.recompute_daily_attendance_result(uuid, uuid, date) TO ` + runtimeRole + `;`,
		`GRANT EXECUTE ON FUNCTION staffing.recompute_daily_attendance_results_for_punch(uuid, uuid, timestamptz) TO ` + runtimeRole + `;`,
		`GRANT EXECUTE ON FUNCTION staffing.submit_time_punch_event(uuid, uuid, uuid, timestamptz, text, text, jsonb, jsonb, jsonb, text, uuid) TO ` + runtimeRole + `;`,
		`GRANT SELECT, INSERT ON staffing.time_profile_events TO ` + runtimeRole + `;`,
		`GRANT USAGE, SELECT ON SEQUENCE staffing.time_profile_events_id_seq TO ` + runtimeRole + `;`,
		`GRANT SELECT, INSERT ON staffing.time_profile_versions TO ` + runtimeRole + `;`,
		`GRANT USAGE, SELECT ON SEQUENCE staffing.time_profile_versions_id_seq TO ` + runtimeRole + `;`,
		`GRANT SELECT ON staffing.holiday_day_events TO ` + runtimeRole + `;`,
		`GRANT SELECT ON staffing.holiday_days TO ` + runtimeRole + `;`,
		`GRANT SELECT, INSERT, UPDATE ON staffing.daily_attendance_results TO ` + runtimeRole + `;`,
		`TRUNCATE staffing.time_profile_versions, staffing.time_profile_events, staffing.holiday_days, staffing.holiday_day_events, staffing.daily_attendance_results;`,
	}

	for _, s := range ddl {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, err := conn.Exec(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

func withUserPassword(dsn string, user string, password string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	u.User = url.UserPassword(user, password)
	return u.String(), nil
}
