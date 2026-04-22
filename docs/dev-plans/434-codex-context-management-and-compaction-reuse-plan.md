# DEV-PLAN-434：Codex 上下文管理与压缩机制复用/重构方案

**状态**: 规划中（2026-04-19 20:48 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T3`
- **范围一句话**：在 `DEV-PLAN-430` 的 CubeBox（丘宝）重做方案中，优先复用或重构 OpenAI Codex 开源仓库已有的上下文管理、会话历史、token 估算、自动/手动 compaction、canonical context reinjection 与测试模式，最大化避免重复造车。
- **关联模块/目录**：`docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`、`modules/cubebox`（候选新模块路径）、`internal/server`、`apps/web`、`config`、`migrations`、`scripts/ci`
- **外部来源**：`https://github.com/openai/codex`
- **核心参考文件**：
  - `codex-rs/core/src/compact.rs`
  - `codex-rs/core/src/compact_remote.rs`
  - `codex-rs/core/src/tasks/compact.rs`
  - `codex-rs/core/src/context_manager/history.rs`
  - `codex-rs/core/templates/compact/prompt.md`
  - `codex-rs/core/templates/compact/summary_prefix.md`
  - `codex-rs/core/src/compact_tests.rs`
  - `codex-rs/core/tests/suite/compact*.rs`
- **关联计划/标准**：`DEV-PLAN-004M1`、`DEV-PLAN-012`、`DEV-PLAN-015`、`DEV-PLAN-017`、`DEV-PLAN-019`、`DEV-PLAN-021`、`DEV-PLAN-022`、`DEV-PLAN-300`、`DEV-PLAN-430`、`DEV-PLAN-437`、`DEV-PLAN-437A`

### 0.1 Simple > Easy 三问

1. **边界**：本计划只处理 CubeBox 的上下文管理与压缩内核复用/重构；UI、AI 网关、模型配置、数据库对象清单仍由 `DEV-PLAN-430` 主计划裁决；`431` 只可暴露 `/compact` 的 UI 入口，不持有 manual compact 的执行语义、触发器和验收 contract。
2. **不变量**：优先从 Codex 开源实现抽取成熟机制；禁止在未完成差距评估前另起一套自研 compaction；禁止把 Codex 的旧对话栈、CLI/TUI、shell 执行、文件写入、插件、MCP 运行时整体搬入本仓。
3. **可解释**：reviewer 必须能在 5 分钟内说明哪些 Codex 机制被复用、哪些因 HRMS 审计/租户/RLS 约束必须重构、哪些明确不引入，以及如何验证没有重复造车。

## 1. 背景

`DEV-PLAN-430` 已提出 CubeBox 需要 token budget、滑动窗口、摘要压缩、工具输出压缩、结构化状态对象和最近回合原文保留。用户进一步确认：Codex 是开源的，应尽可能复用或重构其上下文管理和压缩实现，而不是从零自研。

OpenAI Codex 开源仓库已经包含较成熟的上下文压缩实现：

- 自动压缩：根据模型 `auto_compact_token_limit` 触发。
- 手动压缩：作为 `CompactTask` 运行。
- pre-turn 压缩：新一轮采样前发现历史过大时压缩。
- mid-turn 压缩：模型需要 follow-up 但上下文达到阈值时压缩。
- model downshift 压缩：切换到更小上下文窗口模型时，先用旧模型上下文压缩。
- remote compaction：provider 支持时调用远端压缩能力。
- replacement history：用摘要和保留的最近用户消息替换活跃历史。
- canonical initial context reinjection：压缩后重新注入当前权威上下文，避免旧 developer/system 内容污染。
- stale developer message filtering：远端压缩结果中过期 developer message 不直接保留。
- telemetry 与测试：记录 compaction 触发原因、阶段、状态、前后 token，并有单测和快照测试。

这些能力与 CubeBox 的需求高度重合。430 的实现应把 Codex 作为优先复用的成熟基线。

## 2. 目标

