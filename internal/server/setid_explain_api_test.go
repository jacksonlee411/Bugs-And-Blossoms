package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStatusCodeForFieldDecisionError(t *testing.T) {
	cases := []struct {
		err        error
		wantStatus int
		wantCode   string
	}{
		{err: errors.New(fieldPolicyMissingCode), wantStatus: http.StatusUnprocessableEntity, wantCode: fieldPolicyMissingCode},
		{err: errors.New(fieldDefaultRuleMissingCode), wantStatus: http.StatusUnprocessableEntity, wantCode: fieldDefaultRuleMissingCode},
		{err: errors.New(fieldPolicyConflictCode), wantStatus: http.StatusUnprocessableEntity, wantCode: fieldPolicyConflictCode},
		{err: errors.New("boom"), wantStatus: http.StatusInternalServerError, wantCode: "FIELD_EXPLAIN_MISSING"},
	}
	for _, tc := range cases {
		status, code := statusCodeForFieldDecisionError(tc.err)
		if status != tc.wantStatus || code != tc.wantCode {
			t.Fatalf("status=%d code=%q", status, code)
		}
	}
}

func TestCanViewSetIDFullExplain(t *testing.T) {
	if canViewSetIDFullExplain(context.Background()) {
		t.Fatal("expected false")
	}
	if canViewSetIDFullExplain(withPrincipal(context.Background(), Principal{RoleSlug: "tenant-viewer"})) {
		t.Fatal("expected false")
	}
	if !canViewSetIDFullExplain(withPrincipal(context.Background(), Principal{RoleSlug: "tenant-admin"})) {
		t.Fatal("expected true")
	}
	if !canViewSetIDFullExplain(withPrincipal(context.Background(), Principal{RoleSlug: "superadmin"})) {
		t.Fatal("expected true")
	}
}

func TestTraceIDFromRequestHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/setid-explain", nil)
	if got := traceIDFromRequestHeader(req); got != "" {
		t.Fatalf("trace_id=%q", got)
	}
	req.Header.Set("traceparent", "bad")
	if got := traceIDFromRequestHeader(req); got != "" {
		t.Fatalf("trace_id=%q", got)
	}
	req.Header.Set("traceparent", "00-00000000000000000000000000000000-0000000000000000-01")
	if got := traceIDFromRequestHeader(req); got != "" {
		t.Fatalf("trace_id=%q", got)
	}
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e473g-0000000000000000-01")
	if got := traceIDFromRequestHeader(req); got != "" {
		t.Fatalf("trace_id=%q", got)
	}
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01")
	if got := traceIDFromRequestHeader(req); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("trace_id=%q", got)
	}
}

func TestNormalizeSetIDExplainRequestID(t *testing.T) {
	reqQuery := httptest.NewRequest(http.MethodGet, "/org/api/setid-explain?request_id=req-query", nil)
	if got := normalizeSetIDExplainRequestID(reqQuery); got != "req-query" {
		t.Fatalf("request_id=%q", got)
	}

	reqHeader := httptest.NewRequest(http.MethodGet, "/org/api/setid-explain", nil)
	reqHeader.Header.Set("X-Request-Id", "req-header")
	if got := normalizeSetIDExplainRequestID(reqHeader); got != "req-header" {
		t.Fatalf("request_id=%q", got)
	}

	reqTrace := httptest.NewRequest(http.MethodGet, "/org/api/setid-explain", nil)
	reqTrace.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01")
	if got := normalizeSetIDExplainRequestID(reqTrace); got != "trace-4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("request_id=%q", got)
	}

	reqFallback := httptest.NewRequest(http.MethodGet, "/org/api/setid-explain", nil)
	if got := normalizeSetIDExplainRequestID(reqFallback); got != "setid-explain-auto" {
		t.Fatalf("request_id=%q", got)
	}
}

func TestFallbackSetIDExplainTraceID(t *testing.T) {
	got := fallbackSetIDExplainTraceID("req-1", "staffing.assignment_create.field_policy", "10000001", "2026-01-01")
	if len(got) != 32 {
		t.Fatalf("trace_id=%q", got)
	}
	if got != fallbackSetIDExplainTraceID("req-1", "staffing.assignment_create.field_policy", "10000001", "2026-01-01") {
		t.Fatalf("trace_id should be stable: %q", got)
	}
}

