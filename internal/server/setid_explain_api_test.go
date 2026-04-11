package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
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
	got := fallbackSetIDExplainTraceID("req-1", "staffing.assignment_create.field_policy", "BU-001", "2026-01-01")
	if len(got) != 32 {
		t.Fatalf("trace_id=%q", got)
	}
	if got != fallbackSetIDExplainTraceID("req-1", "staffing.assignment_create.field_policy", "BU-001", "2026-01-01") {
		t.Fatalf("trace_id should be stable: %q", got)
	}
}

func TestResolveFunctionalAreaKey(t *testing.T) {
	if got := resolveFunctionalAreaKey("staffing.assignment_create.field_policy"); got != "staffing" {
		t.Fatalf("functional_area=%q", got)
	}
	if got := resolveFunctionalAreaKey("unknown.key"); got != "" {
		t.Fatalf("functional_area=%q", got)
	}
}

func mustOrgNodeKeyForTest(t *testing.T, orgID int) string {
	t.Helper()
	orgNodeKey, err := encodeOrgNodeKeyFromID(orgID)
	if err != nil {
		t.Fatalf("encode org_node_key: %v", err)
	}
	return orgNodeKey
}

type setIDExplainOrgResolverStub struct {
	byCode map[string]string
}

func (s setIDExplainOrgResolverStub) ResolveOrgNodeKeyByCode(_ context.Context, _ string, orgCode string) (string, error) {
	normalized, err := orgunitpkg.NormalizeOrgCode(orgCode)
	if err != nil {
		return "", err
	}
	orgNodeKey, ok := s.byCode[normalized]
	if !ok {
		return "", orgunitpkg.ErrOrgCodeNotFound
	}
	return orgNodeKey, nil
}

func (s setIDExplainOrgResolverStub) ResolveOrgCodeByNodeKey(_ context.Context, _ string, orgNodeKey string) (string, error) {
	normalized, err := normalizeOrgNodeKeyInput(orgNodeKey)
	if err != nil {
		return "", err
	}
	for orgCode, candidate := range s.byCode {
		candidateKey, candidateErr := normalizeOrgNodeKeyInput(candidate)
		if candidateErr != nil {
			return "", candidateErr
		}
		if candidateKey == normalized {
			return orgCode, nil
		}
	}
	return "", orgunitpkg.ErrOrgNodeKeyNotFound
}

func (s setIDExplainOrgResolverStub) ResolveOrgCodesByNodeKeys(ctx context.Context, tenantID string, orgNodeKeys []string) (map[string]string, error) {
	out := make(map[string]string, len(orgNodeKeys))
	for _, orgNodeKey := range orgNodeKeys {
		orgCode, err := s.ResolveOrgCodeByNodeKey(ctx, tenantID, orgNodeKey)
		if err != nil {
			return nil, err
		}
		out[orgNodeKey] = orgCode
	}
	return out, nil
}

