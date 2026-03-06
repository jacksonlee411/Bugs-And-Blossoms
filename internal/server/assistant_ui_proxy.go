package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

const (
	assistantUIProxyMethodNotAllowedCode  = "assistant_ui_method_not_allowed"
	assistantUIProxyPathInvalidCode       = "assistant_ui_path_invalid"
	assistantUIProxyUpstreamUnavailable   = "assistant_ui_upstream_unavailable"
	assistantUIProxyDefaultRequestIDValue = "-"
	assistantUIBridgeScriptPath           = "/assistant-ui/bridge.js"
	assistantUIBridgeScriptTag            = `<script src="/assistant-ui/bridge.js" defer></script>`
	assistantUISWCleanupScriptTag         = `<script data-assistant-ui-sw-cleanup="1">(function(){try{if('serviceWorker'in navigator&&navigator.serviceWorker&&navigator.serviceWorker.getRegistrations){navigator.serviceWorker.getRegistrations().then(function(registrations){registrations.forEach(function(registration){registration.unregister().catch(function(){});});}).catch(function(){});}if(typeof window!=='undefined'&&window.caches&&window.caches.keys){window.caches.keys().then(function(keys){keys.forEach(function(key){window.caches.delete(key).catch(function(){});});}).catch(function(){});}}catch(err){}})();</script>`
)

var assistantUIProxyForbiddenBypassPrefixes = []string{
	"/internal",
	"/iam",
	"/org",
	"/jobcatalog",
	"/person",
	"/staffing",
	"/dicts",
}

var assistantUIRegisterSWScriptTagPattern = regexp.MustCompile(`(?is)<script[^>]*id=["']vite-plugin-pwa:register-sw["'][^>]*>\s*</script>`)
var assistantUIBootstrapSlugPattern = regexp.MustCompile(`[^a-z0-9]+`)

var assistantUIBootstrapHTTPClient = &http.Client{Timeout: 10 * time.Second}

type assistantUIBootstrapCookiesCtxKey struct{}

type assistantUIBootstrapCredentials struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func newAssistantUIProxyHandler() http.Handler {
	targetRaw := assistantRuntimeDefaultUpstreamURL()
	targetURL, err := url.Parse(targetRaw)
	if err != nil || targetURL.Scheme == "" || targetURL.Host == "" {
		return assistantUIUnavailableHandler("LIBRECHAT_UPSTREAM is invalid")
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	basePath := strings.TrimSuffix(targetURL.Path, "/")
	proxyDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		proxyDirector(req)
		filterAssistantUIProxyRequestHeaders(req)
		proxyPath := strings.TrimPrefix(req.URL.Path, "/assistant-ui")
		if proxyPath == "" {
			proxyPath = "/"
		}
		req.URL.Path = joinProxyPath(basePath, proxyPath)
		req.Host = targetURL.Host
		req.Header.Set("X-Forwarded-Prefix", "/assistant-ui")
	}
	proxy.ModifyResponse = modifyAssistantUIProxyResponse
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, _ error) {
		serveAssistantUIUnavailableResponse(w, r, http.StatusBadGateway, "upstream_unreachable")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == assistantUIBridgeScriptPath {
			serveAssistantUIBridgeScript(w, r)
			return
		}
		if !assistantUIProxyMethodAllowed(r.Method) {
			writeAssistantUIProxyError(
				w,
				r,
				http.StatusMethodNotAllowed,
				assistantUIProxyMethodNotAllowedCode,
				"assistant_ui_method_not_allowed",
				"method_not_allowed",
			)
			return
		}
		if r.URL.Path != "/assistant-ui" && !pathHasPrefixSegment(r.URL.Path, "/assistant-ui") {
			writeAssistantUIProxyError(
				w,
				r,
				http.StatusBadRequest,
				assistantUIProxyPathInvalidCode,
				"assistant_ui_path_invalid",
				"path_invalid",
			)
			return
		}
		proxyPath := strings.TrimPrefix(r.URL.Path, "/assistant-ui")
		if assistantUIProxyBypassPathForbidden(proxyPath) {
			writeAssistantUIProxyError(
				w,
				r,
				http.StatusBadRequest,
				assistantUIProxyPathInvalidCode,
				"assistant_ui_path_invalid",
				"path_bypass_forbidden",
			)
			return
		}
		proxyReq, bootstrapCookies, err := assistantUIProxyBootstrapRequest(r, targetURL)
		if err != nil {
			assistantUIProxyLog(r, "upstream_auth_bootstrap_failed:"+err.Error())
			serveAssistantUIUnavailableResponse(w, r, http.StatusBadGateway, "upstream_auth_bootstrap_failed")
			return
		}
		if assistantUIProxyIsLoginPath(proxyReq) && len(bootstrapCookies) > 0 {
			assistantUIWriteProxyResponseCookies(w, bootstrapCookies)
			http.Redirect(w, proxyReq, "/assistant-ui", http.StatusFound)
			return
		}
		proxy.ServeHTTP(w, proxyReq)
	})
}

