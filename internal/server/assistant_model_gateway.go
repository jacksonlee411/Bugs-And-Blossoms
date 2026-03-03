package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	assistantIntentSchemaVersionV1     = "assistant.intent.v1"
	assistantCompilerContractVersionV1 = "assistant.compiler.v1"
	assistantCapabilityMapVersionV1    = "2026-02-23"
)

var assistantProviderNameAllowlist = map[string]struct{}{
	"openai":   {},
	"deepseek": {},
	"claude":   {},
	"gemini":   {},
}

type assistantProviderRouting struct {
	Strategy        string `json:"strategy"`
	FallbackEnabled bool   `json:"fallback_enabled"`
}

type assistantModelProviderConfig struct {
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	Model     string `json:"model"`
	Endpoint  string `json:"endpoint"`
	TimeoutMS int    `json:"timeout_ms"`
	Retries   int    `json:"retries"`
	Priority  int    `json:"priority"`
	KeyRef    string `json:"key_ref"`
}

type assistantModelConfig struct {
	ProviderRouting assistantProviderRouting       `json:"provider_routing"`
	Providers       []assistantModelProviderConfig `json:"providers"`
}

type assistantResolveIntentRequest struct {
	Prompt         string
	ConversationID string
	TenantID       string
}

type assistantResolveIntentResult struct {
	Intent        assistantIntentSpec
	ProviderName  string
	ModelName     string
	ModelRevision string
}

type assistantProviderStatus struct {
	Name         string `json:"name"`
	Healthy      string `json:"healthy"`
	HealthReason string `json:"health_reason,omitempty"`
}

type assistantProviderAdapter interface {
	Invoke(ctx context.Context, prompt string, provider assistantModelProviderConfig) ([]byte, error)
}

type assistantProviderHealthProber interface {
	Probe(ctx context.Context, provider assistantModelProviderConfig) error
}

var assistantIntentMarshalFn = json.Marshal
var assistantOpenAIRequestMarshalFn = json.Marshal
var assistantOpenAINewRequestWithContextFn = http.NewRequestWithContext
var assistantOpenAIHTTPClientFactory = func() *http.Client { return nil }

var errAssistantModelProbeUnsupported = errors.New("assistant_model_probe_unsupported")

type assistantDeterministicProviderAdapter struct{}

func (assistantDeterministicProviderAdapter) Invoke(_ context.Context, prompt string, provider assistantModelProviderConfig) ([]byte, error) {
	endpoint := strings.ToLower(strings.TrimSpace(provider.Endpoint))
	switch {
	case assistantIsSimulateEndpoint(endpoint) && strings.HasPrefix(endpoint, "simulate://timeout"):
		return nil, errAssistantModelTimeout
	case assistantIsSimulateEndpoint(endpoint) && strings.HasPrefix(endpoint, "simulate://rate-limit"):
		return nil, errAssistantModelRateLimited
	case assistantIsSimulateEndpoint(endpoint) && strings.HasPrefix(endpoint, "simulate://unavailable"):
		return nil, errAssistantModelProviderUnavailable
	}
	intent := assistantExtractIntent(strings.TrimSpace(prompt))
	payload, err := assistantIntentMarshalFn(intent)
	if err != nil {
		return nil, errAssistantPlanSchemaConstrainedDecodeFailed
	}
	return payload, nil
}

func (assistantDeterministicProviderAdapter) Probe(_ context.Context, provider assistantModelProviderConfig) error {
	endpoint := strings.ToLower(strings.TrimSpace(provider.Endpoint))
	switch {
	case assistantIsSimulateEndpoint(endpoint) && strings.HasPrefix(endpoint, "simulate://timeout"):
		return errAssistantModelTimeout
	case assistantIsSimulateEndpoint(endpoint) && strings.HasPrefix(endpoint, "simulate://rate-limit"):
		return errAssistantModelRateLimited
	case assistantIsSimulateEndpoint(endpoint) && strings.HasPrefix(endpoint, "simulate://unavailable"):
		return errAssistantModelProviderUnavailable
	case assistantEndpointInvalidForRuntime(provider.Endpoint):
		return errAssistantModelConfigInvalid
	default:
		return nil
	}
}

