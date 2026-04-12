# DEV-PLAN-350：Assistant Tooling 对齐统一策略模型实施方案

**状态**: 修订中（2026-04-12 11:36 CST）

## 1. 背景

在 `DEV-PLAN-340` 中，已经确认 Assistant 当前并不是第二套 OrgUnit 写内核，但确实在 `create_orgunit` dry-run 阶段重复承担了部分领域前置裁决；在 `DEV-PLAN-330` 中，又进一步冻结了统一策略模型，要求所有字段与动作裁决都必须回到：

`原始请求上下文 -> Context Resolver -> PolicyContext -> 唯一 PDP -> explain/version`

因此，后续整改不能停留在“给 Assistant 新增一个专用预检服务”这一层，而必须把 Assistant 明确收敛为 `330` 统一策略主链的消费方，并将 function calling/tooling 限定为只读事实收集层，而不是新的业务执行架构。

`DEV-PLAN-361` 进一步明确：这里的“唯一 PDP”是稳定的主链边界，但其求值实现方式可以由 OPA/Rego 承接；变化点是 PDP 引擎实现，不是策略平台四层事实源，也不是 Assistant/写链/UI 的职责分工。

本计划用于承接 `330 + 340 + 361` 的联合落地，冻结 Assistant Tooling 的正式目标边界、迁移路径与验收口径，并把“唯一 PDP 可由 OPA 承接”的实现假设正式纳入主线。

边界补充冻结：`350` 依赖已完成的 `361` Rego-backed PDP adapter；只有在 `361` 完成唯一 PDP 引擎替换并封板后，`350` 才开始 `PrecheckProjection`、Assistant/tool/write service/explain 的统一消费收口。

## 2. 问题定性

### 2.1 已确认的问题

1. [X] Assistant 不是第二写入口，但已在 `internal/server/assistant_create_policy_precheck.go` 中重复解释部分 OrgUnit create 规则。
2. [X] `DEV-PLAN-330` 已明确要求策略裁决必须由统一 `PolicyContext -> 唯一 PDP -> explain/version` 主链输出。
3. [X] 仅通过“Assistant 调一个局部预检服务”无法满足 `330` 的 SoT 要求，因为那会形成一个只给 Assistant 使用的旁路口径。

### 2.2 正式定性

1. [X] 本问题的本质不是“是否要把 Assistant 改造成通用 Function Calling Agent”。
2. [X] 本问题的本质是“如何在保留事务型 Assistant 主链的前提下，让 Assistant 不再成为第二个策略解释器”。
3. [X] function calling/tooling 在本仓的正式定位应是：
   - 模型接口层优化；
   - 只读事实收集机制；
   - 不改变 `One Door`、`No Tx, No RLS`、`Contract First` 的业务执行架构。

## 3. 目标与非目标

### 3.1 目标

1. [ ] 建立一条对齐 `DEV-PLAN-330` 的 Assistant 策略消费主链：
   `ActionSchema -> Readonly Tool Registry -> Assistant Runtime -> Context Resolver -> 唯一 PDP -> OrgUnit Precheck Projection -> DryRun/Confirm/Commit -> OrgUnitWriteService -> DB Kernel`
2. [ ] 让 Assistant 只消费统一 `Precheck Projection`，不再直接读取 `SetID Strategy Registry`、`tenant_field_configs`、`Mutation Policy` 任一底层实现。
3. [ ] 为 OrgUnit 八动作冻结统一的 `ActionSchema + PolicyContext + PrecheckProjection + CommitAdapter` 扩展框架。
4. [ ] 让 explain/version、`resolved_setid`、`setid_source`、projection digest 进入 Assistant 快照，满足稳定回放与审计叙事。
5. [ ] 明确 `唯一 PDP` 的稳定契约与可替换实现边界：`350` 冻结的是 `Context Resolver -> 唯一 PDP -> PrecheckProjection` 主链，而不是限定 PDP 必须继续由手写 Go 求值器实现。

### 3.2 非目标

1. [ ] 不把当前项目重构成通用 Agent 平台或开放式 MCP 写工具平台。
2. [ ] 第一阶段不新增对外公开 API；优先在仓内 Go 服务边界内收敛。
3. [ ] 第一阶段默认不新增数据库表；如后续发现快照字段无法复用现有结构，再单独立计划获批。
4. [ ] 第一阶段不同时迁移所有领域动作；默认以 `create_orgunit` 作为 `340 + 330` 联合止血样板。
5. [ ] 不把 OPA 视为本仓策略平台替代品；即便引入 OPA，也只承接唯一 PDP 的求值引擎职责。

