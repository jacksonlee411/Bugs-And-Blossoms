package server

import (
	"context"
	"errors"
	"strings"
)

var (
	assistantAnnotateIntentPlanFn = assistantAnnotateIntentPlan
	assistantCanonicalHashFn      = assistantCanonicalHash
	assistantPlanHashFn           = assistantPlanHash
	assistantBuildDryRunFn        = assistantBuildDryRun
)

func (s *assistantConversationService) resolveIntent(ctx context.Context, tenantID string, conversationID string, userInput string) (assistantResolveIntentResult, error) {
	text := strings.TrimSpace(userInput)
	localIntent := assistantExtractIntent(text)
	if assistantBoundaryViolationDetected(text) {
		return assistantResolveIntentResult{}, errAssistantPlanBoundaryViolation
	}
	if s == nil {
		return assistantResolveIntentResult{}, errAssistantServiceMissing
	}
	if s.gatewayErr != nil {
		return assistantResolveIntentResult{}, s.gatewayErr
	}
	if s.modelGateway == nil {
		return assistantResolveIntentResult{}, errAssistantModelProviderUnavailable
	}
	resolved, err := s.modelGateway.ResolveIntent(ctx, assistantResolveIntentRequest{
		Prompt:         text,
		ConversationID: conversationID,
		TenantID:       tenantID,
	})
	if err != nil {
		if assistantShouldFallbackIntentLocally(err) {
			return assistantResolveIntentLocally(text)
		}
		return assistantResolveIntentResult{}, err
	}
	resolved, upgradedFromPlanOnly := assistantNormalizeResolvedIntentWithLocalFacts(resolved, localIntent)
	if assistantIntentSchemaInvalid(resolved.Intent) {
		if upgradedFromPlanOnly {
			return resolved, nil
		}
		// Real provider may occasionally emit partial JSON; retry once with the same runtime path.
		retryResolved, retryErr := s.modelGateway.ResolveIntent(ctx, assistantResolveIntentRequest{
			Prompt:         text,
			ConversationID: conversationID,
			TenantID:       tenantID,
		})
		if retryErr != nil {
			if assistantShouldFallbackIntentLocally(retryErr) {
				return assistantResolveIntentLocally(text)
			}
			return assistantResolveIntentResult{}, retryErr
		}
		retryResolved, retryUpgradedFromPlanOnly := assistantNormalizeResolvedIntentWithLocalFacts(retryResolved, localIntent)
		if assistantIntentSchemaInvalid(retryResolved.Intent) {
			if retryUpgradedFromPlanOnly {
				return retryResolved, nil
			}
			return assistantResolveIntentLocally(text)
		}
		return retryResolved, nil
	}
	return resolved, nil
}

func assistantNormalizeResolvedIntentWithLocalFacts(resolved assistantResolveIntentResult, localIntent assistantIntentSpec) (assistantResolveIntentResult, bool) {
	resolved.Intent = assistantOverlayExplicitIntentFacts(resolved.Intent, localIntent)
	if assistantShouldUpgradeIntentFromLocalFacts(resolved.Intent.Action, localIntent) {
		resolved.Intent = localIntent
		return resolved, true
	}
	return resolved, false
}

func assistantShouldUpgradeIntentFromLocalFacts(resolvedAction string, localIntent assistantIntentSpec) bool {
	if strings.TrimSpace(localIntent.Action) != assistantIntentCreateOrgUnit {
		return false
	}
	action := strings.TrimSpace(resolvedAction)
	if action == "" || action == assistantIntentPlanOnly {
		return true
	}
	_, ok := assistantLookupDefaultActionSpec(action)
	return !ok
}

func assistantOverlayExplicitIntentFacts(intent assistantIntentSpec, localIntent assistantIntentSpec) assistantIntentSpec {
	if strings.TrimSpace(intent.Action) != assistantIntentCreateOrgUnit {
		return intent
	}
	if strings.TrimSpace(intent.ParentRefText) == "" && strings.TrimSpace(localIntent.ParentRefText) != "" {
		intent.ParentRefText = strings.TrimSpace(localIntent.ParentRefText)
	}
	if strings.TrimSpace(intent.EntityName) == "" && strings.TrimSpace(localIntent.EntityName) != "" {
		intent.EntityName = strings.TrimSpace(localIntent.EntityName)
	}
	if strings.TrimSpace(localIntent.EffectiveDate) != "" {
		intent.EffectiveDate = strings.TrimSpace(localIntent.EffectiveDate)
		return intent
	}
	intent.EffectiveDate = ""
	return intent
}

