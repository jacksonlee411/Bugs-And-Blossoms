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

### 3.4 执行注册层的正式边界

本计划中的执行注册层，只是 `api_key -> executor` 的受控映射层，不是第二套业务实现。

执行注册层只允许承担：

- 白名单注册：声明哪些 `api_key` 允许进入执行面
- 参数收口：把模型输出的 `params` 校验为受控输入
- 顺序调度：按 `steps[]` 顺序调用现有只读 API
- 结果转交：把原始结果交给结果解释阶段

执行注册层不得承担：

- 重新实现模块业务查询逻辑
- 直接查询数据库、投影表、内部 SQL 函数或 store
- 重新发明一套 AI 专用读 DTO、权限模型或错误语义
- 脱离现有业务模块读链路形成第二读事实源

判定标准冻结为：

- 若移除执行注册层后，现有模块读能力仍完整存在且对外行为不变，则执行注册层只是受控分发层。
- 若移除执行注册层后，查询能力本身不再成立，则说明该层已经承载了第二套实现，本计划不允许这种设计。

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
5. 执行注册层只能做受控映射、参数收口和顺序调度，不得重新实现模块查询逻辑。
6. 当 `ReadPlan` 校验失败时，必须 fail-closed，不得退化成自由查询。

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
- [ ] 执行注册层删去后，现有模块只读能力本身仍成立；该层只承担受控映射、参数收口和顺序调度，而不是第二套实现。
- [ ] `orgunit` 首批样板能跑通最小查询闭环。
- [ ] 查询失败语义清晰，覆盖 `knowledge_pack_invalid` 与 `api_catalog_drift_or_executor_missing`，且不以模型猜测替代真实系统结果。

## 11. 反模式与禁止项

- 不得把模块知识写死在 `CubeBox` 运行时代码中
- 不得复活旧 `assistant_knowledge_*` 路径或同类历史运行时
- 不得把知识包做成第二套 API 或权限系统
- 不得让模型直接拼 SQL、直读内部表或调用未登记执行器
- 不得让知识包直接决定可执行面；执行面只能由代码中的 `api_key -> executor` 注册表冻结
- 不得让执行注册层重新承载模块查询逻辑、数据库访问、AI 专用 DTO、AI 专用权限判断或第二套错误体系
- 不得一开始就引入通用 planner DSL、workflow engine 或大而全 tool registry
- 不得把“支持多步查询”膨胀成 DAG 编排、并发 fan-out/fan-in、动态工具发现或通用任务图框架
- 不得在 `461` 首期冻结租户级覆盖目录路径、存储面或发布面
- 不得把知识包膨胀成重型固定模板，导致维护和演化成本过高

## 12. 实施步骤与进展跟踪

以下步骤用于后续实施排期、PR 拆分和 readiness 跟踪。首期按“先骨架、后样板、再联调”的顺序推进；每一步都必须保持可验证、可回退、可审查。

### 12.1 Step 1：冻结 `orgunit` 模块级知识包样板

- [x] 在 `modules/orgunit/presentation/cubebox/` 下创建 `CUBEBOX-SKILL.md`、`queries.md`、`apis.md`、`examples.md`
- [x] 只覆盖 `orgunit.details`、`orgunit.list`、`orgunit.search`、`orgunit.audit`
- [x] `apis.md` 中的 `api_key` 命名与后续代码注册表保持一一对应
- [x] 不引入租户级覆盖目录、租户级发布机制或额外知识运行时

交付结果：

- 仓库内存在首批模块级知识包实物
- reviewer 可以直接阅读 Markdown 知识包理解首期查询面

### 12.2 Step 2：实现 `ReadPlan` 最小契约与校验器

- [x] 在 `modules/cubebox` 冻结 `ReadPlan` 类型与 `steps[]` 最小 schema
- [x] 实现结构化解析与 fail-closed 校验
- [x] 对非法知识包或非法计划返回受控失败，不退化为自由查询
- [x] 明确支持线性多步只读编排，不支持 DAG、并发分叉或动态工具发现

交付结果：

- 运行时能够稳定接收和校验结构化 `ReadPlan`
- 非法输入会命中明确失败语义，而不是进入执行面

### 12.3 Step 3：实现 `api_key -> executor` 执行注册层

