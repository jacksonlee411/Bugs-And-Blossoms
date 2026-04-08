# DEV-PLAN-293：Assistant Runtime Proposal 降权与 Authoritative Gate 最小收口方案

**状态**: 修订中（2026-04-09 CST）

## 1. 背景
1. [X] `DEV-PLAN-240C` 已完成 ActionRegistry/ActionInterceptor/risk gate 单点化，`DEV-PLAN-268` 已完成单一外部语义核与 `prepareTurnDraft(...)` 主链接线，`DEV-PLAN-272` 已将七动作扩到正式运行态；当前主链骨架并不缺失。
2. [X] 当前剩余问题不在“缺少 catalog / scope / 新工作台”，而在于模型输出仍过早被投影成 turn 级业务真值：`assistant_model_gateway.go` 返回结果经过 `orchestrateSemanticTurn(...)` 与 `prepareTurnDraft(...)` 后，会在 authoritative gate 之前参与 `intent / plan / dry_run` 的正式生成。
3. [X] 现状虽然已有 registry、candidate retrieval、ActionInterceptor、confirm/commit PG 幂等与 `CommitAdapter` 闭环，但 draft 阶段的 authoritative 边界仍不够清晰，导致 runtime 漂移有机会进入 turn 快照。
4. [X] 本次修订不解决“能力扩面”，只解决“职责错位”：把 runtime 的职责降级为 proposal，把后端既有规则提升为唯一裁决源。
5. [X] `DEV-PLAN-300/301` 已冻结本仓测试分层方向：不得继续以 `*_coverage_test.go` / `*_gap_test.go` / `*_more_test.go` 作为默认维护载体；Assistant 方案必须同步改用“职责重组”口径。
6. [X] `DEV-PLAN-310` 已冻结时间语义上位原则：Assistant 中的时间上下文只能是从属条件，不得把 `as_of` / 查看日期提升为主锚点，不得让读时间回填写时间。

## 1.1 与 `DEV-PLAN-300/301/310` 的关系
1. [X] `DEV-PLAN-293` 只负责 Assistant runtime 真值边界收口，不重新定义测试体系，也不重新定义仓库级时间语义。
2. [X] 测试结构、分层与命名必须服从 `DEV-PLAN-300/301`；若 `293` 的测试章节与其冲突，以 `300/301` 为准。
3. [X] 时间上下文、`as_of`、current/history、`effective_date` 的边界必须服从 `DEV-PLAN-310`；若 `293` 中出现与 `310` 冲突的时间传播方式，以 `310` 为准。
4. [X] 若后续需要改变“proposal 只是建议”“时间上下文从属化”“测试责任下沉”这三条上位原则，必须先更新 `300/301/310`，再回写本计划。

## 2. 设计原则与范围冻结

### 2.1 核心原则
1. [X] 不新增产品概念，只重排现有职责。
2. [X] runtime-first 可以保留，但 backend 必须说了算。
3. [X] 任何业务真值只能来自 authoritative gate 之后的 turn state，而不能来自模型原始输出。
4. [X] 时间条件是从属输入，不是会话主锚点；Assistant 不得把读链路 `as_of` 语义提升为 runtime 主状态。
5. [X] 测试改造遵循“按职责重组”而不是“保留补洞文件继续累加”。

### 2.2 本计划范围（必须完成）
1. [ ] 在 Assistant 内部新增 `assistantRuntimeProposal` 语义层，明确模型输出只是 hint/proposal。
2. [ ] 在现有 `prepareTurnDraft(...)` 主链内新增一个薄的 authoritative gate，用于统一接收或拒绝 proposal。
3. [ ] 将 `plan / dry_run / version_tuple / plan_hash` 的正式生成时机后移到 gate 之后。
4. [ ] 明确 confirm / commit 只能依赖持久化后的 authoritative turn state，不再回看 runtime 原始输出。
5. [ ] 明确 Assistant 的读时间与写时间边界：只允许消费显式业务日期，不允许把 `as_of` / 查看日期 / candidate 历史切片回填写侧 `effective_date`。
6. [ ] 以最小文档和测试改动收口现有链路，不新增第二套前后端运行时模型。

