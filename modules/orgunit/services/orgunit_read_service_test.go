package services

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type orgUnitReadFakeStore struct {
	nodes           map[string]OrgUnitReadNode
	children        map[string][]string
	listTree        bool
	listChildrenN   int
	lastSearchLimit int
}

func newOrgUnitReadFakeStore(t *testing.T) *orgUnitReadFakeStore {
	t.Helper()
	root := mustOrgUnitReadKey(t, 10000001)
	blossom := mustOrgUnitReadKey(t, 10000002)
	east := mustOrgUnitReadKey(t, 10000003)
	sh := mustOrgUnitReadKey(t, 10000004)
	flowers := mustOrgUnitReadKey(t, 10000005)

	return &orgUnitReadFakeStore{
		nodes: map[string]OrgUnitReadNode{
			root: {
				OrgCode:         "ROOT",
				OrgNodeKey:      root,
				Name:            "Root",
				Status:          "active",
				PathOrgCodes:    []string{"ROOT"},
				PathOrgNodeKeys: []string{root},
			},
			blossom: {
				OrgCode:         "BLOSSOM",
				OrgNodeKey:      blossom,
				Name:            "Blossom",
				Status:          "active",
				PathOrgCodes:    []string{"ROOT", "BLOSSOM"},
				PathOrgNodeKeys: []string{root, blossom},
			},
			east: {
				OrgCode:         "EAST",
				OrgNodeKey:      east,
				Name:            "East",
				Status:          "active",
				PathOrgCodes:    []string{"ROOT", "BLOSSOM", "EAST"},
				PathOrgNodeKeys: []string{root, blossom, east},
			},
			sh: {
				OrgCode:         "SH",
				OrgNodeKey:      sh,
				Name:            "Shanghai",
				Status:          "active",
				PathOrgCodes:    []string{"ROOT", "BLOSSOM", "EAST", "SH"},
				PathOrgNodeKeys: []string{root, blossom, east, sh},
			},
			flowers: {
				OrgCode:         "FLOWERS",
				OrgNodeKey:      flowers,
				Name:            "Flowers",
				Status:          "active",
				PathOrgCodes:    []string{"FLOWERS"},
				PathOrgNodeKeys: []string{flowers},
			},
		},
		children: map[string][]string{
			root:    {blossom},
			blossom: {east},
			east:    {sh},
		},
	}
}

func (s *orgUnitReadFakeStore) ListRoots(context.Context, string, string, bool) ([]OrgUnitReadNode, error) {
	var out []OrgUnitReadNode
	for key, node := range s.nodes {
		if len(node.PathOrgNodeKeys) == 1 && node.PathOrgNodeKeys[0] == key {
			out = append(out, node)
		}
	}
	return out, nil
}

func (s *orgUnitReadFakeStore) ListChildren(_ context.Context, _ string, parentOrgNodeKey string, _ string, _ bool) ([]OrgUnitReadNode, error) {
	s.listChildrenN++
	var out []OrgUnitReadNode
	for _, key := range s.children[strings.TrimSpace(parentOrgNodeKey)] {
		out = append(out, s.nodes[key])
	}
	return out, nil
}

func (s *orgUnitReadFakeStore) ListTree(context.Context, string, string, bool) ([]OrgUnitReadNode, error) {
	s.listTree = true
	out := make([]OrgUnitReadNode, 0, len(s.nodes))
	for _, node := range s.nodes {
		out = append(out, node)
	}
	return out, nil
}

func (s *orgUnitReadFakeStore) ResolveByOrgNodeKey(_ context.Context, _ string, orgNodeKey string, _ string, _ bool) (OrgUnitReadNode, error) {
	node, ok := s.nodes[strings.TrimSpace(orgNodeKey)]
	if !ok {
		return OrgUnitReadNode{}, ErrOrgUnitReadNotFound
	}
	return node, nil
}

