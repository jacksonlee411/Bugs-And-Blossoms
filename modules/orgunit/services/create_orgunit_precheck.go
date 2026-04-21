package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/fieldmeta"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

const (
	CreateOrgUnitMutationPolicyVersionV1        = "orgunit.create.mutation_policy.v1"
	CreateOrgUnitPolicyContextContractVersionV1 = "orgunit.create.policy_context.v1"
	CreateOrgUnitPrecheckProjectionContractV1   = "orgunit.create.precheck_projection.v1"
	createOrgUnitReadinessReady                 = "ready"
	createOrgUnitReadinessMissingFields         = "missing_fields"
	createOrgUnitReadinessCandidateConfirmation = "candidate_confirmation_required"
	createOrgUnitReadinessRejected              = "rejected"
	createOrgUnitContextCodeBusinessUnitInvalid = "business_unit_context_invalid"
	createOrgUnitContextCodeSetIDBindingMissing = "setid_binding_missing"
	createOrgUnitContextCodeSetIDSourceInvalid  = "setid_source_invalid"
	createOrgUnitCandidateRequirementParentOrg  = "parent_org_code"
)

var createOrgUnitFieldDecisionOrder = []string{
	"effective_date",
	"name",
	"parent_org_code",
	"is_business_unit",
	"manager_pernr",
	orgUnitCreateFieldOrgCode,
	orgUnitCreateFieldOrgType,
}

type CreateOrgUnitPolicyContextV1 struct {
	TenantID            string `json:"tenant_id"`
	EffectiveDate       string `json:"effective_date"`
	BusinessUnitOrgCode string `json:"business_unit_org_code"`
	BusinessUnitNodeKey string `json:"business_unit_node_key"`
	ResolvedSetID       string `json:"resolved_setid"`
	SetIDSource         string `json:"setid_source"`
	PolicyContextDigest string `json:"policy_context_digest"`
}

type CreateOrgUnitFieldDecisionV1 struct {
	FieldKey             string   `json:"field_key"`
	Visible              bool     `json:"visible"`
	Required             bool     `json:"required"`
	Maintainable         bool     `json:"maintainable"`
	FieldPayloadKey      string   `json:"field_payload_key"`
	ResolvedDefaultValue string   `json:"resolved_default_value"`
	DefaultRuleRef       string   `json:"default_rule_ref"`
	AllowedValueCodes    []string `json:"allowed_value_codes"`
}

type CreateOrgUnitPrecheckProjectionV1 struct {
	Readiness                         string                         `json:"readiness"`
	MissingFields                     []string                       `json:"missing_fields"`
	FieldDecisions                    []CreateOrgUnitFieldDecisionV1 `json:"field_decisions"`
	CandidateConfirmationRequirements []string                       `json:"candidate_confirmation_requirements"`
	PendingDraftSummary               string                         `json:"pending_draft_summary"`
	EffectivePolicyVersion            string                         `json:"effective_policy_version"`
	MutationPolicyVersion             string                         `json:"mutation_policy_version"`
	ResolvedSetID                     string                         `json:"resolved_setid"`
	SetIDSource                       string                         `json:"setid_source"`
	PolicyExplain                     string                         `json:"policy_explain"`
	RejectionReasons                  []string                       `json:"rejection_reasons"`
	ProjectionDigest                  string                         `json:"projection_digest"`
}

type CreateOrgUnitPrecheckInputV1 struct {
	TenantID                          string
	EffectiveDate                     string
	BusinessUnitOrgCode               string
	EffectivePolicyVersion            string
	CanAdmin                          bool
	CandidateConfirmationRequired     bool
	CandidateConfirmationRequirements []string
	Name                              string
	OrgCode                           string
	ManagerPernr                      string
	IsBusinessUnit                    *bool
	Ext                               map[string]any
	EnabledFieldConfigs               []types.TenantFieldConfig
}

type CreateOrgUnitPrecheckResultV1 struct {
	PolicyContext CreateOrgUnitPolicyContextV1       `json:"policy_context"`
	Projection    CreateOrgUnitPrecheckProjectionV1  `json:"projection"`
	ContextError  *CreateOrgUnitPolicyContextErrorV1 `json:"-"`
}

type CreateOrgUnitPolicyContextErrorV1 struct {
	Code  string
	Cause error
}