type assistantOpenAIProviderAdapter struct {
	httpClient *http.Client
	fallback   assistantProviderAdapter
}

type assistantOpenAIChatCompletionRequest struct {
	Model          string                                     `json:"model"`
	Temperature    float64                                    `json:"temperature"`
	TopP           float64                                    `json:"top_p"`
	N              int                                        `json:"n"`
	Messages       []assistantOpenAIChatCompletionMessage     `json:"messages"`
	ResponseFormat *assistantOpenAIChatCompletionResponseSpec `json:"response_format,omitempty"`
}

type assistantOpenAIChatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type assistantOpenAIChatCompletionResponseSpec struct {
	Type       string                            `json:"type"`
	JSONSchema assistantOpenAIChatJSONSchemaSpec `json:"json_schema"`
}

type assistantOpenAIChatJSONSchemaSpec struct {
	Name   string `json:"name"`
	Strict bool   `json:"strict"`
	Schema any    `json:"schema"`
}

type assistantOpenAIChatCompletionResponse struct {
	Choices []assistantOpenAIChatCompletionChoice `json:"choices"`
}

type assistantOpenAIChatCompletionChoice struct {
	Message assistantOpenAIChatCompletionChoiceMessage `json:"message"`
}

type assistantOpenAIChatCompletionChoiceMessage struct {
	Content any `json:"content"`
}

type assistantOpenAIInvokeResult struct {
	RawBody    []byte
	StatusCode int
}

func (a assistantOpenAIProviderAdapter) Invoke(ctx context.Context, prompt string, provider assistantModelProviderConfig) ([]byte, error) {
	endpoint := strings.TrimSpace(provider.Endpoint)
	requestURL, err := assistantBuildOpenAIChatCompletionURL(endpoint)
	if err != nil {
		return nil, errAssistantModelConfigInvalid
	}
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		return nil, errAssistantModelSecretMissing
	}
	requestCtx := ctx
	if requestCtx == nil {
		requestCtx = context.Background()
	}
	client := a.httpClient
	if client == nil {
		client = &http.Client{}
	}
	buildPayload := func(enableSchemaFormat bool) assistantOpenAIChatCompletionRequest {
		payload := assistantOpenAIChatCompletionRequest{
			Model:       strings.TrimSpace(provider.Model),
			Temperature: 0,
			TopP:        1,
			N:           1,
			Messages: []assistantOpenAIChatCompletionMessage{
				{
					Role: "system",
					Content: "你是企业 HR 组织变更助手。你必须只输出严格 JSON，禁止输出解释、Markdown 或其他文本。" +
						"JSON 必须符合 schema 且 additionalProperties=false。",
				},
				{
					Role:    "user",
					Content: strings.TrimSpace(prompt),
				},
			},
		}
		if enableSchemaFormat {
			payload.ResponseFormat = &assistantOpenAIChatCompletionResponseSpec{
				Type: "json_schema",
				JSONSchema: assistantOpenAIChatJSONSchemaSpec{
					Name:   "assistant_intent_spec",
					Strict: true,
					Schema: map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"properties": map[string]any{
							"action": map[string]any{
								"type": "string",
							},
							"parent_ref_text": map[string]any{
								"type": "string",
							},
							"entity_name": map[string]any{
								"type": "string",
							},
							"effective_date": map[string]any{
								"type": "string",
							},
						},
						"required": []string{"action"},
					},
				},
			}
		}
		return payload
	}
	invokePayload := func(payload assistantOpenAIChatCompletionRequest) (assistantOpenAIInvokeResult, error) {
		body, err := assistantOpenAIRequestMarshalFn(payload)
		if err != nil {
			return assistantOpenAIInvokeResult{}, errAssistantModelConfigInvalid
		}
		timeoutCtx, cancel := context.WithTimeout(requestCtx, time.Duration(provider.TimeoutMS)*time.Millisecond)
		defer cancel()
		req, err := assistantOpenAINewRequestWithContextFn(timeoutCtx, http.MethodPost, requestURL, bytes.NewReader(body))
		if err != nil {
			return assistantOpenAIInvokeResult{}, errAssistantModelConfigInvalid
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		resp, err := client.Do(req)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
				return assistantOpenAIInvokeResult{}, errAssistantModelTimeout
			}
			return assistantOpenAIInvokeResult{}, errAssistantModelProviderUnavailable
		}
		defer resp.Body.Close()
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if readErr != nil {
			return assistantOpenAIInvokeResult{}, errAssistantModelProviderUnavailable
		}
		result := assistantOpenAIInvokeResult{
			RawBody:    raw,
			StatusCode: resp.StatusCode,
		}
		switch {
		case resp.StatusCode == http.StatusTooManyRequests:
			return result, errAssistantModelRateLimited
		case resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusGatewayTimeout:
			return result, errAssistantModelTimeout
		case resp.StatusCode >= 500:
			return result, errAssistantModelProviderUnavailable
		case resp.StatusCode >= 400:
			return result, errAssistantModelConfigInvalid
		}
		return result, nil
	}
	decodeContent := func(raw []byte) ([]byte, error) {
		var completion assistantOpenAIChatCompletionResponse
		if err := json.Unmarshal(raw, &completion); err != nil {
			return nil, errAssistantPlanSchemaConstrainedDecodeFailed
		}
		if len(completion.Choices) == 0 {
			return nil, errAssistantPlanSchemaConstrainedDecodeFailed
		}
		content := assistantExtractOpenAIMessageContent(completion.Choices[0].Message.Content)
		if strings.TrimSpace(content) == "" {
			return nil, errAssistantPlanSchemaConstrainedDecodeFailed
		}
		return assistantNormalizeOpenAIIntentPayload(content), nil
	}

	result, err := invokePayload(buildPayload(true))
	if err != nil {
		if !errors.Is(err, errAssistantModelConfigInvalid) || !assistantOpenAIResponseFormatUnsupported(result.RawBody) {
			return nil, err
		}
		result, err = invokePayload(buildPayload(false))
		if err != nil {
			return nil, err
		}
	}
	return decodeContent(result.RawBody)
}

