# DEV-PLAN-501：CubeBox 两次相同查询会话差异与边界判定正式调查报告

**状态**: 已完成（2026-05-04 22:26 CST）

## 0. 适用范围与结论摘要

- **调查对象**：
  1. `conv_0fc7637be99c47538e311860ffe972b2`
  2. `conv_2d934635cd6449e48eb45ff4b9a0dddb`
- **冻结输入**：`你好,请列出全部的财务组织,包括他们的基本信息/组织路径名称和审计信息`
- **结论摘要**：
  1. 两次会话的输入文本、租户、principal、模型链路一致，差异不来自租户/权限/模型配置切换。
  2. 失败会话只有一次 turn，planner/plan contract 层直接触发 `ai_plan_boundary_violation`，随后写入 `turn.error + turn.completed(status=failed)`。
  3. 成功会话并不是“一次请求直接成功”，而是第一次同句请求悬空，第二次同句请求成功。
4. 第二次同句请求会读取第一次已落库的 `turn.user_message.accepted` 历史，因此第二次 planner 的模型输入上下文已经不同于失败会话首轮。
5. 当前系统对该问法的表现不稳定，已证实存在“首轮失败或悬空，同句再次发送后成功”的现象。

### 0.1 文档职责封口

1. `DEV-PLAN-501` 只承载调查事实、证据来源与已证实结论，不作为后续实现 owner。
2. 任何 typed contract 分类、错误主类/子类、turn 生命周期收敛、raw 采样与测试补齐，统一转由 `DEV-PLAN-502` 承接。
3. 本报告中的“允许范围”仅保留为当时调查口语与原始问法，不作为长期工程术语。

## 1. 证据来源与口径

### 1.1 直接取证

1. PostgreSQL 表：
   - `iam.cubebox_conversations`
   - `iam.cubebox_conversation_events`
2. 关键代码：
   - `internal/server/cubebox_query_flow.go`
   - `internal/server/cubebox_api_tool_runner.go`
   - `modules/cubebox/api_call_plan.go`
   - `modules/cubebox/turn_prep.go`
   - `modules/cubebox/store.go`
   - `modules/cubebox/compaction.go`
   - `modules/cubebox/query_working_results.go`
   - `modules/cubebox/query_entity.go`
   - `modules/orgunit/presentation/cubebox/apis.md`
   - `apps/web/src/pages/cubebox/api.ts`
   - `apps/web/src/pages/cubebox/CubeBoxProvider.tsx`
   - `apps/web/src/pages/cubebox/reducer.ts`

### 1.2 可重构但非直接持久化证据

以下内容当前可以依据代码精确说明组装逻辑，但没有落库原文：

1. planner 调用时传给模型的 `Messages`
2. narrator 调用时传给模型的 `Messages`
3. query evidence window / working_results / api_tools 的组装结果

### 1.3 当前缺失的直接证据

以下内容当前未持久化，不能在本报告中伪装成已取证事实：

1. 失败会话 planner 的 raw JSON 输出
2. 成功会话 planner 的 raw JSON 输出
3. narrator 的 raw 文本输出流
4. 成功会话第二次同句发送到底是用户手工点击发送，还是某种 UI 触发行为

## 2. 调查对象冻结

### 2.1 会话元信息

根据 `iam.cubebox_conversations` 直接查询：

| conversation_id | tenant_uuid | principal_id | title | status | archived |
| --- | --- | --- | --- | --- | --- |
| `conv_0fc7637be99c47538e311860ffe972b2` | `00000000-0000-0000-0000-000000000001` | `33e5e61a-2734-474e-9621-6dfa031e34bc` | `新对话` | `active` | `false` |
| `conv_2d934635cd6449e48eb45ff4b9a0dddb` | `00000000-0000-0000-0000-000000000001` | `33e5e61a-2734-474e-9621-6dfa031e34bc` | `新对话` | `active` | `false` |

### 2.2 模型链路一致性

