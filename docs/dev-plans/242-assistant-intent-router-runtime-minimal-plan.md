# DEV-PLAN-242：Assistant Intent Router 运行时最小落地计划

**状态**: 规划中（2026-03-11 CST；本次修订将 `242` 细化为“可按文件直接实施”的运行时蓝图，承接 `240E/241/246`，并显式收口当前 `plan_only/route_kind` 半成品状态）

## 1. 背景与定位
1. [ ] `240E` 已冻结：Assistant 必须先经过 `understand / route`，再决定是否进入正式业务动作链；`241` 已把知识资产、最小 `Intent Route Catalog`、`plan_context_v1` 和版本快照接入运行时，但尚未形成**正式的路由裁决对象**。
2. [ ] 当前代码已经出现 `route` 相关地基，但仍未达到“可作为正式运行时 SSOT”的程度：
   - [ ] `internal/server/assistant_knowledge_runtime.go` 已有 `route_kind` 常量、`intent_route_catalog.json`、`routeIntent(...)` 与 `buildPlanContextV1(...)`；
   - [ ] `internal/server/assistant_api.go` 与 `internal/server/assistant_persistence.go` 已在 `createTurn / createTurnPG` 中调用 `knowledgeRuntime.routeIntent(...)`；
   - [ ] `assistantIntentSpec` 与 `assistantPlanSummary` 已出现 `intent_id / route_kind / route_catalog_version / knowledge_snapshot_digest` 等字段。
3. [ ] 但当前仍存在三类关键缺口，导致 `242` 还不能视为完成：
   - [ ] **缺少正式路由裁决 DTO**：当前 `route` 结果只是散落在 `assistantIntentSpec` 与 `plan` 字段里，没有独立的 `route decision` 审计对象；
   - [ ] **阶段语义仍被 `plan_only` 误导**：`assistantTurnPendingDraftSummary(...)` 与 `assistantTurnPhase(...)` 仍主要按“是否有 dry-run explain / 是否缺字段 / 是否候选确认”推导，`knowledge_qa/chitchat/uncertain` 仍可能落成可确认态；
   - [ ] **确认/提交门禁没有显式读 route 决策**：当前 `confirm/commit` 仍主要围绕 `intent.action + action spec + required checks` 运转，尚未把“非动作 / 待澄清”作为一级阻断条件。
4. [ ] 本计划的定位冻结为：在不重写 `reply` 主链、不提前实现 `243` 多轮追问策略的前提下，把当前已有 `route` 地基收敛成**正式、可审计、fail-closed、可被后续 `243/245` 直接复用**的最小运行时主链。

## 2. 当前实现基线、根因与改造落点

### 2.1 当前实际调用链
1. [ ] 当前 turn 创建主链可概括为：
   - [ ] `resolveIntent(...)`：模型解析 + 本地抽取补强；
   - [ ] `assistantMergeIntentWithPendingTurn(...)`：与待处理 turn 合并；
   - [ ] `knowledgeRuntime.routeIntent(...)`：基于 catalog 给 `intent` 写入 `intent_id / route_kind`；
   - [ ] `assistantIntentValidationErrors(...)` / `resolveCandidates(...)`；
   - [ ] `lookupActionSpec(...)` / `assistantBuildPlan(...)` / `assistantEvaluateActionGate(...)`；
   - [ ] `buildPlanContextV1(...)` / `assistantApplyPlanContextV1(...)`；
   - [ ] `assistantRefreshTurnDerivedFields(...)` 推导 `missing_fields / pending_draft_summary / phase`。
2. [ ] 这条链路说明 `route` 已经插入 create-turn，但还只是**弱裁决**：
   - [ ] 它没有独立 DTO；
   - [ ] 它没有自己的 reason codes / confidence band；
   - [ ] 它没有自己的持久化列；
   - [ ] 它没有成为 `phase / confirm / commit` 的正式输入。

### 2.2 当前直接风险
1. [ ] `knowledge_qa/chitchat/uncertain` 虽已可写入 `intent.route_kind`，但若仍沿用 `plan_only` 动作 spec + `PendingDraftSummary = DryRun.Explain` 的现状，就会把“非动作说明”误包装成“可确认草稿”。
2. [ ] `route` 结果现在主要靠 `assistantIntentSpec` 投影保存；一旦后续需要追加 `clarification_required / reason_codes / candidate_action_ids / decision_source`，会继续把 `intent` 膨胀成混合体。
3. [ ] `createTurn` 内存路径和 `createTurnPG` 持久化路径虽然都已接线 `routeIntent(...)`，但如果不把 route 变成单独 builder，两条路径后续很容易再次漂移。
4. [ ] 若继续沿用“`action` 注册表是否存在”来隐式代表“是否应进入动作链”，自然语言理解仍会在大量场景下退化成 `assistant_intent_unsupported`。

### 2.3 根因到文件的映射
1. [ ] `internal/server/assistant_intent_pipeline.go`
   - [ ] 当前负责“理解”但不输出正式 route decision；
   - [ ] `assistantShouldUpgradeIntentFromLocalFacts(...)` 的升级信号尚未被结构化记录。
2. [ ] `internal/server/assistant_api.go`
   - [ ] 内存路径已在 `createTurn(...)` 中调用 `knowledgeRuntime.routeIntent(...)`，但后续仍直接走 action spec 与 dry-run 派生逻辑；
   - [ ] route 结果尚未进入独立 `turn.RouteDecision`。
3. [ ] `internal/server/assistant_persistence.go`
   - [ ] PG 路径与内存路径重复一套 create-turn 逻辑；
   - [ ] `iam.assistant_turns` 现无 `route_decision_json`，无法稳定审计 route 真相。
4. [ ] `internal/server/assistant_phase_snapshot.go`
   - [ ] `assistantTurnPhase(...)` 与 `assistantTurnPendingDraftSummary(...)` 没有 route-aware 判断；
   - [ ] 非动作 turn 仍可能因为 `DryRun.Explain` 非空而被推入 `await_commit_confirm`。
5. [ ] `internal/server/assistant_action_interceptor.go`
   - [ ] 当前 gate 只看 action spec / authz / risk / required checks；
   - [ ] 缺少“非动作 / 待澄清 route 禁止 confirm/commit”的显式检查。
6. [ ] `internal/server/assistant_knowledge_runtime.go`
   - [ ] 已有最小 catalog 与 pack runtime；
   - [ ] 但 `routeIntent(...)` 目前更像 projection helper，不足以承担正式 route 决策全量职责。

## 3. 目标与非目标

