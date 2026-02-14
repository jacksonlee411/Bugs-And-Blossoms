package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type mutationCapabilitiesStoreStub struct {
	*resolveOrgCodeStore

	resolveTargetFn func(ctx context.Context, tenantID string, orgID int, effectiveDate string) (orgUnitMutationTargetEvent, error)
	listEnabledFn   func(ctx context.Context, tenantID string, asOf string) ([]orgUnitTenantFieldConfig, error)
	evalRescindFn   func(ctx context.Context, tenantID string, orgID int) ([]string, error)
}

func (s mutationCapabilitiesStoreStub) ResolveMutationTargetEvent(ctx context.Context, tenantID string, orgID int, effectiveDate string) (orgUnitMutationTargetEvent, error) {
	if s.resolveTargetFn != nil {
		return s.resolveTargetFn(ctx, tenantID, orgID, effectiveDate)
	}
	return orgUnitMutationTargetEvent{}, nil
}

func (s mutationCapabilitiesStoreStub) ListEnabledTenantFieldConfigsAsOf(ctx context.Context, tenantID string, asOf string) ([]orgUnitTenantFieldConfig, error) {
	if s.listEnabledFn != nil {
		return s.listEnabledFn(ctx, tenantID, asOf)
	}
	return []orgUnitTenantFieldConfig{}, nil
}

func (s mutationCapabilitiesStoreStub) EvaluateRescindOrgDenyReasons(ctx context.Context, tenantID string, orgID int) ([]string, error) {
	if s.evalRescindFn != nil {
		return s.evalRescindFn(ctx, tenantID, orgID)
	}
	return []string{}, nil
}

func TestAllowedCoreFieldsForTargetEvent(t *testing.T) {
	tests := []struct {
		event string
		want  []string
	}{
		{event: "CREATE", want: []string{"effective_date", "is_business_unit", "manager_pernr", "name", "parent_org_code"}},
		{event: "RENAME", want: []string{"effective_date", "name"}},
		{event: "MOVE", want: []string{"effective_date", "parent_org_code"}},
		{event: "SET_BUSINESS_UNIT", want: []string{"effective_date", "is_business_unit"}},
		{event: "DISABLE", want: []string{"effective_date"}},
		{event: "ENABLE", want: []string{"effective_date"}},
		{event: "UNKNOWN", want: []string{"effective_date"}},
	}
	for _, tt := range tests {
		got := allowedCoreFieldsForTargetEvent(tt.event)
		if strings.Join(got, ",") != strings.Join(tt.want, ",") {
			t.Fatalf("event=%s got=%v want=%v", tt.event, got, tt.want)
		}
	}
}

