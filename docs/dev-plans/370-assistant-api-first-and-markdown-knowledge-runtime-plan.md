# DEV-PLAN-370：Assistant API-First 与 Markdown Knowledge Runtime 主链收敛方案

**状态**: 进行中（2026-04-13 10:58 CST）

## 1. 背景与问题定性

### 1.1 背景

`DEV-PLAN-350` 已明确 Assistant 的业务操作必须收敛到统一策略主链，避免 Assistant 自带第二套策略解释器；`DEV-PLAN-360/360A` 已明确 LibreChat 只能保留 UI 壳，运行时主链必须由仓库内可控层接管；`DEV-PLAN-361` 已明确 OPA/PDP 的采用边界，业务授权与准入判定不能散落在各处私有逻辑中。

当前 Assistant 仍存在两类结构性问题：

- 业务事实与业务解释耦合，部分运行时逻辑仍直接读取底层策略、字段配置或手工登记内容，形成第二解释面。
- 知识资产已有结构化运行时产物，但作者体验长期停留在“散落代码 + 手工 JSON + 局部约定”阶段，不利于持续整理、审阅、扩展与版本管理。

Karpathy 的 LLM Wiki 思路对本仓的启发不在于“让 Markdown 替代业务数据库”，而在于把“知识编排层”降到文件夹级复杂度，让知识以可读、可 diff、可 review、可编译的形式存在，同时保持业务事实仍由正式 API 主链提供。

### 1.2 历史问题诊断：为什么知识会散落

1. [ ] 历史上并不存在一个统一的 Assistant Knowledge SoT，只有按实现入口分散的局部 SoT。
2. [ ] `internal/server/assistant_action_registry.go` 长期同时承担“动作 contract”与“动作说明性知识”两类职责，导致 contract / knowledge 混写。
3. [ ] `internal/server/assistant_api.go` 内部长期保留计划摘要、验证解释、用户提示等知识型文本，形成 API handler 自带知识片段。
4. [ ] `internal/server/assistant_reply_nlg.go` 内部长期保留大量 fallback 文案与业务解释，形成 reply 侧知识散点。
5. [ ] `internal/server/assistant_knowledge/*.json` 虽然承载了一部分结构化资产，但历史上并不是由统一 compiler 从单主源生成，而是与代码散点共同构成运行时知识输入。
6. [ ] 因此，历史实际存在的不是“全局 SoT”，而是“按入口和实现阶段堆积出来的分布式局部 SoT”；这正是 370 系列必须切断的对象。

### 1.3 问题定性

本方案要解决的不是“如何把 Assistant 做成 RAG”，而是以下主线问题：

- Assistant 驱动查询或操作时，运行时应全部通过受控 API 获取实时事实，不得直接触碰 DB、策略表、字段配置表等底层实现细节。
- Assistant 的解释知识、工具说明、查询剧本、回复指导，应从 Markdown 文件夹维护，再编译为结构化运行时资产；运行时不直接消费原始 Markdown，也不直接消费 `docs/dev-plans/`。
- 知识层不得再维持“Markdown + 手工 JSON + 代码散点文本 + legacy overlay”的混源模式；必须收敛为单主源与单消费面。

### 1.4 核心判断

1. [ ] Markdown 只负责“编排知识”，不负责“业务真相”。
2. [ ] API 只负责“事实读取/动作提交”，不负责“知识撰写”。
3. [ ] Runtime 只消费“受控 API + compiler 产物”，不消费“底层存储 + 人工散装配置 + 代码内知识文本”。
4. [ ] `DEV-PLAN-370` 只负责 Assistant Runtime / Knowledge 层的收敛，不作为覆盖 `350/360/360A/361` 的总母法。

## 2. 目标、非目标与边界

### 2.1 目标

1. [ ] 建立 Assistant 的 `API-first` 主链：所有查询与操作均通过正式 API 或 Tool API 取得事实，不允许运行时直连底层存储。
2. [ ] 建立 Assistant 的 `Markdown-first` 知识主源：所有编排知识统一由 `internal/server/assistant_knowledge_md/` 维护。
3. [ ] 将 `internal/server/assistant_knowledge/` 收敛为 compiler 唯一输出目录，禁止人工改写与第二写入口。
4. [ ] 冻结查询链、操作链、提交链的责任分界，避免第二解释器、第二策略主链、第二知识真相源继续扩散。
5. [ ] 通过一次性 hard cut 删除 legacy overlay / pass-through / mixed-source runtime，而不是保留长期渐进兼容带。

