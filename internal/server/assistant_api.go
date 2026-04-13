package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
)

const (
	assistantStateDraft     = "draft"
	assistantStateProposed  = "proposed"
	assistantStateValidated = "validated"
	assistantStateConfirmed = "confirmed"
	assistantStateCommitted = "committed"
	assistantStateCanceled  = "canceled"
	assistantStateExpired   = "expired"

	assistantResolutionAuto          = "auto"
	assistantResolutionUserConfirmed = "user_confirmed"

	assistantIntentCreateOrgUnit        = "create_orgunit"
	assistantIntentAddOrgUnitVersion    = "add_orgunit_version"
	assistantIntentInsertOrgUnitVersion = "insert_orgunit_version"
	assistantIntentCorrectOrgUnit       = "correct_orgunit"
	assistantIntentDisableOrgUnit       = "disable_orgunit"
	assistantIntentEnableOrgUnit        = "enable_orgunit"
	assistantIntentMoveOrgUnit          = "move_orgunit"
	assistantIntentRenameOrgUnit        = "rename_orgunit"
	assistantIntentPlanOnly             = "plan_only"
)

var (
	assistantDateCNRE   = regexp.MustCompile(`(20\d{2})年(\d{1,2})月(\d{1,2})日`)
	assistantDateISORE  = regexp.MustCompile(`(20\d{2}-\d{2}-\d{2})`)
	assistantBoundaryRE = regexp.MustCompile(`(?i)(\bselect\b|\binsert\s+into\b|\bupdate\s+\S+\s+set\b|\bdelete\s+from\b|\bdrop\s+table\b|\btruncate\s+table\b|\balter\s+table\b|--|/\*|\*/|;)`)
)

type assistantConversationService struct {
	orgStore              OrgUnitStore
	writeSvc              orgunitservices.OrgUnitWriteService
	actionRegistry        assistantActionRegistry
	commitAdapterRegistry assistantCommitAdapterRegistry
	modelGateway          *assistantModelGateway
	gatewayErr            error
	knowledgeRuntime      *assistantKnowledgeRuntime
	knowledgeErr          error
	pool                  assistantTxBeginner
	mu                    sync.RWMutex
	byID                  map[string]*assistantConversation
	byActorID             map[string][]string
}

