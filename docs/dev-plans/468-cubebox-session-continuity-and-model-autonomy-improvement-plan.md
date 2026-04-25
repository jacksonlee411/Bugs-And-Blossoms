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

需要补充冻结的是：`turn.query_entity.confirmed` 作为“执行后事实锚点”的方向是正确的，但 `RecentConfirmedEntity` 作为“本地代码替模型提前选定当前指代对象”的 owner 方向并不成立。前者应保留，后者应降级为兼容访问器或派生字段。

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

更关键的问题不是“字段太少”，而是 owner 漂移：当前 `RecentConfirmedEntity` 让本地代码在 planner 之前先做了一次语义收敛，相当于先替模型决定“当前最应该参考哪个实体”。这会把“事实供给”与“语义判断”混在一起。重估后应改为：代码只提供有限窗口内的结构化事实；模型负责判断当前轮到底引用哪个实体、是否继承日期、是否需要澄清。

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

该规则本身可以继续保留。`468` 的方向不是新建另一套 planner 状态机，而是在**不扩写 planner envelope** 的前提下，让模型通过知识包和有限上下文生成更好的 `ReadPlan` 编排，或在确实缺参时返回现有 `missing_params + clarifying_question`。本计划不引入 `read_plan/need_clarification/pass_through/unsupported_query` 一类额外状态枚举。

### 2.7 扩大调查：当前仍存在的模型限制面

本轮扩大调查后，当前限制大模型发挥的实现点不止连续追问，还包括以下几类。

#### A. planner 虽继续输出 `ReadPlan/NO_QUERY`，但知识包与运行时能力不足以支撑模型编排

`buildPlannerMessages(...)` 要求模型只有两种职责：输出严格合法 `ReadPlan JSON`，或输出 `NO_QUERY`，见 `internal/server/cubebox_query_flow.go:363`。这能保证执行入口简单，但会把模型擅长的中间判断压扁：

- 用户仍在支持领域内，但缺少可继承实体；
- 用户给出候选选择，需要结合上一轮候选；
- 用户表达有情绪或强指代，需要先自然确认理解；
- 用户问题可执行但需要解释默认值选择依据。

扩大调查结论：`468` 不通过新增 planner 状态机来解决这些问题，而是通过两件事收敛：

- 让 planner 获得更好的 `query_dialogue_context`
- 让 `ReadPlan` 在保持现有 schema 的前提下支持**通用前序步骤结果引用**

也就是说，planner 仍只输出现有 `ReadPlan` 或现有澄清态；但“先搜索定位、再续查详情/下级组织”的决策应由模型依据知识包来做，而不是由本地代码写模块专用编排。

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

需要明确反转的 owner 是：

- **应保留在代码里的**：执行完成后记录 `turn.query_entity.confirmed`，并把它作为 append-only query facts 的一部分。
- **应回交给模型的**：在多个已确认实体、候选和最近对话同时存在时，判断本轮“它 / 该组织 / 最开始那个 / 第一个”究竟指谁。
- **不应再由代码长期承担的**：从事件流中挑一个 privileged winner 再以 `RecentConfirmedEntity` 身份喂给 planner，仿佛这已经是系统结论。

#### F. `DEV-PLAN-469` 第一阶段先停用会话压缩摘要

`BuildPromptViewWithCompaction(...)` 的 `buildSummaryText(...)` 当前只是把较早 timeline 拼成 `role: content` 文本，见 `modules/cubebox/compaction.go:141`；recent user message 还会按估算 token 做字符级截断，见 `modules/cubebox/compaction.go:202`。

按 `DEV-PLAN-469` 第一阶段，首步不是继续改良这段本地伪摘要，而是先停用会话压缩摘要能力：manual compact 已取消为产品需求，只保留 pre-turn auto prompt view 准备；provider prompt view 不再消费本地拼接 `summary_text`，而是回到“完整历史视图 + canonical context”的基线。

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

#### K. `ReadPlan` 把澄清文本直接作为用户回复，当前可保留

