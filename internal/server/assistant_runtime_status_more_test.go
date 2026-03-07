package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestAssistantRuntimeStatusAdditionalCoverage(t *testing.T) {
	if err := assistantRuntimeProbeUpstream("://bad"); err == nil {
		t.Fatal("expected invalid upstream parse error")
	}
	services := assistantRuntimeUpsertService(nil, assistantRuntimeService{Name: "meili", Healthy: "unknown", Required: false, Reason: "new"})
	if len(services) != 1 || services[0].Healthy != assistantRuntimeHealthUnavailable {
		t.Fatalf("services=%+v", services)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "ok") }))
	defer upstream.Close()
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "versions.lock.yaml")
	if err := os.WriteFile(lockPath, []byte("upstream:\n  repo: demo/repo\nservices: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	statusPath := filepath.Join(dir, "missing.status.json")
	t.Setenv("LIBRECHAT_UPSTREAM", upstream.URL)
	t.Setenv("ASSISTANT_RUNTIME_VERSIONS_LOCK", lockPath)
	t.Setenv("ASSISTANT_RUNTIME_STATUS_FILE", statusPath)
	t.Setenv("ASSISTANT_DOMAIN_ALLOWLIST_PATH", "config/assistant/domain-allowlist.yaml")
	status := assistantRuntimeStatus()
	if status.ErrorCode != "assistant_runtime_dependency_unavailable" || status.Status != assistantRuntimeHealthUnavailable {
		t.Fatalf("status=%+v", status)
	}
}

func TestAssistantDomainPatternDangerousIPv6LinkLocal(t *testing.T) {
	if !assistantDomainPatternDangerous("fe80::1") {
		t.Fatal("expected IPv6 link-local to be dangerous")
	}
}

func TestAssistantRuntimeUpsertServiceContinueBranch(t *testing.T) {
	services := []assistantRuntimeService{{Name: "api", Healthy: "healthy"}, {Name: "search", Healthy: "healthy"}}
	services = assistantRuntimeUpsertService(services, assistantRuntimeService{Name: "search", Healthy: "degraded", Reason: "changed"})
	if services[1].Healthy != assistantRuntimeHealthDegraded {
		t.Fatalf("services=%+v", services)
	}
}
