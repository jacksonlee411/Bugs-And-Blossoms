# DEV-PLAN-502：CubeBox 查询边界契约与 Turn 生命周期收敛方案

**状态**: 规划中（2026-05-05 07:33 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：承接 `DEV-PLAN-500/501` 的真实会话调查结论和本地 `openai/codex` 资源对照评估，把当前被笼统称为“允许范围”的多层失败语义收敛为可审计、可重放、可定位的 typed query boundary contract 与 `turn` 生命周期契约。
- **关联模块/目录**：`internal/server/cubebox_query_flow.go`、`internal/server/cubebox_api_tool_runner.go`、`modules/cubebox/*`、`apps/web/src/pages/cubebox/*`、`modules/orgunit/presentation/cubebox/*`
- **关联计划/标准**：`AGENTS.md`、`docs/dev-plans/003-simple-not-easy-review-guide.md`、`docs/dev-plans/431-codex-ui-protocol-and-shell-reuse-plan.md`、`docs/dev-plans/432-codex-session-persistence-reuse-plan.md`、`docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md`、`docs/dev-plans/437a-cubebox-phase-a-canonical-conversation-contract.md`、`docs/dev-plans/464-cubebox-query-architecture-convergence-plan.md`、`docs/dev-plans/490-cubebox-api-first-tooling-refactor-plan.md`、`docs/dev-plans/500-cubebox-two-conversations-boundary-investigation-plan.md`、`docs/dev-plans/501-cubebox-two-conversations-boundary-investigation-report.md`
- **本地 Codex 对照口径**：`third_party/openai-codex` 当前本地提交 `5591912f0bf176257f71b3efbd37ee4479dfdfaf`；本计划只借鉴协议与边界设计，不引入 Codex runtime、CLI/TUI、exec、patch、plugin 或 MCP 写操作能力。

### 0.1 文档职责封口

1. `DEV-PLAN-502` 是本组文档的唯一实施入口与长期 owner。
2. `DEV-PLAN-500` 只保留调查协议职责，`DEV-PLAN-501` 只保留事实报告职责。
3. 本计划是唯一把调查期口语中的“允许范围/超出范围”替换为 typed contract 分类的文档，不再在其他 500/501 文档中重复定义长期术语。
4. 后续代码、测试、采样、前端恢复语义与 error mapping 的实施路径统一跟随本计划推进。

### 0.2 Simple > Easy 三问

1. **边界**：本计划只解决 boundary contract、错误分类、事件闭环和可观测性；不顺手重写 planner、不放宽工具白名单、不新增自动 retry 策略。
2. **不变量**：相同用户输入是否能成功，必须由当前权威上下文、工具目录、权限、预算和模型输出契约共同决定；任何失败都必须能追溯到唯一主分类和具体子分类，不得继续只落到模糊的 `ai_plan_boundary_violation`。
3. **可解释**：reviewer 必须能在 5 分钟内说明某次失败属于 `ModelOutputContract`、`ToolExecutionContract`、`RuntimeGuardrail`、`AuthorizationContract` 还是 `TurnLifecycleContract`，以及它如何写入事件、如何回放、如何展示。

## 1. 背景与问题陈述

`DEV-PLAN-500/501` 已证实：

1. 两次相同输入的会话差异不来自租户、principal 或模型配置切换。
2. 失败会话首轮在 planner/plan contract 层落 `ai_plan_boundary_violation`。
3. 成功会话不是首轮直接成功，而是第一次同句请求悬空，第二次同句请求带入前一次残留 `turn.user_message.accepted` 历史后成功。
4. 当前没有持久化 planner/narrator raw request/response，无法把失败细分到 raw JSON 子型。

本地 `openai/codex` 对照显示，Codex 同样采用多层边界：模型输出协议、tool item 生命周期、动态工具 schema 校验、approval/sandbox safety、上下文预算和 compaction、turn completed/aborted/error 状态机。差异在于 Codex 倾向用 typed event + typed error + history reducer 重建状态，而 CubeBox 当前把多种失败语义压到“允许范围/超出范围”这一笼统说法里。