### 3.1 核心目标
1. [ ] 在 `resolveIntent` 与 `plan/confirm/commit` 之间建立正式 `Intent Router` 运行时节点，产出独立 `assistantIntentRouteDecision`。
2. [ ] 冻结最小 route 决策字段：`route_kind / intent_id / candidate_action_ids / confidence_band / clarification_required / reason_codes`。
3. [ ] 让 `phase`、`pending_draft_summary`、`confirm/commit` 门禁显式消费 `route decision`，而不是继续侧面依赖 `plan_only`。
4. [ ] 对 `knowledge_qa / chitchat / uncertain` 明确实现**不进入 confirm/commit** 的 fail-closed 运行时保护。
5. [ ] 对 `business_action + clarification_required=true` 输出正式 route 裁决，并把后续追问责任明确留给 `243`。
6. [ ] 让 turn 审计链能稳定记录本次 route 所用的 `knowledge_snapshot_digest / route_catalog_version / resolver_contract_version` 以及 route decision 本体。
7. [ ] 冻结边界：`route_decision.clarification_required` 只表示 route 层建议进入澄清；它不是 turn 上 open clarification 的事实主源，`243` 上线后该事实由 `assistantClarificationDecision` 承接。

### 3.2 非目标
1. [ ] 不在本计划中实现多轮澄清、轮次上限与退出策略；该部分由 `243` 承接。
2. [ ] 不在本计划中统一用户可见语气、模板与 `reply_nlg` 主链；该部分由 `245` 承接。
3. [ ] 不在本计划中要求模型直接返回 `confidence_band` 或 `reason_codes`；最小实现允许在后端基于现有模型输出 + catalog + 本地规则生成 route decision。
4. [ ] 不新增第二条执行旁路；所有业务写仍只能走既有 `confirm / commit / task / adapter` 主链。
5. [ ] 不新建表；若需要持久化 route decision，只允许在现有 `iam.assistant_turns` 上加列，不得引入并行事实表。
6. [ ] 不在本计划中引入新的 phase 枚举；`243` 上线前，待澄清场景仍以 `idle + route_decision.clarification_required=true` 表示，而不是发明临时 phase。

## 4. 运行时不变量（冻结）
1. [ ] `assistantIntentRouteDecision` 是 `242` 之后“是否允许进入动作链”的运行时 SSOT；`assistantIntentSpec.RouteKind` 只保留为投影字段，供兼容读取与调试使用。
1.1 [ ] 该 SSOT 的作用域只限于 route 层：用于判定是否允许进入动作链、以及为后续 `243` 提供受控输入；它不替代 `243` 的 turn 级 `Clarification` 事实。
2. [ ] `assistantIntentSpec.Action` 不再独自代表“允许执行某动作”；它只能作为 `business_action` route 分支中的候选输入事实之一。
3. [ ] `knowledge_qa / chitchat / uncertain` 不得进入：
   - [ ] `assistantTurnPendingDraftSummary(...)` 的可确认摘要分支；
   - [ ] `assistantEvaluateActionGate(... StageConfirm/StageCommit ...)`；
   - [ ] `applyConfirmTurn(...) / prepareCommitTurn(...) / submitCommitTaskPG(...)`。
4. [ ] `business_action + clarification_required=true` 不得进入 `confirm/commit`，但必须保留足够的 route 决策审计信息，供 `243` 继续追问。
5. [ ] route 目录缺失、route 结果非法、route 与 action spec 冲突时必须 fail-closed，不能偷偷回退为“当成 `plan_only` 正常确认”。
6. [ ] `knowledge_snapshot_digest` 必须可稳定映射到本次 route 所依赖的 `route_catalog / resolver_contract / context_template` 版本集合，避免后续 `243` 无法复核。
7. [ ] create-turn 的成功不代表可提交；“是否已创建 turn”和“是否允许进入动作链”必须显式区分。

### 4.1 行为矩阵（冻结）
| route_kind | clarification_required | create-turn | phase | pending_draft_summary | confirm | commit |
| --- | --- | --- | --- | --- | --- | --- |
| `business_action` | `false` | 成功 | 依现有字段缺失/候选/确认规则推导 | 允许 | 允许 | 允许 |
| `business_action` | `true` | 成功 | `idle` | 空 | 阻断 | 阻断 |
| `knowledge_qa` | `false` | 成功 | `idle` | 空 | 阻断 | 阻断 |
| `chitchat` | `false` | 成功 | `idle` | 空 | 阻断 | 阻断 |
| `uncertain` | `true` | 成功 | `idle` | 空 | 阻断 | 阻断 |
| route 运行时非法 | 任意 | 失败 | 无 | 无 | 无 | 无 |

## 5. 运行时契约设计（最小冻结）

### 5.1 新增 DTO：`assistantIntentRouteDecision`
1. [ ] 在 `internal/server/assistant_intent_router.go` 新增正式 DTO，最小字段冻结为：
   - [ ] `RouteKind string`：`business_action / knowledge_qa / chitchat / uncertain`；
   - [ ] `IntentID string`；
   - [ ] `CandidateActionIDs []string`；
   - [ ] `ConfidenceBand string`：`high / medium / low`；
   - [ ] `ClarificationRequired bool`；
   - [ ] `ReasonCodes []string`；
   - [ ] `RouteCatalogVersion string`；
   - [ ] `KnowledgeSnapshotDigest string`；
   - [ ] `ResolverContractVersion string`；
   - [ ] `DecisionSource string`：首期允许 `knowledge_runtime_v1`。
2. [ ] `assistantTurn` 增加字段：`RouteDecision assistantIntentRouteDecision 'json:"route_decision,omitempty"'`。
3. [ ] `assistantResolveIntentResult` 本期不强制扩展；若后续需要 provider 级原始信号，可在 `244/245` 或更后续计划中追加。

### 5.2 枚举与 reason codes（首期冻结）
1. [ ] `ConfidenceBand` 只允许：`high / medium / low`。
2. [ ] `ReasonCodes` 首期至少覆盖：
   - [ ] `route_business_action_registered`；
   - [ ] `route_non_business_catalog_match`；
   - [ ] `route_uncertain_no_match`；
   - [ ] `route_local_intent_upgrade`；
   - [ ] `route_model_plan_only`；
   - [ ] `route_confidence_below_min`；
   - [ ] `route_action_unregistered`；
   - [ ] `route_non_business_blocked`；
   - [ ] `route_clarification_required`；
   - [ ] `route_catalog_version_missing`；
   - [ ] `route_decision_missing`；
   - [ ] `route_candidate_action_conflict`。
3. [ ] 本计划不在 `ReasonCodes` 中塞入用户可见文案；用户文案仍由后续 `245` 统一生成。

