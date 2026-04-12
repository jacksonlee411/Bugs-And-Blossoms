package server

import (
	"strings"

	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
)

type assistantOrgUnitVersionProjectionSnapshot struct {
	PolicyContextContractVersion      string                                                   `json:"policy_context_contract_version"`
	PrecheckProjectionContractVersion string                                                   `json:"precheck_projection_contract_version"`
	PolicyContext                     orgunitservices.OrgUnitAppendVersionPolicyContextV1      `json:"policy_context"`
	Projection                        orgunitservices.OrgUnitAppendVersionPrecheckProjectionV1 `json:"projection"`
}

func assistantOrgUnitVersionProjectionSnapshotFromResult(
	result orgunitservices.OrgUnitAppendVersionPrecheckResultV1,
) *assistantOrgUnitVersionProjectionSnapshot {
	return &assistantOrgUnitVersionProjectionSnapshot{
		PolicyContextContractVersion:      orgunitservices.OrgUnitAppendVersionPolicyContextContractVersionV1,
		PrecheckProjectionContractVersion: orgunitservices.OrgUnitAppendVersionPrecheckProjectionContractV1,
		PolicyContext:                     result.PolicyContext,
		Projection:                        orgunitservices.CloneOrgUnitAppendVersionProjectionV1(result.Projection),
	}
}

func assistantCloneOrgUnitVersionProjectionSnapshot(
	snapshot *assistantOrgUnitVersionProjectionSnapshot,
) *assistantOrgUnitVersionProjectionSnapshot {
	if snapshot == nil {
		return nil
	}
	copySnapshot := *snapshot
	copySnapshot.Projection = orgunitservices.CloneOrgUnitAppendVersionProjectionV1(snapshot.Projection)
	return &copySnapshot
}

func assistantOrgUnitVersionProjectionForTurn(
	turn *assistantTurn,
) (*assistantOrgUnitVersionProjectionSnapshot, bool) {
	if turn == nil {
		return nil, false
	}
	switch strings.TrimSpace(turn.Intent.Action) {
	case assistantIntentAddOrgUnitVersion, assistantIntentInsertOrgUnitVersion:
	default:
		return nil, false
	}
	if turn.DryRun.OrgUnitVersionProjection == nil {
		return nil, false
	}
	return turn.DryRun.OrgUnitVersionProjection, true
}

func assistantOrgUnitVersionProjectionValidationErrors(
	snapshot *assistantOrgUnitVersionProjectionSnapshot,
) []string {
	if snapshot == nil {
		return nil
	}
	out := append([]string(nil), snapshot.Projection.RejectionReasons...)
	for _, fieldKey := range snapshot.Projection.MissingFields {
		switch strings.TrimSpace(fieldKey) {
		case "effective_date":
			out = append(out, "missing_effective_date")
		case "org_code":
			out = append(out, "missing_org_code")
		case "change_fields":
			out = append(out, "missing_change_fields")
		case "new_name":
			out = append(out, "missing_new_name")
		case "new_parent_ref_text":
			out = append(out, "missing_new_parent_ref_text")
		default:
			out = append(out, "FIELD_REQUIRED_VALUE_MISSING")
		}
	}
	if strings.TrimSpace(snapshot.Projection.Readiness) == "candidate_confirmation_required" {
		out = append(out, "candidate_confirmation_required")
	}
	return assistantNormalizeValidationErrors(out)
}

func assistantOrgUnitVersionProjectionContractMissing(turn *assistantTurn) bool {
	if turn == nil {
		return false
	}
	switch strings.TrimSpace(turn.Intent.Action) {
	case assistantIntentAddOrgUnitVersion, assistantIntentInsertOrgUnitVersion:
	default:
		return false
	}
	if strings.TrimSpace(turn.Intent.IntentSchemaVersion) == "" {
		return false
	}
	snapshot, ok := assistantOrgUnitVersionProjectionForTurn(turn)
	if !ok {
		return true
	}
	if strings.TrimSpace(snapshot.PolicyContextContractVersion) != orgunitservices.OrgUnitAppendVersionPolicyContextContractVersionV1 {
		return true
	}
	if strings.TrimSpace(snapshot.PrecheckProjectionContractVersion) != orgunitservices.OrgUnitAppendVersionPrecheckProjectionContractV1 {
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
