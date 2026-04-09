package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	assistantIntentSchemaVersionV1     = "assistant.intent.v1"
	assistantCompilerContractVersionV1 = "assistant.compiler.v1"
	assistantCapabilityMapVersionV1    = "2026-02-23"
)

var assistantProviderNameAllowlist = map[string]struct{}{
	"openai":   {},
	"deepseek": {},
	"claude":   {},
	"gemini":   {},
}

type assistantProviderRouting struct {
	Strategy        string `json:"strategy"`
	FallbackEnabled bool   `json:"fallback_enabled"`
}

type assistantModelProviderConfig struct {
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	Model     string `json:"model"`
	Endpoint  string `json:"endpoint"`
	TimeoutMS int    `json:"timeout_ms"`
	Retries   int    `json:"retries"`
	Priority  int    `json:"priority"`
	KeyRef    string `json:"key_ref"`
}

type assistantModelConfig struct {
	ProviderRouting assistantProviderRouting       `json:"provider_routing"`
	Providers       []assistantModelProviderConfig `json:"providers"`
}

type assistantResolveIntentRequest struct {
	Prompt         string
	ConversationID string
	TenantID       string
}

type assistantResolveIntentResult struct {
	Proposal            assistantRuntimeProposal
	Intent              assistantIntentSpec
	SemanticState       assistantConversationSemanticState
	ProviderName        string
	ModelName           string
	ModelRevision       string
	GoalSummary         string
	UserVisibleReply    string
	NextQuestion        string
	Readiness           string
	SelectedCandidateID string
}

type assistantSemanticIntentPayload struct {
	Action              string                              `json:"action"`
	IntentID            string                              `json:"intent_id,omitempty"`
	RouteKind           string                              `json:"route_kind,omitempty"`
	RouteCatalogVersion string                              `json:"route_catalog_version,omitempty"`
	ParentRefText       string                              `json:"parent_ref_text,omitempty"`
	EntityName          string                              `json:"entity_name,omitempty"`
	EffectiveDate       string                              `json:"effective_date,omitempty"`
	OrgCode             string                              `json:"org_code,omitempty"`
	TargetEffectiveDate string                              `json:"target_effective_date,omitempty"`
	NewName             string                              `json:"new_name,omitempty"`
	NewParentRefText    string                              `json:"new_parent_ref_text,omitempty"`
	IntentSchemaVersion string                              `json:"intent_schema_version,omitempty"`
	ContextHash         string                              `json:"context_hash,omitempty"`
	IntentHash          string                              `json:"intent_hash,omitempty"`
	GoalSummary         string                              `json:"goal_summary,omitempty"`
	UserVisibleReply    string                              `json:"user_visible_reply,omitempty"`
	NextQuestion        string                              `json:"next_question,omitempty"`
	Readiness           string                              `json:"readiness,omitempty"`
	SelectedCandidateID string                              `json:"selected_candidate_id,omitempty"`
	RetrievalNeeded     bool                                `json:"retrieval_needed,omitempty"`
	RetrievalRequests   []assistantSemanticRetrievalRequest `json:"retrieval_requests,omitempty"`
	RetrievalResults    []assistantSemanticRetrievalResult  `json:"retrieval_results,omitempty"`
	ConfidenceNote      string                              `json:"confidence_note,omitempty"`
}

func (p assistantSemanticIntentPayload) proposal() assistantRuntimeProposal {
	return assistantNormalizeRuntimeProposal(assistantRuntimeProposal{
		ActionHint:          strings.TrimSpace(p.Action),
		IntentIDHint:        strings.TrimSpace(p.IntentID),
		RouteKindHint:       strings.TrimSpace(p.RouteKind),
		RouteCatalogVersion: strings.TrimSpace(p.RouteCatalogVersion),
		ParentRefText:       strings.TrimSpace(p.ParentRefText),
		EntityName:          strings.TrimSpace(p.EntityName),
		EffectiveDate:       strings.TrimSpace(p.EffectiveDate),
		OrgCode:             strings.TrimSpace(p.OrgCode),
		TargetEffectiveDate: strings.TrimSpace(p.TargetEffectiveDate),
		NewName:             strings.TrimSpace(p.NewName),
		NewParentRefText:    strings.TrimSpace(p.NewParentRefText),
		SelectedCandidateID: strings.TrimSpace(p.SelectedCandidateID),
		Readiness:           strings.TrimSpace(p.Readiness),
		GoalSummary:         strings.TrimSpace(p.GoalSummary),
		UserVisibleReply:    strings.TrimSpace(p.UserVisibleReply),
		NextQuestion:        strings.TrimSpace(p.NextQuestion),
		RetrievalNeeded:     p.RetrievalNeeded,
		RetrievalRequests:   assistantNormalizeSemanticRetrievalRequests(p.RetrievalRequests),
		ConfidenceNote:      strings.TrimSpace(p.ConfidenceNote),
	})
}

// 保留旧 helper 作为兼容桥；其返回值仅是 proposal 的投影，不代表 authoritative intent。
func (p assistantSemanticIntentPayload) intentSpec() assistantIntentSpec {
	return p.proposal().intentSpec()
}

func (p assistantSemanticIntentPayload) semanticState() assistantConversationSemanticState {
	return p.proposal().semanticState()
}

type assistantProviderStatus struct {
	Name         string `json:"name"`
	Healthy      string `json:"healthy"`
	HealthReason string `json:"health_reason,omitempty"`
}

type assistantProviderAdapter interface {
	Invoke(ctx context.Context, prompt string, provider assistantModelProviderConfig) ([]byte, error)
}

type assistantProviderHealthProber interface {
	Probe(ctx context.Context, provider assistantModelProviderConfig) error
}

var assistantIntentMarshalFn = json.Marshal
var assistantOpenAIRequestMarshalFn = json.Marshal
var assistantOpenAINewRequestWithContextFn = http.NewRequestWithContext

func assistantDefaultOpenAIHTTPClient() *http.Client {
	return nil
}

var assistantOpenAIHTTPClientFactory = assistantDefaultOpenAIHTTPClient

var errAssistantModelProbeUnsupported = errors.New("assistant_model_probe_unsupported")

type assistantDeterministicProviderAdapter struct{}

func (assistantDeterministicProviderAdapter) Invoke(_ context.Context, prompt string, provider assistantModelProviderConfig) ([]byte, error) {
	endpoint := strings.ToLower(strings.TrimSpace(provider.Endpoint))
	switch {
	case assistantIsSimulateEndpoint(endpoint) && strings.HasPrefix(endpoint, "simulate://timeout"):
		return nil, errAssistantModelTimeout
	case assistantIsSimulateEndpoint(endpoint) && strings.HasPrefix(endpoint, "simulate://rate-limit"):
		return nil, errAssistantModelRateLimited
	case assistantIsSimulateEndpoint(endpoint) && strings.HasPrefix(endpoint, "simulate://unavailable"):
		return nil, errAssistantModelProviderUnavailable
	}
	payload, err := assistantIntentMarshalFn(assistantSyntheticSemanticPayloadForPrompt(prompt))
	if err != nil {
		return nil, errAssistantPlanSchemaConstrainedDecodeFailed
	}
	return payload, nil
}

func (assistantDeterministicProviderAdapter) Probe(_ context.Context, provider assistantModelProviderConfig) error {
	endpoint := strings.ToLower(strings.TrimSpace(provider.Endpoint))
	switch {
	case assistantIsSimulateEndpoint(endpoint) && strings.HasPrefix(endpoint, "simulate://timeout"):
		return errAssistantModelTimeout
	case assistantIsSimulateEndpoint(endpoint) && strings.HasPrefix(endpoint, "simulate://rate-limit"):
		return errAssistantModelRateLimited
	case assistantIsSimulateEndpoint(endpoint) && strings.HasPrefix(endpoint, "simulate://unavailable"):
		return errAssistantModelProviderUnavailable
	case assistantEndpointInvalidForRuntime(provider.Endpoint):
		return errAssistantModelConfigInvalid
	default:
		return nil
	}
}

