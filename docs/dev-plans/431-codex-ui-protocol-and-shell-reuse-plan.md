# DEV-PLAN-431：Codex UI 协议、状态机与右悬挂壳层复用/重构方案

**状态**: 规划中（2026-04-19 21:18 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T3`
- **范围一句话**：承接 `DEV-PLAN-430` 的 Slice 1“UI 壳与用户可见入口”，最大化复用或重构 OpenAI Codex 开源仓库中可用于 UI 的协议、线程/回合状态机、事件流、历史重建 reducer、compact/token 通知和交互模式；不适合直接复用的 Rust TUI/terminal 渲染层不引入，本仓使用 React/MUI 自行实现右侧悬挂抽屉视觉层。
- **关联模块/目录**：`docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`、`docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md`、`apps/web`、`internal/server`、`modules/cubebox`（候选新模块路径）、`config/routing`、`config/access`、`config/errors`
- **外部来源**：`https://github.com/openai/codex`
- **核心参考文件/目录**：
  - `codex-rs/app-server-protocol/src/protocol/thread_history.rs`
  - `codex-rs/app-server-protocol/schema/json/v2/*.json`
  - `codex-rs/protocol/src/message_history.rs`
  - `codex-rs/core/src/session/**`
  - `codex-rs/core/src/message_history.rs`
  - `codex-rs/tui/src/chatwidget/**`
  - `codex-rs/tui/src/bottom_pane/**`
  - `codex-rs/tui/src/markdown_stream.rs`
  - `codex-rs/tui/src/history_cell.rs`
  - `codex-rs/tui/src/slash_command.rs`
  - `codex-rs/tui/src/status*.rs`
- **关联计划/标准**：`DEV-PLAN-004M1`、`DEV-PLAN-012`、`DEV-PLAN-015`、`DEV-PLAN-017`、`DEV-PLAN-019`、`DEV-PLAN-021`、`DEV-PLAN-022`、`DEV-PLAN-300`、`DEV-PLAN-430`、`DEV-PLAN-434`、`DEV-PLAN-437`、`DEV-PLAN-437A`

### 0.1 Simple > Easy 三问

1. **边界**：本计划只处理 CubeBox 的 UI 协议、前端状态机、事件流和右悬挂壳层；AI 网关由 430 Slice 2 承接，上下文压缩内核由 434 承接；`340-383` 与 `380A-380G` 系列只保留为历史背景，不再构成当前 UI 主线的实现依据。
2. **不变量**：除 Rust terminal/TUI 视觉渲染、shell/file/patch/exec/plugin 写操作等不适合内容外，Codex 中可复用的 UI 协议、线程/回合模型、事件粒度、历史重建 reducer 和交互状态应优先复用或重构；不得在未评估 Codex 对应机制前重新设计一套平行概念；不得因为引入右侧抽屉而形成第二前端主链。
3. **可解释**：reviewer 必须能在 5 分钟内说明 UI 视觉层为何本仓自研、UI 协议/状态机为何参考 Codex、哪些事件被采纳、哪些高风险能力被裁掉，以及用户如何从右侧悬挂入口完成最小对话闭环。

## 1. 背景

`DEV-PLAN-430` 已定义 CubeBox 的首期用户可见入口：Web Shell 右侧悬挂抽屉、点击图标拉出、会话列表、输入框、模型选择、上下文 chips、流式回复和会话恢复。用户进一步确认：虽然 Codex 开源仓库中的 Rust TUI 不适合直接复用为 Web/MUI 组件，但 Codex 的 UI 协议、线程/回合状态机、事件流和历史重建 reducer 应按“应复用则复用”的原则引进。

同时，`DEV-PLAN-430` 已明确新一轮 CubeBox 重做不再受 `340-383` 与 `380A-380G` 系列计划约束；这些文档只保留为历史证据，不能继续决定当前 UI 契约、页面形态、阶段划分或完成定义。

Codex 开源仓库中与 UI 层高度相关的成熟资产包括：