func (e *CreateOrgUnitPolicyContextErrorV1) Error() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.Code)
}

func (e *CreateOrgUnitPolicyContextErrorV1) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

type CreateOrgUnitPrecheckReader interface {
	ResolveOrgNodeKey(ctx context.Context, tenantID string, orgCode string) (string, error)
	ResolveSetID(ctx context.Context, tenantID string, orgNodeKey string, asOf string) (string, error)
	IsOrgTreeInitialized(ctx context.Context, tenantID string) (bool, error)
	ListEnabledTenantFieldConfigsAsOf(ctx context.Context, tenantID string, asOf string) ([]types.TenantFieldConfig, error)
}

type createOrgUnitPrecheckEvaluation struct {
	Result          CreateOrgUnitPrecheckResultV1
	MutationPolicy  OrgUnitMutationPolicyDecision
	EnabledFieldCfg []types.TenantFieldConfig
	OrgCodeDecision orgUnitFieldDecision
	OrgTypeDecision orgUnitFieldDecision
	OrgCodeFound    bool
	OrgTypeFound    bool
	OrgCodeValue    createFieldDecisionValue
	OrgTypeValue    createFieldDecisionValue
}

func BuildCreateOrgUnitPrecheckProjectionV1(
	ctx context.Context,
	reader CreateOrgUnitPrecheckReader,
	input CreateOrgUnitPrecheckInputV1,
) (CreateOrgUnitPrecheckResultV1, error) {
	eval, err := evaluateCreateOrgUnitPrecheckV1(ctx, reader, input)
	if err != nil {
		return CreateOrgUnitPrecheckResultV1{}, err
	}
	return eval.Result, nil
}

