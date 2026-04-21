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
