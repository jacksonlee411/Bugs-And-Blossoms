package cubebox

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

var ErrProviderConfigInvalid = errors.New("CUBEBOX_PROVIDER_CONFIG_INVALID")
var ErrProviderDisabled = errors.New("CUBEBOX_PROVIDER_DISABLED")
var ErrModelSlugMissing = errors.New("CUBEBOX_MODEL_SLUG_MISSING")
var ErrSecretRefInvalid = errors.New("CUBEBOX_SECRET_REF_INVALID")
var ErrSecretMissing = errors.New("CUBEBOX_SECRET_MISSING")
var ErrProviderUnauthorized = errors.New("CUBEBOX_PROVIDER_UNAUTHORIZED")
var ErrProviderRateLimited = errors.New("CUBEBOX_PROVIDER_RATE_LIMITED")
var ErrProviderUnavailable = errors.New("CUBEBOX_PROVIDER_UNAVAILABLE")
var ErrProviderStreamInvalid = errors.New("CUBEBOX_PROVIDER_STREAM_INVALID")
var ErrProviderTimeout = errors.New("CUBEBOX_PROVIDER_TIMEOUT")

const terminalAppendTimeout = 5 * time.Second

type RuntimeConfigReader interface {
	GetActiveModelRuntimeConfig(ctx context.Context, tenantID string) (ActiveModelRuntimeConfig, error)
}

type StreamAppendStore interface {
	CompactConversation(ctx context.Context, tenantID string, principalID string, conversationID string, canonicalContext CanonicalContext, reason string) (CompactConversationResponse, error)
	AppendEvent(ctx context.Context, tenantID string, principalID string, conversationID string, event CanonicalEvent) error
	AppendEvents(ctx context.Context, tenantID string, principalID string, conversationID string, events []CanonicalEvent) error
}

type SecretResolver interface {
	ResolveSecretRef(ctx context.Context, tenantID string, providerID string, secretRef string) (string, error)
}

type ProviderAdapter interface {
	StreamChatCompletion(ctx context.Context, request ProviderChatRequest) (ProviderChatStream, error)
}

type ProviderChatRequest struct {
	BaseURL string
	APIKey  string
	Model   string
	Input   string
}

type ProviderChatChunk struct {
	Delta string
	Done  bool
}

type ProviderChatStream interface {
	Recv() (ProviderChatChunk, error)
	Close() error
}

type GatewayStreamRequest struct {
	TenantID       string
	PrincipalID    string
	ConversationID string
	Prompt         string
	NextSequence   int
}

type GatewayEventSink interface {
	Write(CanonicalEvent) bool
	WriteFallback(CanonicalEvent)
}

type GatewayService struct {
	runtime        *Runtime
	configReader   RuntimeConfigReader
	adapter        ProviderAdapter
	secretResolver SecretResolver
	now            func() time.Time
}

type gatewayLifecycleMeta struct {
	traceID      string
	providerID   string
	providerType string
	modelSlug    string
	runtime      string
	startedAt    time.Time
}

func NewGatewayService(runtime *Runtime, configReader RuntimeConfigReader, adapter ProviderAdapter, secretResolver SecretResolver) *GatewayService {
	return &GatewayService{
		runtime:        runtime,
		configReader:   configReader,
		adapter:        adapter,
		secretResolver: secretResolver,
		now:            func() time.Time { return time.Now().UTC() },
	}
}

