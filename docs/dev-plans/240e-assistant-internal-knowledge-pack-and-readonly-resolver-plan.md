# DEV-PLAN-240E：Assistant 内部知识包与只读 Resolver 方案（承接 240-M6）

**状态**: 规划中（2026-03-09 20:25 CST；经重评后彻底重写：本计划的唯一目标是为 AI 助手建立知识与上下文主链；当前阶段先完成契约冻结与范围收敛，暂不以其运行时实现作为 `271-S5`、`240F` 或 `285` 的启动前置；若后续进入运行时主链并形成影响性合入，再按 `271-S5` 证据新鲜度规则补跑）

## 1. 初衷与目的
1. [ ] `240E` 的初衷与目的已冻结为：**只为 AI 助手提供知识和上下文能力**，不承担 UI 承载、事务执行、耐久任务、外部协议接入或额外执行入口建设。
2. [ ] 本计划服务的直接对象，是 `260` 的真实业务 Case 2~4：多轮补全、候选确认、提交说明、成功/失败回执，都需要稳定、可解释、可版本冻结的知识支持。
3. [ ] 本计划要解决的核心问题不是“如何引入更多工具”，而是：
   - [ ] 业务规则、字段说明、候选解释、skill 文档、错误解释等知识，应该以什么形式进入 AI 助手；
   - [ ] AI 助手在 `understand / route / plan / confirm / commit / task / reply` 各阶段，应如何获取和使用这些知识；
   - [ ] 用户看到的提示、说明、确认摘要、失败解释，应如何与同一套知识主链保持一致。
4. [ ] 本计划的落地目标，是建立一条**内部知识与上下文主链**：知识整理 → 结构化打包 → 只读查询 → 意图判断/分流 → 上下文装配 → 动作指导 → 用户反馈。
5. [ ] 当前路线图定位冻结为：`240E` 先完成契约与样例设计，不以运行时落地阻塞 `271 -> 240F -> 285` 主路径；仅当后续确认知识层需要进入运行时主链时，才按本计划的 `PR-240E-01~03` 顺序实施。

## 2. 背景与当前基线
1. [X] `260` 已冻结：业务 FSM、DTO 字段与阶段推进以后端为 SSOT；前端与运行态承载面不得重算 `phase / missing_fields / candidates / pending_draft_summary / selected_candidate_id / commit_reply / error_code`。
2. [X] `223` 已冻结：会话、turn、phase、审计状态转移与 DTO rebuild 必须来自持久化事实源，保证服务重启后 Case 2~4 仍能按相同语义继续推进。
3. [X] `240A~240D` 已把动作注册、风控拦截、耐久执行与人工接管主链收敛到本仓后端；因此知识层必须服务于现有动作主链，而不是平行定义另一套业务语义。
4. [ ] 当前仓内知识主要分散在多处：
   - [ ] skill 文档与计划文档；
   - [ ] 字段规则与错误码目录；
   - [ ] 候选展示与确认语义说明；
   - [ ] 相邻实现中的隐式约束与说明文字。
5. [ ] 当前缺口不是“没有知识”，而是“缺少面向运行时的结构化知识主链”，导致知识难以冻结、难以裁剪、难以回放，也难以稳定指导用户反馈。
6. [ ] 当前 `240E` 文案默认从“已识别 action”开始，缺少“用户输入先做意图分流”的前置契约；这会导致闲聊、知识问答、半结构化动作表达都被迫进入同一动作假设。

## 3. 范围与知识分类（冻结）

### 3.1 静态知识
1. [ ] 指相对稳定、可版本冻结的说明性知识，例如：
   - [ ] 动作说明；
   - [ ] 字段定义；
   - [ ] 缺字段补全引导规则；
   - [ ] 候选解释规则；
   - [ ] 成功/失败反馈指导语义；
   - [ ] skill 文档摘要。
2. [ ] 静态知识不直接执行，只用于指导 AI 助手如何解释、确认、总结和反馈。

### 3.2 动态知识
1. [ ] 指运行时按租户、对象、阶段变化的只读知识，例如：
   - [ ] 当前租户可见的组织信息；
   - [ ] 候选对象的解释性详情；
   - [ ] 字段约束与当前状态；
   - [ ] 只读配置、启用状态、版本信息；
   - [ ] 错误解释所需的上下文细节。
