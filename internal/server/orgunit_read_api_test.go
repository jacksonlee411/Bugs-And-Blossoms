package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type orgUnitScopeRuntimeStub struct {
	scopes     []principalOrgScope
	err        error
	scopeCalls int
}

func (s orgUnitScopeRuntimeStub) AuthorizePrincipal(context.Context, string, string, string, string) (bool, error) {
	return false, nil
}

func (s orgUnitScopeRuntimeStub) CapabilitiesForPrincipal(context.Context, string, string) ([]string, error) {
	return nil, nil
}

func (s *orgUnitScopeRuntimeStub) OrgScopesForPrincipal(context.Context, string, string, string) ([]principalOrgScope, error) {
	s.scopeCalls++
	if s.err != nil {
		return nil, s.err
	}
	return append([]principalOrgScope(nil), s.scopes...), nil
}

func (s orgUnitScopeRuntimeStub) ListRoleDefinitions(context.Context, string) ([]authzRoleDefinition, error) {
	return nil, nil
}

func (s orgUnitScopeRuntimeStub) GetRoleDefinition(context.Context, string, string) (authzRoleDefinition, bool, error) {
	return authzRoleDefinition{}, false, nil
}

func (s orgUnitScopeRuntimeStub) CreateRoleDefinition(context.Context, string, saveAuthzRoleDefinitionInput) (authzRoleDefinition, error) {
	return authzRoleDefinition{}, nil
}

func (s orgUnitScopeRuntimeStub) UpdateRoleDefinition(context.Context, string, string, saveAuthzRoleDefinitionInput) (authzRoleDefinition, error) {
	return authzRoleDefinition{}, nil
}

func (s orgUnitScopeRuntimeStub) GetPrincipalAssignment(context.Context, string, string) (principalAuthzAssignment, bool, error) {
	return principalAuthzAssignment{}, false, nil
}

func (s orgUnitScopeRuntimeStub) ReplacePrincipalAssignment(context.Context, string, string, replacePrincipalAssignmentInput) (principalAuthzAssignment, error) {
	return principalAuthzAssignment{}, nil
}

func TestHandleOrgUnitsAPI_ListRoots(t *testing.T) {
	store := &resolveOrgCodeStore{
		listNodes: []OrgUnitNode{{ID: "10000001", OrgCode: "A001", Name: "Root", IsBusinessUnit: true}},
	}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		OrgUnits []map[string]any `json:"org_units"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode err=%v", err)
	}
	if len(payload.OrgUnits) != 1 {
		t.Fatalf("expected 1 org unit, got %d", len(payload.OrgUnits))
	}
	if got := payload.OrgUnits[0]["org_node_key"]; got != mustReadTestOrgNodeKey(t, 10000001) {
		t.Fatalf("org_node_key=%v", got)
	}
	if got := payload.OrgUnits[0]["has_visible_children"]; got != false {
		t.Fatalf("has_visible_children=%v", got)
	}
}

func TestHandleOrgUnitsAPI_ListVisibleRootsFromScopedMiddleNode(t *testing.T) {
	store := newOrgUnitMemoryStore()
	ctx := context.Background()
	root, err := store.CreateNodeCurrent(ctx, "t1", "2026-01-01", "ROOT", "Root", "", false)
	if err != nil {
		t.Fatalf("create root err=%v", err)
	}
	middle, err := store.CreateNodeCurrent(ctx, "t1", "2026-01-01", "MID", "Middle", root.ID, false)
	if err != nil {
		t.Fatalf("create middle err=%v", err)
	}
	if _, err := store.CreateNodeCurrent(ctx, "t1", "2026-01-01", "LEAF", "Leaf", middle.ID, false); err != nil {
		t.Fatalf("create leaf err=%v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01", nil)
	req = req.WithContext(withPrincipal(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}), Principal{ID: "principal-a"}))
	rec := httptest.NewRecorder()
	runtime := &orgUnitScopeRuntimeStub{scopes: []principalOrgScope{{
		OrgNodeKey:         middle.ID,
		IncludeDescendants: true,
	}}}
	handleOrgUnitsAPI(rec, req, store, nil, runtime)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		OrgUnits []orgUnitListItem `json:"org_units"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode err=%v", err)
	}
	if len(payload.OrgUnits) != 1 || payload.OrgUnits[0].OrgCode != "MID" {
		t.Fatalf("visible roots=%+v", payload.OrgUnits)
	}
	if payload.OrgUnits[0].OrgNodeKey != middle.ID {
		t.Fatalf("org_node_key=%q want=%q", payload.OrgUnits[0].OrgNodeKey, middle.ID)
	}
	if payload.OrgUnits[0].HasVisibleChildren == nil || !*payload.OrgUnits[0].HasVisibleChildren {
		t.Fatalf("has_visible_children missing: %+v", payload.OrgUnits[0])
	}
}

