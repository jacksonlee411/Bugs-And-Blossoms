package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	orgunittypes "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
)

func TestHandleOrgUnitsAPI_PostMissingService(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units", strings.NewReader(`{}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, newOrgUnitMemoryStore(), nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsAPI_CreateSuccess(t *testing.T) {
	svc := orgUnitWriteServiceStub{
		createFn: func(_ context.Context, tenantID string, req orgunitservices.CreateOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			if tenantID != "t1" {
				return orgunittypes.OrgUnitResult{}, errors.New("bad tenant")
			}
			return orgunittypes.OrgUnitResult{
				OrgCode:       "A001",
				EffectiveDate: req.EffectiveDate,
				Fields:        map[string]any{"name": req.Name},
			}, nil
		},
	}
	body := strings.NewReader(`{"org_code":"A001","name":"Root","effective_date":"2026-01-01"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, newOrgUnitMemoryStore(), svc)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "A001") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleOrgUnitsRenameAPI_NotFound(t *testing.T) {
	svc := orgUnitWriteServiceStub{
		renameFn: func(context.Context, string, orgunitservices.RenameOrgUnitRequest) error {
			return errors.New("ORG_CODE_NOT_FOUND")
		},
	}
	body := strings.NewReader(`{"org_code":"A001","new_name":"New","effective_date":"2026-01-01"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rename", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsRenameAPI(rec, req, svc)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleOrgUnitsRenameAPI_BadJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rename", strings.NewReader("{"))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsRenameAPI(rec, req, orgUnitWriteServiceStub{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsRenameAPI_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/rename", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsRenameAPI(rec, req, orgUnitWriteServiceStub{})
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsRenameAPI_TenantMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rename", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	handleOrgUnitsRenameAPI(rec, req, orgUnitWriteServiceStub{})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsRenameAPI_ServiceMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rename", strings.NewReader(`{}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsRenameAPI(rec, req, nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsAPI_BadJSON(t *testing.T) {
	svc := orgUnitWriteServiceStub{}
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units", strings.NewReader("{"))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, newOrgUnitMemoryStore(), svc)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsAPI_CreateRejectsExtLabelsSnapshot(t *testing.T) {
	svc := orgUnitWriteServiceStub{
		createFn: func(context.Context, string, orgunitservices.CreateOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			t.Fatalf("create should not be called when ext_labels_snapshot is provided")
			return orgunittypes.OrgUnitResult{}, nil
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units", strings.NewReader(`{"org_code":"A001","name":"Root","effective_date":"2026-01-01","ext_labels_snapshot":{"org_type":"Department"}}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, newOrgUnitMemoryStore(), svc)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleOrgUnitsAPI_CreateError(t *testing.T) {
	svc := orgUnitWriteServiceStub{
		createFn: func(context.Context, string, orgunitservices.CreateOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			return orgunittypes.OrgUnitResult{}, errors.New("ORG_CODE_INVALID")
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units", strings.NewReader(`{"org_code":"bad","name":"Root","effective_date":"2026-01-01"}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsAPI(rec, req, newOrgUnitMemoryStore(), svc)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitWriteAPIs_RequireExplicitEffectiveDate(t *testing.T) {
	tests := []struct {
		name string
		url  string
		body string
		call func(http.ResponseWriter, *http.Request, orgunitservices.OrgUnitWriteService)
	}{
		{
			name: "create",
			url:  "/org/api/org-units",
			body: `{"org_code":"A001","name":"Root"}`,
			call: func(w http.ResponseWriter, r *http.Request, svc orgunitservices.OrgUnitWriteService) {
				handleOrgUnitsAPI(w, r, newOrgUnitMemoryStore(), svc)
			},
		},
		{
			name: "rename",
			url:  "/org/api/org-units/rename",
			body: `{"org_code":"A001","new_name":"New","ext":{"org_type":"DEPARTMENT"}}`,
			call: handleOrgUnitsRenameAPI,
		},
		{
			name: "move",
			url:  "/org/api/org-units/move",
			body: `{"org_code":"A001","new_parent_org_code":"A0001"}`,
			call: handleOrgUnitsMoveAPI,
		},
		{
			name: "disable",
			url:  "/org/api/org-units/disable",
			body: `{"org_code":"A001"}`,
			call: handleOrgUnitsDisableAPI,
		},
		{
			name: "enable",
			url:  "/org/api/org-units/enable",
			body: `{"org_code":"A001"}`,
			call: handleOrgUnitsEnableAPI,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := orgUnitWriteServiceStub{
				createFn: func(context.Context, string, orgunitservices.CreateOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
					t.Fatalf("create should not be called when effective_date is missing")
					return orgunittypes.OrgUnitResult{}, nil
				},
				renameFn: func(context.Context, string, orgunitservices.RenameOrgUnitRequest) error {
					t.Fatalf("rename should not be called when effective_date is missing")
					return nil
				},
				moveFn: func(context.Context, string, orgunitservices.MoveOrgUnitRequest) error {
					t.Fatalf("move should not be called when effective_date is missing")
					return nil
				},
				disableFn: func(context.Context, string, orgunitservices.DisableOrgUnitRequest) error {
					t.Fatalf("disable should not be called when effective_date is missing")
					return nil
				},
				enableFn: func(context.Context, string, orgunitservices.EnableOrgUnitRequest) error {
					t.Fatalf("enable should not be called when effective_date is missing")
					return nil
				},
			}
			req := httptest.NewRequest(http.MethodPost, tt.url, strings.NewReader(tt.body))
			req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
			rec := httptest.NewRecorder()
			tt.call(rec, req, svc)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), `"code":"invalid_effective_date"`) || !strings.Contains(rec.Body.String(), "effective_date required") {
				t.Fatalf("unexpected body=%s", rec.Body.String())
			}
		})
	}
}

func TestHandleOrgUnitsMoveAPI_Success(t *testing.T) {
	called := false
	svc := orgUnitWriteServiceStub{
		moveFn: func(_ context.Context, _ string, req orgunitservices.MoveOrgUnitRequest) error {
			called = true
			if req.NewParentOrgCode == "" {
				t.Fatalf("expected parent org code")
			}
			if req.Ext["org_type"] != "DEPARTMENT" {
				t.Fatalf("ext=%v", req.Ext)
			}
			return nil
		},
	}
	body := strings.NewReader(`{"org_code":"A001","new_parent_org_code":"A0001","effective_date":"2026-01-01","ext":{"org_type":"DEPARTMENT"}}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/move", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsMoveAPI(rec, req, svc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !called {
		t.Fatalf("expected move call")
	}
}

func TestHandleOrgUnitsMoveAPI_BadJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/move", strings.NewReader("{"))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsMoveAPI(rec, req, orgUnitWriteServiceStub{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsDisableAPI_Success(t *testing.T) {
	called := false
	svc := orgUnitWriteServiceStub{
		disableFn: func(_ context.Context, _ string, req orgunitservices.DisableOrgUnitRequest) error {
			called = true
			if req.OrgCode == "" {
				t.Fatalf("expected org code")
			}
			if req.Ext["org_type"] != "DEPARTMENT" {
				t.Fatalf("ext=%v", req.Ext)
			}
			return nil
		},
	}
	body := strings.NewReader(`{"org_code":"A001","effective_date":"2026-01-01","ext":{"org_type":"DEPARTMENT"}}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/disable", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsDisableAPI(rec, req, svc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !called {
		t.Fatalf("expected disable call")
	}
}

func TestHandleOrgUnitsDisableAPI_BadJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/disable", strings.NewReader("{"))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsDisableAPI(rec, req, orgUnitWriteServiceStub{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsEnableAPI_Success(t *testing.T) {
	called := false
	svc := orgUnitWriteServiceStub{
		enableFn: func(_ context.Context, _ string, req orgunitservices.EnableOrgUnitRequest) error {
			called = true
			if req.OrgCode == "" {
				t.Fatalf("expected org code")
			}
			if req.Ext["org_type"] != "DEPARTMENT" {
				t.Fatalf("ext=%v", req.Ext)
			}
			return nil
		},
	}
	body := strings.NewReader(`{"org_code":"A001","effective_date":"2026-01-01","ext":{"org_type":"DEPARTMENT"}}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/enable", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsEnableAPI(rec, req, svc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !called {
		t.Fatalf("expected enable call")
	}
}

func TestHandleOrgUnitsEnableAPI_BadJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/enable", strings.NewReader("{"))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsEnableAPI(rec, req, orgUnitWriteServiceStub{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsAppendActionAPIs_RejectExtLabelsSnapshot(t *testing.T) {
	tests := []struct {
		name string
		url  string
		body string
		call func(http.ResponseWriter, *http.Request, orgunitservices.OrgUnitWriteService)
	}{
		{
			name: "rename",
			url:  "/org/api/org-units/rename",
			body: `{"org_code":"A001","new_name":"New","effective_date":"2026-01-01","ext_labels_snapshot":{"org_type":"Department"}}`,
			call: handleOrgUnitsRenameAPI,
		},
		{
			name: "move",
			url:  "/org/api/org-units/move",
			body: `{"org_code":"A001","new_parent_org_code":"A0001","effective_date":"2026-01-01","ext_labels_snapshot":{"org_type":"Department"}}`,
			call: handleOrgUnitsMoveAPI,
		},
		{
			name: "disable",
			url:  "/org/api/org-units/disable",
			body: `{"org_code":"A001","effective_date":"2026-01-01","ext_labels_snapshot":{"org_type":"Department"}}`,
			call: handleOrgUnitsDisableAPI,
		},
		{
			name: "enable",
			url:  "/org/api/org-units/enable",
			body: `{"org_code":"A001","effective_date":"2026-01-01","ext_labels_snapshot":{"org_type":"Department"}}`,
			call: handleOrgUnitsEnableAPI,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.url, strings.NewReader(tt.body))
			req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
			rec := httptest.NewRecorder()
			tt.call(rec, req, orgUnitWriteServiceStub{})
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestHandleOrgUnitsCorrectionsAPI_Success(t *testing.T) {
	called := false
	svc := orgUnitWriteServiceStub{
		correctFn: func(_ context.Context, tenantID string, req orgunitservices.CorrectOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			called = true
			if tenantID != "t1" {
				t.Fatalf("unexpected tenant: %s", tenantID)
			}
			if req.TargetEffectiveDate != "2026-01-01" {
				t.Fatalf("unexpected effective date: %s", req.TargetEffectiveDate)
			}
			if req.RequestID != "r1" {
				t.Fatalf("unexpected request_id: %s", req.RequestID)
			}
			if req.Patch.Name == nil || *req.Patch.Name != "New Name" {
				t.Fatalf("expected patch name")
			}
			if req.Patch.Ext["org_type"] != "DEPARTMENT" {
				t.Fatalf("expected ext payload forwarded, got=%v", req.Patch.Ext)
			}
			return orgunittypes.OrgUnitResult{
				OrgCode:       "A001",
				EffectiveDate: "2026-01-01",
				Fields:        map[string]any{"name": "New Name"},
			}, nil
		},
	}
	body := strings.NewReader(`{"org_code":"A001","effective_date":"2026-01-01","patch":{"name":"New Name","ext":{"org_type":"DEPARTMENT"}},"request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/corrections", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsCorrectionsAPI(rec, req, svc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !called {
		t.Fatalf("expected correct call")
	}
	if !strings.Contains(rec.Body.String(), "A001") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleOrgUnitsCorrectionsAPI_RejectClientExtLabelsSnapshot(t *testing.T) {
	svc := orgUnitWriteServiceStub{
		correctFn: func(context.Context, string, orgunitservices.CorrectOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			t.Fatalf("service should not be called")
			return orgunittypes.OrgUnitResult{}, nil
		},
	}
	body := strings.NewReader(`{"org_code":"A001","effective_date":"2026-01-01","patch":{"ext":{"org_type":"DEPARTMENT"},"ext_labels_snapshot":{"org_type":"Department"}},"request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/corrections", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsCorrectionsAPI(rec, req, svc)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), orgUnitErrPatchFieldNotAllowed) {
		t.Fatalf("body=%q", rec.Body.String())
	}
}

func TestHandleOrgUnitsCorrectionsAPI_TenantMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/corrections", strings.NewReader(`{"org_code":"A001"}`))
	rec := httptest.NewRecorder()
	handleOrgUnitsCorrectionsAPI(rec, req, orgUnitWriteServiceStub{})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsCorrectionsAPI_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/corrections", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsCorrectionsAPI(rec, req, orgUnitWriteServiceStub{})
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsCorrectionsAPI_BadJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/corrections", strings.NewReader("{"))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsCorrectionsAPI(rec, req, orgUnitWriteServiceStub{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsCorrectionsAPI_MissingService(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/corrections", strings.NewReader(`{"org_code":"A001","effective_date":"2026-01-01","patch":{"name":"New"},"request_id":"r1"}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsCorrectionsAPI(rec, req, nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsCorrectionsAPI_NotFound(t *testing.T) {
	svc := orgUnitWriteServiceStub{
		correctFn: func(context.Context, string, orgunitservices.CorrectOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			return orgunittypes.OrgUnitResult{}, errors.New("ORG_EVENT_NOT_FOUND")
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/corrections", strings.NewReader(`{"org_code":"A001","effective_date":"2026-01-01","patch":{"name":"New"},"request_id":"r1"}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsCorrectionsAPI(rec, req, svc)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgUnitsStatusCorrectionsAPI_Success(t *testing.T) {
	called := false
	svc := orgUnitWriteServiceStub{
		correctStatusFn: func(_ context.Context, tenantID string, req orgunitservices.CorrectStatusOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			called = true
			if tenantID != "t1" {
				t.Fatalf("tenant=%s", tenantID)
			}
			if req.OrgCode != "A001" || req.TargetEffectiveDate != "2026-01-01" || req.TargetStatus != "disabled" || req.RequestID != "r1" {
				t.Fatalf("unexpected request: %+v", req)
			}
			return orgunittypes.OrgUnitResult{OrgCode: "A001", EffectiveDate: "2026-01-01", Fields: map[string]any{"target_status": "disabled"}}, nil
		},
	}
	body := strings.NewReader(`{"org_code":"A001","effective_date":"2026-01-01","target_status":"disabled","request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/status-corrections", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsStatusCorrectionsAPI(rec, req, svc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !called {
		t.Fatalf("expected correct status call")
	}
	if !strings.Contains(rec.Body.String(), "A001") {
		t.Fatalf("body=%q", rec.Body.String())
	}
}

func TestHandleOrgUnitsStatusCorrectionsAPI_BasicErrors(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/status-corrections", strings.NewReader(`{"org_code":"A001"}`))
		rec := httptest.NewRecorder()
		handleOrgUnitsStatusCorrectionsAPI(rec, req, orgUnitWriteServiceStub{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/status-corrections", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsStatusCorrectionsAPI(rec, req, orgUnitWriteServiceStub{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("bad json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/status-corrections", strings.NewReader("{"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsStatusCorrectionsAPI(rec, req, orgUnitWriteServiceStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("service missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/status-corrections", strings.NewReader(`{"org_code":"A001","effective_date":"2026-01-01","target_status":"disabled","request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsStatusCorrectionsAPI(rec, req, nil)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("status correction unsupported", func(t *testing.T) {
		svc := orgUnitWriteServiceStub{correctStatusFn: func(context.Context, string, orgunitservices.CorrectStatusOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			return orgunittypes.OrgUnitResult{}, errors.New("ORG_STATUS_CORRECTION_UNSUPPORTED_TARGET")
		}}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/status-corrections", strings.NewReader(`{"org_code":"A001","effective_date":"2026-01-01","target_status":"disabled","request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsStatusCorrectionsAPI(rec, req, svc)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestHandleOrgUnitsRescindsAPI_Success(t *testing.T) {
	called := false
	svc := orgUnitWriteServiceStub{
		rescindRecordFn: func(_ context.Context, tenantID string, req orgunitservices.RescindRecordOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			called = true
			if tenantID != "t1" {
				t.Fatalf("tenant=%s", tenantID)
			}
			if req.OrgCode != "A001" || req.TargetEffectiveDate != "2026-01-01" || req.RequestID != "r1" || req.Reason != "bad" {
				t.Fatalf("unexpected request: %+v", req)
			}
			return orgunittypes.OrgUnitResult{OrgCode: "A001", EffectiveDate: "2026-01-01"}, nil
		},
	}
	body := strings.NewReader(`{"org_code":"A001","effective_date":"2026-01-01","request_id":"r1","reason":"bad"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsRescindsAPI(rec, req, svc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !called {
		t.Fatalf("expected rescind call")
	}
	if !strings.Contains(rec.Body.String(), "RESCIND_EVENT") {
		t.Fatalf("body=%q", rec.Body.String())
	}
}

func TestHandleOrgUnitsRescindsAPI_BasicErrors(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds", strings.NewReader(`{"org_code":"A001"}`))
		rec := httptest.NewRecorder()
		handleOrgUnitsRescindsAPI(rec, req, orgUnitWriteServiceStub{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/rescinds", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsRescindsAPI(rec, req, orgUnitWriteServiceStub{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("bad json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds", strings.NewReader("{"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsRescindsAPI(rec, req, orgUnitWriteServiceStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("service missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds", strings.NewReader(`{"org_code":"A001"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsRescindsAPI(rec, req, nil)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("not found", func(t *testing.T) {
		svc := orgUnitWriteServiceStub{rescindRecordFn: func(context.Context, string, orgunitservices.RescindRecordOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			return orgunittypes.OrgUnitResult{}, errors.New("ORG_EVENT_NOT_FOUND")
		}}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds", strings.NewReader(`{"org_code":"A001","effective_date":"2026-01-01","request_id":"r1","reason":"bad"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsRescindsAPI(rec, req, svc)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("request id conflict", func(t *testing.T) {
		svc := orgUnitWriteServiceStub{rescindRecordFn: func(context.Context, string, orgunitservices.RescindRecordOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			return orgunittypes.OrgUnitResult{}, errors.New("ORG_REQUEST_ID_CONFLICT")
		}}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds", strings.NewReader(`{"org_code":"A001","effective_date":"2026-01-01","request_id":"r1","reason":"bad"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsRescindsAPI(rec, req, svc)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestHandleOrgUnitsRescindsOrgAPI_Success(t *testing.T) {
	called := false
	svc := orgUnitWriteServiceStub{
		rescindOrgFn: func(_ context.Context, tenantID string, req orgunitservices.RescindOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			called = true
			if tenantID != "t1" {
				t.Fatalf("tenant=%s", tenantID)
			}
			if req.OrgCode != "A001" || req.RequestID != "r2" || req.Reason != "bad-org" {
				t.Fatalf("unexpected request: %+v", req)
			}
			return orgunittypes.OrgUnitResult{OrgCode: "A001", Fields: map[string]any{"rescinded_events": int64(3)}}, nil
		},
	}
	body := strings.NewReader(`{"org_code":"A001","request_id":"r2","reason":"bad-org"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds/org", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleOrgUnitsRescindsOrgAPI(rec, req, svc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !called {
		t.Fatalf("expected rescind org call")
	}
	if !strings.Contains(rec.Body.String(), "\"rescinded_events\":3") {
		t.Fatalf("body=%q", rec.Body.String())
	}
}

func TestHandleOrgUnitsRescindsOrgAPI_RescindedEventsFieldTypes(t *testing.T) {
	cases := []struct {
		name  string
		field any
		want  string
	}{
		{name: "int", field: 3, want: `"rescinded_events":3`},
		{name: "int32", field: int32(4), want: `"rescinded_events":4`},
		{name: "int64", field: int64(5), want: `"rescinded_events":5`},
		{name: "float64", field: float64(6), want: `"rescinded_events":6`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := orgUnitWriteServiceStub{
				rescindOrgFn: func(context.Context, string, orgunitservices.RescindOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
					return orgunittypes.OrgUnitResult{OrgCode: "A001", Fields: map[string]any{"rescinded_events": tc.field}}, nil
				},
			}
			req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds/org", strings.NewReader(`{"org_code":"A001","request_id":"r2","reason":"bad-org"}`))
			req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
			rec := httptest.NewRecorder()
			handleOrgUnitsRescindsOrgAPI(rec, req, svc)
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("body=%q", rec.Body.String())
			}
		})
	}
}

func TestHandleOrgUnitsRescindsOrgAPI_BasicErrors(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds/org", strings.NewReader(`{"org_code":"A001"}`))
		rec := httptest.NewRecorder()
		handleOrgUnitsRescindsOrgAPI(rec, req, orgUnitWriteServiceStub{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/rescinds/org", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsRescindsOrgAPI(rec, req, orgUnitWriteServiceStub{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("bad json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds/org", strings.NewReader("{"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsRescindsOrgAPI(rec, req, orgUnitWriteServiceStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("service missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds/org", strings.NewReader(`{"org_code":"A001"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsRescindsOrgAPI(rec, req, nil)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("conflict", func(t *testing.T) {
		svc := orgUnitWriteServiceStub{rescindOrgFn: func(context.Context, string, orgunitservices.RescindOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			return orgunittypes.OrgUnitResult{}, errors.New("ORG_ROOT_DELETE_FORBIDDEN")
		}}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds/org", strings.NewReader(`{"org_code":"A001","request_id":"r2","reason":"bad-org"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsRescindsOrgAPI(rec, req, svc)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("has children conflict", func(t *testing.T) {
		svc := orgUnitWriteServiceStub{rescindOrgFn: func(context.Context, string, orgunitservices.RescindOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			return orgunittypes.OrgUnitResult{}, errors.New("ORG_HAS_CHILDREN_CANNOT_DELETE")
		}}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds/org", strings.NewReader(`{"org_code":"A001","request_id":"r2","reason":"bad-org"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsRescindsOrgAPI(rec, req, svc)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("has dependencies conflict", func(t *testing.T) {
		svc := orgUnitWriteServiceStub{rescindOrgFn: func(context.Context, string, orgunitservices.RescindOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			return orgunittypes.OrgUnitResult{}, errors.New("ORG_HAS_DEPENDENCIES_CANNOT_DELETE")
		}}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds/org", strings.NewReader(`{"org_code":"A001","request_id":"r2","reason":"bad-org"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsRescindsOrgAPI(rec, req, svc)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestWriteOrgUnitServiceError_StatusMapping(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{"not_found", errors.New("ORG_CODE_NOT_FOUND"), http.StatusNotFound},
		{"bad_request_id", errors.New("ORG_CODE_INVALID"), http.StatusBadRequest},
		{"conflict", errors.New("EVENT_DATE_CONFLICT"), http.StatusConflict},
		{"request_id_conflict", errors.New("ORG_REQUEST_ID_CONFLICT"), http.StatusConflict},
		{"request_id_conflict_pg_message", &pgconn.PgError{Message: "ORG_REQUEST_ID_CONFLICT"}, http.StatusConflict},
		{"status_correction_unsupported", errors.New("ORG_STATUS_CORRECTION_UNSUPPORTED_TARGET"), http.StatusConflict},
		{"orgunit_codes_write_forbidden_pg_message", &pgconn.PgError{Message: "ORGUNIT_CODES_WRITE_FORBIDDEN"}, http.StatusUnprocessableEntity},
		{"high_risk_reorder", errors.New("ORG_HIGH_RISK_REORDER_FORBIDDEN"), http.StatusConflict},
		{"root_delete_forbidden", errors.New("ORG_ROOT_DELETE_FORBIDDEN"), http.StatusConflict},
		{"bad_request_msg", newBadRequestError("name is required"), http.StatusBadRequest},
		{"stable_unknown", errors.New("SOME_DB_CODE"), http.StatusUnprocessableEntity},
		{"unknown", errors.New("boom"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/org/api/org-units", nil)
			rec := httptest.NewRecorder()
			writeOrgUnitServiceError(rec, req, tt.err, "orgunit_failed")
			if rec.Code != tt.status {
				t.Fatalf("status=%d", rec.Code)
			}
		})
	}
}

func TestOrgUnitAPIStatusForCode_Branches(t *testing.T) {
	tests := []struct {
		code   string
		status int
		ok     bool
	}{
		{code: orgUnitErrCodeInvalid, status: http.StatusBadRequest, ok: true},
		{code: orgUnitErrCodeNotFound, status: http.StatusNotFound, ok: true},
		{code: orgUnitErrRequestIDConflict, status: http.StatusConflict, ok: true},
		{code: orgUnitErrFieldPolicyMissing, status: http.StatusUnprocessableEntity, ok: true},
		{code: "UNKNOWN_CODE", status: 0, ok: false},
	}
	for _, tt := range tests {
		status, ok := orgUnitAPIStatusForCode(tt.code)
		if status != tt.status || ok != tt.ok {
			t.Fatalf("code=%s status=%d ok=%v", tt.code, status, ok)
		}
	}
}

func TestWriteOrgUnitServiceError_UsesStablePgMessageCode(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/corrections", nil)
	rec := httptest.NewRecorder()

	writeOrgUnitServiceError(rec, req, &pgconn.PgError{Message: "ORGUNIT_CODES_WRITE_FORBIDDEN", Code: "P0001"}, "orgunit_correct_failed")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if got := payload["code"]; got != "ORGUNIT_CODES_WRITE_FORBIDDEN" {
		t.Fatalf("code=%v", got)
	}
}

func TestWriteOrgUnitServiceError_BadRequestStableUnknownCodePreserved(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/corrections", nil)
	rec := httptest.NewRecorder()

	writeOrgUnitServiceError(rec, req, newBadRequestError("SOME_DB_CODE"), "orgunit_correct_failed")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if got := payload["code"]; got != "SOME_DB_CODE" {
		t.Fatalf("code=%v", got)
	}
	if got := payload["message"]; got != "Some DB code." {
		t.Fatalf("message=%v", got)
	}
}

func TestWriteOrgUnitServiceError_BlankCodeFallsBackToDefault(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/corrections", nil)
	rec := httptest.NewRecorder()

	writeOrgUnitServiceError(rec, req, errors.New("   "), "orgunit_correct_failed")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if got := payload["code"]; got != "orgunit_correct_failed" {
		t.Fatalf("code=%v", got)
	}
	if got := payload["message"]; got != "Orgunit correct failed." {
		t.Fatalf("message=%v", got)
	}
}