2. [ ] 动态知识只能通过只读查询获得，不能承担写职责。

### 3.3 执行知识
1. [ ] 指指导 AI 助手“什么时候该计划、什么时候该确认、什么时候该等待任务、什么时候该反馈结果”的运行知识。
2. [ ] 执行知识的正式主源仍是本仓既有动作主链，包括：
   - [ ] `AssistantActionSpec`
   - [ ] `capability_key`
   - [ ] `required_checks`
   - [ ] `CommitAdapter`
   - [ ] `task` 状态机
   - [ ] DTO 字段语义
3. [ ] 本计划不重新定义执行主链，只规定这些执行知识如何被读取、摘要、装配与反馈给 AI 助手。

### 3.4 意图知识（新增冻结）
1. [ ] 指 AI 助手在动作前理解用户真实目的的规则知识，包括：
   - [ ] `intent_class` 分类：`business_action` / `knowledge_qa` / `chitchat` / `uncertain`；
   - [ ] `intent_to_action` 映射：`intent_id -> action_id`（可为空）；
   - [ ] `clarification_policy`：低置信度、混合意图、跨阶段输入时的澄清策略；
   - [ ] `negative_patterns`：明确不应触发业务动作的表达模式。
2. [ ] 意图知识只负责“分流与澄清”，不直接推进 `confirm/commit`，不改 `260` 的业务 FSM。

## 4. 目标与非目标

### 4.1 核心目标
1. [ ] 建立 `Internal Knowledge Pack`：把内部知识收敛为结构化、可裁剪、可版本冻结的知识包，而不是把原始文档全文直接塞给模型。
2. [ ] 建立 `Readonly Resolver`：按 action / phase / tenant / conversation / turn 提供动态只读知识查询能力。
3. [ ] 建立 `Intent Router + Clarification Policy`：让 AI 助手先判断“是否为业务动作”，再决定是否进入动作链路。
4. [ ] 建立 `Context Assembler`：把静态知识、动态知识、执行知识、意图知识按阶段装配成 AI 助手可消费的最小上下文。
5. [ ] 建立 `Reply Guidance` 口径：确保 AI 助手对用户的澄清提问、补全提示、确认摘要、候选解释、成功回执、失败说明都来自同一套知识主链。
6. [ ] 保持 `260/223/240D` 不变量：知识层只服务既有主链，不新增执行入口，不改变 DTO 主语义，不形成第二事实源。

### 4.2 非目标
1. [ ] 不在本计划内重定义 `260` 的业务 FSM。
2. [ ] 不在本计划内改变 `223` 的事实源模式。
3. [ ] 不在本计划内改变 `240D` 的耐久执行与人工接管主链。
4. [ ] 不在本计划内建设新的 UI 承载面。
5. [ ] 不在本计划内扩展工具生态或建立新的外部协议主链。

## 5. 知识形式与主源规则

### 5.1 Internal Knowledge Pack（静态知识包）
1. [ ] 知识包按 `action_id` 和可选 `phase` 组织，不按“整篇文档全文”组织。
2. [ ] 每个知识包至少包含：
   - [ ] `action_id`
   - [ ] `knowledge_version`
   - [ ] `summary`
   - [ ] `intent_classes[]`
   - [ ] `intent_to_action_rules[]`
   - [ ] `required_fields`
   - [ ] `clarification_prompts[]`
   - [ ] `confirmation_rules`
   - [ ] `candidate_explanation_rules`
   - [ ] `success_reply_guidance`
   - [ ] `failure_reply_guidance`
   - [ ] `negative_examples[]`
   - [ ] `source_refs[]`
3. [ ] `source_refs[]` 只作为审计与调试索引，不直接作为用户可见主文案来源。
4. [ ] 运行时只允许消费结构化知识包，不允许以原始 skill 文档或原始 dev-plan 全文直接替代知识包。

### 5.2 Skill 文档等知识形式的规定
1. [ ] `skill` 文档、规则文档、设计文档可以作为**人类可维护来源**，但不直接等于运行时知识。
2. [ ] 这些文档进入运行时前，必须先被提炼为结构化知识包或 Resolver 可消费的规则片段。
3. [ ] 文档的职责是描述意图与约束；知识包的职责是为运行时提供稳定、紧凑、可冻结的知识输入。
4. [ ] 若某项知识无法在五分钟内被说明清楚并结构化表达，应视为知识设计仍不成熟，不得直接注入运行时。