### 2.3 本计划非目标（明确排除）
1. [X] 不引入 `surface catalog`。
2. [X] 不引入 `controlledOptions`。
3. [X] 不引入 `local choice scope`。
4. [X] 不引入新的前端工作台状态机。
5. [X] 不重做正式前端入口；`/app/assistant/librechat` 与现有对外 DTO 保持不变。
6. [X] 不把 Assistant 再拆成一组新模块。
7. [X] 不改现有 `confirmed / committed` 状态机。
8. [X] 不改 action registry、commit adapter、conversation/turn 持久化主合同。
9. [X] 不新增数据库表、列或迁移。

## 3. 现状问题与目标收口

### 3.1 当前问题陈述
1. [X] `assistant_model_gateway.go` 当前返回的 payload 语义过于接近 `assistantIntentSpec`，虽然名字是 semantic intent，但在实现上已接近业务真值。
2. [X] `prepareTurnDraft(...)` 当前顺序为：`semantic -> route projection -> dry_run -> clarification -> plan -> plan gate -> version_tuple -> annotate -> turn`；这意味着 plan/dry_run 生成早于 authoritative acceptance。
3. [X] `assistantBuildPlan(...)`、`assistantBuildDryRunWithRetrieval(...)`、`assistantAnnotateIntentPlan(...)` 当前都直接消费模型下游结果，因此 `plan_hash` 的基线仍混入 runtime 解释结果。
4. [X] confirm / commit 阶段虽已较强，但 draft 阶段 authoritative 边界不够清晰，使“runtime 只是建议”的纪律在代码层尚未完全冻结。

### 3.2 本次目标
1. [ ] 将模型输出固定为 proposal，而不是 intent spec / decision / business truth。
2. [ ] 将 authoritative gate 固定为 draft 阶段唯一正式裁决步骤。
3. [ ] 将 turn 上的 `Intent / Candidates / ResolvedCandidateID / Plan / DryRun` 冻结为 authoritative snapshot。
4. [ ] 保证 `CommitAdapter`、receipt/task、confirm/commit、error mapping 对外行为不回退。
5. [ ] 保证时间条件只作为从属输入存在：Assistant 不新增 `timeAnchor` 类主锚点，也不通过缺省 today 或读链视角补齐写侧日期。

## 4. 目标架构（最小改造版）

### 4.1 新的内部对象边界
1. [ ] 新增 `assistantRuntimeProposal`，只表达 hint，不表达决策：

```go
type assistantRuntimeProposal struct {
    ActionHint          string
    RouteKindHint       string
    IntentIDHint        string
    RouteCatalogVersion string
    ParentRefText       string
    EntityName          string
    EffectiveDate       string
    OrgCode             string
    TargetEffectiveDate string
    NewName             string
    NewParentRefText    string
    SelectedCandidateID string
    Readiness           string
    GoalSummary         string
    UserVisibleReply    string
    NextQuestion        string
    RetrievalNeeded     bool
    RetrievalRequests   []assistantSemanticRetrievalRequest
    ConfidenceNote      string
}
```

补充约束：
1. [ ] `EffectiveDate` / `TargetEffectiveDate` 仅表示用户显式表达或语义核明确提取到的写侧业务日期，不得由 `as_of`、current/history 视角、candidate 元数据或隐式 today 派生。
2. [ ] proposal 中若未来需要表达读链路时间条件，必须与写侧业务日期分字段建模，且仍作为从属条件，不得变成会话主锚点。

2. [ ] 新增 `assistantAuthoritativeDecision`，只表达 authoritative acceptance 结果：

```go
type assistantAuthoritativeDecision struct {
    Accepted            bool
    Intent              assistantIntentSpec
    ActionSpec          assistantActionSpec
    RouteDecision       assistantIntentRouteDecision
    Candidates          []assistantCandidate
    ResolvedCandidate   *assistantCandidate
    ResolvedCandidateID string
    SelectedCandidateID string
    Clarification       *assistantClarificationDecision
    FailClosedCode      string
}
```

3. [ ] `assistantIntentSpec` 继续保留，但语义改为：只在 authoritative gate 接受 proposal 后才成为 turn 正式业务快照。

