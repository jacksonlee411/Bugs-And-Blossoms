package server

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

const (
	assistantUIProxyMethodNotAllowedCode  = "assistant_ui_method_not_allowed"
	assistantUIProxyPathInvalidCode       = "assistant_ui_path_invalid"
	assistantUIProxyUpstreamUnavailable   = "assistant_ui_upstream_unavailable"
	assistantUIProxyDefaultRequestIDValue = "-"
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
	proxy.ModifyResponse = func(resp *http.Response) error {
		if err := rewriteAssistantUIProxyHTMLBase(resp); err != nil {
			return err
		}
		filterAssistantUIProxyResponseCookies(resp)
		return nil
	}
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
	if strings.TrimSpace(parts[1]) == "" {
		return ""
	}
	return "Bearer " + parts[1]
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

	original := string(body)
	rewritten := original
	rewritten = strings.ReplaceAll(rewritten, `<base href="/" />`, `<base href="/assistant-ui/" />`)
	rewritten = strings.ReplaceAll(rewritten, `<base href="/">`, `<base href="/assistant-ui/">`)
	rewritten = strings.ReplaceAll(rewritten, `<base href='/' />`, `<base href='/assistant-ui/' />`)
	rewritten = strings.ReplaceAll(rewritten, `<base href='/'>`, `<base href='/assistant-ui/'>`)
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
