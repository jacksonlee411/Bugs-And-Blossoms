# DEV-PLAN-500：CubeBox 两次相同查询会话差异与边界判定专项调查方案

**状态**: 已关闭（2026-05-05 12:37 CST；由 `DEV-PLAN-501` 交付并关闭）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：针对 `conv_0fc7637be99c47538e311860ffe972b2` 与 `conv_2d934635cd6449e48eb45ff4b9a0dddb` 两次内容相同但结果不同的 `CubeBox` 查询会话，冻结专项调查范围、证据口径、步骤分解、模型输入输出取证口径，以及调查期口语问题中所谓“允许范围/超出范围”的逐步审计方法。
- **关联模块/目录**：`internal/server/cubebox_query_flow.go`、`internal/server/cubebox_api.go`、`internal/server/cubebox_api_tool_runner.go`、`modules/cubebox/*`、`modules/orgunit/presentation/cubebox/*`、`apps/web/src/pages/cubebox/*`、`modules/cubebox/infrastructure/sqlc/*`
- **关联计划/标准**：`AGENTS.md`、`docs/dev-plans/000-docs-format.md`、`docs/dev-plans/001-technical-design-template.md`、`docs/dev-plans/003-simple-not-easy-review-guide.md`、`docs/dev-plans/012-ci-quality-gates.md`、`docs/dev-plans/460-cubebox-digital-assistant-positioning-and-execution-contract.md`、`docs/dev-plans/464-cubebox-query-architecture-convergence-plan.md`、`docs/dev-plans/473-cubebox-model-owned-query-context-input-remediation-plan.md`、`docs/dev-plans/474-cubebox-cross-turn-result-list-follow-up-plan.md`、`docs/dev-plans/490-cubebox-api-first-tooling-refactor-plan.md`
- **用户入口/触点**：`/internal/cubebox/turns:stream`、CubeBox 右侧抽屉、会话持久化回放、query planner、query runner、query narrator、前端流式恢复/重发链路

### 0.1 Simple > Easy 三问

1. **边界**：本调查只回答“这两次真实会话为什么出现不同结果”，不顺手把 planner 稳定性问题、UI 自动重试策略、query runtime 宽化改动直接并入本计划实施。
2. **不变量**：调查必须基于真实证据链，优先使用会话事件、真实落库记录、前端/后端当前实现和可复现实验；不得用“模型随机”“网络偶发”等笼统措辞替代证据。
3. **可解释**：reviewer 必须能在 5 分钟内讲清两次会话各自经历了哪些步骤、每一步调用了什么、模型是否参与、何时触发边界/协议校验、以及在哪一步被认定失败。

### 0.2 文档职责封口

1. `DEV-PLAN-500` 只定义专项调查协议、问题清单和取证口径；不得作为后续实现、错误分类或产品行为契约的 owner。
2. `DEV-PLAN-501` 是本调查协议的正式交付物，并已关闭本计划。
3. `DEV-PLAN-502` 是唯一实施入口，负责把调查期口语中的“允许范围/超出范围”正式替换为 typed contract 分类：`ModelOutputContract`、`ToolExecutionContract`、`RuntimeGuardrail`、`AuthorizationContract`、`TurnLifecycleContract`。
4. 本文件中保留的“允许范围/超出范围”仅用于对应用户原始问题和调查语境，不再作为内部工程模型名称。

### 0.3 现状研究摘要

- 当前 `CubeBox` 查询链采用 `cubebox-query-api-calls` runtime：planner 输出 `API_CALLS / CLARIFY / DONE / NO_QUERY`，执行面只允许当前注册的 HTTP API tool overlay。
- 当前会话会持久化到 `iam.cubebox_conversations` 与 `iam.cubebox_conversation_events`；query flow 每次开工前会回放整段 canonical events，构造 `QueryContext`。
- 当前前端 `streamTurn()` 要求流式过程中必须收到 `turn.completed` 或 `turn.error` 作为终态；若流结束但未见终态，前端本地 fail-closed 报 `stream turn failed: missing terminal event`。
- 调查期口语中的“允许范围/超出范围”并不是单一一处判定，而是由多层共同构成；长期工程分类见 `DEV-PLAN-502`：
  - planner outcome schema/边界校验
  - API call plan 线性约束校验
  - API tool 参数白名单/必填校验
  - query loop budget / repeated plan 限制
  - API tool catalog 与 authz/runtime 限制