- [ ] 在 `modules/cubebox` 增加受控执行注册表
- [ ] 每个 `api_key` 只映射到现有模块只读能力
- [ ] 注册层只承担白名单注册、参数收口和顺序调度
- [ ] 不允许注册层直接查库、重写业务查询逻辑或形成第二读事实源

交付结果：

- 运行时存在唯一执行事实源
- 未注册 `api_key` 会 fail-closed，并命中 `api_catalog_drift_or_executor_missing`

### 12.4 Step 4：接入 `orgunit` 首批只读执行器

- [ ] 为 `orgunit.details`、`orgunit.list`、`orgunit.search`、`orgunit.audit` 接入执行器
- [ ] 执行器只复用现有 `orgunit` 读链路，不复制实现
- [ ] 参数校验与上下文注入遵循当前用户、当前租户、当前 session 边界
- [ ] 原始结果保持受控结构，交由结果解释阶段消费

交付结果：

- `orgunit` 成为 `461` 首个可跑通的模块样板
- 删除执行注册层后，现有 `orgunit` 读能力本身不受影响

### 12.5 Step 5：接入 `CubeBox` 查询主链

- [ ] 在现有 `CubeBox` turn 主链中插入“知识包加载 -> `ReadPlan` 生成 -> 校验 -> 执行 -> 结果解释”路径
- [ ] 保持当前对话 UI、会话持久化、权限与租户边界不变
- [ ] 不把查询编排实现为第二套对话 runtime
- [ ] 查询链路失败时走受控错误码与现有错误映射

交付结果：

- 用户可在现有 `CubeBox` 抽屉中触发知识包驱动查询
- 查询执行链不破坏当前 `CubeBox` 会话主链

### 12.6 Step 6：补齐错误语义、测试与 Readiness 证据

- [ ] 接入 `knowledge_pack_invalid` 与 `api_catalog_drift_or_executor_missing`
- [ ] 为知识包加载、`ReadPlan` 校验、执行注册表和 `orgunit` 样板补最小稳定测试
- [ ] 记录首期实现范围、已接入 `api_key`、未覆盖能力与已知限制
- [ ] 在对应 `docs/dev-records/` 中沉淀 readiness 证据

交付结果：

- 首期闭环具备可回归验证能力
- 后续扩模块时有明确基线，不需要回头重判边界

### 12.7 进展记录规则

- 每完成一个步骤，更新本节对应 checklist 状态。
- 每个步骤至少对应一个可审查 PR；不要把多个步骤揉成一次不可分审的大提交。
- 若某一步实施中发现超出 `461` 当前边界的新问题，必须先更新 `docs/dev-plans/` 对应 owner 文档，再继续编码。
- 若首期实现需要压缩范围，只允许删减样板覆盖面，不允许突破“知识包驱动 + 唯一执行注册表 + 复用现有读 API”三条主边界。

### 12.8 建议 PR 切分

为降低评审成本，首期建议按以下顺序拆分 PR。若实际实现中发现某两个 PR 高度耦合，可以合并，但不得跨越主边界。

#### PR-1：知识包样板落地

- 覆盖 Step 1
- 新增 `modules/orgunit/presentation/cubebox/` 下的 4 个 Markdown 文件
- 目标是把首批 `orgunit` 查询面冻结成可阅读、可评审的知识包实物
- 本 PR 不引入任何运行时代码

建议文件清单：

- [x] `modules/orgunit/presentation/cubebox/CUBEBOX-SKILL.md`
- [x] `modules/orgunit/presentation/cubebox/queries.md`
- [x] `modules/orgunit/presentation/cubebox/apis.md`
- [x] `modules/orgunit/presentation/cubebox/examples.md`

建议各文件最小内容：

- `CUBEBOX-SKILL.md`
  - 模块定位：`orgunit` 是什么、主要业务对象是什么
  - 当前页面/上下文如何帮助补参
  - 首批只允许的查询意图概览
  - 对 `queries.md`、`apis.md`、`examples.md` 的引用关系
- `queries.md`
  - 只定义 `orgunit.details`、`orgunit.list`、`orgunit.search`、`orgunit.audit`
  - 每个意图的必填参数、可选参数、缺参追问口径
  - 常见自然语言表达与意图映射
- `apis.md`
  - 只列出首批 4 个 `api_key`
  - 每个 `api_key` 的用途、参数、关注字段、权限前提
  - 明确声明“禁止直查数据库、禁止调用未列出的接口”
  - 明确声明“本文件不是执行事实源，执行事实源以后续代码注册表为准”
