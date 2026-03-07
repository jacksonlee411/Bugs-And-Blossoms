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

func TestAssistantUIProxyBootstrapRequestAndEnsureAuthCoverage(t *testing.T) {
	oldClient := assistantUIBootstrapHTTPClient
	defer func() { assistantUIBootstrapHTTPClient = oldClient }()
	targetURL, _ := url.Parse("https://assistant.local")
	tenant := Tenant{ID: "tenant-1"}
	principal := Principal{ID: "principal-1", Email: "user@example.com", KratosIdentityID: "kratos-1"}

	t.Run("bootstrap success clones request and injects cookies", func(t *testing.T) {
		assistantUIBootstrapHTTPClient = &http.Client{Transport: assistantRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/api/auth/login" {
				t.Fatalf("path=%s", req.URL.Path)
			}
			resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`ok`)), Header: make(http.Header)}
			resp.Header.Add("Set-Cookie", (&http.Cookie{Name: "refreshToken", Value: "rf-1", Path: "/"}).String())
			resp.Header.Add("Set-Cookie", (&http.Cookie{Name: "token_provider", Value: "librechat", Path: "/"}).String())
			return resp, nil
		})}
		req := httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui/login", nil)
		req = req.WithContext(withPrincipal(withTenant(req.Context(), tenant), principal))
		proxyReq, cookies, err := assistantUIProxyBootstrapRequest(req, targetURL)
		if err != nil || proxyReq == req || len(cookies) != 2 {
			t.Fatalf("proxyReq=%v cookies=%+v err=%v", proxyReq, cookies, err)
		}
		if got := proxyReq.Header.Get("Cookie"); !strings.Contains(got, "refreshToken=rf-1") || !strings.Contains(got, "token_provider=librechat") {
			t.Fatalf("cookie header=%q", got)
		}
	})

	t.Run("refresh success short-circuits login", func(t *testing.T) {
		calls := 0
		assistantUIBootstrapHTTPClient = &http.Client{Transport: assistantRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			if req.URL.Path != "/api/auth/refresh" {
				t.Fatalf("unexpected path=%s", req.URL.Path)
			}
			resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`ok`)), Header: make(http.Header)}
			resp.Header.Add("Set-Cookie", (&http.Cookie{Name: "refreshToken", Value: "rf-2", Path: "/"}).String())
			return resp, nil
		})}
		cookies, err := assistantUIEnsureUpstreamAuth(context.Background(), targetURL, tenant, principal, []*http.Cookie{{Name: "refreshToken", Value: "rf-1"}})
		if err != nil || calls != 1 || len(cookies) != 1 || cookies[0].Value != "rf-2" {
			t.Fatalf("cookies=%+v calls=%d err=%v", cookies, calls, err)
		}
	})

	t.Run("unexpected login status fails closed", func(t *testing.T) {
		assistantUIBootstrapHTTPClient = &http.Client{Transport: assistantRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(strings.NewReader(`x`)), Header: make(http.Header)}, nil
		})}
		if _, err := assistantUIEnsureUpstreamAuth(context.Background(), targetURL, tenant, principal, nil); err == nil {
			t.Fatal("expected login status error")
		}
	})

	t.Run("register then second login failure surfaces error", func(t *testing.T) {
		calls := 0
		assistantUIBootstrapHTTPClient = &http.Client{Transport: assistantRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			switch calls {
			case 1:
				return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader(`x`)), Header: make(http.Header)}, nil
			case 2:
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`ok`)), Header: make(http.Header)}, nil
			default:
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`ok`)), Header: make(http.Header)}, nil
			}
		})}
		if _, err := assistantUIEnsureUpstreamAuth(context.Background(), targetURL, tenant, principal, nil); err == nil {
			t.Fatal("expected missing cookies after register/login to fail")
		}
	})
}

