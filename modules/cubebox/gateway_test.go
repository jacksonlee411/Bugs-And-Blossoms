package cubebox

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

type providerChunkStub struct {
	chunks []ProviderChatChunk
	errs   []error
	index  int
}

func (s *providerChunkStub) Recv() (ProviderChatChunk, error) {
	if s.index < len(s.errs) && s.errs[s.index] != nil {
		err := s.errs[s.index]
		s.index++
		return ProviderChatChunk{}, err
	}
	if s.index >= len(s.chunks) {
		return ProviderChatChunk{}, io.EOF
	}
	chunk := s.chunks[s.index]
	s.index++
	return chunk, nil
}

func (s *providerChunkStub) Close() error { return nil }

type runtimeConfigReaderStub struct {
	config ActiveModelRuntimeConfig
	err    error
}

func (s runtimeConfigReaderStub) GetActiveModelRuntimeConfig(context.Context, string) (ActiveModelRuntimeConfig, error) {
	if s.err != nil {
		return ActiveModelRuntimeConfig{}, s.err
	}
	return s.config, nil
}

type modelHealthWriterStub struct {
	last ModelHealthWriteInput
}

func (s *modelHealthWriterStub) RecordModelHealthCheck(_ context.Context, _ string, _ string, input ModelHealthWriteInput) (ModelHealth, error) {
	s.last = input
	return ModelHealth{
		ProviderID:   input.ProviderID,
		ModelSlug:    input.ModelSlug,
		Status:       input.Status,
		LatencyMS:    input.LatencyMS,
		ErrorSummary: input.ErrorSummary,
	}, nil
}

type providerAdapterStub struct {
	stream ProviderChatStream
	err    error
}

func (s providerAdapterStub) StreamChatCompletion(context.Context, ProviderChatRequest) (ProviderChatStream, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.stream, nil
}

type secretResolverStub struct {
	secret string
	err    error
}

func (s secretResolverStub) ResolveSecretRef(context.Context, string, string, string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.secret, nil
}

type appendEventStoreStub struct {
	events          []CanonicalEvent
	appendErr       error
	appendEventsErr error
	appendEventsCtx context.Context
}

func (s *appendEventStoreStub) CompactConversation(context.Context, string, string, string, CanonicalContext, string) (CompactConversationResponse, error) {
	return CompactConversationResponse{}, nil
}

func (s *appendEventStoreStub) AppendEvent(_ context.Context, _ string, _ string, _ string, event CanonicalEvent) error {
	if s.appendErr != nil {
		return s.appendErr
	}
	s.events = append(s.events, event)
	return nil
}

func (s *appendEventStoreStub) AppendEvents(_ context.Context, _ string, _ string, _ string, events []CanonicalEvent) error {
	if s.appendEventsErr != nil {
		return s.appendEventsErr
	}
	s.events = append(s.events, events...)
	return nil
}

type appendEventsContextStoreStub struct {
	appendEventStoreStub
	appendEventsCtxErr error
}

func (s *appendEventsContextStoreStub) AppendEvents(ctx context.Context, tenantID string, principalID string, conversationID string, events []CanonicalEvent) error {
	s.appendEventsCtx = ctx
	s.appendEventsCtxErr = ctx.Err()
	return s.appendEventStoreStub.AppendEvents(ctx, tenantID, principalID, conversationID, events)
}

type eventSinkStub struct {
	events []CanonicalEvent
}

func (s *eventSinkStub) Write(event CanonicalEvent) bool {
	s.events = append(s.events, event)
	return true
}

func (s *eventSinkStub) WriteFallback(event CanonicalEvent) {
	s.events = append(s.events, event)
}

func TestEnvSecretResolverResolveSecretRef(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test")
	resolver := EnvSecretResolver{}

	secret, err := resolver.ResolveSecretRef(context.Background(), "tenant-1", "provider-1", "env://OPENAI_API_KEY")
	if err != nil {
		t.Fatalf("resolve secret: %v", err)
	}
	if secret != "sk-test" {
		t.Fatalf("secret=%q", secret)
	}
}

