package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	cubeboxdomain "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/domain"
)

const (
	conversationCursorSalt   = "assistant-conversation-cursor-v1"
	healthHealthy            = "healthy"
	healthDegraded           = "degraded"
	healthUnavailable        = "unavailable"
	defaultConversationLimit = 20
	maxConversationLimit     = 100
	taskDispatchBatchSize    = 5
)

var (
	ErrConversationNotFound           = errors.New("cubebox_conversation_not_found")
	ErrConversationForbidden          = errors.New("cubebox_conversation_forbidden")
	ErrTenantMismatch                 = errors.New("cubebox_tenant_mismatch")
	ErrConversationCursorInvalid      = errors.New("cubebox_conversation_cursor_invalid")
	ErrDeleteBlockedByTask            = errors.New("cubebox_conversation_delete_blocked_by_running_task")
	ErrTaskNotFound                   = errors.New("cubebox_task_not_found")
	ErrTurnNotFound                   = errors.New("cubebox_turn_not_found")
	ErrTaskCancelNotAllowed           = errors.New("cubebox_task_cancel_not_allowed")
	ErrTaskStateInvalid               = errors.New("cubebox_task_state_invalid")
	ErrPlanContractMismatch           = errors.New("cubebox_plan_contract_version_mismatch")
	ErrPlanDeterminismViolation       = errors.New("cubebox_plan_determinism_violation")
	ErrIdempotencyConflict            = errors.New("cubebox_idempotency_key_conflict")
	ErrConfirmationRequired           = errors.New("cubebox_confirmation_required")
	ErrConfirmationExpired            = errors.New("cubebox_confirmation_expired")
	ErrConversationStateInvalid       = errors.New("cubebox_conversation_state_invalid")
	ErrAuthSnapshotExpired            = errors.New("cubebox_auth_snapshot_expired")
	ErrRoleDriftDetected              = errors.New("cubebox_role_drift_detected")
	ErrConversationSnapshotSyncFailed = errors.New("cubebox_conversation_snapshot_sync_failed")
)

type ConversationReader interface {
	ListConversations(ctx context.Context, tenantID string, actorID string, limit int32, cursorUpdatedAt time.Time, cursorConversationID string) ([]cubeboxdomain.ConversationRecord, error)
	GetConversation(ctx context.Context, tenantID string, conversationID string) (cubeboxdomain.ConversationRecord, error)
	ListConversationTurns(ctx context.Context, tenantID string, conversationID string) ([]cubeboxdomain.ConversationTurnRecord, error)
	ListConversationStateTransitions(ctx context.Context, tenantID string, conversationID string) ([]cubeboxdomain.StateTransitionRecord, error)
	SyncConversationSnapshot(ctx context.Context, tenantID string, conversation cubeboxdomain.Conversation) error
	CountBlockingTasks(ctx context.Context, tenantID string, conversationID string) (int64, error)
	DeleteConversation(ctx context.Context, tenantID string, conversationID string) (int64, error)
	GetTask(ctx context.Context, tenantID string, taskID string) (cubeboxdomain.TaskRecord, error)
	GetTaskForDispatch(ctx context.Context, tenantID string, taskID string) (cubeboxdomain.TaskRecord, error)
	GetTaskActorID(ctx context.Context, tenantID string, taskID string) (string, error)
	SubmitTask(ctx context.Context, tenantID string, record cubeboxdomain.TaskRecord) (cubeboxdomain.TaskRecord, bool, error)
	CancelTask(ctx context.Context, tenantID string, taskID string, now time.Time) (cubeboxdomain.TaskRecord, bool, error)
	ListDispatchOutbox(ctx context.Context, tenantID string, status string, limit int32) ([]cubeboxdomain.TaskDispatchOutboxRecord, error)
	UpdateTaskState(ctx context.Context, tenantID string, update cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error)
	InsertTaskEvent(ctx context.Context, tenantID string, event cubeboxdomain.TaskEventRecord) error
	UpdateTaskDispatchOutbox(ctx context.Context, tenantID string, update cubeboxdomain.TaskDispatchOutboxUpdate) error
}

type RuntimeProbe interface {
	BackendStatus(ctx context.Context) cubeboxdomain.RuntimeComponentStatus
	KnowledgeRuntimeStatus(ctx context.Context) cubeboxdomain.RuntimeComponentStatus
	ModelGatewayStatus(ctx context.Context) cubeboxdomain.RuntimeComponentStatus
	Models(ctx context.Context) ([]cubeboxdomain.ModelEntry, error)
}

type LegacyFacade interface {
	ListConversations(ctx context.Context, tenantID string, actorID string, pageSize int, cursor string) ([]cubeboxdomain.ConversationListItem, string, error)
	GetConversation(ctx context.Context, tenantID string, actorID string, conversationID string) (*cubeboxdomain.Conversation, error)
	CreateConversation(ctx context.Context, tenantID string, principal Principal) (*cubeboxdomain.Conversation, error)
	CreateTurn(ctx context.Context, tenantID string, principal Principal, conversationID string, userInput string) (*cubeboxdomain.Conversation, error)
	ConfirmTurn(ctx context.Context, tenantID string, principal Principal, conversationID string, turnID string, candidateID string) (*cubeboxdomain.Conversation, error)
	CommitTurn(ctx context.Context, tenantID string, principal Principal, conversationID string, turnID string) (*cubeboxdomain.TaskReceipt, error)
	SubmitTask(ctx context.Context, tenantID string, principal Principal, req cubeboxdomain.TaskSubmitRequest) (*cubeboxdomain.TaskReceipt, error)
	GetTask(ctx context.Context, tenantID string, principal Principal, taskID string) (*cubeboxdomain.TaskDetail, error)
	CancelTask(ctx context.Context, tenantID string, principal Principal, taskID string) (*cubeboxdomain.TaskCancelResponse, error)
	ExecuteTaskWorkflow(ctx context.Context, tenantID string, principal Principal, conversation *cubeboxdomain.Conversation, turnID string) (TaskWorkflowExecutionResult, error)
	RenderReply(ctx context.Context, tenantID string, principal Principal, conversationID string, turnID string, req map[string]any) (map[string]any, error)
}

type TaskWorkflowExecutionResult struct {
	ApplyErrorCode string
	Conversation   *cubeboxdomain.Conversation
}

type Principal struct {
	ID       string
	RoleSlug string
}

type Facade struct {
	reader  ConversationReader
	runtime RuntimeProbe
	files   *FileService
	legacy  LegacyFacade
	nowFn   func() time.Time
}