### 5.3 与现有字段的关系
1. [ ] `assistantIntentSpec.IntentID / RouteKind / RouteCatalogVersion` 保留，但它们是 `RouteDecision` 的冗余投影，写入时必须从 `RouteDecision` 回填，不允许双向各自演化。
2. [ ] `assistantPlanSummary.RouteCatalogVersion / KnowledgeSnapshotDigest / ResolverContractVersion / ContextTemplateVersion / ReplyGuidanceVersion` 继续保留，因为它们属于计划快照审计，不与 `RouteDecision` 冲突。
3. [ ] `DryRun.ValidationErrors` 仍服务字段缺失与动作前校验，不承担 route 决策真相；route 结果进入 `DryRun.Explain` 仅作为临时表现层投影，不可反向当作 route SSOT。

### 5.4 `route_decision_json` 参考结构（冻结）
1. [ ] 首期持久化结构参考如下：
```json
{
  "route_kind": "business_action",
  "intent_id": "org.orgunit_create",
  "candidate_action_ids": ["create_orgunit"],
  "confidence_band": "medium",
  "clarification_required": true,
  "reason_codes": [
    "route_local_intent_upgrade",
    "route_clarification_required"
  ],
  "route_catalog_version": "2026-03-11.v1",
  "knowledge_snapshot_digest": "sha256:...",
  "resolver_contract_version": "resolver_contract_v1",
  "decision_source": "knowledge_runtime_v1"
}
```
2. [ ] `reason_codes` 必须稳定排序后再持久化，避免同一逻辑因为顺序波动造成审计 diff 噪声。
3. [ ] `candidate_action_ids` 首期长度允许为 `0` 或 `1`；若后续扩展为多候选动作，不得破坏本 JSON 契约。

### 5.5 建议函数签名（冻结）
1. [ ] 推荐新增类型与函数签名如下：
```go
type assistantIntentRouteDecision struct {
    RouteKind              string   `json:"route_kind,omitempty"`
    IntentID               string   `json:"intent_id,omitempty"`
    CandidateActionIDs     []string `json:"candidate_action_ids,omitempty"`
    ConfidenceBand         string   `json:"confidence_band,omitempty"`
    ClarificationRequired  bool     `json:"clarification_required,omitempty"`
    ReasonCodes            []string `json:"reason_codes,omitempty"`
    RouteCatalogVersion    string   `json:"route_catalog_version,omitempty"`
    KnowledgeSnapshotDigest string  `json:"knowledge_snapshot_digest,omitempty"`
    ResolverContractVersion string  `json:"resolver_contract_version,omitempty"`
    DecisionSource         string   `json:"decision_source,omitempty"`
}

func assistantBuildIntentRouteDecision(
    userInput string,
    resolved assistantResolveIntentResult,
    mergedIntent assistantIntentSpec,
    runtime *assistantKnowledgeRuntime,
    pendingTurn *assistantTurn,
) (assistantIntentRouteDecision, error)

func assistantProjectIntentRouteDecision(
    mergedIntent assistantIntentSpec,
    decision assistantIntentRouteDecision,
) assistantIntentSpec

func assistantValidateIntentRouteDecision(decision assistantIntentRouteDecision) error

func assistantCheckRouteDecision(input assistantActionGateInput) assistantActionGateDecision

func assistantTurnRouteKind(turn *assistantTurn) string

func assistantTurnRouteClarificationRequired(turn *assistantTurn) bool

func assistantTurnActionChainAllowed(turn *assistantTurn) bool
```
2. [ ] `assistantBuildIntentRouteDecision(...)` 的唯一职责是输出正式 `RouteDecision`；兼容字段回填改由单独的 `assistantProjectIntentRouteDecision(...)` 承担，避免 Router 再次退化成“顺手改 DTO”的混合边界。
3. [ ] `assistantValidateIntentRouteDecision(...)` 推荐返回仓内 error 变量，而不是 `fmt.Errorf` 拼接字符串；这样可直接进入 API 映射和 idempotency 恢复分支。
4. [ ] `assistantTurnActionChainAllowed(...)` 应只回答“是否允许进入确认/提交链”，不要顺便计算 phase、回复文案或字段缺失。
5. [ ] `assistantCheckRouteDecision(...)` 应只在 `StageConfirm / StageCommit` 强阻断；`StagePlan` 允许非动作 create-turn 成功，但不允许非法 decision 混过。
6. [ ] `pendingTurn` 首期默认为可选上下文输入；若最终实现未证明它保护了明确不变量，应保持未使用或删除，避免提前把 `243` 的多轮澄清状态耦合进 `242`。

## 6. 路由算法与 builder（冻结）

### 6.1 新增 builder：`assistantBuildIntentRouteDecision(...)`
1. [ ] 在 `internal/server/assistant_intent_router.go` 新增纯函数 builder，输入最小集合为：
   - [ ] `userInput string`；
   - [ ] `resolved assistantResolveIntentResult`；
   - [ ] `mergedIntent assistantIntentSpec`；
   - [ ] `knowledgeRuntime *assistantKnowledgeRuntime`；
   - [ ] `pendingTurn *assistantTurn`（可选；仅在能证明其用于保护明确 route 不变量时启用，不得修改 route 真相）。
2. [ ] builder 输出：
   - [ ] `assistantIntentRouteDecision`；
   - [ ] 错误仅用于硬失败（catalog/runtime/contract 冲突）。
3. [ ] builder 必须是**纯计算函数**：
   - [ ] 不做 DB 查询；
   - [ ] 不写日志 side effect；
   - [ ] 不直接生成回复；
   - [ ] 不直接推进 phase。
4. [ ] 兼容投影必须通过独立 helper 完成：`assistantProjectIntentRouteDecision(...)` 统一从 `decision` 回填 `intent.intent_id / route_kind / route_catalog_version`，但该 helper 不是 Router 本体。

### 6.2 首期决策规则
1. [ ] 若 `mergedIntent.Action` 命中已注册 action spec，则：
   - [ ] `route_kind=business_action`；
   - [ ] `candidate_action_ids=[action]`；
   - [ ] `intent_id` 优先取 catalog 中该 action 对应条目，否则回退 `action.<action_id>`；
   - [ ] `reason_codes` 至少追加 `route_business_action_registered`。
2. [ ] 若 action 为空或为 `plan_only`，则必须由 `Intent Route Catalog` 判定非动作或 uncertain；不得仅靠 `plan_only` 自然流入 confirm。
3. [ ] 若命中 catalog 的 `knowledge_qa / chitchat` 条目，则：
   - [ ] `candidate_action_ids=[]`；
   - [ ] `clarification_required=false`；
   - [ ] `reason_codes` 追加 `route_non_business_catalog_match`。
4. [ ] 若既不是注册动作，也没有命中非动作 catalog，则：
   - [ ] `route_kind=uncertain`；
   - [ ] `candidate_action_ids=[]`；
   - [ ] `clarification_required=true`；
   - [ ] `reason_codes` 追加 `route_uncertain_no_match` 与 `route_clarification_required`。
