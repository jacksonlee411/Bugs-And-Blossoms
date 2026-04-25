package server

import (
	"context"
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
		Plan:                 cubebox.ReadPlan{Intent: "orgunit.details", Confidence: 0.9, Steps: []cubebox.ReadPlanStep{{ID: "step-1", APIKey: "orgunit.details", Params: map[string]any{"org_code": "1001", "as_of": "2026-04-23"}, DependsOn: []string{}}}},
		Results:              []cubebox.ExecuteResult{{APIKey: "orgunit.details", StepID: "step-1", Payload: map[string]any{"org_unit": map[string]any{"name": "总部"}}}},
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

func TestValidateQueryNarrationTextRejectsInternalLeakage(t *testing.T) {
	for _, text := range []string{
		"```json\n{\"results\":[{\"payload\":{\"name\":\"飞虫与鲜花\"}}]}\n```",
		"{\"results\":[{\"step_id\":\"step-1\",\"payload\":{\"org_unit\":{\"org_code\":\"100000\"}}}]}",
		"step-1 调用了 orgunit.details，result_focus 是 org_unit.name。",
		"内部参数 org_code：100000，as_of：2026-04-24。",
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
	} {
		if err := validateQueryNarrationText(text); err != nil {
			t.Fatalf("expected natural language to pass for %q, got %v", text, err)
		}
	}
}

func TestQueryExecutionClarifyingQuestionReturnsAmbiguousSearchPrompt(t *testing.T) {
	err := &orgUnitSearchAmbiguousError{
		Query: "华东",
		Candidates: []OrgUnitSearchCandidate{
			{OrgCode: "1001", Name: "华东销售中心", Status: "active"},
			{OrgCode: "1002", Name: "华东运营中心", Status: "disabled"},
		},
	}
	text := queryExecutionClarifyingQuestion(err)
	if !strings.Contains(text, "华东") || !strings.Contains(text, "1001") || !strings.Contains(text, "1002") {
		t.Fatalf("unexpected clarification text=%q", text)
	}
}

func TestQueryFlowReturnsPlannerClarificationVerbatim(t *testing.T) {
	flow := &cubeboxQueryFlow{
		runtime: cubebox.NewRuntime(),
		store: cubeboxStoreStub{
			appendFn: func(context.Context, string, string, string, cubebox.CanonicalEvent) error { return nil },
			compactFn: func(context.Context, string, string, string, cubebox.CanonicalContext, string) (cubebox.CompactConversationResponse, error) {
				return cubebox.CompactConversationResponse{NextSequence: 1}, nil
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

type capturingGatewaySink struct {
	events []cubebox.CanonicalEvent
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
		TenantID:             "tenant-a",
		Prompt:               "查一下 100000 在 2026-04-24 的组织详情",
		Plan:                 cubebox.ReadPlan{Intent: "orgunit.details", Confidence: 0.9, Steps: []cubebox.ReadPlanStep{{ID: "step-1", APIKey: "orgunit.details", Params: map[string]any{"org_code": "100000", "as_of": "2026-04-24"}, DependsOn: []string{}}}},
		Results:              []cubebox.ExecuteResult{{APIKey: "orgunit.details", StepID: "step-1", Payload: map[string]any{"org_unit": map[string]any{"org_code": "100000", "name": "飞虫与鲜花", "status": "active"}}}},
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
}