- 当前已发现真实现象：同一句用户输入在两个不同 conversation 中，一个直接落 `ai_plan_boundary_violation`，另一个在存在前一轮未收口历史的情况下第二次重发后成功回答。

## 1. 背景与上下文

- **需求来源**：真实线上/本地运行态调查请求。
- **当前痛点**：同一用户输入在两个会话中的结果不一致，若只看最终回答或只看代码局部，无法回答“为什么一个失败、一个成功”。
- **业务价值**：给后续修复提供准确证据，避免把偶发症状误判为模型随机性或单点代码 bug。
- **仓库级约束**：
  - 调查必须遵循 Contract First，不得跳过现有 SSOT。
  - 必须基于真实 canonical events、query flow 和前端流式逻辑。
  - 不得以降低边界门禁或扩大 fallback 作为调查结论替代。

## 2. 目标与非目标

### 2.1 核心目标

1. [ ] 冻结两次会话的完整步骤时间线，回答“**一共进行了哪些步骤**”。
2. [ ] 对每个步骤列出实际调用的 API / 运行时功能 / 本地状态转换，回答“**每次步骤调用了什么 API 或其他功能**”。
3. [ ] 对每个步骤标明是否调用模型、调用的是 planner 还是 narrator，并记录是否存在“未调用模型直接返回”的分支，回答“**每次是否都调用了模型**”。
4. [ ] 对每次模型调用冻结输入材料与输出结果的取证方案，回答“**输入给模型的内容是什么，模型返回的内容是什么**”。
5. [ ] 对每个关键步骤冻结边界/协议校验来源、校验逻辑和失败判定条件，回答用户原始问题中“**每一步骤是如何计算当前的允许范围，又是如何判断超出范围的**”。
6. [ ] 输出一份最终调查报告，能够明确指出两次会话差异发生在哪些步骤、由哪些输入差异触发、由哪一层判定导致结果分叉。

### 2.2 非目标

- 不在本计划内直接修改 query planner / runner / narrator 行为。
- 不把本计划扩张成 CubeBox 全量稳定性治理路线图。
- 不把单次调查结论直接升级为产品契约；除非后续另起实施计划并补测试。
- 不把“可能是网络/模型随机”作为结论；若需要引用此类因素，必须有日志或实验支撑。
- 不在本计划内冻结 typed error category/subcode/source_layer；该职责由 `DEV-PLAN-502` 统一承接。

### 2.3 用户可见性交付

- **用户可见入口**：无新增 UI；交付物为专项调查文档与后续证据记录。
- **最小可操作闭环**：读者仅凭本计划与调查记录，即可复盘两次会话的差异链路，并为后续修复创建实施计划。

## 2.4 工具链与门禁（SSOT 引用）

- **命中触发器（勾选）**：
  - [ ] Go 代码（本计划只读代码，不实施）
  - [ ] `apps/web/**`（本计划只读代码，不实施）
  - [ ] i18n
  - [ ] DB Schema / Migration
  - [ ] sqlc
  - [ ] Routing
  - [ ] AuthN / Tenancy / RLS（本计划只读调查，不调整契约）
  - [ ] Authz（若调查中发现 capability 裁剪需补记）
  - [ ] E2E
  - [X] 文档 / readiness / 证据记录
  - [ ] 其他专项门禁：`error-message`（实施期由 `DEV-PLAN-502` 触发）

