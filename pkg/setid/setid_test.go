package setid

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeExecer struct {
	sql  string
	args []any
	err  error
}

func (e *fakeExecer) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	e.sql = sql
	e.args = args
	return pgconn.CommandTag{}, e.err
}

type fakeRow struct {
	val string
	err error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*(dest[0].(*string)) = r.val
	return nil
}

type fakeQueryRower struct {
	sql  string
	args []any
	row  pgx.Row
}

func (q *fakeQueryRower) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	q.sql = sql
	q.args = args
	return q.row
}

func TestEnsureBootstrap(t *testing.T) {
	var e fakeExecer
	if err := EnsureBootstrap(context.Background(), &e, "t1", "p1"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if e.sql == "" || len(e.args) != 2 {
		t.Fatalf("unexpected exec call: sql=%q args=%v", e.sql, e.args)
	}
}

func TestEnsureBootstrap_Error(t *testing.T) {
	want := errors.New("boom")
	e := &fakeExecer{err: want}
	if err := EnsureBootstrap(context.Background(), e, "t1", "p1"); !errors.Is(err, want) {
		t.Fatalf("err=%v", err)
	}
}

func TestResolve(t *testing.T) {
	q := &fakeQueryRower{row: fakeRow{val: "SHARE"}}
	got, err := Resolve(context.Background(), q, "t1", "10000001", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if got != "SHARE" {
		t.Fatalf("got=%q", got)
	}
	if q.sql == "" || len(q.args) != 3 {
		t.Fatalf("unexpected query call: sql=%q args=%v", q.sql, q.args)
	}
}

func TestResolve_Error(t *testing.T) {
	want := errors.New("scan fail")
	q := &fakeQueryRower{row: fakeRow{err: want}}
	_, err := Resolve(context.Background(), q, "t1", "10000001", "2026-01-01")
	if !errors.Is(err, want) {
		t.Fatalf("err=%v", err)
	}
}
