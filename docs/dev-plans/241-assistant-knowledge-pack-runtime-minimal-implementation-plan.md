# DEV-PLAN-241：Assistant 知识资产运行时最小实现计划（承接 240E）

**状态**: 规划中（2026-03-11 CST；本次修订将 `241` 明确为 `240E` 的“执行前置能力准备 + plan 阶段最小接线”子计划，并把 `Intent Router / Clarification / Reply Guidance` 等扩面能力拆分给 `242~245`）

## 1. 背景与定位
1. [ ] `DEV-PLAN-240E` 已冻结为知识治理主契约：明确四类知识资产、主源矩阵、版本审计与模板边界；`241` 只负责最小运行时实现，不再发明新的资产类型或主源分配。
2. [ ] 当前 Assistant 的知识仍主要散落在以下位置：
   - [ ] `docs/dev-plans/240e-assistant-internal-knowledge-pack-and-readonly-resolver-plan.md` 等契约文档；
   - [ ] `internal/server/assistant_action_registry.go` 中的 `PlanTitle / PlanSummary / CapabilityKey / RequiredChecks`；
   - [ ] `internal/server/assistant_api.go` 中的意图提取、计划构建、干跑解释与错误提示；
   - [ ] `internal/server/assistant_reply_nlg.go` 与相关回复网关中的用户提示文案。
3. [ ] 当前 PostgreSQL 已持久化会话、turn、task 与审计快照，但这些表保存的是上下文事实，不是独立、结构化、可版本冻结的知识资产。
4. [ ] 本计划的定位冻结为：在不引入新的外部协议主链、不扩大工具生态的前提下，为 `240E` 落地一条**最小可运行知识消费链**，优先服务 `org.orgunit_create` 与一个非动作样例。
5. [ ] 本计划不重写 `240E` 的契约；若 `241` 与 `240E` 冲突，以 `240E` 的资产模型、主源矩阵、版本审计与停止线为准，并先回写主计划再改实现。

## 2. 目标与非目标

### 2.1 核心目标
1. [ ] 为 `240E` 定义的四类知识资产落地最小 schema、Loader 与语义校验能力。
2. [ ] 在运行时建立最小 `Readonly Resolver` 分层，实现对会话快照、业务对象事实、动作契约投影与错误解释的结构化只读查询。
3. [ ] 在 turn/task 审计链中显式记录 `knowledge_snapshot_digest / route_catalog_version / resolver_contract_version / context_template_version / reply_guidance_version`。
4. [ ] 仅在受控模板下，把知识资产与 Resolver 结果接入一个动作、一个阶段的最小闭环，验证知识层不会与执行主链形成双主源。
5. [ ] 让 `knowledge_qa/chitchat` 非动作样例成为一等公民，验证非动作输入不会误入 `confirm/commit`。

### 2.2 非目标
1. [ ] 不在本计划内引入 Redis、外部向量数据库、外部知识平台、MCP 新主链或额外运行时协议。
2. [ ] 不在本计划内建设通用 RAG 文档上传、切片、向量化、跨文档开放问答；这类需求由 `DEV-PLAN-250` 另行承接。
3. [ ] 不改变 `260/223/240A~240D` 已冻结的业务 FSM、持久化事实源、confirm/commit/task 正式语义。
4. [ ] 不允许 `241` 在运行时新增知识资产类型、重分配主源职责或放宽 `240E` 的模板白名单与版本审计要求。
5. [ ] 首期不追求全阶段覆盖；优先落地单动作、单阶段、单模板闭环，再逐步扩面。
6. [ ] 不在本计划中承担 `route` 阶段正式 Runtime 重构：`Intent Router` 由 `DEV-PLAN-242` 承接，`Clarification Policy` 由 `DEV-PLAN-243` 承接。
7. [ ] 不在本计划中承担理解知识资产的大规模治理扩面：`Interpretation Pack + Intent Route Catalog` 编译治理由 `DEV-PLAN-244` 承接。
8. [ ] 不在本计划中承担 `reply` 主链知识化改造：`Reply Guidance Pack + Reply Realizer` 由 `DEV-PLAN-245` 承接。

