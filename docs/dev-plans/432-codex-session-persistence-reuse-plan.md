# DEV-PLAN-432：Codex 会话持久化、索引与恢复语义复用/重构方案

**状态**: 部分完成（2026-04-22 CST；`PR-437C` 已完成最小封板范围：正式数据面、最小 lifecycle API、抽屉恢复、store/API/UI 共用 reconstruction roundtrip golden、压缩后恢复与跨租户 fail-closed 验证均已补齐；更大范围的 summary/usage event 数据面与后续持久化扩展仍待推进）

## 0. 适用范围与评审分级

- **评审分级**：`T3`
- **范围一句话**：承接 `DEV-PLAN-430` 的 Slice 3“会话持久化”，最大化复用或重构 OpenAI Codex 开源仓库中与会话持久化直接相关的 append-only 历史、session/thread 索引、archive/resume 生命周期、rollout 记录与 thread history reconstruction 语义；底层存储不复用本地 JSONL/rollout 文件，而是按本仓 PostgreSQL、RLS、Authz、审计要求落地。
- **关联模块/目录**：`docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`、`docs/dev-plans/431-codex-ui-protocol-and-shell-reuse-plan.md`、`docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md`、`modules/cubebox`（候选新模块路径）、`internal/server`、`migrations`、`modules/*/infrastructure/persistence/schema`、`sqlc`
- **外部来源**：`https://github.com/openai/codex`
- **核心参考文件/目录**：
  - `codex-rs/core/src/message_history.rs`
  - `codex-rs/protocol/src/message_history.rs`
  - `codex-rs/core/src/state/session.rs`
  - `codex-rs/rollout/src/session_index.rs`
  - `codex-rs/rollout/src/metadata.rs`
  - `codex-rs/rollout/src/recorder.rs`
  - `codex-rs/rollout/src/state_db.rs`
  - `codex-rs/core/src/rollout.rs`
  - `codex-rs/core/src/session/rollout_reconstruction.rs`
  - `codex-rs/app-server-protocol/src/protocol/thread_history.rs`
  - `codex-rs/thread-store/src/local/archive_thread.rs`
  - `codex-rs/thread-store/src/local/unarchive_thread.rs`
- **关联计划/标准**：`DEV-PLAN-004M1`、`DEV-PLAN-012`、`DEV-PLAN-015`、`DEV-PLAN-019`、`DEV-PLAN-021`、`DEV-PLAN-022`、`DEV-PLAN-024`、`DEV-PLAN-025`、`DEV-PLAN-300`、`DEV-PLAN-430`、`DEV-PLAN-431`、`DEV-PLAN-434`、`DEV-PLAN-437`、`DEV-PLAN-437A`

### 0.1 Simple > Easy 三问

1. **边界**：本计划只处理 CubeBox 的会话持久化、索引、恢复与归档语义；`431` 只承接会话列表/恢复入口的 UI 壳与抽屉恢复交互，不持有 `list/read/resume/archive/rename` 的生命周期 contract；上下文压缩内核由 `434` 承接，AI 网关和模型调用由 `430` 主计划承接。
2. **不变量**：除本地文件存储与单机路径模型不适合内容外，Codex 中可复用的 append-only 历史、session index、thread lifecycle、rollout/reconstruction 语义应优先复用或重构；禁止在未评估 Codex 对应机制前自研一套平行会话持久化语义。
3. **可解释**：reviewer 必须能在 5 分钟内说明哪些 Codex 持久化语义被采纳、哪些因多租户/RLS/审计要求必须重构、哪些明确不引入，以及如何验证恢复/归档/重命名/压缩后恢复的行为正确；同时能区分“UI 入口在 431”与“生命周期 contract 在 432”的 owner 边界。

## 1. 背景

`DEV-PLAN-430` 当前将 Slice 3 定义为 conversation、message、summary、usage event 的 schema 和 sqlc，以及新建、列出、恢复、归档、删除会话。用户要求沿用 431/434 的原则：如果 Codex 开源仓库已有成熟持久化语义，应优先复用或重构，而不是另起一套完全自研模型。

