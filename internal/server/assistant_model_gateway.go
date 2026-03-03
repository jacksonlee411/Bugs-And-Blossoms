package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
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

type assistantDeterministicProviderAdapter struct{}

func (assistantDeterministicProviderAdapter) Invoke(_ context.Context, prompt string, provider assistantModelProviderConfig) ([]byte, error) {
	endpoint := strings.ToLower(strings.TrimSpace(provider.Endpoint))
	switch {
	case strings.HasPrefix(endpoint, "simulate://timeout"):
		return nil, errAssistantModelTimeout
	case strings.HasPrefix(endpoint, "simulate://rate-limit"):
		return nil, errAssistantModelRateLimited
	case strings.HasPrefix(endpoint, "simulate://unavailable"):
		return nil, errAssistantModelProviderUnavailable
	}
	intent := assistantExtractIntent(strings.TrimSpace(prompt))
	payload, err := json.Marshal(intent)
	if err != nil {
		return nil, errAssistantPlanSchemaConstrainedDecodeFailed
	}
	return payload, nil
}

type assistantModelGateway struct {
	mu       sync.RWMutex
	config   assistantModelConfig
	adapters map[string]assistantProviderAdapter
}

func newAssistantModelGateway() *assistantModelGateway {
	gateway := &assistantModelGateway{
		config: defaultAssistantModelConfig(),
		adapters: map[string]assistantProviderAdapter{
			"openai":   assistantDeterministicProviderAdapter{},
			"deepseek": assistantDeterministicProviderAdapter{},
			"claude":   assistantDeterministicProviderAdapter{},
			"gemini":   assistantDeterministicProviderAdapter{},
		},
	}
	if fromEnv := strings.TrimSpace(os.Getenv("ASSISTANT_MODEL_CONFIG_JSON")); fromEnv != "" {
		var parsed assistantModelConfig
		if err := json.Unmarshal([]byte(fromEnv), &parsed); err == nil {
			if normalized, errs := normalizeAssistantModelConfig(parsed, false); len(errs) == 0 {
				gateway.config = normalized
			}
		}
	}
	return gateway
}

func defaultAssistantModelConfig() assistantModelConfig {
	return assistantModelConfig{
		ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
		Providers: []assistantModelProviderConfig{
			{
				Name:      "openai",
				Enabled:   true,
				Model:     "gpt-4o-mini",
				Endpoint:  "builtin://openai",
				TimeoutMS: 8000,
				Retries:   1,
				Priority:  10,
				KeyRef:    "OPENAI_API_KEY",
			},
			{
				Name:      "deepseek",
				Enabled:   false,
				Model:     "deepseek-chat",
				Endpoint:  "builtin://deepseek",
				TimeoutMS: 8000,
				Retries:   1,
				Priority:  20,
				KeyRef:    "DEEPSEEK_API_KEY",
			},
			{
				Name:      "claude",
				Enabled:   false,
				Model:     "claude-3-5-sonnet-latest",
				Endpoint:  "builtin://claude",
				TimeoutMS: 8000,
				Retries:   1,
				Priority:  30,
				KeyRef:    "ANTHROPIC_API_KEY",
			},
			{
				Name:      "gemini",
				Enabled:   false,
				Model:     "gemini-2.0-flash",
				Endpoint:  "builtin://gemini",
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
		case strings.HasPrefix(strings.ToLower(strings.TrimSpace(provider.Endpoint)), "simulate://timeout"):
			status.Healthy = "degraded"
			status.HealthReason = "simulated_timeout"
		case strings.HasPrefix(strings.ToLower(strings.TrimSpace(provider.Endpoint)), "simulate://rate-limit"):
			status.Healthy = "degraded"
			status.HealthReason = "simulated_rate_limited"
		case !assistantIsBuiltInEndpoint(provider.Endpoint) && strings.TrimSpace(os.Getenv(strings.TrimSpace(provider.KeyRef))) == "":
			status.Healthy = "degraded"
			status.HealthReason = "secret_missing"
		default:
			status.Healthy = "healthy"
		}
		statuses = append(statuses, status)
	}
	return providers, statuses
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
		if !assistantIsBuiltInEndpoint(provider.Endpoint) && strings.TrimSpace(os.Getenv(strings.TrimSpace(provider.KeyRef))) == "" {
			return assistantResolveIntentResult{}, errAssistantModelSecretMissing
		}
		adapter := g.adapters[strings.ToLower(strings.TrimSpace(provider.Name))]
		if adapter == nil {
			return assistantResolveIntentResult{}, errAssistantModelConfigInvalid
		}
		raw, err := adapter.Invoke(ctx, req.Prompt, provider)
		if err != nil {
			switch {
			case errorsIsAny(err, errAssistantModelTimeout, errAssistantModelRateLimited, errAssistantModelProviderUnavailable):
				lastTransientErr = err
				continue
			default:
				return assistantResolveIntentResult{}, err
			}
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
	if enabledCount == 0 {
		return assistantResolveIntentResult{}, errAssistantModelProviderUnavailable
	}
	if lastTransientErr != nil {
		return assistantResolveIntentResult{}, lastTransientErr
	}
	return assistantResolveIntentResult{}, errAssistantModelProviderUnavailable
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
		if checkSecret && !assistantIsBuiltInEndpoint(provider.Endpoint) && provider.KeyRef != "" && strings.TrimSpace(os.Getenv(provider.KeyRef)) == "" {
			errs = append(errs, "providers."+strconv.Itoa(idx)+" secret missing for key_ref")
		}
	}
	return normalized, errs
}

func assistantIsBuiltInEndpoint(endpoint string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(endpoint)), "builtin://")
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
