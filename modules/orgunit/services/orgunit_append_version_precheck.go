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
	OrgUnitAppendVersionMutationPolicyVersionV1        = "orgunit.append_version.mutation_policy.v1"
	OrgUnitAppendVersionPolicyContextContractVersionV1 = "orgunit.append_version.policy_context.v1"
	OrgUnitAppendVersionPrecheckProjectionContractV1   = "orgunit.append_version.precheck_projection.v1"

	orgUnitAppendVersionReadinessReady                 = "ready"
	orgUnitAppendVersionReadinessMissingFields         = "missing_fields"
	orgUnitAppendVersionReadinessCandidateConfirmation = "candidate_confirmation_required"
	orgUnitAppendVersionReadinessRejected              = "rejected"

	orgUnitAppendVersionContextCodeOrgInvalid          = "org_context_invalid"
	orgUnitAppendVersionContextCodeSetIDBindingMissing = "setid_binding_missing"
	orgUnitAppendVersionContextCodeSetIDSourceInvalid  = "setid_source_invalid"

	orgUnitAppendVersionCandidateRequirementNewParent = "new_parent_org_code"
)

var orgUnitAppendVersionFieldDecisionOrder = []string{
	"org_code",
	"effective_date",
	"name",
	"parent_org_code",
}

type OrgUnitAppendVersionPolicyContextV1 struct {
	TenantID            string `json:"tenant_id"`
	CapabilityKey       string `json:"capability_key"`
	Intent              string `json:"intent"`
	EffectiveDate       string `json:"effective_date"`
	OrgCode             string `json:"org_code"`
	OrgNodeKey          string `json:"org_node_key"`
	ResolvedSetID       string `json:"resolved_setid"`
	SetIDSource         string `json:"setid_source"`
	PolicyContextDigest string `json:"policy_context_digest"`
}

type OrgUnitAppendVersionFieldDecisionV1 struct {
	FieldKey             string   `json:"field_key"`
	Visible              bool     `json:"visible"`
	Required             bool     `json:"required"`
	Maintainable         bool     `json:"maintainable"`
	FieldPayloadKey      string   `json:"field_payload_key"`
	ResolvedDefaultValue string   `json:"resolved_default_value"`
	DefaultRuleRef       string   `json:"default_rule_ref"`
	AllowedValueCodes    []string `json:"allowed_value_codes"`
}

type OrgUnitAppendVersionPrecheckProjectionV1 struct {
	Readiness                         string                                `json:"readiness"`
	MissingFields                     []string                              `json:"missing_fields"`
	FieldDecisions                    []OrgUnitAppendVersionFieldDecisionV1 `json:"field_decisions"`
	CandidateConfirmationRequirements []string                              `json:"candidate_confirmation_requirements"`
	PendingDraftSummary               string                                `json:"pending_draft_summary"`
	EffectivePolicyVersion            string                                `json:"effective_policy_version"`
	MutationPolicyVersion             string                                `json:"mutation_policy_version"`
	ResolvedSetID                     string                                `json:"resolved_setid"`
	SetIDSource                       string                                `json:"setid_source"`
	PolicyExplain                     string                                `json:"policy_explain"`
	RejectionReasons                  []string                              `json:"rejection_reasons"`
	ProjectionDigest                  string                                `json:"projection_digest"`
}

type OrgUnitAppendVersionPrecheckInputV1 struct {
	Intent                            string
	TenantID                          string
	CapabilityKey                     string
	EffectiveDate                     string
	OrgCode                           string
	EffectivePolicyVersion            string
	CanAdmin                          bool
	CandidateConfirmationRequired     bool
	CandidateConfirmationRequirements []string
	NewName                           string
	NewParentOrgCode                  string
	NewParentRequested                bool
	EnabledFieldConfigs               []types.TenantFieldConfig
}