func assistantUIProxyBootstrapRequest(r *http.Request, targetURL *url.URL) (*http.Request, []*http.Cookie, error) {
	if !assistantUIProxyShouldBootstrapAuth(r) {
		return r, nil, nil
	}
	tenant, ok := currentTenant(r.Context())
	if !ok || strings.TrimSpace(tenant.ID) == "" {
		return r, nil, nil
	}
	principal, ok := currentPrincipal(r.Context())
	if !ok || strings.TrimSpace(principal.ID) == "" {
		return r, nil, nil
	}
	bootstrapCookies, err := assistantUIEnsureUpstreamAuth(r.Context(), targetURL, tenant, principal, r.Cookies())
	if err != nil {
		return nil, nil, err
	}
	if len(bootstrapCookies) == 0 {
		return r, nil, nil
	}
	ctx := context.WithValue(r.Context(), assistantUIBootstrapCookiesCtxKey{}, bootstrapCookies)
	proxyReq := r.Clone(ctx)
	assistantUIApplyAuthCookiesToRequest(proxyReq, bootstrapCookies)
	return proxyReq, bootstrapCookies, nil
}

func assistantUIProxyShouldBootstrapAuth(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	proxyPath := strings.TrimPrefix(r.URL.Path, "/assistant-ui")
	switch proxyPath {
	case "", "/", "/login":
		return true
	default:
		return false
	}
}

func assistantUIProxyIsLoginPath(r *http.Request) bool {
	if r == nil {
		return false
	}
	return strings.TrimPrefix(r.URL.Path, "/assistant-ui") == "/login"
}

func assistantUIEnsureUpstreamAuth(ctx context.Context, targetURL *url.URL, tenant Tenant, principal Principal, requestCookies []*http.Cookie) ([]*http.Cookie, error) {
	existingCookies := assistantUIFilterAllowedAuthCookies(requestCookies)
	if len(existingCookies) > 0 {
		refreshedCookies, ok, err := assistantUIRefreshUpstreamSession(ctx, targetURL, existingCookies)
		if err != nil {
			return nil, err
		}
		if ok {
			return refreshedCookies, nil
		}
	}
	creds := assistantUIBootstrapCredentialSet(tenant, principal)
	loginCookies, loginStatus, err := assistantUILoginUpstreamSession(ctx, targetURL, creds)
	if err != nil {
		return nil, err
	}
	if loginStatus == http.StatusOK && len(loginCookies) > 0 {
		return loginCookies, nil
	}
	if loginStatus != http.StatusBadRequest && loginStatus != http.StatusUnauthorized && loginStatus != http.StatusForbidden && loginStatus != http.StatusNotFound {
		return nil, fmt.Errorf("assistant-ui upstream login status=%d", loginStatus)
	}
	if registerStatus, err := assistantUIRegisterUpstreamSession(ctx, targetURL, creds); err != nil {
		return nil, err
	} else if registerStatus != http.StatusOK {
		return nil, fmt.Errorf("assistant-ui upstream register status=%d", registerStatus)
	}
	loginCookies, loginStatus, err = assistantUILoginUpstreamSession(ctx, targetURL, creds)
	if err != nil {
		return nil, err
	}
	if loginStatus != http.StatusOK || len(loginCookies) == 0 {
		return nil, fmt.Errorf("assistant-ui upstream login after register status=%d", loginStatus)
	}
	return loginCookies, nil
}

func assistantUIRefreshUpstreamSession(ctx context.Context, targetURL *url.URL, cookies []*http.Cookie) ([]*http.Cookie, bool, error) {
	status, refreshedCookies, err := assistantUIProxyJSONRequest(ctx, targetURL, http.MethodPost, "/api/auth/refresh?retry=1", nil, cookies)
	if err != nil {
		return nil, false, err
	}
	switch status {
	case http.StatusOK:
		if len(refreshedCookies) == 0 {
			return assistantUIFilterAllowedAuthCookies(cookies), true, nil
		}
		return refreshedCookies, true, nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("assistant-ui upstream refresh status=%d", status)
	}
}

func assistantUILoginUpstreamSession(ctx context.Context, targetURL *url.URL, creds assistantUIBootstrapCredentials) ([]*http.Cookie, int, error) {
	status, cookies, err := assistantUIProxyJSONRequest(ctx, targetURL, http.MethodPost, "/api/auth/login", map[string]string{
		"email":    creds.Email,
		"password": creds.Password,
	}, nil)
	return cookies, status, err
}

func assistantUIRegisterUpstreamSession(ctx context.Context, targetURL *url.URL, creds assistantUIBootstrapCredentials) (int, error) {
	status, _, err := assistantUIProxyJSONRequest(ctx, targetURL, http.MethodPost, "/api/auth/register", map[string]string{
		"name":             creds.Name,
		"username":         creds.Username,
		"email":            creds.Email,
		"password":         creds.Password,
		"confirm_password": creds.Password,
	}, nil)
	return status, err
}