- app-server protocol：`ThreadStart`、`ThreadRead`、`ThreadList`、`ThreadResume`、`ThreadArchive`、`ThreadCompactStart`、`TurnInterrupt`、`AgentMessageDelta`、`TurnCompleted`、`ContextCompactedNotification`、`ThreadTokenUsageUpdatedNotification` 等 schema。
- thread history builder：把底层 rollout/event 重建为 UI 可消费的 `Turn` 列表。
- streaming event model：区分 user message、agent message delta、reasoning、tool item started/completed、error、turn complete、compact、token usage。
- TUI 交互模式：输入区、历史 cell、markdown stream、slash command、status indicator、compact warning、interrupt/stop 等。

这些内容可以显著降低 CubeBox UI 协议和状态机重复造车风险。

但本计划必须同时满足本仓现行前端单链路约束：右侧悬挂抽屉是 `CubeBox` 正式聊天 UI 的一种承载壳层，不是第二套正式页面、第二套路由或第二套 store。

## 2. 目标

1. 对 Codex UI 相关源码完成许可证、依赖、协议粒度和安全边界评估。
2. 冻结 CubeBox UI 事件协议，以 Codex app-server protocol v2 为优先参考。
3. 重构 Codex thread/turn/item/event 概念为 CubeBox 前后端共享契约。
4. 重构 Codex `ThreadHistoryBuilder` 思路，形成 CubeBox 前端可消费的 conversation timeline reducer。
5. 重构 Codex streaming UI 状态：turn started、message delta、item started/completed、turn complete、error、compact、token usage、interrupt。
6. 借鉴 Codex TUI 交互模式，实现 Web/MUI 右侧悬挂抽屉：输入区、消息历史、流式 markdown、状态条、compact/token 提示、stop/interrupt。
7. 裁掉 Codex 中不适合 CubeBox 首期的 shell/file/patch/exec/plugin/MCP 写操作协议和 terminal 渲染实现。
8. 产出 430 Slice 1 的最小用户可见闭环：打开抽屉 -> 新建/恢复会话 -> 发送消息 -> 流式回复 -> 停止/完成 -> 关闭重开后状态恢复。
9. 将正式 UI 主链收口为右侧抽屉承载，避免再次形成页面版/抽屉版双实现。

## 3. 非目标

1. 不 vendoring 整个 `openai/codex` 仓库。
2. 不引入 `codex-rs/tui` 作为运行时或前端依赖。
3. 不把 ratatui/terminal rendering、alternate screen、terminal key handling 移植到 Web。
4. 不引入 Codex app-server/exec-server 作为本仓运行时进程。
5. 不启用 Codex 的 shell command、file write、apply patch、exec approval、plugin install、marketplace 等能力。
6. 不把 Codex 的认证、账号、ChatGPT plan、rate limit UI 作为本仓事实源。
7. 不替代本仓 MUI 设计系统、丘比蓝主题、i18n、路由、Authz 和错误码契约。
8. 不把历史 `340-383` 与 `380A-380G` 系列的页面骨架、测试语义或 API 形态视为当前实现必须兼容的前提。

## 4. Codex UI 资产采纳矩阵

