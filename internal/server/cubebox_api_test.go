package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox"
)

func newTestGateway(runtime *cubebox.Runtime) *cubebox.GatewayService {
	return cubebox.NewGatewayService(runtime, nil, nil, nil)
}

type cubeboxAPIPlanProducerStub struct {
	result  cubeboxAPIPlanProductionResult
	results []cubeboxAPIPlanProductionResult
	err     error
	fn      func(context.Context, cubeboxAPIPlanProductionInput) (cubeboxAPIPlanProductionResult, error)
	index   *int
}

func (s cubeboxAPIPlanProducerStub) ProduceAPIPlan(ctx context.Context, input cubeboxAPIPlanProductionInput) (cubeboxAPIPlanProductionResult, error) {
	if s.fn != nil {
		return s.fn(ctx, input)
	}
	if s.err != nil {
		return cubeboxAPIPlanProductionResult{}, s.err
	}
	if len(s.results) > 0 {
		if s.index == nil {
			return s.results[0], nil
		}
		if *s.index >= len(s.results) {
			return s.results[len(s.results)-1], nil
		}
		result := s.results[*s.index]
		*s.index = *s.index + 1
		return result, nil
	}
	return s.result, nil
}

type cubeboxQueryNarratorStub struct {
	text      string
	err       error
	fn        func(context.Context, cubeboxQueryNarrationInput) (string, error)
	noQueryFn func(context.Context, cubeboxNoQueryGuidanceInput) (string, error)
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

func (s cubeboxQueryNarratorStub) NarrateNoQueryGuidance(ctx context.Context, input cubeboxNoQueryGuidanceInput) (string, error) {
	if s.noQueryFn != nil {
		return s.noQueryFn(ctx, input)
	}
	return fallbackNoQueryGuidanceText(buildNoQueryGuidanceEnvelope(input)), nil
}

type cubeboxAuthorizerStub struct {
	allowed map[string]bool
	err     error
}

func (s cubeboxAuthorizerStub) CapabilitiesForPrincipal(context.Context, string, string) ([]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make([]string, 0, len(s.allowed))
	for key, allowed := range s.allowed {
		if allowed {
			out = append(out, key)
		}
	}
	return out, nil
}

func (s cubeboxAuthorizerStub) AuthorizePrincipal(context.Context, string, string, string, string) (bool, error) {
	return false, nil
}

func (s cubeboxAuthorizerStub) OrgScopesForPrincipal(context.Context, string, string, string) ([]principalOrgScope, error) {
	return nil, nil
}

func (s cubeboxAuthorizerStub) ListRoleDefinitions(context.Context, string) ([]authzRoleDefinition, error) {
	return nil, nil
}

func (s cubeboxAuthorizerStub) GetRoleDefinition(context.Context, string, string) (authzRoleDefinition, bool, error) {
	return authzRoleDefinition{}, false, nil
}

func (s cubeboxAuthorizerStub) CreateRoleDefinition(context.Context, string, saveAuthzRoleDefinitionInput) (authzRoleDefinition, error) {
	return authzRoleDefinition{}, nil
}

func (s cubeboxAuthorizerStub) UpdateRoleDefinition(context.Context, string, string, saveAuthzRoleDefinitionInput) (authzRoleDefinition, error) {
	return authzRoleDefinition{}, nil
}

func (s cubeboxAuthorizerStub) GetPrincipalAssignment(context.Context, string, string) (principalAuthzAssignment, bool, error) {
	return principalAuthzAssignment{}, false, nil
}

func (s cubeboxAuthorizerStub) ReplacePrincipalAssignment(context.Context, string, string, replacePrincipalAssignmentInput) (principalAuthzAssignment, error) {
	return principalAuthzAssignment{}, nil
}

type cubeboxStoreStub struct {
	createFn               func(context.Context, string, string) (cubebox.ConversationReplayResponse, error)
	getFn                  func(context.Context, string, string, string) (cubebox.ConversationReplayResponse, error)
	listFn                 func(context.Context, string, string, int32) (cubebox.ConversationListResponse, error)
	renameFn               func(context.Context, string, string, string, string) (cubebox.ConversationReplayResponse, error)
	archiveFn              func(context.Context, string, string, string, bool) (cubebox.ConversationReplayResponse, error)
	preparePromptViewFn    func(context.Context, string, string, string, cubebox.CanonicalContext, string) (cubebox.PromptViewPreparationResponse, error)
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
	if s.getFn == nil {
		return cubebox.ConversationReplayResponse{
			Conversation: cubebox.Conversation{ID: strings.TrimSpace(conversationID), Title: "新对话", Status: "active"},
		}, nil
	}
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

func (s cubeboxStoreStub) PrepareConversationPromptView(
	ctx context.Context,
	tenantID string,
	principalID string,
	conversationID string,
	canonicalContext cubebox.CanonicalContext,
	reason string,
) (cubebox.PromptViewPreparationResponse, error) {
	if s.preparePromptViewFn == nil {
		return cubebox.PromptViewPreparationResponse{}, errors.New("unexpected")
	}
	return s.preparePromptViewFn(ctx, tenantID, principalID, conversationID, canonicalContext, reason)
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
		preparePromptViewFn: func(context.Context, string, string, string, cubebox.CanonicalContext, string) (cubebox.PromptViewPreparationResponse, error) {
			return cubebox.PromptViewPreparationResponse{}, nil
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

func TestCubeBoxStreamTurnAPIUsesPreparedSequenceForStableMessageIDs(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/turns:stream", strings.NewReader(`{"conversation_id":"conv_1","prompt":"hello after restart","next_sequence":2}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	runtime := cubebox.NewRuntime()
	var appended []cubebox.CanonicalEvent
	handleCubeBoxStreamTurnAPI(rec, req, runtime, cubeboxStoreStub{
		preparePromptViewFn: func(context.Context, string, string, string, cubebox.CanonicalContext, string) (cubebox.PromptViewPreparationResponse, error) {
			return cubebox.PromptViewPreparationResponse{
				Conversation: cubebox.Conversation{ID: "conv_1", Title: "新对话", Status: "active"},
				NextSequence: 10,
			}, nil
		},
		appendFn: func(_ context.Context, _ string, _ string, _ string, event cubebox.CanonicalEvent) error {
			appended = append(appended, event)
			return nil
		},
	}, newTestGateway(runtime), nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(appended) == 0 {
		t.Fatal("expected appended events")
	}
	for _, event := range appended {
		if event.TurnID == nil || *event.TurnID != "turn_seq_10" {
			t.Fatalf("event turn id should derive from prepared sequence, got %#v", event.TurnID)
		}
		if event.Sequence < 10 {
			t.Fatalf("event sequence=%d should start from prepared sequence", event.Sequence)
		}
		switch event.Type {
		case "turn.started":
			if event.Payload["user_message_id"] != "msg_user_seq_10" {
				t.Fatalf("started payload=%#v", event.Payload)
			}
		case "turn.user_message.accepted":
			if event.Payload["message_id"] != "msg_user_seq_10" {
				t.Fatalf("user payload=%#v", event.Payload)
			}
		case "turn.agent_message.delta", "turn.agent_message.completed":
			if event.Payload["message_id"] != "msg_agent_seq_10" {
				t.Fatalf("agent payload=%#v", event.Payload)
			}
		}
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
		preparePromptViewFn: func(context.Context, string, string, string, cubebox.CanonicalContext, string) (cubebox.PromptViewPreparationResponse, error) {
			return cubebox.PromptViewPreparationResponse{}, nil
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
		preparePromptViewFn: func(context.Context, string, string, string, cubebox.CanonicalContext, string) (cubebox.PromptViewPreparationResponse, error) {
			return cubebox.PromptViewPreparationResponse{}, nil
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
		preparePromptViewFn: func(context.Context, string, string, string, cubebox.CanonicalContext, string) (cubebox.PromptViewPreparationResponse, error) {
			return cubebox.PromptViewPreparationResponse{}, nil
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

	var promptViewPrepared bool
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
		preparePromptViewFn: func(_ context.Context, tenantID string, principalID string, conversationID string, canonicalContext cubebox.CanonicalContext, reason string) (cubebox.PromptViewPreparationResponse, error) {
			promptViewPrepared = true
			if tenantID != "tenant-a" || principalID != "principal-a" || conversationID != "conv_1" {
				t.Fatalf("unexpected prompt-view preparation actor tenant=%s principal=%s conversation=%s", tenantID, principalID, conversationID)
			}
			if canonicalContext.TenantID != "tenant-a" || canonicalContext.PrincipalID != "principal-a" {
				t.Fatalf("unexpected canonical context=%+v", canonicalContext)
			}
			if canonicalContext.Page != "/app/cubebox" || canonicalContext.BusinessObject != "conversation" {
				t.Fatalf("expected default canonical context, got %+v", canonicalContext)
			}
			if reason != "pre_turn_auto" {
				t.Fatalf("unexpected preparation reason=%s", reason)
			}
			return cubebox.PromptViewPreparationResponse{
				Conversation: cubebox.Conversation{ID: "conv_1", Title: "新对话", Status: "active"},
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
	if !promptViewPrepared {
		t.Fatal("expected pre-turn prompt view preparation")
	}
	if len(appended) == 0 {
		t.Fatal("expected appended stream events")
	}
	if appended[0].Sequence != 12 {
		t.Fatalf("expected first stream event to continue from prepared next sequence, got %d", appended[0].Sequence)
	}
}

func TestCubeBoxStreamTurnAPIUsesAPIQueryFlow(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/turns:stream", strings.NewReader(`{"conversation_id":"conv_1","prompt":"列出今天全部组织","next_sequence":4}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-a"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "principal-a"}))

	plan := queryAPIPlanForOrgUnitListWithParams(map[string]any{
		"as_of":            "2026-04-25",
		"all_org_units":    true,
		"page":             1,
		"page_size":        100,
		"include_disabled": false,
	})
	var runnerRequest cubebox.ExecuteRequest
	var runnerPlan cubebox.APICallPlan
	var appended []cubebox.CanonicalEvent
	runtime := cubebox.NewRuntime()
	store := cubeboxStoreStub{
		preparePromptViewFn: func(_ context.Context, tenantID string, principalID string, conversationID string, canonicalContext cubebox.CanonicalContext, reason string) (cubebox.PromptViewPreparationResponse, error) {
			if tenantID != "tenant-a" || principalID != "principal-a" || conversationID != "conv_1" {
				t.Fatalf("unexpected prepare actor tenant=%s principal=%s conversation=%s", tenantID, principalID, conversationID)
			}
			if canonicalContext.TenantID != "tenant-a" || canonicalContext.PrincipalID != "principal-a" {
				t.Fatalf("unexpected canonical context=%+v", canonicalContext)
			}
			if reason != "pre_turn_auto" {
				t.Fatalf("unexpected preparation reason=%s", reason)
			}
			return cubebox.PromptViewPreparationResponse{
				Conversation: cubebox.Conversation{ID: "conv_1", Title: "新对话", Status: "active"},
				NextSequence: 4,
			}, nil
		},
		appendFn: func(_ context.Context, tenantID string, principalID string, conversationID string, event cubebox.CanonicalEvent) error {
			if tenantID != "tenant-a" || principalID != "principal-a" || conversationID != "conv_1" {
				t.Fatalf("unexpected append actor tenant=%s principal=%s conversation=%s", tenantID, principalID, conversationID)
			}
			appended = append(appended, event)
			return nil
		},
	}
	flow := queryLoopTestFlow(cubeboxAPIToolRunnerStub{
		fn: func(_ context.Context, request cubebox.ExecuteRequest, gotPlan cubebox.APICallPlan) ([]cubebox.ExecuteResult, error) {
			runnerRequest = request
			runnerPlan = gotPlan
			return []cubebox.ExecuteResult{{
				Method:      "GET",
				Path:        "/org/api/org-units",
				OperationID: "orgunit.list",
				Payload: map[string]any{
					"as_of":     "2026-04-25",
					"org_units": []map[string]any{{"org_code": "100000", "name": "总部"}},
				},
				PresentedCandidates: []cubebox.QueryCandidate{
					{Domain: "orgunit", EntityKey: "100000", Name: "总部", AsOf: "2026-04-25"},
				},
			}}, nil
		},
	}, cubeboxAPIPlanProducerStub{results: []cubeboxAPIPlanProductionResult{
		queryPlannerAPICallsResult(plan),
		queryPlannerDoneResult(),
	}, index: new(int)}, cubeboxQueryNarratorStub{text: "今天共有 1 个组织：总部。"})
	flow.runtime = runtime
	flow.store = store

	handleCubeBoxStreamTurnAPI(rec, req, runtime, store, newTestGateway(runtime), flow)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if runnerRequest.TenantID != "tenant-a" || runnerRequest.PrincipalID != "principal-a" || runnerRequest.ConversationID != "conv_1" {
		t.Fatalf("unexpected runner request=%#v", runnerRequest)
	}
	if len(runnerPlan.Calls) != 1 || runnerPlan.Calls[0].Method != "GET" || runnerPlan.Calls[0].Path != "/org/api/org-units" {
		t.Fatalf("unexpected runner plan=%#v", runnerPlan)
	}
	body := rec.Body.String()
	for _, expected := range []string{
		`"runtime":"cubebox-query-api-calls"`,
		`"type":"turn.agent_message.delta"`,
		`今天共有 1 个组织：总部。`,
		`"type":"turn.completed"`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected stream body to contain %q, got %s", expected, body)
		}
	}
	if len(appended) == 0 {
		t.Fatal("expected query flow events appended")
	}
	foundCandidates := false
	for _, event := range appended {
		if event.Type == cubebox.QueryCandidatesPresentedEventType {
			foundCandidates = true
			break
		}
	}
	if !foundCandidates {
		t.Fatalf("expected presented candidates metadata, got %#v", appended)
	}
}

func TestCubeBoxStreamTurnAPIQueryFlowExecutionErrorDoesNotFallback(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/turns:stream", strings.NewReader(`{"conversation_id":"conv_1","prompt":"列出今天全部组织","next_sequence":4}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-a"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "principal-a"}))

	runtime := cubebox.NewRuntime()
	store := cubeboxStoreStub{
		preparePromptViewFn: func(context.Context, string, string, string, cubebox.CanonicalContext, string) (cubebox.PromptViewPreparationResponse, error) {
			return cubebox.PromptViewPreparationResponse{
				Conversation: cubebox.Conversation{ID: "conv_1", Title: "新对话", Status: "active"},
				NextSequence: 4,
			}, nil
		},
		appendFn: func(context.Context, string, string, string, cubebox.CanonicalEvent) error { return nil },
	}
	flow := queryLoopTestFlow(cubeboxAPIToolRunnerStub{err: cubebox.ErrAPICatalogDriftOrExecutorMissing}, cubeboxAPIPlanProducerStub{result: queryPlannerAPICallsResult(queryAPIPlanForOrgUnitList("2026-04-25", ""))}, cubeboxQueryNarratorStub{text: "should not narrate"})
	flow.runtime = runtime
	flow.store = store

	handleCubeBoxStreamTurnAPI(rec, req, runtime, store, newTestGateway(runtime), flow)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"type":"turn.error"`) || !strings.Contains(body, `"code":"api_catalog_drift_or_executor_missing"`) {
		t.Fatalf("expected query flow terminal error, got %s", body)
	}
	if strings.Contains(body, `"runtime":"deterministic-fixture"`) {
		t.Fatalf("query flow error should not fallback to gateway, got %s", body)
	}
}

func TestCubeBoxStreamTurnAPIRejectsLegacyPageContextField(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/turns:stream", strings.NewReader(`{"conversation_id":"conv_1","prompt":"hello","next_sequence":8,"page_context":{"page":"/org/units/100000"}}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-a"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "principal-a"}))

	handleCubeBoxStreamTurnAPI(rec, req, cubebox.NewRuntime(), cubeboxStoreStub{}, newTestGateway(cubebox.NewRuntime()), nil)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"invalid_json"`) {
		t.Fatalf("expected invalid_json, got %s", rec.Body.String())
	}
}
