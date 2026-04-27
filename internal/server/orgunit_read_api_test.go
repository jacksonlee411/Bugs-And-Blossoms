package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

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
		{name: "status invalid", rawURL: "/org/api/org-units?status=bad", wantErr: true},
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
	if len(payload.OrgUnits) != 1 || payload.OrgUnits[0].HasChildren == nil || !*payload.OrgUnits[0].HasChildren {
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