type assistantTxBeginner interface {
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

type assistantConversation struct {
	ConversationID string                     `json:"conversation_id"`
	TenantID       string                     `json:"tenant_id"`
	ActorID        string                     `json:"actor_id"`
	ActorRole      string                     `json:"actor_role"`
	State          string                     `json:"state"`
	CurrentPhase   string                     `json:"current_phase,omitempty"`
	Transitions    []assistantStateTransition `json:"state_transitions,omitempty"`
	CreatedAt      time.Time                  `json:"created_at"`
	UpdatedAt      time.Time                  `json:"updated_at"`
	Turns          []*assistantTurn           `json:"turns"`
}

type assistantStateTransition struct {
	ID         int64     `json:"id,omitempty"`
	TurnID     string    `json:"turn_id,omitempty"`
	TurnAction string    `json:"turn_action,omitempty"`
	RequestID  string    `json:"request_id"`
	TraceID    string    `json:"trace_id"`
	FromState  string    `json:"from_state"`
	ToState    string    `json:"to_state"`
	FromPhase  string    `json:"from_phase,omitempty"`
	ToPhase    string    `json:"to_phase,omitempty"`
	ReasonCode string    `json:"reason_code,omitempty"`
	ActorID    string    `json:"actor_id"`
	ChangedAt  time.Time `json:"changed_at"`
}

type assistantTurn struct {
	TurnID              string                          `json:"turn_id"`
	UserInput           string                          `json:"user_input"`
	State               string                          `json:"state"`
	Phase               string                          `json:"phase,omitempty"`
	RiskTier            string                          `json:"risk_tier"`
	RequestID           string                          `json:"request_id"`
	TraceID             string                          `json:"trace_id"`
	PolicyVersion       string                          `json:"policy_version"`
	CompositionVersion  string                          `json:"composition_version"`
	MappingVersion      string                          `json:"mapping_version"`
	Intent              assistantIntentSpec             `json:"intent"`
	RouteDecision       assistantIntentRouteDecision    `json:"route_decision,omitempty"`
	Clarification       *assistantClarificationDecision `json:"clarification,omitempty"`
	Plan                assistantPlanSummary            `json:"plan"`
	PendingDraftSummary string                          `json:"pending_draft_summary,omitempty"`
	MissingFields       []string                        `json:"missing_fields,omitempty"`
	Candidates          []assistantCandidate            `json:"candidates"`
	ResolvedCandidateID string                          `json:"resolved_candidate_id,omitempty"`
	SelectedCandidateID string                          `json:"selected_candidate_id,omitempty"`
	AmbiguityCount      int                             `json:"ambiguity_count"`
	Confidence          float64                         `json:"confidence"`
	ResolutionSource    string                          `json:"resolution_source,omitempty"`
	DryRun              assistantDryRunResult           `json:"dry_run"`
	CommitResult        *assistantCommitResult          `json:"commit_result,omitempty"`
	CommitReply         *assistantCommitReply           `json:"commit_reply,omitempty"`
	ErrorCode           string                          `json:"error_code,omitempty"`
	ReplyNLG            *assistantRenderReplyResponse   `json:"reply_nlg,omitempty"`
	CreatedAt           time.Time                       `json:"created_at"`
	UpdatedAt           time.Time                       `json:"updated_at"`
}

type assistantIntentSpec struct {
	Action              string `json:"action"`
	IntentID            string `json:"intent_id,omitempty"`
	RouteKind           string `json:"route_kind,omitempty"`
	RouteCatalogVersion string `json:"route_catalog_version,omitempty"`
	ParentRefText       string `json:"parent_ref_text,omitempty"`
	EntityName          string `json:"entity_name,omitempty"`
	EffectiveDate       string `json:"effective_date,omitempty"`
	OrgCode             string `json:"org_code,omitempty"`
	TargetEffectiveDate string `json:"target_effective_date,omitempty"`
	NewName             string `json:"new_name,omitempty"`
	NewParentRefText    string `json:"new_parent_ref_text,omitempty"`
	IntentSchemaVersion string `json:"intent_schema_version,omitempty"`
	ContextHash         string `json:"context_hash,omitempty"`
	IntentHash          string `json:"intent_hash,omitempty"`
}

type assistantPlanSummary struct {
	Title                   string                      `json:"title"`
	ActionID                string                      `json:"action_id,omitempty"`
	ActionVersion           string                      `json:"action_version,omitempty"`
	CapabilityKey           string                      `json:"capability_key"`
	CommitAdapterKey        string                      `json:"commit_adapter_key,omitempty"`
	Summary                 string                      `json:"summary"`
	CapabilityMapVersion    string                      `json:"capability_map_version,omitempty"`
	CompilerContractVersion string                      `json:"compiler_contract_version,omitempty"`
	SkillManifestDigest     string                      `json:"skill_manifest_digest,omitempty"`
	ModelProvider           string                      `json:"model_provider,omitempty"`
	ModelName               string                      `json:"model_name,omitempty"`
	ModelRevision           string                      `json:"model_revision,omitempty"`
	KnowledgeSnapshotDigest string                      `json:"knowledge_snapshot_digest,omitempty"`
	RouteCatalogVersion     string                      `json:"route_catalog_version,omitempty"`
	ResolverContractVersion string                      `json:"resolver_contract_version,omitempty"`
	ContextTemplateVersion  string                      `json:"context_template_version,omitempty"`
	ReplyGuidanceVersion    string                      `json:"reply_guidance_version,omitempty"`
	VersionTuple            json.RawMessage             `json:"version_tuple,omitempty"`
	ConfirmTTLSeconds       int                         `json:"confirm_ttl_seconds,omitempty"`
	ExpiresAt               string                      `json:"expires_at,omitempty"`
	SkillExecutionPlan      assistantSkillExecutionPlan `json:"skill_execution_plan,omitempty"`
	ConfigDeltaPlan         assistantConfigDeltaPlan    `json:"config_delta_plan,omitempty"`
}

type assistantSkillExecutionPlan struct {
	SelectedSkills []string `json:"selected_skills"`
	ExecutionOrder []string `json:"execution_order"`
	RiskTier       string   `json:"risk_tier"`
	RequiredChecks []string `json:"required_checks"`
}

type assistantConfigDeltaPlan struct {
	CapabilityKey string                  `json:"capability_key"`
	Changes       []assistantConfigChange `json:"changes"`
}

type assistantConfigChange struct {
	Field string `json:"field"`
	After any    `json:"after"`
}

type assistantDryRunResult struct {
	Diff                     []map[string]any                           `json:"diff"`
	Explain                  string                                     `json:"explain"`
	ValidationErrors         []string                                   `json:"validation_errors,omitempty"`
	Retrieval                assistantSemanticRetrievalResult           `json:"retrieval,omitempty"`
	WouldCommit              bool                                       `json:"would_commit"`
	PlanHash                 string                                     `json:"plan_hash,omitempty"`
	CreateOrgUnitProjection  *assistantCreateOrgUnitProjectionSnapshot  `json:"create_orgunit_projection,omitempty"`
	OrgUnitVersionProjection *assistantOrgUnitVersionProjectionSnapshot `json:"orgunit_version_projection,omitempty"`
}

type assistantCandidate struct {
	OrgID         int     `json:"-"`
	OrgNodeKey    string  `json:"-"`
	CandidateID   string  `json:"candidate_id"`
	CandidateCode string  `json:"candidate_code"`
	Name          string  `json:"name"`
	Path          string  `json:"path"`
	AsOf          string  `json:"as_of"`
	IsActive      bool    `json:"is_active"`
	MatchScore    float64 `json:"match_score"`
}

func assistantCandidateNormalizedOrgNodeKey(candidate assistantCandidate) (string, bool) {
	if orgNodeKey := strings.TrimSpace(candidate.OrgNodeKey); orgNodeKey != "" {
		normalized, err := normalizeOrgNodeKeyInput(orgNodeKey)
		if err == nil {
			return normalized, true
		}
	}
	if candidate.OrgID > 0 {
		orgNodeKey, err := encodeOrgNodeKeyFromID(candidate.OrgID)
		if err == nil {
			return orgNodeKey, true
		}
	}
	return "", false
}

func assistantSearchCandidateOrgNodeKey(item OrgUnitSearchCandidate) (string, bool) {
	if orgNodeKey := strings.TrimSpace(item.OrgNodeKey); orgNodeKey != "" {
		normalized, err := normalizeOrgNodeKeyInput(orgNodeKey)
		if err == nil {
			return normalized, true
		}
	}
	if item.OrgID > 0 {
		orgNodeKey, err := encodeOrgNodeKeyFromID(item.OrgID)
		if err == nil {
			return orgNodeKey, true
		}
	}
	return "", false
}

func assistantOpaqueCandidateID(orgNodeKey string, name string) string {
	normalizedOrgNodeKey, _ := normalizeOrgNodeKeyInput(strings.TrimSpace(orgNodeKey))
	payload := strings.Join([]string{
		"assistant_candidate",
		normalizedOrgNodeKey,
		strings.TrimSpace(name),
	}, "|")
	if payload == "assistant_candidate||" {
		return ""
	}
	sum := sha256.Sum256([]byte(payload))
	return "cand_" + hex.EncodeToString(sum[:6])
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

type assistantConversationListResponse struct {
	Items      []assistantConversationListItem `json:"items"`
	NextCursor string                          `json:"next_cursor"`
}

type assistantConversationListItem struct {
	ConversationID string                             `json:"conversation_id"`
	State          string                             `json:"state"`
	UpdatedAt      time.Time                          `json:"updated_at"`
	LastTurn       *assistantConversationListLastTurn `json:"last_turn,omitempty"`
}

type assistantConversationListLastTurn struct {
	TurnID    string `json:"turn_id"`
	UserInput string `json:"user_input"`
	State     string `json:"state"`
	RiskTier  string `json:"risk_tier"`
}

type assistantCreateTurnRequest struct {
	UserInput string `json:"user_input"`
}

type assistantConfirmRequest struct {
	CandidateID string `json:"candidate_id"`
}

type assistantRenderReplyRequest struct {
	Stage            string `json:"stage"`
	Kind             string `json:"kind"`
	Outcome          string `json:"outcome"`
	ErrorCode        string `json:"error_code"`
	ErrorMessage     string `json:"error_message"`
	NextAction       string `json:"next_action"`
	Locale           string `json:"locale"`
	FallbackText     string `json:"fallback_text"`
	AllowMissingTurn bool   `json:"allow_missing_turn"`
}

func newAssistantConversationService(orgStore OrgUnitStore, writeSvc orgunitservices.OrgUnitWriteService) *assistantConversationService {
	gateway, err := newAssistantModelGateway()
	knowledgeRuntime, knowledgeErr := assistantLoadKnowledgeRuntime()
	return &assistantConversationService{
		orgStore:         orgStore,
		writeSvc:         writeSvc,
		modelGateway:     gateway,
		gatewayErr:       err,
		knowledgeRuntime: knowledgeRuntime,
		knowledgeErr:     knowledgeErr,
		byID:             make(map[string]*assistantConversation),
		byActorID:        make(map[string][]string),
	}
}

func newAssistantConversationServiceWithPool(orgStore OrgUnitStore, writeSvc orgunitservices.OrgUnitWriteService, pool *pgxpool.Pool) *assistantConversationService {
	svc := newAssistantConversationService(orgStore, writeSvc)
	if pool != nil {
		svc.pool = pool
	}
	return svc
}

func handleAssistantConversationsAPI(w http.ResponseWriter, r *http.Request, svc *assistantConversationService) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
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
	if r.Method == http.MethodGet {
		pageSize := 20
		if rawPageSize := strings.TrimSpace(r.URL.Query().Get("page_size")); rawPageSize != "" {
			parsed, err := strconv.Atoi(rawPageSize)
			if err != nil {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "invalid page_size")
				return
			}
			pageSize = parsed
		}
		if pageSize <= 0 {
			pageSize = 20
		}
		if pageSize > 100 {
			pageSize = 100
		}
		cursor := strings.TrimSpace(r.URL.Query().Get("cursor"))
		items, nextCursor, err := svc.listConversations(r.Context(), tenant.ID, principal.ID, pageSize, cursor)
		if err != nil {
			switch {
			case errors.Is(err, errAssistantConversationCursorInvalid):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "assistant_conversation_cursor_invalid", "assistant conversation cursor invalid")
			default:
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "assistant_conversation_list_failed", "assistant conversation list failed")
			}
			return
		}
		writeJSON(w, http.StatusOK, assistantConversationListResponse{
			Items:      items,
			NextCursor: nextCursor,
		})
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

	conversation, err := svc.createConversationWithContext(r.Context(), tenant.ID, principal)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "assistant_conversation_create_failed", "assistant conversation create failed")
		return
	}
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
		if errors.Is(err, errAssistantTenantMismatch) {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "tenant_mismatch", "tenant mismatch")
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
		case errors.Is(err, errAssistantTenantMismatch):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "tenant_mismatch", "tenant mismatch")
		case errors.Is(err, errAssistantConversationForbidden):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "forbidden", "forbidden")
		case errors.Is(err, errAssistantPlanSchemaConstrainedDecodeFailed):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "ai_plan_schema_constrained_decode_failed", "ai plan schema constrained decode failed")
		case errors.Is(err, errAssistantPlanBoundaryViolation):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "ai_plan_boundary_violation", "ai plan boundary violation")
		case assistantIsRuntimeUnavailableError(err):
			assistantWriteRuntimeUnavailable(w, r)
		case errors.Is(err, errAssistantRouteRuntimeInvalid):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, errAssistantRouteRuntimeInvalid.Error(), "assistant route runtime invalid")
		case errors.Is(err, errAssistantRouteCatalogMissing):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusServiceUnavailable, errAssistantRouteCatalogMissing.Error(), "assistant route catalog missing")
		case errors.Is(err, errAssistantRouteActionConflict):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, errAssistantRouteActionConflict.Error(), "assistant route action conflict")
		case errors.Is(err, errAssistantRouteDecisionMissing):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, errAssistantRouteDecisionMissing.Error(), "assistant route decision missing")
		case errors.Is(err, errAssistantPlanDeterminismViolation):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "ai_plan_determinism_violation", "ai plan determinism violation")
		case errors.Is(err, errAssistantUnsupportedIntent):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "assistant_intent_unsupported", "assistant intent unsupported")
		case errors.Is(err, errAssistantActionSpecMissing):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, errAssistantActionSpecMissing.Error(), "assistant action spec missing")
		case errors.Is(err, errAssistantActionAuthzDenied):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, errAssistantActionAuthzDenied.Error(), "assistant action authz denied")
		case errors.Is(err, errAssistantActionRiskGateDenied):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, errAssistantActionRiskGateDenied.Error(), "assistant action risk gate denied")
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
			case errors.Is(err, errAssistantTenantMismatch):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "tenant_mismatch", "tenant mismatch")
			case errors.Is(err, errAssistantConversationForbidden):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "forbidden", "forbidden")
			case errors.Is(err, errAssistantTurnNotFound):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "conversation_turn_not_found", "conversation turn not found")
			case errors.Is(err, errAssistantIdempotencyKeyConflict):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "idempotency_key_conflict", "idempotency key conflict")
			case errors.Is(err, errAssistantRequestInProgress):
				w.Header().Set("Retry-After", assistantDefaultRetryAfterSecs)
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "request_in_progress", "request in progress")
			case errors.Is(err, errAssistantConfirmationRequired):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "conversation_confirmation_required", "conversation confirmation required")
			case errors.Is(err, errAssistantConfirmationExpired):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "conversation_confirmation_expired", "conversation confirmation expired")
			case errors.Is(err, errAssistantConversationStateInvalid):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "conversation_state_invalid", "conversation state invalid")
			case errors.Is(err, errAssistantCandidateNotFound):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "assistant_candidate_not_found", "assistant candidate not found")
			case errors.Is(err, errAssistantRouteNonBusinessBlocked):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, errAssistantRouteNonBusinessBlocked.Error(), "assistant route non business blocked")
			case errors.Is(err, errAssistantRouteDecisionMissing):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, errAssistantRouteDecisionMissing.Error(), "assistant route decision missing")
			case errors.Is(err, errAssistantRouteRuntimeInvalid):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, errAssistantRouteRuntimeInvalid.Error(), "assistant route runtime invalid")
			case errors.Is(err, errAssistantRouteActionConflict):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, errAssistantRouteActionConflict.Error(), "assistant route action conflict")
			case errors.Is(err, errAssistantUnsupportedIntent):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "assistant_intent_unsupported", "assistant intent unsupported")
			case errors.Is(err, errAssistantActionAuthzDenied):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, errAssistantActionAuthzDenied.Error(), "assistant action authz denied")
			case errors.Is(err, errAssistantActionRiskGateDenied):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, errAssistantActionRiskGateDenied.Error(), "assistant action risk gate denied")
			case errors.Is(err, errAssistantPlanContractVersionMismatch):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "ai_plan_contract_version_mismatch", "ai plan contract version mismatch")
			case assistantIsGateUnavailableError(err):
				assistantWriteGateUnavailable(w, r)
			default:
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "assistant_turn_confirm_failed", "assistant turn confirm failed")
			}
			return
		}
		writeJSON(w, http.StatusOK, conversation)
		return
	case "commit":
		receipt, err := svc.submitCommitTask(r.Context(), tenant.ID, principal, conversationID, turnID)
		if err != nil {
			switch {
			case errors.Is(err, errAssistantConversationNotFound):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "conversation_not_found", "conversation not found")
			case errors.Is(err, errAssistantTurnNotFound):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "conversation_turn_not_found", "conversation turn not found")
			case errors.Is(err, errAssistantIdempotencyKeyConflict):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "idempotency_key_conflict", "idempotency key conflict")
			case errors.Is(err, errAssistantRequestInProgress):
				w.Header().Set("Retry-After", assistantDefaultRetryAfterSecs)
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "request_in_progress", "request in progress")
			case assistantIsGateUnavailableError(err):
				assistantWriteGateUnavailable(w, r)
			case errors.Is(err, errAssistantTaskStateInvalid):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "assistant_task_state_invalid", "assistant task state invalid")
			case errors.Is(err, errAssistantConfirmationRequired):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "conversation_confirmation_required", "conversation confirmation required")
			case errors.Is(err, errAssistantConfirmationExpired):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "conversation_confirmation_expired", "conversation confirmation expired")
			case errors.Is(err, errAssistantRouteNonBusinessBlocked):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, errAssistantRouteNonBusinessBlocked.Error(), "assistant route non business blocked")
			case errors.Is(err, errAssistantRouteDecisionMissing):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, errAssistantRouteDecisionMissing.Error(), "assistant route decision missing")
			case errors.Is(err, errAssistantRouteRuntimeInvalid):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, errAssistantRouteRuntimeInvalid.Error(), "assistant route runtime invalid")
			case errors.Is(err, errAssistantRouteActionConflict):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, errAssistantRouteActionConflict.Error(), "assistant route action conflict")
			case errors.Is(err, errAssistantConversationStateInvalid):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "conversation_state_invalid", "conversation state invalid")
			case errors.Is(err, errAssistantPlanContractVersionMismatch):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "ai_plan_contract_version_mismatch", "ai plan contract version mismatch")
			case errors.Is(err, errAssistantVersionTupleStale):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "ai_version_tuple_stale", "ai version tuple stale")
			case errors.Is(err, errAssistantAuthSnapshotExpired):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "ai_actor_auth_snapshot_expired", "ai actor auth snapshot expired")
			case errors.Is(err, errAssistantRoleDriftDetected):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "ai_actor_role_drift_detected", "ai actor role drift detected")
			case errors.Is(err, errAssistantUnsupportedIntent):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "assistant_intent_unsupported", "assistant intent unsupported")
			case errors.Is(err, errAssistantCandidateNotFound):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "conversation_confirmation_required", "conversation confirmation required")
			case errors.Is(err, errAssistantActionAuthzDenied):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, errAssistantActionAuthzDenied.Error(), "assistant action authz denied")
			case errors.Is(err, errAssistantActionRiskGateDenied):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, errAssistantActionRiskGateDenied.Error(), "assistant action risk gate denied")
			case errors.Is(err, errAssistantActionRequiredCheckFailed):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, errAssistantActionRequiredCheckFailed.Error(), "assistant action required check failed")
			default:
				if status, code, message, ok := assistantResolveCommitError(err); ok {
					routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, message)
					return
				}
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "assistant_commit_failed", "assistant commit failed")
			}
			return
		}
		writeJSON(w, http.StatusAccepted, receipt)
		return
	case "reply":
		var req assistantRenderReplyRequest
		if hasRequestBody(r) {
			dec := json.NewDecoder(r.Body)
			dec.DisallowUnknownFields()
			if err := dec.Decode(&req); err != nil {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
				return
			}
		}
		reply, err := svc.renderTurnReply(r.Context(), tenant.ID, principal, conversationID, turnID, req)
		if err != nil {
			switch {
			case errors.Is(err, errAssistantConversationNotFound):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "conversation_not_found", "conversation not found")
			case errors.Is(err, errAssistantTenantMismatch):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "tenant_mismatch", "tenant mismatch")
			case errors.Is(err, errAssistantConversationForbidden):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "forbidden", "forbidden")
			case errors.Is(err, errAssistantTurnNotFound):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "conversation_turn_not_found", "conversation turn not found")
			default:
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "assistant_reply_render_failed", "assistant reply render failed")
			}
			return
		}
		writeJSON(w, http.StatusOK, reply)
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

