package cubebox

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	compactionSummaryPrefix = "[[summary]] "
	defaultRecentItemLimit  = 4
)

type PromptItem struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type CanonicalContext struct {
	TenantID       string
	PrincipalID    string
	Language       string
	Page           string
	Permissions    []string
	BusinessObject string
	Model          string
}

type CompactionResult struct {
	SummaryID      string       `json:"summary_id"`
	SourceRange    [2]int       `json:"source_range"`
	SourceDigest   string       `json:"source_digest"`
	SummaryText    string       `json:"summary_text"`
	TokenBefore    int          `json:"token_before"`
	TokenAfter     int          `json:"token_after"`
	PromptView     []PromptItem `json:"prompt_view"`
	Compacted      bool         `json:"compacted"`
	Reason         string       `json:"reason"`
	CanonicalBlock string       `json:"canonical_block"`
}

func BuildPromptViewWithCompaction(events []CanonicalEvent, context CanonicalContext, currentUserInput string) CompactionResult {
	timeline := collectPromptTimeline(events)
	tokenBefore := estimatePromptTokens(timeline, currentUserInput)
	recent := keepRecentTimeline(timeline, defaultRecentItemLimit)
	compacted := len(timeline) > len(recent)
	compactedCount := len(timeline) - len(recent)
	sourceRange := [2]int{1, compactedCount}
	if !compacted {
		sourceRange = [2]int{0, 0}
	}
	if len(timeline) == 0 {
		sourceRange = [2]int{0, 0}
	}

	canonicalBlock := buildCanonicalContextBlock(context)
	summaryText := ""
	sourceDigest := ""
	if compacted {
		summaryText = buildSummaryText(timeline[:len(timeline)-len(recent)])
		sourceDigest = digestTimeline(timeline[:len(timeline)-len(recent)])
	}

	prompt := make([]PromptItem, 0, 2+len(recent)+1)
	prompt = append(prompt, PromptItem{Role: "system", Content: "你是 CubeBox，在当前租户与权限上下文下提供帮助。"})
	prompt = append(prompt, PromptItem{Role: "system", Content: canonicalBlock})
	if compacted {
		prompt = append(prompt, PromptItem{Role: "system", Content: compactionSummaryPrefix + summaryText})
	}
	prompt = append(prompt, recent...)
	if trimmed := strings.TrimSpace(currentUserInput); trimmed != "" {
		prompt = append(prompt, PromptItem{Role: "user", Content: trimmed})
	}

	return CompactionResult{
		SummaryID:      "summary_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		SourceRange:    sourceRange,
		SourceDigest:   sourceDigest,
		SummaryText:    summaryText,
		TokenBefore:    tokenBefore,
		TokenAfter:     estimatePromptTokens(prompt, ""),
		PromptView:     prompt,
		Compacted:      compacted,
		Reason:         compactionReason(compacted, tokenBefore),
		CanonicalBlock: canonicalBlock,
	}
}

func BuildCompactionEvent(conversationID string, turnID *string, sequence int, now time.Time, result CompactionResult) CanonicalEvent {
	return CanonicalEvent{
		EventID:        "evt_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		ConversationID: strings.TrimSpace(conversationID),
		TurnID:         turnID,
		Sequence:       sequence,
		Type:           "turn.context_compacted",
		TS:             now.UTC().Format(time.RFC3339),
		Payload: map[string]any{
			"summary_id":    result.SummaryID,
			"source_range":  []int{result.SourceRange[0], result.SourceRange[1]},
			"summary_text":  result.SummaryText,
			"source_digest": result.SourceDigest,
			"token_before":  result.TokenBefore,
			"token_after":   result.TokenAfter,
			"reason":        result.Reason,
		},
	}
}

func buildCanonicalContextBlock(context CanonicalContext) string {
	permissions := strings.Join(filterNonEmpty(context.Permissions), ", ")
	return strings.TrimSpace(fmt.Sprintf(
		"tenant=%s\nprincipal=%s\nlanguage=%s\npage=%s\npermissions=%s\nbusiness_object=%s\nmodel=%s",
		strings.TrimSpace(context.TenantID),
		strings.TrimSpace(context.PrincipalID),
		normalizeDefault(context.Language, "zh"),
		normalizeDefault(context.Page, "cubebox"),
			normalizeDefault(permissions, "cubebox.conversations:use"),
		normalizeDefault(context.BusinessObject, "conversation"),
		normalizeDefault(context.Model, "deterministic-runtime"),
	))
}

func buildSummaryText(items []PromptItem) string {
	if len(items) == 0 {
		return "暂无可压缩历史。"
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s: %s", item.Role, content))
	}
	if len(parts) == 0 {
		return "暂无可压缩历史。"
	}
	return strings.Join(parts, "\n")
}

func collectPromptTimeline(events []CanonicalEvent) []PromptItem {
	items := make([]PromptItem, 0, len(events))
	agentChunks := make(map[string]string)
	for _, event := range events {
		switch event.Type {
		case "turn.user_message.accepted":
			text := strings.TrimSpace(stringValue(event.Payload["text"]))
			if text == "" || strings.HasPrefix(text, compactionSummaryPrefix) {
				continue
			}
			items = append(items, PromptItem{Role: "user", Content: text})
		case "turn.agent_message.delta":
			messageID := stringValue(event.Payload["message_id"])
			if messageID == "" {
				continue
			}
			agentChunks[messageID] = agentChunks[messageID] + stringValue(event.Payload["delta"])
		case "turn.agent_message.completed":
			messageID := stringValue(event.Payload["message_id"])
			text := strings.TrimSpace(agentChunks[messageID])
			if text == "" {
				continue
			}
			items = append(items, PromptItem{Role: "assistant", Content: text})
			delete(agentChunks, messageID)
		case "turn.context_compacted":
			text := strings.TrimSpace(stringValue(event.Payload["summary_text"]))
			if text == "" {
				continue
			}
			items = append(items, PromptItem{Role: "summary", Content: text})
		}
	}
	return items
}

func keepRecentTimeline(items []PromptItem, limit int) []PromptItem {
	if limit <= 0 || len(items) <= limit {
		return append([]PromptItem(nil), items...)
	}
	return append([]PromptItem(nil), items[len(items)-limit:]...)
}

func estimatePromptTokens(items []PromptItem, extraUserInput string) int {
	total := 0
	for _, item := range items {
		total += estimateTextTokens(item.Content)
	}
	total += estimateTextTokens(extraUserInput)
	return total
}

func estimateTextTokens(text string) int {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0
	}
	runes := len([]rune(trimmed))
	return runes/4 + 1
}

func digestTimeline(items []PromptItem) string {
	h := sha256.Sum256([]byte(buildSummaryText(items)))
	return hex.EncodeToString(h[:])
}

func compactionReason(compacted bool, tokenBefore int) string {
	if compacted {
		return "history_limit_exceeded"
	}
	if tokenBefore > 0 {
		return "within_budget"
	}
	return "empty_history"
}

func filterNonEmpty(values []string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	return filtered
}

func normalizeDefault(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}