当 `MissingParams` 非空时，query flow 直接把 `ClarifyingQuestion` 写入 `turn.agent_message.delta`，见 `internal/server/cubebox_query_flow.go:229`。这比本地固定 stopline 更接近“模型生成澄清”，因此不属于 `queryNarrationForbiddenPatterns` 式假想限制。

当前无需为此再引入新的 planner outcome。更简单的方向是：

- 保留现有 `missing_params + clarifying_question`
- 用 metadata event 与 `query_dialogue_context` 提供候选结构和上下文事实
- 让模型继续在现有澄清态内生成更自然的追问

### 2.8 可回交模型事项清单

基于 2.7，本计划补充以下“尽可能应由模型来做”的事项清单：

| 事项 | 当前偏差 | 回交方向 | 代码仍保留 |
| --- | --- | --- | --- |
| 查询域判断 | `NO_QUERY` 过粗，容易掩盖“本可执行但缺上下文”的情况 | 在不扩写 planner 状态机的前提下，模型继续只输出 `ReadPlan` / 现有澄清态 / `NO_QUERY`；通过知识包和上下文减少误判 | 不支持/不安全查询安全兜底 |
| 默认值选择 | 知识包已有规则，但上下文不足时容易被本地补丁替代 | 模型按知识包与当前日期选择低风险默认值 | 日期格式与参数合法性校验 |
| 指代解析 | 只给最近单实体，不足以理解多轮指代 | 模型消费有限 `query_dialogue_context` 解析 | 事件提取与窗口裁剪 |
| 候选选择 | 本地拼候选澄清文案 | 模型基于候选列表自然追问或理解“第一个” | 候选结构、不可静默猜测约束 |
| 多步编排 | 运行时没有稳定的前序结果引用机制，模型即使想“先搜再查”也难以合法表达 | 模型依据知识包生成多步 `ReadPlan`；代码仅提供通用 `@step-x.field` 引用解析 | `depends_on` 顺序校验、引用解析 fail-closed |
| 结果叙述 | prompt 与 regex 过度规定表达形态 | 模型按结果和上下文自然回答 | 禁止 raw JSON / 内部计划泄露 |
| 长历史摘要 | 本地拼接旧 timeline | 按 `DEV-PLAN-469 Phase 1` 先停用会话压缩摘要并回到完整历史视图；后续再评估模型语义摘要 | 原始事件、预算、当前上下文 reinjection |
| 页面补参 | 前端未传，后端硬编码 page | 模型基于受控页面/对象上下文选择参数 | 前端只传事实，不拼 prompt |
| 错误解释 | 共享层增长模块私有文案 | 模型解释可恢复业务错误与下一步 | 错误码、权限/执行失败 fail-closed |
| provider/runtime 元数据 | 普通 prompt view 注入 provider/model/runtime，后续又把泄露当成禁词风险 | 模型只获得业务上下文与必要能力边界，不获得运维元数据 | metadata event、管理 UI、日志可保留 |

## 3. 设计目标

1. [ ] 支持同一会话内 3 到 5 轮内的基本连续追问：`该组织`、`那个组织`、`刚才那个`、`最开始那个`、`它`。
2. [ ] 支持实体与日期共同继承：上一轮确认 `org_code=100000`、`as_of=2026-04-25` 后，下一轮“查该组织下级”不得再次追问 `parent_org_code`。
3. [ ] 支持候选选择闭环：当上一轮展示多个候选后，用户用名称、序号或短语选择一个，planner 能消费候选上下文。
4. [ ] 支持模型生成业务澄清问题：领域内缺参由模型按知识包自然追问，不由固定 stopline 取代。
5. [ ] 支持模型依据知识包生成最小必要的线性多步 `ReadPlan`：例如“先按名称搜索唯一命中组织，再继续查询其下级组织/详情”。
6. [ ] 支持 narrator 消费轻量对话上下文：最终回答能体现“已按刚才的组织继续查询”，但不得编造结果中没有的事实。
7. [ ] 收缩本地表达层约束：只禁止内部实现泄露和原始结构化数据回显，不再用大量 regex 管控普通中文表达风格。
8. [ ] 在 `DEV-PLAN-469 Phase 1 / No-Summary Baseline` 下，上述连续追问链路仍然成立；`468 P0` 不依赖会话压缩摘要。
9. [ ] 冻结“安全硬护栏 vs 语义回交模型”的边界，避免继续把模型能力误压成本地规则。
10. [ ] 保持安全边界不变：权限、租户、只读注册表、`ReadPlan` schema、参数校验、执行失败 fail-closed 均不放松。

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
- 最近一次候选列表（真实结果条数不做限制，但注入模型的候选最多保留前 `100` 条）
- 当前轮指代解析结果

