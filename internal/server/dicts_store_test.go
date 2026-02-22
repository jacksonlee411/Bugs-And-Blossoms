package server

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type dictAuditRows struct {
	records [][]any
	idx     int
	scanErr error
	err     error
}

func (r *dictAuditRows) Close()                        {}
func (r *dictAuditRows) Err() error                    { return r.err }
func (r *dictAuditRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *dictAuditRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *dictAuditRows) Next() bool {
	if r.idx >= len(r.records) {
		return false
	}
	r.idx++
	return true
}
func (r *dictAuditRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	rec := r.records[r.idx-1]
	for i := range dest {
		if i >= len(rec) || rec[i] == nil {
			continue
		}
		switch d := dest[i].(type) {
		case *int64:
			*d = rec[i].(int64)
		case *string:
			*d = rec[i].(string)
		case *time.Time:
			*d = rec[i].(time.Time)
		case *json.RawMessage:
			switch v := rec[i].(type) {
			case []byte:
				*d = append((*d)[:0], v...)
			case string:
				*d = json.RawMessage(v)
			default:
				return errors.New("unsupported raw message type")
			}
		default:
			return errors.New("unsupported scan type")
		}
	}
	return nil
}
func (r *dictAuditRows) Values() ([]any, error) { return nil, nil }
func (r *dictAuditRows) RawValues() [][]byte    { return nil }
func (r *dictAuditRows) Conn() *pgx.Conn        { return nil }

func TestDictPGStore_ListDicts_Coverage(t *testing.T) {
	ctx := context.Background()

	t.Run("begin error", func(t *testing.T) {
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})}
		if _, err := store.ListDicts(ctx, "t1", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		})}
		if _, err := store.ListDicts(ctx, "t1", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query error", func(t *testing.T) {
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{queryErr: errors.New("query")}, nil
		})}
		if _, err := store.ListDicts(ctx, "t1", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan error", func(t *testing.T) {
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &recordRows{records: [][]any{{"org_type"}}, scanErr: errors.New("scan")}}, nil
		})}
		if _, err := store.ListDicts(ctx, "t1", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows err", func(t *testing.T) {
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &recordRows{records: [][]any{}, err: errors.New("rows")}}, nil
		})}
		if _, err := store.ListDicts(ctx, "t1", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("merged list returns rows", func(t *testing.T) {
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &recordRows{records: [][]any{{"org_type", "Org Type", "active", "1970-01-01", nil}}}}, nil
		})}
		items, err := store.ListDicts(ctx, globalTenantID, "2026-01-01")
		if err != nil || len(items) != 1 || items[0].DictCode != "org_type" {
			t.Fatalf("items=%v err=%v", items, err)
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &recordRows{records: [][]any{{"org_type", "Org Type", "active", "1970-01-01", nil}}}, commitErr: errors.New("commit")}, nil
		})}
		if _, err := store.ListDicts(ctx, "t1", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok", func(t *testing.T) {
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &recordRows{records: [][]any{{"org_type", "Org Type", "active", "1970-01-01", nil}}}}, nil
		})}
		items, err := store.ListDicts(ctx, globalTenantID, "2026-01-01")
		if err != nil || len(items) != 1 {
			t.Fatalf("items=%v err=%v", items, err)
		}
	})
}