type OrgUnitAppendVersionPrecheckResultV1 struct {
	PolicyContext OrgUnitAppendVersionPolicyContextV1       `json:"policy_context"`
	Projection    OrgUnitAppendVersionPrecheckProjectionV1  `json:"projection"`
	ContextError  *OrgUnitAppendVersionPolicyContextErrorV1 `json:"-"`
}

type OrgUnitAppendVersionPolicyContextErrorV1 struct {
	Code  string
	Cause error
}

func (e *OrgUnitAppendVersionPolicyContextErrorV1) Error() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.Code)
}

func (e *OrgUnitAppendVersionPolicyContextErrorV1) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

type OrgUnitAppendVersionFactsV1 struct {
	TargetExistsAsOf bool
}

type OrgUnitAppendVersionPrecheckReader interface {
	ResolveOrgNodeKey(ctx context.Context, tenantID string, orgCode string) (string, error)
	ResolveSetID(ctx context.Context, tenantID string, orgNodeKey string, asOf string) (string, error)
	IsOrgTreeInitialized(ctx context.Context, tenantID string) (bool, error)
	ResolveSetIDStrategyFieldDecision(
		ctx context.Context,
		tenantID string,
		capabilityKey string,
		fieldKey string,
		businessUnitNodeKey string,
		asOf string,
	) (types.SetIDStrategyFieldDecision, bool, error)
	ListEnabledTenantFieldConfigsAsOf(ctx context.Context, tenantID string, asOf string) ([]types.TenantFieldConfig, error)
	ResolveAppendFacts(ctx context.Context, tenantID string, orgNodeKey string, effectiveDate string) (OrgUnitAppendVersionFactsV1, error)
}

type orgUnitAppendVersionPrecheckEvaluation struct {
	Result           OrgUnitAppendVersionPrecheckResultV1
	MutationDecision OrgUnitWriteCapabilitiesDecision
	EnabledFieldCfg  []types.TenantFieldConfig
	NameDecision     types.SetIDStrategyFieldDecision
	NameFound        bool
	ParentDecision   types.SetIDStrategyFieldDecision
	ParentFound      bool
}

func BuildOrgUnitAppendVersionPrecheckProjectionV1(
	ctx context.Context,
	reader OrgUnitAppendVersionPrecheckReader,
	input OrgUnitAppendVersionPrecheckInputV1,
) (OrgUnitAppendVersionPrecheckResultV1, error) {
	eval, err := evaluateOrgUnitAppendVersionPrecheckV1(ctx, reader, input)
	if err != nil {
		return OrgUnitAppendVersionPrecheckResultV1{}, err
	}
	return eval.Result, nil
}

