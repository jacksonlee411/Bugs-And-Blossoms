package server

import (
	"log"
	"net/http"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

const (
	assistantUIRetiredCode              = "assistant_ui_retired"
	assistantRuntimeUpstreamUnavailable = "assistant_ui_upstream_unavailable"
	assistantUIDefaultRequestIDValue    = "-"
)

func newAssistantUIRetiredHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assistantUILog(r, assistantRuntimeReasonRetiredByDesign)
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusGone, assistantUIRetiredCode, "assistant_ui_retired")
	})
}

func assistantUILog(r *http.Request, reason string) {
	tenantID := assistantUIDefaultRequestIDValue
	if tenant, ok := currentTenant(r.Context()); ok {
		if value := strings.TrimSpace(tenant.ID); value != "" {
			tenantID = value
		}
	}
	requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
	if requestID == "" {
		requestID = assistantUIDefaultRequestIDValue
	}
	traceID := assistantUITraceID(r)
	if traceID == "" {
		traceID = assistantUIDefaultRequestIDValue
	}
	log.Printf(
		"assistant_ui_retired_event tenant_id=%s request_id=%s trace_id=%s path=%s method=%s reason=%s",
		tenantID,
		requestID,
		traceID,
		r.URL.Path,
		r.Method,
		reason,
	)
}

func assistantUITraceID(r *http.Request) string {
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
