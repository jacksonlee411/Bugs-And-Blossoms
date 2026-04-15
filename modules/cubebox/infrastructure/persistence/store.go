package persistence

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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

func (s *PGStore) ListConversations(ctx context.Context, tenantID string, actorID string, limit int32) ([]cubeboxsqlc.IamCubeboxConversation, error) {
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
		TenantUuid: tenantUUID,
		ActorID:    actorID,
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
		items []cubeboxsqlc.IamCubeboxFile
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
