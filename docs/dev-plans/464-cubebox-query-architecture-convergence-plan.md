# DEV-PLAN-464：CubeBox 查询链轻量化收敛与模型 owner 回正方案

**状态**: 实施中（2026-04-24 23:40 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：承接 `DEV-PLAN-463` 已冻结的问题诊断，作为 CubeBox 查询链的重构/整改方案，把当前“知识包外观 + server 二次业务理解 + 模板化回答”的偏航，收敛回“模型负责语义理解与解释、代码只负责护栏与执行”的最小查询链。
- **关联模块/目录**：`docs/dev-plans/460-cubebox-digital-assistant-positioning-and-execution-contract.md`、`docs/dev-plans/461-cubebox-query-scenarios-minimal-contract.md`、`docs/dev-plans/462-cubebox-codex-compaction-adoption-value-and-unified-convergence-plan.md`、`docs/dev-plans/463-cubebox-orgunit-tree-discovery-gap-investigation-and-remediation-plan.md`、`modules/cubebox`、`modules/orgunit/presentation/cubebox`、`internal/server`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-003`、`DEV-PLAN-012`、`DEV-PLAN-017`、`DEV-PLAN-019`、`DEV-PLAN-022`、`DEV-PLAN-300`、`DEV-PLAN-301`、`DEV-PLAN-304`、`DEV-PLAN-430`、`DEV-PLAN-437A`、`DEV-PLAN-460`、`DEV-PLAN-461`、`DEV-PLAN-462`、`DEV-PLAN-463`
- **用户入口/触点**：主应用壳层右侧 `CubeBox` 抽屉、`/internal/cubebox/turns:stream`、`internal/server/cubebox_query_flow.go`、`modules/cubebox/*`、`modules/orgunit/presentation/cubebox/*`
- **上游调查 SSOT**：问题事实、偏航诊断、真实复验证据与直接 stopline 以 `DEV-PLAN-463` 及 `docs/dev-records/DEV-PLAN-463-READINESS.md` 为准；本文件只持有轻量化整改方向、owner 收口、迁移阶段与验收标准。

### 0.1 Simple > Easy 三问

1. **边界**：本计划解决的是查询链 owner 偏航，不是把 CubeBox 扩成 agent 平台、workflow 平台、查询 DSL 平台或第二套模块 runtime。
2. **不变量**：查询仍必须继承 `DEV-PLAN-460` 冻结的当前用户、当前租户、当前 session 与现有只读边界；不得新增数据库直查、第二读事实源、第二授权面。
3. **可解释**：reviewer 必须能在 5 分钟内说明三件事：模型负责什么，本地代码负责什么，为什么 `internal/server` 不该继续替模型做 `orgunit` 业务判断。

### 0.2 本计划冻结的总判断

- `DEV-PLAN-463` 已说明：当前问题不只是 `has_children` 漏查，而是 CubeBox 查询链逐步偏成“模型产出草案，server 再做第二轮业务理解、默认值补丁、澄清改写和模板化解释”。
- 本计划冻结的收敛方向不是“再发明一套更完整的本地语义 runtime”，而是反过来收缩本地职责，把默认值、缺参澄清、查询解释等高语义工作尽量还给大模型。
- 因此 `464` 的目标是让查询链回到：知识包提供上下文与约束，模型负责理解与回答，本地代码只做 fail-closed 护栏、执行和最小 deterministic fallback。
- `464` 的定位是**重构/整改 owner**，不是 `460/461/462` 之上的上位方案；凡属数字助手定位、查询主契约、统一收敛方法论的上位 owner，仍分别以 `460/461/462` 为准。

## 1. 背景与问题定义

当前 CubeBox 对外看起来像“知识包驱动的查询系统”，但真正稳定其行为的仍是 `internal/server` 中的业务关键词补丁、模块私有前序引用协议和能力专属模板摘要。

这会带来四类结构性后果：

1. 知识包变成 prompt 素材，而不是模型实际遵循的主语义来源。
2. 大模型被压缩成“严格 JSON 生成器”，没有真正承担默认值选择、缺参澄清和结果解释这些它更擅长的工作。
3. `internal/server` 持续长出 `orgunit` 私有业务判断，逼近第二套模块语义层。
4. 回答层继续模板化，会把 `461/462` 想冻结的统一收敛问题，重新打回 capability-specific patch。

`464` 的目标不是在现状上再补一层 contract / projector / composer，而是收回过量本地职责，让运行时重新变薄。

## 2. 目标与非目标

### 2.1 核心目标

- [x] 把默认值选择、缺参澄清、结果解释这些高语义工作尽量回交给模型，而不是继续沉到 `internal/server`。
- [x] 把本地代码收敛为“权限/租户边界 + 可执行面白名单 + schema/fail-closed 校验 + 现有读 API 执行”。
- [x] 删除 `internal/server` 中当前对 `orgunit` 的二次业务理解与 prompt 关键词补丁。
- [x] 收缩执行层，避免继续长出模块私有编排协议与隐藏控制字段。
- [x] 承接 `461/462` 已冻结的回答收敛原则，但不在本计划中再发明一套更重的解释平台。

### 2.2 非目标

- 不在本计划内建设通用知识 schema、planner DSL、FactSet 平台、AnswerComposer 平台、模板平台或展示 DTO 平台。
- 不在本计划内引入 DAG planner、并发编排、动态工具发现、通用 workflow engine 或完整 tool platform。
- 不在本计划内建设向量检索、租户知识库中台或文档发布系统。
- 不在本计划内让查询执行绕过现有读链路直接查库。
- 不在本计划内承诺“组织树一次性展开全部层级”或“自由多跳探索式查询”。

## 3. 收敛原则

### 3.1 模型负责语义，本地负责护栏

- 模型负责：
  - 意图理解
  - 默认值选择
  - 缺参澄清
  - 结果叙述
- 本地代码负责：
  - 权限/租户边界
  - 可执行面白名单
  - schema / fail-closed 校验
  - 现有读 API 执行

### 3.2 不把知识包翻译成第二套本地语义系统

- 知识包仍是模型的主要理解材料。
- 本地只解析“本地必须知道”的最小锚点，不把 `queries.md` / `apis.md` 全量翻译成新的运行时 owner 层。
- 默认值策略、追问语气、解释关注点等内容，优先留在知识包中供模型消费，而不是提升成本地 struct 契约。

### 3.3 不把回答收敛做成新平台

- 本计划承接 `461/462` 对最小返回边界、统一预算与统一降级的要求。
- 但首期不建设 `FactSet`、`AnswerComposer`、模板平台或展示 DSL。
- 首期不再把查询结果收口命名成新的本地平台或中间层。

### 3.4 执行层必须继续变薄

- 执行注册层仍只做 `api_key -> executor` 白名单映射、参数校验与顺序执行。
- 模块执行器只做参数转换和调用既有读链路。
- 不允许继续在模块执行器里发明私有编排协议、隐藏唯一性开关或摘要模板。

## 4. 目标主链

冻结后的查询主链应收敛为：

1. 用户输入自然语言查询。
2. 运行时加载模块知识包并交给模型。
3. 模型基于知识包、当前上下文和当前输入，直接完成：
   - 意图理解
   - 默认值选择
   - 缺参澄清或 `ReadPlan` 生成
4. 本地共享校验层只执行：
   - schema 校验
   - intent / `api_key` 白名单校验
   - 权限/租户边界校验
   - fail-closed 错误返回
5. 共享执行层按 `ReadPlan` 顺序执行现有读 API。
6. 模型基于执行结果完成结果叙述。

## 5. Owner 收口

### 5.1 模型 owner

以下内容默认由模型 owner：

- 用户意图判定
- `as_of=today` 一类默认值选择
- “组织树默认先查一级组织”这类查询语义
- 缺参时是否追问、怎么追问
- 解释时优先强调哪些结果点

### 5.2 本地 owner

以下内容必须继续由本地 owner：

- 权限/租户边界
- 哪些 `api_key` 可执行
- schema / fail-closed 校验
- 实际执行

### 5.3 知识包 owner

知识包继续持有：

- 模块术语
- 查询意图说明
- 默认语义说明
- 缺参澄清口径
- 允许调用的现有读 API 说明
- 少量高质量样例

但知识包不直接成为本地执行事实源；执行事实源仍以代码中的注册表和边界校验为准，符合 `460/461` 的冻结口径。

## 6. 本地最小锚点

为了让运行时继续 fail-closed，本地只允许保留代码内已有的执行安全边界；不得再把知识包提炼成新的执行锚点。

P1 允许本地继续依赖的事实源只有：

1. 代码中的 `api_key -> executor` 注册表
2. 代码中的 `ReadPlan` schema 与校验逻辑

P1 明确不进入本地运行时 owner 的内容：

- 从知识包提炼 `allowed_apis`
- 从知识包提炼 `required_params`
- `default_policies`
- `clarification_policies`
- `explain_focus`
- 展示模板元数据
- 结果字段导出 schema
- 通用字段路径语言
- 本地解释输入/结果视图平台

这些内容若存在，默认继续留在知识包文本中，由模型理解、选择和叙述；本地代码不得把它们再次翻译成执行事实源。

## 7. Planner 收敛方案

### 7.1 planner 的正式职责

planner 应负责：

1. 读取知识包。
2. 完成意图判定、默认值选择、缺参澄清与线性步骤生成。
3. 输出受 schema 约束的 `ReadPlan` 或澄清问题。

server 不再做第二轮业务理解，只允许做边界校验与 fail-closed。

### 7.2 明确要删除的 server 语义补丁

以下模式必须从 `internal/server` 中移除：

- `normalizeCubeboxReadPlan(...)` 内的模块语义改写
- `isOrgUnitRootListPrompt(...)` / `isOrgUnitChildrenPrompt(...)` 一类 prompt 关键词补丁
- server 侧对 `orgunit.list` 默认值的硬填
- server 侧对 `orgunit` 澄清文案的手工 humanize

### 7.3 迁移要求

- 迁移时应先更新知识包样例与测试夹具，让模型输出新 owner 形状，再删除旧 server 补丁。
- 不允许出现“知识包仍教模型产出旧语义，但 server 已开始拒绝旧形状”的中间漂移。

## 8. 执行层收敛方案

### 8.1 执行注册层边界

共享执行层只允许承担：

- `api_key -> executor` 白名单注册
- 必填参数校验
- 线性依赖校验
- 顺序执行
- 结果转交

共享执行层不得继续承担：

- 模块业务判断
- 回答模板
- prompt 关键词判断
- 模块私有隐藏协议扩张

### 8.2 模块执行器边界

模块执行器只做三件事：

1. 把共享 runtime 传入的字面量参数转为模块现有读链路输入。
2. 调用现有读链路。
3. 返回原始业务 payload 或稳定领域结果。

模块执行器不再承担：

- `SummaryRenderer`
- 查询结果中文解释
- `target_unique` 一类隐藏流程控制协议扩张

### 8.3 对跨步引用的收敛原则

- 当前 `org_code_from` 视为过渡债务，不是长期 contract。
- 首选方案不是立刻建设通用 `ParamRef`，而是先证明是否能通过更清晰的样例、显式澄清或更薄的线性约束删除该协议。
- 只有确认跨步引用是多个查询场景的最小共同需求时，才允许单开后续 owner 计划；`464` 首期不冻结 DSL。

### 8.4 模块参数与现有读契约 owner

- 模型与知识包应直接产出 canonical 参数；执行层不再负责自然语言别名折叠。
- 以 `orgunit.list.status` 为例，知识包与执行器只接受 `active`、`disabled`、`all`；`inactive` 不再作为 CubeBox executor 的兼容语义。
- `path_org_codes`、`has_more` 这类字段若仍存在于现有 `orgunit` 读响应中，则其 owner 属于 `orgunit` 读契约，而不是 `464` 执行层薄化。
- 若未来要删除上述字段，必须同步修改 `orgunit` 读 API、知识包说明、样例与测试，不能只在 CubeBox executor 单点切除。

## 9. 解释阶段收敛方案

### 9.1 解释阶段的最小形态

本计划不建设新的解释平台，首期只冻结：

1. 查询结果叙述由模型负责。
2. 本地不得继续扩张 capability-specific 模板体系。
3. 本地不得把整份 raw payload JSON 直接写成用户回答。

### 9.3 与 `461/462` 的关系

- `461` 继续持有查询结果进入回答前的最小返回边界。
- `462` 继续持有查询返回边界与会话压缩的上位方法论和 reviewer 口径。
- `464` 只持有“如何把当前实现从 server 模板/补丁迁回模型 owner”的整改路径，不重复重定义 `461/462` 的上位语义。
- 任何需要重写 `460/461/462` owner 边界的内容，都不应通过 `464` 间接改写；应回到对应上位计划直接修订。

## 10. 测试迁移与反回流方案

### 10.1 测试分层原则

测试应从“保护现状补丁”迁移为“保护护栏与 owner 边界”：

1. `modules/cubebox`
   - 验证最小锚点提炼
   - 验证 `ReadPlan` schema 与白名单校验
   - 验证结果叙述不再由本地模板承担
2. 模块侧
   - 验证知识包样例与现有读链路一致
   - 验证执行器只做参数映射
3. `internal/server`
   - 验证会话、provider、stream、错误映射
   - 不再验证 `orgunit` 关键词补丁

### 10.2 必删型测试预期

以下断言不应继续作为长期 contract：

- “查询组织树会被 server 自动改写成根列表默认执行”
- “server 会自动 humanize 某类 orgunit 澄清”
- “orgunit.list 必须走某个模块专属模板摘要器”

### 10.3 新的 stopline 测试

应新增的测试/门禁方向：

- server 不再持有模块专属 prompt 关键词补丁
- 新增本地锚点字段必须能证明是 fail-closed 所必需
- 查询结果叙述不得重新回到 capability-specific prose 模板
- 知识包样例变更后，planner 与执行边界测试必须同步更新

## 11. 分阶段实施方案

### 11.1 Phase A / P0：症状止血与 stopline 冻结

1. [x] 完成 `DEV-PLAN-463` 的直接缺陷修复，消除根列表 `has_children` 缺失导致的错误回答。
2. [x] 在代码评审口径中冻结 stopline：
   - 不再新增 `isOrgUnit*Prompt(...)`
   - 不再新增新的 `SummaryRenderer`
   - 不再新增新的模块私有隐藏协议
3. [x] 明确 `463 = 调查/诊断`、`464 = 轻量化整改 owner`

### 11.2 Phase B / P1：知识包与 planner owner 回正

1. [x] 将知识包样例、说明与 planner prompt 收敛为“模型负责默认值、澄清和解释”的新 owner 口径。
2. [x] 本地只保留最小锚点提炼，不再把默认值/澄清/解释关注点提升成本地 contract。
3. [x] 更新相关测试夹具，先切换知识包与 planner 输出，再移除旧 server 补丁。

### 11.3 Phase C / P1：server 去二次业务理解

1. [x] 删除 `normalizeCubeboxReadPlan(...)` 中的模块语义改写。
2. [x] 删除 `isOrgUnitRootListPrompt(...)` / `isOrgUnitChildrenPrompt(...)` 及等价逻辑。
3. [x] 删除 server 侧默认值硬填与澄清改写。

### 11.4 Phase D / P1：执行层继续变薄

1. [x] 收缩 `cubebox_orgunit_executors.go` 为薄适配层。
2. [x] 停止扩张 `org_code_from` / `target_unique` 一类私有协议。
3. [x] 仅在证明确有跨场景最小共性需求时，再单开后续 plan 处理跨步引用。

### 11.5 Phase E / P1-P2：解释 owner 回归模型

1. [x] 让模型承担查询结果叙述。
2. [x] 删除本地 capability-specific 模板路径。
3. [x] 不建设 `FactSet` / `AnswerComposer` / 模板平台作为前置条件。

## 12. 验收标准

1. [x] `internal/server/cubebox_query_flow.go` 不再持有 `orgunit` 关键词补丁、默认值硬填或澄清改写。
2. [x] 本地运行时只依赖代码中的注册表与 schema 校验，而不是从知识包提炼新的执行锚点或把默认值/澄清/解释翻译成第二套本地语义系统。
3. [x] `cubebox_orgunit_executors.go` 不再新增模块私有隐藏协议与模板摘要器。
4. [x] 查询解释重新以模型为主 owner，本地不再扩张结果叙述职责。
5. [x] `461/462` 的 fail-closed 返回边界与上位方法论仍保持单一 owner，`464` 不与其重复持有上位语义。
6. [x] 测试资产主要保护护栏和边界，而不是保护现状 patch。
7. [x] P1 未引入通用知识 schema、planner DSL、FactSet 平台、AnswerComposer 平台、模板平台或其他与当前 stopline 无直接关系的抽象。

## 13. 需要执行的门禁与核验

- 命中文档：`make check doc`
- 命中 Go 代码：按 `AGENTS.md` 执行 `go fmt ./... && go vet ./... && make check lint && make test`
- 若命中查询主链、planner、执行层或解释阶段，至少补跑对应定向测试，并保留一次真实页面或真实会话复验记录到 `docs/dev-records/DEV-PLAN-463-READINESS.md` 或后续相应 readiness 文档

## 15. 当前实现备注

- 2026-04-24 本轮已完成：query narrator 主切、模板化结果 fail-closed、执行注册表未知参数拒绝、`orgunit.details` 去 `org_code_from`、`orgunit.search` 去 `target_unique`、根列表补齐 `has_children`、知识包样例回正、`orgunit.list.status` 收敛到 canonical 值、executor payload 改为复用现有 `orgunit` 响应结构。
- `path_org_codes` 与 `has_more` 当前仍保留，因为它们已进入现有 `orgunit` 读契约；若后续删除，应由 `orgunit` owner 统一收口。
- 当前剩余收口工作只包括：全量门禁、页面复验与 readiness 回填；不再继续扩大运行时抽象。

## 14. 交付物

- `docs/dev-plans/464-cubebox-query-architecture-convergence-plan.md`
- `modules/cubebox` 中与最小锚点、`ReadPlan` 校验及执行边界相关的收敛实现
- `modules/orgunit/presentation/cubebox/*` 中与知识包 owner 回正相关的更新
- `internal/server/cubebox_query_flow.go`、`internal/server/cubebox_orgunit_executors.go` 等处的去二次业务理解收敛实现
