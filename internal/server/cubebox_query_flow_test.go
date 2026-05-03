package server

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox"
)

func TestCubeboxProviderQueryNarratorRejectsTargetMismatch(t *testing.T) {
	narrator := &cubeboxProviderQueryNarrator{
		configReader: cubeboxRuntimeConfigReaderStub{config: cubebox.ActiveModelRuntimeConfig{
			Provider:   cubebox.ModelProvider{ID: "provider-a", ProviderType: "openai-compatible", BaseURL: "https://example.com", Enabled: true},
			Selection:  cubebox.ActiveModelSelection{ModelSlug: "gpt-5.2"},
			Credential: cubebox.ModelCredential{SecretRef: "env://OPENAI_API_KEY"},
		}},
		adapter:        &cubeboxProviderAdapterStub{},
		secretResolver: cubeboxSecretResolverStub{secret: "sk-test"},
	}

	_, err := narrator.NarrateQueryResult(context.Background(), cubeboxQueryNarrationInput{
		TenantID:             "tenant-a",
		Prompt:               "查总部",
		Results:              []cubebox.QueryNarrationResult{{Domain: "orgunit", Data: map[string]any{"entity": map[string]any{"name": "总部"}}}},
		ExpectedProviderID:   "provider-b",
		ExpectedProviderType: "openai-compatible",
		ExpectedModelSlug:    "gpt-5.2",
	})
	if !errors.Is(err, errCubeboxQueryNarrationTargetMismatch) {
		t.Fatalf("expected target mismatch, got %v", err)
	}
}

func TestQueryNarrationErrorToTerminalMapsTargetMismatch(t *testing.T) {
	terminal := queryNarrationErrorToTerminal(errCubeboxQueryNarrationTargetMismatch)
	if terminal.Code != "ai_reply_model_target_mismatch" {
		t.Fatalf("unexpected code=%s", terminal.Code)
	}
}

func TestQueryNarrationErrorToTerminalMapsProviderFailure(t *testing.T) {
	terminal := queryNarrationErrorToTerminal(cubebox.ErrProviderUnavailable)
	if terminal.Code != "ai_reply_render_failed" {
		t.Fatalf("unexpected code=%s", terminal.Code)
	}
	if !terminal.Retryable {
		t.Fatal("expected retryable provider failure")
	}
}

func TestQueryNarrationErrorToTerminalMapsContractViolation(t *testing.T) {
	terminal := queryNarrationErrorToTerminal(errCubeboxQueryNarrationContractViolation)
	if terminal.Code != "ai_reply_render_failed" {
		t.Fatalf("unexpected code=%s", terminal.Code)
	}
	if !terminal.Retryable {
		t.Fatal("expected retryable contract violation")
	}
}

func TestQueryPlannerErrorToTerminalMapsProviderFailure(t *testing.T) {
	terminal := queryPlannerErrorToTerminal(cubebox.ErrProviderUnavailable)
	if terminal.Code != "ai_reply_render_failed" {
		t.Fatalf("unexpected code=%s", terminal.Code)
	}
	if !terminal.Retryable {
		t.Fatal("expected retryable provider failure")
	}
	if strings.Contains(terminal.Message, "叙述") {
		t.Fatalf("planner provider failure message should mention planning, got %q", terminal.Message)
	}
}

func TestBuildQueryNarrationMessagesForbidsInternalLeakage(t *testing.T) {
	messages := buildQueryNarrationMessages(`{"user_prompt":"查一下 100000 在 2026-04-24 的组织详情"}`)
	if len(messages) != 2 {
		t.Fatalf("unexpected message count=%d", len(messages))
	}
	systemPrompt := messages[0].Content
	for _, snippet := range []string{
		"不得逐字回显整份原始 JSON",
		"不得暴露实现细节或计划执行痕迹",
		"api_key",
		"executor_key",
		"payload",
		"好的回答",
		"不好的回答",
	} {
		if !strings.Contains(systemPrompt, snippet) {
			t.Fatalf("expected narrator prompt to contain %q, got %q", snippet, systemPrompt)
		}
	}
}

func TestBuildQueryClarificationMessagesUsesClarifierPrompt(t *testing.T) {
	messages := buildQueryClarificationMessages(`{"user_prompt":"查华东","candidates":[{"entity_key":"1001"}]}`)
	if len(messages) != 2 {
		t.Fatalf("unexpected message count=%d", len(messages))
	}
	if !strings.Contains(messages[0].Content, "查询澄清器") || !strings.Contains(messages[0].Content, "不能静默替用户选择") {
		t.Fatalf("unexpected clarification prompt=%q", messages[0].Content)
	}
}

func TestBuildNoQueryGuidanceMessagesUsesControlledPrompt(t *testing.T) {
	messages := buildNoQueryGuidanceMessages(`{"scope_summary":"当前主要支持组织相关只读查询。","suggested_prompts":["查“华东销售中心”的详情"]}`)
	if len(messages) != 2 {
		t.Fatalf("unexpected message count=%d", len(messages))
	}
	systemPrompt := messages[0].Content
	for _, snippet := range []string{
		"只能使用 provided suggested_prompts",
		"不得新增未提供的能力或示例",
		"不得提到内部术语",
		"输出纯文本",
	} {
		if !strings.Contains(systemPrompt, snippet) {
			t.Fatalf("expected no-query guidance prompt to contain %q, got %q", snippet, systemPrompt)
		}
	}
}

func TestValidateNoQueryGuidanceTextRejectsInternalTerms(t *testing.T) {
	facts := cubeboxNoQueryGuidanceEnvelope{
		ScopeSummary:     "当前主要支持组织相关只读查询。",
		SuggestedPrompts: []string{"查“华东销售中心”的详情"},
	}
	text := "当前主要支持组织相关只读查询。\n\n你可以直接这样问：\n1. 查“华东销售中心”的详情\n\n内部状态是 NO_QUERY。"
	if err := validateNoQueryGuidanceText(text, facts); !errors.Is(err, errCubeboxQueryNarrationContractViolation) {
		t.Fatalf("expected contract violation, got %v", err)
	}
}

func TestValidateNoQueryGuidanceTextRequiresProvidedPrompts(t *testing.T) {
	facts := cubeboxNoQueryGuidanceEnvelope{
		ScopeSummary:     "当前主要支持组织相关只读查询。",
		SuggestedPrompts: []string{"查“华东销售中心”的详情", "搜索名称包含“销售”的组织"},
	}
	text := "当前主要支持组织相关只读查询。\n\n你可以直接这样问：\n1. 查“华东销售中心”的详情"
	if err := validateNoQueryGuidanceText(text, facts); !errors.Is(err, errCubeboxQueryNarrationContractViolation) {
		t.Fatalf("expected missing prompt violation, got %v", err)
	}
}

