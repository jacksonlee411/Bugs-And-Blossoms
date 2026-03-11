# DEV-PLAN-244：Assistant Interpretation Pack 与 Intent Route Catalog 编译治理计划

**状态**: 规划中（2026-03-11 CST；本次修订将 `244` 细化为“可按文件直接实施”的理解层知识治理蓝图，承接 `240E` 主计划与 `246` 阶段 D，且保持“不阻塞 `242/243` 最小 runtime 落地”的路线约束）

## 1. 背景与定位
1. [ ] `240E` 已冻结：`Interpretation Pack` 与 `Intent Route Catalog` 是 Assistant 理解层主源，职责仅限**意图分流、澄清模板引用、负样例约束、表达解释**，不得越权定义执行真相。
2. [ ] `246` 已冻结阶段顺序：`244` 位于 `243` 之后的阶段 D，其职责不是重写 runtime route，而是把理解层资产从“能跑”提升到“可编译、可审计、可冻结、可迁移”。
3. [ ] 当前仓内其实已经有最小实现：
   - [ ] `internal/server/assistant_knowledge_runtime.go` 已定义 `assistantInterpretationPack`、`assistantIntentRouteCatalog`、`assistantIntentRouteEntry` 以及 `assistantCompileKnowledgeRuntime(...)`；
   - [ ] `internal/server/assistant_knowledge/interpretation/*.json` 与 `internal/server/assistant_knowledge/intent_route_catalog.json` 已存在首批样例；
   - [ ] `internal/server/assistant_intent_router.go` / `internal/server/assistant_api.go` / `internal/server/assistant_intent_pipeline.go` 已开始消费 `intent_id / route_kind / route_catalog_version / knowledge_snapshot_digest`。
4. [ ] 但这些地基仍不足以支撑“理解层完成治理”的结论，原因不在于没有资产，而在于还缺少**资产治理闭环**：
   - [ ] 资产字段虽已存在，但还未冻结到可稳定扩面的契约级别；
   - [ ] 编译器仍以内嵌 runtime 校验为主，缺少面向治理的错误分类、迁移边界与审计证据；
   - [ ] 仓内仍残留散点 prompt、本地规则与 helper 文案，尚未形成“理解资产唯一入口”；
   - [ ] 尚未建立“旧常量/旧 prompt → 资产工件”的清理清单与 stopline。
5. [ ] 本计划定位冻结为：**在不改写 `242/243` 主体职责、不发明第二套路由真相的前提下，把理解层资产治理补齐到可直接扩面实施的程度**。

## 2. 当前实现基线、缺口与根因

### 2.1 当前已具备的地基
1. [ ] `internal/server/assistant_knowledge_runtime.go` 已具备以下最小能力：
   - [ ] 载入 `intent_route_catalog.json` 与 `interpretation/*.json`；
   - [ ] 校验 `asset_type / locale / route_kind / source_refs / forbidden_keys`；
   - [ ] 对 `intent_id` 唯一性、`business_action -> action_id` 注册有效性做 fail-closed 校验；
   - [ ] 生成 `knowledge_snapshot_digest` 与 `route_catalog_version`；
   - [ ] 在 `buildPlanContextV1(...)` 中为非动作输入选择 interpretation pack。
2. [ ] 这说明 `244` 不是从零开始，而是要在现有地基上做**治理扩面与清退收口**。

### 2.2 当前仍存在的缺口
1. [ ] `Interpretation Pack` 仍偏“展示包”，尚未冻结以下治理语义：
   - [ ] `clarification_prompts[]` 与 `template_id` 的唯一性、用途边界与被引用关系；
   - [ ] `negative_examples[]` 的归属语义（只约束理解，不允许变相编码执行结果）；
   - [ ] `intent_classes[]` 是否必须与 route catalog 内实际可达 `intent_id` 集合对齐。
2. [ ] `Intent Route Catalog` 虽已最小可用，但还缺少更强的治理校验：
   - [ ] `clarification_template_id` 与 interpretation pack 的交叉引用校验还不够显式；
   - [ ] `required_slots[]` 目前缺少“仅排序/澄清引用，不得重定义执行必填真相”的代码级断言；
   - [ ] `min_confidence` 缺少与 `242` route 决策 reason code / confidence band 的对齐约束。
