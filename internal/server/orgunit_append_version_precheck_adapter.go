package server

import (
	"context"
	"errors"
	"strings"

	orgunittypes "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
)

type orgUnitAppendFactsReader interface {
	ResolveAppendFacts(ctx context.Context, tenantID string, orgNodeKey string, effectiveDate string) (orgUnitAppendFacts, error)
}

type orgUnitAppendVersionPrecheckServerReader struct {
	store OrgUnitStore
}

func (r orgUnitAppendVersionPrecheckServerReader) ResolveOrgNodeKey(ctx context.Context, tenantID string, orgCode string) (string, error) {
	if r.store == nil {
		return "", errors.New("orgunit_store_missing")
	}
	return r.store.ResolveOrgNodeKeyByCode(ctx, tenantID, orgCode)
}

func (r orgUnitAppendVersionPrecheckServerReader) ResolveSetID(ctx context.Context, tenantID string, orgNodeKey string, asOf string) (string, error) {
	resolver, ok := any(r.store).(orgUnitSetIDResolver)
	if !ok {
		return "", errors.New(setIDContextCodeSetIDResolverMissing)
	}
	return resolver.ResolveSetID(ctx, tenantID, orgNodeKey, asOf)
}

func (r orgUnitAppendVersionPrecheckServerReader) IsOrgTreeInitialized(ctx context.Context, tenantID string) (bool, error) {
	reader, ok := any(r.store).(orgUnitTreeInitializationReader)
	if !ok {
		return false, nil
	}
	return reader.IsOrgTreeInitialized(ctx, tenantID)
}

func (r orgUnitAppendVersionPrecheckServerReader) ResolveSetIDStrategyFieldDecision(
	ctx context.Context,
	tenantID string,
	capabilityKey string,
	fieldKey string,
	orgNodeKey string,
	asOf string,
) (orgunittypes.SetIDStrategyFieldDecision, bool, error) {
	resolvedSetID := ""
	orgNodeKey = strings.TrimSpace(orgNodeKey)
	if orgNodeKey != "" {
		var err error
		resolvedSetID, err = r.ResolveSetID(ctx, tenantID, orgNodeKey, asOf)
		if err != nil {
			return orgunittypes.SetIDStrategyFieldDecision{}, false, err
		}
	}
	decision, err := defaultSetIDStrategyRegistryStore.resolveFieldDecision(
		ctx,
		tenantID,
		capabilityKey,
		fieldKey,
		resolvedSetID,
		orgNodeKey,
		asOf,
	)
	if err != nil {
		return orgunittypes.SetIDStrategyFieldDecision{}, false, err
	}
	return orgunittypes.SetIDStrategyFieldDecision{
		CapabilityKey:     strings.TrimSpace(decision.CapabilityKey),
		FieldKey:          strings.TrimSpace(decision.FieldKey),
		Required:          decision.Required,
		Visible:           decision.Visible,
		Maintainable:      decision.Maintainable,
		DefaultRuleRef:    strings.TrimSpace(decision.DefaultRuleRef),
		DefaultValue:      strings.TrimSpace(decision.ResolvedDefaultVal),
		AllowedValueCodes: append([]string(nil), decision.AllowedValueCodes...),
	}, true, nil
}

func (r orgUnitAppendVersionPrecheckServerReader) ListEnabledTenantFieldConfigsAsOf(
	ctx context.Context,
	tenantID string,
	asOf string,
) ([]orgunittypes.TenantFieldConfig, error) {
	reader, ok := any(r.store).(assistantTenantFieldConfigReader)
	if !ok {
		return nil, nil
	}
	configs, err := reader.ListEnabledTenantFieldConfigsAsOf(ctx, tenantID, asOf)
	if err != nil {
		return nil, err
	}
	out := make([]orgunittypes.TenantFieldConfig, 0, len(configs))
	for _, cfg := range configs {
		out = append(out, orgunittypes.TenantFieldConfig{
			FieldKey:         strings.TrimSpace(cfg.FieldKey),
			ValueType:        strings.TrimSpace(cfg.ValueType),
			DataSourceType:   strings.TrimSpace(cfg.DataSourceType),
			DataSourceConfig: append([]byte(nil), cfg.DataSourceConfig...),
		})
	}
	return out, nil
}

func (r orgUnitAppendVersionPrecheckServerReader) ResolveAppendFacts(
	ctx context.Context,
	tenantID string,
	orgNodeKey string,
	effectiveDate string,
) (orgunitservices.OrgUnitAppendVersionFactsV1, error) {
	reader, ok := any(r.store).(orgUnitAppendFactsReader)
	if !ok {
		return orgunitservices.OrgUnitAppendVersionFactsV1{}, nil
	}
	facts, err := reader.ResolveAppendFacts(ctx, tenantID, orgNodeKey, effectiveDate)
	if err != nil {
		return orgunitservices.OrgUnitAppendVersionFactsV1{}, err
	}
	return orgunitservices.OrgUnitAppendVersionFactsV1{
		TargetExistsAsOf: facts.TargetExistsAsOf,
	}, nil
}

func buildOrgUnitAppendVersionPrecheckResultV1(
	ctx context.Context,
	store OrgUnitStore,
	input orgunitservices.OrgUnitAppendVersionPrecheckInputV1,
) (orgunitservices.OrgUnitAppendVersionPrecheckResultV1, error) {
	input.TenantID = strings.TrimSpace(input.TenantID)
	input.CapabilityKey = strings.TrimSpace(input.CapabilityKey)
	if strings.TrimSpace(input.EffectivePolicyVersion) == "" && strings.TrimSpace(input.CapabilityKey) != "" {
		effectivePolicyVersion, _ := resolveOrgUnitEffectivePolicyVersion(input.TenantID, input.CapabilityKey)
		input.EffectivePolicyVersion = effectivePolicyVersion
	}
	return orgunitservices.BuildOrgUnitAppendVersionPrecheckProjectionV1(
		ctx,
		orgUnitAppendVersionPrecheckServerReader{store: store},
		input,
	)
}
