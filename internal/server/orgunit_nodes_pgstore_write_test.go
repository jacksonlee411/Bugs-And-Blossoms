package server

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

func TestOrgUnitPGStore_ResolveSetID(t *testing.T) {
	ctx := context.Background()

	t.Run("begin error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})).(*orgUnitPGStore)
		if _, err := store.ResolveSetID(ctx, "t1", "10000001", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		})).(*orgUnitPGStore)
		if _, err := store.ResolveSetID(ctx, "t1", "10000001", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid org id", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		})).(*orgUnitPGStore)
		if _, err := store.ResolveSetID(ctx, "t1", "nope", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("resolve error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("resolve")}, nil
		})).(*orgUnitPGStore)
		if _, err := store.ResolveSetID(ctx, "t1", "10000001", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{commitErr: errors.New("commit")}
			tx.row = &stubRow{vals: []any{"S2601"}}
			return tx, nil
		})).(*orgUnitPGStore)
		if _, err := store.ResolveSetID(ctx, "t1", "10000001", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"S2601"}}
			return tx, nil
		})).(*orgUnitPGStore)
		got, err := store.ResolveSetID(ctx, "t1", "10000001", "2026-01-01")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if got != "S2601" {
			t.Fatalf("expected S2601, got %q", got)
		}
	})
}

func TestOrgUnitPGStore_ResolveOrgID(t *testing.T) {
	ctx := context.Background()

	t.Run("begin error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})).(*orgUnitPGStore)
		if _, err := store.ResolveOrgID(ctx, "t1", "A1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		})).(*orgUnitPGStore)
		if _, err := store.ResolveOrgID(ctx, "t1", "A1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("resolve error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("resolve")}, nil
		})).(*orgUnitPGStore)
		if _, err := store.ResolveOrgID(ctx, "t1", "A1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{commitErr: errors.New("commit")}
			tx.row = &stubRow{vals: []any{10000001}}
			return tx, nil
		})).(*orgUnitPGStore)
		if _, err := store.ResolveOrgID(ctx, "t1", "A1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{10000001}}
			return tx, nil
		})).(*orgUnitPGStore)
		got, err := store.ResolveOrgID(ctx, "t1", "A1")
		if err != nil || got != 10000001 {
			t.Fatalf("got=%d err=%v", got, err)
		}
	})
}

func TestOrgUnitPGStore_ResolveOrgCode(t *testing.T) {
	ctx := context.Background()

	t.Run("begin error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})).(*orgUnitPGStore)
		if _, err := store.ResolveOrgCode(ctx, "t1", 10000001); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		})).(*orgUnitPGStore)
		if _, err := store.ResolveOrgCode(ctx, "t1", 10000001); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("resolve error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("resolve")}, nil
		})).(*orgUnitPGStore)
		if _, err := store.ResolveOrgCode(ctx, "t1", 10000001); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{commitErr: errors.New("commit")}
			tx.row = &stubRow{vals: []any{"A1"}}
			return tx, nil
		})).(*orgUnitPGStore)
		if _, err := store.ResolveOrgCode(ctx, "t1", 10000001); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"A1"}}
			return tx, nil
		})).(*orgUnitPGStore)
		got, err := store.ResolveOrgCode(ctx, "t1", 10000001)
		if err != nil || got != "A1" {
			t.Fatalf("got=%q err=%v", got, err)
		}
	})
}

func TestOrgUnitPGStore_ResolveOrgCodes(t *testing.T) {
	ctx := context.Background()

	t.Run("begin error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})).(*orgUnitPGStore)
		if _, err := store.ResolveOrgCodes(ctx, "t1", []int{0}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		})).(*orgUnitPGStore)
		if _, err := store.ResolveOrgCodes(ctx, "t1", []int{0}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("resolve error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{queryErr: errors.New("resolve")}, nil
		})).(*orgUnitPGStore)
		if _, err := store.ResolveOrgCodes(ctx, "t1", []int{0}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{commitErr: errors.New("commit"), rows: &fakeRows{}}, nil
		})).(*orgUnitPGStore)
		if _, err := store.ResolveOrgCodes(ctx, "t1", []int{0}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &fakeRows{}}, nil
		})).(*orgUnitPGStore)
		got, err := store.ResolveOrgCodes(ctx, "t1", []int{0})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if got[0] != "n1" {
			t.Fatalf("unexpected codes: %v", got)
		}
	})
}

