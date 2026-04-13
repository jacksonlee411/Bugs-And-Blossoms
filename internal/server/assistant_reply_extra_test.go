package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAssistantModelGatewayRenderReplyBranches(t *testing.T) {
	t.Run("nil gateway", func(t *testing.T) {
		_, err := ((*assistantModelGateway)(nil)).RenderReply(context.Background(), assistantReplyRenderPrompt{})
		if !errors.Is(err, errAssistantModelProviderUnavailable) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("no openai provider", func(t *testing.T) {
		g := &assistantModelGateway{config: assistantModelConfig{Providers: []assistantModelProviderConfig{{Name: "deepseek", Enabled: true}}}}
		_, err := g.RenderReply(context.Background(), assistantReplyRenderPrompt{})
		if !errors.Is(err, errAssistantModelProviderUnavailable) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("invalid endpoint", func(t *testing.T) {
		g := &assistantModelGateway{config: assistantModelConfig{Providers: []assistantModelProviderConfig{{Name: "openai", Enabled: true, Endpoint: "://bad", TimeoutMS: 1, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"}}}}
		t.Setenv("OPENAI_API_KEY", "k")
		_, err := g.RenderReply(context.Background(), assistantReplyRenderPrompt{})
		if !errors.Is(err, errAssistantModelConfigInvalid) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("missing secret", func(t *testing.T) {
		g := &assistantModelGateway{config: assistantModelConfig{Providers: []assistantModelProviderConfig{{Name: "openai", Enabled: true, Endpoint: "https://api.openai.com/v1", TimeoutMS: 1, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"}}}}
		t.Setenv("OPENAI_API_KEY", "")
		_, err := g.RenderReply(context.Background(), assistantReplyRenderPrompt{})
		if !errors.Is(err, errAssistantModelSecretMissing) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("retry then success", func(t *testing.T) {
		oldFactory := assistantOpenAIHTTPClientFactory
		defer func() { assistantOpenAIHTTPClientFactory = oldFactory }()
		calls := 0
		assistantOpenAIHTTPClientFactory = func() *http.Client {
			return &http.Client{Transport: assistantRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(strings.NewReader(`{"error":"boom"}`)), Header: make(http.Header)}, nil
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"{\"text\":\"已完成\",\"kind\":\"success\",\"stage\":\"commit_result\"}"}}]}`)), Header: make(http.Header)}, nil
			})}
		}
		t.Setenv("OPENAI_API_KEY", "k")
		g := &assistantModelGateway{config: assistantModelConfig{Providers: []assistantModelProviderConfig{{Name: "openai", Enabled: true, Endpoint: "https://api.openai.com/v1", TimeoutMS: 100, Retries: 1, Priority: 1, KeyRef: "OPENAI_API_KEY"}}}}
		result, err := g.RenderReply(context.Background(), assistantReplyRenderPrompt{})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if result.Text != "已完成" || result.Stage != "commit_result" || calls != 2 {
			t.Fatalf("result=%+v calls=%d", result, calls)
		}
	})
}

func TestAssistantOpenAIProviderAdapterRenderReplyBranches(t *testing.T) {
	t.Run("bad endpoint", func(t *testing.T) {
		_, err := (assistantOpenAIProviderAdapter{}).RenderReply(context.Background(), assistantModelProviderConfig{Endpoint: "://bad"}, assistantReplyRenderPrompt{})
		if !errors.Is(err, errAssistantModelConfigInvalid) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("missing secret", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "")
		_, err := (assistantOpenAIProviderAdapter{}).RenderReply(context.Background(), assistantModelProviderConfig{Endpoint: "https://api.openai.com/v1"}, assistantReplyRenderPrompt{})
		if !errors.Is(err, errAssistantModelSecretMissing) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("response format unsupported fallback", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "k")
		calls := 0
		adapter := assistantOpenAIProviderAdapter{httpClient: &http.Client{Transport: assistantRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			body, _ := io.ReadAll(req.Body)
			if calls == 1 {
				if !strings.Contains(string(body), "response_format") {
					t.Fatalf("first payload missing response_format: %s", string(body))
				}
				return &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"response_format unsupported"}}`)), Header: make(http.Header)}, nil
			}
			if strings.Contains(string(body), "response_format") {
				t.Fatalf("fallback payload should not include response_format: %s", string(body))
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"{\"text\":\"好的\",\"kind\":\"info\",\"stage\":\"draft\"}"}}]}`)), Header: make(http.Header)}, nil
		})}}
		result, err := adapter.RenderReply(context.Background(), assistantModelProviderConfig{Endpoint: "https://api.openai.com/v1", TimeoutMS: 100, KeyRef: "OPENAI_API_KEY"}, assistantReplyRenderPrompt{})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if result.Text != "好的" || calls != 2 {
			t.Fatalf("result=%+v calls=%d", result, calls)
		}
	})

	t.Run("plain text fallback after schema and json decode failure", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "k")
		calls := 0
		adapter := assistantOpenAIProviderAdapter{httpClient: &http.Client{Transport: assistantRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"response_format unsupported"}}`)), Header: make(http.Header)}, nil
			}
			if calls == 2 {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{`)), Header: make(http.Header)}, nil
			}
			body, _ := io.ReadAll(req.Body)
			if !strings.Contains(string(body), "禁止输出 JSON") {
				t.Fatalf("plain text fallback should disable json mode: %s", string(body))
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"直接告诉用户已经处理完成"}}]}`)), Header: make(http.Header)}, nil
		})}}
		result, err := adapter.RenderReply(context.Background(), assistantModelProviderConfig{Endpoint: "https://api.openai.com/v1", TimeoutMS: 100, KeyRef: "OPENAI_API_KEY"}, assistantReplyRenderPrompt{Outcome: "success"})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if !strings.Contains(result.Text, "处理完成") || calls != 3 {
			t.Fatalf("result=%+v calls=%d", result, calls)
		}
	})
}

