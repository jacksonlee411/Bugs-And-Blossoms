# DEV-PLAN-240C：Assistant ActionInterceptor 与风险门左移计划（承接 240-M4）

> 归档说明（2026-04-12）：本文件已自 `docs/dev-plans/` 迁入 `docs/archive/dev-plans/`，仅保留为历史参考，不再作为现行 SSOT。

**状态**: 已完成（2026-03-09 13:25 CST；`assistant_action_interceptor.go`、ActionSpec `Security` 单主源、create/confirm/commit 三阶段接线、错误码映射与回归测试已落地；`go test ./internal/server` 通过，`271-S5` 后续仅需把 `240C` 作为已完成输入参与 `240D/240E` 与证据新鲜度联动）

## 1. 背景
1. [X] `240A` 已完成 `ActionRegistry/CommitAdapter/version_tuple OCC`，`240B` 已完成 confirm/commit 状态机统一，但 `auth_object/auth_action/risk_tier/required_checks` 仍未进入运行时单点 gate。
2. [X] 当前 `assistantActionSpec` 仅冻结了 `CapabilityKey/RiskTier/Handler` 的最小子集，`auth_object/auth_action/required_checks` 仍散落在 `assistant_intent_pipeline.go`、`assistant_api.go` 与 `assistant_persistence.go` 的流程代码里。
3. [X] `271-S5` 已明确把 `240C/240D/240E` 视为结构性尾项；在 `240C` 未形成运行时 fail-closed 产物前，不得把 `271-S5` 或 `285` 口径更新为“仅剩封板”。

## 2. 评审结论（2026-03-09）

### 2.1 原版 240C 的主要缺口
1. [X] **缺少代码落点图**：仅描述“做拦截链”，但未冻结修改文件、接入点、删除点，无法直接排工实施。
2. [X] **阶段边界不清**：未区分 `plan / confirm / commit` 各阶段分别检查什么，容易把 `required_checks` 再次分散回流程代码。
3. [X] **契约未和现状对齐**：`240` 主计划已冻结 `auth_object/auth_action/risk_tier/required_checks`，但当前 `assistantActionSpec` 尚未承载这些字段；原版 240C 没写清“先补契约，再接 gate”。
4. [X] **错误码与审计口径缺失**：没有稳定错误码矩阵、HTTP 映射、turn 审计字段回写要求，无法对齐 `make check error-message`。
5. [X] **测试与复跑要求不足**：没有列出最小单测集、负向样例、以及对 `288/290` 证据失效的联动条件，无法支撑 `271-S5` 关闭。

### 2.2 本次修订的决策
1. [X] 将 `240C` 收敛为 **“不改 DB schema、不改 DTO 字段、不切异步任务”** 的运行时治理计划，只解决 M4：`ActionSpec 安全字段补齐 + 单点拦截 + 稳定错误码 + 审计可追踪`。
2. [X] 冻结 `ActionInterceptor` 为后端内部能力，当前仅接入 `create_orgunit`，不在本计划内扩展新的业务动作。
3. [X] 冻结先后顺序：**先补 `ActionSpec.Security` 契约，再引入 interceptor，再把 plan/confirm/commit 接入，再删除散落校验与补测试**。

## 3. 范围与非目标

### 3.1 本计划范围（必须完成）
1. [ ] 扩展 `assistantActionSpec`，补齐 `security.auth_object`、`security.auth_action`、`security.risk_tier`、`security.required_checks`。
2. [ ] 新增 `ActionInterceptor`，统一执行 `capability -> authz -> risk -> required_checks`。
3. [ ] 将 `create turn`、`confirm turn`、`commit turn` 三处正式路径接到统一 gate。
4. [ ] 将错误码、HTTP 状态、turn 审计回写收敛到单点。
5. [ ] 删除/迁移现有散落的 `risk_tier/required_checks/capability` 判断，避免双写规则。

