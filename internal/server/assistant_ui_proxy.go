package server

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

func newAssistantUIProxyHandler() http.Handler {
	targetRaw := strings.TrimSpace(os.Getenv("LIBRECHAT_UPSTREAM"))
	if targetRaw == "" {
		return assistantUIUnavailableHandler("LIBRECHAT_UPSTREAM is not configured")
	}
	targetURL, err := url.Parse(targetRaw)
	if err != nil || targetURL.Scheme == "" || targetURL.Host == "" {
		return assistantUIUnavailableHandler("LIBRECHAT_UPSTREAM is invalid")
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	basePath := strings.TrimSuffix(targetURL.Path, "/")
	proxyDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		proxyDirector(req)
		proxyPath := strings.TrimPrefix(req.URL.Path, "/assistant-ui")
		if proxyPath == "" {
			proxyPath = "/"
		}
		req.URL.Path = joinProxyPath(basePath, proxyPath)
		req.Host = targetURL.Host
		req.Header.Set("X-Forwarded-Prefix", "/assistant-ui")
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("<html><body><h3>LibreChat upstream unavailable</h3></body></html>"))
	}

	return proxy
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
