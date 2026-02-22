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

type recordRows struct {
	records [][]any
	idx     int
	scanErr error
	err     error
}

func (r *recordRows) Close()                        {}
func (r *recordRows) Err() error                    { return r.err }
func (r *recordRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *recordRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *recordRows) Next() bool {
	if r.idx >= len(r.records) {
		return false
	}
	r.idx++
	return true
}
func (r *recordRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	rec := r.records[r.idx-1]
	for i := range dest {
		if i >= len(rec) || rec[i] == nil {
			continue
		}
		switch d := dest[i].(type) {
		case *int:
			*d = rec[i].(int)
		case *int64:
			*d = rec[i].(int64)
		case *string:
			*d = rec[i].(string)
		case *bool:
			*d = rec[i].(bool)
		case *time.Time:
			*d = rec[i].(time.Time)
		case *[]int:
			*d = append([]int(nil), rec[i].([]int)...)
		case *[]byte:
			*d = append([]byte(nil), rec[i].([]byte)...)
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
func (r *recordRows) Values() ([]any, error) { return nil, nil }
func (r *recordRows) RawValues() [][]byte    { return nil }
func (r *recordRows) Conn() *pgx.Conn        { return nil }

func TestOrgUnitPGStore_ListNodesCurrentWithVisibility_Coverage(t *testing.T) {
	ctx := context.Background()

	t.Run("includeDisabled=false delegates", func(t *testing.T) {
		store := &orgUnitPGStore{pool: &fakeBeginner{}}
		if _, err := store.ListNodesCurrentWithVisibility(ctx, "t1", "2026-01-01", false); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("begin error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})}
		if _, err := store.ListNodesCurrentWithVisibility(ctx, "t1", "2026-01-01", true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		})}
		if _, err := store.ListNodesCurrentWithVisibility(ctx, "t1", "2026-01-01", true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{queryErr: errors.New("query")}, nil
		})}
		if _, err := store.ListNodesCurrentWithVisibility(ctx, "t1", "2026-01-01", true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &recordRows{records: [][]any{{"10000001", "A001", "Root", "active", true, true, time.Unix(1, 0).UTC()}}, scanErr: errors.New("scan")}}, nil
		})}
		if _, err := store.ListNodesCurrentWithVisibility(ctx, "t1", "2026-01-01", true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows err", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &recordRows{records: [][]any{}, err: errors.New("rows")}}, nil
		})}
		if _, err := store.ListNodesCurrentWithVisibility(ctx, "t1", "2026-01-01", true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{
				rows:      &recordRows{records: [][]any{{"10000001", "A001", "Root", "active", true, true, time.Unix(1, 0).UTC()}}},
				commitErr: errors.New("commit"),
			}, nil
		})}
		if _, err := store.ListNodesCurrentWithVisibility(ctx, "t1", "2026-01-01", true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &recordRows{records: [][]any{{"10000001", "A001", "Root", "disabled", false, false, time.Unix(1, 0).UTC()}}}}, nil
		})}
		nodes, err := store.ListNodesCurrentWithVisibility(ctx, "t1", "2026-01-01", true)
		if err != nil || len(nodes) != 1 || nodes[0].Status != "disabled" {
			t.Fatalf("nodes=%v err=%v", nodes, err)
		}
	})
}

