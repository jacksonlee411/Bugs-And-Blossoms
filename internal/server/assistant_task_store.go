package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	assistantTaskTypeAsyncPlan = "assistant_async_plan"

	assistantTaskStatusQueued               = "queued"
	assistantTaskStatusRunning              = "running"
	assistantTaskStatusSucceeded            = "succeeded"
	assistantTaskStatusFailed               = "failed"
	assistantTaskStatusManualTakeoverNeeded = "manual_takeover_required"
	assistantTaskStatusCanceled             = "canceled"

	assistantTaskDispatchPending  = "pending"
	assistantTaskDispatchStarted  = "started"
	assistantTaskDispatchFailed   = "failed"
	assistantTaskDispatchCanceled = "canceled"

	assistantTaskDefaultDispatchDeadline = 10 * time.Minute
	assistantTaskDefaultMaxAttempts      = 3
	assistantTaskDispatchBatchSize       = 5
)

var assistantTaskMarshalFn = json.Marshal

type assistantTaskContractSnapshot struct {
	IntentSchemaVersion     string `json:"intent_schema_version"`
	CompilerContractVersion string `json:"compiler_contract_version"`
	CapabilityMapVersion    string `json:"capability_map_version"`
	SkillManifestDigest     string `json:"skill_manifest_digest"`
	ContextHash             string `json:"context_hash"`
	IntentHash              string `json:"intent_hash"`
	PlanHash                string `json:"plan_hash"`
}

type assistantTaskSubmitRequest struct {
	ConversationID   string                        `json:"conversation_id"`
	TurnID           string                        `json:"turn_id"`
	TaskType         string                        `json:"task_type"`
	RequestID        string                        `json:"request_id"`
	TraceID          string                        `json:"trace_id"`
	ContractSnapshot assistantTaskContractSnapshot `json:"contract_snapshot"`
}

type assistantTaskAsyncReceipt struct {
	TaskID      string    `json:"task_id"`
	TaskType    string    `json:"task_type"`
	Status      string    `json:"status"`
	WorkflowID  string    `json:"workflow_id"`
	SubmittedAt time.Time `json:"submitted_at"`
	PollURI     string    `json:"poll_uri"`
}

type assistantTaskDetailResponse struct {
	TaskID            string                        `json:"task_id"`
	TaskType          string                        `json:"task_type"`
	Status            string                        `json:"status"`
	DispatchStatus    string                        `json:"dispatch_status"`
	Attempt           int                           `json:"attempt"`
	MaxAttempts       int                           `json:"max_attempts"`
	LastErrorCode     string                        `json:"last_error_code,omitempty"`
	WorkflowID        string                        `json:"workflow_id"`
	RequestID         string                        `json:"request_id"`
	TraceID           string                        `json:"trace_id,omitempty"`
	ConversationID    string                        `json:"conversation_id"`
	TurnID            string                        `json:"turn_id"`
	SubmittedAt       time.Time                     `json:"submitted_at"`
	CancelRequestedAt *time.Time                    `json:"cancel_requested_at,omitempty"`
	CompletedAt       *time.Time                    `json:"completed_at,omitempty"`
	UpdatedAt         time.Time                     `json:"updated_at"`
	ContractSnapshot  assistantTaskContractSnapshot `json:"contract_snapshot"`
}

type assistantTaskCancelResponse struct {
	assistantTaskDetailResponse
	CancelAccepted bool `json:"cancel_accepted"`
}

