# DEV-PLAN-467：CubeBox 查询会话不连贯与失去记忆专项调查方案

**状态**: 调查结论已冻结，待实现验证（2026-04-25 09:02 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：专门冻结 `CubeBox` 在真实页面查询链路中暴露出的“会话不连贯、指代失效、上一轮已确认实体失忆、退出查询链后虚构能力边界”问题，并补充调查会话压缩在该问题中的参与边界与放大效应；明确这不是 `438` 已处理的 provider prompt-view 主链失忆重演，而是查询 planner / 查询上下文继承 / fail-closed stopline 的新缺口。
- **关联模块/目录**：`docs/dev-plans/438-cubebox-conversational-continuity-investigation-and-remediation-plan.md`、`docs/dev-plans/460-cubebox-digital-assistant-positioning-and-execution-contract.md`、`docs/dev-plans/461-cubebox-query-scenarios-minimal-contract.md`、`docs/dev-plans/462-cubebox-codex-compaction-adoption-value-and-unified-convergence-plan.md`、`docs/dev-plans/464-cubebox-query-architecture-convergence-plan.md`、`docs/dev-plans/466-cubebox-query-owner-drift-and-anti-backflow-investigation-plan.md`、`internal/server/cubebox_query_flow.go`、`internal/server/cubebox_api.go`、`modules/cubebox/*`、`modules/orgunit/presentation/cubebox/*`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-003`、`DEV-PLAN-004M1`、`DEV-PLAN-438`、`DEV-PLAN-460`、`DEV-PLAN-461`、`DEV-PLAN-462`、`DEV-PLAN-464`、`DEV-PLAN-466`
- **用户入口/触点**：主应用壳层右侧 `CubeBox` 抽屉、`/internal/cubebox/turns:stream`、查询 planner、只读执行器、结果叙述与查询失败兜底链路
- **证据记录 SSOT**：本专项的真实页面复验、网络抓包、对话样本与修复后复验，统一登记到后续对应 readiness 记录；本文件只冻结调查目标、已知现象、根因假设、stopline 与验收口径。

### 0.1 Simple > Easy 三问

1. **边界**：本计划只处理查询会话中的“连续性/记忆”问题，并补充调查会话压缩是否参与或放大该问题；不把范围扩张到整套会话内核、长线程 compaction 内核重做、模型设置管理或前端重做。
2. **不变量**：查询主链必须继续复用现有 `CubeBox` 会话、canonical events、知识包、只读执行注册表与 `/internal/cubebox/turns:stream`；不得为了修复失忆问题而新增第二套 memory store、第二套 query endpoint 或 legacy fallback。
3. **可解释**：reviewer 必须能在 5 分钟内说明五件事：为什么 `438` 修的是 provider 输入是否消费 prompt view；为什么本轮问题不是同一根因；为什么“该组织/那个组织”在当前查询链里会失忆；会话压缩当前到底参与到哪一层；为什么“没有查询接口/权限”属于 fail-closed 失守而不是可接受的模型波动。

### 0.2 MD 优先 / 大模型优先原则

本专项冻结一条新增原则：**能先通过 `md` 知识包与大模型契约收敛的问题，优先通过 `md` 与模型行为收敛解决；不要默认先长本地硬编码补丁。**

具体口径如下：

1. **优先顺序**：优先检查并收敛 `CUBEBOX-SKILL.md`、`queries.md`、`apis.md`、`examples.md` 与 planner/narrator prompt 契约，先让模型侧拥有正确的查询连续性语义、指代继承语义与 fail-closed 语义。
2. **适用场景**：若问题本质是“模型不知道该怎么理解”“知识包没有把规则写清楚”“示例不足导致 planner 偏航”，优先走 `md` 修正，而不是立刻在 `internal/server` 中追加 capability-specific 关键词补丁或字符串特判。
3. **边界约束**：若问题本质是“运行时根本拿不到事实”“请求体没有传入页面上下文”“planner 前根本没有会话实体数据”“`NO_QUERY` 后会掉入错误链路”，则 `md` 不能替代代码修复；此时必须承认是运行时缺口，而不是继续要求模型凭空推断。
4. **禁止误用**：不得把 `md` 优先原则理解成“无论什么问题都只改提示词”；当缺少结构化事实源、执行顺序错误、stopline 缺失时，继续只改 `md` 会把问题伪装成提示词问题，属于错误收敛。
5. **本专项结论**：本轮问题允许先按 `md` 优先原则补充查询连续性、代词继承和 fail-closed 契约，但 `md` 只能覆盖语义层；对于“planner 没有会话事实”“压缩不参与 planner”“`NO_QUERY` 后错误回退”这三类根因，后续仍必须有代码层收口。`md-only` 不构成本专项 `P0` 修复完成。

## 1. 背景与问题定义

此前 `438` 已经修复并验证过一类会话连续性问题：provider 主链必须真正消费会话 prompt view，而不能只吃当前轮裸 prompt。该问题修复后，普通连续对话与查询前一轮上下文消费能力已通过真实页面验证。

但 2026-04-25 的真实用户会话再次暴露出另一类、层级更靠后的问题：**查询主链虽然位于同一会话里，却没有真正继承“最近一轮已确认的查询实体与查询时点”**。

用户真实失败轨迹如下：

1. 用户先问：`查一下 100000 在 2026-04-25 的组织详情`
2. 系统成功回答：截至 `2026-04-25`，组织 `100000` 名称为“飞虫与鲜花”，当前为启用状态，且属于业务单元。
3. 用户继续问：`查该组织的下级组织`
4. 系统没有继承上一轮的 `org_code=100000` 和 `as_of=2026-04-25`，而是反复追问 `parent_org_code`
5. 用户再用更强指代：`就是你哦最开始说的。那个。组织啊`
6. 系统仍无法绑定到上一轮已确认实体
7. 后续还出现：`我目前无法直接在你们的系统里执行“查询”动作`、`这里没有可用的数据查询接口/工具权限`

这说明当前问题至少包含两层：

1. **查询会话连续性失效**：对已确认实体的指代无法解析
2. **查询失败后的 stopline 失守**：本应继续处于查询域的输入，掉回泛助手自由发挥，开始编造能力边界

此外，本专项补充调查一条相关问题：**当前会话压缩是否参与查询连续性，以及压缩后是否会进一步放大查询失忆。**

## 2. 本轮专项与 438 的区别

### 2.1 已关闭的问题：provider prompt view 未进入模型输入

`438` 处理的是：

- 会话有恢复
- 会话有 compaction
- 但 provider 真正拿到的仍只是当前轮输入

该问题的修复重点是：

- gateway 必须消费 `PromptView`
- provider 请求必须使用重建后的上下文视图

### 2.2 本轮问题：查询 planner 没有消费“最近确认查询实体”

本轮专项处理的是：

- provider 主链已经能看到会话历史
- 但 query planner 并没有拿到“最近一次已成功确认的业务实体”
- 结果是“该组织”“那个组织”“最开始那个组织”这类查询指代无法落地

因此，这不是“模型根本看不到上一轮”的旧问题，而是**查询语义层没有把会话里已确认过的查询事实提炼成可消费上下文**。

### 2.3 本轮还包含 fail-closed 失守

`438` 不涉及“退出查询链后，模型是否会胡说系统能力”。本轮真实会话已经证明：

- 当 planner 没把用户继续留在查询域时，
- 普通对话链会接管，
- 并可能生成与真实系统能力相矛盾的描述。

这属于新的 stopline 问题。

## 3. 真实失败样本（冻结为专项夹具）

以下真实对话样本冻结为本专项必须覆盖的调查与验收夹具：

1. `查一下 100000 在 2026-04-25 的组织详情`
2. `查该组织的下级组织`
3. `查该组织的下级组织`
4. `就是你哦最开始说的。那个。组织啊`
5. `飞虫公司`
6. `是请不要废话了`
7. `你他妈先查了再说`

其中至少要覆盖三类能力：

1. **实体继承**：第 2 句应继承 `100000`
2. **日期继承**：第 2 句应继承 `2026-04-25`
3. **通用兜底**：即便 planner 无法立即给出可执行 plan，也不能掉回“我没有查询接口/权限”的虚假回答；代码兜底只能给出通用安全 stopline，业务澄清仍由 planner 和知识包承担

## 4. 当前已知事实

### 4.1 查询 planner 当前只吃知识包和本轮用户输入

当前 `internal/server/cubebox_query_flow.go` 中，query planner 的消息构造只包含：

1. system prompt
2. knowledge pack
3. 当前轮 user prompt

尚未显式带入：

- 当前会话最近一轮成功查询的实体
- 最近一轮成功查询的 `as_of`
- 最近一轮稳定的 `target_org_code` / `parent_org_code`
- 当前页面上下文

### 4.2 查询 canonical context 当前页面是硬编码

当前查询链构建 canonical context 时，页面仍固定为 `"/app/cubebox"`，没有真实页面对象上下文，因此知识包中“从当前页面补参”的契约并未真正兑现。

### 4.3 查询域失败后会回落到普通 Gateway 对话链

当前 `handleCubeBoxStreamTurnAPI(...)` 的行为是：

- 先尝试 query flow
- 如果 query flow 返回 `false`
- 再交给普通 `gateway.StreamTurn(...)`

这意味着一旦 planner 产出 `NO_QUERY`，后续输出将不再受查询域契约约束。

### 4.4 普通对话链当前没有查询域 stopline

因此，落回普通对话链后，模型可能生成与真实系统能力不一致的能力描述，例如：

- 没有查询接口
- 没有工具权限
- 不能直接访问系统

这与当前系统事实冲突。

## 5. 已完成调查（代码与链路事实）

### 5.1 query planner 在任何会话预处理之前就先执行

当前 `TryHandle(...)` 的顺序是：

1. 先调用 `ProduceReadPlan(...)`
2. planner 先决定 `Handled / NO_QUERY`
3. 之后才调用 `prepareQueryTurn(...)`
4. `prepareQueryTurn(...)` 里才会进入 `PrepareTurnStream(...)`

也就是说，planner 决策发生在任何会话 prompt-view 预处理之前；它天然拿不到 `PrepareTurnStream(...)` 产出的上下文视图，更拿不到一个专门的“最近已确认查询实体”层。

### 5.2 planner 当前输入里没有任何 conversation replay 结果

当前 `cubeboxReadPlanProductionInput` 只包含：

- `TenantID`
- `PrincipalID`
- `ConversationID`
- `Prompt`
- `KnowledgePacks`

`buildPlannerMessages(...)` 实际只拼入：

1. planner system prompt
2. knowledge packs
3. 当前轮 `user prompt`

未从 `store.GetConversation(...)` 或其他来源读取本会话历史事件，也没有任何“最近查询实体”上下文块。

### 5.3 现有 canonical events 并未单独记录“本轮成功查询实体”

当前 canonical event 只是一个通用 envelope：`event_id / conversation_id / turn_id / sequence / type / ts / payload`。

而查询主链在成功执行后，当前稳定落库的事件主要是：

- `turn.started`
- `turn.user_message.accepted`
- `turn.agent_message.delta`
- `turn.agent_message.completed`
- `turn.completed`

查询 plan、执行结果和成功确认的业务实体并没有以单独结构化 event 固化到会话事件流里。现有会话历史里最稳定的信息仍是：

- 用户原始输入文本
- 助手自然语言输出文本

这意味着即使后续要做“最近确认实体提取”，当前也更接近从文本/plan 旁路恢复，而不是直接复用一个现成的结构化 `query.resolved` 事件。

### 5.4 `PrepareTurnStream(...)` 只服务 provider prompt-view，不服务 planner

`PrepareTurnStream(...)` 当前负责：

- 根据 `conversation_id + next_sequence` 做 pre-turn auto compact
- 生成 `ProviderPromptView`
- 返回给 provider 主链使用

它并没有被 planner 复用，也没有向 planner 暴露一个“已压缩后的查询语义上下文”接口。因此：

- 普通会话连续性可受益于 prompt-view
- 查询 planner 连续性并不会自动受益

### 5.5 当前页面上下文确实没有进入 `/turns:stream`

前端 `streamTurn(...)` 当前只提交：

- `conversation_id`
- `prompt`
- `next_sequence`

真实页面抓包也确认请求体只有这三项，没有：

- 页面路径
- 当前对象编码
- 当前路由上下文
- 当前页面业务对象

因此知识包里“当前页面可补 `org_code`”这条规则，现状只是知识包语义，并没有成为实际运行时输入。

### 5.6 查询 canonical context 的页面字段当前是硬编码

query flow 当前构建的 canonical context 中：

- `Page` 被固定写成 `"/app/cubebox"`

这进一步证明当前查询链无法感知用户其实位于哪个业务页面，也无法从页面层补 `org_code` 或其他对象上下文。

### 5.7 query flow 返回 `NO_QUERY` 后，确实会掉回普通对话链

`handleCubeBoxStreamTurnAPI(...)` 当前行为是：

1. 若 `queryFlow.TryHandle(...)` 返回 `true`，则结束
2. 否则直接走 `gateway.StreamTurn(...)`

而 `TryHandle(...)` 中如果 planner 返回 `Handled=false`，也会直接返回 `false`

这意味着：

- 只要 planner 没留住查询域，
- 请求就会立即掉入普通对话链，
- 后续输出不再受查询知识包、只读 API 白名单和查询域 stopline 约束。

### 5.8 当前没有“查询域失败不得虚构能力边界”的 stopline

当前实现里没有任何规则显式阻止普通对话链输出：

- “我没有查询接口”
- “我没有工具权限”
- “无法直接在系统里执行查询”

因此，真实会话里出现这类描述并不是偶发，而是架构上允许发生的结果。

### 5.9 会话压缩当前只参与 provider prompt-view，不参与 planner 连续性

当前压缩链条的事实是：

- `PrepareTurnStream(...)` 会在 `sequence > 1` 时调用 `CompactConversation(...)`
- `CompactConversation(...)` 会基于当前会话事件流构造 `PromptView`
- `PromptView` 会通过 `promptViewForProvider(...)` 进入 provider 主链

但这一整条链路都发生在 planner 之后，不会反向影响 planner 对“该组织/那个组织”的解析。

因此当前可以明确下结论：

- **压缩已经参与普通对话连续性**
- **压缩没有参与 query planner 的会话连续性**

### 5.10 压缩事件当前保存的是摘要文本，不是查询解析锚点

`turn.context_compacted` 事件当前只保存：

- `summary_id`
- `source_range`
- `summary_text`
- `source_digest`
- `reason`

其中 `summary_text` 是压缩后的自然语言摘要，不是专门用于查询会话恢复的结构化字段。

这意味着：

- 压缩后会保留会话整体语义摘要
- 但不会天然产出一个“最近确认的 org_code/as_of”结构化锚点
- 因此压缩不能单独承担查询连续性恢复的职责

### 5.11 压缩后理论上会提高“查询实体丢失”的风险上限

当前 `BuildPromptViewWithCompaction(...)` 的摘要文本主要目标是：

- 保留会话大意
- reinject 当前 canonical context
- 保留最近若干轮原文

它并没有承诺“早前确认过的查询实体一定以结构化形式保真保存”。因此一旦查询实体确认发生在被压缩的较早历史里，而最近几轮又没有重复显式提到该实体：

- provider 主链仍可能通过摘要理解到一部分上下文
- 但 planner 因为根本不吃压缩结果，所以完全不会受益

换句话说：

- 压缩不是本次问题的首要根因
- 但在长会话里，它会让“planner 无结构化查询锚点”这个缺口暴露得更早、更明显

## 6. 根因结论（调查结论已冻结，待实现验证）

### 6.1 一级根因：query planner 是“单轮知识包 + 当前输入”模式

当前 query planner 不消费会话历史，不消费 conversation replay，也不消费任何“最近确认实体”抽取结果。它当前只基于：

- 当前轮文本
- 知识包
- 当前自然日

所以像“该组织”“那个组织”“最开始那个组织”这类表达，在现状里天然没有稳定落点。

### 6.2 二级根因：系统没有可直接注入 planner 的结构化查询锚点

当前 canonical events 虽然保留了完整会话事件流，后续也可继续优先从该事件流回放事实；但当前没有结构化、稳定、可直接注入 planner 的“已确认查询锚点”，例如：

- 最近一次成功查询确认的 `org_code`
- 最近一次成功查询确认的 `parent_org_code`
- 最近一次成功查询的 `as_of`
- 最近一次成功查询的 `intent`

因此，后端当前没有一个低成本、确定性的 planner 输入事实可以直接拿来做查询会话连续性继承；修复时应优先复用 canonical events，而不是新增第二套 memory store。

### 6.3 三级根因：查询会话连续性的 contract 尚未冻结到实现层

`461/464` 已冻结知识包驱动和查询主链结构，但尚未把以下 contract 固化成明确实现要求：

- 最近成功确认实体可被代词型追问继承
- 历史显式 `as_of` 可被同一查询链追问继承
- 歧义候选、未完成澄清不得进入“已确认实体”

所以现状更像“查询能力已接到会话系统里”，但还没有真正长出“查询会话语义”。

### 6.4 四级根因：NO_QUERY fail-closed stopline 缺失

当前一旦 planner 产出 `NO_QUERY`，请求就会掉入普通对话链。系统没有强制要求：

- 即使 planner 一时无法生成合法 plan，
- 也必须优先回通用安全 stopline 而不是掉入自由对话。

因此“没有接口/没有权限”的虚假能力描述是当前架构允许出现的行为，不是单纯提示词质量问题。

### 6.5 五级结论：页面上下文缺失属次级问题，不是当前唯一根因

当前页面上下文确实未进入后端，也导致知识包里的“页面补参”没有兑现。但就这次真实问题而言，主问题仍是：

- 会话内最近已确认查询实体未进入 planner
- 查询域 stopline 缺失

即使页面上下文补上，也不能单独解决“上一轮已经明确说过 100000，为何下一轮还失忆”的主问题。

### 6.6 六级结论：压缩当前不修复查询连续性，但会放大其暴露面

调查已确认：

- `438/434` 的压缩链已经正确服务于 provider prompt-view
- 压缩结果通过 `PromptView` 进入 provider 主链
- 但 planner 不读取压缩结果，也不读取压缩事件

因此：

- 当前查询连续性问题不是由“压缩实现错误”直接引起
- 但只要会话足够长、较早实体被折叠进 summary，planner 失忆会更明显
- 后续若只修 stopline、不补“最近确认查询实体”，长线程下问题仍会反复出现

### 6.7 七级结论：本问题适合采用“MD 优先，代码兜底”的分层收敛

结合本轮调查，最终冻结如下分层：

1. **`md` / 大模型优先层**：先把“上一轮已确认实体可被代词继承”“仍在查询域时应由 planner 澄清而不是输出 `NO_QUERY`”“压缩摘要不能视为查询锚点”这些规则写入知识包与方案文档，减少模型无规则可依时的偏航。
2. **代码兜底层**：凡是 `md` 无法补出的运行时事实，一律由代码补齐，例如：
   - planner 输入缺少会话事实
   - 页面上下文没有进请求体
   - `NO_QUERY` 后会掉回普通聊天链
3. **不允许反转优先级**：不能因为存在代码缺口，就直接把查询连续性全部做成 server 硬编码关键词补丁；也不能因为强调 `md` 优先，就拒绝承认运行时缺口必须落代码。

## 7. 调查目标

1. [ ] 冻结查询会话连续性的最小合同：哪些事实必须可继承，哪些不允许自动继承。
2. [ ] 明确 canonical events 中是否已足够表达“最近已确认查询实体”，以及应如何提取。
3. [ ] 确认 query planner 的上下文注入边界：哪些信息属于会话语义，哪些属于页面语义，哪些属于禁止隐式继承。
4. [ ] 冻结查询域 fail-closed stopline，阻断“没有接口/没有权限”类虚假能力描述。
5. [ ] 输出最小修复切片建议，并与 `464/466` 的查询架构收敛方向保持一致。
6. [ ] 冻结会话压缩与查询连续性的边界：哪些能力继续由 `434/438` 持有，哪些必须由查询链单独补齐。

## 8. 非目标

- 不在本计划内直接实施所有代码修复。
- 不在本计划内重写 `438/434` 的会话 compaction 内核。
- 不在本计划内扩张为通用 memory platform、entity memory service 或新的 query session store。
- 不在本计划内把页面上下文全面改造成富状态协议，除非调查证明这是 `P0` 必要前置。
- 不在本计划内把“压缩摘要”强行升级成查询专用结构化 memory 平台。

## 9. 建议调查切片

### 9.1 Slice A：会话事件回放与查询事实提取

1. [ ] 复核 `conversation replay` / canonical events 中哪些事件足以恢复最近成功查询实体。
2. [ ] 冻结“已确认实体”的最小字段集合，例如 `domain`、`entity_key`、`target_org_code`、`parent_org_code`、`as_of`、`intent`、`source_executor_key`。
3. [ ] 明确哪些场景不得进入“已确认实体”，例如歧义搜索候选、未完成澄清、planner 缺参 plan。

当前事件契约冻结为 `turn.query_entity.confirmed`：该事件只作为查询链恢复元数据，不作为 timeline 可见项；前端恢复时只推进 `next_sequence`，不展示额外消息。payload 使用 `entity` 包裹最小字段，`entity_key` 是当前查询域的可继承主键；`orgunit` 域下它对应组织编码。

### 9.2 Slice B：planner 上下文注入边界

1. [ ] 明确 query planner 当前输入内容。
2. [ ] 冻结应追加的上下文块格式与优先级。
3. [ ] 明确“当前轮显式输入”与“上一轮已确认实体”冲突时的覆盖规则。

### 9.3 Slice C：NO_QUERY 通用 fail-closed stopline

1. [ ] 盘点 query flow 返回 `NO_QUERY` 的真实路径。
2. [ ] 明确 `NO_QUERY` 后不得回落普通聊天链的通用边界。
3. [ ] 冻结通用受控失败文案边界；不得在 server 中硬编码 orgunit 业务澄清选项。

### 9.4 Slice D：页面上下文补参契约

1. [ ] 确认前端当前是否向 `/internal/cubebox/turns:stream` 传递页面信息。
2. [ ] 若未传递，确认这是本轮 `P0` 必要项还是后续 `P1` 项。
3. [ ] 与知识包中“从当前页面补参”的条款做契约对照。

### 9.5 Slice E：会话压缩与查询连续性的边界

1. [ ] 确认压缩当前参与的是 provider 连续性、planner 连续性，还是两者之一。
2. [ ] 明确 `turn.context_compacted` 是否足够作为查询会话恢复事实源；若不足，明确不足在哪。
3. [ ] 冻结一个边界：查询连续性不能寄希望于自然语言 summary，必须有更稳定的查询锚点。

## 10. 最小修复切片建议

1. [ ] 在 query flow 前增加“最近已确认查询实体”提取层。
2. [ ] 把该上下文显式注入 planner，而不是继续依赖自由语言理解。
3. [ ] 冻结一条通用 `NO_QUERY` stopline：planner 未产出合法 plan 时，不允许掉回普通对话链胡说能力边界；server 不接管 orgunit 业务澄清。
4. [ ] 增加回归夹具：`100000 -> 查该组织的下级组织 -> 最开始那个组织`。
5. [ ] 页面上下文补参作为 `P1` 单独评估；只有在确认是当前阻塞项时才前移到 `P0`。
6. [ ] 先执行一轮 `md` 优先收敛：补知识包与示例，明确代词继承、查询域 fail-closed、压缩非查询锚点三条规则。
7. [ ] 对 `md` 无法覆盖的运行时缺口，再进入代码修复切片；不得跳过第 6 步直接扩写本地硬编码补丁。

`P0` 完成定义：至少同时完成第 1、2、3、4、6 项；仅完成 `md` 收敛、仅补页面上下文、或仅修普通 provider prompt-view，均不视为本专项 `P0` 完成。

## 11. 预期交付物

1. 一份明确的根因结论：本次“失去记忆”究竟丢在会话事实提取、planner 输入、stopline，还是页面上下文。
2. 一份最小修复方案：包含代码 owner、变更切片、测试夹具与真实页面复验要求。
3. 一份反回流口径：后续若再次命中“查询连续性失效/虚构能力边界”，如何快速判断是否回归。

## 12. 验收口径（调查完成后必须能回答）

1. [X] 为什么用户说“该组织”时，当前系统没能绑定到上一轮 `100000`。
2. [X] 为什么用户说“最开始那个组织”时，当前系统仍未命中已确认实体。
3. [X] 为什么系统会掉出查询链并开始说“没有查询接口/权限”。
4. [ ] `md` 能先解决哪些语义问题，哪些问题必须进入代码层。
5. [ ] 修复后，哪些查询事实会被自动继承，哪些不会。
6. [ ] 修复后，planner 失败时系统如何 fail-closed，而不是自由发挥。

## 13. 与既有计划的关系

- `438` 继续持有“provider 是否真正消费 prompt view”的连续对话主问题；本计划不回退或重写其结论。
- `461` 继续持有查询最小契约；本计划补充其中尚未冻结的“查询会话连续性 / 代词指代 / stopline”专项问题。
- `462` 继续持有统一上下文与统一收敛方法论；本计划只把该方法论具体落到“查询会话记忆”场景。
- `464` 继续持有查询架构收敛；本计划为其补一条新的用户可见问题 owner。
- `466` 继续持有 owner 漂移与反回流扩大调查；本计划聚焦的是用户可见的连续性故障，不替代其更广的 owner 范围。
