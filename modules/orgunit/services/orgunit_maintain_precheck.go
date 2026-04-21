package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
)

const (
	OrgUnitMaintainPolicyContextContractVersionV1 = "orgunit.maintain.policy_context.v1"
	OrgUnitMaintainPrecheckProjectionContractV1   = "orgunit.maintain.precheck_projection.v1"

	OrgUnitMaintainIntentCorrect = "correct"
	OrgUnitMaintainIntentDisable = "disable"
	OrgUnitMaintainIntentEnable  = "enable"
	OrgUnitMaintainIntentRename  = "rename"
	OrgUnitMaintainIntentMove    = "move"

	orgUnitMaintainReadinessReady                 = "ready"
	orgUnitMaintainReadinessMissingFields         = "missing_fields"
	orgUnitMaintainReadinessCandidateConfirmation = "candidate_confirmation_required"
	orgUnitMaintainReadinessRejected              = "rejected"

	orgUnitMaintainContextCodeOrgInvalid = "org_context_invalid"

	orgUnitMaintainCandidateRequirementNewParent = "new_parent_org_code"
)

type OrgUnitMaintainPolicyContextV1 struct {
	TenantID            string `json:"tenant_id"`
	Intent              string `json:"intent"`
	EffectiveDate       string `json:"effective_date,omitempty"`
	TargetEffectiveDate string `json:"target_effective_date,omitempty"`
	OrgCode             string `json:"org_code"`
	OrgNodeKey          string `json:"org_node_key"`
	PolicyContextDigest string `json:"policy_context_digest"`
}

type OrgUnitMaintainFieldDecisionV1 struct {
	FieldKey             string   `json:"field_key"`
	Visible              bool     `json:"visible"`
	Required             bool     `json:"required"`
	Maintainable         bool     `json:"maintainable"`
	FieldPayloadKey      string   `json:"field_payload_key"`
	ResolvedDefaultValue string   `json:"resolved_default_value"`
	DefaultRuleRef       string   `json:"default_rule_ref"`
	AllowedValueCodes    []string `json:"allowed_value_codes"`
}

type OrgUnitMaintainPrecheckProjectionV1 struct {
	Readiness                         string                           `json:"readiness"`
	MissingFields                     []string                         `json:"missing_fields"`
	FieldDecisions                    []OrgUnitMaintainFieldDecisionV1 `json:"field_decisions"`
	CandidateConfirmationRequirements []string                         `json:"candidate_confirmation_requirements"`
	PendingDraftSummary               string                           `json:"pending_draft_summary"`
	PolicyExplain                     string                           `json:"policy_explain"`
	RejectionReasons                  []string                         `json:"rejection_reasons"`
	ProjectionDigest                  string                           `json:"projection_digest"`
}

type OrgUnitMaintainPrecheckInputV1 struct {
	Intent                            string
	TenantID                          string
	EffectiveDate                     string
	TargetEffectiveDate               string
	OrgCode                           string
	CanAdmin                          bool
	CandidateConfirmationRequired     bool
	CandidateConfirmationRequirements []string
	NewName                           string
	NewParentOrgCode                  string
	NewParentRequested                bool
}

type OrgUnitMaintainPrecheckResultV1 struct {
	PolicyContext OrgUnitMaintainPolicyContextV1       `json:"policy_context"`
	Projection    OrgUnitMaintainPrecheckProjectionV1  `json:"projection"`
	ContextError  *OrgUnitMaintainPolicyContextErrorV1 `json:"-"`
}

type OrgUnitMaintainPolicyContextErrorV1 struct {
	Code  string
	Cause error
}

func (e *OrgUnitMaintainPolicyContextErrorV1) Error() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.Code)
}

func (e *OrgUnitMaintainPolicyContextErrorV1) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

type OrgUnitMaintainTargetEventV1 struct {
	HasEffective       bool
	EffectiveEventType types.OrgUnitEventType
	HasRaw             bool
	RawEventType       types.OrgUnitEventType
}

type OrgUnitMaintainPrecheckReader interface {
	ResolveOrgNodeKey(ctx context.Context, tenantID string, orgCode string) (string, error)
	IsOrgTreeInitialized(ctx context.Context, tenantID string) (bool, error)
	ListEnabledTenantFieldConfigsAsOf(ctx context.Context, tenantID string, asOf string) ([]types.TenantFieldConfig, error)
	ResolveTargetExistsAsOf(ctx context.Context, tenantID string, orgNodeKey string, asOf string) (bool, error)
	ResolveMutationTargetEvent(ctx context.Context, tenantID string, orgNodeKey string, effectiveDate string) (OrgUnitMaintainTargetEventV1, error)
}

type orgUnitMaintainPrecheckEvaluation struct {
	Result           OrgUnitMaintainPrecheckResultV1
	TargetEvent      OrgUnitMaintainTargetEventV1
	MutationDecision OrgUnitMutationPolicyDecision
	EnabledFieldCfg  []types.TenantFieldConfig
	NameDecision     orgUnitFieldDecision
	NameFound        bool
	ParentDecision   orgUnitFieldDecision
	ParentFound      bool
}

