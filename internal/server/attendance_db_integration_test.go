package server

import (
	"context"
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

func withUserPassword(dsn string, user string, password string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	u.User = url.UserPassword(user, password)
	return u.String(), nil
}
