package cubebox

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Conversation struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Archived bool   `json:"archived"`
}

type CanonicalEvent struct {
	EventID        string         `json:"event_id"`
	ConversationID string         `json:"conversation_id"`
	TurnID         *string        `json:"turn_id"`
	Sequence       int            `json:"sequence"`
	Type           string         `json:"type"`
	TS             string         `json:"ts"`
	Payload        map[string]any `json:"payload"`
}

type TimelineEventStream struct {
	Conversation Conversation     `json:"conversation"`
	Events       []CanonicalEvent `json:"events"`
}

type ConversationReplayResponse struct {
	Conversation Conversation     `json:"conversation"`
	Events       []CanonicalEvent `json:"events"`
	NextSequence int              `json:"next_sequence"`
}

type DeterministicTurn struct {
	ConversationID     string
	TurnID             string
	UserMessageID      string
	AssistantMessageID string
	Prompt             string
	Chunks             []string
	ShouldError        bool
	interrupt          <-chan struct{}
}

type Runtime struct {
	counter    atomic.Uint64
	mu         sync.Mutex
	interrupts map[string]chan struct{}
}

func NewRuntime() *Runtime {
	return &Runtime{
		interrupts: make(map[string]chan struct{}),
	}
}

func (r *Runtime) NewConversation() ConversationReplayResponse {
	conversation := Conversation{
		ID:       r.nextID("conv"),
		Title:    "新对话",
		Status:   "active",
		Archived: false,
	}
	return ConversationReplayResponse{
		Conversation: conversation,
		Events: []CanonicalEvent{
			{
				EventID:        r.nextID("evt"),
				ConversationID: conversation.ID,
				TurnID:         nil,
				Sequence:       1,
				Type:           "conversation.loaded",
				TS:             time.Now().UTC().Format(time.RFC3339),
				Payload: map[string]any{
					"title":    conversation.Title,
					"status":   conversation.Status,
					"archived": conversation.Archived,
				},
			},
		},
		NextSequence: 2,
	}
}

func (r *Runtime) NextEventID() string {
	return r.nextID("evt")
}

func (r *Runtime) LoadConversation(conversationID string) ConversationReplayResponse {
	conversation := Conversation{
		ID:       strings.TrimSpace(conversationID),
		Title:    "新对话",
		Status:   "active",
		Archived: false,
	}
	return ConversationReplayResponse{
		Conversation: conversation,
		Events: []CanonicalEvent{
			{
				EventID:        r.nextID("evt"),
				ConversationID: conversation.ID,
				TurnID:         nil,
				Sequence:       1,
				Type:           "conversation.loaded",
				TS:             time.Now().UTC().Format(time.RFC3339),
				Payload: map[string]any{
					"title":    conversation.Title,
					"status":   conversation.Status,
					"archived": conversation.Archived,
				},
			},
		},
		NextSequence: 2,
	}
}

func (r *Runtime) StartTurn(conversationID string, prompt string) DeterministicTurn {
	turnID := r.nextID("turn")
	ch := make(chan struct{})

	r.mu.Lock()
	r.interrupts[turnID] = ch
	r.mu.Unlock()

	return DeterministicTurn{
		ConversationID:     strings.TrimSpace(conversationID),
		TurnID:             turnID,
		UserMessageID:      r.nextID("msg_user"),
		AssistantMessageID: r.nextID("msg_agent"),
		Prompt:             strings.TrimSpace(prompt),
		Chunks:             deterministicChunks(prompt),
		ShouldError:        strings.Contains(strings.ToLower(prompt), "error"),
		interrupt:          ch,
	}
}

func (r *Runtime) InterruptTurn(turnID string) bool {
	r.mu.Lock()
	ch, ok := r.interrupts[strings.TrimSpace(turnID)]
	if ok {
		delete(r.interrupts, strings.TrimSpace(turnID))
	}
	r.mu.Unlock()
	if !ok {
		return false
	}
	close(ch)
	return true
}

func (r *Runtime) FinishTurn(turnID string) {
	r.mu.Lock()
	delete(r.interrupts, strings.TrimSpace(turnID))
	r.mu.Unlock()
}

func (r *Runtime) nextID(prefix string) string {
	n := r.counter.Add(1)
	return fmt.Sprintf("%s_%06d", prefix, n)
}

func (t DeterministicTurn) InterruptSignal() <-chan struct{} {
	return t.interrupt
}

func deterministicChunks(prompt string) []string {
	reply := fmt.Sprintf("已收到你的消息：%s\n我正在整理回复。", strings.TrimSpace(prompt))
	if strings.TrimSpace(prompt) == "" {
		reply = "已收到你的消息。\n我正在整理回复。"
	}
	runes := []rune(reply)
	first := min(8, len(runes))
	second := min(20, len(runes))
	return []string{
		string(runes[:first]),
		string(runes[first:second]),
		string(runes[second:]),
	}
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