### 2.2 非目标

1. [ ] 不以 Markdown 替代 PostgreSQL、事件流、projection、PDP、PrecheckProjection 等业务事实源。
2. [ ] 不在本方案中引入向量库、embedding、RAG 平台、知识图谱数据库、专门 CMS 作为正式依赖。
3. [ ] 不把 `actions/*.md` 升格为动作 API / Tool API contract 主源。
4. [ ] 不直接把 `docs/dev-plans/` 当作运行时知识目录；契约文档与运行时知识必须分离。
5. [ ] 不接受“query 先迁、action 永久 overlay”“人工 JSON 与 Markdown 长期共存”“代码散点继续保留作兜底”这类长期迁移形态。

### 2.3 与既有计划的边界

1. [ ] `DEV-PLAN-350`：是策略与 Authoritative Gate 收敛母法，继续拥有 `ActionSchema`、`PolicyContext`、`PrecheckProjection`、`Readonly Tool Registry` 的契约裁决权；本方案只定义这些契约在 Runtime / Knowledge 层如何被消费与校验，不改其权威归属。
2. [ ] `DEV-PLAN-360`：是 Assistant / LibreChat 分层角色、停线、失败原则与防回流约束的架构母法；本方案只在其冻结边界内定义 Runtime / Knowledge 形态。
3. [ ] `DEV-PLAN-360A`：是 successor DTO / API / compat 生死表 / `runtime-status` 语义的执行面 SSOT；本方案只消费其单入口与切换约束，不单独定义这些对外契约细节。
4. [ ] `DEV-PLAN-361`：定义 OPA/PDP 采用边界；本方案要求 Assistant 只能通过其暴露的能力边界消费授权判断，不得本地重演授权逻辑。
5. [ ] `docs/archive/dev-plans/240E/241/244/245` 等历史计划文档仅保留历史背景价值，不再参与 `370` 系列的现行裁决，不得作为继续保留 overlay、pass-through、partial ownership 的依据。
6. [ ] 因此，`DEV-PLAN-370` 的正式定位是“Assistant Runtime / Knowledge Convergence Plan”，而不是覆盖 `350/360/360A/361` 的总母法。

### 2.4 API-first 裁决规则

1. [ ] 凡属于 Runtime 获取实时业务事实、策略裁决、预检查结果、提交准入结果所依赖的正式 API / Tool API 契约，一律由 `DEV-PLAN-350` 裁决；`370` 只消费，不单独定义新的事实面权威表达。
2. [ ] 凡属于 Markdown 作者体验、knowledge compiler、reply guidance、wiki/index、intent route 编排资产的主源、格式与生成方式，一律由 `370` 裁决；`350` 不替代知识资产主源。
3. [ ] 若某能力同时跨越“知识编排”和“事实读取”，必须拆成两层：
   - 事实读取/API 契约层：先回写 `350`
   - Markdown/编译产物/回复编排层：回写 `370`
4. [ ] 仅在 Markdown 中出现某个 tool、topic、query plan，并不自动赋予其“正式 Tool API”资格；一旦进入 Runtime 正式调用面，其 `tool_name`、schema、错误语义、版本口径必须先由 `350` 或其继承的 registry SSOT 冻结。

### 2.5 与 `DEV-PLAN-375` 的路线图关系

1. [X] `DEV-PLAN-375` 已作为 `350-370` 的编排母法建立；本计划继续是 Runtime / Knowledge 层与 Markdown knowledge runtime 的 SSOT。
2. [X] 为避免同时扩张 `business_action` contract 与 knowledge runtime，后续实施顺序冻结为：
   - `370A`：单主源准备，完成 Markdown 主源、compiler、全量 generated-clean 与反回流门禁
   - `370B`：在 `350A / 350B / 350C` 全部完成后，对 runtime 执行一次性 hard cut，并删除旧知识入口
