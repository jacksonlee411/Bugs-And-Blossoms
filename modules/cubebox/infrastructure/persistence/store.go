package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	cubeboxdomain "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/domain"
	cubeboxsqlc "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/infrastructure/sqlc/gen"
)

const (
	fileScanStatusReady        = "ready"
	fileCleanupStatusPending   = "pending"
	fileCleanupReasonMetaWrite = "metadata_write_failed"
	fileCleanupReasonObjDelete = "object_delete_failed"
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

func (s *PGStore) ListConversations(ctx context.Context, tenantID string, actorID string, limit int32, cursorUpdatedAt time.Time, cursorConversationID string) ([]cubeboxdomain.ConversationRecord, error) {
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
	return mapConversationRecords(items), nil
}

func (s *PGStore) GetConversation(ctx context.Context, tenantID string, conversationID string) (cubeboxdomain.ConversationRecord, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return cubeboxdomain.ConversationRecord{}, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return cubeboxdomain.ConversationRecord{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	item, err := queries.GetConversationByID(ctx, cubeboxsqlc.GetConversationByIDParams{
		TenantUuid:     tenantUUID,
		ConversationID: conversationID,
	})
	if err != nil {
		return cubeboxdomain.ConversationRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return cubeboxdomain.ConversationRecord{}, err
	}
	return mapConversationRecord(item), nil
}

func (s *PGStore) ListConversationTurns(ctx context.Context, tenantID string, conversationID string) ([]cubeboxdomain.ConversationTurnRecord, error) {
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
	return mapTurnRecords(items), nil
}

func (s *PGStore) ListConversationStateTransitions(ctx context.Context, tenantID string, conversationID string) ([]cubeboxdomain.StateTransitionRecord, error) {
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
	return mapTransitionRecords(items), nil
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

func (s *PGStore) GetTask(ctx context.Context, tenantID string, taskID string) (cubeboxdomain.TaskRecord, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return cubeboxdomain.TaskRecord{}, err
	}
	taskUUID, err := parseUUID(taskID)
	if err != nil {
		return cubeboxdomain.TaskRecord{}, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return cubeboxdomain.TaskRecord{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	item, err := queries.GetTaskByID(ctx, cubeboxsqlc.GetTaskByIDParams{
		TenantUuid: tenantUUID,
		TaskID:     taskUUID,
	})
	if err != nil {
		return cubeboxdomain.TaskRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return cubeboxdomain.TaskRecord{}, err
	}
	return mapTaskRecord(item), nil
}

func (s *PGStore) GetTaskForDispatch(ctx context.Context, tenantID string, taskID string) (cubeboxdomain.TaskRecord, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return cubeboxdomain.TaskRecord{}, err
	}
	taskUUID, err := parseUUID(taskID)
	if err != nil {
		return cubeboxdomain.TaskRecord{}, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return cubeboxdomain.TaskRecord{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	item, err := queries.GetTaskByIDForUpdate(ctx, cubeboxsqlc.GetTaskByIDForUpdateParams{
		TenantUuid: tenantUUID,
		TaskID:     taskUUID,
	})
	if err != nil {
		return cubeboxdomain.TaskRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return cubeboxdomain.TaskRecord{}, err
	}
	return mapTaskRecord(item), nil
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

func (s *PGStore) SubmitTask(ctx context.Context, tenantID string, record cubeboxdomain.TaskRecord) (cubeboxdomain.TaskRecord, bool, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return cubeboxdomain.TaskRecord{}, false, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return cubeboxdomain.TaskRecord{}, false, err
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
			return cubeboxdomain.TaskRecord{}, true, err
		}
		return mapTaskRecord(existing), true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return cubeboxdomain.TaskRecord{}, false, err
	}

	inserted, err := queries.InsertTask(ctx, cubeboxsqlc.InsertTaskParams{
		TenantUuid:               tenantUUID,
		TaskID:                   mustParseUUID(record.TaskID),
		ConversationID:           record.ConversationID,
		TurnID:                   record.TurnID,
		TaskType:                 record.TaskType,
		RequestID:                record.RequestID,
		RequestHash:              record.RequestHash,
		WorkflowID:               record.WorkflowID,
		Status:                   record.Status,
		DispatchStatus:           record.DispatchStatus,
		DispatchAttempt:          int32(record.DispatchAttempt),
		DispatchDeadlineAt:       timestamptzPtr(record.DispatchDeadlineAt),
		Attempt:                  int32(record.Attempt),
		MaxAttempts:              int32(record.MaxAttempts),
		LastErrorCode:            stringPtr(record.LastErrorCode),
		TraceID:                  stringPtr(record.TraceID),
		IntentSchemaVersion:      record.IntentSchemaVersion,
		CompilerContractVersion:  record.CompilerContractVersion,
		CapabilityMapVersion:     record.CapabilityMapVersion,
		SkillManifestDigest:      record.SkillManifestDigest,
		ContextHash:              record.ContextHash,
		IntentHash:               record.IntentHash,
		PlanHash:                 record.PlanHash,
		KnowledgeSnapshotDigest:  stringPtr(record.KnowledgeSnapshotDigest),
		RouteCatalogVersion:      stringPtr(record.RouteCatalogVersion),
		ResolverContractVersion:  stringPtr(record.ResolverContractVersion),
		ContextTemplateVersion:   stringPtr(record.ContextTemplateVersion),
		ReplyGuidanceVersion:     stringPtr(record.ReplyGuidanceVersion),
		PolicyContextDigest:      stringPtr(record.PolicyContextDigest),
		EffectivePolicyVersion:   stringPtr(record.EffectivePolicyVersion),
		ResolvedSetid:            stringPtr(record.ResolvedSetID),
		SetidSource:              stringPtr(record.SetIDSource),
		PrecheckProjectionDigest: stringPtr(record.PrecheckProjectionDigest),
		MutationPolicyVersion:    stringPtr(record.MutationPolicyVersion),
		SubmittedAt:              timestamptzValue(record.SubmittedAt),
		CancelRequestedAt:        timestamptzPtr(record.CancelRequestedAt),
		CompletedAt:              timestamptzPtr(record.CompletedAt),
		CreatedAt:                timestamptzValue(record.CreatedAt),
		UpdatedAt:                timestamptzValue(record.UpdatedAt),
	})
	if err != nil {
		return cubeboxdomain.TaskRecord{}, false, err
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
		return cubeboxdomain.TaskRecord{}, false, err
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
		return cubeboxdomain.TaskRecord{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return cubeboxdomain.TaskRecord{}, false, err
	}
	return mapTaskRecord(inserted), false, nil
}

func (s *PGStore) CancelTask(ctx context.Context, tenantID string, taskID string, now time.Time) (cubeboxdomain.TaskRecord, bool, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return cubeboxdomain.TaskRecord{}, false, err
	}
	taskUUID, err := parseUUID(taskID)
	if err != nil {
		return cubeboxdomain.TaskRecord{}, false, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return cubeboxdomain.TaskRecord{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	record, err := queries.GetTaskByID(ctx, cubeboxsqlc.GetTaskByIDParams{
		TenantUuid: tenantUUID,
		TaskID:     taskUUID,
	})
	if err != nil {
		return cubeboxdomain.TaskRecord{}, false, err
	}
	if record.Status == "succeeded" || record.Status == "failed" || record.Status == "canceled" {
		if err := tx.Commit(ctx); err != nil {
			return cubeboxdomain.TaskRecord{}, false, err
		}
		return mapTaskRecord(record), false, nil
	}

	cancelRequestedAt := pgtype.Timestamptz{Time: now.UTC(), Valid: true}
	completedAt := pgtype.Timestamptz{Time: now.UTC(), Valid: true}
	dispatchStatus := record.DispatchStatus
	if dispatchStatus == "pending" {
		dispatchStatus = "failed"
	}
	updated, err := queries.UpdateTaskState(ctx, cubeboxsqlc.UpdateTaskStateParams{
		TenantUuid:        tenantUUID,
		TaskID:            taskUUID,
		Status:            "canceled",
		DispatchStatus:    dispatchStatus,
		DispatchAttempt:   record.DispatchAttempt,
		Attempt:           record.Attempt,
		LastErrorCode:     nil,
		CancelRequestedAt: cancelRequestedAt,
		CompletedAt:       completedAt,
		UpdatedAt:         pgtype.Timestamptz{Time: now.UTC(), Valid: true},
	})
	if err != nil {
		return cubeboxdomain.TaskRecord{}, false, err
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
		return cubeboxdomain.TaskRecord{}, false, err
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
		return cubeboxdomain.TaskRecord{}, false, err
	}
	if _, err := queries.MarkTaskOutboxCanceled(ctx, cubeboxsqlc.MarkTaskOutboxCanceledParams{
		TenantUuid: tenantUUID,
		TaskID:     taskUUID,
		UpdatedAt:  pgtype.Timestamptz{Time: now.UTC(), Valid: true},
	}); err != nil {
		return cubeboxdomain.TaskRecord{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return cubeboxdomain.TaskRecord{}, false, err
	}
	return mapTaskRecord(updated), true, nil
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

func (s *PGStore) ListDispatchOutbox(ctx context.Context, tenantID string, status string, limit int32) ([]cubeboxdomain.TaskDispatchOutboxRecord, error) {
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
	return mapDispatchOutboxRecords(items), nil
}

func (s *PGStore) UpdateTaskState(ctx context.Context, tenantID string, update cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return cubeboxdomain.TaskRecord{}, err
	}
	taskUUID, err := parseUUID(update.TaskID)
	if err != nil {
		return cubeboxdomain.TaskRecord{}, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return cubeboxdomain.TaskRecord{}, err
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
		return cubeboxdomain.TaskRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return cubeboxdomain.TaskRecord{}, err
	}
	return mapTaskRecord(record), nil
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

func (s *PGStore) ConversationExists(ctx context.Context, tenantID string, conversationID string) (bool, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return false, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	exists, err := queries.ConversationExists(ctx, cubeboxsqlc.ConversationExistsParams{
		TenantUuid:     tenantUUID,
		ConversationID: conversationID,
	})
	if err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return exists, nil
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

func (s *PGStore) InsertFile(
	ctx context.Context,
	tenantID string,
	record cubeboxdomain.FileObject,
	fileID string,
	actorID string,
	now time.Time,
) (cubeboxsqlc.IamCubeboxFile, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return cubeboxsqlc.IamCubeboxFile{}, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return cubeboxsqlc.IamCubeboxFile{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	inserted, err := queries.InsertFile(ctx, cubeboxsqlc.InsertFileParams{
		TenantUuid:      tenantUUID,
		FileID:          strings.TrimSpace(fileID),
		StorageProvider: strings.TrimSpace(record.StorageProvider),
		StorageKey:      strings.TrimSpace(record.StorageKey),
		FileName:        strings.TrimSpace(record.Filename),
		MediaType:       strings.TrimSpace(record.ContentType),
		SizeBytes:       record.SizeBytes,
		Sha256:          strings.TrimSpace(record.SHA256),
		ScanStatus:      fileScanStatusReady,
		UploadedBy:      strings.TrimSpace(actorID),
		UploadedAt:      timestamptzValue(now),
		UpdatedAt:       timestamptzValue(now),
	})
	if err != nil {
		return cubeboxsqlc.IamCubeboxFile{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return cubeboxsqlc.IamCubeboxFile{}, err
	}
	return inserted, nil
}

func (s *PGStore) CreateFile(
	ctx context.Context,
	tenantID string,
	record cubeboxdomain.FileObject,
	fileID string,
	actorID string,
	conversationID string,
	now time.Time,
) (cubeboxsqlc.IamCubeboxFile, []cubeboxsqlc.IamCubeboxFileLink, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return cubeboxsqlc.IamCubeboxFile{}, nil, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return cubeboxsqlc.IamCubeboxFile{}, nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	inserted, err := queries.InsertFile(ctx, cubeboxsqlc.InsertFileParams{
		TenantUuid:      tenantUUID,
		FileID:          strings.TrimSpace(fileID),
		StorageProvider: strings.TrimSpace(record.StorageProvider),
		StorageKey:      strings.TrimSpace(record.StorageKey),
		FileName:        strings.TrimSpace(record.Filename),
		MediaType:       strings.TrimSpace(record.ContentType),
		SizeBytes:       record.SizeBytes,
		Sha256:          strings.TrimSpace(record.SHA256),
		ScanStatus:      fileScanStatusReady,
		UploadedBy:      strings.TrimSpace(actorID),
		UploadedAt:      timestamptzValue(now),
		UpdatedAt:       timestamptzValue(now),
	})
	if err != nil {
		return cubeboxsqlc.IamCubeboxFile{}, nil, err
	}

	links := []cubeboxsqlc.IamCubeboxFileLink{}
	if strings.TrimSpace(conversationID) != "" {
		link, linkErr := queries.InsertConversationFileLink(ctx, cubeboxsqlc.InsertConversationFileLinkParams{
			TenantUuid:     tenantUUID,
			FileID:         strings.TrimSpace(fileID),
			ConversationID: strings.TrimSpace(conversationID),
			CreatedBy:      strings.TrimSpace(actorID),
		})
		if linkErr != nil {
			return cubeboxsqlc.IamCubeboxFile{}, nil, linkErr
		}
		links = append(links, link)
	}
	if err := tx.Commit(ctx); err != nil {
		return cubeboxsqlc.IamCubeboxFile{}, nil, err
	}
	return inserted, links, nil
}

func (s *PGStore) CountFileLinks(ctx context.Context, tenantID string, fileID string) (int64, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return 0, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	count, err := queries.CountFileLinksByFileID(ctx, cubeboxsqlc.CountFileLinksByFileIDParams{
		TenantUuid: tenantUUID,
		FileID:     fileID,
	})
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *PGStore) DeleteFile(ctx context.Context, tenantID string, fileID string) (int64, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return 0, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	rows, err := queries.DeleteFileByID(ctx, cubeboxsqlc.DeleteFileByIDParams{
		TenantUuid: tenantUUID,
		FileID:     fileID,
	})
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return rows, nil
}

func timestamptzPtr(ts *time.Time) pgtype.Timestamptz {
	if ts == nil || ts.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: ts.UTC(), Valid: true}
}

func timestamptzValue(ts time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: ts.UTC(), Valid: !ts.IsZero()}
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

func (s *PGStore) ListFileLinks(ctx context.Context, tenantID string, fileID string) ([]cubeboxsqlc.IamCubeboxFileLink, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return nil, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	items, err := queries.ListFileLinksByFileID(ctx, cubeboxsqlc.ListFileLinksByFileIDParams{
		TenantUuid: tenantUUID,
		FileID:     fileID,
	})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *PGStore) ListTenantFileLinks(ctx context.Context, tenantID string) ([]cubeboxsqlc.IamCubeboxFileLink, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return nil, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	items, err := queries.ListFileLinksByTenant(ctx, tenantUUID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *PGStore) InsertFileCleanupJob(ctx context.Context, tenantID string, job cubeboxdomain.FileCleanupJob, now time.Time) (cubeboxsqlc.IamCubeboxFileCleanupJob, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return cubeboxsqlc.IamCubeboxFileCleanupJob{}, err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return cubeboxsqlc.IamCubeboxFileCleanupJob{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	reason := strings.TrimSpace(job.CleanupReason)
	switch reason {
	case fileCleanupReasonMetaWrite, fileCleanupReasonObjDelete:
	default:
		reason = fileCleanupReasonMetaWrite
	}

	inserted, err := queries.InsertFileCleanupJob(ctx, cubeboxsqlc.InsertFileCleanupJobParams{
		TenantUuid:      tenantUUID,
		FileID:          strings.TrimSpace(job.FileID),
		StorageProvider: strings.TrimSpace(job.StorageProvider),
		StorageKey:      strings.TrimSpace(job.StorageKey),
		CleanupReason:   reason,
		Status:          fileCleanupStatusPending,
		AttemptCount:    0,
		NextRetryAt:     timestamptzValue(now),
		LastError:       stringPtr(strings.TrimSpace(job.LastError)),
		CreatedAt:       timestamptzValue(now),
		UpdatedAt:       timestamptzValue(now),
	})
	if err != nil {
		return cubeboxsqlc.IamCubeboxFileCleanupJob{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return cubeboxsqlc.IamCubeboxFileCleanupJob{}, err
	}
	return inserted, nil
}

func (s *PGStore) Healthy(ctx context.Context, tenantID string) error {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		tenantID = uuid.Nil.String()
	}
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return err
	}
	tx, queries, err := s.beginTenantTx(ctx, tenantID)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := queries.ListFilesByTenant(ctx, cubeboxsqlc.ListFilesByTenantParams{
		TenantUuid: tenantUUID,
		Limit:      1,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
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

func mustParseUUID(raw string) pgtype.UUID {
	parsed, _ := parseUUID(raw)
	return parsed
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func stringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func timePtr(ts time.Time) *time.Time {
	value := ts.UTC()
	return &value
}

func mapConversationRecord(item cubeboxsqlc.IamCubeboxConversation) cubeboxdomain.ConversationRecord {
	return cubeboxdomain.ConversationRecord{
		ConversationID: strings.TrimSpace(item.ConversationID),
		ActorID:        strings.TrimSpace(item.ActorID),
		ActorRole:      strings.TrimSpace(item.ActorRole),
		State:          strings.TrimSpace(item.State),
		CurrentPhase:   strings.TrimSpace(item.CurrentPhase),
		CreatedAt:      item.CreatedAt.Time.UTC(),
		UpdatedAt:      item.UpdatedAt.Time.UTC(),
	}
}

func mapConversationRecords(items []cubeboxsqlc.IamCubeboxConversation) []cubeboxdomain.ConversationRecord {
	out := make([]cubeboxdomain.ConversationRecord, 0, len(items))
	for _, item := range items {
		out = append(out, mapConversationRecord(item))
	}
	return out
}

func mapTurnRecord(item cubeboxsqlc.IamCubeboxTurn) cubeboxdomain.ConversationTurnRecord {
	return cubeboxdomain.ConversationTurnRecord{
		TurnID:              strings.TrimSpace(item.TurnID),
		UserInput:           strings.TrimSpace(item.UserInput),
		State:               strings.TrimSpace(item.State),
		Phase:               strings.TrimSpace(item.Phase),
		RiskTier:            strings.TrimSpace(item.RiskTier),
		RequestID:           strings.TrimSpace(item.RequestID),
		TraceID:             strings.TrimSpace(item.TraceID),
		PolicyVersion:       strings.TrimSpace(item.PolicyVersion),
		CompositionVersion:  strings.TrimSpace(item.CompositionVersion),
		MappingVersion:      strings.TrimSpace(item.MappingVersion),
		IntentJSON:          append([]byte(nil), item.IntentJson...),
		RouteDecisionJSON:   append([]byte(nil), item.RouteDecisionJson...),
		ClarificationJSON:   append([]byte(nil), item.ClarificationJson...),
		CandidatesJSON:      append([]byte(nil), item.CandidatesJson...),
		PlanJSON:            append([]byte(nil), item.PlanJson...),
		DryRunJSON:          append([]byte(nil), item.DryRunJson...),
		ResolvedCandidateID: stringValue(item.ResolvedCandidateID),
		SelectedCandidateID: stringValue(item.SelectedCandidateID),
		AmbiguityCount:      int(item.AmbiguityCount),
		Confidence:          item.Confidence,
		ResolutionSource:    stringValue(item.ResolutionSource),
		PendingDraftSummary: stringValue(item.PendingDraftSummary),
		MissingFieldsJSON:   append([]byte(nil), item.MissingFields...),
		CommitResultJSON:    append([]byte(nil), item.CommitResultJson...),
		CommitReplyJSON:     append([]byte(nil), item.CommitReply...),
		ErrorCode:           stringValue(item.ErrorCode),
		CreatedAt:           item.CreatedAt.Time.UTC(),
		UpdatedAt:           item.UpdatedAt.Time.UTC(),
	}
}

func mapTurnRecords(items []cubeboxsqlc.IamCubeboxTurn) []cubeboxdomain.ConversationTurnRecord {
	out := make([]cubeboxdomain.ConversationTurnRecord, 0, len(items))
	for _, item := range items {
		out = append(out, mapTurnRecord(item))
	}
	return out
}

func mapTransitionRecord(item cubeboxsqlc.IamCubeboxStateTransition) cubeboxdomain.StateTransitionRecord {
	return cubeboxdomain.StateTransitionRecord{
		ID:         item.ID,
		TurnID:     stringValue(item.TurnID),
		TurnAction: stringValue(item.TurnAction),
		RequestID:  strings.TrimSpace(item.RequestID),
		TraceID:    strings.TrimSpace(item.TraceID),
		FromState:  strings.TrimSpace(item.FromState),
		ToState:    strings.TrimSpace(item.ToState),
		FromPhase:  strings.TrimSpace(item.FromPhase),
		ToPhase:    strings.TrimSpace(item.ToPhase),
		ReasonCode: stringValue(item.ReasonCode),
		ActorID:    strings.TrimSpace(item.ActorID),
		ChangedAt:  item.ChangedAt.Time.UTC(),
	}
}

func mapTransitionRecords(items []cubeboxsqlc.IamCubeboxStateTransition) []cubeboxdomain.StateTransitionRecord {
	out := make([]cubeboxdomain.StateTransitionRecord, 0, len(items))
	for _, item := range items {
		out = append(out, mapTransitionRecord(item))
	}
	return out
}

func mapTaskRecord(item cubeboxsqlc.IamCubeboxTask) cubeboxdomain.TaskRecord {
	record := cubeboxdomain.TaskRecord{
		TaskID:                   item.TaskID.String(),
		ConversationID:           strings.TrimSpace(item.ConversationID),
		TurnID:                   strings.TrimSpace(item.TurnID),
		TaskType:                 strings.TrimSpace(item.TaskType),
		RequestID:                strings.TrimSpace(item.RequestID),
		RequestHash:              strings.TrimSpace(item.RequestHash),
		WorkflowID:               strings.TrimSpace(item.WorkflowID),
		Status:                   strings.TrimSpace(item.Status),
		DispatchStatus:           strings.TrimSpace(item.DispatchStatus),
		DispatchAttempt:          int(item.DispatchAttempt),
		Attempt:                  int(item.Attempt),
		MaxAttempts:              int(item.MaxAttempts),
		LastErrorCode:            stringValue(item.LastErrorCode),
		TraceID:                  stringValue(item.TraceID),
		IntentSchemaVersion:      strings.TrimSpace(item.IntentSchemaVersion),
		CompilerContractVersion:  strings.TrimSpace(item.CompilerContractVersion),
		CapabilityMapVersion:     strings.TrimSpace(item.CapabilityMapVersion),
		SkillManifestDigest:      strings.TrimSpace(item.SkillManifestDigest),
		ContextHash:              strings.TrimSpace(item.ContextHash),
		IntentHash:               strings.TrimSpace(item.IntentHash),
		PlanHash:                 strings.TrimSpace(item.PlanHash),
		KnowledgeSnapshotDigest:  stringValue(item.KnowledgeSnapshotDigest),
		RouteCatalogVersion:      stringValue(item.RouteCatalogVersion),
		ResolverContractVersion:  stringValue(item.ResolverContractVersion),
		ContextTemplateVersion:   stringValue(item.ContextTemplateVersion),
		ReplyGuidanceVersion:     stringValue(item.ReplyGuidanceVersion),
		PolicyContextDigest:      stringValue(item.PolicyContextDigest),
		EffectivePolicyVersion:   stringValue(item.EffectivePolicyVersion),
		ResolvedSetID:            stringValue(item.ResolvedSetid),
		SetIDSource:              stringValue(item.SetidSource),
		PrecheckProjectionDigest: stringValue(item.PrecheckProjectionDigest),
		MutationPolicyVersion:    stringValue(item.MutationPolicyVersion),
		SubmittedAt:              item.SubmittedAt.Time.UTC(),
		CreatedAt:                item.CreatedAt.Time.UTC(),
		UpdatedAt:                item.UpdatedAt.Time.UTC(),
	}
	if item.DispatchDeadlineAt.Valid {
		record.DispatchDeadlineAt = timePtr(item.DispatchDeadlineAt.Time.UTC())
	}
	if item.CancelRequestedAt.Valid {
		record.CancelRequestedAt = timePtr(item.CancelRequestedAt.Time.UTC())
	}
	if item.CompletedAt.Valid {
		record.CompletedAt = timePtr(item.CompletedAt.Time.UTC())
	}
	return record
}

func mapDispatchOutboxRecords(items []cubeboxsqlc.IamCubeboxTaskDispatchOutbox) []cubeboxdomain.TaskDispatchOutboxRecord {
	out := make([]cubeboxdomain.TaskDispatchOutboxRecord, 0, len(items))
	for _, item := range items {
		nextRetryAt := time.Time{}
		if item.NextRetryAt.Valid {
			nextRetryAt = item.NextRetryAt.Time.UTC()
		}
		out = append(out, cubeboxdomain.TaskDispatchOutboxRecord{
			TaskID:      item.TaskID.String(),
			Status:      strings.TrimSpace(item.Status),
			Attempt:     int(item.Attempt),
			NextRetryAt: nextRetryAt,
		})
	}
	return out
}

func nonEmptyStringPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
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
