package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

type assistantAdapterFunc func(ctx context.Context, prompt string, provider assistantModelProviderConfig) ([]byte, error)

func (f assistantAdapterFunc) Invoke(ctx context.Context, prompt string, provider assistantModelProviderConfig) ([]byte, error) {
	return f(ctx, prompt, provider)
}

func TestAssistantModelGateway_BranchCoverage(t *testing.T) {
	originalMarshal := assistantIntentMarshalFn
	defer func() { assistantIntentMarshalFn = originalMarshal }()

	if _, err := (assistantDeterministicProviderAdapter{}).Invoke(context.Background(), "x", assistantModelProviderConfig{Endpoint: "simulate://rate-limit"}); !errors.Is(err, errAssistantModelRateLimited) {
		t.Fatalf("unexpected err=%v", err)
	}
	if _, err := (assistantDeterministicProviderAdapter{}).Invoke(context.Background(), "x", assistantModelProviderConfig{Endpoint: "simulate://unavailable"}); !errors.Is(err, errAssistantModelProviderUnavailable) {
		t.Fatalf("unexpected err=%v", err)
	}
	assistantIntentMarshalFn = func(any) ([]byte, error) { return nil, errors.New("marshal failed") }
	if _, err := (assistantDeterministicProviderAdapter{}).Invoke(context.Background(), "x", assistantModelProviderConfig{Endpoint: "builtin://openai"}); !errors.Is(err, errAssistantPlanSchemaConstrainedDecodeFailed) {
		t.Fatalf("unexpected err=%v", err)
	}
	assistantIntentMarshalFn = originalMarshal

	t.Setenv("ASSISTANT_MODEL_CONFIG_JSON", `{"provider_routing":{"strategy":"priority_failover","fallback_enabled":true},"providers":[{"name":"openai","enabled":true,"model":"gpt-4o-mini","endpoint":"builtin://openai","timeout_ms":1000,"retries":0,"priority":1,"key_ref":"OPENAI_API_KEY"}]}`)
	gw := newAssistantModelGateway()
	if len(gw.snapshot().Providers) != 1 {
		t.Fatalf("expected 1 provider")
	}
	if err := os.Setenv("ASSISTANT_MODEL_CONFIG_JSON", "{"); err != nil {
		t.Fatalf("set env err=%v", err)
	}
	_ = newAssistantModelGateway()

	gw.mu.Lock()
	gw.config = assistantModelConfig{ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true}, Providers: []assistantModelProviderConfig{{Name: "openai", Enabled: false, Model: "m", Endpoint: "builtin://openai", TimeoutMS: 1, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"}, {Name: "deepseek", Enabled: true, Model: "m", Endpoint: "simulate://timeout", TimeoutMS: 1, Retries: 0, Priority: 2, KeyRef: "DEEPSEEK_API_KEY"}, {Name: "claude", Enabled: true, Model: "m", Endpoint: "simulate://rate-limit", TimeoutMS: 1, Retries: 0, Priority: 3, KeyRef: "ANTHROPIC_API_KEY"}, {Name: "gemini", Enabled: true, Model: "m", Endpoint: "https://example.invalid", TimeoutMS: 1, Retries: 0, Priority: 4, KeyRef: "MISSING_KEY"}, {Name: "openai", Enabled: true, Model: "m", Endpoint: "builtin://openai", TimeoutMS: 1, Retries: 0, Priority: 5, KeyRef: "OPENAI_API_KEY"}}}
	gw.adapters = map[string]assistantProviderAdapter{"openai": assistantDeterministicProviderAdapter{}, "deepseek": assistantDeterministicProviderAdapter{}, "claude": assistantDeterministicProviderAdapter{}, "gemini": nil}
	gw.mu.Unlock()
	_, statuses := gw.listProviderStatus()
	if len(statuses) == 0 {
		t.Fatal("expected status")
	}
	foundSecretMissing := false
	gw.mu.Lock()
	gw.config = assistantModelConfig{
		ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
		Providers: []assistantModelProviderConfig{
			{Name: "openai", Enabled: true, Model: "m", Endpoint: "https://example.invalid", TimeoutMS: 1, Retries: 0, Priority: 1, KeyRef: "MISSING_OPENAI_KEY"},
		},
	}
	gw.adapters = map[string]assistantProviderAdapter{"openai": assistantDeterministicProviderAdapter{}}
	gw.mu.Unlock()
	_, statuses = gw.listProviderStatus()
	for _, st := range statuses {
		if st.HealthReason == "secret_missing" {
			foundSecretMissing = true
		}
	}
	if !foundSecretMissing {
		t.Fatalf("expected secret_missing status, got=%+v", statuses)
	}

	_, errs := gw.validateConfig(assistantModelConfig{ProviderRouting: assistantProviderRouting{Strategy: "unsupported"}, Providers: []assistantModelProviderConfig{{Name: "openai", Enabled: true, Model: "", Endpoint: "", TimeoutMS: 0, Retries: -1, Priority: 1, KeyRef: ""}, {Name: "openai", Enabled: true, Model: "m", Endpoint: "builtin://openai", TimeoutMS: 1, Retries: 0, Priority: 1, KeyRef: "K"}}})
	if len(errs) == 0 {
		t.Fatal("expected validation errors")
	}
	normalized, errs := gw.validateConfig(assistantModelConfig{
		ProviderRouting: assistantProviderRouting{Strategy: ""},
		Providers: []assistantModelProviderConfig{
			{Name: "openai", Enabled: false, Model: "m", Endpoint: "builtin://openai", TimeoutMS: 1, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"},
			{Name: "openai", Enabled: true, Model: "m", Endpoint: "https://example.invalid", TimeoutMS: 1, Retries: 0, Priority: 2, KeyRef: "MISSING_VALIDATE_KEY"},
		},
	})
	if normalized.ProviderRouting.Strategy != "priority_failover" {
		t.Fatalf("unexpected normalized strategy=%s", normalized.ProviderRouting.Strategy)
	}
	if len(errs) == 0 {
		t.Fatal("expected secret missing when checkSecret enabled")
	}
	if _, errs := gw.applyConfig(assistantModelConfig{ProviderRouting: assistantProviderRouting{Strategy: "invalid"}}); len(errs) == 0 {
		t.Fatal("expected apply errors")
	}
	if models := gw.listModels(); len(models) == 0 {
		t.Fatal("expected enabled models")
	}
	if models := (&assistantModelGateway{config: assistantModelConfig{Providers: []assistantModelProviderConfig{
		{Name: "openai", Enabled: false, Model: "m", Endpoint: "builtin://openai", TimeoutMS: 1, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"},
		{Name: "deepseek", Enabled: true, Model: "m", Endpoint: "builtin://deepseek", TimeoutMS: 1, Retries: 0, Priority: 2, KeyRef: "DEEPSEEK_API_KEY"},
	}}}).listModels(); len(models) != 1 || models[0].Name != "deepseek" {
		t.Fatalf("unexpected listModels=%+v", models)
	}

	gw.mu.Lock()
	gw.config = assistantModelConfig{ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true}, Providers: []assistantModelProviderConfig{{Name: "bad", Enabled: true, Model: "m", Endpoint: "builtin://openai", TimeoutMS: 1, Retries: 0, Priority: 1, KeyRef: "K"}}}
	gw.mu.Unlock()
	if _, err := gw.ResolveIntent(context.Background(), assistantResolveIntentRequest{Prompt: "x"}); !errors.Is(err, errAssistantModelConfigInvalid) {
		t.Fatalf("unexpected err=%v", err)
	}

	gw.mu.Lock()
	gw.config = assistantModelConfig{ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true}, Providers: []assistantModelProviderConfig{{Name: "openai", Enabled: true, Model: "m", Endpoint: "https://example.invalid", TimeoutMS: 1, Retries: 0, Priority: 1, KeyRef: "MISSING_KEY"}}}
	gw.adapters = map[string]assistantProviderAdapter{"openai": assistantDeterministicProviderAdapter{}}
	gw.mu.Unlock()
	if _, err := gw.ResolveIntent(context.Background(), assistantResolveIntentRequest{Prompt: "x"}); !errors.Is(err, errAssistantModelSecretMissing) {
		t.Fatalf("unexpected err=%v", err)
	}

	gw.mu.Lock()
	gw.config = assistantModelConfig{
		ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
		Providers: []assistantModelProviderConfig{
			{Name: "openai", Enabled: false, Model: "m", Endpoint: "builtin://openai", TimeoutMS: 1, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"},
			{Name: "openai", Enabled: true, Model: "m", Endpoint: "builtin://openai", TimeoutMS: 1, Retries: 0, Priority: 2, KeyRef: "OPENAI_API_KEY"},
		},
	}
	gw.adapters = map[string]assistantProviderAdapter{"openai": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
		return []byte(`{"action":"plan_only"}`), nil
	})}
	gw.mu.Unlock()
	if resolved, err := gw.ResolveIntent(context.Background(), assistantResolveIntentRequest{Prompt: "x"}); err != nil || resolved.Intent.Action != "plan_only" {
		t.Fatalf("unexpected resolve result=%+v err=%v", resolved, err)
	}

	gw.mu.Lock()
	gw.config = assistantModelConfig{
		ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
		Providers: []assistantModelProviderConfig{
			{Name: "openai", Enabled: true, Model: "", Endpoint: "builtin://openai", TimeoutMS: 0, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"},
		},
	}
	gw.adapters = map[string]assistantProviderAdapter{"openai": assistantDeterministicProviderAdapter{}}
	gw.mu.Unlock()
	if _, err := gw.ResolveIntent(context.Background(), assistantResolveIntentRequest{Prompt: "x"}); !errors.Is(err, errAssistantModelConfigInvalid) {
		t.Fatalf("unexpected err=%v", err)
	}

	gw.mu.Lock()
	gw.config = assistantModelConfig{
		ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
		Providers: []assistantModelProviderConfig{
			{Name: "openai", Enabled: true, Model: "m", Endpoint: "builtin://openai", TimeoutMS: 1, Retries: 0, Priority: 1, KeyRef: ""},
		},
	}
	gw.adapters = map[string]assistantProviderAdapter{"openai": assistantDeterministicProviderAdapter{}}
	gw.mu.Unlock()
	if _, err := gw.ResolveIntent(context.Background(), assistantResolveIntentRequest{Prompt: "x"}); !errors.Is(err, errAssistantModelConfigInvalid) {
		t.Fatalf("unexpected err=%v", err)
	}

	gw.mu.Lock()
	gw.config = assistantModelConfig{
		ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
		Providers: []assistantModelProviderConfig{
			{Name: "openai", Enabled: true, Model: "m", Endpoint: "builtin://openai", TimeoutMS: 1, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"},
		},
	}
	gw.adapters = map[string]assistantProviderAdapter{"openai": nil}
	gw.mu.Unlock()
	if _, err := gw.ResolveIntent(context.Background(), assistantResolveIntentRequest{Prompt: "x"}); !errors.Is(err, errAssistantModelConfigInvalid) {
		t.Fatalf("unexpected err=%v", err)
	}

	gw.mu.Lock()
	gw.config = assistantModelConfig{
		ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
		Providers: []assistantModelProviderConfig{
			{Name: "openai", Enabled: true, Model: "m", Endpoint: "builtin://openai", TimeoutMS: 1, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"},
		},
	}
	gw.adapters = map[string]assistantProviderAdapter{"openai": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
		return nil, errors.New("invoke failed")
	})}
	gw.mu.Unlock()
	if _, err := gw.ResolveIntent(context.Background(), assistantResolveIntentRequest{Prompt: "x"}); err == nil || err.Error() != "invoke failed" {
		t.Fatalf("unexpected err=%v", err)
	}

	t.Setenv("OPENAI_API_KEY", "dummy")
	gw.mu.Lock()
	gw.config = assistantModelConfig{ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true}, Providers: []assistantModelProviderConfig{{Name: "openai", Enabled: true, Model: "m", Endpoint: "builtin://openai", TimeoutMS: 1, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"}}}
	gw.adapters = map[string]assistantProviderAdapter{"openai": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
		return []byte(`{"action":"create_orgunit","extra":1}`), nil
	})}
	gw.mu.Unlock()
	if _, err := gw.ResolveIntent(context.Background(), assistantResolveIntentRequest{Prompt: "x"}); !errors.Is(err, errAssistantPlanSchemaConstrainedDecodeFailed) {
		t.Fatalf("unexpected err=%v", err)
	}

	gw.mu.Lock()
	gw.adapters = map[string]assistantProviderAdapter{"openai": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
		return nil, errAssistantModelTimeout
	})}
	gw.mu.Unlock()
	if _, err := gw.ResolveIntent(context.Background(), assistantResolveIntentRequest{Prompt: "x"}); !errors.Is(err, errAssistantModelTimeout) {
		t.Fatalf("unexpected err=%v", err)
	}

	gw.mu.Lock()
	gw.config = assistantModelConfig{ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true}, Providers: nil}
	gw.mu.Unlock()
	if _, err := gw.ResolveIntent(context.Background(), assistantResolveIntentRequest{Prompt: "x"}); !errors.Is(err, errAssistantModelProviderUnavailable) {
		t.Fatalf("unexpected err=%v", err)
	}

	gw.mu.Lock()
	gw.config = assistantModelConfig{
		ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
		Providers: []assistantModelProviderConfig{
			{Name: "openai", Enabled: true, Model: "m", Endpoint: "builtin://openai", TimeoutMS: 1, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"},
			{Name: "deepseek", Enabled: true, Model: "m", Endpoint: "builtin://deepseek", TimeoutMS: 1, Retries: 0, Priority: 1, KeyRef: "DEEPSEEK_API_KEY"},
		},
	}
	gw.adapters = map[string]assistantProviderAdapter{
		"openai": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
			return []byte(`{"action":"plan_only"}`), nil
		}),
		"deepseek": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
			return []byte(`{"action":"plan_only"}`), nil
		}),
	}
	gw.mu.Unlock()
	if resolved, err := gw.ResolveIntent(context.Background(), assistantResolveIntentRequest{Prompt: "x"}); err != nil || resolved.ProviderName != "deepseek" {
		t.Fatalf("unexpected resolve result=%+v err=%v", resolved, err)
	}

	if !errorsIsAny(errors.New("x"), errors.New("y")) {
		// equals by value string does not match target pointer, expect false path
	}
	if errorsIsAny(nil, errAssistantModelTimeout) {
		t.Fatal("nil should not match")
	}

	if got := assistantCanonicalHash(map[string]any{"bad": func() {}}); got != "" {
		t.Fatalf("expected empty hash got=%q", got)
	}
	if got := assistantSkillManifestDigest([]string{"b", "a"}); got == "" {
		t.Fatal("digest should not be empty")
	}

	payload := map[string]any{"action": "create_orgunit"}
	raw, _ := json.Marshal(payload)
	intent, err := assistantStrictDecodeIntent(raw)
	if err != nil || intent.Action != assistantIntentCreateOrgUnit {
		t.Fatalf("unexpected strict decode err=%v intent=%+v", err, intent)
	}
}

