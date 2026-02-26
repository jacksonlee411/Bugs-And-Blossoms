package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/cel-go/cel"
)

func TestHandleInternalRulesEvaluateAPI(t *testing.T) {
	resetSetIDStrategyRegistryRuntimeForTest()
	t.Cleanup(resetSetIDStrategyRegistryRuntimeForTest)
	resetFunctionalAreaSwitchStoreForTest()
	t.Cleanup(resetFunctionalAreaSwitchStoreForTest)
	internalRuleEligibilityProgramCache = sync.Map{}
	internalRuleDecisionProgramCache = sync.Map{}

	store := scopeAPIStore{
		resolveSetIDFn: func(_ context.Context, _ string, orgUnitID string, _ string) (string, error) {
			switch orgUnitID {
			case "10000001":
				return "A0001", nil
			case "10000002":
				return "B0001", nil
			default:
				return "", errors.New("SETID_NOT_FOUND")
			}
		},
	}

	_, _ = defaultSetIDStrategyRegistryRuntime.upsert("t1", setIDStrategyRegistryItem{
		CapabilityKey:       "staffing.assignment_create.field_policy",
		OwnerModule:         "staffing",
		FieldKey:            "field_x",
		PersonalizationMode: personalizationModeTenantOnly,
		OrgApplicability:    orgApplicabilityTenant,
		BusinessUnitID:      "",
		Required:            false,
		Visible:             true,
		DefaultRuleRef:      "rule://tenant",
		DefaultValue:        "tenant",
		ExplainRequired:     true,
		EffectiveDate:       "2026-01-01",
		Priority:            100,
	})
	_, _ = defaultSetIDStrategyRegistryRuntime.upsert("t1", setIDStrategyRegistryItem{
		CapabilityKey:       "staffing.assignment_create.field_policy",
		OwnerModule:         "staffing",
		FieldKey:            "field_x",
		PersonalizationMode: personalizationModeSetID,
		OrgApplicability:    orgApplicabilityBusinessUnit,
		BusinessUnitID:      "10000001",
		Required:            true,
		Visible:             true,
		DefaultRuleRef:      "rule://bu-a",
		DefaultValue:        "bu-a",
		ExplainRequired:     true,
		EffectiveDate:       "2026-01-01",
		Priority:            200,
	})
	_, _ = defaultSetIDStrategyRegistryRuntime.upsert("t1", setIDStrategyRegistryItem{
		CapabilityKey:       "staffing.assignment_create.field_policy",
		OwnerModule:         "staffing",
		FieldKey:            "field_hidden",
		PersonalizationMode: personalizationModeSetID,
		OrgApplicability:    orgApplicabilityBusinessUnit,
		BusinessUnitID:      "10000001",
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
	handleInternalRulesEvaluateAPI(recNoTenant, httptest.NewRequest(http.MethodPost, "/internal/rules/evaluate", bytes.NewBufferString(`{}`)), store)
	if recNoTenant.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", recNoTenant.Code)
	}

	recMethod := httptest.NewRecorder()
	reqMethod := httptest.NewRequest(http.MethodGet, "/internal/rules/evaluate", nil)
	reqMethod = reqMethod.WithContext(withTenant(reqMethod.Context(), Tenant{ID: "t1"}))
	handleInternalRulesEvaluateAPI(recMethod, reqMethod, store)
	if recMethod.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", recMethod.Code)
	}

	recStoreMissing := httptest.NewRecorder()
	reqStoreMissing := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_id":"10000001","as_of":"2026-01-01"}`)
	reqStoreMissing = reqStoreMissing.WithContext(withPrincipal(reqStoreMissing.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recStoreMissing, reqStoreMissing, nil)
	if recStoreMissing.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", recStoreMissing.Code)
	}
	if !strings.Contains(recStoreMissing.Body.String(), "setid_resolver_missing") {
		t.Fatalf("unexpected body: %q", recStoreMissing.Body.String())
	}

	recForbidden := httptest.NewRecorder()
	reqForbidden := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_id":"10000001","as_of":"2026-01-01"}`)
	handleInternalRulesEvaluateAPI(recForbidden, reqForbidden, store)
	if recForbidden.Code != http.StatusForbidden {
		t.Fatalf("status=%d", recForbidden.Code)
	}
	if !strings.Contains(recForbidden.Body.String(), scopeReasonActorScopeForbidden) {
		t.Fatalf("unexpected body: %q", recForbidden.Body.String())
	}

	recBadRequest := httptest.NewRecorder()
	reqBadRequest := makeReq(`{"capability_key":"","field_key":"","business_unit_id":"","as_of":""}`)
	reqBadRequest = reqBadRequest.WithContext(withPrincipal(reqBadRequest.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recBadRequest, reqBadRequest, store)
	if recBadRequest.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", recBadRequest.Code)
	}

	recBadJSON := httptest.NewRecorder()
	reqBadJSON := makeReq(`{`)
	reqBadJSON = reqBadJSON.WithContext(withPrincipal(reqBadJSON.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recBadJSON, reqBadJSON, store)
	if recBadJSON.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", recBadJSON.Code)
	}
	if !strings.Contains(recBadJSON.Body.String(), `"code":"bad_json"`) {
		t.Fatalf("unexpected body: %q", recBadJSON.Body.String())
	}

	recInvalidField := httptest.NewRecorder()
	reqInvalidField := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"bad key","business_unit_id":"10000001","as_of":"2026-01-01"}`)
	reqInvalidField = reqInvalidField.WithContext(withPrincipal(reqInvalidField.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recInvalidField, reqInvalidField, store)
	if recInvalidField.Code != http.StatusBadRequest || !strings.Contains(recInvalidField.Body.String(), `"code":"invalid_request"`) {
		t.Fatalf("status=%d body=%s", recInvalidField.Code, recInvalidField.Body.String())
	}

	recInvalidAsOf := httptest.NewRecorder()
	reqInvalidAsOf := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_id":"10000001","as_of":"2026-13-01"}`)
	reqInvalidAsOf = reqInvalidAsOf.WithContext(withPrincipal(reqInvalidAsOf.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recInvalidAsOf, reqInvalidAsOf, store)
	if recInvalidAsOf.Code != http.StatusBadRequest || !strings.Contains(recInvalidAsOf.Body.String(), `"code":"invalid_as_of"`) {
		t.Fatalf("status=%d body=%s", recInvalidAsOf.Code, recInvalidAsOf.Body.String())
	}

	recInvalidBU := httptest.NewRecorder()
	reqInvalidBU := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_id":"bad","as_of":"2026-01-01"}`)
	reqInvalidBU = reqInvalidBU.WithContext(withPrincipal(reqInvalidBU.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recInvalidBU, reqInvalidBU, store)
	if recInvalidBU.Code != http.StatusBadRequest || !strings.Contains(recInvalidBU.Body.String(), `"code":"invalid_business_unit_id"`) {
		t.Fatalf("status=%d body=%s", recInvalidBU.Code, recInvalidBU.Body.String())
	}

	recAreaMissing := httptest.NewRecorder()
	reqAreaMissing := makeReq(`{"capability_key":"unknown.key","field_key":"field_x","business_unit_id":"10000001","as_of":"2026-01-01"}`)
	reqAreaMissing = reqAreaMissing.WithContext(withPrincipal(reqAreaMissing.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recAreaMissing, reqAreaMissing, store)
	if recAreaMissing.Code != http.StatusForbidden || !strings.Contains(recAreaMissing.Body.String(), functionalAreaMissingCode) {
		t.Fatalf("status=%d body=%s", recAreaMissing.Code, recAreaMissing.Body.String())
	}

	defaultFunctionalAreaSwitchStore.setEnabled("t1", "staffing", false)
	recAreaDisabled := httptest.NewRecorder()
	reqAreaDisabled := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_id":"10000001","as_of":"2026-01-01"}`)
	reqAreaDisabled = reqAreaDisabled.WithContext(withPrincipal(reqAreaDisabled.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recAreaDisabled, reqAreaDisabled, store)
	if recAreaDisabled.Code != http.StatusForbidden || !strings.Contains(recAreaDisabled.Body.String(), functionalAreaDisabledCode) {
		t.Fatalf("status=%d body=%s", recAreaDisabled.Code, recAreaDisabled.Body.String())
	}
	defaultFunctionalAreaSwitchStore.setEnabled("t1", "staffing", true)

	recInvalidTarget := httptest.NewRecorder()
	reqInvalidTarget := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_id":"10000001","target_org_unit_id":"bad","as_of":"2026-01-01"}`)
	reqInvalidTarget = reqInvalidTarget.WithContext(withPrincipal(reqInvalidTarget.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recInvalidTarget, reqInvalidTarget, store)
	if recInvalidTarget.Code != http.StatusBadRequest || !strings.Contains(recInvalidTarget.Body.String(), `"code":"invalid_org_unit_id"`) {
		t.Fatalf("status=%d body=%s", recInvalidTarget.Code, recInvalidTarget.Body.String())
	}

	recContextMismatch := httptest.NewRecorder()
	reqContextMismatch := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_id":"10000001","as_of":"2026-01-01"}`)
	reqContextMismatch.Header.Set("X-Actor-Scope", "saas")
	reqContextMismatch = reqContextMismatch.WithContext(withPrincipal(reqContextMismatch.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recContextMismatch, reqContextMismatch, store)
	if recContextMismatch.Code != http.StatusForbidden {
		t.Fatalf("status=%d", recContextMismatch.Code)
	}
	if !strings.Contains(recContextMismatch.Body.String(), capabilityReasonContextMismatch) {
		t.Fatalf("unexpected body: %q", recContextMismatch.Body.String())
	}

	recBUA := httptest.NewRecorder()
	reqBUA := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_id":"10000001","as_of":"2026-01-01","request_id":"req-bu-a"}`)
	reqBUA = reqBUA.WithContext(withPrincipal(reqBUA.Context(), Principal{ID: "p1", RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recBUA, reqBUA, store)
	if recBUA.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recBUA.Code, recBUA.Body.String())
	}
	if !strings.Contains(recBUA.Body.String(), `"decision":"allow"`) ||
		!strings.Contains(recBUA.Body.String(), `"reason_code":"`+fieldRequiredInContextCode+`"`) ||
		!strings.Contains(recBUA.Body.String(), `"functional_area_key":"staffing"`) ||
		!strings.Contains(recBUA.Body.String(), `"setid":"A0001"`) ||
		!strings.Contains(recBUA.Body.String(), `"selected_rule_id":"staffing.assignment_create.field_policy|field_x|business_unit|10000001|2026-01-01"`) {
		t.Fatalf("unexpected body: %q", recBUA.Body.String())
	}

	recBUB := httptest.NewRecorder()
	reqBUB := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_id":"10000002","as_of":"2026-01-01","request_id":"req-bu-b"}`)
	reqBUB = reqBUB.WithContext(withPrincipal(reqBUB.Context(), Principal{ID: "p1", RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recBUB, reqBUB, store)
	if recBUB.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recBUB.Code, recBUB.Body.String())
	}
	if !strings.Contains(recBUB.Body.String(), `"decision":"allow"`) ||
		!strings.Contains(recBUB.Body.String(), `"reason_code":"`+fieldVisibleInContextCode+`"`) ||
		!strings.Contains(recBUB.Body.String(), `"setid":"B0001"`) ||
		!strings.Contains(recBUB.Body.String(), `"selected_rule_id":"staffing.assignment_create.field_policy|field_x|tenant||2026-01-01"`) {
		t.Fatalf("unexpected body: %q", recBUB.Body.String())
	}

	recHidden := httptest.NewRecorder()
	reqHidden := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_hidden","business_unit_id":"10000001","as_of":"2026-01-01","request_id":"req-hidden"}`)
	reqHidden = reqHidden.WithContext(withPrincipal(reqHidden.Context(), Principal{ID: "p1", RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recHidden, reqHidden, store)
	if recHidden.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recHidden.Code, recHidden.Body.String())
	}
	if !strings.Contains(recHidden.Body.String(), `"decision":"deny"`) ||
		!strings.Contains(recHidden.Body.String(), `"reason_code":"`+fieldHiddenInContextCode+`"`) {
		t.Fatalf("unexpected body: %q", recHidden.Body.String())
	}

	recResolveErr := httptest.NewRecorder()
	reqResolveErr := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_id":"99999999","as_of":"2026-01-01","request_id":"req-resolve-err"}`)
	reqResolveErr = reqResolveErr.WithContext(withPrincipal(reqResolveErr.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recResolveErr, reqResolveErr, store)
	if recResolveErr.Code != http.StatusForbidden || !strings.Contains(recResolveErr.Body.String(), capabilityReasonContextMismatch) {
		t.Fatalf("status=%d body=%s", recResolveErr.Code, recResolveErr.Body.String())
	}

	storeEmptySetID := scopeAPIStore{
		resolveSetIDFn: func(context.Context, string, string, string) (string, error) {
			return "   ", nil
		},
	}
	recEmptySetID := httptest.NewRecorder()
	reqEmptySetID := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_id":"10000001","as_of":"2026-01-01","request_id":"req-empty-setid"}`)
	reqEmptySetID = reqEmptySetID.WithContext(withPrincipal(reqEmptySetID.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recEmptySetID, reqEmptySetID, storeEmptySetID)
	if recEmptySetID.Code != http.StatusForbidden || !strings.Contains(recEmptySetID.Body.String(), capabilityReasonContextMismatch) {
		t.Fatalf("status=%d body=%s", recEmptySetID.Code, recEmptySetID.Body.String())
	}

	prevCanView := canViewInternalRulesEvaluate
	canViewInternalRulesEvaluate = func(context.Context) bool { return true }
	t.Cleanup(func() { canViewInternalRulesEvaluate = prevCanView })

	recDynamicMismatch := httptest.NewRecorder()
	reqDynamicMismatch := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_id":"10000001","target_org_unit_id":"10000002","as_of":"2026-01-01","request_id":"req-dyn-mismatch"}`)
	reqDynamicMismatch = reqDynamicMismatch.WithContext(withPrincipal(reqDynamicMismatch.Context(), Principal{RoleSlug: "tenant-viewer"}))
	handleInternalRulesEvaluateAPI(recDynamicMismatch, reqDynamicMismatch, store)
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
	reqListInvalidAsOf := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_id":"10000001","as_of":"2026-01-01","request_id":"req-list-asof"}`)
	reqListInvalidAsOf = reqListInvalidAsOf.WithContext(withPrincipal(reqListInvalidAsOf.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recListInvalidAsOf, reqListInvalidAsOf, store)
	if recListInvalidAsOf.Code != http.StatusBadRequest || !strings.Contains(recListInvalidAsOf.Body.String(), `"code":"invalid_as_of"`) {
		t.Fatalf("status=%d body=%s", recListInvalidAsOf.Code, recListInvalidAsOf.Body.String())
	}

	useSetIDStrategyRegistryStore(setIDStrategyRegistryStoreStub{
		listFn: func(context.Context, string, string, string, string) ([]setIDStrategyRegistryItem, error) {
			return nil, errors.New("boom")
		},
	})
	recListInternalErr := httptest.NewRecorder()
	reqListInternalErr := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_id":"10000001","as_of":"2026-01-01","request_id":"req-list-internal"}`)
	reqListInternalErr = reqListInternalErr.WithContext(withPrincipal(reqListInternalErr.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recListInternalErr, reqListInternalErr, store)
	if recListInternalErr.Code != http.StatusInternalServerError || !strings.Contains(recListInternalErr.Body.String(), `"code":"internal_error"`) {
		t.Fatalf("status=%d body=%s", recListInternalErr.Code, recListInternalErr.Body.String())
	}
	useSetIDStrategyRegistryStore(previousStore)

	internalRuleEligibilityProgramCache = sync.Map{}
	internalRuleDecisionProgramCache = sync.Map{}
	prevEnv := newInternalRulesCELEnv
	newInternalRulesCELEnv = func() (*cel.Env, error) {
		return nil, errors.New("env not available")
	}
	t.Cleanup(func() { newInternalRulesCELEnv = prevEnv })
	recEvalErr := httptest.NewRecorder()
	reqEvalErr := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_id":"10000001","as_of":"2026-01-01","request_id":"req-eval-err"}`)
	reqEvalErr = reqEvalErr.WithContext(withPrincipal(reqEvalErr.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recEvalErr, reqEvalErr, store)
	if recEvalErr.Code != http.StatusInternalServerError || !strings.Contains(recEvalErr.Body.String(), `"code":"internal_error"`) {
		t.Fatalf("status=%d body=%s", recEvalErr.Code, recEvalErr.Body.String())
	}
	newInternalRulesCELEnv = prevEnv

	internalRuleEligibilityProgramCache = sync.Map{}
	internalRuleDecisionProgramCache = sync.Map{}
	recAutoIDs := httptest.NewRecorder()
	reqAutoIDs := makeReq(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_id":"10000001","as_of":"2026-01-01"}`)
	reqAutoIDs = reqAutoIDs.WithContext(withPrincipal(reqAutoIDs.Context(), Principal{RoleSlug: "tenant-admin"}))
	handleInternalRulesEvaluateAPI(recAutoIDs, reqAutoIDs, store)
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

func TestEvaluateInternalRuleCandidates(t *testing.T) {
	internalRuleEligibilityProgramCache = sync.Map{}
	internalRuleDecisionProgramCache = sync.Map{}

	candidates := []internalRuleCandidate{
		{
			RuleID:          "tenant",
			Priority:        100,
			EffectiveDate:   "2026-01-01",
			EligibilityExpr: "true",
			DecisionExpr:    "\"allow\"",
			ReasonCode:      fieldVisibleInContextCode,
		},
		{
			RuleID:          "bu",
			Priority:        200,
			EffectiveDate:   "2026-01-01",
			EligibilityExpr: `ctx["business_unit_id"] == "10000001"`,
			DecisionExpr:    "\"deny\"",
			ReasonCode:      fieldHiddenInContextCode,
		},
	}

	decision, reasonCode, selected, matched, err := evaluateInternalRuleCandidates(map[string]string{"business_unit_id": "10000001"}, candidates)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if decision != internalRuleDecisionDeny || reasonCode != fieldHiddenInContextCode || matched != 2 || selected == nil || selected.RuleID != "bu" {
		t.Fatalf("unexpected result decision=%s reason=%s matched=%d selected=%+v", decision, reasonCode, matched, selected)
	}

	decision, reasonCode, selected, matched, err = evaluateInternalRuleCandidates(map[string]string{"business_unit_id": "10000002"}, candidates)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if decision != internalRuleDecisionAllow || reasonCode != fieldVisibleInContextCode || matched != 1 || selected == nil || selected.RuleID != "tenant" {
		t.Fatalf("unexpected fallback result decision=%s reason=%s matched=%d selected=%+v", decision, reasonCode, matched, selected)
	}

	_, _, _, _, err = evaluateInternalRuleCandidates(map[string]string{"business_unit_id": "10000001"}, []internalRuleCandidate{{
		RuleID:          "bad",
		Priority:        1,
		EffectiveDate:   "2026-01-01",
		EligibilityExpr: "",
		DecisionExpr:    "\"allow\"",
	}})
	if err == nil {
		t.Fatal("expected error")
	}

	decision, reasonCode, selected, matched, err = evaluateInternalRuleCandidates(map[string]string{"business_unit_id": "10000001"}, []internalRuleCandidate{{
		RuleID:          "none",
		Priority:        1,
		EffectiveDate:   "2026-01-01",
		EligibilityExpr: "false",
		DecisionExpr:    "\"allow\"",
	}})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if decision != internalRuleDecisionDeny || reasonCode != fieldPolicyMissingCode || selected != nil || matched != 0 {
		t.Fatalf("unexpected no-match decision=%s reason=%s selected=%+v matched=%d", decision, reasonCode, selected, matched)
	}

	decision, reasonCode, selected, matched, err = evaluateInternalRuleCandidates(map[string]string{"business_unit_id": "10000001"}, []internalRuleCandidate{{
		RuleID:          "unknown-decision",
		Priority:        1,
		EffectiveDate:   "2026-01-01",
		EligibilityExpr: "true",
		DecisionExpr:    "\"MAYBE\"",
		ReasonCode:      "",
	}})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if decision != internalRuleDecisionDeny || reasonCode != fieldPolicyMissingCode || selected == nil || matched != 1 {
		t.Fatalf("unexpected unknown decision result decision=%s reason=%s selected=%+v matched=%d", decision, reasonCode, selected, matched)
	}

	decision, reasonCode, selected, matched, err = evaluateInternalRuleCandidates(map[string]string{"business_unit_id": "10000001"}, []internalRuleCandidate{{
		RuleID:          "allow-default-reason",
		Priority:        1,
		EffectiveDate:   "2026-01-01",
		EligibilityExpr: "true",
		DecisionExpr:    "\"allow\"",
		ReasonCode:      " ",
	}})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if decision != internalRuleDecisionAllow || reasonCode != fieldVisibleInContextCode || selected == nil || matched != 1 {
		t.Fatalf("unexpected allow fallback reason decision=%s reason=%s selected=%+v matched=%d", decision, reasonCode, selected, matched)
	}

	_, _, _, _, err = evaluateInternalRuleCandidates(map[string]string{"business_unit_id": "10000001"}, []internalRuleCandidate{{
		RuleID:          "bad-decision",
		Priority:        1,
		EffectiveDate:   "2026-01-01",
		EligibilityExpr: "true",
		DecisionExpr:    "",
	}})
	if err == nil {
		t.Fatal("expected decision expression error")
	}
}

func TestEvalInternalExpressionsAndProgramCache(t *testing.T) {
	internalRuleEligibilityProgramCache = sync.Map{}
	internalRuleDecisionProgramCache = sync.Map{}

	if _, err := evalInternalEligibilityExpr(`int(ctx["actor_id"]) > 0`, map[string]string{"actor_id": "not-int"}); err == nil {
		t.Fatal("expected eligibility eval runtime error")
	}
	if _, err := evalInternalDecisionExpr("", map[string]string{"trace_id": "01"}); err == nil {
		t.Fatal("expected decision compile error")
	}
	if _, err := evalInternalDecisionExpr(`string(1 / int(ctx["trace_id"]))`, map[string]string{"trace_id": "0"}); err == nil {
		t.Fatal("expected decision eval runtime error")
	}

	cache := &sync.Map{}
	if _, err := loadOrCompileInternalProgram("", cel.BoolType, cache); err == nil {
		t.Fatal("expected empty expression error")
	}
	if _, err := loadOrCompileInternalProgram(`ctx[`, cel.BoolType, cache); err == nil {
		t.Fatal("expected compile error")
	}
	if _, err := loadOrCompileInternalProgram(`"allow"`, cel.BoolType, cache); err == nil {
		t.Fatal("expected output type mismatch")
	}

	prevEnv := newInternalRulesCELEnv
	newInternalRulesCELEnv = func() (*cel.Env, error) {
		return nil, errors.New("env error")
	}
	if _, err := loadOrCompileInternalProgram("true", cel.BoolType, cache); err == nil {
		t.Fatal("expected env error")
	}
	newInternalRulesCELEnv = prevEnv

	prevProgram := newInternalRulesCELProgram
	newInternalRulesCELProgram = func(*cel.Env, *cel.Ast) (cel.Program, error) {
		return nil, errors.New("program error")
	}
	if _, err := loadOrCompileInternalProgram("true", cel.BoolType, cache); err == nil {
		t.Fatal("expected program error")
	}
	newInternalRulesCELProgram = prevProgram

	internalRuleEligibilityProgramCache = sync.Map{}
	envCalls := 0
	newInternalRulesCELEnv = func() (*cel.Env, error) {
		envCalls++
		return cel.NewEnv(cel.Variable("ctx", cel.MapType(cel.StringType, cel.StringType)))
	}
	t.Cleanup(func() { newInternalRulesCELEnv = prevEnv })

	if _, err := loadOrCompileInternalProgram("true", cel.BoolType, &internalRuleEligibilityProgramCache); err != nil {
		t.Fatalf("first compile err=%v", err)
	}
	if _, err := loadOrCompileInternalProgram("true", cel.BoolType, &internalRuleEligibilityProgramCache); err != nil {
		t.Fatalf("cached compile err=%v", err)
	}
	if envCalls != 1 {
		t.Fatalf("expected env calls=1, got %d", envCalls)
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
