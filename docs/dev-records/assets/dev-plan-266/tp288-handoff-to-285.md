# DEV-PLAN-288 -> DEV-PLAN-285 交接清单

- 生成时间：2026-03-08 22:49:12 CST
- 当前结论：`可作为 285 的 266 子域通过前置件直接引用。`
- 使用边界：本交接单只证明 `DEV-PLAN-266` 的真实入口 stopline 已闭环；不替代 `DEV-PLAN-290/291`，也不表示 `260` Case 2~4 已通过。

## 已通过项
- 真实入口固定为 `/app/assistant/librechat`；证据不再依赖历史 `/assistant-ui` 直链。
- `tp288-e2e-001/002` 已按固定命名产物重跑并通过，命令、时间、截图、DOM、网络、trace、断言文件均已入 `docs/dev-records/assets/dev-plan-266/`。
- `native_send_emitted=0` 已由两条用例的 `*-network.json` 固化，未出现 `/api/agents/chat` 或 `/api/messages` 原生发送 POST。
- `official_message_tree_only=true` 已由两条用例的 `*-assertions.json` 固化，用户可见回复仅落在官方消息树中。
- `single_assistant_bubble=true` 已由两条用例的 `*-assertions.json` 固化；失败内泡、重试新增单新泡均满足 stopline。
- `conversation_id/turn_id/request_id -> 唯一 assistant bubble` 已由两条用例的 `*-dom.json` 与 `*-assertions.json` 固化，可回查 `binding_key/message_id`。
- `tp288-real-entry-evidence-index.json` 已按用例维度列出 `command/executed_at/artifacts/assertions/result`，可供 `285` 直接引用。

## 交付资产
- 总索引：`docs/dev-records/assets/dev-plan-266/tp288-real-entry-evidence-index.json`
- Case 001：`docs/dev-records/assets/dev-plan-266/tp288-e2e-001-page.png`、`docs/dev-records/assets/dev-plan-266/tp288-e2e-001-dom.json`、`docs/dev-records/assets/dev-plan-266/tp288-e2e-001-network.json`、`docs/dev-records/assets/dev-plan-266/tp288-e2e-001-trace.zip`、`docs/dev-records/assets/dev-plan-266/tp288-e2e-001-assertions.json`
- Case 002：`docs/dev-records/assets/dev-plan-266/tp288-e2e-002-page.png`、`docs/dev-records/assets/dev-plan-266/tp288-e2e-002-dom.json`、`docs/dev-records/assets/dev-plan-266/tp288-e2e-002-network.json`、`docs/dev-records/assets/dev-plan-266/tp288-e2e-002-trace.zip`、`docs/dev-records/assets/dev-plan-266/tp288-e2e-002-assertions.json`
- 执行日志：`docs/dev-records/dev-plan-266-execution-log.md`

## 对 285 的使用方式
- 在 `285 §2.3/§3/§4` 中，可将 `tp288-real-entry-evidence-index.json` 作为 `266 stopline 全通过` 的直接引用输入。
- `285` 仍需单独等待 `290` 输出 `260` Case Matrix 真实结论，以及 `291` 输出升级兼容前置结论。
- 若 `285` 启动时发现 `288` 证据生成时间早于最近一次影响消息绑定、渲染路径、路由/认证链路、错误码语义或 fail-closed 行为的合入，则不得直接引用旧结论。

## 失效条件（对齐 DEV-PLAN-271 S5）
- `290A` 合入影响 pending placeholder bubble / binding 生命周期。
- `240C/240D/240E` 合入影响 runtime gate、durable execution、MCP 写准入、错误码或 fail-closed 语义。
- send/store/render 主路径、消息绑定 key、正式入口路由/认证链路发生变化。

## 建议动作
- `285` 启动前先核对 `docs/dev-records/assets/dev-plan-266/tp288-real-entry-evidence-index.json` 的生成时间与最近影响性合入时间。
- 若命中上述失效条件，先按同口径重跑 `tp288-e2e-001/002`，再刷新索引与本交接单。
