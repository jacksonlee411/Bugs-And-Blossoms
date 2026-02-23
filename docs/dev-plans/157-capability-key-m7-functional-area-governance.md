# DEV-PLAN-157：Capability Key Phase 7 Functional Area 治理落地（承接 150 M7）

**状态**: 已完成（2026-02-23 07:00 UTC）

## 1. 背景与上下文 (Context)
- **需求来源**：`DEV-PLAN-150` 工作流 F 与 M7。
- **当前痛点**：仅细粒度 capability 管理会导致模块级启停成本高，缺少租户可操作的上层治理开关。
- **业务价值**：Functional Area 提供“模块级能力治理”，便于租户启停、灰度与错误隔离。

## 2. 目标与非目标 (Goals & Non-Goals)
### 2.1 核心目标
- [X] 落地 Functional Area 词汇表（`functional_area_key/display_name/owner_module/lifecycle_status`）。
- [X] capability_key 与功能域建立唯一归属关系。
- [X] 建立功能域开关继承模型（`functional_area -> capability_key`）。
- [X] 功能域关闭或 `reserved` 状态下全链路 fail-closed（API/internal）。
- [X] 拒绝码与 explain 对齐：`FUNCTIONAL_AREA_MISSING/DISABLED/NOT_ACTIVE`。

### 2.2 非目标
- 不承担激活事务与版本一致性实现（留给 158）。
- 不承担字段级分段安全（留给 159）。

### 2.3 工具链与门禁（SSOT 引用）
- **触发器清单（本计划命中）**：
  - [x] Go 代码（`go fmt ./... && go vet ./... && make check lint && make test`）
  - [x] capability 门禁（`make check capability-key`）
  - [x] 路由治理（`make check routing`）
  - [x] 文档（`make check doc`）
- **SSOT**：`AGENTS.md`、`Makefile`、`.github/workflows/quality-gates.yml`。

## 3. 架构与关键决策 (Architecture & Decisions)
### 3.1 分层治理架构
- 词汇层：Functional Area 注册与生命周期定义。
- 归属层：capability 与功能域唯一绑定。
- 执行层：功能域开关继承到 capability 判定。

### 3.2 关键设计决策（ADR 摘要）
- **决策 1：唯一归属（选定）**
  - A：一个 capability 可多域归属。缺点：审计复杂。
  - B（选定）：一个 capability 仅归属一个功能域。
- **决策 2：reserved 默认阻断（选定）**
  - A：reserved 可试运行。缺点：行为不确定。
  - B（选定）：reserved 禁止运行时映射与启用。

## 4. 数据模型与约束 (Data Model & Constraints)
### 4.1 词汇表模型
- `functional_area_key`（唯一稳定键）
- `display_name`
- `owner_module`
- `lifecycle_status`（`active/reserved/deprecated`）

### 4.2 约束
- capability 缺失功能域归属时 fail-closed。
- 词汇表禁止别名并存。

## 5. 接口契约 (API Contracts)
### 5.1 功能域开关合同
- 租户级开关可查看、可审计。
- 关闭后下游 capability 自动失效。

### 5.2 错误码合同
- 功能域缺失：`FUNCTIONAL_AREA_MISSING`
- 功能域关闭：`FUNCTIONAL_AREA_DISABLED`
- 功能域未激活：`FUNCTIONAL_AREA_NOT_ACTIVE`

## 6. 核心逻辑与算法 (Business Logic & Algorithms)
### 6.1 功能域判定算法
1. [X] 根据 capability_key 查询功能域归属。
2. [X] 校验生命周期与租户开关状态。
3. [X] 若不满足则输出功能域拒绝码。
4. [X] 满足时进入后续 capability 规则判定。

## 7. 安全与鉴权 (Security & Authz)
- 功能域判定属于 Authz 前置门，失败即拒绝。
- 不绕过 RLS 边界，不放宽租户隔离。
- 功能域状态变更需审计留痕。

## 8. 依赖与里程碑 (Dependencies & Milestones)
- **依赖**：`DEV-PLAN-151`、`DEV-PLAN-156`、`DEV-PLAN-158`。
- **里程碑**：
  1. [X] M7.1 词汇表冻结与注册。
  2. [X] M7.2 capability 归属矩阵补齐。
  3. [X] M7.3 功能域开关执行链路回归通过。

## 9. 测试与验收标准 (Acceptance Criteria)
- [X] 功能域词汇表唯一、合法、生命周期正确。
- [X] capability 归属完整且唯一。
- [X] 功能域关闭后 capability 全量失效。
- [X] reserved 功能域不可运行时启用。

## 10. 运维与监控 (Ops & Monitoring)
- 指标：功能域开关命中率、功能域拒绝率、错误码分布。
- 故障处置：错误关闭导致误伤时，按激活协议回滚（158）。
- 证据：功能域变更审计记录归档。

## 11. 关联文档
- `docs/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md`
- `docs/dev-plans/156-capability-key-m3-m9-route-capability-mapping-and-gates.md`
- `docs/dev-plans/158-capability-key-m6-policy-activation-and-version-consistency.md`

## 12. 执行记录（2026-02-23 07:00 UTC）
- [X] 新增 `internal/server/functional_area_governance.go`，实现 capability->functional_area 唯一归属、生命周期判定与租户级开关继承模型。
- [X] `setid-explain` 与 `internal/rules/evaluate` 接入功能域 fail-closed 判定，拒绝码统一为 `FUNCTIONAL_AREA_MISSING/FUNCTIONAL_AREA_DISABLED/FUNCTIONAL_AREA_NOT_ACTIVE`。
- [X] `config/capability/route-capability-map.v1.json` 补充 capability 激活语义字段（`activation_state/current_policy_version`），作为后续 158 的版本锚点输入。
- [X] 新增/扩展测试：`functional_area_governance_test.go`、`setid_explain_api_test.go`、`internal_rules_evaluate_api_test.go`、`capability_route_registry_test.go`。
- [X] `go test ./internal/server -run "TestFunctionalAreaSwitchStore|TestEvaluateFunctionalAreaGate|TestHandleSetIDExplainAPI|TestHandleInternalRulesEvaluateAPI|TestCapabilityRouteRegistryContract|TestAuthzRequirementForRoute" -count=1`
