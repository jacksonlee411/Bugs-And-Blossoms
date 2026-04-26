package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
)

type cubeboxCreateConversationRequest struct{}

type cubeboxConversationPatchRequest struct {
	Title    *string `json:"title"`
	Archived *bool   `json:"archived"`
}

type cubeboxStreamTurnRequest struct {
	ConversationID string `json:"conversation_id"`
	Prompt         string `json:"prompt"`
	NextSequence   int    `json:"next_sequence"`
}

type cubeboxInterruptRequest struct {
	Reason string `json:"reason"`
}

type cubeboxInterruptResponse struct {
	TurnID      string `json:"turn_id"`
	Interrupted bool   `json:"interrupted"`
}

type cubeboxConversationStore interface {
	CreateConversation(ctx context.Context, tenantID string, principalID string) (cubebox.ConversationReplayResponse, error)
	GetConversation(ctx context.Context, tenantID string, principalID string, conversationID string) (cubebox.ConversationReplayResponse, error)
	ListConversations(ctx context.Context, tenantID string, principalID string, limit int32) (cubebox.ConversationListResponse, error)
	RenameConversation(ctx context.Context, tenantID string, principalID string, conversationID string, title string) (cubebox.ConversationReplayResponse, error)
	ArchiveConversation(ctx context.Context, tenantID string, principalID string, conversationID string, archived bool) (cubebox.ConversationReplayResponse, error)
	PrepareConversationPromptView(ctx context.Context, tenantID string, principalID string, conversationID string, canonicalContext cubebox.CanonicalContext, reason string) (cubebox.PromptViewPreparationResponse, error)
	AppendEvent(ctx context.Context, tenantID string, principalID string, conversationID string, event cubebox.CanonicalEvent) error
	AppendEvents(ctx context.Context, tenantID string, principalID string, conversationID string, events []cubebox.CanonicalEvent) error
	GetModelSettings(ctx context.Context, tenantID string) (cubebox.ModelSettingsSnapshot, error)
	UpsertModelProvider(ctx context.Context, tenantID string, principalID string, input cubebox.UpsertModelProviderInput) (cubebox.ModelProvider, error)
	RotateModelCredential(ctx context.Context, tenantID string, principalID string, input cubebox.RotateModelCredentialInput) (cubebox.ModelCredential, error)
	DeactivateCredential(ctx context.Context, tenantID string, credentialID string) (cubebox.ModelCredential, error)
	SelectActiveModel(ctx context.Context, tenantID string, principalID string, input cubebox.SelectActiveModelInput) (cubebox.ActiveModelSelection, error)
	VerifyActiveModel(ctx context.Context, tenantID string, principalID string) (cubebox.ModelHealth, error)
	GetActiveModelRuntimeConfig(ctx context.Context, tenantID string) (cubebox.ActiveModelRuntimeConfig, error)
}

type cubeboxTurnStore interface {
	cubeboxConversationStore
	cubebox.StreamAppendStore
}

type cubeboxProviderUpsertRequest struct {
	ProviderID   string `json:"provider_id"`
	ProviderType string `json:"provider_type"`
	DisplayName  string `json:"display_name"`
	BaseURL      string `json:"base_url"`
	Enabled      bool   `json:"enabled"`
}

type cubeboxCredentialRotateRequest struct {
	ProviderID   string `json:"provider_id"`
	SecretRef    string `json:"secret_ref"`
	MaskedSecret string `json:"masked_secret"`
}

type cubeboxSelectionRequest struct {
	ProviderID        string         `json:"provider_id"`
	ModelSlug         string         `json:"model_slug"`
	CapabilitySummary map[string]any `json:"capability_summary"`
}

type cubeboxCapabilitiesResponse struct {
	Conversation cubeboxConversationCapabilities `json:"conversation"`
	Settings     cubeboxSettingsCapabilities     `json:"settings"`
}

type cubeboxConversationCapabilities struct {
	Read bool `json:"read"`
	Use  bool `json:"use"`
}

