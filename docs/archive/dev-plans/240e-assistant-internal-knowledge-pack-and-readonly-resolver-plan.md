# DEV-PLAN-240E：Assistant 内部知识资产与上下文主链治理方案（承接 240-M6）

> 归档说明（2026-04-12）：本文件已自 `docs/dev-plans/` 迁入 `docs/archive/dev-plans/`，仅保留为历史参考，不再作为现行 SSOT。

**状态**: 规划中（2026-03-11 CST；本次修订将 `240E` 收敛为“知识治理主契约”，明确主源矩阵、资产模型、版本审计与停止线；运行时最小实现由 `DEV-PLAN-241` 承接）

## 1. 初衷与目的
1. [ ] `240E` 的唯一目标冻结为：**为 AI 助手建立知识与上下文主链的治理契约**，不承担 UI 承载、事务执行、耐久任务、外部协议接入或额外执行入口建设。
2. [ ] 本计划直接服务 `260` 的真实业务 Case 1~4，尤其是多轮补全、候选确认、提交说明、成功/失败回执与非动作分流所需的解释一致性。
3. [ ] 本计划要解决的核心问题不是“如何引入更多工具”，而是：
   - [ ] 哪些内容属于执行主源，哪些内容只属于解释/分流/表达；
   - [ ] AI 助手在 `understand / route / plan / confirm / commit / task / reply` 各阶段，允许消费哪些类型的知识；
   - [ ] 用户看到的澄清、补全提示、确认摘要、失败解释，如何与同一套知识主链保持一致；
   - [ ] 知识变更如何被版本化、审计化，并与 `request / trace / turn / task` 建立可复核关联。
4. [ ] 本计划的产出是**治理母法**：知识资产模型、主源矩阵、上下文注入边界、版本审计规则、停止线与实施分层。
5. [ ] `240E` 先完成契约冻结与样例设计，不以运行时落地阻塞 `271 -> 240F -> 285` 主路径；仅当知识层确认进入运行时主链时，才由 `241` 按本计划约束实施。

## 2. 背景与当前基线
1. [X] `260` 已冻结：业务 FSM、DTO 字段与阶段推进以后端为 SSOT；前端与运行态承载面不得重算 `phase / missing_fields / candidates / pending_draft_summary / selected_candidate_id / commit_reply / error_code`。
2. [X] `223` 已冻结：会话、turn、phase、审计状态转移与 DTO rebuild 必须来自持久化事实源，保证服务重启后 Case 1~4 仍能按相同语义继续推进。
3. [X] `240A~240D` 已把动作注册、风控拦截、耐久执行与人工接管主链收敛到本仓后端；因此知识层必须服务于现有动作主链，而不是平行定义另一套业务语义。
4. [ ] 当前仓内知识主要分散在多处：
   - [ ] skill 文档与计划文档；
   - [ ] 字段规则与错误码目录；
   - [ ] 候选展示与确认语义说明；
   - [ ] 相邻实现中的隐式约束与说明文字。
5. [ ] 当前缺口不是“没有知识”，而是“缺少面向运行时的结构化知识资产与受控注入边界”，导致知识难以冻结、难以裁剪、难以回放，也难以稳定指导用户反馈。
6. [ ] 当前若继续以“大而全知识包 + 自由上下文拼装”推进，最容易产生三类风险：
   - [ ] 与执行主链形成双主源；
   - [ ] `Resolver / Clarification` 膨胀为影子状态机；
   - [ ] 缺少版本与模板约束，造成证据不可复核。

## 3. 范围与知识分类（冻结）

### 3.1 静态知识
1. [ ] 指相对稳定、可版本冻结的说明性知识，例如：
   - [ ] 意图分类说明；
   - [ ] 字段展示名与补全引导；
   - [ ] 候选解释模板；
   - [ ] 确认摘要模板；
   - [ ] 成功/失败/等待/人工接管回复指导语义；
   - [ ] 非动作场景的澄清与回复模板。
2. [ ] 静态知识不直接执行，只用于指导 AI 助手如何解释、确认、总结和反馈。

