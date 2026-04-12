package server

import (
	"context"
	"strings"

	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
)

type assistantPolicyContractValues struct {
	PolicyContextDigest      string
	EffectivePolicyVersion   string
	ResolvedSetID            string
	SetIDSource              string
	PrecheckProjectionDigest string
	MutationPolicyVersion    string
}

type assistantOrgUnitVersionPolicyBindingSpec struct {
	CapabilityKey  string
	AppendIntent   string
	MaintainIntent string
}

func (s *assistantConversationService) enrichAuthoritativeOrgUnitDryRunWithPolicy(
	ctx context.Context,
	tenantID string,
	intent assistantIntentSpec,
	candidates []assistantCandidate,
	resolvedCandidateID string,
	dryRun assistantDryRunResult,
) assistantDryRunResult {
	if s == nil {
		return dryRun
	}
	dryRun = s.enrichCreateOrgUnitDryRunWithPolicy(ctx, tenantID, intent, candidates, resolvedCandidateID, dryRun)
	dryRun = s.enrichOrgUnitVersionDryRunWithPolicy(ctx, tenantID, intent, candidates, resolvedCandidateID, dryRun)
	return dryRun
}

func (s *assistantConversationService) enrichOrgUnitVersionDryRunWithPolicy(
	ctx context.Context,
	tenantID string,
	intent assistantIntentSpec,
	candidates []assistantCandidate,
	resolvedCandidateID string,
	dryRun assistantDryRunResult,
) assistantDryRunResult {
	if s == nil || s.orgStore == nil {
		return dryRun
	}
	binding, ok := assistantOrgUnitVersionPolicyBinding(intent.Action)
	if !ok {
		return dryRun
	}
	if assistantSliceHas(dryRun.ValidationErrors, "parent_candidate_not_found") {
		return dryRun
	}
	newParentOrgCode, newParentRequested, ok := assistantOrgUnitVersionPolicyParentOrgCode(intent, candidates, resolvedCandidateID, dryRun.ValidationErrors)
	if !ok {
		return dryRun
	}

	var (
		snapshot *assistantOrgUnitVersionProjectionSnapshot
		err      error
	)
	switch {
	case strings.TrimSpace(binding.AppendIntent) != "":
		var result orgunitservices.OrgUnitAppendVersionPrecheckResultV1
		result, err = buildOrgUnitAppendVersionPrecheckResultV1(ctx, s.orgStore, orgunitservices.OrgUnitAppendVersionPrecheckInputV1{
			Intent:                        strings.TrimSpace(binding.AppendIntent),
			TenantID:                      strings.TrimSpace(tenantID),
			CapabilityKey:                 strings.TrimSpace(binding.CapabilityKey),
			EffectiveDate:                 strings.TrimSpace(intent.EffectiveDate),
			OrgCode:                       strings.TrimSpace(intent.OrgCode),
			CanAdmin:                      canEditOrgNodes(ctx),
			CandidateConfirmationRequired: assistantSliceHas(dryRun.ValidationErrors, "candidate_confirmation_required"),
			NewName:                       strings.TrimSpace(intent.NewName),
			NewParentOrgCode:              newParentOrgCode,
			NewParentRequested:            newParentRequested,
		})
		if err == nil {
			snapshot = assistantOrgUnitVersionProjectionSnapshotFromAppendResult(result)
		}
	case strings.TrimSpace(binding.MaintainIntent) != "":
		var result orgunitservices.OrgUnitMaintainPrecheckResultV1
		result, err = buildOrgUnitMaintainPrecheckResultV1(ctx, s.orgStore, orgunitservices.OrgUnitMaintainPrecheckInputV1{
			Intent:                        strings.TrimSpace(binding.MaintainIntent),
			TenantID:                      strings.TrimSpace(tenantID),
			CapabilityKey:                 strings.TrimSpace(binding.CapabilityKey),
			EffectiveDate:                 strings.TrimSpace(intent.EffectiveDate),
			TargetEffectiveDate:           strings.TrimSpace(intent.TargetEffectiveDate),
			OrgCode:                       strings.TrimSpace(intent.OrgCode),
			CanAdmin:                      canEditOrgNodes(ctx),
			CandidateConfirmationRequired: assistantSliceHas(dryRun.ValidationErrors, "candidate_confirmation_required"),
			NewName:                       strings.TrimSpace(intent.NewName),
			NewParentOrgCode:              newParentOrgCode,
			NewParentRequested:            newParentRequested,
		})
		if err == nil {
			snapshot = assistantOrgUnitVersionProjectionSnapshotFromMaintainResult(result)
		}
	}
	if err != nil || snapshot == nil {
		return dryRun
	}
	dryRun.OrgUnitVersionProjection = snapshot
	dryRun.ValidationErrors = assistantOrgUnitVersionProjectionValidationErrors(snapshot)
	if explain := strings.TrimSpace(snapshot.Projection.PolicyExplain); explain != "" {
		dryRun.Explain = explain
	} else if len(dryRun.ValidationErrors) == 0 {
		dryRun.Explain = "计划已生成，等待确认后可提交"
	}
	return dryRun
}