func assistantWriteRuntimeUnavailable(w http.ResponseWriter, r *http.Request) {
	routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusServiceUnavailable, errAssistantRuntimeUnavailable.Error(), "assistant runtime unavailable")
}

func assistantWriteGateUnavailable(w http.ResponseWriter, r *http.Request) {
	routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusServiceUnavailable, errAssistantGateUnavailable.Error(), "assistant gate unavailable")
}

func assistantIsRuntimeUnavailableError(err error) bool {
	switch {
	case errors.Is(err, errAssistantRuntimeUnavailable):
		return true
	case errors.Is(err, errAssistantModelProviderUnavailable):
		return true
	case errors.Is(err, errAssistantModelTimeout):
		return true
	case errors.Is(err, errAssistantModelRateLimited):
		return true
	case errors.Is(err, errAssistantModelConfigInvalid):
		return true
	case errors.Is(err, errAssistantRuntimeConfigInvalid):
		return true
	case errors.Is(err, errAssistantRuntimeConfigMissing):
		return true
	case errors.Is(err, errAssistantModelSecretMissing):
		return true
	default:
		return false
	}
}

func assistantIsGateUnavailableError(err error) bool {
	switch {
	case errors.Is(err, errAssistantGateUnavailable):
		return true
	case errors.Is(err, errAssistantTaskWorkflowUnavailable):
		return true
	case errors.Is(err, errAssistantServiceMissing):
		return true
	default:
		return false
	}
}

