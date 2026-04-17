package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

type assistantAdapterFunc func(ctx context.Context, prompt string, provider assistantModelProviderConfig) ([]byte, error)

func (f assistantAdapterFunc) Invoke(ctx context.Context, prompt string, provider assistantModelProviderConfig) ([]byte, error) {
	return f(ctx, prompt, provider)
}

type assistantRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f assistantRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type assistantErrReadCloser struct{}

func (assistantErrReadCloser) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (assistantErrReadCloser) Close() error             { return nil }

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

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("ASSISTANT_MODEL_CONFIG_JSON", `{"provider_routing":{"strategy":"priority_failover","fallback_enabled":true},"providers":[{"name":"openai","enabled":true,"model":"gpt-5-codex","endpoint":"https://api.openai.com/v1","timeout_ms":1000,"retries":0,"priority":1,"key_ref":"OPENAI_API_KEY"}]}`)
	gw, err := newAssistantModelGateway()
	if err != nil {
		t.Fatalf("new gateway err=%v", err)
	}
	if len(gw.snapshot().Providers) != 1 {
		t.Fatalf("expected 1 provider")
	}
	if err := os.Setenv("ASSISTANT_MODEL_CONFIG_JSON", "{"); err != nil {
		t.Fatalf("set env err=%v", err)
	}
	if _, err := newAssistantModelGateway(); !errors.Is(err, errAssistantRuntimeConfigInvalid) {
		t.Fatalf("expected runtime config invalid, got=%v", err)
	}

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
			{Name: "openai", Enabled: true, Model: "m", Endpoint: "https://api.openai.com/v1", TimeoutMS: 1, Retries: 0, Priority: 2, KeyRef: "OPENAI_API_KEY"},
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
			{Name: "openai", Enabled: true, Model: "m", Endpoint: "https://api.openai.com/v1", TimeoutMS: 1, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"},
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
			{Name: "openai", Enabled: true, Model: "m", Endpoint: "https://api.openai.com/v1", TimeoutMS: 1, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"},
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
	gw.config = assistantModelConfig{ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true}, Providers: []assistantModelProviderConfig{{Name: "openai", Enabled: true, Model: "m", Endpoint: "https://api.openai.com/v1", TimeoutMS: 1, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"}}}
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
			{Name: "openai", Enabled: true, Model: "m", Endpoint: "https://api.openai.com/v1", TimeoutMS: 1, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"},
			{Name: "deepseek", Enabled: true, Model: "m", Endpoint: "https://api.deepseek.com", TimeoutMS: 1, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"},
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

func TestAssistantModelGateway_NoDeterministicSwapInTestEnv(t *testing.T) {
	t.Setenv("ASSISTANT_RUNTIME_ENV", "test")
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("ASSISTANT_MODEL_CONFIG_JSON", `{"provider_routing":{"strategy":"priority_failover","fallback_enabled":true},"providers":[{"name":"openai","enabled":true,"model":"gpt-5-codex","endpoint":"https://api.openai.com/v1","timeout_ms":1000,"retries":0,"priority":1,"key_ref":"OPENAI_API_KEY"}]}`)
	gw, err := newAssistantModelGateway()
	if err != nil {
		t.Fatalf("new gateway err=%v", err)
	}
	adapter := gw.adapters["openai"]
	if adapter == nil {
		t.Fatal("openai adapter missing")
	}
	if _, ok := adapter.(assistantDeterministicProviderAdapter); ok {
		t.Fatal("openai adapter should stay real provider adapter in test env")
	}
	if _, ok := adapter.(assistantOpenAIProviderAdapter); !ok {
		t.Fatalf("unexpected openai adapter type=%T", adapter)
	}
}

func TestAssistantModelGateway_ListProviderStatus_ProbeConnectivity(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	modelsProbeCalls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path=%s", r.URL.Path)
		}
		modelsProbeCalls++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"gpt-5-codex"}]}`))
	}))
	defer server.Close()
	gw := &assistantModelGateway{
		config: assistantModelConfig{
			ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
			Providers: []assistantModelProviderConfig{
				{
					Name:      "openai",
					Enabled:   true,
					Model:     "gpt-5-codex",
					Endpoint:  server.URL + "/v1",
					TimeoutMS: 1000,
					Retries:   0,
					Priority:  1,
					KeyRef:    "OPENAI_API_KEY",
				},
			},
		},
		adapters: map[string]assistantProviderAdapter{
			"openai": assistantOpenAIProviderAdapter{httpClient: server.Client()},
		},
	}
	_, statuses := gw.listProviderStatus()
	if len(statuses) != 1 {
		t.Fatalf("unexpected statuses=%+v", statuses)
	}
	if statuses[0].Healthy != "healthy" || statuses[0].HealthReason != "" {
		t.Fatalf("expected healthy by probe, got=%+v", statuses[0])
	}
	if modelsProbeCalls != 1 {
		t.Fatalf("expected exactly one probe call, got=%d", modelsProbeCalls)
	}

	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	_, statuses = gw.listProviderStatus()
	if len(statuses) != 1 {
		t.Fatalf("unexpected statuses=%+v", statuses)
	}
	if statuses[0].Healthy != "unavailable" || statuses[0].HealthReason != "probe_failed" {
		t.Fatalf("expected unavailable/probe_failed, got=%+v", statuses[0])
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

func TestAssistantOpenAIProviderAdapter_InvokeAndParseContentPayloadObject(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method=%s", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":[{"type":"output_text","content":"{\"result\":{\"intent\":{\"action\":\"create_orgunit\",\"route_kind\":\"business_action\",\"intent_id\":\"org.orgunit_create\",\"parent_ref_text\":\"AI治理办公室\",\"entity_name\":\"人力资源部239A补全\",\"effective_date\":\"2026-03-25\"}}}"}]}}]}`))
	}))
	defer server.Close()
	t.Setenv("OPENAI_API_KEY", "test-key")
	adapter := assistantOpenAIProviderAdapter{
		httpClient: server.Client(),
		fallback:   assistantDeterministicProviderAdapter{},
	}
	payload, err := adapter.Invoke(context.Background(), "在 AI治理办公室 下新建 人力资源部239A补全，生效日期 2026-03-25", assistantModelProviderConfig{
		Name:      "openai",
		Model:     "gpt-5-codex",
		Endpoint:  server.URL + "/v1",
		TimeoutMS: 1000,
		KeyRef:    "OPENAI_API_KEY",
	})
	if err != nil {
		t.Fatalf("invoke err=%v", err)
	}
	semantic, err := assistantStrictDecodeSemanticIntent(payload)
	if err != nil {
		t.Fatalf("strict decode err=%v payload=%s", err, string(payload))
	}
	if semantic.Action != assistantIntentCreateOrgUnit || semantic.RouteKind != assistantRouteKindBusinessAction || semantic.IntentID != "org.orgunit_create" || semantic.ParentRefText != "AI治理办公室" || semantic.EntityName != "人力资源部239A补全" || semantic.EffectiveDate != "2026-03-25" {
		t.Fatalf("unexpected semantic=%+v payload=%s", semantic, string(payload))
	}
}

func TestAssistantModelGateway_DefaultConfigFollowsRuntime(t *testing.T) {
	t.Setenv("ASSISTANT_RUNTIME_ENV", "production")
	prod := defaultAssistantModelConfig()
	if prod.Providers[0].Endpoint != "https://api.openai.com/v1" || prod.Providers[0].Model != "gpt-5-codex" {
		t.Fatalf("unexpected production defaults: %+v", prod.Providers[0])
	}

	t.Setenv("ASSISTANT_RUNTIME_ENV", "dev")
	dev := defaultAssistantModelConfig()
	if dev.Providers[0].Endpoint != "https://api.openai.com/v1" || dev.Providers[0].Model != "gpt-5-codex" {
		t.Fatalf("unexpected dev defaults: %+v", dev.Providers[0])
	}
}

