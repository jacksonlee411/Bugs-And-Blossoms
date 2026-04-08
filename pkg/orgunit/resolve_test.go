package orgunit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	orgunit "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type stubTx struct {
	execErr  error
	queryErr error
	rows     pgx.Rows
	row      pgx.Row
	rowErr   error
}

func (t *stubTx) Begin(context.Context) (pgx.Tx, error) { return t, nil }
func (t *stubTx) Commit(context.Context) error          { return nil }
func (t *stubTx) Rollback(context.Context) error        { return nil }
func (t *stubTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *stubTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return fakeBatchResults{} }
func (t *stubTx) LargeObjects() pgx.LargeObjects                         { return pgx.LargeObjects{} }
func (t *stubTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *stubTx) Conn() *pgx.Conn { return nil }

func (t *stubTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	if t.execErr != nil {
		return pgconn.CommandTag{}, t.execErr
	}
	return pgconn.CommandTag{}, nil
}

func (t *stubTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	if t.queryErr != nil {
		return nil, t.queryErr
	}
	if t.rows != nil {
		return t.rows, nil
	}
	return &fakeRows{}, nil
}

func (t *stubTx) QueryRow(context.Context, string, ...any) pgx.Row {
	if t.rowErr != nil {
		return stubRow{err: t.rowErr}
	}
	if t.row != nil {
		r := t.row
		t.row = nil
		return r
	}
	return stubRow{err: pgx.ErrNoRows}
}

type stubRow struct {
	vals []any
	err  error
}

func (r stubRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i := range dest {
		if i >= len(r.vals) || r.vals[i] == nil {
			switch d := dest[i].(type) {
			case *string:
				*d = ""
			case *time.Time:
				*d = time.Time{}
			case *bool:
				*d = false
			case *int:
				*d = 0
			case *int64:
				*d = 0
			}
			continue
		}
		switch d := dest[i].(type) {
		case *string:
			*d = r.vals[i].(string)
		case *time.Time:
			*d = r.vals[i].(time.Time)
		case *bool:
			*d = r.vals[i].(bool)
		case *int:
			*d = r.vals[i].(int)
		case *int64:
			*d = r.vals[i].(int64)
		}
	}
	return nil
}

type fakeBatchResults struct{}

func (fakeBatchResults) Exec() (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil }
func (fakeBatchResults) Query() (pgx.Rows, error)         { return &fakeRows{}, nil }
func (fakeBatchResults) QueryRow() pgx.Row                { return stubRow{} }
func (fakeBatchResults) Close() error                     { return nil }

type fakeRows struct {
	idx int
}

func (r *fakeRows) Close()                        {}
func (r *fakeRows) Err() error                    { return nil }
func (r *fakeRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *fakeRows) Next() bool {
	if r.idx > 0 {
		return false
	}
	r.idx++
	return true
}
func (r *fakeRows) Scan(dest ...any) error {
	for _, d := range dest {
		switch v := d.(type) {
		case *string:
			*v = ""
		case *bool:
			*v = false
		case *time.Time:
			*v = time.Unix(0, 0).UTC()
		case *int:
			*v = 0
		case *int64:
			*v = 0
		default:
			return errors.New("unsupported scan type")
		}
	}
	return nil
}
func (r *fakeRows) Values() ([]any, error) { return nil, nil }
func (r *fakeRows) RawValues() [][]byte    { return nil }
func (r *fakeRows) Conn() *pgx.Conn        { return nil }

type stubRows struct {
	idx     int
	rows    [][]any
	err     error
	scanErr error
}

func (r *stubRows) Close()                        {}
func (r *stubRows) Err() error                    { return r.err }
func (r *stubRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *stubRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *stubRows) Next() bool {
	return r.idx < len(r.rows)
}
func (r *stubRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	if r.idx >= len(r.rows) {
		return errors.New("no rows")
	}
	row := r.rows[r.idx]
	r.idx++
	return stubRow{vals: row}.Scan(dest...)
}
func (r *stubRows) Values() ([]any, error) { return nil, nil }
func (r *stubRows) RawValues() [][]byte    { return nil }
func (r *stubRows) Conn() *pgx.Conn        { return nil }

