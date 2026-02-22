package routing

import (
	"encoding/json"
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

func TestTraceIDFromRequest(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		traceparent string
		want        string
	}{
		{name: "empty", traceparent: "", want: ""},
		{name: "malformed segments", traceparent: "00-abc-01", want: ""},
		{name: "invalid chars", traceparent: "00-0123456789abcdef0123456789abcdeg-0123456789abcdef-01", want: ""},
		{name: "all zero trace", traceparent: "00-00000000000000000000000000000000-0123456789abcdef-01", want: ""},
		{name: "valid", traceparent: "00-ABCDEFABCDEFABCDEFABCDEFABCDEFAB-0123456789abcdef-01", want: "abcdefabcdefabcdefabcdefabcdefab"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			if tc.traceparent != "" {
				req.Header.Set("traceparent", tc.traceparent)
			}
			if got := traceIDFromRequest(req); got != tc.want {
				t.Fatalf("traceIDFromRequest()=%q want %q", got, tc.want)
			}
		})
	}
}

func TestWriteError_TraceIDFromTraceparent(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("traceparent", "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()

	WriteError(rec, req, RouteClassInternalAPI, http.StatusBadRequest, "bad", "bad")

	var body ErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.TraceID != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("trace_id=%q", body.TraceID)
	}
}