## 2.3 与 242~245 的边界冻结
1. [ ] `241` 只负责为后续扩面打地基：知识资产 schema、语义校验、最小快照、最小 Resolver、`plan_context_v1`。
2. [ ] `241` 可以为 `242~245` 预留通用接口与快照字段，但不得提前把 `route_context_v1 / confirm_context_v1 / reply_context_v1` 接入运行时主链。
3. [ ] `241` 中若出现“需要先判断 `route_kind` 再决定是否进入动作链”的需求，应登记到 `242`，而不是在本计划内临时加旁路判断。
4. [ ] `241` 中若出现“需要根据低置信度自动追问”的需求，应登记到 `243`，而不是继续扩写本地规则兜底。
5. [ ] `241` 中若出现“需要统一澄清提问/成功失败表达”的需求，应分别登记到 `244/245`，不在本计划内用 helper 文案硬补。

## 3. 承接 `240E` 的实现约束（冻结）
1. [ ] `241` 只实现以下四类知识资产，不再回退到“大 Knowledge Pack” 模型：
   - [ ] `Interpretation Pack`
   - [ ] `Action View Pack`
   - [ ] `Reply Guidance Pack`
   - [ ] `Intent Route Catalog`
2. [ ] `241` 不得在知识资产中声明 `required_fields / phase / confirm 条件 / commit 条件` 等执行真相字段。
3. [ ] 真正的必填字段、阶段推进条件、提交条件只能由执行主链通过 `Contract Resolver` 只读投影获得。
4. [ ] `241` 的上下文接线只能基于 `Context Template Registry` 的注册模板进行，不允许运行时自由拼装字段。
5. [ ] 若 `241` 需要新增资产字段、模板类型、版本语义或审计口径，必须先回写 `240E`。

## 4. 实施范围冻结
1. [ ] 首期动作样例：`org.orgunit_create`。
2. [ ] 首期非动作样例：`knowledge_qa` 或 `chitchat` 至少一种。
3. [ ] 首期运行时接线阶段：`plan`。
4. [ ] 首期模板：`plan_context_v1`。
5. [ ] `reply` 阶段仅保留资产与版本准备，不作为首批运行时硬目标；后续扩面须在首批证据稳定后推进。
6. [ ] 首期允许补一个“为 `242/243` 准备的 route 输入快照骨架”，但该骨架不得参与本计划运行时裁决。
7. [ ] 若 `Intent Route Catalog` 提供 `required_slots[] / clarification_template_id`，其语义只限于 route/clarification 的排序与模板引用；不得在 `241` 内被解释为正式执行必填真相。
8. [ ] 首期允许补一个“为 `245` 准备的 reply 资产索引与版本字段”，但不得切换现有 reply 主链。

## 5. 知识资产文件结构（首期冻结）
1. [ ] 建议文件组织（名称可微调，但模型不变）：
   - [ ] `internal/server/assistant_knowledge/interpretation/*.json`
   - [ ] `internal/server/assistant_knowledge/action_view/*.json`
   - [ ] `internal/server/assistant_knowledge/reply_guidance/*.json`
   - [ ] `internal/server/assistant_knowledge/intent_route_catalog.json`
2. [ ] 首期至少提供以下样例：
   - [ ] `org.orgunit_create` 的 `Action View Pack`；
   - [ ] `org.orgunit_create` 所需的最小 `Interpretation Pack`；
   - [ ] 一个 `knowledge_qa/chitchat` 的 `Interpretation Pack`；
   - [ ] 一个最小 `Reply Guidance Pack` 样例（即使首批暂不接入 `reply` 运行时，也必须完成 schema 与版本冻结）；
   - [ ] 对应的 `Intent Route Catalog` 样例。
3. [ ] 首期支持语言冻结为 `zh/en`。
4. [ ] 若某项知识无法被结构化进上述资产类型，则视为知识设计未成熟，不得先以原文直塞方式进入运行时。

## 6. Schema、Loader 与语义校验
1. [ ] 首期优先实现 `Knowledge Contract Compiler` 的最小能力，而不是先做广覆盖接线。
2. [ ] 最小校验范围至少包括：
   - [ ] schema 完整性；
   - [ ] `action_id` 必须已注册；
   - [ ] `intent_id` 必须唯一；
   - [ ] `route_kind` 必须合法；
   - [ ] locale 仅允许 `zh/en`；
   - [ ] 错误码引用必须存在；
   - [ ] 模板字段引用必须命中白名单；
   - [ ] 知识资产不得声明执行真相字段。