3. [ ] 理解层仍存在散点实现，说明主源还未完全收拢：
   - [ ] `internal/server/assistant_knowledge_runtime.go` 内仍保留非动作 fallback 文案；
   - [ ] `internal/server/assistant_api.go` 仍保留 `assistant_intent_unsupported` 等旧出口；
   - [ ] `internal/server/assistant_reply_nlg.go` 仍有泛化失败文案，尚未完全让位于知识主链；
   - [ ] `internal/server/assistant_intent_pipeline.go` 与本地升级逻辑仍可能携带“未资产化”的理解信号。
4. [ ] 目前缺少**迁移清单与证据模板**：没有正式记录“哪些 prompt/规则已迁入资产、哪些仍待清退、为什么还不能删”。

### 2.3 根因到文件的映射
1. [ ] `internal/server/assistant_knowledge_runtime.go`
   - [ ] 当前同时承担类型定义、载入、编译、校验、runtime fallback，多职责耦合；
   - [ ] 更像“运行时 helper”，还不是治理语义清晰的编译入口。
2. [ ] `internal/server/assistant_knowledge/*.json`
   - [ ] 现有样例可证明路径通，但覆盖面仍不足以支持治理 stopline；
   - [ ] 缺少“需澄清”“非动作问答”“负样例”等对照样本矩阵。
3. [ ] `internal/server/assistant_intent_router.go`
   - [ ] 已开始使用 route catalog，但尚未把 catalog 的治理约束显式外显为编译期依赖契约。
4. [ ] `internal/server/assistant_api.go` / `internal/server/assistant_reply_nlg.go`
   - [ ] 仍保留一部分理解层与表达层兜底文案；
   - [ ] 若不在 `244` 中建立“清退散点 prompt”的正式清单，后续很容易继续回流。

## 3. 目标与非目标

### 3.1 核心目标
1. [ ] 把 `Interpretation Pack` 与 `Intent Route Catalog` 从“已有 JSON 文件”提升为**治理级资产**：字段语义冻结、引用关系明确、fail-closed 校验完整。
2. [ ] 把理解层编译逻辑收敛为单一入口，确保 `242/243/245` 只能消费**已编译工件**，而不是重新读取散乱 JSON 或 helper 常量。
3. [ ] 建立“散点 prompt / 规则 / helper → 资产工件”的迁移清单与 stopline，阻断理解层继续回退到本地字符串与临时规则。
4. [ ] 为 `242/243` 提供更稳定的 route 资产契约：`intent_id / route_kind / required_slots / min_confidence / clarification_template_id / source_refs / route_catalog_version`。
5. [ ] 为 `245` 提供清晰边界：`244` 只负责理解层资产与编译治理，不直接承担用户可见 reply 主链统一。

### 3.2 非目标
1. [ ] 不在本计划中重写 `242` 的 `assistantIntentRouteDecision` runtime 主链；`244` 只提供更干净的输入工件与治理约束。
2. [ ] 不在本计划中重写 `243` 的 `Clarification` 状态机、轮次控制与恢复主链。
3. [ ] 不在本计划中把 `Action View Pack`、`Reply Guidance Pack` 的表达统一一起做完；这两部分分别由 `241/245` 承接。
4. [ ] 不在本计划中引入外部知识平台、向量库、RAG、文档上传或新的运行时协议。
5. [ ] 不允许本计划借由 `required_slots[]`、`negative_examples[]` 或模板文本去定义 `phase / required_fields / confirm 条件 / commit 条件` 等执行真相。

## 4. 与 240E / 241 / 242 / 243 / 245 / 246 的边界冻结
1. [ ] 与 `240E` 的关系：`244` 只做主计划的理解层实施细化；若需要新增资产类型、改主源矩阵或放宽边界，必须先回写 `240E`。
2. [ ] 与 `241` 的关系：`241` 负责最小 schema/快照/Resolver 接线；`244` 负责把其中与理解层相关的资产编译治理补齐，但不得破坏 `241` 已冻结的 `knowledge_snapshot_digest / route_catalog_version / plan_context_v1` 口径。
3. [ ] 与 `242` 的关系：`242` 消费 route catalog 做正式 route decision；`244` 不得反过来定义 route runtime 状态机，只能提供更强的 catalog 契约和清退散点规则。
4. [ ] 与 `243` 的关系：`243` 的澄清裁决与恢复仍是唯一主链；`244` 仅负责保证 `clarification_template_id`、`required_slots[]`、`negative_examples[]` 的来源稳定，不得直接替代 `Clarification` builder。
5. [ ] 与 `245` 的关系：`244` 允许定义理解层模板引用，但不负责最终用户可见回复措辞统一；若发现需求已进入 reply 表达，应转交 `245`。
6. [ ] 与 `246` 的关系：`244` 必须遵守“**不阻塞 `242/243` 最小 runtime**”原则；若某项治理扩面无法证明会直接降低散点依赖，应后置，不得拖慢前序阶段封板。

