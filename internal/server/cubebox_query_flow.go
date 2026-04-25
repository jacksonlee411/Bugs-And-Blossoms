package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox"
)

type cubeboxReadPlanProducer interface {
	ProduceReadPlan(ctx context.Context, input cubeboxReadPlanProductionInput) (cubeboxReadPlanProductionResult, error)
}

type cubeboxQueryNarrator interface {
	NarrateQueryResult(ctx context.Context, input cubeboxQueryNarrationInput) (string, error)
}

type cubeboxReadPlanProductionInput struct {
	TenantID       string
	PrincipalID    string
	ConversationID string
	Prompt         string
	KnowledgePacks []cubebox.KnowledgePack
	RecentEntity   *cubebox.QueryEntity
}

type cubeboxReadPlanProductionResult struct {
	Handled      bool
	Plan         cubebox.ReadPlan
	ProviderID   string
	ProviderType string
	ModelSlug    string
}

type cubeboxQueryFlow struct {
	runtime        *cubebox.Runtime
	store          cubeboxTurnStore
	registry       *cubebox.ExecutionRegistry
	producer       cubeboxReadPlanProducer
	narrator       cubeboxQueryNarrator
	knowledgePacks []cubebox.KnowledgePack
	now            func() time.Time
}

type cubeboxPreparedQueryTurn struct {
	turn      cubebox.DeterministicTurn
	lifecycle cubeboxQueryLifecycle
	sequence  int
}

type cubeboxProviderReadPlanProducer struct {
	configReader   cubebox.RuntimeConfigReader
	adapter        cubebox.ProviderAdapter
	secretResolver cubebox.SecretResolver
	now            func() time.Time
}

type cubeboxProviderQueryNarrator struct {
	configReader   cubebox.RuntimeConfigReader
	adapter        cubebox.ProviderAdapter
	secretResolver cubebox.SecretResolver
}

type cubeboxQueryNarrationInput struct {
	TenantID             string
	PrincipalID          string
	ConversationID       string
	Prompt               string
	Plan                 cubebox.ReadPlan
	Results              []cubebox.ExecuteResult
	ExpectedProviderID   string
	ExpectedProviderType string
	ExpectedModelSlug    string
}

type cubeboxQueryLifecycle struct {
	traceID      string
	providerID   string
	providerType string
	modelSlug    string
	runtime      string
	startedAt    time.Time
}

type cubeboxQueryNarrationEnvelope struct {
	UserPrompt string                  `json:"user_prompt"`
	Plan       cubebox.ReadPlan        `json:"plan"`
	Results    []cubebox.ExecuteResult `json:"results"`
}

var errCubeboxQueryNarrationTargetMismatch = errors.New("cubebox query narration target mismatch")
var errCubeboxQueryNarrationContractViolation = errors.New("cubebox query narration contract violation")

var queryNarrationForbiddenPatterns = []*regexp.Regexp{
	regexp.MustCompile("```"),
	regexp.MustCompile(`(?m)^\s*\{`),
	regexp.MustCompile(`(?m)^\s*[\}\]]\s*$`),
	regexp.MustCompile(`(?i)\bstep-\d+\b`),
	regexp.MustCompile(`(?i)\b(api_key|result_focus|payload|results)\b`),
	regexp.MustCompile(`(?i)\b(plan|steps|params|depends_on|explain_focus|missing_params|clarifying_question)\s*["'：:=]`),
	regexp.MustCompile(`(?i)\b(org_code|parent_org_code|as_of|include_disabled|ext_fields)\s*["'：:=]`),
}

func newCubeboxQueryFlow(
	runtime *cubebox.Runtime,
	store cubeboxTurnStore,
	registry *cubebox.ExecutionRegistry,
	producer cubeboxReadPlanProducer,
	narrator cubeboxQueryNarrator,
	knowledgePackDirs []string,
) (*cubeboxQueryFlow, error) {
	if runtime == nil || store == nil || registry == nil || producer == nil || narrator == nil || len(knowledgePackDirs) == 0 {
		return nil, nil
	}
	packs := make([]cubebox.KnowledgePack, 0, len(knowledgePackDirs))
	for _, dir := range knowledgePackDirs {
		pack, err := cubebox.LoadKnowledgePack(strings.TrimSpace(dir))
		if err != nil {
			return nil, err
		}
		packs = append(packs, pack)
	}
	return &cubeboxQueryFlow{
		runtime:        runtime,
		store:          store,
		registry:       registry,
		producer:       producer,
		narrator:       narrator,
		knowledgePacks: packs,
		now:            func() time.Time { return time.Now().UTC() },
	}, nil
}