type cubeboxSettingsCapabilities struct {
	Read       bool `json:"read"`
	Verify     bool `json:"verify"`
	Select     bool `json:"select"`
	Update     bool `json:"update"`
	Rotate     bool `json:"rotate"`
	Deactivate bool `json:"deactivate"`
}

type cubeboxCapabilityAuthorizer interface {
	Authorize(subject string, domain string, object string, action string) (allowed bool, enforced bool, err error)
}

func handleCubeBoxConversationsAPI(w http.ResponseWriter, r *http.Request, store cubeboxConversationStore) {
	switch r.Method {
	case http.MethodPost:
		handleCubeBoxCreateConversationAPI(w, r, store)
	case http.MethodGet:
		handleCubeBoxListConversationsAPI(w, r, store)
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func handleCubeBoxCapabilitiesAPI(w http.ResponseWriter, r *http.Request, authorizer cubeboxCapabilityAuthorizer) {
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	tenant, principal, ok := cubeboxRequestActor(w, r)
	if !ok {
		return
	}
	if authorizer == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_error", "authz error")
		return
	}
	subject := authz.SubjectFromRoleSlug(principal.RoleSlug)
	domain := authz.DomainFromTenantID(tenant.ID)
	can := func(object string, action string) (bool, bool) {
		allowed, enforced, err := authorizer.Authorize(subject, domain, object, action)
		if err != nil {
			return false, false
		}
		return !enforced || allowed, true
	}
	conversationRead, ok := can(authz.ObjectCubeBoxConversations, authz.ActionRead)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_error", "authz error")
		return
	}
	conversationUse, ok := can(authz.ObjectCubeBoxConversations, authz.ActionUse)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_error", "authz error")
		return
	}
	settingsRead, ok := can(authz.ObjectCubeBoxModelCredential, authz.ActionRead)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_error", "authz error")
		return
	}
	settingsVerify, ok := can(authz.ObjectCubeBoxModelSelection, authz.ActionVerify)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_error", "authz error")
		return
	}
	settingsSelect, ok := can(authz.ObjectCubeBoxModelSelection, authz.ActionSelect)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_error", "authz error")
		return
	}
	settingsUpdate, ok := can(authz.ObjectCubeBoxModelProvider, authz.ActionUpdate)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_error", "authz error")
		return
	}
	settingsRotate, ok := can(authz.ObjectCubeBoxModelCredential, authz.ActionRotate)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_error", "authz error")
		return
	}
	settingsDeactivate, ok := can(authz.ObjectCubeBoxModelCredential, authz.ActionDeactivate)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_error", "authz error")
		return
	}

	writeJSON(w, http.StatusOK, cubeboxCapabilitiesResponse{
		Conversation: cubeboxConversationCapabilities{
			Read: conversationRead,
			Use:  conversationUse,
		},
		Settings: cubeboxSettingsCapabilities{
			Read:       settingsRead,
			Verify:     settingsVerify,
			Select:     settingsSelect,
			Update:     settingsUpdate,
			Rotate:     settingsRotate,
			Deactivate: settingsDeactivate,
		},
	})
}

func handleCubeBoxCreateConversationAPI(w http.ResponseWriter, r *http.Request, store cubeboxConversationStore) {
	tenant, principal, ok := cubeboxRequestActor(w, r)
	if !ok {
		return
	}

	var req cubeboxCreateConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "invalid_json", "invalid json")
		return
	}

	payload, err := store.CreateConversation(r.Context(), tenant.ID, principal.ID)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "cubebox_conversation_create_failed", "create conversation failed")
		return
	}
	writeJSON(w, http.StatusCreated, payload)
}

func handleCubeBoxListConversationsAPI(w http.ResponseWriter, r *http.Request, store cubeboxConversationStore) {
	tenant, principal, ok := cubeboxRequestActor(w, r)
	if !ok {
		return
	}
	limit := int32(20)
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 100 {
			limit = int32(parsed)
		}
	}
	payload, err := store.ListConversations(r.Context(), tenant.ID, principal.ID, limit)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "cubebox_conversation_list_failed", "list conversations failed")
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

