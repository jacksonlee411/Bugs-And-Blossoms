package server

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
)

func TestAssistantReplyModelGatewayTransientFailure(t *testing.T) {
	oldFactory := assistantOpenAIHTTPClientFactory
	defer func() { assistantOpenAIHTTPClientFactory = oldFactory }()
	assistantOpenAIHTTPClientFactory = func() *http.Client {
		return &http.Client{Transport: assistantRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(strings.NewReader(`{"error":"boom"}`)), Header: make(http.Header)}, nil
		})}
	}
	t.Setenv("OPENAI_API_KEY", "k")
	g := &assistantModelGateway{config: assistantModelConfig{Providers: []assistantModelProviderConfig{{Name: "openai", Enabled: true, Endpoint: "https://api.openai.com/v1", TimeoutMS: 100, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"}}}}
	if _, err := g.RenderReply(context.Background(), assistantReplyRenderPrompt{}); !errors.Is(err, errAssistantModelProviderUnavailable) {
		t.Fatalf("err=%v", err)
	}
}

func TestAssistantReplyParseMoreCoverage(t *testing.T) {
	if _, err := assistantDecodeOpenAIReplyPlainTextResult([]byte(`not-json`), assistantReplyRenderPrompt{}); !errors.Is(err, errAssistantReplyRenderFailed) {
		t.Fatalf("err=%v", err)
	}
	if _, err := assistantDecodeOpenAIReplyResult([]byte(`{"choices":[]}`), assistantReplyRenderPrompt{}); !errors.Is(err, errAssistantReplyRenderFailed) {
		t.Fatalf("err=%v", err)
	}
	if got := assistantReplyTextCandidate(`{"answer":"答案"}`); got != "答案" {
		t.Fatalf("answer=%q", got)
	}
	if got := assistantReplyTextCandidate(`{"output_text":"输出"}`); got != "输出" {
		t.Fatalf("output_text=%q", got)
	}
	if got := assistantReplyTextCandidate(`  raw-text  `); got != "raw-text" {
		t.Fatalf("raw=%q", got)
	}
	if got := assistantParseReplyPayload(""); got != (assistantReplyPayload{}) {
		t.Fatalf("payload=%+v", got)
	}
}

func TestAssistantRenderTurnReplyMoreCoverage(t *testing.T) {
	svc := newAssistantConversationService(nil, nil)
	principal, conversation, turn := seedAssistantReplyConversation(t, svc)
	_ = turn

	original := assistantRenderReplyWithModelFn
	defer func() { assistantRenderReplyWithModelFn = original }()

	t.Run("role mismatch", func(t *testing.T) {
		_, err := svc.renderTurnReply(context.Background(), "tenant_1", Principal{ID: principal.ID, RoleSlug: "viewer"}, conversation.ConversationID, "turn_reply_1", assistantRenderReplyRequest{})
		if !errors.Is(err, errAssistantConversationForbidden) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("latest turn and default source", func(t *testing.T) {
		invoked := false
		assistantRenderReplyWithModelFn = func(_ context.Context, _ *assistantConversationService, prompt assistantReplyRenderPrompt) (assistantReplyModelResult, error) {
			invoked = true
			return assistantReplyModelResult{Text: "默认来源", ReplyModelName: assistantReplyTargetModelName}, nil
		}
		reply, err := svc.renderTurnReply(nil, "tenant_1", principal, conversation.ConversationID, "", assistantRenderReplyRequest{})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if invoked {
			t.Fatal("reply model hook should not be invoked")
		}
		if reply.ReplySource != assistantReplySourceProjection || reply.TurnID == "" {
			t.Fatalf("reply=%+v", reply)
		}
	})

	t.Run("empty text rejected", func(t *testing.T) {
		invoked := false
		assistantRenderReplyWithModelFn = func(_ context.Context, _ *assistantConversationService, prompt assistantReplyRenderPrompt) (assistantReplyModelResult, error) {
			invoked = true
			return assistantReplyModelResult{Text: " ", ReplyModelName: assistantReplyTargetModelName, Stage: prompt.Stage, Kind: prompt.Kind}, nil
		}
		reply, err := svc.renderTurnReply(context.Background(), "tenant_1", principal, conversation.ConversationID, "turn_reply_1", assistantRenderReplyRequest{Stage: "draft"})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if invoked {
			t.Fatal("reply model hook should not be invoked")
		}
		if strings.TrimSpace(reply.Text) == "" {
			t.Fatalf("expected local projection reply, got=%+v", reply)
		}
	})

	var nilSvc *assistantConversationService
	nilSvc.persistRenderedReply("tenant_1", principal.ID, conversation.ConversationID, "turn_reply_1", &assistantRenderReplyResponse{Text: "x"})
	svc.persistRenderedReply("tenant_1", principal.ID, "", "turn_reply_1", &assistantRenderReplyResponse{Text: "x"})
	svc.persistRenderedReply("tenant_1", principal.ID, conversation.ConversationID, "", &assistantRenderReplyResponse{Text: "x"})
	svc.persistRenderedReply("tenant_1", principal.ID, conversation.ConversationID, "missing", &assistantRenderReplyResponse{Text: "x"})
}

func TestAssistantOpenAIProviderAdapterRenderReplyStatusMappings(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "k")
	cases := []struct {
		name string
		rt   func(*http.Request) (*http.Response, error)
		want error
	}{
		{
			name: "rate limited",
			rt: func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusTooManyRequests, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
			},
			want: errAssistantModelRateLimited,
		},
		{
			name: "gateway timeout",
			rt: func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusGatewayTimeout, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
			},
			want: errAssistantModelTimeout,
		},
		{
			name: "request timeout error",
			rt: func(*http.Request) (*http.Response, error) {
				return nil, context.DeadlineExceeded
			},
			want: errAssistantModelTimeout,
		},
		{
			name: "bad request",
			rt: func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"boom"}}`)), Header: make(http.Header)}, nil
			},
			want: errAssistantModelConfigInvalid,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			adapter := assistantOpenAIProviderAdapter{httpClient: &http.Client{Transport: assistantRoundTripperFunc(tc.rt)}}
			_, err := adapter.RenderReply(context.Background(), assistantModelProviderConfig{Endpoint: "https://api.openai.com/v1", TimeoutMS: 100, KeyRef: "OPENAI_API_KEY"}, assistantReplyRenderPrompt{})
			if !errors.Is(err, tc.want) {
				t.Fatalf("err=%v want=%v", err, tc.want)
			}
		})
	}
}

func TestAssistantReplyTextSanitizeAndSignals(t *testing.T) {
	if got := assistantReplyTextCandidate(`{"reply":"好的"}`); got != "好的" {
		t.Fatalf("reply=%q", got)
	}
	if got := assistantSanitizeUserFacingReplyText("正常回复", "zh"); got != "正常回复" {
		t.Fatalf("sanitize=%q", got)
	}
	if got := assistantSanitizeUserFacingReplyText("assistant_reply_render_failed", "en"); !strings.Contains(got, "could not be completed") {
		t.Fatalf("sanitize technical=%q", got)
	}
	cases := map[string]bool{
		"assistant_reply_render_failed": true,
		"正常回复":                          false,
	}
	for input, want := range cases {
		if got := assistantReplyContainsTechnicalSignal(input); got != want {
			t.Fatalf("input=%q got=%v want=%v", input, got, want)
		}
	}
}

func TestAssistantReplyGatewayAndFallbackMoreCoverage(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "k")
	oldFactory := assistantOpenAIHTTPClientFactory
	defer func() { assistantOpenAIHTTPClientFactory = oldFactory }()
	assistantOpenAIHTTPClientFactory = func() *http.Client {
		calls := 0
		return &http.Client{Transport: assistantRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return nil, context.DeadlineExceeded
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"{\"text\":\"继续\"}"}}]}`)), Header: make(http.Header)}, nil
		})}
	}
	g := &assistantModelGateway{config: assistantModelConfig{Providers: []assistantModelProviderConfig{{Name: "openai", Enabled: true, Endpoint: "https://api.openai.com/v1", TimeoutMS: 100, Retries: 1, Priority: 2, KeyRef: "OPENAI_API_KEY"}, {Name: "openai", Enabled: false, Endpoint: "https://api.openai.com/v1", TimeoutMS: 100, Retries: 1, Priority: 1, KeyRef: "OPENAI_API_KEY"}}}}
	result, err := g.RenderReply(context.Background(), assistantReplyRenderPrompt{})
	if err != nil || result.Text != "继续" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestAssistantDecodeOpenAIReplyResultMoreCoverage(t *testing.T) {
	if _, err := assistantDecodeOpenAIReplyResult([]byte(`not-json`), assistantReplyRenderPrompt{}); !errors.Is(err, errAssistantReplyRenderFailed) {
		t.Fatalf("err=%v", err)
	}
	if _, err := assistantDecodeOpenAIReplyResult([]byte(`{"choices":[{"message":{"content":"{\"kind\":\"info\"}"}}]}`), assistantReplyRenderPrompt{}); !errors.Is(err, errAssistantReplyRenderFailed) {
		t.Fatalf("err=%v", err)
	}
	if got := assistantReplyTextCandidate(`{"text":"文本"}`); got != "文本" {
		t.Fatalf("text=%q", got)
	}
}

