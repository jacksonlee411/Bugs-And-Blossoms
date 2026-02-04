package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

type arrayRow struct {
	vals []any
	err  error
}

func (r arrayRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i := range dest {
		if i >= len(r.vals) || r.vals[i] == nil {
			continue
		}
		switch d := dest[i].(type) {
		case *int:
			*d = r.vals[i].(int)
		case *string:
			*d = r.vals[i].(string)
		case *[]int:
			*d = append([]int(nil), r.vals[i].([]int)...)
		case *bool:
			*d = r.vals[i].(bool)
		default:
			return errors.New("unsupported scan type")
		}
	}
	return nil
}

type orgUnitReadStoreStub struct {
	*orgUnitMemoryStore
	listChildrenFn func(ctx context.Context, tenantID string, parentID int, asOfDate string) ([]OrgUnitChild, error)
	detailsFn      func(ctx context.Context, tenantID string, orgID int, asOfDate string) (OrgUnitNodeDetails, error)
	searchFn       func(ctx context.Context, tenantID string, query string, asOfDate string) (OrgUnitSearchResult, error)
}

func (s *orgUnitReadStoreStub) ListChildren(ctx context.Context, tenantID string, parentID int, asOfDate string) ([]OrgUnitChild, error) {
	if s.listChildrenFn != nil {
		return s.listChildrenFn(ctx, tenantID, parentID, asOfDate)
	}
	return []OrgUnitChild{}, nil
}

func (s *orgUnitReadStoreStub) GetNodeDetails(ctx context.Context, tenantID string, orgID int, asOfDate string) (OrgUnitNodeDetails, error) {
	if s.detailsFn != nil {
		return s.detailsFn(ctx, tenantID, orgID, asOfDate)
	}
	return OrgUnitNodeDetails{}, nil
}

func (s *orgUnitReadStoreStub) SearchNode(ctx context.Context, tenantID string, query string, asOfDate string) (OrgUnitSearchResult, error) {
	if s.searchFn != nil {
		return s.searchFn(ctx, tenantID, query, asOfDate)
	}
	return OrgUnitSearchResult{}, nil
}