因此，`DEV-PLAN-500` 的拆层作为调查方法是合理的；但作为长期架构命名和错误模型，需要收敛。

## 2. 目标与非目标

### 2.1 目标

1. [ ] 将调查期口语中的“允许范围”改写为明确的 typed contract 分类。
2. [ ] 把 planner 输出失败、plan shape 失败、tool 参数失败、预算失败、权限失败、stream terminal 失败拆成稳定错误主类与子类。
3. [ ] 保证每个后端 `turn` 要么完成、失败、被中断，要么能被恢复/标记为悬空异常；不得继续静默留下不可解释的半截状态。
4. [ ] 为 planner/narrator 增加受控可审计 raw request/response 采样能力，至少本地与排障环境可追。
5. [ ] 让前端 reducer 与恢复链路消费同一套 typed lifecycle，不再把缺 terminal event 混同为 query boundary。
6. [ ] 补齐最小测试与 readiness 证据，覆盖 `DEV-PLAN-501` 暴露的“首轮失败或悬空、二次发送成功”路径。

### 2.2 非目标

- 不放宽 planner contract、tool schema、authz 或 runtime guardrail。
- 不新增“失败后自动重试”作为默认产品行为。
- 不把 Codex 的 shell/file/patch/exec/plugin/MCP 写操作能力引入 CubeBox。
- 不把 planner raw 输出永久作为业务审计事实；raw 采样只用于受控排障，不替代 canonical event。
- 不把本计划扩张成全量 CubeBox query runtime 重写。

## 3. 目标分层模型

### 3.1 新主分类

| 主分类 | 负责什么 | 代表失败 | 是否属于用户问题“超范围” |
| --- | --- | --- | --- |
| `ModelOutputContract` | 模型 planner/narrator 输出是否满足协议 | JSON decode、outcome 无效、narrator 输出约束失败 | 部分是 |
| `ToolExecutionContract` | API plan 和 tool 调用是否符合工具目录与 schema | 非线性 plan、未知 tool、未知参数、必填缺失 | 是 |
| `RuntimeGuardrail` | 查询循环、预算、重复 plan、上下文窗口 | planning round 超限、repeated plan、working result 超限 | 否，属于运行时保护 |
| `AuthorizationContract` | 租户、principal、capability、route requirement | authz deny、capability/catalog drift | 否，属于权限边界 |
| `TurnLifecycleContract` | 流式协议与事件闭环 | 缺 terminal event、悬空 turn、interrupt/abort/replaced | 否，属于生命周期协议 |

### 3.2 Owner 边界表

| 事项 | Owner | 本计划职责 |
| --- | --- | --- |
| 调查协议与原始问题拆解 | `DEV-PLAN-500` | 只引用，不再修改调查协议语义 |
| 事实报告与证据缺口 | `DEV-PLAN-501` | 只引用事实结论和已标注缺口 |
| typed error 分类与映射 | `DEV-PLAN-502` | 唯一 owner |
| `turn.error` payload 增量字段 | `DEV-PLAN-502` + `DEV-PLAN-437A` | 本计划定义字段语义；若触及 envelope，由 `DEV-PLAN-437A` 裁决 |
| 前端 reducer / typed lifecycle 消费 | `DEV-PLAN-502` + `DEV-PLAN-431/437A` | 本计划定义 failure 分类与验收；UI 协议形态沿用既有 owner |
| context budget / compaction | `DEV-PLAN-434` | 本计划只引用预算失败分类，不改 compaction 策略 |
| API tool registry / overlay | `DEV-PLAN-490` | 本计划只细分 tool contract 失败，不扩张工具目录 |
| raw request/response 采样 | `DEV-PLAN-502` | 首期仅限本地/排障采样；若落库必须另起数据契约计划 |

### 3.3 命名裁决