- `examples.md`
  - 每个意图至少 1 条高质量问法
  - 至少包含 1 个缺参追问示例
  - 至少包含 1 个多步只读编排示例
  - 示例中的 `api_key` 必须只使用首批 4 个冻结值

PR-1 验收点：

- [x] 目录与文件路径符合 `461` 冻结的模块级知识包规范
- [x] 4 个文件都能被 reviewer 独立阅读理解，不依赖运行时代码补完语义
- [x] 查询意图范围没有超出 `orgunit.details/list/search/audit`
- [x] `api_key` 命名在 4 个文件之间保持一致，没有同义漂移
- [x] 文档未引入租户级覆盖目录、租户级发布机制或第二知识运行时
- [x] 文档未声明数据库、SQL、store、内部函数等非法执行面
- [x] 至少有 1 个示例体现“一个问题下的线性多步只读编排”
- [x] `make check doc` 通过

PR-1 评审重点：

- reviewer 应重点看“知识是否清楚、边界是否收敛、`api_key` 是否稳定”，而不是提前评审运行时实现细节
- 若在 PR-1 中发现必须依赖代码实现才能写清语义，优先回到文档收敛，而不是把运行时代码夹带进这个 PR

#### PR-2：`ReadPlan` 最小契约与校验器

- 覆盖 Step 2
- 在 `modules/cubebox` 增加 `ReadPlan` 类型、结构化解析和 fail-closed 校验
- 接入最小失败语义，但暂不接模块执行器
- 本 PR 不接数据库、不接 `orgunit` 读链路

PR-2 实际落点：

- [x] `modules/cubebox/read_plan.go`
- [x] `modules/cubebox/knowledge_pack.go`
- [x] `modules/cubebox/read_plan_test.go`
- [x] `modules/cubebox/knowledge_pack_test.go`
- [x] `internal/routing/responder.go` 增加最小错误提示映射

PR-2 验收点：

- [x] `ReadPlan` 类型与 `steps[]` 最小 schema 已在代码中冻结
- [x] 结构化解析失败会命中受控失败，而不是进入执行面
- [x] 非线性 `depends_on`、缺失必要字段等边界问题会 fail-closed
- [x] 知识包缺文件、缺结构化锚点时会命中 `knowledge_pack_invalid`
- [x] `go test ./modules/cubebox/... ./internal/routing/...` 通过

#### PR-3：执行注册层骨架

- 覆盖 Step 3
- 增加 `api_key -> executor` 注册表、执行器接口和线性多步调度骨架
- 只冻结受控映射层，不引入第二套查询实现
- 本 PR 可以使用 stub executor 验证主链，但不应复制任何真实业务查询逻辑

#### PR-4：`orgunit` 首批执行器接入

- 覆盖 Step 4
- 把 `orgunit.details`、`orgunit.list`、`orgunit.search`、`orgunit.audit` 接入注册表
- 执行器只复用现有 `orgunit` 读链路
- 本 PR 重点审查“是否出现第二套实现”与“是否越过现有权限边界”

#### PR-5：接入 `CubeBox` 查询主链

- 覆盖 Step 5
- 在现有 `CubeBox` turn 主链中接入知识包加载、`ReadPlan` 生成、校验、执行和结果解释
- 保持当前会话、流式、租户和权限主链不被破坏
- 本 PR 重点审查链路是否仍然单一、失败是否 fail-closed

#### PR-6：错误语义、测试与 readiness 收口

- 覆盖 Step 6
- 接入 `knowledge_pack_invalid` 与 `api_catalog_drift_or_executor_missing`
- 补最小稳定测试与 `docs/dev-records/` 证据
- 本 PR 重点审查可回归性、错误提示和首期范围是否闭合

#### 合并原则

- `PR-1` 到 `PR-3` 解决“能否建立受控查询骨架”。
- `PR-4` 到 `PR-5` 解决“首个模块样板能否接入并跑通”。
- `PR-6` 负责把首期闭环收口到可验证状态。
- 若时间或风险受限，允许在 `PR-4` 只先接一个 `api_key` 做最小样板，但必须在文档和 readiness 中显式记录缩范围结果。

## 13. 关联文档

- `docs/dev-plans/460-cubebox-digital-assistant-positioning-and-execution-contract.md`
- `docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`
- `docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md`