## 5. 资产契约（冻结）

### 5.1 `Interpretation Pack` 契约
1. [ ] 字段冻结为：
   - [ ] `asset_type = interpretation_pack`
   - [ ] `pack_id`
   - [ ] `knowledge_version`
   - [ ] `locale`
   - [ ] `intent_classes[]`
   - [ ] `clarification_prompts[]`
   - [ ] `negative_examples[]`
   - [ ] `source_refs[]`
2. [ ] `pack_id` 语义冻结：
   - [ ] 对于非动作理解包，建议与 `intent_id` 对齐，例如 `knowledge.general_qa`；
   - [ ] 对于共享解释包，允许多 `intent_id` 复用，但必须在执行记录中说明复用原因；
   - [ ] 同一 `pack_id` 下允许多 locale，不允许同 locale 重复。
3. [ ] `intent_classes[]` 语义冻结：
   - [ ] 只允许声明 `business_action / knowledge_qa / chitchat / uncertain` 中的一项或多项；
   - [ ] 其作用是约束该 pack 被哪些意图类别引用，不得用于表达执行阶段或结果。
4. [ ] `clarification_prompts[]` 语义冻结：
   - [ ] 元素结构仅允许 `template_id / text`；
   - [ ] `template_id` 在单个 pack 内必须唯一；
   - [ ] `text` 只能表达理解/澄清引导，不得嵌入执行结果承诺、提交条件或成功回执文案。
5. [ ] `negative_examples[]` 语义冻结：
   - [ ] 仅用于约束理解层不要误判意图；
   - [ ] 不得编码租户数据、业务对象主键、临时环境信息或执行输出；
   - [ ] 不得以“缺少某字段时直接失败”方式替代 `243` 的澄清主链。
6. [ ] `source_refs[]` 语义冻结：
   - [ ] 必须全部指向仓内有效路径；
   - [ ] 至少一个引用应直接指向该意图/规则的契约来源；
   - [ ] 不允许把 `source_refs[]` 当作自由备注区或跨仓链接集合。

### 5.2 `Intent Route Catalog` 契约
1. [ ] 字段冻结为：
   - [ ] `asset_type = intent_route_catalog`
   - [ ] `route_catalog_version`
   - [ ] `source_refs[]`
   - [ ] `entries[]`
2. [ ] `entries[]` 每项字段冻结为：
   - [ ] `intent_id`
   - [ ] `route_kind`
   - [ ] `action_id`
   - [ ] `required_slots[]`
   - [ ] `min_confidence`
   - [ ] `clarification_template_id`
   - [ ] `keywords[]`
3. [ ] `intent_id` 语义冻结：
   - [ ] 全 catalog 内唯一；
   - [ ] 与 interpretation pack 的 `pack_id` 允许一对一或多对一，但必须能从编译结果中明确解析对应关系；
   - [ ] 不得同时把同一 `intent_id` 映射到多个 `route_kind`。
4. [ ] `route_kind` 只允许：`business_action / knowledge_qa / chitchat / uncertain`。
5. [ ] `action_id` 语义冻结：
   - [ ] 仅 `business_action` 允许非空；
   - [ ] 若非空，必须已在 `assistant_action_registry` 注册；
   - [ ] 不允许通过 catalog 发明隐式 action alias。
6. [ ] `required_slots[]` 语义冻结：
   - [ ] 仅用于 route/clarification 的**排序提示与模板引用**；
   - [ ] 不得作为 `required_fields` 真相来源；
   - [ ] 允许为空，但若非空，项值必须可映射到执行主链已知字段名。
7. [ ] `min_confidence` 语义冻结：
   - [ ] 只允许 `[0,1]` 区间；
   - [ ] 含义是“route catalog 推荐阈值”，不是 provider 原始分数真相；
   - [ ] 其使用结果必须由 `242` 转译成标准 `confidence_band / reason_codes`，不得在 catalog 内直接声明 band。
