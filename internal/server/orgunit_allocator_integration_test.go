package server

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestOrgunitAllocateOrgID_Concurrent(t *testing.T) {
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

	if err := ensureOrgunitAllocatorSchemaForTest(ctx, adminConn); err != nil {
		t.Fatal(err)
	}

	tenant := "00000000-0000-0000-0000-00000000a001"
	if _, err := adminConn.Exec(ctx, `DELETE FROM orgunit.org_id_allocators WHERE tenant_uuid = $1::uuid;`, tenant); err != nil {
		t.Fatal(err)
	}

	const workers = 8
	results := make(chan int, workers)
	errs := make(chan error, workers)

	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			conn, err := pgx.Connect(ctx, adminDSN)
			if err != nil {
				errs <- err
				return
			}
			defer conn.Close(context.Background())
			var id int
			tx, err := conn.Begin(ctx)
			if err != nil {
				errs <- err
				return
			}
			defer func() { _ = tx.Rollback(context.Background()) }()
			if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenant); err != nil {
				errs <- err
				return
			}
			if err := tx.QueryRow(ctx, `SELECT orgunit.allocate_org_id($1::uuid);`, tenant).Scan(&id); err != nil {
				errs <- err
				return
			}
			if err := tx.Commit(ctx); err != nil {
				errs <- err
				return
			}
			results <- id
		}()
	}

	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	seen := make(map[int]struct{})
	for id := range results {
		if _, ok := seen[id]; ok {
			t.Fatalf("duplicate org_id allocated: %d", id)
		}
		seen[id] = struct{}{}
		if id < 10000000 || id > 99999999 {
			t.Fatalf("org_id out of range: %d", id)
		}
	}
	if len(seen) != workers {
		t.Fatalf("expected %d ids, got %d", workers, len(seen))
	}
}

func TestOrgunitAllocateOrgID_Exhausted(t *testing.T) {
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

	if err := ensureOrgunitAllocatorSchemaForTest(ctx, adminConn); err != nil {
		t.Fatal(err)
	}

	tenant := "00000000-0000-0000-0000-00000000a002"
	if _, err := adminConn.Exec(ctx, `
INSERT INTO orgunit.org_id_allocators (tenant_uuid, next_org_id)
VALUES ($1::uuid, 100000000)
ON CONFLICT (tenant_uuid) DO UPDATE
SET next_org_id = EXCLUDED.next_org_id, updated_at = now();
`, tenant); err != nil {
		t.Fatal(err)
	}

	conn, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	var id int
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenant); err != nil {
		t.Fatal(err)
	}
	err = tx.QueryRow(ctx, `SELECT orgunit.allocate_org_id($1::uuid);`, tenant).Scan(&id)
	if err == nil {
		t.Fatal("expected ORG_ID_EXHAUSTED error")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Message != "ORG_ID_EXHAUSTED" {
		t.Fatalf("expected ORG_ID_EXHAUSTED, got %v", err)
	}
}

func ensureOrgunitAllocatorSchemaForTest(ctx context.Context, conn *pgx.Conn) error {
	ddl := []string{
		`CREATE EXTENSION IF NOT EXISTS pgcrypto;`,
		`CREATE SCHEMA IF NOT EXISTS orgunit;`,
		`
CREATE TABLE IF NOT EXISTS orgunit.org_id_allocators (
  tenant_uuid uuid NOT NULL,
  next_org_id int NOT NULL CHECK (next_org_id BETWEEN 10000000 AND 100000000),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_uuid)
);
`,
		`
CREATE OR REPLACE FUNCTION orgunit.assert_current_tenant(p_tenant_uuid uuid)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  IF current_setting('app.current_tenant', true) IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'RLS_TENANT_CONTEXT_MISSING';
  END IF;
  IF current_setting('app.current_tenant')::uuid <> p_tenant_uuid THEN
    RAISE EXCEPTION USING MESSAGE = 'RLS_TENANT_MISMATCH';
  END IF;
END;
$$;
`,
		`
CREATE OR REPLACE FUNCTION orgunit.allocate_org_id(p_tenant_uuid uuid)
RETURNS int
LANGUAGE plpgsql
AS $$
DECLARE
  v_next int;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  INSERT INTO orgunit.org_id_allocators (tenant_uuid, next_org_id)
  VALUES (p_tenant_uuid, 10000001)
  ON CONFLICT (tenant_uuid) DO UPDATE
  SET next_org_id = orgunit.org_id_allocators.next_org_id + 1,
      updated_at = now()
  WHERE orgunit.org_id_allocators.next_org_id <= 99999999
  RETURNING next_org_id - 1 INTO v_next;

  IF v_next IS NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_ID_EXHAUSTED',
      DETAIL = format('tenant_uuid=%s', p_tenant_uuid);
  END IF;

  RETURN v_next;
END;
$$;
`,
	}

	for _, stmt := range ddl {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