## 4. 冻结原则

### 4.1 角色边界

1. [ ] Assistant 可以决定：
   - 现在该问什么；
   - 哪些候选需要用户确认；
   - 当前是否具备 confirm 条件；
   - 当前是否允许进入 commit gate。
2. [ ] Assistant 不可以决定：
   - 某字段是否必填；
   - 默认值是什么；
   - 当前 patch 是否允许；
   - `resolved_setid` 如何推导；
   - `priority_mode / local_override_mode` 如何解释。
3. [ ] 上述裁决必须只来自 `DEV-PLAN-330` 冻结的统一策略主链输出。

### 4.2 Tooling 原则

1. [ ] Tooling 第一阶段只允许只读工具，不允许新增写工具或第二提交入口。
2. [ ] 所有策略类工具必须走 `Context Resolver + 唯一 PDP（可由 OPA 承接） + Mutation Policy` 主链，禁止直接暴露底层 registry/store。
3. [ ] Tool 输出、dry-run 输出、Precheck Projection 输出必须共享同一语义契约，禁止三套字段口径并存。

### 4.3 写链原则

1. [ ] 最终写入仍必须走 `modules/orgunit/services/orgunit_write_service.go`。
2. [ ] 写服务保留最终 fail-closed 再校验职责。
3. [ ] Assistant 可见解释链与写服务前置解释链必须共享同一策略裁决核心，而不是各自维护一套实现。
4. [ ] 若后续采纳 `DEV-PLAN-361`，则 OPA 只能位于“唯一 PDP 求值引擎”这一层，不能旁路 `Context Resolver`、不能替代 `Mutation Policy`、不能替代 `OrgUnitWriteService -> DB Kernel` 写链。

## 5. 目标架构

### 5.1 总体结构

目标结构冻结为：

`ActionSchema -> Readonly Tool Registry -> Assistant Runtime -> Context Resolver -> 唯一 PDP -> OrgUnit Precheck Projection -> DryRun/Confirm/Commit -> OrgUnitWriteService -> DB Kernel`

### 5.2 各层职责

1. [ ] `ActionSchema`
   - 描述动作输入、候选确认需求、策略消费契约与只读工具依赖。
2. [ ] `Readonly Tool Registry`
   - 向模型暴露受控只读事实，不暴露底层策略实现。
3. [ ] `Assistant Runtime`
   - 负责编排对话状态、候选确认、risk gate、task gate、commit gate。
4. [ ] `Context Resolver`
   - 负责把外部输入规范化为 `PolicyContext`，包括：
     - `business_unit_org_code -> business_unit_node_key`
     - `business_unit_node_key -> resolved_setid`
     - `setid_source`
5. [ ] `唯一 PDP`
   - 负责输出字段级动态决策、explain 与版本语义；
   - 可由仓内 PDP 适配层 + OPA/Rego 承接求值实现；
   - 但对上仍保持统一 `Decision`/explain/version 契约。
6. [ ] `OrgUnit Precheck Projection`
   - 负责组合 PDP 与 Mutation Policy 输出，形成 Assistant 可消费的统一投影，而不是新的领域规则实现。
7. [ ] `OrgUnitWriteService`
   - 负责最终 fail-closed 校验与提交编排。
8. [ ] `DB Kernel`
   - 保持 `One Door` 单链路提交。

## 6. 核心契约冻结

### 6.1 PolicyContext

`PolicyContext` 至少应稳定包含以下规范化事实：

1. [ ] `tenant_id`
2. [ ] `capability_key`
3. [ ] `as_of`
4. [ ] `business_unit_org_code`
5. [ ] `business_unit_node_key`
6. [ ] `resolved_setid`
7. [ ] `setid_source`
8. [ ] `effective_policy_version`
9. [ ] `subject/action context`

补充冻结：

1. [ ] `business_unit_org_code` 只作为外部输入口径。
2. [ ] `business_unit_node_key` 才是内部命中键。
3. [ ] `resolved_setid` 必须经 `Context Resolver` 解析后进入 PDP，Assistant 不得自行拼装。

