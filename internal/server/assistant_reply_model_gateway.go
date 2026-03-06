package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

const assistantReplyTargetModelName = "gpt-5.2"

func (g *assistantModelGateway) RenderReply(ctx context.Context, prompt assistantReplyRenderPrompt) (assistantReplyModelResult, error) {
	if g == nil {
		return assistantReplyModelResult{}, errAssistantModelProviderUnavailable
	}
	cfg := g.snapshot()
	providers := cloneAssistantProviderSlice(cfg.Providers)
	sort.SliceStable(providers, func(i, j int) bool {
		if providers[i].Priority == providers[j].Priority {
			return providers[i].Name < providers[j].Name
		}
		return providers[i].Priority < providers[j].Priority
	})

	openaiFound := false
	lastTransientErr := errAssistantModelProviderUnavailable
	for _, provider := range providers {
		if !provider.Enabled {
			continue
		}
		if strings.TrimSpace(strings.ToLower(provider.Name)) != "openai" {
			continue
		}
		openaiFound = true
		if strings.TrimSpace(provider.Endpoint) == "" || provider.TimeoutMS <= 0 {
			return assistantReplyModelResult{}, errAssistantModelConfigInvalid
		}
		if assistantEndpointInvalidForRuntime(provider.Endpoint) {
			return assistantReplyModelResult{}, errAssistantModelConfigInvalid
		}
		if assistantProviderRequiresOpenAIKey(provider) && strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) == "" {
			return assistantReplyModelResult{}, errAssistantModelSecretMissing
		}
		adapter := assistantOpenAIProviderAdapter{httpClient: assistantOpenAIHTTPClientFactory()}
		attempts := provider.Retries + 1
		if attempts < 1 {
			attempts = 1
		}
		for attempt := 0; attempt < attempts; attempt++ {
			result, err := adapter.RenderReply(ctx, provider, prompt)
			if err != nil {
				if errorsIsAny(err, errAssistantModelTimeout, errAssistantModelRateLimited, errAssistantModelProviderUnavailable) {
					lastTransientErr = err
					if attempt < attempts-1 {
						continue
					}
					break
				}
				return assistantReplyModelResult{}, err
			}
			return result, nil
		}
	}
	if !openaiFound {
		return assistantReplyModelResult{}, errAssistantModelProviderUnavailable
	}
	return assistantReplyModelResult{}, lastTransientErr
}

func (a assistantOpenAIProviderAdapter) RenderReply(ctx context.Context, provider assistantModelProviderConfig, prompt assistantReplyRenderPrompt) (assistantReplyModelResult, error) {
	requestURL, err := assistantBuildOpenAIChatCompletionURL(strings.TrimSpace(provider.Endpoint))
	if err != nil {
		return assistantReplyModelResult{}, errAssistantModelConfigInvalid
	}
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		return assistantReplyModelResult{}, errAssistantModelSecretMissing
	}
	requestCtx := ctx
	if requestCtx == nil {
		requestCtx = context.Background()
	}
	client := a.httpClient
	if client == nil {
		client = &http.Client{}
	}
	promptPayload, _ := json.Marshal(prompt)
	buildPayload := func(enableSchemaFormat bool, plainTextMode bool) assistantOpenAIChatCompletionRequest {
		systemPrompt := "你是企业 HR 助手的最终回复生成器。你会收到机器态上下文 JSON。" +
			"你必须输出给最终用户可直接阅读的自然语言回复，并严格只输出 JSON。" +
			"禁止输出内部实现细节，禁止原样暴露技术错误码。"
		if plainTextMode {
			systemPrompt = "你是企业 HR 助手的最终回复生成器。你会收到机器态上下文 JSON。" +
				"你必须直接输出给最终用户看的自然语言回复。" +
				"禁止输出 JSON、代码块、Markdown、内部实现细节与技术错误码。"
		}
		payload := assistantOpenAIChatCompletionRequest{
			Model:       assistantReplyTargetModelName,
			Temperature: 0,
			TopP:        1,
			N:           1,
			Messages: []assistantOpenAIChatCompletionMessage{
				{
					Role:    "system",
					Content: systemPrompt,
				},
				{
					Role:    "user",
					Content: string(promptPayload),
				},
			},
		}
		if enableSchemaFormat && !plainTextMode {
			payload.ResponseFormat = &assistantOpenAIChatCompletionResponseSpec{
				Type: "json_schema",
				JSONSchema: assistantOpenAIChatJSONSchemaSpec{
					Name:   "assistant_reply_nlg",
					Strict: true,
					Schema: map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"properties": map[string]any{
							"text": map[string]any{
								"type": "string",
							},
							"kind": map[string]any{
								"type": "string",
							},
							"stage": map[string]any{
								"type": "string",
							},
						},
						"required": []string{"text"},
					},
				},
			}
		}
		return payload
	}
	invokePayload := func(payload assistantOpenAIChatCompletionRequest) (assistantOpenAIInvokeResult, error) {
		body, err := assistantOpenAIRequestMarshalFn(payload)
		if err != nil {
			return assistantOpenAIInvokeResult{}, errAssistantModelConfigInvalid
		}
		timeoutCtx, cancel := context.WithTimeout(requestCtx, time.Duration(provider.TimeoutMS)*time.Millisecond)
		defer cancel()
		req, err := assistantOpenAINewRequestWithContextFn(timeoutCtx, http.MethodPost, requestURL, bytes.NewReader(body))
		if err != nil {
			return assistantOpenAIInvokeResult{}, errAssistantModelConfigInvalid
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		resp, err := client.Do(req)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
				return assistantOpenAIInvokeResult{}, errAssistantModelTimeout
			}
			return assistantOpenAIInvokeResult{}, errAssistantModelProviderUnavailable
		}
		defer resp.Body.Close()
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if readErr != nil {
			return assistantOpenAIInvokeResult{}, errAssistantModelProviderUnavailable
		}
		result := assistantOpenAIInvokeResult{RawBody: raw, StatusCode: resp.StatusCode}
		switch {
		case resp.StatusCode == http.StatusTooManyRequests:
			return result, errAssistantModelRateLimited
		case resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusGatewayTimeout:
			return result, errAssistantModelTimeout
		case resp.StatusCode >= 500:
			return result, errAssistantModelProviderUnavailable
		case resp.StatusCode >= 400:
			return result, errAssistantModelConfigInvalid
		}
		return result, nil
	}

	result, err := invokePayload(buildPayload(true, false))
	if err != nil {
		if !errors.Is(err, errAssistantModelConfigInvalid) || !assistantOpenAIResponseFormatUnsupported(result.RawBody) {
			return assistantReplyModelResult{}, err
		}
		result, err = invokePayload(buildPayload(false, false))
		if err != nil {
			return assistantReplyModelResult{}, err
		}
	}
	if decoded, decodeErr := assistantDecodeOpenAIReplyResult(result.RawBody, prompt); decodeErr == nil {
		return decoded, nil
	}
	if decoded, decodeErr := assistantDecodeOpenAIReplyPlainTextResult(result.RawBody, prompt); decodeErr == nil {
		return decoded, nil
	}
	result, err = invokePayload(buildPayload(false, true))
	if err != nil {
		return assistantReplyModelResult{}, err
	}
	return assistantDecodeOpenAIReplyPlainTextResult(result.RawBody, prompt)
}

