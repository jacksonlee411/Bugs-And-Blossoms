# DEV-PLAN-241：Assistant 知识包运行时最小落地计划（承接 240E）

**状态**: 规划中（2026-03-10 08:16 CST）

## 1. 背景与定位
1. [ ] `DEV-PLAN-240E` 已冻结“内部知识包 + 只读 Resolver”的契约方向，但当前仍停留在知识分类、主源规则与推荐架构层面，尚未形成最小运行时实现。
2. [ ] 当前 Assistant 的知识维护仍主要分散在以下位置：
   - [ ] `docs/dev-plans/240e-assistant-internal-knowledge-pack-and-readonly-resolver-plan.md` 等契约文档；
   - [ ] `internal/server/assistant_action_registry.go` 中的 `PlanTitle / PlanSummary / CapabilityKey / RequiredChecks`；
   - [ ] `internal/server/assistant_api.go` 中的 `assistantExtractIntent`、`assistantBuildPlan`、`assistantBuildDryRun`、错误解释与候选提示；
   - [ ] `internal/server/assistant_reply_nlg.go` 与 `internal/server/assistant_reply_model_gateway.go` 中的用户回复提示与 fallback 文案。
3. [ ] 当前 PostgreSQL 已持久化会话、turn、task 与审计快照，但这些表保存的是**上下文事实**，不是独立、结构化、可版本冻结的知识包。
4. [ ] 本计划的定位冻结为：在不引入新的外部协议主链、不扩大工具生态的前提下，为 `240E` 落地一条 **最小可运行知识主链**，优先服务 `260` Case 2~4 的业务动作场景。
5. [ ] 本计划是专门实施计划，不重写 `240E` 的契约；若本计划与 `240E` 冲突，以 `240E` 的范围和停止线为准，并先回写契约再改实现。

## 2. 目标与非目标

### 2.1 核心目标
1. [ ] 建立可提交到仓库、可审阅、可版本冻结的 `Knowledge Pack` 结构，替代运行时对零散代码文案的直接依赖。
2. [ ] 建立最小 `Readonly Resolver` 层，为 `understand/route/plan/confirm/reply` 提供结构化动态只读知识。
3. [ ] 建立最小 `Context Assembler`，把静态知识包、动态 Resolver 结果、现有执行主链快照装配为模型可消费的最小上下文。
4. [ ] 让 `assistantExtractIntent`/`assistantBuildPlan`/`assistantReplyFallbackText` 中的知识性内容逐步迁出到结构化知识包，代码只保留执行骨架与 fail-closed 校验。
5. [ ] 在不新增第二事实源的前提下，把 `knowledge_version` / `knowledge_pack_digest` 等快照显式纳入 turn/task 审计链。
6. [ ] 保持现有 `240A~240D` 的执行主链不变：动作注册、风险门、提交适配、任务状态机、DTO rebuild 仍为正式语义主源。

### 2.2 非目标
1. [ ] 不在本计划内引入 `Redis`、外部向量数据库、独立知识平台、MCP 新主链或额外运行时协议。
2. [ ] 不在本计划内建设通用 RAG 文档上传、切片、向量化、跨文档开放问答；这类需求由 `DEV-PLAN-250` 另行承接。
3. [ ] 不改变 `260/223/240D` 已冻结的业务 FSM、持久化事实源、confirm/commit/task 正式语义。
4. [ ] 不允许 Resolver 承担写职责，不允许知识层绕过 `CommitAdapter`、任务主链或 DB Kernel 写入口。
5. [ ] 不在本计划内扩展到所有 Assistant 能力；首期仅覆盖 `org.orgunit_create` 及其相邻的缺字段提示、候选解释、确认摘要、成功/失败回执。

## 3. 现状评估（作为实施输入冻结）
1. [ ] 静态/执行知识当前主要硬编码在 Go：
   - [ ] 动作摘要与安全要求：`internal/server/assistant_action_registry.go`
   - [ ] 意图本地兜底与计划/干跑解释：`internal/server/assistant_api.go`
   - [ ] 回复提示与技术信号屏蔽：`internal/server/assistant_reply_nlg.go`
