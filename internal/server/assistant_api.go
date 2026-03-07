package server

import (
	"context"
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

	assistantIntentCreateOrgUnit = "create_orgunit"
)

var (
	assistantParentUnderRE = regexp.MustCompile(`在(.+?)之下`)
	assistantDeptNameRE    = regexp.MustCompile(`名为(.+?)的部门`)
	assistantDateCNRE      = regexp.MustCompile(`(20\d{2})年(\d{1,2})月(\d{1,2})日`)
	assistantDateISORE     = regexp.MustCompile(`(20\d{2}-\d{2}-\d{2})`)
	assistantBoundaryRE    = regexp.MustCompile(`(?i)(\bselect\b|\binsert\s+into\b|\bupdate\s+\S+\s+set\b|\bdelete\s+from\b|\bdrop\s+table\b|\btruncate\s+table\b|\balter\s+table\b|--|/\*|\*/|;)`)
)

type assistantConversationService struct {
	orgStore              OrgUnitStore
	writeSvc              orgunitservices.OrgUnitWriteService
	actionRegistry        assistantActionRegistry
	commitAdapterRegistry assistantCommitAdapterRegistry
	modelGateway          *assistantModelGateway
	gatewayErr            error
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
	TurnID              string                        `json:"turn_id"`
	UserInput           string                        `json:"user_input"`
	State               string                        `json:"state"`
	Phase               string                        `json:"phase,omitempty"`
	RiskTier            string                        `json:"risk_tier"`
	RequestID           string                        `json:"request_id"`
	TraceID             string                        `json:"trace_id"`
	PolicyVersion       string                        `json:"policy_version"`
	CompositionVersion  string                        `json:"composition_version"`
	MappingVersion      string                        `json:"mapping_version"`
	Intent              assistantIntentSpec           `json:"intent"`
	Plan                assistantPlanSummary          `json:"plan"`
	PendingDraftSummary string                        `json:"pending_draft_summary,omitempty"`
	MissingFields       []string                      `json:"missing_fields,omitempty"`
	Candidates          []assistantCandidate          `json:"candidates"`
	ResolvedCandidateID string                        `json:"resolved_candidate_id,omitempty"`
	SelectedCandidateID string                        `json:"selected_candidate_id,omitempty"`
	AmbiguityCount      int                           `json:"ambiguity_count"`
	Confidence          float64                       `json:"confidence"`
	ResolutionSource    string                        `json:"resolution_source,omitempty"`
	DryRun              assistantDryRunResult         `json:"dry_run"`
	CommitResult        *assistantCommitResult        `json:"commit_result,omitempty"`
	CommitReply         *assistantCommitReply         `json:"commit_reply,omitempty"`
	ErrorCode           string                        `json:"error_code,omitempty"`
	ReplyNLG            *assistantRenderReplyResponse `json:"reply_nlg,omitempty"`
	CreatedAt           time.Time                     `json:"created_at"`
	UpdatedAt           time.Time                     `json:"updated_at"`
}

type assistantIntentSpec struct {
	Action              string `json:"action"`
	ParentRefText       string `json:"parent_ref_text,omitempty"`
	EntityName          string `json:"entity_name,omitempty"`
	EffectiveDate       string `json:"effective_date,omitempty"`
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
	Diff             []map[string]any `json:"diff"`
	Explain          string           `json:"explain"`
	ValidationErrors []string         `json:"validation_errors,omitempty"`
	WouldCommit      bool             `json:"would_commit"`
	PlanHash         string           `json:"plan_hash,omitempty"`
}