func handleCubeBoxConversationAPI(w http.ResponseWriter, r *http.Request, store cubeboxConversationStore) {
	switch r.Method {
	case http.MethodGet:
		handleCubeBoxLoadConversationAPI(w, r, store)
	case http.MethodPatch:
		handleCubeBoxPatchConversationAPI(w, r, store)
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func handleCubeBoxLoadConversationAPI(w http.ResponseWriter, r *http.Request, store cubeboxConversationStore) {
	conversationID := conversationIDFromPath(r.URL.Path)
	if conversationID == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "conversation_id_required", "conversation id required")
		return
	}

	tenant, principal, ok := cubeboxRequestActor(w, r)
	if !ok {
		return
	}
	payload, err := store.GetConversation(r.Context(), tenant.ID, principal.ID, conversationID)
	if err != nil {
		if errors.Is(err, cubebox.ErrConversationNotFound) {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "cubebox_conversation_not_found", "conversation not found")
			return
		}
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "cubebox_conversation_read_failed", "read conversation failed")
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

func handleCubeBoxPatchConversationAPI(w http.ResponseWriter, r *http.Request, store cubeboxConversationStore) {
	conversationID := conversationIDFromPath(r.URL.Path)
	if conversationID == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "conversation_id_required", "conversation id required")
		return
	}
	tenant, principal, ok := cubeboxRequestActor(w, r)
	if !ok {
		return
	}
	var req cubeboxConversationPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "invalid_json", "invalid json")
		return
	}

	var (
		payload cubebox.ConversationReplayResponse
		err     error
	)
	if req.Title != nil {
		payload, err = store.RenameConversation(r.Context(), tenant.ID, principal.ID, conversationID, *req.Title)
	} else if req.Archived != nil {
		payload, err = store.ArchiveConversation(r.Context(), tenant.ID, principal.ID, conversationID, *req.Archived)
	} else {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "invalid_form", "title or archived required")
		return
	}
	if err != nil {
		if errors.Is(err, cubebox.ErrConversationNotFound) {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "cubebox_conversation_not_found", "conversation not found")
			return
		}
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "cubebox_conversation_update_failed", "update conversation failed")
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

func handleCubeBoxStreamTurnAPI(w http.ResponseWriter, r *http.Request, runtime *cubebox.Runtime, store cubeboxTurnStore, gateway *cubebox.GatewayService, queryFlow *cubeboxQueryFlow) {
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	tenant, principal, ok := cubeboxRequestActor(w, r)
	if !ok {
		return
	}

	var req cubeboxStreamTurnRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "invalid_json", "invalid json")
		return
	}
	req.ConversationID = strings.TrimSpace(req.ConversationID)
	if req.ConversationID == "" || strings.TrimSpace(req.Prompt) == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "invalid_form", "conversation and prompt required")
		return
	}
	if req.NextSequence <= 0 {
		req.NextSequence = 1
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "stream_not_supported", "stream not supported")
		return
	}

	if gateway == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "cubebox_turn_stream_failed", "stream unavailable")
		return
	}

	streamRequest := cubebox.GatewayStreamRequest{
		TenantID:       tenant.ID,
		PrincipalID:    principal.ID,
		ConversationID: req.ConversationID,
		Prompt:         req.Prompt,
		NextSequence:   req.NextSequence,
	}
	sink := cubeboxSSEEventSink{
		w:       w,
		flusher: flusher,
	}
	if queryFlow != nil && queryFlow.TryHandle(r.Context(), streamRequest, sink) {
		return
	}
	gateway.StreamTurn(r.Context(), streamRequest, store, sink)
}

