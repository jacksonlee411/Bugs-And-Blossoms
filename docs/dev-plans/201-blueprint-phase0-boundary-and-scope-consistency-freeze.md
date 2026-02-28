# DEV-PLAN-201：200蓝图 Phase 0 边界冻结与跨层作用域一致性基线

**状态**: 规划中（2026-02-28 16:50 UTC）

## 1. 背景与上下文
承接 DEV-PLAN-200 第 2.2/2.2A/5.1A 的冻结项。目标是先把 `mapping_scope`、`tenant-only Dict`、`ResolveSetID` 三者关系定死，阻断后续实现期返工。

## 2. 目标与非目标
### 2.1 核心目标
1. [ ] 冻结 Surface/Intent/Capability 与 `mapping_scope` 术语口径，形成单一词汇表。
2. [ ] 冻结跨层作用域一致性矩阵（tenant/global mapping 与 tenant-only Dict 的命中与拒绝语义）。
3. [ ] 冻结字段级 explain 最小字段：`mapping_scope + resolved_setid + setid_source + data_scope_decision`。
4. [ ] 冻结 fail-closed 错误码口径（`mapping_missing/mapping_ambiguous/dict_baseline_not_ready/dict_value_not_found_as_of`）。

### 2.2 非目标
1. [ ] 不落地运行时代码与接口实现（仅完成契约冻结）。
2. [ ] 不引入任何 legacy 回退通道或兼容别名窗口。

## 3. 对齐关系（与 DEV-PLAN-200）
- 对应 DEV-PLAN-200 的 Phase 0；里程碑映射：M0/M1（术语与边界冻结、跨层作用域一致性冻结）。
- 输入依赖：DEV-PLAN-200（6.1/9.1/9.2）。
- 后续输出依赖：DEV-PLAN-203/204/206。

### 3.1 标准对齐（DEV-PLAN-005）
[ ] `STD-002`（as_of/上下文显式化）：作用域决议必须显式携带 `tenant + as_of + resolved_setid`。
[ ] `STD-004`（No Legacy）：禁止通过 global fallback 或双链路兜底弥补映射缺失。
[ ] `STD-011`（错误码明确化，承接 DEV-PLAN-140）：作用域失败路径输出稳定错误码与可解释字段。

## 4. 关键设计（Simple > Easy）
1. [ ] 单一事实源：同一语义仅一个主写层，不新增平行事实源。
2. [ ] 显式不变量：边界、失败路径、状态转换可在 5 分钟内解释清楚。
3. [ ] Fail-Closed：缺上下文/缺策略/版本冲突/权限不满足一律拒绝。
4. [ ] No Legacy：不引入双链路、回退通道、兼容别名窗口。
5. [ ] 规格先行：实现偏离本计划时，先更新计划再改代码。

## 5. 实施步骤
1. [ ] 盘点现有 `surface+intent -> capability_key` 及 scope 语义，生成差异清单（仅事实，不改代码）。
2. [ ] 编制跨层作用域矩阵（tenant/global mapping × tenant dict baseline 有/无），冻结 allow/deny 与错误码。
3. [ ] 定义 explain 字段契约与审计字段（含 `data_scope_decision`），补齐 machine-readable 示例。
4. [ ] 补充门禁映射：将 scope consistency 纳入 M0 证据路径与集成测试要求。

## 6. 门禁与验证（SSOT 引用）
- 触发器与本地必跑矩阵：`AGENTS.md`
- 命令入口：`Makefile`
- CI 门禁：`.github/workflows/quality-gates.yml`
- 本计划预计命中门禁：
  - [ ] `make check capability-route-map`
  - [ ] `make check capability-key`
  - [ ] `make check no-legacy`
  - [ ] `make check doc`

## 7. 验收标准
1. [ ] `mapping_scope` 不再被解释为数据读取权限，仅用于 capability 映射覆盖决议。
2. [ ] 命中 `global mapping` 时仍 tenant-only 取数；缺租户基线稳定返回 `dict_baseline_not_ready`。
3. [ ] 对应 200 的 M0 scope consistency 证据模板可直接执行。

## 8. 风险与缓解
1. [ ] 术语冻结不彻底，后续文档/代码继续混用 scope 概念。缓解：冻结词汇表并纳入评审模板。
2. [ ] 错误码语义漂移。缓解：给出错误码->场景映射表并在后续计划复用。

## 9. 交付物与证据
- 证据归档：`docs/dev-records/dev-plan-200-m0-scope-consistency-evidence.md`
- 交付物最小集：契约文档更新、自动化测试/门禁项、Readiness 证据记录。

## 10. 文档完整性与 DEV-PLAN-003 对齐自检
1. [X] 已覆盖对应阶段目标、边界、不变量与失败路径。
2. [X] 已声明 Goals/Non-Goals、依赖关系、实施步骤、标准对齐与验收标准。
3. [X] 已包含门禁入口与证据归档路径（避免仅“能跑”不可审计）。
4. [X] 已落实 Simple > Easy：不新增多事实源、不引入 legacy 双链路。

## 11. 关联文档
- `docs/dev-plans/200-composable-building-block-architecture-blueprint.md`
- `docs/dev-plans/003-simple-not-easy-review-guide.md`