### 3.2 本计划非目标（明确排除）
1. [X] 不改 Casbin 领域模型与 `pkg/authz` 规则结构；仅复用既有 `loadAuthorizer()`、`authz.SubjectFromRoleSlug()`、`authz.DomainFromTenantID()`。
2. [X] 不引入新的数据库表、列或迁移；沿用现有 `assistant_turns.risk_tier/error_code`、`assistant_state_transitions` 与会话快照字段。
3. [X] 不切换默认异步耐久执行；该部分仍由 `240D` 承接。
4. [X] 不处理内部知识包与只读 Resolver 收口；该部分仍由 `240E` 承接。
5. [X] 不扩大 `260/266/280` 的 DTO/UI 契约；前端继续只消费既有 DTO。

## 4. 当前实现盘点（作为实施基线）
1. [X] `internal/server/assistant_action_registry.go`：已有 `assistantActionSpec`、默认 `create_orgunit` 注册、`CommitAdapter` 分发；但缺 `Security` 子结构。
2. [X] `internal/server/assistant_intent_pipeline.go`：`required_checks` 当前在 `assistantCompileIntentToPlans(...)` 中硬编码，属于本计划必须迁移的散点。
3. [X] `internal/server/assistant_api.go`：`assistantRiskTierForIntent(...)` 与 create/confirm/commit API 错误映射分散；create turn 仍可在未统一鉴权 gate 的情况下继续 dry-run。
4. [X] `internal/server/assistant_persistence.go`：PG 路径中的 `applyCommitTurn(...)` 仍以局部 guard 拼接提交前约束，尚未统一接入 `ActionInterceptor`。
5. [X] `internal/server/authz_middleware.go`、`internal/server/capability_route_registry.go`：已有 authorizer 装配与 capability 注册表，可直接复用，不需要新建第二套权限系统。

## 5. 设计冻结（直接实施口径）

### 5.1 ActionSpec 收敛口径
1. [ ] `assistantActionSpec` 新增 `Security assistantActionSecuritySpec`，结构如下：

```go
type assistantActionSecuritySpec struct {
	AuthObject     string
	AuthAction     string
	RiskTier       string
	RequiredChecks []string
}
```

2. [ ] 默认 `create_orgunit` 注册项冻结为：
   - `CapabilityKey = org.orgunit_create.field_policy`
   - `Security.AuthObject = authz.ObjectOrgSetIDCapability`
   - `Security.AuthAction = authz.ActionAdmin`
   - `Security.RiskTier = high`
   - `Security.RequiredChecks = [strict_decode, boundary_lint, candidate_confirmation, dry_run]`
3. [ ] `assistantPlanSummary.CommitAdapterKey`、`assistantSkillExecutionPlan.RiskTier/RequiredChecks` 均必须来自 `ActionSpec.Security`，不得再在编译函数中写死默认值。

### 5.2 Interceptor 运行时契约
1. [ ] 新增文件：`internal/server/assistant_action_interceptor.go`。
2. [ ] 冻结最小接口：

```go
type assistantActionStage string

const (
	assistantActionStagePlan    assistantActionStage = "plan"
	assistantActionStageConfirm assistantActionStage = "confirm"
	assistantActionStageCommit  assistantActionStage = "commit"
)

type assistantActionGateInput struct {
	Stage       assistantActionStage
	TenantID    string
	Principal   Principal
	Action      assistantActionSpec
	Intent      assistantIntentSpec
	Turn        *assistantTurn
	Candidates  []assistantCandidate
	ResolvedID  string
	DryRun      *assistantDryRunResult
	RequestPath string
}

type assistantActionGateDecision struct {
	Allowed    bool
	Error      error
	ErrorCode  string
	HTTPStatus int
	ReasonCode string
}
```

3. [ ] `ActionInterceptor` 只返回“允许/拒绝 + 稳定错误码/HTTP/原因码”，**不直接写 HTTP，不直接写响应 DTO**；API handler 只做统一映射。
4. [ ] 任一阶段未显式经过 `ActionInterceptor` 不得继续进入下一步业务执行，满足 `No Auth Pass, No DryRun / No Capability, No Plan`。

### 5.3 阶段检查矩阵（冻结）
| 阶段 | 必跑检查 | 失败后行为 |
| --- | --- | --- |
| `plan` | `spec_registered` → `capability_registered` → `authz_allowed` → `risk_gate` | 直接拒绝，不生成正式 dry-run 结果 |
| `confirm` | `spec_registered` → `authz_allowed` → `candidate_confirmation` → `risk_gate` | 直接拒绝，不推进到 `confirmed` |
| `commit` | `spec_registered` → `authz_allowed` → `risk_gate` → `required_checks` | 直接拒绝，不调用 `CommitAdapter` |