func evaluateCreateOrgUnitPrecheckV1(
	ctx context.Context,
	reader CreateOrgUnitPrecheckReader,
	input CreateOrgUnitPrecheckInputV1,
) (createOrgUnitPrecheckEvaluation, error) {
	normalizedInput := normalizeCreateOrgUnitPrecheckInput(input)
	result := CreateOrgUnitPrecheckResultV1{
		PolicyContext: CreateOrgUnitPolicyContextV1{
			TenantID:            normalizedInput.TenantID,
			EffectiveDate:       normalizedInput.EffectiveDate,
			BusinessUnitOrgCode: normalizedInput.BusinessUnitOrgCode,
		},
		Projection: CreateOrgUnitPrecheckProjectionV1{
			MutationPolicyVersion:             CreateOrgUnitMutationPolicyVersionV1,
			EffectivePolicyVersion:            normalizedInput.EffectivePolicyVersion,
			CandidateConfirmationRequirements: normalizeCreateOrgUnitCandidateRequirements(normalizedInput.CandidateConfirmationRequired, normalizedInput.CandidateConfirmationRequirements),
			MissingFields:                     []string{},
			FieldDecisions:                    []CreateOrgUnitFieldDecisionV1{},
			RejectionReasons:                  []string{},
		},
	}

	fieldCfgs, enabledExtFieldKeys, err := resolveCreateOrgUnitEnabledFieldConfigs(ctx, reader, normalizedInput)
	if err != nil {
		return createOrgUnitPrecheckEvaluation{}, err
	}
	eval := createOrgUnitPrecheckEvaluation{
		Result:          result,
		EnabledFieldCfg: fieldCfgs,
	}

	if len(eval.Result.Projection.CandidateConfirmationRequirements) == 0 {
		policyContext, contextErr := resolveCreateOrgUnitPolicyContextV1(ctx, reader, normalizedInput)
		eval.Result.PolicyContext = policyContext
		eval.Result.ContextError = contextErr
		if contextErr != nil {
			eval.Result.Projection.RejectionReasons = append(eval.Result.Projection.RejectionReasons, strings.TrimSpace(contextErr.Code))
		}
	} else {
		eval.Result.PolicyContext.PolicyContextDigest = buildCreateOrgUnitPolicyContextDigest(eval.Result.PolicyContext)
	}

	treeInitialized, err := resolveCreateOrgUnitTreeInitialized(ctx, reader, normalizedInput.TenantID)
	if err != nil {
		return createOrgUnitPrecheckEvaluation{}, err
	}
	orgAlreadyExists, err := resolveCreateOrgUnitAlreadyExists(ctx, reader, normalizedInput.TenantID, normalizedInput.OrgCode)
	if err != nil {
		return createOrgUnitPrecheckEvaluation{}, err
	}

	mutationDecision, err := resolveOrgUnitMutationPolicyInWrite(OrgUnitMutationPolicyKey{
		ActionKind:       OrgUnitActionCreate,
		EmittedEventType: OrgUnitEmittedCreate,
	}, OrgUnitMutationPolicyFacts{
		CanAdmin:            normalizedInput.CanAdmin,
		EnabledExtFieldKeys: enabledExtFieldKeys,
		TreeInitialized:     treeInitialized,
		OrgAlreadyExists:    orgAlreadyExists,
		CreateAsRoot:        normalizedInput.BusinessUnitOrgCode == "" && len(eval.Result.Projection.CandidateConfirmationRequirements) == 0,
	})
	if err != nil {
		return createOrgUnitPrecheckEvaluation{}, err
	}
	eval.MutationPolicy = mutationDecision
	if len(mutationDecision.DenyReasons) > 0 {
		eval.Result.Projection.RejectionReasons = append(eval.Result.Projection.RejectionReasons, mutationDecision.DenyReasons...)
	}

	if eval.Result.ContextError == nil && len(eval.Result.Projection.CandidateConfirmationRequirements) == 0 {
		orgCodeDecision, orgCodeFound, orgCodeErr := resolveCreateOrgUnitFieldDecision(ctx, reader, normalizedInput, eval.Result.PolicyContext.BusinessUnitNodeKey, orgUnitCreateFieldOrgCode)
		if orgCodeErr != "" {
			eval.Result.Projection.RejectionReasons = append(eval.Result.Projection.RejectionReasons, orgCodeErr)
		}
		eval.OrgCodeDecision = orgCodeDecision
		eval.OrgCodeFound = orgCodeFound
		if orgCodeFound {
			orgCodeValue, resolveErr := resolveCreateFieldDecisionValue(orgUnitCreateFieldOrgCode, normalizedInput.OrgCode, normalizedInput.OrgCode != "", orgCodeDecision)
			if resolveErr != nil {
				eval.Result.Projection.RejectionReasons = append(eval.Result.Projection.RejectionReasons, strings.TrimSpace(resolveErr.Error()))
			} else {
				eval.OrgCodeValue = orgCodeValue
				if orgCodeDecision.Required && strings.TrimSpace(orgCodeValue.value) == "" && orgCodeValue.autoCodeSpec == nil {
					eval.Result.Projection.MissingFields = append(eval.Result.Projection.MissingFields, orgUnitCreateFieldOrgCode)
				}
				if allowErr := validateFieldOptionAllowed(orgCodeValue.value, orgCodeDecision.AllowedValueCodes); allowErr != nil {
					eval.Result.Projection.RejectionReasons = append(eval.Result.Projection.RejectionReasons, strings.TrimSpace(allowErr.Error()))
				}
			}
		} else {
			eval.Result.Projection.RejectionReasons = append(eval.Result.Projection.RejectionReasons, errFieldPolicyMissing)
		}

		providedOrgType, orgTypeProvided, readErr := readCreateExtFieldString(normalizedInput.Ext, orgUnitCreateFieldOrgType)
		if readErr != nil {
			eval.Result.Projection.RejectionReasons = append(eval.Result.Projection.RejectionReasons, strings.TrimSpace(readErr.Error()))
		}
		orgTypeDecision, orgTypeFound, orgTypeErr := resolveCreateOrgUnitFieldDecision(ctx, reader, normalizedInput, eval.Result.PolicyContext.BusinessUnitNodeKey, orgUnitCreateFieldOrgType)
		if orgTypeErr != "" {
			eval.Result.Projection.RejectionReasons = append(eval.Result.Projection.RejectionReasons, orgTypeErr)
		}
		eval.OrgTypeDecision = orgTypeDecision
		eval.OrgTypeFound = orgTypeFound
		if orgTypeFound {
			orgTypeValue, resolveErr := resolveCreateFieldDecisionValue(orgUnitCreateFieldOrgType, providedOrgType, orgTypeProvided, orgTypeDecision)
			if resolveErr != nil {
				eval.Result.Projection.RejectionReasons = append(eval.Result.Projection.RejectionReasons, strings.TrimSpace(resolveErr.Error()))
			} else {
				eval.OrgTypeValue = orgTypeValue
				if orgTypeDecision.Required && strings.TrimSpace(orgTypeValue.value) == "" {
					eval.Result.Projection.MissingFields = append(eval.Result.Projection.MissingFields, orgUnitCreateFieldOrgType)
				}
				if allowErr := validateFieldOptionAllowed(orgTypeValue.value, orgTypeDecision.AllowedValueCodes); allowErr != nil {
					eval.Result.Projection.RejectionReasons = append(eval.Result.Projection.RejectionReasons, strings.TrimSpace(allowErr.Error()))
				}
				if strings.TrimSpace(orgTypeValue.value) != "" && mutationDecision.FieldPayloadKeys[orgUnitCreateFieldOrgType] == "" {
					eval.Result.Projection.RejectionReasons = append(eval.Result.Projection.RejectionReasons, errPatchFieldNotAllowed)
				}
			}
		} else {
			// org_type is optional and may be fully absent when the ext field is not enabled.
		}
	}

	eval.Result.Projection.MissingFields = normalizeCreateOrgUnitMissingFields(
		normalizedInput.EffectiveDate,
		normalizedInput.Name,
		eval.Result.Projection.MissingFields,
	)
	eval.Result.Projection.RejectionReasons = normalizeCreateOrgUnitRejectionReasons(eval.Result.Projection.RejectionReasons)
	eval.Result.Projection.FieldDecisions = buildCreateOrgUnitFieldDecisions(eval, enabledExtFieldKeys)
	eval.Result.Projection.Readiness = resolveCreateOrgUnitReadiness(eval.Result.Projection)
	eval.Result.Projection.PendingDraftSummary = buildCreateOrgUnitPendingDraftSummary(normalizedInput)
	eval.Result.Projection.ResolvedSetID = eval.Result.PolicyContext.ResolvedSetID
	eval.Result.Projection.SetIDSource = eval.Result.PolicyContext.SetIDSource
	eval.Result.Projection.PolicyExplain = buildCreateOrgUnitPolicyExplain(eval.Result.Projection)
	eval.Result.PolicyContext.PolicyContextDigest = buildCreateOrgUnitPolicyContextDigest(eval.Result.PolicyContext)
	eval.Result.Projection.ProjectionDigest = buildCreateOrgUnitProjectionDigest(eval.Result.Projection)
	return eval, nil
}

