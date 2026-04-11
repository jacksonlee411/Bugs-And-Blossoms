package server

import (
	"context"
	"strings"
)

type assistantTenantFieldConfigReader interface {
	ListEnabledTenantFieldConfigsAsOf(ctx context.Context, tenantID string, asOf string) ([]orgUnitTenantFieldConfig, error)
}

func (s *assistantConversationService) enrichCreateOrgUnitDryRunWithPolicy(
	ctx context.Context,
	tenantID string,
	intent assistantIntentSpec,
	candidates []assistantCandidate,
	resolvedCandidateID string,
	dryRun assistantDryRunResult,
) assistantDryRunResult {
	if s == nil || strings.TrimSpace(intent.Action) != assistantIntentCreateOrgUnit {
		return dryRun
	}
	if len(assistantIntentValidationErrors(intent)) > 0 {
		return dryRun
	}
	if strings.TrimSpace(intent.EffectiveDate) == "" || strings.TrimSpace(resolvedCandidateID) == "" {
		return dryRun
	}
	parentOrgCode := assistantResolvedCandidateCode(candidates, resolvedCandidateID)
	policyCtx, ok := s.resolveCreateOrgUnitPolicyContext(ctx, tenantID, parentOrgCode, intent.EffectiveDate)
	if !ok {
		return dryRun
	}
	orgCodeDecision, ok := assistantResolveCreateFieldDecision(
		ctx,
		tenantID,
		orgUnitCreateFieldOrgCode,
		policyCtx.ResolvedSetID,
		policyCtx.BusinessUnitNodeKey,
		intent.EffectiveDate,
	)
	if ok && assistantCreateFieldDecisionMissingRequiredValue(orgCodeDecision, strings.TrimSpace(intent.OrgCode)) {
		dryRun.ValidationErrors = append(dryRun.ValidationErrors, "FIELD_REQUIRED_VALUE_MISSING")
	}
	orgTypeDecision, ok := assistantResolveCreateFieldDecision(
		ctx,
		tenantID,
		orgUnitCreateFieldOrgType,
		policyCtx.ResolvedSetID,
		policyCtx.BusinessUnitNodeKey,
		intent.EffectiveDate,
	)
	if ok {
		resolvedOrgType := assistantCreateFieldDecisionResolvedValue(orgTypeDecision, "")
		if assistantCreateFieldDecisionMissingRequiredValue(orgTypeDecision, "") {
			dryRun.ValidationErrors = append(dryRun.ValidationErrors, "FIELD_REQUIRED_VALUE_MISSING")
		} else if resolvedOrgType != "" && !s.isCreateOrgTypeFieldEnabled(ctx, tenantID, intent.EffectiveDate) {
			dryRun.ValidationErrors = append(dryRun.ValidationErrors, "PATCH_FIELD_NOT_ALLOWED")
		}
	}
	dryRun.ValidationErrors = assistantNormalizeValidationErrors(dryRun.ValidationErrors)
	if len(dryRun.ValidationErrors) == 0 {
		dryRun.Explain = "计划已生成，等待确认后可提交"
		return dryRun
	}
	dryRun.Explain = assistantDryRunValidationExplain(dryRun.ValidationErrors)
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

func (s *assistantConversationService) resolveCreateOrgUnitPolicyContext(ctx context.Context, tenantID string, parentOrgCode string, effectiveDate string) (setIDPolicyContext, bool) {
	if s == nil || s.orgStore == nil {
		return setIDPolicyContext{}, false
	}
	setIDResolver, ok := any(s.orgStore).(orgUnitSetIDResolver)
	if !ok {
		return setIDPolicyContext{}, false
	}
	contextResolver := newSetIDContextResolver(s.orgStore, setIDResolver)
	resolvedCtx, err := contextResolver.ResolveOrgContext(ctx, tenantID, strings.TrimSpace(parentOrgCode), strings.TrimSpace(effectiveDate), "business_unit_org_code")
	if err != nil {
		return setIDPolicyContext{}, false
	}
	return setIDPolicyContext{
		TenantID:            strings.TrimSpace(tenantID),
		CapabilityKey:       orgUnitCreateFieldPolicyCapabilityKey,
		AsOf:                strings.TrimSpace(effectiveDate),
		BusinessUnitOrgCode: resolvedCtx.OrgCode,
		BusinessUnitNodeKey: resolvedCtx.OrgNodeKey,
		ResolvedSetID:       resolvedCtx.ResolvedSetID,
		SetIDSource:         resolvedCtx.SetIDSource,
	}, true
}

func assistantResolveCreateFieldDecision(ctx context.Context, tenantID string, fieldKey string, resolvedSetID string, businessUnitNodeKey string, effectiveDate string) (setIDFieldDecision, bool) {
	decision, err := defaultSetIDStrategyRegistryStore.resolveFieldDecision(
		ctx,
		tenantID,
		orgUnitCreateFieldPolicyCapabilityKey,
		fieldKey,
		strings.TrimSpace(resolvedSetID),
		strings.TrimSpace(businessUnitNodeKey),
		strings.TrimSpace(effectiveDate),
	)
	if err != nil {
		return setIDFieldDecision{}, false
	}
	return decision, true
}

func assistantCreateFieldDecisionResolvedValue(decision setIDFieldDecision, providedValue string) string {
	providedValue = strings.TrimSpace(providedValue)
	if providedValue != "" {
		return providedValue
	}
	if value := strings.TrimSpace(decision.ResolvedDefaultVal); value != "" {
		return value
	}
	if strings.TrimSpace(decision.DefaultRuleRef) != "" {
		return "__rule__"
	}
	return ""
}

func assistantCreateFieldDecisionMissingRequiredValue(decision setIDFieldDecision, providedValue string) bool {
	if !decision.Required {
		return false
	}
	return assistantCreateFieldDecisionResolvedValue(decision, providedValue) == ""
}

func (s *assistantConversationService) isCreateOrgTypeFieldEnabled(ctx context.Context, tenantID string, effectiveDate string) bool {
	if s == nil || s.orgStore == nil {
		return false
	}
	reader, ok := s.orgStore.(assistantTenantFieldConfigReader)
	if !ok {
		return false
	}
	configs, err := reader.ListEnabledTenantFieldConfigsAsOf(ctx, tenantID, effectiveDate)
	if err != nil {
		return false
	}
	for _, cfg := range configs {
		if strings.TrimSpace(cfg.FieldKey) != orgUnitCreateFieldOrgType {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(cfg.ValueType), "text") {
			return false
		}
		if !strings.EqualFold(strings.TrimSpace(cfg.DataSourceType), "DICT") {
			return false
		}
		dictCode, ok := dictCodeFromDataSourceConfig(cfg.DataSourceConfig)
		return ok && strings.EqualFold(strings.TrimSpace(dictCode), "org_type")
	}
	return false
}
