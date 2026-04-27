package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
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
				{Domain: "orgunit", Intent: "orgunit.search", EntityKey: "1001", AsOf: "2026-04-24", SourceAPIKey: "orgunit.search", TargetOrgCode: "1001"},
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
		`"source_api_key"`,
		`"target_org_code"`,
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("expected clarification body to omit %q, got %q", forbidden, text)
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
				{Domain: "orgunit", Intent: "orgunit.list", EntityKey: "200006", AsOf: "2026-04-27", ParentOrgCode: "200006"},
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
		cubeboxReadPlanProductionResult{ProviderID: "provider-a", ProviderType: "openai-compatible", ModelSlug: "gpt-5.2"},
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
		cubeboxReadPlanProductionResult{ProviderID: "provider-a", ProviderType: "openai-compatible", ModelSlug: "gpt-5.2"},
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

func TestQueryFlowReturnsPlannerClarificationVerbatim(t *testing.T) {
	flow := &cubeboxQueryFlow{
		runtime: cubebox.NewRuntime(),
		store: cubeboxStoreStub{
			appendFn: func(context.Context, string, string, string, cubebox.CanonicalEvent) error { return nil },
			preparePromptViewFn: func(context.Context, string, string, string, cubebox.CanonicalContext, string) (cubebox.PromptViewPreparationResponse, error) {
				return cubebox.PromptViewPreparationResponse{NextSequence: 1}, nil
			},
		},
		registry: &cubebox.ExecutionRegistry{},
		producer: cubeboxReadPlanProducerStub{result: cubeboxReadPlanProductionResult{
			Handled: true,
			Plan: cubebox.ReadPlan{
				Intent:             "orgunit.list",
				Confidence:         0.4,
				MissingParams:      []string{"parent_org_code"},
				ClarifyingQuestion: "请提供 parent_org_code。",
			},
			ProviderID:   "openai-compatible",
			ProviderType: "openai-compatible",
			ModelSlug:    "gpt-5.2",
		}},
		narrator: cubeboxQueryNarratorStub{fn: func(context.Context, cubeboxQueryNarrationInput) (string, error) {
			t.Fatal("narrator should not be called for clarification")
			return "", nil
		}},
		knowledgePacks: []cubebox.KnowledgePack{
			{Dir: "modules/orgunit/presentation/cubebox", Files: map[string]string{"CUBEBOX-SKILL.md": "x", "queries.md": "x", "apis.md": "x", "examples.md": "x"}},
		},
		now: func() time.Time { return time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC) },
	}
	sink := &capturingGatewaySink{}

	handled := flow.TryHandle(context.Background(), cubebox.GatewayStreamRequest{
		TenantID:       "t1",
		PrincipalID:    "p1",
		ConversationID: "conv-1",
		Prompt:         "看华东事业部下面的子组织",
		NextSequence:   1,
	}, sink)
	if !handled {
		t.Fatal("expected handled")
	}
	if !strings.Contains(strings.Join(sink.deltas(), "\n"), "请提供 parent_org_code。") {
		t.Fatalf("expected verbatim clarification, got %+v", sink.events)
	}
}