func TestOrgUnitPGStore_SetBusinessUnitCurrent_Errors(t *testing.T) {
	cases := []struct {
		name          string
		store         *orgUnitPGStore
		effectiveDate string
		orgID         string
		requestID     string
		randError     bool
	}{
		{
			name: "begin error",
			store: &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return nil, errors.New("begin fail")
			})},
			effectiveDate: "2026-01-01",
			orgID:         "10000001",
			requestID:     "r1",
		},
		{
			name: "set_config error",
			store: &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{execErr: errors.New("exec fail"), execErrAt: 1}, nil
			})},
			effectiveDate: "2026-01-01",
			orgID:         "10000001",
			requestID:     "r1",
		},
		{
			name: "missing effective date",
			store: &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})},
			effectiveDate: "",
			orgID:         "10000001",
			requestID:     "r1",
		},
		{
			name: "missing org id",
			store: &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})},
			effectiveDate: "2026-01-01",
			orgID:         "",
			requestID:     "r1",
		},
		{
			name: "event id error",
			store: &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})},
			effectiveDate: "2026-01-01",
			orgID:         "10000001",
			requestID:     "r1",
			randError:     true,
		},
		{
			name: "savepoint error",
			store: &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{
					row:       &stubRow{vals: []any{"e1"}},
					execErr:   errors.New("exec fail"),
					execErrAt: 2,
				}, nil
			})},
			effectiveDate: "2026-01-01",
			orgID:         "10000001",
			requestID:     "r1",
		},
		{
			name: "submit error",
			store: &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{row: &stubRow{vals: []any{"e1"}}, execErr: errors.New("exec fail"), execErrAt: 3}, nil
			})},
			effectiveDate: "2026-01-01",
			orgID:         "10000001",
			requestID:     "r1",
		},
		{
			name: "commit error",
			store: &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{row: &stubRow{vals: []any{"e1"}}, commitErr: errors.New("commit fail")}, nil
			})},
			effectiveDate: "2026-01-01",
			orgID:         "10000001",
			requestID:     "r1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			call := func() {
				if err := tc.store.SetBusinessUnitCurrent(context.Background(), "t1", tc.effectiveDate, tc.orgID, true, tc.requestID); err == nil {
					t.Fatal("expected error")
				}
			}
			if tc.randError {
				withRandReader(t, randErrReader{}, call)
				return
			}
			call()
		})
	}
}

func TestOrgUnitPGStore_SetBusinessUnitCurrent_Success(t *testing.T) {
	store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return &stubTx{row: &stubRow{vals: []any{"e1"}}}, nil
	})}
	if err := store.SetBusinessUnitCurrent(context.Background(), "t1", "2026-01-01", "10000001", true, ""); err != nil {
		t.Fatalf("err=%v", err)
	}
}

func TestOrgUnitPGStore_SetBusinessUnitCurrent_Idempotent(t *testing.T) {
	dupErr := &pgconn.PgError{Code: "23505", ConstraintName: "org_events_one_per_day_unique"}

	t.Run("already-set", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{
				row:       &stubRow{vals: []any{true}},
				execErr:   dupErr,
				execErrAt: 3,
			}, nil
		})}
		if err := store.SetBusinessUnitCurrent(context.Background(), "t1", "2026-01-01", "10000001", true, "r1"); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("mismatch", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{
				row:       &stubRow{vals: []any{false}},
				execErr:   dupErr,
				execErrAt: 3,
			}, nil
		})}
		if err := store.SetBusinessUnitCurrent(context.Background(), "t1", "2026-01-01", "10000001", true, "r1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("already-set event-date-conflict", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{
				row:       &stubRow{vals: []any{true}},
				execErr:   errors.New("EVENT_DATE_CONFLICT"),
				execErrAt: 3,
			}, nil
		})}
		if err := store.SetBusinessUnitCurrent(context.Background(), "t1", "2026-01-01", "10000001", true, "r1"); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("mismatch event-date-conflict", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{
				row:       &stubRow{vals: []any{false}},
				execErr:   errors.New("EVENT_DATE_CONFLICT"),
				execErrAt: 3,
			}, nil
		})}
		if err := store.SetBusinessUnitCurrent(context.Background(), "t1", "2026-01-01", "10000001", true, "r1"); err == nil {
			t.Fatal("expected error")
		}
	})
}