func TestBuildQueryClarificationEnvelopeOmitsQueryIntent(t *testing.T) {
	body, err := json.Marshal(buildQueryClarificationEnvelope(cubeboxQueryClarificationInput{
		Prompt:             "查华东",
		ErrorCode:          "org_unit_search_ambiguous",
		CandidateGroupID:   "candgrp_test_1",
		CandidateSource:    "execution_error",
		CandidateCount:     2,
		CannotSilentSelect: true,
		QueryContext: cubebox.QueryContext{
			RecentConfirmedEntities: []cubebox.QueryEntity{
				{Domain: "orgunit", Intent: "orgunit.search", EntityKey: "1001", AsOf: "2026-04-24", SourceOperationID: "orgunit.search"},
			},
			RecentCandidateGroups: []cubebox.QueryCandidateGroup{
				{
					GroupID:            "candgrp_prev_1",
					CandidateSource:    "execution_error",
					CandidateCount:     1,
					CannotSilentSelect: true,
					Candidates: []cubebox.QueryCandidate{
						{Domain: "orgunit", EntityKey: "1001", Name: "华东销售中心", AsOf: "2026-04-24"},
					},
				},
			},
		},
		Candidates: []cubebox.QueryCandidate{
			{Domain: "orgunit", EntityKey: "1001", Name: "华东销售中心", AsOf: "2026-04-24"},
			{Domain: "orgunit", EntityKey: "1002", Name: "华东运营中心", AsOf: "2026-04-24"},
		},
	}))
	if err != nil {
		t.Fatalf("marshal clarification envelope: %v", err)
	}
	text := string(body)
	for _, snippet := range []string{
		`"user_prompt":"查华东"`,
		`"query_evidence_window"`,
		`"observations"`,
		`"candidates"`,
		`"candidate_group_id":"candgrp_test_1"`,
		`"group_id":"candgrp_test_1"`,
		`"kind":"presented_options"`,
		`"entity_key":"1001"`,
		`"error_code":"org_unit_search_ambiguous"`,
		`"candidate_source":"execution_error"`,
		`"candidate_count":2`,
		`"cannot_silent_select":true`,
	} {
		if !strings.Contains(text, snippet) {
			t.Fatalf("expected clarification body to contain %q, got %q", snippet, text)
		}
	}
	for _, forbidden := range []string{
		`"query_intent"`,
		`"intent":"orgunit.search"`,
		`"resolved_entity"`,
		`"source_operation_id"`,
		`"target_org_code"`,
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("expected clarification body to omit %q, got %q", forbidden, text)
		}
	}
}

func TestBuildQueryEvidenceWindowPromptBlockIncludesResultListGuidance(t *testing.T) {
	block := buildQueryEvidenceWindowPromptBlock(cubebox.QueryContext{
		RecentCandidateGroups: []cubebox.QueryCandidateGroup{
			{
				GroupID:         "resultgrp_finance",
				CandidateSource: "results",
				CandidateCount:  2,
				Candidates: []cubebox.QueryCandidate{
					{Domain: "orgunit", EntityKey: "200001", Name: "财务部", AsOf: "2026-04-27"},
					{Domain: "orgunit", EntityKey: "200002", Name: "财务一组", AsOf: "2026-04-27"},
				},
			},
		},
	}, "增加列出他们的组织路径")

	for _, snippet := range []string{
		`"kind":"result_list"`,
		`"group_id":"resultgrp_finance"`,
		`"option_source":"results"`,
		`补充字段/增加列/列出路径`,
	} {
		if !strings.Contains(block, snippet) {
			t.Fatalf("expected result list snippet %q in block=%q", snippet, block)
		}
	}
}

func TestBuildQueryClarificationEnvelopeProjectsCurrentCandidateGroupIntoEvidenceWindow(t *testing.T) {
	rawCandidates := make([]cubebox.QueryCandidate, 0, 25)
	for i := 0; i < 25; i++ {
		rawCandidates = append(rawCandidates, cubebox.QueryCandidate{
			Domain:    "orgunit",
			EntityKey: "10" + strings.Repeat("0", 2) + string(rune('A'+(i%26))),
			Name:      "候选组织",
			AsOf:      "2026-04-24",
		})
	}

	envelope := buildQueryClarificationEnvelope(cubeboxQueryClarificationInput{
		Prompt:             "查华东",
		CandidateGroupID:   "candgrp_live_1",
		CandidateSource:    "execution_error",
		CandidateCount:     len(rawCandidates),
		CannotSilentSelect: true,
		QueryContext: cubebox.QueryContext{
			RecentCandidateGroups: []cubebox.QueryCandidateGroup{
				{
					GroupID:         "candgrp_prev_1",
					CandidateSource: "execution_error",
					CandidateCount:  1,
					Candidates: []cubebox.QueryCandidate{
						{Domain: "orgunit", EntityKey: "1001", Name: "旧候选", AsOf: "2026-04-24"},
					},
				},
			},
		},
		Candidates: rawCandidates,
	})

	if got, want := len(envelope.Candidates), queryContextMaxClarifierCandidatesPerGroup; got != want {
		t.Fatalf("expected top-level candidates truncated to %d, got %d", want, got)
	}
	if envelope.CandidateGroupID != "candgrp_live_1" || envelope.CandidateCount != len(rawCandidates) || !envelope.CannotSilentSelect {
		t.Fatalf("unexpected envelope metadata=%#v", envelope)
	}
	body, err := json.Marshal(envelope.QueryEvidenceWindow)
	if err != nil {
		t.Fatalf("marshal evidence window: %v", err)
	}
	text := string(body)
	if !strings.Contains(text, `"kind":"presented_options"`) || !strings.Contains(text, `"group_id":"candgrp_live_1"`) {
		t.Fatalf("expected current group in evidence window, got %s", text)
	}
	if strings.Contains(text, `10`+strings.Repeat("0", 2)+`U`) {
		t.Fatalf("expected evidence options truncated, got %s", text)
	}
}

func TestBuildQueryNarrationEnvelopeUsesOnlyCurrentResultsAsFacts(t *testing.T) {
	envelope := buildQueryNarrationEnvelope(cubeboxQueryNarrationInput{
		Prompt: "列出全部组织",
		QueryContext: cubebox.QueryContext{
			RecentConfirmedEntities: []cubebox.QueryEntity{
				{Domain: "orgunit", Intent: "orgunit.list", EntityKey: "200006", AsOf: "2026-04-27"},
			},
			RecentDialogueTurns: []cubebox.QueryDialogueTurn{
				{UserPrompt: "列出成本组织", AssistantReply: "未查询到任何成本组织。"},
			},
		},
		Results: []cubebox.QueryNarrationResult{{
			Domain: "orgunit",
			Data: map[string]any{
				"as_of":     "2026-04-27",
				"org_units": []map[string]any{{"org_code": "100000", "name": "飞虫与鲜花"}},
			},
		}},
	})
	body, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	text := string(body)
	for _, snippet := range []string{`"user_prompt":"列出全部组织"`, `"org_units"`, `"org_code":"100000"`} {
		if !strings.Contains(text, snippet) {
			t.Fatalf("expected narration envelope to contain %q, got %s", snippet, text)
		}
	}
	for _, forbidden := range []string{"query_evidence_window", "200006", "未查询到任何成本组织"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("expected narration envelope to omit historical fact %q, got %s", forbidden, text)
		}
	}
}

func TestValidateQueryNarrationTextRejectsInternalLeakage(t *testing.T) {
	for _, text := range []string{
		"```json\n{\"results\":[{\"payload\":{\"name\":\"飞虫与鲜花\"}}]}\n```",
		"{\"results\":[{\"step_id\":\"step-1\",\"payload\":{\"org_unit\":{\"org_code\":\"100000\"}}}]}",
		"step-1 调用了 orgunit.details，result_focus 是 org_unit.name。",
		"请根据 params.org_code=100000 和 plan.steps[0] 继续执行。",
	} {
		if err := validateQueryNarrationText(text); !errors.Is(err, errCubeboxQueryNarrationContractViolation) {
			t.Fatalf("expected contract violation for %q, got %v", text, err)
		}
	}
}

func TestValidateQueryNarrationTextAllowsNaturalLanguage(t *testing.T) {
	for _, text := range []string{
		"截至 2026-04-24，组织 100000 是“飞虫与鲜花”，当前为启用状态，属于业务单元。系统里暂未记录它的上级组织和负责人，也没有扩展字段。",
		"组织 100000 在 2026-04-24 的详情如下：组织基本信息 - 名称：飞虫与鲜花；上级组织 - 未记录。",
		"状态：启用；是否业务单元：是；负责人：未记录。",
		"组织编码 100000\n生效日期 2026-04-25\n- 状态：启用\n- 上级组织：未记录",
	} {
		if err := validateQueryNarrationText(text); err != nil {
			t.Fatalf("expected natural language to pass for %q, got %v", text, err)
		}
	}
}

