# DEV-PLAN-441：旧策略模块残余清理方案

**状态**: 规划中（2026-04-20 15:20 CST，作为 DEV-PLAN-440 的配套残余清理计划）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：在独立 `strategy` 模块已删除的前提下，清理当前仓库中仍残留的“旧策略模块”语义、命名、运行时壳层、测试资产与文档入口；凡命中 SetID 根删除范围的对象，排序与 owner 统一服从 `DEV-PLAN-440`。
- **关联模块/目录**：`pkg/fieldpolicy`、`modules/orgunit`、`internal/server`、`cmd/dbtool`、`docs/dev-plans`、`AGENTS.md`
- **关联计划/标准**：`AGENTS.md`、`docs/dev-plans/000-docs-format.md`、`docs/dev-plans/003-simple-not-easy-review-guide.md`、`docs/dev-plans/012-ci-quality-gates.md`、`docs/dev-plans/015-ddd-layering-framework.md`、`docs/archive/dev-plans/330-strategy-module-architecture-and-design-convergence-plan.md`
- **用户入口/触点**：策略 registry / explain / 字段决策运行时、`fieldpolicy` PDP、dbtool snapshot/validate 工具、主文档中的策略模块入口

### 0.1 Simple > Easy 三问

1. **边界**：本计划只清理“旧策略模块残余”，不承担 SetID 全量删除；涉及 SetID 根移除的对象由 `DEV-PLAN-440` 统一 owner 与排期。
2. **不变量**：仓库内不能再出现“已无独立模块，但代码/文档仍把它当模块”的语义漂移；不能保留第二套 PDP 或跨层重复决策实现。
3. **可解释**：清理后，维护者应能清楚回答“当前动态字段策略唯一 SoT、唯一 PDP、唯一 owner module 是谁”，且不再需要提“旧策略模块”。

### 0.2 现状研究摘要

- **现状实现**：
  - 独立 `modules/strategy` 已不存在。
  - 但 `pkg/fieldpolicy`、`setid_strategy_registry`、`setid_strategy.rego`、`SetIDStrategyFieldDecision`、`cmd/dbtool/orgunit_setid_strategy_registry_*` 仍保留强烈“旧策略模块”痕迹。
  - `AGENTS.md` 仍有 `DEV-PLAN-330` 入口，把“策略模块”作为活体概念暴露。
- **现状约束**：
  - 不能在清理旧术语时破坏仍然存活的现行动态策略链路。
  - 不能把“清理残余”做成纯 rename 而保留旧结构。
- **最容易出错的位置**：
  - `pkg/fieldpolicy` 与 `modules/orgunit` 的职责分界。
  - server API 与 dbtool 仍把 registry 当独立治理子系统。
  - 文档入口把“旧策略模块调查”继续当成主线事实源。
- **本次不沿用的“容易做法”**：
  - 仅改名不删跨层重复实现。
  - 保留 `setid_strategy_registry` / `fieldpolicy` 专有术语但宣称“模块已删”。
  - 继续让 `cmd/dbtool` 承担生产语义快照校验。

## 1. 背景与上下文

- `DEV-PLAN-330` 已完成当时的架构调查与收口，但当前仓库语义已经前进：独立策略模块不存在，继续保留“旧策略模块”术语会制造认知债务。
- 用户本次目标不是重做动态策略，而是**清理旧策略模块的残余**。
- 因此需要把“哪些是现行动态字段策略能力、哪些只是历史命名/工具/文档壳层”冻结清楚。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 清理“旧策略模块”术语、文档入口与代码命名残余。
- [ ] 合并或删除跨层重复的字段策略决策实现，只保留唯一 PDP 主链。
- [ ] 删除与独立策略模块时代绑定、且不再被 `440` 明确保留的 dbtool、snapshot、validate、compat 壳层。
- [ ] 将现行动态策略能力明确归位到当前 owner module / package，不再以“旧策略模块”表述。

### 2.2 非目标

- [ ] 本计划不单独承接 SetID 根删除；若对象同时属于 SetID 大链路，则由 `DEV-PLAN-440` 统筹并决定删改先后。
- [ ] 本计划不重新设计新的规则引擎能力边界。
- [ ] 本计划不额外增加新的策略治理抽象来替代旧残余。

### 2.3 用户可见性交付

- **用户可见入口**：最终用户不应看到“策略模块”这一概念；如仍有字段策略页面或 API，其表述应回归当前 owner module 的正式能力名称。
- **最小可操作闭环**：维护者通过文档地图与代码入口，只能找到“现行动态策略主链”，找不到“旧策略模块”作为第二概念层。

## 2.4 工具链与门禁

