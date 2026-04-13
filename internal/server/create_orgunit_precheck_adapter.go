package server

import (
	"context"
	"errors"
	"strings"

	orgunittypes "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
)

type assistantTenantFieldConfigReader interface {
	ListEnabledTenantFieldConfigsAsOf(ctx context.Context, tenantID string, asOf string) ([]orgUnitTenantFieldConfig, error)
}

type orgUnitTreeInitializationReader interface {
	IsOrgTreeInitialized(ctx context.Context, tenantID string) (bool, error)
}

type createOrgUnitPrecheckServerReader struct {
	store OrgUnitStore
}

func (r createOrgUnitPrecheckServerReader) ResolveOrgNodeKey(ctx context.Context, tenantID string, orgCode string) (string, error) {
	if r.store == nil {
		return "", errors.New("orgunit_store_missing")
	}
	return r.store.ResolveOrgNodeKeyByCode(ctx, tenantID, orgCode)
}

func (r createOrgUnitPrecheckServerReader) ResolveSetID(ctx context.Context, tenantID string, orgNodeKey string, asOf string) (string, error) {
	resolver, ok := any(r.store).(orgUnitSetIDResolver)
	if !ok {
		return "", errors.New(setIDContextCodeSetIDResolverMissing)
	}
	return resolver.ResolveSetID(ctx, tenantID, orgNodeKey, asOf)
}

func (r createOrgUnitPrecheckServerReader) IsOrgTreeInitialized(ctx context.Context, tenantID string) (bool, error) {
	reader, ok := any(r.store).(orgUnitTreeInitializationReader)
	if !ok {
		return false, nil
	}
	return reader.IsOrgTreeInitialized(ctx, tenantID)
}

func (r createOrgUnitPrecheckServerReader) ResolveSetIDStrategyFieldDecision(
	ctx context.Context,
	tenantID string,
	capabilityKey string,
	fieldKey string,
	businessUnitNodeKey string,
	asOf string,
) (orgunittypes.SetIDStrategyFieldDecision, bool, error) {
	resolvedSetID := ""
	businessUnitNodeKey = strings.TrimSpace(businessUnitNodeKey)
	if businessUnitNodeKey != "" {
		var err error
		resolvedSetID, err = r.ResolveSetID(ctx, tenantID, businessUnitNodeKey, asOf)
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
		businessUnitNodeKey,
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

func (r createOrgUnitPrecheckServerReader) ListEnabledTenantFieldConfigsAsOf(
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

func buildCreateOrgUnitPrecheckResultV1(
	ctx context.Context,
	store OrgUnitStore,
	input orgunitservices.CreateOrgUnitPrecheckInputV1,
) (orgunitservices.CreateOrgUnitPrecheckResultV1, error) {
	input.TenantID = strings.TrimSpace(input.TenantID)
	input.CapabilityKey = strings.TrimSpace(firstNonEmpty(input.CapabilityKey, orgUnitCreateFieldPolicyCapabilityKey))
	if strings.TrimSpace(input.EffectivePolicyVersion) == "" {
		effectivePolicyVersion, _ := resolveOrgUnitEffectivePolicyVersion(input.TenantID, input.CapabilityKey)
		input.EffectivePolicyVersion = effectivePolicyVersion
	}
	return orgunitservices.BuildCreateOrgUnitPrecheckProjectionV1(
		ctx,
		createOrgUnitPrecheckServerReader{store: store},
		input,
	)
}