func TestBuildPlannerMessagesIncludesQueryEvidenceWindow(t *testing.T) {
	producer := &cubeboxProviderReadPlanProducer{
		now: func() time.Time { return time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC) },
	}

	messages := producer.buildPlannerMessages(cubeboxReadPlanProductionInput{
		Prompt: "查该组织的下级组织",
		KnowledgePacks: []cubebox.KnowledgePack{
			{Dir: "modules/orgunit/presentation/cubebox", Files: map[string]string{"CUBEBOX-SKILL.md": "x", "queries.md": "x", "apis.md": "x", "examples.md": "x"}},
		},
		QueryContext: cubebox.QueryContext{
			RecentConfirmedEntities: []cubebox.QueryEntity{
				{
					Domain:        "orgunit",
					Intent:        "orgunit.details",
					EntityKey:     "100000",
					AsOf:          "2026-04-25",
					SourceAPIKey:  "orgunit.details",
					TargetOrgCode: "100000",
					ParentOrgCode: "ROOT",
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
			for _, forbidden := range []string{`"intent":"orgunit.details"`, `"source_api_key"`, `"target_org_code"`, `"parent_org_code"`, "recent_confirmed_entity", "recent_candidates", "recent_candidate_groups"} {
				if strings.Contains(message.Content, forbidden) {
					t.Fatalf("expected planner context to omit %q, got %q", forbidden, message.Content)
				}
			}
			for _, required := range []string{"不是本地目标绑定", "本地不会替你从历史上下文补 target", "不会因为输入短而抢先拒绝"} {
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
	producer := &cubeboxProviderReadPlanProducer{
		now: func() time.Time { return time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC) },
	}
	messages := producer.buildPlannerMessages(cubeboxReadPlanProductionInput{
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
	producer := &cubeboxProviderReadPlanProducer{
		now: func() time.Time { return time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC) },
	}
	messages := producer.buildPlannerMessages(cubeboxReadPlanProductionInput{
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

func TestQueryFlowInjectsClarificationResumeIntoPlannerInput(t *testing.T) {
	registry, err := cubebox.NewExecutionRegistry(cubebox.RegisteredExecutor{
		APIKey:         "orgunit.list",
		RequiredParams: []string{"as_of"},
		OptionalParams: []string{"parent_org_code", "include_disabled"},
		Executor: queryExecutorStub{
			executeFn: func(context.Context, cubebox.ExecuteRequest, map[string]any) (cubebox.ExecuteResult, error) {
				return cubebox.ExecuteResult{
					Payload: map[string]any{
						"org_units": []map[string]any{{"org_code": "100000", "name": "总部", "has_children": true}},
					},
				}, nil
			},
		},
	})
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}

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
					},
				}, nil
			},
			preparePromptViewFn: func(context.Context, string, string, string, cubebox.CanonicalContext, string) (cubebox.PromptViewPreparationResponse, error) {
				return cubebox.PromptViewPreparationResponse{NextSequence: 1}, nil
			},
		},
		registry: registry,
		producer: cubeboxReadPlanProducerStub{fn: func(_ context.Context, input cubeboxReadPlanProductionInput) (cubeboxReadPlanProductionResult, error) {
			plannerCalls++
			if plannerCalls == 1 {
				if input.QueryContext.ClarificationResume == nil {
					t.Fatalf("expected clarification resume, got %#v", input.QueryContext)
				}
				if !input.QueryContext.ClarificationResume.ReplyCandidate || input.QueryContext.ClarificationResume.RawUserReply != "1日" {
					t.Fatalf("unexpected clarification resume=%#v", input.QueryContext.ClarificationResume)
				}
				if input.QueryContext.ClarificationResume.SourceTurnID != "turn_prev" {
					t.Fatalf("unexpected source turn id=%#v", input.QueryContext.ClarificationResume)
				}
				return queryPlannerReadPlanResult(queryPlanForOrgUnitList("2025-01-01", "")), nil
			}
			return queryPlannerDoneResult(), nil
		}},
		narrator: cubeboxQueryNarratorStub{fn: func(_ context.Context, input cubeboxQueryNarrationInput) (string, error) {
			return "已按 2025-01-01 查询。", nil
		}},
		knowledgePacks: []cubebox.KnowledgePack{
			{Dir: "modules/orgunit/presentation/cubebox", Files: map[string]string{"CUBEBOX-SKILL.md": "x", "queries.md": "x", "apis.md": "x", "examples.md": "x"}},
		},
		now: func() time.Time { return time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC) },
	}

	sink := &capturingGatewaySink{}
	if handled := flow.TryHandle(context.Background(), queryGatewayRequest("1日"), sink); !handled {
		t.Fatal("expected handled")
	}
	if plannerCalls != 2 {
		t.Fatalf("expected 2 planner calls, got %d", plannerCalls)
	}
	if text := strings.Join(sink.deltas(), "\n"); !strings.Contains(text, "2025-01-01") {
		t.Fatalf("expected final narration to include resolved date, got %#v", sink.events)
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
		registry: &cubebox.ExecutionRegistry{},
		producer: cubeboxReadPlanProducerStub{fn: func(_ context.Context, input cubeboxReadPlanProductionInput) (cubeboxReadPlanProductionResult, error) {
			plannerCalls++
			if input.QueryContext.ClarificationResume != nil {
				t.Fatalf("expected no clarification resume after prior clarification already consumed, got %#v", input.QueryContext.ClarificationResume)
			}
			return cubeboxReadPlanProductionResult{
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
		registry: &cubebox.ExecutionRegistry{},
		producer: cubeboxReadPlanProducerStub{fn: func(_ context.Context, input cubeboxReadPlanProductionInput) (cubeboxReadPlanProductionResult, error) {
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
			return cubeboxReadPlanProductionResult{
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

func TestQueryFlowLetsPlannerOwnAuditTargetAfterCandidateSelection(t *testing.T) {
	var executedOrgCodes []string
	registry, err := cubebox.NewExecutionRegistry(cubebox.RegisteredExecutor{
		APIKey:         "orgunit.audit",
		RequiredParams: []string{"org_code"},
		OptionalParams: []string{"limit"},
		Executor: queryExecutorStub{
			validateParamsFn: func(raw map[string]any) (map[string]any, error) { return raw, nil },
			executeFn: func(_ context.Context, _ cubebox.ExecuteRequest, params map[string]any) (cubebox.ExecuteResult, error) {
				orgCode, _ := params["org_code"].(string)
				executedOrgCodes = append(executedOrgCodes, orgCode)
				return cubebox.ExecuteResult{
					Payload: map[string]any{
						"org_code": orgCode,
						"events":   []any{},
					},
					ConfirmedEntity: &cubebox.QueryEntity{
						Domain:    "orgunit",
						Intent:    "orgunit.audit",
						EntityKey: orgCode,
					},
				}, nil
			},
		},
	})
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}

	var firstPlannerInput cubeboxReadPlanProductionInput
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
							Type: cubebox.QueryEntityConfirmedEventType,
							Payload: map[string]any{"entity": map[string]any{
								"domain":     "orgunit",
								"intent":     "orgunit.audit",
								"entity_key": "100000",
							}},
						},
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
									map[string]any{"domain": "orgunit", "entity_key": "200001", "name": "财务部", "as_of": "2026-01-07"},
									map[string]any{"domain": "orgunit", "entity_key": "200002", "name": "财务一组", "as_of": "2026-01-07"},
									map[string]any{"domain": "orgunit", "entity_key": "200004", "name": "财务四组", "as_of": "2026-01-07"},
								},
							},
						},
						{
							Type: "turn.user_message.accepted",
							Payload: map[string]any{
								"text": "以上全部",
							},
						},
					},
				}, nil
			},
			preparePromptViewFn: func(context.Context, string, string, string, cubebox.CanonicalContext, string) (cubebox.PromptViewPreparationResponse, error) {
				return cubebox.PromptViewPreparationResponse{NextSequence: 1}, nil
			},
		},
		registry: registry,
		producer: cubeboxReadPlanProducerStub{fn: func(_ context.Context, input cubeboxReadPlanProductionInput) (cubeboxReadPlanProductionResult, error) {
			plannerCalls++
			if plannerCalls == 1 {
				firstPlannerInput = input
				return queryPlannerReadPlanResult(cubebox.ReadPlan{
					Intent:     "orgunit.audit",
					Confidence: 0.9,
					Steps: []cubebox.ReadPlanStep{
						{ID: "step-1", APIKey: "orgunit.audit", Params: map[string]any{"org_code": "200001"}, DependsOn: []string{}},
						{ID: "step-2", APIKey: "orgunit.audit", Params: map[string]any{"org_code": "200002"}, DependsOn: []string{"step-1"}},
						{ID: "step-3", APIKey: "orgunit.audit", Params: map[string]any{"org_code": "200004"}, DependsOn: []string{"step-2"}},
					},
				}), nil
			}
			return queryPlannerDoneResult(), nil
		}},
		narrator: cubeboxQueryNarratorStub{fn: func(_ context.Context, input cubeboxQueryNarrationInput) (string, error) {
			if got, want := len(input.Results), 3; got != want {
				t.Fatalf("expected %d narration results, got %#v", want, input.Results)
			}
			return "已查询 3 个财务组织的审计信息。", nil
		}},
		knowledgePacks: []cubebox.KnowledgePack{
			{Dir: "modules/orgunit/presentation/cubebox", Files: map[string]string{"CUBEBOX-SKILL.md": "x", "queries.md": "x", "apis.md": "x", "examples.md": "x"}},
		},
		now: func() time.Time { return time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC) },
	}

	sink := &capturingGatewaySink{}
	if handled := flow.TryHandle(context.Background(), queryGatewayRequest("审计信息"), sink); !handled {
		t.Fatal("expected handled")
	}
	if plannerCalls != 2 {
		t.Fatalf("expected 2 planner calls, got %d", plannerCalls)
	}
	if !reflect.DeepEqual(executedOrgCodes, []string{"200001", "200002", "200004"}) {
		t.Fatalf("executor must use planner explicit audit targets, got %#v", executedOrgCodes)
	}
	if input := firstPlannerInput.Prompt; input != "审计信息" {
		t.Fatalf("expected short audit prompt to reach planner, got %q", input)
	}
	if firstPlannerInput.QueryContext.RecentConfirmedEntity == nil || firstPlannerInput.QueryContext.RecentConfirmedEntity.EntityKey != "100000" {
		t.Fatalf("expected earlier root evidence preserved for model input, got %#v", firstPlannerInput.QueryContext)
	}
	if got := len(firstPlannerInput.QueryContext.RecentCandidateGroups); got != 1 {
		t.Fatalf("expected finance candidate evidence preserved, got %#v", firstPlannerInput.QueryContext.RecentCandidateGroups)
	}
	if text := strings.Join(sink.deltas(), "\n"); !strings.Contains(text, "3 个财务组织") {
		t.Fatalf("expected final audit narration, got %#v", sink.events)
	}
}

