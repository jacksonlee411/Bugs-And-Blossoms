package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleInternalRulesEvaluateAPI(t *testing.T) {
	resetSetIDStrategyRegistryRuntimeForTest()
	t.Cleanup(resetSetIDStrategyRegistryRuntimeForTest)
	resetFunctionalAreaSwitchStoreForTest()
	t.Cleanup(resetFunctionalAreaSwitchStoreForTest)

	orgNodeKeyA := mustOrgNodeKeyForTest(t, 10000001)
	orgNodeKeyB := mustOrgNodeKeyForTest(t, 10000002)
	store := scopeAPIStore{
		resolveSetIDFn: func(_ context.Context, _ string, orgUnitID string, _ string) (string, error) {
			switch orgUnitID {
			case "10000001", orgNodeKeyA:
				return "A0001", nil
			case "10000002", orgNodeKeyB:
				return "B0001", nil
			default:
				return "", errors.New("SETID_NOT_FOUND")
			}
		},
	}
	orgResolver := setIDExplainOrgResolverStub{
		byCode: map[string]string{
			"BU-001": orgNodeKeyA,
			"BU-002": orgNodeKeyB,
			"BU-999": mustOrgNodeKeyForTest(t, 99999999),
		},
	}

	tenantItem, _ := defaultSetIDStrategyRegistryRuntime.upsert("t1", setIDStrategyRegistryItem{
		CapabilityKey:       "staffing.assignment_create.field_policy",
		OwnerModule:         "staffing",
		FieldKey:            "field_x",
		PersonalizationMode: personalizationModeTenantOnly,
		OrgApplicability:    orgApplicabilityTenant,
		BusinessUnitNodeKey: "",
		Required:            false,
		Visible:             true,
		DefaultRuleRef:      "rule://tenant",
		DefaultValue:        "tenant",
		ExplainRequired:     true,
		EffectiveDate:       "2026-01-01",
		Priority:            100,
	})
	buAItem, _ := defaultSetIDStrategyRegistryRuntime.upsert("t1", setIDStrategyRegistryItem{
		CapabilityKey:       "staffing.assignment_create.field_policy",
		OwnerModule:         "staffing",
		FieldKey:            "field_x",
		PersonalizationMode: personalizationModeSetID,
		OrgApplicability:    orgApplicabilityBusinessUnit,
		BusinessUnitNodeKey: orgNodeKeyA,
		ResolvedSetID:       "A0001",
		Required:            true,
		Visible:             true,
		DefaultRuleRef:      "rule://bu-a",
		DefaultValue:        "bu-a",
		ExplainRequired:     true,
		EffectiveDate:       "2026-01-01",
		Priority:            200,
	})
	hiddenItem, _ := defaultSetIDStrategyRegistryRuntime.upsert("t1", setIDStrategyRegistryItem{
		CapabilityKey:       "staffing.assignment_create.field_policy",
		OwnerModule:         "staffing",
		FieldKey:            "field_hidden",
		PersonalizationMode: personalizationModeSetID,
		OrgApplicability:    orgApplicabilityBusinessUnit,
		BusinessUnitNodeKey: orgNodeKeyA,
		ResolvedSetID:       "A0001",
		Required:            false,
		Visible:             false,
		DefaultRuleRef:      "rule://hidden",
		DefaultValue:        "hidden",
		ExplainRequired:     true,
		EffectiveDate:       "2026-01-01",
		Priority:            300,
	})

	makeReq := func(body string) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/internal/rules/evaluate", bytes.NewBufferString(body))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		return req
	}

	recNoTenant := httptest.NewRecorder()
	handleInternalRulesEvaluateAPI(recNoTenant, httptest.NewRequest(http.MethodPost, "/internal/rules/evaluate", bytes.NewBufferString(`{}`)), store, orgResolver)
	if recNoTenant.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", recNoTenant.Code)
	}

	recMethod := httptest.NewRecorder()
	reqMethod := httptest.NewRequest(http.MethodGet, "/internal/rules/evaluate", nil)
	reqMethod = reqMethod.WithContext(withTenant(reqMethod.Context(), Tenant{ID: "t1"}))
	handleInternalRulesEvaluateAPI(recMethod, reqMethod, store, orgResolver)
	if recMethod.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", recMethod.Code)
	}

	recStoreMissing := httptest.NewRecorder()
	reqStoreMissing := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_org_code":"BU-001","as_of":"2026-01-01"}`)
	reqStoreMissing = reqStoreMissing.WithContext(withPrincipal(reqStoreMissing.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recStoreMissing, reqStoreMissing, nil, orgResolver)
	if recStoreMissing.Code != http.StatusInternalServerError || !strings.Contains(recStoreMissing.Body.String(), "setid_resolver_missing") {
		t.Fatalf("status=%d body=%s", recStoreMissing.Code, recStoreMissing.Body.String())
	}

	recForbidden := httptest.NewRecorder()
	reqForbidden := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_org_code":"BU-001","as_of":"2026-01-01"}`)
	handleInternalRulesEvaluateAPI(recForbidden, reqForbidden, store, orgResolver)
	if recForbidden.Code != http.StatusForbidden || !strings.Contains(recForbidden.Body.String(), scopeReasonActorScopeForbidden) {
		t.Fatalf("status=%d body=%s", recForbidden.Code, recForbidden.Body.String())
	}

	recBadRequest := httptest.NewRecorder()
	reqBadRequest := makeReq(`{"capability_key":"","field_key":"","business_unit_org_code":"","as_of":""}`)
	reqBadRequest = reqBadRequest.WithContext(withPrincipal(reqBadRequest.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recBadRequest, reqBadRequest, store, orgResolver)
	if recBadRequest.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", recBadRequest.Code)
	}

	recBadJSON := httptest.NewRecorder()
	reqBadJSON := makeReq(`{`)
	reqBadJSON = reqBadJSON.WithContext(withPrincipal(reqBadJSON.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recBadJSON, reqBadJSON, store, orgResolver)
	if recBadJSON.Code != http.StatusBadRequest || !strings.Contains(recBadJSON.Body.String(), `"code":"bad_json"`) {
		t.Fatalf("status=%d body=%s", recBadJSON.Code, recBadJSON.Body.String())
	}

	recInvalidField := httptest.NewRecorder()
	reqInvalidField := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"bad key","business_unit_org_code":"BU-001","as_of":"2026-01-01"}`)
	reqInvalidField = reqInvalidField.WithContext(withPrincipal(reqInvalidField.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recInvalidField, reqInvalidField, store, orgResolver)
	if recInvalidField.Code != http.StatusBadRequest || !strings.Contains(recInvalidField.Body.String(), `"code":"invalid_request"`) {
		t.Fatalf("status=%d body=%s", recInvalidField.Code, recInvalidField.Body.String())
	}

	recInvalidAsOf := httptest.NewRecorder()
	reqInvalidAsOf := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_org_code":"BU-001","as_of":"2026-13-01"}`)
	reqInvalidAsOf = reqInvalidAsOf.WithContext(withPrincipal(reqInvalidAsOf.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recInvalidAsOf, reqInvalidAsOf, store, orgResolver)
	if recInvalidAsOf.Code != http.StatusBadRequest || !strings.Contains(recInvalidAsOf.Body.String(), `"code":"invalid_as_of"`) {
		t.Fatalf("status=%d body=%s", recInvalidAsOf.Code, recInvalidAsOf.Body.String())
	}

	recInvalidBU := httptest.NewRecorder()
	reqInvalidBU := makeReq("{\"capability_key\":\"staffing.assignment_create.field_policy\",\"field_key\":\"field_x\",\"business_unit_org_code\":\"bad\\u007f\",\"as_of\":\"2026-01-01\"}")
	reqInvalidBU = reqInvalidBU.WithContext(withPrincipal(reqInvalidBU.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recInvalidBU, reqInvalidBU, store, orgResolver)
	if recInvalidBU.Code != http.StatusBadRequest || !strings.Contains(recInvalidBU.Body.String(), `"code":"business_unit_org_code_invalid"`) {
		t.Fatalf("status=%d body=%s", recInvalidBU.Code, recInvalidBU.Body.String())
	}

	recAreaMissing := httptest.NewRecorder()
	reqAreaMissing := makeReq(`{"capability_key":"unknown.key","field_key":"field_x","business_unit_org_code":"BU-001","as_of":"2026-01-01"}`)
	reqAreaMissing = reqAreaMissing.WithContext(withPrincipal(reqAreaMissing.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recAreaMissing, reqAreaMissing, store, orgResolver)
	if recAreaMissing.Code != http.StatusForbidden || !strings.Contains(recAreaMissing.Body.String(), functionalAreaMissingCode) {
		t.Fatalf("status=%d body=%s", recAreaMissing.Code, recAreaMissing.Body.String())
	}

	defaultFunctionalAreaSwitchStore.setEnabled("t1", "staffing", false)
	recAreaDisabled := httptest.NewRecorder()
	reqAreaDisabled := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_org_code":"BU-001","as_of":"2026-01-01"}`)
	reqAreaDisabled = reqAreaDisabled.WithContext(withPrincipal(reqAreaDisabled.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recAreaDisabled, reqAreaDisabled, store, orgResolver)
	if recAreaDisabled.Code != http.StatusForbidden || !strings.Contains(recAreaDisabled.Body.String(), functionalAreaDisabledCode) {
		t.Fatalf("status=%d body=%s", recAreaDisabled.Code, recAreaDisabled.Body.String())
	}
	defaultFunctionalAreaSwitchStore.setEnabled("t1", "staffing", true)

	recInvalidTarget := httptest.NewRecorder()
	reqInvalidTarget := makeReq("{\"capability_key\":\"staffing.assignment_create.field_policy\",\"field_key\":\"field_x\",\"business_unit_org_code\":\"BU-001\",\"org_code\":\"bad\\u007f\",\"as_of\":\"2026-01-01\"}")
	reqInvalidTarget = reqInvalidTarget.WithContext(withPrincipal(reqInvalidTarget.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recInvalidTarget, reqInvalidTarget, store, orgResolver)
	if recInvalidTarget.Code != http.StatusBadRequest || !strings.Contains(recInvalidTarget.Body.String(), `"code":"org_code_invalid"`) {
		t.Fatalf("status=%d body=%s", recInvalidTarget.Code, recInvalidTarget.Body.String())
	}

	recContextMismatch := httptest.NewRecorder()
	reqContextMismatch := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_org_code":"BU-001","as_of":"2026-01-01"}`)
	reqContextMismatch.Header.Set("X-Actor-Scope", "saas")
	reqContextMismatch = reqContextMismatch.WithContext(withPrincipal(reqContextMismatch.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recContextMismatch, reqContextMismatch, store, orgResolver)
	if recContextMismatch.Code != http.StatusForbidden || !strings.Contains(recContextMismatch.Body.String(), capabilityReasonContextMismatch) {
		t.Fatalf("status=%d body=%s", recContextMismatch.Code, recContextMismatch.Body.String())
	}

	recBUA := httptest.NewRecorder()
	reqBUA := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_org_code":"BU-001","as_of":"2026-01-01","request_id":"req-bu-a"}`)
	reqBUA = reqBUA.WithContext(withPrincipal(reqBUA.Context(), Principal{ID: "p1", RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recBUA, reqBUA, store, orgResolver)
	if recBUA.Code != http.StatusOK ||
		!strings.Contains(recBUA.Body.String(), `"decision":"allow"`) ||
		!strings.Contains(recBUA.Body.String(), `"reason_code":"`+fieldRequiredInContextCode+`"`) ||
		!strings.Contains(recBUA.Body.String(), `"functional_area_key":"staffing"`) ||
		!strings.Contains(recBUA.Body.String(), `"setid":"A0001"`) ||
		!strings.Contains(recBUA.Body.String(), `"selected_rule_id":"`+strategyRegistryPolicyID(buAItem)+`"`) {
		t.Fatalf("status=%d body=%s", recBUA.Code, recBUA.Body.String())
	}

	recBUB := httptest.NewRecorder()
	reqBUB := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_org_code":"BU-002","as_of":"2026-01-01","request_id":"req-bu-b"}`)
	reqBUB = reqBUB.WithContext(withPrincipal(reqBUB.Context(), Principal{ID: "p1", RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recBUB, reqBUB, store, orgResolver)
	if recBUB.Code != http.StatusOK ||
		!strings.Contains(recBUB.Body.String(), `"decision":"allow"`) ||
		!strings.Contains(recBUB.Body.String(), `"reason_code":"`+fieldVisibleInContextCode+`"`) ||
		!strings.Contains(recBUB.Body.String(), `"setid":"B0001"`) ||
		!strings.Contains(recBUB.Body.String(), `"selected_rule_id":"`+strategyRegistryPolicyID(tenantItem)+`"`) {
		t.Fatalf("status=%d body=%s", recBUB.Code, recBUB.Body.String())
	}

	recHidden := httptest.NewRecorder()
	reqHidden := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_hidden","business_unit_org_code":"BU-001","as_of":"2026-01-01","request_id":"req-hidden"}`)
	reqHidden = reqHidden.WithContext(withPrincipal(reqHidden.Context(), Principal{ID: "p1", RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recHidden, reqHidden, store, orgResolver)
	if recHidden.Code != http.StatusOK ||
		!strings.Contains(recHidden.Body.String(), `"decision":"deny"`) ||
		!strings.Contains(recHidden.Body.String(), `"reason_code":"`+fieldHiddenInContextCode+`"`) ||
		!strings.Contains(recHidden.Body.String(), `"selected_rule_id":"`+strategyRegistryPolicyID(hiddenItem)+`"`) {
		t.Fatalf("status=%d body=%s", recHidden.Code, recHidden.Body.String())
	}

	recResolveErr := httptest.NewRecorder()
	reqResolveErr := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_org_code":"BU-999","as_of":"2026-01-01","request_id":"req-resolve-err"}`)
	reqResolveErr = reqResolveErr.WithContext(withPrincipal(reqResolveErr.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recResolveErr, reqResolveErr, store, orgResolver)
	if recResolveErr.Code != http.StatusForbidden || !strings.Contains(recResolveErr.Body.String(), capabilityReasonContextMismatch) {
		t.Fatalf("status=%d body=%s", recResolveErr.Code, recResolveErr.Body.String())
	}

	storeEmptySetID := scopeAPIStore{
		resolveSetIDFn: func(context.Context, string, string, string) (string, error) {
			return "   ", nil
		},
	}
	recEmptySetID := httptest.NewRecorder()
	reqEmptySetID := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_org_code":"BU-001","as_of":"2026-01-01","request_id":"req-empty-setid"}`)
	reqEmptySetID = reqEmptySetID.WithContext(withPrincipal(reqEmptySetID.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recEmptySetID, reqEmptySetID, storeEmptySetID, orgResolver)
	if recEmptySetID.Code != http.StatusForbidden || !strings.Contains(recEmptySetID.Body.String(), capabilityReasonContextMismatch) {
		t.Fatalf("status=%d body=%s", recEmptySetID.Code, recEmptySetID.Body.String())
	}

	prevCanView := canViewInternalRulesEvaluate
	canViewInternalRulesEvaluate = func(context.Context) bool { return true }
	t.Cleanup(func() { canViewInternalRulesEvaluate = prevCanView })

	recDynamicMismatch := httptest.NewRecorder()
	reqDynamicMismatch := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_org_code":"BU-001","org_code":"BU-002","as_of":"2026-01-01","request_id":"req-dyn-mismatch"}`)
	reqDynamicMismatch = reqDynamicMismatch.WithContext(withPrincipal(reqDynamicMismatch.Context(), Principal{RoleSlug: "tenant-viewer"}))
	handleInternalRulesEvaluateAPI(recDynamicMismatch, reqDynamicMismatch, store, orgResolver)
	if recDynamicMismatch.Code != http.StatusForbidden || !strings.Contains(recDynamicMismatch.Body.String(), capabilityReasonContextMismatch) {
		t.Fatalf("status=%d body=%s", recDynamicMismatch.Code, recDynamicMismatch.Body.String())
	}

	previousStore := defaultSetIDStrategyRegistryStore
	t.Cleanup(func() { useSetIDStrategyRegistryStore(previousStore) })

	useSetIDStrategyRegistryStore(setIDStrategyRegistryStoreStub{
		listFn: func(context.Context, string, string, string, string) ([]setIDStrategyRegistryItem, error) {
			return nil, errors.New("invalid as_of")
		},
	})
	recListInvalidAsOf := httptest.NewRecorder()
	reqListInvalidAsOf := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_org_code":"BU-001","as_of":"2026-01-01","request_id":"req-list-asof"}`)
	reqListInvalidAsOf = reqListInvalidAsOf.WithContext(withPrincipal(reqListInvalidAsOf.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recListInvalidAsOf, reqListInvalidAsOf, store, orgResolver)
	if recListInvalidAsOf.Code != http.StatusInternalServerError || !strings.Contains(recListInvalidAsOf.Body.String(), `"code":"internal_error"`) {
		t.Fatalf("status=%d body=%s", recListInvalidAsOf.Code, recListInvalidAsOf.Body.String())
	}

	useSetIDStrategyRegistryStore(setIDStrategyRegistryStoreStub{
		listFn: func(context.Context, string, string, string, string) ([]setIDStrategyRegistryItem, error) {
			return nil, errors.New("boom")
		},
	})
	recListInternalErr := httptest.NewRecorder()
	reqListInternalErr := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_org_code":"BU-001","as_of":"2026-01-01","request_id":"req-list-internal"}`)
	reqListInternalErr = reqListInternalErr.WithContext(withPrincipal(reqListInternalErr.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recListInternalErr, reqListInternalErr, store, orgResolver)
	if recListInternalErr.Code != http.StatusInternalServerError || !strings.Contains(recListInternalErr.Body.String(), `"code":"internal_error"`) {
		t.Fatalf("status=%d body=%s", recListInternalErr.Code, recListInternalErr.Body.String())
	}
	useSetIDStrategyRegistryStore(previousStore)

	recMissingPolicy := httptest.NewRecorder()
	reqMissingPolicy := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"missing_field","business_unit_org_code":"BU-001","as_of":"2026-01-01","request_id":"req-missing"}`)
	reqMissingPolicy = reqMissingPolicy.WithContext(withPrincipal(reqMissingPolicy.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recMissingPolicy, reqMissingPolicy, store, orgResolver)
	if recMissingPolicy.Code != http.StatusOK ||
		!strings.Contains(recMissingPolicy.Body.String(), `"decision":"deny"`) ||
		!strings.Contains(recMissingPolicy.Body.String(), `"reason_code":"`+fieldPolicyMissingCode+`"`) {
		t.Fatalf("status=%d body=%s", recMissingPolicy.Code, recMissingPolicy.Body.String())
	}

	recAutoIDs := httptest.NewRecorder()
	reqAutoIDs := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_org_code":"BU-001","as_of":"2026-01-01"}`)
	reqAutoIDs = reqAutoIDs.WithContext(withPrincipal(reqAutoIDs.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recAutoIDs, reqAutoIDs, store, orgResolver)
	if recAutoIDs.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recAutoIDs.Code, recAutoIDs.Body.String())
	}
	var autoResp internalRulesEvaluateResponse
	if err := json.Unmarshal(recAutoIDs.Body.Bytes(), &autoResp); err != nil {
		t.Fatalf("unmarshal=%v body=%s", err, recAutoIDs.Body.String())
	}
	if autoResp.RequestID != "setid-explain-auto" {
		t.Fatalf("request_id=%q", autoResp.RequestID)
	}
	if len(autoResp.TraceID) != 32 {
		t.Fatalf("trace_id=%q", autoResp.TraceID)
	}
}

func TestResolveInternalRulesEvaluation(t *testing.T) {
	buNodeKey := mustOrgNodeKeyForTest(t, 10000001)
	items := []setIDStrategyRegistryItem{
		{
			CapabilityKey:    "staffing.assignment_create.field_policy",
			FieldKey:         "field_x",
			OrgApplicability: orgApplicabilityTenant,
			Required:         false,
			Visible:          true,
			Maintainable:     true,
			DefaultValue:     "tenant",
			Priority:         100,
			EffectiveDate:    "2026-01-01",
			UpdatedAt:        "2026-01-01T00:00:00Z",
		},
		{
			CapabilityKey:       "staffing.assignment_create.field_policy",
			FieldKey:            "field_x",
			OrgApplicability:    orgApplicabilityBusinessUnit,
			ResolvedSetID:       "A0001",
			BusinessUnitNodeKey: buNodeKey,
			Required:            true,
			Visible:             true,
			Maintainable:        true,
			DefaultValue:        "bu",
			Priority:            200,
			EffectiveDate:       "2026-01-01",
			UpdatedAt:           "2026-01-02T00:00:00Z",
		},
	}

	evaluation, err := resolveInternalRulesEvaluation(items, "staffing.assignment_create.field_policy", "field_x", "A0001", buNodeKey)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if evaluation.Decision != internalRuleDecisionAllow || evaluation.ReasonCode != fieldRequiredInContextCode || evaluation.Resolution == nil {
		t.Fatalf("evaluation=%+v", evaluation)
	}
	selected := internalRuleCandidateFromResolution(evaluation.Resolution, evaluation.Decision, evaluation.ReasonCode)
	if selected == nil || selected.RuleID != evaluation.Resolution.PrimaryPolicyID || selected.Priority != 200 {
		t.Fatalf("selected=%+v resolution=%+v", selected, evaluation.Resolution)
	}

	missing, err := resolveInternalRulesEvaluation(nil, "staffing.assignment_create.field_policy", "missing", "A0001", buNodeKey)
	if err != nil {
		t.Fatalf("missing err=%v", err)
	}
	if missing.Decision != internalRuleDecisionDeny || missing.ReasonCode != fieldPolicyMissingCode || missing.Resolution != nil {
		t.Fatalf("missing=%+v", missing)
	}

	conflict, err := resolveInternalRulesEvaluation([]setIDStrategyRegistryItem{{
		CapabilityKey:       "staffing.assignment_create.field_policy",
		FieldKey:            "field_conflict",
		OrgApplicability:    orgApplicabilityBusinessUnit,
		ResolvedSetID:       "A0001",
		BusinessUnitNodeKey: buNodeKey,
		Required:            true,
		Visible:             false,
		Maintainable:        true,
		Priority:            1,
		EffectiveDate:       "2026-01-01",
		UpdatedAt:           "2026-01-01T00:00:00Z",
	}}, "staffing.assignment_create.field_policy", "field_conflict", "A0001", buNodeKey)
	if err != nil {
		t.Fatalf("conflict err=%v", err)
	}
	if conflict.Decision != internalRuleDecisionDeny || conflict.ReasonCode != fieldPolicyConflictCode || conflict.Resolution != nil {
		t.Fatalf("conflict=%+v", conflict)
	}
}

func TestInternalRuleDecisionFromError(t *testing.T) {
	if decision, reasonCode, ok := internalRuleDecisionFromError(errors.New(fieldPolicyMissingCode)); !ok || decision != internalRuleDecisionDeny || reasonCode != fieldPolicyMissingCode {
		t.Fatalf("decision=%q reason_code=%q ok=%v", decision, reasonCode, ok)
	}
	if _, _, ok := internalRuleDecisionFromError(errors.New("boom")); ok {
		t.Fatal("unexpected handled error")
	}
}

func TestInternalRuleBriefExplain(t *testing.T) {
	if got := internalRuleBriefExplain(nil, 0); got != "no eligible rule candidate" {
		t.Fatalf("got=%q", got)
	}
	if got := internalRuleBriefExplain(&internalRuleCandidate{RuleID: "r1", Priority: 10}, 2); got != "selected r1 (priority=10, matched=2)" {
		t.Fatalf("got=%q", got)
	}
}
