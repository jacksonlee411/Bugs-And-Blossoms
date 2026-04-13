# DEV-PLAN-268 执行日志：Assistant 单一语义核与运行时瘦身收口

> 归档说明（2026-04-12）：本记录已自 `docs/dev-records/` 迁入 `docs/archive/dev-records/`，仅保留为历史执行证据，不再作为活体入口。

> 对应计划：`docs/archive/dev-plans/268-assistant-external-llm-semantic-core-and-runtime-thinning-implementation-plan.md`

## 1. 执行时间

- 首次实施窗口：2026-03-15 至 2026-03-16（CST）
- 收尾封板与证据补齐：2026-03-16（CST）
- 执行范围：`internal/server` Assistant 语义主链、`/reply` 投影链、检索状态收口、相关回归测试与计划文档封板

## 2. 本次落地内容

1. 单一外部模型语义核已成为 turn draft 主链。
- `prepareTurnDraft(...)` 统一先进入 `orchestrateSemanticTurn(...)`，再执行本地 route projection、dry-run、gate、confirm/commit 边界。
- 关键文件：
  - `internal/server/assistant_semantic_state.go`
  - `internal/server/assistant_intent_pipeline.go`
  - `internal/server/assistant_api.go`

2. `Context Assembler + Semantic Orchestrator` 已作为代码侧 SSOT 落地。
- 新增最小上下文装配与受控双回合编排：
  - `internal/server/assistant_semantic_contract.go`
  - `internal/server/assistant_context_assembler.go`
  - `internal/server/assistant_semantic_orchestrator.go`
- 语义主链冻结为：`用户输入 -> 最小上下文 -> 结构化语义 -> 本地只读检索 -> 必要时第二次语义收敛 -> dry-run / gate / confirm / commit`。

3. `/reply` 已退役独立 reply 模型主链。
- `/turns/:id:reply` 只返回：
  - 已存 semantic snapshot；
  - 本地 projection；
  - fail-closed fallback。
- `assistant_reply_nlg.go` 不再把 reply model 调用作为正式主链；旧 reply 错误映射不可达分支已删除。

4. 检索状态与错误归因已收口。
- `retrieval_results[]` 冻结为：
  - `not_requested`
  - `deferred_by_boundary`
  - `no_match`
  - `multiple_matches`
  - `single_match`
- dry-run / semantic state / 用户提示已区分“未执行”和“无匹配”，不再混成同一失败文案。

5. 本地职责已收缩为确定性边界。
- 保留：`ActionSpec / Readonly Resolver / Dry Run / Action Gate / Confirm Gate / Commit Adapter / Audit`。
- 降级或旁路：`plan_only`、本地 overlay/fallback 主判断链、clarification 主补槽链、reply 独立润色链。
- `confirm/commit` 继续保持 `plan_hash`、TTL、候选一致性、OCC、One Door 与 fail-closed。

## 3. 体验型验收与覆盖锚点

1. 轻微自然语言变体、缺字段续轮与显式事实优先。
- `TestAssistantIntentPipeline_Branches`
- `TestAssistantIntentPipeline_MergesPendingTurnContextForMissingFields`
- `TestAssistantIntentPipeline_FirstSemanticPassNoLongerSupplementsExplicitSlots`

2. 日期类确定性边界仍保留，本地不再扩写其它业务语义。
- `TestAssistantIntentPipeline_LocalTemporalHelpers`
- `TestAssistantClarificationPolicy_ParsingAndResumeCoverage`
- `TestAssistantIntentPipeline_FailsClosedWithoutSemanticRetryOnInvalidFirstPass`

3. 检索请求、检索结果与状态归一化收口。
- `TestAssistant268SemanticContractClosureCoverage`
- `TestAssistant268ModelGatewayRetrievalNormalizationCoverage`
- `TestAssistant268SemanticOrchestratorClosureCoverage`

4. reply 不跨域，且不再触发独立 reply 模型主链。
- `TestAssistantRenderTurnReplyMoreCoverage`
- `TestAssistantReplyNLGPipeline`
- `TestAssistantReplyFallbackText_HidesTechnicalSignals`
- `TestAssistant268ReplyRuntimeHelpers`

5. 单轮模型调用预算与 fail-closed 行为受控。
- `TestAssistantIntentPipeline_DoesNotRetryEvenIfSecondSemanticPassWouldSucceed`
- `TestAssistantIntentPipeline_UnsupportedActionFailsClosed`
- `TestAssistant268TurnActionAPIClosureCoverage`
- `TestAssistantClarificationGate_BlocksConfirmAndCommit`

## 4. 验证结果

1. 2026-03-16（CST）收尾复核：
- `go test ./internal/server/...`：通过。
- `go vet ./...`：通过。

2. 同一实施工作区前序验证结果：
- `go fmt ./...`：通过。
- `make check lint`：通过。
- `make test`：通过，覆盖率门禁命中 `100.00%`。

3. 文档门禁：
- `make check doc`：通过。

## 5. 上游回写清单

- `docs/archive/dev-plans/268-assistant-external-llm-semantic-core-and-runtime-thinning-implementation-plan.md`
- `docs/archive/dev-plans/267-assistant-dialog-rigidity-retrospective-and-architecture-correction-plan.md`
- `AGENTS.md`

## 6. 结论

- `DEV-PLAN-268` 的代码实施、测试收口与计划封板已完成。
- `DEV-PLAN-267` 提出的纠偏方向已被 `268` 完整承接并落地为运行时代码，不再停留在“架构反思”层。
- 本轮完成后，Assistant 主链的下一优先级不再是继续修补本地理解链，而是沿 `240` 主计划继续推进更高阶的执行编排与耐久化收口。
