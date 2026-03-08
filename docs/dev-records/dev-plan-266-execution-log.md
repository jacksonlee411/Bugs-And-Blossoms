# DEV-PLAN-266 执行日志（AI对话官方 UI 单通道与气泡内回写）

**状态**: 已完成（2026-03-09 02:14 CST；已在 `290A/290` 回灌后按真实入口重跑 `tp288-e2e-001/002`，`tp288-real-entry-evidence-index.json`、`tp288-handoff-to-285.md` 与固定命名截图/DOM/网络/trace/断言资产均已刷新，`266` 当前可继续作为 `285` 的已完成输入之一被引用）

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
5. [X] `266` 收口清单已补齐：`288` 已完成默认基线复跑、固定命名证据索引与 `285` 交接单整理。

## 4.1 本轮推进记录（2026-03-08）
1. [X] 已移除 `tp288` 的环境开关 skip，使真实入口用例直接进入默认 Playwright 基线。
2. [X] 已执行 `cd e2e && pnpm exec playwright test tests/tp288-librechat-real-entry-evidence.spec.js --list`，确认默认发现到 2 个用例。
3. [X] 已修复 `scripts/e2e/run.sh` 的 migration admin 用户口径：改为显式使用 `E2E_DB_ADMIN_USER`，避免 `.env` 中 `DB_USER=app_runtime` 触发 `must be owner of table assistant_conversations`。
4. [X] 已确认 `tp288` 在 `TRUST_PROXY=1` 与正确 `KRATOS_PUBLIC_URL` 环境下会真正进入正式入口页面，不再停留在租户登录 `422 invalid_credentials`。
5. [X] 关键环境前置已确认：`dev-server` 运行需启用 `TRUST_PROXY=1`，否则租户解析会回落 `localhost`，触发 `/iam/api/sessions` `invalid_credentials`（422）。
6. [X] 已执行 `pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp288-librechat-real-entry-evidence.spec.js --grep "tp288-e2e-002"`，结果通过。
7. [X] 已执行 `pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp288-librechat-real-entry-evidence.spec.js`，结果 `2 passed`（`tp288-e2e-001/002` 全通过）。

## 4.2 证据固化补记（2026-03-08 22:49 CST）
1. [X] 已为 `tp288-e2e-001/002` 增补固定命名资产：`*-page.png`、`*-dom.json`、`*-network.json`、`*-trace.zip`、`*-assertions.json`。
2. [X] 已重写 `docs/dev-records/assets/dev-plan-266/tp288-real-entry-evidence-index.json`，按用例维度记录 `command/executed_at/artifacts/assertions/result`。
3. [X] 已新增 `docs/dev-records/assets/dev-plan-266/tp288-handoff-to-285.md`，供 `285` 直接引用 `266` 子域 stopline 结论。
4. [X] 已执行 `make check doc`，文档门禁通过。

## 5. 证据资产
1. [X] `docs/dev-records/assets/dev-plan-266/tp266-mock-stopline-trace.zip`
2. [X] `docs/dev-records/assets/dev-plan-266/live-runtime-page.png`
3. [X] `docs/dev-records/assets/dev-plan-266/live-runtime-metrics.json`
4. [X] `docs/dev-records/assets/dev-plan-266/live-runtime-alerts.json`
5. [X] `docs/dev-records/assets/dev-plan-266/live-runtime-stream.json`
6. [X] `docs/dev-records/assets/dev-plan-266/live-runtime-turn-responses.json`
7. [X] `docs/dev-records/assets/dev-plan-266/tp288-real-entry-evidence-index.json`
8. [X] `docs/dev-records/assets/dev-plan-266/tp288-e2e-001-page.png`
9. [X] `docs/dev-records/assets/dev-plan-266/tp288-e2e-001-dom.json`
10. [X] `docs/dev-records/assets/dev-plan-266/tp288-e2e-001-network.json`
11. [X] `docs/dev-records/assets/dev-plan-266/tp288-e2e-001-trace.zip`
12. [X] `docs/dev-records/assets/dev-plan-266/tp288-e2e-001-assertions.json`
13. [X] `docs/dev-records/assets/dev-plan-266/tp288-e2e-002-page.png`
14. [X] `docs/dev-records/assets/dev-plan-266/tp288-e2e-002-dom.json`
15. [X] `docs/dev-records/assets/dev-plan-266/tp288-e2e-002-network.json`
16. [X] `docs/dev-records/assets/dev-plan-266/tp288-e2e-002-trace.zip`
17. [X] `docs/dev-records/assets/dev-plan-266/tp288-e2e-002-assertions.json`
18. [X] `docs/dev-records/assets/dev-plan-266/tp288-handoff-to-285.md`

