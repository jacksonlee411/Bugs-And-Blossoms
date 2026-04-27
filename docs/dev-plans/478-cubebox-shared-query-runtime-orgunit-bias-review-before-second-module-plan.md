# DEV-PLAN-478：CubeBox 第二业务模块接入前共享 Query Runtime 去 `orgunit` 污染复核方案

**状态**: 规划中（2026-04-28 06:52 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：在第二个业务模块接入 `CubeBox` 查询链之前，系统性复核共享 narrator、共享 query flow、共享 knowledge pack 装配与 `no_query` guidance 是否仍被 `orgunit` 场景单边塑形，并冻结调查分类、owner 边界、实施顺序与 stopline。
- **关联模块/目录**：`docs/dev-plans/468-cubebox-session-continuity-and-model-autonomy-improvement-plan.md`、`docs/dev-plans/460-cubebox-digital-assistant-positioning-and-execution-contract.md`、`docs/dev-plans/461-cubebox-query-scenarios-minimal-contract.md`、`docs/dev-plans/464-cubebox-query-architecture-convergence-plan.md`、`internal/server/cubebox_query_flow.go`、`internal/server/cubebox_query_flow_test.go`、`modules/cubebox/*`、`modules/orgunit/presentation/cubebox/*`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-003`、`DEV-PLAN-460`、`DEV-PLAN-461`、`DEV-PLAN-464`、`DEV-PLAN-468`
- **用户入口/触点**：主应用壳层右侧 `CubeBox` 抽屉、`/internal/cubebox/turns:stream`、共享 planner/narrator/clarifier/no-query guidance 链路

### 0.1 Simple > Easy 三问

1. **边界**：本计划复核的是“共享层是否残留 `orgunit` 偏置”，不是现在就接第二模块，也不是顺手重写 `orgunit` 自身知识包。
2. **不变量**：模块私有语义必须留在模块知识包或模块 executor 侧；共享层只持有通用查询 contract、fail-closed 边界、结构化事实窗口和受控用户回答约束。
3. **可解释**：reviewer 必须能在 5 分钟内分清三类命中：哪些属于共享层污染，哪些属于模块实现本来就该存在，哪些只是测试夹具用 `orgunit` 作为当前唯一样例。

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

因此，本计划的目的不是提前改代码，而是先把“什么算污染、谁来收、按什么顺序收”冻结成独立 owner 文档。

## 2. 核心目标

1. [ ] 冻结共享 narrator / clarifier / no-query guidance / planner correction 中的 `orgunit` 污染命中清单。
2. [ ] 明确共享 query runtime 与模块知识包/模块 executor 的 owner 边界，避免第二模块接入时继续把模块语义回灌到共享层。
3. [ ] 冻结多知识包装配的最小策略：共享层如何装配多个知识包、如何汇总或选择 no-query guidance、哪些规则继续由模块知识包自带。
4. [ ] 产出分批整改切片与 stopline，让后续实现 PR 能逐步落地而不引入“第二模块先兼容、共享层后收”的回流窗口。

## 3. 非目标

1. 不在本计划内直接接入第二业务模块。
2. 不在本计划内重写 `orgunit` 知识包业务语义或 `orgunit` executor 返回面。
3. 不在本计划内引入新的通用平台名词，例如 capability matrix engine、tool ontology、answer template system。
4. 不在本计划内实施 per-api 授权；该条仍由 `DEV-PLAN-468 P2-2` 持有。

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

### 4.4 类别 D：知识包装配路径仍以单模块为默认

当前发现：

1. `newCubeBoxQueryFlow(...)` 当前只从 `modules/orgunit/presentation/cubebox` 装 knowledge pack。
2. `NoQueryGuidanceFromKnowledgePacks(...)` 只返回第一份命中的 guidance，未定义多模块共存时的策略。
3. 多处 server 测试 fixture 也默认只有 `modules/orgunit/presentation/cubebox` 一个 knowledge pack。

风险：

1. 第二模块接入前若不先冻结装配策略，后续实现很容易临时拼接多个模块 prompt，导致 guidance 冲突、测试散乱、模块优先级不明。
2. 共享层可能为了“先跑起来”再次引入 capability-specific prompt 排序补丁。

owner 冻结：

- 共享层需要先明确“多知识包如何装入 planner prompt / no-query guidance / drift 校验”的最小 contract。
- 该 contract 必须能支持多个模块并存，但不要求在本计划中实现动态模块发现。

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

## 5. 建议实施顺序

### Phase A / P1：先做共享层污染盘点与边界冻结

1. [ ] 为 narrator / clarifier / no-query guidance / planner correction 各自列出“允许保留的共享规则”和“必须下沉的模块语义”。
2. [ ] 为 `extractQueryCandidatesFromPayload(...)`、scope correction、unsupported-dimension 判定补充 owner 裁决。
3. [ ] 冻结“哪些测试命中只是 `orgunit` 样例，哪些测试其实把 `orgunit` 写成共享 contract”。

### Phase B / P1：冻结多知识包装配 contract

1. [ ] 明确第二模块接入后 `knowledgePackDirs` 的装配入口。
2. [ ] 明确 `NoQueryGuidanceFromKnowledgePacks(...)` 的选择/聚合策略。
3. [ ] 明确共享层是否允许依赖 knowledge pack 声明模块 scope summary，还是必须有更高层受控组合策略。

### Phase C / P2：逐项整改共享层 capability-specific 逻辑

1. [ ] narrator / clarifier prompt 去模块词。
2. [ ] scope correction 与 unsupported-dimension 从 `orgunit` 硬编码收敛为可扩展 contract。
3. [ ] 候选/结果集抽取从 `orgunit` payload 猜测收敛为模块显式事实提供。
4. [ ] `queryExecutionErrorToTerminal(...)` 和澄清事实提取从模块专有错误分支收敛为共享错误 contract。

### Phase D / P3：测试与文档回写

1. [ ] 将共享测试从“断言 `orgunit` 默认文案”改为“断言共享 contract + 当前模块样例”。
2. [ ] 在 `468` 与后续第二模块接入计划中回写 owner 关系与验证口径。

## 6. Stopline

在本计划关闭前，新增第二业务模块相关变更默认不得：

1. [ ] 在共享 narrator / clarifier prompt 中新增第二个模块的字段名、示例回答或业务对象措辞。
2. [ ] 在共享 query flow 中新增新的 `<module>.details/list/search/...` executor_key 关键词补丁。
3. [ ] 在共享层继续新增按 payload 字段名猜模块的候选提取逻辑。
4. [ ] 通过“先把第二模块 knowledge pack 排在 `orgunit` 前/后面”这种顺序补丁规避多模块 guidance 冲突。
5. [ ] 在共享 terminal error 映射里继续堆模块专属中文错误文案。

## 7. 验收标准

1. [ ] 能把当前命中清楚分成“共享层污染”“模块样例夹具”“模块本来该有的业务语义”三类。
2. [ ] 为第二模块接入前的共享 narrator / query flow / knowledge pack 装配给出明确 owner 和实施顺序，而不是只写一句“后续复核”。
3. [ ] `468` 当前剩余主线与 `AGENTS.md` 文档地图都能直接指向本 owner 文档。
4. [ ] 文档门禁 `make check doc` 通过。

## 8. 当前执行记录

1. [X] 2026-04-28：完成共享层首轮扫描，确认主要命中集中在 `internal/server/cubebox_query_flow.go` 的 narrator/clarifier prompt、scope correction、unsupported-dimension 判定、候选抽取与单模块 knowledge pack 装配入口。
2. [X] 2026-04-28：确认 `468` line 635 所指“第二业务模块接入后的共享 narrator 去模块化污染复核”尚无独立 owner 文档，本计划即作为该剩余主线的正式承接者。