func TestFallbackCandidateClarificationTextContainsCandidates(t *testing.T) {
	text := fallbackCandidateClarificationText([]cubebox.QueryCandidate{
		{Domain: "orgunit", EntityKey: "1001", Name: "华东销售中心", Status: "active"},
		{Domain: "orgunit", EntityKey: "1002", Name: "华东运营中心", Status: "disabled"},
	})
	if !strings.Contains(text, "1001") || !strings.Contains(text, "1002") {
		t.Fatalf("unexpected clarification text=%q", text)
	}
}

func TestBuildExecutionClarificationTextPassesStructuredClarificationFacts(t *testing.T) {
	var got cubeboxQueryClarificationInput
	flow := &cubeboxQueryFlow{
		clarifier: cubeboxQueryClarifierStub{fn: func(_ context.Context, input cubeboxQueryClarificationInput) (string, error) {
			got = input
			return "请确认要继续查询的组织编码。", nil
		}},
	}
	text := flow.buildExecutionClarificationText(
		context.Background(),
		cubebox.GatewayStreamRequest{TenantID: "t1", Prompt: "查华东"},
		cubeboxAPIPlanProductionResult{ProviderID: "provider-a", ProviderType: "openai-compatible", ModelSlug: "gpt-5.2"},
		cubebox.QueryContext{},
		cubebox.QueryCandidateGroup{
			GroupID:            "candgrp_test_1",
			CandidateSource:    "execution_error",
			CandidateCount:     2,
			CannotSilentSelect: true,
			Candidates: []cubebox.QueryCandidate{
				{Domain: "orgunit", EntityKey: "1001", Name: "华东销售中心", AsOf: "2026-04-24"},
				{Domain: "orgunit", EntityKey: "1002", Name: "华东运营中心", AsOf: "2026-04-24"},
			},
		},
		"org_unit_search_ambiguous",
	)
	if text != "请确认要继续查询的组织编码。" {
		t.Fatalf("unexpected clarification text=%q", text)
	}
	if got.CandidateGroupID != "candgrp_test_1" || got.ErrorCode != "org_unit_search_ambiguous" || got.CandidateSource != "execution_error" || got.CandidateCount != 2 || !got.CannotSilentSelect {
		t.Fatalf("unexpected structured clarification input=%#v", got)
	}
}

func TestBuildExecutionClarificationTextProjectsClarifierCandidatesToBudget(t *testing.T) {
	var got cubeboxQueryClarificationInput
	rawCandidates := make([]cubebox.QueryCandidate, 0, 25)
	for i := 0; i < 25; i++ {
		rawCandidates = append(rawCandidates, cubebox.QueryCandidate{
			Domain:    "orgunit",
			EntityKey: "200" + strings.Repeat("0", 1) + string(rune('A'+(i%26))),
			Name:      "候选组织",
			AsOf:      "2026-04-24",
		})
	}
	flow := &cubeboxQueryFlow{
		clarifier: cubeboxQueryClarifierStub{fn: func(_ context.Context, input cubeboxQueryClarificationInput) (string, error) {
			got = input
			return "请确认。", nil
		}},
	}

	_ = flow.buildExecutionClarificationText(
		context.Background(),
		cubebox.GatewayStreamRequest{TenantID: "t1", Prompt: "查华东"},
		cubeboxAPIPlanProductionResult{ProviderID: "provider-a", ProviderType: "openai-compatible", ModelSlug: "gpt-5.2"},
		cubebox.QueryContext{
			RecentCandidateGroups: []cubebox.QueryCandidateGroup{
				{
					GroupID:         "candgrp_prev_1",
					CandidateSource: "execution_error",
					CandidateCount:  1,
					Candidates: []cubebox.QueryCandidate{
						{Domain: "orgunit", EntityKey: "1001", Name: "旧候选", AsOf: "2026-04-24"},
					},
				},
			},
		},
		cubebox.QueryCandidateGroup{
			GroupID:            "candgrp_test_1",
			CandidateSource:    "execution_error",
			CandidateCount:     len(rawCandidates),
			CannotSilentSelect: true,
			Candidates:         rawCandidates,
		},
		"org_unit_search_ambiguous",
	)

	if got.CandidateGroupID != "candgrp_test_1" {
		t.Fatalf("unexpected candidate group id=%#v", got)
	}
	if got.CandidateCount != len(rawCandidates) || !got.CannotSilentSelect {
		t.Fatalf("unexpected clarification metadata=%#v", got)
	}
	if gotLen, want := len(got.Candidates), queryContextMaxClarifierCandidatesPerGroup; gotLen != want {
		t.Fatalf("expected projected candidates length %d, got %d", want, gotLen)
	}
	if last := got.Candidates[len(got.Candidates)-1]; last.EntityKey != rawCandidates[queryContextMaxClarifierCandidatesPerGroup-1].EntityKey {
		t.Fatalf("expected candidates to preserve order under projection, got %#v", got.Candidates)
	}
}

func TestBuildPlannerMessagesIncludesQueryEvidenceWindow(t *testing.T) {
	producer := &cubeboxProviderAPIPlanProducer{
		now: func() time.Time { return time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC) },
	}

	messages := producer.buildPlannerMessages(cubeboxAPIPlanProductionInput{
		Prompt: "查该组织的下级组织",
		KnowledgePacks: []cubebox.KnowledgePack{
			{Dir: "modules/orgunit/presentation/cubebox", Files: map[string]string{"CUBEBOX-SKILL.md": "x", "queries.md": "x", "apis.md": "x", "examples.md": "x"}},
		},
		QueryContext: cubebox.QueryContext{
			RecentConfirmedEntities: []cubebox.QueryEntity{
				{
					Domain:            "orgunit",
					Intent:            "orgunit.details",
					EntityKey:         "100000",
					AsOf:              "2026-04-25",
					SourceOperationID: "orgunit.details",
				},
			},
			RecentCandidateGroups: []cubebox.QueryCandidateGroup{
				{
					GroupID:         "candgrp_test_1",
					CandidateSource: "execution_error",
					CandidateCount:  1,
					Candidates: []cubebox.QueryCandidate{
						{Domain: "orgunit", EntityKey: "200000", Name: "飞虫公司", AsOf: "2026-04-25"},
					},
				},
			},
		},
	})

	if len(messages) < 3 {
		t.Fatalf("expected planner messages, got %+v", messages)
	}
	found := false
	for _, message := range messages {
		if strings.Contains(message.Content, "query_evidence_window") {
			found = true
			if !strings.Contains(message.Content, "observations") || !strings.Contains(message.Content, "entity_fact") || !strings.Contains(message.Content, "presented_options") || !strings.Contains(message.Content, "100000") {
				t.Fatalf("unexpected context block=%q", message.Content)
			}
			for _, forbidden := range []string{`"intent":"orgunit.details"`, `"source_operation_id"`, `"target_org_code"`, `"parent_org_code"`, "recent_confirmed_entity", "recent_candidates", "recent_candidate_groups"} {
				if strings.Contains(message.Content, forbidden) {
					t.Fatalf("expected planner context to omit %q, got %q", forbidden, message.Content)
				}
			}
			for _, required := range []string{"不是本地目标绑定", "本地不会替你从历史上下文补单个 winner", "不会因为输入短而抢先拒绝"} {
				if !strings.Contains(message.Content, required) {
					t.Fatalf("expected planner context to contain %q, got %q", required, message.Content)
				}
			}
		}
	}
	if !found {
		t.Fatalf("expected query_evidence_window block, got %+v", messages)
	}
}

