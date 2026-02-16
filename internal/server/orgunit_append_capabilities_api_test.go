package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type orgUnitStoreWithAppendCapabilities struct {
	OrgUnitStore

	resolveOrgIDFn      func(ctx context.Context, tenantID string, orgCode string) (int, error)
	listExtConfigsFn    func(ctx context.Context, tenantID string, asOf string) ([]orgUnitTenantFieldConfig, error)
	isTreeInitializedFn func(ctx context.Context, tenantID string) (bool, error)
	resolveFactsFn      func(ctx context.Context, tenantID string, orgID int, effectiveDate string) (orgUnitAppendFacts, error)
}

func (s orgUnitStoreWithAppendCapabilities) ResolveOrgID(ctx context.Context, tenantID string, orgCode string) (int, error) {
	if s.resolveOrgIDFn != nil {
		return s.resolveOrgIDFn(ctx, tenantID, orgCode)
	}
	return 0, orgunitpkg.ErrOrgCodeNotFound
}

func (s orgUnitStoreWithAppendCapabilities) ListEnabledTenantFieldConfigsAsOf(ctx context.Context, tenantID string, asOf string) ([]orgUnitTenantFieldConfig, error) {
	if s.listExtConfigsFn != nil {
		return s.listExtConfigsFn(ctx, tenantID, asOf)
	}
	return []orgUnitTenantFieldConfig{}, nil
}

func (s orgUnitStoreWithAppendCapabilities) IsOrgTreeInitialized(ctx context.Context, tenantID string) (bool, error) {
	if s.isTreeInitializedFn != nil {
		return s.isTreeInitializedFn(ctx, tenantID)
	}
	return true, nil
}

func (s orgUnitStoreWithAppendCapabilities) ResolveAppendFacts(ctx context.Context, tenantID string, orgID int, effectiveDate string) (orgUnitAppendFacts, error) {
	if s.resolveFactsFn != nil {
		return s.resolveFactsFn(ctx, tenantID, orgID, effectiveDate)
	}
	return orgUnitAppendFacts{TreeInitialized: true, TargetExistsAsOf: true}, nil
}