func (a assistantOpenAIProviderAdapter) Probe(ctx context.Context, provider assistantModelProviderConfig) error {
	requestURL, err := assistantBuildOpenAIModelsURL(provider.Endpoint)
	if err != nil {
		return errAssistantModelConfigInvalid
	}
	apiKey := strings.TrimSpace(os.Getenv(strings.TrimSpace(provider.KeyRef)))
	if apiKey == "" {
		return errAssistantModelSecretMissing
	}
	requestCtx := ctx
	if requestCtx == nil {
		requestCtx = context.Background()
	}
	client := a.httpClient
	if client == nil {
		client = &http.Client{}
	}
	timeoutMS := assistantProbeTimeoutMS(provider.TimeoutMS)
	timeoutCtx, cancel := context.WithTimeout(requestCtx, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()
	req, err := assistantOpenAINewRequestWithContextFn(timeoutCtx, http.MethodGet, requestURL, nil)
	if err != nil {
		return errAssistantModelConfigInvalid
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
			return errAssistantModelTimeout
		}
		return errAssistantModelProviderUnavailable
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == http.StatusTooManyRequests:
		return errAssistantModelRateLimited
	case resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusGatewayTimeout:
		return errAssistantModelTimeout
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return errAssistantModelSecretMissing
	case resp.StatusCode >= 500:
		return errAssistantModelProviderUnavailable
	default:
		return errAssistantModelConfigInvalid
	}
}

func assistantOpenAIResponseFormatUnsupported(raw []byte) bool {
	if len(raw) == 0 {
		return false
	}
	return strings.Contains(strings.ToLower(string(raw)), "response_format")
}

