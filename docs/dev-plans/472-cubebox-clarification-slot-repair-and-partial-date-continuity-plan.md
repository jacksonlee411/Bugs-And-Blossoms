# DEV-PLAN-472：CubeBox 模型主导的澄清续接与残缺日期连续性修复方案

**状态**: 实施中（2026-04-27；模型 owner 方向继续保留，prompt-facing 上下文形态由 `DEV-PLAN-473` 统一收敛为 `query_evidence_window.open_clarification`）

> 纠偏说明：本计划早期文本使用 `query_dialogue_context` / `clarification_resume` 描述澄清续接事实块。`DEV-PLAN-473` 已将 CubeBox 查询链的 prompt-facing 上下文统一纠偏为中性的 `query_evidence_window`：上一轮 open clarification 应通过 `query_evidence_window.open_clarification` 提供给模型，`recent_*` 与 `clarification_resume` 不再作为新的主语义输入或 target 绑定来源。本文中保留的旧字段名仅作为历史设计语义说明；后续实现以 `DEV-PLAN-473` 为准。

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：专门处理 `CubeBox` 在同一 `conversation_id` 内进入澄清后，用户用短补充答复继续提供缺失信息时，系统无法让模型把该短答续接到上一轮澄清的问题；首期修复“残缺日期答复”“候选澄清后短答”和“本月 N 日”被误判为新缺参的问题。
- **关联模块/目录**：`internal/server/cubebox_query_flow.go`、`modules/cubebox/*`、`modules/orgunit/presentation/cubebox/*`、`internal/server/cubebox_api_test.go`、`internal/server/cubebox_query_flow_test.go`
- **关联计划/标准**：`AGENTS.md`、`docs/dev-plans/000-docs-format.md`、`docs/dev-plans/003-simple-not-easy-review-guide.md`、`docs/dev-plans/012-ci-quality-gates.md`、`docs/dev-plans/460-cubebox-digital-assistant-positioning-and-execution-contract.md`、`docs/dev-plans/461-cubebox-query-scenarios-minimal-contract.md`、`docs/dev-plans/464-cubebox-query-architecture-convergence-plan.md`、`docs/archive/dev-plans/467-cubebox-query-conversational-continuity-and-memory-loss-investigation-plan.md`、`docs/dev-plans/468-cubebox-session-continuity-and-model-autonomy-improvement-plan.md`、`docs/dev-plans/468c-cubebox-query-context-fact-window-plan.md`、`docs/dev-plans/471-cubebox-intra-turn-iterative-read-planning-plan.md`
- **用户入口/触点**：主应用壳层右侧 `CubeBox` 抽屉、`/internal/cubebox/turns:stream`、查询 planner、查询澄清链路、查询会话上下文提取链路

### 0.1 Simple > Easy 三问

1. **边界**：本计划只处理同一查询会话内“上一轮已向用户追问，本轮用户用短答继续”的澄清恢复问题；不建设本地 NLU、slot repair engine、跨会话记忆、页面事实补参、第二套 planner 状态机或第二套查询 endpoint。
2. **不变量**：查询执行边界、`ReadPlan` schema、tenant/session/principal、只读执行注册表、日期最终 canonical 形态 `YYYY-MM-DD`、fail-closed 参数校验均不放松；模型可以理解短答，但本地代码不能替模型猜业务事实。
3. **可解释**：reviewer 必须能在 5 分钟内说明三件事：上一轮澄清事实如何被提供给模型；模型如何基于该事实输出新的 `ReadPlan` 或继续澄清；本地代码只在哪里做 schema、日期合法性与执行边界校验。

### 0.2 对原 472 草案的批判结论

原草案方向是“新增待补槽位状态 + 本地 partial date repair helper + repair 成功事件”。这个方向能快速修一个样例，但会把 CubeBox 拉回 `DEV-PLAN-464` 明确反对的形态：模型产出草案，server 再做第二轮业务理解、默认值补丁和澄清改写。

具体问题如下：

