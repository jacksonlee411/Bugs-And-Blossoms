# DEV-PLAN-430：CubeBox（丘宝）IDE 式对话助手重做架构方案

**状态**: 规划中（2026-04-19 20:13 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T3`
- **范围一句话**：在旧对话栈完成历史归档之后，重新设计一个名为 `CubeBox`、中文名为“丘宝”的一方对话助手模块；首期交付对齐 VS Code Codex 插件观感的右侧悬挂抽屉、可配置外部大模型的 AI 网关，以及具备上下文压缩与会话隔离能力的连续对话内核。
- **关联模块/目录**：`AGENTS.md`、`apps/web`、`internal/server`、`modules/cubebox`（候选新模块路径）、`config`、`migrations`、`scripts/ci`
- **关联计划/标准**：`DEV-PLAN-004M1`、`DEV-PLAN-012`、`DEV-PLAN-015`、`DEV-PLAN-016`、`DEV-PLAN-017`、`DEV-PLAN-019`、`DEV-PLAN-021`、`DEV-PLAN-022`、`DEV-PLAN-300`、`DEV-PLAN-431`、`DEV-PLAN-432`、`DEV-PLAN-433`、`DEV-PLAN-434`、`DEV-PLAN-435`、`DEV-PLAN-437`、`DEV-PLAN-437A`
- **用户入口/触点**：Web 应用右侧悬挂对话入口、`/internal/cubebox` 服务端 API、模型配置页、会话列表、会话详情、流式回复、错误提示、审计记录；不提供 VS Code 插件形态或其他 IDE 客户端。

### 0.1 Simple > Easy 三问

1. **边界**：本计划定义归档旧对话栈之后的新模块架构，不复用旧 `assistant`、旧 `CubeBox` 或 LibreChat 的任何代码、路由、表、错误码、测试或第三方资产；`340-383` 与 `380A-380G` 系列仅保留为历史证据，不再构成当前实现前提、执行分批或完成定义。
2. **不变量**：新实现必须是一方模块、单一路径、无 legacy fallback；API Key 不进入前端明文状态；模型调用必须走服务端网关；对话上下文必须有明确 token budget、压缩策略、会话隔离与持久化边界。
3. **可解释**：reviewer 必须能在 5 分钟内说明右悬挂 UI、AI 网关、会话上下文管理、租户隔离、鉴权、审计和门禁如何协同，且能说明它为什么不是旧对话栈回流。

### 0.2 历史切断关系

- 旧对话栈相关计划已转入历史归档区；它们只保留为历史证据，不再作为现行主线 SSOT。
- 本计划是新的 PoR 候选，不继承 `220-293`、`340-384`、`380-380H`、`391D` 的实现假设、阶段划分、子计划依赖或完成定义。
- 若新方案需要借鉴历史实现，只允许把它视为“可选历史案例”；不得把旧 DTO、旧路由、旧 capability、旧表结构或旧 UI 视为默认沿用前提。
- 实施前必须把 `make check chat-surface-clean` 从“全局关键词阻断旧残留”升级为“允许本计划批准的新模块路径，继续阻断旧路径、旧 API、旧 DB 对象、旧第三方资产”的精确门禁。
- 新 CubeBox 正式运行面冻结为 Web Shell 右侧抽屉与 `/internal/cubebox`；它们代表 `430-435` 新主线，不再被 `chat-surface-clean` 视为旧 `cubebox` runtime 回流。旧 `assistant`、LibreChat、`/app/assistant`、`/internal/assistant`、`/assistant-ui` 与历史 compat/retired 语义仍必须 fail-closed。

## 1. 背景与问题陈述

当前仓库已经完成旧对话栈拆除，具备重新设计智能对话助手的干净基线。新的 CubeBox 需要满足三个产品目标：

1. 在用户界面上形成类似 VS Code Codex 插件的右侧悬挂抽屉体验，点击图标即可拉出或收起，不打断主业务页面。
2. 提供 AI 网关能力，首期只交付一个 OpenAI-compatible provider、active model 选择、API Key、base URL、显式连通性验证与基础健康状态；限额、route alias、default model 与故障切换策略列为非首期，暂缓到后续计划。
3. 特别强化对话连贯性，借鉴 Codex 与 Continue 等工具的上下文收集、会话压缩、滑动窗口、结构化状态和会话恢复做法。

本仓是 HRMS implementation repo，不是 VS Code extension 仓库。因此交付形态冻结为 Web 应用内的一方模块和右侧悬挂抽屉；本计划不提供真正的 VS Code 插件或其他 IDE 客户端，也不为其保留实现范围。

## 2. 研究依据与采用口径

### 2.1 VS Code / IDE 式 UI 参考

- VS Code 官方文档显示，Views 可包含 Tree View、Welcome View 或 Webview View，也可被用户移动到 Secondary Sidebar；Webview 适合承载超出原生 API 能力的自定义 UI。
- VS Code 侧边栏文档也提示，Secondary Sidebar 是辅助位置，扩展默认不能直接把 View 贡献到该位置，用户可拖动 Views 调整布局。
- 因此本仓不承诺“安装即进入 VS Code Secondary Sidebar”的 IDE 插件能力；Web 产品内只复刻其右侧悬挂抽屉交互。VS Code 相关资料在本计划中仅作为交互参考，不构成当前交付范围。
- OpenAI Codex 开源仓库中的 Rust TUI 不适合直接复用为本仓 Web/MUI 组件，但其 app-server protocol、thread/turn 状态机、事件流、history reducer、compact/token UI 通知和交互模式是 Slice 1 的优先复用/重构基线；详细方案见 `DEV-PLAN-431`。

### 2.2 AI 网关参考

- Bifrost 的公开说明强调 Go 实现、高并发、低网关开销、自适应负载均衡和多模型路由；`DEV-PLAN-433` 冻结其为 Slice 2 的主参考，要求尽量复用或重构其代码或功能，避免 CubeBox 从零自研一套平行网关。
- One API 类项目强调 OpenAI-compatible 统一接口和多供应商适配，可作为模型供应商抽象、模型别名和渠道配置参考。
- LiteLLM、Portkey 等生态可作为 provider 覆盖、错误归一化、观测和配额治理能力参考，但本计划不默认引入 Python 网关或 SaaS 作为运行时依赖。
- Codex 在网关层只作为局部复用来源：provider adapter、Responses/OpenAI-compatible bridge、SSE/stream parser 与流式测试样式；详细方案见 `DEV-PLAN-433`。

### 2.3 Codex / Continue 会话管理参考

- OpenAI 关于 Codex agent loop 的公开文章说明，Codex 会构造完整输入、接收 SSE 流，并在上下文接近阈值时 compact 对话，把历史输入替换成更小但能代表此前工作的项目列表。
- OpenAI Codex CLI 已开源，仓库 `openai/codex` 中的 `codex-rs/core/src/compact.rs`、`compact_remote.rs`、`context_manager/history.rs` 与 `templates/compact/**` 是 CubeBox 上下文管理与压缩的优先复用/重构基线；详细复用计划见 `DEV-PLAN-434`。
- OpenAI Agents SDK session 文档提供了 client-side session、history compaction、`sessionInputCallback` 这类可裁剪历史、去重工具结果、突出关键上下文的机制。
- Continue 的 context providers 公开文档说明了“显式上下文注入”这类交互思路，可作为 CubeBox 页面内业务上下文选择的交互参考；其 coding-assistant 专属 provider 范围不纳入本计划。
- 本计划采用“优先重构 Codex 开源上下文管理/压缩机制 + 本仓 append-only 审计适配”的策略，不把无限堆叠消息历史作为连贯性的实现方式。

## 2A. 上游复用审计框架

### 2A.1 统一冻结规则

- 所有上游参考必须固定到具体 `commit SHA`，禁止使用 `main`、`master`、`latest`、release tag 别名或“以当前最新版为准”。
- 所有复用对象必须落到“文件、目录、协议、测试样例、页面信息架构”之一，禁止只写“参考 Codex/Bifrost”。
- 所有复用对象只允许四种状态：`直接复用`、`重构复用`、`只借鉴语义`、`明确不引入`；不得出现“部分参考”“后续再看”“适配后使用”之类模糊状态。
- 任何自研设计都必须写明“为何不能直接复用上游”，且理由必须是本仓约束，例如 `DDD 边界`、`RLS/Authz`、`Go + pgx + PostgreSQL`、`append-only 审计`、`前端单主链`、`密钥治理`、`错误码/i18n 契约`，不得写成个人偏好。
- 在对应切片完成“上游差距评估 + 文件级映射 + 状态冻结”之前，不得开始实现该切片。

### 2A.2 统一上游映射表模板

所有 `430` 子计划必须至少维护一张可审计的上游映射表，字段冻结如下：

| 子计划/切片 | 上游项目 | 上游 commit SHA | 上游制品类型 | 上游路径或对象名 | 本仓对应切片/模块 | 采用状态 | 不可直接复用原因 | 原因类型 | 必备验证 | PR 证据位置 | readiness 证据位置 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `431 / Slice 1` | `openai/codex` | `ef071cf816950dc416b2a975e7ed023eea639026` | `协议/文件/测试样例` | `详见 DEV-PLAN-431 / 4B` | `CubeBox UI 协议与壳层` | `PR-437A 最小冻结已补齐` | `详见 DEV-PLAN-431 / 4B` | `详见 DEV-PLAN-431 / 4B` | `golden/snapshot/E2E` | `待补` | `待补` |
| `433 / Slice 2` | `maximhq/bifrost`、`openai/codex` | `Bifrost: de67db28676a8a80ba1e738ebf8f9318d82d16f7; Codex: ef071cf816950dc416b2a975e7ed023eea639026` | `目录/文件/协议/测试样例` | `详见 DEV-PLAN-433 / 5B` | `CubeBox AI 网关` | `PR-437A 最小冻结已补齐` | `详见 DEV-PLAN-433 / 5B` | `详见 DEV-PLAN-433 / 5B` | `fixture/SSE 对照/集成测试` | `待补` | `待补` |
| `434 / Slice 4` | `openai/codex` | `ef071cf816950dc416b2a975e7ed023eea639026` | `文件/模板/测试样例` | `详见 DEV-PLAN-434 / 4B` | `CubeBox 上下文压缩` | `PR-437D 已具备正式封板条件` | `详见 DEV-PLAN-434 / 4B` | `详见 DEV-PLAN-434 / 4B` | `golden/snapshot/纯函数测试` | `docs/dev-plans/437-cubebox-implementation-roadmap-for-fast-start.md#phase-d上下文压缩最小闭环` | `docs/dev-records/DEV-PLAN-437-READINESS.md#phase-d--pr-437d-当前证据2026-04-22` |
| `435 / Slice 5` | `maximhq/bifrost`、`songquanpeng/one-api`、`openai/codex` | `待补` | `页面信息架构/目录/文件` | `待补` | `CubeBox 模型配置 UI` | `待补` | `待补` | `待补` | `IA snapshot/E2E/Authz` | `待补` | `待补` |

字段说明冻结如下：

- `上游制品类型` 只允许填写 `文件`、`目录`、`协议`、`测试样例`、`页面信息架构`。
- `原因类型` 只允许填写 `仓库约束`、`安全边界`、`依赖不兼容`、`协议不匹配`、`DDD 边界`。
- `必备验证` 必须直接指向可执行制品，例如 golden fixture、snapshot、SSE 对照、集成测试、E2E、Authz 测试；不能写“人工验证”作为唯一证据。

### 2A.3 PR 与 readiness 证据要求

- 每个落实现的 PR 必须附“上游映射表增量”，说明本次代码对应哪一个上游制品；如果没有对应上游，即判定为自研，必须附不可复用理由。
- PR reviewer 只问三件事：
  1. 这段实现对应哪个上游制品。
  2. 如果没有对应，为什么必须自研。
  3. 自研部分是否比上游更小，而不是更大。
- readiness 必须保留以下证据：上游 `commit SHA`、采用矩阵、裁剪矩阵、差异说明、golden/fixture/snapshot/集成测试结果。
- 验收以“是否沿着已冻结的复用路线实现”为主，不以“功能跑通”替代复用审计。

### 2A.4 PR-437A 首轮最小冻结

- `PR-437A` 不要求 `431-435` 一次性补齐所有未来切片的映射表，只要求补齐首轮开工需要的最小冻结集。
- 首轮共享 companion doc 冻结为 `DEV-PLAN-437A`，用于统一 `431` reducer、`432` reconstruction 与 `434` compact event 的最小 canonical contract。
- 首轮最小冻结 owner 如下：
  - `431`：见 `DEV-PLAN-431 / 4B`
  - `433`：见 `DEV-PLAN-433 / 5B`
  - `434`：见 `DEV-PLAN-434 / 4B`

## 3. 目标

1. 新增 CubeBox（丘宝）作为一方对话助手模块，英文模块名为 `cubebox`。
2. 提供 Web 应用内右侧悬挂抽屉入口：默认停靠右侧，点击图标拉出，再次点击收起。
3. 支持流式对话回复，用户能看到渐进式输出、代码块、错误提示和中止按钮。
4. 提供 AI 网关配置能力：provider、base URL、active model、API Key、显式连通性验证与基础健康状态。
5. 网关对前端暴露单一内部 API，不让前端直接持有外部供应商 API Key。
6. 会话持久化支持新建、读取、恢复和归档会话。
7. 上下文管理支持 token budget、保留输出区、滑动窗口、摘要压缩、工具输出压缩、结构化状态对象和最近回合原文保留。
8. 支持显式上下文来源：当前页面、当前业务对象、用户选中内容、最近操作、错误详情。
9. 以租户、用户和会话为隔离边界，遵守 RLS、Casbin、审计和错误码契约。
10. 建立测试、E2E、门禁和 readiness 记录，证明新模块可发现、可操作、可审计。

## 4. 非目标

1. 不恢复旧 `assistant`、旧 `CubeBox` 或 LibreChat 的代码、表、路由、测试、错误码或第三方资产。
2. 不 vendoring LibreChat、Chatbot UI、Open WebUI 或其他完整聊天前端。
3. 不把用户 API Key 存入浏览器 localStorage、sessionStorage、Webview state 或前端配置文件。
4. 不引入 Redis、Ristretto、BigCache 等外部缓存作为默认方案；如需外部缓存，必须按 AGENTS.md 外部依赖准入完成用户审批、文档更新和一致性评审。
5. 不把 AI 网关做成通用治理平台、PDP 或 capability governance 回流点。
6. 不在首期实现真正的 VS Code extension；首期仅实现 Web Shell 内的 IDE 式右侧悬挂体验。
7. CubeBox 可以辅助发起业务写入，但它只是由当前用户调用的普通工具，不是独立授权主体；查询和写入都必须以当前用户身份、当前租户和当前会话上下文执行，写入前征得用户确认即可，不引入过度复杂的代理授权或独立主体设计。

## 5. 产品与交互方案

### 5.1 入口与布局

- 在全局 Web Shell 右侧增加 CubeBox 图标按钮，图标常驻但不抢占主导航。
- 点击图标后，从右侧拉出悬挂抽屉，覆盖或挤压策略由页面宽度决定：
  - 桌面宽屏：右侧固定宽度抽屉，可与主页面并行。
  - 中等宽度：右侧覆盖式抽屉，保留半透明遮罩或边界阴影。
  - 移动端：全屏对话页或底部 sheet，不强行保留右侧悬挂。
- 抽屉内至少包含会话标题、模型选择、消息列表、输入框、上下文 chips、发送/停止按钮、会话菜单。
- 默认主题使用项目 UI 主题色丘比蓝 `#09a7a3`，但整体应继承现有 MUI 设计系统和主题变量。

### 5.2 会话操作

- 提供“新会话”按钮，触发严格会话隔离：清空当前消息窗口、摘要、结构化状态与工具结果缓存。
- 提供历史会话列表，按最近更新时间、页面来源和标题展示。
- 支持恢复旧会话，但恢复后必须以该会话自己的历史、摘要和状态对象为输入，不与当前会话混用。
- 支持手动压缩上下文，作为自动压缩之外的显式操作。

### 5.3 用户可见性

- 新模块必须有导航可发现入口和端到端操作，不允许只做后端 API。
- 首期最小用户闭环：打开右侧抽屉 -> 配置或选择可用模型 -> 新建会话 -> 发送问题 -> 流式收到回复 -> 关闭抽屉 -> 重新打开后会话状态仍可恢复。

## 6. AI 网关架构

### 6.1 网关职责

- 对内暴露统一聊天接口，首期兼容 OpenAI chat/completions 或 Responses 风格的最小子集。
- 对外适配范围冻结为一个 OpenAI-compatible provider 最小闭环；其他供应商不在当前交付范围内。
- 管理 API Key、base URL、active model、启停状态和显式连通性验证/基础健康状态。
- 统一处理 SSE 流式转发、错误映射、审计与最小 lifecycle telemetry。
- quota、route alias、default model、fallback/failover 不进入首期运行时闭环；`DEV-PLAN-433/435` 只能做上游评估与后续预留，不得把这些能力列为首期验收项。

### 6.2 运行时技术口径

- 默认使用 Go + pgx + PostgreSQL 实现一方网关，不默认引入 Python 网关进程或外部缓存服务。
- provider adapter 必须是可插拔接口，避免在业务 handler 中散落供应商分支。
- API Key 必须服务端加密保存，前端只看到 provider alias、模型展示名和健康状态。
- 请求路径必须显式租户注入、显式事务边界和 fail-closed 错误处理。
- 网关主请求链只做必要鉴权、请求映射和 SSE 转发；`request-start` / `usage-intent` / `audit-start` 持久化仍是长期目标，但首轮只要求 canonical event 内的最小 lifecycle telemetry 与 final 语义稳定。`usage_event` 数据面已暂缓，项目当前也未建设 `outbox` 能力，因此 `outbox` 不属于本轮实施范围；该模式仍优先复用/重构 Bifrost/Codex 的 telemetry、stream lifecycle 与测试样式，不为 CubeBox 扩大一套平行自研审计框架。

### 6.3 配置模型

首期需要冻结以下最小配置对象，但新增表和迁移必须在实施前再次获得用户手工确认：

- `model_provider`：供应商编码、展示名、base URL、协议类型、启停状态、健康状态。
- `model_credential`：加密 API Key、密钥版本、创建人、更新时间、最后验证结果。
- `model_selection`：当前启用的 active model、展示名与必要默认参数。
- `model_usage_event`：后续阶段预留对象；如恢复实施，记录请求时间、会话、模型、输入输出 token、错误码、延迟、trace_id 与 start/final 状态。本轮不落地 `usage_event` 数据面，也不承接 `outbox` 重试状态。

### 6.4 外部网关借鉴边界

- 借鉴 Bifrost：Go runtime、低开销请求转发、显式验证与 SSE 直通。
- 借鉴 One API：OpenAI-compatible 统一入口与最小 provider/config 信息架构。
- 借鉴 LiteLLM / Portkey：错误归一化与观测字段组织方式。
- Slice 2 执行口径以 `DEV-PLAN-433` 为准：Bifrost 为主参考，要求尽量复用或重构其代码或功能；Codex 只复用局部 provider adapter / bridge / stream parser；本仓继续保留密钥治理、RLS/Authz、错误码、审计和持久化的主导权，但主导权不等于扩大自研，必须优先复用上游 telemetry/lifecycle/test 形状。
- 不直接复制外部项目数据库模型作为本仓事实源，不绕过本仓 RLS/Authz/路由/错误码门禁。

### 6.5 首期明确暂缓项

- `fallback/failover`：首期只做单 provider fail-closed 与健康验证，不做多 provider 故障切换。
- `quota`：首期只保留 provider/model/latency/error 等最小 lifecycle 字段，不做租户/用户额度治理；正式 token usage 数据面后移。
- `route alias/default model`：首期只有 active model 选择，不提供 alias、route graph 或 default model 管理面。
- 上述能力仍可在 `433/435` 做上游映射和后续预留，但不得作为首期 required gate 或验收条件。

## 7. 会话连贯性与上下文管理

### 7.1 会话数据结构

每个会话至少包含：

- `conversation_id`、`tenant_id`、`principal_id`、标题、状态、创建时间、更新时间。
- 原始消息流：用户消息、助手消息、系统提示、工具调用摘要、错误事件。
- 压缩摘要：按时间段或主题生成的层次化摘要。
- 结构化状态对象：当前页面、业务对象、用户意图、已确认事实、可用工具、模型配置、最近错误。
- 上下文来源索引：当前页面、业务对象、用户选择、附件、显式 `@` 上下文。

### 7.2 Prompt 组装顺序

每轮请求按固定顺序组装：

1. 系统基线指令：安全、租户隔离、CubeBox 仅代表当前用户调用工具、业务写入必须走 One Door、输出格式和错误处理规则。
2. 模块上下文：当前页面、业务对象、用户权限摘要、可用工具摘要。
3. 历史压缩摘要：只包含仍然相关的关键决策、文件/对象、业务事实和未完成事项。
4. 结构化状态对象：确定性 JSON，不由模型自由改写。
5. 工具输出压缩结果：保留必要元数据，不塞入大体积原始输出。
6. 最近 3 到 5 轮原文：保留当前任务的细粒度语义。
7. 当前用户输入和显式上下文。

### 7.3 Token budget 与压缩策略

- 每个模型配置必须声明 `max_input_tokens`、`reserved_output_tokens` 和 `auto_compact_threshold`。
- 有效输入预算 = 模型上下文窗口 - 保留输出区 - 系统/工具固定开销。
- 当预计输入超过阈值时，先压缩最旧且相关性低的消息块，再丢弃可重建的工具原始输出。
- 压缩摘要必须保留业务对象、日期、用户已确认选择、错误码、待办项和显式约束。
- 最近用户请求、最近助手回复、最近工具调用结果不得被压缩到不可追溯状态。
- 支持手动 `/compact` 或 UI 操作触发压缩，并在会话中记录压缩事件；UI 命令入口由 `DEV-PLAN-431` 承接，compaction 语义与执行链由 `DEV-PLAN-434` 承接。

### 7.4 上下文来源

- 隐式上下文：当前页面 route、当前业务对象 ID、当前表单草稿、最近错误、当前用户语言。
- 显式上下文：用户选择的对象、上传的文本片段、粘贴的错误日志、`@CurrentPage`、`@Record` 等。
- 不提供 Git diff、terminal、代码库检索、MCP server 或其他 coding-assistant 风格上下文注入能力。
- 外部 provider 首期允许接收当前用户有权访问的真实 HR 业务对象摘要与表单草稿；本计划不扩散建立独立的数据出境/脱敏专项契约，仍以现有租户隔离、Authz/RLS、审计、错误码和 440 收口期间的“不扩大新治理面”原则约束输入范围。

### 7.5 会话隔离

- 新会话必须清空 active memory、压缩摘要、工具结果缓存和结构化状态。
- 恢复会话只能加载该会话自己的持久化状态。
- 不同租户、不同用户之间不得共享消息、摘要、provider credential 或上下文缓存。
- 如果用户切换租户或权限变化，当前会话必须重新校验可见性，不可继续使用旧权限上下文。

## 8. 安全、鉴权与审计

- 前端不得直接请求外部模型供应商。
- API Key 只允许服务端保存和解密，密钥展示永远只显示掩码。
- 模型配置管理需要独立权限对象；普通用户只能选择已授权模型，不可读取密钥。
- 对话请求必须记录 trace_id、conversation_id、active model、latency、错误码和调用结果摘要；正式 token usage 持久化后移。
- 所有用户可见错误必须走项目错误码与 i18n 文案，不直接透出供应商原始错误。
- Prompt 和工具上下文不得包含不属于当前租户和当前用户权限范围的数据。
- CubeBox 是当前用户调用的普通工具，不是 Casbin/Authz 中被独立授权的 subject；业务查询和写入 API 必须完全按当前用户已有权限、当前租户、当前 session 执行，不允许使用 CubeBox service account、代理主体或权限提升。
- 模型输出不得绕过业务模块提交入口；任何业务写入都必须由 CubeBox 带着当前用户身份回到现有业务模块 One Door、事务、RLS、Authz 和审计链路。
- 写入操作只要求用户在 UI 中显式确认待提交动作、关键字段和影响对象；确认后使用当前用户身份提交。首期不引入 delegated actor、长期代理授权、独立策略主体或多阶段审批动作链。
- 审计中 `actor/principal` 必须仍是当前用户；CubeBox 只能作为 `channel/source/tool`、`conversation_id` 与 `trace_id` 记录，不能成为业务授权主体。

## 9. 数据库与迁移策略

- 新增表前必须先完成对象清单评审，并获得用户手工确认。
- 首期推荐把会话、消息、摘要、模型配置与密钥元数据放在新模块 schema 下，避免污染 iam 或业务模块；`usage_event` 数据面后移。
- 密钥密文与密钥元数据必须分离；密钥明文不得进入日志、审计 payload 或前端返回。
- Goose migration、Atlas schema、sqlc query 必须按本仓现行模块闭环执行。
- sqlc 生成后必须确认没有旧对话栈对象名回流。

## 10. 实施切片

### Slice 0：契约与门禁准备

- [ ] 将本计划评审到 `准备就绪`。
- [ ] 更新 `chat-surface-clean` 为精确反回流门禁：允许新主线 `/internal/cubebox`、`modules/cubebox` 和后续明确批准的新 CubeBox 对象，继续阻断旧 `assistant`、LibreChat、`/app/assistant`、`/internal/assistant`、`/assistant-ui`、旧表名和旧错误码。
- [ ] 新增 readiness 记录入口，登记每个切片的命令、证据和残留命中解释。
- [ ] 冻结 `431`、`433`、`434`、`435` 的上游 `commit SHA`、文件级映射表、采用状态与 stopline；`PR-437A` 只要求补齐首轮会消费的最小冻结集，详见 `DEV-PLAN-437` 与 `DEV-PLAN-437A`。
- [ ] 冻结首期暂缓项：fallback/failover、quota、route alias、default model 只允许进入上游评估与后续预留，不得进入首期验收。
- [ ] 冻结 CubeBox 工具调用权限原则：CubeBox 不是独立授权主体，查询/写入按当前用户权限执行；写入只需用户显式确认，并走现有业务模块 One Door、事务、RLS、Authz 和审计。

### Slice 1：UI 壳与用户可见入口

- [ ] 按 `DEV-PLAN-431` 先完成 Codex app-server protocol、thread/turn 状态机、事件流和 TUI 交互模式复用/重构评估。
- [ ] 在 Web Shell 新增右侧悬挂抽屉与入口图标。
- [ ] 用 React/MUI 实现抽屉开关、响应式布局、主题变量、空状态、会话列表占位和输入框；不得直接移植 Codex Rust TUI 渲染层。
- [ ] 重构 Codex thread history builder 思路，建立 CubeBox 前端 timeline reducer。
- [ ] 建立首期 UI 事件契约：conversation、turn、message delta、context item、compact、error、interrupt、complete；`token usage` 事件后移，不作为首期前置。
- [ ] 增加前端状态持久化，但不保存密钥或敏感上下文。
- [ ] 由 `DEV-PLAN-431` 持有抽屉打开/关闭、active conversation UI 恢复、slash command 入口与第二主链防漂移；会话 lifecycle contract 不在本切片重复裁决。
- [ ] 补组件测试和基础 E2E：打开、关闭、恢复 UI 状态；会话恢复/归档正确性由 `DEV-PLAN-432` 承接。

### Slice 2：AI 网关最小闭环

- [ ] 按 `DEV-PLAN-433` 先完成 Bifrost 资产评估与复用/重构清单冻结，不从零自研平行网关。
- [ ] 以 Bifrost 为主参考，结合 Codex provider adapter / bridge / stream parser，建立 provider adapter 接口与一个 OpenAI-compatible provider；其他供应商不进入首期闭环。
- [ ] 以 Bifrost 为主参考实现服务端模型配置读取、密钥解密、请求映射、SSE 转发与错误映射；fallback 不在当前交付范围。
- [ ] 以 Bifrost 的 health/readiness 思路实现显式连通性验证与基础健康检查。
- [ ] 以 Bifrost/Codex 的 telemetry、stream lifecycle 和测试样式为主参考，先完成 canonical event 内的最小 lifecycle telemetry 与 final 语义收口；`request-start` / `usage-intent` / `audit-start` 持久化为长期目标，`outbox` 不在本轮实施范围。
- [ ] 补 handler、service、adapter 单元测试、流式响应测试和错误路径测试。

### Slice 3：会话持久化

- [ ] 按 `DEV-PLAN-432` 先完成 Codex append-only history、session index、archive/resume、rollout/reconstruction 语义复用/重构评估。
- [ ] 新增 conversation、message、summary 的 schema 和 sqlc；`usage_event` 数据面暂停实施，不作为本轮 Slice 3 前置。
- [ ] 实现新建、读取、列出、恢复、归档会话；生命周期语义优先对齐 Codex thread list/read/resume/archive。
- [ ] 实现消息落库、流式回复完成后的最终状态固化；原始消息必须 append-only，不因压缩被覆盖。
- [ ] 补租户隔离、权限、RLS、并发和错误路径测试。
- [ ] `conversation list/read/resume/archive/rename` 的生命周期 contract、持久化语义与 API 行为由 `DEV-PLAN-432` 持有；`DEV-PLAN-431` 只消费其 UI 入口与展示结果。

### Slice 4：上下文管理与压缩

- [x] 按 `DEV-PLAN-434` 完成 Codex 上下文管理与 compaction 复用/重构评估，不从零自研同类机制。
- [x] 已按首期范围重构 Codex token estimator、auto compact threshold、manual compact、replacement history、summary prefix 与 canonical context reinjection 思路。
- [x] 已将 Codex 活跃 history replacement 改造为 CubeBox prompt view replacement，数据库原始消息继续保持 append-only。
- [x] 已实现 prompt builder 的固定顺序和结构化状态对象。
- [x] 已实现摘要压缩任务，首期固定使用当前 active model 执行 compaction，不引入独立 summary model。
- [x] 已按首期范围实现最近回合原文保留；工具输出压缩的更大范围治理继续后移。
- [x] 已补纯函数测试、压缩边界测试、摘要不丢关键事实测试，以及以 fixture / snapshot 承担 golden 等价物的 prompt shape 快照测试。
- [x] `/compact`、auto compact、manual compact` 的语义、触发器、执行链与验收已由 `DEV-PLAN-434` 持有；`DEV-PLAN-431` 只承接 composer 命令入口与状态提示。
- 当前备注（`2026-04-22`）：
  - `PR-437D` 已具备正式封板条件，最小闭环包括 manual compact、pre-turn auto compact、canonical context reinjection、prompt view replacement、`/compact` UI 入口与压缩后恢复链路。
  - 本轮实现级收口已补齐：no-op compaction 不再伪造 compact event / 空摘要项，compaction 序号推进已收敛为单事务安全，不再因并发 compact 抢占 `sequence` 而阻断正常请求。
  - mid-turn compact、remote compaction、model downshift compact、真实 tokenizer 校准与更大范围工具输出压缩仍按 `434` 计划后移，不阻断当前 `Phase D` 封板。

### Slice 5：模型配置 UI 与管理权限

- [ ] 按 `DEV-PLAN-435` 先完成 Bifrost 管理面资产评估与复用/重构清单冻结，不为 Slice 5 再切换第二套主参考。
- [ ] 以 Bifrost 为主参考新增模型供应商配置页或设置面板，`One API` 仅补充渠道/模型映射的信息架构。
- [ ] 支持新增、验证、启用、停用、轮换 API Key；密钥生命周期、掩码展示、审计和权限矩阵由本仓主导。
- [ ] 支持 active model 选择与基础 provider 配置展示，并与 `DEV-PLAN-433` 的 provider / health / validation 语义对齐；quota、route alias、default model、fallback 不在当前交付范围。
- [ ] 冻结显式 Authz 矩阵：平台 admin、平台 operator、租户 admin、普通用户分别映射到 subject/domain/object/action；外部项目只可复用页面 IA、provider/config 命名和验证交互，不复用其用户/角色模型。
- [ ] 补 Authz、路由、错误提示、i18n 和 E2E。

### Slice 6：封板验证

- [ ] 执行 Go、前端、routing、authz、i18n、doc、markdown、E2E 与 preflight。
- [ ] readiness 记录用户可见闭环、流式回复、会话恢复、上下文压缩、密钥不出前端和旧对话栈无回流证据。
- 当前备注（`2026-04-22`）：
  - `Phase C` 与 `Phase D` 均已在 `DEV-PLAN-437-READINESS` 中回填为“已具备正式封板条件”。
  - `Phase D` 当前已回填的验证命令包括 `go test ./modules/cubebox ./internal/server`、`pnpm -C apps/web exec vitest run src/pages/cubebox/api.test.ts src/pages/cubebox/reducer.test.ts src/pages/cubebox/CubeBoxProvider.test.tsx src/pages/cubebox/CubeBoxPanel.test.tsx src/pages/cubebox/CubeBoxPanel.restore.test.tsx`、`make check routing`、`make authz-test`、`make check doc`、`make check chat-surface-clean`。
  - `make preflight` 保留为发 PR 前统一对齐动作，不作为本轮仅文档封板收口的新增阻断前置。

## 11. 测试与覆盖率

- Go 单元测试覆盖 provider adapter、prompt builder、token budget、summary compaction、error mapping、credential masking，以及最小 lifecycle telemetry/final 语义推进。
- 服务层测试覆盖租户隔离、权限失败、模型不可用、SSE 中断与错误映射。
- 前端测试覆盖抽屉开关、输入、停止生成、会话恢复、配置表单、错误提示。
- E2E 覆盖最小用户闭环：配置模型 -> 打开抽屉 -> 新建会话 -> 流式回复 -> 关闭重开 -> 恢复会话。
- 上游对照测试必须直接消费上游冻结后的协议形状、事件形状、SSE 片段、压缩 prompt shape 或页面 IA，而不是只测本仓自造 DTO。
- 覆盖率缺口按 `DEV-PLAN-300` 分类处理：可构造真实分支补测试，可证明死分支删除，不通过新增补洞式文件绕过。

## 12. 本地必跑与门禁

- Go 代码：`go fmt ./... && go vet ./... && make check lint && make test`
- 前端 UI：`pnpm --dir apps/web check`，涉及生成物时执行 `make generate && make css`
- 多语言：`make check tr`
- Routing：`make check routing`
- Authz：`make authz-pack && make authz-test && make authz-lint`
- sqlc：`make sqlc-generate`，命中 DB 触发器再跑 `make sqlc-verify-schema`
- 文档：`make check doc && make markdownlint`
- 旧栈反回流：`make check chat-surface-clean`
- PR 前：`make preflight`

## 13. Stopline

- 不得在未更新反回流门禁前新增 `modules/cubebox` 活体代码。
- 不得在未完成对应子计划的“上游差距评估 + 文件级映射 + 状态冻结”前开始实现该切片。
- 不得在 PR 中使用“参考了 X，结合本仓情况做了适配”但不附文件级映射和不可复用原因。
- 不得引入 LibreChat 或旧 `assistant` 兼容层。
- 不得把供应商 API Key 暴露给前端。
- 不得在没有用户手工确认的情况下新增数据库表。
- 不得用 Redis 等外部缓存替代 Go 原生 + pgx + PostgreSQL 默认方案。
- 不得把 CubeBox 设计成独立授权主体、service account 或权限提升通道；任何查询/写入都必须按当前用户权限执行，写入必须先获得用户显式确认并走 One Door、事务、RLS、Authz、幂等与审计。
- 不得让压缩摘要成为唯一事实源；原始消息和压缩事件必须可审计。
- 不得以“上下文越多越好”为原则无限追加历史；必须通过预算、压缩和显式上下文选择保持高信噪比。
- 不得把“功能跑通”当作切片验收的唯一标准；若未证明实现仍沿着已冻结的复用路线，则视为未通过。

## 14. 冻结决策

1. **E2E 口径冻结**：required gate 只允许使用本地 deterministic provider、mock SSE 或仓内可控 fake provider；不把真实外部模型调用纳入阻断式 CI。真实模型调用只允许作为手工 smoke、非阻断 nightly 或 readiness 补充证据存在，不得成为 merge 前置条件。
2. **API Key 加密方案冻结**：复用仓库现有服务端密钥体系作为主密钥/KEK，CubeBox 模块内采用 envelope encryption 数据模型落地 `model_credential`。模块侧只保存密文、密钥版本、掩码展示字段、验证结果与轮换审计元数据；密钥明文只允许出现在录入与即时验证路径，不得写入前端状态、日志、审计 payload 或普通查询返回。
3. **模型配置权限边界冻结**：首期由平台管理员负责 provider、credential、active model 与基础健康验证等全局配置；租户管理员只负责在已授权范围内选择当前可用模型，不直接管理供应商密钥，也不持有 quota、route alias、default model 或 fallback 等治理能力。权限矩阵必须显式落为 subject/domain/object/action：平台 admin、平台 operator、租户 admin、普通用户分别冻结可见、验证、启停、active model 选择和密钥轮换能力。后续若要开放租户自持 provider/key 或更复杂模型治理，必须另立计划并重新评审 Authz、RLS、审计与密钥治理边界。
4. **summary model 策略冻结**：本计划不做独立 summary model，不采用“仅规则裁剪”替代语义压缩；compaction 固定使用当前 active model 执行，相关配置、健康检查与管理面不增加第二条 summary model 配置链。
5. **CubeBox 工具授权边界冻结**：CubeBox 只是当前用户主动调用的普通工具，不是独立授权主体；业务查询和业务写入完全继承当前用户权限。写入前只需要用户确认动作和关键字段，确认后以当前用户身份调用现有 API / One Door；首期不设计代理授权、独立主体、多阶段审批或额外 PDP。
6. **VS Code 客户端边界冻结**：本计划不实现真正的 VS Code extension 客户端，也不立 IDE adapter 子计划；当前交付范围只包含 Web Shell 内的一方 CubeBox 主链，VS Code 仅作为交互参考来源，不进入实施、测试、门禁或完成定义。

## 15. 参考链接

- VS Code Views：`https://code.visualstudio.com/api/ux-guidelines/views`
- VS Code Sidebars：`https://code.visualstudio.com/api/ux-guidelines/sidebars`
- VS Code Webviews：`https://code.visualstudio.com/api/ux-guidelines/webviews`
- DEV-PLAN-431：Codex UI 协议、状态机与右悬挂壳层复用/重构方案：`docs/dev-plans/431-codex-ui-protocol-and-shell-reuse-plan.md`
- DEV-PLAN-432：Codex 会话持久化、索引与恢复语义复用/重构方案：`docs/dev-plans/432-codex-session-persistence-reuse-plan.md`
- OpenAI Codex agent loop：`https://openai.com/index/unrolling-the-codex-agent-loop/`
- OpenAI Codex 开源仓库：`https://github.com/openai/codex`
- DEV-PLAN-434：Codex 上下文管理与压缩机制复用/重构方案：`docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md`
- OpenAI Agents SDK Sessions：`https://openai.github.io/openai-agents-js/guides/sessions/`
- Continue Context Providers：`https://docs.continue.dev/customize/custom-providers`
- Bifrost AI Gateway：`https://github.com/maximhq/bifrost`
- One API：`https://github.com/songquanpeng/one-api`
