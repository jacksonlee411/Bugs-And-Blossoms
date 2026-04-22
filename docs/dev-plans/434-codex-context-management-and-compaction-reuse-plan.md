# DEV-PLAN-434：Codex 上下文管理与压缩机制复用/重构方案

**状态**: 已完成（2026-04-22 22:48 CST；按 DEV-PLAN-003 评审意见收敛）

## 0. 适用范围与评审分级

- **评审分级**：`T2`（原 CubeBox 系列 `T3` 口径按 `DEV-PLAN-003/001` 收敛为 T2；命中 API、持久化事件、权限/租户上下文、AI 上下文压缩与用户可见链路）
- **范围一句话**：在 `DEV-PLAN-430` 的 CubeBox（丘宝）重做方案中，优先复用或重构 OpenAI Codex 开源仓库已有的上下文管理、会话历史、token 估算、自动/手动 compaction、canonical context reinjection 与测试模式，最大化避免重复造车。
- **关联模块/目录**：`docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`、`modules/cubebox`（候选新模块路径）、`internal/server`、`apps/web`、`config`、`migrations`、`scripts/ci`
- **外部来源**：`https://github.com/openai/codex/tree/ef071cf816950dc416b2a975e7ed023eea639026`
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

1. **边界**：本计划只处理 CubeBox 的上下文管理与压缩内核复用/重构；`turn.context_compacted` 的物理 envelope 与跨 UI/replay 事件命名以 `DEV-PLAN-437A` 为 SSOT，本计划只持有 compaction 业务语义与生成规则；UI、AI 网关、模型配置、数据库对象清单仍由 `DEV-PLAN-430` 主计划裁决；`431` 只可暴露 `/compact` 的 UI 入口，不持有 manual compact 的执行语义、触发器和验收 contract。
2. **不变量**：优先从 Codex 开源实现抽取成熟机制；禁止在未完成差距评估前另起一套自研 compaction；禁止把 Codex 的旧对话栈、CLI/TUI、shell 执行、文件写入、插件、MCP 运行时整体搬入本仓。
3. **可解释**：reviewer 必须能在 5 分钟内说明哪些 Codex 机制被复用、哪些因 HRMS 审计/租户/RLS 约束必须重构、哪些明确不引入，以及如何验证没有重复造车。

### 0.2 现状研究摘要（T2）

- **现状实现**：`modules/cubebox/compaction.go` 已承接 prompt view builder、summary prefix、recent message 保留、canonical context reinjection 与 `turn.context_compacted` 生成；`modules/cubebox/store.go` 在单事务内完成 manual/pre-turn compaction 的事件读取、会话锁定、条件性落库；`modules/cubebox/gateway.go` 在发送前执行 pre-turn auto compact；`internal/server/cubebox_api.go` 暴露 `/internal/cubebox/conversations/{conversation_id}:compact`；`apps/web/src/pages/cubebox` 消费 compact event 并渲染 `/compact` 入口。
- **现状约束**：原始消息 append-only；prompt view 可替换但不得替代审计事实；租户/权限/模型上下文每轮由服务端 canonical context builder 重新注入；`turn.context_compacted` 的最小 envelope 对齐 `DEV-PLAN-437A`，首期只冻结 `summary_id/source_range/summary_text/source_digest/reason`，token before/after 保持纯函数调试信息，不进入事件契约。
- **最容易出错的位置**：manual compact 与 pre-turn auto compact 并发争抢 sequence；no-op compaction 伪造空摘要；旧 developer/system/权限上下文从摘要复活；UI/replay 与服务端各自定义 compact DTO；超长最近用户消息导致 prompt view 超预算。
- **本次不沿用的“容易做法”**：不 vendoring 整个 Codex；不引入 remote compaction capability/fallback、post-turn async precompact、独立 summary model 或完整 telemetry 数据面；不把 summary 模板扩成重型业务交接表；不让 UI 层自造第二套 compact 语义。

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
  - compaction 调试信息。
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
| token 估算 | `approx_token_count` 与 byte heuristic | 首期复用近似估算思路，后续如有必要再接真实 tokenizer |
| 自动压缩阈值 | `auto_compact_token_limit` | 采纳，首期使用服务端固定阈值，不提前做模型/租户参数化 |
| 手动压缩 | `CompactTask` | 采纳，执行语义由本计划持有，UI 入口映射为 `431` 的 `/compact` 或按钮 |
| pre-turn 压缩 | `run_pre_sampling_compact` | 采纳，发送前同步压缩 |
| mid-turn 压缩 | `run_auto_compact(...BeforeLastUserMessage...)` | P1 采纳；首期可先阻断 follow-up 超限 |
| model downshift | `maybe_run_previous_model_inline_compact` | P1 采纳；模型切换到小窗口时需要 |
| local compaction | `compact.rs` | 采纳 prompt 与 replacement history 思路 |
| remote compaction | `compact_remote.rs` | 后续观察项；首期不设计 capability 位、不写 fallback 规则 |
| summary prompt | `templates/compact/prompt.md` | 采纳最小 handoff 思路，只要求概括当前进展、关键约束、下一步、关键引用 |
| summary prefix | `templates/compact/summary_prefix.md` | 采纳，用于识别摘要消息和防重复摘要 |
| 最近用户消息保留 | `COMPACT_USER_MESSAGE_MAX_TOKENS` | 采纳，首期使用固定策略常量或服务端默认值，不做租户/模型配置化 |
| stale developer 过滤 | `should_keep_compacted_history_item` | 强制采纳，权限/租户/模型配置必须重新注入 |
| canonical context reinjection | `insert_initial_context_before_last_real_user_or_summary` | 强制采纳，注入当前权威业务上下文 |
| telemetry | `CodexCompactionEvent` | 只借鉴最小调试信息思路；首期不扩成独立数据面、审计字段体系或硬验收项，`token_before/token_after` 不进入 canonical event payload |
| 测试模式 | `compact_tests.rs` + suite snapshots | 采纳，建立纯函数单测 + prompt shape 快照 |