func TestBuildPlannerMessagesIncludesClarificationResume(t *testing.T) {
	producer := &cubeboxProviderAPIPlanProducer{
		now: func() time.Time { return time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC) },
	}
	messages := producer.buildPlannerMessages(cubeboxAPIPlanProductionInput{
		Prompt: "1日",
		KnowledgePacks: []cubebox.KnowledgePack{
			{Dir: "modules/orgunit/presentation/cubebox", Files: map[string]string{"CUBEBOX-SKILL.md": "x", "queries.md": "x", "apis.md": "x", "examples.md": "x"}},
		},
		QueryContext: cubebox.QueryContext{
			RecentDialogueTurns: []cubebox.QueryDialogueTurn{
				{UserPrompt: "查出顶级点的全部各级下级组织，时间节点是2025年1月", AssistantReply: "请提供完整查询日期，例如 2025-01-01。"},
			},
			LastClarification: &cubebox.QueryClarification{
				SourceTurnID:       "turn_prev",
				Intent:             "orgunit.list",
				MissingParams:      []string{"as_of"},
				ClarifyingQuestion: "请提供完整查询日期，例如 2025-01-01。",
			},
			ClarificationResume: &cubebox.QueryClarificationResume{
				ReplyCandidate:     true,
				SourceTurnID:       "turn_prev",
				Intent:             "orgunit.list",
				MissingParams:      []string{"as_of"},
				ClarifyingQuestion: "请提供完整查询日期，例如 2025-01-01。",
				RawUserReply:       "1日",
			},
		},
	})

	joined := plannerMessageText(messages)
	for _, expected := range []string{
		`"open_clarification"`,
		`"reply_candidate":true`,
		`"raw_user_reply":"1日"`,
		"不要因为输入短就抢先输出 NO_QUERY",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected planner prompt to contain %q, got %s", expected, joined)
		}
	}
}

func TestBuildPlannerMessagesIncludesRelativeMonthDayGuidance(t *testing.T) {
	producer := &cubeboxProviderAPIPlanProducer{
		now: func() time.Time { return time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC) },
	}
	messages := producer.buildPlannerMessages(cubeboxAPIPlanProductionInput{
		Prompt: "查询全部财务组织本月9日的详情",
		KnowledgePacks: []cubebox.KnowledgePack{
			{Dir: "modules/orgunit/presentation/cubebox", Files: map[string]string{"CUBEBOX-SKILL.md": "x", "queries.md": "x", "apis.md": "x", "examples.md": "x"}},
		},
	})

	joined := plannerMessageText(messages)
	for _, expected := range []string{
		"本月N日/这个月N号",
		"2026-04-09",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected planner prompt to contain %q, got %s", expected, joined)
		}
	}
}

func TestBuildPlannerMessagesGuidesKeywordListQueries(t *testing.T) {
	producer := &cubeboxProviderAPIPlanProducer{
		now: func() time.Time { return time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC) },
	}
	messages := producer.buildPlannerMessages(cubeboxAPIPlanProductionInput{
		Prompt: "列出全部包含成本关键字的组织",
		KnowledgePacks: []cubebox.KnowledgePack{
			{Dir: "modules/orgunit/presentation/cubebox", Files: map[string]string{
				"CUBEBOX-SKILL.md": "x",
				"queries.md":       mustReadTestFile(t, "modules/orgunit/presentation/cubebox/queries.md"),
				"apis.md":          "x",
				"examples.md":      mustReadTestFile(t, "modules/orgunit/presentation/cubebox/examples.md"),
			}},
		},
	})

	joined := plannerMessageText(messages)
	for _, expected := range []string{
		"列出全部包含成本关键字的组织",
		"keyword=成本",
		`"keyword": "成本"`,
		"不要填写 `parent_org_code`",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected planner prompt to contain %q, got %s", expected, joined)
		}
	}
}

func TestBuildPlannerMessagesGuidesCorrectionToOverrideHistoricalKeywordFilter(t *testing.T) {
	producer := &cubeboxProviderAPIPlanProducer{
		now: func() time.Time { return time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC) },
	}
	messages := producer.buildPlannerMessages(cubeboxAPIPlanProductionInput{
		Prompt: "不只是包含成本关键字的组织，而是全部的组织",
		QueryContext: cubebox.QueryContext{
			RecentDialogueTurns: []cubebox.QueryDialogueTurn{
				{
					UserPrompt:     "列出全部包含成本关键字的组织",
					AssistantReply: "名称包含“成本”关键字的组织共有 3 个：成本A组、成本B组、成本C组。",
				},
			},
			RecentCandidateGroups: []cubebox.QueryCandidateGroup{{
				CandidateSource: "results",
				CandidateCount:  3,
				Candidates: []cubebox.QueryCandidate{
					{Domain: "orgunit", EntityKey: "200005", Name: "成本A组", AsOf: "2026-04-27"},
					{Domain: "orgunit", EntityKey: "200006", Name: "成本B组", AsOf: "2026-04-27"},
					{Domain: "orgunit", EntityKey: "200007", Name: "成本C组", AsOf: "2026-04-27"},
				},
			}},
			RecentConfirmedEntities: []cubebox.QueryEntity{
				{Domain: "orgunit", Intent: "orgunit.details", EntityKey: "200007", AsOf: "2026-04-27", SourceOperationID: "orgunit.details"},
			},
		},
		KnowledgePacks: []cubebox.KnowledgePack{
			{Dir: "modules/orgunit/presentation/cubebox", Files: map[string]string{
				"CUBEBOX-SKILL.md": mustReadTestFile(t, "modules/orgunit/presentation/cubebox/CUBEBOX-SKILL.md"),
				"queries.md":       mustReadTestFile(t, "modules/orgunit/presentation/cubebox/queries.md"),
				"apis.md":          mustReadTestFile(t, "modules/orgunit/presentation/cubebox/apis.md"),
				"examples.md":      mustReadTestFile(t, "modules/orgunit/presentation/cubebox/examples.md"),
			}},
		},
		APITools: []cubebox.APITool{
			testCubeBoxAPITool("GET", "/org/api/org-units", "orgunit.list", []string{"as_of"}, []string{"include_disabled", "parent_org_code", "all_org_units", "keyword", "page", "page_size"}),
		},
	})

	joined := plannerMessageText(messages)
	for _, expected := range []string{
		"当前用户输入优先于历史事实",
		"不得继承历史缩窄参数、单个 entity_key 或 result_list target set",
		"不得继承历史 `keyword`、`parent_org_code`、`entity_key` 或 `result_list`",
		"不只是包含成本关键字的组织，而是全部的组织",
		"不得继承历史 `keyword=成本`",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected planner prompt to contain %q, got %s", expected, joined)
		}
	}
	for _, forbidden := range []string{
		`"kind":"result_list"`,
		`"kind":"entity_fact"`,
		`"entity_key":"200007"`,
		`"parent_org_code":"200006"`,
	} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("expected scope override prompt to omit historical target %q, got %s", forbidden, joined)
		}
	}
}

