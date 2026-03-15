# DEV-PLAN-268：Assistant 外部大模型单一语义核与本地最小执行边界实施计划

**状态**: 规划中（2026-03-14 22:52 CST）

## 1. 背景与上下文
1. [ ] `DEV-PLAN-267` 已明确：当前 Assistant 的核心问题不是某一层单点失效，而是 `intent -> route -> clarification -> phase -> reply` 多层链路同时保留判断权，导致整体慢、硬、重复、容易误导。
2. [ ] 当前 runtime 试图在本地维护一套缩小版对话大脑：
   - [ ] 本地 intent overlay / fallback；
   - [ ] 本地 route decision；
   - [ ] 本地 clarification decision；
   - [ ] 本地 reply kind 与 reply 二次改写。
3. [ ] 上述结构在“工程可控”层面有收益，但在真实对话体验上形成系统性副作用：
   - [ ] 多轮重复判断同一件事；
   - [ ] 任一层保守降级都会把整体拖入 `uncertain/non_business_route`；
   - [ ] 额外模型调用与本地状态机共同拉长响应链路。
4. [ ] 本计划的立场是：在当前窄域业务条件下，应更大胆地把语义理解、上下文修复、下一问生成与用户可见回复交给外部大模型；本地系统只保留事实源、只读检索、执行边界和写入主链。

## 2. 目标与非目标
1. [ ] 核心目标：以单一外部大模型语义核替代本地 `intent/route/clarification/reply` 多层主决策链。
2. [ ] 核心目标：把本地 runtime 收缩为最小职责集合：`Context Assembler`、`Readonly Resolver`、`Dry Run`、`Action Gate`、`Confirm Gate`、`Commit Adapter`、`Audit`。
3. [ ] 核心目标：删除或降级 `plan_only`、本地 fallback extractor、本地 route 重判、本地 clarification 重判、独立 reply 润色调用。
4. [ ] 核心目标：建立“模型请求检索 -> 本地执行检索 -> 结构化回填 -> 模型收敛”的最小双回合能力，默认单轮最多两次模型调用。
5. [ ] 核心目标：保留并强化 One Door、`confirm/commit` 严格 gate、OCC、幂等、审计与 fail-closed 执行边界。
6. [ ] 核心目标：将体验验收从“结构正确”升级为“任务完成效率、错误归因、领域忠实度、时延预算”。
7. [ ] 非目标：本计划不引入新的数据库表、外部缓存、向量检索平台、MCP 第二主链或第二写入口。
8. [ ] 非目标：本计划不允许模型直接决定 commit 放行，不允许模型直接生成可写事实 ID 或绕过 ActionSpec。
9. [ ] 非目标：本计划不保留“模型失败后自动切回旧本地理解链”的兜底方案。

## 3. 目标架构
1. [ ] 单轮主链收敛为：
   - [ ] 用户输入；
   - [ ] 本地 `Context Assembler` 组装最小上下文；
   - [ ] 外部模型输出结构化 `Conversation Semantic State`；
   - [ ] 如模型请求检索，则本地 `Readonly Resolver` 执行并回填结果；
   - [ ] 必要时进行第二次模型收敛；
   - [ ] 本地执行 `Dry Run/Gate/Confirm/Commit`；
   - [ ] 将结构化状态投影为 turn 审计字段与用户可见结果。
2. [ ] 外部模型应成为以下能力的唯一主语义源：
   - [ ] 用户目标理解；
   - [ ] 动作候选选择；
   - [ ] 槽位补全与上下文修复；
   - [ ] 是否需要检索、检索什么；
   - [ ] 当前最合适的下一问；
   - [ ] 当前给用户看的回复文本。
3. [ ] 本地 runtime 只保留以下确定性职责：
   - [ ] 提供租户隔离、会话历史摘要、允许动作白名单、业务约束、候选检索结果；
   - [ ] 解析并校验模型输出 JSON；
   - [ ] 执行只读检索；
   - [ ] 执行 dry-run、风控、鉴权、确认与提交；
   - [ ] 维护审计、回执、任务与状态投影。
