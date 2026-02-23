# DEV-PLAN-151：Capability Key Phase 1 契约冻结与门禁基线（承接 150 M1）

**状态**: 规划中（2026-02-23 04:05 UTC）

## 1. 背景与上下文 (Context)
- **需求来源**：`DEV-PLAN-150` 的 M1（契约冻结）与 P0 目标。
- **当前痛点**：150 与 102 系列虽已基本收口，但仍存在“字段/术语/门禁口径分散在多文档”的风险，后续实现易出现“实现先行、契约回补”的反复。
- **业务价值**：先冻结单一语义与门禁基线，可降低后续 152-160 的返工概率，确保 capability_key 机制按统一口径落地。

## 2. 目标与非目标 (Goals & Non-Goals)
### 2.1 核心目标
- [ ] 冻结替换矩阵：`scope_code -> capability_key`、`package_id -> setid`。
- [ ] 冻结分层语义：`domain_capability/process_capability` 与 `StaticContext/ProcessContext` 边界。
- [ ] 冻结 Functional Area 词汇表与生命周期（`active/reserved/deprecated`）。
- [ ] 冻结错误码与 explain 最小字段口径（含 `policy_version`）。
- [ ] 冻结门禁基线：命名、注册、映射、禁词、反漂移、CI required checks。
- [ ] 形成“冲突口径清零表”，作为 152-160 的统一输入。

### 2.2 非目标
- 不做运行时代码切换与旧路径清理（留给 152）。
- 不做规则内核与动态关系实现（留给 153/155）。
- 不做 UI 页面交付（留给 160）。

### 2.3 工具链与门禁（SSOT 引用）
- **触发器清单（本计划命中）**：
  - [x] 文档变更（`make check doc`）
  - [x] 路由治理口径引用（`make check routing`，用于门禁清单冻结）
  - [x] Authz 口径引用（`make authz-pack && make authz-test && make authz-lint`）
  - [x] capability 反漂移口径（`make check no-scope-package && make check capability-key`）
- **SSOT**：`AGENTS.md`、`Makefile`、`.github/workflows/quality-gates.yml`、`docs/dev-plans/012-ci-quality-gates.md`。

## 3. 架构与关键决策 (Architecture & Decisions)
### 3.1 契约冻结架构（文档 SSOT）
- 150 作为总装 PoR，151 负责“术语、字段、错误码、门禁”四类契约冻结。
- 102C1/102C2/102C3/102D 作为执行细则文档，只能引用冻结结果，不再自定义并行口径。

### 3.2 关键设计决策（ADR 摘要）
- **决策 1：先冻结再编码（选定）**
  - 选项 A：边实现边修文档。缺点：跨计划漂移高。
  - 选项 B（选定）：先冻结合同与门禁，再进入 152+ 实施。
- **决策 2：门禁前置（选定）**
  - 选项 A：实现后补门禁。缺点：回漂不可控。
  - 选项 B（选定）：在 M1 冻结门禁准入，后续 PR 直接受 CI 约束。

## 4. 数据模型与约束 (Data Model & Constraints)
### 4.1 契约字段清单（冻结）
- 业务主键：`capability_key`（能力语义）+ `setid`（配置归属）。
- 上下文字段：`business_unit`、`actor_scope`、`as_of`。
- 分层字段：`capability_type`（`domain_capability/process_capability`）。
- 功能域字段：`functional_area_key`、`lifecycle_status`。
- 激活字段：`policy_version`、`activation_state`（`draft/active`）。

### 4.2 约束
- `capability_key` 禁上下文编码、禁动态拼接。
- 每个 `capability_key` 必须且仅能归属一个 `functional_area_key`。
- `reserved` 功能域禁止接入运行时路由映射。

## 5. 接口契约 (API Contracts)
- 本计划不新增业务 API。
- 输出物是“契约清单与门禁清单”，供 152-160 直接消费。
- 对外统一约束：后续 API 契约必须遵循 `capability_key + setid` 主口径。

## 6. 核心逻辑与算法 (Business Logic & Algorithms)
### 6.1 契约冻结执行流程
1. [ ] 汇总 150 与 102 系列口径差异。
2. [ ] 对每个冲突项指定唯一 SSOT（150 或指定子计划）。
3. [ ] 冻结字段、错误码、门禁规则并形成清零表。
4. [ ] 用 `make check doc` 校验后发布为实施输入。

## 7. 安全与鉴权 (Security & Authz)
- 151 不改鉴权实现，但冻结鉴权输入边界与 fail-closed 原则。
- 明确 RLS 与 Casbin 边界不变：RLS 圈地，Authz 判定能力动作。
- 冻结拒绝码分类，避免前后端自由文本漂移。

## 8. 依赖与里程碑 (Dependencies & Milestones)
- **依赖**：`DEV-PLAN-150`、`DEV-PLAN-102C6`、`DEV-PLAN-102D`。
- **里程碑**：
  1. [ ] M1.1 差异清单完成。
  2. [ ] M1.2 契约与字段口径冻结完成。
  3. [ ] M1.3 门禁清单接入方案冻结。

## 9. 测试与验收标准 (Acceptance Criteria)
- [ ] 文档层冲突清零（无并行权威定义）。
- [ ] 术语/字段/错误码在 150 与 102 系列一致。
- [ ] 门禁清单可映射到 Makefile 与 CI required checks。
- [ ] `make check doc` 通过。

## 10. 运维与监控 (Ops & Monitoring)
- 不引入运行时开关与双链路。
- 若发现契约冲突，先停实施推进，回到 151 修订后再继续。
- 证据输出到后续 `docs/dev-records/`（由 160 统一收口）。

## 11. 关联文档
- `docs/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md`
- `docs/dev-plans/102c6-remove-scope-code-and-converge-to-capability-key-plan.md`
- `docs/dev-plans/102d-context-rule-evaluation-engine-on-top-of-102-foundation.md`