func assistantNormalizeOpenAIIntentPayload(content string) []byte {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return []byte(trimmed)
	}
	obj, ok := assistantDecodeOpenAIIntentPayloadObject(trimmed)
	if !ok {
		return []byte(trimmed)
	}
	parentRefText := assistantFirstString(obj,
		"parent_ref_text",
		"parent_department",
		"parent_org",
		"parent_orgunit",
		"parent_unit",
		"parent")
	entityName := assistantFirstString(obj,
		"entity_name",
		"department_name",
		"org_name",
		"orgunit_name",
		"name")
	effectiveDate := assistantFirstString(obj,
		"effective_date",
		"established_date",
		"establishment_date",
		"start_date",
		"date")
	action := assistantNormalizeOpenAIIntentAction(assistantFirstString(obj, "action"))
	if action == "" && parentRefText != "" && entityName != "" {
		action = assistantIntentCreateOrgUnit
	}
	normalized := map[string]any{}
	if action != "" {
		normalized["action"] = action
	}
	if parentRefText != "" {
		normalized["parent_ref_text"] = parentRefText
	}
	if entityName != "" {
		normalized["entity_name"] = entityName
	}
	if effectiveDate != "" {
		normalized["effective_date"] = effectiveDate
	}
	if len(normalized) == 0 {
		return []byte(trimmed)
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		return []byte(trimmed)
	}
	return payload
}

func assistantDecodeOpenAIIntentPayloadObject(content string) (map[string]any, bool) {
	var object map[string]any
	if err := json.Unmarshal([]byte(content), &object); err == nil {
		return object, true
	}
	extracted, ok := assistantExtractJSONObject(content)
	if !ok {
		return nil, false
	}
	if err := json.Unmarshal([]byte(extracted), &object); err != nil {
		return nil, false
	}
	return object, true
}

func assistantExtractJSONObject(content string) (string, bool) {
	start := strings.IndexByte(content, '{')
	if start < 0 {
		return "", false
	}
	depth := 0
	inString := false
	escaped := false
	for index := start; index < len(content); index++ {
		ch := content[index]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return strings.TrimSpace(content[start : index+1]), true
			}
		}
	}
	return "", false
}