| Codex 资产 | 代表位置 | CubeBox 采纳策略 |
| --- | --- | --- |
| Thread lifecycle | app-server-protocol v2 thread schemas | 采纳概念，映射为 conversation start/read/list/resume/archive/set-name |
| Turn lifecycle | `TurnStarted` / `TurnCompleted` / `TurnInterrupt` | 采纳，映射为前端 turn 状态机 |
| Message delta | `AgentMessageDeltaNotification` | 采纳，作为 SSE/WebSocket delta 事件参考 |
| Item started/completed | `ItemStartedNotification` / `ItemCompletedNotification` | 采纳，用于工具/上下文 provider/compact 状态展示 |
| Context compacted | `ContextCompactedNotification` | 采纳，并与 434 compaction 事件对齐 |
| Token usage update | `ThreadTokenUsageUpdatedNotification` | 采纳，用于抽屉状态条和阈值提示 |
| Thread history reducer | `thread_history.rs` | 重构，形成 CubeBox timeline reducer |
| Markdown streaming | `markdown_stream.rs` | 借鉴行为，不直接移植 terminal renderer |
| History cell | `history_cell.rs` | 借鉴消息分组/状态展示，不直接移植 UI 代码 |
| Slash command | `slash_command.rs` | 借鉴 `/compact`、`/new`、`/clear` 等模式，命令集合本仓裁剪 |
| Status indicator | `status*.rs` | 借鉴状态分类，使用 MUI 实现 |
| Bottom pane/chat widget | `bottom_pane/**`、`chatwidget/**` | 借鉴交互信息架构，不直接复用渲染代码 |
| Shell/file/patch/exec | protocol v2 exec/fs/patch schemas | 不引入首期默认能力 |
| Plugin/marketplace | protocol v2 plugin/marketplace schemas | 不引入 |
| TUI key handling | `tui/**` | 不引入 |

## 4A. 上游映射表模板

本计划的实现、PR 与 readiness 必须直接引用下表；未填完前不得开始 Slice 1-6 的实现。

| 上游项目 | 上游 commit SHA | 上游制品类型 | 上游路径或对象名 | CubeBox 对应对象/切片 | 采用状态 | 不可直接复用原因 | 原因类型 | 必备验证 | PR 证据位置 | readiness 证据位置 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `openai/codex` | `待补` | `协议` | `codex-rs/app-server-protocol/schema/json/v2/*.json` | `CubeBox UI event schema / Slice 1` | `待补` | `待补` | `待补` | `schema fixture + snapshot` | `待补` | `待补` |
| `openai/codex` | `待补` | `文件` | `codex-rs/app-server-protocol/src/protocol/thread_history.rs` | `timeline reducer / Slice 2` | `待补` | `待补` | `待补` | `golden reducer fixture` | `待补` | `待补` |
| `openai/codex` | `待补` | `文件` | `codex-rs/tui/src/markdown_stream.rs` | `streaming markdown 行为 / Slice 4` | `待补` | `待补` | `待补` | `streaming snapshot` | `待补` | `待补` |
| `openai/codex` | `待补` | `文件` | `codex-rs/tui/src/slash_command.rs` | `composer 命令入口 / Slice 6` | `待补` | `待补` | `待补` | `command parser fixture` | `待补` | `待补` |
| `openai/codex` | `待补` | `页面信息架构` | `codex-rs/tui/src/chatwidget/**`、`codex-rs/tui/src/bottom_pane/**` | `drawer/timeline/composer/status bar 信息架构 / Slice 3` | `待补` | `待补` | `待补` | `IA snapshot + E2E` | `待补` | `待补` |

填写规则：

- `采用状态` 只允许填写 `直接复用`、`重构复用`、`只借鉴语义`、`明确不引入`。
- 若某行是 `只借鉴语义` 或 `明确不引入`，`不可直接复用原因` 必须写到仓库约束级别，例如 `前端单主链`、`MUI 渲染栈`、`非 terminal 环境`、`禁止 shell/file/patch`。
- `必备验证` 至少要锁住协议形状、事件序列或 UI 行为之一；不能只写“页面能打开”。

## 4B. PR-437A 首轮最小冻结

首轮固定 `openai/codex` 参考 commit SHA：

- `ef071cf816950dc416b2a975e7ed023eea639026`

`PR-437A` 只冻结首轮会消费的最小对象，不要求一次性补齐 `4A` 全表：