func assistantPublicFailureCode(err error) (string, bool) {
	switch {
	case assistantIsRuntimeUnavailableError(err):
		return errAssistantRuntimeUnavailable.Error(), true
	case assistantIsGateUnavailableError(err):
		return errAssistantGateUnavailable.Error(), true
	default:
		return "", false
	}
}

var (
	errAssistantConversationNotFound              = errors.New("assistant_conversation_not_found")
	errAssistantConversationForbidden             = errors.New("assistant_conversation_forbidden")
	errAssistantTenantMismatch                    = errors.New("assistant_tenant_mismatch")
	errAssistantConversationCorrupted             = errors.New("assistant_conversation_corrupted")
	errAssistantTurnNotFound                      = errors.New("assistant_turn_not_found")
	errAssistantConfirmationRequired              = errors.New("assistant_confirmation_required")
	errAssistantConfirmationExpired               = errors.New("assistant_confirmation_expired")
	errAssistantCandidateNotFound                 = errors.New("assistant_candidate_not_found")
	errAssistantAuthSnapshotExpired               = errors.New("assistant_auth_snapshot_expired")
	errAssistantRoleDriftDetected                 = errors.New("assistant_role_drift_detected")
	errAssistantUnsupportedIntent                 = errors.New("assistant_unsupported_intent")
	errAssistantServiceMissing                    = errors.New("assistant_service_missing")
	errAssistantConversationStateInvalid          = errors.New("assistant_conversation_state_invalid")
	errAssistantPlanSchemaConstrainedDecodeFailed = errors.New("assistant_plan_schema_constrained_decode_failed")
	errAssistantPlanBoundaryViolation             = errors.New("assistant_plan_boundary_violation")
	errAssistantPlanContractVersionMismatch       = errors.New("assistant_plan_contract_version_mismatch")
	errAssistantVersionTupleStale                 = errors.New("assistant_version_tuple_stale")
	errAssistantPlanDeterminismViolation          = errors.New("assistant_plan_determinism_violation")
	errAssistantRuntimeUnavailable                = errors.New("assistant_runtime_unavailable")
	errAssistantGateUnavailable                   = errors.New("assistant_gate_unavailable")
	errAssistantModelProviderUnavailable          = errors.New("assistant_model_provider_unavailable")
	errAssistantModelTimeout                      = errors.New("assistant_model_timeout")
	errAssistantModelRateLimited                  = errors.New("assistant_model_rate_limited")
	errAssistantModelConfigInvalid                = errors.New("assistant_model_config_invalid")
	errAssistantRuntimeConfigInvalid              = errors.New("assistant_runtime_config_invalid")
	errAssistantRuntimeConfigMissing              = errors.New("assistant_runtime_config_missing")
	errAssistantModelSecretMissing                = errors.New("assistant_model_secret_missing")
	errAssistantConversationCursorInvalid         = errors.New("assistant_conversation_cursor_invalid")
	errAssistantIdempotencyKeyConflict            = errors.New("assistant_idempotency_key_conflict")
	errAssistantRequestInProgress                 = errors.New("assistant_request_in_progress")
	errAssistantActionSpecMissing                 = errors.New("ai_action_spec_missing")
	errAssistantActionCapabilityUnregistered      = errors.New("ai_capability_unregistered")
	errAssistantActionAuthzDenied                 = errors.New("ai_action_authz_denied")
	errAssistantActionRiskGateDenied              = errors.New("ai_action_risk_gate_denied")
	errAssistantActionRequiredCheckFailed         = errors.New("ai_action_required_check_failed")
	errAssistantRouteRuntimeInvalid               = errors.New("ai_route_runtime_invalid")
	errAssistantRouteCatalogMissing               = errors.New("ai_route_catalog_missing")
	errAssistantRouteActionConflict               = errors.New("ai_route_action_conflict")
	errAssistantRouteDecisionMissing              = errors.New("ai_route_decision_missing")
	errAssistantRouteNonBusinessBlocked           = errors.New("ai_route_non_business_blocked")
	errAssistantRouteClarificationRequired        = errors.New("ai_route_clarification_required")
	errAssistantClarificationRequired             = errors.New("assistant_clarification_required")
	errAssistantClarificationRoundsExhausted      = errors.New("assistant_clarification_rounds_exhausted")
	errAssistantManualHintRequired                = errors.New("assistant_manual_hint_required")
	errAssistantClarificationRuntimeInvalid       = errors.New("assistant_clarification_runtime_invalid")
	errAssistantTaskNotFound                      = errors.New("assistant_task_not_found")
	errAssistantTaskStateInvalid                  = errors.New("assistant_task_state_invalid")
	errAssistantTaskCancelNotAllowed              = errors.New("assistant_task_cancel_not_allowed")
	errAssistantTaskWorkflowUnavailable           = errors.New("assistant_task_workflow_unavailable")
	errAssistantTaskDispatchFailed                = errors.New("assistant_task_dispatch_failed")
	errAssistantReplyRenderFailed                 = errors.New("assistant_reply_render_failed")
	errAssistantReplyModelTargetMismatch          = errors.New("assistant_reply_model_target_mismatch")
)

