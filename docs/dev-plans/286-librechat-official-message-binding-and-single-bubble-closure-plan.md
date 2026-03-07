# DEV-PLAN-286：266 剩余项 A——官方消息树绑定与同轮单气泡收口

**状态**: 规划中（2026-03-07 23:59 CST）

## 1. 背景
1. [ ] `DEV-PLAN-266` 已完成单通道拦截主干，但“模型回复稳定回写到官方消息树 + 同轮唯一 assistant 气泡 + 三元组可追溯映射”仍未封板。
2. [ ] 若该缺口不先原子化收口，`DEV-PLAN-285` 阶段会退化为“封板期补实现”，违反 `271` 的阶段出口策略。

## 2. 目标与非目标
### 2.1 目标
1. [ ] 收口 `266` 中“回复必须留在官方聊天流内”的剩余要求。
2. [ ] 建立并固化“同轮唯一 assistant 气泡”绑定机制，避免串泡、重复气泡或外挂回执回流。
3. [ ] 形成 `conversation_id/turn_id/request_id -> assistant message item` 的稳定一一映射证据。

### 2.2 非目标
1. [ ] 不重定义业务 FSM 语义（`phase/missing_fields/candidates/pending_draft_summary/selected_candidate_id/commit_reply/error_code` 仍以 `223/260` 为 SSOT）。
2. [ ] 不承担失败文案策略与重试语义设计（由 `DEV-PLAN-287` 承接）。
3. [ ] 不做封板总回归（由 `DEV-PLAN-288/285` 承接）。

## 3. 266 剩余项映射（本计划承接）
1. [ ] `266 §6.5`：`模型回复出现在官方聊天流内部`。
2. [ ] `266 §6.5`：`同轮只存在一个 assistant 回复气泡，且关联 conversation_id/turn_id/request_id`。
3. [ ] `266 §6.6-2`：`回复留在官方聊天气泡内`。
4. [ ] `266 §6.6-3`：`同轮只看到一份回复`。
5. [ ] `266 §7-2/§7-5`：同轮唯一气泡与三元组一一对应硬门槛。

## 4. 顺序与 readiness
1. [X] `DEV-PLAN-283` 已完成正式入口切换，真实入口固定为 `/app/assistant/librechat`。
2. [ ] `DEV-PLAN-284` send/store/render 正式 patch 至少完成消息写回控制点接管。
3. [ ] 未满足“官方消息树唯一落点”前，不得进入 `288` 的证据封板阶段。

## 5. 实施步骤
1. [ ] 冻结消息锚点：明确每轮 assistant 占位/目标消息实体的唯一定位键（禁止同轮复用旧锚点）。
2. [ ] 收敛写回路径：成功回复只允许写回官方消息数组，不允许回落到 bridge panel/overlay/notice。
3. [ ] 建立映射审计：为每轮记录 `conversation_id/turn_id/request_id/message_id` 关联证据，支持 trace 回放核对。
4. [ ] 增加防重复机制：并发发送、重试、重新生成场景下，确保同轮只更新同一 assistant 气泡。
5. [ ] 补齐回归：首轮、连续轮、重试轮均覆盖“官方流内 + 单气泡 + 可追溯映射”。

## 6. 验收标准
1. [ ] 用户在真实入口仅能看到官方消息树内的 assistant 回复，无页面外挂回复容器。
2. [ ] 同轮 assistant 回复数量恒为 1，不出现串泡/双泡/外挂重复显示。
3. [ ] 任意抽样轮次都可回查到 `conversation_id/turn_id/request_id/message_id` 一一对应证据。
4. [ ] 未命中以上任一项即 fail-closed，不得标记 `266` 已完成。

## 7. 测试与门禁（SSOT 引用）
1. [ ] 文档门禁：`make check doc`。
2. [ ] 前端与 E2E 回归命令以 `AGENTS.md` 触发器矩阵、`Makefile`、`docs/dev-plans/012-ci-quality-gates.md` 为 SSOT。
3. [ ] 证据统一沉淀到 `docs/dev-records/assets/dev-plan-266/`，并在 `dev-plan-266-execution-log.md` 追加记录。

## 8. 交付物
1. [ ] 本计划文档：`docs/dev-plans/286-librechat-official-message-binding-and-single-bubble-closure-plan.md`。
2. [ ] 消息绑定与单气泡回归用例（含真实入口 E2E 断言）。
3. [ ] 三元组到消息实体映射证据（trace/截图/日志）。

## 9. 关联文档
- `docs/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
- `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `docs/dev-plans/284-librechat-source-level-send-and-render-takeover-plan.md`
- `docs/dev-plans/288-librechat-266-live-e2e-and-evidence-closure-plan.md`
- `AGENTS.md`
