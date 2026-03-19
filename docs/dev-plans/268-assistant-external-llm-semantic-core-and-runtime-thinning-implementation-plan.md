# DEV-PLAN-268：Assistant 外部大模型单一语义核与本地最小执行边界实施计划

**状态**: 已完成（2026-03-16 08:32 CST；单一外部模型语义核、`Context Assembler` / `Semantic Orchestrator`、`/reply` 投影化与检索状态收口已落地；`go test ./internal/server/...`、`go vet ./...`、`make check lint`、`make test` 已通过，覆盖率门禁命中 `100.00%`）

## 0. 当前进展（2026-03-16）
1. [x] 已完成单一语义核主链切换：
   - [x] `prepareTurnDraft(...)` 统一经由 `orchestrateSemanticTurn(...)` 驱动；
   - [x] 新增 `assistant_semantic_contract.go`、`assistant_context_assembler.go`、`assistant_semantic_orchestrator.go` 作为代码侧 SSOT；
   - [x] 本地 `intent/clarification/reply` 不再承担主语义判断。
2. [x] 已完成 `/reply` 独立模型链路退役：
   - [x] `/turns/:id:reply` 仅返回已存 semantic snapshot、本地 projection 或 fail-closed fallback；
   - [x] reply 不再走独立润色模型主链；
   - [x] 旧 reply 错误映射死分支已删除。
3. [x] 已完成检索状态与错误归因收口：
   - [x] `retrieval_results[]` 稳定收敛为 `not_requested / deferred_by_boundary / no_match / multiple_matches / single_match`；
   - [x] dry-run / semantic state / 用户提示已对齐“未请求”和“无匹配”；
   - [x] 已删除 normalize 后再次判空 retrieval 状态的不可达死分支。
4. [x] 已完成验证闭环：
   - [x] `go fmt ./...`；
   - [x] `go test ./internal/server/...`；
   - [x] `go vet ./...`；
   - [x] `make check lint`；
   - [x] `make test`（覆盖率门禁 `100.00%`）。

## 1. 背景与上下文
1. [x] `DEV-PLAN-267` 已明确：当前 Assistant 的核心问题不是某一层单点失效，而是 `intent -> route -> clarification -> phase -> reply` 多层链路同时保留判断权，导致整体慢、硬、重复、容易误导。
2. [x] 当前 runtime 试图在本地维护一套缩小版对话大脑：
   - [x] 本地 intent overlay / fallback；
   - [x] 本地 route decision；
   - [x] 本地 clarification decision；
   - [x] 本地 reply kind 与 reply 二次改写。
3. [x] 上述结构在“工程可控”层面有收益，但在真实对话体验上形成系统性副作用：
   - [x] 多轮重复判断同一件事；
   - [x] 任一层保守降级都会把整体拖入 `uncertain/non_business_route`；
   - [x] 额外模型调用与本地状态机共同拉长响应链路。
4. [x] 本计划的立场是：在当前窄域业务条件下，应更大胆地把语义理解、上下文修复、下一问生成与用户可见回复交给外部大模型；本地系统只保留事实源、只读检索、执行边界和写入主链。

## 2. 目标与非目标
1. [x] 核心目标：以单一外部大模型语义核替代本地 `intent/route/clarification/reply` 多层主决策链。
2. [x] 核心目标：把本地 runtime 收缩为最小职责集合：`Context Assembler`、`Readonly Resolver`、`Dry Run`、`Action Gate`、`Confirm Gate`、`Commit Adapter`、`Audit`。
3. [x] 核心目标：删除或降级 `plan_only`、本地 fallback extractor、本地 route 重判、本地 clarification 重判、独立 reply 润色调用。
4. [x] 核心目标：建立“模型请求检索 -> 本地执行检索 -> 结构化回填 -> 模型收敛”的最小双回合能力，默认单轮最多两次模型调用。
5. [x] 核心目标：保留并强化 One Door、`confirm/commit` 严格 gate、OCC、幂等、审计与 fail-closed 执行边界。
6. [x] 核心目标：将体验验收从“结构正确”升级为“任务完成效率、错误归因、领域忠实度、时延预算”。
7. [x] 非目标：本计划不引入新的数据库表、外部缓存、向量检索平台、MCP 第二主链或第二写入口。
8. [x] 非目标：本计划不允许模型直接决定 commit 放行，不允许模型直接生成可写事实 ID 或绕过 ActionSpec。
9. [x] 非目标：本计划不保留“模型失败后自动切回旧本地理解链”的兜底方案。