## 4A. 上游参考最小冻结

首轮固定 `openai/codex` 参考 commit SHA：

- `ef071cf816950dc416b2a975e7ed023eea639026`

首期只冻结以下最小参考关系，不把对象级证据矩阵、PR 落点或 readiness 落点写成实现前置：

- `codex-rs/core/src/context_manager/history.rs`：重构复用其 history replacement 思路，但继续保持本仓 append-only 原始消息。
- `codex-rs/core/src/compact.rs`：重构复用本地 compaction 主流程与 canonical context reinjection 思路。
- `codex-rs/core/templates/compact/prompt.md`：只借鉴最小 handoff 结构，不把其扩写为重型业务交接模板。
- `codex-rs/core/templates/compact/summary_prefix.md`：重构复用摘要识别前缀，避免摘要被再次当普通用户消息收集。
- `codex-rs/core/src/compact_tests.rs` 与 `codex-rs/core/tests/suite/compact*.rs`：借鉴纯函数测试与 snapshot 思路，不额外建立对象级证据矩阵。

## 5. CubeBox 目标架构

### 5.1 分层

- `domain`：只冻结首期必需的最小结构，如 `PromptItem`、`CompactionResult`、`CanonicalContext`。
- `services/context`：承接 history、token 估算、compaction 纯函数和 prompt view replacement。
- `services/gateway`：执行当前主模型或当前 route 对应模型调用，负责 SSE/Responses/Chat adapter。
- `infrastructure/persistence`：append-only 原始消息和最小 `turn.context_compacted` 事件落库；首期不扩成独立 telemetry 数据面。
- `presentation`：暴露 `/compact` 执行能力、自动压缩提示、上下文用量展示和历史恢复；UI 入口壳层由 `431` 消费。

`Phase A` 起，compaction 路径的最小命名与 envelope 对齐 `DEV-PLAN-437A`，避免上下文压缩路径单独长出第二套事件模型。

### 5.2 数据原则

- 原始消息永不因压缩被覆盖。
- 压缩摘要首期只要求稳定记录 `summary_id`、`source_range`、`source_digest`；`token before/after` 仅作为可选调试信息，`summary_prompt_version` 等字段判定为过度设计，不进入事件契约、测试夹具或验收项。
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

- [x] 固定参考 commit SHA，不使用浮动 `main` 作为实现依据。
- [x] 确认 Apache-2.0 许可证、NOTICE、第三方依赖和代码复制要求：首期仅重构思路与测试模式，不复制上游 Rust 源码；若后续复制代码片段，必须保留 Apache-2.0 许可证来源说明。
- [x] 盘点 `compact.rs`、`compact_remote.rs`、`context_manager/history.rs` 的依赖闭包：本仓只重构本地 compaction、replacement history、summary prefix、canonical reinjection；`compact_remote.rs` 的 provider capability/fallback 链路不进入首期。
- [x] 输出“可直接借鉴 / 可小段移植 / 必须重写 / 不引入”四类清单：prompt/snapshot 思路可直接借鉴；summary prefix 与 recent-user-message token guardrail 可小段重构；Rust runtime、provider、CLI/TUI 必须重写或不引入。
- [x] 记录首期最小参考文件和复用边界，不把对象级证据矩阵或 companion contract 写成实现前置。

### Slice 1：最小结构与纯函数边界

- [x] 冻结首期必需的最小结构，如 `PromptItem`、`CompactionResult`、`CanonicalContext`。
- [x] 明确 history builder、summary 生成、prompt view replacement 哪些逻辑保持纯函数，便于测试。
- [x] 不为了“未来也许可替换”提前抽完整接口墙；出现第二实现需求前，不引入额外 `*Builder/*Planner/*Runner` 抽象。