| 上游项目 | 上游 commit SHA | 上游制品类型 | 上游路径或对象名 | CubeBox 对应对象/切片 | 采用状态 | 不可直接复用原因 | 原因类型 | 必备验证 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `openai/codex` | `ef071cf816950dc416b2a975e7ed023eea639026` | `协议` | `codex-rs/app-server-protocol/schema/json/v2/*.json` | `Phase A/B event naming + SSE envelope` | `重构复用` | 需按本仓 conversation/turn/event 命名与错误码契约收口 | `协议不匹配` | `schema fixture + snapshot` |
| `openai/codex` | `ef071cf816950dc416b2a975e7ed023eea639026` | `文件` | `codex-rs/app-server-protocol/src/protocol/thread_history.rs` | `timeline reducer / reconstruction 对齐` | `重构复用` | `432` 恢复输出与 `431` reducer 需共享同形 contract，不能直接照搬 Rust 类型 | `DDD 边界` | `golden reducer fixture` |
| `openai/codex` | `ef071cf816950dc416b2a975e7ed023eea639026` | `文件` | `codex-rs/tui/src/markdown_stream.rs` | `流式 delta 合并行为` | `只借鉴语义` | 本仓使用 React/MUI，不引入 terminal renderer | `依赖不兼容` | `streaming snapshot` |

`431` 在 `PR-437A` 与 `PR-437B` 必须同时消费 `DEV-PLAN-437A` 的共享 canonical contract；不得让 reducer、SSE 客户端和恢复页各自维护不同命名。

## 5. CubeBox UI 架构

### 5.1 前端分层

- `shell entry`：全局右侧图标、抽屉挂载、响应式布局。
- `conversation store`：管理 conversation list、active conversation、turns、items、streaming status。
- `event reducer`：参考 Codex `ThreadHistoryBuilder`，把后端事件流规整为 UI timeline。
- `composer`：输入框、上下文 chips、模型选择、发送/停止、slash command。
- `timeline`：用户消息、助手消息、流式 markdown、工具/context item、compact item、错误 item。
- `status bar`：模型、token usage、compact 状态、连接状态、错误提示。
- `settings entry`：跳转模型配置页或打开配置面板。

### 5.1A 单前端链路约束

- 右侧抽屉不是第二产品入口，而是 `CubeBox` 唯一正式聊天 UI 承载壳层。
- 右侧抽屉必须承载同一套：
  - `API client`
  - `conversation store`
  - `event reducer`
  - `timeline/composer/status bar` 组件语义
- 移动端或窄屏可退化为全屏页面或 bottom sheet，但仍属于同一套 UI 主链，不得形成抽屉版/页面版两套独立状态模型。
- 不允许新增第二套路由、第二套前端 store、第二套错误映射或第二套 SSE 消费逻辑来专门服务抽屉壳层。

### 5.2 后端到前端事件契约

首期建议冻结以下 CubeBox UI event，名称可参考 Codex，但字段需 HRMS 化：

- `conversation.started`
- `conversation.loaded`
- `conversation.archived`
- `conversation.renamed`
- `turn.started`
- `turn.user_message.accepted`
- `turn.agent_message.delta`
- `turn.agent_message.completed`
- `turn.context_item.started`
- `turn.context_item.completed`
- `turn.compaction.started`
- `turn.context_compacted`
- `turn.token_usage.updated`
- `turn.error`
- `turn.interrupted`
- `turn.completed`

这些事件既可由 SSE 传输，也可由 WebSocket 传输；首期优先 SSE，除非 430 网关切片另行裁决。

`Phase A` 最小事件名、SSE envelope、reducer 输入与 reconstruction 输出的共享 companion doc 冻结为 `DEV-PLAN-437A`；本节后续扩展不得与其冲突。

事件 canonical 语义必须由 `CubeBox` 正式模块持有；`internal/server` 只承接 HTTP/SSE delivery 与 adapter，不得把 thread/turn/event 语义再次散落在 delivery 层。

### 5.3 Timeline reducer

CubeBox 前端必须有一个纯函数 reducer：

