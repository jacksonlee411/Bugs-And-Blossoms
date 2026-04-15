package domain

import "time"

type TaskContractSnapshot struct {
	IntentSchemaVersion      string `json:"intent_schema_version"`
	CompilerContractVersion  string `json:"compiler_contract_version"`
	CapabilityMapVersion     string `json:"capability_map_version"`
	SkillManifestDigest      string `json:"skill_manifest_digest"`
	ContextHash              string `json:"context_hash"`
	IntentHash               string `json:"intent_hash"`
	PlanHash                 string `json:"plan_hash"`
	KnowledgeSnapshotDigest  string `json:"knowledge_snapshot_digest,omitempty"`
	RouteCatalogVersion      string `json:"route_catalog_version,omitempty"`
	ResolverContractVersion  string `json:"resolver_contract_version,omitempty"`
	ContextTemplateVersion   string `json:"context_template_version,omitempty"`
	ReplyGuidanceVersion     string `json:"reply_guidance_version,omitempty"`
	PolicyContextDigest      string `json:"policy_context_digest,omitempty"`
	EffectivePolicyVersion   string `json:"effective_policy_version,omitempty"`
	ResolvedSetID            string `json:"resolved_setid,omitempty"`
	SetIDSource              string `json:"setid_source,omitempty"`
	PrecheckProjectionDigest string `json:"precheck_projection_digest,omitempty"`
	MutationPolicyVersion    string `json:"mutation_policy_version,omitempty"`
}

type TaskSubmitRequest struct {
	ConversationID   string               `json:"conversation_id"`
	TurnID           string               `json:"turn_id"`
	TaskType         string               `json:"task_type"`
	RequestID        string               `json:"request_id"`
	TraceID          string               `json:"trace_id"`
	ContractSnapshot TaskContractSnapshot `json:"contract_snapshot"`
}

type TaskReceipt struct {
	TaskID      string    `json:"task_id"`
	TaskType    string    `json:"task_type"`
	Status      string    `json:"status"`
	WorkflowID  string    `json:"workflow_id"`
	SubmittedAt time.Time `json:"submitted_at"`
	PollURI     string    `json:"poll_uri"`
}

type TaskDetail struct {
	TaskID            string               `json:"task_id"`
	TaskType          string               `json:"task_type"`
	Status            string               `json:"status"`
	DispatchStatus    string               `json:"dispatch_status"`
	Attempt           int                  `json:"attempt"`
	MaxAttempts       int                  `json:"max_attempts"`
	LastErrorCode     string               `json:"last_error_code,omitempty"`
	WorkflowID        string               `json:"workflow_id"`
	RequestID         string               `json:"request_id"`
	TraceID           string               `json:"trace_id,omitempty"`
	ConversationID    string               `json:"conversation_id"`
	TurnID            string               `json:"turn_id"`
	SubmittedAt       time.Time            `json:"submitted_at"`
	CancelRequestedAt *time.Time           `json:"cancel_requested_at,omitempty"`
	CompletedAt       *time.Time           `json:"completed_at,omitempty"`
	UpdatedAt         time.Time            `json:"updated_at"`
	ContractSnapshot  TaskContractSnapshot `json:"contract_snapshot"`
}

type TaskCancelResponse struct {
	TaskDetail
	CancelAccepted bool `json:"cancel_accepted"`
}

type TaskRecord struct {
	TaskID                   string
	ConversationID           string
	TurnID                   string
	TaskType                 string
	RequestID                string
	RequestHash              string
	WorkflowID               string
	Status                   string
	DispatchStatus           string
	DispatchAttempt          int
	DispatchDeadlineAt       *time.Time
	Attempt                  int
	MaxAttempts              int
	LastErrorCode            string
	TraceID                  string
	IntentSchemaVersion      string
	CompilerContractVersion  string
	CapabilityMapVersion     string
	SkillManifestDigest      string
	ContextHash              string
	IntentHash               string
	PlanHash                 string
	KnowledgeSnapshotDigest  string
	RouteCatalogVersion      string
	ResolverContractVersion  string
	ContextTemplateVersion   string
	ReplyGuidanceVersion     string
	PolicyContextDigest      string
	EffectivePolicyVersion   string
	ResolvedSetID            string
	SetIDSource              string
	PrecheckProjectionDigest string
	MutationPolicyVersion    string
	SubmittedAt              time.Time
	CancelRequestedAt        *time.Time
	CompletedAt              *time.Time
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

type TaskDispatchOutboxRecord struct {
	TaskID      string
	Status      string
	Attempt     int
	NextRetryAt time.Time
}

type TaskStateUpdate struct {
	TaskID            string
	Status            string
	DispatchStatus    string
	DispatchAttempt   int
	Attempt           int
	LastErrorCode     string
	CancelRequestedAt *time.Time
	CompletedAt       *time.Time
	UpdatedAt         time.Time
}

type TaskEventRecord struct {
	TaskID     string
	FromStatus string
	ToStatus   string
	EventType  string
	ErrorCode  string
	OccurredAt time.Time
}

type TaskDispatchOutboxUpdate struct {
	TaskID      string
	Status      string
	Attempt     int
	NextRetryAt time.Time
	UpdatedAt   time.Time
}