func NewFacade(reader ConversationReader, runtime RuntimeProbe, files *FileService, legacy LegacyFacade) *Facade {
	return &Facade{
		reader:  reader,
		runtime: runtime,
		files:   files,
		legacy:  legacy,
		nowFn: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (f *Facade) ListConversations(ctx context.Context, tenantID string, actorID string, pageSize int, cursor string) ([]cubeboxdomain.ConversationListItem, string, error) {
	if pageSize <= 0 {
		pageSize = defaultConversationLimit
	}
	if pageSize > maxConversationLimit {
		pageSize = maxConversationLimit
	}
	decoded, err := decodeConversationCursor(cursor, tenantID, actorID)
	if err != nil {
		return nil, "", err
	}
	cursorUpdatedAt := time.Time{}
	cursorConversationID := ""
	if decoded != nil {
		cursorUpdatedAt = decoded.UpdatedAt
		cursorConversationID = decoded.ConversationID
	}
	if f.reader == nil {
		if f.legacy == nil {
			return nil, "", nil
		}
		return f.legacy.ListConversations(ctx, tenantID, actorID, pageSize, cursor)
	}
	rows, err := f.reader.ListConversations(ctx, strings.TrimSpace(tenantID), strings.TrimSpace(actorID), int32(pageSize+1), cursorUpdatedAt, cursorConversationID)
	if err != nil {
		return nil, "", err
	}
	if len(rows) > pageSize+1 {
		rows = rows[:pageSize+1]
	}
	items := make([]cubeboxdomain.ConversationListItem, 0, min(len(rows), pageSize))
	for idx, row := range rows {
		if idx >= pageSize {
			break
		}
			item := cubeboxdomain.ConversationListItem{
				ConversationID: strings.TrimSpace(row.ConversationID),
				State:          strings.TrimSpace(row.State),
				UpdatedAt:      row.UpdatedAt.UTC(),
			}
		if detail, err := f.loadConversation(ctx, tenantID, actorID, row.ConversationID, false); err == nil && detail != nil && len(detail.Turns) > 0 {
			last := detail.Turns[len(detail.Turns)-1]
			item.LastTurn = &cubeboxdomain.ConversationLastTurn{
				TurnID:    last.TurnID,
				UserInput: last.UserInput,
				State:     last.State,
				RiskTier:  last.RiskTier,
			}
		}
		items = append(items, item)
	}
	nextCursor := ""
	if len(rows) > pageSize {
		last := rows[pageSize-1]
		nextCursor = encodeConversationCursor(conversationCursor{
			UpdatedAt:      last.UpdatedAt.UTC(),
			ConversationID: strings.TrimSpace(last.ConversationID),
		}, tenantID, actorID)
	}
	return items, nextCursor, nil
}

func (f *Facade) GetConversation(ctx context.Context, tenantID string, actorID string, conversationID string) (*cubeboxdomain.Conversation, error) {
	return f.loadConversation(ctx, tenantID, actorID, conversationID, true)
}

func (f *Facade) DeleteConversation(ctx context.Context, tenantID string, actorID string, conversationID string) error {
	if _, err := f.GetConversation(ctx, tenantID, actorID, conversationID); err != nil {
		return err
	}
	if f.reader == nil {
		return nil
	}
	blocking, err := f.reader.CountBlockingTasks(ctx, tenantID, conversationID)
	if err != nil {
		return err
	}
	if blocking > 0 {
		return ErrDeleteBlockedByTask
	}
	rows, err := f.reader.DeleteConversation(ctx, tenantID, conversationID)
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrConversationNotFound
	}
	return nil
}

func (f *Facade) CreateConversation(ctx context.Context, tenantID string, principal Principal) (*cubeboxdomain.Conversation, error) {
	if f.legacy == nil {
		return nil, nil
	}
	conversation, err := f.legacy.CreateConversation(ctx, tenantID, principal)
	if err != nil {
		return nil, err
	}
	if err := f.syncConversationSnapshot(ctx, tenantID, conversation); err != nil {
		return nil, err
	}
	return conversation, nil
}

func (f *Facade) CreateTurn(ctx context.Context, tenantID string, principal Principal, conversationID string, userInput string) (*cubeboxdomain.Conversation, error) {
	if f.legacy == nil {
		return nil, nil
	}
	conversation, err := f.legacy.CreateTurn(ctx, tenantID, principal, conversationID, userInput)
	if err != nil {
		return nil, err
	}
	if err := f.syncConversationSnapshot(ctx, tenantID, conversation); err != nil {
		return nil, err
	}
	return conversation, nil
}

func (f *Facade) ConfirmTurn(ctx context.Context, tenantID string, principal Principal, conversationID string, turnID string, candidateID string) (*cubeboxdomain.Conversation, error) {
	if f.legacy == nil {
		return nil, nil
	}
	conversation, err := f.legacy.ConfirmTurn(ctx, tenantID, principal, conversationID, turnID, candidateID)
	if err != nil {
		return nil, err
	}
	if err := f.syncConversationSnapshot(ctx, tenantID, conversation); err != nil {
		return nil, err
	}
	return conversation, nil
}

func (f *Facade) CommitTurn(ctx context.Context, tenantID string, principal Principal, conversationID string, turnID string) (*cubeboxdomain.TaskReceipt, error) {
	if f.reader != nil {
		row, err := f.reader.GetConversation(ctx, tenantID, conversationID)
		if err == nil {
			if strings.TrimSpace(row.ActorID) != strings.TrimSpace(principal.ID) {
				return nil, ErrAuthSnapshotExpired
			}
			if strings.TrimSpace(row.ActorRole) != strings.TrimSpace(principal.RoleSlug) {
				return nil, ErrRoleDriftDetected
			}
			turns, err := f.reader.ListConversationTurns(ctx, tenantID, conversationID)
			if err != nil {
				return nil, err
			}
			turn, ok := findTurn(turns, turnID)
			if !ok {
				return nil, ErrTurnNotFound
			}
			switch strings.TrimSpace(turn.State) {
			case "committed":
				return nil, ErrTaskStateInvalid
			case "canceled", "expired":
				return nil, ErrConversationStateInvalid
			case "validated":
				if turnConfirmationExpired(turn, f.now()) {
					return nil, ErrConfirmationExpired
				}
				return nil, ErrConfirmationRequired
			case "confirmed":
			default:
				return nil, ErrConversationStateInvalid
			}
			req, err := buildCommitTaskSubmitRequest(conversationID, turn)
			if err != nil {
				return nil, err
			}
			return f.SubmitTask(ctx, tenantID, principal, req)
		}
	}
	if f.legacy == nil {
		return nil, nil
	}
	return f.legacy.CommitTurn(ctx, tenantID, principal, conversationID, turnID)
}

func (f *Facade) SubmitTask(ctx context.Context, tenantID string, principal Principal, req cubeboxdomain.TaskSubmitRequest) (*cubeboxdomain.TaskReceipt, error) {
	if f.reader != nil {
		req = normalizeTaskSubmitRequest(req)
		if err := validateTaskSubmitRequest(req); err != nil {
			return nil, err
		}
		row, err := f.reader.GetConversation(ctx, tenantID, req.ConversationID)
		if err != nil {
			return nil, ErrConversationNotFound
		}
		if strings.TrimSpace(row.ActorID) != strings.TrimSpace(principal.ID) {
			return nil, ErrConversationForbidden
		}
		turns, err := f.reader.ListConversationTurns(ctx, tenantID, req.ConversationID)
		if err != nil {
			return nil, err
		}
		turn, ok := findTurn(turns, req.TurnID)
		if !ok {
			return nil, ErrTurnNotFound
		}
		if err := validateTaskSnapshotAgainstTurn(req.ContractSnapshot, turn); err != nil {
			return nil, err
		}
		record, existed, err := f.reader.SubmitTask(ctx, tenantID, taskRecordFromSubmitRequest(tenantID, req, f.now()))
		if err == nil {
			_ = f.dispatchPendingTasks(ctx, tenantID, 1)
			if existed {
				hash, hashErr := taskRequestHash(req)
				if hashErr != nil {
					return nil, hashErr
				}
				if strings.TrimSpace(record.RequestHash) != strings.TrimSpace(hash) {
					return nil, ErrIdempotencyConflict
				}
			}
			return mapTaskReceipt(record), nil
		}
	}
	if f.legacy == nil {
		return nil, nil
	}
	return f.legacy.SubmitTask(ctx, tenantID, principal, req)
}

func (f *Facade) GetTask(ctx context.Context, tenantID string, principal Principal, taskID string) (*cubeboxdomain.TaskDetail, error) {
	if f.reader != nil {
		_ = f.dispatchPendingTasks(ctx, tenantID, taskDispatchBatchSize)
		record, err := f.reader.GetTask(ctx, tenantID, taskID)
		if err == nil {
			actorID, actorErr := f.reader.GetTaskActorID(ctx, tenantID, taskID)
			if actorErr != nil {
				return nil, actorErr
			}
			if strings.TrimSpace(actorID) != strings.TrimSpace(principal.ID) {
				return nil, ErrTaskNotFound
			}
			return mapTask(record), nil
		}
	}
	if f.legacy == nil {
		return nil, ErrTaskNotFound
	}
	return f.legacy.GetTask(ctx, tenantID, principal, taskID)
}

func (f *Facade) CancelTask(ctx context.Context, tenantID string, principal Principal, taskID string) (*cubeboxdomain.TaskCancelResponse, error) {
	if f.reader != nil {
		actorID, err := f.reader.GetTaskActorID(ctx, tenantID, taskID)
		if err != nil {
			return nil, ErrTaskNotFound
		}
		if strings.TrimSpace(actorID) != strings.TrimSpace(principal.ID) {
			return nil, ErrConversationForbidden
		}
		record, accepted, err := f.reader.CancelTask(ctx, tenantID, taskID, f.now())
		if err == nil {
			return mapTaskCancelResponse(record, accepted), nil
		}
	}
	if f.legacy == nil {
		return nil, nil
	}
	return f.legacy.CancelTask(ctx, tenantID, principal, taskID)
}

func (f *Facade) dispatchPendingTasks(ctx context.Context, tenantID string, limit int) error {
	if f == nil || f.reader == nil {
		return nil
	}
	if limit <= 0 {
		limit = 1
	}
	outboxRows, err := f.reader.ListDispatchOutbox(ctx, tenantID, taskDispatchPending, int32(limit))
	if err != nil {
		return err
	}
	now := f.now()
	for _, outbox := range outboxRows {
		if err := f.dispatchTask(ctx, tenantID, outbox, now); err != nil {
			return err
		}
	}
	return nil
}

func (f *Facade) dispatchTask(ctx context.Context, tenantID string, outbox cubeboxdomain.TaskDispatchOutboxRecord, now time.Time) error {
	taskID := strings.TrimSpace(outbox.TaskID)
	task, err := f.reader.GetTaskForDispatch(ctx, tenantID, taskID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return f.reader.UpdateTaskDispatchOutbox(ctx, tenantID, cubeboxdomain.TaskDispatchOutboxUpdate{
				TaskID:      taskID,
				Status:      taskDispatchFailed,
				Attempt:     int(outbox.Attempt),
				NextRetryAt: outbox.NextRetryAt,
				UpdatedAt:   now,
			})
		}
		return err
	}
	if taskStatusTerminal(task.Status) {
		return f.reader.UpdateTaskDispatchOutbox(ctx, tenantID, cubeboxdomain.TaskDispatchOutboxUpdate{
			TaskID:      taskID,
			Status:      taskDispatchFailed,
			Attempt:     int(outbox.Attempt),
			NextRetryAt: outbox.NextRetryAt,
			UpdatedAt:   now,
		})
	}

	attempt := int(outbox.Attempt) + 1
	if task.DispatchDeadlineAt != nil && now.After(task.DispatchDeadlineAt.UTC()) {
		if err := f.markTaskDispatchDeadlineExceeded(ctx, tenantID, task, attempt, now); err != nil {
			return err
		}
		return f.reader.UpdateTaskDispatchOutbox(ctx, tenantID, cubeboxdomain.TaskDispatchOutboxUpdate{
			TaskID:      taskID,
			Status:      taskDispatchFailed,
			Attempt:     attempt,
			NextRetryAt: outbox.NextRetryAt,
			UpdatedAt:   now,
		})
	}

	updatedTask, fromStatus, err := f.markTaskRunningIfNeeded(ctx, tenantID, task, attempt, now)
	if err != nil {
		return err
	}
	task = updatedTask

	handled, err := f.validateDispatchSnapshot(ctx, tenantID, task, fromStatus, now)
	if err != nil {
		lastError := taskErrorCode(err)
		nextRetryAt := now.Add(taskDispatchBackoff(attempt))
		if attempt >= int(task.MaxAttempts) || (task.DispatchDeadlineAt != nil && nextRetryAt.After(task.DispatchDeadlineAt.UTC())) {
			if markErr := f.markTaskDispatchFailure(ctx, tenantID, task, lastError, attempt, now); markErr != nil {
				return markErr
			}
			return f.reader.UpdateTaskDispatchOutbox(ctx, tenantID, cubeboxdomain.TaskDispatchOutboxUpdate{
				TaskID:      taskID,
				Status:      taskDispatchFailed,
				Attempt:     attempt,
				NextRetryAt: nextRetryAt,
				UpdatedAt:   now,
			})
		}
		if _, updateErr := f.reader.UpdateTaskState(ctx, tenantID, cubeboxdomain.TaskStateUpdate{
			TaskID:          taskID,
			Status:          strings.TrimSpace(task.Status),
			DispatchStatus:  taskDispatchPending,
			DispatchAttempt: attempt,
			Attempt:         int(task.Attempt),
			LastErrorCode:   lastError,
			UpdatedAt:       now,
		}); updateErr != nil {
			return updateErr
		}
		return f.reader.UpdateTaskDispatchOutbox(ctx, tenantID, cubeboxdomain.TaskDispatchOutboxUpdate{
			TaskID:      taskID,
			Status:      taskDispatchPending,
			Attempt:     attempt,
			NextRetryAt: nextRetryAt,
			UpdatedAt:   now,
		})
	}
	if handled {
		return f.reader.UpdateTaskDispatchOutbox(ctx, tenantID, cubeboxdomain.TaskDispatchOutboxUpdate{
			TaskID:      taskID,
			Status:      taskDispatchStarted,
			Attempt:     attempt,
			NextRetryAt: outbox.NextRetryAt,
			UpdatedAt:   now,
		})
	}
	if f.legacy == nil {
		return ErrTaskStateInvalid
	}
	conversationRow, err := f.reader.GetConversation(ctx, tenantID, task.ConversationID)
	if err != nil {
		return err
	}
	conversation, err := f.loadConversation(ctx, tenantID, strings.TrimSpace(conversationRow.ActorID), strings.TrimSpace(task.ConversationID), false)
	if err != nil {
		return err
	}
	result, err := f.legacy.ExecuteTaskWorkflow(ctx, tenantID, Principal{
		ID:       strings.TrimSpace(conversationRow.ActorID),
		RoleSlug: strings.TrimSpace(conversationRow.ActorRole),
	}, conversation, strings.TrimSpace(task.TurnID))
	if err != nil {
		lastError := taskErrorCode(err)
		nextRetryAt := now.Add(taskDispatchBackoff(attempt))
		if attempt >= int(task.MaxAttempts) || (task.DispatchDeadlineAt != nil && nextRetryAt.After(task.DispatchDeadlineAt.UTC())) {
			if markErr := f.markTaskDispatchFailure(ctx, tenantID, task, lastError, attempt, now); markErr != nil {
				return markErr
			}
			return f.reader.UpdateTaskDispatchOutbox(ctx, tenantID, cubeboxdomain.TaskDispatchOutboxUpdate{
				TaskID:      taskID,
				Status:      taskDispatchFailed,
				Attempt:     attempt,
				NextRetryAt: nextRetryAt,
				UpdatedAt:   now,
			})
		}
		if _, updateErr := f.reader.UpdateTaskState(ctx, tenantID, cubeboxdomain.TaskStateUpdate{
			TaskID:          taskID,
			Status:          strings.TrimSpace(task.Status),
			DispatchStatus:  taskDispatchPending,
			DispatchAttempt: attempt,
			Attempt:         int(task.Attempt),
			LastErrorCode:   lastError,
			UpdatedAt:       now,
		}); updateErr != nil {
			return updateErr
		}
		return f.reader.UpdateTaskDispatchOutbox(ctx, tenantID, cubeboxdomain.TaskDispatchOutboxUpdate{
			TaskID:      taskID,
			Status:      taskDispatchPending,
			Attempt:     attempt,
			NextRetryAt: nextRetryAt,
			UpdatedAt:   now,
		})
	}
	handled, syncErr := f.syncWorkflowConversationResult(ctx, tenantID, task, fromStatus, outbox, attempt, now, result)
	if syncErr != nil {
		return syncErr
	}
	if handled {
		return nil
	}
	if strings.TrimSpace(result.ApplyErrorCode) != "" {
		return f.finalizeTaskManualTakeover(ctx, tenantID, task, fromStatus, outbox, attempt, now, result.ApplyErrorCode)
	}

	if _, err := f.reader.UpdateTaskState(ctx, tenantID, cubeboxdomain.TaskStateUpdate{
		TaskID:          taskID,
		Status:          taskStatusSucceeded,
		DispatchStatus:  taskDispatchStarted,
		DispatchAttempt: attempt,
		Attempt:         int(task.Attempt),
		LastErrorCode:   "",
		CompletedAt:     timePtr(now),
		UpdatedAt:       now,
	}); err != nil {
		return err
	}
	if err := f.reader.InsertTaskEvent(ctx, tenantID, cubeboxdomain.TaskEventRecord{
		TaskID:     taskID,
		FromStatus: fromStatus,
		ToStatus:   taskStatusSucceeded,
		EventType:  taskStatusSucceeded,
		OccurredAt: now,
	}); err != nil {
		return err
	}
	return f.reader.UpdateTaskDispatchOutbox(ctx, tenantID, cubeboxdomain.TaskDispatchOutboxUpdate{
		TaskID:      taskID,
		Status:      taskDispatchStarted,
		Attempt:     attempt,
		NextRetryAt: outbox.NextRetryAt,
		UpdatedAt:   now,
	})
}