type assistantTaskRecord struct {
	TaskID             string
	TenantID           string
	ConversationID     string
	TurnID             string
	TaskType           string
	RequestID          string
	RequestHash        string
	WorkflowID         string
	Status             string
	DispatchStatus     string
	DispatchAttempt    int
	DispatchDeadlineAt time.Time
	Attempt            int
	MaxAttempts        int
	LastErrorCode      string
	TraceID            string
	ContractSnapshot   assistantTaskContractSnapshot
	SubmittedAt        time.Time
	CancelRequestedAt  *time.Time
	CompletedAt        *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type assistantTaskOutboxRecord struct {
	ID          int64
	TenantID    string
	TaskID      string
	WorkflowID  string
	Status      string
	Attempt     int
	NextRetryAt time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func assistantTaskStatusTerminal(status string) bool {
	switch strings.TrimSpace(status) {
	case assistantTaskStatusSucceeded, assistantTaskStatusFailed, assistantTaskStatusCanceled:
		return true
	default:
		return false
	}
}

func assistantTaskStatusCancellable(status string) bool {
	switch strings.TrimSpace(status) {
	case assistantTaskStatusQueued, assistantTaskStatusRunning, assistantTaskStatusManualTakeoverNeeded:
		return true
	default:
		return false
	}
}

func assistantTaskDispatchBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	backoff := 300 * time.Millisecond
	for i := 1; i < attempt; i++ {
		backoff *= 2
		if backoff >= 2*time.Second {
			return 2 * time.Second
		}
	}
	return backoff
}

func assistantTaskWorkflowID(tenantID string, conversationID string, turnID string, requestID string) string {
	return fmt.Sprintf(
		"assistant_async_orchestration_v1:%s:%s:%s:%s",
		strings.TrimSpace(tenantID),
		strings.TrimSpace(conversationID),
		strings.TrimSpace(turnID),
		strings.TrimSpace(requestID),
	)
}

func assistantTaskRequestHash(req assistantTaskSubmitRequest) (string, error) {
	payload := struct {
		ConversationID   string                        `json:"conversation_id"`
		TurnID           string                        `json:"turn_id"`
		TaskType         string                        `json:"task_type"`
		RequestID        string                        `json:"request_id"`
		ContractSnapshot assistantTaskContractSnapshot `json:"contract_snapshot"`
	}{
		ConversationID:   strings.TrimSpace(req.ConversationID),
		TurnID:           strings.TrimSpace(req.TurnID),
		TaskType:         strings.TrimSpace(req.TaskType),
		RequestID:        strings.TrimSpace(req.RequestID),
		ContractSnapshot: req.ContractSnapshot,
	}
	raw, err := assistantTaskMarshalFn(payload)
	if err != nil {
		return "", err
	}
	return assistantHashText(strings.TrimSpace(req.TaskType) + "\n" + string(raw)), nil
}

func assistantTaskValidateSubmitRequest(req assistantTaskSubmitRequest) error {
	if strings.TrimSpace(req.ConversationID) == "" {
		return errors.New("conversation_id required")
	}
	if strings.TrimSpace(req.TurnID) == "" {
		return errors.New("turn_id required")
	}
	if strings.TrimSpace(req.TaskType) == "" {
		return errors.New("task_type required")
	}
	if strings.TrimSpace(req.TaskType) != assistantTaskTypeAsyncPlan {
		return errors.New("task_type invalid")
	}
	if strings.TrimSpace(req.RequestID) == "" {
		return errors.New("request_id required")
	}
	if strings.TrimSpace(req.ContractSnapshot.IntentSchemaVersion) == "" ||
		strings.TrimSpace(req.ContractSnapshot.CompilerContractVersion) == "" ||
		strings.TrimSpace(req.ContractSnapshot.CapabilityMapVersion) == "" ||
		strings.TrimSpace(req.ContractSnapshot.SkillManifestDigest) == "" ||
		strings.TrimSpace(req.ContractSnapshot.ContextHash) == "" ||
		strings.TrimSpace(req.ContractSnapshot.IntentHash) == "" ||
		strings.TrimSpace(req.ContractSnapshot.PlanHash) == "" {
		return errors.New("contract_snapshot incomplete")
	}
	return nil
}

func assistantTaskValidateSnapshotAgainstTurn(snapshot assistantTaskContractSnapshot, turn *assistantTurn) error {
	if turn == nil {
		return errAssistantTurnNotFound
	}
	if strings.TrimSpace(snapshot.IntentSchemaVersion) != strings.TrimSpace(turn.Intent.IntentSchemaVersion) ||
		strings.TrimSpace(snapshot.CompilerContractVersion) != strings.TrimSpace(turn.Plan.CompilerContractVersion) ||
		strings.TrimSpace(snapshot.CapabilityMapVersion) != strings.TrimSpace(turn.Plan.CapabilityMapVersion) ||
		strings.TrimSpace(snapshot.SkillManifestDigest) != strings.TrimSpace(turn.Plan.SkillManifestDigest) ||
		strings.TrimSpace(snapshot.ContextHash) != strings.TrimSpace(turn.Intent.ContextHash) ||
		strings.TrimSpace(snapshot.IntentHash) != strings.TrimSpace(turn.Intent.IntentHash) ||
		strings.TrimSpace(snapshot.PlanHash) != strings.TrimSpace(turn.DryRun.PlanHash) {
		return errAssistantPlanContractVersionMismatch
	}
	return nil
}

func assistantTaskReceiptFromRecord(record assistantTaskRecord) assistantTaskAsyncReceipt {
	return assistantTaskAsyncReceipt{
		TaskID:      record.TaskID,
		TaskType:    record.TaskType,
		Status:      record.Status,
		WorkflowID:  record.WorkflowID,
		SubmittedAt: record.SubmittedAt,
		PollURI:     "/internal/assistant/tasks/" + record.TaskID,
	}
}

func assistantTaskDetailFromRecord(record assistantTaskRecord) assistantTaskDetailResponse {
	return assistantTaskDetailResponse{
		TaskID:            record.TaskID,
		TaskType:          record.TaskType,
		Status:            record.Status,
		DispatchStatus:    record.DispatchStatus,
		Attempt:           record.Attempt,
		MaxAttempts:       record.MaxAttempts,
		LastErrorCode:     record.LastErrorCode,
		WorkflowID:        record.WorkflowID,
		RequestID:         record.RequestID,
		TraceID:           record.TraceID,
		ConversationID:    record.ConversationID,
		TurnID:            record.TurnID,
		SubmittedAt:       record.SubmittedAt,
		CancelRequestedAt: record.CancelRequestedAt,
		CompletedAt:       record.CompletedAt,
		UpdatedAt:         record.UpdatedAt,
		ContractSnapshot:  record.ContractSnapshot,
	}
}

func assistantBuildTaskSnapshotFromTurn(turn *assistantTurn) assistantTaskContractSnapshot {
	if turn == nil {
		return assistantTaskContractSnapshot{}
	}
	return assistantTaskContractSnapshot{
		IntentSchemaVersion:     strings.TrimSpace(turn.Intent.IntentSchemaVersion),
		CompilerContractVersion: strings.TrimSpace(turn.Plan.CompilerContractVersion),
		CapabilityMapVersion:    strings.TrimSpace(turn.Plan.CapabilityMapVersion),
		SkillManifestDigest:     strings.TrimSpace(turn.Plan.SkillManifestDigest),
		ContextHash:             strings.TrimSpace(turn.Intent.ContextHash),
		IntentHash:              strings.TrimSpace(turn.Intent.IntentHash),
		PlanHash:                strings.TrimSpace(turn.DryRun.PlanHash),
	}
}

func assistantBuildTaskSubmitRequestFromTurn(conversationID string, turn *assistantTurn) (assistantTaskSubmitRequest, error) {
	if turn == nil {
		return assistantTaskSubmitRequest{}, errAssistantTurnNotFound
	}
	req := assistantTaskSubmitRequest{
		ConversationID:   strings.TrimSpace(conversationID),
		TurnID:           strings.TrimSpace(turn.TurnID),
		TaskType:         assistantTaskTypeAsyncPlan,
		RequestID:        strings.TrimSpace(turn.RequestID),
		TraceID:          strings.TrimSpace(turn.TraceID),
		ContractSnapshot: assistantBuildTaskSnapshotFromTurn(turn),
	}
	if err := assistantTaskValidateSubmitRequest(req); err != nil {
		if strings.Contains(err.Error(), "contract_snapshot incomplete") {
			return assistantTaskSubmitRequest{}, errAssistantPlanContractVersionMismatch
		}
		return assistantTaskSubmitRequest{}, err
	}
	return req, nil
}

func assistantTaskRecordFromSubmitRequest(tenantID string, req assistantTaskSubmitRequest, requestHash string, now time.Time) assistantTaskRecord {
	return assistantTaskRecord{
		TaskID:             uuid.NewString(),
		TenantID:           tenantID,
		ConversationID:     strings.TrimSpace(req.ConversationID),
		TurnID:             strings.TrimSpace(req.TurnID),
		TaskType:           assistantTaskTypeAsyncPlan,
		RequestID:          strings.TrimSpace(req.RequestID),
		RequestHash:        requestHash,
		WorkflowID:         assistantTaskWorkflowID(tenantID, req.ConversationID, req.TurnID, req.RequestID),
		Status:             assistantTaskStatusQueued,
		DispatchStatus:     assistantTaskDispatchPending,
		DispatchAttempt:    0,
		DispatchDeadlineAt: now.Add(assistantTaskDefaultDispatchDeadline),
		Attempt:            0,
		MaxAttempts:        assistantTaskDefaultMaxAttempts,
		LastErrorCode:      "",
		TraceID:            strings.TrimSpace(req.TraceID),
		ContractSnapshot:   req.ContractSnapshot,
		SubmittedAt:        now,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

func (s *assistantConversationService) insertAssistantTaskGraphTx(ctx context.Context, tx pgx.Tx, tenantID string, record assistantTaskRecord, now time.Time) error {
	if err := s.insertAssistantTaskTx(ctx, tx, record); err != nil {
		return err
	}
	if err := s.insertAssistantTaskEventTx(ctx, tx, tenantID, record.TaskID, "", assistantTaskStatusQueued, "queued", "", nil, now); err != nil {
		return err
	}
	return s.insertAssistantTaskOutboxTx(ctx, tx, tenantID, record.TaskID, record.WorkflowID, now)
}

func (s *assistantConversationService) submitTask(ctx context.Context, tenantID string, principal Principal, req assistantTaskSubmitRequest) (*assistantTaskAsyncReceipt, error) {
	if s == nil || s.pool == nil {
		return nil, errAssistantTaskWorkflowUnavailable
	}
	return s.submitTaskPG(ctx, tenantID, principal, req)
}

func (s *assistantConversationService) submitCommitTask(ctx context.Context, tenantID string, principal Principal, conversationID string, turnID string) (*assistantTaskAsyncReceipt, error) {
	if s == nil || s.pool == nil {
		return nil, errAssistantTaskWorkflowUnavailable
	}
	return s.submitCommitTaskPG(ctx, tenantID, principal, conversationID, turnID)
}

func (s *assistantConversationService) getTask(ctx context.Context, tenantID string, principal Principal, taskID string) (*assistantTaskDetailResponse, error) {
	if s == nil || s.pool == nil {
		return nil, errAssistantTaskWorkflowUnavailable
	}
	return s.getTaskPG(ctx, tenantID, principal, taskID)
}

func (s *assistantConversationService) cancelTask(ctx context.Context, tenantID string, principal Principal, taskID string) (*assistantTaskCancelResponse, error) {
	if s == nil || s.pool == nil {
		return nil, errAssistantTaskWorkflowUnavailable
	}
	return s.cancelTaskPG(ctx, tenantID, principal, taskID)
}

func (s *assistantConversationService) submitTaskPG(ctx context.Context, tenantID string, principal Principal, req assistantTaskSubmitRequest) (*assistantTaskAsyncReceipt, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := assistantTaskValidateSubmitRequest(req); err != nil {
		return nil, err
	}
	requestHash, err := assistantTaskRequestHash(req)
	if err != nil {
		return nil, err
	}

	tx, err := s.beginAssistantTx(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	conversation, err := s.loadConversationTx(ctx, tx, tenantID, req.ConversationID, true)
	if err != nil {
		return nil, err
	}
	if conversation.ActorID != principal.ID {
		return nil, errAssistantConversationForbidden
	}
	turn := assistantLookupTurn(conversation, req.TurnID)
	if turn == nil {
		return nil, errAssistantTurnNotFound
	}
	if err := assistantTaskValidateSnapshotAgainstTurn(req.ContractSnapshot, turn); err != nil {
		return nil, err
	}

	existing, exists, err := s.loadAssistantTaskBySubmitKeyTx(ctx, tx, tenantID, req.ConversationID, req.TurnID, req.RequestID, true)
	if err != nil {
		return nil, err
	}
	if exists {
		if strings.TrimSpace(existing.RequestHash) != strings.TrimSpace(requestHash) {
			return nil, errAssistantIdempotencyKeyConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		receipt := assistantTaskReceiptFromRecord(existing)
		return &receipt, nil
	}

	now := time.Now().UTC()
	record := assistantTaskRecordFromSubmitRequest(tenantID, req, requestHash, now)
	if err := s.insertAssistantTaskGraphTx(ctx, tx, tenantID, record, now); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	receipt := assistantTaskReceiptFromRecord(record)
	return &receipt, nil
}

func (s *assistantConversationService) submitCommitTaskPG(ctx context.Context, tenantID string, principal Principal, conversationID string, turnID string) (*assistantTaskAsyncReceipt, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	tx, err := s.beginAssistantTx(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	conversation, err := s.loadConversationTx(ctx, tx, tenantID, conversationID, true)
	if err != nil {
		return nil, err
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

	claimKey := assistantIdempotencyKey{
		TenantID:       tenantID,
		ConversationID: conversationID,
		TurnID:         turnID,
		TurnAction:     "commit",
		RequestID:      turn.RequestID,
	}
	claim, err := s.claimIdempotencyTx(ctx, tx, claimKey, assistantHashText("commit\n"))
	if err != nil {
		return nil, err
	}
	switch claim.State {
	case assistantIdempotencyClaimConflict:
		return nil, errAssistantIdempotencyKeyConflict
	case assistantIdempotencyClaimInProgress:
		return nil, errAssistantRequestInProgress
	case assistantIdempotencyClaimDone:
		return assistantRestoreTaskReceiptFromIdempotency(claim)
	}

	result := assistantTurnMutationResult{}
	prepared, result, preErr := s.prepareCommitTurn(ctx, conversation, turn, principal, tenantID)
	assistantRefreshConversationDerivedFields(conversation)
	if preErr != nil {
		if err := s.persistConversationTurnMutationTx(ctx, tx, tenantID, conversation, turn, result); err != nil {
			return nil, err
		}
		if err := s.finalizeIdempotencyErrorTx(ctx, tx, claimKey, preErr); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return nil, preErr
	}
	if prepared.SkipExecution {
		if err := s.finalizeIdempotencyErrorTx(ctx, tx, claimKey, errAssistantTaskStateInvalid); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return nil, errAssistantTaskStateInvalid
	}

	req, err := assistantBuildTaskSubmitRequestFromTurn(conversationID, turn)
	if err != nil {
		if finalizeErr := s.finalizeIdempotencyErrorTx(ctx, tx, claimKey, err); finalizeErr != nil {
			return nil, finalizeErr
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return nil, commitErr
		}
		return nil, err
	}
	requestHash, err := assistantTaskRequestHash(req)
	if err != nil {
		if finalizeErr := s.finalizeIdempotencyErrorTx(ctx, tx, claimKey, err); finalizeErr != nil {
			return nil, finalizeErr
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return nil, commitErr
		}
		return nil, err
	}
	existing, exists, err := s.loadAssistantTaskBySubmitKeyTx(ctx, tx, tenantID, conversationID, turnID, req.RequestID, true)
	if err != nil {
		return nil, err
	}
	if exists {
		if strings.TrimSpace(existing.RequestHash) != strings.TrimSpace(requestHash) {
			if finalizeErr := s.finalizeIdempotencyErrorTx(ctx, tx, claimKey, errAssistantIdempotencyKeyConflict); finalizeErr != nil {
				return nil, finalizeErr
			}
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return nil, commitErr
			}
			return nil, errAssistantIdempotencyKeyConflict
		}
		receipt := assistantTaskReceiptFromRecord(existing)
		if err := s.finalizeIdempotencyJSONSuccessTx(ctx, tx, claimKey, http.StatusAccepted, receipt); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return &receipt, nil
	}

	now := time.Now().UTC()
	record := assistantTaskRecordFromSubmitRequest(tenantID, req, requestHash, now)
	if err := s.insertAssistantTaskGraphTx(ctx, tx, tenantID, record, now); err != nil {
		return nil, err
	}
	receipt := assistantTaskReceiptFromRecord(record)
	if err := s.finalizeIdempotencyJSONSuccessTx(ctx, tx, claimKey, http.StatusAccepted, receipt); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &receipt, nil
}

func (s *assistantConversationService) getTaskPG(ctx context.Context, tenantID string, principal Principal, taskID string) (*assistantTaskDetailResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	_ = s.dispatchAssistantTasks(ctx, tenantID, assistantTaskDispatchBatchSize)

	tx, err := s.beginAssistantTx(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	record, exists, err := s.loadAssistantTaskByIDTx(ctx, tx, tenantID, taskID, false)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errAssistantTaskNotFound
	}
	if err := s.ensureAssistantTaskActorTx(ctx, tx, tenantID, record.ConversationID, principal.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	view := assistantTaskDetailFromRecord(record)
	return &view, nil
}

func (s *assistantConversationService) cancelTaskPG(ctx context.Context, tenantID string, principal Principal, taskID string) (*assistantTaskCancelResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	tx, err := s.beginAssistantTx(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	record, exists, err := s.loadAssistantTaskByIDTx(ctx, tx, tenantID, taskID, true)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errAssistantTaskNotFound
	}
	if err := s.ensureAssistantTaskActorTx(ctx, tx, tenantID, record.ConversationID, principal.ID); err != nil {
		return nil, err
	}

	if assistantTaskStatusTerminal(record.Status) {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		view := assistantTaskDetailFromRecord(record)
		return &assistantTaskCancelResponse{assistantTaskDetailResponse: view, CancelAccepted: false}, nil
	}
	if !assistantTaskStatusCancellable(record.Status) {
		return nil, errAssistantTaskCancelNotAllowed
	}

	now := time.Now().UTC()
	cancelRequestedAt := now
	record.CancelRequestedAt = &cancelRequestedAt
	if strings.TrimSpace(record.DispatchStatus) == assistantTaskDispatchPending {
		record.DispatchStatus = assistantTaskDispatchFailed
	}
	fromStatus := record.Status
	record.Status = assistantTaskStatusCanceled
	record.CompletedAt = &now
	record.UpdatedAt = now
	record.LastErrorCode = ""
	if err := s.updateAssistantTaskStateTx(ctx, tx, record); err != nil {
		return nil, err
	}
	if err := s.insertAssistantTaskEventTx(ctx, tx, tenantID, record.TaskID, fromStatus, assistantTaskStatusCanceled, "cancel_requested", "", nil, now); err != nil {
		return nil, err
	}
	if err := s.insertAssistantTaskEventTx(ctx, tx, tenantID, record.TaskID, fromStatus, assistantTaskStatusCanceled, "canceled", "", nil, now); err != nil {
		return nil, err
	}
	if err := s.markAssistantTaskOutboxCanceledTx(ctx, tx, tenantID, record.TaskID, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	view := assistantTaskDetailFromRecord(record)
	return &assistantTaskCancelResponse{assistantTaskDetailResponse: view, CancelAccepted: true}, nil
}

func (s *assistantConversationService) dispatchAssistantTasks(ctx context.Context, tenantID string, batchSize int) error {
	if s == nil || s.pool == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if batchSize <= 0 {
		batchSize = 1
	}
	tx, err := s.beginAssistantTx(ctx, tenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	outboxRows, err := s.selectAssistantTaskOutboxPendingTx(ctx, tx, batchSize)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, outbox := range outboxRows {
		task, exists, loadErr := s.loadAssistantTaskByIDTx(ctx, tx, tenantID, outbox.TaskID, true)
		if loadErr != nil {
			return loadErr
		}
		if !exists {
			if err := s.updateAssistantTaskOutboxStateTx(ctx, tx, outbox.ID, assistantTaskDispatchFailed, outbox.Attempt, outbox.NextRetryAt, now); err != nil {
				return err
			}
			continue
		}
		if assistantTaskStatusTerminal(task.Status) {
			if err := s.updateAssistantTaskOutboxStateTx(ctx, tx, outbox.ID, assistantTaskDispatchFailed, outbox.Attempt, outbox.NextRetryAt, now); err != nil {
				return err
			}
			continue
		}
		attempt := outbox.Attempt + 1
		task.DispatchAttempt = attempt
		task.UpdatedAt = now
		if now.After(task.DispatchDeadlineAt) {
			if err := s.markAssistantTaskDispatchDeadlineExceededTx(ctx, tx, &task, now); err != nil {
				return err
			}
			if err := s.updateAssistantTaskOutboxStateTx(ctx, tx, outbox.ID, assistantTaskDispatchFailed, attempt, outbox.NextRetryAt, now); err != nil {
				return err
			}
			continue
		}
		if err := s.executeAssistantTaskWorkflowTx(ctx, tx, tenantID, &task, now); err != nil {
			nextRetryAt := now.Add(assistantTaskDispatchBackoff(attempt))
			if attempt >= task.MaxAttempts || nextRetryAt.After(task.DispatchDeadlineAt) {
				if err := s.markAssistantTaskDispatchFailureTx(ctx, tx, &task, now); err != nil {
					return err
				}
				if err := s.updateAssistantTaskOutboxStateTx(ctx, tx, outbox.ID, assistantTaskDispatchFailed, attempt, nextRetryAt, now); err != nil {
					return err
				}
				continue
			}
			task.DispatchStatus = assistantTaskDispatchPending
			task.LastErrorCode = "assistant_task_workflow_unavailable"
			task.UpdatedAt = now
			if err := s.updateAssistantTaskStateTx(ctx, tx, task); err != nil {
				return err
			}
			if err := s.updateAssistantTaskOutboxStateTx(ctx, tx, outbox.ID, assistantTaskDispatchPending, attempt, nextRetryAt, now); err != nil {
				return err
			}
			continue
		}
		if err := s.updateAssistantTaskOutboxStateTx(ctx, tx, outbox.ID, assistantTaskDispatchStarted, attempt, outbox.NextRetryAt, now); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *assistantConversationService) executeAssistantTaskWorkflowTx(ctx context.Context, tx pgx.Tx, tenantID string, task *assistantTaskRecord, now time.Time) error {
	if task == nil {
		return errAssistantTaskStateInvalid
	}
	if assistantTaskStatusTerminal(task.Status) {
		return nil
	}
	fromStatus := task.Status
	if strings.TrimSpace(task.Status) == assistantTaskStatusQueued {
		task.Status = assistantTaskStatusRunning
		task.Attempt++
		task.DispatchStatus = assistantTaskDispatchStarted
		task.UpdatedAt = now
		task.LastErrorCode = ""
		if err := s.updateAssistantTaskStateTx(ctx, tx, *task); err != nil {
			return err
		}
		if err := s.insertAssistantTaskEventTx(ctx, tx, tenantID, task.TaskID, fromStatus, task.Status, "running", "", nil, now); err != nil {
			return err
		}
		fromStatus = task.Status
	}

	currentSnapshot, err := s.loadAssistantTurnContractSnapshotTx(ctx, tx, tenantID, task.ConversationID, task.TurnID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errAssistantTaskStateInvalid
		}
		return err
	}
	if currentSnapshot != task.ContractSnapshot {
		return s.markAssistantTaskManualTakeoverTx(ctx, tx, tenantID, task, fromStatus, "ai_plan_contract_version_mismatch", now)
	}
	if strings.TrimSpace(task.ContractSnapshot.PlanHash) == "" {
		return s.markAssistantTaskManualTakeoverTx(ctx, tx, tenantID, task, fromStatus, "ai_plan_determinism_violation", now)
	}

	conversation, err := s.loadConversationTx(ctx, tx, tenantID, task.ConversationID, true)
	if err != nil {
		if errors.Is(err, errAssistantConversationNotFound) {
			return s.markAssistantTaskManualTakeoverTx(ctx, tx, tenantID, task, fromStatus, errAssistantConversationNotFound.Error(), now)
		}
		return err
	}
	turn := assistantLookupTurn(conversation, task.TurnID)
	if turn == nil {
		return s.markAssistantTaskManualTakeoverTx(ctx, tx, tenantID, task, fromStatus, errAssistantTurnNotFound.Error(), now)
	}
	principal := assistantTaskExecutionPrincipal(conversation)
	_, applyErr, execErr := s.executeCommitCoreTx(ctx, tx, tenantID, principal, conversation, turn)
	if execErr != nil {
		return execErr
	}
	if applyErr != nil {
		return s.markAssistantTaskManualTakeoverTx(ctx, tx, tenantID, task, fromStatus, assistantTaskErrorCode(applyErr), now)
	}

	task.Status = assistantTaskStatusSucceeded
	task.DispatchStatus = assistantTaskDispatchStarted
	task.LastErrorCode = ""
	task.CompletedAt = &now
	task.UpdatedAt = now
	if err := s.updateAssistantTaskStateTx(ctx, tx, *task); err != nil {
		return err
	}
	if err := s.insertAssistantTaskEventTx(ctx, tx, tenantID, task.TaskID, fromStatus, task.Status, "succeeded", "", nil, now); err != nil {
		return err
	}
	return nil
}

func assistantTaskExecutionPrincipal(conversation *assistantConversation) Principal {
	if conversation == nil {
		return Principal{}
	}
	return Principal{
		ID:       strings.TrimSpace(conversation.ActorID),
		TenantID: strings.TrimSpace(conversation.TenantID),
		RoleSlug: strings.TrimSpace(conversation.ActorRole),
	}
}

func assistantTaskErrorCode(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(err.Error())
}

func (s *assistantConversationService) markAssistantTaskManualTakeoverTx(ctx context.Context, tx pgx.Tx, tenantID string, task *assistantTaskRecord, fromStatus string, errorCode string, now time.Time) error {
	if task == nil {
		return errAssistantTaskStateInvalid
	}
	task.Status = assistantTaskStatusManualTakeoverNeeded
	task.LastErrorCode = strings.TrimSpace(errorCode)
	task.CompletedAt = &now
	task.UpdatedAt = now
	if err := s.updateAssistantTaskStateTx(ctx, tx, *task); err != nil {
		return err
	}
	if err := s.insertAssistantTaskEventTx(ctx, tx, tenantID, task.TaskID, fromStatus, task.Status, "manual_takeover_required", task.LastErrorCode, nil, now); err != nil {
		return err
	}
	return s.insertAssistantTaskEventTx(ctx, tx, tenantID, task.TaskID, task.Status, task.Status, "dead_lettered", task.LastErrorCode, nil, now)
}

func (s *assistantConversationService) markAssistantTaskDispatchFailureTx(ctx context.Context, tx pgx.Tx, task *assistantTaskRecord, now time.Time) error {
	task.Status = assistantTaskStatusManualTakeoverNeeded
	task.DispatchStatus = assistantTaskDispatchFailed
	task.LastErrorCode = "assistant_task_dispatch_failed"
	task.CompletedAt = &now
	task.UpdatedAt = now
	if err := s.updateAssistantTaskStateTx(ctx, tx, *task); err != nil {
		return err
	}
	if err := s.insertAssistantTaskEventTx(ctx, tx, task.TenantID, task.TaskID, assistantTaskStatusRunning, task.Status, "manual_takeover_required", task.LastErrorCode, nil, now); err != nil {
		return err
	}
	return s.insertAssistantTaskEventTx(ctx, tx, task.TenantID, task.TaskID, task.Status, task.Status, "dead_lettered", task.LastErrorCode, nil, now)
}

func (s *assistantConversationService) markAssistantTaskDispatchDeadlineExceededTx(ctx context.Context, tx pgx.Tx, task *assistantTaskRecord, now time.Time) error {
	task.Status = assistantTaskStatusManualTakeoverNeeded
	task.DispatchStatus = assistantTaskDispatchFailed
	task.LastErrorCode = "assistant_task_dispatch_failed"
	task.CompletedAt = &now
	task.UpdatedAt = now
	if err := s.updateAssistantTaskStateTx(ctx, tx, *task); err != nil {
		return err
	}
	if err := s.insertAssistantTaskEventTx(ctx, tx, task.TenantID, task.TaskID, assistantTaskStatusQueued, task.Status, "manual_takeover_required", task.LastErrorCode, nil, now); err != nil {
		return err
	}
	return s.insertAssistantTaskEventTx(ctx, tx, task.TenantID, task.TaskID, task.Status, task.Status, "dead_lettered", task.LastErrorCode, nil, now)
}

func (s *assistantConversationService) loadAssistantTaskBySubmitKeyTx(ctx context.Context, tx pgx.Tx, tenantID string, conversationID string, turnID string, requestID string, forUpdate bool) (assistantTaskRecord, bool, error) {
	query := `
SELECT
  task_id::text,
  tenant_uuid::text,
  conversation_id,
  turn_id,
  task_type,
  request_id,
  request_hash,
  workflow_id,
  status,
  dispatch_status,
  dispatch_attempt,
  dispatch_deadline_at,
  attempt,
  max_attempts,
  last_error_code,
  trace_id,
  intent_schema_version,
  compiler_contract_version,
  capability_map_version,
  skill_manifest_digest,
  context_hash,
  intent_hash,
  plan_hash,
  submitted_at,
  cancel_requested_at,
  completed_at,
  created_at,
  updated_at
FROM iam.assistant_tasks
WHERE tenant_uuid = $1::uuid
  AND conversation_id = $2
  AND turn_id = $3
  AND request_id = $4`
	if forUpdate {
		query += " FOR UPDATE"
	}
	return scanAssistantTaskRecord(tx.QueryRow(ctx, query, tenantID, conversationID, turnID, requestID))
}

func (s *assistantConversationService) loadAssistantTaskByIDTx(ctx context.Context, tx pgx.Tx, tenantID string, taskID string, forUpdate bool) (assistantTaskRecord, bool, error) {
	query := `
SELECT
  task_id::text,
  tenant_uuid::text,
  conversation_id,
  turn_id,
  task_type,
  request_id,
  request_hash,
  workflow_id,
  status,
  dispatch_status,
  dispatch_attempt,
  dispatch_deadline_at,
  attempt,
  max_attempts,
  last_error_code,
  trace_id,
  intent_schema_version,
  compiler_contract_version,
  capability_map_version,
  skill_manifest_digest,
  context_hash,
  intent_hash,
  plan_hash,
  submitted_at,
  cancel_requested_at,
  completed_at,
  created_at,
  updated_at
FROM iam.assistant_tasks
WHERE tenant_uuid = $1::uuid
  AND task_id = $2::uuid`
	if forUpdate {
		query += " FOR UPDATE"
	}
	return scanAssistantTaskRecord(tx.QueryRow(ctx, query, tenantID, taskID))
}

func scanAssistantTaskRecord(row pgx.Row) (assistantTaskRecord, bool, error) {
	var (
		record            assistantTaskRecord
		lastErrorCode     sql.NullString
		traceID           sql.NullString
		cancelRequestedAt *time.Time
		completedAt       *time.Time
	)
	err := row.Scan(
		&record.TaskID,
		&record.TenantID,
		&record.ConversationID,
		&record.TurnID,
		&record.TaskType,
		&record.RequestID,
		&record.RequestHash,
		&record.WorkflowID,
		&record.Status,
		&record.DispatchStatus,
		&record.DispatchAttempt,
		&record.DispatchDeadlineAt,
		&record.Attempt,
		&record.MaxAttempts,
		&lastErrorCode,
		&traceID,
		&record.ContractSnapshot.IntentSchemaVersion,
		&record.ContractSnapshot.CompilerContractVersion,
		&record.ContractSnapshot.CapabilityMapVersion,
		&record.ContractSnapshot.SkillManifestDigest,
		&record.ContractSnapshot.ContextHash,
		&record.ContractSnapshot.IntentHash,
		&record.ContractSnapshot.PlanHash,
		&record.SubmittedAt,
		&cancelRequestedAt,
		&completedAt,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return assistantTaskRecord{}, false, nil
		}
		return assistantTaskRecord{}, false, err
	}
	if lastErrorCode.Valid {
		record.LastErrorCode = lastErrorCode.String
	}
	if traceID.Valid {
		record.TraceID = traceID.String
	}
	record.CancelRequestedAt = cancelRequestedAt
	record.CompletedAt = completedAt
	return record, true, nil
}

func (s *assistantConversationService) insertAssistantTaskTx(ctx context.Context, tx pgx.Tx, record assistantTaskRecord) error {
	_, err := tx.Exec(ctx, `
INSERT INTO iam.assistant_tasks (
  tenant_uuid,
  task_id,
  conversation_id,
  turn_id,
  task_type,
  request_id,
  request_hash,
  workflow_id,
  status,
  dispatch_status,
  dispatch_attempt,
  dispatch_deadline_at,
  attempt,
  max_attempts,
  last_error_code,
  trace_id,
  intent_schema_version,
  compiler_contract_version,
  capability_map_version,
  skill_manifest_digest,
  context_hash,
  intent_hash,
  plan_hash,
  submitted_at,
  cancel_requested_at,
  completed_at,
  created_at,
  updated_at
) VALUES (
  $1::uuid,
  $2::uuid,
  $3,
  $4,
  $5,
  $6,
  $7,
  $8,
  $9,
  $10,
  $11,
  $12,
  $13,
  $14,
  NULLIF($15, ''),
  NULLIF($16, ''),
  $17,
  $18,
  $19,
  $20,
  $21,
  $22,
  $23,
  $24,
  $25,
  $26,
  $27,
  $28
)
`, record.TenantID, record.TaskID, record.ConversationID, record.TurnID, record.TaskType, record.RequestID, record.RequestHash, record.WorkflowID,
		record.Status, record.DispatchStatus, record.DispatchAttempt, record.DispatchDeadlineAt, record.Attempt, record.MaxAttempts,
		record.LastErrorCode, record.TraceID, record.ContractSnapshot.IntentSchemaVersion, record.ContractSnapshot.CompilerContractVersion,
		record.ContractSnapshot.CapabilityMapVersion, record.ContractSnapshot.SkillManifestDigest, record.ContractSnapshot.ContextHash,
		record.ContractSnapshot.IntentHash, record.ContractSnapshot.PlanHash, record.SubmittedAt, record.CancelRequestedAt,
		record.CompletedAt, record.CreatedAt, record.UpdatedAt)
	return err
}

func (s *assistantConversationService) updateAssistantTaskStateTx(ctx context.Context, tx pgx.Tx, record assistantTaskRecord) error {
	_, err := tx.Exec(ctx, `
UPDATE iam.assistant_tasks
SET status = $3,
    dispatch_status = $4,
    dispatch_attempt = $5,
    attempt = $6,
    last_error_code = NULLIF($7, ''),
    cancel_requested_at = $8,
    completed_at = $9,
    updated_at = $10
WHERE tenant_uuid = $1::uuid
  AND task_id = $2::uuid
`, record.TenantID, record.TaskID, record.Status, record.DispatchStatus, record.DispatchAttempt, record.Attempt, record.LastErrorCode, record.CancelRequestedAt, record.CompletedAt, record.UpdatedAt)
	return err
}

func (s *assistantConversationService) ensureAssistantTaskActorTx(ctx context.Context, tx pgx.Tx, tenantID string, conversationID string, actorID string) error {
	var storedActorID string
	if err := tx.QueryRow(ctx, `
SELECT actor_id
FROM iam.assistant_conversations
WHERE tenant_uuid = $1::uuid
  AND conversation_id = $2
`, tenantID, conversationID).Scan(&storedActorID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errAssistantConversationNotFound
		}
		return err
	}
	if strings.TrimSpace(storedActorID) != strings.TrimSpace(actorID) {
		return errAssistantConversationForbidden
	}
	return nil
}

func (s *assistantConversationService) insertAssistantTaskEventTx(
	ctx context.Context,
	tx pgx.Tx,
	tenantID string,
	taskID string,
	fromStatus string,
	toStatus string,
	eventType string,
	errorCode string,
	payload map[string]any,
	occurredAt time.Time,
) error {
	var fromStatusArg any
	if strings.TrimSpace(fromStatus) != "" {
		fromStatusArg = strings.TrimSpace(fromStatus)
	}
	var errorCodeArg any
	if strings.TrimSpace(errorCode) != "" {
		errorCodeArg = strings.TrimSpace(errorCode)
	}
	var payloadArg any
	if payload != nil {
		raw, err := assistantTaskMarshalFn(payload)
		if err != nil {
			return err
		}
		payloadArg = string(raw)
	}
	_, err := tx.Exec(ctx, `
INSERT INTO iam.assistant_task_events (
  tenant_uuid,
  task_id,
  from_status,
  to_status,
  event_type,
  error_code,
  payload,
  occurred_at
) VALUES (
  $1::uuid,
  $2::uuid,
  $3,
  $4,
  $5,
  $6,
  $7::jsonb,
  $8
)
`, tenantID, taskID, fromStatusArg, toStatus, eventType, errorCodeArg, payloadArg, occurredAt)
	return err
}

func (s *assistantConversationService) insertAssistantTaskOutboxTx(ctx context.Context, tx pgx.Tx, tenantID string, taskID string, workflowID string, now time.Time) error {
	_, err := tx.Exec(ctx, `
INSERT INTO iam.assistant_task_dispatch_outbox (
  tenant_uuid,
  task_id,
  workflow_id,
  status,
  attempt,
  next_retry_at,
  created_at,
  updated_at
) VALUES (
  $1::uuid,
  $2::uuid,
  $3,
  'pending',
  0,
  $4,
  $4,
  $4
)
ON CONFLICT (tenant_uuid, task_id)
DO UPDATE SET
  workflow_id = EXCLUDED.workflow_id,
  status = EXCLUDED.status,
  attempt = EXCLUDED.attempt,
  next_retry_at = EXCLUDED.next_retry_at,
  updated_at = EXCLUDED.updated_at
`, tenantID, taskID, workflowID, now)
	return err
}

func (s *assistantConversationService) selectAssistantTaskOutboxPendingTx(ctx context.Context, tx pgx.Tx, batchSize int) ([]assistantTaskOutboxRecord, error) {
	rows, err := tx.Query(ctx, `
SELECT id, tenant_uuid::text, task_id::text, workflow_id, status, attempt, next_retry_at, created_at, updated_at
FROM iam.assistant_task_dispatch_outbox
WHERE status = 'pending'
  AND next_retry_at <= now()
ORDER BY id
FOR UPDATE SKIP LOCKED
LIMIT $1
`, batchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	outbox := make([]assistantTaskOutboxRecord, 0, batchSize)
	for rows.Next() {
		var record assistantTaskOutboxRecord
		if err := rows.Scan(
			&record.ID,
			&record.TenantID,
			&record.TaskID,
			&record.WorkflowID,
			&record.Status,
			&record.Attempt,
			&record.NextRetryAt,
			&record.CreatedAt,
			&record.UpdatedAt,
		); err != nil {
			return nil, err
		}
		outbox = append(outbox, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return outbox, nil
}

func (s *assistantConversationService) updateAssistantTaskOutboxStateTx(ctx context.Context, tx pgx.Tx, outboxID int64, status string, attempt int, nextRetryAt time.Time, updatedAt time.Time) error {
	_, err := tx.Exec(ctx, `
UPDATE iam.assistant_task_dispatch_outbox
SET status = $2,
    attempt = $3,
    next_retry_at = $4,
    updated_at = $5
WHERE id = $1
`, outboxID, status, attempt, nextRetryAt, updatedAt)
	return err
}

func (s *assistantConversationService) markAssistantTaskOutboxCanceledTx(ctx context.Context, tx pgx.Tx, tenantID string, taskID string, now time.Time) error {
	_, err := tx.Exec(ctx, `
UPDATE iam.assistant_task_dispatch_outbox
SET status = 'canceled',
    updated_at = $3
WHERE tenant_uuid = $1::uuid
  AND task_id = $2::uuid
  AND status = 'pending'
`, tenantID, taskID, now)
	return err
}

func (s *assistantConversationService) loadAssistantTurnContractSnapshotTx(ctx context.Context, tx pgx.Tx, tenantID string, conversationID string, turnID string) (assistantTaskContractSnapshot, error) {
	var (
		intentJSON []byte
		planJSON   []byte
		dryRunJSON []byte
	)
	if err := tx.QueryRow(ctx, `
SELECT intent_json, plan_json, dry_run_json
FROM iam.assistant_turns
WHERE tenant_uuid = $1::uuid
  AND conversation_id = $2
  AND turn_id = $3
`, tenantID, conversationID, turnID).Scan(&intentJSON, &planJSON, &dryRunJSON); err != nil {
		return assistantTaskContractSnapshot{}, err
	}
	var intent assistantIntentSpec
	var plan assistantPlanSummary
	var dryRun assistantDryRunResult
	if err := json.Unmarshal(intentJSON, &intent); err != nil {
		return assistantTaskContractSnapshot{}, err
	}
	if err := json.Unmarshal(planJSON, &plan); err != nil {
		return assistantTaskContractSnapshot{}, err
	}
	if err := json.Unmarshal(dryRunJSON, &dryRun); err != nil {
		return assistantTaskContractSnapshot{}, err
	}
	return assistantTaskContractSnapshot{
		IntentSchemaVersion:     strings.TrimSpace(intent.IntentSchemaVersion),
		CompilerContractVersion: strings.TrimSpace(plan.CompilerContractVersion),
		CapabilityMapVersion:    strings.TrimSpace(plan.CapabilityMapVersion),
		SkillManifestDigest:     strings.TrimSpace(plan.SkillManifestDigest),
		ContextHash:             strings.TrimSpace(intent.ContextHash),
		IntentHash:              strings.TrimSpace(intent.IntentHash),
		PlanHash:                strings.TrimSpace(dryRun.PlanHash),
	}, nil
}