func assistantUIProxyJSONRequest(ctx context.Context, targetURL *url.URL, method string, path string, payload any, cookies []*http.Cookie) (int, []*http.Cookie, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, err
		}
		body = bytes.NewReader(encoded)
	}
	requestURL := *targetURL
	requestURL.Path = joinProxyPath(strings.TrimSuffix(targetURL.Path, "/"), path)
	requestURL.RawQuery = ""
	if idx := strings.Index(path, "?"); idx >= 0 {
		requestURL.Path = joinProxyPath(strings.TrimSuffix(targetURL.Path, "/"), path[:idx])
		requestURL.RawQuery = path[idx+1:]
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL.String(), body)
	if err != nil {
		return 0, nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	assistantUIApplyAuthCookiesToRequest(req, cookies)
	resp, err := assistantUIBootstrapHTTPClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, assistantUIFilterAllowedAuthCookies(resp.Cookies()), nil
}

func assistantUIBootstrapCredentialSet(tenant Tenant, principal Principal) assistantUIBootstrapCredentials {
	tenantSlug := assistantUIBootstrapSlug(tenant.ID)
	principalSlug := assistantUIBootstrapSlug(principal.ID)
	kratosSlug := assistantUIBootstrapSlug(principal.KratosIdentityID)
	if tenantSlug == "" {
		tenantSlug = "tenant"
	}
	if principalSlug == "" {
		principalSlug = "principal"
	}
	seed := strings.Join([]string{
		"assistant-ui",
		tenant.ID,
		principal.ID,
		principal.KratosIdentityID,
		principal.Email,
	}, "|")
	digest := sha256.Sum256([]byte(seed))
	name := fmt.Sprintf("Assistant %s %s", assistantUIBootstrapShortSlug(tenantSlug, 12), assistantUIBootstrapShortSlug(principalSlug, 12))
	username := fmt.Sprintf("bb_%s_%s", assistantUIBootstrapShortSlug(tenantSlug, 16), assistantUIBootstrapShortSlug(principalSlug, 24))
	if kratosSlug != "" {
		username = fmt.Sprintf("%s_%s", username, assistantUIBootstrapShortSlug(kratosSlug, 12))
	}
	if len(username) > 80 {
		username = username[:80]
	}
	return assistantUIBootstrapCredentials{
		Name:     name,
		Username: username,
		Email:    fmt.Sprintf("bb.%s.%s@assistant.local", assistantUIBootstrapShortSlug(tenantSlug, 24), assistantUIBootstrapShortSlug(principalSlug, 24)),
		Password: "Bb!" + hex.EncodeToString(digest[:]),
	}
}

func assistantUIBootstrapSlug(input string) string {
	value := strings.ToLower(strings.TrimSpace(input))
	value = assistantUIBootstrapSlugPattern.ReplaceAllString(value, "")
	return strings.Trim(value, ".-_ ")
}

func assistantUIBootstrapShortSlug(input string, limit int) string {
	value := assistantUIBootstrapSlug(input)
	if value == "" {
		return "x"
	}
	if limit > 0 && len(value) > limit {
		return value[:limit]
	}
	return value
}

func assistantUIApplyAuthCookiesToRequest(req *http.Request, cookies []*http.Cookie) {
	if req == nil {
		return
	}
	byName := map[string]*http.Cookie{}
	for _, cookie := range req.Cookies() {
		if _, allowed := assistantUIProxyAllowedAuthCookies[cookie.Name]; !allowed {
			continue
		}
		if strings.TrimSpace(cookie.Value) == "" {
			continue
		}
		clone := *cookie
		byName[cookie.Name] = &clone
	}
	for _, cookie := range cookies {
		if _, allowed := assistantUIProxyAllowedAuthCookies[cookie.Name]; !allowed {
			continue
		}
		if strings.TrimSpace(cookie.Value) == "" {
			continue
		}
		clone := *cookie
		byName[cookie.Name] = &clone
	}
	req.Header.Del("Cookie")
	parts := make([]string, 0, len(byName))
	for _, name := range []string{"refreshToken", "token_provider", "openid_user_id"} {
		cookie, ok := byName[name]
		if !ok {
			continue
		}
		parts = append(parts, (&http.Cookie{Name: cookie.Name, Value: cookie.Value}).String())
	}
	if len(parts) > 0 {
		req.Header.Set("Cookie", strings.Join(parts, "; "))
	}
}

func assistantUIFilterAllowedAuthCookies(cookies []*http.Cookie) []*http.Cookie {
	filtered := make([]*http.Cookie, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie == nil {
			continue
		}
		if _, allowed := assistantUIProxyAllowedAuthCookies[cookie.Name]; !allowed {
			continue
		}
		if strings.TrimSpace(cookie.Value) == "" {
			continue
		}
		clone := *cookie
		filtered = append(filtered, &clone)
	}
	return filtered
}