func TestBuildPlannerMessagesSharedGuidanceDoesNotNameOrgunitFields(t *testing.T) {
	producer := &cubeboxProviderAPIPlanProducer{
		now: func() time.Time { return time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC) },
	}
	messages := producer.buildPlannerMessages(cubeboxAPIPlanProductionInput{
		Prompt: "不只是这个对象，而是全部",
		KnowledgePacks: []cubebox.KnowledgePack{
			{Dir: "modules/sample/presentation/cubebox", Files: map[string]string{
				"CUBEBOX-SKILL.md": "# Skill\n\nqueries.md\napis.md\nexamples.md\n",
				"queries.md":       "```yaml\nintents:\n  - key: sample.list\n    required_params: [as_of]\n    optional_params: [all_samples]\n```\n",
				"apis.md":          "```yaml\napi_tools:\n  - operation_id: sample.list\n    method: GET\n    path: /sample/api/samples\n    required_params: [as_of]\n    optional_params: [all_samples]\n```\n",
				"examples.md":      "```json\n{\"outcome\":\"API_CALLS\",\"calls\":[{\"id\":\"step-1\",\"method\":\"GET\",\"path\":\"/sample/api/samples\",\"params\":{\"as_of\":\"2026-04-25\"},\"depends_on\":[]}]}\n```\n",
			}},
		},
	})

	if len(messages) == 0 {
		t.Fatal("expected planner messages")
	}
	sharedSystemPrompt := messages[0].Content
	for _, forbidden := range []string{"parent_org_code", "all_org_units", "orgunit.list", "all org", "all organization", "all organisations", "all organizations"} {
		if strings.Contains(sharedSystemPrompt, forbidden) {
			t.Fatalf("shared planner prompt leaked module term %q: %s", forbidden, sharedSystemPrompt)
		}
	}
	if !strings.Contains(sharedSystemPrompt, "历史缩窄参数") {
		t.Fatalf("expected neutral scope wording, got %s", sharedSystemPrompt)
	}
}

func TestBuildQueryEvidenceWindowDropsHistoricalTargetsWhenPromptOverridesScope(t *testing.T) {
	window := buildQueryEvidenceWindow(cubebox.QueryContext{
		RecentDialogueTurns: []cubebox.QueryDialogueTurn{
			{UserPrompt: "列出全部包含成本关键字的组织", AssistantReply: "成本A组、成本B组、成本C组。"},
		},
		RecentConfirmedEntities: []cubebox.QueryEntity{
			{Domain: "orgunit", Intent: "orgunit.details", EntityKey: "200007", AsOf: "2026-04-27"},
		},
		RecentCandidateGroups: []cubebox.QueryCandidateGroup{{
			CandidateSource: "results",
			CandidateCount:  1,
			Candidates: []cubebox.QueryCandidate{
				{Domain: "orgunit", EntityKey: "200007", Name: "成本C组", AsOf: "2026-04-27"},
			},
		}},
	}, "不只是包含成本关键字的组织，而是全部的组织", cubeboxQueryEvidenceWindowProjectionBudget{
		MaxConfirmedEntities:  5,
		MaxCandidateGroups:    5,
		MaxCandidatesPerGroup: 100,
		MaxDialogueTurns:      5,
	})

	if window.CurrentUserInput != "不只是包含成本关键字的组织，而是全部的组织" {
		t.Fatalf("unexpected current user input=%q", window.CurrentUserInput)
	}
	if len(window.RecentTurns) != 0 || len(window.Observations) != 0 || window.OpenClarification != nil {
		t.Fatalf("expected historical targets omitted for scope override, got %+v", window)
	}
}

func TestBuildPlannerMessagesTreatsPaginationAsControlDefaults(t *testing.T) {
	producer := &cubeboxProviderAPIPlanProducer{
		now: func() time.Time { return time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC) },
	}
	messages := producer.buildPlannerMessages(cubeboxAPIPlanProductionInput{
		Prompt: "今天全部组织",
		KnowledgePacks: []cubebox.KnowledgePack{
			{Dir: "modules/orgunit/presentation/cubebox", Files: map[string]string{
				"CUBEBOX-SKILL.md": mustReadTestFile(t, "modules/orgunit/presentation/cubebox/CUBEBOX-SKILL.md"),
				"queries.md":       mustReadTestFile(t, "modules/orgunit/presentation/cubebox/queries.md"),
				"apis.md":          mustReadTestFile(t, "modules/orgunit/presentation/cubebox/apis.md"),
				"examples.md":      mustReadTestFile(t, "modules/orgunit/presentation/cubebox/examples.md"),
			}},
		},
		APITools: []cubebox.APITool{
			testCubeBoxAPITool("GET", "/org/api/org-units", "orgunit.list", []string{"as_of"}, []string{"include_disabled", "all_org_units", "page", "page_size"}),
		},
	})

	joined := plannerMessageText(messages)
	for _, expected := range []string{
		"page/page_size 是分页执行控制，不是业务必填参数",
		"缺省时默认 page=1,page_size=100",
		"不要向用户追问 page=1,page_size=100",
		"all_org_units=true",
		"`page` / `page_size` 是执行控制参数，不是业务必填参数",
		"未指定分页时默认 `page=1,page_size=100`",
		"用户只提供一个正整数作为分页短答时",
		"page=1 表示第一页",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected planner prompt to contain %q, got %s", expected, joined)
		}
	}
}

func TestQueryFlowDoesNotRebuildClosedClarificationResume(t *testing.T) {
	var plannerCalls int
	flow := &cubeboxQueryFlow{
		runtime: cubebox.NewRuntime(),
		store: cubeboxStoreStub{
			appendFn: func(context.Context, string, string, string, cubebox.CanonicalEvent) error { return nil },
			getFn: func(context.Context, string, string, string) (cubebox.ConversationReplayResponse, error) {
				return cubebox.ConversationReplayResponse{
					Conversation: cubebox.Conversation{ID: "conv-1"},
					Events: []cubebox.CanonicalEvent{
						{
							Type: "turn.user_message.accepted",
							Payload: map[string]any{
								"text": "查出顶级点的全部各级下级组织，时间节点是2025年1月",
							},
						},
						{
							Type: "turn.agent_message.delta",
							Payload: map[string]any{
								"message_id": "msg_agent_1",
								"delta":      "请提供完整查询日期，例如 2025-01-01。",
							},
						},
						{
							Type: "turn.agent_message.completed",
							Payload: map[string]any{
								"message_id": "msg_agent_1",
							},
						},
						{
							Type:   cubebox.QueryClarificationRequestedEventType,
							TurnID: turnIDPtrForTest("turn_prev"),
							Payload: map[string]any{
								"intent":              "orgunit.list",
								"missing_params":      []string{"as_of"},
								"clarifying_question": "请提供完整查询日期，例如 2025-01-01。",
							},
						},
						{
							Type: "turn.user_message.accepted",
							Payload: map[string]any{
								"text": "1日",
							},
						},
					},
				}, nil
			},
			preparePromptViewFn: func(context.Context, string, string, string, cubebox.CanonicalContext, string) (cubebox.PromptViewPreparationResponse, error) {
				return cubebox.PromptViewPreparationResponse{NextSequence: 1}, nil
			},
		},
		runner: cubeboxAPIToolRunnerStub{},
		producer: cubeboxAPIPlanProducerStub{fn: func(_ context.Context, input cubeboxAPIPlanProductionInput) (cubeboxAPIPlanProductionResult, error) {
			plannerCalls++
			if input.QueryContext.ClarificationResume != nil {
				t.Fatalf("expected no clarification resume after prior clarification already consumed, got %#v", input.QueryContext.ClarificationResume)
			}
			return cubeboxAPIPlanProductionResult{
				Handled:         false,
				Outcome:         cubebox.PlannerOutcome{Type: cubebox.PlannerOutcomeNoQuery},
				ProviderID:      "openai-compatible",
				ProviderType:    "openai-compatible",
				ModelSlug:       "gpt-5.2",
				ExplicitOutcome: true,
			}, nil
		}},
		narrator: cubeboxQueryNarratorStub{},
		knowledgePacks: []cubebox.KnowledgePack{
			{Dir: "modules/orgunit/presentation/cubebox", Files: map[string]string{"CUBEBOX-SKILL.md": "x", "queries.md": "x", "apis.md": "x", "examples.md": "x"}},
		},
		now: func() time.Time { return time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC) },
	}

	sink := &capturingGatewaySink{}
	if handled := flow.TryHandle(context.Background(), queryGatewayRequest("那查一下华东销售中心"), sink); !handled {
		t.Fatal("expected handled")
	}
	if plannerCalls != 1 {
		t.Fatalf("expected 1 planner call, got %d", plannerCalls)
	}
}

