# DEV-PLAN-239A：LibreChat 对话式自动执行与独立页面实施方案

**状态**: 已完成（2026-03-04 16:55 CST）

## 1. 背景与问题
- `DEV-PLAN-239` 已恢复 `/assistant-ui` 可交互与可写转发，但业务执行仍主要依赖右侧「当前回合操作」按钮（Regenerate/Confirm/Commit）。
- `DEV-PLAN-239B` 验证结论显示：239A 主干能力已实现，但“直接验证用例（Case 1~6）”仍存在测试闭环与证据闭环缺口，尚未达到 100% 验收。
- 本次修订目标：**不剪裁 239A 原目标**，并新增强制收口要求，确保 239A 目标与 239B 用例都 100% 达成。

## 2. 目标与非目标

### 2.1 目标（保持不缩水）
1. [X] 建立 `assistant-ui` ↔ `AssistantPage` 的自动执行消息桥：聊天发送后自动触发后端 `createTurn -> confirm -> commit` 链路（保持 One Door）。
2. [X] 支持对话式补全与确认：
   - 缺字段：提示需补充信息并在后续对话中合并补全；
   - 多候选：在对话中回复候选编号/编码完成确认；
   - 执行确认：在对话中回复确认词完成执行。
3. [X] 提供 `/app/assistant/librechat` 独立页面，仅渲染 LibreChat 壳层（无右侧事务面板）。
4. [X] 保持租户/会话边界与代理安全策略不退化。
5. [X] `239A Case 1~6` 全部通过，且 `239B` 识别的缺口全部关闭（100%）。

### 2.2 非目标（保持不变）
1. [X] 不新增业务写入口；最终写入仍走既有 `/internal/assistant/*` 与 One Door。
2. [X] 不引入 legacy 双链路。
3. [X] 不新增数据库 schema/迁移/sqlc 变更。

## 3. 方案概览（修订后）

### 3.1 消息桥增强（维持）
- 在 `/assistant-ui` HTML 代理响应中注入桥脚本（外链 `bridge.js`），捕获 LibreChat 发送动作并向父页 `postMessage`：
  - `assistant.prompt.sync`（用户输入）
  - `assistant.bridge.ready`（桥就绪）
- 父页可反向发送：
  - `assistant.flow.notice`（提示/确认指引/成功回执）
- 为避免 LibreChat 白屏回归（PWA SW 旧缓存导致资源漂移）：
  - 代理层移除 `vite-plugin-pwa:register-sw` 注册脚本；
  - 注入一次性 SW/Cache 清理脚本，自动注销旧 SW 并清理旧缓存。

### 3.2 自动执行编排（收口增强）
- `AssistantPage` 与 `LibreChatPage` 均必须执行一致的对话编排链路：
  1. [X] 确保会话存在；
  2. [X] 判断是否命中“待确认回合”（候选选择/确认执行）；
  3. [X] 必要时合并缺失字段并重构标准输入；
  4. [X] 调用 `createAssistantTurn`；
  5. [X] 根据 `dry_run.validation_errors` 自动分流：
     - 缺字段：返回补充提示；
     - 候选歧义：解析候选编号/编码，不足继续提示；
     - 可提交：自动 `confirm + commit`。

### 3.3 独立页面（收口增强）
- 保持 `/app/assistant/librechat` 路由与导航入口。
- 页面只保留 LibreChat iframe（含 `channel/nonce`）。
- 新增强制要求：`LibreChatPage` 必须拥有专属测试套件，不能仅依赖 `AssistantPage` 侧证据。

## 4. 安全与边界（收口增强）
- 继续使用 `/assistant-ui/**` 受保护前缀、会话与租户校验。
- 桥消息必须校验 `origin + channel + nonce`。
- 代理仍保持路径/方法边界与敏感头处理，不可旁路业务写 API。
- 执行确认词必须“状态前置 + 语义收敛”：
  - 仅在 `validated/confirmed` 待推进状态下可触发提交推进；
  - 增加误触发负测，避免普通聊天文本被误识别为提交指令。

## 5. 验收标准（升级为 100% 可测）
1. [X] 在 `/app/assistant`，发送完整创建指令后，无需点击右侧按钮即可成功提交并显示提交结果。
2. [X] 缺字段场景：通过后续聊天补全后可继续自动执行。
3. [X] 多候选场景：通过聊天回复候选编号/编码后可自动确认并提交。
4. [X] `/app/assistant/librechat` 可独立访问并完成聊天交互。
5. [X] 回归通过：`AssistantPage`、`LibreChatPage`、消息桥、代理改造相关测试全部通过。
6. [X] 不再注册 LibreChat PWA Service Worker，旧缓存不会持续接管页面渲染。
7. [X] Case 6 落库核验自动化通过：聊天提交后在 `/app/org/units` 可检索到组织且生效日期一致。
8. [X] `DEV-PLAN-239B` 中 GAP-239B-01~05 全部关闭并在执行记录中有证据。