func assistantShouldFallbackIntentLocally(err error) bool {
	return errors.Is(err, errAssistantPlanSchemaConstrainedDecodeFailed)
}

func assistantResolveIntentLocally(userInput string) (assistantResolveIntentResult, error) {
	intent := assistantExtractIntent(strings.TrimSpace(userInput))
	_ = assistantBuildPlan(intent)
	return assistantResolveIntentResult{
		Intent:        intent,
		ProviderName:  "deterministic",
		ModelName:     "builtin-intent-extractor",
		ModelRevision: assistantIntentSchemaVersionV1,
	}, nil
}

func assistantCompileIntentToPlans(intent assistantIntentSpec, resolvedCandidateID string) (assistantSkillExecutionPlan, assistantConfigDeltaPlan) {
	spec, _ := assistantLookupDefaultActionSpec(intent.Action)
	return assistantCompileIntentToPlansWithSpec(intent, resolvedCandidateID, spec)
}

func assistantCompileIntentToPlansWithSpec(intent assistantIntentSpec, resolvedCandidateID string, spec assistantActionSpec) (assistantSkillExecutionPlan, assistantConfigDeltaPlan) {
	skill := assistantSkillExecutionPlan{
		SelectedSkills: []string{"assistant.plan_only"},
		ExecutionOrder: []string{"assistant.plan_only"},
		RiskTier:       strings.TrimSpace(spec.Security.RiskTier),
		RequiredChecks: append([]string(nil), spec.Security.RequiredChecks...),
	}
	if skill.RiskTier == "" {
		skill.RiskTier = "low"
	}
	if len(skill.RequiredChecks) == 0 {
		skill.RequiredChecks = []string{"strict_decode", "boundary_lint"}
	}
	capabilityKey := strings.TrimSpace(spec.CapabilityKey)
	if capabilityKey == "" {
		capabilityKey = "org.orgunit_create.field_policy"
	}
	delta := assistantConfigDeltaPlan{
		CapabilityKey: capabilityKey,
		Changes:       make([]assistantConfigChange, 0, 4),
	}
	switch strings.TrimSpace(intent.Action) {
	case assistantIntentCreateOrgUnit:
		skill.SelectedSkills = []string{"org.orgunit_create"}
		skill.ExecutionOrder = []string{"org.orgunit_create"}
		delta.Changes = append(delta.Changes,
			assistantConfigChange{Field: "name", After: intent.EntityName},
			assistantConfigChange{Field: "effective_date", After: intent.EffectiveDate},
		)
		if strings.TrimSpace(resolvedCandidateID) != "" {
			delta.Changes = append(delta.Changes, assistantConfigChange{Field: "parent_candidate_id", After: resolvedCandidateID})
		}
	case assistantIntentAddOrgUnitVersion:
		skill.SelectedSkills = []string{"org.orgunit_add_version"}
		skill.ExecutionOrder = []string{"org.orgunit_add_version"}
		delta.Changes = append(delta.Changes,
			assistantConfigChange{Field: "org_code", After: intent.OrgCode},
			assistantConfigChange{Field: "effective_date", After: intent.EffectiveDate},
		)
	case assistantIntentInsertOrgUnitVersion:
		skill.SelectedSkills = []string{"org.orgunit_insert_version"}
		skill.ExecutionOrder = []string{"org.orgunit_insert_version"}
		delta.Changes = append(delta.Changes,
			assistantConfigChange{Field: "org_code", After: intent.OrgCode},
			assistantConfigChange{Field: "effective_date", After: intent.EffectiveDate},
		)
	case assistantIntentCorrectOrgUnit:
		skill.SelectedSkills = []string{"org.orgunit_correct"}
		skill.ExecutionOrder = []string{"org.orgunit_correct"}
		delta.Changes = append(delta.Changes,
			assistantConfigChange{Field: "org_code", After: intent.OrgCode},
			assistantConfigChange{Field: "target_effective_date", After: intent.TargetEffectiveDate},
		)
	case assistantIntentRenameOrgUnit:
		skill.SelectedSkills = []string{"org.orgunit_rename"}
		skill.ExecutionOrder = []string{"org.orgunit_rename"}
		delta.Changes = append(delta.Changes,
			assistantConfigChange{Field: "org_code", After: intent.OrgCode},
			assistantConfigChange{Field: "effective_date", After: intent.EffectiveDate},
			assistantConfigChange{Field: "new_name", After: intent.NewName},
		)
	case assistantIntentMoveOrgUnit:
		skill.SelectedSkills = []string{"org.orgunit_move"}
		skill.ExecutionOrder = []string{"org.orgunit_move"}
		delta.Changes = append(delta.Changes,
			assistantConfigChange{Field: "org_code", After: intent.OrgCode},
			assistantConfigChange{Field: "effective_date", After: intent.EffectiveDate},
		)
		if strings.TrimSpace(resolvedCandidateID) != "" {
			delta.Changes = append(delta.Changes, assistantConfigChange{Field: "new_parent_candidate_id", After: resolvedCandidateID})
		}
	case assistantIntentDisableOrgUnit:
		skill.SelectedSkills = []string{"org.orgunit_disable"}
		skill.ExecutionOrder = []string{"org.orgunit_disable"}
		delta.Changes = append(delta.Changes,
			assistantConfigChange{Field: "org_code", After: intent.OrgCode},
			assistantConfigChange{Field: "effective_date", After: intent.EffectiveDate},
		)
	case assistantIntentEnableOrgUnit:
		skill.SelectedSkills = []string{"org.orgunit_enable"}
		skill.ExecutionOrder = []string{"org.orgunit_enable"}
		delta.Changes = append(delta.Changes,
			assistantConfigChange{Field: "org_code", After: intent.OrgCode},
			assistantConfigChange{Field: "effective_date", After: intent.EffectiveDate},
		)
	}
	if strings.TrimSpace(intent.NewName) != "" && strings.TrimSpace(intent.Action) != assistantIntentRenameOrgUnit {
		delta.Changes = append(delta.Changes, assistantConfigChange{Field: "new_name", After: intent.NewName})
	}
	if strings.TrimSpace(intent.NewParentRefText) != "" && strings.TrimSpace(resolvedCandidateID) == "" {
		delta.Changes = append(delta.Changes, assistantConfigChange{Field: "new_parent_ref_text", After: intent.NewParentRefText})
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

	contextHash := assistantCanonicalHashFn(map[string]any{
		"tenant_id":       strings.TrimSpace(tenantID),
		"conversation_id": strings.TrimSpace(conversationID),
		"user_input":      strings.TrimSpace(userInput),
	})
	if contextHash == "" {
		return errAssistantPlanDeterminismViolation
	}
	intent.ContextHash = contextHash

	intentHash := assistantCanonicalHashFn(map[string]any{
		"action":                intent.Action,
		"parent_ref_text":       intent.ParentRefText,
		"entity_name":           intent.EntityName,
		"effective_date":        intent.EffectiveDate,
		"org_code":              intent.OrgCode,
		"target_effective_date": intent.TargetEffectiveDate,
		"new_name":              intent.NewName,
		"new_parent_ref_text":   intent.NewParentRefText,
		"intent_schema_version": intent.IntentSchemaVersion,
		"context_hash":          intent.ContextHash,
	})
	if intentHash == "" {
		return errAssistantPlanDeterminismViolation
	}
	intent.IntentHash = intentHash

	dryRun.WouldCommit = false
	dryRun.ValidationErrors = append([]string(nil), dryRun.ValidationErrors...)
	planHash := assistantPlanHashFn(*intent, *plan, *dryRun)
	if planHash == "" {
		return errAssistantPlanDeterminismViolation
	}
	dryRun.PlanHash = planHash
	if assistantPlanHashFn(*intent, *plan, *dryRun) != planHash {
		return errAssistantPlanDeterminismViolation
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
			"action_id":                 plan.ActionID,
			"action_version":            plan.ActionVersion,
			"capability_key":            plan.CapabilityKey,
			"commit_adapter_key":        plan.CommitAdapterKey,
			"summary":                   plan.Summary,
			"capability_map_version":    plan.CapabilityMapVersion,
			"compiler_contract_version": plan.CompilerContractVersion,
			"skill_manifest_digest":     plan.SkillManifestDigest,
			"model_provider":            plan.ModelProvider,
			"model_name":                plan.ModelName,
			"model_revision":            plan.ModelRevision,
			"version_tuple":             plan.VersionTuple,
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
