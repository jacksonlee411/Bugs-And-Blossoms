package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	orgunittypes "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
)

type fakeOrgUnitWriteService struct {
	writeFn func(ctx context.Context, tenantID string, req orgunitservices.WriteOrgUnitRequest) (orgunitservices.OrgUnitWriteResult, error)
}

func (s fakeOrgUnitWriteService) Write(ctx context.Context, tenantID string, req orgunitservices.WriteOrgUnitRequest) (orgunitservices.OrgUnitWriteResult, error) {
	if s.writeFn != nil {
		return s.writeFn(ctx, tenantID, req)
	}
	return orgunitservices.OrgUnitWriteResult{}, nil
}

func (s fakeOrgUnitWriteService) Create(context.Context, string, orgunitservices.CreateOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, nil
}
func (s fakeOrgUnitWriteService) Rename(context.Context, string, orgunitservices.RenameOrgUnitRequest) error {
	return nil
}
func (s fakeOrgUnitWriteService) Move(context.Context, string, orgunitservices.MoveOrgUnitRequest) error {
	return nil
}
func (s fakeOrgUnitWriteService) Disable(context.Context, string, orgunitservices.DisableOrgUnitRequest) error {
	return nil
}
func (s fakeOrgUnitWriteService) Enable(context.Context, string, orgunitservices.EnableOrgUnitRequest) error {
	return nil
}
func (s fakeOrgUnitWriteService) SetBusinessUnit(context.Context, string, orgunitservices.SetBusinessUnitRequest) error {
	return nil
}
func (s fakeOrgUnitWriteService) Correct(context.Context, string, orgunitservices.CorrectOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, nil
}
func (s fakeOrgUnitWriteService) CorrectStatus(context.Context, string, orgunitservices.CorrectStatusOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, nil
}
func (s fakeOrgUnitWriteService) RescindRecord(context.Context, string, orgunitservices.RescindRecordOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, nil
}
func (s fakeOrgUnitWriteService) RescindOrg(context.Context, string, orgunitservices.RescindOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	return orgunittypes.OrgUnitResult{}, nil
}