func (s *GatewayService) StreamTurn(
	ctx context.Context,
	request GatewayStreamRequest,
	store StreamAppendStore,
	sink GatewayEventSink,
) {
	startedAt := s.now()
	turn := s.runtime.StartTurn(TurnOwner{
		TenantID:       request.TenantID,
		PrincipalID:    request.PrincipalID,
		ConversationID: request.ConversationID,
	}, request.Prompt)
	defer s.runtime.FinishTurn(turn.TurnID)

	lifecycle := gatewayLifecycleMeta{
		traceID:   "trace_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		runtime:   "deterministic-fixture",
		startedAt: startedAt,
	}
	hasProviderRuntime := s.configReader != nil && s.adapter != nil && s.secretResolver != nil
	var config ActiveModelRuntimeConfig
	var configErr error
	if hasProviderRuntime {
		lifecycle.runtime = "unavailable"
		config, configErr = s.configReader.GetActiveModelRuntimeConfig(ctx, request.TenantID)
		if configErr == nil {
			lifecycle.providerID = strings.TrimSpace(config.Provider.ID)
			lifecycle.providerType = strings.TrimSpace(config.Provider.ProviderType)
			lifecycle.modelSlug = strings.TrimSpace(config.Selection.ModelSlug)
			lifecycle.runtime = "openai-chat-completions"
		}
	}

	sequence := request.NextSequence
	if sequence <= 0 {
		sequence = 1
	}
	if sequence > 1 {
		compactPayload, err := store.CompactConversation(ctx, request.TenantID, request.PrincipalID, request.ConversationID, CanonicalContext{
			TenantID:       request.TenantID,
			PrincipalID:    request.PrincipalID,
			Language:       "zh",
			Page:           "/app/cubebox",
			Permissions:    []string{"cubebox.conversations:use"},
			BusinessObject: "conversation",
			ProviderID:     lifecycle.providerID,
			ProviderType:   lifecycle.providerType,
			ModelSlug:      lifecycle.modelSlug,
			Runtime:        lifecycle.runtime,
		}, "pre_turn_auto")
		if err != nil && !errors.Is(err, ErrConversationNotFound) {
			s.appendTerminalError(ctx, store, sink, request, turn.TurnID, &sequence, lifecycle, "cubebox_turn_stream_failed", "会话压缩失败，当前响应已终止。", false)
			return
		}
		if compactPayload.NextSequence > sequence {
			sequence = compactPayload.NextSequence
		}
	}

	writeEvent := func(eventType string, payload map[string]any) bool {
		event := CanonicalEvent{
			EventID:        "evt_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
			ConversationID: request.ConversationID,
			TurnID:         &turn.TurnID,
			Sequence:       sequence,
			Type:           eventType,
			TS:             s.now().Format(time.RFC3339),
			Payload:        payload,
		}
		sequence += 1
		if err := store.AppendEvent(ctx, request.TenantID, request.PrincipalID, request.ConversationID, event); err != nil {
			sink.WriteFallback(CanonicalEvent{
				EventID:        event.EventID,
				ConversationID: request.ConversationID,
				TurnID:         &turn.TurnID,
				Sequence:       event.Sequence,
				Type:           "turn.error",
				TS:             s.now().Format(time.RFC3339),
				Payload:        s.errorPayload("event_log_write_failed", "会话事件落库失败，当前响应已终止。", false, lifecycle),
			})
			return false
		}
		return sink.Write(event)
	}

	if !writeEvent("turn.started", s.startedPayload(turn.UserMessageID, lifecycle)) {
		return
	}
	if !writeEvent("turn.user_message.accepted", map[string]any{"message_id": turn.UserMessageID, "text": turn.Prompt}) {
		return
	}

	if !hasProviderRuntime {
		s.streamDeterministicFixture(ctx, turn, request, lifecycle, store, sink, &sequence, writeEvent)
		return
	}

	if configErr != nil {
		s.writeConfigError(store, sink, ctx, request, turn.TurnID, &sequence, lifecycle, configErr)
		return
	}
	if !config.Provider.Enabled {
		s.writeConfigError(store, sink, ctx, request, turn.TurnID, &sequence, lifecycle, ErrProviderDisabled)
		return
	}
	if strings.TrimSpace(config.Selection.ModelSlug) == "" {
		s.writeConfigError(store, sink, ctx, request, turn.TurnID, &sequence, lifecycle, ErrModelSlugMissing)
		return
	}
	secret, err := s.secretResolver.ResolveSecretRef(ctx, request.TenantID, config.Provider.ID, config.Credential.SecretRef)
	if err != nil {
		s.writeConfigError(store, sink, ctx, request, turn.TurnID, &sequence, lifecycle, err)
		return
	}
	providerCtx, cancelProvider := context.WithCancel(ctx)
	defer cancelProvider()
	var interrupted atomic.Bool
	go func() {
		select {
		case <-turn.InterruptSignal():
			interrupted.Store(true)
			cancelProvider()
		case <-providerCtx.Done():
		}
	}()

	stream, err := s.adapter.StreamChatCompletion(providerCtx, ProviderChatRequest{
		BaseURL: strings.TrimSpace(config.Provider.BaseURL),
		APIKey:  secret,
		Model:   strings.TrimSpace(config.Selection.ModelSlug),
		Input:   turn.Prompt,
	})
	if err != nil {
		s.writeProviderError(store, sink, ctx, request, turn.TurnID, &sequence, lifecycle, err)
		return
	}
	defer func() { _ = stream.Close() }()

	for {
		select {
		case <-ctx.Done():
			s.writeProviderError(store, sink, ctx, request, turn.TurnID, &sequence, lifecycle, providerErrorFromContext(ctx.Err()))
			return
		case <-turn.InterruptSignal():
			interrupted.Store(true)
			cancelProvider()
			_ = writeEvent("turn.interrupted", s.interruptedPayload("user_requested", lifecycle))
			_ = writeEvent("turn.completed", s.completedPayload("interrupted", lifecycle))
			return
		default:
		}

		chunk, err := stream.Recv()
		if err != nil {
			if interrupted.Load() {
				_ = writeEvent("turn.interrupted", s.interruptedPayload("user_requested", lifecycle))
				_ = writeEvent("turn.completed", s.completedPayload("interrupted", lifecycle))
				return
			}
			if ctx.Err() != nil {
				s.writeProviderError(store, sink, ctx, request, turn.TurnID, &sequence, lifecycle, providerErrorFromContext(ctx.Err()))
				return
			}
			if errors.Is(err, io.EOF) {
				_ = writeEvent("turn.agent_message.completed", map[string]any{"message_id": turn.AssistantMessageID})
				_ = writeEvent("turn.completed", s.completedPayload("completed", lifecycle))
				return
			}
			s.writeProviderError(store, sink, ctx, request, turn.TurnID, &sequence, lifecycle, err)
			return
		}
		if chunk.Done {
			_ = writeEvent("turn.agent_message.completed", map[string]any{"message_id": turn.AssistantMessageID})
			_ = writeEvent("turn.completed", s.completedPayload("completed", lifecycle))
			return
		}
		if strings.TrimSpace(chunk.Delta) == "" {
			continue
		}
		if !writeEvent("turn.agent_message.delta", map[string]any{
			"message_id": turn.AssistantMessageID,
			"delta":      chunk.Delta,
		}) {
			return
		}
	}
}