func TestHandleSetIDExplainAPI(t *testing.T) {
	resetSetIDStrategyRegistryRuntimeForTest()
	t.Cleanup(resetSetIDStrategyRegistryRuntimeForTest)
	resetFunctionalAreaSwitchStoreForTest()
	t.Cleanup(resetFunctionalAreaSwitchStoreForTest)

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
	orgNodeKeyA := mustOrgNodeKeyForTest(t, 10000001)
	orgNodeKeyB := mustOrgNodeKeyForTest(t, 10000002)
	orgResolver := setIDExplainOrgResolverStub{
		byCode: map[string]string{
			"BU-001": orgNodeKeyA,
			"BU-002": orgNodeKeyB,
		},
	}

	reqNoTenant := httptest.NewRequest(http.MethodGet, "/org/api/setid-explain", nil)
	recNoTenant := httptest.NewRecorder()
	handleSetIDExplainAPI(recNoTenant, reqNoTenant, store, orgResolver)
	if recNoTenant.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", recNoTenant.Code)
	}

	reqMethod := httptest.NewRequest(http.MethodPost, "/org/api/setid-explain", nil)
	reqMethod = reqMethod.WithContext(withTenant(reqMethod.Context(), Tenant{ID: "t1"}))
	recMethod := httptest.NewRecorder()
	handleSetIDExplainAPI(recMethod, reqMethod, store, orgResolver)
	if recMethod.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", recMethod.Code)
	}

	recMissing := httptest.NewRecorder()
	handleSetIDExplainAPI(recMissing, makeReq("/org/api/setid-explain"), store, orgResolver)
	if recMissing.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", recMissing.Code)
	}

	recBadAsOf := httptest.NewRecorder()
	handleSetIDExplainAPI(recBadAsOf, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_org_code=BU-001&as_of=bad"), store, orgResolver)
	if recBadAsOf.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", recBadAsOf.Code)
	}

	recBadBU := httptest.NewRecorder()
	handleSetIDExplainAPI(recBadBU, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_org_code=bad%7F&as_of=2026-01-01"), store, orgResolver)
	if recBadBU.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", recBadBU.Code)
	}

	recAreaMissing := httptest.NewRecorder()
	handleSetIDExplainAPI(recAreaMissing, makeReq("/org/api/setid-explain?capability_key=unknown.key&field_key=field_x&business_unit_org_code=BU-001&as_of=2026-01-01"), store, orgResolver)
	if recAreaMissing.Code != http.StatusForbidden || !strings.Contains(recAreaMissing.Body.String(), functionalAreaMissingCode) {
		t.Fatalf("status=%d body=%s", recAreaMissing.Code, recAreaMissing.Body.String())
	}

	defaultFunctionalAreaSwitchStore.setEnabled("t1", "staffing", false)
	recAreaDisabled := httptest.NewRecorder()
	handleSetIDExplainAPI(recAreaDisabled, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_org_code=BU-001&as_of=2026-01-01"), store, orgResolver)
	if recAreaDisabled.Code != http.StatusForbidden || !strings.Contains(recAreaDisabled.Body.String(), functionalAreaDisabledCode) {
		t.Fatalf("status=%d body=%s", recAreaDisabled.Code, recAreaDisabled.Body.String())
	}
	defaultFunctionalAreaSwitchStore.setEnabled("t1", "staffing", true)

	recBadLevel := httptest.NewRecorder()
	handleSetIDExplainAPI(recBadLevel, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_org_code=BU-001&as_of=2026-01-01&level=bad"), store, orgResolver)
	if recBadLevel.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", recBadLevel.Code)
	}

	recFullForbidden := httptest.NewRecorder()
	handleSetIDExplainAPI(recFullForbidden, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_org_code=BU-001&as_of=2026-01-01&level=full"), store, orgResolver)
	if recFullForbidden.Code != http.StatusForbidden {
		t.Fatalf("status=%d", recFullForbidden.Code)
	}

	recBadOrg := httptest.NewRecorder()
	handleSetIDExplainAPI(recBadOrg, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_org_code=BU-001&org_code=bad%7F&as_of=2026-01-01"), store, orgResolver)
	if recBadOrg.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", recBadOrg.Code)
	}

	recRelationMismatchReq := makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_org_code=BU-001&org_code=BU-002&as_of=2026-01-01")
	recRelationMismatchReq = recRelationMismatchReq.WithContext(withPrincipal(recRelationMismatchReq.Context(), Principal{RoleSlug: "tenant-viewer"}))
	recRelationMismatch := httptest.NewRecorder()
	handleSetIDExplainAPI(recRelationMismatch, recRelationMismatchReq, store, orgResolver)
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
	handleSetIDExplainAPI(recResolveErr, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_org_code=BU-001&as_of=2026-01-01"), resolveErrStore, orgResolver)
	if recResolveErr.Code != http.StatusForbidden {
		t.Fatalf("status=%d", recResolveErr.Code)
	}
	if !strings.Contains(recResolveErr.Body.String(), capabilityReasonContextMismatch) {
		t.Fatalf("unexpected body: %q", recResolveErr.Body.String())
	}

	recSetIDMismatch := httptest.NewRecorder()
	handleSetIDExplainAPI(recSetIDMismatch, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_org_code=BU-001&as_of=2026-01-01&setid=B0001"), store, orgResolver)
	if recSetIDMismatch.Code != http.StatusForbidden {
		t.Fatalf("status=%d", recSetIDMismatch.Code)
	}
	if !strings.Contains(recSetIDMismatch.Body.String(), capabilityReasonContextMismatch) {
		t.Fatalf("unexpected body: %q", recSetIDMismatch.Body.String())
	}

	recMissingPolicy := httptest.NewRecorder()
	handleSetIDExplainAPI(recMissingPolicy, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_org_code=BU-001&as_of=2026-01-01"), store, orgResolver)
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
		OrgApplicability:    orgApplicabilityBusinessUnit,
		BusinessUnitNodeKey: orgNodeKeyA,
		Required:            true,
		Visible:             true,
		DefaultRuleRef:      "rule://a1",
		DefaultValue:        "a1",
		ExplainRequired:     true,
		EffectiveDate:       "2026-01-01",
		Priority:            200,
	})

	briefReq := makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_org_code=BU-001&as_of=2026-01-01&level=brief&request_id=req-1")
	briefReq.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01")
	recBrief := httptest.NewRecorder()
	handleSetIDExplainAPI(recBrief, briefReq, store, orgResolver)
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
		!strings.Contains(recBrief.Body.String(), `"business_unit_org_code":"BU-001"`) ||
		!strings.Contains(recBrief.Body.String(), `"reason_code":"`+fieldRequiredInContextCode+`"`) {
		t.Fatalf("unexpected body: %q", recBrief.Body.String())
	}
	if strings.Contains(recBrief.Body.String(), `"tenant_id":"t1"`) ||
		strings.Contains(recBrief.Body.String(), `"business_unit_id":"10000001"`) ||
		strings.Contains(recBrief.Body.String(), `"org_code":"BU-001"`) {
		t.Fatalf("unexpected brief fields: %q", recBrief.Body.String())
	}

	fullReq := makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_org_code=BU-001&as_of=2026-01-01&level=full")
	fullReq = fullReq.WithContext(withPrincipal(fullReq.Context(), Principal{RoleSlug: "tenant-admin"}))
	recFull := httptest.NewRecorder()
	handleSetIDExplainAPI(recFull, fullReq, store, orgResolver)
	if recFull.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recFull.Code, recFull.Body.String())
	}
	if !strings.Contains(recFull.Body.String(), `"tenant_id":"t1"`) ||
		!strings.Contains(recFull.Body.String(), `"business_unit_org_code":"BU-001"`) ||
		strings.Contains(recFull.Body.String(), `"business_unit_id":"10000001"`) ||
		strings.Contains(recFull.Body.String(), `"org_code":"BU-001"`) {
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
		OrgApplicability:    orgApplicabilityBusinessUnit,
		BusinessUnitNodeKey: orgNodeKeyA,
		Required:            false,
		Visible:             false,
		DefaultRuleRef:      "rule://b2",
		DefaultValue:        "b2",
		ExplainRequired:     true,
		EffectiveDate:       "2026-01-01",
		Priority:            200,
	})
	recDeny := httptest.NewRecorder()
	handleSetIDExplainAPI(recDeny, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_hidden&business_unit_org_code=BU-001&as_of=2026-01-01"), store, orgResolver)
	if recDeny.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recDeny.Code, recDeny.Body.String())
	}
	if !strings.Contains(recDeny.Body.String(), `"decision":"deny"`) || !strings.Contains(recDeny.Body.String(), fieldHiddenInContextCode) {
		t.Fatalf("unexpected body: %q", recDeny.Body.String())
	}
	if !strings.Contains(recDeny.Body.String(), `"reason_code":"`+fieldHiddenInContextCode+`"`) {
		t.Fatalf("unexpected body: %q", recDeny.Body.String())
	}
	if strings.Contains(recDeny.Body.String(), `"resolved_default_value":"b2"`) ||
		!strings.Contains(recDeny.Body.String(), `"visibility":"hidden"`) ||
		!strings.Contains(recDeny.Body.String(), `"masked_default_value":"***"`) {
		t.Fatalf("unexpected body: %q", recDeny.Body.String())
	}

	_, _ = defaultSetIDStrategyRegistryRuntime.upsert("t1", setIDStrategyRegistryItem{
		CapabilityKey:       "staffing.assignment_create.field_policy",
		OwnerModule:         "staffing",
		FieldKey:            "field_masked",
		PersonalizationMode: personalizationModeSetID,
		OrgApplicability:    orgApplicabilityBusinessUnit,
		BusinessUnitNodeKey: orgNodeKeyA,
		Required:            false,
		Visible:             true,
		DefaultRuleRef:      "mask://redact",
		DefaultValue:        "secret-v1",
		ExplainRequired:     true,
		EffectiveDate:       "2026-01-01",
		Priority:            200,
	})
	recMasked := httptest.NewRecorder()
	handleSetIDExplainAPI(recMasked, makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_masked&business_unit_org_code=BU-001&as_of=2026-01-01"), store, orgResolver)
	if recMasked.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recMasked.Code, recMasked.Body.String())
	}
	if !strings.Contains(recMasked.Body.String(), `"reason_code":"`+fieldMaskedInContextCode+`"`) ||
		!strings.Contains(recMasked.Body.String(), `"visibility":"masked"`) ||
		!strings.Contains(recMasked.Body.String(), `"mask_strategy":"redact"`) ||
		!strings.Contains(recMasked.Body.String(), `"masked_default_value":"***"`) ||
		strings.Contains(recMasked.Body.String(), `"resolved_default_value":"secret-v1"`) {
		t.Fatalf("unexpected body: %q", recMasked.Body.String())
	}

	recActorScopeMismatchReq := makeReq("/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_org_code=BU-001&as_of=2026-01-01")
	recActorScopeMismatchReq.Header.Set("X-Actor-Scope", "saas")
	recActorScopeMismatch := httptest.NewRecorder()
	handleSetIDExplainAPI(recActorScopeMismatch, recActorScopeMismatchReq, store, orgResolver)
	if recActorScopeMismatch.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", recActorScopeMismatch.Code, recActorScopeMismatch.Body.String())
	}
	if !strings.Contains(recActorScopeMismatch.Body.String(), capabilityReasonContextMismatch) {
		t.Fatalf("unexpected body: %q", recActorScopeMismatch.Body.String())
	}
}

