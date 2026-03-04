package server

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"strings"

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
		writeAssistantUIProxyError(
			w,
			r,
			http.StatusBadGateway,
			assistantUIProxyUpstreamUnavailable,
			"assistant_ui_upstream_unavailable",
			"upstream_unreachable",
		)
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
		proxy.ServeHTTP(w, r)
	})
}

func assistantUIUnavailableHandler(reason string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeAssistantUIProxyError(
			w,
			r,
			http.StatusServiceUnavailable,
			assistantUIProxyUpstreamUnavailable,
			"assistant_ui_upstream_unavailable",
			"upstream_invalid:"+reason,
		)
	})
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
  function mountNoticeLayer() {
    var container = document.getElementById("assistant-flow-notice-layer");
    if (container) {
      return container;
    }
    container = document.createElement("div");
    container.id = "assistant-flow-notice-layer";
    container.style.position = "fixed";
    container.style.right = "16px";
    container.style.bottom = "16px";
    container.style.maxWidth = "360px";
    container.style.zIndex = "2147483000";
    container.style.display = "flex";
    container.style.flexDirection = "column";
    container.style.gap = "8px";
    document.body.appendChild(container);
    return container;
  }
  function showNotice(text, severity) {
    var content = normalize(text);
    if (!content) {
      return;
    }
    var layer = mountNoticeLayer();
    var item = document.createElement("div");
    item.style.padding = "10px 12px";
    item.style.borderRadius = "10px";
    item.style.fontSize = "12px";
    item.style.lineHeight = "1.45";
    item.style.color = "#123";
    item.style.whiteSpace = "pre-wrap";
    item.style.wordBreak = "break-word";
    item.style.boxShadow = "0 4px 14px rgba(0,0,0,0.2)";
    item.style.border = "1px solid rgba(0,0,0,0.08)";
    var level = (severity || "info").toLowerCase();
    if (level === "success") {
      item.style.background = "#e8f5e9";
      item.style.borderColor = "#81c784";
    } else if (level === "warning") {
      item.style.background = "#fff8e1";
      item.style.borderColor = "#ffd54f";
    } else if (level === "error") {
      item.style.background = "#ffebee";
      item.style.borderColor = "#ef9a9a";
    } else {
      item.style.background = "#e3f2fd";
      item.style.borderColor = "#90caf9";
    }
    item.textContent = content;
    layer.appendChild(item);
    window.setTimeout(function () {
      if (item.parentNode) {
        item.parentNode.removeChild(item);
      }
    }, 9000);
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
    if (data.type === "assistant.flow.notice" && data.payload && typeof data.payload === "object") {
      showNotice(data.payload.text, data.payload.severity);
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
	resp.Header.Del("Set-Cookie")
	for _, cookie := range cookies {
		if _, allowed := assistantUIProxyAllowedAuthCookies[cookie.Name]; !allowed {
			continue
		}
		if cookie.Path == "" || cookie.Path == "/" {
			cookie.Path = "/assistant-ui"
		}
		cookie.Secure = false
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
