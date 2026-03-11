package server

import (
	"strings"
	"time"
)

const (
	assistantClarificationKindMissingSlots       = "missing_slots"
	assistantClarificationKindCandidatePick      = "candidate_pick"
	assistantClarificationKindCandidateConfirm   = "candidate_confirm"
	assistantClarificationKindIntentDisambiguate = "intent_disambiguation"
	assistantClarificationKindFormatConfirm      = "format_confirmation"

	assistantClarificationStatusOpen      = "open"
	assistantClarificationStatusResolved  = "resolved"
	assistantClarificationStatusExhausted = "exhausted"
	assistantClarificationStatusAborted   = "aborted"

	assistantClarificationExitBusinessResume = "business_action_resume"
	assistantClarificationExitUncertain      = "uncertain"
	assistantClarificationExitManualHint     = "manual_hint"

	assistantClarificationReasonIntentDisambiguationRequired = "intent_disambiguation_required"
	assistantClarificationReasonCandidatePickRequired        = "candidate_pick_required"
	assistantClarificationReasonCandidateConfirmRequired     = "candidate_confirm_required"
	assistantClarificationReasonDateFormatRequired           = "date_format_confirmation_required"
	assistantClarificationReasonMissingRequiredSlot          = "missing_required_slot"
	assistantClarificationReasonNoProgress                   = "clarification_no_progress"
	assistantClarificationReasonRoundsExhausted              = "clarification_rounds_exhausted"
)

const (
	assistantClarificationMaxRoundsIntentDisambiguate = 2
	assistantClarificationMaxRoundsCandidatePick      = 2
	assistantClarificationMaxRoundsCandidateConfirm   = 1
	assistantClarificationMaxRoundsFormatConfirm      = 2
	assistantClarificationMaxRoundsMissingSlots       = 3
)

type assistantClarificationDecision struct {
	ClarificationKind       string   `json:"clarification_kind,omitempty"`
	Status                  string   `json:"status,omitempty"`
	PromptTemplateID        string   `json:"prompt_template_id,omitempty"`
	RequiredSlots           []string `json:"required_slots,omitempty"`
	MissingSlots            []string `json:"missing_slots,omitempty"`
	CandidateActionIDs      []string `json:"candidate_action_ids,omitempty"`
	CandidateIDs            []string `json:"candidate_ids,omitempty"`
	ReasonCodes             []string `json:"reason_codes,omitempty"`
	MaxRounds               int      `json:"max_rounds,omitempty"`
	CurrentRound            int      `json:"current_round,omitempty"`
	ExitTo                  string   `json:"exit_to,omitempty"`
	AwaitPhase              string   `json:"await_phase,omitempty"`
	KnowledgeSnapshotDigest string   `json:"knowledge_snapshot_digest,omitempty"`
	RouteCatalogVersion     string   `json:"route_catalog_version,omitempty"`
}

type assistantClarificationBuildInput struct {
	UserInput            string
	Intent               assistantIntentSpec
	RouteDecision        assistantIntentRouteDecision
	DryRun               assistantDryRunResult
	Candidates           []assistantCandidate
	ResolvedCandidateID  string
	SelectedCandidateID  string
	Runtime              *assistantKnowledgeRuntime
	PendingClarification *assistantClarificationDecision
	ResumeProgress       bool
}

type assistantClarificationResumeResult struct {
	Intent              assistantIntentSpec
	ResolvedCandidateID string
	SelectedCandidateID string
	Progress            bool
}

var (
	assistantBuildClarificationDecisionFn = assistantBuildClarificationDecision
	assistantResumeFromClarificationFn    = assistantResumeFromClarification
)

func assistantClarificationKindAwaitPhase(kind string) string {
	switch strings.TrimSpace(kind) {
	case assistantClarificationKindMissingSlots:
		return assistantPhaseAwaitMissingFields
	case assistantClarificationKindCandidatePick:
		return assistantPhaseAwaitCandidatePick
	case assistantClarificationKindCandidateConfirm:
		return assistantPhaseAwaitCandidateConfirm
	case assistantClarificationKindIntentDisambiguate, assistantClarificationKindFormatConfirm:
		return assistantPhaseAwaitClarification
	default:
		return ""
	}
}