### 6.2 PrecheckProjection

`PrecheckProjection` 至少应稳定输出以下信息：

1. [ ] `readiness`
2. [ ] `missing_fields`
3. [ ] `field_decisions`
4. [ ] `candidate_confirmation_requirements`
5. [ ] `pending_draft_summary`
6. [ ] `effective_policy_version`
7. [ ] `mutation_policy_version`
8. [ ] `resolved_setid`
9. [ ] `setid_source`
10. [ ] `policy_explain`
11. [ ] `rejection_reasons`
12. [ ] `projection_digest`

补充冻结：

1. [ ] `field_decisions` 中的 `required/default/allowed/maintainable` 必须来自统一 PDP 与 Mutation Policy 投影，不得由 Assistant 二次推导。
2. [ ] `rejection_reasons` 必须支持 fail-closed 叙事，而不是仅返回布尔值。

### 6.3 ActionSchema

`assistantActionSpec` 需要扩展以下策略消费契约字段：

1. [ ] `PolicyContextContractVersion`
2. [ ] `PrecheckProjectionContractVersion`
3. [ ] `RequiredPolicyFacts`
4. [ ] `ReadonlyTools`
5. [ ] `MutationPolicyKey`
6. [ ] `CapabilityBucketKey`

正式要求：

1. [ ] 新增 OrgUnit 动作时，优先补 `ActionSchema`，而不是继续在 Assistant 主链写新的 orgunit if/switch。
2. [ ] `ActionSchema` 只声明 Assistant 需要哪些事实与契约版本，不承担领域裁决本身。

### 6.4 Readonly Tool Registry

第一阶段只冻结以下两类工具：

1. [ ] 业务对象只读工具：
   - `orgunit_candidate_lookup`
   - `orgunit_candidate_snapshot`
2. [ ] 策略只读工具：
   - `orgunit_action_precheck`
   - `orgunit_field_explain`

正式要求：

1. [ ] 策略工具输出必须复用 `PrecheckProjection` 或其子视图，不得绕开统一投影。
2. [ ] 业务对象工具只返回候选与快照事实，不承担策略解释。

### 6.5 Assistant 快照

Assistant turn/task 快照新增冻结字段：

1. [ ] `policy_context_digest`
2. [ ] `effective_policy_version`
3. [ ] `resolved_setid`
4. [ ] `setid_source`
5. [ ] `precheck_projection_digest`
6. [ ] `mutation_policy_version`

正式要求：

1. [ ] 快照必须支持恢复后回放同一份策略解释叙事。
2. [ ] 若解析失败、版本冲突或 projection 缺失，必须 fail-closed，不得进入 confirm/commit。

## 7. 实施步骤

### Phase 0：冻结统一边界与实现假设

1. [ ] 定义 `PolicyContext` 输入契约，并明确其唯一入口为 `Context Resolver`。
2. [ ] 定义 `PrecheckProjection` 输出契约，并明确 Assistant 只消费投影，不消费策略底层表与 store。
3. [ ] 扩展 `assistantActionSpec` 的策略契约字段，形成 `ActionSchema` 冻结版本。
4. [ ] 明确 `ResolvedSetID / SetIDSource / EffectivePolicyVersion / ProjectionDigest` 的快照固化口径。
5. [ ] 明确 `唯一 PDP` 的稳定边界与实现假设：
   - 上层只依赖 `Decision/explain/version` 契约；
   - 求值实现可由 OPA 承接；
   - `Mutation Policy`、写链、策略事实源边界不因此改变。

### Phase 1：消费已完成的唯一 PDP 适配层

1. [ ] 以 `361` 已完成的 Rego-backed `fieldpolicy.Resolve(...)` 作为上游前置，不在 `350` 内再次承担唯一 PDP 引擎替换。
2. [ ] 以 `create_orgunit` 为首个统一消费动作。
3. [ ] 基于稳定 `Decision / explain / version` 契约定义 `PrecheckProjection`，只做组合与消费，不重写 PDP 求值逻辑。
4. [ ] 保证 `PrecheckProjection` 对同一 `PolicyContext` 的解释完全建立在已完成的 PDP adapter 与 `Mutation Policy` 输出之上。

### Phase 2：切断 Assistant 第二解释器

