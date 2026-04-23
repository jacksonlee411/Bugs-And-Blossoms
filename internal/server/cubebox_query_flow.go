package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox"
)

type cubeboxReadPlanProducer interface {
	ProduceReadPlan(ctx context.Context, input cubeboxReadPlanProductionInput) (cubeboxReadPlanProductionResult, error)
}

type cubeboxReadPlanProductionInput struct {
	TenantID       string
	PrincipalID    string
	ConversationID string
	Prompt         string
	KnowledgePacks []cubebox.KnowledgePack
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
	store          cubeboxConversationStore
	registry       *cubebox.ExecutionRegistry
	producer       cubeboxReadPlanProducer
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

type cubeboxQueryLifecycle struct {
	traceID      string
	providerID   string
	providerType string
	modelSlug    string
	runtime      string
	startedAt    time.Time
}

func newCubeboxQueryFlow(
	runtime *cubebox.Runtime,
	store cubeboxConversationStore,
	registry *cubebox.ExecutionRegistry,
	producer cubeboxReadPlanProducer,
	knowledgePackDirs []string,
) (*cubeboxQueryFlow, error) {
	if runtime == nil || store == nil || registry == nil || producer == nil || len(knowledgePackDirs) == 0 {
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

func (f *cubeboxQueryFlow) TryHandle(
	ctx context.Context,
	request cubebox.GatewayStreamRequest,
	sink cubebox.GatewayEventSink,
) bool {
	if f == nil || f.runtime == nil || f.store == nil || f.registry == nil || f.producer == nil || len(f.knowledgePacks) == 0 {
		return false
	}

	produced, err := f.producer.ProduceReadPlan(ctx, cubeboxReadPlanProductionInput{
		TenantID:       strings.TrimSpace(request.TenantID),
		PrincipalID:    strings.TrimSpace(request.PrincipalID),
		ConversationID: strings.TrimSpace(request.ConversationID),
		Prompt:         request.Prompt,
		KnowledgePacks: append([]cubebox.KnowledgePack(nil), f.knowledgePacks...),
	})
	if err != nil {
		return false
	}
	if !produced.Handled {
		return false
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

	if err := cubebox.ValidateReadPlan(produced.Plan); err != nil {
		f.writeQueryTerminalError(ctx, request, prepared.turn.TurnID, &prepared.sequence, prepared.lifecycle, sink, queryPlanErrorToTerminal(err))
		return true
	}

	results, err := f.registry.ExecutePlan(ctx, cubebox.ExecuteRequest{
		TenantID:       request.TenantID,
		PrincipalID:    request.PrincipalID,
		ConversationID: request.ConversationID,
	}, produced.Plan)
	if err != nil {
		f.writeQueryTerminalError(ctx, request, prepared.turn.TurnID, &prepared.sequence, prepared.lifecycle, sink, queryExecutionErrorToTerminal(err))
		return true
	}

	answer := buildCubeboxQueryAnswer(produced.Plan, results)
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
	messages = append(messages, cubebox.PromptItem{
		Role:    "user",
		Content: input.Prompt,
	})
	return messages
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
			TS:             f.now().Format(time.RFC3339),
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
				TS:             f.now().Format(time.RFC3339),
				Payload:        f.queryErrorPayload("event_log_write_failed", "会话事件落库失败，当前响应已终止。", false, lifecycle),
			})
			return false
		}
		return sink.Write(event)
	}
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
			TS:             f.now().Format(time.RFC3339),
			Payload:        f.queryErrorPayload(terminal.Code, terminal.Message, terminal.Retryable, lifecycle),
		},
		{
			EventID:        "evt_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
			ConversationID: request.ConversationID,
			TurnID:         turnIDPtr(turnID),
			Sequence:       *sequence + 1,
			Type:           "turn.completed",
			TS:             f.now().Format(time.RFC3339),
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
			TS:             f.now().Format(time.RFC3339),
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
		startedAt = f.now()
	}
	latency := f.now().Sub(startedAt).Milliseconds()
	if latency < 0 {
		return 0
	}
	return latency
}