func TestEnvSecretResolverRejectsInvalidRef(t *testing.T) {
	resolver := EnvSecretResolver{}
	if _, err := resolver.ResolveSecretRef(context.Background(), "tenant-1", "provider-1", "file://OPENAI_API_KEY"); !errors.Is(err, ErrSecretRefInvalid) {
		t.Fatalf("expected invalid ref, got %v", err)
	}
	if _, err := resolver.ResolveSecretRef(context.Background(), "tenant-1", "provider-1", "env://MISSING_KEY"); !errors.Is(err, ErrSecretMissing) {
		t.Fatalf("expected missing secret, got %v", err)
	}
}

func TestOpenAICompatibleAdapterStreamChatCompletion(t *testing.T) {
	var sawAuth string
	var sawBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		payload, _ := io.ReadAll(r.Body)
		sawBody = string(payload)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"你好\"},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	adapter := NewOpenAICompatibleAdapter(server.Client())
	stream, err := adapter.StreamChatCompletion(context.Background(), ProviderChatRequest{
		BaseURL: server.URL,
		APIKey:  "sk-test",
		Model:   "gpt-4.1",
		Input:   "hello",
	})
	if err != nil {
		t.Fatalf("stream chat completion: %v", err)
	}
	defer func() { _ = stream.Close() }()

	chunk, err := stream.Recv()
	if err != nil {
		t.Fatalf("recv first chunk: %v", err)
	}
	if chunk.Delta != "你好" {
		t.Fatalf("delta=%q", chunk.Delta)
	}
	done, err := stream.Recv()
	if err != nil {
		t.Fatalf("recv done chunk: %v", err)
	}
	if !done.Done {
		t.Fatalf("expected done chunk, got %+v", done)
	}
	if sawAuth != "Bearer sk-test" {
		t.Fatalf("authorization=%q", sawAuth)
	}
	if !strings.Contains(sawBody, `"model":"gpt-4.1"`) || !strings.Contains(sawBody, `"stream":true`) {
		t.Fatalf("unexpected body=%s", sawBody)
	}
}

func TestOpenAICompatibleAdapterMapsHTTPErrors(t *testing.T) {
	testCases := []struct {
		name   string
		status int
		want   error
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, want: ErrProviderUnauthorized},
		{name: "rate_limited", status: http.StatusTooManyRequests, want: ErrProviderRateLimited},
		{name: "server_error", status: http.StatusBadGateway, want: ErrProviderUnavailable},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
			}))
			defer server.Close()

			adapter := NewOpenAICompatibleAdapter(server.Client())
			_, err := adapter.StreamChatCompletion(context.Background(), ProviderChatRequest{
				BaseURL: server.URL,
				APIKey:  "sk-test",
				Model:   "gpt-4.1",
				Input:   "hello",
			})
			if !errors.Is(err, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
		})
	}
}

func TestModelVerificationServiceVerifyActiveModelHealthy(t *testing.T) {
	writer := &modelHealthWriterStub{}
	service := NewModelVerificationService(
		runtimeConfigReaderStub{
			config: ActiveModelRuntimeConfig{
				Selection:  ActiveModelSelection{ProviderID: "provider-1", ModelSlug: "gpt-4.1"},
				Provider:   ModelProvider{ID: "provider-1", BaseURL: "https://example.invalid/v1", Enabled: true},
				Credential: ModelCredential{SecretRef: "env://OPENAI_API_KEY", Active: true},
			},
		},
		writer,
		providerAdapterStub{stream: &providerChunkStub{chunks: []ProviderChatChunk{{Delta: "ok"}}}},
		secretResolverStub{secret: "sk-test"},
	)

	health, err := service.VerifyActiveModel(context.Background(), "tenant-1", "principal-1")
	if err != nil {
		t.Fatalf("verify active model: %v", err)
	}
	if health.Status != "healthy" {
		t.Fatalf("health=%+v", health)
	}
	if writer.last.ProviderID != "provider-1" || writer.last.ModelSlug != "gpt-4.1" {
		t.Fatalf("unexpected write input=%+v", writer.last)
	}
}