func assistantUIBootstrapCookiesFromContext(ctx context.Context) []*http.Cookie {
	if ctx == nil {
		return nil
	}
	value := ctx.Value(assistantUIBootstrapCookiesCtxKey{})
	cookies, ok := value.([]*http.Cookie)
	if !ok || len(cookies) == 0 {
		return nil
	}
	return assistantUIFilterAllowedAuthCookies(cookies)
}

func assistantUINormalizeProxyResponseCookies(cookies []*http.Cookie) []*http.Cookie {
	byName := map[string]*http.Cookie{}
	for _, cookie := range cookies {
		if cookie == nil {
			continue
		}
		if _, allowed := assistantUIProxyAllowedAuthCookies[cookie.Name]; !allowed {
			continue
		}
		if strings.TrimSpace(cookie.Value) == "" {
			continue
		}
		clone := *cookie
		if clone.Path == "" || clone.Path == "/" {
			clone.Path = "/assistant-ui"
		}
		clone.Secure = false
		byName[clone.Name] = &clone
	}
	out := make([]*http.Cookie, 0, len(byName))
	for _, name := range []string{"refreshToken", "token_provider", "openid_user_id"} {
		cookie, ok := byName[name]
		if !ok {
			continue
		}
		out = append(out, cookie)
	}
	return out
}

func assistantUIWriteProxyResponseCookies(w http.ResponseWriter, cookies []*http.Cookie) {
	for _, cookie := range assistantUINormalizeProxyResponseCookies(cookies) {
		http.SetCookie(w, cookie)
	}
}

func assistantUIUnavailableHandler(reason string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveAssistantUIUnavailableResponse(w, r, http.StatusServiceUnavailable, "upstream_invalid:"+reason)
	})
}

func serveAssistantUIUnavailableResponse(w http.ResponseWriter, r *http.Request, status int, reason string) {
	if assistantUIProxyShouldServeFallbackShell(r) {
		assistantUIProxyLog(r, reason)
		serveAssistantUIFallbackShell(w, r, status)
		return
	}
	writeAssistantUIProxyError(
		w,
		r,
		status,
		assistantUIProxyUpstreamUnavailable,
		"assistant_ui_upstream_unavailable",
		reason,
	)
}

func assistantUIProxyShouldServeFallbackShell(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	proxyPath := strings.TrimPrefix(r.URL.Path, "/assistant-ui")
	if proxyPath != "" && proxyPath != "/" {
		return false
	}
	accept := strings.ToLower(strings.TrimSpace(r.Header.Get("Accept")))
	if strings.Contains(accept, "application/json") && !strings.Contains(accept, "text/html") {
		return false
	}
	return true
}

func serveAssistantUIFallbackShell(w http.ResponseWriter, r *http.Request, upstreamStatus int) {
	body := assistantUIFallbackShellHTML()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Assistant-UI-Mode", "fallback-shell")
	w.Header().Set("X-Assistant-UI-Upstream-Status", strconv.Itoa(upstreamStatus))
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, body)
}