8. [ ] `clarification_template_id` 语义冻结：
   - [ ] 若非空，必须能在目标 interpretation pack 中找到；
   - [ ] 不允许引用 reply guidance 模板或 action view 模板；
   - [ ] 不允许跨 pack 使用未声明用途的模板。
9. [ ] `keywords[]` 语义冻结：
   - [ ] 仅用于当前最小 runtime 的文本匹配辅助；
   - [ ] 不得把 `keywords[]` 膨胀为复杂 DSL；
   - [ ] 若需要更复杂识别信号，应回写后续计划，不在 `244` 内临时扩语言。

### 5.3 明确禁止的字段与语义
1. [ ] 资产文件中继续禁止出现：`required_fields / phase / confirm_conditions / commit_conditions`。
2. [ ] 禁止出现“提交后将成功创建”“确认后自动执行”等结果承诺性表达。
3. [ ] 禁止通过模板文本暗含租户、组织、人员主数据的运行时真相。
4. [ ] 禁止让 catalog/pack 直接回写 `assistantTurn` 或 `assistantPlanSummary` 的执行态字段。

## 6. 编译器与治理流水线（冻结）

### 6.1 编译入口
1. [ ] 保持 `assistantLoadKnowledgeRuntime()` 作为 runtime 载入入口，但需要把“治理语义”显式前置到编译步骤，而不是散落在 runtime helper 中。
2. [ ] 推荐在 `internal/server/assistant_knowledge_runtime.go` 内或相邻新文件中收敛以下职责：
   - [ ] `assistantLoadInterpretationPacks()`：文件读取；
   - [ ] `assistantLoadIntentRouteCatalog()`：catalog 读取；
   - [ ] `assistantCompileInterpretationAssets(...)`：pack 级结构与语义校验；
   - [ ] `assistantCompileIntentRouteCatalog(...)`：catalog 级结构与交叉引用校验；
   - [ ] `assistantCompileKnowledgeRuntime(...)`：整合 digest、索引、版本口径与 runtime 视图。
3. [ ] 是否拆文件由实现时决定，但**编译职责边界**必须按上面四层冻结；禁止继续把所有校验混在一个大函数中无差别扩张。

### 6.2 结构校验
1. [ ] 对 interpretation pack 至少校验：
   - [ ] `asset_type / pack_id / knowledge_version / locale / source_refs` 必填；
   - [ ] `locale` 仅允许 `zh / en`；
   - [ ] `clarification_prompts[]` 中的 `template_id` 非空且在 pack 内唯一；
   - [ ] `negative_examples[]` 去重后不得为空字符串；
   - [ ] 文件 JSON 不得携带未知禁止字段。
2. [ ] 对 route catalog 至少校验：
   - [ ] `asset_type / route_catalog_version / source_refs` 必填；
   - [ ] `intent_id` 全局唯一；
   - [ ] `route_kind` 合法；
   - [ ] `business_action` 必须带合法 `action_id`；
   - [ ] `min_confidence` 位于 `[0,1]`；
   - [ ] `required_slots[]` 去重且项值非空。

### 6.3 交叉引用校验
1. [ ] `clarification_template_id` 若非空，必须命中 interpretation pack 内的 `template_id`。
2. [ ] 非动作 `intent_id` 至少要能找到一个 interpretation pack；缺失时 fail-closed。
3. [ ] 若 `route_kind = business_action` 且提供 `required_slots[]`，这些 slot 必须全部属于该 action 已知字段视图；否则阻断。
4. [ ] `source_refs[]` 必须全部可落到仓内路径，且至少一个引用来自：
   - [ ] `docs/dev-plans/240e-*.md`；或
   - [ ] 对应业务契约文档；或
   - [ ] 实现直接依赖的本仓代码文件。
5. [ ] interpretation pack 的 `intent_classes[]` 若与引用它的 route entry `route_kind` 完全不相交，必须阻断。

### 6.4 编译输出
1. [ ] 编译结果至少需要稳定输出：
   - [ ] `SnapshotDigest`
   - [ ] `RouteCatalogVersion`
   - [ ] `ResolverContractVersion`
   - [ ] interpretation 索引
   - [ ] route catalog 索引
   - [ ] 交叉引用后的模板查找结果
2. [ ] `SnapshotDigest` 计算口径必须冻结：
   - [ ] 输入只允许是资产结构化内容；
   - [ ] 不得把文件修改时间、读取顺序、map 非稳定遍历顺序引入 hash；
   - [ ] 同语义、同内容、同排序的资产必须产出同 digest。
