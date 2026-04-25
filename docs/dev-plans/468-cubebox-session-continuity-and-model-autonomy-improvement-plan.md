# DEV-PLAN-468：CubeBox 同会话连续追问与模型自主性收敛方案

**状态**: 规划中（2026-04-25 13:29 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：承接 `DEV-PLAN-467` 的调查结论，专门处理 `CubeBox` 在同一会话内基本连续追问时的记忆继承、指代解析、澄清表达与查询结果叙述问题，并收敛当前本地代码对大模型语义发挥的过度干预。
- **关联模块/目录**：`docs/dev-plans/438-cubebox-conversational-continuity-investigation-and-remediation-plan.md`、`docs/dev-plans/438a-cubebox-provider-message-role-normalization-and-codex-summary-alignment-plan.md`、`docs/dev-plans/460-cubebox-digital-assistant-positioning-and-execution-contract.md`、`docs/dev-plans/461-cubebox-query-scenarios-minimal-contract.md`、`docs/dev-plans/462-cubebox-codex-compaction-adoption-value-and-unified-convergence-plan.md`、`docs/dev-plans/464-cubebox-query-architecture-convergence-plan.md`、`docs/dev-plans/467-cubebox-query-conversational-continuity-and-memory-loss-investigation-plan.md`、`internal/server/cubebox_query_flow.go`、`modules/cubebox/*`、`modules/orgunit/presentation/cubebox/*`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-003`、`DEV-PLAN-004M1`、`DEV-PLAN-012`、`DEV-PLAN-300`、`DEV-PLAN-301`、`DEV-PLAN-304`、`DEV-PLAN-438`、`DEV-PLAN-460`、`DEV-PLAN-461`、`DEV-PLAN-462`、`DEV-PLAN-464`、`DEV-PLAN-467`
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

当前 `queryNarrationForbiddenPatterns` 包含大量自然语言表达限制，见 `internal/server/cubebox_query_flow.go:105`；`validateQueryNarrationText(...)` 会按这些 regex 拒绝模型回答，见 `internal/server/cubebox_query_flow.go:541`。

需要保留的限制是：不得泄露原始 JSON、内部执行计划、`api_key`、`payload/results` 等实现细节。

不应继续强控的是：禁止普通栏目词、禁止某些中文短语、禁止自然语言里的所有键值感表达。过强 regex 会造成两类副作用：

- 好回答被误杀，导致用户看到“查询结果叙述未通过输出约束校验”
- 模型被迫围绕规避禁词写作，而不是围绕用户问题组织答案

### 2.6 `ReadPlan` 澄清形态偏窄

当前 `ReadPlan` 在 `missing_params` 存在时要求 `clarifying_question` 非空，且 `steps` 必须为空，见 `modules/cubebox/read_plan.go:61`。

该规则能保持执行边界简单，但它也让“可部分解析、只缺一个确认”的情形无法表达候选、上下文解析依据或预期下一步。后续可继续保持 `steps` 为空，但需要给 planner/narrator 一个更清楚的“澄清态”协议，而不是把所有非执行结果都混到 `NO_QUERY` 或固定 stopline。

## 3. 设计目标

1. [ ] 支持同一会话内 3 到 5 轮内的基本连续追问：`该组织`、`那个组织`、`刚才那个`、`最开始那个`、`它`。
2. [ ] 支持实体与日期共同继承：上一轮确认 `org_code=100000`、`as_of=2026-04-25` 后，下一轮“查该组织下级”不得再次追问 `parent_org_code`。
3. [ ] 支持候选选择闭环：当上一轮展示多个候选后，用户用名称、序号或短语选择一个，planner 能消费候选上下文。
4. [ ] 支持模型生成业务澄清问题：领域内缺参由模型按知识包自然追问，不由固定 stopline 取代。
5. [ ] 支持 narrator 消费轻量对话上下文：最终回答能体现“已按刚才的组织继续查询”，但不得编造结果中没有的事实。
6. [ ] 收缩本地表达层约束：只禁止内部实现泄露和原始结构化数据回显，不再用大量 regex 管控普通中文表达风格。
7. [ ] 保持安全边界不变：权限、租户、只读注册表、`ReadPlan` schema、参数校验、执行失败 fail-closed 均不放松。

## 4. 非目标

- 不建设跨会话长期记忆。
- 不引入向量数据库、外部缓存、RAG 知识库或租户文档中台。
- 不让会话 compaction summary 成为查询事实源。
- 不新增第二套查询 API、第二套执行注册表或第二套授权系统。
- 不把 `orgunit` 业务语义重新硬编码到 `internal/server`。
- 不让前端拼 prompt 或承载查询语义。
- 不在本计划内重做整个 CubeBox UI、模型配置 UI 或 provider 网关。

## 5. 关键设计决策

### 5.1 上下文窗口有限、结构化、只服务同一会话

新增或扩展 `QueryContext` 时，只允许从当前 `conversation_id` 的 canonical events 中提取有限窗口：

- 最近 `3-5` 轮用户输入与助手回答摘要
- 最近 `N` 个已确认查询实体
- 最近一次澄清问题
- 最近一次候选列表
- 当前轮指代解析结果

不得读取其他会话，不得引入用户画像，不得把压缩摘要当成可执行查询事实。

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

### 5.3 `NO_QUERY` 从业务澄清中退出

`NO_QUERY` 只应表示“当前请求不是已支持查询域，或不能安全进入查询链”。领域内缺参、歧义、候选选择，应走模型生成的澄清态。

首期允许两种最小实现路径，落地时二选一：

1. **轻量保持 `ReadPlan`**：领域内缺参继续用 `ReadPlan.missing_params + clarifying_question`，但补充知识包与 planner prompt，明确带有最近上下文时不得轻易 `NO_QUERY`。
2. **引入最小 planner outcome**：新增一个很薄的 planner decision envelope，仅区分 `read_plan`、`need_clarification`、`unsupported_domain`，其中 `read_plan` 内部仍复用现有 `ReadPlan`。

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

## 6. 实施切片

### Slice A：扩展查询会话上下文

1. [ ] 将 `modules/cubebox.QueryContext` 从单个 `RecentConfirmedEntity` 扩展为有限窗口结构，至少包含：
   - `RecentConfirmedEntities []QueryEntity`
   - `RecentDialogueTurns []QueryDialogueTurn`
   - `LastClarification *QueryClarification`
   - `RecentCandidates []QueryCandidate`
2. [ ] 保留当前 `RecentConfirmedEntity` 兼容访问器或派生字段，避免一次性大面积改调用方。
3. [ ] `QueryContextFromEvents(...)` 只从当前会话事件流提取，默认最多保留最近 `5` 个对话片段与最近 `5` 个实体/候选。
4. [ ] 单元测试覆盖：
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

1. [ ] 冻结 `NO_QUERY` 的语义：仅用于 unsupported domain 或无法安全进入查询链。
2. [ ] 对领域内缺参、指代歧义、候选选择，使用模型生成的澄清问题。
3. [ ] 如果采用 planner outcome envelope，新增 decode/validate 测试，保持 envelope 只有三态，不扩大为通用 workflow。
4. [ ] 保留服务端 stopline，但只作为安全兜底；不得替代正常业务澄清。
5. [ ] 增加测试：带有最近实体上下文的“查该组织下级”不得走 `NO_QUERY` stopline。

### Slice D：给 narrator 传入轻量对话上下文

1. [ ] 扩展 `cubeboxQueryNarrationInput` 与 `cubeboxQueryNarrationEnvelope`，新增 `DialogueContext` 或 `ResolvedContext`。
2. [ ] narrator prompt 明确：
   - 可以说“我按刚才的组织继续查”
   - 事实结论仍只能来自 `results`
   - 不得把上下文当成额外查询结果
3. [ ] 回答样式不再靠大批 regex 约束，而靠 prompt 与少量安全校验。
4. [ ] 测试覆盖：narrator 接收到 resolved context，且不会泄露内部 JSON。

### Slice E：削弱表达层过度管控

1. [ ] 收缩 `queryNarrationForbiddenPatterns`：
   - 保留：原始 JSON、Markdown 代码块、`api_key`、`payload`、`results`、step id、内部字段路径等实现细节泄露。
   - 移除或改弱：普通中文栏目词、普通“详情如下”、普通自然语言键值表达、业务字段中文名。
2. [ ] 对误杀风险高的 regex 改成更明确的结构化泄露检测。
3. [ ] 对普通风格不再 terminal error；只有内部实现泄露才失败。
4. [ ] 增加回归测试：自然中文列表、短段落、小标题在不泄露内部字段时可通过。

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
| 支持领域缺参 | 由模型自然追问缺少的业务参数 |
| 指代歧义 | 由模型说明有多个可能对象并请用户选择 |
| 不支持领域 | 统一安全 stopline，不编造系统能力 |
| 模型输出越界 plan | schema / registry / executor 校验 fail-closed |

### 7.3 表达层

| 场景 | 期望 |
| --- | --- |
| 普通自然语言列表 | 允许，只要不泄露内部字段 |
| 小标题式回答 | 允许或由 prompt 引导减少，但不因普通中文标题直接失败 |
| 原始 JSON 回显 | 必须拒绝 |
| `api_key` / `payload` / `results` 泄露 | 必须拒绝 |

## 8. 测试与验证

### 8.1 单元测试

- `modules/cubebox/query_entity_test.go` 或相邻职责测试：覆盖扩展后的 `QueryContextFromEvents(...)`。
- `modules/cubebox/read_plan_test.go` 或新增 planner decision 测试：覆盖澄清态/unsupported 态 decode 与 validate。
- narrator 校验测试：覆盖误杀减少与内部实现泄露拦截。

### 8.2 服务端测试

- `internal/server/cubebox_api_test.go`：补充同一会话连续追问夹具，使用 stub provider 验证 planner 输入包含 `query_dialogue_context`。
- 验证 `NO_QUERY` 仅在 unsupported domain 走 stopline。
- 验证查询成功后 metadata event 写入顺序与 payload 裁剪。

### 8.3 真实页面验证

- 使用当前真实 provider 基线：`provider_id=openai-compatible`、`provider_type=codex`、`model_slug=gpt-5.2`。
- 通过主应用壳层右侧 `CubeBox` 抽屉执行 7.1 的关键链路。
- 保存网络请求、canonical event 片段与最终回答样本到 readiness 记录。

### 8.4 必跑门禁

本计划实施涉及 Go 代码与文档时，按 `AGENTS.md` 触发器矩阵执行：

- Go 代码：`go fmt ./... && go vet ./... && make check lint && make test`
- 文档：`make check doc`
- 涉及历史对话面或旧兼容语义：`make check chat-surface-clean`

## 9. Stopline

1. [ ] 不得新增长期记忆、向量库、外部缓存或跨会话用户画像。
2. [ ] 不得让 compaction summary 成为查询事实源；最多作为 provider prompt-view 的叙事上下文。
3. [ ] 不得绕过 `ExecutionRegistry`、只读 API、租户/权限校验或现有参数校验。
4. [ ] 不得在 `internal/server` 继续新增 `orgunit` 关键词补丁来“修”某一句用户输入。
5. [ ] 不得通过前端拼 prompt 解决连续追问。
6. [ ] 不得把 planner outcome 扩张成 workflow/DAG/tool platform。
7. [ ] 不得为避免测试困难而放松 schema、降低门禁或扩大 coverage 排除项。

## 10. 交付物

1. [ ] 扩展后的 `QueryContext` 与事件提取测试。
2. [ ] planner 输入包含有限 `query_dialogue_context`。
3. [ ] `NO_QUERY` 与澄清态边界收敛。
4. [ ] narrator 输入包含轻量对话上下文。
5. [ ] `queryNarrationForbiddenPatterns` 收缩到内部实现泄露防线。
6. [ ] metadata event 支撑候选、澄清和解析来源。
7. [ ] 同会话连续追问服务端测试。
8. [ ] 真实页面复验证据记录。
9. [ ] `AGENTS.md` Doc Map 已登记本计划。

## 11. 当前阶段执行记录

1. [X] 新建本专项计划文档：`docs/dev-plans/468-cubebox-session-continuity-and-model-autonomy-improvement-plan.md`（2026-04-25 13:29 CST）
2. [X] 更新 `AGENTS.md` 文档地图。（2026-04-25 13:29 CST）
3. [X] 执行 `make check doc` 并记录结果：`[doc] OK`。（2026-04-25 13:29 CST）