- 输入：有序事件流、当前 timeline snapshot。
- 输出：conversation timeline、active turn、streaming message、status bar state、error state。
- 不直接访问 DOM，不直接发网络请求。
- 可用 Codex `ThreadHistoryBuilder` 的测试模式做 prompt/timeline shape 快照。
- reducer 的 golden/snapshot 应优先复用或对齐上游 `ThreadHistoryBuilder` 行为，而不是只保留“受其启发”的口头说明。

### 5.4 右侧悬挂抽屉

- 桌面宽屏：固定右侧抽屉，可与主页面并行。
- 中等宽度：覆盖式抽屉。
- 移动端：全屏对话页或 bottom sheet。
- 抽屉关闭不终止后端 turn；重新打开后通过 conversation read/resume 恢复 timeline。
- 用户切换租户或权限变化时，抽屉必须重新校验 active conversation 可见性。
- 页面路由形态与抽屉形态必须复用同一套 timeline 组件和同一份 active conversation 状态，不得分别维护。

## 6. 实施切片

### Slice 0：Codex UI 资产评估

- [ ] 固定 Codex 参考 commit SHA，不使用浮动 `main` 作为实现依据。
- [ ] 确认 Apache-2.0 许可证、NOTICE 和复制要求。
- [ ] 盘点 app-server-protocol v2 与 TUI 相关依赖闭包。
- [ ] 输出“直接采纳协议 / 重构状态机 / 借鉴交互 / 不引入”清单。
- [ ] 按本计划 `4A` 模板补齐文件级上游映射表，并为每个对象冻结采用状态与不可复用原因。
- [ ] `PR-437A` 先以 `4B` 的最小冻结集满足首轮开工条件，再逐步补齐 `4A` 全表。
- [ ] 冻结“具体复用制品”清单，至少明确：
  - `JSON schema` 是否直接消费
  - `TypeScript schema/types` 是否直接消费或生成
  - `ThreadHistoryBuilder` 的哪些行为以 golden/snapshot 方式继承
  - 哪些只保留为交互参考，不进入本仓制品链

### Slice 1：UI 事件契约冻结

- [ ] 定义 CubeBox conversation、turn、item、delta、compact、token、error event。
- [ ] 将 Codex thread/turn 术语映射到 CubeBox conversation/turn。
- [ ] 明确裁掉 shell/file/patch/exec/plugin 事件。
- [ ] 定义 SSE event 格式与错误映射。

### Slice 2：前端 timeline reducer

- [ ] 重构 Codex `ThreadHistoryBuilder` 思路，建立 CubeBox timeline reducer。
- [ ] 支持 user message、agent delta、agent completed、context item、compact item、error、interrupt、turn complete。
- [ ] 增加 reducer 单测和 snapshot。
- [ ] 确保重复 delta、乱序完成、断线重连后的恢复行为可测试。

### Slice 3：右悬挂壳层与基础组件

- [ ] 在 Web Shell 增加右侧 CubeBox 图标。
- [ ] 实现右侧抽屉布局、响应式策略和主题变量。
- [ ] 实现 conversation header、timeline、composer、status bar、empty state。
- [ ] 实现抽屉开关状态持久化，但不保存敏感内容。
- [ ] 确认右侧抽屉为唯一正式承载面，不新增第二套路由、第二套页面或第二主链。

### Slice 4：流式消息与状态显示

- [ ] 接入后端 SSE mock 或 deterministic provider。
- [ ] 实现 markdown 流式渲染，借鉴 Codex markdown streaming 行为。
- [ ] 实现 stop/interrupt。
- [ ] 实现 token usage、compact started/context compacted、error 状态提示。

### Slice 5：会话入口与抽屉恢复交互

- [ ] 实现 conversation list/read/resume/archive/rename 的 UI 入口与展示；生命周期语义、持久化 contract 与 API owner 以 `DEV-PLAN-432` 为准。
- [ ] 关闭抽屉后恢复 active conversation。
- [ ] 权限或租户变化时重新加载并 fail-closed。
- [ ] 增加 E2E 覆盖打开、关闭与抽屉恢复；会话恢复/归档正确性由 `DEV-PLAN-432` 的 API/E2E 承接。