func assistantSyntheticSemanticPayloadForPrompt(prompt string) assistantSemanticIntentPayload {
	return assistantSyntheticSemanticPayload(assistantSemanticCurrentUserInput(prompt))
}

func assistantSyntheticSemanticPayload(userInput string) assistantSemanticIntentPayload {
	text := strings.TrimSpace(userInput)
	payload := assistantSemanticIntentPayload{
		GoalSummary: text,
	}

	actionID := assistantSyntheticSemanticAction(text)
	switch {
	case actionID != "":
		payload.Action = actionID
		payload.RouteKind = assistantRouteKindBusinessAction
		payload.IntentID = assistantSemanticIntentIDForAction(actionID)
	case assistantSyntheticSemanticLooksLikeKnowledgeQA(text):
		payload.Action = assistantIntentPlanOnly
		payload.RouteKind = assistantRouteKindKnowledgeQA
		payload.IntentID = "knowledge.general_qa"
	case assistantSyntheticSemanticLooksLikeChitchat(text):
		payload.Action = assistantIntentPlanOnly
		payload.RouteKind = assistantRouteKindChitchat
		payload.IntentID = "chat.greeting"
	default:
		payload.Action = assistantIntentPlanOnly
		payload.RouteKind = assistantRouteKindUncertain
		payload.IntentID = "route.uncertain"
	}
	return payload
}

func assistantSyntheticSemanticAction(userInput string) string {
	text := strings.TrimSpace(userInput)
	switch {
	case text == "":
		return ""
	case strings.Contains(text, "插入版本"):
		return assistantIntentInsertOrgUnitVersion
	case strings.Contains(text, "新增版本"):
		return assistantIntentAddOrgUnitVersion
	case strings.Contains(text, "更正"):
		return assistantIntentCorrectOrgUnit
	case strings.Contains(text, "移动"):
		return assistantIntentMoveOrgUnit
	case strings.Contains(text, "重命名"):
		return assistantIntentRenameOrgUnit
	case strings.Contains(text, "停用"), strings.Contains(text, "禁用"):
		return assistantIntentDisableOrgUnit
	case strings.Contains(text, "启用"):
		return assistantIntentEnableOrgUnit
	case strings.Contains(text, "新建"), strings.Contains(text, "创建"):
		return assistantIntentCreateOrgUnit
	default:
		return ""
	}
}

func assistantSemanticIntentIDForAction(actionID string) string {
	switch strings.TrimSpace(actionID) {
	case assistantIntentCreateOrgUnit:
		return "org.orgunit_create"
	case assistantIntentAddOrgUnitVersion:
		return "org.orgunit_add_version"
	case assistantIntentInsertOrgUnitVersion:
		return "org.orgunit_insert_version"
	case assistantIntentCorrectOrgUnit:
		return "org.orgunit_correct"
	case assistantIntentRenameOrgUnit:
		return "org.orgunit_rename"
	case assistantIntentMoveOrgUnit:
		return "org.orgunit_move"
	case assistantIntentDisableOrgUnit:
		return "org.orgunit_disable"
	case assistantIntentEnableOrgUnit:
		return "org.orgunit_enable"
	default:
		return "action." + strings.TrimSpace(actionID)
	}
}