func TestModelVerificationServiceVerifyActiveModelWritesFailedOnUnauthorized(t *testing.T) {
	writer := &modelHealthWriterStub{}
	service := NewModelVerificationService(
		runtimeConfigReaderStub{
			config: ActiveModelRuntimeConfig{
				Selection:  ActiveModelSelection{ProviderID: "provider-1", ModelSlug: "gpt-4.1"},
				Provider:   ModelProvider{ID: "provider-1", BaseURL: "https://example.invalid/v1", Enabled: true},
				Credential: ModelCredential{SecretRef: "env://OPENAI_API_KEY", Active: true},
			},
		},
		writer,
		providerAdapterStub{err: ErrProviderUnauthorized},
		secretResolverStub{secret: "sk-test"},
	)

	health, err := service.VerifyActiveModel(context.Background(), "tenant-1", "principal-1")
	if err != nil {
		t.Fatalf("verify active model: %v", err)
	}
	if health.Status != "failed" || health.ErrorSummary != "provider_auth_failed" {
		t.Fatalf("health=%+v", health)
	}
}

func TestModelVerificationServiceVerifyActiveModelWritesDegradedOnRateLimit(t *testing.T) {
	writer := &modelHealthWriterStub{}
	service := NewModelVerificationService(
		runtimeConfigReaderStub{
			config: ActiveModelRuntimeConfig{
				Selection:  ActiveModelSelection{ProviderID: "provider-1", ModelSlug: "gpt-4.1"},
				Provider:   ModelProvider{ID: "provider-1", BaseURL: "https://example.invalid/v1", Enabled: true},
				Credential: ModelCredential{SecretRef: "env://OPENAI_API_KEY", Active: true},
			},
		},
		writer,
		providerAdapterStub{err: ErrProviderRateLimited},
		secretResolverStub{secret: "sk-test"},
	)

	health, err := service.VerifyActiveModel(context.Background(), "tenant-1", "principal-1")
	if err != nil {
		t.Fatalf("verify active model: %v", err)
	}
	if health.Status != "degraded" || health.ErrorSummary != "provider_rate_limited" {
		t.Fatalf("health=%+v", health)
	}
}

func TestGatewayServiceStreamTurnWritesLifecycleTelemetry(t *testing.T) {
	store := &appendEventStoreStub{}
	sink := &eventSinkStub{}
	service := NewGatewayService(
		NewRuntime(),
		runtimeConfigReaderStub{
			config: ActiveModelRuntimeConfig{
				Selection:  ActiveModelSelection{ProviderID: "provider-1", ModelSlug: "gpt-4.1"},
				Provider:   ModelProvider{ID: "provider-1", ProviderType: "openai-compatible", BaseURL: "https://example.invalid/v1", Enabled: true},
				Credential: ModelCredential{SecretRef: "env://OPENAI_API_KEY", Active: true},
			},
		},
		providerAdapterStub{stream: &providerChunkStub{chunks: []ProviderChatChunk{{Delta: "你好"}, {Done: true}}}},
		secretResolverStub{secret: "sk-test"},
	)
	now := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time {
		current := now
		now = now.Add(25 * time.Millisecond)
		return current
	}

	service.StreamTurn(context.Background(), GatewayStreamRequest{
		TenantID:       "tenant-1",
		PrincipalID:    "principal-1",
		ConversationID: "conv-1",
		Prompt:         "hello",
		NextSequence:   1,
	}, store, sink)

	types := collectEventTypes(sink.events)
	wantTypes := []string{
		"turn.started",
		"turn.user_message.accepted",
		"turn.agent_message.delta",
		"turn.agent_message.completed",
		"turn.completed",
	}
	if !reflect.DeepEqual(types, wantTypes) {
		t.Fatalf("event types=%v", types)
	}

	started := sink.events[0]
	if started.Payload["trace_id"] == "" {
		t.Fatalf("missing trace_id in started payload: %+v", started.Payload)
	}
	if started.Payload["provider_id"] != "provider-1" || started.Payload["provider_type"] != "openai-compatible" {
		t.Fatalf("unexpected provider metadata: %+v", started.Payload)
	}
	if started.Payload["model_slug"] != "gpt-4.1" || started.Payload["runtime"] != "openai-chat-completions" {
		t.Fatalf("unexpected runtime metadata: %+v", started.Payload)
	}

	completed := sink.events[len(sink.events)-1]
	if completed.Payload["status"] != "completed" {
		t.Fatalf("unexpected completed payload: %+v", completed.Payload)
	}
	if completed.Payload["trace_id"] != started.Payload["trace_id"] {
		t.Fatalf("trace_id mismatch started=%v completed=%v", started.Payload["trace_id"], completed.Payload["trace_id"])
	}
	if _, ok := completed.Payload["latency_ms"]; !ok {
		t.Fatalf("missing latency_ms in completed payload: %+v", completed.Payload)
	}
}