type queryTerminalError struct {
	Code      string
	Message   string
	Retryable bool
}

func buildDefaultCubeboxQueryFlow(
	runtime *cubebox.Runtime,
	store cubeboxConversationStore,
	orgStore OrgUnitStore,
	producer cubeboxReadPlanProducer,
) (*cubeboxQueryFlow, error) {
	if runtime == nil || store == nil || orgStore == nil || producer == nil {
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
		startedAt:    f.now(),
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
		return queryTerminalError{Code: "ai_plan_boundary_violation", Message: "查询计划引用了未登记的只读能力，请调整问题后重试。", Retryable: false}
	default:
		return queryTerminalError{Code: "cubebox_turn_stream_failed", Message: "查询执行失败，请稍后重试。", Retryable: false}
	}
}

func buildCubeboxQueryAnswer(plan cubebox.ReadPlan, results []cubebox.ExecuteResult) string {
	lines := []string{"已完成只读查询。"}
	if len(plan.ExplainFocus) > 0 {
		lines = append(lines, "本次关注："+strings.Join(plan.ExplainFocus, "、")+"。")
	}
	for _, result := range results {
		lines = append(lines, fmt.Sprintf("%s（%s）", strings.TrimSpace(result.StepID), strings.TrimSpace(result.APIKey)))
		summaries := summarizeQueryResult(result)
		if len(summaries) == 0 {
			lines = append(lines, "未返回可解释的重点字段。")
			continue
		}
		lines = append(lines, summaries...)
	}
	return strings.Join(lines, "\n")
}

func summarizeQueryResult(result cubebox.ExecuteResult) []string {
	out := make([]string, 0, len(result.ResultFocus))
	for _, focus := range result.ResultFocus {
		values := collectQueryFocusValues(result.Payload, splitQueryFocus(focus))
		if len(values) == 0 {
			continue
		}
		out = append(out, fmt.Sprintf("%s：%s", strings.TrimSpace(focus), strings.Join(limitQuerySummaryValues(values), "，")))
	}
	if len(out) > 0 {
		return out
	}
	raw, err := json.Marshal(result.Payload)
	if err != nil {
		return nil
	}
	return []string{string(raw)}
}

func splitQueryFocus(focus string) []string {
	parts := strings.Split(strings.TrimSpace(focus), ".")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func collectQueryFocusValues(value any, path []string) []string {
	if len(path) == 0 {
		if rendered, ok := renderQuerySummaryValue(value); ok {
			return []string{rendered}
		}
		return nil
	}

	token := path[0]
	if strings.HasSuffix(token, "[]") {
		key := strings.TrimSuffix(token, "[]")
		obj, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		next, ok := obj[key]
		if !ok || next == nil {
			return nil
		}
		items, ok := next.([]any)
		if !ok {
			return nil
		}
		out := make([]string, 0, len(items))
		for _, item := range items {
			out = append(out, collectQueryFocusValues(item, path[1:])...)
		}
		return uniqueQuerySummaryValues(out)
	}

	obj, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	next, ok := obj[token]
	if !ok || next == nil {
		return nil
	}
	return collectQueryFocusValues(next, path[1:])
}

func renderQuerySummaryValue(value any) (string, bool) {
	switch v := value.(type) {
	case string:
		v = strings.TrimSpace(v)
		return v, v != ""
	case bool:
		if v {
			return "true", true
		}
		return "false", true
	case int:
		return fmt.Sprintf("%d", v), true
	case int32:
		return fmt.Sprintf("%d", v), true
	case int64:
		return fmt.Sprintf("%d", v), true
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.6f", v), "0"), "."), true
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return "", false
		}
		text := strings.TrimSpace(string(raw))
		return text, text != ""
	}
}

func uniqueQuerySummaryValues(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func limitQuerySummaryValues(values []string) []string {
	values = uniqueQuerySummaryValues(values)
	if len(values) <= 5 {
		return values
	}
	return append(values[:5], "…")
}

func turnIDPtr(v string) *string {
	value := v
	return &value
}