func TestAssistantOpenAIProviderAdapter_ErrorBranches(t *testing.T) {
	originalMarshal := assistantOpenAIRequestMarshalFn
	originalNewRequest := assistantOpenAINewRequestWithContextFn
	defer func() {
		assistantOpenAIRequestMarshalFn = originalMarshal
		assistantOpenAINewRequestWithContextFn = originalNewRequest
	}()
	assistantOpenAIRequestMarshalFn = json.Marshal
	assistantOpenAINewRequestWithContextFn = http.NewRequestWithContext

	t.Run("fallback missing", func(t *testing.T) {
		adapter := assistantOpenAIProviderAdapter{}
		if _, err := adapter.Invoke(context.Background(), "x", assistantModelProviderConfig{
			Name: "openai", Model: "gpt-5-codex", Endpoint: "builtin://openai", TimeoutMS: 50, KeyRef: "OPENAI_API_KEY",
		}); !errors.Is(err, errAssistantModelConfigInvalid) {
			t.Fatalf("unexpected err=%v", err)
		}
	})

	t.Run("invalid endpoint", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "test-key")
		adapter := assistantOpenAIProviderAdapter{fallback: assistantDeterministicProviderAdapter{}}
		if _, err := adapter.Invoke(context.Background(), "x", assistantModelProviderConfig{
			Name: "openai", Model: "gpt-5-codex", Endpoint: "http://api.openai.com/v1", TimeoutMS: 50, KeyRef: "OPENAI_API_KEY",
		}); !errors.Is(err, errAssistantModelConfigInvalid) {
			t.Fatalf("unexpected err=%v", err)
		}
	})

	t.Run("openai key missing", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "")
		adapter := assistantOpenAIProviderAdapter{fallback: assistantDeterministicProviderAdapter{}}
		if _, err := adapter.Invoke(context.Background(), "x", assistantModelProviderConfig{
			Name: "openai", Model: "gpt-5-codex", Endpoint: "https://api.openai.com/v1", TimeoutMS: 50, KeyRef: "OPENAI_API_KEY",
		}); !errors.Is(err, errAssistantModelSecretMissing) {
			t.Fatalf("unexpected err=%v", err)
		}
	})

	t.Run("marshal/new-request errors", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "test-key")
		adapter := assistantOpenAIProviderAdapter{fallback: assistantDeterministicProviderAdapter{}}
		assistantOpenAIRequestMarshalFn = func(any) ([]byte, error) { return nil, errors.New("marshal failed") }
		if _, err := adapter.Invoke(context.Background(), "x", assistantModelProviderConfig{
			Name: "openai", Model: "gpt-5-codex", Endpoint: "https://api.openai.com/v1", TimeoutMS: 50, KeyRef: "OPENAI_API_KEY",
		}); !errors.Is(err, errAssistantModelConfigInvalid) {
			t.Fatalf("unexpected err=%v", err)
		}
		assistantOpenAIRequestMarshalFn = json.Marshal
		assistantOpenAINewRequestWithContextFn = func(context.Context, string, string, io.Reader) (*http.Request, error) {
			return nil, errors.New("new request failed")
		}
		if _, err := adapter.Invoke(context.Background(), "x", assistantModelProviderConfig{
			Name: "openai", Model: "gpt-5-codex", Endpoint: "https://api.openai.com/v1", TimeoutMS: 50, KeyRef: "OPENAI_API_KEY",
		}); !errors.Is(err, errAssistantModelConfigInvalid) {
			t.Fatalf("unexpected err=%v", err)
		}
		assistantOpenAINewRequestWithContextFn = http.NewRequestWithContext
	})

	t.Run("network and status mappings", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "test-key")
		baseAdapter := assistantOpenAIProviderAdapter{fallback: assistantDeterministicProviderAdapter{}}
		if _, err := baseAdapter.Invoke(context.Background(), "x", assistantModelProviderConfig{
			Name: "openai", Model: "gpt-5-codex", Endpoint: "https://127.0.0.1:1/v1", TimeoutMS: 20, KeyRef: "OPENAI_API_KEY",
		}); !errors.Is(err, errAssistantModelProviderUnavailable) {
			t.Fatalf("unexpected err=%v", err)
		}
		if _, err := baseAdapter.Invoke(nil, "x", assistantModelProviderConfig{
			Name: "openai", Model: "gpt-5-codex", Endpoint: "https://127.0.0.1:1/v1", TimeoutMS: 20, KeyRef: "OPENAI_API_KEY",
		}); !errors.Is(err, errAssistantModelProviderUnavailable) {
			t.Fatalf("unexpected nil-ctx err=%v", err)
		}

		timeoutAdapter := assistantOpenAIProviderAdapter{
			httpClient: &http.Client{
				Transport: assistantRoundTripperFunc(func(*http.Request) (*http.Response, error) {
					return nil, context.DeadlineExceeded
				}),
			},
			fallback: assistantDeterministicProviderAdapter{},
		}
		if _, err := timeoutAdapter.Invoke(context.Background(), "x", assistantModelProviderConfig{
			Name: "openai", Model: "gpt-5-codex", Endpoint: "https://api.openai.com/v1", TimeoutMS: 20, KeyRef: "OPENAI_API_KEY",
		}); !errors.Is(err, errAssistantModelTimeout) {
			t.Fatalf("unexpected timeout err=%v", err)
		}

		readErrAdapter := assistantOpenAIProviderAdapter{
			httpClient: &http.Client{
				Transport: assistantRoundTripperFunc(func(*http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       assistantErrReadCloser{},
						Header:     make(http.Header),
					}, nil
				}),
			},
			fallback: assistantDeterministicProviderAdapter{},
		}
		if _, err := readErrAdapter.Invoke(context.Background(), "x", assistantModelProviderConfig{
			Name: "openai", Model: "gpt-5-codex", Endpoint: "https://api.openai.com/v1", TimeoutMS: 20, KeyRef: "OPENAI_API_KEY",
		}); !errors.Is(err, errAssistantModelProviderUnavailable) {
			t.Fatalf("unexpected read err=%v", err)
		}

		makeServer := func(status int, body string) *httptest.Server {
			return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/chat/completions" {
					t.Fatalf("unexpected path=%s", r.URL.Path)
				}
				w.WriteHeader(status)
				if body != "" {
					_, _ = w.Write([]byte(body))
				}
			}))
		}

		cases := []struct {
			name    string
			status  int
			body    string
			wantErr error
		}{
			{name: "rate limited", status: http.StatusTooManyRequests, wantErr: errAssistantModelRateLimited},
			{name: "timeout status", status: http.StatusRequestTimeout, wantErr: errAssistantModelTimeout},
			{name: "provider unavailable", status: http.StatusInternalServerError, wantErr: errAssistantModelProviderUnavailable},
			{name: "config invalid", status: http.StatusBadRequest, wantErr: errAssistantModelConfigInvalid},
			{name: "bad json", status: http.StatusOK, body: "{", wantErr: errAssistantPlanSchemaConstrainedDecodeFailed},
			{name: "empty choices", status: http.StatusOK, body: `{"choices":[]}`, wantErr: errAssistantPlanSchemaConstrainedDecodeFailed},
			{name: "empty content", status: http.StatusOK, body: `{"choices":[{"message":{"content":[]}}]}`, wantErr: errAssistantPlanSchemaConstrainedDecodeFailed},
		}
		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				server := makeServer(tc.status, tc.body)
				defer server.Close()
				adapter := assistantOpenAIProviderAdapter{
					httpClient: server.Client(),
					fallback:   assistantDeterministicProviderAdapter{},
				}
				_, err := adapter.Invoke(context.Background(), "x", assistantModelProviderConfig{
					Name: "openai", Model: "gpt-5-codex", Endpoint: server.URL + "/v1", TimeoutMS: 1000, KeyRef: "OPENAI_API_KEY",
				})
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err=%v want=%v", err, tc.wantErr)
				}
			})
		}
	})

	t.Run("response_format unsupported fallback", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "test-key")
		calls := 0
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			raw, _ := io.ReadAll(r.Body)
			var payload map[string]any
			if err := json.Unmarshal(raw, &payload); err != nil {
				t.Fatalf("decode payload failed: %v", err)
			}
			_, hasResponseFormat := payload["response_format"]
			if calls == 1 {
				if !hasResponseFormat {
					t.Fatalf("first request should include response_format, payload=%s", string(raw))
				}
				responseFormat, ok := payload["response_format"].(map[string]any)
				if !ok {
					t.Fatalf("response_format type mismatch payload=%s", string(raw))
				}
				jsonSchema, ok := responseFormat["json_schema"].(map[string]any)
				if !ok {
					t.Fatalf("json_schema missing payload=%s", string(raw))
				}
				schema, ok := jsonSchema["schema"].(map[string]any)
				if !ok {
					t.Fatalf("schema missing payload=%s", string(raw))
				}
				properties, ok := schema["properties"].(map[string]any)
				if !ok || properties["route_kind"] == nil || properties["intent_id"] == nil {
					t.Fatalf("route contract missing in response_format payload=%s", string(raw))
				}
				required, ok := schema["required"].([]any)
				if !ok || len(required) < 3 {
					t.Fatalf("required fields missing payload=%s", string(raw))
				}
				requiredSet := map[string]struct{}{}
				for _, item := range required {
					text, _ := item.(string)
					requiredSet[text] = struct{}{}
				}
				if _, ok := requiredSet["route_kind"]; !ok {
					t.Fatalf("route_kind not required payload=%s", string(raw))
				}
				if _, ok := requiredSet["intent_id"]; !ok {
					t.Fatalf("intent_id not required payload=%s", string(raw))
				}
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":{"message":"invalid response_format"}}`))
				return
			}
			if hasResponseFormat {
				t.Fatalf("fallback request should not include response_format, payload=%s", string(raw))
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"action\":\"create_department\",\"route_kind\":\"business_action\",\"intent_id\":\"org.orgunit_create\",\"route_catalog_version\":\"semantic.v1\",\"parent_department\":\"鲜花组织\",\"department_name\":\"测试部\",\"established_date\":\"2026-01-01\"}"}}]}`))
		}))
		defer server.Close()
		adapter := assistantOpenAIProviderAdapter{
			httpClient: server.Client(),
			fallback:   assistantDeterministicProviderAdapter{},
		}
		payload, err := adapter.Invoke(context.Background(), "x", assistantModelProviderConfig{
			Name: "openai", Model: "gpt-5-codex", Endpoint: server.URL + "/v1", TimeoutMS: 1000, KeyRef: "OPENAI_API_KEY",
		})
		if err != nil {
			t.Fatalf("fallback invoke err=%v", err)
		}
		semantic, decodeErr := assistantStrictDecodeSemanticIntent(payload)
		if decodeErr != nil {
			t.Fatalf("strict decode fallback payload failed: %v payload=%s", decodeErr, string(payload))
		}
		intent := semantic.intentSpec()
		if intent.Action != assistantIntentCreateOrgUnit || intent.RouteKind != assistantRouteKindBusinessAction || intent.IntentID != "org.orgunit_create" || intent.RouteCatalogVersion != "semantic.v1" || intent.ParentRefText != "鲜花组织" || intent.EntityName != "测试部" || intent.EffectiveDate != "2026-01-01" {
			t.Fatalf("unexpected fallback intent=%+v semantic=%+v payload=%s", intent, semantic, string(payload))
		}
		if calls != 2 {
			t.Fatalf("expected 2 calls, got=%d", calls)
		}
	})

	t.Run("config invalid without response_format hint no fallback", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "test-key")
		calls := 0
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls++
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"invalid request"}}`))
		}))
		defer server.Close()
		adapter := assistantOpenAIProviderAdapter{
			httpClient: server.Client(),
			fallback:   assistantDeterministicProviderAdapter{},
		}
		_, err := adapter.Invoke(context.Background(), "x", assistantModelProviderConfig{
			Name: "openai", Model: "gpt-5-codex", Endpoint: server.URL + "/v1", TimeoutMS: 1000, KeyRef: "OPENAI_API_KEY",
		})
		if !errors.Is(err, errAssistantModelConfigInvalid) {
			t.Fatalf("unexpected err=%v", err)
		}
		if calls != 1 {
			t.Fatalf("expected 1 call, got=%d", calls)
		}
	})
}