func (f *Facade) syncWorkflowConversationResult(ctx context.Context, tenantID string, task cubeboxdomain.TaskRecord, fromStatus string, outbox cubeboxdomain.TaskDispatchOutboxRecord, attempt int, now time.Time, result TaskWorkflowExecutionResult) (bool, error) {
	if result.Conversation == nil {
		return false, nil
	}
	if err := f.syncConversationSnapshot(ctx, tenantID, result.Conversation); err == nil {
		return false, nil
	}
	errorCode := strings.TrimSpace(result.ApplyErrorCode)
	if errorCode == "" {
		errorCode = ErrConversationSnapshotSyncFailed.Error()
	}
	if err := f.finalizeTaskManualTakeover(ctx, tenantID, task, fromStatus, outbox, attempt, now, errorCode); err != nil {
		return false, err
	}
	return true, nil
}

func (f *Facade) finalizeTaskManualTakeover(ctx context.Context, tenantID string, task cubeboxdomain.TaskRecord, fromStatus string, outbox cubeboxdomain.TaskDispatchOutboxRecord, attempt int, now time.Time, errorCode string) error {
	if err := f.markTaskManualTakeover(ctx, tenantID, task, fromStatus, errorCode, now); err != nil {
		return err
	}
	return f.reader.UpdateTaskDispatchOutbox(ctx, tenantID, cubeboxdomain.TaskDispatchOutboxUpdate{
		TaskID:      task.TaskID,
		Status:      taskDispatchStarted,
		Attempt:     attempt,
		NextRetryAt: outbox.NextRetryAt,
		UpdatedAt:   now,
	})
}

func (f *Facade) markTaskRunningIfNeeded(ctx context.Context, tenantID string, task cubeboxdomain.TaskRecord, attempt int, now time.Time) (cubeboxdomain.TaskRecord, string, error) {
	fromStatus := strings.TrimSpace(task.Status)
	if fromStatus != taskStatusQueued {
		return task, fromStatus, nil
	}
	updated, err := f.reader.UpdateTaskState(ctx, tenantID, cubeboxdomain.TaskStateUpdate{
		TaskID:          task.TaskID,
		Status:          taskStatusRunning,
		DispatchStatus:  taskDispatchStarted,
		DispatchAttempt: attempt,
		Attempt:         int(task.Attempt) + 1,
		LastErrorCode:   "",
		UpdatedAt:       now,
	})
	if err != nil {
		return cubeboxdomain.TaskRecord{}, "", err
	}
	if err := f.reader.InsertTaskEvent(ctx, tenantID, cubeboxdomain.TaskEventRecord{
		TaskID:     task.TaskID,
		FromStatus: fromStatus,
		ToStatus:   taskStatusRunning,
		EventType:  taskStatusRunning,
		OccurredAt: now,
	}); err != nil {
		return cubeboxdomain.TaskRecord{}, "", err
	}
	return updated, taskStatusRunning, nil
}

func (f *Facade) validateDispatchSnapshot(ctx context.Context, tenantID string, task cubeboxdomain.TaskRecord, fromStatus string, now time.Time) (bool, error) {
	turns, err := f.reader.ListConversationTurns(ctx, tenantID, task.ConversationID)
	if err != nil {
		return false, err
	}
	turn, ok := findTurn(turns, task.TurnID)
	if !ok {
		if markErr := f.markTaskManualTakeover(ctx, tenantID, task, fromStatus, ErrTurnNotFound.Error(), now); markErr != nil {
			return false, markErr
		}
		return true, nil
	}
	currentSnapshot, err := taskSnapshotFromTurn(turn)
	if err != nil {
		return false, err
	}
	storedSnapshot := taskSnapshotFromRecord(task)
	if !taskSnapshotCompatible(currentSnapshot, storedSnapshot) {
		if markErr := f.markTaskManualTakeover(ctx, tenantID, task, fromStatus, ErrPlanContractMismatch.Error(), now); markErr != nil {
			return false, markErr
		}
		return true, nil
	}
	if strings.TrimSpace(storedSnapshot.PlanHash) == "" {
		if markErr := f.markTaskManualTakeover(ctx, tenantID, task, fromStatus, ErrPlanDeterminismViolation.Error(), now); markErr != nil {
			return false, markErr
		}
		return true, nil
	}
	return false, nil
}

