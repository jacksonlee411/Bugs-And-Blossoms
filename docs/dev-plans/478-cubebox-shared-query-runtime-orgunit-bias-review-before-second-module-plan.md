# DEV-PLAN-478：CubeBox 第二业务模块接入前共享 Query Runtime 去 `orgunit` 污染复核方案

**状态**: 规划中（2026-04-28 06:52 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：在第二个业务模块接入 `CubeBox` 查询链之前，系统性复核共享 narrator、共享 query flow、共享 query context / evidence window、共享 knowledge pack 装配、执行注册表校验与 `no_query` guidance 是否仍被 `orgunit` 场景单边塑形，并冻结调查分类、owner 边界、实施顺序、测试准入与 stopline。
- **关联模块/目录**：`docs/dev-plans/468-cubebox-session-continuity-and-model-autonomy-improvement-plan.md`、`docs/dev-plans/460-cubebox-digital-assistant-positioning-and-execution-contract.md`、`docs/dev-plans/461-cubebox-query-scenarios-minimal-contract.md`、`docs/dev-plans/464-cubebox-query-architecture-convergence-plan.md`、`docs/dev-plans/466-cubebox-query-owner-drift-and-anti-backflow-investigation-plan.md`、`internal/server/cubebox_query_flow.go`、`internal/server/cubebox_query_flow_test.go`、`internal/server/cubebox_orgunit_executors.go`、`modules/cubebox/knowledge_pack.go`、`modules/cubebox/query_entity.go`、`modules/cubebox/read_executor.go`、`modules/orgunit/presentation/cubebox/*`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-003`、`DEV-PLAN-460`、`DEV-PLAN-461`、`DEV-PLAN-464`、`DEV-PLAN-466`、`DEV-PLAN-468`、`DEV-PLAN-471`、`DEV-PLAN-472`、`DEV-PLAN-473`、`DEV-PLAN-474`、`DEV-PLAN-476`、`DEV-PLAN-477`
- **用户入口/触点**：主应用壳层右侧 `CubeBox` 抽屉、`/internal/cubebox/turns:stream`、共享 planner/narrator/clarifier/no-query guidance 链路

### 0.1 Simple > Easy 三问

1. **边界**：本计划复核的是“共享层是否残留 `orgunit` 偏置”，不是现在就接第二模块，也不是顺手重写 `orgunit` 自身知识包。
2. **不变量**：模块私有语义必须留在模块知识包或模块 executor 侧；共享层只持有通用查询 contract、fail-closed 边界、结构化事实窗口和受控用户回答约束；共享 `QueryContext` / `QueryEntity` / metadata event 不得继续增加模块专属字段。
3. **可解释**：reviewer 必须能在 5 分钟内分清三类命中：哪些属于共享层污染，哪些属于模块实现本来就该存在，哪些只是测试夹具用 `orgunit` 作为当前唯一样例。

### 0.2 与 `DEV-PLAN-473` 的关系

`DEV-PLAN-473` 已冻结 CubeBox 查询链的模型主导方向：本地提供中性事实、工具目录、硬约束校验和执行；模型负责语义判断、目标选择、澄清恢复、日期补全、候选集合解释和多步只读编排。

因此，本计划在去 `orgunit` 污染时必须遵守以下边界：

1. 不把 `orgunit` 专用 scope correction 改造成通用本地 scope engine、模块 scope plugin 或关键词解释器。
2. 不把 `orgunit` candidate extraction 改造成按模块 payload 猜测 target 的通用本地 selector。
3. 不新增 `selected_target`、`target_binding`、`current_entity`、`working_set` 一类执行事实源。
4. 不因为第二模块接入而恢复入口级静态菜单、保守拒绝、slot repair 或本地 NLU。
5. 若模型输出不稳，优先改善 `query_evidence_window`、knowledge pack、`read_api_catalog`、prompt 或 loop 约束；不得回退到 Go 代码自然语言分支。

## 1. 背景与问题定义

`DEV-PLAN-468` 当前剩余主线之一，已经明确登记为“第二业务模块接入后的共享 narrator 去模块化污染复核”。这条线现在不是 blocker，但如果不在第二模块落地前先冻结调查与收口方案，后续很容易出现两类问题：

1. 第二模块必须迁就当前 `orgunit` 口径去写知识包、返回 payload 结构和 no-query guidance，导致共享层事实 owner 漂移。
2. 共享 narrator / query flow 会继续把 `orgunit` 的字段、示例、错误语义和纠偏逻辑误当成“通用查询 contract”的一部分。

当前代码主链已经有明显信号表明这不是假想风险，而是现实残口：

1. narrator 共享 prompt 仍使用 `orgunit` 示例回答，例如“组织 100000 是飞虫与鲜花”，见 `internal/server/cubebox_query_flow.go:891-915`。
2. clarifier 共享 prompt 仍写“可以用于组织追问”，把某一模块对象类型写进共享说明，见 `internal/server/cubebox_query_flow.go:962-968`。
3. planner correction / scope override 仍直接要求重规划 `orgunit.list`，并写死 `all_org_units=true`、`org_code`、`parent_org_code` 等字段，见 `internal/server/cubebox_query_flow.go:1238-1243`。
4. `queryPromptMentionsUnsupportedOrgUnitDimension(...)` 仍以内置 `orgunit` 术语列表决定是否把 boundary violation 降为 `NO_QUERY`，见 `internal/server/cubebox_query_flow.go:1255-1285`。
5. `allOrgScopePlanHasNarrowingConflict(...)` 仍把 `orgunit.details/search/audit/tree/list` 当作共享层内建知识，见 `internal/server/cubebox_query_flow.go:1394-1413`。
6. `extractQueryCandidatesFromPayload(...)` 仍直接假设共享候选提取只面向 `org_units` / `target_org_code` 结构，并把 domain 固定成 `orgunit`，见 `internal/server/cubebox_query_flow.go:1524-1564`。
7. 查询流构造函数当前只装载 `modules/orgunit/presentation/cubebox` 这一条知识包目录，`NoQueryGuidanceFromKnowledgePacks(...)` 也只返回第一份命中的 guidance，说明多模块装配路径尚未真正复核，见 `internal/server/cubebox_query_flow.go:1666`、`1858-1865` 与 `modules/cubebox/knowledge_pack.go:177-205`。
8. `modules/cubebox/query_entity.go` 的共享 `QueryEntity` 仍包含 `TargetOrgCode` / `ParentOrgCode` 这类 `orgunit` 字段，说明 evidence window 的数据结构本身也存在单模块塑形风险。
9. `newCubeboxQueryFlow(...)` 当前对每份 knowledge pack 直接调用 `ValidateKnowledgePackAgainstRegistry(pack, registry)`；而该校验会要求单份 pack 覆盖注册表全部 executor_key。第二模块接入后，如果继续保持“每个模块一份 pack”，就需要先冻结 aggregate 校验或 pack 分组校验策略，否则多模块装配会被当前单模块假设阻断。
10. 当前共享层测试大量使用 `orgunit` 作为唯一 fixture，缺少一个非 `orgunit` 假模块证明 narrator、planner prompt、no-query guidance、candidate extraction、terminal error fallback 与 registry validation 都不依赖组织字段。

因此，本计划的目的不是提前改代码，而是先把“什么算污染、谁来收、按什么顺序收”冻结成独立 owner 文档。

## 2. 核心目标

1. [ ] 冻结共享 narrator / clarifier / no-query guidance / planner correction 中的 `orgunit` 污染命中清单。
2. [ ] 明确共享 query runtime 与模块知识包/模块 executor 的 owner 边界，避免第二模块接入时继续把模块语义回灌到共享层。
3. [ ] 冻结多知识包装配的最小策略：共享层如何装配多个知识包、如何把 knowledge pack 与 `ExecutionRegistry` 成组校验、如何汇总或选择 no-query guidance、哪些规则继续由模块知识包自带。
4. [ ] 冻结共享 `QueryContext` / `QueryEntity` / metadata event 的字段归属，明确现有 `orgunit` 字段是过渡债务、模块事实，还是必须迁出的共享层污染。
5. [ ] 将 `DEV-PLAN-473` 的模型主导原则固化为 478 的整改 stopline：去模块化污染不得新增本地语义裁决层。
6. [ ] 产出分批整改切片、非 `orgunit` fixture 验证口径与 stopline，让后续实现 PR 能逐步落地而不引入“第二模块先兼容、共享层后收”的回流窗口。

## 3. 非目标

1. 不在本计划内直接接入第二业务模块。
2. 不在本计划内重写 `orgunit` 知识包业务语义或 `orgunit` executor 返回面。
3. 不在本计划内引入新的通用平台名词，例如 capability matrix engine、tool ontology、answer template system。
4. 不在本计划内实施 per-api 授权；该条仍由 `DEV-PLAN-468 P2-2` 持有。
5. 不在本计划内建设通用本地 NLU、scope engine、slot repair engine、target selector、candidate state machine 或 answer template system；这类方向已被 `DEV-PLAN-473` 明确排除。

## 4. 调查分类与当前发现

### 4.1 类别 A：共享 prompt / guidance 残留单模块示例与措辞

当前发现：

1. narrator system prompt 的“好回答/坏回答”示例直接写死 `orgunit` 业务事实。
2. clarifier system prompt 直接出现“组织追问”措辞。
3. `fallbackNoQueryGuidanceText(...)` 本身是通用模板，但当前 `NoQueryGuidanceFromKnowledgePacks(...)` 返回的是第一份命中的模块 guidance，若未来多个模块都提供 guidance，当前共享层缺少选择或聚合策略。
4. `internal/server/cubebox_query_flow_test.go` 中大量共享测试直接把 `orgunit` 文案当成默认 contract，而不是把它们明确标注为“当前唯一模块样例夹具”。

风险：

1. 第二模块接入后，narrator/clarifier/no-query guidance 的共享 contract 很容易继续围绕 `orgunit` 用词演化。
2. 测试会把单模块示例误固化成共享层外部行为。

owner 冻结：

- 共享层只应保留“结构化事实如何转成用户回答”的通用要求，不应把某个模块对象类型写进共享 prompt。
- 模块示例、模块建议问法和模块业务名词应由对应 knowledge pack 持有。

### 4.2 类别 B：共享 planner correction / scope correction 残留 `orgunit` 专用纠偏

当前发现：

1. `plannerOutcomeConflictsWithAllOrgScopeCorrection(...)` 和 `allOrgScopePlanHasNarrowingConflict(...)` 直接把 `orgunit.list/details/search/audit/tree` 与 `all_org_units` / `parent_org_code` / `org_code` 作为共享层逻辑条件。
2. 纠偏文案直接命令模型“请按全部组织重新规划 orgunit.list”，说明共享层现在并不只是提供通用历史纠偏事实，而是在做 `orgunit` 业务纠偏 owner。
3. `queryPromptMentionsUnsupportedOrgUnitDimension(...)` 用硬编码术语决定 boundary violation 是否降级为 `NO_QUERY`。

风险：

1. 第二模块若也存在“当前轮纠正历史范围”的需求，会被迫复用 `orgunit` 式纠偏，而不是走模块知识包自己的范围语义。
2. 共享层会继续膨胀 capability-specific 特判。

owner 冻结：

- 共享层可持有“当前轮显式纠正历史范围时，不得盲继承历史 observation”的通用规则。
- 但“如何识别某个模块的全量范围、哪些 executor_key 属于缩窄还是展开、哪些参数表示过滤条件”应由模块知识包或模块级规则声明，不应写死在共享 query flow。
- 按 `DEV-PLAN-473`，共享层不得把上述模块规则提升为本地 scope engine；当前输入、历史 evidence 和模块知识包应交给模型判断，执行器只校验模型输出的显式 `ReadPlan` 参数。

### 4.3 类别 C：共享候选/结果集抽取仍假设 `orgunit` payload 形状

当前发现：

1. `extractQueryCandidatesFromPayload(...)` 只会从 `org_units` 和 `target_org_code` 提取候选，并直接把 `Domain` 设为 `orgunit`。
2. 这意味着“结果集续接”“候选组”“轻量 evidence window”当前虽然是共享 runtime 结构，但其共享抽取器仍是 `orgunit` 专用实现。

风险：

1. 第二模块若返回的 payload 不是 `org_units` / `target_org_code` 形状，当前共享 evidence-window 构造无法自然复用。
2. 若继续在共享层为每个模块追加 `extractXxxCandidatesFromPayload(...)` 分支，会复制出 capability-specific runtime。

owner 冻结：

- 共享层需要的是“候选抽取 contract”，不是 `orgunit` payload 名称表。
- 后续要么把候选/结果集摘要下沉到模块 executor 明确返回，要么引入最小的共享观察接口，但不得继续在共享 query flow 累积按 payload 字段名猜模块的逻辑。
- 候选观察只能作为 `query_evidence_window` 的事实输入；不得在本地把候选组解析成 selected target、winner、current entity 或隐式执行参数。

### 4.4 类别 D：知识包装配路径仍以单模块为默认

当前发现：

1. `newCubeBoxQueryFlow(...)` 当前只从 `modules/orgunit/presentation/cubebox` 装 knowledge pack。
2. `NoQueryGuidanceFromKnowledgePacks(...)` 只返回第一份命中的 guidance，未定义多模块共存时的策略。
3. 多处 server 测试 fixture 也默认只有 `modules/orgunit/presentation/cubebox` 一个 knowledge pack。
4. `newCubeboxQueryFlow(...)` 当前逐份调用 `ValidateKnowledgePackAgainstRegistry(pack, registry)`；该函数会反向要求注册表中的 executor_key 都出现在当前 pack 的 `apis.md` 中。该口径适合单模块样板，但在多模块共存时会把“单份 pack 覆盖整个 registry”误当成共享 contract。

风险：

1. 第二模块接入前若不先冻结装配策略，后续实现很容易临时拼接多个模块 prompt，导致 guidance 冲突、测试散乱、模块优先级不明。
2. 共享层可能为了“先跑起来”再次引入 capability-specific prompt 排序补丁。
3. 第二模块接入时可能被迫把其他模块 executor_key 写进自己的 knowledge pack，或反过来弱化 registry drift 校验，二者都会破坏 `468/466` 已冻结的早发现边界。

owner 冻结：

- 共享层需要先明确“多知识包如何装入 planner prompt / no-query guidance / drift 校验”的最小 contract。
- 该 contract 必须能支持多个模块并存，但不要求在本计划中实现动态模块发现。
- 多模块 drift 校验应按“全部已装 knowledge packs 的 `apis.md` 并集”与 `ExecutionRegistry` 做一致性校验，或显式按模块分组注册/校验；不得继续要求单份 pack 覆盖全局 registry。

### 4.5 类别 E：共享错误码/终端文案仍直接暴露模块专有语义

当前发现：

1. `queryExecutionErrorToTerminal(...)` 仍硬编码 `orgunit_not_found` 与对应中文提示。
2. 澄清事实提取当前仍绑定 `orgUnitSearchAmbiguousError`。

风险：

1. 第二模块接入后，如果继续沿用这种模式，共享层会变成模块错误分发器。
2. 用户可见 terminal 文案会继续在共享层堆积模块专属自然语言。

owner 冻结：

- 共享层应只持有稳定的通用错误类别与 fail-closed terminal contract。
- 模块专有“not found / ambiguous / unsupported dimension”语义应尽量降为结构化事实或模块侧错误分类，而不是继续膨胀共享层自然语言分支。

### 4.6 类别 F：共享 query context / evidence window 数据结构已有 `orgunit` 字段

当前发现：

1. `modules/cubebox/query_entity.go` 的 `QueryEntity` 当前包含通用字段 `Domain` / `Intent` / `EntityKey` / `AsOf` / `SourceExecutorKey`，但也包含 `TargetOrgCode` / `ParentOrgCode`。
2. `QueryCandidate` 当前字段较薄，但测试与 payload 生产路径默认把 `Domain=orgunit`、`entity_key=org_code`、`name/status/as_of` 作为唯一候选形状。
3. metadata event（例如 `turn.query_entity.confirmed`、`turn.query_candidates.presented`、`turn.query_clarification.requested`）当前是共享事件类型；一旦继续把模块专属字段直接塞进共享 payload，第二模块会被迫复用 `orgunit` 字段名或新增并列字段。

风险：

1. 即使 prompt 与 query flow 分支去掉 `orgunit` 字样，共享 evidence window 仍可能用字段结构把第二模块拖回组织模型。
2. 后续若直接新增 `target_<module>_code`、`parent_<module>_code` 之类字段，共享 context 会退化成 capability-specific DTO 汇总层。

owner 冻结：

- 共享层只应持有 `domain`、`intent`、`entity_key`、`as_of`、`source_executor_key`、`display name/status` 等跨模块稳定事实。
- 模块专属事实应进入模块 executor 显式返回的观察 payload，或进入受控 `attributes` / `facts` 结构；不得继续在共享 struct 上平铺模块字段。
- 现有 `TargetOrgCode` / `ParentOrgCode` 必须在后续实现中被裁决为“过渡债务并迁出”或“归入模块事实容器”，不能默认为长期共享 contract。

### 4.7 类别 G：共享测试缺少非 `orgunit` 反例 fixture

当前发现：

1. `internal/server/cubebox_query_flow_test.go`、`modules/cubebox/knowledge_pack_test.go`、`modules/cubebox/read_executor_test.go` 等共享测试大量使用 `orgunit.details/list/search/audit` 作为唯一样例。
2. 当前测试能保护 `orgunit` 主链，但不能证明共享 narrator、planner prompt、knowledge pack validation、candidate extraction 与 no-query guidance 在非组织模块下仍成立。

风险：

1. 第二模块接入前看似“测试都绿”，但测试实际只证明了单模块样板没有坏。
2. reviewer 难以区分测试中的 `orgunit` 是合法样例，还是已经被写成共享行为 contract。

owner 冻结：

- 后续实现必须引入一个最小非 `orgunit` fake module fixture，用于共享层单测；该 fixture 不需要接真实业务模块、数据库或 UI。
- fake module 应覆盖最小 `executor_key`、knowledge pack、read result payload、candidate / no-query guidance 场景，证明共享层不依赖 `org_units`、`org_code`、`parent_org_code`、`target_org_code`、`orgunit.list`。

### 4.8 类别 H：去污染整改可能滑向新的本地语义层

当前发现：

1. 类别 B/C/F 的修复都天然存在诱惑：把 `orgunit` 特判抽象成“更通用”的本地 scope/candidate/context framework。
2. 但 `DEV-PLAN-473` 已明确：本地不应判断“用户指的是谁、当前范围是什么、候选集合怎么解释、短输入是否延续上一轮”等自然语言语义。
3. 第二模块接入前，如果用“模块配置 + 本地 runtime 解释”替换 `orgunit` 硬编码，表面上去掉了 `orgunit` 字样，实质上会新增一套模型之外的语义 owner。

风险：

1. 共享层从 `orgunit` 污染变成“通用但仍由 Go 代码判定语义”的平台污染。
2. 后续每个模块都会被要求声明 scope kind、candidate selector、fallback question、field role 等本地可解释配置，知识包与模型被再次降级为运行时配置素材。
3. 出现模型不稳时，团队会继续向本地配置和 Go 分支堆规则，而不是改善 evidence、prompt、工具目录和 loop。

owner 冻结：

- 478 的收敛方向是“去掉共享层模块语义裁决”，不是“把模块语义裁决参数化”。
- 允许新增的共享 contract 只能服务于硬边界和事实传递，例如 executor registration、knowledge pack 装配、read_api_catalog、query_evidence_window、结构化 observation、schema 校验、预算和错误分类。
- 不允许新增的共享 contract 包括本地 NLU、scope classification、candidate winner selection、slot repair、静态澄清策略、回答模板与模块语义 fallback。

## 5. 建议实施顺序

### Phase A / P1：先做共享层污染盘点与边界冻结

1. [ ] 为 narrator / clarifier / no-query guidance / planner correction 各自列出“允许保留的共享规则”和“必须下沉的模块语义”。
2. [ ] 为 `extractQueryCandidatesFromPayload(...)`、scope correction、unsupported-dimension 判定补充 owner 裁决。
3. [ ] 冻结“哪些测试命中只是 `orgunit` 样例，哪些测试其实把 `orgunit` 写成共享 contract”。
4. [ ] 为 `QueryEntity`、`QueryCandidate`、`QueryCandidateGroup`、`QueryClarification` 与 metadata event payload 列出共享字段白名单；将 `TargetOrgCode` / `ParentOrgCode` 标记为待裁决迁移项。
5. [ ] 对每个拟新增共享 contract 标注其类型：`hard-boundary`、`fact-transport`、`model-input` 或 `local-semantic-decision`；命中 `local-semantic-decision` 的设计默认退回。

### Phase B / P1：冻结多知识包装配 contract

1. [ ] 明确第二模块接入后 `knowledgePackDirs`、executor 注册项、`read_api_catalog` 与 knowledge pack 校验必须作为同一组装配单元更新。
2. [ ] 明确多 knowledge pack 校验策略：按 pack 并集校验 registry，或按模块分组校验；不得继续要求单份 pack 覆盖全局 registry。
3. [ ] 明确 `NoQueryGuidanceFromKnowledgePacks(...)` 的选择/聚合策略，包括多模块 scope summary 的排序、去重、冲突处理和默认展示数量。
4. [ ] 明确共享层是否允许依赖 knowledge pack 声明模块 scope summary，还是必须有更高层受控组合策略。

### Phase C / P2：逐项整改共享层 capability-specific 逻辑

1. [ ] narrator / clarifier prompt 去模块词。
2. [ ] scope correction 与 unsupported-dimension 从 `orgunit` 硬编码收敛为模型可消费的中性 evidence / knowledge pack 规则；不得收敛为本地 scope engine。
3. [ ] 候选/结果集抽取从 `orgunit` payload 猜测收敛为模块显式事实提供。
4. [ ] `queryExecutionErrorToTerminal(...)` 和澄清事实提取从模块专有错误分支收敛为共享错误 contract。
5. [ ] `QueryEntity` / metadata event 中模块专属字段迁出或收敛到受控事实容器，避免继续平铺 `target_<module>_code` 这类字段。
6. [ ] 对候选组、结果集和 clarification resume 的后续使用补充测试：本地只投影 evidence，不能隐式补 `ReadPlan` 参数或生成 selected target。

### Phase D / P2：非 `orgunit` fixture 与测试准入

1. [ ] 新增最小 fake module knowledge pack，不接真实业务模块与数据库，只用于共享层单测。
2. [ ] 新增 fake module executor 或 stub registry，覆盖 `executor_key` 注册、planner prompt block、no-query guidance、candidate observation 与 terminal fallback。
3. [ ] 将共享测试从“断言 `orgunit` 默认文案”改为“断言共享 contract + 当前模块样例”；`orgunit` 场景测试必须明确归类为模块样例或模块 executor 测试。

### Phase E / P3：文档与门禁回写

1. [ ] 在 `468` 与后续第二模块接入计划中回写 owner 关系与验证口径。
2. [ ] 评估是否新增轻量反回流脚本或现有门禁扩展，用于扫描共享 query flow 中新增模块 executor_key、payload 字段猜测与模块专属错误文案。

## 6. Stopline

在本计划关闭前，新增第二业务模块相关变更默认不得：

1. [ ] 在共享 narrator / clarifier prompt 中新增第二个模块的字段名、示例回答或业务对象措辞。
2. [ ] 在共享 query flow 中新增新的 `<module>.details/list/search/...` executor_key 关键词补丁。
3. [ ] 在共享层继续新增按 payload 字段名猜模块的候选提取逻辑。
4. [ ] 通过“先把第二模块 knowledge pack 排在 `orgunit` 前/后面”这种顺序补丁规避多模块 guidance 冲突。
5. [ ] 在共享 terminal error 映射里继续堆模块专属中文错误文案。
6. [ ] 在共享 `QueryEntity` / `QueryCandidate` / `QueryClarification` / metadata event payload 中新增模块专属平铺字段。
7. [ ] 为了让多模块校验通过而降低 `ValidateKnowledgePackAgainstRegistry(...)` 的 fail-closed 能力；必须改成成组校验或模块分组校验。
8. [ ] 只通过 `orgunit` 测试夹具证明共享 runtime 多模块可用；必须有至少一个非 `orgunit` fake module 覆盖关键共享链路。
9. [ ] 以“去 `orgunit` 硬编码”为名新增本地 NLU、scope classifier、candidate selector、slot repair、target binding、working set 或静态澄清策略。
10. [ ] 从 `query_evidence_window`、metadata event、assistant prose 或历史工具结果中自动补 executor 参数；所有可执行 target 必须来自模型输出的显式 `ReadPlan` 并通过校验。

## 7. 最小非 `orgunit` fixture 准入要求

后续实现不需要为了本计划接入真实第二业务模块，但必须用一个最小 fake module 证明共享层 contract 不依赖 `orgunit`。建议 fixture 约束如下：

1. fake module 使用非组织命名，例如 `asset` / `project` / `sample`，不得包含 `org`、`orgunit`、`org_code`、`parent_org_code`、`target_org_code`、`org_units`。
2. 至少包含一份内存 knowledge pack，声明一个 `executor_key`，例如 `sample.details` 或 `asset.search`。
3. 至少包含一个 executor stub，返回非 `orgunit` payload，并能提供候选或实体事实。
4. 测试必须覆盖：
   - planner prompt 能同时装入多份 knowledge pack；
   - registry 与 knowledge packs 能成组通过 drift 校验；
   - no-query guidance 不只取第一份模块 guidance 或能按冻结策略稳定聚合；
   - candidate / result_list observation 不依赖 `org_units` / `target_org_code`；
   - fake module 的候选与结果集只进入 `query_evidence_window`，不会被本地转换成 selected target 或隐式 executor 参数；
   - terminal fallback 不需要新增模块专属中文错误分支。

## 8. 验收标准

1. [ ] 能把当前命中清楚分成“共享层污染”“模块样例夹具”“模块本来该有的业务语义”三类。
2. [ ] 为第二模块接入前的共享 narrator / query flow / knowledge pack 装配给出明确 owner 和实施顺序，而不是只写一句“后续复核”。
3. [ ] 共享 `QueryContext` / `QueryEntity` / metadata event 的模块专属字段已有裁决：保留为通用字段、迁入受控事实容器、或登记为待删除过渡债务。
4. [ ] 多 knowledge pack + `ExecutionRegistry` 的校验策略已冻结，不再以单份 `orgunit` pack 覆盖全局 registry 作为隐含前提。
5. [ ] 至少一个非 `orgunit` fake module 测试能覆盖 planner prompt 装配、no-query guidance、candidate observation 与 terminal fallback。
6. [ ] 478 与 `DEV-PLAN-473` 的边界已明确：去 `orgunit` 污染不会新增本地语义裁决层；所有目标选择、候选解释、澄清恢复和短输入理解仍由模型基于 evidence 与 knowledge pack 处理。
7. [ ] Stopline 已能被 review checklist 或轻量门禁执行，阻断共享 query flow 新增模块 executor_key 特判、payload 字段猜测、模块专属错误文案、共享 context 模块字段平铺和本地语义 selector。
8. [ ] `468` 当前剩余主线与 `AGENTS.md` 文档地图都能直接指向本 owner 文档。
9. [ ] 文档门禁 `make check doc` 通过。

## 9. 当前执行记录

1. [X] 2026-04-28：完成共享层首轮扫描，确认主要命中集中在 `internal/server/cubebox_query_flow.go` 的 narrator/clarifier prompt、scope correction、unsupported-dimension 判定、候选抽取与单模块 knowledge pack 装配入口。
2. [X] 2026-04-28：确认 `468` line 635 所指“第二业务模块接入后的共享 narrator 去模块化污染复核”尚无独立 owner 文档，本计划即作为该剩余主线的正式承接者。
3. [X] 2026-04-28：补充复核缺口：共享 `QueryEntity` / metadata event 字段污染、多 knowledge pack 与 registry 成组校验、非 `orgunit` fake module 测试准入、反回流门禁化要求。
4. [X] 2026-04-28：按 `DEV-PLAN-473` 补充模型主导边界：478 的目标是删除共享层模块语义裁决，而不是把 `orgunit` 特判参数化为新的本地语义 runtime。