func TestHandleSetIDExplainAPI_UsesBusinessUnitOrgNodeKey(t *testing.T) {
	previousStore := defaultSetIDStrategyRegistryStore
	t.Cleanup(func() { useSetIDStrategyRegistryStore(previousStore) })

	businessUnitNodeKey := mustOrgNodeKeyForTest(t, 10000001)
	useSetIDStrategyRegistryStore(setIDStrategyRegistryStoreStub{
		resolveFieldDecisionFn: func(_ context.Context, _ string, capabilityKey string, fieldKey string, businessUnitNodeKeyArg string, asOf string) (setIDFieldDecision, error) {
			if capabilityKey != "staffing.assignment_create.field_policy" {
				t.Fatalf("capability_key=%q", capabilityKey)
			}
			if fieldKey != "field_x" {
				t.Fatalf("field_key=%q", fieldKey)
			}
			if businessUnitNodeKeyArg != businessUnitNodeKey {
				t.Fatalf("business_unit_node_key=%q want=%q", businessUnitNodeKeyArg, businessUnitNodeKey)
			}
			if asOf != "2026-01-01" {
				t.Fatalf("as_of=%q", asOf)
			}
			return setIDFieldDecision{
				CapabilityKey: capabilityKey,
				FieldKey:      fieldKey,
				Visible:       true,
				Maintainable:  true,
			}, nil
		},
	})

	store := scopeAPIStore{
		resolveSetIDFn: func(_ context.Context, _ string, orgUnitID string, _ string) (string, error) {
			if orgUnitID != businessUnitNodeKey {
				t.Fatalf("resolve setid orgUnitID=%q want=%q", orgUnitID, businessUnitNodeKey)
			}
			return "A0001", nil
		},
	}
	orgResolver := setIDExplainOrgResolverStub{
		byCode: map[string]string{
			"BU-001": businessUnitNodeKey,
		},
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_org_code=BU-001&as_of=2026-01-01",
		nil,
	)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))

	rec := httptest.NewRecorder()
	handleSetIDExplainAPI(rec, req, store, orgResolver)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleSetIDExplainAPI_BUVarianceAcceptance(t *testing.T) {
	resetSetIDStrategyRegistryRuntimeForTest()
	t.Cleanup(resetSetIDStrategyRegistryRuntimeForTest)

	orgNodeKeyA := mustOrgNodeKeyForTest(t, 10000001)
	orgNodeKeyB := mustOrgNodeKeyForTest(t, 10000002)
	store := scopeAPIStore{
		resolveSetIDFn: func(_ context.Context, _ string, orgUnitID string, _ string) (string, error) {
			switch orgUnitID {
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

	_, _ = defaultSetIDStrategyRegistryRuntime.upsert("t1", setIDStrategyRegistryItem{
		CapabilityKey:       "staffing.assignment_create.field_policy",
		OwnerModule:         "staffing",
		FieldKey:            "field_x",
		PersonalizationMode: personalizationModeSetID,
		OrgApplicability:    orgApplicabilityBusinessUnit,
		BusinessUnitNodeKey: orgNodeKeyA,
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
		OrgApplicability:    orgApplicabilityBusinessUnit,
		BusinessUnitNodeKey: orgNodeKeyB,
		Required:            false,
		Visible:             false,
		DefaultRuleRef:      "rule://b2",
		DefaultValue:        "b2",
		ExplainRequired:     true,
		EffectiveDate:       "2026-01-01",
		Priority:            200,
	})

	makeReq := func(businessUnitOrgCode string) *http.Request {
		req := httptest.NewRequest(
			http.MethodGet,
			"/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&as_of=2026-01-01&business_unit_org_code="+businessUnitOrgCode,
			nil,
		)
		return req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	}

	recBUA := httptest.NewRecorder()
	handleSetIDExplainAPI(recBUA, makeReq("BU-001"), store, orgResolver)
	if recBUA.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recBUA.Code, recBUA.Body.String())
	}
	if !strings.Contains(recBUA.Body.String(), `"required":true`) ||
		!strings.Contains(recBUA.Body.String(), fieldRequiredInContextCode) ||
		!strings.Contains(recBUA.Body.String(), `"resolved_default_value":"a1"`) {
		t.Fatalf("unexpected body: %q", recBUA.Body.String())
	}

	recBUB := httptest.NewRecorder()
	handleSetIDExplainAPI(recBUB, makeReq("BU-002"), store, orgResolver)
	if recBUB.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recBUB.Code, recBUB.Body.String())
	}
	if !strings.Contains(recBUB.Body.String(), `"decision":"deny"`) ||
		!strings.Contains(recBUB.Body.String(), fieldHiddenInContextCode) ||
		!strings.Contains(recBUB.Body.String(), `"masked_default_value":"***"`) ||
		!strings.Contains(recBUB.Body.String(), `"visibility":"hidden"`) ||
		strings.Contains(recBUB.Body.String(), `"resolved_default_value":"b2"`) {
		t.Fatalf("unexpected body: %q", recBUB.Body.String())
	}
}

