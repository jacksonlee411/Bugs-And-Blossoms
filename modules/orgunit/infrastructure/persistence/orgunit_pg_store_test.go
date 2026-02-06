package persistence

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/ports"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
)

type beginFunc func(ctx context.Context) (pgx.Tx, error)

func (f beginFunc) Begin(ctx context.Context) (pgx.Tx, error) { return f(ctx) }

type txStub struct {
	execErr   error
	row       pgx.Row
	commitErr error
}

func (t *txStub) Begin(context.Context) (pgx.Tx, error) { return t, nil }
func (t *txStub) Commit(context.Context) error          { return t.commitErr }
func (t *txStub) Rollback(context.Context) error        { return nil }
func (t *txStub) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *txStub) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return fakeBatchResults{} }
func (t *txStub) LargeObjects() pgx.LargeObjects                         { return pgx.LargeObjects{} }
func (t *txStub) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *txStub) Conn() *pgx.Conn { return nil }

func (t *txStub) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, t.execErr
}

func (t *txStub) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return &stubRows{}, nil
}

func (t *txStub) QueryRow(context.Context, string, ...any) pgx.Row {
	if t.row != nil {
		return t.row
	}
	return stubRow{err: errors.New("row not mocked")}
}

type stubRows struct{}

func (r *stubRows) Close()                        {}
func (r *stubRows) Err() error                    { return nil }
func (r *stubRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *stubRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *stubRows) Next() bool             { return false }
func (r *stubRows) Scan(...any) error      { return nil }
func (r *stubRows) Values() ([]any, error) { return nil, nil }
func (r *stubRows) RawValues() [][]byte    { return nil }
func (r *stubRows) Conn() *pgx.Conn        { return nil }

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
			continue
		}
		switch d := dest[i].(type) {
		case *string:
			*d = r.vals[i].(string)
		case *int:
			*d = r.vals[i].(int)
		case *int64:
			*d = r.vals[i].(int64)
		case *types.OrgUnitEventType:
			switch v := r.vals[i].(type) {
			case types.OrgUnitEventType:
				*d = v
			case string:
				*d = types.OrgUnitEventType(v)
			}
		case *[]byte:
			*d = r.vals[i].([]byte)
		case *time.Time:
			*d = r.vals[i].(time.Time)
		}
	}
	return nil
}

type fakeBatchResults struct{}

func (fakeBatchResults) Exec() (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil }
func (fakeBatchResults) Query() (pgx.Rows, error)         { return &stubRows{}, nil }
func (fakeBatchResults) QueryRow() pgx.Row                { return stubRow{} }
func (fakeBatchResults) Close() error                     { return nil }

