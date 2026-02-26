package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/cel-go/cel"
)

type orgUnitStoreWithFieldPolicies struct {
	OrgUnitStore
	listFn    func(ctx context.Context, tenantID string) ([]orgUnitTenantFieldPolicy, error)
	resolveFn func(ctx context.Context, tenantID string, fieldKey string, scopeType string, scopeKey string, asOf string) (orgUnitTenantFieldPolicy, bool, error)
	upsertFn  func(ctx context.Context, tenantID string, fieldKey string, scopeType string, scopeKey string, maintainable bool, defaultMode string, defaultRuleExpr *string, enabledOn string, requestID string, initiatorUUID string) (orgUnitTenantFieldPolicy, bool, error)
	disableFn func(ctx context.Context, tenantID string, fieldKey string, scopeType string, scopeKey string, disabledOn string, requestID string, initiatorUUID string) (orgUnitTenantFieldPolicy, bool, error)
}

func (s orgUnitStoreWithFieldPolicies) ListTenantFieldPolicies(ctx context.Context, tenantID string) ([]orgUnitTenantFieldPolicy, error) {
	if s.listFn != nil {
		return s.listFn(ctx, tenantID)
	}
	return []orgUnitTenantFieldPolicy{}, nil
}

func (s orgUnitStoreWithFieldPolicies) ResolveTenantFieldPolicy(ctx context.Context, tenantID string, fieldKey string, scopeType string, scopeKey string, asOf string) (orgUnitTenantFieldPolicy, bool, error) {
	if s.resolveFn != nil {
		return s.resolveFn(ctx, tenantID, fieldKey, scopeType, scopeKey, asOf)
	}
	return orgUnitTenantFieldPolicy{}, false, nil
}

func (s orgUnitStoreWithFieldPolicies) UpsertTenantFieldPolicy(ctx context.Context, tenantID string, fieldKey string, scopeType string, scopeKey string, maintainable bool, defaultMode string, defaultRuleExpr *string, enabledOn string, requestID string, initiatorUUID string) (orgUnitTenantFieldPolicy, bool, error) {
	if s.upsertFn != nil {
		return s.upsertFn(ctx, tenantID, fieldKey, scopeType, scopeKey, maintainable, defaultMode, defaultRuleExpr, enabledOn, requestID, initiatorUUID)
	}
	return orgUnitTenantFieldPolicy{}, false, nil
}

func (s orgUnitStoreWithFieldPolicies) DisableTenantFieldPolicy(ctx context.Context, tenantID string, fieldKey string, scopeType string, scopeKey string, disabledOn string, requestID string, initiatorUUID string) (orgUnitTenantFieldPolicy, bool, error) {
	if s.disableFn != nil {
		return s.disableFn(ctx, tenantID, fieldKey, scopeType, scopeKey, disabledOn, requestID, initiatorUUID)
	}
	return orgUnitTenantFieldPolicy{}, false, nil
}

type orgUnitStoreWithFieldConfigsAndPolicies struct {
	OrgUnitStore
	listConfigsFn func(ctx context.Context, tenantID string) ([]orgUnitTenantFieldConfig, error)
	resolveFn     func(ctx context.Context, tenantID string, fieldKey string, scopeType string, scopeKey string, asOf string) (orgUnitTenantFieldPolicy, bool, error)
	enableFn      func(ctx context.Context, tenantID string, fieldKey string, valueType string, dataSourceType string, dataSourceConfig json.RawMessage, displayLabel *string, enabledOn string, requestID string, initiatorUUID string) (orgUnitTenantFieldConfig, bool, error)
	disableFn     func(ctx context.Context, tenantID string, fieldKey string, disabledOn string, requestID string, initiatorUUID string) (orgUnitTenantFieldConfig, bool, error)
}

func (s orgUnitStoreWithFieldConfigsAndPolicies) ListTenantFieldConfigs(ctx context.Context, tenantID string) ([]orgUnitTenantFieldConfig, error) {
	if s.listConfigsFn != nil {
		return s.listConfigsFn(ctx, tenantID)
	}
	return []orgUnitTenantFieldConfig{}, nil
}

