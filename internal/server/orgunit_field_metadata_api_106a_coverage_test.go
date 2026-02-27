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
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
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

type orgUnitCodeResolverStub struct {
	OrgUnitCodeResolver
	resolveOrgIDFn func(ctx context.Context, tenantID string, orgCode string) (int, error)
}

func (s orgUnitCodeResolverStub) ResolveOrgID(ctx context.Context, tenantID string, orgCode string) (int, error) {
	if s.resolveOrgIDFn != nil {
		return s.resolveOrgIDFn(ctx, tenantID, orgCode)
	}
	if s.OrgUnitCodeResolver != nil {
		return s.OrgUnitCodeResolver.ResolveOrgID(ctx, tenantID, orgCode)
	}
	return 0, errors.New("org_code_resolver_missing")
}

func (s orgUnitCodeResolverStub) ResolveOrgCode(ctx context.Context, tenantID string, orgID int) (string, error) {
	if s.OrgUnitCodeResolver != nil {
		return s.OrgUnitCodeResolver.ResolveOrgCode(ctx, tenantID, orgID)
	}
	return "", errors.New("org_code_resolver_missing")
}

func (s orgUnitCodeResolverStub) ResolveOrgCodes(ctx context.Context, tenantID string, orgIDs []int) (map[int]string, error) {
	if s.OrgUnitCodeResolver != nil {
		return s.OrgUnitCodeResolver.ResolveOrgCodes(ctx, tenantID, orgIDs)
	}
	return nil, errors.New("org_code_resolver_missing")
}

type setIDGovernanceStoreStub struct {
	SetIDGovernanceStore
	resolveSetIDFn func(ctx context.Context, tenantID string, orgUnitID string, asOfDate string) (string, error)
}

func (s setIDGovernanceStoreStub) ResolveSetID(ctx context.Context, tenantID string, orgUnitID string, asOfDate string) (string, error) {
	if s.resolveSetIDFn != nil {
		return s.resolveSetIDFn(ctx, tenantID, orgUnitID, asOfDate)
	}
	if s.SetIDGovernanceStore != nil {
		return s.SetIDGovernanceStore.ResolveSetID(ctx, tenantID, orgUnitID, asOfDate)
	}
	return "", errors.New("SETID_NOT_FOUND")
}

type orgUnitStoreWithEnabledFieldConfigAndSetID struct {
	*orgUnitMemoryStore
	cfg            orgUnitTenantFieldConfig
	ok             bool
	err            error
	resolveSetIDFn func(ctx context.Context, tenantID string, orgUnitID string, asOfDate string) (string, error)
}

func (s orgUnitStoreWithEnabledFieldConfigAndSetID) GetEnabledTenantFieldConfigAsOf(_ context.Context, _ string, _ string, _ string) (orgUnitTenantFieldConfig, bool, error) {
	return s.cfg, s.ok, s.err
}

func (s orgUnitStoreWithEnabledFieldConfigAndSetID) ResolveSetID(ctx context.Context, tenantID string, orgUnitID string, asOfDate string) (string, error) {
	if s.resolveSetIDFn != nil {
		return s.resolveSetIDFn(ctx, tenantID, orgUnitID, asOfDate)
	}
	return s.orgUnitMemoryStore.ResolveSetID(ctx, tenantID, orgUnitID, asOfDate)
}

type orgUnitMemoryStoreResolveOrgIDErr struct {
	*orgUnitMemoryStore
	err error
}

func (s orgUnitMemoryStoreResolveOrgIDErr) ResolveOrgID(context.Context, string, string) (int, error) {
	if s.err != nil {
		return 0, s.err
	}
	return 0, errors.New("resolve_org_id_error")
}

