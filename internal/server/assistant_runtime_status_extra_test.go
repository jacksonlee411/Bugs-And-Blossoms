package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAssistantRuntimeHelperCoverage(t *testing.T) {
	resp := assistantRuntimeApplyUpstreamProbe(assistantRuntimeStatusResponse{})
	if resp.ErrorCode != "" {
		t.Fatalf("unexpected resp=%+v", resp)
	}
	resp = assistantRuntimeApplyUpstreamProbe(assistantRuntimeStatusResponse{Upstream: assistantRuntimeUpstreamStatus{URL: "://bad"}})
	if resp.ErrorCode != assistantUIProxyUpstreamUnavailable {
		t.Fatalf("unexpected resp=%+v", resp)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	if err := assistantRuntimeProbeUpstream(server.URL); err != nil {
		t.Fatalf("probe err=%v", err)
	}
	services := assistantRuntimeUpsertService(nil, assistantRuntimeService{Name: "api", Required: true, Healthy: "healthy", Reason: "ok"})
	services = assistantRuntimeUpsertService(services, assistantRuntimeService{Name: "api", Required: false, Healthy: "degraded", Reason: "changed"})
	if len(services) != 1 || services[0].Required || services[0].Healthy != assistantRuntimeHealthDegraded || services[0].Reason != "changed" {
		t.Fatalf("services=%+v", services)
	}
	if got := assistantRuntimeUpsertService(services, assistantRuntimeService{Name: "", Healthy: "healthy"}); len(got) != 1 {
		t.Fatalf("unexpected services=%+v", got)
	}
}
