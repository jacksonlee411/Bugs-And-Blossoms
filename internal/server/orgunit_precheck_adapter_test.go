package server

import (
	"context"
	"errors"
	"testing"

	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
)

type orgUnitStoreWithAdapterMethods struct {
	orgUnitStoreStub
	resolveSetIDFn       func(context.Context, string, string, string) (string, error)
	treeInitFn           func(context.Context, string) (bool, error)
	fieldConfigsFn       func(context.Context, string, string) ([]orgUnitTenantFieldConfig, error)
	resolveFactsFn       func(context.Context, string, string, string) (orgUnitAppendFacts, error)
	resolveTargetEventFn func(context.Context, string, string, string) (orgUnitMutationTargetEvent, error)
}

func (s orgUnitStoreWithAdapterMethods) ResolveSetID(ctx context.Context, tenantID string, orgNodeKey string, asOf string) (string, error) {
	if s.resolveSetIDFn != nil {
		return s.resolveSetIDFn(ctx, tenantID, orgNodeKey, asOf)
	}
	return "", nil
}

func (s orgUnitStoreWithAdapterMethods) IsOrgTreeInitialized(ctx context.Context, tenantID string) (bool, error) {
	if s.treeInitFn != nil {
		return s.treeInitFn(ctx, tenantID)
	}
	return false, nil
}

func (s orgUnitStoreWithAdapterMethods) ListEnabledTenantFieldConfigsAsOf(ctx context.Context, tenantID string, asOf string) ([]orgUnitTenantFieldConfig, error) {
	if s.fieldConfigsFn != nil {
		return s.fieldConfigsFn(ctx, tenantID, asOf)
	}
	return nil, nil
}

func (s orgUnitStoreWithAdapterMethods) ResolveAppendFacts(ctx context.Context, tenantID string, orgNodeKey string, effectiveDate string) (orgUnitAppendFacts, error) {
	if s.resolveFactsFn != nil {
		return s.resolveFactsFn(ctx, tenantID, orgNodeKey, effectiveDate)
	}
	return orgUnitAppendFacts{}, nil
}

func (s orgUnitStoreWithAdapterMethods) ResolveMutationTargetEvent(ctx context.Context, tenantID string, orgNodeKey string, effectiveDate string) (orgUnitMutationTargetEvent, error) {
	if s.resolveTargetEventFn != nil {
		return s.resolveTargetEventFn(ctx, tenantID, orgNodeKey, effectiveDate)
	}
	return orgUnitMutationTargetEvent{}, nil
}

func TestCreateOrgUnitPrecheckServerReader_Branches(t *testing.T) {
	ctx := context.Background()

	if _, err := (createOrgUnitPrecheckServerReader{}).ResolveOrgNodeKey(ctx, "tenant-1", "FLOWER-A"); err == nil || err.Error() != "orgunit_store_missing" {
		t.Fatalf("expected missing store err, got=%v", err)
	}
	if _, err := (createOrgUnitPrecheckServerReader{store: orgUnitStoreStub{}}).ResolveSetID(ctx, "tenant-1", "10000001", "2026-01-01"); err == nil || err.Error() != setIDContextCodeSetIDResolverMissing {
		t.Fatalf("expected missing setid resolver err, got=%v", err)
	}

	reader := createOrgUnitPrecheckServerReader{store: orgUnitStoreWithAdapterMethods{
		resolveSetIDFn: func(context.Context, string, string, string) (string, error) {
			return "", errors.New("resolve setid failed")
		},
	}}
	if _, _, err := reader.ResolveSetIDStrategyFieldDecision(ctx, "tenant-1", "cap", "field", "10000001", "2026-01-01"); err == nil || err.Error() != "resolve setid failed" {
		t.Fatalf("expected resolve setid failure, got=%v", err)
	}
}

