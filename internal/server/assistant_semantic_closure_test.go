package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func assistantTestSemanticGateway(adapter assistantProviderAdapter) *assistantModelGateway {
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
			"openai": adapter,
		},
	}
}

func TestAssistant268SemanticContractClosureCoverage(t *testing.T) {
	if assistantSemanticReadinessKnown("unknown") {
		t.Fatal("unexpected readiness accepted")
	}
	if assistantSemanticRetrievalStateKnown("unexpected") {
		t.Fatal("unexpected retrieval state accepted")
	}
	if got := assistantNormalizeSemanticRetrievalRequests(nil); got != nil {
		t.Fatalf("nil retrieval requests should stay nil, got=%+v", got)
	}
	if got := assistantNormalizeSemanticRetrievalRequests([]assistantSemanticRetrievalRequest{{Kind: " "}}); got != nil {
		t.Fatalf("empty retrieval requests should be dropped, got=%+v", got)
	}
	if got := assistantNormalizeSemanticRetrievalResults(nil); got != nil {
		t.Fatalf("nil retrieval results should stay nil, got=%+v", got)
	}
	if got := assistantNormalizeSemanticRetrievalResults([]assistantSemanticRetrievalResult{{Kind: "candidate_lookup", State: "bad"}}); got != nil {
		t.Fatalf("invalid retrieval results should be dropped, got=%+v", got)
	}

	requests := assistantNormalizeSemanticRetrievalRequests([]assistantSemanticRetrievalRequest{
		{Kind: " candidate_lookup ", Slot: " parent_ref_text ", RefText: " 鲜花组织 ", AsOf: " 2026-01-01 ", Limit: 0},
		{Kind: "candidate_lookup", Slot: "parent_ref_text", RefText: "鲜花组织", AsOf: "2026-01-01", Limit: 2},
		{Kind: "", RefText: "ignored"},
	})
	if len(requests) != 1 || requests[0].Limit != 10 || requests[0].RefText != "鲜花组织" {
		t.Fatalf("unexpected normalized requests=%+v", requests)
	}

	results := assistantNormalizeSemanticRetrievalResults([]assistantSemanticRetrievalResult{
		{Kind: " candidate_lookup ", Slot: " parent_ref_text ", State: " single_match ", RefText: " 鲜花组织 ", AsOf: " 2026-01-01 ", CandidateCount: -1, CandidateIDs: []string{"FLOWER-A", "FLOWER-A"}, SelectedCandidateID: " FLOWER-A "},
		{Kind: "candidate_lookup", Slot: "parent_ref_text", State: "single_match", RefText: "鲜花组织", AsOf: "2026-01-01"},
		{Kind: "candidate_lookup", State: "bad"},
	})
	if len(results) != 1 || results[0].CandidateCount != 0 || len(results[0].CandidateIDs) != 1 {
		t.Fatalf("unexpected normalized results=%+v", results)
	}

	resolvedWithState := assistantResolveIntentResult{
		Intent: assistantIntentSpec{
			Action:              assistantIntentCreateOrgUnit,
			IntentID:            "org.orgunit_create",
			RouteKind:           assistantRouteKindBusinessAction,
			RouteCatalogVersion: "v1",
			ParentRefText:       "ignored-by-state",
			EffectiveDate:       "2026-01-01",
		},
		SemanticState: assistantConversationSemanticState{
			Action:              assistantIntentCreateOrgUnit,
			IntentID:            "org.orgunit_create",
			RouteKind:           assistantRouteKindBusinessAction,
			RouteCatalogVersion: "v2",
			Slots: assistantIntentSpec{
				Action:              assistantIntentCreateOrgUnit,
				IntentID:            "org.orgunit_create",
				RouteKind:           assistantRouteKindBusinessAction,
				RouteCatalogVersion: "v2",
				ParentRefText:       "鲜花组织",
				EffectiveDate:       "2026-01-01",
			},
			GoalSummary:         "创建运营部",
			Readiness:           assistantSemanticReadinessReadyForConfirm,
			RetrievalRequests:   requests,
			RetrievalResults:    results,
			SelectedCandidateID: "FLOWER-A",
		},
	}
	state := assistantSemanticStateFromResolved(resolvedWithState)
	if state.RouteCatalogVersion != "v2" || len(state.RetrievalRequests) != 1 || len(state.RetrievalResults) != 1 {
		t.Fatalf("unexpected semantic state=%+v", state)
	}

	resolvedWithoutState := assistantResolveIntentResult{
		Intent: assistantIntentSpec{
			Action:              assistantIntentCreateOrgUnit,
			IntentID:            "org.orgunit_create",
			RouteKind:           assistantRouteKindBusinessAction,
			RouteCatalogVersion: "v1",
			ParentRefText:       "鲜花组织",
		},
		GoalSummary:         "创建运营部",
		UserVisibleReply:    "已生成草案",
		NextQuestion:        "请确认",
		Readiness:           assistantSemanticReadinessReadyForDryRun,
		SelectedCandidateID: "FLOWER-A",
	}
	fallbackState := assistantSemanticStateFromResolved(resolvedWithoutState)
	if fallbackState.GoalSummary != "创建运营部" || fallbackState.Slots.ParentRefText != "鲜花组织" {
		t.Fatalf("unexpected fallback semantic state=%+v", fallbackState)
	}

	assistantSyncResolvedSemanticResult(nil)
	assistantSyncResolvedSemanticResult(&resolvedWithoutState)
	if resolvedWithoutState.SemanticState.Action != assistantIntentCreateOrgUnit || resolvedWithoutState.SelectedCandidateID != "FLOWER-A" {
		t.Fatalf("unexpected synced resolved=%+v", resolvedWithoutState)
	}

	if !assistantSemanticStateNeedsRetrieval(assistantConversationSemanticState{
		Slots: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", RouteKind: assistantRouteKindBusinessAction},
	}) {
		t.Fatal("expected business parent lookup to need retrieval even before as_of is available")
	}
	if assistantSemanticStateNeedsRetrieval(assistantConversationSemanticState{
		Slots: assistantIntentSpec{ParentRefText: "鲜花组织", RouteKind: assistantRouteKindKnowledgeQA},
	}) {
		t.Fatal("non-business route should not need retrieval")
	}
	if !assistantSemanticStateNeedsRetrieval(assistantConversationSemanticState{RetrievalRequests: requests}) {
		t.Fatal("explicit retrieval request should require retrieval")
	}

	if !assistantModelSemanticStateInvalid(assistantResolveIntentResult{
		Intent: assistantIntentSpec{
			Action:              assistantIntentCreateOrgUnit,
			IntentID:            "org.orgunit_create",
			RouteKind:           assistantRouteKindBusinessAction,
			RouteCatalogVersion: "v1",
			EffectiveDate:       "2026-01-01",
		},
		SemanticState: assistantConversationSemanticState{
			Action:    assistantIntentCreateOrgUnit,
			IntentID:  "org.orgunit_create",
			RouteKind: assistantRouteKindBusinessAction,
			Slots: assistantIntentSpec{
				Action:              assistantIntentCreateOrgUnit,
				IntentID:            "org.orgunit_create",
				RouteKind:           assistantRouteKindBusinessAction,
				RouteCatalogVersion: "v1",
				EffectiveDate:       "2026-01-01",
			},
			Readiness: "bad",
		},
	}) {
		t.Fatal("invalid readiness should fail semantic state validation")
	}
	if assistantModelSemanticStateInvalid(assistantResolveIntentResult{
		Intent: assistantIntentSpec{
			Action:              assistantIntentCreateOrgUnit,
			IntentID:            "org.orgunit_create",
			RouteKind:           assistantRouteKindBusinessAction,
			RouteCatalogVersion: "v1",
			EffectiveDate:       "2026-01-01",
		},
		SemanticState: assistantConversationSemanticState{
			Action:    assistantIntentCreateOrgUnit,
			IntentID:  "org.orgunit_create",
			RouteKind: assistantRouteKindBusinessAction,
			Slots: assistantIntentSpec{
				Action:              assistantIntentCreateOrgUnit,
				IntentID:            "org.orgunit_create",
				RouteKind:           assistantRouteKindBusinessAction,
				RouteCatalogVersion: "v1",
				EffectiveDate:       "2026-01-01",
			},
			Readiness:        assistantSemanticReadinessReadyForDryRun,
			RetrievalResults: []assistantSemanticRetrievalResult{{Kind: assistantSemanticRetrievalKindCandidateLookup, State: "bad"}},
		},
	}) {
		t.Fatal("retrieval states should be normalized before semantic validation")
	}
}

