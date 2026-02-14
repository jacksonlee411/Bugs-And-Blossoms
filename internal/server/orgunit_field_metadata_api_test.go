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
)

type orgUnitStoreWithFieldConfigs struct {
	OrgUnitStore

	listFn    func(ctx context.Context, tenantID string) ([]orgUnitTenantFieldConfig, error)
	enableFn  func(ctx context.Context, tenantID string, fieldKey string, valueType string, dataSourceType string, dataSourceConfig json.RawMessage, enabledOn string, requestCode string, initiatorUUID string) (orgUnitTenantFieldConfig, bool, error)
	disableFn func(ctx context.Context, tenantID string, fieldKey string, disabledOn string, requestCode string, initiatorUUID string) (orgUnitTenantFieldConfig, bool, error)
}

func (s orgUnitStoreWithFieldConfigs) ListTenantFieldConfigs(ctx context.Context, tenantID string) ([]orgUnitTenantFieldConfig, error) {
	if s.listFn != nil {
		return s.listFn(ctx, tenantID)
	}
	return []orgUnitTenantFieldConfig{}, nil
}

func (s orgUnitStoreWithFieldConfigs) EnableTenantFieldConfig(ctx context.Context, tenantID string, fieldKey string, valueType string, dataSourceType string, dataSourceConfig json.RawMessage, enabledOn string, requestCode string, initiatorUUID string) (orgUnitTenantFieldConfig, bool, error) {
	if s.enableFn != nil {
		return s.enableFn(ctx, tenantID, fieldKey, valueType, dataSourceType, dataSourceConfig, enabledOn, requestCode, initiatorUUID)
	}
	return orgUnitTenantFieldConfig{}, false, nil
}

func (s orgUnitStoreWithFieldConfigs) DisableTenantFieldConfig(ctx context.Context, tenantID string, fieldKey string, disabledOn string, requestCode string, initiatorUUID string) (orgUnitTenantFieldConfig, bool, error) {
	if s.disableFn != nil {
		return s.disableFn(ctx, tenantID, fieldKey, disabledOn, requestCode, initiatorUUID)
	}
	return orgUnitTenantFieldConfig{}, false, nil
}

type orgUnitStoreWithEnabledFieldConfig struct {
	OrgUnitStore
	cfg orgUnitTenantFieldConfig
	ok  bool
	err error
}

func (s orgUnitStoreWithEnabledFieldConfig) GetEnabledTenantFieldConfigAsOf(ctx context.Context, tenantID string, fieldKey string, asOf string) (orgUnitTenantFieldConfig, bool, error) {
	return s.cfg, s.ok, s.err
}

func TestHandleOrgUnitFieldDefinitionsAPI(t *testing.T) {
	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-definitions", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldDefinitionsAPI(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-definitions", nil)
		rec := httptest.NewRecorder()
		handleOrgUnitFieldDefinitionsAPI(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-definitions", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldDefinitionsAPI(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var body orgUnitFieldDefinitionsAPIResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(body.Fields) == 0 {
			t.Fatalf("expected fields")
		}

		// Contract (DEV-PLAN-100D2): DICT/ENTITY must include non-empty data_source_config_options.
		prevKey := ""
		for _, f := range body.Fields {
			if strings.TrimSpace(f.FieldKey) == "" {
				t.Fatalf("field_key blank")
			}
			// Ensure stable ordering.
			if prevKey != "" && f.FieldKey < prevKey {
				t.Fatalf("fields not sorted: %q before %q", prevKey, f.FieldKey)
			}
			prevKey = f.FieldKey

			switch strings.ToUpper(strings.TrimSpace(f.DataSourceType)) {
			case "DICT", "ENTITY":
				if len(f.DataSourceConfigOptions) == 0 {
					t.Fatalf("field %q expected non-empty data_source_config_options", f.FieldKey)
				}
				for _, raw := range f.DataSourceConfigOptions {
					var tmp map[string]any
					if err := json.Unmarshal(raw, &tmp); err != nil || tmp == nil || len(tmp) == 0 {
						t.Fatalf("field %q has invalid option=%s err=%v", f.FieldKey, string(raw), err)
					}
				}
			default:
				if f.DataSourceConfigOptions != nil {
					t.Fatalf("field %q expected data_source_config_options omitted", f.FieldKey)
				}
			}
		}
	})
}