func (f *Facade) markTaskManualTakeover(ctx context.Context, tenantID string, task cubeboxdomain.TaskRecord, fromStatus string, errorCode string, now time.Time) error {
	taskID := task.TaskID
	if _, err := f.reader.UpdateTaskState(ctx, tenantID, cubeboxdomain.TaskStateUpdate{
		TaskID:          taskID,
		Status:          taskStatusManualTakeover,
		DispatchStatus:  strings.TrimSpace(task.DispatchStatus),
		DispatchAttempt: task.DispatchAttempt,
		Attempt:         int(task.Attempt),
		LastErrorCode:   strings.TrimSpace(errorCode),
		CompletedAt:     timePtr(now),
		UpdatedAt:       now,
	}); err != nil {
		return err
	}
	if err := f.reader.InsertTaskEvent(ctx, tenantID, cubeboxdomain.TaskEventRecord{
		TaskID:     taskID,
		FromStatus: fromStatus,
		ToStatus:   taskStatusManualTakeover,
		EventType:  "manual_takeover_required",
		ErrorCode:  strings.TrimSpace(errorCode),
		OccurredAt: now,
	}); err != nil {
		return err
	}
	return f.reader.InsertTaskEvent(ctx, tenantID, cubeboxdomain.TaskEventRecord{
		TaskID:     taskID,
		FromStatus: taskStatusManualTakeover,
		ToStatus:   taskStatusManualTakeover,
		EventType:  "dead_lettered",
		ErrorCode:  strings.TrimSpace(errorCode),
		OccurredAt: now,
	})
}

func (f *Facade) markTaskDispatchFailure(ctx context.Context, tenantID string, task cubeboxdomain.TaskRecord, errorCode string, attempt int, now time.Time) error {
	taskID := task.TaskID
	if _, err := f.reader.UpdateTaskState(ctx, tenantID, cubeboxdomain.TaskStateUpdate{
		TaskID:          taskID,
		Status:          taskStatusManualTakeover,
		DispatchStatus:  taskDispatchFailed,
		DispatchAttempt: attempt,
		Attempt:         int(task.Attempt),
		LastErrorCode:   strings.TrimSpace(errorCode),
		CompletedAt:     timePtr(now),
		UpdatedAt:       now,
	}); err != nil {
		return err
	}
	if err := f.reader.InsertTaskEvent(ctx, tenantID, cubeboxdomain.TaskEventRecord{
		TaskID:     taskID,
		FromStatus: taskStatusRunning,
		ToStatus:   taskStatusManualTakeover,
		EventType:  "manual_takeover_required",
		ErrorCode:  strings.TrimSpace(errorCode),
		OccurredAt: now,
	}); err != nil {
		return err
	}
	return f.reader.InsertTaskEvent(ctx, tenantID, cubeboxdomain.TaskEventRecord{
		TaskID:     taskID,
		FromStatus: taskStatusManualTakeover,
		ToStatus:   taskStatusManualTakeover,
		EventType:  "dead_lettered",
		ErrorCode:  strings.TrimSpace(errorCode),
		OccurredAt: now,
	})
}

func (f *Facade) markTaskDispatchDeadlineExceeded(ctx context.Context, tenantID string, task cubeboxdomain.TaskRecord, attempt int, now time.Time) error {
	taskID := task.TaskID
	lastError := taskTerminalErrorCode(task.LastErrorCode)
	if _, err := f.reader.UpdateTaskState(ctx, tenantID, cubeboxdomain.TaskStateUpdate{
		TaskID:          taskID,
		Status:          taskStatusManualTakeover,
		DispatchStatus:  taskDispatchFailed,
		DispatchAttempt: attempt,
		Attempt:         int(task.Attempt),
		LastErrorCode:   lastError,
		CompletedAt:     timePtr(now),
		UpdatedAt:       now,
	}); err != nil {
		return err
	}
	if err := f.reader.InsertTaskEvent(ctx, tenantID, cubeboxdomain.TaskEventRecord{
		TaskID:     taskID,
		FromStatus: taskStatusQueued,
		ToStatus:   taskStatusManualTakeover,
		EventType:  "manual_takeover_required",
		ErrorCode:  lastError,
		OccurredAt: now,
	}); err != nil {
		return err
	}
	return f.reader.InsertTaskEvent(ctx, tenantID, cubeboxdomain.TaskEventRecord{
		TaskID:     taskID,
		FromStatus: taskStatusManualTakeover,
		ToStatus:   taskStatusManualTakeover,
		EventType:  "dead_lettered",
		ErrorCode:  lastError,
		OccurredAt: now,
	})
}

func (f *Facade) RenderReply(ctx context.Context, tenantID string, principal Principal, conversationID string, turnID string, req map[string]any) (map[string]any, error) {
	if f.legacy == nil {
		return nil, nil
	}
	return f.legacy.RenderReply(ctx, tenantID, principal, conversationID, turnID, req)
}

func (f *Facade) Models(ctx context.Context) ([]cubeboxdomain.ModelEntry, error) {
	if f == nil || f.runtime == nil {
		return nil, nil
	}
	return f.runtime.Models(ctx)
}

func (f *Facade) RuntimeStatus(ctx context.Context) cubeboxdomain.RuntimeStatus {
	status := cubeboxdomain.RuntimeStatus{
		Status:    healthHealthy,
		CheckedAt: f.now().Format(time.RFC3339Nano),
		Frontend:  cubeboxdomain.RuntimeComponentStatus{Healthy: healthHealthy},
		Backend:   cubeboxdomain.RuntimeComponentStatus{Healthy: healthHealthy},
		Capabilities: cubeboxdomain.RuntimeCapabilities{
			ConversationEnabled: true,
			FilesEnabled:        true,
			AgentsUIEnabled:     false,
			AgentsWriteEnabled:  false,
			MemoryEnabled:       false,
			WebSearchEnabled:    false,
			FileSearchEnabled:   false,
			MCPEnabled:          false,
		},
		RetiredCapabilities: []string{
			"librechat_web_ui",
			"agents",
			"memory",
			"web_search",
			"file_search",
			"mcp",
		},
	}

	if f.runtime == nil {
		status.Backend = cubeboxdomain.RuntimeComponentStatus{Healthy: healthUnavailable, Reason: "assistant_service_missing"}
		status.KnowledgeRuntime = cubeboxdomain.RuntimeComponentStatus{Healthy: healthUnavailable, Reason: "knowledge_runtime_missing"}
		status.ModelGateway = cubeboxdomain.RuntimeComponentStatus{Healthy: healthUnavailable, Reason: "model_gateway_missing"}
		status.Status = healthUnavailable
	} else {
		status.Backend = f.runtime.BackendStatus(ctx)
		status.KnowledgeRuntime = f.runtime.KnowledgeRuntimeStatus(ctx)
		status.ModelGateway = f.runtime.ModelGatewayStatus(ctx)
		status.Status = aggregateRuntimeStatus(status.Backend, status.KnowledgeRuntime, status.ModelGateway)
	}

	status.FileStore = cubeboxdomain.RuntimeComponentStatus{Healthy: healthHealthy}
	if f.files == nil {
		status.FileStore = cubeboxdomain.RuntimeComponentStatus{Healthy: healthUnavailable, Reason: "file_store_missing"}
		status.Status = healthUnavailable
	} else if err := f.files.Healthy(ctx); err != nil {
		status.FileStore = cubeboxdomain.RuntimeComponentStatus{Healthy: healthUnavailable, Reason: "file_store_unavailable"}
		status.Status = healthUnavailable
	}

	if status.KnowledgeRuntime.Healthy == "" {
		status.KnowledgeRuntime = cubeboxdomain.RuntimeComponentStatus{Healthy: healthUnavailable, Reason: "knowledge_runtime_missing"}
		status.Status = healthUnavailable
	}
	if status.ModelGateway.Healthy == "" {
		status.ModelGateway = cubeboxdomain.RuntimeComponentStatus{Healthy: healthUnavailable, Reason: "model_gateway_missing"}
		status.Status = healthUnavailable
	}
	return status
}

func (f *Facade) ListFiles(ctx context.Context, tenantID string, conversationID string) ([]cubeboxdomain.FileRecord, error) {
	if f == nil || f.files == nil {
		return nil, nil
	}
	return f.files.ListFiles(ctx, tenantID, conversationID)
}

func (f *Facade) SaveFile(ctx context.Context, tenantID string, actorID string, conversationID string, filename string, mediaType string, body io.Reader) (cubeboxdomain.FileRecord, error) {
	if f == nil || f.files == nil {
		return cubeboxdomain.FileRecord{}, nil
	}
	return f.files.SaveFile(ctx, tenantID, actorID, conversationID, filename, mediaType, body)
}

func (f *Facade) DeleteFile(ctx context.Context, tenantID string, fileID string) (bool, error) {
	if f == nil || f.files == nil {
		return false, nil
	}
	return f.files.DeleteFile(ctx, tenantID, fileID)
}

