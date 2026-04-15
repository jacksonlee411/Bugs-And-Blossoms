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

func TestInternalEvaluationContextCELContextMap(t *testing.T) {
	t.Parallel()

	ctx := internalEvaluationContext{
		TenantID:            "tenant_1",
		ActorID:             "actor_1",
		ActorRole:           "tenant-admin",
		CapabilityKey:       "staffing.assignment_create.field_policy",
		FieldKey:            "department_id",
		SetID:               "A0001",
		SetIDSource:         "business_unit",
		BusinessUnitOrgCode: "BU-001",
		OrgCode:             "ORG-001",
		AsOf:                "2026-04-15",
		RequestID:           "req-1",
		TraceID:             "trace-1",
		businessUnitNodeKey: "nodekey_1",
	}

	got := ctx.celContextMap()
	if got["tenant_id"] != "tenant_1" || got["actor_id"] != "actor_1" {
		t.Fatalf("unexpected map=%v", got)
	}
	if got["business_unit_node_key"] != "nodekey_1" || got["setid_source"] != "business_unit" {
		t.Fatalf("unexpected map=%v", got)
	}
	if got["request_id"] != "req-1" || got["trace_id"] != "trace-1" {
		t.Fatalf("unexpected map=%v", got)
	}
}

func TestHandleInternalRulesEvaluateAPI(t *testing.T) {
	previousStore := defaultSetIDStrategyRegistryStore
	t.Cleanup(func() { useSetIDStrategyRegistryStore(previousStore) })
	resetFunctionalAreaSwitchStoreForTest()
	t.Cleanup(resetFunctionalAreaSwitchStoreForTest)
	resetSetIDStrategyRegistryRuntimeForTest()
	t.Cleanup(resetSetIDStrategyRegistryRuntimeForTest)
	resetPolicyActivationRuntimeForTest()

	orgNodeKeyA := mustOrgNodeKeyForTest(t, 10000001)
	orgNodeKeyB := mustOrgNodeKeyForTest(t, 10000002)
	store := scopeAPIStore{
		resolveSetIDFn: func(_ context.Context, _ string, orgNodeKey string, _ string) (string, error) {
			switch orgNodeKey {
			case orgNodeKeyA:
				return "A0001", nil
			case orgNodeKeyB:
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
		},
	}
	newReq := func(method string, body string) *http.Request {
		req := httptest.NewRequest(method, "/internal/rules/evaluate", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "actor-1", RoleSlug: "tenant-admin"}))
		return req
	}
	validBody := `{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_org_code":"BU-001","as_of":"2026-01-01","request_id":"req-1"}`

	t.Run("method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleInternalRulesEvaluateAPI(rec, newReq(http.MethodGet, validBody), store, orgResolver)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/internal/rules/evaluate", bytes.NewBufferString(validBody))
		req = req.WithContext(withPrincipal(req.Context(), Principal{RoleSlug: "tenant-admin"}))
		rec := httptest.NewRecorder()
		handleInternalRulesEvaluateAPI(rec, req, store, orgResolver)
		if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "tenant_missing") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("setid store missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleInternalRulesEvaluateAPI(rec, newReq(http.MethodPost, validBody), nil, orgResolver)
		if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "setid_resolver_missing") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("scope forbidden", func(t *testing.T) {
		req := newReq(http.MethodPost, validBody)
		req = req.WithContext(withPrincipal(req.Context(), Principal{RoleSlug: "tenant-viewer"}))
		rec := httptest.NewRecorder()
		handleInternalRulesEvaluateAPI(rec, req, store, orgResolver)
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), scopeReasonActorScopeForbidden) {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("bad json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleInternalRulesEvaluateAPI(rec, newReq(http.MethodPost, `{`), store, orgResolver)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "bad_json") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid request", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleInternalRulesEvaluateAPI(rec, newReq(http.MethodPost, `{"capability_key":"staffing.assignment_create.field_policy","field_key":"bad field","business_unit_org_code":"BU-001","as_of":"2026-01-01"}`), store, orgResolver)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid field_key") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid as_of", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleInternalRulesEvaluateAPI(rec, newReq(http.MethodPost, `{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_org_code":"BU-001","as_of":"bad"}`), store, orgResolver)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_as_of") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("business unit invalid", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleInternalRulesEvaluateAPI(rec, newReq(http.MethodPost, `{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_org_code":"bad\u007f","as_of":"2026-01-01"}`), store, orgResolver)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "business_unit_org_code_invalid") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("capability context mismatch", func(t *testing.T) {
		req := newReq(http.MethodPost, validBody)
		req.Header.Set("X-Actor-Scope", "saas")
		rec := httptest.NewRecorder()
		handleInternalRulesEvaluateAPI(rec, req, store, orgResolver)
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), capabilityReasonContextMismatch) {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("functional area disabled", func(t *testing.T) {
		defaultFunctionalAreaSwitchStore.setEnabled("t1", "staffing", false)
		t.Cleanup(func() { defaultFunctionalAreaSwitchStore.setEnabled("t1", "staffing", true) })
		rec := httptest.NewRecorder()
		handleInternalRulesEvaluateAPI(rec, newReq(http.MethodPost, validBody), store, orgResolver)
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), functionalAreaDisabledCode) {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("target org invalid", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleInternalRulesEvaluateAPI(rec, newReq(http.MethodPost, `{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_org_code":"BU-001","org_code":"bad\u007f","as_of":"2026-01-01"}`), store, orgResolver)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "org_code_invalid") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("target org relation mismatch", func(t *testing.T) {
		previousCanView := canViewInternalRulesEvaluate
		canViewInternalRulesEvaluate = func(context.Context) bool { return true }
		t.Cleanup(func() { canViewInternalRulesEvaluate = previousCanView })

		req := newReq(http.MethodPost, `{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_org_code":"BU-001","org_code":"BU-002","as_of":"2026-01-01"}`)
		req = req.WithContext(withPrincipal(req.Context(), Principal{RoleSlug: "tenant-viewer"}))
		rec := httptest.NewRecorder()
		handleInternalRulesEvaluateAPI(rec, req, store, orgResolver)
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), capabilityReasonContextMismatch) {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("registry list error", func(t *testing.T) {
		useSetIDStrategyRegistryStore(setIDStrategyRegistryStoreStub{
			listFn: func(context.Context, string, string, string, string) ([]setIDStrategyRegistryItem, error) {
				return nil, errors.New("boom")
			},
		})
		rec := httptest.NewRecorder()
		handleInternalRulesEvaluateAPI(rec, newReq(http.MethodPost, validBody), store, orgResolver)
		if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "internal_error") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("missing policy returns deny response", func(t *testing.T) {
		useSetIDStrategyRegistryStore(setIDStrategyRegistryStoreStub{})
		req := newReq(http.MethodPost, `{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_missing","business_unit_org_code":"BU-001","as_of":"2026-01-01"}`)
		rec := httptest.NewRecorder()
		handleInternalRulesEvaluateAPI(rec, req, store, orgResolver)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var response internalRulesEvaluateResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("json err=%v body=%s", err, rec.Body.String())
		}
		if response.Decision != internalRuleDecisionDeny || response.ReasonCode != fieldPolicyMissingCode {
			t.Fatalf("response=%+v", response)
		}
		if response.SelectedRule != nil || response.BriefExplain != "no eligible rule candidate" {
			t.Fatalf("response=%+v", response)
		}
		if response.Context.ActorID != "actor-1" || response.Context.ActorRole != "tenant-admin" {
			t.Fatalf("context=%+v", response.Context)
		}
	})

	t.Run("success selected rule", func(t *testing.T) {
		useSetIDStrategyRegistryStore(setIDStrategyRegistryStoreStub{
			listFn: func(_ context.Context, tenantID string, capabilityKey string, fieldKey string, asOf string) ([]setIDStrategyRegistryItem, error) {
				if tenantID != "t1" || capabilityKey != "staffing.assignment_create.field_policy" || fieldKey != "field_x" || asOf != "2026-01-01" {
					t.Fatalf("list args tenant=%q capability=%q field=%q asOf=%q", tenantID, capabilityKey, fieldKey, asOf)
				}
				return []setIDStrategyRegistryItem{{
					CapabilityKey:       "staffing.assignment_create.field_policy",
					OwnerModule:         "staffing",
					FieldKey:            "field_x",
					PersonalizationMode: personalizationModeSetID,
					OrgApplicability:    orgApplicabilityBusinessUnit,
					BusinessUnitNodeKey: orgNodeKeyA,
					ResolvedSetID:       "A0001",
					Required:            true,
					Visible:             true,
					Maintainable:        true,
					DefaultRuleRef:      "rule://a1",
					DefaultValue:        "a1",
					Priority:            200,
					ExplainRequired:     true,
					EffectiveDate:       "2026-01-01",
					EndDate:             "2026-12-31",
					UpdatedAt:           "2026-01-01T00:00:00Z",
				}}, nil
			},
		})
		req := newReq(http.MethodPost, validBody)
		req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01")
		rec := httptest.NewRecorder()
		handleInternalRulesEvaluateAPI(rec, req, store, orgResolver)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var response internalRulesEvaluateResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("json err=%v body=%s", err, rec.Body.String())
		}
		if response.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" || response.RequestID != "req-1" {
			t.Fatalf("response=%+v", response)
		}
		if response.Decision != internalRuleDecisionAllow || response.ReasonCode != fieldRequiredInContextCode {
			t.Fatalf("response=%+v", response)
		}
		if response.SelectedRule == nil || response.SelectedRule.Priority != 200 || response.SelectedRule.EffectiveDate != "2026-01-01" || response.SelectedRule.EndDate != "2026-12-31" {
			t.Fatalf("selected_rule=%+v", response.SelectedRule)
		}
		if response.CandidatesEvaluated != 1 || response.EligibilityMatched != 1 {
			t.Fatalf("response=%+v", response)
		}
		if !strings.Contains(response.BriefExplain, "selected ") || !strings.Contains(response.BriefExplain, "matched=1") {
			t.Fatalf("brief_explain=%q", response.BriefExplain)
		}
	})
}