func TestBuildPlannerMessagesIncludesReadCatalogAndWorkingResults(t *testing.T) {
	producer := &cubeboxProviderReadPlanProducer{
		now: func() time.Time { return time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC) },
	}
	state := cubebox.NewQueryWorkingResultsState("查组织树", cubebox.DefaultQueryLoopBudget())
	state.NotePlanningRound()
	state.AppendPlan(1, cubebox.ReadPlan{
		Intent: "orgunit.list",
		Steps: []cubebox.ReadPlanStep{{
			ID:        "step-1",
			APIKey:    "orgunit.list",
			Params:    map[string]any{"as_of": "2026-04-25"},
			DependsOn: []string{},
		}},
	}, []cubebox.ExecuteResult{{
		Payload: map[string]any{"org_units": []map[string]any{{"org_code": "100000", "has_children": true}}},
	}})
	snapshot := state.Snapshot()

	messages := producer.buildPlannerMessages(cubeboxReadPlanProductionInput{
		Prompt: "继续查有下级的组织",
		KnowledgePacks: []cubebox.KnowledgePack{
			{Dir: "modules/orgunit/presentation/cubebox", Files: map[string]string{"CUBEBOX-SKILL.md": "x", "queries.md": "x", "apis.md": "x", "examples.md": "x"}},
		},
		ReadAPICatalog: []cubebox.ReadAPICatalogEntry{{APIKey: "orgunit.list", RequiredParams: []string{"as_of"}, OptionalParams: []string{"parent_org_code"}}},
		WorkingResults: &snapshot,
	})

	joined := plannerMessageText(messages)
	for _, expected := range []string{
		"read_api_catalog",
		"working_results",
		`"api_key":"orgunit.list"`,
		"executed_fingerprints",
		"不要输出裸 DONE",
		"不要用 NO_QUERY 表示已经查够",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected planner prompt to contain %q, got %s", expected, joined)
		}
	}
}