func normalizeCreateOrgUnitPrecheckInput(input CreateOrgUnitPrecheckInputV1) CreateOrgUnitPrecheckInputV1 {
	input.TenantID = strings.TrimSpace(input.TenantID)
	input.EffectiveDate = strings.TrimSpace(input.EffectiveDate)
	input.BusinessUnitOrgCode = strings.TrimSpace(input.BusinessUnitOrgCode)
	input.EffectivePolicyVersion = strings.TrimSpace(input.EffectivePolicyVersion)
	input.Name = strings.TrimSpace(input.Name)
	input.OrgCode = strings.TrimSpace(input.OrgCode)
	input.ManagerPernr = strings.TrimSpace(input.ManagerPernr)
	if input.Ext == nil {
		input.Ext = map[string]any{}
	}
	return input
}

func resolveCreateOrgUnitEnabledFieldConfigs(
	ctx context.Context,
	reader CreateOrgUnitPrecheckReader,
	input CreateOrgUnitPrecheckInputV1,
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

func resolveCreateOrgUnitPolicyContextV1(
	ctx context.Context,
	reader CreateOrgUnitPrecheckReader,
	input CreateOrgUnitPrecheckInputV1,
) (CreateOrgUnitPolicyContextV1, *CreateOrgUnitPolicyContextErrorV1) {
	ctxV1 := CreateOrgUnitPolicyContextV1{
		TenantID:            input.TenantID,
		EffectiveDate:       input.EffectiveDate,
		BusinessUnitOrgCode: input.BusinessUnitOrgCode,
	}
	if strings.TrimSpace(input.BusinessUnitOrgCode) == "" {
		ctxV1.PolicyContextDigest = buildCreateOrgUnitPolicyContextDigest(ctxV1)
		return ctxV1, nil
	}
	if reader == nil {
		err := &CreateOrgUnitPolicyContextErrorV1{Code: createOrgUnitContextCodeBusinessUnitInvalid}
		ctxV1.PolicyContextDigest = buildCreateOrgUnitPolicyContextDigest(ctxV1)
		return ctxV1, err
	}
	businessUnitOrgCode, err := normalizeOrgCode(input.BusinessUnitOrgCode)
	if err != nil {
		contextErr := &CreateOrgUnitPolicyContextErrorV1{
			Code:  createOrgUnitContextCodeBusinessUnitInvalid,
			Cause: err,
		}
		ctxV1.PolicyContextDigest = buildCreateOrgUnitPolicyContextDigest(ctxV1)
		return ctxV1, contextErr
	}
	ctxV1.BusinessUnitOrgCode = businessUnitOrgCode
	orgNodeKey, err := reader.ResolveOrgNodeKey(ctx, input.TenantID, businessUnitOrgCode)
	if err != nil {
		contextErr := &CreateOrgUnitPolicyContextErrorV1{
			Code:  createOrgUnitContextCodeBusinessUnitInvalid,
			Cause: err,
		}
		ctxV1.PolicyContextDigest = buildCreateOrgUnitPolicyContextDigest(ctxV1)
		return ctxV1, contextErr
	}
	ctxV1.BusinessUnitNodeKey = strings.TrimSpace(orgNodeKey)
	resolvedSetID, err := reader.ResolveSetID(ctx, input.TenantID, ctxV1.BusinessUnitNodeKey, input.EffectiveDate)
	if err != nil {
		contextErr := &CreateOrgUnitPolicyContextErrorV1{
			Code:  createOrgUnitContextCodeSetIDBindingMissing,
			Cause: err,
		}
		ctxV1.PolicyContextDigest = buildCreateOrgUnitPolicyContextDigest(ctxV1)
		return ctxV1, contextErr
	}
	ctxV1.ResolvedSetID = strings.ToUpper(strings.TrimSpace(resolvedSetID))
	if ctxV1.ResolvedSetID == "" {
		contextErr := &CreateOrgUnitPolicyContextErrorV1{Code: createOrgUnitContextCodeSetIDBindingMissing}
		ctxV1.PolicyContextDigest = buildCreateOrgUnitPolicyContextDigest(ctxV1)
		return ctxV1, contextErr
	}
	setIDSource, sourceErr := classifyCreateOrgUnitSetIDSource(ctxV1.ResolvedSetID)
	if sourceErr != nil {
		contextErr := &CreateOrgUnitPolicyContextErrorV1{
			Code:  createOrgUnitContextCodeSetIDSourceInvalid,
			Cause: sourceErr,
		}
		ctxV1.PolicyContextDigest = buildCreateOrgUnitPolicyContextDigest(ctxV1)
		return ctxV1, contextErr
	}
	ctxV1.SetIDSource = setIDSource
	ctxV1.PolicyContextDigest = buildCreateOrgUnitPolicyContextDigest(ctxV1)
	return ctxV1, nil
}

func resolveCreateOrgUnitTreeInitialized(ctx context.Context, reader CreateOrgUnitPrecheckReader, tenantID string) (bool, error) {
	if reader == nil {
		return false, nil
	}
	return reader.IsOrgTreeInitialized(ctx, tenantID)
}

func resolveCreateOrgUnitAlreadyExists(ctx context.Context, reader CreateOrgUnitPrecheckReader, tenantID string, orgCode string) (bool, error) {
	orgCode = strings.TrimSpace(orgCode)
	if orgCode == "" || reader == nil {
		return false, nil
	}
	normalizedOrgCode, err := normalizeOrgCode(orgCode)
	if err != nil {
		return false, nil
	}
	_, err = reader.ResolveOrgNodeKey(ctx, tenantID, normalizedOrgCode)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, orgunitpkg.ErrOrgCodeNotFound) {
		return false, nil
	}
	return false, err
}