- **本次引用的 SSOT**：
  - `AGENTS.md`
  - `docs/dev-plans/000-docs-format.md`
  - `docs/dev-plans/001-technical-design-template.md`
  - `docs/dev-plans/003-simple-not-easy-review-guide.md`
  - `docs/dev-plans/012-ci-quality-gates.md`
  - `docs/dev-plans/460-cubebox-digital-assistant-positioning-and-execution-contract.md`
  - `docs/dev-plans/464-cubebox-query-architecture-convergence-plan.md`
  - `docs/dev-plans/473-cubebox-model-owned-query-context-input-remediation-plan.md`
  - `docs/dev-plans/474-cubebox-cross-turn-result-list-follow-up-plan.md`
  - `docs/dev-plans/490-cubebox-api-first-tooling-refactor-plan.md`
  - `Makefile`

## 2.5 测试设计与分层

| 层级 | 本计划承接内容 | 代表对象/文件 | 说明 |
| --- | --- | --- | --- |
| `modules/cubebox` | `QueryContext` 回放、evidence window、working results / budget、API call plan 约束 | `modules/cubebox/*.go` | 明确 query memory 和边界校验来源 |
| `internal/server` | query flow 编排、planner/narrator 调用点、terminal error 写入、API tool runner 校验 | `internal/server/cubebox_query_flow.go`、`internal/server/cubebox_api_tool_runner.go` | 明确“在哪里触发边界/协议校验、在哪里 fail-closed” |
| `apps/web/src/pages/cubebox` | `turns:stream` 流式消费、终态要求、悬空 turn 本地处理、会话重发行为 | `api.ts`、`CubeBoxProvider.tsx`、`reducer.ts` | 明确第一次请求未收口时前端如何影响第二次请求 |
| 数据证据 | `iam.cubebox_conversation_events` / `iam.cubebox_conversations` | PostgreSQL | 真实步骤与 payload SSOT |

## 3. 调查对象冻结

### 3.1 会话对象

1. `conv_0fc7637be99c47538e311860ffe972b2`
2. `conv_2d934635cd6449e48eb45ff4b9a0dddb`

### 3.2 用户输入对象

两次会话的调查输入统一冻结为：

`你好,请列出全部的财务组织,包括他们的基本信息/组织路径名称和审计信息`

### 3.3 对比原则

1. 必须把两次会话拆成**各自独立的步骤序列**，不能只比最终结果。
2. 必须同时比对：
   - 会话事件序列
   - query flow 内部状态推进
   - planner / runner / narrator 调用链
   - 前端流式状态机与重发行为
3. 必须明确区分：
   - 用户主动第二次发送
   - 前端因本地流式失败后再次发起
   - 后端自动 retry / planner correction / loop 内继续规划

## 4. 调查问题清单（必须逐条回答）

### 4.1 步骤清单

对两次会话分别输出：

1. 会话创建步骤
2. 第 N 次 `turns:stream` 请求
3. query flow 是否接管
4. planner 是否调用
5. planner 输出是什么
6. API plan 校验是否通过
7. runner 执行了哪些 step
8. narrator 是否调用
9. 最终写入了哪些 canonical events
10. 前端是否收到完整终态事件
11. 是否发生本地 fail-closed / 恢复 / 重发

### 4.2 每步调用清单

对每一步至少记录：

- 触发方（前端 / 后端 / 数据库 / 模型）
- 调用对象（HTTP API / Go 函数 / 数据库查询 / 模型接口）
- 输入
- 输出
- 是否持久化到 canonical events

### 4.3 模型参与清单

对每一步标注：

- `未调用模型`
- `调用 planner 模型`
- `调用 narrator 模型`

若调用模型，必须记录：

- provider / model slug
- 调用时机
- 输入材料来源
- 输出原文
- 输出经过了哪层 decode / validate / normalize

### 4.4 Typed Contract 边界与失败判定清单

对每个关键步骤必须回答：

1. 当前 typed contract 边界由哪些事实共同决定：
   - query evidence window
   - current API tools
   - working results
   - API call plan schema
   - query loop budget
   - authz/runtime
2. 当前步骤具体用了哪段代码做校验。
3. 触发失败时具体抛了哪个 error。
4. 该 error 如何映射到用户可见报错。

## 5. 实施步骤

### 5.1 阶段 A：会话证据冻结