1. **owner 漂移**：`1日` 是否回答上一轮“2025年1月缺具体日期”、`全部` 是否回答上一轮候选澄清，是语义理解问题，应由模型在上下文中判断。本地 parser 一旦承担这个判断，就重新变成隐形 NLU。
2. **局部修补会扩张成 slot engine**：首期看似只修 `partial_date`，但同类短答还包括候选序号、`全部`、`这个`、`另一个`、否定后重选、布尔确认、名称补答。若继续本地编码，会快速长出第二套澄清状态机。
3. **事实供给与语义判断混淆**：代码可以提供上一轮 `missing_params`、候选组、澄清问题、最近问答和当前用户原文；但不应在 planner 之前生成 `resolved_params` 来替模型决定“本轮就是补齐 as_of”。
4. **日期本地解析是错层复杂度**：中文残缺日期的解析不是安全边界，而是语言理解。安全边界是最终 `as_of` 必须为合法 `YYYY-MM-DD`；该校验应留在本地，解析与继承应交给模型。
5. **canonical event 风险**：若写入 `turn.query_clarification.repaired` 并把本地 repair 结果当事实，会制造新的执行事实源。后续若模型、本地 parser、执行校验三者不一致，排查会变复杂。
6. **与 471 的 loop 方向不一致**：`全部` 这类候选集合答复不该在 472 中新增本地集合执行逻辑；它应被作为澄清续接事实交给 planner，后续是否 fanout 由 471 的只读 loop 与执行预算处理。

因此，472 的修正方向不是“把本地 repair 写得更完善”，而是：

> 本地只把 open clarification 和当前短答以结构化、可审计、有限上下文提供给 planner；模型负责绑定、补全、消歧和决定继续执行还是继续澄清；本地只做 schema、合法日期、白名单、租户、权限和预算校验。

### 0.3 现状研究摘要

- query flow 已能把 `last_clarification`、`recent_dialogue_turns`、`recent_confirmed_entities`、候选组等事实窗口注入 planner，但这些事实还没有被明确标注为“当前用户输入可能是在回答上一轮澄清”。
- `orgunit` 知识包要求 `as_of` 最终必须是 `YYYY-MM-DD`；这应继续作为输出校验和知识包约束，而不是触发本地中文日期 parser。
- 当前最容易出错的位置不是“缺少一个日期补全函数”，而是 planner 输入没有把上一轮澄清、本轮短答、候选组和最近问答组织成一个清晰的 `clarification_resume` 事实块。
- 本次不再沿用的容易做法：不新增本地 slot repair helper；不新增 `as_of_day` 之类本地子槽位；不在 `internal/server` 用关键词判断 `1日`、`全部`；不把 repair 结果写成独立业务事实源。

## 1. 背景与上下文

真实失败样本已经明确暴露出当前缺口：

1. 用户：`查出顶级点的全部各级下级组织，时间节点是2025年1月`
2. 系统：要求提供具体日期
3. 用户：`1日`
4. 系统：没有把 `1日` 解释为对上一轮日期澄清的补充，而是再次追问“你说的 1 日是指哪一天”，甚至重新追问查询目标类型

另一个同类失败样本说明，问题不只体现在日期片段，也体现在澄清后“集合型补答”被误判成新问题：

1. 用户：`列出全部财务组织的详情`
2. 系统：返回多个候选组织，并追问“想查看哪一个的详情”，给出：
   - `200001 财务部`
   - `200002 财务一组`
   - `200004 财务四组`
3. 用户：`全部`
4. 系统：没有把 `全部` 理解为“对上一轮候选澄清的集合型回答”，而是重新改问“你说的‘全部’是要查哪一类组织信息”

这说明当前系统虽然已经具备：

- 会话级 `query_dialogue_context`
- 最近澄清事实 `last_clarification`
- 最近问答 `recent_dialogue_turns`
- 候选组与最近确认实体

但仍缺少一个模型可直接消费的澄清恢复入口：

1. **当前输入身份不清**：planner 看到 `1日` 或 `全部`，却没有强提示“这可能是在回答上一轮澄清”。
2. **上一轮澄清事实不成组**：`missing_params`、澄清问题、候选组、上一轮用户原文散落在上下文里，模型需要自己拼关系。
3. **本地错误倾向**：若用代码先解析 `1日`，短期能修日期，长期会把候选、代词、否定重选等同类理解继续压成本地规则。

