# DEV-PLAN-212：200蓝图 Phase 6 评测门禁与触发式 Temporal 平台化验收

**状态**: 已完成（2026-02-28 21:20 UTC）

## 1. 背景与上下文
在 M10D0 基线后，先完成 planner/skill 回归门禁，再按触发条件执行 M10D1 生产级平台化验收。

## 2. 目标与非目标
### 2.1 核心目标
1. [X] 建立 planner 与 skill 固定样本评测门禁（成功率/拒绝准确率/人工接管率）。
2. [X] 建立 `risk_tier` 驱动审批策略与回归阈值。
3. [X] 冻结 M10D1 触发条件（预发/生产窗口或容量阈值触达）。
4. [X] 触发后完成 Temporal 生产级验收（HA/回放兼容/灾备演练）。

### 2.2 非目标
1. [X] 未触发条件时不强行推进生产级平台化。
2. [X] 不扩大业务范围到 200 之外模块。

## 3. 对齐关系（与 DEV-PLAN-200）
- 对应 DEV-PLAN-200 的 Phase 6；里程碑映射：M11/M11A/M10D1（触发式）。
- 输入依赖：DEV-PLAN-207/209/210/211。
- 输出：DEV-PLAN-200 全阶段收口。

### 3.1 标准对齐（DEV-PLAN-005）
- [X] `STD-008`：planner/skill 评测门禁接入 `make preflight` 与 CI required checks。
- [X] `STD-004`：未触发条件时不提前平台化，避免引入运行时双策略。
- [X] `STD-011`：评测失败/阈值不达标输出稳定拒绝原因与诊断字段。

## 4. 关键设计（Simple > Easy）
1. [X] 单一事实源：同一语义仅一个主写层，不新增平行事实源。
2. [X] 显式不变量：边界、失败路径、状态转换可在 5 分钟内解释清楚。
3. [X] Fail-Closed：缺上下文/缺策略/版本冲突/权限不满足一律拒绝。
4. [X] No Legacy：不引入双链路、回退通道、兼容别名窗口。
5. [X] 规格先行：实现偏离本计划时，先更新计划再改代码。

## 5. 实施步骤
1. [X] 构建 planner/skill 固定样本集与评分器，设置回归阈值。
2. [X] 把评测与审批策略接入 `make preflight`/CI required checks。
3. [X] 定义并公告 M10D1 触发条件与责任人机制。
4. [X] 触发后执行生产级平台化验收并归档演练证据。

## 6. 门禁与验证（SSOT 引用）
- 触发器与本地必跑矩阵：`AGENTS.md`
- 命令入口：`Makefile`
- CI 门禁：`.github/workflows/quality-gates.yml`
- 本计划预计命中门禁：
  - [X] `make preflight`
  - [X] `make test`
  - [X] `make check doc`

## 7. 验收标准
1. [X] M11/M11A 指标稳定在阈值内。
2. [X] M10D1 仅在触发条件命中后执行且证据完备。
3. [X] 200 蓝图所有阶段具备闭环证据。

## 8. 风险与缓解
1. [X] 评测样本偏置。缓解：分层样本集 + 定期扩样。
2. [X] 提前平台化增加运维负担。缓解：严格触发式执行。

## 9. 交付物与证据
- 证据归档：`docs/dev-records/dev-plan-200-m11-eval-gate-evidence.md`、`docs/dev-records/dev-plan-200-m11a-skill-eval-gate-evidence.md`、`docs/dev-records/dev-plan-200-m10d1-self-host-temporal-production-evidence.md`
- 交付物最小集：契约文档更新、自动化测试/门禁项、Readiness 证据记录。
- 本次执行记录：见 `docs/dev-records/dev-plan-200-m11-eval-gate-evidence.md`、`docs/dev-records/dev-plan-200-m11a-skill-eval-gate-evidence.md`、`docs/dev-records/dev-plan-200-m10d1-self-host-temporal-production-evidence.md` 的 2026-02-28 条目。

## 10. 文档完整性与 DEV-PLAN-003 对齐自检
1. [X] 已覆盖对应阶段目标、边界、不变量与失败路径。
2. [X] 已声明 Goals/Non-Goals、依赖关系、实施步骤、标准对齐与验收标准。
3. [X] 已包含门禁入口与证据归档路径（避免仅“能跑”不可审计）。
4. [X] 已落实 Simple > Easy：不新增多事实源、不引入 legacy 双链路。

## 11. 关联文档
- `docs/dev-plans/200-composable-building-block-architecture-blueprint.md`
- `docs/dev-plans/012-ci-quality-gates.md`
