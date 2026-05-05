package server

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
)

func TestHandleOrgUnitsBusinessUnitAPI_TenantMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", nil)
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, orgUnitWriteServiceStub{})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/set-business-unit", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, orgUnitWriteServiceStub{})
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_BadJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", strings.NewReader("{"))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, orgUnitWriteServiceStub{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_InvalidRequest(t *testing.T) {
	body := bytes.NewBufferString(`{"org_code":"","effective_date":"","request_id":""}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, orgUnitWriteServiceStub{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_BothOrgIDAndCode(t *testing.T) {
	body := bytes.NewBufferString(`{"org_unit_id":"10000001","org_code":"A1","effective_date":"2026-01-01","request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, orgUnitWriteServiceStub{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_InvalidEffectiveDate(t *testing.T) {
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"bad","is_business_unit":true,"request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, orgUnitWriteServiceStub{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_OrgUnitIDProvided(t *testing.T) {
	body := bytes.NewBufferString(`{"org_unit_id":"123","effective_date":"2026-01-01","is_business_unit":true,"request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, orgUnitWriteServiceStub{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_WriteServicePath(t *testing.T) {
	var captured orgunitservices.SetBusinessUnitRequest
	svc := orgUnitWriteServiceStub{
		setBusinessUnitFn: func(_ context.Context, tenantID string, req orgunitservices.SetBusinessUnitRequest) error {
			if tenantID != "t1" {
				t.Fatalf("tenant=%s", tenantID)
			}
			captured = req
			return nil
		},
	}

	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true,"ext":{"org_type":"DEPARTMENT"}}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, svc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if captured.OrgCode != "A1" || captured.EffectiveDate != "2026-01-01" || !captured.IsBusinessUnit {
		t.Fatalf("captured=%+v", captured)
	}
	if captured.Ext["org_type"] != "DEPARTMENT" {
		t.Fatalf("ext=%v", captured.Ext)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_WriteServiceRejectsExtLabelsSnapshot(t *testing.T) {
	svc := orgUnitWriteServiceStub{}
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true,"ext_labels_snapshot":{"org_type":"Department"}}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, svc)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_WriteServiceRejectsUnknownField(t *testing.T) {
	svc := orgUnitWriteServiceStub{}
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true,"request_id":"req-unknown","unknown":1}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, svc)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_WriteServiceValidationAndErrorMapping(t *testing.T) {
	t.Run("effective date required", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", bytes.NewBufferString(`{"org_code":"A1","effective_date":"   ","is_business_unit":true}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsBusinessUnitAPI(rec, req, orgUnitWriteServiceStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("org code required", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", bytes.NewBufferString(`{"org_unit_id":"10000001","effective_date":"2026-01-01","is_business_unit":true}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsBusinessUnitAPI(rec, req, orgUnitWriteServiceStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("service error mapped", func(t *testing.T) {
		svc := orgUnitWriteServiceStub{
			setBusinessUnitFn: func(context.Context, string, orgunitservices.SetBusinessUnitRequest) error {
				return errors.New("ORG_INVALID_ARGUMENT")
			},
		}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsBusinessUnitAPI(rec, req, svc)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestHandleOrgUnitsBusinessUnitAPI_ServiceMissing(t *testing.T) {
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