1. “允许范围”仅作为用户可见自然语言提示保留，不再作为内部主模型名称。
2. 内部错误、日志、事件 payload 使用上表主分类。
3. `ai_plan_boundary_violation` 保留为兼容期用户可见 code 的候选，但必须增加内部 `category/subcode/source_layer`，避免排障时丢失子型。
4. 前端 `stream turn failed: missing terminal event` 归入 `TurnLifecycleContract`，不得继续归入 query plan boundary。
5. `boundary` 在本计划内只表示 typed contract 边界，不再表示一个笼统的“允许范围”总桶。

## 4. 事件与错误契约

### 4.1 Terminal event 不变量

每次 `POST /internal/cubebox/turns:stream` 后端接管并写入 `turn.started` 后，必须满足以下之一：

1. 写入 `turn.completed(status=completed)`。
2. 写入 `turn.error` + `turn.completed(status=failed)`。
3. 写入 `turn.interrupted` 或等价 abort/replaced 事件，并能被恢复链路识别。
4. 若进程中断导致未写 terminal event，恢复链路必须能把该 turn 标记为 `lifecycle_orphaned`，并给出稳定用户可见提示。

### 4.2 Terminal error payload

`turn.error` payload 至少应具备：

| 字段 | 说明 |
| --- | --- |
| `code` | 现有用户可见错误码，保持兼容 |
| `category` | `ModelOutputContract` / `ToolExecutionContract` / `RuntimeGuardrail` / `AuthorizationContract` / `TurnLifecycleContract` |
| `subcode` | 可稳定测试的具体子类 |
| `source_layer` | `planner` / `plan_shape` / `tool_schema` / `tool_runner` / `budget` / `authz` / `stream_lifecycle` / `narrator` |
| `retryable` | 是否建议用户重试 |
| `trace_id` | 与 `turn.started` 对齐 |

### 4.3 建议子类

| category | subcode |
| --- | --- |
| `ModelOutputContract` | `planner_outcome_invalid`、`planner_schema_decode_failed`、`narrator_contract_violation`、`narrator_target_mismatch` |
| `ToolExecutionContract` | `plan_empty`、`plan_non_linear`、`tool_not_registered`、`tool_param_unknown`、`tool_param_required_missing`、`tool_param_type_invalid` |
| `RuntimeGuardrail` | `planning_round_budget_exceeded`、`executed_step_budget_exceeded`、`working_result_budget_exceeded`、`repeated_plan_detected` |
| `AuthorizationContract` | `capability_denied`、`route_requirement_mismatch`、`catalog_drift`、`tenant_missing`、`principal_missing` |
| `TurnLifecycleContract` | `terminal_event_missing`、`turn_orphaned_on_restore`、`turn_interrupted`、`turn_replaced` |

## 5. Codex 对照采纳裁决

| Codex 机制 | CubeBox 采纳方式 |
| --- | --- |
| `TurnStarted` / `TurnComplete` / `TurnAborted` | 采纳生命周期模型，映射为 CubeBox `turn.started/completed/interrupted/orphaned` |
| `ItemStarted` / `ItemCompleted` | 借鉴，用于后续工具执行可见化；首期不要求 UI 展示每个 API step |
| typed `ErrorEvent` / `CodexErrorInfo` | 采纳 typed error 思路，转换为 `category/subcode/source_layer` |
| dynamic tool name/schema validation | 采纳失败分类思路，落到 `ToolExecutionContract` |
| approval/sandbox safety | 不直接引入；对应本仓 `AuthorizationContract` 与 authz/runtime fail-closed |
| context budget/compaction/canonical reinjection | 已由 `DEV-PLAN-434` 承接；本计划只引用预算失败分类 |
| history reducer | 继续由 `DEV-PLAN-431/437A` 承接；本计划要求 reducer 能识别 typed lifecycle |

## 6. 实施步骤

### 阶段 A：契约冻结

1. [ ] 在 `modules/cubebox` 增加或收敛 query terminal error 类型，冻结 `category/subcode/source_layer` 枚举。
2. [ ] 更新 `DEV-PLAN-437A` 或引用其事件 envelope，确认 `turn.error` payload 扩展字段不会形成第二套事件模型。
3. [ ] 明确兼容策略：用户可见 `code/message` 保持稳定，内部新增字段用于审计和测试。

### 阶段 B：后端错误映射收敛

