package server

import (
	"context"
	"regexp"
	"strings"
)

var (
	assistantAnnotateIntentPlanFn              = assistantAnnotateIntentPlan
	assistantCanonicalHashFn                   = assistantCanonicalHash
	assistantPlanHashFn                        = assistantPlanHash
	assistantBuildDryRunFn                     = assistantBuildDryRun
	assistantCreateOrgUnitParentOrgCodeRE      = regexp.MustCompile(`(?:^|[，,。；\s])(?:请)?(?:使用)?(?:上级组织|父组织)?(?:编码|org_code|OrgCode)\s*([A-Za-z0-9_-]+)`)
	assistantCreateOrgUnitCarryForwardParentRE = regexp.MustCompile(`(?:^|[，,。；\s])(?:请)?在(?:父组织)?\s*([^\s，。,；]+?)\s*(?:之下|下)`)
	assistantCreateOrgUnitParentOrgPrefixRE    = regexp.MustCompile(`(?:^|[，,。；\s])(?:上级组织|父组织)\s*([^\s，。,；]+?)(?:$|[，,。；\s])`)
	assistantCreateOrgUnitCarryForwardNameRE   = regexp.MustCompile(`(?:新建|创建)\s*(?:一个|一個)?(?:名为)?\s*([^\s，。,；]+?)(?:的)?(?:部门|组织)?(?:$|[，,。；\s])`)
)

func (s *assistantConversationService) resolveIntent(ctx context.Context, tenantID string, conversationID string, userInput string) (assistantResolveIntentResult, error) {
	return s.resolveIntentWithPendingTurn(ctx, tenantID, conversationID, userInput, nil)
}

func (s *assistantConversationService) resolveIntentWithPendingTurn(ctx context.Context, tenantID string, conversationID string, userInput string, pendingTurn *assistantTurn) (assistantResolveIntentResult, error) {
	text := strings.TrimSpace(userInput)
	explicitTemporalHints := assistantExtractExplicitTemporalHints(text)
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
	prompt := assistantBuildSemanticPrompt(text, pendingTurn)
	resolved, err := s.modelGateway.ResolveIntent(ctx, assistantResolveIntentRequest{
		Prompt:         prompt,
		ConversationID: conversationID,
		TenantID:       tenantID,
	})
	if err != nil {
		return assistantResolveIntentResult{}, err
	}
	resolved.Proposal = assistantRuntimeProposalFromIntent(assistantSanitizeResolvedIntentFacts(resolved.Proposal.intentSpec(), explicitTemporalHints, pendingTurn))
	if assistantSemanticStatePresent(resolved.SemanticState) {
		state := resolved.SemanticState
		state.Slots = assistantSanitizeResolvedIntentFacts(state.intentSpec(), explicitTemporalHints, pendingTurn)
		resolved.SemanticState = state
	}
	assistantSyncResolvedSemanticResult(&resolved)
	if assistantModelSemanticStateInvalid(resolved) {
		return assistantResolveIntentResult{}, errAssistantPlanSchemaConstrainedDecodeFailed
	}
	return resolved, nil
}

func assistantModelIntentInvalid(intent assistantIntentSpec) bool {
	action := strings.TrimSpace(intent.Action)
	routeKind := strings.TrimSpace(intent.RouteKind)
	if !assistantValidRouteKind(routeKind) {
		return true
	}
	if strings.TrimSpace(intent.IntentID) == "" {
		return true
	}
	if routeKind == assistantRouteKindBusinessAction {
		if action == "" || action == assistantIntentPlanOnly {
			return true
		}
		if _, ok := assistantLookupDefaultActionSpec(action); !ok {
			return true
		}
	} else if action != "" && action != assistantIntentPlanOnly {
		return true
	}
	if effectiveDate := strings.TrimSpace(intent.EffectiveDate); effectiveDate != "" && !assistantDateISOYMD(effectiveDate) {
		return true
	}
	if targetDate := strings.TrimSpace(intent.TargetEffectiveDate); targetDate != "" && !assistantDateISOYMD(targetDate) {
		return true
	}
	return false
}

type assistantExplicitTemporalHints struct {
	EffectiveDate       string
	TargetEffectiveDate string
}

func assistantExtractExplicitTemporalHints(input string) assistantExplicitTemporalHints {
	text := strings.TrimSpace(input)
	date := ""
	if m := assistantDateISORE.FindStringSubmatch(text); len(m) == 2 {
		date = strings.TrimSpace(m[1])
	}
	if date == "" {
		if m := assistantDateCNRE.FindStringSubmatch(text); len(m) == 4 {
			year := strings.TrimSpace(m[1])
			month := strings.TrimSpace(m[2])
			day := strings.TrimSpace(m[3])
			if len(month) == 1 {
				month = "0" + month
			}
			if len(day) == 1 {
				day = "0" + day
			}
			date = year + "-" + month + "-" + day
		}
	}
	return assistantExplicitTemporalHints{
		EffectiveDate:       date,
		TargetEffectiveDate: date,
	}
}