1. 对 Codex 上下文管理与压缩相关源码完成许可证、依赖、类型、运行时和安全边界评估。
2. 冻结 CubeBox 对 Codex 机制的复用优先级，避免同类机制在本仓重复设计。
3. 抽取或重构以下核心能力：
   - 会话历史容器。
   - token 估算与模型上下文窗口阈值。
   - 自动/手动 compaction 触发器。
   - compaction prompt 与 summary prefix。
   - replacement history 构造。
   - 最近用户消息保留与 token 限制。
   - canonical context reinjection。
   - stale developer/context message 过滤。
   - compaction telemetry。
   - compaction 单元测试与快照测试模式。
4. 将 Codex 面向 coding agent 的概念改造成 CubeBox 的 HRMS 对话上下文：
   - `ResponseItem` 映射为 CubeBox message item。
   - developer/system context 映射为租户、权限、页面、业务对象和安全约束。
   - tool output 映射为业务上下文 provider 输出摘要。
   - shell/file/git 语义作为可选 provider，不进入首期默认 runtime。
5. 保持本仓审计要求：原始消息 append-only，压缩摘要只作为 prompt view，不物理替代审计事实。
6. 输出最小可用复用层，使 430 Slice 4 不再从零实现压缩内核。
7. 持有 `/compact`、auto compact、manual compact 的正式语义、执行链与验收口径；`431` 只负责 composer 命令入口与状态提示。

## 3. 非目标

1. 不 vendoring 整个 `openai/codex` 仓库。
2. 不引入 Codex CLI/TUI/app-server/exec-server 作为运行时进程。
3. 不引入 Codex 的 shell 执行、文件写入、patch、插件、技能或 MCP 工具运行时作为 CubeBox 首期能力。
4. 不绕过本仓 Go + pgx + PostgreSQL 默认方案。
5. 不把 Codex 的上下文替换策略直接照搬为“删除或覆盖原始消息”。
6. 不把 Codex provider、auth、session telemetry 作为本仓事实源。
7. 不因复用 Codex 机制而放宽 `DEV-PLAN-004M1` 与 `DEV-PLAN-430` 已冻结的 no-legacy / 旧对话栈反回流要求。

## 4. Codex 机制拆解与 CubeBox 采纳策略

| Codex 机制 | Codex 代表实现 | CubeBox 采纳策略 |
| --- | --- | --- |
| 历史容器 | `ContextManager` | 重构为 CubeBox 内部历史 view builder；数据库仍 append-only |
| token 估算 | `approx_token_count` 与 byte heuristic | 先复用思路，抽象 `TokenEstimator`，后续按模型替换 tokenizer |
| 自动压缩阈值 | `auto_compact_token_limit` | 采纳，映射到 `model_route.auto_compact_threshold` |
| 手动压缩 | `CompactTask` | 采纳，执行语义由本计划持有，UI 入口映射为 `431` 的 `/compact` 或按钮 |
| pre-turn 压缩 | `run_pre_sampling_compact` | 采纳，发送前同步压缩 |
| mid-turn 压缩 | `run_auto_compact(...BeforeLastUserMessage...)` | P1 采纳；首期可先阻断 follow-up 超限 |
| model downshift | `maybe_run_previous_model_inline_compact` | P1 采纳；模型切换到小窗口时需要 |
| local compaction | `compact.rs` | 采纳 prompt 与 replacement history 思路 |
| remote compaction | `compact_remote.rs` | 作为 provider capability 预留，不首期强依赖 |
| summary prompt | `templates/compact/prompt.md` | 采纳并改写为 HRMS handoff summary 模板 |
| summary prefix | `templates/compact/summary_prefix.md` | 采纳，用于识别摘要消息和防重复摘要 |
| 最近用户消息保留 | `COMPACT_USER_MESSAGE_MAX_TOKENS` | 采纳，转成模型/租户可配置上限 |
| stale developer 过滤 | `should_keep_compacted_history_item` | 强制采纳，权限/租户/模型配置必须重新注入 |
| canonical context reinjection | `insert_initial_context_before_last_real_user_or_summary` | 强制采纳，注入当前权威业务上下文 |
| telemetry | `CodexCompactionEvent` | 采纳字段模型，落到本仓 usage/audit event |
| 测试模式 | `compact_tests.rs` + suite snapshots | 采纳，建立纯函数单测 + prompt shape 快照 |