type cubeboxSSEEventSink struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func (s cubeboxSSEEventSink) Write(event cubebox.CanonicalEvent) bool {
	b, err := json.Marshal(event)
	if err != nil {
		return false
	}
	if _, err := s.w.Write([]byte("data: ")); err != nil {
		return false
	}
	if _, err := s.w.Write(b); err != nil {
		return false
	}
	if _, err := s.w.Write([]byte("\n\n")); err != nil {
		return false
	}
	s.flusher.Flush()
	return true
}

func (s cubeboxSSEEventSink) WriteFallback(event cubebox.CanonicalEvent) {
	_ = s.Write(event)
}

func handleCubeBoxInterruptTurnAPI(w http.ResponseWriter, r *http.Request, runtime *cubebox.Runtime) {
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req cubeboxInterruptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "invalid_json", "invalid json")
		return
	}

	turnID := interruptTurnIDFromPath(r.URL.Path)
	if turnID == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "turn_id_required", "turn id required")
		return
	}
	tenant, principal, ok := cubeboxRequestActor(w, r)
	if !ok {
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		req.Reason = "user_requested"
	}
	conversationID := strings.TrimSpace(r.URL.Query().Get("conversation_id"))
	if conversationID == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "conversation_id_required", "conversation id required")
		return
	}

	writeJSON(w, http.StatusOK, cubeboxInterruptResponse{
		TurnID: turnID,
		Interrupted: runtime.InterruptTurnForOwner(turnID, cubebox.TurnOwner{
			TenantID:       tenant.ID,
			PrincipalID:    principal.ID,
			ConversationID: conversationID,
		}),
	})
}

func handleCubeBoxSettingsAPI(w http.ResponseWriter, r *http.Request, store cubeboxConversationStore) {
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	tenant, _, ok := cubeboxRequestActor(w, r)
	if !ok {
		return
	}
	payload, err := store.GetModelSettings(r.Context(), tenant.ID)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "ai_model_config_invalid", "settings load failed")
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

func handleCubeBoxSettingsProvidersAPI(w http.ResponseWriter, r *http.Request, store cubeboxConversationStore) {
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	tenant, principal, ok := cubeboxRequestActor(w, r)
	if !ok {
		return
	}
	var req cubeboxProviderUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "invalid_json", "invalid json")
		return
	}
	if strings.TrimSpace(req.ProviderID) == "" || strings.TrimSpace(req.ProviderType) == "" || strings.TrimSpace(req.DisplayName) == "" || strings.TrimSpace(req.BaseURL) == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "ai_model_config_invalid", "provider config invalid")
		return
	}
	payload, err := store.UpsertModelProvider(r.Context(), tenant.ID, principal.ID, cubebox.UpsertModelProviderInput{
		ProviderID:   req.ProviderID,
		ProviderType: req.ProviderType,
		DisplayName:  req.DisplayName,
		BaseURL:      req.BaseURL,
		Enabled:      req.Enabled,
	})
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "ai_model_config_invalid", "provider save failed")
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

func handleCubeBoxSettingsCredentialsAPI(w http.ResponseWriter, r *http.Request, store cubeboxConversationStore) {
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	tenant, principal, ok := cubeboxRequestActor(w, r)
	if !ok {
		return
	}
	var req cubeboxCredentialRotateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "invalid_json", "invalid json")
		return
	}
	if strings.TrimSpace(req.ProviderID) == "" || strings.TrimSpace(req.SecretRef) == "" || strings.TrimSpace(req.MaskedSecret) == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "ai_model_secret_missing", "credential missing")
		return
	}
	payload, err := store.RotateModelCredential(r.Context(), tenant.ID, principal.ID, cubebox.RotateModelCredentialInput{
		ProviderID:   req.ProviderID,
		SecretRef:    req.SecretRef,
		MaskedSecret: req.MaskedSecret,
	})
	if err != nil {
		if errors.Is(err, cubebox.ErrModelProviderNotFound) {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "ai_model_provider_unavailable", "provider unavailable")
			return
		}
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "ai_model_config_invalid", "credential save failed")
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

