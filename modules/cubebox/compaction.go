package cubebox

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	compactionSummaryPrefix        = "[[summary]] "
	defaultRecentUserMessageTokens = 400
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
	PageContext    *PageContext
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
	sourceRange := [2]int{0, 0}

	canonicalBlock := buildCanonicalContextBlock(context)

	prompt := make([]PromptItem, 0, 2+len(timeline)+1)
	prompt = append(prompt, PromptItem{Role: "system", Content: "你是 CubeBox，在当前租户与权限上下文下提供帮助。"})
	prompt = append(prompt, PromptItem{Role: "system", Content: canonicalBlock})
	prompt = append(prompt, timeline...)
	if strings.TrimSpace(currentUserInput) != "" {
		prompt = append(prompt, PromptItem{Role: "user", Content: currentUserInput})
	}

	return CompactionResult{
		SummaryID:      "summary_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		SourceRange:    sourceRange,
		SourceDigest:   "",
		SummaryText:    "",
		TokenBefore:    tokenBefore,
		TokenAfter:     estimatePromptTokens(prompt, ""),
		PromptView:     prompt,
		Compacted:      false,
		Reason:         compactionReason(false, tokenBefore),
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
			"reason":        result.Reason,
		},
	}
}

func buildCanonicalContextBlock(context CanonicalContext) string {
	permissions := strings.Join(filterNonEmpty(context.Permissions), ", ")
	pageFacts := ""
	if normalized := NormalizePageContext(context.PageContext); normalized != nil {
		payload := map[string]any{}
		if normalized.CurrentObject != nil {
			payload["current_object"] = normalized.CurrentObject
		}
		if normalized.View != nil {
			payload["view"] = normalized.View
		}
		if len(payload) > 0 {
			body, err := json.Marshal(payload)
			if err == nil {
				pageFacts = string(body)
			}
		}
	}
	lines := []string{
		fmt.Sprintf("tenant=%s", strings.TrimSpace(context.TenantID)),
		fmt.Sprintf("principal=%s", strings.TrimSpace(context.PrincipalID)),
		fmt.Sprintf("language=%s", normalizeDefault(context.Language, "zh")),
		fmt.Sprintf("page=%s", normalizeDefault(context.Page, "cubebox")),
		fmt.Sprintf("permissions=%s", normalizeDefault(permissions, "cubebox.conversations:use")),
		fmt.Sprintf("business_object=%s", normalizeDefault(context.BusinessObject, "conversation")),
	}
	if pageFacts != "" {
		lines = append(lines, "page_facts="+pageFacts)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func buildSummaryText(items []PromptItem) string {
	if len(items) == 0 {
		return "暂无可压缩历史。"
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		content := item.Content
		if strings.TrimSpace(content) == "" {
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
			text := stringValue(event.Payload["text"])
			if strings.TrimSpace(text) == "" || strings.HasPrefix(text, compactionSummaryPrefix) {
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
			text := agentChunks[messageID]
			if strings.TrimSpace(text) == "" {
				continue
			}
			items = append(items, PromptItem{Role: "assistant", Content: text})
			delete(agentChunks, messageID)
		}
	}
	return items
}

func preserveOrTrimTextToApproxTokenLimit(text string, tokenLimit int) string {
	if tokenLimit <= 0 || strings.TrimSpace(text) == "" || estimateTextTokens(text) <= tokenLimit {
		return text
	}
	return trimTextToApproxTokenLimit(text, tokenLimit)
}

func trimTextToApproxTokenLimit(text string, tokenLimit int) string {
	content := strings.TrimSpace(text)
	if tokenLimit <= 0 || content == "" || estimateTextTokens(content) <= tokenLimit {
		return content
	}
	runes := []rune(content)
	maxRunes := tokenLimit * 4
	if maxRunes <= 0 {
		return content
	}
	if len(runes) > maxRunes {
		runes = runes[:maxRunes]
	}
	trimmed := strings.TrimSpace(string(runes))
	for trimmed != "" && estimateTextTokens(trimmed) > tokenLimit {
		current := []rune(trimmed)
		if len(current) == 0 {
			break
		}
		trimmed = strings.TrimSpace(string(current[:len(current)-1]))
	}
	if trimmed == "" {
		return "[truncated]"
	}
	return trimmed + "\n[truncated]"
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
	return fmt.Sprint(value)
}
