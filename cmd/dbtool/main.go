package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
)

func main() {
	if len(os.Args) < 2 {
		fatalf("usage: dbtool <rls-smoke|orgunit-smoke> [args]")
	}

	switch os.Args[1] {
	case "rls-smoke":
		rlsSmoke(os.Args[2:])
	case "orgunit-smoke":
		orgunitSmoke(os.Args[2:])
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
	_, err = tx.Exec(ctx, `SELECT count(*) FROM orgunit.nodes;`)
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

	var nodeID string
	if err := tx.QueryRow(ctx, `
SELECT orgunit.submit_orgunit_event(
  $1::uuid,
  'node_created',
  jsonb_build_object('name', 'A1')
)::text
`, tenantA).Scan(&nodeID); err != nil {
		fatal(err)
	}

	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM orgunit.nodes;`).Scan(&count); err != nil {
		fatal(err)
	}
	if count != 1 {
		fatalf("expected count=1 under tenant A, got %d", count)
	}

	if _, err := tx.Exec(ctx, `SAVEPOINT sp_cross_event;`); err != nil {
		fatal(err)
	}
	_, err = tx.Exec(ctx, `
SELECT orgunit.submit_orgunit_event(
  $1::uuid,
  'node_created',
  jsonb_build_object('name', 'B1')
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
	if err := tx2.QueryRow(ctx, `SELECT count(*) FROM orgunit.nodes;`).Scan(&count); err != nil {
		fatal(err)
	}
	if count != 0 {
		fatalf("expected count=0 under tenant B, got %d (node created under A: %s)", count, nodeID)
	}
	if err := tx2.Commit(ctx); err != nil {
		fatal(err)
	}

	fmt.Println("[orgunit-smoke] OK")
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