func TestHandleOrgUnitAppendCapabilitiesAPI_BasicValidation(t *testing.T) {
	base := orgUnitStoreWithAppendCapabilities{OrgUnitStore: newOrgUnitMemoryStore()}

	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/append-capabilities?org_code=A001&effective_date=2026-01-01", nil)
		rec := httptest.NewRecorder()
		handleOrgUnitAppendCapabilitiesAPI(rec, req, base)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/append-capabilities", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitAppendCapabilitiesAPI(rec, req, base)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("missing required query", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/append-capabilities", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitAppendCapabilitiesAPI(rec, req, base)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("org_code invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/append-capabilities?org_code=bad%7f&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitAppendCapabilitiesAPI(rec, req, base)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("effective_date invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/append-capabilities?org_code=A001&effective_date=bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitAppendCapabilitiesAPI(rec, req, base)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("store missing append capability interface", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/append-capabilities?org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitAppendCapabilitiesAPI(rec, req, &resolveOrgCodeStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestHandleOrgUnitAppendCapabilitiesAPI_SuccessResponses(t *testing.T) {
	t.Run("org not found as-of returns explainable disabled update actions", func(t *testing.T) {
		store := orgUnitStoreWithAppendCapabilities{
			OrgUnitStore: newOrgUnitMemoryStore(),
			resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
				return 0, orgunitpkg.ErrOrgCodeNotFound
			},
			listExtConfigsFn: func(_ context.Context, _ string, _ string) ([]orgUnitTenantFieldConfig, error) {
				return []orgUnitTenantFieldConfig{{FieldKey: "org_type"}}, nil
			},
			isTreeInitializedFn: func(_ context.Context, _ string) (bool, error) {
				return true, nil
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/append-capabilities?org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{RoleSlug: "tenant-admin"}))
		rec := httptest.NewRecorder()
		handleOrgUnitAppendCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		var body orgUnitAppendCapabilitiesAPIResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode err=%v", err)
		}
		if body.Capabilities.Create.Enabled != true {
			t.Fatalf("expected create enabled")
		}
		if got := strings.Join(body.Capabilities.EventUpdate["RENAME"].DenyReasons, ","); got != "ORG_NOT_FOUND_AS_OF" {
			t.Fatalf("unexpected deny reasons=%s", got)
		}
		if len(body.Capabilities.EventUpdate["RENAME"].AllowedFields) != 0 {
			t.Fatalf("expected fail-closed fields")
		}
	})

	t.Run("root move denied", func(t *testing.T) {
		store := orgUnitStoreWithAppendCapabilities{
			OrgUnitStore: newOrgUnitMemoryStore(),
			resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
				return 10000001, nil
			},
			listExtConfigsFn: func(_ context.Context, _ string, _ string) ([]orgUnitTenantFieldConfig, error) {
				return []orgUnitTenantFieldConfig{}, nil
			},
			isTreeInitializedFn: func(_ context.Context, _ string) (bool, error) {
				return true, nil
			},
			resolveFactsFn: func(_ context.Context, _ string, _ int, _ string) (orgUnitAppendFacts, error) {
				return orgUnitAppendFacts{
					TreeInitialized:  true,
					TargetExistsAsOf: true,
					IsRoot:           true,
				}, nil
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/append-capabilities?org_code=ROOT&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{RoleSlug: "tenant-admin"}))
		rec := httptest.NewRecorder()
		handleOrgUnitAppendCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		var body orgUnitAppendCapabilitiesAPIResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode err=%v", err)
		}
		if got := strings.Join(body.Capabilities.EventUpdate["MOVE"].DenyReasons, ","); got != "ORG_ROOT_CANNOT_BE_MOVED" {
			t.Fatalf("unexpected deny reasons=%s", got)
		}
	})

	t.Run("forbidden returns explainable disabled capabilities", func(t *testing.T) {
		store := orgUnitStoreWithAppendCapabilities{
			OrgUnitStore: newOrgUnitMemoryStore(),
			resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
				return 10000001, nil
			},
			isTreeInitializedFn: func(_ context.Context, _ string) (bool, error) {
				return true, nil
			},
			resolveFactsFn: func(_ context.Context, _ string, _ int, _ string) (orgUnitAppendFacts, error) {
				return orgUnitAppendFacts{TreeInitialized: true, TargetExistsAsOf: true}, nil
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/append-capabilities?org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitAppendCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var body orgUnitAppendCapabilitiesAPIResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode err=%v", err)
		}
		if body.Capabilities.Create.Enabled {
			t.Fatalf("expected create disabled")
		}
		if got := strings.Join(body.Capabilities.Create.DenyReasons, ","); got != "FORBIDDEN,ORG_ALREADY_EXISTS" {
			t.Fatalf("unexpected deny reasons=%s", got)
		}
		if len(body.Capabilities.Create.AllowedFields) != 0 || len(body.Capabilities.Create.FieldPayloadKeys) != 0 {
			t.Fatalf("expected fail-closed create capability")
		}
	})

	t.Run("blank ext field keys are ignored", func(t *testing.T) {
		store := orgUnitStoreWithAppendCapabilities{
			OrgUnitStore: newOrgUnitMemoryStore(),
			listExtConfigsFn: func(_ context.Context, _ string, _ string) ([]orgUnitTenantFieldConfig, error) {
				return []orgUnitTenantFieldConfig{
					{FieldKey: " "},
					{FieldKey: "org_type"},
				}, nil
			},
			isTreeInitializedFn: func(_ context.Context, _ string) (bool, error) { return true, nil },
			resolveOrgIDFn:      func(_ context.Context, _ string, _ string) (int, error) { return 0, orgunitpkg.ErrOrgCodeNotFound },
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/append-capabilities?org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{RoleSlug: "tenant-admin"}))
		rec := httptest.NewRecorder()
		handleOrgUnitAppendCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var body orgUnitAppendCapabilitiesAPIResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode err=%v", err)
		}
		if got := strings.Join(body.Capabilities.Create.AllowedFields, ","); !strings.Contains(got, "org_type") || strings.Contains(got, ",,") {
			t.Fatalf("allowed=%v", body.Capabilities.Create.AllowedFields)
		}
	})
}