func TestOrgUnitPGStore_SubmitEvent(t *testing.T) {
	ctx := context.Background()

	store := NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return nil, errors.New("begin")
	}))
	if _, err := store.SubmitEvent(ctx, "t1", "e1", nil, "CREATE", "2026-01-01", nil, "r1", "t1"); err == nil {
		t.Fatal("expected begin error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{execErr: errors.New("exec")}, nil
	}))
	if _, err := store.SubmitEvent(ctx, "t1", "e1", nil, "CREATE", "2026-01-01", nil, "r1", "t1"); err == nil {
		t.Fatal("expected exec error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{err: errors.New("row")}}, nil
	}))
	if _, err := store.SubmitEvent(ctx, "t1", "e1", nil, "CREATE", "2026-01-01", nil, "r1", "t1"); err == nil {
		t.Fatal("expected row error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{vals: []any{int64(10)}}, commitErr: errors.New("commit")}, nil
	}))
	if _, err := store.SubmitEvent(ctx, "t1", "e1", nil, "CREATE", "2026-01-01", nil, "r1", "t1"); err == nil {
		t.Fatal("expected commit error")
	}

	orgID := 10000001
	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{vals: []any{int64(10)}}}, nil
	}))
	if _, err := store.SubmitEvent(ctx, "t1", "e1", &orgID, "RENAME", "2026-01-01", nil, "r1", "t1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOrgUnitPGStore_SubmitCorrection(t *testing.T) {
	ctx := context.Background()

	store := NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return nil, errors.New("begin")
	}))
	if _, err := store.SubmitCorrection(ctx, "t1", 1, "2026-01-01", nil, "req", "t1"); err == nil {
		t.Fatal("expected begin error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{execErr: errors.New("exec")}, nil
	}))
	if _, err := store.SubmitCorrection(ctx, "t1", 1, "2026-01-01", nil, "req", "t1"); err == nil {
		t.Fatal("expected exec error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{err: errors.New("row")}}, nil
	}))
	if _, err := store.SubmitCorrection(ctx, "t1", 1, "2026-01-01", nil, "req", "t1"); err == nil {
		t.Fatal("expected row error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{vals: []any{"corr"}}, commitErr: errors.New("commit")}, nil
	}))
	if _, err := store.SubmitCorrection(ctx, "t1", 1, "2026-01-01", nil, "req", "t1"); err == nil {
		t.Fatal("expected commit error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{vals: []any{"corr"}}}, nil
	}))
	if _, err := store.SubmitCorrection(ctx, "t1", 1, "2026-01-01", nil, "req", "t1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOrgUnitPGStore_SubmitRescindEvent(t *testing.T) {
	ctx := context.Background()

	store := NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return nil, errors.New("begin")
	}))
	if _, err := store.SubmitRescindEvent(ctx, "t1", 1, "2026-01-01", "bad", "req", "t1"); err == nil {
		t.Fatal("expected begin error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{execErr: errors.New("exec")}, nil
	}))
	if _, err := store.SubmitRescindEvent(ctx, "t1", 1, "2026-01-01", "bad", "req", "t1"); err == nil {
		t.Fatal("expected exec error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{err: errors.New("row")}}, nil
	}))
	if _, err := store.SubmitRescindEvent(ctx, "t1", 1, "2026-01-01", "bad", "req", "t1"); err == nil {
		t.Fatal("expected row error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{vals: []any{"corr"}}, commitErr: errors.New("commit")}, nil
	}))
	if _, err := store.SubmitRescindEvent(ctx, "t1", 1, "2026-01-01", "bad", "req", "t1"); err == nil {
		t.Fatal("expected commit error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{vals: []any{"corr"}}}, nil
	}))
	if _, err := store.SubmitRescindEvent(ctx, "t1", 1, "2026-01-01", "bad", "req", "t1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOrgUnitPGStore_SubmitRescindOrg(t *testing.T) {
	ctx := context.Background()

	store := NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return nil, errors.New("begin")
	}))
	if _, err := store.SubmitRescindOrg(ctx, "t1", 1, "bad", "req", "t1"); err == nil {
		t.Fatal("expected begin error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{execErr: errors.New("exec")}, nil
	}))
	if _, err := store.SubmitRescindOrg(ctx, "t1", 1, "bad", "req", "t1"); err == nil {
		t.Fatal("expected exec error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{err: errors.New("row")}}, nil
	}))
	if _, err := store.SubmitRescindOrg(ctx, "t1", 1, "bad", "req", "t1"); err == nil {
		t.Fatal("expected row error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{vals: []any{2}}, commitErr: errors.New("commit")}, nil
	}))
	if _, err := store.SubmitRescindOrg(ctx, "t1", 1, "bad", "req", "t1"); err == nil {
		t.Fatal("expected commit error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{vals: []any{2}}}, nil
	}))
	if _, err := store.SubmitRescindOrg(ctx, "t1", 1, "bad", "req", "t1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOrgUnitPGStore_FindEventByUUID(t *testing.T) {
	ctx := context.Background()

	store := NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return nil, errors.New("begin")
	}))
	if _, err := store.FindEventByUUID(ctx, "t1", "e1"); err == nil {
		t.Fatal("expected begin error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{execErr: errors.New("exec")}, nil
	}))
	if _, err := store.FindEventByUUID(ctx, "t1", "e1"); err == nil {
		t.Fatal("expected exec error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{err: pgx.ErrNoRows}}, nil
	}))
	if _, err := store.FindEventByUUID(ctx, "t1", "e1"); !errors.Is(err, ports.ErrOrgEventNotFound) {
		t.Fatalf("expected org event not found, got %v", err)
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{err: errors.New("row")}}, nil
	}))
	if _, err := store.FindEventByUUID(ctx, "t1", "e1"); err == nil {
		t.Fatal("expected row error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{vals: []any{int64(1), "e1", 10000001, "CREATE", "2026-01-01", []byte(`{"a":"b"}`), time.Unix(1, 0).UTC()}}, commitErr: errors.New("commit")}, nil
	}))
	if _, err := store.FindEventByUUID(ctx, "t1", "e1"); err == nil {
		t.Fatal("expected commit error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{vals: []any{int64(1), "e1", 10000001, "CREATE", "2026-01-01", []byte(`{"a":"b"}`), time.Unix(1, 0).UTC()}}}, nil
	}))
	if _, err := store.FindEventByUUID(ctx, "t1", "e1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOrgUnitPGStore_FindEventByEffectiveDate(t *testing.T) {
	ctx := context.Background()

	store := NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return nil, errors.New("begin")
	}))
	if _, err := store.FindEventByEffectiveDate(ctx, "t1", 1, "2026-01-01"); err == nil {
		t.Fatal("expected begin error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{execErr: errors.New("exec")}, nil
	}))
	if _, err := store.FindEventByEffectiveDate(ctx, "t1", 1, "2026-01-01"); err == nil {
		t.Fatal("expected exec error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{err: pgx.ErrNoRows}}, nil
	}))
	if _, err := store.FindEventByEffectiveDate(ctx, "t1", 1, "2026-01-01"); !errors.Is(err, ports.ErrOrgEventNotFound) {
		t.Fatalf("expected org event not found, got %v", err)
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{err: errors.New("row")}}, nil
	}))
	if _, err := store.FindEventByEffectiveDate(ctx, "t1", 1, "2026-01-01"); err == nil {
		t.Fatal("expected row error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{vals: []any{int64(1), "e1", 10000001, "CREATE", "2026-01-01", []byte(`{"a":"b"}`), time.Unix(1, 0).UTC()}}, commitErr: errors.New("commit")}, nil
	}))
	if _, err := store.FindEventByEffectiveDate(ctx, "t1", 1, "2026-01-01"); err == nil {
		t.Fatal("expected commit error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{vals: []any{int64(1), "e1", 10000001, "CREATE", "2026-01-01", []byte(`{"a":"b"}`), time.Unix(1, 0).UTC()}}}, nil
	}))
	if _, err := store.FindEventByEffectiveDate(ctx, "t1", 1, "2026-01-01"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOrgUnitPGStore_ResolveOrgID(t *testing.T) {
	ctx := context.Background()

	store := NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return nil, errors.New("begin")
	}))
	if _, err := store.ResolveOrgID(ctx, "t1", "ROOT"); err == nil {
		t.Fatal("expected begin error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{execErr: errors.New("exec")}, nil
	}))
	if _, err := store.ResolveOrgID(ctx, "t1", "ROOT"); err == nil {
		t.Fatal("expected exec error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{err: pgx.ErrNoRows}}, nil
	}))
	if _, err := store.ResolveOrgID(ctx, "t1", "ROOT"); err == nil {
		t.Fatal("expected resolve error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{vals: []any{10000001}}, commitErr: errors.New("commit")}, nil
	}))
	if _, err := store.ResolveOrgID(ctx, "t1", "ROOT"); err == nil {
		t.Fatal("expected commit error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{vals: []any{10000001}}}, nil
	}))
	if _, err := store.ResolveOrgID(ctx, "t1", "ROOT"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOrgUnitPGStore_ResolveOrgCode(t *testing.T) {
	ctx := context.Background()

	store := NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return nil, errors.New("begin")
	}))
	if _, err := store.ResolveOrgCode(ctx, "t1", 1); err == nil {
		t.Fatal("expected begin error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{execErr: errors.New("exec")}, nil
	}))
	if _, err := store.ResolveOrgCode(ctx, "t1", 1); err == nil {
		t.Fatal("expected exec error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{err: pgx.ErrNoRows}}, nil
	}))
	if _, err := store.ResolveOrgCode(ctx, "t1", 1); err == nil {
		t.Fatal("expected resolve error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{vals: []any{"ROOT"}}, commitErr: errors.New("commit")}, nil
	}))
	if _, err := store.ResolveOrgCode(ctx, "t1", 1); err == nil {
		t.Fatal("expected commit error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{vals: []any{"ROOT"}}}, nil
	}))
	if _, err := store.ResolveOrgCode(ctx, "t1", 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOrgUnitPGStore_FindPersonByPernr(t *testing.T) {
	ctx := context.Background()

	store := NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return nil, errors.New("begin")
	}))
	if _, err := store.FindPersonByPernr(ctx, "t1", "1001"); err == nil {
		t.Fatal("expected begin error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{execErr: errors.New("exec")}, nil
	}))
	if _, err := store.FindPersonByPernr(ctx, "t1", "1001"); err == nil {
		t.Fatal("expected exec error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{err: pgx.ErrNoRows}}, nil
	}))
	if _, err := store.FindPersonByPernr(ctx, "t1", "1001"); !errors.Is(err, ports.ErrPersonNotFound) {
		t.Fatalf("expected person not found, got %v", err)
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{err: errors.New("row")}}, nil
	}))
	if _, err := store.FindPersonByPernr(ctx, "t1", "1001"); err == nil {
		t.Fatal("expected row error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{vals: []any{"p1", "1001", "Name", "active"}}, commitErr: errors.New("commit")}, nil
	}))
	if _, err := store.FindPersonByPernr(ctx, "t1", "1001"); err == nil {
		t.Fatal("expected commit error")
	}

	store = NewOrgUnitPGStore(beginFunc(func(context.Context) (pgx.Tx, error) {
		return &txStub{row: stubRow{vals: []any{"p1", "1001", "Name", "active"}}}, nil
	}))
	if _, err := store.FindPersonByPernr(ctx, "t1", "1001"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
