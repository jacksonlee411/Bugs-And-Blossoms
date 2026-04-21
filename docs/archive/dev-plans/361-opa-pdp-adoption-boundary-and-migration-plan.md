# DEV-PLAN-361：OPA 作为唯一 PDP 候选引擎的引入边界与迁移方案

**状态**: 修订中（2026-04-12 CST）

**本次修订目的**

1. [X] 先消除 `DEV-PLAN-361` 与 `DEV-PLAN-350` 的职责漂移，再启动代码实施。
2. [X] 将 `361` 冻结为“唯一 PDP 引擎替换 + parity 基线”，不再承载 `PrecheckProjection`、Assistant precheck、write service consumer convergence。
3. [X] 在同一文档批次同步 `DEV-PLAN-350/360A` 联动边界，避免 runtime cutover、consumer convergence 与 PDP 实现细节继续混写。

## 1. 背景

1. [X] `DEV-PLAN-330` 已冻结本仓“配置与策略”统一模型，要求所有策略裁决回到：
   `原始请求上下文 -> Context Resolver -> PolicyContext -> 唯一 PDP -> explain/version`。
2. [X] `DEV-PLAN-340/341` 已确认当前主要问题不是第二写入口，而是 Assistant 与正式写链之外，正在形成“第二策略解释器”。
3. [X] 当前仓内已经存在一套自研 PDP 雏形：
   - `pkg/fieldpolicy/setid_strategy_pdp.go` 负责字段动态策略命中、bucket 排序、fallback 与 explain trace；
   - `internal/server/setid_explain_api.go`、`internal/server/internal_rules_evaluate_api.go`、`internal/server/orgunit_create_field_decisions_api.go` 与 `modules/orgunit/infrastructure/persistence/orgunit_pg_store.go` 均依赖该入口或其语义；
   - `internal/server/assistant_create_policy_precheck.go` 与 `modules/orgunit/services/orgunit_write_service.go` 仍存在后续统一消费需求，但这些收口工作应移交 `DEV-PLAN-350`。
4. [X] 因而本计划要解决的不是“是否重写策略平台”，而是“是否用 OPA/Rego 替换唯一 PDP 的求值实现，并建立稳定 parity 基线”，以便后续 `350` 在不再扩张第二解释器的前提下收口 consumer。

## 2. 问题定性

### 2.1 当前复杂度来源

1. [X] 策略平台治理复杂度已经存在且合理，主要包括：
   - `tenant_field_configs` 静态元数据；
   - `SetID Strategy Registry` 动态策略主写；
   - `OrgUnit Mutation Policy` 写动作约束；
   - `Policy Activation` 版本与激活。
2. [X] 当前真正持续膨胀的是“规则求值复杂度”，表现为：
   - 唯一 PDP 的 bucket、fallback、mode matrix、默认值与 explain trace 仍由手写 Go 代码长期维护；
   - explain / evaluate / field-decisions / persistence parity 需要依赖同一求值核心才能稳定；
   - 若在引擎替换前继续推进 Assistant / write service 收口，会把“求值器替换”和“consumer convergence”缠在一起，扩大漂移面。

### 2.2 OPA 在本仓的正式定位

1. [X] OPA 不是本仓“配置与策略”模块本身。
2. [X] OPA 的正式定位是：**唯一 PDP 的进程内 Rego SDK 求值引擎**。
3. [X] 本轮形态冻结为：
   - 进程内 Rego SDK；
   - 不做 Wasm；
   - 不做 sidecar；
   - 不保留长期 compat 开关；
   - 不把引擎替换膨胀成“先搭规则平台”。
4. [X] 即使引入 OPA，下列边界仍属于仓内策略平台，不由 OPA 替代：
   - 配置写入口；
   - schema / migration / RLS；
   - 策略治理台与激活链路；
   - `OrgUnitWriteService -> DB Kernel` 写链；
   - Assistant 事务编排、候选确认与 commit gate。

## 3. 目标与非目标

### 3.1 目标

