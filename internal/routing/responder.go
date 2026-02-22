package routing

import (
	"encoding/json"
	"net/http"
	"strings"
)

type ErrorEnvelope struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	TraceID string            `json:"trace_id"`
	Meta    ErrorEnvelopeMeta `json:"meta"`
}

type ErrorEnvelopeMeta struct {
	Path   string `json:"path"`
	Method string `json:"method"`
}

func WriteError(w http.ResponseWriter, r *http.Request, rc RouteClass, status int, code string, message string) {
	if isJSONOnly(rc) || wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(ErrorEnvelope{
			Code:    code,
			Message: message,
			TraceID: traceIDFromRequest(r),
			Meta: ErrorEnvelopeMeta{
				Path:   r.URL.Path,
				Method: r.Method,
			},
		})
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte("<!doctype html><html><body>"))
	_, _ = w.Write([]byte(message))
	_, _ = w.Write([]byte("</body></html>"))
}

func wantsJSON(r *http.Request) bool {
	return r.Header.Get("Accept") == "application/json" || r.Header.Get("Accept") == "application/json; charset=utf-8"
}

func isJSONOnly(rc RouteClass) bool {
	return rc == RouteClassInternalAPI || rc == RouteClassPublicAPI || rc == RouteClassWebhook
}

func traceIDFromRequest(r *http.Request) string {
	traceparent := strings.TrimSpace(r.Header.Get("traceparent"))
	if traceparent == "" {
		return ""
	}
	parts := strings.Split(traceparent, "-")
	if len(parts) != 4 {
		return ""
	}
	traceID := strings.ToLower(parts[1])
	if len(traceID) != 32 || traceID == "00000000000000000000000000000000" {
		return ""
	}
	for _, ch := range traceID {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return ""
		}
	}
	return traceID
}
