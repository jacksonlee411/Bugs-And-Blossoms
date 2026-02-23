package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleInternalPolicyStateAPI(t *testing.T) {
	resetPolicyActivationRuntimeForTest()
	t.Cleanup(resetPolicyActivationRuntimeForTest)

	recTenantMissing := httptest.NewRecorder()
	reqTenantMissing := httptest.NewRequest(http.MethodGet, "/internal/policies/state?capability_key=org.policy_activation.manage", nil)
	handleInternalPolicyStateAPI(recTenantMissing, reqTenantMissing)
	if recTenantMissing.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", recTenantMissing.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/internal/policies/state?capability_key=org.policy_activation.manage", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{RoleSlug: "tenant-admin"}))
	rec := httptest.NewRecorder()
	handleInternalPolicyStateAPI(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"capability_key":"org.policy_activation.manage"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	recMethod := httptest.NewRecorder()
	reqMethod := httptest.NewRequest(http.MethodPost, "/internal/policies/state", nil)
	reqMethod = reqMethod.WithContext(withTenant(reqMethod.Context(), Tenant{ID: "t1"}))
	reqMethod = reqMethod.WithContext(withPrincipal(reqMethod.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalPolicyStateAPI(recMethod, reqMethod)
	if recMethod.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", recMethod.Code)
	}

	recMissingCapability := httptest.NewRecorder()
	reqMissingCapability := httptest.NewRequest(http.MethodGet, "/internal/policies/state", nil)
	reqMissingCapability = reqMissingCapability.WithContext(withTenant(reqMissingCapability.Context(), Tenant{ID: "t1"}))
	reqMissingCapability = reqMissingCapability.WithContext(withPrincipal(reqMissingCapability.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalPolicyStateAPI(recMissingCapability, reqMissingCapability)
	if recMissingCapability.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", recMissingCapability.Code)
	}

	recForbidden := httptest.NewRecorder()
	reqForbidden := httptest.NewRequest(http.MethodGet, "/internal/policies/state?capability_key=org.policy_activation.manage", nil)
	reqForbidden = reqForbidden.WithContext(withTenant(reqForbidden.Context(), Tenant{ID: "t1"}))
	handleInternalPolicyStateAPI(recForbidden, reqForbidden)
	if recForbidden.Code != http.StatusForbidden {
		t.Fatalf("status=%d", recForbidden.Code)
	}

	recUnknown := httptest.NewRecorder()
	reqUnknown := httptest.NewRequest(http.MethodGet, "/internal/policies/state?capability_key=unknown.key", nil)
	reqUnknown = reqUnknown.WithContext(withTenant(reqUnknown.Context(), Tenant{ID: "t1"}))
	reqUnknown = reqUnknown.WithContext(withPrincipal(reqUnknown.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalPolicyStateAPI(recUnknown, reqUnknown)
	if recUnknown.Code != http.StatusNotFound || !strings.Contains(recUnknown.Body.String(), functionalAreaMissingCode) {
		t.Fatalf("status=%d body=%s", recUnknown.Code, recUnknown.Body.String())
	}
}

func TestHandleInternalPolicyMutationAPIs(t *testing.T) {
	resetPolicyActivationRuntimeForTest()
	t.Cleanup(resetPolicyActivationRuntimeForTest)

	makeReq := func(path string, body string) *http.Request {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{RoleSlug: "tenant-admin"}))
		return req
	}

	recTenantMissing := httptest.NewRecorder()
	reqTenantMissing := httptest.NewRequest(http.MethodPost, "/internal/policies/draft", bytes.NewBufferString(`{"capability_key":"org.policy_activation.manage","draft_policy_version":"2026-03-01"}`))
	handleInternalPolicyDraftAPI(recTenantMissing, reqTenantMissing)
	if recTenantMissing.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", recTenantMissing.Code)
	}

	recMethod := httptest.NewRecorder()
	reqMethod := httptest.NewRequest(http.MethodGet, "/internal/policies/draft", nil)
	reqMethod = reqMethod.WithContext(withTenant(reqMethod.Context(), Tenant{ID: "t1"}))
	reqMethod = reqMethod.WithContext(withPrincipal(reqMethod.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalPolicyDraftAPI(recMethod, reqMethod)
	if recMethod.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", recMethod.Code)
	}

	recForbidden := httptest.NewRecorder()
	reqForbidden := httptest.NewRequest(http.MethodPost, "/internal/policies/draft", bytes.NewBufferString(`{"capability_key":"org.policy_activation.manage","draft_policy_version":"2026-03-01"}`))
	reqForbidden = reqForbidden.WithContext(withTenant(reqForbidden.Context(), Tenant{ID: "t1"}))
	handleInternalPolicyDraftAPI(recForbidden, reqForbidden)
	if recForbidden.Code != http.StatusForbidden {
		t.Fatalf("status=%d", recForbidden.Code)
	}

	recBadJSON := httptest.NewRecorder()
	reqBadJSON := makeReq("/internal/policies/draft", "{")
	handleInternalPolicyDraftAPI(recBadJSON, reqBadJSON)
	if recBadJSON.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", recBadJSON.Code)
	}

	recBadRequest := httptest.NewRecorder()
	reqBadRequest := makeReq("/internal/policies/draft", `{"capability_key":"","draft_policy_version":"2026-03-01"}`)
	handleInternalPolicyDraftAPI(recBadRequest, reqBadRequest)
	if recBadRequest.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", recBadRequest.Code)
	}

	recDraftMissing := httptest.NewRecorder()
	reqDraftMissing := makeReq("/internal/policies/activate", `{"capability_key":"org.policy_activation.manage","target_policy_version":"2026-03-01"}`)
	handleInternalPolicyActivateAPI(recDraftMissing, reqDraftMissing)
	if recDraftMissing.Code != http.StatusConflict || !strings.Contains(recDraftMissing.Body.String(), policyActivationCodeDraftMissing) {
		t.Fatalf("status=%d body=%s", recDraftMissing.Code, recDraftMissing.Body.String())
	}

	recDraft := httptest.NewRecorder()
	reqDraft := makeReq("/internal/policies/draft", `{"capability_key":"org.policy_activation.manage","draft_policy_version":"2026-03-01","operator":"tester"}`)
	handleInternalPolicyDraftAPI(recDraft, reqDraft)
	if recDraft.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recDraft.Code, recDraft.Body.String())
	}

	recActivate := httptest.NewRecorder()
	reqActivate := makeReq("/internal/policies/activate", `{"capability_key":"org.policy_activation.manage","target_policy_version":"2026-03-01","operator":"tester"}`)
	handleInternalPolicyActivateAPI(recActivate, reqActivate)
	if recActivate.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recActivate.Code, recActivate.Body.String())
	}
	var activated capabilityPolicyState
	if err := json.Unmarshal(recActivate.Body.Bytes(), &activated); err != nil {
		t.Fatalf("unmarshal=%v body=%s", err, recActivate.Body.String())
	}
	if activated.ActivePolicyVersion != "2026-03-01" || activated.ActivationState != policyActivationStateActive {
		t.Fatalf("activated=%+v", activated)
	}

	recRollback := httptest.NewRecorder()
	reqRollback := makeReq("/internal/policies/rollback", `{"capability_key":"org.policy_activation.manage","target_policy_version":"2026-02-23","operator":"tester"}`)
	handleInternalPolicyRollbackAPI(recRollback, reqRollback)
	if recRollback.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recRollback.Code, recRollback.Body.String())
	}

	recUnknown := httptest.NewRecorder()
	reqUnknown := makeReq("/internal/policies/draft", `{"capability_key":"unknown.key","draft_policy_version":"2026-03-01","operator":"tester"}`)
	handleInternalPolicyDraftAPI(recUnknown, reqUnknown)
	if recUnknown.Code != http.StatusNotFound || !strings.Contains(recUnknown.Body.String(), functionalAreaMissingCode) {
		t.Fatalf("status=%d body=%s", recUnknown.Code, recUnknown.Body.String())
	}

	recVersionRequired := httptest.NewRecorder()
	reqVersionRequired := makeReq("/internal/policies/draft", `{"capability_key":"org.policy_activation.manage","draft_policy_version":" "}`)
	handleInternalPolicyDraftAPI(recVersionRequired, reqVersionRequired)
	if recVersionRequired.Code != http.StatusBadRequest || !strings.Contains(recVersionRequired.Body.String(), policyActivationCodeVersionRequired) {
		t.Fatalf("status=%d body=%s", recVersionRequired.Code, recVersionRequired.Body.String())
	}
}

func TestWritePolicyActivationError(t *testing.T) {
	cases := []struct {
		err        error
		wantStatus int
		wantCode   string
	}{
		{err: errors.New(functionalAreaMissingCode), wantStatus: http.StatusNotFound, wantCode: functionalAreaMissingCode},
		{err: errors.New(policyActivationCodeVersionRequired), wantStatus: http.StatusBadRequest, wantCode: policyActivationCodeVersionRequired},
		{err: errors.New(policyActivationCodeDraftMissing), wantStatus: http.StatusConflict, wantCode: policyActivationCodeDraftMissing},
		{err: errors.New(policyActivationCodeRollbackMissing), wantStatus: http.StatusConflict, wantCode: policyActivationCodeRollbackMissing},
		{err: errors.New("unknown"), wantStatus: http.StatusInternalServerError, wantCode: "unknown"},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/policies/activate", nil)
		writePolicyActivationError(rec, req, tc.err)
		if rec.Code != tc.wantStatus || !strings.Contains(rec.Body.String(), `"code":"`+tc.wantCode+`"`) {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	}
}