5. [ ] 若模型返回 `plan_only` 但本地抽取升级为业务动作（现有 `assistantShouldUpgradeIntentFromLocalFacts(...)` 分支），则：
   - [ ] `reason_codes` 追加 `route_local_intent_upgrade`；
   - [ ] `confidence_band` 至少降为 `medium`，不可直接宣称 `high`。
6. [ ] 若 route 判定为 `business_action` 但 action 未注册，则：
   - [ ] 返回硬错误 `ai_route_action_conflict` 或 `ai_route_runtime_invalid`；
   - [ ] 不得偷偷回退成 `plan_only` 成功 turn。

### 6.3 首期置信度规则
1. [ ] 首期不要求模型提供置信度数值；改由后端生成 `confidence_band`：
   - [ ] `high`：已注册业务动作，且不是 `local upgrade`，且 route 与 action spec 无冲突；
   - [ ] `medium`：业务动作，但来自 `local upgrade`、弱匹配或有显著歧义信号；
   - [ ] `low`：未形成稳定动作判定，或仅命中弱非动作/uncertain 信号。
2. [ ] `clarification_required` 首期判定规则：
   - [ ] `route_kind=business_action` 且 `confidence_band!=high` → `true`；
   - [ ] `route_kind=uncertain` → `true`；
   - [ ] `route_kind=knowledge_qa/chitchat` → `false`。
3. [ ] `243` 上线前，本计划不额外引入“候选动作多选”算法；因此首期 `candidate_action_ids` 通常为 `0` 或 `1` 个元素。

### 6.4 伪代码（冻结）
```text
buildRouteDecision(userInput, resolved, mergedIntent, runtime, pendingTurn):
  ensure runtime exists
  ensure runtime.RouteCatalogVersion not empty

  projectedIntent = mergedIntent
  if projectedIntent.Action registered:
    decision.route_kind = business_action
    decision.intent_id = catalog.lookupByAction(projectedIntent.Action) or action.<id>
    decision.candidate_action_ids = [projectedIntent.Action]
    decision.confidence_band = high
    decision.reason_codes += route_business_action_registered
  else:
    projectedIntent = runtime.routeIntent(userInput, projectedIntent)
    decision.route_kind = projectedIntent.RouteKind
    decision.intent_id = projectedIntent.IntentID
    decision.candidate_action_ids = []

    if decision.route_kind in {knowledge_qa, chitchat}:
      decision.confidence_band = low
      decision.clarification_required = false
      decision.reason_codes += route_non_business_catalog_match
    else if decision.route_kind == uncertain:
      decision.confidence_band = low
      decision.clarification_required = true
      decision.reason_codes += route_uncertain_no_match, route_clarification_required
    else:
      fail closed

  if localUpgradeDetected(resolved, mergedIntent):
    decision.confidence_band = medium
    decision.clarification_required = true
    decision.reason_codes += route_local_intent_upgrade, route_clarification_required

  attach runtime snapshot fields
  normalize reason_codes sort+dedupe
  backfill projectedIntent.intent_id / route_kind / route_catalog_version
  return decision, projectedIntent
```

### 6.5 builder 结果的校验规则
1. [ ] 新增 `assistantValidateIntentRouteDecision(...)`：
   - [ ] `route_kind` 必须合法；
   - [ ] `intent_id` 非空；
   - [ ] `confidence_band` 必须合法；
   - [ ] `business_action` 的 `candidate_action_ids` 至少 1 个；
   - [ ] 非 `business_action` 的 `candidate_action_ids` 必须为空；
   - [ ] `route_catalog_version / knowledge_snapshot_digest / resolver_contract_version / decision_source` 必须非空。
2. [ ] builder 出口和持久化入口都应调用校验；前者拦逻辑错误，后者防未来回归绕过。

## 7. 接线方案（按文件直接实施）

### 7.1 `internal/server/assistant_intent_pipeline.go`
1. [ ] 保持 `resolveIntent(...)` 专注于“模型解析 + 本地补强”，不要在该函数里继续塞 route 决策逻辑。
2. [ ] 现有 `assistantNormalizeResolvedIntentWithLocalFacts(...)` 与 `assistantShouldUpgradeIntentFromLocalFacts(...)` 继续保留，但其作用明确收敛为 **route builder 的输入信号**。
3. [ ] 若需要把“是否发生 local upgrade”显式传给 route builder，可新增轻量元信息结构，但不得让 `resolveIntent(...)` 直接返回最终 route decision。
4. [ ] 若后续为了避免重复判断而新增 helper，优先命名为 `assistantRouteUpgradeSignal(...)` 一类纯函数，不要继续把语义塞进 `assistantIntentSpec` 字段。

### 7.2 `internal/server/assistant_api.go` 与 `internal/server/assistant_persistence.go`
1. [ ] 在 `createTurn(...)` 与 `createTurnPG(...)` 中统一替换当前散落逻辑：
   - [ ] `assistantMergeIntentWithPendingTurn(...)` 之后，不再直接调用 `knowledgeRuntime.routeIntent(...)` 当最终结果；
   - [ ] 改为统一调用 `assistantBuildIntentRouteDecision(...)`；
   - [ ] 再由 builder 回填 `intent.IntentID / intent.RouteKind / intent.RouteCatalogVersion`。
2. [ ] `lookupActionSpec(...)` 必须基于 `route decision` 分流：
   - [ ] `business_action`：按 `candidate_action_ids[0]` / `intent.Action` 查 action spec；
   - [ ] 非动作：允许继续使用 `assistantIntentPlanOnly` 的只读 spec 生成说明性 plan，但不得让该 spec 代表“可确认动作”；
   - [ ] `uncertain + clarification_required=true`：同样使用只读 spec 承载解释，但不得进入确认链。
3. [ ] `plan_context_v1` 继续复用，但构建时优先读 `RouteDecision`；不得再用 `intent.RouteKind` 推断非动作场景。
4. [ ] 在 `createTurn(...)` 与 `createTurnPG(...)` 中，`turn.RouteDecision` 的赋值应发生在：
   - [ ] `assistantMergeIntentWithPendingTurn(...)` 之后；
   - [ ] `assistantIntentValidationErrors(...)` 之前；
   - [ ] `lookupActionSpec(...)` 之前。
5. [ ] route 产生的 plan 语义调整：
   - [ ] 非动作 turn 允许保留“说明性 plan”，但 `Plan.Title/Summary` 只能表达“不会触发业务提交”；
   - [ ] 不允许生成让前端误解为“可确认”的 commit 摘要。

