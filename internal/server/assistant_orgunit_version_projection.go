package server

import (
	"strings"

	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
)

type assistantOrgUnitVersionPolicyContext struct {
	TenantID            string `json:"tenant_id"`
	CapabilityKey       string `json:"capability_key"`
	Intent              string `json:"intent"`
	EffectiveDate       string `json:"effective_date,omitempty"`
	TargetEffectiveDate string `json:"target_effective_date,omitempty"`
	OrgCode             string `json:"org_code"`
	OrgNodeKey          string `json:"org_node_key"`
	ResolvedSetID       string `json:"resolved_setid"`
	SetIDSource         string `json:"setid_source"`
	PolicyContextDigest string `json:"policy_context_digest"`
}

type assistantOrgUnitVersionFieldDecision struct {
	FieldKey             string   `json:"field_key"`
	Visible              bool     `json:"visible"`
	Required             bool     `json:"required"`
	Maintainable         bool     `json:"maintainable"`
	FieldPayloadKey      string   `json:"field_payload_key"`
	ResolvedDefaultValue string   `json:"resolved_default_value"`
	DefaultRuleRef       string   `json:"default_rule_ref"`
	AllowedValueCodes    []string `json:"allowed_value_codes"`
}

type assistantOrgUnitVersionProjection struct {
	Readiness                         string                                 `json:"readiness"`
	MissingFields                     []string                               `json:"missing_fields"`
	FieldDecisions                    []assistantOrgUnitVersionFieldDecision `json:"field_decisions"`
	CandidateConfirmationRequirements []string                               `json:"candidate_confirmation_requirements"`
	PendingDraftSummary               string                                 `json:"pending_draft_summary"`
	EffectivePolicyVersion            string                                 `json:"effective_policy_version"`
	MutationPolicyVersion             string                                 `json:"mutation_policy_version"`
	ResolvedSetID                     string                                 `json:"resolved_setid"`
	SetIDSource                       string                                 `json:"setid_source"`
	PolicyExplain                     string                                 `json:"policy_explain"`
	RejectionReasons                  []string                               `json:"rejection_reasons"`
	ProjectionDigest                  string                                 `json:"projection_digest"`
}

type assistantOrgUnitVersionProjectionSnapshot struct {
	PolicyContextContractVersion      string                               `json:"policy_context_contract_version"`
	PrecheckProjectionContractVersion string                               `json:"precheck_projection_contract_version"`
	PolicyContext                     assistantOrgUnitVersionPolicyContext `json:"policy_context"`
	Projection                        assistantOrgUnitVersionProjection    `json:"projection"`
}

func assistantOrgUnitVersionProjectionSnapshotFromAppendResult(
	result orgunitservices.OrgUnitAppendVersionPrecheckResultV1,
) *assistantOrgUnitVersionProjectionSnapshot {
	return &assistantOrgUnitVersionProjectionSnapshot{
		PolicyContextContractVersion:      orgunitservices.OrgUnitAppendVersionPolicyContextContractVersionV1,
		PrecheckProjectionContractVersion: orgunitservices.OrgUnitAppendVersionPrecheckProjectionContractV1,
		PolicyContext: assistantOrgUnitVersionPolicyContext{
			TenantID:            strings.TrimSpace(result.PolicyContext.TenantID),
			CapabilityKey:       strings.TrimSpace(result.PolicyContext.CapabilityKey),
			Intent:              strings.TrimSpace(result.PolicyContext.Intent),
			EffectiveDate:       strings.TrimSpace(result.PolicyContext.EffectiveDate),
			OrgCode:             strings.TrimSpace(result.PolicyContext.OrgCode),
			OrgNodeKey:          strings.TrimSpace(result.PolicyContext.OrgNodeKey),
			ResolvedSetID:       strings.TrimSpace(result.PolicyContext.ResolvedSetID),
			SetIDSource:         strings.TrimSpace(result.PolicyContext.SetIDSource),
			PolicyContextDigest: strings.TrimSpace(result.PolicyContext.PolicyContextDigest),
		},
		Projection: assistantOrgUnitVersionProjection{
			Readiness:                         strings.TrimSpace(result.Projection.Readiness),
			MissingFields:                     append([]string(nil), result.Projection.MissingFields...),
			FieldDecisions:                    assistantOrgUnitVersionFieldDecisionsFromAppend(result.Projection.FieldDecisions),
			CandidateConfirmationRequirements: append([]string(nil), result.Projection.CandidateConfirmationRequirements...),
			PendingDraftSummary:               strings.TrimSpace(result.Projection.PendingDraftSummary),
			EffectivePolicyVersion:            strings.TrimSpace(result.Projection.EffectivePolicyVersion),
			MutationPolicyVersion:             strings.TrimSpace(result.Projection.MutationPolicyVersion),
			ResolvedSetID:                     strings.TrimSpace(result.Projection.ResolvedSetID),
			SetIDSource:                       strings.TrimSpace(result.Projection.SetIDSource),
			PolicyExplain:                     strings.TrimSpace(result.Projection.PolicyExplain),
			RejectionReasons:                  append([]string(nil), result.Projection.RejectionReasons...),
			ProjectionDigest:                  strings.TrimSpace(result.Projection.ProjectionDigest),
		},
	}
}

