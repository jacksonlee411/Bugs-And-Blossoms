package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type orgUnitStoreWithWriteCapabilities struct {
	OrgUnitStore

	resolveOrgIDFn      func(ctx context.Context, tenantID string, orgCode string) (int, error)
	listExtConfigsFn    func(ctx context.Context, tenantID string, asOf string) ([]orgUnitTenantFieldConfig, error)
	isTreeInitializedFn func(ctx context.Context, tenantID string) (bool, error)
	resolveFactsFn      func(ctx context.Context, tenantID string, orgID int, effectiveDate string) (orgUnitAppendFacts, error)
	resolveTargetFn     func(ctx context.Context, tenantID string, orgID int, effectiveDate string) (orgUnitMutationTargetEvent, error)
}

func (s orgUnitStoreWithWriteCapabilities) ResolveOrgID(ctx context.Context, tenantID string, orgCode string) (int, error) {
	if s.resolveOrgIDFn != nil {
		return s.resolveOrgIDFn(ctx, tenantID, orgCode)
	}
	return 0, orgunitpkg.ErrOrgCodeNotFound
}

func (s orgUnitStoreWithWriteCapabilities) ListEnabledTenantFieldConfigsAsOf(ctx context.Context, tenantID string, asOf string) ([]orgUnitTenantFieldConfig, error) {
	if s.listExtConfigsFn != nil {
		return s.listExtConfigsFn(ctx, tenantID, asOf)
	}
	return []orgUnitTenantFieldConfig{}, nil
}

func (s orgUnitStoreWithWriteCapabilities) IsOrgTreeInitialized(ctx context.Context, tenantID string) (bool, error) {
	if s.isTreeInitializedFn != nil {
		return s.isTreeInitializedFn(ctx, tenantID)
	}
	return true, nil
}

func (s orgUnitStoreWithWriteCapabilities) ResolveAppendFacts(ctx context.Context, tenantID string, orgID int, effectiveDate string) (orgUnitAppendFacts, error) {
	if s.resolveFactsFn != nil {
		return s.resolveFactsFn(ctx, tenantID, orgID, effectiveDate)
	}
	return orgUnitAppendFacts{TreeInitialized: true, TargetExistsAsOf: true}, nil
}

func (s orgUnitStoreWithWriteCapabilities) ResolveMutationTargetEvent(ctx context.Context, tenantID string, orgID int, effectiveDate string) (orgUnitMutationTargetEvent, error) {
	if s.resolveTargetFn != nil {
		return s.resolveTargetFn(ctx, tenantID, orgID, effectiveDate)
	}
	return orgUnitMutationTargetEvent{HasEffective: true, EffectiveEventType: "CREATE", HasRaw: true, RawEventType: "CREATE"}, nil
}

