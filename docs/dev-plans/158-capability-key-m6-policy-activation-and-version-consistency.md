# DEV-PLAN-158：Capability Key Phase 8 策略激活与版本一致性（承接 150 M6）

**状态**: 规划中（2026-02-23 04:40 UTC）

## 1. 背景与上下文 (Context)
- **需求来源**：`DEV-PLAN-150` 工作流 G 与 M6。
- **当前痛点**：策略变更若“即时散射生效”，在多实例与缓存场景下会出现鉴权分裂。
- **业务价值**：建立激活协议后，可实现“待发布、原子切换、可回滚”的企业级授权变更流程。

## 2. 目标与非目标 (Goals & Non-Goals)
### 2.1 核心目标
- [ ] 建立租户级 `draft/active + policy_version` 状态模型。
- [ ] 实现激活事务：pending -> active 原子切换。
- [ ] 建立缓存失效与刷新机制，以 `(tenant, policy_version)` 为一致性锚点。
- [ ] 建立回滚协议与审计证据链。
- [ ] 验证在途请求在切换窗口内不产生分裂结果。

### 2.2 非目标
- 不承担 capability 映射与门禁实现（留给 156）。
- 不承担 UI 页面交付（留给 160）。

### 2.3 工具链与门禁（SSOT 引用）
- **触发器清单（本计划命中）**：
  - [x] Go 代码（`go fmt ./... && go vet ./... && make check lint && make test`）
  - [x] 文档（`make check doc`）
  - [x] 总收口（`make preflight`）
- **SSOT**：`AGENTS.md`、`Makefile`、`docs/dev-plans/012-ci-quality-gates.md`。

## 3. 架构与关键决策 (Architecture & Decisions)
### 3.1 激活协议架构
- 草稿层：变更先进入 `draft`。
- 激活层：事务提交后切换 `active policy_version`。
- 缓存层：按版本失效与刷新，确保实例一致。

### 3.2 关键设计决策（ADR 摘要）
- **决策 1：版本锚点（选定）**
  - A：仅时间戳切换。缺点：对账困难。
  - B（选定）：显式 `policy_version` 作为一致性与审计锚点。
- **决策 2：激活事务（选定）**
  - A：逐节点异步刷新。缺点：窗口期结果不一致。
  - B（选定）：先事务切换，再统一刷新读侧。

## 4. 数据模型与约束 (Data Model & Constraints)
### 4.1 状态模型
- `activation_state`：`draft` / `active`
- `policy_version`：租户级单调版本号
- `activated_at` / `activated_by`
- `rollback_from_version`

### 4.2 约束
- 同一租户任一时刻只能有一个 active 版本。
- draft 版本不得参与运行时判定。
- 回滚必须保留链路证据。

## 5. 接口契约 (API Contracts)
### 5.1 内部激活接口（示意）
- `POST /internal/policies/activate`
- 输入：`tenant_id`, `target_policy_version`, `operator`
- 输出：`active_policy_version`, `activation_id`

### 5.2 回滚接口（示意）
- `POST /internal/policies/rollback`
- 输出：回滚后的 active 版本与审计引用。

## 6. 核心逻辑与算法 (Business Logic & Algorithms)
### 6.1 激活算法
1. [ ] 校验目标版本处于 draft。
2. [ ] 开启事务并写入激活记录。
3. [ ] 原子切换 active 版本。
4. [ ] 发布缓存失效信号并等待关键实例确认。
5. [ ] 输出激活结果与审计索引。

## 7. 安全与鉴权 (Security & Authz)
- 激活/回滚仅授权管理员可执行。
- 所有策略变更操作必须记录 `who/when/version`。
- 激活失败默认不切换，不允许部分生效。

## 8. 依赖与里程碑 (Dependencies & Milestones)
- **依赖**：`DEV-PLAN-151`、`DEV-PLAN-155`、`DEV-PLAN-157`。
- **里程碑**：
  1. [ ] M6.1 状态机与版本模型冻结。
  2. [ ] M6.2 激活事务联调通过。
  3. [ ] M6.3 缓存一致性与回滚回归通过。

## 9. 测试与验收标准 (Acceptance Criteria)
- [ ] 激活前后行为可预测且可追踪。
- [ ] 多实例无版本分裂。
- [ ] 回滚流程可执行且证据完整。
- [ ] `make preflight` 通过。

## 10. 运维与监控 (Ops & Monitoring)
- 监控项：激活成功率、版本分裂告警、回滚次数。
- 故障处置：激活失败立即保持旧 active 并触发告警。
- 证据：激活/回滚日志与回归报告归档。

## 11. 关联文档
- `docs/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md`
- `docs/dev-plans/157-capability-key-m7-functional-area-governance.md`
- `docs/dev-plans/160-capability-key-m8-m10-ui-delivery-and-evidence-closure.md`