3. [ ] Loader 负责：
   - [ ] 加载 schema 与工件；
   - [ ] 计算 digest；
   - [ ] 建立按 `asset_type / action_id / locale / template_version` 的索引；
   - [ ] 对坏 schema、重复键、非法引用直接 fail-closed。
4. [ ] `source_refs[]` 必须被校验为有效仓内引用，且至少存在一个。

## 7. Readonly Resolver 最小实现
1. [ ] 首期 Resolver 不按“功能杂糅”实现，而按责任边界分层：
   - [ ] `conversation_snapshot_resolver`：返回上一轮 `phase / missing_fields / candidates / selected_candidate_id / error_code` 等上下文；
   - [ ] `domain_fact_resolver`：返回候选组织的路径、编码、状态、生效日期与最小解释字段；
   - [ ] `contract_projection_resolver`：只读投影动作所需字段展示、字段约束与执行主链摘要；
   - [ ] `error_catalog_resolver`：把错误码映射到用户可解释的结构化说明。
2. [ ] 首期可先落地其中最小组合，但接口层必须按上述责任边界冻结。
3. [ ] Resolver 输出必须结构化且 fail-closed：查询失败时返回受控错误，不允许 silent fallback 为自由文本猜测。
4. [ ] Resolver 不得直接修改 turn、不得提交业务写入、不得覆盖正式 DTO 字段、不得输出推荐决策。

## 8. 上下文模板与接线策略
1. [ ] 首期只允许使用 `plan_context_v1` 模板，不同时推进 `route / confirm / reply` 多阶段接线。
2. [ ] `plan_context_v1` 最小装配内容建议为：
   - [ ] `action_view_pack.summary`
   - [ ] `field_display_map[]`
   - [ ] `missing_field_guidance[]`
   - [ ] `contract_projection.required_fields_view`
   - [ ] `contract_projection.action_spec_summary`
   - [ ] `conversation_snapshot.current_phase`
3. [ ] `plan_context_v1` 必须冻结：
   - [ ] 允许字段白名单；
   - [ ] 最大候选数；
   - [ ] 最大字符预算；
   - [ ] 是否允许历史摘要。
4. [ ] 首期只替换一个用户可见切面：`assistantBuildPlan` 的计划摘要与缺字段解释，不同时替换 `reply` 主链。
5. [ ] 非动作样例首期只验证 route 层分流与“不会误入 commit”，不要求与业务动作共享同一运行时模板。

## 9. 版本快照与审计收口
1. [ ] 版本快照必须前置，不得等到运行时大面积接线后再补。
2. [ ] 首期至少纳入以下字段：
   - [ ] `knowledge_snapshot_digest`
   - [ ] `route_catalog_version`
   - [ ] `resolver_contract_version`
   - [ ] `context_template_version`
   - [ ] `reply_guidance_version`
3. [ ] 首期优先复用现有 turn/task JSON 快照与任务合同字段，尽量避免新表。
4. [ ] DoD：
   - [ ] 能回放某次对话使用了哪版知识资产与模板；
   - [ ] 能判断知识版本变更是否影响历史证据新鲜度；
   - [ ] 不形成第二事实源。

## 10. 分批实施

### 10.1 PR-241-01：知识资产契约、Schema 与语义校验
1. [ ] 定义四类知识资产 schema、Go 结构体、digest 计算与 `Knowledge Contract Compiler` 最小实现。
2. [ ] 落地 `org.orgunit_create` 与一个非动作样例的最小资产样例。
3. [ ] DoD：
   - [ ] 运行时能按资产类型与标识取到唯一工件；
   - [ ] 坏 schema、重复键、非法引用直接 fail-closed；
   - [ ] `source_refs[]` 与 locale 校验通过；
   - [ ] `knowledge_snapshot_digest` 可稳定复算。

### 10.2 PR-241-02：版本快照与审计前置
1. [ ] 将 `knowledge_snapshot_digest / route_catalog_version / resolver_contract_version / context_template_version / reply_guidance_version` 纳入 turn/task 快照。
2. [ ] DoD：
   - [ ] 任意一次对话都可追溯知识版本；
   - [ ] 知识变更可被证据新鲜度规则识别；
   - [ ] 未写入快照时不得启用运行时消费。