func TestOrgUnitAppendVersionPrecheckServerReader_Branches(t *testing.T) {
	ctx := context.Background()

	if _, err := (orgUnitAppendVersionPrecheckServerReader{}).ResolveOrgNodeKey(ctx, "tenant-1", "FLOWER-A"); err == nil || err.Error() != "orgunit_store_missing" {
		t.Fatalf("expected missing store err, got=%v", err)
	}
	if _, err := (orgUnitAppendVersionPrecheckServerReader{store: orgUnitStoreStub{}}).ResolveSetID(ctx, "tenant-1", "10000001", "2026-01-01"); err == nil || err.Error() != setIDContextCodeSetIDResolverMissing {
		t.Fatalf("expected missing setid resolver err, got=%v", err)
	}
	if ok, err := (orgUnitAppendVersionPrecheckServerReader{store: orgUnitStoreStub{}}).IsOrgTreeInitialized(ctx, "tenant-1"); err != nil || ok {
		t.Fatalf("expected no tree reader fallback, ok=%v err=%v", ok, err)
	}

	resolveErrReader := orgUnitAppendVersionPrecheckServerReader{store: orgUnitStoreWithAdapterMethods{
		resolveSetIDFn: func(context.Context, string, string, string) (string, error) {
			return "", errors.New("resolve setid failed")
		},
	}}
	if _, _, err := resolveErrReader.ResolveSetIDStrategyFieldDecision(ctx, "tenant-1", "cap", "field", "10000001", "2026-01-01"); err == nil || err.Error() != "resolve setid failed" {
		t.Fatalf("expected resolve setid failure, got=%v", err)
	}

	previous := defaultSetIDStrategyRegistryStore
	defer func() { defaultSetIDStrategyRegistryStore = previous }()
	defaultSetIDStrategyRegistryStore = setIDStrategyRegistryStoreStub{
		resolveFieldDecisionFn: func(context.Context, string, string, string, string, string, string) (setIDFieldDecision, error) {
			return setIDFieldDecision{}, errors.New("policy boom")
		},
	}
	registryErrReader := orgUnitAppendVersionPrecheckServerReader{store: orgUnitStoreWithAdapterMethods{
		resolveSetIDFn: func(context.Context, string, string, string) (string, error) {
			return "S2601", nil
		},
	}}
	if _, _, err := registryErrReader.ResolveSetIDStrategyFieldDecision(ctx, "tenant-1", "cap", "field", "10000001", "2026-01-01"); err == nil || err.Error() != "policy boom" {
		t.Fatalf("expected registry failure, got=%v", err)
	}

	if cfgs, err := (orgUnitAppendVersionPrecheckServerReader{store: orgUnitStoreStub{}}).ListEnabledTenantFieldConfigsAsOf(ctx, "tenant-1", "2026-01-01"); err != nil || cfgs != nil {
		t.Fatalf("expected nil field configs fallback, cfgs=%v err=%v", cfgs, err)
	}
	if facts, err := (orgUnitAppendVersionPrecheckServerReader{store: orgUnitStoreStub{}}).ResolveAppendFacts(ctx, "tenant-1", "10000001", "2026-01-01"); err != nil || facts != (orgunitservices.OrgUnitAppendVersionFactsV1{}) {
		t.Fatalf("expected zero append facts fallback, facts=%+v err=%v", facts, err)
	}

	appendFactsReader := orgUnitAppendVersionPrecheckServerReader{store: orgUnitStoreWithAdapterMethods{
		resolveFactsFn: func(context.Context, string, string, string) (orgUnitAppendFacts, error) {
			return orgUnitAppendFacts{}, errors.New("append facts failed")
		},
	}}
	if _, err := appendFactsReader.ResolveAppendFacts(ctx, "tenant-1", "10000001", "2026-01-01"); err == nil || err.Error() != "append facts failed" {
		t.Fatalf("expected append facts failure, got=%v", err)
	}
}

func TestOrgUnitMaintainPrecheckServerReader_Branches(t *testing.T) {
	ctx := context.Background()

	if ok, err := (orgUnitMaintainPrecheckServerReader{}).ResolveTargetExistsAsOf(ctx, "tenant-1", "10000001", "2026-01-01"); err != nil || ok {
		t.Fatalf("expected zero target exists fallback, ok=%v err=%v", ok, err)
	}
	if target, err := (orgUnitMaintainPrecheckServerReader{orgUnitAppendVersionPrecheckServerReader: orgUnitAppendVersionPrecheckServerReader{store: orgUnitStoreStub{}}}).ResolveMutationTargetEvent(ctx, "tenant-1", "10000001", "2026-01-01"); err != nil || target != (orgunitservices.OrgUnitMaintainTargetEventV1{}) {
		t.Fatalf("expected zero target fallback, target=%+v err=%v", target, err)
	}

	targetErrReader := orgUnitMaintainPrecheckServerReader{orgUnitAppendVersionPrecheckServerReader: orgUnitAppendVersionPrecheckServerReader{store: orgUnitStoreWithAdapterMethods{
		resolveTargetEventFn: func(context.Context, string, string, string) (orgUnitMutationTargetEvent, error) {
			return orgUnitMutationTargetEvent{}, errors.New("target event failed")
		},
	}}}
	if _, err := targetErrReader.ResolveMutationTargetEvent(ctx, "tenant-1", "10000001", "2026-01-01"); err == nil || err.Error() != "target event failed" {
		t.Fatalf("expected target event failure, got=%v", err)
	}

	targetReader := orgUnitMaintainPrecheckServerReader{orgUnitAppendVersionPrecheckServerReader: orgUnitAppendVersionPrecheckServerReader{store: orgUnitStoreWithAdapterMethods{
		resolveFactsFn: func(context.Context, string, string, string) (orgUnitAppendFacts, error) {
			return orgUnitAppendFacts{TargetExistsAsOf: true}, nil
		},
		resolveTargetEventFn: func(context.Context, string, string, string) (orgUnitMutationTargetEvent, error) {
			return orgUnitMutationTargetEvent{HasEffective: true, EffectiveEventType: "CREATE", HasRaw: true, RawEventType: "CREATE"}, nil
		},
	}}}
	if ok, err := targetReader.ResolveTargetExistsAsOf(ctx, "tenant-1", "10000001", "2026-01-01"); err != nil || !ok {
		t.Fatalf("expected target exists, ok=%v err=%v", ok, err)
	}
	target, err := targetReader.ResolveMutationTargetEvent(ctx, "tenant-1", "10000001", "2026-01-01")
	if err != nil {
		t.Fatalf("target err=%v", err)
	}
	if !target.HasEffective || string(target.EffectiveEventType) != "CREATE" || !target.HasRaw || string(target.RawEventType) != "CREATE" {
		t.Fatalf("target=%+v", target)
	}

	targetExistsErrReader := orgUnitMaintainPrecheckServerReader{orgUnitAppendVersionPrecheckServerReader: orgUnitAppendVersionPrecheckServerReader{store: orgUnitStoreWithAdapterMethods{
		resolveFactsFn: func(context.Context, string, string, string) (orgUnitAppendFacts, error) {
			return orgUnitAppendFacts{}, errors.New("append facts failed")
		},
	}}}
	if _, err := targetExistsErrReader.ResolveTargetExistsAsOf(ctx, "tenant-1", "10000001", "2026-01-01"); err == nil || err.Error() != "append facts failed" {
		t.Fatalf("expected target exists failure, got=%v", err)
	}
}