3. [X] `370A` 不承接 `assistantActionSpec`、Tool registry、`PolicyContextContractVersion`、`PrecheckProjectionContractVersion` 的裁决；这些仍由 `350` 扩张。
4. [X] `370B` 必须等 `350A / 350B / 350C` 全部完成后再启动，且只能消费已冻结的动作 contract，不得反向定义新的动作 API / Tool API 主源。

### 2.6 单主源 Hard Cut 规则

1. [ ] `internal/server/assistant_knowledge_md/` 是唯一允许人工维护的 Assistant Runtime Knowledge 主源。
2. [ ] `internal/server/assistant_knowledge/` 只允许由 compiler 生成；任何人工编辑、脚本旁路写入或第二写入口都视为违规。
3. [ ] 禁止 `legacy overlay`、`legacy pass-through`、`partial ownership`、`mixed-source runtime` 作为正式迁移方案。
4. [ ] `source_refs` 只能指向现行事实源与现行契约文档，不得引用 `docs/archive/**`。
5. [ ] `assistant_action_registry.go` 只允许保留执行 contract 与注册职责，不得继续持有业务知识文本、回复模板或解释性摘要。
6. [ ] `assistant_api.go`、`assistant_reply_nlg.go` 只允许保留最小技术 fallback，不得继续持有业务知识型文本。
7. [ ] 若 Markdown 产物与 `350` 已冻结 contract 冲突，必须 fail-closed，而不是由 runtime 在本地兜底解释。
8. [ ] Hard cut 后，runtime 只允许从单一 compiler 产物面读取知识，不得继续混读代码常量、手工 JSON 或 archive 文档。

## 3. 真相矩阵与目标架构

### 3.1 真相矩阵

| 层次 | 主源 | 运行时消费方式 | 禁止事项 |
| --- | --- | --- | --- |
| 业务事实 | DB / Projection / WriteService / PDP / PrecheckProjection | 只通过 API / Tool API 读取 | Runtime 直连表、直读策略配置、直读字段配置 |
| 动作 contract | `DEV-PLAN-350` + 其冻结 registry/DTO | Runtime 按正式 contract 消费 | 由 Markdown 或 reply 文案反向定义 contract |
| 编排知识 | `internal/server/assistant_knowledge_md/` | 先编译，再由 runtime 读取结构化产物 | 人工直接维护 JSON 产物；runtime 直接读取 docs；代码常量继续充当知识主源 |
| 提交准入 | Authoritative Gate / CommitAdapter | 统一提交 API | Assistant 本地判断是否可提交 |
| 回复生成 | Reply Guidance + API 返回事实 | Runtime 组合生成 | 直接用自由文本绕开明确错误契约 |

### 3.2 目标架构

1. [ ] UI 层  
继承 `DEV-PLAN-360A` 已冻结的 successor 单入口约束，通过现有 `/internal/assistant/*` 入口承接会话、turn、task、runtime-status，不新增第二套前台协议，也不在本文重复定义 DTO / 错误码 / compat 生死表。
2. [ ] Runtime 编排层  
负责意图识别、知识读取、工具调用、回复生成、任务推进；只允许访问编译后的知识资产、受控 Tool API、统一提交 API。
3. [ ] Tool / Query API 层  
向 Runtime 暴露只读事实接口与预检查接口，例如候选实体查找、快照读取、字段解释、预检查结果、知识条目检索。
4. [ ] Authoritative Commit 层  
唯一允许进入写链的入口，继续走 `authoritative gate -> commit adapter -> write service -> DB kernel`。

### 3.3 查询、知识问答与操作三条主链

1. [ ] `business_query`
面向实时业务查询，事实必须来自 API。
2. [ ] `knowledge_qa`
面向解释性、制度性、流程性、产品性知识问答，事实来自编译后的 Markdown 知识资产，不承担实时业务真相职责。
3. [ ] `business_action`
面向创建、修改、提交、审批等操作，必须先经过预检查，再进入统一提交主链。
4. [ ] 现有 `chitchat` 与 `uncertain` 继续保留，但不得吞并上述三条正式主链。

