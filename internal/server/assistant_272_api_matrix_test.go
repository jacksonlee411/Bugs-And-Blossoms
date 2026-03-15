package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func assistant272TestGateway(t *testing.T, payload []byte) *assistantModelGateway {
	t.Helper()
	t.Setenv("OPENAI_API_KEY", "dummy")
	return &assistantModelGateway{
		config: assistantModelConfig{
			ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
			Providers: []assistantModelProviderConfig{{
				Name:      "openai",
				Enabled:   true,
				Model:     "gpt-5-codex",
				Endpoint:  "https://api.openai.com/v1",
				TimeoutMS: 1000,
				Retries:   0,
				Priority:  1,
				KeyRef:    "OPENAI_API_KEY",
			}},
		},
		adapters: map[string]assistantProviderAdapter{
			"openai": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
				return payload, nil
			}),
		},
	}
}

func TestAssistant272TurnAPI_CreateAndConfirmMatrix(t *testing.T) {
	originalAuthorizer := assistantLoadAuthorizerFn
	originalDefinitions := capabilityDefinitionByKey
	defer func() {
		assistantLoadAuthorizerFn = originalAuthorizer
		capabilityDefinitionByKey = originalDefinitions
	}()
	assistantLoadAuthorizerFn = func() (authorizer, error) {
		return assistantGateAuthorizerStub{allowed: true, enforced: true}, nil
	}

	capabilityDefinitionByKey = map[string]capabilityDefinition{}
	for _, tc := range assistant272ActionCases() {
		spec, ok := assistantLookupDefaultActionSpec(tc.action)
		if !ok {
			t.Fatalf("missing spec action=%s", tc.action)
		}
		capabilityDefinitionByKey[spec.CapabilityKey] = capabilityDefinition{CapabilityKey: spec.CapabilityKey, Status: routeCapabilityStatusActive, ActivationState: routeCapabilityStatusActive}
	}

	principal := Principal{ID: "actor_1", RoleSlug: "tenant-admin"}
	for _, tc := range assistant272ActionCases() {
		t.Run(tc.name, func(t *testing.T) {
			store := newOrgUnitMemoryStore()
			if _, err := store.CreateNodeCurrent(context.Background(), "tenant-1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
				t.Fatal(err)
			}
			svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
			payload, err := json.Marshal(assistantSemanticIntentPayload{
				Action:              tc.intent.Action,
				IntentID:            assistantSemanticIntentIDForAction(tc.intent.Action),
				RouteKind:           assistantRouteKindBusinessAction,
				ParentRefText:       tc.intent.ParentRefText,
				EntityName:          tc.intent.EntityName,
				EffectiveDate:       tc.intent.EffectiveDate,
				OrgCode:             tc.intent.OrgCode,
				TargetEffectiveDate: tc.intent.TargetEffectiveDate,
				NewName:             tc.intent.NewName,
				NewParentRefText:    tc.intent.NewParentRefText,
				IntentSchemaVersion: tc.intent.IntentSchemaVersion,
				ContextHash:         tc.intent.ContextHash,
				IntentHash:          tc.intent.IntentHash,
				Readiness:           assistantSemanticReadinessReadyForConfirm,
			})
			if err != nil {
				t.Fatalf("marshal intent action=%s err=%v", tc.action, err)
			}
			svc.modelGateway = assistant272TestGateway(t, payload)
			svc.gatewayErr = nil
			conversation := svc.createConversation("tenant-1", principal)

			createRec := httptest.NewRecorder()
			createReq := assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/"+conversation.ConversationID+"/turns", `{"user_input":"`+tc.userInput+`"}`, true, true)
			createReq = createReq.WithContext(withPrincipal(createReq.Context(), principal))
			handleAssistantConversationTurnsAPI(createRec, createReq, svc)
			if createRec.Code != http.StatusOK {
				t.Fatalf("create status=%d body=%s", createRec.Code, createRec.Body.String())
			}
			var created assistantConversation
			if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
				t.Fatalf("unmarshal create response err=%v body=%s", err, createRec.Body.String())
			}
			turn := assistantLookupTurn(&created, created.Turns[len(created.Turns)-1].TurnID)
			if turn == nil || turn.Intent.Action != tc.action || turn.State != assistantStateValidated {
				t.Fatalf("unexpected created turn=%+v", turn)
			}

			confirmRec := httptest.NewRecorder()
			confirmReq := assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/"+conversation.ConversationID+"/turns/"+turn.TurnID+":confirm", `{}`, true, true)
			confirmReq = confirmReq.WithContext(withPrincipal(confirmReq.Context(), principal))
			handleAssistantTurnActionAPI(confirmRec, confirmReq, svc)
			if confirmRec.Code != http.StatusOK {
				t.Fatalf("confirm status=%d body=%s", confirmRec.Code, confirmRec.Body.String())
			}
			var confirmed assistantConversation
			if err := json.Unmarshal(confirmRec.Body.Bytes(), &confirmed); err != nil {
				t.Fatalf("unmarshal confirm response err=%v body=%s", err, confirmRec.Body.String())
			}
			confirmedTurn := assistantLookupTurn(&confirmed, turn.TurnID)
			if confirmedTurn == nil || confirmedTurn.State != assistantStateConfirmed {
				t.Fatalf("unexpected confirmed turn=%+v", confirmedTurn)
			}
		})
	}
}