func newCubeboxProviderReadPlanProducer(
	configReader cubebox.RuntimeConfigReader,
	adapter cubebox.ProviderAdapter,
	secretResolver cubebox.SecretResolver,
) *cubeboxProviderReadPlanProducer {
	if configReader == nil || adapter == nil || secretResolver == nil {
		return nil
	}
	return &cubeboxProviderReadPlanProducer{
		configReader:   configReader,
		adapter:        adapter,
		secretResolver: secretResolver,
		now:            func() time.Time { return time.Now() },
	}
}

func newCubeboxProviderQueryNarrator(
	configReader cubebox.RuntimeConfigReader,
	adapter cubebox.ProviderAdapter,
	secretResolver cubebox.SecretResolver,
) *cubeboxProviderQueryNarrator {
	if configReader == nil || adapter == nil || secretResolver == nil {
		return nil
	}
	return &cubeboxProviderQueryNarrator{
		configReader:   configReader,
		adapter:        adapter,
		secretResolver: secretResolver,
	}
}

func (f *cubeboxQueryFlow) TryHandle(
	ctx context.Context,
	request cubebox.GatewayStreamRequest,
	sink cubebox.GatewayEventSink,
) bool {
	if f == nil || f.runtime == nil || f.store == nil || f.registry == nil || f.producer == nil || f.narrator == nil || len(f.knowledgePacks) == 0 {
		return false
	}

	queryContext, err := f.queryContext(ctx, request)
	if err != nil {
		return false
	}
	recentEntity := queryContext.RecentConfirmedEntity
	produced, err := f.producer.ProduceReadPlan(ctx, cubeboxReadPlanProductionInput{
		TenantID:       strings.TrimSpace(request.TenantID),
		PrincipalID:    strings.TrimSpace(request.PrincipalID),
		ConversationID: strings.TrimSpace(request.ConversationID),
		Prompt:         request.Prompt,
		KnowledgePacks: append([]cubebox.KnowledgePack(nil), f.knowledgePacks...),
		RecentEntity:   recentEntity,
	})
	if err != nil {
		return false
	}
	if !produced.Handled {
		return f.writeQueryNoQueryStopline(ctx, request, sink, produced)
	}

	prepared, err := f.prepareQueryTurn(ctx, request, produced)
	if err != nil {
		return false
	}
	defer f.runtime.FinishTurn(prepared.turn.TurnID)

	writeEvent := f.newQueryEventWriter(ctx, request, prepared.turn.TurnID, &prepared.sequence, prepared.lifecycle, sink)
	if !writeEvent("turn.started", f.queryStartedPayload(prepared.turn.UserMessageID, prepared.lifecycle)) {
		return true
	}
	if !writeEvent("turn.user_message.accepted", map[string]any{"message_id": prepared.turn.UserMessageID, "text": prepared.turn.Prompt}) {
		return true
	}

	if err := cubebox.ValidateReadPlan(produced.Plan); err != nil {
		f.writeQueryTerminalError(ctx, request, prepared.turn.TurnID, &prepared.sequence, prepared.lifecycle, sink, queryPlanErrorToTerminal(err))
		return true
	}

	if len(produced.Plan.MissingParams) > 0 {
		text := strings.TrimSpace(produced.Plan.ClarifyingQuestion)
		if text == "" {
			f.writeQueryTerminalError(ctx, request, prepared.turn.TurnID, &prepared.sequence, prepared.lifecycle, sink, queryTerminalError{
				Code:      "ai_plan_boundary_violation",
				Message:   "查询计划缺少必要澄清信息，请补充后重试。",
				Retryable: false,
			})
			return true
		}
		if !writeEvent("turn.agent_message.delta", map[string]any{"message_id": prepared.turn.AssistantMessageID, "delta": text}) {
			return true
		}
		_ = writeEvent("turn.agent_message.completed", map[string]any{"message_id": prepared.turn.AssistantMessageID})
		_ = writeEvent("turn.completed", f.queryCompletedPayload("completed", prepared.lifecycle))
		return true
	}

	results, err := f.registry.ExecutePlan(ctx, cubebox.ExecuteRequest{
		TenantID:       request.TenantID,
		PrincipalID:    request.PrincipalID,
		ConversationID: request.ConversationID,
	}, produced.Plan)
	if err != nil {
		if text := queryExecutionClarifyingQuestion(err); text != "" {
			if !writeEvent("turn.agent_message.delta", map[string]any{"message_id": prepared.turn.AssistantMessageID, "delta": text}) {
				return true
			}
			_ = writeEvent("turn.agent_message.completed", map[string]any{"message_id": prepared.turn.AssistantMessageID})
			_ = writeEvent("turn.completed", f.queryCompletedPayload("completed", prepared.lifecycle))
			return true
		}
		f.writeQueryTerminalError(ctx, request, prepared.turn.TurnID, &prepared.sequence, prepared.lifecycle, sink, queryExecutionErrorToTerminal(err))
		return true
	}

	answer, err := f.narrator.NarrateQueryResult(ctx, cubeboxQueryNarrationInput{
		TenantID:             request.TenantID,
		PrincipalID:          request.PrincipalID,
		ConversationID:       request.ConversationID,
		Prompt:               request.Prompt,
		Plan:                 produced.Plan,
		Results:              results,
		ExpectedProviderID:   produced.ProviderID,
		ExpectedProviderType: produced.ProviderType,
		ExpectedModelSlug:    produced.ModelSlug,
	})
	if err != nil {
		f.writeQueryTerminalError(ctx, request, prepared.turn.TurnID, &prepared.sequence, prepared.lifecycle, sink, queryNarrationErrorToTerminal(err))
		return true
	}
	if anchor := confirmedQueryEntity(results); anchor != nil {
		f.appendQueryMetadataEvent(ctx, request, prepared.turn.TurnID, &prepared.sequence, cubebox.QueryEntityConfirmedEventType, map[string]any{"entity": anchor.Payload()})
	}
	if !writeEvent("turn.agent_message.delta", map[string]any{"message_id": prepared.turn.AssistantMessageID, "delta": answer}) {
		return true
	}
	_ = writeEvent("turn.agent_message.completed", map[string]any{"message_id": prepared.turn.AssistantMessageID})
	_ = writeEvent("turn.completed", f.queryCompletedPayload("completed", prepared.lifecycle))
	return true
}

