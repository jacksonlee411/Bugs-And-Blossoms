package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/google/cel-go/cel"
	celtypes "github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type orgUnitWriteResolverStoreStub struct {
	orgUnitWriteStoreStub
	findByRequestFn func(ctx context.Context, tenantID string, requestID string) (types.OrgUnitEvent, bool, error)
	resolvePolicyFn func(ctx context.Context, tenantID string, fieldKey string, scopeType string, scopeKey string, asOf string) (types.TenantFieldPolicy, bool, error)
}

func (s orgUnitWriteResolverStoreStub) FindEventByRequestID(ctx context.Context, tenantID string, requestID string) (types.OrgUnitEvent, bool, error) {
	if s.findByRequestFn != nil {
		return s.findByRequestFn(ctx, tenantID, requestID)
	}
	return types.OrgUnitEvent{}, false, nil
}

func (s orgUnitWriteResolverStoreStub) ResolveTenantFieldPolicy(ctx context.Context, tenantID string, fieldKey string, scopeType string, scopeKey string, asOf string) (types.TenantFieldPolicy, bool, error) {
	if s.resolvePolicyFn != nil {
		return s.resolvePolicyFn(ctx, tenantID, fieldKey, scopeType, scopeKey, asOf)
	}
	return types.TenantFieldPolicy{}, false, nil
}

type orgUnitWriteAutoCodeStoreStub struct {
	orgUnitWriteSetIDResolverStoreStub
	submitCreateAutoFn func(
		ctx context.Context,
		tenantID string,
		eventUUID string,
		effectiveDate string,
		payload json.RawMessage,
		requestID string,
		initiatorUUID string,
		prefix string,
		width int,
	) (int64, string, error)
}

func (s orgUnitWriteAutoCodeStoreStub) SubmitCreateEventWithGeneratedCode(
	ctx context.Context,
	tenantID string,
	eventUUID string,
	effectiveDate string,
	payload json.RawMessage,
	requestID string,
	initiatorUUID string,
	prefix string,
	width int,
) (int64, string, error) {
	if s.submitCreateAutoFn != nil {
		return s.submitCreateAutoFn(ctx, tenantID, eventUUID, effectiveDate, payload, requestID, initiatorUUID, prefix, width)
	}
	return 0, "", errors.New("SubmitCreateEventWithGeneratedCode not mocked")
}

type orgUnitWriteSetIDResolverStoreStub struct {
	orgUnitWriteStoreStub
	resolveDecisionFn func(ctx context.Context, tenantID string, capabilityKey string, fieldKey string, businessUnitID string, asOf string) (types.SetIDStrategyFieldDecision, bool, error)
}

func (s orgUnitWriteSetIDResolverStoreStub) ResolveSetIDStrategyFieldDecision(
	ctx context.Context,
	tenantID string,
	capabilityKey string,
	fieldKey string,
	businessUnitID string,
	asOf string,
) (types.SetIDStrategyFieldDecision, bool, error) {
	if s.resolveDecisionFn != nil {
		return s.resolveDecisionFn(ctx, tenantID, capabilityKey, fieldKey, businessUnitID, asOf)
	}
	return types.SetIDStrategyFieldDecision{}, false, nil
}

func TestResolveCreateByRequestID_Branches(t *testing.T) {
	ctx := context.Background()

	t.Run("store without request reader", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, found, err := svc.resolveCreateByRequestID(ctx, "t1", "r1")
		if err != nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("reader error", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{},
			findByRequestFn: func(context.Context, string, string) (types.OrgUnitEvent, bool, error) {
				return types.OrgUnitEvent{}, false, errors.New("boom")
			},
		})
		if _, found, err := svc.resolveCreateByRequestID(ctx, "t1", "r1"); err == nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{},
			findByRequestFn: func(context.Context, string, string) (types.OrgUnitEvent, bool, error) {
				return types.OrgUnitEvent{}, false, nil
			},
		})
		_, found, err := svc.resolveCreateByRequestID(ctx, "t1", "r1")
		if err != nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("conflict when event type is not create", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{},
			findByRequestFn: func(context.Context, string, string) (types.OrgUnitEvent, bool, error) {
				return types.OrgUnitEvent{EventType: types.OrgUnitEventUpdate}, true, nil
			},
		})
		if _, found, err := svc.resolveCreateByRequestID(ctx, "t1", "r1"); err == nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("payload decode error", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{},
			findByRequestFn: func(context.Context, string, string) (types.OrgUnitEvent, bool, error) {
				return types.OrgUnitEvent{
					EventType: types.OrgUnitEventCreate,
					Payload:   json.RawMessage("{"),
				}, true, nil
			},
		})
		if _, found, err := svc.resolveCreateByRequestID(ctx, "t1", "r1"); err == nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("fallback resolve org code error", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{
				resolveOrgCodeFn: func(context.Context, string, int) (string, error) {
					return "", errors.New("resolve")
				},
			},
			findByRequestFn: func(context.Context, string, string) (types.OrgUnitEvent, bool, error) {
				return types.OrgUnitEvent{
					EventType:     types.OrgUnitEventCreate,
					OrgID:         10000001,
					EffectiveDate: "2026-01-01",
					EventUUID:     "e1",
					Payload:       json.RawMessage(`{"name":"Root"}`),
				}, true, nil
			},
		})
		if _, found, err := svc.resolveCreateByRequestID(ctx, "t1", "r1"); err == nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("fallback resolve org code success", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{
				resolveOrgCodeFn: func(context.Context, string, int) (string, error) {
					return "ROOT", nil
				},
			},
			findByRequestFn: func(context.Context, string, string) (types.OrgUnitEvent, bool, error) {
				return types.OrgUnitEvent{
					EventType:     types.OrgUnitEventCreate,
					OrgID:         10000001,
					EffectiveDate: "2026-01-01",
					EventUUID:     "e1",
					Payload:       json.RawMessage(`{"name":"Root"}`),
				}, true, nil
			},
		})
		result, found, err := svc.resolveCreateByRequestID(ctx, "t1", "r1")
		if err != nil || !found {
			t.Fatalf("result=%+v found=%v err=%v", result, found, err)
		}
		if result.OrgCode != "ROOT" || result.Fields["org_code"] != "ROOT" {
			t.Fatalf("result=%+v", result)
		}
	})

	t.Run("payload org code success", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{},
			findByRequestFn: func(context.Context, string, string) (types.OrgUnitEvent, bool, error) {
				return types.OrgUnitEvent{
					EventType:     types.OrgUnitEventCreate,
					OrgID:         10000001,
					EffectiveDate: "2026-01-01",
					EventUUID:     "e1",
					Payload:       json.RawMessage(`{"org_code":" ROOT "}`),
				}, true, nil
			},
		})
		result, found, err := svc.resolveCreateByRequestID(ctx, "t1", "r1")
		if err != nil || !found {
			t.Fatalf("result=%+v found=%v err=%v", result, found, err)
		}
		if result.OrgCode != "ROOT" {
			t.Fatalf("result=%+v", result)
		}
	})
}