func TestAssistantModelGateway_ResolveIntentRetryAndGuardBranches(t *testing.T) {
	t.Setenv("ASSISTANT_RUNTIME_ENV", "production")
	t.Setenv("OPENAI_API_KEY", "test-key")

	gw := &assistantModelGateway{
		config: assistantModelConfig{
			ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
			Providers: []assistantModelProviderConfig{
				{Name: "openai", Enabled: true, Model: "gpt-5-codex", Endpoint: "https://api.openai.com/v1", TimeoutMS: 100, Retries: 1, Priority: 1, KeyRef: "CUSTOM_OPENAI_KEY_REF"},
			},
		},
	}
	t.Setenv("CUSTOM_OPENAI_KEY_REF", "configured")
	attempts := 0
	gw.adapters = map[string]assistantProviderAdapter{
		"openai": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
			attempts++
			if attempts == 1 {
				return nil, errAssistantModelTimeout
			}
			return []byte(`{"action":"plan_only"}`), nil
		}),
	}
	resolved, err := gw.ResolveIntent(context.Background(), assistantResolveIntentRequest{Prompt: "x"})
	if err != nil || resolved.Intent.Action != "plan_only" || attempts != 2 {
		t.Fatalf("resolved=%+v err=%v attempts=%d", resolved, err, attempts)
	}

	gw.config.Providers[0].Retries = -1
	gw.adapters["openai"] = assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
		return nil, errAssistantModelTimeout
	})
	if _, err := gw.ResolveIntent(context.Background(), assistantResolveIntentRequest{Prompt: "x"}); !errors.Is(err, errAssistantModelTimeout) {
		t.Fatalf("unexpected err=%v", err)
	}

	gw.config.Providers[0].Endpoint = "builtin://openai"
	if _, err := gw.ResolveIntent(context.Background(), assistantResolveIntentRequest{Prompt: "x"}); !errors.Is(err, errAssistantModelConfigInvalid) {
		t.Fatalf("unexpected err=%v", err)
	}

	gw.config.Providers[0].Endpoint = "https://api.openai.com/v1"
	t.Setenv("OPENAI_API_KEY", "")
	if _, err := gw.ResolveIntent(context.Background(), assistantResolveIntentRequest{Prompt: "x"}); !errors.Is(err, errAssistantModelSecretMissing) {
		t.Fatalf("unexpected err=%v", err)
	}
}