### 7.3 `internal/server/assistant_phase_snapshot.go`
1. [ ] `assistantRefreshTurnDerivedFields(...)` 必须显式读取 `turn.RouteDecision`。
2. [ ] `assistantTurnPhase(...)` 新规则：
   - [ ] `route_kind=knowledge_qa/chitchat/uncertain` → 默认返回 `idle`，不得返回 `await_commit_confirm`；
   - [ ] `route_kind=business_action + clarification_required=true` → 仍返回 `idle`（真正澄清 phase 由 `243` 引入），但必须保证不会出现 confirm CTA；
   - [ ] 只有 `business_action + clarification_required=false` 时，才允许沿用现有 `missing_fields / candidate_confirmation / pending_draft_summary` 推导链。
3. [ ] `assistantTurnPendingDraftSummary(...)` 新规则：
   - [ ] 非动作与待澄清 route 一律返回空字符串；
   - [ ] 只有可进入动作链的 `business_action` 才生成确认摘要。
4. [ ] `assistantTurnMissingFields(...)` 保持聚焦“字段缺失”，不吞并 route reason codes。
5. [ ] 推荐新增 helper：
   - [ ] `assistantTurnRouteKind(turn *assistantTurn) string`
   - [ ] `assistantTurnRouteClarificationRequired(turn *assistantTurn) bool`
   - [ ] `assistantTurnActionChainAllowed(turn *assistantTurn) bool`
   以避免 phase / summary / gate 各自散写 route 判断。

### 7.4 `internal/server/assistant_action_interceptor.go`
1. [ ] 在 `assistantEvaluateActionGate(...)` 中新增 route 检查，且对 `StageConfirm / StageCommit` 应作为**第一道 gate**执行；只有 route 允许进入动作链后，才继续 capability/authz/risk/required checks。
2. [ ] 新增检查函数，例如 `assistantCheckRouteDecision(...)`：
   - [ ] `route_kind!=business_action` → 直接阻断 `confirm/commit`；
   - [ ] `clarification_required=true` → 直接阻断 `confirm/commit`；
   - [ ] `candidate_action_ids` 与 `input.Action.ID` 冲突 → fail-closed。
3. [ ] 阻断后返回新错误码，不再复用 `errAssistantUnsupportedIntent` 掩盖 route 语义。
4. [ ] `plan` 阶段是否检查 route：
   - [ ] `StagePlan` 允许非动作 turn 通过，以便 create-turn 成功；
   - [ ] 但若 route decision 自身非法，`StagePlan` 也必须 fail-closed。
5. [ ] `applyConfirmTurn(...) / prepareCommitTurn(...) / submitCommitTaskPG(...)` 仍应保留断言式 route 检查，作为深层 fail-closed 保险；但这些检查不构成新的业务旁路，也不改变“入口先过 route gate”的主顺序。

### 7.5 `internal/server/assistant_knowledge_runtime.go`
1. [ ] 现有 `routeIntent(...)` 可保留，但角色下沉为 builder 内部辅助函数或被重命名为 `routeIntentLegacyProjection(...)`；`242` 完成后它不得继续作为最终 runtime SSOT。
2. [ ] 若保留 `routeIntent(...)`，必须明确：
   - [ ] 它只负责 catalog 层的初步匹配；
   - [ ] `confidence_band / clarification_required / reason_codes / candidate_action_ids` 由新的 route builder 统一补齐。
3. [ ] `buildPlanContextV1(...)` 应接受 `RouteDecision` 或从 `intent` 读取前先校验与 `RouteDecision` 一致，防止双主源漂移。
4. [ ] 推荐新增 helper：
   - [ ] `assistantRouteCatalogIntentByAction(actionID string) (assistantIntentRouteEntry, bool)`
   - [ ] `assistantRouteSnapshotFields(runtime *assistantKnowledgeRuntime) ...`
   以减少 builder 里对 runtime 内部字段的直接耦合。

### 7.6 函数级改造清单（冻结）
1. [ ] 必改函数：
   - [ ] `(*assistantConversationService).createTurn`
   - [ ] `(*assistantConversationService).createTurnPG`
   - [ ] `assistantRefreshTurnDerivedFields`
   - [ ] `assistantTurnPhase`
   - [ ] `assistantTurnPendingDraftSummary`
   - [ ] `assistantEvaluateActionGate`
   - [ ] `(*assistantConversationService).upsertTurnTx`
   - [ ] `(*assistantConversationService).loadConversationTx`
2. [ ] 建议新增函数：
   - [ ] `assistantBuildIntentRouteDecision`
   - [ ] `assistantValidateIntentRouteDecision`
   - [ ] `assistantCheckRouteDecision`
   - [ ] `assistantTurnActionChainAllowed`
3. [ ] 建议避免的改造方式：
   - [ ] 不要把 route 全部逻辑继续塞回 `assistantBuildPlan(...)`；
   - [ ] 不要把 route reason codes 混入 `DryRun.ValidationErrors`；
   - [ ] 不要只在 API 路径修而漏掉 PG 路径。

## 8. 持久化与审计方案（冻结）

### 8.1 Schema 方案
1. [ ] 在 `modules/iam/infrastructure/persistence/schema/` 新增迁移，为 `iam.assistant_turns` 增加：
   - [ ] `route_decision_json jsonb NULL`；
   - [ ] `CHECK (route_decision_json IS NULL OR jsonb_typeof(route_decision_json) = 'object')`。
2. [ ] 首期允许历史 turn 为 `NULL`，避免伪造回填；但 `PR-242-01` 上线后**新写入 turn 不得再持久化空 route decision**。
3. [ ] 本计划不为历史 turn 设计复杂 backfill 规则，避免把错误的“推测路由”写成历史真相。

### 8.2 读写接线
1. [ ] `internal/server/assistant_persistence.go`：
   - [ ] `upsertTurnTx(...)` 插入/更新 `route_decision_json`；
   - [ ] `loadConversationTx(...)` 反序列化 `turn.RouteDecision`；
   - [ ] 若读到历史 `NULL`，只允许保留空值做历史兼容显示，不得在确认/提交路径上把它当作“允许执行”。
2. [ ] 内存态 `assistantConversationService` 也必须同步保存 `turn.RouteDecision`，保证 memory/PG 行为一致。
3. [ ] 推荐在 `upsertTurnTx(...)` 前做一层 `assistantValidateIntentRouteDecision(...)`，避免脏数据入库。

### 8.3 审计关联要求
1. [ ] `RouteDecision` 中必须写入：
   - [ ] `RouteCatalogVersion`；
   - [ ] `KnowledgeSnapshotDigest`；
   - [ ] `ResolverContractVersion`；
   - [ ] `DecisionSource`。
2. [ ] turn 已有 `request_id / trace_id / turn_id`；本计划不重复造审计主键，但要确保 route decision 可随 turn 一并查询与复盘。
3. [ ] `reason_codes` 必须可用于解释“为什么未进入动作链”，但不得成为用户可见错误消息本体。

