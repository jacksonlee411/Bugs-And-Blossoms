package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type extListStoreStub struct {
	*resolveOrgCodeStore
	listFn func(ctx context.Context, tenantID string, req orgUnitListPageRequest) ([]orgUnitListItem, int, error)
}

func (s *extListStoreStub) ListOrgUnitsPage(ctx context.Context, tenantID string, req orgUnitListPageRequest) ([]orgUnitListItem, int, error) {
	if s.listFn != nil {
		return s.listFn(ctx, tenantID, req)
	}
	return []orgUnitListItem{}, 0, nil
}

func TestHandleOrgUnitsAPI_ExtQueryRequiresGridOrPagination(t *testing.T) {
	store := newOrgUnitMemoryStore()
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&sort=ext:org_type", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if got := payload["code"]; got != "invalid_request" {
		t.Fatalf("code=%v", got)
	}
}

func TestHandleOrgUnitsAPI_ExtQueryNotAllowedForParentOrgCode(t *testing.T) {
	store := newOrgUnitMemoryStore()
	_, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-01", "A001", "Root", "", true)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&mode=grid&parent_org_code=A001&sort=ext:org_type", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if got := payload["code"]; got != "invalid_request" {
		t.Fatalf("code=%v", got)
	}
}

func TestHandleOrgUnitsAPI_ExtQueryStoreMissing(t *testing.T) {
	store := newOrgUnitMemoryStore()
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&mode=grid&sort=ext:org_type", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if got := payload["code"]; got != orgUnitErrExtQueryFieldNotAllowed {
		t.Fatalf("code=%v", got)
	}
}

func TestHandleOrgUnitsAPI_ExtQueryStoreRejectsNotAllowed(t *testing.T) {
	store := &extListStoreStub{
		resolveOrgCodeStore: &resolveOrgCodeStore{},
		listFn: func(_ context.Context, _ string, _ orgUnitListPageRequest) ([]orgUnitListItem, int, error) {
			return nil, 0, errOrgUnitExtQueryFieldNotAllowed
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&mode=grid&sort=ext:org_type", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if got := payload["code"]; got != orgUnitErrExtQueryFieldNotAllowed {
		t.Fatalf("code=%v", got)
	}
}

func TestHandleOrgUnitsAPI_ExtQuerySuccess_PassesParams(t *testing.T) {
	var gotReq orgUnitListPageRequest
	store := &extListStoreStub{
		resolveOrgCodeStore: &resolveOrgCodeStore{},
		listFn: func(_ context.Context, _ string, req orgUnitListPageRequest) ([]orgUnitListItem, int, error) {
			gotReq = req
			return []orgUnitListItem{{OrgCode: "A001", Name: "Root", Status: "active"}}, 1, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01&mode=grid&sort=ext:org_type&order=desc&ext_filter_field_key=org_type&ext_filter_value=DEPARTMENT&page=0&size=10", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, store, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	if gotReq.ExtSortFieldKey != "org_type" {
		t.Fatalf("ExtSortFieldKey=%q", gotReq.ExtSortFieldKey)
	}
	if gotReq.SortOrder != "desc" {
		t.Fatalf("SortOrder=%q", gotReq.SortOrder)
	}
	if gotReq.ExtFilterFieldKey != "org_type" || gotReq.ExtFilterValue != "DEPARTMENT" {
		t.Fatalf("filter=%q value=%q", gotReq.ExtFilterFieldKey, gotReq.ExtFilterValue)
	}
	if gotReq.Limit != 10 || gotReq.Offset != 0 {
		t.Fatalf("limit=%d offset=%d", gotReq.Limit, gotReq.Offset)
	}
}