func assistantDecodeOpenAIReplyPlainTextResult(raw []byte, prompt assistantReplyRenderPrompt) (assistantReplyModelResult, error) {
	var completion assistantOpenAIChatCompletionResponse
	if err := json.Unmarshal(raw, &completion); err != nil {
		return assistantReplyModelResult{}, errAssistantReplyRenderFailed
	}
	if len(completion.Choices) == 0 {
		return assistantReplyModelResult{}, errAssistantReplyRenderFailed
	}
	content := assistantExtractOpenAIMessageContent(completion.Choices[0].Message.Content)
	parsed := assistantParseReplyPayload(content)
	text := strings.TrimSpace(parsed.Text)
	if text == "" {
		text = assistantReplyTextCandidate(content)
	}
	text = assistantSanitizeUserFacingReplyText(text, prompt.Locale)

	resolvedStage := assistantReplyStage(parsed.Stage, prompt.Outcome, nil)
	return assistantReplyModelResult{
		Text:           text,
		Kind:           assistantReplyKind(parsed.Kind, resolvedStage, prompt.Outcome),
		Stage:          resolvedStage,
		ReplyModelName: assistantReplyTargetModelName,
		ReplySource:    assistantReplySourceModel,
		UsedFallback:   false,
	}, nil
}

func assistantDecodeOpenAIReplyResult(raw []byte, prompt assistantReplyRenderPrompt) (assistantReplyModelResult, error) {
	var completion assistantOpenAIChatCompletionResponse
	if err := json.Unmarshal(raw, &completion); err != nil {
		return assistantReplyModelResult{}, errAssistantReplyRenderFailed
	}
	if len(completion.Choices) == 0 {
		return assistantReplyModelResult{}, errAssistantReplyRenderFailed
	}
	content := assistantExtractOpenAIMessageContent(completion.Choices[0].Message.Content)
	parsed := assistantParseReplyPayload(content)
	if strings.TrimSpace(parsed.Text) == "" {
		return assistantReplyModelResult{}, errAssistantReplyRenderFailed
	}
	text := assistantSanitizeUserFacingReplyText(strings.TrimSpace(parsed.Text), prompt.Locale)
	resolvedStage := assistantReplyStage(parsed.Stage, prompt.Outcome, nil)
	return assistantReplyModelResult{
		Text:           text,
		Kind:           assistantReplyKind(parsed.Kind, resolvedStage, prompt.Outcome),
		Stage:          resolvedStage,
		ReplyModelName: assistantReplyTargetModelName,
		ReplySource:    assistantReplySourceModel,
		UsedFallback:   false,
	}, nil
}

type assistantReplyPayload struct {
	Text  string `json:"text"`
	Kind  string `json:"kind"`
	Stage string `json:"stage"`
}

func assistantReplyTextCandidate(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}
	parsed := assistantParseReplyPayload(trimmed)
	if strings.TrimSpace(parsed.Text) != "" {
		return strings.TrimSpace(parsed.Text)
	}
	var generic map[string]any
	if err := json.Unmarshal([]byte(trimmed), &generic); err == nil {
		for _, key := range []string{"reply", "message", "content", "answer", "output_text", "text"} {
			switch value := generic[key].(type) {
			case string:
				if strings.TrimSpace(value) != "" {
					return strings.TrimSpace(value)
				}
			case []any:
				for _, entry := range value {
					if text, ok := entry.(string); ok && strings.TrimSpace(text) != "" {
						return strings.TrimSpace(text)
					}
				}
			}
		}
	}
	return trimmed
}

func assistantParseReplyPayload(content string) assistantReplyPayload {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return assistantReplyPayload{}
	}
	var payload assistantReplyPayload
	if err := json.Unmarshal([]byte(trimmed), &payload); err == nil {
		return payload
	}
	if object, ok := assistantExtractJSONObject(trimmed); ok {
		if err := json.Unmarshal([]byte(object), &payload); err == nil {
			return payload
		}
	}
	return assistantReplyPayload{Text: trimmed}
}