func TestHandleOrgUnitMutationCapabilitiesAPI_ErrorBranches(t *testing.T) {
	base := &resolveOrgCodeStore{}

	t.Run("method not allowed", func(t *testing.T) {
		store := mutationCapabilitiesStoreStub{resolveOrgCodeStore: base}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/mutation-capabilities", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitMutationCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/mutation-capabilities", nil)
		rec := httptest.NewRecorder()
		handleOrgUnitMutationCapabilitiesAPI(rec, req, mutationCapabilitiesStoreStub{resolveOrgCodeStore: base})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("store missing interface", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/mutation-capabilities", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitMutationCapabilitiesAPI(rec, req, newOrgUnitMemoryStore())
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("missing params", func(t *testing.T) {
		store := mutationCapabilitiesStoreStub{resolveOrgCodeStore: base}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/mutation-capabilities?org_code=&effective_date=", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitMutationCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("org_code invalid", func(t *testing.T) {
		store := mutationCapabilitiesStoreStub{resolveOrgCodeStore: base}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/mutation-capabilities?org_code=bad%7F&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitMutationCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("effective_date invalid", func(t *testing.T) {
		store := mutationCapabilitiesStoreStub{resolveOrgCodeStore: base}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/mutation-capabilities?org_code=A001&effective_date=bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitMutationCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("resolve org id invalid", func(t *testing.T) {
		store := mutationCapabilitiesStoreStub{resolveOrgCodeStore: &resolveOrgCodeStore{resolveErr: orgunitpkg.ErrOrgCodeInvalid}}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/mutation-capabilities?org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitMutationCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("resolve org id not found", func(t *testing.T) {
		store := mutationCapabilitiesStoreStub{resolveOrgCodeStore: &resolveOrgCodeStore{resolveErr: orgunitpkg.ErrOrgCodeNotFound}}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/mutation-capabilities?org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitMutationCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("resolve org id internal error", func(t *testing.T) {
		store := mutationCapabilitiesStoreStub{resolveOrgCodeStore: &resolveOrgCodeStore{resolveErr: errors.New("boom")}}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/mutation-capabilities?org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitMutationCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("resolve target error", func(t *testing.T) {
		store := mutationCapabilitiesStoreStub{
			resolveOrgCodeStore: &resolveOrgCodeStore{resolveID: 10000001},
			resolveTargetFn: func(context.Context, string, int, string) (orgUnitMutationTargetEvent, error) {
				return orgUnitMutationTargetEvent{}, errors.New("boom")
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/mutation-capabilities?org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitMutationCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("target rescinded", func(t *testing.T) {
		store := mutationCapabilitiesStoreStub{
			resolveOrgCodeStore: &resolveOrgCodeStore{resolveID: 10000001},
			resolveTargetFn: func(context.Context, string, int, string) (orgUnitMutationTargetEvent, error) {
				return orgUnitMutationTargetEvent{HasEffective: false, HasRaw: true}, nil
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/mutation-capabilities?org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitMutationCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("target not found", func(t *testing.T) {
		store := mutationCapabilitiesStoreStub{
			resolveOrgCodeStore: &resolveOrgCodeStore{resolveID: 10000001},
			resolveTargetFn: func(context.Context, string, int, string) (orgUnitMutationTargetEvent, error) {
				return orgUnitMutationTargetEvent{HasEffective: false, HasRaw: false}, nil
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/mutation-capabilities?org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitMutationCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("list enabled configs error", func(t *testing.T) {
		store := mutationCapabilitiesStoreStub{
			resolveOrgCodeStore: &resolveOrgCodeStore{resolveID: 10000001},
			resolveTargetFn: func(context.Context, string, int, string) (orgUnitMutationTargetEvent, error) {
				return orgUnitMutationTargetEvent{HasEffective: true, EffectiveEventType: "CREATE", HasRaw: true, RawEventType: "CREATE"}, nil
			},
			listEnabledFn: func(context.Context, string, string) ([]orgUnitTenantFieldConfig, error) {
				return nil, errors.New("boom")
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/mutation-capabilities?org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitMutationCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("evaluate rescind org error", func(t *testing.T) {
		store := mutationCapabilitiesStoreStub{
			resolveOrgCodeStore: &resolveOrgCodeStore{resolveID: 10000001},
			resolveTargetFn: func(context.Context, string, int, string) (orgUnitMutationTargetEvent, error) {
				return orgUnitMutationTargetEvent{HasEffective: true, EffectiveEventType: "CREATE", HasRaw: true, RawEventType: "CREATE"}, nil
			},
			listEnabledFn: func(context.Context, string, string) ([]orgUnitTenantFieldConfig, error) {
				return []orgUnitTenantFieldConfig{}, nil
			},
			evalRescindFn: func(context.Context, string, int) ([]string, error) {
				return nil, errors.New("boom")
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/mutation-capabilities?org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitMutationCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestHandleOrgUnitMutationCapabilitiesAPI_Success(t *testing.T) {
	t.Run("viewer (forbidden) create target includes ext fields", func(t *testing.T) {
		store := mutationCapabilitiesStoreStub{
			resolveOrgCodeStore: &resolveOrgCodeStore{resolveID: 10000001},
			resolveTargetFn: func(context.Context, string, int, string) (orgUnitMutationTargetEvent, error) {
				return orgUnitMutationTargetEvent{HasEffective: true, EffectiveEventType: "CREATE", HasRaw: true, RawEventType: "CREATE"}, nil
			},
			listEnabledFn: func(context.Context, string, string) ([]orgUnitTenantFieldConfig, error) {
				return []orgUnitTenantFieldConfig{
					{FieldKey: " "},
					{FieldKey: "org_type"},
				}, nil
			},
			evalRescindFn: func(context.Context, string, int) ([]string, error) {
				return []string{orgUnitErrRootDeleteForbidden}, nil
			},
		}

		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/mutation-capabilities?org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1", RoleSlug: "tenant-viewer"}))
		rec := httptest.NewRecorder()
		handleOrgUnitMutationCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		var body orgUnitMutationCapabilitiesAPIResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if body.Capabilities.CorrectEvent.Enabled {
			t.Fatalf("expected correct_event disabled")
		}
		if len(body.Capabilities.CorrectEvent.AllowedFields) == 0 {
			t.Fatalf("expected allowed fields")
		}
		if body.Capabilities.CorrectEvent.FieldPayloadKeys["org_type"] != "ext.org_type" {
			t.Fatalf("ext mapping=%q", body.Capabilities.CorrectEvent.FieldPayloadKeys["org_type"])
		}
		if len(body.Capabilities.CorrectEvent.DenyReasons) == 0 || body.Capabilities.CorrectEvent.DenyReasons[0] != "FORBIDDEN" {
			t.Fatalf("deny=%v", body.Capabilities.CorrectEvent.DenyReasons)
		}
		if body.Capabilities.CorrectStatus.Enabled {
			t.Fatalf("expected correct_status disabled")
		}
		if len(body.Capabilities.CorrectStatus.DenyReasons) == 0 {
			t.Fatalf("expected correct_status deny reasons")
		}
		if body.Capabilities.RescindOrg.Enabled {
			t.Fatalf("expected rescind_org disabled")
		}
		if len(body.Capabilities.RescindOrg.DenyReasons) == 0 {
			t.Fatalf("expected rescind_org deny reasons")
		}
	})

	t.Run("admin enable target supports correct_status and raw falls back", func(t *testing.T) {
		store := mutationCapabilitiesStoreStub{
			resolveOrgCodeStore: &resolveOrgCodeStore{resolveID: 10000001},
			resolveTargetFn: func(context.Context, string, int, string) (orgUnitMutationTargetEvent, error) {
				return orgUnitMutationTargetEvent{HasEffective: true, EffectiveEventType: "ENABLE", HasRaw: false, RawEventType: ""}, nil
			},
			listEnabledFn: func(context.Context, string, string) ([]orgUnitTenantFieldConfig, error) {
				return []orgUnitTenantFieldConfig{}, nil
			},
			evalRescindFn: func(context.Context, string, int) ([]string, error) {
				return []string{}, nil
			},
		}

		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/mutation-capabilities?org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1", RoleSlug: "tenant-admin"}))
		rec := httptest.NewRecorder()
		handleOrgUnitMutationCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var body orgUnitMutationCapabilitiesAPIResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if body.RawTargetEventType != "ENABLE" {
			t.Fatalf("raw_target=%q", body.RawTargetEventType)
		}
		if !body.Capabilities.CorrectStatus.Enabled || len(body.Capabilities.CorrectStatus.AllowedTargetStatuses) != 2 {
			t.Fatalf("correct_status=%#v", body.Capabilities.CorrectStatus)
		}
		if !body.Capabilities.RescindOrg.Enabled {
			t.Fatalf("expected rescind_org enabled")
		}
	})
}