func TestHandleOrgUnitFieldConfigsEnableCandidatesAPI_BranchCoverage(t *testing.T) {
	t.Run("enabled_on invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-configs:enable-candidates?enabled_on=bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsEnableCandidatesAPI(rec, req, newDictMemoryStore(), nil, nil)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("dict store nil", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-configs:enable-candidates?enabled_on=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsEnableCandidatesAPI(rec, req, nil, nil, nil)
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
		}, nil, nil)
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
		}, nil, nil)
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

	t.Run("org_code invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-configs:enable-candidates?enabled_on=2026-01-01&org_code=A%0A1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsEnableCandidatesAPI(rec, req, newDictMemoryStore(), nil, nil)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("org_code provided but resolver missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-configs:enable-candidates?enabled_on=2026-01-01&org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsEnableCandidatesAPI(rec, req, newDictMemoryStore(), nil, nil)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("setid resolver missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-configs:enable-candidates?enabled_on=2026-01-01&org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldConfigsEnableCandidatesAPI(rec, req, newDictMemoryStore(), orgUnitCodeResolverStub{
			resolveOrgIDFn: func(context.Context, string, string) (int, error) { return 10000001, nil },
		}, nil)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("org_code resolve failure mapping", func(t *testing.T) {
		cases := []struct {
			name   string
			err    error
			status int
		}{
			{name: "invalid", err: orgunitpkg.ErrOrgCodeInvalid, status: http.StatusBadRequest},
			{name: "not-found", err: orgunitpkg.ErrOrgCodeNotFound, status: http.StatusNotFound},
			{name: "internal", err: errors.New("boom"), status: http.StatusInternalServerError},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-configs:enable-candidates?enabled_on=2026-01-01&org_code=A001", nil)
				req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
				rec := httptest.NewRecorder()
				handleOrgUnitFieldConfigsEnableCandidatesAPI(rec, req, newDictMemoryStore(), orgUnitCodeResolverStub{
					resolveOrgIDFn: func(context.Context, string, string) (int, error) { return 0, tc.err },
				}, setIDGovernanceStoreStub{})
				if rec.Code != tc.status {
					t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
				}
			})
		}
	})

	t.Run("setid resolve fail and empty setid", func(t *testing.T) {
		t.Run("resolve error", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-configs:enable-candidates?enabled_on=2026-01-01&org_code=A001", nil)
			req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
			rec := httptest.NewRecorder()
			handleOrgUnitFieldConfigsEnableCandidatesAPI(rec, req, newDictMemoryStore(), orgUnitCodeResolverStub{
				resolveOrgIDFn: func(context.Context, string, string) (int, error) { return 10000001, nil },
			}, setIDGovernanceStoreStub{
				resolveSetIDFn: func(context.Context, string, string, string) (string, error) {
					return "", errors.New("SETID_NOT_FOUND")
				},
			})
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
		t.Run("empty setid", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-configs:enable-candidates?enabled_on=2026-01-01&org_code=A001", nil)
			req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
			rec := httptest.NewRecorder()
			handleOrgUnitFieldConfigsEnableCandidatesAPI(rec, req, newDictMemoryStore(), orgUnitCodeResolverStub{
				resolveOrgIDFn: func(context.Context, string, string) (int, error) { return 10000001, nil },
			}, setIDGovernanceStoreStub{
				resolveSetIDFn: func(context.Context, string, string, string) (string, error) { return " ", nil },
			})
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	})

	t.Run("org_code setid source mapping", func(t *testing.T) {
		dictStore := dictRegistryStoreStub{
			listFn: func(context.Context, string, string) ([]DictItem, error) {
				return []DictItem{{DictCode: "org_type", Name: "Org Type"}}, nil
			},
		}
		cases := []struct {
			name      string
			setID     string
			expectSrc string
		}{
			{name: "deflt", setID: "DEFLT", expectSrc: "deflt"},
			{name: "share", setID: "SHARE", expectSrc: "share_preview"},
			{name: "custom", setID: "S9000", expectSrc: "custom"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/field-configs:enable-candidates?enabled_on=2026-01-01&org_code=A001", nil)
				req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
				rec := httptest.NewRecorder()
				handleOrgUnitFieldConfigsEnableCandidatesAPI(rec, req, dictStore, orgUnitCodeResolverStub{
					resolveOrgIDFn: func(context.Context, string, string) (int, error) { return 10000001, nil },
				}, setIDGovernanceStoreStub{
					resolveSetIDFn: func(context.Context, string, string, string) (string, error) { return tc.setID, nil },
				})
				if rec.Code != http.StatusOK {
					t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
				}
				var body orgUnitFieldConfigsEnableCandidatesAPIResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatalf("unmarshal: %v", err)
				}
				if len(body.DictFields) != 1 || body.DictFields[0].SetID != tc.setID || body.DictFields[0].SetIDSource != tc.expectSrc {
					t.Fatalf("dict_fields=%+v", body.DictFields)
				}
			})
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
			enableFn: func(_ context.Context, _ string, fieldKey string, valueType string, dataSourceType string, _ json.RawMessage, displayLabel *string, enabledOn string, requestID string, _ string) (orgUnitTenantFieldConfig, bool, error) {
				gotDisplayLabel = cloneOptionalString(displayLabel)
				if fieldKey != "d_org_type" || valueType != "text" || dataSourceType != "DICT" || enabledOn != "2026-01-01" || requestID != "r1" {
					t.Fatalf("unexpected args field=%s vt=%s dst=%s enabled_on=%s request=%s", fieldKey, valueType, dataSourceType, enabledOn, requestID)
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

		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-configs", bytes.NewReader([]byte(`{"field_key":"d_org_type","enabled_on":"2026-01-01","request_id":"r1"}`)))
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
			enableFn: func(_ context.Context, _ string, fieldKey string, valueType string, dataSourceType string, _ json.RawMessage, displayLabel *string, enabledOn string, requestID string, _ string) (orgUnitTenantFieldConfig, bool, error) {
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
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-configs", bytes.NewReader([]byte(`{"field_key":"short_name","enabled_on":"2026-01-01","request_id":"r1"}`)))
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
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-configs", bytes.NewReader([]byte(`{"field_key":"short_name","enabled_on":"2026-01-01","request_id":"r1","data_source_config":{"x":1}}`)))
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
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/field-configs", bytes.NewReader([]byte(`{"field_key":"short_name","enabled_on":"2026-01-01","request_id":"r1"}`)))
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

	t.Run("org_code not found", func(t *testing.T) {
		store := orgUnitStoreWithEnabledFieldConfig{
			OrgUnitStore: base,
			ok:           true,
			cfg:          orgUnitTenantFieldConfig{FieldKey: "d_org_type", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/fields:options?as_of=2026-01-01&field_key=d_org_type&org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldOptionsAPI(rec, req, store)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("org_code resolver internal error", func(t *testing.T) {
		store := orgUnitStoreWithEnabledFieldConfig{
			OrgUnitStore: orgUnitMemoryStoreResolveOrgIDErr{orgUnitMemoryStore: base, err: errors.New("boom")},
			ok:           true,
			cfg:          orgUnitTenantFieldConfig{FieldKey: "d_org_type", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/fields:options?as_of=2026-01-01&field_key=d_org_type&org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldOptionsAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("setid resolver missing", func(t *testing.T) {
		if _, err := base.CreateNodeCurrent(context.Background(), "t1", "2026-01-01", "A001", "Org A", "", false); err != nil {
			t.Fatalf("create node: %v", err)
		}
		store := orgUnitStoreWithEnabledFieldConfig{
			OrgUnitStore: base,
			ok:           true,
			cfg:          orgUnitTenantFieldConfig{FieldKey: "d_org_type", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/fields:options?as_of=2026-01-01&field_key=d_org_type&org_code=A001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitFieldOptionsAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("setid resolve error and empty setid", func(t *testing.T) {
		mem := newOrgUnitMemoryStore()
		if _, err := mem.CreateNodeCurrent(context.Background(), "t1", "2026-01-01", "A001", "Org A", "", false); err != nil {
			t.Fatalf("create node: %v", err)
		}
		t.Run("resolve error", func(t *testing.T) {
			store := orgUnitStoreWithEnabledFieldConfigAndSetID{
				orgUnitMemoryStore: mem,
				ok:                 true,
				cfg:                orgUnitTenantFieldConfig{FieldKey: "d_org_type", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)},
				resolveSetIDFn: func(context.Context, string, string, string) (string, error) {
					return "", errors.New("SETID_NOT_FOUND")
				},
			}
			req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/fields:options?as_of=2026-01-01&field_key=d_org_type&org_code=A001", nil)
			req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
			rec := httptest.NewRecorder()
			handleOrgUnitFieldOptionsAPI(rec, req, store)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
		t.Run("empty setid", func(t *testing.T) {
			store := orgUnitStoreWithEnabledFieldConfigAndSetID{
				orgUnitMemoryStore: mem,
				ok:                 true,
				cfg:                orgUnitTenantFieldConfig{FieldKey: "d_org_type", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)},
				resolveSetIDFn:     func(context.Context, string, string, string) (string, error) { return " ", nil },
			}
			req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/fields:options?as_of=2026-01-01&field_key=d_org_type&org_code=A001", nil)
			req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
			rec := httptest.NewRecorder()
			handleOrgUnitFieldOptionsAPI(rec, req, store)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	})

	t.Run("org_code setid source mapping", func(t *testing.T) {
		if err := dictpkg.RegisterResolver(orgunitDictResolverStub{
			listFn: func(context.Context, string, string, string, string, int) ([]dictpkg.Option, error) {
				return []dictpkg.Option{{Code: "20", Label: "单位"}}, nil
			},
		}); err != nil {
			t.Fatalf("register resolver err=%v", err)
		}
		mem := newOrgUnitMemoryStore()
		if _, err := mem.CreateNodeCurrent(context.Background(), "t1", "2026-01-01", "A001", "Org A", "", false); err != nil {
			t.Fatalf("create node: %v", err)
		}
		cases := []struct {
			name      string
			setID     string
			expectSrc string
		}{
			{name: "deflt", setID: "DEFLT", expectSrc: "deflt"},
			{name: "share", setID: "SHARE", expectSrc: "share_preview"},
			{name: "custom", setID: "S9000", expectSrc: "custom"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				store := orgUnitStoreWithEnabledFieldConfigAndSetID{
					orgUnitMemoryStore: mem,
					ok:                 true,
					cfg:                orgUnitTenantFieldConfig{FieldKey: "d_org_type", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)},
					resolveSetIDFn:     func(context.Context, string, string, string) (string, error) { return tc.setID, nil },
				}
				req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/fields:options?as_of=2026-01-01&field_key=d_org_type&org_code=A001", nil)
				req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
				rec := httptest.NewRecorder()
				handleOrgUnitFieldOptionsAPI(rec, req, store)
				if rec.Code != http.StatusOK {
					t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
				}
				var body orgUnitFieldOptionsAPIResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatalf("unmarshal: %v", err)
				}
				if len(body.Options) != 1 || body.Options[0].SetID != tc.setID || body.Options[0].SetIDSource != tc.expectSrc {
					t.Fatalf("options=%+v", body.Options)
				}
			})
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
