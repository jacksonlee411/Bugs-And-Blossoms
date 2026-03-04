package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestAssistantUIProxyHandler(t *testing.T) {
	t.Run("defaults to local upstream when env missing", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "ok:"+r.URL.Path)
		}))
		defer upstream.Close()
		parsed, err := url.Parse(upstream.URL)
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv("LIBRECHAT_PORT", parsed.Port())
		t.Setenv("LIBRECHAT_UPSTREAM", "")
		h := newAssistantUIProxyHandler()
		req := httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "ok:/") {
			t.Fatalf("unexpected body=%s", rec.Body.String())
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
			w.Header().Set("Set-Cookie", "upstream_sid=1; Path=/")
			w.Header().Set("Content-Type", "text/plain")
			_, _ = io.WriteString(w, r.Method+"|"+r.URL.Path+"|"+r.Header.Get("X-Forwarded-Prefix")+"|"+r.Host+"|"+r.Header.Get("Cookie")+"|"+r.Header.Get("Authorization")+"|"+r.Header.Get("X-Client-Trace"))
		}))
		defer upstream.Close()

		t.Setenv("LIBRECHAT_UPSTREAM", upstream.URL+"/chat")
		h := newAssistantUIProxyHandler()

		req := httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui/assets/app.js", nil)
		req.Header.Set("Cookie", "sid=local")
		req.Header.Set("Authorization", "Bearer test")
		req.Header.Set("X-Client-Trace", "trace-001")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if got := rec.Body.String(); got == "" || !strings.Contains(got, "GET|/chat/") {
			t.Fatalf("unexpected body=%s", got)
		}
		if strings.Contains(rec.Body.String(), "sid=local") {
			t.Fatalf("cookie header should be stripped, got=%q", rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "Bearer test") {
			t.Fatalf("authorization header should be stripped, got=%q", rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "trace-001") {
			t.Fatalf("custom header should be forwarded, got=%q", rec.Body.String())
		}
		if setCookie := rec.Result().Header.Get("Set-Cookie"); setCookie != "" {
			t.Fatalf("set-cookie should be stripped, got=%q", setCookie)
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

	t.Run("method and path guard", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "ok")
		}))
		defer upstream.Close()

		t.Setenv("LIBRECHAT_UPSTREAM", upstream.URL)
		h := newAssistantUIProxyHandler()

		postReq := httptest.NewRequest(http.MethodPost, "http://localhost/assistant-ui", nil)
		postRec := httptest.NewRecorder()
		h.ServeHTTP(postRec, postReq)
		if postRec.Code != http.StatusOK {
			t.Fatalf("post status=%d body=%s", postRec.Code, postRec.Body.String())
		}
		optionsReq := httptest.NewRequest(http.MethodOptions, "http://localhost/assistant-ui", nil)
		optionsRec := httptest.NewRecorder()
		h.ServeHTTP(optionsRec, optionsReq)
		if optionsRec.Code != http.StatusOK {
			t.Fatalf("options status=%d body=%s", optionsRec.Code, optionsRec.Body.String())
		}
		putReq := httptest.NewRequest(http.MethodPut, "http://localhost/assistant-ui", nil)
		putRec := httptest.NewRecorder()
		h.ServeHTTP(putRec, putReq)
		if putRec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("put status=%d body=%s", putRec.Code, putRec.Body.String())
		}

		pathReq := httptest.NewRequest(http.MethodGet, "http://localhost/not-assistant", nil)
		pathRec := httptest.NewRecorder()
		h.ServeHTTP(pathRec, pathReq)
		if pathRec.Code != http.StatusBadRequest {
			t.Fatalf("path status=%d body=%s", pathRec.Code, pathRec.Body.String())
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
