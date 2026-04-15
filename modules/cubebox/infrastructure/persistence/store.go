package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	cubeboxdomain "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/domain"
	cubeboxsqlc "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/infrastructure/sqlc/gen"
)

type pgBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type PGStore struct {
	pool pgBeginner
}

func NewPGStore(pool pgBeginner) *PGStore {
	return &PGStore{pool: pool}
}

func (s *PGStore) ListConversations(ctx context.Context, tenantID string, actorID string, limit int32, cursorUpdatedAt time.Time, cursorConversationID string) ([]cubeboxsqlc.IamCubeboxConversation, error) {
	if limit <= 0 {
		limit = 20
	}
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return nil, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	items, err := queries.ListConversationsByActor(ctx, cubeboxsqlc.ListConversationsByActorParams{
		TenantUuid:     tenantUUID,
		ActorID:        actorID,
		Column3:        pgtype.Timestamptz{Time: cursorUpdatedAt.UTC(), Valid: !cursorUpdatedAt.IsZero()},
		ConversationID: cursorConversationID,
		Limit:          limit,
	})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *PGStore) GetConversation(ctx context.Context, tenantID string, conversationID string) (cubeboxsqlc.IamCubeboxConversation, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return cubeboxsqlc.IamCubeboxConversation{}, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return cubeboxsqlc.IamCubeboxConversation{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	item, err := queries.GetConversationByID(ctx, cubeboxsqlc.GetConversationByIDParams{
		TenantUuid:     tenantUUID,
		ConversationID: conversationID,
	})
	if err != nil {
		return cubeboxsqlc.IamCubeboxConversation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return cubeboxsqlc.IamCubeboxConversation{}, err
	}
	return item, nil
}

func (s *PGStore) ListConversationTurns(ctx context.Context, tenantID string, conversationID string) ([]cubeboxsqlc.IamCubeboxTurn, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return nil, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	items, err := queries.ListConversationTurns(ctx, cubeboxsqlc.ListConversationTurnsParams{
		TenantUuid:     tenantUUID,
		ConversationID: conversationID,
	})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *PGStore) ListConversationStateTransitions(ctx context.Context, tenantID string, conversationID string) ([]cubeboxsqlc.IamCubeboxStateTransition, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return nil, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	items, err := queries.ListConversationStateTransitions(ctx, cubeboxsqlc.ListConversationStateTransitionsParams{
		TenantUuid:     tenantUUID,
		ConversationID: conversationID,
	})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *PGStore) SyncConversationSnapshot(ctx context.Context, tenantID string, conversation cubeboxdomain.Conversation) error {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return err
	}
	tx, _, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `
INSERT INTO iam.cubebox_conversations (
  tenant_uuid,
  conversation_id,
  actor_id,
  actor_role,
  state,
  current_phase,
  created_at,
  updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (tenant_uuid, conversation_id)
DO UPDATE SET
  actor_id = EXCLUDED.actor_id,
  actor_role = EXCLUDED.actor_role,
  state = EXCLUDED.state,
  current_phase = EXCLUDED.current_phase,
  updated_at = EXCLUDED.updated_at
`,
		tenantUUID,
		strings.TrimSpace(conversation.ConversationID),
		strings.TrimSpace(conversation.ActorID),
		strings.TrimSpace(conversation.ActorRole),
		strings.TrimSpace(conversation.State),
		strings.TrimSpace(conversation.CurrentPhase),
		pgtype.Timestamptz{Time: conversation.CreatedAt.UTC(), Valid: !conversation.CreatedAt.IsZero()},
		pgtype.Timestamptz{Time: conversation.UpdatedAt.UTC(), Valid: !conversation.UpdatedAt.IsZero()},
	); err != nil {
		return err
	}

	for _, turn := range conversation.Turns {
		if err := syncTurnSnapshot(ctx, tx, tenantUUID, conversation.ConversationID, turn); err != nil {
			return err
		}
	}
	for _, transition := range conversation.Transitions {
		if err := syncTransitionSnapshot(ctx, tx, tenantUUID, conversation.ConversationID, transition); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *PGStore) CountBlockingTasks(ctx context.Context, tenantID string, conversationID string) (int64, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return 0, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	count, err := queries.CountBlockingTasksForConversation(ctx, cubeboxsqlc.CountBlockingTasksForConversationParams{
		TenantUuid:     tenantUUID,
		ConversationID: conversationID,
	})
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *PGStore) DeleteConversation(ctx context.Context, tenantID string, conversationID string) (int64, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return 0, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	rows, err := queries.DeleteConversationByID(ctx, cubeboxsqlc.DeleteConversationByIDParams{
		TenantUuid:     tenantUUID,
		ConversationID: conversationID,
	})
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return rows, nil
}

