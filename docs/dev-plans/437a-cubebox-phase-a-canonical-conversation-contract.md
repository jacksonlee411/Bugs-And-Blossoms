# DEV-PLAN-437A：CubeBox Phase A 共享 Canonical Contract

**状态**: 规划中（2026-04-21 11:55 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：作为 `DEV-PLAN-437 / PR-437A` 的 companion doc，冻结 `431/432/434` 在首轮开工时必须共享的最小 conversation/turn/item/event 契约、SSE envelope、reducer 输入形状、reconstruction 输出形状与 deterministic provider fixture 口径。
- **关联模块/目录**：`docs/dev-plans/431-codex-ui-protocol-and-shell-reuse-plan.md`、`docs/dev-plans/432-codex-session-persistence-reuse-plan.md`、`docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md`、`docs/dev-plans/437-cubebox-implementation-roadmap-for-fast-start.md`、`apps/web`、`internal/server`、`modules/cubebox`
- **关联计划/标准**：`DEV-PLAN-430`、`DEV-PLAN-431`、`DEV-PLAN-432`、`DEV-PLAN-434`、`DEV-PLAN-437`
- **用户入口/触点**：右侧抽屉、SSE 流式回复、会话恢复、历史 `compact_item` 回放兼容

### 0.1 Simple > Easy 三问

1. **边界**：本文件只冻结首轮共享互操作契约；不替代 `431` 的完整 UI 协议、`432` 的完整生命周期 contract、`434` 的完整 compaction 执行语义。
2. **不变量**：`431` reducer、`432` reconstruction、`434` compaction event 不得各自发明相近但不同的命名或 envelope；首轮必须共享同一份 canonical event contract。
3. **可解释**：reviewer 必须能在 5 分钟内说明 SSE 事件、恢复输出和 reducer 输入为什么是同一套 shape，而不是三套各自转换的 DTO。

## 1. 目标

1. 冻结首轮 conversation / turn / item 基本术语。
2. 冻结 SSE event envelope。
3. 冻结 `Phase B` 必需事件名。
4. 冻结 `432` reconstruction 交付给 `431` reducer 的输出形状。
5. 冻结 deterministic provider / mock SSE / fake provider 的 fixture 口径。

## 2. 非目标

1. 不在本文件裁决 provider 配置、health、Authz、DB schema。
2. 不在本文件定义完整管理面对象。
3. 不在本文件扩展 shell/file/patch/exec/plugin/MCP 事件。
4. 不在本文件引入 mid-turn compact、remote compaction、model downshift。

## 3. Canonical 术语

| 术语 | 正式含义 | owner | 备注 |
| --- | --- | --- | --- |
| `conversation` | 一条可列出、可读取、可恢复、可归档的会话主线 | `432` | `431` 只消费其 UI 入口与展示 |
| `turn` | 一次用户输入驱动的一轮助手处理 | `431` + `432` | 运行态事件由 `431` 消费；恢复态由 `432` 重建 |
| `item` | 挂在 turn 下的可见条目，如 user message、assistant message、context item、compact item、error item | `431` | `434` 只新增 compact 相关 item 语义 |
| `timeline event` | 驱动 reducer 与 reconstruction 的有序事件 | `431` | `432` 输出同形事件流，避免第二套 read model DTO |
| `prompt view` | 当前发给模型的有效上下文视图 | `434` | 不覆盖 append-only 原始消息 |

## 4. Canonical SSE Envelope

所有运行态 SSE 事件与恢复态重建事件统一采用以下 envelope：

```json
{
  "event_id": "evt_01",
  "conversation_id": "conv_01",
  "turn_id": "turn_01",
  "sequence": 12,
  "type": "turn.agent_message.delta",
  "ts": "2026-04-21T12:00:00Z",
  "payload": {}
}
```

字段冻结如下：

| 字段 | 含义 | 规则 |
| --- | --- | --- |
| `event_id` | 事件唯一 ID | 同一 conversation 内不可重复 |
| `conversation_id` | 会话 ID | conversation 级事件也必须带上 |
| `turn_id` | 当前 turn ID | conversation 级事件可为 `null` |
| `sequence` | 单调递增顺序号 | reducer 与 reconstruction 都按此排序 |
| `type` | 事件名 | 只允许使用本文件第 5 节冻结的命名 |
| `ts` | 事件时间 | 审计/排序辅助，不替代 `sequence` |
| `payload` | 事件体 | 不同 `type` 的字段差异在 payload 内承载 |

## 5. Phase A / B 冻结事件集

### 5.1 首轮必须事件

| 事件名 | 用途 | 最小 payload |
| --- | --- | --- |
| `conversation.loaded` | 读取或恢复 conversation 后装载初始状态 | `title`、`status`、`archived` |
| `turn.started` | 一轮开始 | `user_message_id`、`trace_id`、`provider_id`、`provider_type`、`model_slug`、`runtime` |
| `turn.user_message.accepted` | 用户消息已进入本轮 | `message_id`、`text` |
| `turn.agent_message.delta` | 助手流式增量 | `message_id`、`delta` |
| `turn.agent_message.completed` | 助手消息完成 | `message_id` |
| `turn.context_item.started` | 上下文条目开始 | `item_id`、`kind` |
| `turn.context_item.completed` | 上下文条目完成 | `item_id`、`kind`、`summary` |
| `turn.query_entity.confirmed` | 查询链确认了可继承业务实体 | `entity.domain`、`entity.entity_key`、`entity.intent`、`entity.as_of`、`entity.source_api_key` |
| `turn.error` | 本轮失败 | `code`、`message`、`retryable`、`trace_id`、`provider_id`、`provider_type`、`model_slug`、`runtime`、`latency_ms` |
| `turn.interrupted` | 本轮被用户或系统中断 | `reason`、`trace_id`、`provider_id`、`provider_type`、`model_slug`、`runtime`、`latency_ms` |
| `turn.completed` | 本轮结束 | `status`、`trace_id`、`provider_id`、`provider_type`、`model_slug`、`runtime`、`latency_ms` |