func TestHandleOrgUnitsWriteAPI_BasicValidation(t *testing.T) {
	resetPolicyActivationRuntimeForTest()
	svc := fakeOrgUnitWriteService{}
	createPolicyVersion, _ := resolveOrgUnitEffectivePolicyVersion("t1", orgUnitCreateFieldPolicyCapabilityKey)

	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/write", bytes.NewBufferString(`{}`))
		rec := httptest.NewRecorder()
		handleOrgUnitsWriteAPI(rec, req, svc)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsWriteAPI(rec, req, svc)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("bad json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/write", bytes.NewBufferString("{"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsWriteAPI(rec, req, svc)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("create missing policy_version", func(t *testing.T) {
		body := `{"intent":"create_org","org_code":"ROOT","effective_date":"2026-01-01","request_id":"r1","patch":{"name":"X"}}`
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/write", bytes.NewBufferString(body))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsWriteAPI(rec, req, svc)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), orgUnitErrFieldPolicyVersionRequired) {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("create stale policy_version", func(t *testing.T) {
		body := `{"intent":"create_org","org_code":"ROOT","effective_date":"2026-01-01","policy_version":"2025-01-01","request_id":"r1","patch":{"name":"X"}}`
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/write", bytes.NewBufferString(body))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsWriteAPI(rec, req, svc)
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), orgUnitErrFieldPolicyVersionStale) {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("outdated intent policy_version rejected in compatibility window when baseline exists", func(t *testing.T) {
		originalNow := nowUTCForOrgUnitPolicyVersion
		nowUTCForOrgUnitPolicyVersion = func() time.Time {
			return time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
		}
		t.Cleanup(func() { nowUTCForOrgUnitPolicyVersion = originalNow })

		body := `{"intent":"create_org","org_code":"ROOT","effective_date":"2026-01-01","policy_version":"2026-02-23","request_id":"r1","patch":{"name":"X"}}`
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/write", bytes.NewBufferString(body))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsWriteAPI(rec, req, svc)
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), orgUnitErrFieldPolicyVersionStale) {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("effective policy_version accepted in compatibility window", func(t *testing.T) {
		originalNow := nowUTCForOrgUnitPolicyVersion
		nowUTCForOrgUnitPolicyVersion = func() time.Time {
			return time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
		}
		t.Cleanup(func() { nowUTCForOrgUnitPolicyVersion = originalNow })

		body := fmt.Sprintf(`{"intent":"create_org","org_code":"ROOT","effective_date":"2026-01-01","policy_version":"%s","request_id":"r1","patch":{"name":"X"}}`, createPolicyVersion)
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/write", bytes.NewBufferString(body))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsWriteAPI(rec, req, svc)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("add_version missing policy_version", func(t *testing.T) {
		body := `{"intent":"add_version","org_code":"A001","effective_date":"2026-01-01","request_id":"r1","patch":{"name":"X"}}`
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/write", bytes.NewBufferString(body))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsWriteAPI(rec, req, svc)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), orgUnitErrFieldPolicyVersionRequired) {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("correct stale policy_version", func(t *testing.T) {
		body := `{"intent":"correct","org_code":"A001","effective_date":"2026-01-02","target_effective_date":"2026-01-01","policy_version":"2025-01-01","request_id":"r1","patch":{"name":"X"}}`
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/write", bytes.NewBufferString(body))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsWriteAPI(rec, req, svc)
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), orgUnitErrFieldPolicyVersionStale) {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("forbid ext_labels_snapshot in request", func(t *testing.T) {
		body := fmt.Sprintf(`{"intent":"create_org","org_code":"ROOT","effective_date":"2026-01-01","policy_version":"%s","request_id":"r1","patch":{"name":"X","ext_labels_snapshot":{"x":"y"}}}`, createPolicyVersion)
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/write", bytes.NewBufferString(body))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsWriteAPI(rec, req, svc)
		if rec.Code == http.StatusOK {
			t.Fatalf("expected error, got ok: %s", rec.Body.String())
		}
	})

	t.Run("unknown field rejected by DisallowUnknownFields", func(t *testing.T) {
		body := fmt.Sprintf(`{"intent":"create_org","org_code":"ROOT","effective_date":"2026-01-01","policy_version":"%s","request_id":"r1","patch":{"name":"X"},"x":1}`, createPolicyVersion)
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/write", bytes.NewBufferString(body))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsWriteAPI(rec, req, svc)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestHandleOrgUnitsWriteAPI_ResultAndErrorMapping(t *testing.T) {
	resetPolicyActivationRuntimeForTest()
	createPolicyVersion, _ := resolveOrgUnitEffectivePolicyVersion("t1", orgUnitCreateFieldPolicyCapabilityKey)
	body := fmt.Sprintf(`{"intent":"create_org","org_code":"ROOT","effective_date":"2026-01-01","policy_version":"%s","request_id":"r1","patch":{"name":"Root A"}}`, createPolicyVersion)

	t.Run("service nil", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/write", bytes.NewBufferString(body))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsWriteAPI(rec, req, nil)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("success envelope", func(t *testing.T) {
		svc := fakeOrgUnitWriteService{
			writeFn: func(_ context.Context, tenantID string, req orgunitservices.WriteOrgUnitRequest) (orgunitservices.OrgUnitWriteResult, error) {
				if tenantID != "t1" || req.Intent != "create_org" || req.OrgCode != "ROOT" || req.PolicyVersion != createPolicyVersion || req.RequestID != "r1" {
					t.Fatalf("req=%+v tenant=%s", req, tenantID)
				}
				return orgunitservices.OrgUnitWriteResult{
					OrgCode:       "ROOT",
					EffectiveDate: "2026-01-01",
					EventType:     "CREATE",
					EventUUID:     "evt-1",
					Fields:        map[string]any{"name": "Root A"},
				}, nil
			},
		}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/write", bytes.NewBufferString(body))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsWriteAPI(rec, req, svc)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("json err=%v", err)
		}
		if resp["event_type"] != "CREATE" || resp["event_uuid"] != "evt-1" {
			t.Fatalf("resp=%v", resp)
		}
	})

	t.Run("bad json from service is mapped", func(t *testing.T) {
		svc := fakeOrgUnitWriteService{
			writeFn: func(context.Context, string, orgunitservices.WriteOrgUnitRequest) (orgunitservices.OrgUnitWriteResult, error) {
				return orgunitservices.OrgUnitWriteResult{}, errOrgUnitBadJSON
			},
		}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/write", bytes.NewBufferString(body))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsWriteAPI(rec, req, svc)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("stable code maps to 400", func(t *testing.T) {
		svc := fakeOrgUnitWriteService{
			writeFn: func(context.Context, string, orgunitservices.WriteOrgUnitRequest) (orgunitservices.OrgUnitWriteResult, error) {
				return orgunitservices.OrgUnitWriteResult{}, errors.New("PATCH_FIELD_NOT_ALLOWED")
			},
		}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/write", bytes.NewBufferString(body))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitsWriteAPI(rec, req, svc)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}
