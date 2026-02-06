package server

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

func TestParseOrgID8(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty", input: "", wantErr: true},
		{name: "len_mismatch", input: "123", wantErr: true},
		{name: "non_digit", input: "12ab5678", wantErr: true},
		{name: "out_of_range", input: "00000000", wantErr: true},
		{name: "ok", input: "10000001", wantErr: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseOrgID8(tc.input)
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestCanEditOrgNodes(t *testing.T) {
	if canEditOrgNodes(context.Background()) {
		t.Fatal("expected false without principal")
	}
	if canEditOrgNodes(withPrincipal(context.Background(), Principal{RoleSlug: " "})) {
		t.Fatal("expected false for empty role")
	}
	if canEditOrgNodes(withPrincipal(context.Background(), Principal{RoleSlug: "tenant-viewer"})) {
		t.Fatal("expected false for viewer")
	}
	if !canEditOrgNodes(withPrincipal(context.Background(), Principal{RoleSlug: authz.RoleTenantAdmin})) {
		t.Fatal("expected true for tenant admin")
	}
	if !canEditOrgNodes(withPrincipal(context.Background(), Principal{RoleSlug: " SUPERADMIN "})) {
		t.Fatal("expected true for superadmin")
	}
}

func TestParseOptionalOrgID8(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		_, ok, err := parseOptionalOrgID8("")
		if err != nil || ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})
	t.Run("invalid", func(t *testing.T) {
		if _, _, err := parseOptionalOrgID8("bad"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("ok", func(t *testing.T) {
		got, ok, err := parseOptionalOrgID8("10000001")
		if err != nil || !ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
		if got != 10000001 {
			t.Fatalf("expected 10000001, got %d", got)
		}
	})
}

func TestOrgUnitMemoryStore(t *testing.T) {
	s := newOrgUnitMemoryStore()
	s.now = func() time.Time { return time.Unix(123, 0).UTC() }

	created, err := s.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A001", "Hello World", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", created.ID, "Hello"); err != nil {
		t.Fatal(err)
	}
	nodes, err := s.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("len=%d", len(nodes))
	}
	if nodes[0].Name != "Hello" {
		t.Fatalf("name=%q", nodes[0].Name)
	}
	if nodes[0].OrgCode != "A001" {
		t.Fatalf("org_code=%q", nodes[0].OrgCode)
	}
	if nodes[0].CreatedAt != time.Unix(123, 0).UTC() {
		t.Fatalf("created_at=%s", nodes[0].CreatedAt)
	}
	orgID, err := s.ResolveOrgID(context.Background(), "t1", "A001")
	if err != nil || orgID != 10000000 {
		t.Fatalf("orgID=%d err=%v", orgID, err)
	}
	if code, err := s.ResolveOrgCode(context.Background(), "t1", 10000000); err != nil || code != "A001" {
		t.Fatalf("code=%q err=%v", code, err)
	}
}

func TestOrgUnitMemoryStore_ResolveOrgID_Errors(t *testing.T) {
	s := newOrgUnitMemoryStore()
	if _, err := s.ResolveOrgID(context.Background(), "t1", "bad\x7f"); !errors.Is(err, orgunitpkg.ErrOrgCodeInvalid) {
		t.Fatalf("err=%v", err)
	}
	if _, err := s.ResolveOrgID(context.Background(), "t1", "A001"); !errors.Is(err, orgunitpkg.ErrOrgCodeNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestOrgUnitMemoryStore_ResolveOrgCode_NotFound(t *testing.T) {
	s := newOrgUnitMemoryStore()
	if _, err := s.ResolveOrgCode(context.Background(), "t1", 10000001); !errors.Is(err, orgunitpkg.ErrOrgIDNotFound) {
		t.Fatalf("err=%v", err)
	}
}

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

func TestOrgUnitMemoryStore_ResolveOrgCodes(t *testing.T) {
	ctx := context.Background()
	store := newOrgUnitMemoryStore()

	got, err := store.ResolveOrgCodes(ctx, "t1", nil)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty map, got %v", got)
	}

	n1, err := store.CreateNodeCurrent(ctx, "t1", "2026-01-01", "A1", "A", "", false)
	if err != nil {
		t.Fatal(err)
	}
	n2, err := store.CreateNodeCurrent(ctx, "t1", "2026-01-01", "B1", "B", "", false)
	if err != nil {
		t.Fatal(err)
	}
	id1, _ := parseOrgID8(n1.ID)
	id2, _ := parseOrgID8(n2.ID)

	codes, err := store.ResolveOrgCodes(ctx, "t1", []int{id1, id2})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if codes[id1] != "A1" || codes[id2] != "B1" {
		t.Fatalf("unexpected codes: %v", codes)
	}

	if _, err := store.ResolveOrgCodes(ctx, "t1", []int{id1, 99999999}); !errors.Is(err, orgunitpkg.ErrOrgIDNotFound) {
		t.Fatalf("err=%v", err)
	}

	badStore := newOrgUnitMemoryStore()
	badStore.nodes["t1"] = []OrgUnitNode{{ID: "bad", OrgCode: "A1"}}
	if _, err := badStore.ResolveOrgCodes(ctx, "t1", []int{10000001}); !errors.Is(err, orgunitpkg.ErrOrgIDNotFound) {
		t.Fatalf("err=%v", err)
	}
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

func TestOrgUnitMemoryStore_ResolveSetID(t *testing.T) {
	store := newOrgUnitMemoryStore()

	if _, err := store.ResolveSetID(context.Background(), "t1", "", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}
	got, err := store.ResolveSetID(context.Background(), "t1", "10000001", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if got != "S2601" {
		t.Fatalf("expected S2601, got %q", got)
	}
}

func TestOrgUnitMemoryStore_RenameNodeCurrent_Errors(t *testing.T) {
	s := newOrgUnitMemoryStore()
	created, err := s.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A001", "A", "", false)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "", "B"); err == nil {
		t.Fatal("expected error")
	}
	if err := s.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", created.ID, ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "10000002", "B"); err == nil {
		t.Fatal("expected error")
	}
}

func TestOrgUnitMemoryStore_RenameNodeCurrent_Success(t *testing.T) {
	s := newOrgUnitMemoryStore()
	created, err := s.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A002", "A", "", false)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", created.ID, "B"); err != nil {
		t.Fatalf("err=%v", err)
	}
	nodes, err := s.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].Name != "B" {
		t.Fatalf("nodes=%v", nodes)
	}
}

func TestOrgUnitMemoryStore_MoveDisableNodeCurrent(t *testing.T) {
	s := newOrgUnitMemoryStore()
	created, err := s.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A003", "A", "", false)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "10000002", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", created.ID, ""); err != nil {
		t.Fatalf("err=%v", err)
	}

	if err := s.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", "10000002"); err == nil {
		t.Fatal("expected error")
	}
	if err := s.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", created.ID); err != nil {
		t.Fatalf("err=%v", err)
	}

	nodes, err := s.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 0 {
		t.Fatalf("len=%d", len(nodes))
	}
}