### 3.2 动态知识
1. [ ] 指运行时按租户、对象、阶段变化的只读知识，例如：
   - [ ] 当前租户可见的组织信息；
   - [ ] 候选对象的解释性详情；
   - [ ] 当前阶段的缺字段、候选状态、错误码上下文；
   - [ ] 只读配置、启用状态、版本信息；
   - [ ] 错误解释所需的补充事实。
2. [ ] 动态知识只能通过只读查询获得，不能承担写职责。

### 3.3 执行知识
1. [ ] 指指导 AI 助手“什么时候该计划、什么时候该确认、什么时候该等待任务、什么时候该反馈结果”的正式运行知识。
2. [ ] 执行知识的正式主源仍是本仓既有动作主链，包括：
   - [ ] `AssistantActionSpec`；
   - [ ] `capability_key`；
   - [ ] `required_checks`；
   - [ ] `CommitAdapter`；
   - [ ] `task` 状态机；
   - [ ] DTO 字段语义；
   - [ ] 受控错误码契约。
3. [ ] 本计划不重新定义执行主链，只规定执行知识如何被只读投影、摘要、装配与反馈给 AI 助手。

### 3.4 意图知识
1. [ ] 指 AI 助手在动作前理解用户真实目的的规则知识，包括：
   - [ ] `intent_class` 分类：`business_action` / `knowledge_qa` / `chitchat` / `uncertain`；
   - [ ] `intent_id -> route_kind/action_id` 的映射；
   - [ ] 低置信度、混合意图、跨阶段输入时的澄清策略；
   - [ ] 明确不应触发业务动作的 `negative_patterns`。
2. [ ] 意图知识只负责“分流与澄清”，不直接推进 `confirm/commit`，不改 `260` 的业务 FSM。

## 4. 目标与非目标

### 4.1 核心目标
1. [ ] 建立清晰的**主源矩阵**，明确执行真相、意图分流、解释模板、动态事实、上下文模板分别由谁定义。
2. [ ] 建立结构化**知识资产模型**，避免把 skill/dev-plan 全文或页面 patch 文案直接塞给模型。
3. [ ] 建立显式的**Readonly Resolver 分层**，让动态只读知识以结构化结果进入 AI 助手。
4. [ ] 建立**Intent Route Catalog + Clarification Policy**，让 AI 助手先判断“是否为业务动作”，再决定是否进入动作链路。
5. [ ] 建立**Context Template Registry**，把各阶段允许注入模型的字段白名单、数量上限与字符预算冻结下来。
6. [ ] 建立**版本与审计规则**，确保影响 route / explain / reply 的知识变化都能与 `request / trace / turn / task` 关联复核。
7. [ ] 保持 `260/223/240A~240D` 不变量：知识层只服务既有主链，不新增执行入口，不改变 DTO 主语义，不形成第二事实源。

### 4.2 非目标
1. [ ] 不把知识层建设成通用知识平台、文档上传平台、向量检索平台或新协议中台。
2. [ ] 不在本计划内引入 Redis、外部向量数据库、外部知识 API、Node.js 第二主链或前端本地 helper 主链。
3. [ ] 不允许知识资产重定义 `required_fields / phase / confirm 条件 / commit 条件 / task 正式语义`。
4. [ ] 不允许 `Resolver / Router / Clarification` 覆盖正式 DTO 字段、隐式推进状态或承担写职责。
5. [ ] 不允许通过降低门禁、扩大覆盖率排除项或增加 fallback 双链路来掩盖知识设计缺陷。

## 5. 主源矩阵与知识资产模型（冻结）

### 5.1 主源矩阵（Source of Truth Matrix）
1. [ ] 执行主源：`ActionSpec / capability_key / required_checks / CommitAdapter / task FSM / DTO / error code contract`。
2. [ ] 分流主源：`Intent Route Catalog`。
3. [ ] 表达主源：`Interpretation Pack / Action View Pack / Reply Guidance Pack`。
4. [ ] 动态事实主源：`Resolver Result`（来自既有事实表与只读域服务）。
5. [ ] 注入主源：`Context Template Registry`。
6. [ ] 上述五类主源彼此不得越权：
   - [ ] 表达主源不得重定义执行主源；
   - [ ] 分流主源不得推进正式 FSM；
   - [ ] Resolver 不得发明新的业务真相；
   - [ ] 上下文模板不得自由拼装未注册字段。