func (p *cubeboxProviderReadPlanProducer) ProduceReadPlan(
	ctx context.Context,
	input cubeboxReadPlanProductionInput,
) (cubeboxReadPlanProductionResult, error) {
	if p == nil || p.configReader == nil || p.adapter == nil || p.secretResolver == nil {
		return cubeboxReadPlanProductionResult{}, nil
	}

	config, err := p.configReader.GetActiveModelRuntimeConfig(ctx, strings.TrimSpace(input.TenantID))
	if err != nil {
		return cubeboxReadPlanProductionResult{}, err
	}
	if !config.Provider.Enabled {
		return cubeboxReadPlanProductionResult{}, cubebox.ErrProviderDisabled
	}
	modelSlug := strings.TrimSpace(config.Selection.ModelSlug)
	if modelSlug == "" {
		return cubeboxReadPlanProductionResult{}, cubebox.ErrModelSlugMissing
	}
	secret, err := p.secretResolver.ResolveSecretRef(ctx, strings.TrimSpace(input.TenantID), config.Provider.ID, config.Credential.SecretRef)
	if err != nil {
		return cubeboxReadPlanProductionResult{}, err
	}

	stream, err := p.adapter.StreamChatCompletion(ctx, cubebox.ProviderChatRequest{
		BaseURL:  strings.TrimSpace(config.Provider.BaseURL),
		APIKey:   secret,
		Model:    modelSlug,
		Messages: p.buildPlannerMessages(input),
		Input:    input.Prompt,
	})
	if err != nil {
		return cubeboxReadPlanProductionResult{}, err
	}
	defer func() { _ = stream.Close() }()

	var out strings.Builder
	for {
		chunk, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return cubeboxReadPlanProductionResult{}, err
		}
		out.WriteString(chunk.Delta)
		if chunk.Done {
			break
		}
	}
	raw := strings.TrimSpace(out.String())
	if raw == "NO_QUERY" {
		return cubeboxReadPlanProductionResult{
			Handled:      false,
			ProviderID:   strings.TrimSpace(config.Provider.ID),
			ProviderType: strings.TrimSpace(config.Provider.ProviderType),
			ModelSlug:    modelSlug,
		}, nil
	}
	plan, err := cubebox.DecodeReadPlan([]byte(raw))
	if err != nil {
		return cubeboxReadPlanProductionResult{}, err
	}
	return cubeboxReadPlanProductionResult{
		Handled:      true,
		Plan:         plan,
		ProviderID:   strings.TrimSpace(config.Provider.ID),
		ProviderType: strings.TrimSpace(config.Provider.ProviderType),
		ModelSlug:    modelSlug,
	}, nil
}