根据 `turn.started` / `turn.completed` / `turn.error` payload 直接取证，两会话使用的链路一致：

- `runtime=cubebox-query-api-calls`
- `provider_id=deepseek`
- `provider_type=openai-compatible`
- `model_slug=deepseek-v4-flash`

结论：差异不来自 provider/model 切换。

## 3. 两次会话一共进行了哪些步骤

## 3.1 失败会话：`conv_2d934635cd6449e48eb45ff4b9a0dddb`

### 步骤时间线

| 步骤 | 事件/动作 | 是否持久化 | 是否调用模型 |
| --- | --- | --- | --- |
| 1 | 会话已加载 `conversation.loaded` | 是 | 否 |
| 2 | 发起一次 `POST /internal/cubebox/turns:stream` | 否（HTTP 请求本身） | 否 |
| 3 | query flow 接管，写入 `turn.started` | 是 | 否 |
| 4 | 写入 `turn.user_message.accepted` | 是 | 否 |
| 5 | planner 调用模型产出 query plan | 否 | 是，planner |
| 6 | planner/plan contract 校验失败，写入 `turn.error(code=ai_plan_boundary_violation)` | 是 | 否 |
| 7 | 同步写入 `turn.completed(status=failed)` | 是 | 否 |

### 实际事件序列

| sequence | turn_id | event_type | 关键 payload |
| --- | --- | --- | --- |
| 1 | - | `conversation.loaded` | `title=新对话,status=active` |
| 2 | `turn_seq_2` | `turn.started` | `trace_id=trace_24e9ed...`,`provider_id=deepseek`,`model_slug=deepseek-v4-flash` |
| 3 | `turn_seq_2` | `turn.user_message.accepted` | 用户原句 |
| 4 | `turn_seq_2` | `turn.error` | `code=ai_plan_boundary_violation`,`message=查询计划超出允许范围，请调整问题后重试。` |
| 5 | `turn_seq_2` | `turn.completed` | `status=failed` |

## 3.2 成功会话：`conv_0fc7637be99c47538e311860ffe972b2`

### 步骤时间线

| 步骤 | 事件/动作 | 是否持久化 | 是否调用模型 |
| --- | --- | --- | --- |
| 1 | 会话已加载 `conversation.loaded` | 是 | 否 |
| 2 | 第一次 `POST /internal/cubebox/turns:stream` | 否（HTTP 请求本身） | 否 |
| 3 | query flow 接管，写入第一次 `turn.started` | 是 | 否 |
| 4 | 写入第一次 `turn.user_message.accepted` | 是 | 否 |
| 5 | 第一次流未形成终态事件，turn 悬空 | 只看到前两条事件 | 无法直接证明 |
| 6 | 第二次同句 `POST /internal/cubebox/turns:stream` | 否（HTTP 请求本身） | 否 |
| 7 | query flow 第二次接管，写入第二次 `turn.started` | 是 | 否 |
| 8 | 写入第二次 `turn.user_message.accepted` | 是 | 否 |
| 9 | planner 调用模型形成可执行 plan | 否 | 是，planner |
| 10 | runner 执行 API tools，确认实体并写 `turn.query_entity.confirmed` | 是 | 否 |
| 11 | narrator 调用模型生成最终回答 | 否 | 是，narrator |
| 12 | 写 `turn.agent_message.delta` / `turn.agent_message.completed` | 是 | 否 |
| 13 | 写 `turn.completed(status=completed)` | 是 | 否 |

### 实际事件序列