不得读取其他会话，不得引入用户画像，不得把压缩摘要当成可执行查询事实；在 `DEV-PLAN-469 Phase 1` 下，查询上下文默认只从 canonical events 提取。

### 5.1A `DEV-PLAN-469` 第一阶段是 `468 P0` 的前置基线

- `468 P0` 默认前提：会话压缩摘要先停用；只保留 pre-turn auto prompt view 准备，manual compact 不再作为产品能力存在，也不再向 provider prompt view 注入本地拼接 `summary_text`。
- 连续追问修复必须在“完整历史视图 + canonical events + query_dialogue_context”下成立，而不是靠 compaction summary 掩盖。
- planner、narrator 与 `QueryContextFromEvents(...)` 不得新增对 `turn.context_compacted` 的读取依赖。
- 后续若 `DEV-PLAN-469` Phase 2/3 恢复模型摘要或 remote compact，它也只能作为预算优化层，不得反向改写 `468 P0` 的事实来源和验收口径。

### 5.2 本地代码负责“给上下文”，不负责“替模型理解”

本地允许做：

- 从事件流提取结构化锚点
- 裁剪最近对话窗口
- 按固定 JSON 形态传给 planner/narrator
- 校验 planner 输出是否在白名单与 schema 内
- 保留执行后 confirmed entity / candidate / clarification metadata 的 append-only 事实记录

本地不允许做：

- 用关键词判断 `该组织` 到底是哪种业务查询
- 在 `internal/server` 中硬填模块默认参数
- 用 capability-specific 规则改写用户意图
- 用本地模板替模型写最终回答
- 在多实体并存时预先替模型决定“当前就是最近这个实体”

补充 owner 判定：

- `confirmed entity` 是**执行后事实**，属于代码侧事实层；
- `resolved entity for current turn` 是**当前轮语义决策**，属于模型侧理解层；
- 二者可以在 metadata event 中关联，但不应再混成单个 `RecentConfirmedEntity` owner。

### 5.3 保持现有 `ReadPlan` 契约，不新增 planner 状态机

`468` 的简化原则是：继续使用现有 `ReadPlan`、`missing_params + clarifying_question` 和 `NO_QUERY`，不在本计划内新增 `pass_through`、`unsupported_query` 或其他 planner decision envelope。

本计划的重点不是扩写 planner 输出类型，而是：

- 通过知识包让模型更少误用 `NO_QUERY`
- 通过上下文供给让模型更少“因为拿不到事实而只能追问编码”
- 通过通用前序结果引用能力，让模型能在现有 `ReadPlan` 内完成线性多步编排

也就是说，`NO_QUERY` 仍保留为现有兜底；但 `468 P0` 的目标是让模型在更多本可执行场景下生成合法 `ReadPlan`，而不是新增一套代码侧分流状态机。

### 5.3A 多步编排 owner 在模型，代码只提供通用引用解析

`ReadPlan` 继续保持线性多步 `depends_on` 约束，不引入 DAG、workflow engine、工具发现平台或新的执行 DSL。

在此基础上，新增一类**通用参数引用语法**，供模型通过知识包驱动编排，例如：

- `@step-1.target_org_code`
- `@step-1.payload.target_org_code`

代码侧职责仅限于：

- 校验引用语法是否合法
- 只允许引用前序 step 的受控字段
- 执行前把引用解析成真实参数值
- 解析失败时 fail-closed

代码侧不得因为 `orgunit`、`dict` 或未来其他模块而新增模块专用编排分支。是否“先搜索再查详情/子组织”，由模型依据知识包、当前问题和上下文决定。

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

其中 `turn.query_candidates.presented` 的口径补充冻结为：

