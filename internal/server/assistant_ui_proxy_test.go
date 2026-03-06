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
		bridgeReq := httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui/bridge.js", nil)
		bridgeRec := httptest.NewRecorder()
		h.ServeHTTP(bridgeRec, bridgeReq)
		if bridgeRec.Code != http.StatusOK {
			t.Fatalf("bridge status=%d body=%s", bridgeRec.Code, bridgeRec.Body.String())
		}
		if !strings.Contains(bridgeRec.Body.String(), "assistant.prompt.sync") {
			t.Fatalf("expected assistant bridge script body, got=%q", bridgeRec.Body.String())
		}
		if !strings.Contains(bridgeRec.Body.String(), "assistant.flow.dialog") {
			t.Fatalf("expected assistant.flow.dialog support in bridge script, got=%q", bridgeRec.Body.String())
		}
		if !strings.Contains(bridgeRec.Body.String(), "#messages-view") {
			t.Fatalf("bridge script should include messages-view selector for chat root, got=%q", bridgeRec.Body.String())
		}
		if strings.Contains(bridgeRec.Body.String(), "assistant-flow-notice-layer") {
			t.Fatalf("bridge script should no longer rely on notice layer overlay, got=%q", bridgeRec.Body.String())
		}
		if strings.Contains(bridgeRec.Body.String(), "return document.body") {
			t.Fatalf("bridge script must not fall back to document.body for dialog root, got=%q", bridgeRec.Body.String())
		}
		if !strings.Contains(bridgeRec.Body.String(), "assistant.bridge.render_error") {
			t.Fatalf("bridge script should emit render_error when dialog root is missing, got=%q", bridgeRec.Body.String())
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
		bridgePutReq := httptest.NewRequest(http.MethodPut, "http://localhost/assistant-ui/bridge.js", nil)
		bridgePutReq.Header.Set("Accept", "application/json")
		bridgePutRec := httptest.NewRecorder()
		h.ServeHTTP(bridgePutRec, bridgePutReq)
		if bridgePutRec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("bridge put status=%d body=%s", bridgePutRec.Code, bridgePutRec.Body.String())
		}
		if got := decodeAssistantUIProxyErrorCode(t, bridgePutRec); got != assistantUIProxyMethodNotAllowedCode {
			t.Fatalf("bridge put code=%s", got)
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
	req.Header.Set("Authorization", "bearer token-2")
	if got := sanitizeAssistantUIProxyAuthorizationHeader(req); got != "Bearer token-2" {
		t.Fatalf("bearer auth should be case-insensitive, got=%q", got)
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

func TestServeAssistantUIBridgeScript(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui/bridge.js", nil)
	rec := httptest.NewRecorder()
	serveAssistantUIBridgeScript(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "assistant.prompt.sync") {
		t.Fatalf("unexpected bridge body=%q", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "assistant.flow.dialog") {
		t.Fatalf("expected assistant.flow.dialog support in bridge script, got=%q", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "#messages-view") {
		t.Fatalf("bridge script should include messages-view selector for chat root, got=%q", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "assistant-flow-notice-layer") {
		t.Fatalf("bridge script should no longer rely on notice layer overlay, got=%q", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "return document.body") {
		t.Fatalf("bridge script must not fall back to document.body for dialog root, got=%q", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "assistant.bridge.render_error") {
		t.Fatalf("bridge script should emit render_error when dialog root is missing, got=%q", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/javascript") {
		t.Fatalf("unexpected content-type=%q", ct)
	}

	headReq := httptest.NewRequest(http.MethodHead, "http://localhost/assistant-ui/bridge.js", nil)
	headRec := httptest.NewRecorder()
	serveAssistantUIBridgeScript(headRec, headReq)
	if headRec.Code != http.StatusOK {
		t.Fatalf("head status=%d", headRec.Code)
	}
	if headRec.Body.Len() != 0 {
		t.Fatalf("head response body should be empty, got=%q", headRec.Body.String())
	}

	postReq := httptest.NewRequest(http.MethodPost, "http://localhost/assistant-ui/bridge.js", nil)
	postReq.Header.Set("Accept", "application/json")
	postRec := httptest.NewRecorder()
	serveAssistantUIBridgeScript(postRec, postReq)
	if postRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("post status=%d body=%s", postRec.Code, postRec.Body.String())
	}
	if got := decodeAssistantUIProxyErrorCode(t, postRec); got != assistantUIProxyMethodNotAllowedCode {
		t.Fatalf("post code=%s", got)
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
	if !strings.Contains(got, assistantUIBridgeScriptTag) {
		t.Fatalf("expected bridge script tag injection, got=%q", got)
	}
	if !strings.Contains(got, assistantUISWCleanupScriptTag) {
		t.Fatalf("expected sw cleanup script tag injection, got=%q", got)
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

	mislabeledJSON := `{"openAI":{"order":0}}`
	mislabeledJSONResp := &http.Response{
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(mislabeledJSON)),
		ContentLength: int64(len(mislabeledJSON)),
	}
	mislabeledJSONResp.Header.Set("Content-Type", "text/html; charset=utf-8")
	mislabeledJSONResp.Header.Set("Content-Length", strconv.Itoa(len(mislabeledJSON)))
	if err := rewriteAssistantUIProxyHTMLBase(mislabeledJSONResp); err != nil {
		t.Fatalf("mislabeled json response should not return error: %v", err)
	}
	mislabeledJSONBody, err := io.ReadAll(mislabeledJSONResp.Body)
	if err != nil {
		t.Fatalf("read mislabeled json body: %v", err)
	}
	if got := string(mislabeledJSONBody); got != mislabeledJSON {
		t.Fatalf("mislabeled json should remain unchanged, got=%q", got)
	}
	if strings.Contains(string(mislabeledJSONBody), assistantUIBridgeScriptTag) {
		t.Fatalf("mislabeled json should not include bridge script, got=%q", string(mislabeledJSONBody))
	}
	if strings.Contains(string(mislabeledJSONBody), assistantUISWCleanupScriptTag) {
		t.Fatalf("mislabeled json should not include sw cleanup script, got=%q", string(mislabeledJSONBody))
	}
	if got := mislabeledJSONResp.Header.Get("Content-Length"); got != strconv.Itoa(len(mislabeledJSON)) {
		t.Fatalf("mislabeled json content-length should remain original, got=%q", got)
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
	if !strings.Contains(string(noBaseBody), assistantUIBridgeScriptTag) {
		t.Fatalf("no-base html should include bridge script tag, got=%q", string(noBaseBody))
	}
	if !strings.Contains(string(noBaseBody), assistantUISWCleanupScriptTag) {
		t.Fatalf("no-base html should include sw cleanup script tag, got=%q", string(noBaseBody))
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
	if !strings.Contains(string(singleQuoteBody), assistantUIBridgeScriptTag) {
		t.Fatalf("single quote html should include bridge script tag, got=%q", string(singleQuoteBody))
	}
	if !strings.Contains(string(singleQuoteBody), assistantUISWCleanupScriptTag) {
		t.Fatalf("single quote html should include sw cleanup script tag, got=%q", string(singleQuoteBody))
	}

	withPwaRegisterScript := `<!DOCTYPE html><html><head><script id="vite-plugin-pwa:register-sw" src="./registerSW.js"></script></head><body>ok</body></html>`
	withPwaRegisterResp := &http.Response{
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(withPwaRegisterScript)),
		ContentLength: int64(len(withPwaRegisterScript)),
	}
	withPwaRegisterResp.Header.Set("Content-Type", "text/html")
	if err := rewriteAssistantUIProxyHTMLBase(withPwaRegisterResp); err != nil {
		t.Fatalf("pwa register script rewrite should not return error: %v", err)
	}
	withPwaRegisterBody, err := io.ReadAll(withPwaRegisterResp.Body)
	if err != nil {
		t.Fatalf("read pwa register rewrite body: %v", err)
	}
	withPwaRegisterText := string(withPwaRegisterBody)
	if strings.Contains(withPwaRegisterText, `vite-plugin-pwa:register-sw`) {
		t.Fatalf("pwa register script should be removed, got=%q", withPwaRegisterText)
	}
	if !strings.Contains(withPwaRegisterText, assistantUISWCleanupScriptTag) {
		t.Fatalf("pwa register rewrite should include sw cleanup script, got=%q", withPwaRegisterText)
	}

	alreadyInjectedHTML := `<!DOCTYPE html><html><head><title>x</title>` + assistantUISWCleanupScriptTag + assistantUIBridgeScriptTag + `</head><body>ok</body></html>`
	alreadyInjectedResp := &http.Response{
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(alreadyInjectedHTML)),
		ContentLength: int64(len(alreadyInjectedHTML)),
	}
	alreadyInjectedResp.Header.Set("Content-Type", "text/html")
	alreadyInjectedResp.Header.Set("Content-Length", strconv.Itoa(len(alreadyInjectedHTML)))
	if err := rewriteAssistantUIProxyHTMLBase(alreadyInjectedResp); err != nil {
		t.Fatalf("already injected html should not return error: %v", err)
	}
	alreadyInjectedBody, err := io.ReadAll(alreadyInjectedResp.Body)
	if err != nil {
		t.Fatalf("read already-injected html body: %v", err)
	}
	if string(alreadyInjectedBody) != alreadyInjectedHTML {
		t.Fatalf("already injected html should remain unchanged, got=%q", string(alreadyInjectedBody))
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

func TestModifyAssistantUIProxyResponse(t *testing.T) {
	resp := &http.Response{
		Header: make(http.Header),
		Body: io.NopCloser(strings.NewReader(
			`<!doctype html><html><head><base href="/" /></head><body>x</body></html>`,
		)),
	}
	resp.Header.Set("Content-Type", "text/html")
	resp.Header.Add("Set-Cookie", "refreshToken=rf-1; Path=/; HttpOnly")
	resp.Header.Add("Set-Cookie", "sid=local; Path=/; HttpOnly")
	if err := modifyAssistantUIProxyResponse(resp); err != nil {
		t.Fatalf("modifyAssistantUIProxyResponse returned error: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read modified body: %v", err)
	}
	if !strings.Contains(string(body), `<base href="/assistant-ui/" />`) {
		t.Fatalf("expected html rewrite, got=%q", string(body))
	}
	cookies := resp.Cookies()
	if len(cookies) != 1 || cookies[0].Name != "refreshToken" {
		t.Fatalf("expected only refreshToken cookie, got=%+v", cookies)
	}

	respErr := &http.Response{
		Header: make(http.Header),
		Body:   assistantUIProxyErrReadCloser{},
	}
	respErr.Header.Set("Content-Type", "text/html")
	if err := modifyAssistantUIProxyResponse(respErr); err == nil {
		t.Fatal("expected read error from modifyAssistantUIProxyResponse")
	}
}

func TestInjectAssistantUIProxyBridgeScript(t *testing.T) {
	already := `<html><head>` + assistantUIBridgeScriptTag + `</head><body>x</body></html>`
	if got := injectAssistantUIProxyBridgeScript(already); got != already {
		t.Fatalf("already-injected html should remain unchanged, got=%q", got)
	}

	withHead := `<!doctype html><html><head><title>x</title></head><body>x</body></html>`
	gotHead := injectAssistantUIProxyBridgeScript(withHead)
	if !strings.Contains(gotHead, `<head><title>x</title>`+assistantUIBridgeScriptTag+`</head>`) {
		t.Fatalf("expected injection before </head>, got=%q", gotHead)
	}

	withBody := `<html><body>x</body></html>`
	gotBody := injectAssistantUIProxyBridgeScript(withBody)
	if !strings.Contains(gotBody, `x`+assistantUIBridgeScriptTag+`</body>`) {
		t.Fatalf("expected injection before </body>, got=%q", gotBody)
	}

	plain := `<html>x</html>`
	gotPlain := injectAssistantUIProxyBridgeScript(plain)
	if gotPlain != plain+assistantUIBridgeScriptTag {
		t.Fatalf("expected append fallback, got=%q", gotPlain)
	}
}

func TestInjectAssistantUIProxyServiceWorkerCleanupScript(t *testing.T) {
	already := `<html><head>` + assistantUISWCleanupScriptTag + `</head><body>x</body></html>`
	if got := injectAssistantUIProxyServiceWorkerCleanupScript(already); got != already {
		t.Fatalf("already-injected html should remain unchanged, got=%q", got)
	}

	withHead := `<!doctype html><html><head><title>x</title></head><body>x</body></html>`
	gotHead := injectAssistantUIProxyServiceWorkerCleanupScript(withHead)
	if !strings.Contains(gotHead, `<head><title>x</title>`+assistantUISWCleanupScriptTag+`</head>`) {
		t.Fatalf("expected injection before </head>, got=%q", gotHead)
	}

	withBody := `<html><body>x</body></html>`
	gotBody := injectAssistantUIProxyServiceWorkerCleanupScript(withBody)
	if !strings.Contains(gotBody, `x`+assistantUISWCleanupScriptTag+`</body>`) {
		t.Fatalf("expected injection before </body>, got=%q", gotBody)
	}

	plain := `<html>x</html>`
	gotPlain := injectAssistantUIProxyServiceWorkerCleanupScript(plain)
	if gotPlain != plain+assistantUISWCleanupScriptTag {
		t.Fatalf("expected append fallback, got=%q", gotPlain)
	}
}

func TestStripAssistantUIProxyServiceWorkerRegistration(t *testing.T) {
	cases := []string{
		`<script id="vite-plugin-pwa:register-sw" src="./registerSW.js"></script>`,
		`<script src="./registerSW.js" id='vite-plugin-pwa:register-sw' defer></script>`,
		`<SCRIPT ID="vite-plugin-pwa:register-sw" SRC="./registerSW.js"></SCRIPT>`,
	}
	for _, input := range cases {
		got := stripAssistantUIProxyServiceWorkerRegistration(`<html><head>` + input + `</head></html>`)
		if strings.Contains(strings.ToLower(got), "vite-plugin-pwa:register-sw") {
			t.Fatalf("register-sw script should be removed, got=%q", got)
		}
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