func (s *GatewayService) streamDeterministicFixture(
	ctx context.Context,
	turn DeterministicTurn,
	request GatewayStreamRequest,
	lifecycle gatewayLifecycleMeta,
	store StreamAppendStore,
	sink GatewayEventSink,
	sequence *int,
	writeEvent func(string, map[string]any) bool,
) {
	if turn.ShouldError {
		s.appendTerminalError(ctx, store, sink, request, turn.TurnID, sequence, lifecycle, "deterministic_provider_error", "当前回复暂时失败，请稍后重试。", false)
		return
	}
	for _, chunk := range turn.Chunks {
		select {
		case <-ctx.Done():
			return
		case <-turn.InterruptSignal():
			_ = writeEvent("turn.interrupted", s.interruptedPayload("user_requested", lifecycle))
			_ = writeEvent("turn.completed", s.completedPayload("interrupted", lifecycle))
			return
		case <-time.After(25 * time.Millisecond):
		}
		if !writeEvent("turn.agent_message.delta", map[string]any{
			"message_id": turn.AssistantMessageID,
			"delta":      chunk,
		}) {
			return
		}
	}
	_ = writeEvent("turn.agent_message.completed", map[string]any{"message_id": turn.AssistantMessageID})
	_ = writeEvent("turn.completed", s.completedPayload("completed", lifecycle))
}

func (s *GatewayService) writeConfigError(store StreamAppendStore, sink GatewayEventSink, ctx context.Context, request GatewayStreamRequest, turnID string, sequence *int, lifecycle gatewayLifecycleMeta, err error) {
	code := "ai_model_config_invalid"
	message := "模型配置无效，请联系管理员检查。"
	switch {
	case errors.Is(err, ErrModelProviderNotFound), errors.Is(err, ErrProviderDisabled), errors.Is(err, ErrActiveModelSelectionNotFound):
		code = "ai_model_provider_unavailable"
		message = "当前模型供应商不可用，请稍后重试。"
	case errors.Is(err, ErrModelCredentialNotFound), errors.Is(err, ErrSecretMissing), errors.Is(err, ErrSecretRefInvalid):
		code = "ai_model_secret_missing"
		message = "当前模型密钥不可用，请联系管理员检查。"
	}
	s.appendTerminalError(ctx, store, sink, request, turnID, sequence, lifecycle, code, message, false)
}

