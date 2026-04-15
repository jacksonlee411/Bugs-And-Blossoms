package server

import (
	"context"
	"strings"

	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
)

func (s *assistantConversationService) enrichCreateOrgUnitDryRunWithPolicy(
	ctx context.Context,
	tenantID string,
	intent assistantIntentSpec,
	candidates []assistantCandidate,
	resolvedCandidateID string,
	dryRun assistantDryRunResult,
) assistantDryRunResult {
	if s == nil || s.orgStore == nil || strings.TrimSpace(intent.Action) != assistantIntentCreateOrgUnit {
		return dryRun
	}
	if assistantSliceHas(dryRun.ValidationErrors, "parent_candidate_not_found") {
		return dryRun
	}
	parentOrgCode, ok := assistantCreateOrgUnitPolicyParentOrgCode(intent, candidates, resolvedCandidateID, dryRun.ValidationErrors)
	if !ok {
		return dryRun
	}

	result, err := buildCreateOrgUnitPrecheckResultV1(ctx, s.orgStore, orgunitservices.CreateOrgUnitPrecheckInputV1{
		TenantID:                      strings.TrimSpace(tenantID),
		CapabilityKey:                 orgUnitCreateFieldPolicyCapabilityKey,
		EffectiveDate:                 strings.TrimSpace(intent.EffectiveDate),
		BusinessUnitOrgCode:           parentOrgCode,
		CanAdmin:                      canEditOrgNodes(ctx),
		CandidateConfirmationRequired: assistantSliceHas(dryRun.ValidationErrors, "candidate_confirmation_required"),
		Name:                          strings.TrimSpace(intent.EntityName),
		OrgCode:                       strings.TrimSpace(intent.OrgCode),
		Ext:                           map[string]any{},
	})
	if err != nil {
		return dryRun
	}

	snapshot := assistantCreateOrgUnitProjectionSnapshotFromResult(result)
	dryRun.CreateOrgUnitProjection = snapshot
	dryRun.ValidationErrors = assistantCreateOrgUnitProjectionValidationErrors(snapshot)
	if explain := strings.TrimSpace(snapshot.Projection.PolicyExplain); explain != "" {
		dryRun.Explain = explain
	}
	return dryRun
}

func assistantResolvedCandidateCode(candidates []assistantCandidate, resolvedCandidateID string) string {
	needle := strings.TrimSpace(resolvedCandidateID)
	if needle == "" {
		return ""
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.CandidateID) != needle {
			continue
		}
		if code := strings.TrimSpace(candidate.CandidateCode); code != "" {
			return code
		}
		return strings.TrimSpace(candidate.CandidateID)
	}
	return needle
}

func assistantCreateOrgUnitPolicyParentOrgCode(
	intent assistantIntentSpec,
	candidates []assistantCandidate,
	resolvedCandidateID string,
	validationErrors []string,
) (string, bool) {
	if strings.TrimSpace(resolvedCandidateID) != "" {
		return assistantResolvedCandidateCode(candidates, resolvedCandidateID), true
	}
	if assistantSliceHas(validationErrors, "candidate_confirmation_required") {
		return "", true
	}
	if strings.TrimSpace(intent.ParentRefText) == "" {
		return "", false
	}
	if len(candidates) == 1 {
		return assistantResolvedCandidateCode(candidates, strings.TrimSpace(candidates[0].CandidateID)), true
	}
	return "", false
}
