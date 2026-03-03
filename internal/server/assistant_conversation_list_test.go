package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