1. [ ] 将 `internal/server/assistant_create_policy_precheck.go` 收敛为“调用统一 `PrecheckProjection` 的薄适配层”。
2. [ ] 删除其中对以下内容的本地理解：
   - `resolveFieldDecision`
   - 租户字段配置开关
   - `org_code` / `d_org_type` 特判
   - `resolved_setid` 拼装
3. [ ] 保证 `FIELD_REQUIRED_VALUE_MISSING`、`PATCH_FIELD_NOT_ALLOWED` 等 dry-run 结论来源于统一 projection，而不是 Assistant 本地推导。

### Phase 3：收口写服务前置解释

1. [ ] 将 `modules/orgunit/services/orgunit_write_service.go` 中 `applyCreatePolicyDefaults(...)` 所依赖的“策略读取与解释”能力抽为统一只读消费层。
2. [ ] 保持写服务的最终 fail-closed 校验职责，但不再把“可解释预检”锁死在写服务内部。
3. [ ] 让 Assistant dry-run、explain API 与正式写链前置解释共享同一策略裁决核心。

### Phase 4：引入只读 Tool Loop

1. [ ] 在 `ActionSchema` 与 `PrecheckProjection` 稳定后，再向模型开放只读工具。
2. [ ] Assistant 允许模型请求：
   - `orgunit_candidate_lookup`
   - `orgunit_candidate_snapshot`
   - `orgunit_action_precheck`
   - `orgunit_field_explain`
3. [ ] 工具执行结果必须结构化回填到 Assistant runtime 中，并复用统一投影语义。
4. [ ] 严禁把 tool loop 扩展成写工具或旁路 commit 链。

### Phase 5：统一八动作

1. [ ] 将 `create/add_version/insert_version/correct/move/rename/disable/enable` 全部纳入：
   `ActionSchema -> PolicyContext -> PDP -> Mutation Policy -> PrecheckProjection`
2. [ ] 将 Assistant 主链收敛为只关心以下编排事实：
   - `readiness`
   - `missing_fields`
   - `candidate_confirmation`
   - `pending_draft_summary`
   - `confirm_eligibility`
   - `task/commit gate`
3. [ ] 新增动作时，目标增量应收敛为：
   - `ActionSchema`
   - `Projection mapping`
   - `CommitAdapter`

## 8. 代码影响范围

本计划默认影响以下代码边界，后续实施应优先围绕这些位置收敛：

1. [ ] `internal/server/assistant_action_registry.go`
2. [ ] `internal/server/assistant_action_interceptor.go`
3. [ ] `internal/server/assistant_model_gateway.go`
4. [ ] `internal/server/assistant_semantic_orchestrator.go`
5. [ ] `internal/server/assistant_task_store.go`
6. [ ] `internal/server/assistant_create_policy_precheck.go`
7. [ ] `modules/orgunit/services/orgunit_write_service.go`
8. [ ] `pkg/fieldpolicy/setid_strategy_pdp.go`（作为 `361` 上游依赖，不属于本计划主要交付）
9. [ ] `361` 已交付的统一 PDP/OPA 适配层文件（作为消费前置，不在 `350` 重复实现）

说明：

1. [ ] `internal/server` 应继续承担协议编排与运行时适配，不应反向吸入新的领域策略实现。
2. [ ] `modules/orgunit/services` 应承载正式的策略消费与写前 fail-closed 语义。

## 9. 测试与覆盖率

本计划遵循 `AGENTS.md`、`DEV-PLAN-300` 与 `DEV-PLAN-301` 的测试分层要求。

### 9.1 覆盖率口径

1. [ ] 统计口径以仓库现行 Go 测试与 CI 门禁为准，不在本计划复制门禁脚本实现。
2. [ ] 新增测试优先落在最小稳定职责边界，避免继续追加 `*_gap_test.go`、`*_coverage_test.go` 一类补洞式测试。

### 9.2 必测集合

1. [ ] `330` 对齐测试：
   - 同一 `PolicyContext` 输入下，Assistant dry-run 与写服务前置解释得到同一策略结论。
2. [ ] `Context Resolver` 测试：
   - `business_unit_org_code -> business_unit_node_key -> resolved_setid/setid_source` 稳定可回放。
