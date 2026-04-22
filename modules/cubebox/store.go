package cubebox

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	cubeboxsqlc "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/infrastructure/sqlc/gen"
)

var ErrConversationNotFound = errors.New("CUBEBOX_CONVERSATION_NOT_FOUND")

type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type Store struct {
	pool TxBeginner
}

type ConversationSummary struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Archived  bool   `json:"archived"`
	UpdatedAt string `json:"updated_at"`
}

func NewStore(pool TxBeginner) *Store {
	return &Store{pool: pool}
}

func (s *Store) CreateConversation(ctx context.Context, tenantID string, principalID string) (ConversationReplayResponse, error) {
	conversationID := "conv_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	now := time.Now().UTC()
	conversation := Conversation{
		ID:       conversationID,
		Title:    "新对话",
		Status:   "active",
		Archived: false,
	}
	event := CanonicalEvent{
		EventID:        "evt_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		ConversationID: conversationID,
		TurnID:         nil,
		Sequence:       1,
		Type:           "conversation.loaded",
		TS:             now.Format(time.RFC3339),
		Payload: map[string]any{
			"title":    conversation.Title,
			"status":   conversation.Status,
			"archived": conversation.Archived,
		},
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ConversationReplayResponse{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return ConversationReplayResponse{}, err
	}

	q := cubeboxsqlc.New(tx)
	if _, err := q.CreateConversation(ctx, cubeboxsqlc.CreateConversationParams{
		Column1:        uuidToPGType(tenantID),
		ConversationID: conversationID,
		Column3:        uuidToPGType(principalID),
		Title:          conversation.Title,
		Status:         conversation.Status,
		Archived:       conversation.Archived,
		CreatedAt:      timestamptz(now),
		UpdatedAt:      timestamptz(now),
		ArchivedAt:     nullTimestamptz(),
	}); err != nil {
		return ConversationReplayResponse{}, err
	}
	if err := appendEvent(ctx, q, tenantID, event, now); err != nil {
		return ConversationReplayResponse{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return ConversationReplayResponse{}, err
	}
	return ConversationReplayResponse{
		Conversation: conversation,
		Events:       []CanonicalEvent{event},
		NextSequence: 2,
	}, nil
}

func (s *Store) GetConversation(ctx context.Context, tenantID string, principalID string, conversationID string) (ConversationReplayResponse, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ConversationReplayResponse{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return ConversationReplayResponse{}, err
	}
	q := cubeboxsqlc.New(tx)
	row, err := q.GetConversation(ctx, cubeboxsqlc.GetConversationParams{
		Column1:        uuidToPGType(tenantID),
		ConversationID: strings.TrimSpace(conversationID),
		Column3:        uuidToPGType(principalID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ConversationReplayResponse{}, ErrConversationNotFound
		}
		return ConversationReplayResponse{}, err
	}
	events, err := q.ListConversationEvents(ctx, cubeboxsqlc.ListConversationEventsParams{
		Column1:        uuidToPGType(tenantID),
		ConversationID: strings.TrimSpace(conversationID),
	})
	if err != nil {
		return ConversationReplayResponse{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ConversationReplayResponse{}, err
	}

	return ConversationReplayResponse{
		Conversation: Conversation{
			ID:       row.ConversationID,
			Title:    row.Title,
			Status:   row.Status,
			Archived: row.Archived,
		},
		Events:       mapEvents(events),
		NextSequence: nextSequence(events),
	}, nil
}

func (s *Store) ListConversations(ctx context.Context, tenantID string, principalID string, limit int32) (ConversationListResponse, error) {
	if limit <= 0 {
		limit = 20
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ConversationListResponse{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return ConversationListResponse{}, err
	}
	rows, err := cubeboxsqlc.New(tx).ListConversations(ctx, cubeboxsqlc.ListConversationsParams{
		Column1: uuidToPGType(tenantID),
		Column2: uuidToPGType(principalID),
		Limit:   limit,
	})
	if err != nil {
		return ConversationListResponse{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ConversationListResponse{}, err
	}

	items := make([]ConversationListItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, ConversationListItem{
			ID:        row.ConversationID,
			Title:     row.Title,
			Status:    row.Status,
			Archived:  row.Archived,
			UpdatedAt: pgTime(row.UpdatedAt).Format(time.RFC3339),
		})
	}
	return ConversationListResponse{Items: items}, nil
}

func (s *Store) RenameConversation(ctx context.Context, tenantID string, principalID string, conversationID string, title string) (ConversationReplayResponse, error) {
	return s.updateConversationMetadata(ctx, tenantID, principalID, conversationID, conversationMetadataUpdate{
		title:     strings.TrimSpace(title),
		eventType: "conversation.renamed",
	})
}

func (s *Store) ArchiveConversation(ctx context.Context, tenantID string, principalID string, conversationID string, archived bool) (ConversationReplayResponse, error) {
	status := "active"
	eventType := "conversation.unarchived"
	if archived {
		status = "archived"
		eventType = "conversation.archived"
	}
	return s.updateConversationMetadata(ctx, tenantID, principalID, conversationID, conversationMetadataUpdate{
		archived:   &archived,
		status:     &status,
		eventType:  eventType,
		archivedAt: nullableTimeValue(archived),
	})
}

func (s *Store) AppendEvent(ctx context.Context, tenantID string, principalID string, conversationID string, event CanonicalEvent) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return err
	}
	q := cubeboxsqlc.New(tx)
	if _, err := q.GetConversation(ctx, cubeboxsqlc.GetConversationParams{
		Column1:        uuidToPGType(tenantID),
		ConversationID: strings.TrimSpace(conversationID),
		Column3:        uuidToPGType(principalID),
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrConversationNotFound
		}
		return err
	}
	now := time.Now().UTC()
	if err := appendEvent(ctx, q, tenantID, event, now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
UPDATE iam.cubebox_conversations
SET updated_at = $4
WHERE tenant_uuid = $1::uuid AND conversation_id = $2 AND principal_id = $3::uuid
`, tenantID, strings.TrimSpace(conversationID), principalID, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) CompactConversation(ctx context.Context, tenantID string, principalID string, conversationID string, canonicalContext CanonicalContext, reason string) (CompactConversationResponse, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CompactConversationResponse{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return CompactConversationResponse{}, err
	}
	q := cubeboxsqlc.New(tx)
	current, err := q.GetConversation(ctx, cubeboxsqlc.GetConversationParams{
		Column1:        uuidToPGType(tenantID),
		ConversationID: strings.TrimSpace(conversationID),
		Column3:        uuidToPGType(principalID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CompactConversationResponse{}, ErrConversationNotFound
		}
		return CompactConversationResponse{}, err
	}
	if _, err := tx.Exec(ctx, `
SELECT conversation_id
FROM iam.cubebox_conversations
WHERE tenant_uuid = $1::uuid AND conversation_id = $2 AND principal_id = $3::uuid
FOR UPDATE
`, tenantID, strings.TrimSpace(conversationID), principalID); err != nil {
		return CompactConversationResponse{}, err
	}
	rows, err := q.ListConversationEvents(ctx, cubeboxsqlc.ListConversationEventsParams{
		Column1:        uuidToPGType(tenantID),
		ConversationID: strings.TrimSpace(conversationID),
	})
	if err != nil {
		return CompactConversationResponse{}, err
	}
	next := nextSequence(rows)
	result := BuildPromptViewWithCompaction(mapEvents(rows), canonicalContext, "")
	if strings.TrimSpace(reason) != "" {
		result.Reason = strings.TrimSpace(reason)
	}

	response := CompactConversationResponse{
		Conversation: Conversation{
			ID:       current.ConversationID,
			Title:    current.Title,
			Status:   current.Status,
			Archived: current.Archived,
		},
		PromptView:   result.PromptView,
		NextSequence: next,
	}
	if !result.Compacted {
		if err := tx.Commit(ctx); err != nil {
			return CompactConversationResponse{}, err
		}
		return response, nil
	}

	now := time.Now().UTC()
	event := BuildCompactionEvent(conversationID, nil, next, now, result)
	if err := appendEvent(ctx, q, tenantID, event, now); err != nil {
		return CompactConversationResponse{}, err
	}
	if _, err := tx.Exec(ctx, `
UPDATE iam.cubebox_conversations
SET updated_at = $4
WHERE tenant_uuid = $1::uuid AND conversation_id = $2 AND principal_id = $3::uuid
`, tenantID, strings.TrimSpace(conversationID), principalID, now); err != nil {
		return CompactConversationResponse{}, err
	}
	response.Event = &event
	response.NextSequence = next + 1
	if err := tx.Commit(ctx); err != nil {
		return CompactConversationResponse{}, err
	}
	return response, nil
}

type conversationMetadataUpdate struct {
	title      string
	archived   *bool
	status     *string
	eventType  string
	archivedAt *time.Time
}

func (s *Store) updateConversationMetadata(ctx context.Context, tenantID string, principalID string, conversationID string, update conversationMetadataUpdate) (ConversationReplayResponse, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ConversationReplayResponse{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return ConversationReplayResponse{}, err
	}
	q := cubeboxsqlc.New(tx)
	current, err := q.GetConversation(ctx, cubeboxsqlc.GetConversationParams{
		Column1:        uuidToPGType(tenantID),
		ConversationID: strings.TrimSpace(conversationID),
		Column3:        uuidToPGType(principalID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ConversationReplayResponse{}, ErrConversationNotFound
		}
		return ConversationReplayResponse{}, err
	}
	events, err := q.ListConversationEvents(ctx, cubeboxsqlc.ListConversationEventsParams{
		Column1:        uuidToPGType(tenantID),
		ConversationID: strings.TrimSpace(conversationID),
	})
	if err != nil {
		return ConversationReplayResponse{}, err
	}
	next := nextSequence(events)
	now := time.Now().UTC()

	status := current.Status
	if update.status != nil {
		status = *update.status
	}
	if update.title != "" {
		row, err := q.UpdateConversationTitle(ctx, cubeboxsqlc.UpdateConversationTitleParams{
			Column1:        uuidToPGType(tenantID),
			ConversationID: strings.TrimSpace(conversationID),
			Column3:        uuidToPGType(principalID),
			Title:          update.title,
			UpdatedAt:      timestamptz(now),
		})
		if err != nil {
			return ConversationReplayResponse{}, err
		}
		current = row
	}
	targetArchived := current.Archived
	if update.archived != nil {
		targetArchived = *update.archived
	}
	if current.Archived != targetArchived || current.Status != status {
		row, err := q.UpdateConversationArchive(ctx, cubeboxsqlc.UpdateConversationArchiveParams{
			Column1:        uuidToPGType(tenantID),
			ConversationID: strings.TrimSpace(conversationID),
			Column3:        uuidToPGType(principalID),
			Status:         status,
			Archived:       targetArchived,
			ArchivedAt:     timestamptzPtr(update.archivedAt),
			UpdatedAt:      timestamptz(now),
		})
		if err != nil {
			return ConversationReplayResponse{}, err
		}
		current = row
	}

	payload := map[string]any{
		"title":    current.Title,
		"status":   current.Status,
		"archived": current.Archived,
	}
	event := CanonicalEvent{
		EventID:        "evt_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		ConversationID: current.ConversationID,
		TurnID:         nil,
		Sequence:       next,
		Type:           update.eventType,
		TS:             now.Format(time.RFC3339),
		Payload:        payload,
	}
	if err := appendEvent(ctx, q, tenantID, event, now); err != nil {
		return ConversationReplayResponse{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ConversationReplayResponse{}, err
	}

	allEvents := append(mapEvents(events), event)
	return ConversationReplayResponse{
		Conversation: Conversation{
			ID:       current.ConversationID,
			Title:    current.Title,
			Status:   current.Status,
			Archived: current.Archived,
		},
		Events:       allEvents,
		NextSequence: next + 1,
	}, nil
}

func appendEvent(ctx context.Context, q *cubeboxsqlc.Queries, tenantID string, event CanonicalEvent, now time.Time) error {
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return err
	}
	_, err = q.AppendConversationEvent(ctx, cubeboxsqlc.AppendConversationEventParams{
		Column1:        uuidToPGType(tenantID),
		ConversationID: event.ConversationID,
		EventID:        event.EventID,
		Sequence:       int32(event.Sequence),
		TurnID:         optionalString(event.TurnID),
		EventType:      event.Type,
		Payload:        payload,
		CreatedAt:      timestamptz(now),
	})
	return err
}

func mapEvents(rows []cubeboxsqlc.IamCubeboxConversationEvent) []CanonicalEvent {
	events := make([]CanonicalEvent, 0, len(rows))
	for _, row := range rows {
		payload := map[string]any{}
		if len(row.Payload) > 0 {
			_ = json.Unmarshal(row.Payload, &payload)
		}
		events = append(events, CanonicalEvent{
			EventID:        row.EventID,
			ConversationID: row.ConversationID,
			TurnID:         derefString(row.TurnID),
			Sequence:       int(row.Sequence),
			Type:           row.EventType,
			TS:             pgTime(row.CreatedAt).Format(time.RFC3339),
			Payload:        payload,
		})
	}
	return events
}

func nextSequence(rows []cubeboxsqlc.IamCubeboxConversationEvent) int {
	if len(rows) == 0 {
		return 1
	}
	return int(rows[len(rows)-1].Sequence) + 1
}

func mustParseUUID(input string) uuid.UUID {
	return uuid.MustParse(strings.TrimSpace(input))
}

func derefString(input *string) *string {
	if input == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*input)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func optionalString(input *string) *string {
	if input == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*input)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func nullableTime(input *time.Time) *time.Time {
	if input == nil {
		return nil
	}
	utc := input.UTC()
	return &utc
}

func nullableTimeValue(enabled bool) *time.Time {
	if !enabled {
		return nil
	}
	now := time.Now().UTC()
	return &now
}

func uuidToPGType(input string) pgtype.UUID {
	var value pgtype.UUID
	_ = value.Scan(strings.TrimSpace(input))
	return value
}

func timestamptz(input time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: input.UTC(), Valid: true}
}

func nullTimestamptz() pgtype.Timestamptz {
	return pgtype.Timestamptz{}
}

func timestamptzPtr(input *time.Time) pgtype.Timestamptz {
	if input == nil {
		return nullTimestamptz()
	}
	return timestamptz(*input)
}

func pgTime(input pgtype.Timestamptz) time.Time {
	if !input.Valid {
		return time.Time{}
	}
	return input.Time.UTC()
}
