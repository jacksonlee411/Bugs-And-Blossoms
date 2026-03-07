package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestAssistantUIProxyHelperCoverage(t *testing.T) {
	if assistantUIProxyShouldBootstrapAuth(nil) {
		t.Fatal("nil request should not bootstrap")
	}
	if assistantUIProxyIsLoginPath(nil) {
		t.Fatal("nil request should not be login path")
	}
	req := httptest.NewRequest(http.MethodPost, "/assistant-ui", nil)
	if assistantUIProxyShouldBootstrapAuth(req) {
		t.Fatal("post should not bootstrap")
	}
	req = httptest.NewRequest(http.MethodGet, "/assistant-ui/other", nil)
	if assistantUIProxyShouldBootstrapAuth(req) {
		t.Fatal("other path should not bootstrap")
	}
	req = httptest.NewRequest(http.MethodGet, "/assistant-ui/login", nil)
	if !assistantUIProxyShouldBootstrapAuth(req) || !assistantUIProxyIsLoginPath(req) {
		t.Fatal("login path should bootstrap")
	}
	if got := assistantUIBootstrapShortSlug("", 8); got != "x" {
		t.Fatalf("slug=%q", got)
	}
	if got := assistantUIBootstrapShortSlug("ABC.DEF", 3); got != "abc" {
		t.Fatalf("slug=%q", got)
	}
	cookieReq := httptest.NewRequest(http.MethodGet, "/", nil)
	cookieReq.AddCookie(&http.Cookie{Name: "refreshToken", Value: "old"})
	assistantUIApplyAuthCookiesToRequest(cookieReq, []*http.Cookie{{Name: "refreshToken", Value: "new"}, {Name: "token_provider", Value: "librechat"}, {Name: "bad", Value: "x"}})
	if got := cookieReq.Header.Get("Cookie"); !strings.Contains(got, "refreshToken=new") || !strings.Contains(got, "token_provider=librechat") {
		t.Fatalf("cookie header=%q", got)
	}
	filtered := assistantUIFilterAllowedAuthCookies([]*http.Cookie{{Name: "refreshToken", Value: "a"}, {Name: "bad", Value: "b"}, nil})
	if len(filtered) != 1 || filtered[0].Name != "refreshToken" {
		t.Fatalf("filtered=%+v", filtered)
	}
	ctx := context.WithValue(context.Background(), assistantUIBootstrapCookiesCtxKey{}, []*http.Cookie{{Name: "refreshToken", Value: "a"}, {Name: "bad", Value: "b"}})
	fromCtx := assistantUIBootstrapCookiesFromContext(ctx)
	if len(fromCtx) != 1 || fromCtx[0].Name != "refreshToken" {
		t.Fatalf("fromCtx=%+v", fromCtx)
	}
	if !assistantUIProxyShouldServeFallbackShell(httptest.NewRequest(http.MethodGet, "/assistant-ui", nil)) {
		t.Fatal("html get should serve fallback shell")
	}
	jsonReq := httptest.NewRequest(http.MethodGet, "/assistant-ui", nil)
	jsonReq.Header.Set("Accept", "application/json")
	if assistantUIProxyShouldServeFallbackShell(jsonReq) {
		t.Fatal("json accept should not serve fallback shell")
	}
	if !assistantUIProxyLooksLikeHTML([]byte("<html><body>x</body></html>")) || assistantUIProxyLooksLikeHTML([]byte("not html")) {
		t.Fatal("unexpected html detection")
	}
}

func TestAssistantUIRefreshUpstreamSessionBranches(t *testing.T) {
	oldClient := assistantUIBootstrapHTTPClient
	defer func() { assistantUIBootstrapHTTPClient = oldClient }()
	targetURL, _ := url.Parse("https://assistant.local")
	baseCookies := []*http.Cookie{{Name: "refreshToken", Value: "rf"}}

	assistantUIBootstrapHTTPClient = &http.Client{Transport: assistantRoundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("boom")
	})}
	if _, _, err := assistantUIRefreshUpstreamSession(context.Background(), targetURL, baseCookies); err == nil {
		t.Fatal("expected error")
	}

	assistantUIBootstrapHTTPClient = &http.Client{Transport: assistantRoundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`ok`)), Header: make(http.Header)}, nil
	})}
	cookies, ok, err := assistantUIRefreshUpstreamSession(context.Background(), targetURL, baseCookies)
	if err != nil || !ok || len(cookies) != 1 || cookies[0].Value != "rf" {
		t.Fatalf("cookies=%+v ok=%v err=%v", cookies, ok, err)
	}

	assistantUIBootstrapHTTPClient = &http.Client{Transport: assistantRoundTripperFunc(func(*http.Request) (*http.Response, error) {
		resp := &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader(`no`)), Header: make(http.Header)}
		return resp, nil
	})}
	if cookies, ok, err := assistantUIRefreshUpstreamSession(context.Background(), targetURL, baseCookies); err != nil || ok || cookies != nil {
		t.Fatalf("cookies=%+v ok=%v err=%v", cookies, ok, err)
	}

	assistantUIBootstrapHTTPClient = &http.Client{Transport: assistantRoundTripperFunc(func(*http.Request) (*http.Response, error) {
		resp := &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(strings.NewReader(`x`)), Header: make(http.Header)}
		return resp, nil
	})}
	if _, _, err := assistantUIRefreshUpstreamSession(context.Background(), targetURL, baseCookies); err == nil {
		t.Fatal("expected refresh status error")
	}
}

func TestAssistantUIProxyBootstrapRequestBranches(t *testing.T) {
	targetURL, _ := url.Parse("https://assistant.local")
	req := httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui", nil)
	proxyReq, cookies, err := assistantUIProxyBootstrapRequest(req, targetURL)
	if err != nil || proxyReq != req || cookies != nil {
		t.Fatalf("proxyReq=%v cookies=%v err=%v", proxyReq, cookies, err)
	}

	req = httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-1"}))
	proxyReq, cookies, err = assistantUIProxyBootstrapRequest(req, targetURL)
	if err != nil || proxyReq != req || cookies != nil {
		t.Fatalf("proxyReq=%v cookies=%v err=%v", proxyReq, cookies, err)
	}
}