4. [ ] `phase/reason_codes/reply_kind` 不再充当主控制中心，只允许作为审计或 UI 投影存在。

## 4. 结构化语义契约
1. [ ] 新的外部模型主输出至少应包含以下字段：
   - [ ] `goal_summary`：当前用户目标摘要；
   - [ ] `proposed_action`：候选动作 ID；
   - [ ] `slots`：当前已收集槽位；
   - [ ] `retrieval_requests[]`：模型请求本地执行的检索动作；
   - [ ] `retrieval_needed`：是否需要本地回填事实；
   - [ ] `next_question`：若尚未可执行，下一句应问什么；
   - [ ] `user_visible_reply`：当前直接给用户的文本；
   - [ ] `readiness`：如 `need_more_info / ready_for_dry_run / ready_for_confirm`；
   - [ ] `confidence_note`：仅用于审计，不直接对用户暴露。
2. [ ] 本地回填给模型的检索结果至少应包含以下状态：
   - [ ] `not_requested`；
   - [ ] `deferred_by_boundary`；
   - [ ] `no_match`；
   - [ ] `multiple_matches`；
   - [ ] `single_match`。
3. [ ] 模型输出必须满足以下边界：
   - [ ] 不得把 `org_id/candidate_id/version_tuple` 等可写事实 ID 视为权威值；
   - [ ] 不得直接声明“已提交”；
   - [ ] 不得越过本地 ActionSpec、Authz、Risk、DryRun、Confirm、Commit Gate。
4. [ ] 本地系统对模型输出只做受控接受：
   - [ ] schema 不通过则失败；
   - [ ] `action / route_kind / intent_id` 必须一次完整返回，缺失任一字段即 fail-closed；
   - [ ] 动作不在 allowlist 则失败；
   - [ ] 引用不存在的候选事实则失败；
   - [ ] ready 只是建议，最终是否可 confirm/commit 由本地 gate 决定。
5. [ ] 本地语义修正只保留最小确定性边界：
   - [ ] 仅允许保留日期类硬约束（显式日期优先、幻觉日期清空、续轮日期不误清）；
   - [ ] 不再对父组织名、实体名、动作类型进行本地 overlay / fallback / 二次猜测。

## 5. 本地职责收口与删改范围
1. [ ] 保留并强化的本地能力：
   - [ ] `Action Registry / ActionSpec`；
   - [ ] `Readonly Resolver`；
   - [ ] `Dry Run`；
   - [ ] `Action Interceptor`；
   - [ ] `Confirm Gate`；
   - [ ] `Commit Adapter + OCC + One Door`；
   - [ ] `Task/Audit`。
2. [ ] 需要删除或降级为投影的本地能力：
   - [ ] 本地 intent fallback / overlay 主判断链；
   - [ ] 以 `route_kind` 为中心的业务/非业务主分流；
   - [ ] 以 clarification kind 为中心的表单式多状态机；
   - [ ] reply kind 选择与 reply 二次模型改写主链；
   - [ ] `plan_only` 作为伪动作参与主链判断。
3. [ ] 需要保留但角色改变的能力：
   - [ ] `knowledge runtime` 从“本地路由中心”转为“上下文资产装配来源”；
   - [ ] `phase` 从“流程控制器”转为“审计和 UI 展示字段”；
   - [ ] `missing_fields/candidates/error_code` 从“多层决策输入”转为“结构化执行结果”。

## 6. 实施步骤
### 6.1 M1：冻结单一语义核契约与双回合预算
1. [ ] 定义新的 `Conversation Semantic State` 契约，覆盖 `goal/proposed_action/slots/retrieval_requests/next_question/user_visible_reply/readiness`。
2. [ ] 冻结“默认单轮最多两次模型调用”的预算：
   - [ ] 第一次：语义决策；
   - [ ] 第二次：仅在本地检索回填后允许的收敛调用。