### 5.3 Readonly Resolver（动态只读查询）
1. [ ] Resolver 的唯一职责是返回动态只读知识，绝不执行写操作。
2. [ ] Resolver 输出必须是结构化结果，且能与 `tenant_id / conversation_id / turn_id / request_id / trace_id` 关联。
3. [ ] 当前最小建议能力：
   - [ ] `candidate_detail_resolver`
   - [ ] `field_constraint_resolver`
   - [ ] `tenant_visible_orgunit_resolver`
   - [ ] `conversation_intent_context_resolver`
   - [ ] `intent_disambiguation_resolver`
   - [ ] `error_explanation_resolver`
4. [ ] Resolver 结果只能作为 Planner/Reply 的辅助上下文，不得直接覆盖正式 DTO 字段，不得替代事实源。

### 5.4 Intent Route Catalog（意图到动作映射）
1. [ ] 路由主键为 `intent_id`，最小映射结构为：
   - [ ] `route_kind`（`business_action` / `knowledge_qa` / `chitchat` / `uncertain`）
   - [ ] `action_id`（仅 `business_action` 必填）
   - [ ] `required_slots[]`
   - [ ] `min_confidence`
   - [ ] `clarification_template_id`
2. [ ] `route_kind=business_action` 且置信度低于门槛时，必须先进入澄清，不得直接进入动作 `plan`。
3. [ ] `route_kind=knowledge_qa/chitchat` 时，不得触发 `confirm/commit` 路径。
4. [ ] 所有 route 决策必须可审计到 `tenant_id / conversation_id / turn_id / request_id / trace_id`。

## 6. AI 助手如何获取知识与判断意图（按阶段冻结）

### 6.1 意图判断与分流阶段（understand/route）
1. [ ] AI 助手在进入动作计划前，应先获取：
   - [ ] 意图分类与映射规则（来自知识包）；
   - [ ] 会话上下文（上一 turn 的 phase/缺字段/候选状态）；
   - [ ] 当前阶段允许动作集合与禁止动作集合；
   - [ ] 澄清模板与确认提示规则。
2. [ ] 该阶段输出必须结构化，至少包含：
   - [ ] `route_kind`
   - [ ] `intent_id`
   - [ ] `candidate_action_ids[]`
   - [ ] `confidence_band`
   - [ ] `clarification_required`
   - [ ] `reason_codes[]`
3. [ ] 分流阶段的目标是：
   - [ ] 先判断“是否要触发业务动作”，避免闲聊误入动作链路；
   - [ ] 对混合/模糊输入给出可解释澄清；
   - [ ] 只有在 `business_action` 且满足阈值时才进入 `plan`。

### 6.2 计划阶段（plan）
1. [ ] 仅当 `6.1` 输出 `route_kind=business_action` 且无需澄清时，AI 助手才进入计划阶段，并获取：
   - [ ] 当前动作对应的静态知识包；
   - [ ] 必要的字段定义与补全规则；
   - [ ] 当前阶段的执行知识摘要。
2. [ ] 计划阶段的知识目标是：
   - [ ] 将已判断意图稳定映射到 `action_id/capability_key`；
   - [ ] 判断是否缺字段；
   - [ ] 形成清晰的下一步引导；
   - [ ] 生成对用户友好的计划摘要。

### 6.3 确认阶段（confirm）
1. [ ] AI 助手在要求用户确认前，应获取：
   - [ ] 确认规则；
   - [ ] 候选解释规则；
   - [ ] 当前已补全字段摘要；
   - [ ] 当前候选项解释性详情（如有）。
2. [ ] 确认阶段的知识目标是：
   - [ ] 让用户清楚知道“将要做什么”；
   - [ ] 让用户理解“当前候选项是谁、为什么是它”；
   - [ ] 避免含糊或不完整的确认提示。

### 6.4 提交与任务阶段（commit/task）
1. [ ] AI 助手在提交与等待任务结果时，应获取：
   - [ ] 提交前摘要指导语义；
   - [ ] 任务进行中反馈指导语义；
   - [ ] 成功/失败/转人工接管的反馈规则；
   - [ ] 错误解释 Resolver 返回的结构化结果（如有）。