func TestGatewayServiceStreamTurnMapsProviderErrorWithLifecycleTelemetry(t *testing.T) {
	store := &appendEventStoreStub{}
	sink := &eventSinkStub{}
	service := NewGatewayService(
		NewRuntime(),
		runtimeConfigReaderStub{
			config: ActiveModelRuntimeConfig{
				Selection:  ActiveModelSelection{ProviderID: "provider-1", ModelSlug: "gpt-4.1"},
				Provider:   ModelProvider{ID: "provider-1", ProviderType: "openai-compatible", BaseURL: "https://example.invalid/v1", Enabled: true},
				Credential: ModelCredential{SecretRef: "env://OPENAI_API_KEY", Active: true},
			},
		},
		providerAdapterStub{err: ErrProviderTimeout},
		secretResolverStub{secret: "sk-test"},
	)
	now := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time {
		current := now
		now = now.Add(10 * time.Millisecond)
		return current
	}

	service.StreamTurn(context.Background(), GatewayStreamRequest{
		TenantID:       "tenant-1",
		PrincipalID:    "principal-1",
		ConversationID: "conv-1",
		Prompt:         "hello",
		NextSequence:   1,
	}, store, sink)

	if len(sink.events) < 4 {
		t.Fatalf("events=%d", len(sink.events))
	}
	errorEvent := sink.events[len(sink.events)-2]
	completedEvent := sink.events[len(sink.events)-1]
	if errorEvent.Type != "turn.error" || completedEvent.Type != "turn.completed" {
		t.Fatalf("unexpected terminal events: %+v %+v", errorEvent, completedEvent)
	}
	if errorEvent.Payload["code"] != "ai_model_provider_unavailable" {
		t.Fatalf("unexpected error code payload: %+v", errorEvent.Payload)
	}
	if errorEvent.Payload["trace_id"] == "" || completedEvent.Payload["trace_id"] != errorEvent.Payload["trace_id"] {
		t.Fatalf("trace_id mismatch error=%+v completed=%+v", errorEvent.Payload, completedEvent.Payload)
	}
	if _, ok := errorEvent.Payload["latency_ms"]; !ok {
		t.Fatalf("missing latency_ms in error payload: %+v", errorEvent.Payload)
	}
	if completedEvent.Payload["status"] != "failed" {
		t.Fatalf("unexpected completed payload: %+v", completedEvent.Payload)
	}
	if got := collectEventTypes(store.events); !reflect.DeepEqual(got, collectEventTypes(sink.events)) {
		t.Fatalf("terminal events were not appended before SSE, store=%v sink=%v", got, collectEventTypes(sink.events))
	}
}

func TestGatewayServiceStreamTurnAppendsTerminalErrorAfterRequestContextCancelled(t *testing.T) {
	store := &appendEventsContextStoreStub{}
	sink := &eventSinkStub{}
	service := NewGatewayService(
		NewRuntime(),
		runtimeConfigReaderStub{
			config: ActiveModelRuntimeConfig{
				Selection:  ActiveModelSelection{ProviderID: "provider-1", ModelSlug: "gpt-4.1"},
				Provider:   ModelProvider{ID: "provider-1", ProviderType: "openai-compatible", BaseURL: "https://example.invalid/v1", Enabled: true},
				Credential: ModelCredential{SecretRef: "env://OPENAI_API_KEY", Active: true},
			},
		},
		providerAdapterStub{err: ErrProviderTimeout},
		secretResolverStub{secret: "sk-test"},
	)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	service.StreamTurn(ctx, GatewayStreamRequest{
		TenantID:       "tenant-1",
		PrincipalID:    "principal-1",
		ConversationID: "conv-1",
		Prompt:         "hello",
		NextSequence:   1,
	}, store, sink)

	if got := collectEventTypes(store.events); !reflect.DeepEqual(got, []string{
		"turn.started",
		"turn.user_message.accepted",
		"turn.error",
		"turn.completed",
	}) {
		t.Fatalf("terminal events must survive cancelled request context, got %v", got)
	}
	if store.appendEventsCtx == nil {
		t.Fatal("expected append events context")
	}
	if err := store.appendEventsCtxErr; err != nil {
		t.Fatalf("terminal append context must not inherit request cancellation, err=%v", err)
	}
}