## 4. Markdown Knowledge Runtime 方案

### 4.1 目录策略

1. [ ] 运行时知识目录与契约文档目录分离，正式知识源目录固定为：
   - `internal/server/assistant_knowledge_md/intent/`
   - `internal/server/assistant_knowledge_md/actions/`
   - `internal/server/assistant_knowledge_md/replies/`
   - `internal/server/assistant_knowledge_md/tools/`
   - `internal/server/assistant_knowledge_md/wiki/`
2. [ ] `docs/dev-plans/` 继续是契约设计 SSOT，不作为运行时 prompt/source 目录。
3. [ ] `internal/server/assistant_knowledge/` 继续保留为编译产物目录或嵌入式运行时资产目录。
4. [ ] Markdown 是唯一人工维护主源；JSON 或 embed 资产是唯一运行时主源。

### 4.2 知识分类

1. [ ] `intent/*.md`
定义用户表达、意图别名、歧义消解、主链归属、所需槽位与工具/知识引用。
2. [ ] `actions/*.md`
定义动作说明、适用前提、输入槽位、预检查说明、proposal/reply 指导；其内容属于说明性知识，不拥有动作 contract 裁决权。
3. [ ] `replies/*.md`
定义回复指导、错误提示映射、缺字段引导、成功确认模板。
4. [ ] `tools/*.md`
定义工具契约说明、输入输出示例、字段解释、何时调用、何时禁止调用；其引用的正式 `tool_name/schema` 必须来自 `350` 已冻结 contract。
5. [ ] `wiki/*.md`
定义流程知识、业务术语、产品解释、非实时说明性知识。

### 4.3 Markdown 文件结构

1. [ ] 每个 Markdown 文件必须采用“前置元数据 + 正文”两段式，前置元数据建议使用 YAML front matter，至少包含：
   - `id`
   - `title`
   - `locale`
   - `kind`
   - `version`
   - `source_refs`
   - `applies_to`
   - `status`
2. [ ] 按类型补充：
   - `intent`：`route_kind`、`required_slots`、`tool_refs`、`wiki_refs`
   - `action`：`action_key`、`required_checks`、`proposal_template`
   - `reply`：`reply_key`、`error_codes`
   - `tool`：`tool_name`、`input_schema_ref`、`output_schema_ref`
   - `wiki`：`topic_key`、`related_topics`
3. [ ] `source_refs` 必须指向现行仓库内文件，且不得包含 `docs/archive/**`。

### 4.4 编译原则

1. [ ] 建立 Knowledge Compiler，将 Markdown 编译为运行时结构化资产，至少产出：
   - `intent_route_catalog.json`
   - `interpretation/*.json`
   - `action_view/*.json`
   - `reply_guidance/*.json`
   - `tool_catalog.json`
   - `wiki_index.json`
2. [ ] 保留并继续强化以下审计字段：
   - `source_refs`
   - `knowledge_snapshot_digest`
   - `route_catalog_version`
   - `resolver_contract_version`
   - `context_template_version`
   - `reply_guidance_version`
3. [ ] 编译失败必须 fail-closed；任何缺失主键、重复 ID、非法 front matter、未注册 tool、坏链接、archive 引用、contract 冲突，都应阻断生成。
4. [ ] `actions/*.md`、`tools/*.md` 的编译结果不得自行发明策略字段；凡涉及 `PolicyContext`、`PrecheckProjection`、`RequiredChecks`、`Readonly Tool Registry` 的运行时契约，统一对齐 `DEV-PLAN-350`。

## 5. API-First 运行时收敛

### 5.1 UI API 保持单入口

1. [ ] 用户界面继续只使用现有 Assistant API：
   - `/internal/assistant/conversations`
   - `/internal/assistant/conversations/{conversation_id}/turns`
   - `/internal/assistant/tasks`
   - `/internal/assistant/runtime-status`
2. [ ] 不得因 Markdown 知识运行时引入第二套对外 Assistant UI 协议。
3. [ ] `/internal/assistant/*` successor DTO、错误码、`runtime-status` 字段生死表继续以 `DEV-PLAN-360A` 为准；`370` 只消费其单入口约束，不单独升格为执行面权威。

