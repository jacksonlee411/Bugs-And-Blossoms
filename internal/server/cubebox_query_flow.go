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

type cubeboxQueryClarifier interface {
	ClarifyQuery(ctx context.Context, input cubeboxQueryClarificationInput) (string, error)
}

type cubeboxNoQueryGuidanceNarrator interface {
	NarrateNoQueryGuidance(ctx context.Context, input cubeboxNoQueryGuidanceInput) (string, error)
}

type cubeboxReadPlanProductionInput struct {
	TenantID       string
	PrincipalID    string
	ConversationID string
	Prompt         string
	KnowledgePacks []cubebox.KnowledgePack
	QueryContext   cubebox.QueryContext
	ReadAPICatalog []cubebox.ReadAPICatalogEntry
	WorkingResults *cubebox.QueryWorkingResults
	Corrections    []string
}

type cubeboxReadPlanProductionResult struct {
	Handled         bool
	Outcome         cubebox.PlannerOutcome
	Plan            cubebox.ReadPlan
	ProviderID      string
	ProviderType    string
	ModelSlug       string
	ExplicitOutcome bool
}

type cubeboxQueryFlow struct {
	runtime        *cubebox.Runtime
	store          cubeboxTurnStore
	registry       *cubebox.ExecutionRegistry
	producer       cubeboxReadPlanProducer
	narrator       cubeboxQueryNarrator
	clarifier      cubeboxQueryClarifier
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

type cubeboxQueryClarificationInput struct {
	TenantID             string
	Prompt               string
	QueryContext         cubebox.QueryContext
	Candidates           []cubebox.QueryCandidate
	CandidateGroupID     string
	ErrorCode            string
	CandidateSource      string
	CandidateCount       int
	CannotSilentSelect   bool
	ExpectedProviderID   string
	ExpectedProviderType string
	ExpectedModelSlug    string
}

type cubeboxQueryNarrationInput struct {
	TenantID             string
	PrincipalID          string
	ConversationID       string
	Prompt               string
	Results              []cubebox.QueryNarrationResult
	QueryContext         cubebox.QueryContext
	ExpectedProviderID   string
	ExpectedProviderType string
	ExpectedModelSlug    string
}

type cubeboxNoQueryGuidanceInput struct {
	TenantID             string
	Prompt               string
	ScopeSummary         string
	SuggestedPrompts     []string
	QueryContextHint     cubeboxNoQueryContextHint
	ExpectedProviderID   string
	ExpectedProviderType string
	ExpectedModelSlug    string
}

type cubeboxNoQueryContextHint struct {
	HasConversationEvidence bool `json:"has_conversation_evidence"`
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
	UserPrompt string                            `json:"user_prompt"`
	Results    []cubeboxQueryNarrationResultView `json:"results"`
}

type cubeboxQueryNarrationResultView struct {
	Domain string         `json:"domain,omitempty"`
	Data   map[string]any `json:"data,omitempty"`
}

type cubeboxQueryEvidenceWindowProjectionBudget struct {
	MaxConfirmedEntities  int
	MaxCandidateGroups    int
	MaxCandidatesPerGroup int
	MaxDialogueTurns      int
}

type cubeboxQueryClarificationEnvelope struct {
	UserPrompt          string                      `json:"user_prompt"`
	QueryEvidenceWindow cubebox.QueryEvidenceWindow `json:"query_evidence_window"`
	Candidates          []cubebox.QueryCandidate    `json:"candidates"`
	CandidateGroupID    string                      `json:"candidate_group_id,omitempty"`
	ErrorCode           string                      `json:"error_code,omitempty"`
	CandidateSource     string                      `json:"candidate_source,omitempty"`
	CandidateCount      int                         `json:"candidate_count,omitempty"`
	CannotSilentSelect  bool                        `json:"cannot_silent_select,omitempty"`
}

type cubeboxNoQueryGuidanceEnvelope struct {
	ScopeSummary     string                    `json:"scope_summary"`
	SuggestedPrompts []string                  `json:"suggested_prompts"`
	QueryContextHint cubeboxNoQueryContextHint `json:"query_context_hint"`
}

var errCubeboxQueryNarrationTargetMismatch = errors.New("cubebox query narration target mismatch")
var errCubeboxQueryNarrationContractViolation = errors.New("cubebox query narration contract violation")

const (
	queryContextMaxCanonicalCandidateGroups = 5

	queryContextMaxPlannerConfirmedEntities  = 5
	queryContextMaxPlannerCandidateGroups    = 5
	queryContextMaxPlannerCandidatesPerGroup = 100
	queryContextMaxPlannerDialogueTurns      = 5

	queryContextMaxClarifierConfirmedEntities  = 5
	queryContextMaxClarifierCandidateGroups    = 2
	queryContextMaxClarifierCandidatesPerGroup = 20
	queryContextMaxClarifierDialogueTurns      = 2

	queryContextMaxNarratorConfirmedEntities  = 5
	queryContextMaxNarratorCandidateGroups    = 1
	queryContextMaxNarratorCandidatesPerGroup = 5
	queryContextMaxNarratorDialogueTurns      = 2
)

var queryNarrationForbiddenPatterns = []*regexp.Regexp{
	regexp.MustCompile("```"),
	regexp.MustCompile(`(?m)^\s*\{`),
	regexp.MustCompile(`(?m)^\s*[\}\]]\s*$`),
	regexp.MustCompile(`(?i)\bstep-\d+\b`),
	regexp.MustCompile(`(?i)\b(api_key|result_focus|payload|results)\b`),
	regexp.MustCompile(`(?i)\b(plan|steps|params|depends_on|explain_focus|missing_params|clarifying_question)\s*["'：:=]`),
	regexp.MustCompile(`(?i)\b(plan|steps|params|depends_on|explain_focus|missing_params|clarifying_question)(\.[A-Za-z0-9_-]+|\[[0-9]+\])+`),
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
	var clarifier cubeboxQueryClarifier
	if value, ok := narrator.(cubeboxQueryClarifier); ok {
		clarifier = value
	}
	packs := make([]cubebox.KnowledgePack, 0, len(knowledgePackDirs))
	for _, dir := range knowledgePackDirs {
		pack, err := cubebox.LoadKnowledgePack(strings.TrimSpace(dir))
		if err != nil {
			return nil, err
		}
		if err := cubebox.ValidateKnowledgePackAgainstRegistry(pack, registry); err != nil {
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
		clarifier:      clarifier,
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

func (n *cubeboxProviderQueryNarrator) ClarifyQuery(ctx context.Context, input cubeboxQueryClarificationInput) (string, error) {
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
	body, err := json.Marshal(buildQueryClarificationEnvelope(input))
	if err != nil {
		return "", err
	}
	stream, err := n.adapter.StreamChatCompletion(ctx, cubebox.ProviderChatRequest{
		BaseURL:  strings.TrimSpace(config.Provider.BaseURL),
		APIKey:   secret,
		Model:    modelSlug,
		Messages: buildQueryClarificationMessages(string(body)),
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

func (n *cubeboxProviderQueryNarrator) NarrateNoQueryGuidance(ctx context.Context, input cubeboxNoQueryGuidanceInput) (string, error) {
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
	envelope := buildNoQueryGuidanceEnvelope(input)
	body, err := json.Marshal(envelope)
	if err != nil {
		return "", err
	}
	stream, err := n.adapter.StreamChatCompletion(ctx, cubebox.ProviderChatRequest{
		BaseURL:  strings.TrimSpace(config.Provider.BaseURL),
		APIKey:   secret,
		Model:    modelSlug,
		Messages: buildNoQueryGuidanceMessages(string(body)),
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
	if err := validateNoQueryGuidanceText(text, envelope); err != nil {
		return "", err
	}
	return text, nil
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
	readAPICatalog := f.registry.ReadAPICatalog()
	workingState := cubebox.NewQueryWorkingResultsState(request.Prompt, cubebox.DefaultQueryLoopBudget())
	workingState.NotePlanningRound()
	produced, err := f.produceReadPlan(ctx, request, queryContext, readAPICatalog, workingState)
	if err != nil {
		if isQueryPlannerContractError(err) {
			return f.writeQueryPlannerTerminalError(ctx, request, sink, cubeboxReadPlanProductionResult{}, queryPlanErrorToTerminal(err))
		}
		return false
	}
	outcome := produced.Outcome
	if outcome.Type == cubebox.PlannerOutcomeNoQuery {
		return f.writeQueryNoQueryStopline(ctx, request, sink, produced, queryContext)
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

	var finalResults []cubebox.ExecuteResult
	for {
		switch outcome.Type {
		case cubebox.PlannerOutcomeReadPlan:
			plan := outcome.Plan
			if err := cubebox.ValidateReadPlan(plan); err != nil {
				f.writeQueryTerminalError(ctx, request, prepared.turn.TurnID, &prepared.sequence, prepared.lifecycle, sink, queryPlanErrorToTerminal(err))
				return true
			}
			if exceeded, duplicated := noteRepeatedPlanSteps(workingState, plan); duplicated {
				if repeatedPlanCanCompleteAllOrgScopeCorrection(request.Prompt, plan, workingState.Snapshot()) {
					outcome = cubebox.PlannerOutcome{Type: cubebox.PlannerOutcomeDone}
					continue
				}
				if exceeded {
					f.writeQueryTerminalError(ctx, request, prepared.turn.TurnID, &prepared.sequence, prepared.lifecycle, sink, queryLoopRepeatedPlanTerminal())
					return true
				}
				if !workingState.CanPlan() {
					f.writeQueryTerminalError(ctx, request, prepared.turn.TurnID, &prepared.sequence, prepared.lifecycle, sink, queryLoopBudgetExceededTerminal())
					return true
				}
				workingState.NotePlanningRound()
				next, err := f.produceReadPlan(ctx, request, queryContext, readAPICatalog, workingState)
				if err != nil {
					f.writeQueryTerminalError(ctx, request, prepared.turn.TurnID, &prepared.sequence, prepared.lifecycle, sink, queryPlannerErrorToTerminal(err))
					return true
				}
				produced = next
				outcome = next.Outcome
				continue
			}
			if !workingState.CanExecute(plan) {
				f.writeQueryTerminalError(ctx, request, prepared.turn.TurnID, &prepared.sequence, prepared.lifecycle, sink, queryLoopBudgetExceededTerminal())
				return true
			}
			results, err := f.registry.ExecutePlan(ctx, cubebox.ExecuteRequest{
				TenantID:       request.TenantID,
				PrincipalID:    request.PrincipalID,
				ConversationID: request.ConversationID,
			}, plan)
			if err != nil {
				if f.writeQueryExecutionClarification(ctx, request, prepared, queryContext, produced, err, writeEvent) {
					return true
				}
				f.writeQueryTerminalError(ctx, request, prepared.turn.TurnID, &prepared.sequence, prepared.lifecycle, sink, queryExecutionErrorToTerminal(err))
				return true
			}
			workingState.AppendPlan(workingState.Snapshot().RoundIndex, plan, results)
			finalResults = results
			if !workingState.CanPlan() {
				f.writeQueryTerminalError(ctx, request, prepared.turn.TurnID, &prepared.sequence, prepared.lifecycle, sink, queryLoopBudgetExceededTerminal())
				return true
			}
			workingState.NotePlanningRound()
			next, err := f.produceReadPlan(ctx, request, queryContext, readAPICatalog, workingState)
			if err != nil {
				f.writeQueryTerminalError(ctx, request, prepared.turn.TurnID, &prepared.sequence, prepared.lifecycle, sink, queryPlannerErrorToTerminal(err))
				return true
			}
			produced = next
			outcome = next.Outcome
		case cubebox.PlannerOutcomeClarify:
			return f.writePlannerClarification(ctx, request, prepared, outcome, sink, writeEvent)
		case cubebox.PlannerOutcomeDone:
			if !workingState.HasExecution() || len(finalResults) == 0 {
				f.writeQueryTerminalError(ctx, request, prepared.turn.TurnID, &prepared.sequence, prepared.lifecycle, sink, queryDoneWithoutResultTerminal())
				return true
			}
			answer, err := f.narrator.NarrateQueryResult(ctx, cubeboxQueryNarrationInput{
				TenantID:             request.TenantID,
				PrincipalID:          request.PrincipalID,
				ConversationID:       request.ConversationID,
				Prompt:               request.Prompt,
				Results:              f.registry.ProjectNarrationResults(finalResults),
				QueryContext:         queryContext,
				ExpectedProviderID:   produced.ProviderID,
				ExpectedProviderType: produced.ProviderType,
				ExpectedModelSlug:    produced.ModelSlug,
			})
			if err != nil {
				f.writeQueryTerminalError(ctx, request, prepared.turn.TurnID, &prepared.sequence, prepared.lifecycle, sink, queryNarrationErrorToTerminal(err))
				return true
			}
			f.writeQueryResultMetadata(ctx, request, prepared.turn.TurnID, &prepared.sequence, finalResults)
			if !writeEvent("turn.agent_message.delta", map[string]any{"message_id": prepared.turn.AssistantMessageID, "delta": answer}) {
				return true
			}
			_ = writeEvent("turn.agent_message.completed", map[string]any{"message_id": prepared.turn.AssistantMessageID})
			_ = writeEvent("turn.completed", f.queryCompletedPayload("completed", prepared.lifecycle))
			return true
		case cubebox.PlannerOutcomeNoQuery:
			if workingState.HasExecution() {
				f.writeQueryTerminalError(ctx, request, prepared.turn.TurnID, &prepared.sequence, prepared.lifecycle, sink, queryNoQueryAfterExecutionTerminal())
				return true
			}
			answer := f.noQueryGuidanceText(ctx, request, queryContext, produced)
			if !writeEvent("turn.agent_message.delta", map[string]any{"message_id": prepared.turn.AssistantMessageID, "delta": answer}) {
				return true
			}
			_ = writeEvent("turn.agent_message.completed", map[string]any{"message_id": prepared.turn.AssistantMessageID})
			_ = writeEvent("turn.completed", f.queryCompletedPayload("completed", prepared.lifecycle))
			return true
		default:
			f.writeQueryTerminalError(ctx, request, prepared.turn.TurnID, &prepared.sequence, prepared.lifecycle, sink, queryPlanErrorToTerminal(cubebox.ErrPlannerOutcomeInvalid))
			return true
		}
	}
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
	outcome, err := cubebox.DecodePlannerOutcome([]byte(raw))
	if err != nil {
		return cubeboxReadPlanProductionResult{}, err
	}
	handled := outcome.Type != cubebox.PlannerOutcomeNoQuery
	return cubeboxReadPlanProductionResult{
		Handled:         handled,
		Outcome:         outcome,
		Plan:            outcome.Plan,
		ProviderID:      strings.TrimSpace(config.Provider.ID),
		ProviderType:    strings.TrimSpace(config.Provider.ProviderType),
		ModelSlug:       modelSlug,
		ExplicitOutcome: true,
	}, nil
}

func (p *cubeboxProviderReadPlanProducer) buildPlannerMessages(input cubeboxReadPlanProductionInput) []cubebox.PromptItem {
	currentDate := p.now()
	monthDayExample := time.Date(currentDate.Year(), currentDate.Month(), 9, 0, 0, 0, 0, currentDate.Location()).Format("2006-01-02")
	messages := []cubebox.PromptItem{
		{
			Role: "system",
			Content: strings.TrimSpace(fmt.Sprintf(`
你是 CubeBox 的只读查询计划器。
	你的职责只有两种：
	1. 如果用户请求可以由下面提供的模块知识包、已登记只读 API 和当前 turn 内 working_results 继续推进，就输出 planner outcome JSON。
	2. 如果用户请求不是这些知识包支持的查询场景，就输出 NO_QUERY outcome。

	输出要求：
	- 不要输出 Markdown 代码块
	- 不要输出解释、前后缀或额外文本
	- 只允许生成只读查询计划
	- 推荐输出 JSON envelope：{"outcome":"READ_PLAN","plan":{...}}、{"outcome":"CLARIFY","missing_params":[...],"clarifying_question":"..."}、{"outcome":"DONE"}、{"outcome":"NO_QUERY"}
	- 兼容裸 ReadPlan JSON 与裸 NO_QUERY；不要输出裸 DONE，DONE 必须使用 JSON envelope
	- READ_PLAN.plan 必须符合线性多步只读编排约束，且每次只规划当前最小必要查询
	- CLARIFY 表示缺少必要参数或无法稳定判断引用对象，必须给出 missing_params 与 clarifying_question
	- DONE 表示当前 working_results 已足够进入最终回答；不要用 NO_QUERY 表示“已经查够”
	- NO_QUERY 只表示用户请求超出当前查询域
		- working_results 是当前 turn 内临时 tool observation；不要把它写成长时记忆，也不要生成业务专用待查队列或 winner 状态
		- 不要重复执行 working_results.executed_fingerprints 中已经出现的查询 fingerprint；若重复提示已出现，必须选择新的 READ_PLAN、CLARIFY 或 DONE
		- 判断是否继续查询时，只能阅读 working_results.latest_observation/items/summary 与知识包规则；通用 query flow 不会替你解释业务字段
		- 如果用户说“今天/当前/现在”，可按当前自然日 %s 解释
		- 如果用户说“本月N日/这个月N号”，可按当前自然日所在年月解释为完整日期；例如当前自然日为 %s 时，“本月9日”是 %s
			- page/size 是分页执行控制，不是业务必填参数；缺省时默认 page=1,size=100，不要向用户追问 page=1,size=100
			- 用户只给一个正整数作为分页短答时，优先当作 size，page 默认 1
			- 用户可见 page 为 1 基页码；page=1 表示第一页
			- 当前用户输入优先于历史事实；当用户明确否定、纠正或扩大上一轮范围（例如“不只是/不是/不限于/不要只看 X，而是全部/所有”）时，不得继承历史 keyword、parent_org_code、单个 entity_key 或 result_list target set，必须按当前输入重新规划
				`, currentDate.Format("2006-01-02"), currentDate.Format("2006-01-02"), monthDayExample)),
		},
	}
	for _, pack := range input.KnowledgePacks {
		messages = append(messages, cubebox.PromptItem{
			Role:    "system",
			Content: buildKnowledgePackPromptBlock(pack),
		})
	}
	if block := buildReadAPICatalogPromptBlock(input.ReadAPICatalog); block != "" {
		messages = append(messages, cubebox.PromptItem{
			Role:    "system",
			Content: block,
		})
	}
	if block := buildQueryEvidenceWindowPromptBlock(input.QueryContext, input.Prompt); block != "" {
		messages = append(messages, cubebox.PromptItem{
			Role:    "system",
			Content: block,
		})
	}
	if input.WorkingResults != nil {
		if block := buildWorkingResultsPromptBlock(*input.WorkingResults); block != "" {
			messages = append(messages, cubebox.PromptItem{
				Role:    "system",
				Content: block,
			})
		}
	}
	for _, correction := range input.Corrections {
		correction = strings.TrimSpace(correction)
		if correction == "" {
			continue
		}
		messages = append(messages, cubebox.PromptItem{
			Role:    "system",
			Content: "planner correction: " + correction,
		})
	}
	messages = append(messages, cubebox.PromptItem{
		Role:    "user",
		Content: input.Prompt,
	})
	return messages
}

func buildQueryEvidenceWindowPromptBlock(queryContext cubebox.QueryContext, currentUserInput string) string {
	window := buildQueryEvidenceWindow(queryContext, currentUserInput, cubeboxQueryEvidenceWindowProjectionBudget{
		MaxConfirmedEntities:  queryContextMaxPlannerConfirmedEntities,
		MaxCandidateGroups:    queryContextMaxPlannerCandidateGroups,
		MaxCandidatesPerGroup: queryContextMaxPlannerCandidatesPerGroup,
		MaxDialogueTurns:      queryContextMaxPlannerDialogueTurns,
	})
	body, err := json.Marshal(map[string]any{
		"query_evidence_window": window,
	})
	if err != nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf(`查询证据窗口：
%s

使用规则：
	- query_evidence_window 只是历史事实、用户/助手文本摘要与只读 observation，不是本地目标绑定。
	- 当前用户输入优先；模型负责判断是否引用历史事实、是否需要继续查询、是否需要澄清。
	- 若当前用户输入显式纠正、否定或扩大历史范围，例如“不只是/不是/不限于/不要只看 X，而是全部/所有”，必须把 observations/recent_turns 视为被纠正的历史结果，不得继承旧 keyword、parent_org_code、entity_key 或 target set。
	- open_clarification.reply_candidate=true 表示当前输入可能在回答上一轮澄清；不要因为输入短就抢先输出 NO_QUERY。
	- observations.kind=entity_fact 只表示先前工具结果曾产生某个实体事实，不是当前轮 winner。
	- observations.kind=presented_options 只表示先前给用户展示过一组选项；用户说“第一个/第二个/以上/全部/这些/都要/不是这个/另一个”时，由模型结合 recent_turns 和当前输入自行判断。
- observations.kind=result_list 表示上一轮已经成功返回过一组明确结果；若当前轮要求“补充字段/增加列/列出路径”，可将该组 entity_key 作为当前 target set，并在规模可控时生成线性 READ_PLAN 逐个补查详情字段。
- 如果目标明确，输出显式 ReadPlan 参数。
- 如果缺少执行所需事实，由模型生成澄清问题。
- 如果已有 working_results 足够，由模型输出 DONE。
- 本地不会替你从历史上下文补单个 winner，也不会因为输入短而抢先拒绝。
- 对 result_list 的自动续接只适用于小批量、明确对象集合；若对象过多，必须改为 CLARIFY 要求用户缩小范围。
	- 该窗口不是授权来源，也不是会话压缩摘要。`, string(body)))
}

func buildQueryEvidenceWindow(
	queryContext cubebox.QueryContext,
	currentUserInput string,
	budget cubeboxQueryEvidenceWindowProjectionBudget,
) cubebox.QueryEvidenceWindow {
	window := cubebox.BuildQueryEvidenceWindow(queryContext, currentUserInput, cubebox.QueryEvidenceWindowBudget{
		MaxEntityObservations: budget.MaxConfirmedEntities,
		MaxOptionGroups:       budget.MaxCandidateGroups,
		MaxOptionsPerGroup:    budget.MaxCandidatesPerGroup,
		MaxDialogueTurns:      budget.MaxDialogueTurns,
	})
	if queryPromptOverridesHistoricalScope(currentUserInput) {
		return cubebox.QueryEvidenceWindow{
			CurrentUserInput: window.CurrentUserInput,
		}
	}
	return window
}

func queryPromptOverridesHistoricalScope(prompt string) bool {
	text := strings.ToLower(strings.TrimSpace(prompt))
	if text == "" {
		return false
	}
	for _, marker := range []string{
		"不只是", "不只", "不仅是", "不限于", "不限", "不要只", "别只",
		"not just", "not only", "not limited to", "instead of only",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return strings.Contains(text, "不是") && strings.Contains(text, "而是")
}

func buildReadAPICatalogPromptBlock(entries []cubebox.ReadAPICatalogEntry) string {
	if len(entries) == 0 {
		return ""
	}
	body, err := json.Marshal(map[string]any{"read_api_catalog": entries})
	if err != nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf(`已登记只读 API 目录：
%s

使用规则：
- READ_PLAN.steps[].api_key 必须来自 read_api_catalog。
- steps[].params 只能包含对应 API 的 required_params 与 optional_params。
- 缺少 required_params 时输出 CLARIFY，不要猜测。`, string(body)))
}

func buildWorkingResultsPromptBlock(snapshot cubebox.QueryWorkingResults) string {
	body := cubebox.WorkingResultsPromptBlock(snapshot)
	if strings.TrimSpace(body) == "" {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf(`当前 turn 内 working_results：
%s

使用规则：
- working_results 只是当前 turn 已执行只读工具的临时 observation，不是长期会话事实。
- completed_plans 与 latest_observation 说明已经查过什么；executed_fingerprints 说明哪些查询步骤不能重复执行。
- 如果 latest_observation 已足够回答用户问题，输出 {"outcome":"DONE"}。
- 如果还需要继续查询，输出新的 {"outcome":"READ_PLAN","plan":...}，且不得重复 executed_fingerprints 中的查询。
- 如果缺少必要参数或无法稳定判断引用对象，输出 CLARIFY。
- 如果请求超出查询域，输出 NO_QUERY；不要用 NO_QUERY 表示已经查够。`, body))
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
	envelope := buildQueryNarrationEnvelope(input)
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
- 根据用户问题和结果内容选择合适的表达结构；可以是短答、分段、小标题或列表。
- 把枚举、布尔和空值翻译成自然中文，例如 active=启用、disabled=停用、true=是、false=否、null/空字符串/空列表=未记录或没有。
- 对单个实体详情，优先直接回答并补充关键事实；如用户需要对比、审计或明细，可以展开说明。
- 如果输入里带有 query_evidence_window，可用它解释“刚才那个/继续查”的衔接关系，但不得把上下文当成新的业务事实源。
- 如果某些字段为空，只在和用户问题相关时用一句话说明“未记录……”，不要机械逐项写“空”。

硬约束：
- 只能依据输入 JSON 中的 user_prompt、query_evidence_window、results 叙述。
- 事实性结论只能来自 results，不得从 query_evidence_window 推导新的业务事实。
- 不得编造任何 results 中不存在的字段、值、条数、层级或结论。
- 不得补做新的查询、推断新的默认值、追加新的澄清问题。
- 不得输出 Markdown 代码块。
- 不得逐字回显整份原始 JSON。
- 不得暴露实现细节或计划执行痕迹；不要出现“step-1”“api_key”“result_focus”“payload”“results”“executor_key”“params.org_code”“plan.steps”这类内部字段路径或执行结构名。
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

func buildNoQueryGuidanceEnvelope(input cubeboxNoQueryGuidanceInput) cubeboxNoQueryGuidanceEnvelope {
	return cubeboxNoQueryGuidanceEnvelope{
		ScopeSummary:     strings.TrimSpace(input.ScopeSummary),
		SuggestedPrompts: normalizeNoQuerySuggestedPrompts(input.SuggestedPrompts),
		QueryContextHint: input.QueryContextHint,
	}
}

func buildNoQueryGuidanceMessages(body string) []cubebox.PromptItem {
	return []cubebox.PromptItem{
		{
			Role: "system",
			Content: strings.TrimSpace(`
你负责把给定的受控事实整理成面向用户的中文回复。

要求：
1. 先用一句话说明当前支持范围。
2. 空一行后输出“你可以直接这样问：”
3. 然后输出带序号的列表，使用 1. 2. 3. 格式，每条单独一行。
4. 只能使用 provided suggested_prompts，不得新增未提供的能力或示例。
5. 不得提到内部术语，例如 NO_QUERY、ReadPlan、planner、知识包、API。
6. 语气直接、简洁，不道歉，不解释系统内部机制。
7. 输出纯文本，直接作为用户可见消息。
`),
		},
		{
			Role:    "user",
			Content: body,
		},
	}
}

func buildQueryClarificationMessages(body string) []cubebox.PromptItem {
	return []cubebox.PromptItem{
		{
			Role: "system",
			Content: strings.TrimSpace(`
你是 CubeBox 的查询澄清器。
你的职责只有一件事：当执行阶段发现多个候选对象时，基于当前问题、轻量上下文和候选列表，向用户生成一句自然中文追问。

要求：
- 只允许要求用户从候选中确认目标，不能静默替用户选择。
- 输入中的 error_code、candidate_source、candidate_count、cannot_silent_select 是结构化澄清事实；可以用于组织追问，但不要把这些字段名直接输出给用户。
- 直接给出追问，必要时附上极短候选列表。
- 可以用“刚才那个/继续查”的语气承接上下文，但不得编造候选之外的新事实。
- 不得输出 Markdown 代码块、JSON、内部字段名或执行痕迹。
- 输出纯文本，直接作为用户可见消息。
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

func validateNoQueryGuidanceText(text string, facts cubeboxNoQueryGuidanceEnvelope) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return cubebox.ErrProviderStreamInvalid
	}
	if err := validateQueryNarrationText(text); err != nil {
		return err
	}
	for _, forbidden := range []string{"NO_QUERY", "ReadPlan", "planner", "知识包", "API"} {
		if strings.Contains(text, forbidden) {
			return errCubeboxQueryNarrationContractViolation
		}
	}
	if facts.ScopeSummary != "" && !strings.Contains(text, facts.ScopeSummary) {
		return errCubeboxQueryNarrationContractViolation
	}
	for _, prompt := range facts.SuggestedPrompts {
		if !strings.Contains(text, prompt) {
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

func (f *cubeboxQueryFlow) produceReadPlan(
	ctx context.Context,
	request cubebox.GatewayStreamRequest,
	queryContext cubebox.QueryContext,
	readAPICatalog []cubebox.ReadAPICatalogEntry,
	workingState *cubebox.QueryWorkingResultsState,
) (cubeboxReadPlanProductionResult, error) {
	if f == nil || f.producer == nil {
		return cubeboxReadPlanProductionResult{}, cubebox.ErrProviderConfigInvalid
	}
	queryContext = withClarificationResume(queryContext, request.Prompt)
	corrections := []string{}
	paginationCorrected := false
	scopeOverrideCorrected := false
	for {
		snapshot := workingState.Snapshot()
		produced, err := f.producer.ProduceReadPlan(ctx, cubeboxReadPlanProductionInput{
			TenantID:       strings.TrimSpace(request.TenantID),
			PrincipalID:    strings.TrimSpace(request.PrincipalID),
			ConversationID: strings.TrimSpace(request.ConversationID),
			Prompt:         request.Prompt,
			KnowledgePacks: append([]cubebox.KnowledgePack(nil), f.knowledgePacks...),
			QueryContext:   queryContext,
			ReadAPICatalog: append([]cubebox.ReadAPICatalogEntry(nil), readAPICatalog...),
			WorkingResults: &snapshot,
			Corrections:    append([]string(nil), corrections...),
		})
		if err != nil {
			if shouldDowngradeQueryBoundaryViolationToNoQuery(err, request.Prompt, snapshot) {
				return cubeboxReadPlanProductionResult{
					Handled:         false,
					Outcome:         cubebox.PlannerOutcome{Type: cubebox.PlannerOutcomeNoQuery},
					ExplicitOutcome: true,
				}, nil
			}
			return cubeboxReadPlanProductionResult{}, err
		}
		produced.Outcome = normalizeProducedPlannerOutcome(produced, workingState)
		if plannerClarificationOnlyMissingPaginationControls(produced.Outcome) {
			if paginationCorrected || workingState == nil || !workingState.CanPlan() {
				return cubeboxReadPlanProductionResult{}, cubebox.ErrReadPlanBoundaryViolation
			}
			corrections = append(corrections, "上一轮输出了只缺 page/size 的 CLARIFY；这是无效输出。page/size 是执行控制参数，不是业务缺参；不得追问 page/size，请默认 page=1,size=100 并输出 READ_PLAN。")
			paginationCorrected = true
			workingState.NotePlanningRound()
			continue
		}
		if plannerOutcomeConflictsWithAllOrgScopeCorrection(request.Prompt, produced.Outcome, queryContext, snapshot) {
			if scopeOverrideCorrected || workingState == nil || !workingState.CanPlan() {
				return cubeboxReadPlanProductionResult{}, cubebox.ErrReadPlanBoundaryViolation
			}
			corrections = append(corrections, "当前用户输入正在把上一轮关键词/上级组织/单实体范围纠正为全部组织；前半句只描述被否定的旧范围。不得输出 keyword、parent_org_code、org_code、entity_key 或单实体详情计划；请按全部组织重新规划 orgunit.list，设置 all_org_units=true，默认 as_of=今天、page=1,size=100。")
			scopeOverrideCorrected = true
			workingState.NotePlanningRound()
			continue
		}
		if produced.Outcome.Type == cubebox.PlannerOutcomeReadPlan || produced.Outcome.Type == cubebox.PlannerOutcomeClarify {
			produced.Plan = produced.Outcome.Plan
		}
		produced.Handled = produced.Outcome.Type != cubebox.PlannerOutcomeNoQuery
		return produced, nil
	}
}

func shouldDowngradeQueryBoundaryViolationToNoQuery(err error, prompt string, snapshot cubebox.QueryWorkingResults) bool {
	if !errors.Is(err, cubebox.ErrReadPlanBoundaryViolation) {
		return false
	}
	if snapshot.RoundIndex > 1 || len(snapshot.CompletedPlans) > 0 || len(snapshot.ExecutedFingerprints) > 0 || snapshot.LatestObservation != nil {
		return false
	}
	return queryPromptMentionsUnsupportedOrgUnitDimension(prompt)
}

func queryPromptMentionsUnsupportedOrgUnitDimension(prompt string) bool {
	text := strings.ToLower(strings.TrimSpace(prompt))
	if text == "" {
		return false
	}
	unsupportedTerms := []string{
		"成本组织",
		"成本中心",
		"组织类型",
		"org type",
		"org_type",
		"cost center",
		"cost org",
	}
	for _, term := range unsupportedTerms {
		if strings.Contains(text, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

func normalizeProducedPlannerOutcome(produced cubeboxReadPlanProductionResult, workingState *cubebox.QueryWorkingResultsState) cubebox.PlannerOutcome {
	if produced.ExplicitOutcome && produced.Outcome.Type != "" {
		return produced.Outcome
	}
	if !produced.Handled {
		return cubebox.PlannerOutcome{Type: cubebox.PlannerOutcomeNoQuery}
	}
	if len(produced.Plan.MissingParams) > 0 {
		return cubebox.PlannerOutcome{
			Type:               cubebox.PlannerOutcomeClarify,
			Plan:               produced.Plan,
			MissingParams:      append([]string(nil), produced.Plan.MissingParams...),
			ClarifyingQuestion: strings.TrimSpace(produced.Plan.ClarifyingQuestion),
		}
	}
	if workingState != nil && workingState.HasExecution() && len(produced.Plan.Steps) == 0 {
		return cubebox.PlannerOutcome{Type: cubebox.PlannerOutcomeDone}
	}
	if workingState != nil && workingState.HasExecution() && len(produced.Plan.Steps) > 0 {
		allStepsExecuted := true
		for _, step := range produced.Plan.Steps {
			fingerprint := cubebox.StepFingerprint(step)
			if fingerprint == "" || !workingState.HasExecuted(fingerprint) {
				allStepsExecuted = false
				break
			}
		}
		if allStepsExecuted {
			return cubebox.PlannerOutcome{Type: cubebox.PlannerOutcomeDone}
		}
	}
	return cubebox.PlannerOutcome{Type: cubebox.PlannerOutcomeReadPlan, Plan: produced.Plan}
}

func plannerClarificationOnlyMissingPaginationControls(outcome cubebox.PlannerOutcome) bool {
	if outcome.Type != cubebox.PlannerOutcomeClarify {
		return false
	}
	missingParams := outcome.MissingParams
	if len(missingParams) == 0 {
		missingParams = outcome.Plan.MissingParams
	}
	if len(missingParams) == 0 {
		return false
	}
	for _, item := range missingParams {
		switch strings.ToLower(strings.TrimSpace(item)) {
		case "page", "size":
			continue
		default:
			return false
		}
	}
	return true
}

func plannerOutcomeConflictsWithAllOrgScopeCorrection(prompt string, outcome cubebox.PlannerOutcome, queryContext cubebox.QueryContext, snapshot cubebox.QueryWorkingResults) bool {
	if outcome.Type != cubebox.PlannerOutcomeReadPlan {
		return false
	}
	if !queryPromptOverridesToAllOrgScope(prompt) {
		return false
	}
	if !allOrgScopePlanHasNarrowingConflict(outcome.Plan) {
		return false
	}
	return allOrgScopeConflictHasHistoricalEvidence(queryContext, snapshot)
}

func repeatedPlanCanCompleteAllOrgScopeCorrection(prompt string, plan cubebox.ReadPlan, snapshot cubebox.QueryWorkingResults) bool {
	if !queryPromptOverridesToAllOrgScope(prompt) {
		return false
	}
	if allOrgScopePlanHasNarrowingConflict(plan) {
		return false
	}
	if !allOrgScopeWorkingResultsHasExecution(snapshot) {
		return false
	}
	for _, step := range plan.Steps {
		fingerprint := cubebox.StepFingerprint(step)
		if fingerprint == "" || !queryStringSliceContains(snapshot.ExecutedFingerprints, fingerprint) {
			return false
		}
	}
	return len(plan.Steps) > 0
}

func queryPromptOverridesToAllOrgScope(prompt string) bool {
	if !queryPromptOverridesHistoricalScope(prompt) {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(prompt))
	if text == "" {
		return false
	}
	for _, marker := range []string{
		"全部", "所有", "全量", "全租户", "不限关键字", "不限特定关键字", "不限层级",
		"all org", "all organization", "all organisations", "all organizations",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func allOrgScopePlanHasNarrowingConflict(plan cubebox.ReadPlan) bool {
	intent := strings.ToLower(strings.TrimSpace(plan.Intent))
	if intent == "orgunit.details" || intent == "orgunit.search" || intent == "orgunit.audit" || intent == "orgunit.tree" {
		return true
	}
	for _, step := range plan.Steps {
		apiKey := strings.ToLower(strings.TrimSpace(step.APIKey))
		if apiKey == "orgunit.details" || apiKey == "orgunit.search" || apiKey == "orgunit.audit" || apiKey == "orgunit.tree" {
			return true
		}
		for _, param := range []string{"keyword", "parent_org_code", "org_code", "entity_key", "target_org_code"} {
			if paramValueIsPresent(step.Params[param]) {
				return true
			}
		}
		if apiKey == "orgunit.list" && !paramBoolIsTrue(step.Params["all_org_units"]) && !paramValueIsPresent(step.Params["keyword"]) && !paramValueIsPresent(step.Params["is_business_unit"]) {
			return true
		}
	}
	return false
}

func allOrgScopeConflictHasHistoricalEvidence(queryContext cubebox.QueryContext, snapshot cubebox.QueryWorkingResults) bool {
	if len(queryContext.RecentDialogueTurns) > 0 ||
		len(queryContext.RecentConfirmedEntities) > 0 ||
		len(queryContext.RecentCandidateGroups) > 0 ||
		queryContext.LastClarification != nil ||
		queryContext.ClarificationResume != nil {
		return true
	}
	return allOrgScopeWorkingResultsHasExecution(snapshot)
}

func allOrgScopeWorkingResultsHasExecution(snapshot cubebox.QueryWorkingResults) bool {
	if strings.TrimSpace(snapshot.OriginalUserGoal) != "" && !queryPromptOverridesToAllOrgScope(snapshot.OriginalUserGoal) {
		return true
	}
	if len(snapshot.CompletedPlans) > 0 || snapshot.LatestObservation != nil || len(snapshot.ExecutedFingerprints) > 0 {
		return true
	}
	return len(snapshot.RepeatObservations) > 0
}

func paramValueIsPresent(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(v) != ""
	case []string:
		return len(v) > 0
	case []any:
		return len(v) > 0
	default:
		return true
	}
}

func paramBoolIsTrue(value any) bool {
	v, ok := value.(bool)
	return ok && v
}

func queryStringSliceContains(items []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, item := range items {
		if strings.TrimSpace(item) == target {
			return true
		}
	}
	return false
}

func noteRepeatedPlanSteps(workingState *cubebox.QueryWorkingResultsState, plan cubebox.ReadPlan) (bool, bool) {
	if workingState == nil {
		return false, false
	}
	duplicated := false
	exceeded := false
	for _, step := range plan.Steps {
		fingerprint := cubebox.StepFingerprint(step)
		if fingerprint == "" || !workingState.HasExecuted(fingerprint) {
			continue
		}
		duplicated = true
		if workingState.NoteRepeat(fingerprint) {
			exceeded = true
		}
	}
	return exceeded, duplicated
}

func confirmedQueryEntity(results []cubebox.ExecuteResult) *cubebox.QueryEntity {
	for i := len(results) - 1; i >= 0; i-- {
		if entity := normalizedQueryEntity(results[i].ConfirmedEntity); entity != nil {
			return entity
		}
	}
	return nil
}

func queryPresentedCandidates(results []cubebox.ExecuteResult) []cubebox.QueryCandidate {
	for _, result := range results {
		candidates := extractQueryCandidatesFromPayload(result.Payload)
		if len(candidates) > 0 {
			return candidates
		}
	}
	return nil
}

func newQueryCandidateGroupID() string {
	return "candgrp_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}

func queryCandidatePayloads(items []cubebox.QueryCandidate) []any {
	out := make([]any, 0, len(items))
	for _, item := range items {
		payload := item.Payload()
		if len(payload) == 0 {
			continue
		}
		out = append(out, payload)
	}
	return out
}

func extractQueryCandidatesFromPayload(payload map[string]any) []cubebox.QueryCandidate {
	if len(payload) == 0 {
		return nil
	}
	if rawItems, ok := payload["org_units"].([]any); ok {
		candidates := make([]cubebox.QueryCandidate, 0, minIntServer(len(rawItems), 100))
		asOf := strings.TrimSpace(stringValue(payload["as_of"]))
		for _, rawItem := range rawItems {
			item, ok := rawItem.(map[string]any)
			if !ok {
				continue
			}
			candidate := cubebox.NormalizeQueryCandidate(cubebox.QueryCandidate{
				Domain:    "orgunit",
				EntityKey: stringValue(item["org_code"]),
				Name:      stringValue(item["name"]),
				AsOf:      asOf,
				Status:    stringValue(item["status"]),
			})
			if candidate == nil {
				continue
			}
			candidates = append(candidates, *candidate)
			if len(candidates) >= 100 {
				break
			}
		}
		return candidates
	}
	if targetOrgCode := strings.TrimSpace(stringValue(payload["target_org_code"])); targetOrgCode != "" {
		candidate := cubebox.NormalizeQueryCandidate(cubebox.QueryCandidate{
			Domain:    "orgunit",
			EntityKey: targetOrgCode,
			Name:      stringValue(payload["target_name"]),
			AsOf:      strings.TrimSpace(stringValue(payload["tree_as_of"])),
		})
		if candidate != nil {
			return []cubebox.QueryCandidate{*candidate}
		}
	}
	return nil
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return ""
	}
}

func minIntServer(a int, b int) int {
	if a < b {
		return a
	}
	return b
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
	queryContext cubebox.QueryContext,
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
	answer := f.noQueryGuidanceText(ctx, request, queryContext, produced)
	if !writeEvent("turn.agent_message.delta", map[string]any{"message_id": prepared.turn.AssistantMessageID, "delta": answer}) {
		return true
	}
	_ = writeEvent("turn.agent_message.completed", map[string]any{"message_id": prepared.turn.AssistantMessageID})
	_ = writeEvent("turn.completed", f.queryCompletedPayload("completed", prepared.lifecycle))
	return true
}

func (f *cubeboxQueryFlow) writeQueryPlannerTerminalError(
	ctx context.Context,
	request cubebox.GatewayStreamRequest,
	sink cubebox.GatewayEventSink,
	produced cubeboxReadPlanProductionResult,
	terminal queryTerminalError,
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
	f.writeQueryTerminalError(ctx, request, prepared.turn.TurnID, &prepared.sequence, prepared.lifecycle, sink, terminal)
	return true
}

func (f *cubeboxQueryFlow) noQueryGuidanceText(
	ctx context.Context,
	request cubebox.GatewayStreamRequest,
	queryContext cubebox.QueryContext,
	produced cubeboxReadPlanProductionResult,
) string {
	input := f.noQueryGuidanceInput(request, queryContext, produced)
	if strings.TrimSpace(input.ScopeSummary) == "" || len(normalizeNoQuerySuggestedPrompts(input.SuggestedPrompts)) == 0 {
		return fallbackNoQueryGuidanceText(buildNoQueryGuidanceEnvelope(input))
	}
	if narrator, ok := f.narrator.(cubeboxNoQueryGuidanceNarrator); ok {
		if text, err := narrator.NarrateNoQueryGuidance(ctx, input); err == nil && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}
	return fallbackNoQueryGuidanceText(buildNoQueryGuidanceEnvelope(input))
}

func (f *cubeboxQueryFlow) noQueryGuidanceInput(
	request cubebox.GatewayStreamRequest,
	queryContext cubebox.QueryContext,
	produced cubeboxReadPlanProductionResult,
) cubeboxNoQueryGuidanceInput {
	guidance := cubebox.NoQueryGuidanceFromKnowledgePacks(f.knowledgePacks)
	prompts := guidance.SuggestedPrompts
	return cubeboxNoQueryGuidanceInput{
		TenantID:         request.TenantID,
		Prompt:           request.Prompt,
		ScopeSummary:     guidance.ScopeSummary,
		SuggestedPrompts: prompts,
		QueryContextHint: cubeboxNoQueryContextHint{
			HasConversationEvidence: hasQueryConversationEvidence(queryContext),
		},
		ExpectedProviderID:   produced.ProviderID,
		ExpectedProviderType: produced.ProviderType,
		ExpectedModelSlug:    produced.ModelSlug,
	}
}

func hasQueryConversationEvidence(queryContext cubebox.QueryContext) bool {
	return len(queryContext.RecentDialogueTurns) > 0 ||
		len(queryContext.RecentConfirmedEntities) > 0 ||
		len(queryContext.RecentCandidateGroups) > 0 ||
		queryContext.LastClarification != nil ||
		queryContext.ClarificationResume != nil
}

func fallbackNoQueryGuidanceText(facts cubeboxNoQueryGuidanceEnvelope) string {
	scope := strings.TrimSpace(facts.ScopeSummary)
	if scope == "" {
		scope = "当前输入未进入已支持查询闭环。"
	}
	prompts := normalizeNoQuerySuggestedPrompts(facts.SuggestedPrompts)
	if len(prompts) == 0 {
		prompts = []string{"请换成明确的数据查询问题"}
	}
	var out strings.Builder
	out.WriteString(scope)
	out.WriteString("\n\n你可以直接这样问：")
	for index, prompt := range prompts {
		out.WriteString("\n")
		out.WriteString(fmt.Sprintf("%d. %s", index+1, prompt))
	}
	return out.String()
}

func normalizeNoQuerySuggestedPrompts(items []string) []string {
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func (f *cubeboxQueryFlow) writePlannerClarification(
	ctx context.Context,
	request cubebox.GatewayStreamRequest,
	prepared cubeboxPreparedQueryTurn,
	outcome cubebox.PlannerOutcome,
	sink cubebox.GatewayEventSink,
	writeEvent func(string, map[string]any) bool,
) bool {
	text := strings.TrimSpace(outcome.ClarifyingQuestion)
	if text == "" {
		text = strings.TrimSpace(outcome.Plan.ClarifyingQuestion)
	}
	missingParams := append([]string(nil), outcome.MissingParams...)
	if len(missingParams) == 0 {
		missingParams = append([]string(nil), outcome.Plan.MissingParams...)
	}
	if text == "" || len(missingParams) == 0 {
		f.writeQueryTerminalError(ctx, request, prepared.turn.TurnID, &prepared.sequence, prepared.lifecycle, sink, queryPlanErrorToTerminal(cubebox.ErrReadPlanBoundaryViolation))
		return true
	}
	if !writeEvent("turn.agent_message.delta", map[string]any{"message_id": prepared.turn.AssistantMessageID, "delta": text}) {
		return true
	}
	f.appendQueryMetadataEvent(ctx, request, prepared.turn.TurnID, &prepared.sequence, cubebox.QueryClarificationRequestedEventType, map[string]any{
		"source_turn_id":      prepared.turn.TurnID,
		"intent":              strings.TrimSpace(outcome.Plan.Intent),
		"missing_params":      missingParams,
		"clarifying_question": text,
	})
	_ = writeEvent("turn.agent_message.completed", map[string]any{"message_id": prepared.turn.AssistantMessageID})
	_ = writeEvent("turn.completed", f.queryCompletedPayload("completed", prepared.lifecycle))
	return true
}

func (f *cubeboxQueryFlow) writeQueryExecutionClarification(
	ctx context.Context,
	request cubebox.GatewayStreamRequest,
	prepared cubeboxPreparedQueryTurn,
	queryContext cubebox.QueryContext,
	produced cubeboxReadPlanProductionResult,
	err error,
	writeEvent func(string, map[string]any) bool,
) bool {
	candidates := queryExecutionCandidates(err)
	if len(candidates) == 0 {
		return false
	}
	errorCode, candidateSource, candidateCount, cannotSilentSelect := queryExecutionClarificationFacts(err)
	candidateGroup := cubebox.QueryCandidateGroup{
		GroupID:            newQueryCandidateGroupID(),
		CandidateSource:    candidateSource,
		CandidateCount:     candidateCount,
		CannotSilentSelect: cannotSilentSelect,
		Candidates:         append([]cubebox.QueryCandidate(nil), candidates...),
	}
	text := f.buildExecutionClarificationText(ctx, request, produced, queryContext, candidateGroup, errorCode)
	if text == "" {
		text = "找到了多个候选项，请从中明确你要继续查询的对象。"
	}
	if !writeEvent("turn.agent_message.delta", map[string]any{"message_id": prepared.turn.AssistantMessageID, "delta": text}) {
		return true
	}
	payload := map[string]any{
		"source_turn_id":      prepared.turn.TurnID,
		"clarifying_question": text,
	}
	if candidateGroup.GroupID != "" {
		payload["candidate_group_id"] = candidateGroup.GroupID
	}
	if errorCode != "" {
		payload["error_code"] = errorCode
	}
	if candidateGroup.CandidateSource != "" {
		payload["candidate_source"] = candidateGroup.CandidateSource
	}
	if candidateGroup.CandidateCount > 0 {
		payload["candidate_count"] = candidateGroup.CandidateCount
	}
	if candidateGroup.CannotSilentSelect {
		payload["cannot_silent_select"] = true
	}
	f.appendQueryMetadataEvent(ctx, request, prepared.turn.TurnID, &prepared.sequence, cubebox.QueryClarificationRequestedEventType, payload)
	f.appendQueryMetadataEvent(ctx, request, prepared.turn.TurnID, &prepared.sequence, cubebox.QueryCandidatesPresentedEventType, candidateGroup.Payload())
	_ = writeEvent("turn.agent_message.completed", map[string]any{"message_id": prepared.turn.AssistantMessageID})
	_ = writeEvent("turn.completed", f.queryCompletedPayload("completed", prepared.lifecycle))
	return true
}

func (f *cubeboxQueryFlow) writeQueryResultMetadata(
	ctx context.Context,
	request cubebox.GatewayStreamRequest,
	turnID string,
	sequence *int,
	results []cubebox.ExecuteResult,
) {
	if anchor := confirmedQueryEntity(results); anchor != nil {
		f.appendQueryMetadataEvent(ctx, request, turnID, sequence, cubebox.QueryEntityConfirmedEventType, map[string]any{"entity": anchor.Payload()})
	}
	if candidates := queryPresentedCandidates(results); len(candidates) > 0 {
		f.appendQueryMetadataEvent(ctx, request, turnID, sequence, cubebox.QueryCandidatesPresentedEventType, cubebox.QueryCandidateGroup{
			GroupID:         newQueryCandidateGroupID(),
			CandidateSource: "results",
			CandidateCount:  len(candidates),
			Candidates:      append([]cubebox.QueryCandidate(nil), candidates...),
		}.Payload())
	}
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
		return cubeboxPreparedQueryTurn{}, err
	}
	turn := f.runtime.StartTurnWithIDs(cubebox.TurnOwner{
		TenantID:       request.TenantID,
		PrincipalID:    request.PrincipalID,
		ConversationID: request.ConversationID,
	}, request.Prompt, prepared.TurnIDs)
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
	if errors.Is(err, cubebox.ErrPlannerOutcomeInvalid) {
		return queryTerminalError{Code: "cubebox_query_planner_outcome_invalid", Message: "未能形成可执行查询计划，请换一种说法或补充查询条件后重试。", Retryable: false}
	}
	if errors.Is(err, cubebox.ErrReadPlanSchemaConstrainedDecodeFailed) {
		return queryTerminalError{Code: "ai_plan_schema_constrained_decode_failed", Message: "查询计划解析失败，请补全信息后重试。", Retryable: false}
	}
	return queryTerminalError{Code: "ai_plan_boundary_violation", Message: "查询计划超出允许范围，请调整问题后重试。", Retryable: false}
}

func queryPlannerErrorToTerminal(err error) queryTerminalError {
	if isQueryPlannerContractError(err) {
		return queryPlanErrorToTerminal(err)
	}
	switch {
	case errors.Is(err, errCubeboxQueryNarrationTargetMismatch):
		return queryTerminalError{Code: "ai_reply_model_target_mismatch", Message: "查询计划未命中预期的大模型链路，请稍后重试。", Retryable: false}
	case errors.Is(err, errCubeboxQueryNarrationContractViolation):
		return queryTerminalError{Code: "ai_reply_render_failed", Message: "查询计划生成未通过输出约束校验，请稍后重试。", Retryable: true}
	case errors.Is(err, cubebox.ErrProviderDisabled), errors.Is(err, cubebox.ErrModelSlugMissing), errors.Is(err, cubebox.ErrProviderConfigInvalid):
		return queryTerminalError{Code: "ai_model_config_invalid", Message: "模型配置无效，请联系管理员检查。", Retryable: false}
	case errors.Is(err, cubebox.ErrSecretMissing), errors.Is(err, cubebox.ErrSecretRefInvalid):
		return queryTerminalError{Code: "ai_model_secret_missing", Message: "当前模型密钥不可用，请联系管理员检查。", Retryable: false}
	case errors.Is(err, cubebox.ErrProviderUnauthorized):
		return queryTerminalError{Code: "ai_model_provider_unavailable", Message: "当前模型认证失败，请联系管理员检查。", Retryable: false}
	case errors.Is(err, cubebox.ErrProviderRateLimited):
		return queryTerminalError{Code: "ai_reply_render_failed", Message: "查询计划生成失败，请稍后重试。", Retryable: true}
	case errors.Is(err, cubebox.ErrProviderUnavailable), errors.Is(err, cubebox.ErrProviderTimeout), errors.Is(err, cubebox.ErrProviderStreamInvalid):
		return queryTerminalError{Code: "ai_reply_render_failed", Message: "查询计划生成失败，请稍后重试。", Retryable: true}
	default:
		return queryTerminalError{Code: "ai_reply_render_failed", Message: "查询计划生成失败，请稍后重试。", Retryable: false}
	}
}

func isQueryPlannerContractError(err error) bool {
	return errors.Is(err, cubebox.ErrPlannerOutcomeInvalid) ||
		errors.Is(err, cubebox.ErrReadPlanSchemaConstrainedDecodeFailed) ||
		errors.Is(err, cubebox.ErrReadPlanBoundaryViolation)
}

func queryLoopBudgetExceededTerminal() queryTerminalError {
	return queryTerminalError{Code: "cubebox_query_loop_budget_exceeded", Message: "这次查询需要的步骤超出当前单轮预算，请缩小查询范围后重试。", Retryable: false}
}

func queryLoopRepeatedPlanTerminal() queryTerminalError {
	return queryTerminalError{Code: "cubebox_query_loop_repeated_plan", Message: "查询计划重复且无法继续推进，请缩小范围或换一种说法后重试。", Retryable: false}
}

func queryDoneWithoutResultTerminal() queryTerminalError {
	return queryTerminalError{Code: "cubebox_query_done_without_result", Message: "查询计划未产生可用结果，请补充查询条件后重试。", Retryable: false}
}

func queryNoQueryAfterExecutionTerminal() queryTerminalError {
	return queryTerminalError{Code: "cubebox_query_no_query_after_execution", Message: "查询计划在执行后偏离支持范围，请换一种说法后重试。", Retryable: false}
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

func queryExecutionCandidates(err error) []cubebox.QueryCandidate {
	var ambiguous *orgUnitSearchAmbiguousError
	if errors.As(err, &ambiguous) {
		return ambiguous.QueryCandidates()
	}
	return nil
}

func queryExecutionClarificationFacts(err error) (string, string, int, bool) {
	var ambiguous *orgUnitSearchAmbiguousError
	if errors.As(err, &ambiguous) {
		return "org_unit_search_ambiguous", "execution_error", len(ambiguous.QueryCandidates()), true
	}
	return "", "", 0, false
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

func (f *cubeboxQueryFlow) buildExecutionClarificationText(
	ctx context.Context,
	request cubebox.GatewayStreamRequest,
	produced cubeboxReadPlanProductionResult,
	queryContext cubebox.QueryContext,
	candidateGroup cubebox.QueryCandidateGroup,
	errorCode string,
) string {
	if len(candidateGroup.Candidates) == 0 {
		return ""
	}
	queryContext = withAppendedQueryCandidateGroup(queryContext, candidateGroup)
	projectedGroup := latestProjectedCandidateGroup(projectQueryCandidateGroups(
		queryContext.RecentCandidateGroups,
		queryContextMaxClarifierCandidateGroups,
		queryContextMaxClarifierCandidatesPerGroup,
	))
	if len(projectedGroup.Candidates) == 0 {
		projectedGroup = candidateGroup
	}
	if f == nil || f.clarifier == nil {
		return fallbackCandidateClarificationText(projectedGroup.Candidates)
	}
	text, err := f.clarifier.ClarifyQuery(ctx, cubeboxQueryClarificationInput{
		TenantID:             request.TenantID,
		Prompt:               request.Prompt,
		QueryContext:         queryContext,
		Candidates:           append([]cubebox.QueryCandidate(nil), projectedGroup.Candidates...),
		CandidateGroupID:     projectedGroup.GroupID,
		ErrorCode:            strings.TrimSpace(errorCode),
		CandidateSource:      strings.TrimSpace(projectedGroup.CandidateSource),
		CandidateCount:       projectedGroup.CandidateCount,
		CannotSilentSelect:   projectedGroup.CannotSilentSelect,
		ExpectedProviderID:   produced.ProviderID,
		ExpectedProviderType: produced.ProviderType,
		ExpectedModelSlug:    produced.ModelSlug,
	})
	if err != nil {
		return fallbackCandidateClarificationText(projectedGroup.Candidates)
	}
	return strings.TrimSpace(text)
}

func fallbackCandidateClarificationText(candidates []cubebox.QueryCandidate) string {
	items := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		code := strings.TrimSpace(candidate.EntityKey)
		name := strings.TrimSpace(candidate.Name)
		status := strings.TrimSpace(candidate.Status)
		item := code
		if name != "" {
			if item != "" {
				item += "「" + name + "」"
			} else {
				item = "「" + name + "」"
			}
		}
		if status != "" {
			item += "（" + status + "）"
		}
		if item != "" {
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return "找到了多个候选项，请明确你要继续查询的对象。"
	}
	return "找到了多个候选项，请确认要继续查询哪一个：" + strings.Join(items, "、") + "。"
}

func buildQueryClarificationEnvelope(input cubeboxQueryClarificationInput) cubeboxQueryClarificationEnvelope {
	queryContext := input.QueryContext
	currentGroup := cubebox.QueryCandidateGroup{
		GroupID:            strings.TrimSpace(input.CandidateGroupID),
		CandidateSource:    strings.TrimSpace(input.CandidateSource),
		CandidateCount:     input.CandidateCount,
		CannotSilentSelect: input.CannotSilentSelect,
		Candidates:         append([]cubebox.QueryCandidate(nil), input.Candidates...),
	}
	if len(currentGroup.Candidates) > 0 {
		queryContext = withAppendedQueryCandidateGroup(queryContext, currentGroup)
	}
	window := buildQueryEvidenceWindow(queryContext, input.Prompt, cubeboxQueryEvidenceWindowProjectionBudget{
		MaxConfirmedEntities:  queryContextMaxClarifierConfirmedEntities,
		MaxCandidateGroups:    queryContextMaxClarifierCandidateGroups,
		MaxCandidatesPerGroup: queryContextMaxClarifierCandidatesPerGroup,
		MaxDialogueTurns:      queryContextMaxClarifierDialogueTurns,
	})
	currentProjectedGroup := latestProjectedCandidateGroup(projectQueryCandidateGroups(
		queryContext.RecentCandidateGroups,
		queryContextMaxClarifierCandidateGroups,
		queryContextMaxClarifierCandidatesPerGroup,
	))
	return cubeboxQueryClarificationEnvelope{
		UserPrompt:          strings.TrimSpace(input.Prompt),
		QueryEvidenceWindow: window,
		Candidates:          append([]cubebox.QueryCandidate(nil), currentProjectedGroup.Candidates...),
		CandidateGroupID:    strings.TrimSpace(currentProjectedGroup.GroupID),
		ErrorCode:           strings.TrimSpace(input.ErrorCode),
		CandidateSource:     strings.TrimSpace(currentProjectedGroup.CandidateSource),
		CandidateCount:      currentProjectedGroup.CandidateCount,
		CannotSilentSelect:  currentProjectedGroup.CannotSilentSelect,
	}
}

func buildQueryNarrationEnvelope(input cubeboxQueryNarrationInput) cubeboxQueryNarrationEnvelope {
	return cubeboxQueryNarrationEnvelope{
		UserPrompt: strings.TrimSpace(input.Prompt),
		Results:    buildQueryNarrationResults(input.Results),
	}
}

func withAppendedQueryCandidateGroup(queryContext cubebox.QueryContext, group cubebox.QueryCandidateGroup) cubebox.QueryContext {
	if len(group.Candidates) == 0 {
		return queryContext
	}
	out := queryContext
	out.RecentCandidateGroups = append([]cubebox.QueryCandidateGroup(nil), queryContext.RecentCandidateGroups...)
	if group.CandidateCount <= 0 {
		group.CandidateCount = len(group.Candidates)
	}
	if groupID := strings.TrimSpace(group.GroupID); groupID != "" {
		for i := range out.RecentCandidateGroups {
			if strings.TrimSpace(out.RecentCandidateGroups[i].GroupID) == groupID {
				out.RecentCandidateGroups[i] = group
				out.RecentCandidates = append([]cubebox.QueryCandidate(nil), group.Candidates...)
				return out
			}
		}
	}
	out.RecentCandidateGroups = append(out.RecentCandidateGroups, group)
	if len(out.RecentCandidateGroups) > queryContextMaxCanonicalCandidateGroups {
		out.RecentCandidateGroups = out.RecentCandidateGroups[len(out.RecentCandidateGroups)-queryContextMaxCanonicalCandidateGroups:]
	}
	out.RecentCandidates = append([]cubebox.QueryCandidate(nil), group.Candidates...)
	return out
}

func latestProjectedCandidateGroup(groups []cubebox.QueryCandidateGroup) cubebox.QueryCandidateGroup {
	if len(groups) == 0 {
		return cubebox.QueryCandidateGroup{}
	}
	group := groups[len(groups)-1]
	group.Candidates = append([]cubebox.QueryCandidate(nil), group.Candidates...)
	return group
}

func projectQueryCandidateGroups(groups []cubebox.QueryCandidateGroup, maxGroups int, maxCandidates int) []cubebox.QueryCandidateGroup {
	if len(groups) == 0 {
		return nil
	}
	selected := append([]cubebox.QueryCandidateGroup(nil), groups...)
	if maxGroups > 0 && len(selected) > maxGroups {
		selected = selected[len(selected)-maxGroups:]
	}
	out := make([]cubebox.QueryCandidateGroup, 0, len(selected))
	for _, group := range selected {
		candidates := append([]cubebox.QueryCandidate(nil), group.Candidates...)
		if maxCandidates > 0 && len(candidates) > maxCandidates {
			candidates = candidates[:maxCandidates]
		}
		item := cubebox.QueryCandidateGroup{
			GroupID:            strings.TrimSpace(group.GroupID),
			CandidateSource:    strings.TrimSpace(group.CandidateSource),
			CandidateCount:     group.CandidateCount,
			CannotSilentSelect: group.CannotSilentSelect,
			Candidates:         candidates,
		}
		if item.CandidateCount <= 0 {
			item.CandidateCount = len(candidates)
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func withClarificationResume(queryContext cubebox.QueryContext, rawUserReply string) cubebox.QueryContext {
	out := queryContext
	if queryContext.ClarificationResume == nil {
		out.ClarificationResume = nil
		return out
	}
	out.ClarificationResume = cubebox.BuildQueryClarificationResume(queryContext, rawUserReply)
	return out
}

func buildQueryNarrationResults(results []cubebox.QueryNarrationResult) []cubeboxQueryNarrationResultView {
	out := make([]cubeboxQueryNarrationResultView, 0, len(results))
	for _, result := range results {
		payload := copyQueryNarrationData(result.Data)
		out = append(out, cubeboxQueryNarrationResultView{
			Domain: strings.TrimSpace(result.Domain),
			Data:   payload,
		})
	}
	return out
}

func copyQueryNarrationData(source map[string]any) map[string]any {
	if len(source) == 0 {
		return nil
	}
	target := make(map[string]any, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}