func TestGatewayServiceStreamTurnMarksConfigErrorRuntimeUnavailable(t *testing.T) {
	store := &appendEventStoreStub{}
	sink := &eventSinkStub{}
	service := NewGatewayService(
		NewRuntime(),
		runtimeConfigReaderStub{err: ErrActiveModelSelectionNotFound},
		providerAdapterStub{stream: &providerChunkStub{chunks: []ProviderChatChunk{{Delta: "unreachable"}}}},
		secretResolverStub{secret: "sk-test"},
	)

	service.StreamTurn(context.Background(), GatewayStreamRequest{
		TenantID:       "tenant-1",
		PrincipalID:    "principal-1",
		ConversationID: "conv-1",
		Prompt:         "hello",
		NextSequence:   1,
	}, store, sink)

	if len(sink.events) < 4 {
		t.Fatalf("events=%d", len(sink.events))
	}
	started := sink.events[0]
	errorEvent := sink.events[len(sink.events)-2]
	completedEvent := sink.events[len(sink.events)-1]
	for _, event := range []CanonicalEvent{started, errorEvent, completedEvent} {
		if event.Payload["runtime"] != "unavailable" {
			t.Fatalf("expected runtime unavailable, event=%+v", event)
		}
	}
	if errorEvent.Payload["runtime"] == "deterministic-fixture" {
		t.Fatalf("config error must not be reported as deterministic runtime: %+v", errorEvent.Payload)
	}
	if errorEvent.Payload["code"] != "ai_model_provider_unavailable" || completedEvent.Payload["status"] != "failed" {
		t.Fatalf("unexpected terminal payloads: error=%+v completed=%+v", errorEvent.Payload, completedEvent.Payload)
	}
}

func TestGatewayServiceStreamTurnWritesFallbackOnlyWhenTerminalAppendFails(t *testing.T) {
	store := &appendEventStoreStub{appendEventsErr: errors.New("append failed")}
	sink := &eventSinkStub{}
	service := NewGatewayService(
		NewRuntime(),
		runtimeConfigReaderStub{
			config: ActiveModelRuntimeConfig{
				Selection:  ActiveModelSelection{ProviderID: "provider-1", ModelSlug: "gpt-4.1"},
				Provider:   ModelProvider{ID: "provider-1", ProviderType: "openai-compatible", BaseURL: "https://example.invalid/v1", Enabled: true},
				Credential: ModelCredential{SecretRef: "env://OPENAI_API_KEY", Active: true},
			},
		},
		providerAdapterStub{err: ErrProviderTimeout},
		secretResolverStub{secret: "sk-test"},
	)

	service.StreamTurn(context.Background(), GatewayStreamRequest{
		TenantID:       "tenant-1",
		PrincipalID:    "principal-1",
		ConversationID: "conv-1",
		Prompt:         "hello",
		NextSequence:   1,
	}, store, sink)

	if len(store.events) != 2 {
		t.Fatalf("expected started and user events to be appended before provider failure, got %d", len(store.events))
	}
	if len(sink.events) == 0 {
		t.Fatal("expected fallback SSE")
	}
	fallback := sink.events[len(sink.events)-1]
	if fallback.Type != "turn.error" || fallback.Payload["code"] != "event_log_write_failed" {
		t.Fatalf("unexpected fallback event=%+v", fallback)
	}
	if got := collectEventTypes(sink.events); containsType(got, "turn.completed") {
		t.Fatalf("fallback path must not fake terminal completed event, got %v", got)
	}
}

func collectEventTypes(events []CanonicalEvent) []string {
	types := make([]string, 0, len(events))
	for _, event := range events {
		types = append(types, event.Type)
	}
	return types
}

func containsType(types []string, want string) bool {
	for _, eventType := range types {
		if eventType == want {
			return true
		}
	}
	return false
}