### 5.2 Tool API 分层

1. [ ] 为 Runtime 提供内部只读 API，而不是让 Runtime 直接读底层模块状态。
2. [ ] 运行时可消费的正式业务工具以 `350` 为准；`370` 不单独扩张正式 registry。
3. [ ] 若后续需要把知识检索能力暴露为正式 Tool API（例如 `wiki_lookup`），必须先回写 `350` 冻结其注册名、schema、错误语义与版本，再由 `370` 在 Markdown `tools/*.md` 中引用。
4. [ ] Tool API 只返回受控 DTO，不暴露底层表结构。
5. [ ] Tool API 可以调用 service / projection / PDP / resolver，但 Runtime 不得感知其内部来源。
6. [ ] 策略类 Tool API 必须复用 `DEV-PLAN-350` 已冻结的 `PrecheckProjection` 或其受控子视图，不得在 Runtime 侧发明第二套策略 DTO。

### 5.3 提交链冻结

1. [ ] 所有业务操作必须遵守以下链路：  
`Assistant Runtime -> proposal/precheck -> authoritative gate -> commit adapter -> write service -> DB kernel`
2. [ ] 禁止：
   - Runtime 直接调用写服务
   - Runtime 自己拼接策略结论
   - Runtime 绕过 Gate 直接提交
   - Runtime 通过知识文件声明“可直接写入”

### 5.4 热点漂移点治理

1. [ ] `assistant_create_policy_precheck.go`
目标：移除直接读取底层策略/字段配置的解释职责，改由统一 Tool/API 返回预检查结果。
2. [ ] `assistant_action_registry.go`
目标：只保留动作 contract、注册与执行装配职责；动作说明、proposal/reply guidance、plan 摘要不得继续驻留其中。
3. [ ] `assistant_knowledge_runtime.go`
目标：保留运行时消费职责，但知识输入必须收敛到 compiler 产物。
4. [ ] `assistant_api.go`
目标：只保留协议装配与最小技术 fallback；计划摘要、验证解释、业务提示迁出到 Markdown 编译产物。
5. [ ] `assistant_reply_nlg.go`
目标：继续保留回复组装职责，但业务知识型 fallback 文案迁出到 Markdown 编译产物；仅保留最小技术降级文案。

## 6. 运行时行为规范

### 6.1 `business_query`

1. [ ] 处理流程冻结为：意图识别 -> 读取 intent 编译产物 -> 调用 Tool API 获取实时事实 -> 组合 reply guidance -> 输出带事实依据的回复。
2. [ ] 不得把实时事实缓存进 Markdown。
3. [ ] 缺少必要槽位时必须走追问，而不是猜测。

### 6.2 `knowledge_qa`

1. [ ] 处理流程冻结为：意图识别 -> 检索 wiki/reply guidance 编译产物 -> 必要时调用只读 Tool API 补充上下文 -> 输出结构化说明。
2. [ ] `knowledge_qa` 不承担策略裁决。
3. [ ] 不允许用 wiki 内容替代实时组织状态、权限结果、字段规则结果。

### 6.3 `business_action`

1. [ ] 处理流程冻结为：意图识别 -> 读取 action / intent / tool / reply 编译产物 -> 收集槽位 -> 调用预检查 Tool API -> 生成 proposal -> 经 authoritative gate 判定 -> 进入正式提交链 -> 生成成功/失败回复与 task 状态。
2. [ ] 任何“是否允许做”必须以 API / Authoritative Gate 返回为准。
3. [ ] Markdown 只能解释“通常需要什么”，不能解释“当前用户一定能不能做”。
4. [ ] `proposal / precheck / gate / commit` 的正式裁决语义继续以 `DEV-PLAN-350` 为准；本方案只定义 Runtime 如何围绕该主链组织知识与调用顺序。

## 7. 编译链、门禁与测试

### 7.1 新门禁

