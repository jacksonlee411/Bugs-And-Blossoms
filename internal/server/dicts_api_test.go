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

	dictpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/dict"
)

type dictStoreStub struct {
	listDictsFn         func(ctx context.Context, tenantID string, asOf string) ([]DictItem, error)
	createDictFn        func(ctx context.Context, tenantID string, req DictCreateRequest) (DictItem, bool, error)
	disableDictFn       func(ctx context.Context, tenantID string, req DictDisableRequest) (DictItem, bool, error)
	listDictValuesFn    func(ctx context.Context, tenantID string, dictCode string, asOf string, keyword string, limit int, status string) ([]DictValueItem, error)
	createFn            func(ctx context.Context, tenantID string, req DictCreateValueRequest) (DictValueItem, bool, error)
	disableFn           func(ctx context.Context, tenantID string, req DictDisableValueRequest) (DictValueItem, bool, error)
	correctFn           func(ctx context.Context, tenantID string, req DictCorrectValueRequest) (DictValueItem, bool, error)
	listAuditFn         func(ctx context.Context, tenantID string, dictCode string, code string, limit int) ([]DictValueAuditItem, error)
	resolveValueLabelFn func(ctx context.Context, tenantID string, asOf string, dictCode string, code string) (string, bool, error)
	listOptionsFn       func(ctx context.Context, tenantID string, asOf string, dictCode string, keyword string, limit int) ([]dictpkg.Option, error)
}

func (s dictStoreStub) CreateDict(ctx context.Context, tenantID string, req DictCreateRequest) (DictItem, bool, error) {
	if s.createDictFn != nil {
		return s.createDictFn(ctx, tenantID, req)
	}
	return DictItem{}, false, nil
}

func (s dictStoreStub) DisableDict(ctx context.Context, tenantID string, req DictDisableRequest) (DictItem, bool, error) {
	if s.disableDictFn != nil {
		return s.disableDictFn(ctx, tenantID, req)
	}
	return DictItem{}, false, nil
}

func (s dictStoreStub) ListDicts(ctx context.Context, tenantID string, asOf string) ([]DictItem, error) {
	if s.listDictsFn != nil {
		return s.listDictsFn(ctx, tenantID, asOf)
	}
	return []DictItem{}, nil
}

func (s dictStoreStub) ListDictValues(ctx context.Context, tenantID string, dictCode string, asOf string, keyword string, limit int, status string) ([]DictValueItem, error) {
	if s.listDictValuesFn != nil {
		return s.listDictValuesFn(ctx, tenantID, dictCode, asOf, keyword, limit, status)
	}
	return []DictValueItem{}, nil
}

func (s dictStoreStub) CreateDictValue(ctx context.Context, tenantID string, req DictCreateValueRequest) (DictValueItem, bool, error) {
	if s.createFn != nil {
		return s.createFn(ctx, tenantID, req)
	}
	return DictValueItem{}, false, nil
}

func (s dictStoreStub) DisableDictValue(ctx context.Context, tenantID string, req DictDisableValueRequest) (DictValueItem, bool, error) {
	if s.disableFn != nil {
		return s.disableFn(ctx, tenantID, req)
	}
	return DictValueItem{}, false, nil
}

func (s dictStoreStub) CorrectDictValue(ctx context.Context, tenantID string, req DictCorrectValueRequest) (DictValueItem, bool, error) {
	if s.correctFn != nil {
		return s.correctFn(ctx, tenantID, req)
	}
	return DictValueItem{}, false, nil
}

func (s dictStoreStub) ListDictValueAudit(ctx context.Context, tenantID string, dictCode string, code string, limit int) ([]DictValueAuditItem, error) {
	if s.listAuditFn != nil {
		return s.listAuditFn(ctx, tenantID, dictCode, code, limit)
	}
	return []DictValueAuditItem{}, nil
}

func (s dictStoreStub) ResolveValueLabel(ctx context.Context, tenantID string, asOf string, dictCode string, code string) (string, bool, error) {
	if s.resolveValueLabelFn != nil {
		return s.resolveValueLabelFn(ctx, tenantID, asOf, dictCode, code)
	}
	return "", false, nil
}

func (s dictStoreStub) ListOptions(ctx context.Context, tenantID string, asOf string, dictCode string, keyword string, limit int) ([]dictpkg.Option, error) {
	if s.listOptionsFn != nil {
		return s.listOptionsFn(ctx, tenantID, asOf, dictCode, keyword, limit)
	}
	return []dictpkg.Option{}, nil
}

func dictAPIRequest(method string, target string, body []byte, withTenantCtx bool) *http.Request {
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	if withTenantCtx {
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "00000000-0000-0000-0000-000000000001", Domain: "localhost", Name: "T1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "00000000-0000-0000-0000-000000000111", TenantID: "00000000-0000-0000-0000-000000000001", RoleSlug: "tenant-admin", Status: "active"}))
	}
	return req
}

func responseCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		return ""
	}
	code, _ := payload["code"].(string)
	return code
}

