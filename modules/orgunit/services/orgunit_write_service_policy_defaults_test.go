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
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
)

type orgUnitWriteResolverStoreStub struct {
	orgUnitWriteStoreStub
	findByRequestFn func(ctx context.Context, tenantID string, requestCode string) (types.OrgUnitEvent, bool, error)
	resolvePolicyFn func(ctx context.Context, tenantID string, fieldKey string, scopeType string, scopeKey string, asOf string) (types.TenantFieldPolicy, bool, error)
}

func (s orgUnitWriteResolverStoreStub) FindEventByRequestCode(ctx context.Context, tenantID string, requestCode string) (types.OrgUnitEvent, bool, error) {
	if s.findByRequestFn != nil {
		return s.findByRequestFn(ctx, tenantID, requestCode)
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
	orgUnitWriteResolverStoreStub
	submitCreateAutoFn func(
		ctx context.Context,
		tenantID string,
		eventUUID string,
		effectiveDate string,
		payload json.RawMessage,
		requestCode string,
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
	requestCode string,
	initiatorUUID string,
	prefix string,
	width int,
) (int64, string, error) {
	if s.submitCreateAutoFn != nil {
		return s.submitCreateAutoFn(ctx, tenantID, eventUUID, effectiveDate, payload, requestCode, initiatorUUID, prefix, width)
	}
	return 0, "", errors.New("SubmitCreateEventWithGeneratedCode not mocked")
}

func TestResolveCreateByRequestCode_Branches(t *testing.T) {
	ctx := context.Background()

	t.Run("store without request reader", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, found, err := svc.resolveCreateByRequestCode(ctx, "t1", "r1")
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
		if _, found, err := svc.resolveCreateByRequestCode(ctx, "t1", "r1"); err == nil || found {
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
		_, found, err := svc.resolveCreateByRequestCode(ctx, "t1", "r1")
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
		if _, found, err := svc.resolveCreateByRequestCode(ctx, "t1", "r1"); err == nil || found {
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
		if _, found, err := svc.resolveCreateByRequestCode(ctx, "t1", "r1"); err == nil || found {
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
		if _, found, err := svc.resolveCreateByRequestCode(ctx, "t1", "r1"); err == nil || found {
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
		result, found, err := svc.resolveCreateByRequestCode(ctx, "t1", "r1")
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
		result, found, err := svc.resolveCreateByRequestCode(ctx, "t1", "r1")
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
	fieldConfigs := []types.TenantFieldConfig{}

	t.Run("store without resolver", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		req := &WriteOrgUnitRequest{}
		spec, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", fieldConfigs, req)
		if err != nil || spec != nil {
			t.Fatalf("spec=%+v err=%v", spec, err)
		}
	})

	t.Run("resolver error", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{},
			resolvePolicyFn: func(context.Context, string, string, string, string, string) (types.TenantFieldPolicy, bool, error) {
				return types.TenantFieldPolicy{}, false, errors.New("boom")
			},
		})
		req := &WriteOrgUnitRequest{}
		if _, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", fieldConfigs, req); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("not maintainable with provided org_code", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{},
			resolvePolicyFn: func(context.Context, string, string, string, string, string) (types.TenantFieldPolicy, bool, error) {
				return types.TenantFieldPolicy{Maintainable: false, DefaultMode: orgUnitDefaultModeNone}, true, nil
			},
		})
		req := &WriteOrgUnitRequest{OrgCode: "ROOT"}
		if _, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", fieldConfigs, req); err == nil || err.Error() != errFieldNotMaintainable {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("none mode not maintainable requires default rule", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{},
			resolvePolicyFn: func(context.Context, string, string, string, string, string) (types.TenantFieldPolicy, bool, error) {
				return types.TenantFieldPolicy{Maintainable: false, DefaultMode: orgUnitDefaultModeNone}, true, nil
			},
		})
		req := &WriteOrgUnitRequest{}
		if _, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", fieldConfigs, req); err == nil || err.Error() != errDefaultRuleRequired {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("none mode maintainable true", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{},
			resolvePolicyFn: func(context.Context, string, string, string, string, string) (types.TenantFieldPolicy, bool, error) {
				return types.TenantFieldPolicy{Maintainable: true, DefaultMode: orgUnitDefaultModeNone}, true, nil
			},
		})
		req := &WriteOrgUnitRequest{}
		spec, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", fieldConfigs, req)
		if err != nil || spec != nil {
			t.Fatalf("spec=%+v err=%v", spec, err)
		}
	})

	t.Run("cel mode with provided org_code is skipped", func(t *testing.T) {
		rule := `next_org_code("O", 6)`
		svc := newWriteService(orgUnitWriteResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{},
			resolvePolicyFn: func(context.Context, string, string, string, string, string) (types.TenantFieldPolicy, bool, error) {
				return types.TenantFieldPolicy{Maintainable: true, DefaultMode: orgUnitDefaultModeCEL, DefaultRuleExpr: &rule}, true, nil
			},
		})
		req := &WriteOrgUnitRequest{OrgCode: "ROOT"}
		spec, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", fieldConfigs, req)
		if err != nil || spec != nil {
			t.Fatalf("spec=%+v err=%v", spec, err)
		}
	})

	t.Run("cel mode missing expression", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{},
			resolvePolicyFn: func(context.Context, string, string, string, string, string) (types.TenantFieldPolicy, bool, error) {
				return types.TenantFieldPolicy{Maintainable: true, DefaultMode: orgUnitDefaultModeCEL}, true, nil
			},
		})
		req := &WriteOrgUnitRequest{}
		if _, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", fieldConfigs, req); err == nil || err.Error() != errDefaultRuleRequired {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("cel mode invalid expression", func(t *testing.T) {
		rule := `next_org_code("O", )`
		svc := newWriteService(orgUnitWriteResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{},
			resolvePolicyFn: func(context.Context, string, string, string, string, string) (types.TenantFieldPolicy, bool, error) {
				return types.TenantFieldPolicy{Maintainable: true, DefaultMode: orgUnitDefaultModeCEL, DefaultRuleExpr: &rule}, true, nil
			},
		})
		req := &WriteOrgUnitRequest{}
		if _, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", fieldConfigs, req); err == nil || err.Error() != errFieldPolicyExprInvalid {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("cel mode valid expression", func(t *testing.T) {
		rule := `next_org_code("O", 6)`
		svc := newWriteService(orgUnitWriteResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{},
			resolvePolicyFn: func(context.Context, string, string, string, string, string) (types.TenantFieldPolicy, bool, error) {
				return types.TenantFieldPolicy{Maintainable: false, DefaultMode: orgUnitDefaultModeCEL, DefaultRuleExpr: &rule}, true, nil
			},
		})
		req := &WriteOrgUnitRequest{}
		spec, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", fieldConfigs, req)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if spec == nil || spec.Prefix != "O" || spec.Width != 6 {
			t.Fatalf("spec=%+v", spec)
		}
	})

	t.Run("unknown mode and not maintainable", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{},
			resolvePolicyFn: func(context.Context, string, string, string, string, string) (types.TenantFieldPolicy, bool, error) {
				return types.TenantFieldPolicy{Maintainable: false, DefaultMode: "BAD"}, true, nil
			},
		})
		req := &WriteOrgUnitRequest{}
		if _, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", fieldConfigs, req); err == nil || err.Error() != errDefaultRuleRequired {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("unknown mode and maintainable", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{},
			resolvePolicyFn: func(context.Context, string, string, string, string, string) (types.TenantFieldPolicy, bool, error) {
				return types.TenantFieldPolicy{Maintainable: true, DefaultMode: "BAD"}, true, nil
			},
		})
		req := &WriteOrgUnitRequest{}
		if _, err := svc.applyCreatePolicyDefaults(ctx, "t1", "2026-01-01", fieldConfigs, req); err == nil || err.Error() != errDefaultRuleEvalFailed {
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
		if _, err := parseNextOrgCodeRule(`next_org_code("", 6)`); err == nil || err.Error() != errDefaultRuleEvalFailed {
			t.Fatalf("err=%v", err)
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
			RequestCode:   "r1",
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
			RequestCode:   "r1",
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
			RequestCode:   "r1",
			Patch:         OrgUnitWritePatch{Name: &name},
		})
		if err == nil || err.Error() != errDefaultRuleRequired {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("create apply policy defaults returns error", func(t *testing.T) {
		store := orgUnitWriteResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{
				listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
					return []types.TenantFieldConfig{}, nil
				},
			},
			resolvePolicyFn: func(context.Context, string, string, string, string, string) (types.TenantFieldPolicy, bool, error) {
				return types.TenantFieldPolicy{}, false, errors.New("policy")
			},
		}
		svc := NewOrgUnitWriteService(store)
		name := "Root"
		_, err := svc.Write(ctx, "t1", WriteOrgUnitRequest{
			Intent:        "create_org",
			EffectiveDate: "2026-01-01",
			RequestCode:   "r1",
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
			RequestCode:   "r1",
			Patch:         OrgUnitWritePatch{Name: &name},
		})
		if err == nil || err.Error() != errOrgCodeInvalid {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("auto code resolver exists but submitter missing", func(t *testing.T) {
		rule := `next_org_code("O", 6)`
		store := orgUnitWriteResolverStoreStub{
			orgUnitWriteStoreStub: orgUnitWriteStoreStub{
				listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
					return []types.TenantFieldConfig{}, nil
				},
			},
			resolvePolicyFn: func(context.Context, string, string, string, string, string) (types.TenantFieldPolicy, bool, error) {
				return types.TenantFieldPolicy{FieldKey: "org_code", ScopeType: "FORM", ScopeKey: "orgunit.create_dialog", Maintainable: false, DefaultMode: "CEL", DefaultRuleExpr: &rule}, true, nil
			},
		}
		svc := NewOrgUnitWriteService(store)
		name := "Root"
		_, err := svc.Write(ctx, "t1", WriteOrgUnitRequest{
			Intent:        "create_org",
			EffectiveDate: "2026-01-01",
			RequestCode:   "r1",
			Patch:         OrgUnitWritePatch{Name: &name},
		})
		if err == nil || err.Error() != errDefaultRuleEvalFailed {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("auto submit error is mapped", func(t *testing.T) {
		rule := `next_org_code("O", 6)`
		store := orgUnitWriteAutoCodeStoreStub{
			orgUnitWriteResolverStoreStub: orgUnitWriteResolverStoreStub{
				orgUnitWriteStoreStub: orgUnitWriteStoreStub{
					listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
						return []types.TenantFieldConfig{}, nil
					},
				},
				resolvePolicyFn: func(context.Context, string, string, string, string, string) (types.TenantFieldPolicy, bool, error) {
					return types.TenantFieldPolicy{FieldKey: "org_code", ScopeType: "FORM", ScopeKey: "orgunit.create_dialog", Maintainable: false, DefaultMode: "CEL", DefaultRuleExpr: &rule}, true, nil
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
			RequestCode:   "r1",
			Patch:         OrgUnitWritePatch{Name: &name},
		})
		if err == nil || err.Error() != errOrgCodeConflict {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("auto submit success", func(t *testing.T) {
		rule := `next_org_code("O", 6)`
		store := orgUnitWriteAutoCodeStoreStub{
			orgUnitWriteResolverStoreStub: orgUnitWriteResolverStoreStub{
				orgUnitWriteStoreStub: orgUnitWriteStoreStub{
					listEnabledFieldCfgsFn: func(context.Context, string, string) ([]types.TenantFieldConfig, error) {
						return []types.TenantFieldConfig{}, nil
					},
				},
				resolvePolicyFn: func(context.Context, string, string, string, string, string) (types.TenantFieldPolicy, bool, error) {
					return types.TenantFieldPolicy{FieldKey: "org_code", ScopeType: "FORM", ScopeKey: "orgunit.create_dialog", Maintainable: false, DefaultMode: "CEL", DefaultRuleExpr: &rule}, true, nil
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
			RequestCode:   "r1",
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
