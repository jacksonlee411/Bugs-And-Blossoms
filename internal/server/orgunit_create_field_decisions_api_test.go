package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type orgUnitCreateFieldDecisionStoreStub struct {
	orgUnitStoreStub
	resolveOrgIDFn func(ctx context.Context, tenantID string, orgCode string) (int, error)
}

func (s orgUnitCreateFieldDecisionStoreStub) ResolveOrgID(ctx context.Context, tenantID string, orgCode string) (int, error) {
	if s.resolveOrgIDFn != nil {
		return s.resolveOrgIDFn(ctx, tenantID, orgCode)
	}
	return s.orgUnitStoreStub.ResolveOrgID(ctx, tenantID, orgCode)
}

func TestHandleOrgUnitCreateFieldDecisionsAPI(t *testing.T) {
	previousStore := defaultSetIDStrategyRegistryStore
	t.Cleanup(func() { useSetIDStrategyRegistryStore(previousStore) })
	resetFunctionalAreaSwitchStoreForTest()
	t.Cleanup(resetFunctionalAreaSwitchStoreForTest)
	resetPolicyActivationRuntimeForTest()

	newReq := func(rawURL string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, rawURL, nil)
		return req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	}

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/create-field-decisions", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOrgUnitCreateFieldDecisionsAPI(rec, req, orgUnitCreateFieldDecisionStoreStub{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/create-field-decisions?effective_date=2026-01-01", nil)
		rec := httptest.NewRecorder()
		handleOrgUnitCreateFieldDecisionsAPI(rec, req, orgUnitCreateFieldDecisionStoreStub{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("store missing", func(t *testing.T) {
		req := newReq("/org/api/org-units/create-field-decisions?effective_date=2026-01-01")
		rec := httptest.NewRecorder()
		handleOrgUnitCreateFieldDecisionsAPI(rec, req, nil)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("effective date required", func(t *testing.T) {
		req := newReq("/org/api/org-units/create-field-decisions")
		rec := httptest.NewRecorder()
		handleOrgUnitCreateFieldDecisionsAPI(rec, req, orgUnitCreateFieldDecisionStoreStub{})
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":"invalid_request"`) {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("effective date invalid", func(t *testing.T) {
		req := newReq("/org/api/org-units/create-field-decisions?effective_date=bad")
		rec := httptest.NewRecorder()
		handleOrgUnitCreateFieldDecisionsAPI(rec, req, orgUnitCreateFieldDecisionStoreStub{})
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":"invalid_effective_date"`) {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("parent org invalid", func(t *testing.T) {
		req := newReq("/org/api/org-units/create-field-decisions?effective_date=2026-01-01&parent_org_code=bad%7f")
		rec := httptest.NewRecorder()
		handleOrgUnitCreateFieldDecisionsAPI(rec, req, orgUnitCreateFieldDecisionStoreStub{})
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":"org_code_invalid"`) {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("parent org not found", func(t *testing.T) {
		req := newReq("/org/api/org-units/create-field-decisions?effective_date=2026-01-01&parent_org_code=ROOT")
		rec := httptest.NewRecorder()
		handleOrgUnitCreateFieldDecisionsAPI(rec, req, orgUnitCreateFieldDecisionStoreStub{
			resolveOrgIDFn: func(context.Context, string, string) (int, error) {
				return 0, orgunitpkg.ErrOrgCodeNotFound
			},
		})
		if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"code":"org_code_not_found"`) {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("parent org resolve context failed", func(t *testing.T) {
		req := newReq("/org/api/org-units/create-field-decisions?effective_date=2026-01-01&parent_org_code=ROOT")
		rec := httptest.NewRecorder()
		handleOrgUnitCreateFieldDecisionsAPI(rec, req, orgUnitCreateFieldDecisionStoreStub{
			resolveOrgIDFn: func(context.Context, string, string) (int, error) {
				return 0, errors.New("boom")
			},
		})
		if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), `"code":"boom"`) {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("capability context mismatch", func(t *testing.T) {
		req := newReq("/org/api/org-units/create-field-decisions?effective_date=2026-01-01")
		req.Header.Set("X-Actor-Scope", "saas")
		rec := httptest.NewRecorder()
		handleOrgUnitCreateFieldDecisionsAPI(rec, req, orgUnitCreateFieldDecisionStoreStub{})
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), capabilityReasonContextMismatch) {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("field policy missing fail closed", func(t *testing.T) {
		useSetIDStrategyRegistryStore(setIDStrategyRegistryStoreStub{})
		req := newReq("/org/api/org-units/create-field-decisions?effective_date=2026-01-01")
		rec := httptest.NewRecorder()
		handleOrgUnitCreateFieldDecisionsAPI(rec, req, orgUnitCreateFieldDecisionStoreStub{})
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), fieldPolicyMissingCode) {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("functional area disabled", func(t *testing.T) {
		defaultFunctionalAreaSwitchStore.setEnabled("t1", "org_foundation", false)
		t.Cleanup(func() {
			defaultFunctionalAreaSwitchStore.setEnabled("t1", "org_foundation", true)
		})
		req := newReq("/org/api/org-units/create-field-decisions?effective_date=2026-01-01")
		rec := httptest.NewRecorder()
		handleOrgUnitCreateFieldDecisionsAPI(rec, req, orgUnitCreateFieldDecisionStoreStub{})
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), functionalAreaDisabledCode) {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("success tenant level", func(t *testing.T) {
		state, err := defaultPolicyActivationRuntime.setDraft("t1", orgUnitCreateFieldPolicyCapabilityKey, "2026-03-01", "tester")
		if err != nil {
			t.Fatalf("setDraft err=%v", err)
		}
		if _, err := defaultPolicyActivationRuntime.activate("t1", orgUnitCreateFieldPolicyCapabilityKey, state.DraftPolicyVersion, "tester"); err != nil {
			t.Fatalf("activate err=%v", err)
		}
		var captured []string
		useSetIDStrategyRegistryStore(setIDStrategyRegistryStoreStub{
			resolveFieldDecisionFn: func(_ context.Context, _ string, capabilityKey string, fieldKey string, businessUnitID string, asOf string) (setIDFieldDecision, error) {
				captured = append(captured, capabilityKey+"|"+fieldKey+"|"+businessUnitID+"|"+asOf)
				switch fieldKey {
				case orgUnitCreateFieldOrgCode:
					return setIDFieldDecision{
						CapabilityKey:  capabilityKey,
						FieldKey:       fieldKey,
						Required:       true,
						Visible:        true,
						Maintainable:   false,
						DefaultRuleRef: `next_org_code("F", 8)`,
					}, nil
				case orgUnitCreateFieldOrgType:
					return setIDFieldDecision{
						CapabilityKey:      capabilityKey,
						FieldKey:           fieldKey,
						Required:           true,
						Visible:            true,
						Maintainable:       true,
						ResolvedDefaultVal: "11",
						AllowedValueCodes:  []string{"11"},
					}, nil
				default:
					return setIDFieldDecision{}, errors.New("unexpected field")
				}
			},
		})

		req := newReq("/org/api/org-units/create-field-decisions?effective_date=2026-01-01")
		rec := httptest.NewRecorder()
		handleOrgUnitCreateFieldDecisionsAPI(rec, req, orgUnitCreateFieldDecisionStoreStub{})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if len(captured) != 2 {
			t.Fatalf("captured=%v", captured)
		}

		var response orgUnitCreateFieldDecisionsAPIResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("json err=%v body=%s", err, rec.Body.String())
		}
		if response.CapabilityKey != orgUnitCreateFieldPolicyCapabilityKey {
			t.Fatalf("capability_key=%q", response.CapabilityKey)
		}
		if response.BusinessUnitID != "" {
			t.Fatalf("business_unit_id=%q", response.BusinessUnitID)
		}
		expectedPolicyVersion, parts := resolveOrgUnitEffectivePolicyVersion("t1", orgUnitCreateFieldPolicyCapabilityKey)
		if response.PolicyVersion != expectedPolicyVersion {
			t.Fatalf("policy_version=%q want=%q", response.PolicyVersion, expectedPolicyVersion)
		}
		if response.BaselineCapabilityKey != orgUnitWriteFieldPolicyCapabilityKey {
			t.Fatalf("baseline_capability_key=%q", response.BaselineCapabilityKey)
		}
		if response.PolicyVersionAlg != orgUnitEffectivePolicyVersionAlgorithm {
			t.Fatalf("policy_version_alg=%q", response.PolicyVersionAlg)
		}
		if response.IntentPolicyVersion != parts.IntentPolicyVersion || response.BaselinePolicyVersion != parts.BaselinePolicyVersion {
			t.Fatalf("parts mismatch response=%+v parts=%+v", response, parts)
		}
		if len(response.FieldDecisions) != 2 {
			t.Fatalf("field_decisions=%+v", response.FieldDecisions)
		}
	})
}

func TestResolveCreateFieldDecisionBusinessUnitID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/create-field-decisions", nil)

	t.Run("empty parent uses tenant level", func(t *testing.T) {
		got, err := resolveCreateFieldDecisionBusinessUnitID(req, orgUnitCreateFieldDecisionStoreStub{}, "t1", "")
		if err != nil || got != "" {
			t.Fatalf("got=%q err=%v", got, err)
		}
	})

	t.Run("invalid normalized business unit id", func(t *testing.T) {
		_, err := resolveCreateFieldDecisionBusinessUnitID(req, orgUnitCreateFieldDecisionStoreStub{
			resolveOrgIDFn: func(context.Context, string, string) (int, error) {
				return 123, nil
			},
		}, "t1", "ROOT")
		if err == nil || err.Error() != "invalid business_unit_id" {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("success with parent org", func(t *testing.T) {
		got, err := resolveCreateFieldDecisionBusinessUnitID(req, orgUnitCreateFieldDecisionStoreStub{
			resolveOrgIDFn: func(context.Context, string, string) (int, error) {
				return 10000001, nil
			},
		}, "t1", "ROOT")
		if err != nil || got != "10000001" {
			t.Fatalf("got=%q err=%v", got, err)
		}
	})

	t.Run("normalize and resolve errors", func(t *testing.T) {
		if _, err := resolveCreateFieldDecisionBusinessUnitID(req, orgUnitCreateFieldDecisionStoreStub{}, "t1", "bad\x7f"); err == nil {
			t.Fatal("expected normalize error")
		}
		if _, err := resolveCreateFieldDecisionBusinessUnitID(req, orgUnitCreateFieldDecisionStoreStub{
			resolveOrgIDFn: func(context.Context, string, string) (int, error) {
				return 0, errors.New("boom")
			},
		}, "t1", "ROOT"); err == nil || err.Error() != "boom" {
			t.Fatalf("err=%v", err)
		}
	})
}