func BuildOrgUnitMaintainPrecheckProjectionV1(
	ctx context.Context,
	reader OrgUnitMaintainPrecheckReader,
	input OrgUnitMaintainPrecheckInputV1,
) (OrgUnitMaintainPrecheckResultV1, error) {
	eval, err := evaluateOrgUnitMaintainPrecheckV1(ctx, reader, input)
	if err != nil {
		return OrgUnitMaintainPrecheckResultV1{}, err
	}
	return eval.Result, nil
}

func evaluateOrgUnitMaintainPrecheckV1(
	ctx context.Context,
	reader OrgUnitMaintainPrecheckReader,
	input OrgUnitMaintainPrecheckInputV1,
) (orgUnitMaintainPrecheckEvaluation, error) {
	normalizedInput := normalizeOrgUnitMaintainPrecheckInput(input)
	result := OrgUnitMaintainPrecheckResultV1{
		PolicyContext: OrgUnitMaintainPolicyContextV1{
			TenantID:            normalizedInput.TenantID,
			Intent:              normalizedInput.Intent,
			EffectiveDate:       normalizedInput.EffectiveDate,
			TargetEffectiveDate: normalizedInput.TargetEffectiveDate,
			OrgCode:             normalizedInput.OrgCode,
		},
		Projection: OrgUnitMaintainPrecheckProjectionV1{
			CandidateConfirmationRequirements: normalizeOrgUnitMaintainCandidateRequirements(normalizedInput.CandidateConfirmationRequired, normalizedInput.CandidateConfirmationRequirements),
			MissingFields:                     []string{},
			FieldDecisions:                    []OrgUnitMaintainFieldDecisionV1{},
			RejectionReasons:                  []string{},
		},
	}

	fieldCfgs, enabledExtFieldKeys, err := resolveOrgUnitMaintainEnabledFieldConfigs(ctx, reader, normalizedInput)
	if err != nil {
		return orgUnitMaintainPrecheckEvaluation{}, err
	}
	eval := orgUnitMaintainPrecheckEvaluation{
		Result:          result,
		EnabledFieldCfg: fieldCfgs,
	}

	policyAsOf := orgUnitMaintainPolicyAsOf(normalizedInput)
	if strings.TrimSpace(normalizedInput.OrgCode) != "" {
		policyContext, contextErr := resolveOrgUnitMaintainPolicyContextV1(ctx, reader, normalizedInput, policyAsOf)
		eval.Result.PolicyContext = policyContext
		eval.Result.ContextError = contextErr
		if contextErr != nil {
			eval.Result.Projection.RejectionReasons = append(eval.Result.Projection.RejectionReasons, strings.TrimSpace(contextErr.Code))
		}
	} else {
		eval.Result.PolicyContext.PolicyContextDigest = buildOrgUnitMaintainPolicyContextDigest(eval.Result.PolicyContext)
	}

	treeInitialized, err := resolveOrgUnitMaintainTreeInitialized(ctx, reader, normalizedInput.TenantID)
	if err != nil {
		return orgUnitMaintainPrecheckEvaluation{}, err
	}
	targetExistsAsOf, err := resolveOrgUnitMaintainTargetExistsAsOf(ctx, reader, normalizedInput, eval.Result.PolicyContext.OrgNodeKey)
	if err != nil {
		return orgUnitMaintainPrecheckEvaluation{}, err
	}

	if eval.Result.ContextError == nil && strings.TrimSpace(normalizedInput.OrgCode) != "" {
		nameDecision, nameFound, nameErr := resolveOrgUnitMaintainFieldDecision(ctx, reader, normalizedInput, eval.Result.PolicyContext.OrgNodeKey, "name", policyAsOf)
		if nameErr != "" {
			eval.Result.Projection.RejectionReasons = append(eval.Result.Projection.RejectionReasons, nameErr)
		}
		eval.NameDecision = nameDecision
		eval.NameFound = nameFound

		parentDecision, parentFound, parentErr := resolveOrgUnitMaintainFieldDecision(ctx, reader, normalizedInput, eval.Result.PolicyContext.OrgNodeKey, "parent_org_code", policyAsOf)
		if parentErr != "" {
			eval.Result.Projection.RejectionReasons = append(eval.Result.Projection.RejectionReasons, parentErr)
		}
		eval.ParentDecision = parentDecision
		eval.ParentFound = parentFound
	}

	mutationDecision, targetEvent, err := resolveOrgUnitMaintainMutationDecision(ctx, reader, normalizedInput, eval.Result.PolicyContext.OrgNodeKey, treeInitialized, targetExistsAsOf, enabledExtFieldKeys)
	if err != nil {
		return orgUnitMaintainPrecheckEvaluation{}, err
	}
	eval.MutationDecision = mutationDecision
	eval.TargetEvent = targetEvent
	if len(mutationDecision.DenyReasons) > 0 {
		eval.Result.Projection.RejectionReasons = append(eval.Result.Projection.RejectionReasons, mutationDecision.DenyReasons...)
	}

	eval.Result.Projection.MissingFields = normalizeOrgUnitMaintainMissingFields(normalizedInput, eval)
	eval.Result.Projection.RejectionReasons = normalizeOrgUnitMaintainRejectionReasons(normalizeOrgUnitMaintainFieldRejectionReasons(normalizedInput, eval))
	eval.Result.Projection.FieldDecisions = buildOrgUnitMaintainFieldDecisions(normalizedInput, eval, enabledExtFieldKeys)
	eval.Result.Projection.Readiness = resolveOrgUnitMaintainReadiness(eval.Result.Projection)
	eval.Result.Projection.PendingDraftSummary = buildOrgUnitMaintainPendingDraftSummary(normalizedInput)
	eval.Result.Projection.PolicyExplain = buildOrgUnitMaintainPolicyExplain(eval.Result.Projection)
	eval.Result.PolicyContext.PolicyContextDigest = buildOrgUnitMaintainPolicyContextDigest(eval.Result.PolicyContext)
	eval.Result.Projection.ProjectionDigest = buildOrgUnitMaintainProjectionDigest(eval.Result.Projection)
	return eval, nil
}