func assistantClarificationKindMaxRounds(kind string) int {
	switch strings.TrimSpace(kind) {
	case assistantClarificationKindIntentDisambiguate:
		return assistantClarificationMaxRoundsIntentDisambiguate
	case assistantClarificationKindCandidatePick:
		return assistantClarificationMaxRoundsCandidatePick
	case assistantClarificationKindCandidateConfirm:
		return assistantClarificationMaxRoundsCandidateConfirm
	case assistantClarificationKindFormatConfirm:
		return assistantClarificationMaxRoundsFormatConfirm
	case assistantClarificationKindMissingSlots:
		return assistantClarificationMaxRoundsMissingSlots
	default:
		return 0
	}
}

func assistantClarificationDecisionPresent(decision *assistantClarificationDecision) bool {
	if decision == nil {
		return false
	}
	return strings.TrimSpace(decision.ClarificationKind) != "" ||
		strings.TrimSpace(decision.Status) != "" ||
		strings.TrimSpace(decision.PromptTemplateID) != "" ||
		len(decision.RequiredSlots) > 0 ||
		len(decision.MissingSlots) > 0 ||
		len(decision.CandidateActionIDs) > 0 ||
		len(decision.CandidateIDs) > 0 ||
		len(decision.ReasonCodes) > 0 ||
		decision.MaxRounds > 0 ||
		decision.CurrentRound > 0 ||
		strings.TrimSpace(decision.ExitTo) != "" ||
		strings.TrimSpace(decision.AwaitPhase) != "" ||
		strings.TrimSpace(decision.KnowledgeSnapshotDigest) != "" ||
		strings.TrimSpace(decision.RouteCatalogVersion) != ""
}

func assistantTurnOpenClarification(turn *assistantTurn) *assistantClarificationDecision {
	if turn == nil || turn.Clarification == nil {
		return nil
	}
	if strings.TrimSpace(turn.Clarification.Status) != assistantClarificationStatusOpen {
		return nil
	}
	return turn.Clarification
}

func assistantTurnHasOpenClarification(turn *assistantTurn) bool {
	return assistantTurnOpenClarification(turn) != nil
}

func assistantClarificationMissingSlotsFromValidation(validation []string) []string {
	turn := &assistantTurn{DryRun: assistantDryRunResult{ValidationErrors: append([]string(nil), validation...)}}
	return assistantTurnMissingFields(turn)
}

func assistantClarificationRequiredSlots(runtime *assistantKnowledgeRuntime, routeDecision assistantIntentRouteDecision, intent assistantIntentSpec) []string {
	if runtime == nil {
		return nil
	}
	intentID := strings.TrimSpace(routeDecision.IntentID)
	if intentID == "" {
		intentID = strings.TrimSpace(intent.IntentID)
	}
	actionID := strings.TrimSpace(intent.Action)
	for _, entry := range runtime.routeCatalog.Entries {
		if intentID != "" && strings.TrimSpace(entry.IntentID) != intentID {
			continue
		}
		if actionID != "" && actionID != assistantIntentPlanOnly {
			entryActionID := strings.TrimSpace(entry.ActionID)
			if entryActionID != "" && entryActionID != actionID {
				continue
			}
		}
		return append([]string(nil), assistantNormalizeRouteStringSlice(entry.RequiredSlots)...)
	}
	return nil
}

func assistantClarificationPromptTemplate(runtime *assistantKnowledgeRuntime, routeDecision assistantIntentRouteDecision, intent assistantIntentSpec) string {
	if runtime == nil {
		return ""
	}
	intentID := strings.TrimSpace(routeDecision.IntentID)
	if intentID == "" {
		intentID = strings.TrimSpace(intent.IntentID)
	}
	actionID := strings.TrimSpace(intent.Action)
	for _, entry := range runtime.routeCatalog.Entries {
		if intentID != "" && strings.TrimSpace(entry.IntentID) != intentID {
			continue
		}
		if actionID != "" && actionID != assistantIntentPlanOnly {
			entryActionID := strings.TrimSpace(entry.ActionID)
			if entryActionID != "" && entryActionID != actionID {
				continue
			}
		}
		return strings.TrimSpace(entry.ClarificationTemplateID)
	}
	return ""
}

