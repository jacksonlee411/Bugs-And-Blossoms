package server

import (
	"strings"

	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
)

type assistantCreateOrgUnitProjectionSnapshot struct {
	PolicyContextContractVersion      string                                            `json:"policy_context_contract_version"`
	PrecheckProjectionContractVersion string                                            `json:"precheck_projection_contract_version"`
	PolicyContext                     orgunitservices.CreateOrgUnitPolicyContextV1      `json:"policy_context"`
	Projection                        orgunitservices.CreateOrgUnitPrecheckProjectionV1 `json:"projection"`
}

func assistantCreateOrgUnitProjectionSnapshotFromResult(
	result orgunitservices.CreateOrgUnitPrecheckResultV1,
) *assistantCreateOrgUnitProjectionSnapshot {
	snapshot := &assistantCreateOrgUnitProjectionSnapshot{
		PolicyContextContractVersion:      orgunitservices.CreateOrgUnitPolicyContextContractVersionV1,
		PrecheckProjectionContractVersion: orgunitservices.CreateOrgUnitPrecheckProjectionContractV1,
		PolicyContext:                     result.PolicyContext,
		Projection:                        result.Projection,
	}
	snapshot.Projection.MissingFields = append([]string(nil), result.Projection.MissingFields...)
	snapshot.Projection.CandidateConfirmationRequirements = append([]string(nil), result.Projection.CandidateConfirmationRequirements...)
	snapshot.Projection.RejectionReasons = append([]string(nil), result.Projection.RejectionReasons...)
	snapshot.Projection.FieldDecisions = cloneCreateOrgUnitFieldDecisions(result.Projection.FieldDecisions)
	return snapshot
}

func assistantCloneCreateOrgUnitProjectionSnapshot(
	snapshot *assistantCreateOrgUnitProjectionSnapshot,
) *assistantCreateOrgUnitProjectionSnapshot {
	if snapshot == nil {
		return nil
	}
	copySnapshot := *snapshot
	copySnapshot.Projection.MissingFields = append([]string(nil), snapshot.Projection.MissingFields...)
	copySnapshot.Projection.CandidateConfirmationRequirements = append([]string(nil), snapshot.Projection.CandidateConfirmationRequirements...)
	copySnapshot.Projection.RejectionReasons = append([]string(nil), snapshot.Projection.RejectionReasons...)
	copySnapshot.Projection.FieldDecisions = cloneCreateOrgUnitFieldDecisions(snapshot.Projection.FieldDecisions)
	return &copySnapshot
}

func cloneCreateOrgUnitFieldDecisions(
	items []orgunitservices.CreateOrgUnitFieldDecisionV1,
) []orgunitservices.CreateOrgUnitFieldDecisionV1 {
	if len(items) == 0 {
		return nil
	}
	out := make([]orgunitservices.CreateOrgUnitFieldDecisionV1, 0, len(items))
	for _, item := range items {
		copyItem := item
		copyItem.AllowedValueCodes = append([]string(nil), item.AllowedValueCodes...)
		out = append(out, copyItem)
	}
	return out
}

func assistantCreateOrgUnitProjectionForTurn(
	turn *assistantTurn,
) (*assistantCreateOrgUnitProjectionSnapshot, bool) {
	if turn == nil || strings.TrimSpace(turn.Intent.Action) != assistantIntentCreateOrgUnit {
		return nil, false
	}
	if turn.DryRun.CreateOrgUnitProjection == nil {
		return nil, false
	}
	return turn.DryRun.CreateOrgUnitProjection, true
}

func assistantCreateOrgUnitProjectionValidationErrors(
	snapshot *assistantCreateOrgUnitProjectionSnapshot,
) []string {
	if snapshot == nil {
		return nil
	}
	out := append([]string(nil), snapshot.Projection.RejectionReasons...)
	for _, fieldKey := range snapshot.Projection.MissingFields {
		switch strings.TrimSpace(fieldKey) {
		case "effective_date":
			out = append(out, "missing_effective_date")
		case "name":
			out = append(out, "missing_entity_name")
		default:
			out = append(out, "FIELD_REQUIRED_VALUE_MISSING")
		}
	}
	if strings.TrimSpace(snapshot.Projection.Readiness) == "candidate_confirmation_required" {
		out = append(out, "candidate_confirmation_required")
	}
	return assistantNormalizeValidationErrors(out)
}

func assistantCreateOrgUnitProjectionContractMissing(turn *assistantTurn) bool {
	if turn == nil || strings.TrimSpace(turn.Intent.Action) != assistantIntentCreateOrgUnit {
		return false
	}
	if strings.TrimSpace(turn.Intent.IntentSchemaVersion) == "" {
		return false
	}
	snapshot, ok := assistantCreateOrgUnitProjectionForTurn(turn)
	if !ok {
		return true
	}
	if strings.TrimSpace(snapshot.PolicyContextContractVersion) != orgunitservices.CreateOrgUnitPolicyContextContractVersionV1 {
		return true
	}
	if strings.TrimSpace(snapshot.PrecheckProjectionContractVersion) != orgunitservices.CreateOrgUnitPrecheckProjectionContractV1 {
		return true
	}
	if strings.TrimSpace(snapshot.PolicyContext.PolicyContextDigest) == "" {
		return true
	}
	if strings.TrimSpace(snapshot.Projection.EffectivePolicyVersion) == "" {
		return true
	}
	if strings.TrimSpace(snapshot.Projection.ProjectionDigest) == "" {
		return true
	}
	return strings.TrimSpace(snapshot.Projection.MutationPolicyVersion) == ""
}

func assistantTaskCreatePolicyContractIncomplete(snapshot assistantTaskContractSnapshot) bool {
	hasCreateSnapshot := strings.TrimSpace(snapshot.PolicyContextDigest) != "" ||
		strings.TrimSpace(snapshot.EffectivePolicyVersion) != "" ||
		strings.TrimSpace(snapshot.PrecheckProjectionDigest) != "" ||
		strings.TrimSpace(snapshot.MutationPolicyVersion) != "" ||
		strings.TrimSpace(snapshot.ResolvedSetID) != "" ||
		strings.TrimSpace(snapshot.SetIDSource) != ""
	if !hasCreateSnapshot {
		return false
	}
	if strings.TrimSpace(snapshot.PolicyContextDigest) == "" {
		return true
	}
	if strings.TrimSpace(snapshot.EffectivePolicyVersion) == "" {
		return true
	}
	if strings.TrimSpace(snapshot.PrecheckProjectionDigest) == "" {
		return true
	}
	return strings.TrimSpace(snapshot.MutationPolicyVersion) == ""
}
