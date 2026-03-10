package server

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
)

type assistantActionStage string

const (
	assistantActionStagePlan    assistantActionStage = "plan"
	assistantActionStageConfirm assistantActionStage = "confirm"
	assistantActionStageCommit  assistantActionStage = "commit"
)

type assistantActionGateInput struct {
	Stage      assistantActionStage
	TenantID   string
	Principal  Principal
	Action     assistantActionSpec
	Intent     assistantIntentSpec
	Turn       *assistantTurn
	Candidates []assistantCandidate
	ResolvedID string
	DryRun     *assistantDryRunResult
	UserInput  string
}

type assistantActionGateDecision struct {
	Allowed    bool
	Error      error
	ErrorCode  string
	HTTPStatus int
	ReasonCode string
}

var assistantLoadAuthorizerFn = func() (authorizer, error) {
	return loadAuthorizer()
}

func assistantEvaluateActionGate(input assistantActionGateInput) assistantActionGateDecision {
	if strings.TrimSpace(input.Action.ID) == "" {
		return assistantActionGateDecision{Allowed: false, Error: errAssistantActionSpecMissing, ErrorCode: errAssistantActionSpecMissing.Error(), HTTPStatus: http.StatusUnprocessableEntity, ReasonCode: "action_spec_missing"}
	}
	if decision := assistantCheckCapabilityRegistered(input.Action); !decision.Allowed {
		return decision
	}
	if decision := assistantCheckActionAuthz(input); !decision.Allowed {
		return decision
	}
	if decision := assistantCheckRiskGate(input); !decision.Allowed {
		return decision
	}
	if input.Stage == assistantActionStageConfirm {
		if decision := assistantCheckConfirmRequirements(input); !decision.Allowed {
			return decision
		}
	}
	if input.Stage == assistantActionStageCommit {
		if decision := assistantCheckRequiredChecks(input); !decision.Allowed {
			return decision
		}
	}
	return assistantActionGateDecision{Allowed: true}
}

func assistantCheckCapabilityRegistered(action assistantActionSpec) assistantActionGateDecision {
	definition, ok := capabilityDefinitionForKey(action.CapabilityKey)
	if !ok {
		return assistantActionGateDecision{Allowed: false, Error: errAssistantActionCapabilityUnregistered, ErrorCode: errAssistantActionCapabilityUnregistered.Error(), HTTPStatus: http.StatusUnprocessableEntity, ReasonCode: "capability_unregistered"}
	}
	if strings.TrimSpace(definition.Status) != routeCapabilityStatusActive || strings.TrimSpace(definition.ActivationState) != routeCapabilityStatusActive {
		return assistantActionGateDecision{Allowed: false, Error: errAssistantActionCapabilityUnregistered, ErrorCode: errAssistantActionCapabilityUnregistered.Error(), HTTPStatus: http.StatusUnprocessableEntity, ReasonCode: "capability_inactive"}
	}
	return assistantActionGateDecision{Allowed: true}
}

func assistantCheckActionAuthz(input assistantActionGateInput) assistantActionGateDecision {
	authorizer, err := assistantLoadAuthorizerFn()
	if err != nil {
		return assistantActionGateDecision{Allowed: false, Error: errAssistantServiceMissing, ErrorCode: errAssistantServiceMissing.Error(), HTTPStatus: http.StatusInternalServerError, ReasonCode: "action_authz_unavailable"}
	}
	subject := authz.SubjectFromRoleSlug(input.Principal.RoleSlug)
	domain := authz.DomainFromTenantID(input.TenantID)
	object := strings.TrimSpace(input.Action.Security.AuthObject)
	action := strings.TrimSpace(input.Action.Security.AuthAction)
	allowed, enforced, err := authorizer.Authorize(subject, domain, object, action)
	if err != nil {
		return assistantActionGateDecision{Allowed: false, Error: errAssistantServiceMissing, ErrorCode: errAssistantServiceMissing.Error(), HTTPStatus: http.StatusInternalServerError, ReasonCode: "action_authz_error"}
	}
	if enforced && !allowed {
		return assistantActionGateDecision{Allowed: false, Error: errAssistantActionAuthzDenied, ErrorCode: errAssistantActionAuthzDenied.Error(), HTTPStatus: http.StatusForbidden, ReasonCode: "action_authz_denied"}
	}
	return assistantActionGateDecision{Allowed: true}
}

