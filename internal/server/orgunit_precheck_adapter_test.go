package server

import (
	"context"
	"errors"
	"testing"

	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
)

type orgUnitStoreWithAdapterMethods struct {
	orgUnitStoreStub
	resolveSetIDFn func(context.Context, string, string, string) (string, error)
	treeInitFn     func(context.Context, string) (bool, error)
	fieldConfigsFn func(context.Context, string, string) ([]orgUnitTenantFieldConfig, error)
	resolveFactsFn func(context.Context, string, string, string) (orgUnitAppendFacts, error)
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
