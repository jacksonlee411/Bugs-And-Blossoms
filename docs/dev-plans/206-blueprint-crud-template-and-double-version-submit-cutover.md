# DEV-PLAN-206：200蓝图 Phase 2 CRUD 模板统一与双版本提交收口

**状态**: 规划中（2026-02-28 16:50 UTC）

## 1. 背景与上下文
本计划把 6.1 读决议与 L3 写模板打通，确保 create/add/insert/correct 走同一提交链路，且无 legacy 回退。

## 2. 目标与非目标
### 2.1 核心目标
1. [ ] 统一 create/add/insert/correct 提交流程与校验顺序（值决议 -> required -> allowed -> 写入）。
2. [ ] 提交时强制校验 `policy_version + composition_version`，阻断 TOCTOU。
3. [ ] 冻结“先 ResolveSetID 再取数”的提交前置条件。
4. [ ] 按 No Legacy 原则完成单次切换与旧路径下线。

### 2.2 非目标
1. [ ] 不引入 AI 编排入口。
2. [ ] 不新增任何第二写入口。

## 3. 对齐关系（与 DEV-PLAN-200）
- 对应 DEV-PLAN-200 的 Phase 2；里程碑映射：M5/M6。
- 输入依赖：DEV-PLAN-203/204/205。
- 后续输出依赖：DEV-PLAN-207/210。

### 3.1 标准对齐（DEV-PLAN-005）
[ ] `STD-001`：统一提交命令命名使用 `request_id + trace_id`。
[ ] `STD-002`：提交链路上下文（`as_of/resolved_setid/intent`）显式传递。
[ ] `STD-004`：切换后不保留 runtime legacy 回退链路。

## 4. 关键设计（Simple > Easy）
1. [ ] 单一事实源：同一语义仅一个主写层，不新增平行事实源。
2. [ ] 显式不变量：边界、失败路径、状态转换可在 5 分钟内解释清楚。
3. [ ] Fail-Closed：缺上下文/缺策略/版本冲突/权限不满足一律拒绝。
4. [ ] No Legacy：不引入双链路、回退通道、兼容别名窗口。
5. [ ] 规格先行：实现偏离本计划时，先更新计划再改代码。

## 5. 实施步骤
1. [ ] 抽象统一提交模板与 intent 适配层，消除各动作分支重复实现。
2. [ ] 在提交链路注入双版本校验与冲突返回（fail-closed）。
3. [ ] 执行单次切换剧本：只读对照 -> 预发验收 -> 上线 -> 下线旧路径。
4. [ ] 补齐集成回归：版本冲突、上下文漂移、重试幂等等。

## 6. 门禁与验证（SSOT 引用）
- 触发器与本地必跑矩阵：`AGENTS.md`
- 命令入口：`Makefile`
- CI 门禁：`.github/workflows/quality-gates.yml`
- 本计划预计命中门禁：
  - [ ] `make check no-legacy`
  - [ ] `make test`
  - [ ] `make check error-message`
  - [ ] `make check doc`

## 7. 验收标准
1. [ ] 四类 intent 提交语义一致。
2. [ ] 版本冲突可复现且错误码稳定。
3. [ ] 上线后不存在 runtime 双链路。

## 8. 风险与缓解
1. [ ] 切换窗口故障。缓解：仅环境级保护（只读/停写）+ 修复后重试。
2. [ ] 动作语义被过度抽象。缓解：模板最小化，差异仅留策略层。

## 9. 交付物与证据
- 证据归档：`docs/dev-records/dev-plan-200-m5-version-consistency-evidence.md`、`docs/dev-records/dev-plan-200-m6-cutover-evidence.md`
- 交付物最小集：契约文档更新、自动化测试/门禁项、Readiness 证据记录。

## 10. 文档完整性与 DEV-PLAN-003 对齐自检
1. [X] 已覆盖对应阶段目标、边界、不变量与失败路径。
2. [X] 已声明 Goals/Non-Goals、依赖关系、实施步骤、标准对齐与验收标准。
3. [X] 已包含门禁入口与证据归档路径（避免仅“能跑”不可审计）。
4. [X] 已落实 Simple > Easy：不新增多事实源、不引入 legacy 双链路。

## 11. 关联文档
- `docs/dev-plans/200-composable-building-block-architecture-blueprint.md`
- `docs/dev-plans/004m1-no-legacy-principle-cleanup-and-gates.md`
