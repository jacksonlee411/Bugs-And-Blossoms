# DEV-PLAN-153：Capability Key Phase 3 上下文化授权与动态关系（承接 150 M2/M5）

**状态**: 已完成（2026-02-23 06:30 UTC）

## 1. 背景与上下文 (Context)
- **需求来源**：`DEV-PLAN-150` M2（高风险 Spike）与 M5（授权收敛）。
- **当前痛点**：仅角色判定无法覆盖跨层级组织关系，易出现“角色正确但上下文错误仍放行”。
- **业务价值**：引入动态关系与时态判定后，可对齐 Workday 的 constrained security 核心能力。

## 2. 目标与非目标 (Goals & Non-Goals)
### 2.1 核心目标
- [X] 冻结并落地 `subject/domain/object/action + context` 判定输入。
- [X] 服务端权威回填上下文，冲突输入 fail-closed。
- [X] 支持 `actor.manages(target, as_of)` 等动态关系函数。
- [X] 冻结“CEL 执行期禁止 DB IO”，关系数据预加载到 `EvaluationContext`。
- [X] 完成 deny reason_code 与前端提示、explain 对齐。
- [X] 建立动态关系性能预算（p95、查询次数上限）与回归门禁。

### 2.2 非目标
- 不实现通用规则内核编译缓存（留给 155）。
- 不实现字段级分段安全（留给 159）。
- 不实现策略激活协议（留给 158）。

### 2.3 工具链与门禁（SSOT 引用）
- **触发器清单（本计划命中）**：
  - [x] Go 代码（`go fmt ./... && go vet ./... && make check lint && make test`）
  - [x] Authz（`make authz-pack && make authz-test && make authz-lint`）
  - [x] 路由治理（`make check routing`）
  - [x] 文档（`make check doc`）
- **SSOT**：`AGENTS.md`、`Makefile`、`.github/workflows/quality-gates.yml`。

## 3. 架构与关键决策 (Architecture & Decisions)
### 3.1 授权执行链
- 基础授权（Casbin）-> 上下文约束 -> 动态关系判定 -> 拒绝码/explain 输出。
- 关系求值依赖预加载关系集，不允许执行期回源查询。

### 3.2 关键设计决策（ADR 摘要）
- **决策 1：动态关系函数显式接收 as_of（选定）**
  - A：按当前组织关系判定。缺点：历史补录错判。
  - B（选定）：`actor.manages(target, as_of)`，按历史关系判定。
- **决策 2：执行期禁 IO（选定）**
  - A：在 CEL 里动态查库。缺点：延迟不可控。
  - B（选定）：构建 `EvaluationContext` 时一次性预加载关系集。

## 4. 数据模型与约束 (Data Model & Constraints)
### 4.1 关系上下文模型
- 输入最小集：`tenant_id`、`actor_id`、`business_unit`、`setid`、`as_of`。
- 预加载关系集示例：`managed_org_ids`、`subordinate_actor_ids`。
- 输出字段：`decision`、`reason_code`、`trace_id`、`request_id`。

### 4.2 约束
- `as_of` 必填，缺失即拒绝。
- 关系集预加载失败、冲突或不完整时 fail-closed。

## 5. 接口契约 (API Contracts)
### 5.1 鉴权输入合同
- 外部接口不允许直接注入完整 context。
- 服务端负责回填 `setid/business_unit/actor_scope`。

### 5.2 错误码合同
- 维持 150/151 冻结口径：上下文缺失、上下文冲突、关系不满足必须有稳定 reason_code。

## 6. 核心逻辑与算法 (Business Logic & Algorithms)
### 6.1 动态关系判定算法
1. [X] 解析并校验基础上下文。
2. [X] 预加载 `as_of` 时点关系集。
3. [X] 执行基础授权与上下文匹配。
4. [X] 执行 `actor.manages(target, as_of)` 等函数判定。
5. [X] 生成 `decision/reason_code` 与 explain 摘要。

## 7. 安全与鉴权 (Security & Authz)
- RLS 继续负责租户边界，153 不放宽任何跨租户判定。
- 动态关系失败默认拒绝，禁止静默降级到“仅角色通过”。
- 内部日志记录最小必要字段，不回显敏感原值。

## 8. 依赖与里程碑 (Dependencies & Milestones)
- **依赖**：`DEV-PLAN-151`、`DEV-PLAN-152`、`DEV-PLAN-102C1`。
- **里程碑**：
  1. [X] M2.1 Spike：时态一致性 + 性能预算 + 禁 IO 通过。
  2. [X] M5.1 上下文化授权在关键写链路 enforce。
  3. [X] M5.2 deny 回归与 explain 对齐完成。

## 9. 测试与验收标准 (Acceptance Criteria)
- [X] role 正确但 context 错误必须拒绝。
- [X] `actor.manages(target, as_of)` 在历史补录场景命中正确。
- [X] CEL 执行路径无 DB IO。
- [X] 满足鉴权 p95 性能预算。

## 10. 运维与监控 (Ops & Monitoring)
- 监控项：鉴权拒绝率、关系预加载失败率、鉴权延迟 p95。
- 故障处置：进入只读/停写保护后前向修复，不回退 legacy 逻辑。
- 将 Spike 压测报告纳入 `docs/dev-records/`。

## 11. 关联文档
- `docs/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md`
- `docs/dev-plans/151-capability-key-m1-contract-freeze-and-gates-baseline.md`
- `docs/dev-plans/102c1-setid-contextual-security-model.md`

## 12. 执行记录（2026-02-23 06:30 UTC）
- [X] `go test ./internal/server -run "TestResolveCapabilityContext|TestCapabilityDynamicRelations|TestHandleSetIDExplainAPI|TestHandleSetIDStrategyRegistryAPI"`
- [X] `make check capability-contract`
- [X] `make check capability-key`
- [X] `make check no-scope-package`
- [X] `make check routing`
- [X] `make check doc`