func (s *PGStore) GetTask(ctx context.Context, tenantID string, taskID string) (cubeboxsqlc.IamCubeboxTask, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, err
	}
	taskUUID, err := parseUUID(taskID)
	if err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	item, err := queries.GetTaskByID(ctx, cubeboxsqlc.GetTaskByIDParams{
		TenantUuid: tenantUUID,
		TaskID:     taskUUID,
	})
	if err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, err
	}
	return item, nil
}

func (s *PGStore) GetTaskForDispatch(ctx context.Context, tenantID string, taskID string) (cubeboxsqlc.IamCubeboxTask, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, err
	}
	taskUUID, err := parseUUID(taskID)
	if err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	item, err := queries.GetTaskByIDForUpdate(ctx, cubeboxsqlc.GetTaskByIDForUpdateParams{
		TenantUuid: tenantUUID,
		TaskID:     taskUUID,
	})
	if err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, err
	}
	return item, nil
}

func (s *PGStore) GetTaskActorID(ctx context.Context, tenantID string, taskID string) (string, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return "", err
	}
	taskUUID, err := parseUUID(taskID)
	if err != nil {
		return "", err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	actorID, err := queries.GetConversationActorByTaskID(ctx, cubeboxsqlc.GetConversationActorByTaskIDParams{
		TenantUuid: tenantUUID,
		TaskID:     taskUUID,
	})
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return actorID, nil
}

func (s *PGStore) SubmitTask(ctx context.Context, tenantID string, record cubeboxsqlc.IamCubeboxTask) (cubeboxsqlc.IamCubeboxTask, bool, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, false, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	existing, err := queries.GetTaskBySubmitKey(ctx, cubeboxsqlc.GetTaskBySubmitKeyParams{
		TenantUuid:     tenantUUID,
		ConversationID: record.ConversationID,
		TurnID:         record.TurnID,
		RequestID:      record.RequestID,
	})
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return cubeboxsqlc.IamCubeboxTask{}, true, err
		}
		return existing, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return cubeboxsqlc.IamCubeboxTask{}, false, err
	}

	inserted, err := queries.InsertTask(ctx, cubeboxsqlc.InsertTaskParams{
		TenantUuid:               tenantUUID,
		TaskID:                   record.TaskID,
		ConversationID:           record.ConversationID,
		TurnID:                   record.TurnID,
		TaskType:                 record.TaskType,
		RequestID:                record.RequestID,
		RequestHash:              record.RequestHash,
		WorkflowID:               record.WorkflowID,
		Status:                   record.Status,
		DispatchStatus:           record.DispatchStatus,
		DispatchAttempt:          record.DispatchAttempt,
		DispatchDeadlineAt:       record.DispatchDeadlineAt,
		Attempt:                  record.Attempt,
		MaxAttempts:              record.MaxAttempts,
		LastErrorCode:            record.LastErrorCode,
		TraceID:                  record.TraceID,
		IntentSchemaVersion:      record.IntentSchemaVersion,
		CompilerContractVersion:  record.CompilerContractVersion,
		CapabilityMapVersion:     record.CapabilityMapVersion,
		SkillManifestDigest:      record.SkillManifestDigest,
		ContextHash:              record.ContextHash,
		IntentHash:               record.IntentHash,
		PlanHash:                 record.PlanHash,
		KnowledgeSnapshotDigest:  record.KnowledgeSnapshotDigest,
		RouteCatalogVersion:      record.RouteCatalogVersion,
		ResolverContractVersion:  record.ResolverContractVersion,
		ContextTemplateVersion:   record.ContextTemplateVersion,
		ReplyGuidanceVersion:     record.ReplyGuidanceVersion,
		PolicyContextDigest:      record.PolicyContextDigest,
		EffectivePolicyVersion:   record.EffectivePolicyVersion,
		ResolvedSetid:            record.ResolvedSetid,
		SetidSource:              record.SetidSource,
		PrecheckProjectionDigest: record.PrecheckProjectionDigest,
		MutationPolicyVersion:    record.MutationPolicyVersion,
		SubmittedAt:              record.SubmittedAt,
		CancelRequestedAt:        record.CancelRequestedAt,
		CompletedAt:              record.CompletedAt,
		CreatedAt:                record.CreatedAt,
		UpdatedAt:                record.UpdatedAt,
	})
	if err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, false, err
	}
	if err := queries.InsertTaskEvent(ctx, cubeboxsqlc.InsertTaskEventParams{
		TenantUuid: tenantUUID,
		TaskID:     inserted.TaskID,
		FromStatus: nil,
		ToStatus:   inserted.Status,
		EventType:  inserted.Status,
		ErrorCode:  nil,
		Payload:    nil,
		OccurredAt: inserted.SubmittedAt,
	}); err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, false, err
	}
	if err := queries.UpsertTaskDispatchOutbox(ctx, cubeboxsqlc.UpsertTaskDispatchOutboxParams{
		TenantUuid:  tenantUUID,
		TaskID:      inserted.TaskID,
		WorkflowID:  inserted.WorkflowID,
		Status:      inserted.DispatchStatus,
		Attempt:     0,
		NextRetryAt: inserted.SubmittedAt,
		CreatedAt:   inserted.SubmittedAt,
		UpdatedAt:   inserted.SubmittedAt,
	}); err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, false, err
	}
	return inserted, false, nil
}