### 8.4 迁移与上线顺序
1. [ ] 先上 schema 与读写兼容，再上 route gate；避免代码先依赖新列而数据库未准备好。
2. [ ] 建议顺序：
   - [ ] 迁移加列；
   - [ ] 代码支持读写 `route_decision_json`；
   - [ ] builder 开始写新值；
   - [ ] phase/gate 改为强依赖 route decision；
   - [ ] 最后补 route 新错误码映射与测试。
3. [ ] 在 route gate 启用前，允许历史 turn `route_decision_json=NULL`；route gate 启用后，对新 turn 必须非空，对历史 turn 则在 confirm/commit 处 fail-closed。

## 9. API 语义与错误码（冻结）

### 9.1 create-turn 语义
1. [ ] 以下场景仍返回成功创建 turn，而不是 HTTP 错误：
   - [ ] `knowledge_qa`；
   - [ ] `chitchat`；
   - [ ] `business_action + clarification_required=true`；
   - [ ] `uncertain`。
2. [ ] 这些“软分流”场景以 `turn.route_decision` 作为唯一运行时判读主源；`turn.intent.route_kind` 仅作兼容投影，`turn.dry_run.explain` 仅作展示说明，二者都不得再参与“是否可确认/提交”的运行时判断。
3. [ ] create-turn 响应的最小判读规则：
   - [ ] 看 `route_decision.route_kind` 决定是否为动作；
   - [ ] 看 `route_decision.clarification_required` 决定是否需交给 `243`；
   - [ ] 不再用 `plan.title/summary` 推断是否可提交。

### 9.2 硬失败错误码
1. [ ] 仅以下 route 异常返回 API 错误：
   - [ ] `ai_route_runtime_invalid`：route builder 输出非法枚举/缺主字段；
   - [ ] `ai_route_catalog_missing`：catalog/runtime 缺失；
   - [ ] `ai_route_action_conflict`：route decision 与 action spec 冲突；
   - [ ] `ai_route_decision_missing`：确认/提交阶段缺 route decision。
2. [ ] 错误映射需补到：
   - [ ] `internal/server/assistant_api.go`；
   - [ ] `internal/server/assistant_persistence.go` 的 idempotency error payload / restore 分支。

### 9.3 confirm/commit 阶段错误码
1. [ ] 非动作 route 进入 confirm/commit 时返回：`ai_route_non_business_blocked`。
2. [ ] 待澄清 route 进入 confirm/commit 时返回：`ai_route_clarification_required`。
3. [ ] route decision 缺失或非法时返回：`ai_route_decision_missing` 或 `ai_route_runtime_invalid`。
4. [ ] 以上错误码不得继续落成 `assistant_unsupported_intent`，避免隐藏真实运行时语义。

### 9.4 响应示例（冻结）
1. [ ] `knowledge_qa` create-turn 示例：
```json
{
  "turn_id": "turn_x",
  "phase": "idle",
  "intent": {
    "action": "plan_only",
    "intent_id": "knowledge.general_qa",
    "route_kind": "knowledge_qa"
  },
  "route_decision": {
    "route_kind": "knowledge_qa",
    "clarification_required": false,
    "confidence_band": "low",
    "reason_codes": ["route_non_business_catalog_match"]
  },
  "pending_draft_summary": "",
  "dry_run": {
    "explain": "这是知识问答场景，不会触发业务提交。"
  }
}
```
2. [ ] `uncertain` create-turn 示例：
```json
{
  "turn_id": "turn_y",
  "phase": "idle",
  "route_decision": {
    "route_kind": "uncertain",
    "clarification_required": true,
    "confidence_band": "low",
    "reason_codes": [
      "route_uncertain_no_match",
      "route_clarification_required"
    ]
  },
  "pending_draft_summary": ""
}
```
3. [ ] 非动作 confirm 阻断示例：
```json
{
  "code": "ai_route_non_business_blocked",
  "message": "assistant route non business blocked"
}
```

### 9.5 错误码注册清单（建议冻结）
1. [ ] 建议新增仓内 error 变量：
```go
var (
    errAssistantRouteRuntimeInvalid     = errors.New("ai_route_runtime_invalid")
    errAssistantRouteCatalogMissing     = errors.New("ai_route_catalog_missing")
    errAssistantRouteActionConflict     = errors.New("ai_route_action_conflict")
    errAssistantRouteDecisionMissing    = errors.New("ai_route_decision_missing")
    errAssistantRouteNonBusinessBlocked = errors.New("ai_route_non_business_blocked")
    errAssistantRouteClarificationRequired = errors.New("ai_route_clarification_required")
)
```
2. [ ] 建议错误码语义对应关系：
   - [ ] `ai_route_runtime_invalid`：builder 输出缺字段、非法枚举、快照字段为空、非动作却带 action candidates 等内部契约错误；
   - [ ] `ai_route_catalog_missing`：`ensureKnowledgeRuntime()` 成功前置条件不成立，或 route catalog 缺失/不可读；
   - [ ] `ai_route_action_conflict`：`route_kind=business_action` 但 `candidate_action_ids[0]`、`intent.action`、`ActionSpec.ID` 三者不一致；
   - [ ] `ai_route_decision_missing`：confirm/commit 或持久化恢复阶段需要 route decision，但 turn 上为空；
   - [ ] `ai_route_non_business_blocked`：`knowledge_qa/chitchat` 进入确认或提交；
   - [ ] `ai_route_clarification_required`：`uncertain` 或 `business_action+clarification_required=true` 进入确认或提交。
3. [ ] 建议 HTTP 映射：
   - [ ] `ai_route_runtime_invalid` → `422 Unprocessable Entity`；
   - [ ] `ai_route_catalog_missing` → `503 Service Unavailable`；
   - [ ] `ai_route_action_conflict` → `422 Unprocessable Entity`；
   - [ ] `ai_route_decision_missing` → `409 Conflict`；
   - [ ] `ai_route_non_business_blocked` → `409 Conflict`；
   - [ ] `ai_route_clarification_required` → `409 Conflict`。
4. [ ] API 映射位置建议：
   - [ ] `handleAssistantTurnsAPI(...)` 的 create-turn 错误分支；
   - [ ] `handleAssistantTurnActionAPI(...)` 的 confirm/commit 错误分支；
   - [ ] `assistantIdempotencyErrorPayload(...)`；
   - [ ] `assistantErrorFromIdempotencyCode(...)`。
5. [ ] `assistant_unsupported_intent` 保留边界：仅用于“确认为业务动作，但仓内确实不存在该动作实现/commit adapter”的老语义；如果是 route 问题，一律改用上述新错误码。

## 10. 分批实施（可直接开工）

