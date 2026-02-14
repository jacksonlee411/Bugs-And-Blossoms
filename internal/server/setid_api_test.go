package server

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

func TestHandleSetIDsAPI_TenantMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/setids", nil)
	rec := httptest.NewRecorder()
	handleSetIDsAPI(rec, req, newSetIDMemoryStore())
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetIDsAPI_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/org/api/setids", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDsAPI(rec, req, newSetIDMemoryStore())
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetIDsAPI_Get_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/setids", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDsAPI(rec, req, newSetIDMemoryStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "DEFLT") {
		t.Fatalf("unexpected body: %q", body)
	}
	if !strings.Contains(body, "SHARE") {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleSetIDsAPI_Get_EnsureBootstrapError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/setids", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDsAPI(rec, req, errSetIDStore{err: errBoom{}})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestHandleSetIDsAPI_Get_ListError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/setids", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDsAPI(rec, req, partialSetIDStore{listSetErr: errors.New("boom")})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestHandleSetIDsAPI_BadJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/setids", strings.NewReader("{"))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDsAPI(rec, req, newSetIDMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetIDsAPI_InvalidRequest(t *testing.T) {
	body := bytes.NewBufferString(`{"setid":"","name":"","request_code":""}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/setids", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDsAPI(rec, req, newSetIDMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetIDsAPI_InvalidEffectiveDate(t *testing.T) {
	body := bytes.NewBufferString(`{"setid":"A0001","name":"A","effective_date":"bad","request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/setids", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDsAPI(rec, req, newSetIDMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid_effective_date") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleSetIDsAPI_EnsureBootstrapError(t *testing.T) {
	body := bytes.NewBufferString(`{"setid":"A0001","name":"A","request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/setids", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDsAPI(rec, req, errSetIDStore{err: errBoom{}})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetIDsAPI_CreateSetIDError(t *testing.T) {
	body := bytes.NewBufferString(`{"setid":"A0001","name":"A","request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/setids", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDsAPI(rec, req, partialSetIDStore{createSetErr: errors.New("SETID_ALREADY_EXISTS")})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetIDsAPI_Success(t *testing.T) {
	body := bytes.NewBufferString(`{"setid":"A0001","name":"A","request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/setids", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDsAPI(rec, req, newSetIDMemoryStore())
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "A0001") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleSetIDBindingsAPI_BadInputs(t *testing.T) {
	badTenant := httptest.NewRequest(http.MethodPost, "/org/api/setid-bindings", nil)
	badTenantRec := httptest.NewRecorder()
	handleSetIDBindingsAPI(badTenantRec, badTenant, newSetIDMemoryStore(), newOrgUnitMemoryStore())
	if badTenantRec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", badTenantRec.Code)
	}

	req := httptest.NewRequest(http.MethodPut, "/org/api/setid-bindings", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDBindingsAPI(rec, req, newSetIDMemoryStore(), newOrgUnitMemoryStore())
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}

	badAsOf := httptest.NewRequest(http.MethodGet, "/org/api/setid-bindings?as_of=bad", nil)
	badAsOf = badAsOf.WithContext(withTenant(badAsOf.Context(), Tenant{ID: "t1", Name: "T"}))
	badAsOfRec := httptest.NewRecorder()
	handleSetIDBindingsAPI(badAsOfRec, badAsOf, newSetIDMemoryStore(), newOrgUnitMemoryStore())
	if badAsOfRec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", badAsOfRec.Code)
	}

	badJSON := httptest.NewRequest(http.MethodPost, "/org/api/setid-bindings", strings.NewReader("{"))
	badJSON = badJSON.WithContext(withTenant(badJSON.Context(), Tenant{ID: "t1", Name: "T"}))
	badJSONRec := httptest.NewRecorder()
	handleSetIDBindingsAPI(badJSONRec, badJSON, newSetIDMemoryStore(), newOrgUnitMemoryStore())
	if badJSONRec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", badJSONRec.Code)
	}

	missing := httptest.NewRequest(http.MethodPost, "/org/api/setid-bindings", bytes.NewBufferString(`{"org_code":""}`))
	missing = missing.WithContext(withTenant(missing.Context(), Tenant{ID: "t1", Name: "T"}))
	missingRec := httptest.NewRecorder()
	handleSetIDBindingsAPI(missingRec, missing, newSetIDMemoryStore(), newOrgUnitMemoryStore())
	if missingRec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", missingRec.Code)
	}

	badDate := httptest.NewRequest(http.MethodPost, "/org/api/setid-bindings", bytes.NewBufferString(`{"org_code":"A001","setid":"A0001","effective_date":"bad","request_code":"r1"}`))
	badDate = badDate.WithContext(withTenant(badDate.Context(), Tenant{ID: "t1", Name: "T"}))
	badDateRec := httptest.NewRecorder()
	handleSetIDBindingsAPI(badDateRec, badDate, newSetIDMemoryStore(), newOrgUnitMemoryStore())
	if badDateRec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", badDateRec.Code)
	}
}

type setidBindingsNilStore struct{ partialSetIDStore }

func (setidBindingsNilStore) ListSetIDBindings(context.Context, string, string) ([]SetIDBindingRow, error) {
	return nil, nil
}

func TestHandleSetIDBindingsAPI_Get_DefaultAsOf_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/setid-bindings", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDBindingsAPI(rec, req, partialSetIDStore{}, newOrgUnitMemoryStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"tenant_id":"t1"`) {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleSetIDBindingsAPI_Get_StoreError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/setid-bindings?as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDBindingsAPI(rec, req, partialSetIDStore{listBindErr: errors.New("boom")}, newOrgUnitMemoryStore())
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestHandleSetIDBindingsAPI_Get_NilRows(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/setid-bindings?as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDBindingsAPI(rec, req, setidBindingsNilStore{}, newOrgUnitMemoryStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"bindings":[]`) {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleSetIDBindingsAPI_OrgUnitIDNotAllowed(t *testing.T) {
	body := bytes.NewBufferString(`{"org_unit_id":"10000001","org_code":"A001","setid":"A0001","effective_date":"2026-01-01","request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/setid-bindings", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDBindingsAPI(rec, req, newSetIDMemoryStore(), newOrgUnitMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestHandleSetIDBindingsAPI_OrgCodeInvalid(t *testing.T) {
	body := bytes.NewBufferString(`{"org_code":"bad\u007f","setid":"A0001","effective_date":"2026-01-01","request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/setid-bindings", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDBindingsAPI(rec, req, newSetIDMemoryStore(), newOrgUnitMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "org_code_invalid") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleSetIDBindingsAPI_OrgCodeNotFound(t *testing.T) {
	body := bytes.NewBufferString(`{"org_code":"A001","setid":"A0001","effective_date":"2026-01-01","request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/setid-bindings", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDBindingsAPI(rec, req, newSetIDMemoryStore(), newOrgUnitMemoryStore())
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "org_code_not_found") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleSetIDBindingsAPI_OrgCodeResolveInvalid(t *testing.T) {
	body := bytes.NewBufferString(`{"org_code":"A001","setid":"A0001","effective_date":"2026-01-01","request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/setid-bindings", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDBindingsAPI(rec, req, newSetIDMemoryStore(), errOrgUnitStore{err: orgunitpkg.ErrOrgCodeInvalid})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "org_code_invalid") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleSetIDBindingsAPI_OrgCodeResolveError(t *testing.T) {
	body := bytes.NewBufferString(`{"org_code":"A001","setid":"A0001","effective_date":"2026-01-01","request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/setid-bindings", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDBindingsAPI(rec, req, newSetIDMemoryStore(), errOrgUnitStore{err: errBoom{}})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetIDBindingsAPI_StoreError(t *testing.T) {
	orgStore := newOrgUnitMemoryStore()
	_, _ = orgStore.CreateNodeCurrent(context.Background(), "t1", "2026-01-01", "A001", "Org", "", true)
	body := bytes.NewBufferString(`{"org_code":"A001","setid":"A0001","effective_date":"2026-01-01","request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/setid-bindings", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDBindingsAPI(rec, req, errSetIDStore{err: errBoom{}}, orgStore)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetIDBindingsAPI_Success(t *testing.T) {
	store := newSetIDMemoryStore().(*setidMemoryStore)
	if err := store.EnsureBootstrap(context.Background(), "t1", "t1"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := store.CreateSetID(context.Background(), "t1", "A0001", "A", "2026-01-01", "r1", "t1"); err != nil {
		t.Fatalf("err=%v", err)
	}

	orgStore := newOrgUnitMemoryStore()
	_, _ = orgStore.CreateNodeCurrent(context.Background(), "t1", "2026-01-01", "A001", "Org", "", true)
	body := bytes.NewBufferString(`{"org_code":"A001","setid":"A0001","effective_date":"2026-01-01","request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/setid-bindings", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDBindingsAPI(rec, req, store, orgStore)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "A0001") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleGlobalSetIDsAPI_BadInputs(t *testing.T) {
	badMethod := httptest.NewRequest(http.MethodPut, "/org/api/global-setids", nil)
	badMethodRec := httptest.NewRecorder()
	handleGlobalSetIDsAPI(badMethodRec, badMethod, newSetIDMemoryStore())
	if badMethodRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", badMethodRec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/org/api/global-setids", nil)
	rec := httptest.NewRecorder()
	handleGlobalSetIDsAPI(rec, req, newSetIDMemoryStore())
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}

	okGet := httptest.NewRequest(http.MethodGet, "/org/api/global-setids", nil)
	okGet = okGet.WithContext(withTenant(okGet.Context(), Tenant{ID: "t1", Name: "T"}))
	okGetRec := httptest.NewRecorder()
	handleGlobalSetIDsAPI(okGetRec, okGet, newSetIDMemoryStore())
	if okGetRec.Code != http.StatusOK {
		t.Fatalf("status=%d", okGetRec.Code)
	}

	getErr := httptest.NewRequest(http.MethodGet, "/org/api/global-setids", nil)
	getErr = getErr.WithContext(withTenant(getErr.Context(), Tenant{ID: "t1", Name: "T"}))
	getErrRec := httptest.NewRecorder()
	handleGlobalSetIDsAPI(getErrRec, getErr, errSetIDStore{err: errBoom{}})
	if getErrRec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", getErrRec.Code)
	}

	badTenant := httptest.NewRequest(http.MethodPost, "/org/api/global-setids", nil)
	badTenantRec := httptest.NewRecorder()
	handleGlobalSetIDsAPI(badTenantRec, badTenant, newSetIDMemoryStore())
	if badTenantRec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", badTenantRec.Code)
	}

	badJSON := httptest.NewRequest(http.MethodPost, "/org/api/global-setids", strings.NewReader("{"))
	badJSON = badJSON.WithContext(withTenant(badJSON.Context(), Tenant{ID: "t1", Name: "T"}))
	badJSONRec := httptest.NewRecorder()
	handleGlobalSetIDsAPI(badJSONRec, badJSON, newSetIDMemoryStore())
	if badJSONRec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", badJSONRec.Code)
	}

	missing := httptest.NewRequest(http.MethodPost, "/org/api/global-setids", bytes.NewBufferString(`{"name":""}`))
	missing = missing.WithContext(withTenant(missing.Context(), Tenant{ID: "t1", Name: "T"}))
	missingRec := httptest.NewRecorder()
	handleGlobalSetIDsAPI(missingRec, missing, newSetIDMemoryStore())
	if missingRec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", missingRec.Code)
	}

	forbidden := httptest.NewRequest(http.MethodPost, "/org/api/global-setids", bytes.NewBufferString(`{"name":"Shared","request_code":"r1"}`))
	forbidden = forbidden.WithContext(withTenant(forbidden.Context(), Tenant{ID: "t1", Name: "T"}))
	forbiddenRec := httptest.NewRecorder()
	handleGlobalSetIDsAPI(forbiddenRec, forbidden, newSetIDMemoryStore())
	if forbiddenRec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", forbiddenRec.Code)
	}
}

func TestHandleGlobalSetIDsAPI_StoreErrorAndSuccess(t *testing.T) {
	body := bytes.NewBufferString(`{"name":"Shared","request_code":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/org/api/global-setids", body)
	req.Header.Set("X-Actor-Scope", "saas")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleGlobalSetIDsAPI(rec, req, errSetIDStore{err: errBoom{}})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}

	okBody := bytes.NewBufferString(`{"name":"Shared","request_code":"r2"}`)
	okReq := httptest.NewRequest(http.MethodPost, "/org/api/global-setids", okBody)
	okReq.Header.Set("X-Actor-Scope", "saas")
	okReq = okReq.WithContext(withTenant(okReq.Context(), Tenant{ID: "t1", Name: "T"}))
	okRec := httptest.NewRecorder()
	handleGlobalSetIDsAPI(okRec, okReq, newSetIDMemoryStore())
	if okRec.Code != http.StatusCreated {
		t.Fatalf("status=%d", okRec.Code)
	}
	if !strings.Contains(okRec.Body.String(), "SHARE") {
		t.Fatalf("unexpected body: %q", okRec.Body.String())
	}
}

func TestWriteInternalAPIError_Statuses(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
	}{
		{name: "stable", err: errors.New("SETID_NOT_FOUND"), status: http.StatusUnprocessableEntity},
		{name: "bad-request", err: newBadRequestError("bad"), status: http.StatusBadRequest},
		{name: "default", err: errors.New("boom"), status: http.StatusInternalServerError},
		{name: "unknown-code", err: errors.New("UNKNOWN"), status: http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/org/api/setids", nil)
			rec := httptest.NewRecorder()
			writeInternalAPIError(rec, req, tc.err, "fallback")
			if rec.Code != tc.status {
				t.Fatalf("status=%d", rec.Code)
			}
		})
	}
}