func resolveCreateOrgUnitFieldDecision(
	ctx context.Context,
	reader CreateOrgUnitPrecheckReader,
	input CreateOrgUnitPrecheckInputV1,
	businessUnitNodeKey string,
	fieldKey string,
) (orgUnitFieldDecision, bool, string) {
	_ = ctx
	_ = reader
	_ = input.TenantID
	_ = businessUnitNodeKey
	return resolveCreateOrgUnitStaticFieldDecision(fieldConfigKeys(input.EnabledFieldConfigs), fieldKey)
}

func filterEnabledOrgUnitExtFieldConfigs(cfgs []types.TenantFieldConfig) ([]types.TenantFieldConfig, []string, error) {
	outCfgs := make([]types.TenantFieldConfig, 0, len(cfgs))
	keys := make([]string, 0, len(cfgs))
	for _, cfg := range cfgs {
		key := strings.TrimSpace(cfg.FieldKey)
		if key == "" {
			continue
		}
		if isReservedExtFieldKey(key) {
			continue
		}
		if _, ok := fieldmeta.LookupFieldDefinition(key); !ok && !fieldmeta.IsCustomPlainFieldKey(key) && !fieldmeta.IsCustomDictFieldKey(key) {
			continue
		}
		if fieldmeta.IsCustomDictFieldKey(key) {
			if !strings.EqualFold(strings.TrimSpace(cfg.ValueType), "text") {
				continue
			}
			if !strings.EqualFold(strings.TrimSpace(cfg.DataSourceType), "DICT") {
				continue
			}
			wantDictCode, _ := fieldmeta.DictCodeFromDictFieldKey(key)
			gotDictCode, ok := fieldmeta.DictCodeFromDataSourceConfig(cfg.DataSourceConfig)
			if !ok || !strings.EqualFold(strings.TrimSpace(gotDictCode), strings.TrimSpace(wantDictCode)) {
				continue
			}
		}
		cfg.FieldKey = key
		outCfgs = append(outCfgs, cfg)
		keys = append(keys, key)
	}
	return outCfgs, normalizeFieldKeys(keys), nil
}