### 10.1 PR-242-01：DTO、Schema 与持久化骨架
1. [ ] 新增 `internal/server/assistant_intent_router.go`，定义：
   - [ ] `assistantIntentRouteDecision`；
   - [ ] 枚举校验函数；
   - [ ] `ReasonCodes` 常量。
2. [ ] 修改 `internal/server/assistant_api.go`：给 `assistantTurn` 增加 `RouteDecision` 字段。
3. [ ] 修改 `internal/server/assistant_persistence.go`：序列化/反序列化 `route_decision_json`。
4. [ ] 新增 IAM 迁移：为 `iam.assistant_turns` 加 `route_decision_json` 列与 check 约束。
5. [ ] 本 PR 不改业务行为，只完成结构落位和内存/PG 对齐。
6. [ ] 完成定义：
   - [ ] `assistantTurn` 的 JSON 能携带 `route_decision`；
   - [ ] PG round-trip 不丢失 `route_decision_json`；
   - [ ] 所有现有测试在默认空 `RouteDecision` 下仍能编译通过。

### 10.2 PR-242-02：Route Builder 与 create-turn 主接线
1. [ ] 实现 `assistantBuildIntentRouteDecision(...)`。
2. [ ] 替换 `createTurn(...) / createTurnPG(...)` 中直接依赖 `knowledgeRuntime.routeIntent(...)` 的逻辑，统一由 builder 产出：
   - [ ] `turn.RouteDecision`；
   - [ ] 投影后的 `turn.Intent`；
   - [ ] route 相关 plan 版本字段。
3. [ ] 保持 `assistantBuildPlan(...)` 与 `assistantApplyPlanContextV1(...)` 可复用，但其输入改为 route 决策之后的结果。
4. [ ] 完成定义：
   - [ ] create-turn 返回中存在完整 `route_decision`；
   - [ ] 非动作 turn 的 `phase` 仍为 `idle`；
   - [ ] `plan_context_v1` 不再只依赖 `intent.RouteKind`。

### 10.3 PR-242-03：阶段语义与 confirm/commit 阻断
1. [ ] 修改 `assistant_phase_snapshot.go`，防止非动作 / 待澄清 route 落成 `await_commit_confirm`。
2. [ ] 修改 `assistant_action_interceptor.go`，新增 route gate。
3. [ ] 修改 `applyConfirmTurn(...) / prepareCommitTurn(...)` 相关路径，确保 route 错误不会被后续逻辑掩盖成通用 confirmation/unsupported。
4. [ ] 完成定义：
   - [ ] 非动作 / uncertain / 待澄清 turn 全部无法进入 `confirm/commit`；
   - [ ] `assistantTurnPendingDraftSummary(...)` 在这些场景返回空；
   - [ ] 历史空 route turn 在 confirm/commit 上 fail-closed。

### 10.4 PR-242-04：错误码、API 映射与回归测试
1. [ ] 在 API 层补齐 route 新错误码映射。
2. [ ] 在 idempotency 错误恢复分支补齐 route 错误码恢复。
3. [ ] 新增回归测试，覆盖 memory / PG / API / persistence / phase snapshot 多层面行为。
4. [ ] 完成定义：
   - [ ] 所有 route 相关阻断都有专属错误码；
   - [ ] `assistant_unsupported_intent` 只保留给真正的 action 不支持场景，不再吞并 route 语义；
   - [ ] idempotency 恢复不会把 route 错误降级成未知错误。

### 10.5 推荐实际落地顺序
1. [ ] 先写 DTO + schema + persistence，再写 builder；
2. [ ] 然后收紧 `phase` 与 `pending_draft_summary`；
3. [ ] 再上 confirm/commit route gate；
4. [ ] 最后补 API 错误码和测试；
5. [ ] 任何阶段若发现需要“先造临时 phase / 先加 plan_only 旁路”才能通过，应立即停止并回到本计划修订，不得边写边漂移。

## 11. 测试与覆盖率（直接对应文件）
1. [ ] 新增 `internal/server/assistant_intent_router_test.go`：
   - [ ] 注册业务动作 → `business_action + high`；
   - [ ] `plan_only + 非动作 catalog` → `knowledge_qa/chitchat`；
   - [ ] 无匹配 → `uncertain + clarification_required=true`；
   - [ ] local upgrade → `business_action + medium + route_local_intent_upgrade`；
   - [ ] 非法 builder 输出 → `ai_route_runtime_invalid`。
2. [ ] 补充 `internal/server/assistant_phase_snapshot_test.go`：
   - [ ] `knowledge_qa/chitchat/uncertain` 不进入 `await_commit_confirm`；
   - [ ] `business_action + clarification_required=true` 不生成确认摘要；
   - [ ] 只有 `business_action + clarification_required=false` 才能保留原有 phase 推导。
3. [ ] 补充 `internal/server/assistant_api_gap_test.go` 与/或 `internal/server/assistant_api_coverage_test.go`：
   - [ ] 创建非动作 turn 返回 200，但 `phase=idle`；
   - [ ] 非动作 turn 调 `confirm/commit` 返回 route 新错误码；
   - [ ] route runtime 缺失时 create-turn fail-closed；
   - [ ] `business_action + clarification_required=true` 创建成功但 confirm/commit 阻断。
4. [ ] 补充 `internal/server/assistant_persistence_coverage_test.go`：
   - [ ] `route_decision_json` 可写可读；
   - [ ] 历史 `NULL` 记录不会在 confirm/commit 被当成可执行 turn；
   - [ ] idempotency 恢复能识别 route 新错误码。
5. [ ] 补充 `internal/server/assistant_action_interceptor_test.go`：
   - [ ] `knowledge_qa/chitchat/uncertain` 在 confirm/commit 阶段被 route gate 拒绝；
   - [ ] `candidate_action_ids` 与 `Action.ID` 冲突时 fail-closed。
6. [ ] 至少新增 5 条自然语言回归样例，不得只覆盖黄金句式：
   - [ ] “帮我解释一下为什么组织不能创建” → `knowledge_qa`；
   - [ ] “你好，在吗” → `chitchat`；
   - [ ] “在鲜花组织下面搞一个新的那个部门吧” → `business_action + clarification_required=true` 或 `medium`；
   - [ ] “帮我建个组织，名字以后再说” → `business_action + clarification_required=true`；
   - [ ] “这个系统能做什么” → `knowledge_qa` 或 `uncertain`，但不能进动作链。