func assistantOrgUnitVersionPolicyBinding(action string) (assistantOrgUnitVersionPolicyBindingSpec, bool) {
	switch strings.TrimSpace(action) {
	case assistantIntentAddOrgUnitVersion:
		return assistantOrgUnitVersionPolicyBindingSpec{
			CapabilityKey: orgUnitAddVersionFieldPolicyCapabilityKey,
			AppendIntent:  string(orgunitservices.OrgUnitWriteIntentAddVersion),
		}, true
	case assistantIntentInsertOrgUnitVersion:
		return assistantOrgUnitVersionPolicyBindingSpec{
			CapabilityKey: orgUnitInsertVersionFieldPolicyCapabilityKey,
			AppendIntent:  string(orgunitservices.OrgUnitWriteIntentInsertVersion),
		}, true
	case assistantIntentCorrectOrgUnit:
		return assistantOrgUnitVersionPolicyBindingSpec{
			CapabilityKey:  orgUnitCorrectFieldPolicyCapabilityKey,
			MaintainIntent: orgunitservices.OrgUnitMaintainIntentCorrect,
		}, true
	case assistantIntentRenameOrgUnit:
		return assistantOrgUnitVersionPolicyBindingSpec{
			CapabilityKey:  orgUnitWriteFieldPolicyCapabilityKey,
			MaintainIntent: orgunitservices.OrgUnitMaintainIntentRename,
		}, true
	case assistantIntentMoveOrgUnit:
		return assistantOrgUnitVersionPolicyBindingSpec{
			CapabilityKey:  orgUnitWriteFieldPolicyCapabilityKey,
			MaintainIntent: orgunitservices.OrgUnitMaintainIntentMove,
		}, true
	default:
		return assistantOrgUnitVersionPolicyBindingSpec{}, false
	}
}

func assistantOrgUnitVersionPolicyParentOrgCode(
	intent assistantIntentSpec,
	candidates []assistantCandidate,
	resolvedCandidateID string,
	validationErrors []string,
) (orgCode string, requested bool, ok bool) {
	if strings.TrimSpace(intent.NewParentRefText) == "" {
		return "", false, true
	}
	if strings.TrimSpace(resolvedCandidateID) != "" {
		return assistantResolvedCandidateCode(candidates, resolvedCandidateID), true, true
	}
	if len(candidates) == 1 {
		return assistantResolvedCandidateCode(candidates, strings.TrimSpace(candidates[0].CandidateID)), true, true
	}
	if assistantSliceHas(validationErrors, "candidate_confirmation_required") {
		return "", true, true
	}
	return "", true, false
}

func assistantActionRequiresPolicyProjection(action string) bool {
	switch strings.TrimSpace(action) {
	case assistantIntentCreateOrgUnit,
		assistantIntentAddOrgUnitVersion,
		assistantIntentInsertOrgUnitVersion,
		assistantIntentCorrectOrgUnit,
		assistantIntentRenameOrgUnit,
		assistantIntentMoveOrgUnit:
		return true
	default:
		return false
	}
}

