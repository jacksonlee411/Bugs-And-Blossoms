package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleAssistantTurnActionAPIReplyErrorBranches(t *testing.T) {
	principal := Principal{ID: "actor-1", RoleSlug: "tenant-admin"}
	cases := []struct {
		name     string
		stubErr  error
		wantCode string
		wantHTTP int
	}{
		{name: "target mismatch", stubErr: errAssistantReplyModelTargetMismatch, wantCode: "ai_reply_model_target_mismatch", wantHTTP: http.StatusUnprocessableEntity},
		{name: "render failed", stubErr: errAssistantReplyRenderFailed, wantCode: "ai_reply_render_failed", wantHTTP: http.StatusUnprocessableEntity},
		{name: "provider unavailable", stubErr: errAssistantModelProviderUnavailable, wantCode: "ai_model_provider_unavailable", wantHTTP: http.StatusServiceUnavailable},
		{name: "timeout", stubErr: errAssistantModelTimeout, wantCode: "ai_model_timeout", wantHTTP: http.StatusGatewayTimeout},
		{name: "rate limited", stubErr: errAssistantModelRateLimited, wantCode: "ai_model_rate_limited", wantHTTP: http.StatusTooManyRequests},
		{name: "config invalid", stubErr: errAssistantModelConfigInvalid, wantCode: "ai_model_config_invalid", wantHTTP: http.StatusUnprocessableEntity},
		{name: "runtime invalid", stubErr: errAssistantRuntimeConfigInvalid, wantCode: "ai_runtime_config_invalid", wantHTTP: http.StatusUnprocessableEntity},
		{name: "runtime missing", stubErr: errAssistantRuntimeConfigMissing, wantCode: "ai_runtime_config_missing", wantHTTP: http.StatusServiceUnavailable},
		{name: "secret missing", stubErr: errAssistantModelSecretMissing, wantCode: "ai_model_secret_missing", wantHTTP: http.StatusInternalServerError},
		{name: "default", stubErr: errors.New("boom"), wantCode: "assistant_reply_render_failed", wantHTTP: http.StatusInternalServerError},
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
			assistantRenderReplyWithModelFn = func(_ context.Context, _ *assistantConversationService, _ assistantReplyRenderPrompt) (assistantReplyModelResult, error) {
				return assistantReplyModelResult{}, tc.stubErr
			}
			path := "/internal/assistant/conversations/" + conv.ConversationID + "/turns/" + turn.TurnID + ":reply"
			rec := httptest.NewRecorder()
			handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, path, `{}`, true, true), svc)
			if rec.Code != tc.wantHTTP || assistantDecodeErrCode(t, rec) != tc.wantCode {
				t.Fatalf("status=%d code=%s body=%s", rec.Code, assistantDecodeErrCode(t, rec), rec.Body.String())
			}
		})
	}
}