func TestHandleOrgUnitsAPI_AsOfRequired(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, newOrgUnitMemoryStore(), nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "as_of required") {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestHandleOrgUnitsAPI_InvalidAsOf(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=bad", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, newOrgUnitMemoryStore(), nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsAPI_ListParentInvalidOrgCode(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&parent_org_code=bad%7F", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, newOrgUnitMemoryStore(), nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsAPI_ListParentNotFound(t *testing.T) {
	store := &resolveOrgCodeStore{resolveErr: orgunitpkg.ErrOrgCodeNotFound}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&parent_org_code=A001", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsAPI_ListParentResolveInvalid(t *testing.T) {
	store := &resolveOrgCodeStore{resolveErr: orgunitpkg.ErrOrgCodeInvalid}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&parent_org_code=A001", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsAPI_ListParentResolveError(t *testing.T) {
	store := &resolveOrgCodeStore{resolveErr: errBoom{}}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&parent_org_code=A001", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsAPI_ListChildrenNotFound(t *testing.T) {
	store := &resolveOrgCodeStore{
		resolveID:       10000001,
		listChildrenErr: errOrgUnitNotFound,
	}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&parent_org_code=A001", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsAPI_ListChildrenError(t *testing.T) {
	store := &resolveOrgCodeStore{
		resolveID:       10000001,
		listChildrenErr: errBoom{},
	}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&parent_org_code=A001", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsAPI_ListChildrenSuccess(t *testing.T) {
	store := &resolveOrgCodeStore{
		resolveID:    10000001,
		listChildren: []OrgUnitChild{{OrgID: 10000002, OrgCode: "A002", Name: "Child", HasChildren: true}},
	}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&parent_org_code=A001", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "A002") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleOrgUnitsAPI_ListServerModeChildren(t *testing.T) {
	store := &resolveOrgCodeStore{
		resolveID: 10000001,
		listChildren: []OrgUnitChild{
			{OrgID: 10000002, OrgCode: "A002", Name: "Finance", Status: "active", IsBusinessUnit: false, HasChildren: true},
			{OrgID: 10000003, OrgCode: "A003", Name: "Admin", Status: "disabled", IsBusinessUnit: true, HasChildren: false},
			{OrgID: 10000004, OrgCode: "B001", Name: "Budget", Status: "active", IsBusinessUnit: false, HasChildren: false},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&parent_org_code=A001&include_disabled=1&mode=grid&status=inactive&sort=name&order=asc&page=0&size=10", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Page     int               `json:"page"`
		Size     int               `json:"size"`
		Total    int               `json:"total"`
		OrgUnits []orgUnitListItem `json:"org_units"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode err=%v", err)
	}
	if payload.Page != 0 || payload.Size != 10 || payload.Total != 1 {
		t.Fatalf("unexpected page payload: %+v", payload)
	}
	if len(payload.OrgUnits) != 1 || payload.OrgUnits[0].OrgCode != "A003" {
		t.Fatalf("unexpected rows: %+v", payload.OrgUnits)
	}
}

func TestHandleOrgUnitsAPI_ListServerModePagingAndSort(t *testing.T) {
	store := &resolveOrgCodeStore{
		resolveID: 10000001,
		listChildren: []OrgUnitChild{
			{OrgID: 10000002, OrgCode: "A002", Name: "Finance", Status: "active"},
			{OrgID: 10000003, OrgCode: "A003", Name: "Admin", Status: "active"},
			{OrgID: 10000004, OrgCode: "B001", Name: "Budget", Status: "active"},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&parent_org_code=A001&mode=grid&q=A&sort=code&order=desc&page=0&size=2", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Total    int               `json:"total"`
		OrgUnits []orgUnitListItem `json:"org_units"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode err=%v", err)
	}
	if payload.Total != 2 {
		t.Fatalf("total=%d", payload.Total)
	}
	if len(payload.OrgUnits) != 2 {
		t.Fatalf("len=%d", len(payload.OrgUnits))
	}
	if payload.OrgUnits[0].OrgCode != "A003" || payload.OrgUnits[1].OrgCode != "A002" {
		t.Fatalf("unexpected order: %+v", payload.OrgUnits)
	}
}

func TestHandleOrgUnitsAPI_ListServerModeInvalidQuery(t *testing.T) {
	tests := []string{
		"/org/api/org-units?as_of=2026-01-01&mode=grid&order=desc",
		"/org/api/org-units?as_of=2026-01-01&mode=grid&sort=foo",
		"/org/api/org-units?as_of=2026-01-01&mode=grid&page=-1",
		"/org/api/org-units?as_of=2026-01-01&mode=grid&size=0",
	}
	for _, rawURL := range tests {
		req := httptest.NewRequest(http.MethodGet, rawURL, nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsAPI(rec, req, newOrgUnitMemoryStore(), nil)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("url=%s status=%d body=%s", rawURL, rec.Code, rec.Body.String())
		}
	}
}

func TestHandleOrgUnitsAPI_ListServerModeNotFoundAndError(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		store := &resolveOrgCodeStore{
			resolveID:       10000001,
			listChildrenErr: errOrgUnitNotFound,
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&parent_org_code=A001&mode=grid&page=0&size=10", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsAPI(rec, req, store, nil)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("internal error", func(t *testing.T) {
		store := &resolveOrgCodeStore{
			listNodesErr: errBoom{},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&mode=grid&page=0&size=10", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsAPI(rec, req, store, nil)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestParseOrgUnitListQueryOptions(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		wantAny bool
		wantErr bool
		check   func(t *testing.T, opts orgUnitListQueryOptions)
	}{
		{
			name:    "empty",
			rawURL:  "/org/api/org-units",
			wantAny: false,
			check: func(t *testing.T, opts orgUnitListQueryOptions) {
				if opts.Page != orgUnitListDefaultPage || opts.PageSize != orgUnitListDefaultPageSize {
					t.Fatalf("unexpected defaults: %+v", opts)
				}
			},
		},
		{
			name:    "mode status and sort",
			rawURL:  "/org/api/org-units?mode=grid&status=inactive&sort=name&order=desc&page=2&size=25&q=Root",
			wantAny: true,
			check: func(t *testing.T, opts orgUnitListQueryOptions) {
				if opts.Status != orgUnitListStatusDisabled {
					t.Fatalf("status=%s", opts.Status)
				}
				if opts.SortField != orgUnitListSortName || opts.SortOrder != orgUnitListSortOrderDesc {
					t.Fatalf("sort=%+v", opts)
				}
				if !opts.Paginate || opts.Page != 2 || opts.PageSize != 25 {
					t.Fatalf("paginate=%+v", opts)
				}
				if opts.Keyword != "Root" {
					t.Fatalf("keyword=%q", opts.Keyword)
				}
			},
		},
		{
			name:    "status empty and disabled",
			rawURL:  "/org/api/org-units?status=&mode=grid",
			wantAny: true,
			check: func(t *testing.T, opts orgUnitListQueryOptions) {
				if opts.Status != "" {
					t.Fatalf("status=%q", opts.Status)
				}
			},
		},
		{
			name:    "status disabled alias",
			rawURL:  "/org/api/org-units?status=disabled",
			wantAny: true,
			check: func(t *testing.T, opts orgUnitListQueryOptions) {
				if opts.Status != orgUnitListStatusDisabled {
					t.Fatalf("status=%q", opts.Status)
				}
			},
		},
		{
			name:    "status active",
			rawURL:  "/org/api/org-units?status=active",
			wantAny: true,
			check: func(t *testing.T, opts orgUnitListQueryOptions) {
				if opts.Status != orgUnitListStatusActive {
					t.Fatalf("status=%q", opts.Status)
				}
			},
		},
		{
			name:    "sort default order",
			rawURL:  "/org/api/org-units?sort=code",
			wantAny: true,
			check: func(t *testing.T, opts orgUnitListQueryOptions) {
				if opts.SortOrder != orgUnitListSortOrderAsc {
					t.Fatalf("sort order=%s", opts.SortOrder)
				}
			},
		},
		{
			name:    "page only",
			rawURL:  "/org/api/org-units?page=1",
			wantAny: true,
			check: func(t *testing.T, opts orgUnitListQueryOptions) {
				if !opts.Paginate || opts.Page != 1 || opts.PageSize != orgUnitListDefaultPageSize {
					t.Fatalf("opts=%+v", opts)
				}
			},
		},
		{
			name:    "size only",
			rawURL:  "/org/api/org-units?size=50",
			wantAny: true,
			check: func(t *testing.T, opts orgUnitListQueryOptions) {
				if !opts.Paginate || opts.Page != orgUnitListDefaultPage || opts.PageSize != 50 {
					t.Fatalf("opts=%+v", opts)
				}
			},
		},
		{
			name:    "is business unit true",
			rawURL:  "/org/api/org-units?is_business_unit=true",
			wantAny: true,
			check: func(t *testing.T, opts orgUnitListQueryOptions) {
				if opts.IsBusinessUnit == nil || !*opts.IsBusinessUnit {
					t.Fatalf("opts=%+v", opts)
				}
			},
		},
		{
			name:    "is business unit false",
			rawURL:  "/org/api/org-units?is_business_unit=0",
			wantAny: true,
			check: func(t *testing.T, opts orgUnitListQueryOptions) {
				if opts.IsBusinessUnit == nil || *opts.IsBusinessUnit {
					t.Fatalf("opts=%+v", opts)
				}
			},
		},
		{
			name:    "all org units",
			rawURL:  "/org/api/org-units?all_org_units=true",
			wantAny: true,
			check: func(t *testing.T, opts orgUnitListQueryOptions) {
				if !opts.AllOrgUnits {
					t.Fatalf("opts=%+v", opts)
				}
			},
		},
		{name: "status invalid", rawURL: "/org/api/org-units?status=bad", wantErr: true},
		{name: "all org units invalid", rawURL: "/org/api/org-units?all_org_units=yes", wantErr: true},
		{name: "is business unit invalid", rawURL: "/org/api/org-units?is_business_unit=yes", wantErr: true},
		{name: "sort invalid", rawURL: "/org/api/org-units?sort=bad", wantErr: true},
		{name: "sort empty", rawURL: "/org/api/org-units?sort=", wantErr: true},
		{name: "order without sort", rawURL: "/org/api/org-units?order=asc", wantErr: true},
		{name: "order invalid", rawURL: "/org/api/org-units?sort=code&order=bad", wantErr: true},
		{name: "order empty", rawURL: "/org/api/org-units?sort=code&order=", wantErr: true},
		{name: "page empty", rawURL: "/org/api/org-units?page=", wantErr: true},
		{name: "page invalid", rawURL: "/org/api/org-units?page=-1", wantErr: true},
		{name: "size empty", rawURL: "/org/api/org-units?size=", wantErr: true},
		{name: "size too large", rawURL: "/org/api/org-units?size=9999", wantErr: true},
		{name: "ext sort missing key", rawURL: "/org/api/org-units?sort=ext:", wantErr: true},
		{name: "ext filter missing key", rawURL: "/org/api/org-units?ext_filter_value=DEPARTMENT", wantErr: true},
		{name: "ext filter missing value", rawURL: "/org/api/org-units?ext_filter_field_key=org_type", wantErr: true},
		{name: "ext filter empty field key", rawURL: "/org/api/org-units?ext_filter_field_key=&ext_filter_value=DEPARTMENT", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.rawURL, nil)
			opts, hasAny, err := parseOrgUnitListQueryOptions(req.URL.Query())
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err=%v", err)
			}
			if hasAny != tt.wantAny {
				t.Fatalf("hasAny=%v want=%v", hasAny, tt.wantAny)
			}
			if tt.check != nil {
				tt.check(t, opts)
			}
		})
	}
}

func TestFilterOrgUnitListItems(t *testing.T) {
	isBU := true
	notBU := false
	items := []orgUnitListItem{
		{OrgCode: "A001", Name: "Root Org", Status: "", IsBusinessUnit: &isBU},
		{OrgCode: "B002", Name: "Beta Team", Status: "active", IsBusinessUnit: &notBU},
		{OrgCode: "C003", Name: "Closed Team", Status: "disabled"},
	}

	all := filterOrgUnitListItems(items, "", "", nil)
	if len(all) != 3 {
		t.Fatalf("len=%d", len(all))
	}

	byStatus := filterOrgUnitListItems(items, "", "active", nil)
	if len(byStatus) != 2 {
		t.Fatalf("status len=%d", len(byStatus))
	}

	byKeyword := filterOrgUnitListItems(items, "closed", "", nil)
	if len(byKeyword) != 1 || byKeyword[0].OrgCode != "C003" {
		t.Fatalf("keyword rows=%+v", byKeyword)
	}

	byBusinessUnit := filterOrgUnitListItems(items, "", "", &isBU)
	if len(byBusinessUnit) != 1 || byBusinessUnit[0].OrgCode != "A001" {
		t.Fatalf("business unit rows=%+v", byBusinessUnit)
	}
}

func TestSortOrgUnitListItems(t *testing.T) {
	items := []orgUnitListItem{
		{OrgCode: "B002", Name: "Beta", Status: "disabled"},
		{OrgCode: "A001", Name: "Alpha", Status: "active"},
		{OrgCode: "C003", Name: "Alpha", Status: "active"},
	}

	sortOrgUnitListItems(items, orgUnitListSortName, orgUnitListSortOrderAsc)
	if items[0].OrgCode != "A001" || items[1].OrgCode != "C003" {
		t.Fatalf("name asc order=%+v", items)
	}

	sortOrgUnitListItems(items, orgUnitListSortStatus, orgUnitListSortOrderDesc)
	if items[0].Status != "disabled" {
		t.Fatalf("status desc order=%+v", items)
	}

	sortOrgUnitListItems(items, "", "")
	if len(items) != 3 {
		t.Fatalf("unexpected rows=%+v", items)
	}
}

func TestListOrgUnitListPage(t *testing.T) {
	t.Run("pager store", func(t *testing.T) {
		store := &orgUnitListPageReaderStore{
			resolveOrgCodeStore: &resolveOrgCodeStore{},
			items:               []orgUnitListItem{{OrgCode: "A001", Name: "Root", Status: "active"}},
			total:               1,
		}
		items, total, err := listOrgUnitListPage(context.Background(), store, "t1", orgUnitListPageRequest{
			AsOf: "2026-01-01",
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if total != 1 || len(items) != 1 || items[0].OrgCode != "A001" {
			t.Fatalf("items=%+v total=%d", items, total)
		}
	})

	t.Run("children and paging", func(t *testing.T) {
		parentOrgNodeKey, err := encodeOrgNodeKeyFromID(10000001)
		if err != nil {
			t.Fatalf("encode err=%v", err)
		}
		store := &resolveOrgCodeStore{
			listChildren: []OrgUnitChild{
				{OrgID: 10000002, OrgCode: "A002", Name: "Two", Status: "", IsBusinessUnit: true, HasChildren: true},
				{OrgID: 10000003, OrgCode: "A003", Name: "Three", Status: "disabled"},
			},
		}
		items, total, err := listOrgUnitListPage(context.Background(), store, "t1", orgUnitListPageRequest{
			AsOf:             "2026-01-01",
			ParentOrgNodeKey: &parentOrgNodeKey,
			SortField:        orgUnitListSortCode,
			SortOrder:        orgUnitListSortOrderAsc,
			Limit:            1,
			Offset:           -1,
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if total != 2 || len(items) != 1 {
			t.Fatalf("items=%+v total=%d", items, total)
		}
		if items[0].Status != "active" {
			t.Fatalf("status=%q", items[0].Status)
		}
		if items[0].IsBusinessUnit == nil || !*items[0].IsBusinessUnit {
			t.Fatalf("is_business_unit missing")
		}
		if items[0].HasChildren == nil || !*items[0].HasChildren {
			t.Fatalf("has_children missing")
		}

		empty, totalAfter, err := listOrgUnitListPage(context.Background(), store, "t1", orgUnitListPageRequest{
			AsOf:             "2026-01-01",
			ParentOrgNodeKey: &parentOrgNodeKey,
			Limit:            1,
			Offset:           5,
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if totalAfter != 2 || len(empty) != 0 {
			t.Fatalf("unexpected empty=%+v total=%d", empty, totalAfter)
		}

		clamped, totalClamped, err := listOrgUnitListPage(context.Background(), store, "t1", orgUnitListPageRequest{
			AsOf:             "2026-01-01",
			ParentOrgNodeKey: &parentOrgNodeKey,
			Limit:            5,
			Offset:           0,
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if totalClamped != 2 || len(clamped) != 2 {
			t.Fatalf("unexpected clamped=%+v total=%d", clamped, totalClamped)
		}
	})

	t.Run("roots and errors", func(t *testing.T) {
		store := &resolveOrgCodeStore{
			listNodes: []OrgUnitNode{
				{OrgCode: "Z009", Name: "Zero", Status: "", IsBusinessUnit: false, HasChildren: true},
				{OrgCode: "B002", Name: "Beta", Status: "disabled", IsBusinessUnit: true, HasChildren: false},
				{OrgCode: "A001", Name: "Alpha", Status: "active", IsBusinessUnit: false, HasChildren: false},
			},
		}
		items, total, err := listOrgUnitListPage(context.Background(), store, "t1", orgUnitListPageRequest{
			AsOf:    "2026-01-01",
			Keyword: "Alpha",
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if total != 1 || len(items) != 1 || items[0].OrgCode != "A001" {
			t.Fatalf("items=%+v total=%d", items, total)
		}
		if !(orgUnitListPageRequest{AsOf: "2026-01-01", Keyword: "Alpha"}).ShouldSearchAllOrgUnits() {
			t.Fatal("keyword list without parent should search all org units")
		}
		isBusinessUnit := true
		if !(orgUnitListPageRequest{AsOf: "2026-01-01", IsBusinessUnit: &isBusinessUnit}).ShouldSearchAllOrgUnits() {
			t.Fatal("business unit list without parent should search all org units")
		}
		if (orgUnitListPageRequest{AsOf: "2026-01-01"}).ShouldSearchAllOrgUnits() {
			t.Fatal("plain list without parent should keep root scope")
		}

		businessUnits, totalBusinessUnits, err := listOrgUnitListPage(context.Background(), store, "t1", orgUnitListPageRequest{
			AsOf:           "2026-01-01",
			IsBusinessUnit: &isBusinessUnit,
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if totalBusinessUnits != 1 || len(businessUnits) != 1 || businessUnits[0].OrgCode != "B002" {
			t.Fatalf("businessUnits=%+v total=%d", businessUnits, totalBusinessUnits)
		}

		roots, totalRoots, err := listOrgUnitListPage(context.Background(), store, "t1", orgUnitListPageRequest{
			AsOf: "2026-01-01",
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if totalRoots != 3 || len(roots) != 3 || roots[0].Status != orgUnitListStatusActive {
			t.Fatalf("roots=%+v total=%d", roots, totalRoots)
		}
		if roots[0].HasChildren == nil || !*roots[0].HasChildren {
			t.Fatalf("root has_children missing: %+v", roots[0])
		}

		errStore := &resolveOrgCodeStore{listNodesErr: errBoom{}}
		if _, _, err := listOrgUnitListPage(context.Background(), errStore, "t1", orgUnitListPageRequest{AsOf: "2026-01-01"}); err == nil {
			t.Fatalf("expected list nodes error")
		}

		childErrStore := &resolveOrgCodeStore{listChildrenErr: errBoom{}}
		parentOrgNodeKey, err := encodeOrgNodeKeyFromID(10000001)
		if err != nil {
			t.Fatalf("encode err=%v", err)
		}
		if _, _, err := listOrgUnitListPage(context.Background(), childErrStore, "t1", orgUnitListPageRequest{AsOf: "2026-01-01", ParentOrgNodeKey: &parentOrgNodeKey}); err == nil {
			t.Fatalf("expected list children error")
		}
	})
}

func TestListOrgUnitListPage_HydratesFallbackScopePath(t *testing.T) {
	store := newOrgUnitMemoryStore()
	ctx := context.Background()
	root, err := store.CreateNodeCurrent(ctx, "t1", "2026-01-01", "ROOT", "Root", "", false)
	if err != nil {
		t.Fatalf("create root err=%v", err)
	}
	child, err := store.CreateNodeCurrent(ctx, "t1", "2026-01-01", "CHILD", "Child", root.ID, false)
	if err != nil {
		t.Fatalf("create child err=%v", err)
	}
	grandchild, err := store.CreateNodeCurrent(ctx, "t1", "2026-01-01", "GRAND", "Grandchild", child.ID, false)
	if err != nil {
		t.Fatalf("create grandchild err=%v", err)
	}

	items, total, err := listOrgUnitListPage(ctx, store, "t1", orgUnitListPageRequest{
		AsOf:    "2026-01-01",
		Keyword: "grand",
	})
	if err != nil {
		t.Fatalf("list err=%v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("items=%+v total=%d", items, total)
	}
	wantPath := []string{root.ID, child.ID, grandchild.ID}
	if !reflect.DeepEqual(items[0].PathOrgNodeKeys, wantPath) {
		t.Fatalf("path=%v want=%v", items[0].PathOrgNodeKeys, wantPath)
	}

	scopeCtx := withPrincipal(ctx, Principal{ID: "principal-a"})
	filtered, filteredTotal, err := filterOrgUnitListItemsByCurrentScope(scopeCtx, &orgUnitScopeRuntimeStub{
		scopes: []principalOrgScope{{
			OrgNodeKey:         child.ID,
			IncludeDescendants: true,
		}},
	}, "t1", items, total)
	if err != nil {
		t.Fatalf("filter err=%v", err)
	}
	if filteredTotal != 1 || len(filtered) != 1 || filtered[0].OrgNodeKey != grandchild.ID {
		t.Fatalf("filtered=%+v total=%d", filtered, filteredTotal)
	}
}

func TestFilterOrgUnitListItemsByScope_AllowsEmptyCandidateList(t *testing.T) {
	t.Run("current principal", func(t *testing.T) {
		runtime := &orgUnitScopeRuntimeStub{err: errAuthzOrgScopeRequired}
		ctx := withPrincipal(context.Background(), Principal{ID: "principal-a"})
		filtered, total, err := filterOrgUnitListItemsByCurrentScope(ctx, runtime, "t1", []orgUnitListItem{}, 0)
		if err != nil {
			t.Fatalf("filter err=%v", err)
		}
		if len(filtered) != 0 || total != 0 {
			t.Fatalf("filtered=%+v total=%d", filtered, total)
		}
		if runtime.scopeCalls != 0 {
			t.Fatalf("scopeCalls=%d", runtime.scopeCalls)
		}
	})

	t.Run("explicit principal", func(t *testing.T) {
		runtime := &orgUnitScopeRuntimeStub{err: errAuthzOrgScopeRequired}
		filtered, total, err := filterOrgUnitListItemsByPrincipalScope(context.Background(), runtime, "t1", "principal-a", []orgUnitListItem{}, 0)
		if err != nil {
			t.Fatalf("filter err=%v", err)
		}
		if len(filtered) != 0 || total != 0 {
			t.Fatalf("filtered=%+v total=%d", filtered, total)
		}
		if runtime.scopeCalls != 0 {
			t.Fatalf("scopeCalls=%d", runtime.scopeCalls)
		}
	})
}

func TestHandleOrgUnitsAPI_ListNodesError(t *testing.T) {
	store := &resolveOrgCodeStore{listNodesErr: errBoom{}}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsAPI_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/org/api/org-units", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, newOrgUnitMemoryStore(), nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsAPI_TenantMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units", nil)
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, newOrgUnitMemoryStore(), nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsAPI_AsOfSuccess(t *testing.T) {
	store := &resolveOrgCodeStore{
		listNodes: []OrgUnitNode{{ID: "10000001", OrgCode: "A001", Name: "Root", HasChildren: true}},
	}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}

	var payload struct {
		OrgUnits []orgUnitListItem `json:"org_units"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode err=%v", err)
	}
	if len(payload.OrgUnits) != 1 || payload.OrgUnits[0].HasVisibleChildren == nil {
		t.Fatalf("unexpected roots payload: %+v", payload.OrgUnits)
	}
}

func TestHandleOrgUnitsDetailsAPI_Success(t *testing.T) {
	store := newOrgUnitMemoryStore()
	created, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-01", "A001", "Root", "", true)
	if err != nil {
		t.Fatalf("create err=%v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/details?org_code="+created.OrgCode+"&as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsDetailsAPI(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), created.OrgCode) {
		t.Fatalf("body=%q", rec.Body.String())
	}
}

func TestHandleOrgUnitsAPI_ParentOrgCodePassesNodeKeyToListReader(t *testing.T) {
	store := &resolveOrgCodeStore{
		resolveID:    10000001,
		listChildren: []OrgUnitChild{{OrgID: 10000002, OrgCode: "A002", Name: "Child", Status: "active"}},
	}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&parent_org_code=A001", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	want, err := encodeOrgNodeKeyFromID(10000001)
	if err != nil {
		t.Fatalf("encode err=%v", err)
	}
	if store.listChildrenByNodeKeyArg != want {
		t.Fatalf("parentOrgNodeKey=%q want=%q", store.listChildrenByNodeKeyArg, want)
	}
}

func TestHandleOrgUnitsDetailsAPI_BasicErrors(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/details?org_code=bad%7F&as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsDetailsAPI(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/org/api/org-units/details?org_code=A001&as_of=2026-01-01", nil)
	req2 = req2.WithContext(withTenant(req2.Context(), Tenant{ID: "t1", Name: "T"}))
	rec2 := httptest.NewRecorder()
	handleOrgUnitsDetailsAPI(rec2, req2, newOrgUnitMemoryStore())
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec2.Code)
	}
}

func TestHandleOrgUnitsVersionsAPI_Success(t *testing.T) {
	store := newOrgUnitMemoryStore()
	created, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-01", "A001", "Root", "", true)
	if err != nil {
		t.Fatalf("create err=%v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/versions?org_code="+created.OrgCode, nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsVersionsAPI(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "2026-01-01") {
		t.Fatalf("body=%q", rec.Body.String())
	}
}

func TestHandleOrgUnitsDetailsAPI_PassesNodeKeyToReader(t *testing.T) {
	store := &resolveOrgCodeStore{
		resolveID: 10000001,
		getNodeDetails: OrgUnitNodeDetails{
			OrgID:   10000001,
			OrgCode: "A001",
			Name:    "Root",
			Status:  "active",
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/details?org_code=A001&as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsDetailsAPI(rec, req, store)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	want, err := encodeOrgNodeKeyFromID(10000001)
	if err != nil {
		t.Fatalf("encode err=%v", err)
	}
	if store.detailsByNodeKeyArg != want {
		t.Fatalf("orgNodeKey=%q want=%q", store.detailsByNodeKeyArg, want)
	}
}

func TestHandleOrgUnitsDetailsAPI_PassesNodeKeyToExtSnapshotStore(t *testing.T) {
	orgNodeKey := mustOrgNodeKeyForTest(t, 10000001)
	store := &orgUnitDetailsExtStoreStub{
		resolveOrgCodeStore: &resolveOrgCodeStore{
			resolveID: 10000001,
			getNodeDetails: OrgUnitNodeDetails{
				OrgID:      10000001,
				OrgNodeKey: orgNodeKey,
				OrgCode:    "A001",
				Name:       "Root",
				Status:     "active",
			},
		},
		cfgs: []orgUnitTenantFieldConfig{
			{FieldKey: "short_name", PhysicalCol: "ext_str_01"},
		},
		snapshot: orgUnitVersionExtSnapshot{
			VersionValues: map[string]any{"ext_str_01": "R&D"},
			VersionLabels: map[string]string{},
			EventLabels:   map[string]string{},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/details?org_code=A001&as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsDetailsAPI(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.snapshotByNodeKeyArg != orgNodeKey {
		t.Fatalf("snapshotByNodeKeyArg=%q want=%q", store.snapshotByNodeKeyArg, orgNodeKey)
	}
	if store.snapshotOrgIDArg != 0 {
		t.Fatalf("snapshotOrgIDArg=%d want=0", store.snapshotOrgIDArg)
	}
}

func TestHandleOrgUnitsVersionsAPI_PassesNodeKeyToReader(t *testing.T) {
	store := &resolveOrgCodeStore{
		resolveID:        10000001,
		listNodeVersions: []OrgUnitNodeVersion{{EventID: 1, EffectiveDate: "2026-01-01", EventType: "CREATE"}},
	}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/versions?org_code=A001", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsVersionsAPI(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	want, err := encodeOrgNodeKeyFromID(10000001)
	if err != nil {
		t.Fatalf("encode err=%v", err)
	}
	if store.versionsByNodeKeyArg != want {
		t.Fatalf("orgNodeKey=%q want=%q", store.versionsByNodeKeyArg, want)
	}
}

func TestHandleOrgUnitsAuditAPI_PassesNodeKeyToReader(t *testing.T) {
	store := &resolveOrgCodeStore{
		resolveID:   10000001,
		auditEvents: []OrgUnitNodeAuditEvent{{EventID: 1, EventUUID: "e1", EventType: "CREATE", EffectiveDate: "2026-01-01"}},
	}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/audit?org_code=A001", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAuditAPI(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	want, err := encodeOrgNodeKeyFromID(10000001)
	if err != nil {
		t.Fatalf("encode err=%v", err)
	}
	if store.auditByNodeKeyArg != want {
		t.Fatalf("orgNodeKey=%q want=%q", store.auditByNodeKeyArg, want)
	}
}

func TestHandleOrgUnitsAuditAPI_Success(t *testing.T) {
	store := newOrgUnitMemoryStore()
	created, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-01", "A001", "Root", "", true)
	if err != nil {
		t.Fatalf("create err=%v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/audit?org_code="+created.OrgCode+"&limit=1", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAuditAPI(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"events\"") {
		t.Fatalf("body=%q", rec.Body.String())
	}
}

func TestHandleOrgUnitsSearchAPI_Success(t *testing.T) {
	store := newOrgUnitMemoryStore()
	_, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-01", "A001", "Root", "", true)
	if err != nil {
		t.Fatalf("create err=%v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/search?query=A001&as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsSearchAPI(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"target_org_code\":\"A001\"") {
		t.Fatalf("body=%q", rec.Body.String())
	}
}

func TestHandleOrgUnitsSearchAPI_AmbiguousCandidates(t *testing.T) {
	store := newOrgUnitMemoryStore()
	for _, item := range []struct {
		code string
		name string
	}{
		{code: "A001", name: "East Sales Center"},
		{code: "A002", name: "East Operations Center"},
	} {
		if _, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-01", item.code, item.name, "", true); err != nil {
			t.Fatalf("create %s err=%v", item.code, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/search?query=East&as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsSearchAPI(rec, req, store)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, expected := range []string{
		`"error_code":"org_unit_search_ambiguous"`,
		`"cannot_silent_select":true`,
		`"org_code":"A001"`,
		`"org_code":"A002"`,
	} {
		if !strings.Contains(rec.Body.String(), expected) {
			t.Fatalf("expected body to contain %q, got %s", expected, rec.Body.String())
		}
	}
}

func TestHandleOrgUnitsSearchAPI_ReturnsOnlyVisibleCandidate(t *testing.T) {
	store := newOrgUnitMemoryStore()
	for _, item := range []struct {
		code string
		name string
	}{
		{code: "A001", name: "East Center"},
		{code: "A002", name: "East Center"},
	} {
		if _, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-01", item.code, item.name, "", true); err != nil {
			t.Fatalf("create %s err=%v", item.code, err)
		}
	}
	visibleOrgNodeKey, err := store.ResolveOrgNodeKeyByCode(context.Background(), "t1", "A002")
	if err != nil {
		t.Fatalf("resolve org node key err=%v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/search?query=East&as_of=2026-01-01", nil)
	ctx := withTenant(req.Context(), Tenant{ID: "t1", Name: "T"})
	ctx = withPrincipal(ctx, Principal{ID: "principal-a"})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	runtime := &orgUnitScopeRuntimeStub{scopes: []principalOrgScope{{
		OrgNodeKey: visibleOrgNodeKey,
	}}}
	handleOrgUnitsSearchAPI(rec, req, store, runtime)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var result OrgUnitSearchResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode err=%v", err)
	}
	if result.TargetOrgCode != "A002" {
		t.Fatalf("target_org_code=%q result=%+v body=%s", result.TargetOrgCode, result, rec.Body.String())
	}
	if len(result.PathOrgCodes) != 1 || result.PathOrgCodes[0] != "A002" {
		t.Fatalf("path_org_codes=%v", result.PathOrgCodes)
	}
}

func TestHandleOrgUnitsSearchAPI_ScopeFilterScansPastInitialCandidateLimit(t *testing.T) {
	store := newOrgUnitMemoryStore()
	for i := 1; i <= 9; i++ {
		code := fmt.Sprintf("A%03d", i)
		if _, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-01", code, "East Center", "", true); err != nil {
			t.Fatalf("create %s err=%v", code, err)
		}
	}
	visibleOrgNodeKey, err := store.ResolveOrgNodeKeyByCode(context.Background(), "t1", "A009")
	if err != nil {
		t.Fatalf("resolve org node key err=%v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/search?query=East&as_of=2026-01-01", nil)
	ctx := withTenant(req.Context(), Tenant{ID: "t1", Name: "T"})
	ctx = withPrincipal(ctx, Principal{ID: "principal-a"})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	runtime := &orgUnitScopeRuntimeStub{scopes: []principalOrgScope{{
		OrgNodeKey: visibleOrgNodeKey,
	}}}
	handleOrgUnitsSearchAPI(rec, req, store, runtime)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var result OrgUnitSearchResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode err=%v", err)
	}
	if result.TargetOrgCode != "A009" {
		t.Fatalf("target_org_code=%q result=%+v", result.TargetOrgCode, result)
	}
}

func TestHandleOrgUnitsSearchAPI_BasicErrors(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/search?query=&as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsSearchAPI(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/org/api/org-units/search?query=A001&as_of=2026-01-01", nil)
	req2 = req2.WithContext(withTenant(req2.Context(), Tenant{ID: "t1", Name: "T"}))
	rec2 := httptest.NewRecorder()
	handleOrgUnitsSearchAPI(rec2, req2, newOrgUnitMemoryStore())
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec2.Code)
	}
}
