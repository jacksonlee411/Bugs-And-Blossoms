package cubebox

import (
	"strings"
	"testing"
	"time"
)

func TestBuildPromptViewWithCompactionUsesFullHistoryViewAndReinjectsCanonicalContext(t *testing.T) {
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
		PageContext: &PageContext{
			Page:           "/org/units/100000",
			BusinessObject: "orgunit",
			CurrentObject: &PageObjectContext{
				Domain:    "orgunit",
				EntityKey: "100000",
				Label:     "总部",
			},
			View: &PageViewContext{AsOf: "2026-04-25"},
		},
	}, "请基于最新上下文继续回答")

	if result.Compacted {
		t.Fatal("expected phase-1 no-summary baseline to avoid compact summary")
	}
	if result.SourceRange != [2]int{0, 0} {
		t.Fatalf("unexpected source range: %#v", result.SourceRange)
	}
	if result.SummaryText != "" {
		t.Fatalf("expected no summary text, got %q", result.SummaryText)
	}
	if len(result.PromptView) != 8 {
		t.Fatalf("expected canonical context plus full history and current input, got %d", len(result.PromptView))
	}
	if result.PromptView[1].Role != "system" || result.PromptView[1].Content == "" {
		t.Fatalf("expected canonical context reinjection, got %#v", result.PromptView[1])
	}
	if strings.Contains(result.PromptView[1].Content, "provider_id=") || strings.Contains(result.PromptView[1].Content, "runtime=") {
		t.Fatalf("expected canonical block without runtime metadata, got %#v", result.PromptView[1])
	}
	if !strings.Contains(result.PromptView[1].Content, "page_facts=") || !strings.Contains(result.PromptView[1].Content, "\"entity_key\":\"100000\"") {
		t.Fatalf("expected page facts in canonical block, got %#v", result.PromptView[1])
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

func TestBuildPromptViewWithCompactionPreservesCurrentUserInputWhitespace(t *testing.T) {
	result := BuildPromptViewWithCompaction(nil, CanonicalContext{TenantID: "tenant-a", PrincipalID: "principal-a"}, "\n  请继续回答  \n")

	last := result.PromptView[len(result.PromptView)-1]
	if last.Role != "user" || last.Content != "\n  请继续回答  \n" {
		t.Fatalf("unexpected final prompt item: %#v", last)
	}
}

func TestBuildPromptViewWithCompactionPreservesRecentUserWhitespaceWithinBudget(t *testing.T) {
	result := BuildPromptViewWithCompaction([]CanonicalEvent{
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

func TestBuildPromptViewWithCompactionSkipsSummaryPrefixAndIgnoresHistoricalCompactEvents(t *testing.T) {
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
		t.Fatal("did not expect compaction for phase-1 baseline")
	}
	if len(result.PromptView) != 3 {
		t.Fatalf("expected only system baseline and one raw user message, got %#v", result.PromptView)
	}
	for _, item := range result.PromptView {
		if item.Role == "user" && item.Content == "[[summary]] stale summary" {
			t.Fatalf("summary prefix user message should be filtered: %#v", result.PromptView)
		}
		if strings.Contains(item.Content, "已确认租户和权限") {
			t.Fatalf("historical compact summary must not be replayed into provider prompt view: %#v", result.PromptView)
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

	if result.Compacted {
		t.Fatal("expected no compact summary in phase-1 baseline")
	}
	if result.SummaryText != "" {
		t.Fatal("expected no summary text for prompt view")
	}
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

func TestBuildPromptViewWithCompactionPreservesLongHistoricalUserMessageInFullHistoryView(t *testing.T) {
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

func TestTrimTextToApproxTokenLimitTruncatesBoundaryCase(t *testing.T) {
	const tokenLimit = 400
	boundary := strings.Repeat("长", tokenLimit*4)
	if estimateTextTokens(boundary) <= tokenLimit {
		t.Fatalf("expected boundary text to exceed token limit, got %d", estimateTextTokens(boundary))
	}
	trimmed := trimTextToApproxTokenLimit(boundary, tokenLimit)
	if !strings.Contains(trimmed, "[truncated]") {
		t.Fatalf("expected boundary text to be truncated, got %q", trimmed)
	}
	if estimateTextTokens(trimmed) > tokenLimit+4 {
		t.Fatalf("expected trimmed boundary text within guardrail, got %d", estimateTextTokens(trimmed))
	}
}

func TestPreserveOrTrimTextToApproxTokenLimitKeepsBoundaryWhitespaceWhenWithinBudget(t *testing.T) {
	const tokenLimit = 400
	input := "\n  short recent user input  \n"
	got := preserveOrTrimTextToApproxTokenLimit(input, tokenLimit)
	if got != input {
		t.Fatalf("expected within-budget text to remain unchanged, got %q", got)
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
