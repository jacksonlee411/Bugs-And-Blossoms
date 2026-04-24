package server

import (
	"context"
	"errors"
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
		adapter:        cubeboxProviderAdapterStub{},
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
	stream cubebox.ProviderChatStream
	err    error
}

func (s cubeboxProviderAdapterStub) StreamChatCompletion(context.Context, cubebox.ProviderChatRequest) (cubebox.ProviderChatStream, error) {
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