func assistantCheckRiskGate(input assistantActionGateInput) assistantActionGateDecision {
	riskTier := strings.ToLower(strings.TrimSpace(input.Action.Security.RiskTier))
	switch riskTier {
	case "", "low", "medium", "high":
	default:
		return assistantActionGateDecision{Allowed: false, Error: errAssistantActionRiskGateDenied, ErrorCode: errAssistantActionRiskGateDenied.Error(), HTTPStatus: http.StatusConflict, ReasonCode: "risk_tier_invalid"}
	}
	if input.Stage == assistantActionStageCommit && riskTier == "high" && input.Turn != nil && strings.TrimSpace(input.Turn.State) != assistantStateConfirmed {
		return assistantActionGateDecision{Allowed: false, Error: errAssistantActionRiskGateDenied, ErrorCode: errAssistantActionRiskGateDenied.Error(), HTTPStatus: http.StatusConflict, ReasonCode: "high_risk_commit_requires_confirmation"}
	}
	return assistantActionGateDecision{Allowed: true}
}

func assistantCheckRequiredChecks(input assistantActionGateInput) assistantActionGateDecision {
	for _, check := range input.Action.Security.RequiredChecks {
		switch strings.TrimSpace(check) {
		case "":
			continue
		case "strict_decode":
			if input.Stage != assistantActionStagePlan {
				continue
			}
			if assistantIntentSchemaInvalid(input.Intent) {
				return assistantRequiredCheckDenied("strict_decode_failed")
			}
		case "boundary_lint":
			if input.Stage != assistantActionStagePlan {
				continue
			}
			if input.UserInput != "" && assistantBoundaryViolationDetected(input.UserInput) {
				return assistantRequiredCheckDenied("boundary_lint_failed")
			}
		case "candidate_confirmation":
			if assistantIntentRequiresCandidateConfirmation(input.Intent) && strings.TrimSpace(input.ResolvedID) == "" {
				if input.Stage == assistantActionStageCommit {
					return assistantActionGateDecision{Allowed: false, Error: errAssistantCandidateNotFound, ErrorCode: errAssistantCandidateNotFound.Error(), HTTPStatus: http.StatusUnprocessableEntity, ReasonCode: "candidate_missing_at_commit"}
				}
				return assistantActionGateDecision{Allowed: false, Error: errAssistantConfirmationRequired, ErrorCode: errAssistantConfirmationRequired.Error(), HTTPStatus: http.StatusConflict, ReasonCode: "candidate_confirmation_required"}
			}
		case "dry_run":
			if input.DryRun == nil || len(assistantDryRunValidationErrorsForGate(input)) > 0 {
				return assistantRequiredCheckDenied("dry_run_validation_failed")
			}
		default:
			return assistantRequiredCheckDenied("required_check_unknown")
		}
	}
	return assistantActionGateDecision{Allowed: true}
}

func assistantCheckConfirmRequirements(input assistantActionGateInput) assistantActionGateDecision {
	if assistantIntentRequiresCandidateConfirmation(input.Intent) && strings.TrimSpace(input.ResolvedID) == "" {
		return assistantActionGateDecision{Allowed: false, Error: errAssistantConfirmationRequired, ErrorCode: errAssistantConfirmationRequired.Error(), HTTPStatus: http.StatusConflict, ReasonCode: "candidate_confirmation_required"}
	}
	return assistantActionGateDecision{Allowed: true}
}

func assistantIntentRequiresCandidateConfirmation(intent assistantIntentSpec) bool {
	switch strings.TrimSpace(intent.Action) {
	case assistantIntentCreateOrgUnit:
		return true
	case assistantIntentAddOrgUnitVersion, assistantIntentInsertOrgUnitVersion, assistantIntentCorrectOrgUnit, assistantIntentMoveOrgUnit:
		return strings.TrimSpace(intent.NewParentRefText) != ""
	default:
		return false
	}
}

