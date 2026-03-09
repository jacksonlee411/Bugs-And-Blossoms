package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleAssistantTurnActionAPIReplySuccessAndValidation(t *testing.T) {
	principal := Principal{ID: "actor-1", RoleSlug: "tenant-admin"}
	svc := &assistantConversationService{byID: map[string]*assistantConversation{}, byActorID: map[string][]string{}}
	conv := svc.createConversation("tenant-1", principal)
	turn := &assistantTurn{TurnID: "turn-1", State: assistantStateValidated}
	svc.mu.Lock()
	svc.byID[conv.ConversationID].Turns = []*assistantTurn{turn}
	svc.mu.Unlock()

	original := assistantRenderReplyWithModelFn
	defer func() { assistantRenderReplyWithModelFn = original }()
	assistantRenderReplyWithModelFn = func(_ context.Context, _ *assistantConversationService, _ assistantReplyRenderPrompt) (assistantReplyModelResult, error) {
		return assistantReplyModelResult{Text: "已处理", Kind: "info", Stage: "draft", ReplyModelName: assistantReplyTargetModelName}, nil
	}

	path := "/internal/assistant/conversations/" + conv.ConversationID + "/turns/" + turn.TurnID + ":reply"
	rec := httptest.NewRecorder()
	handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, path, `{}`, true, true), svc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var reply assistantRenderReplyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &reply); err != nil {
		t.Fatalf("decode reply: %v", err)
	}
	if reply.Text != "已处理" || reply.TurnID != turn.TurnID {
		t.Fatalf("reply=%+v", reply)
	}

	badJSONRec := httptest.NewRecorder()
	handleAssistantTurnActionAPI(badJSONRec, assistantReqWithContext(http.MethodPost, path, `{`, true, true), svc)
	if badJSONRec.Code != http.StatusBadRequest || assistantDecodeErrCode(t, badJSONRec) != "bad_json" {
		t.Fatalf("status=%d code=%s body=%s", badJSONRec.Code, assistantDecodeErrCode(t, badJSONRec), badJSONRec.Body.String())
	}

	unsupportedRec := httptest.NewRecorder()
	handleAssistantTurnActionAPI(unsupportedRec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/"+conv.ConversationID+"/turns/"+turn.TurnID+":unsupported", `{}`, true, true), svc)
	if unsupportedRec.Code != http.StatusBadRequest || assistantDecodeErrCode(t, unsupportedRec) != "invalid_request" {
		t.Fatalf("status=%d code=%s body=%s", unsupportedRec.Code, assistantDecodeErrCode(t, unsupportedRec), unsupportedRec.Body.String())
	}
}

func TestHandleAssistantTurnActionAPIReplyNotFoundAndForbidden(t *testing.T) {
	principal := Principal{ID: "actor-1", RoleSlug: "tenant-admin"}
	basePath := "/internal/assistant/conversations/conv-1/turns/turn-1:reply"

	t.Run("conversation not found", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, basePath, `{}`, true, true), newAssistantConversationService(nil, nil))
		if rec.Code != http.StatusNotFound || assistantDecodeErrCode(t, rec) != "conversation_not_found" {
			t.Fatalf("status=%d code=%s body=%s", rec.Code, assistantDecodeErrCode(t, rec), rec.Body.String())
		}
	})

	t.Run("tenant mismatch", func(t *testing.T) {
		svc := &assistantConversationService{byID: map[string]*assistantConversation{
			"conv-1": {ConversationID: "conv-1", TenantID: "tenant-2", ActorID: principal.ID, ActorRole: principal.RoleSlug},
		}}
		rec := httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, basePath, `{}`, true, true), svc)
		if rec.Code != http.StatusForbidden || assistantDecodeErrCode(t, rec) != "tenant_mismatch" {
			t.Fatalf("status=%d code=%s body=%s", rec.Code, assistantDecodeErrCode(t, rec), rec.Body.String())
		}
	})

	t.Run("forbidden by role mismatch", func(t *testing.T) {
		svc := &assistantConversationService{byID: map[string]*assistantConversation{
			"conv-1": {ConversationID: "conv-1", TenantID: "tenant-1", ActorID: principal.ID, ActorRole: "viewer", Turns: []*assistantTurn{{TurnID: "turn-1"}}},
		}}
		rec := httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, basePath, `{}`, true, true), svc)
		if rec.Code != http.StatusForbidden || assistantDecodeErrCode(t, rec) != "forbidden" {
			t.Fatalf("status=%d code=%s body=%s", rec.Code, assistantDecodeErrCode(t, rec), rec.Body.String())
		}
	})

	t.Run("turn not found", func(t *testing.T) {
		svc := &assistantConversationService{byID: map[string]*assistantConversation{
			"conv-1": {ConversationID: "conv-1", TenantID: "tenant-1", ActorID: principal.ID, ActorRole: principal.RoleSlug, Turns: []*assistantTurn{}},
		}}
		rec := httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, basePath, `{}`, true, true), svc)
		if rec.Code != http.StatusNotFound || assistantDecodeErrCode(t, rec) != "conversation_turn_not_found" {
			t.Fatalf("status=%d code=%s body=%s", rec.Code, assistantDecodeErrCode(t, rec), rec.Body.String())
		}
	})
}