如果不修这一层，`CubeBox` 即使保留更多历史问答，也仍会在澄清后续问时显得“像失忆”。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 冻结“模型主导澄清续接”运行时最小契约：上一轮 open clarification、候选组、最近问答、当前用户原文如何进入 planner。
- [ ] 让 query flow 在存在 open clarification 时，优先把当前输入作为 `clarification_reply_candidate` 提供给模型，而不是在本地解析或直接重判为新问题。
- [ ] 让模型在看到 `2025年1月` 的上一轮上下文和当前 `1日` 时，输出包含 `as_of=2025-01-01` 的合法 `ReadPlan`；本地只校验日期合法性和执行边界。
- [ ] 让模型在看到上一轮候选组和当前 `全部` / `以上全部` 时，判断这是候选澄清答复；能执行则继续输出合法计划，不能执行则继续澄清或受控失败，不得跳回无关入口级选项。
- [ ] 让模型在用户直接说 `本月9日` / `这个月9号` 时，基于 planner system prompt 的当前自然日年月输出完整 `YYYY-MM-DD`，不得继续要求用户提供完整日期。
- [ ] 保持 fail-closed：无法稳定理解、日期不合法、候选集合无法在预算内执行或 `ReadPlan` 不合法时，继续澄清或返回受控边界错误，不得静默猜测。
- [ ] 冻结测试夹具与回归口径，覆盖真实失败样本以及同类短答场景。

### 2.2 非目标

- 不建设本地中文日期解析器、通用 slot repair engine 或关键词 NLU。
- 不新增 `as_of_day`、`candidate_all` 这类为了本地 repair 产生的执行参数。
- 不建设跨会话长期记忆。
- 不引入页面事实、URL 参数或前端当前对象作为补参来源。
- 不扩展到写链路、proposal/commit 链路或普通闲聊链路。
- 不在 472 中新增候选集合 fanout 的本地业务执行逻辑；集合执行能力若不足，由 `DEV-PLAN-471` 的只读 loop 和预算继续承接。
- 不把知识包改造成第二套 slot schema 平台；知识包只说明业务语义、字段约束和澄清规则。

### 2.3 用户可见性交付

- **用户可见入口**：现有 `CubeBox` 抽屉，不新增 UI 入口。
- **最小可操作闭环**：用户在同一会话中被追问日期后，只需继续回答 `1日`、`1号`、`那就1日`、`就1号` 这类短补充，系统应由模型续接原查询，而不是再次问“哪一天”或重新问“你想查什么”。
- **验收样例**：`时间节点是2025年1月` -> `1日` -> planner 输出合法 `ReadPlan`，执行时使用 `2025-01-01`。

## 2.4 工具链与门禁（SSOT 引用）

- **命中触发器（勾选）**：
  - [ ] Go 代码
  - [ ] 文档 / readiness / 证据记录
  - [ ] 其他专项门禁：`chat-surface-clean`、`error-message`

- **本次引用的 SSOT**：
  - `AGENTS.md`
  - `docs/dev-plans/000-docs-format.md`
  - `docs/dev-plans/003-simple-not-easy-review-guide.md`
  - `docs/dev-plans/012-ci-quality-gates.md`
  - `docs/dev-plans/460-cubebox-digital-assistant-positioning-and-execution-contract.md`
  - `docs/dev-plans/461-cubebox-query-scenarios-minimal-contract.md`
  - `docs/dev-plans/464-cubebox-query-architecture-convergence-plan.md`
  - `docs/archive/dev-plans/467-cubebox-query-conversational-continuity-and-memory-loss-investigation-plan.md`
  - `docs/dev-plans/468-cubebox-session-continuity-and-model-autonomy-improvement-plan.md`
  - `docs/dev-plans/468c-cubebox-query-context-fact-window-plan.md`
  - `docs/dev-plans/471-cubebox-intra-turn-iterative-read-planning-plan.md`
  - `Makefile`

## 2.5 测试设计与分层

| 层级 | 本计划承接内容 | 代表对象/文件 | 说明 |
| --- | --- | --- | --- |
| `modules/cubebox` | `query_dialogue_context` / planner 输入中的 `clarification_resume` 投影、open clarification 提取、planner outcome 校验 | `modules/cubebox/*_test.go` | 测上下文事实装配和 schema 校验，不测本地日期解析 |
| `internal/server` | query flow 如何在 planner 前把当前短答标记为澄清答复候选，并如何消费模型输出的 `ReadPlan` / `CLARIFY` | `internal/server/cubebox_query_flow_test.go`、`internal/server/cubebox_api_test.go` | 用 stub planner 验证流转和事件写入 |
| `knowledge pack` | 模型应如何使用 `clarification_resume`、残缺日期上下文和候选组 | `modules/orgunit/presentation/cubebox/*` | 补规则和少量样例，不做 prose 模板库 |
| `E2E` | 本计划不要求首期新增 E2E；若后续补 browser 真实复验，记入 readiness | `docs/dev-records/` | 当前以后端链路与真实对话夹具为主 |