type assistantCandidate struct {
	OrgID         int     `json:"org_id,omitempty"`
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
	return &assistantConversationService{
		orgStore:     orgStore,
		writeSvc:     writeSvc,
		modelGateway: gateway,
		gatewayErr:   err,
		byID:         make(map[string]*assistantConversation),
		byActorID:    make(map[string][]string),
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
		case errors.Is(err, errAssistantModelProviderUnavailable):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusServiceUnavailable, "ai_model_provider_unavailable", "ai model provider unavailable")
		case errors.Is(err, errAssistantModelTimeout):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusGatewayTimeout, "ai_model_timeout", "ai model timeout")
		case errors.Is(err, errAssistantModelRateLimited):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusTooManyRequests, "ai_model_rate_limited", "ai model rate limited")
		case errors.Is(err, errAssistantModelConfigInvalid):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "ai_model_config_invalid", "ai model config invalid")
		case errors.Is(err, errAssistantRuntimeConfigInvalid):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "ai_runtime_config_invalid", "ai runtime config invalid")
		case errors.Is(err, errAssistantRuntimeConfigMissing):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusServiceUnavailable, "ai_runtime_config_missing", "ai runtime config missing")
		case errors.Is(err, errAssistantModelSecretMissing):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "ai_model_secret_missing", "ai model secret missing")
		case errors.Is(err, errAssistantPlanDeterminismViolation):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "ai_plan_determinism_violation", "ai plan determinism violation")
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
			case errors.Is(err, errAssistantTenantMismatch):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "tenant_mismatch", "tenant mismatch")
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
			case errors.Is(err, errAssistantReplyModelTargetMismatch):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "ai_reply_model_target_mismatch", "assistant reply model target mismatch")
			case errors.Is(err, errAssistantReplyRenderFailed):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "ai_reply_render_failed", "assistant reply render failed")
			case errors.Is(err, errAssistantModelProviderUnavailable):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusServiceUnavailable, "ai_model_provider_unavailable", "ai model provider unavailable")
			case errors.Is(err, errAssistantModelTimeout):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusGatewayTimeout, "ai_model_timeout", "ai model timeout")
			case errors.Is(err, errAssistantModelRateLimited):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusTooManyRequests, "ai_model_rate_limited", "ai model rate limited")
			case errors.Is(err, errAssistantModelConfigInvalid):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "ai_model_config_invalid", "ai model config invalid")
			case errors.Is(err, errAssistantRuntimeConfigInvalid):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "ai_runtime_config_invalid", "ai runtime config invalid")
			case errors.Is(err, errAssistantRuntimeConfigMissing):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusServiceUnavailable, "ai_runtime_config_missing", "ai runtime config missing")
			case errors.Is(err, errAssistantModelSecretMissing):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "ai_model_secret_missing", "ai model secret missing")
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

	resolvedIntent, err := s.resolveIntent(ctx, tenantID, conversationID, userInput)
	if err != nil {
		return nil, err
	}
	intent := assistantMergeIntentWithPendingTurn(resolvedIntent.Intent, assistantLatestPendingTurn(conversation))
	intentValidationErrors := assistantIntentValidationErrors(intent)
	candidates := make([]assistantCandidate, 0)
	resolvedCandidateID := ""
	resolutionSource := ""
	ambiguityCount := 0
	confidence := 0.65
	if intent.Action == assistantIntentCreateOrgUnit && intent.ParentRefText != "" && len(intentValidationErrors) == 0 {
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

	plan := assistantBuildPlan(intent)
	turnCreatedAt := time.Now().UTC()
	plan = assistantFreezeConfirmWindow(plan, turnCreatedAt)
	plan.ModelProvider = resolvedIntent.ProviderName
	plan.ModelName = resolvedIntent.ModelName
	plan.ModelRevision = resolvedIntent.ModelRevision
	skillExecutionPlan, configDeltaPlan := assistantCompileIntentToPlans(intent, resolvedCandidateID)
	plan.SkillExecutionPlan = skillExecutionPlan
	plan.ConfigDeltaPlan = configDeltaPlan
	if _, ok := capabilityDefinitionForKey(plan.CapabilityKey); !ok {
		return nil, errAssistantPlanBoundaryViolation
	}
	dryRun := assistantBuildDryRun(intent, candidates, resolvedCandidateID)
	tempTurn := &assistantTurn{Intent: intent, Plan: plan, Candidates: candidates, ResolvedCandidateID: resolvedCandidateID, DryRun: dryRun}
	if err := s.refreshTurnVersionTuple(ctx, tenantID, tempTurn); err != nil {
		return nil, err
	}
	plan = tempTurn.Plan
	dryRun = tempTurn.DryRun
	if err := assistantAnnotateIntentPlanFn(tenantID, conversationID, userInput, &intent, &plan, &dryRun); err != nil {
		return nil, err
	}
	policyVersion, compositionVersion, mappingVersion := assistantTurnVersionSnapshot(plan.CapabilityKey)

	turn := &assistantTurn{
		TurnID:              "turn_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		UserInput:           userInput,
		State:               assistantStateValidated,
		RiskTier:            assistantRiskTierForIntent(intent),
		RequestID:           "assistant_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		TraceID:             strings.ReplaceAll(uuid.NewString(), "-", ""),
		PolicyVersion:       policyVersion,
		CompositionVersion:  compositionVersion,
		MappingVersion:      mappingVersion,
		Intent:              intent,
		Plan:                plan,
		Candidates:          candidates,
		ResolvedCandidateID: resolvedCandidateID,
		AmbiguityCount:      ambiguityCount,
		Confidence:          confidence,
		ResolutionSource:    resolutionSource,
		DryRun:              dryRun,
		CreatedAt:           turnCreatedAt,
		UpdatedAt:           turnCreatedAt,
	}
	assistantRefreshTurnDerivedFields(turn)

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

func (s *assistantConversationService) commitTurn(ctx context.Context, tenantID string, principal Principal, conversationID string, turnID string) (*assistantConversation, error) {
	if s.pool != nil {
		return s.commitTurnPG(ctx, tenantID, principal, conversationID, turnID)
	}
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
		return nil, errAssistantTenantMismatch
	}
	if principal.ID != conversation.ActorID {
		return nil, errAssistantAuthSnapshotExpired
	}
	if strings.TrimSpace(principal.RoleSlug) != strings.TrimSpace(conversation.ActorRole) {
		return nil, errAssistantRoleDriftDetected
	}
	turn := assistantLookupTurn(conversation, turnID)
	if turn == nil {
		return nil, errAssistantTurnNotFound
	}
	result, applyErr := s.applyCommitTurn(ctx, conversation, turn, principal, tenantID)
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
			OrgID:         item.OrgID,
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
		if riskTier := strings.TrimSpace(spec.RiskTier); riskTier != "" {
			return riskTier
		}
	}
	return "low"
}

