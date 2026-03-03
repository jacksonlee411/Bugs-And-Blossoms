package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
