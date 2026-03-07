# DEV-PLAN-290：260-M5 真实 Case 验收与证据固化专项

**状态**: 规划中（2026-03-07 23:59 CST）

## 1. 背景
1. [ ] `DEV-PLAN-260` 的最终通过依赖 Case 1~4 在真实入口完整闭环，且必须同时满足 `266` 共通 stopline。
2. [ ] `M5` 以验证和证据为主，若与实现改造混在同一计划中，容易出现“边改边验收”导致口径漂移。

## 2. 目标与非目标
### 2.1 目标
1. [ ] 在 `/app/assistant/librechat` 完成 Case 1~4 真实验收。
2. [ ] 固化每个 Case 的页面、网络、trace 与状态证据，形成可复核证据链。
3. [ ] 输出 `260` 收口结论，作为 `271-S5` 与 `285` 的输入。

### 2.2 非目标
1. [ ] 不承担 `M2~M4` 实现缺口修复（由 `DEV-PLAN-289` 回退处理）。
2. [ ] 不承担升级兼容回归实施（由 `DEV-PLAN-291/237` 承接）。
3. [ ] 不修改 `260` FSM/DTO 契约定义。

## 3. 顺序与依赖
1. [ ] 前置：`DEV-PLAN-289` 完成，`260-M2~M4` 无待修缺口。
2. [ ] 前置：`266` 剩余项由 `286/287/288` 覆盖并达到可验收状态。
3. [ ] 后置：本计划通过后方可进入 `285` 封板汇总。

## 4. 实施步骤
1. [ ] Case 1 验收：通道连通 + `266` 共通 stopline 同时成立。
2. [ ] Case 2 验收：草案 -> 确认 -> 提交顺序严格成立。
3. [ ] Case 3 验收：缺字段补全 -> 确认 -> 提交闭环成立。
4. [ ] Case 4 验收：多候选 -> 选择 -> 二次确认 -> 提交闭环成立。
5. [ ] 证据固化：每个 Case 保存截图、DOM 断言、请求日志、trace 及失败分支记录。
6. [ ] 执行日志：将本轮真实验收写入 `dev-plan-260` 相关执行记录，显式区分旧口径记录与新口径记录。

## 5. 验收标准
1. [ ] Case 1~4 全部通过，且每个 Case 均满足 `266` 共通 stopline。
2. [ ] 任一 Case 出现双链路、外挂回复、同轮多泡或官方原始错误体验即判失败。
3. [ ] 证据可追溯、可复核、可重复执行。

## 6. 测试与门禁（SSOT 引用）
1. [ ] 触发器与命令以 `AGENTS.md`、`Makefile`、`docs/dev-plans/012-ci-quality-gates.md` 为准。
2. [ ] 文档改动至少通过 `make check doc`。

## 7. 交付物
1. [ ] 本计划文档：`docs/dev-plans/290-librechat-260-m5-real-case-validation-and-evidence-plan.md`。
2. [ ] Case 1~4 验收记录与证据资产索引。
3. [ ] 面向 `285` 的 `260` 收口结论。

## 8. 关联文档
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
- `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `docs/dev-plans/285-librechat-cutover-regression-and-closure-plan.md`
- `docs/dev-plans/289-librechat-260-m2-m4-implementation-closure-plan.md`
- `AGENTS.md`
