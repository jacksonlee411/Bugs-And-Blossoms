# DEV-PLAN-202：200蓝图 Phase 0 策略决议确定性与 allowed_value_codes 语义收敛

**状态**: 已完成（2026-02-28 15:10 UTC）

## 1. 背景与上下文
承接 DEV-PLAN-200 第 5.2 与 5.1A。目标是将策略冲突决议从“描述性规则”收敛为“可复算算法 + 可回放证据”。

## 2. 目标与非目标
### 2.1 核心目标
1. [X] 冻结候选过滤/分桶/特异度/优先级排序的确定性执行序列（对齐 DEV-PLAN-200 §5.2）。
2. [X] 冻结 `allowed_value_codes` 的“先层级后优先级”求值流程及子集约束（对齐 DEV-PLAN-200 §5.1A）。
3. [X] 冻结 `required/default/allowed` 三者一致性阻断规则与错误码。
4. [X] 输出冲突矩阵测试样例（同位冲突、空策略、非法组合）与回放证据格式。

### 2.2 非目标
1. [X] 不在本计划变更页面 IA 或交互层实现。
2. [X] 不扩展新策略 DSL，保持枚举治理（Simple > Easy）。

## 3. 对齐关系（与 DEV-PLAN-200）
- 对应 DEV-PLAN-200 的 Phase 0；里程碑映射：M0（冲突决议算法与 allowed 集合语义冻结）。
- 输入依赖：DEV-PLAN-201 输出的作用域/错误码基线。
- 后续输出依赖：DEV-PLAN-206/207。

### 3.1 标准对齐（DEV-PLAN-005）
- [X] `STD-002`：冲突决议输入上下文必须显式，禁止隐式默认维度。
- [X] `STD-004`：冲突无法化解时 fail-closed，不引入 legacy 分支。
- [X] `STD-011`：`policy_conflict_ambiguous` 等错误码到提示口径一一对应。

## 4. 关键设计（Simple > Easy）
1. [X] 单一事实源：同一语义仅一个主写层，不新增平行事实源。
2. [X] 显式不变量：边界、失败路径、状态转换可在 5 分钟内解释清楚。
3. [X] Fail-Closed：缺上下文/缺策略/版本冲突/权限不满足一律拒绝。
4. [X] No Legacy：不引入双链路、回退通道、兼容别名窗口。
5. [X] 规格先行：实现偏离本计划时，先更新计划再改代码。

## 5. 实施步骤
1. [X] 冻结冲突决议算法伪代码，并定义每步输入/输出与失败路径。
2. [X] 定义 `priority_mode + local_override_mode` 全矩阵（合法/非法）与 expected result。
3. [X] 定义 replay 证据结构：`winner_policy_ids + resolution_trace + policy_version`。
4. [X] 补齐单元测试与集成回放用例目录规范（引用现有门禁入口）。

## 6. 门禁与验证（SSOT 引用）
- 触发器与本地必跑矩阵：`AGENTS.md`
- 命令入口：`Makefile`
- CI 门禁：`.github/workflows/quality-gates.yml`
- 本计划预计命中门禁：
  - [X] `make test`（覆盖率门禁 100% 通过）
  - [X] `make check error-message`
  - [X] `make check doc`

## 7. 验收标准
1. [X] 同输入 `(tenant, capability, intent, as_of, setid)` 的输出可重复复算。
2. [X] `allowed_value_codes ⊆ candidate_pool` 恒成立；违约时稳定 fail-closed。
3. [X] `policy_conflict_ambiguous` 的触发条件明确且可测试。

## 8. 风险与缓解
1. [X] 实现使用不稳定排序键导致环境间漂移。缓解：重放一致性断言。
2. [X] 组合语义过于复杂。缓解：坚持有限枚举与非法组合硬拒绝。

## 9. 交付物与证据
- 证据归档：`docs/dev-records/dev-plan-200-m0-policy-resolution-evidence.md`、`docs/dev-records/dev-plan-200-m0-allowed-value-semantics-evidence.md`
- 交付物最小集：契约文档更新、自动化测试/门禁项、Readiness 证据记录。
- 本次执行记录：见上述 2 份证据文档的 2026-02-28 条目。

## 10. 文档完整性与 DEV-PLAN-003 对齐自检
1. [X] 已覆盖对应阶段目标、边界、不变量与失败路径。
2. [X] 已声明 Goals/Non-Goals、依赖关系、实施步骤、标准对齐与验收标准。
3. [X] 已包含门禁入口与证据归档路径（避免仅“能跑”不可审计）。
4. [X] 已落实 Simple > Easy：不新增多事实源、不引入 legacy 双链路。

## 11. 关联文档
- `docs/dev-plans/200-composable-building-block-architecture-blueprint.md`
- `docs/dev-plans/184-field-metadata-and-runtime-policy-sot-convergence.md`