2. [ ] 动态知识当前通过业务查询临时拼装，例如候选组织搜索与详情读取来自 `OrgUnitStore`；但没有独立的 Resolver 接口与审计契约。
3. [ ] 持久化层当前能保存 `intent_json / plan_json / candidates_json / dry_run_json / commit_result_json`，并具备 `context_hash / intent_hash / plan_hash / skill_manifest_digest` 等任务快照字段；这为知识层版本快照预留了审计落点。
4. [ ] 当前最缺的不是知识内容本身，而是：
   - [ ] 缺少结构化、最小化、运行时可直接消费的知识包格式；
   - [ ] 缺少统一的动态只读查询接口；
   - [ ] 缺少把知识包/Resolver/执行主链快照拼成同一份上下文的装配点；
   - [ ] 缺少与 turn/task 绑定的知识版本快照。

## 4. 设计总览

### 4.1 单主源原则
1. [ ] **人类维护主源**：仍可来自 `DEV-PLAN`、错误码目录、规则文档与代码中的已冻结执行语义。
2. [ ] **运行时消费主源**：必须是结构化 `Knowledge Pack` 与结构化 `Resolver Result`，不得直接将 dev-plan 全文、skill 全文或页面 helper 文案塞给模型。
3. [ ] **执行语义主源**：仍为 `AssistantActionSpec`、`capability_key`、`required_checks`、`CommitAdapter`、task 状态机与 DTO 字段，不由知识包重定义。

### 4.2 存储分层
1. [ ] **静态知识包存储**：首期采用仓内文件形式（建议 `internal/server/assistant_knowledge/packs/*.json`），由 Go 运行时加载；不引入独立数据库表或外部知识服务。
2. [ ] **动态知识存储**：继续来自 PostgreSQL 现有事实表与域服务只读查询，不单独复制落库。
3. [ ] **知识快照存储**：首期优先复用现有 turn/task JSON 与快照字段，记录：
   - [ ] `knowledge_pack_version`
   - [ ] `knowledge_pack_digest`
   - [ ] `resolver_contract_version`
   - [ ] `context_assembly_version`
4. [ ] **审计索引**：`source_refs[]` 只用于调试与审计，不得成为用户可见文案的直接来源。

### 4.3 运行时组件
1. [ ] `Knowledge Pack Builder`：把人类维护来源提炼为结构化包；首期可采用“人工维护 JSON + loader 校验”方式，不强求自动生成器。
2. [ ] `Knowledge Pack Loader`：负责加载、schema 校验、digest 计算、按 `action_id/phase/locale` 索引。
3. [ ] `Readonly Resolver Layer`：提供显式接口，输出结构化结果并带 `tenant_id / conversation_id / turn_id / request_id / trace_id` 关联字段。
4. [ ] `Intent Router`：首期不单独引入模型编排器；先在现有 `resolveIntent` 前后补充 route metadata 与澄清策略，不改变业务主链入口。
5. [ ] `Context Assembler`：在 `understand/route/plan/confirm/reply` 前按需选择知识包片段与 Resolver 结果，输出最小 JSON 上下文。

## 5. Knowledge Pack 结构（首期冻结）
1. [ ] 每个知识包至少包含以下字段：
   - [ ] `action_id`
   - [ ] `knowledge_version`
   - [ ] `locale`
   - [ ] `summary`
   - [ ] `intent_classes[]`
   - [ ] `intent_to_action_rules[]`
   - [ ] `required_fields[]`
   - [ ] `clarification_prompts[]`
   - [ ] `confirmation_rules`
   - [ ] `candidate_explanation_rules`
   - [ ] `success_reply_guidance`
   - [ ] `failure_reply_guidance`
   - [ ] `negative_examples[]`
   - [ ] `source_refs[]`
2. [ ] 首期必须至少提供两类样例：
   - [ ] `org.orgunit_create`
   - [ ] 一个非动作样例（`knowledge_qa` 或 `chitchat`）
3. [ ] 首期支持语言冻结为 `zh/en`，与仓库现行 i18n 约束一致。
4. [ ] 若某条知识无法被结构化进上述字段，则视为知识设计未成熟，不得先以“原文直塞”方式进入运行时。

