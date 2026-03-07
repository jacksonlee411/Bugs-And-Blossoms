# DEV-PLAN-289：260-M2~M4 实施收口专项（事实源推进 + 主链联调）

**状态**: 规划中（2026-03-07 23:59 CST）

## 1. 背景
1. [ ] `DEV-PLAN-260` 已完成 P0 契约冻结，但 `M2~M4` 仍涉及后端事实源推进、FSM guard 接线、与 `266/284` 承载面联调。
2. [ ] 该阶段若继续混在 `260` 主计划内推进，变更半径过大，不利于 `271` 双泳道并行节奏。

## 2. 目标与非目标
### 2.1 目标
1. [ ] 完成 `260-M2`：对话状态推进以后端持久化事实源为准，稳定重建 DTO。
2. [ ] 完成 `260-M3`：对话文案与模型链路按冻结语义输出，不引入前端语义补算。
3. [ ] 完成 `260-M4`：与 `266/284` 对齐“单通道 + 官方消息树唯一落点 + DTO-only 消费”主链联调。

### 2.2 非目标
1. [ ] 不做 Case 1~4 封板验收与证据归档（由 `DEV-PLAN-290` 承接）。
2. [ ] 不做升级兼容回归封板（由 `DEV-PLAN-291/285` 承接）。
3. [ ] 不重定义 `260` 的 FSM/DTO 契约正文。

## 3. 顺序与依赖
1. [ ] 前置：`DEV-PLAN-286/287/288` 已进入执行，保证 `266` 前置缺口有明确归属。
2. [ ] 并行：可与 `240-M2/M3` 并行推进，但不得引入未冻结字段语义。
3. [ ] 后置：本计划完成后，`DEV-PLAN-290` 才进入正式 Case 验收。

## 4. 实施步骤
1. [ ] 事实源收口：确认 `GET conversation` 可稳定重建 `phase/missing_fields/candidates/pending_draft_summary/selected_candidate_id/commit_reply/error_code`。
2. [ ] guard 收口：按 `260` 冻结阶段转移表校验 confirm/commit 约束，阻断越级提交。
3. [ ] 模型链路收口：确保回执语义与状态推进一致，不通过前端局部状态修正业务结果。
4. [ ] 主链联调：在 `/app/assistant/librechat` 验证 `260` 业务推进与 `266/284` 承载面行为一致。

## 5. 验收标准
1. [ ] `260-M2~M4` 对应任务全部完成且无前端业务重算回流。
2. [ ] DTO 重建与阶段推进在重试/恢复场景下语义一致。
3. [ ] 与 `266/284` 联调不出现双链路、双落点、外挂容器回流。

## 6. 测试与门禁（SSOT 引用）
1. [ ] 触发器与门禁命令以 `AGENTS.md`、`Makefile`、`docs/dev-plans/012-ci-quality-gates.md` 为准。
2. [ ] 文档改动至少通过 `make check doc`。

## 7. 交付物
1. [ ] 本计划文档：`docs/dev-plans/289-librechat-260-m2-m4-implementation-closure-plan.md`。
2. [ ] `260-M2~M4` 的联调记录与 stopline 通过证据。

## 8. 关联文档
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
- `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `docs/dev-plans/284-librechat-source-level-send-and-render-takeover-plan.md`
- `docs/dev-plans/290-librechat-260-m5-real-case-validation-and-evidence-plan.md`
- `AGENTS.md`