1. [ ] `make check assistant-knowledge-single-source`
阻断第二人工写入口、手工 JSON 回写与 Markdown/JSON ownership 漂移。
2. [ ] `make check assistant-knowledge-generated-clean`
校验编译产物与源文件一致，防止漏生成或手改产物。
3. [ ] `make check assistant-no-legacy-overlay`
阻断 overlay、pass-through、mixed-source runtime 等回流方案。
4. [ ] `make check assistant-no-knowledge-literals`
阻断 `assistant_action_registry.go`、`assistant_api.go`、`assistant_reply_nlg.go` 等处新增业务知识型文本。
5. [ ] `make check assistant-knowledge-no-archive-ref`
阻断 `source_refs` 或编译输入引用 `docs/archive/**`。
6. [ ] `make check assistant-knowledge-contract-separation`
阻断 Markdown 反向定义动作 contract，也阻断 contract 代码继续持有说明性知识。
7. [ ] `make check assistant-api-only`
阻断 Runtime / Resolver 直接读取底层策略表、字段配置表、手工 registry 数据源。
8. [ ] `make check assistant-no-knowledge-db`
阻断知识层引入数据库、向量库、外部知识平台依赖作为正式主源。

### 7.2 测试面

1. [ ] 编译器测试
   - Markdown 解析
   - front matter 校验
   - 重复 ID / 坏引用 / archive 引用 / 未注册 tool / 非法版本
   - contract 冲突 fail-closed
   - 产物 digest 稳定性
2. [ ] Runtime 单元测试
   - `business_query` 与 `knowledge_qa` 分流
   - `business_action` 槽位收集、预检查调用、proposal 生成
   - reply guidance 选择与错误映射
   - hard cut 后单一知识消费面断言
3. [ ] API 集成测试
   - Tool API 返回契约稳定
   - Runtime 通过 API 获取事实
   - Runtime 不再直接触碰底层策略实现
4. [ ] 端到端测试
   - 查询实时组织信息
   - 查询解释性知识
   - 发起创建/更正类操作
   - 缺字段追问
   - 预检查失败
   - Gate 拒绝
   - 提交成功并返回 task / turn 结果

### 7.3 验收标准

1. [ ] Assistant 查询/操作主链全部通过 API 或 Tool API 获取事实。
2. [ ] Runtime 不再直接读取底层策略/字段配置/store/手工 registry 作为业务判断来源。
3. [ ] 运行时知识主源已迁移到 `assistant_knowledge_md/`。
4. [ ] `assistant_knowledge/` 中运行时资产全部由 compiler 生成，并可由门禁校验一致性。
5. [ ] Hard cut 后，runtime 不再混读代码散点文本、手工 JSON 或 legacy overlay。
6. [ ] `assistant_action_registry.go` 仅保留 contract；`assistant_api.go`、`assistant_reply_nlg.go` 不再持有业务知识型文本。
7. [ ] 新门禁已接入 CI 并具备 fail-closed 能力。
8. [ ] 至少一个查询链、一个知识问答链、一个操作链完成 E2E 验证。

## 8. 分阶段实施

> 为避免 `business_action` contract 扩张与 knowledge/runtime cutover 相互缠绕，本计划的后续实施顺序由 `DEV-PLAN-375` 冻结为 `370A -> 370B`。  
> `370A` 负责单主源准备；`370B` 负责一次性 hard cut。

### 8.1 Phase 0：契约冻结

1. [ ] 冻结本方案的真相矩阵、目录策略、分类模型、主链边界。
2. [ ] 冻结 `business_query / knowledge_qa / business_action` 三类主链定义。
3. [ ] 冻结单主源 hard cut 规则与反回流门禁集合。

### 8.2 Phase 1：`370A` 单主源准备

1. [ ] 建立 `assistant_knowledge_md/` 目录结构。
2. [ ] 建立 front matter 与 compiler schema。
3. [ ] 完成全量 Markdown -> JSON/Embed 编译链。
4. [ ] 让 `assistant_knowledge/` 全部转为 generated-clean。
5. [ ] 接入 single-source / no-overlay / no-literals / no-archive-ref / contract-separation 等门禁。

### 8.3 Phase 2：`350A/B/C` 动作 contract 冻结

1. [ ] 完成 `350A / 350B / 350C`，使 `business_action` 所需正式 contract 全部冻结。
2. [ ] 370 系列只消费其结果，不在本阶段补写动作 contract。

