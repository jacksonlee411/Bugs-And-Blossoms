package cubebox

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"
)

type ModelHealthWriteInput struct {
	ProviderID   string
	ModelSlug    string
	Status       string
	LatencyMS    *int
	ErrorSummary string
}

type ModelHealthWriter interface {
	RecordModelHealthCheck(ctx context.Context, tenantID string, principalID string, input ModelHealthWriteInput) (ModelHealth, error)
}

type ModelVerificationService struct {
	configReader   RuntimeConfigReader
	healthWriter   ModelHealthWriter
	adapter        ProviderAdapter
	secretResolver SecretResolver
	now            func() time.Time
}

func NewModelVerificationService(
	configReader RuntimeConfigReader,
	healthWriter ModelHealthWriter,
	adapter ProviderAdapter,
	secretResolver SecretResolver,
) *ModelVerificationService {
	return &ModelVerificationService{
		configReader:   configReader,
		healthWriter:   healthWriter,
		adapter:        adapter,
		secretResolver: secretResolver,
		now:            func() time.Time { return time.Now().UTC() },
	}
}

func (s *ModelVerificationService) VerifyActiveModel(ctx context.Context, tenantID string, principalID string) (ModelHealth, error) {
	config, err := s.configReader.GetActiveModelRuntimeConfig(ctx, tenantID)
	if err != nil {
		return ModelHealth{}, err
	}
	if strings.TrimSpace(config.Selection.ModelSlug) == "" {
		return ModelHealth{}, ErrModelSlugMissing
	}
	if !config.Provider.Enabled {
		return s.recordFailure(ctx, tenantID, principalID, config.Provider.ID, config.Selection.ModelSlug, "failed", "ai_model_provider_unavailable", nil)
	}

	secret, err := s.secretResolver.ResolveSecretRef(ctx, tenantID, config.Provider.ID, config.Credential.SecretRef)
	if err != nil {
		status, summary := verifyStatusAndSummary(err)
		return s.recordFailure(ctx, tenantID, principalID, config.Provider.ID, config.Selection.ModelSlug, status, summary, nil)
	}

	startedAt := s.now()
	stream, err := s.adapter.StreamChatCompletion(ctx, ProviderChatRequest{
		BaseURL: strings.TrimSpace(config.Provider.BaseURL),
		APIKey:  secret,
		Model:   strings.TrimSpace(config.Selection.ModelSlug),
		Messages: []PromptItem{
			{Role: "user", Content: "health check"},
		},
		Input: "health check",
	})
	if err != nil {
		status, summary := verifyStatusAndSummary(err)
		latency := latencyFrom(startedAt, s.now())
		return s.recordFailure(ctx, tenantID, principalID, config.Provider.ID, config.Selection.ModelSlug, status, summary, &latency)
	}
	defer func() { _ = stream.Close() }()

	for {
		chunk, err := stream.Recv()
		latency := latencyFrom(startedAt, s.now())
		switch {
		case err == nil && strings.TrimSpace(chunk.Delta) != "":
			return s.healthWriter.RecordModelHealthCheck(ctx, tenantID, principalID, ModelHealthWriteInput{
				ProviderID: config.Provider.ID,
				ModelSlug:  config.Selection.ModelSlug,
				Status:     "healthy",
				LatencyMS:  &latency,
			})
		case err == nil && chunk.Done:
			return s.recordFailure(ctx, tenantID, principalID, config.Provider.ID, config.Selection.ModelSlug, "degraded", "provider_empty_response", &latency)
		case err == nil:
			continue
		case errors.Is(err, io.EOF):
			return s.recordFailure(ctx, tenantID, principalID, config.Provider.ID, config.Selection.ModelSlug, "degraded", "provider_empty_response", &latency)
		default:
			status, summary := verifyStatusAndSummary(err)
			return s.recordFailure(ctx, tenantID, principalID, config.Provider.ID, config.Selection.ModelSlug, status, summary, &latency)
		}
	}
}

func (s *ModelVerificationService) recordFailure(
	ctx context.Context,
	tenantID string,
	principalID string,
	providerID string,
	modelSlug string,
	status string,
	summary string,
	latency *int,
) (ModelHealth, error) {
	return s.healthWriter.RecordModelHealthCheck(ctx, tenantID, principalID, ModelHealthWriteInput{
		ProviderID:   providerID,
		ModelSlug:    modelSlug,
		Status:       status,
		LatencyMS:    latency,
		ErrorSummary: summary,
	})
}

func verifyStatusAndSummary(err error) (string, string) {
	switch {
	case errors.Is(err, ErrSecretMissing):
		return "failed", "ai_model_secret_missing"
	case errors.Is(err, ErrSecretRefInvalid):
		return "failed", "ai_model_secret_missing"
	case errors.Is(err, ErrProviderUnauthorized):
		return "failed", "provider_auth_failed"
	case errors.Is(err, ErrProviderConfigInvalid), errors.Is(err, ErrProviderStreamInvalid):
		return "failed", "provider_stream_invalid"
	case errors.Is(err, ErrProviderRateLimited):
		return "degraded", "provider_rate_limited"
	case errors.Is(err, ErrProviderTimeout):
		return "degraded", "provider_stream_timeout"
	case errors.Is(err, ErrProviderUnavailable):
		return "degraded", "provider_unavailable"
	default:
		return "failed", "provider_verify_failed"
	}
}

func latencyFrom(startedAt time.Time, finishedAt time.Time) int {
	if startedAt.IsZero() || finishedAt.Before(startedAt) {
		return 0
	}
	return int(finishedAt.Sub(startedAt).Milliseconds())
}