### Slice 2：本地 compaction 内核重构

- [x] 重构 Codex `build_compacted_history_with_limit` 思路，保留最近用户消息和摘要。
- [x] 重构 `collect_user_messages`，过滤 CubeBox 系统上下文、摘要消息和非用户输入。
- [x] 重构 `summary_prefix` 机制，避免摘要被再次当普通用户消息收集。
- [x] 重构 context window exceeded 时的 oldest item trim，并补齐最近用户消息 token guardrail；只裁剪 prompt view，不破坏消息 pair 和审计引用。
- [x] 实现 `replace active prompt view`，不替换数据库原始消息。

### Slice 3：canonical context reinjection

- [x] 重构 Codex `insert_initial_context_before_last_real_user_or_summary` 规则。
- [x] 实现 CubeBox canonical context builder：租户、权限、页面、业务对象、语言、可用模型。
- [x] 压缩后丢弃旧 developer/system 权限上下文，重新注入当前权威上下文。
- [x] 增加权限变化、租户切换、模型切换的测试；更完整 store 级跨租户隔离测试后移。

### Slice 4：触发器与运行时接入

- [x] 实现 pre-turn auto compact。
- [x] 实现 manual compact task。
- [ ] P1 再实现 mid-turn compact 与 model downshift compact。
- [x] 将 compaction failure 映射为明确错误，不静默裁剪历史。
- [x] 向 `431` 暴露稳定的 manual compact 触发 contract 与状态提示字段，避免 UI 层自定义第二套 compact 语义。

### Slice 5：后续观察项（不属于首期）

- [ ] remote compaction、provider capability 位和相关 fallback 规则后移；当前不进入对象模型、运行时主链或验收范围。
- [ ] 独立 summary model、真实 tokenizer 精准校准和 model downshift compact 继续按 P1 / 后续计划推进。
- [ ] 仅当本地 compaction 已稳定且真实运行证据证明不足时，再单独评估是否引入这些能力。

### Slice 6：测试与快照

- [x] 已移植 Codex compaction 单测思路：文本拼接、摘要过滤、最近用户消息保留、超长用户消息截断。
- [x] 已新增 CubeBox 权限/租户上下文重新注入测试。
- [x] 已新增 prompt shape snapshot，验证压缩前后 Prompt 顺序稳定；首期以纯函数 fixture / snapshot 作为 golden 等价物，不另建独立 golden 文件体系。
- [x] 新增上下文超限、最近用户消息 token guardrail 和最小 `turn.context_compacted` envelope 测试。
- [ ] 跨租户隔离、模型下切、工具输出压缩测试后移到 P1 / 后续切片。

当前备注（`2026-04-22`）：

- 已落地最小 compaction 纯函数与 prompt view builder，覆盖摘要过滤、最近消息保留、canonical context reinjection 与 `turn.context_compacted` envelope。
- 已接入 manual compact API、pre-turn auto compact、前端 `/compact` 入口与 restore/reconstruction fixture。
- 已补一轮实现级收口：
  - no-op compaction 不再伪造 `turn.context_compacted` 事件，也不再在 UI / restore 链路中生成空摘要项。
  - store 侧 `CompactConversation` 已改为单事务内完成会话锁定、事件读取、是否压缩判定与条件性落库，避免 pre-turn auto compact / manual compact 并发时因 `sequence` 竞争把正常请求打成 500。
  - `turn.context_compacted` event payload 已按 `437A` 最小 envelope 收敛，`token_before/token_after` 保持纯函数调试信息，不进入 canonical event payload。
  - 最近用户消息已补固定 token guardrail，超长消息只在 prompt view 中截断，原始事件仍可恢复。
- 当前范围已明确收敛：不再把对象级上游映射矩阵、完整 telemetry、remote compaction、post-turn async precompact、独立 summary model 作为首期封板前提。
- provider 级真实 tokenizer 校准、更完整的 store 级跨租户隔离测试，以及远端/下切类 compaction 继续后移。
- 后续补充发现（`2026-04-23`）：
  - Codex 的最近用户消息限制采用“单一 token 语义 + 超限即按 token 截断”，不做字符级二次放行。
  - 本仓边界值缺口与当前轮重复输入去重问题转交 `DEV-PLAN-438A/438` 收口，不在本计划内继续膨胀 owner。

### Slice 7：430 回填与封板

- [x] 已更新 `DEV-PLAN-430`，将 Slice 4 的上下文压缩实现引用到本计划。
- [x] readiness 可继续保留为过程证据，但不再作为 `434` 首期实现前置。
- [x] 已执行并记录文档、Go、前端、routing、authz 验证；`preflight` 保留为发 PR 前统一对齐动作，不阻断本轮文档封板收口。

