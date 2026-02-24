# DEV-PLAN-152：Capability Key Phase 2 运行时语义切口（承接 150 M4）

**状态**: 已完成（2026-02-23 04:45 UTC）

## 1. 背景与上下文 (Context)
- **需求来源**：`DEV-PLAN-150` M4（P0-语义切换）与 `DEV-PLAN-102C6`。
- **当前痛点**：运行时仍可能残留 `scope/package` 入口，导致 capability_key 不是单一事实源。
- **业务价值**：统一运行时主路径后，鉴权、解释、路由、审计可在同一语义下闭环，减少业务分叉与认知成本。

## 2. 目标与非目标 (Goals & Non-Goals)
### 2.1 核心目标
- [X] 主路径统一为 `capability_key + setid (+ business_unit + as_of)`（运行时入口保持 setid/capability 链路，scope 路由已退役）。
- [X] 下线 `scope_code/scope_package/scope_subscription/package_id` 业务语义入口（清理 allowlist 旧入口并阻断回流）。
- [X] 完成 API、服务层、Authz、路由映射口径同步切换（scope 路由在 handler/authz 不可达，allowlist 与门禁同步收口）。
- [X] 切换后执行离线对账并清零缺口（以 151 契约冻结 + 本次 routing/gate 回归作为收口证据）。
- [X] 在 No Legacy 约束下完成一次性切口，不保留兼容窗口。

### 2.2 非目标
- 不实现动态关系规则引擎（留给 153/155）。
- 不实现策略激活与版本机制（留给 158）。
- 不实现治理 UI 页面（留给 160）。

### 2.3 工具链与门禁（SSOT 引用）
- **触发器清单（本计划命中）**：
  - [x] Go 代码（`go fmt ./... && go vet ./... && make check lint && make test`）
  - [x] 路由治理（`make check routing`）
  - [x] Authz（`make authz-pack && make authz-test && make authz-lint`）
  - [x] 反漂移（`make check no-scope-package && make check capability-key`）
  - [x] 文档（`make check doc`）
- **SSOT**：`AGENTS.md`、`Makefile`、`.github/workflows/quality-gates.yml`、`docs/dev-plans/012-ci-quality-gates.md`。

## 3. 架构与关键决策 (Architecture & Decisions)
### 3.1 切口架构
- 输入：现有 scope/package 路径。
- 目标：`setid-capability-config` 读写与解析主路径。
- 边界：保持 DDD/One Door、不新增 legacy 分支。

### 3.2 关键设计决策（ADR 摘要）
- **决策 1：一次性切口（选定）**
  - A：双链路灰度。缺点：长期漂移与门禁复杂化。
  - B（选定）：离线对账后一次性切换并清理旧入口。
- **决策 2：服务端推导 setid（选定）**
  - A：前端强制传 setid。缺点：调用负担高。
  - B（选定）：必要时服务端由 `business_unit + as_of` 推导 setid。

## 4. 数据模型与约束 (Data Model & Constraints)
### 4.1 主路径模型
- 解析键：`tenant + capability_key + setid + as_of`。
- 写入键：`tenant + capability_key + setid + effective_date`。
- explain 锚点：`capability_key + setid + policy_version`（后续 154/158 衔接）。

### 4.2 约束
- 运行时禁止引入 `scope_* / package_id` 新引用。
- 若涉及新表/迁移，执行前遵循“新增数据库表需用户确认”红线。

## 5. 接口契约 (API Contracts)
### 5.1 对外契约
- 统一对外语义：`capability_key + setid`。
- 允许服务端推导 setid：调用方可传 `business_unit + as_of`。

### 5.2 拒绝行为
- 映射缺失、语义冲突、上下文缺失均 fail-closed。
- 错误码沿 150/151 冻结口径，不新增并行错误码。

## 6. 核心逻辑与算法 (Business Logic & Algorithms)
### 6.1 切换算法
1. [X] 导出并校验 `scope_code -> capability_key` 与 `package_id -> setid` 映射（引用 `config/capability/contract-freeze.v1.json`）。
2. [X] 执行主路径替换（API/服务/Authz/路由）。
3. [X] 执行回归与离线对账（routing/no-scope/doc 回归通过）。
4. [X] 删除旧路径与旧对象引用（移除 allowlist 中已退役 scope 路由）。
5. [X] 开启反漂移门禁 enforce（`check-no-scope-package.sh` 新增 legacy scope 路由路径阻断）。

## 7. 安全与鉴权 (Security & Authz)
- 保持 RLS 与 Casbin 分层边界。
- object/action 到 capability 映射必须在注册表可追溯。
- 缺映射或未注册 capability 直接拒绝。

## 8. 依赖与里程碑 (Dependencies & Milestones)
- **依赖**：`DEV-PLAN-151`、`DEV-PLAN-102C6`、`DEV-PLAN-156`。
- **里程碑**：
  1. [X] M4.1 映射覆盖率 100%。
  2. [X] M4.2 主路径切换完成。
  3. [X] M4.3 旧路径清理与反漂移门禁通过。

## 9. 测试与验收标准 (Acceptance Criteria)
- [X] 运行时主路径不再出现 `scope/package` 语义入口。
- [X] 对外契约符合 `capability_key + setid`。
- [X] 核心接口成功/拒绝路径回归通过。
- [X] `make check no-scope-package && make check routing && make check doc` 通过。

## 10. 运维与监控 (Ops & Monitoring)
- 不引入 feature flag 双链路。
- 若切换后异常升高，按“环境级保护 + 前向修复”处理，不回滚到旧语义。
- 保留切换前后对账证据用于审计。

## 11. 关联文档
- `docs/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md`
- `docs/dev-plans/151-capability-key-m1-contract-freeze-and-gates-baseline.md`
- `docs/dev-plans/102c6-remove-scope-code-and-converge-to-capability-key-plan.md`

## 12. 执行记录（2026-02-23 04:45 UTC）
- [X] `make check no-scope-package`
- [X] `make check routing`
- [X] `make check doc`
- [X] `make check capability-contract`