func assistantRequiredCheckDenied(reason string) assistantActionGateDecision {
	return assistantActionGateDecision{Allowed: false, Error: errAssistantActionRequiredCheckFailed, ErrorCode: errAssistantActionRequiredCheckFailed.Error(), HTTPStatus: http.StatusConflict, ReasonCode: strings.TrimSpace(reason)}
}

func assistantDryRunValidationErrorsForGate(input assistantActionGateInput) []string {
	if input.DryRun == nil {
		return []string{"dry_run_missing"}
	}
	if assistantDryRunMissing(*input.DryRun) {
		turnState := ""
		if input.Turn != nil {
			turnState = input.Turn.State
		}
		*input.DryRun = assistantHydratedDryRunForGate(turnState, input.Intent, input.Candidates, input.ResolvedID)
	}
	errorsList := assistantNormalizeValidationErrors(input.DryRun.ValidationErrors)
	if strings.TrimSpace(input.ResolvedID) == "" {
		return errorsList
	}
	filtered := make([]string, 0, len(errorsList))
	for _, code := range errorsList {
		if code == "candidate_confirmation_required" {
			continue
		}
		filtered = append(filtered, code)
	}
	return filtered
}

func assistantDryRunMissing(dryRun assistantDryRunResult) bool {
	return len(dryRun.Diff) == 0 && strings.TrimSpace(dryRun.Explain) == "" && len(dryRun.ValidationErrors) == 0 && !dryRun.WouldCommit && strings.TrimSpace(dryRun.PlanHash) == ""
}

func assistantHydratedDryRunForGate(turnState string, intent assistantIntentSpec, candidates []assistantCandidate, resolvedID string) assistantDryRunResult {
	if strings.TrimSpace(turnState) == assistantStateConfirmed && strings.TrimSpace(resolvedID) != "" {
		return assistantDryRunResult{Explain: "计划已确认，等待提交"}
	}
	return assistantBuildDryRunFn(intent, candidates, resolvedID)
}

func assistantHydrateTurnForActionGate(turn *assistantTurn) {
	if turn == nil || !assistantDryRunMissing(turn.DryRun) {
		return
	}
	turn.DryRun = assistantHydratedDryRunForGate(turn.State, turn.Intent, turn.Candidates, turn.ResolvedCandidateID)
}

func assistantApplyGateDecision(conversation *assistantConversation, turn *assistantTurn, principal Principal, turnAction string, decision assistantActionGateDecision) (assistantTurnMutationResult, error) {
	if decision.Allowed {
		return assistantTurnMutationResult{}, nil
	}
	if conversation == nil || turn == nil {
		if decision.Error != nil {
			return assistantTurnMutationResult{}, decision.Error
		}
		return assistantTurnMutationResult{}, errors.New(strings.TrimSpace(decision.ErrorCode))
	}
	fromState := turn.State
	fromPhase := turn.Phase
	now := time.Now().UTC()
	if !(turnAction == "confirm" && errors.Is(decision.Error, errAssistantConfirmationRequired)) {
		turn.ErrorCode = strings.TrimSpace(decision.ErrorCode)
	}
	turn.UpdatedAt = now
	assistantRefreshTurnDerivedFields(turn)
	conversation.UpdatedAt = now
	conversation.State = turn.State
	conversation.CurrentPhase = turn.Phase
	transition := &assistantStateTransition{
		TurnID:     turn.TurnID,
		TurnAction: turnAction,
		RequestID:  turn.RequestID,
		TraceID:    turn.TraceID,
		FromState:  fromState,
		ToState:    turn.State,
		FromPhase:  fromPhase,
		ToPhase:    turn.Phase,
		ReasonCode: strings.TrimSpace(decision.ReasonCode),
		ActorID:    principal.ID,
		ChangedAt:  now,
	}
	conversation.Transitions = append(conversation.Transitions, *transition)
	if decision.Error != nil {
		return assistantTurnMutationResult{Transition: transition, PersistTurn: true}, decision.Error
	}
	return assistantTurnMutationResult{Transition: transition, PersistTurn: true}, errors.New(strings.TrimSpace(decision.ErrorCode))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
