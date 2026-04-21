package services

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type orgUnitWriteResolverStoreStub struct {
	orgUnitWriteStoreStub
	findByRequestFn func(ctx context.Context, tenantID string, requestID string) (types.OrgUnitEvent, bool, error)
}

func (s orgUnitWriteResolverStoreStub) FindEventByRequestID(ctx context.Context, tenantID string, requestID string) (types.OrgUnitEvent, bool, error) {
	if s.findByRequestFn != nil {
		return s.findByRequestFn(ctx, tenantID, requestID)
	}
	return types.OrgUnitEvent{}, false, nil
}

type orgUnitWriteAutoCodeStoreStub struct {
	orgUnitWriteStoreStub
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
			findByRequestFn: func(context.Context, string, string) (types.OrgUnitEvent, bool, error) {
				return types.OrgUnitEvent{}, false, errors.New("boom")
			},
		})
		if _, found, err := svc.resolveCreateByRequestID(ctx, "t1", "r1"); err == nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("payload org code success", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteResolverStoreStub{
			findByRequestFn: func(context.Context, string, string) (types.OrgUnitEvent, bool, error) {
				return types.OrgUnitEvent{
					EventType:     types.OrgUnitEventCreate,
					OrgNodeKey:    "10000001",
					EffectiveDate: "2026-01-01",
					EventUUID:     "e1",
					Payload:       json.RawMessage(`{"org_code":" ROOT "}`),
				}, true, nil
			},
		})
		result, found, err := svc.resolveCreateByRequestID(ctx, "t1", "r1")
		if err != nil || !found || result.OrgCode != "ROOT" {
			t.Fatalf("result=%+v found=%v err=%v", result, found, err)
		}
	})
}