func buildCreateOrgUnitFieldDecisions(eval createOrgUnitPrecheckEvaluation, enabledExtFieldKeys []string) []CreateOrgUnitFieldDecisionV1 {
	mutationPayloadKeys := eval.MutationPolicy.FieldPayloadKeys
	decisions := make([]CreateOrgUnitFieldDecisionV1, 0, len(createOrgUnitFieldDecisionOrder)+len(enabledExtFieldKeys))
	for _, fieldKey := range createOrgUnitFieldDecisionOrder {
		switch fieldKey {
		case orgUnitCreateFieldOrgCode:
			decisions = append(decisions, buildCreateOrgUnitPDPFieldDecision(fieldKey, eval.OrgCodeDecision, eval.OrgCodeFound, mutationPayloadKeys[fieldKey]))
		case orgUnitCreateFieldOrgType:
			decisions = append(decisions, buildCreateOrgUnitPDPFieldDecision(fieldKey, eval.OrgTypeDecision, eval.OrgTypeFound, mutationPayloadKeys[fieldKey]))
		default:
			decisions = append(decisions, buildCreateOrgUnitCoreFieldDecision(fieldKey, mutationPayloadKeys[fieldKey]))
		}
	}
	for _, fieldKey := range enabledExtFieldKeys {
		if fieldKey == orgUnitCreateFieldOrgType {
			continue
		}
		decisions = append(decisions, CreateOrgUnitFieldDecisionV1{
			FieldKey:             fieldKey,
			Visible:              true,
			Required:             false,
			Maintainable:         strings.TrimSpace(mutationPayloadKeys[fieldKey]) != "",
			FieldPayloadKey:      strings.TrimSpace(mutationPayloadKeys[fieldKey]),
			ResolvedDefaultValue: "",
			DefaultRuleRef:       "",
			AllowedValueCodes:    []string{},
		})
	}
	return decisions
}

func buildCreateOrgUnitCoreFieldDecision(fieldKey string, payloadKey string) CreateOrgUnitFieldDecisionV1 {
	required := false
	switch strings.TrimSpace(fieldKey) {
	case "effective_date", "name":
		required = true
	}
	return CreateOrgUnitFieldDecisionV1{
		FieldKey:             fieldKey,
		Visible:              true,
		Required:             required,
		Maintainable:         strings.TrimSpace(payloadKey) != "",
		FieldPayloadKey:      strings.TrimSpace(payloadKey),
		ResolvedDefaultValue: "",
		DefaultRuleRef:       "",
		AllowedValueCodes:    []string{},
	}
}

