# DEV-PLAN-466：CubeBox 查询链 owner 漂移与反回流扩大调查方案

**状态**: 规划中（2026-04-25 08:12 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：承接 `DEV-PLAN-465` 对 `orgunit.details` 的局部收敛，扩大调查 `CubeBox` 查询链中仍残留的 owner 漂移、重复契约、模块私有语义回流与 fail-closed 缺口，并冻结后续整改优先级与 stopline。
- **关联模块/目录**：`docs/dev-plans/460-cubebox-digital-assistant-positioning-and-execution-contract.md`、`docs/dev-plans/461-cubebox-query-scenarios-minimal-contract.md`、`docs/dev-plans/462-cubebox-codex-compaction-adoption-value-and-unified-convergence-plan.md`、`docs/dev-plans/463-cubebox-orgunit-tree-discovery-gap-investigation-and-remediation-plan.md`、`docs/dev-plans/464-cubebox-query-architecture-convergence-plan.md`、`docs/dev-plans/465-cubebox-orgunit-executor-contract-boundary-and-field-owner-convergence-plan.md`、`internal/server`、`modules/cubebox`、`modules/orgunit/presentation/cubebox`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-003`、`DEV-PLAN-460`、`DEV-PLAN-461`、`DEV-PLAN-462`、`DEV-PLAN-463`、`DEV-PLAN-464`、`DEV-PLAN-465`
- **用户入口/触点**：Web Shell 右侧 `CubeBox` 抽屉、`/internal/cubebox/turns:stream`、`internal/server/cubebox_query_flow.go`、`internal/server/cubebox_orgunit_executors.go`、`modules/cubebox/*`、`modules/orgunit/presentation/cubebox/*`

### 0.1 Simple > Easy 三问

1. **边界**：本计划解决的是 `CubeBox` 查询链里残留的 owner 漂移与重复实现，不是把查询链再扩成新平台、新 DSL 或第二套模块 runtime。
2. **不变量**：查询链仍必须遵守 `460/461/464` 已冻结的边界：模型负责语义理解、缺参澄清与结果叙述；本地代码只负责权限/租户边界、可执行面白名单、schema/fail-closed 校验与现有读 API 执行。
3. **可解释**：reviewer 必须能在 5 分钟内说明：哪些问题属于“同一契约重复组装”，哪些属于“共享层残留模块私有语义”，哪些只是测试/文档漂移，不得把三类问题混为一谈。

## 1. 背景与问题定义

`DEV-PLAN-465` 已冻结 `orgunit.details` 的一个局部事实：

1. 字段展示 owner 属于 `orgunit` 读契约，不属于 `CubeBox`；
2. 当前问题更接近“复用同一契约但重复组装”，而不是“再造第二套字段标准”；
3. `include_disabled` 仍存在一处参数 fail-closed 风险。

但沿着同一条查询链继续向外调查后，发现问题并不止于 `orgunit.details`：

1. `orgunit.search`、`orgunit.audit`、`orgunit.list` 仍不同程度存在“公开 API 与 `CubeBox` executor 各组装一次响应”的重复实现；
2. 共享 `query flow` 中还保留了 `orgunit` 专属澄清/错误语义；
3. 共享 narrator 中仍写死了明显的 `orgunit` 字段与文案约束；
4. knowledge pack 与执行注册表之间还缺少主动防漂移校验；
5. 参数类型与枚举的 fail-closed 仍主要散落在各模块 executor 内，缺少一致的调查与收口清单；
6. 个别 readiness 文档仍保留已过时的 `SummaryRenderer` 叙事。

因此，465 不应被视为单点修复，而应被纳入一份更大的“查询链 owner 漂移与反回流”调查 owner 文档。

## 2. 核心目标

1. [ ] 冻结 `CubeBox` 查询链中仍残留的同型问题分类与优先级。
2. [ ] 区分“代码行为风险”与“文档/测试漂移风险”，避免整改顺序失焦。
3. [ ] 明确哪些问题应由 `orgunit` owner 收口，哪些问题应由共享 `CubeBox` query runtime 收口。
4. [ ] 为后续 PR 分批落地提供 stopline 与验收标准，阻断相同问题继续回流。

## 3. 非目标

- 不在本计划内直接实施所有代码整改。
- 不在本计划内引入通用 DTO 平台、通用参数 schema 平台、FactSet 平台、AnswerComposer 平台或新的模板系统。
- 不在本计划内扩展第二个业务模块接入查询链。
- 不在本计划内重写 `460/461/462/464` 的上位 owner，只承接并细化它们在当前代码上的剩余问题。

## 4. 调查结论总览

本轮扩大调查把问题分为五类，其中前两类为 `P1`，后三类为 `P2/P3`：

| 类别 | 严重度 | 现象 | owner 倾向 |
| --- | --- | --- | --- |
| A. 业务响应契约重复组装 | `P1` | 同一业务响应在公开 API 与 `CubeBox` executor 各实现一次 | `orgunit` 读契约 owner |
| B. 共享 query flow 残留模块私有语义 | `P1` | 共享层仍持有 `orgunit` 专属澄清/错误语义 | 共享 `CubeBox` runtime owner |
| C. 共享 narrator 残留模块私有回答约束 | `P2` | 共享 narrator 里写死 `orgunit` 字段和文案 | 共享 `CubeBox` runtime owner |
| D. knowledge pack 与执行注册表缺少主动防漂移校验 | `P2` | 只能在运行时命中错误 `api_key` 后才 fail-closed | `modules/cubebox` owner |
| E. 参数类型 fail-closed 零散分布 | `P2` | 参数名已收敛，但参数类型/枚举校验分散在各 executor | 模块 executor owner |
| F. readiness / 文档叙事过时 | `P3` | 文档仍引用已不再存在的旧挂点 | 对应文档 owner |

## 5. 详细问题清单

### 5.1 类别 A：业务响应契约重复组装

#### A1. `orgunit.details` 之外，`search` / `audit` / `list` 也存在同型重复

调查发现以下路径仍存在“公开 API 与 `CubeBox` executor 各组装一次”的模式：

1. `orgunit.search`
   - `internal/server/orgunit_api.go`
   - `internal/server/cubebox_orgunit_executors.go`
2. `orgunit.audit`
   - `internal/server/orgunit_api.go`
   - `internal/server/cubebox_orgunit_executors.go`
3. `orgunit.list`
   - 查询核心已有共享函数 `listOrgUnitListPage(...)`
   - 但公开 API 仍有多条分支手工组装 `orgUnitListResponse`
   - `CubeBox` executor 也再包装一次响应

这类问题与 465 的性质一致：

- 不一定意味着字段 owner 已漂；
- 但意味着同一稳定契约的组装散落在多个入口；
- 后续字段变更、空切片语义或错误映射调整时，极易出现 drift。

#### A2. owner 冻结

该类问题的 owner 应仍在业务模块读契约侧，而不是 `CubeBox` 侧。

对 `orgunit` 来说，应优先由业务读契约 owner 抽取共享 response builder / helper，然后让公开 API 与 `CubeBox` 共同复用。

### 5.2 类别 B：共享 query flow 残留模块私有语义

#### B1. `orgunit.search` 歧义澄清仍由本地代码 owner

当前 `orgUnitSearchAmbiguousError` 在 `internal/server/cubebox_orgunit_executors.go` 中自带 `ClarifyingQuestion()`，并由共享 `query flow` 的 `queryExecutionClarifyingQuestion(...)` 消费。

这意味着：

1. 本地代码仍在生成模块专属面向用户的澄清文案；
2. 共享 query flow 知道 `orgunit` 的歧义语义，而不是只做通用边界处理；
3. 这与 `464` 中“模型负责缺参澄清、本地只保留最小 deterministic fallback”的方向存在紧张关系。

#### B2. 共享 query flow 仍硬编码 `orgunit_not_found`

`queryExecutionErrorToTerminal(...)` 当前仍对 `errOrgUnitNotFound` 做专门映射。对只有 `orgunit` 一个模块的现状来说这能工作，但它会形成共享层对具体模块错误语义的耦合。

owner 冻结：

- 共享层可以保留极小的通用错误类别；
- 但不应无限增长模块私有错误语义分支。

### 5.3 类别 C：共享 narrator 残留模块私有回答约束

当前 narrator 相关代码仍直接写入大量 `orgunit` 语义，例如：

- `org_code`
- `parent_org_code`
- `include_disabled`
- `ext_fields`
- “组织基本信息”
- “上级组织”
- “组织全路径”

这说明共享 narrator 仍在用 `orgunit` 场景去定义“什么叫不好的回答”。

短期内这不会立刻造成功能错误，但会带来两个风险：

1. 第二个模块接入查询链时，共享 narrator 会天然偏向 `orgunit` 语义；
2. 共享层的 contract validation 会被单模块字段集合污染。

### 5.4 类别 D：knowledge pack 与执行注册表缺少主动防漂移校验

当前 knowledge pack 校验只保证：

- 必填文件存在
- `queries.md` 有 intents block
- `apis.md` 有 api_key block
- `examples.md` 有至少一个可解析的 `ReadPlan` 示例

但当前仍缺少以下主动校验：

1. `apis.md` 中声明的 `api_key` 是否全部在执行注册表中注册；
2. 执行注册表里实际存在的 `api_key` 是否已被知识包覆盖；
3. `queries.md` / `apis.md` 的参数口径是否与 executor 的必填/可选参数集合一致。

因此当前只能在模型产出错误计划后，于运行时命中 `api_catalog_drift_or_executor_missing`；这属于“有 fail-closed，但缺少早发现”。

### 5.5 类别 E：参数类型 fail-closed 仍零散分布

`ReadPlan` 当前主要校验结构，执行注册表主要校验参数名与必填存在性，参数值类型与枚举校验仍下沉在各 executor 内。

465 已暴露其中一个具体例子：

- `include_disabled` 会把非法字符串静默落成 `false`

同型风险还可能继续出现在：

- `status`
- `page`
- `size`
- `limit`
- 未来新增的布尔、枚举与日期参数

本类问题当前不要求引入新平台，但需要形成系统性的调查与逐项收口清单，而不是只在单点 bug 出现时被动修。

### 5.6 类别 F：文档与 readiness 叙事仍有过时残留

例如 `docs/dev-records/DEV-PLAN-462-READINESS.md` 当前仍在叙述：

- `SummaryRenderer`
- capability-specific summary 挂载

而当前代码主链已经转向 narrator 模式。该问题不直接影响运行时，但会误导后续评审与新整改 owner 判断。

## 6. 优先级与实施顺序建议

### 6.1 Phase A / P1：先收口重复契约组装

优先顺序建议：

1. `orgunit.details`
2. `orgunit.search`
3. `orgunit.audit`
4. `orgunit.list`

理由：

- 这是最直接、最容易 drift 的问题；
- 与 `465` 已冻结的 owner 边界完全一致；
- 不需要引入新抽象，只需把 builder / helper 收回业务读契约 owner。

### 6.2 Phase B / P1：收口共享 query flow 的模块私有语义

优先处理：

1. `orgUnitSearchAmbiguousError` 的用户澄清 owner
2. `orgunit_not_found` 这类共享层模块专属错误映射

目标是阻断共享 runtime 继续增长 capability-specific 语义。

### 6.3 Phase C / P2：补主动防漂移校验

优先处理：

1. knowledge pack `api_key` 集合与执行注册表的一致性校验
2. knowledge pack 参数集合与 executor 参数集合的一致性校验

目标是把“运行时才发现 drift”前移为“加载或测试阶段发现 drift”。

### 6.4 Phase D / P2：逐项收紧参数类型 fail-closed

对现有参数逐项做：

1. 类型白名单
2. 枚举白名单
3. 非法值到 `invalid_request` 的稳定映射

但明确不走“新平台先行”的路线。

### 6.5 Phase E / P3：文档与 readiness 回填

同步清理过时叙事，避免 reviewer 继续依据旧结构做错误判断。

## 7. Stopline 冻结

在本计划完成前，代码评审应默认阻断以下模式继续新增：

1. 在 `CubeBox` executor 中新增新的业务响应 DTO 私有组装。
2. 在共享 `query flow` 中新增新的模块专属澄清文案或业务错误分支。
3. 在共享 narrator 中新增新的模块专属字段禁词或 capability-specific 反例模板。
4. 让 knowledge pack 声明与执行注册表继续各自演化而无一致性校验。
5. 以“先兼容后收口”为理由继续接受宽松字符串布尔/枚举解析，而没有明确 fail-closed owner。

## 8. 验收标准

1. [ ] 已形成覆盖上述 A-F 六类问题的正式调查 SSOT，而不是散落在对话或临时记录中。
2. [ ] 已明确区分：哪些问题属于业务读契约 owner，哪些问题属于共享 `CubeBox` runtime owner。
3. [ ] 已给出清晰的 `P1/P2/P3` 优先级与建议实施顺序。
4. [ ] 已冻结 stopline，防止相同问题在后续 query 模块接入时继续回流。
5. [ ] 未把本计划扩写成新的平台设计、DSL 设计或模板系统方案。

## 9. 门禁与执行记录

- 本轮交付只涉及文档：
  - `make check doc`

## 10. 交付物

- `docs/dev-plans/466-cubebox-query-owner-drift-and-anti-backflow-investigation-plan.md`

## 11. 关联文档

- `docs/dev-plans/460-cubebox-digital-assistant-positioning-and-execution-contract.md`
- `docs/dev-plans/461-cubebox-query-scenarios-minimal-contract.md`
- `docs/dev-plans/462-cubebox-codex-compaction-adoption-value-and-unified-convergence-plan.md`
- `docs/dev-plans/463-cubebox-orgunit-tree-discovery-gap-investigation-and-remediation-plan.md`
- `docs/dev-plans/464-cubebox-query-architecture-convergence-plan.md`
- `docs/dev-plans/465-cubebox-orgunit-executor-contract-boundary-and-field-owner-convergence-plan.md`
