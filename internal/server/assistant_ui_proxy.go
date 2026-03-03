package server

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

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
		resp.Header.Del("Set-Cookie")
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("<html><body><h3>LibreChat upstream unavailable</h3></body></html>"))
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if r.URL.Path != "/assistant-ui" && !pathHasPrefixSegment(r.URL.Path, "/assistant-ui") {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "invalid_request", "invalid request")
			return
		}
		proxy.ServeHTTP(w, r)
	})
}

func assistantUIUnavailableHandler(reason string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("<html><body><h3>LibreChat unavailable</h3><p>" + reason + "</p></body></html>"))
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

var assistantUIProxyAllowedRequestHeaders = map[string]struct{}{
	"Accept":          {},
	"Accept-Encoding": {},
	"Accept-Language": {},
	"Cache-Control":   {},
	"Content-Type":    {},
	"Origin":          {},
	"Referer":         {},
	"User-Agent":      {},
}

func filterAssistantUIProxyRequestHeaders(req *http.Request) {
	for header := range req.Header {
		if _, ok := assistantUIProxyAllowedRequestHeaders[http.CanonicalHeaderKey(header)]; ok {
			continue
		}
		req.Header.Del(header)
	}
	req.Header.Del("Cookie")
	req.Header.Del("Authorization")
}