- 不限制实际查询结果返回给用户的总条数。
- 候选事件只提炼后续追问真正需要的最小字段，例如 `entity_key`、`name`、`as_of`。
- 注入 `query_dialogue_context` 的候选最多保留前 `100` 条；超过部分不再继续喂给模型。
- 代码不得在写候选事件时预先替模型挑选唯一 winner，只做顺序保留与字段裁剪。

## 6. 实施切片

### Slice A：统一 `QueryContext` 与 metadata event 最小闭环

1. [ ] 以 `DEV-PLAN-469 Phase 1 / No-Summary Baseline` 作为 `468 P0` 前置；实现、测试与真实页面复验均不得假设 compaction summary 可用。
2. [ ] 将 `modules/cubebox.QueryContext` 从单个 `RecentConfirmedEntity` 扩展为有限窗口结构，至少包含：
   - `RecentConfirmedEntities []QueryEntity`
   - `RecentDialogueTurns []QueryDialogueTurn`
   - `LastClarification *QueryClarification`
   - `RecentCandidates []QueryCandidate`
3. [ ] 保留当前 `RecentConfirmedEntity` 兼容访问器或派生字段，避免一次性大面积改调用方；但它降级为 compatibility accessor，不再作为 planner 语义 owner。
4. [ ] 成功查询后继续写 `turn.query_entity.confirmed`；当查询返回可供下一轮指代解析的实体列表、候选列表或澄清请求时，统一写入不可见 metadata event。
5. [ ] 事件 payload 必须小而稳定，只记录后续追问需要的锚点，不记录整份查询结果；普通列表结果也应可转为 `recent_candidates`，真实返回条数不做限制，但注入模型时最多保留前 `100` 条。
6. [ ] `QueryContextFromEvents(...)` 只从当前会话事件流提取，默认最多保留最近 `5` 个对话片段与最近 `5` 个实体/候选组；消费 confirmed/candidates/clarification/resolved-context 事件形成下一轮 planner 输入。
7. [ ] 若需记录“当前轮解析到了哪个实体”，应写成单独的 resolved-context metadata，而不是回写覆盖 `confirmed entity` 的事实语义。
8. [ ] 单元测试覆盖：
   - 多个实体时保留最近列表
   - 无效实体跳过
   - 普通列表结果转 `recent_candidates`
   - 候选事件提取
   - 澄清事件提取
   - 窗口裁剪
   - 候选列表超过 `100` 条时仅向模型注入前 `100` 条
   - 不再在 `QueryContextFromEvents(...)` 中做“当前轮应选哪个实体”的本地 winner 判定

### Slice B：扩展 planner 输入与知识包指引

1. [ ] 将 `cubeboxReadPlanProductionInput` 中的 `RecentEntity` 扩展为完整 `QueryContext`。
2. [ ] `buildPlannerMessages(...)` 注入一个稳定 JSON 块，例如 `query_dialogue_context`，包含最近实体、上一轮问答摘要、候选与澄清状态。
3. [ ] planner system prompt 明确：
   - 当前轮显式输入优先
   - `该/那个/它/刚才/最开始` 应优先尝试从 `query_dialogue_context` 解析
   - 支持领域内缺参应输出澄清态，而不是 `NO_QUERY`
   - 上下文不是授权来源，不能绕过 API 白名单
   - `RecentConfirmedEntity` 仅可作为兼容提示，不代表代码已经替模型选定当前引用对象
4. [ ] 补充知识包规则与样例，覆盖：
   - `查详情 -> 查该组织下级` 的连续追问
   - `只有名称时先搜索唯一命中，再继续查详情/下级组织` 的线性编排
   - 列表结果转候选后，模型可理解用户说的 `第一个`、`那个公司`

### Slice C：开放通用前序结果引用解析