## 4A. 上游映射表模板

本计划必须把 Codex compaction 机制拆成具体制品，不允许只保留“受其启发”的自研压缩器；未填完前不得进入 Slice 2-6 的实现。

| 上游项目 | 上游 commit SHA | 上游制品类型 | 上游路径或对象名 | CubeBox 对应对象/切片 | 采用状态 | 不可直接复用原因 | 原因类型 | 必备验证 | PR 证据位置 | readiness 证据位置 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `openai/codex` | `待补` | `文件` | `codex-rs/core/src/compact.rs` | `local compaction planner/runner / Slice 2` | `待补` | `待补` | `待补` | `prompt shape golden` | `待补` | `待补` |
| `openai/codex` | `待补` | `文件` | `codex-rs/core/src/compact_remote.rs` | `remote compaction capability / Slice 5` | `待补` | `待补` | `待补` | `remote fallback fixture` | `待补` | `待补` |
| `openai/codex` | `待补` | `文件` | `codex-rs/core/src/context_manager/history.rs` | `history view builder / Slice 1-2` | `待补` | `待补` | `待补` | `history reducer golden` | `待补` | `待补` |
| `openai/codex` | `待补` | `文件` | `codex-rs/core/templates/compact/prompt.md` | `HRMS compaction prompt / Slice 2` | `待补` | `待补` | `待补` | `prompt template snapshot` | `待补` | `待补` |
| `openai/codex` | `待补` | `文件` | `codex-rs/core/templates/compact/summary_prefix.md` | `summary prefix / Slice 2` | `待补` | `待补` | `待补` | `summary prefix fixture` | `待补` | `待补` |
| `openai/codex` | `待补` | `测试样例` | `codex-rs/core/src/compact_tests.rs`、`codex-rs/core/tests/suite/compact*.rs` | `compaction 回归测试 / Slice 6` | `待补` | `待补` | `待补` | `golden/snapshot suite` | `待补` | `待补` |

填写规则：

- `采用状态` 只允许填写 `直接复用`、`重构复用`、`只借鉴语义`、`明确不引入`。
- 若某机制不能直接复用，必须把原因写成仓库约束，例如 `append-only 审计`、`租户/RLS`、`禁止 shell/file/git 默认能力`、`当前模型治理边界`。
- `必备验证` 至少锁住一项上游形状：history replacement、summary prefix、compact prompt、telemetry 字段、remote fallback 行为。

## 4B. PR-437A 首轮最小冻结

首轮固定 `openai/codex` 参考 commit SHA：

- `ef071cf816950dc416b2a975e7ed023eea639026`

`PR-437A` 只冻结首轮 compaction 开工需要的最小对象：

| 上游项目 | 上游 commit SHA | 上游制品类型 | 上游路径或对象名 | CubeBox 对应对象/切片 | 采用状态 | 不可直接复用原因 | 原因类型 | 必备验证 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `openai/codex` | `ef071cf816950dc416b2a975e7ed023eea639026` | `文件` | `codex-rs/core/src/context_manager/history.rs` | `history view builder / event 对齐` | `重构复用` | 需对齐本仓 append-only 审计与 `DEV-PLAN-437A` 共享 event envelope | `仓库约束` | `history reducer golden` |
| `openai/codex` | `ef071cf816950dc416b2a975e7ed023eea639026` | `文件` | `codex-rs/core/src/compact.rs` | `local compaction planner / runner` | `重构复用` | 原始消息不得被覆盖，且需保留当前权威上下文重注入 | `仓库约束` | `prompt shape golden` |
| `openai/codex` | `ef071cf816950dc416b2a975e7ed023eea639026` | `文件` | `codex-rs/core/templates/compact/prompt.md` | `HRMS compaction prompt` | `重构复用` | 模板必须领域化，不直接使用 coding-agent handoff 文本 | `协议不匹配` | `prompt template snapshot` |
| `openai/codex` | `ef071cf816950dc416b2a975e7ed023eea639026` | `文件` | `codex-rs/core/templates/compact/summary_prefix.md` | `summary prefix / compact item 识别` | `重构复用` | 需与本仓 message item、summary item 命名一致 | `协议不匹配` | `summary prefix fixture` |

