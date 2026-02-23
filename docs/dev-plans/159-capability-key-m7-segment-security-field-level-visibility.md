# DEV-PLAN-159：Capability Key Phase 9 字段级分段安全（承接 150 M7）

**状态**: 已完成（2026-02-23 15:58 UTC）

## 1. 背景与上下文 (Context)
- **需求来源**：`DEV-PLAN-150` 工作流 H（Segment-Based Security）。
- **当前痛点**：路由级拦截无法满足“同源对象不同字段可见性差异”场景。
- **业务价值**：字段级分段安全可在不拆 API 的前提下实现敏感字段最小暴露。

## 2. 目标与非目标 (Goals & Non-Goals)
### 2.1 核心目标
- [X] 在序列化/BFF 层实现字段级可见/隐藏/脱敏策略。
- [X] 复用 `EvaluationContext + CEL` 产出字段决策。
- [X] 输出字段级 explain reason_code，支持审计追踪。
- [X] 冻结日志红线：禁止记录过滤前敏感原值。
- [X] 给出 P2 下推（SQL Projection / Field Mask）评估基线。

### 2.2 非目标
- 不在本期强制数据层下推实现。
- 不承担页面治理交付（留给 160）。

### 2.3 工具链与门禁（SSOT 引用）
- **触发器清单（本计划命中）**：
  - [x] Go 代码（`go fmt ./... && go vet ./... && make check lint && make test`）
  - [x] 文档（`make check doc`）
  - [x] 总收口（`make preflight`）
- **SSOT**：`AGENTS.md`、`Makefile`、`docs/dev-plans/012-ci-quality-gates.md`。

## 3. 架构与关键决策 (Architecture & Decisions)
### 3.1 字段级安全架构
- 判定层：根据上下文与 capability 产出字段决策。
- 执行层：序列化阶段应用隐藏/脱敏。
- 审计层：输出字段级决策与 reason_code。

### 3.2 关键设计决策（ADR 摘要）
- **决策 1：先 BFF/序列化层闭环（选定）**
  - A：直接做 SQL 下推。缺点：实施成本高。
  - B（选定）：先形成可用闭环，再评估下推。
- **决策 2：日志红线先行（选定）**
  - A：先做可见性，后补日志治理。缺点：泄露风险。
  - B（选定）：功能与日志红线同步上线。

## 4. 数据模型与约束 (Data Model & Constraints)
### 4.1 字段决策模型
- `field_key`
- `decision`（visible/hidden/masked）
- `reason_code`
- `mask_strategy`（可选）
- `policy_version`

### 4.2 约束
- `visible=false` 且 `required=true` 视为策略冲突。
- 脱敏策略必须可重复执行且可审计。

## 5. 接口契约 (API Contracts)
### 5.1 响应行为
- 同一 API 在不同上下文下返回不同字段集。
- 被隐藏字段使用“不可见/脱敏”占位，不回传原值。

### 5.2 explain 行为
- `brief` 输出字段差异摘要。
- `full` 输出字段级决策详情（受权限控制）。

## 6. 核心逻辑与算法 (Business Logic & Algorithms)
### 6.1 字段过滤算法
1. [X] 根据上下文计算字段决策。
2. [X] 对原始对象执行隐藏/脱敏转换。
3. [X] 生成字段级 explain 摘要。
4. [X] 输出安全响应并记录审计字段。

## 7. 安全与鉴权 (Security & Authz)
- 字段级策略判定失败时 fail-closed。
- 日志/监控严禁输出过滤前敏感值。
- 与功能域开关、激活版本保持一致。

## 8. 依赖与里程碑 (Dependencies & Milestones)
- **依赖**：`DEV-PLAN-154`、`DEV-PLAN-155`、`DEV-PLAN-157`、`DEV-PLAN-158`。
- **里程碑**：
  1. [X] M7.1 字段策略执行器最小闭环。
  2. [X] M7.2 关键对象差异返回回归通过。
  3. [X] M7.3 日志红线门禁通过。

## 9. 测试与验收标准 (Acceptance Criteria)
- [X] 至少 1 个关键对象实现字段差异返回。
- [X] 字段级 reason_code 可解释且可审计。
- [X] 日志中无过滤前敏感字段原值。
- [X] `make preflight` 通过。

## 10. 运维与监控 (Ops & Monitoring)
- 监控项：字段隐藏命中率、脱敏命中率、日志违规告警。
- 异常处置：出现敏感日志泄露立即阻断发布并修复。
- 证据：字段级回归与日志扫描结果归档。

## 11. 关联文档
- `docs/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md`
- `docs/dev-plans/154-capability-key-m5-explain-and-audit-convergence.md`
- `docs/dev-plans/160-capability-key-m8-m10-ui-delivery-and-evidence-closure.md`

## 12. 执行记录（2026-02-23 15:58 UTC）
- [X] `setid-explain` 接入字段级分段安全输出：新增 `visibility/mask_strategy/masked_default_value`，并引入 `FIELD_MASKED_IN_CONTEXT`。
- [X] 字段隐藏与脱敏场景不再回传 `resolved_default_value` 原值（隐藏和 `mask://` 规则统一返回 `***` 占位）。
- [X] 新增审计摘要函数 `briefSetIDFieldDecisions`，日志仅保留字段决策摘要，不记录敏感原值。
- [X] 新增回归测试覆盖 visible/hidden/masked 三类决策与日志红线：`setid_explain_api_test.go`。
- [X] 已执行：`go vet ./... && make check lint && make check routing && make check capability-key && make check doc && make test`。