## 6. Readonly Resolver 设计（首期最小集）
1. [ ] 首期只实现以下 Resolver：
   - [ ] `conversation_intent_context_resolver`：返回上一轮 `phase / missing_fields / candidates / selected_candidate_id / error_code` 等上下文；
   - [ ] `candidate_detail_resolver`：返回候选组织的路径、编码、状态、生效日期与最小解释字段；
   - [ ] `field_constraint_resolver`：返回动作所需字段、日期格式与缺字段解释；
   - [ ] `error_explanation_resolver`：把错误码映射到用户可解释的结构化说明。
2. [ ] 首期不实现跨文档检索、语义相似度 TopK、向量召回。
3. [ ] Resolver 输出必须结构化且 fail-closed：查询失败时返回受控错误，不允许 silent fallback 为自由文本猜测。
4. [ ] Resolver 不得直接修改 turn、不得提交业务写入、不得覆盖正式 DTO 字段。

## 7. 上下文装配策略
1. [ ] `understand/route`：装配 `intent route rule + conversation context + allowed/blocked action set + clarification prompts`。
2. [ ] `plan`：装配 `action knowledge pack + required_fields + field constraints + action spec summary`。
3. [ ] `confirm`：装配 `confirmation_rules + candidate_explanation + selected candidate detail + pending draft summary`。
4. [ ] `reply`：装配 `success/failure guidance + error explanation + current machine state`。
5. [ ] 所有上下文装配结果必须是**最小 JSON**，不允许把整份包、整篇文档或过量历史消息原样塞入模型。

## 8. 工具链评估与结论
1. [ ] 现阶段**不引入新工具链**作为本计划前置条件；首期仅使用现有：
   - [ ] Go
   - [ ] PostgreSQL / JSONB
   - [ ] 现有 `assistantModelGateway`
   - [ ] 现有 `OrgUnitStore` 与会话/任务持久化
2. [ ] 不引入 `pgvector`；若未来出现“文档上传 + 切片 + 语义检索”明确需求，再由 `DEV-PLAN-250` 承接。
3. [ ] 不引入 Redis/BigCache/Ristretto；缓存若有需要，仅允许首期使用进程内短 TTL 或 request-scope 复用。
4. [ ] 不引入新的前端 helper、Node.js bridge、外部知识 API 来承接 `240E/241` 主职责。

## 9. 分批实施

### 9.1 PR-241-01：知识包契约与 Loader
1. [ ] 定义知识包 JSON schema、Go 结构体、digest 计算与加载校验逻辑。
2. [ ] 落地 `org.orgunit_create` 的 `zh/en` 最小知识包样例。
3. [ ] DoD：
   - [ ] 运行时能按 `action_id` 取到唯一知识包；
   - [ ] 缺字段、坏 schema、重复 action/locale 直接 fail-closed；
   - [ ] `knowledge_pack_digest` 可稳定复算。

### 9.2 PR-241-02：Resolver 最小实现
1. [ ] 定义 Resolver 接口与统一结果结构。
2. [ ] 落地 `conversation_intent_context_resolver`、`candidate_detail_resolver`、`field_constraint_resolver`、`error_explanation_resolver`。
3. [ ] DoD：
   - [ ] Resolver 全部只读；
   - [ ] 输出带租户/请求关联字段；
   - [ ] Resolver 错误能映射为受控错误码。

### 9.3 PR-241-03：Context Assembler 接线
1. [ ] 在 `resolveIntent`、`build plan`、`render reply` 之前增加上下文装配点。
2. [ ] 先替换以下知识来源：
   - [ ] `assistantBuildPlan` 的摘要说明；
   - [ ] `assistantBuildDryRun` 的缺字段解释文本；
   - [ ] `assistantReplyFallbackText` 的关键业务提示文本。
3. [ ] DoD：
   - [ ] 用户可见提示优先来自知识包/Resolver；
   - [ ] 原有执行语义与 DTO 字段不变；
   - [ ] 无“整篇原文直塞”痕迹。

