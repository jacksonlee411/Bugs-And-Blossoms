package cubebox

import (
	"encoding/json"
	"fmt"
	"strings"
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

type PromptViewBuildResult struct {
	TokenBefore    int          `json:"token_before"`
	TokenAfter     int          `json:"token_after"`
	PromptView     []PromptItem `json:"prompt_view"`
	CanonicalBlock string       `json:"canonical_block"`
}

func buildPromptViewForProvider(events []CanonicalEvent, context CanonicalContext, currentUserInput string) []PromptItem {
	return BuildPromptView(events, context, currentUserInput).PromptView
}

func BuildPromptView(events []CanonicalEvent, context CanonicalContext, currentUserInput string) PromptViewBuildResult {
	timeline := collectPromptTimeline(events)
	tokenBefore := estimatePromptTokens(timeline, currentUserInput)
	canonicalBlock := buildCanonicalContextBlock(context)

	prompt := make([]PromptItem, 0, 2+len(timeline)+1)
	prompt = append(prompt, PromptItem{Role: "system", Content: "你是 CubeBox，在当前租户与权限上下文下提供帮助。"})
	prompt = append(prompt, PromptItem{Role: "system", Content: canonicalBlock})
	prompt = append(prompt, timeline...)
	if strings.TrimSpace(currentUserInput) != "" {
		prompt = append(prompt, PromptItem{Role: "user", Content: currentUserInput})
	}

	return PromptViewBuildResult{
		TokenBefore:    tokenBefore,
		TokenAfter:     estimatePromptTokens(prompt, ""),
		PromptView:     prompt,
		CanonicalBlock: canonicalBlock,
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

func collectPromptTimeline(events []CanonicalEvent) []PromptItem {
	items := make([]PromptItem, 0, len(events))
	agentChunks := make(map[string]string)
	for _, event := range events {
		switch event.Type {
		case "turn.user_message.accepted":
			text := stringValue(event.Payload["text"])
			if strings.TrimSpace(text) == "" {
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