1. [ ] 冻结 `361` 的唯一职责：OPA 仅承接唯一 PDP 求值，不改变策略平台四层事实源与 One Door 写链。
2. [ ] 以 `create_orgunit` 所依赖的 SetID Strategy PDP 为唯一样板，完成手写 Go PDP 到 Rego-backed PDP adapter 的生产替换。
3. [ ] 为 `DEV-PLAN-350` 提供稳定的 PDP adapter 边界：上游只暴露统一 `fieldpolicy.Resolve(...) -> Decision/explain/version` 契约，不把 consumer convergence 混入 `361`。
4. [ ] 建立 parity 基线，确保现有 explain/evaluate/create-field-decisions 与 OrgUnit PG store 在替换后继续共享同一裁决结果、错误码与 trace 语义。

### 3.2 非目标

1. [ ] 本轮不交付 `PrecheckProjection`，该 contract 与消费链收口由 `DEV-PLAN-350` 承接。
2. [ ] 本轮不修改 `internal/server/assistant_create_policy_precheck.go`。
3. [ ] 本轮不修改 `modules/orgunit/services/orgunit_write_service.go` 的前置解释路径。
4. [ ] 本轮不合并 `modules/orgunit/services/orgunit_mutation_policy.go` 到 Rego 或统一投影。
5. [ ] 本轮不扩展 `assistant_action_registry.go`、不推进 Assistant/tool/write service/explain 的统一消费。
6. [ ] 本轮不做 Wasm、不做 sidecar、不保留长期 compat 引擎开关。
7. [ ] 本轮不新增数据库表；若后续确需额外快照字段或持久化产物，必须另立计划获批。

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

1. [ ] Go 侧只负责：
   - `Context Resolver` 规范化输入；
   - 装载字段策略事实源；
   - 调用进程内 Rego evaluator；
   - 将结果映射为统一 `Decision / explain / version` 输出。
2. [ ] 生产路径中，唯一允许承接 Rego 求值的入口是 `pkg/fieldpolicy.Resolve(...)`；外部调用方签名保持不变。
3. [ ] `PrecheckProjection` 被正式标注为 `DEV-PLAN-350` 的 downstream contract，而不是 `361` 的交付物。
4. [ ] `361` 完成后，下游 consumer 可以继续基于稳定 PDP adapter 收口，但收口动作本身不属于本计划。

## 5. 复杂度收益评估

### 5.1 预期减少的复杂度

1. [ ] 减少 bucket 排序、fallback、default 解析、mode matrix 与 trace 在手写 Go 求值器中的长期维护成本。
2. [ ] 减少 explain / evaluate / field-decisions / persistence 之间因引用不同实现而发生的 parity 风险。
3. [ ] 减少未来继续在 Go 中扩张规则解释器细节的冲动，让后续 consumer convergence 只依赖稳定 PDP adapter。

### 5.2 不会减少的复杂度

1. [ ] 不会减少策略平台治理复杂度：事实源、激活、RLS、治理台、版本发布仍需仓内维护。
2. [ ] 不会减少业务上下文解析复杂度：`Context Resolver` 仍是本仓业务知识。
3. [ ] 不会减少最终写链与 fail-closed 复杂度：写服务与 DB Kernel 仍必须独立承担最终拒绝职责。
4. [ ] 会新增 Rego 学习与调试成本；因此必须把范围收敛到“唯一 PDP 引擎替换”，不混入 `350` 的 downstream 收口。

## 6. 迁移范围分层

### 6.1 唯一替换级（生产实现替换）

1. [ ] `pkg/fieldpolicy/setid_strategy_pdp.go`
   - 当前是手写 Go PDP 主实现；
   - `361` 只替换这一处生产求值内核；
   - 替换后保持 `Resolve(...)` 等对外签名不变，由其内部统一转入 Rego-backed evaluator。

### 6.2 Parity 验证级（必须自动吃到同一内核）

1. [ ] `modules/orgunit/infrastructure/persistence/orgunit_pg_store.go`
   - 通过 `ResolveSetIDStrategyFieldDecision(...)` 验证持久层已消费同一 `fieldpolicy.Resolve(...)`。