func TestAssistantModelGateway_RuntimeEndpointValidation(t *testing.T) {
	t.Setenv("ASSISTANT_RUNTIME_ENV", "production")
	if !assistantEndpointInvalidForRuntime("builtin://openai") {
		t.Fatal("builtin endpoint should be invalid in production")
	}
	if assistantEndpointInvalidForRuntime("https://api.openai.com/v1") {
		t.Fatal("https endpoint should be valid in production")
	}
	_, errs := normalizeAssistantModelConfig(assistantModelConfig{
		ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
		Providers: []assistantModelProviderConfig{
			{Name: "openai", Enabled: true, Model: "gpt-5-codex", Endpoint: "builtin://openai", TimeoutMS: 1000, Retries: 1, Priority: 1, KeyRef: "OPENAI_API_KEY"},
		},
	}, false)
	if len(errs) == 0 {
		t.Fatal("expected endpoint validation error in production")
	}
}

func TestAssistantOpenAIProviderAdapter_InvokeAndParseContentArray(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method=%s", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":[{"type":"output_text","text":"{\"action\":\"create_orgunit\"}"}]}}]}`))
	}))
	defer server.Close()
	t.Setenv("OPENAI_API_KEY", "test-key")
	adapter := assistantOpenAIProviderAdapter{
		httpClient: server.Client(),
		fallback:   assistantDeterministicProviderAdapter{},
	}
	payload, err := adapter.Invoke(context.Background(), "在鲜花组织之下新建运营部", assistantModelProviderConfig{
		Name:      "openai",
		Model:     "gpt-5-codex",
		Endpoint:  server.URL + "/v1",
		TimeoutMS: 1000,
		KeyRef:    "OPENAI_API_KEY",
	})
	if err != nil {
		t.Fatalf("invoke err=%v", err)
	}
	intent, err := assistantStrictDecodeIntent(payload)
	if err != nil {
		t.Fatalf("strict decode err=%v", err)
	}
	if intent.Action != assistantIntentCreateOrgUnit {
		t.Fatalf("intent=%+v", intent)
	}
}