func TestQueryFlowLoopsUntilPlannerDoneAndNarratesOnce(t *testing.T) {
	registry, err := cubebox.NewExecutionRegistry(cubebox.RegisteredExecutor{
		APIKey:         "orgunit.list",
		RequiredParams: []string{"as_of"},
		OptionalParams: []string{"include_disabled"},
		Executor: queryExecutorStub{
			executeFn: func(context.Context, cubebox.ExecuteRequest, map[string]any) (cubebox.ExecuteResult, error) {
				return cubebox.ExecuteResult{
					Payload: map[string]any{
						"org_units": []map[string]any{{"org_code": "100000", "name": "总部", "has_children": false}},
					},
				}, nil
			},
		},
	})
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}
	var plannerCalls int
	var narratorCalls int
	var appended []cubebox.CanonicalEvent
	flow := &cubeboxQueryFlow{
		runtime: cubebox.NewRuntime(),
		store: cubeboxStoreStub{
			appendFn: func(_ context.Context, _ string, _ string, _ string, event cubebox.CanonicalEvent) error {
				appended = append(appended, event)
				return nil
			},
		},
		registry: registry,
		producer: cubeboxReadPlanProducerStub{fn: func(_ context.Context, input cubeboxReadPlanProductionInput) (cubeboxReadPlanProductionResult, error) {
			plannerCalls++
			if len(input.ReadAPICatalog) != 1 || input.WorkingResults == nil {
				t.Fatalf("expected read api catalog and working results, got catalog=%#v working=%#v", input.ReadAPICatalog, input.WorkingResults)
			}
			if plannerCalls == 1 {
				if input.WorkingResults.LatestObservation != nil {
					t.Fatalf("first planner call should not have latest observation=%#v", input.WorkingResults.LatestObservation)
				}
				return queryPlannerReadPlanResult(queryPlanForOrgUnitList("2026-04-25", "")), nil
			}
			if input.WorkingResults.LatestObservation == nil || input.WorkingResults.LatestObservation.ItemCount != 1 {
				t.Fatalf("expected latest observation on second planner call, got %#v", input.WorkingResults)
			}
			return queryPlannerDoneResult(), nil
		}},
		narrator: cubeboxQueryNarratorStub{fn: func(_ context.Context, input cubeboxQueryNarrationInput) (string, error) {
			narratorCalls++
			if got, want := len(input.Results), 1; got != want {
				t.Fatalf("expected %d narration result, got %#v", want, input.Results)
			}
			return "总部没有下级组织。", nil
		}},
		knowledgePacks: []cubebox.KnowledgePack{{Dir: "modules/orgunit/presentation/cubebox", Files: map[string]string{"CUBEBOX-SKILL.md": "x", "queries.md": "x", "apis.md": "x", "examples.md": "x"}}},
		now:            func() time.Time { return time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC) },
	}

	sink := &capturingGatewaySink{}
	if handled := flow.TryHandle(context.Background(), queryGatewayRequest("查组织树"), sink); !handled {
		t.Fatal("expected query flow handled")
	}
	if plannerCalls != 2 || narratorCalls != 1 {
		t.Fatalf("expected 2 planner calls and 1 narrator call, got planner=%d narrator=%d", plannerCalls, narratorCalls)
	}
	if text := strings.Join(sink.deltas(), "\n"); !strings.Contains(text, "总部没有下级组织。") {
		t.Fatalf("expected final narration, got events=%#v", sink.events)
	}
	body, err := json.Marshal(appended)
	if err != nil {
		t.Fatalf("marshal appended: %v", err)
	}
	if strings.Contains(string(body), "working_results") {
		t.Fatalf("working_results must not be written to canonical events: %s", body)
	}
}