func TestOrgUnitPGStore_ListChildren_AndVisibility_Coverage(t *testing.T) {
	ctx := context.Background()

	t.Run("ListChildren begin error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})}
		if _, err := store.ListChildren(ctx, "t1", 1, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ListChildren set tenant error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		})}
		if _, err := store.ListChildren(ctx, "t1", 1, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ListChildren exists scan error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("row")}, nil
		})}
		if _, err := store.ListChildren(ctx, "t1", 1, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ListChildren not found", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{false}}
			return tx, nil
		})}
		if _, err := store.ListChildren(ctx, "t1", 1, "2026-01-01"); !errors.Is(err, errOrgUnitNotFound) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("ListChildren query error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{queryErr: errors.New("query")}
			tx.row = &stubRow{vals: []any{true}}
			return tx, nil
		})}
		if _, err := store.ListChildren(ctx, "t1", 1, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ListChildren scan error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{rows: &recordRows{records: [][]any{{1, "A001", "Child", true, true}}, scanErr: errors.New("scan")}}
			tx.row = &stubRow{vals: []any{true}}
			return tx, nil
		})}
		if _, err := store.ListChildren(ctx, "t1", 1, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ListChildren rows err", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{rows: &recordRows{records: [][]any{}, err: errors.New("rows")}}
			tx.row = &stubRow{vals: []any{true}}
			return tx, nil
		})}
		if _, err := store.ListChildren(ctx, "t1", 1, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ListChildren commit error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{rows: &recordRows{records: [][]any{{1, "A001", "Child", true, false}}}, commitErr: errors.New("commit")}
			tx.row = &stubRow{vals: []any{true}}
			return tx, nil
		})}
		if _, err := store.ListChildren(ctx, "t1", 1, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ListChildren ok", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{rows: &recordRows{records: [][]any{{1, "A001", "Child", false, true}}}}
			tx.row = &stubRow{vals: []any{true}}
			return tx, nil
		})}
		children, err := store.ListChildren(ctx, "t1", 1, "2026-01-01")
		if err != nil || len(children) != 1 || children[0].Status != "active" {
			t.Fatalf("children=%v err=%v", children, err)
		}
	})

	t.Run("ListChildrenWithVisibility includeDisabled=false delegates", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{rows: &recordRows{records: [][]any{}}}
			tx.row = &stubRow{vals: []any{true}}
			return tx, nil
		})}
		if _, err := store.ListChildrenWithVisibility(ctx, "t1", 1, "2026-01-01", false); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("ListChildrenWithVisibility begin error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})}
		if _, err := store.ListChildrenWithVisibility(ctx, "t1", 1, "2026-01-01", true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ListChildrenWithVisibility set tenant error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		})}
		if _, err := store.ListChildrenWithVisibility(ctx, "t1", 1, "2026-01-01", true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ListChildrenWithVisibility exists scan error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("row")}, nil
		})}
		if _, err := store.ListChildrenWithVisibility(ctx, "t1", 1, "2026-01-01", true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ListChildrenWithVisibility exists not found", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{false}}
			return tx, nil
		})}
		if _, err := store.ListChildrenWithVisibility(ctx, "t1", 1, "2026-01-01", true); !errors.Is(err, errOrgUnitNotFound) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("ListChildrenWithVisibility query error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{queryErr: errors.New("query")}
			tx.row = &stubRow{vals: []any{true}}
			return tx, nil
		})}
		if _, err := store.ListChildrenWithVisibility(ctx, "t1", 1, "2026-01-01", true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ListChildrenWithVisibility scan error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{rows: &recordRows{records: [][]any{{1, "A001", "Child", "disabled", false, true}}, scanErr: errors.New("scan")}}
			tx.row = &stubRow{vals: []any{true}}
			return tx, nil
		})}
		if _, err := store.ListChildrenWithVisibility(ctx, "t1", 1, "2026-01-01", true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ListChildrenWithVisibility rows err", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{rows: &recordRows{records: [][]any{}, err: errors.New("rows")}}
			tx.row = &stubRow{vals: []any{true}}
			return tx, nil
		})}
		if _, err := store.ListChildrenWithVisibility(ctx, "t1", 1, "2026-01-01", true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ListChildrenWithVisibility commit error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{rows: &recordRows{records: [][]any{{1, "A001", "Child", "active", true, false}}}, commitErr: errors.New("commit")}
			tx.row = &stubRow{vals: []any{true}}
			return tx, nil
		})}
		if _, err := store.ListChildrenWithVisibility(ctx, "t1", 1, "2026-01-01", true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ListChildrenWithVisibility ok", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{rows: &recordRows{records: [][]any{{1, "A001", "Child", "disabled", false, true}}}}
			tx.row = &stubRow{vals: []any{true}}
			return tx, nil
		})}
		children, err := store.ListChildrenWithVisibility(ctx, "t1", 1, "2026-01-01", true)
		if err != nil || len(children) != 1 || children[0].Status != "disabled" {
			t.Fatalf("children=%v err=%v", children, err)
		}
	})
}