func assistantOrgUnitVersionProjectionSnapshotFromMaintainResult(
	result orgunitservices.OrgUnitMaintainPrecheckResultV1,
) *assistantOrgUnitVersionProjectionSnapshot {
	return &assistantOrgUnitVersionProjectionSnapshot{
		PolicyContextContractVersion:      orgunitservices.OrgUnitMaintainPolicyContextContractVersionV1,
		PrecheckProjectionContractVersion: orgunitservices.OrgUnitMaintainPrecheckProjectionContractV1,
		PolicyContext: assistantOrgUnitVersionPolicyContext{
			TenantID:            strings.TrimSpace(result.PolicyContext.TenantID),
			CapabilityKey:       strings.TrimSpace(result.PolicyContext.CapabilityKey),
			Intent:              strings.TrimSpace(result.PolicyContext.Intent),
			EffectiveDate:       strings.TrimSpace(result.PolicyContext.EffectiveDate),
			TargetEffectiveDate: strings.TrimSpace(result.PolicyContext.TargetEffectiveDate),
			OrgCode:             strings.TrimSpace(result.PolicyContext.OrgCode),
			OrgNodeKey:          strings.TrimSpace(result.PolicyContext.OrgNodeKey),
			ResolvedSetID:       strings.TrimSpace(result.PolicyContext.ResolvedSetID),
			SetIDSource:         strings.TrimSpace(result.PolicyContext.SetIDSource),
			PolicyContextDigest: strings.TrimSpace(result.PolicyContext.PolicyContextDigest),
		},
		Projection: assistantOrgUnitVersionProjection{
			Readiness:                         strings.TrimSpace(result.Projection.Readiness),
			MissingFields:                     append([]string(nil), result.Projection.MissingFields...),
			FieldDecisions:                    assistantOrgUnitVersionFieldDecisionsFromMaintain(result.Projection.FieldDecisions),
			CandidateConfirmationRequirements: append([]string(nil), result.Projection.CandidateConfirmationRequirements...),
			PendingDraftSummary:               strings.TrimSpace(result.Projection.PendingDraftSummary),
			EffectivePolicyVersion:            strings.TrimSpace(result.Projection.EffectivePolicyVersion),
			MutationPolicyVersion:             strings.TrimSpace(result.Projection.MutationPolicyVersion),
			ResolvedSetID:                     strings.TrimSpace(result.Projection.ResolvedSetID),
			SetIDSource:                       strings.TrimSpace(result.Projection.SetIDSource),
			PolicyExplain:                     strings.TrimSpace(result.Projection.PolicyExplain),
			RejectionReasons:                  append([]string(nil), result.Projection.RejectionReasons...),
			ProjectionDigest:                  strings.TrimSpace(result.Projection.ProjectionDigest),
		},
	}
}

func assistantCloneOrgUnitVersionProjectionSnapshot(
	snapshot *assistantOrgUnitVersionProjectionSnapshot,
) *assistantOrgUnitVersionProjectionSnapshot {
	if snapshot == nil {
		return nil
	}
	copySnapshot := *snapshot
	copySnapshot.Projection = assistantCloneOrgUnitVersionProjection(snapshot.Projection)
	return &copySnapshot
}

func assistantCloneOrgUnitVersionProjection(projection assistantOrgUnitVersionProjection) assistantOrgUnitVersionProjection {
	return assistantOrgUnitVersionProjection{
		Readiness:                         strings.TrimSpace(projection.Readiness),
		MissingFields:                     append([]string(nil), projection.MissingFields...),
		FieldDecisions:                    assistantCloneOrgUnitVersionFieldDecisions(projection.FieldDecisions),
		CandidateConfirmationRequirements: append([]string(nil), projection.CandidateConfirmationRequirements...),
		PendingDraftSummary:               strings.TrimSpace(projection.PendingDraftSummary),
		EffectivePolicyVersion:            strings.TrimSpace(projection.EffectivePolicyVersion),
		MutationPolicyVersion:             strings.TrimSpace(projection.MutationPolicyVersion),
		ResolvedSetID:                     strings.TrimSpace(projection.ResolvedSetID),
		SetIDSource:                       strings.TrimSpace(projection.SetIDSource),
		PolicyExplain:                     strings.TrimSpace(projection.PolicyExplain),
		RejectionReasons:                  append([]string(nil), projection.RejectionReasons...),
		ProjectionDigest:                  strings.TrimSpace(projection.ProjectionDigest),
	}
}

