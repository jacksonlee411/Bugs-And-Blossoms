# DEV-PLAN-361：OPA 作为唯一 PDP 候选引擎的引入边界与迁移方案

**状态**: 规划中（2026-04-12 10:18 CST）

## 1. 背景

1. [X] `DEV-PLAN-330` 已冻结本仓“配置与策略”统一模型，明确要求所有策略裁决回到：
   `原始请求上下文 -> Context Resolver -> PolicyContext -> 唯一 PDP -> explain/version`。
2. [X] `DEV-PLAN-340/341` 已确认当前主要问题不是第二写入口，而是 Assistant 与正式写链之外，正在形成“第二策略解释器”。
3. [X] 当前仓内已存在一套自研 PDP 雏形：
   - `pkg/fieldpolicy/setid_strategy_pdp.go` 负责字段动态策略命中、bucket 排序、fallback 与 explain trace；
   - `modules/orgunit/services/orgunit_mutation_policy.go` 负责写动作约束；
   - `modules/orgunit/services/orgunit_write_service.go` 与 `internal/server/assistant_create_policy_precheck.go` 又各自消费或重复解释部分策略。
4. [X] 因此，是否引入 OPA 的核心问题不是“要不要重写策略平台”，而是“是否用 OPA 承接唯一 PDP 的求值职责，以减少规则求值层复杂度与重复解释风险”。

## 2. 问题定性

### 2.1 当前复杂度来源

1. [X] 策略平台治理复杂度已经存在且合理，主要包括：
   - `tenant_field_configs` 静态元数据；
   - `SetID Strategy Registry` 动态策略主写；
   - `OrgUnit Mutation Policy` 写动作约束；
   - `Policy Activation` 版本与激活。
2. [X] 当前真正持续膨胀的是“规则求值复杂度”，表现为：
   - 唯一 PDP 的规则与排序逻辑手写在 Go 代码中；
   - Assistant dry-run 与写服务前置解释存在重复理解；
   - explain/version/trace 需要在多条链路维持一致；
   - 新增动作时容易继续扩散到 if/switch 与专用预检代码。

### 2.2 OPA 在本仓的正式定位

1. [X] OPA 不是本仓“配置与策略”模块本身。
2. [X] OPA 的正式定位是：**唯一 PDP 的候选求值引擎**。
3. [X] 即使引入 OPA，下列边界仍属于仓内策略平台，不由 OPA 替代：
   - 配置写入口；
   - schema / migration / RLS；
   - 策略治理台与激活链路；
   - `OrgUnitWriteService -> DB Kernel` 写链；
   - Assistant 事务编排、候选确认与 commit gate。
4. [X] 因而本计划解决的问题是“是否替换唯一 PDP 的实现方式”，不是“是否替换策略平台方案”。

## 3. 目标与非目标

### 3.1 目标

1. [ ] 冻结 OPA 引入后的正式边界：OPA 仅承接唯一 PDP 求值，不改变策略平台四层事实源与 One Door 写链。
2. [ ] 冻结改造分层：替换级、抽取级、适配级、保留级，避免实施时误把“引入 OPA”扩大为全仓推倒重来。
3. [ ] 为 `DEV-PLAN-350` 提供可执行迁移路径，使 Assistant 最终只消费统一 `PrecheckProjection`，不再成为第二策略解释器。
4. [ ] 定义一个最小试点路径，默认只覆盖 `create_orgunit` 样板，不要求第一阶段同时迁移全部八动作。

### 3.2 非目标

1. [ ] 不把本仓重构成通用 policy platform 产品。
2. [ ] 不在第一阶段替换 `tenant_field_configs`、`SetID Strategy Registry`、`Policy Activation` 的主写与治理机制。
3. [ ] 不在第一阶段把 Assistant 重构为通用 agent/tool 平台。
4. [ ] 不默认新增数据库表；若后续需要额外快照字段或策略产物持久化，必须另立计划获批。

## 4. 架构边界（冻结）

### 4.1 引入 OPA 前后不变的边界

1. [ ] 配置与策略模块仍是业务策略平台，唯一事实源仍由 `DEV-PLAN-330` 冻结的四层组成。
2. [ ] `Context Resolver` 仍由仓内 Go 代码负责，OPA 不负责：
   - `business_unit_org_code -> business_unit_node_key`
   - `business_unit_node_key -> resolved_setid`
   - `setid_source`
3. [ ] 最终写链仍必须走：
   `Assistant/HTTP API -> OrgUnitWriteService -> DB Kernel`
4. [ ] 写服务仍保留最终 fail-closed 再校验职责；OPA 不能成为旁路写入口，也不能代替 DB Kernel。

### 4.2 引入 OPA 后变化的边界

1. [ ] `PolicyContext -> 唯一 PDP -> explain/version` 的求值核心可由 OPA/Rego 承接。
2. [ ] Go 服务端职责调整为：
   - 组装规范化 `PolicyContext`
   - 装载策略事实源为 PDP 输入
   - 调用 OPA 求值
   - 将结果映射为 `Decision` / `PrecheckProjection` / explain API 输出