func assistantFirstString(object map[string]any, keys ...string) string {
	for _, key := range keys {
		value, exists := object[key]
		if !exists {
			continue
		}
		text, ok := value.(string)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(text)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func assistantNormalizeOpenAIIntentAction(action string) string {
	trimmed := strings.TrimSpace(action)
	if trimmed == "" {
		return ""
	}
	normalized := strings.ToLower(trimmed)
	normalized = strings.ReplaceAll(normalized, "-", "_")
	switch normalized {
	case assistantIntentCreateOrgUnit,
		"create_department",
		"createdepartment",
		"create_org_unit",
		"create_organization_unit",
		"createorganizationunit",
		"orgunit_create":
		return assistantIntentCreateOrgUnit
	default:
		return trimmed
	}
}

type assistantModelGateway struct {
	mu       sync.RWMutex
	config   assistantModelConfig
	adapters map[string]assistantProviderAdapter
}

func newAssistantModelGateway() (*assistantModelGateway, error) {
	openAIClient := assistantOpenAIHTTPClientFactory()
	gateway := &assistantModelGateway{
		adapters: map[string]assistantProviderAdapter{
			"openai": assistantOpenAIProviderAdapter{httpClient: openAIClient},
		},
	}
	fromEnv := strings.TrimSpace(os.Getenv("ASSISTANT_MODEL_CONFIG_JSON"))
	if fromEnv == "" {
		return nil, errAssistantRuntimeConfigMissing
	}
	var parsed assistantModelConfig
	if err := json.Unmarshal([]byte(fromEnv), &parsed); err != nil {
		return nil, errAssistantRuntimeConfigInvalid
	}
	normalized, errs := normalizeAssistantModelConfig(parsed, true)
	if len(errs) > 0 {
		return nil, errAssistantRuntimeConfigInvalid
	}
	gateway.config = normalized
	return gateway, nil
}

func defaultAssistantModelConfig() assistantModelConfig {
	return assistantModelConfig{
		ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
		Providers: []assistantModelProviderConfig{
			{
				Name:      "openai",
				Enabled:   true,
				Model:     "gpt-5-codex",
				Endpoint:  "https://api.openai.com/v1",
				TimeoutMS: 8000,
				Retries:   1,
				Priority:  10,
				KeyRef:    "OPENAI_API_KEY",
			},
			{
				Name:      "deepseek",
				Enabled:   false,
				Model:     "deepseek-chat",
				Endpoint:  "https://api.deepseek.com",
				TimeoutMS: 8000,
				Retries:   1,
				Priority:  20,
				KeyRef:    "DEEPSEEK_API_KEY",
			},
			{
				Name:      "claude",
				Enabled:   false,
				Model:     "claude-3-5-sonnet-latest",
				Endpoint:  "https://api.anthropic.com",
				TimeoutMS: 8000,
				Retries:   1,
				Priority:  30,
				KeyRef:    "ANTHROPIC_API_KEY",
			},
			{
				Name:      "gemini",
				Enabled:   false,
				Model:     "gemini-2.0-flash",
				Endpoint:  "https://generativelanguage.googleapis.com",
				TimeoutMS: 8000,
				Retries:   1,
				Priority:  40,
				KeyRef:    "GEMINI_API_KEY",
			},
		},
	}
}

func (g *assistantModelGateway) snapshot() assistantModelConfig {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return cloneAssistantModelConfig(g.config)
}

func (g *assistantModelGateway) listProviderStatus() ([]assistantModelProviderConfig, []assistantProviderStatus) {
	cfg := g.snapshot()
	providers := cloneAssistantProviderSlice(cfg.Providers)
	statuses := make([]assistantProviderStatus, 0, len(providers))
	for _, provider := range providers {
		status := assistantProviderStatus{Name: provider.Name}
		switch {
		case !provider.Enabled:
			status.Healthy = "disabled"
			status.HealthReason = "provider_disabled"
		case g.adapters[strings.ToLower(strings.TrimSpace(provider.Name))] == nil:
			status.Healthy = "unavailable"
			status.HealthReason = "provider_adapter_missing"
		case assistantEndpointInvalidForRuntime(provider.Endpoint):
			status.Healthy = "unavailable"
			status.HealthReason = "endpoint_invalid"
		case assistantProviderRequiresSecret(provider) && strings.TrimSpace(os.Getenv(strings.TrimSpace(provider.KeyRef))) == "":
			status.Healthy = "unavailable"
			status.HealthReason = "secret_missing"
		case assistantProviderRequiresOpenAIKey(provider) && strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) == "":
			status.Healthy = "unavailable"
			status.HealthReason = "openai_key_missing"
		default:
			status.Healthy = "unavailable"
			status.HealthReason = "probe_failed"
			if probeErr := g.probeProviderStatus(provider); probeErr == nil {
				status.Healthy = "healthy"
				status.HealthReason = ""
			} else {
				status.Healthy, status.HealthReason = assistantProviderHealthFromProbeErr(probeErr)
			}
		}
		statuses = append(statuses, status)
	}
	return providers, statuses
}

func (g *assistantModelGateway) probeProviderStatus(provider assistantModelProviderConfig) error {
	adapter := g.adapters[strings.ToLower(strings.TrimSpace(provider.Name))]
	if adapter == nil {
		return errAssistantModelProbeUnsupported
	}
	prober, ok := adapter.(assistantProviderHealthProber)
	if !ok {
		return errAssistantModelProbeUnsupported
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(assistantProbeTimeoutMS(provider.TimeoutMS))*time.Millisecond)
	defer cancel()
	return prober.Probe(ctx, provider)
}

func assistantProviderHealthFromProbeErr(err error) (string, string) {
	switch {
	case err == nil:
		return "healthy", ""
	case errors.Is(err, errAssistantModelRateLimited):
		return "degraded", "rate_limited"
	case errors.Is(err, errAssistantModelTimeout):
		return "degraded", "probe_timeout"
	case errors.Is(err, errAssistantModelSecretMissing):
		return "unavailable", "secret_missing"
	case errors.Is(err, errAssistantModelConfigInvalid):
		return "unavailable", "endpoint_invalid"
	case errors.Is(err, errAssistantModelProbeUnsupported):
		return "unavailable", "probe_unsupported"
	default:
		return "unavailable", "probe_failed"
	}
}

func (g *assistantModelGateway) validateConfig(config assistantModelConfig) (assistantModelConfig, []string) {
	return normalizeAssistantModelConfig(config, true)
}

func (g *assistantModelGateway) applyConfig(config assistantModelConfig) (assistantModelConfig, []string) {
	normalized, errs := normalizeAssistantModelConfig(config, true)
	if len(errs) > 0 {
		return assistantModelConfig{}, errs
	}
	g.mu.Lock()
	g.config = normalized
	g.mu.Unlock()
	return normalized, nil
}

func (g *assistantModelGateway) listModels() []assistantModelProviderConfig {
	cfg := g.snapshot()
	providers := cloneAssistantProviderSlice(cfg.Providers)
	models := make([]assistantModelProviderConfig, 0, len(providers))
	for _, provider := range providers {
		if !provider.Enabled {
			continue
		}
		models = append(models, provider)
	}
	return models
}

func (g *assistantModelGateway) ResolveIntent(ctx context.Context, req assistantResolveIntentRequest) (assistantResolveIntentResult, error) {
	cfg := g.snapshot()
	providers := cloneAssistantProviderSlice(cfg.Providers)
	sort.SliceStable(providers, func(i, j int) bool {
		if providers[i].Priority == providers[j].Priority {
			return providers[i].Name < providers[j].Name
		}
		return providers[i].Priority < providers[j].Priority
	})

	lastTransientErr := error(nil)
	enabledCount := 0
	for _, provider := range providers {
		if !provider.Enabled {
			continue
		}
		enabledCount++
		if _, ok := assistantProviderNameAllowlist[strings.ToLower(strings.TrimSpace(provider.Name))]; !ok {
			return assistantResolveIntentResult{}, errAssistantModelConfigInvalid
		}
		if strings.TrimSpace(provider.Model) == "" || strings.TrimSpace(provider.Endpoint) == "" || provider.TimeoutMS <= 0 {
			return assistantResolveIntentResult{}, errAssistantModelConfigInvalid
		}
		if strings.TrimSpace(provider.KeyRef) == "" {
			return assistantResolveIntentResult{}, errAssistantModelConfigInvalid
		}
		if assistantEndpointInvalidForRuntime(provider.Endpoint) {
			return assistantResolveIntentResult{}, errAssistantModelConfigInvalid
		}
		if assistantProviderRequiresSecret(provider) && strings.TrimSpace(os.Getenv(strings.TrimSpace(provider.KeyRef))) == "" {
			return assistantResolveIntentResult{}, errAssistantModelSecretMissing
		}
		if assistantProviderRequiresOpenAIKey(provider) && strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) == "" {
			return assistantResolveIntentResult{}, errAssistantModelSecretMissing
		}
		adapter := g.adapters[strings.ToLower(strings.TrimSpace(provider.Name))]
		if adapter == nil {
			return assistantResolveIntentResult{}, errAssistantModelConfigInvalid
		}
		invokeErr := errAssistantModelProviderUnavailable
		attempts := provider.Retries + 1
		if attempts < 1 {
			attempts = 1
		}
		for attempt := 0; attempt < attempts; attempt++ {
			raw, err := adapter.Invoke(ctx, req.Prompt, provider)
			if err != nil {
				invokeErr = err
				if errorsIsAny(err, errAssistantModelTimeout, errAssistantModelRateLimited, errAssistantModelProviderUnavailable) && attempt < attempts-1 {
					continue
				}
				break
			}
			intent, err := assistantStrictDecodeIntent(raw)
			if err != nil {
				return assistantResolveIntentResult{}, errAssistantPlanSchemaConstrainedDecodeFailed
			}
			return assistantResolveIntentResult{
				Intent:        intent,
				ProviderName:  strings.ToLower(strings.TrimSpace(provider.Name)),
				ModelName:     strings.TrimSpace(provider.Model),
				ModelRevision: assistantModelRevision(provider),
			}, nil
		}
		switch {
		case errorsIsAny(invokeErr, errAssistantModelTimeout, errAssistantModelRateLimited, errAssistantModelProviderUnavailable):
			lastTransientErr = invokeErr
			continue
		default:
			return assistantResolveIntentResult{}, invokeErr
		}
	}
	if enabledCount == 0 || lastTransientErr == nil {
		return assistantResolveIntentResult{}, errAssistantModelProviderUnavailable
	}
	return assistantResolveIntentResult{}, lastTransientErr
}