func TestOrgUnitMemoryStore_ListChildren(t *testing.T) {
	s := newOrgUnitMemoryStore()
	node, err := s.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A001", "Root", "", false)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := strconv.Atoi(node.ID)
	children, err := s.ListChildren(context.Background(), "t1", id, "2026-01-06")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(children) != 0 {
		t.Fatalf("expected empty children, got %d", len(children))
	}
	if _, err := s.ListChildren(context.Background(), "t1", id+1, "2026-01-06"); !errors.Is(err, errOrgUnitNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestOrgUnitMemoryStore_GetNodeDetails(t *testing.T) {
	s := newOrgUnitMemoryStore()
	node, err := s.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A001", "Root", "", true)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := strconv.Atoi(node.ID)
	details, err := s.GetNodeDetails(context.Background(), "t1", id, "2026-01-06")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if details.OrgCode != "A001" || details.Name != "Root" || !details.IsBusinessUnit {
		t.Fatalf("unexpected details: %#v", details)
	}
	if _, err := s.GetNodeDetails(context.Background(), "t1", id+1, "2026-01-06"); !errors.Is(err, errOrgUnitNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestOrgUnitMemoryStore_SearchNode(t *testing.T) {
	s := newOrgUnitMemoryStore()
	node, err := s.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A001", "Root", "", false)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := strconv.Atoi(node.ID)
	if _, err := s.SearchNode(context.Background(), "t1", " ", "2026-01-06"); err == nil {
		t.Fatal("expected query required error")
	}
	res, err := s.SearchNode(context.Background(), "t1", "A001", "2026-01-06")
	if err != nil || res.TargetOrgID != id {
		t.Fatalf("expected code match, got %#v err=%v", res, err)
	}
	res, err = s.SearchNode(context.Background(), "t1", "roo", "2026-01-06")
	if err != nil || res.TargetOrgID != id {
		t.Fatalf("expected name match, got %#v err=%v", res, err)
	}
	if _, err := s.SearchNode(context.Background(), "t1", "missing", "2026-01-06"); !errors.Is(err, errOrgUnitNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}

	t.Run("invalid id in store", func(t *testing.T) {
		s := newOrgUnitMemoryStore()
		s.nodes["t1"] = []OrgUnitNode{{ID: "bad", OrgCode: "A001", Name: "A001"}}
		if _, err := s.SearchNode(context.Background(), "t1", "A001", "2026-01-06"); !errors.Is(err, errOrgUnitNotFound) {
			t.Fatalf("expected not found, got %v", err)
		}
	})
}

func TestOrgUnitPGStore_ListChildren(t *testing.T) {
	ctx := context.Background()
	tx := &stubTx{
		row:  &stubRow{vals: []any{true}},
		rows: &stubRows{},
	}
	store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return tx, nil
	}))
	children, err := store.ListChildren(ctx, "t1", 10000001, "2026-01-06")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(children) != 1 {
		t.Fatalf("expected child, got %d", len(children))
	}
}

func TestOrgUnitPGStore_ListChildren_Errors(t *testing.T) {
	ctx := context.Background()
	t.Run("begin", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		if _, err := store.ListChildren(ctx, "t1", 10000001, "2026-01-06"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("exec", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		if _, err := store.ListChildren(ctx, "t1", 10000001, "2026-01-06"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("exists error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("row")}, nil
		}))
		if _, err := store.ListChildren(ctx, "t1", 10000001, "2026-01-06"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("not found", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{row: &stubRow{vals: []any{false}}}, nil
		}))
		if _, err := store.ListChildren(ctx, "t1", 10000001, "2026-01-06"); !errors.Is(err, errOrgUnitNotFound) {
			t.Fatalf("expected not found, got %v", err)
		}
	})
	t.Run("query error", func(t *testing.T) {
		tx := &stubTx{row: &stubRow{vals: []any{true}}, queryErr: errors.New("query")}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, err := store.ListChildren(ctx, "t1", 10000001, "2026-01-06"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("scan error", func(t *testing.T) {
		tx := &stubTx{row: &stubRow{vals: []any{true}}, rows: &stubRows{scanErr: errors.New("scan")}}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, err := store.ListChildren(ctx, "t1", 10000001, "2026-01-06"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("rows error", func(t *testing.T) {
		tx := &stubTx{row: &stubRow{vals: []any{true}}, rows: &stubRows{err: errors.New("rows")}}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, err := store.ListChildren(ctx, "t1", 10000001, "2026-01-06"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("commit error", func(t *testing.T) {
		tx := &stubTx{row: &stubRow{vals: []any{true}}, rows: &stubRows{}, commitErr: errors.New("commit")}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, err := store.ListChildren(ctx, "t1", 10000001, "2026-01-06"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestOrgUnitPGStore_GetNodeDetails(t *testing.T) {
	ctx := context.Background()
	tx := &stubTx{
		row: &stubRow{vals: []any{10000001, "A001", "Root", 0, "", "", true, "1001", "Manager"}},
	}
	store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return tx, nil
	}))
	details, err := store.GetNodeDetails(ctx, "t1", 10000001, "2026-01-06")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if details.OrgCode != "A001" || details.Name != "Root" || details.ManagerPernr != "1001" {
		t.Fatalf("unexpected details: %#v", details)
	}
}

func TestOrgUnitPGStore_GetNodeDetails_Errors(t *testing.T) {
	ctx := context.Background()
	t.Run("begin", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		if _, err := store.GetNodeDetails(ctx, "t1", 10000001, "2026-01-06"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("exec", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		if _, err := store.GetNodeDetails(ctx, "t1", 10000001, "2026-01-06"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("not found", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: pgx.ErrNoRows}, nil
		}))
		if _, err := store.GetNodeDetails(ctx, "t1", 10000001, "2026-01-06"); !errors.Is(err, errOrgUnitNotFound) {
			t.Fatalf("expected not found, got %v", err)
		}
	})
	t.Run("row error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("row")}, nil
		}))
		if _, err := store.GetNodeDetails(ctx, "t1", 10000001, "2026-01-06"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("commit error", func(t *testing.T) {
		tx := &stubTx{row: &stubRow{vals: []any{10000001, "A001", "Root", 0, "", "", false, "", ""}}, commitErr: errors.New("commit")}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, err := store.GetNodeDetails(ctx, "t1", 10000001, "2026-01-06"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestOrgUnitPGStore_SearchNode(t *testing.T) {
	ctx := context.Background()
	t.Run("query required", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		if _, err := store.SearchNode(ctx, "t1", " ", "2026-01-06"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("begin error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		if _, err := store.SearchNode(ctx, "t1", "A001", "2026-01-06"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("exec error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		if _, err := store.SearchNode(ctx, "t1", "A001", "2026-01-06"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("code error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("row")}, nil
		}))
		if _, err := store.SearchNode(ctx, "t1", "A001", "2026-01-06"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("not found", func(t *testing.T) {
		tx := &stubTx{rowErr: pgx.ErrNoRows, row2Err: pgx.ErrNoRows}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, err := store.SearchNode(ctx, "t1", "A001", "2026-01-06"); !errors.Is(err, errOrgUnitNotFound) {
			t.Fatalf("expected not found, got %v", err)
		}
	})
	t.Run("name error", func(t *testing.T) {
		tx := &stubTx{row: &stubRow{err: pgx.ErrNoRows}, row2Err: errors.New("row2")}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, err := store.SearchNode(ctx, "t1", "A001", "2026-01-06"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("invalid code uses name search", func(t *testing.T) {
		tx := &stubTx{row: arrayRow{vals: []any{10000003, "C003", "Other", []int{10000003}}}}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		res, err := store.SearchNode(ctx, "t1", "A\n1", "2026-01-06")
		if err != nil || res.TargetOrgID != 10000003 {
			t.Fatalf("unexpected result: %#v err=%v", res, err)
		}
	})
	t.Run("code success", func(t *testing.T) {
		tx := &stubTx{row: arrayRow{vals: []any{10000001, "A001", "Root", []int{10000001}}}}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		res, err := store.SearchNode(ctx, "t1", "A001", "2026-01-06")
		if err != nil || res.TargetOrgID != 10000001 {
			t.Fatalf("unexpected result: %#v err=%v", res, err)
		}
	})
	t.Run("name success", func(t *testing.T) {
		tx := &stubTx{
			row:  &stubRow{err: pgx.ErrNoRows},
			row2: arrayRow{vals: []any{10000002, "B002", "Child", []int{10000001, 10000002}}},
		}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		res, err := store.SearchNode(ctx, "t1", "Child", "2026-01-06")
		if err != nil || res.TargetOrgID != 10000002 {
			t.Fatalf("unexpected result: %#v err=%v", res, err)
		}
	})
	t.Run("commit error", func(t *testing.T) {
		tx := &stubTx{
			row:       arrayRow{vals: []any{10000001, "A001", "Root", []int{10000001}}},
			commitErr: errors.New("commit"),
		}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, err := store.SearchNode(ctx, "t1", "A001", "2026-01-06"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestHandleOrgNodeChildren(t *testing.T) {
	t.Run("missing tenant", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/children?as_of=2026-01-06&parent_id=10000001", nil)
		rec := httptest.NewRecorder()
		handleOrgNodeChildren(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/nodes/children?as_of=2026-01-06&parent_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeChildren(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("invalid as_of", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/children?as_of=bad&parent_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeChildren(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("missing parent", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/children?as_of=2026-01-06", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeChildren(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("invalid parent", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/children?as_of=2026-01-06&parent_id=bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeChildren(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("not found", func(t *testing.T) {
		store := &orgUnitReadStoreStub{
			orgUnitMemoryStore: newOrgUnitMemoryStore(),
			listChildrenFn: func(context.Context, string, int, string) ([]OrgUnitChild, error) {
				return nil, errOrgUnitNotFound
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/children?as_of=2026-01-06&parent_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeChildren(rec, req, store)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("store error", func(t *testing.T) {
		store := &orgUnitReadStoreStub{
			orgUnitMemoryStore: newOrgUnitMemoryStore(),
			listChildrenFn: func(context.Context, string, int, string) ([]OrgUnitChild, error) {
				return nil, errors.New("boom")
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/children?as_of=2026-01-06&parent_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeChildren(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("success", func(t *testing.T) {
		store := &orgUnitReadStoreStub{
			orgUnitMemoryStore: newOrgUnitMemoryStore(),
			listChildrenFn: func(context.Context, string, int, string) ([]OrgUnitChild, error) {
				return []OrgUnitChild{{OrgID: 10000001, OrgCode: "A001", Name: "Root", HasChildren: true}}, nil
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/children?as_of=2026-01-06&parent_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeChildren(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, `data-org-id="10000001"`) || !strings.Contains(body, `lazy`) {
			t.Fatalf("unexpected body: %q", body)
		}
	})
}

func TestHandleOrgNodeDetails(t *testing.T) {
	t.Run("missing tenant", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/details?as_of=2026-01-06&org_id=10000001", nil)
		rec := httptest.NewRecorder()
		handleOrgNodeDetails(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/nodes/details?as_of=2026-01-06&org_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetails(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("invalid as_of", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/details?as_of=bad&org_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetails(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("missing org_id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/details?as_of=2026-01-06", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetails(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("invalid org_id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/details?as_of=2026-01-06&org_id=bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetails(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("not found", func(t *testing.T) {
		store := &orgUnitReadStoreStub{
			orgUnitMemoryStore: newOrgUnitMemoryStore(),
			detailsFn: func(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
				return OrgUnitNodeDetails{}, errOrgUnitNotFound
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/details?as_of=2026-01-06&org_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetails(rec, req, store)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("store error", func(t *testing.T) {
		store := &orgUnitReadStoreStub{
			orgUnitMemoryStore: newOrgUnitMemoryStore(),
			detailsFn: func(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
				return OrgUnitNodeDetails{}, errors.New("boom")
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/details?as_of=2026-01-06&org_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetails(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("success", func(t *testing.T) {
		store := &orgUnitReadStoreStub{
			orgUnitMemoryStore: newOrgUnitMemoryStore(),
			detailsFn: func(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
				return OrgUnitNodeDetails{OrgID: 10000001, OrgCode: "A001", Name: "Root", IsBusinessUnit: true, ManagerPernr: "1001", ManagerName: "Manager"}, nil
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/details?as_of=2026-01-06&org_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetails(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "A001") || !strings.Contains(body, "Manager") {
			t.Fatalf("unexpected body: %q", body)
		}
	})
	t.Run("success with parent", func(t *testing.T) {
		store := &orgUnitReadStoreStub{
			orgUnitMemoryStore: newOrgUnitMemoryStore(),
			detailsFn: func(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
				return OrgUnitNodeDetails{
					OrgID:      10000002,
					OrgCode:    "B002",
					Name:       "Child",
					ParentID:   10000001,
					ParentCode: "A001",
					ParentName: "Root",
				}, nil
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/details?as_of=2026-01-06&org_id=10000002", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetails(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if body := rec.Body.String(); !strings.Contains(body, "A001") || !strings.Contains(body, "Root") {
			t.Fatalf("unexpected body: %q", body)
		}
	})
	t.Run("success with parent name only and empty manager label", func(t *testing.T) {
		store := &orgUnitReadStoreStub{
			orgUnitMemoryStore: newOrgUnitMemoryStore(),
			detailsFn: func(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
				return OrgUnitNodeDetails{
					OrgID:        10000003,
					OrgCode:      "C003",
					Name:         "Leaf",
					ParentID:     10000002,
					ParentName:   "Parent Only",
					ManagerPernr: " ",
				}, nil
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/details?as_of=2026-01-06&org_id=10000003", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetails(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if body := rec.Body.String(); !strings.Contains(body, "Parent Only") {
			t.Fatalf("unexpected body: %q", body)
		}
	})
}

func TestHandleOrgNodeSearch(t *testing.T) {
	t.Run("missing tenant", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/search?as_of=2026-01-06&query=A001", nil)
		rec := httptest.NewRecorder()
		handleOrgNodeSearch(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/nodes/search?as_of=2026-01-06&query=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeSearch(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("invalid as_of", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/search?as_of=bad&query=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeSearch(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("missing query", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/search?as_of=2026-01-06", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeSearch(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("not found", func(t *testing.T) {
		store := &orgUnitReadStoreStub{
			orgUnitMemoryStore: newOrgUnitMemoryStore(),
			searchFn: func(context.Context, string, string, string) (OrgUnitSearchResult, error) {
				return OrgUnitSearchResult{}, errOrgUnitNotFound
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/search?as_of=2026-01-06&query=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeSearch(rec, req, store)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("store error", func(t *testing.T) {
		store := &orgUnitReadStoreStub{
			orgUnitMemoryStore: newOrgUnitMemoryStore(),
			searchFn: func(context.Context, string, string, string) (OrgUnitSearchResult, error) {
				return OrgUnitSearchResult{}, errors.New("boom")
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/search?as_of=2026-01-06&query=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeSearch(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("success", func(t *testing.T) {
		store := &orgUnitReadStoreStub{
			orgUnitMemoryStore: newOrgUnitMemoryStore(),
			searchFn: func(context.Context, string, string, string) (OrgUnitSearchResult, error) {
				return OrgUnitSearchResult{TargetOrgID: 10000001, TargetOrgCode: "A001", TargetName: "Root", PathOrgIDs: []int{10000001}, AsOf: "2026-01-06"}, nil
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/search?as_of=2026-01-06&query=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeSearch(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		var out OrgUnitSearchResult
		if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		if out.TargetOrgID != 10000001 {
			t.Fatalf("unexpected result: %#v", out)
		}
	})
}