func assistantCloneOrgUnitVersionFieldDecisions(items []assistantOrgUnitVersionFieldDecision) []assistantOrgUnitVersionFieldDecision {
	if len(items) == 0 {
		return nil
	}
	out := make([]assistantOrgUnitVersionFieldDecision, 0, len(items))
	for _, item := range items {
		copyItem := item
		copyItem.AllowedValueCodes = append([]string(nil), item.AllowedValueCodes...)
		out = append(out, copyItem)
	}
	return out
}

func assistantOrgUnitVersionFieldDecisionsFromAppend(
	items []orgunitservices.OrgUnitAppendVersionFieldDecisionV1,
) []assistantOrgUnitVersionFieldDecision {
	if len(items) == 0 {
		return nil
	}
	out := make([]assistantOrgUnitVersionFieldDecision, 0, len(items))
	for _, item := range items {
		out = append(out, assistantOrgUnitVersionFieldDecision{
			FieldKey:             strings.TrimSpace(item.FieldKey),
			Visible:              item.Visible,
			Required:             item.Required,
			Maintainable:         item.Maintainable,
			FieldPayloadKey:      strings.TrimSpace(item.FieldPayloadKey),
			ResolvedDefaultValue: strings.TrimSpace(item.ResolvedDefaultValue),
			DefaultRuleRef:       strings.TrimSpace(item.DefaultRuleRef),
			AllowedValueCodes:    append([]string(nil), item.AllowedValueCodes...),
		})
	}
	return out
}

func assistantOrgUnitVersionFieldDecisionsFromMaintain(
	items []orgunitservices.OrgUnitMaintainFieldDecisionV1,
) []assistantOrgUnitVersionFieldDecision {
	if len(items) == 0 {
		return nil
	}
	out := make([]assistantOrgUnitVersionFieldDecision, 0, len(items))
	for _, item := range items {
		out = append(out, assistantOrgUnitVersionFieldDecision{
			FieldKey:             strings.TrimSpace(item.FieldKey),
			Visible:              item.Visible,
			Required:             item.Required,
			Maintainable:         item.Maintainable,
			FieldPayloadKey:      strings.TrimSpace(item.FieldPayloadKey),
			ResolvedDefaultValue: strings.TrimSpace(item.ResolvedDefaultValue),
			DefaultRuleRef:       strings.TrimSpace(item.DefaultRuleRef),
			AllowedValueCodes:    append([]string(nil), item.AllowedValueCodes...),
		})
	}
	return out
}

func assistantOrgUnitVersionProjectionForTurn(
	turn *assistantTurn,
) (*assistantOrgUnitVersionProjectionSnapshot, bool) {
	if turn == nil {
		return nil, false
	}
	if !assistantOrgUnitVersionProjectionActionSupported(turn.Intent.Action) {
		return nil, false
	}
	if turn.DryRun.OrgUnitVersionProjection == nil {
		return nil, false
	}
	return turn.DryRun.OrgUnitVersionProjection, true
}

func assistantOrgUnitVersionProjectionActionSupported(action string) bool {
	switch strings.TrimSpace(action) {
	case assistantIntentAddOrgUnitVersion,
		assistantIntentInsertOrgUnitVersion,
		assistantIntentCorrectOrgUnit,
		assistantIntentDisableOrgUnit,
		assistantIntentEnableOrgUnit,
		assistantIntentRenameOrgUnit,
		assistantIntentMoveOrgUnit:
		return true
	default:
		return false
	}
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
		case "target_effective_date":
			out = append(out, "missing_target_effective_date")
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
	expectedPolicyContract, expectedProjectionContract, ok := assistantOrgUnitVersionProjectionExpectedContracts(turn.Intent.Action)
	if !ok {
		return false
	}
	if strings.TrimSpace(turn.Intent.IntentSchemaVersion) == "" {
		return false
	}
	snapshot, ok := assistantOrgUnitVersionProjectionForTurn(turn)
	if !ok {
		return true
	}
	if strings.TrimSpace(snapshot.PolicyContextContractVersion) != expectedPolicyContract {
		return true
	}
	if strings.TrimSpace(snapshot.PrecheckProjectionContractVersion) != expectedProjectionContract {
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

func assistantOrgUnitVersionProjectionExpectedContracts(action string) (string, string, bool) {
	switch strings.TrimSpace(action) {
	case assistantIntentAddOrgUnitVersion, assistantIntentInsertOrgUnitVersion:
		return orgunitservices.OrgUnitAppendVersionPolicyContextContractVersionV1, orgunitservices.OrgUnitAppendVersionPrecheckProjectionContractV1, true
	case assistantIntentCorrectOrgUnit,
		assistantIntentDisableOrgUnit,
		assistantIntentEnableOrgUnit,
		assistantIntentRenameOrgUnit,
		assistantIntentMoveOrgUnit:
		return orgunitservices.OrgUnitMaintainPolicyContextContractVersionV1, orgunitservices.OrgUnitMaintainPrecheckProjectionContractV1, true
	default:
		return "", "", false
	}
}