func TestQueryFlowCanReplanFromWorkingResultsObservation(t *testing.T) {
	var executeParams []map[string]any
	registry, err := cubebox.NewExecutionRegistry(cubebox.RegisteredExecutor{
		APIKey:         "orgunit.list",
		RequiredParams: []string{"as_of"},
		OptionalParams: []string{"include_disabled", "parent_org_code"},
		Executor: queryExecutorStub{
			executeFn: func(_ context.Context, _ cubebox.ExecuteRequest, params map[string]any) (cubebox.ExecuteResult, error) {
				executeParams = append(executeParams, params)
				if len(executeParams) == 1 {
					return cubebox.ExecuteResult{Payload: map[string]any{
						"org_units": []map[string]any{{"org_code": "100000", "name": "总部", "has_children": true}},
					}}, nil
				}
				return cubebox.ExecuteResult{Payload: map[string]any{
					"org_units": []map[string]any{{"org_code": "100100", "name": "研发部", "has_children": false}},
				}}, nil
			},
		},
	})
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}
	var plannerCalls int
	var narratorResults int
	flow := &cubeboxQueryFlow{
		runtime: cubebox.NewRuntime(),
		store: cubeboxStoreStub{
			appendFn: func(context.Context, string, string, string, cubebox.CanonicalEvent) error { return nil },
		},
		registry: registry,
		producer: cubeboxReadPlanProducerStub{fn: func(_ context.Context, input cubeboxReadPlanProductionInput) (cubeboxReadPlanProductionResult, error) {
			plannerCalls++
			switch plannerCalls {
			case 1:
				return queryPlannerReadPlanResult(queryPlanForOrgUnitList("2026-04-25", "")), nil
			case 2:
				if input.WorkingResults == nil || input.WorkingResults.LatestObservation == nil {
					t.Fatalf("expected working observation for replanning, got %#v", input.WorkingResults)
				}
				if !workingObservationContainsOrgCode(input.WorkingResults.LatestObservation, "100000") {
					t.Fatalf("expected root org in latest observation, got %#v", input.WorkingResults.LatestObservation)
				}
				return queryPlannerReadPlanResult(queryPlanForOrgUnitList("2026-04-25", "100000")), nil
			default:
				return queryPlannerDoneResult(), nil
			}
		}},
		narrator: cubeboxQueryNarratorStub{fn: func(_ context.Context, input cubeboxQueryNarrationInput) (string, error) {
			narratorResults = len(input.Results)
			return "组织树已展开。", nil
		}},
		knowledgePacks: []cubebox.KnowledgePack{{Dir: "modules/orgunit/presentation/cubebox", Files: map[string]string{"CUBEBOX-SKILL.md": "x", "queries.md": "x", "apis.md": "x", "examples.md": "x"}}},
		now:            func() time.Time { return time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC) },
	}

	sink := &capturingGatewaySink{}
	if handled := flow.TryHandle(context.Background(), queryGatewayRequest("展开组织树"), sink); !handled {
		t.Fatal("expected handled")
	}
	if plannerCalls != 3 || narratorResults != 1 {
		t.Fatalf("expected 3 planner calls and final narration result only, got planner=%d results=%d", plannerCalls, narratorResults)
	}
	if got, want := len(executeParams), 2; got != want {
		t.Fatalf("expected %d executions, got %#v", want, executeParams)
	}
	if _, exists := executeParams[0]["parent_org_code"]; exists {
		t.Fatalf("first root list should not set parent_org_code, got %#v", executeParams[0])
	}
	if executeParams[1]["parent_org_code"] != "100000" {
		t.Fatalf("expected second plan to query observed child parent, got %#v", executeParams[1])
	}
}

func TestQueryFlowNarratesOnlyLatestExecutionResultsAfterExploration(t *testing.T) {
	registry, err := cubebox.NewExecutionRegistry(cubebox.RegisteredExecutor{
		APIKey:         "orgunit.list",
		RequiredParams: []string{"as_of"},
		OptionalParams: []string{"parent_org_code"},
		Executor: queryExecutorStub{
			executeFn: func(_ context.Context, _ cubebox.ExecuteRequest, params map[string]any) (cubebox.ExecuteResult, error) {
				if _, ok := params["parent_org_code"]; !ok {
					return cubebox.ExecuteResult{Payload: map[string]any{
						"org_units": []map[string]any{{"org_code": "100000", "name": "飞虫与鲜花", "has_children": true}},
					}}, nil
				}
				return cubebox.ExecuteResult{Payload: map[string]any{
					"org_units": []map[string]any{
						{"org_code": "100000", "name": "飞虫与鲜花"},
						{"org_code": "200000", "name": "飞虫公司"},
						{"org_code": "300000", "name": "鲜花公司"},
						{"org_code": "200001", "name": "财务部"},
						{"org_code": "200002", "name": "财务一组"},
						{"org_code": "200003", "name": "财务三组"},
						{"org_code": "200004", "name": "财务四组"},
						{"org_code": "200005", "name": "成本A组"},
						{"org_code": "200006", "name": "成本B组"},
						{"org_code": "200007", "name": "成本C组"},
					},
				}}, nil
			},
		},
	})
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}
	var plannerCalls int
	var narratedItems int
	flow := queryLoopTestFlow(registry, cubeboxReadPlanProducerStub{fn: func(context.Context, cubeboxReadPlanProductionInput) (cubeboxReadPlanProductionResult, error) {
		plannerCalls++
		if plannerCalls == 1 {
			return queryPlannerReadPlanResult(queryPlanForOrgUnitList("2026-04-27", "")), nil
		}
		if plannerCalls == 2 {
			return queryPlannerReadPlanResult(queryPlanForOrgUnitList("2026-04-27", "100000")), nil
		}
		return queryPlannerDoneResult(), nil
	}}, cubeboxQueryNarratorStub{fn: func(_ context.Context, input cubeboxQueryNarrationInput) (string, error) {
		if got, want := len(input.Results), 1; got != want {
			t.Fatalf("expected only latest execution result, got %#v", input.Results)
		}
		items, ok := input.Results[0].Data["org_units"].([]map[string]any)
		if !ok {
			t.Fatalf("expected org_units result, got %#v", input.Results[0].Data["org_units"])
		}
		narratedItems = len(items)
		return "系统中查询到的组织共有 10 个。", nil
	}})

	sink := &capturingGatewaySink{}
	if handled := flow.TryHandle(context.Background(), queryGatewayRequest("列出全部组织"), sink); !handled {
		t.Fatal("expected handled")
	}
	if plannerCalls != 3 {
		t.Fatalf("expected 3 planner calls, got %d", plannerCalls)
	}
	if narratedItems != 10 {
		t.Fatalf("expected narration to see final 10 org units, got %d", narratedItems)
	}
}