## 3. 目标架构
1. [x] 单轮主链收敛为：
   - [x] 用户输入；
   - [x] 本地 `Context Assembler` 组装最小上下文；
   - [x] 外部模型输出结构化 `Conversation Semantic State`；
   - [x] 如模型请求检索，则本地 `Readonly Resolver` 执行并回填结果；
   - [x] 必要时进行第二次模型收敛；
   - [x] 本地执行 `Dry Run/Gate/Confirm/Commit`；
   - [x] 将结构化状态投影为 turn 审计字段与用户可见结果。
2. [x] 外部模型应成为以下能力的唯一主语义源：
   - [x] 用户目标理解；
   - [x] 动作候选选择；
   - [x] 槽位补全与上下文修复；
   - [x] 是否需要检索、检索什么；
   - [x] 当前最合适的下一问；
   - [x] 当前给用户看的回复文本。
3. [x] 本地 runtime 只保留以下确定性职责：
   - [x] 提供租户隔离、会话历史摘要、允许动作白名单、业务约束、候选检索结果；
   - [x] 解析并校验模型输出 JSON；
   - [x] 执行只读检索；
   - [x] 执行 dry-run、风控、鉴权、确认与提交；
   - [x] 维护审计、回执、任务与状态投影。
4. [x] `phase/reason_codes/reply_kind` 不再充当主控制中心，只允许作为审计或 UI 投影存在。

## 4. 结构化语义契约
1. [x] 新的外部模型主输出至少应包含以下字段：
   - [x] `goal_summary`：当前用户目标摘要；
   - [x] `proposed_action`：候选动作 ID；
   - [x] `slots`：当前已收集槽位；
   - [x] `retrieval_requests[]`：模型请求本地执行的检索动作；
   - [x] `retrieval_needed`：是否需要本地回填事实；
   - [x] `next_question`：若尚未可执行，下一句应问什么；
   - [x] `user_visible_reply`：当前直接给用户的文本；
   - [x] `readiness`：如 `need_more_info / ready_for_dry_run / ready_for_confirm`；
   - [x] `confidence_note`：仅用于审计，不直接对用户暴露。
2. [x] 当前实现冻结为以下最小语义契约（代码侧 SSOT 以 `assistant_semantic_contract.go` 为准）：
   - [x] 顶层动作字段继续保留 `action / intent_id / route_kind / route_catalog_version`，避免本地再做第二次 route 判定；
   - [x] `slots` 只承载业务槽位：`parent_ref_text / entity_name / effective_date / org_code / target_effective_date / new_name / new_parent_ref_text`；
   - [x] `retrieval_requests[]` 当前只允许最小读链路：`kind=candidate_lookup`，并显式标注 `slot / ref_text / as_of / limit`；
   - [x] `retrieval_results[]` 由本地回填，状态冻结为 `not_requested / deferred_by_boundary / no_match / multiple_matches / single_match`；
   - [x] `user_visible_reply` 与 `next_question` 二选一或同时存在，但不得再依赖独立 reply 润色链补写；
   - [x] `selected_candidate_id` 仅作为候选选择建议，本地仍需校验候选存在性与上下文一致性。
3. [x] 本地回填给模型的检索结果至少应包含以下状态：
   - [x] `not_requested`；
   - [x] `deferred_by_boundary`；
   - [x] `no_match`；
   - [x] `multiple_matches`；
   - [x] `single_match`。
4. [x] 模型输出必须满足以下边界：
   - [x] 不得把 `org_id/candidate_id/version_tuple` 等可写事实 ID 视为权威值；
   - [x] 不得直接声明“已提交”；
   - [x] 不得越过本地 ActionSpec、Authz、Risk、DryRun、Confirm、Commit Gate。
5. [x] 本地系统对模型输出只做受控接受：
   - [x] schema 不通过则失败；
   - [x] `action / route_kind / intent_id` 必须一次完整返回，缺失任一字段即 fail-closed；
   - [x] 动作不在 allowlist 则失败；
   - [x] 引用不存在的候选事实则失败；
   - [x] ready 只是建议，最终是否可 confirm/commit 由本地 gate 决定。
6. [x] 本地语义修正只保留最小确定性边界：
   - [x] 仅允许保留日期类硬约束（显式日期优先、幻觉日期清空、续轮日期不误清）；
   - [x] 不再对父组织名、实体名、动作类型进行本地 overlay / fallback / 二次猜测。
