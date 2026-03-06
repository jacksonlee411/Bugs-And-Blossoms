# DEV-PLAN-262 执行日志（LibreChat 对话回执渲染越界）

**状态**: 进行中（2026-03-06 07:57 CST）

## 1. 问题复现与截图证据（M1）
1. [X] 已复现“回执渲染在对话框外”问题。
2. [X] 复现截图：
   - `docs/archive/dev-records/dev-plan-262-librechat-outside-chat-full.png`
   - `docs/archive/dev-records/dev-plan-262-librechat-outside-chat-iframe.png`
3. [X] DOM 定位结论（复现态）：
   - `data-assistant-dialog-stream` 挂载路径：`div[data-assistant-dialog-stream=1] <- body <- html`
   - `inChatRoot=false`，`hasChatRoot=false`

## 2. 根因与修复（M2/M3）
1. [X] 根因确认：bridge 脚本 `findDialogRoot()` 存在 `main/#__next/body` 兜底，聊天锚点未命中时会直接 append 到页面外层。
2. [X] 修复实现（`internal/server/assistant_ui_proxy.go`）：
   - 移除 `document.body` 兜底与 `main/#__next` 作为业务回执挂载目标。
   - 新增“合法锚点”判定，仅允许聊天容器类节点。
   - 新增消息队列 + `MutationObserver` 延迟重试（容器未就绪时等待）。
   - fail-closed：超时仍无合法锚点时丢弃队列并回传 `assistant.bridge.render_error`，避免越界渲染。
3. [X] 测试补强（`internal/server/assistant_ui_proxy_test.go`）：
   - 新增断言：脚本不得包含 `return document.body`。
   - 新增断言：脚本包含 `assistant.bridge.render_error`。

## 3. 验证结果（M4）
1. [X] Go 测试通过：
   - `go test ./internal/server -run "TestAssistantUIProxyHandler|TestServeAssistantUIBridgeScript|TestRewriteAssistantUIProxyHTMLBase" -count=1`
2. [X] 既有 E2E 闭环回归通过：
   - `pnpm --dir e2e exec playwright test tests/tp260-librechat-dialog-closure.spec.js`
3. [X] 修复后截图与 DOM 取证：
   - `docs/archive/dev-records/dev-plan-262-librechat-post-fix-iframe.png`
   - 结果：`found=false`（未再挂到 body 外层），越界渲染已被阻断。

## 4. 待补项
1. [X] 增补一个持久化 E2E DOM 断言（直接校验聊天容器内挂载，而非临时验证脚本）：
   - 新增 `e2e/tests/tp262-librechat-dialog-anchor.spec.js`
   - 验证命令：`pnpm --dir e2e exec playwright test tests/tp262-librechat-dialog-anchor.spec.js`（通过）

## 5. 用户反馈“无对话回执”复验与修复确认
1. [X] 复验场景：`/app/assistant/librechat` 实时链路（已登录 app，会话与 turn 返回 200）下，原先出现“无回执可见”。
2. [X] 根因补充：LibreChat v0.8.0 聊天主容器使用 `#messages-view`，旧锚点选择器未覆盖，导致桥脚本无法挂载回执流。
3. [X] 修复：在 bridge 聊天锚点选择器中新增 `#messages-view`（并保持禁止挂载到 `body/main`）。
4. [X] 证据截图（修复后）：
   - `docs/archive/dev-records/dev-plan-262-live-login-prompt-full.png`
   - `docs/archive/dev-records/dev-plan-262-live-login-prompt-iframe.png`
   - 结果：页面已出现助手对话回执文案（如“自动执行通道已连接...”“信息不完整，请补充...”），且未挂载到 `body` 外层。
5. [X] 回归：
   - `go test ./internal/server -run "TestAssistantUIProxyHandler|TestServeAssistantUIBridgeScript|TestRewriteAssistantUIProxyHTMLBase" -count=1`
   - `pnpm --dir e2e exec playwright test tests/tp262-librechat-dialog-anchor.spec.js tests/tp260-librechat-dialog-closure.spec.js`