- **黑盒 / 白盒策略**：
  - planner 输入投影、outcome 解析和 query flow 适配优先黑盒。
  - 不新增 `*_coverage_test.go`、`*_gap_test.go`、`*_more_test.go`、`*_extra_test.go` 一类补洞式测试。

- **并行 / 全局状态策略**：
  - 纯投影测试可并行。
  - 涉及时间源、会话事件流或共享 stub 状态的测试不得无脑并行。

- **fuzz / benchmark 适用性**：
  - 本计划不新增本地自然语言日期 parser，因此不需要为日期解析补 fuzz。
  - 若后续引入任何本地自然语言解析逻辑，必须先回写本计划并重新评审 owner 边界。

## 3. 架构与关键决策

### 3.1 5 分钟主流程

```mermaid
flowchart LR
  U[用户短答] --> Q[query flow]
  Q --> C[读取 query_dialogue_context]
  C --> R[构造 clarification_resume]
  R --> P[planner / LLM]
  P --> V[本地 schema 与边界校验]
  V --> E[只读执行 / 继续澄清]
```

- **主流程叙事**：
  1. 先从 canonical events 读取 `query_dialogue_context`。
  2. 若上一轮存在 open clarification，则构造 `clarification_resume`，包含上一轮澄清问题、`missing_params`、候选组、最近问答、当前用户原文和 `reply_candidate=true`。
  3. 本地不解析 `1日`、`全部`，也不生成 `resolved_params`；planner 基于上下文决定当前短答是否回答上一轮澄清。
  4. planner 输出合法 `ReadPlan`、`CLARIFY`、`NO_QUERY` 或 471 已冻结的 outcome。
  5. 本地只做 schema、日期合法性、api 白名单、租户、权限、预算和执行校验。
  6. 若 planner 输出合法计划，则继续只读执行；若仍缺参或不确定，则继续澄清。

- **失败路径叙事**：
  - 无 open clarification：按现有 planner 流程处理。
  - 有 open clarification 但模型判断当前输入是新问题：允许输出新的合法 `ReadPlan`，但不得由本地关键词判断抢先改写。
  - 模型输出非法日期或非法参数：本地 fail-closed，继续澄清或返回受控边界错误。
  - 模型把候选集合答复转成超过预算的计划：由 471 预算和执行校验拦截。

- **恢复叙事**：
  - 澄清续接成功后，不写入“本地 repaired 事实”；写入正常的 planner/执行/确认类 canonical events。
  - 若需要审计，可记录 `turn.query_clarification.resume_attempted` 这类输入事实事件，但 payload 只能表达“给了模型哪些上下文”，不得表达本地已解析业务结论。

### 3.2 模块归属与职责边界

- **owner module**：`modules/cubebox` 持有 `clarification_resume` 投影、planner outcome 解析和 query context 补充结构；`internal/server` 只承接 query flow 编排与事件接线。
- **模型 owner**：短答绑定、残缺日期补全、候选集合语义、继续澄清措辞、是否继承上一轮语义。
- **本地 owner**：上下文供给、schema 校验、合法日期校验、执行注册表、权限/租户/session/principal、预算、事件落库。
- **跨模块交互方式**：仍通过 `query_context`、planner prompt、已登记执行器交互；不引入跨模块专用 repair 接口。
- **组合根落点**：query flow 继续在 `internal/server/cubebox_query_flow.go` 组合；上下文投影和 planner outcome 逻辑尽量下沉到 `modules/cubebox`。

### 3.3 ADR 摘要

- **决策 1**：用 `clarification_resume` 事实块喂给模型，而不是新增本地 slot repair 层
  - **备选 A**：继续只靠散落的最近问答和 `last_clarification`
  - **备选 B**：新增本地 partial date repair helper
  - **选定理由**：当前缺口是模型输入没有明确组织澄清恢复上下文，而不是缺少本地日期 parser；模型更适合做短答绑定和残缺日期理解，本地只应校验最终结果。

