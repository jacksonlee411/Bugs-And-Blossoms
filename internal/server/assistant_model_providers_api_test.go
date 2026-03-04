package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleAssistantModelProvidersAPI(t *testing.T) {
	svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/internal/assistant/model-providers", nil)
	handleAssistantModelProvidersAPI(rec, req, svc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload assistantModelProvidersResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal=%v", err)
	}
	if len(payload.Providers) == 0 {
		t.Fatal("expected providers")
	}
}

func TestHandleAssistantModelProvidersValidateAndModelsAPI(t *testing.T) {
	svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})

	invalidBody := `{"provider_routing":{"strategy":"priority_failover","fallback_enabled":true},"providers":[{"name":"invalid","enabled":true,"model":"x","endpoint":"builtin://x","timeout_ms":1000,"retries":1,"priority":1,"key_ref":"X"}]}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/assistant/model-providers:validate", http.NoBody)
	req.Body = io.NopCloser(strings.NewReader(invalidBody))
	handleAssistantModelProvidersValidateAPI(rec, req, svc)
	if rec.Code != http.StatusOK {
		t.Fatalf("validate status=%d", rec.Code)
	}
	var validateResp assistantModelProvidersValidateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &validateResp); err != nil {
		t.Fatalf("unmarshal validate=%v", err)
	}
	if validateResp.Valid {
		t.Fatal("expected invalid payload rejected")
	}

	modelsRec := httptest.NewRecorder()
	modelsReq := httptest.NewRequest(http.MethodGet, "/internal/assistant/models", nil)
	handleAssistantModelsAPI(modelsRec, modelsReq, svc)
	if modelsRec.Code != http.StatusOK {
		t.Fatalf("models status=%d", modelsRec.Code)
	}
	var models assistantModelsResponse
	if err := json.Unmarshal(modelsRec.Body.Bytes(), &models); err != nil {
		t.Fatalf("unmarshal models=%v", err)
	}
	if len(models.Models) == 0 {
		t.Fatal("expected at least one model")
	}
}
