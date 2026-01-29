package server

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleSetIDsAPI_TenantMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/orgunit/api/setids", nil)
	rec := httptest.NewRecorder()
	handleSetIDsAPI(rec, req, newSetIDMemoryStore())
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetIDsAPI_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/orgunit/api/setids", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDsAPI(rec, req, newSetIDMemoryStore())
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetIDsAPI_BadJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/orgunit/api/setids", strings.NewReader("{"))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDsAPI(rec, req, newSetIDMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetIDsAPI_InvalidRequest(t *testing.T) {
	body := bytes.NewBufferString(`{"setid":"","name":"","request_id":""}`)
	req := httptest.NewRequest(http.MethodPost, "/orgunit/api/setids", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDsAPI(rec, req, newSetIDMemoryStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetIDsAPI_EnsureBootstrapError(t *testing.T) {
	body := bytes.NewBufferString(`{"setid":"A0001","name":"A","request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/orgunit/api/setids", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDsAPI(rec, req, errSetIDStore{err: errBoom{}})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetIDsAPI_CreateSetIDError(t *testing.T) {
	body := bytes.NewBufferString(`{"setid":"A0001","name":"A","request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/orgunit/api/setids", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDsAPI(rec, req, partialSetIDStore{createSetErr: errors.New("SETID_ALREADY_EXISTS")})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetIDsAPI_Success(t *testing.T) {
	body := bytes.NewBufferString(`{"setid":"A0001","name":"A","request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/orgunit/api/setids", body)
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
	badTenant := httptest.NewRequest(http.MethodPost, "/orgunit/api/setid-bindings", nil)
	badTenantRec := httptest.NewRecorder()
	handleSetIDBindingsAPI(badTenantRec, badTenant, newSetIDMemoryStore())
	if badTenantRec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", badTenantRec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/orgunit/api/setid-bindings", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDBindingsAPI(rec, req, newSetIDMemoryStore())
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}

	badJSON := httptest.NewRequest(http.MethodPost, "/orgunit/api/setid-bindings", strings.NewReader("{"))
	badJSON = badJSON.WithContext(withTenant(badJSON.Context(), Tenant{ID: "t1", Name: "T"}))
	badJSONRec := httptest.NewRecorder()
	handleSetIDBindingsAPI(badJSONRec, badJSON, newSetIDMemoryStore())
	if badJSONRec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", badJSONRec.Code)
	}

	missing := httptest.NewRequest(http.MethodPost, "/orgunit/api/setid-bindings", bytes.NewBufferString(`{"org_unit_id":""}`))
	missing = missing.WithContext(withTenant(missing.Context(), Tenant{ID: "t1", Name: "T"}))
	missingRec := httptest.NewRecorder()
	handleSetIDBindingsAPI(missingRec, missing, newSetIDMemoryStore())
	if missingRec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", missingRec.Code)
	}

	badDate := httptest.NewRequest(http.MethodPost, "/orgunit/api/setid-bindings", bytes.NewBufferString(`{"org_unit_id":"org1","setid":"A0001","effective_date":"bad","request_id":"r1"}`))
	badDate = badDate.WithContext(withTenant(badDate.Context(), Tenant{ID: "t1", Name: "T"}))
	badDateRec := httptest.NewRecorder()
	handleSetIDBindingsAPI(badDateRec, badDate, newSetIDMemoryStore())
	if badDateRec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", badDateRec.Code)
	}
}

func TestHandleSetIDBindingsAPI_StoreError(t *testing.T) {
	body := bytes.NewBufferString(`{"org_unit_id":"org1","setid":"A0001","effective_date":"2026-01-01","request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/orgunit/api/setid-bindings", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDBindingsAPI(rec, req, errSetIDStore{err: errBoom{}})
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

	body := bytes.NewBufferString(`{"org_unit_id":"org1","setid":"A0001","effective_date":"2026-01-01","request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/orgunit/api/setid-bindings", body)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetIDBindingsAPI(rec, req, store)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "A0001") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleGlobalSetIDsAPI_BadInputs(t *testing.T) {
	badMethod := httptest.NewRequest(http.MethodPut, "/orgunit/api/global-setids", nil)
	badMethodRec := httptest.NewRecorder()
	handleGlobalSetIDsAPI(badMethodRec, badMethod, newSetIDMemoryStore())
	if badMethodRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", badMethodRec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/orgunit/api/global-setids", nil)
	rec := httptest.NewRecorder()
	handleGlobalSetIDsAPI(rec, req, newSetIDMemoryStore())
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}

	okGet := httptest.NewRequest(http.MethodGet, "/orgunit/api/global-setids", nil)
	okGet = okGet.WithContext(withTenant(okGet.Context(), Tenant{ID: "t1", Name: "T"}))
	okGetRec := httptest.NewRecorder()
	handleGlobalSetIDsAPI(okGetRec, okGet, newSetIDMemoryStore())
	if okGetRec.Code != http.StatusOK {
		t.Fatalf("status=%d", okGetRec.Code)
	}

	getErr := httptest.NewRequest(http.MethodGet, "/orgunit/api/global-setids", nil)
	getErr = getErr.WithContext(withTenant(getErr.Context(), Tenant{ID: "t1", Name: "T"}))
	getErrRec := httptest.NewRecorder()
	handleGlobalSetIDsAPI(getErrRec, getErr, errSetIDStore{err: errBoom{}})
	if getErrRec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", getErrRec.Code)
	}

	badTenant := httptest.NewRequest(http.MethodPost, "/orgunit/api/global-setids", nil)
	badTenantRec := httptest.NewRecorder()
	handleGlobalSetIDsAPI(badTenantRec, badTenant, newSetIDMemoryStore())
	if badTenantRec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", badTenantRec.Code)
	}

	badJSON := httptest.NewRequest(http.MethodPost, "/orgunit/api/global-setids", strings.NewReader("{"))
	badJSON = badJSON.WithContext(withTenant(badJSON.Context(), Tenant{ID: "t1", Name: "T"}))
	badJSONRec := httptest.NewRecorder()
	handleGlobalSetIDsAPI(badJSONRec, badJSON, newSetIDMemoryStore())
	if badJSONRec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", badJSONRec.Code)
	}

	missing := httptest.NewRequest(http.MethodPost, "/orgunit/api/global-setids", bytes.NewBufferString(`{"name":""}`))
	missing = missing.WithContext(withTenant(missing.Context(), Tenant{ID: "t1", Name: "T"}))
	missingRec := httptest.NewRecorder()
	handleGlobalSetIDsAPI(missingRec, missing, newSetIDMemoryStore())
	if missingRec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", missingRec.Code)
	}

	forbidden := httptest.NewRequest(http.MethodPost, "/orgunit/api/global-setids", bytes.NewBufferString(`{"name":"Shared","request_id":"r1"}`))
	forbidden = forbidden.WithContext(withTenant(forbidden.Context(), Tenant{ID: "t1", Name: "T"}))
	forbiddenRec := httptest.NewRecorder()
	handleGlobalSetIDsAPI(forbiddenRec, forbidden, newSetIDMemoryStore())
	if forbiddenRec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", forbiddenRec.Code)
	}
}

func TestHandleGlobalSetIDsAPI_StoreErrorAndSuccess(t *testing.T) {
	body := bytes.NewBufferString(`{"name":"Shared","request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/orgunit/api/global-setids", body)
	req.Header.Set("X-Actor-Scope", "saas")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()
	handleGlobalSetIDsAPI(rec, req, errSetIDStore{err: errBoom{}})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}

	okBody := bytes.NewBufferString(`{"name":"Shared","request_id":"r2"}`)
	okReq := httptest.NewRequest(http.MethodPost, "/orgunit/api/global-setids", okBody)
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
			req := httptest.NewRequest(http.MethodPost, "/orgunit/api/setids", nil)
			rec := httptest.NewRecorder()
			writeInternalAPIError(rec, req, tc.err, "fallback")
			if rec.Code != tc.status {
				t.Fatalf("status=%d", rec.Code)
			}
		})
	}
}