func TestResolveFunctionalAreaKey(t *testing.T) {
	if got := resolveFunctionalAreaKey("staffing.assignment_create.field_policy"); got != "staffing" {
		t.Fatalf("functional_area=%q", got)
	}
	if got := resolveFunctionalAreaKey("unknown.key"); got != "org_foundation" {
		t.Fatalf("functional_area=%q", got)
	}
}

func TestHandleSetIDExplainAPI(t *testing.T) {
	resetSetIDStrategyRegistryRuntimeForTest()
	t.Cleanup(resetSetIDStrategyRegistryRuntimeForTest)

	makeReq := func(path string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		return req
	}
	store := scopeAPIStore{
		resolveSetIDFn: func(context.Context, string, string, string) (string, error) {
			return "A0001", nil
		},
	}

	reqNoTenant := httptest.NewRequest(http.MethodGet, "/org/api/setid-explain", nil)
	recNoTenant := httptest.NewRecorder()
	handleSetIDExplainAPI(recNoTenant, reqNoTenant, store)
	if recNoTenant.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", recNoTenant.Code)
	}

	reqMethod := httptest.NewRequest(http.MethodPost, "/org/api/setid-explain", nil)
	reqMethod = reqMethod.WithContext(withTenant(reqMethod.Context(), Tenant{ID: "t1"}))
	recMethod := httptest.NewRecorder()
	handleSetIDExplainAPI(recMethod, reqMethod, store)
	if recMethod.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", recMethod.Code)
	}

	recMissing := httptest.NewRecorder()
	handleSetIDExplainAPI(recMissing, makeReq("/org/api/setid-explain"), store)
	if recMissing.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", recMissing.Code)
	}

	recBadAsOf := httptest.NewRecorder()
	handleSetIDExplainAPI(recBadAsOf, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_id=10000001&as_of=bad"), store)
	if recBadAsOf.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", recBadAsOf.Code)
	}

	recBadBU := httptest.NewRecorder()
	handleSetIDExplainAPI(recBadBU, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_id=bad&as_of=2026-01-01"), store)
	if recBadBU.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", recBadBU.Code)
	}

	recBadLevel := httptest.NewRecorder()
	handleSetIDExplainAPI(recBadLevel, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_id=10000001&as_of=2026-01-01&level=bad"), store)
	if recBadLevel.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", recBadLevel.Code)
	}

	recFullForbidden := httptest.NewRecorder()
	handleSetIDExplainAPI(recFullForbidden, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_id=10000001&as_of=2026-01-01&level=full"), store)
	if recFullForbidden.Code != http.StatusForbidden {
		t.Fatalf("status=%d", recFullForbidden.Code)
	}

	recBadOrg := httptest.NewRecorder()
	handleSetIDExplainAPI(recBadOrg, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_id=10000001&org_unit_id=bad&as_of=2026-01-01"), store)
	if recBadOrg.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", recBadOrg.Code)
	}

	recRelationMismatchReq := makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_id=10000001&org_unit_id=10000002&as_of=2026-01-01")
	recRelationMismatchReq = recRelationMismatchReq.WithContext(withPrincipal(recRelationMismatchReq.Context(), Principal{RoleSlug: "tenant-viewer"}))
	recRelationMismatch := httptest.NewRecorder()
	handleSetIDExplainAPI(recRelationMismatch, recRelationMismatchReq, store)
	if recRelationMismatch.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", recRelationMismatch.Code, recRelationMismatch.Body.String())
	}
	if !strings.Contains(recRelationMismatch.Body.String(), capabilityReasonContextMismatch) {
		t.Fatalf("unexpected body: %q", recRelationMismatch.Body.String())
	}

	resolveErrStore := store
	resolveErrStore.resolveSetIDFn = func(context.Context, string, string, string) (string, error) {
		return "", errors.New("SETID_NOT_FOUND")
	}
	recResolveErr := httptest.NewRecorder()
	handleSetIDExplainAPI(recResolveErr, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_id=10000001&as_of=2026-01-01"), resolveErrStore)
	if recResolveErr.Code != http.StatusForbidden {
		t.Fatalf("status=%d", recResolveErr.Code)
	}
	if !strings.Contains(recResolveErr.Body.String(), capabilityReasonContextMismatch) {
		t.Fatalf("unexpected body: %q", recResolveErr.Body.String())
	}

	recSetIDMismatch := httptest.NewRecorder()
	handleSetIDExplainAPI(recSetIDMismatch, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_id=10000001&as_of=2026-01-01&setid=B0001"), store)
	if recSetIDMismatch.Code != http.StatusForbidden {
		t.Fatalf("status=%d", recSetIDMismatch.Code)
	}
	if !strings.Contains(recSetIDMismatch.Body.String(), capabilityReasonContextMismatch) {
		t.Fatalf("unexpected body: %q", recSetIDMismatch.Body.String())
	}

	recMissingPolicy := httptest.NewRecorder()
	handleSetIDExplainAPI(recMissingPolicy, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_id=10000001&as_of=2026-01-01"), store)
	if recMissingPolicy.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d", recMissingPolicy.Code)
	}
	if !strings.Contains(recMissingPolicy.Body.String(), fieldPolicyMissingCode) {
		t.Fatalf("unexpected body: %q", recMissingPolicy.Body.String())
	}

	_, _ = defaultSetIDStrategyRegistryRuntime.upsert("t1", setIDStrategyRegistryItem{
		CapabilityKey:       "staffing.assignment_create.field_policy",
		OwnerModule:         "staffing",
		FieldKey:            "field_x",
		PersonalizationMode: personalizationModeSetID,
		OrgLevel:            orgLevelBusinessUnit,
		BusinessUnitID:      "10000001",
		Required:            true,
		Visible:             true,
		DefaultRuleRef:      "rule://a1",
		DefaultValue:        "a1",
		ExplainRequired:     true,
		EffectiveDate:       "2026-01-01",
		Priority:            200,
	})

	briefReq := makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_id=10000001&as_of=2026-01-01&level=brief&request_id=req-1")
	briefReq.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01")
	recBrief := httptest.NewRecorder()
	handleSetIDExplainAPI(recBrief, briefReq, store)
	if recBrief.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recBrief.Code, recBrief.Body.String())
	}
	if !strings.Contains(recBrief.Body.String(), `"decision":"allow"`) || !strings.Contains(recBrief.Body.String(), fieldRequiredInContextCode) {
		t.Fatalf("unexpected body: %q", recBrief.Body.String())
	}
	if !strings.Contains(recBrief.Body.String(), `"trace_id":"4bf92f3577b34da6a3ce929d0e0e4736"`) {
		t.Fatalf("unexpected body: %q", recBrief.Body.String())
	}
	if !strings.Contains(recBrief.Body.String(), `"setid":"A0001"`) ||
		!strings.Contains(recBrief.Body.String(), `"functional_area_key":"staffing"`) ||
		!strings.Contains(recBrief.Body.String(), `"policy_version":"2026-02-23"`) ||
		!strings.Contains(recBrief.Body.String(), `"reason_code":"`+fieldRequiredInContextCode+`"`) {
		t.Fatalf("unexpected body: %q", recBrief.Body.String())
	}
	if strings.Contains(recBrief.Body.String(), `"tenant_id":"t1"`) || strings.Contains(recBrief.Body.String(), `"org_unit_id":"10000001"`) {
		t.Fatalf("unexpected brief fields: %q", recBrief.Body.String())
	}

	fullReq := makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_id=10000001&as_of=2026-01-01&level=full")
	fullReq = fullReq.WithContext(withPrincipal(fullReq.Context(), Principal{RoleSlug: "tenant-admin"}))
	recFull := httptest.NewRecorder()
	handleSetIDExplainAPI(recFull, fullReq, store)
	if recFull.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recFull.Code, recFull.Body.String())
	}
	if !strings.Contains(recFull.Body.String(), `"tenant_id":"t1"`) || !strings.Contains(recFull.Body.String(), `"org_unit_id":"10000001"`) {
		t.Fatalf("unexpected body: %q", recFull.Body.String())
	}
	if !strings.Contains(recFull.Body.String(), `"resolved_config_version":"2026-02-23"`) {
		t.Fatalf("unexpected body: %q", recFull.Body.String())
	}

	_, _ = defaultSetIDStrategyRegistryRuntime.upsert("t1", setIDStrategyRegistryItem{
		CapabilityKey:       "staffing.assignment_create.field_policy",
		OwnerModule:         "staffing",
		FieldKey:            "field_hidden",
		PersonalizationMode: personalizationModeSetID,
		OrgLevel:            orgLevelBusinessUnit,
		BusinessUnitID:      "10000001",
		Required:            false,
		Visible:             false,
		DefaultRuleRef:      "rule://b2",
		DefaultValue:        "b2",
		ExplainRequired:     true,
		EffectiveDate:       "2026-01-01",
		Priority:            200,
	})
	recDeny := httptest.NewRecorder()
	handleSetIDExplainAPI(recDeny, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_hidden&business_unit_id=10000001&as_of=2026-01-01"), store)
	if recDeny.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recDeny.Code, recDeny.Body.String())
	}
	if !strings.Contains(recDeny.Body.String(), `"decision":"deny"`) || !strings.Contains(recDeny.Body.String(), fieldHiddenInContextCode) {
		t.Fatalf("unexpected body: %q", recDeny.Body.String())
	}
	if !strings.Contains(recDeny.Body.String(), `"reason_code":"`+fieldHiddenInContextCode+`"`) {
		t.Fatalf("unexpected body: %q", recDeny.Body.String())
	}

	recActorScopeMismatchReq := makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_id=10000001&as_of=2026-01-01")
	recActorScopeMismatchReq.Header.Set("X-Actor-Scope", "saas")
	recActorScopeMismatch := httptest.NewRecorder()
	handleSetIDExplainAPI(recActorScopeMismatch, recActorScopeMismatchReq, store)
	if recActorScopeMismatch.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", recActorScopeMismatch.Code, recActorScopeMismatch.Body.String())
	}
	if !strings.Contains(recActorScopeMismatch.Body.String(), capabilityReasonContextMismatch) {
		t.Fatalf("unexpected body: %q", recActorScopeMismatch.Body.String())
	}
}

