# DEV-PLAN-430：CubeBox（丘宝）IDE 式对话助手重做架构方案

**状态**: 规划中（2026-04-19 20:13 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T3`
- **范围一句话**：在 `DEV-PLAN-392` 全量移除旧对话栈之后，重新设计一个名为 `CubeBox`、中文名为“丘宝”的一方对话助手模块；首期交付对齐 VS Code Codex 插件观感的右侧悬挂抽屉、可配置外部大模型的 AI 网关，以及具备上下文压缩与会话隔离能力的连续对话内核。
- **关联模块/目录**：`AGENTS.md`、`docs/dev-plans/392-remove-assistant-cubebox-and-librechat-rebuild-plan.md`、`apps/web`、`internal/server`、`modules/cubebox`（候选新模块路径）、`config`、`migrations`、`scripts/ci`
- **关联计划/标准**：`DEV-PLAN-004M1`、`DEV-PLAN-012`、`DEV-PLAN-015`、`DEV-PLAN-016`、`DEV-PLAN-017`、`DEV-PLAN-019`、`DEV-PLAN-021`、`DEV-PLAN-022`、`DEV-PLAN-300`、`DEV-PLAN-392`、`DEV-PLAN-431`、`DEV-PLAN-432`、`DEV-PLAN-433`、`DEV-PLAN-434`、`DEV-PLAN-435`
- **用户入口/触点**：Web 应用右侧悬挂对话入口、模型配置页、会话列表、会话详情、流式回复、错误提示、审计记录；VS Code 插件形态作为后续可选客户端适配器，不在首期实现范围内。

### 0.1 Simple > Easy 三问

1. **边界**：本计划定义 392 之后的新模块架构，不复用旧 `assistant`、旧 `CubeBox` 或 LibreChat 的任何代码、路由、表、错误码、测试或第三方资产。
2. **不变量**：新实现必须是一方模块、单一路径、无 legacy fallback；API Key 不进入前端明文状态；模型调用必须走服务端网关；对话上下文必须有明确 token budget、压缩策略、会话隔离与持久化边界。
3. **可解释**：reviewer 必须能在 5 分钟内说明右悬挂 UI、AI 网关、会话上下文管理、租户隔离、鉴权、审计和门禁如何协同，且能说明它为什么不是旧对话栈回流。

### 0.2 392 后置关系

- `DEV-PLAN-392` 的结论仍然有效：旧对话栈已删除，旧计划只保留为历史证据。
- 本计划是新的 PoR 候选，不继承 `220-293`、`340-384`、`380-380H`、`391D` 的实现假设。
- 实施前必须把 `make check chat-surface-clean` 从“全局关键词阻断旧残留”升级为“允许本计划批准的新模块路径，继续阻断旧路径、旧 API、旧 DB 对象、旧第三方资产”的精确门禁。

## 1. 背景与问题陈述

当前仓库已经完成旧对话栈拆除，具备重新设计智能对话助手的干净基线。新的 CubeBox 需要满足三个产品目标：

1. 在用户界面上形成类似 VS Code Codex 插件的右侧悬挂抽屉体验，点击图标即可拉出或收起，不打断主业务页面。
2. 提供 AI 网关能力，允许租户或管理员配置外部大模型供应商、模型、API Key、限额、超时和故障切换策略。
3. 特别强化对话连贯性，借鉴 Codex 与 Continue 等工具的上下文收集、会话压缩、滑动窗口、结构化状态和会话恢复做法。

本仓是 HRMS implementation repo，不是 VS Code extension 仓库。因此首期默认交付形态是 Web 应用内的一方模块和右侧悬挂抽屉；若后续要做真正的 VS Code 插件，应作为客户端适配器另立子计划，并通过同一后端网关和会话 API 接入。

## 2. 研究依据与采用口径

### 2.1 VS Code / IDE 式 UI 参考

- VS Code 官方文档显示，Views 可包含 Tree View、Welcome View 或 Webview View，也可被用户移动到 Secondary Sidebar；Webview 适合承载超出原生 API 能力的自定义 UI。
- VS Code 侧边栏文档也提示，Secondary Sidebar 是辅助位置，扩展默认不能直接把 View 贡献到该位置，用户可拖动 Views 调整布局。
- 因此本仓首期不承诺“安装即进入 VS Code Secondary Sidebar”的 IDE 插件能力；Web 产品内只复刻其右侧悬挂抽屉交互。如果后续做 VS Code 插件，必须先验证目标 VS Code API 版本是否支持默认右侧布局，否则采用官方支持的 View Container + 用户可移动布局方案。
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
- Continue 的 context providers 公开文档展示了 `@File`、`@Code`、`@Git Diff`、`@Current File`、`@Terminal`、`@Codebase`、`@Problems`、MCP 等显式上下文注入模式。
- 本计划采用“优先重构 Codex 开源上下文管理/压缩机制 + 本仓 append-only 审计适配”的策略，不把无限堆叠消息历史作为连贯性的实现方式。

## 3. 目标

1. 新增 CubeBox（丘宝）作为一方对话助手模块，英文模块名为 `cubebox`。
2. 提供 Web 应用内右侧悬挂抽屉入口：默认停靠右侧，点击图标拉出，再次点击收起。
3. 支持流式对话回复，用户能看到渐进式输出、代码块、错误提示和中止按钮。
4. 提供 AI 网关配置能力：供应商、base URL、模型、API Key、超时、限额、启停、健康检查、故障切换。
5. 网关对前端暴露单一内部 API，不让前端直接持有外部供应商 API Key。
6. 会话持久化支持新建、恢复、重命名、归档和删除会话。
7. 上下文管理支持 token budget、保留输出区、滑动窗口、摘要压缩、工具输出压缩、结构化状态对象和最近回合原文保留。
8. 支持显式上下文来源：当前页面、当前业务对象、用户选中内容、最近操作、错误详情、可选 Git diff / terminal / MCP 客户端上下文。
9. 以租户、用户和会话为隔离边界，遵守 RLS、Casbin、审计和错误码契约。
10. 建立测试、E2E、门禁和 readiness 记录，证明新模块可发现、可操作、可审计。

## 4. 非目标

1. 不恢复旧 `assistant`、旧 `CubeBox` 或 LibreChat 的代码、表、路由、测试、错误码或第三方资产。
2. 不 vendoring LibreChat、Chatbot UI、Open WebUI 或其他完整聊天前端。
3. 不把用户 API Key 存入浏览器 localStorage、sessionStorage、Webview state 或前端配置文件。
4. 不引入 Redis、Ristretto、BigCache 等外部缓存作为默认方案；如需外部缓存，必须按 AGENTS.md 外部依赖准入完成用户审批、文档更新和一致性评审。
5. 不把 AI 网关做成通用治理平台、PDP 或 capability governance 回流点。
6. 不在首期实现真正的 VS Code extension；首期仅实现 Web Shell 内的 IDE 式右侧悬挂体验。
7. 不允许模型自动提交业务写入；首期对业务动作只允许建议、解释和生成草稿，提交仍走现有业务模块 One Door。

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
- 对外适配多个供应商：OpenAI-compatible、Anthropic-compatible、Google-compatible、DeepSeek、豆包、通义千问等供应商作为配置项逐步开放。
- 管理 API Key、base URL、模型名、默认参数、超时、重试、故障切换、启停状态和健康检查。
- 统一处理 SSE 流式转发、错误映射、审计、token 用量记录和配额判断。

### 6.2 运行时技术口径

- 默认使用 Go + pgx + PostgreSQL 实现一方网关，不默认引入 Python 网关进程或外部缓存服务。
- provider adapter 必须是可插拔接口，避免在业务 handler 中散落供应商分支。
- API Key 必须服务端加密保存，前端只看到 provider alias、模型展示名和健康状态。
- 请求路径必须显式租户注入、显式事务边界和 fail-closed 错误处理。
- 网关主请求链只做必要鉴权、限额判断、请求映射和 SSE 转发；用量统计、审计写入和健康指标可在响应完成后异步落库，但必须保证失败可观测。

### 6.3 配置模型

首期需要冻结以下配置对象，但新增表和迁移必须在实施前再次获得用户手工确认：

- `model_provider`：供应商编码、展示名、base URL、协议类型、启停状态、健康状态。
- `model_credential`：加密 API Key、密钥版本、创建人、更新时间、最后验证结果。
- `model_route`：模型别名、上游模型名、默认 provider、fallback provider、超时、最大输入 token、最大输出 token。
- `model_quota`：租户级、用户级或模型级 RPM/TPM/每日预算。
- `model_usage_event`：请求时间、会话、模型、输入输出 token、错误码、延迟、trace_id。

### 6.4 外部网关借鉴边界

- 借鉴 Bifrost：Go runtime、低开销路由、故障切换、自适应 provider 选择、SSE 直通。
- 借鉴 One API：OpenAI-compatible 统一入口、多供应商适配、模型别名与渠道配置。
- 借鉴 LiteLLM / Portkey：provider 覆盖、观测、限额、虚拟 key 和错误归一化。
- Slice 2 执行口径以 `DEV-PLAN-433` 为准：Bifrost 为主参考，要求尽量复用或重构其代码或功能；Codex 只复用局部 provider adapter / bridge / stream parser；本仓继续保留密钥治理、RLS/Authz、错误码、审计和持久化的自研主导权。
- 不直接复制外部项目数据库模型作为本仓事实源，不绕过本仓 RLS/Authz/路由/错误码门禁。

## 7. 会话连贯性与上下文管理

### 7.1 会话数据结构

每个会话至少包含：

- `conversation_id`、`tenant_id`、`principal_id`、标题、状态、创建时间、更新时间。
- 原始消息流：用户消息、助手消息、系统提示、工具调用摘要、错误事件。
- 压缩摘要：按时间段或主题生成的层次化摘要。
- 结构化状态对象：当前页面、业务对象、用户意图、已确认事实、可用工具、模型配置、最近错误。
- 上下文来源索引：当前页面、业务对象、用户选择、附件、显式 `@` 上下文、MCP 工具输出摘要。

### 7.2 Prompt 组装顺序

每轮请求按固定顺序组装：

1. 系统基线指令：安全、租户隔离、禁止自动写入、输出格式和错误处理规则。
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
- 支持手动 `/compact` 或 UI 操作触发压缩，并在会话中记录压缩事件。

### 7.4 上下文来源

- 隐式上下文：当前页面 route、当前业务对象 ID、当前表单草稿、最近错误、当前用户语言。
- 显式上下文：用户选择的对象、上传的文本片段、粘贴的错误日志、`@CurrentPage`、`@Record`、`@Diff`、`@Terminal` 等。
- MCP：首期只定义接口和安全边界，不默认启用外部 MCP server；启用前必须做工具白名单、权限、超时和输出压缩。
- 代码库上下文：如果未来接入代码库检索，只能作为只读 context provider，不允许模型直接执行 shell 或写文件。

### 7.5 会话隔离

- 新会话必须清空 active memory、压缩摘要、工具结果缓存和结构化状态。
- 恢复会话只能加载该会话自己的持久化状态。
- 不同租户、不同用户之间不得共享消息、摘要、provider credential 或上下文缓存。
- 如果用户切换租户或权限变化，当前会话必须重新校验可见性，不可继续使用旧权限上下文。

## 8. 安全、鉴权与审计

- 前端不得直接请求外部模型供应商。
- API Key 只允许服务端保存和解密，密钥展示永远只显示掩码。
- 模型配置管理需要独立权限对象；普通用户只能选择已授权模型，不可读取密钥。
- 对话请求必须记录 trace_id、conversation_id、model alias、latency、token usage、错误码和调用结果摘要。
- 所有用户可见错误必须走项目错误码与 i18n 文案，不直接透出供应商原始错误。
- Prompt 和工具上下文不得包含不属于当前租户和当前用户权限范围的数据。
- 模型输出不得绕过业务模块提交入口；任何业务写入都必须回到现有 One Door、事务、RLS 和审计链路。

## 9. 数据库与迁移策略

- 新增表前必须先完成对象清单评审，并获得用户手工确认。
- 首期推荐把会话、消息、摘要、模型配置、密钥元数据、用量事件放在新模块 schema 下，避免污染 iam 或业务模块。
- 密钥密文与密钥元数据必须分离；密钥明文不得进入日志、审计 payload 或前端返回。
- Goose migration、Atlas schema、sqlc query 必须按本仓现行模块闭环执行。
- sqlc 生成后必须确认没有旧对话栈对象名回流。

## 10. 实施切片

### Slice 0：契约与门禁准备

- [ ] 将本计划评审到 `准备就绪`。
- [ ] 更新 `chat-surface-clean` 为精确反回流门禁：允许本计划批准的新 `CubeBox` 路径和对象，继续阻断旧 `assistant`、LibreChat、旧路由、旧表名和旧错误码。
- [ ] 新增 readiness 记录入口，登记每个切片的命令、证据和残留命中解释。

### Slice 1：UI 壳与用户可见入口

- [ ] 按 `DEV-PLAN-431` 先完成 Codex app-server protocol、thread/turn 状态机、事件流和 TUI 交互模式复用/重构评估。
- [ ] 在 Web Shell 新增右侧悬挂抽屉与入口图标。
- [ ] 用 React/MUI 实现抽屉开关、响应式布局、主题变量、空状态、会话列表占位和输入框；不得直接移植 Codex Rust TUI 渲染层。
- [ ] 重构 Codex thread history builder 思路，建立 CubeBox 前端 timeline reducer。
- [ ] 建立首期 UI 事件契约：conversation、turn、message delta、context item、compact、token usage、error、interrupt、complete。
- [ ] 增加前端状态持久化，但不保存密钥或敏感上下文。
- [ ] 补组件测试和基础 E2E：打开、关闭、恢复 UI 状态。

### Slice 2：AI 网关最小闭环

- [ ] 按 `DEV-PLAN-433` 先完成 Bifrost 资产评估与复用/重构清单冻结，不从零自研平行网关。
- [ ] 以 Bifrost 为主参考，结合 Codex provider adapter / bridge / stream parser，建立 provider adapter 接口与一个 OpenAI-compatible provider。
- [ ] 以 Bifrost 为主参考实现服务端模型配置读取、密钥解密、请求映射、SSE 转发、错误映射与 fallback 骨架。
- [ ] 以 Bifrost 的 health/readiness 思路实现模型健康检查与配置验证。
- [ ] 补 handler、service、adapter 单元测试、流式响应测试和 failover 测试。

### Slice 3：会话持久化

- [ ] 按 `DEV-PLAN-432` 先完成 Codex append-only history、session index、archive/resume、rollout/reconstruction 语义复用/重构评估。
- [ ] 新增 conversation、message、summary、usage event 的 schema 和 sqlc。
- [ ] 实现新建、列出、恢复、归档、删除会话；生命周期语义优先对齐 Codex thread list/read/resume/archive。
- [ ] 实现消息落库、流式回复完成后的最终状态固化；原始消息必须 append-only，不因压缩被覆盖。
- [ ] 补租户隔离、权限、RLS、并发和错误路径测试。

### Slice 4：上下文管理与压缩

- [ ] 按 `DEV-PLAN-434` 先完成 Codex 上下文管理与 compaction 复用/重构评估，不从零自研同类机制。
- [ ] 重构 Codex token estimator、auto compact threshold、manual compact、replacement history、summary prefix 与 canonical context reinjection 思路。
- [ ] 将 Codex 活跃 history replacement 改造为 CubeBox prompt view replacement，数据库原始消息保持 append-only。
- [ ] 实现 prompt builder 的固定顺序和结构化状态对象。
- [ ] 实现摘要压缩任务，首期可使用同一 provider 的小模型或配置的 summary model。
- [ ] 实现工具输出压缩和最近回合原文保留。
- [ ] 补纯函数测试、压缩边界测试、摘要不丢关键事实测试，以及 Codex 移植点的 prompt shape 快照测试。

### Slice 5：模型配置 UI 与管理权限

- [ ] 按 `DEV-PLAN-435` 先完成 Bifrost 管理面资产评估与复用/重构清单冻结，不为 Slice 5 再切换第二套主参考。
- [ ] 以 Bifrost 为主参考新增模型供应商配置页或设置面板，`One API` 仅补充渠道/模型映射的信息架构。
- [ ] 支持新增、验证、启用、停用、轮换 API Key；密钥生命周期、掩码展示、审计和权限矩阵由本仓主导。
- [ ] 支持模型别名、fallback、超时、限额和默认模型配置，并与 `DEV-PLAN-433` 的 provider route / health / capability 语义对齐。
- [ ] 补 Authz、路由、错误提示、i18n 和 E2E。

### Slice 6：可选上下文 provider（暂缓）

> 状态：`暂缓（post-MVP）`。原因：首期先收敛 `431/432/433/434/435` 的 UI 壳、会话持久化、AI 网关、压缩内核与模型配置管理，避免在未冻结主链前扩张 context provider / MCP 范围。

- [ ] 暂缓实现当前页面、当前业务对象、当前错误、用户选择文本之外的扩展 context provider。
- [ ] 暂缓设计完整 `@` 显式上下文选择器，仅保留接口预留和对象边界说明。
- [ ] MCP 仅做白名单与接口预留；默认不启用外部 MCP server，且不进入首期交付范围。
- [ ] 暂缓上下文权限裁剪和输出压缩的扩展 provider 测试，首期只覆盖主链上下文注入能力。

### Slice 7：封板验证

- [ ] 执行 Go、前端、routing、authz、i18n、doc、markdown、E2E 与 preflight。
- [ ] readiness 记录用户可见闭环、流式回复、会话恢复、上下文压缩、密钥不出前端和旧对话栈无回流证据。

## 11. 测试与覆盖率

- Go 单元测试覆盖 provider adapter、prompt builder、token budget、summary compaction、error mapping、quota、credential masking。
- 服务层测试覆盖租户隔离、权限失败、模型不可用、SSE 中断、重试与 fallback。
- 前端测试覆盖抽屉开关、输入、停止生成、会话恢复、配置表单、错误提示。
- E2E 覆盖最小用户闭环：配置模型 -> 打开抽屉 -> 新建会话 -> 流式回复 -> 关闭重开 -> 恢复会话。
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
- 不得引入 LibreChat 或旧 `assistant` 兼容层。
- 不得把供应商 API Key 暴露给前端。
- 不得在没有用户手工确认的情况下新增数据库表。
- 不得用 Redis 等外部缓存替代 Go 原生 + pgx + PostgreSQL 默认方案。
- 不得让模型输出直接执行业务写入。
- 不得让压缩摘要成为唯一事实源；原始消息和压缩事件必须可审计。
- 不得以“上下文越多越好”为原则无限追加历史；必须通过预算、压缩和显式上下文选择保持高信噪比。

## 14. 待裁决问题

1. 首期是否允许使用真实外部模型做 E2E，还是只在 required gate 使用本地 deterministic provider？
2. API Key 加密应复用仓库现有密钥机制，还是新增模块级 envelope encryption？
3. 模型配置是租户管理员可配，还是先由平台管理员全局配置后按租户授权？
4. 摘要压缩首期使用同一主模型、独立 summary model，还是先只做规则裁剪？
5. 是否需要真正的 VS Code extension 客户端；如果需要，应另立 `DEV-PLAN-431` 作为 IDE adapter 子计划。

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
