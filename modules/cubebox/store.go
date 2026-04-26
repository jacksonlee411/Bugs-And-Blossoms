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

var ErrModelProviderNotFound = errors.New("CUBEBOX_MODEL_PROVIDER_NOT_FOUND")
var ErrActiveModelSelectionNotFound = errors.New("CUBEBOX_ACTIVE_MODEL_SELECTION_NOT_FOUND")
var ErrModelCredentialNotFound = errors.New("CUBEBOX_MODEL_CREDENTIAL_NOT_FOUND")
var ErrModelCapabilitySummaryInvalid = errors.New("CUBEBOX_MODEL_CAPABILITY_SUMMARY_INVALID")

type UpsertModelProviderInput struct {
	ProviderID   string
	ProviderType string
	DisplayName  string
	BaseURL      string
	Enabled      bool
}

type RotateModelCredentialInput struct {
	ProviderID   string
	SecretRef    string
	MaskedSecret string
}

type SelectActiveModelInput struct {
	ProviderID        string
	ModelSlug         string
	CapabilitySummary map[string]any
}

type ActiveModelRuntimeConfig struct {
	Selection  ActiveModelSelection
	Provider   ModelProvider
	Credential ModelCredential
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
	return s.AppendEvents(ctx, tenantID, principalID, conversationID, []CanonicalEvent{event})
}