func TestHandleSetIDExplainAPI_BUVarianceAcceptance(t *testing.T) {
	resetSetIDStrategyRegistryRuntimeForTest()
	t.Cleanup(resetSetIDStrategyRegistryRuntimeForTest)

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
		PersonalizationMode: personalizationModeSetID,
		OrgLevel:            orgLevelBusinessUnit,
		BusinessUnitID:      "10000001",
		Required:            true,
		Visible:             true,
		DefaultRuleRef:      "rule://a1",
		DefaultValue:        "a1",
		ExplainRequired:     true,
		EffectiveDate:       "2026-01-01",
		Priority:            200,
	})
	_, _ = defaultSetIDStrategyRegistryRuntime.upsert("t1", setIDStrategyRegistryItem{
		CapabilityKey:       "staffing.assignment_create.field_policy",
		OwnerModule:         "staffing",
		FieldKey:            "field_x",
		PersonalizationMode: personalizationModeSetID,
		OrgLevel:            orgLevelBusinessUnit,
		BusinessUnitID:      "10000002",
		Required:            false,
		Visible:             false,
		DefaultRuleRef:      "rule://b2",
		DefaultValue:        "b2",
		ExplainRequired:     true,
		EffectiveDate:       "2026-01-01",
		Priority:            200,
	})

	makeReq := func(businessUnitID string) *http.Request {
		req := httptest.NewRequest(
			http.MethodGet,
			"/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&as_of=2026-01-01&business_unit_id="+businessUnitID,
			nil,
		)
		return req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	}

	recBUA := httptest.NewRecorder()
	handleSetIDExplainAPI(recBUA, makeReq("10000001"), store)
	if recBUA.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recBUA.Code, recBUA.Body.String())
	}
	if !strings.Contains(recBUA.Body.String(), `"required":true`) ||
		!strings.Contains(recBUA.Body.String(), fieldRequiredInContextCode) ||
		!strings.Contains(recBUA.Body.String(), `"resolved_default_value":"a1"`) {
		t.Fatalf("unexpected body: %q", recBUA.Body.String())
	}

	recBUB := httptest.NewRecorder()
	handleSetIDExplainAPI(recBUB, makeReq("10000002"), store)
	if recBUB.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recBUB.Code, recBUB.Body.String())
	}
	if !strings.Contains(recBUB.Body.String(), `"decision":"deny"`) ||
		!strings.Contains(recBUB.Body.String(), fieldHiddenInContextCode) ||
		!strings.Contains(recBUB.Body.String(), `"resolved_default_value":"b2"`) {
		t.Fatalf("unexpected body: %q", recBUB.Body.String())
	}
}
