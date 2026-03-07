# DEV-PLAN-291：237 升级兼容回归前置专项（供 285 封板复核）

**状态**: 规划中（2026-03-07 23:59 CST）

## 1. 背景
1. [ ] `DEV-PLAN-271` 的 `S5` 明确要求补齐 `237` 的 source/runtime compatibility 回归后，才能进入 `285` 封板。
2. [ ] 当前 `237` 范围覆盖升级流程与回归闭环，作为前置专项拆分可降低与 `260/266` 验收并行时的冲突风险。

## 2. 目标与非目标
### 2.1 目标
1. [ ] 形成可执行的升级兼容回归矩阵（source、patch stack、runtime、入口边界、关键交互路径）。
2. [ ] 固化升级回放流程与失败 stopline，确保“可预期、可回滚、可审计”。
3. [ ] 输出一组可直接供 `285` 使用的前置证据与复核清单。

### 2.2 非目标
1. [ ] 不在本计划内直接宣称封板完成（封板由 `DEV-PLAN-285` 承担）。
2. [ ] 不改写 `237` 主计划业务契约，仅做前置专项执行与证据收敛。
3. [ ] 不承担 `260` 业务 Case 验收。

## 3. 顺序与依赖
1. [ ] 可与 `260-M5` 验收并行准备，但最终通过判定应先于 `285` 启动。
2. [ ] 若回归发现旧入口、旧桥职责或双链路回流，必须回退到对应计划修复，不得带缺口进入封板。

## 4. 实施步骤
1. [ ] 冻结升级兼容矩阵：明确版本、patch、运行态组件和关键路径检查项。
2. [ ] 执行回放：按统一脚本/流程跑 source 与 runtime compatibility 回归。
3. [ ] 检查 stopline：无旧入口回流、无旧桥职责回流、无双入口/双回执语义。
4. [ ] 汇总产物：形成供 `285` 引用的前置通过报告与风险清单。

## 5. 验收标准
1. [ ] `237` 对应升级兼容回归项全部完成并可复核。
2. [ ] 关键失败路径具备稳定错误语义与证据记录。
3. [ ] 产物可直接作为 `285` 封板输入，不需在封板阶段补设计。

## 6. 测试与门禁（SSOT 引用）
1. [ ] 触发器与门禁命令以 `AGENTS.md`、`Makefile`、`docs/dev-plans/012-ci-quality-gates.md` 为准。
2. [ ] 文档改动至少通过 `make check doc`。

## 7. 交付物
1. [ ] 本计划文档：`docs/dev-plans/291-librechat-237-upgrade-compatibility-readiness-plan.md`。
2. [ ] 升级兼容回归矩阵与执行结果。
3. [ ] 面向 `285` 的前置通过清单与风险列表。

## 8. 关联文档
- `docs/dev-plans/237-librechat-upgrade-and-regression-closure-plan.md`
- `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `docs/dev-plans/285-librechat-cutover-regression-and-closure-plan.md`
- `AGENTS.md`