func TestResolveInternalRulesEvaluation(t *testing.T) {
	orgNodeKey := mustOrgNodeKeyForTest(t, 10000001)
	hiddenItem := setIDStrategyRegistryItem{
		CapabilityKey:       "staffing.assignment_create.field_policy",
		OwnerModule:         "staffing",
		FieldKey:            "field_x",
		PersonalizationMode: personalizationModeSetID,
		OrgApplicability:    orgApplicabilityBusinessUnit,
		BusinessUnitNodeKey: orgNodeKey,
		ResolvedSetID:       "A0001",
		Required:            false,
		Visible:             false,
		Maintainable:        false,
		DefaultRuleRef:      "rule://hidden",
		DefaultValue:        "secret",
		Priority:            100,
		ExplainRequired:     true,
		EffectiveDate:       "2026-01-01",
	}

	hidden, err := resolveInternalRulesEvaluation([]setIDStrategyRegistryItem{hiddenItem}, "staffing.assignment_create.field_policy", "field_x", "A0001", orgNodeKey)
	if err != nil {
		t.Fatalf("hidden err=%v", err)
	}
	if hidden.Decision != internalRuleDecisionDeny || hidden.ReasonCode != fieldHiddenInContextCode || hidden.Resolution == nil {
		t.Fatalf("hidden=%+v", hidden)
	}

	visibleItem := hiddenItem
	visibleItem.Visible = true
	visibleItem.Required = true
	visibleItem.DefaultRuleRef = "rule://visible"
	visibleItem.DefaultValue = "value"
	visible, err := resolveInternalRulesEvaluation([]setIDStrategyRegistryItem{visibleItem}, "staffing.assignment_create.field_policy", "field_x", "A0001", orgNodeKey)
	if err != nil {
		t.Fatalf("visible err=%v", err)
	}
	if visible.Decision != internalRuleDecisionAllow || visible.ReasonCode != fieldRequiredInContextCode || visible.Resolution == nil {
		t.Fatalf("visible=%+v", visible)
	}

	missing, err := resolveInternalRulesEvaluation(nil, "staffing.assignment_create.field_policy", "field_x", "A0001", orgNodeKey)
	if err != nil {
		t.Fatalf("missing err=%v", err)
	}
	if missing.Decision != internalRuleDecisionDeny || missing.ReasonCode != fieldPolicyMissingCode || missing.Resolution != nil {
		t.Fatalf("missing=%+v", missing)
	}
}