func (s *orgUnitReadFakeStore) ResolveByOrgCode(_ context.Context, _ string, orgCode string, _ string, _ bool) (OrgUnitReadNode, error) {
	for _, node := range s.nodes {
		if node.OrgCode == strings.TrimSpace(orgCode) {
			return node, nil
		}
	}
	return OrgUnitReadNode{}, ErrOrgUnitReadNotFound
}

func (s *orgUnitReadFakeStore) Search(_ context.Context, _ string, query string, _ string, _ bool, limit int) ([]OrgUnitReadNode, error) {
	s.lastSearchLimit = limit
	query = strings.ToLower(strings.TrimSpace(query))
	var out []OrgUnitReadNode
	for _, node := range s.nodes {
		if strings.Contains(strings.ToLower(node.OrgCode), query) || strings.Contains(strings.ToLower(node.Name), query) {
			out = append(out, node)
		}
	}
	return out, nil
}

func TestOrgUnitReadServiceVisibleRoots(t *testing.T) {
	store := newOrgUnitReadFakeStore(t)
	svc := NewOrgUnitReadService(store)
	blossom := mustOrgUnitReadKey(t, 10000002)

	got, err := svc.VisibleRoots(context.Background(), OrgUnitReadRequest{
		TenantID: "t1",
		AsOf:     "2026-01-01",
		ScopeFilter: OrgUnitReadScopeFilter{
			PrincipalID: "principal-a",
			Scopes: []OrgUnitScope{{
				OrgNodeKey:         blossom,
				IncludeDescendants: true,
			}},
		},
	})
	if err != nil {
		t.Fatalf("VisibleRoots err=%v", err)
	}
	if len(got) != 1 || got[0].OrgCode != "BLOSSOM" {
		t.Fatalf("roots=%+v", got)
	}
	if !got[0].HasVisibleChildren {
		t.Fatalf("visible children not detected: %+v", got[0])
	}
}

func TestOrgUnitReadServiceVisibleRootsDeduplicatesDescendantScopes(t *testing.T) {
	store := newOrgUnitReadFakeStore(t)
	svc := NewOrgUnitReadService(store)
	blossom := mustOrgUnitReadKey(t, 10000002)
	east := mustOrgUnitReadKey(t, 10000003)

	got, err := svc.VisibleRoots(context.Background(), OrgUnitReadRequest{
		TenantID: "t1",
		AsOf:     "2026-01-01",
		ScopeFilter: OrgUnitReadScopeFilter{
			PrincipalID: "principal-a",
			Scopes: []OrgUnitScope{
				{OrgNodeKey: blossom, IncludeDescendants: true},
				{OrgNodeKey: east, IncludeDescendants: false},
			},
		},
	})
	if err != nil {
		t.Fatalf("VisibleRoots err=%v", err)
	}
	if len(got) != 1 || got[0].OrgCode != "BLOSSOM" {
		t.Fatalf("roots=%+v", got)
	}
}

