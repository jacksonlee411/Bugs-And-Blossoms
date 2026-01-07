package server

import (
	"net/http"
	"os"
	"strings"
)

func effectiveHost(r *http.Request) string {
	if os.Getenv("TRUST_PROXY") == "1" {
		if h := forwardedHost(r); h != "" {
			return normalizeHostname(h)
		}
	}
	return normalizeHostname(r.Host)
}

func forwardedHost(r *http.Request) string {
	raw := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if raw == "" {
		return ""
	}
	first, _, ok := strings.Cut(raw, ",")
	if ok {
		raw = first
	}
	return strings.TrimSpace(raw)
}

func normalizeHostname(host string) string {
	host = strings.TrimSpace(host)
	host = hostWithoutPort(host)
	return strings.ToLower(strings.TrimSpace(host))
}

func hostWithoutPort(host string) string {
	if h, _, ok := strings.Cut(host, ":"); ok {
		return h
	}
	return host
}