3. [ ] `route_catalog_version` 与 `knowledge_snapshot_digest` 必须能被 `241/242/243` 直接审计记录，不允许运行时再临时拼接第二版本口径。

### 6.5 错误分类与 fail-closed
1. [ ] 编译错误至少分为：
   - [ ] 结构错误；
   - [ ] 引用错误；
   - [ ] 越权语义错误；
   - [ ] 版本/快照错误。
2. [ ] 任何非法引用、缺失 pack、越权字段、模板引用失配都必须阻断启动或阻断测试，不允许静默降级到默认 prompt。
3. [ ] 唯一允许的 fallback 是 locale 回退（请求 locale → `zh` → `en`），且前提是该 pack 或模板已通过编译校验进入正式索引。

## 7. 直接实施范围与文件落点

### 7.1 必改文件（首期）
1. [ ] `docs/dev-plans/244-assistant-interpretation-pack-and-intent-route-catalog-compiler-plan.md`
   - [ ] 作为实施契约主文档；
   - [ ] 后续范围变化先回写此文档。
2. [ ] `internal/server/assistant_knowledge_runtime.go`
   - [ ] 收敛 interpretation 与 route catalog 的类型、编译与引用校验；
   - [ ] 删除或最小化理解层硬编码 fallback；
   - [ ] 保持 `241` 的 digest/version 审计口径不变。
3. [ ] `internal/server/assistant_knowledge/intent_route_catalog.json`
   - [ ] 补齐首批 `intent_id / clarification_template_id / required_slots / min_confidence / source_refs` 治理样例。
4. [ ] `internal/server/assistant_knowledge/interpretation/*.json`
   - [ ] 为 `business_action:create_orgunit`、`knowledge_qa`、`uncertain` 或待澄清样例补齐 pack 与模板。
5. [ ] `internal/server/assistant_knowledge_runtime_test.go`
   - [ ] 覆盖结构校验、digest 稳定性、locale fallback、正常索引行为。
6. [ ] `internal/server/assistant_knowledge_runtime_more_test.go`
   - [ ] 覆盖交叉引用错误、越权字段、source refs、template id、slot 校验等 fail-closed 分支。

### 7.2 视实现命中的协同文件
1. [ ] `internal/server/assistant_intent_router.go`
   - [ ] 确认只消费已编译 catalog 结果，不再旁路读取散点配置；
   - [ ] 若需要新增 reason code，对齐 `242`，不得在 `244` 内另起口径。
2. [ ] `internal/server/assistant_intent_pipeline.go`
   - [ ] 若仍存在理解层硬编码信号，迁移到资产或在执行记录中登记暂缓原因。
3. [ ] `internal/server/assistant_api.go`
   - [ ] 只允许保留错误映射与 gate，不允许继续新增理解层 prompt 文案。
4. [ ] `internal/server/assistant_reply_nlg.go`
   - [ ] 若发现理解层失败文案仍硬编码在 reply 路径，登记到 `245` 或补充清退说明；`244` 不直接接管 reply 主链，但要阻断继续新增。

### 7.3 新增文档与记录
1. [ ] `docs/dev-records/dev-plan-244-execution-log.md`
   - [ ] 记录资产迁移清单、散点 prompt 清退情况、未完成项与原因；
   - [ ] 记录每次 `route_catalog_version / knowledge_snapshot_digest` 变化的原因归类。
2. [ ] 若新增治理附表，必须放在 `docs/dev-records/`，不得在仓库根目录新增 `.md`。

## 8. 实施顺序（可直接开工）

### 8.1 PR-244-01：资产盘点与停止线清单
1. [ ] 盘点仓内所有理解层散点来源：
   - [ ] `assistant_knowledge_runtime.go` 中的 fallback 文案；
   - [ ] `assistant_api.go` 中的 unsupported/clarification 旧出口；
   - [ ] `assistant_intent_pipeline.go` 中的本地升级与非资产化信号；
   - [ ] `assistant_reply_nlg.go` 中与理解层耦合的失败文案。
2. [ ] 输出迁移清单：`现有位置 -> 目标资产/保留理由 -> 计划归属（244/245/不动）`。
3. [ ] 验收：
   - [ ] 执行记录中存在完整迁移清单；
   - [ ] 明确哪些项必须在 `244` 清理，哪些项后置到 `245`。