3. [ ] `PDP 唯一性` 测试：
   - Assistant 不直接读 registry/store 时，仍能得到 `required/default/allowed/maintainable` 与 explain/version；
   - `350` 默认依赖已完成的 `361` Rego-backed PDP adapter，只验证消费契约稳定，不重复承担引擎等价替换。
4. [ ] `340` 回归测试：
   - Assistant 不再直接调用字段决议底层逻辑。
5. [ ] `Projection 一致性` 测试：
   - `PrecheckProjection`、tool 输出、dry-run 输出三者字段语义一致。
6. [ ] `快照回放` 测试：
   - turn/task 恢复后，能回放同一 `effective_policy_version/resolved_setid/precheck_projection_digest`。
7. [ ] `fail-closed` 测试：
   - `PolicyContext` 解析失败、`resolved_setid` 缺失、PDP 冲突、Mutation Policy 拒绝时，Assistant 不进入 confirm/commit。
8. [ ] `动作扩展成本` 测试：
   - 新增一个 OrgUnit 动作时，只需补 `ActionSchema + Projection mapping + CommitAdapter`，无需再改多处主链分支。
9. [ ] `OPA 等价回归` 测试：
   - `setid_strategy_pdp` 的现有样例在 OPA 适配后仍输出同一 explain、优先级命中与错误码。

## 10. 风险与缓解

1. [ ] 风险：只把 `assistant_create_policy_precheck.go` 包一层 service，但底层仍各自解释策略。
   - 缓解：必须同时抽出共享的统一策略消费层，而不是只加 façade。
2. [ ] 风险：Tooling 演化为写工具，形成第二提交入口。
   - 缓解：第一阶段工具注册表只允许只读工具，并在运行时 gate 中显式阻断写工具。
3. [ ] 风险：Assistant 快照未固化策略版本与 digest，导致恢复/回放叙事漂移。
   - 缓解：把快照字段纳入正式契约与回归测试。
4. [ ] 风险：`create_orgunit` 收敛后，其他七动作继续保留各自动作分支，导致新旧模型长期并存。
   - 缓解：在 Phase 5 明确八动作统一收口目标，并把新增动作成本测试纳入验收。
5. [ ] 风险：OPA 只被 Assistant 使用，而写服务前置解释仍停留在另一套本地实现，形成新的旁路。
   - 缓解：Phase 3 要求写服务前置解释与 Assistant/tooling 共同消费统一 PDP/Projection 主链。
6. [ ] 风险：团队把 OPA 误当作策略平台替代品，导致 `Context Resolver`、`Mutation Policy`、写链边界被错误下沉。
   - 缓解：在 Phase 0 冻结 OPA 仅承接唯一 PDP 求值引擎职责，并在验收中显式检查边界未漂移。

## 11. 验收标准

1. [ ] Assistant 不再成为第二个策略解释器。
2. [ ] Assistant 不再直接依赖任何只给自己开的策略旁路。
3. [ ] 所有字段与动作裁决，都可回溯到 `DEV-PLAN-330` 的统一模型输出。
4. [ ] `create_orgunit` dry-run 与正式写前解释结果一致。
5. [ ] Function calling/tooling 只增强只读事实收集，不改变业务执行架构。
6. [ ] 若采纳 OPA，则其角色已被验证为“唯一 PDP 候选求值引擎”，而不是新的策略平台或第二条策略解释支线。
7. [ ] 同一 `PolicyContext` 下，Assistant dry-run、tool explain、写服务前置解释得到同一字段决策、错误码与 `effective_policy_version`。

## 12. 关联事实源

1. `docs/dev-plans/330-strategy-module-architecture-and-design-convergence-plan.md`
2. `docs/dev-plans/340-assistant-orgunit-duplicate-maintenance-investigation-and-convergence-plan.md`
3. `docs/dev-plans/341-assistant-mainline-evolution-and-340-350-correlation-investigation.md`
4. `docs/dev-plans/360-librechat-depower-and-langgraph-langchain-layered-takeover-plan.md`
5. `docs/dev-plans/360a-librechat-feature-disablement-and-runtime-cutover-plan.md`
6. `docs/dev-plans/361-opa-pdp-adoption-boundary-and-migration-plan.md`
7. `internal/server/assistant_action_registry.go`
8. `internal/server/assistant_create_policy_precheck.go`
9. `modules/orgunit/services/orgunit_write_service.go`
10. `pkg/fieldpolicy/setid_strategy_pdp.go`