## 5. CubeBox 目标架构

### 5.1 分层

- `domain`：定义 conversation、message item、summary item、compaction event、prompt view 的纯模型。
- `services/context`：重构 Codex 的 history、token estimate、compact trigger、replacement view builder。
- `services/gateway`：执行当前主模型或当前 route 对应模型调用，负责 SSE/Responses/Chat adapter。
- `infrastructure/persistence`：append-only 原始消息、summary、compaction event 落库。
- `presentation`：暴露 `/compact` 执行能力、自动压缩提示、上下文用量展示和历史恢复；UI 入口壳层由 `431` 消费。

`Phase A` 起，`MessageItem`、`SummaryItem`、`CompactionEvent` 的最小命名与 envelope 对齐 `DEV-PLAN-437A`，避免 compaction 路径单独长出第二套事件模型。

### 5.2 数据原则

- 原始消息永不因压缩被覆盖。
- 压缩摘要必须记录 source message range、source digest、summary prompt version、summary model、token before/after。
- 活跃 Prompt 只读取 prompt view，不直接拼完整历史。
- 权限、租户、模型配置、当前页面、业务对象必须每轮由 canonical context builder 重新生成。

### 5.3 Prompt view 结构

CubeBox 的 prompt view 应对齐 Codex replacement history 思路，但加入本仓审计与业务上下文：

1. 当前系统基线和安全约束。
2. 当前租户、用户、权限、页面、业务对象的 canonical context。
3. 已存在 compaction summary。
4. 最近用户消息和最近助手回复，受 token 上限保护。
5. 当前工具/context provider 输出摘要。
6. 当前用户输入。

## 6. 实施切片

### Slice 0：Codex 源码与许可证评估

- [ ] 固定参考 commit SHA，不使用浮动 `main` 作为实现依据。
- [ ] 确认 Apache-2.0 许可证、NOTICE、第三方依赖和代码复制要求。
- [ ] 盘点 `compact.rs`、`compact_remote.rs`、`context_manager/history.rs` 的依赖闭包。
- [ ] 输出“可直接借鉴 / 可小段移植 / 必须重写 / 不引入”四类清单。
- [ ] 按本计划 `4A` 模板补齐文件级/模板级/测试样例级上游映射表，并为每个对象冻结采用状态与不可复用原因。
- [ ] `PR-437A` 先以 `4B` 的最小冻结集满足 Phase A companion contract 与首轮 compaction 开工条件。

### Slice 1：概念映射与最小接口

- [ ] 定义 CubeBox `MessageItem`、`PromptItem`、`SummaryItem`、`CompactionEvent`。
- [ ] 定义 `HistoryViewBuilder`、`TokenEstimator`、`CompactionPlanner`、`CompactionRunner`、`CanonicalContextBuilder` 接口。
- [ ] 将 Codex `ResponseItem` / `TurnItem` / `ContextManager` 概念映射为 CubeBox 类型。
- [ ] 明确哪些类型必须保持纯函数，方便测试。

### Slice 2：本地 compaction 内核重构

- [ ] 重构 Codex `build_compacted_history_with_limit` 思路，保留最近用户消息和摘要。
- [ ] 重构 `collect_user_messages`，过滤 CubeBox 系统上下文、摘要消息和非用户输入。
- [ ] 重构 `summary_prefix` 机制，避免摘要被再次当普通用户消息收集。
- [ ] 重构 context window exceeded 时的 oldest item trim，但不能破坏消息 pair 和审计引用。
- [ ] 实现 `replace active prompt view`，不替换数据库原始消息。