func (s *GatewayService) writeProviderError(store StreamAppendStore, sink GatewayEventSink, ctx context.Context, request GatewayStreamRequest, turnID string, sequence *int, lifecycle gatewayLifecycleMeta, err error) {
	code := "cubebox_turn_stream_failed"
	message := "当前回复暂时失败，请稍后重试。"
	retryable := false
	switch {
	case errors.Is(err, ErrProviderUnauthorized):
		code = "ai_model_provider_unavailable"
		message = "当前模型认证失败，请联系管理员检查。"
	case errors.Is(err, ErrProviderRateLimited):
		code = "cubebox_turn_stream_failed"
		message = "当前模型请求过多，请稍后重试。"
		retryable = true
	case errors.Is(err, ErrProviderUnavailable), errors.Is(err, ErrProviderTimeout):
		code = "ai_model_provider_unavailable"
		message = "当前模型供应商暂不可用，请稍后重试。"
		retryable = true
	case errors.Is(err, ErrProviderStreamInvalid):
		code = "cubebox_turn_stream_failed"
		message = "当前模型返回异常响应，请稍后重试。"
	}
	s.appendTerminalError(ctx, store, sink, request, turnID, sequence, lifecycle, code, message, retryable)
}

func (s *GatewayService) appendTerminalError(
	ctx context.Context,
	store StreamAppendStore,
	sink GatewayEventSink,
	request GatewayStreamRequest,
	turnID string,
	sequence *int,
	lifecycle gatewayLifecycleMeta,
	code string,
	message string,
	retryable bool,
) {
	events := s.terminalErrorEvents(request.ConversationID, &turnID, sequence, lifecycle, code, message, retryable)
	appendCtx, cancel := terminalAppendContext(ctx)
	defer cancel()
	if err := store.AppendEvents(appendCtx, request.TenantID, request.PrincipalID, request.ConversationID, events); err != nil {
		sink.WriteFallback(CanonicalEvent{
			EventID:        "evt_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
			ConversationID: request.ConversationID,
			TurnID:         &turnID,
			Sequence:       events[0].Sequence,
			Type:           "turn.error",
			TS:             s.now().Format(time.RFC3339),
			Payload: map[string]any{
				"code":      "event_log_write_failed",
				"message":   "会话事件落库失败，当前响应已终止。",
				"retryable": false,
				"trace_id":  lifecycle.traceID,
			},
		})
		return
	}
	for _, event := range events {
		if !sink.Write(event) {
			return
		}
	}
}

func (s *GatewayService) terminalErrorEvents(conversationID string, turnID *string, sequence *int, lifecycle gatewayLifecycleMeta, code string, message string, retryable bool) []CanonicalEvent {
	errorEvent := CanonicalEvent{
		EventID:        "evt_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		ConversationID: conversationID,
		TurnID:         turnID,
		Sequence:       *sequence,
		Type:           "turn.error",
		TS:             s.now().Format(time.RFC3339),
		Payload:        s.errorPayload(code, message, retryable, lifecycle),
	}
	*sequence = *sequence + 1
	terminalEvent := CanonicalEvent{
		EventID:        "evt_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		ConversationID: conversationID,
		TurnID:         turnID,
		Sequence:       *sequence,
		Type:           "turn.completed",
		TS:             s.now().Format(time.RFC3339),
		Payload:        s.completedPayload("failed", lifecycle),
	}
	*sequence = *sequence + 1
	return []CanonicalEvent{errorEvent, terminalEvent}
}

func terminalAppendContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), terminalAppendTimeout)
}

func providerErrorFromContext(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrProviderTimeout
	}
	return ErrProviderUnavailable
}

func (s *GatewayService) startedPayload(userMessageID string, lifecycle gatewayLifecycleMeta) map[string]any {
	return map[string]any{
		"user_message_id": userMessageID,
		"trace_id":        lifecycle.traceID,
		"provider_id":     lifecycle.providerID,
		"provider_type":   lifecycle.providerType,
		"model_slug":      lifecycle.modelSlug,
		"runtime":         lifecycle.runtime,
	}
}

func (s *GatewayService) errorPayload(code string, message string, retryable bool, lifecycle gatewayLifecycleMeta) map[string]any {
	payload := s.lifecyclePayload(lifecycle)
	payload["code"] = code
	payload["message"] = message
	payload["retryable"] = retryable
	payload["latency_ms"] = s.latencyMS(lifecycle)
	return payload
}

func (s *GatewayService) interruptedPayload(reason string, lifecycle gatewayLifecycleMeta) map[string]any {
	payload := s.lifecyclePayload(lifecycle)
	payload["reason"] = reason
	payload["latency_ms"] = s.latencyMS(lifecycle)
	return payload
}