### 5.2 知识资产总模型
1. [ ] `240E` 不再把所有知识塞进单个“大 Knowledge Pack”；改为以下四类工件：
   - [ ] `Interpretation Pack`：意图分类说明、澄清提示、负例、非动作场景解释；
   - [ ] `Action View Pack`：字段展示名、缺字段解释、候选解释模板、确认摘要模板；
   - [ ] `Reply Guidance Pack`：成功/失败/等待/人工接管等反馈指导；
   - [ ] `Intent Route Catalog`：`intent_id -> route_kind/action_id` 路由目录。
2. [ ] 四类工件都必须可版本冻结、可审阅、可审计引用。
3. [ ] 首期支持语言冻结为 `zh/en`，与仓库现行 i18n 约束一致。

### 5.3 Interpretation Pack
1. [ ] 用于 `understand / route / clarification` 阶段的意图解释与澄清。
2. [ ] 最小字段建议：
   - [ ] `pack_id`
   - [ ] `knowledge_version`
   - [ ] `locale`
   - [ ] `intent_classes[]`
   - [ ] `clarification_prompts[]`
   - [ ] `negative_examples[]`
   - [ ] `source_refs[]`
3. [ ] `Interpretation Pack` 不得声明业务必填字段、阶段推进条件或提交条件。

### 5.4 Action View Pack
1. [ ] 用于 `plan / confirm` 阶段的解释性展示，不承载执行真相。
2. [ ] 最小字段建议：
   - [ ] `action_id`
   - [ ] `knowledge_version`
   - [ ] `locale`
   - [ ] `summary`
   - [ ] `field_display_map[]`
   - [ ] `missing_field_guidance[]`
   - [ ] `field_examples[]`
   - [ ] `candidate_explanation_templates[]`
   - [ ] `confirmation_summary_templates[]`
   - [ ] `source_refs[]`
3. [ ] `Action View Pack` 不得包含 `required_fields` 真相字段；真正的必填集只能由执行主链通过只读投影提供。

### 5.5 Reply Guidance Pack
1. [ ] 用于 `reply` 阶段，把成功/失败/等待/人工接管等用户可见反馈统一到同一套知识主链。
2. [ ] 最小字段建议：
   - [ ] `reply_kind`
   - [ ] `knowledge_version`
   - [ ] `locale`
   - [ ] `guidance_templates[]`
   - [ ] `tone_constraints[]`
   - [ ] `negative_examples[]`
   - [ ] `source_refs[]`
3. [ ] `Reply Guidance Pack` 只指导表达方式，不得决定事实状态。

### 5.6 Intent Route Catalog
1. [ ] `Intent Route Catalog` 是独立工件，不再作为“大 Knowledge Pack” 的附属字段。
2. [ ] 最小字段建议：
   - [ ] `intent_id`
   - [ ] `route_kind`（`business_action` / `knowledge_qa` / `chitchat` / `uncertain`）
   - [ ] `action_id`（仅 `business_action` 必填）
   - [ ] `required_slots[]`
   - [ ] `min_confidence`
   - [ ] `clarification_template_id`
   - [ ] `route_catalog_version`
3. [ ] 约束：
   - [ ] `route_kind=business_action` 且置信度低于门槛时，必须先进入澄清；
   - [ ] `route_kind=knowledge_qa/chitchat/uncertain` 时，不得触发 `confirm/commit`；
   - [ ] 所有 route 决策必须可审计到 `tenant_id / conversation_id / turn_id / request_id / trace_id`。

### 5.7 source_refs 与可审阅性
1. [ ] `source_refs[]` 是审计索引，不是用户可见文案来源。
2. [ ] 每个知识资产至少有一个有效 `source_ref`，且只能指向仓内 SSOT 文档或正式契约代码位置。
3. [ ] 若某项知识无法在五分钟内被说明清楚并结构化表达，应视为知识设计仍不成熟，不得直接注入运行时。