### 4.2 主链顺序冻结
1. [ ] 新顺序固定为：

```text
用户输入
-> runtime proposal
-> route decision
-> authoritative candidate resolve
-> assistantAcceptProposal(...)
-> authoritative intent
-> clarification
-> plan / dry_run / version_tuple / plan_hash
-> confirm
-> commit
```

2. [ ] 旧顺序中“proposal 先生成正式 dry_run/plan”的做法必须删除。
3. [ ] `assistantAcceptProposal(...)` 之前，任何对象都不得被视为 turn 真值。

### 4.3 Authoritative Gate 最小职责
1. [ ] gate 只做五件事：
   - action 是否存在于 `assistant_action_registry.go`
   - 当前租户/角色是否允许
   - candidate 是否存在于 authoritative retrieval 结果
   - 缺字段时是否进入 clarification
   - 不满足条件时 fail-closed
2. [ ] gate 不引入 catalog/family/surface 新层级，只复用现有 registry、ActionInterceptor、candidate retrieval、clarification builder。
3. [ ] gate 不直接做 commit，不直接写 HTTP；它只产出 authoritative decision。
4. [ ] gate 若发现 proposal 中的时间字段来源不明、来自读视角回填、或依赖隐式 today，必须 fail-closed 或进入 clarification，不得继续产出 authoritative intent。

## 5. 代码落点与最小切刀

### 5.1 `internal/server/assistant_model_gateway.go`
1. [ ] 将 `assistantSemanticIntentPayload` 的业务语义从“semantic intent payload”收敛为“runtime proposal payload”。
2. [ ] 将 `assistantResolveIntentResult` 改为 proposal 语义结果；命名可调整为 `assistantResolveProposalResult`，若保留旧名，则字段语义必须在注释和调用处同步降级。
3. [ ] `intentSpec()` 应改为 `proposalIntentSpec()` 或 `proposal()`；不得继续把 gateway 输出直接视为 authoritative `assistantIntentSpec`。
4. [ ] provider/model/revision 元信息保留，不改变 deterministic/openai provider 兼容性。

### 5.2 `internal/server/assistant_semantic_contract.go`
1. [ ] `assistantConversationSemanticState` 继续保留，但其 `Slots` 来源改为 proposal 投影，而不是 authoritative intent。
2. [ ] `assistantSemanticStateFromResolved(...)` / `assistantSyncResolvedSemanticResult(...)` 必须明确：这里只同步 proposal 语义，不宣布业务真值。
3. [ ] `assistantModelSemanticStateInvalid(...)` 继续承担 strict decode / route kind / readiness 基础校验，但不承担最终 action acceptance。

### 5.3 `internal/server/assistant_semantic_state.go`
1. [ ] `prepareTurnDraft(...)` 内新增薄 helper：`assistantAcceptProposal(...)`。
2. [ ] `assistantBuildPlan(...)`、`assistantBuildDryRunWithRetrieval(...)`、`assistantCompileIntentToPlansWithSpec(...)`、`refreshTurnVersionTuple(...)`、`assistantAnnotateIntentPlan(...)` 必须后移到 gate 接受 proposal 之后。
3. [ ] `assistantBuildIntentRouteDecisionFn(...)` 继续保留，route decision 仍是主链的一部分，但它只能与 proposal 一起作为 gate 输入。
4. [ ] `assistantBuildClarificationDecisionFn(...)` 继续保留，但 clarification 必须基于 authoritative 缺口，而不能让 runtime hint 直接冒充已确认 action。
5. [ ] 若 `assistantAcceptProposal(...)` 中存在 proposal 归一化、时间字段合法化、plan/hash 输入规范化等纯规则，应优先抽成可直接测试的小 helper，避免全部耦合在 `internal/server` 巨型流程测试里。

### 5.4 `internal/server/assistant_persistence.go`
1. [ ] `applyConfirmTurn(...)`、`prepareCommitTurn(...)`、`applyCommitTurn(...)` 必须明确只消费持久化后的 authoritative turn state。
2. [ ] `CommitAdapter` 只能从以下字段取值：
   - `turn.Intent`
   - `turn.Candidates`
   - `turn.ResolvedCandidateID`
   - `turn.Plan`
   - `turn.DryRun`