func TestQueryFlowFailsClosedWhenPlannerReturnsNoQueryAfterExecution(t *testing.T) {
	registry, err := cubebox.NewExecutionRegistry(cubebox.RegisteredExecutor{
		APIKey:         "orgunit.list",
		RequiredParams: []string{"as_of"},
		Executor: queryExecutorStub{
			executeFn: func(context.Context, cubebox.ExecuteRequest, map[string]any) (cubebox.ExecuteResult, error) {
				return cubebox.ExecuteResult{Payload: map[string]any{"org_units": []any{map[string]any{"org_code": "100000"}}}}, nil
			},
		},
	})
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}
	calls := 0
	flow := queryLoopTestFlow(registry, cubeboxReadPlanProducerStub{fn: func(context.Context, cubeboxReadPlanProductionInput) (cubeboxReadPlanProductionResult, error) {
		calls++
		if calls == 1 {
			return queryPlannerReadPlanResult(queryPlanForOrgUnitList("2026-04-25", "")), nil
		}
		return cubeboxReadPlanProductionResult{Handled: false, Outcome: cubebox.PlannerOutcome{Type: cubebox.PlannerOutcomeNoQuery}}, nil
	}}, cubeboxQueryNarratorStub{fn: func(context.Context, cubeboxQueryNarrationInput) (string, error) {
		t.Fatal("narrator should not run after post-execution NO_QUERY")
		return "", nil
	}})

	sink := &capturingGatewaySink{}
	if handled := flow.TryHandle(context.Background(), queryGatewayRequest("查组织树"), sink); !handled {
		t.Fatal("expected handled")
	}
	if !sink.hasErrorCode("cubebox_query_no_query_after_execution") {
		t.Fatalf("expected no-query-after-execution terminal, got %#v", sink.events)
	}
}

func TestQueryFlowFailsClosedWhenDoneHasNoExecution(t *testing.T) {
	flow := queryLoopTestFlow(&cubebox.ExecutionRegistry{}, cubeboxReadPlanProducerStub{fn: func(context.Context, cubeboxReadPlanProductionInput) (cubeboxReadPlanProductionResult, error) {
		return queryPlannerDoneResult(), nil
	}}, cubeboxQueryNarratorStub{fn: func(context.Context, cubeboxQueryNarrationInput) (string, error) {
		t.Fatal("narrator should not run when DONE has no execution")
		return "", nil
	}})

	sink := &capturingGatewaySink{}
	if handled := flow.TryHandle(context.Background(), queryGatewayRequest("查组织树"), sink); !handled {
		t.Fatal("expected handled")
	}
	if !sink.hasErrorCode("cubebox_query_done_without_result") {
		t.Fatalf("expected done-without-result terminal, got %#v", sink.events)
	}
}

func TestQueryFlowMapsPostExecutionPlannerProviderErrorAsModelFailure(t *testing.T) {
	registry, err := cubebox.NewExecutionRegistry(cubebox.RegisteredExecutor{
		APIKey:         "orgunit.list",
		RequiredParams: []string{"as_of"},
		Executor: queryExecutorStub{
			executeFn: func(context.Context, cubebox.ExecuteRequest, map[string]any) (cubebox.ExecuteResult, error) {
				return cubebox.ExecuteResult{Payload: map[string]any{"org_units": []any{map[string]any{"org_code": "100000"}}}}, nil
			},
		},
	})
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}
	calls := 0
	flow := queryLoopTestFlow(registry, cubeboxReadPlanProducerStub{fn: func(context.Context, cubeboxReadPlanProductionInput) (cubeboxReadPlanProductionResult, error) {
		calls++
		if calls == 1 {
			return queryPlannerReadPlanResult(queryPlanForOrgUnitList("2026-04-25", "")), nil
		}
		return cubeboxReadPlanProductionResult{}, cubebox.ErrProviderUnavailable
	}}, cubeboxQueryNarratorStub{fn: func(context.Context, cubeboxQueryNarrationInput) (string, error) {
		t.Fatal("narrator should not run when post-execution planner provider fails")
		return "", nil
	}})

	sink := &capturingGatewaySink{}
	if handled := flow.TryHandle(context.Background(), queryGatewayRequest("查组织树"), sink); !handled {
		t.Fatal("expected handled")
	}
	if !sink.hasErrorCode("ai_reply_render_failed") {
		t.Fatalf("expected model failure terminal, got %#v", sink.events)
	}
	if sink.hasErrorCode("ai_plan_boundary_violation") {
		t.Fatalf("provider failure must not be mapped as plan boundary, got %#v", sink.events)
	}
}

