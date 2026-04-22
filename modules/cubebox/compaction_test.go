package cubebox

import (
	"strings"
	"testing"
	"time"
)

func TestBuildPromptViewWithCompactionCompactsOldHistoryAndReinjectsCanonicalContext(t *testing.T) {
	result := BuildPromptViewWithCompaction([]CanonicalEvent{
		{
			Type:    "turn.user_message.accepted",
			Payload: map[string]any{"text": "请总结当前进度"},
		},
		{
			Type:    "turn.agent_message.delta",
			Payload: map[string]any{"message_id": "msg_agent_1", "delta": "当前已完成 Phase B，"},
		},
		{
			Type:    "turn.agent_message.completed",
			Payload: map[string]any{"message_id": "msg_agent_1"},
		},
		{
			Type:    "turn.user_message.accepted",
			Payload: map[string]any{"text": "接下来做什么？"},
		},
		{
			Type:    "turn.agent_message.delta",
			Payload: map[string]any{"message_id": "msg_agent_2", "delta": "先补齐 Phase C 验证。"},
		},
		{
			Type:    "turn.agent_message.completed",
			Payload: map[string]any{"message_id": "msg_agent_2"},
		},
		{
			Type:    "turn.user_message.accepted",
			Payload: map[string]any{"text": "然后开始压缩实现。"},
		},
	}, CanonicalContext{
		TenantID:       "tenant-a",
		PrincipalID:    "principal-a",
		Language:       "zh",
		Page:           "/app/cubebox",
		Permissions:    []string{"cubebox.conversations:use"},
		BusinessObject: "conversation",
		Model:          "deterministic-runtime",
	}, "请基于最新上下文继续回答")

	if !result.Compacted {
		t.Fatal("expected compaction to happen")
	}
	if result.SourceRange != [2]int{1, 1} {
		t.Fatalf("unexpected source range: %#v", result.SourceRange)
	}
	if len(result.PromptView) < 4 {
		t.Fatalf("expected prompt view with canonical context and recent items, got %d", len(result.PromptView))
	}
	if result.PromptView[1].Role != "system" || result.PromptView[1].Content == "" {
		t.Fatalf("expected canonical context reinjection, got %#v", result.PromptView[1])
	}
	if result.PromptView[2].Role != "system" || result.PromptView[2].Content == "" {
		t.Fatalf("expected summary item in prompt view, got %#v", result.PromptView[2])
	}
	last := result.PromptView[len(result.PromptView)-1]
	if last.Role != "user" || last.Content != "请基于最新上下文继续回答" {
		t.Fatalf("unexpected final prompt item: %#v", last)
	}
	if result.TokenBefore <= 0 || result.TokenAfter <= 0 {
		t.Fatalf("expected non-zero token estimates, before=%d after=%d", result.TokenBefore, result.TokenAfter)
	}
}

func TestBuildPromptViewWithCompactionSkipsSummaryPrefixAndKeepsRecentItems(t *testing.T) {
	result := BuildPromptViewWithCompaction([]CanonicalEvent{
		{
			Type:    "turn.user_message.accepted",
			Payload: map[string]any{"text": "[[summary]] stale summary"},
		},
		{
			Type:    "turn.context_compacted",
			Payload: map[string]any{"summary_text": "已确认租户和权限。"},
		},
		{
			Type:    "turn.user_message.accepted",
			Payload: map[string]any{"text": "继续输出最终方案"},
		},
	}, CanonicalContext{TenantID: "tenant-a", PrincipalID: "principal-a"}, "")

	if result.Compacted {
		t.Fatal("did not expect compaction for short history")
	}
	for _, item := range result.PromptView {
		if item.Role == "user" && item.Content == "[[summary]] stale summary" {
			t.Fatalf("summary prefix user message should be filtered: %#v", result.PromptView)
		}
	}
}

func TestBuildPromptViewWithCompactionDoesNotReplaceOriginalMessages(t *testing.T) {
	events := []CanonicalEvent{
		{
			Type:    "turn.user_message.accepted",
			Payload: map[string]any{"message_id": "msg_user_1", "text": "第一轮原始问题"},
		},
		{
			Type:    "turn.agent_message.delta",
			Payload: map[string]any{"message_id": "msg_agent_1", "delta": "第一轮原始回答"},
		},
		{
			Type:    "turn.agent_message.completed",
			Payload: map[string]any{"message_id": "msg_agent_1"},
		},
		{
			Type:    "turn.user_message.accepted",
			Payload: map[string]any{"message_id": "msg_user_2", "text": "第二轮继续追问"},
		},
		{
			Type:    "turn.agent_message.delta",
			Payload: map[string]any{"message_id": "msg_agent_2", "delta": "第二轮继续回答"},
		},
		{
			Type:    "turn.agent_message.completed",
			Payload: map[string]any{"message_id": "msg_agent_2"},
		},
		{
			Type:    "turn.user_message.accepted",
			Payload: map[string]any{"message_id": "msg_user_3", "text": "第三轮最新问题"},
		},
	}

	result := BuildPromptViewWithCompaction(events, CanonicalContext{
		TenantID:    "tenant-a",
		PrincipalID: "principal-a",
	}, "")

	if !result.Compacted {
		t.Fatal("expected compaction to happen")
	}
	if result.SummaryText == "" {
		t.Fatal("expected summary text for prompt view")
	}
	original := collectPromptTimeline(events)
	if len(original) != 5 {
		t.Fatalf("expected original timeline to remain fully reconstructable, got %d", len(original))
	}
	if original[0].Content != "第一轮原始问题" || original[1].Content != "第一轮原始回答" {
		t.Fatalf("expected original messages to remain unchanged, got %#v", original)
	}
	foundLatestUser := false
	for _, item := range result.PromptView {
		if item.Role == "user" && item.Content == "第三轮最新问题" {
			foundLatestUser = true
		}
	}
	if !foundLatestUser {
		t.Fatalf("expected latest raw user message to remain in prompt view, got %#v", result.PromptView)
	}
}