### 8.2 PR-244-02：契约加固与编译器分层
1. [ ] 在编译器中补齐 interpretation pack 的 `template_id` 唯一性、`intent_classes[]` 合法性、`negative_examples[]` 清洗与去重校验。
2. [ ] 在 route catalog 编译中补齐：
   - [ ] `clarification_template_id` 交叉引用；
   - [ ] `required_slots[]` 的“仅排序/引用，不得成为执行真相”断言；
   - [ ] `min_confidence` 区间与空值策略。
3. [ ] 将“读取”“结构校验”“交叉引用”“runtime 索引输出”分层，不再把所有规则揉进一个条件块。
4. [ ] 验收：
   - [ ] 非法模板引用、非法 slot、非法 confidence、非法 intent_classes 均被阻断；
   - [ ] 现有 runtime 测试仍可通过或按新契约更新。

### 8.3 PR-244-03：样例资产扩面与自然语言对照集
1. [ ] 至少补齐三类正式样例：
   - [ ] `business_action:create_orgunit`；
   - [ ] `knowledge_qa`；
   - [ ] `uncertain` 或“需要澄清”的自然表达。
2. [ ] 每类样例都要具备：
   - [ ] `source_refs[]`；
   - [ ] 对应 interpretation pack；
   - [ ] 必要时的 `clarification_template_id`；
   - [ ] 至少一条负样例。
3. [ ] 验收：
   - [ ] 资产可证明“理解层样例”已不再只靠黄金句式和本地 helper 文案；
   - [ ] 自然语言回归中能覆盖非动作、待澄清与动作三类入口。

### 8.4 PR-244-04：清退散点 prompt / 规则依赖
1. [ ] 清理或收口首批明确属于理解层的硬编码 fallback：
   - [ ] 非动作默认说明；
   - [ ] 纯理解层 unsupported 提示；
   - [ ] 与 interpretation pack 重复的澄清引导。
2. [ ] 对暂时不能删除的散点逻辑，必须满足：
   - [ ] 在执行记录中登记；
   - [ ] 明确归属 `245` 或更后续计划；
   - [ ] 不再新增第二写入口。
3. [ ] 验收：
   - [ ] 新增/保留的硬编码项都有书面说明；
   - [ ] 不存在“资产已定义，但 runtime 仍优先读本地字符串”的双主源。

### 8.5 PR-244-05：审计与封板证据
1. [ ] 确认 `knowledge_snapshot_digest / route_catalog_version` 在资产变更时稳定变化，在无语义变化时保持稳定。
2. [ ] 完成执行记录：
   - [ ] 迁移清单结果；
   - [ ] 被阻断的非法样例清单；
   - [ ] 未清退项与后续归属。
3. [ ] 验收：
   - [ ] 能证明理解层已脱离散乱 prompt/规则作为主要事实源；
   - [ ] 能证明 `244` 没有改写 `242/243` 的正式 runtime 主链。

## 9. 测试矩阵（冻结）

### 9.1 编译成功路径
1. [ ] `TestAssistantCompileKnowledgeRuntime_InterpretationTemplateRefsValid`
2. [ ] `TestAssistantCompileKnowledgeRuntime_RouteCatalogCrossRefsValid`
3. [ ] `TestAssistantCompileKnowledgeRuntime_SnapshotDigestStable`
4. [ ] `TestAssistantKnowledgeRuntime_FindInterpretationLocaleFallback`
5. [ ] `TestAssistantKnowledgeRuntime_RouteCatalogVersionPropagates`

### 9.2 编译失败路径
1. [ ] `TestAssistantCompileKnowledgeRuntime_RejectsDuplicateInterpretationTemplateID`
2. [ ] `TestAssistantCompileKnowledgeRuntime_RejectsInvalidIntentClass`
3. [ ] `TestAssistantCompileKnowledgeRuntime_RejectsUnknownClarificationTemplateID`
4. [ ] `TestAssistantCompileKnowledgeRuntime_RejectsInvalidRequiredSlot`
5. [ ] `TestAssistantCompileKnowledgeRuntime_RejectsMinConfidenceOutOfRange`
6. [ ] `TestAssistantCompileKnowledgeRuntime_RejectsMissingInterpretationForNonBusinessIntent`
7. [ ] `TestAssistantCompileKnowledgeRuntime_RejectsForbiddenExecutionTruthKeys`
8. [ ] `TestAssistantCompileKnowledgeRuntime_RejectsInvalidSourceRefs`