func normalizeOrgUnitMaintainPrecheckInput(input OrgUnitMaintainPrecheckInputV1) OrgUnitMaintainPrecheckInputV1 {
	normalized := input
	normalized.Intent = strings.ToLower(strings.TrimSpace(input.Intent))
	normalized.TenantID = strings.TrimSpace(input.TenantID)
	normalized.EffectiveDate = strings.TrimSpace(input.EffectiveDate)
	normalized.TargetEffectiveDate = strings.TrimSpace(input.TargetEffectiveDate)
	normalized.OrgCode = strings.TrimSpace(input.OrgCode)
	normalized.NewName = strings.TrimSpace(input.NewName)
	normalized.NewParentOrgCode = strings.TrimSpace(input.NewParentOrgCode)
	normalized.CandidateConfirmationRequirements = append([]string(nil), input.CandidateConfirmationRequirements...)
	normalized.NewParentRequested = input.NewParentRequested || normalized.NewParentOrgCode != ""
	return normalized
}

func orgUnitMaintainPolicyAsOf(input OrgUnitMaintainPrecheckInputV1) string {
	if strings.TrimSpace(input.Intent) == OrgUnitMaintainIntentCorrect {
		return strings.TrimSpace(input.TargetEffectiveDate)
	}
	return strings.TrimSpace(input.EffectiveDate)
}

func resolveOrgUnitMaintainEnabledFieldConfigs(
	ctx context.Context,
	reader OrgUnitMaintainPrecheckReader,
	input OrgUnitMaintainPrecheckInputV1,
) ([]types.TenantFieldConfig, []string, error) {
	if reader == nil {
		return nil, nil, nil
	}
	asOf := orgUnitMaintainPolicyAsOf(input)
	cfgs, err := reader.ListEnabledTenantFieldConfigsAsOf(ctx, input.TenantID, asOf)
	if err != nil {
		return nil, nil, err
	}
	out := make([]types.TenantFieldConfig, 0, len(cfgs))
	extKeys := make([]string, 0, len(cfgs))
	for _, cfg := range cfgs {
		fieldKey := strings.TrimSpace(cfg.FieldKey)
		if fieldKey == "" {
			continue
		}
		out = append(out, cfg)
		extKeys = append(extKeys, fieldKey)
	}
	sort.Strings(extKeys)
	return out, normalizeFieldKeys(extKeys), nil
}

func resolveOrgUnitMaintainPolicyContextV1(
	ctx context.Context,
	reader OrgUnitMaintainPrecheckReader,
	input OrgUnitMaintainPrecheckInputV1,
	asOf string,
) (OrgUnitMaintainPolicyContextV1, *OrgUnitMaintainPolicyContextErrorV1) {
	ctxV1 := OrgUnitMaintainPolicyContextV1{
		TenantID:            strings.TrimSpace(input.TenantID),
		Intent:              strings.TrimSpace(input.Intent),
		EffectiveDate:       strings.TrimSpace(input.EffectiveDate),
		TargetEffectiveDate: strings.TrimSpace(input.TargetEffectiveDate),
		OrgCode:             strings.TrimSpace(input.OrgCode),
	}
	if reader == nil {
		ctxV1.PolicyContextDigest = buildOrgUnitMaintainPolicyContextDigest(ctxV1)
		return ctxV1, nil
	}
	orgCode := strings.TrimSpace(input.OrgCode)
	if orgCode == "" {
		ctxV1.PolicyContextDigest = buildOrgUnitMaintainPolicyContextDigest(ctxV1)
		return ctxV1, nil
	}
	orgNodeKey, err := reader.ResolveOrgNodeKey(ctx, input.TenantID, orgCode)
	if err != nil {
		return ctxV1, &OrgUnitMaintainPolicyContextErrorV1{Code: orgUnitMaintainContextCodeOrgInvalid, Cause: err}
	}
	ctxV1.OrgNodeKey = strings.TrimSpace(orgNodeKey)
	ctxV1.PolicyContextDigest = buildOrgUnitMaintainPolicyContextDigest(ctxV1)
	return ctxV1, nil
}