func assistantTurnPolicyProjectionContractMissing(turn *assistantTurn) bool {
	return assistantCreateOrgUnitProjectionContractMissing(turn) || assistantOrgUnitVersionProjectionContractMissing(turn)
}

func assistantPolicyContractValuesFromTurn(turn *assistantTurn) (assistantPolicyContractValues, bool) {
	if projection, ok := assistantCreateOrgUnitProjectionForTurn(turn); ok {
		return assistantPolicyContractValues{
			PolicyContextDigest:      strings.TrimSpace(projection.PolicyContext.PolicyContextDigest),
			EffectivePolicyVersion:   strings.TrimSpace(projection.Projection.EffectivePolicyVersion),
			ResolvedSetID:            strings.TrimSpace(projection.Projection.ResolvedSetID),
			SetIDSource:              strings.TrimSpace(projection.Projection.SetIDSource),
			PrecheckProjectionDigest: strings.TrimSpace(projection.Projection.ProjectionDigest),
			MutationPolicyVersion:    strings.TrimSpace(projection.Projection.MutationPolicyVersion),
		}, true
	}
	if projection, ok := assistantOrgUnitVersionProjectionForTurn(turn); ok {
		return assistantPolicyContractValues{
			PolicyContextDigest:      strings.TrimSpace(projection.PolicyContext.PolicyContextDigest),
			EffectivePolicyVersion:   strings.TrimSpace(projection.Projection.EffectivePolicyVersion),
			ResolvedSetID:            strings.TrimSpace(projection.Projection.ResolvedSetID),
			SetIDSource:              strings.TrimSpace(projection.Projection.SetIDSource),
			PrecheckProjectionDigest: strings.TrimSpace(projection.Projection.ProjectionDigest),
			MutationPolicyVersion:    strings.TrimSpace(projection.Projection.MutationPolicyVersion),
		}, true
	}
	return assistantPolicyContractValues{}, false
}

func assistantPolicyContractValuesFromDryRun(dryRun assistantDryRunResult) (assistantPolicyContractValues, bool) {
	if dryRun.CreateOrgUnitProjection != nil {
		return assistantPolicyContractValues{
			PolicyContextDigest:      strings.TrimSpace(dryRun.CreateOrgUnitProjection.PolicyContext.PolicyContextDigest),
			EffectivePolicyVersion:   strings.TrimSpace(dryRun.CreateOrgUnitProjection.Projection.EffectivePolicyVersion),
			ResolvedSetID:            strings.TrimSpace(dryRun.CreateOrgUnitProjection.Projection.ResolvedSetID),
			SetIDSource:              strings.TrimSpace(dryRun.CreateOrgUnitProjection.Projection.SetIDSource),
			PrecheckProjectionDigest: strings.TrimSpace(dryRun.CreateOrgUnitProjection.Projection.ProjectionDigest),
			MutationPolicyVersion:    strings.TrimSpace(dryRun.CreateOrgUnitProjection.Projection.MutationPolicyVersion),
		}, true
	}
	if dryRun.OrgUnitVersionProjection != nil {
		return assistantPolicyContractValues{
			PolicyContextDigest:      strings.TrimSpace(dryRun.OrgUnitVersionProjection.PolicyContext.PolicyContextDigest),
			EffectivePolicyVersion:   strings.TrimSpace(dryRun.OrgUnitVersionProjection.Projection.EffectivePolicyVersion),
			ResolvedSetID:            strings.TrimSpace(dryRun.OrgUnitVersionProjection.Projection.ResolvedSetID),
			SetIDSource:              strings.TrimSpace(dryRun.OrgUnitVersionProjection.Projection.SetIDSource),
			PrecheckProjectionDigest: strings.TrimSpace(dryRun.OrgUnitVersionProjection.Projection.ProjectionDigest),
			MutationPolicyVersion:    strings.TrimSpace(dryRun.OrgUnitVersionProjection.Projection.MutationPolicyVersion),
		}, true
	}
	return assistantPolicyContractValues{}, false
}
