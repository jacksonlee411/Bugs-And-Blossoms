package orgunit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type stubTx struct {
	execErr error
	row     pgx.Row
	rowErr  error
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

func TestNormalizeOrgCode(t *testing.T) {
	t.Run("valid normalize", func(t *testing.T) {
		got, err := NormalizeOrgCode("a_b-1")
		if err != nil || got != "A_B-1" {
			t.Fatalf("got=%q err=%v", got, err)
		}
	})

	t.Run("empty invalid", func(t *testing.T) {
		if _, err := NormalizeOrgCode(""); !errors.Is(err, ErrOrgCodeInvalid) {
			t.Fatalf("expected ErrOrgCodeInvalid, got %v", err)
		}
	})

	t.Run("trim invalid", func(t *testing.T) {
		if _, err := NormalizeOrgCode(" A1 "); !errors.Is(err, ErrOrgCodeInvalid) {
			t.Fatalf("expected ErrOrgCodeInvalid, got %v", err)
		}
	})

	t.Run("pattern invalid", func(t *testing.T) {
		if _, err := NormalizeOrgCode("A 1"); !errors.Is(err, ErrOrgCodeInvalid) {
			t.Fatalf("expected ErrOrgCodeInvalid, got %v", err)
		}
	})
}

func TestResolveOrgID(t *testing.T) {
	t.Run("invalid org code", func(t *testing.T) {
		if _, err := ResolveOrgID(context.Background(), &stubTx{}, "t1", " bad "); !errors.Is(err, ErrOrgCodeInvalid) {
			t.Fatalf("expected ErrOrgCodeInvalid, got %v", err)
		}
	})

	t.Run("assert tenant error", func(t *testing.T) {
		tx := &stubTx{execErr: errors.New("exec fail")}
		if _, err := ResolveOrgID(context.Background(), tx, "t1", "A1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("not found", func(t *testing.T) {
		tx := &stubTx{rowErr: pgx.ErrNoRows}
		if _, err := ResolveOrgID(context.Background(), tx, "t1", "A1"); !errors.Is(err, ErrOrgCodeNotFound) {
			t.Fatalf("expected ErrOrgCodeNotFound, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		tx := &stubTx{row: stubRow{vals: []any{12345678}}}
		id, err := ResolveOrgID(context.Background(), tx, "t1", "a1")
		if err != nil || id != 12345678 {
			t.Fatalf("id=%d err=%v", id, err)
		}
	})
}

func TestResolveOrgCode(t *testing.T) {
	t.Run("assert tenant error", func(t *testing.T) {
		tx := &stubTx{execErr: errors.New("exec fail")}
		if _, err := ResolveOrgCode(context.Background(), tx, "t1", 1); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("not found", func(t *testing.T) {
		tx := &stubTx{rowErr: pgx.ErrNoRows}
		if _, err := ResolveOrgCode(context.Background(), tx, "t1", 1); !errors.Is(err, ErrOrgIDNotFound) {
			t.Fatalf("expected ErrOrgIDNotFound, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		tx := &stubTx{row: stubRow{vals: []any{"A1"}}}
		code, err := ResolveOrgCode(context.Background(), tx, "t1", 1)
		if err != nil || code != "A1" {
			t.Fatalf("code=%q err=%v", code, err)
		}
	})
}