- **决策 2**：不新增 `as_of_day` / `repair_kind` 作为本地执行事实
  - **备选 A**：把日期子槽位建成本地 runtime 字段
  - **备选 B**：由模型直接输出完整 `as_of`
  - **选定理由**：执行契约只接受完整 `as_of`，本地子槽位会形成第二套参数真相。若上一轮模型需要表达已知部分参数，可作为 `clarification_context.known_params` 提供给下一轮模型，而不是作为执行参数。

- **决策 3**：澄清恢复成功不写本地 `repaired` 业务事实
  - **备选 A**：写 `turn.query_clarification.repaired`，记录本地解析结果
  - **备选 B**：只写正常 planner/执行事件，可选记录 resume 输入事实
  - **选定理由**：本地 repair 结果不是事实源。真正可执行事实是通过模型输出并通过本地校验的 `ReadPlan` 及执行结果。

### 3.4 Simple > Easy 自评

- **这次保持简单的关键点**：不把短答理解做成本地 NLU；不新增执行参数；不新增第二状态机；只强化模型输入事实块和输出校验。
- **明确拒绝的“容易做法”**：
  - [X] legacy alias / 双链路 / fallback
  - [X] 前端自造第二套澄清状态机
  - [X] 在 `internal/server` 堆 `1日`、`全部`、`第一个` 等关键词补丁
  - [X] 本地 partial date parser / slot repair engine
  - [X] 用 `turn.query_clarification.repaired` 固化本地语义判断

## 4. 数据模型、状态模型与约束

### 4.1 新增/扩展的结构化事实

本计划冻结新增 planner 输入事实块，命名可在实现时微调，但语义必须稳定：

- `clarification_resume`
  - `reply_candidate`
  - `source_turn_id`
  - `intent`
  - `missing_params`
  - `clarifying_question`
  - `known_params`
  - `candidate_group_id`
  - `candidate_source`
  - `candidate_count`
  - `cannot_silent_select`
  - `candidates`
  - `recent_dialogue_turns`
  - `raw_user_reply`

其中：

- `reply_candidate=true`：只表示“当前输入可能是在回答上一轮澄清”，不是本地已确认。
- `known_params`：只能来自上一轮 planner/clarification 已显式输出或已确认的上下文事实；不得由本地自然语言 parser 从 prose 中抽取。
- `raw_user_reply`：当前用户原文，供模型理解短答。
- `candidate_group_id` / `candidate_source` / `candidate_count` / `cannot_silent_select`：复用 468C 已冻结的候选澄清事实，帮助模型处理 `全部`、序号和候选重选。
- `candidates`：上一轮澄清绑定的候选实体列表，由 `candidate_group_id` 关联最近候选组后投影；只作为 planner 输入事实，不代表本地已替用户选择。

### 4.2 时间语义与标识语义

- **Valid Time**：最终业务查询时点仍统一为 `date`，格式 `YYYY-MM-DD`。
- **残缺日期理解 owner**：模型基于 `clarification_resume` 和知识包理解 `2025年1月` + `1日`；本地不保存 `as_of.year/month/day` 这类子槽位作为执行事实。
- **本地校验 owner**：
  - `as_of` 必须是合法 `YYYY-MM-DD`。
  - 非法日期如 `2025-02-31` 必须 fail-closed。
  - 缺少年/月/日时不得进入执行层。
- **关键约束**：
  - 不得从当前系统日期猜测缺失年份或月份。
  - 不得从普通 assistant prose、页面对象或 unrelated candidate 中抽取日期事实。
  - 若模型输出与上一轮上下文冲突，且不能用当前轮显式输入解释，必须继续澄清或 fail-closed。

### 4.3 QueryContext 扩展契约

在现有 `recent_confirmed_entities`、`recent_dialogue_turns`、`last_clarification`、候选组之外，本计划新增或扩展：

- 最近一次 open clarification 的 `clarification_resume` 视图
- 当前用户原文作为 `raw_user_reply`
- 当前轮是否存在 `reply_candidate` 的布尔信号

关键约束：

1. 这些字段只服务同一 `conversation_id` 内的短程连续追问。
2. 这些字段不是授权来源，不是执行结果事实。
3. projection helper 只能裁剪数量、关联候选组，不得解析自然语言或改写业务参数。