### 8.4 Phase 3：`370B` 一次性 Hard Cut

1. [ ] Runtime 统一切到 compiler 产物单消费面。
2. [ ] 删除 overlay / pass-through / mixed-source runtime。
3. [ ] 删除 `assistant_action_registry.go`、`assistant_api.go`、`assistant_reply_nlg.go` 中残留知识入口。
4. [ ] 完成 hard cut 回归、E2E 与门禁接线。

### 8.5 Phase 4：稳定化

1. [ ] 固化 runtime-status 对知识 digest / 编译版本 / tool contracts 的可观测性；如需对外字段，必须回写 `DEV-PLAN-360A`。
2. [ ] 清理 hard cut 后残留死分支、旧 JSON 维护脚本与历史兜底逻辑。
3. [ ] 将 `370` 收口到“完成或仅剩独立缺陷修复”状态。

## 9. 重要接口与类型变更

### 9.1 新增内部概念

1. [ ] `business_query`
2. [ ] `tool_catalog`
3. [ ] `wiki_index`
4. [ ] `knowledge compiler`
5. [ ] `assistant_knowledge_md` 单主源目录

### 9.2 运行时类型变更方向

1. [ ] `intent route` 需要支持 `business_query`。
2. [ ] `action view` 需要支持从 Markdown 编译出的说明性字段，但不得越权定义动作 contract。
3. [ ] `reply guidance` 需要支持错误码映射与缺字段追问模板。
4. [ ] 如需让 `runtime-status` 暴露知识 digest、编译版本、tool catalog 版本，其字段合同应回写 `DEV-PLAN-360A`，由执行面 SSOT 冻结。

### 9.3 API 兼容性

1. [ ] 对外 Assistant UI API 尽量保持兼容。
2. [ ] 内部 Tool API 可新增，但不得破坏已有 `/internal/assistant/*` 主协议。
3. [ ] 如需新增 `runtime-status` 字段，应以向后兼容方式扩展，并由 `DEV-PLAN-360A` 统一冻结最小 DTO / 错误码 / 失败语义。

## 10. 假设与默认决策

1. [ ] 默认采用“Markdown 主源 + compiler 产物 + runtime 消费”的三级模式，而非 Markdown 直接运行。
2. [ ] 默认不引入向量库、RAG API、外部知识平台作为正式知识层。
3. [ ] 默认 `docs/dev-plans/` 不参与运行时知识读取，避免契约文档与运行时知识混源。
4. [ ] 默认不接受 `legacy overlay`、`pass-through`、`partial ownership` 作为长期迁移形态。
5. [ ] 默认优先在 OrgUnit / Assistant 现有链路上完成样板闭环，再向其他模块复制。
6. [ ] 默认 `DEV-PLAN-370` 作为 Assistant Runtime / Knowledge 层收敛子法：承接 `API-first + Markdown-first + compiler + business_query / knowledge_qa / business_action`，但不改写 `350` 的策略母法、`360` 的架构母法、`360A` 的执行面 SSOT、`361` 的 PDP 边界法。

## 11. 关联事实源

1. [ ] `DEV-PLAN-350`：Assistant Tooling 与统一策略模型收敛。
2. [ ] `DEV-PLAN-360`：LibreChat 去能力化与分层接管。
3. [ ] `DEV-PLAN-360A`：LibreChat 特性禁用与运行时切换。
4. [ ] `DEV-PLAN-361`：OPA/PDP 采用边界与迁移。
5. [ ] `DEV-PLAN-375`：Assistant 主线实施路线图（350-370）。
6. [ ] `DEV-PLAN-370A`：单主源、compiler 与反回流门禁准备。
7. [ ] `DEV-PLAN-370B`：一次性 runtime hard cut 与旧知识入口删除。
8. [ ] 现状代码参考：
   - `internal/server/assistant_knowledge_runtime.go`
   - `internal/server/assistant_action_registry.go`
   - `internal/server/assistant_create_policy_precheck.go`
   - `internal/server/assistant_reply_nlg.go`
   - `internal/server/assistant_intent_router.go`
   - `internal/server/assistant_api.go`
   - `internal/server/assistant_runtime_status.go`