3. [ ] 不允许从 runtime proposal/raw semantic payload 重新补业务字段。
4. [ ] 不允许从历史查看上下文、candidate `as_of`、current/history 页面状态或其他读链条件回填写侧 `effective_date` / `target_effective_date`。

### 5.5 `internal/server/assistant_api.go`
1. [ ] 对外 HTTP 合同保持不变。
2. [ ] 若内部错误映射因 gate 后移需要调整，必须保持既有错误码主集合不漂移。
3. [ ] `assistantBuildPlan(...)` 的文案含义应更新为 authoritative plan，而不是模型拟定计划。
4. [ ] 若对外 DTO 中保留历史字段名或时间字段，语义仍必须与 `DEV-PLAN-310` 对齐，不得让 API 调用方误以为“查看日期”可直接驱动写侧日期。

### 5.6 `AGENTS.md`
1. [ ] 在 Doc Map 中新增 `DEV-PLAN-293` 入口链接。
2. [ ] 不复制本计划内容，只维护文档可发现性。

## 6. 实施步骤
1. [ ] 第一步：在 `assistant_model_gateway.go` 定义 proposal 语义类型，并让 strict decode 产物回到 proposal 层。
2. [ ] 第二步：在 `assistant_semantic_contract.go` / `assistant_semantic_state.go` 中把 semantic state 明确降级为 proposal 视图。
3. [ ] 第三步：在 `prepareTurnDraft(...)` 中加入 `assistantAcceptProposal(...)`，并后移 `plan / dry_run / version_tuple / annotate`。
4. [ ] 第四步：显式切断 Assistant 中的读写时间串线，补齐“只信显式业务日期、拒绝 `as_of` 回填”的规则。
5. [ ] 第五步：把 proposal 归一化、authoritative 接受、时间字段合法化等可下沉规则拆成小职责 helper，并按 `DEV-PLAN-300/301` 组织测试。
6. [ ] 第六步：在 `assistant_persistence.go` 补充“commit 只信 authoritative turn state”的纪律与测试。
7. [ ] 第七步：补齐回归测试与 live E2E 验证，确认对外合同不变。

## 7. 测试与覆盖率

### 7.1 测试原则（以 `DEV-PLAN-300/301` 为准）
1. [ ] 不再把“继续维护 `*_coverage_test.go` / `*_gap_test.go` / `*_more_test.go`”作为本计划验收条件。
2. [ ] proposal 归一化、authoritative gate 的纯规则、时间字段边界校验等逻辑，优先抽到更小职责单元并直接测试。
3. [ ] `internal/server` 侧测试只保留适配层与组合层职责：主链编排、持久化状态、错误映射、confirm/commit 边界。
4. [ ] E2E 继续承担真实流程验收，但不作为补洞型单测扩张的理由。

### 7.2 新增或改写的关键断言
1. [ ] 模型返回未知 action hint 时，draft 阶段 fail-closed，不生成 authoritative turn intent。
2. [ ] 模型返回 candidate id 但不在 authoritative retrieval 结果中时，不得直接接受。
3. [ ] 缺字段时只能进入 clarification/missing-fields，不得生成可提交 authoritative dry_run。
4. [ ] gate accepted 后才允许生成 authoritative `plan / dry_run / plan_hash / version_tuple`。
5. [ ] commit 只依赖 persisted authoritative turn state；即使模拟 runtime 原始输出漂移，提交结果仍不变。
6. [ ] proposal 中缺失业务日期时，只能走 clarification / fail-closed；不得从 `as_of`、current/history 或隐式 today 自动补齐。
7. [ ] candidate 或 retrieval 中即使带有 `as_of`/历史切片日期，也不得被写回 authoritative intent 的 `effective_date` / `target_effective_date`。
8. [ ] 读时间变化不应改变 confirm/commit 的写入参数；只有用户显式业务日期变化才允许改变提交结果。