## 6. Resolver、路由与上下文模板边界（冻结）

### 6.1 Readonly Resolver 分层
1. [ ] `Resolver` 只返回动态只读事实，绝不执行写操作。
2. [ ] `Resolver` 按责任边界分为：
   - [ ] `Conversation Resolver`：读取会话/turn/task 快照与当前阶段状态；
   - [ ] `Domain Fact Resolver`：读取租户内业务对象事实；
   - [ ] `Contract Resolver`：只读投影动作契约、字段展示约束与错误码映射；
   - [ ] `Error Catalog Resolver`：返回错误码解释所需的结构化说明。
3. [ ] `Resolver` 输出必须结构化，且能与 `tenant_id / conversation_id / turn_id / request_id / trace_id` 关联。
4. [ ] `Resolver` 只返回事实，不返回推荐决策，不生成新的 DTO，不覆盖正式 DTO 字段。

### 6.2 Intent Router 与 Clarification Policy
1. [ ] `Intent Router` 的唯一职责是输出结构化 route 决策：
   - [ ] `route_kind`
   - [ ] `intent_id`
   - [ ] `candidate_action_ids[]`
   - [ ] `confidence_band`
   - [ ] `clarification_required`
   - [ ] `reason_codes[]`
2. [ ] `Intent Router` 不得直接推进正式业务 `phase`，不得触发提交，不得改写 DTO。
3. [ ] `Clarification Policy` 只影响 route 决策，不影响正式 FSM；必须定义澄清轮次上限。
4. [ ] 连续澄清失败、低置信度未收敛或多意图持续冲突时，只能回到 `uncertain` 或给出人工提示，不得硬猜动作。

### 6.3 Context Template Registry
1. [ ] `Context Assembler` 不允许“按需自由拼装”，必须依赖 `Context Template Registry`。
2. [ ] 每个阶段只允许使用已注册模板，例如：
   - [ ] `route_context_v1`
   - [ ] `plan_context_v1`
   - [ ] `confirm_context_v1`
   - [ ] `reply_context_v1`
3. [ ] 每个模板必须冻结：
   - [ ] 允许字段白名单；
   - [ ] 最大候选数；
   - [ ] 最大字符预算；
   - [ ] 是否允许历史 turn 摘要；
   - [ ] 是否允许错误解释上下文。
4. [ ] 未注册模板、模板外字段或超预算拼装，均视为 fail-closed。

### 6.4 Reply Realizer
1. [ ] 用户可见反馈统一采用“结构化骨架 + 受控自然语言实现”两段式：
   - [ ] 后端主链先产出结构化反馈骨架；
   - [ ] `Reply Guidance Pack` 与模型只负责受控表达，不决定事实。
2. [ ] 回复层必须覆盖：澄清提问、缺字段提示、候选解释、确认摘要、提交成功、提交失败、任务等待、人工接管。
3. [ ] 回复层不得依赖页面 patch、本地模板或散落 helper 作为正式主链。

## 7. 版本、审计与契约编译
1. [ ] 以下版本字段在主计划层冻结：
   - [ ] `knowledge_snapshot_digest`
   - [ ] `route_catalog_version`
   - [ ] `resolver_contract_version`
   - [ ] `context_template_version`
   - [ ] `reply_guidance_version`
2. [ ] 任何影响 route / explain / reply 的知识变更，都必须能关联到 `request / trace / turn / task`，并能复算当次对话消费知识资产集合的 `knowledge_snapshot_digest`。
3. [ ] 新增 `Knowledge Contract Compiler` 概念，用于在启动期或构建期执行语义校验，至少覆盖：
   - [ ] `action_id` 必须已注册；
   - [ ] `intent_id` 必须唯一；
   - [ ] `route_kind` 必须合法；
   - [ ] 错误码引用必须存在；
   - [ ] locale 仅允许 `zh/en`；
   - [ ] 上下文模板只允许引用白名单字段；
   - [ ] 知识资产不得声明与执行主链冲突的必填/阶段/提交条件。
