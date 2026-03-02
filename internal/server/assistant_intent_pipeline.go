package server

import (
	"context"
	"strings"
)

func (s *assistantConversationService) resolveIntent(ctx context.Context, tenantID string, conversationID string, userInput string) (assistantResolveIntentResult, error) {
	text := strings.TrimSpace(userInput)
	if assistantBoundaryViolationDetected(text) {
		return assistantResolveIntentResult{}, errAssistantPlanBoundaryViolation
	}
	if s == nil || s.modelGateway == nil {
		intent, err := assistantDecodeIntent(text)
		if err != nil {
			return assistantResolveIntentResult{}, err
		}
		return assistantResolveIntentResult{Intent: intent, ProviderName: "builtin", ModelName: "rule-based", ModelRevision: "r000000000000"}, nil
	}
	resolved, err := s.modelGateway.ResolveIntent(ctx, assistantResolveIntentRequest{
		Prompt:         text,
		ConversationID: conversationID,
		TenantID:       tenantID,
	})
	if err != nil {
		return assistantResolveIntentResult{}, err
	}
	if assistantIntentSchemaInvalid(resolved.Intent) {
		return assistantResolveIntentResult{}, errAssistantPlanSchemaConstrainedDecodeFailed
	}
	return resolved, nil
}

func assistantCompileIntentToPlans(intent assistantIntentSpec, resolvedCandidateID string) (assistantSkillExecutionPlan, assistantConfigDeltaPlan) {
	skill := assistantSkillExecutionPlan{
		SelectedSkills: []string{"assistant.plan_only"},
		ExecutionOrder: []string{"assistant.plan_only"},
		RiskTier:       assistantRiskTierForIntent(intent),
		RequiredChecks: []string{"strict_decode", "boundary_lint"},
	}
	delta := assistantConfigDeltaPlan{
		CapabilityKey: "org.orgunit_create.field_policy",
		Changes:       make([]assistantConfigChange, 0, 3),
	}
	if intent.Action == assistantIntentCreateOrgUnit {
		skill.SelectedSkills = []string{"org.orgunit_create"}
		skill.ExecutionOrder = []string{"org.orgunit_create"}
		skill.RequiredChecks = []string{"strict_decode", "boundary_lint", "candidate_confirmation", "dry_run"}
		delta.Changes = append(delta.Changes,
			assistantConfigChange{Field: "name", After: intent.EntityName},
			assistantConfigChange{Field: "effective_date", After: intent.EffectiveDate},
		)
		if strings.TrimSpace(resolvedCandidateID) != "" {
			delta.Changes = append(delta.Changes, assistantConfigChange{Field: "parent_candidate_id", After: resolvedCandidateID})
		}
	}
	return skill, delta
}

func assistantAnnotateIntentPlan(tenantID string, conversationID string, userInput string, intent *assistantIntentSpec, plan *assistantPlanSummary, dryRun *assistantDryRunResult) error {
	if intent == nil || plan == nil || dryRun == nil {
		return errAssistantPlanDeterminismViolation
	}
	intent.IntentSchemaVersion = assistantIntentSchemaVersionV1
	plan.CompilerContractVersion = assistantCompilerContractVersionV1
	plan.CapabilityMapVersion = assistantCapabilityMapVersionV1
	plan.SkillManifestDigest = assistantSkillManifestDigest(plan.SkillExecutionPlan.SelectedSkills)

	contextHash := assistantCanonicalHash(map[string]any{
		"tenant_id":       strings.TrimSpace(tenantID),
		"conversation_id": strings.TrimSpace(conversationID),
		"user_input":      strings.TrimSpace(userInput),
	})
	if contextHash == "" {
		return errAssistantPlanDeterminismViolation
	}
	intent.ContextHash = contextHash

	intentHash := assistantCanonicalHash(map[string]any{
		"action":                intent.Action,
		"parent_ref_text":       intent.ParentRefText,
		"entity_name":           intent.EntityName,
		"effective_date":        intent.EffectiveDate,
		"intent_schema_version": intent.IntentSchemaVersion,
		"context_hash":          intent.ContextHash,
	})
	if intentHash == "" {
		return errAssistantPlanDeterminismViolation
	}
	intent.IntentHash = intentHash

	dryRun.WouldCommit = false
	dryRun.ValidationErrors = append([]string(nil), dryRun.ValidationErrors...)
	planHash := assistantPlanHash(*intent, *plan, *dryRun)
	if planHash == "" {
		return errAssistantPlanDeterminismViolation
	}
	dryRun.PlanHash = planHash
	if assistantPlanHash(*intent, *plan, *dryRun) != planHash {
		return errAssistantPlanDeterminismViolation
	}
	if assistantTurnContractVersionMismatchedForCreate(*intent, *plan) {
		return errAssistantPlanContractVersionMismatch
	}
	return nil
}

func assistantPlanHash(intent assistantIntentSpec, plan assistantPlanSummary, dryRun assistantDryRunResult) string {
	cloneDryRun := dryRun
	cloneDryRun.PlanHash = ""
	return assistantCanonicalHash(map[string]any{
		"intent": intent,
		"plan": map[string]any{
			"title":                     plan.Title,
			"capability_key":            plan.CapabilityKey,
			"summary":                   plan.Summary,
			"capability_map_version":    plan.CapabilityMapVersion,
			"compiler_contract_version": plan.CompilerContractVersion,
			"skill_manifest_digest":     plan.SkillManifestDigest,
			"model_provider":            plan.ModelProvider,
			"model_name":                plan.ModelName,
			"model_revision":            plan.ModelRevision,
			"skill_execution_plan":      plan.SkillExecutionPlan,
			"config_delta_plan":         plan.ConfigDeltaPlan,
		},
		"dry_run": cloneDryRun,
	})
}

func assistantTurnContractVersionMismatchedForCreate(intent assistantIntentSpec, plan assistantPlanSummary) bool {
	if strings.TrimSpace(intent.IntentSchemaVersion) != assistantIntentSchemaVersionV1 {
		return true
	}
	if strings.TrimSpace(plan.CompilerContractVersion) != assistantCompilerContractVersionV1 {
		return true
	}
	if strings.TrimSpace(plan.CapabilityMapVersion) != assistantCapabilityMapVersionV1 {
		return true
	}
	if strings.TrimSpace(plan.SkillManifestDigest) == "" {
		return true
	}
	return false
}

func assistantTurnContractVersionMismatched(turn *assistantTurn) bool {
	if turn == nil {
		return false
	}
	if strings.TrimSpace(turn.Intent.IntentSchemaVersion) == "" {
		return false
	}
	return assistantTurnContractVersionMismatchedForCreate(turn.Intent, turn.Plan)
}
