package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLibreChatVendoredCompatAPIRetiredWithoutSession(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	t.Setenv("ALLOWLIST_PATH", allowlistPath)

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver:  localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{Email: "tenant-admin@example.invalid", KratosIdentityID: "kid1"}},
		OrgUnitStore:     newOrgUnitMemoryStore(),
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, libreChatCompatAPIPrefix+"/user", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusGone {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["code"] != libreChatCompatRetiredCode {
		t.Fatalf("code=%v", payload["code"])
	}
	if !strings.Contains(payload["message"].(string), "/internal/assistant/session") {
		t.Fatalf("message=%v", payload["message"])
	}
}

func TestLibreChatVendoredCompatAPIFormalEntryAliasReturnsGone(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	t.Setenv("ALLOWLIST_PATH", allowlistPath)

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver:  localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{Email: "tenant-admin@example.invalid", KratosIdentityID: "kid1"}},
		OrgUnitStore:     newOrgUnitMemoryStore(),
	})
	if err != nil {
		t.Fatal(err)
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/iam/api/sessions", strings.NewReader(`{"email":"tenant-admin@example.invalid","password":"pw"}`))
	loginReq.Host = "localhost:8080"
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	h.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusNoContent {
		t.Fatalf("login status=%d body=%s", loginRec.Code, loginRec.Body.String())
	}
	sidCookie := loginRec.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodPost, libreChatFormalEntryAPIPrefix+"/auth/refresh", http.NoBody)
	req.Host = "localhost:8080"
	req.AddCookie(sidCookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusGone {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["code"] != libreChatCompatRetiredCode {
		t.Fatalf("code=%v", payload["code"])
	}
}

func TestLibreChatVendoredCompatAPIRetiredWithSIDSession(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	t.Setenv("ALLOWLIST_PATH", allowlistPath)

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver:  localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{Email: "tenant-admin@example.invalid", KratosIdentityID: "kid1"}},
		OrgUnitStore:     newOrgUnitMemoryStore(),
	})
	if err != nil {
		t.Fatal(err)
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/iam/api/sessions", strings.NewReader(`{"email":"tenant-admin@example.invalid","password":"pw"}`))
	loginReq.Host = "localhost:8080"
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	h.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusNoContent {
		t.Fatalf("login status=%d body=%s", loginRec.Code, loginRec.Body.String())
	}
	result := loginRec.Result()
	if len(result.Cookies()) == 0 {
		t.Fatal("expected sid cookie")
	}
	sidCookie := result.Cookies()[0]

	t.Run("session compat endpoints return gone", func(t *testing.T) {
		cases := []struct {
			name      string
			method    string
			path      string
			successor string
		}{
			{name: "refresh-compat", method: http.MethodPost, path: libreChatCompatAPIPrefix + "/auth/refresh", successor: "/internal/assistant/session/refresh"},
			{name: "logout-compat", method: http.MethodPost, path: libreChatCompatAPIPrefix + "/auth/logout", successor: "/internal/assistant/session/logout"},
			{name: "user-compat", method: http.MethodGet, path: libreChatCompatAPIPrefix + "/user", successor: "/internal/assistant/session"},
			{name: "role-user-compat", method: http.MethodGet, path: libreChatCompatAPIPrefix + "/roles/user", successor: "/internal/assistant/session"},
			{name: "role-admin-compat", method: http.MethodGet, path: libreChatCompatAPIPrefix + "/roles/admin", successor: "/internal/assistant/session"},
			{name: "refresh-formal-alias", method: http.MethodPost, path: libreChatFormalEntryAPIPrefix + "/auth/refresh", successor: "/internal/assistant/session/refresh"},
			{name: "user-formal-alias", method: http.MethodGet, path: libreChatFormalEntryAPIPrefix + "/user", successor: "/internal/assistant/session"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest(tc.method, tc.path, nil)
				req.Host = "localhost:8080"
				req.AddCookie(sidCookie)
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)
				if rec.Code != http.StatusGone {
					t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
				}
				var payload map[string]any
				if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
					t.Fatal(err)
				}
				if payload["code"] != libreChatCompatRetiredCode {
					t.Fatalf("code=%v", payload["code"])
				}
				if !strings.Contains(payload["message"].(string), tc.successor) {
					t.Fatalf("message=%v", payload["message"])
				}
			})
		}
	})

	t.Run("removed bootstrap compat routes return not found", func(t *testing.T) {
		removedBootstrapRoutes := []string{
			libreChatCompatAPIPrefix + "/config",
			libreChatCompatAPIPrefix + "/endpoints",
			libreChatCompatAPIPrefix + "/models",
			libreChatFormalEntryAPIPrefix + "/config",
			libreChatFormalEntryAPIPrefix + "/endpoints",
			libreChatFormalEntryAPIPrefix + "/models",
		}
		for _, path := range removedBootstrapRoutes {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Host = "localhost:8080"
			req.AddCookie(sidCookie)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("path=%s status=%d body=%s", path, rec.Code, rec.Body.String())
			}
		}
	})
}

