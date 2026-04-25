# DEV-PLAN-468：CubeBox 同会话连续追问与模型自主性收敛方案

**状态**: 规划中（2026-04-25 15:10 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：承接 `DEV-PLAN-467` 的调查结论，专门处理 `CubeBox` 在同一会话内基本连续追问时的记忆继承、指代解析、澄清表达与查询结果叙述问题，并扩大调查当前本地代码对大模型语义发挥的过度干预，以及哪些本可由模型承担的理解、澄清、候选选择、默认值选择、结果叙述与上下文收敛事项被提前编码到了 Go/TS 规则里。
- **关联模块/目录**：`docs/dev-plans/438-cubebox-conversational-continuity-investigation-and-remediation-plan.md`、`docs/dev-plans/438a-cubebox-provider-message-role-normalization-and-codex-summary-alignment-plan.md`、`docs/dev-plans/460-cubebox-digital-assistant-positioning-and-execution-contract.md`、`docs/dev-plans/461-cubebox-query-scenarios-minimal-contract.md`、`docs/dev-plans/462-cubebox-codex-compaction-adoption-value-and-unified-convergence-plan.md`、`docs/dev-plans/464-cubebox-query-architecture-convergence-plan.md`、`docs/dev-plans/467-cubebox-query-conversational-continuity-and-memory-loss-investigation-plan.md`、`docs/dev-plans/469-cubebox-model-driven-compaction-critical-redesign-plan.md`、`internal/server/cubebox_query_flow.go`、`modules/cubebox/*`、`modules/orgunit/presentation/cubebox/*`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-003`、`DEV-PLAN-004M1`、`DEV-PLAN-012`、`DEV-PLAN-300`、`DEV-PLAN-301`、`DEV-PLAN-304`、`DEV-PLAN-438`、`DEV-PLAN-460`、`DEV-PLAN-461`、`DEV-PLAN-462`、`DEV-PLAN-464`、`DEV-PLAN-467`、`DEV-PLAN-469`
- **用户入口/触点**：主应用壳层右侧 `CubeBox` 抽屉、`/internal/cubebox/turns:stream`、查询 planner、只读执行器、查询 narrator、canonical events
- **证据记录 SSOT**：后续真实页面复验、网络请求、对话样本、命令执行结果统一登记到 `docs/dev-records/DEV-PLAN-468-READINESS.md`；本文件只冻结方案、边界、实施切片与验收口径。

### 0.1 Simple > Easy 三问

1. **边界**：本计划只处理同一 `conversation_id` 内的短程连续追问，不建设长期记忆、向量库、用户画像、跨会话记忆、第二套查询 endpoint 或第二套授权/事实源。
2. **不变量**：查询链必须继续继承现有租户、当前用户、当前 session、canonical events、知识包、只读执行注册表、`ReadPlan` schema 校验与现有 API 执行边界；不得为了“更聪明”而放松权限、租户、白名单、参数校验或只读约束。
3. **可解释**：reviewer 必须能在 5 分钟内说明三件事：模型负责理解连续追问和自然表达；本地代码负责提供正确上下文和执行护栏；为什么当前过量的本地 stopline、regex 与模板化约束会削弱同会话连贯性。

### 0.2 本计划冻结的总判断

`CubeBox` 当前的问题不是“完全没有记忆”，而是**记忆输入太窄、上下文分配不均、表达层被本地代码过度管控**。

当前实现已经有若干正确方向：

- 查询链会从会话事件中提取最近一个 `turn.query_entity.confirmed`，并在 planner 输入中注入 `RecentEntity`。
- 查询成功后会写入结构化实体锚点，避免完全依赖自然语言回答反推实体。
- `NO_QUERY` 后已有统一 stopline，避免继续掉回普通对话链后虚构“没有查询接口/权限”。
- 执行侧仍走 `api_key -> executor` 注册表、参数校验与只读 API，没有让模型直接访问数据库。

但这些还不足以支撑用户最常见的连续追问：

- “查一下 `100000` 在今天的详情”之后问“查该组织的下级组织”
- “刚才那个组织呢”
- “最开始说的那个组织”
- “不用再问了，就是刚才那个”
- “那它的负责人/上级/下级是什么”

核心收敛方向是：**代码从“替模型决定怎么说、怎么澄清、怎么压制表达”退回到“提供高质量上下文 + 强制安全边界”；模型重新承担指代解析、澄清、默认值选择与自然叙述。**

### 0.3 本轮扩大调查口径（2026-04-25 补充）

本计划不应被理解为“继续限制大模型，防止模型发挥”。恰恰相反，`468` 的主轴应从“用本地规则压住模型”调整为：**安全护栏不变，语义工作重新交给模型**。

必须保留的硬护栏包括：

- 当前用户、当前租户、当前 session、Authz/RLS 与审计归属。
- 只读执行注册表、`api_key` 白名单、现有读 API、参数类型与 schema 校验。
- 不让模型直查数据库、直接写库、调用未登记接口、绕过业务 API 或越权执行。
- 不把会话压缩摘要、知识包或模型输出当作授权来源。
- 不向用户泄露原始执行 JSON、内部执行计划、密钥、provider 配置或未公开字段路径。

应扩大回交给模型的语义职责包括：

- 判断用户是否仍处于受支持业务查询域。
- 基于有限会话上下文解析“该组织 / 那个 / 最开始那个 / 它”。
- 在低风险只读场景选择默认值，并在缺参或歧义时自然澄清。
- 面对候选列表时让用户选择，而不是由本地代码生成固定澄清话术。
- 基于执行结果做自然叙述、减少重复、承接上一轮上下文和用户语气。
- 对较长历史和较长结果做语义级摘要或收敛时，优先让模型承担表达与取舍，本地只提供预算、事实边界和当前权威上下文。

## 1. 背景与问题定义

`DEV-PLAN-438` 已处理 provider 主链没有消费 prompt view 的会话连续性问题；`DEV-PLAN-467` 则进一步冻结了查询链中的连续追问失效问题。本计划是 `467` 之后的实施方案，不重复调查“是否失忆”，而是明确怎么修。

对标 `Codex` 类成熟对话/工具运行时，本计划采用的经验不是引入更重平台，而是三条朴素原则：

1. 模型输入必须包含足够的最近上下文，而不是只给当前句子。
2. 工具执行边界必须严格、可验证、fail-closed。
3. 本地代码不应把自然语言表达风格、澄清问题和查询解释写成大批硬编码模板或禁词规则。

当前 `CubeBox` 在第 2 点上基本方向正确，但第 1 点和第 3 点仍不足。

## 2. 当前实现诊断

### 2.1 planner 只拿到“当前输入 + 知识包 + 最近一个实体”

当前 `cubeboxReadPlanProductionInput` 只有 `Prompt`、`KnowledgePacks` 和 `RecentEntity` 等字段，见 `internal/server/cubebox_query_flow.go:29`。`QueryContext` 也只暴露 `RecentConfirmedEntity`，见 `modules/cubebox/query_entity.go:17`。

这能覆盖最简单的“上一轮实体”继承，但覆盖不了：

- 多轮之后引用“最开始那个”
- 上一轮是澄清问题而不是成功查询
- 上一轮展示了多个候选，用户用自然语言选择其中一个
- 当前轮需要同时继承实体、日期、查询意图和上一次回答焦点
- 用户追问“它/该组织/那个/刚才说的”时需要看最近 3 到 5 轮语境

### 2.2 planner 不复用 provider prompt view，只消费查询上下文锚点

当前 `TryHandle(...)` 的顺序是先 `queryContext(...)`，再 `ProduceReadPlan(...)`，只有 planner 确认 handled 后才进入 `prepareQueryTurn(...)` 与 `PrepareTurnStream(...)`，见 `internal/server/cubebox_query_flow.go:181`、`internal/server/cubebox_query_flow.go:807`。

这意味着普通 provider prompt view 的连续性能力不会自动进入查询 planner。该设计本身可以接受，因为 planner 不应吃完整无界上下文；但必须补一个**轻量、结构化、有限窗口**的查询对话上下文，而不能只给一个最近实体。

### 2.3 `NO_QUERY` 是单一哨兵，语义过粗

当前 planner prompt 要求：可查询则输出 `ReadPlan JSON`，否则只输出 `NO_QUERY`，见 `internal/server/cubebox_query_flow.go:363`。`NO_QUERY` 被解析为 `Handled=false`，随后服务端写固定 stopline，见 `internal/server/cubebox_query_flow.go:342`、`internal/server/cubebox_query_flow.go:743`。

统一 stopline 可以避免模型胡说系统没有查询能力，但它也把三类不同情况压成一类：

- 真实不支持的领域
- 支持领域但缺少业务参数，需要澄清
- 支持领域且有上下文，但 planner 没拿到足够上下文导致误判

这种压平会削弱模型本应承担的自然澄清能力。

### 2.4 narrator 看不到连续追问上下文

当前 narrator 输入 envelope 只有 `user_prompt`、`plan`、`results`，见 `internal/server/cubebox_query_flow.go:96` 和 `internal/server/cubebox_query_flow.go:458`。

这让 narrator 可以基于执行结果回答当前问题，但看不到：

- 当前轮“该组织”实际解析自哪个上一轮实体
- 用户是否刚刚因为重复追问而处于挫败状态
- 上一轮助手已经说过哪些事实，本轮应避免重复铺陈
- 本轮是澄清后的继续执行，还是新的查询

因此 narrator 难以做到真正像同一会话内的连续回答，只能做一次性查询结果摘要。

### 2.5 表达层 regex 过强，容易把模型变成模板填空器

当前 `queryNarrationForbiddenPatterns` 已收缩为最后一道泄露拦截，见 `internal/server/cubebox_query_flow.go:105`；`validateQueryNarrationText(...)` 只应因原始 JSON、Markdown 代码块、内部执行字段、内部计划字段或字段路径泄露而拒绝模型回答，见 `internal/server/cubebox_query_flow.go:541`。

需要保留的限制是：不得泄露原始 JSON、内部执行计划、`api_key`、`payload/results` 等实现细节。

已经从失败条件中移除的是：禁止普通栏目词、禁止某些中文短语、禁止自然语言里的所有键值感表达。过强 regex 会造成两类副作用：

- 好回答被误杀，导致用户看到“查询结果叙述未通过输出约束校验”
- 模型被迫围绕规避禁词写作，而不是围绕用户问题组织答案

### 2.6 `ReadPlan` 澄清形态偏窄

当前 `ReadPlan` 在 `missing_params` 存在时要求 `clarifying_question` 非空，且 `steps` 必须为空，见 `modules/cubebox/read_plan.go:61`。

该规则能保持执行边界简单，但它也让“可部分解析、只缺一个确认”的情形无法表达候选、上下文解析依据或预期下一步。后续可继续保持 `steps` 为空，但需要给 planner/narrator 一个更清楚的“澄清态”协议，而不是把所有非执行结果都混到 `NO_QUERY` 或固定 stopline。

### 2.7 扩大调查：当前仍存在的模型限制面

本轮扩大调查后，当前限制大模型发挥的实现点不止连续追问，还包括以下几类。

#### A. planner 被压成“JSON/NO_QUERY 二选一”

`buildPlannerMessages(...)` 要求模型只有两种职责：输出严格合法 `ReadPlan JSON`，或输出 `NO_QUERY`，见 `internal/server/cubebox_query_flow.go:363`。这能保证执行入口简单，但会把模型擅长的中间判断压扁：

- 用户仍在支持领域内，但缺少可继承实体；
- 用户给出候选选择，需要结合上一轮候选；
- 用户表达有情绪或强指代，需要先自然确认理解；
- 用户问题可执行但需要解释默认值选择依据。

扩大调查结论：`ReadPlan` schema 作为执行边界应保留，但 planner 对外不宜长期只有 `ReadPlan / NO_QUERY` 两态。最小可行方向是引入极薄的四态 outcome：`read_plan`、`need_clarification`、`unsupported_query`、`pass_through`，让模型拥有可解释的澄清、不支持判断与普通对话放行空间。

其中 `unsupported_query` 只用于“用户确实在请求数据查询或系统操作，但当前查询链不支持或不能安全执行”；`pass_through` 用于普通寒暄、非查询解释、写作或开放聊天，应交还普通 provider 对话链，不应显示查询 stopline。

#### B. narrator 被 prompt 与 regex 双重强控

narrator system prompt 原先要求默认 `1 到 3 句`、不要固定栏目、不要小标题、不要键值对，并列出大量不应出现的 `orgunit` 字段与中文标题，见 `internal/server/cubebox_query_flow.go:503`。随后 `queryNarrationForbiddenPatterns` 又用 regex 拦截 `已完成只读查询`、`本次关注`、`详情如下`、`组织基本信息`、`上级组织`、`负责人`、`组织全路径`、`扩展字段` 以及普通 `active/disabled/true/false/null` 键值形态，见 `internal/server/cubebox_query_flow.go:105` 的历史版本。

扩大调查结论：这已经超出“防泄露”边界，进入“本地规定模型怎么写”的表达控制。应保留内部实现泄露检测，但移除普通中文结构、业务字段中文名、小标题、短列表和自然键值表达的失败判定。回答风格应主要通过 prompt 引导，而不是通过 terminal error 惩罚。

2026-04-25 本轮收敛已先落地最小改动：`queryNarrationForbiddenPatterns` 只保留原始 JSON/Markdown 代码块、`step-*`、`api_key/result_focus/payload/results`、内部 plan 字段和内部参数字段路径等泄露检测；普通“详情如下”、中文栏目、小标题、业务字段中文名、`状态：启用` 一类自然键值表达不再触发 terminal error。

#### C. 共享 query flow 仍在生成模块专属澄清与错误文案

`orgUnitSearchAmbiguousError.ClarifyingQuestion()` 在本地拼接“找到了多个与……匹配的组织，请提供组织编码……”并列出候选，见 `internal/server/cubebox_orgunit_executors.go:45`。共享 query flow 再通过 `queryExecutionClarifyingQuestion(...)` 消费该错误，见 `internal/server/cubebox_query_flow.go:912`。

这类文案本质上是候选解释与下一步引导，适合由模型基于候选、用户问题和会话语气生成。代码只应提供候选结构、候选数量、可选项 ID/名称/状态，以及“不能静默选择”的安全约束。

同类问题还包括 `queryExecutionErrorToTerminal(...)` 对 `errOrgUnitNotFound` 的 `orgunit_not_found` 固定文案映射，见 `internal/server/cubebox_query_flow.go:899`。短期可以保留通用错误码和安全兜底，但不应在共享层继续增长模块私有自然语言说明。

#### D. 查询链没有给模型足够上下文，却要求模型正确理解

前端 `streamTurn(...)` 当前只提交 `conversation_id`、`prompt`、`next_sequence`，见 `apps/web/src/pages/cubebox/api.ts:108`；后端 query canonical context 还把 `Page` 固定成 `"/app/cubebox"`，见 `internal/server/cubebox_query_flow.go:845`。因此知识包里“当前页面可补参”的规则没有真实运行时输入。

这不是模型能力不足，而是模型拿不到事实。代码不应用关键词补丁替代模型理解；应把当前页面、当前业务对象、当前选中组织、当前日期视图等事实以受控结构传入 planner/narrator，让模型做语义选择。

#### E. 查询上下文事件只保留单个实体，缺少候选与澄清状态

`QueryContext` 当前只有 `RecentConfirmedEntity`，见 `modules/cubebox/query_entity.go:17`。这限制了模型处理以下常见追问：

- “第一个”
- “最开始那个”
- “不是这个，另一个”
- “不用再问了，就是刚才那个”
- “按刚才的日期继续查”

扩大调查结论：本地代码应扩大“上下文供给”，但不要扩大“本地理解”。也就是说，代码负责从 canonical events 中提取有限窗口、候选、澄清状态和已确认实体列表；模型负责解析用户当前指代并决定澄清还是执行。

#### F. `DEV-PLAN-469` 第一阶段先停用会话压缩摘要

`BuildPromptViewWithCompaction(...)` 的 `buildSummaryText(...)` 当前只是把较早 timeline 拼成 `role: content` 文本，见 `modules/cubebox/compaction.go:141`；recent user message 还会按估算 token 做字符级截断，见 `modules/cubebox/compaction.go:202`。

按 `DEV-PLAN-469` 第一阶段，首步不是继续改良这段本地伪摘要，而是先停用会话压缩摘要能力：manual compact / pre-turn auto compact 可以保留统一入口，但 provider prompt view 不再消费本地拼接 `summary_text`，而是回到“完整历史视图 + canonical context”的基线。

因此，`468 P0` 不能再把 compaction 视为连续追问修复的前置增强项。连续追问、指代解析、候选澄清和 narrator 承接必须在**无摘要基线**下成立，依赖的是 canonical events、有限 `query_dialogue_context` 和 provider 主链正确消费完整历史视图，而不是依赖 `turn.context_compacted` 或本地摘要文本掩盖问题。

后续若 `CubeBox` 需要恢复真正的语义摘要或 remote compact，由 `DEV-PLAN-469` 后续阶段承接；该项不属于 `468 P0` 的交付前提。

#### G. 知识包校验仍停留在外形校验，模型与执行边界缺少早发现协同

`ValidateKnowledgePack(...)` 当前主要校验文件存在、fenced block 存在、`queries.md` 有 intents、`apis.md` 有 `api_key`、`examples.md` 有 `ReadPlan` 示例，见 `modules/cubebox/knowledge_pack.go:62`。它不校验知识包 API/参数集合与执行注册表一致。

该问题不是“应该让模型自由执行”，而是“应该让模型获得更可靠的、不会漂移的知识”。早发现校验应保护模型输入质量，避免模型按过期知识包生成计划后才在运行时失败。

#### H. 参数类型校验应更硬，不属于应回交模型的事项

与上述语义职责不同，参数类型、枚举和值域必须继续由代码 fail-closed。例如 `status` 只允许 `active/disabled/all`，`page/size/limit` 必须是合法整数，日期必须是 `YYYY-MM-DD`，这些不是模型自主发挥空间。

需要特别区分的是：自然语言别名折叠应由模型和知识包负责，例如“停用/无效”映射到 `disabled`；但执行层收到 canonical 参数后的类型与枚举校验必须继续硬拒绝非法值。`normalizeOptionalBool(...)` 这类布尔解析不应接受模糊字符串并静默落默认，相关风险已在 `DEV-PLAN-466` 归类。

#### I. provider/runtime/model 元数据被注入普通 prompt view，随后又被当成“需要防泄露”的假想风险

普通 Gateway 的 canonical context 会把 `provider_id`、`provider_type`、`model_slug`、`runtime` 注入 provider prompt view，见 `modules/cubebox/compaction.go:116`；查询链也会在 `buildQueryCanonicalContext(...)` 中填充这些字段，见 `internal/server/cubebox_query_flow.go:839`。这与 narrator 输入不同：narrator envelope 当前不包含 provider/secret/runtime，但普通对话 prompt view 是包含 provider/runtime/model 元数据的。

扩大调查结论：这不是 `queryNarrationForbiddenPatterns` 应继续扩张的理由，而是另一个边界问题。`provider_id/provider_type/model_slug/runtime` 可作为事件 metadata、运维诊断或 UI 状态，但默认不应进入给大模型的业务上下文。否则系统一边主动告诉模型 provider/runtime，一边再用禁词假设要防止模型泄露，属于自造泄露面。`secret` 当前仅作为 HTTP Authorization header 发送，不在 messages 中；不要把 `secret` 与 provider/runtime metadata 混为同一类风险。

#### J. `api_key` 命名制造密钥误解，属于内部执行键命名问题

查询链里的 `api_key` 是只读执行注册表的 API catalog key，例如 `orgunit.details`，见 `modules/cubebox/read_plan.go:25` 与 `modules/cubebox/read_executor.go:34`，不是 provider 密钥。当前命名会让模型、人和文档都把它和真实 secret/API key 混淆，进而催生过宽的“禁词式”安全需求。

扩大调查结论：短期可继续在泄露防线中拦 `api_key` 这个内部字段名；中期在契约层采用 `executor_key`，并保证它只存在于内部 planner/executor 协议，不进入用户可见叙述。`tool_key` 容易误导为通用工具平台，`read_api_key` 仍保留 `api_key` 误解，均不作为本计划推荐命名。

#### K. `ReadPlan` 把澄清文本直接作为用户回复，当前合理但边界偏窄

当 `MissingParams` 非空时，query flow 直接把 `ClarifyingQuestion` 写入 `turn.agent_message.delta`，见 `internal/server/cubebox_query_flow.go:229`。这比本地固定 stopline 更接近“模型生成澄清”，因此不属于 `queryNarrationForbiddenPatterns` 式假想限制。

但它仍受 `ReadPlan` schema 限制：澄清态只能携带 `missing_params + clarifying_question`，不能携带候选结构、已解析依据或下一步预期。结果是候选澄清仍容易回流到 Go 代码固定文案，见 `orgUnitSearchAmbiguousError.ClarifyingQuestion()`。后续应由 planner outcome 或 metadata event 承载候选结构，让模型生成用户可见追问。

### 2.8 可回交模型事项清单

基于 2.7，本计划补充以下“尽可能应由模型来做”的事项清单：

| 事项 | 当前偏差 | 回交方向 | 代码仍保留 |
| --- | --- | --- | --- |
| 查询域判断 | `NO_QUERY` 过粗，寒暄和不安全查询都会被压成固定 stopline | 模型输出 `read_plan`、`need_clarification`、`unsupported_query` 或 `pass_through` | 不支持/不安全查询安全兜底；普通对话放行到 provider |
| 默认值选择 | 知识包已有规则，但上下文不足时容易被本地补丁替代 | 模型按知识包与当前日期选择低风险默认值 | 日期格式与参数合法性校验 |
| 指代解析 | 只给最近单实体，不足以理解多轮指代 | 模型消费有限 `query_dialogue_context` 解析 | 事件提取与窗口裁剪 |
| 候选选择 | 本地拼候选澄清文案 | 模型基于候选列表自然追问或理解“第一个” | 候选结构、不可静默猜测约束 |
| 结果叙述 | prompt 与 regex 过度规定表达形态 | 模型按结果和上下文自然回答 | 禁止 raw JSON / 内部计划泄露 |
| 长历史摘要 | 本地拼接旧 timeline | 按 `DEV-PLAN-469 Phase 1` 先停用会话压缩摘要并回到完整历史视图；后续再评估模型语义摘要 | 原始事件、预算、当前上下文 reinjection |
| 页面补参 | 前端未传，后端硬编码 page | 模型基于受控页面/对象上下文选择参数 | 前端只传事实，不拼 prompt |
| 错误解释 | 共享层增长模块私有文案 | 模型解释可恢复业务错误与下一步 | 错误码、权限/执行失败 fail-closed |
| provider/runtime 元数据 | 普通 prompt view 注入 provider/model/runtime，后续又把泄露当成禁词风险 | 模型只获得业务上下文与必要能力边界，不获得运维元数据 | metadata event、管理 UI、日志可保留 |
| `api_key` 命名 | 内部执行键名称与真实密钥概念混淆 | 内部协议改为 `executor_key`，不进入用户可见叙述 | 执行注册表白名单与 schema 校验 |

## 3. 设计目标

1. [ ] 支持同一会话内 3 到 5 轮内的基本连续追问：`该组织`、`那个组织`、`刚才那个`、`最开始那个`、`它`。
2. [ ] 支持实体与日期共同继承：上一轮确认 `org_code=100000`、`as_of=2026-04-25` 后，下一轮“查该组织下级”不得再次追问 `parent_org_code`。
3. [ ] 支持候选选择闭环：当上一轮展示多个候选后，用户用名称、序号或短语选择一个，planner 能消费候选上下文。
4. [ ] 支持模型生成业务澄清问题：领域内缺参由模型按知识包自然追问，不由固定 stopline 取代。
5. [ ] 支持 narrator 消费轻量对话上下文：最终回答能体现“已按刚才的组织继续查询”，但不得编造结果中没有的事实。
6. [ ] 收缩本地表达层约束：只禁止内部实现泄露和原始结构化数据回显，不再用大量 regex 管控普通中文表达风格。
7. [ ] 在 `DEV-PLAN-469 Phase 1 / No-Summary Baseline` 下，上述连续追问链路仍然成立；`468 P0` 不依赖会话压缩摘要。
8. [ ] 冻结“安全硬护栏 vs 语义回交模型”的边界，避免继续把模型能力误压成本地规则。
9. [ ] 保持安全边界不变：权限、租户、只读注册表、`ReadPlan` schema、参数校验、执行失败 fail-closed 均不放松。

## 4. 非目标

- 不建设跨会话长期记忆。
- 不引入向量数据库、外部缓存、RAG 知识库或租户文档中台。
- 不让会话 compaction summary 成为查询事实源。
- 不新增第二套查询 API、第二套执行注册表或第二套授权系统。
- 不把 `orgunit` 业务语义重新硬编码到 `internal/server`。
- 不让前端拼 prompt 或承载查询语义。
- 不在本计划内重做整个 CubeBox UI、模型配置 UI 或 provider 网关。
- 不在 `468 P0` 内实现或恢复模型生成的长期语义摘要、remote compaction、memory pipeline 或跨会话记忆；当前阶段以 `DEV-PLAN-469 Phase 1 / No-Summary Baseline` 为前提，只把后续 compaction 重构登记为 `469` owner 输入。
- 不把“回交给模型”理解成放松 schema、参数类型、枚举、权限、白名单或执行失败边界。

## 5. 关键设计决策

### 5.1 上下文窗口有限、结构化、只服务同一会话

新增或扩展 `QueryContext` 时，只允许从当前 `conversation_id` 的 canonical events 中提取有限窗口：

- 最近 `3-5` 轮用户输入与助手回答摘要
- 最近 `N` 个已确认查询实体
- 最近一次澄清问题
- 最近一次候选列表
- 当前轮指代解析结果

不得读取其他会话，不得引入用户画像，不得把压缩摘要当成可执行查询事实；在 `DEV-PLAN-469 Phase 1` 下，查询上下文默认只从 canonical events 提取。

### 5.1A `DEV-PLAN-469` 第一阶段是 `468 P0` 的前置基线

- `468 P0` 默认前提：会话压缩摘要先停用；manual compact / pre-turn auto compact 不再向 provider prompt view 注入本地拼接 `summary_text`。
- 连续追问修复必须在“完整历史视图 + canonical events + query_dialogue_context”下成立，而不是靠 compaction summary 掩盖。
- planner、narrator 与 `QueryContextFromEvents(...)` 不得新增对 `turn.context_compacted` 的读取依赖。
- 后续若 `DEV-PLAN-469` Phase 2/3 恢复模型摘要或 remote compact，它也只能作为预算优化层，不得反向改写 `468 P0` 的事实来源和验收口径。

### 5.2 本地代码负责“给上下文”，不负责“替模型理解”

本地允许做：

- 从事件流提取结构化锚点
- 裁剪最近对话窗口
- 按固定 JSON 形态传给 planner/narrator
- 校验 planner 输出是否在白名单与 schema 内

本地不允许做：

- 用关键词判断 `该组织` 到底是哪种业务查询
- 在 `internal/server` 中硬填模块默认参数
- 用 capability-specific 规则改写用户意图
- 用本地模板替模型写最终回答

### 5.3 `NO_QUERY` 从业务澄清与普通对话中退出

`NO_QUERY` 不应继续作为单一业务结果。领域内缺参、歧义、候选选择，应走模型生成的澄清态；普通寒暄、非查询解释、写作或开放聊天，应作为 `pass_through` 回到普通 provider 对话链；只有用户确实在请求数据查询或系统操作但当前查询链不支持或不能安全执行时，才走查询 stopline。

首期允许两种最小实现路径，落地时二选一：

1. **轻量保持 `ReadPlan`**：领域内缺参继续用 `ReadPlan.missing_params + clarifying_question`，但补充知识包与 planner prompt，明确带有最近上下文时不得轻易 `NO_QUERY`。
2. **引入最小 planner outcome**：新增一个很薄的 planner decision envelope，仅区分 `read_plan`、`need_clarification`、`unsupported_query`、`pass_through`，其中 `read_plan` 内部仍复用现有 `ReadPlan`。

若要覆盖 `你好` 这类普通寒暄，首期应优先选择 planner outcome envelope；仅保持 `ReadPlan + NO_QUERY` 无法区分普通对话与不安全查询，除非额外再引入一层本地分类器，而这会重新把语义判断拉回代码侧。

无论选择哪种路径，都不得引入 DAG、workflow engine、工具发现平台或新的执行 DSL。

### 5.4 narrator 获得上下文，但仍只依据 results 给事实结论

narrator 可以看到：

- 当前轮用户输入
- 被解析到的上一轮实体
- 上一轮问题/回答短摘要
- 当前执行 plan 与 results

但事实性结论仍只能来自 `results`。对话上下文只能用于指代解释、衔接语气和避免重复，不得用于补充结果中不存在的业务事实。

### 5.5 metadata event 是查询上下文来源，不是用户可见输出

可新增不可见 canonical metadata event，例如：

- `turn.query_context.resolved`
- `turn.query_candidates.presented`
- `turn.query_clarification.requested`

这些事件只服务下一轮 planner/narrator，不直接进入用户可见消息，也不作为授权来源。

### 5.6 `api_key` 去密钥化命名方案

查询链里的 `api_key` 不是 provider credential，也不是外部系统密钥，而是 planner 选择只读执行器时使用的内部注册键。为避免继续把内部执行键误读成 secret，本计划后续采用 `executor_key` 作为唯一命名。

具体替换范围：

- `ReadPlanStep.APIKey` / JSON `api_key` -> `ReadPlanStep.ExecutorKey` / JSON `executor_key`。
- `RegisteredExecutor.APIKey` -> `RegisteredExecutor.ExecutorKey`。
- `ExecuteResult.APIKey` -> `ExecuteResult.ExecutorKey`。
- `Resolve(apiKey)`、局部变量与错误上下文中的 `apiKey` -> `Resolve(executorKey)` / `executorKey`。
- `QueryEntity.SourceAPIKey` / JSON `source_api_key` -> `QueryEntity.SourceExecutorKey` / JSON `source_executor_key`。
- 知识包 `apis.md`、`examples.md` 与 planner 示例中的 `api_key` -> `executor_key`。
- 错误文案从 `api_key required/not registered` 收敛为 `executor_key required/not registered`。

实施边界：

- Greenfield + No Legacy：直接替换内部契约，不长期保留双字段兼容；旧 `api_key` 输入应由 `DecodeReadPlan` 的 unknown-field 校验拒绝。
- 最终泄露防线短期同时拦截 `api_key` 与 `executor_key`，因为二者都是内部 planner/executor 字段，不应出现在用户可见回答中。
- 不修改 `ProviderChatRequest.APIKey` 等真实 provider credential 字段；这些字段语义上确实用于 provider Authorization header，不能与查询执行键混改。
- 该改名不放松执行注册表白名单、参数 schema、权限或只读执行边界，只消除命名误导。

## 6. 实施切片

### Slice A：扩展查询会话上下文

1. [ ] 以 `DEV-PLAN-469 Phase 1 / No-Summary Baseline` 作为 `468 P0` 前置；实现、测试与真实页面复验均不得假设 compaction summary 可用。
2. [ ] 将 `modules/cubebox.QueryContext` 从单个 `RecentConfirmedEntity` 扩展为有限窗口结构，至少包含：
   - `RecentConfirmedEntities []QueryEntity`
   - `RecentDialogueTurns []QueryDialogueTurn`
   - `LastClarification *QueryClarification`
   - `RecentCandidates []QueryCandidate`
3. [ ] 保留当前 `RecentConfirmedEntity` 兼容访问器或派生字段，避免一次性大面积改调用方。
4. [ ] `QueryContextFromEvents(...)` 只从当前会话事件流提取，默认最多保留最近 `5` 个对话片段与最近 `5` 个实体/候选。
5. [ ] 单元测试覆盖：
   - 多个实体时保留最近列表
   - 无效实体跳过
   - 候选事件提取
   - 澄清事件提取
   - 窗口裁剪

### Slice B：扩展 planner 输入

1. [ ] 将 `cubeboxReadPlanProductionInput` 中的 `RecentEntity` 扩展为完整 `QueryContext`。
2. [ ] `buildPlannerMessages(...)` 注入一个稳定 JSON 块，例如 `query_dialogue_context`，包含最近实体、上一轮问答摘要、候选与澄清状态。
3. [ ] planner system prompt 明确：
   - 当前轮显式输入优先
   - `该/那个/它/刚才/最开始` 应优先尝试从 `query_dialogue_context` 解析
   - 支持领域内缺参应输出澄清态，而不是 `NO_QUERY`
   - 上下文不是授权来源，不能绕过 API 白名单
4. [ ] 补充知识包样例，覆盖“查详情 -> 查该组织下级”的连续追问。

### Slice C：收敛 `NO_QUERY` 与澄清协议

1. [ ] 冻结 `NO_QUERY` 退出长期契约：planner outcome 应显式区分 `unsupported_query` 与 `pass_through`。
2. [ ] 对领域内缺参、指代歧义、候选选择，使用模型生成的澄清问题。
3. [ ] 如果采用 planner outcome envelope，新增 decode/validate 测试，保持 envelope 只有四态，不扩大为通用 workflow。
4. [ ] 保留服务端 stopline，但只作为不支持/不安全查询的安全兜底；不得替代正常业务澄清或普通对话。
5. [ ] 增加测试：带有最近实体上下文的“查该组织下级”不得走 `NO_QUERY` stopline。
6. [ ] 增加测试：`你好`、`请介绍一下你能做什么` 等普通对话不得显示查询 stopline，应进入普通 provider 对话链。

### Slice D：给 narrator 传入轻量对话上下文

1. [ ] 扩展 `cubeboxQueryNarrationInput` 与 `cubeboxQueryNarrationEnvelope`，新增 `DialogueContext` 或 `ResolvedContext`。
2. [ ] narrator prompt 明确：
   - 可以说“我按刚才的组织继续查”
   - 事实结论仍只能来自 `results`
   - 不得把上下文当成额外查询结果
3. [ ] 回答样式不再靠大批 regex 约束，而靠 prompt 与少量安全校验。
4. [ ] 测试覆盖：narrator 接收到 resolved context，且不会泄露内部 JSON。

### Slice E：削弱表达层过度管控

1. [x] 收缩 `queryNarrationForbiddenPatterns`：
   - 保留：原始 JSON、Markdown 代码块、`api_key`、`payload`、`results`、step id、内部 plan 字段、内部参数字段路径等实现细节泄露。
   - 移除：普通中文栏目词、普通“详情如下”、普通自然语言键值表达、业务字段中文名。
2. [x] 对误杀风险高的 regex 改成更明确的结构化泄露检测。
3. [x] 对普通风格不再 terminal error；只有内部实现泄露才失败。
4. [x] 增加回归测试：自然中文列表、短段落、小标题在不泄露内部字段时可通过。
5. [ ] 后续继续把 narrator 输入从 raw `plan/results` 收敛为安全 presentation DTO，从源头减少最后一道拦截的压力。

### Slice F：补充查询 metadata event

1. [ ] 成功查询后继续写 `turn.query_entity.confirmed`，并补充写入解析来源，例如当前轮是否由上一轮实体继承。
2. [ ] 当 planner/narrator 展示候选或要求澄清时，写入不可见 metadata event。
3. [ ] 事件 payload 必须小而稳定，只记录后续追问需要的锚点，不记录整份查询结果。
4. [ ] `QueryContextFromEvents(...)` 消费这些 event，形成下一轮 planner/narrator 输入。

### Slice G：真实会话复验

1. [ ] 在真实页面执行并记录以下链路：
   - `查一下 100000 在 2026-04-25 的组织详情`
   - `查该组织的下级组织`
   - `那它的负责人呢`
2. [ ] 验证第二轮继承 `org_code=100000` 与 `as_of=2026-04-25`，不得重复追问 `parent_org_code`。
3. [ ] 验证 unsupported domain 仍 fail-closed，不掉回“我没有查询接口/权限”的虚假描述。
4. [ ] 证据登记到 `docs/dev-records/DEV-PLAN-468-READINESS.md`。

### Slice H：扩大限制面分流与回交优先级

本 slice 只冻结分流，不要求一次性实现所有扩大项。

1. [ ] `P0` 纳入：
   - 以 `DEV-PLAN-469 Phase 1` 停用会话压缩摘要为前置；连续追问闭环不得依赖 compaction summary；
   - `query_dialogue_context` 有限上下文输入；
   - `NO_QUERY` 与澄清态、普通对话 `pass_through` 区分；
   - narrator 上下文输入；
   - 表达层 regex 收缩；
   - 候选/澄清 metadata event 的最小闭环。
2. [ ] `P1` 纳入：
   - 前端向 `/internal/cubebox/turns:stream` 传受控页面/对象事实；
   - 共享 query flow 去除模块专属澄清文案，把候选列表交给模型追问；
   - 知识包 API/参数集合与执行注册表一致性校验；
   - provider/runtime/model 元数据从 provider prompt view 中剥离，仅保留在事件 metadata、管理 UI 或日志中。
3. [ ] `P2/后续 owner` 纳入：
   - 模型参与会话语义摘要或摘要改写；
   - 更完整的长结果语义收敛；
   - 第二个业务模块接入后的共享 narrator 去模块化污染复核；
   - `ReadPlanStep.api_key` 内部字段命名去密钥化，采用 `executor_key`，同步替换执行注册表、执行结果、实体来源字段、知识包示例与错误文案；旧 `api_key` 不保留长期兼容。
4. [ ] 每个后续实现 PR 必须先说明：它是在“给模型事实/上下文”，还是在“替模型做语义判断”。后者默认需要收敛或单独论证。

## 7. 验收场景

### 7.1 基本连续追问

| 场景 | 输入序列 | 期望 |
| --- | --- | --- |
| 实体继承 | `查一下 100000 在 2026-04-25 的组织详情` -> `查该组织的下级组织` | 第二轮直接继承 `100000` 与日期，进入可执行查询或给出真实结果 |
| 日期继承 | `查一下 100000 今天的详情` -> `查该组织的下级组织` | 第二轮继承同一自然日 |
| 多轮指代 | `查 100000` -> `查它的下级` -> `再查它的上级` | planner 能根据最近语境解析，不因中间一轮回答失去锚点 |
| 最开始指代 | `查 100000` -> `查 200000` -> `查最开始那个组织的下级` | 若上下文窗口内有两个候选，优先解析“最开始”为较早实体；不确定时由模型澄清 |
| 候选选择 | `查飞虫公司` -> 系统给候选 -> `第一个` | 下一轮能消费候选列表并形成明确 plan |

### 7.2 澄清与 unsupported 区分

| 场景 | 期望 |
| --- | --- |
| `DEV-PLAN-469 Phase 1` 无摘要基线 | 连续追问闭环在不依赖 compaction summary 的情况下仍成立 |
| 支持领域缺参 | 由模型自然追问缺少的业务参数 |
| 指代歧义 | 由模型说明有多个可能对象并请用户选择 |
| 不支持或不安全查询 | 统一安全 stopline，不编造系统能力 |
| 普通寒暄/非查询对话 | 回到普通 provider 对话链，不显示查询 stopline |
| 模型输出越界 plan | schema / registry / executor 校验 fail-closed |

### 7.3 表达层

| 场景 | 期望 |
| --- | --- |
| 普通自然语言列表 | 允许，只要不泄露内部字段 |
| 小标题式回答 | 允许或由 prompt 引导减少，但不因普通中文标题直接失败 |
| 原始 JSON 回显 | 必须拒绝 |
| `api_key` / `payload` / `results` / `step-*` 泄露 | 必须拒绝 |
| `状态：启用`、`负责人：未记录` 一类自然键值表达 | 允许，只要不泄露内部字段 |

## 8. 测试与验证

### 8.1 单元测试

- `modules/cubebox/query_entity_test.go` 或相邻职责测试：覆盖扩展后的 `QueryContextFromEvents(...)`。
- `modules/cubebox/read_plan_test.go` 或新增 planner decision 测试：覆盖澄清态/unsupported 态 decode 与 validate。
- narrator 校验测试：覆盖误杀减少与内部实现泄露拦截。

### 8.2 服务端测试

- `internal/server/cubebox_api_test.go`：补充同一会话连续追问夹具，使用 stub provider 验证 planner 输入包含 `query_dialogue_context`。
- 验证 `NO_QUERY` 仅在 unsupported domain 走 stopline。
- 验证查询成功后 metadata event 写入顺序与 payload 裁剪。
- 验证在 `DEV-PLAN-469 Phase 1` 基线下，planner/narrator/QueryContext 不依赖 compaction summary。

### 8.3 真实页面验证

- 使用当前真实 provider 基线：`provider_id=openai-compatible`、`provider_type=codex`、`model_slug=gpt-5.2`。
- 通过主应用壳层右侧 `CubeBox` 抽屉在 `DEV-PLAN-469 Phase 1 / No-Summary Baseline` 下执行 7.1 的关键链路。
- 保存网络请求、canonical event 片段与最终回答样本到 readiness 记录。

### 8.4 必跑门禁

本计划实施涉及 Go 代码与文档时，按 `AGENTS.md` 触发器矩阵执行：

- Go 代码：`go fmt ./... && go vet ./... && make check lint && make test`
- 文档：`make check doc`
- 涉及历史对话面或旧兼容语义：`make check chat-surface-clean`

## 9. Stopline

1. [ ] 不得新增长期记忆、向量库、外部缓存或跨会话用户画像。
2. [ ] `468 P0` 不得依赖 compaction summary；在 `DEV-PLAN-469 Phase 1` 下该能力视为停用。后续阶段即使恢复，也最多作为 provider prompt-view 的叙事上下文，不得成为查询事实源。
3. [ ] 不得绕过 `ExecutionRegistry`、只读 API、租户/权限校验或现有参数校验。
4. [ ] 不得在 `internal/server` 继续新增 `orgunit` 关键词补丁来“修”某一句用户输入。
5. [ ] 不得通过前端拼 prompt 解决连续追问。
6. [ ] 不得把 planner outcome 扩张成 workflow/DAG/tool platform。
7. [ ] 不得为避免测试困难而放松 schema、降低门禁或扩大 coverage 排除项。
8. [ ] 不得以“模型自主性”为理由放松权限、租户、白名单、参数类型、枚举或只读边界。
9. [ ] 不得继续用普通中文表达禁词、栏目词或业务字段中文名作为 terminal error 条件来限制模型回答风格。

## 10. 交付物

1. [ ] 扩展后的 `QueryContext` 与事件提取测试。
2. [ ] planner 输入包含有限 `query_dialogue_context`。
3. [ ] `NO_QUERY` 与澄清态边界收敛。
4. [ ] narrator 输入包含轻量对话上下文。
5. [ ] `queryNarrationForbiddenPatterns` 收缩到内部实现泄露防线。
6. [ ] metadata event 支撑候选、澄清和解析来源。
7. [ ] 同会话连续追问服务端测试。
8. [ ] 真实页面复验证据记录。
9. [ ] 模型限制面扩大调查清单与回交优先级已冻结。
10. [ ] `AGENTS.md` Doc Map 已登记本计划。

## 11. 当前阶段执行记录

1. [X] 新建本专项计划文档：`docs/dev-plans/468-cubebox-session-continuity-and-model-autonomy-improvement-plan.md`（2026-04-25 13:29 CST）
2. [X] 更新 `AGENTS.md` 文档地图。（2026-04-25 13:29 CST）
3. [X] 执行 `make check doc` 并记录结果：`[doc] OK`。（2026-04-25 13:29 CST）
4. [X] 扩大调查本项目当前对大模型的限制面，新增“安全硬护栏 vs 语义回交模型”口径、限制面清单、可回交模型事项清单与分流优先级。（2026-04-25）
5. [X] 根据 `DEV-PLAN-469` 第一阶段，将 `468 P0` 调整为 `No-Summary Baseline` 前提，明确先停用会话压缩摘要，再推进连续追问修复。（2026-04-25 15:10 CST）
