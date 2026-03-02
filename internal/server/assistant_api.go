package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
)

const (
	assistantStateDraft     = "draft"
	assistantStateProposed  = "proposed"
	assistantStateValidated = "validated"
	assistantStateConfirmed = "confirmed"
	assistantStateCommitted = "committed"

	assistantResolutionAuto          = "auto"
	assistantResolutionUserConfirmed = "user_confirmed"

	assistantIntentCreateOrgUnit = "create_orgunit"
)

var (
	assistantParentUnderRE = regexp.MustCompile(`在(.+?)之下`)
	assistantDeptNameRE    = regexp.MustCompile(`名为(.+?)的部门`)
	assistantDateCNRE      = regexp.MustCompile(`(20\d{2})年(\d{1,2})月(\d{1,2})日`)
	assistantDateISORE     = regexp.MustCompile(`(20\d{2}-\d{2}-\d{2})`)
)

type assistantConversationService struct {
	orgStore  OrgUnitStore
	writeSvc  orgunitservices.OrgUnitWriteService
	mu        sync.RWMutex
	byID      map[string]*assistantConversation
	byActorID map[string][]string
}

type assistantConversation struct {
	ConversationID string           `json:"conversation_id"`
	TenantID       string           `json:"tenant_id"`
	ActorID        string           `json:"actor_id"`
	ActorRole      string           `json:"actor_role"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
	Turns          []*assistantTurn `json:"turns"`
}

type assistantTurn struct {
	TurnID              string                 `json:"turn_id"`
	UserInput           string                 `json:"user_input"`
	State               string                 `json:"state"`
	RiskTier            string                 `json:"risk_tier"`
	RequestID           string                 `json:"request_id"`
	TraceID             string                 `json:"trace_id"`
	PolicyVersion       string                 `json:"policy_version"`
	CompositionVersion  string                 `json:"composition_version"`
	MappingVersion      string                 `json:"mapping_version"`
	Intent              assistantIntentSpec    `json:"intent"`
	Plan                assistantPlanSummary   `json:"plan"`
	Candidates          []assistantCandidate   `json:"candidates"`
	ResolvedCandidateID string                 `json:"resolved_candidate_id,omitempty"`
	AmbiguityCount      int                    `json:"ambiguity_count"`
	Confidence          float64                `json:"confidence"`
	ResolutionSource    string                 `json:"resolution_source,omitempty"`
	DryRun              assistantDryRunResult  `json:"dry_run"`
	CommitResult        *assistantCommitResult `json:"commit_result,omitempty"`
	CreatedAt           time.Time              `json:"created_at"`
	UpdatedAt           time.Time              `json:"updated_at"`
}

type assistantIntentSpec struct {
	Action        string `json:"action"`
	ParentRefText string `json:"parent_ref_text,omitempty"`
	EntityName    string `json:"entity_name,omitempty"`
	EffectiveDate string `json:"effective_date,omitempty"`
}

type assistantPlanSummary struct {
	Title         string `json:"title"`
	CapabilityKey string `json:"capability_key"`
	Summary       string `json:"summary"`
}

type assistantDryRunResult struct {
	Diff    []map[string]any `json:"diff"`
	Explain string           `json:"explain"`
}

type assistantCandidate struct {
	CandidateID   string  `json:"candidate_id"`
	CandidateCode string  `json:"candidate_code"`
	Name          string  `json:"name"`
	Path          string  `json:"path"`
	AsOf          string  `json:"as_of"`
	IsActive      bool    `json:"is_active"`
	MatchScore    float64 `json:"match_score"`
}

type assistantCommitResult struct {
	OrgCode       string `json:"org_code"`
	ParentOrgCode string `json:"parent_org_code"`
	EffectiveDate string `json:"effective_date"`
	EventType     string `json:"event_type"`
	EventUUID     string `json:"event_uuid"`
}

type assistantCreateConversationRequest struct {
	Title string `json:"title"`
}

type assistantCreateTurnRequest struct {
	UserInput string `json:"user_input"`
}

type assistantConfirmRequest struct {
	CandidateID string `json:"candidate_id"`
}

func newAssistantConversationService(orgStore OrgUnitStore, writeSvc orgunitservices.OrgUnitWriteService) *assistantConversationService {
	return &assistantConversationService{
		orgStore:  orgStore,
		writeSvc:  writeSvc,
		byID:      make(map[string]*assistantConversation),
		byActorID: make(map[string][]string),
	}
}

func handleAssistantConversationsAPI(w http.ResponseWriter, r *http.Request, svc *assistantConversationService) {
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if svc == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "assistant_service_missing", "assistant service missing")
		return
	}
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	principal, ok := currentPrincipal(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	var req assistantCreateConversationRequest
	if hasRequestBody(r) {
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
			return
		}
	}

	conversation := svc.createConversation(tenant.ID, principal)
	writeJSON(w, http.StatusOK, conversation)
}

func handleAssistantConversationDetailAPI(w http.ResponseWriter, r *http.Request, svc *assistantConversationService) {
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if svc == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "assistant_service_missing", "assistant service missing")
		return
	}
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	principal, ok := currentPrincipal(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	conversationID, ok := extractConversationIDFromPath(r.URL.Path)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "invalid conversation path")
		return
	}
	conversation, err := svc.getConversation(tenant.ID, principal.ID, conversationID)
	if err != nil {
		if errors.Is(err, errAssistantConversationNotFound) {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "conversation_not_found", "conversation not found")
			return
		}
		if errors.Is(err, errAssistantConversationForbidden) {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "forbidden", "forbidden")
			return
		}
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "assistant_conversation_load_failed", "assistant conversation load failed")
		return
	}

	writeJSON(w, http.StatusOK, conversation)
}

func handleAssistantConversationTurnsAPI(w http.ResponseWriter, r *http.Request, svc *assistantConversationService) {
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if svc == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "assistant_service_missing", "assistant service missing")
		return
	}
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	principal, ok := currentPrincipal(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	conversationID, ok := extractConversationTurnsPathConversationID(r.URL.Path)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "invalid turns path")
		return
	}

	var req assistantCreateTurnRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
		return
	}
	userInput := strings.TrimSpace(req.UserInput)
	if userInput == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "invalid_request", "user_input required")
		return
	}

	conversation, err := svc.createTurn(r.Context(), tenant.ID, principal, conversationID, userInput)
	if err != nil {
		switch {
		case errors.Is(err, errAssistantConversationNotFound):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "conversation_not_found", "conversation not found")
		case errors.Is(err, errAssistantConversationForbidden):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "forbidden", "forbidden")
		case errors.Is(err, errAssistantIntentDateRequired):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "invalid_effective_date", "effective_date required")
		default:
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "assistant_turn_create_failed", "assistant turn create failed")
		}
		return
	}

	writeJSON(w, http.StatusOK, conversation)
}

func handleAssistantTurnActionAPI(w http.ResponseWriter, r *http.Request, svc *assistantConversationService) {
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if svc == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "assistant_service_missing", "assistant service missing")
		return
	}
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	principal, ok := currentPrincipal(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	conversationID, turnID, action, ok := extractAssistantTurnActionPath(r.URL.Path)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "invalid turn action path")
		return
	}

	switch action {
	case "confirm":
		var req assistantConfirmRequest
		if hasRequestBody(r) {
			dec := json.NewDecoder(r.Body)
			dec.DisallowUnknownFields()
			if err := dec.Decode(&req); err != nil {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
				return
			}
		}
		conversation, err := svc.confirmTurn(tenant.ID, principal, conversationID, turnID, strings.TrimSpace(req.CandidateID))
		if err != nil {
			switch {
			case errors.Is(err, errAssistantConversationNotFound):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "conversation_not_found", "conversation not found")
			case errors.Is(err, errAssistantConversationForbidden):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "forbidden", "forbidden")
			case errors.Is(err, errAssistantTurnNotFound):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "conversation_turn_not_found", "conversation turn not found")
			case errors.Is(err, errAssistantConfirmationRequired):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "conversation_confirmation_required", "conversation confirmation required")
			case errors.Is(err, errAssistantCandidateNotFound):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "assistant_candidate_not_found", "assistant candidate not found")
			default:
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "assistant_turn_confirm_failed", "assistant turn confirm failed")
			}
			return
		}
		writeJSON(w, http.StatusOK, conversation)
		return
	case "commit":
		conversation, err := svc.commitTurn(r.Context(), tenant.ID, principal, conversationID, turnID)
		if err != nil {
			switch {
			case errors.Is(err, errAssistantConversationNotFound):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "conversation_not_found", "conversation not found")
			case errors.Is(err, errAssistantConversationForbidden):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "forbidden", "forbidden")
			case errors.Is(err, errAssistantTurnNotFound):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "conversation_turn_not_found", "conversation turn not found")
			case errors.Is(err, errAssistantConfirmationRequired):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "conversation_confirmation_required", "conversation confirmation required")
			case errors.Is(err, errAssistantAuthSnapshotExpired):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "ai_actor_auth_snapshot_expired", "ai actor auth snapshot expired")
			case errors.Is(err, errAssistantRoleDriftDetected):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "ai_actor_role_drift_detected", "ai actor role drift detected")
			case errors.Is(err, errAssistantUnsupportedIntent):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "assistant_intent_unsupported", "assistant intent unsupported")
			case errors.Is(err, errAssistantServiceMissing):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "orgunit_service_missing", "orgunit service missing")
			case errors.Is(err, errAssistantCandidateNotFound):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "conversation_confirmation_required", "conversation confirmation required")
			default:
				if status, code, message, ok := assistantResolveCommitError(err); ok {
					routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, message)
					return
				}
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "assistant_commit_failed", "assistant commit failed")
			}
			return
		}
		writeJSON(w, http.StatusOK, conversation)
		return
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "assistant action unsupported")
		return
	}
}

func assistantResolveCommitError(err error) (status int, code string, message string, ok bool) {
	code = strings.TrimSpace(stablePgMessage(err))
	if code == "" {
		code = strings.TrimSpace(err.Error())
	}
	if code == "" {
		return 0, "", "", false
	}

	status, known := orgUnitAPIStatusForCode(code)
	if !known {
		if isBadRequestError(err) || isPgInvalidInput(err) {
			return http.StatusBadRequest, "invalid_request", err.Error(), true
		}
		if !isStableDBCode(code) {
			return 0, "", "", false
		}
		status = http.StatusUnprocessableEntity
	}

	message = code
	if mapped := strings.TrimSpace(orgNodeWriteErrorMessage(errors.New(code))); mapped != "" && mapped != code {
		message = mapped
	}
	return status, code, message, true
}

var (
	errAssistantConversationNotFound  = errors.New("assistant_conversation_not_found")
	errAssistantConversationForbidden = errors.New("assistant_conversation_forbidden")
	errAssistantConversationCorrupted = errors.New("assistant_conversation_corrupted")
	errAssistantTurnNotFound          = errors.New("assistant_turn_not_found")
	errAssistantConfirmationRequired  = errors.New("assistant_confirmation_required")
	errAssistantCandidateNotFound     = errors.New("assistant_candidate_not_found")
	errAssistantAuthSnapshotExpired   = errors.New("assistant_auth_snapshot_expired")
	errAssistantRoleDriftDetected     = errors.New("assistant_role_drift_detected")
	errAssistantUnsupportedIntent     = errors.New("assistant_unsupported_intent")
	errAssistantServiceMissing        = errors.New("assistant_service_missing")
	errAssistantIntentDateRequired    = errors.New("assistant_intent_date_required")
)

func (s *assistantConversationService) createConversation(tenantID string, principal Principal) *assistantConversation {
	now := time.Now().UTC()
	conversation := &assistantConversation{
		ConversationID: "conv_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		TenantID:       tenantID,
		ActorID:        principal.ID,
		ActorRole:      strings.TrimSpace(principal.RoleSlug),
		CreatedAt:      now,
		UpdatedAt:      now,
		Turns:          make([]*assistantTurn, 0, 4),
	}

	s.mu.Lock()
	s.byID[conversation.ConversationID] = conversation
	s.byActorID[principal.ID] = append(s.byActorID[principal.ID], conversation.ConversationID)
	s.mu.Unlock()

	return cloneConversation(conversation)
}

func (s *assistantConversationService) getConversation(tenantID string, actorID string, conversationID string) (*assistantConversation, error) {
	s.mu.RLock()
	conversation, ok := s.byID[conversationID]
	s.mu.RUnlock()
	if !ok {
		return nil, errAssistantConversationNotFound
	}
	if conversation == nil {
		return nil, errAssistantConversationCorrupted
	}
	if conversation.TenantID != tenantID || conversation.ActorID != actorID {
		return nil, errAssistantConversationForbidden
	}
	return cloneConversation(conversation), nil
}

func (s *assistantConversationService) createTurn(ctx context.Context, tenantID string, principal Principal, conversationID string, userInput string) (*assistantConversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	conversation, ok := s.byID[conversationID]
	if !ok {
		return nil, errAssistantConversationNotFound
	}
	if conversation.TenantID != tenantID || conversation.ActorID != principal.ID {
		return nil, errAssistantConversationForbidden
	}

	intent := assistantExtractIntent(userInput)
	if intent.Action == assistantIntentCreateOrgUnit && intent.EffectiveDate == "" {
		return nil, errAssistantIntentDateRequired
	}
	candidates := make([]assistantCandidate, 0)
	resolvedCandidateID := ""
	resolutionSource := ""
	ambiguityCount := 0
	confidence := 0.65
	if intent.Action == assistantIntentCreateOrgUnit && intent.ParentRefText != "" {
		resolved, err := s.resolveCandidates(ctx, tenantID, intent.ParentRefText, intent.EffectiveDate)
		if err != nil {
			return nil, err
		}
		candidates = resolved
		ambiguityCount = len(candidates)
		switch len(candidates) {
		case 0:
			confidence = 0.3
		case 1:
			resolvedCandidateID = candidates[0].CandidateID
			resolutionSource = assistantResolutionAuto
			confidence = 0.95
		default:
			confidence = 0.55
		}
	}

	turn := &assistantTurn{
		TurnID:              "turn_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		UserInput:           userInput,
		State:               assistantStateValidated,
		RiskTier:            assistantRiskTierForIntent(intent),
		RequestID:           "assistant_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		TraceID:             strings.ReplaceAll(uuid.NewString(), "-", ""),
		PolicyVersion:       capabilityPolicyVersionBaseline,
		CompositionVersion:  capabilityPolicyVersionBaseline,
		MappingVersion:      capabilityPolicyVersionBaseline,
		Intent:              intent,
		Plan:                assistantBuildPlan(intent),
		Candidates:          candidates,
		ResolvedCandidateID: resolvedCandidateID,
		AmbiguityCount:      ambiguityCount,
		Confidence:          confidence,
		ResolutionSource:    resolutionSource,
		DryRun:              assistantBuildDryRun(intent, candidates, resolvedCandidateID),
		CreatedAt:           time.Now().UTC(),
		UpdatedAt:           time.Now().UTC(),
	}

	conversation.Turns = append(conversation.Turns, turn)
	conversation.UpdatedAt = time.Now().UTC()

	return cloneConversation(conversation), nil
}

func (s *assistantConversationService) confirmTurn(tenantID string, principal Principal, conversationID string, turnID string, candidateID string) (*assistantConversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	conversation, turn, err := s.lookupMutableTurn(tenantID, principal.ID, conversationID, turnID)
	if err != nil {
		return nil, err
	}
	if turn.State == assistantStateCommitted {
		return cloneConversation(conversation), nil
	}
	if turn.State != assistantStateValidated && turn.State != assistantStateConfirmed {
		return nil, errAssistantConfirmationRequired
	}
	if turn.AmbiguityCount > 1 {
		if candidateID == "" {
			return nil, errAssistantConfirmationRequired
		}
		if !assistantCandidateExists(turn.Candidates, candidateID) {
			return nil, errAssistantCandidateNotFound
		}
		turn.ResolvedCandidateID = candidateID
		turn.ResolutionSource = assistantResolutionUserConfirmed
	}
	if turn.Intent.Action == assistantIntentCreateOrgUnit && turn.ResolvedCandidateID == "" {
		return nil, errAssistantConfirmationRequired
	}
	turn.State = assistantStateConfirmed
	turn.UpdatedAt = time.Now().UTC()
	conversation.UpdatedAt = turn.UpdatedAt
	return cloneConversation(conversation), nil
}

func (s *assistantConversationService) commitTurn(ctx context.Context, tenantID string, principal Principal, conversationID string, turnID string) (*assistantConversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	conversation, ok := s.byID[conversationID]
	if !ok {
		return nil, errAssistantConversationNotFound
	}
	if conversation == nil {
		return nil, errAssistantConversationCorrupted
	}
	if conversation.TenantID != tenantID {
		return nil, errAssistantConversationForbidden
	}
	if principal.ID != conversation.ActorID {
		return nil, errAssistantAuthSnapshotExpired
	}
	var turn *assistantTurn
	for _, item := range conversation.Turns {
		if item != nil && item.TurnID == turnID {
			turn = item
			break
		}
	}
	if turn == nil {
		return nil, errAssistantTurnNotFound
	}
	if strings.TrimSpace(principal.RoleSlug) != strings.TrimSpace(conversation.ActorRole) {
		return nil, errAssistantRoleDriftDetected
	}
	if turn.State == assistantStateCommitted {
		return cloneConversation(conversation), nil
	}
	if turn.State != assistantStateConfirmed {
		return nil, errAssistantConfirmationRequired
	}
	if turn.Intent.Action != assistantIntentCreateOrgUnit {
		return nil, errAssistantUnsupportedIntent
	}
	if turn.ResolvedCandidateID == "" {
		return nil, errAssistantCandidateNotFound
	}
	if s.writeSvc == nil {
		return nil, errAssistantServiceMissing
	}

	resolved, ok := assistantFindCandidate(turn.Candidates, turn.ResolvedCandidateID)
	if !ok {
		return nil, errAssistantCandidateNotFound
	}
	name := turn.Intent.EntityName
	if strings.TrimSpace(name) == "" {
		name = "新建组织"
	}
	parentOrgCode := resolved.CandidateCode
	orgCode := assistantGeneratedOrgCode(turn.TurnID)
	result, err := s.writeSvc.Write(ctx, tenantID, orgunitservices.WriteOrgUnitRequest{
		Intent:        string(orgunitservices.OrgUnitWriteIntentCreateOrg),
		OrgCode:       orgCode,
		EffectiveDate: turn.Intent.EffectiveDate,
		PolicyVersion: turn.PolicyVersion,
		RequestID:     turn.RequestID,
		Patch: orgunitservices.OrgUnitWritePatch{
			Name:          ptrString(name),
			ParentOrgCode: ptrString(parentOrgCode),
		},
		InitiatorUUID: principal.ID,
	})
	if err != nil {
		return nil, err
	}

	turn.CommitResult = &assistantCommitResult{
		OrgCode:       result.OrgCode,
		ParentOrgCode: parentOrgCode,
		EffectiveDate: result.EffectiveDate,
		EventType:     result.EventType,
		EventUUID:     result.EventUUID,
	}
	turn.State = assistantStateCommitted
	turn.UpdatedAt = time.Now().UTC()
	conversation.UpdatedAt = turn.UpdatedAt

	return cloneConversation(conversation), nil
}

func (s *assistantConversationService) lookupMutableTurn(tenantID string, actorID string, conversationID string, turnID string) (*assistantConversation, *assistantTurn, error) {
	conversation, ok := s.byID[conversationID]
	if !ok {
		return nil, nil, errAssistantConversationNotFound
	}
	if conversation == nil {
		return nil, nil, errAssistantConversationCorrupted
	}
	if conversation.TenantID != tenantID || conversation.ActorID != actorID {
		return nil, nil, errAssistantConversationForbidden
	}
	for _, turn := range conversation.Turns {
		if turn.TurnID == turnID {
			return conversation, turn, nil
		}
	}
	return nil, nil, errAssistantTurnNotFound
}

func (s *assistantConversationService) resolveCandidates(ctx context.Context, tenantID string, parentRefText string, asOf string) ([]assistantCandidate, error) {
	if s.orgStore == nil {
		return nil, nil
	}
	rows, err := s.orgStore.SearchNodeCandidates(ctx, tenantID, parentRefText, asOf, 10)
	if err != nil {
		return nil, err
	}
	candidates := make([]assistantCandidate, 0, len(rows))
	for _, item := range rows {
		path := item.Name
		if details, detailsErr := s.orgStore.GetNodeDetails(ctx, tenantID, item.OrgID, asOf); detailsErr == nil {
			path = strings.TrimSpace(details.FullNamePath)
			if path == "" {
				path = item.Name
			}
		}
		candidateID := strings.TrimSpace(item.OrgCode)
		if candidateID == "" {
			candidateID = strconv.Itoa(item.OrgID)
		}
		candidates = append(candidates, assistantCandidate{
			CandidateID:   candidateID,
			CandidateCode: strings.TrimSpace(item.OrgCode),
			Name:          strings.TrimSpace(item.Name),
			Path:          path,
			AsOf:          asOf,
			IsActive:      strings.EqualFold(strings.TrimSpace(item.Status), "active"),
			MatchScore:    0.8,
		})
	}
	return candidates, nil
}

func assistantRiskTierForIntent(intent assistantIntentSpec) string {
	switch intent.Action {
	case assistantIntentCreateOrgUnit:
		return "high"
	default:
		return "low"
	}
}

func assistantBuildPlan(intent assistantIntentSpec) assistantPlanSummary {
	summary := "生成只读计划，不执行提交"
	title := "只读规划"
	capabilityKey := "org.orgunit_create.field_policy"
	if intent.Action == assistantIntentCreateOrgUnit {
		title = "创建组织计划"
		summary = "在指定父组织下创建部门，提交前需要确认候选主键"
	}
	return assistantPlanSummary{
		Title:         title,
		CapabilityKey: capabilityKey,
		Summary:       summary,
	}
}

func assistantBuildDryRun(intent assistantIntentSpec, candidates []assistantCandidate, resolvedCandidateID string) assistantDryRunResult {
	diff := make([]map[string]any, 0, 3)
	if intent.Action == assistantIntentCreateOrgUnit {
		diff = append(diff,
			map[string]any{"field": "name", "after": intent.EntityName},
			map[string]any{"field": "effective_date", "after": intent.EffectiveDate},
		)
		if resolvedCandidateID != "" {
			diff = append(diff, map[string]any{"field": "parent_candidate_id", "after": resolvedCandidateID})
		} else if len(candidates) > 1 {
			diff = append(diff, map[string]any{"field": "parent_candidate_id", "after": "pending_confirmation"})
		}
	}
	explain := "计划已生成，等待确认后可提交"
	if intent.Action == assistantIntentCreateOrgUnit && len(candidates) > 1 {
		explain = "检测到多个同名父组织候选，需先确认候选主键"
	}
	return assistantDryRunResult{Diff: diff, Explain: explain}
}

func assistantExtractIntent(input string) assistantIntentSpec {
	text := strings.TrimSpace(input)
	intent := assistantIntentSpec{Action: "plan_only"}
	if strings.Contains(text, "新建") && strings.Contains(text, "部门") {
		intent.Action = assistantIntentCreateOrgUnit
	}
	if m := assistantParentUnderRE.FindStringSubmatch(text); len(m) == 2 {
		intent.ParentRefText = strings.TrimSpace(m[1])
	}
	if m := assistantDeptNameRE.FindStringSubmatch(text); len(m) == 2 {
		intent.EntityName = strings.TrimSpace(m[1])
	}
	if m := assistantDateISORE.FindStringSubmatch(text); len(m) == 2 {
		intent.EffectiveDate = strings.TrimSpace(m[1])
	}
	if intent.EffectiveDate == "" {
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
			intent.EffectiveDate = year + "-" + month + "-" + day
		}
	}
	return intent
}

func assistantCandidateExists(candidates []assistantCandidate, candidateID string) bool {
	for _, candidate := range candidates {
		if candidate.CandidateID == candidateID {
			return true
		}
	}
	return false
}

func assistantFindCandidate(candidates []assistantCandidate, candidateID string) (assistantCandidate, bool) {
	for _, candidate := range candidates {
		if candidate.CandidateID == candidateID {
			return candidate, true
		}
	}
	return assistantCandidate{}, false
}

func assistantGeneratedOrgCode(turnID string) string {
	seed := strings.ToUpper(strings.ReplaceAll(turnID, "-", ""))
	seed = strings.ReplaceAll(seed, "turn_", "")
	if len(seed) > 10 {
		seed = seed[:10]
	}
	if seed == "" {
		seed = "AIDEFAULT"
	}
	return "AI" + seed
}

func extractConversationIDFromPath(path string) (string, bool) {
	parts := assistantSplitPathSegments(path)
	if len(parts) != 4 {
		return "", false
	}
	if parts[0] != "internal" || parts[1] != "assistant" || parts[2] != "conversations" {
		return "", false
	}
	conversationID := strings.TrimSpace(parts[3])
	if conversationID == "" {
		return "", false
	}
	return conversationID, true
}

func extractConversationTurnsPathConversationID(path string) (string, bool) {
	parts := assistantSplitPathSegments(path)
	if len(parts) != 5 {
		return "", false
	}
	if parts[0] != "internal" || parts[1] != "assistant" || parts[2] != "conversations" || parts[4] != "turns" {
		return "", false
	}
	conversationID := strings.TrimSpace(parts[3])
	if conversationID == "" {
		return "", false
	}
	return conversationID, true
}

func extractAssistantTurnActionPath(path string) (conversationID string, turnID string, action string, ok bool) {
	parts := assistantSplitPathSegments(path)
	if len(parts) != 6 {
		return "", "", "", false
	}
	if parts[0] != "internal" || parts[1] != "assistant" || parts[2] != "conversations" || parts[4] != "turns" {
		return "", "", "", false
	}
	conversationID = strings.TrimSpace(parts[3])
	turnAction := strings.TrimSpace(parts[5])
	if conversationID == "" || turnAction == "" {
		return "", "", "", false
	}
	index := strings.LastIndex(turnAction, ":")
	if index <= 0 || index >= len(turnAction)-1 {
		return "", "", "", false
	}
	turnID = strings.TrimSpace(turnAction[:index])
	action = strings.TrimSpace(turnAction[index+1:])
	if turnID == "" || action == "" {
		return "", "", "", false
	}
	return conversationID, turnID, action, true
}

func cloneConversation(in *assistantConversation) *assistantConversation {
	if in == nil {
		return nil
	}
	out := *in
	out.Turns = make([]*assistantTurn, 0, len(in.Turns))
	for _, turn := range in.Turns {
		if turn == nil {
			continue
		}
		copyTurn := *turn
		copyTurn.Candidates = append([]assistantCandidate(nil), turn.Candidates...)
		copyTurn.DryRun.Diff = append([]map[string]any(nil), turn.DryRun.Diff...)
		if turn.CommitResult != nil {
			copyResult := *turn.CommitResult
			copyTurn.CommitResult = &copyResult
		}
		out.Turns = append(out.Turns, &copyTurn)
	}
	return &out
}

func hasRequestBody(r *http.Request) bool {
	if r == nil || r.Body == nil {
		return false
	}
	if r.ContentLength > 0 {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("Transfer-Encoding")), "chunked") {
		return true
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func ptrString(v string) *string {
	value := v
	return &value
}

func assistantSplitPathSegments(path string) []string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}