同时，`431` 已被收口为 UI 壳与前端状态机计划，因此 `conversation list/read/resume/archive/rename` 的生命周期 contract、持久化语义与 API 行为必须由本计划持有，`431` 只消费其结果。

Codex 开源仓库中与会话持久化强相关的成熟机制包括：

- append-only message history：`history.jsonl` 逐行追加，天然利于恢复和审计。
- session/thread index：`session_index.jsonl` 追加写入，最新条目覆盖旧名称。
- rollout 记录：以事件流记录 thread 运行轨迹。
- thread history reconstruction：从 rollout/event 重建 UI 可消费的 thread/turn 视图。
- archive/unarchive/read/list/resume 生命周期。
- session-scoped mutable state 与 persisted history 分层。

这些能力与 CubeBox 的需求高度重合。差异只在于：Codex 偏向单机本地文件和本地状态 DB，而 CubeBox 必须落在本仓 PostgreSQL、RLS、Authz、审计与多租户约束下。

## 2. 目标

1. 对 Codex 会话持久化相关源码完成许可证、依赖、数据语义和恢复路径评估。
2. 冻结 CubeBox 的 conversation/message/summary/index/archive/resume 语义，以 Codex 相关实现为优先参考。
3. 复用或重构以下核心语义：
   - append-only 原始消息历史。
   - 会话索引与最新名称覆盖。
   - 会话归档/反归档。
   - 会话列表/读取/恢复。
   - 事件流记录与 timeline reconstruction。
   - 压缩后恢复与历史重建。
4. 将 Codex 的本地文件模型映射为本仓 PostgreSQL 模块 schema + sqlc + RLS。
5. 确保原始消息、压缩摘要、compaction event、usage event、UI event reducer 间的关系可审计、可回放、可恢复。
6. 输出 430 Slice 3 的最小可用持久化基线，使其不从零设计同类语义。

## 3. 非目标

1. 不 vendoring 整个 `openai/codex` 仓库。
2. 不使用 Codex 的 `history.jsonl`、`session_index.jsonl`、rollout 文件或 state db 作为本仓运行时存储。
3. 不引入本地 home 目录扫描、thread path 查找、resume picker 文件系统逻辑。
4. 不引入 Codex 账号、登录、ChatGPT plan、cloud auth 与 rate limit 存储模型。
5. 不绕过本仓 PostgreSQL、RLS、Authz、Atlas/Goose、sqlc 闭环。
6. 不让 compaction summary 替代原始消息。
7. 不引入 shell/file/patch/exec/plugin 的持久化数据模型。

## 4. Codex 持久化资产采纳矩阵

| Codex 资产 | 代表位置 | CubeBox 采纳策略 |
| --- | --- | --- |
| append-only message history | `core/src/message_history.rs` | 采纳语义，底层改为 PostgreSQL message log |
| `HistoryEntry` 结构 | `protocol/src/message_history.rs` | 采纳最小概念，映射到 `conversation_id + ts + payload` |
| session-scoped mutable state | `core/src/state/session.rs` | 采纳分层思想；运行时 state 与持久化 state 分离 |
| session index | `rollout/src/session_index.rs` | 采纳最新索引覆盖旧名称的语义，落为 DB index/event |
| rollout metadata | `rollout/src/metadata.rs` | 借鉴 session meta 字段分层，不直接复用文件形态 |
| rollout recorder | `rollout/src/recorder.rs` | 采纳事件记录思想，落为 event/audit 表 |
| state db / rollout storage | `rollout/src/state_db.rs` | 不复用存储实现，只参考职责拆分 |
| rollout reconstruction | `core/src/session/rollout_reconstruction.rs` | 采纳恢复与重建语义 |
| thread history builder | `app-server-protocol/src/protocol/thread_history.rs` | 强制与 431 对齐，重构为前后端共享恢复逻辑 |
| archive/unarchive | `thread-store/src/local/archive_thread.rs` | 采纳生命周期语义，改为 DB 状态变更 |
| thread list/read/resume tests | `app-server/tests/suite/v2/thread_*.rs` | 采纳验收样式，转成本仓 API/E2E |

## 5. CubeBox 持久化架构

### 5.1 数据分层