3. [ ] 兼容性约束：
   - [ ] 新快照字段必须与现有 `intent_schema_version / compiler_contract_version / capability_map_version / skill_manifest_digest / context_hash / intent_hash / plan_hash` 并存，不得覆盖既有执行契约快照；
   - [ ] `route` 与 `reply` 相关字段即使暂未接线，也必须有稳定空值/默认值语义，避免后续扩面时破坏历史回放。

### 10.3 PR-241-03：Resolver 最小实现
1. [ ] 实现 `conversation_snapshot_resolver` 与 `contract_projection_resolver` 的最小组合；必要时再补 `domain_fact_resolver`。
2. [ ] DoD：
   - [ ] Resolver 全部只读；
   - [ ] 输出带租户/请求关联字段；
   - [ ] Resolver 错误能映射为受控错误码；
   - [ ] 不输出推荐决策或写入口信息。
3. [ ] 可扩展性约束：
   - [ ] 接口签名必须能被 `242/243/245` 复用；
   - [ ] `conversation_snapshot_resolver` 需能输出后续 route/clarification 所需的最小上下文字段，但本计划内不消费这些字段做 route 决策；
   - [ ] `error_catalog_resolver` 至少预留接口与版本字段，便于 `245` 统一失败解释。

### 10.4 PR-241-04：单动作、单阶段、单模板接线
1. [ ] 在 `assistantBuildPlan` 前增加 `plan_context_v1` 上下文装配点。
2. [ ] 首期仅替换以下知识来源：
   - [ ] `assistantBuildPlan` 的摘要说明；
   - [ ] 与 `plan` 阶段直接相关的缺字段解释文本。
3. [ ] DoD：
   - [ ] 用户可见计划摘要优先来自知识资产与只读投影；
   - [ ] 原有执行语义与 DTO 字段不变；
   - [ ] 无“整篇原文直塞”痕迹；
   - [ ] 非动作输入不会误入 `confirm/commit`。
4. [ ] 约束补充：
   - [ ] `assistantBuildPlan` 与缺字段解释切换后，不得再新增新的硬编码解释入口；
   - [ ] 候选说明若在本批次一并接线，仍必须归属 `plan_context_v1`，不得提前发明 `reply_context_v1` 旁路；
   - [ ] 若发现仅靠 `plan_context_v1` 无法承载 route 澄清，应停止扩大 `241` 范围并转交 `242/243`。

### 10.5 PR-241-05：非动作样例与后续扩面准备
1. [ ] 验证 `knowledge_qa/chitchat` 至少一种样例能被稳定分流。
2. [ ] 为 `reply_context_v1` 预留资产与版本口径；首批需提供最小 `Reply Guidance Pack` 样例，但不在本批次中强行扩大运行时接线范围。
3. [ ] DoD：
   - [ ] 非动作样例成为一等公民，而非测试补丁；
   - [ ] 扩面条件清晰：必须以首批证据稳定为前提。
4. [ ] 与后续计划的交接物：
   - [ ] 向 `242` 输出可消费的 `Intent Route Catalog` 最小样例与 route 快照字段；
   - [ ] 向 `243` 输出可消费的缺字段/候选/错误码结构化输入；
   - [ ] 向 `245` 输出可消费的 `Reply Guidance Pack` 样例与 `reply_guidance_version` 口径。

## 11. 测试与覆盖率
1. [ ] 覆盖率口径：沿用仓库当前 Go 测试与 CI 门禁，不新增排除项。
2. [ ] 统计范围至少覆盖：
   - [ ] 知识资产 loader/schema/semantic validator/digest 计算；
   - [ ] Resolver 只读查询与错误路径；
   - [ ] `plan_context_v1` 的最小装配结果；
   - [ ] 租户隔离与 fail-closed；
   - [ ] 非动作输入不会误入 `confirm/commit`。
3. [ ] 关键场景：
   - [ ] 同一资产键出现重复时阻断；
   - [ ] 非法 `action_id / intent_id / 错误码 / template field` 阻断；
   - [ ] Resolver 返回空数据/跨租户数据时阻断；
   - [ ] 知识版本变化能被 turn/task 快照识别；
   - [ ] `org.orgunit_create` 的计划摘要与缺字段解释来源统一；
   - [ ] `knowledge_qa/chitchat` 不触发业务动作路径。