func TestAssistantReplyHelperCoverage(t *testing.T) {
	if assistantReplyOutcome("", "x") != "failure" || assistantReplyOutcome("success", "") != "success" {
		t.Fatal("unexpected reply outcome")
	}
	if assistantReplyStage("candidate_confirm", "success", nil) != "candidate_confirm" {
		t.Fatal("expected explicit stage")
	}
	turn := &assistantTurn{}
	turn.CommitResult = &assistantCommitResult{OrgCode: "ORG1", ParentOrgCode: "P1", EffectiveDate: "2026-01-01"}
	if assistantReplyStage("", "success", turn) != "commit_result" {
		t.Fatal("expected commit_result")
	}
	turn.CommitResult = nil
	turn.DryRun.ValidationErrors = []string{"missing_entity_name"}
	if assistantReplyStage("", "success", turn) != "missing_fields" {
		t.Fatal("expected missing_fields")
	}
	turn.DryRun.ValidationErrors = []string{"candidate_confirmation_required"}
	if assistantReplyStage("", "success", turn) != "candidate_list" {
		t.Fatal("expected candidate_list")
	}
	turn.DryRun.ValidationErrors = nil
	turn.Candidates = []assistantCandidate{{CandidateID: "1"}, {CandidateID: "2"}}
	if assistantReplyStage("", "success", turn) != "candidate_list" {
		t.Fatal("expected candidate_list by candidates")
	}
	turn.ResolvedCandidateID = "2"
	if assistantReplyStage("", "success", turn) != "candidate_confirm" {
		t.Fatal("expected candidate_confirm")
	}
	if assistantReplyStage("", "failure", nil) != "commit_failed" {
		t.Fatal("expected commit_failed")
	}
	if assistantReplyKind("", "missing_fields", "success") != "warning" || assistantReplyKind("", "commit_result", "success") != "success" || assistantReplyKind("", "draft", "failure") != "error" {
		t.Fatal("unexpected reply kind")
	}
	if assistantReplyLocale("en") != "en" || assistantReplyLocale("fr") != "zh" {
		t.Fatal("unexpected locale")
	}
	if got := assistantReplyTextCandidate(`{"message":"你好"}`); got != "你好" {
		t.Fatalf("candidate=%q", got)
	}
	if got := assistantReplyTextCandidate(`{"content":["A","B"]}`); got != "A" {
		t.Fatalf("candidate array=%q", got)
	}
	if got := assistantParseReplyPayload(`prefix {"text":"ok","kind":"info"} suffix`); got.Text != "ok" {
		t.Fatalf("payload=%+v", got)
	}
	if got := assistantParseReplyPayload(`plain text`); got.Text != "plain text" {
		t.Fatalf("payload=%+v", got)
	}
	plain, err := assistantDecodeOpenAIReplyPlainTextResult([]byte(`{"choices":[{"message":{"content":"{\"message\":\"你好\"}"}}]}`), assistantReplyRenderPrompt{})
	if err != nil || plain.Text != "你好" {
		t.Fatalf("plain=%+v err=%v", plain, err)
	}
	if _, err := assistantDecodeOpenAIReplyPlainTextResult([]byte(`{"choices":[]}`), assistantReplyRenderPrompt{}); !errors.Is(err, errAssistantReplyRenderFailed) {
		t.Fatalf("unexpected err=%v", err)
	}
}