func TestHandleDictsAPI_Coverage(t *testing.T) {
	store := dictStoreStub{}

	t.Run("method not allowed", func(t *testing.T) {
		req := dictAPIRequest(http.MethodDelete, "/iam/api/dicts?as_of=2026-01-01", nil, true)
		rec := httptest.NewRecorder()
		handleDictsAPI(rec, req, store)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("tenant missing", func(t *testing.T) {
		req := dictAPIRequest(http.MethodGet, "/iam/api/dicts?as_of=2026-01-01", nil, false)
		rec := httptest.NewRecorder()
		handleDictsAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid as_of", func(t *testing.T) {
		req := dictAPIRequest(http.MethodGet, "/iam/api/dicts?as_of=bad", nil, true)
		rec := httptest.NewRecorder()
		handleDictsAPI(rec, req, store)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("store error", func(t *testing.T) {
		req := dictAPIRequest(http.MethodGet, "/iam/api/dicts?as_of=2026-01-01", nil, true)
		rec := httptest.NewRecorder()
		handleDictsAPI(rec, req, dictStoreStub{listDictsFn: func(context.Context, string, string) ([]DictItem, error) {
			return nil, errDictNotFound
		}})
		if rec.Code != http.StatusNotFound || responseCode(t, rec) != "dict_not_found" {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("success", func(t *testing.T) {
		req := dictAPIRequest(http.MethodGet, "/iam/api/dicts?as_of=2026-01-01", nil, true)
		rec := httptest.NewRecorder()
		handleDictsAPI(rec, req, dictStoreStub{listDictsFn: func(_ context.Context, tenantID string, asOf string) ([]DictItem, error) {
			if tenantID == "" || asOf != "2026-01-01" {
				t.Fatalf("tenant=%q asOf=%q", tenantID, asOf)
			}
			return []DictItem{{DictCode: dictCodeOrgType, Name: "Org Type", Status: "active", EnabledOn: "1970-01-01"}}, nil
		}})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestHandleDictsCreateAndDisableAPI_Coverage(t *testing.T) {
	t.Run("create dict validations", func(t *testing.T) {
		cases := []struct {
			name   string
			body   string
			status int
			store  DictStore
		}{
			{name: "bad json", body: "{", status: http.StatusBadRequest},
			{name: "dict required", body: `{"dict_code":"","name":"X","enabled_on":"2026-01-01","request_id":"r1"}`, status: http.StatusBadRequest},
			{name: "dict invalid", body: `{"dict_code":"bad-code","name":"X","enabled_on":"2026-01-01","request_id":"r1"}`, status: http.StatusBadRequest},
			{name: "name required", body: `{"dict_code":"org_type","name":"","enabled_on":"2026-01-01","request_id":"r1"}`, status: http.StatusBadRequest},
			{name: "enabled_on required", body: `{"dict_code":"org_type","name":"X","enabled_on":"bad","request_id":"r1"}`, status: http.StatusBadRequest},
			{name: "request required", body: `{"dict_code":"org_type","name":"X","enabled_on":"2026-01-01","request_id":""}`, status: http.StatusBadRequest},
			{name: "store error", body: `{"dict_code":"org_type","name":"X","enabled_on":"2026-01-01","request_id":"r1"}`, status: http.StatusConflict, store: dictStoreStub{createDictFn: func(context.Context, string, DictCreateRequest) (DictItem, bool, error) {
				return DictItem{}, false, errDictCodeConflict
			}}},
			{name: "ok retry", body: `{"dict_code":"org_type","name":"X","enabled_on":"2026-01-01","request_id":"r1"}`, status: http.StatusOK, store: dictStoreStub{createDictFn: func(context.Context, string, DictCreateRequest) (DictItem, bool, error) {
				return DictItem{DictCode: "org_type", Name: "X", Status: "active", EnabledOn: "2026-01-01"}, true, nil
			}}},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				store := tc.store
				if store == nil {
					store = dictStoreStub{}
				}
				req := dictAPIRequest(http.MethodPost, "/iam/api/dicts", []byte(tc.body), true)
				rec := httptest.NewRecorder()
				handleDictsAPI(rec, req, store)
				if rec.Code != tc.status {
					t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
				}
			})
		}
	})

	t.Run("create dict tenant missing", func(t *testing.T) {
		req := dictAPIRequest(http.MethodPost, "/iam/api/dicts", []byte(`{"dict_code":"org_type","name":"X","enabled_on":"2026-01-01","request_id":"r1"}`), false)
		rec := httptest.NewRecorder()
		handleDictsCreateAPI(rec, req, dictStoreStub{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("disable dict coverage", func(t *testing.T) {
		reqBadMethod := dictAPIRequest(http.MethodGet, "/iam/api/dicts:disable", nil, true)
		recBadMethod := httptest.NewRecorder()
		handleDictsDisableAPI(recBadMethod, reqBadMethod, dictStoreStub{})
		if recBadMethod.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", recBadMethod.Code)
		}

		reqBadJSON := dictAPIRequest(http.MethodPost, "/iam/api/dicts:disable", []byte("{"), true)
		recBadJSON := httptest.NewRecorder()
		handleDictsDisableAPI(recBadJSON, reqBadJSON, dictStoreStub{})
		if recBadJSON.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", recBadJSON.Code)
		}

		reqBad := dictAPIRequest(http.MethodPost, "/iam/api/dicts:disable", []byte(`{"dict_code":"","disabled_on":"bad","request_id":""}`), true)
		recBad := httptest.NewRecorder()
		handleDictsDisableAPI(recBad, reqBad, dictStoreStub{})
		if recBad.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", recBad.Code)
		}

		reqInvalidDate := dictAPIRequest(http.MethodPost, "/iam/api/dicts:disable", []byte(`{"dict_code":"org_type","disabled_on":"bad","request_id":"r1"}`), true)
		recInvalidDate := httptest.NewRecorder()
		handleDictsDisableAPI(recInvalidDate, reqInvalidDate, dictStoreStub{})
		if recInvalidDate.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", recInvalidDate.Code)
		}

		reqRequestRequired := dictAPIRequest(http.MethodPost, "/iam/api/dicts:disable", []byte(`{"dict_code":"org_type","disabled_on":"2026-01-01","request_id":""}`), true)
		recRequestRequired := httptest.NewRecorder()
		handleDictsDisableAPI(recRequestRequired, reqRequestRequired, dictStoreStub{})
		if recRequestRequired.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", recRequestRequired.Code)
		}

		reqNoTenant := dictAPIRequest(http.MethodPost, "/iam/api/dicts:disable", []byte(`{"dict_code":"org_type","disabled_on":"2026-01-01","request_id":"r1"}`), false)
		recNoTenant := httptest.NewRecorder()
		handleDictsDisableAPI(recNoTenant, reqNoTenant, dictStoreStub{})
		if recNoTenant.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", recNoTenant.Code)
		}

		reqErr := dictAPIRequest(http.MethodPost, "/iam/api/dicts:disable", []byte(`{"dict_code":"org_type","disabled_on":"2026-01-01","request_id":"r1"}`), true)
		recErr := httptest.NewRecorder()
		handleDictsDisableAPI(recErr, reqErr, dictStoreStub{disableDictFn: func(context.Context, string, DictDisableRequest) (DictItem, bool, error) {
			return DictItem{}, false, errDictCodeConflict
		}})
		if recErr.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s", recErr.Code, recErr.Body.String())
		}

		reqOK := dictAPIRequest(http.MethodPost, "/iam/api/dicts:disable", []byte(`{"dict_code":"org_type","disabled_on":"2026-01-01","request_id":"r1"}`), true)
		recOK := httptest.NewRecorder()
		handleDictsDisableAPI(recOK, reqOK, dictStoreStub{disableDictFn: func(context.Context, string, DictDisableRequest) (DictItem, bool, error) {
			return DictItem{DictCode: "org_type", Name: "Org Type", Status: "inactive", EnabledOn: "1970-01-01", DisabledOn: cloneOptionalString(new("2026-01-01"))}, false, nil
		}})
		if recOK.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", recOK.Code, recOK.Body.String())
		}
	})
}

//go:fix inline
func ptr(v string) *string { return new(v) }

func TestHandleDictValuesAPI_Coverage(t *testing.T) {
	t.Run("dispatch get", func(t *testing.T) {
		req := dictAPIRequest(http.MethodGet, "/iam/api/dicts/values?dict_code=org_type&as_of=2026-01-01&status=all", nil, true)
		rec := httptest.NewRecorder()
		handleDictValuesAPI(rec, req, dictStoreStub{})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("dispatch post", func(t *testing.T) {
		body := []byte(`{"dict_code":"org_type","code":"30","label":"中心","enabled_on":"2026-01-01","request_id":"r1"}`)
		req := dictAPIRequest(http.MethodPost, "/iam/api/dicts/values", body, true)
		rec := httptest.NewRecorder()
		handleDictValuesAPI(rec, req, dictStoreStub{createFn: func(_ context.Context, _ string, req DictCreateValueRequest) (DictValueItem, bool, error) {
			if req.Code != "30" || req.DictCode != "org_type" {
				t.Fatalf("req=%+v", req)
			}
			return DictValueItem{DictCode: req.DictCode, Code: req.Code, Label: req.Label, Status: "active", EnabledOn: req.EnabledOn, UpdatedAt: time.Unix(1, 0).UTC()}, false, nil
		}})
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("dispatch default", func(t *testing.T) {
		req := dictAPIRequest(http.MethodDelete, "/iam/api/dicts/values", nil, true)
		rec := httptest.NewRecorder()
		handleDictValuesAPI(rec, req, dictStoreStub{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestHandleDictValuesListAPI_Coverage(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		req := dictAPIRequest(http.MethodGet, "/iam/api/dicts/values?dict_code=org_type&as_of=2026-01-01", nil, false)
		rec := httptest.NewRecorder()
		handleDictValuesListAPI(rec, req, dictStoreStub{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid as_of", func(t *testing.T) {
		req := dictAPIRequest(http.MethodGet, "/iam/api/dicts/values?dict_code=org_type&as_of=bad", nil, true)
		rec := httptest.NewRecorder()
		handleDictValuesListAPI(rec, req, dictStoreStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("dict code required", func(t *testing.T) {
		req := dictAPIRequest(http.MethodGet, "/iam/api/dicts/values?as_of=2026-01-01", nil, true)
		rec := httptest.NewRecorder()
		handleDictValuesListAPI(rec, req, dictStoreStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("dict code invalid", func(t *testing.T) {
		req := dictAPIRequest(http.MethodGet, "/iam/api/dicts/values?dict_code=bad-code&as_of=2026-01-01", nil, true)
		rec := httptest.NewRecorder()
		handleDictValuesListAPI(rec, req, dictStoreStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("dict not found", func(t *testing.T) {
		req := dictAPIRequest(http.MethodGet, "/iam/api/dicts/values?dict_code=unknown&as_of=2026-01-01", nil, true)
		rec := httptest.NewRecorder()
		handleDictValuesListAPI(rec, req, dictStoreStub{listDictValuesFn: func(context.Context, string, string, string, string, int, string) ([]DictValueItem, error) {
			return nil, errDictNotFound
		}})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid limit", func(t *testing.T) {
		req := dictAPIRequest(http.MethodGet, "/iam/api/dicts/values?dict_code=org_type&as_of=2026-01-01&limit=bad", nil, true)
		rec := httptest.NewRecorder()
		handleDictValuesListAPI(rec, req, dictStoreStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid status", func(t *testing.T) {
		req := dictAPIRequest(http.MethodGet, "/iam/api/dicts/values?dict_code=org_type&as_of=2026-01-01&status=bad", nil, true)
		rec := httptest.NewRecorder()
		handleDictValuesListAPI(rec, req, dictStoreStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("store error", func(t *testing.T) {
		req := dictAPIRequest(http.MethodGet, "/iam/api/dicts/values?dict_code=org_type&as_of=2026-01-01", nil, true)
		rec := httptest.NewRecorder()
		handleDictValuesListAPI(rec, req, dictStoreStub{listDictValuesFn: func(context.Context, string, string, string, string, int, string) ([]DictValueItem, error) {
			return nil, errDictValueConflict
		}})
		if rec.Code != http.StatusConflict || responseCode(t, rec) != "dict_value_conflict" {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("success with limit clamp and status default", func(t *testing.T) {
		req := dictAPIRequest(http.MethodGet, "/iam/api/dicts/values?dict_code=org_type&as_of=2026-01-01&limit=999", nil, true)
		rec := httptest.NewRecorder()
		handleDictValuesListAPI(rec, req, dictStoreStub{listDictValuesFn: func(_ context.Context, _ string, dictCode string, asOf string, keyword string, limit int, status string) ([]DictValueItem, error) {
			if dictCode != "org_type" || asOf != "2026-01-01" || keyword != "" || limit != 50 || status != "all" {
				t.Fatalf("dictCode=%q asOf=%q keyword=%q limit=%d status=%q", dictCode, asOf, keyword, limit, status)
			}
			return []DictValueItem{{DictCode: dictCode, Code: "10", Label: "部门", Status: "active", EnabledOn: "2026-01-01", UpdatedAt: time.Unix(2, 0).UTC()}}, nil
		}})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestHandleDictValuesMutationsAndAudit_Coverage(t *testing.T) {
	t.Run("create validations and retry", func(t *testing.T) {
		cases := []struct {
			name   string
			body   string
			status int
			store  DictStore
		}{
			{name: "bad json", body: "{", status: http.StatusBadRequest},
			{name: "dict required", body: `{"dict_code":"","code":"10","label":"x","enabled_on":"2026-01-01","request_id":"r1"}`, status: http.StatusBadRequest},
			{name: "dict invalid", body: `{"dict_code":"bad-code","code":"10","label":"x","enabled_on":"2026-01-01","request_id":"r1"}`, status: http.StatusBadRequest},
			{name: "dict not found", body: `{"dict_code":"x","code":"10","label":"x","enabled_on":"2026-01-01","request_id":"r1"}`, status: http.StatusNotFound, store: dictStoreStub{createFn: func(context.Context, string, DictCreateValueRequest) (DictValueItem, bool, error) {
				return DictValueItem{}, false, errDictNotFound
			}}},
			{name: "code required", body: `{"dict_code":"org_type","code":"","label":"x","enabled_on":"2026-01-01","request_id":"r1"}`, status: http.StatusBadRequest},
			{name: "label required", body: `{"dict_code":"org_type","code":"10","label":"","enabled_on":"2026-01-01","request_id":"r1"}`, status: http.StatusBadRequest},
			{name: "date invalid", body: `{"dict_code":"org_type","code":"10","label":"x","enabled_on":"bad","request_id":"r1"}`, status: http.StatusBadRequest},
			{name: "request required", body: `{"dict_code":"org_type","code":"10","label":"x","enabled_on":"2026-01-01","request_id":""}`, status: http.StatusBadRequest},
			{name: "store error", body: `{"dict_code":"org_type","code":"10","label":"x","enabled_on":"2026-01-01","request_id":"r1"}`, status: http.StatusBadRequest, store: dictStoreStub{createFn: func(context.Context, string, DictCreateValueRequest) (DictValueItem, bool, error) {
				return DictValueItem{}, false, errDictRequestIDRequired
			}}},
			{name: "ok retry", body: `{"dict_code":"org_type","code":"10","label":"x","enabled_on":"2026-01-01","request_id":"r1"}`, status: http.StatusOK, store: dictStoreStub{createFn: func(context.Context, string, DictCreateValueRequest) (DictValueItem, bool, error) {
				return DictValueItem{DictCode: "org_type", Code: "10", Label: "x", Status: "active", EnabledOn: "2026-01-01", UpdatedAt: time.Unix(3, 0).UTC()}, true, nil
			}}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				store := tc.store
				if store == nil {
					store = dictStoreStub{}
				}
				req := dictAPIRequest(http.MethodPost, "/iam/api/dicts/values", []byte(tc.body), true)
				rec := httptest.NewRecorder()
				handleDictValuesCreateAPI(rec, req, store)
				if rec.Code != tc.status {
					t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
				}
			})
		}
	})

	t.Run("create tenant missing", func(t *testing.T) {
		req := dictAPIRequest(http.MethodPost, "/iam/api/dicts/values", []byte(`{"dict_code":"org_type"}`), false)
		rec := httptest.NewRecorder()
		handleDictValuesCreateAPI(rec, req, dictStoreStub{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("disable coverage", func(t *testing.T) {
		reqBadMethod := dictAPIRequest(http.MethodGet, "/iam/api/dicts/values:disable", nil, true)
		recBadMethod := httptest.NewRecorder()
		handleDictValuesDisableAPI(recBadMethod, reqBadMethod, dictStoreStub{})
		if recBadMethod.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", recBadMethod.Code)
		}

		reqBadJSON := dictAPIRequest(http.MethodPost, "/iam/api/dicts/values:disable", []byte("{"), true)
		recBadJSON := httptest.NewRecorder()
		handleDictValuesDisableAPI(recBadJSON, reqBadJSON, dictStoreStub{})
		if recBadJSON.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", recBadJSON.Code)
		}

		reqBad := dictAPIRequest(http.MethodPost, "/iam/api/dicts/values:disable", []byte(`{"dict_code":"org_type","code":"","disabled_on":"bad","request_id":""}`), true)
		recBad := httptest.NewRecorder()
		handleDictValuesDisableAPI(recBad, reqBad, dictStoreStub{})
		if recBad.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", recBad.Code)
		}

		reqInvalidDate := dictAPIRequest(http.MethodPost, "/iam/api/dicts/values:disable", []byte(`{"dict_code":"org_type","code":"10","disabled_on":"bad","request_id":"r1"}`), true)
		recInvalidDate := httptest.NewRecorder()
		handleDictValuesDisableAPI(recInvalidDate, reqInvalidDate, dictStoreStub{})
		if recInvalidDate.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", recInvalidDate.Code)
		}

		reqRequestRequired := dictAPIRequest(http.MethodPost, "/iam/api/dicts/values:disable", []byte(`{"dict_code":"org_type","code":"10","disabled_on":"2026-01-01","request_id":""}`), true)
		recRequestRequired := httptest.NewRecorder()
		handleDictValuesDisableAPI(recRequestRequired, reqRequestRequired, dictStoreStub{})
		if recRequestRequired.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", recRequestRequired.Code)
		}

		reqNoTenant := dictAPIRequest(http.MethodPost, "/iam/api/dicts/values:disable", []byte(`{"dict_code":"org_type","code":"10","disabled_on":"2026-01-01","request_id":"r1"}`), false)
		recNoTenant := httptest.NewRecorder()
		handleDictValuesDisableAPI(recNoTenant, reqNoTenant, dictStoreStub{})
		if recNoTenant.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", recNoTenant.Code)
		}

		reqDictRequired := dictAPIRequest(http.MethodPost, "/iam/api/dicts/values:disable", []byte(`{"dict_code":"","code":"10","disabled_on":"2026-01-01","request_id":"r1"}`), true)
		recDictRequired := httptest.NewRecorder()
		handleDictValuesDisableAPI(recDictRequired, reqDictRequired, dictStoreStub{})
		if recDictRequired.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", recDictRequired.Code)
		}

		reqDictNotFound := dictAPIRequest(http.MethodPost, "/iam/api/dicts/values:disable", []byte(`{"dict_code":"x","code":"10","disabled_on":"2026-01-01","request_id":"r1"}`), true)
		recDictNotFound := httptest.NewRecorder()
		handleDictValuesDisableAPI(recDictNotFound, reqDictNotFound, dictStoreStub{disableFn: func(context.Context, string, DictDisableValueRequest) (DictValueItem, bool, error) {
			return DictValueItem{}, false, errDictNotFound
		}})
		if recDictNotFound.Code != http.StatusNotFound {
			t.Fatalf("status=%d", recDictNotFound.Code)
		}

		reqErr := dictAPIRequest(http.MethodPost, "/iam/api/dicts/values:disable", []byte(`{"dict_code":"org_type","code":"10","disabled_on":"2026-01-01","request_id":"r1"}`), true)
		recErr := httptest.NewRecorder()
		handleDictValuesDisableAPI(recErr, reqErr, dictStoreStub{disableFn: func(context.Context, string, DictDisableValueRequest) (DictValueItem, bool, error) {
			return DictValueItem{}, false, errDictValueNotFoundAsOf
		}})
		if recErr.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", recErr.Code, recErr.Body.String())
		}

		reqOK := dictAPIRequest(http.MethodPost, "/iam/api/dicts/values:disable", []byte(`{"dict_code":"org_type","code":"10","disabled_on":"2026-01-01","request_id":"r1"}`), true)
		recOK := httptest.NewRecorder()
		handleDictValuesDisableAPI(recOK, reqOK, dictStoreStub{disableFn: func(context.Context, string, DictDisableValueRequest) (DictValueItem, bool, error) {
			return DictValueItem{DictCode: "org_type", Code: "10", Label: "部门", Status: "inactive", EnabledOn: "2026-01-01", UpdatedAt: time.Unix(4, 0).UTC()}, false, nil
		}})
		if recOK.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", recOK.Code, recOK.Body.String())
		}
	})

	t.Run("correct coverage", func(t *testing.T) {
		reqBadMethod := dictAPIRequest(http.MethodGet, "/iam/api/dicts/values:correct", nil, true)
		recBadMethod := httptest.NewRecorder()
		handleDictValuesCorrectAPI(recBadMethod, reqBadMethod, dictStoreStub{})
		if recBadMethod.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", recBadMethod.Code)
		}

		reqBadJSON := dictAPIRequest(http.MethodPost, "/iam/api/dicts/values:correct", []byte("{"), true)
		recBadJSON := httptest.NewRecorder()
		handleDictValuesCorrectAPI(recBadJSON, reqBadJSON, dictStoreStub{})
		if recBadJSON.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", recBadJSON.Code)
		}

		reqBad := dictAPIRequest(http.MethodPost, "/iam/api/dicts/values:correct", []byte(`{"dict_code":"org_type","code":"10","label":"","correction_day":"bad","request_id":""}`), true)
		recBad := httptest.NewRecorder()
		handleDictValuesCorrectAPI(recBad, reqBad, dictStoreStub{})
		if recBad.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", recBad.Code)
		}

		reqInvalidDate := dictAPIRequest(http.MethodPost, "/iam/api/dicts/values:correct", []byte(`{"dict_code":"org_type","code":"10","label":"X","correction_day":"bad","request_id":"r1"}`), true)
		recInvalidDate := httptest.NewRecorder()
		handleDictValuesCorrectAPI(recInvalidDate, reqInvalidDate, dictStoreStub{})
		if recInvalidDate.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", recInvalidDate.Code)
		}

		reqRequestRequired := dictAPIRequest(http.MethodPost, "/iam/api/dicts/values:correct", []byte(`{"dict_code":"org_type","code":"10","label":"X","correction_day":"2026-01-01","request_id":""}`), true)
		recRequestRequired := httptest.NewRecorder()
		handleDictValuesCorrectAPI(recRequestRequired, reqRequestRequired, dictStoreStub{})
		if recRequestRequired.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", recRequestRequired.Code)
		}

		reqNoTenant := dictAPIRequest(http.MethodPost, "/iam/api/dicts/values:correct", []byte(`{"dict_code":"org_type","code":"10","label":"X","correction_day":"2026-01-01","request_id":"r1"}`), false)
		recNoTenant := httptest.NewRecorder()
		handleDictValuesCorrectAPI(recNoTenant, reqNoTenant, dictStoreStub{})
		if recNoTenant.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", recNoTenant.Code)
		}

		reqDictRequired := dictAPIRequest(http.MethodPost, "/iam/api/dicts/values:correct", []byte(`{"dict_code":"","code":"10","label":"X","correction_day":"2026-01-01","request_id":"r1"}`), true)
		recDictRequired := httptest.NewRecorder()
		handleDictValuesCorrectAPI(recDictRequired, reqDictRequired, dictStoreStub{})
		if recDictRequired.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", recDictRequired.Code)
		}

		reqDictNotFound := dictAPIRequest(http.MethodPost, "/iam/api/dicts/values:correct", []byte(`{"dict_code":"x","code":"10","label":"X","correction_day":"2026-01-01","request_id":"r1"}`), true)
		recDictNotFound := httptest.NewRecorder()
		handleDictValuesCorrectAPI(recDictNotFound, reqDictNotFound, dictStoreStub{correctFn: func(context.Context, string, DictCorrectValueRequest) (DictValueItem, bool, error) {
			return DictValueItem{}, false, errDictNotFound
		}})
		if recDictNotFound.Code != http.StatusNotFound {
			t.Fatalf("status=%d", recDictNotFound.Code)
		}

		reqCodeRequired := dictAPIRequest(http.MethodPost, "/iam/api/dicts/values:correct", []byte(`{"dict_code":"org_type","code":"","label":"X","correction_day":"2026-01-01","request_id":"r1"}`), true)
		recCodeRequired := httptest.NewRecorder()
		handleDictValuesCorrectAPI(recCodeRequired, reqCodeRequired, dictStoreStub{})
		if recCodeRequired.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", recCodeRequired.Code)
		}

		reqErr := dictAPIRequest(http.MethodPost, "/iam/api/dicts/values:correct", []byte(`{"dict_code":"org_type","code":"10","label":"X","correction_day":"2026-01-01","request_id":"r1"}`), true)
		recErr := httptest.NewRecorder()
		handleDictValuesCorrectAPI(recErr, reqErr, dictStoreStub{correctFn: func(context.Context, string, DictCorrectValueRequest) (DictValueItem, bool, error) {
			return DictValueItem{}, false, errDictValueConflict
		}})
		if recErr.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s", recErr.Code, recErr.Body.String())
		}

		reqOK := dictAPIRequest(http.MethodPost, "/iam/api/dicts/values:correct", []byte(`{"dict_code":"org_type","code":"10","label":"X","correction_day":"2026-01-01","request_id":"r1"}`), true)
		recOK := httptest.NewRecorder()
		handleDictValuesCorrectAPI(recOK, reqOK, dictStoreStub{correctFn: func(context.Context, string, DictCorrectValueRequest) (DictValueItem, bool, error) {
			return DictValueItem{DictCode: "org_type", Code: "10", Label: "X", Status: "active", EnabledOn: "2026-01-01", UpdatedAt: time.Unix(5, 0).UTC()}, false, nil
		}})
		if recOK.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", recOK.Code, recOK.Body.String())
		}
	})

	t.Run("audit coverage", func(t *testing.T) {
		reqBadMethod := dictAPIRequest(http.MethodPost, "/iam/api/dicts/values/audit", nil, true)
		recBadMethod := httptest.NewRecorder()
		handleDictValuesAuditAPI(recBadMethod, reqBadMethod, dictStoreStub{})
		if recBadMethod.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", recBadMethod.Code)
		}

		reqBadMissing := dictAPIRequest(http.MethodGet, "/iam/api/dicts/values/audit?dict_code=&code=10", nil, true)
		recBadMissing := httptest.NewRecorder()
		handleDictValuesAuditAPI(recBadMissing, reqBadMissing, dictStoreStub{})
		if recBadMissing.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", recBadMissing.Code)
		}

		reqNoTenant := dictAPIRequest(http.MethodGet, "/iam/api/dicts/values/audit?dict_code=org_type&code=10", nil, false)
		recNoTenant := httptest.NewRecorder()
		handleDictValuesAuditAPI(recNoTenant, reqNoTenant, dictStoreStub{})
		if recNoTenant.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", recNoTenant.Code)
		}

		reqDictNotFound := dictAPIRequest(http.MethodGet, "/iam/api/dicts/values/audit?dict_code=x&code=10", nil, true)
		recDictNotFound := httptest.NewRecorder()
		handleDictValuesAuditAPI(recDictNotFound, reqDictNotFound, dictStoreStub{listAuditFn: func(context.Context, string, string, string, int) ([]DictValueAuditItem, error) {
			return nil, errDictNotFound
		}})
		if recDictNotFound.Code != http.StatusNotFound {
			t.Fatalf("status=%d", recDictNotFound.Code)
		}

		reqCodeRequired := dictAPIRequest(http.MethodGet, "/iam/api/dicts/values/audit?dict_code=org_type&code=", nil, true)
		recCodeRequired := httptest.NewRecorder()
		handleDictValuesAuditAPI(recCodeRequired, reqCodeRequired, dictStoreStub{})
		if recCodeRequired.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", recCodeRequired.Code)
		}

		reqBadLimit := dictAPIRequest(http.MethodGet, "/iam/api/dicts/values/audit?dict_code=org_type&code=10&limit=bad", nil, true)
		recBadLimit := httptest.NewRecorder()
		handleDictValuesAuditAPI(recBadLimit, reqBadLimit, dictStoreStub{})
		if recBadLimit.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", recBadLimit.Code)
		}

		reqErr := dictAPIRequest(http.MethodGet, "/iam/api/dicts/values/audit?dict_code=org_type&code=10", nil, true)
		recErr := httptest.NewRecorder()
		handleDictValuesAuditAPI(recErr, reqErr, dictStoreStub{listAuditFn: func(context.Context, string, string, string, int) ([]DictValueAuditItem, error) {
			return nil, errors.New("DICT_EFFECTIVE_DAY_REQUIRED")
		}})
		if recErr.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", recErr.Code, recErr.Body.String())
		}

		reqOK := dictAPIRequest(http.MethodGet, "/iam/api/dicts/values/audit?dict_code=org_type&code=10&limit=999", nil, true)
		recOK := httptest.NewRecorder()
		handleDictValuesAuditAPI(recOK, reqOK, dictStoreStub{listAuditFn: func(_ context.Context, _ string, dictCode string, code string, limit int) ([]DictValueAuditItem, error) {
			if dictCode != "org_type" || code != "10" || limit != 200 {
				t.Fatalf("dictCode=%q code=%q limit=%d", dictCode, code, limit)
			}
			return []DictValueAuditItem{{EventID: 1, DictCode: "org_type", Code: "10", EventType: dictEventCreated, EffectiveDay: "2026-01-01", RequestID: "r1", InitiatorUUID: "u1", TxTime: time.Unix(6, 0).UTC(), Payload: json.RawMessage(`{"label":"部门"}`)}}, nil
		}})
		if recOK.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", recOK.Code, recOK.Body.String())
		}
	})
}

func TestDictAPIHelpers_Coverage(t *testing.T) {
	t.Run("requiredAsOf and isDate", func(t *testing.T) {
		reqBad := httptest.NewRequest(http.MethodGet, "/x?as_of=bad", nil)
		if _, ok := requiredAsOf(reqBad); ok {
			t.Fatal("expected invalid")
		}
		reqOK := httptest.NewRequest(http.MethodGet, "/x?as_of=2026-01-01", nil)
		if got, ok := requiredAsOf(reqOK); !ok || got != "2026-01-01" {
			t.Fatalf("got=%q ok=%v", got, ok)
		}
		if isDate("") || isDate("bad") || !isDate("2026-01-01") {
			t.Fatal("isDate unexpected")
		}
	})

	t.Run("writeDictAPIError status mapping", func(t *testing.T) {
		cases := []struct {
			err    error
			status int
			code   string
		}{
			{err: errDictCodeRequired, status: http.StatusBadRequest, code: "dict_code_required"},
			{err: errDictNotFound, status: http.StatusNotFound, code: "dict_not_found"},
			{err: errDictValueConflict, status: http.StatusConflict, code: "dict_value_conflict"},
			{err: errDictBaselineNotReady, status: http.StatusConflict, code: "dict_baseline_not_ready"},
			{err: errDictReleaseIDRequired, status: http.StatusBadRequest, code: "dict_release_id_required"},
			{err: errDictReleasePayloadInvalid, status: http.StatusConflict, code: "dict_release_payload_invalid"},
			{err: errors.New("DICT_REQUEST_CODE_REQUIRED"), status: http.StatusBadRequest, code: "invalid_request"},
			{err: errors.New("totally bad-code"), status: http.StatusInternalServerError, code: "internal_error"},
		}
		for _, tc := range cases {
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			rec := httptest.NewRecorder()
			writeDictAPIError(rec, req, tc.err, "fallback")
			if rec.Code != tc.status || responseCode(t, rec) != tc.code {
				t.Fatalf("status=%d code=%q body=%s", rec.Code, responseCode(t, rec), rec.Body.String())
			}
		}
	})

	t.Run("dictErrorCode and defaultStableCode", func(t *testing.T) {
		if got := dictErrorCode(errDictValueLabelRequired); got != "dict_value_label_required" {
			t.Fatalf("got=%q", got)
		}
		if got := dictErrorCode(errDictValueCodeRequired); got != "dict_value_code_required" {
			t.Fatalf("got=%q", got)
		}
		if got := dictErrorCode(errors.New("DICT_EFFECTIVE_DAY_REQUIRED")); got != "invalid_as_of" {
			t.Fatalf("got=%q", got)
		}
		if got := dictErrorCode(errors.New("DICT_REQUEST_CODE_REQUIRED")); got != "invalid_request" {
			t.Fatalf("got=%q", got)
		}
		if got := dictErrorCode(errors.New("DICT_REQUEST_ID_REQUIRED")); got != "invalid_request" {
			t.Fatalf("got=%q", got)
		}
		if got := dictErrorCode(errors.New("DICT_NOT_FOUND")); got != "dict_not_found" {
			t.Fatalf("got=%q", got)
		}
		if got := dictErrorCode(errors.New("DICT_BASELINE_NOT_READY")); got != "dict_baseline_not_ready" {
			t.Fatalf("got=%q", got)
		}
		if got := dictErrorCode(errDictReleaseIDRequired); got != "dict_release_id_required" {
			t.Fatalf("got=%q", got)
		}
		if got := dictErrorCode(errDictReleaseSourceInvalid); got != "dict_release_source_invalid" {
			t.Fatalf("got=%q", got)
		}
		if got := dictErrorCode(errDictReleaseTargetRequired); got != "dict_release_target_required" {
			t.Fatalf("got=%q", got)
		}
		if got := dictErrorCode(errDictReleasePayloadInvalid); got != "dict_release_payload_invalid" {
			t.Fatalf("got=%q", got)
		}
		if got := dictErrorCode(errors.New("DICT_RELEASE_SOURCE_INVALID")); got != "dict_release_source_invalid" {
			t.Fatalf("got=%q", got)
		}
		if got := defaultStableCode("", "fb"); got != "fb" {
			t.Fatalf("got=%q", got)
		}
		if got := defaultStableCode("unknown", "fb"); got != "fb" {
			t.Fatalf("got=%q", got)
		}
		if got := defaultStableCode("bad-code", "fb"); got != "fb" {
			t.Fatalf("got=%q", got)
		}
		if got := defaultStableCode("my_code", "fb"); got != "my_code" {
			t.Fatalf("got=%q", got)
		}
	})
}