func assistantStrictDecodeIntent(raw []byte) (assistantIntentSpec, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var intent assistantIntentSpec
	if err := decoder.Decode(&intent); err != nil {
		return assistantIntentSpec{}, err
	}
	if err := decoder.Decode(&struct{}{}); err == nil {
		return assistantIntentSpec{}, errAssistantPlanSchemaConstrainedDecodeFailed
	}
	return intent, nil
}

func assistantModelRevision(provider assistantModelProviderConfig) string {
	seed := strings.TrimSpace(provider.Name) + "|" + strings.TrimSpace(provider.Model) + "|" + strings.TrimSpace(provider.Endpoint)
	h := sha256.Sum256([]byte(seed))
	return "r" + hex.EncodeToString(h[:6])
}

func normalizeAssistantModelConfig(config assistantModelConfig, checkSecret bool) (assistantModelConfig, []string) {
	normalized := cloneAssistantModelConfig(config)
	normalized.ProviderRouting.Strategy = strings.TrimSpace(strings.ToLower(normalized.ProviderRouting.Strategy))
	if normalized.ProviderRouting.Strategy == "" {
		normalized.ProviderRouting.Strategy = "priority_failover"
	}
	providers := cloneAssistantProviderSlice(normalized.Providers)
	sort.SliceStable(providers, func(i, j int) bool {
		if providers[i].Priority == providers[j].Priority {
			return providers[i].Name < providers[j].Name
		}
		return providers[i].Priority < providers[j].Priority
	})
	normalized.Providers = providers

	errs := make([]string, 0)
	if normalized.ProviderRouting.Strategy != "priority_failover" {
		errs = append(errs, "provider_routing.strategy must be priority_failover")
	}
	seenPriority := map[int]struct{}{}
	for idx := range normalized.Providers {
		provider := &normalized.Providers[idx]
		provider.Name = strings.TrimSpace(strings.ToLower(provider.Name))
		provider.Model = strings.TrimSpace(provider.Model)
		provider.Endpoint = strings.TrimSpace(provider.Endpoint)
		provider.KeyRef = strings.TrimSpace(provider.KeyRef)
		if _, ok := assistantProviderNameAllowlist[provider.Name]; !ok {
			errs = append(errs, "providers."+strconv.Itoa(idx)+".name is invalid")
		}
		if !provider.Enabled {
			continue
		}
		if provider.Model == "" || provider.Endpoint == "" || provider.KeyRef == "" {
			errs = append(errs, "providers."+strconv.Itoa(idx)+" missing required fields")
		}
		if provider.TimeoutMS <= 0 {
			errs = append(errs, "providers."+strconv.Itoa(idx)+" timeout_ms must be > 0")
		}
		if provider.Retries < 0 {
			errs = append(errs, "providers."+strconv.Itoa(idx)+" retries must be >= 0")
		}
		if _, exists := seenPriority[provider.Priority]; exists {
			errs = append(errs, "provider priority duplicated")
		}
		seenPriority[provider.Priority] = struct{}{}
		if assistantEndpointInvalidForRuntime(provider.Endpoint) {
			errs = append(errs, "providers."+strconv.Itoa(idx)+" endpoint invalid for runtime")
		}
		if checkSecret && assistantProviderRequiresSecret(*provider) && provider.KeyRef != "" && strings.TrimSpace(os.Getenv(provider.KeyRef)) == "" {
			errs = append(errs, "providers."+strconv.Itoa(idx)+" secret missing for key_ref")
		}
		if checkSecret && assistantProviderRequiresOpenAIKey(*provider) && strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) == "" {
			errs = append(errs, "providers."+strconv.Itoa(idx)+" OPENAI_API_KEY missing")
		}
	}
	return normalized, errs
}