func (s *Store) AppendEvents(ctx context.Context, tenantID string, principalID string, conversationID string, events []CanonicalEvent) error {
	if len(events) == 0 {
		return nil
	}
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
	for _, event := range events {
		if err := appendEvent(ctx, q, tenantID, event, now); err != nil {
			return err
		}
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

func (s *Store) PrepareConversationPromptView(ctx context.Context, tenantID string, principalID string, conversationID string, canonicalContext CanonicalContext, reason string) (PromptViewPreparationResponse, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PromptViewPreparationResponse{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return PromptViewPreparationResponse{}, err
	}
	q := cubeboxsqlc.New(tx)
	current, err := q.GetConversation(ctx, cubeboxsqlc.GetConversationParams{
		Column1:        uuidToPGType(tenantID),
		ConversationID: strings.TrimSpace(conversationID),
		Column3:        uuidToPGType(principalID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PromptViewPreparationResponse{}, ErrConversationNotFound
		}
		return PromptViewPreparationResponse{}, err
	}
	if _, err := tx.Exec(ctx, `
SELECT conversation_id
FROM iam.cubebox_conversations
WHERE tenant_uuid = $1::uuid AND conversation_id = $2 AND principal_id = $3::uuid
FOR UPDATE
`, tenantID, strings.TrimSpace(conversationID), principalID); err != nil {
		return PromptViewPreparationResponse{}, err
	}
	rows, err := q.ListConversationEvents(ctx, cubeboxsqlc.ListConversationEventsParams{
		Column1:        uuidToPGType(tenantID),
		ConversationID: strings.TrimSpace(conversationID),
	})
	if err != nil {
		return PromptViewPreparationResponse{}, err
	}
	next := nextSequence(rows)
	response := PromptViewPreparationResponse{
		Conversation: Conversation{
			ID:       current.ConversationID,
			Title:    current.Title,
			Status:   current.Status,
			Archived: current.Archived,
		},
		PromptView:   buildPromptViewForProvider(mapEvents(rows), canonicalContext, ""),
		NextSequence: next,
	}
	if err := tx.Commit(ctx); err != nil {
		return PromptViewPreparationResponse{}, err
	}
	return response, nil
}

func (s *Store) GetModelSettings(ctx context.Context, tenantID string) (ModelSettingsSnapshot, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ModelSettingsSnapshot{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return ModelSettingsSnapshot{}, err
	}

	q := cubeboxsqlc.New(tx)
	providerRows, err := q.ListModelProviders(ctx, uuidToPGType(tenantID))
	if err != nil {
		return ModelSettingsSnapshot{}, err
	}
	credentialRows, err := q.ListModelCredentials(ctx, uuidToPGType(tenantID))
	if err != nil {
		return ModelSettingsSnapshot{}, err
	}

	snapshot := ModelSettingsSnapshot{
		Providers:   make([]ModelProvider, 0, len(providerRows)),
		Credentials: make([]ModelCredential, 0, len(credentialRows)),
	}
	for _, row := range providerRows {
		provider := ModelProvider{
			ID:           row.ProviderID,
			ProviderType: row.ProviderType,
			DisplayName:  row.DisplayName,
			BaseURL:      row.BaseUrl,
			Enabled:      row.Enabled,
			UpdatedAt:    pgTime(row.UpdatedAt).Format(time.RFC3339),
		}
		if row.DisabledAt.Valid {
			provider.DisabledAt = pgTime(row.DisabledAt).Format(time.RFC3339)
		}
		snapshot.Providers = append(snapshot.Providers, provider)
	}
	for _, row := range credentialRows {
		credential := ModelCredential{
			ID:           row.CredentialID,
			ProviderID:   row.ProviderID,
			SecretRef:    row.SecretRef,
			MaskedSecret: row.MaskedSecret,
			Version:      int(row.Version),
			Active:       row.Active,
			CreatedAt:    pgTime(row.CreatedAt).Format(time.RFC3339),
		}
		if row.DisabledAt.Valid {
			credential.DisabledAt = pgTime(row.DisabledAt).Format(time.RFC3339)
		}
		snapshot.Credentials = append(snapshot.Credentials, credential)
	}

	selectionRow, err := q.GetActiveModelSelection(ctx, uuidToPGType(tenantID))
	if err == nil {
		capability := map[string]any{}
		if len(selectionRow.CapabilitySummary) > 0 {
			_ = json.Unmarshal(selectionRow.CapabilitySummary, &capability)
		}
		snapshot.Selection = &ActiveModelSelection{
			ProviderID:        selectionRow.ProviderID,
			ModelSlug:         selectionRow.ModelSlug,
			CapabilitySummary: capability,
			UpdatedAt:         pgTime(selectionRow.UpdatedAt).Format(time.RFC3339),
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return ModelSettingsSnapshot{}, err
	}

	if snapshot.Selection != nil {
		healthRow, err := q.GetLatestModelHealthCheckByProviderAndModel(ctx, cubeboxsqlc.GetLatestModelHealthCheckByProviderAndModelParams{
			Column1:    uuidToPGType(tenantID),
			ProviderID: snapshot.Selection.ProviderID,
			ModelSlug:  snapshot.Selection.ModelSlug,
		})
		if err == nil {
			snapshot.Health = mapHealthRow(healthRow)
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return ModelSettingsSnapshot{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return ModelSettingsSnapshot{}, err
	}
	return snapshot, nil
}

func (s *Store) GetActiveModelRuntimeConfig(ctx context.Context, tenantID string) (ActiveModelRuntimeConfig, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ActiveModelRuntimeConfig{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return ActiveModelRuntimeConfig{}, err
	}

	q := cubeboxsqlc.New(tx)
	selectionRow, err := q.GetActiveModelSelection(ctx, uuidToPGType(tenantID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ActiveModelRuntimeConfig{}, ErrActiveModelSelectionNotFound
		}
		return ActiveModelRuntimeConfig{}, err
	}

	providerRow, err := q.GetModelProvider(ctx, cubeboxsqlc.GetModelProviderParams{
		Column1:    uuidToPGType(tenantID),
		ProviderID: selectionRow.ProviderID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ActiveModelRuntimeConfig{}, ErrModelProviderNotFound
		}
		return ActiveModelRuntimeConfig{}, err
	}

	credentialRows, err := q.ListModelCredentialsByProvider(ctx, cubeboxsqlc.ListModelCredentialsByProviderParams{
		Column1:    uuidToPGType(tenantID),
		ProviderID: selectionRow.ProviderID,
	})
	if err != nil {
		return ActiveModelRuntimeConfig{}, err
	}
	if len(credentialRows) == 0 || !credentialRows[0].Active {
		return ActiveModelRuntimeConfig{}, ErrModelCredentialNotFound
	}

	capability := map[string]any{}
	if len(selectionRow.CapabilitySummary) > 0 {
		_ = json.Unmarshal(selectionRow.CapabilitySummary, &capability)
	}

	config := ActiveModelRuntimeConfig{
		Selection: ActiveModelSelection{
			ProviderID:        selectionRow.ProviderID,
			ModelSlug:         selectionRow.ModelSlug,
			CapabilitySummary: capability,
			UpdatedAt:         pgTime(selectionRow.UpdatedAt).Format(time.RFC3339),
		},
		Provider: ModelProvider{
			ID:           providerRow.ProviderID,
			ProviderType: providerRow.ProviderType,
			DisplayName:  providerRow.DisplayName,
			BaseURL:      providerRow.BaseUrl,
			Enabled:      providerRow.Enabled,
			UpdatedAt:    pgTime(providerRow.UpdatedAt).Format(time.RFC3339),
		},
		Credential: ModelCredential{
			ID:           credentialRows[0].CredentialID,
			ProviderID:   credentialRows[0].ProviderID,
			SecretRef:    credentialRows[0].SecretRef,
			MaskedSecret: credentialRows[0].MaskedSecret,
			Version:      int(credentialRows[0].Version),
			Active:       credentialRows[0].Active,
			CreatedAt:    pgTime(credentialRows[0].CreatedAt).Format(time.RFC3339),
		},
	}
	if providerRow.DisabledAt.Valid {
		config.Provider.DisabledAt = pgTime(providerRow.DisabledAt).Format(time.RFC3339)
	}
	if credentialRows[0].DisabledAt.Valid {
		config.Credential.DisabledAt = pgTime(credentialRows[0].DisabledAt).Format(time.RFC3339)
	}

	if err := tx.Commit(ctx); err != nil {
		return ActiveModelRuntimeConfig{}, err
	}
	return config, nil
}

func (s *Store) UpsertModelProvider(ctx context.Context, tenantID string, principalID string, input UpsertModelProviderInput) (ModelProvider, error) {
	now := time.Now().UTC()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ModelProvider{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return ModelProvider{}, err
	}

	row, err := cubeboxsqlc.New(tx).UpsertModelProvider(ctx, cubeboxsqlc.UpsertModelProviderParams{
		Column1:      uuidToPGType(tenantID),
		ProviderID:   strings.TrimSpace(input.ProviderID),
		ProviderType: strings.TrimSpace(input.ProviderType),
		DisplayName:  strings.TrimSpace(input.DisplayName),
		BaseUrl:      strings.TrimSpace(input.BaseURL),
		Enabled:      input.Enabled,
		Column7:      uuidToPGType(principalID),
		Column8:      uuidToPGType(principalID),
		CreatedAt:    timestamptz(now),
		UpdatedAt:    timestamptz(now),
		DisabledAt:   timestamptzPtr(nullableTimeValue(!input.Enabled)),
	})
	if err != nil {
		return ModelProvider{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ModelProvider{}, err
	}
	provider := ModelProvider{
		ID:           row.ProviderID,
		ProviderType: row.ProviderType,
		DisplayName:  row.DisplayName,
		BaseURL:      row.BaseUrl,
		Enabled:      row.Enabled,
		UpdatedAt:    pgTime(row.UpdatedAt).Format(time.RFC3339),
	}
	if row.DisabledAt.Valid {
		provider.DisabledAt = pgTime(row.DisabledAt).Format(time.RFC3339)
	}
	return provider, nil
}

func (s *Store) RotateModelCredential(ctx context.Context, tenantID string, principalID string, input RotateModelCredentialInput) (ModelCredential, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ModelCredential{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return ModelCredential{}, err
	}
	q := cubeboxsqlc.New(tx)
	providerID := strings.TrimSpace(input.ProviderID)
	if _, err := q.GetModelProvider(ctx, cubeboxsqlc.GetModelProviderParams{
		Column1:    uuidToPGType(tenantID),
		ProviderID: providerID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ModelCredential{}, ErrModelProviderNotFound
		}
		return ModelCredential{}, err
	}

	current, err := q.ListModelCredentialsByProvider(ctx, cubeboxsqlc.ListModelCredentialsByProviderParams{
		Column1:    uuidToPGType(tenantID),
		ProviderID: providerID,
	})
	if err != nil {
		return ModelCredential{}, err
	}
	nextVersion := int32(1)
	if len(current) > 0 {
		nextVersion = current[0].Version + 1
	}
	now := time.Now().UTC()
	if err := q.DeactivateProviderCredentials(ctx, cubeboxsqlc.DeactivateProviderCredentialsParams{
		Column1:    uuidToPGType(tenantID),
		ProviderID: providerID,
		DisabledAt: timestamptz(now),
	}); err != nil {
		return ModelCredential{}, err
	}
	row, err := q.InsertModelCredential(ctx, cubeboxsqlc.InsertModelCredentialParams{
		Column1:      uuidToPGType(tenantID),
		CredentialID: "cred_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		ProviderID:   providerID,
		SecretRef:    strings.TrimSpace(input.SecretRef),
		MaskedSecret: strings.TrimSpace(input.MaskedSecret),
		Version:      nextVersion,
		Active:       true,
		Column8:      uuidToPGType(principalID),
		CreatedAt:    timestamptz(now),
		DisabledAt:   nullTimestamptz(),
	})
	if err != nil {
		return ModelCredential{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ModelCredential{}, err
	}
	return ModelCredential{
		ID:           row.CredentialID,
		ProviderID:   row.ProviderID,
		SecretRef:    row.SecretRef,
		MaskedSecret: row.MaskedSecret,
		Version:      int(row.Version),
		Active:       row.Active,
		CreatedAt:    pgTime(row.CreatedAt).Format(time.RFC3339),
	}, nil
}

func (s *Store) DeactivateCredential(ctx context.Context, tenantID string, credentialID string) (ModelCredential, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ModelCredential{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return ModelCredential{}, err
	}
	row, err := cubeboxsqlc.New(tx).DeactivateCredential(ctx, cubeboxsqlc.DeactivateCredentialParams{
		Column1:      uuidToPGType(tenantID),
		CredentialID: strings.TrimSpace(credentialID),
		DisabledAt:   timestamptz(time.Now().UTC()),
	})
	if err != nil {
		return ModelCredential{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ModelCredential{}, err
	}
	credential := ModelCredential{
		ID:           row.CredentialID,
		ProviderID:   row.ProviderID,
		SecretRef:    row.SecretRef,
		MaskedSecret: row.MaskedSecret,
		Version:      int(row.Version),
		Active:       row.Active,
		CreatedAt:    pgTime(row.CreatedAt).Format(time.RFC3339),
	}
	if row.DisabledAt.Valid {
		credential.DisabledAt = pgTime(row.DisabledAt).Format(time.RFC3339)
	}
	return credential, nil
}

func (s *Store) SelectActiveModel(ctx context.Context, tenantID string, principalID string, input SelectActiveModelInput) (ActiveModelSelection, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ActiveModelSelection{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return ActiveModelSelection{}, err
	}
	q := cubeboxsqlc.New(tx)
	providerID := strings.TrimSpace(input.ProviderID)
	if _, err := q.GetModelProvider(ctx, cubeboxsqlc.GetModelProviderParams{
		Column1:    uuidToPGType(tenantID),
		ProviderID: providerID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ActiveModelSelection{}, ErrModelProviderNotFound
		}
		return ActiveModelSelection{}, err
	}
	if input.CapabilitySummary == nil {
		input.CapabilitySummary = map[string]any{}
	}
	payload, err := json.Marshal(input.CapabilitySummary)
	if err != nil {
		return ActiveModelSelection{}, err
	}
	now := time.Now().UTC()
	row, err := q.UpsertModelSelection(ctx, cubeboxsqlc.UpsertModelSelectionParams{
		Column1:           uuidToPGType(tenantID),
		ProviderID:        providerID,
		ModelSlug:         strings.TrimSpace(input.ModelSlug),
		CapabilitySummary: payload,
		Column5:           uuidToPGType(principalID),
		CreatedAt:         timestamptz(now),
		UpdatedAt:         timestamptz(now),
	})
	if err != nil {
		return ActiveModelSelection{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ActiveModelSelection{}, err
	}
	capability := map[string]any{}
	_ = json.Unmarshal(row.CapabilitySummary, &capability)
	return ActiveModelSelection{
		ProviderID:        row.ProviderID,
		ModelSlug:         row.ModelSlug,
		CapabilitySummary: capability,
		UpdatedAt:         pgTime(row.UpdatedAt).Format(time.RFC3339),
	}, nil
}

func (s *Store) VerifyActiveModel(ctx context.Context, tenantID string, principalID string) (ModelHealth, error) {
	service := NewModelVerificationService(s, s, NewOpenAICompatibleAdapter(nil), EnvSecretResolver{})
	return service.VerifyActiveModel(ctx, tenantID, principalID)
}

func (s *Store) RecordModelHealthCheck(ctx context.Context, tenantID string, principalID string, input ModelHealthWriteInput) (ModelHealth, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ModelHealth{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return ModelHealth{}, err
	}
	q := cubeboxsqlc.New(tx)
	now := time.Now().UTC()
	var latencyValue int32
	var hasLatency bool
	if input.LatencyMS != nil {
		latencyValue = int32(*input.LatencyMS)
		hasLatency = true
	}
	var summary *string
	if trimmed := strings.TrimSpace(input.ErrorSummary); trimmed != "" {
		summary = &trimmed
	}

	row, err := q.InsertModelHealthCheck(ctx, cubeboxsqlc.InsertModelHealthCheckParams{
		Column1:       uuidToPGType(tenantID),
		HealthCheckID: "health_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		ProviderID:    strings.TrimSpace(input.ProviderID),
		ModelSlug:     strings.TrimSpace(input.ModelSlug),
		Status:        strings.TrimSpace(input.Status),
		LatencyMs: func() *int32 {
			if !hasLatency {
				return nil
			}
			return int32PtrOrNil(input.Status, latencyValue)
		}(),
		ErrorSummary: summary,
		Column8:      uuidToPGType(principalID),
		ValidatedAt:  timestamptz(now),
	})
	if err != nil {
		return ModelHealth{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ModelHealth{}, err
	}
	health := mapHealthRow(row)
	if health == nil {
		return ModelHealth{}, errors.New("cubebox: health mapping failed")
	}
	return *health, nil
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

func int32PtrOrNil(status string, value int32) *int32 {
	if strings.TrimSpace(status) == "failed" {
		return nil
	}
	return &value
}

func stringPtr(input string) *string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil
	}
	return &trimmed
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

func mapHealthRow(row cubeboxsqlc.IamCubeboxModelHealthCheck) *ModelHealth {
	health := &ModelHealth{
		ID:          row.HealthCheckID,
		ProviderID:  row.ProviderID,
		ModelSlug:   row.ModelSlug,
		Status:      row.Status,
		ValidatedAt: pgTime(row.ValidatedAt).Format(time.RFC3339),
	}
	if row.LatencyMs != nil {
		value := int(*row.LatencyMs)
		health.LatencyMS = &value
	}
	if row.ErrorSummary != nil {
		health.ErrorSummary = strings.TrimSpace(*row.ErrorSummary)
	}
	return health
}