## 5. 设计方案

### 5.1 问题分层

本专项把现象拆成四层：

1. **事实层**：上一轮澄清、候选组、最近问答和当前用户原文是否被完整提供给模型？
2. **模型理解层**：模型是否把当前短答绑定到上一轮澄清，并输出合法计划或继续澄清？
3. **本地护栏层**：schema、日期合法性、白名单、权限、租户、预算是否拦住非法输出？
4. **event 层**：成功执行后如何通过既有 canonical events 形成后续可继承事实？

### 5.2 `clarification_resume` 输入块

当上一轮存在 open clarification 时，planner 输入中应增加类似结构：

```json
{
  "clarification_resume": {
    "reply_candidate": true,
    "source_turn_id": "turn_prev",
    "intent": "orgunit.list",
    "missing_params": ["as_of"],
    "clarifying_question": "请提供完整查询日期，例如 2025-01-01。",
    "known_params": {
      "as_of_text": "2025年1月",
      "query_scope": "root_descendants"
    },
    "raw_user_reply": "1日"
  }
}
```

说明：

1. `known_params.as_of_text` 是上一轮模型/上下文已经显式保留下来的事实文本，不是本地 parser 生成的 `year/month`。
2. planner 可以据此输出完整 `as_of=2025-01-01` 的 `ReadPlan`。
3. 本地只校验输出日期合法，不负责把 `1日` 拼成 `2025-01-01`。
4. 若无法稳定理解，planner 应输出 `CLARIFY`，不得输出 `NO_QUERY` 或无关入口级问题。

候选澄清场景应增加类似结构：

```json
{
  "clarification_resume": {
    "reply_candidate": true,
    "source_turn_id": "turn_prev",
    "intent": "orgunit.details",
    "missing_params": ["org_code"],
    "candidate_group_id": "cg_prev",
    "candidate_source": "execution_error",
    "candidate_count": 3,
    "cannot_silent_select": true,
    "candidates": [
      {"domain": "orgunit", "entity_key": "200001", "name": "财务部", "as_of": "2026-04-25"},
      {"domain": "orgunit", "entity_key": "200002", "name": "财务一组", "as_of": "2026-04-25"},
      {"domain": "orgunit", "entity_key": "200004", "name": "财务四组", "as_of": "2026-04-25"}
    ],
    "raw_user_reply": "全部"
  }
}
```

planner 负责判断 `全部` 是否表示候选集合；若能在 471 loop 预算内执行，则继续输出合法小计划；否则继续澄清或返回受控限制说明。

### 5.3 残缺日期处理规则

本计划冻结的是模型规则和本地校验规则，不冻结本地解析规则。

模型应理解的正向样例：

1. 上一轮用户说 `2025年1月`，系统追问具体日期，本轮用户答 `1日` / `1号` / `那就1日`
   - 期望模型输出 `as_of=2025-01-01`
2. 上一轮用户说 `2025年`，系统追问具体日期，本轮用户答 `1月1日`
   - 期望模型输出 `as_of=2025-01-01`
3. 本轮用户直接给完整 `2025-01-01`
   - 期望模型显式使用当前轮日期

必须继续澄清或 fail-closed 的场景：

1. 只给 `1日`，但上下文没有稳定年月
2. 只给 `1月`，上下文没有稳定年份
3. 模型输出非法日期，例如 `2025-02-31`
4. 当前轮与上一轮上下文冲突，且不是明确改写

### 5.4 澄清恢复优先级

当上一轮存在 open clarification 时，当前轮处理顺序冻结为：

1. 本地构造 `clarification_resume`，不做语义分类。
2. planner 基于 `clarification_resume` 判断当前输入是澄清答复还是新问题。
3. 若 planner 输出合法 `ReadPlan`，本地校验并执行。
4. 若 planner 输出 `CLARIFY`，继续澄清同一问题或更准确的问题。
5. 若 planner 输出 `NO_QUERY`，只有在模型明确判断当前输入脱离支持查询域时才允许；不得把领域内缺参短答误判成 no-query guidance。

### 5.5 planner contract 扩展

planner contract 需要补充明确规则：

