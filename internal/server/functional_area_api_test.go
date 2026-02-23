package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleInternalFunctionalAreaStateAPI(t *testing.T) {
	resetFunctionalAreaSwitchStoreForTest()
	t.Cleanup(resetFunctionalAreaSwitchStoreForTest)

	recTenantMissing := httptest.NewRecorder()
	reqTenantMissing := httptest.NewRequest(http.MethodGet, "/internal/functional-areas/state", nil)
	handleInternalFunctionalAreaStateAPI(recTenantMissing, reqTenantMissing)
	if recTenantMissing.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", recTenantMissing.Code)
	}

	recMethod := httptest.NewRecorder()
	reqMethod := httptest.NewRequest(http.MethodPost, "/internal/functional-areas/state", nil)
	reqMethod = reqMethod.WithContext(withTenant(reqMethod.Context(), Tenant{ID: "t1"}))
	reqMethod = reqMethod.WithContext(withPrincipal(reqMethod.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalFunctionalAreaStateAPI(recMethod, reqMethod)
	if recMethod.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", recMethod.Code)
	}

	recForbidden := httptest.NewRecorder()
	reqForbidden := httptest.NewRequest(http.MethodGet, "/internal/functional-areas/state", nil)
	reqForbidden = reqForbidden.WithContext(withTenant(reqForbidden.Context(), Tenant{ID: "t1"}))
	handleInternalFunctionalAreaStateAPI(recForbidden, reqForbidden)
	if recForbidden.Code != http.StatusForbidden {
		t.Fatalf("status=%d", recForbidden.Code)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/internal/functional-areas/state", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalFunctionalAreaStateAPI(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"functional_area_key":"staffing"`) ||
		!strings.Contains(rec.Body.String(), `"lifecycle_status":"active"`) {
		t.Fatalf("unexpected body=%s", rec.Body.String())
	}
}

func TestHandleInternalFunctionalAreaSwitchAPI(t *testing.T) {
	resetFunctionalAreaSwitchStoreForTest()
	t.Cleanup(resetFunctionalAreaSwitchStoreForTest)

	makeReq := func(method string, body string, withPrincipalCtx bool) *http.Request {
		req := httptest.NewRequest(method, "/internal/functional-areas/switch", strings.NewReader(body))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		if withPrincipalCtx {
			req = req.WithContext(withPrincipal(req.Context(), Principal{RoleSlug: "tenant-admin"}))
		}
		return req
	}

	recTenantMissing := httptest.NewRecorder()
	reqTenantMissing := httptest.NewRequest(http.MethodPost, "/internal/functional-areas/switch", strings.NewReader(`{"functional_area_key":"staffing","enabled":false}`))
	handleInternalFunctionalAreaSwitchAPI(recTenantMissing, reqTenantMissing)
	if recTenantMissing.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", recTenantMissing.Code)
	}

	recMethod := httptest.NewRecorder()
	handleInternalFunctionalAreaSwitchAPI(recMethod, makeReq(http.MethodGet, "", true))
	if recMethod.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", recMethod.Code)
	}

	recForbidden := httptest.NewRecorder()
	handleInternalFunctionalAreaSwitchAPI(recForbidden, makeReq(http.MethodPost, `{"functional_area_key":"staffing","enabled":false}`, false))
	if recForbidden.Code != http.StatusForbidden {
		t.Fatalf("status=%d", recForbidden.Code)
	}

	recBadJSON := httptest.NewRecorder()
	handleInternalFunctionalAreaSwitchAPI(recBadJSON, makeReq(http.MethodPost, "{", true))
	if recBadJSON.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", recBadJSON.Code)
	}

	recBadRequest := httptest.NewRecorder()
	handleInternalFunctionalAreaSwitchAPI(recBadRequest, makeReq(http.MethodPost, `{"functional_area_key":" ","enabled":false}`, true))
	if recBadRequest.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", recBadRequest.Code)
	}

	recMissing := httptest.NewRecorder()
	handleInternalFunctionalAreaSwitchAPI(recMissing, makeReq(http.MethodPost, `{"functional_area_key":"unknown","enabled":false}`, true))
	if recMissing.Code != http.StatusNotFound || !strings.Contains(recMissing.Body.String(), functionalAreaMissingCode) {
		t.Fatalf("status=%d body=%s", recMissing.Code, recMissing.Body.String())
	}

	recNotActive := httptest.NewRecorder()
	handleInternalFunctionalAreaSwitchAPI(recNotActive, makeReq(http.MethodPost, `{"functional_area_key":"compensation","enabled":false}`, true))
	if recNotActive.Code != http.StatusConflict || !strings.Contains(recNotActive.Body.String(), functionalAreaNotActiveCode) {
		t.Fatalf("status=%d body=%s", recNotActive.Code, recNotActive.Body.String())
	}

	recDisable := httptest.NewRecorder()
	handleInternalFunctionalAreaSwitchAPI(recDisable, makeReq(http.MethodPost, `{"functional_area_key":"staffing","enabled":false,"operator":"tester"}`, true))
	if recDisable.Code != http.StatusOK || !strings.Contains(recDisable.Body.String(), `"enabled":false`) {
		t.Fatalf("status=%d body=%s", recDisable.Code, recDisable.Body.String())
	}

	recEnable := httptest.NewRecorder()
	handleInternalFunctionalAreaSwitchAPI(recEnable, makeReq(http.MethodPost, `{"functional_area_key":"staffing","enabled":true,"operator":"tester"}`, true))
	if recEnable.Code != http.StatusOK || !strings.Contains(recEnable.Body.String(), `"enabled":true`) {
		t.Fatalf("status=%d body=%s", recEnable.Code, recEnable.Body.String())
	}
}