### Slice 3：canonical context reinjection

- [ ] 重构 Codex `insert_initial_context_before_last_real_user_or_summary` 规则。
- [ ] 实现 CubeBox canonical context builder：租户、权限、页面、业务对象、语言、可用模型、工具白名单。
- [ ] 压缩后丢弃旧 developer/system 权限上下文，重新注入当前权威上下文。
- [ ] 增加权限变化、租户切换、模型切换的测试。

### Slice 4：触发器与运行时接入

- [ ] 实现 pre-turn auto compact。
- [ ] 实现 manual compact task。
- [ ] 实现 post-turn async precompact 可选任务。
- [ ] P1 再实现 mid-turn compact 与 model downshift compact。
- [ ] 将 compaction failure 映射为明确错误，不静默裁剪历史。
- [ ] 向 `431` 暴露稳定的 manual compact 触发 contract 与状态提示字段，避免 UI 层自定义第二套 compact 语义。

### Slice 5：远端 compaction 能力

- [ ] 首期固定使用当前主模型或当前 route 对应模型运行本地 prompt compaction，不引入独立 summary model。
- [ ] 预留 provider `supports_remote_compaction` 能力位。
- [ ] 若 provider 支持远端 compaction，必须经过输出过滤和 canonical context reinjection。
- [ ] remote compaction 失败必须回退到本地 compaction 或 fail-closed，不允许继续发送超限 Prompt。

### Slice 6：测试与快照

- [ ] 移植 Codex compaction 单测思路：文本拼接、摘要过滤、最近用户消息保留、超长用户消息截断。
- [ ] 新增 CubeBox 权限/租户上下文重新注入测试。
- [ ] 新增 prompt shape snapshot，验证压缩前后 Prompt 顺序稳定。
- [ ] 新增摘要失败、模型下切、上下文超限、工具输出压缩测试。
- [ ] 新增跨租户隔离测试，确认摘要不能读取其他租户消息。

当前备注（`2026-04-22`）：

- 已落地最小 compaction 纯函数与 prompt view builder，覆盖摘要过滤、最近消息保留、canonical context reinjection 与 `turn.context_compacted` envelope。
- 已接入 manual compact API、pre-turn auto compact、前端 `/compact` 入口与 restore/reconstruction fixture。
- 已补一轮实现级收口：
  - no-op compaction 不再伪造 `turn.context_compacted` 事件，也不再在 UI / restore 链路中生成空摘要项。
  - store 侧 `CompactConversation` 已改为单事务内完成会话锁定、事件读取、是否压缩判定与条件性落库，避免 pre-turn auto compact / manual compact 并发时因 `sequence` 竞争把正常请求打成 500。
- 尚未完成 provider 级真实 tokenizer 校准、远端 compaction、独立 summary model，以及更完整的 store 级跨租户隔离测试。

### Slice 7：430 回填与封板

- [ ] 更新 `DEV-PLAN-430`，将 Slice 4 的上下文压缩实现引用到本计划。
- [ ] readiness 记录 Codex 参考 commit、复用清单、裁剪矩阵、prompt shape golden、summary prefix fixture、测试证据。
- [ ] 执行文档、Go、前端、routing、authz、preflight 验证。

## 7. 关键设计决策

### 7.1 原始历史不替换

Codex 会替换活跃 history。CubeBox 只能替换 prompt view，不能替换数据库中的原始消息。原因：

- 本仓需要审计和可追溯。
- 租户/RLS 需要明确消息来源。
- 用户可能需要恢复压缩前全文。
- 摘要模型可能出错，不能让摘要成为唯一事实源。

### 7.2 当前权威上下文必须重注入

Codex 会过滤 stale developer messages 并重新注入 initial context。CubeBox 必须强制采纳该原则：

- 权限变化不能靠旧摘要延续。
- 租户切换必须失效旧上下文。
- 当前页面和业务对象必须每轮重新读取。
- 模型配置、工具白名单和安全约束必须由服务端生成。