func TestQueryFlowDowngradesUnsupportedOrgUnitDimensionBoundaryViolationToNoQuery(t *testing.T) {
	flow := queryLoopTestFlow(&cubebox.ExecutionRegistry{}, cubeboxReadPlanProducerStub{
		err: cubebox.ErrReadPlanBoundaryViolation,
	}, cubeboxQueryNarratorStub{fn: func(context.Context, cubeboxQueryNarrationInput) (string, error) {
		t.Fatal("narrator should not run for no-query downgrade")
		return "", nil
	}})

	sink := &capturingGatewaySink{}
	if handled := flow.TryHandle(context.Background(), queryGatewayRequest("列出全部的成本组织，以及他们的路径长名称"), sink); !handled {
		t.Fatal("expected handled")
	}
	if sink.hasErrorCode("ai_plan_boundary_violation") {
		t.Fatalf("unsupported dimension should downgrade to no-query, got %#v", sink.events)
	}
	text := strings.Join(sink.deltas(), "\n")
	if !strings.Contains(text, "当前输入未进入已支持查询闭环") && !strings.Contains(text, "当前主要支持组织相关只读查询") {
		t.Fatalf("expected no-query guidance text, got %q", text)
	}
	if strings.Contains(text, "NO_QUERY") || strings.Contains(text, "planner") {
		t.Fatalf("no-query guidance leaked internals: %q", text)
	}
}

func TestQueryFlowBudgetExhaustionDoesNotNarratePartialAnswer(t *testing.T) {
	executions := 0
	registry, err := cubebox.NewExecutionRegistry(cubebox.RegisteredExecutor{
		APIKey:         "orgunit.list",
		RequiredParams: []string{"as_of"},
		OptionalParams: []string{"parent_org_code"},
		Executor: queryExecutorStub{
			executeFn: func(context.Context, cubebox.ExecuteRequest, map[string]any) (cubebox.ExecuteResult, error) {
				executions++
				return cubebox.ExecuteResult{Payload: map[string]any{"org_units": []any{map[string]any{"org_code": "100000"}}}}, nil
			},
		},
	})
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}
	flow := queryLoopTestFlow(registry, cubeboxReadPlanProducerStub{fn: func(_ context.Context, input cubeboxReadPlanProductionInput) (cubeboxReadPlanProductionResult, error) {
		parent := ""
		if input.WorkingResults != nil {
			parent = "P" + string(rune('0'+input.WorkingResults.RoundIndex))
		}
		return queryPlannerReadPlanResult(queryPlanForOrgUnitList("2026-04-25", parent)), nil
	}}, cubeboxQueryNarratorStub{fn: func(context.Context, cubeboxQueryNarrationInput) (string, error) {
		t.Fatal("narrator should not run when budget is exhausted")
		return "", nil
	}})

	sink := &capturingGatewaySink{}
	if handled := flow.TryHandle(context.Background(), queryGatewayRequest("查完整组织树"), sink); !handled {
		t.Fatal("expected handled")
	}
	if executions != cubebox.DefaultQueryLoopMaxPlanningRounds {
		t.Fatalf("expected executions up to planning budget, got %d", executions)
	}
	if !sink.hasErrorCode("cubebox_query_loop_budget_exceeded") {
		t.Fatalf("expected budget terminal, got %#v", sink.events)
	}
	if text := strings.Join(sink.deltas(), "\n"); strings.Contains(text, "partial") || strings.Contains(text, "部分") {
		t.Fatalf("budget terminal must not narrate partial answer, got %q", text)
	}
}

func TestQueryFlowRepeatedPlanFailsClosedWithoutReexecution(t *testing.T) {
	executions := 0
	registry, err := cubebox.NewExecutionRegistry(cubebox.RegisteredExecutor{
		APIKey:         "orgunit.list",
		RequiredParams: []string{"as_of"},
		Executor: queryExecutorStub{
			executeFn: func(context.Context, cubebox.ExecuteRequest, map[string]any) (cubebox.ExecuteResult, error) {
				executions++
				return cubebox.ExecuteResult{Payload: map[string]any{"org_units": []any{}}}, nil
			},
		},
	})
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}
	plan := queryPlanForOrgUnitList("2026-04-25", "")
	flow := queryLoopTestFlow(registry, cubeboxReadPlanProducerStub{fn: func(context.Context, cubeboxReadPlanProductionInput) (cubeboxReadPlanProductionResult, error) {
		return queryPlannerReadPlanResult(plan), nil
	}}, cubeboxQueryNarratorStub{fn: func(context.Context, cubeboxQueryNarrationInput) (string, error) {
		t.Fatal("narrator should not run for repeated plan terminal")
		return "", nil
	}})

	sink := &capturingGatewaySink{}
	if handled := flow.TryHandle(context.Background(), queryGatewayRequest("查组织树"), sink); !handled {
		t.Fatal("expected handled")
	}
	if executions != 1 {
		t.Fatalf("repeated fingerprint must not reexecute, got executions=%d", executions)
	}
	if !sink.hasErrorCode("cubebox_query_loop_repeated_plan") {
		t.Fatalf("expected repeated-plan terminal, got %#v", sink.events)
	}
}