func TestAssistantUIProxyHelperMoreCoverage(t *testing.T) {
	targetURL, _ := url.Parse("https://assistant.local/base")
	if _, _, err := assistantUIProxyJSONRequest(context.Background(), targetURL, http.MethodPost, "/api/auth/login", map[string]any{"bad": make(chan int)}, nil); err == nil {
		t.Fatal("expected marshal error")
	}
	if _, _, err := assistantUIProxyJSONRequest(context.Background(), targetURL, "\n", "/api/auth/login", nil, nil); err == nil {
		t.Fatal("expected request build error")
	}

	creds := assistantUIBootstrapCredentialSet(Tenant{}, Principal{KratosIdentityID: strings.Repeat("k", 40)})
	if !strings.Contains(creds.Name, "Assistant tenant principal") || len(creds.Username) > 80 || !strings.Contains(creds.Username, "bb_tenant_principal") {
		t.Fatalf("creds=%+v", creds)
	}

	cookieReq := httptest.NewRequest(http.MethodGet, "/", nil)
	cookieReq.AddCookie(&http.Cookie{Name: "refreshToken", Value: ""})
	assistantUIApplyAuthCookiesToRequest(cookieReq, []*http.Cookie{{Name: "refreshToken", Value: "new"}, {Name: "openid_user_id", Value: "uid"}, {Name: "token_provider", Value: ""}})
	if got := cookieReq.Header.Get("Cookie"); !strings.Contains(got, "refreshToken=new") || !strings.Contains(got, "openid_user_id=uid") || strings.Contains(got, "token_provider=") {
		t.Fatalf("cookie header=%q", got)
	}

	filtered := assistantUIFilterAllowedAuthCookies([]*http.Cookie{{Name: "refreshToken", Value: "a"}, {Name: "token_provider", Value: ""}, {Name: "bad", Value: "b"}, nil})
	if len(filtered) != 1 || filtered[0].Name != "refreshToken" {
		t.Fatalf("filtered=%+v", filtered)
	}
	if got := assistantUIBootstrapCookiesFromContext(nil); got != nil {
		t.Fatalf("expected nil ctx cookies, got=%+v", got)
	}
	ctx := context.WithValue(context.Background(), assistantUIBootstrapCookiesCtxKey{}, "bad")
	if got := assistantUIBootstrapCookiesFromContext(ctx); got != nil {
		t.Fatalf("expected nil typed cookies, got=%+v", got)
	}

	normalized := assistantUINormalizeProxyResponseCookies([]*http.Cookie{{Name: "refreshToken", Value: "rf", Path: "/"}, {Name: "token_provider", Value: "librechat", Path: "/chat"}, {Name: "openid_user_id", Value: ""}, nil})
	if len(normalized) != 2 || normalized[0].Path != "/assistant-ui" || normalized[1].Path != "/chat" {
		t.Fatalf("normalized=%+v", normalized)
	}

	if assistantUIProxyShouldServeFallbackShell(nil) {
		t.Fatal("nil request should not serve fallback shell")
	}
	headReq := httptest.NewRequest(http.MethodHead, "/assistant-ui", nil)
	if !assistantUIProxyShouldServeFallbackShell(headReq) {
		t.Fatal("head request should serve fallback shell")
	}
	subReq := httptest.NewRequest(http.MethodGet, "/assistant-ui/assets/app.js", nil)
	if assistantUIProxyShouldServeFallbackShell(subReq) {
		t.Fatal("sub path should not serve fallback shell")
	}
	fallbackRec := httptest.NewRecorder()
	serveAssistantUIFallbackShell(fallbackRec, headReq, http.StatusBadGateway)
	if fallbackRec.Code != http.StatusOK || fallbackRec.Body.Len() != 0 {
		t.Fatalf("status=%d body=%q", fallbackRec.Code, fallbackRec.Body.String())
	}

	longHTML := []byte("<" + strings.Repeat("a", 1100) + "body")
	if assistantUIProxyLooksLikeHTML(longHTML) {
		t.Fatal("long non-html snippet should not be treated as html")
	}
	longDoc := []byte("<!doctype html>" + strings.Repeat("a", 1100))
	if !assistantUIProxyLooksLikeHTML(longDoc) {
		t.Fatal("doctype html should be detected")
	}
}