func TestHandleOrgUnitWriteCapabilitiesAPI_BasicValidation(t *testing.T) {
	base := orgUnitStoreWithWriteCapabilities{OrgUnitStore: newOrgUnitMemoryStore()}

	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write-capabilities?intent=create_org&org_code=A001&effective_date=2026-01-01", nil)
		rec := httptest.NewRecorder()
		handleOrgUnitWriteCapabilitiesAPI(rec, req, base)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/write-capabilities", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitWriteCapabilitiesAPI(rec, req, base)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("missing required query", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write-capabilities", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitWriteCapabilitiesAPI(rec, req, base)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("org_code invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write-capabilities?intent=create_org&org_code=bad%7f&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitWriteCapabilitiesAPI(rec, req, base)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("effective_date invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write-capabilities?intent=create_org&org_code=A001&effective_date=bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitWriteCapabilitiesAPI(rec, req, base)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("create_org allows empty org_code", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write-capabilities?intent=create_org&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitWriteCapabilitiesAPI(rec, req, base)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("store missing interface", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write-capabilities?intent=create_org&org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitWriteCapabilitiesAPI(rec, req, &resolveOrgCodeStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestHandleOrgUnitWriteCapabilitiesAPI_SuccessEnvelope(t *testing.T) {
	store := orgUnitStoreWithWriteCapabilities{
		OrgUnitStore: newOrgUnitMemoryStore(),
		listExtConfigsFn: func(context.Context, string, string) ([]orgUnitTenantFieldConfig, error) {
			return []orgUnitTenantFieldConfig{{FieldKey: "org_type"}}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write-capabilities?intent=create_org&org_code=A001&effective_date=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{RoleSlug: "tenant-admin"}))
	rec := httptest.NewRecorder()
	handleOrgUnitWriteCapabilitiesAPI(rec, req, store)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json err=%v", err)
	}
	if resp["intent"] != "create_org" {
		t.Fatalf("intent=%v", resp["intent"])
	}
	if resp["enabled"] != true {
		t.Fatalf("enabled=%v", resp["enabled"])
	}
}

func TestHandleOrgUnitWriteCapabilitiesAPI_CoversBranches(t *testing.T) {
	t.Run("correct missing org_code", func(t *testing.T) {
		store := orgUnitStoreWithWriteCapabilities{OrgUnitStore: newOrgUnitMemoryStore()}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write-capabilities?intent=correct&effective_date=2026-01-01&target_effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitWriteCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("correct requires target_effective_date", func(t *testing.T) {
		store := orgUnitStoreWithWriteCapabilities{OrgUnitStore: newOrgUnitMemoryStore()}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write-capabilities?intent=correct&org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitWriteCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("correct target_effective_date invalid", func(t *testing.T) {
		store := orgUnitStoreWithWriteCapabilities{OrgUnitStore: newOrgUnitMemoryStore()}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write-capabilities?intent=correct&org_code=A001&effective_date=2026-01-01&target_effective_date=bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitWriteCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("intent not supported", func(t *testing.T) {
		store := orgUnitStoreWithWriteCapabilities{OrgUnitStore: newOrgUnitMemoryStore()}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write-capabilities?intent=nope&org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitWriteCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("create_org org already exists sets deny reasons", func(t *testing.T) {
		store := orgUnitStoreWithWriteCapabilities{
			OrgUnitStore: newOrgUnitMemoryStore(),
			resolveOrgIDFn: func(context.Context, string, string) (int, error) {
				return 1, nil
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write-capabilities?intent=create_org&org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{RoleSlug: "tenant-admin"}))
		rec := httptest.NewRecorder()
		handleOrgUnitWriteCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Enabled     bool     `json:"enabled"`
			DenyReasons []string `json:"deny_reasons"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("json err=%v", err)
		}
		if resp.Enabled {
			t.Fatalf("expected disabled")
		}
		found := slices.Contains(resp.DenyReasons, "ORG_ALREADY_EXISTS")
		if !found {
			t.Fatalf("deny=%v", resp.DenyReasons)
		}
	})

	t.Run("ext configs error -> 500", func(t *testing.T) {
		store := orgUnitStoreWithWriteCapabilities{
			OrgUnitStore: newOrgUnitMemoryStore(),
			listExtConfigsFn: func(context.Context, string, string) ([]orgUnitTenantFieldConfig, error) {
				return nil, errors.New("boom")
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write-capabilities?intent=create_org&org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitWriteCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("tree init error -> 500", func(t *testing.T) {
		store := orgUnitStoreWithWriteCapabilities{
			OrgUnitStore: newOrgUnitMemoryStore(),
			isTreeInitializedFn: func(context.Context, string) (bool, error) {
				return false, errors.New("boom")
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write-capabilities?intent=create_org&org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitWriteCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("add_version resolve org unexpected error -> 500", func(t *testing.T) {
		store := orgUnitStoreWithWriteCapabilities{
			OrgUnitStore: newOrgUnitMemoryStore(),
			resolveOrgIDFn: func(context.Context, string, string) (int, error) {
				return 0, errors.New("boom")
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write-capabilities?intent=add_version&org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitWriteCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("add_version missing org_code", func(t *testing.T) {
		store := orgUnitStoreWithWriteCapabilities{OrgUnitStore: newOrgUnitMemoryStore()}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write-capabilities?intent=add_version&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitWriteCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("add_version happy path returns enabled envelope", func(t *testing.T) {
		store := orgUnitStoreWithWriteCapabilities{
			OrgUnitStore: newOrgUnitMemoryStore(),
			resolveOrgIDFn: func(context.Context, string, string) (int, error) {
				return 1, nil
			},
			resolveFactsFn: func(context.Context, string, int, string) (orgUnitAppendFacts, error) {
				return orgUnitAppendFacts{TreeInitialized: true, TargetExistsAsOf: true}, nil
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write-capabilities?intent=add_version&org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{RoleSlug: "tenant-admin"}))
		rec := httptest.NewRecorder()
		handleOrgUnitWriteCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Enabled          bool              `json:"enabled"`
			AllowedFields    []string          `json:"allowed_fields"`
			FieldPayloadKeys map[string]string `json:"field_payload_keys"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("json err=%v", err)
		}
		if !resp.Enabled || len(resp.AllowedFields) == 0 || len(resp.FieldPayloadKeys) == 0 {
			t.Fatalf("resp=%+v", resp)
		}
	})

	t.Run("add_version facts error -> 500", func(t *testing.T) {
		store := orgUnitStoreWithWriteCapabilities{
			OrgUnitStore: newOrgUnitMemoryStore(),
			resolveOrgIDFn: func(context.Context, string, string) (int, error) {
				return 1, nil
			},
			resolveFactsFn: func(context.Context, string, int, string) (orgUnitAppendFacts, error) {
				return orgUnitAppendFacts{}, errors.New("boom")
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write-capabilities?intent=add_version&org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitWriteCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("add_version org not found returns success envelope with deny reasons", func(t *testing.T) {
		store := orgUnitStoreWithWriteCapabilities{
			OrgUnitStore: newOrgUnitMemoryStore(),
			resolveOrgIDFn: func(context.Context, string, string) (int, error) {
				return 0, orgunitpkg.ErrOrgCodeNotFound
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write-capabilities?intent=add_version&org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitWriteCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		var resp struct {
			Enabled     bool     `json:"enabled"`
			DenyReasons []string `json:"deny_reasons"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("json err=%v", err)
		}
		if resp.Enabled {
			t.Fatalf("expected disabled")
		}
		found := slices.Contains(resp.DenyReasons, "ORG_NOT_FOUND_AS_OF")
		if !found {
			t.Fatalf("deny=%v", resp.DenyReasons)
		}
	})

	t.Run("correct target resolve error -> 500", func(t *testing.T) {
		store := orgUnitStoreWithWriteCapabilities{
			OrgUnitStore: newOrgUnitMemoryStore(),
			resolveOrgIDFn: func(context.Context, string, string) (int, error) {
				return 1, nil
			},
			resolveTargetFn: func(context.Context, string, int, string) (orgUnitMutationTargetEvent, error) {
				return orgUnitMutationTargetEvent{}, errors.New("boom")
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write-capabilities?intent=correct&org_code=A001&effective_date=2026-01-01&target_effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitWriteCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("create_org resolve org unexpected error -> 500", func(t *testing.T) {
		store := orgUnitStoreWithWriteCapabilities{
			OrgUnitStore: newOrgUnitMemoryStore(),
			resolveOrgIDFn: func(context.Context, string, string) (int, error) {
				return 0, errors.New("boom")
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write-capabilities?intent=create_org&org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitWriteCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("correct org not found keeps 200 envelope", func(t *testing.T) {
		store := orgUnitStoreWithWriteCapabilities{
			OrgUnitStore: newOrgUnitMemoryStore(),
			resolveOrgIDFn: func(context.Context, string, string) (int, error) {
				return 0, orgunitpkg.ErrOrgCodeNotFound
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write-capabilities?intent=correct&org_code=A001&effective_date=2026-01-01&target_effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitWriteCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("correct target not found maps to ORG_EVENT_NOT_FOUND", func(t *testing.T) {
		store := orgUnitStoreWithWriteCapabilities{
			OrgUnitStore: newOrgUnitMemoryStore(),
			resolveOrgIDFn: func(context.Context, string, string) (int, error) {
				return 1, nil
			},
			resolveTargetFn: func(context.Context, string, int, string) (orgUnitMutationTargetEvent, error) {
				return orgUnitMutationTargetEvent{HasEffective: false, HasRaw: false}, nil
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write-capabilities?intent=correct&org_code=A001&effective_date=2026-01-01&target_effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{RoleSlug: "tenant-admin"}))
		rec := httptest.NewRecorder()
		handleOrgUnitWriteCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Enabled     bool     `json:"enabled"`
			DenyReasons []string `json:"deny_reasons"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("json err=%v", err)
		}
		if resp.Enabled {
			t.Fatalf("expected disabled")
		}
		if len(resp.DenyReasons) == 0 || resp.DenyReasons[0] != "ORG_EVENT_NOT_FOUND" {
			t.Fatalf("deny=%v", resp.DenyReasons)
		}
	})

	t.Run("correct target rescinded maps to ORG_EVENT_RESCINDED", func(t *testing.T) {
		store := orgUnitStoreWithWriteCapabilities{
			OrgUnitStore: newOrgUnitMemoryStore(),
			resolveOrgIDFn: func(context.Context, string, string) (int, error) {
				return 1, nil
			},
			resolveTargetFn: func(context.Context, string, int, string) (orgUnitMutationTargetEvent, error) {
				return orgUnitMutationTargetEvent{HasEffective: false, HasRaw: true}, nil
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write-capabilities?intent=correct&org_code=A001&effective_date=2026-01-01&target_effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{RoleSlug: "tenant-admin"}))
		rec := httptest.NewRecorder()
		handleOrgUnitWriteCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Enabled     bool     `json:"enabled"`
			DenyReasons []string `json:"deny_reasons"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("json err=%v", err)
		}
		if resp.Enabled {
			t.Fatalf("expected disabled")
		}
		if len(resp.DenyReasons) == 0 || resp.DenyReasons[0] != "ORG_EVENT_RESCINDED" {
			t.Fatalf("deny=%v", resp.DenyReasons)
		}
	})

	t.Run("correct target has effective returns enabled envelope", func(t *testing.T) {
		store := orgUnitStoreWithWriteCapabilities{
			OrgUnitStore: newOrgUnitMemoryStore(),
			resolveOrgIDFn: func(context.Context, string, string) (int, error) {
				return 1, nil
			},
			resolveTargetFn: func(context.Context, string, int, string) (orgUnitMutationTargetEvent, error) {
				return orgUnitMutationTargetEvent{HasEffective: true, EffectiveEventType: "CREATE", HasRaw: true, RawEventType: "CREATE"}, nil
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write-capabilities?intent=correct&org_code=A001&effective_date=2026-01-01&target_effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{RoleSlug: "tenant-admin"}))
		rec := httptest.NewRecorder()
		handleOrgUnitWriteCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("json err=%v", err)
		}
		if !resp.Enabled {
			t.Fatalf("expected enabled")
		}
	})

	t.Run("correct org resolve unexpected error -> 500", func(t *testing.T) {
		store := orgUnitStoreWithWriteCapabilities{
			OrgUnitStore: newOrgUnitMemoryStore(),
			resolveOrgIDFn: func(context.Context, string, string) (int, error) {
				return 0, errors.New("boom")
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write-capabilities?intent=correct&org_code=A001&effective_date=2026-01-01&target_effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitWriteCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("resolve capabilities returns error -> 500", func(t *testing.T) {
		orig := resolveWriteCapabilitiesInAPI
		resolveWriteCapabilitiesInAPI = func(orgunitservices.OrgUnitWriteIntent, []string, orgunitservices.OrgUnitWriteCapabilitiesFacts) (orgunitservices.OrgUnitWriteCapabilitiesDecision, error) {
			return orgunitservices.OrgUnitWriteCapabilitiesDecision{}, errors.New("policy fail")
		}
		t.Cleanup(func() { resolveWriteCapabilitiesInAPI = orig })

		store := orgUnitStoreWithWriteCapabilities{
			OrgUnitStore: newOrgUnitMemoryStore(),
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write-capabilities?intent=create_org&org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitWriteCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("deny reasons envelope includes forbidden and filters ext keys", func(t *testing.T) {
		store := orgUnitStoreWithWriteCapabilities{
			OrgUnitStore: newOrgUnitMemoryStore(),
			listExtConfigsFn: func(context.Context, string, string) ([]orgUnitTenantFieldConfig, error) {
				return []orgUnitTenantFieldConfig{{FieldKey: "unknown"}, {FieldKey: "org_type"}}, nil
			},
			isTreeInitializedFn: func(context.Context, string) (bool, error) { return false, nil },
		}

		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/write-capabilities?intent=create_org&org_code=A001&effective_date=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitWriteCapabilitiesAPI(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		var resp struct {
			Enabled       bool     `json:"enabled"`
			DenyReasons   []string `json:"deny_reasons"`
			AllowedFields []string `json:"allowed_fields"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("json err=%v", err)
		}
		if resp.Enabled {
			t.Fatalf("expected disabled")
		}
		hasForbidden := false
		for _, code := range resp.DenyReasons {
			if code == "FORBIDDEN" {
				hasForbidden = true
			}
		}
		if !hasForbidden {
			t.Fatalf("deny=%v", resp.DenyReasons)
		}
		for _, field := range resp.AllowedFields {
			if field == "unknown" {
				t.Fatalf("allowed=%v", resp.AllowedFields)
			}
		}
	})
}