### 5.1A 历史兼容事件

| 事件名 | 用途 | 最小 payload |
| --- | --- | --- |
| `turn.context_compacted` | 历史 compact 回放兼容；`DEV-PLAN-469 Phase 1` 下新 turn 默认不新增该事件 | `summary_id`、`source_range` |

### 5.2 明确暂缓事件

以下事件不进入 `PR-437A / PR-437B` 必须项：

- `conversation.archived`
- `conversation.renamed`
- `turn.compaction.started`
- `turn.token_usage.updated`
- provider failover / route alias / quota 相关事件
- shell/file/patch/exec/plugin/marketplace 相关事件

### 5.3 首轮 lifecycle 字段规则

`PR-437B` 之后，`433 Slice 2.5A` 的最小 lifecycle telemetry 继续复用本节事件名，并冻结以下字段规则：

1. `trace_id`：
   - 同一 turn 内稳定不变。
   - 首轮只要求字符串可关联，不要求前端理解其生成算法。
2. `provider_id` / `provider_type` / `model_slug`：
   - 首轮必须由运行时真实配置派生。
   - deterministic fixture 路径也必须给出稳定值，避免测试链路长出第二套 shape。
3. `runtime`：
   - 首轮只允许 `openai-chat-completions` 或 `deterministic-fixture`。
4. `latency_ms`：
   - 仅要求出现在 terminal lifecycle 事件，即 `turn.error`、`turn.interrupted`、`turn.completed`。
   - 以服务端开始处理该 turn 到 terminal event 写出前的壁钟时间计算。

## 6. Reducer 输入与 Reconstruction 输出

### 6.1 `431` reducer 输入

`431` reducer 的输入统一为：

```ts
type TimelineEventStream = {
  conversation: {
    id: string
    title: string
    status: "active" | "archived"
    archived: boolean
  }
  events: CanonicalEvent[]
}
```

要求：

1. `events` 必须按 `sequence` 单调递增。
2. reducer 不直接读取数据库记录，不消费第二套 persistence DTO。
3. SSE 增量与恢复读取都必须先归一为 `CanonicalEvent[]` 再喂给 reducer。

### 6.2 `432` reconstruction 输出

`432` read / resume / archive read model 对 `431` 的正式交付物就是同一份 `TimelineEventStream`：

```ts
type ConversationReplayResponse = {
  conversation: {
    id: string
    title: string
    status: "active" | "archived"
    archived: boolean
  }
  events: CanonicalEvent[]
  next_sequence: number
}
```

约束：

1. `432` 不再额外交付“只给恢复页使用”的专用 timeline DTO。
2. `431` 不在前端再次拼装一套 conversation lifecycle 解释器。
3. reconstruction 输出中的 compact 相关事件必须与 `434` 事件命名一致。

## 7. Canonical Item 语义

首轮 `item.kind` 只允许：

- `user_message`
- `agent_message`
- `context_item`
- `compact_item`
- `error_item`

说明：

- `431` timeline 组件按 `item.kind` 决定展示语义。
- `434` 只新增 `compact_item` 的生成与消费规则，不新增第二套 timeline item 分类。
- 当前状态（`2026-04-22`）：
  - `compact_item` 已由 `turn.context_compacted` 统一定义为历史回放兼容 item，并被 reducer / restore flow 消费。
  - `DEV-PLAN-469 Phase 1` 下新 turn 默认不再新增该 event；当前仍未引入第二套“只供恢复页使用”的 compact DTO，继续维持 `ConversationReplayResponse.events -> reducer -> timeline` 单链路。

## 8. Deterministic Provider / Mock SSE 口径

`PR-437A / PR-437B` 冻结以下测试与开发口径：

1. required gate 只允许 deterministic provider、mock SSE 或 fake provider。
2. fixture 必须输出稳定的 `CanonicalEvent` 序列，而不是任意拼接字符串。
3. 至少要覆盖：
   - 正常 delta -> completed
   - delta -> interrupted
   - error
4. 真实外部 provider 调用只允许作为非阻断 smoke 或 readiness 补充证据。

推荐 fixture 形状：

```json
{
  "fixture_id": "stream_basic_ok",
  "conversation": {
    "id": "conv_fixture_01",
    "title": "Fixture Conversation",
    "status": "active",
    "archived": false
  },
  "events": []
}
```

## 9. Owner 边界

| 主题 | owner | 其他方职责 |
| --- | --- | --- |
| 事件名与 reducer 消费语义 | `431` | `432/434` 只消费同形 contract |
| read/resume/reconstruction 数据来源 | `432` | 输出 shape 必须对齐本文件 |
| compaction event 的业务语义 | `434` | 事件名与 envelope 必须对齐本文件 |
| 路线图与阶段编排 | `437` | 不重写本文件字段定义 |

## 10. Stopline

- 不得让 `431`、`432`、`434` 各自定义第二套 event name。
- 不得在恢复路径上额外交付一套“前端专用 timeline DTO”。
- 不得把 `prompt view` 事件或 compaction 事件写成与运行态 SSE 不同名的对象。
- 不得把 deterministic provider fixture 简化成“只返回一段最终文本”而没有事件序列。

## 11. 关联文档

- `docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`
- `docs/dev-plans/431-codex-ui-protocol-and-shell-reuse-plan.md`
- `docs/dev-plans/432-codex-session-persistence-reuse-plan.md`
- `docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md`
- `docs/dev-plans/437-cubebox-implementation-roadmap-for-fast-start.md`
