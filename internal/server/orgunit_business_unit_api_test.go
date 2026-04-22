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
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

func TestHandleOrgUnitsBusinessUnitAPI_TenantMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", nil)
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/set-business-unit", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_BadJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", strings.NewReader("{"))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_InvalidRequest(t *testing.T) {
	body := bytes.NewBufferString(`{"org_code":"","effective_date":"","request_id":""}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_BothOrgIDAndCode(t *testing.T) {
	body := bytes.NewBufferString(`{"org_unit_id":"10000001","org_code":"A1","effective_date":"2026-01-01","request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_InvalidEffectiveDate(t *testing.T) {
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"bad","is_business_unit":true,"request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, &resolveOrgCodeStore{resolveID: 10000001})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_OrgUnitIDProvided(t *testing.T) {
	body := bytes.NewBufferString(`{"org_unit_id":"123","effective_date":"2026-01-01","is_business_unit":true,"request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_StoreError(t *testing.T) {
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true,"request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, struct{}{})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_OrgCodeInvalid(t *testing.T) {
	store := &resolveOrgCodeStore{}
	body := bytes.NewBufferString(`{"org_code":"bad\u007f","effective_date":"2026-01-01","is_business_unit":true,"request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, store)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_OrgCodeNotFound(t *testing.T) {
	store := &resolveOrgCodeStore{resolveErr: orgunitpkg.ErrOrgCodeNotFound}
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true,"request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, store)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_OrgCodeResolveInvalid(t *testing.T) {
	store := &resolveOrgCodeStore{resolveErr: orgunitpkg.ErrOrgCodeInvalid}
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true,"request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, store)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_OrgCodeResolveError(t *testing.T) {
	store := &resolveOrgCodeStore{resolveErr: errBoom{}}
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true,"request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, store)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_SetBusinessUnitError(t *testing.T) {
	store := &resolveOrgCodeStore{resolveID: 10000001, setErr: errBoom{}}
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true,"request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, store)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_OrgCodeSuccess(t *testing.T) {
	store := &resolveOrgCodeStore{resolveID: 10000001}
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true,"request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if len(store.setArgs) != 4 || store.setArgs[2] != "AAAKTFWB" {
		t.Fatalf("unexpected set args: %+v", store.setArgs)
	}
}

func TestHandleOrgUnitsBusinessUnitAPI_Success(t *testing.T) {
	store := newOrgUnitMemoryStore()
	created, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-01", "ORG1", "Org1", "", false)
	if err != nil {
		t.Fatalf("err=%v", err)
	}

	body := bytes.NewBufferString(`{"org_code":"` + created.OrgCode + `","effective_date":"2026-01-01","is_business_unit":true,"request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), created.OrgCode) {
		t.Fatalf("unexpected body: %q", rec.Body.String())
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

func TestHandleOrgUnitsBusinessUnitAPI_DependencyMissing(t *testing.T) {
	body := bytes.NewBufferString(`{"org_code":"A1","effective_date":"2026-01-01","is_business_unit":true}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/set-business-unit", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsBusinessUnitAPI(rec, req, struct{}{})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