func TestAssistantReplyNLGAdditionalCoverage(t *testing.T) {
	turn := &assistantTurn{}
	if got := assistantReplyFallbackText(assistantRenderReplyRequest{}, "candidate_confirm", turn, "zh"); !strings.Contains(got, "已收到") {
		t.Fatalf("candidate_confirm=%q", got)
	}
	if got := assistantReplyFallbackText(assistantRenderReplyRequest{}, "commit_failed", turn, "zh"); !strings.Contains(got, "未能完成") {
		t.Fatalf("commit_failed=%q", got)
	}
	turn.DryRun.Explain = "请确认后继续"
	if got := assistantReplyFallbackText(assistantRenderReplyRequest{}, "draft", turn, "zh"); got != "请确认后继续" {
		t.Fatalf("draft explain=%q", got)
	}
	if _, err := assistantRenderReplyWithModel(context.Background(), &assistantConversationService{modelGateway: &assistantModelGateway{config: assistantModelConfig{Providers: []assistantModelProviderConfig{{Name: "openai", Enabled: true, Endpoint: "https://api.openai.com/v1", TimeoutMS: 100, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"}}}}}, assistantReplyRenderPrompt{}); err == nil {
		t.Fatal("expected provider unavailable without stubbed client")
	}
	var svc assistantConversationService
	svc.persistRenderedReply("tenant", "actor", "conv", "turn", nil)
	cases := []string{"trace_id=1", "request_id=1", "runtime config invalid", "model provider unavailable", "assistant_turn_create_failed", "a_b_c_d_e_f"}
	for _, input := range cases {
		if !assistantReplyContainsTechnicalSignal(input) {
			t.Fatalf("expected technical signal for %q", input)
		}
	}
}

func TestAssistantModelGatewayMoreBranches(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "k")
	oldFactory := assistantOpenAIHTTPClientFactory
	oldMarshal := assistantOpenAIRequestMarshalFn
	oldNewReq := assistantOpenAINewRequestWithContextFn
	defer func() {
		assistantOpenAIHTTPClientFactory = oldFactory
		assistantOpenAIRequestMarshalFn = oldMarshal
		assistantOpenAINewRequestWithContextFn = oldNewReq
	}()

	t.Run("equal priority sort and negative retries", func(t *testing.T) {
		assistantOpenAIRequestMarshalFn = oldMarshal
		assistantOpenAINewRequestWithContextFn = oldNewReq
		calls := 0
		assistantOpenAIHTTPClientFactory = func() *http.Client {
			return &http.Client{Transport: assistantRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"{\"text\":\"ok\"}"}}]}`)), Header: make(http.Header)}, nil
			})}
		}
		g := &assistantModelGateway{config: assistantModelConfig{Providers: []assistantModelProviderConfig{
			{Name: "openai-b", Enabled: false, Endpoint: "https://api.openai.com/v1", TimeoutMS: 100, Priority: 1, KeyRef: "OPENAI_API_KEY"},
			{Name: "openai", Enabled: true, Endpoint: "https://api.openai.com/v1", TimeoutMS: 100, Priority: 1, Retries: -1, KeyRef: "OPENAI_API_KEY"},
		}}}
		result, err := g.RenderReply(context.Background(), assistantReplyRenderPrompt{})
		if err != nil || calls != 1 {
			t.Fatalf("result=%+v calls=%d err=%v", result, calls, err)
		}
	})

	t.Run("invalid timeout config", func(t *testing.T) {
		assistantOpenAIRequestMarshalFn = oldMarshal
		assistantOpenAINewRequestWithContextFn = oldNewReq
		g := &assistantModelGateway{config: assistantModelConfig{Providers: []assistantModelProviderConfig{{Name: "openai", Enabled: true, Endpoint: "https://api.openai.com/v1", TimeoutMS: 0, KeyRef: "OPENAI_API_KEY"}}}}
		if _, err := g.RenderReply(context.Background(), assistantReplyRenderPrompt{}); !errors.Is(err, errAssistantModelConfigInvalid) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("non transient error returns immediately", func(t *testing.T) {
		assistantOpenAIRequestMarshalFn = oldMarshal
		assistantOpenAINewRequestWithContextFn = oldNewReq
		assistantOpenAIHTTPClientFactory = func() *http.Client {
			return &http.Client{Transport: assistantRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"bad"}}`)), Header: make(http.Header)}, nil
			})}
		}
		g := &assistantModelGateway{config: assistantModelConfig{Providers: []assistantModelProviderConfig{{Name: "openai", Enabled: true, Endpoint: "https://api.openai.com/v1", TimeoutMS: 100, Retries: 1, KeyRef: "OPENAI_API_KEY"}}}}
		if _, err := g.RenderReply(context.Background(), assistantReplyRenderPrompt{}); !errors.Is(err, errAssistantModelConfigInvalid) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("adapter nil ctx and nil client provider unavailable", func(t *testing.T) {
		assistantOpenAIRequestMarshalFn = oldMarshal
		assistantOpenAINewRequestWithContextFn = oldNewReq
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		addr := ln.Addr().String()
		_ = ln.Close()
		result, err := (assistantOpenAIProviderAdapter{}).RenderReply(nil, assistantModelProviderConfig{Endpoint: "https://" + addr + "/v1", TimeoutMS: 100, KeyRef: "OPENAI_API_KEY"}, assistantReplyRenderPrompt{})
		if !errors.Is(err, errAssistantModelProviderUnavailable) {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	})

	t.Run("marshal and new request errors", func(t *testing.T) {
		assistantOpenAIRequestMarshalFn = oldMarshal
		assistantOpenAINewRequestWithContextFn = oldNewReq
		assistantOpenAIRequestMarshalFn = func(any) ([]byte, error) { return nil, errors.New("marshal") }
		if _, err := (assistantOpenAIProviderAdapter{}).RenderReply(context.Background(), assistantModelProviderConfig{Endpoint: "https://api.openai.com/v1", TimeoutMS: 100, KeyRef: "OPENAI_API_KEY"}, assistantReplyRenderPrompt{}); !errors.Is(err, errAssistantModelConfigInvalid) {
			t.Fatalf("err=%v", err)
		}
		assistantOpenAIRequestMarshalFn = oldMarshal
		assistantOpenAINewRequestWithContextFn = func(context.Context, string, string, io.Reader) (*http.Request, error) {
			return nil, errors.New("newreq")
		}
		if _, err := (assistantOpenAIProviderAdapter{}).RenderReply(context.Background(), assistantModelProviderConfig{Endpoint: "https://api.openai.com/v1", TimeoutMS: 100, KeyRef: "OPENAI_API_KEY"}, assistantReplyRenderPrompt{}); !errors.Is(err, errAssistantModelConfigInvalid) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("provider unavailable and read body error", func(t *testing.T) {
		assistantOpenAIRequestMarshalFn = oldMarshal
		assistantOpenAINewRequestWithContextFn = oldNewReq
		adapter := assistantOpenAIProviderAdapter{httpClient: &http.Client{Transport: assistantRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("dial")
		})}}
		if _, err := adapter.RenderReply(context.Background(), assistantModelProviderConfig{Endpoint: "https://api.openai.com/v1", TimeoutMS: 100, KeyRef: "OPENAI_API_KEY"}, assistantReplyRenderPrompt{}); !errors.Is(err, errAssistantModelProviderUnavailable) {
			t.Fatalf("err=%v", err)
		}
		adapter = assistantOpenAIProviderAdapter{httpClient: &http.Client{Transport: assistantRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: assistantErrReadCloser{}, Header: make(http.Header)}, nil
		})}}
		if _, err := adapter.RenderReply(context.Background(), assistantModelProviderConfig{Endpoint: "https://api.openai.com/v1", TimeoutMS: 100, KeyRef: "OPENAI_API_KEY"}, assistantReplyRenderPrompt{}); !errors.Is(err, errAssistantModelProviderUnavailable) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("second and third pass failure branches", func(t *testing.T) {
		assistantOpenAIRequestMarshalFn = oldMarshal
		assistantOpenAINewRequestWithContextFn = oldNewReq
		calls := 0
		adapter := assistantOpenAIProviderAdapter{httpClient: &http.Client{Transport: assistantRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			switch calls {
			case 1:
				return &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"response_format unsupported"}}`)), Header: make(http.Header)}, nil
			case 2:
				return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(strings.NewReader(`x`)), Header: make(http.Header)}, nil
			default:
				return &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader(`x`)), Header: make(http.Header)}, nil
			}
		})}}
		if _, err := adapter.RenderReply(context.Background(), assistantModelProviderConfig{Endpoint: "https://api.openai.com/v1", TimeoutMS: 100, KeyRef: "OPENAI_API_KEY"}, assistantReplyRenderPrompt{}); !errors.Is(err, errAssistantModelProviderUnavailable) {
			t.Fatalf("err=%v", err)
		}
	})

	if result, err := assistantDecodeOpenAIReplyResult([]byte(`{"choices":[{"message":{"content":"{\"text\":\"assistant_reply_render_failed\"}"}}]}`), assistantReplyRenderPrompt{Locale: "zh"}); err != nil || !strings.Contains(result.Text, "未能完成") {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if got := assistantReplyTextCandidate(`{}`); got != `{}` {
		t.Fatalf("candidate=%q", got)
	}
}

func TestAssistantReplyNLGMissedBranches(t *testing.T) {
	svc := newAssistantConversationService(nil, nil)
	principal, conversation, _ := seedAssistantReplyConversation(t, svc)
	if _, err := svc.renderTurnReply(context.Background(), "tenant_1", principal, "missing-conv", "turn_reply_1", assistantRenderReplyRequest{}); !errors.Is(err, errAssistantConversationNotFound) {
		t.Fatalf("err=%v", err)
	}
	if _, err := svc.renderTurnReply(context.Background(), "tenant_1", principal, conversation.ConversationID, "missing-turn", assistantRenderReplyRequest{}); !errors.Is(err, errAssistantTurnNotFound) {
		t.Fatalf("err=%v", err)
	}
	svc.persistRenderedReply("tenant_1", principal.ID, "missing-conv", "turn_reply_1", &assistantRenderReplyResponse{Text: "x"})
	if got := assistantReplyFallbackText(assistantRenderReplyRequest{}, "commit_failed", &assistantTurn{}, "zh"); !strings.Contains(got, "未能完成") {
		t.Fatalf("commit failed=%q", got)
	}
	if got := assistantReplyFallbackText(assistantRenderReplyRequest{}, "draft", &assistantTurn{}, "en"); !strings.Contains(got, "received") {
		t.Fatalf("draft en=%q", got)
	}
	if got := assistantReplyFallbackText(assistantRenderReplyRequest{}, "candidate_list", &assistantTurn{}, "en"); !strings.Contains(got, "Candidate confirmation") {
		t.Fatalf("candidate list en=%q", got)
	}
	if got := assistantReplyFallbackText(assistantRenderReplyRequest{}, "candidate_list", &assistantTurn{Candidates: []assistantCandidate{{CandidateID: "cid-1", CandidateCode: "C1"}}}, "zh"); !strings.Contains(got, "cid-1 / C1") {
		t.Fatalf("candidate list id=%q", got)
	}
	if got := assistantReplyFallbackText(assistantRenderReplyRequest{}, "draft", nil, "zh"); !strings.Contains(got, "已收到") {
		t.Fatalf("nil turn zh=%q", got)
	}
	for _, input := range []string{"", "a1_b2_c3_d4_e5_f6", "trace_id=1", "a_b_c_d_e_f_g"} {
		_ = assistantReplyContainsTechnicalSignal(input)
	}
}

func TestAssistantReplyAdapterPlainTextSecondDecodeSuccess(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "k")
	adapter := assistantOpenAIProviderAdapter{httpClient: &http.Client{Transport: assistantRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"plain reply"}}]}`)), Header: make(http.Header)}, nil
	})}}
	result, err := adapter.RenderReply(context.Background(), assistantModelProviderConfig{Endpoint: "https://api.openai.com/v1", TimeoutMS: 100, KeyRef: "OPENAI_API_KEY"}, assistantReplyRenderPrompt{})
	if err != nil || result.Text != "plain reply" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestAssistantReplyFallbackAndSignalMissedBranches(t *testing.T) {
	if got := assistantReplyFallbackText(assistantRenderReplyRequest{}, "commit_failed", &assistantTurn{}, "zh"); got != "本次请求未能完成，请根据提示调整后重试。" {
		t.Fatalf("zh commit failed=%q", got)
	}
	if got := assistantReplyFallbackText(assistantRenderReplyRequest{}, "commit_failed", &assistantTurn{}, "en"); !strings.Contains(got, "could not be completed") {
		t.Fatalf("en commit failed=%q", got)
	}
	for _, input := range []string{"abc123_def456_ghi789", "123_456_789_abc"} {
		if !assistantReplyContainsTechnicalSignal(input) {
			t.Fatalf("expected technical signal for %q", input)
		}
	}
}