func (s *GatewayService) completedPayload(status string, lifecycle gatewayLifecycleMeta) map[string]any {
	payload := s.lifecyclePayload(lifecycle)
	payload["status"] = status
	payload["latency_ms"] = s.latencyMS(lifecycle)
	return payload
}

func (s *GatewayService) lifecyclePayload(lifecycle gatewayLifecycleMeta) map[string]any {
	return map[string]any{
		"trace_id":      lifecycle.traceID,
		"provider_id":   lifecycle.providerID,
		"provider_type": lifecycle.providerType,
		"model_slug":    lifecycle.modelSlug,
		"runtime":       lifecycle.runtime,
	}
}

func (s *GatewayService) latencyMS(lifecycle gatewayLifecycleMeta) int64 {
	startedAt := lifecycle.startedAt
	if startedAt.IsZero() {
		startedAt = s.now()
	}
	latency := s.now().Sub(startedAt).Milliseconds()
	if latency < 0 {
		return 0
	}
	return latency
}

type EnvSecretResolver struct{}

func (EnvSecretResolver) ResolveSecretRef(_ context.Context, _ string, _ string, secretRef string) (string, error) {
	secretRef = strings.TrimSpace(secretRef)
	if secretRef == "" {
		return "", ErrSecretMissing
	}
	if !strings.HasPrefix(secretRef, "env://") {
		return "", ErrSecretRefInvalid
	}
	envName := strings.TrimSpace(strings.TrimPrefix(secretRef, "env://"))
	if envName == "" {
		return "", ErrSecretRefInvalid
	}
	value := strings.TrimSpace(os.Getenv(envName))
	if value == "" {
		return "", ErrSecretMissing
	}
	return value, nil
}

type OpenAICompatibleAdapter struct {
	client *http.Client
}

func NewOpenAICompatibleAdapter(client *http.Client) *OpenAICompatibleAdapter {
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	return &OpenAICompatibleAdapter{client: client}
}

func (a *OpenAICompatibleAdapter) StreamChatCompletion(ctx context.Context, request ProviderChatRequest) (ProviderChatStream, error) {
	if strings.TrimSpace(request.BaseURL) == "" || strings.TrimSpace(request.Model) == "" {
		return nil, ErrProviderConfigInvalid
	}
	body, err := json.Marshal(map[string]any{
		"model": request.Model,
		"messages": []map[string]string{
			{"role": "user", "content": request.Input},
		},
		"stream": true,
	})
	if err != nil {
		return nil, err
	}
	url := strings.TrimRight(strings.TrimSpace(request.BaseURL), "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+request.APIKey)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrProviderTimeout
		}
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return nil, ErrProviderTimeout
		}
		return nil, ErrProviderUnavailable
	}
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		_ = resp.Body.Close()
		return nil, ErrProviderUnauthorized
	case http.StatusTooManyRequests:
		_ = resp.Body.Close()
		return nil, ErrProviderRateLimited
	default:
		if resp.StatusCode >= 500 {
			_ = resp.Body.Close()
			return nil, ErrProviderUnavailable
		}
		if resp.StatusCode >= 400 {
			_ = resp.Body.Close()
			return nil, ErrProviderConfigInvalid
		}
	}
	return &openAICompatibleStream{
		body:    resp.Body,
		scanner: bufio.NewScanner(resp.Body),
	}, nil
}

type openAICompatibleStream struct {
	body    io.ReadCloser
	scanner *bufio.Scanner
}

func (s *openAICompatibleStream) Recv() (ProviderChatChunk, error) {
	for s.scanner.Scan() {
		line := strings.TrimSpace(s.scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			return ProviderChatChunk{}, ErrProviderStreamInvalid
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			return ProviderChatChunk{Done: true}, nil
		}
		var decoded struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
			return ProviderChatChunk{}, ErrProviderStreamInvalid
		}
		if len(decoded.Choices) == 0 {
			continue
		}
		if decoded.Choices[0].FinishReason != nil && *decoded.Choices[0].FinishReason != "" {
			return ProviderChatChunk{Done: true}, nil
		}
		return ProviderChatChunk{Delta: decoded.Choices[0].Delta.Content}, nil
	}
	if err := s.scanner.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return ProviderChatChunk{}, ErrProviderTimeout
		}
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return ProviderChatChunk{}, ErrProviderTimeout
		}
		return ProviderChatChunk{}, ErrProviderStreamInvalid
	}
	return ProviderChatChunk{}, io.EOF
}

func (s *openAICompatibleStream) Close() error {
	if s.body == nil {
		return nil
	}
	return s.body.Close()
}