| sequence | turn_id | event_type | 关键 payload |
| --- | --- | --- | --- |
| 1 | - | `conversation.loaded` | `title=新对话,status=active` |
| 2 | `turn_seq_2` | `turn.started` | `trace_id=trace_8a95a855...` |
| 3 | `turn_seq_2` | `turn.user_message.accepted` | 用户原句 |
| 4 | `turn_seq_4` | `turn.started` | `trace_id=trace_41739783...` |
| 5 | `turn_seq_4` | `turn.user_message.accepted` | 用户原句 |
| 6 | `turn_seq_4` | `turn.query_entity.confirmed` | `domain=orgunit,intent=orgunit.details,entity_key=200004,as_of=2026-05-04` |
| 7 | `turn_seq_4` | `turn.agent_message.delta` | 最终中文回答全文 |
| 8 | `turn_seq_4` | `turn.agent_message.completed` | `message_id=msg_agent_seq_4` |
| 9 | `turn_seq_4` | `turn.completed` | `status=completed` |

### 关键差异

成功会话的第一次发送只留下了：

- `turn.started`
- `turn.user_message.accepted`

但没有：

- `turn.error`
- `turn.completed`
- `turn.agent_message.*`

结论：成功会话本质上是“第一次悬空 + 第二次成功”，不是“第一次直接成功”。

## 4. 每次步骤调用了什么 API 或其他功能

## 4.1 前端发起步骤

前端发送入口是 [`apps/web/src/pages/cubebox/CubeBoxProvider.tsx`](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/cubebox/CubeBoxProvider.tsx:152) 的 `sendMessage()`：

1. 取 `conversationID`
2. 取 `nextSequence`
3. 调 [`streamTurn()`](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/cubebox/api.ts:91)
4. 向 `POST /internal/cubebox/turns:stream` 发送：
   - `conversation_id`
   - `prompt`
   - `next_sequence`

前端本身没有自动 retry 逻辑；catch 分支只会 dispatch `stream_failed_locally`。

## 4.2 后端 query flow 步骤

后端 query flow 主入口是 [`TryHandle()`](/home/lee/Projects/Bugs-And-Blossoms/internal/server/cubebox_query_flow.go:408)：

1. `queryContext()`：回放会话历史事件，构造 `QueryContext`
2. `runner.Tools()`：取得当前 runtime 可用的 API tools
3. `NewQueryWorkingResultsState()`：初始化 query loop budget/working state
4. `produceAPIPlan()`：调用 planner 模型，产出 planner outcome
5. `prepareQueryTurn()`：准备 turn id、sequence、canonical context
6. 写 `turn.started`
7. 写 `turn.user_message.accepted`
8. 若 `PlannerOutcomeAPICalls`：
   - `ValidateAPICallPlan()`
   - `runner.ExecutePlan()`
   - 再次 `produceAPIPlan()` 或进入 `DONE`
9. 若 `PlannerOutcomeDone`：
   - `NarrateQueryResult()`
   - 写 `turn.agent_message.delta`
   - 写 `turn.agent_message.completed`
   - 写 `turn.completed`
10. 若任一 terminal error：
   - `writeQueryTerminalError()`
   - 一次性写 `turn.error + turn.completed(status=failed)`

## 4.3 API tool 执行步骤

`runner.ExecutePlan()` 位于 [`internal/server/cubebox_api_tool_runner.go`](/home/lee/Projects/Bugs-And-Blossoms/internal/server/cubebox_api_tool_runner.go:65)：

1. 再次 `ValidateAPICallPlan(plan)`
2. 路由校验 `toolMap[method+path]`
3. 参数白名单与必填校验 `validateAPICallParams()`
4. authz 运行时校验 `AuthorizePrincipal()`
5. 构造本地 HTTP 请求
6. 分派到以下只读 API handler：
   - `/org/api/org-units`
   - `/org/api/org-units/details`
   - `/org/api/org-units/search`
   - `/org/api/org-units/audit`

## 5. 每次是否都调用了模型

## 5.1 失败会话

| 步骤 | 是否调用模型 | 模型角色 |
| --- | --- | --- |
| `conversation.loaded` | 否 | - |
| `turn.started` | 否 | - |
| `turn.user_message.accepted` | 否 | - |
| planner 规划 | 是 | planner |
| `turn.error` / `turn.completed(failed)` | 否 | - |

结论：失败会话只调用了 planner，没有调用 narrator。

## 5.2 成功会话