func assistantSanitizeResolvedIntentFacts(intent assistantIntentSpec, temporalHints assistantExplicitTemporalHints, pendingTurn *assistantTurn) assistantIntentSpec {
	sanitized := intent
	explicitEffectiveDate := strings.TrimSpace(temporalHints.EffectiveDate)
	explicitTargetDate := strings.TrimSpace(firstNonEmpty(temporalHints.TargetEffectiveDate, temporalHints.EffectiveDate))
	pendingEffectiveDate := ""
	pendingTargetDate := ""
	if pendingTurn != nil {
		pendingEffectiveDate = strings.TrimSpace(pendingTurn.Intent.EffectiveDate)
		pendingTargetDate = strings.TrimSpace(firstNonEmpty(pendingTurn.Intent.TargetEffectiveDate, pendingTurn.Intent.EffectiveDate))
	}
	switch strings.TrimSpace(sanitized.Action) {
	case assistantIntentCreateOrgUnit, assistantIntentAddOrgUnitVersion, assistantIntentInsertOrgUnitVersion, assistantIntentRenameOrgUnit, assistantIntentMoveOrgUnit, assistantIntentDisableOrgUnit, assistantIntentEnableOrgUnit:
		switch {
		case explicitEffectiveDate != "":
			sanitized.EffectiveDate = explicitEffectiveDate
		case pendingEffectiveDate == "":
			sanitized.EffectiveDate = ""
		}
	case assistantIntentCorrectOrgUnit:
		switch {
		case explicitTargetDate != "":
			sanitized.TargetEffectiveDate = explicitTargetDate
		case pendingTargetDate == "":
			sanitized.TargetEffectiveDate = ""
		}
	}
	return assistantCarryForwardPendingIntentFacts(sanitized, pendingTurn)
}

func assistantCarryForwardPendingIntentFacts(intent assistantIntentSpec, pendingTurn *assistantTurn) assistantIntentSpec {
	if pendingTurn == nil || pendingTurn.Clarification == nil {
		return intent
	}
	if strings.TrimSpace(pendingTurn.Clarification.Status) != assistantClarificationStatusOpen {
		return intent
	}
	if strings.TrimSpace(intent.Action) == "" || strings.TrimSpace(intent.Action) != strings.TrimSpace(pendingTurn.Intent.Action) {
		return intent
	}
	carried := intent
	switch strings.TrimSpace(intent.Action) {
	case assistantIntentCreateOrgUnit:
		carried.ParentRefText = firstNonEmpty(carried.ParentRefText, pendingTurn.Intent.ParentRefText)
		carried.EntityName = firstNonEmpty(carried.EntityName, pendingTurn.Intent.EntityName)
		carried.EffectiveDate = firstNonEmpty(carried.EffectiveDate, pendingTurn.Intent.EffectiveDate)
	case assistantIntentAddOrgUnitVersion, assistantIntentInsertOrgUnitVersion, assistantIntentDisableOrgUnit, assistantIntentEnableOrgUnit:
		carried.OrgCode = firstNonEmpty(carried.OrgCode, pendingTurn.Intent.OrgCode)
		carried.EffectiveDate = firstNonEmpty(carried.EffectiveDate, pendingTurn.Intent.EffectiveDate)
	case assistantIntentCorrectOrgUnit:
		carried.OrgCode = firstNonEmpty(carried.OrgCode, pendingTurn.Intent.OrgCode)
		carried.TargetEffectiveDate = firstNonEmpty(carried.TargetEffectiveDate, pendingTurn.Intent.TargetEffectiveDate, pendingTurn.Intent.EffectiveDate)
	case assistantIntentRenameOrgUnit:
		carried.OrgCode = firstNonEmpty(carried.OrgCode, pendingTurn.Intent.OrgCode)
		carried.EffectiveDate = firstNonEmpty(carried.EffectiveDate, pendingTurn.Intent.EffectiveDate)
		carried.NewName = firstNonEmpty(carried.NewName, pendingTurn.Intent.NewName)
	case assistantIntentMoveOrgUnit:
		carried.OrgCode = firstNonEmpty(carried.OrgCode, pendingTurn.Intent.OrgCode)
		carried.EffectiveDate = firstNonEmpty(carried.EffectiveDate, pendingTurn.Intent.EffectiveDate)
		carried.NewParentRefText = firstNonEmpty(carried.NewParentRefText, pendingTurn.Intent.NewParentRefText)
	}
	return carried
}

