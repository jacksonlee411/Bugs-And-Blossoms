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
		getScopeSubscriptionFn: func(context.Context, string, string, string, string) (ScopeSubscription, error) {
			return ScopeSubscription{
				SetID:        "A0001",
				ScopeCode:    "jobcatalog",
				PackageID:    "pkg-1",
				PackageOwner: "tenant",
			}, nil
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
	handleSetIDExplainAPI(recBadAsOf, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_id=10000001&scope_code=jobcatalog&as_of=bad"), store)
	if recBadAsOf.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", recBadAsOf.Code)
	}

	recBadBU := httptest.NewRecorder()
	handleSetIDExplainAPI(recBadBU, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_id=bad&scope_code=jobcatalog&as_of=2026-01-01"), store)
	if recBadBU.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", recBadBU.Code)
	}

	recBadLevel := httptest.NewRecorder()
	handleSetIDExplainAPI(recBadLevel, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_id=10000001&scope_code=jobcatalog&as_of=2026-01-01&level=bad"), store)
	if recBadLevel.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", recBadLevel.Code)
	}

	recFullForbidden := httptest.NewRecorder()
	handleSetIDExplainAPI(recFullForbidden, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_id=10000001&scope_code=jobcatalog&as_of=2026-01-01&level=full"), store)
	if recFullForbidden.Code != http.StatusForbidden {
		t.Fatalf("status=%d", recFullForbidden.Code)
	}

	recBadOrg := httptest.NewRecorder()
	handleSetIDExplainAPI(recBadOrg, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_id=10000001&org_unit_id=bad&scope_code=jobcatalog&as_of=2026-01-01"), store)
	if recBadOrg.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", recBadOrg.Code)
	}

	resolveErrStore := store
	resolveErrStore.resolveSetIDFn = func(context.Context, string, string, string) (string, error) {
		return "", errors.New("SETID_NOT_FOUND")
	}
	recResolveErr := httptest.NewRecorder()
	handleSetIDExplainAPI(recResolveErr, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_id=10000001&scope_code=jobcatalog&as_of=2026-01-01"), resolveErrStore)
	if recResolveErr.Code != http.StatusForbidden {
		t.Fatalf("status=%d", recResolveErr.Code)
	}

	subErrStore := store
	subErrStore.getScopeSubscriptionFn = func(context.Context, string, string, string, string) (ScopeSubscription, error) {
		return ScopeSubscription{}, errors.New("SCOPE_SUBSCRIPTION_MISSING")
	}
	recSubErr := httptest.NewRecorder()
	handleSetIDExplainAPI(recSubErr, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_id=10000001&scope_code=jobcatalog&as_of=2026-01-01"), subErrStore)
	if recSubErr.Code != http.StatusNotFound {
		t.Fatalf("status=%d", recSubErr.Code)
	}

	recMissingPolicy := httptest.NewRecorder()
	handleSetIDExplainAPI(recMissingPolicy, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_id=10000001&scope_code=jobcatalog&as_of=2026-01-01"), store)
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

	briefReq := makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_id=10000001&scope_code=jobcatalog&as_of=2026-01-01&level=brief&request_id=req-1")
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

	fullReq := makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_id=10000001&scope_code=jobcatalog&as_of=2026-01-01&level=full")
	fullReq = fullReq.WithContext(withPrincipal(fullReq.Context(), Principal{RoleSlug: "tenant-admin"}))
	recFull := httptest.NewRecorder()
	handleSetIDExplainAPI(recFull, fullReq, store)
	if recFull.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recFull.Code, recFull.Body.String())
	}
	if !strings.Contains(recFull.Body.String(), `"tenant_id":"t1"`) || !strings.Contains(recFull.Body.String(), `"org_unit_id":"10000001"`) {
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
	handleSetIDExplainAPI(recDeny, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_hidden&business_unit_id=10000001&scope_code=jobcatalog&as_of=2026-01-01"), store)
	if recDeny.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recDeny.Code, recDeny.Body.String())
	}
	if !strings.Contains(recDeny.Body.String(), `"decision":"deny"`) || !strings.Contains(recDeny.Body.String(), fieldHiddenInContextCode) {
		t.Fatalf("unexpected body: %q", recDeny.Body.String())
	}
}