### Slice 6：composer 命令入口与快捷操作

- [ ] 实现最小 slash command 输入解析与 UI 入口：`/new`、`/compact`、`/clear-draft`。
- [ ] `/new` 只在 UI 层触发新会话入口；会话新建与 lifecycle contract 以 `DEV-PLAN-432` 为准。
- [ ] `/compact` 只在 UI 层触发 manual compact 入口；compaction 语义与执行链以 `DEV-PLAN-434` 为准。
- [ ] `/clear-draft` 由前端草稿状态直接处理。
- [ ] 命令解析参考 Codex TUI 思路，但命令集合由 CubeBox 白名单控制。
- [ ] 不引入 shell、file、patch、exec 类命令。

### Slice 7：430 回填与封板

- [ ] 更新 `DEV-PLAN-430` Slice 1 引用本计划。
- [ ] readiness 记录 Codex 参考 commit、采纳/裁剪矩阵、UI 截图或录像、测试结果。
- [ ] 执行文档、前端、Go、routing、authz、E2E 和 preflight 验证。

## 7. 测试与验收

- [ ] UI event reducer 纯函数测试覆盖主要事件。
- [ ] timeline snapshot 覆盖 user/agent/context/compact/error/interrupt/complete。
- [ ] Codex `thread_history` 对照 fixture 已冻结，CubeBox reducer 输出与映射表中的上游行为一致。
- [ ] 抽屉开关测试覆盖桌面、中等宽度、移动端。
- [ ] SSE 流式渲染测试覆盖 delta 合并、完成态、错误态和中断态。
- [ ] 协议 schema、delta 事件、slash command 和 IA snapshot 都有可回归的 golden/snapshot 证据。
- [ ] 会话恢复测试覆盖关闭抽屉、刷新页面、重新读取 conversation。
- [ ] 权限/租户变化测试覆盖 fail-closed。
- [ ] `make check chat-surface-clean` 仍能阻断旧对话栈回流。

## 8. Stopline

- 不得直接移植 Codex Rust TUI 渲染层。
- 不得引入 terminal rendering、alternate screen、terminal key handling。
- 不得引入 shell/file/patch/exec/plugin/marketplace 作为默认 UI 能力。
- 不得在未评估 Codex app-server-protocol 前自定义平行 thread/turn/event 模型。
- 不得在 `4A` 映射表缺失 `commit SHA`、文件级对象或采用状态时开始实现 reducer、schema 或 drawer 主链。
- 不得只写“借鉴 Codex 交互”而不落到具体文件、协议或测试样例。
- 不得为了 CubeBox 再新增第二套路由、第二套 store 或第二套聊天页面实现。
- 不得把 Codex 账号、登录、ChatGPT plan、rate limit UI 作为本仓事实源。
- 不得在前端保存 API Key 或敏感 prompt 上下文。
- 不得绕过本仓 MUI、i18n、routing、Authz、错误码和 E2E 门禁。

## 9. 本地必跑与门禁

- 文档变更：`make check doc && make markdownlint`
- 前端 UI：`pnpm --dir apps/web check`
- 生成物：涉及 `.templ`、MUI presentation assets 时执行 `make generate && make css`
- Routing/Authz/API 变更：`make check routing && make authz-pack && make authz-test && make authz-lint`
- 旧栈反回流：`make check chat-surface-clean`
- PR 前：`make preflight`

## 10. 参考链接

- OpenAI Codex 仓库：`https://github.com/openai/codex`
- Codex app-server protocol：`https://github.com/openai/codex/tree/main/codex-rs/app-server-protocol`
- Codex thread history builder：`https://github.com/openai/codex/blob/main/codex-rs/app-server-protocol/src/protocol/thread_history.rs`
- Codex TUI：`https://github.com/openai/codex/tree/main/codex-rs/tui/src`
- DEV-PLAN-430：`docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`
- DEV-PLAN-434：`docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md`