func assistantIsSimulateEndpoint(endpoint string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(endpoint)), "simulate://")
}

func assistantIsHTTPSAPIEndpoint(endpoint string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(endpoint)), "https://")
}

func assistantEndpointInvalidForRuntime(endpoint string) bool {
	normalized := strings.TrimSpace(strings.ToLower(endpoint))
	if normalized == "" {
		return true
	}
	return !assistantIsHTTPSAPIEndpoint(normalized)
}

func assistantProviderRequiresSecret(provider assistantModelProviderConfig) bool {
	return assistantIsHTTPSAPIEndpoint(provider.Endpoint)
}

func assistantProviderRequiresOpenAIKey(provider assistantModelProviderConfig) bool {
	return strings.TrimSpace(strings.ToLower(provider.Name)) == "openai" && assistantIsHTTPSAPIEndpoint(provider.Endpoint)
}

func assistantProbeTimeoutMS(providerTimeoutMS int) int {
	timeoutMS := providerTimeoutMS
	if timeoutMS <= 0 {
		timeoutMS = 1500
	}
	if timeoutMS < 500 {
		timeoutMS = 500
	}
	if timeoutMS > 3000 {
		timeoutMS = 3000
	}
	return timeoutMS
}

func assistantBuildOpenAIModelsURL(endpoint string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return "", err
	}
	if strings.ToLower(base.Scheme) != "https" {
		return "", fmt.Errorf("openai endpoint must use https")
	}
	base.RawQuery = ""
	base.Fragment = ""
	cleanPath := strings.TrimSpace(base.Path)
	if cleanPath == "" {
		base.Path = "/models"
		return base.String(), nil
	}
	trimmed := strings.TrimSuffix(cleanPath, "/")
	if strings.HasSuffix(trimmed, "/chat/completions") {
		trimmed = strings.TrimSuffix(trimmed, "/chat/completions")
	}
	if trimmed == "" || trimmed == "." {
		base.Path = "/models"
		return base.String(), nil
	}
	base.Path = "/" + path.Join(strings.TrimPrefix(trimmed, "/"), "models")
	return base.String(), nil
}