func TestQueryFlowInjectsCandidateClarificationResumeIntoPlannerInput(t *testing.T) {
	var plannerCalls int
	flow := &cubeboxQueryFlow{
		runtime: cubebox.NewRuntime(),
		store: cubeboxStoreStub{
			appendFn: func(context.Context, string, string, string, cubebox.CanonicalEvent) error { return nil },
			getFn: func(context.Context, string, string, string) (cubebox.ConversationReplayResponse, error) {
				return cubebox.ConversationReplayResponse{
					Conversation: cubebox.Conversation{ID: "conv-1"},
					Events: []cubebox.CanonicalEvent{
						{
							Type: "turn.user_message.accepted",
							Payload: map[string]any{
								"text": "列出全部财务组织的详情",
							},
						},
						{
							Type: cubebox.QueryCandidatesPresentedEventType,
							Payload: map[string]any{
								"group_id":             "candgrp_finance",
								"candidate_source":     "execution_error",
								"candidate_count":      3,
								"cannot_silent_select": true,
								"candidates": []any{
									map[string]any{"domain": "orgunit", "entity_key": "200001", "name": "财务部", "as_of": "2026-04-25"},
									map[string]any{"domain": "orgunit", "entity_key": "200002", "name": "财务一组", "as_of": "2026-04-25"},
									map[string]any{"domain": "orgunit", "entity_key": "200004", "name": "财务四组", "as_of": "2026-04-25"},
								},
							},
						},
						{
							Type:   cubebox.QueryClarificationRequestedEventType,
							TurnID: turnIDPtrForTest("turn_prev"),
							Payload: map[string]any{
								"clarifying_question":  "找到了多个候选项，请确认要继续查询哪一个。",
								"candidate_group_id":   "candgrp_finance",
								"candidate_source":     "execution_error",
								"candidate_count":      3,
								"cannot_silent_select": true,
							},
						},
					},
				}, nil
			},
			preparePromptViewFn: func(context.Context, string, string, string, cubebox.CanonicalContext, string) (cubebox.PromptViewPreparationResponse, error) {
				return cubebox.PromptViewPreparationResponse{NextSequence: 1}, nil
			},
		},
		runner: cubeboxAPIToolRunnerStub{},
		producer: cubeboxAPIPlanProducerStub{fn: func(_ context.Context, input cubeboxAPIPlanProductionInput) (cubeboxAPIPlanProductionResult, error) {
			plannerCalls++
			resume := input.QueryContext.ClarificationResume
			if resume == nil {
				t.Fatalf("expected clarification resume, got %#v", input.QueryContext)
			}
			if !resume.ReplyCandidate || resume.RawUserReply != "全部" {
				t.Fatalf("unexpected clarification resume=%#v", resume)
			}
			if resume.CandidateGroupID != "candgrp_finance" || resume.CandidateCount != 3 || !resume.CannotSilentSelect {
				t.Fatalf("unexpected candidate clarification resume=%#v", resume)
			}
			if got := len(resume.Candidates); got != 3 {
				t.Fatalf("expected resume candidates, got %d in %#v", got, resume)
			}
			if resume.Candidates[0].EntityKey != "200001" || resume.Candidates[2].EntityKey != "200004" {
				t.Fatalf("unexpected resume candidates=%#v", resume.Candidates)
			}
			return cubeboxAPIPlanProductionResult{
				Handled:         false,
				Outcome:         cubebox.PlannerOutcome{Type: cubebox.PlannerOutcomeNoQuery},
				ProviderID:      "openai-compatible",
				ProviderType:    "openai-compatible",
				ModelSlug:       "gpt-5.2",
				ExplicitOutcome: true,
			}, nil
		}},
		narrator: cubeboxQueryNarratorStub{},
		knowledgePacks: []cubebox.KnowledgePack{
			{Dir: "modules/orgunit/presentation/cubebox", Files: map[string]string{"CUBEBOX-SKILL.md": "x", "queries.md": "x", "apis.md": "x", "examples.md": "x"}},
		},
		now: func() time.Time { return time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC) },
	}

	sink := &capturingGatewaySink{}
	if handled := flow.TryHandle(context.Background(), queryGatewayRequest("全部"), sink); !handled {
		t.Fatal("expected handled")
	}
	if plannerCalls != 1 {
		t.Fatalf("expected 1 planner call, got %d", plannerCalls)
	}
}

func TestCubeboxProviderAPIPlanProducerRejectsBareDone(t *testing.T) {
	producer := &cubeboxProviderAPIPlanProducer{
		configReader: cubeboxRuntimeConfigReaderStub{config: cubebox.ActiveModelRuntimeConfig{
			Provider:   cubebox.ModelProvider{ID: "provider-a", ProviderType: "openai-compatible", BaseURL: "https://example.com", Enabled: true},
			Selection:  cubebox.ActiveModelSelection{ModelSlug: "gpt-5.2"},
			Credential: cubebox.ModelCredential{SecretRef: "env://OPENAI_API_KEY"},
		}},
		adapter: &cubeboxProviderAdapterStub{stream: &cubeboxProviderChatStreamTextStub{chunks: []cubebox.ProviderChatChunk{
			{Delta: "DONE"},
			{Done: true},
		}}},
		secretResolver: cubeboxSecretResolverStub{secret: "sk-test"},
		now:            func() time.Time { return time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC) },
	}

	_, err := producer.ProduceAPIPlan(context.Background(), cubeboxAPIPlanProductionInput{
		TenantID: "tenant-a",
		Prompt:   "查组织树",
	})
	if !errors.Is(err, cubebox.ErrPlannerOutcomeInvalid) {
		t.Fatalf("expected planner outcome invalid for bare DONE, got %v", err)
	}
}

func TestCubeboxProviderQueryNarratorNarratesNoQueryGuidance(t *testing.T) {
	adapter := &cubeboxProviderAdapterStub{
		stream: &cubeboxProviderChatStreamTextStub{
			chunks: []cubebox.ProviderChatChunk{
				{Delta: "当前主要支持组织相关只读查询。\n\n你可以直接这样问：\n1. 查“华东销售中心”的详情\n2. 查“华东销售中心”当前的下级组织\n3. 搜索名称包含“销售”的组织"},
				{Done: true},
			},
		},
	}
	narrator := &cubeboxProviderQueryNarrator{
		configReader: cubeboxRuntimeConfigReaderStub{config: cubebox.ActiveModelRuntimeConfig{
			Provider:   cubebox.ModelProvider{ID: "provider-a", ProviderType: "openai-compatible", BaseURL: "https://example.com", Enabled: true},
			Selection:  cubebox.ActiveModelSelection{ModelSlug: "gpt-5.2"},
			Credential: cubebox.ModelCredential{SecretRef: "env://OPENAI_API_KEY"},
		}},
		adapter:        adapter,
		secretResolver: cubeboxSecretResolverStub{secret: "sk-test"},
	}

	text, err := narrator.NarrateNoQueryGuidance(context.Background(), cubeboxNoQueryGuidanceInput{
		TenantID:             "tenant-a",
		ScopeSummary:         "当前主要支持组织相关只读查询。",
		SuggestedPrompts:     []string{"查“华东销售中心”的详情", "查“华东销售中心”当前的下级组织", "搜索名称包含“销售”的组织"},
		ExpectedProviderID:   "provider-a",
		ExpectedProviderType: "openai-compatible",
		ExpectedModelSlug:    "gpt-5.2",
	})
	if err != nil {
		t.Fatalf("NarrateNoQueryGuidance err=%v", err)
	}
	if !strings.Contains(text, "你可以直接这样问：") {
		t.Fatalf("unexpected guidance text=%q", text)
	}
	if adapter.lastRequest.Input == "" || !strings.Contains(adapter.lastRequest.Input, `"suggested_prompts"`) {
		t.Fatalf("expected structured guidance facts as input, got %#v", adapter.lastRequest)
	}
	if !strings.Contains(plannerMessageText(adapter.lastRequest.Messages), "只能使用 provided suggested_prompts") {
		t.Fatalf("unexpected guidance messages=%#v", adapter.lastRequest.Messages)
	}
}