func assistantUIFallbackShellHTML() string {
	return `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <base href="/assistant-ui/" />
  <title>Assistant Chat</title>
  <style>
    :root { color-scheme: light; }
    body {
      margin: 0;
      font-family: Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: #f6f8fb;
      color: #123;
    }
    main {
      max-width: 960px;
      margin: 0 auto;
      min-height: 100vh;
      display: flex;
      flex-direction: column;
      gap: 12px;
      padding: 16px;
      box-sizing: border-box;
    }
    .banner {
      border: 1px solid #ffd54f;
      background: #fff8e1;
      color: #5d4037;
      border-radius: 12px;
      padding: 12px 14px;
      font-size: 13px;
      line-height: 1.5;
    }
    .stream {
      flex: 1;
      min-height: 360px;
      border: 1px solid rgba(0,0,0,0.08);
      background: #fff;
      border-radius: 16px;
      padding: 16px;
      box-shadow: 0 8px 24px rgba(15, 23, 42, 0.06);
      overflow: auto;
      display: flex;
      flex-direction: column;
      gap: 8px;
    }
    .placeholder {
      font-size: 13px;
      color: #456;
      background: #eef6f6;
      border: 1px dashed #09a7a3;
      border-radius: 10px;
      padding: 10px 12px;
    }
    form {
      display: flex;
      gap: 12px;
      align-items: flex-end;
    }
    textarea {
      flex: 1;
      min-height: 96px;
      resize: vertical;
      border-radius: 12px;
      border: 1px solid rgba(0,0,0,0.12);
      padding: 12px 14px;
      font: inherit;
      line-height: 1.5;
      box-sizing: border-box;
      background: #fff;
    }
    button {
      border: 0;
      border-radius: 12px;
      background: #09a7a3;
      color: #fff;
      font: inherit;
      font-weight: 600;
      padding: 12px 18px;
      cursor: pointer;
    }
    button:hover { background: #078a87; }
  </style>
  <script src="/assistant-ui/bridge.js" defer></script>
</head>
<body>
  <main>
    <div class="banner">LibreChat 上游当前不可达，已切换到仓内最小聊天壳层；对话消息仍通过现有 Assistant Bridge 与后端编排链路处理。</div>
    <div class="stream" data-testid="chat-container" role="log" aria-label="Assistant Transcript">
      <div class="placeholder">请在下方输入需求并点击“发送”，或按 Enter 发送、Shift+Enter 换行。</div>
      <div data-assistant-dialog-stream="1"></div>
    </div>
    <form data-assistant-fallback-form="1">
      <textarea placeholder="请输入你的需求，例如：在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026年1月1日。"></textarea>
      <button type="submit">发送</button>
    </form>
  </main>
  <script>
    (function () {
      var form = document.querySelector('[data-assistant-fallback-form="1"]');
      var input = form ? form.querySelector('textarea') : null;
      if (!form || !input) {
        return;
      }
      form.addEventListener('submit', function (event) {
        event.preventDefault();
        var value = (input.value || '').trim();
        if (!value) {
          input.focus();
          return;
        }
        input.value = '';
      });
      input.addEventListener('keydown', function (event) {
        if (event.key !== 'Enter' || event.shiftKey || event.ctrlKey || event.metaKey || event.altKey || event.isComposing) {
          return;
        }
        event.preventDefault();
        if (typeof form.requestSubmit === 'function') {
          form.requestSubmit();
          return;
        }
        form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
      });
    })();
  </script>
</body>
</html>`
}

func serveAssistantUIBridgeScript(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeAssistantUIProxyError(
			w,
			r,
			http.StatusMethodNotAllowed,
			assistantUIProxyMethodNotAllowedCode,
			"assistant_ui_method_not_allowed",
			"bridge_script_method_not_allowed",
		)
		return
	}
	body := assistantUIBridgeScriptBody()
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = io.WriteString(w, body)
}