func assistantClarificationActionCandidates(userInput string, routeDecision assistantIntentRouteDecision, intent assistantIntentSpec) []string {
	candidates := append([]string(nil), routeDecision.CandidateActionIDs...)
	text := strings.ToLower(strings.TrimSpace(userInput))
	if strings.Contains(text, "新建") || strings.Contains(text, "创建") || strings.Contains(text, "create") {
		candidates = append(candidates, assistantIntentCreateOrgUnit)
	}
	if strings.Contains(text, "移动") || strings.Contains(text, "move") {
		candidates = append(candidates, assistantIntentMoveOrgUnit)
	}
	if action := strings.TrimSpace(intent.Action); action != "" && action != assistantIntentPlanOnly {
		candidates = append(candidates, action)
	}
	return assistantNormalizeRouteStringSlice(candidates)
}

func assistantClarificationActionStable(intent assistantIntentSpec, routeDecision assistantIntentRouteDecision) bool {
	action := strings.TrimSpace(intent.Action)
	if action == "" || action == assistantIntentPlanOnly {
		return false
	}
	routeKind := strings.TrimSpace(routeDecision.RouteKind)
	if routeKind == "" {
		routeKind = strings.TrimSpace(intent.RouteKind)
	}
	if routeKind != assistantRouteKindBusinessAction {
		return false
	}
	if len(routeDecision.CandidateActionIDs) > 1 {
		return false
	}
	return true
}

func assistantClarificationCurrentSlot(required []string, missing []string) []string {
	if len(missing) == 0 {
		return nil
	}
	missingSet := make(map[string]struct{}, len(missing))
	for _, slot := range missing {
		key := strings.TrimSpace(slot)
		if key == "" {
			continue
		}
		missingSet[key] = struct{}{}
	}
	for _, slot := range required {
		key := strings.TrimSpace(slot)
		if key == "" {
			continue
		}
		if _, ok := missingSet[key]; ok {
			return []string{key}
		}
	}
	return []string{strings.TrimSpace(missing[0])}
}

func assistantClarificationDecisionProgressed(prev *assistantClarificationDecision, next *assistantClarificationDecision) bool {
	if prev == nil {
		return false
	}
	if next == nil {
		return true
	}
	if len(next.MissingSlots) < len(prev.MissingSlots) {
		return true
	}
	if len(prev.CandidateIDs) > 1 && len(next.CandidateIDs) == 1 {
		return true
	}
	if len(prev.CandidateActionIDs) > 1 && len(next.CandidateActionIDs) == 1 {
		return true
	}
	return false
}

func assistantClarificationExitForKind(kind string) string {
	switch strings.TrimSpace(kind) {
	case assistantClarificationKindMissingSlots, assistantClarificationKindCandidatePick, assistantClarificationKindCandidateConfirm, assistantClarificationKindIntentDisambiguate, assistantClarificationKindFormatConfirm:
		return assistantClarificationExitBusinessResume
	default:
		return assistantClarificationExitUncertain
	}
}