- 如果存在 `clarification_resume.reply_candidate=true`，先判断当前 `raw_user_reply` 是否回答上一轮澄清。
- 对日期短答，优先结合 `clarification_resume.known_params`、最近问答和知识包输出完整 `YYYY-MM-DD`；不能稳定补齐时输出 `CLARIFY`。
- 对候选短答，优先结合 `clarification_resume.candidates`、`candidate_group_id` 和候选组判断 `全部`、`以上全部`、序号、名称、否定重选等语义；不能稳定判断时输出 `CLARIFY`。
- 对 `本月N日` / `这个月N号`，基于 planner system prompt 给出的当前自然日年月补齐完整 `YYYY-MM-DD`；不能再要求用户把相对日期改写成完整日期。
- 不得因为当前输入很短就输出 `NO_QUERY` 或重新列入口级选项。
- 最终执行参数必须是 `ReadPlan` schema 接受的完整参数；不得输出 `as_of_day`、`candidate_all` 等本地临时槽位。

### 5.6 canonical event 写回

澄清续接成功后，不新增本地 `repaired` 业务事实事件。系统继续写既有事件：

- planner outcome / read plan 事件
- query execution 事件
- query entity confirmed / candidate group / clarification requested 事件

若实现确实需要可观测性，可新增输入事实事件：

- 可选事件名示例：`turn.query_clarification.resume_attempted`
- 最少 payload：
  - `source_clarification_turn_id`
  - `missing_params`
  - `candidate_group_id`
  - `raw_user_reply`

该事件只能表示“本轮把哪些澄清上下文交给模型”，不得包含本地生成的 `resolved_params`。

## 6. 安全边界与 Stopline

1. [ ] 本地不得实现中文残缺日期 parser、候选短答 parser 或通用 slot repair engine。
2. [ ] planner 输出的最终日期必须通过合法 `YYYY-MM-DD` 校验；非法日期不得进入执行层。
3. [ ] planner 输出不得越过 `ReadPlan` 参数校验；最终进入执行层的仍必须是合法 canonical 参数。
4. [ ] 无法稳定理解短答时继续澄清，不得静默继续执行。
5. [ ] 不得因为模型续接成功就跳过 tenant/session/principal 或执行注册表边界。
6. [ ] 不得让普通自然语言摘要、历史 assistant prose、页面对象或 unrelated candidate 成为日期补全事实源。
7. [ ] 不得将 472 做成 orgunit 专用硬编码分支；模块知识只进入知识包与 planner 输入。
8. [ ] `全部` 的候选集合执行不得绕开 471 loop 预算、执行步数、重复查询检测和注册表校验。

## 7. 实施步骤

1. [ ] 删除原草案中的本地 `slot repair helper` / `partial_date parser` 方向，冻结“模型主导澄清续接” owner 边界。
2. [ ] 扩展 `QueryContext` / planner projection，新增 `clarification_resume` 输入块。
3. [ ] 在 query flow 中加入“存在 open clarification 时构造 `reply_candidate=true`”的 planner 前置上下文装配；不得在此处解析 `1日`、`全部`。
4. [ ] 扩展 planner contract/system prompt：明确短答应优先按上一轮澄清理解，领域内缺参继续 `CLARIFY`，不得误走 `NO_QUERY`。
5. [ ] 更新 `modules/orgunit/presentation/cubebox/CUBEBOX-SKILL.md`、`queries.md`、`examples.md`，补充残缺日期短答和候选集合短答样例；只写语义规则和 `ReadPlan` 例子，不写本地模板。
6. [ ] 确认 `ReadPlan` / executor 层已有合法日期校验；若缺失，只补 canonical 日期校验，不补自然语言解析。
7. [ ] 可选新增 `turn.query_clarification.resume_attempted`，仅记录输入事实；若无明确排查收益，可不新增事件。
8. [ ] 补齐测试：
   - `2025年1月 -> 1日`：stub planner 看到 `clarification_resume` 后输出 `as_of=2025-01-01`，query flow 执行原查询。
   - `2025年 -> 1月1日`：同上。
   - 无稳定年月时仅输入 `1日`：stub planner 输出 `CLARIFY`，query flow 不执行。
   - 模型输出非法日期：本地校验拦截。
   - 候选澄清后仅输入 `全部` / `以上全部`：planner 输入包含候选组、候选实体列表和 `raw_user_reply`，不得被本地重判成入口级新问题。
   - 直接输入 `全部财务组织本月9日的详情`：planner prompt 包含当前自然日月内日期规则，知识包样例要求解析到当前年月的 `YYYY-MM-DD`。
   - 无 open clarification 时的短输入：保持现有普通 planner 路径。