2. [ ] 提交与任务阶段的知识目标是：
   - [ ] 准确告诉用户当前是在“执行中”“已成功”“需要继续补充”还是“需要人工接管”；
   - [ ] 避免用不稳定、本地拼接或与事实源不一致的话术冒充真实状态。

### 6.5 回复阶段（reply）
1. [ ] AI 助手所有面对用户的业务反馈，都必须从同一套知识主链获得指导，而不是由前端 helper、页面 patch 或零散模板各自生成。
2. [ ] 回复阶段应覆盖的最小类型：
   - [ ] 意图澄清提问（低置信度/多意图）；
   - [ ] 缺字段提示；
   - [ ] 候选解释；
   - [ ] 确认摘要；
   - [ ] 提交成功回执；
   - [ ] 提交失败说明；
   - [ ] 任务等待 / 稍后查看提示；
   - [ ] 人工接管提示。

## 7. 推荐架构
1. [ ] **Knowledge Pack Builder**：把内部文档、规则与说明提炼成结构化知识包。
2. [ ] **Readonly Resolver Layer**：提供按租户、动作、阶段、对象查询的只读知识能力。
3. [ ] **Intent Router**：在 `plan` 前输出结构化 route 决策，区分业务动作与非动作意图。
4. [ ] **Clarification Planner**：在低置信度或多意图场景生成受控澄清问题，并记录澄清轮次。
5. [ ] **Context Assembler**：在 `understand/route/plan/confirm/commit/task/reply` 前按需装配最小上下文。
6. [ ] **Assistant 主链**：继续由既有 `ActionSpec / gate / task / adapter / DTO rebuild` 承接执行语义。
7. [ ] **用户反馈**：只消费由主链 + 知识层共同生成的受控上下文，不依赖页面外挂提示或本地拼接补丁。

## 8. 对 `260` Case 1~4 的直接收益映射
1. [ ] **Case 1**：可先分流到 `knowledge_qa/chitchat`，避免非动作输入误触发动作链路。
2. [ ] **Case 2**：意图已明确时，知识包可帮助生成更稳定的“准备提交摘要”和确认说明；正式 `create -> confirm -> commit` 语义仍以后端 FSM 为准。
3. [ ] **Case 3**：知识包 + Resolver 能显著提升“缺字段解释”“补全引导”“补全后确认摘要”的质量与一致性。
4. [ ] **Case 4**：Resolver 可补充候选解释与差异说明；若候选歧义与意图歧义叠加，优先进入澄清链路而非直接确认。
5. [ ] 以上收益全部属于“知识增强与意图分流增强”，不改变 `260` 已冻结的执行主链。

## 9. 直接实施拆分（按批次）

### 9.1 PR-240E-01：Knowledge Pack 契约与最小样例
1. [ ] 目标：定义知识包结构、版本语义与 `intent_id/action_id/phase` 映射规则。
2. [ ] DoD：
   - [ ] 至少为 `org.orgunit_create` 形成一个最小知识包样例；
   - [ ] 至少给出一个 `knowledge_qa/chitchat` 的非动作样例；
   - [ ] 明确其与 skill 文档、错误码、DTO 语义的关系；
   - [ ] 不改变现有执行主链。

### 9.2 PR-240E-02：Readonly Resolver 契约与最小实现
1. [ ] 目标：定义只读 Resolver 接口与审计字段，覆盖“会话意图上下文”与“候选解释/字段约束”中的最小组合。
2. [ ] DoD：
   - [ ] Resolver 结果结构化；
   - [ ] Resolver 带租户边界；
   - [ ] Resolver 不承担写职责；
   - [ ] 可输出 route 所需的上下文片段与澄清建议；
   - [ ] 与 `223/240D` 的 request/trace 口径可关联。

### 9.3 PR-240E-03：上下文装配与反馈指导收口
1. [ ] 目标：明确知识包与 Resolver 结果如何在 `understand/route/plan/confirm/commit/task/reply` 注入 AI 助手，尤其是如何统一指导用户反馈。
2. [ ] DoD：
   - [ ] `understand/route/plan/confirm/commit/task/reply` 各阶段的上下文输入边界清晰；
   - [ ] 低置信度与多意图场景必须先澄清后执行；
   - [ ] 用户可见反馈来源统一；
   - [ ] 不形成第二主源。