func assistantBuildOpenAIChatCompletionURL(endpoint string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(base.Scheme) != "https" || strings.TrimSpace(base.Host) == "" {
		return "", fmt.Errorf("invalid endpoint")
	}
	trimmedPath := strings.TrimSpace(base.Path)
	if strings.HasSuffix(trimmedPath, "/chat/completions") {
		return base.String(), nil
	}
	base.Path = path.Join(trimmedPath, "/chat/completions")
	base.RawQuery = ""
	base.Fragment = ""
	return base.String(), nil
}

func assistantExtractOpenAIMessageContent(raw any) string {
	switch value := raw.(type) {
	case string:
		return value
	case []any:
		parts := make([]string, 0, len(value))
		for _, entry := range value {
			object, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			switch text := object["text"].(type) {
			case string:
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "")
	default:
		return ""
	}
}

func cloneAssistantProviderSlice(in []assistantModelProviderConfig) []assistantModelProviderConfig {
	if len(in) == 0 {
		return nil
	}
	out := make([]assistantModelProviderConfig, len(in))
	copy(out, in)
	return out
}

func cloneAssistantModelConfig(in assistantModelConfig) assistantModelConfig {
	out := in
	out.Providers = cloneAssistantProviderSlice(in.Providers)
	return out
}

func errorsIsAny(err error, targets ...error) bool {
	for _, target := range targets {
		if target != nil && err == target {
			return true
		}
	}
	return false
}

func assistantCanonicalHash(v any) string {
	payload, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func assistantSkillManifestDigest(skills []string) string {
	if len(skills) == 0 {
		return ""
	}
	copied := append([]string(nil), skills...)
	sort.Strings(copied)
	return assistantCanonicalHash(copied)
}