func buildCreateOrgUnitPDPFieldDecision(fieldKey string, decision orgUnitFieldDecision, found bool, payloadKey string) CreateOrgUnitFieldDecisionV1 {
	if !found {
		return CreateOrgUnitFieldDecisionV1{
			FieldKey:             fieldKey,
			Visible:              false,
			Required:             false,
			Maintainable:         false,
			FieldPayloadKey:      strings.TrimSpace(payloadKey),
			ResolvedDefaultValue: "",
			DefaultRuleRef:       "",
			AllowedValueCodes:    []string{},
		}
	}
	return CreateOrgUnitFieldDecisionV1{
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

func normalizeCreateOrgUnitCandidateRequirements(required bool, requirements []string) []string {
	if !required && len(requirements) == 0 {
		return []string{}
	}
	if len(requirements) == 0 {
		requirements = []string{createOrgUnitCandidateRequirementParentOrg}
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

func normalizeCreateOrgUnitMissingFields(effectiveDate string, name string, existing []string) []string {
	candidates := make([]string, 0, len(existing)+2)
	if strings.TrimSpace(effectiveDate) == "" {
		candidates = append(candidates, "effective_date")
	}
	if strings.TrimSpace(name) == "" {
		candidates = append(candidates, "name")
	}
	candidates = append(candidates, existing...)
	seen := make(map[string]struct{}, len(candidates))
	for _, key := range candidates {
		value := strings.TrimSpace(key)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
	}
	ordered := make([]string, 0, len(seen))
	for _, key := range []string{"effective_date", "name", orgUnitCreateFieldOrgCode, orgUnitCreateFieldOrgType} {
		value := strings.TrimSpace(key)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; !ok {
			continue
		}
		ordered = append(ordered, value)
		delete(seen, value)
	}
	if len(seen) > 0 {
		extra := make([]string, 0, len(seen))
		for key := range seen {
			extra = append(extra, key)
		}
		sort.Strings(extra)
		ordered = append(ordered, extra...)
	}
	return ordered
}

func normalizeCreateOrgUnitRejectionReasons(reasons []string) []string {
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
		code := strings.TrimSpace(reason)
		switch code {
		case createOrgUnitContextCodeBusinessUnitInvalid, createOrgUnitContextCodeSetIDBindingMissing, createOrgUnitContextCodeSetIDSourceInvalid:
			appendReason(&contextReasons, code)
		case "FORBIDDEN", "ORG_ALREADY_EXISTS", "ORG_ROOT_ALREADY_EXISTS", "ORG_TREE_NOT_INITIALIZED":
			appendReason(&mutationReasons, code)
		default:
			appendReason(&fieldReasons, code)
		}
	}
	out := make([]string, 0, len(contextReasons)+len(mutationReasons)+len(fieldReasons))
	out = append(out, contextReasons...)
	out = append(out, mutationReasons...)
	out = append(out, fieldReasons...)
	return out
}

func resolveCreateOrgUnitReadiness(projection CreateOrgUnitPrecheckProjectionV1) string {
	if len(projection.RejectionReasons) > 0 {
		return createOrgUnitReadinessRejected
	}
	if len(projection.CandidateConfirmationRequirements) > 0 {
		return createOrgUnitReadinessCandidateConfirmation
	}
	if len(projection.MissingFields) > 0 {
		return createOrgUnitReadinessMissingFields
	}
	return createOrgUnitReadinessReady
}

func buildCreateOrgUnitPendingDraftSummary(input CreateOrgUnitPrecheckInputV1) string {
	parts := make([]string, 0, 3)
	if parent := strings.TrimSpace(input.BusinessUnitOrgCode); parent != "" {
		parts = append(parts, "上级组织："+parent)
	}
	if name := strings.TrimSpace(input.Name); name != "" {
		parts = append(parts, "新建组织："+name)
	}
	if effectiveDate := strings.TrimSpace(input.EffectiveDate); effectiveDate != "" {
		parts = append(parts, "生效日期："+effectiveDate)
	}
	return strings.Join(parts, "；")
}

func buildCreateOrgUnitPolicyExplain(projection CreateOrgUnitPrecheckProjectionV1) string {
	switch strings.TrimSpace(projection.Readiness) {
	case createOrgUnitReadinessReady:
		return "计划已生成，等待确认后可提交"
	case createOrgUnitReadinessMissingFields:
		return "仍有必填字段未补全"
	case createOrgUnitReadinessCandidateConfirmation:
		return "仍需确认候选父组织"
	case createOrgUnitReadinessRejected:
		if len(projection.RejectionReasons) == 0 {
			return "当前草案已被策略拒绝"
		}
		return strings.Join(projection.RejectionReasons, ",")
	default:
		return ""
	}
}

func buildCreateOrgUnitPolicyContextDigest(ctx CreateOrgUnitPolicyContextV1) string {
	payload := struct {
		TenantID            string `json:"tenant_id"`
		EffectiveDate       string `json:"effective_date"`
		BusinessUnitOrgCode string `json:"business_unit_org_code"`
		BusinessUnitNodeKey string `json:"business_unit_node_key"`
		ResolvedSetID       string `json:"resolved_setid"`
		SetIDSource         string `json:"setid_source"`
	}{
		TenantID:            strings.TrimSpace(ctx.TenantID),
		EffectiveDate:       strings.TrimSpace(ctx.EffectiveDate),
		BusinessUnitOrgCode: strings.TrimSpace(ctx.BusinessUnitOrgCode),
		BusinessUnitNodeKey: strings.TrimSpace(ctx.BusinessUnitNodeKey),
		ResolvedSetID:       strings.TrimSpace(ctx.ResolvedSetID),
		SetIDSource:         strings.TrimSpace(ctx.SetIDSource),
	}
	return createOrgUnitDigest(payload)
}

func buildCreateOrgUnitProjectionDigest(projection CreateOrgUnitPrecheckProjectionV1) string {
	payload := struct {
		Readiness                         string                         `json:"readiness"`
		MissingFields                     []string                       `json:"missing_fields"`
		FieldDecisions                    []CreateOrgUnitFieldDecisionV1 `json:"field_decisions"`
		CandidateConfirmationRequirements []string                       `json:"candidate_confirmation_requirements"`
		PendingDraftSummary               string                         `json:"pending_draft_summary"`
		EffectivePolicyVersion            string                         `json:"effective_policy_version"`
		MutationPolicyVersion             string                         `json:"mutation_policy_version"`
		ResolvedSetID                     string                         `json:"resolved_setid"`
		SetIDSource                       string                         `json:"setid_source"`
		PolicyExplain                     string                         `json:"policy_explain"`
		RejectionReasons                  []string                       `json:"rejection_reasons"`
	}{
		Readiness:                         strings.TrimSpace(projection.Readiness),
		MissingFields:                     append([]string(nil), projection.MissingFields...),
		FieldDecisions:                    append([]CreateOrgUnitFieldDecisionV1(nil), projection.FieldDecisions...),
		CandidateConfirmationRequirements: append([]string(nil), projection.CandidateConfirmationRequirements...),
		PendingDraftSummary:               strings.TrimSpace(projection.PendingDraftSummary),
		EffectivePolicyVersion:            strings.TrimSpace(projection.EffectivePolicyVersion),
		MutationPolicyVersion:             strings.TrimSpace(projection.MutationPolicyVersion),
		ResolvedSetID:                     strings.TrimSpace(projection.ResolvedSetID),
		SetIDSource:                       strings.TrimSpace(projection.SetIDSource),
		PolicyExplain:                     strings.TrimSpace(projection.PolicyExplain),
		RejectionReasons:                  append([]string(nil), projection.RejectionReasons...),
	}
	return createOrgUnitDigest(payload)
}

func createOrgUnitDigest(payload any) string {
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func normalizeCreateOrgUnitAllowedValueCodes(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	if len(out) == 0 {
		return []string{}
	}
	return out
}

func classifyCreateOrgUnitSetIDSource(resolvedSetID string) (string, error) {
	resolvedSetID = strings.ToUpper(strings.TrimSpace(resolvedSetID))
	switch resolvedSetID {
	case "":
		return "", errors.New("resolved_setid empty")
	case "DEFLT":
		return "deflt", nil
	case "SHARE":
		return "share_preview", nil
	default:
		return "custom", nil
	}
}
