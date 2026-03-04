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
	req = httptest.NewRequest(http.MethodPost, "/internal/assistant/model-providers:validate", strings.NewReader(`{}`))
	handleAssistantModelProvidersValidateAPI(rec, req, nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/internal/assistant/models", nil)
	handleAssistantModelsAPI(rec, req, svc)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}