func (p *cubeboxProviderReadPlanProducer) buildPlannerMessages(input cubeboxReadPlanProductionInput) []cubebox.PromptItem {
	messages := []cubebox.PromptItem{
		{
			Role: "system",
			Content: strings.TrimSpace(fmt.Sprintf(`
你是 CubeBox 的只读查询计划器。
你的职责只有两种：
1. 如果用户请求可以由下面提供的模块知识包和已登记只读 API 完成，就输出一个严格合法的 ReadPlan JSON。
2. 如果用户请求不是这些知识包支持的查询场景，就只输出 NO_QUERY。

输出要求：
- 不要输出 Markdown 代码块
- 不要输出解释、前后缀或额外文本
- 只允许生成只读查询计划
- 缺少必要参数时，必须输出带 missing_params 与 clarifying_question 的 ReadPlan
- 可执行计划必须符合线性多步只读编排约束
- 如果用户说“今天/当前/现在”，可按当前自然日 %s 解释
`, p.now().Format("2006-01-02"))),
		},
	}
	for _, pack := range input.KnowledgePacks {
		messages = append(messages, cubebox.PromptItem{
			Role:    "system",
			Content: buildKnowledgePackPromptBlock(pack),
		})
	}
	if input.RecentEntity != nil {
		messages = append(messages, cubebox.PromptItem{
			Role:    "system",
			Content: buildRecentQueryEntityPromptBlock(*input.RecentEntity),
		})
	}
	messages = append(messages, cubebox.PromptItem{
		Role:    "user",
		Content: input.Prompt,
	})
	return messages
}

func buildRecentQueryEntityPromptBlock(entity cubebox.QueryEntity) string {
	body, err := json.Marshal(map[string]any{
		"recent_confirmed_query_entity": entity,
	})
	if err != nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf(`查询会话上下文：
%s

使用规则：
- 该块只提供上一轮已成功只读查询确认的结构化实体事实。
- 字段语义、继承规则与澄清策略以已加载知识包为准，通用 query flow 不解释具体业务字段。
- 当前轮用户显式输入优先；该上下文不是授权来源，也不是会话压缩摘要。`, string(body)))
}

func buildKnowledgePackPromptBlock(pack cubebox.KnowledgePack) string {
	parts := []string{
		fmt.Sprintf("知识包目录：%s", strings.TrimSpace(pack.Dir)),
		"[CUBEBOX-SKILL.md]\n" + strings.TrimSpace(pack.Files["CUBEBOX-SKILL.md"]),
		"[queries.md]\n" + strings.TrimSpace(pack.Files["queries.md"]),
		"[apis.md]\n" + strings.TrimSpace(pack.Files["apis.md"]),
		"[examples.md]\n" + strings.TrimSpace(pack.Files["examples.md"]),
	}
	return strings.Join(parts, "\n\n")
}

func (n *cubeboxProviderQueryNarrator) NarrateQueryResult(
	ctx context.Context,
	input cubeboxQueryNarrationInput,
) (string, error) {
	if n == nil || n.configReader == nil || n.adapter == nil || n.secretResolver == nil {
		return "", cubebox.ErrProviderConfigInvalid
	}
	config, err := n.configReader.GetActiveModelRuntimeConfig(ctx, strings.TrimSpace(input.TenantID))
	if err != nil {
		return "", err
	}
	if !config.Provider.Enabled {
		return "", cubebox.ErrProviderDisabled
	}
	providerID := strings.TrimSpace(config.Provider.ID)
	providerType := strings.TrimSpace(config.Provider.ProviderType)
	modelSlug := strings.TrimSpace(config.Selection.ModelSlug)
	if modelSlug == "" {
		return "", cubebox.ErrModelSlugMissing
	}
	if providerID != strings.TrimSpace(input.ExpectedProviderID) ||
		providerType != strings.TrimSpace(input.ExpectedProviderType) ||
		modelSlug != strings.TrimSpace(input.ExpectedModelSlug) {
		return "", errCubeboxQueryNarrationTargetMismatch
	}
	secret, err := n.secretResolver.ResolveSecretRef(ctx, strings.TrimSpace(input.TenantID), providerID, config.Credential.SecretRef)
	if err != nil {
		return "", err
	}
	envelope := cubeboxQueryNarrationEnvelope{
		UserPrompt: strings.TrimSpace(input.Prompt),
		Plan:       input.Plan,
		Results:    input.Results,
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		return "", err
	}
	stream, err := n.adapter.StreamChatCompletion(ctx, cubebox.ProviderChatRequest{
		BaseURL:  strings.TrimSpace(config.Provider.BaseURL),
		APIKey:   secret,
		Model:    modelSlug,
		Messages: buildQueryNarrationMessages(string(body)),
		Input:    string(body),
	})
	if err != nil {
		return "", err
	}
	defer func() { _ = stream.Close() }()

	var out strings.Builder
	for {
		chunk, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return "", err
		}
		out.WriteString(chunk.Delta)
		if chunk.Done {
			break
		}
	}
	text := strings.TrimSpace(out.String())
	if text == "" {
		return "", cubebox.ErrProviderStreamInvalid
	}
	if err := validateQueryNarrationText(text); err != nil {
		return "", err
	}
	return text, nil
}

