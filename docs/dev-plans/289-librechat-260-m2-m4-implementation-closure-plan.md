# DEV-PLAN-289：260-M2~M4 实施收口专项（事实源推进 + 主链联调）

**状态**: 已完成（2026-03-08 CST）

## 1. 背景
1. [X] `DEV-PLAN-260` 已完成 P0 契约冻结；本计划已补齐 `M2~M4` 所需的事实源推进、FSM guard 接线与 `266/284` 承载面联调实现。
2. [X] 该阶段已作为独立执行单元收口，避免继续混在 `260` 主计划内扩大变更半径。

## 2. 目标与非目标
### 2.1 目标
1. [X] 完成 `260-M2`：对话状态推进以后端持久化事实源为准，稳定重建 DTO。
2. [X] 完成 `260-M3`：对话文案与模型链路按冻结语义输出，不引入前端语义补算。
3. [X] 完成 `260-M4`：与 `266/284` 对齐“单通道 + 官方消息树唯一落点 + DTO-only 消费”主链联调。

### 2.2 非目标
1. [X] 不做 Case 1~4 封板验收与证据归档（由 `DEV-PLAN-290` 承接）。
2. [X] 不做升级兼容回归封板（由 `DEV-PLAN-291/285` 承接）。
3. [X] 不重定义 `260` 的 FSM/DTO 契约正文。

## 3. 顺序与依赖
1. [X] 前置：`DEV-PLAN-286/287/288` 已形成明确归属，其中 `286/287` 已完成。
2. [X] 并行：与 `240-M2/M3` 的并行窗口已按冻结字段语义完成。
3. [X] 后置：本计划完成后，`DEV-PLAN-290` 可进入正式 Case 验收。

## 4. 实施步骤
1. [X] 事实源收口：补齐内存/PG createTurn 派生字段刷新与 pending turn 上下文合并，保证 `GET conversation`/当前会话可稳定重建 DTO。
2. [X] guard 收口：前端提交按钮已按后端 `state + phase` 收敛为“先 confirm、再 commit”，阻断越级提交。
3. [X] 模型链路收口：缺字段补全改为后端 pending context 合并，前端不再补算业务语义。
4. [X] 主链联调：`/app/assistant/librechat` 已完成 DTO-only 正式链路联调，行为与 `266/284` 承载面收口一致。

## 5. 验收标准
1. [X] `260-M2~M4` 对应任务全部完成且无前端业务重算回流。
2. [X] DTO 重建与阶段推进在重试/恢复场景下语义一致。
3. [X] 与 `266/284` 联调不出现双链路、双落点、外挂容器回流。

## 6. 测试与门禁（SSOT 引用）
1. [X] 触发器与门禁命令以 `AGENTS.md`、`Makefile`、`docs/dev-plans/012-ci-quality-gates.md` 为准。
2. [X] 文档改动至少通过 `make check doc`。

## 7. 交付物
1. [X] 本计划文档：`docs/dev-plans/289-librechat-260-m2-m4-implementation-closure-plan.md`。
2. [X] `260-M2~M4` 的联调记录与 stopline 通过证据（Go/前端测试、`go vet`、`make check lint`、`make test`、`make librechat-web-build`）。

## 8. 关联文档
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
- `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `docs/dev-plans/284-librechat-source-level-send-and-render-takeover-plan.md`
- `docs/dev-plans/290-librechat-260-m5-real-case-validation-and-evidence-plan.md`
- `AGENTS.md`