## 6. 本次补充修复
1. [X] `apps/web/src/pages/assistant/AssistantPage.tsx`：首轮 turn 创建失败时，补齐桥接气泡回写路径，避免只在页面右侧报错而不回写到聊天壳层。
2. [X] `apps/web/src/pages/assistant/AssistantPage.tsx`：`postBridgeDialog` 支持显式传入 `conversation_id/turn_id/request_id/trace_id`，减少异步 state 导致的同轮 `message_id` 漂移。
3. [X] `apps/web/src/pages/assistant/AssistantPage.test.tsx`：新增首轮结构化失败回写单测。
4. [X] `e2e/tests/tp266-librechat-single-channel-in-bubble.spec.js`：历史 mock stopline 记录保留。
5. [X] `e2e/tests/tp288-librechat-real-entry-evidence.spec.js`：真实入口 `/app/assistant/librechat` 回归脚本已接入默认 Playwright 基线；当前已把登录与承载面探针更新为租户 host + “direct page / iframe 双承载探测”，并继续保留 `data-assistant-*` 三元组与“无 native send POST”作为最终 stopline。

## 7. 288 推进卡点复盘（沉淀给后续复跑）
1. [X] 启动链卡点：`localhost` SW + `sid/auth` 兼容缺口会直接导致白屏/401/回跳，先修启动链再查业务层。
2. [X] 产物生效卡点：仅执行 `make librechat-web-build` 不会让已运行 Go 进程加载新前端包；必须重启 server。
3. [X] 渲染链卡点：formal 渲染需同时覆盖 `components/Messages/*` 与 `Chat/Messages/MessageParts` 主路径，单改旧回退链不足。
4. [X] 消息覆盖卡点：retry 二轮若沿用同 `messageId`，会在 upsert 时覆盖首轮；已通过 runtime 匹配策略修复并补单测。
5. [X] 断言口径卡点：文本“存在于气泡”与“全页计数=0”不能并存；已统一为“目标容器命中 + 全页唯一计数”。
6. [X] patch 维护卡点：手写 patch 容易 hunk 失配；后续统一用源文件 diff 生成 patch，并以 `make librechat-web-build` 校验。
7. [X] runner 噪声卡点：Playwright 偶发 `step id not found` 与业务失败要分层判断，最终以业务断言和 trace 为准。

## 8. 后续复跑检查清单（可直接执行）
1. [X] 前置：确认 `292` 已生效（auth/startup 链路可通）后再跑 `288`。
2. [X] 前置：`TRUST_PROXY=1`，并显式设置正确 `KRATOS_PUBLIC_URL/E2E_KRATOS_ADMIN_URL`。
3. [X] 执行：每次 patch stack 或 assets 变更后，固定执行“`make librechat-web-build` + 重启 server + 复跑 tp288”。
4. [X] 验收：至少同时满足 `tp288-e2e-001/002` 通过、`data-assistant-binding-key` 命中、无 native send POST。

## 4.3 新鲜度刷新补记（2026-03-09 02:14 CST）
1. [X] 因 `290A` 与 `290` 回灌命中消息绑定/渲染链，已按 `271-S5` 规则重跑 `tp288-e2e-001/002`。
2. [X] 本轮重跑结果保持 `2 passed`，`official_message_tree_only=true`、`single_assistant_bubble=true`、`conversation_turn_request_binding_unique=true`。
3. [X] 已刷新 `docs/dev-records/assets/dev-plan-266/tp288-real-entry-evidence-index.json` 与 `docs/dev-records/assets/dev-plan-266/tp288-handoff-to-285.md` 的时间戳和引用口径。