func assistantUIBridgeScriptBody() string {
	return `(function () {
  if (!window || !window.parent || window.parent === window) {
    return;
  }
  var params = new URLSearchParams(window.location.search || "");
  var channel = (params.get("channel") || "").trim();
  var nonce = (params.get("nonce") || "").trim();
  if (!channel || !nonce) {
    return;
  }
  var lastPayload = "";
  var lastAt = 0;
  function normalize(input) {
    if (typeof input !== "string") {
      return "";
    }
    return input.replace(/\\s+/g, " ").trim();
  }
  function emitPrompt(input) {
    var text = normalize(input);
    if (!text) {
      return;
    }
    var now = Date.now();
    if (text === lastPayload && now - lastAt < 1200) {
      return;
    }
    lastPayload = text;
    lastAt = now;
    window.parent.postMessage({
      type: "assistant.prompt.sync",
      channel: channel,
      nonce: nonce,
      payload: { input: text, source: "librechat" }
    }, window.location.origin);
  }
  function readInputValue(node) {
    if (!node) {
      return "";
    }
    if (typeof node.value === "string") {
      return node.value;
    }
    if (typeof node.textContent === "string") {
      return node.textContent;
    }
    return "";
  }
  function findInput(node) {
    if (node && node.matches && node.matches('textarea, [contenteditable="true"]')) {
      return node;
    }
    if (node && node.querySelector) {
      var nested = node.querySelector('textarea, [contenteditable="true"]');
      if (nested) {
        return nested;
      }
    }
    if (node && node.closest) {
      var form = node.closest("form");
      if (form) {
        var inForm = form.querySelector('textarea, [contenteditable="true"]');
        if (inForm) {
          return inForm;
        }
      }
    }
    return document.querySelector('form textarea, form [contenteditable="true"], textarea, [contenteditable="true"]');
  }
  var dialogQueue = [];
  var dialogObserver = null;
  var dialogObserverTimer = null;
  var dialogObserverAttempts = 0;
  var dialogObserverMaxAttempts = 80;
  function isValidDialogRoot(node) {
    if (!node || !node.tagName) {
      return false;
    }
    var tag = String(node.tagName || "").toLowerCase();
    if (tag === "body" || tag === "html" || tag === "main") {
      return false;
    }
    return true;
  }
	  function findDialogRoot() {
	    var selectors = [
	      "#messages-view",
	      '[data-testid="conversation-container"] [role="log"]',
	      '[data-testid="chat-container"] [role="log"]',
	      '[data-testid="conversation-container"]',
	      '[data-testid="chat-container"]',
      '[data-testid="messages-container"]',
      '[data-testid="message-list"]',
      '[role="log"]'
    ];
    for (var i = 0; i < selectors.length; i += 1) {
      var node = document.querySelector(selectors[i]);
      if (isValidDialogRoot(node)) {
        return node;
      }
    }
    return null;
  }
  function ensureDialogStream() {
    var root = findDialogRoot();
    if (!root) {
      return null;
    }
    var container = document.querySelector('[data-assistant-dialog-stream="1"]');
    if (!container) {
      container = document.createElement("div");
      container.setAttribute("data-assistant-dialog-stream", "1");
      container.style.display = "flex";
      container.style.flexDirection = "column";
      container.style.gap = "8px";
      container.style.margin = "12px 0";
      container.style.pointerEvents = "none";
    }
    if (container.parentElement !== root) {
      root.appendChild(container);
    }
    return container;
  }
  function styleDialogItem(item, level) {
    item.style.padding = "10px 12px";
    item.style.borderRadius = "10px";
    item.style.fontSize = "12px";
    item.style.lineHeight = "1.45";
    item.style.color = "#123";
    item.style.whiteSpace = "pre-wrap";
    item.style.wordBreak = "break-word";
    item.style.border = "1px solid rgba(0,0,0,0.08)";
    item.style.maxWidth = "min(680px, 100%)";
    item.style.alignSelf = "flex-start";
    item.style.background = "#eef3ff";
    item.style.borderColor = "#c7d7ff";
    if (level === "success") {
      item.style.background = "#e8f5e9";
      item.style.borderColor = "#81c784";
    } else if (level === "warning") {
      item.style.background = "#fff8e1";
      item.style.borderColor = "#ffd54f";
    } else if (level === "error") {
      item.style.background = "#ffebee";
      item.style.borderColor = "#ef9a9a";
    }
  }
  function appendDialogMessageToStream(stream, payload, fallbackSeverity) {
    if (!stream) {
      return;
    }
    var text = normalize(payload && payload.text);
    if (!text) {
      return;
    }
    var level = normalize(payload && (payload.kind || payload.severity || fallbackSeverity)).toLowerCase();
    if (!level) {
      level = "info";
    }
    var stage = normalize(payload && payload.stage);
    var item = document.createElement("div");
    styleDialogItem(item, level);
    if (stage) {
      item.setAttribute("data-assistant-dialog-stage", stage);
    }
    item.textContent = text;
    stream.appendChild(item);
    item.scrollIntoView({ block: "nearest", inline: "nearest" });
  }
  function stopDialogObserver() {
    if (dialogObserver) {
      dialogObserver.disconnect();
      dialogObserver = null;
    }
    if (dialogObserverTimer !== null) {
      window.clearInterval(dialogObserverTimer);
      dialogObserverTimer = null;
    }
  }
  function startDialogObserver() {
    if (dialogObserver || !document.documentElement) {
      return;
    }
    dialogObserverAttempts = 0;
    dialogObserver = new MutationObserver(function () {
      flushDialogQueue();
    });
    dialogObserver.observe(document.documentElement, { childList: true, subtree: true });
    dialogObserverTimer = window.setInterval(function () {
      dialogObserverAttempts += 1;
      if (flushDialogQueue()) {
        return;
      }
      if (dialogObserverAttempts < dialogObserverMaxAttempts) {
        return;
      }
      stopDialogObserver();
      if (dialogQueue.length > 0) {
        dialogQueue = [];
        window.parent.postMessage({
          type: "assistant.bridge.render_error",
          channel: channel,
          nonce: nonce,
          payload: { code: "dialog_root_not_found", message: "dialog root not found" }
        }, window.location.origin);
      }
    }, 250);
  }
  function flushDialogQueue() {
    if (dialogQueue.length === 0) {
      stopDialogObserver();
      return true;
    }
    var stream = ensureDialogStream();
    if (!stream) {
      startDialogObserver();
      return false;
    }
    while (dialogQueue.length > 0) {
      var next = dialogQueue.shift();
      appendDialogMessageToStream(stream, next.payload, next.fallbackSeverity);
    }
    stopDialogObserver();
    return true;
  }
  function appendDialogMessage(payload, fallbackSeverity) {
    var text = normalize(payload && payload.text);
    if (!text) {
      return;
    }
    dialogQueue.push({ payload: payload, fallbackSeverity: fallbackSeverity });
    flushDialogQueue();
  }
  window.addEventListener("message", function (event) {
    if (event.origin !== window.location.origin) {
      return;
    }
    var data = event.data;
    if (!data || typeof data !== "object") {
      return;
    }
    if (data.channel !== channel || data.nonce !== nonce) {
      return;
    }
    if (data.type === "assistant.flow.dialog" && data.payload && typeof data.payload === "object") {
      appendDialogMessage(data.payload, "info");
      return;
    }
    if (data.type === "assistant.flow.notice" && data.payload && typeof data.payload === "object") {
      appendDialogMessage(data.payload, "info");
    }
  });
  document.addEventListener("submit", function (event) {
    var target = findInput(event.target);
    emitPrompt(readInputValue(target));
  }, true);
  document.addEventListener("keydown", function (event) {
    if (event.key !== "Enter" || event.shiftKey || event.ctrlKey || event.metaKey || event.altKey) {
      return;
    }
    if (event.isComposing) {
      return;
    }
    var target = findInput(event.target);
    emitPrompt(readInputValue(target));
  }, true);
  document.addEventListener("click", function (event) {
    var node = event.target;
    if (!node || !node.closest) {
      return;
    }
    var button = node.closest('button, [role="button"]');
    if (!button) {
      return;
    }
    var label = normalize(button.textContent || "");
    if (!/send|发送|提交/.test(label)) {
      return;
    }
    var target = findInput(button);
    emitPrompt(readInputValue(target));
  }, true);
  window.parent.postMessage({
    type: "assistant.bridge.ready",
    channel: channel,
    nonce: nonce,
    payload: { source: "assistant-ui-bridge" }
  }, window.location.origin);
})();`
}

