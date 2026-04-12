package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestAssistantRuntimeStatus_LockMissing(t *testing.T) {
	t.Setenv("LIBRECHAT_UPSTREAM", "http://127.0.0.1:3080")
	t.Setenv("ASSISTANT_RUNTIME_VERSIONS_LOCK", filepath.Join(t.TempDir(), "missing.lock.yaml"))
	t.Setenv("ASSISTANT_RUNTIME_STATUS_FILE", filepath.Join(t.TempDir(), "missing.status.json"))

	status := assistantRuntimeStatus()
	if status.Status != assistantRuntimeHealthUnavailable {
		t.Fatalf("status=%s", status.Status)
	}
	if status.ErrorCode != "assistant_runtime_versions_lock_missing" {
		t.Fatalf("error_code=%s", status.ErrorCode)
	}
	if len(status.Services) == 0 || status.Services[0].Name != "api" {
		t.Fatalf("unexpected services=%+v", status.Services)
	}
}

func TestAssistantRuntimeStatus_InvalidUpstream(t *testing.T) {
	t.Setenv("LIBRECHAT_UPSTREAM", "://bad")
	status := assistantRuntimeStatus()
	if status.Status != assistantRuntimeHealthUnavailable {
		t.Fatalf("status=%s", status.Status)
	}
	if status.ErrorCode != "ai_runtime_config_invalid" {
		t.Fatalf("error_code=%s", status.ErrorCode)
	}
	if len(status.Services) != 1 || status.Services[0].Reason != "upstream_invalid" {
		t.Fatalf("services=%+v", status.Services)
	}
}