func TestOrgUnitReadServiceVisibleRootsSkipsStaleScopeAndFailsClosed(t *testing.T) {
	store := newOrgUnitReadFakeStore(t)
	svc := NewOrgUnitReadService(store)
	blossom := mustOrgUnitReadKey(t, 10000002)
	missing := mustOrgUnitReadKey(t, 10000999)

	got, err := svc.VisibleRoots(context.Background(), OrgUnitReadRequest{
		TenantID: "t1",
		AsOf:     "2026-01-01",
		ScopeFilter: OrgUnitReadScopeFilter{
			PrincipalID: "principal-a",
			Scopes: []OrgUnitScope{
				{OrgNodeKey: missing, IncludeDescendants: true},
				{OrgNodeKey: blossom, IncludeDescendants: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("VisibleRoots err=%v", err)
	}
	if len(got) != 1 || got[0].OrgCode != "BLOSSOM" {
		t.Fatalf("roots=%+v", got)
	}

	got, err = svc.VisibleRoots(context.Background(), OrgUnitReadRequest{
		TenantID: "t1",
		AsOf:     "2026-01-01",
		ScopeFilter: OrgUnitReadScopeFilter{
			PrincipalID: "principal-a",
			Scopes: []OrgUnitScope{{
				OrgNodeKey:         missing,
				IncludeDescendants: true,
			}},
		},
	})
	if err == nil || !errors.Is(err, ErrOrgUnitReadScopeForbidden) {
		t.Fatalf("expected scope forbidden, got roots=%+v err=%v", got, err)
	}
}

func TestOrgUnitReadServiceVisibleRootsReturnsTopmostVisibleScopes(t *testing.T) {
	store := newOrgUnitReadFakeStore(t)
	svc := NewOrgUnitReadService(store)
	blossom := mustOrgUnitReadKey(t, 10000002)
	east := mustOrgUnitReadKey(t, 10000003)

	got, err := svc.VisibleRoots(context.Background(), OrgUnitReadRequest{
		TenantID: "t1",
		AsOf:     "2026-01-01",
		ScopeFilter: OrgUnitReadScopeFilter{
			PrincipalID: "principal-a",
			Scopes: []OrgUnitScope{
				{OrgNodeKey: blossom, IncludeDescendants: false},
				{OrgNodeKey: east, IncludeDescendants: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("VisibleRoots err=%v", err)
	}
	if gotCodes := orgUnitReadNodeCodes(got); !reflect.DeepEqual(gotCodes, []string{"BLOSSOM", "EAST"}) {
		t.Fatalf("root codes=%v want [BLOSSOM EAST]", gotCodes)
	}
}

func TestOrgUnitReadServiceVisibleRootsKeepsDescendantWhenAncestorDoesNotCoverDescendants(t *testing.T) {
	store := newOrgUnitReadFakeStore(t)
	svc := NewOrgUnitReadService(store)
	root := mustOrgUnitReadKey(t, 10000001)
	sh := mustOrgUnitReadKey(t, 10000004)

	got, err := svc.VisibleRoots(context.Background(), OrgUnitReadRequest{
		TenantID: "t1",
		AsOf:     "2026-01-01",
		ScopeFilter: OrgUnitReadScopeFilter{
			PrincipalID: "principal-a",
			Scopes: []OrgUnitScope{
				{OrgNodeKey: root, IncludeDescendants: false},
				{OrgNodeKey: sh, IncludeDescendants: false},
			},
		},
	})
	if err != nil {
		t.Fatalf("VisibleRoots err=%v", err)
	}
	if gotCodes := orgUnitReadNodeCodes(got); !reflect.DeepEqual(gotCodes, []string{"ROOT", "SH"}) {
		t.Fatalf("root codes=%v want [ROOT SH]", gotCodes)
	}
}

func TestOrgUnitReadServiceVisibleRootsAncestorDescendantScopeCoversChildScope(t *testing.T) {
	store := newOrgUnitReadFakeStore(t)
	svc := NewOrgUnitReadService(store)
	root := mustOrgUnitReadKey(t, 10000001)
	sh := mustOrgUnitReadKey(t, 10000004)

	got, err := svc.VisibleRoots(context.Background(), OrgUnitReadRequest{
		TenantID: "t1",
		AsOf:     "2026-01-01",
		ScopeFilter: OrgUnitReadScopeFilter{
			PrincipalID: "principal-a",
			Scopes: []OrgUnitScope{
				{OrgNodeKey: root, IncludeDescendants: true},
				{OrgNodeKey: sh, IncludeDescendants: false},
			},
		},
	})
	if err != nil {
		t.Fatalf("VisibleRoots err=%v", err)
	}
	if len(got) != 1 || got[0].OrgCode != "ROOT" {
		t.Fatalf("roots=%+v", got)
	}
}

func TestOrgUnitReadServiceChildrenAreScopeAware(t *testing.T) {
	store := newOrgUnitReadFakeStore(t)
	svc := NewOrgUnitReadService(store)
	blossom := mustOrgUnitReadKey(t, 10000002)
	east := mustOrgUnitReadKey(t, 10000003)

	got, err := svc.Children(context.Background(), OrgUnitChildrenRequest{
		TenantID:         "t1",
		AsOf:             "2026-01-01",
		ParentOrgNodeKey: blossom,
		ScopeFilter: OrgUnitReadScopeFilter{
			PrincipalID: "principal-a",
			Scopes: []OrgUnitScope{{
				OrgNodeKey:         east,
				IncludeDescendants: false,
			}},
		},
	})
	if err == nil || !errors.Is(err, ErrOrgUnitReadScopeForbidden) {
		t.Fatalf("expected parent forbidden, got nodes=%+v err=%v", got, err)
	}

	got, err = svc.Children(context.Background(), OrgUnitChildrenRequest{
		TenantID:         "t1",
		AsOf:             "2026-01-01",
		ParentOrgNodeKey: blossom,
		ScopeFilter: OrgUnitReadScopeFilter{
			PrincipalID: "principal-a",
			Scopes: []OrgUnitScope{{
				OrgNodeKey:         blossom,
				IncludeDescendants: true,
			}},
		},
	})
	if err != nil {
		t.Fatalf("Children err=%v", err)
	}
	if len(got) != 1 || got[0].OrgCode != "EAST" {
		t.Fatalf("children=%+v", got)
	}
}

func TestOrgUnitReadServiceListScopesBeforePagination(t *testing.T) {
	store := newOrgUnitReadFakeStore(t)
	svc := NewOrgUnitReadService(store)
	blossom := mustOrgUnitReadKey(t, 10000002)

	got, total, err := svc.List(context.Background(), OrgUnitListRequest{
		TenantID:    "t1",
		AsOf:        "2026-01-01",
		AllOrgUnits: true,
		ScopeFilter: OrgUnitReadScopeFilter{
			PrincipalID: "principal-a",
			Scopes: []OrgUnitScope{{
				OrgNodeKey:         blossom,
				IncludeDescendants: true,
			}},
		},
		SortField: "code",
		SortOrder: "asc",
		Limit:     2,
		Offset:    1,
	})
	if err != nil {
		t.Fatalf("List err=%v", err)
	}
	if total != 3 {
		t.Fatalf("total=%d want 3", total)
	}
	if gotCodes := orgUnitReadNodeCodes(got); !reflect.DeepEqual(gotCodes, []string{"EAST", "SH"}) {
		t.Fatalf("page codes=%v want [EAST SH]", gotCodes)
	}
	if !store.listTree {
		t.Fatal("expected List to use bulk tree store")
	}
	if store.listChildrenN != 0 {
		t.Fatalf("ListChildren calls=%d, want 0", store.listChildrenN)
	}
}

func TestOrgUnitReadServiceListFiltersChildrenWithinScope(t *testing.T) {
	store := newOrgUnitReadFakeStore(t)
	svc := NewOrgUnitReadService(store)
	blossom := mustOrgUnitReadKey(t, 10000002)

	got, total, err := svc.List(context.Background(), OrgUnitListRequest{
		TenantID:         "t1",
		AsOf:             "2026-01-01",
		ParentOrgNodeKey: blossom,
		ScopeFilter: OrgUnitReadScopeFilter{
			PrincipalID: "principal-a",
			Scopes: []OrgUnitScope{{
				OrgNodeKey:         blossom,
				IncludeDescendants: true,
			}},
		},
		Status: "active",
	})
	if err != nil {
		t.Fatalf("List err=%v", err)
	}
	if total != 1 || len(got) != 1 || got[0].OrgCode != "EAST" {
		t.Fatalf("children=%+v total=%d", got, total)
	}
}

func TestOrgUnitReadServiceListVisibleChildrenUsesExactPathPrefix(t *testing.T) {
	root := mustOrgUnitReadKey(t, 10000001)
	blossom := mustOrgUnitReadKey(t, 10000002)
	east := mustOrgUnitReadKey(t, 10000003)
	sh := mustOrgUnitReadKey(t, 10000004)

	nodes := []OrgUnitReadNode{
		{
			OrgCode:         "BLOSSOM",
			OrgNodeKey:      blossom,
			Name:            "Blossom",
			Status:          "active",
			PathOrgNodeKeys: []string{root, blossom},
		},
		{
			OrgCode:         "SH",
			OrgNodeKey:      sh,
			Name:            "Shanghai",
			Status:          "active",
			PathOrgNodeKeys: []string{root, east, sh},
		},
	}

	got := orgUnitReadService{}.decorateVisibleChildrenFromCandidates(nodes, nodes)
	if len(got) != 2 {
		t.Fatalf("nodes=%+v", got)
	}
	if got[0].HasVisibleChildren {
		t.Fatalf("cross-branch candidate marked as visible child: %+v", got[0])
	}
}

func TestOrgUnitReadServiceSearchReturnsSafePathFromVisibleRoot(t *testing.T) {
	store := newOrgUnitReadFakeStore(t)
	svc := NewOrgUnitReadService(store)
	blossom := mustOrgUnitReadKey(t, 10000002)

	got, err := svc.Search(context.Background(), OrgUnitSearchRequest{
		TenantID: "t1",
		AsOf:     "2026-01-01",
		Query:    "Shanghai",
		ScopeFilter: OrgUnitReadScopeFilter{
			PrincipalID: "principal-a",
			Scopes: []OrgUnitScope{{
				OrgNodeKey:         blossom,
				IncludeDescendants: true,
			}},
		},
	})
	if err != nil {
		t.Fatalf("Search err=%v", err)
	}
	if len(got) != 1 || got[0].OrgCode != "SH" {
		t.Fatalf("search=%+v", got)
	}
	wantPath := []string{"BLOSSOM", "EAST", "SH"}
	if !reflect.DeepEqual(got[0].PathOrgCodes, wantPath) {
		t.Fatalf("path=%v want=%v", got[0].PathOrgCodes, wantPath)
	}
}

func TestOrgUnitReadServiceSearchReturnsDeepSafePathFromVisibleRoot(t *testing.T) {
	store := newOrgUnitReadFakeStore(t)
	svc := NewOrgUnitReadService(store)
	blossom := mustOrgUnitReadKey(t, 10000002)
	team := mustOrgUnitReadKey(t, 10000006)
	leaf := mustOrgUnitReadKey(t, 10000007)
	store.nodes[team] = OrgUnitReadNode{
		OrgCode:         "TEAM",
		OrgNodeKey:      team,
		Name:            "Team",
		Status:          "active",
		PathOrgCodes:    []string{"ROOT", "BLOSSOM", "EAST", "TEAM"},
		PathOrgNodeKeys: []string{mustOrgUnitReadKey(t, 10000001), blossom, mustOrgUnitReadKey(t, 10000003), team},
	}
	store.nodes[leaf] = OrgUnitReadNode{
		OrgCode:         "LEAF",
		OrgNodeKey:      leaf,
		Name:            "Leaf",
		Status:          "active",
		PathOrgCodes:    []string{"ROOT", "BLOSSOM", "EAST", "TEAM", "LEAF"},
		PathOrgNodeKeys: []string{mustOrgUnitReadKey(t, 10000001), blossom, mustOrgUnitReadKey(t, 10000003), team, leaf},
	}
	store.children[mustOrgUnitReadKey(t, 10000003)] = []string{team}
	store.children[team] = []string{leaf}

	got, err := svc.Search(context.Background(), OrgUnitSearchRequest{
		TenantID: "t1",
		AsOf:     "2026-01-01",
		Query:    "Leaf",
		ScopeFilter: OrgUnitReadScopeFilter{
			PrincipalID: "principal-a",
			Scopes: []OrgUnitScope{{
				OrgNodeKey:         blossom,
				IncludeDescendants: true,
			}},
		},
	})
	if err != nil {
		t.Fatalf("Search err=%v", err)
	}
	if len(got) != 1 || got[0].OrgCode != "LEAF" {
		t.Fatalf("search=%+v", got)
	}
	wantPath := []string{"BLOSSOM", "EAST", "TEAM", "LEAF"}
	if !reflect.DeepEqual(got[0].PathOrgCodes, wantPath) {
		t.Fatalf("path=%v want=%v", got[0].PathOrgCodes, wantPath)
	}
}

func TestOrgUnitReadServiceSearchOmitsInvisibleOtherBranch(t *testing.T) {
	store := newOrgUnitReadFakeStore(t)
	svc := NewOrgUnitReadService(store)
	flowers := mustOrgUnitReadKey(t, 10000005)

	got, err := svc.Search(context.Background(), OrgUnitSearchRequest{
		TenantID: "t1",
		AsOf:     "2026-01-01",
		Query:    "Shanghai",
		ScopeFilter: OrgUnitReadScopeFilter{
			PrincipalID: "principal-a",
			Scopes: []OrgUnitScope{{
				OrgNodeKey:         flowers,
				IncludeDescendants: true,
			}},
		},
	})
	if err != nil {
		t.Fatalf("Search err=%v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no invisible branch results, got %+v", got)
	}
}

func TestOrgUnitReadServiceSearchScansPastPhysicalLimitBeforeScopeFilter(t *testing.T) {
	store := newOrgUnitReadFakeStore(t)
	svc := NewOrgUnitReadService(store)
	sh := mustOrgUnitReadKey(t, 10000004)

	got, err := svc.Search(context.Background(), OrgUnitSearchRequest{
		TenantID: "t1",
		AsOf:     "2026-01-01",
		Query:    "s",
		Limit:    1,
		ScopeFilter: OrgUnitReadScopeFilter{
			PrincipalID: "principal-a",
			Scopes: []OrgUnitScope{{
				OrgNodeKey:         sh,
				IncludeDescendants: true,
			}},
		},
	})
	if err != nil {
		t.Fatalf("Search err=%v", err)
	}
	if store.lastSearchLimit != -1 {
		t.Fatalf("search limit=%d want -1", store.lastSearchLimit)
	}
	if len(got) != 1 || got[0].OrgCode != "SH" {
		t.Fatalf("search=%+v", got)
	}
}

func TestOrgUnitReadServiceResolveFailClosedOutsideScope(t *testing.T) {
	store := newOrgUnitReadFakeStore(t)
	svc := NewOrgUnitReadService(store)
	flowers := mustOrgUnitReadKey(t, 10000005)

	got, err := svc.Resolve(context.Background(), OrgUnitResolveRequest{
		TenantID: "t1",
		AsOf:     "2026-01-01",
		OrgCodes: []string{"SH"},
		ScopeFilter: OrgUnitReadScopeFilter{
			PrincipalID: "principal-a",
			Scopes: []OrgUnitScope{{
				OrgNodeKey:         flowers,
				IncludeDescendants: true,
			}},
		},
	})
	if err != nil {
		t.Fatalf("Resolve err=%v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected fail-closed empty result, got %+v", got)
	}
}

func mustOrgUnitReadKey(t *testing.T, seq int64) string {
	t.Helper()
	key, err := orgunitpkg.EncodeOrgNodeKey(seq)
	if err != nil {
		t.Fatalf("EncodeOrgNodeKey(%d) err=%v", seq, err)
	}
	return key
}

func orgUnitReadNodeCodes(nodes []OrgUnitReadNode) []string {
	out := make([]string, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, node.OrgCode)
	}
	return out
}
