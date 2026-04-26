package cubebox

import (
	"strings"
	"testing"
)

func TestBuildPromptViewUsesFullHistoryViewAndReinjectsCanonicalContext(t *testing.T) {
	result := BuildPromptView([]CanonicalEvent{
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
	}, "请基于最新上下文继续回答")

	if len(result.PromptView) != 8 {
		t.Fatalf("expected canonical context plus full history and current input, got %d", len(result.PromptView))
	}
	if result.PromptView[1].Role != "system" || result.PromptView[1].Content == "" {
		t.Fatalf("expected canonical context reinjection, got %#v", result.PromptView[1])
	}
	if strings.Contains(result.PromptView[1].Content, "provider_id=") || strings.Contains(result.PromptView[1].Content, "runtime=") {
		t.Fatalf("expected canonical block without runtime metadata, got %#v", result.PromptView[1])
	}
	if strings.Contains(result.PromptView[1].Content, "page"+"_facts=") {
		t.Fatalf("expected canonical block without page facts, got %#v", result.PromptView[1])
	}
	if result.PromptView[2].Role != "user" || result.PromptView[2].Content != "请总结当前进度" {
		t.Fatalf("expected first raw history item after canonical block, got %#v", result.PromptView[2])
	}
	last := result.PromptView[len(result.PromptView)-1]
	if last.Role != "user" || last.Content != "请基于最新上下文继续回答" {
		t.Fatalf("unexpected final prompt item: %#v", last)
	}
	if result.TokenBefore <= 0 || result.TokenAfter <= 0 {
		t.Fatalf("expected non-zero token estimates, before=%d after=%d", result.TokenBefore, result.TokenAfter)
	}
}

func TestBuildPromptViewPreservesCurrentUserInputWhitespace(t *testing.T) {
	result := BuildPromptView(nil, CanonicalContext{TenantID: "tenant-a", PrincipalID: "principal-a"}, "\n  请继续回答  \n")

	last := result.PromptView[len(result.PromptView)-1]
	if last.Role != "user" || last.Content != "\n  请继续回答  \n" {
		t.Fatalf("unexpected final prompt item: %#v", last)
	}
}

func TestBuildPromptViewPreservesRecentUserWhitespaceWithinBudget(t *testing.T) {
	result := BuildPromptView([]CanonicalEvent{
		{
			Type:    "turn.user_message.accepted",
			Payload: map[string]any{"message_id": "msg_user_1", "text": "\n  第一轮原始问题  \n"},
		},
		{
			Type:    "turn.agent_message.delta",
			Payload: map[string]any{"message_id": "msg_agent_1", "delta": "收到"},
		},
		{
			Type:    "turn.agent_message.completed",
			Payload: map[string]any{"message_id": "msg_agent_1"},
		},
	}, CanonicalContext{TenantID: "tenant-a", PrincipalID: "principal-a"}, "")

	found := false
	for _, item := range result.PromptView {
		if item.Role == "user" && item.Content == "\n  第一轮原始问题  \n" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected recent user whitespace to remain unchanged: %#v", result.PromptView)
	}
}

func TestBuildPromptViewSkipsSummaryPrefixAndIgnoresHistoricalCompactEvents(t *testing.T) {
	result := BuildPromptView([]CanonicalEvent{
		{
			Type:    "turn.user_message.accepted",
			Payload: map[string]any{"text": "继续输出最终方案之前的原始输入"},
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

	if len(result.PromptView) != 4 {
		t.Fatalf("expected system baseline plus two raw user messages, got %#v", result.PromptView)
	}
	for _, item := range result.PromptView {
		if strings.Contains(item.Content, "已确认租户和权限") {
			t.Fatalf("historical compact summary must not be replayed into provider prompt view: %#v", result.PromptView)
		}
	}
}

func TestBuildPromptViewDoesNotReplaceOriginalMessages(t *testing.T) {
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

	result := BuildPromptView(events, CanonicalContext{
		TenantID:    "tenant-a",
		PrincipalID: "principal-a",
	}, "")

	original := collectPromptTimeline(events)
	if len(original) != 5 {
		t.Fatalf("expected original timeline to remain fully reconstructable, got %d", len(original))
	}
	if original[0].Content != "第一轮原始问题" || original[1].Content != "第一轮原始回答" {
		t.Fatalf("expected original messages to remain unchanged, got %#v", original)
	}
	if len(result.PromptView) != 7 {
		t.Fatalf("expected full history prompt view, got %#v", result.PromptView)
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

func TestCollectPromptTimelinePreservesOriginalWhitespaceAndSkipsCompactionEvents(t *testing.T) {
	events := []CanonicalEvent{
		{
			Type:    "turn.user_message.accepted",
			Payload: map[string]any{"message_id": "msg_user_1", "text": "\n1) first\n\n2) second\n"},
		},
		{
			Type:    "turn.agent_message.delta",
			Payload: map[string]any{"message_id": "msg_agent_1", "delta": "A"},
		},
		{
			Type:    "turn.agent_message.delta",
			Payload: map[string]any{"message_id": "msg_agent_1", "delta": "\n\n"},
		},
		{
			Type:    "turn.agent_message.delta",
			Payload: map[string]any{"message_id": "msg_agent_1", "delta": "B"},
		},
		{
			Type:    "turn.agent_message.completed",
			Payload: map[string]any{"message_id": "msg_agent_1"},
		},
		{
			Type:    "turn.context_compacted",
			Payload: map[string]any{"summary_text": "\nsummary line\n"},
		},
	}

	timeline := collectPromptTimeline(events)
	if len(timeline) != 2 {
		t.Fatalf("unexpected timeline=%#v", timeline)
	}
	if timeline[0].Content != "\n1) first\n\n2) second\n" {
		t.Fatalf("unexpected user content=%q", timeline[0].Content)
	}
	if timeline[1].Content != "A\n\nB" {
		t.Fatalf("unexpected assistant content=%q", timeline[1].Content)
	}
}

func TestBuildPromptViewPreservesLongHistoricalUserMessageInFullHistoryView(t *testing.T) {
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

	result := BuildPromptView(events, CanonicalContext{TenantID: "tenant-a", PrincipalID: "principal-a"}, "")

	foundHistoricalUser := false
	for _, item := range result.PromptView {
		if item.Role == "user" && item.Content == longUserMessage {
			foundHistoricalUser = true
		}
		if item.Role == "user" && strings.Contains(item.Content, "[truncated]") {
			t.Fatalf("phase-1 full history view must not truncate historical user messages: %#v", result.PromptView)
		}
	}
	if !foundHistoricalUser {
		t.Fatalf("expected long historical user message to remain intact in prompt view: %#v", result.PromptView)
	}
	original := collectPromptTimeline(events)
	if original[2].Content != longUserMessage {
		t.Fatal("expected original timeline to remain unmodified")
	}
}
