package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAssistantUIProxyHandler(t *testing.T) {
	t.Run("unavailable when upstream missing", func(t *testing.T) {
		t.Setenv("LIBRECHAT_UPSTREAM", "")
		h := newAssistantUIProxyHandler()
		req := httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("unavailable when upstream invalid", func(t *testing.T) {
		t.Setenv("LIBRECHAT_UPSTREAM", "://bad")
		h := newAssistantUIProxyHandler()
		req := httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("proxy forward and error handler", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = io.WriteString(w, r.URL.Path+"|"+r.Header.Get("X-Forwarded-Prefix")+"|"+r.Host)
		}))
		defer upstream.Close()

		t.Setenv("LIBRECHAT_UPSTREAM", upstream.URL+"/chat")
		h := newAssistantUIProxyHandler()

		req := httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui/assets/app.js", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if got := rec.Body.String(); got == "" || got[0:5] != "/chat" {
			t.Fatalf("unexpected body=%s", got)
		}

		unreachableURL := "http://127.0.0.1:1"
		t.Setenv("LIBRECHAT_UPSTREAM", unreachableURL)
		errorProxy := newAssistantUIProxyHandler()
		errorReq := httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui", nil)
		errorRec := httptest.NewRecorder()
		errorProxy.ServeHTTP(errorRec, errorReq)
		if errorRec.Code != http.StatusBadGateway {
			t.Fatalf("status=%d body=%s", errorRec.Code, errorRec.Body.String())
		}
	})
}

func TestAssistantUIUnavailableHandler(t *testing.T) {
	h := assistantUIUnavailableHandler("reason")
	req := httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestJoinProxyPath(t *testing.T) {
	cases := []struct {
		base   string
		suffix string
		want   string
	}{
		{base: "", suffix: "/x", want: "/x"},
		{base: "/", suffix: "/x", want: "/x"},
		{base: "/chat/", suffix: "/x", want: "/chat/x"},
		{base: "/chat/", suffix: "x", want: "/chat/x"},
		{base: "/chat", suffix: "/x", want: "/chat/x"},
		{base: "/chat", suffix: "x", want: "/chat/x"},
	}
	for _, tc := range cases {
		if got := joinProxyPath(tc.base, tc.suffix); got != tc.want {
			t.Fatalf("joinProxyPath(%q, %q)=%q want=%q", tc.base, tc.suffix, got, tc.want)
		}
	}
}
