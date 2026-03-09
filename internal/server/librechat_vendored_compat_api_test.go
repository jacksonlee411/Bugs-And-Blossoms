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

func TestLibreChatVendoredCompatAPIRequiresSession(t *testing.T) {
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
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["code"] != "assistant_vendored_sid_missing" {
		t.Fatalf("code=%v", payload["code"])
	}
}

func TestLibreChatVendoredCompatAPIFormalEntryAliasWorksWithSIDSession(t *testing.T) {
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
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestLibreChatVendoredCompatAPIWorksWithSIDSession(t *testing.T) {
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

	t.Run("refresh returns compat token and user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, libreChatCompatAPIPrefix+"/auth/refresh", http.NoBody)
		req.Host = "localhost:8080"
		req.AddCookie(sidCookie)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var payload struct {
			Token string `json:"token"`
			User  struct {
				ID       string `json:"id"`
				Email    string `json:"email"`
				Role     string `json:"role"`
				Provider string `json:"provider"`
			} `json:"user"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(payload.Token, "compat-sid.") {
			t.Fatalf("token=%q", payload.Token)
		}
		if payload.User.Email != "tenant-admin@example.invalid" {
			t.Fatalf("email=%q", payload.User.Email)
		}
		if payload.User.Role != libreChatCompatRoleUser {
			t.Fatalf("role=%q", payload.User.Role)
		}
		if payload.User.Provider != libreChatCompatProvider {
			t.Fatalf("provider=%q", payload.User.Provider)
		}
	})

	t.Run("user config endpoints models roles", func(t *testing.T) {
		cases := []struct {
			name string
			path string
		}{
			{name: "user", path: libreChatCompatAPIPrefix + "/user"},
			{name: "config", path: libreChatCompatAPIPrefix + "/config"},
			{name: "endpoints", path: libreChatCompatAPIPrefix + "/endpoints"},
			{name: "models", path: libreChatCompatAPIPrefix + "/models"},
			{name: "role-user", path: libreChatCompatAPIPrefix + "/roles/user"},
			{name: "role-admin", path: libreChatCompatAPIPrefix + "/roles/admin"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, tc.path, nil)
				req.Host = "localhost:8080"
				req.AddCookie(sidCookie)
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
				}
			})
		}

		var configPayload map[string]any
		configReq := httptest.NewRequest(http.MethodGet, libreChatCompatAPIPrefix+"/config", nil)
		configReq.Host = "localhost:8080"
		configReq.AddCookie(sidCookie)
		configRec := httptest.NewRecorder()
		h.ServeHTTP(configRec, configReq)
		if err := json.Unmarshal(configRec.Body.Bytes(), &configPayload); err != nil {
			t.Fatal(err)
		}
		if configPayload["appTitle"] != "Bugs & Blossoms Assistant" {
			t.Fatalf("appTitle=%v", configPayload["appTitle"])
		}

		var endpointsPayload map[string]map[string]any
		endpointsReq := httptest.NewRequest(http.MethodGet, libreChatCompatAPIPrefix+"/endpoints", nil)
		endpointsReq.Host = "localhost:8080"
		endpointsReq.AddCookie(sidCookie)
		endpointsRec := httptest.NewRecorder()
		h.ServeHTTP(endpointsRec, endpointsReq)
		if err := json.Unmarshal(endpointsRec.Body.Bytes(), &endpointsPayload); err != nil {
			t.Fatal(err)
		}
		if _, ok := endpointsPayload["openAI"]; !ok {
			t.Fatalf("payload=%v", endpointsPayload)
		}

		var modelsPayload map[string][]string
		modelsReq := httptest.NewRequest(http.MethodGet, libreChatCompatAPIPrefix+"/models", nil)
		modelsReq.Host = "localhost:8080"
		modelsReq.AddCookie(sidCookie)
		modelsRec := httptest.NewRecorder()
		h.ServeHTTP(modelsRec, modelsReq)
		if err := json.Unmarshal(modelsRec.Body.Bytes(), &modelsPayload); err != nil {
			t.Fatal(err)
		}
		if got := modelsPayload["openAI"]; len(got) != 1 || got[0] != "gpt-5-codex" {
			t.Fatalf("models=%v", modelsPayload)
		}
	})

	t.Run("logout revokes sid-backed compat session", func(t *testing.T) {
		logoutReq := httptest.NewRequest(http.MethodPost, libreChatCompatAPIPrefix+"/auth/logout", http.NoBody)
		logoutReq.Host = "localhost:8080"
		logoutReq.AddCookie(sidCookie)
		logoutRec := httptest.NewRecorder()
		h.ServeHTTP(logoutRec, logoutReq)
		if logoutRec.Code != http.StatusNoContent {
			t.Fatalf("status=%d body=%s", logoutRec.Code, logoutRec.Body.String())
		}

		userReq := httptest.NewRequest(http.MethodGet, libreChatCompatAPIPrefix+"/user", nil)
		userReq.Host = "localhost:8080"
		userReq.Header.Set("Accept", "application/json")
		userReq.AddCookie(sidCookie)
		userRec := httptest.NewRecorder()
		h.ServeHTTP(userRec, userReq)
		if userRec.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d body=%s", userRec.Code, userRec.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(userRec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload["code"] != "assistant_vendored_session_invalid" {
			t.Fatalf("code=%v", payload["code"])
		}
	})
}

func TestLibreChatVendoredCompatAPIHandler_UnitCoverage(t *testing.T) {
	t.Run("serve http not found", func(t *testing.T) {
		h := newLibreChatCompatAPIHandler(nil, nil)
		req := httptest.NewRequest(http.MethodGet, libreChatCompatAPIPrefix+"/unknown", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("serve http rejects non compat path", func(t *testing.T) {
		h := newLibreChatCompatAPIHandler(nil, nil)
		req := httptest.NewRequest(http.MethodGet, "/totally-different", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("method not allowed branches", func(t *testing.T) {
		h := newLibreChatCompatAPIHandler(nil, nil)
		paths := []string{
			libreChatCompatAPIPrefix + "/auth/refresh",
			libreChatCompatAPIPrefix + "/user",
			libreChatCompatAPIPrefix + "/roles/user",
			libreChatCompatAPIPrefix + "/config",
			libreChatCompatAPIPrefix + "/endpoints",
			libreChatCompatAPIPrefix + "/models",
		}
		for _, path := range paths {
			req := httptest.NewRequest(http.MethodDelete, path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("path=%s status=%d", path, rec.Code)
			}
		}
		logoutReq := httptest.NewRequest(http.MethodGet, libreChatCompatAPIPrefix+"/auth/logout", nil)
		logoutRec := httptest.NewRecorder()
		h.ServeHTTP(logoutRec, logoutReq)
		if logoutRec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("logout status=%d", logoutRec.Code)
		}
	})

	t.Run("unauthorized without context", func(t *testing.T) {
		h := newLibreChatCompatAPIHandler(nil, nil)
		paths := []struct {
			method string
			path   string
		}{
			{method: http.MethodPost, path: libreChatCompatAPIPrefix + "/auth/refresh"},
			{method: http.MethodGet, path: libreChatCompatAPIPrefix + "/user"},
			{method: http.MethodGet, path: libreChatCompatAPIPrefix + "/roles/user"},
			{method: http.MethodGet, path: libreChatCompatAPIPrefix + "/config"},
			{method: http.MethodGet, path: libreChatCompatAPIPrefix + "/endpoints"},
			{method: http.MethodGet, path: libreChatCompatAPIPrefix + "/models"},
		}
		for _, tc := range paths {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("path=%s status=%d", tc.path, rec.Code)
			}
		}
	})

	t.Run("logout without sid still clears cookie", func(t *testing.T) {
		h := newLibreChatCompatAPIHandler(nil, nil)
		req := httptest.NewRequest(http.MethodPost, libreChatCompatAPIPrefix+"/auth/logout", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("compat providers and model error branches", func(t *testing.T) {
		hMissing := &libreChatCompatAPIHandler{}
		if providers, code, _ := hMissing.compatProviders(); len(providers) != 0 || code != "ai_runtime_config_missing" {
			t.Fatalf("providers=%v code=%q", providers, code)
		}

		hEmpty := &libreChatCompatAPIHandler{assistantSvc: &assistantConversationService{modelGateway: &assistantModelGateway{config: assistantModelConfig{}}}}
		if providers, code, _ := hEmpty.compatProviders(); len(providers) != 0 || code != "assistant_startup_endpoints_unavailable" {
			t.Fatalf("providers=%v code=%q", providers, code)
		}

		modelsOnlyEmpty := &libreChatCompatAPIHandler{assistantSvc: &assistantConversationService{modelGateway: &assistantModelGateway{config: assistantModelConfig{Providers: []assistantModelProviderConfig{{Name: "custom-provider", Enabled: true, Model: ""}}}}}}
		req := httptest.NewRequest(http.MethodGet, libreChatCompatAPIPrefix+"/models", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "p1", Email: "u@example.invalid"}), Tenant{ID: "t1"}))
		req.AddCookie(&http.Cookie{Name: sidCookieName, Value: "sid-1"})
		rec := httptest.NewRecorder()
		modelsOnlyEmpty.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload["code"] != "assistant_startup_models_unavailable" {
			t.Fatalf("code=%v", payload["code"])
		}

		authedReq := httptest.NewRequest(http.MethodGet, libreChatCompatAPIPrefix+"/endpoints", nil)
		authedReq = authedReq.WithContext(withTenant(withPrincipal(authedReq.Context(), Principal{ID: "p1", Email: "u@example.invalid"}), Tenant{ID: "t1"}))
		authedReq.AddCookie(&http.Cookie{Name: sidCookieName, Value: "sid-1"})
		authedRec := httptest.NewRecorder()
		hEmpty.ServeHTTP(authedRec, authedReq)
		if authedRec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d body=%s", authedRec.Code, authedRec.Body.String())
		}

		modelsReqEmptyProviders := httptest.NewRequest(http.MethodGet, libreChatCompatAPIPrefix+"/models", nil)
		modelsReqEmptyProviders = modelsReqEmptyProviders.WithContext(withTenant(withPrincipal(modelsReqEmptyProviders.Context(), Principal{ID: "p1", Email: "u@example.invalid"}), Tenant{ID: "t1"}))
		modelsReqEmptyProviders.AddCookie(&http.Cookie{Name: sidCookieName, Value: "sid-1"})
		modelsRecEmptyProviders := httptest.NewRecorder()
		hEmpty.ServeHTTP(modelsRecEmptyProviders, modelsReqEmptyProviders)
		if modelsRecEmptyProviders.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d body=%s", modelsRecEmptyProviders.Code, modelsRecEmptyProviders.Body.String())
		}

		hSorted := &libreChatCompatAPIHandler{assistantSvc: &assistantConversationService{modelGateway: &assistantModelGateway{config: assistantModelConfig{Providers: []assistantModelProviderConfig{{Name: "zeta", Enabled: true, Model: "m2", Priority: 20}, {Name: "alpha", Enabled: true, Model: "m1", Priority: 10}, {Name: "beta", Enabled: true, Model: "m3", Priority: 10}}}}}}
		providers, code, message := hSorted.compatProviders()
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
	})
}
