package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
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
		req.Header.Set("Accept", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if got := decodeAssistantUIProxyErrorCode(t, rec); got != assistantUIProxyUpstreamUnavailable {
			t.Fatalf("code=%s", got)
		}
	})

	t.Run("proxy forward and error handler", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Set-Cookie", "upstream_sid=1; Path=/")
			w.Header().Add("Set-Cookie", "refreshToken=rf-1; Path=/; HttpOnly; SameSite=Strict")
			w.Header().Add("Set-Cookie", "token_provider=librechat; Path=/; HttpOnly; SameSite=Strict")
			w.Header().Set("Content-Type", "text/plain")
			_, _ = io.WriteString(
				w,
				r.Method+"|"+r.URL.Path+"|"+r.Header.Get("X-Forwarded-Prefix")+"|"+r.Host+"|"+r.Header.Get("Cookie")+"|"+r.Header.Get("Authorization")+"|"+r.Header.Get("Accept-Language")+"|"+r.Header.Get("X-Client-Trace")+"|"+r.Header.Get("Accept-Encoding"),
			)
		}))
		defer upstream.Close()

		t.Setenv("LIBRECHAT_UPSTREAM", upstream.URL+"/chat")
		h := newAssistantUIProxyHandler()

		req := httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui/assets/app.js", nil)
		req.Header.Set("Cookie", "sid=local; refreshToken=rf-1; token_provider=librechat")
		req.Header.Set("Authorization", "Bearer test")
		req.Header.Set("Accept-Language", "zh-CN")
		req.Header.Set("Accept-Encoding", "gzip, br")
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
		if !strings.Contains(rec.Body.String(), "|Bearer test|") {
			t.Fatalf("bearer authorization header should be forwarded, got=%q", rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "|refreshToken=rf-1; token_provider=librechat|") {
			t.Fatalf("allowed auth cookies should be forwarded, got=%q", rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "sid=local") {
			t.Fatalf("app sid cookie should be stripped, got=%q", rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "trace-001") {
			t.Fatalf("non-allowlisted header should be stripped, got=%q", rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "|zh-CN|") {
			t.Fatalf("allowlisted header should be forwarded, got=%q", rec.Body.String())
		}
		if setCookie := rec.Result().Header.Get("Set-Cookie"); setCookie == "" {
			t.Fatal("expected auth set-cookie headers to be preserved for librechat auth")
		}
		for _, cookie := range rec.Result().Cookies() {
			if cookie.Name == "upstream_sid" {
				t.Fatalf("unexpected upstream_sid cookie in response: %+v", cookie)
			}
			if cookie.Name == "refreshToken" || cookie.Name == "token_provider" {
				if cookie.Path != "/assistant-ui" {
					t.Fatalf("expected path rewrite to /assistant-ui, got cookie=%+v", cookie)
				}
			}
		}

		unreachableURL := "http://127.0.0.1:1"
		t.Setenv("LIBRECHAT_UPSTREAM", unreachableURL)
		errorProxy := newAssistantUIProxyHandler()
		errorReq := httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui", nil)
		errorReq.Header.Set("Accept", "application/json")
		errorRec := httptest.NewRecorder()
		errorProxy.ServeHTTP(errorRec, errorReq)
		if errorRec.Code != http.StatusBadGateway {
			t.Fatalf("status=%d body=%s", errorRec.Code, errorRec.Body.String())
		}
		if got := decodeAssistantUIProxyErrorCode(t, errorRec); got != assistantUIProxyUpstreamUnavailable {
			t.Fatalf("code=%s", got)
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
		headReq := httptest.NewRequest(http.MethodHead, "http://localhost/assistant-ui", nil)
		headRec := httptest.NewRecorder()
		h.ServeHTTP(headRec, headReq)
		if headRec.Code != http.StatusOK {
			t.Fatalf("head status=%d body=%s", headRec.Code, headRec.Body.String())
		}
		optionsReq := httptest.NewRequest(http.MethodOptions, "http://localhost/assistant-ui", nil)
		optionsRec := httptest.NewRecorder()
		h.ServeHTTP(optionsRec, optionsReq)
		if optionsRec.Code != http.StatusOK {
			t.Fatalf("options status=%d body=%s", optionsRec.Code, optionsRec.Body.String())
		}
		putReq := httptest.NewRequest(http.MethodPut, "http://localhost/assistant-ui", nil)
		putReq.Header.Set("Accept", "application/json")
		putRec := httptest.NewRecorder()
		h.ServeHTTP(putRec, putReq)
		if putRec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("put status=%d body=%s", putRec.Code, putRec.Body.String())
		}
		if got := decodeAssistantUIProxyErrorCode(t, putRec); got != assistantUIProxyMethodNotAllowedCode {
			t.Fatalf("code=%s", got)
		}

		pathReq := httptest.NewRequest(http.MethodGet, "http://localhost/not-assistant", nil)
		pathReq.Header.Set("Accept", "application/json")
		pathRec := httptest.NewRecorder()
		h.ServeHTTP(pathRec, pathReq)
		if pathRec.Code != http.StatusBadRequest {
			t.Fatalf("path status=%d body=%s", pathRec.Code, pathRec.Body.String())
		}
		if got := decodeAssistantUIProxyErrorCode(t, pathRec); got != assistantUIProxyPathInvalidCode {
			t.Fatalf("code=%s", got)
		}

		bypassReq := httptest.NewRequest(http.MethodPost, "http://localhost/assistant-ui/org/api/org-units", strings.NewReader(`{"org_code":"BYPASS220"}`))
		bypassReq.Header.Set("Accept", "application/json")
		bypassRec := httptest.NewRecorder()
		h.ServeHTTP(bypassRec, bypassReq)
		if bypassRec.Code != http.StatusBadRequest {
			t.Fatalf("bypass status=%d body=%s", bypassRec.Code, bypassRec.Body.String())
		}
		if got := decodeAssistantUIProxyErrorCode(t, bypassRec); got != assistantUIProxyPathInvalidCode {
			t.Fatalf("bypass code=%s", got)
		}
	})
}

func TestAssistantUIUnavailableHandler(t *testing.T) {
	h := assistantUIUnavailableHandler("reason")
	req := httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := decodeAssistantUIProxyErrorCode(t, rec); got != assistantUIProxyUpstreamUnavailable {
		t.Fatalf("code=%s", got)
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

func TestAssistantUIProxyLog(t *testing.T) {
	var output bytes.Buffer
	prevWriter := log.Writer()
	prevFlags := log.Flags()
	prevPrefix := log.Prefix()
	log.SetOutput(&output)
	log.SetFlags(0)
	log.SetPrefix("")
	defer func() {
		log.SetOutput(prevWriter)
		log.SetFlags(prevFlags)
		log.SetPrefix(prevPrefix)
	}()

	req := httptest.NewRequest(http.MethodPost, "http://localhost/assistant-ui/assets", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req.Header.Set("X-Request-ID", "req-1")
	req.Header.Set("traceparent", "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01")
	assistantUIProxyLog(req, "method_not_allowed")
	if got := output.String(); !strings.Contains(got, "tenant_id=t1") || !strings.Contains(got, "request_id=req-1") || !strings.Contains(got, "trace_id=0123456789abcdef0123456789abcdef") {
		t.Fatalf("unexpected log output=%q", got)
	}
	if got := output.String(); !strings.Contains(got, "path=/assistant-ui/assets") || !strings.Contains(got, "method=POST") || !strings.Contains(got, "reason=method_not_allowed") {
		t.Fatalf("unexpected log output=%q", got)
	}

	output.Reset()
	req2 := httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui", nil)
	assistantUIProxyLog(req2, "upstream_unreachable")
	if got := output.String(); !strings.Contains(got, "tenant_id=-") || !strings.Contains(got, "request_id=-") || !strings.Contains(got, "trace_id=-") {
		t.Fatalf("expected default placeholders, got=%q", got)
	}
}

func TestAssistantUIProxyTraceID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui", nil)
	if got := assistantUIProxyTraceID(req); got != "" {
		t.Fatalf("trace id should be empty, got=%q", got)
	}

	req.Header.Set("traceparent", "bad")
	if got := assistantUIProxyTraceID(req); got != "" {
		t.Fatalf("trace id should be empty for malformed input, got=%q", got)
	}

	req.Header.Set("traceparent", "00-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bbbbbbbbbbbbbbbb-01")
	if got := assistantUIProxyTraceID(req); got != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("trace id mismatch: %q", got)
	}
}

func TestSanitizeAssistantUIProxyRequestCookieHeader(t *testing.T) {
	if got := sanitizeAssistantUIProxyRequestCookieHeader(nil); got != "" {
		t.Fatalf("expected empty cookie header for nil request, got=%q", got)
	}
	reqEmpty := httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui", nil)
	if got := sanitizeAssistantUIProxyRequestCookieHeader(reqEmpty); got != "" {
		t.Fatalf("expected empty cookie header for request without cookies, got=%q", got)
	}
	req := httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui", nil)
	req.Header.Set("Cookie", "sid=local; refreshToken=rf-1; token_provider=librechat; openid_user_id=oid-1; token_provider=")
	got := sanitizeAssistantUIProxyRequestCookieHeader(req)
	if strings.Contains(got, "sid=local") {
		t.Fatalf("unexpected sid in sanitized cookies: %q", got)
	}
	if !strings.Contains(got, "refreshToken=rf-1") || !strings.Contains(got, "token_provider=librechat") || !strings.Contains(got, "openid_user_id=oid-1") {
		t.Fatalf("missing expected auth cookies: %q", got)
	}
}

func TestSanitizeAssistantUIProxyAuthorizationHeader(t *testing.T) {
	if got := sanitizeAssistantUIProxyAuthorizationHeader(nil); got != "" {
		t.Fatalf("expected empty auth header for nil request, got=%q", got)
	}
	req := httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui", nil)
	if got := sanitizeAssistantUIProxyAuthorizationHeader(req); got != "" {
		t.Fatalf("empty auth header should be dropped, got=%q", got)
	}
	req.Header.Set("Authorization", "Basic abc")
	if got := sanitizeAssistantUIProxyAuthorizationHeader(req); got != "" {
		t.Fatalf("non-bearer auth should be dropped, got=%q", got)
	}
	req.Header.Set("Authorization", "Bearer")
	if got := sanitizeAssistantUIProxyAuthorizationHeader(req); got != "" {
		t.Fatalf("malformed bearer auth should be dropped, got=%q", got)
	}
	req.Header.Set("Authorization", "Bearer token-1 extra")
	if got := sanitizeAssistantUIProxyAuthorizationHeader(req); got != "" {
		t.Fatalf("multi-part bearer auth should be dropped, got=%q", got)
	}
	req.Header.Set("Authorization", "Bearer token-1")
	if got := sanitizeAssistantUIProxyAuthorizationHeader(req); got != "Bearer token-1" {
		t.Fatalf("bearer auth should be normalized, got=%q", got)
	}
}

func TestFilterAssistantUIProxyResponseCookies(t *testing.T) {
	filterAssistantUIProxyResponseCookies(nil)

	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Add("Set-Cookie", "sid=local; Path=/; HttpOnly")
	resp.Header.Add("Set-Cookie", "refreshToken=rf-1; Path=/librechat; HttpOnly")
	resp.Header.Add("Set-Cookie", "token_provider=librechat; Path=/; HttpOnly")
	filterAssistantUIProxyResponseCookies(resp)

	cookies := resp.Cookies()
	if len(cookies) != 2 {
		t.Fatalf("expected 2 allowed cookies, got=%d cookies=%+v", len(cookies), cookies)
	}
	for _, cookie := range cookies {
		if cookie.Name == "refreshToken" && cookie.Path != "/librechat" {
			t.Fatalf("expected custom path to be preserved, got cookie=%+v", cookie)
		}
		if cookie.Name == "token_provider" && cookie.Path != "/assistant-ui" {
			t.Fatalf("expected root path to be rewritten, got cookie=%+v", cookie)
		}
	}
}

type assistantUIProxyErrReadCloser struct{}

func (assistantUIProxyErrReadCloser) Read([]byte) (int, error) { return 0, errors.New("read-failed") }
func (assistantUIProxyErrReadCloser) Close() error             { return nil }

func TestRewriteAssistantUIProxyHTMLBase(t *testing.T) {
	body := `<!DOCTYPE html><html><head><base href="/" /><title>x</title></head><body>ok</body></html>`
	resp := &http.Response{
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
	}
	resp.Header.Set("Content-Type", "text/html; charset=utf-8")
	resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
	if err := rewriteAssistantUIProxyHTMLBase(resp); err != nil {
		t.Fatalf("rewriteAssistantUIProxyHTMLBase returned error: %v", err)
	}
	rewritten, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read rewritten body: %v", err)
	}
	got := string(rewritten)
	if !strings.Contains(got, `<base href="/assistant-ui/" />`) {
		t.Fatalf("expected base href rewrite, got=%q", got)
	}
	if strings.Contains(got, `<base href="/" />`) {
		t.Fatalf("unexpected old base href remains: %q", got)
	}
	if resp.ContentLength != int64(len(rewritten)) {
		t.Fatalf("unexpected content length: %d", resp.ContentLength)
	}
	if resp.Header.Get("Content-Length") != strconv.Itoa(len(rewritten)) {
		t.Fatalf("unexpected header content-length=%q", resp.Header.Get("Content-Length"))
	}
}

func TestRewriteAssistantUIProxyHTMLBase_NoopAndErrorBranches(t *testing.T) {
	if err := rewriteAssistantUIProxyHTMLBase(nil); err != nil {
		t.Fatalf("nil response should not return error: %v", err)
	}

	respBodyNil := &http.Response{Header: make(http.Header)}
	if err := rewriteAssistantUIProxyHTMLBase(respBodyNil); err != nil {
		t.Fatalf("response with nil body should not return error: %v", err)
	}

	nonHTML := &http.Response{
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(`{"ok":true}`)),
		ContentLength: int64(len(`{"ok":true}`)),
	}
	nonHTML.Header.Set("Content-Type", "application/json")
	if err := rewriteAssistantUIProxyHTMLBase(nonHTML); err != nil {
		t.Fatalf("non-html response should not return error: %v", err)
	}

	noBaseHTML := `<!DOCTYPE html><html><head><title>x</title></head><body>ok</body></html>`
	noBaseResp := &http.Response{
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(noBaseHTML)),
		ContentLength: int64(len(noBaseHTML)),
	}
	noBaseResp.Header.Set("Content-Type", "text/html")
	noBaseResp.Header.Set("Content-Length", strconv.Itoa(len(noBaseHTML)))
	if err := rewriteAssistantUIProxyHTMLBase(noBaseResp); err != nil {
		t.Fatalf("no-base html should not return error: %v", err)
	}
	noBaseBody, err := io.ReadAll(noBaseResp.Body)
	if err != nil {
		t.Fatalf("read no-base html body: %v", err)
	}
	if string(noBaseBody) != noBaseHTML {
		t.Fatalf("no-base html should remain unchanged, got=%q", string(noBaseBody))
	}

	singleQuoteHTML := `<!DOCTYPE html><html><head><base href='/' /></head><body>ok</body></html>`
	singleQuoteResp := &http.Response{
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(singleQuoteHTML)),
		ContentLength: int64(len(singleQuoteHTML)),
	}
	singleQuoteResp.Header.Set("Content-Type", "text/html")
	if err := rewriteAssistantUIProxyHTMLBase(singleQuoteResp); err != nil {
		t.Fatalf("single quote base rewrite should not return error: %v", err)
	}
	singleQuoteBody, err := io.ReadAll(singleQuoteResp.Body)
	if err != nil {
		t.Fatalf("read single-quote html body: %v", err)
	}
	if !strings.Contains(string(singleQuoteBody), `<base href='/assistant-ui/' />`) {
		t.Fatalf("single quote base should be rewritten, got=%q", string(singleQuoteBody))
	}

	readErrResp := &http.Response{
		Header: make(http.Header),
		Body:   assistantUIProxyErrReadCloser{},
	}
	readErrResp.Header.Set("Content-Type", "text/html")
	if err := rewriteAssistantUIProxyHTMLBase(readErrResp); err == nil {
		t.Fatal("expected read error to be returned")
	}
}

func decodeAssistantUIProxyErrorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var envelope routing.ErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode error envelope: %v body=%s", err, rec.Body.String())
	}
	return envelope.Code
}