4. [ ] 回归保护（承接本次问题复盘）：
   - [ ] 现有“自然语言表达不规范但语义明确”的样例，在 `241` 范围内至少不能再因为计划摘要/缺字段解释漂移而恶化；
   - [ ] `241` 不以扩充 `assistantExtractIntent` 规则作为主要交付，不把“理解僵化”的根因伪装成 `plan` 层收口。

## 12. 停止线（Fail-Closed）
1. [ ] 若实现仍直接把 dev-plan、skill 原文全文注入运行时，则本计划失败。
2. [ ] 若知识资产声明了与执行主链冲突的真相字段，则本计划失败。
3. [ ] 若 Resolver 承担写操作、覆盖正式 DTO 字段、输出推荐决策或跨租户读取，则本计划失败。
4. [ ] 若 `Context Assembler` 使用未注册模板或自由拼接模板外字段，则本计划失败。
5. [ ] 若未写入知识版本快照就进入运行时消费，则本计划失败。
6. [ ] 若首批实现同时扩到多个动作、多个阶段或多个模板，导致无法证明最小闭环，则本计划失败。
7. [ ] 若用户可见业务反馈仍主要依赖页面本地拼接 helper，而非统一知识主链，则本计划失败。
8. [ ] 若 `241` 为追求表面闭环而把 `Intent Router / Clarification / Reply` 临时塞回本计划，导致与 `242~245` 边界混乱，则本计划失败。

## 13. 验收标准
1. [ ] 仓库内存在可审阅、可版本冻结的四类知识资产样例，并至少覆盖 `org.orgunit_create` 与一个非动作样例。
2. [ ] 运行时存在显式 Resolver 接口，且其输出结构化、只读、带审计关联字段。
3. [ ] turn/task 快照可追溯当次对话所用知识版本、digest 与模板版本。
4. [ ] `assistantBuildPlan` 已消费受控模板装配的知识上下文，而非继续直接依赖零散硬编码文本。
5. [ ] 非动作输入能稳定被分流，且不会误入 `confirm/commit`。
6. [ ] 整个实现不引入新的外部工具链前置条件，不改变 `240A~240D` 的正式执行主链。
7. [ ] `241` 完成后，`242~245` 可在不重写 `241` 产物的前提下继续扩面。

## 14. 门禁与 SSOT 引用
1. [ ] 文档与实现触发器以 `AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 为准。
2. [ ] 本计划一旦进入代码实现，至少命中以下门禁：
   - [ ] `go fmt ./... && go vet ./... && make check lint && make test`
   - [ ] `make check no-legacy`
   - [ ] `make check error-message`
   - [ ] `make check doc`
3. [ ] 若实现触达 Assistant 路由、能力映射或错误码契约，还需按实际命中情况补跑对应门禁；本文不复制脚本细节，以 SSOT 为准。

## 15. 交付物
1. [ ] 本计划文档：`docs/dev-plans/241-assistant-knowledge-pack-runtime-minimal-implementation-plan.md`
2. [ ] 后续执行记录：`docs/dev-records/dev-plan-241-execution-log.md`
3. [ ] 后续实现产物（待批准实施后落地）：
   - [ ] 四类知识资产 schema 与样例；
   - [ ] `Knowledge Contract Compiler` 最小实现；
   - [ ] `Readonly Resolver` 接口与最小实现；
   - [ ] `plan_context_v1` 接线说明；
   - [ ] 相关测试与证据索引。

## 16. 关联文档
- `docs/dev-plans/240-assistant-org-transaction-orchestration-modernization-plan.md`
- `docs/dev-plans/240e-assistant-internal-knowledge-pack-and-readonly-resolver-plan.md`
- `docs/dev-plans/240f-assistant-280-aligned-closure-and-regression-plan.md`
- `docs/dev-plans/250-go-gateway-rag-and-authz-phase1-2-plan.md`
- `docs/dev-plans/242-assistant-intent-router-runtime-minimal-plan.md`
- `docs/dev-plans/243-assistant-clarification-policy-and-slot-repair-plan.md`
- `docs/dev-plans/244-assistant-interpretation-pack-and-intent-route-catalog-compiler-plan.md`
- `docs/dev-plans/245-assistant-reply-guidance-pack-and-reply-realizer-plan.md`
- `docs/dev-plans/246-assistant-understand-route-clarify-roadmap.md`
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
