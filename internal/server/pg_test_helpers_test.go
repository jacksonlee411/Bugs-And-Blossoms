package server

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type beginnerFunc func(ctx context.Context) (pgx.Tx, error)

func (f beginnerFunc) Begin(ctx context.Context) (pgx.Tx, error) { return f(ctx) }

type stubTx struct {
	execErr    error
	execErrAt  int
	execN      int
	execSQLs   []string
	queryErr   error
	queryErrAt int
	queryN     int
	commitErr  error
	rowErr     error
	row2Err    error
	row3Err    error
	row4Err    error
	row5Err    error
	row6Err    error

	rows  pgx.Rows
	rows2 pgx.Rows
	rows3 pgx.Rows
	row   pgx.Row
	row2  pgx.Row
	row3  pgx.Row
	row4  pgx.Row
	row5  pgx.Row
	row6  pgx.Row
}

func (t *stubTx) Begin(ctx context.Context) (pgx.Tx, error) { return t, nil }
func (t *stubTx) Commit(context.Context) error              { return t.commitErr }
func (t *stubTx) Rollback(context.Context) error            { return nil }
func (t *stubTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *stubTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return fakeBatchResults{} }
func (t *stubTx) LargeObjects() pgx.LargeObjects                         { return pgx.LargeObjects{} }
func (t *stubTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *stubTx) Conn() *pgx.Conn { return nil }

func (t *stubTx) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	t.execSQLs = append(t.execSQLs, sql)
	t.execN++
	if t.execErr != nil {
		at := t.execErrAt
		if at == 0 {
			at = 1
		}
		if t.execN == at {
			return pgconn.CommandTag{}, t.execErr
		}
	}
	return pgconn.CommandTag{}, nil
}

func (t *stubTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	t.queryN++
	if t.queryErr != nil {
		at := t.queryErrAt
		if at == 0 {
			at = 1
		}
		if t.queryN == at {
			return nil, t.queryErr
		}
	}
	if t.queryN == 1 && t.rows != nil {
		return t.rows, nil
	}
	if t.queryN == 2 && t.rows2 != nil {
		return t.rows2, nil
	}
	if t.queryN == 3 && t.rows3 != nil {
		return t.rows3, nil
	}
	if t.rows != nil {
		return t.rows, nil
	}
	return &fakeRows{}, nil
}

func (t *stubTx) QueryRow(context.Context, string, ...any) pgx.Row {
	if t.rowErr != nil {
		return &stubRow{err: t.rowErr}
	}
	if t.row != nil {
		r := t.row
		t.row = nil
		return r
	}
	if t.row2Err != nil {
		return &stubRow{err: t.row2Err}
	}
	if t.row2 != nil {
		r := t.row2
		t.row2 = nil
		return r
	}
	if t.row3Err != nil {
		return &stubRow{err: t.row3Err}
	}
	if t.row3 != nil {
		r := t.row3
		t.row3 = nil
		return r
	}
	if t.row4Err != nil {
		return &stubRow{err: t.row4Err}
	}
	if t.row4 != nil {
		r := t.row4
		t.row4 = nil
		return r
	}
	if t.row5Err != nil {
		return &stubRow{err: t.row5Err}
	}
	if t.row5 != nil {
		r := t.row5
		t.row5 = nil
		return r
	}
	if t.row6Err != nil {
		return &stubRow{err: t.row6Err}
	}
	if t.row6 != nil {
		r := t.row6
		t.row6 = nil
		return r
	}
	return fakeRow{}
}

type stubRows struct {
	empty   bool
	nextN   int
	scanErr error
	err     error
}

func (r *stubRows) Close()                        {}
func (r *stubRows) Err() error                    { return r.err }
func (r *stubRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *stubRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *stubRows) Next() bool {
	if r.empty {
		return false
	}
	if r.nextN > 0 {
		return false
	}
	r.nextN++
	return true
}
func (r *stubRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	return (&fakeRows{}).Scan(dest...)
}
func (r *stubRows) Values() ([]any, error) { return nil, nil }
func (r *stubRows) RawValues() [][]byte    { return nil }
func (r *stubRows) Conn() *pgx.Conn        { return nil }

type stubRow struct {
	vals []any
	err  error
}

func (r *stubRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return fakeRow{vals: r.vals}.Scan(dest...)
}

type fakeBeginner struct {
	beginCount int
}

func (b *fakeBeginner) Begin(context.Context) (pgx.Tx, error) {
	b.beginCount++
	return &fakeTx{beginCount: b.beginCount}, nil
}

type fakeTx struct {
	beginCount int
	orgIDN     int
	uuidN      int
	committed  bool
	rolled     bool
}

func (t *fakeTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (t *fakeTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return &fakeRows{idx: 0}, nil
}

func (t *fakeTx) QueryRow(_ context.Context, q string, _ ...any) pgx.Row {
	if strings.Contains(q, "gen_random_uuid") {
		t.uuidN++
		switch t.uuidN {
		case 1:
			return fakeRow{vals: []any{"u1"}}
		default:
			return fakeRow{vals: []any{"e1"}}
		}
	}
	if strings.Contains(q, "FROM orgunit.org_events") {
		t.orgIDN++
		return fakeRow{vals: []any{10000000 + t.orgIDN, time.Unix(789, 0).UTC()}}
	}
	return &stubRow{err: errors.New("unexpected QueryRow")}
}

func (t *fakeTx) Commit(context.Context) error   { t.committed = true; return nil }
func (t *fakeTx) Rollback(context.Context) error { t.rolled = true; return nil }

func (t *fakeTx) Begin(context.Context) (pgx.Tx, error) { return t, nil }
func (t *fakeTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *fakeTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return fakeBatchResults{} }
func (t *fakeTx) LargeObjects() pgx.LargeObjects                         { return pgx.LargeObjects{} }
func (t *fakeTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *fakeTx) Conn() *pgx.Conn { return nil }

type fakeBatchResults struct{}

func (fakeBatchResults) Exec() (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil }
func (fakeBatchResults) Query() (pgx.Rows, error)         { return &fakeRows{}, nil }
func (fakeBatchResults) QueryRow() pgx.Row                { return fakeRow{} }
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
	strings := []string{"n1", "Node"}
	stringN := 0
	for _, d := range dest {
		switch v := d.(type) {
		case *string:
			if stringN < len(strings) {
				*v = strings[stringN]
				stringN++
				continue
			}
			*v = ""
		case *bool:
			*v = false
		case *time.Time:
			*v = time.Unix(123, 0).UTC()
		case *[]int:
			*v = []int{1, 2}
		case *[]byte:
			*v = []byte(`{}`)
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

type fakeRow struct {
	vals []any
}

func (r fakeRow) Scan(dest ...any) error {
	for i := range dest {
		if i >= len(r.vals) || r.vals[i] == nil {
			switch d := dest[i].(type) {
			case *string:
				*d = ""
			case *time.Time:
				*d = time.Time{}
			case *[]int:
				*d = nil
			case *[]byte:
				*d = nil
			case **time.Time:
				*d = nil
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
		case *[]int:
			switch v := r.vals[i].(type) {
			case []int:
				*d = append([]int(nil), v...)
			case []int32:
				out := make([]int, 0, len(v))
				for _, n := range v {
					out = append(out, int(n))
				}
				*d = out
			default:
				*d = nil
			}
		case *[]byte:
			switch v := r.vals[i].(type) {
			case []byte:
				*d = append([]byte(nil), v...)
			case string:
				*d = []byte(v)
			default:
				*d = nil
			}
		case **time.Time:
			v := r.vals[i].(time.Time)
			*d = &v
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