## 6. 触发器与门禁（与 239B 收口对齐）
- 命中：Go 代理代码、Web UI 页面/路由/导航、测试、文档。
- 本地必跑（最小集）：
  - `go fmt ./... && go vet ./...`
  - `make check lint && make test`
  - `pnpm --dir apps/web test -- src/pages/assistant/assistantAutoRun.test.ts src/pages/assistant/assistantMessageBridge.test.ts src/pages/assistant/AssistantPage.test.tsx src/pages/assistant/LibreChatPage.test.tsx`
  - `go test ./internal/server -run "TestAssistantUIProxyHandler|TestServeAssistantUIBridgeScript|TestRewriteAssistantUIProxyHTMLBase|TestInjectAssistantUIProxyServiceWorkerCleanupScript|TestStripAssistantUIProxyServiceWorkerRegistration|TestWithTenantAndSession" -count=1`
  - `make e2e`（至少包含 239A 新增用例）
  - `make generate && make css`（如前端产物变更）
  - `make check doc`

## 7. 直接验证用例（239A，冻结且必须全通过）

> 验证入口：`http://localhost:8080/app/assistant/librechat`  
> 目标：在 LibreChat 独立页完成“发一句话自动执行”，并在信息不全/多候选/执行确认时仅通过对话完成闭环。

### Case 1：通道连通（前置）
- 提示词：`你好`
- 预期：
  - 页面出现“自动执行通道已连接：可直接在 LibreChat 对话中输入需求。”
  - 页面不白屏、输入框可用、可正常发消息。

### Case 2：一句话自动执行（完整信息）
- 提示词：`在 AI治理办公室 下新建 人力资源部2，生效日期 2026-01-01`
- 预期：
  - 无需点击右侧 Regenerate/Confirm/Commit；
  - 自动触发 create -> confirm -> commit；
  - 对话侧收到提交结果提示（含 `effective_date=2026-01-01`）。

### Case 3：信息不充分 -> 对话补全
- 第一句：`在 AI治理办公室 下新建 人力资源部239A补全`
- 第二句：`生效日期 2026-03-25`
- 预期：
  - 第一句先提示缺少字段；
  - 第二句补全后自动继续执行并提交成功；
  - 全程不依赖右侧按钮。

### Case 4：多候选确认（对话内完成）
- 第一句（父组织名需能命中多候选）：`在 共享服务中心 下新建 239A候选验证部，生效日期 2026-03-26`
- 第二句（候选出现后）：`选第2个`（或候选编码）
- 预期：
  - 系统返回候选列表；
  - 通过“选第N个/编码”完成候选确认；
  - 自动提交成功。

### Case 5：执行前确认词
- 提示词：`确认执行`
- 可替代：`确认提交` / `立即执行` / `同意执行` / `yes` / `ok`
- 预期：
  - 对于待确认回合，系统直接推进到提交；
  - 不再要求点击右侧 Confirm/Commit；
  - 普通非提交语义文本不会误触发提交（新增负测）。

### Case 6：结果落库核验
- 操作：登录后访问 `http://localhost:8080/app/org/units`，检索上述新建组织名称。
- 预期：
  - 组织可检索；
  - 生效日期与提示词一致；
  - 验证 One Door 写入链路成功。

## 8. 实施任务（239B 缺口对齐）

### 8.1 测试补齐（必须）
1. [X] 新增 `apps/web/src/pages/assistant/LibreChatPage.test.tsx`：覆盖 Case 1/2/3/4/5 主干流程。
2. [X] 扩展 `AssistantPage.test.tsx`：补齐 Case 3 双轮补全与 Case 5 误触发负测。
3. [X] 新增自动化用例（server integration，239A 专项）：覆盖 Case 6 的落库与查询断言（`internal/server/assistant_api_test.go`）。

### 8.2 行为硬化（必须）
1. [X] 收窄确认词策略，避免宽匹配误提交。
2. [X] 保证 `AssistantPage` 与 `LibreChatPage` 编排逻辑一致，不允许一侧缺分支。

### 8.3 文档与证据收口（必须）
1. [X] 新增 `docs/dev-records/dev-plan-239a-execution-log.md`，逐条记录 Case 1~6 实测证据。
2. [X] `DEV-PLAN-239B` 缺口全部关闭后，回填 239A 勾选项并将状态更新为 `已完成`。

## 9. 停止线（Stopline）
- 任一 Case 未通过、任一关键测试失败、或 239B 缺口仍开放时：
  - [ ] 禁止将 239A 标记为已完成；
  - [ ] 禁止以“临时手工验证”替代自动化回归；
  - [ ] 禁止降低目标范围或删除 Case。