func (s orgUnitStoreWithFieldConfigsAndPolicies) ResolveTenantFieldPolicy(ctx context.Context, tenantID string, fieldKey string, scopeType string, scopeKey string, asOf string) (orgUnitTenantFieldPolicy, bool, error) {
	if s.resolveFn != nil {
		return s.resolveFn(ctx, tenantID, fieldKey, scopeType, scopeKey, asOf)
	}
	return orgUnitTenantFieldPolicy{}, false, nil
}

func (s orgUnitStoreWithFieldConfigsAndPolicies) EnableTenantFieldConfig(ctx context.Context, tenantID string, fieldKey string, valueType string, dataSourceType string, dataSourceConfig json.RawMessage, displayLabel *string, enabledOn string, requestID string, initiatorUUID string) (orgUnitTenantFieldConfig, bool, error) {
	if s.enableFn != nil {
		return s.enableFn(ctx, tenantID, fieldKey, valueType, dataSourceType, dataSourceConfig, displayLabel, enabledOn, requestID, initiatorUUID)
	}
	return orgUnitTenantFieldConfig{}, false, nil
}

func (s orgUnitStoreWithFieldConfigsAndPolicies) DisableTenantFieldConfig(ctx context.Context, tenantID string, fieldKey string, disabledOn string, requestID string, initiatorUUID string) (orgUnitTenantFieldConfig, bool, error) {
	if s.disableFn != nil {
		return s.disableFn(ctx, tenantID, fieldKey, disabledOn, requestID, initiatorUUID)
	}
	return orgUnitTenantFieldConfig{}, false, nil
}

func (s orgUnitStoreWithFieldConfigsAndPolicies) ListTenantFieldPolicies(context.Context, string) ([]orgUnitTenantFieldPolicy, error) {
	return []orgUnitTenantFieldPolicy{}, nil
}

func (s orgUnitStoreWithFieldConfigsAndPolicies) UpsertTenantFieldPolicy(context.Context, string, string, string, string, bool, string, *string, string, string, string) (orgUnitTenantFieldPolicy, bool, error) {
	return orgUnitTenantFieldPolicy{}, false, nil
}

func (s orgUnitStoreWithFieldConfigsAndPolicies) DisableTenantFieldPolicy(context.Context, string, string, string, string, string, string, string) (orgUnitTenantFieldPolicy, bool, error) {
	return orgUnitTenantFieldPolicy{}, false, nil
}

