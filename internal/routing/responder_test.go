package routing

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteError_AcceptJSONCharset(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Accept", "application/json; charset=utf-8")
	rec := httptest.NewRecorder()

	WriteError(rec, req, RouteClassUI, http.StatusNotFound, "not_found", "not found")
	if !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("content-type=%q", rec.Header().Get("Content-Type"))
	}
}