type capturingGatewaySink struct {
	events []cubebox.CanonicalEvent
}

type cubeboxQueryClarifierStub struct {
	fn func(context.Context, cubeboxQueryClarificationInput) (string, error)
}

func (s cubeboxQueryClarifierStub) ClarifyQuery(ctx context.Context, input cubeboxQueryClarificationInput) (string, error) {
	if s.fn == nil {
		return "", nil
	}
	return s.fn(ctx, input)
}

func (s *capturingGatewaySink) Write(event cubebox.CanonicalEvent) bool {
	s.events = append(s.events, event)
	return true
}

func (s *capturingGatewaySink) WriteFallback(event cubebox.CanonicalEvent) {
	s.events = append(s.events, event)
}

func (s *capturingGatewaySink) deltas() []string {
	out := make([]string, 0)
	for _, event := range s.events {
		if event.Type != "turn.agent_message.delta" {
			continue
		}
		if delta, ok := event.Payload["delta"].(string); ok {
			out = append(out, delta)
		}
	}
	return out
}

func (s *capturingGatewaySink) hasErrorCode(code string) bool {
	if s == nil {
		return false
	}
	for _, event := range s.events {
		if event.Type != "turn.error" {
			continue
		}
		if got, ok := event.Payload["code"].(string); ok && got == code {
			return true
		}
	}
	return false
}

func plannerMessageText(messages []cubebox.PromptItem) string {
	parts := make([]string, 0, len(messages))
	for _, message := range messages {
		parts = append(parts, message.Content)
	}
	return strings.Join(parts, "\n")
}

func queryGatewayRequest(prompt string) cubebox.GatewayStreamRequest {
	return cubebox.GatewayStreamRequest{
		TenantID:       "t1",
		PrincipalID:    "p1",
		ConversationID: "conv-1",
		Prompt:         prompt,
		NextSequence:   1,
	}
}

func queryAPIPlanForOrgUnitList(asOf string, parentOrgCode string) cubebox.APICallPlan {
	params := map[string]any{"as_of": asOf}
	if strings.TrimSpace(parentOrgCode) != "" {
		params["parent_org_code"] = strings.TrimSpace(parentOrgCode)
	}
	return queryAPIPlanForOrgUnitListWithParams(params)
}

func queryAPIPlanForOrgUnitListWithParams(params map[string]any) cubebox.APICallPlan {
	copied := make(map[string]any, len(params))
	for key, value := range params {
		copied[key] = value
	}
	return cubebox.APICallPlan{
		Calls: []cubebox.APICallStep{{
			ID:          "step-1",
			Method:      "GET",
			Path:        "/org/api/org-units",
			Params:      copied,
			ResultFocus: []string{"org_units[].org_code", "org_units[].has_children"},
			DependsOn:   []string{},
		}},
	}
}

func allOrgScopeCorrectionHistory(context.Context, string, string, string) (cubebox.ConversationReplayResponse, error) {
	return cubebox.ConversationReplayResponse{
		Conversation: cubebox.Conversation{ID: "conv-1", Title: "组织查询", Status: "active"},
		Events: []cubebox.CanonicalEvent{
			{Type: "turn.user_message.accepted", Payload: map[string]any{"text": "列出全部包含成本关键字的组织"}},
			{Type: "turn.query_candidates.presented", Payload: map[string]any{
				"candidate_source": "results",
				"candidate_count":  float64(3),
				"candidates": []any{
					map[string]any{"domain": "orgunit", "entity_key": "200005", "name": "成本A组", "as_of": "2026-04-27"},
					map[string]any{"domain": "orgunit", "entity_key": "200006", "name": "成本B组", "as_of": "2026-04-27"},
					map[string]any{"domain": "orgunit", "entity_key": "200007", "name": "成本C组", "as_of": "2026-04-27"},
				},
			}},
			{Type: "turn.agent_message.delta", Payload: map[string]any{"message_id": "msg_agent_1", "delta": "名称包含“成本”关键字的组织共有 3 个。"}},
			{Type: "turn.agent_message.completed", Payload: map[string]any{"message_id": "msg_agent_1"}},
		},
	}, nil
}

func mustReadTestFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(mustResolveRepoPath(path))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}

func queryPlannerAPICallsResult(plan cubebox.APICallPlan) cubeboxAPIPlanProductionResult {
	return cubeboxAPIPlanProductionResult{
		Handled: true,
		Outcome: cubebox.PlannerOutcome{
			Type:  cubebox.PlannerOutcomeAPICalls,
			Calls: plan,
		},
		Plan:            plan,
		ProviderID:      "openai-compatible",
		ProviderType:    "openai-compatible",
		ModelSlug:       "gpt-5.2",
		ExplicitOutcome: true,
	}
}

func queryPlannerDoneResult() cubeboxAPIPlanProductionResult {
	return cubeboxAPIPlanProductionResult{
		Handled:         true,
		Outcome:         cubebox.PlannerOutcome{Type: cubebox.PlannerOutcomeDone},
		ProviderID:      "openai-compatible",
		ProviderType:    "openai-compatible",
		ModelSlug:       "gpt-5.2",
		ExplicitOutcome: true,
	}
}

func queryLoopTestFlow(runner cubeboxAPIToolRunner, producer cubeboxAPIPlanProducerStub, narrator cubeboxQueryNarratorStub) *cubeboxQueryFlow {
	if runner == nil {
		runner = cubeboxAPIToolRunnerStub{}
	}
	return &cubeboxQueryFlow{
		runtime: cubebox.NewRuntime(),
		store: cubeboxStoreStub{
			appendFn: func(context.Context, string, string, string, cubebox.CanonicalEvent) error { return nil },
		},
		runner:   runner,
		producer: producer,
		narrator: narrator,
		knowledgePacks: []cubebox.KnowledgePack{
			{Dir: "modules/orgunit/presentation/cubebox", Files: map[string]string{"CUBEBOX-SKILL.md": "x", "queries.md": "x", "apis.md": "x", "examples.md": "x"}},
		},
		now: func() time.Time { return time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC) },
	}
}