func TestHandleOrgUnitFieldConfigsAPI(t *testing.T) {
	base := newOrgUnitMemoryStore()

	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-configs?as_of=2026-01-01", nil)
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsAPI(rec, req, base)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("store missing interface", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-configs?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsAPI(rec, req, base)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get invalid as_of", func(t *testing.T) {
		store := orgUnitStoreWithFieldConfigs{OrgUnitStore: base}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-configs?as_of=bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get invalid status", func(t *testing.T) {
		store := orgUnitStoreWithFieldConfigs{OrgUnitStore: base}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-configs?as_of=2026-01-01&status=bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get list error", func(t *testing.T) {
		store := orgUnitStoreWithFieldConfigs{
			OrgUnitStore: base,
			listFn: func(context.Context, string) ([]orgUnitTenantFieldConfig, error) {
				return nil, errors.New("boom")
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-configs?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get success with status filter", func(t *testing.T) {
		now := time.Unix(123, 0).UTC()
		disabledOn := "2026-02-01"
		store := orgUnitStoreWithFieldConfigs{
			OrgUnitStore: base,
			listFn: func(context.Context, string) ([]orgUnitTenantFieldConfig, error) {
				return []orgUnitTenantFieldConfig{
					{FieldKey: "org_type", ValueType: "text", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`), PhysicalCol: "ext_str_01", EnabledOn: "2026-01-01", DisabledOn: nil, UpdatedAt: now},
					{FieldKey: "short_name", ValueType: "text", DataSourceType: "PLAIN", DataSourceConfig: json.RawMessage(`{}`), PhysicalCol: "ext_str_02", EnabledOn: "2026-01-01", DisabledOn: &disabledOn, UpdatedAt: now},
				}, nil
			},
		}

		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-configs?as_of=2026-01-10&status=enabled", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsAPI(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		var body orgUnitFieldConfigsAPIResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(body.FieldConfigs) != 2 {
			t.Fatalf("len=%d", len(body.FieldConfigs))
		}

		req2 := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-configs?as_of=2026-03-01&status=enabled", nil)
		req2 = req2.WithContext(withTenant(req2.Context(), Tenant{ID: "t1"}))
		rec2 := httptest.NewRecorder()
		handleOrgUnitFieldConfigsAPI(rec2, req2, store)
		if rec2.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec2.Code, rec2.Body.String())
		}
		var body2 orgUnitFieldConfigsAPIResponse
		if err := json.Unmarshal(rec2.Body.Bytes(), &body2); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(body2.FieldConfigs) != 1 {
			t.Fatalf("len=%d", len(body2.FieldConfigs))
		}
	})

	t.Run("get success with disabled status filter", func(t *testing.T) {
		now := time.Unix(123, 0).UTC()
		disabledOn := "2026-02-01"
		store := orgUnitStoreWithFieldConfigs{
			OrgUnitStore: base,
			listFn: func(context.Context, string) ([]orgUnitTenantFieldConfig, error) {
				return []orgUnitTenantFieldConfig{
					{FieldKey: "org_type", ValueType: "text", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`), PhysicalCol: "ext_str_01", EnabledOn: "2026-01-01", DisabledOn: nil, UpdatedAt: now},
					{FieldKey: "short_name", ValueType: "text", DataSourceType: "PLAIN", DataSourceConfig: json.RawMessage(`{}`), PhysicalCol: "ext_str_02", EnabledOn: "2026-01-01", DisabledOn: &disabledOn, UpdatedAt: now},
				}, nil
			},
		}

		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-configs?as_of=2026-03-01&status=disabled", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsAPI(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var body orgUnitFieldConfigsAPIResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(body.FieldConfigs) != 1 || body.FieldConfigs[0].FieldKey != "short_name" {
			t.Fatalf("items=%v", body.FieldConfigs)
		}
	})

	t.Run("post bad json", func(t *testing.T) {
		store := orgUnitStoreWithFieldConfigs{OrgUnitStore: base}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-configs", strings.NewReader("{"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post invalid request", func(t *testing.T) {
		store := orgUnitStoreWithFieldConfigs{OrgUnitStore: base}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-configs", bytes.NewReader([]byte(`{"field_key":"","enabled_on":"","request_code":""}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post enabled_on invalid", func(t *testing.T) {
		store := orgUnitStoreWithFieldConfigs{OrgUnitStore: base}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-configs", bytes.NewReader([]byte(`{"field_key":"org_type","enabled_on":"bad","request_code":"r1"}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post invalid data_source_config (missing) maps to bad request", func(t *testing.T) {
		store := orgUnitStoreWithFieldConfigs{OrgUnitStore: base}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-configs", bytes.NewReader([]byte(`{"field_key":"org_type","enabled_on":"2026-01-01","request_code":"r1"}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload["code"] != orgUnitErrFieldConfigInvalidDataSourceConfig {
			t.Fatalf("code=%v", payload["code"])
		}
	})

	t.Run("post plain missing data_source_config defaults to {}", func(t *testing.T) {
		now := time.Unix(456, 0).UTC()
		var gotCfg json.RawMessage
		store := orgUnitStoreWithFieldConfigs{
			OrgUnitStore: base,
			enableFn: func(_ context.Context, _ string, _ string, _ string, _ string, dataSourceConfig json.RawMessage, _ string, _ string, _ string) (orgUnitTenantFieldConfig, bool, error) {
				gotCfg = append([]byte(nil), dataSourceConfig...)
				return orgUnitTenantFieldConfig{
					FieldKey:         "short_name",
					ValueType:        "text",
					DataSourceType:   "PLAIN",
					DataSourceConfig: dataSourceConfig,
					PhysicalCol:      "ext_str_01",
					EnabledOn:        "2026-01-01",
					UpdatedAt:        now,
				}, false, nil
			},
		}

		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-configs", bytes.NewReader([]byte(`{"field_key":"short_name","enabled_on":"2026-01-01","request_code":"r1"}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsAPI(rec, req, store)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if strings.TrimSpace(string(gotCfg)) != "{}" {
			t.Fatalf("got data_source_config=%s", string(gotCfg))
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if raw, ok := payload["data_source_config"]; !ok || raw == nil {
			t.Fatalf("missing data_source_config in response: %v", payload)
		}
	})

	t.Run("post field definition not found", func(t *testing.T) {
		store := orgUnitStoreWithFieldConfigs{OrgUnitStore: base}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-configs", bytes.NewReader([]byte(`{"field_key":"nope","enabled_on":"2026-01-01","request_code":"r1"}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsAPI(rec, req, store)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("post store error maps to conflict", func(t *testing.T) {
		store := orgUnitStoreWithFieldConfigs{
			OrgUnitStore: base,
			enableFn: func(context.Context, string, string, string, string, json.RawMessage, string, string, string) (orgUnitTenantFieldConfig, bool, error) {
				return orgUnitTenantFieldConfig{}, false, errors.New(orgUnitErrFieldConfigSlotExhausted)
			},
		}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-configs", bytes.NewReader([]byte(`{"field_key":"org_type","enabled_on":"2026-01-01","request_code":"r1","data_source_config":{"dict_code":"org_type"}}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsAPI(rec, req, store)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("post success created and retry", func(t *testing.T) {
		now := time.Unix(456, 0).UTC()
		cfg := orgUnitTenantFieldConfig{
			FieldKey:         "org_type",
			ValueType:        "text",
			DataSourceType:   "DICT",
			DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`),
			PhysicalCol:      "ext_str_01",
			EnabledOn:        "2026-01-01",
			UpdatedAt:        now,
		}
		wasRetry := false
		store := orgUnitStoreWithFieldConfigs{
			OrgUnitStore: base,
			enableFn: func(context.Context, string, string, string, string, json.RawMessage, string, string, string) (orgUnitTenantFieldConfig, bool, error) {
				return cfg, wasRetry, nil
			},
		}

		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-configs", bytes.NewReader([]byte(`{"field_key":"org_type","enabled_on":"2026-01-01","request_code":"r1","data_source_config":{"dict_code":"org_type"}}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsAPI(rec, req, store)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		wasRetry = true
		req2 := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-configs", bytes.NewReader([]byte(`{"field_key":"org_type","enabled_on":"2026-01-01","request_code":"r1","data_source_config":{"dict_code":"org_type"}}`)))
		req2 = req2.WithContext(withTenant(req2.Context(), Tenant{ID: "t1"}))
		rec2 := httptest.NewRecorder()
		handleOrgUnitFieldConfigsAPI(rec2, req2, store)
		if rec2.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec2.Code, rec2.Body.String())
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		store := orgUnitStoreWithFieldConfigs{OrgUnitStore: base}
		req := httptest.NewRequest(http.MethodPut, "/org/api/org-units/field-configs", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsAPI(rec, req, store)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestHandleOrgUnitFieldConfigsDisableAPI(t *testing.T) {
	base := newOrgUnitMemoryStore()

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-configs:disable", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsDisableAPI(rec, req, base)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-configs:disable", nil)
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsDisableAPI(rec, req, base)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("store missing interface", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-configs:disable", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsDisableAPI(rec, req, base)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("bad json", func(t *testing.T) {
		store := orgUnitStoreWithFieldConfigs{OrgUnitStore: base}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-configs:disable", strings.NewReader("{"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsDisableAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid request", func(t *testing.T) {
		store := orgUnitStoreWithFieldConfigs{OrgUnitStore: base}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-configs:disable", bytes.NewReader([]byte(`{"field_key":"","disabled_on":"","request_code":""}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsDisableAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("disabled_on invalid", func(t *testing.T) {
		store := orgUnitStoreWithFieldConfigs{OrgUnitStore: base}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-configs:disable", bytes.NewReader([]byte(`{"field_key":"org_type","disabled_on":"bad","request_code":"r1"}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsDisableAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("store error maps to conflict", func(t *testing.T) {
		store := orgUnitStoreWithFieldConfigs{
			OrgUnitStore: base,
			disableFn: func(context.Context, string, string, string, string, string) (orgUnitTenantFieldConfig, bool, error) {
				return orgUnitTenantFieldConfig{}, false, errors.New(orgUnitErrFieldConfigDisabledOnInvalid)
			},
		}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-configs:disable", bytes.NewReader([]byte(`{"field_key":"org_type","disabled_on":"2026-02-01","request_code":"r1"}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsDisableAPI(rec, req, store)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("success", func(t *testing.T) {
		now := time.Unix(789, 0).UTC()
		disabledOn := "2026-02-01"
		cfg := orgUnitTenantFieldConfig{
			FieldKey:         "org_type",
			ValueType:        "text",
			DataSourceType:   "DICT",
			DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`),
			PhysicalCol:      "ext_str_01",
			EnabledOn:        "2026-01-01",
			DisabledOn:       &disabledOn,
			UpdatedAt:        now,
		}
		store := orgUnitStoreWithFieldConfigs{
			OrgUnitStore: base,
			disableFn: func(context.Context, string, string, string, string, string) (orgUnitTenantFieldConfig, bool, error) {
				return cfg, false, nil
			},
		}
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-configs:disable", bytes.NewReader([]byte(`{"field_key":"org_type","disabled_on":"2026-02-01","request_code":"r1"}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsDisableAPI(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestHandleOrgUnitFieldOptionsAPI(t *testing.T) {
	base := newOrgUnitMemoryStore()

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/fields:options", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldOptionsAPI(rec, req, base)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/fields:options?as_of=2026-01-01&field_key=org_type", nil)
		rec := httptest.NewRecorder()
		handleOrgUnitFieldOptionsAPI(rec, req, base)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("store missing interface", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/fields:options?as_of=2026-01-01&field_key=org_type", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldOptionsAPI(rec, req, base)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid as_of", func(t *testing.T) {
		store := orgUnitStoreWithEnabledFieldConfig{OrgUnitStore: base}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/fields:options?as_of=bad&field_key=org_type", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldOptionsAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("field_key required", func(t *testing.T) {
		store := orgUnitStoreWithEnabledFieldConfig{OrgUnitStore: base}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/fields:options?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldOptionsAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("store error", func(t *testing.T) {
		store := orgUnitStoreWithEnabledFieldConfig{OrgUnitStore: base, err: errors.New("boom")}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/fields:options?as_of=2026-01-01&field_key=org_type", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldOptionsAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("field not enabled", func(t *testing.T) {
		store := orgUnitStoreWithEnabledFieldConfig{OrgUnitStore: base, ok: false}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/fields:options?as_of=2026-01-01&field_key=org_type", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldOptionsAPI(rec, req, store)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload["code"] != orgUnitErrFieldOptionsFieldNotEnabled {
			t.Fatalf("code=%v", payload["code"])
		}
	})

	t.Run("plain not supported", func(t *testing.T) {
		store := orgUnitStoreWithEnabledFieldConfig{
			OrgUnitStore: base,
			ok:           true,
			cfg:          orgUnitTenantFieldConfig{FieldKey: "short_name", DataSourceType: "PLAIN"},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/fields:options?as_of=2026-01-01&field_key=short_name", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldOptionsAPI(rec, req, store)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload["code"] != orgUnitErrFieldOptionsNotSupported {
			t.Fatalf("code=%v", payload["code"])
		}
	})

	t.Run("dict definition missing", func(t *testing.T) {
		store := orgUnitStoreWithEnabledFieldConfig{
			OrgUnitStore: base,
			ok:           true,
			cfg:          orgUnitTenantFieldConfig{FieldKey: "missing", DataSourceType: "DICT"},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/fields:options?as_of=2026-01-01&field_key=missing", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldOptionsAPI(rec, req, store)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("dict config mismatches definition data_source_type fails closed", func(t *testing.T) {
		store := orgUnitStoreWithEnabledFieldConfig{
			OrgUnitStore: base,
			ok:           true,
			cfg:          orgUnitTenantFieldConfig{FieldKey: "short_name", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/fields:options?as_of=2026-01-01&field_key=short_name", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldOptionsAPI(rec, req, store)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload["code"] != orgUnitErrFieldOptionsNotSupported {
			t.Fatalf("code=%v", payload["code"])
		}
	})

	t.Run("dict_code empty treated as not supported", func(t *testing.T) {
		store := orgUnitStoreWithEnabledFieldConfig{
			OrgUnitStore: base,
			ok:           true,
			cfg:          orgUnitTenantFieldConfig{FieldKey: "org_type", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":" "}`)},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/fields:options?as_of=2026-01-01&field_key=org_type", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldOptionsAPI(rec, req, store)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload["code"] != orgUnitErrFieldOptionsNotSupported {
			t.Fatalf("code=%v", payload["code"])
		}
	})

	t.Run("dict success with keyword and limit parsing", func(t *testing.T) {
		store := orgUnitStoreWithEnabledFieldConfig{
			OrgUnitStore: base,
			ok:           true,
			cfg:          orgUnitTenantFieldConfig{FieldKey: "org_type", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)},
		}

		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/fields:options?as_of=2026-01-01&field_key=org_type&limit=bad&q=comp", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldOptionsAPI(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "COMPANY") {
			t.Fatalf("body=%s", rec.Body.String())
		}

		req2 := httptest.NewRequest(http.MethodGet, "/org/api/org-units/fields:options?as_of=2026-01-01&field_key=org_type&limit=100", nil)
		req2 = req2.WithContext(withTenant(req2.Context(), Tenant{ID: "t1"}))
		rec2 := httptest.NewRecorder()
		handleOrgUnitFieldOptionsAPI(rec2, req2, store)
		if rec2.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec2.Code, rec2.Body.String())
		}
	})

	t.Run("entity not supported", func(t *testing.T) {
		store := orgUnitStoreWithEnabledFieldConfig{
			OrgUnitStore: base,
			ok:           true,
			cfg:          orgUnitTenantFieldConfig{FieldKey: "org_type", DataSourceType: "ENTITY"},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/fields:options?as_of=2026-01-01&field_key=org_type", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldOptionsAPI(rec, req, store)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload["code"] != orgUnitErrFieldOptionsNotSupported {
			t.Fatalf("code=%v", payload["code"])
		}
	})
}

func TestOrgUnitFieldMetadataAPI_HelperCoverage(t *testing.T) {
	t.Run("orgUnitFieldDataSourceConfigOptionsJSON nil for plain", func(t *testing.T) {
		def, ok := lookupOrgUnitFieldDefinition("short_name")
		if !ok {
			t.Fatalf("expected short_name definition")
		}
		if got := orgUnitFieldDataSourceConfigOptionsJSON(def); got != nil {
			t.Fatalf("expected nil, got=%v", got)
		}
	})

	t.Run("orgUnitFieldDataSourceConfigOptionsJSON sorts and skips marshal errors", func(t *testing.T) {
		def := orgUnitFieldDefinition{
			FieldKey:       "x",
			DataSourceType: "DICT",
			DataSourceConfigOptions: []map[string]any{
				{"dict_code": "b"},
				{"bad": func() {}}, // json.Marshal should fail.
				{"dict_code": "a"},
			},
		}
		got := orgUnitFieldDataSourceConfigOptionsJSON(def)
		if len(got) != 2 {
			t.Fatalf("len=%d got=%v", len(got), got)
		}
		if string(got[0]) != `{"dict_code":"a"}` || string(got[1]) != `{"dict_code":"b"}` {
			t.Fatalf("got=%v", got)
		}
	})

	t.Run("normalizeOrgUnitEnableDataSourceConfig plain", func(t *testing.T) {
		def := orgUnitFieldDefinition{DataSourceType: "PLAIN"}
		if cfg, ok := normalizeOrgUnitEnableDataSourceConfig(def, nil); !ok || string(cfg) != "{}" {
			t.Fatalf("cfg=%s ok=%v", string(cfg), ok)
		}
		if cfg, ok := normalizeOrgUnitEnableDataSourceConfig(def, json.RawMessage(`{}`)); !ok || string(cfg) != "{}" {
			t.Fatalf("cfg=%s ok=%v", string(cfg), ok)
		}
		if _, ok := normalizeOrgUnitEnableDataSourceConfig(def, json.RawMessage(`{"x":1}`)); ok {
			t.Fatalf("expected non-empty object to be rejected")
		}
		if _, ok := normalizeOrgUnitEnableDataSourceConfig(def, json.RawMessage(`[]`)); ok {
			t.Fatalf("expected non-object to be rejected")
		}
		if _, ok := normalizeOrgUnitEnableDataSourceConfig(def, json.RawMessage(`null`)); ok {
			t.Fatalf("expected null to be rejected")
		}
	})

	t.Run("normalizeOrgUnitEnableDataSourceConfig dict", func(t *testing.T) {
		def := orgUnitFieldDefinition{
			DataSourceType: "DICT",
			DataSourceConfigOptions: []map[string]any{
				{"bad": func() {}}, // should be skipped
				{"dict_code": "org_type"},
			},
		}

		if _, ok := normalizeOrgUnitEnableDataSourceConfig(def, nil); ok {
			t.Fatalf("expected missing config to fail")
		}
		if _, ok := normalizeOrgUnitEnableDataSourceConfig(def, json.RawMessage(`null`)); ok {
			t.Fatalf("expected null config to fail")
		}
		if _, ok := normalizeOrgUnitEnableDataSourceConfig(def, json.RawMessage(`{`)); ok {
			t.Fatalf("expected invalid json to fail")
		}
		if _, ok := normalizeOrgUnitEnableDataSourceConfig(def, json.RawMessage(`[]`)); ok {
			t.Fatalf("expected non-object to fail")
		}
		if _, ok := normalizeOrgUnitEnableDataSourceConfig(def, json.RawMessage(`{"dict_code":"missing"}`)); ok {
			t.Fatalf("expected option mismatch to fail")
		}

		cfg, ok := normalizeOrgUnitEnableDataSourceConfig(def, json.RawMessage(`{"dict_code":"org_type"}`))
		if !ok || string(cfg) != `{"dict_code":"org_type"}` {
			t.Fatalf("cfg=%s ok=%v", string(cfg), ok)
		}
	})

	t.Run("normalizeOrgUnitEnableDataSourceConfig default rejects", func(t *testing.T) {
		if _, ok := normalizeOrgUnitEnableDataSourceConfig(orgUnitFieldDefinition{DataSourceType: "NOPE"}, json.RawMessage(`{}`)); ok {
			t.Fatalf("expected unknown type to be rejected")
		}
	})

	t.Run("dictCodeFromDataSourceConfig", func(t *testing.T) {
		if _, ok := dictCodeFromDataSourceConfig(nil); ok {
			t.Fatalf("expected empty to fail")
		}
		if _, ok := dictCodeFromDataSourceConfig(json.RawMessage(`{`)); ok {
			t.Fatalf("expected invalid json to fail")
		}
		if _, ok := dictCodeFromDataSourceConfig(json.RawMessage(`null`)); ok {
			t.Fatalf("expected null to fail")
		}
		if _, ok := dictCodeFromDataSourceConfig(json.RawMessage(`{}`)); ok {
			t.Fatalf("expected missing dict_code to fail")
		}
		if _, ok := dictCodeFromDataSourceConfig(json.RawMessage(`{"dict_code":1}`)); ok {
			t.Fatalf("expected non-string dict_code to fail")
		}
		if _, ok := dictCodeFromDataSourceConfig(json.RawMessage(`{"dict_code":"  "}`)); ok {
			t.Fatalf("expected blank dict_code to fail")
		}
		if got, ok := dictCodeFromDataSourceConfig(json.RawMessage(`{"dict_code":" org_type "}`)); !ok || got != "org_type" {
			t.Fatalf("got=%q ok=%v", got, ok)
		}
	})
}