func joinProxyPath(base string, suffix string) string {
	if base == "" || base == "/" {
		return suffix
	}
	if strings.HasSuffix(base, "/") {
		if strings.HasPrefix(suffix, "/") {
			return base + strings.TrimPrefix(suffix, "/")
		}
		return base + suffix
	}
	if strings.HasPrefix(suffix, "/") {
		return base + suffix
	}
	return base + "/" + suffix
}

var assistantUIProxyAllowedMethods = map[string]struct{}{
	http.MethodGet:     {},
	http.MethodHead:    {},
	http.MethodPost:    {},
	http.MethodOptions: {},
}

func assistantUIProxyMethodAllowed(method string) bool {
	_, ok := assistantUIProxyAllowedMethods[method]
	return ok
}

var assistantUIProxyAllowedRequestHeaders = map[string]struct{}{
	"Accept":          {},
	"Accept-Language": {},
	"Authorization":   {},
	"Cache-Control":   {},
	"Cookie":          {},
	"Content-Type":    {},
	"Origin":          {},
	"Referer":         {},
	"User-Agent":      {},
}

var assistantUIProxyAllowedAuthCookies = map[string]struct{}{
	"refreshToken":   {},
	"token_provider": {},
	"openid_user_id": {},
}

func filterAssistantUIProxyRequestHeaders(req *http.Request) {
	cookieHeader := sanitizeAssistantUIProxyRequestCookieHeader(req)
	authHeader := sanitizeAssistantUIProxyAuthorizationHeader(req)
	for key := range req.Header {
		if _, allowed := assistantUIProxyAllowedRequestHeaders[key]; allowed {
			continue
		}
		req.Header.Del(key)
	}
	if cookieHeader == "" {
		req.Header.Del("Cookie")
	} else {
		req.Header.Set("Cookie", cookieHeader)
	}
	if authHeader == "" {
		req.Header.Del("Authorization")
	} else {
		req.Header.Set("Authorization", authHeader)
	}
	req.Header.Del("Accept-Encoding")
}

func sanitizeAssistantUIProxyRequestCookieHeader(req *http.Request) string {
	if req == nil {
		return ""
	}
	cookies := req.Cookies()
	filtered := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		if _, allowed := assistantUIProxyAllowedAuthCookies[cookie.Name]; !allowed {
			continue
		}
		if strings.TrimSpace(cookie.Value) == "" {
			continue
		}
		filtered = append(filtered, (&http.Cookie{Name: cookie.Name, Value: cookie.Value}).String())
	}
	return strings.Join(filtered, "; ")
}

func sanitizeAssistantUIProxyAuthorizationHeader(req *http.Request) string {
	if req == nil {
		return ""
	}
	raw := strings.TrimSpace(req.Header.Get("Authorization"))
	if raw == "" {
		return ""
	}
	parts := strings.Fields(raw)
	if len(parts) != 2 {
		return ""
	}
	if !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return "Bearer " + parts[1]
}

func modifyAssistantUIProxyResponse(resp *http.Response) error {
	if err := rewriteAssistantUIProxyHTMLBase(resp); err != nil {
		return err
	}
	filterAssistantUIProxyResponseCookies(resp)
	return nil
}

func filterAssistantUIProxyResponseCookies(resp *http.Response) {
	if resp == nil {
		return
	}
	cookies := resp.Cookies()
	if resp.Request != nil {
		cookies = append(cookies, assistantUIBootstrapCookiesFromContext(resp.Request.Context())...)
	}
	resp.Header.Del("Set-Cookie")
	for _, cookie := range assistantUINormalizeProxyResponseCookies(cookies) {
		resp.Header.Add("Set-Cookie", cookie.String())
	}
}