func fakeModuleKnowledgePack(dir string, operationID string, requiredParams []string, scopeSummary string, prompts []string) cubebox.KnowledgePack {
	path := "/" + strings.TrimPrefix(strings.ReplaceAll(operationID, ".", "/api/"), "/")
	return cubebox.KnowledgePack{
		Dir: dir,
		Files: map[string]string{
			"CUBEBOX-SKILL.md": "# Skill\n\nqueries.md\napis.md\nexamples.md\n",
			"queries.md":       "```yaml\nintents:\n  - key: " + operationID + "\n    required_params: [" + strings.Join(requiredParams, ", ") + "]\n    optional_params: []\nno_query_guidance:\n  scope_summary: " + scopeSummary + "\n  suggested_prompts:\n" + yamlPromptList(prompts) + "```\n",
			"apis.md":          "```yaml\napi_tools:\n  - operation_id: " + operationID + "\n    method: GET\n    path: " + path + "\n    required_params: [" + strings.Join(requiredParams, ", ") + "]\n    optional_params: []\n```\n",
			"examples.md":      "```json\n{\"outcome\":\"API_CALLS\",\"calls\":[{\"id\":\"step-1\",\"method\":\"GET\",\"path\":\"" + path + "\",\"params\":{\"" + requiredParams[0] + "\":\"S-100\"},\"depends_on\":[]}]}\n```\n",
		},
	}
}

func yamlPromptList(items []string) string {
	if len(items) == 0 {
		return ""
	}
	var out strings.Builder
	for _, item := range items {
		out.WriteString("    - ")
		out.WriteString(item)
		out.WriteString("\n")
	}
	return out.String()
}

func TestQueryFlowAggregatesNoQueryGuidanceAcrossKnowledgePacks(t *testing.T) {
	flow := queryLoopTestFlow(cubeboxAPIToolRunnerStub{}, cubeboxAPIPlanProducerStub{result: cubeboxAPIPlanProductionResult{
		Handled:         false,
		ProviderID:      "openai-compatible",
		ProviderType:    "openai-compatible",
		ModelSlug:       "gpt-5.2",
		ExplicitOutcome: true,
	}}, cubeboxQueryNarratorStub{noQueryFn: func(_ context.Context, input cubeboxNoQueryGuidanceInput) (string, error) {
		return fallbackNoQueryGuidanceText(buildNoQueryGuidanceEnvelope(input)), nil
	}})
	flow.knowledgePacks = []cubebox.KnowledgePack{
		fakeModuleKnowledgePack("modules/orgunit/presentation/cubebox", "orgunit.details", []string{"org_code"}, "当前主要支持组织相关只读查询。", []string{"查“华东销售中心”的详情", "搜索名称包含“销售”的组织"}),
		fakeModuleKnowledgePack("modules/sample/presentation/cubebox", "sample.details", []string{"sample_id"}, "也支持样例对象只读查询。", []string{"查样例对象 S-100 的详情", "搜索名称包含“固定资产”的样例对象", "搜索名称包含“销售”的组织"}),
	}

	sink := &capturingGatewaySink{}
	if handled := flow.TryHandle(context.Background(), queryGatewayRequest("帮我写封邮件"), sink); !handled {
		t.Fatal("expected handled")
	}
	text := strings.Join(sink.deltas(), "\n")
	for _, snippet := range []string{
		"当前主要支持组织相关只读查询。 也支持样例对象只读查询。",
		"1. 查“华东销售中心”的详情",
		"2. 搜索名称包含“销售”的组织",
		"3. 查样例对象 S-100 的详情",
		"4. 搜索名称包含“固定资产”的样例对象",
	} {
		if !strings.Contains(text, snippet) {
			t.Fatalf("expected aggregated guidance snippet %q, got %q", snippet, text)
		}
	}
}

func TestCubeboxProviderQueryNarratorBuildsStrictMessagesAndRejectsInternalLeakage(t *testing.T) {
	adapter := &cubeboxProviderAdapterStub{
		stream: &cubeboxProviderChatStreamTextStub{
			chunks: []cubebox.ProviderChatChunk{
				{Delta: "{\"results\":[{\"step_id\":\"step-1\",\"payload\":{\"org_unit\":{\"org_code\":\"100000\"}}}]}"},
				{Done: true},
			},
		},
	}
	narrator := &cubeboxProviderQueryNarrator{
		configReader: cubeboxRuntimeConfigReaderStub{config: cubebox.ActiveModelRuntimeConfig{
			Provider:   cubebox.ModelProvider{ID: "provider-a", ProviderType: "openai-compatible", BaseURL: "https://example.com", Enabled: true},
			Selection:  cubebox.ActiveModelSelection{ModelSlug: "gpt-5.2"},
			Credential: cubebox.ModelCredential{SecretRef: "env://OPENAI_API_KEY"},
		}},
		adapter:        adapter,
		secretResolver: cubeboxSecretResolverStub{secret: "sk-test"},
	}

	_, err := narrator.NarrateQueryResult(context.Background(), cubeboxQueryNarrationInput{
		TenantID: "tenant-a",
		Prompt:   "查一下 100000 在 2026-04-24 的组织详情",
		Results: []cubebox.QueryNarrationResult{{
			Domain: "orgunit",
			Data: map[string]any{
				"org_unit": map[string]any{
					"org_code":        "100000",
					"name":            "飞虫与鲜花",
					"status":          "active",
					"parent_org_code": "ROOT",
				},
				"as_of": "2026-04-24",
			},
		}},
		QueryContext: cubebox.QueryContext{
			RecentConfirmedEntities: []cubebox.QueryEntity{
				{
					Domain:            "orgunit",
					Intent:            "orgunit.details",
					EntityKey:         "100000",
					AsOf:              "2026-04-24",
					SourceOperationID: "orgunit.details",
				},
			},
			RecentCandidateGroups: []cubebox.QueryCandidateGroup{
				{
					GroupID:         "candgrp_test_1",
					CandidateSource: "results",
					CandidateCount:  1,
					Candidates: []cubebox.QueryCandidate{
						{Domain: "orgunit", EntityKey: "100000", Name: "飞虫与鲜花", AsOf: "2026-04-24"},
					},
				},
			},
			RecentDialogueTurns: []cubebox.QueryDialogueTurn{
				{UserPrompt: "查总部", AssistantReply: "总部是 100000。"},
			},
		},
		ExpectedProviderID:   "provider-a",
		ExpectedProviderType: "openai-compatible",
		ExpectedModelSlug:    "gpt-5.2",
	})
	if !errors.Is(err, errCubeboxQueryNarrationContractViolation) {
		t.Fatalf("expected contract violation, got %v", err)
	}
	if len(adapter.lastRequest.Messages) != 2 {
		t.Fatalf("expected 2 narrator messages, got %+v", adapter.lastRequest.Messages)
	}
	systemPrompt := adapter.lastRequest.Messages[0].Content
	if !strings.Contains(systemPrompt, "不得逐字回显整份原始 JSON") {
		t.Fatalf("expected strict narrator prompt, got %q", systemPrompt)
	}
	for _, snippet := range []string{
		"可以是短答、分段、小标题或列表",
		"params.org_code",
		"plan.steps",
	} {
		if !strings.Contains(systemPrompt, snippet) {
			t.Fatalf("expected narrator prompt to contain %q, got %q", snippet, systemPrompt)
		}
	}
	body := adapter.lastRequest.Messages[1].Content
	for _, snippet := range []string{
		`"org_unit"`,
		`"org_code":"100000"`,
		`"parent_org_code":"ROOT"`,
		`"as_of":"2026-04-24"`,
	} {
		if !strings.Contains(body, snippet) {
			t.Fatalf("expected narrator body to contain %q, got %q", snippet, body)
		}
	}
	for _, forbidden := range []string{
		`"step_id":"step-1"`,
		`"operation_id":"orgunit.details"`,
		`"payload":`,
		`"plan":`,
		`"query_evidence_window"`,
		`"entity_key":"100000"`,
		`"executed_steps"`,
		`"resolved_entity"`,
		`"source_operation_id"`,
		`"target_org_code"`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("expected narrator body to omit raw execution envelope %q, got %q", forbidden, body)
		}
	}
}