### 9.3 自然语言回归（首批必备）
1. [ ] “帮我看看系统支持哪些组织操作” → 命中 `knowledge_qa`，不得进入动作提交链。
2. [ ] “在鲜花组织下面建个部门，名字先不定” → 进入 `business_action`，但需走 `243` 的缺字段/澄清主链，而不是由 pack 直接给执行结论。
3. [ ] “我想调整组织，但还没想好是新建还是移动” → 命中 `uncertain` 或待澄清路径，不得伪装成可确认 plan。
4. [ ] 任一中文/英文 locale 样例切换后，`pack` 选择与 digest 审计口径保持可解释一致。

## 10. 验收标准
1. [ ] `244` 完成后，`Interpretation Pack` 与 `Intent Route Catalog` 都有明确的结构、语义、交叉引用与 fail-closed 校验。
2. [ ] 仓内存在正式迁移清单，能回答“哪些理解层 prompt/规则已资产化，哪些仍待后续计划处理”。
3. [ ] `242/243` 只消费已编译的理解层工件，不再旁路读取散点 JSON 或新增本地 prompt 规则。
4. [ ] `required_slots[]` 没有演变成第二套执行必填真相，`Clarification` 主链仍由 `243` 唯一裁决。
5. [ ] `route_catalog_version / knowledge_snapshot_digest` 的变更可被稳定复算并解释原因。
6. [ ] 至少三类样例（动作 / 非动作 / 待澄清）已进入正式资产与测试，而不是仅留在 dev-plan 文本描述。
7. [ ] 理解层“完成治理”的结论可以由代码、资产、测试、执行记录四类证据同时支持。

## 11. 停止线（Fail-Closed）
1. [ ] 若 `244` 最终只是补几份 JSON 样例，而没有建立迁移清单、交叉引用校验与清退 stopline，本计划失败。
2. [ ] 若 `required_slots[]`、`negative_examples[]` 或模板文本开始定义 `phase / required_fields / confirm/commit` 真相，本计划失败。
3. [ ] 若实现结果继续依赖 `assistant_knowledge_runtime.go`、`assistant_api.go`、`assistant_reply_nlg.go` 中的散点 prompt 作为主要理解来源，本计划失败。
4. [ ] 若编译器对非法模板引用、非法 source refs、非法 slot、非法 confidence 选择静默降级，本计划失败。
5. [ ] 若 `244` 反向改写 `242/243` 的 runtime 职责边界，把 route/clarification 主链重新揉回知识编译器，本计划失败。
6. [ ] 若为兼容旧逻辑而保留“资产与本地字符串双读取、谁先命中算谁”的双主源，本计划失败。

## 12. 门禁与本地验证入口
1. [ ] 文档与实现触发器以 `AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 为准。
2. [ ] 本计划进入代码实现后，至少命中：
   - [ ] `go fmt ./... && go vet ./... && make check lint && make test`
   - [ ] `make check no-legacy`
   - [ ] `make check error-message`
   - [ ] `make check doc`
3. [ ] 若命中 Assistant 配置主源、route 映射、文案错误码或生成物，还需按 SSOT 补跑相应门禁；本文不复制脚本实现。

## 13. 交付物
1. [ ] 本计划文档：`docs/dev-plans/244-assistant-interpretation-pack-and-intent-route-catalog-compiler-plan.md`
2. [ ] 执行记录：`docs/dev-records/dev-plan-244-execution-log.md`
3. [ ] 代码交付物：
   - [ ] interpretation pack / route catalog 契约加固；
   - [ ] 编译器分层与交叉引用校验；
   - [ ] 首批正式样例资产；
   - [ ] 理解层散点 prompt/规则迁移清单；
   - [ ] 回归测试与封板证据。

## 14. 关联文档
- `docs/dev-plans/240e-assistant-internal-knowledge-pack-and-readonly-resolver-plan.md`
- `docs/dev-plans/241-assistant-knowledge-pack-runtime-minimal-implementation-plan.md`
- `docs/dev-plans/242-assistant-intent-router-runtime-minimal-plan.md`
- `docs/dev-plans/243-assistant-clarification-policy-and-slot-repair-plan.md`
- `docs/dev-plans/245-assistant-reply-guidance-pack-and-reply-realizer-plan.md`
- `docs/dev-plans/246-assistant-understand-route-clarify-roadmap.md`