1. [ ] 在不改变 `ReadPlan` 总体 schema 的前提下，引入通用前序结果引用能力，例如 `@step-1.target_org_code`。
2. [ ] 引用能力只允许读取前序 step 的受控 payload 字段；不得允许任意表达式、任意 JSONPath 或代码执行。
3. [ ] 执行前统一解析引用；解析失败、字段不存在、类型不匹配时 fail-closed。
4. [ ] 知识包只负责告诉模型什么时候适合“先 search、再 list/details”；代码只负责把合法引用解析成真实参数，不承接业务专用编排分支。
5. [ ] 增加测试：带有最近实体上下文的“查该组织下级”不得走 `NO_QUERY` stopline。
6. [ ] 增加测试：`查询飞虫公司的下级组织，只有名称` 可由模型生成“先 search、再 list”的合法线性 `ReadPlan`。

### Slice D：真实会话复验与 readiness

1. [ ] 在真实页面执行并记录以下链路：
   - `系统里有哪些组织`
   - `列出它的全部下级组织`
   - `请列出鲜花公司的全部下级组织，允许先按名称搜索定位该组织`
2. [ ] 在真实页面执行并记录以下链路：
   - `查一下 100000 在 2026-04-25 的组织详情`
   - `查该组织的下级组织`
   - `那它的负责人呢`
3. [ ] 验证同会话内已出现并已返回给用户的组织，不得再次机械性要求用户补 `org_code`；当目标查询需要稳定编码且用户只给名称时，允许模型先 search 唯一命中后继续执行。
4. [ ] 验证第二轮继承 `org_code=100000` 与 `as_of=2026-04-25`，不得重复追问 `parent_org_code`。
5. [ ] 验证 unsupported domain 仍 fail-closed，不掉回“我没有查询接口/权限”的虚假描述。
6. [ ] 证据登记到 `docs/dev-records/DEV-PLAN-468-READINESS.md`。

### Slice E：`P1/P2` 后续收口

1. [x] 已完成的表达层收缩保留在 `468` 范围内：
   - 收缩 `queryNarrationForbiddenPatterns`
   - 对误杀风险高的 regex 改成更明确的结构化泄露检测
   - 对普通风格不再 terminal error
   - 增加自然中文列表/短段落/小标题可通过的回归测试
2. [ ] `P1` 纳入：
   - 给 narrator 传入轻量对话上下文，但事实结论仍只能来自 `results`
   - 把 narrator 输入从 raw `plan/results` 收敛为安全 presentation DTO
   - 前端向 `/internal/cubebox/turns:stream` 传受控页面/对象事实
   - 共享 query flow 去除模块专属澄清文案，把候选列表交给模型追问
   - 知识包 API/参数集合与执行注册表一致性校验
   - provider/runtime/model 元数据从 provider prompt view 中剥离，仅保留在事件 metadata、管理 UI 或日志中
3. [ ] `P2/后续 owner` 纳入：
   - 模型参与会话语义摘要或摘要改写
   - 更完整的长结果语义收敛
   - 第二个业务模块接入后的共享 narrator 去模块化污染复核
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
| 模型输出越界 plan | schema / registry / executor 校验 fail-closed |
| 只有名称但目标查询需要编码 | 模型可先搜索唯一命中后继续执行，不必总是追问编码 |

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
- `modules/cubebox/read_plan_test.go` 或相邻测试：覆盖通用前序结果引用语法与非法引用 fail-closed。
- narrator 校验测试：覆盖误杀减少与内部实现泄露拦截。

### 8.2 服务端测试

- `internal/server/cubebox_api_test.go`：补充同一会话连续追问夹具，使用 stub provider 验证 planner 输入包含 `query_dialogue_context`。
- 验证 `查询飞虫公司的下级组织，只有名称` 可走 `search -> list` 线性编排。
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
6. [ ] 不得把通用前序结果引用扩张成 workflow/DAG/tool platform。
7. [ ] 不得为避免测试困难而放松 schema、降低门禁或扩大 coverage 排除项。
8. [ ] 不得以“模型自主性”为理由放松权限、租户、白名单、参数类型、枚举或只读边界。
9. [ ] 不得继续用普通中文表达禁词、栏目词或业务字段中文名作为 terminal error 条件来限制模型回答风格。

## 10. 交付物

1. [ ] 扩展后的 `QueryContext` 与事件提取测试。
2. [ ] planner 输入包含有限 `query_dialogue_context`。
3. [ ] 通用前序结果引用解析与测试。
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