## 10. 测试与覆盖率
1. [ ] 覆盖率口径：沿用仓库当前 Go 测试与 CI 覆盖率门禁；新增知识层代码不得通过扩大排除项规避测试。
2. [ ] 统计范围：至少覆盖知识包构建、Resolver 只读接口、意图分流、上下文装配、租户边界、错误路径。
3. [ ] 关键场景：
   - [ ] 知识包版本变化可检测；
   - [ ] Resolver 缺少租户上下文时 fail-closed；
   - [ ] Resolver 不得返回写入口信息或执行副作用；
   - [ ] 闲聊/知识问答输入不得误触发动作链路；
   - [ ] 低置信度与多意图输入必须触发澄清提示；
   - [ ] 上下文装配不会覆盖正式 DTO 主语义；
   - [ ] 用户反馈与主链事实源一致。

## 11. 停止线（Fail-Closed）
1. [ ] 若实现把原始 skill 文档全文直接注入运行时，替代结构化知识包，则本计划失败。
2. [ ] 若 Resolver 出现写副作用、隐式提交或跨租户读取，则本计划失败。
3. [ ] 若知识层尝试覆盖或重定义 `260` 的正式 DTO 字段语义，则本计划失败。
4. [ ] 若 `route_kind=knowledge_qa/chitchat/uncertain` 仍可直接进入动作 `confirm/commit`，则本计划失败。
5. [ ] 若低置信度或多意图输入未经过澄清即直接执行动作，则本计划失败。
6. [ ] 若用户反馈仍主要依赖页面 patch、本地模板或运行态散落逻辑，而非统一知识主链，则本计划失败。
7. [ ] 若知识层变化无法与 request/trace/turn/task 口径关联审计，则本计划失败。

## 12. 验收标准
1. [ ] 形成一份明确的“内部知识包 + 只读 Resolver”契约文档，且口径与 `260/223/240D` 一致。
2. [ ] 明确静态知识、动态知识、执行知识、意图知识四层分工，不再混用。
3. [ ] 明确 skill 文档等人类可读来源与运行时知识包的关系，避免“文档即运行时输入”的漂移。
4. [ ] 明确 AI 助手在 `understand/route/plan/confirm/commit/task/reply` 阶段如何获取知识、分流意图、指导动作与反馈用户。
5. [ ] 明确“意图分流 -> 动作映射 -> 低置信度澄清 -> 用户确认”的 fail-closed 口径。
6. [ ] 为 `260` Case 1~4 提供稳定、可解释、可版本冻结的知识增强路径，同时不破坏执行主链与事实源不变量。

## 13. 交付物
1. [ ] 本计划文档。
2. [ ] 后续若批准实施：知识包契约样例、Resolver 契约样例、上下文装配说明、相邻测试与执行记录。
3. [ ] 后续运行时最小实施由 `DEV-PLAN-241` 承接：`docs/dev-plans/241-assistant-knowledge-pack-runtime-minimal-implementation-plan.md`。
4. [ ] 若知识层进入运行时主链并影响 `271-S5` 证据结论：对应证据刷新记录与索引。

## 14. 门禁与 SSOT 引用
1. [ ] 仓库级 Go / 文档 / CI 门禁入口以 `AGENTS.md`、`Makefile`、`.github/workflows/quality-gates.yml` 为准。
2. [ ] 本计划相关门禁：`doc`、`assistant-config-single-source`、`no-legacy`、`capability-key`、`capability-route-map`。
3. [ ] 若知识层最终进入运行时主链并影响 `260/288/290/291` 语义结论，必须按 `271-S5` 规则评估证据新鲜度。

## 15. 关联文档
- `docs/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
- `docs/dev-plans/240-assistant-org-transaction-orchestration-modernization-plan.md`
- `docs/dev-plans/240c-assistant-action-interceptor-and-risk-gate-plan.md`
- `docs/dev-plans/240d-assistant-durable-execution-and-manual-takeover-plan.md`
- `docs/dev-plans/241-assistant-knowledge-pack-runtime-minimal-implementation-plan.md`
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `docs/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md`
- `docs/dev-plans/285-librechat-cutover-regression-and-closure-plan.md`
- `AGENTS.md`