### 7.3 摘要模板要领域化

Codex 的模板面向 coding agent handoff。CubeBox 需要 HRMS 领域模板，必须保留：

- 当前用户目标。
- 已确认业务事实。
- 业务对象引用。
- 日期和生效日。
- 权限/安全约束摘要。
- 错误码和失败原因。
- 待办项与未决问题。
- 不得自动提交业务写入的约束。

### 7.4 token 估算先近似后可替换

Codex 使用近似 token 估算和 byte heuristic。CubeBox 首期可以复用这个成熟取舍，但接口必须可替换：

- OpenAI-compatible 模型可接入 tokenizer。
- 非 OpenAI 模型先使用近似估算和安全垫。
- 每次真实调用后的 usage 必须回写，用于校准阈值。

## 8. 验收标准

- [ ] 已记录 Codex 参考 commit SHA 和许可证评估结果。
- [ ] 已形成 Codex 机制复用矩阵，不再从零设计同类 compaction。
- [ ] CubeBox compaction 内核具备 pre-turn auto compact 和 manual compact。
- [ ] Prompt view 替换不覆盖原始消息。
- [ ] canonical context reinjection 每轮生效。
- [ ] 旧权限/旧租户/旧模型上下文不会从摘要中复活。
- [ ] 摘要消息可被识别并避免重复收集。
- [ ] 最近用户消息保留受 token 上限保护。
- [ ] compaction telemetry 记录 trigger、reason、phase、status、token before/after。
- [ ] 单测和快照覆盖 Codex 移植点与 CubeBox 差异点。
- [ ] PR 与 readiness 中都能把 compaction 代码、模板和测试对应回 `4A` 的具体上游制品。
- [ ] `make check chat-surface-clean` 仍能阻断旧对话栈回流。

## 9. Stopline

- 不得在未完成 Slice 0/1 前实现自研 compaction。
- 不得复制 Codex 代码但省略许可证和来源说明。
- 不得在 `4A` 映射表缺失 `commit SHA`、文件级对象或采用状态时开始 compaction planner、history replacement 或 summary template 实现。
- 不得把 compaction 写成“受 Codex 启发”的新压缩器却没有上游文件、模板和测试样例映射。
- 不得将 Codex CLI/TUI/exec/server 运行时引入本仓。
- 不得把 shell/file/git 写操作能力作为 CubeBox 默认工具。
- 不得直接信任 remote compaction 返回的 developer/system/context 内容。
- 不得让摘要替代原始消息审计。
- 不得在压缩失败后静默丢弃历史。

## 10. 本地必跑与门禁

- 文档变更：`make check doc && make markdownlint`
- 旧栈反回流：`make check chat-surface-clean`
- Go 代码变更：`go fmt ./... && go vet ./... && make check lint && make test`
- Routing/Authz/API 变更：`make check routing && make authz-pack && make authz-test && make authz-lint`
- DB/sqlc 变更：按模块执行 schema/migration/sqlc 闭环，新增表前必须获得用户手工确认
- PR 前：`make preflight`

## 11. 参考链接

- OpenAI Codex 仓库：`https://github.com/openai/codex`
- Codex `compact.rs`：`https://github.com/openai/codex/blob/main/codex-rs/core/src/compact.rs`
- Codex `compact_remote.rs`：`https://github.com/openai/codex/blob/main/codex-rs/core/src/compact_remote.rs`
- Codex `tasks/compact.rs`：`https://github.com/openai/codex/blob/main/codex-rs/core/src/tasks/compact.rs`
- Codex `context_manager/history.rs`：`https://github.com/openai/codex/blob/main/codex-rs/core/src/context_manager/history.rs`
- Codex compaction prompt：`https://github.com/openai/codex/blob/main/codex-rs/core/templates/compact/prompt.md`
- Codex summary prefix：`https://github.com/openai/codex/blob/main/codex-rs/core/templates/compact/summary_prefix.md`
- Codex compaction tests：`https://github.com/openai/codex/blob/main/codex-rs/core/src/compact_tests.rs`