func assistantFinalizeClarificationDecision(decision *assistantClarificationDecision, input assistantClarificationBuildInput) *assistantClarificationDecision {
	if decision == nil {
		return nil
	}
	decision.ClarificationKind = strings.TrimSpace(decision.ClarificationKind)
	decision.AwaitPhase = assistantClarificationKindAwaitPhase(decision.ClarificationKind)
	decision.MaxRounds = assistantClarificationKindMaxRounds(decision.ClarificationKind)
	if decision.MaxRounds <= 0 {
		decision.MaxRounds = 1
	}
	if decision.CurrentRound <= 0 {
		decision.CurrentRound = 1
	}
	if strings.TrimSpace(decision.Status) == "" {
		decision.Status = assistantClarificationStatusOpen
	}
	decision.RequiredSlots = assistantNormalizeRouteStringSlice(decision.RequiredSlots)
	decision.MissingSlots = assistantNormalizeRouteStringSlice(decision.MissingSlots)
	decision.CandidateActionIDs = assistantNormalizeRouteStringSlice(decision.CandidateActionIDs)
	decision.CandidateIDs = assistantNormalizeRouteStringSlice(decision.CandidateIDs)
	decision.ReasonCodes = assistantNormalizeRouteStringSlice(decision.ReasonCodes)
	if strings.TrimSpace(decision.ExitTo) == "" {
		decision.ExitTo = assistantClarificationExitForKind(decision.ClarificationKind)
	}
	decision.KnowledgeSnapshotDigest = strings.TrimSpace(input.RouteDecision.KnowledgeSnapshotDigest)
	decision.RouteCatalogVersion = strings.TrimSpace(input.RouteDecision.RouteCatalogVersion)
	if strings.TrimSpace(decision.PromptTemplateID) == "" {
		decision.PromptTemplateID = assistantClarificationPromptTemplate(input.Runtime, input.RouteDecision, input.Intent)
	}

	prev := input.PendingClarification
	if prev != nil && strings.TrimSpace(prev.Status) == assistantClarificationStatusOpen {
		progressed := input.ResumeProgress || assistantClarificationDecisionProgressed(prev, decision)
		if strings.TrimSpace(prev.ClarificationKind) == strings.TrimSpace(decision.ClarificationKind) {
			baseRound := prev.CurrentRound
			if baseRound <= 0 {
				baseRound = 1
			}
			if progressed {
				decision.CurrentRound = baseRound
			} else {
				decision.CurrentRound = baseRound + 1
				decision.ReasonCodes = assistantNormalizeRouteStringSlice(append(decision.ReasonCodes, assistantClarificationReasonNoProgress))
				if assistantSliceHas(prev.ReasonCodes, assistantClarificationReasonNoProgress) {
					decision.Status = assistantClarificationStatusAborted
					decision.ExitTo = assistantClarificationExitManualHint
				}
			}
		}
	}
	if decision.CurrentRound > decision.MaxRounds {
		decision.Status = assistantClarificationStatusExhausted
		decision.ExitTo = assistantClarificationExitUncertain
		decision.ReasonCodes = assistantNormalizeRouteStringSlice(append(decision.ReasonCodes, assistantClarificationReasonRoundsExhausted))
	}
	return decision
}

