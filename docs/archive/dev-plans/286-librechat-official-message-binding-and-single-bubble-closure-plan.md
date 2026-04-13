# DEV-PLAN-286：266 剩余项 A——官方消息树绑定与同轮单气泡收口

> 归档说明（2026-04-12）：本文件已自 `docs/dev-plans/` 迁入 `docs/archive/dev-plans/`，仅保留为历史参考，不再作为现行 SSOT。

**状态**: 已完成（2026-03-08 CST）

## 1. 背景
1. [X] `DEV-PLAN-266` 已完成单通道拦截主干，但“模型回复稳定回写到官方消息树 + 同轮唯一 assistant 气泡 + 三元组可追溯映射”已由本计划完成首个收口点。
2. [X] 该缺口已先行原子化收口，`DEV-PLAN-285` 继续保持“仅封板、不补实现”的阶段边界。

## 2. 目标与非目标
### 2.1 目标
1. [X] 收口 `266` 中“回复必须留在官方聊天流内”的剩余要求。
2. [X] 建立并固化“同轮唯一 assistant 气泡”绑定机制，避免串泡、重复气泡或外挂回执回流。
3. [X] 形成 `conversation_id/turn_id/request_id -> assistant message item` 的稳定一一映射证据。

### 2.2 非目标
1. [X] 不重定义业务 FSM 语义（`phase/missing_fields/candidates/pending_draft_summary/selected_candidate_id/commit_reply/error_code` 仍以 `223/260` 为 SSOT）。
2. [X] 不承担失败文案策略与重试语义设计（由 `DEV-PLAN-287` 承接）。
3. [X] 不做封板总回归（由 `DEV-PLAN-288/285` 承接）。

## 3. 266 剩余项映射（本计划承接）
1. [X] `266 §6.5`：`模型回复出现在官方聊天流内部`。
2. [X] `266 §6.5`：`同轮只存在一个 assistant 回复气泡，且关联 conversation_id/turn_id/request_id`。
3. [X] `266 §6.6-2`：`回复留在官方聊天气泡内`。
4. [X] `266 §6.6-3`：`同轮只看到一份回复`。
5. [X] `266 §7-2/§7-5`：同轮唯一气泡与三元组一一对应硬门槛。

## 4. 顺序与 readiness
1. [X] `DEV-PLAN-283` 已完成正式入口切换，真实入口固定为 `/app/assistant/librechat`。
2. [X] `DEV-PLAN-284` send/store/render 正式 patch 已完成消息写回控制点接管。
3. [X] 已满足“官方消息树唯一落点”的前置条件，可继续进入 `288` 的证据封板阶段。

## 5. 实施步骤
1. [X] 冻结消息锚点：以 `conversation_id::turn_id::request_id` 作为稳定 `bindingKey`，明确每轮 assistant 占位/目标消息实体的唯一定位键。
2. [X] 收敛写回路径：成功回复只允许写回官方消息数组，不允许回落到 bridge panel/overlay/notice。
3. [X] 建立映射审计：在 runtime payload 与官方气泡 DOM 上同步暴露 `conversation_id/turn_id/request_id/message_id/bindingKey` 关联证据。
4. [X] 增加防重复机制：通过 `upsertAssistantFormalMessage(...)` 按 `messageId/bindingKey` 合并更新并去重，确保同轮只更新同一 assistant 气泡。
5. [X] 补齐回归：新增 runtime/组件定向测试，覆盖稳定绑定键、重复气泡去重与官方气泡 DOM 映射暴露。

## 6. 验收标准
1. [X] 用户在真实入口仅能看到官方消息树内的 assistant 回复，无页面外挂回复容器。
2. [X] 同轮 assistant 回复数量恒为 1，不出现串泡/双泡/外挂重复显示。
3. [X] 任意抽样轮次都可回查到 `conversation_id/turn_id/request_id/message_id` 一一对应证据。
4. [X] 本计划已满足自身收口项；`266` 是否完成仍取决于 `287/288` 后续达标。

## 7. 测试与门禁（SSOT 引用）
1. [X] 文档门禁：`make check doc`。
2. [X] 前端与 E2E 回归命令以 `AGENTS.md` 触发器矩阵、`Makefile`、`docs/dev-plans/012-ci-quality-gates.md` 为 SSOT。
3. [X] 本轮实现证据以定向测试、构建结果与文档状态更新固化；`288` 再统一补齐 E2E/截图类证据。

## 8. 交付物
1. [X] 本计划文档：`docs/archive/dev-plans/286-librechat-official-message-binding-and-single-bubble-closure-plan.md`。
2. [X] 消息绑定与单气泡回归用例（本轮已补 runtime/组件测试；真实入口 E2E 断言由 `288` 继续补齐）。
3. [X] 三元组到消息实体映射证据（本轮已在 payload/DOM 测试断言中固化；trace/截图由 `288` 汇总）。

## 9. 关联文档
- `docs/archive/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
- `docs/archive/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `docs/archive/dev-plans/284-librechat-source-level-send-and-render-takeover-plan.md`
- `docs/archive/dev-plans/288-librechat-266-live-e2e-and-evidence-closure-plan.md`
- `AGENTS.md`