### 7.3 测试重组方向
1. [ ] 只为覆盖率存在、但断言“模型输出直接等于 turn 真值”的用例，应改写为“proposal -> gate -> authoritative turn”。
2. [ ] 只为覆盖率存在、但把 runtime `Readiness/UserVisibleReply/SelectedCandidateID` 当作 turn 真值的重复覆盖，应删除、合并或改写为 proposal 素材断言。
3. [ ] 若现有测试主要验证业务规则而非 HTTP/持久化适配，应优先下沉到更小职责 helper，而不是继续留在 `internal/server` 补洞文件中。
4. [ ] 不允许通过降低 coverage 阈值、扩大排除项来替代上述重组。

### 7.4 必保留的 E2E
1. [ ] `e2e/tests/tp288-librechat-real-entry-evidence.spec.js`
2. [ ] `e2e/tests/tp288b-librechat-live-task-receipt-contract.spec.js`
3. [ ] `e2e/tests/tp290b-librechat-live-intent-action-chain.spec.js`
4. [ ] `e2e/tests/tp290b-librechat-live-intent-action-negative.spec.js`

E2E 验收重点：
1. [ ] receipt/task/refresh 合同保持不变。
2. [ ] confirm/commit 仍由后端 authoritative turn state 驱动。
3. [ ] `plan_only`、bad candidate、commit without confirm 继续 fail-closed。

## 8. 验收标准
1. [ ] 任何路径都不能再把 runtime proposal 直接落成 turn 真值。
2. [ ] `plan / dry_run / version_tuple / plan_hash` 只能由 authoritative gate 接受后的输入生成。
3. [ ] `CommitAdapter` 不存在绕过 authoritative turn state 的旁路。
4. [ ] 不新增新的产品概念、新的前端状态机或新的正式入口。
5. [ ] `tp288 / tp288b / tp290b` 主链不回退。
6. [ ] Assistant 中不存在“读链路时间 -> 写侧业务日期”的隐式传染。
7. [ ] 测试结构与命名不再把补洞文件本身作为正式交付目标，且新增测试符合 `DEV-PLAN-300/301` 分层方向。

## 9. 停止线
1. [ ] 若任何路径仍可把 runtime proposal 直接写入 `turn.Intent` 作为正式业务真值，本计划失败。
2. [ ] 若 `CommitAdapter` 仍可通过 runtime 原始字段补齐提交参数，本计划失败。
3. [ ] 若为了实现本计划而引入新的 product surface、local scope、工作台状态机或第二套 Assistant 架构，本计划失败。
4. [ ] 若 `plan_hash` 仍混入未被 authoritative gate 接受的 runtime 原始字段，本计划失败。
5. [ ] 若任何路径仍允许把 `as_of`、查看日期、current/history 页面状态、candidate 历史切片或隐式 today 写入 `effective_date` / `target_effective_date`，本计划失败。
6. [ ] 若本计划继续以新增或保留 `*_coverage_test.go` / `*_gap_test.go` / `*_more_test.go` 作为主要实施方式，而不是按职责重组测试，本计划失败。

## 10. 交付物
1. [ ] 计划文档：`docs/dev-plans/293-assistant-runtime-proposal-authoritative-gate-minimal-refactor-plan.md`
2. [ ] 文档入口更新：`AGENTS.md`
3. [ ] 后续实现涉及的主要代码路径：
   - `internal/server/assistant_model_gateway.go`
   - `internal/server/assistant_semantic_contract.go`
   - `internal/server/assistant_semantic_state.go`
   - `internal/server/assistant_persistence.go`
   - `internal/server/assistant_api.go`

## 11. 关联文档
1. `docs/dev-plans/240c-assistant-action-interceptor-and-risk-gate-plan.md`
2. `docs/dev-plans/268-assistant-external-llm-semantic-core-and-runtime-thinning-implementation-plan.md`
3. `docs/dev-plans/272-assistant-orgunit-seven-actions-expansion-plan.md`
4. `docs/dev-plans/240d-assistant-durable-execution-and-manual-takeover-plan.md`
5. `docs/dev-plans/012-ci-quality-gates.md`
6. `docs/dev-plans/300-test-system-investigation-report.md`
7. `docs/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md`
8. `docs/dev-plans/310-project-wide-view-as-of-semantics-review-and-minimal-convergence-plan.md`
