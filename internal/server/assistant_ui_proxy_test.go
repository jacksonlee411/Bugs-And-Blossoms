package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAssistantUIRetiredHandlerReturnsGone(t *testing.T) {
	handler := newAssistantUIRetiredHandler()
	req := httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusGone {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAssistantUIRetiredHandlerRejectsBypassWriteByRetiringWholePrefix(t *testing.T) {
	handler := newAssistantUIRetiredHandler()
	req := httptest.NewRequest(http.MethodPost, "http://localhost/assistant-ui/org/api/org-units", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusGone {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAssistantUIRetiredHandlerRetiresNestedPath(t *testing.T) {
	handler := newAssistantUIRetiredHandler()
	req := httptest.NewRequest(http.MethodHead, "http://localhost/assistant-ui/bridge.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusGone {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAssistantUITraceID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui", nil)
	req.Header.Set("traceparent", "00-1234567890abcdef1234567890abcdef-1234567890abcdef-01")
	if got := assistantUITraceID(req); got != "1234567890abcdef1234567890abcdef" {
		t.Fatalf("traceID=%q", got)
	}
	req.Header.Set("traceparent", "bad")
	if got := assistantUITraceID(req); got != "" {
		t.Fatalf("traceID=%q", got)
	}
}