func (f *Facade) now() time.Time {
	if f != nil && f.nowFn != nil {
		return f.nowFn()
	}
	return time.Now().UTC()
}

func (f *Facade) syncConversationSnapshot(ctx context.Context, tenantID string, conversation *cubeboxdomain.Conversation) error {
	if f == nil || f.reader == nil || conversation == nil {
		return nil
	}
	return f.reader.SyncConversationSnapshot(ctx, tenantID, *conversation)
}

func (f *Facade) loadConversation(ctx context.Context, tenantID string, actorID string, conversationID string, allowLegacy bool) (*cubeboxdomain.Conversation, error) {
	if f.reader != nil {
		row, err := f.reader.GetConversation(ctx, tenantID, conversationID)
		if err == nil {
			if strings.TrimSpace(row.ActorID) != strings.TrimSpace(actorID) {
				return nil, ErrConversationForbidden
			}
			turnRows, err := f.reader.ListConversationTurns(ctx, tenantID, conversationID)
			if err != nil {
				return nil, err
			}
			transitionRows, err := f.reader.ListConversationStateTransitions(ctx, tenantID, conversationID)
			if err != nil {
				return nil, err
			}
			return mapConversation(row, turnRows, transitionRows), nil
		}
	}
	if !allowLegacy || f.legacy == nil {
		return nil, ErrConversationNotFound
	}
	return f.legacy.GetConversation(ctx, tenantID, actorID, conversationID)
}

type conversationCursor struct {
	UpdatedAt      time.Time
	ConversationID string
}

func encodeConversationCursor(cursor conversationCursor, tenantID string, actorID string) string {
	if cursor.UpdatedAt.IsZero() || strings.TrimSpace(cursor.ConversationID) == "" {
		return ""
	}
	base := strings.Join([]string{
		strings.TrimSpace(tenantID),
		strings.TrimSpace(actorID),
		cursor.UpdatedAt.UTC().Format(time.RFC3339Nano),
		strings.TrimSpace(cursor.ConversationID),
	}, "|")
	signature := hashText(base + "|" + conversationCursorSalt)
	return base64.RawURLEncoding.EncodeToString([]byte(base + "|" + signature))
}