func (s *PGStore) CancelTask(ctx context.Context, tenantID string, taskID string, now time.Time) (cubeboxsqlc.IamCubeboxTask, bool, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, false, err
	}
	taskUUID, err := parseUUID(taskID)
	if err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, false, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	record, err := queries.GetTaskByID(ctx, cubeboxsqlc.GetTaskByIDParams{
		TenantUuid: tenantUUID,
		TaskID:     taskUUID,
	})
	if err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, false, err
	}
	if record.Status == "succeeded" || record.Status == "failed" || record.Status == "canceled" {
		if err := tx.Commit(ctx); err != nil {
			return cubeboxsqlc.IamCubeboxTask{}, false, err
		}
		return record, false, nil
	}

	cancelRequestedAt := pgtype.Timestamptz{Time: now.UTC(), Valid: true}
	completedAt := pgtype.Timestamptz{Time: now.UTC(), Valid: true}
	dispatchStatus := record.DispatchStatus
	if dispatchStatus == "pending" {
		dispatchStatus = "failed"
	}
	updated, err := queries.UpdateTaskState(ctx, cubeboxsqlc.UpdateTaskStateParams{
		TenantUuid:         tenantUUID,
		TaskID:             taskUUID,
		Status:             "canceled",
		DispatchStatus:     dispatchStatus,
		DispatchAttempt:    record.DispatchAttempt,
		Attempt:            record.Attempt,
		LastErrorCode:      nil,
		CancelRequestedAt:  cancelRequestedAt,
		CompletedAt:        completedAt,
		UpdatedAt:          pgtype.Timestamptz{Time: now.UTC(), Valid: true},
	})
	if err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, false, err
	}

	if err := queries.InsertTaskEvent(ctx, cubeboxsqlc.InsertTaskEventParams{
		TenantUuid: tenantUUID,
		TaskID:     updated.TaskID,
		FromStatus: stringPtr(record.Status),
		ToStatus:   updated.Status,
		EventType:  "cancel_requested",
		ErrorCode:  nil,
		Payload:    nil,
		OccurredAt: pgtype.Timestamptz{Time: now.UTC(), Valid: true},
	}); err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, false, err
	}
	if err := queries.InsertTaskEvent(ctx, cubeboxsqlc.InsertTaskEventParams{
		TenantUuid: tenantUUID,
		TaskID:     updated.TaskID,
		FromStatus: stringPtr(record.Status),
		ToStatus:   updated.Status,
		EventType:  "canceled",
		ErrorCode:  nil,
		Payload:    nil,
		OccurredAt: pgtype.Timestamptz{Time: now.UTC(), Valid: true},
	}); err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, false, err
	}
	if _, err := queries.MarkTaskOutboxCanceled(ctx, cubeboxsqlc.MarkTaskOutboxCanceledParams{
		TenantUuid: tenantUUID,
		TaskID:     taskUUID,
		UpdatedAt:  pgtype.Timestamptz{Time: now.UTC(), Valid: true},
	}); err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, false, err
	}
	return updated, true, nil
}

