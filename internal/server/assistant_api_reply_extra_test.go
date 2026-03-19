package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleAssistantTurnActionAPIReplyErrorBranches(t *testing.T) {
	principal := Principal{ID: "actor-1", RoleSlug: "tenant-admin"}
	cases := []struct {
		name    string
		stubErr error
	}{
		{name: "target mismatch", stubErr: errAssistantReplyModelTargetMismatch},
		{name: "render failed", stubErr: errAssistantReplyRenderFailed},
		{name: "provider unavailable", stubErr: errAssistantModelProviderUnavailable},
		{name: "timeout", stubErr: errAssistantModelTimeout},
		{name: "rate limited", stubErr: errAssistantModelRateLimited},
		{name: "config invalid", stubErr: errAssistantModelConfigInvalid},
		{name: "runtime invalid", stubErr: errAssistantRuntimeConfigInvalid},
		{name: "runtime missing", stubErr: errAssistantRuntimeConfigMissing},
		{name: "secret missing", stubErr: errAssistantModelSecretMissing},
		{name: "default", stubErr: errors.New("boom")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &assistantConversationService{byID: map[string]*assistantConversation{}, byActorID: map[string][]string{}}
			conv := svc.createConversation("tenant-1", principal)
			turn := &assistantTurn{TurnID: "turn-1", State: assistantStateValidated}
			svc.mu.Lock()
			svc.byID[conv.ConversationID].Turns = []*assistantTurn{turn}
			svc.mu.Unlock()
			original := assistantRenderReplyWithModelFn
			defer func() { assistantRenderReplyWithModelFn = original }()
			invoked := false
			assistantRenderReplyWithModelFn = func(_ context.Context, _ *assistantConversationService, _ assistantReplyRenderPrompt) (assistantReplyModelResult, error) {
				invoked = true
				return assistantReplyModelResult{}, tc.stubErr
			}
			path := "/internal/assistant/conversations/" + conv.ConversationID + "/turns/" + turn.TurnID + ":reply"
			rec := httptest.NewRecorder()
			handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, path, `{}`, true, true), svc)
			if invoked {
				t.Fatalf("reply model hook should not be invoked under projection-only reply path")
			}
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			var reply assistantRenderReplyResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &reply); err != nil {
				t.Fatalf("decode reply: %v", err)
			}
			if reply.TurnID != turn.TurnID {
				t.Fatalf("unexpected reply=%+v", reply)
			}
			if reply.ReplySource != assistantReplySourceProjection && reply.ReplySource != assistantReplySourceFallback {
				t.Fatalf("reply should come from local projection/fallback, got=%+v", reply)
			}
		})
	}
}