func assistantBuildClarificationDecision(input assistantClarificationBuildInput) *assistantClarificationDecision {
	validationErrors := assistantNormalizeValidationErrors(input.DryRun.ValidationErrors)
	missingSlots := assistantClarificationMissingSlotsFromValidation(validationErrors)
	requiredSlots := assistantClarificationRequiredSlots(input.Runtime, input.RouteDecision, input.Intent)
	selectedCandidateID := strings.TrimSpace(firstNonEmpty(input.SelectedCandidateID, input.ResolvedCandidateID))
	actionStable := assistantClarificationActionStable(input.Intent, input.RouteDecision)
	actionCandidates := assistantClarificationActionCandidates(input.UserInput, input.RouteDecision, input.Intent)
	routeKind := strings.TrimSpace(input.RouteDecision.RouteKind)
	if routeKind == "" {
		routeKind = strings.TrimSpace(input.Intent.RouteKind)
	}

	if (!actionStable && (input.RouteDecision.ClarificationRequired || len(actionCandidates) > 1 || routeKind == assistantRouteKindUncertain)) || len(actionCandidates) > 1 {
		decision := &assistantClarificationDecision{
			ClarificationKind:  assistantClarificationKindIntentDisambiguate,
			PromptTemplateID:   assistantClarificationPromptTemplate(input.Runtime, input.RouteDecision, input.Intent),
			RequiredSlots:      requiredSlots,
			CandidateActionIDs: actionCandidates,
			ReasonCodes:        []string{assistantClarificationReasonIntentDisambiguationRequired},
		}
		if len(decision.CandidateActionIDs) > 2 {
			decision.CandidateActionIDs = decision.CandidateActionIDs[:2]
		}
		return assistantFinalizeClarificationDecision(decision, input)
	}

	if actionStable && assistantSliceHas(validationErrors, "candidate_confirmation_required") && len(input.Candidates) > 1 && selectedCandidateID == "" {
		candidateIDs := make([]string, 0, len(input.Candidates))
		for _, candidate := range input.Candidates {
			if id := strings.TrimSpace(candidate.CandidateID); id != "" {
				candidateIDs = append(candidateIDs, id)
			}
		}
		decision := &assistantClarificationDecision{
			ClarificationKind: assistantClarificationKindCandidatePick,
			PromptTemplateID:  assistantClarificationPromptTemplate(input.Runtime, input.RouteDecision, input.Intent),
			RequiredSlots:     requiredSlots,
			CandidateIDs:      candidateIDs,
			ReasonCodes:       []string{assistantClarificationReasonCandidatePickRequired},
		}
		return assistantFinalizeClarificationDecision(decision, input)
	}

	if actionStable && assistantSliceHas(validationErrors, "candidate_confirmation_required") && selectedCandidateID != "" {
		decision := &assistantClarificationDecision{
			ClarificationKind: assistantClarificationKindCandidateConfirm,
			PromptTemplateID:  assistantClarificationPromptTemplate(input.Runtime, input.RouteDecision, input.Intent),
			RequiredSlots:     requiredSlots,
			CandidateIDs:      []string{selectedCandidateID},
			ReasonCodes:       []string{assistantClarificationReasonCandidateConfirmRequired},
		}
		return assistantFinalizeClarificationDecision(decision, input)
	}

	if actionStable {
		formatMissing := make([]string, 0, 2)
		if assistantSliceHas(validationErrors, "invalid_effective_date_format") {
			formatMissing = append(formatMissing, "effective_date")
		}
		if assistantSliceHas(validationErrors, "invalid_target_effective_date_format") {
			formatMissing = append(formatMissing, "target_effective_date")
		}
		if len(formatMissing) == 0 && assistantContainsRelativeDateToken(input.UserInput) && !assistantDateISOYMD(input.Intent.EffectiveDate) {
			formatMissing = append(formatMissing, "effective_date")
		}
		if len(formatMissing) > 0 {
			decision := &assistantClarificationDecision{
				ClarificationKind: assistantClarificationKindFormatConfirm,
				PromptTemplateID:  assistantClarificationPromptTemplate(input.Runtime, input.RouteDecision, input.Intent),
				RequiredSlots:     requiredSlots,
				MissingSlots:      assistantClarificationCurrentSlot(requiredSlots, formatMissing),
				ReasonCodes:       []string{assistantClarificationReasonDateFormatRequired},
			}
			return assistantFinalizeClarificationDecision(decision, input)
		}
	}

	if actionStable && len(missingSlots) > 0 {
		decision := &assistantClarificationDecision{
			ClarificationKind: assistantClarificationKindMissingSlots,
			PromptTemplateID:  assistantClarificationPromptTemplate(input.Runtime, input.RouteDecision, input.Intent),
			RequiredSlots:     requiredSlots,
			MissingSlots:      assistantClarificationCurrentSlot(requiredSlots, missingSlots),
			ReasonCodes:       []string{assistantClarificationReasonMissingRequiredSlot},
		}
		return assistantFinalizeClarificationDecision(decision, input)
	}
	return nil
}

func assistantDateISOYMD(value string) bool {
	text := strings.TrimSpace(value)
	if text == "" {
		return false
	}
	parsed, err := time.Parse("2006-01-02", text)
	return err == nil && parsed.Format("2006-01-02") == text
}

func assistantContainsRelativeDateToken(input string) bool {
	text := strings.TrimSpace(input)
	if text == "" {
		return false
	}
	return strings.Contains(text, "今天") ||
		strings.Contains(text, "明天") ||
		strings.Contains(text, "后天") ||
		strings.Contains(text, "下个月一号") ||
		strings.Contains(text, "下个月1号") ||
		strings.Contains(text, "下月一号") ||
		strings.Contains(text, "下月1号")
}

func assistantNormalizeDateFromInput(input string, now time.Time) (string, bool) {
	if iso := strings.TrimSpace(assistantDateISORE.FindString(strings.TrimSpace(input))); iso != "" && assistantDateISOYMD(iso) {
		return iso, true
	}
	text := strings.TrimSpace(input)
	if text == "" {
		return "", false
	}
	base := now.UTC()
	if base.IsZero() {
		base = time.Now().UTC()
	}
	switch {
	case strings.Contains(text, "今天"):
		return base.Format("2006-01-02"), true
	case strings.Contains(text, "明天"):
		return base.AddDate(0, 0, 1).Format("2006-01-02"), true
	case strings.Contains(text, "后天"):
		return base.AddDate(0, 0, 2).Format("2006-01-02"), true
	case strings.Contains(text, "下个月一号"), strings.Contains(text, "下个月1号"), strings.Contains(text, "下月一号"), strings.Contains(text, "下月1号"):
		first := time.Date(base.Year(), base.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)
		return first.Format("2006-01-02"), true
	default:
		return "", false
	}
}