3. [ ] 明确取消独立 reply 润色调用，不再把 reply 作为单独模型链路。
4. [ ] 明确模型失败路径：显式失败或人工接管，不回退旧本地理解链。
5. [ ] 明确取消“同一用户输入因语义不完整而在 pipeline 再请求一次模型”的本地重试。

### 6.2 M2：实现 Context Assembler 与模型主编排器
1. [ ] 新增或收敛单一语义编排器，负责：
   - [ ] 组装最小上下文；
   - [ ] 调用外部模型；
   - [ ] 解析结构化结果；
   - [ ] 根据 `retrieval_requests` 触发本地检索；
   - [ ] 进行必要的第二次模型收敛。
2. [ ] `Context Assembler` 只提供模型真正需要的上下文：
   - [ ] 租户隔离信息；
   - [ ] 会话必要摘要；
   - [ ] 当前允许动作与动作约束；
   - [ ] 当前已收集的确定性事实；
   - [ ] 检索结果与检索状态；
   - [ ] confirm/commit 边界说明。
3. [ ] 禁止把内部流程控制字段原样堆给模型作为主语义输入，避免把旧状态机重新包一层继续存在。

### 6.3 M3：收口本地多层判断链
1. [ ] 删除或旁路 `intent -> route -> clarification -> reply` 作为主链的调用路径。
2. [ ] `assistant_intent_router`、`assistant_clarification_policy`、`assistant_reply_nlg` 仅允许保留投影、兼容读或过渡期桥接职责，禁止继续承担主判断。
3. [ ] 退役 `plan_only` 在正式业务主链中的语义地位。
4. [ ] 停止本地 overlay/fallback 改写外部模型语义结果。

### 6.4 M4：检索与错误归因修复
1. [ ] 本地检索不再硬绑定“所有 intent 校验都已通过”这一前提。
2. [ ] 检索输出必须稳定区分：
   - [ ] 未请求；
   - [ ] 被边界暂缓；
   - [ ] 无结果；
   - [ ] 多结果；
   - [ ] 唯一结果。
3. [ ] 用户提示必须与上述状态一一对应，禁止“未执行却提示未匹配”。
4. [ ] 候选检索策略可在本计划内收敛到更宽容但可解释的 alias/synonym/name match，不引入新基础设施。

### 6.5 M5：执行边界硬化与 confirm/commit 收口
1. [ ] 继续以 `ActionSpec` 作为本地动作白名单和安全主源。
2. [ ] 模型输出的 `ready_for_confirm` 或等价字段，只能触发本地 dry-run 和 confirm summary 生成，不得直接提交。
3. [ ] confirm 仍必须校验 `plan_hash`、TTL、候选事实与当前上下文一致性。
4. [ ] commit 仍必须经由受控 `Commit Adapter`、OCC、One Door 和任务审计。
5. [ ] 本地需要保证：即使模型输出看似正确，只要 dry-run/gate 不通过，系统也必须 fail-closed。

### 6.6 M6：体验验收与旧链路移除
1. [ ] 建立与 `246B` 并列的体验型验收：
   - [ ] 用户平均轮次；
   - [ ] 错误归因准确率；
   - [ ] 领域漂移率；
   - [ ] 单轮模型调用次数；
   - [ ] 平均响应时延；
   - [ ] 真实任务完成率。
2. [ ] 明确过渡完成后应删除或长期禁用的旧链路入口，避免双链路继续共存。
3. [ ] 若旧链路仍需短期桥接，必须限定为只读/审计/兼容投影，且设置明确移除 stopline。

## 7. 代码影响范围
1. [ ] 主要改动预计集中在：
   - [ ] `internal/server/assistant_api.go`
   - [ ] `internal/server/assistant_model_gateway.go`
   - [ ] `internal/server/assistant_reply_model_gateway.go`
   - [ ] `internal/server/assistant_intent_pipeline.go`
   - [ ] `internal/server/assistant_intent_router.go`
   - [ ] `internal/server/assistant_clarification_policy.go`
   - [ ] `internal/server/assistant_reply_nlg.go`
   - [ ] `internal/server/assistant_knowledge_runtime.go`
   - [ ] `internal/server/orgunit_nodes.go`
