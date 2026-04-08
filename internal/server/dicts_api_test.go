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

	"github.com/jackc/pgx/v5"
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
		if _, err := requiredAsOf(reqBad); err == nil {
			t.Fatal("expected invalid")
		}
		reqOK := httptest.NewRequest(http.MethodGet, "/x?as_of=2026-01-01", nil)
		if got, err := requiredAsOf(reqOK); err != nil || got != "2026-01-01" {
			t.Fatalf("got=%q err=%v", got, err)
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

func TestDictPGStore_ExtraCoverage(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateDict and DisableDict wrappers", func(t *testing.T) {
		createTx := &stubTx{}
		createTx.row = &stubRow{vals: []any{int64(1), false}}
		createTx.row2 = &stubRow{vals: []any{[]byte(`{"dict_code":"expense_type","name":"Expense Type","status":"active","enabled_on":"2026-01-01"}`)}}
		storeCreate := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return createTx, nil })}
		item, wasRetry, err := storeCreate.CreateDict(ctx, "t1", DictCreateRequest{DictCode: "expense_type", Name: "Expense Type", EnabledOn: "2026-01-01", RequestID: "r1", Initiator: "u1"})
		if err != nil || wasRetry || item.DictCode != "expense_type" {
			t.Fatalf("item=%+v retry=%v err=%v", item, wasRetry, err)
		}

		disableTx := &stubTx{}
		disableTx.row = &stubRow{vals: []any{int64(2), true}}
		disableTx.row2 = &stubRow{vals: []any{[]byte(`{"dict_code":"expense_type","name":"Expense Type","status":"inactive","enabled_on":"2026-01-01","disabled_on":"2026-01-02"}`)}}
		storeDisable := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return disableTx, nil })}
		disabled, disableRetry, err := storeDisable.DisableDict(ctx, "t1", DictDisableRequest{DictCode: "expense_type", DisabledOn: "2026-01-02", RequestID: "r2", Initiator: "u1"})
		if err != nil || !disableRetry || disabled.Status != "inactive" || disabled.DisabledOn == nil {
			t.Fatalf("disabled=%+v retry=%v err=%v", disabled, disableRetry, err)
		}
	})

	t.Run("submitDictEvent error branches", func(t *testing.T) {
		storeBegin := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})}
		if _, _, err := storeBegin.submitDictEvent(ctx, "t1", "x", dictRegistryEventCreated, "2026-01-01", map[string]any{"name": "X"}, "r1", "u1"); err == nil {
			t.Fatal("expected begin error")
		}

		storeExec := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		})}
		if _, _, err := storeExec.submitDictEvent(ctx, "t1", "x", dictRegistryEventCreated, "2026-01-01", map[string]any{"name": "X"}, "r1", "u1"); err == nil {
			t.Fatal("expected exec error")
		}

		store := &dictPGStore{pool: &fakeBeginner{}}
		if _, _, err := store.submitDictEvent(ctx, "t1", "x", dictRegistryEventCreated, "2026-01-01", map[string]any{"x": func() {}}, "r1", "u1"); err == nil {
			t.Fatal("expected marshal error")
		}

		queryErrTx := &stubTx{rowErr: errors.New("row")}
		storeQueryErr := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return queryErrTx, nil })}
		if _, _, err := storeQueryErr.submitDictEvent(ctx, "t1", "x", dictRegistryEventCreated, "2026-01-01", map[string]any{"name": "X"}, "r1", "u1"); err == nil {
			t.Fatal("expected query error")
		}

		badSnapshotTx := &stubTx{}
		badSnapshotTx.row = &stubRow{vals: []any{int64(1), false}}
		badSnapshotTx.row2 = &stubRow{vals: []any{[]byte(`{`)}}
		storeBadSnapshot := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return badSnapshotTx, nil })}
		if _, _, err := storeBadSnapshot.submitDictEvent(ctx, "t1", "x", dictRegistryEventCreated, "2026-01-01", map[string]any{"name": "X"}, "r1", "u1"); err == nil {
			t.Fatal("expected bad snapshot error")
		}

		commitErrTx := &stubTx{commitErr: errors.New("commit")}
		commitErrTx.row = &stubRow{vals: []any{int64(1), false}}
		commitErrTx.row2 = &stubRow{vals: []any{[]byte(`{"dict_code":"x","name":"X","status":"active","enabled_on":"2026-01-01"}`)}}
		storeCommitErr := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return commitErrTx, nil })}
		if _, _, err := storeCommitErr.submitDictEvent(ctx, "t1", "x", dictRegistryEventCreated, "2026-01-01", map[string]any{"name": "X"}, "r1", "u1"); err == nil {
			t.Fatal("expected commit error")
		}
	})

	t.Run("resolve source helpers", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{"t1"}}
		if source, err := resolveDictSourceTenantTx(ctx, tx, "t1", "org_type"); err != nil || source != "t1" {
			t.Fatalf("source=%q err=%v", source, err)
		}

		txNoRows := &stubTx{}
		txNoRows.row = &stubRow{err: pgx.ErrNoRows}
		if _, err := resolveDictSourceTenantTx(ctx, txNoRows, "t1", "missing"); !errors.Is(err, errDictNotFound) {
			t.Fatalf("err=%v", err)
		}

		txErr := &stubTx{}
		txErr.row = &stubRow{err: errors.New("boom")}
		if _, err := resolveDictSourceTenantTx(ctx, txErr, "t1", "org_type"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("resolve label helper branches", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{"部门"}}
		if label, ok, err := resolveValueLabelByTenant(ctx, tx, "t1", "2026-01-01", "org_type", "10"); err != nil || !ok || label != "部门" {
			t.Fatalf("label=%q ok=%v err=%v", label, ok, err)
		}

		txNoRows := &stubTx{}
		txNoRows.row = &stubRow{err: pgx.ErrNoRows}
		if _, ok, err := resolveValueLabelByTenant(ctx, txNoRows, "t1", "2026-01-01", "org_type", "10"); err != nil || ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}

		txErr := &stubTx{}
		txErr.row = &stubRow{err: errors.New("boom")}
		if _, _, err := resolveValueLabelByTenant(ctx, txErr, "t1", "2026-01-01", "org_type", "10"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("assertTenantDictActiveAsOfTx second query error", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{false}}
		tx.row2Err = errors.New("boom")
		if err := assertTenantDictActiveAsOfTx(ctx, tx, "t1", "org_type", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submitValueEvent query branch error", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{true}}
		tx.row2Err = errors.New("boom")
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.submitValueEvent(ctx, "t1", "org_type", "10", dictEventCreated, "2026-01-01", map[string]any{"label": "部门"}, "r1", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submitValueEvent payload marshal error", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{true}}
		tx.row2 = &stubRow{vals: []any{true}}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.submitValueEvent(ctx, "t1", "org_type", "10", dictEventCreated, "2026-01-01", map[string]any{"x": func() {}}, "r1", "u1"); err == nil {
			t.Fatal("expected marshal error")
		}
	})

	t.Run("submitValueEvent active check query error", func(t *testing.T) {
		tx := &stubTx{rowErr: errors.New("boom")}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.submitValueEvent(ctx, "t1", "org_type", "10", dictEventCreated, "2026-01-01", map[string]any{"label": "部门"}, "r1", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("getDictFromEventTx query error", func(t *testing.T) {
		tx := &stubTx{rowErr: errors.New("boom")}
		if _, err := getDictFromEventTx(ctx, tx, "t1", 1); err == nil {
			t.Fatal("expected query error")
		}
	})

	t.Run("ListDictValueAudit source not found", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{err: pgx.ErrNoRows}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListDictValueAudit(ctx, "t1", "missing", "10", 10); !errors.Is(err, errDictNotFound) {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestDictMemoryStore_ExtraCoverage(t *testing.T) {
	ctx := context.Background()
	store := newDictMemoryStore().(*dictMemoryStore)

	t.Run("ListDicts coverage branches (tenant/global merge + inactive skip + sort)", func(t *testing.T) {
		tenantID := "00000000-0000-0000-0000-000000000001"

		// tenant-only active dict
		if _, _, err := store.CreateDict(ctx, tenantID, DictCreateRequest{DictCode: "expense_type", Name: "Expense Type", EnabledOn: "2026-01-01"}); err != nil {
			t.Fatalf("CreateDict err=%v", err)
		}
		// tenant-only inactive (future) dict to hit the "continue" branch
		if _, _, err := store.CreateDict(ctx, tenantID, DictCreateRequest{DictCode: "future_dict", Name: "Future Dict", EnabledOn: "2099-01-01"}); err != nil {
			t.Fatalf("CreateDict err=%v", err)
		}
		// global-only dict to hit the global loop append path
		store.dicts[globalTenantID]["global_only"] = DictItem{DictCode: "global_only", Name: "Global Only", Status: "active", EnabledOn: "1970-01-01"}

		items, err := store.ListDicts(ctx, tenantID, "2026-01-01")
		if err != nil {
			t.Fatalf("ListDicts err=%v", err)
		}
		// At least 2 items to exercise sort comparator.
		if len(items) < 2 {
			t.Fatalf("expected >=2 items; got=%d", len(items))
		}
	})

	if _, _, err := store.CreateDict(ctx, "t1", DictCreateRequest{DictCode: "", Name: "X", EnabledOn: "2026-01-01"}); !errors.Is(err, errDictCodeRequired) {
		t.Fatalf("err=%v", err)
	}
	if _, _, err := store.CreateDict(ctx, "t1", DictCreateRequest{DictCode: "x", Name: "", EnabledOn: "2026-01-01"}); !errors.Is(err, errDictNameRequired) {
		t.Fatalf("err=%v", err)
	}
	if _, _, err := store.CreateDict(ctx, "t1", DictCreateRequest{DictCode: "x", Name: "X", EnabledOn: ""}); !errors.Is(err, errDictEffectiveDayRequired) {
		t.Fatalf("err=%v", err)
	}

	if _, _, err := store.DisableDict(ctx, "t1", DictDisableRequest{DictCode: "", DisabledOn: "2026-01-01"}); !errors.Is(err, errDictCodeRequired) {
		t.Fatalf("err=%v", err)
	}
	if _, _, err := store.DisableDict(ctx, "t1", DictDisableRequest{DictCode: "unknown", DisabledOn: "2026-01-01"}); !errors.Is(err, errDictNotFound) {
		t.Fatalf("err=%v", err)
	}
	if _, _, err := store.DisableDict(ctx, "t1", DictDisableRequest{DictCode: "org_type", DisabledOn: ""}); !errors.Is(err, errDictDisabledOnRequired) {
		t.Fatalf("err=%v", err)
	}

	created, _, err := store.CreateDict(ctx, "t1", DictCreateRequest{DictCode: "expense_type", Name: "Expense Type", EnabledOn: "2026-01-01"})
	if err != nil || created.Status != "active" {
		t.Fatalf("created=%+v err=%v", created, err)
	}
	if got := dictActiveAsOf(created, "2025-01-01"); got {
		t.Fatal("expected inactive before enabled_on")
	}
	if _, _, err := store.DisableDict(ctx, "t1", DictDisableRequest{DictCode: "expense_type", DisabledOn: "2026-01-01"}); !errors.Is(err, errDictCodeConflict) {
		t.Fatalf("err=%v", err)
	}

	if _, err := store.ListDictValueAudit(ctx, "t1", "missing", "10", 10); !errors.Is(err, errDictNotFound) {
		t.Fatalf("err=%v", err)
	}

	if _, ok := store.resolveSourceTenant("t1", "unknown"); ok {
		t.Fatal("expected miss")
	}

	t.Run("DisableDict item not found branch", func(t *testing.T) {
		tenantID := "00000000-0000-0000-0000-000000000001"
		if _, _, err := store.DisableDict(ctx, tenantID, DictDisableRequest{DictCode: "missing_dict", DisabledOn: "2026-01-02"}); !errors.Is(err, errDictNotFound) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("DisableDict already disabled conflict branch", func(t *testing.T) {
		tenantID := "00000000-0000-0000-0000-000000000001"
		// Create + disable once, then disable again with a later day to hit the DisabledOn conflict check.
		if _, _, err := store.CreateDict(ctx, tenantID, DictCreateRequest{DictCode: "tmp_dict", Name: "Tmp Dict", EnabledOn: "2026-01-01"}); err != nil {
			t.Fatalf("CreateDict err=%v", err)
		}
		if _, _, err := store.DisableDict(ctx, tenantID, DictDisableRequest{DictCode: "tmp_dict", DisabledOn: "2026-01-03"}); err != nil {
			t.Fatalf("DisableDict err=%v", err)
		}
		if _, _, err := store.DisableDict(ctx, tenantID, DictDisableRequest{DictCode: "tmp_dict", DisabledOn: "2026-01-04"}); !errors.Is(err, errDictCodeConflict) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("ListDictValues status default + dict mismatch + status filter + same-code sort branch", func(t *testing.T) {
		tenantID := "00000000-0000-0000-0000-000000000001"
		now := time.Unix(0, 0).UTC()

		// Add an unrelated dict_code value to hit the "item.DictCode != dictCode" continue branch.
		store.values[tenantID] = append(store.values[tenantID], DictValueItem{DictCode: "other", Code: "x", Label: "X", EnabledOn: "1970-01-01", UpdatedAt: now})

		// Add two segments with the same code so the sort's "same code" branch is exercised.
		store.values[tenantID] = append(store.values[tenantID],
			DictValueItem{DictCode: dictCodeOrgType, Code: "30", Label: "中心", EnabledOn: "1970-01-01", UpdatedAt: now},
			DictValueItem{DictCode: dictCodeOrgType, Code: "30", Label: "中心(旧)", EnabledOn: "1960-01-01", UpdatedAt: now},
		)

		// Add an inactive value for status filtering.
		store.values[tenantID] = append(store.values[tenantID], DictValueItem{DictCode: dictCodeOrgType, Code: "40", Label: "未来", EnabledOn: "2099-01-01", UpdatedAt: now})

		// status empty => default to "all"
		if _, err := store.ListDictValues(ctx, tenantID, dictCodeOrgType, "2026-01-01", "", 10, ""); err != nil {
			t.Fatalf("ListDictValues err=%v", err)
		}
		// status filter branch
		if _, err := store.ListDictValues(ctx, tenantID, dictCodeOrgType, "2026-01-01", "", 10, "active"); err != nil {
			t.Fatalf("ListDictValues err=%v", err)
		}
	})

	t.Run("ResolveValueLabel dict not found branch", func(t *testing.T) {
		label, ok, err := store.ResolveValueLabel(ctx, "t1", "2026-01-01", "missing", "10")
		if err != nil || ok || label != "" {
			t.Fatalf("label=%q ok=%v err=%v", label, ok, err)
		}
	})

	t.Run("ListOptions propagate ListDictValues error", func(t *testing.T) {
		if _, err := store.ListOptions(ctx, "t1", "2026-01-01", "missing", "", 10); !errors.Is(err, errDictNotFound) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("ListDictValueAudit empty code branch", func(t *testing.T) {
		tenantID := "00000000-0000-0000-0000-000000000001"
		events, err := store.ListDictValueAudit(ctx, tenantID, dictCodeOrgType, "", 10)
		if err != nil || events == nil || len(events) != 0 {
			t.Fatalf("events=%v err=%v", events, err)
		}
	})

	inactive := DictItem{DictCode: "x", Name: "X", EnabledOn: "2026-01-01", DisabledOn: new("2026-01-02")}
	if dictActiveAsOf(inactive, "2026-01-02") {
		t.Fatal("expected inactive")
	}
	if dictStatusAsOf(inactive, "2026-01-02") != "inactive" {
		t.Fatal("expected inactive status")
	}
	value := DictValueItem{DictCode: "x", Code: "1", EnabledOn: "2026-01-01", DisabledOn: new("2026-01-02")}
	if valueStatusAsOf(value, "2026-01-02") != "inactive" {
		t.Fatal("expected inactive value status")
	}
	if dictDisplayName(dictCodeOrgType) != "Org Type" {
		t.Fatal("expected org type display")
	}
	if dictDisplayName(" expense_type ") != " expense_type " {
		t.Fatal("expected default display name passthrough")
	}
}

func TestDictAPI_ExtraCoverage(t *testing.T) {
	t.Run("create dict created status", func(t *testing.T) {
		req := dictAPIRequest(http.MethodPost, "/iam/api/dicts", []byte(`{"dict_code":"expense_type","name":"Expense Type","enabled_on":"2026-01-01","request_id":"r1"}`), true)
		rec := httptest.NewRecorder()
		handleDictsAPI(rec, req, dictStoreStub{createDictFn: func(context.Context, string, DictCreateRequest) (DictItem, bool, error) {
			return DictItem{DictCode: "expense_type", Name: "Expense Type", Status: "active", EnabledOn: "2026-01-01"}, false, nil
		}})
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid dict code branches", func(t *testing.T) {
		cases := []struct {
			target string
			h      func(http.ResponseWriter, *http.Request, DictStore)
		}{
			{target: "/iam/api/dicts:disable", h: handleDictsDisableAPI},
			{target: "/iam/api/dicts/values:disable", h: handleDictValuesDisableAPI},
			{target: "/iam/api/dicts/values:correct", h: handleDictValuesCorrectAPI},
		}
		for _, tc := range cases {
			req := dictAPIRequest(http.MethodPost, tc.target, []byte(`{"dict_code":"bad-code","code":"10","label":"X","disabled_on":"2026-01-01","correction_day":"2026-01-01","request_id":"r1"}`), true)
			rec := httptest.NewRecorder()
			tc.h(rec, req, dictStoreStub{})
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("target=%s status=%d", tc.target, rec.Code)
			}
		}

		reqCreateInvalid := dictAPIRequest(http.MethodPost, "/iam/api/dicts", []byte(`{"dict_code":"bad-code","name":"X","enabled_on":"2026-01-01","request_id":"r1"}`), true)
		recCreateInvalid := httptest.NewRecorder()
		handleDictsAPI(recCreateInvalid, reqCreateInvalid, dictStoreStub{})
		if recCreateInvalid.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", recCreateInvalid.Code)
		}

		reqAudit := dictAPIRequest(http.MethodGet, "/iam/api/dicts/values/audit?dict_code=bad-code&code=10", nil, true)
		recAudit := httptest.NewRecorder()
		handleDictValuesAuditAPI(recAudit, reqAudit, dictStoreStub{})
		if recAudit.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", recAudit.Code)
		}
	})

	t.Run("dictErrorCode extra mappings", func(t *testing.T) {
		cases := map[error]string{
			errDictCodeInvalid:           "dict_code_invalid",
			errDictNameRequired:          "dict_name_required",
			errDictCodeConflict:          "dict_code_conflict",
			errDictDisabled:              "dict_disabled",
			errDictDisabledOnRequired:    "dict_disabled_on_required",
			errDictDisabledOnInvalidDate: "dict_disabled_on_required",
			errDictValueDictDisabled:     "dict_value_dict_disabled",
		}
		for in, want := range cases {
			if got := dictErrorCode(in); got != want {
				t.Fatalf("want=%q got=%q", want, got)
			}
		}
		if got := dictErrorCode(errDictEffectiveDayRequired); got != "invalid_as_of" {
			t.Fatalf("got=%q", got)
		}
		if got := dictErrorCode(errors.New("DICT_DISABLED_ON_REQUIRED")); got != "dict_disabled_on_required" {
			t.Fatalf("got=%q", got)
		}
		if got := dictErrorCode(errors.New("DICT_ENABLED_ON_REQUIRED")); got != "dict_enabled_on_required" {
			t.Fatalf("got=%q", got)
		}
	})
}