封板裁决（`2026-04-22`）：

- `PR-437D` / `Phase D` 已具备正式封板条件。
- 首期 `prompt shape snapshot` 以纯函数 fixture / snapshot 承担 golden 等价物角色，后续若需要独立 golden 文件体系，可作为非阻断增强项继续演进。
- mid-turn compact、remote compaction、model downshift compact、真实 tokenizer 校准与更完整跨租户 store 测试继续按 P1 / 后续切片推进，不构成当前封板缺口。

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

### 7.3 摘要模板保持最小 handoff 结构

Codex 的 compact prompt 只要求摘要概括“当前进展、关键约束、下一步、关键引用”。CubeBox 首期沿用这一最小 handoff 思路：

- 当前进展：到目前为止已确认的关键信息。
- 关键约束：当前仍有效的权限、租户、页面或业务限制。
- 下一步：后续需要继续回答或完成的事项。
- 关键引用：必要时保留最小业务对象或上下文引用，避免摘要脱锚。

首期不把 summary 扩写为重型业务交接模板；日期、生效日、错误码、安全约束等信息只有在确实构成当前关键约束时才进入摘要，而不是固定栏目。

### 7.4 token 估算先近似后可替换

Codex 使用近似 token 估算和 byte heuristic。CubeBox 首期可以复用这个成熟取舍，但接口必须可替换：

- OpenAI-compatible 模型可接入 tokenizer。
- 非 OpenAI 模型先使用近似估算和安全垫。
- 真实调用后的正式 usage 持久化已后移；阈值校准先依赖近似估算与 fixture，不把完整 telemetry 设计写成首期前提。

## 8. 验收标准

- [ ] 已记录 Codex 参考 commit SHA 和许可证评估结果。
- [ ] 已明确 Codex 最小参考文件与复用边界，不再从零设计同类 compaction。
- [ ] CubeBox compaction 内核具备 pre-turn auto compact 和 manual compact。
- [ ] Prompt view 替换不覆盖原始消息。
- [ ] canonical context reinjection 每轮生效。
- [ ] 旧权限/旧租户/旧模型上下文不会从摘要中复活。
- [ ] 摘要消息可被识别并避免重复收集。
- [ ] 最近用户消息保留受 token 上限保护。
- [ ] `turn.context_compacted` 最小 envelope 稳定、可恢复、可重放；物理 envelope 与事件命名以 `DEV-PLAN-437A` 为 SSOT，本计划只约束 compaction 业务语义与生成规则。
- [ ] 单测和快照覆盖 compaction 纯函数、summary prefix、canonical context reinjection、最近用户消息 token guardrail 与最小 envelope。
- [ ] `make check chat-surface-clean` 仍能阻断旧对话栈回流。

## 9. Stopline

- 不得在未完成 Slice 0/1 前实现自研 compaction。
- 不得复制 Codex 代码但省略许可证和来源说明。
- 不得把 compaction 写成“受 Codex 启发”的新压缩器，却不说明参考了哪些上游文件或为何必须重构。
- 不得将 Codex CLI/TUI/exec/server 运行时引入本仓。
- 不得把 shell/file/git 写操作能力作为 CubeBox 默认工具。
- 首期不得预先引入 remote compaction capability 位、fallback 规则或第二套 summary model 配置链。
- 不得把 summary 扩写成重型业务交接模板并写成硬契约。
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

- OpenAI Codex 仓库（固定参考 commit）：`https://github.com/openai/codex/tree/ef071cf816950dc416b2a975e7ed023eea639026`
- Codex `compact.rs`：`https://github.com/openai/codex/blob/ef071cf816950dc416b2a975e7ed023eea639026/codex-rs/core/src/compact.rs`
- Codex `compact_remote.rs`：`https://github.com/openai/codex/blob/ef071cf816950dc416b2a975e7ed023eea639026/codex-rs/core/src/compact_remote.rs`
- Codex `tasks/compact.rs`：`https://github.com/openai/codex/blob/ef071cf816950dc416b2a975e7ed023eea639026/codex-rs/core/src/tasks/compact.rs`
- Codex `context_manager/history.rs`：`https://github.com/openai/codex/blob/ef071cf816950dc416b2a975e7ed023eea639026/codex-rs/core/src/context_manager/history.rs`
- Codex compaction prompt：`https://github.com/openai/codex/blob/ef071cf816950dc416b2a975e7ed023eea639026/codex-rs/core/templates/compact/prompt.md`
- Codex summary prefix：`https://github.com/openai/codex/blob/ef071cf816950dc416b2a975e7ed023eea639026/codex-rs/core/templates/compact/summary_prefix.md`
- Codex compaction tests：`https://github.com/openai/codex/blob/ef071cf816950dc416b2a975e7ed023eea639026/codex-rs/core/src/compact_tests.rs`