1. [ ] 从 `iam.cubebox_conversations` 与 `iam.cubebox_conversation_events` 导出两次会话的完整事件序列。
2. [ ] 对每个事件标注：
   - `sequence`
   - `turn_id`
   - `event_type`
   - `payload`
   - `created_at`
3. [ ] 生成两份逐会话事件时间线和一份并排对照表。

### 5.2 阶段 B：步骤级运行链拆解

1. [ ] 依据 query flow、gateway、前端 provider/reducer，把每次会话拆成“步骤”。
2. [ ] 明确每一步对应的 HTTP API、Go 入口函数、数据库读写和模型调用点。
3. [ ] 对成功会话额外识别：
   - 首次请求是否悬空
   - 第二次相同输入是如何形成的
   - 第二次请求是否带入新的历史上下文

### 5.3 阶段 C：模型输入输出取证

1. [ ] 复原两次会话中每次 planner 调用的输入材料：
   - system prompt
   - knowledge packs
   - api_tools
   - query_evidence_window
   - working_results
   - 当前 user prompt
2. [ ] 取证 planner 原始输出：
   - 原始 JSON / 文本
   - decode 后 outcome
   - 若失败，失败发生在 decode、validate 还是 correction 后
3. [ ] 复原 narrator 调用输入与原始输出（若存在 narrator）。

### 5.4 阶段 D：Typed Contract 边界与失败判定链复盘

1. [ ] 为每个 planner/runner/narrator 关键步骤制作“typed contract 边界校验表”：
   - 输入事实
   - 校验规则
   - 所在代码位置
   - 结果
2. [ ] 对失败会话明确指出：
   - 哪一步第一次触发 typed contract 失败
   - 具体命中的 boundary 条件
   - 是否存在 planner correction 机会，为什么没有转成可执行 plan
3. [ ] 对成功会话明确指出：
   - 为什么没有命中相同 boundary
   - 哪一步开始与失败会话分叉
   - 分叉所需的新输入事实来自哪里

### 5.5 阶段 E：结论与后续建议

1. [ ] 输出“差异根因摘要”：
   - 不是差异项
   - 真正差异项
2. [ ] 输出“最小修复面建议”，但不在本计划内实施。
3. [ ] 若需修复，另起实施计划并关联本计划调查结论。

## 6. 交付物

1. [ ] 本调查方案：`docs/dev-plans/500-cubebox-two-conversations-boundary-investigation-plan.md`
2. [ ] Readiness / 证据记录：
   - `docs/dev-records/DEV-PLAN-500-READINESS.md`
   - 必须登记查询命令、时间、环境、结果摘要
3. [ ] 最终调查报告：
   - 两次会话步骤总表
   - API/功能调用矩阵
   - 模型调用矩阵
   - 模型输入输出证据
   - typed contract 边界/失败判定矩阵
   - 差异根因结论

## 7. 验收标准

1. [ ] 能完整回答两次对话**一共进行了哪些步骤**。
2. [ ] 能完整回答每次步骤**调用了什么 API 或其他功能**。
3. [ ] 能完整回答每次步骤**是否调用了模型**。
4. [ ] 能完整给出每次模型调用的**输入内容与返回内容**，且注明取证来源。
5. [ ] 能完整说明每一步是如何触发 typed contract 边界校验，以及如何判定失败；同时能回答用户原始问题中“允许范围/超出范围”的口语表述。
6. [ ] 能明确指出两次会话第一次分叉发生在哪一步、由什么输入差异触发、最终为何一个成功一个失败。

## 8. 风险与注意事项

- 若真实 provider 原始输入输出无法直接从现有日志回收，必须明确标注“现状缺少取证面”，并提出后续最小审计补点方案；不得伪造模型原文。
- 若会话历史中存在未收口 turn，必须同时从数据库事件与前端本地状态机两侧解释，不得只从一侧下结论。
- 若发现成功路径依赖偶发历史上下文污染、未收口流式请求或重复提交，必须在结论中明确标为“非稳定支持路径”。