补充冻结：
1. [ ] `strict_decode`：要求 `assistantIntentSchemaInvalid(intent) == false`。
2. [ ] `boundary_lint`：要求 `assistantBoundaryViolationDetected(user_input) == false`；create turn 阶段直接复用现有判定结果，不重复解析自然语言。
3. [ ] `candidate_confirmation`：当 `ResolvedCandidateID == ""` 或候选仍歧义时拒绝 `confirm/commit`。
4. [ ] `dry_run`：要求 `DryRun.ValidationErrors` 为空；若存在校验错误，不得进入 `commit`。
5. [ ] `risk_gate`：`risk_tier=high` 时，`commit` 仅允许从已确认 turn 进入；不新增前端字段，不改变现有“人工确认后提交”语义。

### 5.4 错误码冻结
| 场景 | 错误码 | HTTP |
| --- | --- | --- |
| action 未注册 | `ai_action_spec_missing` | `422` |
| capability 未注册/失活 | `ai_capability_unregistered` | `422` |
| authz 拒绝 | `ai_action_authz_denied` | `403` |
| 风险门拒绝 | `ai_action_risk_gate_denied` | `409` |
| required check 不通过 | `ai_action_required_check_failed` | `409` |
| actor 身份快照失效 | 继续沿用 `ai_actor_auth_snapshot_expired` | `403` |
| role 漂移 | 继续沿用 `ai_actor_role_drift_detected` | `403` |
| contract/version tuple 漂移 | 继续沿用既有 `ai_plan_contract_version_mismatch` / `ai_version_tuple_stale` | 既有 |

补充规则：
1. [ ] `candidate_confirmation` 与 `dry_run` 若失败，统一先落 `ai_action_required_check_failed`；细分原因写入 `ReasonCode`（如 `candidate_confirmation_required`、`dry_run_validation_failed`）。
2. [ ] API 层不得把 `err.Error()` 直接透传为用户错误码；必须显式映射到上表。
3. [ ] 任何新增错误码必须同步补测试，并满足 `make check error-message`。

### 5.5 审计冻结
1. [ ] 本计划不新增表结构；使用现有 turn 快照与状态转移审计承载 gate 结果。
2. [ ] 每次 gate 拒绝必须至少留下以下可回放事实：
   - `turn.risk_tier`
   - `turn.error_code`
   - `transition.reason_code`
   - `turn.plan.capability_key`
   - `turn.plan.skill_execution_plan.required_checks`
3. [ ] `auth_object/auth_action` 无需本次落库新增字段；先要求可从 `assistantActionSpec` + 执行日志复算，避免本计划引入 schema 漂移。

## 6. 代码改动清单（直接实施顺序）

### 6.1 P0：补齐契约与默认注册
1. [ ] 修改 `internal/server/assistant_action_registry.go`：为 `assistantActionSpec` 新增 `Security` 字段，并将默认 `create_orgunit` 注册项迁移到 `Security` 内。
2. [ ] 删除 `assistantRiskTierForIntent(...)` 对默认风险等级的主源职责；若保留 helper，只允许从 `ActionSpec.Security.RiskTier` 读取。
3. [ ] 修改 `internal/server/assistant_intent_pipeline.go`：`assistantCompileIntentToPlans(...)` 不再写死 `RiskTier/RequiredChecks`，统一从 `ActionSpec` 注入。
4. [ ] 停止线：若 `create_orgunit` 仍可在无 `Security` 定义时产生正式 plan，则 `P0` 失败。

### 6.2 P1：新增统一拦截器
1. [ ] 新增 `internal/server/assistant_action_interceptor.go`，实现：
   - `assistantEvaluateActionGate(...)`
   - `assistantCheckCapabilityRegistered(...)`
   - `assistantCheckActionAuthz(...)`
   - `assistantCheckRiskGate(...)`
   - `assistantCheckRequiredChecks(...)`