func TestOrgCodeFromEventPayload(t *testing.T) {
	if code, err := orgCodeFromEventPayload(nil); err != nil || code != "" {
		t.Fatalf("code=%q err=%v", code, err)
	}
	if _, err := orgCodeFromEventPayload(json.RawMessage("{")); err == nil {
		t.Fatal("expected json error")
	}
	code, err := orgCodeFromEventPayload(json.RawMessage(`{"org_code":" ROOT "}`))
	if err != nil || code != "ROOT" {
		t.Fatalf("code=%q err=%v", code, err)
	}
}

func TestApplyCreatePolicyDefaults_Branches(t *testing.T) {
	ctx := context.Background()

	t.Run("store without setid resolver", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		req := &WriteOrgUnitRequest{}
		if _, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", nil, req); err == nil || err.Error() != errFieldPolicyMissing {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("org_code decision missing", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteSetIDResolverStoreStub{
			resolveDecisionFn: func(context.Context, string, string, string, string, string) (types.SetIDStrategyFieldDecision, bool, error) {
				return types.SetIDStrategyFieldDecision{}, false, nil
			},
		})
		req := &WriteOrgUnitRequest{}
		if _, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", nil, req); err == nil || err.Error() != errFieldPolicyMissing {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestParseCompileAndMapCreateAutoCodeHelpers(t *testing.T) {
	celCompileCache = sync.Map{}

	t.Run("parse empty expression", func(t *testing.T) {
		if _, err := parseNextOrgCodeRule(" "); err == nil || err.Error() != errFieldPolicyExprInvalid {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("parse regex mismatch", func(t *testing.T) {
		if _, err := parseNextOrgCodeRule(`"x"`); err == nil || err.Error() != errDefaultRuleEvalFailed {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("parse invalid width", func(t *testing.T) {
		if _, err := parseNextOrgCodeRule(`next_org_code("O", 0)`); err == nil || err.Error() != errDefaultRuleEvalFailed {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("parse empty prefix", func(t *testing.T) {
		spec, err := parseNextOrgCodeRule(`next_org_code("", 6)`)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if spec.Prefix != "" || spec.Width != 6 {
			t.Fatalf("spec=%+v", spec)
		}
	})

	t.Run("parse success", func(t *testing.T) {
		spec, err := parseNextOrgCodeRule(`next_org_code("ORG", 3)`)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if spec.Prefix != "ORG" || spec.Width != 3 {
			t.Fatalf("spec=%+v", spec)
		}
	})

	t.Run("parse single quotes rejected", func(t *testing.T) {
		if _, err := parseNextOrgCodeRule(`next_org_code('ORG', 3)`); err == nil || err.Error() != errDefaultRuleEvalFailed {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("compile syntax error", func(t *testing.T) {
		if err := compileCELExpr(`next_org_code("O", )`); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("compile non-string output", func(t *testing.T) {
		if err := compileCELExpr(`1+1`); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("compile cache hit", func(t *testing.T) {
		expr := `next_org_code("O", 2)`
		if err := compileCELExpr(expr); err != nil {
			t.Fatalf("first compile err=%v", err)
		}
		if err := compileCELExpr(expr); err != nil {
			t.Fatalf("second compile err=%v", err)
		}
	})

	t.Run("mapCreateAutoCodeError", func(t *testing.T) {
		if got := mapCreateAutoCodeError(errors.New("org_code_exhausted")); got.Error() != errOrgCodeExhausted {
			t.Fatalf("got=%v", got)
		}
		if got := mapCreateAutoCodeError(errors.New("ORG_REQUEST_ID_CONFLICT")); got.Error() != errOrgRequestIDConflict {
			t.Fatalf("got=%v", got)
		}
		if got := mapCreateAutoCodeError(errors.New("duplicate key value violates unique constraint org_unit_codes_org_code_unique")); got.Error() != errOrgCodeConflict {
			t.Fatalf("got=%v", got)
		}
		raw := errors.New("raw")
		if got := mapCreateAutoCodeError(raw); got != raw {
			t.Fatalf("got=%v want=%v", got, raw)
		}
	})
}

func TestWrite_CreateOrg_AutoCodeBranches(t *testing.T) {
	ctx := context.Background()

	t.Run("idempotency resolve error", func(t *testing.T) {
		store := orgUnitWriteResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{},
			findByRequestFn: func(context.Context, string, string) (types.OrgUnitEvent, bool, error) {
				return types.OrgUnitEvent{}, false, errors.New("boom")
			},
		}
		svc := NewOrgUnitWriteService(store)
		name := "Root"
		_, err := svc.Write(ctx, "t1", WriteOrgUnitRequest{
			Intent:        "create_org",
			EffectiveDate: "2026-01-01",
			RequestID:     "r1",
			Patch:         OrgUnitWritePatch{Name: &name},
		})
		if err == nil || err.Error() != "boom" {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("idempotency hit returns stored result", func(t *testing.T) {
		store := orgUnitWriteResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{
				listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
					t.Fatalf("listEnabledFieldConfigs should not be called")
					return nil, nil
				},
			},
			findByRequestFn: func(context.Context, string, string) (types.OrgUnitEvent, bool, error) {
				return types.OrgUnitEvent{
					EventType:     types.OrgUnitEventCreate,
					EventUUID:     "e1",
					EffectiveDate: "2026-01-01",
					Payload:       json.RawMessage(`{"org_code":"ROOT"}`),
				}, true, nil
			},
		}
		svc := NewOrgUnitWriteService(store)
		name := "Root"
		result, err := svc.Write(ctx, "t1", WriteOrgUnitRequest{
			Intent:        "create_org",
			EffectiveDate: "2026-01-01",
			RequestID:     "r1",
			Patch:         OrgUnitWritePatch{Name: &name},
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if result.OrgCode != "ROOT" || result.EventUUID != "e1" {
			t.Fatalf("result=%+v", result)
		}
	})

	t.Run("auto code without policy returns default rule required", func(t *testing.T) {
		store := orgUnitWriteStoreStub{
			listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
				return []types.TenantFieldConfig{}, nil
			},
		}
		svc := NewOrgUnitWriteService(store)
		name := "Root"
		_, err := svc.Write(ctx, "t1", WriteOrgUnitRequest{
			Intent:        "create_org",
			EffectiveDate: "2026-01-01",
			RequestID:     "r1",
			Patch:         OrgUnitWritePatch{Name: &name},
		})
		if err == nil || err.Error() != errFieldPolicyMissing {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("create apply policy defaults returns error", func(t *testing.T) {
		store := orgUnitWriteSetIDResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{
				listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
					return []types.TenantFieldConfig{}, nil
				},
			},
			resolveDecisionFn: func(context.Context, string, string, string, string, string) (types.SetIDStrategyFieldDecision, bool, error) {
				return types.SetIDStrategyFieldDecision{}, false, errors.New("policy")
			},
		}
		svc := NewOrgUnitWriteService(store)
		name := "Root"
		_, err := svc.Write(ctx, "t1", WriteOrgUnitRequest{
			Intent:        "create_org",
			EffectiveDate: "2026-01-01",
			RequestID:     "r1",
			Patch:         OrgUnitWritePatch{Name: &name},
		})
		if err == nil || err.Error() != "policy" {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("non-create intent without org_code", func(t *testing.T) {
		store := orgUnitWriteStoreStub{
			listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
				return []types.TenantFieldConfig{}, nil
			},
		}
		svc := NewOrgUnitWriteService(store)
		name := "Root"
		_, err := svc.Write(ctx, "t1", WriteOrgUnitRequest{
			Intent:        "add_version",
			EffectiveDate: "2026-01-01",
			RequestID:     "r1",
			Patch:         OrgUnitWritePatch{Name: &name},
		})
		if err == nil || err.Error() != errOrgCodeInvalid {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("auto code resolver exists but submitter missing", func(t *testing.T) {
		store := orgUnitWriteSetIDResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{
				listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
					return []types.TenantFieldConfig{}, nil
				},
			},
			resolveDecisionFn: func(_ context.Context, _ string, _ string, fieldKey string, _ string, _ string) (types.SetIDStrategyFieldDecision, bool, error) {
				switch fieldKey {
				case orgUnitCreateFieldOrgCode:
					return types.SetIDStrategyFieldDecision{
						FieldKey:       orgUnitCreateFieldOrgCode,
						Maintainable:   false,
						Required:       true,
						DefaultRuleRef: `next_org_code("O", 6)`,
					}, true, nil
				case orgUnitCreateFieldOrgType:
					return types.SetIDStrategyFieldDecision{FieldKey: orgUnitCreateFieldOrgType, Maintainable: true}, true, nil
				default:
					return types.SetIDStrategyFieldDecision{}, false, nil
				}
			},
		}
		svc := NewOrgUnitWriteService(store)
		name := "Root"
		_, err := svc.Write(ctx, "t1", WriteOrgUnitRequest{
			Intent:        "create_org",
			EffectiveDate: "2026-01-01",
			RequestID:     "r1",
			Patch:         OrgUnitWritePatch{Name: &name},
		})
		if err == nil || err.Error() != errDefaultRuleEvalFailed {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("auto submit error is mapped", func(t *testing.T) {
		store := orgUnitWriteAutoCodeStoreStub{
			orgUnitWriteSetIDResolverStoreStub: orgUnitWriteSetIDResolverStoreStub{
				orgUnitWriteStoreStub: orgUnitWriteStoreStub{
					listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
						return []types.TenantFieldConfig{}, nil
					},
				},
				resolveDecisionFn: func(_ context.Context, _ string, _ string, fieldKey string, _ string, _ string) (types.SetIDStrategyFieldDecision, bool, error) {
					switch fieldKey {
					case orgUnitCreateFieldOrgCode:
						return types.SetIDStrategyFieldDecision{
							FieldKey:       orgUnitCreateFieldOrgCode,
							Maintainable:   false,
							Required:       true,
							DefaultRuleRef: `next_org_code("O", 6)`,
						}, true, nil
					case orgUnitCreateFieldOrgType:
						return types.SetIDStrategyFieldDecision{FieldKey: orgUnitCreateFieldOrgType, Maintainable: true}, true, nil
					default:
						return types.SetIDStrategyFieldDecision{}, false, nil
					}
				},
			},
			submitCreateAutoFn: func(context.Context, string, string, string, json.RawMessage, string, string, string, int) (int64, string, error) {
				return 0, "", errors.New("duplicate key value violates unique constraint org_unit_codes_org_code_unique")
			},
		}
		svc := NewOrgUnitWriteService(store)
		name := "Root"
		_, err := svc.Write(ctx, "t1", WriteOrgUnitRequest{
			Intent:        "create_org",
			EffectiveDate: "2026-01-01",
			RequestID:     "r1",
			Patch:         OrgUnitWritePatch{Name: &name},
		})
		if err == nil || err.Error() != errOrgCodeConflict {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("auto submit success", func(t *testing.T) {
		store := orgUnitWriteAutoCodeStoreStub{
			orgUnitWriteSetIDResolverStoreStub: orgUnitWriteSetIDResolverStoreStub{
				orgUnitWriteStoreStub: orgUnitWriteStoreStub{
					listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
						return []types.TenantFieldConfig{}, nil
					},
				},
				resolveDecisionFn: func(_ context.Context, _ string, _ string, fieldKey string, _ string, _ string) (types.SetIDStrategyFieldDecision, bool, error) {
					switch fieldKey {
					case orgUnitCreateFieldOrgCode:
						return types.SetIDStrategyFieldDecision{
							FieldKey:       orgUnitCreateFieldOrgCode,
							Maintainable:   false,
							Required:       true,
							DefaultRuleRef: `next_org_code("O", 6)`,
						}, true, nil
					case orgUnitCreateFieldOrgType:
						return types.SetIDStrategyFieldDecision{FieldKey: orgUnitCreateFieldOrgType, Maintainable: true}, true, nil
					default:
						return types.SetIDStrategyFieldDecision{}, false, nil
					}
				},
			},
			submitCreateAutoFn: func(context.Context, string, string, string, json.RawMessage, string, string, string, int) (int64, string, error) {
				return 9, "O000001", nil
			},
		}
		svc := NewOrgUnitWriteService(store)
		name := "Root"
		result, err := svc.Write(ctx, "t1", WriteOrgUnitRequest{
			Intent:        "create_org",
			EffectiveDate: "2026-01-01",
			RequestID:     "r1",
			Patch:         OrgUnitWritePatch{Name: &name},
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if result.OrgCode != "O000001" || result.Fields["org_code"] != "O000001" {
			t.Fatalf("result=%+v", result)
		}
	})

}

func TestApplyCreatePolicyDefaults_FromSetIDRegistry(t *testing.T) {
	ctx := context.Background()

	buildResolver := func(
		orgCodeDecision types.SetIDStrategyFieldDecision,
		orgTypeDecision types.SetIDStrategyFieldDecision,
	) orgUnitWriteSetIDResolverStoreStub {
		return orgUnitWriteSetIDResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{
				resolveOrgIDFn: func(context.Context, string, string) (int, error) {
					return 10000001, nil
				},
			},
			resolveDecisionFn: func(_ context.Context, _ string, capabilityKey string, fieldKey string, businessUnitID string, _ string) (types.SetIDStrategyFieldDecision, bool, error) {
				if capabilityKey != orgUnitCreateFieldPolicyCapabilityKey {
					return types.SetIDStrategyFieldDecision{}, false, errors.New("unexpected capability")
				}
				if businessUnitID != "10000001" {
					return types.SetIDStrategyFieldDecision{}, false, errors.New("unexpected business unit")
				}
				switch fieldKey {
				case orgUnitCreateFieldOrgCode:
					return orgCodeDecision, true, nil
				case orgUnitCreateFieldOrgType:
					return orgTypeDecision, true, nil
				default:
					return types.SetIDStrategyFieldDecision{}, false, nil
				}
			},
		}
	}

	t.Run("reject not maintainable org_code manual input", func(t *testing.T) {
		svc := newWriteService(buildResolver(
			types.SetIDStrategyFieldDecision{
				FieldKey:       orgUnitCreateFieldOrgCode,
				Required:       true,
				Maintainable:   false,
				DefaultRuleRef: `next_org_code("F", 8)`,
			},
			types.SetIDStrategyFieldDecision{
				FieldKey:          orgUnitCreateFieldOrgType,
				Maintainable:      true,
				Required:          true,
				DefaultValue:      "11",
				AllowedValueCodes: []string{"11"},
			},
		))

		req := &WriteOrgUnitRequest{
			OrgCode: "MANUAL01",
			Patch: OrgUnitWritePatch{
				ParentOrgCode: stringPtr("ROOT"),
			},
		}
		if _, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", nil, req); err == nil || err.Error() != errFieldNotMaintainable {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("required org_type missing rejected", func(t *testing.T) {
		svc := newWriteService(buildResolver(
			types.SetIDStrategyFieldDecision{
				FieldKey:       orgUnitCreateFieldOrgCode,
				Required:       true,
				Maintainable:   false,
				DefaultRuleRef: `next_org_code("F", 8)`,
			},
			types.SetIDStrategyFieldDecision{
				FieldKey:          orgUnitCreateFieldOrgType,
				Maintainable:      true,
				Required:          true,
				AllowedValueCodes: []string{"11"},
			},
		))

		req := &WriteOrgUnitRequest{
			Patch: OrgUnitWritePatch{
				ParentOrgCode: stringPtr("ROOT"),
			},
		}
		if _, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", nil, req); err == nil || err.Error() != errFieldRequiredValueMissing {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("required false org_type keeps empty", func(t *testing.T) {
		svc := newWriteService(buildResolver(
			types.SetIDStrategyFieldDecision{
				FieldKey:       orgUnitCreateFieldOrgCode,
				Required:       true,
				Maintainable:   false,
				DefaultRuleRef: `next_org_code("F", 8)`,
			},
			types.SetIDStrategyFieldDecision{
				FieldKey:          orgUnitCreateFieldOrgType,
				Maintainable:      true,
				Required:          false,
				AllowedValueCodes: []string{"10"},
			},
		))

		req := &WriteOrgUnitRequest{
			Patch: OrgUnitWritePatch{
				ParentOrgCode: stringPtr("ROOT"),
				Ext:           map[string]any{orgUnitCreateFieldOrgType: ""},
			},
		}
		spec, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", nil, req)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if spec == nil || spec.Prefix != "F" || spec.Width != 8 {
			t.Fatalf("spec=%+v", spec)
		}
		if req.Patch.Ext != nil {
			if _, ok := req.Patch.Ext[orgUnitCreateFieldOrgType]; ok {
				t.Fatalf("ext=%v", req.Patch.Ext)
			}
		}
	})

	t.Run("required false org_type explicit empty does not refill default", func(t *testing.T) {
		svc := newWriteService(buildResolver(
			types.SetIDStrategyFieldDecision{
				FieldKey:       orgUnitCreateFieldOrgCode,
				Required:       true,
				Maintainable:   false,
				DefaultRuleRef: `next_org_code("F", 8)`,
			},
			types.SetIDStrategyFieldDecision{
				FieldKey:          orgUnitCreateFieldOrgType,
				Maintainable:      true,
				Required:          false,
				DefaultValue:      "10",
				AllowedValueCodes: []string{"10"},
			},
		))

		req := &WriteOrgUnitRequest{
			Patch: OrgUnitWritePatch{
				ParentOrgCode: stringPtr("ROOT"),
				Ext:           map[string]any{orgUnitCreateFieldOrgType: ""},
			},
		}
		spec, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", nil, req)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if spec == nil || spec.Prefix != "F" || spec.Width != 8 {
			t.Fatalf("spec=%+v", spec)
		}
		if req.Patch.Ext != nil {
			if _, ok := req.Patch.Ext[orgUnitCreateFieldOrgType]; ok {
				t.Fatalf("ext=%v", req.Patch.Ext)
			}
		}
	})

	t.Run("maintainable org_code keeps user value and fills org_type default", func(t *testing.T) {
		svc := newWriteService(buildResolver(
			types.SetIDStrategyFieldDecision{
				FieldKey:          orgUnitCreateFieldOrgCode,
				Required:          true,
				Maintainable:      true,
				AllowedValueCodes: []string{"A001"},
			},
			types.SetIDStrategyFieldDecision{
				FieldKey:          orgUnitCreateFieldOrgType,
				Maintainable:      true,
				Required:          false,
				DefaultValue:      "10",
				AllowedValueCodes: []string{"10"},
			},
		))

		req := &WriteOrgUnitRequest{
			OrgCode: "A001",
			Patch: OrgUnitWritePatch{
				ParentOrgCode: stringPtr("ROOT"),
			},
		}
		spec, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", nil, req)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if spec != nil {
			t.Fatalf("spec=%+v", spec)
		}
		if req.OrgCode != "A001" {
			t.Fatalf("org_code=%q", req.OrgCode)
		}
		if req.Patch.Ext == nil || req.Patch.Ext[orgUnitCreateFieldOrgType] != "10" {
			t.Fatalf("ext=%v", req.Patch.Ext)
		}
	})

	t.Run("org_type option out of allowlist rejected", func(t *testing.T) {
		svc := newWriteService(buildResolver(
			types.SetIDStrategyFieldDecision{
				FieldKey:       orgUnitCreateFieldOrgCode,
				Required:       true,
				Maintainable:   false,
				DefaultRuleRef: `next_org_code("F", 8)`,
			},
			types.SetIDStrategyFieldDecision{
				FieldKey:          orgUnitCreateFieldOrgType,
				Maintainable:      true,
				Required:          false,
				AllowedValueCodes: []string{"10"},
			},
		))

		req := &WriteOrgUnitRequest{
			Patch: OrgUnitWritePatch{
				ParentOrgCode: stringPtr("ROOT"),
				Ext:           map[string]any{orgUnitCreateFieldOrgType: "11"},
			},
		}
		if _, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", nil, req); err == nil || err.Error() != errFieldOptionNotAllowed {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("org_code resolver error mapped", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteSetIDResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{
				resolveOrgIDFn: func(context.Context, string, string) (int, error) { return 10000001, nil },
			},
			resolveDecisionFn: func(context.Context, string, string, string, string, string) (types.SetIDStrategyFieldDecision, bool, error) {
				return types.SetIDStrategyFieldDecision{}, false, errors.New("FIELD_DEFAULT_RULE_MISSING")
			},
		})
		req := &WriteOrgUnitRequest{Patch: OrgUnitWritePatch{ParentOrgCode: stringPtr("ROOT")}}
		if _, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", nil, req); err == nil || err.Error() != errDefaultRuleRequired {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("org_code policy missing", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteSetIDResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{
				resolveOrgIDFn: func(context.Context, string, string) (int, error) { return 10000001, nil },
			},
			resolveDecisionFn: func(_ context.Context, _ string, _ string, fieldKey string, _ string, _ string) (types.SetIDStrategyFieldDecision, bool, error) {
				if fieldKey == orgUnitCreateFieldOrgCode {
					return types.SetIDStrategyFieldDecision{}, false, nil
				}
				return types.SetIDStrategyFieldDecision{FieldKey: orgUnitCreateFieldOrgType, Maintainable: true}, true, nil
			},
		})
		req := &WriteOrgUnitRequest{Patch: OrgUnitWritePatch{ParentOrgCode: stringPtr("ROOT")}}
		if _, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", nil, req); err == nil || err.Error() != errFieldPolicyMissing {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("org_code required missing", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteSetIDResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{
				resolveOrgIDFn: func(context.Context, string, string) (int, error) { return 10000001, nil },
			},
			resolveDecisionFn: func(_ context.Context, _ string, _ string, fieldKey string, _ string, _ string) (types.SetIDStrategyFieldDecision, bool, error) {
				switch fieldKey {
				case orgUnitCreateFieldOrgCode:
					return types.SetIDStrategyFieldDecision{FieldKey: orgUnitCreateFieldOrgCode, Maintainable: true, Required: true}, true, nil
				case orgUnitCreateFieldOrgType:
					return types.SetIDStrategyFieldDecision{FieldKey: orgUnitCreateFieldOrgType, Maintainable: true}, true, nil
				default:
					return types.SetIDStrategyFieldDecision{}, false, nil
				}
			},
		})
		req := &WriteOrgUnitRequest{Patch: OrgUnitWritePatch{ParentOrgCode: stringPtr("ROOT")}}
		if _, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", nil, req); err == nil || err.Error() != errFieldRequiredValueMissing {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("org_code option out of allowlist", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteSetIDResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{
				resolveOrgIDFn: func(context.Context, string, string) (int, error) { return 10000001, nil },
			},
			resolveDecisionFn: func(_ context.Context, _ string, _ string, fieldKey string, _ string, _ string) (types.SetIDStrategyFieldDecision, bool, error) {
				switch fieldKey {
				case orgUnitCreateFieldOrgCode:
					return types.SetIDStrategyFieldDecision{FieldKey: orgUnitCreateFieldOrgCode, Maintainable: true, AllowedValueCodes: []string{"A001"}}, true, nil
				case orgUnitCreateFieldOrgType:
					return types.SetIDStrategyFieldDecision{FieldKey: orgUnitCreateFieldOrgType, Maintainable: true}, true, nil
				default:
					return types.SetIDStrategyFieldDecision{}, false, nil
				}
			},
		})
		req := &WriteOrgUnitRequest{
			OrgCode: "B001",
			Patch:   OrgUnitWritePatch{ParentOrgCode: stringPtr("ROOT")},
		}
		if _, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", nil, req); err == nil || err.Error() != errFieldOptionNotAllowed {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("org_type resolver error and missing", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteSetIDResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{
				resolveOrgIDFn: func(context.Context, string, string) (int, error) { return 10000001, nil },
			},
			resolveDecisionFn: func(_ context.Context, _ string, _ string, fieldKey string, _ string, _ string) (types.SetIDStrategyFieldDecision, bool, error) {
				if fieldKey == orgUnitCreateFieldOrgCode {
					return types.SetIDStrategyFieldDecision{
						FieldKey:       orgUnitCreateFieldOrgCode,
						Maintainable:   false,
						DefaultRuleRef: `next_org_code("F", 8)`,
						Required:       true,
					}, true, nil
				}
				return types.SetIDStrategyFieldDecision{}, false, errors.New(errFieldPolicyConflict)
			},
		})
		req := &WriteOrgUnitRequest{Patch: OrgUnitWritePatch{ParentOrgCode: stringPtr("ROOT")}}
		if _, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", nil, req); err == nil || err.Error() != errFieldPolicyConflict {
			t.Fatalf("err=%v", err)
		}

		svc = newWriteService(orgUnitWriteSetIDResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{
				resolveOrgIDFn: func(context.Context, string, string) (int, error) { return 10000001, nil },
			},
			resolveDecisionFn: func(_ context.Context, _ string, _ string, fieldKey string, _ string, _ string) (types.SetIDStrategyFieldDecision, bool, error) {
				if fieldKey == orgUnitCreateFieldOrgCode {
					return types.SetIDStrategyFieldDecision{
						FieldKey:       orgUnitCreateFieldOrgCode,
						Maintainable:   false,
						DefaultRuleRef: `next_org_code("F", 8)`,
						Required:       true,
					}, true, nil
				}
				return types.SetIDStrategyFieldDecision{}, false, nil
			},
		})
		if _, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", nil, req); err == nil || err.Error() != errFieldPolicyMissing {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("org_type ext type invalid", func(t *testing.T) {
		svc := newWriteService(buildResolver(
			types.SetIDStrategyFieldDecision{
				FieldKey:       orgUnitCreateFieldOrgCode,
				Required:       true,
				Maintainable:   false,
				DefaultRuleRef: `next_org_code("F", 8)`,
			},
			types.SetIDStrategyFieldDecision{
				FieldKey:          orgUnitCreateFieldOrgType,
				Maintainable:      true,
				Required:          false,
				AllowedValueCodes: []string{"11"},
			},
		))
		req := &WriteOrgUnitRequest{
			Patch: OrgUnitWritePatch{
				ParentOrgCode: stringPtr("ROOT"),
				Ext:           map[string]any{orgUnitCreateFieldOrgType: 11},
			},
		}
		if _, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", nil, req); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("parent org code invalid", func(t *testing.T) {
		svc := newWriteService(buildResolver(
			types.SetIDStrategyFieldDecision{},
			types.SetIDStrategyFieldDecision{},
		))
		req := &WriteOrgUnitRequest{
			Patch: OrgUnitWritePatch{
				ParentOrgCode: stringPtr("bad\x7f"),
			},
		}
		if _, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", nil, req); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestCreateFieldDecisionHelperFunctions(t *testing.T) {
	t.Run("resolveCreateFieldDecisionValue branches", func(t *testing.T) {
		if _, err := resolveCreateFieldDecisionValue(orgUnitCreateFieldOrgCode, "A001", true, types.SetIDStrategyFieldDecision{Maintainable: false}); err == nil || err.Error() != errFieldNotMaintainable {
			t.Fatalf("err=%v", err)
		}
		result, err := resolveCreateFieldDecisionValue(orgUnitCreateFieldOrgType, "11", true, types.SetIDStrategyFieldDecision{Maintainable: true})
		if err != nil || result.value != "11" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		result, err = resolveCreateFieldDecisionValue(orgUnitCreateFieldOrgCode, "", false, types.SetIDStrategyFieldDecision{Maintainable: false, DefaultRuleRef: `next_org_code("F", 8)`})
		if err != nil || result.autoCodeSpec == nil || result.autoCodeSpec.Prefix != "F" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		result, err = resolveCreateFieldDecisionValue(orgUnitCreateFieldOrgType, "", false, types.SetIDStrategyFieldDecision{Maintainable: false, DefaultRuleRef: `"11"`})
		if err != nil || result.value != "11" || result.autoCodeSpec != nil {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		if _, err := resolveCreateFieldDecisionValue(orgUnitCreateFieldOrgType, "", false, types.SetIDStrategyFieldDecision{Maintainable: false, DefaultRuleRef: "1+1"}); err == nil || err.Error() != errDefaultRuleEvalFailed {
			t.Fatalf("err=%v", err)
		}
		result, err = resolveCreateFieldDecisionValue(orgUnitCreateFieldOrgType, "", false, types.SetIDStrategyFieldDecision{Maintainable: false, DefaultValue: "12"})
		if err != nil || result.value != "12" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		result, err = resolveCreateFieldDecisionValue(orgUnitCreateFieldOrgType, "", false, types.SetIDStrategyFieldDecision{Maintainable: true})
		if err != nil || result.value != "" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		result, err = resolveCreateFieldDecisionValue(orgUnitCreateFieldOrgType, "", true, types.SetIDStrategyFieldDecision{Maintainable: true, DefaultValue: "10"})
		if err != nil || result.value != "" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		if _, err := resolveCreateFieldDecisionValue(orgUnitCreateFieldOrgType, "", false, types.SetIDStrategyFieldDecision{Maintainable: false}); err == nil || err.Error() != errDefaultRuleRequired {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("resolveCreateDefaultFromRule branches", func(t *testing.T) {
		if _, _, err := resolveCreateDefaultFromRule(orgUnitCreateFieldOrgCode, ""); err == nil || err.Error() != errDefaultRuleRequired {
			t.Fatalf("err=%v", err)
		}
		if _, _, err := resolveCreateDefaultFromRule(orgUnitCreateFieldOrgCode, `next_org_code("O", )`); err == nil || err.Error() != errFieldPolicyExprInvalid {
			t.Fatalf("err=%v", err)
		}
		value, spec, err := resolveCreateDefaultFromRule(orgUnitCreateFieldOrgCode, `"11"`)
		if err != nil || value != "11" || spec != nil {
			t.Fatalf("value=%q spec=%+v err=%v", value, spec, err)
		}
		if _, _, err := resolveCreateDefaultFromRule(orgUnitCreateFieldOrgType, "1+1"); err == nil || err.Error() != errDefaultRuleEvalFailed {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("validateFieldOptionAllowed branches", func(t *testing.T) {
		if err := validateFieldOptionAllowed("", []string{"11"}); err != nil {
			t.Fatalf("err=%v", err)
		}
		if err := validateFieldOptionAllowed("11", nil); err != nil {
			t.Fatalf("err=%v", err)
		}
		if err := validateFieldOptionAllowed("11", []string{" 11 "}); err != nil {
			t.Fatalf("err=%v", err)
		}
		if err := validateFieldOptionAllowed("99", []string{"11"}); err == nil || err.Error() != errFieldOptionNotAllowed {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("readCreateExtFieldString branches", func(t *testing.T) {
		if value, provided, err := readCreateExtFieldString(nil, orgUnitCreateFieldOrgType); err != nil || value != "" || provided {
			t.Fatalf("value=%q provided=%v err=%v", value, provided, err)
		}
		if value, provided, err := readCreateExtFieldString(map[string]any{}, orgUnitCreateFieldOrgType); err != nil || value != "" || provided {
			t.Fatalf("value=%q provided=%v err=%v", value, provided, err)
		}
		if value, provided, err := readCreateExtFieldString(map[string]any{orgUnitCreateFieldOrgType: nil}, orgUnitCreateFieldOrgType); err != nil || value != "" || !provided {
			t.Fatalf("value=%q provided=%v err=%v", value, provided, err)
		}
		if value, provided, err := readCreateExtFieldString(map[string]any{"other": "x"}, orgUnitCreateFieldOrgType); err != nil || value != "" || provided {
			t.Fatalf("value=%q provided=%v err=%v", value, provided, err)
		}
		if _, provided, err := readCreateExtFieldString(map[string]any{orgUnitCreateFieldOrgType: 11}, orgUnitCreateFieldOrgType); err == nil || !provided {
			t.Fatal("expected error")
		}
		if value, provided, err := readCreateExtFieldString(map[string]any{orgUnitCreateFieldOrgType: " 11 "}, orgUnitCreateFieldOrgType); err != nil || value != "11" || !provided {
			t.Fatalf("value=%q provided=%v err=%v", value, provided, err)
		}
	})

	t.Run("mapSetIDFieldDecisionError branches", func(t *testing.T) {
		if got := mapSetIDFieldDecisionError(errors.New(errFieldPolicyMissing)); got.Error() != errFieldPolicyMissing {
			t.Fatalf("got=%v", got)
		}
		if got := mapSetIDFieldDecisionError(errors.New(errFieldPolicyConflict)); got.Error() != errFieldPolicyConflict {
			t.Fatalf("got=%v", got)
		}
		if got := mapSetIDFieldDecisionError(errors.New("FIELD_DEFAULT_RULE_MISSING")); got.Error() != errDefaultRuleRequired {
			t.Fatalf("got=%v", got)
		}
		raw := errors.New("raw")
		if got := mapSetIDFieldDecisionError(raw); got != raw {
			t.Fatalf("got=%v want=%v", got, raw)
		}
	})
}

func TestResolveCreateBusinessUnitID(t *testing.T) {
	ctx := context.Background()

	t.Run("nil or empty parent org code", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		if got, err := svc.resolveCreateBusinessUnitID(ctx, "t1", nil); err != nil || got != "" {
			t.Fatalf("got=%q err=%v", got, err)
		}
		blank := "  "
		if got, err := svc.resolveCreateBusinessUnitID(ctx, "t1", &blank); err != nil || got != "" {
			t.Fatalf("got=%q err=%v", got, err)
		}
	})

	t.Run("normalize parent code invalid", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		bad := "bad\x7f"
		if _, err := svc.resolveCreateBusinessUnitID(ctx, "t1", &bad); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("resolve org id errors", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{
			resolveOrgIDFn: func(context.Context, string, string) (int, error) {
				return 0, orgunitpkg.ErrOrgCodeNotFound
			},
		})
		parent := "ROOT"
		if _, err := svc.resolveCreateBusinessUnitID(ctx, "t1", &parent); err == nil || err.Error() != errParentNotFoundAsOf {
			t.Fatalf("err=%v", err)
		}

		svc = newWriteService(orgUnitWriteStoreStub{
			resolveOrgIDFn: func(context.Context, string, string) (int, error) {
				return 0, errors.New("boom")
			},
		})
		if _, err := svc.resolveCreateBusinessUnitID(ctx, "t1", &parent); err == nil || err.Error() != "boom" {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("business unit id must be 8 digits", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{
			resolveOrgIDFn: func(context.Context, string, string) (int, error) {
				return 123, nil
			},
		})
		parent := "ROOT"
		if _, err := svc.resolveCreateBusinessUnitID(ctx, "t1", &parent); err == nil || err.Error() != errFieldPolicyConflict {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{
			resolveOrgIDFn: func(context.Context, string, string) (int, error) {
				return 10000001, nil
			},
		})
		parent := "ROOT"
		got, err := svc.resolveCreateBusinessUnitID(ctx, "t1", &parent)
		if err != nil || got != "10000001" {
			t.Fatalf("got=%q err=%v", got, err)
		}
	})
}

func TestEvaluateCELExprToString_Branches(t *testing.T) {
	t.Run("env error", func(t *testing.T) {
		orig := newOrgUnitWriteCELEnv
		newOrgUnitWriteCELEnv = func() (*cel.Env, error) { return nil, errors.New("env") }
		t.Cleanup(func() { newOrgUnitWriteCELEnv = orig })
		if _, err := evaluateCELExprToString(`"x"`); err == nil || err.Error() != "env" {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("compile error", func(t *testing.T) {
		if _, err := evaluateCELExprToString(`next_org_code("O", )`); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("output type invalid", func(t *testing.T) {
		if _, err := evaluateCELExprToString("1+1"); err == nil || err.Error() != "default expression must return string" {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("program eval error", func(t *testing.T) {
		if _, err := evaluateCELExprToString(`["a"][2]`); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("program build error", func(t *testing.T) {
		orig := newOrgUnitWriteCELProgram
		newOrgUnitWriteCELProgram = func(*cel.Env, *cel.Ast) (cel.Program, error) { return nil, errors.New("program") }
		t.Cleanup(func() { newOrgUnitWriteCELProgram = orig })
		if _, err := evaluateCELExprToString(`"11"`); err == nil || err.Error() != "program" {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("eval value not string", func(t *testing.T) {
		orig := newOrgUnitWriteCELEnv
		newOrgUnitWriteCELEnv = func() (*cel.Env, error) {
			return cel.NewEnv(
				cel.Function(
					orgUnitNextOrgCodeFuncName,
					cel.Overload(
						"next_org_code_string_int",
						[]*cel.Type{cel.StringType, cel.IntType},
						cel.StringType,
						cel.FunctionBinding(func(...ref.Val) ref.Val {
							return celtypes.Int(1)
						}),
					),
				),
			)
		}
		t.Cleanup(func() { newOrgUnitWriteCELEnv = orig })
		if _, err := evaluateCELExprToString(`next_org_code("O", 6)`); err == nil || err.Error() != "default expression must return string" {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		value, err := evaluateCELExprToString(`"11"`)
		if err != nil || value != "11" {
			t.Fatalf("value=%q err=%v", value, err)
		}
	})
}

func TestWriteCELHelpers(t *testing.T) {
	if got := orgUnitWriteCELNextOrgCode(); fmt.Sprint(got) != fmt.Sprint(celtypes.String("")) {
		t.Fatalf("got=%v", got)
	}

	celCompileCache = sync.Map{}
	orig := newOrgUnitWriteCELEnv
	newOrgUnitWriteCELEnv = func() (*cel.Env, error) { return nil, errors.New("env") }
	t.Cleanup(func() { newOrgUnitWriteCELEnv = orig })
	if err := compileCELExpr(`next_org_code("T", 11)`); err == nil || err.Error() != "env" {
		t.Fatalf("err=%v", err)
	}

	// restore for next checks in this test
	newOrgUnitWriteCELEnv = orig
	if err := compileCELExpr(`next_org_code("O", 6)`); err != nil {
		t.Fatalf("err=%v", err)
	}
}