2. [ ] 允许新增少量收口文件，例如：
   - [ ] `assistant_semantic_state.go`
   - [ ] `assistant_semantic_orchestrator.go`
   - [ ] `assistant_context_assembler.go`
3. [ ] 除非后续实施被证明绝对必要，否则本计划不新增数据库表；优先复用现有 turn/conversation 结构承载新的结构化语义结果与审计投影。

## 8. 风险与停止线
1. [ ] 若实施后仍保留两套正式主脑：外部模型一套、本地 route/clarification/reply 一套，则本计划失败。
2. [ ] 若模型失败后回退旧本地理解链，而不是显式失败或人工接管，则本计划失败。
3. [ ] 若 reply 仍作为独立模型润色调用长期存在，则本计划失败。
4. [ ] 若检索状态仍无法区分“未执行”和“无匹配”，则本计划失败。
5. [ ] 若模型可直接绕过 dry-run、confirm、commit gate，则本计划失败。
6. [ ] 若为适配新方案引入 legacy、双链路或第二写入口，则本计划失败。
7. [ ] 若默认单轮超过两次模型调用，且无明确收益证据，则本计划失败。
8. [ ] 若本地上下文装配继续无节制膨胀，把旧状态机原样打包给模型，则本计划失败。

## 9. 测试与覆盖率
1. [ ] 覆盖率口径、统计范围、目标阈值与证据记录继续以 `AGENTS.md`、`Makefile` 与 CI workflow 为 SSOT；本计划不复制脚本细节。
2. [ ] 命中 Assistant runtime、model gateway、resolver、reply、routing 的代码改动，必须补充最小回归测试与体验型 case。
3. [ ] 体验型验收至少覆盖：
   - [ ] 轻微自然语言变体不应误降级；
   - [ ] 相对日期与常见日期格式可由单一模型语义核稳定处理；
   - [ ] 组织别名/同义称谓可恢复到候选检索；
   - [ ] 候选检索状态区分准确；
   - [ ] reply 不跨域；
   - [ ] 错误归因准确；
   - [ ] 单轮模型调用次数符合预算；
   - [ ] 模型失败不会触发旧本地理解链回退；
   - [ ] confirm/commit 边界仍然 fail-closed。

## 10. 交付物
1. [ ] 本计划文档：`docs/dev-plans/268-assistant-external-llm-semantic-core-and-runtime-thinning-implementation-plan.md`
2. [ ] 单一语义核契约与编排实现
3. [ ] 本地多层判断链裁剪结果
4. [ ] 检索状态与错误归因修复
5. [ ] 体验型验收记录与执行证据

## 11. 关联文档
1. [ ] `docs/dev-plans/240-assistant-org-transaction-orchestration-modernization-plan.md`
2. [ ] `docs/dev-plans/240e-assistant-internal-knowledge-pack-and-readonly-resolver-plan.md`
3. [ ] `docs/dev-plans/241-assistant-knowledge-pack-runtime-minimal-implementation-plan.md`
4. [ ] `docs/dev-plans/242-assistant-intent-router-runtime-minimal-plan.md`
5. [ ] `docs/dev-plans/243-assistant-clarification-policy-and-slot-repair-plan.md`
6. [ ] `docs/dev-plans/244-assistant-interpretation-pack-and-intent-route-catalog-compiler-plan.md`
7. [ ] `docs/dev-plans/245-assistant-reply-guidance-pack-and-reply-realizer-plan.md`
8. [ ] `docs/dev-plans/246-assistant-understand-route-clarify-roadmap.md`
9. [ ] `docs/dev-plans/267-assistant-dialog-rigidity-retrospective-and-architecture-correction-plan.md`