func assistantSupplementCreateOrgUnitIntentForDraft(intent assistantIntentSpec, pendingTurn *assistantTurn, userInput string) assistantIntentSpec {
	intent = assistantNormalizeIntentSpec(intent)
	if !assistantCreateOrgUnitLocalSupplementEligible(intent, pendingTurn, userInput) {
		return intent
	}
	if strings.TrimSpace(intent.ParentRefText) != "" && strings.TrimSpace(intent.EntityName) != "" {
		return assistantUpgradeIntentToCreateOrgUnit(intent)
	}

	out := intent
	if strings.TrimSpace(out.ParentRefText) == "" {
		out.ParentRefText = strings.TrimSpace(firstNonEmpty(
			assistantExtractCreateOrgUnitParentRefText(userInput),
			assistantExtractCreateOrgUnitParentRefTextFromPending(pendingTurn),
		))
	}
	if strings.TrimSpace(out.EntityName) == "" {
		out.EntityName = strings.TrimSpace(firstNonEmpty(
			assistantExtractCreateOrgUnitEntityName(userInput),
			assistantExtractCreateOrgUnitEntityNameFromPending(pendingTurn),
		))
	}
	if strings.TrimSpace(out.EffectiveDate) == "" {
		explicitDates := assistantExtractExplicitTemporalHints(userInput)
		out.EffectiveDate = strings.TrimSpace(firstNonEmpty(
			explicitDates.EffectiveDate,
			assistantExtractCreateOrgUnitEffectiveDateFromPending(pendingTurn),
		))
	}
	return assistantUpgradeIntentToCreateOrgUnit(out)
}

func assistantCreateOrgUnitLocalSupplementEligible(intent assistantIntentSpec, pendingTurn *assistantTurn, userInput string) bool {
	intent = assistantNormalizeIntentSpec(intent)
	action := strings.TrimSpace(intent.Action)
	if action == assistantIntentCreateOrgUnit {
		return true
	}
	if action != "" && action != assistantIntentPlanOnly {
		return false
	}
	routeKind := strings.TrimSpace(intent.RouteKind)
	if routeKind != "" && routeKind != assistantRouteKindUncertain {
		return false
	}
	if pendingTurn != nil && strings.TrimSpace(pendingTurn.Intent.Action) == assistantIntentCreateOrgUnit {
		return assistantCreateOrgUnitSupplementEvidenceFromUserInput(userInput)
	}
	return assistantLooksLikeCreateOrgUnitUserInput(userInput)
}

func assistantLooksLikeCreateOrgUnitUserInput(input string) bool {
	text := strings.TrimSpace(input)
	if text == "" {
		return false
	}
	if !strings.Contains(text, "新建") && !strings.Contains(text, "创建") {
		return false
	}
	return assistantExtractCreateOrgUnitParentRefText(text) != "" || assistantExtractCreateOrgUnitEntityName(text) != ""
}

func assistantCreateOrgUnitSupplementEvidenceFromUserInput(input string) bool {
	text := strings.TrimSpace(input)
	if text == "" {
		return false
	}
	if assistantExtractCreateOrgUnitParentRefText(text) != "" || assistantExtractCreateOrgUnitEntityName(text) != "" {
		return true
	}
	hints := assistantExtractExplicitTemporalHints(text)
	return strings.TrimSpace(hints.EffectiveDate) != ""
}

func assistantUpgradeIntentToCreateOrgUnit(intent assistantIntentSpec) assistantIntentSpec {
	upgraded := assistantNormalizeIntentSpec(intent)
	upgraded.Action = assistantIntentCreateOrgUnit
	upgraded.RouteKind = assistantRouteKindBusinessAction
	if upgraded.IntentID == "" || upgraded.IntentID == assistantRouteFallbackUncertainID {
		upgraded.IntentID = assistantSemanticIntentIDForAction(assistantIntentCreateOrgUnit)
	}
	if upgraded.RouteCatalogVersion == "" {
		if runtime, err := assistantLoadKnowledgeRuntimeFn(); err == nil && runtime != nil {
			if entry, ok := runtime.routeByAction[assistantIntentCreateOrgUnit]; ok {
				upgraded.RouteCatalogVersion = strings.TrimSpace(runtime.RouteCatalogVersion)
				if upgraded.IntentID == "" {
					upgraded.IntentID = strings.TrimSpace(entry.IntentID)
				}
			} else {
				upgraded.RouteCatalogVersion = strings.TrimSpace(runtime.RouteCatalogVersion)
			}
		}
	}
	return assistantNormalizeIntentSpec(upgraded)
}