func TestHandleOrgUnitAppendCapabilitiesAPI_InternalErrorBranches(t *testing.T) {
	t.Run("tree init resolve error", func(t *testing.T) {
		store := orgUnitStoreWithAppendCapabilities{
			OrgUnitStore: newOrgUnitMemoryStore(),
			isTreeInitializedFn: func(_ context.Context, _ string) (bool, error) {
				return false, errors.New("boom")
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/append-capabilities?org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{RoleSlug: "tenant-admin"}))
		rec := httptest.NewRecorder()
		handleOrgUnitAppendCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("resolve org invalid", func(t *testing.T) {
		store := orgUnitStoreWithAppendCapabilities{
			OrgUnitStore:        newOrgUnitMemoryStore(),
			isTreeInitializedFn: func(_ context.Context, _ string) (bool, error) { return true, nil },
			resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
				return 0, orgunitpkg.ErrOrgCodeInvalid
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/append-capabilities?org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitAppendCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("resolve org unexpected error", func(t *testing.T) {
		store := orgUnitStoreWithAppendCapabilities{
			OrgUnitStore:        newOrgUnitMemoryStore(),
			isTreeInitializedFn: func(_ context.Context, _ string) (bool, error) { return true, nil },
			resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
				return 0, errors.New("boom")
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/append-capabilities?org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitAppendCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("list ext configs error", func(t *testing.T) {
		store := orgUnitStoreWithAppendCapabilities{
			OrgUnitStore:        newOrgUnitMemoryStore(),
			isTreeInitializedFn: func(_ context.Context, _ string) (bool, error) { return true, nil },
			resolveOrgIDFn:      func(_ context.Context, _ string, _ string) (int, error) { return 10000001, nil },
			listExtConfigsFn: func(_ context.Context, _ string, _ string) ([]orgUnitTenantFieldConfig, error) {
				return nil, errors.New("boom")
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/append-capabilities?org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitAppendCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("resolve append facts error", func(t *testing.T) {
		store := orgUnitStoreWithAppendCapabilities{
			OrgUnitStore:        newOrgUnitMemoryStore(),
			isTreeInitializedFn: func(_ context.Context, _ string) (bool, error) { return true, nil },
			resolveOrgIDFn:      func(_ context.Context, _ string, _ string) (int, error) { return 10000001, nil },
			resolveFactsFn: func(_ context.Context, _ string, _ int, _ string) (orgUnitAppendFacts, error) {
				return orgUnitAppendFacts{}, errors.New("boom")
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/append-capabilities?org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{RoleSlug: "tenant-admin"}))
		rec := httptest.NewRecorder()
		handleOrgUnitAppendCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("create policy resolve error", func(t *testing.T) {
		orig := resolveOrgUnitMutationPolicyForAppend
		resolveOrgUnitMutationPolicyForAppend = func(key orgunitservices.OrgUnitMutationPolicyKey, facts orgunitservices.OrgUnitMutationPolicyFacts) (orgunitservices.OrgUnitMutationPolicyDecision, error) {
			if key.ActionKind == orgunitservices.OrgUnitActionCreate {
				return orgunitservices.OrgUnitMutationPolicyDecision{}, errors.New("boom")
			}
			return orig(key, facts)
		}
		t.Cleanup(func() { resolveOrgUnitMutationPolicyForAppend = orig })

		store := orgUnitStoreWithAppendCapabilities{
			OrgUnitStore:        newOrgUnitMemoryStore(),
			isTreeInitializedFn: func(_ context.Context, _ string) (bool, error) { return true, nil },
			resolveOrgIDFn:      func(_ context.Context, _ string, _ string) (int, error) { return 0, orgunitpkg.ErrOrgCodeNotFound },
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/append-capabilities?org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitAppendCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("event policy resolve error", func(t *testing.T) {
		orig := resolveOrgUnitMutationPolicyForAppend
		resolveOrgUnitMutationPolicyForAppend = func(key orgunitservices.OrgUnitMutationPolicyKey, facts orgunitservices.OrgUnitMutationPolicyFacts) (orgunitservices.OrgUnitMutationPolicyDecision, error) {
			if key.ActionKind == orgunitservices.OrgUnitActionEventUpdate {
				return orgunitservices.OrgUnitMutationPolicyDecision{}, errors.New("boom")
			}
			return orig(key, facts)
		}
		t.Cleanup(func() { resolveOrgUnitMutationPolicyForAppend = orig })

		store := orgUnitStoreWithAppendCapabilities{
			OrgUnitStore:        newOrgUnitMemoryStore(),
			isTreeInitializedFn: func(_ context.Context, _ string) (bool, error) { return true, nil },
			resolveOrgIDFn:      func(_ context.Context, _ string, _ string) (int, error) { return 0, orgunitpkg.ErrOrgCodeNotFound },
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/append-capabilities?org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitAppendCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}