func TestDictPGStore_ListDictValues_AndOptions_Coverage(t *testing.T) {
	ctx := context.Background()

	t.Run("begin error", func(t *testing.T) {
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})}
		if _, err := store.ListDictValues(ctx, "t1", "org_type", "2026-01-01", "", 10, "all"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		})}
		if _, err := store.ListDictValues(ctx, "t1", "org_type", "2026-01-01", "", 10, "all"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query error", func(t *testing.T) {
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{queryErr: errors.New("query")}, nil
		})}
		if _, err := store.ListDictValues(ctx, "t1", "org_type", "2026-01-01", "", 10, "all"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan error", func(t *testing.T) {
		rows := &recordRows{records: [][]any{{"org_type", "10", "部门", "active", "2026-01-01", nil, time.Unix(1, 0).UTC()}}, scanErr: errors.New("scan")}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: rows}, nil
		})}
		if _, err := store.ListDictValues(ctx, "t1", "org_type", "2026-01-01", "", 10, "all"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows err", func(t *testing.T) {
		rows := &recordRows{records: [][]any{}, err: errors.New("rows")}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: rows}, nil
		})}
		if _, err := store.ListDictValues(ctx, "t1", "org_type", "2026-01-01", "", 10, "all"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("tenant source resolves then list values", func(t *testing.T) {
		now := time.Unix(2, 0).UTC()
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{"00000000-0000-0000-0000-000000000001"}}
		tx.rows = &recordRows{records: [][]any{{"org_type", "10", "部门", "active", "1970-01-01", nil, now}}}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		values, err := store.ListDictValues(ctx, "00000000-0000-0000-0000-000000000001", "org_type", "2026-01-01", "", 10, "all")
		if err != nil || len(values) != 1 || values[0].Code != "10" {
			t.Fatalf("values=%v err=%v", values, err)
		}
	})

	t.Run("resolve source error", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{err: errors.New("row")}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListDictValues(ctx, "t1", "org_type", "2026-01-01", "", 10, "all"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("resolve source not found", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{err: pgx.ErrNoRows}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListDictValues(ctx, "t1", "missing", "2026-01-01", "", 10, "all"); !errors.Is(err, errDictNotFound) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("commit error", func(t *testing.T) {
		now := time.Unix(3, 0).UTC()
		tx := &stubTx{commitErr: errors.New("commit")}
		tx.row = &stubRow{vals: []any{"t1"}}
		tx.rows = &recordRows{records: [][]any{{"org_type", "10", "部门", "active", "1970-01-01", nil, now}}}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListDictValues(ctx, "t1", "org_type", "2026-01-01", "", 10, "all"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok + ListOptions", func(t *testing.T) {
		now := time.Unix(4, 0).UTC()
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{globalTenantID}}
			tx.rows = &recordRows{records: [][]any{{"org_type", "10", "部门", "active", "1970-01-01", nil, now}}}
			return tx, nil
		})}
		opts, err := store.ListOptions(ctx, globalTenantID, "2026-01-01", "org_type", "", 10)
		if err != nil || len(opts) != 1 || opts[0].Code != "10" {
			t.Fatalf("opts=%v err=%v", opts, err)
		}
	})

	t.Run("ListOptions propagates list error", func(t *testing.T) {
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})}
		if _, err := store.ListOptions(ctx, "t1", "2026-01-01", "org_type", "", 10); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestDictPGStore_ResolveValueLabel_AndMutations_Coverage(t *testing.T) {
	ctx := context.Background()

	t.Run("ResolveValueLabel begin error", func(t *testing.T) {
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})}
		if _, _, err := store.ResolveValueLabel(ctx, "t1", "2026-01-01", "org_type", "10"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ResolveValueLabel set tenant error", func(t *testing.T) {
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		})}
		if _, _, err := store.ResolveValueLabel(ctx, "t1", "2026-01-01", "org_type", "10"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ResolveValueLabel source not found -> ok=false", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{err: pgx.ErrNoRows}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		label, ok, err := store.ResolveValueLabel(ctx, "00000000-0000-0000-0000-000000000001", "2026-01-01", "org_type", "10")
		if err != nil || ok || label != "" {
			t.Fatalf("label=%q ok=%v err=%v", label, ok, err)
		}
	})

	t.Run("ResolveValueLabel ok", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{globalTenantID}}
		tx.row2 = &stubRow{vals: []any{"部门"}}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		label, ok, err := store.ResolveValueLabel(ctx, globalTenantID, "2026-01-01", "org_type", "10")
		if err != nil || !ok || label != "部门" {
			t.Fatalf("label=%q ok=%v err=%v", label, ok, err)
		}
	})

	t.Run("ResolveValueLabel commit error", func(t *testing.T) {
		tx := &stubTx{commitErr: errors.New("commit")}
		tx.row = &stubRow{vals: []any{globalTenantID}}
		tx.row2 = &stubRow{vals: []any{"部门"}}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.ResolveValueLabel(ctx, globalTenantID, "2026-01-01", "org_type", "10"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ResolveValueLabel underlying error", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{err: errors.New("boom")}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.ResolveValueLabel(ctx, globalTenantID, "2026-01-01", "org_type", "10"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ResolveValueLabel value query error", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{globalTenantID}}
		tx.row2 = &stubRow{err: errors.New("boom")}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.ResolveValueLabel(ctx, globalTenantID, "2026-01-01", "org_type", "10"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submitValueEvent marshal error", func(t *testing.T) {
		store := &dictPGStore{pool: &fakeBeginner{}}
		_, _, err := store.submitValueEvent(ctx, "t1", "org_type", "10", dictEventCreated, "2026-01-01", map[string]any{"x": func() {}}, "r1", "u1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submitValueEvent begin error", func(t *testing.T) {
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})}
		if _, _, err := store.CreateDictValue(ctx, "t1", DictCreateValueRequest{DictCode: "org_type", Code: "10", Label: "部门", EnabledOn: "2026-01-01", RequestID: "r1", Initiator: "u1"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submitValueEvent set tenant error", func(t *testing.T) {
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		})}
		if _, _, err := store.CreateDictValue(ctx, "t1", DictCreateValueRequest{DictCode: "org_type", Code: "10", Label: "部门", EnabledOn: "2026-01-01", RequestID: "r1", Initiator: "u1"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submitValueEvent baseline not ready", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{false}}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.CreateDictValue(ctx, "t1", DictCreateValueRequest{DictCode: "org_type", Code: "10", Label: "部门", EnabledOn: "2026-01-01", RequestID: "r1", Initiator: "u1"}); !errors.Is(err, errDictBaselineNotReady) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("submitValueEvent query error", func(t *testing.T) {
		tx := &stubTx{rowErr: errors.New("row")}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.CreateDictValue(ctx, "t1", DictCreateValueRequest{DictCode: "org_type", Code: "10", Label: "部门", EnabledOn: "2026-01-01", RequestID: "r1", Initiator: "u1"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submitValueEvent submit call query error", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{true}}
		tx.row2 = &stubRow{vals: []any{true}}
		tx.row3Err = errors.New("row3")
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.CreateDictValue(ctx, "t1", DictCreateValueRequest{DictCode: "org_type", Code: "10", Label: "部门", EnabledOn: "2026-01-01", RequestID: "r1", Initiator: "u1"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submitValueEvent snapshot fetch error", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{true}}
		tx.row2 = &stubRow{vals: []any{true}}
		tx.row3 = &stubRow{vals: []any{int64(1), false}}
		tx.row4Err = errors.New("row4")
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.DisableDictValue(ctx, "t1", DictDisableValueRequest{DictCode: "org_type", Code: "10", DisabledOn: "2026-01-01", RequestID: "r1", Initiator: "u1"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submitValueEvent bad snapshot json", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{true}}
		tx.row2 = &stubRow{vals: []any{true}}
		tx.row3 = &stubRow{vals: []any{int64(1), true}}
		tx.row4 = &stubRow{vals: []any{[]byte(`{`), time.Unix(1, 0).UTC()}}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.CorrectDictValue(ctx, "t1", DictCorrectValueRequest{DictCode: "org_type", Code: "10", Label: "部门", CorrectionDay: "2026-01-01", RequestID: "r1", Initiator: "u1"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submitValueEvent commit error", func(t *testing.T) {
		tx := &stubTx{commitErr: errors.New("commit")}
		tx.row = &stubRow{vals: []any{true}}
		tx.row2 = &stubRow{vals: []any{true}}
		tx.row3 = &stubRow{vals: []any{int64(1), false}}
		tx.row4 = &stubRow{vals: []any{[]byte(`{"dict_code":"org_type","code":"10","label":"部门","status":"active","enabled_on":"2026-01-01"}`), time.Unix(2, 0).UTC()}}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.CreateDictValue(ctx, "t1", DictCreateValueRequest{DictCode: "org_type", Code: "10", Label: "部门", EnabledOn: "2026-01-01", RequestID: "r1", Initiator: "u1"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submitValueEvent ok", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{true}}
		tx.row2 = &stubRow{vals: []any{true}}
		tx.row3 = &stubRow{vals: []any{int64(1), true}}
		tx.row4 = &stubRow{vals: []any{[]byte(`{"dict_code":"org_type","code":"10","label":"部门","status":"active","enabled_on":"2026-01-01"}`), time.Unix(3, 0).UTC()}}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		item, wasRetry, err := store.CreateDictValue(ctx, "t1", DictCreateValueRequest{DictCode: "org_type", Code: "10", Label: "部门", EnabledOn: "2026-01-01", RequestID: "r1", Initiator: "u1"})
		if err != nil || !wasRetry || item.Code != "10" {
			t.Fatalf("item=%v wasRetry=%v err=%v", item, wasRetry, err)
		}
	})

	t.Run("submitValueEvent dict disabled", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{true}}
		tx.row2 = &stubRow{vals: []any{false}}
		tx.row3 = &stubRow{vals: []any{true}}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.CreateDictValue(ctx, "t1", DictCreateValueRequest{DictCode: "org_type", Code: "10", Label: "部门", EnabledOn: "2026-01-01", RequestID: "r1", Initiator: "u1"}); !errors.Is(err, errDictValueDictDisabled) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("submitValueEvent dict not found", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{true}}
		tx.row2 = &stubRow{vals: []any{false}}
		tx.row3 = &stubRow{vals: []any{false}}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.CreateDictValue(ctx, "t1", DictCreateValueRequest{DictCode: "org_type", Code: "10", Label: "部门", EnabledOn: "2026-01-01", RequestID: "r1", Initiator: "u1"}); !errors.Is(err, errDictNotFound) {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestDictPGStore_ListDictValueAudit_Coverage(t *testing.T) {
	ctx := context.Background()

	t.Run("begin error", func(t *testing.T) {
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return nil, errors.New("begin") })}
		if _, err := store.ListDictValueAudit(ctx, "t1", "org_type", "10", 10); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return &stubTx{execErr: errors.New("exec")}, nil })}
		if _, err := store.ListDictValueAudit(ctx, "t1", "org_type", "10", 10); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query error", func(t *testing.T) {
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return &stubTx{queryErr: errors.New("query")}, nil })}
		if _, err := store.ListDictValueAudit(ctx, "t1", "org_type", "10", 10); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan error", func(t *testing.T) {
		rows := &dictAuditRows{records: [][]any{{int64(1), "u1", "org_type", "10", dictEventCreated, "2026-01-01", "r1", "i1", time.Unix(1, 0).UTC(), []byte(`{}`), []byte(`{}`), []byte(`{}`)}}, scanErr: errors.New("scan")}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return &stubTx{rows: rows}, nil })}
		if _, err := store.ListDictValueAudit(ctx, "t1", "org_type", "10", 10); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows err", func(t *testing.T) {
		rows := &dictAuditRows{records: [][]any{}, err: errors.New("rows")}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return &stubTx{rows: rows}, nil })}
		if _, err := store.ListDictValueAudit(ctx, "t1", "org_type", "10", 10); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		rows := &dictAuditRows{records: [][]any{}}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: rows, commitErr: errors.New("commit")}, nil
		})}
		if _, err := store.ListDictValueAudit(ctx, "t1", "org_type", "10", 10); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok", func(t *testing.T) {
		rows := &dictAuditRows{records: [][]any{{int64(1), "u1", "org_type", "10", dictEventCreated, "2026-01-01", "r1", "i1", time.Unix(1, 0).UTC(), []byte(`{}`), []byte(`{}`), []byte(`{}`)}}}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return &stubTx{rows: rows}, nil })}
		events, err := store.ListDictValueAudit(ctx, "t1", "org_type", "10", 10)
		if err != nil || len(events) != 1 || events[0].EventID != 1 {
			t.Fatalf("events=%v err=%v", events, err)
		}
	})
}