func TestLibreChatVendoredCompatAPIHelperCoverage(t *testing.T) {
	t.Run("removed writer returns not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, libreChatCompatAPIPrefix+"/config", nil)
		rec := httptest.NewRecorder()
		writeLibreChatCompatEndpointRemoved(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("compat providers and model error branches", func(t *testing.T) {
		if providers, code, _ := assistantStartupProviders(nil); len(providers) != 0 || code != "ai_runtime_config_missing" {
			t.Fatalf("providers=%v code=%q", providers, code)
		}

		emptySvc := &assistantConversationService{modelGateway: &assistantModelGateway{config: assistantModelConfig{}}}
		if providers, code, _ := assistantStartupProviders(emptySvc); len(providers) != 0 || code != "assistant_startup_endpoints_unavailable" {
			t.Fatalf("providers=%v code=%q", providers, code)
		}

		sortedSvc := &assistantConversationService{modelGateway: &assistantModelGateway{config: assistantModelConfig{Providers: []assistantModelProviderConfig{{Name: "zeta", Enabled: true, Model: "m2", Priority: 20}, {Name: "alpha", Enabled: true, Model: "m1", Priority: 10}, {Name: "beta", Enabled: true, Model: "m3", Priority: 10}}}}}
		providers, code, message := assistantStartupProviders(sortedSvc)
		if code != "" || message != "" || len(providers) != 3 {
			t.Fatalf("providers=%v code=%q message=%q", providers, code, message)
		}
		if providers[0].Name != "alpha" || providers[1].Name != "beta" || providers[2].Name != "zeta" {
			t.Fatalf("providers=%v", providers)
		}
	})

	t.Run("user context helpers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if _, _, ok := libreChatCompatUserFromRequest(req); ok {
			t.Fatal("expected missing tenant to fail")
		}
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		if _, _, ok := libreChatCompatUserFromRequest(req); ok {
			t.Fatal("expected missing principal to fail")
		}
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1", Email: "tenant-admin@example.invalid"}))
		if _, _, ok := libreChatCompatUserFromRequest(req); ok {
			t.Fatal("expected missing sid to fail")
		}
		req.AddCookie(&http.Cookie{Name: sidCookieName, Value: "sid-1"})
		user, token, ok := libreChatCompatUserFromRequest(req)
		if !ok {
			t.Fatal("expected compat user")
		}
		if user.Username != "tenant-admin" {
			t.Fatalf("username=%q", user.Username)
		}
		if user.Name != "Tenant Admin" {
			t.Fatalf("name=%q", user.Name)
		}
		if token == "" {
			t.Fatal("expected compat token")
		}
	})

	t.Run("string and endpoint helpers", func(t *testing.T) {
		if got := libreChatCompatUsername(Principal{ID: "p1"}); got != "p1" {
			t.Fatalf("got=%q", got)
		}
		if got := libreChatCompatUsername(Principal{Email: "plain"}); got != "plain" {
			t.Fatalf("got=%q", got)
		}
		if got := libreChatCompatDisplayName(Principal{ID: "p1"}); got != "P1" {
			t.Fatalf("got=%q", got)
		}
		if got := libreChatCompatDisplayName(Principal{}); got != "" {
			t.Fatalf("got=%q", got)
		}
		if got := libreChatCompatTitle(""); got != "Custom" {
			t.Fatalf("got=%q", got)
		}
		if got := libreChatCompatTitle("deepseek-chat"); got != "Deepseek Chat" {
			t.Fatalf("got=%q", got)
		}
		if !libreChatCompatModelExists([]string{"a", "b"}, " a ") {
			t.Fatal("expected model hit")
		}
		if libreChatCompatModelExists([]string{"a", "b"}, "c") {
			t.Fatal("unexpected model hit")
		}

		cases := []struct {
			provider assistantModelProviderConfig
			wantKey  string
			wantType string
			wantName string
		}{
			{provider: assistantModelProviderConfig{Name: "openai"}, wantKey: "openAI", wantType: "openAI", wantName: "OpenAI"},
			{provider: assistantModelProviderConfig{Name: "claude"}, wantKey: "anthropic", wantType: "anthropic", wantName: "Anthropic"},
			{provider: assistantModelProviderConfig{Name: "gemini"}, wantKey: "google", wantType: "google", wantName: "Google"},
			{provider: assistantModelProviderConfig{Name: "bedrock"}, wantKey: "bedrock", wantType: "bedrock", wantName: "Bedrock"},
			{provider: assistantModelProviderConfig{Name: "azure-openai"}, wantKey: "azureOpenAI", wantType: "azureOpenAI", wantName: "Azure OpenAI"},
			{provider: assistantModelProviderConfig{Name: "assistants"}, wantKey: "assistants", wantType: "assistants", wantName: "Assistants"},
			{provider: assistantModelProviderConfig{Name: "azure-assistants"}, wantKey: "azureAssistants", wantType: "azureAssistants", wantName: "Azure Assistants"},
			{provider: assistantModelProviderConfig{Name: "deepseek"}, wantKey: "deepseek", wantType: "custom", wantName: "Deepseek"},
			{provider: assistantModelProviderConfig{Name: ""}, wantKey: "custom", wantType: "custom", wantName: "Custom"},
		}
		for _, tc := range cases {
			gotKey, gotType := libreChatCompatEndpoint(tc.provider)
			if gotKey != tc.wantKey || gotType != tc.wantType {
				t.Fatalf("provider=%q got=(%q,%q) want=(%q,%q)", tc.provider.Name, gotKey, gotType, tc.wantKey, tc.wantType)
			}
			if got := libreChatCompatEndpointLabel(tc.provider, gotKey); got != tc.wantName {
				t.Fatalf("provider=%q got label=%q want=%q", tc.provider.Name, got, tc.wantName)
			}
		}
		if suffix, ok := libreChatCompatAPISuffix(libreChatCompatAPIPrefix); !ok || suffix != "" {
			t.Fatalf("compat prefix suffix=%q ok=%v", suffix, ok)
		}
		if suffix, ok := libreChatCompatAPISuffix(libreChatFormalEntryAPIPrefix); !ok || suffix != "" {
			t.Fatalf("formal prefix suffix=%q ok=%v", suffix, ok)
		}
		if successor, ok := libreChatCompatRetiredSuccessorForPath(libreChatCompatAPIPrefix + "/auth/refresh"); !ok || successor != "/internal/assistant/session/refresh" {
			t.Fatalf("successor=%q ok=%v", successor, ok)
		}
		if successor, ok := libreChatCompatRetiredSuccessorForSuffix("/roles/admin"); !ok || successor != "/internal/assistant/session" {
			t.Fatalf("successor=%q ok=%v", successor, ok)
		}
		if successor, ok := libreChatCompatRetiredSuccessorForPath(libreChatFormalEntryAPIPrefix + "/auth/logout"); !ok || successor != "/internal/assistant/session/logout" {
			t.Fatalf("successor=%q ok=%v", successor, ok)
		}
		if !isLibreChatCompatAPIPath(libreChatFormalEntryAPIPrefix + "/models") {
			t.Fatal("expected formal alias removed path to be recognized as compat api")
		}
	})
}