func TestAssistant268ModelGatewayRetrievalNormalizationCoverage(t *testing.T) {
	prompt := assistantBuildSemanticResolutionPrompt(
		assistantSemanticPromptEnvelope{CurrentUserInput: "继续", AllowedActions: assistantSemanticPromptActions()},
		assistantConversationSemanticState{
			Action: assistantIntentCreateOrgUnit,
			RetrievalResults: []assistantSemanticRetrievalResult{{
				Kind:           assistantSemanticRetrievalKindCandidateLookup,
				State:          assistantSemanticRetrievalStateSingleMatch,
				RefText:        "鲜花组织",
				AsOf:           "2026-01-01",
				CandidateIDs:   []string{"FLOWER-A"},
				CandidateCount: 1,
			}},
		},
	)
	if !strings.Contains(prompt, "boundary_note") || !strings.Contains(prompt, "retrieval_results") {
		t.Fatalf("unexpected semantic resolution prompt=%s", prompt)
	}

	object := map[string]any{
		"slots":            map[string]any{"parentRefText": "鲜花组织"},
		"data":             map[string]any{"entityName": "运营部"},
		"changes":          []any{map[string]any{"effectiveDate": "2026-01-01"}},
		"retrieval_needed": "yes",
		"lookups": []any{
			map[string]any{"type": "candidate_lookup", "field": "parent_ref_text", "query": "鲜花组织", "asOf": "2026-01-01", "limit": "2"},
			"ignored",
		},
		"lookup_results": []any{
			map[string]any{"type": "candidate_lookup", "field": "parent_ref_text", "status": "single_match", "query": "鲜花组织", "asOf": "2026-01-01", "count": json.Number("1"), "candidateIds": []any{"FLOWER-A", "FLOWER-A"}, "candidateId": "FLOWER-A"},
		},
	}
	if got, ok := assistantLookupLooseMap(object, "slots"); !ok || got["parentRefText"] != "鲜花组织" {
		t.Fatalf("unexpected loose map lookup=%v ok=%v", got, ok)
	}
	if _, ok := assistantLookupLooseMap(map[string]any{"slots": "bad"}, "slots"); ok {
		t.Fatal("non-map loose lookup should fail")
	}
	if got, ok := assistantFirstBoolFromObjects([]map[string]any{{"retrieval_needed": "yes"}}, "retrieval_needed"); !ok || !got {
		t.Fatalf("unexpected bool lookup=%v ok=%v", got, ok)
	}
	if got, ok := assistantFirstBoolFromObjects([]map[string]any{{"retrieval_needed": true}}, "retrieval_needed"); !ok || !got {
		t.Fatalf("unexpected bool-typed lookup=%v ok=%v", got, ok)
	}
	if got, ok := assistantFirstBoolFromObjects([]map[string]any{{"retrieval_needed": "no"}}, "retrieval_needed"); !ok || got {
		t.Fatalf("unexpected false bool lookup=%v ok=%v", got, ok)
	}
	if _, ok := assistantFirstBoolFromObjects([]map[string]any{{"retrieval_needed": "maybe"}}, "retrieval_needed"); ok {
		t.Fatal("invalid bool literal should not parse")
	}
	if got := assistantLooseObjectSlice(object, "lookups"); len(got) != 1 {
		t.Fatalf("unexpected loose object slice=%+v", got)
	}
	if got := assistantLooseObjectSlice(map[string]any{"lookups": "bad", "lookup_requests": []any{"bad"}}, "lookups", "lookup_requests"); got != nil {
		t.Fatalf("unexpected loose object slice fallback=%+v", got)
	}
	if got := assistantLooseStringSlice(map[string]any{"candidate_ids": []any{" FLOWER-A ", "FLOWER-A", 1}}, "candidate_ids"); len(got) != 1 || got[0] != "FLOWER-A" {
		t.Fatalf("unexpected loose string slice=%v", got)
	}
	if got := assistantLooseStringSlice(map[string]any{"candidate_ids": "bad"}, "candidate_ids"); got != nil {
		t.Fatalf("unexpected loose string slice fallback=%v", got)
	}
	if got := assistantFirstInt(map[string]any{"limit": "7"}, "limit"); got != 7 {
		t.Fatalf("unexpected int from string=%d", got)
	}
	if got := assistantFirstInt(map[string]any{"count": json.Number("3")}, "count"); got != 3 {
		t.Fatalf("unexpected int from json.Number=%d", got)
	}
	if got := assistantFirstInt(map[string]any{"count": float64(4)}, "count"); got != 4 {
		t.Fatalf("unexpected int from float=%d", got)
	}
	if got := assistantFirstInt(map[string]any{"count": int64(5)}, "count"); got != 5 {
		t.Fatalf("unexpected int from int64=%d", got)
	}
	if got := assistantFirstInt(map[string]any{"count": int(6)}, "count"); got != 6 {
		t.Fatalf("unexpected int from int=%d", got)
	}
	if got := assistantFirstInt(map[string]any{"count": "bad"}, "count"); got != 0 {
		t.Fatalf("unexpected int from invalid string=%d", got)
	}

	candidateObjects := assistantIntentCandidateObjects(object)
	if len(candidateObjects) < 3 || candidateObjects[0]["effectiveDate"] != "2026-01-01" {
		t.Fatalf("unexpected candidate objects=%+v", candidateObjects)
	}

	requests := assistantNormalizeOpenAIRetrievalRequests(object)
	if len(requests) != 1 || requests[0].Limit != 2 {
		t.Fatalf("unexpected normalized openai retrieval requests=%+v", requests)
	}
	if got := assistantNormalizeOpenAIRetrievalRequests(map[string]any{"lookups": []any{map[string]any{"field": "parent_ref_text"}}}); got != nil {
		t.Fatalf("blank retrieval request kinds should be dropped, got=%+v", got)
	}
	results := assistantNormalizeOpenAIRetrievalResults(object)
	if len(results) != 1 || results[0].CandidateCount != 1 || results[0].SelectedCandidateID != "FLOWER-A" {
		t.Fatalf("unexpected normalized openai retrieval results=%+v", results)
	}
	if got := assistantNormalizeOpenAIRetrievalResults(map[string]any{"lookup_results": []any{map[string]any{"field": "parent_ref_text"}}}); got != nil {
		t.Fatalf("blank retrieval result kinds should be dropped, got=%+v", got)
	}

	normalized := assistantNormalizeOpenAIIntentPayload(`{
		"proposed_action":"create_orgunit",
		"intentId":"org.orgunit_create",
		"routeKind":"business_action",
		"slots":{"parentRefText":"鲜花组织","entityName":"运营部","effectiveDate":"2026-01-01"},
		"retrievalNeeded":"true",
		"lookups":[{"type":"candidate_lookup","field":"parent_ref_text","query":"鲜花组织","asOf":"2026-01-01","limit":"2"}],
		"lookupResults":[{"type":"candidate_lookup","field":"parent_ref_text","status":"single_match","query":"鲜花组织","asOf":"2026-01-01","count":"1","candidateIds":["FLOWER-A"],"candidateId":"FLOWER-A"}],
		"confidenceNote":"high"
	}`)
	payload, err := assistantStrictDecodeSemanticIntent(normalized)
	if err != nil {
		t.Fatalf("strict decode semantic with retrieval err=%v payload=%s", err, string(normalized))
	}
	if payload.Action != assistantIntentCreateOrgUnit || !payload.RetrievalNeeded || len(payload.RetrievalRequests) != 1 || len(payload.RetrievalResults) != 1 || payload.ConfidenceNote != "high" {
		t.Fatalf("unexpected normalized payload=%+v", payload)
	}
	if got := string(assistantNormalizeOpenAIIntentPayload("raw-text")); got != "raw-text" {
		t.Fatalf("unexpected passthrough payload=%q", got)
	}
	if got := string(assistantNormalizeOpenAIIntentPayload("{}")); got != "{}" {
		t.Fatalf("unexpected empty object passthrough=%q", got)
	}

	syntheticCases := map[string]string{
		"请停用这个部门":  assistantIntentDisableOrgUnit,
		"请启用这个部门":  assistantIntentEnableOrgUnit,
		"请重命名这个部门": assistantIntentRenameOrgUnit,
		"请创建部门":    assistantIntentCreateOrgUnit,
		"":         "",
	}
	for input, want := range syntheticCases {
		if got := assistantSyntheticSemanticAction(input); got != want {
			t.Fatalf("input=%q got=%q want=%q", input, got, want)
		}
	}
}