func assistantSyntheticSemanticLooksLikeKnowledgeQA(text string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	for _, keyword := range []string{"功能", "help", "怎么", "如何", "什么", "哪些", "支持", "?", "？"} {
		if strings.Contains(strings.ToLower(text), strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func assistantSyntheticSemanticLooksLikeChitchat(text string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	for _, keyword := range []string{"你好", "您好", "hello", "hi", "thanks", "谢谢"} {
		if strings.Contains(strings.ToLower(text), strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

type assistantOpenAIProviderAdapter struct {
	httpClient *http.Client
	fallback   assistantProviderAdapter
}

type assistantOpenAIChatCompletionRequest struct {
	Model          string                                     `json:"model"`
	Temperature    float64                                    `json:"temperature"`
	TopP           float64                                    `json:"top_p"`
	N              int                                        `json:"n"`
	Messages       []assistantOpenAIChatCompletionMessage     `json:"messages"`
	ResponseFormat *assistantOpenAIChatCompletionResponseSpec `json:"response_format,omitempty"`
}

type assistantOpenAIChatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type assistantOpenAIChatCompletionResponseSpec struct {
	Type       string                            `json:"type"`
	JSONSchema assistantOpenAIChatJSONSchemaSpec `json:"json_schema"`
}

type assistantOpenAIChatJSONSchemaSpec struct {
	Name   string `json:"name"`
	Strict bool   `json:"strict"`
	Schema any    `json:"schema"`
}

type assistantOpenAIChatCompletionResponse struct {
	Choices []assistantOpenAIChatCompletionChoice `json:"choices"`
}

type assistantOpenAIChatCompletionChoice struct {
	Message assistantOpenAIChatCompletionChoiceMessage `json:"message"`
}

type assistantOpenAIChatCompletionChoiceMessage struct {
	Content any `json:"content"`
}

type assistantOpenAIInvokeResult struct {
	RawBody    []byte
	StatusCode int
}

func (a assistantOpenAIProviderAdapter) Invoke(ctx context.Context, prompt string, provider assistantModelProviderConfig) ([]byte, error) {
	endpoint := strings.TrimSpace(provider.Endpoint)
	requestURL, err := assistantBuildOpenAIChatCompletionURL(endpoint)
	if err != nil {
		return nil, errAssistantModelConfigInvalid
	}
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		return nil, errAssistantModelSecretMissing
	}
	requestCtx := ctx
	if requestCtx == nil {
		requestCtx = context.Background()
	}
	client := a.httpClient
	if client == nil {
		client = &http.Client{}
	}
	buildPayload := func(enableSchemaFormat bool) assistantOpenAIChatCompletionRequest {
		payload := assistantOpenAIChatCompletionRequest{
			Model:       strings.TrimSpace(provider.Model),
			Temperature: 0,
			TopP:        1,
			N:           1,
			Messages: []assistantOpenAIChatCompletionMessage{
				{
					Role: "system",
					Content: "你是企业 HR 组织变更助手。你会收到一个包含当前用户输入、允许动作、以及可能待续上下文的 JSON。" +
						"你必须只输出严格 JSON，禁止输出解释、Markdown 或其他文本。" +
						"你需要同时输出结构化动作槽位、route_kind、intent_id、当前给用户看的自然语言回复、下一句追问，以及当前 readiness。" +
						"当你需要本地补充候选组织事实时，必须输出 retrieval_requests，并只允许使用 candidate_lookup。" +
						"所有日期统一输出 YYYY-MM-DD；action 只能从 allowed_actions 中选择。" +
						"业务动作必须输出 route_kind=business_action，且 intent_id 必须使用固定映射：" +
						"create_orgunit=org.orgunit_create；add_orgunit_version=org.orgunit_add_version；insert_orgunit_version=org.orgunit_insert_version；correct_orgunit=org.orgunit_correct；move_orgunit=org.orgunit_move；rename_orgunit=org.orgunit_rename；disable_orgunit=org.orgunit_disable；enable_orgunit=org.orgunit_enable。" +
						"知识问答输出 action=plan_only、route_kind=knowledge_qa、intent_id=knowledge.general_qa；" +
						"闲聊输出 action=plan_only、route_kind=chitchat、intent_id=chat.greeting；" +
						"无法确定时输出 action=plan_only、route_kind=uncertain、intent_id=route.uncertain。",
				},
				{
					Role:    "user",
					Content: strings.TrimSpace(prompt),
				},
			},
		}
		if enableSchemaFormat {
			payload.ResponseFormat = &assistantOpenAIChatCompletionResponseSpec{
				Type: "json_schema",
				JSONSchema: assistantOpenAIChatJSONSchemaSpec{
					Name:   "assistant_intent_spec",
					Strict: true,
					Schema: map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"properties": map[string]any{
							"action": map[string]any{
								"type": "string",
							},
							"intent_id": map[string]any{
								"type": "string",
							},
							"route_kind": map[string]any{
								"type": "string",
							},
							"route_catalog_version": map[string]any{
								"type": "string",
							},
							"parent_ref_text": map[string]any{
								"type": "string",
							},
							"entity_name": map[string]any{
								"type": "string",
							},
							"effective_date": map[string]any{
								"type": "string",
							},
							"org_code": map[string]any{
								"type": "string",
							},
							"target_effective_date": map[string]any{
								"type": "string",
							},
							"new_name": map[string]any{
								"type": "string",
							},
							"new_parent_ref_text": map[string]any{
								"type": "string",
							},
							"goal_summary": map[string]any{
								"type": "string",
							},
							"user_visible_reply": map[string]any{
								"type": "string",
							},
							"next_question": map[string]any{
								"type": "string",
							},
							"readiness": map[string]any{
								"type": "string",
							},
							"selected_candidate_id": map[string]any{
								"type": "string",
							},
							"retrieval_needed": map[string]any{
								"type": "boolean",
							},
							"confidence_note": map[string]any{
								"type": "string",
							},
							"retrieval_requests": map[string]any{
								"type": "array",
								"items": map[string]any{
									"type":                 "object",
									"additionalProperties": false,
									"properties": map[string]any{
										"kind": map[string]any{
											"type": "string",
										},
										"slot": map[string]any{
											"type": "string",
										},
										"ref_text": map[string]any{
											"type": "string",
										},
										"as_of": map[string]any{
											"type": "string",
										},
										"limit": map[string]any{
											"type": "integer",
										},
									},
									"required": []string{"kind"},
								},
							},
						},
						"required": []string{"action", "intent_id", "route_kind"},
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
		result := assistantOpenAIInvokeResult{
			RawBody:    raw,
			StatusCode: resp.StatusCode,
		}
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
	decodeContent := func(raw []byte) ([]byte, error) {
		var completion assistantOpenAIChatCompletionResponse
		if err := json.Unmarshal(raw, &completion); err != nil {
			return nil, errAssistantPlanSchemaConstrainedDecodeFailed
		}
		if len(completion.Choices) == 0 {
			return nil, errAssistantPlanSchemaConstrainedDecodeFailed
		}
		content := assistantExtractOpenAIMessageContent(completion.Choices[0].Message.Content)
		if strings.TrimSpace(content) == "" {
			return nil, errAssistantPlanSchemaConstrainedDecodeFailed
		}
		return assistantNormalizeOpenAIIntentPayload(content), nil
	}

	result, err := invokePayload(buildPayload(true))
	if err != nil {
		if !errors.Is(err, errAssistantModelConfigInvalid) || !assistantOpenAIResponseFormatUnsupported(result.RawBody) {
			return nil, err
		}
		result, err = invokePayload(buildPayload(false))
		if err != nil {
			return nil, err
		}
	}
	return decodeContent(result.RawBody)
}

func (a assistantOpenAIProviderAdapter) Probe(ctx context.Context, provider assistantModelProviderConfig) error {
	requestURL, err := assistantBuildOpenAIModelsURL(provider.Endpoint)
	if err != nil {
		return errAssistantModelConfigInvalid
	}
	apiKey := strings.TrimSpace(os.Getenv(strings.TrimSpace(provider.KeyRef)))
	if apiKey == "" {
		return errAssistantModelSecretMissing
	}
	requestCtx := ctx
	if requestCtx == nil {
		requestCtx = context.Background()
	}
	client := a.httpClient
	if client == nil {
		client = &http.Client{}
	}
	timeoutMS := assistantProbeTimeoutMS(provider.TimeoutMS)
	timeoutCtx, cancel := context.WithTimeout(requestCtx, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()
	req, err := assistantOpenAINewRequestWithContextFn(timeoutCtx, http.MethodGet, requestURL, nil)
	if err != nil {
		return errAssistantModelConfigInvalid
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
			return errAssistantModelTimeout
		}
		return errAssistantModelProviderUnavailable
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == http.StatusTooManyRequests:
		return errAssistantModelRateLimited
	case resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusGatewayTimeout:
		return errAssistantModelTimeout
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return errAssistantModelSecretMissing
	case resp.StatusCode >= 500:
		return errAssistantModelProviderUnavailable
	default:
		return errAssistantModelConfigInvalid
	}
}

func assistantOpenAIResponseFormatUnsupported(raw []byte) bool {
	if len(raw) == 0 {
		return false
	}
	return strings.Contains(strings.ToLower(string(raw)), "response_format")
}

func assistantNormalizeOpenAIIntentPayload(content string) []byte {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return []byte(trimmed)
	}
	obj, ok := assistantDecodeOpenAIIntentPayloadObject(trimmed)
	if !ok {
		return []byte(trimmed)
	}
	objects := assistantIntentCandidateObjects(obj)
	action := assistantNormalizeOpenAIIntentAction(assistantFirstStringFromObjects(objects,
		"action",
		"proposed_action",
		"proposedAction",
		"intent_action",
		"intentAction",
		"operationType",
		"operation",
		"type"))
	parentRefText := assistantFirstStringFromObjects(objects,
		"parent_ref_text",
		"parentRefText",
		"parent_department",
		"parentDepartment",
		"parent_department_name",
		"parentDepartmentName",
		"parent_org_name",
		"parentOrgName",
		"parent_org",
		"parentOrg",
		"parent_orgunit",
		"parentOrgunit",
		"parent_org_unit",
		"parentOrgUnit",
		"parent_org_unit_name",
		"parentOrgUnitName",
		"parent_unit",
		"parentUnit",
		"parent_organization",
		"parentOrganization",
		"parent_organization_name",
		"parentOrganizationName",
		"parent",
		"parent.name",
		"parent.code",
		"parentOrg.name",
		"parentOrg.code",
		"parent_org.name",
		"parent_org.code",
		"parentDepartment.name",
		"parentDepartment.code",
		"parent_department.name",
		"parent_department.code",
		"parentOrganization.name",
		"parentOrganization.code",
		"parent_organization.name",
		"parent_organization.code",
		"organizationUnit.parent",
		"department.parentOrgUnitName",
		"department.parent_org_unit_name",
		"department.parentOrgUnit",
		"department.parent_org_unit",
		"newDepartment.parentOrgUnitName",
		"new_department.parent_org_unit_name",
		"organization.parentOrganization",
		"organization.parentOrganizationName",
		"newOrganization.parentOrganization",
		"newOrganization.parentOrganizationName",
		"data.parent",
		"data.parentName",
		"data.parentOrgName",
		"data.parentOrganizationName",
		"data.parent_org_name",
		"params.parent",
		"params.parentName",
		"params.parentOrgName",
		"params.parentOrganizationName",
		"params.parent_org_unit_name",
		"target.parent",
		"target.parentName",
		"target.parentOrgName",
		"orgUnit.parent",
		"orgUnit.parentName")
	orgCode := assistantFirstStringFromObjects(objects,
		"org_code",
		"orgCode",
		"code",
		"target_org_code",
		"targetOrgCode",
		"org_unit_code",
		"orgUnitCode",
		"organizationCode",
		"departmentCode",
		"data.orgCode",
		"data.org_code",
		"params.orgCode",
		"params.org_code",
		"target.orgCode",
		"target.org_code",
		"orgUnit.code")
	targetEffectiveDate := assistantFirstStringFromObjects(objects,
		"target_effective_date",
		"targetEffectiveDate",
		"as_of_date",
		"asOfDate",
		"version_date",
		"target.effectiveDate",
		"target.effective_date",
		"data.targetEffectiveDate",
		"data.target_effective_date",
		"params.targetEffectiveDate",
		"params.target_effective_date")
	newName := assistantFirstStringFromObjects(objects,
		"new_name",
		"newName",
		"target_name",
		"targetName",
		"new_department_name",
		"newDepartmentName",
		"data.newName",
		"data.new_name",
		"params.newName",
		"params.new_name",
		"target.name")
	newParentRefText := assistantFirstStringFromObjects(objects,
		"new_parent_ref_text",
		"newParentRefText",
		"new_parent_name",
		"newParentName",
		"target_parent",
		"targetParent",
		"new_parent_org_name",
		"newParentOrgName",
		"data.newParent",
		"data.new_parent",
		"data.newParentName",
		"params.newParent",
		"params.new_parent",
		"params.newParentName",
		"target.parent",
		"target.parentName")
	entityName := assistantFirstStringFromObjects(objects,
		"entity_name",
		"entityName",
		"department_name",
		"departmentName",
		"org_name",
		"orgName",
		"orgunit_name",
		"orgunitName",
		"new_org_name",
		"newOrgName",
		"new_department_name",
		"newDepartmentName",
		"new_org_unit_name",
		"newOrgUnitName",
		"name",
		"newOrg.name",
		"new_org.name",
		"newDepartment.name",
		"new_department.name",
		"newOrgUnit.name",
		"new_org_unit.name",
		"org_unit.name",
		"department.name",
		"organization.name",
		"organizationUnit.name",
		"newOrganization.name",
		"data.name",
		"data.orgName",
		"data.newOrgName",
		"data.new_org_name",
		"data.newOrgUnitName",
		"data.new_org_unit_name",
		"params.name",
		"params.orgName",
		"params.newOrgName",
		"params.new_org_name",
		"params.newOrgUnitName",
		"params.new_org_unit_name",
		"target.name",
		"target.orgName",
		"orgUnit.name")
	if entityName == "" {
		entityName = assistantFirstCompositeNameFromObjects(objects,
			"department",
			"organization",
			"organizationUnit",
			"data",
			"params",
			"target",
			"orgUnit",
			"newDepartment",
			"newOrganization",
			"newOrg",
			"newOrgUnit")
	}
	effectiveDate := assistantFirstStringFromObjects(objects,
		"effective_date",
		"effectiveDate",
		"established_date",
		"establishment_date",
		"start_date",
		"startDate",
		"date",
		"newOrg.effectiveDate",
		"newOrg.effective_date",
		"new_org.effective_date",
		"newDepartment.effectiveDate",
		"newDepartment.effective_date",
		"new_department.effective_date",
		"newOrgUnit.effectiveDate",
		"newOrgUnit.effective_date",
		"new_org_unit.effective_date",
		"department.effectiveDate",
		"department.effective_date",
		"organization.effectiveDate",
		"organization.effective_date",
		"organizationUnit.effectiveDate",
		"organizationUnit.effective_date",
		"org_unit.effectiveDate",
		"org_unit.effective_date",
		"newOrganization.effectiveDate",
		"newOrganization.effective_date",
		"data.effectiveDate",
		"data.effective_date",
		"params.effectiveDate",
		"params.effective_date",
		"target.effectiveDate",
		"target.effective_date",
		"orgUnit.effectiveDate",
		"orgUnit.effective_date")
	if (action == "" || assistantIsGenericCreateAction(action)) && parentRefText != "" && entityName != "" {
		action = assistantIntentCreateOrgUnit
	}
	normalized := map[string]any{}
	if action != "" {
		normalized["action"] = action
	}
	if intentID := assistantFirstStringFromObjects(objects,
		"intent_id",
		"intentId"); intentID != "" {
		normalized["intent_id"] = intentID
	}
	if routeKind := assistantFirstStringFromObjects(objects,
		"route_kind",
		"routeKind"); routeKind != "" {
		normalized["route_kind"] = routeKind
	}
	if routeCatalogVersion := assistantFirstStringFromObjects(objects,
		"route_catalog_version",
		"routeCatalogVersion"); routeCatalogVersion != "" {
		normalized["route_catalog_version"] = routeCatalogVersion
	}
	if parentRefText != "" {
		normalized["parent_ref_text"] = parentRefText
	}
	if entityName != "" {
		normalized["entity_name"] = entityName
	}
	if effectiveDate != "" {
		normalized["effective_date"] = effectiveDate
	}
	if orgCode != "" {
		normalized["org_code"] = orgCode
	}
	if targetEffectiveDate != "" {
		normalized["target_effective_date"] = targetEffectiveDate
	}
	if newName != "" {
		normalized["new_name"] = newName
	}
	if newParentRefText != "" {
		normalized["new_parent_ref_text"] = newParentRefText
	}
	if goalSummary := assistantFirstStringFromObjects(objects,
		"goal_summary",
		"goalSummary",
		"summary",
		"intent_summary"); goalSummary != "" {
		normalized["goal_summary"] = goalSummary
	}
	if userVisibleReply := assistantFirstStringFromObjects(objects,
		"user_visible_reply",
		"userVisibleReply",
		"reply",
		"response",
		"message"); userVisibleReply != "" {
		normalized["user_visible_reply"] = userVisibleReply
	}
	if nextQuestion := assistantFirstStringFromObjects(objects,
		"next_question",
		"nextQuestion",
		"follow_up_question",
		"followUpQuestion"); nextQuestion != "" {
		normalized["next_question"] = nextQuestion
	}
	if readiness := assistantNormalizeOpenAIReadiness(assistantFirstStringFromObjects(objects,
		"readiness",
		"state",
		"status")); readiness != "" {
		normalized["readiness"] = readiness
	}
	if selectedCandidateID := assistantFirstStringFromObjects(objects,
		"selected_candidate_id",
		"selectedCandidateId",
		"candidate_id",
		"candidateId"); selectedCandidateID != "" {
		normalized["selected_candidate_id"] = selectedCandidateID
	}
	if retrievalNeeded, ok := assistantFirstBoolFromObjects(objects,
		"retrieval_needed",
		"retrievalNeeded"); ok {
		normalized["retrieval_needed"] = retrievalNeeded
	}
	if confidenceNote := assistantFirstStringFromObjects(objects,
		"confidence_note",
		"confidenceNote"); confidenceNote != "" {
		normalized["confidence_note"] = confidenceNote
	}
	if retrievalRequests := assistantNormalizeOpenAIRetrievalRequests(obj); len(retrievalRequests) > 0 {
		normalized["retrieval_requests"] = retrievalRequests
	}
	if retrievalResults := assistantNormalizeOpenAIRetrievalResults(obj); len(retrievalResults) > 0 {
		normalized["retrieval_results"] = retrievalResults
	}
	if len(normalized) == 0 {
		return []byte(trimmed)
	}
	payload, _ := json.Marshal(normalized)
	return payload
}

func assistantDecodeOpenAIIntentPayloadObject(content string) (map[string]any, bool) {
	var object map[string]any
	if err := json.Unmarshal([]byte(content), &object); err == nil {
		return object, true
	}
	extracted, ok := assistantExtractJSONObject(content)
	if !ok {
		return nil, false
	}
	if err := json.Unmarshal([]byte(extracted), &object); err != nil {
		return nil, false
	}
	return object, true
}

func assistantExtractJSONObject(content string) (string, bool) {
	start := strings.IndexByte(content, '{')
	if start < 0 {
		return "", false
	}
	depth := 0
	inString := false
	escaped := false
	for index := start; index < len(content); index++ {
		ch := content[index]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return strings.TrimSpace(content[start : index+1]), true
			}
		}
	}
	return "", false
}

func assistantNormalizeOpenAIReadiness(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "ready_for_confirm", "ready_for_confirmation", "ready":
		return assistantSemanticReadinessReadyForConfirm
	case "ready_for_dry_run", "draft_ready":
		return assistantSemanticReadinessReadyForDryRun
	case "non_business", "knowledge", "qa", "chitchat":
		return assistantSemanticReadinessNonBusiness
	case "need_more_info", "missing_info", "clarification_required", "clarify":
		return assistantSemanticReadinessNeedMoreInfo
	default:
		return ""
	}
}

func assistantFirstString(object map[string]any, keys ...string) string {
	for _, key := range keys {
		value, exists := object[key]
		if !exists {
			continue
		}
		text, ok := value.(string)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(text)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func assistantFirstStringByPaths(object map[string]any, paths ...string) string {
	for _, item := range paths {
		value, ok := assistantLookupPathValue(object, item)
		if !ok {
			continue
		}
		if text, ok := assistantToNonEmptyString(value); ok {
			return text
		}
	}
	return ""
}

func assistantFirstStringFromObjects(objects []map[string]any, paths ...string) string {
	for _, object := range objects {
		if object == nil {
			continue
		}
		if text := assistantFirstStringByPaths(object, paths...); text != "" {
			return text
		}
	}
	return ""
}

func assistantFirstCompositeNameFromObjects(objects []map[string]any, objectPaths ...string) string {
	for _, object := range objects {
		for _, objectPath := range objectPaths {
			value, ok := assistantLookupPathValue(object, objectPath)
			if !ok {
				continue
			}
			asMap, ok := value.(map[string]any)
			if !ok {
				continue
			}
			name, _ := assistantToNonEmptyString(assistantLookupLooseAny(asMap, "name", "orgUnitName", "org_unit_name", "departmentName", "organizationName"))
			code, _ := assistantToNonEmptyString(assistantLookupLooseAny(asMap, "code", "orgUnitCode", "org_unit_code", "departmentCode", "organizationCode"))
			switch {
			case code != "" && name != "":
				if strings.Contains(name, code) {
					return name
				}
				return code + name
			case name != "":
				return name
			case code != "":
				return code
			}
		}
	}
	return ""
}

func assistantIntentCandidateObjects(object map[string]any) []map[string]any {
	objects := []map[string]any{object}
	for _, key := range []string{"slots", "slot_values", "slotValues", "arguments", "data", "params", "target"} {
		if nested, ok := assistantLookupLooseMap(object, key); ok {
			objects = append([]map[string]any{nested}, objects...)
		}
	}
	for _, key := range []string{"changes", "operations", "items"} {
		if nested, ok := assistantFirstMapFromSlice(object, key); ok {
			objects = append([]map[string]any{nested}, objects...)
		}
	}
	for _, key := range []string{"actions"} {
		if nested, ok := assistantFirstMapFromSlice(object, key); ok {
			objects = append([]map[string]any{nested}, objects...)
		}
	}
	return objects
}

func assistantLookupLooseMap(object map[string]any, key string) (map[string]any, bool) {
	value, ok := assistantLookupMapValueLoose(object, key)
	if !ok {
		return nil, false
	}
	asMap, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	return asMap, true
}

func assistantFirstMapFromSlice(object map[string]any, key string) (map[string]any, bool) {
	value, ok := assistantLookupMapValueLoose(object, key)
	if !ok {
		return nil, false
	}
	items, ok := value.([]any)
	if !ok || len(items) == 0 {
		return nil, false
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		return nil, false
	}
	return first, true
}

func assistantLookupLooseAny(object map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := assistantLookupMapValueLoose(object, key); ok {
			return value
		}
	}
	return nil
}

func assistantFirstBoolFromObjects(objects []map[string]any, paths ...string) (bool, bool) {
	for _, object := range objects {
		for _, path := range paths {
			value, ok := assistantLookupPathValue(object, path)
			if !ok {
				continue
			}
			switch typed := value.(type) {
			case bool:
				return typed, true
			case string:
				switch strings.TrimSpace(strings.ToLower(typed)) {
				case "true", "yes", "1":
					return true, true
				case "false", "no", "0":
					return false, true
				}
			}
		}
	}
	return false, false
}

func assistantNormalizeOpenAIRetrievalRequests(object map[string]any) []assistantSemanticRetrievalRequest {
	requests := assistantLooseObjectSlice(object,
		"retrieval_requests",
		"retrievalRequests",
		"lookups",
		"lookup_requests",
		"lookupRequests")
	if len(requests) == 0 {
		return nil
	}
	out := make([]assistantSemanticRetrievalRequest, 0, len(requests))
	for _, item := range requests {
		request := assistantSemanticRetrievalRequest{
			Kind: strings.TrimSpace(firstNonEmpty(
				assistantFirstString(item, "kind", "type"),
			)),
			Slot: strings.TrimSpace(assistantFirstString(item, "slot", "field")),
			RefText: strings.TrimSpace(assistantFirstString(item,
				"ref_text",
				"refText",
				"query",
				"query_text",
				"queryText",
				"name")),
			AsOf:  strings.TrimSpace(assistantFirstString(item, "as_of", "asOf")),
			Limit: assistantFirstInt(item, "limit"),
		}
		if request.Kind == "" {
			continue
		}
		out = append(out, request)
	}
	return assistantNormalizeSemanticRetrievalRequests(out)
}

func assistantNormalizeOpenAIRetrievalResults(object map[string]any) []assistantSemanticRetrievalResult {
	results := assistantLooseObjectSlice(object,
		"retrieval_results",
		"retrievalResults",
		"lookup_results",
		"lookupResults")
	if len(results) == 0 {
		return nil
	}
	out := make([]assistantSemanticRetrievalResult, 0, len(results))
	for _, item := range results {
		result := assistantSemanticRetrievalResult{
			Kind:                strings.TrimSpace(assistantFirstString(item, "kind", "type")),
			Slot:                strings.TrimSpace(assistantFirstString(item, "slot", "field")),
			State:               strings.TrimSpace(assistantFirstString(item, "state", "status")),
			RefText:             strings.TrimSpace(assistantFirstString(item, "ref_text", "refText", "query", "query_text", "queryText")),
			AsOf:                strings.TrimSpace(assistantFirstString(item, "as_of", "asOf")),
			CandidateCount:      assistantFirstInt(item, "candidate_count", "candidateCount", "count"),
			SelectedCandidateID: strings.TrimSpace(assistantFirstString(item, "selected_candidate_id", "selectedCandidateId", "candidate_id", "candidateId")),
		}
		if ids := assistantLooseStringSlice(item, "candidate_ids", "candidateIds"); len(ids) > 0 {
			result.CandidateIDs = ids
		}
		if result.Kind == "" {
			continue
		}
		out = append(out, result)
	}
	return assistantNormalizeSemanticRetrievalResults(out)
}

func assistantLooseObjectSlice(object map[string]any, keys ...string) []map[string]any {
	for _, key := range keys {
		value, ok := assistantLookupMapValueLoose(object, key)
		if !ok {
			continue
		}
		items, ok := value.([]any)
		if !ok {
			continue
		}
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			asMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			out = append(out, asMap)
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

func assistantLooseStringSlice(object map[string]any, keys ...string) []string {
	for _, key := range keys {
		value, ok := assistantLookupMapValueLoose(object, key)
		if !ok {
			continue
		}
		items, ok := value.([]any)
		if !ok {
			continue
		}
		out := make([]string, 0, len(items))
		for _, item := range items {
			text, ok := assistantToNonEmptyString(item)
			if !ok {
				continue
			}
			out = append(out, text)
		}
		if len(out) > 0 {
			return assistantNormalizeRouteStringSlice(out)
		}
	}
	return nil
}

func assistantFirstInt(object map[string]any, keys ...string) int {
	for _, key := range keys {
		value, ok := assistantLookupMapValueLoose(object, key)
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return int(typed)
		case int:
			return typed
		case int64:
			return int(typed)
		case json.Number:
			if parsed, err := typed.Int64(); err == nil {
				return int(parsed)
			}
		case string:
			if parsed, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil {
				return parsed
			}
		}
	}
	return 0
}

func assistantLookupPathValue(object map[string]any, path string) (any, bool) {
	current := any(object)
	segments := strings.Split(strings.TrimSpace(path), ".")
	for _, segment := range segments {
		key := strings.TrimSpace(segment)
		if key == "" {
			return nil, false
		}
		asMap, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := assistantLookupMapValueLoose(asMap, key)
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

func assistantLookupMapValueLoose(object map[string]any, key string) (any, bool) {
	if value, ok := object[key]; ok {
		return value, true
	}
	target := assistantLooseKey(key)
	for candidateKey, candidateValue := range object {
		if assistantLooseKey(candidateKey) == target {
			return candidateValue, true
		}
	}
	return nil, false
}

func assistantLooseKey(key string) string {
	normalized := strings.ToLower(strings.TrimSpace(key))
	normalized = strings.ReplaceAll(normalized, "_", "")
	normalized = strings.ReplaceAll(normalized, "-", "")
	return normalized
}

func assistantToNonEmptyString(value any) (string, bool) {
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", false
	}
	return trimmed, true
}

func assistantNormalizeOpenAIIntentAction(action string) string {
	trimmed := strings.TrimSpace(action)
	if trimmed == "" {
		return ""
	}
	normalized := strings.ToLower(trimmed)
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	switch normalized {
	case assistantIntentCreateOrgUnit,
		"create_department",
		"createdepartment",
		"create_org_unit",
		"create_organization",
		"create_organization_unit",
		"createorganizationunit",
		"orgunit_create":
		return assistantIntentCreateOrgUnit
	case assistantIntentAddOrgUnitVersion, "add_version", "add_org_version", "orgunit_add_version":
		return assistantIntentAddOrgUnitVersion
	case assistantIntentInsertOrgUnitVersion, "insert_version", "insert_org_version", "orgunit_insert_version":
		return assistantIntentInsertOrgUnitVersion
	case assistantIntentCorrectOrgUnit, "correct", "correct_version", "orgunit_correct":
		return assistantIntentCorrectOrgUnit
	case assistantIntentRenameOrgUnit, "rename", "rename_department", "orgunit_rename":
		return assistantIntentRenameOrgUnit
	case assistantIntentMoveOrgUnit, "move", "move_department", "orgunit_move":
		return assistantIntentMoveOrgUnit
	case assistantIntentDisableOrgUnit, "disable", "disable_department", "orgunit_disable":
		return assistantIntentDisableOrgUnit
	case assistantIntentEnableOrgUnit, "enable", "enable_department", "orgunit_enable":
		return assistantIntentEnableOrgUnit
	default:
		compact := strings.ReplaceAll(normalized, "_", "")
		if strings.HasPrefix(compact, "create") &&
			(strings.Contains(compact, "department") ||
				strings.Contains(compact, "orgunit") ||
				strings.Contains(compact, "organizationunit") ||
				strings.Contains(compact, "organization") ||
				strings.Contains(compact, "org")) {
			return assistantIntentCreateOrgUnit
		}
		return trimmed
	}
}

func assistantIsGenericCreateAction(action string) bool {
	normalized := assistantLooseKey(action)
	switch normalized {
	case "create", "add", "insert", "new":
		return true
	default:
		return false
	}
}

type assistantModelGateway struct {
	mu       sync.RWMutex
	config   assistantModelConfig
	adapters map[string]assistantProviderAdapter
}

func newAssistantModelGateway() (*assistantModelGateway, error) {
	openAIClient := assistantOpenAIHTTPClientFactory()
	gateway := &assistantModelGateway{
		adapters: map[string]assistantProviderAdapter{
			"openai": assistantOpenAIProviderAdapter{httpClient: openAIClient},
		},
	}
	fromEnv := strings.TrimSpace(os.Getenv("ASSISTANT_MODEL_CONFIG_JSON"))
	if fromEnv == "" {
		return nil, errAssistantRuntimeConfigMissing
	}
	var parsed assistantModelConfig
	if err := json.Unmarshal([]byte(fromEnv), &parsed); err != nil {
		return nil, errAssistantRuntimeConfigInvalid
	}
	normalized, errs := normalizeAssistantModelConfig(parsed, false)
	if len(errs) > 0 {
		return nil, errAssistantRuntimeConfigInvalid
	}
	gateway.config = normalized
	return gateway, nil
}

func defaultAssistantModelConfig() assistantModelConfig {
	return assistantModelConfig{
		ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
		Providers: []assistantModelProviderConfig{
			{
				Name:      "openai",
				Enabled:   true,
				Model:     "gpt-5-codex",
				Endpoint:  "https://api.openai.com/v1",
				TimeoutMS: 8000,
				Retries:   1,
				Priority:  10,
				KeyRef:    "OPENAI_API_KEY",
			},
			{
				Name:      "deepseek",
				Enabled:   false,
				Model:     "deepseek-chat",
				Endpoint:  "https://api.deepseek.com",
				TimeoutMS: 8000,
				Retries:   1,
				Priority:  20,
				KeyRef:    "DEEPSEEK_API_KEY",
			},
			{
				Name:      "claude",
				Enabled:   false,
				Model:     "claude-3-5-sonnet-latest",
				Endpoint:  "https://api.anthropic.com",
				TimeoutMS: 8000,
				Retries:   1,
				Priority:  30,
				KeyRef:    "ANTHROPIC_API_KEY",
			},
			{
				Name:      "gemini",
				Enabled:   false,
				Model:     "gemini-2.0-flash",
				Endpoint:  "https://generativelanguage.googleapis.com",
				TimeoutMS: 8000,
				Retries:   1,
				Priority:  40,
				KeyRef:    "GEMINI_API_KEY",
			},
		},
	}
}

func (g *assistantModelGateway) snapshot() assistantModelConfig {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return cloneAssistantModelConfig(g.config)
}

func (g *assistantModelGateway) listProviderStatus() ([]assistantModelProviderConfig, []assistantProviderStatus) {
	cfg := g.snapshot()
	providers := cloneAssistantProviderSlice(cfg.Providers)
	statuses := make([]assistantProviderStatus, 0, len(providers))
	for _, provider := range providers {
		status := assistantProviderStatus{Name: provider.Name}
		switch {
		case !provider.Enabled:
			status.Healthy = "disabled"
			status.HealthReason = "provider_disabled"
		case g.adapters[strings.ToLower(strings.TrimSpace(provider.Name))] == nil:
			status.Healthy = "unavailable"
			status.HealthReason = "provider_adapter_missing"
		case assistantEndpointInvalidForRuntime(provider.Endpoint):
			status.Healthy = "unavailable"
			status.HealthReason = "endpoint_invalid"
		case assistantProviderRequiresSecret(provider) && strings.TrimSpace(os.Getenv(strings.TrimSpace(provider.KeyRef))) == "":
			status.Healthy = "unavailable"
			status.HealthReason = "secret_missing"
		case assistantProviderRequiresOpenAIKey(provider) && strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) == "":
			status.Healthy = "unavailable"
			status.HealthReason = "openai_key_missing"
		default:
			status.Healthy = "unavailable"
			status.HealthReason = "probe_failed"
			if probeErr := g.probeProviderStatus(provider); probeErr == nil {
				status.Healthy = "healthy"
				status.HealthReason = ""
			} else {
				status.Healthy, status.HealthReason = assistantProviderHealthFromProbeErr(probeErr)
			}
		}
		statuses = append(statuses, status)
	}
	return providers, statuses
}

func (g *assistantModelGateway) probeProviderStatus(provider assistantModelProviderConfig) error {
	adapter := g.adapters[strings.ToLower(strings.TrimSpace(provider.Name))]
	if adapter == nil {
		return errAssistantModelProbeUnsupported
	}
	prober, ok := adapter.(assistantProviderHealthProber)
	if !ok {
		return errAssistantModelProbeUnsupported
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(assistantProbeTimeoutMS(provider.TimeoutMS))*time.Millisecond)
	defer cancel()
	return prober.Probe(ctx, provider)
}

func assistantProviderHealthFromProbeErr(err error) (string, string) {
	switch {
	case err == nil:
		return "healthy", ""
	case errors.Is(err, errAssistantModelRateLimited):
		return "degraded", "rate_limited"
	case errors.Is(err, errAssistantModelTimeout):
		return "degraded", "probe_timeout"
	case errors.Is(err, errAssistantModelSecretMissing):
		return "unavailable", "secret_missing"
	case errors.Is(err, errAssistantModelConfigInvalid):
		return "unavailable", "endpoint_invalid"
	case errors.Is(err, errAssistantModelProbeUnsupported):
		return "unavailable", "probe_unsupported"
	default:
		return "unavailable", "probe_failed"
	}
}

func (g *assistantModelGateway) validateConfig(config assistantModelConfig) (assistantModelConfig, []string) {
	return normalizeAssistantModelConfig(config, true)
}

func (g *assistantModelGateway) applyConfig(config assistantModelConfig) (assistantModelConfig, []string) {
	normalized, errs := normalizeAssistantModelConfig(config, true)
	if len(errs) > 0 {
		return assistantModelConfig{}, errs
	}
	g.mu.Lock()
	g.config = normalized
	g.mu.Unlock()
	return normalized, nil
}

func (g *assistantModelGateway) listModels() []assistantModelProviderConfig {
	cfg := g.snapshot()
	providers := cloneAssistantProviderSlice(cfg.Providers)
	models := make([]assistantModelProviderConfig, 0, len(providers))
	for _, provider := range providers {
		if !provider.Enabled {
			continue
		}
		models = append(models, provider)
	}
	return models
}

func (g *assistantModelGateway) ResolveIntent(ctx context.Context, req assistantResolveIntentRequest) (assistantResolveIntentResult, error) {
	cfg := g.snapshot()
	providers := cloneAssistantProviderSlice(cfg.Providers)
	sort.SliceStable(providers, func(i, j int) bool {
		if providers[i].Priority == providers[j].Priority {
			return providers[i].Name < providers[j].Name
		}
		return providers[i].Priority < providers[j].Priority
	})

	lastTransientErr := error(nil)
	enabledCount := 0
	for _, provider := range providers {
		if !provider.Enabled {
			continue
		}
		enabledCount++
		if _, ok := assistantProviderNameAllowlist[strings.ToLower(strings.TrimSpace(provider.Name))]; !ok {
			return assistantResolveIntentResult{}, errAssistantModelConfigInvalid
		}
		if strings.TrimSpace(provider.Model) == "" || strings.TrimSpace(provider.Endpoint) == "" || provider.TimeoutMS <= 0 {
			return assistantResolveIntentResult{}, errAssistantModelConfigInvalid
		}
		if strings.TrimSpace(provider.KeyRef) == "" {
			return assistantResolveIntentResult{}, errAssistantModelConfigInvalid
		}
		if assistantEndpointInvalidForRuntime(provider.Endpoint) {
			return assistantResolveIntentResult{}, errAssistantModelConfigInvalid
		}
		if assistantProviderRequiresSecret(provider) && strings.TrimSpace(os.Getenv(strings.TrimSpace(provider.KeyRef))) == "" {
			return assistantResolveIntentResult{}, errAssistantModelSecretMissing
		}
		if assistantProviderRequiresOpenAIKey(provider) && strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) == "" {
			return assistantResolveIntentResult{}, errAssistantModelSecretMissing
		}
		adapter := g.adapters[strings.ToLower(strings.TrimSpace(provider.Name))]
		if adapter == nil {
			return assistantResolveIntentResult{}, errAssistantModelConfigInvalid
		}
		invokeErr := errAssistantModelProviderUnavailable
		attempts := provider.Retries + 1
		if attempts < 1 {
			attempts = 1
		}
		for attempt := 0; attempt < attempts; attempt++ {
			raw, err := adapter.Invoke(ctx, req.Prompt, provider)
			if err != nil {
				invokeErr = err
				if errorsIsAny(err, errAssistantModelTimeout, errAssistantModelRateLimited, errAssistantModelProviderUnavailable) && attempt < attempts-1 {
					continue
				}
				break
			}
			payload, err := assistantStrictDecodeSemanticIntent(raw)
			if err != nil {
				invokeErr = errAssistantPlanSchemaConstrainedDecodeFailed
				if attempt < attempts-1 {
					continue
				}
				break
			}
			resolved := assistantResolveIntentResult{
				Proposal:            payload.proposal(),
				SemanticState:       payload.proposal().semanticState(),
				ProviderName:        strings.ToLower(strings.TrimSpace(provider.Name)),
				ModelName:           strings.TrimSpace(provider.Model),
				ModelRevision:       assistantModelRevision(provider),
				GoalSummary:         strings.TrimSpace(payload.GoalSummary),
				UserVisibleReply:    strings.TrimSpace(payload.UserVisibleReply),
				NextQuestion:        strings.TrimSpace(payload.NextQuestion),
				Readiness:           strings.TrimSpace(payload.Readiness),
				SelectedCandidateID: strings.TrimSpace(payload.SelectedCandidateID),
			}
			assistantSyncResolvedSemanticResult(&resolved)
			return resolved, nil
		}
		switch {
		case errorsIsAny(invokeErr, errAssistantModelTimeout, errAssistantModelRateLimited, errAssistantModelProviderUnavailable, errAssistantPlanSchemaConstrainedDecodeFailed):
			lastTransientErr = invokeErr
			continue
		default:
			return assistantResolveIntentResult{}, invokeErr
		}
	}
	if enabledCount == 0 || lastTransientErr == nil {
		return assistantResolveIntentResult{}, errAssistantModelProviderUnavailable
	}
	return assistantResolveIntentResult{}, lastTransientErr
}

func assistantStrictDecodeIntent(raw []byte) (assistantIntentSpec, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var intent assistantIntentSpec
	if err := decoder.Decode(&intent); err != nil {
		return assistantIntentSpec{}, err
	}
	if err := decoder.Decode(&struct{}{}); err == nil {
		return assistantIntentSpec{}, errAssistantPlanSchemaConstrainedDecodeFailed
	}
	return intent, nil
}

func assistantStrictDecodeSemanticIntent(raw []byte) (assistantSemanticIntentPayload, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var payload assistantSemanticIntentPayload
	if err := decoder.Decode(&payload); err != nil {
		return assistantSemanticIntentPayload{}, err
	}
	if err := decoder.Decode(&struct{}{}); err == nil {
		return assistantSemanticIntentPayload{}, errAssistantPlanSchemaConstrainedDecodeFailed
	}
	return payload, nil
}

func assistantModelRevision(provider assistantModelProviderConfig) string {
	seed := strings.TrimSpace(provider.Name) + "|" + strings.TrimSpace(provider.Model) + "|" + strings.TrimSpace(provider.Endpoint)
	h := sha256.Sum256([]byte(seed))
	return "r" + hex.EncodeToString(h[:6])
}

func normalizeAssistantModelConfig(config assistantModelConfig, checkSecret bool) (assistantModelConfig, []string) {
	normalized := cloneAssistantModelConfig(config)
	normalized.ProviderRouting.Strategy = strings.TrimSpace(strings.ToLower(normalized.ProviderRouting.Strategy))
	if normalized.ProviderRouting.Strategy == "" {
		normalized.ProviderRouting.Strategy = "priority_failover"
	}
	providers := cloneAssistantProviderSlice(normalized.Providers)
	sort.SliceStable(providers, func(i, j int) bool {
		if providers[i].Priority == providers[j].Priority {
			return providers[i].Name < providers[j].Name
		}
		return providers[i].Priority < providers[j].Priority
	})
	normalized.Providers = providers

	errs := make([]string, 0)
	if normalized.ProviderRouting.Strategy != "priority_failover" {
		errs = append(errs, "provider_routing.strategy must be priority_failover")
	}
	seenPriority := map[int]struct{}{}
	for idx := range normalized.Providers {
		provider := &normalized.Providers[idx]
		provider.Name = strings.TrimSpace(strings.ToLower(provider.Name))
		provider.Model = strings.TrimSpace(provider.Model)
		provider.Endpoint = strings.TrimSpace(provider.Endpoint)
		provider.KeyRef = strings.TrimSpace(provider.KeyRef)
		if _, ok := assistantProviderNameAllowlist[provider.Name]; !ok {
			errs = append(errs, "providers."+strconv.Itoa(idx)+".name is invalid")
		}
		if !provider.Enabled {
			continue
		}
		if provider.Model == "" || provider.Endpoint == "" || provider.KeyRef == "" {
			errs = append(errs, "providers."+strconv.Itoa(idx)+" missing required fields")
		}
		if provider.TimeoutMS <= 0 {
			errs = append(errs, "providers."+strconv.Itoa(idx)+" timeout_ms must be > 0")
		}
		if provider.Retries < 0 {
			errs = append(errs, "providers."+strconv.Itoa(idx)+" retries must be >= 0")
		}
		if _, exists := seenPriority[provider.Priority]; exists {
			errs = append(errs, "provider priority duplicated")
		}
		seenPriority[provider.Priority] = struct{}{}
		if assistantEndpointInvalidForRuntime(provider.Endpoint) {
			errs = append(errs, "providers."+strconv.Itoa(idx)+" endpoint invalid for runtime")
		}
		if checkSecret && assistantProviderRequiresSecret(*provider) && provider.KeyRef != "" && strings.TrimSpace(os.Getenv(provider.KeyRef)) == "" {
			errs = append(errs, "providers."+strconv.Itoa(idx)+" secret missing for key_ref")
		}
		if checkSecret && assistantProviderRequiresOpenAIKey(*provider) && strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) == "" {
			errs = append(errs, "providers."+strconv.Itoa(idx)+" OPENAI_API_KEY missing")
		}
	}
	return normalized, errs
}

func assistantIsSimulateEndpoint(endpoint string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(endpoint)), "simulate://")
}

func assistantIsHTTPSAPIEndpoint(endpoint string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(endpoint)), "https://")
}

func assistantEndpointInvalidForRuntime(endpoint string) bool {
	normalized := strings.TrimSpace(strings.ToLower(endpoint))
	if normalized == "" {
		return true
	}
	return !assistantIsHTTPSAPIEndpoint(normalized)
}

func assistantProviderRequiresSecret(provider assistantModelProviderConfig) bool {
	return assistantIsHTTPSAPIEndpoint(provider.Endpoint)
}

func assistantProviderRequiresOpenAIKey(provider assistantModelProviderConfig) bool {
	return strings.TrimSpace(strings.ToLower(provider.Name)) == "openai" && assistantIsHTTPSAPIEndpoint(provider.Endpoint)
}

func assistantProbeTimeoutMS(providerTimeoutMS int) int {
	timeoutMS := providerTimeoutMS
	if timeoutMS <= 0 {
		timeoutMS = 1500
	}
	if timeoutMS < 500 {
		timeoutMS = 500
	}
	if timeoutMS > 3000 {
		timeoutMS = 3000
	}
	return timeoutMS
}

func assistantBuildOpenAIModelsURL(endpoint string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return "", err
	}
	if strings.ToLower(base.Scheme) != "https" {
		return "", fmt.Errorf("openai endpoint must use https")
	}
	base.RawQuery = ""
	base.Fragment = ""
	cleanPath := strings.TrimSpace(base.Path)
	if cleanPath == "" {
		base.Path = "/models"
		return base.String(), nil
	}
	trimmed := strings.TrimSuffix(cleanPath, "/")
	if strings.HasSuffix(trimmed, "/chat/completions") {
		trimmed = strings.TrimSuffix(trimmed, "/chat/completions")
	}
	if trimmed == "" || trimmed == "." {
		base.Path = "/models"
		return base.String(), nil
	}
	base.Path = "/" + path.Join(strings.TrimPrefix(trimmed, "/"), "models")
	return base.String(), nil
}