- `conversation`：会话主记录，含标题、状态、归档状态、创建人、更新时间。
- `message_log`：append-only 原始消息流，按 conversation 有序写入。
- `summary_log`：压缩摘要记录，引用 source message range。
- `compaction_event`：压缩触发、执行、完成、失败事件。
- `usage_event`：模型调用输入输出 token、延迟、错误码。
- `conversation_index_view`：供 list/read/resume 使用的查询视图或物化结果。

### 5.2 原则

- 原始消息 append-only，不覆盖、不回写。
- 会话重命名、归档、恢复采用事件或版本化状态，最新状态对外生效。
- 前端读取会话列表时依赖索引视图，不扫描全量 message_log。
- timeline 恢复依赖 message_log + summary_log + compaction_event 的组合，不以单一 summary 为事实源。
- 压缩前后都必须能还原 conversation timeline。
- RLS 和 Authz 必须作用于 conversation 和其所有子记录。

### 5.3 恢复语义

CubeBox 恢复行为要对齐 Codex 的 resume/read 思路：

1. 根据 conversation id 读取主记录和最新状态。
2. 读取 message_log、summary_log、compaction_event、必要 usage event。
3. 通过与 431 对齐的 timeline reconstruction/reducer 还原前端视图。
4. 重新生成当前 canonical context，不信任旧权限上下文。
5. 如 active turn 未完成，恢复为“可继续观察或重新发起”的明确状态，不静默伪装为完成。

补充冻结：

- `432` 对 `431` 的正式恢复输出，在 `Phase A` 起统一对齐 `DEV-PLAN-437A` 中定义的 `CanonicalEvent` envelope 与 `TimelineEventStream` 形状。
- `432` 不再为恢复路径额外交付只给页面态消费的第二套 timeline DTO。

## 6. 数据对象建议

新增表前仍需用户手工确认；本计划只冻结对象职责：

- `cubebox_conversations`
- `cubebox_conversation_events`
- `cubebox_message_log`
- `cubebox_summary_log`
- `cubebox_compaction_events`
- `cubebox_usage_events`

如果为了简化首期实现，需要合并表，也必须保持 append-only message log 和 summary/event 分层不丢失。

## 7. 实施切片

### Slice 0：Codex 持久化评估

- [ ] 固定 Codex 参考 commit SHA。
- [ ] 确认 Apache-2.0 许可证、NOTICE 和复制要求。
- [ ] 盘点 `message_history.rs`、`session_index.rs`、`rollout_reconstruction.rs`、`thread_history.rs` 的依赖闭包。
- [ ] 输出“采纳语义 / 重构实现 / 不引入”清单。

### Slice 1：持久化模型冻结

- [ ] 定义 conversation、message log、summary log、compaction event、usage event 职责。
- [ ] 定义 list/read/resume/archive/unarchive/rename 生命周期。
- [ ] 明确 append-only 规则和状态覆盖规则。
- [ ] 与 431 的 UI event / timeline reconstruction 契约对齐，并明确哪些字段只作为 UI 消费面暴露。

### Slice 2：Schema 与 sqlc 设计

- [ ] 形成 PostgreSQL schema 草案。
- [ ] 新增表前向用户提交对象清单并获得手工确认。
- [ ] 完成 Goose migration、Atlas schema、sqlc query 设计。
- [ ] 确保 RLS、租户字段、索引和审计字段完整。

### Slice 3：写入路径

- [ ] 实现新建 conversation。
- [ ] 实现 append-only message log 写入。
- [ ] 实现 rename/archive/unarchive 事件或状态更新。
- [ ] 实现 summary/compaction/usage event 写入。

### Slice 4：读取与恢复路径

- [ ] 实现 list conversations。
- [ ] 实现 read conversation。
- [ ] 实现 resume conversation。
- [ ] 实现 reconstruction，把持久化记录恢复为 UI timeline 输入。

### Slice 5：删除与保留策略

- [ ] 明确删除是物理删除、软删除还是仅归档。
- [ ] 删除策略必须与审计要求一致，不得破坏 append-only 事实链。
- [ ] 若允许删除，必须明确哪些记录可删、哪些仅可隐藏。