func TestHandleOrgUnitFieldPoliciesAPI_WriteDisabled(t *testing.T) {
	store := orgUnitStoreWithFieldPolicies{OrgUnitStore: newOrgUnitMemoryStore()}
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-policies", bytes.NewReader([]byte(`{
		"field_key":"org_code",
		"scope_type":"FORM",
		"scope_key":"orgunit.create_dialog",
		"maintainable":false,
		"default_mode":"CEL",
		"default_rule_expr":"next_org_code(\"O\", 6)",
		"enabled_on":"2026-01-01",
		"request_id":"fp1"
	}`)))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	rec := httptest.NewRecorder()
	handleOrgUnitFieldPoliciesAPI(rec, req, store)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["code"] != "write_disabled" {
		t.Fatalf("code=%v", payload["code"])
	}
}

func TestHandleOrgUnitFieldPoliciesAPI_InvalidExpr(t *testing.T) {
	store := orgUnitStoreWithFieldPolicies{OrgUnitStore: newOrgUnitMemoryStore()}
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-policies", bytes.NewReader([]byte(`{
		"field_key":"org_code",
		"scope_type":"FORM",
		"scope_key":"orgunit.create_dialog",
		"maintainable":false,
		"default_mode":"CEL",
		"default_rule_expr":"1+1",
		"enabled_on":"2026-01-01",
		"request_id":"fp1"
	}`)))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	rec := httptest.NewRecorder()
	handleOrgUnitFieldPoliciesAPI(rec, req, store)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["code"] != "write_disabled" {
		t.Fatalf("code=%v", payload["code"])
	}
}

func TestHandleOrgUnitFieldPoliciesResolvePreviewAPI_DefaultFallback(t *testing.T) {
	store := orgUnitStoreWithFieldPolicies{
		OrgUnitStore: newOrgUnitMemoryStore(),
		resolveFn: func(ctx context.Context, tenantID string, fieldKey string, scopeType string, scopeKey string, asOf string) (orgUnitTenantFieldPolicy, bool, error) {
			return orgUnitTenantFieldPolicy{}, false, nil
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-policies:resolve-preview?field_key=org_code&scope_type=FORM&scope_key=orgunit.create_dialog&as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	rec := httptest.NewRecorder()
	handleOrgUnitFieldPoliciesResolvePreviewAPI(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var body orgUnitFieldPoliciesResolvePreviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.ResolvedPolicy.ScopeType != "SYSTEM_DEFAULT" || !body.ResolvedPolicy.Maintainable {
		t.Fatalf("resolved=%+v", body.ResolvedPolicy)
	}
}

func TestHandleOrgUnitFieldPoliciesDisableAPI_WriteDisabled(t *testing.T) {
	store := orgUnitStoreWithFieldPolicies{OrgUnitStore: newOrgUnitMemoryStore()}
	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-policies:disable", bytes.NewReader([]byte(`{
		"field_key":"org_code",
		"scope_type":"FORM",
		"scope_key":"orgunit.create_dialog",
		"disabled_on":"2026-02-01",
		"request_id":"fp-disable"
	}`)))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	rec := httptest.NewRecorder()
	handleOrgUnitFieldPoliciesDisableAPI(rec, req, store)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["code"] != "write_disabled" {
		t.Fatalf("code=%v", payload["code"])
	}
}

func TestHandleOrgUnitFieldPoliciesAPI_ErrorAndRetryBranches(t *testing.T) {
	base := newOrgUnitMemoryStore()

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-policies", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldPoliciesAPI(rec, req, orgUnitStoreWithFieldPolicies{OrgUnitStore: base})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	postCases := []struct {
		name string
		body string
	}{
		{name: "tenant missing", body: `{}`},
		{name: "store missing", body: `{}`},
		{name: "bad json", body: `{`},
		{name: "required fields missing", body: `{"field_key":"","enabled_on":"","request_id":""}`},
		{name: "enabled_on invalid", body: `{"field_key":"org_code","scope_type":"FORM","scope_key":"orgunit.create_dialog","enabled_on":"bad","request_id":"r1"}`},
		{name: "field key not allowed", body: `{"field_key":"bad_field","scope_type":"FORM","scope_key":"orgunit.create_dialog","enabled_on":"2026-01-01","request_id":"r1"}`},
		{name: "scope invalid", body: `{"field_key":"org_code","scope_type":"FORM","scope_key":"bad.scope","enabled_on":"2026-01-01","request_id":"r1"}`},
		{name: "default mode invalid", body: `{"field_key":"org_code","scope_type":"FORM","scope_key":"orgunit.create_dialog","default_mode":"BAD","enabled_on":"2026-01-01","request_id":"r1"}`},
		{name: "default mode empty falls back to none", body: `{"field_key":"org_code","scope_type":"FORM","scope_key":"orgunit.create_dialog","enabled_on":"2026-01-01","request_id":"r1"}`},
		{name: "cel mode requires expression", body: `{"field_key":"org_code","scope_type":"FORM","scope_key":"orgunit.create_dialog","default_mode":"CEL","enabled_on":"2026-01-01","request_id":"r1"}`},
		{name: "system managed policy requires cel default mode", body: `{"field_key":"org_code","scope_type":"FORM","scope_key":"orgunit.create_dialog","maintainable":false,"default_mode":"NONE","enabled_on":"2026-01-01","request_id":"r1"}`},
		{name: "upsert error", body: `{"field_key":"org_code","scope_type":"FORM","scope_key":"orgunit.create_dialog","default_mode":"NONE","enabled_on":"2026-01-01","request_id":"r1"}`},
		{name: "was retry returns 200", body: `{"field_key":"org_code","scope_type":"GLOBAL","scope_key":"ignored","default_mode":"NONE","enabled_on":"2026-01-01","request_id":"r1"}`},
	}
	for _, tc := range postCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-policies", bytes.NewReader([]byte(tc.body)))
			if tc.name != "tenant missing" {
				req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
			}
			rec := httptest.NewRecorder()
			handleOrgUnitFieldPoliciesAPI(rec, req, orgUnitStoreWithFieldPolicies{OrgUnitStore: base})
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			var payload map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if payload["code"] != "write_disabled" {
				t.Fatalf("code=%v", payload["code"])
			}
		})
	}
}

func TestHandleOrgUnitFieldPoliciesDisableAPI_ErrorAndSuccessBranches(t *testing.T) {
	base := newOrgUnitMemoryStore()

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-policies:disable", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldPoliciesDisableAPI(rec, req, orgUnitStoreWithFieldPolicies{OrgUnitStore: base})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	postCases := []struct {
		name string
		body string
	}{
		{name: "tenant missing", body: `{}`},
		{name: "store missing", body: `{}`},
		{name: "bad json", body: `{`},
		{name: "invalid required fields", body: `{"field_key":"","disabled_on":"","request_id":""}`},
		{name: "disabled_on invalid", body: `{"field_key":"org_code","scope_type":"FORM","scope_key":"orgunit.create_dialog","disabled_on":"bad","request_id":"r1"}`},
		{name: "scope invalid", body: `{"field_key":"org_code","scope_type":"FORM","scope_key":"bad.scope","disabled_on":"2026-01-01","request_id":"r1"}`},
		{name: "store error", body: `{"field_key":"org_code","scope_type":"FORM","scope_key":"orgunit.create_dialog","disabled_on":"2026-01-01","request_id":"r1"}`},
		{name: "success", body: `{"field_key":"org_code","scope_type":"FORM","scope_key":"orgunit.create_dialog","disabled_on":"2026-02-01","request_id":"r1"}`},
	}
	for _, tc := range postCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-policies:disable", bytes.NewReader([]byte(tc.body)))
			if tc.name != "tenant missing" {
				req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
			}
			rec := httptest.NewRecorder()
			handleOrgUnitFieldPoliciesDisableAPI(rec, req, orgUnitStoreWithFieldPolicies{OrgUnitStore: base})
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			var payload map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if payload["code"] != "write_disabled" {
				t.Fatalf("code=%v", payload["code"])
			}
		})
	}
}

func TestHandleOrgUnitFieldPoliciesResolvePreviewAPI_ErrorAndFoundBranches(t *testing.T) {
	base := newOrgUnitMemoryStore()

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-policies:resolve-preview", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldPoliciesResolvePreviewAPI(rec, req, orgUnitStoreWithFieldPolicies{OrgUnitStore: base})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-policies:resolve-preview?field_key=org_code&scope_type=FORM&scope_key=orgunit.create_dialog&as_of=2026-01-01", nil)
		rec := httptest.NewRecorder()
		handleOrgUnitFieldPoliciesResolvePreviewAPI(rec, req, orgUnitStoreWithFieldPolicies{OrgUnitStore: base})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("store missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-policies:resolve-preview?field_key=org_code&scope_type=FORM&scope_key=orgunit.create_dialog&as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldPoliciesResolvePreviewAPI(rec, req, base)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid as_of", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-policies:resolve-preview?field_key=org_code&scope_type=FORM&scope_key=orgunit.create_dialog&as_of=bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldPoliciesResolvePreviewAPI(rec, req, orgUnitStoreWithFieldPolicies{OrgUnitStore: base})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("field key missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-policies:resolve-preview?scope_type=FORM&scope_key=orgunit.create_dialog&as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldPoliciesResolvePreviewAPI(rec, req, orgUnitStoreWithFieldPolicies{OrgUnitStore: base})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("scope invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-policies:resolve-preview?field_key=org_code&scope_type=FORM&scope_key=bad.scope&as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldPoliciesResolvePreviewAPI(rec, req, orgUnitStoreWithFieldPolicies{OrgUnitStore: base})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("resolve error", func(t *testing.T) {
		store := orgUnitStoreWithFieldPolicies{
			OrgUnitStore: base,
			resolveFn: func(context.Context, string, string, string, string, string) (orgUnitTenantFieldPolicy, bool, error) {
				return orgUnitTenantFieldPolicy{}, false, errors.New("boom")
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-policies:resolve-preview?field_key=org_code&scope_type=FORM&scope_key=orgunit.create_dialog&as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldPoliciesResolvePreviewAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("found policy", func(t *testing.T) {
		rule := "next_org_code(\"O\", 6)"
		store := orgUnitStoreWithFieldPolicies{
			OrgUnitStore: base,
			resolveFn: func(context.Context, string, string, string, string, string) (orgUnitTenantFieldPolicy, bool, error) {
				return orgUnitTenantFieldPolicy{
					FieldKey:        "org_code",
					ScopeType:       "FORM",
					ScopeKey:        "orgunit.create_dialog",
					Maintainable:    false,
					DefaultMode:     "CEL",
					DefaultRuleExpr: &rule,
					EnabledOn:       "2026-01-01",
				}, true, nil
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-policies:resolve-preview?field_key=org_code&scope_type=FORM&scope_key=orgunit.create_dialog&as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldPoliciesResolvePreviewAPI(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var body orgUnitFieldPoliciesResolvePreviewResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal err=%v", err)
		}
		if body.ResolvedPolicy.DefaultMode != "CEL" || body.ResolvedPolicy.DefaultRuleExpr == nil {
			t.Fatalf("resolved=%+v", body.ResolvedPolicy)
		}
	})
}

func TestOrgUnitFieldPolicyHelpers(t *testing.T) {
	t.Run("isAllowedOrgUnitPolicyFieldKey", func(t *testing.T) {
		if isAllowedOrgUnitPolicyFieldKey("") {
			t.Fatalf("empty should be false")
		}
		if !isAllowedOrgUnitPolicyFieldKey("org_code") {
			t.Fatalf("core field should be true")
		}
		if !isAllowedOrgUnitPolicyFieldKey("org_type") {
			t.Fatalf("builtin field should be true")
		}
		if !isAllowedOrgUnitPolicyFieldKey("d_org_type") {
			t.Fatalf("dict custom field should be true")
		}
		if !isAllowedOrgUnitPolicyFieldKey("x_custom_01") {
			t.Fatalf("plain custom field should be true")
		}
		if isAllowedOrgUnitPolicyFieldKey("bad-field") {
			t.Fatalf("bad field should be false")
		}
	})

	t.Run("normalizeFieldPolicyScope", func(t *testing.T) {
		if gotType, gotKey, ok := normalizeFieldPolicyScope("GLOBAL", "ignored"); !ok || gotType != "GLOBAL" || gotKey != "global" {
			t.Fatalf("global: %q %q %v", gotType, gotKey, ok)
		}
		if gotType, gotKey, ok := normalizeFieldPolicyScope("", "orgunit.create_dialog"); !ok || gotType != "FORM" || gotKey != "orgunit.create_dialog" {
			t.Fatalf("default form: %q %q %v", gotType, gotKey, ok)
		}
		if gotType, gotKey, ok := normalizeFieldPolicyScope("FORM", "orgunit.details.add_version_dialog"); !ok || gotType != "FORM" || gotKey != "orgunit.details.add_version_dialog" {
			t.Fatalf("add version form: %q %q %v", gotType, gotKey, ok)
		}
		if gotType, gotKey, ok := normalizeFieldPolicyScope("FORM", "orgunit.details.insert_version_dialog"); !ok || gotType != "FORM" || gotKey != "orgunit.details.insert_version_dialog" {
			t.Fatalf("insert version form: %q %q %v", gotType, gotKey, ok)
		}
		if gotType, gotKey, ok := normalizeFieldPolicyScope("FORM", "orgunit.details.correct_dialog"); !ok || gotType != "FORM" || gotKey != "orgunit.details.correct_dialog" {
			t.Fatalf("correct form: %q %q %v", gotType, gotKey, ok)
		}
		if _, _, ok := normalizeFieldPolicyScope("FORM", "bad.scope"); ok {
			t.Fatalf("bad form scope should fail")
		}
		if _, _, ok := normalizeFieldPolicyScope("BAD", "x"); ok {
			t.Fatalf("bad type should fail")
		}
	})

	t.Run("validateFieldPolicyCELExpr", func(t *testing.T) {
		if err := validateFieldPolicyCELExpr("next_org_code(\"O\", 6)"); err != nil {
			t.Fatalf("expected valid expr, err=%v", err)
		}
		if err := validateFieldPolicyCELExpr("next_org_code('O', 6)"); err == nil {
			t.Fatalf("expected single-quote style error")
		}
		if err := validateFieldPolicyCELExpr("next_org_code(\"O\", )"); err == nil {
			t.Fatalf("expected syntax error")
		}
		if err := validateFieldPolicyCELExpr("1+1"); err == nil {
			t.Fatalf("expected non-string error")
		}
	})

	t.Run("cel env factory error", func(t *testing.T) {
		if got := orgUnitFieldPolicyCELNextOrgCode(); got == nil {
			t.Fatalf("expected non-nil result")
		}
		orig := newOrgUnitFieldPolicyCELEnv
		newOrgUnitFieldPolicyCELEnv = func() (*cel.Env, error) { return nil, errors.New("env") }
		t.Cleanup(func() { newOrgUnitFieldPolicyCELEnv = orig })
		if err := validateFieldPolicyCELExpr("next_org_code(\"O\", 6)"); err == nil || err.Error() != "env" {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestHandleOrgUnitFieldConfigsAPI_WithPolicyStoreCoverage(t *testing.T) {
	now := time.Unix(123, 0).UTC()
	base := newOrgUnitMemoryStore()

	t.Run("policy found applies to core and ext", func(t *testing.T) {
		rule := "next_org_code(\"O\", 6)"
		store := orgUnitStoreWithFieldConfigsAndPolicies{
			OrgUnitStore: base,
			listConfigsFn: func(context.Context, string) ([]orgUnitTenantFieldConfig, error) {
				return []orgUnitTenantFieldConfig{
					{
						FieldKey:         "x_custom_01",
						ValueType:        "text",
						DataSourceType:   "PLAIN",
						DataSourceConfig: json.RawMessage(`{}`),
						PhysicalCol:      "ext_str_01",
						EnabledOn:        "2026-01-01",
						UpdatedAt:        now,
					},
				}, nil
			},
			resolveFn: func(context.Context, string, string, string, string, string) (orgUnitTenantFieldPolicy, bool, error) {
				return orgUnitTenantFieldPolicy{
					FieldKey:        "org_code",
					ScopeType:       "FORM",
					ScopeKey:        "orgunit.create_dialog",
					Maintainable:    false,
					DefaultMode:     "CEL",
					DefaultRuleExpr: &rule,
					EnabledOn:       "2026-01-01",
					UpdatedAt:       now,
				}, true, nil
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-configs?as_of=2026-01-01&status=all", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsAPI(rec, req, store, newDictMemoryStore())
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var body orgUnitFieldConfigsAPIResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal err=%v", err)
		}
		if len(body.FieldConfigs) == 0 {
			t.Fatalf("expected items")
		}
	})

	t.Run("policy resolve error on core", func(t *testing.T) {
		store := orgUnitStoreWithFieldConfigsAndPolicies{
			OrgUnitStore: base,
			listConfigsFn: func(context.Context, string) ([]orgUnitTenantFieldConfig, error) {
				return []orgUnitTenantFieldConfig{}, nil
			},
			resolveFn: func(context.Context, string, string, string, string, string) (orgUnitTenantFieldPolicy, bool, error) {
				return orgUnitTenantFieldPolicy{}, false, errors.New("boom")
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-configs?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsAPI(rec, req, store, newDictMemoryStore())
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("policy resolve error on ext", func(t *testing.T) {
		callN := 0
		store := orgUnitStoreWithFieldConfigsAndPolicies{
			OrgUnitStore: base,
			listConfigsFn: func(context.Context, string) ([]orgUnitTenantFieldConfig, error) {
				return []orgUnitTenantFieldConfig{
					{
						FieldKey:         "x_custom_01",
						ValueType:        "text",
						DataSourceType:   "PLAIN",
						DataSourceConfig: json.RawMessage(`{}`),
						PhysicalCol:      "ext_str_01",
						EnabledOn:        "2026-01-01",
						UpdatedAt:        now,
					},
				}, nil
			},
			resolveFn: func(_ context.Context, _ string, fieldKey string, _ string, _ string, _ string) (orgUnitTenantFieldPolicy, bool, error) {
				callN++
				if fieldKey == "x_custom_01" {
					return orgUnitTenantFieldPolicy{}, false, errors.New("boom")
				}
				return orgUnitTenantFieldPolicy{}, false, nil
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-configs?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsAPI(rec, req, store, newDictMemoryStore())
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if callN == 0 {
			t.Fatalf("expected resolve calls")
		}
	})
}