func TestAssistantReplyAdapterDecodeAndFallbackBranches(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "k")
	t.Run("strict decode fails but plain text decode succeeds", func(t *testing.T) {
		adapter := assistantOpenAIProviderAdapter{httpClient: &http.Client{Transport: assistantRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"{\"message\":\"你好\"}"}}]}`)), Header: make(http.Header)}, nil
		})}}
		result, err := adapter.RenderReply(context.Background(), assistantModelProviderConfig{Endpoint: "https://api.openai.com/v1", TimeoutMS: 100, KeyRef: "OPENAI_API_KEY"}, assistantReplyRenderPrompt{})
		if err != nil || result.Text != "你好" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	})

	t.Run("third pass invoke error bubbles", func(t *testing.T) {
		calls := 0
		adapter := assistantOpenAIProviderAdapter{httpClient: &http.Client{Transport: assistantRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{`)), Header: make(http.Header)}, nil
			}
			return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(strings.NewReader(`x`)), Header: make(http.Header)}, nil
		})}}
		if _, err := adapter.RenderReply(context.Background(), assistantModelProviderConfig{Endpoint: "https://api.openai.com/v1", TimeoutMS: 100, KeyRef: "OPENAI_API_KEY"}, assistantReplyRenderPrompt{}); !errors.Is(err, errAssistantModelProviderUnavailable) {
			t.Fatalf("err=%v", err)
		}
	})

	if got := assistantReplyTextCandidate(""); got != "" {
		t.Fatalf("empty candidate=%q", got)
	}
}

func TestAssistantReplyFallbackAndSignalFinalBranches(t *testing.T) {
	if got := assistantReplyFallbackText(assistantRenderReplyRequest{ErrorCode: "E1"}, "commit_failed", &assistantTurn{}, "zh"); got != "本次请求未能完成，请根据提示调整后重试。" {
		t.Fatalf("zh code fallback=%q", got)
	}
	if !assistantReplyContainsTechnicalSignal("abc123_def456_ghi789") {
		t.Fatal("expected underscore technical signal")
	}
}