func evaluateOrgUnitAppendVersionPrecheckV1(
	ctx context.Context,
	reader OrgUnitAppendVersionPrecheckReader,
	input OrgUnitAppendVersionPrecheckInputV1,
) (orgUnitAppendVersionPrecheckEvaluation, error) {
	normalizedInput := normalizeOrgUnitAppendVersionPrecheckInput(input)
	result := OrgUnitAppendVersionPrecheckResultV1{
		PolicyContext: OrgUnitAppendVersionPolicyContextV1{
			TenantID:      normalizedInput.TenantID,
			CapabilityKey: normalizedInput.CapabilityKey,
			Intent:        normalizedInput.Intent,
			EffectiveDate: normalizedInput.EffectiveDate,
			OrgCode:       normalizedInput.OrgCode,
		},
		Projection: OrgUnitAppendVersionPrecheckProjectionV1{
			MutationPolicyVersion:             OrgUnitAppendVersionMutationPolicyVersionV1,
			EffectivePolicyVersion:            normalizedInput.EffectivePolicyVersion,
			CandidateConfirmationRequirements: normalizeOrgUnitAppendVersionCandidateRequirements(normalizedInput.CandidateConfirmationRequired, normalizedInput.CandidateConfirmationRequirements),
			MissingFields:                     []string{},
			FieldDecisions:                    []OrgUnitAppendVersionFieldDecisionV1{},
			RejectionReasons:                  []string{},
		},
	}

	fieldCfgs, enabledExtFieldKeys, err := resolveOrgUnitAppendVersionEnabledFieldConfigs(ctx, reader, normalizedInput)
	if err != nil {
		return orgUnitAppendVersionPrecheckEvaluation{}, err
	}
	eval := orgUnitAppendVersionPrecheckEvaluation{
		Result:          result,
		EnabledFieldCfg: fieldCfgs,
	}

	if strings.TrimSpace(normalizedInput.OrgCode) != "" {
		policyContext, contextErr := resolveOrgUnitAppendVersionPolicyContextV1(ctx, reader, normalizedInput)
		eval.Result.PolicyContext = policyContext
		eval.Result.ContextError = contextErr
		if contextErr != nil {
			eval.Result.Projection.RejectionReasons = append(eval.Result.Projection.RejectionReasons, strings.TrimSpace(contextErr.Code))
		}
	} else {
		eval.Result.PolicyContext.PolicyContextDigest = buildOrgUnitAppendVersionPolicyContextDigest(eval.Result.PolicyContext)
	}

	treeInitialized, err := resolveOrgUnitAppendVersionTreeInitialized(ctx, reader, normalizedInput.TenantID)
	if err != nil {
		return orgUnitAppendVersionPrecheckEvaluation{}, err
	}
	targetExistsAsOf, err := resolveOrgUnitAppendVersionTargetExistsAsOf(ctx, reader, normalizedInput, eval.Result.PolicyContext.OrgNodeKey)
	if err != nil {
		return orgUnitAppendVersionPrecheckEvaluation{}, err
	}

	if strings.TrimSpace(normalizedInput.OrgCode) != "" {
		mutationDecision, err := ResolveWriteCapabilities(
			OrgUnitWriteIntent(normalizedInput.Intent),
			enabledExtFieldKeys,
			OrgUnitWriteCapabilitiesFacts{
				CanAdmin:         normalizedInput.CanAdmin,
				TreeInitialized:  treeInitialized,
				TargetExistsAsOf: targetExistsAsOf,
				OrgCode:          normalizedInput.OrgCode,
				EffectiveDate:    normalizedInput.EffectiveDate,
			},
		)
		if err != nil {
			return orgUnitAppendVersionPrecheckEvaluation{}, err
		}
		eval.MutationDecision = mutationDecision
		if len(mutationDecision.DenyReasons) > 0 {
			eval.Result.Projection.RejectionReasons = append(eval.Result.Projection.RejectionReasons, mutationDecision.DenyReasons...)
		}
	}

	if eval.Result.ContextError == nil && strings.TrimSpace(normalizedInput.OrgCode) != "" {
		nameDecision, nameFound, nameErr := resolveOrgUnitAppendVersionFieldDecision(ctx, reader, normalizedInput, eval.Result.PolicyContext.OrgNodeKey, "name")
		if nameErr != "" {
			eval.Result.Projection.RejectionReasons = append(eval.Result.Projection.RejectionReasons, nameErr)
		}
		eval.NameDecision = nameDecision
		eval.NameFound = nameFound
		parentDecision, parentFound, parentErr := resolveOrgUnitAppendVersionFieldDecision(ctx, reader, normalizedInput, eval.Result.PolicyContext.OrgNodeKey, "parent_org_code")
		if parentErr != "" {
			eval.Result.Projection.RejectionReasons = append(eval.Result.Projection.RejectionReasons, parentErr)
		}
		eval.ParentDecision = parentDecision
		eval.ParentFound = parentFound
	}

	eval.Result.Projection.MissingFields = normalizeOrgUnitAppendVersionMissingFields(normalizedInput, eval)
	eval.Result.Projection.RejectionReasons = normalizeOrgUnitAppendVersionRejectionReasons(normalizeOrgUnitAppendVersionFieldRejectionReasons(normalizedInput, eval))
	eval.Result.Projection.FieldDecisions = buildOrgUnitAppendVersionFieldDecisions(eval, enabledExtFieldKeys)
	eval.Result.Projection.Readiness = resolveOrgUnitAppendVersionReadiness(eval.Result.Projection)
	eval.Result.Projection.PendingDraftSummary = buildOrgUnitAppendVersionPendingDraftSummary(normalizedInput)
	eval.Result.Projection.ResolvedSetID = eval.Result.PolicyContext.ResolvedSetID
	eval.Result.Projection.SetIDSource = eval.Result.PolicyContext.SetIDSource
	eval.Result.Projection.PolicyExplain = buildOrgUnitAppendVersionPolicyExplain(eval.Result.Projection)
	eval.Result.PolicyContext.PolicyContextDigest = buildOrgUnitAppendVersionPolicyContextDigest(eval.Result.PolicyContext)
	eval.Result.Projection.ProjectionDigest = buildOrgUnitAppendVersionProjectionDigest(eval.Result.Projection)
	return eval, nil
}

