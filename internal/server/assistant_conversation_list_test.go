package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAssistantConversationListAPI_PaginationAndCursor(t *testing.T) {
	svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
	principal := Principal{ID: "actor-1", RoleSlug: "tenant-admin"}
	_ = svc.createConversation("tenant-1", principal)
	_ = svc.createConversation("tenant-1", principal)

	rec := httptest.NewRecorder()
	req := assistantReqWithContext(http.MethodGet, "/internal/assistant/conversations?page_size=1", "", true, true)
	handleAssistantConversationsAPI(rec, req, svc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var page1 assistantConversationListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &page1); err != nil {
		t.Fatalf("unmarshal page1: %v", err)
	}
	if len(page1.Items) != 1 {
		t.Fatalf("items=%d", len(page1.Items))
	}
	if page1.NextCursor == "" {
		t.Fatal("expected next cursor")
	}

	rec = httptest.NewRecorder()
	req = assistantReqWithContext(http.MethodGet, "/internal/assistant/conversations?page_size=1&cursor="+page1.NextCursor, "", true, true)
	handleAssistantConversationsAPI(rec, req, svc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var page2 assistantConversationListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &page2); err != nil {
		t.Fatalf("unmarshal page2: %v", err)
	}
	if len(page2.Items) != 1 {
		t.Fatalf("items=%d", len(page2.Items))
	}
	if page2.Items[0].ConversationID == page1.Items[0].ConversationID {
		t.Fatal("expected different conversation on next page")
	}

	rec = httptest.NewRecorder()
	req = assistantReqWithContext(http.MethodGet, "/internal/assistant/conversations?cursor=broken", "", true, true)
	handleAssistantConversationsAPI(rec, req, svc)
	if rec.Code != http.StatusBadRequest || assistantDecodeErrCode(t, rec) != "assistant_conversation_cursor_invalid" {
		t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
	}
}

func TestAssistantConversationService_ListConversationsCursorEqualTimestampBranch(t *testing.T) {
	svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
	principal := Principal{ID: "actor-1", RoleSlug: "tenant-admin"}
	_ = svc.createConversation("tenant-1", principal)
	_ = svc.createConversation("tenant-1", principal)

	svc.mu.Lock()
	now := time.Now().UTC().Truncate(time.Second)
	conversationA := &assistantConversation{
		ConversationID: "conv_A",
		TenantID:       "tenant-1",
		ActorID:        principal.ID,
		ActorRole:      principal.RoleSlug,
		State:          assistantStateValidated,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	conversationB := &assistantConversation{
		ConversationID: "conv_B",
		TenantID:       "tenant-1",
		ActorID:        principal.ID,
		ActorRole:      principal.RoleSlug,
		State:          assistantStateValidated,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	svc.byID = map[string]*assistantConversation{
		conversationA.ConversationID: conversationA,
		conversationB.ConversationID: conversationB,
	}
	svc.mu.Unlock()

	cursor := assistantEncodeConversationCursor(assistantConversationCursor{
		UpdatedAt:      now,
		ConversationID: "conv_B",
	}, "tenant-1", principal.ID)
	items, nextCursor, err := svc.listConversations(nil, "tenant-1", principal.ID, 20, cursor)
	if err != nil {
		t.Fatalf("list conversations err=%v", err)
	}
	if nextCursor != "" {
		t.Fatalf("unexpected next cursor=%s", nextCursor)
	}
	if len(items) != 1 || items[0].ConversationID != "conv_A" {
		t.Fatalf("unexpected items=%+v", items)
	}
}