func TestBuildOrgUnitMaintainPrecheckResultV1_DefaultCapabilityAndPolicyVersion(t *testing.T) {
	ctx := context.Background()
	store := newOrgUnitMemoryStore()
	parent, err := store.CreateNodeCurrent(ctx, "tenant-1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true)
	if err != nil {
		t.Fatalf("seed parent err=%v", err)
	}
	if _, err := store.CreateNodeCurrent(ctx, "tenant-1", "2026-01-01", "FLOWER-C", "运营中心", parent.ID, false); err != nil {
		t.Fatalf("seed child err=%v", err)
	}

	correctResult, err := buildOrgUnitMaintainPrecheckResultV1(ctx, store, orgunitservices.OrgUnitMaintainPrecheckInputV1{
		Intent:              orgunitservices.OrgUnitMaintainIntentCorrect,
		TenantID:            "tenant-1",
		OrgCode:             "FLOWER-C",
		TargetEffectiveDate: "2026-01-01",
		CanAdmin:            true,
		NewName:             "运营平台部",
	})
	if err != nil {
		t.Fatalf("correct result err=%v", err)
	}
	if correctResult.PolicyContext.CapabilityKey != orgUnitCorrectFieldPolicyCapabilityKey {
		t.Fatalf("correct capability=%q", correctResult.PolicyContext.CapabilityKey)
	}
	if correctResult.Projection.EffectivePolicyVersion == "" {
		t.Fatalf("missing effective policy version: %+v", correctResult.Projection)
	}

	moveResult, err := buildOrgUnitMaintainPrecheckResultV1(ctx, store, orgunitservices.OrgUnitMaintainPrecheckInputV1{
		Intent:                        orgunitservices.OrgUnitMaintainIntentMove,
		TenantID:                      "tenant-1",
		OrgCode:                       "FLOWER-C",
		EffectiveDate:                 "2026-04-01",
		CanAdmin:                      true,
		CandidateConfirmationRequired: true,
		NewParentRequested:            true,
	})
	if err != nil {
		t.Fatalf("move result err=%v", err)
	}
	if moveResult.PolicyContext.CapabilityKey != orgUnitWriteFieldPolicyCapabilityKey {
		t.Fatalf("move capability=%q", moveResult.PolicyContext.CapabilityKey)
	}
	if moveResult.Projection.EffectivePolicyVersion == "" {
		t.Fatalf("move effective policy version missing: %+v", moveResult.Projection)
	}
	if moveResult.Projection.Readiness == "" {
		t.Fatalf("move readiness=%q", moveResult.Projection.Readiness)
	}

	explicitResult, err := buildOrgUnitMaintainPrecheckResultV1(ctx, store, orgunitservices.OrgUnitMaintainPrecheckInputV1{
		Intent:                 " rename ",
		TenantID:               " tenant-1 ",
		CapabilityKey:          " " + orgUnitWriteFieldPolicyCapabilityKey + " ",
		EffectivePolicyVersion: " explicit-epv ",
		OrgCode:                " FLOWER-C ",
		TargetEffectiveDate:    " 2026-01-01 ",
		CanAdmin:               true,
		NewName:                " 运营支持中心 ",
	})
	if err != nil {
		t.Fatalf("explicit result err=%v", err)
	}
	if explicitResult.PolicyContext.CapabilityKey != orgUnitWriteFieldPolicyCapabilityKey {
		t.Fatalf("explicit capability=%q", explicitResult.PolicyContext.CapabilityKey)
	}
	if explicitResult.Projection.EffectivePolicyVersion != "explicit-epv" {
		t.Fatalf("explicit effective policy version=%q", explicitResult.Projection.EffectivePolicyVersion)
	}
}