func TestAssistant268SemanticOrchestratorClosureCoverage(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "dummy")

	if _, err := (*assistantConversationService)(nil).resolveSemanticPrompt(context.Background(), "tenant_1", "conv_1", "{}"); err != errAssistantServiceMissing {
		t.Fatalf("expected service missing, got=%v", err)
	}
	withGatewayErr := &assistantConversationService{gatewayErr: errAssistantModelTimeout}
	if _, err := withGatewayErr.resolveSemanticPrompt(context.Background(), "tenant_1", "conv_1", "{}"); err != errAssistantModelTimeout {
		t.Fatalf("expected gateway err passthrough, got=%v", err)
	}
	if _, err := (&assistantConversationService{}).resolveSemanticPrompt(context.Background(), "tenant_1", "conv_1", "{}"); err != errAssistantModelProviderUnavailable {
		t.Fatalf("expected provider unavailable, got=%v", err)
	}
	timeoutSvc := newAssistantConversationService(nil, nil)
	timeoutSvc.modelGateway = assistantTestSemanticGateway(assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
		return nil, errAssistantModelTimeout
	}))
	if _, err := timeoutSvc.resolveSemanticPrompt(context.Background(), "tenant_1", "conv_1", "{}"); err != errAssistantModelTimeout {
		t.Fatalf("expected timeout passthrough, got=%v", err)
	}

	invalidSvc := newAssistantConversationService(nil, nil)
	invalidSvc.modelGateway = assistantTestStaticSemanticGateway(`{"action":"create_orgunit","route_kind":"business_action"}`)
	if _, err := invalidSvc.resolveSemanticPrompt(context.Background(), "tenant_1", "conv_1", `{"current_user_input":"继续"}`); err != errAssistantPlanSchemaConstrainedDecodeFailed {
		t.Fatalf("expected schema constrained decode failure, got=%v", err)
	}

	store := newOrgUnitMemoryStore()
	tenantID := "tenant_1"
	if _, err := store.CreateNodeCurrent(context.Background(), tenantID, "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
		t.Fatalf("create node err=%v", err)
	}
	svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
	callCount := 0
	svc.modelGateway = assistantTestSemanticGateway(assistantAdapterFunc(func(_ context.Context, _ string, _ assistantModelProviderConfig) ([]byte, error) {
		callCount++
		if callCount == 1 {
			return []byte(`{"action":"create_orgunit","route_kind":"business_action","intent_id":"org.orgunit_create","parent_ref_text":"鲜花组织","entity_name":"运营部","effective_date":"2026-01-01","readiness":"ready_for_dry_run","retrieval_requests":[{"kind":"candidate_lookup","slot":"parent_ref_text","ref_text":"鲜花组织","as_of":"2026-01-01","limit":2}]}`), nil
		}
		return []byte(`{"action":"create_orgunit","route_kind":"business_action","intent_id":"org.orgunit_create","parent_ref_text":"鲜花组织","entity_name":"运营部","effective_date":"2026-01-01","selected_candidate_id":"FLOWER-A","user_visible_reply":"已生成草案","readiness":"ready_for_confirm"}`), nil
	}))
	resolution, err := svc.orchestrateSemanticTurn(context.Background(), tenantID, Principal{ID: "actor_1", RoleSlug: "tenant-admin"}, "conv_1", "在鲜花组织下创建运营部", nil)
	if err != nil {
		t.Fatalf("orchestrateSemanticTurn err=%v", err)
	}
	if callCount != 2 || resolution.ResolvedCandidateID != "FLOWER-A" || resolution.SelectedCandidateID != "FLOWER-A" {
		t.Fatalf("unexpected semantic turn resolution=%+v calls=%d", resolution, callCount)
	}
	if resolution.Retrieval.State != assistantSemanticRetrievalStateSingleMatch || resolution.ResolutionSource != assistantResolutionUserConfirmed {
		t.Fatalf("unexpected retrieval resolution=%+v", resolution)
	}

	followupErrSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
	followupErrCalls := 0
	followupErrSvc.modelGateway = assistantTestSemanticGateway(assistantAdapterFunc(func(_ context.Context, _ string, _ assistantModelProviderConfig) ([]byte, error) {
		followupErrCalls++
		if followupErrCalls == 1 {
			return []byte(`{"action":"create_orgunit","route_kind":"business_action","intent_id":"org.orgunit_create","parent_ref_text":"鲜花组织","entity_name":"运营部","effective_date":"2026-01-01","readiness":"ready_for_dry_run","retrieval_requests":[{"kind":"candidate_lookup","slot":"parent_ref_text","ref_text":"鲜花组织","as_of":"2026-01-01","limit":2}]}`), nil
		}
		return nil, errAssistantModelTimeout
	}))
	if _, err := followupErrSvc.orchestrateSemanticTurn(context.Background(), tenantID, Principal{ID: "actor_followup_err", RoleSlug: "tenant-admin"}, "conv_followup_err", "2026-01-01 在鲜花组织下创建运营部", nil); err != errAssistantModelTimeout {
		t.Fatalf("expected follow-up timeout, got=%v", err)
	}

	carrySelectedSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
	carryCalls := 0
	carrySelectedSvc.modelGateway = assistantTestSemanticGateway(assistantAdapterFunc(func(_ context.Context, _ string, _ assistantModelProviderConfig) ([]byte, error) {
		carryCalls++
		if carryCalls == 1 {
			return []byte(`{"action":"create_orgunit","route_kind":"business_action","intent_id":"org.orgunit_create","parent_ref_text":"鲜花组织","entity_name":"运营部","effective_date":"2026-01-01","selected_candidate_id":"FLOWER-A","readiness":"ready_for_dry_run","retrieval_requests":[{"kind":"candidate_lookup","slot":"parent_ref_text","ref_text":"鲜花组织","as_of":"2026-01-01","limit":2}]}`), nil
		}
		return []byte(`{"action":"create_orgunit","route_kind":"business_action","intent_id":"org.orgunit_create","parent_ref_text":"鲜花组织","entity_name":"运营部","effective_date":"2026-01-01","readiness":"ready_for_confirm"}`), nil
	}))
	carried, err := carrySelectedSvc.orchestrateSemanticTurn(context.Background(), tenantID, Principal{ID: "actor_carry", RoleSlug: "tenant-admin"}, "conv_carry", "2026-01-01 在鲜花组织下创建运营部", nil)
	if err != nil {
		t.Fatalf("carrySelected orchestrate err=%v", err)
	}
	if carried.SelectedCandidateID != "FLOWER-A" || carried.ResolvedCandidateID != "FLOWER-A" || carried.ResolutionSource != assistantResolutionUserConfirmed {
		t.Fatalf("unexpected carried selection resolution=%+v", carried)
	}

	uncertainUpgradeSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
	uncertainUpgradeCalls := 0
	uncertainUpgradeSvc.modelGateway = assistantTestSemanticGateway(assistantAdapterFunc(func(_ context.Context, _ string, _ assistantModelProviderConfig) ([]byte, error) {
		uncertainUpgradeCalls++
		if uncertainUpgradeCalls == 1 {
			return []byte(`{"action":"plan_only","route_kind":"uncertain","intent_id":"route.uncertain","readiness":"need_more_info"}`), nil
		}
		return []byte(`{"action":"plan_only","route_kind":"uncertain","intent_id":"route.uncertain","readiness":"need_more_info"}`), nil
	}))
	uncertainFirst, err := uncertainUpgradeSvc.orchestrateSemanticTurn(context.Background(), tenantID, Principal{ID: "actor_uncertain_upgrade", RoleSlug: "tenant-admin"}, "conv_uncertain_upgrade", "在鲜花组织下创建运营部", nil)
	if err != nil {
		t.Fatalf("uncertain first orchestrate err=%v", err)
	}
	if uncertainFirst.Resolved.Intent.Action != assistantIntentCreateOrgUnit || uncertainFirst.Resolved.Intent.RouteKind != assistantRouteKindBusinessAction {
		t.Fatalf("expected local upgrade on first uncertain turn, got=%+v", uncertainFirst.Resolved.Intent)
	}
	if uncertainFirst.Retrieval.State != assistantSemanticRetrievalStateDeferredByBoundary {
		t.Fatalf("expected deferred lookup before effective date, got=%+v", uncertainFirst.Retrieval)
	}
	if uncertainFirst.ResolvedCandidateID != "" {
		t.Fatalf("expected no candidate before effective date, got=%q", uncertainFirst.ResolvedCandidateID)
	}
	pendingTurn := &assistantTurn{
		UserInput: "在鲜花组织下创建运营部",
		Intent:    uncertainFirst.Resolved.Intent,
		Clarification: &assistantClarificationDecision{
			Status:            assistantClarificationStatusOpen,
			ClarificationKind: assistantClarificationKindMissingSlots,
		},
	}
	uncertainFollowup, err := uncertainUpgradeSvc.orchestrateSemanticTurn(context.Background(), tenantID, Principal{ID: "actor_uncertain_upgrade", RoleSlug: "tenant-admin"}, "conv_uncertain_upgrade", "生效日期 2026-01-01", pendingTurn)
	if err != nil {
		t.Fatalf("uncertain follow-up orchestrate err=%v", err)
	}
	if uncertainFollowup.Resolved.Intent.Action != assistantIntentCreateOrgUnit || uncertainFollowup.Resolved.Intent.EffectiveDate != "2026-01-01" {
		t.Fatalf("expected create follow-up with effective date, got=%+v", uncertainFollowup.Resolved.Intent)
	}
	if uncertainFollowup.Retrieval.State != assistantSemanticRetrievalStateSingleMatch || uncertainFollowup.ResolvedCandidateID != "FLOWER-A" || uncertainFollowup.ResolutionSource != assistantResolutionAuto {
		t.Fatalf("expected candidate resolved after date supplement, got=%+v", uncertainFollowup)
	}
	if uncertainUpgradeCalls != 2 {
		t.Fatalf("expected deferred_by_boundary turn to skip extra follow-up resolve, calls=%d", uncertainUpgradeCalls)
	}

	notRequested, candidates, err := svc.executeSemanticCandidateLookup(context.Background(), tenantID, assistantConversationSemanticState{})
	if err != nil || notRequested.State != assistantSemanticRetrievalStateNotRequested || candidates != nil {
		t.Fatalf("unexpected not-requested lookup result=%+v candidates=%+v err=%v", notRequested, candidates, err)
	}

	deferred, _, err := svc.executeSemanticCandidateLookup(context.Background(), tenantID, assistantConversationSemanticState{
		Slots: assistantIntentSpec{
			Action:        assistantIntentCreateOrgUnit,
			RouteKind:     assistantRouteKindBusinessAction,
			ParentRefText: "鲜花组织",
		},
	})
	if err != nil || deferred.State != assistantSemanticRetrievalStateDeferredByBoundary {
		t.Fatalf("unexpected deferred lookup result=%+v err=%v", deferred, err)
	}
	blankRefResult, _, err := svc.executeSemanticCandidateLookup(context.Background(), tenantID, assistantConversationSemanticState{
		RetrievalRequests: []assistantSemanticRetrievalRequest{{
			Kind: assistantSemanticRetrievalKindCandidateLookup,
			Slot: "parent_ref_text",
		}},
	})
	if err != nil || blankRefResult.State != assistantSemanticRetrievalStateNotRequested {
		t.Fatalf("unexpected blank-ref lookup result=%+v err=%v", blankRefResult, err)
	}

	if got := assistantSemanticConfidence(assistantConversationSemanticState{Readiness: assistantSemanticReadinessReadyForConfirm}, assistantSemanticRetrievalResult{}); got != 0.95 {
		t.Fatalf("unexpected ready_for_confirm confidence=%v", got)
	}
	if got := assistantSemanticConfidence(assistantConversationSemanticState{Readiness: assistantSemanticReadinessReadyForDryRun}, assistantSemanticRetrievalResult{}); got != 0.80 {
		t.Fatalf("unexpected ready_for_dry_run confidence=%v", got)
	}
	if got := assistantSemanticConfidence(assistantConversationSemanticState{Readiness: assistantSemanticReadinessNeedMoreInfo}, assistantSemanticRetrievalResult{}); got != 0.65 {
		t.Fatalf("unexpected need_more_info confidence=%v", got)
	}
	if got := assistantSemanticConfidence(assistantConversationSemanticState{}, assistantSemanticRetrievalResult{State: assistantSemanticRetrievalStateMultipleMatches}); got != 0.55 {
		t.Fatalf("unexpected multiple_matches confidence=%v", got)
	}
	if got := assistantSemanticConfidence(assistantConversationSemanticState{}, assistantSemanticRetrievalResult{State: assistantSemanticRetrievalStateNoMatch}); got != 0.30 {
		t.Fatalf("unexpected no_match confidence=%v", got)
	}
	if got := assistantSemanticConfidence(assistantConversationSemanticState{RouteKind: assistantRouteKindKnowledgeQA}, assistantSemanticRetrievalResult{}); got != 0.30 {
		t.Fatalf("unexpected non-business confidence=%v", got)
	}
	if got := assistantSemanticConfidence(assistantConversationSemanticState{}, assistantSemanticRetrievalResult{State: assistantSemanticRetrievalStateDeferredByBoundary}); got != 0.65 {
		t.Fatalf("unexpected deferred_by_boundary confidence=%v", got)
	}

	explicitRequest, explicit, ok := assistantSemanticCandidateLookupFromState(assistantConversationSemanticState{
		RetrievalRequests: []assistantSemanticRetrievalRequest{{Kind: assistantSemanticRetrievalKindCandidateLookup, Slot: "parent_ref_text", RefText: "鲜花组织", AsOf: "2026-01-01"}},
	})
	if !ok || !explicit || explicitRequest.RefText != "鲜花组织" {
		t.Fatalf("unexpected explicit request=%+v explicit=%v ok=%v", explicitRequest, explicit, ok)
	}
	derivedRequest, explicit, ok := assistantSemanticCandidateLookupFromState(assistantConversationSemanticState{
		Slots: assistantIntentSpec{
			Action:           assistantIntentMoveOrgUnit,
			RouteKind:        assistantRouteKindBusinessAction,
			NewParentRefText: "鲜花组织",
			EffectiveDate:    "2026-01-01",
		},
	})
	if !ok || explicit || derivedRequest.Slot != "new_parent_ref_text" {
		t.Fatalf("unexpected derived request=%+v explicit=%v ok=%v", derivedRequest, explicit, ok)
	}
	fallbackRequest, explicit, ok := assistantSemanticCandidateLookupFromState(assistantConversationSemanticState{
		RetrievalRequests: []assistantSemanticRetrievalRequest{{Kind: "other"}},
		Slots: assistantIntentSpec{
			Action:        assistantIntentCreateOrgUnit,
			RouteKind:     assistantRouteKindBusinessAction,
			ParentRefText: "鲜花组织",
			EffectiveDate: "2026-01-01",
		},
	})
	if !ok || explicit || fallbackRequest.Slot != "parent_ref_text" {
		t.Fatalf("unexpected fallback request=%+v explicit=%v ok=%v", fallbackRequest, explicit, ok)
	}
	if _, explicit, ok := assistantSemanticCandidateLookupFromState(assistantConversationSemanticState{}); ok || explicit {
		t.Fatalf("unexpected empty candidate lookup request explicit=%v ok=%v", explicit, ok)
	}

	autoSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
	autoSvc.modelGateway = assistantTestStaticSemanticGateway(`{"action":"create_orgunit","route_kind":"business_action","intent_id":"org.orgunit_create","parent_ref_text":"鲜花组织","entity_name":"运营部","effective_date":"2026-01-01","readiness":"ready_for_confirm"}`)
	autoResolution, err := autoSvc.orchestrateSemanticTurn(context.Background(), tenantID, Principal{ID: "actor_2", RoleSlug: "tenant-admin"}, "conv_2", "2026-01-01 在鲜花组织下创建运营部", nil)
	if err != nil {
		t.Fatalf("orchestrateSemanticTurn auto err=%v", err)
	}
	if autoResolution.Retrieval.State != assistantSemanticRetrievalStateSingleMatch || autoResolution.ResolvedCandidateID != "FLOWER-A" || autoResolution.ResolutionSource != assistantResolutionAuto {
		t.Fatalf("unexpected auto semantic turn resolution=%+v", autoResolution)
	}

	multiStore := newOrgUnitMemoryStore()
	if _, err := multiStore.CreateNodeCurrent(context.Background(), tenantID, "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
		t.Fatalf("create multi-store node A err=%v", err)
	}
	if _, err := multiStore.CreateNodeCurrent(context.Background(), tenantID, "2026-01-01", "FLOWER-B", "鲜花组织", "", true); err != nil {
		t.Fatalf("create multi-store node B err=%v", err)
	}
	invalidSelectedSvc := newAssistantConversationService(multiStore, assistantWriteServiceStub{store: multiStore})
	invalidSelectedSvc.modelGateway = assistantTestStaticSemanticGateway(`{"action":"create_orgunit","route_kind":"business_action","intent_id":"org.orgunit_create","parent_ref_text":"鲜花组织","entity_name":"运营部","effective_date":"2026-01-01","selected_candidate_id":"missing"}`)
	invalidSelected, err := invalidSelectedSvc.orchestrateSemanticTurn(context.Background(), tenantID, Principal{ID: "actor_invalid_selected", RoleSlug: "tenant-admin"}, "conv_invalid_selected", "2026-01-01 在鲜花组织下创建运营部", nil)
	if err != nil {
		t.Fatalf("invalidSelected orchestrate err=%v", err)
	}
	if invalidSelected.SelectedCandidateID != "" || invalidSelected.ResolvedCandidateID != "" || invalidSelected.AmbiguityCount != 2 {
		t.Fatalf("unexpected invalid-selected resolution=%+v", invalidSelected)
	}

	if got := assistantReplyRenderModelName(nil); got != assistantReplyTargetModelName {
		t.Fatalf("unexpected default reply render model name=%q", got)
	}
	if got := assistantReplyRenderModelName(&assistantTurn{Plan: assistantPlanSummary{ModelName: "gpt-5-codex"}}); got != "gpt-5-codex" {
		t.Fatalf("unexpected turn reply render model name=%q", got)
	}
}