func assistantBuildOpenAIChatCompletionURL(endpoint string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(base.Scheme) != "https" || strings.TrimSpace(base.Host) == "" {
		return "", fmt.Errorf("invalid endpoint")
	}
	trimmedPath := strings.TrimSpace(base.Path)
	if strings.HasSuffix(trimmedPath, "/chat/completions") {
		return base.String(), nil
	}
	base.Path = path.Join(trimmedPath, "/chat/completions")
	base.RawQuery = ""
	base.Fragment = ""
	return base.String(), nil
}

func assistantExtractOpenAIMessageContent(raw any) string {
	switch value := raw.(type) {
	case string:
		return value
	case []any:
		parts := make([]string, 0, len(value))
		for _, entry := range value {
			object, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			switch text := object["text"].(type) {
			case string:
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "")
	default:
		return ""
	}
}

func cloneAssistantProviderSlice(in []assistantModelProviderConfig) []assistantModelProviderConfig {
	if len(in) == 0 {
		return nil
	}
	out := make([]assistantModelProviderConfig, len(in))
	copy(out, in)
	return out
}

func cloneAssistantModelConfig(in assistantModelConfig) assistantModelConfig {
	out := in
	out.Providers = cloneAssistantProviderSlice(in.Providers)
	return out
}

func errorsIsAny(err error, targets ...error) bool {
	for _, target := range targets {
		if target != nil && err == target {
			return true
		}
	}
	return false
}

func assistantCanonicalHash(v any) string {
	payload, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func assistantSkillManifestDigest(skills []string) string {
	if len(skills) == 0 {
		return ""
	}
	copied := append([]string(nil), skills...)
	sort.Strings(copied)
	return assistantCanonicalHash(copied)
}