func TestAssistantModelGateway_HelperFunctions_ExtraBranches(t *testing.T) {
	if _, err := assistantBuildOpenAIChatCompletionURL("://bad"); err == nil {
		t.Fatal("expected parse error")
	}
	if _, err := assistantBuildOpenAIChatCompletionURL("http://api.openai.com/v1"); err == nil {
		t.Fatal("expected non-https error")
	}
	urlWithSuffix, err := assistantBuildOpenAIChatCompletionURL("https://api.openai.com/v1/chat/completions")
	if err != nil || !strings.HasSuffix(urlWithSuffix, "/chat/completions") {
		t.Fatalf("unexpected url=%s err=%v", urlWithSuffix, err)
	}

	if got := assistantExtractOpenAIMessageContent(map[string]any{"text": "x"}); got != "" {
		t.Fatalf("unexpected content=%q", got)
	}
	if got := assistantExtractOpenAIMessageContent(`{"action":"plan_only"}`); got != `{"action":"plan_only"}` {
		t.Fatalf("unexpected string content=%q", got)
	}
	if got := assistantExtractOpenAIMessageContent([]any{"invalid", map[string]any{"type": "text"}}); got != "" {
		t.Fatalf("unexpected content=%q", got)
	}
	if got := assistantExtractOpenAIMessageContent([]any{
		map[string]any{"content": []any{" 片段A ", "片段B "}},
		map[string]any{"route_kind": "business_action", "action": "create_orgunit"},
	}); !strings.Contains(got, "片段A") || !strings.Contains(got, `"route_kind":"business_action"`) {
		t.Fatalf("unexpected nested-array content=%q", got)
	}
	if !assistantOpenAIResponseFormatUnsupported([]byte(`{"error":{"message":"invalid response_format"}}`)) {
		t.Fatal("expected response_format unsupported detection")
	}
	if assistantOpenAIResponseFormatUnsupported([]byte(`{"error":{"message":"invalid request"}}`)) {
		t.Fatal("expected non response_format error not detected")
	}
	normalizedPayload := assistantNormalizeOpenAIIntentPayload(`{"action":"create_department","parent_department":"鲜花组织","department_name":"测试部","established_date":"2026-01-01"}`)
	intent, err := assistantStrictDecodeIntent(normalizedPayload)
	if err != nil {
		t.Fatalf("normalize payload decode err=%v payload=%s", err, string(normalizedPayload))
	}
	if intent.Action != assistantIntentCreateOrgUnit || intent.ParentRefText != "鲜花组织" || intent.EntityName != "测试部" || intent.EffectiveDate != "2026-01-01" {
		t.Fatalf("unexpected normalized intent=%+v", intent)
	}
	camelPayload := assistantNormalizeOpenAIIntentPayload(`{"action":"create_organization_unit","parentOrg":"AI治理办公室","newOrgName":"人力资源部2","effectiveDate":"2026-01-01"}`)
	camelIntent, err := assistantStrictDecodeIntent(camelPayload)
	if err != nil {
		t.Fatalf("camel payload decode err=%v payload=%s", err, string(camelPayload))
	}
	if camelIntent.Action != assistantIntentCreateOrgUnit || camelIntent.ParentRefText != "AI治理办公室" || camelIntent.EntityName != "人力资源部2" || camelIntent.EffectiveDate != "2026-01-01" {
		t.Fatalf("unexpected camel intent=%+v payload=%s", camelIntent, string(camelPayload))
	}
	nestedPayload := assistantNormalizeOpenAIIntentPayload(`{"action":"CREATE_ORG_UNIT","parentOrg":{"name":"AI治理办公室"},"newOrg":{"name":"人力资源部2","effectiveDate":"2026-01-01"}}`)
	nestedIntent, err := assistantStrictDecodeIntent(nestedPayload)
	if err != nil {
		t.Fatalf("nested payload decode err=%v payload=%s", err, string(nestedPayload))
	}
	if nestedIntent.Action != assistantIntentCreateOrgUnit || nestedIntent.ParentRefText != "AI治理办公室" || nestedIntent.EntityName != "人力资源部2" || nestedIntent.EffectiveDate != "2026-01-01" {
		t.Fatalf("unexpected nested intent=%+v payload=%s", nestedIntent, string(nestedPayload))
	}
	parentAliasPayload := assistantNormalizeOpenAIIntentPayload(`{"action":"create_org_unit","parentOrgName":"AI治理办公室","newOrgName":"人力资源部2","effectiveDate":"2026-01-01"}`)
	parentAliasIntent, err := assistantStrictDecodeIntent(parentAliasPayload)
	if err != nil {
		t.Fatalf("parent alias payload decode err=%v payload=%s", err, string(parentAliasPayload))
	}
	if parentAliasIntent.Action != assistantIntentCreateOrgUnit || parentAliasIntent.ParentRefText != "AI治理办公室" || parentAliasIntent.EntityName != "人力资源部2" || parentAliasIntent.EffectiveDate != "2026-01-01" {
		t.Fatalf("unexpected parent alias intent=%+v payload=%s", parentAliasIntent, string(parentAliasPayload))
	}
	orgAliasPayload := assistantNormalizeOpenAIIntentPayload(`{"action":"create_organization","parentOrganizationName":"AI治理办公室","newDepartmentName":"人力资源部2","effectiveDate":"2026-01-01"}`)
	orgAliasIntent, err := assistantStrictDecodeIntent(orgAliasPayload)
	if err != nil {
		t.Fatalf("org alias payload decode err=%v payload=%s", err, string(orgAliasPayload))
	}
	if orgAliasIntent.Action != assistantIntentCreateOrgUnit || orgAliasIntent.ParentRefText != "AI治理办公室" || orgAliasIntent.EntityName != "人力资源部2" || orgAliasIntent.EffectiveDate != "2026-01-01" {
		t.Fatalf("unexpected org alias intent=%+v payload=%s", orgAliasIntent, string(orgAliasPayload))
	}
	newOrganizationPayload := assistantNormalizeOpenAIIntentPayload(`{"action":"create_organization","parentOrganization":"共享服务中心","newOrganization":{"name":"239A候选验证部","effectiveDate":"2026-03-26"}}`)
	newOrganizationIntent, err := assistantStrictDecodeIntent(newOrganizationPayload)
	if err != nil {
		t.Fatalf("new organization payload decode err=%v payload=%s", err, string(newOrganizationPayload))
	}
	if newOrganizationIntent.Action != assistantIntentCreateOrgUnit || newOrganizationIntent.ParentRefText != "共享服务中心" || newOrganizationIntent.EntityName != "239A候选验证部" || newOrganizationIntent.EffectiveDate != "2026-03-26" {
		t.Fatalf("unexpected new organization intent=%+v payload=%s", newOrganizationIntent, string(newOrganizationPayload))
	}
	changeArrayPayload := assistantNormalizeOpenAIIntentPayload(`{"changes":[{"organizationUnit":{"name":"人力资源部2","parent":"AI治理办公室"},"effectiveDate":"2026-01-01","type":"CREATE"}]}`)
	changeArrayIntent, err := assistantStrictDecodeIntent(changeArrayPayload)
	if err != nil {
		t.Fatalf("change array payload decode err=%v payload=%s", err, string(changeArrayPayload))
	}
	if changeArrayIntent.Action != assistantIntentCreateOrgUnit || changeArrayIntent.ParentRefText != "AI治理办公室" || changeArrayIntent.EntityName != "人力资源部2" || changeArrayIntent.EffectiveDate != "2026-01-01" {
		t.Fatalf("unexpected change array intent=%+v payload=%s", changeArrayIntent, string(changeArrayPayload))
	}
	operationsPayload := assistantNormalizeOpenAIIntentPayload(`{"operations":[{"operationType":"CREATE_DEPARTMENT","parentOrgName":"共享服务中心","org_unit":{"name":"239A候选验证部","effective_date":"2026-03-26"}}]}`)
	operationsIntent, err := assistantStrictDecodeIntent(operationsPayload)
	if err != nil {
		t.Fatalf("operations payload decode err=%v payload=%s", err, string(operationsPayload))
	}
	if operationsIntent.Action != assistantIntentCreateOrgUnit || operationsIntent.ParentRefText != "共享服务中心" || operationsIntent.EntityName != "239A候选验证部" || operationsIntent.EffectiveDate != "2026-03-26" {
		t.Fatalf("unexpected operations intent=%+v payload=%s", operationsIntent, string(operationsPayload))
	}
	markdownPayload := assistantNormalizeOpenAIIntentPayload("```json\n{\"action\":\"plan_only\"}\n```")
	if string(markdownPayload) != `{"action":"plan_only"}` {
		t.Fatalf("unexpected markdown normalized payload=%s", string(markdownPayload))
	}

	movePayload := assistantNormalizeOpenAIIntentPayload(`{"action":"move","orgCode":"FLOWER-C","effectiveDate":"2026-04-01","newParentName":"共享服务中心"}`)
	moveIntent, err := assistantStrictDecodeIntent(movePayload)
	if err != nil {
		t.Fatalf("move payload decode err=%v payload=%s", err, string(movePayload))
	}
	if moveIntent.Action != assistantIntentMoveOrgUnit || moveIntent.OrgCode != "FLOWER-C" || moveIntent.EffectiveDate != "2026-04-01" || moveIntent.NewParentRefText != "共享服务中心" {
		t.Fatalf("unexpected move intent=%+v payload=%s", moveIntent, string(movePayload))
	}

	t.Setenv("ASSISTANT_RUNTIME_ENV", "production")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("CUSTOM_OPENAI_KEY_REF", "configured")
	gw := &assistantModelGateway{
		config: assistantModelConfig{
			ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
			Providers: []assistantModelProviderConfig{
				{Name: "openai", Enabled: true, Model: "gpt-5-codex", Endpoint: "https://api.openai.com/v1", TimeoutMS: 1000, Retries: 0, Priority: 1, KeyRef: "CUSTOM_OPENAI_KEY_REF"},
				{Name: "deepseek", Enabled: true, Model: "deepseek-chat", Endpoint: "builtin://deepseek", TimeoutMS: 1000, Retries: 0, Priority: 2, KeyRef: "DEEPSEEK_API_KEY"},
			},
		},
		adapters: map[string]assistantProviderAdapter{
			"openai":   assistantDeterministicProviderAdapter{},
			"deepseek": assistantDeterministicProviderAdapter{},
		},
	}
	_, statuses := gw.listProviderStatus()
	got := map[string]string{}
	for _, status := range statuses {
		got[status.Name] = status.HealthReason
	}
	if got["openai"] != "openai_key_missing" {
		t.Fatalf("unexpected openai status=%+v", statuses)
	}
	if got["deepseek"] != "endpoint_invalid" {
		t.Fatalf("unexpected deepseek status=%+v", statuses)
	}

	t.Setenv("OPENAI_API_KEY", "")
	_, errs := normalizeAssistantModelConfig(assistantModelConfig{
		ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
		Providers: []assistantModelProviderConfig{
			{Name: "openai", Enabled: true, Model: "gpt-5-codex", Endpoint: "https://api.openai.com/v1", TimeoutMS: 1000, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"},
		},
	}, true)
	joined := strings.Join(errs, "|")
	if !strings.Contains(joined, "OPENAI_API_KEY missing") {
		t.Fatalf("expected openai key validation error, got=%v", errs)
	}

}

type assistantProbeOnlyAdapter struct {
	err error
}

func (a assistantProbeOnlyAdapter) Invoke(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
	return []byte(`{"action":"plan_only"}`), nil
}

func (a assistantProbeOnlyAdapter) Probe(context.Context, assistantModelProviderConfig) error {
	return a.err
}

func TestAssistantDeterministicProviderAdapter_InvokeProbeMissingBranches(t *testing.T) {
	adapter := assistantDeterministicProviderAdapter{}

	if _, err := adapter.Invoke(context.Background(), "x", assistantModelProviderConfig{Endpoint: "simulate://timeout"}); !errors.Is(err, errAssistantModelTimeout) {
		t.Fatalf("unexpected timeout err=%v", err)
	}
	if payload, err := adapter.Invoke(context.Background(), "在鲜花组织之下新建运营部", assistantModelProviderConfig{Endpoint: "simulate://ok"}); err != nil || len(payload) == 0 {
		t.Fatalf("unexpected payload=%s err=%v", string(payload), err)
	}

	if err := adapter.Probe(context.Background(), assistantModelProviderConfig{Endpoint: ""}); !errors.Is(err, errAssistantModelConfigInvalid) {
		t.Fatalf("expected config invalid, got=%v", err)
	}
	if err := adapter.Probe(context.Background(), assistantModelProviderConfig{Endpoint: "simulate://timeout"}); !errors.Is(err, errAssistantModelTimeout) {
		t.Fatalf("expected timeout, got=%v", err)
	}
	if err := adapter.Probe(context.Background(), assistantModelProviderConfig{Endpoint: "simulate://rate-limit"}); !errors.Is(err, errAssistantModelRateLimited) {
		t.Fatalf("expected rate limited, got=%v", err)
	}
	if err := adapter.Probe(context.Background(), assistantModelProviderConfig{Endpoint: "simulate://unavailable"}); !errors.Is(err, errAssistantModelProviderUnavailable) {
		t.Fatalf("expected unavailable, got=%v", err)
	}
	if err := adapter.Probe(context.Background(), assistantModelProviderConfig{Endpoint: "https://api.openai.com/v1"}); err != nil {
		t.Fatalf("expected nil, got=%v", err)
	}
}

func TestAssistantOpenAIProviderAdapter_InvokeSecondPassError(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	calls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"invalid response_format"}}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"server failed"}}`))
	}))
	defer server.Close()

	adapter := assistantOpenAIProviderAdapter{
		httpClient: server.Client(),
		fallback:   assistantDeterministicProviderAdapter{},
	}
	_, err := adapter.Invoke(context.Background(), "x", assistantModelProviderConfig{
		Name:      "openai",
		Model:     "gpt-5-codex",
		Endpoint:  server.URL + "/v1",
		TimeoutMS: 500,
		KeyRef:    "OPENAI_API_KEY",
	})
	if !errors.Is(err, errAssistantModelProviderUnavailable) {
		t.Fatalf("unexpected err=%v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got=%d", calls)
	}
}

func TestAssistantOpenAIProviderAdapter_ProbeBranches(t *testing.T) {
	originalNewRequest := assistantOpenAINewRequestWithContextFn
	defer func() { assistantOpenAINewRequestWithContextFn = originalNewRequest }()

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("CUSTOM_OPENAI_KEY_REF", "present")

	adapter := assistantOpenAIProviderAdapter{}
	if err := adapter.Probe(context.Background(), assistantModelProviderConfig{
		Name:      "openai",
		Endpoint:  "http://api.openai.com/v1",
		TimeoutMS: 500,
		KeyRef:    "CUSTOM_OPENAI_KEY_REF",
	}); !errors.Is(err, errAssistantModelConfigInvalid) {
		t.Fatalf("unexpected invalid endpoint err=%v", err)
	}
	if err := adapter.Probe(context.Background(), assistantModelProviderConfig{
		Name:      "openai",
		Endpoint:  "https://api.openai.com/v1",
		TimeoutMS: 500,
		KeyRef:    "MISSING_KEY",
	}); !errors.Is(err, errAssistantModelSecretMissing) {
		t.Fatalf("unexpected missing key err=%v", err)
	}

	assistantOpenAINewRequestWithContextFn = func(context.Context, string, string, io.Reader) (*http.Request, error) {
		return nil, errors.New("new request failed")
	}
	if err := adapter.Probe(context.Background(), assistantModelProviderConfig{
		Name:      "openai",
		Endpoint:  "https://api.openai.com/v1",
		TimeoutMS: 500,
		KeyRef:    "CUSTOM_OPENAI_KEY_REF",
	}); !errors.Is(err, errAssistantModelConfigInvalid) {
		t.Fatalf("unexpected new request err=%v", err)
	}
	assistantOpenAINewRequestWithContextFn = originalNewRequest

	timeoutAdapter := assistantOpenAIProviderAdapter{
		httpClient: &http.Client{Transport: assistantRoundTripperFunc(func(*http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		})},
	}
	if err := timeoutAdapter.Probe(context.Background(), assistantModelProviderConfig{
		Name:      "openai",
		Endpoint:  "https://api.openai.com/v1",
		TimeoutMS: 500,
		KeyRef:    "CUSTOM_OPENAI_KEY_REF",
	}); !errors.Is(err, errAssistantModelTimeout) {
		t.Fatalf("unexpected timeout err=%v", err)
	}

	unavailableAdapter := assistantOpenAIProviderAdapter{
		httpClient: &http.Client{Transport: assistantRoundTripperFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("dial failed")
		})},
	}
	if err := unavailableAdapter.Probe(context.Background(), assistantModelProviderConfig{
		Name:      "openai",
		Endpoint:  "https://api.openai.com/v1",
		TimeoutMS: 500,
		KeyRef:    "CUSTOM_OPENAI_KEY_REF",
	}); !errors.Is(err, errAssistantModelProviderUnavailable) {
		t.Fatalf("unexpected unavailable err=%v", err)
	}

	statusServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer statusServer.Close()
	if err := (assistantOpenAIProviderAdapter{httpClient: statusServer.Client()}).Probe(nil, assistantModelProviderConfig{
		Name:      "openai",
		Endpoint:  statusServer.URL + "/v1",
		TimeoutMS: 500,
		KeyRef:    "CUSTOM_OPENAI_KEY_REF",
	}); err != nil {
		t.Fatalf("expected nil for success probe, got=%v", err)
	}

	makeProbeServer := func(status int) *httptest.Server {
		return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
		}))
	}
	cases := []struct {
		status int
		want   error
	}{
		{status: http.StatusTooManyRequests, want: errAssistantModelRateLimited},
		{status: http.StatusGatewayTimeout, want: errAssistantModelTimeout},
		{status: http.StatusForbidden, want: errAssistantModelSecretMissing},
		{status: http.StatusInternalServerError, want: errAssistantModelProviderUnavailable},
		{status: http.StatusTeapot, want: errAssistantModelConfigInvalid},
	}
	for _, tc := range cases {
		server := makeProbeServer(tc.status)
		err := (assistantOpenAIProviderAdapter{httpClient: server.Client()}).Probe(context.Background(), assistantModelProviderConfig{
			Name:      "openai",
			Endpoint:  server.URL + "/v1",
			TimeoutMS: 500,
			KeyRef:    "CUSTOM_OPENAI_KEY_REF",
		})
		server.Close()
		if !errors.Is(err, tc.want) {
			t.Fatalf("status=%d err=%v want=%v", tc.status, err, tc.want)
		}
	}

	if err := (assistantOpenAIProviderAdapter{}).Probe(context.Background(), assistantModelProviderConfig{
		Name:      "openai",
		Endpoint:  "https://127.0.0.1:1/v1",
		TimeoutMS: 500,
		KeyRef:    "CUSTOM_OPENAI_KEY_REF",
	}); !errors.Is(err, errAssistantModelProviderUnavailable) {
		t.Fatalf("unexpected nil-client probe err=%v", err)
	}
}

func TestAssistantModelGateway_HelperCoverage(t *testing.T) {
	if got := assistantNormalizeOpenAIIntentPayload("   "); string(got) != "" {
		t.Fatalf("expected empty payload, got=%q", string(got))
	}
	if got := assistantNormalizeOpenAIIntentPayload("plain text"); string(got) != "plain text" {
		t.Fatalf("expected passthrough payload, got=%q", string(got))
	}
	inferred := assistantNormalizeOpenAIIntentPayload(`{"parent_department":"鲜花组织","department_name":"运营部"}`)
	intent, err := assistantStrictDecodeIntent(inferred)
	if err != nil || intent.Action != assistantIntentCreateOrgUnit {
		t.Fatalf("unexpected inferred payload=%s err=%v", string(inferred), err)
	}
	if got := assistantNormalizeOpenAIIntentPayload(`{"foo":"bar"}`); string(got) != `{"foo":"bar"}` {
		t.Fatalf("expected passthrough object payload, got=%q", string(got))
	}
	enveloped := assistantNormalizeOpenAIIntentPayload(`{"response":{"payload":{"intent":{"action":"create_orgunit","route_kind":"business_action","intent_id":"org.orgunit_create","parentOrgName":"共享服务中心","newDepartmentName":"候选验证部239A","effectiveDate":"2026-03-26"}}}}`)
	envelopedPayload, err := assistantStrictDecodeSemanticIntent(enveloped)
	if err != nil {
		t.Fatalf("unexpected enveloped payload=%s err=%v", string(enveloped), err)
	}
	if envelopedPayload.Action != assistantIntentCreateOrgUnit || envelopedPayload.RouteKind != assistantRouteKindBusinessAction || envelopedPayload.IntentID != "org.orgunit_create" || envelopedPayload.ParentRefText != "共享服务中心" || envelopedPayload.EntityName != "候选验证部239A" || envelopedPayload.EffectiveDate != "2026-03-26" {
		t.Fatalf("unexpected enveloped decoded payload=%+v raw=%s", envelopedPayload, string(enveloped))
	}
	normalizedSemantic := assistantNormalizeOpenAIIntentPayload(`{"action":"plan_only","route_kind":"knowledge_qa","intent_id":"knowledge.general_qa","route_catalog_version":"semantic.v1"}`)
	semanticPayload, err := assistantStrictDecodeSemanticIntent(normalizedSemantic)
	if err != nil || semanticPayload.RouteKind != assistantRouteKindKnowledgeQA || semanticPayload.IntentID != "knowledge.general_qa" || semanticPayload.RouteCatalogVersion != "semantic.v1" {
		t.Fatalf("unexpected normalized semantic payload=%s decoded=%+v err=%v", string(normalizedSemantic), semanticPayload, err)
	}

	if _, ok := assistantDecodeOpenAIIntentPayloadObject(`prefix {"action":"plan_only"} suffix`); !ok {
		t.Fatal("expected extracted object decode success")
	}
	if _, ok := assistantDecodeOpenAIIntentPayloadObject(`prefix {"action":} suffix`); ok {
		t.Fatal("expected decode failure for invalid extracted JSON")
	}
	if extracted, ok := assistantExtractJSONObject(`xx {"a":"x\\\"y"} yy`); !ok || extracted == "" {
		t.Fatalf("expected extracted object, got=%q ok=%v", extracted, ok)
	}
	if _, ok := assistantExtractJSONObject(`no brace here`); ok {
		t.Fatal("expected no object when no opening brace")
	}
	if _, ok := assistantExtractJSONObject(`{"a":"unterminated"`); ok {
		t.Fatal("expected no object when braces are incomplete")
	}

	if got := assistantFirstString(map[string]any{"x": 1, "y": "  "}, "x", "y", "z"); got != "" {
		t.Fatalf("expected empty first string, got=%q", got)
	}
	if got := assistantNormalizeOpenAIIntentAction("   "); got != "" {
		t.Fatalf("expected empty action, got=%q", got)
	}
	if got := assistantNormalizeOpenAIIntentAction("custom_action"); got != "custom_action" {
		t.Fatalf("expected passthrough action, got=%q", got)
	}
}

func TestAssistantModelGateway_NewGatewayProbeHealthAndURLCoverage(t *testing.T) {
	t.Setenv("ASSISTANT_MODEL_CONFIG_JSON", "")
	if _, err := newAssistantModelGateway(); !errors.Is(err, errAssistantRuntimeConfigMissing) {
		t.Fatalf("expected runtime missing, got=%v", err)
	}

	t.Setenv("ASSISTANT_MODEL_CONFIG_JSON", `{"provider_routing":{"strategy":"priority_failover","fallback_enabled":true},"providers":[{"name":"openai","enabled":true,"model":"gpt-5-codex","endpoint":"builtin://openai","timeout_ms":500,"retries":0,"priority":1,"key_ref":"OPENAI_API_KEY"}]}`)
	if _, err := newAssistantModelGateway(); !errors.Is(err, errAssistantRuntimeConfigInvalid) {
		t.Fatalf("expected runtime invalid by normalization, got=%v", err)
	}

	t.Setenv("ASSISTANT_MODEL_CONFIG_JSON", `{"provider_routing":{"strategy":"priority_failover","fallback_enabled":false},"providers":[{"name":"openai","enabled":true,"model":"gpt-5-codex","endpoint":"https://api.openai.com/v1","timeout_ms":500,"retries":0,"priority":1,"key_ref":"OPENAI_API_KEY"}]}`)
	t.Setenv("OPENAI_API_KEY", "")
	if gateway, err := newAssistantModelGateway(); err != nil || gateway == nil {
		t.Fatalf("expected gateway init without secret check, gateway=%v err=%v", gateway, err)
	}

	gateway := &assistantModelGateway{
		adapters: map[string]assistantProviderAdapter{
			"openai": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
				return []byte(`{"action":"plan_only"}`), nil
			}),
		},
	}
	if err := gateway.probeProviderStatus(assistantModelProviderConfig{Name: "missing", TimeoutMS: 500}); !errors.Is(err, errAssistantModelProbeUnsupported) {
		t.Fatalf("expected adapter missing unsupported err, got=%v", err)
	}
	if err := gateway.probeProviderStatus(assistantModelProviderConfig{Name: "openai", TimeoutMS: 500}); !errors.Is(err, errAssistantModelProbeUnsupported) {
		t.Fatalf("expected non-prober unsupported err, got=%v", err)
	}
	gateway.adapters["openai"] = assistantProbeOnlyAdapter{err: errAssistantModelRateLimited}
	if err := gateway.probeProviderStatus(assistantModelProviderConfig{Name: "openai", TimeoutMS: 500}); !errors.Is(err, errAssistantModelRateLimited) {
		t.Fatalf("expected probe error passthrough, got=%v", err)
	}

	if healthy, reason := assistantProviderHealthFromProbeErr(nil); healthy != "healthy" || reason != "" {
		t.Fatalf("unexpected healthy mapping=%s/%s", healthy, reason)
	}
	if healthy, reason := assistantProviderHealthFromProbeErr(errAssistantModelRateLimited); healthy != "degraded" || reason != "rate_limited" {
		t.Fatalf("unexpected rate limited mapping=%s/%s", healthy, reason)
	}
	if healthy, reason := assistantProviderHealthFromProbeErr(errAssistantModelTimeout); healthy != "degraded" || reason != "probe_timeout" {
		t.Fatalf("unexpected timeout mapping=%s/%s", healthy, reason)
	}
	if healthy, reason := assistantProviderHealthFromProbeErr(errAssistantModelSecretMissing); healthy != "unavailable" || reason != "secret_missing" {
		t.Fatalf("unexpected secret mapping=%s/%s", healthy, reason)
	}
	if healthy, reason := assistantProviderHealthFromProbeErr(errAssistantModelConfigInvalid); healthy != "unavailable" || reason != "endpoint_invalid" {
		t.Fatalf("unexpected endpoint mapping=%s/%s", healthy, reason)
	}
	if healthy, reason := assistantProviderHealthFromProbeErr(errAssistantModelProbeUnsupported); healthy != "unavailable" || reason != "probe_unsupported" {
		t.Fatalf("unexpected unsupported mapping=%s/%s", healthy, reason)
	}

	if got := assistantProbeTimeoutMS(0); got != 1500 {
		t.Fatalf("expected 1500, got=%d", got)
	}
	if got := assistantProbeTimeoutMS(10); got != 500 {
		t.Fatalf("expected floor 500, got=%d", got)
	}
	if got := assistantProbeTimeoutMS(9000); got != 3000 {
		t.Fatalf("expected cap 3000, got=%d", got)
	}

	if _, err := assistantBuildOpenAIModelsURL("://bad"); err == nil {
		t.Fatal("expected parse error")
	}
	if _, err := assistantBuildOpenAIModelsURL("http://api.openai.com/v1"); err == nil {
		t.Fatal("expected non-https error")
	}
	if url, err := assistantBuildOpenAIModelsURL("https://api.openai.com"); err != nil || url != "https://api.openai.com/models" {
		t.Fatalf("unexpected models url=%s err=%v", url, err)
	}
	if url, err := assistantBuildOpenAIModelsURL("https://api.openai.com/v1/chat/completions"); err != nil || url != "https://api.openai.com/v1/models" {
		t.Fatalf("unexpected completion-suffix models url=%s err=%v", url, err)
	}
	if url, err := assistantBuildOpenAIModelsURL("https://api.openai.com/"); err != nil || url != "https://api.openai.com/models" {
		t.Fatalf("unexpected root models url=%s err=%v", url, err)
	}
}

func TestAssistantModelGateway_HelperCoverage_Additional(t *testing.T) {
	if got := assistantFirstString(map[string]any{"name": " 运营部 "}, "name"); got != "运营部" {
		t.Fatalf("got=%q", got)
	}
	if got := assistantFirstStringFromObjects([]map[string]any{nil, {"intent": map[string]any{"entity_name": "共享服务中心"}}}, "intent.entity_name"); got != "共享服务中心" {
		t.Fatalf("got=%q", got)
	}
	if got := assistantFirstCompositeNameFromObjects([]map[string]any{{"target": map[string]any{"name": "TP290BAIGOV-AI治理办公室", "code": "TP290BAIGOV"}}}, "target"); got != "TP290BAIGOV-AI治理办公室" {
		t.Fatalf("got=%q", got)
	}
	if got := assistantFirstCompositeNameFromObjects([]map[string]any{{"target": map[string]any{"name": "AI治理办公室", "code": "TP290BAIGOV"}}}, "target"); got != "TP290BAIGOVAI治理办公室" {
		t.Fatalf("got=%q", got)
	}
	if got := assistantFirstCompositeNameFromObjects([]map[string]any{{"target": map[string]any{"name": "AI治理办公室"}}}, "target"); got != "AI治理办公室" {
		t.Fatalf("got=%q", got)
	}
	if got := assistantFirstCompositeNameFromObjects([]map[string]any{{"target": map[string]any{"code": "TP290BAIGOV"}}}, "target"); got != "TP290BAIGOV" {
		t.Fatalf("got=%q", got)
	}
	if got := assistantFirstCompositeNameFromObjects([]map[string]any{{"target": "not-map"}}, "target"); got != "" {
		t.Fatalf("got=%q", got)
	}
	objects := assistantIntentCandidateObjects(map[string]any{"actions": []any{map[string]any{"name": "x"}}})
	if len(objects) != 2 {
		t.Fatalf("objects=%v", objects)
	}
	envelopedObjects := assistantIntentCandidateObjects(map[string]any{"result": map[string]any{"intent": map[string]any{"entity_name": "共享服务中心"}}})
	if got := assistantFirstStringFromObjects(envelopedObjects, "entity_name"); got != "共享服务中心" {
		t.Fatalf("got=%q objects=%+v", got, envelopedObjects)
	}
	deepObjects := assistantIntentCandidateObjects(map[string]any{
		"results": []any{
			map[string]any{
				"payload": map[string]any{
					"intent": map[string]any{"entity_name": "候选验证部239A"},
				},
			},
		},
	})
	if got := assistantFirstStringFromObjects(deepObjects, "entity_name"); got != "候选验证部239A" {
		t.Fatalf("got=%q objects=%+v", got, deepObjects)
	}
	if _, ok := assistantFirstMapFromSlice(map[string]any{"items": []any{}}, "items"); ok {
		t.Fatal("expected empty slice miss")
	}
	if _, ok := assistantFirstMapFromSlice(map[string]any{"items": []any{"x"}}, "items"); ok {
		t.Fatal("expected non-map miss")
	}
	if got := assistantLookupLooseAny(map[string]any{"org_unit_name": "AI治理办公室"}, "orgUnitName", "missing"); got != "AI治理办公室" {
		t.Fatalf("got=%v", got)
	}
	if got := assistantLookupLooseAny(map[string]any{"x": 1}, "missing"); got != nil {
		t.Fatalf("got=%v", got)
	}
	if value, ok := assistantLookupPathValue(map[string]any{"a": map[string]any{"b": "c"}}, "a.b"); !ok || value != "c" {
		t.Fatalf("value=%v ok=%v", value, ok)
	}
	if _, ok := assistantLookupPathValue(map[string]any{"a": 1}, "a.b"); ok {
		t.Fatal("expected non-map path miss")
	}
	if _, ok := assistantLookupPathValue(map[string]any{"a": map[string]any{"b": "c"}}, "a..b"); ok {
		t.Fatal("expected empty segment miss")
	}
	if _, ok := assistantToNonEmptyString("   "); ok {
		t.Fatal("expected blank string miss")
	}
	if got := assistantNormalizeOpenAIIntentAction("Create Org"); got != assistantIntentCreateOrgUnit {
		t.Fatalf("got=%q", got)
	}
	if !assistantOpenAIContentObjectLooksLikePayload(map[string]any{"route_kind": "business_action"}) {
		t.Fatal("expected payload-like object")
	}
	if assistantOpenAIContentObjectLooksLikePayload(map[string]any{"foo": "bar"}) {
		t.Fatal("unexpected payload-like detection")
	}
	if userInput, pendingAction := assistantSemanticPromptFallbackContext("  原始文本  "); userInput != "原始文本" || pendingAction != "" {
		t.Fatalf("unexpected raw prompt fallback context=%q/%q", userInput, pendingAction)
	}
	if userInput, pendingAction := assistantSemanticPromptFallbackContext(`{"current_user_input":"","pending_turn":{"action":"create_orgunit"}}`); userInput == "" || pendingAction != assistantIntentCreateOrgUnit {
		t.Fatalf("unexpected empty-user-input fallback context=%q/%q", userInput, pendingAction)
	}
	if assistantPromptEligibleForCreateOrgUnitSemanticFallback(`{"current_user_input":"确认","pending_turn":{"action":"create_orgunit"}}`) {
		t.Fatal("plain confirm should not trigger create prompt fallback")
	}
	if assistantPromptEligibleForCreateOrgUnitSemanticFallback(`{"current_user_input":"系统有哪些功能"}`) {
		t.Fatal("knowledge qa should not trigger create prompt fallback")
	}
	if assistantPromptEligibleForCreateOrgUnitSemanticFallback("") {
		t.Fatal("empty prompt should not trigger create prompt fallback")
	}
	if resolved, ok := assistantResolveIntentFallbackFromPrompt(assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
		return nil, nil
	}), assistantModelProviderConfig{Name: "openai", Model: "m", Endpoint: "https://api.openai.com/v1"}, assistantBuildSemanticPrompt("在 AI治理办公室 下新建 人力资源部2，生效日期 2026-01-01", nil)); ok || resolved.Intent.Action != "" {
		t.Fatalf("non-openai adapter should not allow fallback, resolved=%+v ok=%v", resolved, ok)
	}
	if resolved, ok := assistantResolveIntentFallbackFromPrompt(&assistantOpenAIProviderAdapter{}, assistantModelProviderConfig{Name: "openai", Model: "m", Endpoint: "https://api.openai.com/v1"}, assistantBuildSemanticPrompt("确认", nil)); ok || resolved.Intent.Action != "" {
		t.Fatalf("prompt without create evidence should not allow fallback, resolved=%+v ok=%v", resolved, ok)
	}
	if resolved, ok := assistantResolveIntentFallbackFromPrompt(&assistantOpenAIProviderAdapter{}, assistantModelProviderConfig{Name: "openai", Model: "m", Endpoint: "https://api.openai.com/v1"}, assistantBuildSemanticPrompt("系统有哪些功能", nil)); ok || resolved.Intent.Action != "" {
		t.Fatalf("non-create prompt should not allow fallback, resolved=%+v ok=%v", resolved, ok)
	}
}

func TestAssistantModelGateway_ResolveIntentRetriesTransientInvoke(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "dummy")
	attempts := 0
	gw := &assistantModelGateway{
		config: assistantModelConfig{ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true}, Providers: []assistantModelProviderConfig{{Name: "openai", Enabled: true, Model: "m", Endpoint: "https://api.openai.com/v1", TimeoutMS: 1000, Retries: 1, Priority: 1, KeyRef: "OPENAI_API_KEY"}}},
		adapters: map[string]assistantProviderAdapter{"openai": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
			attempts++
			if attempts == 1 {
				return nil, errAssistantModelTimeout
			}
			return []byte(`{"action":"plan_only"}`), nil
		})},
	}
	resolved, err := gw.ResolveIntent(context.Background(), assistantResolveIntentRequest{Prompt: "x"})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if attempts != 2 || resolved.Intent.Action != assistantIntentPlanOnly {
		t.Fatalf("resolved=%+v attempts=%d", resolved, attempts)
	}
}

func TestAssistantModelGateway_ResolveIntentRetriesStrictDecodeFailure(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "dummy")
	attempts := 0
	gw := &assistantModelGateway{
		config: assistantModelConfig{ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true}, Providers: []assistantModelProviderConfig{{Name: "openai", Enabled: true, Model: "m", Endpoint: "https://api.openai.com/v1", TimeoutMS: 1000, Retries: 1, Priority: 1, KeyRef: "OPENAI_API_KEY"}}},
		adapters: map[string]assistantProviderAdapter{"openai": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
			attempts++
			if attempts == 1 {
				return []byte(`{"choices":[{"message":{"content":"not-json"}}]}`), nil
			}
			return []byte(`{"action":"plan_only"}`), nil
		})},
	}
	resolved, err := gw.ResolveIntent(context.Background(), assistantResolveIntentRequest{Prompt: "x"})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if attempts != 2 || resolved.Intent.Action != assistantIntentPlanOnly {
		t.Fatalf("resolved=%+v attempts=%d", resolved, attempts)
	}
}

func TestAssistantModelGateway_ResolveIntentOpenAIPromptFallbackOnStrictDecodeFailure(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "dummy")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"not-json"}}]}`))
	}))
	defer server.Close()

	gw := &assistantModelGateway{
		config: assistantModelConfig{
			ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
			Providers: []assistantModelProviderConfig{{
				Name:      "openai",
				Enabled:   true,
				Model:     "gpt-5-codex",
				Endpoint:  server.URL + "/v1",
				TimeoutMS: 1000,
				Retries:   0,
				Priority:  1,
				KeyRef:    "OPENAI_API_KEY",
			}},
		},
		adapters: map[string]assistantProviderAdapter{
			"openai": assistantOpenAIProviderAdapter{
				httpClient: server.Client(),
				fallback:   assistantDeterministicProviderAdapter{},
			},
		},
	}

	resolved, err := gw.ResolveIntent(context.Background(), assistantResolveIntentRequest{
		Prompt: assistantBuildSemanticPrompt("在 AI治理办公室 下新建 人力资源部2，生效日期 2026-01-01", nil),
	})
	if err != nil {
		t.Fatalf("resolve intent err=%v", err)
	}
	if resolved.Intent.Action != assistantIntentCreateOrgUnit || resolved.Intent.RouteKind != assistantRouteKindBusinessAction || resolved.Intent.IntentID != "org.orgunit_create" {
		t.Fatalf("unexpected fallback intent=%+v", resolved.Intent)
	}
	if resolved.ProviderName != "openai" || resolved.ModelName != "gpt-5-codex" {
		t.Fatalf("unexpected provider metadata=%+v", resolved)
	}
}

