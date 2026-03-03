package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAssistantModelProvidersAPI_Branches(t *testing.T) {
	svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/assistant/model-providers", nil)
	handleAssistantModelProvidersAPI(rec, req, svc)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/internal/assistant/model-providers", nil)
	handleAssistantModelProvidersAPI(rec, req, nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/internal/assistant/models", nil)
	handleAssistantModelsAPI(rec, req, nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/internal/assistant/model-providers:validate", strings.NewReader("{"))
	handleAssistantModelProvidersValidateAPI(rec, req, svc)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/internal/assistant/model-providers:validate", nil)
	handleAssistantModelProvidersValidateAPI(rec, req, svc)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/internal/assistant/model-providers:apply", strings.NewReader("{"))
	handleAssistantModelProvidersApplyAPI(rec, req, svc)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = assistantReqWithContext(http.MethodPost, "/internal/assistant/model-providers:apply", "{", true, true)
	handleAssistantModelProvidersApplyAPI(rec, req, svc)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}

	invalid := `{"provider_routing":{"strategy":"priority_failover","fallback_enabled":true},"providers":[{"name":"invalid","enabled":true,"model":"x","endpoint":"builtin://x","timeout_ms":1000,"retries":1,"priority":1,"key_ref":"X"}]}`
	rec = httptest.NewRecorder()
	req = assistantReqWithContext(http.MethodPost, "/internal/assistant/model-providers:apply", invalid, true, true)
	handleAssistantModelProvidersApplyAPI(rec, req, svc)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/internal/assistant/models", nil)
	handleAssistantModelsAPI(rec, req, svc)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}
