package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
