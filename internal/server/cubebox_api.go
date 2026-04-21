package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox"
)

type cubeboxCreateConversationRequest struct{}

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

func handleCubeBoxCreateConversationAPI(w http.ResponseWriter, r *http.Request, runtime *cubebox.Runtime) {
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req cubeboxCreateConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "invalid_json", "invalid json")
		return
	}

	writeJSON(w, http.StatusCreated, runtime.NewConversation())
}

func handleCubeBoxLoadConversationAPI(w http.ResponseWriter, r *http.Request, runtime *cubebox.Runtime) {
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	conversationID := conversationIDFromPath(r.URL.Path)
	if conversationID == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "conversation_id_required", "conversation id required")
		return
	}

	writeJSON(w, http.StatusOK, runtime.LoadConversation(conversationID))
}

func handleCubeBoxStreamTurnAPI(w http.ResponseWriter, r *http.Request, runtime *cubebox.Runtime) {
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
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

	turn := runtime.StartTurn(req.ConversationID, req.Prompt)
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
			EventID:        runtime.NextEventID(),
			ConversationID: req.ConversationID,
			TurnID:         turnID,
			Sequence:       sequence,
			Type:           eventType,
			TS:             time.Now().UTC().Format(time.RFC3339),
			Payload:        payload,
		}
		sequence += 1
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

	turnID := turn.TurnID
	if !writeEvent("turn.started", &turnID, map[string]any{"user_message_id": turn.UserMessageID}) {
		return
	}
	if !writeEvent("turn.user_message.accepted", &turnID, map[string]any{"message_id": turn.UserMessageID, "text": turn.Prompt}) {
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

		if !writeEvent("turn.agent_message.delta", &turnID, map[string]any{
			"message_id": turn.AssistantMessageID,
			"delta":      chunk,
		}) {
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

	writeJSON(w, http.StatusOK, cubeboxInterruptResponse{
		TurnID:      turnID,
		Interrupted: runtime.InterruptTurn(turnID),
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