func normalizeOrgUnitAppendVersionPrecheckInput(input OrgUnitAppendVersionPrecheckInputV1) OrgUnitAppendVersionPrecheckInputV1 {
	input.Intent = strings.TrimSpace(input.Intent)
	input.TenantID = strings.TrimSpace(input.TenantID)
	input.CapabilityKey = strings.ToLower(strings.TrimSpace(input.CapabilityKey))
	input.EffectiveDate = strings.TrimSpace(input.EffectiveDate)
	input.OrgCode = strings.TrimSpace(input.OrgCode)
	input.EffectivePolicyVersion = strings.TrimSpace(input.EffectivePolicyVersion)
	input.NewName = strings.TrimSpace(input.NewName)
	input.NewParentOrgCode = strings.TrimSpace(input.NewParentOrgCode)
	if input.NewParentOrgCode != "" {
		input.NewParentRequested = true
	}
	return input
}

func resolveOrgUnitAppendVersionEnabledFieldConfigs(
	ctx context.Context,
	reader OrgUnitAppendVersionPrecheckReader,
	input OrgUnitAppendVersionPrecheckInputV1,
) ([]types.TenantFieldConfig, []string, error) {
	if len(input.EnabledFieldConfigs) > 0 {
		return filterEnabledOrgUnitExtFieldConfigs(input.EnabledFieldConfigs)
	}
	if reader == nil {
		return nil, nil, nil
	}
	cfgs, err := reader.ListEnabledTenantFieldConfigsAsOf(ctx, input.TenantID, input.EffectiveDate)
	if err != nil {
		return nil, nil, err
	}
	return filterEnabledOrgUnitExtFieldConfigs(cfgs)
}