func TestInternalRuleDecisionAndPresentationHelpers(t *testing.T) {
	for _, code := range []string{
		fieldPolicyMissingCode,
		fieldPolicyConflictCode,
		fieldDefaultRuleMissingCode,
		fieldPolicyPriorityModeCode,
		fieldPolicyModeComboCode,
	} {
		decision, reasonCode, ok := internalRuleDecisionFromError(errors.New(" " + code + " "))
		if !ok || decision != internalRuleDecisionDeny || reasonCode != code {
			t.Fatalf("code=%q decision=%q reason=%q ok=%v", code, decision, reasonCode, ok)
		}
	}
	if decision, reasonCode, ok := internalRuleDecisionFromError(errors.New("boom")); ok || decision != "" || reasonCode != "" {
		t.Fatalf("decision=%q reason=%q ok=%v", decision, reasonCode, ok)
	}

	candidate := internalRuleCandidateFromResolution(nil, internalRuleDecisionAllow, fieldRequiredInContextCode)
	if candidate != nil {
		t.Fatalf("candidate=%+v", candidate)
	}
	if got := internalRuleBriefExplain(nil, 0); got != "no eligible rule candidate" {
		t.Fatalf("brief=%q", got)
	}

	item := setIDStrategyRegistryItem{
		Priority:      300,
		EffectiveDate: "2026-01-01",
		EndDate:       "2026-12-31",
	}
	resolution := setIDFieldDecisionResolution{
		PrimaryPolicyID: "policy-1",
		MatchedBucket:   "intent_setid_exact_business_unit_exact",
		PrimaryItem:     &item,
	}
	candidate = internalRuleCandidateFromResolution(&resolution, " allow ", " reason ")
	if candidate == nil || candidate.RuleID != "policy-1" || candidate.DecisionExpr != "allow" || candidate.ReasonCode != "reason" {
		t.Fatalf("candidate=%+v", candidate)
	}
	if got := internalRuleBriefExplain(candidate, 2); got != "selected policy-1 (priority=300, matched=2)" {
		t.Fatalf("brief=%q", got)
	}
}