func TestAssistant268TurnActionAPIClosureCoverage(t *testing.T) {
	methodRec := httptest.NewRecorder()
	handleAssistantTurnActionAPI(methodRec, httptest.NewRequest(http.MethodGet, "/internal/assistant/conversations/conv/turns/turn:reply", nil), newAssistantConversationService(nil, nil))
	if methodRec.Code != http.StatusMethodNotAllowed || assistantDecodeErrCode(t, methodRec) != "method_not_allowed" {
		t.Fatalf("unexpected method response=%d body=%s", methodRec.Code, methodRec.Body.String())
	}

	serviceMissingRec := httptest.NewRecorder()
	handleAssistantTurnActionAPI(serviceMissingRec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/conv/turns/turn:reply", `{}`, true, true), nil)
	if serviceMissingRec.Code != http.StatusServiceUnavailable || assistantDecodeErrCode(t, serviceMissingRec) != "assistant_gate_unavailable" {
		t.Fatalf("unexpected service missing response=%d body=%s", serviceMissingRec.Code, serviceMissingRec.Body.String())
	}

	tenantMissingRec := httptest.NewRecorder()
	handleAssistantTurnActionAPI(tenantMissingRec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/conv/turns/turn:reply", `{}`, false, true), newAssistantConversationService(nil, nil))
	if tenantMissingRec.Code != http.StatusInternalServerError || assistantDecodeErrCode(t, tenantMissingRec) != "tenant_missing" {
		t.Fatalf("unexpected tenant missing response=%d body=%s", tenantMissingRec.Code, tenantMissingRec.Body.String())
	}

	principalMissingRec := httptest.NewRecorder()
	handleAssistantTurnActionAPI(principalMissingRec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/conv/turns/turn:reply", `{}`, true, false), newAssistantConversationService(nil, nil))
	if principalMissingRec.Code != http.StatusUnauthorized || assistantDecodeErrCode(t, principalMissingRec) != "unauthorized" {
		t.Fatalf("unexpected principal missing response=%d body=%s", principalMissingRec.Code, principalMissingRec.Body.String())
	}
}

func TestAssistant268TransitionPhaseClosureCoverage(t *testing.T) {
	cases := []struct {
		state     string
		reason    string
		turnPhase string
		from      bool
		want      string
	}{
		{reason: "turn_created", from: true, want: assistantPhaseIdle},
		{reason: "confirmed", from: false, want: assistantPhaseAwaitCommitConfirm},
		{reason: "committed", from: true, want: assistantPhaseAwaitCommitConfirm},
	}
	for _, tc := range cases {
		if got := assistantTransitionPhaseValue(tc.state, tc.reason, tc.turnPhase, tc.from); got != tc.want {
			t.Fatalf("transition phase mismatch state=%q reason=%q from=%v got=%q want=%q", tc.state, tc.reason, tc.from, got, tc.want)
		}
	}
}