func (s *PGStore) ListTaskEvents(ctx context.Context, tenantID string, taskID string) ([]cubeboxsqlc.IamCubeboxTaskEvent, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return nil, err
	}
	taskUUID, err := parseUUID(taskID)
	if err != nil {
		return nil, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	items, err := queries.ListTaskEventsByTask(ctx, cubeboxsqlc.ListTaskEventsByTaskParams{
		TenantUuid: tenantUUID,
		TaskID:     taskUUID,
	})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *PGStore) ListDispatchOutbox(ctx context.Context, tenantID string, status string, limit int32) ([]cubeboxsqlc.IamCubeboxTaskDispatchOutbox, error) {
	if limit <= 0 {
		limit = 20
	}
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return nil, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	items, err := queries.ListDispatchOutboxByStatus(ctx, cubeboxsqlc.ListDispatchOutboxByStatusParams{
		TenantUuid: tenantUUID,
		Status:     status,
		Limit:      limit,
	})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *PGStore) UpdateTaskState(ctx context.Context, tenantID string, update cubeboxdomain.TaskStateUpdate) (cubeboxsqlc.IamCubeboxTask, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, err
	}
	taskUUID, err := parseUUID(update.TaskID)
	if err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	record, err := queries.UpdateTaskState(ctx, cubeboxsqlc.UpdateTaskStateParams{
		TenantUuid:        tenantUUID,
		TaskID:            taskUUID,
		Status:            update.Status,
		DispatchStatus:    update.DispatchStatus,
		DispatchAttempt:   int32(update.DispatchAttempt),
		Attempt:           int32(update.Attempt),
		LastErrorCode:     stringPtr(update.LastErrorCode),
		CancelRequestedAt: timestamptzPtr(update.CancelRequestedAt),
		CompletedAt:       timestamptzPtr(update.CompletedAt),
		UpdatedAt:         pgtype.Timestamptz{Time: update.UpdatedAt.UTC(), Valid: true},
	})
	if err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return cubeboxsqlc.IamCubeboxTask{}, err
	}
	return record, nil
}