func TestOrgUnitMemoryStore_SetBusinessUnitCurrent(t *testing.T) {
	s := newOrgUnitMemoryStore()
	created, err := s.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A004", "A", "", false)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.SetBusinessUnitCurrent(context.Background(), "t1", "2026-01-06", "", true, "r1"); err == nil {
		t.Fatal("expected error")
	}
	if err := s.SetBusinessUnitCurrent(context.Background(), "t1", "2026-01-06", "10000002", true, "r1"); err == nil {
		t.Fatal("expected error")
	}
	if err := s.SetBusinessUnitCurrent(context.Background(), "t1", "2026-01-06", created.ID, true, "r1"); err != nil {
		t.Fatalf("err=%v", err)
	}
	nodes, err := s.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || !nodes[0].IsBusinessUnit {
		t.Fatalf("nodes=%v", nodes)
	}
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

func TestHandleOrgNodes_MissingTenant(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/org/nodes", nil)
	handleOrgNodes(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgNodes_GET_HX(t *testing.T) {
	store := newOrgUnitMemoryStore()
	_, _ = store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A001", "A", "", false)

	req := httptest.NewRequest(http.MethodGet, "/org/nodes?tree_as_of=2026-01-06", nil)
	req.Header.Set("HX-Request", "true")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); body == "" || bytes.Contains(rec.Body.Bytes(), []byte("<!doctype html>")) {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleOrgNodes_GET_Success(t *testing.T) {
	store := newOrgUnitMemoryStore()
	_, _ = store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A002", "A", "", false)

	req := httptest.NewRequest(http.MethodGet, "/org/nodes?tree_as_of=2026-01-06", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "A") {
		t.Fatalf("unexpected body: %q", body)
	}
}

type errStore struct{}

func (errStore) ListNodesCurrent(context.Context, string, string) ([]OrgUnitNode, error) {
	return nil, errBoom{}
}
func (errStore) CreateNodeCurrent(context.Context, string, string, string, string, string, bool) (OrgUnitNode, error) {
	return OrgUnitNode{}, errBoom{}
}
func (errStore) RenameNodeCurrent(context.Context, string, string, string, string) error {
	return errBoom{}
}
func (errStore) MoveNodeCurrent(context.Context, string, string, string, string) error {
	return errBoom{}
}
func (errStore) DisableNodeCurrent(context.Context, string, string, string) error { return errBoom{} }
func (errStore) SetBusinessUnitCurrent(context.Context, string, string, string, bool, string) error {
	return errBoom{}
}
func (errStore) ResolveOrgID(context.Context, string, string) (int, error)   { return 0, errBoom{} }
func (errStore) ResolveOrgCode(context.Context, string, int) (string, error) { return "", errBoom{} }
func (errStore) ResolveOrgCodes(context.Context, string, []int) (map[int]string, error) {
	return nil, errBoom{}
}
func (errStore) ListChildren(context.Context, string, int, string) ([]OrgUnitChild, error) {
	return nil, errBoom{}
}
func (errStore) GetNodeDetails(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
	return OrgUnitNodeDetails{}, errBoom{}
}
func (errStore) SearchNode(context.Context, string, string, string) (OrgUnitSearchResult, error) {
	return OrgUnitSearchResult{}, errBoom{}
}
func (errStore) SearchNodeCandidates(context.Context, string, string, string, int) ([]OrgUnitSearchCandidate, error) {
	return nil, errBoom{}
}
func (errStore) ListNodeVersions(context.Context, string, int) ([]OrgUnitNodeVersion, error) {
	return nil, errBoom{}
}
func (errStore) MaxEffectiveDateOnOrBefore(context.Context, string, string) (string, bool, error) {
	return "", false, errBoom{}
}
func (errStore) MinEffectiveDate(context.Context, string) (string, bool, error) {
	return "", false, errBoom{}
}

type errBoom struct{}

func (errBoom) Error() string { return "boom" }

type emptyErr struct{}

func (emptyErr) Error() string { return "" }

type emptyErrStore struct{}

func (emptyErrStore) ListNodesCurrent(context.Context, string, string) ([]OrgUnitNode, error) {
	return nil, emptyErr{}
}
func (emptyErrStore) CreateNodeCurrent(context.Context, string, string, string, string, string, bool) (OrgUnitNode, error) {
	return OrgUnitNode{}, emptyErr{}
}
func (emptyErrStore) RenameNodeCurrent(context.Context, string, string, string, string) error {
	return emptyErr{}
}
func (emptyErrStore) MoveNodeCurrent(context.Context, string, string, string, string) error {
	return emptyErr{}
}
func (emptyErrStore) DisableNodeCurrent(context.Context, string, string, string) error {
	return emptyErr{}
}
func (emptyErrStore) SetBusinessUnitCurrent(context.Context, string, string, string, bool, string) error {
	return emptyErr{}
}
func (emptyErrStore) ResolveOrgID(context.Context, string, string) (int, error) {
	return 0, emptyErr{}
}
func (emptyErrStore) ResolveOrgCode(context.Context, string, int) (string, error) {
	return "", emptyErr{}
}
func (emptyErrStore) ResolveOrgCodes(context.Context, string, []int) (map[int]string, error) {
	return nil, emptyErr{}
}
func (emptyErrStore) ListChildren(context.Context, string, int, string) ([]OrgUnitChild, error) {
	return nil, emptyErr{}
}
func (emptyErrStore) GetNodeDetails(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
	return OrgUnitNodeDetails{}, emptyErr{}
}
func (emptyErrStore) SearchNode(context.Context, string, string, string) (OrgUnitSearchResult, error) {
	return OrgUnitSearchResult{}, emptyErr{}
}
func (emptyErrStore) SearchNodeCandidates(context.Context, string, string, string, int) ([]OrgUnitSearchCandidate, error) {
	return nil, emptyErr{}
}
func (emptyErrStore) ListNodeVersions(context.Context, string, int) ([]OrgUnitNodeVersion, error) {
	return nil, emptyErr{}
}
func (emptyErrStore) MaxEffectiveDateOnOrBefore(context.Context, string, string) (string, bool, error) {
	return "", false, emptyErr{}
}
func (emptyErrStore) MinEffectiveDate(context.Context, string) (string, bool, error) {
	return "", false, emptyErr{}
}

func TestHandleOrgNodes_GET_StoreError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/nodes?tree_as_of=2026-01-06", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, errStore{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !bytes.Contains([]byte(body), []byte("boom")) {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleOrgNodes_GET_DeprecatedAsOf(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/nodes?as_of=2026-01-06", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, errStore{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !bytes.Contains([]byte(body), []byte("deprecated as_of")) {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleOrgNodes_POST_BadForm(t *testing.T) {
	store := newOrgUnitMemoryStore()
	body := bytes.NewBufferString("%zz")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgNodes_POST_BadForm_MergesEmptyStoreError(t *testing.T) {
	body := bytes.NewBufferString("%zz")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, emptyErrStore{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !strings.Contains(bodyOut, "bad form") {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_BadForm_MergesNonEmptyStoreError(t *testing.T) {
	body := bytes.NewBufferString("%zz")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, errStore{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	bodyOut := rec.Body.String()
	if !strings.Contains(bodyOut, "bad form") || !strings.Contains(bodyOut, "boom") {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_EmptyName(t *testing.T) {
	store := newOrgUnitMemoryStore()
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", bytes.NewBufferString("org_code=A001&name="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !bytes.Contains([]byte(body), []byte("name is required")) {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleOrgNodes_POST_Create_InvalidOrgCode(t *testing.T) {
	store := newOrgUnitMemoryStore()
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", bytes.NewBufferString("org_code=bad%7F&name=A"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "org_code invalid") {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleOrgNodes_POST_SuccessRedirect(t *testing.T) {
	store := newOrgUnitMemoryStore()
	body := bytes.NewBufferString("org_code=A002&name=A&effective_date=2026-01-06")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/org/nodes?tree_as_of=2026-01-06" {
		t.Fatalf("location=%q", loc)
	}
}

type writeSpyStore struct {
	createCalled  int
	renameCalled  int
	moveCalled    int
	disableCalled int

	argsCreate  []string
	argsRename  []string
	argsMove    []string
	argsDisable []string

	err error
}

func (s *writeSpyStore) ListNodesCurrent(context.Context, string, string) ([]OrgUnitNode, error) {
	return []OrgUnitNode{{ID: "c1", OrgCode: "C001", Name: "C"}}, nil
}
func (s *writeSpyStore) CreateNodeCurrent(_ context.Context, tenantID string, effectiveDate string, orgCode string, name string, parentID string, _ bool) (OrgUnitNode, error) {
	s.createCalled++
	s.argsCreate = []string{tenantID, effectiveDate, orgCode, name, parentID}
	if s.err != nil {
		return OrgUnitNode{}, s.err
	}
	return OrgUnitNode{ID: "10000001", OrgCode: orgCode, Name: name, CreatedAt: time.Unix(1, 0).UTC()}, nil
}
func (s *writeSpyStore) RenameNodeCurrent(_ context.Context, tenantID string, effectiveDate string, orgID string, newName string) error {
	s.renameCalled++
	s.argsRename = []string{tenantID, effectiveDate, orgID, newName}
	return s.err
}
func (s *writeSpyStore) MoveNodeCurrent(_ context.Context, tenantID string, effectiveDate string, orgID string, newParentID string) error {
	s.moveCalled++
	s.argsMove = []string{tenantID, effectiveDate, orgID, newParentID}
	return s.err
}
func (s *writeSpyStore) DisableNodeCurrent(_ context.Context, tenantID string, effectiveDate string, orgID string) error {
	s.disableCalled++
	s.argsDisable = []string{tenantID, effectiveDate, orgID}
	return s.err
}
func (s *writeSpyStore) SetBusinessUnitCurrent(_ context.Context, _ string, _ string, _ string, _ bool, _ string) error {
	return s.err
}
func (s *writeSpyStore) ResolveOrgID(_ context.Context, _ string, orgCode string) (int, error) {
	if s.err != nil {
		return 0, s.err
	}
	normalized, err := orgunitpkg.NormalizeOrgCode(orgCode)
	if err != nil {
		return 0, err
	}
	if normalized == "PARENT" {
		return 10000002, nil
	}
	return 10000001, nil
}
func (s *writeSpyStore) ResolveOrgCode(context.Context, string, int) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return "A001", nil
}
func (s *writeSpyStore) ResolveOrgCodes(ctx context.Context, tenantID string, orgIDs []int) (map[int]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make(map[int]string, len(orgIDs))
	for _, orgID := range orgIDs {
		out[orgID] = "A001"
	}
	return out, nil
}
func (s *writeSpyStore) ListChildren(context.Context, string, int, string) ([]OrgUnitChild, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []OrgUnitChild{}, nil
}
func (s *writeSpyStore) GetNodeDetails(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
	if s.err != nil {
		return OrgUnitNodeDetails{}, s.err
	}
	return OrgUnitNodeDetails{OrgID: 10000001, OrgCode: "A001", Name: "Root"}, nil
}
func (s *writeSpyStore) SearchNode(context.Context, string, string, string) (OrgUnitSearchResult, error) {
	if s.err != nil {
		return OrgUnitSearchResult{}, s.err
	}
	return OrgUnitSearchResult{TargetOrgID: 10000001, TargetOrgCode: "A001", TargetName: "Root", PathOrgIDs: []int{10000001}, TreeAsOf: "2026-01-06"}, nil
}
func (s *writeSpyStore) SearchNodeCandidates(context.Context, string, string, string, int) ([]OrgUnitSearchCandidate, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []OrgUnitSearchCandidate{{OrgID: 10000001, OrgCode: "A001", Name: "Root"}}, nil
}
func (s *writeSpyStore) ListNodeVersions(context.Context, string, int) ([]OrgUnitNodeVersion, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []OrgUnitNodeVersion{{EventID: 1, EffectiveDate: "2026-01-06", EventType: "RENAME"}}, nil
}
func (s *writeSpyStore) MaxEffectiveDateOnOrBefore(_ context.Context, _ string, asOfDate string) (string, bool, error) {
	if s.err != nil {
		return "", false, s.err
	}
	return asOfDate, true, nil
}
func (s *writeSpyStore) MinEffectiveDate(_ context.Context, _ string) (string, bool, error) {
	if s.err != nil {
		return "", false, s.err
	}
	return "2026-01-01", true, nil
}

type actionErrStore struct {
	renameErr          error
	moveErr            error
	disableErr         error
	setBusinessUnitErr error
	resolveFn          func(ctx context.Context, tenantID string, orgCode string) (int, error)
	createArgs         []string
	moveArgs           []string
}

func (s *actionErrStore) ListNodesCurrent(context.Context, string, string) ([]OrgUnitNode, error) {
	return []OrgUnitNode{{ID: "c1", OrgCode: "C001", Name: "C"}}, nil
}
func (s *actionErrStore) CreateNodeCurrent(_ context.Context, tenantID string, effectiveDate string, orgCode string, name string, parentID string, _ bool) (OrgUnitNode, error) {
	s.createArgs = []string{tenantID, effectiveDate, orgCode, name, parentID}
	return OrgUnitNode{ID: "10000001", OrgCode: orgCode, Name: name}, nil
}
func (s *actionErrStore) RenameNodeCurrent(_ context.Context, _ string, _ string, _ string, _ string) error {
	return s.renameErr
}
func (s *actionErrStore) MoveNodeCurrent(_ context.Context, tenantID string, effectiveDate string, orgID string, newParentID string) error {
	s.moveArgs = []string{tenantID, effectiveDate, orgID, newParentID}
	return s.moveErr
}
func (s *actionErrStore) DisableNodeCurrent(context.Context, string, string, string) error {
	return s.disableErr
}
func (s *actionErrStore) SetBusinessUnitCurrent(context.Context, string, string, string, bool, string) error {
	return s.setBusinessUnitErr
}
func (s *actionErrStore) ResolveOrgID(ctx context.Context, tenantID string, orgCode string) (int, error) {
	if s.resolveFn != nil {
		return s.resolveFn(ctx, tenantID, orgCode)
	}
	normalized, err := orgunitpkg.NormalizeOrgCode(orgCode)
	if err != nil {
		return 0, err
	}
	if normalized == "PARENT" {
		return 10000002, nil
	}
	return 10000001, nil
}
func (s *actionErrStore) ResolveOrgCode(context.Context, string, int) (string, error) {
	return "A001", nil
}
func (s *actionErrStore) ResolveOrgCodes(context.Context, string, []int) (map[int]string, error) {
	out := make(map[int]string)
	out[10000001] = "A001"
	return out, nil
}
func (s *actionErrStore) ListChildren(context.Context, string, int, string) ([]OrgUnitChild, error) {
	return []OrgUnitChild{}, nil
}
func (s *actionErrStore) GetNodeDetails(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
	return OrgUnitNodeDetails{OrgID: 10000001, OrgCode: "A001", Name: "Root"}, nil
}
func (s *actionErrStore) SearchNode(context.Context, string, string, string) (OrgUnitSearchResult, error) {
	return OrgUnitSearchResult{TargetOrgID: 10000001, TargetOrgCode: "A001", TargetName: "Root", PathOrgIDs: []int{10000001}, TreeAsOf: "2026-01-06"}, nil
}
func (s *actionErrStore) SearchNodeCandidates(context.Context, string, string, string, int) ([]OrgUnitSearchCandidate, error) {
	return []OrgUnitSearchCandidate{{OrgID: 10000001, OrgCode: "A001", Name: "Root"}}, nil
}
func (s *actionErrStore) ListNodeVersions(context.Context, string, int) ([]OrgUnitNodeVersion, error) {
	return []OrgUnitNodeVersion{{EventID: 1, EffectiveDate: "2026-01-06", EventType: "RENAME"}}, nil
}
func (s *actionErrStore) MaxEffectiveDateOnOrBefore(_ context.Context, _ string, asOfDate string) (string, bool, error) {
	return asOfDate, true, nil
}
func (s *actionErrStore) MinEffectiveDate(_ context.Context, _ string) (string, bool, error) {
	return "2026-01-01", true, nil
}

type recordActionStore struct {
	actionErrStore
	versions              []OrgUnitNodeVersion
	versionsErr           error
	details               OrgUnitNodeDetails
	detailsErr            error
	renameCalled          int
	disableCalled         int
	moveCalled            int
	setBusinessUnitCalled int
	setBusinessUnitArgs   []string
	setBusinessUnitValue  bool
	setBusinessUnitReqID  string
}

func (s *recordActionStore) ListNodeVersions(context.Context, string, int) ([]OrgUnitNodeVersion, error) {
	if s.versionsErr != nil {
		return nil, s.versionsErr
	}
	return s.versions, nil
}

func (s *recordActionStore) GetNodeDetails(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
	if s.detailsErr != nil {
		return OrgUnitNodeDetails{}, s.detailsErr
	}
	if s.details.OrgID == 0 && s.details.OrgCode == "" && s.details.Name == "" {
		return OrgUnitNodeDetails{OrgID: 10000001, OrgCode: "A001", Name: "Root"}, nil
	}
	return s.details, nil
}

func (s *recordActionStore) RenameNodeCurrent(_ context.Context, _ string, _ string, _ string, _ string) error {
	s.renameCalled++
	return s.renameErr
}

func (s *recordActionStore) MoveNodeCurrent(_ context.Context, tenantID string, effectiveDate string, orgID string, newParentID string) error {
	s.moveCalled++
	s.moveArgs = []string{tenantID, effectiveDate, orgID, newParentID}
	return s.moveErr
}

func (s *recordActionStore) DisableNodeCurrent(_ context.Context, _ string, _ string, _ string) error {
	s.disableCalled++
	return s.disableErr
}

func (s *recordActionStore) SetBusinessUnitCurrent(_ context.Context, tenantID string, effectiveDate string, orgID string, isBusinessUnit bool, requestID string) error {
	s.setBusinessUnitCalled++
	s.setBusinessUnitArgs = []string{tenantID, effectiveDate, orgID}
	s.setBusinessUnitValue = isBusinessUnit
	s.setBusinessUnitReqID = requestID
	return s.setBusinessUnitErr
}

type asOfSpyStore struct {
	gotAsOf string
}

func (s *asOfSpyStore) ListNodesCurrent(_ context.Context, _ string, asOf string) ([]OrgUnitNode, error) {
	s.gotAsOf = asOf
	return []OrgUnitNode{{ID: "c1", OrgCode: "C001", Name: "C"}}, nil
}
func (s *asOfSpyStore) CreateNodeCurrent(context.Context, string, string, string, string, string, bool) (OrgUnitNode, error) {
	return OrgUnitNode{}, nil
}
func (s *asOfSpyStore) RenameNodeCurrent(context.Context, string, string, string, string) error {
	return nil
}
func (s *asOfSpyStore) MoveNodeCurrent(context.Context, string, string, string, string) error {
	return nil
}
func (s *asOfSpyStore) DisableNodeCurrent(context.Context, string, string, string) error { return nil }
func (s *asOfSpyStore) SetBusinessUnitCurrent(context.Context, string, string, string, bool, string) error {
	return nil
}
func (s *asOfSpyStore) ResolveOrgID(context.Context, string, string) (int, error) {
	return 10000001, nil
}
func (s *asOfSpyStore) ResolveOrgCode(context.Context, string, int) (string, error) {
	return "A001", nil
}
func (s *asOfSpyStore) ResolveOrgCodes(context.Context, string, []int) (map[int]string, error) {
	return map[int]string{10000001: "A001"}, nil
}
func (s *asOfSpyStore) ListChildren(context.Context, string, int, string) ([]OrgUnitChild, error) {
	return []OrgUnitChild{}, nil
}
func (s *asOfSpyStore) GetNodeDetails(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
	return OrgUnitNodeDetails{OrgID: 10000001, OrgCode: "A001", Name: "Root"}, nil
}
func (s *asOfSpyStore) SearchNode(context.Context, string, string, string) (OrgUnitSearchResult, error) {
	return OrgUnitSearchResult{TargetOrgID: 10000001, TargetOrgCode: "A001", TargetName: "Root", PathOrgIDs: []int{10000001}, TreeAsOf: "2026-01-06"}, nil
}
func (s *asOfSpyStore) SearchNodeCandidates(context.Context, string, string, string, int) ([]OrgUnitSearchCandidate, error) {
	return []OrgUnitSearchCandidate{{OrgID: 10000001, OrgCode: "A001", Name: "Root"}}, nil
}
func (s *asOfSpyStore) ListNodeVersions(context.Context, string, int) ([]OrgUnitNodeVersion, error) {
	return []OrgUnitNodeVersion{{EventID: 1, EffectiveDate: "2026-01-06", EventType: "RENAME"}}, nil
}
func (s *asOfSpyStore) MaxEffectiveDateOnOrBefore(context.Context, string, string) (string, bool, error) {
	return "", false, nil
}
func (s *asOfSpyStore) MinEffectiveDate(context.Context, string) (string, bool, error) {
	return "", false, nil
}

func postOrgNodesForm(t *testing.T, store OrgUnitStore, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()
	handleOrgNodes(rec, req, store)
	return rec
}

func TestHandleOrgNodes_POST_Rename_UsesStore(t *testing.T) {
	store := &writeSpyStore{}
	body := bytes.NewBufferString("action=rename&org_code=ORG-1&new_name=New&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.renameCalled != 1 {
		t.Fatalf("renameCalled=%d", store.renameCalled)
	}
	if got := strings.Join(store.argsRename, "|"); got != "t1|2026-01-05|10000001|New" {
		t.Fatalf("args=%q", got)
	}
	if loc := rec.Header().Get("Location"); loc != "/org/nodes?tree_as_of=2026-01-06" {
		t.Fatalf("location=%q", loc)
	}
}

func TestHandleOrgNodes_POST_Rename_DefaultsEffectiveDateToAsOf(t *testing.T) {
	store := &writeSpyStore{}
	body := bytes.NewBufferString("action=rename&org_code=ORG-1&new_name=New")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if got := strings.Join(store.argsRename, "|"); got != "t1|2026-01-06|10000001|New" {
		t.Fatalf("args=%q", got)
	}
}

func TestHandleOrgNodes_POST_SetBusinessUnit_InvalidFlag(t *testing.T) {
	store := newOrgUnitMemoryStore()
	created, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A010", "A", "", false)
	if err != nil {
		t.Fatal(err)
	}

	form := url.Values{}
	form.Set("action", "set_business_unit")
	form.Set("org_code", created.OrgCode)
	form.Set("effective_date", "2026-01-06")
	form.Set("is_business_unit", "maybe")

	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "is_business_unit 无效") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleOrgNodes_POST_SetBusinessUnit_Success(t *testing.T) {
	store := newOrgUnitMemoryStore()
	created, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A011", "A", "", false)
	if err != nil {
		t.Fatal(err)
	}

	form := url.Values{}
	form.Set("action", "set_business_unit")
	form.Set("org_code", created.OrgCode)
	form.Set("effective_date", "2026-01-06")
	form.Set("is_business_unit", "true")

	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	nodes, err := store.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || !nodes[0].IsBusinessUnit {
		t.Fatalf("nodes=%v", nodes)
	}
}

func TestHandleOrgNodes_POST_SetBusinessUnit_StoreError(t *testing.T) {
	store := &writeSpyStore{err: errors.New("boom")}
	form := url.Values{}
	form.Set("action", "set_business_unit")
	form.Set("org_code", "ORG-1")
	form.Set("effective_date", "2026-01-06")
	form.Set("is_business_unit", "true")

	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleOrgNodes_POST_Rename_Error_ShowsErrorAndNodes(t *testing.T) {
	store := &writeSpyStore{err: errors.New("boom")}
	body := bytes.NewBufferString("action=rename&org_code=ORG-1&new_name=New&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !strings.Contains(bodyOut, "boom") || !strings.Contains(bodyOut, "C") {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_Rename_StoreError(t *testing.T) {
	store := &actionErrStore{renameErr: errors.New("boom")}
	body := bytes.NewBufferString("action=rename&org_code=ORG-1&new_name=New&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !strings.Contains(bodyOut, "boom") {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_Move_EmptyParent_AllowsEmpty(t *testing.T) {
	store := &actionErrStore{}
	form := url.Values{}
	form.Set("action", "move")
	form.Set("org_code", "ORG-1")
	form.Set("effective_date", "2026-01-06")

	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if len(store.moveArgs) != 4 || store.moveArgs[3] != "" {
		t.Fatalf("moveArgs=%v", store.moveArgs)
	}
}

func TestHandleOrgNodes_POST_Move_InvalidParent_ShowsError(t *testing.T) {
	store := &actionErrStore{
		resolveFn: func(_ context.Context, _ string, orgCode string) (int, error) {
			if orgCode == "ORG-1" {
				return 10000001, nil
			}
			if orgCode == "PARENT" {
				return 0, orgunitpkg.ErrOrgCodeNotFound
			}
			return 0, orgunitpkg.ErrOrgCodeInvalid
		},
	}
	form := url.Values{}
	form.Set("action", "move")
	form.Set("org_code", "ORG-1")
	form.Set("new_parent_code", "PARENT")
	form.Set("effective_date", "2026-01-06")

	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "new_parent_code not found") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleOrgNodes_POST_Move_StoreError(t *testing.T) {
	store := &actionErrStore{moveErr: errors.New("boom")}
	form := url.Values{}
	form.Set("action", "move")
	form.Set("org_code", "ORG-1")
	form.Set("new_parent_code", "PARENT")
	form.Set("effective_date", "2026-01-06")

	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleOrgNodes_POST_Disable_StoreError(t *testing.T) {
	store := &actionErrStore{disableErr: errors.New("boom")}
	form := url.Values{}
	form.Set("action", "disable")
	form.Set("org_code", "ORG-1")
	form.Set("effective_date", "2026-01-06")

	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleOrgNodes_POST_SetBusinessUnit_ErrorFromStore(t *testing.T) {
	store := &actionErrStore{setBusinessUnitErr: errors.New("boom")}
	form := url.Values{}
	form.Set("action", "set_business_unit")
	form.Set("org_code", "ORG-1")
	form.Set("effective_date", "2026-01-06")
	form.Set("is_business_unit", "true")

	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleOrgNodes_POST_Create_WithParentCode(t *testing.T) {
	store := &writeSpyStore{}
	form := url.Values{}
	form.Set("org_code", "A020")
	form.Set("name", "A")
	form.Set("effective_date", "2026-01-06")
	form.Set("parent_code", "PARENT")

	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if got := strings.Join(store.argsCreate, "|"); got != "t1|2026-01-06|A020|A|10000002" {
		t.Fatalf("args=%q", got)
	}
}

func TestHandleOrgNodes_POST_Create_ParentCodeResolveError(t *testing.T) {
	store := &actionErrStore{
		resolveFn: func(_ context.Context, _ string, orgCode string) (int, error) {
			if orgCode == "PARENT" {
				return 0, orgunitpkg.ErrOrgCodeNotFound
			}
			return 10000001, nil
		},
	}
	form := url.Values{}
	form.Set("org_code", "A021")
	form.Set("name", "A")
	form.Set("effective_date", "2026-01-06")
	form.Set("parent_code", "PARENT")

	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "parent_code not found") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleOrgNodes_POST_Create_BadEffectiveDate(t *testing.T) {
	store := &writeSpyStore{}
	body := bytes.NewBufferString("org_code=A010&name=A&effective_date=bad")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.createCalled != 0 {
		t.Fatalf("createCalled=%d", store.createCalled)
	}
	if bodyOut := rec.Body.String(); !strings.Contains(bodyOut, "effective_date 无效") {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_Create_BusinessUnitFalse(t *testing.T) {
	store := &writeSpyStore{}
	form := url.Values{}
	form.Set("org_code", "A011")
	form.Set("name", "A")
	form.Set("effective_date", "2026-01-06")
	form.Set("is_business_unit", "no")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.createCalled != 1 {
		t.Fatalf("createCalled=%d", store.createCalled)
	}
}

func TestHandleOrgNodes_POST_Create_InvalidBusinessUnitFlag(t *testing.T) {
	store := &writeSpyStore{}
	form := url.Values{}
	form.Set("org_code", "A012")
	form.Set("name", "A")
	form.Set("effective_date", "2026-01-06")
	form.Set("is_business_unit", "maybe")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !strings.Contains(bodyOut, "is_business_unit 无效") {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_Create_Error_ShowsError(t *testing.T) {
	store := &writeSpyStore{err: errors.New("boom")}
	body := bytes.NewBufferString("org_code=A013&name=A&effective_date=2026-01-06")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !strings.Contains(bodyOut, "boom") {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_Move_UsesStore(t *testing.T) {
	store := &writeSpyStore{}
	body := bytes.NewBufferString("action=move&org_code=ORG-1&new_parent_code=PARENT&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.moveCalled != 1 {
		t.Fatalf("moveCalled=%d", store.moveCalled)
	}
	if got := strings.Join(store.argsMove, "|"); got != "t1|2026-01-05|10000001|10000002" {
		t.Fatalf("args=%q", got)
	}
}

func TestHandleOrgNodes_POST_Disable_UsesStore(t *testing.T) {
	store := &writeSpyStore{}
	body := bytes.NewBufferString("action=disable&org_code=ORG-1&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.disableCalled != 1 {
		t.Fatalf("disableCalled=%d", store.disableCalled)
	}
	if got := strings.Join(store.argsDisable, "|"); got != "t1|2026-01-05|10000001" {
		t.Fatalf("args=%q", got)
	}
}

func TestHandleOrgNodes_POST_Disable_BadEffectiveDate(t *testing.T) {
	store := &writeSpyStore{}
	body := bytes.NewBufferString("action=disable&org_code=ORG-1&effective_date=bad")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.disableCalled != 0 {
		t.Fatalf("disableCalled=%d", store.disableCalled)
	}
	if bodyOut := rec.Body.String(); !bytes.Contains([]byte(bodyOut), []byte("effective_date 无效")) {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_Move_Error_ShowsErrorAndNodes(t *testing.T) {
	store := &writeSpyStore{err: errors.New("boom")}
	body := bytes.NewBufferString("action=move&org_code=ORG-1&new_parent_code=PARENT&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !strings.Contains(bodyOut, "boom") || !strings.Contains(bodyOut, "C") {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_Disable_Error_ShowsErrorAndNodes(t *testing.T) {
	store := &writeSpyStore{err: errors.New("boom")}
	body := bytes.NewBufferString("action=disable&org_code=ORG-1&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !strings.Contains(bodyOut, "boom") || !strings.Contains(bodyOut, "C") {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_MergesErrorHints(t *testing.T) {
	store := newOrgUnitMemoryStore()
	body := bytes.NewBufferString("name=")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=bad", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !strings.Contains(bodyOut, "invalid tree_as_of") {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_DefaultsEffectiveDateToAsOf(t *testing.T) {
	store := &writeSpyStore{}
	body := bytes.NewBufferString("org_code=A001&name=A")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if got := strings.Join(store.argsCreate, "|"); got != "t1|2026-01-06|A001|A|" {
		t.Fatalf("args=%q", got)
	}
}

func TestHandleOrgNodes_POST_Rename_MissingOrgCode(t *testing.T) {
	store := &writeSpyStore{}
	body := bytes.NewBufferString("action=rename&org_code=&new_name=New&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.renameCalled != 0 {
		t.Fatalf("renameCalled=%d", store.renameCalled)
	}
}

func TestHandleOrgNodes_POST_Rename_InvalidOrgCode(t *testing.T) {
	store := &writeSpyStore{}
	body := bytes.NewBufferString("action=rename&org_code=bad%7F&new_name=New&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "org_code invalid") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleOrgNodes_POST_Rename_OrgCodeNotFound(t *testing.T) {
	store := newOrgUnitMemoryStore()
	body := bytes.NewBufferString("action=rename&org_code=ORG-404&new_name=New&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "org_code not found") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleOrgNodes_POST_Rename_MissingNewName(t *testing.T) {
	store := &writeSpyStore{}
	body := bytes.NewBufferString("action=rename&org_code=ORG-1&new_name=&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.renameCalled != 0 {
		t.Fatalf("renameCalled=%d", store.renameCalled)
	}
}

func TestHandleOrgNodes_GET_DefaultAsOf_UsesToday(t *testing.T) {
	store := &asOfSpyStore{}
	req := httptest.NewRequest(http.MethodGet, "/org/nodes", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "/org/nodes?tree_as_of=") {
		t.Fatalf("location=%q", loc)
	}
	wantAsOf := time.Now().UTC().Format("2006-01-02")
	if !strings.Contains(loc, wantAsOf) {
		t.Fatalf("location=%q wantAsOf=%q", loc, wantAsOf)
	}
}

func TestHandleOrgNodes_MethodNotAllowed(t *testing.T) {
	store := newOrgUnitMemoryStore()
	req := httptest.NewRequest(http.MethodPut, "/org/nodes?tree_as_of=2026-01-06", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgNodes_POST_MissingTreeAsOf(t *testing.T) {
	store := newOrgUnitMemoryStore()
	req := httptest.NewRequest(http.MethodPost, "/org/nodes", strings.NewReader("org_code=A001&name=Root"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "tree_as_of") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleOrgNodes_CreateMissingOrgCode(t *testing.T) {
	store := newOrgUnitMemoryStore()
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-06", strings.NewReader("name=A"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "org_code is required") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleOrgNodes_TenantMissing(t *testing.T) {
	store := newOrgUnitMemoryStore()
	req := httptest.NewRequest(http.MethodGet, "/org/nodes?tree_as_of=2026-01-01", nil)
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestRenderOrgNodes(t *testing.T) {
	out := renderOrgNodes(nil, Tenant{Name: "T"}, "", "2026-01-06", true)
	if out == "" {
		t.Fatal("expected output")
	}
	out2 := renderOrgNodes([]OrgUnitNode{{ID: "1", OrgCode: "N001", Name: "N", IsBusinessUnit: true}}, Tenant{Name: "T"}, "err", "2026-01-06", true)
	if out2 == "" {
		t.Fatal("expected output")
	}
	if !strings.Contains(out2, "(BU)") {
		t.Fatalf("unexpected output: %q", out2)
	}
	out3 := renderOrgNodes([]OrgUnitNode{{ID: "2", Name: "MissingCode"}}, Tenant{Name: "T"}, "", "2026-01-06", false)
	if !strings.Contains(out3, "(missing org_code)") {
		t.Fatalf("unexpected output: %q", out3)
	}
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

func TestHandleOrgNodes_RecordActions(t *testing.T) {
	baseVersions := []OrgUnitNodeVersion{
		{EventID: 1, EffectiveDate: "2026-01-01", EventType: "RENAME"},
		{EventID: 2, EffectiveDate: "2026-01-10", EventType: "RENAME"},
	}
	tripleVersions := []OrgUnitNodeVersion{
		{EventID: 1, EffectiveDate: "2026-01-01", EventType: "RENAME"},
		{EventID: 2, EffectiveDate: "2026-01-10", EventType: "RENAME"},
		{EventID: 3, EffectiveDate: "2026-01-20", EventType: "RENAME"},
	}
	versionsWithEmpty := []OrgUnitNodeVersion{
		{EventID: 1, EffectiveDate: "", EventType: "RENAME"},
		{EventID: 2, EffectiveDate: "2026-01-05", EventType: "RENAME"},
		{EventID: 3, EffectiveDate: "2026-01-10", EventType: "RENAME"},
	}

	t.Run("add_record missing effective_date", func(t *testing.T) {
		store := &recordActionStore{versions: baseVersions}
		rec := postOrgNodesForm(t, store, "action=add_record&org_code=A001")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "effective_date is required") {
			t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
		}
	})
	t.Run("add_record invalid effective_date", func(t *testing.T) {
		store := &recordActionStore{versions: baseVersions}
		rec := postOrgNodesForm(t, store, "action=add_record&org_code=A001&effective_date=bad")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "effective_date 无效") {
			t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
		}
	})
	t.Run("add_record invalid org_code", func(t *testing.T) {
		store := &recordActionStore{versions: baseVersions}
		store.resolveFn = func(context.Context, string, string) (int, error) {
			return 0, orgunitpkg.ErrOrgCodeInvalid
		}
		rec := postOrgNodesForm(t, store, "action=add_record&org_code=BAD&effective_date=2026-01-11")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "org_code invalid") {
			t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
		}
	})
	t.Run("add_record versions error", func(t *testing.T) {
		store := &recordActionStore{versionsErr: errors.New("versions")}
		rec := postOrgNodesForm(t, store, "action=add_record&org_code=A001&effective_date=2026-01-11")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "versions") {
			t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
		}
	})
	t.Run("add_record no versions", func(t *testing.T) {
		store := &recordActionStore{versions: []OrgUnitNodeVersion{}}
		rec := postOrgNodesForm(t, store, "action=add_record&org_code=A001&effective_date=2026-01-11")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "no versions found") {
			t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
		}
	})
	t.Run("add_record conflict", func(t *testing.T) {
		store := &recordActionStore{versions: versionsWithEmpty}
		rec := postOrgNodesForm(t, store, "action=add_record&org_code=A001&effective_date=2026-01-10")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "effective_date conflict") {
			t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
		}
	})
	t.Run("insert_record invalid current_effective_date", func(t *testing.T) {
		store := &recordActionStore{versions: baseVersions}
		rec := postOrgNodesForm(t, store, "action=insert_record&org_code=A001&effective_date=2026-01-05&current_effective_date=bad")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "current_effective_date 无效") {
			t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
		}
	})
	t.Run("insert_record selected record not found", func(t *testing.T) {
		store := &recordActionStore{versions: baseVersions}
		rec := postOrgNodesForm(t, store, "action=insert_record&org_code=A001&effective_date=2026-01-05&current_effective_date=2026-01-03")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "record not found") {
			t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
		}
	})
	t.Run("insert_record without current_effective_date", func(t *testing.T) {
		store := &recordActionStore{versions: baseVersions}
		rec := postOrgNodesForm(t, store, "action=insert_record&org_code=A001&effective_date=2026-01-11")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "effective_date must be between existing records") {
			t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
		}
	})
	t.Run("insert_record date exists", func(t *testing.T) {
		store := &recordActionStore{versions: baseVersions}
		rec := postOrgNodesForm(t, store, "action=insert_record&org_code=A001&effective_date=2026-01-01&current_effective_date=2026-01-10")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "effective_date conflict") {
			t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
		}
	})
	t.Run("insert_record last record conflict", func(t *testing.T) {
		store := &recordActionStore{versions: baseVersions}
		rec := postOrgNodesForm(t, store, "action=insert_record&org_code=A001&effective_date=2026-01-05&current_effective_date=2026-01-10")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "effective_date conflict") {
			t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
		}
	})
	t.Run("insert_record out of range", func(t *testing.T) {
		store := &recordActionStore{versions: baseVersions}
		rec := postOrgNodesForm(t, store, "action=insert_record&org_code=A001&effective_date=2026-01-11&current_effective_date=2026-01-01")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "effective_date must be between existing records") {
			t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
		}
	})
	t.Run("insert_record before prev range", func(t *testing.T) {
		store := &recordActionStore{versions: tripleVersions}
		rec := postOrgNodesForm(t, store, "action=insert_record&org_code=A001&effective_date=2025-12-31&current_effective_date=2026-01-10")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "effective_date must be between existing records") {
			t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
		}
	})
	t.Run("insert_record backdate earliest uses regular insert", func(t *testing.T) {
		store := &recordActionStore{versions: baseVersions}
		rec := postOrgNodesForm(t, store, "action=insert_record&org_code=A001&effective_date=2025-12-31&current_effective_date=2026-01-01")
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
		if store.renameCalled != 1 {
			t.Fatalf("rename called=%d", store.renameCalled)
		}
	})
	t.Run("delete_record last record", func(t *testing.T) {
		store := &recordActionStore{versions: []OrgUnitNodeVersion{{EventID: 1, EffectiveDate: "2026-01-01"}}}
		rec := postOrgNodesForm(t, store, "action=delete_record&org_code=A001&effective_date=2026-01-01")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "cannot delete last record") {
			t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
		}
	})
	t.Run("delete_record not found", func(t *testing.T) {
		store := &recordActionStore{versions: baseVersions}
		rec := postOrgNodesForm(t, store, "action=delete_record&org_code=A001&effective_date=2026-01-05")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "record not found") {
			t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
		}
	})
	t.Run("delete_record disable error", func(t *testing.T) {
		store := &recordActionStore{versions: baseVersions}
		store.disableErr = errors.New("disable")
		rec := postOrgNodesForm(t, store, "action=delete_record&org_code=A001&effective_date=2026-01-01")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "disable") {
			t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
		}
	})
	t.Run("delete_record success", func(t *testing.T) {
		store := &recordActionStore{versions: baseVersions}
		rec := postOrgNodesForm(t, store, "action=delete_record&org_code=A001&effective_date=2026-01-01")
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
		if store.disableCalled != 1 {
			t.Fatalf("disable called=%d", store.disableCalled)
		}
		if loc := rec.Header().Get("Location"); loc != "/org/nodes?tree_as_of=2026-01-06" {
			t.Fatalf("location=%q", loc)
		}
	})
	t.Run("add_record details error", func(t *testing.T) {
		store := &recordActionStore{versions: baseVersions, detailsErr: errors.New("details")}
		rec := postOrgNodesForm(t, store, "action=add_record&org_code=A001&effective_date=2026-01-11")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "details") {
			t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
		}
	})
	t.Run("add_record name required after details", func(t *testing.T) {
		store := &recordActionStore{versions: baseVersions, details: OrgUnitNodeDetails{OrgID: 10000001, OrgCode: "A001", Name: ""}}
		rec := postOrgNodesForm(t, store, "action=add_record&org_code=A001&effective_date=2026-01-11")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "name is required") {
			t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
		}
	})
	t.Run("add_record rename error", func(t *testing.T) {
		store := &recordActionStore{versions: baseVersions}
		store.renameErr = errors.New("rename")
		rec := postOrgNodesForm(t, store, "action=add_record&org_code=A001&effective_date=2026-01-11&name=NewName")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "rename") {
			t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
		}
	})
	t.Run("add_record success uses details name", func(t *testing.T) {
		store := &recordActionStore{versions: baseVersions, details: OrgUnitNodeDetails{OrgID: 10000001, OrgCode: "A001", Name: "FromDetails"}}
		rec := postOrgNodesForm(t, store, "action=add_record&org_code=A001&effective_date=2026-01-11")
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
		if store.renameCalled != 1 {
			t.Fatalf("rename called=%d", store.renameCalled)
		}
		if loc := rec.Header().Get("Location"); loc != "/org/nodes?tree_as_of=2026-01-06" {
			t.Fatalf("location=%q", loc)
		}
	})
	t.Run("add_record invalid change_type", func(t *testing.T) {
		store := &recordActionStore{versions: baseVersions}
		rec := postOrgNodesForm(t, store, "action=add_record&org_code=A001&effective_date=2026-01-11&record_change_type=bad")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "record_change_type invalid") {
			t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
		}
	})
	t.Run("add_record move invalid parent_org_code", func(t *testing.T) {
		store := &recordActionStore{versions: baseVersions}
		store.resolveFn = func(_ context.Context, _ string, orgCode string) (int, error) {
			if orgCode == "BAD" {
				return 0, orgunitpkg.ErrOrgCodeNotFound
			}
			return 10000001, nil
		}
		rec := postOrgNodesForm(t, store, "action=add_record&org_code=A001&effective_date=2026-01-11&record_change_type=move&parent_org_code=BAD")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "parent_org_code not found") {
			t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
		}
	})
	t.Run("add_record move success allows root", func(t *testing.T) {
		store := &recordActionStore{versions: baseVersions, detailsErr: errors.New("details")}
		rec := postOrgNodesForm(t, store, "action=add_record&org_code=A001&effective_date=2026-01-11&record_change_type=move")
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
		if store.moveCalled != 1 {
			t.Fatalf("move called=%d", store.moveCalled)
		}
		if got := strings.Join(store.moveArgs, "|"); got != "t1|2026-01-11|10000001|" {
			t.Fatalf("unexpected move args: %q", got)
		}
	})
	t.Run("add_record move error", func(t *testing.T) {
		store := &recordActionStore{versions: baseVersions}
		store.moveErr = errors.New("move")
		rec := postOrgNodesForm(t, store, "action=add_record&org_code=A001&effective_date=2026-01-11&record_change_type=move&parent_org_code=PARENT")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "move") {
			t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
		}
	})
	t.Run("add_record set_business_unit invalid value", func(t *testing.T) {
		store := &recordActionStore{versions: baseVersions}
		rec := postOrgNodesForm(t, store, "action=add_record&org_code=A001&effective_date=2026-01-11&record_change_type=set_business_unit&is_business_unit=maybe")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "is_business_unit 无效") {
			t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
		}
	})
	t.Run("add_record set_business_unit error", func(t *testing.T) {
		store := &recordActionStore{versions: baseVersions}
		store.setBusinessUnitErr = errors.New("set bu")
		rec := postOrgNodesForm(t, store, "action=add_record&org_code=A001&effective_date=2026-01-11&record_change_type=set_business_unit&is_business_unit=true")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "set bu") {
			t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
		}
	})
	t.Run("add_record set_business_unit success", func(t *testing.T) {
		store := &recordActionStore{versions: baseVersions}
		rec := postOrgNodesForm(t, store, "action=add_record&org_code=A001&effective_date=2026-01-11&record_change_type=set_business_unit&is_business_unit=true")
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
		if store.setBusinessUnitCalled != 1 {
			t.Fatalf("set_business_unit called=%d", store.setBusinessUnitCalled)
		}
		if got := strings.Join(store.setBusinessUnitArgs, "|"); got != "t1|2026-01-11|10000001" {
			t.Fatalf("unexpected set_business_unit args: %q", got)
		}
		if !store.setBusinessUnitValue {
			t.Fatal("expected set_business_unit value true")
		}
		if store.setBusinessUnitReqID != "ui:orgunit:record:set-business-unit:10000001:2026-01-11" {
			t.Fatalf("unexpected request id: %q", store.setBusinessUnitReqID)
		}
	})
}