2. [ ] `internal/server/setid_explain_api.go`
   - 验证 explain 端点经同一 Rego-backed PDP 输出 explain/version。
3. [ ] `internal/server/internal_rules_evaluate_api.go`
   - 验证内部 evaluate 面与唯一 PDP 保持等价。
4. [ ] `internal/server/orgunit_create_field_decisions_api.go`
   - 验证 create field decisions 面复用同一裁决核心。

### 6.3 下游移出级（由 DEV-PLAN-350 承接，不属于 361）

1. [ ] `internal/server/assistant_create_policy_precheck.go`
2. [ ] `modules/orgunit/services/orgunit_write_service.go`
3. [ ] `modules/orgunit/services/orgunit_mutation_policy.go`
4. [ ] `internal/server/assistant_action_registry.go`
5. [ ] `internal/server/orgunit_write_capabilities_api.go`
6. [ ] `internal/server/orgunit_write_api.go`
7. [ ] 上述文件涉及的 `PrecheckProjection`、Assistant precheck、write service 前置解释、Mutation Policy 合并、ActionSchema 扩展与统一消费链改造，统一移交 `DEV-PLAN-350`。

### 6.4 保留级（不应因 361 被重写）

1. [ ] `tenant_field_configs` 静态元数据 SoT。
2. [ ] `SetID Strategy Registry` 动态策略主写与治理台。
3. [ ] `Policy Activation` 的版本与激活主链。
4. [ ] `OrgUnitWriteService -> DB Kernel` 提交边界。
5. [ ] Assistant 会话状态机、候选确认、risk gate、commit adapter。
6. [ ] 数据库 schema；`361` 不新增表、不新增长期兼容产物。

## 7. 实施步骤

### Phase 0：文档冻结

1. [ ] 修订 `DEV-PLAN-361`，明确本计划只承接“唯一 PDP 引擎替换 + parity 基线”。
2. [ ] 同批回写 `DEV-PLAN-350`，明确其依赖已完成的 Rego-backed PDP adapter 后再启动 `PrecheckProjection` 与 consumer convergence。
3. [ ] 同批回写 `DEV-PLAN-360A`，明确后端统一策略消费主链由 `350/361` 承接，`360A` 不再定义 PDP 实现细节。
4. [ ] 先运行 `make check doc`；只有文档门禁通过后，才允许进入代码实施。

### Phase 1：OPA/Rego 内核接入

1. [ ] 在 `go.mod` 引入 `github.com/open-policy-agent/opa/rego`。
2. [ ] 固定新增：
   - `pkg/fieldpolicy/rego/setid_strategy.rego`
   - `pkg/fieldpolicy/opa_engine.go`
3. [ ] 使用嵌入式加载 Rego 规则，并以 `sync.Once` 缓存编译结果。
4. [ ] 保持 `pkg/fieldpolicy.Resolve(...)` 现有签名不变；生产路径只允许通过该入口调用 Rego evaluator。
5. [ ] 在替换生产逻辑前，把当前手写 Go 求值算法复制到 `pkg/fieldpolicy/*_test.go` 的 test-only oracle 中做等价回归；生产代码不做长期双跑。

### Phase 2：Parity Cutover 与回归

1. [ ] 将 `pkg/fieldpolicy.Resolve(...)` 的生产内部实现切到 Rego-backed evaluator。
2. [ ] 不改外部调用方协议；仅验证以下现有链路自动吃到同一内核：
   - `setid_explain`
   - `internal_rules_evaluate`
   - `orgunit_create_field_decisions`
   - `OrgUnitPGStore.ResolveSetIDStrategyFieldDecision(...)`
3. [ ] 按固定最小回归运行：
   `go test ./pkg/fieldpolicy ./internal/server/... ./modules/orgunit/infrastructure/persistence/... ./modules/orgunit/services/...`
4. [ ] 若 parity 不成立，优先修复 PDP adapter 或映射层；`350` 相关 downstream 工作不得提前并入本批次。

## 8. 风险与取舍