3. [ ] Assistant、explain API、write capabilities API、写服务前置解释都应消费同一套 PDP/Projection 结果，而不是各自维护裁决逻辑。

## 5. 复杂度收益评估

### 5.1 预期减少的复杂度

1. [ ] 减少规则求值逻辑散落在多个 Go 文件中的复杂度。
2. [ ] 减少 Assistant 与写服务重复解释同一字段策略的复杂度。
3. [ ] 减少 explain/trace/version 在多条链路中手工对齐的复杂度。
4. [ ] 减少新增动作时继续复制 bucket 命中、fallback、allowed values 合并与错误码映射逻辑的复杂度。
5. [ ] 减少“为规则求值器本身继续造轮子”的长期维护成本。

### 5.2 不会减少的复杂度

1. [ ] 不会减少策略平台治理复杂度：事实源、激活、RLS、治理台、版本发布仍需仓内维护。
2. [ ] 不会减少业务上下文解析复杂度：`Context Resolver` 仍是本仓业务知识。
3. [ ] 不会减少最终写链与 fail-closed 复杂度：写服务与 DB Kernel 仍必须独立承担最终拒绝职责。
4. [ ] 会新增规则语言与装载链路的学习/调试成本；因此必须控制引入范围，先样板验证。

## 6. 迁移范围分层

### 6.1 替换级（基本重写职责实现）

1. [ ] `pkg/fieldpolicy/setid_strategy_pdp.go`
   - 当前是自研唯一 PDP 雏形；
   - 若采用 OPA，应改为“Go 适配层 + OPA 求值”模式；
   - bucket、fallback、优先级、trace 等求值细节不再长期保留为手写 Go 主实现。
2. [ ] `internal/server/assistant_create_policy_precheck.go`
   - 当前直接解析 `PolicyContext`、直接查底层字段决议、直接读取 `tenant_field_configs`；
   - 应重写为统一 `PrecheckProjection` 的薄适配层。

### 6.2 抽取级（保留边界，抽离策略解释）

1. [ ] `modules/orgunit/services/orgunit_write_service.go`
   - 保留写服务主体与 fail-closed 校验职责；
   - 将 `applyCreatePolicyDefaults(...)` 及其下游策略解释逻辑抽离到统一 PDP/Projection 消费层。
2. [ ] `modules/orgunit/services/orgunit_mutation_policy.go`
   - 第一阶段可继续作为独立规则源保留；
   - 但其输出必须被统一纳入 `PrecheckProjection`，避免 Assistant 或其他链路重复解释。

### 6.3 适配级（主体保留，改成统一消费）

1. [ ] `internal/server/assistant_action_registry.go`
   - 扩展为 `ActionSchema` 风格元数据，补齐 `PolicyContextContractVersion`、`PrecheckProjectionContractVersion`、`ReadonlyTools` 等契约字段。
2. [ ] `internal/server/setid_explain_api.go`
   - 改为消费统一 PDP explain 结果。
3. [ ] `internal/server/internal_rules_evaluate_api.go`
   - 改为统一策略求值适配边界，而非手工拼装多套决议。
4. [ ] `internal/server/orgunit_create_field_decisions_api.go`
   - 改为直接消费统一字段决策结果。
5. [ ] `internal/server/orgunit_write_capabilities_api.go`
   - 改为消费统一 `effective_policy_version` 与字段可维护性语义。
6. [ ] `internal/server/orgunit_write_api.go`
   - 保留对外写接口契约；
   - 内部版本校验与前置解释切换为统一 PDP/Projection 消费。

### 6.4 保留级（不应因 OPA 被重写）

1. [ ] `tenant_field_configs` 静态元数据 SoT。
2. [ ] `SetID Strategy Registry` 动态策略主写与治理台。
3. [ ] `Policy Activation` 的版本与激活主链。
4. [ ] `OrgUnitWriteService -> DB Kernel` 提交边界。
5. [ ] Assistant 会话状态机、候选确认、风险门、commit adapter。

## 7. 实施步骤

### Phase 0：边界冻结与样板约束

1. [ ] 先更新 `DEV-PLAN-350` 的实施假设，明确“唯一 PDP 的实现方式可以由 OPA 承接，但策略平台四层事实源不变”。
2. [ ] 同步回写 `DEV-PLAN-360/360A`，明确 `361` 影响的是后端统一策略消费实现，而不是 LibreChat/LangGraph/backend 的产品分层角色。
3. [ ] 明确 OPA 第一阶段只承接字段动态策略求值；`Mutation Policy` 默认先作为独立输入源保留，不强制一步并入 Rego。
4. [ ] 明确第一阶段只迁移 `create_orgunit` 样板，避免大范围并发改造。

### Phase 1：建立 OPA 适配内核

1. [ ] 新增仓内 OPA 适配层，负责：
   - `PolicyContext` 输入适配；
   - 策略记录装载；
   - OPA 求值调用；
   - 输出映射为仓内 `Decision` 结构。
