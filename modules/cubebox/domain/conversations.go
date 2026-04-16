package domain

import "time"

type Conversation struct {
	ConversationID string             `json:"conversation_id"`
	TenantID       string             `json:"tenant_id"`
	ActorID        string             `json:"actor_id"`
	ActorRole      string             `json:"actor_role"`
	State          string             `json:"state"`
	CurrentPhase   string             `json:"current_phase,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
	Turns          []ConversationTurn `json:"turns"`
	Transitions    []StateTransition  `json:"state_transitions,omitempty"`
}

type ConversationRecord struct {
	ConversationID string
	ActorID        string
	ActorRole      string
	State          string
	CurrentPhase   string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ConversationTurnRecord struct {
	TurnID              string
	UserInput           string
	State               string
	Phase               string
	RiskTier            string
	RequestID           string
	TraceID             string
	PolicyVersion       string
	CompositionVersion  string
	MappingVersion      string
	IntentJSON          []byte
	RouteDecisionJSON   []byte
	ClarificationJSON   []byte
	CandidatesJSON      []byte
	PlanJSON            []byte
	DryRunJSON          []byte
	ResolvedCandidateID string
	SelectedCandidateID string
	AmbiguityCount      int
	Confidence          float64
	ResolutionSource    string
	PendingDraftSummary string
	MissingFieldsJSON   []byte
	CommitResultJSON    []byte
	CommitReplyJSON     []byte
	ErrorCode           string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type StateTransitionRecord struct {
	ID         int64
	TurnID     string
	TurnAction string
	RequestID  string
	TraceID    string
	FromState  string
	ToState    string
	FromPhase  string
	ToPhase    string
	ReasonCode string
	ActorID    string
	ChangedAt  time.Time
}

type ConversationListItem struct {
	ConversationID string                `json:"conversation_id"`
	State          string                `json:"state"`
	UpdatedAt      time.Time             `json:"updated_at"`
	LastTurn       *ConversationLastTurn `json:"last_turn,omitempty"`
}

type ConversationLastTurn struct {
	TurnID    string `json:"turn_id"`
	UserInput string `json:"user_input"`
	State     string `json:"state"`
	RiskTier  string `json:"risk_tier"`
}

type ConversationTurn struct {
	TurnID              string           `json:"turn_id"`
	UserInput           string           `json:"user_input"`
	State               string           `json:"state"`
	Phase               string           `json:"phase,omitempty"`
	RiskTier            string           `json:"risk_tier"`
	RequestID           string           `json:"request_id"`
	TraceID             string           `json:"trace_id"`
	PolicyVersion       string           `json:"policy_version"`
	CompositionVersion  string           `json:"composition_version"`
	MappingVersion      string           `json:"mapping_version"`
	Intent              map[string]any   `json:"intent,omitempty"`
	RouteDecision       map[string]any   `json:"route_decision,omitempty"`
	Clarification       map[string]any   `json:"clarification,omitempty"`
	Candidates          []map[string]any `json:"candidates,omitempty"`
	Plan                map[string]any   `json:"plan,omitempty"`
	DryRun              map[string]any   `json:"dry_run,omitempty"`
	ResolvedCandidateID string           `json:"resolved_candidate_id,omitempty"`
	SelectedCandidateID string           `json:"selected_candidate_id,omitempty"`
	AmbiguityCount      int              `json:"ambiguity_count"`
	Confidence          float64          `json:"confidence"`
	ResolutionSource    string           `json:"resolution_source,omitempty"`
	PendingDraftSummary string           `json:"pending_draft_summary,omitempty"`
	MissingFields       []string         `json:"missing_fields,omitempty"`
	CommitResult        map[string]any   `json:"commit_result,omitempty"`
	CommitReply         map[string]any   `json:"commit_reply,omitempty"`
	ReplyNLG            map[string]any   `json:"reply_nlg,omitempty"`
	ErrorCode           string           `json:"error_code,omitempty"`
	CreatedAt           time.Time        `json:"created_at"`
	UpdatedAt           time.Time        `json:"updated_at"`
}

type StateTransition struct {
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
