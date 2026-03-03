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

func TestHandleAssistantModelProvidersValidateAndApplyAPI(t *testing.T) {
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

	validBody := `{"provider_routing":{"strategy":"priority_failover","fallback_enabled":true},"providers":[{"name":"openai","enabled":true,"model":"gpt-4o-mini","endpoint":"https://api.openai.com/v1","timeout_ms":1000,"retries":1,"priority":10,"key_ref":"OPENAI_API_KEY"}]}`

	unauthorized := httptest.NewRecorder()
	unauthorizedReq := httptest.NewRequest(http.MethodPost, "/internal/assistant/model-providers:apply", http.NoBody)
	unauthorizedReq.Body = io.NopCloser(strings.NewReader(validBody))
	handleAssistantModelProvidersApplyAPI(unauthorized, unauthorizedReq, svc)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got=%d", unauthorized.Code)
	}

	applyRec := httptest.NewRecorder()
	applyReq := assistantReqWithContext(http.MethodPost, "/internal/assistant/model-providers:apply", validBody, true, true)
	handleAssistantModelProvidersApplyAPI(applyRec, applyReq, svc)
	if applyRec.Code != http.StatusOK {
		t.Fatalf("apply status=%d body=%s", applyRec.Code, applyRec.Body.String())
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
	foundApplied := false
	for _, model := range models.Models {
		if model.Provider == "openai" && model.Model == "gpt-4o-mini" {
			foundApplied = true
			break
		}
	}
	if !foundApplied {
		t.Fatalf("expected applied model openai/gpt-4o-mini, got=%+v", models.Models)
	}
}
