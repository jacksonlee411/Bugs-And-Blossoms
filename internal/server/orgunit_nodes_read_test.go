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
	listChildrenFn     func(ctx context.Context, tenantID string, parentID int, asOfDate string) ([]OrgUnitChild, error)
	detailsFn          func(ctx context.Context, tenantID string, orgID int, asOfDate string) (OrgUnitNodeDetails, error)
	searchFn           func(ctx context.Context, tenantID string, query string, asOfDate string) (OrgUnitSearchResult, error)
	searchCandidatesFn func(ctx context.Context, tenantID string, query string, asOfDate string, limit int) ([]OrgUnitSearchCandidate, error)
	listVersionsFn     func(ctx context.Context, tenantID string, orgID int) ([]OrgUnitNodeVersion, error)
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

func (s *orgUnitReadStoreStub) SearchNodeCandidates(ctx context.Context, tenantID string, query string, asOfDate string, limit int) ([]OrgUnitSearchCandidate, error) {
	if s.searchCandidatesFn != nil {
		return s.searchCandidatesFn(ctx, tenantID, query, asOfDate, limit)
	}
	return []OrgUnitSearchCandidate{}, nil
}

func (s *orgUnitReadStoreStub) ListNodeVersions(ctx context.Context, tenantID string, orgID int) ([]OrgUnitNodeVersion, error) {
	if s.listVersionsFn != nil {
		return s.listVersionsFn(ctx, tenantID, orgID)
	}
	return []OrgUnitNodeVersion{}, nil
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

func TestOrgUnitMemoryStore_SearchNodeCandidates(t *testing.T) {
	ctx := context.Background()
	store := newOrgUnitMemoryStore()
	node, err := store.CreateNodeCurrent(ctx, "t1", "2026-01-06", "A001", "Root", "", false)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := strconv.Atoi(node.ID)

	if _, err := store.SearchNodeCandidates(ctx, "t1", " ", "2026-01-06", 5); err == nil {
		t.Fatal("expected query required error")
	}

	res, err := store.SearchNodeCandidates(ctx, "t1", "A001", "2026-01-06", 5)
	if err != nil || len(res) != 1 || res[0].OrgID != id {
		t.Fatalf("unexpected code match: %#v err=%v", res, err)
	}

	res, err = store.SearchNodeCandidates(ctx, "t1", "roo", "2026-01-06", 0)
	if err != nil || len(res) != 1 || res[0].OrgID != id {
		t.Fatalf("unexpected name match: %#v err=%v", res, err)
	}

	if _, err := store.SearchNodeCandidates(ctx, "t1", "missing", "2026-01-06", 5); !errors.Is(err, errOrgUnitNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}

	t.Run("invalid code id falls back to not found", func(t *testing.T) {
		s := newOrgUnitMemoryStore()
		s.nodes["t1"] = []OrgUnitNode{{ID: "bad", OrgCode: "A001", Name: "Other"}}
		if _, err := s.SearchNodeCandidates(ctx, "t1", "A001", "2026-01-06", 5); !errors.Is(err, errOrgUnitNotFound) {
			t.Fatalf("expected not found, got %v", err)
		}
	})

	t.Run("name search skips invalid ids and respects limit", func(t *testing.T) {
		s := newOrgUnitMemoryStore()
		s.nodes["t1"] = []OrgUnitNode{
			{ID: "bad", OrgCode: "B001", Name: "Alpha"},
			{ID: "10000002", OrgCode: "B002", Name: "Alpha Two"},
			{ID: "10000003", OrgCode: "B003", Name: "Alpha Three"},
		}
		items, err := s.SearchNodeCandidates(ctx, "t1", "alp", "2026-01-06", 1)
		if err != nil || len(items) != 1 || items[0].OrgID != 10000002 {
			t.Fatalf("unexpected result: %#v err=%v", items, err)
		}
	})
}

func TestOrgUnitMemoryStore_ListNodeVersions(t *testing.T) {
	ctx := context.Background()
	store := newOrgUnitMemoryStore()
	node, err := store.CreateNodeCurrent(ctx, "t1", "2026-01-06", "A001", "Root", "", false)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := strconv.Atoi(node.ID)
	versions, err := store.ListNodeVersions(ctx, "t1", id)
	if err != nil || len(versions) != 1 {
		t.Fatalf("unexpected versions: %#v err=%v", versions, err)
	}
	if _, err := store.ListNodeVersions(ctx, "t1", id+1); !errors.Is(err, errOrgUnitNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
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

func TestOrgUnitPGStore_SearchNodeCandidates(t *testing.T) {
	ctx := context.Background()
	t.Run("query required", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		if _, err := store.SearchNodeCandidates(ctx, "t1", " ", "2026-01-06", 5); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("begin error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		if _, err := store.SearchNodeCandidates(ctx, "t1", "A001", "2026-01-06", 5); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("exec error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		if _, err := store.SearchNodeCandidates(ctx, "t1", "A001", "2026-01-06", 5); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("code query error", func(t *testing.T) {
		tx := &stubTx{queryErr: errors.New("query")}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, err := store.SearchNodeCandidates(ctx, "t1", "A001", "2026-01-06", 5); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("code scan error", func(t *testing.T) {
		tx := &stubTx{rows: &stubRows{scanErr: errors.New("scan")}}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, err := store.SearchNodeCandidates(ctx, "t1", "A001", "2026-01-06", 5); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("code rows error", func(t *testing.T) {
		tx := &stubTx{rows: &stubRows{err: errors.New("rows")}}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, err := store.SearchNodeCandidates(ctx, "t1", "A001", "2026-01-06", 5); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("code commit error", func(t *testing.T) {
		tx := &stubTx{rows: &stubRows{}, commitErr: errors.New("commit")}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, err := store.SearchNodeCandidates(ctx, "t1", "A001", "2026-01-06", 5); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("code success", func(t *testing.T) {
		tx := &stubTx{rows: &stubRows{}}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		out, err := store.SearchNodeCandidates(ctx, "t1", "A001", "2026-01-06", 0)
		if err != nil || len(out) != 1 {
			t.Fatalf("unexpected result: %#v err=%v", out, err)
		}
	})
	t.Run("name query error", func(t *testing.T) {
		tx := &stubTx{
			rows:       &stubRows{empty: true},
			queryErr:   errors.New("query"),
			queryErrAt: 2,
		}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, err := store.SearchNodeCandidates(ctx, "t1", "A001", "2026-01-06", 5); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("name scan error", func(t *testing.T) {
		tx := &stubTx{rows: &stubRows{empty: true}, rows2: &stubRows{scanErr: errors.New("scan")}}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, err := store.SearchNodeCandidates(ctx, "t1", "A001", "2026-01-06", 5); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("name rows error", func(t *testing.T) {
		tx := &stubTx{rows: &stubRows{empty: true}, rows2: &stubRows{err: errors.New("rows")}}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, err := store.SearchNodeCandidates(ctx, "t1", "A001", "2026-01-06", 5); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("name not found", func(t *testing.T) {
		tx := &stubTx{rows: &stubRows{empty: true}, rows2: &stubRows{empty: true}}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, err := store.SearchNodeCandidates(ctx, "t1", "A001", "2026-01-06", 5); !errors.Is(err, errOrgUnitNotFound) {
			t.Fatalf("expected not found, got %v", err)
		}
	})
	t.Run("name commit error", func(t *testing.T) {
		tx := &stubTx{rows: &stubRows{empty: true}, rows2: &stubRows{}, commitErr: errors.New("commit")}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, err := store.SearchNodeCandidates(ctx, "t1", "A001", "2026-01-06", 5); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("name success", func(t *testing.T) {
		tx := &stubTx{rows: &stubRows{empty: true}, rows2: &stubRows{}}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		out, err := store.SearchNodeCandidates(ctx, "t1", "A001", "2026-01-06", 5)
		if err != nil || len(out) != 1 {
			t.Fatalf("unexpected result: %#v err=%v", out, err)
		}
	})
}

func TestOrgUnitPGStore_ListNodeVersions(t *testing.T) {
	ctx := context.Background()
	t.Run("begin error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		if _, err := store.ListNodeVersions(ctx, "t1", 10000001); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("exec error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		if _, err := store.ListNodeVersions(ctx, "t1", 10000001); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("query error", func(t *testing.T) {
		tx := &stubTx{queryErr: errors.New("query")}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, err := store.ListNodeVersions(ctx, "t1", 10000001); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("scan error", func(t *testing.T) {
		tx := &stubTx{rows: &stubRows{scanErr: errors.New("scan")}}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, err := store.ListNodeVersions(ctx, "t1", 10000001); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("rows error", func(t *testing.T) {
		tx := &stubTx{rows: &stubRows{err: errors.New("rows")}}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, err := store.ListNodeVersions(ctx, "t1", 10000001); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("commit error", func(t *testing.T) {
		tx := &stubTx{rows: &stubRows{}, commitErr: errors.New("commit")}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if _, err := store.ListNodeVersions(ctx, "t1", 10000001); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("success", func(t *testing.T) {
		tx := &stubTx{rows: &stubRows{}}
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		versions, err := store.ListNodeVersions(ctx, "t1", 10000001)
		if err != nil || len(versions) != 1 {
			t.Fatalf("unexpected result: %#v err=%v", versions, err)
		}
	})
}

func TestHandleOrgNodeChildren(t *testing.T) {
	t.Run("missing tenant", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/children?tree_as_of=2026-01-06&parent_id=10000001", nil)
		rec := httptest.NewRecorder()
		handleOrgNodeChildren(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/nodes/children?tree_as_of=2026-01-06&parent_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeChildren(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("invalid tree_as_of", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/children?tree_as_of=bad&parent_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeChildren(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("missing parent", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/children?tree_as_of=2026-01-06", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeChildren(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("invalid parent", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/children?tree_as_of=2026-01-06&parent_id=bad", nil)
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
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/children?tree_as_of=2026-01-06&parent_id=10000001", nil)
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
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/children?tree_as_of=2026-01-06&parent_id=10000001", nil)
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
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/children?tree_as_of=2026-01-06&parent_id=10000001", nil)
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
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/details?effective_date=2026-01-06&org_id=10000001", nil)
		rec := httptest.NewRecorder()
		handleOrgNodeDetails(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/nodes/details?effective_date=2026-01-06&org_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetails(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("deprecated as_of", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/details?as_of=2026-01-06&org_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetails(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("invalid effective_date", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/details?effective_date=bad&org_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetails(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("invalid tree_as_of", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/details?effective_date=2026-01-06&org_id=10000001&tree_as_of=bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetails(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("missing org_id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/details?effective_date=2026-01-06", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetails(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("invalid org_id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/details?effective_date=2026-01-06&org_id=bad", nil)
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
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/details?effective_date=2026-01-06&org_id=10000001", nil)
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
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/details?effective_date=2026-01-06&org_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetails(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("versions not found", func(t *testing.T) {
		store := &orgUnitReadStoreStub{
			orgUnitMemoryStore: newOrgUnitMemoryStore(),
			detailsFn: func(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
				return OrgUnitNodeDetails{OrgID: 10000001, OrgCode: "A001", Name: "Root"}, nil
			},
			listVersionsFn: func(context.Context, string, int) ([]OrgUnitNodeVersion, error) {
				return nil, errOrgUnitNotFound
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/details?effective_date=2026-01-06&org_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetails(rec, req, store)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("versions error", func(t *testing.T) {
		store := &orgUnitReadStoreStub{
			orgUnitMemoryStore: newOrgUnitMemoryStore(),
			detailsFn: func(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
				return OrgUnitNodeDetails{OrgID: 10000001, OrgCode: "A001", Name: "Root"}, nil
			},
			listVersionsFn: func(context.Context, string, int) ([]OrgUnitNodeVersion, error) {
				return nil, errors.New("boom")
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/details?effective_date=2026-01-06&org_id=10000001", nil)
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
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/details?effective_date=2026-01-06&org_id=10000001", nil)
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
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/details?effective_date=2026-01-06&org_id=10000002", nil)
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
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/details?effective_date=2026-01-06&org_id=10000003", nil)
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
	t.Run("default effective_date uses today", func(t *testing.T) {
		var gotEffectiveDate string
		store := &orgUnitReadStoreStub{
			orgUnitMemoryStore: newOrgUnitMemoryStore(),
			detailsFn: func(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
				return OrgUnitNodeDetails{OrgID: 10000001, OrgCode: "A001", Name: "Root"}, nil
			},
		}
		store.detailsFn = func(_ context.Context, _ string, _ int, asOfDate string) (OrgUnitNodeDetails, error) {
			gotEffectiveDate = asOfDate
			return OrgUnitNodeDetails{OrgID: 10000001, OrgCode: "A001", Name: "Root"}, nil
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/details?org_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetails(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if want := currentUTCDateString(); gotEffectiveDate != want {
			t.Fatalf("effective_date=%q want=%q", gotEffectiveDate, want)
		}
	})
}

func TestRenderOrgNodeDetails(t *testing.T) {
	out := renderOrgNodeDetails(OrgUnitNodeDetails{
		OrgID:          10000001,
		OrgCode:        "A001",
		Name:           "Root",
		IsBusinessUnit: true,
		ManagerPernr:   "1001",
		ManagerName:    "Boss",
	}, "2026-01-06", "2026-01-06", []OrgUnitNodeVersion{{EventID: 1, EffectiveDate: "2026-01-01", EventType: "RENAME"}}, true, "")
	if !strings.Contains(out, "A001") || !strings.Contains(out, "Root") {
		t.Fatalf("unexpected output: %q", out)
	}

	out2 := renderOrgNodeDetails(OrgUnitNodeDetails{
		OrgID:      10000002,
		OrgCode:    "B002",
		Name:       "Child",
		ParentID:   10000001,
		ParentCode: "A001",
		ParentName: "Root",
	}, "2026-01-06", "2026-01-06", []OrgUnitNodeVersion{{EventID: 2, EffectiveDate: "2026-01-01", EventType: "RENAME"}}, true, "")
	if !strings.Contains(out2, "A001 · Root") || !strings.Contains(out2, "上级组织") {
		t.Fatalf("unexpected output: %q", out2)
	}

	out3 := renderOrgNodeDetails(OrgUnitNodeDetails{
		OrgID:   10000003,
		OrgCode: "C003",
		Name:    "Middle",
	}, "2026-01-15", "2026-01-06", []OrgUnitNodeVersion{
		{EventID: 1, EffectiveDate: "2026-01-01", EventType: "RENAME"},
		{EventID: 2, EffectiveDate: "2026-01-10", EventType: "MOVE"},
		{EventID: 3, EffectiveDate: "2026-01-20", EventType: "RENAME"},
	}, true, "success")
	if !strings.Contains(out3, "更新成功") || !strings.Contains(out3, "2026-01-01") || !strings.Contains(out3, "2026-01-20") {
		t.Fatalf("unexpected output: %q", out3)
	}

	out4 := renderOrgNodeDetails(OrgUnitNodeDetails{
		OrgID:   10000004,
		OrgCode: "D004",
		Name:    "Leaf",
	}, "2026-01-05", "2026-01-06", []OrgUnitNodeVersion{
		{EventID: 1, EffectiveDate: "", EventType: "RENAME"},
		{EventID: 2, EffectiveDate: "2026-01-05", EventType: "RENAME"},
	}, true, "")
	if !strings.Contains(out4, `data-min-effective-date="2026-01-05"`) {
		t.Fatalf("unexpected output: %q", out4)
	}
}

func TestRenderOrgNodeSearchPanel(t *testing.T) {
	out := renderOrgNodeSearchPanel(nil)
	if !strings.Contains(out, `data-count="0"`) || !strings.Contains(out, "未找到匹配组织") {
		t.Fatalf("unexpected output: %q", out)
	}

	out2 := renderOrgNodeSearchPanel([]OrgUnitSearchCandidate{{OrgID: 10000001, OrgCode: "A001", Name: "Root"}})
	if !strings.Contains(out2, `data-org-id="10000001"`) || !strings.Contains(out2, "org-node-search-item") {
		t.Fatalf("unexpected output: %q", out2)
	}
}

func TestSelectOrgNodeVersion(t *testing.T) {
	t.Run("empty versions", func(t *testing.T) {
		if _, idx := selectOrgNodeVersion("2026-01-01", nil); idx != -1 {
			t.Fatalf("unexpected idx=%d", idx)
		}
	})
	t.Run("bad effective_date uses last", func(t *testing.T) {
		versions := []OrgUnitNodeVersion{
			{EventID: 1, EffectiveDate: "2026-01-01", EventType: "RENAME"},
			{EventID: 2, EffectiveDate: "2026-01-10", EventType: "MOVE"},
		}
		got, idx := selectOrgNodeVersion("bad", versions)
		if idx != 1 || got.EventID != 2 {
			t.Fatalf("unexpected result: %#v idx=%d", got, idx)
		}
	})
	t.Run("skips bad effective date", func(t *testing.T) {
		versions := []OrgUnitNodeVersion{
			{EventID: 1, EffectiveDate: "2026-01-01", EventType: "RENAME"},
			{EventID: 2, EffectiveDate: "bad", EventType: "MOVE"},
		}
		got, idx := selectOrgNodeVersion("2026-01-05", versions)
		if idx != 0 || got.EventID != 1 {
			t.Fatalf("unexpected result: %#v idx=%d", got, idx)
		}
	})
	t.Run("breaks on future version", func(t *testing.T) {
		versions := []OrgUnitNodeVersion{
			{EventID: 1, EffectiveDate: "2026-01-01", EventType: "RENAME"},
			{EventID: 2, EffectiveDate: "2026-01-10", EventType: "MOVE"},
			{EventID: 3, EffectiveDate: "2026-02-01", EventType: "RENAME"},
		}
		got, idx := selectOrgNodeVersion("2026-01-15", versions)
		if idx != 1 || got.EventID != 2 {
			t.Fatalf("unexpected result: %#v idx=%d", got, idx)
		}
	})
}

func TestHandleOrgNodeDetailsPage(t *testing.T) {
	t.Run("missing tenant", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/view?effective_date=2026-01-06&org_id=10000001", nil)
		rec := httptest.NewRecorder()
		handleOrgNodeDetailsPage(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/nodes/view?effective_date=2026-01-06&org_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetailsPage(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("deprecated as_of", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/view?as_of=2026-01-06&org_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetailsPage(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("invalid effective_date", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/view?effective_date=bad&org_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetailsPage(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("invalid tree_as_of", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/view?effective_date=2026-01-06&org_id=10000001&tree_as_of=bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetailsPage(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("missing org_id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/view?effective_date=2026-01-06", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetailsPage(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("invalid org_id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/view?effective_date=2026-01-06&org_id=bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetailsPage(rec, req, newOrgUnitMemoryStore())
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
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/view?effective_date=2026-01-06&org_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetailsPage(rec, req, store)
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
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/view?effective_date=2026-01-06&org_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetailsPage(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("versions not found", func(t *testing.T) {
		store := &orgUnitReadStoreStub{
			orgUnitMemoryStore: newOrgUnitMemoryStore(),
			detailsFn: func(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
				return OrgUnitNodeDetails{OrgID: 10000001, OrgCode: "A001", Name: "Root"}, nil
			},
			listVersionsFn: func(context.Context, string, int) ([]OrgUnitNodeVersion, error) {
				return nil, errOrgUnitNotFound
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/view?effective_date=2026-01-06&org_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetailsPage(rec, req, store)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("versions error", func(t *testing.T) {
		store := &orgUnitReadStoreStub{
			orgUnitMemoryStore: newOrgUnitMemoryStore(),
			detailsFn: func(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
				return OrgUnitNodeDetails{OrgID: 10000001, OrgCode: "A001", Name: "Root"}, nil
			},
			listVersionsFn: func(context.Context, string, int) ([]OrgUnitNodeVersion, error) {
				return nil, errors.New("boom")
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/view?effective_date=2026-01-06&org_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetailsPage(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("success", func(t *testing.T) {
		store := &orgUnitReadStoreStub{
			orgUnitMemoryStore: newOrgUnitMemoryStore(),
			detailsFn: func(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
				return OrgUnitNodeDetails{OrgID: 10000001, OrgCode: "A001", Name: "Root"}, nil
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/view?effective_date=2026-01-06&org_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetailsPage(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "OrgUnit / Details") || !strings.Contains(body, "A001") {
			t.Fatalf("unexpected body: %q", body)
		}
	})
	t.Run("default effective_date uses today", func(t *testing.T) {
		var gotEffectiveDate string
		store := &orgUnitReadStoreStub{
			orgUnitMemoryStore: newOrgUnitMemoryStore(),
			detailsFn: func(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
				return OrgUnitNodeDetails{OrgID: 10000001, OrgCode: "A001", Name: "Root"}, nil
			},
		}
		store.detailsFn = func(_ context.Context, _ string, _ int, asOfDate string) (OrgUnitNodeDetails, error) {
			gotEffectiveDate = asOfDate
			return OrgUnitNodeDetails{OrgID: 10000001, OrgCode: "A001", Name: "Root"}, nil
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/view?org_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetailsPage(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if want := currentUTCDateString(); gotEffectiveDate != want {
			t.Fatalf("effective_date=%q want=%q", gotEffectiveDate, want)
		}
	})
}

func TestHandleOrgNodeSearch(t *testing.T) {
	t.Run("missing tenant", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/search?tree_as_of=2026-01-06&query=A001", nil)
		rec := httptest.NewRecorder()
		handleOrgNodeSearch(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/nodes/search?tree_as_of=2026-01-06&query=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeSearch(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("invalid tree_as_of", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/search?tree_as_of=bad&query=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeSearch(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("missing query", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/search?tree_as_of=2026-01-06", nil)
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
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/search?tree_as_of=2026-01-06&query=A001", nil)
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
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/search?tree_as_of=2026-01-06&query=A001", nil)
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
				return OrgUnitSearchResult{TargetOrgID: 10000001, TargetOrgCode: "A001", TargetName: "Root", PathOrgIDs: []int{10000001}, TreeAsOf: "2026-01-06"}, nil
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/search?tree_as_of=2026-01-06&query=A001", nil)
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
	t.Run("panel not found", func(t *testing.T) {
		store := &orgUnitReadStoreStub{
			orgUnitMemoryStore: newOrgUnitMemoryStore(),
			searchCandidatesFn: func(context.Context, string, string, string, int) ([]OrgUnitSearchCandidate, error) {
				return nil, errOrgUnitNotFound
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/search?tree_as_of=2026-01-06&query=A001&format=panel", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeSearch(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "未找到匹配组织") {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})
	t.Run("panel store error", func(t *testing.T) {
		store := &orgUnitReadStoreStub{
			orgUnitMemoryStore: newOrgUnitMemoryStore(),
			searchCandidatesFn: func(context.Context, string, string, string, int) ([]OrgUnitSearchCandidate, error) {
				return nil, errors.New("boom")
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/search?tree_as_of=2026-01-06&query=A001&format=panel", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeSearch(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("panel success", func(t *testing.T) {
		store := &orgUnitReadStoreStub{
			orgUnitMemoryStore: newOrgUnitMemoryStore(),
			searchCandidatesFn: func(context.Context, string, string, string, int) ([]OrgUnitSearchCandidate, error) {
				return []OrgUnitSearchCandidate{{OrgID: 10000001, OrgCode: "A001", Name: "Root"}}, nil
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/search?tree_as_of=2026-01-06&query=A001&format=panel", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeSearch(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "org-node-search-item") {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})
}