func handleCubeBoxSettingsCredentialDeactivateAPI(w http.ResponseWriter, r *http.Request, store cubeboxConversationStore) {
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	tenant, _, ok := cubeboxRequestActor(w, r)
	if !ok {
		return
	}
	credentialID := credentialIDFromDeactivatePath(r.URL.Path)
	if credentialID == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "invalid_form", "credential id required")
		return
	}
	payload, err := store.DeactivateCredential(r.Context(), tenant.ID, credentialID)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "ai_model_config_invalid", "credential deactivate failed")
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

func handleCubeBoxSettingsSelectionAPI(w http.ResponseWriter, r *http.Request, store cubeboxConversationStore) {
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	tenant, principal, ok := cubeboxRequestActor(w, r)
	if !ok {
		return
	}
	var req cubeboxSelectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "invalid_json", "invalid json")
		return
	}
	if strings.TrimSpace(req.ProviderID) == "" || strings.TrimSpace(req.ModelSlug) == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "ai_model_config_invalid", "selection invalid")
		return
	}
	if req.CapabilitySummary == nil {
		req.CapabilitySummary = map[string]any{}
	}
	payload, err := store.SelectActiveModel(r.Context(), tenant.ID, principal.ID, cubebox.SelectActiveModelInput{
		ProviderID:        req.ProviderID,
		ModelSlug:         req.ModelSlug,
		CapabilitySummary: req.CapabilitySummary,
	})
	if err != nil {
		if errors.Is(err, cubebox.ErrModelProviderNotFound) {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "ai_model_provider_unavailable", "provider unavailable")
			return
		}
		if errors.Is(err, cubebox.ErrModelCapabilitySummaryInvalid) {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "ai_model_config_invalid", "selection invalid")
			return
		}
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "ai_model_config_invalid", "selection save failed")
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

func handleCubeBoxSettingsVerifyAPI(w http.ResponseWriter, r *http.Request, store cubeboxConversationStore) {
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	tenant, principal, ok := cubeboxRequestActor(w, r)
	if !ok {
		return
	}
	payload, err := store.VerifyActiveModel(r.Context(), tenant.ID, principal.ID)
	if err != nil {
		if errors.Is(err, cubebox.ErrModelProviderNotFound) {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "ai_model_provider_unavailable", "provider unavailable")
			return
		}
		if errors.Is(err, cubebox.ErrModelCredentialNotFound) || errors.Is(err, cubebox.ErrSecretMissing) || errors.Is(err, cubebox.ErrSecretRefInvalid) {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "ai_model_secret_missing", "secret missing")
			return
		}
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "ai_model_config_invalid", "verify failed")
		return
	}
	status := http.StatusOK
	if payload.Status == "failed" {
		status = http.StatusUnprocessableEntity
	}
	writeJSON(w, status, payload)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func conversationIDFromPath(path string) string {
	const prefix = "/internal/cubebox/conversations/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(path, prefix))
}

func interruptTurnIDFromPath(path string) string {
	const prefix = "/internal/cubebox/turns/"
	const suffix = ":interrupt"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return ""
	}
	trimmed := strings.TrimPrefix(path, prefix)
	trimmed = strings.TrimSuffix(trimmed, suffix)
	return strings.TrimSpace(trimmed)
}

func credentialIDFromDeactivatePath(path string) string {
	const prefix = "/internal/cubebox/settings/credentials/"
	const suffix = ":deactivate"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return ""
	}
	trimmed := strings.TrimPrefix(path, prefix)
	trimmed = strings.TrimSuffix(trimmed, suffix)
	return strings.TrimSpace(trimmed)
}

func cubeboxRequestActor(w http.ResponseWriter, r *http.Request) (Tenant, Principal, bool) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return Tenant{}, Principal{}, false
	}
	principal, ok := currentPrincipal(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnauthorized, "principal_missing", "principal missing")
		return Tenant{}, Principal{}, false
	}
	return tenant, principal, true
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