func rewriteAssistantUIProxyHTMLBase(resp *http.Response) error {
	if resp == nil || resp.Body == nil {
		return nil
	}
	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if !strings.HasPrefix(contentType, "text/html") {
		return nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if !assistantUIProxyLooksLikeHTML(body) {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		resp.ContentLength = int64(len(body))
		resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
		return nil
	}

	original := string(body)
	rewritten := original
	rewritten = strings.ReplaceAll(rewritten, `<base href="/" />`, `<base href="/assistant-ui/" />`)
	rewritten = strings.ReplaceAll(rewritten, `<base href="/">`, `<base href="/assistant-ui/">`)
	rewritten = strings.ReplaceAll(rewritten, `<base href='/' />`, `<base href='/assistant-ui/' />`)
	rewritten = strings.ReplaceAll(rewritten, `<base href='/'>`, `<base href='/assistant-ui/'>`)
	rewritten = stripAssistantUIProxyServiceWorkerRegistration(rewritten)
	rewritten = injectAssistantUIProxyServiceWorkerCleanupScript(rewritten)
	rewritten = injectAssistantUIProxyBridgeScript(rewritten)
	if rewritten == original {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		resp.ContentLength = int64(len(body))
		resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
		return nil
	}
	newBody := []byte(rewritten)
	resp.Body = io.NopCloser(bytes.NewReader(newBody))
	resp.ContentLength = int64(len(newBody))
	resp.Header.Set("Content-Length", strconv.Itoa(len(newBody)))
	return nil
}

func assistantUIProxyLooksLikeHTML(body []byte) bool {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '<' {
		return false
	}
	snippetLen := len(trimmed)
	if snippetLen > 1024 {
		snippetLen = 1024
	}
	snippet := strings.ToLower(string(trimmed[:snippetLen]))
	return strings.Contains(snippet, "<!doctype html") ||
		strings.Contains(snippet, "<html") ||
		strings.Contains(snippet, "<head") ||
		strings.Contains(snippet, "<body")
}

func injectAssistantUIProxyBridgeScript(html string) string {
	if strings.Contains(html, assistantUIBridgeScriptTag) {
		return html
	}
	lower := strings.ToLower(html)
	if idx := strings.Index(lower, "</head>"); idx >= 0 {
		return html[:idx] + assistantUIBridgeScriptTag + html[idx:]
	}
	if idx := strings.Index(lower, "</body>"); idx >= 0 {
		return html[:idx] + assistantUIBridgeScriptTag + html[idx:]
	}
	return html + assistantUIBridgeScriptTag
}

func injectAssistantUIProxyServiceWorkerCleanupScript(html string) string {
	if strings.Contains(html, assistantUISWCleanupScriptTag) {
		return html
	}
	lower := strings.ToLower(html)
	if idx := strings.Index(lower, "</head>"); idx >= 0 {
		return html[:idx] + assistantUISWCleanupScriptTag + html[idx:]
	}
	if idx := strings.Index(lower, "</body>"); idx >= 0 {
		return html[:idx] + assistantUISWCleanupScriptTag + html[idx:]
	}
	return html + assistantUISWCleanupScriptTag
}

func stripAssistantUIProxyServiceWorkerRegistration(html string) string {
	return assistantUIRegisterSWScriptTagPattern.ReplaceAllString(html, "")
}

func assistantUIProxyBypassPathForbidden(proxyPath string) bool {
	for _, prefix := range assistantUIProxyForbiddenBypassPrefixes {
		if pathHasPrefixSegment(proxyPath, prefix) {
			return true
		}
	}
	return false
}

func writeAssistantUIProxyError(w http.ResponseWriter, r *http.Request, status int, code string, message string, reason string) {
	assistantUIProxyLog(r, reason)
	routing.WriteError(w, r, routing.RouteClassUI, status, code, message)
}

func assistantUIProxyLog(r *http.Request, reason string) {
	tenantID := assistantUIProxyDefaultRequestIDValue
	if tenant, ok := currentTenant(r.Context()); ok {
		if value := strings.TrimSpace(tenant.ID); value != "" {
			tenantID = value
		}
	}
	requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
	if requestID == "" {
		requestID = assistantUIProxyDefaultRequestIDValue
	}
	traceID := assistantUIProxyTraceID(r)
	if traceID == "" {
		traceID = assistantUIProxyDefaultRequestIDValue
	}
	log.Printf(
		"assistant_ui_proxy_event tenant_id=%s request_id=%s trace_id=%s path=%s method=%s reason=%s",
		tenantID,
		requestID,
		traceID,
		r.URL.Path,
		r.Method,
		reason,
	)
}

func assistantUIProxyTraceID(r *http.Request) string {
	traceparent := strings.TrimSpace(r.Header.Get("traceparent"))
	if traceparent == "" {
		return ""
	}
	parts := strings.Split(traceparent, "-")
	if len(parts) != 4 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