func buildQueryNarrationMessages(body string) []cubebox.PromptItem {
	return []cubebox.PromptItem{
		{
			Role: "system",
			Content: strings.TrimSpace(`
你是 CubeBox 的查询结果叙述器。
你的职责只有一件事：基于已经执行完成的只读查询结果，向用户输出最终中文回答。

回答方式：
- 直接回答用户问题，先给结论，再补充最相关事实。
- 默认使用 1 到 3 句自然中文；只有结果本身是多个对象时，才允许用极短列表列出关键项。
- 把枚举、布尔和空值翻译成自然中文，例如 active=启用、disabled=停用、true=是、false=否、null/空字符串/空列表=未记录或没有。
- 对单个实体详情，优先用完整句子归纳；如用户需要对比或明细，也可以使用简短列表。
- 如果某些字段为空，只在和用户问题相关时用一句话说明“未记录……”，不要机械逐项写“空”。

硬约束：
- 只能依据输入 JSON 中的 user_prompt、plan、results 叙述。
- 不得编造任何 results 中不存在的字段、值、条数、层级或结论。
- 不得补做新的查询、推断新的默认值、追加新的澄清问题。
- 不得输出 Markdown 代码块。
- 不得逐字回显整份原始 JSON。
- 不得暴露实现细节或计划执行痕迹；不要出现“step-1”“api_key”“result_focus”“org_code”“parent_org_code”“as_of”“include_disabled”“ext_fields”“payload”“results”等内部字段或结构名。
- 若结果不足以支持更强结论，只能如实说明。
- 输出纯文本，直接作为用户可见最终回复。

示例：
- 好的回答：截至 2026-04-24，组织 100000 是“飞虫与鲜花”，当前为启用状态，属于业务单元。系统里暂未记录它的上级组织和负责人，也没有扩展字段。
- 不好的回答：{"results":[{"step_id":"step-1","payload":{"org_unit":{"org_code":"100000"}}}]}
`),
		},
		{
			Role:    "user",
			Content: body,
		},
	}
}

func validateQueryNarrationText(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return cubebox.ErrProviderStreamInvalid
	}
	for _, pattern := range queryNarrationForbiddenPatterns {
		if pattern.MatchString(text) {
			return errCubeboxQueryNarrationContractViolation
		}
	}
	return nil
}

func (f *cubeboxQueryFlow) newQueryEventWriter(
	ctx context.Context,
	request cubebox.GatewayStreamRequest,
	turnID string,
	sequence *int,
	lifecycle cubeboxQueryLifecycle,
	sink cubebox.GatewayEventSink,
) func(string, map[string]any) bool {
	return func(eventType string, payload map[string]any) bool {
		event := cubebox.CanonicalEvent{
			EventID:        "evt_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
			ConversationID: request.ConversationID,
			TurnID:         turnIDPtr(turnID),
			Sequence:       *sequence,
			Type:           eventType,
			TS:             f.clockNow().Format(time.RFC3339),
			Payload:        payload,
		}
		*sequence = *sequence + 1
		if err := f.store.AppendEvent(ctx, request.TenantID, request.PrincipalID, request.ConversationID, event); err != nil {
			sink.WriteFallback(cubebox.CanonicalEvent{
				EventID:        event.EventID,
				ConversationID: request.ConversationID,
				TurnID:         turnIDPtr(turnID),
				Sequence:       event.Sequence,
				Type:           "turn.error",
				TS:             f.clockNow().Format(time.RFC3339),
				Payload:        f.queryErrorPayload("event_log_write_failed", "会话事件落库失败，当前响应已终止。", false, lifecycle),
			})
			return false
		}
		return sink.Write(event)
	}
}

func (f *cubeboxQueryFlow) appendQueryMetadataEvent(
	ctx context.Context,
	request cubebox.GatewayStreamRequest,
	turnID string,
	sequence *int,
	eventType string,
	payload map[string]any,
) {
	if f == nil || f.store == nil || sequence == nil {
		return
	}
	event := cubebox.CanonicalEvent{
		EventID:        "evt_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		ConversationID: request.ConversationID,
		TurnID:         turnIDPtr(turnID),
		Sequence:       *sequence,
		Type:           strings.TrimSpace(eventType),
		TS:             f.clockNow().Format(time.RFC3339),
		Payload:        payload,
	}
	if event.Type == "" {
		return
	}
	if err := f.store.AppendEvent(ctx, request.TenantID, request.PrincipalID, request.ConversationID, event); err != nil {
		return
	}
	*sequence = *sequence + 1
}

