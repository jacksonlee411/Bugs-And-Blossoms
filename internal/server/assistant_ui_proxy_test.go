package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAssistantUIProxyHandlerRejectsInvalidMethod(t *testing.T) {
	handler := newAssistantUIProxyHandler()
	req := httptest.NewRequest(http.MethodPost, "http://localhost/assistant-ui", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAssistantUIProxyHandlerRejectsInvalidPath(t *testing.T) {
	handler := newAssistantUIProxyHandler()
	req := httptest.NewRequest(http.MethodGet, "http://localhost/not-assistant", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAssistantUIProxyHandlerRejectsBypassPath(t *testing.T) {
	handler := newAssistantUIProxyHandler()
	req := httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui/org/api/org-units", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAssistantUIProxyHandlerRedirectsAliasToFormalEntry(t *testing.T) {
	handler := newAssistantUIProxyHandler()
	req := httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui/bridge.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Result().Header.Get("Location"); loc != libreChatFormalEntryPrefix {
		t.Fatalf("location=%q", loc)
	}
}

func TestAssistantUIProxyTraceID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui", nil)
	req.Header.Set("traceparent", "00-1234567890abcdef1234567890abcdef-1234567890abcdef-01")
	if got := assistantUIProxyTraceID(req); got != "1234567890abcdef1234567890abcdef" {
		t.Fatalf("traceID=%q", got)
	}
	req.Header.Set("traceparent", "bad")
	if got := assistantUIProxyTraceID(req); got != "" {
		t.Fatalf("traceID=%q", got)
	}
}