func resolveOrgUnitMaintainTreeInitialized(ctx context.Context, reader OrgUnitMaintainPrecheckReader, tenantID string) (bool, error) {
	if reader == nil {
		return false, nil
	}
	return reader.IsOrgTreeInitialized(ctx, tenantID)
}

func resolveOrgUnitMaintainTargetExistsAsOf(
	ctx context.Context,
	reader OrgUnitMaintainPrecheckReader,
	input OrgUnitMaintainPrecheckInputV1,
	orgNodeKey string,
) (bool, error) {
	if reader == nil {
		return false, nil
	}
	if strings.TrimSpace(orgNodeKey) == "" || strings.TrimSpace(input.OrgCode) == "" {
		return false, nil
	}
	switch strings.TrimSpace(input.Intent) {
	case OrgUnitMaintainIntentRename, OrgUnitMaintainIntentMove, OrgUnitMaintainIntentDisable, OrgUnitMaintainIntentEnable:
		return reader.ResolveTargetExistsAsOf(ctx, input.TenantID, orgNodeKey, strings.TrimSpace(input.EffectiveDate))
	case OrgUnitMaintainIntentCorrect:
		return true, nil
	default:
		return false, nil
	}
}

func resolveOrgUnitMaintainFieldDecision(
	ctx context.Context,
	reader OrgUnitMaintainPrecheckReader,
	input OrgUnitMaintainPrecheckInputV1,
	orgNodeKey string,
	fieldKey string,
	asOf string,
) (orgUnitFieldDecision, bool, string) {
	_ = ctx
	_ = reader
	_ = input
	_ = orgNodeKey
	_ = asOf
	return resolveOrgUnitWriteFieldDecision(fieldKey)
}