### 11.1 最小断言矩阵
| 断言 | memory | PG | API | phase | gate |
| --- | --- | --- | --- | --- | --- |
| create-turn 写入 `route_decision` | [ ] | [ ] | [ ] | - | - |
| 非动作 turn `phase=idle` | [ ] | [ ] | [ ] | [ ] | - |
| 非动作 turn 无 `pending_draft_summary` | [ ] | [ ] | [ ] | [ ] | - |
| 非动作 confirm 被阻断 | [ ] | [ ] | [ ] | - | [ ] |
| uncertain commit 被阻断 | [ ] | [ ] | [ ] | - | [ ] |
| 历史空 route turn confirm fail-closed | - | [ ] | [ ] | - | [ ] |
| route 错误码可从 idempotency 恢复 | - | [ ] | - | - | - |

### 11.2 推荐测试用例名（建议直接采用）
1. [ ] `internal/server/assistant_intent_router_test.go`：
   - [ ] `TestAssistantBuildIntentRouteDecision_BusinessActionRegistered`
   - [ ] `TestAssistantBuildIntentRouteDecision_NonBusinessCatalogMatch`
   - [ ] `TestAssistantBuildIntentRouteDecision_UncertainRequiresClarification`
   - [ ] `TestAssistantBuildIntentRouteDecision_LocalUpgradeDowngradesConfidence`
   - [ ] `TestAssistantValidateIntentRouteDecision_RejectsInvalidShape`
2. [ ] `internal/server/assistant_phase_snapshot_test.go`：
   - [ ] `TestAssistantTurnPhase_NonBusinessRoutesStayIdle`
   - [ ] `TestAssistantTurnPendingDraftSummary_NonBusinessRoutesEmpty`
   - [ ] `TestAssistantTurnPendingDraftSummary_ClarificationRequiredEmpty`
3. [ ] `internal/server/assistant_action_interceptor_test.go`：
   - [ ] `TestAssistantCheckRouteDecision_BlockNonBusinessConfirm`
   - [ ] `TestAssistantCheckRouteDecision_BlockClarificationRequiredCommit`
   - [ ] `TestAssistantCheckRouteDecision_BlockCandidateActionConflict`
4. [ ] `internal/server/assistant_api_gap_test.go` 或 `internal/server/assistant_api_coverage_test.go`：
   - [ ] `TestAssistantCreateTurn_NonBusinessRouteReturnsIdleTurn`
   - [ ] `TestAssistantConfirmTurn_NonBusinessRouteBlocked`
   - [ ] `TestAssistantCommitTurn_ClarificationRequiredBlocked`
   - [ ] `TestAssistantCreateTurn_RouteRuntimeInvalidFailsClosed`
5. [ ] `internal/server/assistant_persistence_coverage_test.go`：
   - [ ] `TestAssistantUpsertTurnTx_PersistsRouteDecisionJSON`
   - [ ] `TestAssistantLoadConversationTx_RestoresRouteDecisionJSON`
   - [ ] `TestAssistantIdempotency_RouteErrorsRoundTrip`
6. [ ] 命名约束：
   - [ ] 测试名应直接体现 route 语义，不要继续使用泛化的 `unsupported intent` 命名；
   - [ ] memory / PG 双路径若各有测试，名称中建议显式带上 `PG` 或 `Memory`，防止后期覆盖盲区。

## 12. 验收标准
1. [ ] 仓库内存在正式 `assistantIntentRouteDecision` DTO，且 `createTurn / createTurnPG` 都写入同一结构。
2. [ ] 新建 turn 时，`knowledge_qa/chitchat/uncertain` 不再因为 `plan_only` 而进入 `await_commit_confirm`。
3. [ ] `confirm/commit` 显式读取 `route decision`，并对非动作 / 待澄清 route 进行阻断。
4. [ ] route 审计可在 turn 维度查询到 `reason_codes / confidence_band / route_catalog_version / knowledge_snapshot_digest`。
5. [ ] memory 与 PG 行为一致；不存在一条路径有 route gate、另一条路径没有 route gate 的漂移。
6. [ ] 错误码能区分：
   - [ ] route 硬失败；
   - [ ] 非动作阻断；
   - [ ] 待澄清阻断；
   - [ ] action 未注册。
7. [ ] 非动作/uncertain turn 即使 create 成功，也不会在 UI/API 契约层被误读为“等待你确认提交”。
8. [ ] 现有 Assistant 消费侧（页面 CTA、接口调用方或等价入口）已明确以 `route_decision` 为唯一可提交判据；对 `knowledge_qa/chitchat/uncertain/business_action+clarification_required=true` 不展示 confirm CTA，并提供对应回归测试、截图或等价证据。

## 13. 停止线（Fail-Closed）
1. [ ] 若最终仍只把 `route` 放在 `assistantIntentSpec.RouteKind` 里，而没有独立 `RouteDecision`，本计划失败。
2. [ ] 若 `knowledge_qa/chitchat/uncertain` 仍可能落成 `await_commit_confirm`，本计划失败。
3. [ ] 若 `confirm/commit` 仍不显式检查 route 决策，本计划失败。
4. [ ] 若为兼容旧逻辑而保留“没有 route decision 也可继续 confirm/commit”的新旁路，本计划失败。
5. [ ] 若 route 新错误码最终仍被压扁成 `assistant_unsupported_intent`，本计划失败。
6. [ ] 若 memory 路径与 PG 路径 route 行为不同，本计划失败。

## 14. 门禁与 SSOT 引用
1. [ ] 文档与实现触发器以 `AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 为准。
2. [ ] 本计划一旦进入代码实现，至少命中以下本地门禁：
   - [ ] `go fmt ./... && go vet ./... && make check lint && make test`
   - [ ] `make check no-legacy`
   - [ ] `make check error-message`
   - [ ] `make check doc`
3. [ ] 若命中 Assistant 路由、错误码或 schema 相关门禁，还需按实际变更补跑对应检查；本文不复制脚本细节，以 SSOT 为准。

## 15. 交付物
1. [ ] 本计划文档：`docs/dev-plans/242-assistant-intent-router-runtime-minimal-plan.md`
2. [ ] 执行记录：`docs/dev-records/dev-plan-242-execution-log.md`
3. [ ] 代码交付物：
   - [ ] `assistantIntentRouteDecision` DTO 与 builder；
   - [ ] `route_decision_json` 持久化；
   - [ ] phase / confirm / commit route gate；
   - [ ] route 错误码与回归测试。

## 16. 关联文档
- `docs/dev-plans/240e-assistant-internal-knowledge-pack-and-readonly-resolver-plan.md`
- `docs/dev-plans/241-assistant-knowledge-pack-runtime-minimal-implementation-plan.md`
- `docs/dev-plans/243-assistant-clarification-policy-and-slot-repair-plan.md`
- `docs/dev-plans/244-assistant-interpretation-pack-and-intent-route-catalog-compiler-plan.md`
- `docs/dev-plans/245-assistant-reply-guidance-pack-and-reply-realizer-plan.md`
- `docs/dev-plans/246-assistant-understand-route-clarify-roadmap.md`
