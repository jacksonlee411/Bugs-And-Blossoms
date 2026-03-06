# DEV-PLAN-262：LibreChat 对话回执渲染越界问题调查与收敛方案

**状态**: 进行中（2026-03-06 18:54 CST）

## 1. 背景与问题定义
- 用户反馈：在 `/app/assistant/librechat` 中，助手业务回执没有出现在聊天对话流内部，而是渲染在对话框之外（越界渲染）。
- 当前实现由 `assistant_ui_proxy.go` 注入 `bridge.js`，通过 `assistant.flow.dialog` 在 iframe 内插入消息节点。
- 问题影响：用户将回执视为“非聊天内容”，破坏对话闭环与可用性，属于用户可见功能缺陷。
- 自 `DEV-PLAN-266` 冻结后，262 的定位收敛为：**只解决“消息落点越界/锚点漂移”这一类问题**；它不单独代表“官方单通道 + 气泡内回写 + 无外挂容器”整体目标已完成。

### 1.1 当前验收边界
- 262 的问题复现可以在 `/app/assistant/librechat` / iframe 内进行，但当前主验收入口仍以 `http://localhost:8080/app/assistant/AI对话` 为准。
- 262 的通过只表示“消息落点不再越界”；若仍存在官方原始发送、官方 `Connection error`、外挂容器承担回复等问题，则应由 `DEV-PLAN-266` 继续判失败。

## 2. 目标与非目标

### 2.1 目标
1. [ ] 定位“越界渲染”的单主因或主因组合（DOM 锚点、时序、选择器、样式层级）。
2. [ ] 修复后保证 `assistant.flow.dialog` 回执只进入 LibreChat 聊天流容器内部。
3. [ ] 建立防回归测试（Go 注入脚本测试 + Web/E2E DOM 断言）。
4. [ ] 保持既有 One Door 与对话状态机契约，不引入第二业务写入口。
5. [ ] 明确 262 只是 `266` 的落点子问题，不单独宣布 `266` 已达成。

### 2.2 非目标
1. [ ] 不修改业务事务语义（create/confirm/commit 状态机不扩张）。
2. [ ] 不改数据库 schema / 迁移 / sqlc。
3. [ ] 不改 LibreChat 上游源码，仅在本仓代理注入与页面编排层收敛。
4. [ ] 不把“消息不再越界”误当成“266 的单通道 / 无官方错误体验 / 无外挂容器”已经全部完成。

## 3. 根因假设（待验证）
1. [ ] `findDialogRoot()` 命中 `main/#__next/body` 回退分支，导致 `data-assistant-dialog-stream` 被 append 到聊天区外层。
2. [ ] 聊天容器未就绪时提前创建流容器，后续未重定位到真实 transcript 容器。
3. [ ] 选择器覆盖不完整（版本变更后 DOM 结构变化），桥脚本插入目标漂移。
4. [ ] 样式与布局（flex/overflow/z-index）导致消息视觉上脱离对话框。

## 4. 实施步骤

### 4.1 M1：现场复现与 DOM 证据采集
1. [X] 固化复现步骤（账号、提示词、页面入口、预期/实际）。
2. [X] 采集 iframe 内 DOM 快照与消息节点祖先链，确认实际挂载路径。
3. [X] 输出“选择器命中矩阵”（命中顺序、命中节点、是否位于聊天流内）。

### 4.2 M2：渲染锚点契约冻结
1. [X] 定义唯一“聊天流锚点”策略：优先 transcript/message-list，禁止 `body/main` 兜底直挂。
2. [X] 定义 fail-closed：找不到合法锚点时不渲染消息，并回传技术提示，阻断越界渲染。
3. [X] 明确 MutationObserver 重试窗口与重定位策略（容器变更时迁移而非重复追加）。

### 4.3 M3：实现修复
1. [X] 改造 `internal/server/assistant_ui_proxy.go` 注入脚本的 `findDialogRoot/ensureDialogStream/appendDialogMessage`。
2. [X] 已在 bridge 层输出 `assistant.bridge.render_error` 技术提示，暂不改页面协议。
3. [X] 保持 `assistant.flow.dialog` 协议不变，避免上层协议漂移。

### 4.4 M4：测试与验收闭环
1. [X] Go：补充 `internal/server/assistant_ui_proxy_test.go`，断言“合法锚点 + fail-closed + 重定位”脚本标记。
2. [X] Web/E2E：补充 `/app/assistant/librechat` DOM 断言，验证消息节点位于聊天对话流容器内部。
3. [X] 记录执行证据：`docs/dev-records/dev-plan-262-execution-log.md`。
4. [ ] 若作为当前复验依据重跑，需回到 `AI对话` 入口，并与 `266` 的单通道/无外挂 stopline 联合断言。

## 5. 验收标准
1. [ ] 业务回执消息在视觉与 DOM 结构上都位于聊天流内部（非页面外层/浮层区）。
2. [ ] 连续多轮回执不会重复创建多个越界容器。
3. [ ] 聊天容器晚到/重渲染时，回执仍能正确挂载到合法锚点。
4. [ ] 找不到合法锚点时不发生越界渲染，且有可诊断技术提示。
5. [ ] 262 的通过仅代表“落点/锚点问题”收敛；若仍存在双链路、官方 `Connection error` 或外挂容器承担回复，则 `266` 仍判失败。

## 6. 测试与门禁
- 命中代码变更后按 SSOT 执行：`AGENTS.md`、`docs/dev-plans/012-ci-quality-gates.md`、`Makefile`。
- 最低验证集：
  1. [X] `go test ./internal/server -run "TestAssistantUIProxyHandler|TestServeAssistantUIBridgeScript" -count=1`
  2. [ ] `pnpm --dir apps/web test -- src/pages/assistant/LibreChatPage.test.tsx src/pages/assistant/assistantMessageBridge.test.ts`
  3. [X] `pnpm --dir e2e exec playwright test tests/tp260-librechat-dialog-closure.spec.js`
  4. [X] `pnpm --dir e2e exec playwright test tests/tp262-librechat-dialog-anchor.spec.js`
  5. [X] `make check doc`

## 7. 交付物
1. [X] 计划文档：`docs/dev-plans/262-librechat-dialog-render-outside-chat-investigation-and-fix-plan.md`
2. [X] 执行日志：`docs/dev-records/dev-plan-262-execution-log.md`
3. [X] 修复代码与测试清单（执行阶段回填）。

## 8. 关联文档
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-plans/261-librechat-assistant-conversation-failure-investigation-and-remediation-plan.md`
- `docs/dev-records/dev-plan-261-execution-log.md`
- `docs/dev-records/dev-plan-262-execution-log.md`
- `docs/dev-plans/231-librechat-prerequisites-contract-and-gates-plan.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `AGENTS.md`
