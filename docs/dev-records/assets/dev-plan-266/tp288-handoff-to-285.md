# DEV-PLAN-288 -> DEV-PLAN-285 交接清单

- 生成时间：2026-03-11 07:19:00 CST
- 退役更新：2026-04-13 18:23 CST
- 当前结论：`保留为 285/266 阶段的历史交接记录；不再作为现行 successor 主链 gate。`
- 使用边界：本交接单只证明 `DEV-PLAN-266` 时点的真实入口 stopline 曾闭环；不替代 `DEV-PLAN-290/291`，也不表示当前 `360/360A/375` 仍需继续维护 tp288 这条 mock 脚本。

## 退役说明
- `e2e/tests/tp288-librechat-real-entry-evidence.spec.js` 已在 `DEV-PLAN-360A Phase 3/4` 封板时确认退役归档。
- 退役原因：该脚本是历史 mock 驱动的正式入口证据，当前页面承载已漂移；继续维护它会把历史 mock 口径误当成现行 live successor gate。
- 现行替代证据：`docs/dev-records/assets/dev-plan-288b/tp288b-live-evidence-index.json` 与 `docs/dev-records/assets/dev-plan-290b/tp290b-live-evidence-index.json`。

## 已通过项
- 真实入口固定为 `/app/assistant/librechat`；证据不再依赖历史 `/assistant-ui` 直链。
- `tp288-e2e-001/002` 已在 `272` 影响 runtime gate / 错误码语义后按固定命名产物重跑并通过，命令、时间、截图、DOM、网络、trace、断言文件均已刷新到 `docs/dev-records/assets/dev-plan-266/`。
- `native_send_emitted=0` 已由两条用例的 `*-network.json` 固化，未出现 `/api/agents/chat` 或 `/api/messages` 原生发送 POST。
- `official_message_tree_only=true` 已由两条用例的 `*-assertions.json` 固化，用户可见回复仅落在官方消息树中。
- `single_assistant_bubble=true` 已由两条用例的 `*-assertions.json` 固化；失败内泡、重试新增单新泡均满足 stopline。
- `conversation_id/turn_id/request_id -> 唯一 assistant bubble` 已由两条用例的 `*-dom.json` 与 `*-assertions.json` 固化，可回查 `binding_key/message_id`。
- `tp288-real-entry-evidence-index.json` 已按用例维度列出 `command/executed_at/artifacts/assertions/result`，可供 `285` 直接引用。

## 交付资产
- 总索引：`docs/dev-records/assets/dev-plan-266/tp288-real-entry-evidence-index.json`
- Case 001：`docs/dev-records/assets/dev-plan-266/tp288-e2e-001-page.png`、`docs/dev-records/assets/dev-plan-266/tp288-e2e-001-dom.json`、`docs/dev-records/assets/dev-plan-266/tp288-e2e-001-network.json`、`docs/dev-records/assets/dev-plan-266/tp288-e2e-001-trace.zip`、`docs/dev-records/assets/dev-plan-266/tp288-e2e-001-assertions.json`
- Case 002：`docs/dev-records/assets/dev-plan-266/tp288-e2e-002-page.png`、`docs/dev-records/assets/dev-plan-266/tp288-e2e-002-dom.json`、`docs/dev-records/assets/dev-plan-266/tp288-e2e-002-network.json`、`docs/dev-records/assets/dev-plan-266/tp288-e2e-002-trace.zip`、`docs/dev-records/assets/dev-plan-266/tp288-e2e-002-assertions.json`
- 执行日志：`docs/archive/dev-records/dev-plan-266-execution-log.md`

## 对 285 的使用方式
- 在 `285 §2.3/§3/§4` 中，`tp288-real-entry-evidence-index.json` 仅作为 `266 stopline 全通过` 的历史引用输入，不再要求刷新。
- `285` 仍需单独等待 `290` 输出 `260` Case Matrix 真实结论，以及 `291` 输出升级兼容前置结论。
- 若 `285` 启动时发现 `288` 证据生成时间早于最近一次影响消息绑定、渲染路径、路由/认证链路、错误码语义或 fail-closed 行为的合入，则不得直接引用旧结论。

## 失效条件（对齐 DEV-PLAN-271 S5）
- `290A` 合入影响 pending placeholder bubble / binding 生命周期（本轮已按该规则完成重跑并刷新索引）。
- `272` 合入影响 runtime gate、错误码语义、fail-closed 行为或正式动作链稳定性（本轮已按该规则完成重跑并刷新索引）。
- `240C/240D/240E` 合入影响 runtime gate、durable execution、MCP 写准入、错误码或 fail-closed 语义。
- send/store/render 主路径、消息绑定 key、正式入口路由/认证链路发生变化。

## 建议动作
- 对现行 successor 主链，不再要求重跑 `tp288-e2e-001/002`。
- 若需要验证当前正式入口，请改用 `tp288b/tp290b` live successor 证据链。
