package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox"
)

func newTestGateway(runtime *cubebox.Runtime) *cubebox.GatewayService {
	return cubebox.NewGatewayService(runtime, nil, nil, nil)
}

type cubeboxReadPlanProducerStub struct {
	result cubeboxReadPlanProductionResult
	err    error
}

func (s cubeboxReadPlanProducerStub) ProduceReadPlan(context.Context, cubeboxReadPlanProductionInput) (cubeboxReadPlanProductionResult, error) {
	if s.err != nil {
		return cubeboxReadPlanProductionResult{}, s.err
	}
	return s.result, nil
}

type cubeboxQueryNarratorStub struct {
	text string
	err  error
	fn   func(context.Context, cubeboxQueryNarrationInput) (string, error)
}

func (s cubeboxQueryNarratorStub) NarrateQueryResult(ctx context.Context, input cubeboxQueryNarrationInput) (string, error) {
	if s.fn != nil {
		return s.fn(ctx, input)
	}
	if s.err != nil {
		return "", s.err
	}
	return s.text, nil
}

type cubeboxAuthorizerStub struct {
	allowed map[string]bool
	err     error
}

func (s cubeboxAuthorizerStub) Authorize(_ string, _ string, object string, action string) (bool, bool, error) {
	if s.err != nil {
		return false, true, s.err
	}
	return s.allowed[object+":"+action], true, nil
}

type cubeboxStoreStub struct {
	createFn               func(context.Context, string, string) (cubebox.ConversationReplayResponse, error)
	getFn                  func(context.Context, string, string, string) (cubebox.ConversationReplayResponse, error)
	listFn                 func(context.Context, string, string, int32) (cubebox.ConversationListResponse, error)
	renameFn               func(context.Context, string, string, string, string) (cubebox.ConversationReplayResponse, error)
	archiveFn              func(context.Context, string, string, string, bool) (cubebox.ConversationReplayResponse, error)
	compactFn              func(context.Context, string, string, string, cubebox.CanonicalContext, string) (cubebox.CompactConversationResponse, error)
	appendFn               func(context.Context, string, string, string, cubebox.CanonicalEvent) error
	settingsFn             func(context.Context, string) (cubebox.ModelSettingsSnapshot, error)
	providerFn             func(context.Context, string, string, cubebox.UpsertModelProviderInput) (cubebox.ModelProvider, error)
	credentialFn           func(context.Context, string, string, cubebox.RotateModelCredentialInput) (cubebox.ModelCredential, error)
	deactivateCredentialFn func(context.Context, string, string) (cubebox.ModelCredential, error)
	selectionFn            func(context.Context, string, string, cubebox.SelectActiveModelInput) (cubebox.ActiveModelSelection, error)
	verifyFn               func(context.Context, string, string) (cubebox.ModelHealth, error)
	runtimeConfigFn        func(context.Context, string) (cubebox.ActiveModelRuntimeConfig, error)
}

func (s cubeboxStoreStub) CreateConversation(ctx context.Context, tenantID string, principalID string) (cubebox.ConversationReplayResponse, error) {
	return s.createFn(ctx, tenantID, principalID)
}

func (s cubeboxStoreStub) GetConversation(ctx context.Context, tenantID string, principalID string, conversationID string) (cubebox.ConversationReplayResponse, error) {
	return s.getFn(ctx, tenantID, principalID, conversationID)
}

func (s cubeboxStoreStub) ListConversations(ctx context.Context, tenantID string, principalID string, limit int32) (cubebox.ConversationListResponse, error) {
	return s.listFn(ctx, tenantID, principalID, limit)
}

func (s cubeboxStoreStub) RenameConversation(ctx context.Context, tenantID string, principalID string, conversationID string, title string) (cubebox.ConversationReplayResponse, error) {
	return s.renameFn(ctx, tenantID, principalID, conversationID, title)
}

func (s cubeboxStoreStub) ArchiveConversation(ctx context.Context, tenantID string, principalID string, conversationID string, archived bool) (cubebox.ConversationReplayResponse, error) {
	return s.archiveFn(ctx, tenantID, principalID, conversationID, archived)
}

func (s cubeboxStoreStub) CompactConversation(
	ctx context.Context,
	tenantID string,
	principalID string,
	conversationID string,
	canonicalContext cubebox.CanonicalContext,
	reason string,
) (cubebox.CompactConversationResponse, error) {
	if s.compactFn == nil {
		return cubebox.CompactConversationResponse{}, errors.New("unexpected")
	}
	return s.compactFn(ctx, tenantID, principalID, conversationID, canonicalContext, reason)
}

func (s cubeboxStoreStub) AppendEvent(ctx context.Context, tenantID string, principalID string, conversationID string, event cubebox.CanonicalEvent) error {
	if s.appendFn == nil {
		return nil
	}
	return s.appendFn(ctx, tenantID, principalID, conversationID, event)
}

func (s cubeboxStoreStub) AppendEvents(ctx context.Context, tenantID string, principalID string, conversationID string, events []cubebox.CanonicalEvent) error {
	for _, event := range events {
		if err := s.AppendEvent(ctx, tenantID, principalID, conversationID, event); err != nil {
			return err
		}
	}
	return nil
}

func (s cubeboxStoreStub) GetModelSettings(ctx context.Context, tenantID string) (cubebox.ModelSettingsSnapshot, error) {
	if s.settingsFn == nil {
		return cubebox.ModelSettingsSnapshot{}, errors.New("unexpected")
	}
	return s.settingsFn(ctx, tenantID)
}

func (s cubeboxStoreStub) UpsertModelProvider(ctx context.Context, tenantID string, principalID string, input cubebox.UpsertModelProviderInput) (cubebox.ModelProvider, error) {
	if s.providerFn == nil {
		return cubebox.ModelProvider{}, errors.New("unexpected")
	}
	return s.providerFn(ctx, tenantID, principalID, input)
}

func (s cubeboxStoreStub) RotateModelCredential(ctx context.Context, tenantID string, principalID string, input cubebox.RotateModelCredentialInput) (cubebox.ModelCredential, error) {
	if s.credentialFn == nil {
		return cubebox.ModelCredential{}, errors.New("unexpected")
	}
	return s.credentialFn(ctx, tenantID, principalID, input)
}

func (s cubeboxStoreStub) DeactivateCredential(ctx context.Context, tenantID string, credentialID string) (cubebox.ModelCredential, error) {
	if s.deactivateCredentialFn == nil {
		return cubebox.ModelCredential{}, errors.New("unexpected")
	}
	return s.deactivateCredentialFn(ctx, tenantID, credentialID)
}

func (s cubeboxStoreStub) SelectActiveModel(ctx context.Context, tenantID string, principalID string, input cubebox.SelectActiveModelInput) (cubebox.ActiveModelSelection, error) {
	if s.selectionFn == nil {
		return cubebox.ActiveModelSelection{}, errors.New("unexpected")
	}
	return s.selectionFn(ctx, tenantID, principalID, input)
}

func (s cubeboxStoreStub) VerifyActiveModel(ctx context.Context, tenantID string, principalID string) (cubebox.ModelHealth, error) {
	if s.verifyFn == nil {
		return cubebox.ModelHealth{}, errors.New("unexpected")
	}
	return s.verifyFn(ctx, tenantID, principalID)
}

func (s cubeboxStoreStub) GetActiveModelRuntimeConfig(ctx context.Context, tenantID string) (cubebox.ActiveModelRuntimeConfig, error) {
	if s.runtimeConfigFn == nil {
		return cubebox.ActiveModelRuntimeConfig{}, errors.New("unexpected")
	}
	return s.runtimeConfigFn(ctx, tenantID)
}

