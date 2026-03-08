# DEV-PLAN-266 执行日志（AI对话官方 UI 单通道与气泡内回写）

**状态**: 验证中（2026-03-08 CST；已补真实入口 E2E runner skeleton 与证据索引，待 live runtime 接入默认基线后封板）

## 1. 本次落地范围
1. [X] Go 代理：`assistant-ui/bridge.js` 从“监听并转发”升级为“拦截并接管”。
2. [X] Go 代理：补充 `native_send_attempted`、`native_send_blocked`、`native_send_emitted`、`bridge_reply_embedded` 探针。
3. [X] Go 代理：聊天流消息支持稳定 `message_id` 回写，避免同轮重复气泡。
4. [X] Web：`AssistantPage` / `LibreChatPage` 发送 `assistant.flow.dialog` 时补充稳定 `message_id` 与轮次元数据。
5. [X] E2E：新增 `e2e/tests/tp266-librechat-single-channel-in-bubble.spec.js`，验证真实 `/app/assistant` 入口下的单通道与气泡内回写 stopline。

## 2. 本次实施要点
1. [X] 在 iframe 注入脚本的 capture 阶段阻断提交、回车与发送按钮点击，避免官方原始发送落网。
2. [X] 将 bridge 回复固定渲染到聊天流 `role="log"` 内部，禁止回落到 `body` 或页面外容器。
3. [X] 同一 `message_id` 只更新同一个 assistant 气泡，不追加第二份同轮回复。

## 3. 当前验证状态
1. [X] 已完成 Go / Web / E2E 代码落地。
2. [X] 已补跑 266 专属 mock stopline：`pnpm --dir e2e exec playwright test tests/tp266-librechat-single-channel-in-bubble.spec.js --reporter=line`。
3. [X] 已把运行结果固化到 `docs/dev-records/assets/dev-plan-266/`。

## 4. 本次验证结果
1. [X] Go：`go test ./internal/server -run 'TestAssistantUIProxy|TestModifyAssistantUIProxyResponse|TestAssistantReply|TestAssistantRenderReply' -count=1` 通过。
2. [X] Web：`pnpm --dir apps/web test -- src/pages/assistant/LibreChatPage.test.tsx src/pages/assistant/AssistantPage.test.tsx src/pages/assistant/assistantAutoRun.test.ts` 通过。
3. [X] 266 mock stopline：通过，确认 `native_send_emitted=0`、同轮单气泡回写、无页面外挂容器。
4. [X] 266 真实入口自动化骨架：新增 `e2e/tests/tp288-librechat-real-entry-evidence.spec.js`，覆盖成功、失败、重试、连续多轮四类路径；当前 runner 需要绑定现有 live runtime，默认 `make e2e` 基线尚未具备完整运行条件。
5. [ ] `266` 收口清单待补齐：当前 `286/287` 已完成，`288` 尚待 live runtime 默认基线接线后封板。

## 5. 证据资产
1. [X] `docs/dev-records/assets/dev-plan-266/tp266-mock-stopline-trace.zip`
2. [X] `docs/dev-records/assets/dev-plan-266/live-runtime-page.png`
3. [X] `docs/dev-records/assets/dev-plan-266/live-runtime-metrics.json`
4. [X] `docs/dev-records/assets/dev-plan-266/live-runtime-alerts.json`
5. [X] `docs/dev-records/assets/dev-plan-266/live-runtime-stream.json`
6. [X] `docs/dev-records/assets/dev-plan-266/live-runtime-turn-responses.json`
7. [X] `docs/dev-records/assets/dev-plan-266/tp288-real-entry-evidence-index.json`

## 6. 本次补充修复
1. [X] `apps/web/src/pages/assistant/AssistantPage.tsx`：首轮 turn 创建失败时，补齐桥接气泡回写路径，避免只在页面右侧报错而不回写到聊天壳层。
2. [X] `apps/web/src/pages/assistant/AssistantPage.tsx`：`postBridgeDialog` 支持显式传入 `conversation_id/turn_id/request_id/trace_id`，减少异步 state 导致的同轮 `message_id` 漂移。
3. [X] `apps/web/src/pages/assistant/AssistantPage.test.tsx`：新增首轮结构化失败回写单测。
4. [X] `e2e/tests/tp266-librechat-single-channel-in-bubble.spec.js`：历史 mock stopline 记录保留。
5. [X] `e2e/tests/tp288-librechat-real-entry-evidence.spec.js`：真实入口 `/app/assistant/librechat` 回归，覆盖成功/失败/重试/连续多轮，并把 `data-assistant-*` 三元组映射与“无 native send POST”固化为自动化 stopline。
