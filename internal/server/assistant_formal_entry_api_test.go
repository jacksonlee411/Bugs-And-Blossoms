package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func writeAssistantFormalEntryRuntimeFixtures(t *testing.T) {
	t.Helper()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstream.Close)

	dir := t.TempDir()
	lockPath := filepath.Join(dir, "versions.lock.yaml")
	snapshotPath := filepath.Join(dir, "runtime-status.json")

	lock := `upstream:
  repo: "danny-avila/LibreChat"
  ref: "main"
  imported_at: "2026-04-12T00:00:00Z"
  rollback_ref: "abc123"
services:
  - name: "api"
    required: true
    image: "ghcr.io/danny-avila/librechat"
    tag: "v0.0.1"
    digest: "sha256:1111111111111111111111111111111111111111111111111111111111111111"
`
	if err := os.WriteFile(lockPath, []byte(lock), 0o644); err != nil {
		t.Fatal(err)
	}
	snapshot := `{
  "status": "healthy",
  "checked_at": "2026-04-12T00:00:00Z",
  "services": [
    {"name":"api","required":true,"healthy":"healthy"}
  ]
}`
	if err := os.WriteFile(snapshotPath, []byte(snapshot), 0o644); err != nil {
		t.Fatal(err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("LIBRECHAT_UPSTREAM", upstream.URL)
	t.Setenv("ASSISTANT_RUNTIME_VERSIONS_LOCK", lockPath)
	t.Setenv("ASSISTANT_RUNTIME_STATUS_FILE", snapshotPath)
	t.Setenv("ASSISTANT_DOMAIN_ALLOWLIST_PATH", filepath.Clean(filepath.Join(wd, "..", "..", "config", "assistant", "domain-allowlist.yaml")))
}

func assistantFormalEntryAuthedRequest(method string, path string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.AddCookie(&http.Cookie{Name: sidCookieName, Value: "sid-1"})
	return req.WithContext(
		withTenant(
			withPrincipal(req.Context(), Principal{
				ID:       "principal_1",
				TenantID: "tenant_1",
				Email:    "tenant-admin@example.invalid",
				Status:   "active",
				RoleSlug: "tenant-admin",
			}),
			Tenant{ID: "tenant_1", Domain: "localhost", Name: "Local"},
		),
	)
}

func TestAssistantFormalEntryAPI_UIBootstrapSuccess(t *testing.T) {
	writeAssistantFormalEntryRuntimeFixtures(t)

	handler := newAssistantFormalEntryAPIHandler(&assistantConversationService{
		modelGateway: &assistantModelGateway{
			config: assistantModelConfig{
				Providers: []assistantModelProviderConfig{
					{Name: "openai", Enabled: true, Model: "gpt-5.4", Endpoint: "simulate://ok", TimeoutMS: 1000, Retries: 1, Priority: 1, KeyRef: "OPENAI_API_KEY"},
				},
			},
		},
	}, nil)

	rec := httptest.NewRecorder()
	handler.handleUIBootstrap(rec, assistantFormalEntryAuthedRequest(http.MethodGet, "/internal/assistant/ui-bootstrap"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload assistantFormalUIBootstrapResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ContractVersion != assistantFormalContractVersion {
		t.Fatalf("contract=%q", payload.ContractVersion)
	}
	if payload.Viewer.ID != "principal_1" {
		t.Fatalf("viewer=%+v", payload.Viewer)
	}
	if payload.Runtime.RuntimeCutoverMode != assistantRuntimeCutoverModeUIShellOnly {
		t.Fatalf("runtime=%+v", payload.Runtime)
	}
	if len(payload.Models) != 1 || payload.Models[0].Model != "gpt-5.4" {
		t.Fatalf("models=%+v", payload.Models)
	}
	if payload.UI.AgentsUIEnabled || payload.UI.MemoryEnabled || payload.UI.WebSearchEnabled || payload.UI.FileSearchEnabled || payload.UI.CodeInterpreterEnabled {
		t.Fatalf("ui=%+v", payload.UI)
	}
	if !payload.UI.ArtifactsEnabled {
		t.Fatalf("ui=%+v", payload.UI)
	}
}

func TestAssistantFormalEntryAPI_UIBootstrapUnavailableWhenRuntimeBaselineMissing(t *testing.T) {
	writeAssistantFormalEntryRuntimeFixtures(t)
	t.Setenv("ASSISTANT_DOMAIN_ALLOWLIST_PATH", filepath.Join(t.TempDir(), "missing-domain-policy.yaml"))

	handler := newAssistantFormalEntryAPIHandler(&assistantConversationService{
		modelGateway: &assistantModelGateway{
			config: assistantModelConfig{
				Providers: []assistantModelProviderConfig{
					{Name: "openai", Enabled: true, Model: "gpt-5.4", Endpoint: "simulate://ok", TimeoutMS: 1000, Retries: 1, Priority: 1, KeyRef: "OPENAI_API_KEY"},
				},
			},
		},
	}, nil)

	rec := httptest.NewRecorder()
	handler.handleUIBootstrap(rec, assistantFormalEntryAuthedRequest(http.MethodGet, "/internal/assistant/ui-bootstrap"))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["code"] != assistantUIBootstrapUnavailableCode {
		t.Fatalf("code=%v body=%s", payload["code"], rec.Body.String())
	}
}

func TestAssistantFormalEntryAPI_SessionRefreshAndLogout(t *testing.T) {
	sessions := &stubSessionStore{}
	handler := newAssistantFormalEntryAPIHandler(nil, sessions)

	sessionReq := assistantFormalEntryAuthedRequest(http.MethodGet, "/internal/assistant/session")
	sessionRec := httptest.NewRecorder()
	handler.handleSession(sessionRec, sessionReq)
	if sessionRec.Code != http.StatusOK {
		t.Fatalf("session status=%d body=%s", sessionRec.Code, sessionRec.Body.String())
	}

	refreshRec := httptest.NewRecorder()
	handler.handleSessionRefresh(refreshRec, assistantFormalEntryAuthedRequest(http.MethodPost, "/internal/assistant/session/refresh"))
	if refreshRec.Code != http.StatusOK {
		t.Fatalf("refresh status=%d body=%s", refreshRec.Code, refreshRec.Body.String())
	}
	var refreshPayload assistantFormalSessionRefreshResponse
	if err := json.Unmarshal(refreshRec.Body.Bytes(), &refreshPayload); err != nil {
		t.Fatal(err)
	}
	if refreshPayload.RefreshedAt == "" || !refreshPayload.Authenticated {
		t.Fatalf("payload=%+v", refreshPayload)
	}

	logoutRec := httptest.NewRecorder()
	handler.handleSessionLogout(logoutRec, assistantFormalEntryAuthedRequest(http.MethodPost, "/internal/assistant/session/logout"))
	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("logout status=%d body=%s", logoutRec.Code, logoutRec.Body.String())
	}
	if sessions.revokeSID != "sid-1" {
		t.Fatalf("revokeSID=%q", sessions.revokeSID)
	}
}

func TestAssistantFormalEntryAPI_LogoutRejectsMissingSID(t *testing.T) {
	handler := newAssistantFormalEntryAPIHandler(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/internal/assistant/session/logout", nil)
	req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "p1", Email: "u@example.invalid", Status: "active"}), Tenant{ID: "t1"}))
	rec := httptest.NewRecorder()
	handler.handleSessionLogout(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