func TestApplySetIDFieldVisibility(t *testing.T) {
	visibleDecision, visibleResult, visibleReason := applySetIDFieldVisibility(setIDFieldDecision{
		FieldKey:           "field_visible",
		Visible:            true,
		Required:           true,
		ResolvedDefaultVal: "v1",
	})
	if visibleResult != internalRuleDecisionAllow || visibleReason != fieldRequiredInContextCode || visibleDecision.Visibility != fieldVisibilityVisible {
		t.Fatalf("visible decision=%+v result=%q reason=%q", visibleDecision, visibleResult, visibleReason)
	}
	if visibleDecision.ResolvedDefaultVal != "v1" {
		t.Fatalf("visible resolved_default_value=%q", visibleDecision.ResolvedDefaultVal)
	}

	hiddenDecision, hiddenResult, hiddenReason := applySetIDFieldVisibility(setIDFieldDecision{
		FieldKey:           "field_hidden",
		Visible:            false,
		ResolvedDefaultVal: "secret",
	})
	if hiddenResult != internalRuleDecisionDeny || hiddenReason != fieldHiddenInContextCode || hiddenDecision.Visibility != fieldVisibilityHidden {
		t.Fatalf("hidden decision=%+v result=%q reason=%q", hiddenDecision, hiddenResult, hiddenReason)
	}
	if hiddenDecision.ResolvedDefaultVal != "" || hiddenDecision.MaskedDefaultVal != "***" {
		t.Fatalf("hidden decision=%+v", hiddenDecision)
	}

	maskedDecision, maskedResult, maskedReason := applySetIDFieldVisibility(setIDFieldDecision{
		FieldKey:           "field_masked",
		Visible:            true,
		DefaultRuleRef:     "mask://redact",
		ResolvedDefaultVal: "secret",
	})
	if maskedResult != internalRuleDecisionAllow || maskedReason != fieldMaskedInContextCode || maskedDecision.Visibility != fieldVisibilityMasked {
		t.Fatalf("masked decision=%+v result=%q reason=%q", maskedDecision, maskedResult, maskedReason)
	}
	if maskedDecision.ResolvedDefaultVal != "" || maskedDecision.MaskedDefaultVal != "***" || maskedDecision.MaskStrategy != "redact" {
		t.Fatalf("masked decision=%+v", maskedDecision)
	}

	if strategy, ok := setIDMaskStrategyForDecision(setIDFieldDecision{Visible: true, DefaultRuleRef: "rule://normal"}); ok || strategy != "" {
		t.Fatalf("unexpected strategy=%q ok=%v", strategy, ok)
	}
	if strategy, ok := setIDMaskStrategyForDecision(setIDFieldDecision{Visible: true, DefaultRuleRef: "mask://"}); !ok || strategy != fieldMaskStrategyRedact {
		t.Fatalf("unexpected strategy=%q ok=%v", strategy, ok)
	}
}