- **命中触发器（勾选）**：
  - [X] Go 代码
  - [ ] `apps/web/**` / presentation assets / 生成物
  - [ ] i18n（仅 `en/zh`）
  - [X] DB Schema / Migration / Backfill / Correction
  - [X] sqlc
  - [X] Routing / allowlist / responder
  - [ ] AuthN / Tenancy / RLS
  - [X] Authz（Casbin）
  - [X] 文档 / readiness / 证据记录
  - [X] 其他专项门禁：`ddd-layering-p0`、`ddd-layering-p2`

## 3. 架构与关键决策

### 3.1 统一判断

当前仓库里，“旧策略模块残余”主要分为 4 类：

1. **命名残余**：`setid_strategy_*`、`Strategy Registry`、`策略模块`
2. **结构残余**：跨层重复 PDP / resolver / decision builder
3. **工具残余**：`cmd/dbtool` 中针对策略 registry 的 snapshot / validate / bootstrap 辅助
4. **文档残余**：`AGENTS.md` 与 dev-plan 仍把其当活体模块

### 3.2 ADR 摘要

- **决策 1**：旧策略模块残余按“命名、结构、工具、文档”四类清理，不做仅 rename 的表面收口。
  - **备选 A**：仅更新文档，不动代码。
  - **备选 B**：仅删工具，不动运行时命名。
  - **选定理由**：只清一层会导致残余继续回流。

- **决策 2**：唯一 PDP 原则优先于兼容历史实现；同一裁决算法不允许长期跨 `pkg/fieldpolicy`、`internal/server`、`modules/orgunit` 多地并存。
  - **备选 A**：保留多处实现，以测试保证不漂移。
  - **选定理由**：这不是简单，而是把架构债务测试化。

## 4. 清理对象冻结

### 4.1 命名残余

1. [ ] `SetIDStrategyFieldDecision`
2. [ ] `setid_strategy.rego`
3. [ ] `setid_strategy_pdp.go`
4. [ ] `setid_strategy_registry_*` API / test / dbtool 文件名
5. [ ] 文档中“策略模块”作为现行实现概念的描述

### 4.2 结构残余

1. [ ] `pkg/fieldpolicy` 与 `orgunit` 持久化层的重复裁决逻辑
2. [ ] server 层仅为旧策略模块过渡期保留的 compat helper
3. [ ] 历史路由映射中与旧 registry 概念强绑定的条目

### 4.3 工具残余

1. [ ] `cmd/dbtool/orgunit_setid_strategy_registry_snapshot.go`
2. [ ] `cmd/dbtool/orgunit_setid_strategy_registry_validate.go`
3. [ ] `orgunit_snapshot_bootstrap` 中 `include-setid-strategy-registry` 之类参数与流程

### 4.4 文档残余

1. [ ] `AGENTS.md` 中将 `DEV-PLAN-330` 当作现行活体入口的表述
2. [ ] 仍把“策略模块”当成独立架构单元的 dev-plan 引用
3. [ ] readiness / records 中缺少“330 已完成但其术语已退役”的收口说明

## 5. 实施步骤

1. [ ] 在 `440` readiness 清单基础上，标注哪些命中点属于“SetID 删除完成后仍需继续清理的旧策略残余”。
2. [ ] 明确唯一 PDP 与唯一 owner module，删除重复裁决实现。
3. [ ] 清理 dbtool / bootstrap / validate / snapshot 旧工具，但不得抢跑 `440` 已冻结的 SetID 根删除顺序。
4. [ ] 清理 API、测试与历史路由映射中仍残留的旧命名。
5. [ ] 更新 `AGENTS.md` 与文档地图，将 `DEV-PLAN-330` 调整为历史架构调查入口，而非现行模块入口。
6. [ ] 形成 readiness 证据并补充“哪些对象已随 440 删除、哪些是 441 独立收尾”的封板说明。

## 6. 验收标准

1. [ ] 仓库中不再存在把“策略模块”当作现行独立模块的代码和文档描述。
2. [ ] 字段动态策略裁决只保留唯一主链。
3. [ ] `cmd/dbtool` 不再保留旧策略模块时代的 registry snapshot / validate 壳层；若该对象已被 `440` 一并删除，`441` 仅记录封板结果，不重复 owner。
4. [ ] `AGENTS.md` 文档地图不再制造“旧策略模块仍为现行模块”的认知。

## 7. 与 DEV-PLAN-440 的关系

- `441` 是“旧策略模块残余清理”。
- `440` 是“彻底删除 SetID”的唯一 PoR。
- 若某对象同时满足“旧策略模块残余”与“SetID 根语义”，以 `440` 为主计划，`441` 只负责描述其命名/结构/封板视角，不重复定义删除顺序、停止线或验收 owner。
- `441` 的实现顺序必须服从 `440` 的 Phase 0-4：未完成 `440` 的契约收口与根切断前，`441` 不得把 SetID 相关对象单独宣称为“已收口”。

## 8. 交付物

- `docs/dev-plans/441-legacy-strategy-module-residue-cleanup-plan.md`
- 后续对应 readiness 文档：`docs/dev-records/DEV-PLAN-441-READINESS.md`