7. [x] 本地不得继续保留正则式意图抽取/槽位补全链：
   - [x] 删除本地 `intent extractor` 与基于用户原始文本的字段正则补写；
   - [x] clarification 续轮不再本地解析自由文本去补日期/名称/组织字段；
   - [x] pending turn 只作为上下文装配输入交给模型，不再充当本地补槽解析器；
   - [x] 不再把 pending turn 的 `action/slot` 在本地 merge 回新一轮 intent；
   - [x] synthetic/deterministic provider 也不得根据 pending turn 在本地升级动作或补齐语义。

## 5. 本地职责收口与删改范围
1. [x] 保留并强化的本地能力：
   - [x] `Action Registry / ActionSpec`；
   - [x] `Readonly Resolver`；
   - [x] `Dry Run`；
   - [x] `Action Interceptor`；
   - [x] `Confirm Gate`；
   - [x] `Commit Adapter + OCC + One Door`；
   - [x] `Task/Audit`。
2. [x] 需要删除或降级为投影的本地能力：
   - [x] 本地 intent fallback / overlay 主判断链；
   - [x] 以 `route_kind` 为中心的业务/非业务主分流；
   - [x] 以 clarification kind 为中心的表单式多状态机；
   - [x] reply kind 选择与 reply 二次模型改写主链；
   - [x] `plan_only` 作为伪动作参与主链判断。
3. [x] 需要保留但角色改变的能力：
   - [x] `knowledge runtime` 从“本地路由中心”转为“上下文资产装配来源”；
   - [x] `phase` 从“流程控制器”转为“审计和 UI 展示字段”；
   - [x] `missing_fields/candidates/error_code` 从“多层决策输入”转为“结构化执行结果”。

## 6. 实施步骤
### 6.1 M1：冻结单一语义核契约与双回合预算
1. [x] 定义新的 `Conversation Semantic State` 契约，覆盖 `goal/proposed_action/slots/retrieval_requests/next_question/user_visible_reply/readiness`。
2. [x] 冻结“默认单轮最多两次模型调用”的预算：
   - [x] 第一次：语义决策；
   - [x] 第二次：仅在本地检索回填后允许的收敛调用。
3. [x] 明确取消独立 reply 润色调用，不再把 reply 作为单独模型链路。
4. [x] 明确模型失败路径：显式失败或人工接管，不回退旧本地理解链。
5. [x] 明确取消“同一用户输入因语义不完整而在 pipeline 再请求一次模型”的本地重试。

### 6.2 M2：实现 Context Assembler 与模型主编排器
1. [x] 新增或收敛单一语义编排器，负责：
   - [x] 组装最小上下文；
   - [x] 调用外部模型；
   - [x] 解析结构化结果；
   - [x] 根据 `retrieval_requests` 触发本地检索；
   - [x] 进行必要的第二次模型收敛。
   - [x] 当前文件落点冻结为 `internal/server/assistant_context_assembler.go` 与 `internal/server/assistant_semantic_orchestrator.go`。
2. [x] `Context Assembler` 只提供模型真正需要的上下文：
   - [x] 租户隔离信息；
   - [x] 会话必要摘要；
   - [x] 当前允许动作与动作约束；
   - [x] 当前已收集的确定性事实；
   - [x] 检索结果与检索状态；
   - [x] confirm/commit 边界说明。
   - [x] 过渡期内若为兼容旧测试夹具而需要从已确认槽位推导最小 `candidate_lookup` 请求，该推导也必须在编排器内单点完成，禁止散落到 route/clarification/reply 链。
3. [x] 禁止把内部流程控制字段原样堆给模型作为主语义输入，避免把旧状态机重新包一层继续存在。

### 6.3 M3：收口本地多层判断链
1. [x] 删除或旁路 `intent -> route -> clarification -> reply` 作为主链的调用路径。
2. [x] `assistant_intent_router`、`assistant_clarification_policy`、`assistant_reply_nlg` 仅允许保留投影、兼容读或过渡期桥接职责，禁止继续承担主判断。
3. [x] 退役 `plan_only` 在正式业务主链中的语义地位。
4. [x] 停止本地 overlay/fallback 改写外部模型语义结果。

### 6.4 M4：检索与错误归因修复
1. [x] 本地检索不再硬绑定“所有 intent 校验都已通过”这一前提。
2. [x] 检索输出必须稳定区分：
   - [x] 未请求；
   - [x] 被边界暂缓；
   - [x] 无结果；
   - [x] 多结果；
   - [x] 唯一结果。
3. [x] 用户提示必须与上述状态一一对应，禁止“未执行却提示未匹配”。
4. [x] 候选检索策略保持现有可解释的 lookup 语义，不引入新基础设施。