func resolveOrgUnitAppendVersionPolicyContextV1(
	ctx context.Context,
	reader OrgUnitAppendVersionPrecheckReader,
	input OrgUnitAppendVersionPrecheckInputV1,
) (OrgUnitAppendVersionPolicyContextV1, *OrgUnitAppendVersionPolicyContextErrorV1) {
	ctxV1 := OrgUnitAppendVersionPolicyContextV1{
		TenantID:      input.TenantID,
		CapabilityKey: input.CapabilityKey,
		Intent:        input.Intent,
		EffectiveDate: input.EffectiveDate,
		OrgCode:       input.OrgCode,
	}
	if strings.TrimSpace(input.OrgCode) == "" {
		ctxV1.PolicyContextDigest = buildOrgUnitAppendVersionPolicyContextDigest(ctxV1)
		return ctxV1, nil
	}
	if reader == nil {
		err := &OrgUnitAppendVersionPolicyContextErrorV1{Code: orgUnitAppendVersionContextCodeOrgInvalid}
		ctxV1.PolicyContextDigest = buildOrgUnitAppendVersionPolicyContextDigest(ctxV1)
		return ctxV1, err
	}
	orgCode, err := normalizeOrgCode(input.OrgCode)
	if err != nil {
		contextErr := &OrgUnitAppendVersionPolicyContextErrorV1{
			Code:  orgUnitAppendVersionContextCodeOrgInvalid,
			Cause: err,
		}
		ctxV1.PolicyContextDigest = buildOrgUnitAppendVersionPolicyContextDigest(ctxV1)
		return ctxV1, contextErr
	}
	ctxV1.OrgCode = orgCode
	orgNodeKey, err := reader.ResolveOrgNodeKey(ctx, input.TenantID, orgCode)
	if err != nil {
		contextErr := &OrgUnitAppendVersionPolicyContextErrorV1{
			Code:  orgUnitAppendVersionContextCodeOrgInvalid,
			Cause: err,
		}
		ctxV1.PolicyContextDigest = buildOrgUnitAppendVersionPolicyContextDigest(ctxV1)
		return ctxV1, contextErr
	}
	ctxV1.OrgNodeKey = strings.TrimSpace(orgNodeKey)
	resolvedSetID, err := reader.ResolveSetID(ctx, input.TenantID, ctxV1.OrgNodeKey, input.EffectiveDate)
	if err != nil {
		contextErr := &OrgUnitAppendVersionPolicyContextErrorV1{
			Code:  orgUnitAppendVersionContextCodeSetIDBindingMissing,
			Cause: err,
		}
		ctxV1.PolicyContextDigest = buildOrgUnitAppendVersionPolicyContextDigest(ctxV1)
		return ctxV1, contextErr
	}
	ctxV1.ResolvedSetID = strings.ToUpper(strings.TrimSpace(resolvedSetID))
	if ctxV1.ResolvedSetID == "" {
		contextErr := &OrgUnitAppendVersionPolicyContextErrorV1{Code: orgUnitAppendVersionContextCodeSetIDBindingMissing}
		ctxV1.PolicyContextDigest = buildOrgUnitAppendVersionPolicyContextDigest(ctxV1)
		return ctxV1, contextErr
	}
	setIDSource, sourceErr := classifyCreateOrgUnitSetIDSource(ctxV1.ResolvedSetID)
	if sourceErr != nil {
		contextErr := &OrgUnitAppendVersionPolicyContextErrorV1{
			Code:  orgUnitAppendVersionContextCodeSetIDSourceInvalid,
			Cause: sourceErr,
		}
		ctxV1.PolicyContextDigest = buildOrgUnitAppendVersionPolicyContextDigest(ctxV1)
		return ctxV1, contextErr
	}
	ctxV1.SetIDSource = setIDSource
	ctxV1.PolicyContextDigest = buildOrgUnitAppendVersionPolicyContextDigest(ctxV1)
	return ctxV1, nil
}

func resolveOrgUnitAppendVersionTreeInitialized(ctx context.Context, reader OrgUnitAppendVersionPrecheckReader, tenantID string) (bool, error) {
	if reader == nil {
		return false, nil
	}
	return reader.IsOrgTreeInitialized(ctx, tenantID)
}

func resolveOrgUnitAppendVersionTargetExistsAsOf(
	ctx context.Context,
	reader OrgUnitAppendVersionPrecheckReader,
	input OrgUnitAppendVersionPrecheckInputV1,
	orgNodeKey string,
) (bool, error) {
	if reader == nil || strings.TrimSpace(orgNodeKey) == "" || strings.TrimSpace(input.EffectiveDate) == "" {
		return false, nil
	}
	facts, err := reader.ResolveAppendFacts(ctx, input.TenantID, orgNodeKey, input.EffectiveDate)
	if err != nil {
		return false, err
	}
	return facts.TargetExistsAsOf, nil
}