func TestResolveOrgID_BlackBox(t *testing.T) {
	t.Parallel()

	t.Run("invalid org code", func(t *testing.T) {
		t.Parallel()

		if _, err := orgunit.ResolveOrgID(context.Background(), &stubTx{}, "t1", "bad\n"); !errors.Is(err, orgunit.ErrOrgCodeInvalid) {
			t.Fatalf("expected ErrOrgCodeInvalid, got %v", err)
		}
	})

	t.Run("assert tenant error", func(t *testing.T) {
		t.Parallel()

		tx := &stubTx{execErr: errors.New("exec fail")}
		if _, err := orgunit.ResolveOrgID(context.Background(), tx, "t1", "A1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		tx := &stubTx{rowErr: pgx.ErrNoRows}
		if _, err := orgunit.ResolveOrgID(context.Background(), tx, "t1", "A1"); !errors.Is(err, orgunit.ErrOrgCodeNotFound) {
			t.Fatalf("expected ErrOrgCodeNotFound, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		tx := &stubTx{row: stubRow{vals: []any{12345678}}}
		id, err := orgunit.ResolveOrgID(context.Background(), tx, "t1", "a1")
		if err != nil || id != 12345678 {
			t.Fatalf("id=%d err=%v", id, err)
		}
	})
}

func TestResolveOrgCode_BlackBox(t *testing.T) {
	t.Parallel()

	t.Run("assert tenant error", func(t *testing.T) {
		t.Parallel()

		tx := &stubTx{execErr: errors.New("exec fail")}
		if _, err := orgunit.ResolveOrgCode(context.Background(), tx, "t1", 1); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		tx := &stubTx{rowErr: pgx.ErrNoRows}
		if _, err := orgunit.ResolveOrgCode(context.Background(), tx, "t1", 1); !errors.Is(err, orgunit.ErrOrgIDNotFound) {
			t.Fatalf("expected ErrOrgIDNotFound, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		tx := &stubTx{row: stubRow{vals: []any{"A1"}}}
		code, err := orgunit.ResolveOrgCode(context.Background(), tx, "t1", 1)
		if err != nil || code != "A1" {
			t.Fatalf("code=%q err=%v", code, err)
		}
	})
}

func TestResolveOrgCodes_BlackBox(t *testing.T) {
	t.Parallel()

	t.Run("empty input", func(t *testing.T) {
		t.Parallel()

		got, err := orgunit.ResolveOrgCodes(context.Background(), &stubTx{}, "t1", nil)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(got) != 0 {
			t.Fatalf("expected empty map, got %v", got)
		}
	})

	t.Run("assert tenant error", func(t *testing.T) {
		t.Parallel()

		tx := &stubTx{execErr: errors.New("exec fail")}
		if _, err := orgunit.ResolveOrgCodes(context.Background(), tx, "t1", []int{1}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query error", func(t *testing.T) {
		t.Parallel()

		tx := &stubTx{queryErr: errors.New("query fail")}
		if _, err := orgunit.ResolveOrgCodes(context.Background(), tx, "t1", []int{1}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows error", func(t *testing.T) {
		t.Parallel()

		tx := &stubTx{rows: &stubRows{err: errors.New("rows fail")}}
		if _, err := orgunit.ResolveOrgCodes(context.Background(), tx, "t1", []int{1}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan error", func(t *testing.T) {
		t.Parallel()

		tx := &stubTx{rows: &stubRows{rows: [][]any{{1, "A1"}}, scanErr: errors.New("scan fail")}}
		if _, err := orgunit.ResolveOrgCodes(context.Background(), tx, "t1", []int{1}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		tx := &stubTx{rows: &stubRows{rows: [][]any{{1, "A1"}, {2, "B2"}}}}
		got, err := orgunit.ResolveOrgCodes(context.Background(), tx, "t1", []int{1, 2})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if got[1] != "A1" || got[2] != "B2" {
			t.Fatalf("unexpected map: %v", got)
		}
	})
}