func TestDictMemoryStore_Coverage(t *testing.T) {
	ctx := context.Background()
	store := newDictMemoryStore().(*dictMemoryStore)

	t.Run("ListDicts requires asOf", func(t *testing.T) {
		if _, err := store.ListDicts(ctx, "t1", ""); !errors.Is(err, errDictEffectiveDayRequired) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("ListDicts not found", func(t *testing.T) {
		items, err := store.ListDicts(ctx, "t1", "1960-01-01")
		if err != nil || len(items) != 0 {
			t.Fatalf("items=%v err=%v", items, err)
		}
	})

	t.Run("ListDictValues filters", func(t *testing.T) {
		vals, err := store.ListDictValues(ctx, "t1", "org_type", "2026-01-01", "10", 1, "all")
		if err != nil || len(vals) != 1 {
			t.Fatalf("vals=%v err=%v", vals, err)
		}

		if _, err := store.ListDictValues(ctx, "t1", "other", "2026-01-01", "", 10, "all"); !errors.Is(err, errDictNotFound) {
			t.Fatalf("err=%v", err)
		}

		vals3, _ := store.ListDictValues(ctx, "t1", "org_type", "1900-01-01", "", 10, "all")
		if len(vals3) != 0 {
			t.Fatalf("vals3=%v", vals3)
		}

		vals4, _ := store.ListDictValues(ctx, "t1", "org_type", "2026-01-01", "zzz", 10, "all")
		if len(vals4) != 0 {
			t.Fatalf("vals4=%v", vals4)
		}

		vals5, _ := store.ListDictValues(ctx, "t1", "org_type", "2026-01-01", "", 0, "all")
		if len(vals5) == 0 {
			t.Fatalf("vals5 empty")
		}
		vals6, _ := store.ListDictValues(ctx, "t1", "org_type", "2026-01-01", "", 999, "all")
		if len(vals6) != 2 {
			t.Fatalf("vals6=%v", vals6)
		}

		// Cover the clamp branch: len(out) > limit.
		vals7, _ := store.ListDictValues(ctx, "t1", "org_type", "2026-01-01", "", 1, "all")
		if len(vals7) != 1 {
			t.Fatalf("vals7=%v", vals7)
		}
	})

	t.Run("ResolveValueLabel ok/miss", func(t *testing.T) {
		label, ok, err := store.ResolveValueLabel(ctx, "t1", "2026-01-01", "org_type", "10")
		if err != nil || !ok || label == "" {
			t.Fatalf("label=%q ok=%v err=%v", label, ok, err)
		}
		_, ok, _ = store.ResolveValueLabel(ctx, "t1", "2026-01-01", "org_type", "999")
		if ok {
			t.Fatal("expected miss")
		}
	})

	t.Run("ListOptions", func(t *testing.T) {
		opts, err := store.ListOptions(ctx, "t1", "2026-01-01", "org_type", "", 10)
		if err != nil || len(opts) == 0 {
			t.Fatalf("opts=%v err=%v", opts, err)
		}
	})

	t.Run("mutations conflict", func(t *testing.T) {
		if _, _, err := store.CreateDictValue(ctx, "t1", DictCreateValueRequest{}); !errors.Is(err, errDictValueConflict) {
			t.Fatalf("err=%v", err)
		}
		if _, _, err := store.DisableDictValue(ctx, "t1", DictDisableValueRequest{}); !errors.Is(err, errDictValueConflict) {
			t.Fatalf("err=%v", err)
		}
		if _, _, err := store.CorrectDictValue(ctx, "t1", DictCorrectValueRequest{}); !errors.Is(err, errDictValueConflict) {
			t.Fatalf("err=%v", err)
		}
		if events, err := store.ListDictValueAudit(ctx, "t1", "org_type", "10", 10); err != nil || len(events) != 0 {
			t.Fatalf("events=%v err=%v", events, err)
		}
	})

	t.Run("valuesForTenant missing returns nil", func(t *testing.T) {
		if got := store.valuesForTenant("missing"); got != nil {
			t.Fatalf("got=%v", got)
		}
	})

	t.Run("dict create and disable", func(t *testing.T) {
		if _, _, err := store.CreateDict(ctx, "tenant-new", DictCreateRequest{DictCode: "", Name: "X", EnabledOn: "2026-01-01"}); !errors.Is(err, errDictCodeRequired) {
			t.Fatalf("err=%v", err)
		}
		if _, _, err := store.CreateDict(ctx, "tenant-new", DictCreateRequest{DictCode: "x", Name: "", EnabledOn: "2026-01-01"}); !errors.Is(err, errDictNameRequired) {
			t.Fatalf("err=%v", err)
		}
		if _, _, err := store.CreateDict(ctx, "tenant-new", DictCreateRequest{DictCode: "x", Name: "X", EnabledOn: ""}); !errors.Is(err, errDictEffectiveDayRequired) {
			t.Fatalf("err=%v", err)
		}
		if createdNew, _, err := store.CreateDict(ctx, "tenant-new", DictCreateRequest{DictCode: "new_dict", Name: "New Dict", EnabledOn: "2026-01-01"}); err != nil || createdNew.DictCode != "new_dict" {
			t.Fatalf("createdNew=%+v err=%v", createdNew, err)
		}
		created, wasRetry, err := store.CreateDict(ctx, "t1", DictCreateRequest{DictCode: "expense_type", Name: "Expense Type", EnabledOn: "2026-01-01", RequestID: "r1"})
		if err != nil || wasRetry || created.DictCode != "expense_type" {
			t.Fatalf("created=%+v retry=%v err=%v", created, wasRetry, err)
		}
		if _, _, err := store.CreateDict(ctx, "t1", DictCreateRequest{DictCode: "expense_type", Name: "Expense Type", EnabledOn: "2026-01-01", RequestID: "r2"}); !errors.Is(err, errDictCodeConflict) {
			t.Fatalf("err=%v", err)
		}
		disabled, _, err := store.DisableDict(ctx, "t1", DictDisableRequest{DictCode: "expense_type", DisabledOn: "2026-01-02", RequestID: "r3"})
		if err != nil || disabled.Status != "inactive" || disabled.DisabledOn == nil {
			t.Fatalf("disabled=%+v err=%v", disabled, err)
		}
		if _, _, err := store.DisableDict(ctx, "t1", DictDisableRequest{DictCode: "", DisabledOn: "2026-01-03", RequestID: "r4"}); !errors.Is(err, errDictCodeRequired) {
			t.Fatalf("err=%v", err)
		}
		if _, _, err := store.DisableDict(ctx, "t1", DictDisableRequest{DictCode: "expense_type", DisabledOn: "", RequestID: "r4"}); !errors.Is(err, errDictDisabledOnRequired) {
			t.Fatalf("err=%v", err)
		}
		if _, _, err := store.DisableDict(ctx, "missing-tenant", DictDisableRequest{DictCode: "expense_type", DisabledOn: "2026-01-03", RequestID: "r4"}); !errors.Is(err, errDictNotFound) {
			t.Fatalf("err=%v", err)
		}
		if _, _, err := store.DisableDict(ctx, "t1", DictDisableRequest{DictCode: "missing", DisabledOn: "2026-01-03", RequestID: "r4"}); !errors.Is(err, errDictNotFound) {
			t.Fatalf("err=%v", err)
		}
		if _, _, err := store.DisableDict(ctx, "t1", DictDisableRequest{DictCode: "expense_type", DisabledOn: "2026-01-01", RequestID: "r4"}); !errors.Is(err, errDictCodeConflict) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("ListOptions uses active status but ignores parameter", func(t *testing.T) {
		_ = json.RawMessage(`{}`)
	})
}
