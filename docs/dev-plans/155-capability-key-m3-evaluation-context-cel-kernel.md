# DEV-PLAN-155：Capability Key Phase 5 EvaluationContext + CEL 内核基座（承接 150 M3）

**状态**: 规划中（2026-02-23 04:25 UTC）

## 1. 背景与上下文 (Context)
- **需求来源**：`DEV-PLAN-150` 工作流 D 与 M3。
- **当前痛点**：规则评估能力分散在业务实现中，缺统一执行框架与冲突决议规则。
- **业务价值**：统一 `EvaluationContext + CEL` 内核后，能力决策可复用、可测试、可解释。

## 2. 目标与非目标 (Goals & Non-Goals)
### 2.1 核心目标
- [ ] 冻结 `EvaluationContext` 合同与服务端回填边界。
- [ ] 落地 CEL 编译缓存、执行器、冲突决议、解释器闭环。
- [ ] 支持 `StaticContext/ProcessContext` 双上下文分层。
- [ ] 第一阶段仅开放 `/internal` 评估入口，不对外暴露上下文伪造入口。
- [ ] 至少完成 2 条样板链路（资格过滤 + 命中决策）。

### 2.2 非目标
- 不承担功能域开关与激活状态机实现（留给 157/158）。
- 不承担字段级分段安全落地（留给 159）。

### 2.3 工具链与门禁（SSOT 引用）
- **触发器清单（本计划命中）**：
  - [x] Go 代码（`go fmt ./... && go vet ./... && make check lint && make test`）
  - [x] 路由治理（`make check routing`，校验 internal 入口边界）
  - [x] 文档（`make check doc`）
- **SSOT**：`AGENTS.md`、`Makefile`、`.github/workflows/quality-gates.yml`。

## 3. 架构与关键决策 (Architecture & Decisions)
### 3.1 执行架构
- `API/Service` 构建 `EvaluationContext`。
- `Repository` 粗筛候选规则。
- `CEL Engine` 执行表达式并输出中间解释。
- `Resolver` 按冻结排序规则决策最终命中。

### 3.2 关键设计决策（ADR 摘要）
- **决策 1：内部入口优先（选定）**
  - A：直接开放通用评估外部 API。缺点：上下文伪造风险。
  - B（选定）：先 internal-only，业务接口调用内核。
- **决策 2：编译缓存（选定）**
  - A：每次请求即时编译。缺点：延迟波动高。
  - B（选定）：表达式编译缓存 + 版本失效机制。

## 4. 数据模型与约束 (Data Model & Constraints)
### 4.1 内核核心对象
- `EvaluationContext`：`tenant_id/actor/setid/business_unit/as_of/capability_key/...`
- `RuleCandidate`：`rule_id/priority/effective_range/status/version`
- `EvaluationResult`：`decision/reason_code/selected_rule/explain`

### 4.2 约束
- 上下文字段由服务端回填，不接受客户端覆盖。
- 规则排序固定，禁止业务代码私自改写优先级逻辑。

## 5. 接口契约 (API Contracts)
### 5.1 internal 评估接口
- `POST /internal/rules/evaluate`
- 输入：最小业务参数 + 显式日期；上下文由服务端推导。
- 输出：`selected_rule`、`decision`、`brief_explain`。

### 5.2 业务接口接入原则
- 外部 API 不直接暴露通用规则评估入口。
- 业务接口通过 capability 映射调用内核并输出业务态结果。

## 6. 核心逻辑与算法 (Business Logic & Algorithms)
### 6.1 评估算法
1. [ ] 校验并构建 `EvaluationContext`。
2. [ ] 按 tenant/module/date 粗筛候选规则。
3. [ ] 执行 CEL（编译缓存命中优先）。
4. [ ] 进行冲突决议（priority + tie-break）。
5. [ ] 生成 explain 并返回结果。

## 7. 安全与鉴权 (Security & Authz)
- internal 入口限制在服务内网/内部路由。
- 上下文冲突直接 fail-closed。
- 规则执行与日志输出遵循最小泄露原则。

## 8. 依赖与里程碑 (Dependencies & Milestones)
- **依赖**：`DEV-PLAN-151`、`DEV-PLAN-153`、`DEV-PLAN-102D`。
- **里程碑**：
  1. [ ] M3.1 内核合同冻结。
  2. [ ] M3.2 两条样板链路联调完成。
  3. [ ] M3.3 internal 边界与安全测试通过。

## 9. 测试与验收标准 (Acceptance Criteria)
- [ ] 同输入同日期结果稳定（可重放）。
- [ ] 编译缓存命中/失效路径可测。
- [ ] 冲突决议稳定且可解释。
- [ ] 外部路由无法直接调用 internal 评估入口。

## 10. 运维与监控 (Ops & Monitoring)
- 指标：评估延迟、缓存命中率、规则执行失败率。
- 故障处置：失败率升高时启用环境级保护并前向修复。
- 证据：样板链路压测与回放报告归档。

## 11. 关联文档
- `docs/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md`
- `docs/dev-plans/102d-context-rule-evaluation-engine-on-top-of-102-foundation.md`
- `docs/dev-plans/153-capability-key-m2-m5-contextual-authz-and-dynamic-relations.md`