### Slice 6：测试与验收

- [ ] 追加写入测试：原始消息不覆盖。
- [ ] 索引覆盖测试：最新名称/状态生效。
- [x] archive/unarchive/read/list/resume 测试：已完成当前最小封板范围。现已覆盖 list/read/resume 的前端恢复链路、后端 create/list/load/stream/interrupt API、`PATCH /internal/cubebox/conversations/{conversation_id}` 的 rename/archive/unarchive handler 成功路径，以及 store/API/UI 三层共享的 lifecycle roundtrip golden。
- [x] 压缩后恢复测试：summary 不替代原始消息。
- [x] 跨租户隔离测试：当前 `cubebox_conversations` / `cubebox_conversation_events` 主链已补 fail-closed 验证。
- [x] UI reconstruction fixture / golden 测试，与 431 reducer 对齐。
- [x] 新增供 `431` 壳层消费的 fixture/contract 样式，确保 UI 入口与生命周期 owner 分离。

### Slice 7：430 回填与封板

- [ ] 更新 `DEV-PLAN-430` Slice 3 引用本计划。
- [ ] readiness 记录 Codex 参考 commit、复用清单、schema 对象清单、测试结果。
- [ ] 执行文档、Go、sqlc、routing、authz、preflight 验证。

## 8. 验收标准

- [ ] Codex 参考 commit 与许可证评估已记录。
- [ ] 已形成 append-only message log + summary/event 分层模型。
- [x] list/read/resume/archive/unarchive/rename 生命周期冻结：已完成当前最小封板范围。正式 API 与 UI 消费链路已落地，后端 handler/store 验证与跨层 roundtrip golden 证据已补齐。
- [x] timeline 可由持久化记录重建。
- [x] 原始消息不因压缩被覆盖。
- [ ] summary、compaction、usage event 可按 conversation 追溯。
- [ ] RLS/租户隔离覆盖所有持久化对象。
  当前 `cubebox_conversations` / `cubebox_conversation_events` 主链已补 fail-closed 测试；未来新增 summary/usage 等持久化对象时仍需按同口径继续补齐。
- [x] UI 恢复行为与 431 reducer 对齐。
- [ ] `make check chat-surface-clean` 仍通过。

## 9. Stopline

- 不得直接采用 Codex JSONL/rollout 文件作为本仓运行时存储。
- 不得在未评估 Codex 持久化语义前自研平行 conversation lifecycle。
- 不得让 summary 替代原始消息。
- 不得绕过 PostgreSQL、RLS、Authz、sqlc、migration 闭环。
- 不得在未获用户手工确认前新增数据库表。
- 不得将本地文件路径、home 目录扫描、resume picker 文件系统逻辑带入本仓。

## 10. 本地必跑与门禁

- 文档变更：`make check doc && make markdownlint`
- Go 代码：`go fmt ./... && go vet ./... && make check lint && make test`
- DB/sqlc：按模块执行 schema/migration/sqlc 闭环，新增表前必须获得用户手工确认
- Routing/Authz/API 变更：`make check routing && make authz-pack && make authz-test && make authz-lint`
- 旧栈反回流：`make check chat-surface-clean`
- PR 前：`make preflight`

## 11. 参考链接

- OpenAI Codex 仓库：`https://github.com/openai/codex`
- Codex `message_history.rs`：`https://github.com/openai/codex/blob/main/codex-rs/core/src/message_history.rs`
- Codex `protocol/message_history.rs`：`https://github.com/openai/codex/blob/main/codex-rs/protocol/src/message_history.rs`
- Codex `state/session.rs`：`https://github.com/openai/codex/blob/main/codex-rs/core/src/state/session.rs`
- Codex `session_index.rs`：`https://github.com/openai/codex/blob/main/codex-rs/rollout/src/session_index.rs`
- Codex `thread_history.rs`：`https://github.com/openai/codex/blob/main/codex-rs/app-server-protocol/src/protocol/thread_history.rs`
- DEV-PLAN-430：`docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`
- DEV-PLAN-431：`docs/dev-plans/431-codex-ui-protocol-and-shell-reuse-plan.md`
- DEV-PLAN-434：`docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md`
