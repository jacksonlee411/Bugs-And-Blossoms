package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox"
)

type cubeboxCreateConversationRequest struct{}

type cubeboxConversationPatchRequest struct {
	Title    *string `json:"title"`
	Archived *bool   `json:"archived"`
}

type cubeboxCompactConversationRequest struct {
	Reason string `json:"reason"`
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
	CompactConversation(ctx context.Context, tenantID string, principalID string, conversationID string, canonicalContext cubebox.CanonicalContext, reason string) (cubebox.CompactConversationResponse, error)
	AppendEvent(ctx context.Context, tenantID string, principalID string, conversationID string, event cubebox.CanonicalEvent) error
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

func handleCubeBoxCompactConversationAPI(w http.ResponseWriter, r *http.Request, store cubeboxConversationStore) {
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	conversationID := conversationIDFromCompactPath(r.URL.Path)
	if conversationID == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "conversation_id_required", "conversation id required")
		return
	}
	tenant, principal, ok := cubeboxRequestActor(w, r)
	if !ok {
		return
	}

	var req cubeboxCompactConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "invalid_json", "invalid json")
		return
	}
	payload, err := store.CompactConversation(r.Context(), tenant.ID, principal.ID, conversationID, cubebox.CanonicalContext{
		TenantID:       tenant.ID,
		PrincipalID:    principal.ID,
		Language:       "zh",
		Page:           "/app/cubebox",
		Permissions:    []string{"cubebox.conversations:admin"},
		BusinessObject: "conversation",
		Model:          "deterministic-runtime",
	}, strings.TrimSpace(req.Reason))
	if err != nil {
		if errors.Is(err, cubebox.ErrConversationNotFound) {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "cubebox_conversation_not_found", "conversation not found")
			return
		}
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "cubebox_conversation_update_failed", "compact conversation failed")
		return
	}
	writeJSON(w, http.StatusOK, payload)
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

func handleCubeBoxStreamTurnAPI(w http.ResponseWriter, r *http.Request, runtime *cubebox.Runtime, store cubeboxConversationStore) {
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	tenant, principal, ok := cubeboxRequestActor(w, r)
	if !ok {
		return
	}

	var req cubeboxStreamTurnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "invalid_json", "invalid json")
		return
	}
	req.ConversationID = strings.TrimSpace(req.ConversationID)
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.ConversationID == "" || req.Prompt == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "invalid_form", "conversation and prompt required")
		return
	}
	if req.NextSequence <= 0 {
		req.NextSequence = 1
	}

	if req.NextSequence > 1 {
		compactPayload, err := store.CompactConversation(r.Context(), tenant.ID, principal.ID, req.ConversationID, cubebox.CanonicalContext{
			TenantID:       tenant.ID,
			PrincipalID:    principal.ID,
			Language:       "zh",
			Page:           "/app/cubebox",
			Permissions:    []string{"cubebox.conversations:admin"},
			BusinessObject: "conversation",
			Model:          "deterministic-runtime",
		}, "pre_turn_auto")
		if err != nil && !errors.Is(err, cubebox.ErrConversationNotFound) {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "cubebox_conversation_update_failed", "compact conversation failed")
			return
		}
		req.NextSequence = max(req.NextSequence, compactPayload.NextSequence)
	}

	turn := runtime.StartTurn(cubebox.TurnOwner{
		TenantID:       tenant.ID,
		PrincipalID:    principal.ID,
		ConversationID: req.ConversationID,
	}, req.Prompt)
	defer runtime.FinishTurn(turn.TurnID)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "stream_not_supported", "stream not supported")
		return
	}

	sequence := req.NextSequence
	writeEvent := func(eventType string, turnID *string, payload map[string]any) bool {
		event := cubebox.CanonicalEvent{
			EventID:        "evt_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
			ConversationID: req.ConversationID,
			TurnID:         turnID,
			Sequence:       sequence,
			Type:           eventType,
			TS:             time.Now().UTC().Format(time.RFC3339),
			Payload:        payload,
		}
		sequence += 1
		if err := store.AppendEvent(r.Context(), tenant.ID, principal.ID, req.ConversationID, event); err != nil {
			return false
		}
		b, err := json.Marshal(event)
		if err != nil {
			return false
		}
		if _, err := w.Write([]byte("data: ")); err != nil {
			return false
		}
		if _, err := w.Write(b); err != nil {
			return false
		}
		if _, err := w.Write([]byte("\n\n")); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	writeStreamFailure := func(message string) {
		fallback := cubebox.CanonicalEvent{
			EventID:        "evt_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
			ConversationID: req.ConversationID,
			TurnID:         &turn.TurnID,
			Sequence:       sequence,
			Type:           "turn.error",
			TS:             time.Now().UTC().Format(time.RFC3339),
			Payload: map[string]any{
				"code":      "event_log_write_failed",
				"message":   message,
				"retryable": false,
			},
		}
		if b, err := json.Marshal(fallback); err == nil {
			_, _ = w.Write([]byte("data: "))
			_, _ = w.Write(b)
			_, _ = w.Write([]byte("\n\n"))
			flusher.Flush()
		}
	}
	writeEventOrFail := func(eventType string, turnID *string, payload map[string]any, failureMessage string) bool {
		if writeEvent(eventType, turnID, payload) {
			return true
		}
		writeStreamFailure(failureMessage)
		return false
	}

	turnID := turn.TurnID
	if !writeEventOrFail("turn.started", &turnID, map[string]any{"user_message_id": turn.UserMessageID}, "会话事件落库失败，当前响应已终止。") {
		return
	}
	if !writeEventOrFail("turn.user_message.accepted", &turnID, map[string]any{"message_id": turn.UserMessageID, "text": turn.Prompt}, "会话事件落库失败，当前响应已终止。") {
		return
	}

	if turn.ShouldError {
		_ = writeEvent("turn.error", &turnID, map[string]any{
			"code":      "deterministic_provider_error",
			"message":   "当前回复暂时失败，请稍后重试。",
			"retryable": false,
		})
		_ = writeEvent("turn.completed", &turnID, map[string]any{"status": "failed"})
		return
	}

	for _, chunk := range turn.Chunks {
		select {
		case <-r.Context().Done():
			return
		case <-turn.InterruptSignal():
			_ = writeEvent("turn.interrupted", &turnID, map[string]any{"reason": "user_requested"})
			_ = writeEvent("turn.completed", &turnID, map[string]any{"status": "interrupted"})
			return
		case <-time.After(25 * time.Millisecond):
		}

		if !writeEventOrFail("turn.agent_message.delta", &turnID, map[string]any{
			"message_id": turn.AssistantMessageID,
			"delta":      chunk,
		}, "会话事件落库失败，当前响应已终止。") {
			return
		}
	}

	_ = writeEvent("turn.agent_message.completed", &turnID, map[string]any{"message_id": turn.AssistantMessageID})
	_ = writeEvent("turn.completed", &turnID, map[string]any{"status": "completed"})
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

func conversationIDFromCompactPath(path string) string {
	const prefix = "/internal/cubebox/conversations/"
	const suffix = ":compact"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return ""
	}
	trimmed := strings.TrimPrefix(path, prefix)
	trimmed = strings.TrimSuffix(trimmed, suffix)
	return strings.TrimSpace(trimmed)
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