func TestBriefSetIDFieldDecisions(t *testing.T) {
	if got := briefSetIDFieldDecisions(nil); got != "-" {
		t.Fatalf("brief=%q", got)
	}

	got := briefSetIDFieldDecisions([]setIDFieldDecision{
		{FieldKey: "field_x", Decision: internalRuleDecisionAllow, ReasonCode: fieldVisibleInContextCode, Visibility: fieldVisibilityVisible},
		{FieldKey: "field_y", Decision: internalRuleDecisionDeny, ReasonCode: fieldHiddenInContextCode, Visibility: fieldVisibilityHidden},
	})
	if !strings.Contains(got, "field_x:allow:FIELD_VISIBLE_IN_CONTEXT:visible") || !strings.Contains(got, "field_y:deny:FIELD_HIDDEN_IN_CONTEXT:hidden") {
		t.Fatalf("brief=%q", got)
	}
}

func TestLogSetIDExplainAuditRedactsValues(t *testing.T) {
	var b strings.Builder
	origin := log.Writer()
	log.SetOutput(&b)
	t.Cleanup(func() {
		log.SetOutput(origin)
	})

	logSetIDExplainAudit(setIDExplainResponse{
		Decision:          internalRuleDecisionDeny,
		ReasonCode:        fieldHiddenInContextCode,
		CapabilityKey:     "staffing.assignment_create.field_policy",
		SetID:             "A0001",
		PolicyVersion:     "2026-03-01",
		FunctionalAreaKey: "staffing",
		Level:             explainLevelBrief,
		FieldDecisions: []setIDFieldDecision{
			{
				FieldKey:           "field_hidden",
				Decision:           internalRuleDecisionDeny,
				ReasonCode:         fieldHiddenInContextCode,
				Visibility:         fieldVisibilityHidden,
				ResolvedDefaultVal: "secret-v1",
				MaskedDefaultVal:   "***",
			},
		},
	})

	got := b.String()
	if strings.Contains(got, "secret-v1") {
		t.Fatalf("unexpected raw value in audit log: %q", got)
	}
	if !strings.Contains(got, "field_hidden:deny:FIELD_HIDDEN_IN_CONTEXT:hidden") {
		t.Fatalf("unexpected audit log: %q", got)
	}
}