func TestAssistantModelGateway_ResolveIntentOpenAIPromptFallbackOnMissingRouteContract(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "dummy")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"action\":\"create_orgunit\",\"parent_ref_text\":\"AI治理办公室\",\"effective_date\":\"2026-01-01\"}"}}]}`))
	}))
	defer server.Close()

	gw := &assistantModelGateway{
		config: assistantModelConfig{
			ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
			Providers: []assistantModelProviderConfig{{
				Name:      "openai",
				Enabled:   true,
				Model:     "gpt-5-codex",
				Endpoint:  server.URL + "/v1",
				TimeoutMS: 1000,
				Retries:   0,
				Priority:  1,
				KeyRef:    "OPENAI_API_KEY",
			}},
		},
		adapters: map[string]assistantProviderAdapter{
			"openai": assistantOpenAIProviderAdapter{
				httpClient: server.Client(),
				fallback:   assistantDeterministicProviderAdapter{},
			},
		},
	}

	resolved, err := gw.ResolveIntent(context.Background(), assistantResolveIntentRequest{
		Prompt: assistantBuildSemanticPrompt("在 AI治理办公室 下新建 人力资源部2，生效日期 2026-01-01", nil),
	})
	if err != nil {
		t.Fatalf("resolve intent err=%v", err)
	}
	if resolved.Intent.Action != assistantIntentCreateOrgUnit || resolved.Intent.RouteKind != assistantRouteKindBusinessAction || resolved.Intent.IntentID != "org.orgunit_create" {
		t.Fatalf("unexpected fallback intent=%+v", resolved.Intent)
	}
}

func TestAssistantModelGateway_ResolveIntentOpenAIPromptFallbackOnInvokeDecodeError(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "dummy")
	call := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call++
		if call == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"invalid response_format"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{`))
	}))
	defer server.Close()

	gw := &assistantModelGateway{
		config: assistantModelConfig{
			ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
			Providers: []assistantModelProviderConfig{{
				Name:      "openai",
				Enabled:   true,
				Model:     "gpt-5-codex",
				Endpoint:  server.URL + "/v1",
				TimeoutMS: 1000,
				Retries:   0,
				Priority:  1,
				KeyRef:    "OPENAI_API_KEY",
			}},
		},
		adapters: map[string]assistantProviderAdapter{
			"openai": assistantOpenAIProviderAdapter{
				httpClient: server.Client(),
				fallback: assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
					return nil, errAssistantPlanSchemaConstrainedDecodeFailed
				}),
			},
		},
	}

	resolved, err := gw.ResolveIntent(context.Background(), assistantResolveIntentRequest{
		Prompt: assistantBuildSemanticPrompt("在 AI治理办公室 下新建 人力资源部2，生效日期 2026-01-01", nil),
	})
	if err != nil {
		t.Fatalf("resolve intent err=%v", err)
	}
	if call != 2 {
		t.Fatalf("expected openai adapter second pass before fallback, calls=%d", call)
	}
	if resolved.Intent.Action != assistantIntentCreateOrgUnit || resolved.Intent.IntentID != "org.orgunit_create" {
		t.Fatalf("unexpected fallback intent=%+v", resolved.Intent)
	}
}
