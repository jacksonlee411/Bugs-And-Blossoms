# DEV-PLAN-477：CubeBox `api_key -> executor_key` 契约改名专项方案

**状态**: 规划中（2026-04-27 18:36 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T1`
- **范围一句话**：把 CubeBox 查询链内部执行键从 `api_key` 统一改名为 `executor_key`，同步整理代码、知识包、测试与文档影响面，并冻结一次性切换策略，避免双字段长期兼容。
- **关联模块/目录**：`modules/cubebox`、`internal/server`、`modules/orgunit/presentation/cubebox`、`apps/web/src/pages/cubebox`、`docs/dev-plans`
- **关联计划/标准**：`AGENTS.md`、`docs/dev-plans/000-docs-format.md`、`docs/dev-plans/003-simple-not-easy-review-guide.md`、`docs/dev-plans/012-ci-quality-gates.md`、`docs/dev-plans/437a-cubebox-phase-a-canonical-conversation-contract.md`、`docs/dev-plans/461-cubebox-query-scenarios-minimal-contract.md`、`docs/dev-plans/468-cubebox-session-continuity-and-model-autonomy-improvement-plan.md`
- **用户入口/触点**：主应用壳层右侧 `CubeBox` 抽屉、`/internal/cubebox/turns:stream` 查询链、模块知识包加载链、canonical query events

### 0.1 Simple > Easy 三问

1. **边界**：本次只改 CubeBox 查询链内部执行键命名；真实 provider secret/API credential 命名不在本专项内。
2. **不变量**：运行时仍只有一个执行事实源；改名后不得出现 `api_key` / `executor_key` 双字段长期并存或双主链解析。
3. **可解释**：reviewer 应能在 5 分钟内说明哪些面需要原子改名、哪些同名 `APIKey` 明确不改、为何不能用长期兼容窗口拖住语义收口。

### 0.2 现状研究摘要

- **现状实现**：
  - `ReadPlanStep` 以 `json:"api_key"` 暴露执行键，`modules/cubebox/read_plan.go` 负责 schema 校验与 normalize。
  - `ExecutionRegistry` / `RegisteredExecutor` / `ExecuteResult` / `ReadAPICatalogEntry` / `QueryWorkingResults` 以 `APIKey` 或 `json:"api_key"` 在运行时内部传递执行键。
  - 知识包 `apis.md`、`examples.md` 以及 planner prompt 都把执行键叫作 `api_key`。
  - `QueryEntity` canonical event 里还保留 `source_api_key`。
- **现状约束**：
  - `api_key` 在查询链中不是 provider secret，而是只读执行注册表键；这一命名已在 `DEV-PLAN-468` 被认定会制造“密钥”误解。
  - 同仓还存在真实 provider credential 字段 `ProviderChatRequest.APIKey`、环境变量 `CUBEBOX_OPENAI_API_KEY`、`secret_ref` 等，这些与本专项不是一回事，不能被连带改名。
  - narrator 泄露防线已经同时拦 `api_key` 与 `executor_key` 文本，说明系统已经显式承认 `executor_key` 是候选内部命名，但主契约尚未切过去。
- **最容易出错的位置**：
  - planner 输出 schema、知识包示例与 read API catalog 若不同时切换，会造成模型侧 prompt 与运行时校验漂移。
  - `QueryEntity` 的 `source_api_key` 若不一起改，会在 canonical event / reducer test / prompt 视图里留下半套旧语义。
  - 若先做 decoder 兼容 `api_key` + `executor_key`，再无限期保留，会让 SSOT 再次退回“双字段别名窗口”。
- **本次不沿用的“容易做法”**：
  - 不采用长期“同时接受 `api_key` 和 `executor_key`”的兼容窗口。
  - 不新增 `read_api_key`、`tool_key` 之类第三命名。
  - 不把真实 provider `APIKey` 一并重命名成 `ExecutorKey`，避免把密钥语义和执行键语义再次混在一起。

## 1. 背景与上下文

- **需求来源**：用户要求单独整理 `api_key -> executor_key` 契约改名专项，明确影响面、切片顺序以及如何避免双字段长期兼容。
- **当前痛点**：
  - `api_key` 命名把内部执行键伪装成 secret/API key，误导人和模型。
  - 当前相关要求分散在 `DEV-PLAN-468` 的批判条目里，没有独立 owner 文档。
  - 若直接边写代码边扫命中面，容易漏掉知识包、prompt、working_results、canonical event 等隐性协议面。
- **业务价值**：
  - 收敛 CubeBox 查询链内部语言，降低“密钥误解”导致的安全/提示词噪音。
  - 为后续第二模块接入、知识包校验和查询链治理提供更清晰的执行键语义。
- **仓库级约束**：
  - 不引入 legacy / alias 长期兼容。
  - 契约先行，先落 dev-plan 再实施。
  - 用户可见输出仍不得暴露内部字段名；改名不改变这一防线。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 冻结 `api_key -> executor_key` 的正式改名范围，覆盖代码、知识包、测试、文档与 canonical event。
- [ ] 明确哪些同名 `APIKey` 不在本专项内，避免误改真实 provider secret 面。
- [ ] 冻结“一次性切换、短暂同 PR 原子迁移、无长期双字段兼容”的实施策略。
- [ ] 给出可执行切片，保证每片都能通过最小门禁并保持主链不漂移。

### 2.2 非目标

- 不在本计划内直接提交代码实现；本文件只冻结方案和切片。
- 不调整 provider credential、`secret_ref`、环境变量 `CUBEBOX_OPENAI_API_KEY`、HTTP Authorization header 或模型配置 UI 命名。
- 不改变执行注册表权限边界、RLS、查询 loop 预算、知识包目录结构。
- 不在本专项内引入第二套 planner DSL、tool registry 或新的 event 类型。

### 2.3 用户可见性交付

- **用户可见入口**：无新增 UI 入口；这是查询链内部契约收口专项。
- **最小可操作闭环**：实现后，用户仍通过现有 CubeBox 抽屉发起查询；内部 planner/executor 协议已经统一使用 `executor_key`，并且用户回答面继续不暴露内部字段。
- **当前如何验收不是“僵尸功能”**：
  - 文档冻结后，后续实现必须通过现有查询链测试与至少一条真实查询路径。
  - 改名前后都能解释清楚 planner 输入、知识包示例、执行注册表和 canonical event 的字段语义。

## 3. 关键决策

### 3.1 决策 1：正式命名统一为 `executor_key`

- 选定方案：CubeBox 查询链所有“内部执行键”统一改为 `executor_key`。
- 不选 `tool_key`：会误导为通用工具平台，而当前边界只是只读执行注册表。
- 不选 `read_api_key`：仍保留 `api_key` 误解，没有真正解决“密钥”混淆。

### 3.2 决策 2：`source_api_key` 一并改为 `source_executor_key`

- 选定方案：`QueryEntity` 及其事件 payload 同步改名为 `source_executor_key`。
- 原因：若主执行协议已改，而 query entity 仍保留 `source_api_key`，则 canonical event / prompt-view / reducer test 会残留半套旧语义，继续制造误导。

### 3.3 决策 3：真实 secret/API credential 命名不在本专项内

- 明确保留：
  - `modules/cubebox/gateway.go` / `health.go` / `internal/server/cubebox_query_flow.go` 中 `ProviderChatRequest.APIKey`
  - `AGENTS.md` 中 `CUBEBOX_OPENAI_API_KEY`
  - runtime/config/api DTO 中 `secret_ref`
- 原因：这些字段表达的就是真实 provider secret，不是执行注册表键；若一并改动，会把两个问题重新混为一谈。

### 3.4 决策 4：禁止长期双字段兼容

- 不允许长期同时接受：
  - `steps[].api_key` 与 `steps[].executor_key`
  - `apis.md` 中 `api_key` 与 `executor_key`
  - `source_api_key` 与 `source_executor_key`
- 推荐策略：
  - 在同一实现 PR 中完成 schema、知识包、执行注册表、测试与主要文档的原子切换。
  - 如确因单个 PR 体积需要分片，允许“文档先行 PR”与“代码原子切换 PR”两片；代码切换 PR 内仍不得引入长期 decoder alias。
  - 若必须存在极短暂兼容，仅允许在同一 PR 内用于迁移测试夹层，并在 PR 结束前删除，不得进入 main 长存。

## 4. 影响面清单

### 4.1 必改代码面

1. `modules/cubebox/read_plan.go`
   - `ReadPlanStep.APIKey` -> `ExecutorKey`
   - `json:"api_key"` -> `json:"executor_key"`
   - 错误文案、校验提示、normalize 逻辑同步改名
2. `modules/cubebox/read_executor.go`
   - `ExecuteResult.APIKey` -> `ExecutorKey`
   - `RegisteredExecutor.APIKey` -> `ExecutorKey`
   - registry `Register/Resolve/RegisteredExecutors/ExecutePlan` 文案与变量名同步改名
   - `resolveReadPlanResultField(...)` 中 `case "api_key":` 改为 `case "executor_key":`
3. `modules/cubebox/read_api_catalog.go`
   - `ReadAPICatalogEntry.APIKey` -> `ExecutorKey`
   - `json:"api_key"` -> `json:"executor_key"`
4. `modules/cubebox/knowledge_pack.go`
   - `knowledgePackAPIsDoc` YAML 字段从 `api_key` 改为 `executor_key`
   - `ValidateKnowledgePack(...)` 与 `ValidateKnowledgePackAgainstRegistry(...)` 的错误消息、drift 比对、字段解释同步改名
5. `modules/cubebox/query_working_results.go`
   - `QueryCompletedPlanStep.APIKey` / `QueryWorkingObservation.APIKey` -> `ExecutorKey`
   - JSON 字段从 `api_key` 改为 `executor_key`
   - fingerprint、summary、prompt block 相关结构同步改名
6. `modules/cubebox/query_entity.go`
   - `SourceAPIKey` -> `SourceExecutorKey`
   - JSON 字段从 `source_api_key` 改为 `source_executor_key`
   - 编解码 helper 与 payload 投影同步改名
7. `internal/server/cubebox_query_flow.go`
   - forbidden pattern 继续拦内部字段，但主提示、planner 使用规则、narrator 约束、query catalog 文本、working_results 文本改为 `executor_key`
   - `buildReadAPICatalogPromptBlock(...)` 中 `steps[].api_key` 改为 `steps[].executor_key`
   - 任何 `step.APIKey` 本地变量改为 `step.ExecutorKey`
8. `internal/server/cubebox_orgunit_executors.go`
   - 注册表样板字段名 `APIKey` -> `ExecutorKey`

### 4.2 必改测试面

1. `modules/cubebox/read_plan_test.go`
2. `modules/cubebox/read_executor_test.go`
3. `modules/cubebox/knowledge_pack_test.go`
4. `modules/cubebox/query_entity_test.go`
5. `modules/cubebox/query_working_results_test.go`
6. `modules/cubebox/planner_outcome_test.go`
7. `internal/server/cubebox_query_flow_test.go`
8. `internal/server/cubebox_orgunit_executors_test.go`
9. `internal/server/cubebox_api_test.go`
10. `apps/web/src/pages/cubebox/reducer.test.ts`

### 4.3 必改知识包面

1. `modules/orgunit/presentation/cubebox/apis.md`
   - YAML 键名 `api_key` -> `executor_key`
   - 解释文案从“`api_key -> executor` 注册表”改为“`executor_key -> executor` 注册表”
2. `modules/orgunit/presentation/cubebox/examples.md`
   - 所有 `ReadPlan` / planner 示例中的 `api_key` 改为 `executor_key`
3. `modules/orgunit/presentation/cubebox/CUBEBOX-SKILL.md`
   - “允许的 `api_key`”等说明统一改成 `executor_key`

### 4.4 必改文档契约面

1. `docs/dev-plans/461-cubebox-query-scenarios-minimal-contract.md`
   - 把正式协议描述从 `api_key -> executor` 更新为 `executor_key -> executor`
   - `ReadPlan` JSON schema 示例改名
2. `docs/dev-plans/464-cubebox-query-architecture-convergence-plan.md`
3. `docs/dev-plans/465-cubebox-orgunit-executor-contract-boundary-and-field-owner-convergence-plan.md`
4. `docs/dev-plans/466-cubebox-query-owner-drift-and-anti-backflow-investigation-plan.md`
5. `docs/dev-plans/468-cubebox-session-continuity-and-model-autonomy-improvement-plan.md`
   - 改为引用本专项 owner，并把 “评估改名” 调整为“按 `DEV-PLAN-477` 承接”
6. `docs/dev-plans/471-cubebox-intra-turn-iterative-read-planning-plan.md`
7. `docs/dev-plans/468c-cubebox-query-context-fact-window-plan.md`
8. `docs/dev-plans/437a-cubebox-phase-a-canonical-conversation-contract.md`
   - `turn.query_entity.confirmed` payload 中 `entity.source_api_key` -> `entity.source_executor_key`

### 4.5 可后置的历史/证据文档面

- `docs/dev-records/DEV-PLAN-468-READINESS.md`
- `docs/dev-records/assets/dev-plan-468/*.json`
- 其他只作历史证据引用、不会再作为当前契约输入的 readiness / archive 文档

说明：这些文件不必阻塞首个实现 PR；若内容作为历史证据保留，可在后续“文档扫尾 PR”中统一改注释或补注，不要求重写所有历史样本 JSON。

### 4.6 明确不改面

1. `modules/cubebox/gateway.go` / `health.go` / provider adapter 中 `ProviderChatRequest.APIKey`
2. 运行时配置、数据库、sqlc、API DTO 中的 `secret_ref`
3. 环境变量 `CUBEBOX_OPENAI_API_KEY`
4. 任何真实外部模型 provider secret/credential 命名

## 5. 切片方案

### Slice A：文档 owner 冻结

1. [ ] 新建本专项文档 `DEV-PLAN-477`
2. [ ] `AGENTS.md` 文档地图补充入口
3. [ ] `DEV-PLAN-468` 把“评估 `api_key -> executor_key`”改为引用本专项

验收：

- `make check doc`
- reviewer 能直接从 `AGENTS.md -> DEV-PLAN-477` 找到 owner

### Slice B：运行时结构原子改名

1. [ ] `ReadPlanStep` / `RegisteredExecutor` / `ExecuteResult` / `ReadAPICatalogEntry` / `QueryWorkingResults*` 的字段名与 JSON 字段统一切到 `executor_key`
2. [ ] 所有运行时错误提示、drift 校验、param 引用别名从 `api_key` 改为 `executor_key`
3. [ ] 不保留 `json:"api_key"` 的 decoder alias

验收：

- `go test ./modules/cubebox/... ./internal/server/...`
- 未注册执行键仍命中 `api_catalog_drift_or_executor_missing`
- 所有 planner/registry/working_results 测试样本只使用 `executor_key`

### Slice C：知识包与 prompt 协议同步改名

1. [ ] `apis.md` / `examples.md` / `CUBEBOX-SKILL.md` 统一改名
2. [ ] planner prompt、read_api_catalog prompt、narrator 禁止泄露文案改名
3. [ ] `ValidateKnowledgePackAgainstRegistry(...)` 继续做 fail-closed 一致性校验

验收：

- `go test ./modules/cubebox/... ./internal/server/...`
- 真实 planner prompt 中不再出现“steps[].api_key 必须来自 read_api_catalog”

### Slice D：canonical event 与前端测试扫尾

1. [ ] `source_api_key -> source_executor_key`
2. [ ] `QueryEntity` 相关编码/解码/最小 prompt 视图同步改名
3. [ ] `apps/web/src/pages/cubebox/reducer.test.ts` 与 server 流测试同步更新

验收：

- `pnpm --dir apps/web exec vitest run src/pages/cubebox/reducer.test.ts`
- `go test ./modules/cubebox/... ./internal/server/...`
- prompt-facing entity context 不再出现 `source_api_key`

### Slice E：现行契约文档扫尾

1. [ ] 更新 `437A/461/464/465/466/468/468C/471` 等活体计划中的正式字段表述
2. [ ] 历史 readiness / assets 只在需要时补充“历史字段名说明”，不强制重写原始证据

验收：

- `make check doc`
- 活体计划不再把 `api_key` 当正式 contract 字段

## 6. 双字段兼容禁止策略

1. [ ] 不接受“先支持新字段，再长期保留旧字段”的方案。
2. [ ] 不接受“知识包先改、代码后兼容旧字段”的长期窗口。
3. [ ] 不接受 `source_api_key` 保留为“历史兼容字段”的说法；它和主执行键同属内部契约，必须一起收口。
4. [ ] 若实现期担心一次性改太多，正确拆法是“文档先行 + 代码原子切换”，而不是“运行时双字段并存”。
5. [ ] 若必须短暂保留 alias 做迁移验证，必须满足：
   - 只存在于单个实现 PR 的中间 commit
   - merge 前删除
   - 不能进入 `main`

## 7. 测试与验证

- **Go 层**：
  - `go test ./modules/cubebox/...`
  - `go test ./internal/server/...`
- **前端测试**：
  - `pnpm --dir apps/web exec vitest run src/pages/cubebox/reducer.test.ts`
- **文档门禁**：
  - `make check doc`

重点验证：

1. `DecodeReadPlan(...)` 只接受 `executor_key`。
2. knowledge pack `apis.md` / `examples.md` 与 registry 一致性仍 fail-closed。
3. `QueryEntity` 事件的最小可继承字段已经切到 `source_executor_key`。
4. narrator / forbidden pattern 继续拒绝内部字段泄露，但正式文本与测试基线改为 `executor_key`。
5. 真实 provider secret 命名完全不受影响。

## 8. 实施建议

1. 推荐先合入本计划与引用更新。
2. 代码实现时优先走单 PR 原子切换 `Slice B + Slice C + Slice D`。
3. 若代码实现 PR 过大，可拆为：
   - PR-1：运行时结构 + 测试原子改名
   - PR-2：知识包、活体文档与前端测试扫尾
4. 任何拆分都不得让 `main` 进入“部分文件用 `executor_key`，部分运行时仍要求 `api_key`”状态。

## 9. 当前执行记录

1. [X] 2026-04-27：完成全仓 `api_key` / `executor_key` 影响面扫描，确认代码主命中集中在 `modules/cubebox`、`internal/server`、`modules/orgunit/presentation/cubebox` 与多份 CubeBox 活体计划。
2. [X] 2026-04-27：明确真实 provider secret 面（`ProviderChatRequest.APIKey`、`CUBEBOX_OPENAI_API_KEY`、`secret_ref`）不纳入本专项。
3. [ ] 后续实现 PR 需按本专项切片推进，并在对应 readiness 文档记录执行结果。