9. [ ] 将真实对话 `conv_8856c2e507c74a229a7c3fbb325e72c3` 的最后两轮抽象为固定验收夹具。
10. [ ] 执行门禁并登记 readiness 证据。

## 8. 验收场景

### 8.1 本次真实失败夹具

1. [ ] 用户：`查出顶级点的全部各级下级组织，时间节点是2025年1月`
2. [ ] 系统：追问具体日期
3. [ ] 用户：`1日`
4. [ ] 期望：planner 基于 `clarification_resume` 输出使用 `2025-01-01` 的合法 `ReadPlan`；不得再次问“1日是指哪一天”，也不得重新问查询类型。
5. [ ] 用户：`列出全部财务组织的详情`
6. [ ] 系统：返回多个候选组织并追问要查看哪一个的详情：
   - `200001 财务部`
   - `200002 财务一组`
   - `200004 财务四组`
7. [ ] 用户：`全部`
8. [ ] 期望：planner 输入中存在上一轮候选组和 `raw_user_reply=全部`；模型将其解释为对候选澄清的集合型回答，继续围绕这 3 个候选组织推进或在预算不足时继续澄清；不得跳回无关的“一级组织 / 最近确认组织 / 直接下级组织 / 搜索组织”入口级重判。
9. [ ] 用户：`查询全部财务组织本月9日的详情`
10. [ ] 期望：planner 将 `本月9日` 按当前自然日所在年月补齐为完整日期；若当前自然日为 `2026-04-25`，首轮查询参数应使用 `as_of=2026-04-09`，不得再次追问“请给完整日期”。

### 8.2 正向场景

1. [ ] `查华东组织在2025年1月的详情` -> `1日`
   - 期望：planner 输出 `as_of=2025-01-01` 并继续查详情
2. [ ] `查组织树，时间是2025年` -> `1月1日`
   - 期望：planner 输出 `as_of=2025-01-01` 并继续查询
3. [ ] 候选列表后用户答 `第一个`
   - 期望：planner 使用候选组理解序号；不由本地序号 parser 选择

### 8.3 继续澄清场景

1. [ ] 上一轮只问“请告诉我要按哪一天查询”，没有已知年月；用户答 `1日`
   - 期望：planner 继续问完整日期，不能猜年份和月份
2. [ ] 上一轮已有 `2025年2月`；用户答 `31日`
   - 期望：本地日期校验拦截 `2025-02-31`，继续澄清或返回受控错误

### 8.4 新问题切换场景

1. [ ] 上一轮在追问日期；用户下一轮直接说 `那查一下华东销售中心`
   - 期望：模型可判断这是新问题并输出新的合法 `ReadPlan`；本地不靠关键词抢先决定

## 9. 测试与验证

1. [ ] `go fmt ./...`
2. [ ] `go vet ./...`
3. [ ] `make check lint`
4. [ ] `make test`
5. [ ] `make check doc`

> 实际执行结果与时间戳写入对应 readiness/dev-record；本文件不复制执行日志。

## 10. 风险与开放问题

1. **planner 依赖风险**：模型可能仍忽略 `clarification_resume`。应通过 prompt contract、知识包样例和 stub/真实 provider 验证解决，而不是退回本地 parser。
2. **known_params 来源风险**：若上一轮没有结构化保留 `2025年1月` 这类信息，只靠最近问答可能仍能被模型理解，但稳定性较差。建议优先让 planner 的 `CLARIFY` outcome 可携带 `known_params` 或保留足够 recent dialogue，而不是本地从 prose 抽取。
3. **候选集合执行风险**：`全部` 可能需要多个候选详情查询。472 只负责让模型理解短答并进入合法 planner 路径；多步 fanout、预算和去重由 471 承接。
4. **测试虚假通过风险**：stub planner 测试只能证明上下文装配正确，不能证明真实模型稳定。实施记录必须至少包含一组真实 provider 验证，或明确 stopline。
5. **过度 prompt 化风险**：知识包更新应只写字段语义、澄清规则和少量示例；不得把回答口吻、固定话术或 prose 模板重新塞回知识包。