| 步骤 | 是否调用模型 | 模型角色 |
| --- | --- | --- |
| 第一次 `turn.started` / `turn.user_message.accepted` | 否 | - |
| 第一次 planner 是否完成调用 | 无法直接取证 | 不可写死 |
| 第二次 planner 规划 | 是 | planner |
| API tool runner | 否 | - |
| narrator 生成最终回答 | 是 | narrator |
| `turn.agent_message.*` / `turn.completed` | 否 | - |

结论：成功会话第二次请求至少发生了 planner + narrator 两次模型调用。

## 6. 输入给模型的内容是什么，模型返回的内容是什么

## 6.1 planner：可精确重构的输入组成

planner 调用在 [`ProduceAPIPlan()`](/home/lee/Projects/Bugs-And-Blossoms/internal/server/cubebox_query_flow.go:560) 中执行，真实发送给模型的是：

1. `BaseURL`
2. `APIKey`
3. `Model`
4. `Messages = buildPlannerMessages(input)`
5. `Input = input.Prompt`

其中 `buildPlannerMessages()` 在 [`internal/server/cubebox_query_flow.go`](/home/lee/Projects/Bugs-And-Blossoms/internal/server/cubebox_query_flow.go:627) 组装如下：

1. system：planner 基线规则
2. system：knowledge packs
3. system：`api_tools`
4. system：`query_evidence_window`
5. system：`working_results`
6. system：planner corrections（若有）
7. user：当前 `input.Prompt`

### 失败会话首轮 planner 输入

可重构但未落库的 planner 输入事实：

1. 当前用户输入是冻结原句
2. `QueryContext` 来自空白历史，仅含 `conversation.loaded`
3. `query_evidence_window` 基本为空窗口
4. `working_results` 仅含 round=1 的初始预算
5. `api_tools` 为 orgunit 四个只读 API tool

### 成功会话第二轮 planner 输入

可重构且与失败会话不同的关键点：

1. 当前用户输入仍是冻结原句
2. `QueryContext` 来自前一轮已经落库的历史事件
3. 第二次请求前，历史至少包含第一次的 `turn.user_message.accepted`
4. 因第一次没有 `turn.agent_message.completed`，所以历史里只有“用户先前问过同一句”，没有助手回答

该差异来自：

- [`PrepareConversationPromptView()`](/home/lee/Projects/Bugs-And-Blossoms/modules/cubebox/store.go:275)
- [`QueryContextFromEvents()`](/home/lee/Projects/Bugs-And-Blossoms/modules/cubebox/query_entity.go:169)
- [`PrepareTurnStream()`](/home/lee/Projects/Bugs-And-Blossoms/modules/cubebox/turn_prep.go:16)
- [`buildPromptViewForProvider()`](/home/lee/Projects/Bugs-And-Blossoms/modules/cubebox/compaction.go:29)

结论：成功会话第二轮 planner 输入上下文与失败会话首轮不同，这一点已由事件历史和代码路径共同证实。

## 6.2 planner：返回内容

当前 `ProduceAPIPlan()` 会把模型流式 delta 累积到本地字符串，再立即做：

1. `DecodePlannerOutcome(raw)`
2. `ValidateAPICallPlan()` 或其他 outcome 规范化

但 raw string 没有持久化，也未写入会话事件。

因此当前只能区分：

1. **失败会话**：planner 输出最终落到了 contract error 桶
2. **成功会话第二轮**：planner 输出最终能走到可执行 plan，并产生 API 执行结果

不能直接回收当时的 raw JSON 原文。

## 6.3 narrator：可精确重构的输入组成

narrator 调用在 [`NarrateQueryResult()`](/home/lee/Projects/Bugs-And-Blossoms/internal/server/cubebox_query_flow.go:817) 中执行，真实发送给模型的是：

1. system：结果叙述器规则
2. user：`buildQueryNarrationEnvelope(input)` 的 JSON

该 envelope 至少包含：

1. `user_prompt`
2. `query_evidence_window`
3. `results`