### 9.4 PR-241-04：知识快照与审计收口
1. [ ] 将 `knowledge_pack_version / knowledge_pack_digest / resolver_contract_version / context_assembly_version` 纳入 turn/task 快照。
2. [ ] 首期优先复用现有 JSON 快照与任务合同字段，尽量避免新表。
3. [ ] DoD：
   - [ ] 能回放某次对话使用了哪版知识包；
   - [ ] 能判断知识版本变更是否影响历史证据新鲜度；
   - [ ] 不形成第二事实源。

## 10. 测试与覆盖率
1. [ ] 覆盖率口径：沿用仓库当前 Go 测试与 CI 门禁，不新增排除项。
2. [ ] 统计范围至少覆盖：
   - [ ] 知识包 loader/schema 校验/digest 计算；
   - [ ] Resolver 只读查询与错误路径；
   - [ ] Context Assembler 的最小装配结果；
   - [ ] 租户隔离与 fail-closed；
   - [ ] 非动作输入不会误入 `confirm/commit`。
3. [ ] 关键场景：
   - [ ] 同一 `action_id + locale` 出现重复知识包时阻断；
   - [ ] Resolver 返回空数据/跨租户数据时阻断；
   - [ ] 知识包版本变化能被 turn/task 快照识别；
   - [ ] `org.orgunit_create` 的缺字段解释、候选解释、确认摘要来源统一；
   - [ ] `knowledge_qa/chitchat` 不触发业务动作路径。

## 11. 停止线（Fail-Closed）
1. [ ] 若实现仍直接把 dev-plan、skill 原文全文注入运行时，则本计划失败。
2. [ ] 若 Resolver 承担写操作或覆盖正式 DTO 字段，则本计划失败。
3. [ ] 若知识包重新定义 `ActionSpec / CommitAdapter / task` 正式语义，则本计划失败。
4. [ ] 若为落地知识层而引入 Redis、外部向量库、外部知识平台或 Node.js 第二主链，则本计划失败。
5. [ ] 若用户可见业务反馈仍主要依赖页面本地拼接 helper，而非统一知识主链，则本计划失败。

## 12. 验收标准
1. [ ] 仓库内存在可审阅、可版本冻结的知识包文件，并至少覆盖 `org.orgunit_create` 与一个非动作样例。
2. [ ] 运行时存在显式 Resolver 接口，且其输出结构化、只读、带审计关联字段。
3. [ ] `resolveIntent / plan / reply` 至少三个关键阶段已消费知识包或 Resolver 结果，而非继续直接依赖零散硬编码文本。
4. [ ] turn/task 快照可追溯当次对话所用知识版本与 digest。
5. [ ] 整个实现不引入新的外部工具链前置条件，不改变 `240A~240D` 的正式执行主链。

## 13. 门禁与 SSOT 引用
1. [ ] 文档与实现触发器以 `AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 为准。
2. [ ] 本计划一旦进入代码实现，至少命中以下门禁：
   - [ ] `go fmt ./... && go vet ./... && make check lint && make test`
   - [ ] `make check no-legacy`
   - [ ] `make check error-message`
   - [ ] `make check doc`
3. [ ] 若实现触达 Assistant 路由、能力映射或错误码契约，还需按实际命中情况补跑对应门禁；本文不复制脚本细节，以 SSOT 为准。

## 14. 交付物
1. [ ] 本计划文档：`docs/dev-plans/241-assistant-knowledge-pack-runtime-minimal-implementation-plan.md`
2. [ ] 后续执行记录：`docs/dev-records/dev-plan-241-execution-log.md`
3. [ ] 后续实现产物（待批准实施后落地）：
   - [ ] `Knowledge Pack` schema 与样例；
   - [ ] `Readonly Resolver` 接口与最小实现；
   - [ ] `Context Assembler` 接线说明；
   - [ ] 相关测试与证据索引。

## 15. 关联文档
- `docs/dev-plans/240-assistant-org-transaction-orchestration-modernization-plan.md`
- `docs/dev-plans/240e-assistant-internal-knowledge-pack-and-readonly-resolver-plan.md`
- `docs/dev-plans/240f-assistant-280-aligned-closure-and-regression-plan.md`
- `docs/dev-plans/250-go-gateway-rag-and-authz-phase1-2-plan.md`
- `docs/dev-plans/260-assistant-conversation-fsm-and-user-visible-contract-plan.md`
- `docs/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