func (s *assistantConversationService) createConversationWithContext(ctx context.Context, tenantID string, principal Principal) (*assistantConversation, error) {
	if s.pool != nil {
		return s.createConversationPG(ctx, tenantID, principal)
	}
	return s.createConversation(tenantID, principal), nil
}

func (s *assistantConversationService) listConversations(ctx context.Context, tenantID string, actorID string, pageSize int, cursor string) ([]assistantConversationListItem, string, error) {
	if s.pool != nil {
		return s.listConversationsPG(ctx, tenantID, actorID, pageSize, cursor)
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	s.mu.RLock()
	conversations := make([]*assistantConversation, 0, len(s.byID))
	for _, conversation := range s.byID {
		if conversation == nil {
			continue
		}
		if conversation.TenantID != tenantID || conversation.ActorID != actorID {
			continue
		}
		conversations = append(conversations, cloneConversation(conversation))
	}
	s.mu.RUnlock()
	sort.SliceStable(conversations, func(i, j int) bool {
		left := conversations[i]
		right := conversations[j]
		if left.UpdatedAt.Equal(right.UpdatedAt) {
			return left.ConversationID > right.ConversationID
		}
		return left.UpdatedAt.After(right.UpdatedAt)
	})
	decoded, err := assistantDecodeConversationCursor(cursor, tenantID, actorID)
	if err != nil {
		return nil, "", err
	}
	filtered := make([]*assistantConversation, 0, len(conversations))
	for _, conversation := range conversations {
		if decoded == nil {
			filtered = append(filtered, conversation)
			continue
		}
		if conversation.UpdatedAt.Before(decoded.UpdatedAt) {
			filtered = append(filtered, conversation)
			continue
		}
		if conversation.UpdatedAt.Equal(decoded.UpdatedAt) && conversation.ConversationID < decoded.ConversationID {
			filtered = append(filtered, conversation)
		}
	}
	limit := pageSize + 1
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	nextCursor := ""
	if len(filtered) > pageSize {
		last := filtered[pageSize-1]
		nextCursor = assistantEncodeConversationCursor(assistantConversationCursor{
			UpdatedAt:      last.UpdatedAt,
			ConversationID: last.ConversationID,
		}, tenantID, actorID)
		filtered = filtered[:pageSize]
	}
	items := make([]assistantConversationListItem, 0, len(filtered))
	for _, conversation := range filtered {
		item := assistantConversationListItem{
			ConversationID: conversation.ConversationID,
			State:          conversation.State,
			UpdatedAt:      conversation.UpdatedAt,
		}
		if last := latestTurn(conversation); last != nil {
			item.LastTurn = &assistantConversationListLastTurn{
				TurnID:    last.TurnID,
				UserInput: last.UserInput,
				State:     last.State,
				RiskTier:  last.RiskTier,
			}
		}
		items = append(items, item)
	}
	return items, nextCursor, nil
}

func (s *assistantConversationService) createConversation(tenantID string, principal Principal) *assistantConversation {
	now := time.Now().UTC()
	conversationID := "conv_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	traceID := strings.ReplaceAll(uuid.NewString(), "-", "")
	requestID := "assistant_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	conversation := &assistantConversation{
		ConversationID: conversationID,
		TenantID:       tenantID,
		ActorID:        principal.ID,
		ActorRole:      strings.TrimSpace(principal.RoleSlug),
		State:          assistantStateValidated,
		CurrentPhase:   assistantPhaseIdle,
		CreatedAt:      now,
		UpdatedAt:      now,
		Transitions: []assistantStateTransition{
			{
				RequestID:  requestID,
				TraceID:    traceID,
				FromState:  "init",
				ToState:    assistantStateValidated,
				FromPhase:  "init",
				ToPhase:    assistantPhaseIdle,
				ReasonCode: "conversation_created",
				ActorID:    principal.ID,
				ChangedAt:  now,
			},
		},
		Turns: make([]*assistantTurn, 0, 4),
	}

	s.mu.Lock()
	s.byID[conversation.ConversationID] = conversation
	s.byActorID[principal.ID] = append(s.byActorID[principal.ID], conversation.ConversationID)
	s.mu.Unlock()

	return cloneConversation(conversation)
}

func (s *assistantConversationService) getConversation(tenantID string, actorID string, conversationID string) (*assistantConversation, error) {
	if s.pool != nil {
		return s.getConversationPG(context.Background(), tenantID, actorID, conversationID)
	}
	s.mu.RLock()
	conversation, ok := s.byID[conversationID]
	s.mu.RUnlock()
	if !ok {
		return nil, errAssistantConversationNotFound
	}
	if conversation == nil {
		return nil, errAssistantConversationCorrupted
	}
	if conversation.TenantID != tenantID {
		return nil, errAssistantTenantMismatch
	}
	if conversation.ActorID != actorID {
		return nil, errAssistantConversationForbidden
	}
	return cloneConversation(conversation), nil
}

