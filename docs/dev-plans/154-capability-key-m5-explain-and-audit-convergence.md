# DEV-PLAN-154：Capability Key Phase 4 Explain 与审计收敛（承接 150 M5）

**状态**: 已完成（2026-02-23 05:42 UTC）

## 1. 背景与上下文 (Context)
- **需求来源**：`DEV-PLAN-150` 工作流 C（Explain 与审计）与 M5 目标。
- **当前痛点**：成功/拒绝解释字段在不同链路可能不一致，排障与审计对账成本高。
- **业务价值**：统一 explain 合同后，用户、客服、审计可基于同一证据链定位问题并复盘。

## 2. 目标与非目标 (Goals & Non-Goals)
### 2.1 核心目标
- [X] 冻结 explain 最小字段（含 `trace_id/request_id/capability_key/setid/policy_version`）。
- [X] 冻结 `brief/full` 分级展示与权限边界。
- [X] 统一 success/deny 的 reason_code 与日志字段。
- [X] 形成“关键 deny 路径可回放证据”标准。
- [X] 与 157/158 的功能域与激活状态字段保持一致。

### 2.2 非目标
- 不承担动态关系求值实现（留给 153/155）。
- 不承担 UI 页面完整交付（留给 160）。

### 2.3 工具链与门禁（SSOT 引用）
- **触发器清单（本计划命中）**：
  - [x] Go 代码（`go fmt ./... && go vet ./... && make check lint && make test`）
  - [x] capability 契约门禁（`make check capability-contract && make check capability-key && make check no-scope-package`）
  - [x] Routing 门禁（`make check routing`）
  - [x] 错误提示门禁（`make check error-message`）
  - [x] 文档（`make check doc`）
- **SSOT**：`AGENTS.md`、`Makefile`、`docs/dev-plans/012-ci-quality-gates.md`。

## 3. 架构与关键决策 (Architecture & Decisions)
### 3.1 explain 分层架构
- 业务 API 默认返回 `brief`。
- 审计/管理员场景可请求 `full`。
- full explain 以结构化日志为主，不要求全部字段外露给普通用户。

### 3.2 关键设计决策（ADR 摘要）
- **决策 1：API 简版 + 日志完整版（选定）**
  - A：API 默认返回完整 explain。缺点：泄露面与负载过高。
  - B（选定）：API `brief`，日志 `full`，权限受控查看。
- **决策 2：拒绝码稳定化（选定）**
  - A：模块自定义错误文案。缺点：不可回归。
  - B（选定）：reason_code 固定，文案由前端映射。

## 4. 数据模型与约束 (Data Model & Constraints)
### 4.1 explain 最小字段
- `trace_id`
- `request_id`
- `capability_key`
- `setid`
- `functional_area_key`
- `policy_version`
- `decision`
- `reason_code`

### 4.2 约束
- 缺任一最小字段视为 explainability 失败。
- full explain 仅向授权角色开放。

## 5. 接口契约 (API Contracts)
### 5.1 explain 输出约定
- 业务 API：默认 `explain=brief`。
- 审计接口：支持受控 `explain=full`。
- 错误场景：返回稳定 `reason_code`，并提供 `trace_id/request_id`。

### 5.2 日志约定
- 统一结构化日志键：`decision/reason_code/capability_key/setid/policy_version`。
- 禁止输出过滤前敏感字段原值。

## 6. 核心逻辑与算法 (Business Logic & Algorithms)
### 6.1 explain 组装流程
1. [X] 采集判定输入与上下文摘要。
2. [X] 执行决议并生成 reason_code。
3. [X] 输出 brief（API）与 full（日志）两份结构。
4. [X] 记录审计索引键，支持后续对账。

## 7. 安全与鉴权 (Security & Authz)
- full explain 按最小授权原则访问。
- 对外响应不回显敏感内部规则细节。
- 审计日志受租户与角色双重约束。

## 8. 依赖与里程碑 (Dependencies & Milestones)
- **依赖**：`DEV-PLAN-151`、`DEV-PLAN-153`、`DEV-PLAN-102C3`。
  - **里程碑**：
  1. [X] M5.1 explain 合同冻结。
  2. [X] M5.2 success/deny 样板链路对账通过。
  3. [X] M5.3 审计检索与回放证据归档。

## 9. 测试与验收标准 (Acceptance Criteria)
- [X] success/deny 均满足 explain 最小字段。
- [X] `brief/full` 分级行为与权限控制正确。
- [X] deny 场景 reason_code 稳定可回归。
- [X] `make check capability-contract && make check capability-key && make check no-scope-package && make check routing && make check doc` 通过。

## 10. 运维与监控 (Ops & Monitoring)
- 指标：explain 缺失率、deny 比例、审计检索成功率。
- 异常处置：发现 explain 缺失优先阻断发布并补齐字段。
- 证据：对账样例与日志检索截图归档到 `docs/dev-records/`。

## 11. 关联文档
- `docs/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md`
- `docs/dev-plans/102c3-setid-configuration-hit-explainability.md`
- `docs/dev-plans/157-capability-key-m7-functional-area-governance.md`
- `docs/dev-plans/158-capability-key-m6-policy-activation-and-version-consistency.md`

## 12. 执行记录（2026-02-23 05:42 UTC）
- [X] `go test ./internal/server -run "TestHandleSetIDExplainAPI|TestTraceIDFromRequestHeader|TestNormalizeSetIDExplainRequestID|TestFallbackSetIDExplainTraceID|TestResolveFunctionalAreaKey"`
- [X] `make check capability-contract`
- [X] `make check capability-key`
- [X] `make check no-scope-package`
- [X] `make check routing`
- [X] `make check error-message`
- [X] `make check doc`