1. [ ] 主要收益是替换唯一 PDP 引擎，而不是减少策略平台治理复杂度；若误把 OPA 当作“平台替代品”，方案会跑偏。
2. [ ] Rego 会引入学习成本与调试成本；因此本轮只做最小充分替换，不同时推进 Projection 或 consumer convergence。
3. [ ] 若在 `361` 中顺手改 Assistant / write service，会把 parity 问题与消费链问题缠在一起，放大回归面。
4. [ ] 若保留长期 compat 开关、Wasm 或 sidecar 作为“以后再说”的退路，会把唯一 PDP 替换重新变成多实现并存，不符合本计划目标。

## 9. 验收标准

1. [ ] 生产主链不再使用手写 Go PDP 作为主求值实现，`pkg/fieldpolicy.Resolve(...)` 已切到 Rego-backed evaluator。
2. [ ] 现有 server explain/evaluate/create-field-decisions 与 `OrgUnitPGStore.ResolveSetIDStrategyFieldDecision(...)` 均经同一 Rego-backed `fieldpolicy.Resolve(...)` 输出结果。
3. [ ] parity 重点项保持一致：bucket 顺序、fallback、mode matrix、allowed values、default 解析、trace。
4. [ ] 以下错误码与失败语义保持回归通过：`policy_missing`、`policy_conflict_ambiguous`、`FIELD_DEFAULT_RULE_MISSING`、`policy_mode_invalid`、`policy_mode_combination_invalid`。
5. [ ] 本轮未引入 Wasm、sidecar、长期 compat 开关，也未新增数据库表。
6. [ ] `DEV-PLAN-350` 相关 `PrecheckProjection`、Assistant precheck、write service consumer cutover 未被错误并入 `361`。

## 10. 测试与覆盖率

1. [ ] 覆盖率口径与统计范围遵循仓库现行 SSOT：`AGENTS.md`、`Makefile` 与 CI 门禁；本计划不复制具体脚本实现。
2. [ ] `361` 只要求以下测试层：
   - PDP parity 等价回归测试；
   - existing consumer regression；
   - OrgUnit PG store regression。
3. [ ] parity 必测重点：
   - bucket 顺序；
   - fallback；
   - mode matrix；
   - allowed values；
   - default 解析；
   - trace；
   - `policy_missing`、`policy_conflict_ambiguous`、`FIELD_DEFAULT_RULE_MISSING`、`policy_mode_invalid`、`policy_mode_combination_invalid`。
4. [ ] `PrecheckProjection`、Assistant snapshot、tool 输出统一消费测试移交 `DEV-PLAN-350`，不作为 `361` 通过条件。
5. [ ] 在代码迁移前，不得通过扩大 coverage 排除项或降低阈值替代“切唯一 PDP 引擎 + 建立 parity 基线”。

## 11. 交付物

1. [X] 修订后的方案文档：`docs/dev-plans/361-opa-pdp-adoption-boundary-and-migration-plan.md`
2. [ ] 若实施获批，应补：
   - `pkg/fieldpolicy/rego/setid_strategy.rego`
   - `pkg/fieldpolicy/opa_engine.go`
   - `pkg/fieldpolicy/*_test.go` 中的 test-only oracle 与 parity 回归
   - 执行记录与验证证据

## 12. 关联事实源

1. [X] `docs/archive/dev-plans/330-strategy-module-architecture-and-design-convergence-plan.md`
2. [X] `docs/dev-plans/340-assistant-orgunit-duplicate-maintenance-investigation-and-convergence-plan.md`
3. [X] `docs/dev-plans/341-assistant-mainline-evolution-and-340-350-correlation-investigation.md`
4. [X] `docs/dev-plans/350-assistant-tooling-alignment-with-unified-policy-model-plan.md`
5. [X] `docs/dev-plans/360a-librechat-feature-disablement-and-runtime-cutover-plan.md`
6. [X] `pkg/fieldpolicy/setid_strategy_pdp.go`
7. [X] `internal/server/setid_explain_api.go`
8. [X] `internal/server/internal_rules_evaluate_api.go`
9. [X] `internal/server/orgunit_create_field_decisions_api.go`
10. [X] `modules/orgunit/infrastructure/persistence/orgunit_pg_store.go`
