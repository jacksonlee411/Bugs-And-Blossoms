package server

import (
	"log"
	"net/http"
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
		if proxyPath := strings.TrimPrefix(r.URL.Path, "/assistant-ui"); assistantUIProxyBypassPathForbidden(proxyPath) {
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
		http.Redirect(w, r, libreChatFormalEntryPrefix, http.StatusFound)
	})
}

var assistantUIProxyAllowedMethods = map[string]struct{}{
	http.MethodGet:  {},
	http.MethodHead: {},
}

func assistantUIProxyMethodAllowed(method string) bool {
	_, ok := assistantUIProxyAllowedMethods[method]
	return ok
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