func normalizeOrgUnitMaintainCandidateRequirements(required bool, requirements []string) []string {
	if !required && len(requirements) == 0 {
		return []string{}
	}
	if len(requirements) == 0 {
		requirements = []string{orgUnitMaintainCandidateRequirementNewParent}
	}
	out := make([]string, 0, len(requirements))
	seen := make(map[string]struct{}, len(requirements))
	for _, item := range requirements {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func resolveOrgUnitMaintainMutationDecision(
	ctx context.Context,
	reader OrgUnitMaintainPrecheckReader,
	input OrgUnitMaintainPrecheckInputV1,
	orgNodeKey string,
	treeInitialized bool,
	targetExistsAsOf bool,
	enabledExtFieldKeys []string,
) (OrgUnitMutationPolicyDecision, OrgUnitMaintainTargetEventV1, error) {
	intent := strings.TrimSpace(input.Intent)
	switch intent {
	case OrgUnitMaintainIntentDisable:
		decision, err := ResolvePolicy(OrgUnitMutationPolicyKey{
			ActionKind:       OrgUnitActionEventUpdate,
			EmittedEventType: OrgUnitEmittedDisable,
		}, OrgUnitMutationPolicyFacts{
			CanAdmin:            input.CanAdmin,
			TreeInitialized:     treeInitialized,
			TargetExistsAsOf:    targetExistsAsOf,
			EnabledExtFieldKeys: enabledExtFieldKeys,
		})
		return decision, OrgUnitMaintainTargetEventV1{}, err
	case OrgUnitMaintainIntentEnable:
		decision, err := ResolvePolicy(OrgUnitMutationPolicyKey{
			ActionKind:       OrgUnitActionEventUpdate,
			EmittedEventType: OrgUnitEmittedEnable,
		}, OrgUnitMutationPolicyFacts{
			CanAdmin:            input.CanAdmin,
			TreeInitialized:     treeInitialized,
			TargetExistsAsOf:    targetExistsAsOf,
			EnabledExtFieldKeys: enabledExtFieldKeys,
		})
		return decision, OrgUnitMaintainTargetEventV1{}, err
	case OrgUnitMaintainIntentRename:
		decision, err := ResolvePolicy(OrgUnitMutationPolicyKey{
			ActionKind:       OrgUnitActionEventUpdate,
			EmittedEventType: OrgUnitEmittedRename,
		}, OrgUnitMutationPolicyFacts{
			CanAdmin:            input.CanAdmin,
			TreeInitialized:     treeInitialized,
			TargetExistsAsOf:    targetExistsAsOf,
			EnabledExtFieldKeys: enabledExtFieldKeys,
		})
		return decision, OrgUnitMaintainTargetEventV1{}, err
	case OrgUnitMaintainIntentMove:
		decision, err := ResolvePolicy(OrgUnitMutationPolicyKey{
			ActionKind:       OrgUnitActionEventUpdate,
			EmittedEventType: OrgUnitEmittedMove,
		}, OrgUnitMutationPolicyFacts{
			CanAdmin:            input.CanAdmin,
			TreeInitialized:     treeInitialized,
			TargetExistsAsOf:    targetExistsAsOf,
			IsRoot:              strings.EqualFold(strings.TrimSpace(input.OrgCode), "ROOT"),
			EnabledExtFieldKeys: enabledExtFieldKeys,
		})
		return decision, OrgUnitMaintainTargetEventV1{}, err
	case OrgUnitMaintainIntentCorrect:
		if reader == nil || strings.TrimSpace(orgNodeKey) == "" || strings.TrimSpace(input.TargetEffectiveDate) == "" {
			decision, err := ResolveWriteCapabilities(OrgUnitWriteIntentCorrect, enabledExtFieldKeys, OrgUnitWriteCapabilitiesFacts{
				CanAdmin:            input.CanAdmin,
				TreeInitialized:     treeInitialized,
				TargetExistsAsOf:    targetExistsAsOf,
				OrgCode:             strings.TrimSpace(input.OrgCode),
				TargetEffectiveDate: strings.TrimSpace(input.TargetEffectiveDate),
			})
			return OrgUnitMutationPolicyDecision{
				Enabled:          false,
				AllowedFields:    decision.AllowedFields,
				FieldPayloadKeys: decision.FieldPayloadKeys,
				DenyReasons:      decision.DenyReasons,
			}, OrgUnitMaintainTargetEventV1{}, err
		}
		targetEvent, err := reader.ResolveMutationTargetEvent(ctx, input.TenantID, orgNodeKey, strings.TrimSpace(input.TargetEffectiveDate))
		if err != nil {
			return OrgUnitMutationPolicyDecision{}, OrgUnitMaintainTargetEventV1{}, err
		}
		writeDecision, err := ResolveWriteCapabilities(OrgUnitWriteIntentCorrect, enabledExtFieldKeys, OrgUnitWriteCapabilitiesFacts{
			CanAdmin:             input.CanAdmin,
			TreeInitialized:      treeInitialized,
			TargetExistsAsOf:     targetExistsAsOf,
			TargetEventNotFound:  !targetEvent.HasEffective && !targetEvent.HasRaw,
			TargetEventRescinded: !targetEvent.HasEffective && targetEvent.HasRaw,
			OrgCode:              strings.TrimSpace(input.OrgCode),
			TargetEffectiveDate:  strings.TrimSpace(input.TargetEffectiveDate),
		})
		if err != nil {
			return OrgUnitMutationPolicyDecision{}, OrgUnitMaintainTargetEventV1{}, err
		}
		targetEventType := targetEvent.EffectiveEventType
		if !targetEvent.HasEffective && targetEvent.HasRaw {
			targetEventType = targetEvent.RawEventType
		}
		if strings.TrimSpace(string(targetEventType)) == "" {
			return OrgUnitMutationPolicyDecision{
				Enabled:          false,
				AllowedFields:    writeDecision.AllowedFields,
				FieldPayloadKeys: writeDecision.FieldPayloadKeys,
				DenyReasons:      writeDecision.DenyReasons,
			}, targetEvent, nil
		}
		mutationDecision, err := ResolvePolicy(OrgUnitMutationPolicyKey{
			ActionKind:               OrgUnitActionCorrectEvent,
			EmittedEventType:         OrgUnitEmittedCorrectEvent,
			TargetEffectiveEventType: &targetEventType,
		}, OrgUnitMutationPolicyFacts{
			CanAdmin:            input.CanAdmin,
			EnabledExtFieldKeys: enabledExtFieldKeys,
		})
		if err != nil {
			return OrgUnitMutationPolicyDecision{}, OrgUnitMaintainTargetEventV1{}, err
		}
		mutationDecision.DenyReasons = dedupAndSortDenyReasons(append(writeDecision.DenyReasons, mutationDecision.DenyReasons...))
		if len(mutationDecision.DenyReasons) > 0 {
			mutationDecision.Enabled = false
		}
		return mutationDecision, targetEvent, nil
	default:
		return OrgUnitMutationPolicyDecision{}, OrgUnitMaintainTargetEventV1{}, nil
	}
}

func normalizeOrgUnitMaintainMissingFields(
	input OrgUnitMaintainPrecheckInputV1,
	eval orgUnitMaintainPrecheckEvaluation,
) []string {
	candidates := make([]string, 0, 4)
	if strings.TrimSpace(input.OrgCode) == "" {
		candidates = append(candidates, "org_code")
	}
	switch strings.TrimSpace(input.Intent) {
	case OrgUnitMaintainIntentCorrect:
		if strings.TrimSpace(input.TargetEffectiveDate) == "" {
			candidates = append(candidates, "target_effective_date")
		}
		if strings.TrimSpace(input.NewName) == "" && !input.NewParentRequested {
			candidates = append(candidates, "change_fields")
		}
	case OrgUnitMaintainIntentRename, OrgUnitMaintainIntentMove, OrgUnitMaintainIntentDisable, OrgUnitMaintainIntentEnable:
		if strings.TrimSpace(input.EffectiveDate) == "" {
			candidates = append(candidates, "effective_date")
		}
	}
	if eval.NameFound && eval.NameDecision.Required && strings.TrimSpace(input.NewName) == "" {
		candidates = append(candidates, "new_name")
	}
	if eval.ParentFound && eval.ParentDecision.Required && !input.NewParentRequested {
		candidates = append(candidates, "new_parent_ref_text")
	}
	if strings.TrimSpace(input.Intent) == OrgUnitMaintainIntentRename && strings.TrimSpace(input.NewName) == "" {
		candidates = append(candidates, "new_name")
	}
	if strings.TrimSpace(input.Intent) == OrgUnitMaintainIntentMove && !input.NewParentRequested {
		candidates = append(candidates, "new_parent_ref_text")
	}
	seen := make(map[string]struct{}, len(candidates))
	out := make([]string, 0, len(candidates))
	for _, key := range candidates {
		value := strings.TrimSpace(key)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func normalizeOrgUnitMaintainFieldRejectionReasons(
	input OrgUnitMaintainPrecheckInputV1,
	eval orgUnitMaintainPrecheckEvaluation,
) []string {
	reasons := append([]string(nil), eval.Result.Projection.RejectionReasons...)
	appendReason := func(code string) {
		if strings.TrimSpace(code) != "" {
			reasons = append(reasons, strings.TrimSpace(code))
		}
	}
	namePayloadKey := strings.TrimSpace(eval.MutationDecision.FieldPayloadKeys["name"])
	if strings.TrimSpace(input.NewName) != "" {
		if namePayloadKey == "" {
			appendReason(errPatchFieldNotAllowed)
		} else if eval.NameFound && !eval.NameDecision.Maintainable {
			appendReason(errPatchFieldNotAllowed)
		}
	}
	parentPayloadKey := strings.TrimSpace(eval.MutationDecision.FieldPayloadKeys["parent_org_code"])
	if input.NewParentRequested {
		if parentPayloadKey == "" {
			appendReason(errPatchFieldNotAllowed)
		} else if eval.ParentFound && !eval.ParentDecision.Maintainable {
			appendReason(errPatchFieldNotAllowed)
		}
	}
	if strings.TrimSpace(input.NewName) != "" && eval.NameFound {
		if allowErr := validateFieldOptionAllowed(input.NewName, eval.NameDecision.AllowedValueCodes); allowErr != nil {
			appendReason(allowErr.Error())
		}
	}
	if strings.TrimSpace(input.NewParentOrgCode) != "" && eval.ParentFound {
		if allowErr := validateFieldOptionAllowed(input.NewParentOrgCode, eval.ParentDecision.AllowedValueCodes); allowErr != nil {
			appendReason(allowErr.Error())
		}
	}
	return reasons
}

func normalizeOrgUnitMaintainRejectionReasons(reasons []string) []string {
	if len(reasons) == 0 {
		return []string{}
	}
	contextReasons := []string{}
	mutationReasons := []string{}
	fieldReasons := []string{}
	seen := map[string]struct{}{}
	appendReason := func(target *[]string, code string) {
		code = strings.TrimSpace(code)
		if code == "" {
			return
		}
		if _, ok := seen[code]; ok {
			return
		}
		seen[code] = struct{}{}
		*target = append(*target, code)
	}
	for _, reason := range reasons {
		switch strings.TrimSpace(reason) {
		case orgUnitMaintainContextCodeOrgInvalid:
			appendReason(&contextReasons, reason)
		case "FORBIDDEN", "ORG_TREE_NOT_INITIALIZED", "ORG_NOT_FOUND_AS_OF", "ORG_EVENT_NOT_FOUND", "ORG_EVENT_RESCINDED", "ORG_ROOT_CANNOT_BE_MOVED":
			appendReason(&mutationReasons, reason)
		default:
			appendReason(&fieldReasons, reason)
		}
	}
	out := make([]string, 0, len(contextReasons)+len(mutationReasons)+len(fieldReasons))
	out = append(out, contextReasons...)
	out = append(out, mutationReasons...)
	out = append(out, fieldReasons...)
	return out
}

func resolveOrgUnitMaintainReadiness(projection OrgUnitMaintainPrecheckProjectionV1) string {
	if len(projection.RejectionReasons) > 0 {
		return orgUnitMaintainReadinessRejected
	}
	if len(projection.CandidateConfirmationRequirements) > 0 {
		return orgUnitMaintainReadinessCandidateConfirmation
	}
	if len(projection.MissingFields) > 0 {
		return orgUnitMaintainReadinessMissingFields
	}
	return orgUnitMaintainReadinessReady
}

func buildOrgUnitMaintainFieldDecisions(
	input OrgUnitMaintainPrecheckInputV1,
	eval orgUnitMaintainPrecheckEvaluation,
	enabledExtFieldKeys []string,
) []OrgUnitMaintainFieldDecisionV1 {
	order := orgUnitMaintainFieldDecisionOrder(strings.TrimSpace(input.Intent))
	decisions := make([]OrgUnitMaintainFieldDecisionV1, 0, len(order)+len(enabledExtFieldKeys))
	payloadKeys := eval.MutationDecision.FieldPayloadKeys
	for _, fieldKey := range order {
		switch fieldKey {
		case "org_code", "effective_date", "target_effective_date":
			decisions = append(decisions, OrgUnitMaintainFieldDecisionV1{
				FieldKey:          fieldKey,
				Visible:           true,
				Required:          true,
				Maintainable:      false,
				FieldPayloadKey:   "",
				AllowedValueCodes: []string{},
			})
		case "name":
			decisions = append(decisions, buildOrgUnitMaintainPDPFieldDecision(fieldKey, eval.NameDecision, eval.NameFound, payloadKeys[fieldKey]))
		case "parent_org_code":
			decisions = append(decisions, buildOrgUnitMaintainPDPFieldDecision(fieldKey, eval.ParentDecision, eval.ParentFound, payloadKeys[fieldKey]))
		}
	}
	for _, fieldKey := range enabledExtFieldKeys {
		decisions = append(decisions, OrgUnitMaintainFieldDecisionV1{
			FieldKey:          fieldKey,
			Visible:           true,
			Required:          false,
			Maintainable:      strings.TrimSpace(payloadKeys[fieldKey]) != "",
			FieldPayloadKey:   strings.TrimSpace(payloadKeys[fieldKey]),
			AllowedValueCodes: []string{},
		})
	}
	return decisions
}

func orgUnitMaintainFieldDecisionOrder(intent string) []string {
	switch strings.TrimSpace(intent) {
	case OrgUnitMaintainIntentCorrect:
		return []string{"org_code", "target_effective_date", "name", "parent_org_code"}
	case OrgUnitMaintainIntentMove:
		return []string{"org_code", "effective_date", "parent_org_code"}
	case OrgUnitMaintainIntentDisable, OrgUnitMaintainIntentEnable:
		return []string{"org_code", "effective_date"}
	case OrgUnitMaintainIntentRename:
		return []string{"org_code", "effective_date", "name"}
	default:
		return []string{"org_code"}
	}
}

func buildOrgUnitMaintainPDPFieldDecision(
	fieldKey string,
	decision orgUnitFieldDecision,
	found bool,
	payloadKey string,
) OrgUnitMaintainFieldDecisionV1 {
	if !found {
		return OrgUnitMaintainFieldDecisionV1{
			FieldKey:          fieldKey,
			Visible:           strings.TrimSpace(payloadKey) != "",
			Required:          false,
			Maintainable:      strings.TrimSpace(payloadKey) != "",
			FieldPayloadKey:   strings.TrimSpace(payloadKey),
			AllowedValueCodes: []string{},
		}
	}
	return OrgUnitMaintainFieldDecisionV1{
		FieldKey:             fieldKey,
		Visible:              decision.Visible,
		Required:             decision.Required,
		Maintainable:         decision.Maintainable && strings.TrimSpace(payloadKey) != "",
		FieldPayloadKey:      strings.TrimSpace(payloadKey),
		ResolvedDefaultValue: strings.TrimSpace(decision.DefaultValue),
		DefaultRuleRef:       strings.TrimSpace(decision.DefaultRuleRef),
		AllowedValueCodes:    normalizeCreateOrgUnitAllowedValueCodes(append([]string(nil), decision.AllowedValueCodes...)),
	}
}

func buildOrgUnitMaintainPendingDraftSummary(input OrgUnitMaintainPrecheckInputV1) string {
	parts := make([]string, 0, 4)
	if orgCode := strings.TrimSpace(input.OrgCode); orgCode != "" {
		parts = append(parts, "目标组织："+orgCode)
	}
	switch strings.TrimSpace(input.Intent) {
	case OrgUnitMaintainIntentCorrect:
		if targetDate := strings.TrimSpace(input.TargetEffectiveDate); targetDate != "" {
			parts = append(parts, "目标版本："+targetDate)
		}
	case OrgUnitMaintainIntentDisable:
		if effectiveDate := strings.TrimSpace(input.EffectiveDate); effectiveDate != "" {
			parts = append(parts, "停用生效日期："+effectiveDate)
		}
	case OrgUnitMaintainIntentEnable:
		if effectiveDate := strings.TrimSpace(input.EffectiveDate); effectiveDate != "" {
			parts = append(parts, "启用生效日期："+effectiveDate)
		}
	default:
		if effectiveDate := strings.TrimSpace(input.EffectiveDate); effectiveDate != "" {
			parts = append(parts, "生效日期："+effectiveDate)
		}
	}
	if newName := strings.TrimSpace(input.NewName); newName != "" {
		parts = append(parts, "新名称："+newName)
	}
	if parent := strings.TrimSpace(input.NewParentOrgCode); parent != "" {
		parts = append(parts, "新上级组织："+parent)
	}
	return strings.Join(parts, "；")
}

func buildOrgUnitMaintainPolicyExplain(projection OrgUnitMaintainPrecheckProjectionV1) string {
	switch strings.TrimSpace(projection.Readiness) {
	case orgUnitMaintainReadinessReady:
		return "计划已生成，等待确认后可提交"
	case orgUnitMaintainReadinessMissingFields:
		return "仍有必填字段未补全"
	case orgUnitMaintainReadinessCandidateConfirmation:
		return "仍需确认新的上级组织"
	case orgUnitMaintainReadinessRejected:
		if len(projection.RejectionReasons) == 0 {
			return "当前草案已被策略拒绝"
		}
		return strings.Join(projection.RejectionReasons, ",")
	default:
		return ""
	}
}

func buildOrgUnitMaintainPolicyContextDigest(ctx OrgUnitMaintainPolicyContextV1) string {
	payload := struct {
		TenantID            string `json:"tenant_id"`
		Intent              string `json:"intent"`
		EffectiveDate       string `json:"effective_date,omitempty"`
		TargetEffectiveDate string `json:"target_effective_date,omitempty"`
		OrgCode             string `json:"org_code"`
		OrgNodeKey          string `json:"org_node_key"`
	}{
		TenantID:            strings.TrimSpace(ctx.TenantID),
		Intent:              strings.TrimSpace(ctx.Intent),
		EffectiveDate:       strings.TrimSpace(ctx.EffectiveDate),
		TargetEffectiveDate: strings.TrimSpace(ctx.TargetEffectiveDate),
		OrgCode:             strings.TrimSpace(ctx.OrgCode),
		OrgNodeKey:          strings.TrimSpace(ctx.OrgNodeKey),
	}
	return orgUnitMaintainDigest(payload)
}

func buildOrgUnitMaintainProjectionDigest(projection OrgUnitMaintainPrecheckProjectionV1) string {
	payload := struct {
		Readiness                         string                           `json:"readiness"`
		MissingFields                     []string                         `json:"missing_fields"`
		FieldDecisions                    []OrgUnitMaintainFieldDecisionV1 `json:"field_decisions"`
		CandidateConfirmationRequirements []string                         `json:"candidate_confirmation_requirements"`
		PendingDraftSummary               string                           `json:"pending_draft_summary"`
		PolicyExplain                     string                           `json:"policy_explain"`
		RejectionReasons                  []string                         `json:"rejection_reasons"`
	}{
		Readiness:                         strings.TrimSpace(projection.Readiness),
		MissingFields:                     append([]string(nil), projection.MissingFields...),
		FieldDecisions:                    CloneOrgUnitMaintainFieldDecisions(projection.FieldDecisions),
		CandidateConfirmationRequirements: append([]string(nil), projection.CandidateConfirmationRequirements...),
		PendingDraftSummary:               strings.TrimSpace(projection.PendingDraftSummary),
		PolicyExplain:                     strings.TrimSpace(projection.PolicyExplain),
		RejectionReasons:                  append([]string(nil), projection.RejectionReasons...),
	}
	return orgUnitMaintainDigest(payload)
}

func orgUnitMaintainDigest(payload any) string {
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func CloneOrgUnitMaintainFieldDecisions(items []OrgUnitMaintainFieldDecisionV1) []OrgUnitMaintainFieldDecisionV1 {
	if len(items) == 0 {
		return nil
	}
	out := make([]OrgUnitMaintainFieldDecisionV1, 0, len(items))
	for _, item := range items {
		copyItem := item
		copyItem.AllowedValueCodes = append([]string(nil), item.AllowedValueCodes...)
		out = append(out, copyItem)
	}
	return out
}

func CloneOrgUnitMaintainProjectionV1(projection OrgUnitMaintainPrecheckProjectionV1) OrgUnitMaintainPrecheckProjectionV1 {
	return OrgUnitMaintainPrecheckProjectionV1{
		Readiness:                         strings.TrimSpace(projection.Readiness),
		MissingFields:                     append([]string(nil), projection.MissingFields...),
		FieldDecisions:                    CloneOrgUnitMaintainFieldDecisions(projection.FieldDecisions),
		CandidateConfirmationRequirements: append([]string(nil), projection.CandidateConfirmationRequirements...),
		PendingDraftSummary:               strings.TrimSpace(projection.PendingDraftSummary),
		PolicyExplain:                     strings.TrimSpace(projection.PolicyExplain),
		RejectionReasons:                  append([]string(nil), projection.RejectionReasons...),
		ProjectionDigest:                  strings.TrimSpace(projection.ProjectionDigest),
	}
}