func (s *assistantConversationService) createTurn(ctx context.Context, tenantID string, principal Principal, conversationID string, userInput string) (*assistantConversation, error) {
	ctx = withPrincipal(ctx, principal)
	if s.pool != nil {
		return s.createTurnPG(ctx, tenantID, principal, conversationID, userInput)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	conversation, ok := s.byID[conversationID]
	if !ok {
		return nil, errAssistantConversationNotFound
	}
	if conversation.TenantID != tenantID {
		return nil, errAssistantTenantMismatch
	}
	if conversation.ActorID != principal.ID {
		return nil, errAssistantConversationForbidden
	}
	pendingTurn := assistantLatestPendingTurn(conversation)
	turn, err := s.prepareTurnDraft(ctx, tenantID, principal, conversationID, userInput, pendingTurn)
	if err != nil {
		return nil, err
	}
	turnCreatedAt := turn.CreatedAt
	conversation.Turns = append(conversation.Turns, turn)
	conversation.UpdatedAt = turnCreatedAt
	conversation.State = turn.State
	conversation.CurrentPhase = turn.Phase
	conversation.Transitions = append(conversation.Transitions, assistantStateTransition{
		TurnID:     turn.TurnID,
		RequestID:  turn.RequestID,
		TraceID:    turn.TraceID,
		FromState:  "init",
		ToState:    assistantStateValidated,
		ReasonCode: "turn_created",
		ActorID:    principal.ID,
		ChangedAt:  turnCreatedAt,
	})
	assistantRefreshConversationDerivedFields(conversation)

	return cloneConversation(conversation), nil
}

const assistantConfirmTTLSecondsDefault = 15 * 60

func assistantFreezeConfirmWindow(plan assistantPlanSummary, createdAt time.Time) assistantPlanSummary {
	ttlSeconds := plan.ConfirmTTLSeconds
	if ttlSeconds <= 0 {
		ttlSeconds = assistantConfirmTTLSecondsDefault
	}
	plan.ConfirmTTLSeconds = ttlSeconds
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	if strings.TrimSpace(plan.ExpiresAt) == "" {
		plan.ExpiresAt = createdAt.UTC().Add(time.Duration(ttlSeconds) * time.Second).Format(time.RFC3339)
	}
	return plan
}

func assistantTurnConfirmDeadline(turn *assistantTurn) (time.Time, bool) {
	if turn == nil {
		return time.Time{}, false
	}
	if expiresAt := strings.TrimSpace(turn.Plan.ExpiresAt); expiresAt != "" {
		parsed, err := time.Parse(time.RFC3339, expiresAt)
		if err == nil {
			return parsed.UTC(), true
		}
	}
	base := turn.CreatedAt.UTC()
	if base.IsZero() {
		base = turn.UpdatedAt.UTC()
	}
	if base.IsZero() {
		return time.Time{}, false
	}
	ttlSeconds := turn.Plan.ConfirmTTLSeconds
	if ttlSeconds <= 0 {
		ttlSeconds = assistantConfirmTTLSecondsDefault
	}
	return base.Add(time.Duration(ttlSeconds) * time.Second), true
}

func assistantTurnConfirmExpired(turn *assistantTurn, now time.Time) bool {
	if turn == nil || strings.TrimSpace(turn.State) != assistantStateValidated {
		return false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	deadline, ok := assistantTurnConfirmDeadline(turn)
	if !ok {
		return false
	}
	return !deadline.After(now.UTC())
}

func (s *assistantConversationService) confirmTurn(tenantID string, principal Principal, conversationID string, turnID string, candidateID string) (*assistantConversation, error) {
	if s.pool != nil {
		return s.confirmTurnPG(context.Background(), tenantID, principal, conversationID, turnID, candidateID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	conversation, turn, err := s.lookupMutableTurn(tenantID, principal.ID, conversationID, turnID)
	if err != nil {
		return nil, err
	}
	result, applyErr := s.applyConfirmTurn(conversation, turn, principal, candidateID)
	assistantRefreshConversationDerivedFields(conversation)
	if applyErr != nil {
		return nil, applyErr
	}
	if result.Transition == nil {
		return cloneConversation(conversation), nil
	}

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
	if conversation.TenantID != tenantID {
		return nil, nil, errAssistantTenantMismatch
	}
	if conversation.ActorID != actorID {
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
		orgNodeKey, _ := assistantSearchCandidateOrgNodeKey(item)
		if orgNodeKey != "" {
			if details, detailsErr := getNodeDetailsByVisibilityByNodeKey(ctx, s.orgStore, tenantID, orgNodeKey, asOf, false); detailsErr == nil {
				path = strings.TrimSpace(details.FullNamePath)
				if path == "" {
					path = item.Name
				}
			}
		}
		candidateID := strings.TrimSpace(item.OrgCode)
		if candidateID == "" {
			candidateID = assistantOpaqueCandidateID(orgNodeKey, item.Name)
		}
		candidates = append(candidates, assistantCandidate{
			OrgID:         item.OrgID,
			OrgNodeKey:    orgNodeKey,
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
	if spec, ok := assistantLookupDefaultActionSpec(intent.Action); ok {
		if riskTier := strings.TrimSpace(spec.Security.RiskTier); riskTier != "" {
			return riskTier
		}
	}
	return "low"
}

func assistantBuildPlan(intent assistantIntentSpec) assistantPlanSummary {
	plan := assistantPlanSummary{
		Title:                   "对话回复",
		CapabilityKey:           "org.assistant_conversation.manage",
		Summary:                 "当前轮仅保留对话语义与执行边界，不生成业务提交计划",
		CapabilityMapVersion:    assistantCapabilityMapVersionV1,
		CompilerContractVersion: assistantCompilerContractVersionV1,
	}
	if spec, ok := assistantLookupDefaultActionSpec(intent.Action); ok {
		plan.Title = spec.PlanTitle
		plan.ActionID = spec.ID
		plan.ActionVersion = spec.Version
		plan.CapabilityKey = spec.CapabilityKey
		plan.CommitAdapterKey = spec.Handler.CommitAdapterKey
		plan.Summary = spec.PlanSummary
	}
	switch strings.TrimSpace(intent.RouteKind) {
	case assistantRouteKindKnowledgeQA:
		plan.Summary = "当前轮属于知识问答，只返回说明，不触发业务提交。"
	case assistantRouteKindChitchat:
		plan.Summary = "当前轮属于闲聊响应，不触发业务提交。"
	case assistantRouteKindUncertain:
		plan.Summary = "当前轮语义仍不确定，仅保留澄清投影，不触发业务提交。"
	}
	return plan
}

func assistantBuildDryRun(intent assistantIntentSpec, candidates []assistantCandidate, resolvedCandidateID string) assistantDryRunResult {
	return assistantBuildDryRunWithRetrieval(intent, candidates, resolvedCandidateID, assistantSemanticRetrievalResult{})
}

func assistantBuildDryRunWithRetrieval(intent assistantIntentSpec, candidates []assistantCandidate, resolvedCandidateID string, retrieval assistantSemanticRetrievalResult) assistantDryRunResult {
	diff := make([]map[string]any, 0, 4)
	validationErrors := assistantIntentValidationErrors(intent)
	switch strings.TrimSpace(intent.Action) {
	case assistantIntentCreateOrgUnit:
		diff = append(diff,
			map[string]any{"field": "name", "after": intent.EntityName},
			map[string]any{"field": "effective_date", "after": intent.EffectiveDate},
		)
		if resolvedCandidateID != "" {
			diff = append(diff, map[string]any{"field": "parent_candidate_id", "after": resolvedCandidateID})
		} else if len(candidates) > 1 {
			diff = append(diff, map[string]any{"field": "parent_candidate_id", "after": "pending_confirmation"})
		}
	case assistantIntentAddOrgUnitVersion, assistantIntentInsertOrgUnitVersion:
		diff = append(diff,
			map[string]any{"field": "org_code", "after": intent.OrgCode},
			map[string]any{"field": "effective_date", "after": intent.EffectiveDate},
		)
		if strings.TrimSpace(intent.NewName) != "" {
			diff = append(diff, map[string]any{"field": "new_name", "after": intent.NewName})
		}
		if resolvedCandidateID != "" {
			diff = append(diff, map[string]any{"field": "new_parent_candidate_id", "after": resolvedCandidateID})
		} else if len(candidates) > 1 {
			diff = append(diff, map[string]any{"field": "new_parent_candidate_id", "after": "pending_confirmation"})
		}
	case assistantIntentCorrectOrgUnit:
		diff = append(diff,
			map[string]any{"field": "org_code", "after": intent.OrgCode},
			map[string]any{"field": "target_effective_date", "after": intent.TargetEffectiveDate},
		)
		if strings.TrimSpace(intent.NewName) != "" {
			diff = append(diff, map[string]any{"field": "new_name", "after": intent.NewName})
		}
		if resolvedCandidateID != "" {
			diff = append(diff, map[string]any{"field": "new_parent_candidate_id", "after": resolvedCandidateID})
		} else if len(candidates) > 1 {
			diff = append(diff, map[string]any{"field": "new_parent_candidate_id", "after": "pending_confirmation"})
		}
	case assistantIntentRenameOrgUnit:
		diff = append(diff,
			map[string]any{"field": "org_code", "after": intent.OrgCode},
			map[string]any{"field": "effective_date", "after": intent.EffectiveDate},
			map[string]any{"field": "new_name", "after": intent.NewName},
		)
	case assistantIntentMoveOrgUnit:
		diff = append(diff,
			map[string]any{"field": "org_code", "after": intent.OrgCode},
			map[string]any{"field": "effective_date", "after": intent.EffectiveDate},
		)
		if resolvedCandidateID != "" {
			diff = append(diff, map[string]any{"field": "new_parent_candidate_id", "after": resolvedCandidateID})
		} else if len(candidates) > 1 {
			diff = append(diff, map[string]any{"field": "new_parent_candidate_id", "after": "pending_confirmation"})
		}
	case assistantIntentDisableOrgUnit, assistantIntentEnableOrgUnit:
		diff = append(diff,
			map[string]any{"field": "org_code", "after": intent.OrgCode},
			map[string]any{"field": "effective_date", "after": intent.EffectiveDate},
		)
	}
	explain := "计划已生成，等待确认后可提交"
	if candidateRefText := assistantIntentCandidateRefText(intent); candidateRefText != "" && strings.TrimSpace(resolvedCandidateID) == "" {
		switch strings.TrimSpace(retrieval.State) {
		case assistantSemanticRetrievalStateNoMatch:
			validationErrors = append(validationErrors, "parent_candidate_not_found")
		case assistantSemanticRetrievalStateMultipleMatches:
			validationErrors = append(validationErrors, "candidate_confirmation_required")
		case assistantSemanticRetrievalStateSingleMatch:
		case assistantSemanticRetrievalStateDeferredByBoundary:
		default:
			if len(candidates) == 0 {
				validationErrors = append(validationErrors, "parent_candidate_not_found")
			} else if len(candidates) > 1 {
				validationErrors = append(validationErrors, "candidate_confirmation_required")
			}
		}
	}
	validationErrors = assistantNormalizeValidationErrors(validationErrors)
	if len(validationErrors) > 0 {
		explain = assistantDryRunValidationExplain(validationErrors)
	}
	return assistantDryRunResult{
		Diff:             diff,
		Explain:          explain,
		ValidationErrors: validationErrors,
		Retrieval:        retrieval,
		WouldCommit:      false,
	}
}

func assistantBoundaryViolationDetected(text string) bool {
	return assistantBoundaryRE.MatchString(strings.TrimSpace(text))
}

func assistantIntentSchemaInvalid(intent assistantIntentSpec) bool {
	return len(assistantIntentValidationErrors(intent)) > 0
}

func assistantTurnRequiresIntentClarification(turn *assistantTurn) bool {
	if turn == nil {
		return false
	}
	if assistantTurnHasOpenClarification(turn) {
		return true
	}
	routeKind := assistantTurnRouteKind(turn)
	if routeKind != "" && routeKind != assistantRouteKindBusinessAction {
		return true
	}
	if assistantTurnHasRouteClarificationSignal(turn) {
		return true
	}
	for _, code := range assistantNormalizeValidationErrors(turn.DryRun.ValidationErrors) {
		switch code {
		case "missing_parent_ref_text", "missing_new_parent_ref_text", "parent_candidate_not_found", "missing_entity_name", "missing_new_name", "missing_effective_date", "invalid_effective_date_format", "missing_org_code", "missing_target_effective_date", "invalid_target_effective_date_format", "missing_change_fields", "FIELD_REQUIRED_VALUE_MISSING", "PATCH_FIELD_NOT_ALLOWED", "non_business_route":
			return true
		}
	}
	return false
}

func assistantLatestPendingTurn(conversation *assistantConversation) *assistantTurn {
	turn := latestTurn(conversation)
	if turn == nil {
		return nil
	}
	if strings.TrimSpace(turn.State) != assistantStateValidated {
		return nil
	}
	if assistantTurnHasOpenClarification(turn) {
		return turn
	}
	if len(assistantTurnMissingFields(turn)) > 0 && assistantTurnRouteKind(turn) == assistantRouteKindBusinessAction {
		return turn
	}
	return nil
}

func assistantIntentValidationErrors(intent assistantIntentSpec) []string {
	errors := make([]string, 0, 4)
	appendEffectiveDateError := func(value string) {
		effectiveDate := strings.TrimSpace(value)
		if effectiveDate == "" {
			errors = append(errors, "missing_effective_date")
		} else if !assistantDateISORE.MatchString(effectiveDate) {
			errors = append(errors, "invalid_effective_date_format")
		}
	}
	appendTargetDateError := func(value string) {
		targetDate := strings.TrimSpace(value)
		if targetDate == "" {
			errors = append(errors, "missing_target_effective_date")
		} else if !assistantDateISORE.MatchString(targetDate) {
			errors = append(errors, "invalid_target_effective_date_format")
		}
	}
	appendOrgCodeError := func(value string) {
		if strings.TrimSpace(value) == "" {
			errors = append(errors, "missing_org_code")
		}
	}
	appendChangeFieldsError := func() {
		if strings.TrimSpace(intent.NewName) == "" && strings.TrimSpace(intent.NewParentRefText) == "" {
			errors = append(errors, "missing_change_fields")
		}
	}

	switch strings.TrimSpace(intent.Action) {
	case assistantIntentCreateOrgUnit:
		if strings.TrimSpace(intent.ParentRefText) == "" {
			errors = append(errors, "missing_parent_ref_text")
		}
		if strings.TrimSpace(intent.EntityName) == "" {
			errors = append(errors, "missing_entity_name")
		}
		appendEffectiveDateError(intent.EffectiveDate)
	case assistantIntentAddOrgUnitVersion, assistantIntentInsertOrgUnitVersion:
		appendOrgCodeError(intent.OrgCode)
		appendEffectiveDateError(intent.EffectiveDate)
		appendChangeFieldsError()
	case assistantIntentCorrectOrgUnit:
		appendOrgCodeError(intent.OrgCode)
		appendTargetDateError(intent.TargetEffectiveDate)
		appendChangeFieldsError()
	case assistantIntentRenameOrgUnit:
		appendOrgCodeError(intent.OrgCode)
		appendEffectiveDateError(intent.EffectiveDate)
		if strings.TrimSpace(intent.NewName) == "" {
			errors = append(errors, "missing_new_name")
		}
	case assistantIntentMoveOrgUnit:
		appendOrgCodeError(intent.OrgCode)
		appendEffectiveDateError(intent.EffectiveDate)
		if strings.TrimSpace(intent.NewParentRefText) == "" {
			errors = append(errors, "missing_new_parent_ref_text")
		}
	case assistantIntentDisableOrgUnit, assistantIntentEnableOrgUnit:
		appendOrgCodeError(intent.OrgCode)
		appendEffectiveDateError(intent.EffectiveDate)
	}
	return assistantNormalizeValidationErrors(errors)
}

func assistantNormalizeValidationErrors(validationErrors []string) []string {
	if len(validationErrors) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(validationErrors))
	normalized := make([]string, 0, len(validationErrors))
	for _, item := range validationErrors {
		code := strings.TrimSpace(item)
		if code == "" {
			continue
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		normalized = append(normalized, code)
	}
	return normalized
}

func assistantDryRunValidationExplain(validationErrors []string) string {
	if len(validationErrors) == 0 {
		return "计划已生成，等待确认后可提交"
	}
	if len(validationErrors) == 1 && validationErrors[0] == "candidate_confirmation_required" {
		return "检测到多个同名父组织候选，需先确认候选主键"
	}
	if len(validationErrors) == 1 && validationErrors[0] == "parent_candidate_not_found" {
		return "未找到匹配的上级组织，请补充更准确的上级组织名称或编码后继续。"
	}
	hints := make([]string, 0, len(validationErrors))
	for _, code := range validationErrors {
		switch code {
		case "missing_parent_ref_text":
			hints = append(hints, "上级组织（例如：鲜花组织）")
		case "missing_new_parent_ref_text":
			hints = append(hints, "新的上级组织（例如：共享服务中心）")
		case "parent_candidate_not_found":
			hints = append(hints, "更准确的上级组织名称或编码（例如：鲜花组织 / FLOWER-A）")
		case "missing_entity_name":
			hints = append(hints, "部门名称（例如：运营部）")
		case "missing_new_name":
			hints = append(hints, "新的组织名称（例如：运营一部）")
		case "missing_org_code":
			hints = append(hints, "目标组织编码（例如：FLOWER-A）")
		case "missing_effective_date":
			hints = append(hints, "生效日期（YYYY-MM-DD）")
		case "invalid_effective_date_format":
			hints = append(hints, "生效日期格式（YYYY-MM-DD）")
		case "missing_target_effective_date":
			hints = append(hints, "目标版本日期（YYYY-MM-DD）")
		case "invalid_target_effective_date_format":
			hints = append(hints, "目标版本日期格式（YYYY-MM-DD）")
		case "missing_change_fields":
			hints = append(hints, "至少一项变更内容（例如：新名称或新上级组织）")
		case "FIELD_REQUIRED_VALUE_MISSING":
			return "当前组织创建策略缺少可用默认值，请联系管理员补齐 org_code / 组织类型策略后重试。"
		case "PATCH_FIELD_NOT_ALLOWED":
			return "当前租户未启用创建所需组织字段配置，请联系管理员启用 org_type 字段后重试。"
		case "non_business_route":
			hints = append(hints, "当前输入属于非业务动作请求，不会触发提交")
		}
	}
	if len(hints) == 0 {
		return "信息不完整，请继续补充必填信息后重试。"
	}
	return "信息不完整，请通过下一轮对话补充：" + strings.Join(hints, "；")
}

func assistantTurnVersionSnapshot(capabilityKey string) (policyVersion string, compositionVersion string, mappingVersion string) {
	policyVersion = capabilityPolicyVersionBaseline
	if definition, ok := capabilityDefinitionForKey(capabilityKey); ok {
		currentPolicy := strings.TrimSpace(definition.CurrentPolicy)
		if currentPolicy != "" {
			policyVersion = currentPolicy
		}
	}
	compositionVersion = policyVersion
	mappingVersion = policyVersion
	return policyVersion, compositionVersion, mappingVersion
}

func assistantTurnVersionDrifted(turn *assistantTurn) bool {
	if turn == nil {
		return false
	}
	policyVersion, compositionVersion, mappingVersion := assistantTurnVersionSnapshot(turn.Plan.CapabilityKey)
	if strings.TrimSpace(turn.PolicyVersion) != strings.TrimSpace(policyVersion) {
		return true
	}
	if strings.TrimSpace(turn.CompositionVersion) != strings.TrimSpace(compositionVersion) {
		return true
	}
	return strings.TrimSpace(turn.MappingVersion) != strings.TrimSpace(mappingVersion)
}

func assistantIntentCandidateRefText(intent assistantIntentSpec) string {
	switch strings.TrimSpace(intent.Action) {
	case assistantIntentCreateOrgUnit:
		return strings.TrimSpace(intent.ParentRefText)
	case assistantIntentAddOrgUnitVersion, assistantIntentInsertOrgUnitVersion, assistantIntentCorrectOrgUnit, assistantIntentMoveOrgUnit:
		return strings.TrimSpace(intent.NewParentRefText)
	default:
		return ""
	}
}

func assistantIntentCandidateAsOf(intent assistantIntentSpec) string {
	switch strings.TrimSpace(intent.Action) {
	case assistantIntentCorrectOrgUnit:
		return strings.TrimSpace(intent.TargetEffectiveDate)
	default:
		return strings.TrimSpace(intent.EffectiveDate)
	}
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

func latestTurn(conversation *assistantConversation) *assistantTurn {
	if conversation == nil || len(conversation.Turns) == 0 {
		return nil
	}
	for i := len(conversation.Turns) - 1; i >= 0; i-- {
		turn := conversation.Turns[i]
		if turn != nil {
			return turn
		}
	}
	return nil
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
	return conversationID, turnID, action, true
}

func extractAssistantTaskIDFromPath(path string) (taskID string, ok bool) {
	parts := assistantSplitPathSegments(path)
	if len(parts) != 4 {
		return "", false
	}
	if parts[0] != "internal" || parts[1] != "assistant" || parts[2] != "tasks" {
		return "", false
	}
	taskID = strings.TrimSpace(parts[3])
	if taskID == "" {
		return "", false
	}
	return taskID, true
}

func extractAssistantTaskActionPath(path string) (taskID string, action string, ok bool) {
	parts := assistantSplitPathSegments(path)
	if len(parts) != 4 {
		return "", "", false
	}
	if parts[0] != "internal" || parts[1] != "assistant" || parts[2] != "tasks" {
		return "", "", false
	}
	taskAction := strings.TrimSpace(parts[3])
	if taskAction == "" {
		return "", "", false
	}
	index := strings.LastIndex(taskAction, ":")
	if index <= 0 {
		return "", "", false
	}
	taskID = strings.TrimSpace(taskAction[:index])
	action = strings.TrimSpace(taskAction[index+1:])
	if taskID == "" || action == "" {
		return "", "", false
	}
	return taskID, action, true
}

func cloneConversation(in *assistantConversation) *assistantConversation {
	if in == nil {
		return nil
	}
	out := *in
	out.Transitions = append([]assistantStateTransition(nil), in.Transitions...)
	out.Turns = make([]*assistantTurn, 0, len(in.Turns))
	for _, turn := range in.Turns {
		if turn == nil {
			continue
		}
		copyTurn := *turn
		copyTurn.RouteDecision.CandidateActionIDs = append([]string(nil), turn.RouteDecision.CandidateActionIDs...)
		copyTurn.RouteDecision.ReasonCodes = append([]string(nil), turn.RouteDecision.ReasonCodes...)
		if turn.Clarification != nil {
			copyClarification := *turn.Clarification
			copyClarification.RequiredSlots = append([]string(nil), turn.Clarification.RequiredSlots...)
			copyClarification.MissingSlots = append([]string(nil), turn.Clarification.MissingSlots...)
			copyClarification.CandidateActionIDs = append([]string(nil), turn.Clarification.CandidateActionIDs...)
			copyClarification.CandidateIDs = append([]string(nil), turn.Clarification.CandidateIDs...)
			copyClarification.ReasonCodes = append([]string(nil), turn.Clarification.ReasonCodes...)
			copyTurn.Clarification = &copyClarification
		}
		copyTurn.MissingFields = append([]string(nil), turn.MissingFields...)
		copyTurn.Candidates = append([]assistantCandidate(nil), turn.Candidates...)
		copyTurn.DryRun.Diff = append([]map[string]any(nil), turn.DryRun.Diff...)
		copyTurn.DryRun.ValidationErrors = append([]string(nil), turn.DryRun.ValidationErrors...)
		copyTurn.DryRun.CreateOrgUnitProjection = assistantCloneCreateOrgUnitProjectionSnapshot(turn.DryRun.CreateOrgUnitProjection)
		copyTurn.DryRun.OrgUnitVersionProjection = assistantCloneOrgUnitVersionProjectionSnapshot(turn.DryRun.OrgUnitVersionProjection)
		if turn.CommitResult != nil {
			copyResult := *turn.CommitResult
			copyTurn.CommitResult = &copyResult
		}
		if turn.CommitReply != nil {
			copyReply := *turn.CommitReply
			copyTurn.CommitReply = &copyReply
		}
		if turn.ReplyNLG != nil {
			copyReply := *turn.ReplyNLG
			copyTurn.ReplyNLG = &copyReply
		}
		out.Turns = append(out.Turns, &copyTurn)
	}
	assistantRefreshConversationDerivedFields(&out)
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
