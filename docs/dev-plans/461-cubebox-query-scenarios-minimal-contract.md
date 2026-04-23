# DEV-PLAN-461：CubeBox Markdown 知识包驱动的查询方案

**状态**: 规划中（2026-04-23 10:12 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：冻结 `CubeBox` 查询场景的最小实现方案：通过类似 skill 的 Markdown 知识包向大模型提供模块知识、查询意图、参数补全规则和现有读 API 映射，由模型输出支持线性多步只读编排的结构化查询计划；代码层只负责知识包加载、计划校验和现有读 API 执行，避免把业务知识硬编码在 Go/TS 中。
- **关联模块/目录**：`docs/dev-plans/460-cubebox-digital-assistant-positioning-and-execution-contract.md`、`docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`、`docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md`、`modules/cubebox`、`internal/server`、`apps/web`、`modules/orgunit`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-003`、`DEV-PLAN-017`、`DEV-PLAN-019`、`DEV-PLAN-022`、`DEV-PLAN-430`、`DEV-PLAN-434`、`DEV-PLAN-460`
- **用户入口/触点**：Web Shell 右侧 `CubeBox` 抽屉、后续 `CubeBox` 查询知识包加载链、现有业务模块只读 API

### 0.1 Simple > Easy 三问

1. **边界**：业务知识、意图映射、字段语义和 API 使用规则放在 Markdown 知识包中；代码层只持有通用加载、校验和执行能力。
2. **不变量**：`CubeBox` 查询必须复用现有读 API，不能成为数据库直查器，不能形成第二读事实源，不能绕过当前用户权限、当前租户和当前 session。
3. **可解释**：reviewer 必须能在 5 分钟内说明 `CubeBox` 如何加载知识包、模型如何生成支持线性多步只读编排的 `ReadPlan`、服务端如何校验计划，以及为什么这套机制没有把模块知识硬编码进运行时代码。

### 0.2 现状研究摘要

- **现状实现**：`CubeBox` 已具备对话 UI、会话持久化、prompt/context 管理、provider 网关和基础连续对话主链，但尚未冻结“模块知识如何注入模型”和“自然语言查询如何稳定映射到现有读 API”的通用方案。
- **现状约束**：`460` 已冻结 `CubeBox` 是当前用户的数字助手；文档可以帮助理解和编排，但不能成为授权来源；查询只能走现有权限与现有系统边界。
- **最容易出错的位置**：把模块知识硬编码进 Go/TS；把 Markdown 当成自由提示词而没有结构化约束；把知识包做成第二套 API 契约源；把查询做成数据库直查器或自由探索器。
- **本次不沿用的“容易做法”**：不在代码里写死模块意图、字段别名和 API 映射；不先做通用 planner DSL；不先做向量检索平台；不复活旧 `assistant_knowledge_*` 运行时。

## 1. 背景与上下文

`DEV-PLAN-460` 已冻结 `CubeBox` 的职责应是：

- 意图理解器
- 参数补全器
- 现有读 API 的编排器
- 结果解释器

如果直接把这些知识写死在代码里，会立刻带来三个问题：

1. 业务模块知识扩张时，`modules/cubebox` 或 `internal/server` 会持续膨胀。
2. API 变更、字段语义变更、租户差异化规则会导致代码频繁改动并产生漂移。
3. `CubeBox` 会逐步长成第二套业务知识运行时，违背 `DEV-PLAN-003` 的 “Simple > Easy” 原则。

因此本计划冻结一个更简单的方向：让 `CubeBox` 像读取 skill 一样读取模块级 Markdown 知识包，由知识包描述“模块是什么、支持哪些查询意图、如何补全参数、允许调用哪些现有读 API”，再由模型输出结构化查询计划。代码层只负责通用机制，不负责模块知识本身。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 冻结 `CubeBox` 查询场景的知识来源：模块知识通过 Markdown 知识包提供，而不是硬编码在运行时代码中。
- [ ] 冻结知识包最小目录与格式，确保模型可读、代码可校验。
- [ ] 冻结支持线性多步只读编排的 `ReadPlan` 结构化输出契约，避免模型直接产出不可执行的自由文本查询。
- [ ] 冻结首个样板模块 `orgunit` 的知识包口径，验证“知识包驱动 + 现有读 API 执行”的最小闭环。

### 2.2 非目标

- 不在本计划内定义写入提案、写入确认或写入执行协议。
- 不在本计划内引入通用 workflow engine、planner DSL、任务图执行框架或大而全 tool registry。
- 不在本计划内建设向量检索、全文搜索平台或通用知识库中台。
- 不在本计划内让 `CubeBox` 直接读取数据库、投影表、内部 SQL 函数或未公开的内部接口。
- 不在本计划内冻结租户级知识覆盖的存储面、发布面和权限边界；该能力后移到独立 owner 计划。

### 2.3 用户可见性交付

- **用户可见入口**：现有 `CubeBox` 抽屉。
- **最小可操作闭环**：用户以自然语言提出业务查询；`CubeBox` 加载对应模块知识包并生成支持线性多步只读编排的 `ReadPlan`；服务端按计划执行现有读 API；`CubeBox` 返回解释后的结果。
- **首批样板**：`orgunit` 查询场景，例如查询单个组织详情、条件列表、搜索、审计摘要。

## 3. 核心定义

### 3.1 知识包定义

本计划中的“知识包”指一组受控的 Markdown 文件，用来向模型提供：

- 模块定位
- 业务对象与术语
- 支持的查询意图
- 参数补全规则
- 允许调用的现有读 API
- 少量高质量示例

知识包不是：

- 第二套数据库 schema
- 第二套业务 API 实现
- 运行时可执行脚本
- 权限来源

### 3.2 `ReadPlan` 定义

`ReadPlan` 是模型读取知识包后产出的结构化查询计划。它是 `CubeBox` 查询链路中唯一允许进入执行层的计划对象。

`ReadPlan` 至少包含：

- `intent`
- `confidence`
- `missing_params`
- `steps`
- `explain_focus`

其中 `steps` 用于表达一个用户问题下的线性多步只读编排。首期只支持按顺序执行的前序依赖，不支持图状执行、循环、并发分叉或动态工具发现。

若模型不能稳定形成可执行计划，则必须返回“缺少参数/需要澄清”，而不是自由猜测后直接执行。

### 3.3 代码层职责

代码层只持有以下通用职责：

1. 根据当前页面、当前对象和当前模块发现应加载哪些知识包。
2. 读取并拼装知识包内容供模型使用。
3. 校验模型产出的 `ReadPlan` 是否符合 schema 和允许范围。
4. 通过代码中的 `api_key -> executor` 注册表，把步骤安全映射到现有读 API 执行器。
5. 执行后将原始结果返回给结果解释阶段。

代码层不持有模块知识本身。

## 4. 知识包目录与加载规则

### 4.1 模块级目录规范

建议每个模块的 `CubeBox` 查询知识包放在模块自身目录下，避免重新长出旧对话时代的全局知识运行时：

```text
modules/<module>/presentation/cubebox/
├── CUBEBOX-SKILL.md
├── queries.md
├── apis.md
└── examples.md
```

首批样板模块为：

```text
modules/orgunit/presentation/cubebox/
```

### 4.2 租户级覆盖能力后移

租户级知识覆盖能力不在 `461` 首期冻结具体目录、存储面或发布面。

本计划只冻结一个原则：

- 若未来确有租户级覆盖需求，必须由独立 owner 计划冻结存储面、发布面和权限边界。
- 该覆盖能力只能补充或收窄模块知识，不能扩大权限，不能声明新的底层执行能力。
- `461` 首期只做模块级通用知识包。

### 4.3 加载顺序

知识包加载顺序冻结为：

1. 模块级通用知识包
2. 当前页面 / 当前对象 / 当前用户权限摘要 / 当前租户上下文

## 5. 知识包文件职责

### 5.1 `CUBEBOX-SKILL.md`

持有：

- 模块定位
- 主要业务对象
- 当前页面上下文如何帮助补参
- 当前模块允许的查询意图概览
- 对 `queries.md` / `apis.md` / `examples.md` 的引用

### 5.2 `queries.md`

持有：

- 支持的查询意图
- 每种意图需要哪些参数
- 缺参时应追问什么
- 常见自然语言同义词或别名如何映射到意图

### 5.3 `apis.md`

持有：

- 面向模型的现有只读 API 目录说明
- 每个 API 的用途、必填参数、可选参数、权限前提、关注字段
- 明确声明“禁止直查数据库、禁止调用未列出的接口”

`apis.md` 不是运行时执行事实源。它只用于 prompt-facing 的说明与约束，帮助模型生成合法 `ReadPlan`。

运行时唯一执行事实源必须是代码中的 `api_key -> executor` 注册表；知识包不能直接决定可执行面。

### 5.4 `examples.md`

持有：

- 少量高质量问法
- 对应的参数补全过程
- 对应的 `ReadPlan` 示例

## 6. 文件格式原则

### 6.1 Markdown + 结构化块

知识包文件应采用“Markdown 叙述 + 少量结构化块”的混合方式。

原因：

- 纯 Markdown 可读，但执行不稳定
- 纯 JSON/YAML 可执行，但维护成本高、对 reviewer 不友好

因此建议在 Markdown 中嵌入少量 YAML 或 JSON fenced block，作为模型和代码共同消费的稳定锚点。

### 6.2 结构化块最小约束

知识包中的结构化块只承担三件事：

1. 声明支持的查询意图
2. 声明允许调用的 API 目录
3. 提供少量 `ReadPlan` 示例

不应把知识包膨胀成重型模板、全量对象映射表或第二套 API 文档站点。

## 7. 查询执行模型

### 7.1 主流程

查询主流程冻结为：

1. 用户提出自然语言查询。
2. 服务端根据当前上下文发现应加载的模块知识包。
3. 服务端把知识包 + 当前上下文一并提供给模型。
4. 模型输出支持线性多步只读编排的结构化 `ReadPlan`。
5. 服务端校验 `ReadPlan`。
6. 服务端按 `steps[]` 顺序，通过代码中的执行注册表把每一步的 `api_key` 映射到现有读 API 执行器。
7. 获取各步原始结果后，再交给 `CubeBox` 做结果解释和整理。

### 7.2 `ReadPlan` 最小 schema

首期 `ReadPlan` 最小结构冻结为：

```json
{
  "intent": "string",
  "confidence": 0.0,
  "missing_params": [],
  "steps": [
    {
      "id": "step-1",
      "api_key": "string",
      "params": {},
      "result_focus": [],
      "depends_on": []
    }
  ],
  "explain_focus": []
}
```

约束如下：

- `steps` 至少 1 步。
- 每一步必须包含 `id`、`api_key`、`params`、`result_focus`、`depends_on`。
- `depends_on` 只允许引用前序步骤，形成线性顺序，不允许图状执行、循环依赖或并发分叉。
- 若某一步依赖前一步结果做参数补全，必须通过前序步骤结果显式派生，不能隐式猜测。

若缺少必要参数，允许返回：

```json
{
  "intent": "string",
  "confidence": 0.0,
  "missing_params": ["..."],
  "clarifying_question": "..."
}
```

### 7.3 执行边界

执行边界冻结如下：

1. `api_key` 只能映射到现有业务模块只读 API。
2. 模型不能直接指定数据库、SQL、内部表、内部函数或未登记的执行器。
3. 模型不能在 `params` 中注入当前用户无权访问的对象。
4. `apis.md` 只能帮助模型理解允许的查询面，不能替代代码中的执行注册表。
5. 当 `ReadPlan` 校验失败时，必须 fail-closed，不得退化成自由查询。

## 8. 首批样板：`orgunit`

### 8.1 首批查询意图

`orgunit` 首批只冻结以下查询意图：

- `orgunit.details`
- `orgunit.list`
- `orgunit.search`
- `orgunit.audit`

### 8.2 首批样板目标

通过 `orgunit` 样板验证：

- 模块知识不写死在代码中
- 模型能借助知识包稳定输出支持线性多步只读编排的 `ReadPlan`
- 计划校验后能安全调用现有只读 API
- `CubeBox` 最终能返回业务可读的查询解释

## 9. 权限与失败语义

### 9.1 权限不变量

- 知识包不是授权来源
- 查询必须完全继承当前用户权限
- API 执行阶段仍必须通过现有 Authz / RLS / session 约束
- 权限不足时必须 fail-closed，而不是退化成更宽的结果集

### 9.2 失败语义

查询链路中的失败分为五类：

1. **知识包缺失**
   - 当前模块尚未接入 `CubeBox` 查询能力

2. **知识包非法**
   - `knowledge_pack_invalid`
   - 知识包结构化块缺失、格式非法或解析失败，导致无法形成受控 prompt 输入

3. **计划不可执行**
   - 模型未能产出合法 `ReadPlan`，或缺少必要参数

4. **权限不足**
   - 当前用户无权查看该数据，不得尝试旁路读取

5. **执行目录漂移或底层 API 失败**
   - `api_catalog_drift_or_executor_missing`
   - `ReadPlan` 中的 `api_key` 未在代码执行注册表中注册，或知识包声明与执行注册表不一致
   - 继续走现有错误码与错误映射，不允许 `CubeBox` 发明第二套错误语义

## 10. 验收标准

- [ ] 模块知识、意图映射、字段语义与 API 使用规则不再硬编码在 `modules/cubebox` 或 `internal/server` 中。
- [ ] 首期知识包目录和加载顺序明确，且 `461` 不提前冻结租户级覆盖的目录、存储面和发布面。
- [ ] 模型只能通过知识包产出结构化 `ReadPlan`，而不是直接生成自由文本执行动作。
- [ ] `ReadPlan` 首期支持一个问题下的线性多步只读编排，但不引入图状执行、并发分叉或通用 workflow engine。
- [ ] `ReadPlan` 执行只能复用现有只读 API，不能直查数据库或形成第二读事实源。
- [ ] `apis.md` 只是 prompt-facing 说明，运行时唯一执行事实源仍是代码中的 `api_key -> executor` 注册表。
- [ ] `orgunit` 首批样板能跑通最小查询闭环。
- [ ] 查询失败语义清晰，覆盖 `knowledge_pack_invalid` 与 `api_catalog_drift_or_executor_missing`，且不以模型猜测替代真实系统结果。

## 11. 反模式与禁止项

- 不得把模块知识写死在 `CubeBox` 运行时代码中
- 不得复活旧 `assistant_knowledge_*` 路径或同类历史运行时
- 不得把知识包做成第二套 API 或权限系统
- 不得让模型直接拼 SQL、直读内部表或调用未登记执行器
- 不得让知识包直接决定可执行面；执行面只能由代码中的 `api_key -> executor` 注册表冻结
- 不得一开始就引入通用 planner DSL、workflow engine 或大而全 tool registry
- 不得把“支持多步查询”膨胀成 DAG 编排、并发 fan-out/fan-in、动态工具发现或通用任务图框架
- 不得在 `461` 首期冻结租户级覆盖目录路径、存储面或发布面
- 不得把知识包膨胀成重型固定模板，导致维护和演化成本过高

## 12. 关联文档

- `docs/dev-plans/460-cubebox-digital-assistant-positioning-and-execution-contract.md`
- `docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`
- `docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md`