func TestAssistantReplyNLGHelperCoverage(t *testing.T) {
	svc := &assistantConversationService{byID: map[string]*assistantConversation{}, byActorID: map[string][]string{}}
	principal, conversation, turn := seedAssistantReplyConversation(t, svc)
	_ = principal
	if assistantFindTurnForReply(nil, "x") != nil {
		t.Fatal("nil conversation should return nil")
	}
	if assistantFindTurnForReply(conversation, "missing") != nil {
		t.Fatal("missing turn should return nil")
	}
	machine := assistantReplyMachineFromTurn(turn)
	if machine.IntentAction == "" {
		t.Fatalf("machine=%+v", machine)
	}
	copyReply := &assistantRenderReplyResponse{Text: "ok"}
	svc.persistRenderedReply("tenant_1", "actor_1", conversation.ConversationID, turn.TurnID, copyReply)
	stored := assistantFindTurnForReply(svc.byID[conversation.ConversationID], turn.TurnID)
	if stored == nil || stored.ReplyNLG == nil || stored.ReplyNLG.Text != "ok" {
		t.Fatalf("stored=%+v", stored)
	}
	svc.persistRenderedReply("tenant_1", "bad-actor", conversation.ConversationID, turn.TurnID, &assistantRenderReplyResponse{Text: "bad"})
	if stored.ReplyNLG.Text != "ok" {
		t.Fatalf("unexpected overwrite=%+v", stored.ReplyNLG)
	}
	if _, err := assistantRenderReplyWithModel(context.Background(), nil, assistantReplyRenderPrompt{}); !errors.Is(err, errAssistantServiceMissing) {
		t.Fatalf("err=%v", err)
	}
	badSvc := &assistantConversationService{gatewayErr: errAssistantRuntimeConfigInvalid}
	if _, err := assistantRenderReplyWithModel(context.Background(), badSvc, assistantReplyRenderPrompt{}); !errors.Is(err, errAssistantRuntimeConfigInvalid) {
		t.Fatalf("err=%v", err)
	}
	badSvc.gatewayErr = nil
	if _, err := assistantRenderReplyWithModel(context.Background(), badSvc, assistantReplyRenderPrompt{}); !errors.Is(err, errAssistantModelProviderUnavailable) {
		t.Fatalf("err=%v", err)
	}
}

func TestAssistantReplyFallbackTextCoverage(t *testing.T) {
	turn := &assistantTurn{}
	if got := assistantReplyFallbackText(assistantRenderReplyRequest{FallbackText: "assistant_reply_render_failed"}, "draft", turn, "zh"); got != "本次请求未能完成，请根据提示调整后重试。" {
		t.Fatalf("fallback sanitize=%q", got)
	}
	if got := assistantReplyFallbackText(assistantRenderReplyRequest{ErrorMessage: "字段不合法", ErrorCode: "x"}, "commit_failed", turn, "zh"); got != "字段不合法" {
		t.Fatalf("commit failed message=%q", got)
	}
	if got := assistantReplyFallbackText(assistantRenderReplyRequest{ErrorCode: "x"}, "commit_failed", turn, "en"); !strings.Contains(got, "could not be completed") {
		t.Fatalf("commit failed generic=%q", got)
	}
	if got := assistantReplyFallbackText(assistantRenderReplyRequest{}, "draft", nil, "en"); !strings.Contains(got, "received") {
		t.Fatalf("nil turn=%q", got)
	}
	turn.CommitResult = &assistantCommitResult{OrgCode: "ORG", ParentOrgCode: "PARENT", EffectiveDate: "2026-01-01"}
	if got := assistantReplyFallbackText(assistantRenderReplyRequest{}, "commit_result", turn, "zh"); !strings.Contains(got, "ORG") {
		t.Fatalf("commit result=%q", got)
	}
	turn.CommitResult = nil
	turn.Candidates = nil
	if got := assistantReplyFallbackText(assistantRenderReplyRequest{}, "candidate_list", turn, "zh"); !strings.Contains(got, "候选确认") {
		t.Fatalf("candidate list empty=%q", got)
	}
	turn.Candidates = []assistantCandidate{{Name: "共享服务中心", CandidateCode: "SSC-1", Path: "集团/共享服务中心"}}
	if got := assistantReplyFallbackText(assistantRenderReplyRequest{}, "candidate_list", turn, "zh"); !strings.Contains(got, "1. 共享服务中心 / SSC-1") {
		t.Fatalf("candidate list=%q", got)
	}
	turn.DryRun.ValidationErrors = []string{"missing_effective_date"}
	if got := assistantReplyFallbackText(assistantRenderReplyRequest{}, "missing_fields", turn, "zh"); !strings.Contains(got, "日期") {
		t.Fatalf("missing fields=%q", got)
	}
	turn.DryRun.ValidationErrors = nil
	turn.DryRun.Explain = "assistant_reply_render_failed"
	if got := assistantReplyFallbackText(assistantRenderReplyRequest{}, "draft", turn, "zh"); got != "本次请求未能完成，请根据提示调整后重试。" {
		t.Fatalf("explain sanitize=%q", got)
	}
}