2. [ ] `assistantCheckActionAuthz(...)` 复用：
   - `loadAuthorizer()`
   - `authz.SubjectFromRoleSlug(principal.RoleSlug)`
   - `authz.DomainFromTenantID(tenantID)`
   - `Authorizer.Authorize(subject, domain, authObject, authAction)`
3. [ ] `assistantCheckCapabilityRegistered(...)` 复用 `capabilityDefinitionForKey(...)`；缺失或未激活一律 fail-closed。
4. [ ] 停止线：若拦截器内部出现“检查失败但返回 allowed=true”的 warning-only 分支，则 `P1` 失败。

### 6.3 P2：接入 create turn（plan 阶段）
1. [ ] 在 create turn 主链中，顺序冻结为：
   - 解析/解析意图
   - `lookupActionSpec(intent.Action)`
   - 候选解析
   - `assistantEvaluateActionGate(stage=plan)`
   - 编译 `plan/config_delta/skill_execution_plan`
   - 构建 `dry_run`
2. [ ] 若 `stage=plan` gate 拒绝：
   - 不得继续构建正式 dry-run
   - `turn.error_code` 写入稳定错误码
   - API 返回与错误码矩阵一致的 HTTP
3. [ ] 删除 create turn 中散落的 capability 主源判断，只保留 interceptor 单点判断。

### 6.4 P3：接入 confirm / commit 阶段
1. [ ] `confirmTurn(...)` / `confirmTurnPG(...)` 在推进状态机前统一调用 `assistantEvaluateActionGate(stage=confirm)`。
2. [ ] `applyCommitTurn(...)` 在 `lookupCommitAdapter(...)` 之前统一调用 `assistantEvaluateActionGate(stage=commit)`。
3. [ ] `commit` 阶段 gate 必须先于 `CommitAdapter.Commit(...)`；任何 gate 拒绝都不得触发写门。
4. [ ] 现有 `auth snapshot expired`、`role drift`、`contract/version tuple drift` 保持原有 guard，但统一归位到“commit 阶段 gate 之前或之内”；最终对外口径只能有一个主源。
5. [ ] 停止线：若任何路径仍可绕过 gate 直接命中 `CommitAdapter.Commit(...)`，`P3` 失败。

### 6.5 P4：删除散点与补齐错误映射
1. [ ] 清理 `assistant_intent_pipeline.go` 中硬编码 `RequiredChecks/RiskTier` 的残留。
2. [ ] 清理 `assistant_api.go` / `assistant_persistence.go` 中重复的 capability/risk/required-check 判断，避免“双 gate”。
3. [ ] API handler 对 `ActionInterceptor` 返回的 `ErrorCode/HTTPStatus` 做单点响应映射。
4. [ ] `assistant_reply_nlg` 若消费 `turn.error_code`，不得直出英文错误码本体；沿用现有错误消息契约收敛。

## 7. 最小测试集（实现完成前必须补齐）

### 7.1 单测文件落点
1. [ ] 新增 `internal/server/assistant_action_interceptor_test.go`。
2. [ ] 更新 `internal/server/assistant_action_registry_test.go`，覆盖 `Security` 默认注册与 fallback。
3. [ ] 更新 `internal/server/assistant_api_test.go` / `internal/server/assistant_api_coverage_test.go`，覆盖 create/confirm/commit API 的新错误映射。
4. [ ] 更新 `internal/server/assistant_persistence_gap_test.go` 或邻近 PG 路径测试，覆盖 PG 模式下 gate 拒绝不落写门。

### 7.2 必测用例（不得删减）
1. [ ] `plan`：未注册 action → `ai_action_spec_missing`。
2. [ ] `plan`：capability 不存在 → `ai_capability_unregistered`。
3. [ ] `plan`：authz 拒绝 → `ai_action_authz_denied`，且不调用 dry-run。
4. [ ] `confirm`：候选未定/仍歧义 → `ai_action_required_check_failed` + `reason_code=candidate_confirmation_required`。
5. [ ] `commit`：`risk_tier=high` 但未确认 → `ai_action_risk_gate_denied` 或既有确认冲突码；不得写门。
6. [ ] `commit`：`dry_run.ValidationErrors` 非空 → `ai_action_required_check_failed`。
7. [ ] `commit`：gate 全通过 → 正常命中 `CommitAdapter`，成功路径不回归。
8. [ ] 内存实现与 PG 实现在同一输入下返回同一错误码/HTTP 口径。