func TestAssistantUIProxyHandlerBootstrapFailureAndCredentialTruncation(t *testing.T) {
	oldClient := assistantUIBootstrapHTTPClient
	defer func() { assistantUIBootstrapHTTPClient = oldClient }()
	assistantUIBootstrapHTTPClient = &http.Client{Transport: assistantRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return nil, io.ErrUnexpectedEOF
	})}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "ok") }))
	defer upstream.Close()
	t.Setenv("LIBRECHAT_UPSTREAM", upstream.URL)
	h := newAssistantUIProxyHandler()
	req := httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui/login", nil)
	req = req.WithContext(withPrincipal(withTenant(req.Context(), Tenant{ID: "tenant-1"}), Principal{ID: "principal-1", RoleSlug: "tenant-admin"}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Result().Header.Get("Location"); loc != libreChatFormalEntryPrefix {
		t.Fatalf("location=%q", loc)
	}

	creds := assistantUIBootstrapCredentialSet(Tenant{ID: strings.Repeat("tenant", 20)}, Principal{ID: strings.Repeat("principal", 20), KratosIdentityID: strings.Repeat("kratos", 20)})
	if len(creds.Username) > 80 {
		t.Fatalf("username length=%d creds=%+v", len(creds.Username), creds)
	}

	assistantUIApplyAuthCookiesToRequest(nil, []*http.Cookie{{Name: "refreshToken", Value: "rf"}})
}

func TestAssistantUIEnsureUpstreamAuthRegisterFailureBranches(t *testing.T) {
	oldClient := assistantUIBootstrapHTTPClient
	defer func() { assistantUIBootstrapHTTPClient = oldClient }()
	targetURL, _ := url.Parse("https://assistant.local")
	tenant := Tenant{ID: "tenant-1"}
	principal := Principal{ID: "principal-1", Email: "user@example.com"}

	assistantUIBootstrapHTTPClient = &http.Client{Transport: assistantRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/api/auth/login":
			return &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(strings.NewReader(`x`)), Header: make(http.Header)}, nil
		case "/api/auth/register":
			return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(strings.NewReader(`x`)), Header: make(http.Header)}, nil
		default:
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`ok`)), Header: make(http.Header)}, nil
		}
	})}
	if _, err := assistantUIEnsureUpstreamAuth(context.Background(), targetURL, tenant, principal, nil); err == nil {
		t.Fatal("expected register status failure")
	}
}

func TestAssistantUIProxyBootstrapAndFallbackMoreCoverage(t *testing.T) {
	targetURL, _ := url.Parse("https://assistant.local")
	oldClient := assistantUIBootstrapHTTPClient
	defer func() { assistantUIBootstrapHTTPClient = oldClient }()
	assistantUIBootstrapHTTPClient = &http.Client{Transport: assistantRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader(`x`)), Header: make(http.Header)}, nil
	})}
	req := httptest.NewRequest(http.MethodGet, "http://localhost/assistant-ui/login", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-1"}))
	if proxyReq, cookies, err := assistantUIProxyBootstrapRequest(req, targetURL); err != nil || proxyReq != req || cookies != nil {
		t.Fatalf("proxyReq=%v cookies=%v err=%v", proxyReq, cookies, err)
	}

	jsonHTMLReq := httptest.NewRequest(http.MethodGet, "/assistant-ui", nil)
	jsonHTMLReq.Header.Set("Accept", "application/json, text/html")
	if !assistantUIProxyShouldServeFallbackShell(jsonHTMLReq) {
		t.Fatal("mixed json/html accept should still serve html fallback")
	}
}

func TestAssistantUIProxyMissedBranches(t *testing.T) {
	targetURL, _ := url.Parse("https://assistant.local")
	oldClient := assistantUIBootstrapHTTPClient
	defer func() { assistantUIBootstrapHTTPClient = oldClient }()
	tenant := Tenant{ID: "tenant-1"}
	principal := Principal{ID: "principal-1", Email: "user@example.com"}

	assistantUIBootstrapHTTPClient = &http.Client{Transport: assistantRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/refresh") {
			return nil, errors.New("refresh failed")
		}
		return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader(`x`)), Header: make(http.Header)}, nil
	})}
	if _, err := assistantUIEnsureUpstreamAuth(context.Background(), targetURL, tenant, principal, []*http.Cookie{{Name: "refreshToken", Value: "rf"}}); err == nil {
		t.Fatal("expected refresh error")
	}

	assistantUIBootstrapHTTPClient = &http.Client{Transport: assistantRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/register") {
			return nil, errors.New("register failed")
		}
		return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader(`x`)), Header: make(http.Header)}, nil
	})}
	if _, err := assistantUIEnsureUpstreamAuth(context.Background(), targetURL, tenant, principal, nil); err == nil {
		t.Fatal("expected register error")
	}

	calls := 0
	assistantUIBootstrapHTTPClient = &http.Client{Transport: assistantRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		switch calls {
		case 1:
			return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader(`x`)), Header: make(http.Header)}, nil
		case 2:
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`ok`)), Header: make(http.Header)}, nil
		default:
			return nil, errors.New("second login failed")
		}
	})}
	if _, err := assistantUIEnsureUpstreamAuth(context.Background(), targetURL, tenant, principal, nil); err == nil {
		t.Fatal("expected second login error")
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "bad", Value: "1"})
	assistantUIApplyAuthCookiesToRequest(req, nil)
	if got := req.Header.Get("Cookie"); got != "" {
		t.Fatalf("cookie=%q", got)
	}
	if assistantUIProxyShouldServeFallbackShell(httptest.NewRequest(http.MethodPost, "/assistant-ui", nil)) {
		t.Fatal("post should not fallback shell")
	}
}