func assistantBuildPlan(intent assistantIntentSpec) assistantPlanSummary {
	plan := assistantPlanSummary{
		Title:                   "只读规划",
		CapabilityKey:           "org.orgunit_create.field_policy",
		Summary:                 "生成只读计划，不执行提交",
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
	return plan
}

func assistantBuildDryRun(intent assistantIntentSpec, candidates []assistantCandidate, resolvedCandidateID string) assistantDryRunResult {
	diff := make([]map[string]any, 0, 3)
	validationErrors := assistantIntentValidationErrors(intent)
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
	if intent.Action == assistantIntentCreateOrgUnit && strings.TrimSpace(intent.ParentRefText) != "" && strings.TrimSpace(resolvedCandidateID) == "" {
		if len(candidates) == 0 {
			validationErrors = append(validationErrors, "parent_candidate_not_found")
		} else if len(candidates) > 1 {
			validationErrors = append(validationErrors, "candidate_confirmation_required")
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
		WouldCommit:      false,
	}
}

func assistantDecodeIntent(userInput string) (assistantIntentSpec, error) {
	text := strings.TrimSpace(userInput)
	if assistantBoundaryViolationDetected(text) {
		return assistantIntentSpec{}, errAssistantPlanBoundaryViolation
	}
	intent := assistantExtractIntent(text)
	if assistantIntentSchemaInvalid(intent) {
		return assistantIntentSpec{}, errAssistantPlanSchemaConstrainedDecodeFailed
	}
	plan := assistantBuildPlan(intent)
	if _, ok := capabilityDefinitionForKey(plan.CapabilityKey); !ok {
		return assistantIntentSpec{}, errAssistantPlanBoundaryViolation
	}
	return intent, nil
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
	for _, code := range assistantNormalizeValidationErrors(turn.DryRun.ValidationErrors) {
		switch code {
		case "missing_parent_ref_text", "parent_candidate_not_found", "missing_entity_name", "missing_effective_date", "invalid_effective_date_format":
			return true
		}
	}
	return false
}

func assistantMergeIntentWithPendingTurn(intent assistantIntentSpec, pending *assistantTurn) assistantIntentSpec {
	if pending == nil {
		return intent
	}
	if strings.TrimSpace(pending.Intent.Action) != assistantIntentCreateOrgUnit {
		return intent
	}
	if strings.TrimSpace(pending.Phase) != assistantPhaseAwaitMissingFields {
		return intent
	}
	merged := intent
	if strings.TrimSpace(merged.Action) == "" || strings.TrimSpace(merged.Action) == "plan_only" {
		merged.Action = pending.Intent.Action
	}
	if strings.TrimSpace(merged.ParentRefText) == "" {
		merged.ParentRefText = strings.TrimSpace(pending.Intent.ParentRefText)
	}
	if strings.TrimSpace(merged.EntityName) == "" {
		merged.EntityName = strings.TrimSpace(pending.Intent.EntityName)
	}
	if strings.TrimSpace(merged.EffectiveDate) == "" {
		merged.EffectiveDate = strings.TrimSpace(pending.Intent.EffectiveDate)
	}
	return merged
}

func assistantLatestPendingTurn(conversation *assistantConversation) *assistantTurn {
	turn := latestTurn(conversation)
	if turn == nil {
		return nil
	}
	switch strings.TrimSpace(turn.Phase) {
	case assistantPhaseAwaitMissingFields:
		return turn
	default:
		return nil
	}
}

func assistantIntentValidationErrors(intent assistantIntentSpec) []string {
	if intent.Action != assistantIntentCreateOrgUnit {
		return nil
	}
	errors := make([]string, 0, 3)
	if strings.TrimSpace(intent.ParentRefText) == "" {
		errors = append(errors, "missing_parent_ref_text")
	}
	if strings.TrimSpace(intent.EntityName) == "" {
		errors = append(errors, "missing_entity_name")
	}
	effectiveDate := strings.TrimSpace(intent.EffectiveDate)
	if effectiveDate == "" {
		errors = append(errors, "missing_effective_date")
	} else if !assistantDateISORE.MatchString(effectiveDate) {
		errors = append(errors, "invalid_effective_date_format")
	}
	return errors
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
		case "parent_candidate_not_found":
			hints = append(hints, "更准确的上级组织名称或编码（例如：鲜花组织 / FLOWER-A）")
		case "missing_entity_name":
			hints = append(hints, "部门名称（例如：运营部）")
		case "missing_effective_date":
			hints = append(hints, "成立日期（YYYY-MM-DD）")
		case "invalid_effective_date_format":
			hints = append(hints, "成立日期格式（YYYY-MM-DD）")
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
		copyTurn.MissingFields = append([]string(nil), turn.MissingFields...)
		copyTurn.Candidates = append([]assistantCandidate(nil), turn.Candidates...)
		copyTurn.DryRun.Diff = append([]map[string]any(nil), turn.DryRun.Diff...)
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
