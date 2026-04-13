package server

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAssistantUILog_Coverage(t *testing.T) {
	var buf bytes.Buffer
	originalWriter := log.Writer()
	defer log.SetOutput(originalWriter)
	log.SetOutput(&buf)

	t.Run("with tenant and headers", func(t *testing.T) {
		buf.Reset()
		req := httptest.NewRequest(http.MethodGet, "/assistant-ui", nil)
		req = req.WithContext(withTenant(context.Background(), Tenant{ID: "tenant_1"}))
		req.Header.Set("X-Request-ID", "req_1")
		req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-00")
		assistantUILog(req, "ok")
		output := buf.String()
		if !strings.Contains(output, "tenant_id=tenant_1") || !strings.Contains(output, "request_id=req_1") || !strings.Contains(output, "reason=ok") {
			t.Fatalf("unexpected log output=%q", output)
		}
	})

	t.Run("fallback values", func(t *testing.T) {
		buf.Reset()
		req := httptest.NewRequest(http.MethodHead, "/assistant-ui", nil)
		assistantUILog(req, "missing")
		output := buf.String()
		if !strings.Contains(output, "tenant_id=-") || !strings.Contains(output, "request_id=-") || !strings.Contains(output, "trace_id=-") {
			t.Fatalf("unexpected fallback log output=%q", output)
		}
	})
}