func resolveOrgUnitAppendVersionFieldDecision(
	ctx context.Context,
	reader OrgUnitAppendVersionPrecheckReader,
	input OrgUnitAppendVersionPrecheckInputV1,
	orgNodeKey string,
	fieldKey string,
) (types.SetIDStrategyFieldDecision, bool, string) {
	if reader == nil {
		return types.SetIDStrategyFieldDecision{}, false, errFieldPolicyMissing
	}
	decision, found, err := reader.ResolveSetIDStrategyFieldDecision(
		ctx,
		input.TenantID,
		input.CapabilityKey,
		fieldKey,
		orgNodeKey,
		input.EffectiveDate,
	)
	if err != nil {
		return types.SetIDStrategyFieldDecision{}, false, strings.TrimSpace(mapSetIDFieldDecisionError(err).Error())
	}
	return decision, found, ""
}

func normalizeOrgUnitAppendVersionCandidateRequirements(required bool, requirements []string) []string {
	if !required && len(requirements) == 0 {
		return []string{}
	}
	if len(requirements) == 0 {
		requirements = []string{orgUnitAppendVersionCandidateRequirementNewParent}
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
	sort.Strings(out)
	return out
}

func normalizeOrgUnitAppendVersionMissingFields(
	input OrgUnitAppendVersionPrecheckInputV1,
	eval orgUnitAppendVersionPrecheckEvaluation,
) []string {
	candidates := make([]string, 0, 4)
	if strings.TrimSpace(input.OrgCode) == "" {
		candidates = append(candidates, "org_code")
	}
	if strings.TrimSpace(input.EffectiveDate) == "" {
		candidates = append(candidates, "effective_date")
	}
	if strings.TrimSpace(input.NewName) == "" && !input.NewParentRequested {
		candidates = append(candidates, "change_fields")
	}
	if eval.NameFound && eval.NameDecision.Required && strings.TrimSpace(input.NewName) == "" {
		candidates = append(candidates, "new_name")
	}
	if eval.ParentFound && eval.ParentDecision.Required && !input.NewParentRequested {
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

func normalizeOrgUnitAppendVersionFieldRejectionReasons(
	input OrgUnitAppendVersionPrecheckInputV1,
	eval orgUnitAppendVersionPrecheckEvaluation,
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

func normalizeOrgUnitAppendVersionRejectionReasons(reasons []string) []string {
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
		case orgUnitAppendVersionContextCodeOrgInvalid, orgUnitAppendVersionContextCodeSetIDBindingMissing, orgUnitAppendVersionContextCodeSetIDSourceInvalid:
			appendReason(&contextReasons, reason)
		case "FORBIDDEN", "ORG_TREE_NOT_INITIALIZED", "ORG_NOT_FOUND_AS_OF":
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

func resolveOrgUnitAppendVersionReadiness(projection OrgUnitAppendVersionPrecheckProjectionV1) string {
	if len(projection.RejectionReasons) > 0 {
		return orgUnitAppendVersionReadinessRejected
	}
	if len(projection.CandidateConfirmationRequirements) > 0 {
		return orgUnitAppendVersionReadinessCandidateConfirmation
	}
	if len(projection.MissingFields) > 0 {
		return orgUnitAppendVersionReadinessMissingFields
	}
	return orgUnitAppendVersionReadinessReady
}

func buildOrgUnitAppendVersionFieldDecisions(
	eval orgUnitAppendVersionPrecheckEvaluation,
	enabledExtFieldKeys []string,
) []OrgUnitAppendVersionFieldDecisionV1 {
	decisions := make([]OrgUnitAppendVersionFieldDecisionV1, 0, len(orgUnitAppendVersionFieldDecisionOrder)+len(enabledExtFieldKeys))
	payloadKeys := eval.MutationDecision.FieldPayloadKeys
	for _, fieldKey := range orgUnitAppendVersionFieldDecisionOrder {
		switch fieldKey {
		case "org_code":
			decisions = append(decisions, OrgUnitAppendVersionFieldDecisionV1{
				FieldKey:          fieldKey,
				Visible:           true,
				Required:          true,
				Maintainable:      false,
				FieldPayloadKey:   "",
				AllowedValueCodes: []string{},
			})
		case "effective_date":
			decisions = append(decisions, OrgUnitAppendVersionFieldDecisionV1{
				FieldKey:          fieldKey,
				Visible:           true,
				Required:          true,
				Maintainable:      false,
				FieldPayloadKey:   "",
				AllowedValueCodes: []string{},
			})
		case "name":
			decisions = append(decisions, buildOrgUnitAppendVersionPDPFieldDecision(fieldKey, eval.NameDecision, eval.NameFound, payloadKeys[fieldKey]))
		case "parent_org_code":
			decisions = append(decisions, buildOrgUnitAppendVersionPDPFieldDecision(fieldKey, eval.ParentDecision, eval.ParentFound, payloadKeys[fieldKey]))
		}
	}
	for _, fieldKey := range enabledExtFieldKeys {
		decisions = append(decisions, OrgUnitAppendVersionFieldDecisionV1{
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

func buildOrgUnitAppendVersionPDPFieldDecision(
	fieldKey string,
	decision types.SetIDStrategyFieldDecision,
	found bool,
	payloadKey string,
) OrgUnitAppendVersionFieldDecisionV1 {
	if !found {
		return OrgUnitAppendVersionFieldDecisionV1{
			FieldKey:          fieldKey,
			Visible:           strings.TrimSpace(payloadKey) != "",
			Required:          false,
			Maintainable:      strings.TrimSpace(payloadKey) != "",
			FieldPayloadKey:   strings.TrimSpace(payloadKey),
			AllowedValueCodes: []string{},
		}
	}
	return OrgUnitAppendVersionFieldDecisionV1{
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

func buildOrgUnitAppendVersionPendingDraftSummary(input OrgUnitAppendVersionPrecheckInputV1) string {
	parts := make([]string, 0, 4)
	if orgCode := strings.TrimSpace(input.OrgCode); orgCode != "" {
		parts = append(parts, "目标组织："+orgCode)
	}
	if newName := strings.TrimSpace(input.NewName); newName != "" {
		parts = append(parts, "新名称："+newName)
	}
	if parent := strings.TrimSpace(input.NewParentOrgCode); parent != "" {
		parts = append(parts, "新上级组织："+parent)
	}
	if effectiveDate := strings.TrimSpace(input.EffectiveDate); effectiveDate != "" {
		parts = append(parts, "生效日期："+effectiveDate)
	}
	return strings.Join(parts, "；")
}

func buildOrgUnitAppendVersionPolicyExplain(projection OrgUnitAppendVersionPrecheckProjectionV1) string {
	switch strings.TrimSpace(projection.Readiness) {
	case orgUnitAppendVersionReadinessReady:
		return "计划已生成，等待确认后可提交"
	case orgUnitAppendVersionReadinessMissingFields:
		return "仍有必填字段未补全"
	case orgUnitAppendVersionReadinessCandidateConfirmation:
		return "仍需确认新的上级组织"
	case orgUnitAppendVersionReadinessRejected:
		if len(projection.RejectionReasons) == 0 {
			return "当前草案已被策略拒绝"
		}
		return strings.Join(projection.RejectionReasons, ",")
	default:
		return ""
	}
}

func buildOrgUnitAppendVersionPolicyContextDigest(ctx OrgUnitAppendVersionPolicyContextV1) string {
	payload := struct {
		TenantID      string `json:"tenant_id"`
		CapabilityKey string `json:"capability_key"`
		Intent        string `json:"intent"`
		EffectiveDate string `json:"effective_date"`
		OrgCode       string `json:"org_code"`
		OrgNodeKey    string `json:"org_node_key"`
		ResolvedSetID string `json:"resolved_setid"`
		SetIDSource   string `json:"setid_source"`
	}{
		TenantID:      strings.TrimSpace(ctx.TenantID),
		CapabilityKey: strings.TrimSpace(ctx.CapabilityKey),
		Intent:        strings.TrimSpace(ctx.Intent),
		EffectiveDate: strings.TrimSpace(ctx.EffectiveDate),
		OrgCode:       strings.TrimSpace(ctx.OrgCode),
		OrgNodeKey:    strings.TrimSpace(ctx.OrgNodeKey),
		ResolvedSetID: strings.TrimSpace(ctx.ResolvedSetID),
		SetIDSource:   strings.TrimSpace(ctx.SetIDSource),
	}
	return orgUnitAppendVersionDigest(payload)
}

func buildOrgUnitAppendVersionProjectionDigest(projection OrgUnitAppendVersionPrecheckProjectionV1) string {
	payload := struct {
		Readiness                         string                                `json:"readiness"`
		MissingFields                     []string                              `json:"missing_fields"`
		FieldDecisions                    []OrgUnitAppendVersionFieldDecisionV1 `json:"field_decisions"`
		CandidateConfirmationRequirements []string                              `json:"candidate_confirmation_requirements"`
		PendingDraftSummary               string                                `json:"pending_draft_summary"`
		EffectivePolicyVersion            string                                `json:"effective_policy_version"`
		MutationPolicyVersion             string                                `json:"mutation_policy_version"`
		ResolvedSetID                     string                                `json:"resolved_setid"`
		SetIDSource                       string                                `json:"setid_source"`
		PolicyExplain                     string                                `json:"policy_explain"`
		RejectionReasons                  []string                              `json:"rejection_reasons"`
	}{
		Readiness:                         strings.TrimSpace(projection.Readiness),
		MissingFields:                     append([]string(nil), projection.MissingFields...),
		FieldDecisions:                    append([]OrgUnitAppendVersionFieldDecisionV1(nil), projection.FieldDecisions...),
		CandidateConfirmationRequirements: append([]string(nil), projection.CandidateConfirmationRequirements...),
		PendingDraftSummary:               strings.TrimSpace(projection.PendingDraftSummary),
		EffectivePolicyVersion:            strings.TrimSpace(projection.EffectivePolicyVersion),
		MutationPolicyVersion:             strings.TrimSpace(projection.MutationPolicyVersion),
		ResolvedSetID:                     strings.TrimSpace(projection.ResolvedSetID),
		SetIDSource:                       strings.TrimSpace(projection.SetIDSource),
		PolicyExplain:                     strings.TrimSpace(projection.PolicyExplain),
		RejectionReasons:                  append([]string(nil), projection.RejectionReasons...),
	}
	return orgUnitAppendVersionDigest(payload)
}

func orgUnitAppendVersionDigest(payload any) string {
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func CloneOrgUnitAppendVersionFieldDecisions(items []OrgUnitAppendVersionFieldDecisionV1) []OrgUnitAppendVersionFieldDecisionV1 {
	if len(items) == 0 {
		return nil
	}
	out := make([]OrgUnitAppendVersionFieldDecisionV1, 0, len(items))
	for _, item := range items {
		copyItem := item
		copyItem.AllowedValueCodes = append([]string(nil), item.AllowedValueCodes...)
		out = append(out, copyItem)
	}
	return out
}

func CloneOrgUnitAppendVersionProjectionV1(projection OrgUnitAppendVersionPrecheckProjectionV1) OrgUnitAppendVersionPrecheckProjectionV1 {
	return OrgUnitAppendVersionPrecheckProjectionV1{
		Readiness:                         strings.TrimSpace(projection.Readiness),
		MissingFields:                     append([]string(nil), projection.MissingFields...),
		FieldDecisions:                    CloneOrgUnitAppendVersionFieldDecisions(projection.FieldDecisions),
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
