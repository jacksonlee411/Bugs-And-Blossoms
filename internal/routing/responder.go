package routing

import (
	"encoding/json"
	"net/http"
)

type ErrorEnvelope struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	RequestID string            `json:"request_id"`
	Meta      ErrorEnvelopeMeta `json:"meta"`
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
			Code:      code,
			Message:   message,
			RequestID: "",
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