1. [ ] 拆分 `queryPlanErrorToTerminal()`、`queryExecutionErrorToTerminal()`、`queryPlannerErrorToTerminal()` 的内部分类。
2. [ ] 将 `ValidateAPICallPlan()` 的失败细分为 plan shape 子类，不再全部只暴露为无差别 boundary。
3. [ ] 将 `validateAPICallParams()` 的参数错误细分为 tool schema 子类。
4. [ ] 将 budget/repeated plan 统一归入 `RuntimeGuardrail`。
5. [ ] 将 authz/catalog drift 统一归入 `AuthorizationContract`。

### 阶段 C：Turn 生命周期闭环

1. [ ] 审计 `turn.started` 写入后所有 return/error 路径，确保 terminal event append-first。
2. [ ] 为恢复链路增加 orphaned turn 识别与稳定 UI 状态。
3. [ ] 前端 `streamTurn()` 缺 terminal event 时产生 typed lifecycle failure，而不是普通 stream string error。
4. [ ] 禁止把 orphaned turn 的 `turn.user_message.accepted` 静默作为下一轮 planner 的普通历史事实使用；必须明确标记为未完成上下文。

### 阶段 D：可观测性与 raw 采样

1. [ ] 为 planner/narrator 增加受控 raw request/response 采样开关，默认关闭。
2. [ ] raw 样本写入 `.local/` 或受控 dev-record，不进入 canonical event 与生产默认数据面。
3. [ ] 样本必须带 `trace_id/conversation_id/turn_id/category/subcode`，并脱敏 secret。
4. [ ] 复现 `DEV-PLAN-501` 两会话时，能说明失败具体子型。

### 阶段 E：测试与验证

1. [ ] 单测覆盖 error category/subcode 映射。
2. [ ] 单测覆盖 plan shape 与 tool schema 子类。
3. [ ] 前端 reducer/API 测试覆盖 missing terminal event -> `TurnLifecycleContract`。
4. [ ] 集成或 E2E 覆盖首轮悬空恢复标记，不允许静默污染下一轮 query context。
5. [ ] 文档更新后执行 `make check doc`；涉及 Go/前端实现时按 `AGENTS.md` 触发器补跑对应门禁。

## 7. 验收标准

1. [ ] `DEV-PLAN-501` 中的失败会话可被分类到明确 `category/subcode/source_layer`，不再只能说 `ai_plan_boundary_violation`。
2. [ ] 缺 terminal event 的 turn 在恢复后有稳定 typed 状态，前端与后端说法一致。
3. [ ] 同句二次发送若带入未完成历史，planner 输入中必须能看出该历史是 orphaned/incomplete，而不是普通已完成上下文。
4. [ ] `turn.error` 对用户保持清晰提示，对工程排障提供足够子类信息。
5. [ ] 没有新增 legacy fallback、双链路或自动 retry 默认路径。

## 8. 风险与停止线

- 若实现需要修改 canonical event envelope，必须先对齐 `DEV-PLAN-437A`，不得让 query runtime 单独长出第二套事件协议。
- 若 raw 采样需要落库，必须另起数据契约计划；本计划首期只允许受控本地/排障采样。
- 若发现 planner 需要大改 prompt 或 tool overlay，本计划只记录 owner，不直接扩大范围。
- 若为修复悬空 turn 引入自动 retry，必须另起产品交互计划并经用户确认。

## 9. 关联文档

1. 调查方案：`docs/dev-plans/500-cubebox-two-conversations-boundary-investigation-plan.md`
2. 正式调查报告：`docs/dev-plans/501-cubebox-two-conversations-boundary-investigation-report.md`
3. Codex UI 协议复用方案：`docs/dev-plans/431-codex-ui-protocol-and-shell-reuse-plan.md`
4. Codex 会话持久化与恢复复用方案：`docs/dev-plans/432-codex-session-persistence-reuse-plan.md`
5. Codex context/compaction 复用方案：`docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md`
6. CubeBox Phase A canonical contract：`docs/dev-plans/437a-cubebox-phase-a-canonical-conversation-contract.md`