### 6.5 M5：执行边界硬化与 confirm/commit 收口
1. [x] 继续以 `ActionSpec` 作为本地动作白名单和安全主源。
2. [x] 模型输出的 `ready_for_confirm` 或等价字段，只能触发本地 dry-run 和 confirm summary 生成，不得直接提交。
3. [x] confirm 仍必须校验 `plan_hash`、TTL、候选事实与当前上下文一致性。
4. [x] commit 仍必须经由受控 `Commit Adapter`、OCC、One Door 和任务审计。
5. [x] 本地需要保证：即使模型输出看似正确，只要 dry-run/gate 不通过，系统也必须 fail-closed。

### 6.6 M6：体验验收与旧链路移除
1. [x] 建立与 `246B` 并列的体验型验收：
   - [x] 用户平均轮次；
   - [x] 错误归因准确率；
   - [x] 领域漂移率；
   - [x] 单轮模型调用次数；
   - [x] 平均响应时延；
   - [x] 真实任务完成率。
2. [x] 明确过渡完成后应删除或长期禁用的旧链路入口，避免双链路继续共存。
   - [x] `/turns/:id:reply` 不再触发独立 reply 模型调用，只允许返回 semantic state 投影或本地 fail-closed fallback；
   - [x] `assistant_clarification_policy` 不再主导槽位补全，只保留 phase/UI/audit 投影；
   - [x] `assistant_intent_router` 不再重判业务语义，只保留 route execution boundary 校验与审计快照。
3. [x] 若旧链路仍需短期桥接，必须限定为只读/审计/兼容投影，且设置明确移除 stopline。

## 7. 代码影响范围
1. [x] 本次实际改动集中在：
   - [x] `internal/server/assistant_api.go`
   - [x] `internal/server/assistant_model_gateway.go`
   - [x] `internal/server/assistant_intent_pipeline.go`
   - [x] `internal/server/assistant_reply_nlg.go`
   - [x] `internal/server/assistant_semantic_state.go`
2. [x] 本次新增收口文件：
   - [x] `internal/server/assistant_semantic_contract.go`
   - [x] `internal/server/assistant_semantic_orchestrator.go`
   - [x] `internal/server/assistant_context_assembler.go`
   - [x] `internal/server/assistant_268_semantic_closure_coverage_test.go`
3. [x] 未新增数据库表；继续复用现有 turn/conversation 结构承载新的结构化语义结果与审计投影。

## 8. 风险与停止线
1. [x] 若实施后仍保留两套正式主脑：外部模型一套、本地 route/clarification/reply 一套，则本计划失败。
2. [x] 若模型失败后回退旧本地理解链，而不是显式失败或人工接管，则本计划失败。
3. [x] 若 reply 仍作为独立模型润色调用长期存在，则本计划失败。
4. [x] 若检索状态仍无法区分“未执行”和“无匹配”，则本计划失败。
5. [x] 若模型可直接绕过 dry-run、confirm、commit gate，则本计划失败。
6. [x] 若为适配新方案引入 legacy、双链路或第二写入口，则本计划失败。
7. [x] 若默认单轮超过两次模型调用，且无明确收益证据，则本计划失败。
8. [x] 若本地上下文装配继续无节制膨胀，把旧状态机原样打包给模型，则本计划失败。

## 9. 测试与覆盖率
1. [x] 覆盖率口径、统计范围、目标阈值与证据记录继续以 `AGENTS.md`、`Makefile` 与 CI workflow 为 SSOT；本计划不复制脚本细节。
2. [x] 命中 Assistant runtime、model gateway、resolver、reply、routing 的代码改动，已补充最小回归测试与体验型 case。
3. [x] 体验型验收至少覆盖：
   - [x] 轻微自然语言变体不应误降级；
   - [x] 相对日期与常见日期格式可由单一模型语义核稳定处理；
   - [x] 组织别名/同义称谓可恢复到候选检索；
   - [x] 候选检索状态区分准确；
   - [x] reply 不跨域；
   - [x] 错误归因准确；
   - [x] 单轮模型调用次数符合预算；
   - [x] 模型失败不会触发旧本地理解链回退；
   - [x] confirm/commit 边界仍然 fail-closed。

## 10. 交付物
1. [x] 本计划文档：`docs/dev-plans/268-assistant-external-llm-semantic-core-and-runtime-thinning-implementation-plan.md`
2. [x] 单一语义核契约与编排实现
3. [x] 本地多层判断链裁剪结果
4. [x] 检索状态与错误归因修复
5. [x] 体验型验收记录与执行证据
6. [x] 执行日志：`docs/dev-records/dev-plan-268-execution-log.md`

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