func (s *PGStore) InsertTaskEvent(ctx context.Context, tenantID string, event cubeboxdomain.TaskEventRecord) error {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return err
	}
	taskUUID, err := parseUUID(event.TaskID)
	if err != nil {
		return err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if err := queries.InsertTaskEvent(ctx, cubeboxsqlc.InsertTaskEventParams{
		TenantUuid: tenantUUID,
		TaskID:     taskUUID,
		FromStatus: stringPtr(event.FromStatus),
		ToStatus:   event.ToStatus,
		EventType:  event.EventType,
		ErrorCode:  stringPtr(event.ErrorCode),
		Payload:    nil,
		OccurredAt: pgtype.Timestamptz{Time: event.OccurredAt.UTC(), Valid: true},
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PGStore) UpdateTaskDispatchOutbox(ctx context.Context, tenantID string, update cubeboxdomain.TaskDispatchOutboxUpdate) error {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return err
	}
	taskUUID, err := parseUUID(update.TaskID)
	if err != nil {
		return err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := queries.UpdateTaskDispatchOutbox(ctx, cubeboxsqlc.UpdateTaskDispatchOutboxParams{
		TenantUuid:  tenantUUID,
		TaskID:      taskUUID,
		Status:      update.Status,
		Attempt:     int32(update.Attempt),
		NextRetryAt: pgtype.Timestamptz{Time: update.NextRetryAt.UTC(), Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: update.UpdatedAt.UTC(), Valid: true},
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PGStore) ListFiles(ctx context.Context, tenantID string, conversationID string, limit int32) ([]cubeboxsqlc.IamCubeboxFile, error) {
	if limit <= 0 {
		limit = 200
	}
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return nil, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var (
		items    []cubeboxsqlc.IamCubeboxFile
		queryErr error
	)
	if conversationID == "" {
		items, queryErr = queries.ListFilesByTenant(ctx, cubeboxsqlc.ListFilesByTenantParams{
			TenantUuid: tenantUUID,
			Limit:      limit,
		})
	} else {
		items, queryErr = queries.ListFilesByConversation(ctx, cubeboxsqlc.ListFilesByConversationParams{
			TenantUuid:     tenantUUID,
			ConversationID: conversationID,
		})
	}
	if queryErr != nil {
		return nil, queryErr
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *PGStore) GetFile(ctx context.Context, tenantID string, fileID string) (cubeboxsqlc.IamCubeboxFile, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return cubeboxsqlc.IamCubeboxFile{}, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return cubeboxsqlc.IamCubeboxFile{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	item, err := queries.GetFileByID(ctx, cubeboxsqlc.GetFileByIDParams{
		TenantUuid: tenantUUID,
		FileID:     fileID,
	})
	if err != nil {
		return cubeboxsqlc.IamCubeboxFile{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return cubeboxsqlc.IamCubeboxFile{}, err
	}
	return item, nil
}

func timestamptzPtr(ts *time.Time) pgtype.Timestamptz {
	if ts == nil || ts.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: ts.UTC(), Valid: true}
}

func (s *PGStore) ListConversationFileLinks(ctx context.Context, tenantID string, conversationID string) ([]cubeboxsqlc.IamCubeboxFileLink, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return nil, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	items, err := queries.ListFileLinksByConversation(ctx, cubeboxsqlc.ListFileLinksByConversationParams{
		TenantUuid:     tenantUUID,
		ConversationID: conversationID,
	})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *PGStore) beginTenantTx(ctx context.Context, tenantID string) (pgx.Tx, *cubeboxsqlc.Queries, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		_ = tx.Rollback(context.Background())
		return nil, nil, err
	}
	return tx, cubeboxsqlc.New(tx), nil
}

func parseUUID(raw string) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}

func stringPtr(value string) *string {
	trimmed := value
	return &trimmed
}

func syncTurnSnapshot(
	ctx context.Context,
	tx pgx.Tx,
	tenantUUID pgtype.UUID,
	conversationID string,
	turn cubeboxdomain.ConversationTurn,
) error {
	intentJSON, err := marshalJSON(turn.Intent, true)
	if err != nil {
		return err
	}
	planJSON, err := marshalJSON(turn.Plan, true)
	if err != nil {
		return err
	}
	candidatesJSON, err := marshalJSON(turn.Candidates, false)
	if err != nil {
		return err
	}
	clarificationJSON, err := marshalJSON(turn.Clarification, true)
	if err != nil {
		return err
	}
	dryRunJSON, err := marshalJSON(turn.DryRun, true)
	if err != nil {
		return err
	}
	routeDecisionJSON, err := marshalNullableJSON(turn.RouteDecision)
	if err != nil {
		return err
	}
	missingFieldsJSON, err := marshalJSON(turn.MissingFields, false)
	if err != nil {
		return err
	}
	commitResultJSON, err := marshalNullableJSON(turn.CommitResult)
	if err != nil {
		return err
	}
	commitReplyJSON, err := marshalNullableJSON(turn.CommitReply)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO iam.cubebox_turns (
  tenant_uuid,
  conversation_id,
  turn_id,
  user_input,
  state,
  phase,
  risk_tier,
  request_id,
  trace_id,
  policy_version,
  composition_version,
  mapping_version,
  intent_json,
  plan_json,
  candidates_json,
  candidate_options,
  resolved_candidate_id,
  selected_candidate_id,
  ambiguity_count,
  confidence,
  resolution_source,
  route_decision_json,
  clarification_json,
  dry_run_json,
  pending_draft_summary,
  missing_fields,
  commit_result_json,
  commit_reply,
  error_code,
  created_at,
  updated_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
  $13::jsonb, $14::jsonb, $15::jsonb, $16::jsonb,
  $17, $18, $19, $20, $21, $22::jsonb, $23::jsonb, $24::jsonb,
  $25, $26::jsonb, $27::jsonb, $28::jsonb, $29, $30, $31
)
ON CONFLICT (tenant_uuid, conversation_id, turn_id)
DO UPDATE SET
  user_input = EXCLUDED.user_input,
  state = EXCLUDED.state,
  phase = EXCLUDED.phase,
  risk_tier = EXCLUDED.risk_tier,
  request_id = EXCLUDED.request_id,
  trace_id = EXCLUDED.trace_id,
  policy_version = EXCLUDED.policy_version,
  composition_version = EXCLUDED.composition_version,
  mapping_version = EXCLUDED.mapping_version,
  intent_json = EXCLUDED.intent_json,
  plan_json = EXCLUDED.plan_json,
  candidates_json = EXCLUDED.candidates_json,
  candidate_options = EXCLUDED.candidate_options,
  resolved_candidate_id = EXCLUDED.resolved_candidate_id,
  selected_candidate_id = EXCLUDED.selected_candidate_id,
  ambiguity_count = EXCLUDED.ambiguity_count,
  confidence = EXCLUDED.confidence,
  resolution_source = EXCLUDED.resolution_source,
  route_decision_json = EXCLUDED.route_decision_json,
  clarification_json = EXCLUDED.clarification_json,
  dry_run_json = EXCLUDED.dry_run_json,
  pending_draft_summary = EXCLUDED.pending_draft_summary,
  missing_fields = EXCLUDED.missing_fields,
  commit_result_json = EXCLUDED.commit_result_json,
  commit_reply = EXCLUDED.commit_reply,
  error_code = EXCLUDED.error_code,
  updated_at = EXCLUDED.updated_at
`,
		tenantUUID,
		strings.TrimSpace(conversationID),
		strings.TrimSpace(turn.TurnID),
		strings.TrimSpace(turn.UserInput),
		strings.TrimSpace(turn.State),
		strings.TrimSpace(turn.Phase),
		strings.TrimSpace(turn.RiskTier),
		strings.TrimSpace(turn.RequestID),
		strings.TrimSpace(turn.TraceID),
		strings.TrimSpace(turn.PolicyVersion),
		strings.TrimSpace(turn.CompositionVersion),
		strings.TrimSpace(turn.MappingVersion),
		intentJSON,
		planJSON,
		candidatesJSON,
		[]byte("[]"),
		nilIfBlank(turn.ResolvedCandidateID),
		nilIfBlank(turn.SelectedCandidateID),
		int32(turn.AmbiguityCount),
		turn.Confidence,
		nilIfBlank(turn.ResolutionSource),
		routeDecisionJSON,
		clarificationJSON,
		dryRunJSON,
		nilIfBlank(turn.PendingDraftSummary),
		missingFieldsJSON,
		commitResultJSON,
		commitReplyJSON,
		nilIfBlank(turn.ErrorCode),
		pgtype.Timestamptz{Time: turn.CreatedAt.UTC(), Valid: !turn.CreatedAt.IsZero()},
		pgtype.Timestamptz{Time: turn.UpdatedAt.UTC(), Valid: !turn.UpdatedAt.IsZero()},
	); err != nil {
		return err
	}
	return nil
}

func syncTransitionSnapshot(
	ctx context.Context,
	tx pgx.Tx,
	tenantUUID pgtype.UUID,
	conversationID string,
	transition cubeboxdomain.StateTransition,
) error {
	if _, err := tx.Exec(ctx, `
INSERT INTO iam.cubebox_state_transitions (
  tenant_uuid,
  conversation_id,
  turn_id,
  turn_action,
  request_id,
  trace_id,
  from_state,
  to_state,
  from_phase,
  to_phase,
  reason_code,
  actor_id,
  changed_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
)
`,
		tenantUUID,
		strings.TrimSpace(conversationID),
		nilIfBlank(transition.TurnID),
		nilIfBlank(transition.TurnAction),
		strings.TrimSpace(transition.RequestID),
		strings.TrimSpace(transition.TraceID),
		strings.TrimSpace(transition.FromState),
		strings.TrimSpace(transition.ToState),
		strings.TrimSpace(transition.FromPhase),
		strings.TrimSpace(transition.ToPhase),
		nilIfBlank(transition.ReasonCode),
		strings.TrimSpace(transition.ActorID),
		pgtype.Timestamptz{Time: transition.ChangedAt.UTC(), Valid: !transition.ChangedAt.IsZero()},
	); err != nil {
		if isUniqueViolation(err) {
			return nil
		}
		return err
	}
	return nil
}

func marshalJSON(value any, objectDefault bool) ([]byte, error) {
	if value == nil {
		if objectDefault {
			return []byte("{}"), nil
		}
		return []byte("[]"), nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		if objectDefault {
			return []byte("{}"), nil
		}
		return []byte("[]"), nil
	}
	return raw, nil
}

func marshalNullableJSON(value any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	return raw, nil
}

func nilIfBlank(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