### 7.3 回归要求
1. [ ] `240C` 合入后，按 `271` 要求重跑 `288 + 290` 至少关键用例；证据时间必须晚于本次合入。
2. [ ] 若 `240C` 改动影响错误码语义、fail-closed 行为或 turn DTO 字段含义，`290` 历史证据立即失效。

## 8. 验收标准
1. [ ] `plan/confirm/commit` 三阶段均统一经过 `ActionInterceptor`，不存在旁路。
2. [ ] `auth_object/auth_action/risk_tier/required_checks` 对 `create_orgunit` 来说全部由 `assistantActionSpec.Security` 单点提供。
3. [ ] `No Auth Pass, No DryRun` 生效：authz 拒绝时 create turn 不产生正式 dry-run 结果。
4. [ ] `No Capability, No Plan` 生效：capability 缺失时不得创建正式 plan。
5. [ ] `No Gate Pass, No Commit` 生效：任何 gate 拒绝均不得触发 `CommitAdapter.Commit(...)`。
6. [ ] 拒绝路径错误码稳定，API/turn/error-message 口径一致。
7. [ ] 内存路径与 PG 路径结果一致，不出现同动作不同入口判定差异。

## 9. 停止线（Fail-Closed）
1. [ ] 发现任何路径可绕过 `ActionInterceptor` 继续 dry-run 或 commit，计划失败。
2. [ ] 发现 `RiskTier/RequiredChecks` 仍存在第二主源，计划失败。
3. [ ] 发现 API 把内部错误字符串直接透传给用户，计划失败。
4. [ ] 发现内存/PG 返回不同错误码、不同 HTTP 或不同是否落写结果，计划失败。
5. [ ] 发现 `240C` 合入后未刷新 `288/290` 关键证据却继续推进 `271-S5/285`，计划失败。

## 10. 交付物
1. [ ] 修订后的动作契约：`internal/server/assistant_action_registry.go`。
2. [ ] 新增统一拦截器：`internal/server/assistant_action_interceptor.go`。
3. [ ] create/confirm/commit 接线修改：`internal/server/assistant_api.go`、`internal/server/assistant_persistence.go`、`internal/server/assistant_intent_pipeline.go`。
4. [ ] 单测与回归补齐：`internal/server/assistant_action_interceptor_test.go` 及相关既有测试文件。
5. [ ] 执行证据未单列沉淀（未形成独立的 `dev-plan-240c-execution-log.md`）。

## 11. 门禁与验证（SSOT 引用）
1. [ ] Go/静态检查/测试以 `AGENTS.md`、`Makefile`、`docs/dev-plans/012-ci-quality-gates.md` 为准。
2. [ ] 本计划实施至少覆盖：`go fmt ./... && go vet ./... && make check lint && make test`。
3. [ ] 权限相关改动至少覆盖：`make authz-pack && make authz-test && make authz-lint`。
4. [ ] 错误码/错误消息改动至少覆盖：`make check error-message`。
5. [ ] 文档改动至少覆盖：`make check doc`。

## 12. 关联文档
- `docs/archive/dev-plans/240-assistant-org-transaction-orchestration-modernization-plan.md`
- `docs/archive/dev-plans/240a-assistant-action-registry-and-commit-adapter-plan.md`
- `docs/archive/dev-plans/240b-assistant-state-machine-unification-plan.md`
- `docs/archive/dev-plans/240d-assistant-durable-execution-and-manual-takeover-plan.md`
- `docs/archive/dev-plans/240e-assistant-internal-knowledge-pack-and-readonly-resolver-plan.md`
- `docs/archive/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/archive/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `docs/archive/dev-plans/288-librechat-266-live-e2e-and-evidence-closure-plan.md`
- `docs/archive/dev-plans/290-librechat-260-m5-real-case-validation-and-evidence-plan.md`
- `docs/archive/dev-plans/285-librechat-cutover-regression-and-closure-plan.md`
- `AGENTS.md`