func decodeConversationCursor(encoded string, tenantID string, actorID string) (*conversationCursor, error) {
	trimmed := strings.TrimSpace(encoded)
	if trimmed == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(trimmed)
	if err != nil {
		return nil, ErrConversationCursorInvalid
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 5 {
		return nil, ErrConversationCursorInvalid
	}
	if parts[0] != strings.TrimSpace(tenantID) || parts[1] != strings.TrimSpace(actorID) {
		return nil, ErrConversationCursorInvalid
	}
	expected := hashText(strings.Join(parts[:4], "|") + "|" + conversationCursorSalt)
	if parts[4] != expected {
		return nil, ErrConversationCursorInvalid
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, parts[2])
	if err != nil {
		return nil, ErrConversationCursorInvalid
	}
	conversationID := strings.TrimSpace(parts[3])
	if conversationID == "" {
		return nil, ErrConversationCursorInvalid
	}
	return &conversationCursor{
		UpdatedAt:      updatedAt.UTC(),
		ConversationID: conversationID,
	}, nil
}

func aggregateRuntimeStatus(components ...cubeboxdomain.RuntimeComponentStatus) string {
	status := healthHealthy
	for _, component := range components {
		switch strings.TrimSpace(component.Healthy) {
		case healthUnavailable:
			return healthUnavailable
		case healthDegraded:
			status = healthDegraded
		}
	}
	return status
}

func mapConversation(row cubeboxdomain.ConversationRecord, turns []cubeboxdomain.ConversationTurnRecord, transitions []cubeboxdomain.StateTransitionRecord) *cubeboxdomain.Conversation {
	out := &cubeboxdomain.Conversation{
		ConversationID: strings.TrimSpace(row.ConversationID),
		State:          strings.TrimSpace(row.State),
		CurrentPhase:   strings.TrimSpace(row.CurrentPhase),
		ActorID:        strings.TrimSpace(row.ActorID),
		ActorRole:      strings.TrimSpace(row.ActorRole),
		CreatedAt:      row.CreatedAt.UTC(),
		UpdatedAt:      row.UpdatedAt.UTC(),
	}
	out.Turns = make([]cubeboxdomain.ConversationTurn, 0, len(turns))
	for _, turn := range turns {
		out.Turns = append(out.Turns, cubeboxdomain.ConversationTurn{
			TurnID:              strings.TrimSpace(turn.TurnID),
			UserInput:           strings.TrimSpace(turn.UserInput),
			State:               strings.TrimSpace(turn.State),
			Phase:               strings.TrimSpace(turn.Phase),
			RiskTier:            strings.TrimSpace(turn.RiskTier),
			RequestID:           strings.TrimSpace(turn.RequestID),
			TraceID:             strings.TrimSpace(turn.TraceID),
			PolicyVersion:       strings.TrimSpace(turn.PolicyVersion),
			CompositionVersion:  strings.TrimSpace(turn.CompositionVersion),
			MappingVersion:      strings.TrimSpace(turn.MappingVersion),
			Intent:              jsonObject(turn.IntentJSON),
			RouteDecision:       jsonObject(turn.RouteDecisionJSON),
			Clarification:       jsonObject(turn.ClarificationJSON),
			Candidates:          jsonObjectSlice(turn.CandidatesJSON),
			Plan:                jsonObject(turn.PlanJSON),
			DryRun:              jsonObject(turn.DryRunJSON),
			ResolvedCandidateID: strings.TrimSpace(turn.ResolvedCandidateID),
			SelectedCandidateID: strings.TrimSpace(turn.SelectedCandidateID),
			AmbiguityCount:      turn.AmbiguityCount,
			Confidence:          turn.Confidence,
			ResolutionSource:    strings.TrimSpace(turn.ResolutionSource),
			PendingDraftSummary: strings.TrimSpace(turn.PendingDraftSummary),
			MissingFields:       bytesToStringSlice(turn.MissingFieldsJSON),
			CommitResult:        jsonObject(turn.CommitResultJSON),
			CommitReply:         jsonObject(turn.CommitReplyJSON),
			ErrorCode:           strings.TrimSpace(turn.ErrorCode),
			CreatedAt:           turn.CreatedAt.UTC(),
			UpdatedAt:           turn.UpdatedAt.UTC(),
		})
	}
	out.Transitions = make([]cubeboxdomain.StateTransition, 0, len(transitions))
	for _, transition := range transitions {
		out.Transitions = append(out.Transitions, cubeboxdomain.StateTransition{
			ID:         transition.ID,
			TurnID:     strings.TrimSpace(transition.TurnID),
			TurnAction: strings.TrimSpace(transition.TurnAction),
			RequestID:  strings.TrimSpace(transition.RequestID),
			TraceID:    strings.TrimSpace(transition.TraceID),
			FromState:  strings.TrimSpace(transition.FromState),
			ToState:    strings.TrimSpace(transition.ToState),
			FromPhase:  strings.TrimSpace(transition.FromPhase),
			ToPhase:    strings.TrimSpace(transition.ToPhase),
			ReasonCode: strings.TrimSpace(transition.ReasonCode),
			ActorID:    strings.TrimSpace(transition.ActorID),
			ChangedAt:  transition.ChangedAt.UTC(),
		})
	}
	return out
}

func mapTask(record cubeboxdomain.TaskRecord) *cubeboxdomain.TaskDetail {
	detail := &cubeboxdomain.TaskDetail{
		TaskID:         record.TaskID,
		TaskType:       strings.TrimSpace(record.TaskType),
		Status:         strings.TrimSpace(record.Status),
		DispatchStatus: strings.TrimSpace(record.DispatchStatus),
		Attempt:        record.Attempt,
		MaxAttempts:    record.MaxAttempts,
		LastErrorCode:  strings.TrimSpace(record.LastErrorCode),
		WorkflowID:     strings.TrimSpace(record.WorkflowID),
		RequestID:      strings.TrimSpace(record.RequestID),
		TraceID:        strings.TrimSpace(record.TraceID),
		ConversationID: strings.TrimSpace(record.ConversationID),
		TurnID:         strings.TrimSpace(record.TurnID),
		SubmittedAt:    record.SubmittedAt.UTC(),
		UpdatedAt:      record.UpdatedAt.UTC(),
		ContractSnapshot: cubeboxdomain.TaskContractSnapshot{
			IntentSchemaVersion:      strings.TrimSpace(record.IntentSchemaVersion),
			CompilerContractVersion:  strings.TrimSpace(record.CompilerContractVersion),
			CapabilityMapVersion:     strings.TrimSpace(record.CapabilityMapVersion),
			SkillManifestDigest:      strings.TrimSpace(record.SkillManifestDigest),
			ContextHash:              strings.TrimSpace(record.ContextHash),
			IntentHash:               strings.TrimSpace(record.IntentHash),
			PlanHash:                 strings.TrimSpace(record.PlanHash),
			KnowledgeSnapshotDigest:  strings.TrimSpace(record.KnowledgeSnapshotDigest),
			RouteCatalogVersion:      strings.TrimSpace(record.RouteCatalogVersion),
			ResolverContractVersion:  strings.TrimSpace(record.ResolverContractVersion),
			ContextTemplateVersion:   strings.TrimSpace(record.ContextTemplateVersion),
			ReplyGuidanceVersion:     strings.TrimSpace(record.ReplyGuidanceVersion),
			PolicyContextDigest:      strings.TrimSpace(record.PolicyContextDigest),
			EffectivePolicyVersion:   strings.TrimSpace(record.EffectivePolicyVersion),
			ResolvedSetID:            strings.TrimSpace(record.ResolvedSetID),
			SetIDSource:              strings.TrimSpace(record.SetIDSource),
			PrecheckProjectionDigest: strings.TrimSpace(record.PrecheckProjectionDigest),
			MutationPolicyVersion:    strings.TrimSpace(record.MutationPolicyVersion),
		},
	}
	if record.CancelRequestedAt != nil {
		cancelRequestedAt := record.CancelRequestedAt.UTC()
		detail.CancelRequestedAt = &cancelRequestedAt
	}
	if record.CompletedAt != nil {
		completedAt := record.CompletedAt.UTC()
		detail.CompletedAt = &completedAt
	}
	return detail
}

func mapTaskReceipt(record cubeboxdomain.TaskRecord) *cubeboxdomain.TaskReceipt {
	return &cubeboxdomain.TaskReceipt{
		TaskID:      record.TaskID,
		TaskType:    strings.TrimSpace(record.TaskType),
		Status:      strings.TrimSpace(record.Status),
		WorkflowID:  strings.TrimSpace(record.WorkflowID),
		SubmittedAt: record.SubmittedAt.UTC(),
		PollURI:     "/internal/cubebox/tasks/" + record.TaskID,
	}
}

func mapTaskCancelResponse(record cubeboxdomain.TaskRecord, accepted bool) *cubeboxdomain.TaskCancelResponse {
	detail := mapTask(record)
	if detail == nil {
		return nil
	}
	return &cubeboxdomain.TaskCancelResponse{
		TaskDetail:     *detail,
		CancelAccepted: accepted,
	}
}

func taskRecordFromSubmitRequest(tenantID string, req cubeboxdomain.TaskSubmitRequest, now time.Time) cubeboxdomain.TaskRecord {
	requestHash, _ := taskRequestHash(req)
	taskID := uuid.NewString()
	return cubeboxdomain.TaskRecord{
		TaskID:                   taskID,
		ConversationID:           req.ConversationID,
		TurnID:                   req.TurnID,
		TaskType:                 taskTypeAsyncPlan,
		RequestID:                req.RequestID,
		RequestHash:              requestHash,
		WorkflowID:               taskWorkflowID(tenantID, req.ConversationID, req.TurnID, req.RequestID),
		Status:                   taskStatusQueued,
		DispatchStatus:           taskDispatchPending,
		DispatchAttempt:          0,
		DispatchDeadlineAt:       timePtr(now.Add(taskDispatchDeadlineDelta)),
		Attempt:                  0,
		MaxAttempts:              taskDefaultMaxAttempts,
		LastErrorCode:            "",
		TraceID:                  strings.TrimSpace(req.TraceID),
		IntentSchemaVersion:      req.ContractSnapshot.IntentSchemaVersion,
		CompilerContractVersion:  req.ContractSnapshot.CompilerContractVersion,
		CapabilityMapVersion:     req.ContractSnapshot.CapabilityMapVersion,
		SkillManifestDigest:      req.ContractSnapshot.SkillManifestDigest,
		ContextHash:              req.ContractSnapshot.ContextHash,
		IntentHash:               req.ContractSnapshot.IntentHash,
		PlanHash:                 req.ContractSnapshot.PlanHash,
		KnowledgeSnapshotDigest:  req.ContractSnapshot.KnowledgeSnapshotDigest,
		RouteCatalogVersion:      req.ContractSnapshot.RouteCatalogVersion,
		ResolverContractVersion:  req.ContractSnapshot.ResolverContractVersion,
		ContextTemplateVersion:   req.ContractSnapshot.ContextTemplateVersion,
		ReplyGuidanceVersion:     req.ContractSnapshot.ReplyGuidanceVersion,
		PolicyContextDigest:      req.ContractSnapshot.PolicyContextDigest,
		EffectivePolicyVersion:   req.ContractSnapshot.EffectivePolicyVersion,
		ResolvedSetID:            req.ContractSnapshot.ResolvedSetID,
		SetIDSource:              req.ContractSnapshot.SetIDSource,
		PrecheckProjectionDigest: req.ContractSnapshot.PrecheckProjectionDigest,
		MutationPolicyVersion:    req.ContractSnapshot.MutationPolicyVersion,
		SubmittedAt:              now.UTC(),
		CreatedAt:                now.UTC(),
		UpdatedAt:                now.UTC(),
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func bytesToStringSlice(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" {
		return nil
	}
	value = strings.TrimPrefix(value, "[")
	value = strings.TrimSuffix(value, "]")
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(strings.Trim(part, `"`))
		if part != "" {
			items = append(items, part)
		}
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

func jsonObject(raw []byte) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil || len(out) == 0 {
		return nil
	}
	return out
}

func jsonObjectSlice(raw []byte) []map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var out []map[string]any
	if err := json.Unmarshal(raw, &out); err != nil || len(out) == 0 {
		return nil
	}
	return out
}

func min(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

const (
	taskTypeAsyncPlan         = "assistant_async_plan"
	taskStatusQueued          = "queued"
	taskStatusRunning         = "running"
	taskStatusSucceeded       = "succeeded"
	taskStatusFailed          = "failed"
	taskStatusManualTakeover  = "manual_takeover_required"
	taskStatusCanceled        = "canceled"
	taskDispatchPending       = "pending"
	taskDispatchStarted       = "started"
	taskDispatchFailed        = "failed"
	taskDefaultMaxAttempts    = 3
	taskDispatchDeadlineDelta = 10 * time.Minute
	intentCreateOrgUnit       = "create_orgunit"
	intentCorrectOrgUnit      = "correct_orgunit"
	intentDisableOrgUnit      = "disable_orgunit"
	intentMoveOrgUnit         = "move_orgunit"
	intentRenameOrgUnit       = "rename_orgunit"
	intentPlanOnly            = "plan_only"
)

type turnIntentPayload struct {
	Action              string `json:"action"`
	RouteKind           string `json:"route_kind,omitempty"`
	IntentSchemaVersion string `json:"intent_schema_version,omitempty"`
	ContextHash         string `json:"context_hash,omitempty"`
	IntentHash          string `json:"intent_hash,omitempty"`
}

type turnPlanPayload struct {
	CapabilityMapVersion    string `json:"capability_map_version,omitempty"`
	CompilerContractVersion string `json:"compiler_contract_version,omitempty"`
	SkillManifestDigest     string `json:"skill_manifest_digest,omitempty"`
	KnowledgeSnapshotDigest string `json:"knowledge_snapshot_digest,omitempty"`
	RouteCatalogVersion     string `json:"route_catalog_version,omitempty"`
	ResolverContractVersion string `json:"resolver_contract_version,omitempty"`
	ContextTemplateVersion  string `json:"context_template_version,omitempty"`
	ReplyGuidanceVersion    string `json:"reply_guidance_version,omitempty"`
	ConfirmTTLSeconds       int    `json:"confirm_ttl_seconds,omitempty"`
	ExpiresAt               string `json:"expires_at,omitempty"`
}

type turnDryRunPayload struct {
	PlanHash                 string                        `json:"plan_hash,omitempty"`
	CreateOrgUnitProjection  *createOrgUnitProjection      `json:"create_orgunit_projection,omitempty"`
	OrgUnitVersionProjection *orgUnitVersionProjectionWrap `json:"orgunit_version_projection,omitempty"`
}

type routeDecisionPayload struct {
	KnowledgeSnapshotDigest string `json:"knowledge_snapshot_digest,omitempty"`
	RouteCatalogVersion     string `json:"route_catalog_version,omitempty"`
	ResolverContractVersion string `json:"resolver_contract_version,omitempty"`
}

type createOrgUnitProjection struct {
	PolicyContext policyContextPayload `json:"policy_context"`
	Projection    projectionPayload    `json:"projection"`
}

type orgUnitVersionProjectionWrap struct {
	PolicyContext policyContextPayload `json:"policy_context"`
	Projection    projectionPayload    `json:"projection"`
}

type policyContextPayload struct {
	PolicyContextDigest string `json:"policy_context_digest"`
}

type projectionPayload struct {
	EffectivePolicyVersion string `json:"effective_policy_version"`
	ResolvedSetID          string `json:"resolved_setid"`
	SetIDSource            string `json:"setid_source"`
	ProjectionDigest       string `json:"projection_digest"`
	MutationPolicyVersion  string `json:"mutation_policy_version"`
}

func normalizeTaskSubmitRequest(req cubeboxdomain.TaskSubmitRequest) cubeboxdomain.TaskSubmitRequest {
	req.ConversationID = strings.TrimSpace(req.ConversationID)
	req.TurnID = strings.TrimSpace(req.TurnID)
	req.TaskType = strings.TrimSpace(req.TaskType)
	req.RequestID = strings.TrimSpace(req.RequestID)
	req.TraceID = strings.TrimSpace(req.TraceID)
	req.ContractSnapshot = normalizeTaskSnapshot(req.ContractSnapshot)
	return req
}

func normalizeTaskSnapshot(snapshot cubeboxdomain.TaskContractSnapshot) cubeboxdomain.TaskContractSnapshot {
	snapshot.IntentSchemaVersion = strings.TrimSpace(snapshot.IntentSchemaVersion)
	snapshot.CompilerContractVersion = strings.TrimSpace(snapshot.CompilerContractVersion)
	snapshot.CapabilityMapVersion = strings.TrimSpace(snapshot.CapabilityMapVersion)
	snapshot.SkillManifestDigest = strings.TrimSpace(snapshot.SkillManifestDigest)
	snapshot.ContextHash = strings.TrimSpace(snapshot.ContextHash)
	snapshot.IntentHash = strings.TrimSpace(snapshot.IntentHash)
	snapshot.PlanHash = strings.TrimSpace(snapshot.PlanHash)
	snapshot.KnowledgeSnapshotDigest = strings.TrimSpace(snapshot.KnowledgeSnapshotDigest)
	snapshot.RouteCatalogVersion = strings.TrimSpace(snapshot.RouteCatalogVersion)
	snapshot.ResolverContractVersion = strings.TrimSpace(snapshot.ResolverContractVersion)
	snapshot.ContextTemplateVersion = strings.TrimSpace(snapshot.ContextTemplateVersion)
	snapshot.ReplyGuidanceVersion = strings.TrimSpace(snapshot.ReplyGuidanceVersion)
	snapshot.PolicyContextDigest = strings.TrimSpace(snapshot.PolicyContextDigest)
	snapshot.EffectivePolicyVersion = strings.TrimSpace(snapshot.EffectivePolicyVersion)
	snapshot.ResolvedSetID = strings.TrimSpace(snapshot.ResolvedSetID)
	snapshot.SetIDSource = strings.TrimSpace(snapshot.SetIDSource)
	snapshot.PrecheckProjectionDigest = strings.TrimSpace(snapshot.PrecheckProjectionDigest)
	snapshot.MutationPolicyVersion = strings.TrimSpace(snapshot.MutationPolicyVersion)
	return snapshot
}

func taskRequestHash(req cubeboxdomain.TaskSubmitRequest) (string, error) {
	payload := struct {
		ConversationID   string                             `json:"conversation_id"`
		TurnID           string                             `json:"turn_id"`
		TaskType         string                             `json:"task_type"`
		RequestID        string                             `json:"request_id"`
		ContractSnapshot cubeboxdomain.TaskContractSnapshot `json:"contract_snapshot"`
	}{
		ConversationID:   strings.TrimSpace(req.ConversationID),
		TurnID:           strings.TrimSpace(req.TurnID),
		TaskType:         strings.TrimSpace(req.TaskType),
		RequestID:        strings.TrimSpace(req.RequestID),
		ContractSnapshot: normalizeTaskSnapshot(req.ContractSnapshot),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return hashText(strings.TrimSpace(req.TaskType) + "\n" + string(raw)), nil
}

func validateTaskSubmitRequest(req cubeboxdomain.TaskSubmitRequest) error {
	req = normalizeTaskSubmitRequest(req)
	switch {
	case req.ConversationID == "":
		return errors.New("conversation_id required")
	case req.TurnID == "":
		return errors.New("turn_id required")
	case req.TaskType == "":
		return errors.New("task_type required")
	case req.TaskType != taskTypeAsyncPlan:
		return errors.New("task_type invalid")
	case req.RequestID == "":
		return errors.New("request_id required")
	}
	snapshot := req.ContractSnapshot
	if snapshot.IntentSchemaVersion == "" ||
		snapshot.CompilerContractVersion == "" ||
		snapshot.CapabilityMapVersion == "" ||
		snapshot.SkillManifestDigest == "" ||
		snapshot.ContextHash == "" ||
		snapshot.IntentHash == "" ||
		snapshot.PlanHash == "" {
		return errors.New("contract_snapshot incomplete")
	}
	if taskPolicyContractIncomplete(snapshot) {
		return errors.New("contract_snapshot incomplete")
	}
	return nil
}

func taskPolicyContractIncomplete(snapshot cubeboxdomain.TaskContractSnapshot) bool {
	hasPolicySnapshot := snapshot.PolicyContextDigest != "" ||
		snapshot.EffectivePolicyVersion != "" ||
		snapshot.PrecheckProjectionDigest != "" ||
		snapshot.MutationPolicyVersion != "" ||
		snapshot.ResolvedSetID != "" ||
		snapshot.SetIDSource != ""
	if !hasPolicySnapshot {
		return false
	}
	return snapshot.PolicyContextDigest == "" ||
		snapshot.EffectivePolicyVersion == "" ||
		snapshot.PrecheckProjectionDigest == "" ||
		snapshot.MutationPolicyVersion == ""
}

func taskStatusTerminal(status string) bool {
	switch strings.TrimSpace(status) {
	case taskStatusSucceeded, taskStatusFailed, taskStatusCanceled:
		return true
	default:
		return false
	}
}

func taskStatusCancellable(status string) bool {
	switch strings.TrimSpace(status) {
	case taskStatusQueued, taskStatusRunning, taskStatusManualTakeover:
		return true
	default:
		return false
	}
}

func taskDispatchBackoff(attempt int) time.Duration {
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

func taskErrorCode(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(err.Error())
}

func taskTerminalErrorCode(code string) string {
	code = strings.TrimSpace(code)
	if code != "" {
		return code
	}
	return ErrTaskStateInvalid.Error()
}

func taskSnapshotFromTurn(turn cubeboxdomain.ConversationTurnRecord) (cubeboxdomain.TaskContractSnapshot, error) {
	intent := turnIntentPayload{}
	if len(turn.IntentJSON) > 0 {
		if err := json.Unmarshal(turn.IntentJSON, &intent); err != nil {
			return cubeboxdomain.TaskContractSnapshot{}, err
		}
	}
	plan := turnPlanPayload{}
	if len(turn.PlanJSON) > 0 {
		if err := json.Unmarshal(turn.PlanJSON, &plan); err != nil {
			return cubeboxdomain.TaskContractSnapshot{}, err
		}
	}
	dryRun := turnDryRunPayload{}
	if len(turn.DryRunJSON) > 0 {
		if err := json.Unmarshal(turn.DryRunJSON, &dryRun); err != nil {
			return cubeboxdomain.TaskContractSnapshot{}, err
		}
	}
	decision := routeDecisionPayload{}
	if len(turn.RouteDecisionJSON) > 0 {
		if err := json.Unmarshal(turn.RouteDecisionJSON, &decision); err != nil {
			return cubeboxdomain.TaskContractSnapshot{}, err
		}
	}
	snapshot := normalizeTaskSnapshot(cubeboxdomain.TaskContractSnapshot{
		IntentSchemaVersion:     intent.IntentSchemaVersion,
		CompilerContractVersion: plan.CompilerContractVersion,
		CapabilityMapVersion:    plan.CapabilityMapVersion,
		SkillManifestDigest:     plan.SkillManifestDigest,
		ContextHash:             intent.ContextHash,
		IntentHash:              intent.IntentHash,
		PlanHash:                dryRun.PlanHash,
		KnowledgeSnapshotDigest: plan.KnowledgeSnapshotDigest,
		RouteCatalogVersion:     plan.RouteCatalogVersion,
		ResolverContractVersion: plan.ResolverContractVersion,
		ContextTemplateVersion:  plan.ContextTemplateVersion,
		ReplyGuidanceVersion:    plan.ReplyGuidanceVersion,
	})
	if values, ok := policyContractValuesFromDryRun(dryRun); ok {
		snapshot.PolicyContextDigest = values.PolicyContextDigest
		snapshot.EffectivePolicyVersion = values.EffectivePolicyVersion
		snapshot.ResolvedSetID = values.ResolvedSetID
		snapshot.SetIDSource = values.SetIDSource
		snapshot.PrecheckProjectionDigest = values.PrecheckProjectionDigest
		snapshot.MutationPolicyVersion = values.MutationPolicyVersion
	}
	if !turnRouteAuditVersionsConsistent(turn, plan, decision) {
		return cubeboxdomain.TaskContractSnapshot{}, ErrPlanContractMismatch
	}
	if actionRequiresPolicyProjection(intent.Action) && taskPolicyContractIncomplete(snapshot) {
		return cubeboxdomain.TaskContractSnapshot{}, ErrPlanContractMismatch
	}
	return snapshot, nil
}

func validateTaskSnapshotAgainstTurn(snapshot cubeboxdomain.TaskContractSnapshot, turn cubeboxdomain.ConversationTurnRecord) error {
	current, err := taskSnapshotFromTurn(turn)
	if err != nil {
		return err
	}
	snapshot = normalizeTaskSnapshot(snapshot)
	if current.PolicyContextDigest != "" &&
		(snapshot.PolicyContextDigest == "" ||
			snapshot.EffectivePolicyVersion == "" ||
			snapshot.PrecheckProjectionDigest == "" ||
			snapshot.MutationPolicyVersion == "") {
		return ErrPlanContractMismatch
	}
	if !taskSnapshotCompatible(current, snapshot) {
		return ErrPlanContractMismatch
	}
	return nil
}

func taskSnapshotCompatible(current cubeboxdomain.TaskContractSnapshot, stored cubeboxdomain.TaskContractSnapshot) bool {
	current = normalizeTaskSnapshot(current)
	stored = normalizeTaskSnapshot(stored)
	if current.IntentSchemaVersion != stored.IntentSchemaVersion ||
		current.CompilerContractVersion != stored.CompilerContractVersion ||
		current.CapabilityMapVersion != stored.CapabilityMapVersion ||
		current.SkillManifestDigest != stored.SkillManifestDigest ||
		current.ContextHash != stored.ContextHash ||
		current.IntentHash != stored.IntentHash ||
		current.PlanHash != stored.PlanHash {
		return false
	}
	if stored.KnowledgeSnapshotDigest != "" && current.KnowledgeSnapshotDigest != stored.KnowledgeSnapshotDigest {
		return false
	}
	if stored.RouteCatalogVersion != "" && current.RouteCatalogVersion != stored.RouteCatalogVersion {
		return false
	}
	if stored.ResolverContractVersion != "" && current.ResolverContractVersion != stored.ResolverContractVersion {
		return false
	}
	if stored.ContextTemplateVersion != "" && current.ContextTemplateVersion != stored.ContextTemplateVersion {
		return false
	}
	if stored.ReplyGuidanceVersion != "" && current.ReplyGuidanceVersion != stored.ReplyGuidanceVersion {
		return false
	}
	if stored.PolicyContextDigest != "" && current.PolicyContextDigest != stored.PolicyContextDigest {
		return false
	}
	if stored.EffectivePolicyVersion != "" && current.EffectivePolicyVersion != stored.EffectivePolicyVersion {
		return false
	}
	if stored.ResolvedSetID != "" && current.ResolvedSetID != stored.ResolvedSetID {
		return false
	}
	if stored.SetIDSource != "" && current.SetIDSource != stored.SetIDSource {
		return false
	}
	if stored.PrecheckProjectionDigest != "" && current.PrecheckProjectionDigest != stored.PrecheckProjectionDigest {
		return false
	}
	if stored.MutationPolicyVersion != "" && current.MutationPolicyVersion != stored.MutationPolicyVersion {
		return false
	}
	return true
}

func taskSnapshotFromRecord(record cubeboxdomain.TaskRecord) cubeboxdomain.TaskContractSnapshot {
	return normalizeTaskSnapshot(cubeboxdomain.TaskContractSnapshot{
		IntentSchemaVersion:      strings.TrimSpace(record.IntentSchemaVersion),
		CompilerContractVersion:  strings.TrimSpace(record.CompilerContractVersion),
		CapabilityMapVersion:     strings.TrimSpace(record.CapabilityMapVersion),
		SkillManifestDigest:      strings.TrimSpace(record.SkillManifestDigest),
		ContextHash:              strings.TrimSpace(record.ContextHash),
		IntentHash:               strings.TrimSpace(record.IntentHash),
		PlanHash:                 strings.TrimSpace(record.PlanHash),
		KnowledgeSnapshotDigest:  strings.TrimSpace(record.KnowledgeSnapshotDigest),
		RouteCatalogVersion:      strings.TrimSpace(record.RouteCatalogVersion),
		ResolverContractVersion:  strings.TrimSpace(record.ResolverContractVersion),
		ContextTemplateVersion:   strings.TrimSpace(record.ContextTemplateVersion),
		ReplyGuidanceVersion:     strings.TrimSpace(record.ReplyGuidanceVersion),
		PolicyContextDigest:      strings.TrimSpace(record.PolicyContextDigest),
		EffectivePolicyVersion:   strings.TrimSpace(record.EffectivePolicyVersion),
		ResolvedSetID:            strings.TrimSpace(record.ResolvedSetID),
		SetIDSource:              strings.TrimSpace(record.SetIDSource),
		PrecheckProjectionDigest: strings.TrimSpace(record.PrecheckProjectionDigest),
		MutationPolicyVersion:    strings.TrimSpace(record.MutationPolicyVersion),
	})
}

func buildCommitTaskSubmitRequest(conversationID string, turn cubeboxdomain.ConversationTurnRecord) (cubeboxdomain.TaskSubmitRequest, error) {
	snapshot, err := taskSnapshotFromTurn(turn)
	if err != nil {
		return cubeboxdomain.TaskSubmitRequest{}, err
	}
	if strings.TrimSpace(snapshot.PlanHash) == "" {
		return cubeboxdomain.TaskSubmitRequest{}, ErrPlanDeterminismViolation
	}
	requestID := strings.TrimSpace(turn.RequestID)
	if requestID == "" {
		return cubeboxdomain.TaskSubmitRequest{}, errors.New("request_id required")
	}
	return cubeboxdomain.TaskSubmitRequest{
		ConversationID:   strings.TrimSpace(conversationID),
		TurnID:           strings.TrimSpace(turn.TurnID),
		TaskType:         taskTypeAsyncPlan,
		RequestID:        requestID,
		TraceID:          strings.TrimSpace(turn.TraceID),
		ContractSnapshot: normalizeTaskSnapshot(snapshot),
	}, nil
}

func turnRouteAuditVersionsConsistent(turn cubeboxdomain.ConversationTurnRecord, plan turnPlanPayload, decision routeDecisionPayload) bool {
	if len(turn.RouteDecisionJSON) == 0 {
		return true
	}
	if strings.TrimSpace(plan.KnowledgeSnapshotDigest) == "" ||
		strings.TrimSpace(plan.RouteCatalogVersion) == "" ||
		strings.TrimSpace(plan.ResolverContractVersion) == "" ||
		strings.TrimSpace(plan.ContextTemplateVersion) == "" ||
		strings.TrimSpace(plan.ReplyGuidanceVersion) == "" {
		return false
	}
	return strings.TrimSpace(plan.KnowledgeSnapshotDigest) == strings.TrimSpace(decision.KnowledgeSnapshotDigest) &&
		strings.TrimSpace(plan.RouteCatalogVersion) == strings.TrimSpace(decision.RouteCatalogVersion) &&
		strings.TrimSpace(plan.ResolverContractVersion) == strings.TrimSpace(decision.ResolverContractVersion)
}

func turnConfirmationExpired(turn cubeboxdomain.ConversationTurnRecord, now time.Time) bool {
	if strings.TrimSpace(turn.State) != "validated" {
		return false
	}
	deadline, ok := turnConfirmDeadline(turn)
	if !ok {
		return false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return !deadline.After(now.UTC())
}

func turnConfirmDeadline(turn cubeboxdomain.ConversationTurnRecord) (time.Time, bool) {
	plan := turnPlanPayload{}
	if len(turn.PlanJSON) > 0 {
		if err := json.Unmarshal(turn.PlanJSON, &plan); err != nil {
			return time.Time{}, false
		}
	}
	if expiresAt := strings.TrimSpace(plan.ExpiresAt); expiresAt != "" {
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
	ttlSeconds := plan.ConfirmTTLSeconds
	if ttlSeconds <= 0 {
		ttlSeconds = 15 * 60
	}
	return base.Add(time.Duration(ttlSeconds) * time.Second), true
}

func policyContractValuesFromDryRun(dryRun turnDryRunPayload) (cubeboxdomain.TaskContractSnapshot, bool) {
	if dryRun.CreateOrgUnitProjection != nil {
		return cubeboxdomain.TaskContractSnapshot{
			PolicyContextDigest:      strings.TrimSpace(dryRun.CreateOrgUnitProjection.PolicyContext.PolicyContextDigest),
			EffectivePolicyVersion:   strings.TrimSpace(dryRun.CreateOrgUnitProjection.Projection.EffectivePolicyVersion),
			ResolvedSetID:            strings.TrimSpace(dryRun.CreateOrgUnitProjection.Projection.ResolvedSetID),
			SetIDSource:              strings.TrimSpace(dryRun.CreateOrgUnitProjection.Projection.SetIDSource),
			PrecheckProjectionDigest: strings.TrimSpace(dryRun.CreateOrgUnitProjection.Projection.ProjectionDigest),
			MutationPolicyVersion:    strings.TrimSpace(dryRun.CreateOrgUnitProjection.Projection.MutationPolicyVersion),
		}, true
	}
	if dryRun.OrgUnitVersionProjection != nil {
		return cubeboxdomain.TaskContractSnapshot{
			PolicyContextDigest:      strings.TrimSpace(dryRun.OrgUnitVersionProjection.PolicyContext.PolicyContextDigest),
			EffectivePolicyVersion:   strings.TrimSpace(dryRun.OrgUnitVersionProjection.Projection.EffectivePolicyVersion),
			ResolvedSetID:            strings.TrimSpace(dryRun.OrgUnitVersionProjection.Projection.ResolvedSetID),
			SetIDSource:              strings.TrimSpace(dryRun.OrgUnitVersionProjection.Projection.SetIDSource),
			PrecheckProjectionDigest: strings.TrimSpace(dryRun.OrgUnitVersionProjection.Projection.ProjectionDigest),
			MutationPolicyVersion:    strings.TrimSpace(dryRun.OrgUnitVersionProjection.Projection.MutationPolicyVersion),
		}, true
	}
	return cubeboxdomain.TaskContractSnapshot{}, false
}

func actionRequiresPolicyProjection(action string) bool {
	switch strings.TrimSpace(action) {
	case intentCreateOrgUnit, intentCorrectOrgUnit, intentDisableOrgUnit, intentMoveOrgUnit, intentRenameOrgUnit:
		return true
	default:
		return false
	}
}

func taskWorkflowID(tenantID string, conversationID string, turnID string, requestID string) string {
	return strings.Join([]string{
		"assistant_async_orchestration_v1",
		strings.TrimSpace(tenantID),
		strings.TrimSpace(conversationID),
		strings.TrimSpace(turnID),
		strings.TrimSpace(requestID),
	}, ":")
}

func findTurn(turns []cubeboxdomain.ConversationTurnRecord, turnID string) (cubeboxdomain.ConversationTurnRecord, bool) {
	for _, turn := range turns {
		if strings.TrimSpace(turn.TurnID) == strings.TrimSpace(turnID) {
			return turn, true
		}
	}
	return cubeboxdomain.ConversationTurnRecord{}, false
}

func nilIfBlank(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func timePtr(ts time.Time) *time.Time {
	value := ts.UTC()
	return &value
}