func TestCubeBoxCreateConversationAPI(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations", strings.NewReader(`{}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	handleCubeBoxCreateConversationAPI(rec, req, cubeboxStoreStub{
		createFn: func(_ context.Context, tenantID string, principalID string) (cubebox.ConversationReplayResponse, error) {
			if tenantID != "t1" || principalID != "p1" {
				t.Fatalf("tenant=%q principal=%q", tenantID, principalID)
			}
			return cubebox.ConversationReplayResponse{
				Conversation: cubebox.Conversation{ID: "conv_1", Title: "新对话", Status: "active"},
				Events:       []cubebox.CanonicalEvent{},
				NextSequence: 1,
			}, nil
		},
		getFn: func(context.Context, string, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		listFn: func(context.Context, string, string, int32) (cubebox.ConversationListResponse, error) {
			return cubebox.ConversationListResponse{}, errors.New("unexpected")
		},
		renameFn: func(context.Context, string, string, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		archiveFn: func(context.Context, string, string, string, bool) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"conversation"`) {
		t.Fatalf("unexpected body=%s", rec.Body.String())
	}
}

func TestCubeBoxCapabilitiesAPI(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/capabilities", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1", RoleSlug: "tenant-viewer"}))

	handleCubeBoxCapabilitiesAPI(rec, req, cubeboxAuthorizerStub{
		allowed: map[string]bool{
			"cubebox.conversations:read": true,
			"cubebox.conversations:use":  true,
		},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{
		`"conversation":{"read":true,"use":true}`,
		`"settings":{"read":false`,
		`"verify":false`,
		`"rotate":false`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("expected body to contain %s, got %s", needle, body)
		}
	}
}

func TestCubeBoxListConversationsAPI(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/conversations?limit=5", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	handleCubeBoxListConversationsAPI(rec, req, cubeboxStoreStub{
		createFn: func(context.Context, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		getFn: func(context.Context, string, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		listFn: func(_ context.Context, tenantID string, principalID string, limit int32) (cubebox.ConversationListResponse, error) {
			if tenantID != "t1" || principalID != "p1" || limit != 5 {
				t.Fatalf("tenant=%q principal=%q limit=%d", tenantID, principalID, limit)
			}
			return cubebox.ConversationListResponse{
				Items: []cubebox.ConversationListItem{{ID: "conv_1", Title: "a", Status: "active"}},
			}, nil
		},
		renameFn: func(context.Context, string, string, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		archiveFn: func(context.Context, string, string, string, bool) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
	})

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"items"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCubeBoxLoadConversationAPI(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/conversations/conv_1", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	handleCubeBoxLoadConversationAPI(rec, req, cubeboxStoreStub{
		createFn: func(context.Context, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		getFn: func(_ context.Context, tenantID string, principalID string, conversationID string) (cubebox.ConversationReplayResponse, error) {
			if tenantID != "t1" || principalID != "p1" || conversationID != "conv_1" {
				t.Fatalf("tenant=%q principal=%q conversation=%q", tenantID, principalID, conversationID)
			}
			return cubebox.ConversationReplayResponse{
				Conversation: cubebox.Conversation{ID: conversationID, Title: "a", Status: "active"},
				Events:       []cubebox.CanonicalEvent{},
				NextSequence: 1,
			}, nil
		},
		listFn: func(context.Context, string, string, int32) (cubebox.ConversationListResponse, error) {
			return cubebox.ConversationListResponse{}, errors.New("unexpected")
		},
		renameFn: func(context.Context, string, string, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		archiveFn: func(context.Context, string, string, string, bool) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
	})

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"conv_1"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCubeBoxStreamTurnAPI(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/turns:stream", strings.NewReader(`{"conversation_id":"conv_1","prompt":"hello","next_sequence":1}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	runtime := cubebox.NewRuntime()
	handleCubeBoxStreamTurnAPI(rec, req, runtime, cubeboxStoreStub{
		createFn: func(context.Context, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		getFn: func(context.Context, string, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		listFn: func(context.Context, string, string, int32) (cubebox.ConversationListResponse, error) {
			return cubebox.ConversationListResponse{}, errors.New("unexpected")
		},
		renameFn: func(context.Context, string, string, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		archiveFn: func(context.Context, string, string, string, bool) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		compactFn: func(context.Context, string, string, string, cubebox.CanonicalContext, string) (cubebox.CompactConversationResponse, error) {
			return cubebox.CompactConversationResponse{}, nil
		},
		appendFn: func(_ context.Context, tenantID string, principalID string, conversationID string, event cubebox.CanonicalEvent) error {
			if tenantID != "t1" || principalID != "p1" || conversationID != "conv_1" {
				t.Fatalf("tenant=%q principal=%q conversation=%q", tenantID, principalID, conversationID)
			}
			return nil
		},
	}, newTestGateway(runtime), nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"type":"turn.agent_message.delta"`) {
		t.Fatalf("missing delta event: %s", body)
	}
	if !strings.Contains(body, `"type":"turn.completed"`) {
		t.Fatalf("missing completed event: %s", body)
	}
	if !strings.Contains(body, `"trace_id":"`) {
		t.Fatalf("missing trace_id payload: %s", body)
	}
	if !strings.Contains(body, `"runtime":"deterministic-fixture"`) {
		t.Fatalf("missing runtime payload: %s", body)
	}
	if !strings.Contains(body, `"latency_ms":`) {
		t.Fatalf("missing latency payload: %s", body)
	}
}

func TestCubeBoxStreamTurnAPIPreservesPromptWhitespace(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/turns:stream", strings.NewReader("{\"conversation_id\":\"conv_1\",\"prompt\":\"\\n  hello  \\n\",\"next_sequence\":1}"))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	runtime := cubebox.NewRuntime()
	var gotPrompt string
	handleCubeBoxStreamTurnAPI(rec, req, runtime, cubeboxStoreStub{
		compactFn: func(context.Context, string, string, string, cubebox.CanonicalContext, string) (cubebox.CompactConversationResponse, error) {
			return cubebox.CompactConversationResponse{}, nil
		},
		appendFn: func(_ context.Context, _ string, _ string, _ string, event cubebox.CanonicalEvent) error {
			if event.Type == "turn.user_message.accepted" {
				gotPrompt = event.Payload["text"].(string)
			}
			return nil
		},
	}, newTestGateway(runtime), nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if gotPrompt != "\n  hello  \n" {
		t.Fatalf("unexpected prompt=%q", gotPrompt)
	}
	if !strings.Contains(rec.Body.String(), "\\n  hello  \\n") {
		t.Fatalf("expected response body to preserve prompt whitespace, body=%s", rec.Body.String())
	}
}

func TestCubeBoxInterruptTurnAPI(t *testing.T) {
	runtime := cubebox.NewRuntime()
	turn := runtime.StartTurn(cubebox.TurnOwner{
		TenantID:       "t1",
		PrincipalID:    "p1",
		ConversationID: "conv_1",
	}, "hello")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/turns/"+turn.TurnID+":interrupt?conversation_id=conv_1", strings.NewReader(`{"reason":"user_requested"}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	handleCubeBoxInterruptTurnAPI(rec, req, runtime)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"interrupted":true`) {
		t.Fatalf("unexpected body=%s", rec.Body.String())
	}
}

func TestCubeBoxInterruptTurnAPIRejectsWrongOwner(t *testing.T) {
	runtime := cubebox.NewRuntime()
	turn := runtime.StartTurn(cubebox.TurnOwner{
		TenantID:       "t1",
		PrincipalID:    "p1",
		ConversationID: "conv_1",
	}, "hello")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/turns/"+turn.TurnID+":interrupt?conversation_id=conv_2", strings.NewReader(`{"reason":"user_requested"}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p2"}))

	handleCubeBoxInterruptTurnAPI(rec, req, runtime)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"interrupted":false`) {
		t.Fatalf("unexpected body=%s", rec.Body.String())
	}
}

func TestCubeBoxInterruptTurnAPIRequiresConversationID(t *testing.T) {
	runtime := cubebox.NewRuntime()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/turns/turn_1:interrupt", strings.NewReader(`{"reason":"user_requested"}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	handleCubeBoxInterruptTurnAPI(rec, req, runtime)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCubeBoxStreamTurnAPIWritesFallbackErrorWhenAppendFails(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/turns:stream", strings.NewReader(`{"conversation_id":"conv_1","prompt":"hello","next_sequence":1}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))
	appendCount := 0

	runtime := cubebox.NewRuntime()
	handleCubeBoxStreamTurnAPI(rec, req, runtime, cubeboxStoreStub{
		createFn: func(context.Context, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		getFn: func(context.Context, string, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		listFn: func(context.Context, string, string, int32) (cubebox.ConversationListResponse, error) {
			return cubebox.ConversationListResponse{}, errors.New("unexpected")
		},
		renameFn: func(context.Context, string, string, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		archiveFn: func(context.Context, string, string, string, bool) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		compactFn: func(context.Context, string, string, string, cubebox.CanonicalContext, string) (cubebox.CompactConversationResponse, error) {
			return cubebox.CompactConversationResponse{}, nil
		},
		appendFn: func(_ context.Context, tenantID string, principalID string, conversationID string, event cubebox.CanonicalEvent) error {
			appendCount += 1
			if tenantID != "t1" || principalID != "p1" || conversationID != "conv_1" {
				t.Fatalf("tenant=%q principal=%q conversation=%q", tenantID, principalID, conversationID)
			}
			if appendCount >= 3 {
				return errors.New("boom")
			}
			return nil
		},
	}, newTestGateway(runtime), nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"type":"turn.error"`) {
		t.Fatalf("missing fallback error event: %s", body)
	}
	if !strings.Contains(body, `"code":"event_log_write_failed"`) {
		t.Fatalf("missing fallback error code: %s", body)
	}
	if !strings.Contains(body, `"trace_id":"`) {
		t.Fatalf("missing trace_id in fallback error: %s", body)
	}
}

func TestCubeBoxSettingsAPI(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/settings", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	handleCubeBoxSettingsAPI(rec, req, cubeboxStoreStub{
		settingsFn: func(_ context.Context, tenantID string) (cubebox.ModelSettingsSnapshot, error) {
			if tenantID != "t1" {
				t.Fatalf("tenant=%q", tenantID)
			}
			return cubebox.ModelSettingsSnapshot{
				Providers: []cubebox.ModelProvider{{ID: "openai-compatible", ProviderType: "openai-compatible", DisplayName: "Primary", BaseURL: "https://example.invalid/v1", Enabled: true}},
			}, nil
		},
	})

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"providers"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCubeBoxSettingsProvidersAPI(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/settings/providers", strings.NewReader(`{"provider_id":"openai-compatible","provider_type":"openai-compatible","display_name":"Primary","base_url":"https://example.invalid/v1","enabled":true}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	handleCubeBoxSettingsProvidersAPI(rec, req, cubeboxStoreStub{
		providerFn: func(_ context.Context, tenantID string, principalID string, input cubebox.UpsertModelProviderInput) (cubebox.ModelProvider, error) {
			if tenantID != "t1" || principalID != "p1" || input.ProviderID != "openai-compatible" {
				t.Fatalf("tenant=%q principal=%q input=%+v", tenantID, principalID, input)
			}
			return cubebox.ModelProvider{ID: input.ProviderID, ProviderType: input.ProviderType, DisplayName: input.DisplayName, BaseURL: input.BaseURL, Enabled: input.Enabled}, nil
		},
	})

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"openai-compatible"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCubeBoxSettingsVerifyAPI(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/settings/verify", strings.NewReader(`{}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	handleCubeBoxSettingsVerifyAPI(rec, req, cubeboxStoreStub{
		verifyFn: func(_ context.Context, tenantID string, principalID string) (cubebox.ModelHealth, error) {
			if tenantID != "t1" || principalID != "p1" {
				t.Fatalf("tenant=%q principal=%q", tenantID, principalID)
			}
			latency := 120
			return cubebox.ModelHealth{ID: "health_1", ProviderID: "openai-compatible", ModelSlug: "gpt-4.1", Status: "healthy", LatencyMS: &latency}, nil
		},
	})

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"healthy"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCubeBoxStreamTurnAPIUsesUUIDEventIDs(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/turns:stream", strings.NewReader(`{"conversation_id":"conv_1","prompt":"hello","next_sequence":48}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	var eventIDs []string
	runtime := cubebox.NewRuntime()
	handleCubeBoxStreamTurnAPI(rec, req, runtime, cubeboxStoreStub{
		createFn: func(context.Context, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		getFn: func(context.Context, string, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		listFn: func(context.Context, string, string, int32) (cubebox.ConversationListResponse, error) {
			return cubebox.ConversationListResponse{}, errors.New("unexpected")
		},
		renameFn: func(context.Context, string, string, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		archiveFn: func(context.Context, string, string, string, bool) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		compactFn: func(context.Context, string, string, string, cubebox.CanonicalContext, string) (cubebox.CompactConversationResponse, error) {
			return cubebox.CompactConversationResponse{}, nil
		},
		appendFn: func(_ context.Context, tenantID string, principalID string, conversationID string, event cubebox.CanonicalEvent) error {
			if tenantID != "t1" || principalID != "p1" || conversationID != "conv_1" {
				t.Fatalf("tenant=%q principal=%q conversation=%q", tenantID, principalID, conversationID)
			}
			eventIDs = append(eventIDs, event.EventID)
			return nil
		},
	}, newTestGateway(runtime), nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(eventIDs) == 0 {
		t.Fatal("expected stream events to be appended")
	}
	pattern := regexp.MustCompile(`^evt_[0-9a-f]{32}$`)
	seen := make(map[string]struct{}, len(eventIDs))
	for _, eventID := range eventIDs {
		if !pattern.MatchString(eventID) {
			t.Fatalf("event id %q does not use uuid-based format", eventID)
		}
		if _, ok := seen[eventID]; ok {
			t.Fatalf("duplicate event id %q", eventID)
		}
		seen[eventID] = struct{}{}
	}
}

func TestCubeBoxPatchConversationAPIRenamesConversation(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/internal/cubebox/conversations/conv_1", strings.NewReader(`{"title":"新标题"}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	handleCubeBoxPatchConversationAPI(rec, req, cubeboxStoreStub{
		createFn: func(context.Context, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		getFn: func(context.Context, string, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		listFn: func(context.Context, string, string, int32) (cubebox.ConversationListResponse, error) {
			return cubebox.ConversationListResponse{}, errors.New("unexpected")
		},
		renameFn: func(_ context.Context, tenantID string, principalID string, conversationID string, title string) (cubebox.ConversationReplayResponse, error) {
			if tenantID != "t1" || principalID != "p1" || conversationID != "conv_1" || title != "新标题" {
				t.Fatalf("unexpected args tenant=%s principal=%s conversation=%s title=%s", tenantID, principalID, conversationID, title)
			}
			return cubebox.ConversationReplayResponse{Conversation: cubebox.Conversation{ID: "conv_1", Title: "新标题", Status: "active"}}, nil
		},
		archiveFn: func(context.Context, string, string, string, bool) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "新标题") {
		t.Fatalf("unexpected body=%s", rec.Body.String())
	}
}

func TestCubeBoxPatchConversationAPIArchivesConversation(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/internal/cubebox/conversations/conv_1", strings.NewReader(`{"archived":true}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	handleCubeBoxPatchConversationAPI(rec, req, cubeboxStoreStub{
		createFn: func(context.Context, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		getFn: func(context.Context, string, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		listFn: func(context.Context, string, string, int32) (cubebox.ConversationListResponse, error) {
			return cubebox.ConversationListResponse{}, errors.New("unexpected")
		},
		renameFn: func(context.Context, string, string, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		archiveFn: func(_ context.Context, tenantID string, principalID string, conversationID string, archived bool) (cubebox.ConversationReplayResponse, error) {
			if tenantID != "t1" || principalID != "p1" || conversationID != "conv_1" || !archived {
				t.Fatalf("unexpected args tenant=%s principal=%s conversation=%s archived=%v", tenantID, principalID, conversationID, archived)
			}
			return cubebox.ConversationReplayResponse{Conversation: cubebox.Conversation{ID: "conv_1", Title: "新对话", Status: "archived", Archived: true}}, nil
		},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"archived":true`) {
		t.Fatalf("unexpected body=%s", rec.Body.String())
	}
}

func TestCubeBoxPatchConversationAPIUnarchivesConversation(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/internal/cubebox/conversations/conv_1", strings.NewReader(`{"archived":false}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	handleCubeBoxPatchConversationAPI(rec, req, cubeboxStoreStub{
		createFn: func(context.Context, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		getFn: func(context.Context, string, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		listFn: func(context.Context, string, string, int32) (cubebox.ConversationListResponse, error) {
			return cubebox.ConversationListResponse{}, errors.New("unexpected")
		},
		renameFn: func(context.Context, string, string, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		archiveFn: func(_ context.Context, tenantID string, principalID string, conversationID string, archived bool) (cubebox.ConversationReplayResponse, error) {
			if tenantID != "t1" || principalID != "p1" || conversationID != "conv_1" || archived {
				t.Fatalf("unexpected args tenant=%s principal=%s conversation=%s archived=%v", tenantID, principalID, conversationID, archived)
			}
			return cubebox.ConversationReplayResponse{Conversation: cubebox.Conversation{ID: "conv_1", Title: "新对话", Status: "active", Archived: false}}, nil
		},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"archived":false`) {
		t.Fatalf("unexpected body=%s", rec.Body.String())
	}
}

func TestCubeBoxCompactConversationAPI(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv_1:compact", strings.NewReader(`{"reason":"manual"}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	handleCubeBoxCompactConversationAPI(rec, req, cubeboxStoreStub{
		createFn: func(context.Context, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		getFn: func(context.Context, string, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		listFn: func(context.Context, string, string, int32) (cubebox.ConversationListResponse, error) {
			return cubebox.ConversationListResponse{}, errors.New("unexpected")
		},
		renameFn: func(context.Context, string, string, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		archiveFn: func(context.Context, string, string, string, bool) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		compactFn: func(_ context.Context, tenantID string, principalID string, conversationID string, canonicalContext cubebox.CanonicalContext, reason string) (cubebox.CompactConversationResponse, error) {
			if tenantID != "t1" || principalID != "p1" || conversationID != "conv_1" || reason != "manual" {
				t.Fatalf("unexpected compact args tenant=%s principal=%s conversation=%s reason=%s", tenantID, principalID, conversationID, reason)
			}
			if canonicalContext.TenantID != "t1" || canonicalContext.PrincipalID != "p1" {
				t.Fatalf("unexpected canonical context=%+v", canonicalContext)
			}
			if canonicalContext.Runtime != "unavailable" || canonicalContext.ModelSlug != "unavailable" {
				t.Fatalf("expected unavailable runtime metadata when config is unavailable, got %+v", canonicalContext)
			}
			event := cubebox.CanonicalEvent{
				EventID:        "evt_compact",
				ConversationID: "conv_1",
				Sequence:       5,
				Type:           "turn.context_compacted",
				Payload: map[string]any{
					"summary_id":   "summary_1",
					"source_range": []int{1, 4},
				},
			}
			return cubebox.CompactConversationResponse{
				Conversation: cubebox.Conversation{ID: "conv_1", Title: "新对话", Status: "active"},
				Event:        &event,
				PromptView:   []cubebox.PromptItem{{Role: "system", Content: "tenant=t1"}},
				NextSequence: 6,
			}, nil
		},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"turn.context_compacted"`) {
		t.Fatalf("unexpected body=%s", rec.Body.String())
	}
}

func TestCubeBoxCompactConversationAPIReturnsNoEventWhenCompactionIsSkipped(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv_1:compact", strings.NewReader(`{"reason":"manual"}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	handleCubeBoxCompactConversationAPI(rec, req, cubeboxStoreStub{
		createFn: func(context.Context, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		getFn: func(context.Context, string, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		listFn: func(context.Context, string, string, int32) (cubebox.ConversationListResponse, error) {
			return cubebox.ConversationListResponse{}, errors.New("unexpected")
		},
		renameFn: func(context.Context, string, string, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		archiveFn: func(context.Context, string, string, string, bool) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		compactFn: func(_ context.Context, tenantID string, principalID string, conversationID string, canonicalContext cubebox.CanonicalContext, reason string) (cubebox.CompactConversationResponse, error) {
			return cubebox.CompactConversationResponse{
				Conversation: cubebox.Conversation{ID: conversationID, Title: "新对话", Status: "active"},
				PromptView:   []cubebox.PromptItem{{Role: "system", Content: "tenant=t1"}},
				NextSequence: 3,
			}, nil
		},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"event":{`) {
		t.Fatalf("expected compact skip response without event, got %s", rec.Body.String())
	}
}

func TestCubeBoxLoadConversationAPIReturnsPhaseCLifecycleRoundtripGolden(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/conversations/conv_roundtrip", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	handleCubeBoxLoadConversationAPI(rec, req, cubeboxStoreStub{
		createFn: func(context.Context, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		getFn: func(_ context.Context, tenantID string, principalID string, conversationID string) (cubebox.ConversationReplayResponse, error) {
			if tenantID != "t1" || principalID != "p1" || conversationID != "conv_roundtrip" {
				t.Fatalf("unexpected args tenant=%s principal=%s conversation=%s", tenantID, principalID, conversationID)
			}
			return cubebox.ConversationReplayResponse{
				Conversation: cubebox.Conversation{ID: "conv_roundtrip", Title: "恢复后的活跃会话", Status: "active", Archived: false},
				Events: []cubebox.CanonicalEvent{
					{EventID: "evt_1", ConversationID: "conv_roundtrip", Sequence: 1, Type: "conversation.loaded", Payload: map[string]any{"title": "新对话", "status": "active", "archived": false}},
					{EventID: "evt_2", ConversationID: "conv_roundtrip", Sequence: 2, Type: "conversation.renamed", Payload: map[string]any{"title": "需求澄清", "status": "active", "archived": false}},
					{EventID: "evt_3", ConversationID: "conv_roundtrip", TurnID: ptr("turn_1"), Sequence: 3, Type: "turn.user_message.accepted", Payload: map[string]any{"message_id": "msg_user_1", "text": "请总结当前状态"}},
					{EventID: "evt_4", ConversationID: "conv_roundtrip", TurnID: ptr("turn_1"), Sequence: 4, Type: "turn.agent_message.delta", Payload: map[string]any{"message_id": "msg_agent_1", "delta": "当前已完成持久化，"}},
					{EventID: "evt_5", ConversationID: "conv_roundtrip", TurnID: ptr("turn_1"), Sequence: 5, Type: "turn.agent_message.delta", Payload: map[string]any{"message_id": "msg_agent_1", "delta": "正在进入封板收口。"}},
					{EventID: "evt_6", ConversationID: "conv_roundtrip", TurnID: ptr("turn_1"), Sequence: 6, Type: "turn.agent_message.completed", Payload: map[string]any{"message_id": "msg_agent_1"}},
					{EventID: "evt_7", ConversationID: "conv_roundtrip", Sequence: 7, Type: "conversation.unarchived", Payload: map[string]any{"title": "恢复后的活跃会话", "status": "active", "archived": false}},
				},
				NextSequence: 8,
			}, nil
		},
		listFn: func(context.Context, string, string, int32) (cubebox.ConversationListResponse, error) {
			return cubebox.ConversationListResponse{}, errors.New("unexpected")
		},
		renameFn: func(context.Context, string, string, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		archiveFn: func(context.Context, string, string, string, bool) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{
		`"id":"conv_roundtrip"`,
		`"title":"恢复后的活跃会话"`,
		`"type":"conversation.unarchived"`,
		`"message_id":"msg_user_1"`,
		`"delta":"当前已完成持久化，"`,
		`"next_sequence":8`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("expected body to contain %s, got %s", needle, body)
		}
	}
}

func TestCubeBoxStreamTurnAPIPreTurnAutoCompactUsesActorAndUpdatedSequence(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/turns:stream", strings.NewReader(`{"conversation_id":"conv_1","prompt":"hello","next_sequence":8}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-a"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "principal-a"}))

	var compactCalled bool
	var appended []cubebox.CanonicalEvent
	runtime := cubebox.NewRuntime()
	handleCubeBoxStreamTurnAPI(rec, req, runtime, cubeboxStoreStub{
		createFn: func(context.Context, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		getFn: func(context.Context, string, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		listFn: func(context.Context, string, string, int32) (cubebox.ConversationListResponse, error) {
			return cubebox.ConversationListResponse{}, errors.New("unexpected")
		},
		renameFn: func(context.Context, string, string, string, string) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		archiveFn: func(context.Context, string, string, string, bool) (cubebox.ConversationReplayResponse, error) {
			return cubebox.ConversationReplayResponse{}, errors.New("unexpected")
		},
		compactFn: func(_ context.Context, tenantID string, principalID string, conversationID string, canonicalContext cubebox.CanonicalContext, reason string) (cubebox.CompactConversationResponse, error) {
			compactCalled = true
			if tenantID != "tenant-a" || principalID != "principal-a" || conversationID != "conv_1" {
				t.Fatalf("unexpected compact actor tenant=%s principal=%s conversation=%s", tenantID, principalID, conversationID)
			}
			if canonicalContext.TenantID != "tenant-a" || canonicalContext.PrincipalID != "principal-a" {
				t.Fatalf("unexpected canonical context=%+v", canonicalContext)
			}
			if reason != "pre_turn_auto" {
				t.Fatalf("unexpected compact reason=%s", reason)
			}
			event := cubebox.CanonicalEvent{EventID: "evt_compact", ConversationID: "conv_1", Sequence: 8, Type: "turn.context_compacted"}
			return cubebox.CompactConversationResponse{
				Conversation: cubebox.Conversation{ID: "conv_1", Title: "新对话", Status: "active"},
				Event:        &event,
				NextSequence: 12,
			}, nil
		},
		appendFn: func(_ context.Context, tenantID string, principalID string, conversationID string, event cubebox.CanonicalEvent) error {
			if tenantID != "tenant-a" || principalID != "principal-a" || conversationID != "conv_1" {
				t.Fatalf("unexpected append actor tenant=%s principal=%s conversation=%s", tenantID, principalID, conversationID)
			}
			appended = append(appended, event)
			return nil
		},
	}, newTestGateway(runtime), nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !compactCalled {
		t.Fatal("expected pre-turn auto compact")
	}
	if len(appended) == 0 {
		t.Fatal("expected appended stream events")
	}
	if appended[0].Sequence != 12 {
		t.Fatalf("expected first stream event to continue from compact next sequence, got %d", appended[0].Sequence)
	}
}

func TestCubeBoxStreamTurnAPIUsesQueryFlowWhenHandled(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/turns:stream", strings.NewReader(`{"conversation_id":"conv_1","prompt":"查一下 1001 在 2026-04-23 的组织详情","next_sequence":1}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	registry, err := cubebox.NewExecutionRegistry(cubebox.RegisteredExecutor{
		APIKey:         "orgunit.details",
		RequiredParams: []string{"org_code", "as_of"},
		OptionalParams: []string{"include_disabled"},
		Executor: queryExecutorStub{
			validateParamsFn: func(raw map[string]any) (map[string]any, error) {
				return raw, nil
			},
			executeFn: func(_ context.Context, request cubebox.ExecuteRequest, params map[string]any) (cubebox.ExecuteResult, error) {
				if request.TenantID != "t1" || request.PrincipalID != "p1" || request.ConversationID != "conv_1" {
					t.Fatalf("unexpected request=%+v", request)
				}
				return cubebox.ExecuteResult{
					Payload: map[string]any{
						"org_unit": map[string]any{
							"name":            "总部",
							"parent_org_code": "",
							"manager_name":    "张三",
							"full_name_path":  "总部",
						},
					},
				}, nil
			},
		},
	})
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}
	queryFlow := &cubeboxQueryFlow{
		runtime:  cubebox.NewRuntime(),
		store:    cubeboxStoreStub{},
		registry: registry,
		producer: cubeboxReadPlanProducerStub{
			result: cubeboxReadPlanProductionResult{
				Handled: true,
				Plan: cubebox.ReadPlan{
					Intent:       "orgunit.details",
					Confidence:   0.9,
					ExplainFocus: []string{"组织基本信息"},
					Steps: []cubebox.ReadPlanStep{
						{
							ID:          "step-1",
							APIKey:      "orgunit.details",
							Params:      map[string]any{"org_code": "1001", "as_of": "2026-04-23", "include_disabled": false},
							ResultFocus: []string{"org_unit.name", "org_unit.manager_name", "org_unit.full_name_path"},
							DependsOn:   []string{},
						},
					},
				},
				ProviderID:   "openai-compatible",
				ProviderType: "openai-compatible",
				ModelSlug:    "gpt-5.2",
			},
		},
		narrator: cubeboxQueryNarratorStub{text: "总部当前有效。"},
		knowledgePacks: []cubebox.KnowledgePack{
			{Dir: "modules/orgunit/presentation/cubebox", Files: map[string]string{"CUBEBOX-SKILL.md": "x", "queries.md": "x", "apis.md": "x", "examples.md": "x"}},
		},
		now: func() time.Time { return time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC) },
	}

	handleCubeBoxStreamTurnAPI(rec, req, cubebox.NewRuntime(), cubeboxStoreStub{
		appendFn: func(_ context.Context, tenantID string, principalID string, conversationID string, event cubebox.CanonicalEvent) error {
			if tenantID != "t1" || principalID != "p1" || conversationID != "conv_1" {
				t.Fatalf("tenant=%q principal=%q conversation=%q", tenantID, principalID, conversationID)
			}
			return nil
		},
	}, newTestGateway(cubebox.NewRuntime()), queryFlow)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"runtime":"cubebox-query-read-plan"`) {
		t.Fatalf("expected query runtime, got %s", body)
	}
	if !strings.Contains(body, `总部当前有效。`) {
		t.Fatalf("expected narrated query answer, got %s", body)
	}
	if strings.Contains(body, `deterministic-fixture`) {
		t.Fatalf("expected not to fallback to gateway fixture, got %s", body)
	}
}

func TestCubeBoxStreamTurnAPIUsesQueryFlowWhenHandledForOrgUnitList(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/turns:stream", strings.NewReader(`{"conversation_id":"conv_1","prompt":"请列出全部组织，今天为准，根据你认为合适的格式","next_sequence":1}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	registry, err := cubebox.NewExecutionRegistry(cubebox.RegisteredExecutor{
		APIKey:         "orgunit.list",
		RequiredParams: []string{"as_of"},
		OptionalParams: []string{"include_disabled"},
		Executor: queryExecutorStub{
			validateParamsFn: func(raw map[string]any) (map[string]any, error) {
				return raw, nil
			},
			executeFn: func(_ context.Context, request cubebox.ExecuteRequest, params map[string]any) (cubebox.ExecuteResult, error) {
				if request.TenantID != "t1" || request.PrincipalID != "p1" || request.ConversationID != "conv_1" {
					t.Fatalf("unexpected request=%+v", request)
				}
				return cubebox.ExecuteResult{
					Payload: map[string]any{
						"as_of":            "2026-04-23",
						"include_disabled": false,
						"org_units": []orgUnitListItem{
							{
								OrgCode:        "1001",
								Name:           "总部",
								Status:         "active",
								IsBusinessUnit: ptrBool(true),
								HasChildren:    ptrBool(true),
							},
							{
								OrgCode:        "1002",
								Name:           "华东事业部",
								Status:         "active",
								IsBusinessUnit: ptrBool(false),
								HasChildren:    ptrBool(false),
							},
						},
					},
				}, nil
			},
		},
	})
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}
	queryFlow := &cubeboxQueryFlow{
		runtime:  cubebox.NewRuntime(),
		store:    cubeboxStoreStub{},
		registry: registry,
		producer: cubeboxReadPlanProducerStub{
			result: cubeboxReadPlanProductionResult{
				Handled: true,
				Plan: cubebox.ReadPlan{
					Intent:       "orgunit.list",
					Confidence:   0.9,
					ExplainFocus: []string{"按 2026-04-23 的组织列表（默认不含停用组织）", "输出字段：组织编码、名称、状态、是否业务单元、是否有下级"},
					Steps: []cubebox.ReadPlanStep{
						{
							ID:          "step-1",
							APIKey:      "orgunit.list",
							Params:      map[string]any{"as_of": "2026-04-23", "include_disabled": false},
							ResultFocus: []string{"as_of", "include_disabled"},
							DependsOn:   []string{},
						},
					},
				},
				ProviderID:   "openai-compatible",
				ProviderType: "openai-compatible",
				ModelSlug:    "gpt-5.2",
			},
		},
		narrator: cubeboxQueryNarratorStub{text: "已找到 2 条组织记录。"},
		knowledgePacks: []cubebox.KnowledgePack{
			{Dir: "modules/orgunit/presentation/cubebox", Files: map[string]string{"CUBEBOX-SKILL.md": "x", "queries.md": "x", "apis.md": "x", "examples.md": "x"}},
		},
		now: func() time.Time { return time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC) },
	}

	handleCubeBoxStreamTurnAPI(rec, req, cubebox.NewRuntime(), cubeboxStoreStub{
		appendFn: func(_ context.Context, tenantID string, principalID string, conversationID string, event cubebox.CanonicalEvent) error {
			if tenantID != "t1" || principalID != "p1" || conversationID != "conv_1" {
				t.Fatalf("tenant=%q principal=%q conversation=%q", tenantID, principalID, conversationID)
			}
			return nil
		},
	}, newTestGateway(cubebox.NewRuntime()), queryFlow)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, expected := range []string{
		`"runtime":"cubebox-query-read-plan"`,
		`已找到 2 条组织记录。`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected %q in body=%s", expected, body)
		}
	}
}

func TestCubeBoxStreamTurnAPIPromotesOrgTreeClarificationToDefaultRootList(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/turns:stream", strings.NewReader(`{"conversation_id":"conv_1","prompt":"查询组织树","next_sequence":1}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	var sawParams map[string]any
	registry, err := cubebox.NewExecutionRegistry(cubebox.RegisteredExecutor{
		APIKey:         "orgunit.list",
		RequiredParams: []string{"as_of"},
		OptionalParams: []string{"include_disabled", "parent_org_code"},
		Executor: queryExecutorStub{
			validateParamsFn: func(raw map[string]any) (map[string]any, error) {
				sawParams = raw
				return raw, nil
			},
			executeFn: func(_ context.Context, request cubebox.ExecuteRequest, params map[string]any) (cubebox.ExecuteResult, error) {
				if request.TenantID != "t1" || request.PrincipalID != "p1" || request.ConversationID != "conv_1" {
					t.Fatalf("unexpected request=%+v", request)
				}
				return cubebox.ExecuteResult{
					Payload: map[string]any{
						"as_of":            "2026-04-23",
						"include_disabled": false,
						"org_units": []orgUnitListItem{
							{
								OrgCode:        "1001",
								Name:           "总部",
								Status:         "active",
								IsBusinessUnit: ptrBool(true),
								HasChildren:    ptrBool(true),
							},
						},
					},
				}, nil
			},
		},
	})
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}
	queryFlow := &cubeboxQueryFlow{
		runtime:  cubebox.NewRuntime(),
		store:    cubeboxStoreStub{},
		registry: registry,
		producer: cubeboxReadPlanProducerStub{
			result: cubeboxReadPlanProductionResult{
				Handled: true,
				Plan: cubebox.ReadPlan{
					Intent:             "orgunit.list",
					Confidence:         0.41,
					MissingParams:      []string{"as_of", "parent_org_code"},
					ClarifyingQuestion: "请告诉我要按哪一天查询组织树（格式例如 2026-04-23）。另外你想看全租户的一级组织，还是某个上级组织（请提供 parent_org_code）下面的子组织？",
				},
				ProviderID:   "openai-compatible",
				ProviderType: "openai-compatible",
				ModelSlug:    "gpt-5.2",
			},
		},
		narrator: cubeboxQueryNarratorStub{fn: func(context.Context, cubeboxQueryNarrationInput) (string, error) {
			t.Fatal("narrator should not be called when clarification is required")
			return "", nil
		}},
		knowledgePacks: []cubebox.KnowledgePack{
			{Dir: "modules/orgunit/presentation/cubebox", Files: map[string]string{"CUBEBOX-SKILL.md": "x", "queries.md": "x", "apis.md": "x", "examples.md": "x"}},
		},
		now: func() time.Time { return time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC) },
	}

	handleCubeBoxStreamTurnAPI(rec, req, cubebox.NewRuntime(), cubeboxStoreStub{
		appendFn: func(_ context.Context, tenantID string, principalID string, conversationID string, event cubebox.CanonicalEvent) error {
			if tenantID != "t1" || principalID != "p1" || conversationID != "conv_1" {
				t.Fatalf("tenant=%q principal=%q conversation=%q", tenantID, principalID, conversationID)
			}
			return nil
		},
	}, newTestGateway(cubebox.NewRuntime()), queryFlow)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if sawParams != nil {
		t.Fatalf("expected clarification plan not to execute, got params=%+v", sawParams)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `请告诉我要按哪一天查询组织树`) {
		t.Fatalf("expected raw clarification text, got %s", body)
	}
	if strings.Contains(body, `组织列表：`) {
		t.Fatalf("expected no executed list summary, got %s", body)
	}
}

func TestCubeBoxStreamTurnAPIDoesNotDowngradeChildrenQueryToRootList(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/turns:stream", strings.NewReader(`{"conversation_id":"conv_1","prompt":"看华东事业部下面的子组织","next_sequence":1}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	called := false
	registry, err := cubebox.NewExecutionRegistry(cubebox.RegisteredExecutor{
		APIKey:         "orgunit.list",
		RequiredParams: []string{"as_of"},
		OptionalParams: []string{"include_disabled", "parent_org_code"},
		Executor: queryExecutorStub{
			validateParamsFn: func(raw map[string]any) (map[string]any, error) {
				called = true
				return raw, nil
			},
			executeFn: func(_ context.Context, _ cubebox.ExecuteRequest, _ map[string]any) (cubebox.ExecuteResult, error) {
				called = true
				return cubebox.ExecuteResult{}, nil
			},
		},
	})
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}
	queryFlow := &cubeboxQueryFlow{
		runtime:  cubebox.NewRuntime(),
		store:    cubeboxStoreStub{},
		registry: registry,
		producer: cubeboxReadPlanProducerStub{
			result: cubeboxReadPlanProductionResult{
				Handled: true,
				Plan: cubebox.ReadPlan{
					Intent:             "orgunit.list",
					Confidence:         0.41,
					MissingParams:      []string{"as_of", "parent_org_code"},
					ClarifyingQuestion: "请提供 parent_org_code。",
				},
				ProviderID:   "openai-compatible",
				ProviderType: "openai-compatible",
				ModelSlug:    "gpt-5.2",
			},
		},
		narrator: cubeboxQueryNarratorStub{fn: func(context.Context, cubeboxQueryNarrationInput) (string, error) {
			t.Fatal("narrator should not be called when clarification is required")
			return "", nil
		}},
		knowledgePacks: []cubebox.KnowledgePack{
			{Dir: "modules/orgunit/presentation/cubebox", Files: map[string]string{"CUBEBOX-SKILL.md": "x", "queries.md": "x", "apis.md": "x", "examples.md": "x"}},
		},
		now: func() time.Time { return time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC) },
	}

	handleCubeBoxStreamTurnAPI(rec, req, cubebox.NewRuntime(), cubeboxStoreStub{
		appendFn: func(_ context.Context, tenantID string, principalID string, conversationID string, event cubebox.CanonicalEvent) error {
			if tenantID != "t1" || principalID != "p1" || conversationID != "conv_1" {
				t.Fatalf("tenant=%q principal=%q conversation=%q", tenantID, principalID, conversationID)
			}
			return nil
		},
	}, newTestGateway(cubebox.NewRuntime()), queryFlow)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("expected children query clarification not to execute orgunit.list")
	}
	body := rec.Body.String()
	if !strings.Contains(body, "请提供 parent_org_code。") {
		t.Fatalf("expected raw clarification, got %s", body)
	}
	if strings.Contains(body, "组织列表：") {
		t.Fatalf("expected no root list downgrade, got %s", body)
	}
}

func TestCubeBoxStreamTurnAPILimitsLargeQueryPayloadInDelta(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/turns:stream", strings.NewReader(`{"conversation_id":"conv_1","prompt":"查大结果","next_sequence":1}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	registry, err := cubebox.NewExecutionRegistry(cubebox.RegisteredExecutor{
		APIKey: "orgunit.details",
		Executor: queryExecutorStub{
			validateParamsFn: func(raw map[string]any) (map[string]any, error) { return raw, nil },
			executeFn: func(context.Context, cubebox.ExecuteRequest, map[string]any) (cubebox.ExecuteResult, error) {
				return cubebox.ExecuteResult{
					Payload: map[string]any{
						"as_of":        "2026-04-23",
						"total":        1,
						"large_object": strings.Repeat("raw-payload-should-not-appear", 80),
					},
				}, nil
			},
		},
	})
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}
	queryFlow := &cubeboxQueryFlow{
		runtime: cubebox.NewRuntime(),
		store: cubeboxStoreStub{
			appendFn: func(context.Context, string, string, string, cubebox.CanonicalEvent) error { return nil },
		},
		registry: registry,
		producer: cubeboxReadPlanProducerStub{result: cubeboxReadPlanProductionResult{
			Handled: true,
			Plan: cubebox.ReadPlan{
				Intent:     "orgunit.details",
				Confidence: 0.9,
				Steps: []cubebox.ReadPlanStep{
					{
						ID:          "step-1",
						APIKey:      "orgunit.details",
						Params:      map[string]any{"org_code": "1001", "as_of": "2026-04-23"},
						ResultFocus: []string{"org_unit.name"},
						DependsOn:   []string{},
					},
				},
			},
		}},
		narrator: cubeboxQueryNarratorStub{text: "结果过大，请进入业务页面查看完整明细。"},
		knowledgePacks: []cubebox.KnowledgePack{
			{Dir: "modules/orgunit/presentation/cubebox", Files: map[string]string{"CUBEBOX-SKILL.md": "x", "queries.md": "x", "apis.md": "x", "examples.md": "x"}},
		},
		now: func() time.Time { return time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC) },
	}

	handleCubeBoxStreamTurnAPI(rec, req, cubebox.NewRuntime(), cubeboxStoreStub{
		appendFn: func(context.Context, string, string, string, cubebox.CanonicalEvent) error { return nil },
	}, newTestGateway(cubebox.NewRuntime()), queryFlow)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, expected := range []string{`"runtime":"cubebox-query-read-plan"`, `结果过大，请进入业务页面查看完整明细。`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected %q in body=%s", expected, body)
		}
	}
	if strings.Contains(body, "raw-payload-should-not-appear") || strings.Contains(body, "large_object") {
		t.Fatalf("expected large raw payload to be omitted body=%s", body)
	}
}

func TestCubeBoxStreamTurnAPIFallsBackToGatewayWhenQueryFlowSkips(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/turns:stream", strings.NewReader(`{"conversation_id":"conv_1","prompt":"hello","next_sequence":1}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	queryFlow := &cubeboxQueryFlow{
		runtime:  cubebox.NewRuntime(),
		store:    cubeboxStoreStub{},
		registry: &cubebox.ExecutionRegistry{},
		producer: cubeboxReadPlanProducerStub{result: cubeboxReadPlanProductionResult{Handled: false}},
		narrator: cubeboxQueryNarratorStub{text: "unused"},
		knowledgePacks: []cubebox.KnowledgePack{
			{Dir: "modules/orgunit/presentation/cubebox", Files: map[string]string{"CUBEBOX-SKILL.md": "x", "queries.md": "x", "apis.md": "x", "examples.md": "x"}},
		},
	}

	handleCubeBoxStreamTurnAPI(rec, req, cubebox.NewRuntime(), cubeboxStoreStub{
		compactFn: func(context.Context, string, string, string, cubebox.CanonicalContext, string) (cubebox.CompactConversationResponse, error) {
			return cubebox.CompactConversationResponse{}, nil
		},
		appendFn: func(context.Context, string, string, string, cubebox.CanonicalEvent) error {
			return nil
		},
	}, newTestGateway(cubebox.NewRuntime()), queryFlow)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"runtime":"deterministic-fixture"`) {
		t.Fatalf("expected gateway fallback body=%s", body)
	}
}

func TestCubeBoxStreamTurnAPIWritesQueryErrorWhenQueryExecutionFails(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/turns:stream", strings.NewReader(`{"conversation_id":"conv_1","prompt":"查组织","next_sequence":1}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	registry, err := cubebox.NewExecutionRegistry(cubebox.RegisteredExecutor{
		APIKey:         "orgunit.details",
		RequiredParams: []string{"org_code", "as_of"},
		Executor: queryExecutorStub{
			validateParamsFn: func(raw map[string]any) (map[string]any, error) { return raw, nil },
			executeFn: func(context.Context, cubebox.ExecuteRequest, map[string]any) (cubebox.ExecuteResult, error) {
				return cubebox.ExecuteResult{}, newBadRequestError("org_code invalid")
			},
		},
	})
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}

	queryFlow := &cubeboxQueryFlow{
		runtime: cubebox.NewRuntime(),
		store: cubeboxStoreStub{
			appendFn: func(context.Context, string, string, string, cubebox.CanonicalEvent) error { return nil },
		},
		registry: registry,
		producer: cubeboxReadPlanProducerStub{result: cubeboxReadPlanProductionResult{
			Handled: true,
			Plan: cubebox.ReadPlan{
				Intent:     "orgunit.details",
				Confidence: 0.9,
				Steps: []cubebox.ReadPlanStep{
					{
						ID:          "step-1",
						APIKey:      "orgunit.details",
						Params:      map[string]any{"org_code": "bad", "as_of": "2026-04-23"},
						ResultFocus: []string{"org_unit.name"},
						DependsOn:   []string{},
					},
				},
			},
		}},
		narrator: cubeboxQueryNarratorStub{text: "unused"},
		knowledgePacks: []cubebox.KnowledgePack{
			{Dir: "modules/orgunit/presentation/cubebox", Files: map[string]string{"CUBEBOX-SKILL.md": "x", "queries.md": "x", "apis.md": "x", "examples.md": "x"}},
		},
		now: func() time.Time { return time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC) },
	}

	handleCubeBoxStreamTurnAPI(rec, req, cubebox.NewRuntime(), cubeboxStoreStub{
		appendFn: func(context.Context, string, string, string, cubebox.CanonicalEvent) error { return nil },
	}, newTestGateway(cubebox.NewRuntime()), queryFlow)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"code":"invalid_request"`) {
		t.Fatalf("expected query execution invalid_request body=%s", body)
	}
	if !strings.Contains(body, `"type":"turn.completed"`) {
		t.Fatalf("expected terminal completed event body=%s", body)
	}
}

func TestCubeBoxStreamTurnAPIWritesNotFoundWhenQueryExecutionHasNoResult(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/turns:stream", strings.NewReader(`{"conversation_id":"conv_1","prompt":"查总部","next_sequence":1}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	registry, err := cubebox.NewExecutionRegistry(cubebox.RegisteredExecutor{
		APIKey:         "orgunit.search",
		RequiredParams: []string{"query", "as_of"},
		Executor: queryExecutorStub{
			validateParamsFn: func(raw map[string]any) (map[string]any, error) { return raw, nil },
			executeFn: func(context.Context, cubebox.ExecuteRequest, map[string]any) (cubebox.ExecuteResult, error) {
				return cubebox.ExecuteResult{}, errOrgUnitNotFound
			},
		},
	})
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}

	queryFlow := &cubeboxQueryFlow{
		runtime: cubebox.NewRuntime(),
		store: cubeboxStoreStub{
			appendFn: func(context.Context, string, string, string, cubebox.CanonicalEvent) error { return nil },
		},
		registry: registry,
		producer: cubeboxReadPlanProducerStub{result: cubeboxReadPlanProductionResult{
			Handled: true,
			Plan: cubebox.ReadPlan{
				Intent:     "orgunit.search",
				Confidence: 0.9,
				Steps: []cubebox.ReadPlanStep{
					{
						ID:          "step-1",
						APIKey:      "orgunit.search",
						Params:      map[string]any{"query": "总部", "as_of": "2026-04-23"},
						ResultFocus: []string{"target_org_code"},
						DependsOn:   []string{},
					},
				},
			},
		}},
		narrator: cubeboxQueryNarratorStub{text: "unused"},
		knowledgePacks: []cubebox.KnowledgePack{
			{Dir: "modules/orgunit/presentation/cubebox", Files: map[string]string{"CUBEBOX-SKILL.md": "x", "queries.md": "x", "apis.md": "x", "examples.md": "x"}},
		},
		now: func() time.Time { return time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC) },
	}

	handleCubeBoxStreamTurnAPI(rec, req, cubebox.NewRuntime(), cubeboxStoreStub{
		appendFn: func(context.Context, string, string, string, cubebox.CanonicalEvent) error { return nil },
	}, newTestGateway(cubebox.NewRuntime()), queryFlow)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"code":"orgunit_not_found"`) {
		t.Fatalf("expected orgunit_not_found body=%s", body)
	}
	if !strings.Contains(body, `未找到符合条件的组织，请调整关键词或提供组织编码。`) {
		t.Fatalf("expected explicit not found message body=%s", body)
	}
}

func TestCubeBoxStreamTurnAPIWritesCatalogDriftWhenExecutorMissing(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/turns:stream", strings.NewReader(`{"conversation_id":"conv_1","prompt":"查组织","next_sequence":1}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	registry, err := cubebox.NewExecutionRegistry(cubebox.RegisteredExecutor{
		APIKey:   "orgunit.search",
		Executor: queryExecutorStub{},
	})
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}
	queryFlow := &cubeboxQueryFlow{
		runtime: cubebox.NewRuntime(),
		store: cubeboxStoreStub{
			appendFn: func(context.Context, string, string, string, cubebox.CanonicalEvent) error { return nil },
		},
		registry: registry,
		producer: cubeboxReadPlanProducerStub{result: cubeboxReadPlanProductionResult{
			Handled: true,
			Plan: cubebox.ReadPlan{
				Intent:     "orgunit.details",
				Confidence: 0.9,
				Steps: []cubebox.ReadPlanStep{
					{
						ID:          "step-1",
						APIKey:      "orgunit.details",
						Params:      map[string]any{"org_code": "1001", "as_of": "2026-04-23"},
						ResultFocus: []string{"org_unit.name"},
						DependsOn:   []string{},
					},
				},
			},
		}},
		narrator: cubeboxQueryNarratorStub{text: "unused"},
		knowledgePacks: []cubebox.KnowledgePack{
			{Dir: "modules/orgunit/presentation/cubebox", Files: map[string]string{"CUBEBOX-SKILL.md": "x", "queries.md": "x", "apis.md": "x", "examples.md": "x"}},
		},
		now: func() time.Time { return time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC) },
	}

	handleCubeBoxStreamTurnAPI(rec, req, cubebox.NewRuntime(), cubeboxStoreStub{
		appendFn: func(context.Context, string, string, string, cubebox.CanonicalEvent) error { return nil },
	}, newTestGateway(cubebox.NewRuntime()), queryFlow)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"code":"api_catalog_drift_or_executor_missing"`) {
		t.Fatalf("expected catalog drift error body=%s", body)
	}
	if !strings.Contains(body, `查询执行目录与系统注册表不一致，请稍后重试或联系管理员。`) {
		t.Fatalf("expected explicit catalog drift message body=%s", body)
	}
}

func TestCubeBoxStreamTurnAPIFallsBackToGatewayWhenPlannerErrors(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/turns:stream", strings.NewReader(`{"conversation_id":"conv_1","prompt":"普通聊天","next_sequence":1}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	queryFlow := &cubeboxQueryFlow{
		runtime: cubebox.NewRuntime(),
		store: cubeboxStoreStub{
			appendFn: func(context.Context, string, string, string, cubebox.CanonicalEvent) error { return nil },
		},
		registry: &cubebox.ExecutionRegistry{},
		producer: cubeboxReadPlanProducerStub{err: cubebox.ErrProviderUnavailable},
		narrator: cubeboxQueryNarratorStub{text: "unused"},
		knowledgePacks: []cubebox.KnowledgePack{
			{Dir: "modules/orgunit/presentation/cubebox", Files: map[string]string{"CUBEBOX-SKILL.md": "x", "queries.md": "x", "apis.md": "x", "examples.md": "x"}},
		},
	}

	handleCubeBoxStreamTurnAPI(rec, req, cubebox.NewRuntime(), cubeboxStoreStub{
		compactFn: func(context.Context, string, string, string, cubebox.CanonicalContext, string) (cubebox.CompactConversationResponse, error) {
			return cubebox.CompactConversationResponse{}, nil
		},
		appendFn: func(context.Context, string, string, string, cubebox.CanonicalEvent) error { return nil },
	}, newTestGateway(cubebox.NewRuntime()), queryFlow)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"runtime":"deterministic-fixture"`) {
		t.Fatalf("expected gateway fallback when planner errors body=%s", body)
	}
	if strings.Contains(body, `"code":"ai_model_provider_unavailable"`) {
		t.Fatalf("planner error should not hijack non-query turns body=%s", body)
	}
}

type queryExecutorStub struct {
	validateParamsFn func(map[string]any) (map[string]any, error)
	executeFn        func(context.Context, cubebox.ExecuteRequest, map[string]any) (cubebox.ExecuteResult, error)
}

func ptrBool(value bool) *bool {
	return &value
}

func (s queryExecutorStub) ValidateParams(raw map[string]any) (map[string]any, error) {
	if s.validateParamsFn == nil {
		return raw, nil
	}
	return s.validateParamsFn(raw)
}

func (s queryExecutorStub) Execute(ctx context.Context, request cubebox.ExecuteRequest, params map[string]any) (cubebox.ExecuteResult, error) {
	if s.executeFn == nil {
		return cubebox.ExecuteResult{}, nil
	}
	return s.executeFn(ctx, request, params)
}