func assistantExtractCreateOrgUnitParentRefText(input string) string {
	text := strings.TrimSpace(input)
	if text == "" {
		return ""
	}
	match := assistantCreateOrgUnitParentOrgCodeRE.FindStringSubmatch(text)
	if len(match) == 2 {
		return assistantNormalizeCreateOrgUnitCarryForwardText(match[1])
	}
	match = assistantCreateOrgUnitCarryForwardParentRE.FindStringSubmatch(text)
	if len(match) == 2 {
		return assistantNormalizeCreateOrgUnitCarryForwardText(match[1])
	}
	match = assistantCreateOrgUnitParentOrgPrefixRE.FindStringSubmatch(text)
	if len(match) != 2 {
		return ""
	}
	return assistantNormalizeCreateOrgUnitCarryForwardText(match[1])
}

func assistantExtractCreateOrgUnitEntityName(input string) string {
	text := strings.TrimSpace(input)
	if text == "" {
		return ""
	}
	match := assistantCreateOrgUnitCarryForwardNameRE.FindStringSubmatch(text)
	if len(match) != 2 {
		return ""
	}
	name := assistantNormalizeCreateOrgUnitCarryForwardText(match[1])
	if assistantCreateOrgUnitGenericEntityName(name) {
		return ""
	}
	return name
}

func assistantExtractCreateOrgUnitParentRefTextFromPending(pendingTurn *assistantTurn) string {
	if pendingTurn == nil {
		return ""
	}
	if text := assistantExtractCreateOrgUnitParentRefText(pendingTurn.UserInput); text != "" {
		return text
	}
	reply := assistantPendingTurnReplyText(pendingTurn)
	if !strings.Contains(reply, "上级组织") {
		return ""
	}
	return ""
}

func assistantExtractCreateOrgUnitEntityNameFromPending(pendingTurn *assistantTurn) string {
	if pendingTurn == nil {
		return ""
	}
	if text := assistantExtractCreateOrgUnitEntityName(pendingTurn.UserInput); text != "" {
		return text
	}
	reply := assistantPendingTurnReplyText(pendingTurn)
	start := strings.Index(reply, "新建“")
	if start < 0 {
		return ""
	}
	rest := reply[start+len("新建“"):]
	end := strings.Index(rest, "”")
	if end < 0 {
		return ""
	}
	return assistantNormalizeCreateOrgUnitCarryForwardText(rest[:end])
}

func assistantExtractCreateOrgUnitEffectiveDateFromPending(pendingTurn *assistantTurn) string {
	if pendingTurn == nil {
		return ""
	}
	return strings.TrimSpace(pendingTurn.Intent.EffectiveDate)
}

func assistantPendingTurnReplyText(pendingTurn *assistantTurn) string {
	if pendingTurn == nil {
		return ""
	}
	if pendingTurn.CommitReply != nil && strings.TrimSpace(pendingTurn.CommitReply.Message) != "" {
		return strings.TrimSpace(pendingTurn.CommitReply.Message)
	}
	if pendingTurn.ReplyNLG != nil && strings.TrimSpace(pendingTurn.ReplyNLG.Text) != "" {
		return strings.TrimSpace(pendingTurn.ReplyNLG.Text)
	}
	return ""
}

func assistantNormalizeCreateOrgUnitCarryForwardText(value string) string {
	text := strings.TrimSpace(value)
	text = strings.Trim(text, `"'“”‘’（）()[]【】`)
	text = strings.Trim(text, "，,。；;：:")
	text = strings.TrimSpace(text)
	return text
}

func assistantCreateOrgUnitGenericEntityName(value string) bool {
	switch strings.TrimSpace(value) {
	case "", "部门", "组织", "一个部门", "一个组织", "一個部門", "一個組織":
		return true
	default:
		return false
	}
}

func assistantCompileIntentToPlans(intent assistantIntentSpec, resolvedCandidateID string) (assistantSkillExecutionPlan, assistantConfigDeltaPlan) {
	spec, _ := assistantLookupDefaultActionSpec(intent.Action)
	return assistantCompileIntentToPlansWithSpec(intent, resolvedCandidateID, spec)
}

func assistantCompileIntentToPlansWithSpec(intent assistantIntentSpec, resolvedCandidateID string, spec assistantActionSpec) (assistantSkillExecutionPlan, assistantConfigDeltaPlan) {
	skill := assistantSkillExecutionPlan{
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
	if strings.TrimSpace(plan.SkillManifestDigest) == "" {
		plan.SkillManifestDigest = assistantCanonicalHashFn(map[string]any{
			"projection_only": true,
			"route_kind":      intent.RouteKind,
			"intent_id":       intent.IntentID,
		})
	}

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
		"intent_id":             intent.IntentID,
		"route_kind":            intent.RouteKind,
		"route_catalog_version": intent.RouteCatalogVersion,
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
			"knowledge_snapshot_digest": plan.KnowledgeSnapshotDigest,
			"route_catalog_version":     plan.RouteCatalogVersion,
			"resolver_contract_version": plan.ResolverContractVersion,
			"context_template_version":  plan.ContextTemplateVersion,
			"reply_guidance_version":    plan.ReplyGuidanceVersion,
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