2. [ ] 保持原有 API / 写接口协议不变，先实现 PDP 引擎替换。
3. [ ] 以现有 `pkg/fieldpolicy/setid_strategy_pdp_test.go` 为回归基线，对齐原有求值结果与错误码。

### Phase 2：切断 Assistant 第二解释器

1. [ ] 将 `internal/server/assistant_create_policy_precheck.go` 改为只消费统一 `PrecheckProjection`。
2. [ ] 删除其中直接读取：
   - `SetID Strategy Registry`
   - `tenant_field_configs`
   - `resolved_setid` 拼装与字段特判
3. [ ] 保证 `FIELD_REQUIRED_VALUE_MISSING`、`PATCH_FIELD_NOT_ALLOWED` 等结论来自统一投影。

### Phase 3：收口写服务前置解释

1. [ ] 将 `modules/orgunit/services/orgunit_write_service.go` 中 create 场景的前置策略解释改为统一 PDP/Projection 消费。
2. [ ] 写服务保留最终 fail-closed 再校验，不把所有拒绝都下沉给 OPA。
3. [ ] 保证“Assistant dry-run”与“正式写链前置解释”对同一 `PolicyContext` 输出同一结论。

### Phase 4：扩展 explain 与只读工具

1. [ ] 将 explain API、rules evaluate API、field decisions API 统一切到 OPA/PDP 主链。
2. [ ] 承接 `DEV-PLAN-350`，使 `orgunit_action_precheck`、`orgunit_field_explain` 只读工具复用同一投影语义。
3. [ ] 在样板稳定后，再决定是否将八动作全部迁移。

## 8. 风险与取舍

1. [ ] 主要收益是减少规则求值器的实现复杂度，而不是减少策略平台治理复杂度；若团队误把 OPA 当作“平台替代品”，会导致方案跑偏。
2. [ ] OPA 会引入 Rego 学习成本与双栈调试成本；若最终只有极少量规则且变更不频繁，引入收益可能不足。
3. [ ] 若 OPA 只被 Assistant 使用，而写服务仍走另一套本地解释，则会形成新的旁路，不符合 `DEV-PLAN-330/350`。
4. [ ] 因此本计划要求：**要么不引入 OPA；一旦引入，就必须以“统一唯一 PDP”为目标，而不是新增一条专用解释支线。**

## 9. 验收标准

1. [ ] 已能明确说明 OPA 与本仓策略平台的关系：OPA 仅为唯一 PDP 候选引擎，不替代四层事实源与写链。
2. [ ] `pkg/fieldpolicy/setid_strategy_pdp.go` 的主求值职责已切换为统一 PDP 适配实现，原手写求值器不再继续膨胀。
3. [ ] `internal/server/assistant_create_policy_precheck.go` 不再直接访问底层策略 store 或 `tenant_field_configs`。
4. [ ] `modules/orgunit/services/orgunit_write_service.go` 不再独自维护一套与 Assistant 不同的 create 字段前置解释逻辑。
5. [ ] 同一 `PolicyContext` 下，Assistant dry-run、explain API、写服务前置解释得到同一字段决策、错误码与 `effective_policy_version`。

## 10. 测试与覆盖率

1. [ ] 覆盖率口径与统计范围遵循仓库现行 SSOT：`AGENTS.md`、`Makefile` 与 CI 门禁；本计划不复制具体脚本实现。
2. [ ] 若后续进入代码实施，至少需要补齐以下测试层：
   - PDP 适配层与现有样例的等价回归测试；
   - Assistant precheck 仅消费 Projection 的边界测试；
   - 写服务 fail-closed 再校验测试；
   - explain/version 对齐测试。
3. [ ] 在代码迁移前，不得通过扩大 coverage 排除项或降低阈值替代“删重复解释/统一 PDP”。

## 11. 交付物

1. [X] 方案文档：`docs/dev-plans/361-opa-pdp-adoption-boundary-and-migration-plan.md`
2. [ ] 若后续批准实施，应补：
   - OPA 适配层设计说明；
   - 迁移执行记录；
   - 与 `DEV-PLAN-350` 的联动修订记录。

## 12. 关联事实源

1. [X] `docs/dev-plans/330-strategy-module-architecture-and-design-convergence-plan.md`
2. [X] `docs/dev-plans/340-assistant-orgunit-duplicate-maintenance-investigation-and-convergence-plan.md`
3. [X] `docs/dev-plans/341-assistant-mainline-evolution-and-340-350-correlation-investigation.md`
4. [X] `docs/dev-plans/350-assistant-tooling-alignment-with-unified-policy-model-plan.md`
5. [X] `docs/dev-plans/360-librechat-depower-and-langgraph-langchain-layered-takeover-plan.md`
6. [X] `docs/dev-plans/360a-librechat-feature-disablement-and-runtime-cutover-plan.md`
7. [X] `pkg/fieldpolicy/setid_strategy_pdp.go`
8. [X] `modules/orgunit/services/orgunit_mutation_policy.go`
9. [X] `modules/orgunit/services/orgunit_write_service.go`
10. [X] `internal/server/assistant_create_policy_precheck.go`
