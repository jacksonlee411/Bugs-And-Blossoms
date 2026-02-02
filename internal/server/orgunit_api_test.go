package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

func TestHandleOrgUnitsBusinessUnitAPI_TenantMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/orgunit/api/org-units/set-business-unit", nil)
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/orgunit/api/org-units/set-business-unit", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_BadJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/orgunit/api/org-units/set-business-unit", strings.NewReader("{"))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_InvalidRequest(t *testing.T) {
	body := bytes.NewBufferString(`{"org_unit_id":"","effective_date":"","request_code":""}`)
	req := httptest.NewRequest(http.MethodPost, "/orgunit/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_BothOrgIDAndCode(t *testing.T) {
	body := bytes.NewBufferString(`{"org_unit_id":"10000001","org_code":"A1","effective_date":"2026-01-01","request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/orgunit/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_InvalidEffectiveDate(t *testing.T) {
	body := bytes.NewBufferString(`{"org_unit_id":"10000001","effective_date":"bad","is_business_unit":true,"request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/orgunit/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_InvalidOrgID(t *testing.T) {
	body := bytes.NewBufferString(`{"org_unit_id":"123","effective_date":"2026-01-01","is_business_unit":true,"request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/orgunit/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_StoreError(t *testing.T) {
	body := bytes.NewBufferString(`{"org_unit_id":"10000001","effective_date":"2026-01-01","is_business_unit":true,"request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/orgunit/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, errOrgUnitStore{err: errBoom{}})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

type resolveOrgCodeStore struct {
	resolveID  int
	resolveErr error
	setErr     error
	setArgs    []string
}

func (s *resolveOrgCodeStore) ListNodesCurrent(context.Context, string, string) ([]OrgUnitNode, error) {
	return nil, nil
}
func (s *resolveOrgCodeStore) CreateNodeCurrent(context.Context, string, string, string, string, bool) (OrgUnitNode, error) {
	return OrgUnitNode{}, nil
}
func (s *resolveOrgCodeStore) RenameNodeCurrent(context.Context, string, string, string, string) error {
	return nil
}
func (s *resolveOrgCodeStore) MoveNodeCurrent(context.Context, string, string, string, string) error {
	return nil
}
func (s *resolveOrgCodeStore) DisableNodeCurrent(context.Context, string, string, string) error {
	return nil
}
func (s *resolveOrgCodeStore) SetBusinessUnitCurrent(_ context.Context, tenantID string, effectiveDate string, orgID string, _ bool, requestCode string) error {
	s.setArgs = []string{tenantID, effectiveDate, orgID, requestCode}
	return s.setErr
}
func (s *resolveOrgCodeStore) ResolveOrgID(context.Context, string, string) (int, error) {
	if s.resolveErr != nil {
		return 0, s.resolveErr
	}
	return s.resolveID, nil
}
func (s *resolveOrgCodeStore) ResolveOrgCode(context.Context, string, int) (string, error) {
	return "", nil
}

func TestHandleOrgUnitsBusinessUnitAPI_OrgCodeInvalid(t *testing.T) {
	store := &resolveOrgCodeStore{resolveErr: orgunitpkg.ErrOrgCodeInvalid}
	body := bytes.NewBufferString(`{"org_code":"bad","effective_date":"2026-01-01","is_business_unit":true,"request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/orgunit/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, store)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_OrgCodeNotFound(t *testing.T) {
	store := &resolveOrgCodeStore{resolveErr: orgunitpkg.ErrOrgCodeNotFound}
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true,"request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/orgunit/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, store)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_OrgCodeResolveError(t *testing.T) {
	store := &resolveOrgCodeStore{resolveErr: errBoom{}}
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true,"request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/orgunit/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, store)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_OrgCodeSuccess(t *testing.T) {
	store := &resolveOrgCodeStore{resolveID: 10000001}
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true,"request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/orgunit/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, store)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d", rec.Code)
	}
	if len(store.setArgs) != 4 || store.setArgs[2] != "10000001" {
		t.Fatalf("unexpected set args: %+v", store.setArgs)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_Success(t *testing.T) {
	store := newOrgUnitMemoryStore()
	created, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-01", "Org1", "", false)
	if err != nil {
		t.Fatalf("err=%v", err)
	}

	body := bytes.NewBufferString(`{"org_unit_id":"` + created.ID + `","effective_date":"2026-01-01","is_business_unit":true,"request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/orgunit/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, store)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), created.ID) {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}
