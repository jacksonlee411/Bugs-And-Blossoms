# DEV-PLAN-156：Capability Key Phase 6 路由映射与复合门禁（承接 150 M3/M9）

**状态**: 规划中（2026-02-23 04:30 UTC）

## 1. 背景与上下文 (Context)
- **需求来源**：`DEV-PLAN-150` 工作流 E 与 M9（门禁与回归收口）。
- **当前痛点**：路由与 capability 映射若分散在模块代码中，容易出现缺映射、重复映射、未注册 key 使用。
- **业务价值**：建立单点映射与复合门禁后，可在 CI 早期阻断权限回漂。

## 2. 目标与非目标 (Goals & Non-Goals)
### 2.1 核心目标
- [ ] 建立“路由/动作 -> capability_key”单点注册表与持久化。
- [ ] 阻断缺映射、重复映射、未注册 key 使用。
- [ ] 将命名、注册、映射、禁词、契约检查接入 `make preflight`。
- [ ] 建立新增路由的 capability 准入流程（先登记后编码）。
- [ ] 输出全量回归清单与门禁例外审查机制。

### 2.2 非目标
- 不重写规则评估内核（留给 155）。
- 不承担功能域/激活状态机实现（留给 157/158）。

### 2.3 工具链与门禁（SSOT 引用）
- **触发器清单（本计划命中）**：
  - [x] Go 代码（`go fmt ./... && go vet ./... && make check lint && make test`）
  - [x] 路由治理（`make check routing`）
  - [x] Authz（`make authz-pack && make authz-test && make authz-lint`）
  - [x] capability 门禁（`make check capability-key`）
  - [x] 文档（`make check doc`）
- **SSOT**：`AGENTS.md`、`Makefile`、`.github/workflows/quality-gates.yml`。

## 3. 架构与关键决策 (Architecture & Decisions)
### 3.1 映射治理架构
- 注册层：维护 capability 注册表。
- 映射层：维护路由动作到 capability 的单点映射。
- 校验层：CI 执行复合门禁并阻断违规。

### 3.2 关键设计决策（ADR 摘要）
- **决策 1：集中映射（选定）**
  - A：模块内硬编码映射。缺点：审计困难。
  - B（选定）：集中映射表 + 校验脚本。
- **决策 2：先 shadow 后 enforce（选定）**
  - A：直接 enforce。缺点：初期误报影响交付。
  - B（选定）：短期 shadow 观测后转 enforce。

## 4. 数据模型与约束 (Data Model & Constraints)
### 4.1 映射最小字段
- `route_id` / `method`
- `action`
- `capability_key`
- `owner_module`
- `status`

### 4.2 约束
- 每个受保护路由必须映射且仅映射一个 capability_key。
- capability_key 必须在注册表存在并处于可用状态。

## 5. 接口契约 (API Contracts)
- 本计划不新增外部业务 API。
- 对内提供映射校验报告（用于 CI 日志与审查）。

## 6. 核心逻辑与算法 (Business Logic & Algorithms)
### 6.1 映射校验算法
1. [ ] 加载路由清单与 capability 注册表。
2. [ ] 匹配映射关系并检测缺失/重复/冲突。
3. [ ] 检测 capability 命名禁词与未注册使用。
4. [ ] 输出机器可读报告，CI 决策 pass/fail。

## 7. 安全与鉴权 (Security & Authz)
- 映射缺失时默认拒绝，不允许“临时放行”。
- 复用 Authz 工具链检查 capability 变更对策略的影响。
- 禁止运行时拼接 capability_key。

## 8. 依赖与里程碑 (Dependencies & Milestones)
- **依赖**：`DEV-PLAN-151`、`DEV-PLAN-155`、`DEV-PLAN-157`。
- **里程碑**：
  1. [ ] M3.1 映射注册表可用。
  2. [ ] M9.1 复合门禁接入 CI。
  3. [ ] M9.2 全量回归与例外清零。

## 9. 测试与验收标准 (Acceptance Criteria)
- [ ] 所有受保护路由均存在唯一 capability 映射。
- [ ] 未注册或冲突映射在 CI 阶段被阻断。
- [ ] 门禁报告可用于审计追溯。
- [ ] `make preflight` 通过。

## 10. 运维与监控 (Ops & Monitoring)
- 指标：映射缺失率、门禁误报率、违规 PR 阻断率。
- 故障处置：误报激增时进入 shadow 模式短期校准，不放宽核心 stopline。
- 证据：门禁报表与回归结果归档。

## 11. 关联文档
- `docs/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md`
- `docs/dev-plans/155-capability-key-m3-evaluation-context-cel-kernel.md`
- `docs/dev-plans/157-capability-key-m7-functional-area-governance.md`
