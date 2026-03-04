# DEV-PLAN-239A：LibreChat 对话式自动执行与独立页面实施方案

**状态**: 草拟中（2026-03-04）

## 1. 背景与问题
- `DEV-PLAN-239` 已恢复 `/assistant-ui` 可交互与可写转发，但业务执行仍主要依赖右侧「当前回合操作」按钮（Regenerate/Confirm/Commit）。
- 用户目标升级为：
  1. 在 LibreChat 中“发一句话即可自动执行”（在信息充分且无歧义时）。
  2. 对信息不充分/多候选/执行前确认场景，统一通过对话完成，不再要求右侧按钮点击。
  3. 新增一个 **LibreChat 独立页面**，用于纯聊天与人工验证。

## 2. 目标与非目标

### 2.1 目标
1. [ ] 建立 `assistant-ui` ↔ `AssistantPage` 的自动执行消息桥：聊天发送后自动触发后端 `createTurn -> confirm -> commit` 链路（保持 One Door）。
2. [ ] 支持对话式补全与确认：
   - 缺字段：提示需补充信息并在后续对话中合并补全；
   - 多候选：在对话中回复候选编号/编码完成确认；
   - 执行确认：在对话中回复确认词完成执行。
3. [ ] 提供 `/app/assistant/librechat` 独立页面，仅渲染 LibreChat 壳层（无右侧事务面板）。
4. [ ] 保持租户/会话边界与代理安全策略不退化。

### 2.2 非目标
1. [ ] 不新增业务写入口；最终写入仍走既有 `/internal/assistant/*` 与 One Door。
2. [ ] 不引入 legacy 双链路。
3. [ ] 不新增数据库 schema/迁移/sqlc 变更。

## 3. 方案概览

### 3.1 消息桥增强
- 在 `/assistant-ui` HTML 代理响应中注入桥脚本（外链 `bridge.js`），捕获 LibreChat 发送动作并向父页 `postMessage`：
  - `assistant.prompt.sync`（用户输入）
  - `assistant.bridge.ready`（桥就绪）
- 父页可反向发送：
  - `assistant.flow.notice`（提示/确认指引/成功回执）
- 为避免 LibreChat 白屏回归（PWA SW 旧缓存导致资源漂移）：
  - 代理层移除 `vite-plugin-pwa:register-sw` 注册脚本；
  - 注入一次性 SW/Cache 清理脚本，自动注销旧 SW 并清理旧缓存。

### 3.2 自动执行编排（前端编排，后端执行）
- `AssistantPage` 收到 `assistant.prompt.sync` 后自动入队处理：
  1. 确保会话存在；
  2. 判断是否命中“待确认回合”（候选选择/确认执行）；
  3. 必要时合并缺失字段并重构标准输入；
  4. 调用 `createAssistantTurn`；
  5. 根据 `dry_run.validation_errors` 自动分流：
     - 缺字段：返回补充提示；
     - 候选歧义：尝试从用户回复解析候选；不足则继续提示；
     - 可提交：自动 `confirm + commit`。

### 3.3 独立页面
- 新增 `/app/assistant/librechat` 路由与导航入口。
- 页面只保留 LibreChat iframe（含 `channel/nonce`），用于纯对话验证与演示。

## 4. 安全与边界
- 继续使用 `/assistant-ui/**` 受保护前缀、会话与租户校验。
- 桥消息必须校验 `origin + channel + nonce`。
- 代理仍保持路径/方法边界与敏感头处理，不可旁路业务写 API。

## 5. 验收标准
1. [ ] 在 `/app/assistant`，发送完整创建指令后，无需点击右侧按钮即可成功提交并显示提交结果。
2. [ ] 缺字段场景：通过后续聊天补全后可继续自动执行。
3. [ ] 多候选场景：通过聊天回复候选编号/编码后可自动确认并提交。
4. [ ] `/app/assistant/librechat` 可独立访问并完成聊天交互。
5. [ ] 回归通过：`AssistantPage`、消息桥、代理改造相关单元测试全部通过。
6. [ ] 不再注册 LibreChat PWA Service Worker，旧缓存不会持续接管页面渲染。

## 6. 触发器与门禁
- 命中：Go 代理代码、Web UI 页面/路由/导航、测试、文档。
- 本地必跑（最小集）：
  - `go fmt ./... && go vet ./...`
  - `make check lint && make test`
  - `make generate && make css`（如前端产物变更）
  - `make check doc`

## 7. 直接验证用例（239A）

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
  - 不再要求点击右侧 Confirm/Commit。

### Case 6：结果落库核验
- 操作：登录后访问 `http://localhost:8080/app/org/units`，检索上述新建组织名称。
- 预期：
  - 组织可检索；
  - 生效日期与提示词一致；
  - 验证 One Door 写入链路成功。