func assistantResolveCandidateSelection(input string, candidates []assistantCandidate) (string, bool) {
	text := strings.TrimSpace(strings.ToLower(input))
	if text == "" || len(candidates) == 0 {
		return "", false
	}
	for _, candidate := range candidates {
		id := strings.TrimSpace(candidate.CandidateID)
		if id != "" && strings.EqualFold(id, strings.TrimSpace(input)) {
			return id, true
		}
		code := strings.TrimSpace(candidate.CandidateCode)
		if code != "" && strings.EqualFold(code, strings.TrimSpace(input)) {
			return id, true
		}
	}
	matchID := ""
	for _, candidate := range candidates {
		name := strings.TrimSpace(strings.ToLower(candidate.Name))
		if name == "" {
			continue
		}
		if strings.Contains(text, name) {
			if matchID != "" && matchID != strings.TrimSpace(candidate.CandidateID) {
				return "", false
			}
			matchID = strings.TrimSpace(candidate.CandidateID)
		}
	}
	if matchID != "" {
		return matchID, true
	}
	return "", false
}

func assistantParseCandidateConfirmation(input string) (confirmed bool, denied bool) {
	text := strings.TrimSpace(strings.ToLower(input))
	if text == "" {
		return false, false
	}
	if strings.Contains(text, "不是") || strings.Contains(text, "不对") || strings.Contains(text, "否") || strings.Contains(text, "no") {
		return false, true
	}
	if strings.Contains(text, "确认") || strings.Contains(text, "是") || strings.Contains(text, "好的") || strings.Contains(text, "yes") || strings.Contains(text, "ok") {
		return true, false
	}
	return false, false
}

func assistantResumeFromClarification(pending *assistantTurn, userInput string, intent assistantIntentSpec) assistantClarificationResumeResult {
	result := assistantClarificationResumeResult{Intent: intent}
	open := assistantTurnOpenClarification(pending)
	if open == nil {
		return result
	}
	switch strings.TrimSpace(open.ClarificationKind) {
	case assistantClarificationKindIntentDisambiguate:
		candidates := assistantClarificationActionCandidates(userInput, assistantIntentRouteDecision{}, assistantIntentSpec{})
		if len(candidates) == 1 {
			result.Intent.Action = candidates[0]
			result.Progress = true
		}
	case assistantClarificationKindCandidatePick:
		if id, ok := assistantResolveCandidateSelection(userInput, pending.Candidates); ok {
			result.ResolvedCandidateID = id
			result.SelectedCandidateID = id
			result.Progress = true
		}
	case assistantClarificationKindCandidateConfirm:
		confirmed, denied := assistantParseCandidateConfirmation(userInput)
		if confirmed && strings.TrimSpace(firstNonEmpty(pending.SelectedCandidateID, pending.ResolvedCandidateID)) != "" {
			id := firstNonEmpty(pending.SelectedCandidateID, pending.ResolvedCandidateID)
			result.ResolvedCandidateID = strings.TrimSpace(id)
			result.SelectedCandidateID = strings.TrimSpace(id)
			result.Progress = true
		}
		if denied {
			result.Progress = false
		}
	case assistantClarificationKindFormatConfirm:
		if normalized, ok := assistantNormalizeDateFromInput(userInput, time.Now().UTC()); ok {
			field := "effective_date"
			if len(open.MissingSlots) > 0 {
				field = strings.TrimSpace(open.MissingSlots[0])
			}
			if field == "target_effective_date" {
				result.Intent.TargetEffectiveDate = normalized
			} else {
				result.Intent.EffectiveDate = normalized
			}
			result.Progress = true
		}
	case assistantClarificationKindMissingSlots:
		missing := open.MissingSlots
		if len(missing) == 0 {
			missing = assistantTurnMissingFields(pending)
		}
		next := result.Intent
		for _, slot := range missing {
			switch strings.TrimSpace(slot) {
			case "effective_date":
				if assistantDateISOYMD(next.EffectiveDate) {
					result.Progress = true
				} else if normalized, ok := assistantNormalizeDateFromInput(userInput, time.Now().UTC()); ok {
					next.EffectiveDate = normalized
					result.Progress = true
				}
			case "target_effective_date":
				if assistantDateISOYMD(next.TargetEffectiveDate) {
					result.Progress = true
				} else if normalized, ok := assistantNormalizeDateFromInput(userInput, time.Now().UTC()); ok {
					next.TargetEffectiveDate = normalized
					result.Progress = true
				}
			case "entity_name":
				if strings.TrimSpace(next.EntityName) != "" {
					result.Progress = true
				}
			case "parent_ref_text":
				if strings.TrimSpace(next.ParentRefText) != "" {
					result.Progress = true
				}
			case "new_parent_ref_text":
				if strings.TrimSpace(next.NewParentRefText) != "" {
					result.Progress = true
				}
			case "org_code":
				if strings.TrimSpace(next.OrgCode) != "" {
					result.Progress = true
				}
			case "new_name":
				if strings.TrimSpace(next.NewName) != "" {
					result.Progress = true
				}
			case "change_fields":
				if strings.TrimSpace(next.NewName) != "" || strings.TrimSpace(next.NewParentRefText) != "" {
					result.Progress = true
				}
			}
		}
		result.Intent = next
	}
	return result
}

