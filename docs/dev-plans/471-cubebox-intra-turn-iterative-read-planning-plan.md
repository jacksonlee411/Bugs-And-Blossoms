# DEV-PLAN-471：CubeBox 借鉴 Codex Agent Loop 的同一 Turn 内迭代式只读编排方案

**状态**: 规划中（2026-04-26 10:52 CST；已按 DEV-PLAN-003 评审收敛 P1/P2）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：借鉴 Codex 的“模型 -> 工具执行 -> observation 回灌 -> 模型继续规划 -> 最终回答”agent loop，把 CubeBox 当前“一次 planner -> 一次 execute -> narrator”收敛为“同一用户 turn 内有限次只读 planner/executor 小循环，最后统一 narrator”的查询编排主链。
- **关联模块/目录**：`internal/server/cubebox_query_flow.go`、`modules/cubebox/read_plan.go`、`modules/cubebox/read_executor.go`、`modules/cubebox/query_entity.go`、`modules/orgunit/presentation/cubebox`、`third_party/openai-codex`、`docs/dev-records/DEV-PLAN-471-READINESS.md`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-003`、`DEV-PLAN-012`、`DEV-PLAN-430`、`DEV-PLAN-460`、`DEV-PLAN-461`、`DEV-PLAN-463`、`DEV-PLAN-468`、`DEV-PLAN-468C`、`DEV-PLAN-470`
- **用户入口/触点**：`/internal/cubebox/turns:stream`、CubeBox 页面同一会话内的查询型 turn

### 0.1 Simple > Easy 三问

1. **边界**：模型负责理解当前目标、观察只读结果、决定下一次已登记只读 API 调用或声明查询完成；本地代码负责循环调度、工具目录供给、只读执行、observation 构造、预算/去重/权限/参数校验和最终 narrator 调用。
2. **不变量**：不新增第二套查询 endpoint；不新增 `orgunit` 专用读 API；不引入业务 DSL、JSONPath、脚本求值器、DAG/workflow engine 或 capability-specific 编排分支；不绕过执行注册表、租户、权限与参数校验；最终用户可见回答只由 narrator 在循环结束后输出一次。
3. **可解释**：reviewer 必须能在 5 分钟内说明：Codex agent loop 的哪部分被借鉴；为什么 `working_results` 等价于只读工具 observation 而不是长期记忆；为什么需要明确 `DONE` 终止态；为什么多次小 `ReadPlan` 比扩张单次大计划更简单。

### 0.2 Codex 对标结论

- **借鉴对象**：`openai/codex` 的核心 agent loop。其模式不是一次性生成完整计划，而是在同一个 turn 内反复执行：
  1. 构造模型输入
  2. 模型输出工具调用或最终消息
  3. 本地执行工具调用
  4. 将工具结果作为 observation 写回下一次模型输入
  5. 直到模型不再请求工具并给出最终输出
- **本计划只借鉴的部分**：
  - loop 是一等运行时，由代码负责预算、状态推进和工具结果回灌
  - 工具目录由本地注册表约束，模型只能选择已登记能力
  - 工具 observation 进入下一次模型调用，驱动模型“看结果再规划”
  - 最终用户可见输出与中间工具 observation 分离
- **本计划不借鉴的部分**：
  - 不建设开放式 agent/tool 平台
  - 不允许模型访问 shell、文件系统、数据库或任意 HTTP 工具
  - 不引入并发 subagent、DAG 调度、workflow engine、通用函数调用市场
  - 不把模型输出当作授权来源或执行事实源
- **本地参考源码落点**：`third_party/openai-codex`。该目录仅作为源码学习参考，不得成为运行时依赖，不纳入 Go import、不参与 CubeBox 构建链路；源码 HEAD 留痕只作为 readiness 辅助证据，不作为 P0 用户能力验收项。
- **本地对标阅读路径**：
  - `third_party/openai-codex/codex-rs/core/src/session/turn.rs`：`run_turn(...)` 持有同一 turn 内的模型调用循环，并依据 `needs_follow_up` 决定是否继续 sampling。
  - `third_party/openai-codex/codex-rs/core/src/stream_events_utils.rs`：`handle_output_item_done(...)` 将模型输出解析为工具调用、执行工具，并把工具结果标记为需要后续模型调用。
  - `third_party/openai-codex/codex-rs/core/src/tools/router.rs`：`ToolRouter` 从本地配置构造可用工具集合，并把模型输出路由到受控工具处理器。
  - `third_party/openai-codex/codex-rs/core/src/context_manager/history.rs` 与 `context_manager/normalize.rs`：维护历史项、token 估算、工具调用输出配对与孤儿输出清理。
  - `third_party/openai-codex/codex-rs/core/src/compact.rs` 与 `compact_remote.rs`：在上下文预算压力下执行本地/远端 compaction，并替换 active history。

### 0.3 现状研究摘要

- 当前 `cubeboxQueryFlow.TryHandle(...)` 只做一次 `ProduceReadPlan(...)`、一次 `ExecutionRegistry.ExecutePlan(...)`，随后直接 `NarrateQueryResult(...)`。
- planner 已可消费 `knowledge packs + query_dialogue_context`；其中 `query_dialogue_context` 的跨 turn 事实窗口与 planner projection 口径以 `468C` 为 SSOT。
- executor 已支持线性 `ReadPlan`、前序 step 结果字段引用、参数白名单与 fail-closed 执行。
- `ReadPlan` 仍是单次产物；planner 当前看不到本 turn 已执行的小步骤结果。
- `query_dialogue_context` 面向跨 turn 历史事实，`working_results` 面向同一 turn 临时 observation；两者不能合并、不能互相回写，也不能互相派生新的 canonical facts。
- `resolved_entity` 已由 `468C` 明确移出当前范围；471 不应把它当作 loop 快捷状态、当前 turn winner 或 `DONE` 的摘要载体。
- `orgunit.list` 已返回 `org_units[].has_children`；当前缺口不是业务 API，而是同一 turn 内的 observation 回灌与再规划。

### 0.4 最容易出错的位置

- 把 Codex 借鉴误做成开放式工具平台
- 用 `NO_QUERY` 同时表达“不支持查询域”和“已有结果足够”，导致状态混淆
- 把 `working_results` 写入长期 canonical events，污染后续 turn
- 把当前 turn 的 observation 设计成业务 DSL、JSONPath 或 capability-specific 解释器
- 把 `working_results` 写成 `orgunit` 专用状态，例如待查 parent 队列、业务聚合表或 winner 状态
- 让 `DONE` 裸文本与“非法 JSON fail-closed”混用，导致 planner outcome 解析口径漂移
- 未加预算、去重和重复查询检测，导致 planner/executor 循环失控
- narrator 中途介入并输出半成品用户可见回答

## 1. 背景与问题

### 1.1 需求来源

针对“把那些有下级的下级组织的下级组织列出来”这类查询，用户明确要求通过模型理解与自动编排已有能力解决，而不是新增业务 API、DSL 或本地业务分支。

### 1.2 原始失败案例

- **原始多轮输入**：
  1. `查一下 100000 在 2026-04-25 的组织详情`
  2. `查它的下级组织中有下级组织的下级组织`
- **当时系统表现**：
  - 第一轮可成功返回组织 `100000` 在 `2026-04-25` 的详情
  - 第二轮返回：`查询参数无效，请检查后重试。`
- **用户真实意图**：
  - 第二句中的“它”指向上一轮已查到的组织 `100000`
  - 系统应先查出该组织在 `2026-04-25` 的直接下级组织
  - 再从直接下级中识别哪些组织仍有下级，例如 `has_children=true`
  - 再分别查出这些组织的各自下级组织
  - 最后把这些“有下级的直接下级组织”的下级组织汇总为最终回答
- **本质缺口**：
  - `468C` 负责跨 turn query dialogue fact window，让 planner 能继承最近确认实体、候选组、最近问答与澄清状态
  - 现有 `ReadPlan` / executor 的线性前序引用适合“先 search 唯一命中，再 details/list”的已知链路
  - 本案例要求“执行一步 -> 观察结果 -> 根据结果决定下一步”
  - 因此需要同一 turn 内的模型再入循环，而不是单次大 `ReadPlan`

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 将当前 query flow 从“单次 planner-executor”升级为“同一用户 turn 内有限次 planner-executor-planner loop”。
- [ ] 每次小循环只允许模型输出合法小 `ReadPlan` 或冻结的循环控制态；不得引入新业务计划语言。
- [ ] 将当前 turn 的已执行结果整理成业务无关的稳定结构化 `working_results`，作为下一次 planner 输入中的 tool observation。
- [ ] 增加明确的 planner 终止协议与 wire format：`READ_PLAN`、`CLARIFY`、`DONE`、`NO_QUERY`。
- [ ] 将 pre-execution `NO_QUERY` 的用户可见输出收敛为“服务端受控事实 + 模型纯文本序号列表”；不做卡片、不新增前端组件、不把整段回复硬编码在 Go 中。
- [ ] 保持 narrator 只在 `DONE` 后调用一次；单步查询也通过“执行一次 `ReadPlan` -> 回灌 observation -> planner 返回 `DONE` -> narrator”闭环，避免隐藏式“执行后自动认为足够”的分支。
- [ ] 通过 `max_planning_rounds`、`max_executed_steps`、`max_working_result_items`、重复查询检测与 fail-closed stopline 保证循环不会失控。
- [ ] 对 P0 案例支持串行 fanout：当多个直接下级 `has_children=true` 时，允许 planner 在预算内分批或逐个继续查其下级。

### 2.2 非目标

- 不新增 `orgunit` / 其他业务模块的专用只读 API、递归 API、孙级 API。
- 不引入通用 DSL、JSONPath、表达式求值器、脚本执行器、DAG planner、workflow engine 或 capability-specific mini language。
- 不恢复或扩张 `page_context` 作为本计划的编排输入。
- 不在 query flow 中写业务专用 `if prompt contains ...` 分支。
- 不在本计划内处理跨 turn 长期记忆、会话压缩摘要、remote compact 或模型摘要恢复。
- 不引入并发工具执行；P0 只做串行循环。

### 2.3 用户可见性交付

- **用户可见入口**：CubeBox 查询型对话；仍由 `/internal/cubebox/turns:stream` 承接。
- **最小可操作闭环**：用户在单条问题中提出“需要先查一层结果、再根据结果决定是否继续查”的查询时，系统可在同一 turn 内自动完成多次只读编排，并直接给出最终答案。
- **本期最小验收样例**：
  - 先问：`查一下 100000 在 2026-04-25 的组织详情`
  - 再问：`把那些有下级的下级组织的下级组织列出来`

## 3. 工具链与门禁

- **命中触发器**：
  - [X] Go 代码
  - [ ] `apps/web/**` / presentation assets / 生成物
  - [ ] i18n（仅 `en/zh`）
  - [ ] DB Schema / Migration / Backfill / Correction
  - [ ] sqlc
  - [ ] Routing / allowlist / responder / 相关路由注册/映射
  - [ ] AuthN / Tenancy / RLS
  - [ ] Authz（Casbin）
  - [ ] E2E
  - [X] 文档 / readiness / 证据记录
  - [X] 其他专项门禁：`error-message`、`root-surface`

- **本次引用的 SSOT**：
  - `AGENTS.md`
  - `docs/dev-plans/000-docs-format.md`
  - `docs/dev-plans/003-simple-not-easy-review-guide.md`
  - `docs/dev-plans/012-ci-quality-gates.md`
  - `docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`
  - `docs/dev-plans/460-cubebox-digital-assistant-positioning-and-execution-contract.md`
  - `docs/dev-plans/461-cubebox-query-scenarios-minimal-contract.md`
  - `docs/dev-plans/468-cubebox-session-continuity-and-model-autonomy-improvement-plan.md`
  - `docs/dev-plans/468c-cubebox-query-context-fact-window-plan.md`
  - `docs/dev-plans/470-cubebox-page-context-scope-removal-and-cleanup-plan.md`
  - `third_party/openai-codex`
  - `Makefile`

## 4. 架构方案

### 4.1 Codex Loop 到 CubeBox Query Loop 的映射

| Codex agent loop 概念 | CubeBox 471 对应物 | 说明 |
| --- | --- | --- |
| Model reasoning | query planner | 只负责选择已登记只读 API 或声明完成/澄清/不支持 |
| Tool schema/catalog | `ExecutionRegistry` 生成的 `read_api_catalog` + knowledge packs | 注册表是执行事实源，知识包解释业务语义 |
| Tool call | 小 `ReadPlan` | 每轮仍是现有合法线性 ReadPlan |
| Tool execution | `ExecutionRegistry.ExecutePlan(...)` | 保持租户、权限、参数白名单和 executor 校验 |
| Tool observation | `working_results` | 当前 turn 内临时结构化事实，不入长期事件 |
| Final assistant message | narrator 输出 | 只在 loop 完成后输出一次 |

### 4.2 主流程

```mermaid
flowchart TD
  U[User Prompt] --> C[Build planner input]
  C --> P[Planner]
  P -->|READ_PLAN| E[Execute small ReadPlan]
  E --> O[Build working_results observation]
  O --> C
  P -->|DONE| N[Narrator once]
  P -->|CLARIFY| Q[Clarification terminal]
  P -->|NO_QUERY before execution| X[No-query guidance]
  P -->|NO_QUERY after execution| B[Boundary violation stopline]
  C -->|budget exceeded| S[Fail-closed stopline]
  N --> R[Final Reply]
```

### 4.3 Planner 控制态

当前 `ReadPlan` schema 不应被扩成业务 DSL，但 loop 需要一个薄控制 envelope。P0 冻结 JSON envelope 作为新增 planner outcome 的唯一推荐格式：

```json
{ "outcome": "READ_PLAN", "plan": { "intent": "orgunit.list", "missing_params": [], "steps": [] } }
```

```json
{ "outcome": "DONE" }
```

```json
{ "outcome": "NO_QUERY" }
```

```json
{ "outcome": "CLARIFY", "missing_params": ["as_of"], "clarifying_question": "请提供查询日期。" }
```

解析兼容矩阵：

| 模型输出 | 解析结果 | 约束 |
| --- | --- | --- |
| JSON envelope `{"outcome":"READ_PLAN","plan":...}` | `READ_PLAN` | `plan` 必须是合法小 `ReadPlan` |
| 现有裸 `ReadPlan JSON` | `READ_PLAN` 或 `CLARIFY` | 仅为兼容现有 planner 输出；不得新增第二套业务语义 |
| JSON envelope `{"outcome":"CLARIFY",...}` | `CLARIFY` | `missing_params` 与 `clarifying_question` 必须非空 |
| JSON envelope `{"outcome":"DONE"}` | `DONE` | 只允许在至少执行过一次只读计划后出现 |
| JSON envelope `{"outcome":"NO_QUERY"}` 或现有裸文本 `NO_QUERY` | `NO_QUERY` | 裸文本仅兼容现有 no-query 输出 |
| 裸文本 `DONE`、其他裸文本、非法 JSON、未知 outcome | 非法 outcome | fail-closed，映射为 planner outcome 边界错误 |

P0 冻结以下 planner outcome：

1. `READ_PLAN`
   - 表示本轮需要执行一个合法小 `ReadPlan`
   - 兼容现状：模型直接输出裸 `ReadPlan JSON` 时等价于 `READ_PLAN`
2. `CLARIFY`
   - 表示缺少必要参数或候选不可静默选择
   - 兼容现状：带 `missing_params + clarifying_question` 且无 `steps` 的 `ReadPlan`
3. `DONE`
   - 表示 planner 已看到足够 `working_results`，可以进入 narrator
   - 必须输出 JSON envelope；不接受裸字面量 `DONE`
   - 只允许在至少执行过一次只读计划后出现
4. `NO_QUERY`
   - 只表示用户请求不属于当前知识包支持的查询域
   - 仅允许在尚未执行任何查询时作为 no-query guidance 入口
   - 若已执行过查询后又返回 `NO_QUERY`，视为 planner 边界错误，fail-closed；不能把 `NO_QUERY` 当完成态

### 4.4 Planner 输入结构

每次 planner 调用都包含：

1. 静态 planner system prompt
2. knowledge packs
3. 由 `ExecutionRegistry` 派生的 `read_api_catalog`
4. `query_dialogue_context`
5. 当前用户原始 prompt
6. 当前 turn 内 `working_results`

其中：

- `query_dialogue_context` 只承接跨 turn 历史事实，字段语义、canonical 上限与 planner projection 口径统一以 `DEV-PLAN-468C` 为准。
- `working_results` 只承接当前 turn 的临时 observation，不进入 canonical events，不进入 `query_dialogue_context`。
- 两者必须并列输入给 planner；不得把 `working_results` 合并成 `query_dialogue_context`，也不得从 `query_dialogue_context` 派生当前 turn 的 synthetic observation。
- `resolved_entity` 不在 471 的 planner 输入范围；loop 不得依赖它表达“本 turn 已解析目标”“当前 winner”或 `DONE` 摘要。

`read_api_catalog` 是代码从注册表生成的最小执行事实：

```json
{
  "read_api_catalog": [
    {
      "api_key": "orgunit.list",
      "required_params": ["as_of"],
      "optional_params": ["include_disabled", "parent_org_code", "keyword", "status", "page", "size"]
    }
  ]
}
```

知识包仍负责解释字段语义、默认策略、案例与安全边界；注册表目录负责告诉模型“当前真实可执行工具是什么”。

### 4.5 `working_results` 最小契约

`working_results` 是当前 turn 内的 tool observation，只存在于 `TryHandle(...)` 生命周期内。它记录“已经执行了什么、最新看到什么、哪些 fingerprint 已经跑过、预算还剩多少”，不记录业务专用待办队列、业务 winner 或 capability-specific 聚合事实。

```json
{
  "working_results": {
    "round_index": 2,
    "original_user_goal": "把那些有下级的下级组织的下级组织列出来",
    "budget": {
      "max_planning_rounds": 4,
      "remaining_planning_rounds": 2,
      "max_executed_steps": 8,
      "remaining_executed_steps": 7,
      "max_working_result_items": 50
    },
    "completed_plans": [
      {
        "round": 1,
        "intent": "orgunit.list",
        "steps": [
          {
            "step_id": "step-1",
            "api_key": "orgunit.list",
            "params_fingerprint": "orgunit.list|as_of=2026-04-25|parent_org_code=100000",
            "item_count": 3,
            "truncated": false,
            "summary": {
              "as_of": "2026-04-25",
              "org_unit_count": 3
            }
          }
        ]
      }
    ],
    "latest_observation": {
      "round": 1,
      "step_id": "step-1",
      "api_key": "orgunit.list",
      "params_fingerprint": "orgunit.list|as_of=2026-04-25|parent_org_code=100000",
      "as_of": "2026-04-25",
      "items": [
        {
          "org_code": "110000",
          "name": "示例组织",
          "status": "active",
          "has_children": true
        }
      ],
      "item_count": 3,
      "truncated": false
    },
    "executed_fingerprints": [
      "orgunit.list|as_of=2026-04-25|parent_org_code=100000"
    ],
    "repeat_observations": []
  }
}
```

约束：

- 不包含密钥、provider 配置、内部 session token 或未授权数据。
- 不原样塞入无限 raw payload；必须按预算裁剪。
- 不新增 `aggregated_facts`、`remaining_parent_org_codes`、`current_winner` 等业务命名状态；若 observation item 中包含 `has_children` 等字段，只能来自已授权 read executor 的裁剪响应，query flow 不解释其业务含义。
- 不作为长期 canonical event，不进入 `query_dialogue_context`，也不进入 `468C` 的 query dialogue fact window projection。
- 不作为 narrator 之外的用户可见 JSON。
- 跨 turn 锚点只来自 `468C` 的 `query_dialogue_context`；同一 turn 临时事实只来自 `working_results`。
- `remaining_goal_hint` 如确有必要，只能是 query flow 生成的通用短提示，不得包含业务专用 prose 或隐藏分支语义。
- P0 orgunit 样例中的“挑出 `has_children=true` 后继续查”必须由 planner 根据 `latest_observation.items` 与知识包案例自行规划，不得由 `modules/cubebox` 纯函数计算待查队列。

### 4.6 Pre-execution NO_QUERY 用户引导

`NO_QUERY` 不是执行错误；在尚未执行任何只读 API 前，它表示“当前输入未进入已支持查询闭环”。用户可见回复必须保持纯文本流，不做卡片、不加前端组件。

P0 冻结以下分工：

- planner 只负责判定 `READ_PLAN / CLARIFY / DONE / NO_QUERY`
- 服务端负责从知识包与 `query_dialogue_context` 整理受控提示事实
- 模型负责把受控事实组织成中文纯文本序号列表
- 前端仍只消费普通文本 delta

服务端提供给模型的事实只包含：

```json
{
  "scope_summary": "当前主要支持组织相关只读查询。",
  "suggested_prompts": [
    "查“华东销售中心”的详情",
    "查“华东销售中心”当前的下级组织",
    "搜索名称包含“销售”的组织"
  ],
  "query_context_hint": {
    "has_recent_confirmed_entity": false
  }
}
```

事实来源：

- `scope_summary` 来自知识包固定描述
- `suggested_prompts` 来自知识包 curated examples，默认使用名称型、关键词型、关系型问法，不默认展示编码
- `query_context_hint.has_recent_confirmed_entity` 来自现有 `query_dialogue_context`
- 当存在最近确认实体时，服务端可切换到知识包提供的“这个组织 / 它”续问示例；若示例中包含 `当前日期` 占位，可替换为最近确认实体的 `as_of`

模型输出规则只约束格式与边界，不硬编码完整答案：

```text
你负责把给定的受控事实整理成面向用户的中文回复。

要求：
1. 先用一句话说明当前支持范围。
2. 空一行后输出“你可以直接这样问：”
3. 然后输出带序号的列表，使用 1. 2. 3. 格式，每条单独一行。
4. 只能使用 provided suggested_prompts，不得新增未提供的能力或示例。
5. 不得提到内部术语，例如 NO_QUERY、ReadPlan、planner、知识包、API。
6. 语气直接、简洁，不道歉，不解释系统内部机制。
```

P0 不引入复杂 reason system；仅区分：

- `not_query`：例如“你好”，返回支持范围 + 示例问题
- `too_vague`：例如“查组织”，优先走 `CLARIFY`；如果仍无法形成澄清，则给更具体的名称型示例

`unsupported_domain` 等更细 reason 暂不作为 471 范围。

### 4.7 候选澄清事件契约

当 loop 中某次执行返回“候选不可静默选择”时，471 必须沿用并显式对齐 `468C` 的事件契约，而不是再发明一套 loop 专用关联语义。

规则：

1. `turn.query_candidates.presented` 的 prompt-facing 权威关联键只有 `group_id`。
2. `turn.query_clarification.requested` 在候选澄清场景必须写 `candidate_group_id`，并通过它关联最近一次或同一轮的候选组。
3. `candidate_source` 只描述来源，不得作为候选澄清的关联键。
4. `turn_id`、loop round、`working_results.round_index` 或其他内部计数器只可用于审计或调试，不得作为 prompt-facing 关联主键。
5. `query_dialogue_context` 对候选事实的组织方式统一以 `468C` 的 `recent_candidate_groups` / `recent_candidates` compatibility alias 为准；471 不得改写其语义。

### 4.8 预算与去重

P0 默认预算建议：

- `max_planning_rounds = 4`
- `max_executed_steps = 8`
- `max_working_result_items = 50`
- `max_repeated_plan_fingerprint = 1`

每次执行前生成稳定 `plan_fingerprint` / `step_fingerprint`，至少包含：

- `api_key`
- 全量归一化参数 key/value，按 key 稳定排序
- 空值、布尔、日期、数字与字符串的稳定编码

若 planner 再次请求已执行过的同一 fingerprint：

- 第一次重复：作为 `repeat_observations` 告知 planner 已执行过，不重复执行；消耗一次 planning round，不消耗 executed step，要求其选择下一步或 `DONE`
- 再次重复：fail-closed，输出统一 stopline

预算耗尽口径：

- 每次 planner 调用前检查 `max_planning_rounds`；每次执行前检查 `max_executed_steps`。
- 若预算耗尽时 planner 尚未返回合法 `DONE` / `CLARIFY` / pre-execution `NO_QUERY`，P0 fail-closed，不进入 narrator。
- P0 不提供 partial answer；带范围说明的 partial answer 必须后移到 P1 单独冻结用户可见语义。

### 4.8 Fanout 策略

P0 不引入 fanout DSL，不并发执行，也不由 query flow 维护业务专用待查队列。

- planner 可在下一轮基于 `latest_observation.items` 选择一个或一小批后续普通小 `ReadPlan`。
- query flow 只负责记录已执行 fingerprint、防止重复执行、暴露剩余预算；不根据 `has_children`、`parent_org_code` 或其他业务字段自行生成下一步。
- 多个候选后续调用存在时，知识包案例必须要求 planner 按 observation item 的出现顺序稳定处理；测试夹具也按该顺序断言。
- 若候选后续调用超出预算，P0 fail-closed，不进入 narrator；P1 若要允许 partial answer，必须单独方案冻结“已覆盖范围”的用户可见表达与验收。

## 5. 模块归属与职责边界

- **`internal/server/cubebox_query_flow.go`**
  - 持有 loop orchestration
  - 构造 planner 输入
  - 解析 planner outcome
  - 调用 executor
  - 累积 `working_results`
  - 对齐 `468C` 的 planner projection 与候选澄清事件契约
  - 执行预算、去重、错误映射和 SSE 事件顺序
  - 最终只调用 narrator 一次
- **`modules/cubebox`**
  - 保持 `ReadPlan` schema、校验、执行注册表、参数引用解析与执行结果类型
  - 增加通用 `working_results` / observation 构造所需的纯函数或 DTO 时，应保持业务无关
- **`modules/orgunit/presentation/cubebox`**
  - 更新知识包案例，告诉模型如何根据 `working_results` 中的 `has_children` 继续规划
  - 不声明新 API，不写回答模板，不引入业务 DSL
- **`third_party/openai-codex`**
  - 仅作为源码参考，不参与构建和运行

## 6. 失败路径与 Stopline

| 场景 | 处理 |
| --- | --- |
| planner 输出非法 JSON / 非法 outcome | fail-closed，映射为计划边界错误 |
| `ReadPlan` schema 或参数非法 | fail-closed，沿用现有计划边界错误 |
| 未注册 `api_key` | fail-closed，沿用执行注册表漂移错误 |
| executor 返回候选不可静默选择 | 进入澄清终态，并按 `468C` 契约写 `group_id` / `candidate_group_id` metadata events |
| executor 执行失败 | 沿用现有错误映射 |
| planner 在已执行后返回 `NO_QUERY` | fail-closed，不当作完成 |
| planner 返回 `DONE` 但无任何执行结果 | fail-closed |
| 超过预算或重复查询不收敛 | fail-closed，输出统一 stopline |
| narrator 输出泄露内部字段 | 沿用现有 narrator contract violation |

用户可见错误映射必须对齐 `DEV-PLAN-140` 与 `make check error-message`，新增稳定码时同步登记 catalog 与 `en/zh` 文案：

| Stopline | 稳定错误码建议 | 用户可见提示要求 |
| --- | --- | --- |
| 非法 planner outcome / 裸文本 `DONE` / 未知 outcome | `cubebox_query_planner_outcome_invalid` | 说明“未能形成可执行查询计划”，提示用户换一种说法或补充查询条件 |
| `DONE` 但没有任何执行结果 | `cubebox_query_done_without_result` | 说明“查询计划未产生可用结果”，提示补充查询条件 |
| 已执行后返回 `NO_QUERY` | `cubebox_query_no_query_after_execution` | 说明“查询计划在执行后偏离支持范围”，提示换一种说法重试 |
| planning / step 预算耗尽 | `cubebox_query_loop_budget_exceeded` | 说明“这次查询需要的步骤超出当前单轮预算”，提示缩小范围 |
| 重复 fingerprint 不收敛 | `cubebox_query_loop_repeated_plan` | 说明“查询计划重复且无法继续推进”，提示缩小范围或换一种说法 |
| narrator contract violation | 沿用现有码或登记 `cubebox_query_narrator_contract_violation` | 不暴露内部字段名、payload、step id 或 provider 细节 |

## 7. 测试设计与分层

| 层级 | 本计划承接内容 | 代表对象/文件 | 说明 |
| --- | --- | --- | --- |
| `modules/cubebox` | 业务无关 `working_results` DTO、fingerprint、预算、去重纯函数 | `modules/cubebox/*_test.go` | 优先黑盒测试 |
| `internal/server` | query loop、planner outcome、SSE 顺序、错误映射、narrator 单次调用 | `internal/server/cubebox_query_flow_test.go`、`internal/server/cubebox_api_test.go` | 组合层测试 |
| 知识包 | `working_results` 驱动再规划案例 | `modules/orgunit/presentation/cubebox/examples.md` | 配合 planner prompt 测试夹具 |
| E2E | 浏览器真实对话复验 | `docs/dev-records/DEV-PLAN-471-READINESS.md` | readiness 记录为 P0 证据 |

重点测试：

- 单步可完成问题仍只执行一次 executor、一次 narrator；planner 可按统一 loop 调用两次（`READ_PLAN` 后 `DONE`），不得引入“执行后自动完成”的隐藏分支。
- 需要“先执行再决定”的问题可在同一 turn 内进入第二次 planner。
- planner 看到 `working_results` 后返回 `DONE`，narrator 只调用一次。
- planner outcome 解析覆盖 JSON envelope、兼容裸 `ReadPlan` / `NO_QUERY`、拒绝裸文本 `DONE`。
- planner 在已执行后返回 `NO_QUERY` 必须 fail-closed。
- planner 重复请求相同 fingerprint 时不会重复执行。
- 超过 `max_planning_rounds` 时 fail-closed。
- budget 耗尽且未 `DONE` 时不进入 narrator，不产生 partial answer。
- 中间 `working_results` 不写入长期 canonical event。
- `modules/cubebox` 的 `working_results` 不生成 `orgunit` 专用待查队列、业务 winner 或 capability-specific 聚合字段。
- loop 每轮看到的 `query_dialogue_context` 使用 `468C` planner projection，且不吸收当前 turn observation。
- 候选澄清事件使用 `group_id` / `candidate_group_id` 串联，不得用 `candidate_source`、loop round 或 `turn_id` 当关联键。
- `resolved_entity` 不参与 471 loop 输入、状态推进或完成判定。
- 用户可见输出不泄露 `api_key`、`step-*`、`payload`、`results`、`params` 等内部执行痕迹。
- pre-execution `NO_QUERY` 输出由模型基于受控事实生成纯文本序号列表；默认示例为名称/关键词/关系型，不要求用户记编码，也不泄露 `NO_QUERY`、`ReadPlan`、`planner`、知识包或 `API` 等内部术语。

## 8. 实施步骤

按最小实现切片推进；每个切片都应保持“可独立评审、可单独回归、失败时可停在当前切片不继续放大范围”。

1. [ ] `PR-471-01`：冻结 planner outcome 与 `read_api_catalog`
   - 目标：先把“planner 到 query loop 的控制协议”收紧，再进入循环改造；避免一边改 loop 一边继续放宽 planner 输出语义。
   - 代码落点：
     - `internal/server/cubebox_query_flow.go`
     - `modules/cubebox/read_plan.go`
     - 如有必要，新增 `modules/cubebox/planner_outcome.go`
   - 具体动作：
     - 新增 JSON envelope planner outcome：`READ_PLAN` / `CLARIFY` / `DONE` / `NO_QUERY`
     - 兼容现有裸 `ReadPlan JSON` 作为 `READ_PLAN` / `CLARIFY`
     - 保留现有裸 `NO_QUERY`
     - 新增 JSON envelope `DONE`，明确拒绝裸文本 `DONE`
     - 澄清态继续沿用现有 `missing_params + clarifying_question`
     - 从 `ExecutionRegistry` 派生稳定排序的 `read_api_catalog` prompt block，并明确“注册表是执行事实源，知识包只解释语义”
   - 本片完成判定：
     - planner outcome 解析矩阵冻结
     - 非法 outcome 统一映射为计划边界错误
     - `read_api_catalog` 的输出稳定、可测试、与注册表一致

2. [ ] `PR-471-02`：新增 `working_results` / fingerprint / budget 纯函数
   - 目标：先把循环状态抽成业务无关纯函数，再让 `TryHandle(...)` 消费这些状态；避免把预算、聚合、去重逻辑写散在 server 组合层。
   - 代码落点：
     - 建议新增 `modules/cubebox/query_working_results.go`
     - 建议新增 `modules/cubebox/query_loop_budget.go`
     - 如无必要，不改 `modules/orgunit/**` 业务 executor
   - 具体动作：
     - 定义业务无关 `working_results` DTO：`budget`、`completed_plans`、`latest_observation`、`executed_fingerprints`、`repeat_observations`
     - 生成 `plan_fingerprint` / `step_fingerprint`
     - 维护 `round_index`、`executed_steps` 与剩余预算；不得维护 `remaining_parent_org_codes` 等业务专用待办队列
     - 对 observation 做条数与字段裁剪，禁止无限回灌 raw payload
     - 明确 `working_results` 仅存在于当前 `TryHandle(...)` 生命周期内，不写入 canonical events
   - 本片完成判定：
     - 可独立测试追加 observation、重复 fingerprint 检测、预算耗尽、裁剪结果
     - 可证明 `modules/cubebox` 不根据 `has_children` 或其他业务字段生成下一步

3. [ ] `PR-471-03`：把 `cubeboxQueryFlow.TryHandle(...)` 从单次链改成有限次 loop
   - 目标：把现有 `ProduceReadPlan -> ExecutePlan -> Narrate` 改成有限次 `ProduceReadPlan -> outcome -> ExecutePlan -> append working_results`，但仍只保留一个 narrator 终点。
   - 代码落点：
     - `internal/server/cubebox_query_flow.go`
     - `internal/server/cubebox_query_flow_test.go`
   - 具体动作：
     - 在 `TryHandle(...)` 内引入 `max_planning_rounds`、`max_executed_steps`、`max_repeated_plan_fingerprint` 限制
     - 每轮都重建 planner 输入：`knowledge packs + read_api_catalog + query_dialogue_context(按 468C planner projection) + current user prompt + working_results`
     - `READ_PLAN`：执行小 `ReadPlan`，追加 observation，进入下一轮 planner
     - `CLARIFY`：直接终止为澄清，不进入 narrator
     - `DONE`：仅在至少执行过一次只读计划后允许进入 narrator
    - `NO_QUERY`：未执行前进入 no-query guidance；已执行后返回 `NO_QUERY` 必须 fail-closed
     - 预算耗尽且未 `DONE` 时 fail-closed，不进入 narrator
     - executor 返回候选不可静默选择时，继续沿用现有澄清终态，但 metadata event 必须按 `468C` 契约写 `group_id` / `candidate_group_id`
   - 本片完成判定：
     - 单步问题仍只执行一次 executor、一次 narrator；planner 通过 `READ_PLAN` 后 `DONE` 的统一 loop 完成
     - 需要“先执行再决定”的问题可在同一 turn 内进入第二轮 planner
     - narrator 只在最终完成时调用一次

4. [ ] `PR-471-04`：扩展 planner prompt 与 orgunit 知识包案例
   - 目标：让模型知道“什么时候继续查、什么时候 `DONE`、什么时候绝不能再查重复 fingerprint”；不要把这些规则偷偷塞进 server if-else。
   - 代码落点：
     - `internal/server/cubebox_query_flow.go`
     - `modules/orgunit/presentation/cubebox/examples.md`
     - 如需补说明，可同步 `modules/orgunit/presentation/cubebox/apis.md`
   - 具体动作：
     - 在 planner system prompt 中增加 `working_results` 说明，并明确 `query_dialogue_context` 使用 `468C` 的跨 turn fact window
     - 明确 `DONE` 必须使用 JSON envelope，语义是“当前 observation 已足够进入 narrator”
     - 明确 `NO_QUERY` 只表示“超出查询域”，不能表示“已经查够”
     - 明确 `recent_confirmed_entities` / `recent_candidate_groups` 是跨 turn 历史事实主输入，`recent_confirmed_entity` / `recent_candidates` 只是 compatibility alias
     - 明确 `resolved_entity` 不在本计划输入范围
     - 明确禁止重复请求已执行 fingerprint
     - 在 orgunit 样例中加入“先查直接下级，再由 planner 阅读 observation item 的 `has_children=true` 字段并继续查其下级”的最小案例
     - 在 orgunit 知识包加入 `no_query_guidance` 结构化片段，提供 `scope_summary`、默认名称/关键词/关系示例，以及最近确认实体存在时的续问示例
   - 本片完成判定：
     - planner 提示词与知识包案例对齐
     - 不新增 API、不引入 DSL、不写业务专用分支 prompt

5. [ ] `PR-471-05`：补齐自动化测试，先锁死回归面再做真实复验
   - 目标：优先用最小直接测试锁死循环协议与 stopline，再进入页面复验；避免把问题推到浏览器层才暴露。
   - 代码落点：
     - `modules/cubebox/*_test.go`
     - `internal/server/cubebox_query_flow_test.go`
     - 如需组合层覆盖，可补 `internal/server/cubebox_api_test.go`
   - 重点覆盖：
     - planner outcome 解析：JSON envelope、兼容裸 `ReadPlan` / `NO_QUERY`、拒绝裸 `DONE` / 非法文本
     - `working_results` 构造、预算快照、裁剪、fingerprint
     - loop 正常完成：至少两轮 planner 后 `DONE`
     - loop 澄清终止：候选不可静默选择时直接终止，不进入 narrator
     - repeat / budget fail-closed：重复请求同 fingerprint 或超预算时不重复执行
     - P0 budget 耗尽不进入 narrator、不输出 partial answer
     - narrator 单次调用：中途不得输出用户可见半成品回答
     - 长期事件隔离：中间 `working_results` 不写入 canonical events
     - 业务隔离：`working_results` 不包含 `remaining_parent_org_codes`、业务 winner 或 capability-specific 聚合字段
     - 候选澄清事件契约：`turn.query_candidates.presented.group_id` 与 `turn.query_clarification.requested.candidate_group_id` 正确关联
     - `resolved_entity` 不参与 planner 输入或 loop 完成判定
     - pre-execution `NO_QUERY` guidance facts 与最终纯文本序号列表；覆盖无最近确认实体和有最近确认实体两种路径
   - 本片完成判定：
     - `modules/cubebox` 纯函数测试与 `internal/server` 组合层测试都能独立说明 471 的主约束

6. [ ] `PR-471-06`：真实页面复验与 readiness 证据
   - 目标：在自动化回归通过后，用真实页面验证“同一 turn 内二次规划”确实发生；Codex 源码 HEAD 只作为对标参考留痕，不阻断 P0 功能验收。
   - 代码/文档落点：
     - `docs/dev-records/DEV-PLAN-471-READINESS.md`
   - 具体动作：
     - 记录自动化测试命令、时间戳、结果
     - 记录浏览器真实复验：输入、页面表现、网络请求、截图
     - 若本地对标使用 `third_party/openai-codex`，记录参考源码版本：
       - `git -C third_party/openai-codex rev-parse HEAD`
       - `git -C third_party/openai-codex status --short`
   - 本片完成判定：
     - readiness 能证明 P0 样例已在真实页面闭环通过
     - 第三方参考源码 HEAD 若已记录，只作为后续偏差分析基线，不作为用户能力验收项

## 9. 验收口径

1. [ ] 用户问题需要“先看执行结果，再决定下一步”时，系统可在同一 turn 内自动完成至少两轮小计划。
2. [ ] 对“把那些有下级的下级组织的下级组织列出来”这类问题，模型可以通过已有 `orgunit.list` 能力自动分解并给出最终答案。
3. [ ] 不新增 `orgunit` 专用读 API，`orgunit` executor 注册表保持 `details / list / search / audit` 不变。
4. [ ] 不引入 DSL、JSONPath、脚本表达式、DAG/workflow engine 或 capability-specific 编排分支。
5. [ ] planner outcome wire format 冻结：新增终止态使用 JSON envelope；裸 `DONE` 被拒绝；裸 `ReadPlan` / `NO_QUERY` 仅为现有兼容输入。
6. [ ] narrator 只在所有小循环结束后调用一次。
7. [ ] 当前 turn 的 `working_results` 不写入长期 canonical event，不污染后续 turn 的 `query_dialogue_context`。
8. [ ] `working_results` 只包含业务无关 observation ledger、预算与 fingerprint，不包含 `orgunit` 专用待查队列、业务 winner 或 capability-specific 聚合字段。
9. [ ] 每轮 planner 输入中的 `query_dialogue_context` 以 `468C` planner projection 为 SSOT，且与 `working_results` 保持并列、分层、不互写。
10. [ ] 候选澄清事件按 `468C` 契约使用 `group_id` / `candidate_group_id` 关联；`candidate_source`、loop round 与 `turn_id` 不作为主关联键。
11. [ ] `resolved_entity` 不参与 471 的 planner 输入、loop 状态推进或完成判定。
12. [ ] 超预算、重复查询不收敛、非法 planner outcome 均 fail-closed，且 P0 不输出 partial answer。
13. [ ] 用户可见回答不泄露内部执行痕迹。
14. [ ] pre-execution `NO_QUERY` 用户可见输出为模型基于受控事实生成的纯文本序号列表；默认不展示编码型示例，有最近确认实体时优先展示“这个组织 / 它”的续问示例。
15. [ ] 新增用户可见 stopline 均登记稳定错误码、后端 message 与 `en/zh` 文案，并通过 `make check error-message`。

## 10. 风险与对策

| 风险 | 说明 | 对策 |
| --- | --- | --- |
| Codex 借鉴过度 | 误建开放式 agent/tool 平台 | 只借鉴 loop，不开放任意工具 |
| planner 循环失控 | 模型持续要求更多查询但不收敛 | 回合预算、step 预算、重复 fingerprint stopline |
| `NO_QUERY` 语义混淆 | 查询已足够时误用 `NO_QUERY` | 冻结 `DONE`，已执行后 `NO_QUERY` 视为边界错误 |
| 中间结果污染长期事实 | 当前 turn 工作态误写入会话事件 | `working_results` 仅内存存在，不进入 canonical events |
| narrator 过早输出 | 中途回答导致后续不能继续查 | narrator 只保留为最终阶段 |
| 结果回灌过宽 | 把过多 raw payload 喂回 planner | observation 裁剪、条数限制、字段预算 |
| 再次滑向业务 DSL | 为多轮编排引入复杂计划语言 | 只允许普通小 `ReadPlan` + 薄 outcome |
| fanout 超预算 | 多个 parent 都需继续查询 | P0 串行有限 fanout，预算耗尽 fail-closed |
| 通用 loop 被业务状态污染 | `working_results` 长出 `orgunit` 待查队列或业务 winner | query flow 只记录 observation 与 fingerprint，业务判断留给 planner + 知识包 |
| outcome wire format 漂移 | 新增 `DONE` 与非法文本解析混用 | 新增终止态统一 JSON envelope，裸 `DONE` fail-closed |
| partial answer 语义提前膨胀 | 预算耗尽时试图让 narrator 解释已覆盖范围 | P0 不输出 partial answer，P1 单独冻结范围表达 |

## 11. Readiness 与证据

- [ ] 新建 readiness：`docs/dev-records/DEV-PLAN-471-READINESS.md`
- [ ] 记录自动化测试命令、时间戳与结果
- [ ] 记录浏览器真实复验链路、截图与网络请求
- [ ] 若本地对标使用 `third_party/openai-codex`，记录 clone HEAD、命令、时间戳与结果；该记录是参考证据，不阻断 P0 功能验收