func (f *cubeboxQueryFlow) writeQueryTerminalError(
	ctx context.Context,
	request cubebox.GatewayStreamRequest,
	turnID string,
	sequence *int,
	lifecycle cubeboxQueryLifecycle,
	sink cubebox.GatewayEventSink,
	terminal queryTerminalError,
) {
	events := []cubebox.CanonicalEvent{
		{
			EventID:        "evt_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
			ConversationID: request.ConversationID,
			TurnID:         turnIDPtr(turnID),
			Sequence:       *sequence,
			Type:           "turn.error",
			TS:             f.clockNow().Format(time.RFC3339),
			Payload:        f.queryErrorPayload(terminal.Code, terminal.Message, terminal.Retryable, lifecycle),
		},
		{
			EventID:        "evt_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
			ConversationID: request.ConversationID,
			TurnID:         turnIDPtr(turnID),
			Sequence:       *sequence + 1,
			Type:           "turn.completed",
			TS:             f.clockNow().Format(time.RFC3339),
			Payload:        f.queryCompletedPayload("failed", lifecycle),
		},
	}
	*sequence += 2
	if err := f.store.AppendEvents(ctx, request.TenantID, request.PrincipalID, request.ConversationID, events); err != nil {
		sink.WriteFallback(cubebox.CanonicalEvent{
			EventID:        "evt_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
			ConversationID: request.ConversationID,
			TurnID:         turnIDPtr(turnID),
			Sequence:       events[0].Sequence,
			Type:           "turn.error",
			TS:             f.clockNow().Format(time.RFC3339),
			Payload:        f.queryErrorPayload("event_log_write_failed", "会话事件落库失败，当前响应已终止。", false, lifecycle),
		})
		return
	}
	for _, event := range events {
		if !sink.Write(event) {
			return
		}
	}
}

func (f *cubeboxQueryFlow) queryStartedPayload(userMessageID string, lifecycle cubeboxQueryLifecycle) map[string]any {
	return map[string]any{
		"user_message_id": userMessageID,
		"trace_id":        lifecycle.traceID,
		"provider_id":     lifecycle.providerID,
		"provider_type":   lifecycle.providerType,
		"model_slug":      lifecycle.modelSlug,
		"runtime":         lifecycle.runtime,
	}
}

func (f *cubeboxQueryFlow) queryErrorPayload(code string, message string, retryable bool, lifecycle cubeboxQueryLifecycle) map[string]any {
	payload := f.queryLifecyclePayload(lifecycle)
	payload["code"] = code
	payload["message"] = message
	payload["retryable"] = retryable
	payload["latency_ms"] = f.queryLatencyMS(lifecycle)
	return payload
}

func (f *cubeboxQueryFlow) queryCompletedPayload(status string, lifecycle cubeboxQueryLifecycle) map[string]any {
	payload := f.queryLifecyclePayload(lifecycle)
	payload["status"] = status
	payload["latency_ms"] = f.queryLatencyMS(lifecycle)
	return payload
}

func (f *cubeboxQueryFlow) queryLifecyclePayload(lifecycle cubeboxQueryLifecycle) map[string]any {
	return map[string]any{
		"trace_id":      lifecycle.traceID,
		"provider_id":   lifecycle.providerID,
		"provider_type": lifecycle.providerType,
		"model_slug":    lifecycle.modelSlug,
		"runtime":       lifecycle.runtime,
	}
}

func (f *cubeboxQueryFlow) queryLatencyMS(lifecycle cubeboxQueryLifecycle) int64 {
	startedAt := lifecycle.startedAt
	if startedAt.IsZero() {
		startedAt = f.clockNow()
	}
	latency := f.clockNow().Sub(startedAt).Milliseconds()
	if latency < 0 {
		return 0
	}
	return latency
}

func (f *cubeboxQueryFlow) queryContext(ctx context.Context, request cubebox.GatewayStreamRequest) (cubebox.QueryContext, error) {
	if f == nil || f.store == nil {
		return cubebox.QueryContext{}, nil
	}
	replay, err := f.store.GetConversation(ctx, strings.TrimSpace(request.TenantID), strings.TrimSpace(request.PrincipalID), strings.TrimSpace(request.ConversationID))
	if err != nil {
		return cubebox.QueryContext{}, err
	}
	return cubebox.QueryContextFromEvents(replay.Events), nil
}

func confirmedQueryEntity(results []cubebox.ExecuteResult) *cubebox.QueryEntity {
	for i := len(results) - 1; i >= 0; i-- {
		if entity := normalizedQueryEntity(results[i].ConfirmedEntity); entity != nil {
			return entity
		}
	}
	return nil
}

