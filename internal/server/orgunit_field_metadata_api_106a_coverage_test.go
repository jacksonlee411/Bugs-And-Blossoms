package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dictpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/dict"
)

type dictRegistryStoreStub struct {
	listFn func(ctx context.Context, tenantID string, asOf string) ([]DictItem, error)
}

func (s dictRegistryStoreStub) ListDicts(ctx context.Context, tenantID string, asOf string) ([]DictItem, error) {
	if s.listFn != nil {
		return s.listFn(ctx, tenantID, asOf)
	}
	return []DictItem{}, nil
}

type dictOptionsErrResolver struct{}

func (dictOptionsErrResolver) ResolveValueLabel(context.Context, string, string, string, string) (string, bool, error) {
	return "", false, errors.New("boom")
}
func (dictOptionsErrResolver) ListOptions(context.Context, string, string, string, string, int) ([]dictpkg.Option, error) {
	return nil, errors.New("boom")
}

func TestHandleOrgUnitFieldConfigsEnableCandidatesAPI_BranchCoverage(t *testing.T) {
	t.Run("enabled_on invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-configs:enable-candidates?enabled_on=bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsEnableCandidatesAPI(rec, req, newDictMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("dict store nil", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-configs:enable-candidates?enabled_on=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsEnableCandidatesAPI(rec, req, nil)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("dict list error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-configs:enable-candidates?enabled_on=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsEnableCandidatesAPI(rec, req, dictRegistryStoreStub{
			listFn: func(context.Context, string, string) ([]DictItem, error) { return nil, errors.New("boom") },
		})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("filters invalid dicts and falls back name", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-configs:enable-candidates?enabled_on=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsEnableCandidatesAPI(rec, req, dictRegistryStoreStub{
			listFn: func(_ context.Context, _ string, _ string) ([]DictItem, error) {
				return []DictItem{
					{DictCode: " ", Name: "blank"},                            // skipped
					{DictCode: strings.Repeat("a", 62), Name: "too long"},     // skipped
					{DictCode: "bad-code", Name: "invalid dict_code format"},  // skipped by IsCustomDictFieldKey
					{DictCode: "cost_center", Name: " "},                      // name fallback to dict_code
					{DictCode: "org_type", Name: "Org Type"},                  // ok
					{DictCode: "org_type", Name: "duplicate, should be kept"}, // duplicates are allowed at this layer
					{DictCode: "location_code", Name: "Location Code"},        // ok
				}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		var body orgUnitFieldConfigsEnableCandidatesAPIResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if body.EnabledOn != "2026-01-01" {
			t.Fatalf("enabled_on=%q", body.EnabledOn)
		}
		// Ensure some expected stable items exist.
		foundCostCenter := false
		foundOrgType := false
		foundLocationCode := false
		for _, f := range body.DictFields {
			switch f.FieldKey {
			case "d_cost_center":
				foundCostCenter = true
				if f.Name != "cost_center" {
					t.Fatalf("name=%q", f.Name)
				}
			case "d_org_type":
				foundOrgType = true
			case "d_location_code":
				foundLocationCode = true
			}
		}
		if !foundCostCenter || !foundOrgType || !foundLocationCode {
			t.Fatalf("dict_fields=%+v", body.DictFields)
		}
	})
}

func TestHandleOrgUnitFieldConfigsAPI_WasRetryAndMethodNotAllowed(t *testing.T) {
	base := newOrgUnitMemoryStore()

	dictStore := dictRegistryStoreStub{
		listFn: func(context.Context, string, string) ([]DictItem, error) {
			return []DictItem{{DictCode: "org_type", Name: "Org Type"}}, nil
		},
	}

	t.Run("method not allowed", func(t *testing.T) {
		store := orgUnitStoreWithFieldConfigs{OrgUnitStore: base}
		req := httptest.NewRequest(http.MethodPut, "/org/api/org-units/field-configs", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsAPI(rec, req, store, dictStore)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("dict enable returns 200 on request retry", func(t *testing.T) {
		now := time.Unix(123, 0).UTC()
		var gotDisplayLabel *string
		store := orgUnitStoreWithFieldConfigs{
			OrgUnitStore: base,
			enableFn: func(_ context.Context, _ string, fieldKey string, valueType string, dataSourceType string, _ json.RawMessage, displayLabel *string, enabledOn string, requestCode string, _ string) (orgUnitTenantFieldConfig, bool, error) {
				gotDisplayLabel = cloneOptionalString(displayLabel)
				if fieldKey != "d_org_type" || valueType != "text" || dataSourceType != "DICT" || enabledOn != "2026-01-01" || requestCode != "r1" {
					t.Fatalf("unexpected args field=%s vt=%s dst=%s enabled_on=%s request=%s", fieldKey, valueType, dataSourceType, enabledOn, requestCode)
				}
				return orgUnitTenantFieldConfig{
					FieldKey:       fieldKey,
					ValueType:      valueType,
					DataSourceType: dataSourceType,
					DataSourceConfig: json.RawMessage(`{
  "dict_code":"org_type"
}`),
					PhysicalCol:  "ext_str_01",
					EnabledOn:    enabledOn,
					UpdatedAt:    now,
					DisplayLabel: cloneOptionalString(displayLabel),
				}, true, nil
			},
		}

		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-configs", bytes.NewReader([]byte(`{"field_key":"d_org_type","enabled_on":"2026-01-01","request_code":"r1"}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsAPI(rec, req, store, dictStore)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if gotDisplayLabel == nil || *gotDisplayLabel != "Org Type" {
			t.Fatalf("display_label=%v", gotDisplayLabel)
		}
	})

	t.Run("builtin enable returns 200 on request retry", func(t *testing.T) {
		now := time.Unix(123, 0).UTC()
		store := orgUnitStoreWithFieldConfigs{
			OrgUnitStore: base,
			enableFn: func(_ context.Context, _ string, fieldKey string, valueType string, dataSourceType string, _ json.RawMessage, displayLabel *string, enabledOn string, requestCode string, _ string) (orgUnitTenantFieldConfig, bool, error) {
				if displayLabel != nil {
					t.Fatalf("display_label should be nil for builtin fields")
				}
				return orgUnitTenantFieldConfig{
					FieldKey:       fieldKey,
					ValueType:      valueType,
					DataSourceType: dataSourceType,
					DataSourceConfig: json.RawMessage(`{
}`),
					PhysicalCol: "ext_str_02",
					EnabledOn:   enabledOn,
					UpdatedAt:   now,
				}, true, nil
			},
		}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-configs", bytes.NewReader([]byte(`{"field_key":"short_name","enabled_on":"2026-01-01","request_code":"r1"}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsAPI(rec, req, store, dictStore)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("builtin enable invalid data_source_config is rejected", func(t *testing.T) {
		store := orgUnitStoreWithFieldConfigs{
			OrgUnitStore: base,
			enableFn: func(context.Context, string, string, string, string, json.RawMessage, *string, string, string, string) (orgUnitTenantFieldConfig, bool, error) {
				t.Fatalf("EnableTenantFieldConfig should not be called on invalid config")
				return orgUnitTenantFieldConfig{}, false, nil
			},
		}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-configs", bytes.NewReader([]byte(`{"field_key":"short_name","enabled_on":"2026-01-01","request_code":"r1","data_source_config":{"x":1}}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsAPI(rec, req, store, dictStore)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("builtin enable store error is mapped", func(t *testing.T) {
		store := orgUnitStoreWithFieldConfigs{
			OrgUnitStore: base,
			enableFn: func(context.Context, string, string, string, string, json.RawMessage, *string, string, string, string) (orgUnitTenantFieldConfig, bool, error) {
				return orgUnitTenantFieldConfig{}, false, errors.New("boom")
			},
		}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-configs", bytes.NewReader([]byte(`{"field_key":"short_name","enabled_on":"2026-01-01","request_code":"r1"}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsAPI(rec, req, store, dictStore)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestHandleOrgUnitFieldOptionsAPI_MoreBranches(t *testing.T) {
	base := newOrgUnitMemoryStore()

	t.Run("DICT but data_source_config invalid => not supported", func(t *testing.T) {
		store := orgUnitStoreWithEnabledFieldConfig{
			OrgUnitStore: base,
			ok:           true,
			cfg:          orgUnitTenantFieldConfig{FieldKey: "d_org_type", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{}`)},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/fields:options?as_of=2026-01-01&field_key=d_org_type", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldOptionsAPI(rec, req, store)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("d_ key but suffix/config mismatch => not supported", func(t *testing.T) {
		store := orgUnitStoreWithEnabledFieldConfig{
			OrgUnitStore: base,
			ok:           true,
			cfg:          orgUnitTenantFieldConfig{FieldKey: "d_org_type", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"other"}`)},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/fields:options?as_of=2026-01-01&field_key=d_org_type", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldOptionsAPI(rec, req, store)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("builtin DICT but field-definition missing => not supported", func(t *testing.T) {
		store := orgUnitStoreWithEnabledFieldConfig{
			OrgUnitStore: base,
			ok:           true,
			cfg:          orgUnitTenantFieldConfig{FieldKey: "missing", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/fields:options?as_of=2026-01-01&field_key=missing", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldOptionsAPI(rec, req, store)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("dict options resolver error => 500", func(t *testing.T) {
		if err := dictpkg.RegisterResolver(dictOptionsErrResolver{}); err != nil {
			t.Fatalf("register err=%v", err)
		}
		t.Cleanup(func() { _ = dictpkg.RegisterResolver(orgunitDictResolverStub{}) })

		store := orgUnitStoreWithEnabledFieldConfig{
			OrgUnitStore: base,
			ok:           true,
			cfg:          orgUnitTenantFieldConfig{FieldKey: "d_org_type", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/fields:options?as_of=2026-01-01&field_key=d_org_type", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldOptionsAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestNormalizeOrgUnitEnableDataSourceConfig_EntityMarshalErrorIsSkipped(t *testing.T) {
	def := orgUnitFieldDefinition{
		DataSourceType: "ENTITY",
		DataSourceConfigOptions: []map[string]any{
			{"bad": func() {}}, // json.Marshal should fail.
			{"entity": "person", "id_kind": "uuid"},
		},
	}
	cfg, ok, err := normalizeOrgUnitEnableDataSourceConfig(context.Background(), "t1", "2026-01-01", newDictMemoryStore(), def, json.RawMessage(`{"entity":"person","id_kind":"uuid"}`))
	if err != nil || !ok || string(cfg) != `{"entity":"person","id_kind":"uuid"}` {
		t.Fatalf("cfg=%s ok=%v err=%v", string(cfg), ok, err)
	}
}

func TestNormalizeOrgUnitEnableDataSourceConfig_PlainInvalidJSON(t *testing.T) {
	def := orgUnitFieldDefinition{DataSourceType: "PLAIN"}
	if _, ok, err := normalizeOrgUnitEnableDataSourceConfig(context.Background(), "t1", "2026-01-01", newDictMemoryStore(), def, json.RawMessage(`{`)); err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

func TestNormalizeOrgUnitEnableDataSourceConfigForDictFieldKey_MoreBranches(t *testing.T) {
	t.Run("raw invalid json rejects", func(t *testing.T) {
		cfg, _, ok, err := normalizeOrgUnitEnableDataSourceConfigForDictFieldKey(context.Background(), "t1", "2026-01-01", newDictMemoryStore(), "org_type", json.RawMessage(`{`))
		if err != nil || ok || cfg != nil {
			t.Fatalf("cfg=%v ok=%v err=%v", cfg, ok, err)
		}
	})

	t.Run("dict store list error bubbles", func(t *testing.T) {
		_, _, _, err := normalizeOrgUnitEnableDataSourceConfigForDictFieldKey(context.Background(), "t1", "2026-01-01", dictRegistryStoreStub{
			listFn: func(context.Context, string, string) ([]DictItem, error) { return nil, errors.New("boom") },
		}, "org_type", nil)
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("dict name blank falls back to dict_code", func(t *testing.T) {
		cfg, name, ok, err := normalizeOrgUnitEnableDataSourceConfigForDictFieldKey(context.Background(), "t1", "2026-01-01", dictRegistryStoreStub{
			listFn: func(context.Context, string, string) ([]DictItem, error) {
				return []DictItem{{DictCode: "org_type", Name: " "}}, nil
			},
		}, "org_type", nil)
		if err != nil || !ok || string(cfg) != `{"dict_code":"org_type"}` || name != "org_type" {
			t.Fatalf("cfg=%s name=%q ok=%v err=%v", string(cfg), name, ok, err)
		}
	})

	t.Run("dict code blank rejects", func(t *testing.T) {
		cfg, name, ok, err := normalizeOrgUnitEnableDataSourceConfigForDictFieldKey(context.Background(), "t1", "2026-01-01", newDictMemoryStore(), " ", nil)
		if err != nil || ok || cfg != nil || name != "" {
			t.Fatalf("cfg=%v name=%q ok=%v err=%v", cfg, name, ok, err)
		}
	})

	t.Run("dict store nil returns error", func(t *testing.T) {
		_, _, _, err := normalizeOrgUnitEnableDataSourceConfigForDictFieldKey(context.Background(), "t1", "2026-01-01", nil, "org_type", nil)
		if err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestOrgUnitFieldConfigPresentation_Branches(t *testing.T) {
	// Built-in uses i18n key.
	if key, label, allowFilter, allowSort := orgUnitFieldConfigPresentation(orgUnitTenantFieldConfig{FieldKey: "short_name"}); key == nil || *key == "" || label != nil || allowFilter || allowSort {
		t.Fatalf("got key=%v label=%v filter=%v sort=%v", key, label, allowFilter, allowSort)
	}

	// Dict field uses display label when present.
	lbl := "组织类型"
	if key, label, allowFilter, allowSort := orgUnitFieldConfigPresentation(orgUnitTenantFieldConfig{FieldKey: "d_org_type", DisplayLabel: &lbl}); key != nil || label == nil || *label != "组织类型" || !allowFilter || !allowSort {
		t.Fatalf("got key=%v label=%v filter=%v sort=%v", key, label, allowFilter, allowSort)
	}

	// Dict field falls back to dict_code.
	if key, label, allowFilter, allowSort := orgUnitFieldConfigPresentation(orgUnitTenantFieldConfig{FieldKey: "d_org_type"}); key != nil || label == nil || *label != "org_type" || !allowFilter || !allowSort {
		t.Fatalf("got key=%v label=%v filter=%v sort=%v", key, label, allowFilter, allowSort)
	}

	// Unknown fields are non-queryable and label is field_key.
	if key, label, allowFilter, allowSort := orgUnitFieldConfigPresentation(orgUnitTenantFieldConfig{FieldKey: "x_cost_center"}); key != nil || label == nil || *label != "x_cost_center" || allowFilter || allowSort {
		t.Fatalf("got key=%v label=%v filter=%v sort=%v", key, label, allowFilter, allowSort)
	}
	customLabel := "成本中心"
	if key, label, allowFilter, allowSort := orgUnitFieldConfigPresentation(orgUnitTenantFieldConfig{FieldKey: "x_cost_center", DisplayLabel: &customLabel}); key != nil || label == nil || *label != customLabel || allowFilter || allowSort {
		t.Fatalf("got key=%v label=%v filter=%v sort=%v", key, label, allowFilter, allowSort)
	}
}