func TestApplyCreatePolicyDefaults_StaticRules(t *testing.T) {
	ctx := context.Background()

	t.Run("parent org invalid bubbles", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		req := &WriteOrgUnitRequest{Patch: OrgUnitWritePatch{ParentOrgCode: stringPtr("bad\x7f")}}
		if _, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", nil, req); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("auto code default and org type omitted", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{
			isOrgTreeInitializedFn: func(context.Context, string) (bool, error) { return false, nil },
		})
		req := &WriteOrgUnitRequest{
			Patch: OrgUnitWritePatch{},
		}
		spec, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", nil, req)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if spec == nil || spec.Prefix != "O" || spec.Width != 6 {
			t.Fatalf("spec=%+v", spec)
		}
		if req.OrgCode != "" || req.Patch.Ext != nil {
			t.Fatalf("req=%+v", req)
		}
	})

	t.Run("manual org code preserved and enabled org type default applied", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{
			resolveOrgNodeKeyFn: func(_ context.Context, _ string, orgCode string) (string, error) {
				if orgCode == "ROOT" {
					return mustEncodeTestOrgNodeKey(10000001), nil
				}
				return "", orgunitpkg.ErrOrgCodeNotFound
			},
			isOrgTreeInitializedFn: func(context.Context, string) (bool, error) { return true, nil },
		})
		req := &WriteOrgUnitRequest{
			OrgCode: "A001",
			Patch: OrgUnitWritePatch{
				ParentOrgCode: stringPtr("ROOT"),
			},
		}
		spec, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", []types.TenantFieldConfig{{
			FieldKey:         orgUnitCreateFieldOrgType,
			ValueType:        "text",
			DataSourceType:   "DICT",
			DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`),
		}}, req)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if spec != nil {
			t.Fatalf("spec=%+v", spec)
		}
		if req.OrgCode != "A001" {
			t.Fatalf("req=%+v", req)
		}
		if req.Patch.Ext != nil {
			if _, ok := req.Patch.Ext[orgUnitCreateFieldOrgType]; ok {
				t.Fatalf("req=%+v", req)
			}
		}
	})

	t.Run("org type allowlist empty accepts arbitrary value", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{
			resolveOrgNodeKeyFn:    func(context.Context, string, string) (string, error) { return mustEncodeTestOrgNodeKey(10000001), nil },
			isOrgTreeInitializedFn: func(context.Context, string) (bool, error) { return true, nil },
		})
		req := &WriteOrgUnitRequest{
			Patch: OrgUnitWritePatch{
				ParentOrgCode: stringPtr("ROOT"),
				Ext:           map[string]any{orgUnitCreateFieldOrgType: "11"},
			},
		}
		if _, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", []types.TenantFieldConfig{{FieldKey: orgUnitCreateFieldOrgType}}, req); err != nil {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestWrite_CreateOrg_AutoCodeBranches(t *testing.T) {
	ctx := context.Background()

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
		if err != nil || result.OrgCode != "ROOT" || result.EventUUID != "e1" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	})

	t.Run("auto submit success", func(t *testing.T) {
		store := orgUnitWriteAutoCodeStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{
				resolveOrgIDFn:         func(context.Context, string, string) (int, error) { return 10000001, nil },
				isOrgTreeInitializedFn: func(context.Context, string) (bool, error) { return false, nil },
				listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
					return []types.TenantFieldConfig{}, nil
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
		if err != nil || result.OrgCode != "O000001" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	})
}

func TestCreateFieldDecisionHelperFunctions(t *testing.T) {
	t.Run("resolveCreateFieldDecisionValue branches", func(t *testing.T) {
		if _, err := resolveCreateFieldDecisionValue(orgUnitCreateFieldOrgCode, "A001", true, orgUnitFieldDecision{Maintainable: false}); err == nil || err.Error() != errFieldNotMaintainable {
			t.Fatalf("err=%v", err)
		}
		result, err := resolveCreateFieldDecisionValue(orgUnitCreateFieldOrgType, "11", true, orgUnitFieldDecision{Maintainable: true})
		if err != nil || result.value != "11" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		result, err = resolveCreateFieldDecisionValue(orgUnitCreateFieldOrgCode, "", false, orgUnitFieldDecision{Maintainable: false, DefaultRuleRef: `next_org_code("F", 8)`})
		if err != nil || result.autoCodeSpec == nil || result.autoCodeSpec.Prefix != "F" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		result, err = resolveCreateFieldDecisionValue(orgUnitCreateFieldOrgType, "", false, orgUnitFieldDecision{Maintainable: false, DefaultRuleRef: `"11"`})
		if err != nil || result.value != "11" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		if _, err := resolveCreateFieldDecisionValue(orgUnitCreateFieldOrgType, "", false, orgUnitFieldDecision{Maintainable: false, DefaultRuleRef: "1+1"}); err == nil || err.Error() != errDefaultRuleEvalFailed {
			t.Fatalf("err=%v", err)
		}
		if _, err := resolveCreateFieldDecisionValue(orgUnitCreateFieldOrgType, "", false, orgUnitFieldDecision{Maintainable: false}); err == nil || err.Error() != errDefaultRuleRequired {
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
	})

	t.Run("validateFieldOptionAllowed branches", func(t *testing.T) {
		if err := validateFieldOptionAllowed("99", []string{"11"}); err == nil || err.Error() != errFieldOptionNotAllowed {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("readCreateExtFieldString branches", func(t *testing.T) {
		if _, provided, err := readCreateExtFieldString(map[string]any{orgUnitCreateFieldOrgType: 11}, orgUnitCreateFieldOrgType); err == nil || !provided {
			t.Fatal("expected error")
		}
	})

}

func TestResolveCreateBusinessUnitID(t *testing.T) {
	ctx := context.Background()
	svc := newWriteService(orgUnitWriteStoreStub{
		resolveOrgIDFn: func(context.Context, string, string) (int, error) {
			return 10000001, nil
		},
	})
	parent := "ROOT"
	got, err := svc.resolveCreateBusinessUnitID(ctx, "t1", &parent)
	if err != nil || got != mustEncodeTestOrgNodeKey(10000001) {
		t.Fatalf("got=%q err=%v", got, err)
	}
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
}

func TestParseCompileAndMapCreateAutoCodeHelpers(t *testing.T) {
	celCompileCache = sync.Map{}

	t.Run("parse success", func(t *testing.T) {
		spec, err := parseNextOrgCodeRule(`next_org_code("ORG", 3)`)
		if err != nil || spec.Prefix != "ORG" || spec.Width != 3 {
			t.Fatalf("spec=%+v err=%v", spec, err)
		}
	})
}