func TestAssistantRuntimeStatus_MergesLockAndSnapshot(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	defer upstream.Close()

	dir := t.TempDir()
	lockPath := filepath.Join(dir, "versions.lock.yaml")
	snapshotPath := filepath.Join(dir, "runtime-status.json")
	lock := `upstream:
  repo: "danny-avila/LibreChat"
  ref: "main"
  imported_at: "2026-03-03T17:00:00Z"
  rollback_ref: "abc123"
services:
  - name: "api"
    required: true
    image: "ghcr.io/danny-avila/librechat"
    tag: "v0.0.1"
    digest: "sha256:1111111111111111111111111111111111111111111111111111111111111111"
  - name: "meilisearch"
    required: false
    image: "getmeili/meilisearch"
    tag: "v1.12.0"
    digest: "sha256:2222222222222222222222222222222222222222222222222222222222222222"
`
	if err := os.WriteFile(lockPath, []byte(lock), 0o644); err != nil {
		t.Fatal(err)
	}
	snapshot := `{
  "status": "degraded",
  "checked_at": "2026-03-03T17:01:00Z",
  "services": [
    {"name":"api","required":true,"healthy":"healthy"},
    {"name":"meilisearch","required":false,"healthy":"unavailable","reason":"container_not_running"}
  ]
}`
	if err := os.WriteFile(snapshotPath, []byte(snapshot), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("LIBRECHAT_UPSTREAM", upstream.URL)
	t.Setenv("ASSISTANT_RUNTIME_VERSIONS_LOCK", lockPath)
	t.Setenv("ASSISTANT_RUNTIME_STATUS_FILE", snapshotPath)

	status := assistantRuntimeStatus()
	if status.Status != assistantRuntimeHealthDegraded {
		t.Fatalf("status=%s", status.Status)
	}
	if !status.Capabilities.MCPEnabled || !status.Capabilities.ActionsEnabled {
		t.Fatalf("capabilities=%+v", status.Capabilities)
	}
	if status.Capabilities.AgentsUIEnabled || status.Capabilities.MemoryEnabled || status.Capabilities.WebSearchEnabled || status.Capabilities.FileSearchEnabled || status.Capabilities.CodeInterpreterEnabled {
		t.Fatalf("expected formal entry downsized capabilities, got=%+v", status.Capabilities)
	}
	if !status.Capabilities.ArtifactsEnabled {
		t.Fatalf("expected artifacts enabled, got=%+v", status.Capabilities)
	}
	if status.Capabilities.RuntimeCutoverMode != assistantRuntimeCutoverModeUIShellOnly {
		t.Fatalf("runtime_cutover_mode=%q", status.Capabilities.RuntimeCutoverMode)
	}
	if status.Capabilities.DomainPolicyVersion != "v1" {
		t.Fatalf("domain policy version=%q", status.Capabilities.DomainPolicyVersion)
	}
	if status.Upstream.Repo != "danny-avila/LibreChat" || status.Upstream.Ref != "main" {
		t.Fatalf("unexpected upstream=%+v", status.Upstream)
	}
	if len(status.Services) != 2 {
		t.Fatalf("services=%d", len(status.Services))
	}
	if status.Services[0].Image == "" || status.Services[0].Digest == "" {
		t.Fatalf("lock metadata not merged: %+v", status.Services[0])
	}
}

func TestAssistantRuntimeStatus_UpstreamProbeFailsClosed(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "versions.lock.yaml")
	snapshotPath := filepath.Join(dir, "runtime-status.json")
	if err := os.WriteFile(lockPath, []byte(`services:
  - name: api
    required: true
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(snapshotPath, []byte(`{"status":"healthy","services":[{"name":"api","required":true,"healthy":"healthy"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("LIBRECHAT_UPSTREAM", "http://127.0.0.1:1")
	t.Setenv("ASSISTANT_RUNTIME_VERSIONS_LOCK", lockPath)
	t.Setenv("ASSISTANT_RUNTIME_STATUS_FILE", snapshotPath)

	status := assistantRuntimeStatus()
	if status.Status != assistantRuntimeHealthUnavailable {
		t.Fatalf("status=%s", status.Status)
	}
	if status.ErrorCode != assistantUIProxyUpstreamUnavailable {
		t.Fatalf("error_code=%s", status.ErrorCode)
	}
	if len(status.Services) == 0 || status.Services[0].Reason != "upstream_unreachable" {
		t.Fatalf("services=%+v", status.Services)
	}
}

func TestAssistantRuntimeStatus_DomainPolicyMissingFailsClosed(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "versions.lock.yaml")
	snapshotPath := filepath.Join(dir, "runtime-status.json")
	if err := os.WriteFile(lockPath, []byte("services:\n  - name: api\n    required: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(snapshotPath, []byte("{\"status\":\"healthy\",\"services\":[{\"name\":\"api\",\"required\":true,\"healthy\":\"healthy\"}]}"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("LIBRECHAT_UPSTREAM", "http://127.0.0.1:3080")
	t.Setenv("ASSISTANT_RUNTIME_VERSIONS_LOCK", lockPath)
	t.Setenv("ASSISTANT_RUNTIME_STATUS_FILE", snapshotPath)
	t.Setenv("ASSISTANT_DOMAIN_ALLOWLIST_PATH", filepath.Join(dir, "missing-domain-policy.yaml"))

	status := assistantRuntimeStatus()
	if status.Status != assistantRuntimeHealthUnavailable {
		t.Fatalf("status=%s", status.Status)
	}
	if status.ErrorCode != "assistant_oss_domain_policy_missing" {
		t.Fatalf("error_code=%s", status.ErrorCode)
	}
	if status.Capabilities.DomainPolicyVersion != "" {
		t.Fatalf("unexpected domain policy version=%q", status.Capabilities.DomainPolicyVersion)
	}
}

func TestHandleAssistantRuntimeStatusAPI(t *testing.T) {
	t.Setenv("LIBRECHAT_UPSTREAM", "http://127.0.0.1:3080")
	t.Setenv("ASSISTANT_RUNTIME_VERSIONS_LOCK", filepath.Join(t.TempDir(), "missing.lock.yaml"))
	t.Setenv("ASSISTANT_RUNTIME_STATUS_FILE", filepath.Join(t.TempDir(), "missing.status.json"))

	req := httptest.NewRequest(http.MethodGet, "http://localhost/internal/assistant/runtime-status", nil)
	rec := httptest.NewRecorder()
	handleAssistantRuntimeStatusAPI(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload assistantRuntimeStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Code == "" || payload.ErrorCode == "" {
		t.Fatalf("missing code payload=%+v", payload)
	}

	badMethodReq := httptest.NewRequest(http.MethodPost, "http://localhost/internal/assistant/runtime-status", nil)
	badMethodRec := httptest.NewRecorder()
	handleAssistantRuntimeStatusAPI(badMethodRec, badMethodReq)
	if badMethodRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d body=%s", badMethodRec.Code, badMethodRec.Body.String())
	}
}

func TestAssistantRuntimeHelpers(t *testing.T) {
	if got := assistantRuntimeServicesFromLock(nil); got != nil {
		t.Fatalf("expected nil services, got=%+v", got)
	}

	services := assistantRuntimeServicesFromLock([]struct {
		Name     string `yaml:"name"`
		Required bool   `yaml:"required"`
		Image    string `yaml:"image"`
		Tag      string `yaml:"tag"`
		Digest   string `yaml:"digest"`
	}{
		{Name: "", Required: true},
		{Name: "api", Required: true, Image: "img", Tag: "tag", Digest: "digest"},
	})
	if len(services) != 1 || services[0].Name != "api" {
		t.Fatalf("unexpected lock services=%+v", services)
	}

	base := []assistantRuntimeService{
		{Name: "api", Required: true, Healthy: "healthy", Image: "img", Tag: "tag", Digest: "digest"},
	}
	snapshot := []assistantRuntimeService{
		{Name: "api", Required: true, Healthy: "degraded", Reason: "probe_timeout"},
		{Name: "meilisearch", Required: false, Healthy: "unknown"},
	}
	merged := mergeAssistantRuntimeServices(base, snapshot)
	if len(merged) != 2 {
		t.Fatalf("merged=%+v", merged)
	}
	if merged[0].Healthy != assistantRuntimeHealthDegraded || merged[1].Healthy != assistantRuntimeHealthUnavailable {
		t.Fatalf("unexpected merged health=%+v", merged)
	}

	onlySnapshot := mergeAssistantRuntimeServices(nil, []assistantRuntimeService{{Name: "api", Healthy: "healthy", Required: true}})
	if len(onlySnapshot) != 1 || onlySnapshot[0].Healthy != assistantRuntimeHealthHealthy {
		t.Fatalf("only snapshot=%+v", onlySnapshot)
	}
	if got := mergeAssistantRuntimeServices(nil, nil); got != nil {
		t.Fatalf("expected nil merge result, got=%+v", got)
	}

	mergeMetadata := mergeAssistantRuntimeServices(
		[]assistantRuntimeService{{Name: "api", Required: true, Healthy: "healthy"}},
		[]assistantRuntimeService{{Name: "api", Required: true, Healthy: "healthy", Image: "img", Tag: "tag", Digest: "digest"}, {Name: "", Healthy: "healthy"}},
	)
	if mergeMetadata[0].Image != "img" || mergeMetadata[0].Tag != "tag" || mergeMetadata[0].Digest != "digest" {
		t.Fatalf("expected metadata fill, got=%+v", mergeMetadata[0])
	}

	if got := assistantRuntimeAggregateStatus(nil); got != assistantRuntimeHealthUnavailable {
		t.Fatalf("aggregate nil=%s", got)
	}
	if got := assistantRuntimeAggregateStatus([]assistantRuntimeService{{Name: "api", Required: true, Healthy: "healthy"}}); got != assistantRuntimeHealthHealthy {
		t.Fatalf("aggregate healthy=%s", got)
	}
	if got := assistantRuntimeAggregateStatus([]assistantRuntimeService{{Name: "api", Required: true, Healthy: "healthy"}, {Name: "optional", Required: false, Healthy: "degraded"}}); got != assistantRuntimeHealthDegraded {
		t.Fatalf("aggregate degraded=%s", got)
	}
	if got := assistantRuntimeAggregateStatus([]assistantRuntimeService{{Name: "api", Required: true, Healthy: "unavailable"}}); got != assistantRuntimeHealthUnavailable {
		t.Fatalf("aggregate unavailable=%s", got)
	}
	retired := assistantRuntimeNormalizeService(assistantRuntimeService{
		Name:     "mongodb",
		Required: true,
		Healthy:  assistantRuntimeHealthUnavailable,
		Reason:   assistantRuntimeReasonRetiredByDesign,
	})
	if retired.Required {
		t.Fatalf("retired service should no longer be required: %+v", retired)
	}
	if retired.Healthy != assistantRuntimeHealthRetired {
		t.Fatalf("retired service health=%s", retired.Healthy)
	}
	if got := assistantRuntimeAggregateStatus([]assistantRuntimeService{
		{Name: "api", Required: true, Healthy: assistantRuntimeHealthHealthy},
		{Name: "mongodb", Required: false, Healthy: assistantRuntimeHealthUnavailable, Reason: assistantRuntimeReasonRetiredByDesign},
	}); got != assistantRuntimeHealthHealthy {
		t.Fatalf("aggregate with retired dependency=%s", got)
	}

	if got := assistantRuntimeLockReadErrorCode(os.ErrNotExist); got != "assistant_runtime_versions_lock_missing" {
		t.Fatalf("code=%s", got)
	}
	if got := assistantRuntimeLockReadErrorCode(errors.New("bad")); got != "assistant_runtime_versions_lock_invalid" {
		t.Fatalf("code=%s", got)
	}
	if got := assistantRuntimeDomainPolicyErrorCode(errAssistantDomainPolicyMissing); got != "assistant_oss_domain_policy_missing" {
		t.Fatalf("domain policy code=%s", got)
	}
	if got := assistantRuntimeDomainPolicyErrorCode(errAssistantDomainPolicyInvalid); got != "assistant_oss_domain_policy_invalid" {
		t.Fatalf("domain policy code=%s", got)
	}
	if got := assistantRuntimeDomainPolicyErrorMessage(errAssistantDomainPolicyMissing); got != "assistant domain allowlist policy is missing" {
		t.Fatalf("domain policy message=%q", got)
	}
	if got := assistantRuntimeDomainPolicyErrorMessage(errAssistantDomainPolicyInvalid); got != "assistant domain allowlist policy is invalid" {
		t.Fatalf("domain policy message=%q", got)
	}
}

func TestAssistantRuntimeReadAndResolvePath(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "versions.lock.yaml")
	statusPath := filepath.Join(dir, "runtime-status.json")

	if err := os.WriteFile(lockPath, []byte("services:\n  - name: api\n    required: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statusPath, []byte("{\"status\":\"healthy\",\"checked_at\":\"2026-03-03T00:00:00Z\",\"upstream\":{\"url\":\"http://127.0.0.1:3080\"},\"services\":[]}"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ASSISTANT_RUNTIME_VERSIONS_LOCK", lockPath)
	t.Setenv("ASSISTANT_RUNTIME_STATUS_FILE", statusPath)
	if _, err := readAssistantRuntimeVersionsLock(); err != nil {
		t.Fatalf("read lock: %v", err)
	}
	if _, err := readAssistantRuntimeSnapshot(); err != nil {
		t.Fatalf("read snapshot: %v", err)
	}

	badLock := filepath.Join(dir, "bad.lock.yaml")
	if err := os.WriteFile(badLock, []byte("services: [:"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ASSISTANT_RUNTIME_VERSIONS_LOCK", badLock)
	if _, err := readAssistantRuntimeVersionsLock(); err == nil {
		t.Fatal("expected bad lock yaml error")
	}

	badSnapshot := filepath.Join(dir, "bad.status.json")
	if err := os.WriteFile(badSnapshot, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ASSISTANT_RUNTIME_STATUS_FILE", badSnapshot)
	if _, err := readAssistantRuntimeSnapshot(); err == nil {
		t.Fatal("expected bad snapshot json error")
	}

	abs := assistantRuntimeResolvePath(lockPath, "fallback")
	if abs != lockPath {
		t.Fatalf("resolve abs=%s", abs)
	}

	nested := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	relativeTarget := filepath.Join(dir, "a", "sample.lock")
	if err := os.WriteFile(relativeTarget, []byte("services: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}
	existing := assistantRuntimeResolvePath("../sample.lock", "fallback")
	if existing != filepath.Join("..", "sample.lock") {
		t.Fatalf("existing=%s", existing)
	}
	resolved := assistantRuntimeResolvePath("sample.lock", "fallback")
	if resolved != filepath.Join("..", "sample.lock") {
		t.Fatalf("resolved=%s", resolved)
	}
	missing := assistantRuntimeResolvePath("missing.lock", "fallback.lock")
	if missing != "missing.lock" {
		t.Fatalf("missing=%s", missing)
	}
}