4. [ ] `DEV-PLAN-241` 只能实现 `240E` 已冻结的资产模型、主源矩阵、版本审计与模板边界；若需要新增知识资产类型或调整主源分配，必须先回写 `240E`。

## 8. 推荐架构
1. [ ] **Knowledge Asset Builder**：把内部文档、规则与说明提炼成四类知识资产。
2. [ ] **Knowledge Contract Compiler**：执行 schema + semantic 校验，阻断漂移资产进入运行时。
3. [ ] **Readonly Resolver Layer**：提供按租户、动作、阶段、对象查询的只读事实能力。
4. [ ] **Intent Router**：在 `plan` 前输出结构化 route 决策，区分业务动作与非动作意图。
5. [ ] **Clarification Policy**：在低置信度或多意图场景生成受控澄清问题，不改变正式 FSM。
6. [ ] **Context Template Registry**：冻结各阶段允许进入模型的上下文字段与预算。
7. [ ] **Reply Realizer**：把结构化反馈骨架转换为统一、受控的用户可见表达。
8. [ ] **Assistant 主链**：继续由既有 `ActionSpec / gate / task / adapter / DTO rebuild` 承接执行语义。

## 9. 对 `260` Case 1~4 的直接收益映射
1. [ ] **Case 1**：可先分流到 `knowledge_qa/chitchat/uncertain`，避免非动作输入误触发动作链路。
2. [ ] **Case 2**：意图已明确时，`Action View Pack + Contract Resolver` 可帮助生成更稳定的计划摘要与确认说明；正式 `create -> confirm -> commit` 语义仍以后端 FSM 为准。
3. [ ] **Case 3**：`Action View Pack + Contract Resolver` 能显著提升“缺字段解释”“补全引导”“补全后确认摘要”的一致性。
4. [ ] **Case 4**：`Domain Fact Resolver` 可补充候选解释与差异说明；若候选歧义与意图歧义叠加，优先进入澄清链路而非直接确认。
5. [ ] 以上收益全部属于“知识增强与意图分流增强”，不改变 `260` 已冻结的执行主链。

## 10. 直接实施拆分（按批次）

### 10.1 PR-240E-01：资产模型、主源矩阵与语义校验冻结
1. [ ] 目标：冻结四类知识资产、主源矩阵、`Context Template Registry` 与 `Knowledge Contract Compiler` 的语义边界。
2. [ ] DoD：
   - [ ] 明确 `240E` 是治理母法，`241` 是实施子计划；
   - [ ] 删除知识资产中的执行真相字段；
   - [ ] 明确 `Intent Route Catalog` 为独立工件；
   - [ ] 明确模板白名单与语义校验要求。

### 10.2 PR-240E-02：版本与审计规则冻结
1. [ ] 目标：冻结 `knowledge_snapshot_digest / route_catalog_version / resolver_contract_version / context_template_version / reply_guidance_version` 的口径。
2. [ ] DoD：
   - [ ] 影响 route / explain / reply 的知识变更都有版本字段；
   - [ ] 审计要求能关联 `request / trace / turn / task`；
   - [ ] 明确 `241` 必须先落快照再扩运行时接线。

### 10.3 PR-240E-03：对子计划 `241` 的实现授权与边界交接
1. [ ] 目标：把 `240E` 的治理契约下发给 `241`，仅允许 `241` 做最小运行时实现。
2. [ ] DoD：
   - [ ] `241` 首批只实现单动作、单阶段、单模板闭环；
   - [ ] 非动作样例作为一等公民纳入范围；
   - [ ] 若 `241` 需要超出 `240E` 的资产或主源定义，必须先回写主计划。