func TestBuildPromptViewWithCompactionTruncatesLongRecentUserMessageOnlyInPromptView(t *testing.T) {
	longUserMessage := ""
	for i := 0; i < 1800; i++ {
		longUserMessage += "长"
	}
	events := []CanonicalEvent{
		{
			Type:    "turn.user_message.accepted",
			Payload: map[string]any{"message_id": "msg_old_1", "text": "第一轮原始问题"},
		},
		{
			Type:    "turn.agent_message.delta",
			Payload: map[string]any{"message_id": "msg_agent_1", "delta": "第一轮原始回答"},
		},
		{
			Type:    "turn.agent_message.completed",
			Payload: map[string]any{"message_id": "msg_agent_1"},
		},
		{
			Type:    "turn.user_message.accepted",
			Payload: map[string]any{"message_id": "msg_user_long", "text": longUserMessage},
		},
		{
			Type:    "turn.agent_message.delta",
			Payload: map[string]any{"message_id": "msg_agent_2", "delta": "收到长消息"},
		},
		{
			Type:    "turn.agent_message.completed",
			Payload: map[string]any{"message_id": "msg_agent_2"},
		},
		{
			Type:    "turn.user_message.accepted",
			Payload: map[string]any{"message_id": "msg_user_latest", "text": "最新问题"},
		},
	}

	result := BuildPromptViewWithCompaction(events, CanonicalContext{TenantID: "tenant-a", PrincipalID: "principal-a"}, "")

	if !result.Compacted {
		t.Fatal("expected compaction to happen")
	}
	foundTrimmed := false
	for _, item := range result.PromptView {
		if item.Role == "user" && strings.Contains(item.Content, "[truncated]") {
			foundTrimmed = true
			if estimateTextTokens(item.Content) > defaultRecentUserMessageTokens+4 {
				t.Fatalf("expected trimmed user message within token guardrail, got %d", estimateTextTokens(item.Content))
			}
		}
	}
	if !foundTrimmed {
		t.Fatalf("expected long recent user message to be trimmed in prompt view: %#v", result.PromptView)
	}
	original := collectPromptTimeline(events)
	if original[2].Content != longUserMessage {
		t.Fatal("expected original timeline to remain unmodified")
	}
}

func TestTrimTextToApproxTokenLimitTruncatesBoundaryCase(t *testing.T) {
	boundary := strings.Repeat("长", defaultRecentUserMessageTokens*4)
	if estimateTextTokens(boundary) <= defaultRecentUserMessageTokens {
		t.Fatalf("expected boundary text to exceed token limit, got %d", estimateTextTokens(boundary))
	}
	trimmed := trimTextToApproxTokenLimit(boundary, defaultRecentUserMessageTokens)
	if !strings.Contains(trimmed, "[truncated]") {
		t.Fatalf("expected boundary text to be truncated, got %q", trimmed)
	}
	if estimateTextTokens(trimmed) > defaultRecentUserMessageTokens+4 {
		t.Fatalf("expected trimmed boundary text within guardrail, got %d", estimateTextTokens(trimmed))
	}
}

func TestBuildCompactionEventUsesCanonicalEnvelope(t *testing.T) {
	turnID := "turn_1"
	event := BuildCompactionEvent("conv_1", &turnID, 9, time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC), CompactionResult{
		SummaryID:    "summary_1",
		SourceRange:  [2]int{1, 4},
		SummaryText:  "已确认上下文。",
		SourceDigest: "abc",
		TokenBefore:  120,
		TokenAfter:   64,
		Reason:       "manual",
	})

	if event.Type != "turn.context_compacted" {
		t.Fatalf("unexpected type=%s", event.Type)
	}
	if event.Sequence != 9 {
		t.Fatalf("unexpected sequence=%d", event.Sequence)
	}
	sourceRange, ok := event.Payload["source_range"].([]int)
	if !ok || len(sourceRange) != 2 || sourceRange[0] != 1 || sourceRange[1] != 4 {
		t.Fatalf("unexpected source_range=%#v", event.Payload["source_range"])
	}
	if event.Payload["summary_id"] != "summary_1" {
		t.Fatalf("unexpected payload=%#v", event.Payload)
	}
	if _, ok := event.Payload["token_before"]; ok {
		t.Fatalf("token_before must remain debug-only and outside canonical event payload: %#v", event.Payload)
	}
	if _, ok := event.Payload["token_after"]; ok {
		t.Fatalf("token_after must remain debug-only and outside canonical event payload: %#v", event.Payload)
	}
}