func TestCubeboxProviderReadPlanProducerRejectsBareDone(t *testing.T) {
	producer := &cubeboxProviderReadPlanProducer{
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

	_, err := producer.ProduceReadPlan(context.Background(), cubeboxReadPlanProductionInput{
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

func queryPlanForOrgUnitList(asOf string, parentOrgCode string) cubebox.ReadPlan {
	params := map[string]any{"as_of": asOf}
	if strings.TrimSpace(parentOrgCode) != "" {
		params["parent_org_code"] = strings.TrimSpace(parentOrgCode)
	}
	return cubebox.ReadPlan{
		Intent:        "orgunit.list",
		Confidence:    0.9,
		MissingParams: []string{},
		Steps: []cubebox.ReadPlanStep{{
			ID:          "step-1",
			APIKey:      "orgunit.list",
			Params:      params,
			ResultFocus: []string{"org_units[].org_code", "org_units[].has_children"},
			DependsOn:   []string{},
		}},
		ExplainFocus: []string{"组织列表"},
	}
}

func queryPlannerReadPlanResult(plan cubebox.ReadPlan) cubeboxReadPlanProductionResult {
	return cubeboxReadPlanProductionResult{
		Handled: true,
		Outcome: cubebox.PlannerOutcome{
			Type: cubebox.PlannerOutcomeReadPlan,
			Plan: plan,
		},
		Plan:            plan,
		ProviderID:      "openai-compatible",
		ProviderType:    "openai-compatible",
		ModelSlug:       "gpt-5.2",
		ExplicitOutcome: true,
	}
}

func queryPlannerDoneResult() cubeboxReadPlanProductionResult {
	return cubeboxReadPlanProductionResult{
		Handled:         true,
		Outcome:         cubebox.PlannerOutcome{Type: cubebox.PlannerOutcomeDone},
		ProviderID:      "openai-compatible",
		ProviderType:    "openai-compatible",
		ModelSlug:       "gpt-5.2",
		ExplicitOutcome: true,
	}
}

func queryLoopTestFlow(registry *cubebox.ExecutionRegistry, producer cubeboxReadPlanProducerStub, narrator cubeboxQueryNarratorStub) *cubeboxQueryFlow {
	return &cubeboxQueryFlow{
		runtime: cubebox.NewRuntime(),
		store: cubeboxStoreStub{
			appendFn: func(context.Context, string, string, string, cubebox.CanonicalEvent) error { return nil },
		},
		registry: registry,
		producer: producer,
		narrator: narrator,
		knowledgePacks: []cubebox.KnowledgePack{
			{Dir: "modules/orgunit/presentation/cubebox", Files: map[string]string{"CUBEBOX-SKILL.md": "x", "queries.md": "x", "apis.md": "x", "examples.md": "x"}},
		},
		now: func() time.Time { return time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC) },
	}
}

func workingObservationContainsOrgCode(observation *cubebox.QueryWorkingObservation, orgCode string) bool {
	if observation == nil {
		return false
	}
	for _, item := range observation.Items {
		payload, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if got, ok := payload["org_code"].(string); ok && got == orgCode {
			return true
		}
	}
	return false
}

func turnIDPtrForTest(v string) *string {
	value := strings.TrimSpace(v)
	if value == "" {
		return nil
	}
	return &value
}

type cubeboxRuntimeConfigReaderStub struct {
	config cubebox.ActiveModelRuntimeConfig
	err    error
}

func (s cubeboxRuntimeConfigReaderStub) GetActiveModelRuntimeConfig(context.Context, string) (cubebox.ActiveModelRuntimeConfig, error) {
	if s.err != nil {
		return cubebox.ActiveModelRuntimeConfig{}, s.err
	}
	return s.config, nil
}

type cubeboxSecretResolverStub struct {
	secret string
	err    error
}

func (s cubeboxSecretResolverStub) ResolveSecretRef(context.Context, string, string, string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.secret, nil
}

type cubeboxProviderAdapterStub struct {
	lastRequest cubebox.ProviderChatRequest
	stream      cubebox.ProviderChatStream
	err         error
}

func (s *cubeboxProviderAdapterStub) StreamChatCompletion(_ context.Context, request cubebox.ProviderChatRequest) (cubebox.ProviderChatStream, error) {
	s.lastRequest = request
	if s.err != nil {
		return nil, s.err
	}
	if s.stream != nil {
		return s.stream, nil
	}
	return cubeboxProviderChatStreamStub{}, nil
}

type cubeboxProviderChatStreamStub struct{}

func (cubeboxProviderChatStreamStub) Recv() (cubebox.ProviderChatChunk, error) {
	return cubebox.ProviderChatChunk{Done: true}, nil
}

func (cubeboxProviderChatStreamStub) Close() error { return nil }

type cubeboxProviderChatStreamTextStub struct {
	chunks []cubebox.ProviderChatChunk
	index  int
}

func (s *cubeboxProviderChatStreamTextStub) Recv() (cubebox.ProviderChatChunk, error) {
	if s.index >= len(s.chunks) {
		return cubebox.ProviderChatChunk{}, io.EOF
	}
	chunk := s.chunks[s.index]
	s.index += 1
	return chunk, nil
}

func (*cubeboxProviderChatStreamTextStub) Close() error { return nil }

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
					Domain:        "orgunit",
					Intent:        "orgunit.details",
					EntityKey:     "100000",
					AsOf:          "2026-04-24",
					SourceAPIKey:  "orgunit.details",
					TargetOrgCode: "100000",
					ParentOrgCode: "ROOT",
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
		`"api_key":"orgunit.details"`,
		`"payload":`,
		`"plan":`,
		`"query_evidence_window"`,
		`"entity_key":"100000"`,
		`"executed_steps"`,
		`"executor_key"`,
		`"resolved_entity"`,
		`"source_api_key"`,
		`"target_org_code"`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("expected narrator body to omit raw execution envelope %q, got %q", forbidden, body)
		}
	}
}
