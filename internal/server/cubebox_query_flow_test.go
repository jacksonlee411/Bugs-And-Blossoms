package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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
		`"dialogue_context"`,
		`"candidates"`,
		`"candidate_group_id":"candgrp_test_1"`,
		`"recent_candidate_groups"`,
		`"group_id":"candgrp_test_1"`,
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

func TestBuildQueryClarificationEnvelopeProjectsCurrentCandidateGroupIntoDialogueContext(t *testing.T) {
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

	if got, want := len(envelope.DialogueContext.RecentCandidateGroups), 2; got != want {
		t.Fatalf("expected %d candidate groups, got %#v", want, envelope.DialogueContext.RecentCandidateGroups)
	}
	lastGroup := envelope.DialogueContext.RecentCandidateGroups[len(envelope.DialogueContext.RecentCandidateGroups)-1]
	if lastGroup.GroupID != "candgrp_live_1" {
		t.Fatalf("expected current group appended, got %#v", lastGroup)
	}
	if got, want := len(lastGroup.Candidates), queryContextMaxClarifierCandidatesPerGroup; got != want {
		t.Fatalf("expected current group truncated to %d, got %d", want, got)
	}
	if got, want := len(envelope.Candidates), queryContextMaxClarifierCandidatesPerGroup; got != want {
		t.Fatalf("expected top-level candidates truncated to %d, got %d", want, got)
	}
	if envelope.CandidateGroupID != "candgrp_live_1" || envelope.CandidateCount != len(rawCandidates) || !envelope.CannotSilentSelect {
		t.Fatalf("unexpected envelope metadata=%#v", envelope)
	}
	if lastGroup.Candidates[len(lastGroup.Candidates)-1].EntityKey != envelope.Candidates[len(envelope.Candidates)-1].EntityKey {
		t.Fatalf("expected top-level candidates to mirror projected current group, got %#v %#v", lastGroup.Candidates, envelope.Candidates)
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

func TestBuildPlannerMessagesIncludesQueryDialogueContext(t *testing.T) {
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
		if strings.Contains(message.Content, "query_dialogue_context") {
			found = true
			if !strings.Contains(message.Content, "recent_candidates") || !strings.Contains(message.Content, "recent_candidate_groups") || !strings.Contains(message.Content, "100000") {
				t.Fatalf("unexpected context block=%q", message.Content)
			}
			for _, forbidden := range []string{`"intent":"orgunit.details"`, `"source_api_key"`, `"target_org_code"`, `"parent_org_code"`} {
				if strings.Contains(message.Content, forbidden) {
					t.Fatalf("expected planner context to omit %q, got %q", forbidden, message.Content)
				}
			}
			for _, required := range []string{"recent_confirmed_entity 只是 recent_confirmed_entities 最后一项的兼容别名", "recent_candidates 只是 recent_candidate_groups 最后一组的兼容别名"} {
				if !strings.Contains(message.Content, required) {
					t.Fatalf("expected planner context to contain %q, got %q", required, message.Content)
				}
			}
		}
	}
	if !found {
		t.Fatalf("expected query_dialogue_context block, got %+v", messages)
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
		`"dialogue_context"`,
		`"entity_key":"100000"`,
		`"as_of":"2026-04-24"`,
		`"org_unit"`,
		`"org_code":"100000"`,
		`"parent_org_code":"ROOT"`,
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