## 11. 测试与覆盖率
1. [ ] 覆盖率口径：沿用仓库当前 Go 测试与 CI 覆盖率门禁；新增知识层代码不得通过扩大排除项规避测试。
2. [ ] 统计范围：至少覆盖知识资产 schema、语义编译、Resolver 只读接口、意图分流、模板装配、租户边界、错误路径。
3. [ ] 关键场景：
   - [ ] 知识资产版本变化可检测；
   - [ ] `action_id / intent_id / 错误码 / template field` 的非法引用会被阻断；
   - [ ] Resolver 缺少租户上下文时 fail-closed；
   - [ ] `knowledge_qa/chitchat/uncertain` 不得误触发动作链路；
   - [ ] 低置信度与多意图输入必须触发澄清提示；
   - [ ] 上下文模板不会覆盖正式 DTO 主语义；
   - [ ] 用户反馈与主链事实源一致。

## 12. 停止线（Fail-Closed）
1. [ ] 若实现把原始 skill 文档或 dev-plan 全文直接注入运行时，替代结构化知识资产，则本计划失败。
2. [ ] 若知识资产声明了与执行主链冲突的 `required_fields / phase / confirm 条件 / commit 条件`，则本计划失败。
3. [ ] 若 Resolver 出现写副作用、隐式提交、跨租户读取或返回推荐决策，则本计划失败。
4. [ ] 若 `route_kind=knowledge_qa/chitchat/uncertain` 仍可直接进入动作 `confirm/commit`，则本计划失败。
5. [ ] 若低置信度或多意图输入未经过澄清即直接执行动作，则本计划失败。
6. [ ] 若 `Context Assembler` 存在未注册模板、自由拼字段或超预算装配，则本计划失败。
7. [ ] 若用户反馈仍主要依赖页面 patch、本地模板或运行态散落逻辑，而非统一知识主链，则本计划失败。
8. [ ] 若知识变化无法映射到版本字段并与 `request / trace / turn / task` 建立审计关联，则本计划失败。

## 13. 验收标准
1. [ ] 主计划层已冻结四类知识资产模型、主源矩阵、版本审计规则与模板白名单边界。
2. [ ] `Intent Route Catalog` 已作为独立工件建模，且非动作输入不会误入动作提交链。
3. [ ] `Knowledge Contract Compiler` 的最小语义校验范围已明确，能阻断非法资产进入运行时。
4. [ ] `241` 的实施边界已与 `240E` 对齐：先快照、后接线；先单动作单阶段、后扩面。
5. [ ] 整个知识治理方案不引入新外部工具链前置条件，不改变 `240A~240D` 的正式执行主链。

## 14. 门禁与 SSOT 引用
1. [ ] 文档与实现触发器以 `AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 为准。
2. [ ] 本计划一旦进入代码实现，至少命中以下门禁：
   - [ ] `go fmt ./... && go vet ./... && make check lint && make test`
   - [ ] `make check no-legacy`
   - [ ] `make check error-message`
   - [ ] `make check doc`
3. [ ] 若实现触达 Assistant 路由、能力映射或错误码契约，还需按实际命中情况补跑对应门禁；本文不复制脚本细节，以 SSOT 为准。

## 15. 交付物
1. [ ] 本计划文档：`docs/archive/dev-plans/240e-assistant-internal-knowledge-pack-and-readonly-resolver-plan.md`
2. [ ] 子计划：`docs/archive/dev-plans/241-assistant-knowledge-pack-runtime-minimal-implementation-plan.md`
3. [ ] 后续执行记录未单列沉淀（未形成 `dev-plan-240e-execution-log.md`）。
4. [ ] 后续实现产物（待 `241` 实施时落地）：
   - [ ] 四类知识资产 schema 与样例；
   - [ ] `Knowledge Contract Compiler` 语义校验器；
   - [ ] `Context Template Registry` 模板契约；
   - [ ] 相关测试、快照与证据索引。

## 16. 关联文档
- `docs/archive/dev-plans/240-assistant-org-transaction-orchestration-modernization-plan.md`
- `docs/archive/dev-plans/241-assistant-knowledge-pack-runtime-minimal-implementation-plan.md`
- `docs/archive/dev-plans/240f-assistant-280-aligned-closure-and-regression-plan.md`
- `docs/archive/dev-plans/250-go-gateway-rag-and-authz-phase1-2-plan.md`
- `docs/archive/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/archive/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