func normalizedQueryEntity(entity *cubebox.QueryEntity) *cubebox.QueryEntity {
	if entity == nil {
		return nil
	}
	return cubebox.NormalizeQueryEntity(*entity)
}

func (f *cubeboxQueryFlow) writeQueryNoQueryStopline(
	ctx context.Context,
	request cubebox.GatewayStreamRequest,
	sink cubebox.GatewayEventSink,
	produced cubeboxReadPlanProductionResult,
) bool {
	prepared, err := f.prepareQueryTurn(ctx, request, produced)
	if err != nil {
		return false
	}
	defer f.runtime.FinishTurn(prepared.turn.TurnID)
	writeEvent := f.newQueryEventWriter(ctx, request, prepared.turn.TurnID, &prepared.sequence, prepared.lifecycle, sink)
	if !writeEvent("turn.started", f.queryStartedPayload(prepared.turn.UserMessageID, prepared.lifecycle)) {
		return true
	}
	if !writeEvent("turn.user_message.accepted", map[string]any{"message_id": prepared.turn.UserMessageID, "text": prepared.turn.Prompt}) {
		return true
	}
	if !writeEvent("turn.agent_message.delta", map[string]any{"message_id": prepared.turn.AssistantMessageID, "delta": queryNoQueryStoplineMessage()}) {
		return true
	}
	_ = writeEvent("turn.agent_message.completed", map[string]any{"message_id": prepared.turn.AssistantMessageID})
	_ = writeEvent("turn.completed", f.queryCompletedPayload("completed", prepared.lifecycle))
	return true
}

func queryNoQueryStoplineMessage() string {
	return "当前请求未形成可安全执行的只读查询计划。请换成明确的数据查询问题，或补充查询对象、条件和日期后重试。"
}

type queryTerminalError struct {
	Code      string
	Message   string
	Retryable bool
}

func buildDefaultCubeboxQueryFlow(
	runtime *cubebox.Runtime,
	store cubeboxTurnStore,
	orgStore OrgUnitStore,
	producer cubeboxReadPlanProducer,
	narrator cubeboxQueryNarrator,
) (*cubeboxQueryFlow, error) {
	if runtime == nil || store == nil || orgStore == nil || producer == nil || narrator == nil {
		return nil, nil
	}
	items, err := newCubeBoxOrgUnitRegisteredExecutors(orgStore)
	if err != nil {
		return nil, err
	}
	registry, err := cubebox.NewExecutionRegistry(items...)
	if err != nil {
		return nil, err
	}
	return newCubeboxQueryFlow(
		runtime,
		store,
		registry,
		producer,
		narrator,
		[]string{mustResolveRepoPath(filepath.Join("modules", "orgunit", "presentation", "cubebox"))},
	)
}

func (f *cubeboxQueryFlow) prepareQueryTurn(
	ctx context.Context,
	request cubebox.GatewayStreamRequest,
	produced cubeboxReadPlanProductionResult,
) (cubeboxPreparedQueryTurn, error) {
	turn := f.runtime.StartTurn(cubebox.TurnOwner{
		TenantID:       request.TenantID,
		PrincipalID:    request.PrincipalID,
		ConversationID: request.ConversationID,
	}, request.Prompt)
	lifecycle := cubeboxQueryLifecycle{
		traceID:      "trace_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		providerID:   strings.TrimSpace(produced.ProviderID),
		providerType: strings.TrimSpace(produced.ProviderType),
		modelSlug:    strings.TrimSpace(produced.ModelSlug),
		runtime:      "cubebox-query-read-plan",
		startedAt:    f.clockNow(),
	}
	canonicalContext := f.buildQueryCanonicalContext(request, lifecycle)
	prepared, err := cubebox.PrepareTurnStream(ctx, f.store, request, canonicalContext)
	if err != nil {
		f.runtime.FinishTurn(turn.TurnID)
		return cubeboxPreparedQueryTurn{}, err
	}
	return cubeboxPreparedQueryTurn{
		turn:      turn,
		lifecycle: lifecycle,
		sequence:  prepared.Sequence,
	}, nil
}

func (f *cubeboxQueryFlow) clockNow() time.Time {
	if f != nil && f.now != nil {
		return f.now()
	}
	return time.Now().UTC()
}

func (f *cubeboxQueryFlow) buildQueryCanonicalContext(request cubebox.GatewayStreamRequest, lifecycle cubeboxQueryLifecycle) cubebox.CanonicalContext {
	return cubebox.CanonicalContext{
		TenantID:       request.TenantID,
		PrincipalID:    request.PrincipalID,
		Language:       "zh",
		Page:           "/app/cubebox",
		Permissions:    []string{"cubebox.conversations:use"},
		BusinessObject: "conversation",
		ProviderID:     lifecycle.providerID,
		ProviderType:   lifecycle.providerType,
		ModelSlug:      lifecycle.modelSlug,
		Runtime:        lifecycle.runtime,
	}
}