func TestOrgUnitPGStore_GetNodeDetails_AndVisibility_Coverage(t *testing.T) {
	ctx := context.Background()

	t.Run("GetNodeDetails begin error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return nil, errors.New("begin") })}
		if _, err := store.GetNodeDetails(ctx, "t1", 1, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("GetNodeDetails set tenant error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return &stubTx{execErr: errors.New("exec")}, nil })}
		if _, err := store.GetNodeDetails(ctx, "t1", 1, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("GetNodeDetails no rows", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return &stubTx{rowErr: pgx.ErrNoRows}, nil })}
		if _, err := store.GetNodeDetails(ctx, "t1", 1, "2026-01-01"); !errors.Is(err, errOrgUnitNotFound) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("GetNodeDetails row error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return &stubTx{rowErr: errors.New("row")}, nil })}
		if _, err := store.GetNodeDetails(ctx, "t1", 1, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("GetNodeDetails commit error", func(t *testing.T) {
		tx := &stubTx{commitErr: errors.New("commit")}
		tx.row = &stubRow{vals: []any{
			1,
			"A001",
			"Root",
			"active",
			0,
			"",
			"",
			true,
			"1",
			"Mgr",
			[]int{1},
			"Root",
			time.Unix(1, 0).UTC(),
			time.Unix(2, 0).UTC(),
			"e1",
		}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.GetNodeDetails(ctx, "t1", 1, "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("GetNodeDetails ok", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{
			1,
			"A001",
			"Root",
			"active",
			0,
			"",
			"",
			true,
			"",
			"",
			[]int{1},
			"Root",
			time.Unix(1, 0).UTC(),
			time.Unix(2, 0).UTC(),
			"e1",
		}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		d, err := store.GetNodeDetails(ctx, "t1", 1, "2026-01-01")
		if err != nil || d.OrgCode != "A001" || len(d.PathIDs) != 1 || d.PathIDs[0] != 1 {
			t.Fatalf("details=%+v err=%v", d, err)
		}
	})

	t.Run("GetNodeDetailsWithVisibility includeDisabled=false delegates", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{
			1,
			"A001",
			"Root",
			"active",
			0,
			"",
			"",
			true,
			"",
			"",
			[]int{1},
			"Root",
			time.Unix(1, 0).UTC(),
			time.Unix(2, 0).UTC(),
			"e1",
		}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.GetNodeDetailsWithVisibility(ctx, "t1", 1, "2026-01-01", false); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("GetNodeDetailsWithVisibility ok", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{
			1,
			"A001",
			"Root",
			"disabled",
			0,
			"",
			"",
			true,
			"",
			"",
			[]int{1},
			"Root",
			time.Unix(1, 0).UTC(),
			time.Unix(2, 0).UTC(),
			"e1",
		}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		d, err := store.GetNodeDetailsWithVisibility(ctx, "t1", 1, "2026-01-01", true)
		if err != nil || d.Status != "disabled" {
			t.Fatalf("details=%+v err=%v", d, err)
		}
	})

	t.Run("GetNodeDetailsWithVisibility begin error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return nil, errors.New("begin") })}
		if _, err := store.GetNodeDetailsWithVisibility(ctx, "t1", 1, "2026-01-01", true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("GetNodeDetailsWithVisibility set tenant error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return &stubTx{execErr: errors.New("exec")}, nil })}
		if _, err := store.GetNodeDetailsWithVisibility(ctx, "t1", 1, "2026-01-01", true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("GetNodeDetailsWithVisibility no rows", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return &stubTx{rowErr: pgx.ErrNoRows}, nil })}
		if _, err := store.GetNodeDetailsWithVisibility(ctx, "t1", 1, "2026-01-01", true); !errors.Is(err, errOrgUnitNotFound) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("GetNodeDetailsWithVisibility row error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return &stubTx{rowErr: errors.New("row")}, nil })}
		if _, err := store.GetNodeDetailsWithVisibility(ctx, "t1", 1, "2026-01-01", true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("GetNodeDetailsWithVisibility commit error", func(t *testing.T) {
		tx := &stubTx{commitErr: errors.New("commit")}
		tx.row = &stubRow{vals: []any{
			1,
			"A001",
			"Root",
			"active",
			0,
			"",
			"",
			true,
			"",
			"",
			[]int{1},
			"Root",
			time.Unix(1, 0).UTC(),
			time.Unix(2, 0).UTC(),
			"e1",
		}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.GetNodeDetailsWithVisibility(ctx, "t1", 1, "2026-01-01", true); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestOrgUnitPGStore_SearchNode_AndCandidates_AndVersions_Coverage(t *testing.T) {
	ctx := context.Background()

	t.Run("SearchNode empty query", func(t *testing.T) {
		store := &orgUnitPGStore{pool: &fakeBeginner{}}
		if _, err := store.SearchNode(ctx, "t1", " ", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNode begin error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return nil, errors.New("begin") })}
		if _, err := store.SearchNode(ctx, "t1", "A001", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNode set tenant error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return &stubTx{execErr: errors.New("exec")}, nil })}
		if _, err := store.SearchNode(ctx, "t1", "A001", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNode normalized query error (non-no-rows)", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("row")}, nil
		})}
		if _, err := store.SearchNode(ctx, "t1", "A001", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNode falls back to name not found", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{err: pgx.ErrNoRows}  // normalized lookup
		tx.row2 = &stubRow{err: pgx.ErrNoRows} // name lookup
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.SearchNode(ctx, "t1", "A001", "2026-01-01"); !errors.Is(err, errOrgUnitNotFound) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("SearchNode normalized found", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{10000001, "A001", "Root", []int{10000001}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		got, err := store.SearchNode(ctx, "t1", "A001", "2026-01-01")
		if err != nil || got.TargetOrgID != 10000001 || got.TreeAsOf != "2026-01-01" {
			t.Fatalf("got=%+v err=%v", got, err)
		}
	})

	t.Run("SearchNode found by name", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{err: pgx.ErrNoRows}
		tx.row2 = &stubRow{vals: []any{10000002, "A002", "Other", []int{10000002}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		got, err := store.SearchNode(ctx, "t1", "Other", "2026-01-01")
		if err != nil || got.TargetOrgID != 10000002 {
			t.Fatalf("got=%+v err=%v", got, err)
		}
	})

	t.Run("SearchNode normalize fails uses name branch", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{10000003, "A003", "Name", []int{10000003}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.SearchNode(ctx, "t1", "bad\x7f", "2026-01-01"); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("SearchNode name branch error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return &stubTx{rowErr: errors.New("row")}, nil })}
		if _, err := store.SearchNode(ctx, "t1", "bad\x7f", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNode commit error", func(t *testing.T) {
		tx := &stubTx{commitErr: errors.New("commit")}
		tx.row = &stubRow{vals: []any{10000001, "A001", "Root", []int{10000001}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.SearchNode(ctx, "t1", "A001", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeWithVisibility includeDisabled=false delegates", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{10000001, "A001", "Root", []int{10000001}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.SearchNodeWithVisibility(ctx, "t1", "A001", "2026-01-01", false); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("SearchNodeWithVisibility empty query", func(t *testing.T) {
		store := &orgUnitPGStore{pool: &fakeBeginner{}}
		if _, err := store.SearchNodeWithVisibility(ctx, "t1", " ", "2026-01-01", true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeWithVisibility begin error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return nil, errors.New("begin") })}
		if _, err := store.SearchNodeWithVisibility(ctx, "t1", "A001", "2026-01-01", true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeWithVisibility set tenant error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return &stubTx{execErr: errors.New("exec")}, nil })}
		if _, err := store.SearchNodeWithVisibility(ctx, "t1", "A001", "2026-01-01", true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeWithVisibility normalize fails uses name branch", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{10000003, "A003", "Name", []int{10000003}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.SearchNodeWithVisibility(ctx, "t1", "bad\x7f", "2026-01-01", true); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("SearchNodeWithVisibility name branch error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return &stubTx{rowErr: errors.New("row")}, nil })}
		if _, err := store.SearchNodeWithVisibility(ctx, "t1", "bad\x7f", "2026-01-01", true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeWithVisibility found by name", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{err: pgx.ErrNoRows} // normalized
		tx.row2 = &stubRow{vals: []any{10000002, "A002", "Other", []int{10000002}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		got, err := store.SearchNodeWithVisibility(ctx, "t1", "Other", "2026-01-01", true)
		if err != nil || got.TargetOrgCode != "A002" {
			t.Fatalf("got=%+v err=%v", got, err)
		}
	})

	t.Run("SearchNodeWithVisibility not found", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{err: pgx.ErrNoRows}
		tx.row2 = &stubRow{err: pgx.ErrNoRows}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.SearchNodeWithVisibility(ctx, "t1", "Other", "2026-01-01", true); !errors.Is(err, errOrgUnitNotFound) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("SearchNodeWithVisibility normalized query error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return &stubTx{rowErr: errors.New("row")}, nil })}
		if _, err := store.SearchNodeWithVisibility(ctx, "t1", "A001", "2026-01-01", true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeWithVisibility commit error", func(t *testing.T) {
		tx := &stubTx{commitErr: errors.New("commit")}
		tx.row = &stubRow{vals: []any{10000001, "A001", "Root", []int{10000001}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.SearchNodeWithVisibility(ctx, "t1", "A001", "2026-01-01", true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeCandidates empty query", func(t *testing.T) {
		store := &orgUnitPGStore{pool: &fakeBeginner{}}
		if _, err := store.SearchNodeCandidates(ctx, "t1", "", "2026-01-01", 0); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeCandidates begin error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return nil, errors.New("begin") })}
		if _, err := store.SearchNodeCandidates(ctx, "t1", "A001", "2026-01-01", 0); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeCandidates set tenant error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return &stubTx{execErr: errors.New("exec")}, nil })}
		if _, err := store.SearchNodeCandidates(ctx, "t1", "A001", "2026-01-01", 0); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeCandidates normalized query error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return &stubTx{queryErr: errors.New("query")}, nil })}
		if _, err := store.SearchNodeCandidates(ctx, "t1", "A001", "2026-01-01", 1); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeCandidates limit defaults", func(t *testing.T) {
		tx := &stubTx{rows: &recordRows{records: [][]any{{1, "A001", "Root"}}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.SearchNodeCandidates(ctx, "t1", "A001", "2026-01-01", 0); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("SearchNodeCandidates normalize fails uses name branch", func(t *testing.T) {
		tx := &stubTx{rows: &recordRows{records: [][]any{{2, "A002", "Other"}}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.SearchNodeCandidates(ctx, "t1", "bad\x7f", "2026-01-01", 1); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("SearchNodeCandidates name branch scan error", func(t *testing.T) {
		tx := &stubTx{rows: &recordRows{records: [][]any{{2, "A002", "Other"}}, scanErr: errors.New("scan")}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.SearchNodeCandidates(ctx, "t1", "bad\x7f", "2026-01-01", 1); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeCandidates name branch rows err", func(t *testing.T) {
		tx := &stubTx{rows: &recordRows{records: [][]any{}, err: errors.New("rows")}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.SearchNodeCandidates(ctx, "t1", "bad\x7f", "2026-01-01", 1); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeCandidates name branch commit error", func(t *testing.T) {
		tx := &stubTx{rows: &recordRows{records: [][]any{{2, "A002", "Other"}}}, commitErr: errors.New("commit")}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.SearchNodeCandidates(ctx, "t1", "bad\x7f", "2026-01-01", 1); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeCandidates normalized hit returns early", func(t *testing.T) {
		tx := &stubTx{rows: &recordRows{records: [][]any{{1, "A001", "Root"}}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		out, err := store.SearchNodeCandidates(ctx, "t1", "A001", "2026-01-01", 1)
		if err != nil || len(out) != 1 || out[0].Status != "active" {
			t.Fatalf("out=%v err=%v", out, err)
		}
	})

	t.Run("SearchNodeCandidates normalized empty falls back", func(t *testing.T) {
		tx := &stubTx{
			rows:  &recordRows{records: [][]any{}},
			rows2: &recordRows{records: [][]any{{2, "A002", "Other"}}},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		out, err := store.SearchNodeCandidates(ctx, "t1", "A001", "2026-01-01", 2)
		if err != nil || len(out) != 1 || out[0].OrgID != 2 {
			t.Fatalf("out=%v err=%v", out, err)
		}
	})

	t.Run("SearchNodeCandidates fallback query error", func(t *testing.T) {
		tx := &stubTx{
			rows:       &recordRows{records: [][]any{}},
			queryErr:   errors.New("query2"),
			queryErrAt: 2,
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.SearchNodeCandidates(ctx, "t1", "A001", "2026-01-01", 2); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeCandidates scan error", func(t *testing.T) {
		tx := &stubTx{rows: &recordRows{records: [][]any{{1, "A001", "Root"}}, scanErr: errors.New("scan")}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.SearchNodeCandidates(ctx, "t1", "A001", "2026-01-01", 1); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeCandidates rows err", func(t *testing.T) {
		tx := &stubTx{rows: &recordRows{records: [][]any{}, err: errors.New("rows")}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.SearchNodeCandidates(ctx, "t1", "A001", "2026-01-01", 1); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeCandidates commit error", func(t *testing.T) {
		tx := &stubTx{rows: &recordRows{records: [][]any{{1, "A001", "Root"}}}, commitErr: errors.New("commit")}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.SearchNodeCandidates(ctx, "t1", "A001", "2026-01-01", 1); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeCandidates fallback not found", func(t *testing.T) {
		tx := &stubTx{rows: &recordRows{records: [][]any{}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.SearchNodeCandidates(ctx, "t1", "Root", "2026-01-01", 1); !errors.Is(err, errOrgUnitNotFound) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("SearchNodeCandidatesWithVisibility includeDisabled=false delegates", func(t *testing.T) {
		tx := &stubTx{rows: &recordRows{records: [][]any{{1, "A001", "Root"}}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.SearchNodeCandidatesWithVisibility(ctx, "t1", "A001", "2026-01-01", 1, false); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("SearchNodeCandidatesWithVisibility empty query", func(t *testing.T) {
		store := &orgUnitPGStore{pool: &fakeBeginner{}}
		if _, err := store.SearchNodeCandidatesWithVisibility(ctx, "t1", "", "2026-01-01", 0, true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeCandidatesWithVisibility begin error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return nil, errors.New("begin") })}
		if _, err := store.SearchNodeCandidatesWithVisibility(ctx, "t1", "A001", "2026-01-01", 1, true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeCandidatesWithVisibility set tenant error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return &stubTx{execErr: errors.New("exec")}, nil })}
		if _, err := store.SearchNodeCandidatesWithVisibility(ctx, "t1", "A001", "2026-01-01", 1, true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeCandidatesWithVisibility query error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return &stubTx{queryErr: errors.New("query")}, nil })}
		if _, err := store.SearchNodeCandidatesWithVisibility(ctx, "t1", "A001", "2026-01-01", 1, true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeCandidatesWithVisibility limit defaults", func(t *testing.T) {
		tx := &stubTx{rows: &recordRows{records: [][]any{{1, "A001", "Root", "active"}}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.SearchNodeCandidatesWithVisibility(ctx, "t1", "A001", "2026-01-01", 0, true); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("SearchNodeCandidatesWithVisibility normalize fails uses name branch", func(t *testing.T) {
		tx := &stubTx{rows: &recordRows{records: [][]any{{2, "A002", "Other", "active"}}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.SearchNodeCandidatesWithVisibility(ctx, "t1", "bad\x7f", "2026-01-01", 1, true); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("SearchNodeCandidatesWithVisibility normalized empty falls back", func(t *testing.T) {
		tx := &stubTx{
			rows:  &recordRows{records: [][]any{}},
			rows2: &recordRows{records: [][]any{{2, "A002", "Other", "active"}}},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		out, err := store.SearchNodeCandidatesWithVisibility(ctx, "t1", "A001", "2026-01-01", 2, true)
		if err != nil || len(out) != 1 || out[0].OrgID != 2 {
			t.Fatalf("out=%v err=%v", out, err)
		}
	})

	t.Run("SearchNodeCandidatesWithVisibility fallback scan error", func(t *testing.T) {
		tx := &stubTx{
			rows:  &recordRows{records: [][]any{}},
			rows2: &recordRows{records: [][]any{{2, "A002", "Other", "active"}}, scanErr: errors.New("scan")},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.SearchNodeCandidatesWithVisibility(ctx, "t1", "A001", "2026-01-01", 2, true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeCandidatesWithVisibility fallback rows err", func(t *testing.T) {
		tx := &stubTx{
			rows:  &recordRows{records: [][]any{}},
			rows2: &recordRows{records: [][]any{}, err: errors.New("rows")},
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.SearchNodeCandidatesWithVisibility(ctx, "t1", "A001", "2026-01-01", 2, true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeCandidatesWithVisibility fallback commit error", func(t *testing.T) {
		tx := &stubTx{
			rows:      &recordRows{records: [][]any{}},
			rows2:     &recordRows{records: [][]any{{2, "A002", "Other", "active"}}},
			commitErr: errors.New("commit"),
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.SearchNodeCandidatesWithVisibility(ctx, "t1", "A001", "2026-01-01", 2, true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeCandidatesWithVisibility fallback query error", func(t *testing.T) {
		tx := &stubTx{
			rows:       &recordRows{records: [][]any{}},
			queryErr:   errors.New("query2"),
			queryErrAt: 2,
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.SearchNodeCandidatesWithVisibility(ctx, "t1", "A001", "2026-01-01", 2, true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeCandidatesWithVisibility not found", func(t *testing.T) {
		tx := &stubTx{rows: &recordRows{records: [][]any{}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.SearchNodeCandidatesWithVisibility(ctx, "t1", "Other", "2026-01-01", 1, true); !errors.Is(err, errOrgUnitNotFound) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("SearchNodeCandidatesWithVisibility scan error", func(t *testing.T) {
		tx := &stubTx{rows: &recordRows{records: [][]any{{1, "A001", "Root", "active"}}, scanErr: errors.New("scan")}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.SearchNodeCandidatesWithVisibility(ctx, "t1", "A001", "2026-01-01", 1, true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeCandidatesWithVisibility rows err", func(t *testing.T) {
		tx := &stubTx{rows: &recordRows{records: [][]any{}, err: errors.New("rows")}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.SearchNodeCandidatesWithVisibility(ctx, "t1", "A001", "2026-01-01", 1, true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeCandidatesWithVisibility commit error", func(t *testing.T) {
		tx := &stubTx{rows: &recordRows{records: [][]any{{1, "A001", "Root", "active"}}}, commitErr: errors.New("commit")}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.SearchNodeCandidatesWithVisibility(ctx, "t1", "A001", "2026-01-01", 1, true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("SearchNodeCandidatesWithVisibility ok", func(t *testing.T) {
		tx := &stubTx{rows: &recordRows{records: [][]any{{1, "A001", "Root", "disabled"}}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		out, err := store.SearchNodeCandidatesWithVisibility(ctx, "t1", "A001", "2026-01-01", 1, true)
		if err != nil || len(out) != 1 || out[0].Status != "disabled" {
			t.Fatalf("out=%v err=%v", out, err)
		}
	})

	t.Run("ListNodeVersions begin error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return nil, errors.New("begin") })}
		if _, err := store.ListNodeVersions(ctx, "t1", 1); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ListNodeVersions set tenant error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return &stubTx{execErr: errors.New("exec")}, nil })}
		if _, err := store.ListNodeVersions(ctx, "t1", 1); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ListNodeVersions query error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return &stubTx{queryErr: errors.New("query")}, nil })}
		if _, err := store.ListNodeVersions(ctx, "t1", 1); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ListNodeVersions scan error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &recordRows{records: [][]any{{int64(1), "e1", time.Unix(1, 0).UTC(), "RENAME"}}, scanErr: errors.New("scan")}}, nil
		})}
		if _, err := store.ListNodeVersions(ctx, "t1", 1); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ListNodeVersions rows err", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &recordRows{records: [][]any{}, err: errors.New("rows")}}, nil
		})}
		if _, err := store.ListNodeVersions(ctx, "t1", 1); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ListNodeVersions commit error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{
				rows:      &recordRows{records: [][]any{{int64(1), "e1", time.Unix(1, 0).UTC(), "RENAME"}}},
				commitErr: errors.New("commit"),
			}, nil
		})}
		if _, err := store.ListNodeVersions(ctx, "t1", 1); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ListNodeVersions ok", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &recordRows{records: [][]any{{int64(1), "e1", time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), "RENAME"}}}}, nil
		})}
		out, err := store.ListNodeVersions(ctx, "t1", 1)
		if err != nil || len(out) != 1 || out[0].EffectiveDate != "2026-01-02" {
			t.Fatalf("out=%v err=%v", out, err)
		}
	})
}