func assistantValidateClarificationDecision(decision *assistantClarificationDecision) error {
	if decision == nil || !assistantClarificationDecisionPresent(decision) {
		return nil
	}
	status := strings.TrimSpace(decision.Status)
	switch status {
	case assistantClarificationStatusOpen, assistantClarificationStatusResolved, assistantClarificationStatusExhausted, assistantClarificationStatusAborted:
	default:
		return errAssistantClarificationRuntimeInvalid
	}
	kind := strings.TrimSpace(decision.ClarificationKind)
	switch kind {
	case assistantClarificationKindMissingSlots, assistantClarificationKindCandidatePick, assistantClarificationKindCandidateConfirm, assistantClarificationKindIntentDisambiguate, assistantClarificationKindFormatConfirm:
	default:
		return errAssistantClarificationRuntimeInvalid
	}
	if status == assistantClarificationStatusOpen {
		if strings.TrimSpace(decision.AwaitPhase) != assistantClarificationKindAwaitPhase(kind) {
			return errAssistantClarificationRuntimeInvalid
		}
		if decision.MaxRounds <= 0 || decision.CurrentRound <= 0 || decision.CurrentRound > decision.MaxRounds {
			return errAssistantClarificationRuntimeInvalid
		}
		if strings.TrimSpace(decision.ExitTo) == "" || strings.TrimSpace(decision.KnowledgeSnapshotDigest) == "" || strings.TrimSpace(decision.RouteCatalogVersion) == "" {
			return errAssistantClarificationRuntimeInvalid
		}
	}
	return nil
}

func assistantValidateTurnClarificationRuntime(turn *assistantTurn) error {
	if turn == nil {
		return nil
	}
	if err := assistantValidateClarificationDecision(turn.Clarification); err != nil {
		return err
	}
	open := assistantTurnOpenClarification(turn)
	if open == nil {
		return nil
	}
	expectedPhase := assistantClarificationKindAwaitPhase(open.ClarificationKind)
	if currentPhase := strings.TrimSpace(turn.Phase); currentPhase != "" && currentPhase != expectedPhase {
		return errAssistantClarificationRuntimeInvalid
	}
	if !assistantIntentRouteDecisionPresent(turn.RouteDecision) {
		return errAssistantClarificationRuntimeInvalid
	}
	return nil
}

func assistantSliceHas(values []string, needle string) bool {
	target := strings.TrimSpace(needle)
	if target == "" {
		return false
	}
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

func assistantApplyPlanKnowledgeSnapshot(plan *assistantPlanSummary, routeDecision assistantIntentRouteDecision, runtime *assistantKnowledgeRuntime) {
	if plan == nil {
		return
	}
	plan.KnowledgeSnapshotDigest = strings.TrimSpace(routeDecision.KnowledgeSnapshotDigest)
	plan.RouteCatalogVersion = strings.TrimSpace(routeDecision.RouteCatalogVersion)
	plan.ResolverContractVersion = strings.TrimSpace(routeDecision.ResolverContractVersion)
	if runtime != nil {
		plan.ContextTemplateVersion = strings.TrimSpace(runtime.ContextTemplateVersion)
		plan.ReplyGuidanceVersion = strings.TrimSpace(runtime.ReplyGuidanceVersion)
	}
}