func mustResolveRepoPath(rel string) string {
	rel = strings.TrimSpace(rel)
	if rel == "" || filepath.IsAbs(rel) {
		return rel
	}
	if root, ok := findRepoRootFromCaller(); ok {
		return filepath.Join(root, rel)
	}
	if cwd, err := os.Getwd(); err == nil {
		return filepath.Join(cwd, rel)
	}
	return rel
}

func findRepoRootFromCaller() (string, bool) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", false
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func queryPlanErrorToTerminal(err error) queryTerminalError {
	if errors.Is(err, cubebox.ErrReadPlanSchemaConstrainedDecodeFailed) {
		return queryTerminalError{Code: "ai_plan_schema_constrained_decode_failed", Message: "查询计划解析失败，请补全信息后重试。", Retryable: false}
	}
	return queryTerminalError{Code: "ai_plan_boundary_violation", Message: "查询计划超出允许范围，请调整问题后重试。", Retryable: false}
}

func queryExecutionErrorToTerminal(err error) queryTerminalError {
	switch {
	case errors.Is(err, errOrgUnitNotFound):
		return queryTerminalError{Code: "orgunit_not_found", Message: "未找到符合条件的组织，请调整关键词或提供组织编码。", Retryable: false}
	case isBadRequestError(err), errors.Is(err, cubebox.ErrReadPlanBoundaryViolation):
		return queryTerminalError{Code: "invalid_request", Message: "查询参数无效，请检查后重试。", Retryable: false}
	case errors.Is(err, cubebox.ErrAPICatalogDriftOrExecutorMissing):
		return queryTerminalError{Code: "api_catalog_drift_or_executor_missing", Message: "查询执行目录与系统注册表不一致，请稍后重试或联系管理员。", Retryable: false}
	default:
		return queryTerminalError{Code: "cubebox_turn_stream_failed", Message: "查询执行失败，请稍后重试。", Retryable: false}
	}
}

func queryExecutionClarifyingQuestion(err error) string {
	var ambiguous *orgUnitSearchAmbiguousError
	if errors.As(err, &ambiguous) {
		return strings.TrimSpace(ambiguous.ClarifyingQuestion())
	}
	return ""
}

func queryNarrationErrorToTerminal(err error) queryTerminalError {
	switch {
	case errors.Is(err, errCubeboxQueryNarrationTargetMismatch):
		return queryTerminalError{Code: "ai_reply_model_target_mismatch", Message: "查询结果叙述未命中预期的大模型链路，请稍后重试。", Retryable: false}
	case errors.Is(err, errCubeboxQueryNarrationContractViolation):
		return queryTerminalError{Code: "ai_reply_render_failed", Message: "查询结果叙述未通过输出约束校验，请稍后重试。", Retryable: true}
	case errors.Is(err, cubebox.ErrProviderDisabled), errors.Is(err, cubebox.ErrModelSlugMissing), errors.Is(err, cubebox.ErrProviderConfigInvalid):
		return queryTerminalError{Code: "ai_model_config_invalid", Message: "模型配置无效，请联系管理员检查。", Retryable: false}
	case errors.Is(err, cubebox.ErrSecretMissing), errors.Is(err, cubebox.ErrSecretRefInvalid):
		return queryTerminalError{Code: "ai_model_secret_missing", Message: "当前模型密钥不可用，请联系管理员检查。", Retryable: false}
	case errors.Is(err, cubebox.ErrProviderUnauthorized):
		return queryTerminalError{Code: "ai_model_provider_unavailable", Message: "当前模型认证失败，请联系管理员检查。", Retryable: false}
	case errors.Is(err, cubebox.ErrProviderRateLimited):
		return queryTerminalError{Code: "ai_reply_render_failed", Message: "查询结果叙述失败，请稍后重试。", Retryable: true}
	case errors.Is(err, cubebox.ErrProviderUnavailable), errors.Is(err, cubebox.ErrProviderTimeout), errors.Is(err, cubebox.ErrProviderStreamInvalid):
		return queryTerminalError{Code: "ai_reply_render_failed", Message: "查询结果叙述失败，请稍后重试。", Retryable: true}
	default:
		return queryTerminalError{Code: "ai_reply_render_failed", Message: "查询结果叙述失败，请稍后重试。", Retryable: false}
	}
}

func turnIDPtr(v string) *string {
	value := v
	return &value
}
