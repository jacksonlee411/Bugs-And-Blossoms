package server

import (
	"context"
	"strings"

	orgunittypes "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
)

type orgUnitMutationTargetReader interface {
	ResolveMutationTargetEvent(ctx context.Context, tenantID string, orgNodeKey string, effectiveDate string) (orgUnitMutationTargetEvent, error)
}

type orgUnitMaintainPrecheckServerReader struct {
	orgUnitAppendVersionPrecheckServerReader
}

func (r orgUnitMaintainPrecheckServerReader) ResolveTargetExistsAsOf(
	ctx context.Context,
	tenantID string,
	orgNodeKey string,
	asOf string,
) (bool, error) {
	facts, err := r.ResolveAppendFacts(ctx, tenantID, orgNodeKey, asOf)
	if err != nil {
		return false, err
	}
	return facts.TargetExistsAsOf, nil
}

func (r orgUnitMaintainPrecheckServerReader) ResolveMutationTargetEvent(
	ctx context.Context,
	tenantID string,
	orgNodeKey string,
	effectiveDate string,
) (orgunitservices.OrgUnitMaintainTargetEventV1, error) {
	reader, ok := any(r.store).(orgUnitMutationTargetReader)
	if !ok {
		return orgunitservices.OrgUnitMaintainTargetEventV1{}, nil
	}
	target, err := reader.ResolveMutationTargetEvent(ctx, tenantID, orgNodeKey, effectiveDate)
	if err != nil {
		return orgunitservices.OrgUnitMaintainTargetEventV1{}, err
	}
	return orgunitservices.OrgUnitMaintainTargetEventV1{
		HasEffective:       target.HasEffective,
		EffectiveEventType: orgunittypes.OrgUnitEventType(strings.TrimSpace(target.EffectiveEventType)),
		HasRaw:             target.HasRaw,
		RawEventType:       orgunittypes.OrgUnitEventType(strings.TrimSpace(target.RawEventType)),
	}, nil
}

func buildOrgUnitMaintainPrecheckResultV1(
	ctx context.Context,
	store OrgUnitStore,
	input orgunitservices.OrgUnitMaintainPrecheckInputV1,
) (orgunitservices.OrgUnitMaintainPrecheckResultV1, error) {
	input.TenantID = strings.TrimSpace(input.TenantID)
	input.Intent = strings.TrimSpace(input.Intent)
	input.CapabilityKey = strings.TrimSpace(input.CapabilityKey)
	if input.CapabilityKey == "" {
		switch strings.TrimSpace(input.Intent) {
		case orgunitservices.OrgUnitMaintainIntentCorrect:
			input.CapabilityKey = orgUnitCorrectFieldPolicyCapabilityKey
		case orgunitservices.OrgUnitMaintainIntentRename,
			orgunitservices.OrgUnitMaintainIntentMove,
			orgunitservices.OrgUnitMaintainIntentDisable,
			orgunitservices.OrgUnitMaintainIntentEnable:
			input.CapabilityKey = orgUnitWriteFieldPolicyCapabilityKey
		}
	}
	if strings.TrimSpace(input.EffectivePolicyVersion) == "" && strings.TrimSpace(input.CapabilityKey) != "" {
		effectivePolicyVersion, _ := resolveOrgUnitEffectivePolicyVersion(input.TenantID, input.CapabilityKey)
		input.EffectivePolicyVersion = effectivePolicyVersion
	}
	return orgunitservices.BuildOrgUnitMaintainPrecheckProjectionV1(
		ctx,
		orgUnitMaintainPrecheckServerReader{
			orgUnitAppendVersionPrecheckServerReader: orgUnitAppendVersionPrecheckServerReader{store: store},
		},
		input,
	)
}