## 6.4 narrator：返回内容

成功会话 narrator 的最终文本已被持久化在 `turn.agent_message.delta` 中，因此这是**已直接取证**内容。

失败会话没有 narrator 输出。

## 7. 每一步骤如何触发边界/协议校验，又是如何判定失败的

## 7.1 调查期口语中的“允许范围”不是单点

调查期口语中的“允许范围/超出范围”至少由以下六层共同构成。长期工程术语与 typed contract 分类由 `DEV-PLAN-502` 统一承接。

1. planner outcome contract
2. API call plan 线性结构 contract
3. API tool catalog / request schema contract
4. execution budget / repeat budget
5. authz/runtime contract
6. 前端 stream terminal contract

## 7.2 planner outcome contract

planner raw 输出先经过：

1. `DecodePlannerOutcome()`
2. 结果类型归一化

若命中：

- `ErrPlannerOutcomeInvalid`
- `ErrAPICallPlanSchemaConstrainedDecodeFailed`
- `ErrAPICallPlanBoundaryViolation`

则 `isQueryPlannerContractError()` 返回 true，并在 [`queryPlanErrorToTerminal()`](/home/lee/Projects/Bugs-And-Blossoms/internal/server/cubebox_query_flow.go:1920) 映射成用户可见错误。

其中：

- `ErrPlannerOutcomeInvalid` -> `cubebox_query_planner_outcome_invalid`
- `ErrAPICallPlanSchemaConstrainedDecodeFailed` -> `ai_plan_schema_constrained_decode_failed`
- 其余 boundary -> `ai_plan_boundary_violation`

## 7.3 API call plan shape boundary

[`ValidateAPICallPlan()`](/home/lee/Projects/Bugs-And-Blossoms/modules/cubebox/api_call_plan.go:48) 规定：

1. `calls` 不可为空
2. 每个 `id` 必填且不可重复
3. `method/path/params/depends_on` 必填
4. `depends_on` 长度不得大于 1
5. 第一条必须 `depends_on=[]`
6. 后续每一条必须只依赖前一条

任何违反都落 `ErrAPICallPlanBoundaryViolation`。

## 7.4 API tool boundary

[`validateAPICallParams()`](/home/lee/Projects/Bugs-And-Blossoms/internal/server/cubebox_api_tool_runner.go:150) 与 `ExecutePlan()` 会继续约束：

1. `method/path` 必须存在于 runtime `toolMap`
2. 只允许使用 tool schema 里声明的参数
3. required param 不能缺失
4. required string 不能空
5. numeric param 必须是整数
6. tool 的 capability / route requirement / runtime authz 必须一致

任何违反会落：

- `ErrAPICallPlanBoundaryViolation`
- `ErrAPICatalogDriftOrExecutorMissing`
- scope/authz 相关错误

## 7.5 execution budget boundary

[`DefaultQueryLoopBudget()`](/home/lee/Projects/Bugs-And-Blossoms/modules/cubebox/query_working_results.go:98) 固定预算为：

- `MaxPlanningRounds=80`
- `MaxExecutedSteps=160`
- `MaxWorkingResultItems=1000`
- `MaxRepeatedPlan=2`

query flow 每轮会通过：

1. `CanPlan()`
2. `CanExecute(plan)`
3. `NoteRepeat()`

判断是否超预算或进入重复计划死循环。

## 7.6 authz / runtime boundary

`ExecutePlan()` 还要求：

1. capability key 与 object/action 一致
2. route requirement 与 tool 元数据一致
3. `AuthorizePrincipal()` 返回允许

否则不进入业务查询。

## 7.7 前端 stream terminal boundary

前端 [`streamTurn()`](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/cubebox/api.ts:91) 明确要求：

1. 流式过程中必须看到 `turn.completed` 或 `turn.error`
2. 若流结束但没看到终态，直接抛 `stream turn failed: missing terminal event`

