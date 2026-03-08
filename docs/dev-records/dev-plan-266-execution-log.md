# DEV-PLAN-266 执行日志（AI对话官方 UI 单通道与气泡内回写）

**状态**: 验证中（2026-03-08 CST；真实入口 E2E 已接入默认 Playwright 基线，迁移 admin 环境阻塞已排除；当前待修复正式入口 vendored UI 与 sid 会话的认证/启动闭环缺口，再复跑并补齐封板证据）

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
4. [X] 266 真实入口自动化已接入默认基线：移除 `TP288_USE_EXISTING_RUNTIME=1` 条件后，`e2e/tests/tp288-librechat-real-entry-evidence.spec.js` 已进入 Playwright 默认发现集合（`--list` 可见 2 个用例）。
5. [ ] `266` 收口清单待补齐：当前 `286/287` 已完成，`288` 尚待在修复环境后完成默认基线复跑与证据固化。

## 4.1 本轮推进记录（2026-03-08）
1. [X] 已移除 `tp288` 的环境开关 skip，使真实入口用例直接进入默认 Playwright 基线。
2. [X] 已执行 `cd e2e && pnpm exec playwright test tests/tp288-librechat-real-entry-evidence.spec.js --list`，确认默认发现到 2 个用例。
3. [X] 已修复 `scripts/e2e/run.sh` 的 migration admin 用户口径：改为显式使用 `E2E_DB_ADMIN_USER`，避免 `.env` 中 `DB_USER=app_runtime` 触发 `must be owner of table assistant_conversations`。
4. [X] 已确认 `tp288` 在 `TRUST_PROXY=1` 与正确 `KRATOS_PUBLIC_URL` 环境下会真正进入正式入口页面，不再停留在租户登录 `422 invalid_credentials`。
5. [ ] 当前真实阻塞：`/app/assistant/librechat` 已不再提供 `main iframe`；vendored UI 在正式入口下仍未与 sid 会话完成认证/启动闭环，运行时会内部请求 `/app/login` 并停在空白页，`tp288` 尚无法产出封板证据。

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
5. [X] `e2e/tests/tp288-librechat-real-entry-evidence.spec.js`：真实入口 `/app/assistant/librechat` 回归脚本已接入默认 Playwright 基线；当前已把登录与承载面探针更新为租户 host + “direct page / iframe 双承载探测”，并继续保留 `data-assistant-*` 三元组与“无 native send POST”作为最终 stopline。