type rollbackErrTx struct {
	*stubTx
	rollbackErr error
}

func (t *rollbackErrTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "ROLLBACK TO SAVEPOINT") {
		return pgconn.CommandTag{}, t.rollbackErr
	}
	return t.stubTx.Exec(ctx, sql, args...)
}

func TestOrgUnitPGStore_SetBusinessUnitCurrent_RollbackError(t *testing.T) {
	tx := &rollbackErrTx{
		stubTx: &stubTx{
			row:       &stubRow{vals: []any{"e1"}},
			execErr:   errors.New("exec fail"),
			execErrAt: 3,
		},
		rollbackErr: errors.New("rollback fail"),
	}
	store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
	if err := store.SetBusinessUnitCurrent(context.Background(), "t1", "2026-01-01", "10000001", true, "r1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestOrgUnitPGStore_CorrectNodeEffectiveDate_Errors(t *testing.T) {
	cases := []struct {
		name            string
		store           *orgUnitPGStore
		targetEffective string
		newEffective    string
		requestID       string
	}{
		{
			name: "begin error",
			store: &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return nil, errors.New("begin fail")
			})},
			targetEffective: "2026-01-01",
			newEffective:    "2026-01-02",
			requestID:       "r1",
		},
		{
			name: "set_config error",
			store: &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{execErr: errors.New("exec fail"), execErrAt: 1}, nil
			})},
			targetEffective: "2026-01-01",
			newEffective:    "2026-01-02",
			requestID:       "r1",
		},
		{
			name: "missing target",
			store: &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})},
			targetEffective: "",
			newEffective:    "2026-01-02",
			requestID:       "r1",
		},
		{
			name: "missing new date",
			store: &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})},
			targetEffective: "2026-01-01",
			newEffective:    "",
			requestID:       "r1",
		},
		{
			name: "missing request",
			store: &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})},
			targetEffective: "2026-01-01",
			newEffective:    "2026-01-02",
			requestID:       "",
		},
		{
			name: "submit error",
			store: &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{rowErr: errors.New("row")}, nil
			})},
			targetEffective: "2026-01-01",
			newEffective:    "2026-01-02",
			requestID:       "r1",
		},
		{
			name: "commit error",
			store: &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{row: &stubRow{vals: []any{"c1"}}, commitErr: errors.New("commit")}, nil
			})},
			targetEffective: "2026-01-01",
			newEffective:    "2026-01-02",
			requestID:       "r1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.store.CorrectNodeEffectiveDate(context.Background(), "t1", 10000001, tc.targetEffective, tc.newEffective, tc.requestID); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestOrgUnitPGStore_CorrectNodeEffectiveDate_Success(t *testing.T) {
	store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return &stubTx{row: &stubRow{vals: []any{"c1"}}}, nil
	})}
	if err := store.CorrectNodeEffectiveDate(context.Background(), "t1", 10000001, "2026-01-01", "2026-01-02", "r1"); err != nil {
		t.Fatalf("err=%v", err)
	}
}

func TestOrgUnitPGStore_UsesQuotedCurrentTenantKey(t *testing.T) {
	ctx := context.Background()

	t.Run("MaxEffectiveDateOnOrBefore", func(t *testing.T) {
		tx := &stubTx{row: &stubRow{vals: []any{time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return tx, nil
		})}

		if _, _, err := store.MaxEffectiveDateOnOrBefore(ctx, "t1", "2026-01-02"); err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(tx.execSQLs) == 0 {
			t.Fatal("expected set_config call")
		}
		if got := tx.execSQLs[0]; !strings.Contains(got, "set_config('app.current_tenant', $1, true)") {
			t.Fatalf("unexpected sql: %q", got)
		}
	})

	t.Run("MinEffectiveDate", func(t *testing.T) {
		tx := &stubTx{row: &stubRow{vals: []any{time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return tx, nil
		})}

		if _, _, err := store.MinEffectiveDate(ctx, "t1"); err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(tx.execSQLs) == 0 {
			t.Fatal("expected set_config call")
		}
		if got := tx.execSQLs[0]; !strings.Contains(got, "set_config('app.current_tenant', $1, true)") {
			t.Fatalf("unexpected sql: %q", got)
		}
	})

	t.Run("CorrectNodeEffectiveDate", func(t *testing.T) {
		tx := &stubTx{row: &stubRow{vals: []any{"c1"}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return tx, nil
		})}

		if err := store.CorrectNodeEffectiveDate(ctx, "t1", 10000001, "2026-01-01", "2025-12-31", "r1"); err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(tx.execSQLs) == 0 {
			t.Fatal("expected set_config call")
		}
		if got := tx.execSQLs[0]; !strings.Contains(got, "set_config('app.current_tenant', $1, true)") {
			t.Fatalf("unexpected sql: %q", got)
		}
	})
}