随后 [`CubeBoxProvider.sendMessage()`](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/cubebox/CubeBoxProvider.tsx:152) catch 后只会 dispatch `stream_failed_locally`，没有内建自动 retry。

结论：成功会话第一次悬空，前端按当前代码会本地 fail-closed，但不会自动再发第二次请求。

## 8. 两次会话第一次分叉发生在哪里

第一次可直接证实的分叉点不是 planner 代码行，而是**会话事实层**：

1. 失败会话只有一次 turn，并在首轮结束时写入 terminal failure。
2. 成功会话第一次 turn 没有 terminal event，导致历史里残留一条已接受用户消息。
3. 成功会话第二次同句请求读取到了这条残留历史，形成不同的 `QueryContext` / `PromptView` / `query_evidence_window` 输入条件。

因此最早可证实分叉点是：

- 失败会话：首轮 planner/plan contract 直接终止
- 成功会话：首轮未终止，第二轮在不同历史上下文下再次规划

## 9. 根因结论

## 9.1 已证实结论

1. 失败会话的用户可见报错来自 query planner contract error 映射链，不是前端文案臆造。
2. 成功会话之所以“又能成功”，不是相同输入在相同上下文下一次成功一次失败，而是第二次请求带入了第一次残留历史。
3. 当前系统没有 planner/narrator raw 输入输出持久化，导致无法把失败子型细分到 raw JSON 级别。
4. 当前前端没有自动 retry，因此成功会话第二次同句请求不是代码里显式实现的“自动重试”。

## 9.2 高概率解释，但当前缺少最后一跳直接证据

1. 成功会话第二次同句请求极可能来自用户再次发送，或某个 UI 行为再次触发发送。
2. 失败会话命中的具体子型更可能是：
   - `ErrPlannerOutcomeInvalid`
   - `ErrAPICallPlanSchemaConstrainedDecodeFailed`
   - `ErrAPICallPlanBoundaryViolation`
   三者之一
3. 但因为 raw planner output 未持久化，当前不能写死是哪一种。

## 10. 对用户五个问题的逐项回答

1. **一共进行了哪些步骤**
   - 失败会话：一次 turn，7 个关键步骤。
   - 成功会话：两次同句 turn，其中第一次悬空，第二次完成，13 个关键步骤。

2. **每次步骤调用了什么 API 或其他功能**
   - 前端统一调用 `POST /internal/cubebox/turns:stream`
   - 后端统一经过 query flow、planner、runner、narrator
   - 成功会话第二轮额外执行了 orgunit 四个只读 API tool 体系中的至少部分工具

3. **每次是否都调用了模型**
   - 不是。只有 planner 和 narrator 步骤调用模型。
   - 失败会话只确定调用了 planner。
   - 成功会话第二轮确定调用了 planner 和 narrator。

4. **输入给模型的内容是什么，模型返回的内容是什么**
   - 输入结构可由代码精确重构：system rules + knowledge packs + api_tools + query_evidence_window + working_results + user prompt。
   - 失败会话 planner raw 输出当前不可直接回收。
   - 成功会话 narrator 的最终输出已直接落在 `turn.agent_message.delta`。

5. **每一步骤是如何计算当前的允许范围，又是如何判断超出范围的**
   - 按调查事实，该口语问题实际对应 planner contract、plan shape、tool schema、budget、authz、前端终态要求六类边界/协议校验。
   - 失败首先在 planner/plan contract、tool schema、budget 或 authz 这些层被 fail-closed，再映射到用户可见 terminal error；长期 typed contract 分类见 `DEV-PLAN-502`。

## 11. 后续审计建议

1. 证据层当前缺口已在本报告中明确标注，不再在本报告内延伸实现建议。
2. 实施 owner 与后续修复范围见 `DEV-PLAN-502`。

## 12. 关联文档

1. 调查方案：`docs/dev-plans/500-cubebox-two-conversations-boundary-investigation-plan.md`
2. 证据记录：`docs/dev-records/DEV-PLAN-500-READINESS.md`
