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

type cubeboxStoreStub struct {
	createFn  func(context.Context, string, string) (cubebox.ConversationReplayResponse, error)
	getFn     func(context.Context, string, string, string) (cubebox.ConversationReplayResponse, error)
	listFn    func(context.Context, string, string, int32) (cubebox.ConversationListResponse, error)
	renameFn  func(context.Context, string, string, string, string) (cubebox.ConversationReplayResponse, error)
	archiveFn func(context.Context, string, string, string, bool) (cubebox.ConversationReplayResponse, error)
	compactFn func(context.Context, string, string, string, cubebox.CanonicalContext, string) (cubebox.CompactConversationResponse, error)
	appendFn  func(context.Context, string, string, string, cubebox.CanonicalEvent) error
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

	handleCubeBoxStreamTurnAPI(rec, req, cubebox.NewRuntime(), cubeboxStoreStub{
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
	})

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

	handleCubeBoxStreamTurnAPI(rec, req, cubebox.NewRuntime(), cubeboxStoreStub{
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
	})

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
}

func TestCubeBoxStreamTurnAPIUsesUUIDEventIDs(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/turns:stream", strings.NewReader(`{"conversation_id":"conv_1","prompt":"hello","next_sequence":48}`))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))

	var eventIDs []string
	handleCubeBoxStreamTurnAPI(rec, req, cubebox.NewRuntime(), cubeboxStoreStub{
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
	})

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
		createFn: func(context.Context, string, string) (cubebox.ConversationReplayResponse, error) { return cubebox.ConversationReplayResponse{}, errors.New("unexpected") },
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
		listFn:    func(context.Context, string, string, int32) (cubebox.ConversationListResponse, error) { return cubebox.ConversationListResponse{}, errors.New("unexpected") },
		renameFn:  func(context.Context, string, string, string, string) (cubebox.ConversationReplayResponse, error) { return cubebox.ConversationReplayResponse{}, errors.New("unexpected") },
		archiveFn: func(context.Context, string, string, string, bool) (cubebox.ConversationReplayResponse, error) { return cubebox.ConversationReplayResponse{}, errors.New("unexpected") },
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
	handleCubeBoxStreamTurnAPI(rec, req, cubebox.NewRuntime(), cubeboxStoreStub{
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
	})

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