func TestOrgUnitPGStore_ListNodesCurrent_AndCreateCurrent(t *testing.T) {
	pool := &fakeBeginner{}
	store := &orgUnitPGStore{pool: pool}

	nodes, err := store.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].ID != "n1" {
		t.Fatalf("nodes=%+v", nodes)
	}

	createdCurrent, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "C001", "Current", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if createdCurrent.ID != "10000001" || createdCurrent.Name != "Current" || !createdCurrent.CreatedAt.Equal(time.Unix(789, 0).UTC()) {
		t.Fatalf("created=%+v", createdCurrent)
	}

	createdWithParent, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "C002", "CurrentWithParent", "10000002", false)
	if err != nil {
		t.Fatal(err)
	}
	if createdWithParent.ID == "" {
		t.Fatal("expected id")
	}
}

func TestOrgUnitPGStore_ListBusinessUnitsCurrent(t *testing.T) {
	pool := &fakeBeginner{}
	store := &orgUnitPGStore{pool: pool}

	nodes, err := store.ListBusinessUnitsCurrent(context.Background(), "t1", "2026-01-06")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].ID != "n1" {
		t.Fatalf("nodes=%+v", nodes)
	}
}

func TestOrgUnitPGStore_ListBusinessUnitsCurrent_Errors(t *testing.T) {
	t.Run("begin", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})}
		_, err := store.ListBusinessUnitsCurrent(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		})}
		_, err := store.ListBusinessUnitsCurrent(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{queryErr: errors.New("query")}, nil
		})}
		_, err := store.ListBusinessUnitsCurrent(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &stubRows{scanErr: errors.New("scan")}}, nil
		})}
		_, err := store.ListBusinessUnitsCurrent(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows_err", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &stubRows{err: errors.New("rows")}}, nil
		})}
		_, err := store.ListBusinessUnitsCurrent(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{commitErr: errors.New("commit")}, nil
		})}
		_, err := store.ListBusinessUnitsCurrent(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestOrgUnitPGStore_ListNodesCurrent_Errors(t *testing.T) {
	t.Run("begin", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{queryErr: errors.New("query")}, nil
		}))
		_, err := store.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &stubRows{scanErr: errors.New("scan")}}, nil
		}))
		_, err := store.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows_err", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &stubRows{err: errors.New("rows")}}, nil
		}))
		_, err := store.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{commitErr: errors.New("commit")}, nil
		}))
		_, err := store.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestOrgUnitPGStore_CreateNodeCurrent_Errors(t *testing.T) {
	t.Run("begin", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})}
		_, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A001", "A", "", false)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		})}
		_, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A001", "A", "", false)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("effective_date_required", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		})}
		_, err := store.CreateNodeCurrent(context.Background(), "t1", "", "A001", "A", "", false)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("org_code_invalid", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		})}
		_, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "bad\x7f", "A", "", false)
		if !errors.Is(err, orgunitpkg.ErrOrgCodeInvalid) {
			t.Fatalf("expected ErrOrgCodeInvalid, got %v", err)
		}
	})

	t.Run("parent_id_invalid", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		})}
		_, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A001", "A", "bad", false)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("org_id_scan", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("row")}, nil
		})}
		_, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A001", "A", "", false)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("event_id_error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{10000001}}
			return tx, nil
		})}
		withRandReader(t, randErrReader{}, func() {
			if _, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A001", "A", "", false); err == nil {
				t.Fatal("expected error")
			}
		})
	})

	t.Run("submit_exec", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{execErr: errors.New("exec"), execErrAt: 2}
			tx.row = &stubRow{vals: []any{10000001}}
			return tx, nil
		})}
		_, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A001", "A", "", false)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{commitErr: errors.New("commit")}
			tx.row = &stubRow{vals: []any{10000001}}
			tx.row2 = &stubRow{vals: []any{time.Unix(1, 0).UTC()}}
			return tx, nil
		})}
		_, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A001", "A", "", false)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestOrgUnitPGStore_RenameMoveDisableCurrent(t *testing.T) {
	t.Run("rename_success", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"e1"}}
			return tx, nil
		})}
		if err := store.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", "New"); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("rename_errors", func(t *testing.T) {
		t.Run("begin", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return nil, errors.New("begin")
			})}
			if err := store.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", "New"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("set_config", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{execErr: errors.New("exec")}, nil
			})}
			if err := store.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", "New"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("effective_date_required", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			if err := store.RenameNodeCurrent(context.Background(), "t1", "", "10000001", "New"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("org_id_required", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			if err := store.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "", "New"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("new_name_required", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			if err := store.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", ""); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("event_id_error", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			withRandReader(t, randErrReader{}, func() {
				if err := store.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", "New"); err == nil {
					t.Fatal("expected error")
				}
			})
		})
		t.Run("submit_exec", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				tx := &stubTx{execErr: errors.New("exec"), execErrAt: 2}
				tx.row = &stubRow{vals: []any{"e1"}}
				return tx, nil
			})}
			if err := store.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", "New"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("commit", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				tx := &stubTx{commitErr: errors.New("commit")}
				tx.row = &stubRow{vals: []any{"e1"}}
				return tx, nil
			})}
			if err := store.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", "New"); err == nil {
				t.Fatal("expected error")
			}
		})
	})

	t.Run("move_success", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"e1"}}
			return tx, nil
		})}
		if err := store.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", ""); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("move_success_with_parent", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"e1"}}
			return tx, nil
		})}
		if err := store.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", "10000002"); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("move_errors", func(t *testing.T) {
		t.Run("begin", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return nil, errors.New("begin")
			})}
			if err := store.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", ""); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("set_config", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{execErr: errors.New("exec")}, nil
			})}
			if err := store.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", ""); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("effective_date_required", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			if err := store.MoveNodeCurrent(context.Background(), "t1", "", "10000001", ""); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("org_id_required", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			if err := store.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "", ""); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("new_parent_invalid", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			if err := store.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", "bad"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("event_id_error", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			withRandReader(t, randErrReader{}, func() {
				if err := store.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", ""); err == nil {
					t.Fatal("expected error")
				}
			})
		})
		t.Run("submit_exec", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				tx := &stubTx{execErr: errors.New("exec"), execErrAt: 2}
				tx.row = &stubRow{vals: []any{"e1"}}
				return tx, nil
			})}
			if err := store.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", ""); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("commit", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				tx := &stubTx{commitErr: errors.New("commit")}
				tx.row = &stubRow{vals: []any{"e1"}}
				return tx, nil
			})}
			if err := store.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", ""); err == nil {
				t.Fatal("expected error")
			}
		})
	})

	t.Run("disable_success", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"e1"}}
			return tx, nil
		})}
		if err := store.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001"); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("disable_errors", func(t *testing.T) {
		t.Run("begin", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return nil, errors.New("begin")
			})}
			if err := store.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("set_config", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{execErr: errors.New("exec")}, nil
			})}
			if err := store.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("effective_date_required", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			if err := store.DisableNodeCurrent(context.Background(), "t1", "", "10000001"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("org_id_required", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			if err := store.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", ""); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("event_id_error", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			withRandReader(t, randErrReader{}, func() {
				if err := store.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001"); err == nil {
					t.Fatal("expected error")
				}
			})
		})
		t.Run("submit_exec", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				tx := &stubTx{execErr: errors.New("exec"), execErrAt: 2}
				tx.row = &stubRow{vals: []any{"e1"}}
				return tx, nil
			})}
			if err := store.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("commit", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				tx := &stubTx{commitErr: errors.New("commit")}
				tx.row = &stubRow{vals: []any{"e1"}}
				return tx, nil
			})}
			if err := store.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001"); err == nil {
				t.Fatal("expected error")
			}
		})
	})
}
