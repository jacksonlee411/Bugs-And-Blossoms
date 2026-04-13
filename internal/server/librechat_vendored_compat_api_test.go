package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLibreChatVendoredCompatAPIHelperCoverage(t *testing.T) {
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
	})
}
