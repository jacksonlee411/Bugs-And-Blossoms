# DEV-PLAN-245：Assistant Reply Guidance Pack 与 Reply Realizer 计划

**状态**: 规划中（2026-03-11 CST；本次修订将 `245` 细化为“可按文件直接实施”的 reply 表达统一蓝图，承接 `240E` 主计划与 `246` 阶段 E，并保持“事实仍由 turn/task/DTO 决定、reply 只负责表达与解释”的目标不变）

## 1. 背景与定位
1. [ ] `240E` 已冻结：Assistant 的知识主链必须区分“执行真相”和“解释表达”；reply 资产只能负责**用户可见表达、解释、总结、引导**，不得重算业务事实或推进状态机。
2. [ ] `246` 已冻结阶段顺序：`245` 位于 `244` 之后的阶段 E，前提是：
   - [ ] `241` 已提供最小知识快照、Resolver 与 `reply_guidance_version` 审计口径；
   - [ ] `242` 已提供动作 / 非动作 / 待澄清的正式 route 出口；
   - [ ] `243` 已把澄清、缺字段、候选确认统一为结构化主链；
   - [ ] `244` 已把理解层资产治理扩面完成，不再让 reply 继续依赖散乱 prompt/规则。
3. [ ] 当前仓内 reply 相关能力并非空白，而是处于“可跑但尚未统一”的阶段：
   - [ ] `internal/server/assistant_reply_nlg.go` 已具备 `renderTurnReply(...)`、`assistantReplyStage(...)`、`assistantReplyKind(...)`、`assistantReplyFallbackText(...)` 等链路；
   - [ ] `internal/server/assistant_reply_model_gateway.go` 已具备 reply 模型网关调用；
   - [ ] `internal/server/assistant_knowledge_runtime.go` 已能载入 `reply_guidance/*.json` 并生成 `ReplyGuidanceVersion`；
   - [ ] `internal/server/assistant_knowledge/reply_guidance/` 下已存在 `missing_fields.zh/en.json` 样例；
   - [ ] `assistant_turns.commit_reply`、`reply_nlg`、`reply_guidance_version` 等字段已具备部分承载能力。
4. [ ] 但当前仍不能视为“reply 主链已知识化完成”，因为：
   - [ ] `assistant_reply_nlg.go` 仍主要靠 `assistantReplyFallbackText(...)` 里的硬编码字符串拼接用户可见文本；
   - [ ] `assistant_api.go`、`assistant_phase_snapshot.go`、`assistant_reply_nlg.go` 之间仍存在表达语义分散；
   - [ ] reply 资产目前只有 `missing_fields` 一类样例，无法覆盖澄清、候选、确认摘要、成功/失败回执、任务等待与人工接管；
   - [ ] 还没有正式的 `Reply Realizer` 边界，把“机器状态 → 受控表达骨架 → 最终文本”分成稳定层次。
5. [ ] 本计划定位冻结为：**在不改写执行主链、不回退到页面本地 helper 拼文案的前提下，把 Assistant 用户可见反馈统一收口到 `Reply Guidance Pack + Reply Realizer` 主链**。

## 2. 当前实现基线、缺口与根因

### 2.1 当前实际 reply 调用链
1. [ ] 当前 reply 路径大致为：
   - [ ] API 进入 `assistant_api.go` 的 `reply` 分支；
   - [ ] `renderTurnReply(...)` 读取 conversation/turn；
   - [ ] `assistantReplyOutcome(...)`、`assistantReplyStage(...)`、`assistantReplyKind(...)` 推导当前回复分类；
   - [ ] `assistantReplyFallbackText(...)` 生成 fallback 文本；
   - [ ] `assistantRenderReplyWithModel(...)` 通过 model gateway 产出文本；
   - [ ] `assistantSetReplySnapshot(...)` 与 `persistRenderedReply(...)` 回写 turn 快照。
2. [ ] 当前链路说明 reply 已是独立入口，但仍偏“模型提示 + 本地兜底”模式，而不是“知识资产 + realizer”模式。

### 2.2 当前直接风险
1. [ ] 用户可见表达仍主要散落在 `assistantReplyFallbackText(...)`，导致：
   - [ ] 同一事实在不同 stage 可能出现不同措辞；
   - [ ] 澄清、缺字段、候选、失败解释容易继续各自扩 helper；
   - [ ] 与 `243` 的正式 `Clarification` 主链容易脱节。
2. [ ] 现有 reply 阶段仅覆盖 `draft / missing_fields / candidate_list / candidate_confirm / commit_result / commit_failed`，尚未正式吸纳：
   - [ ] `await_clarification`；
   - [ ] route 非动作解释；
   - [ ] task `queued/running` 等等待表达；
   - [ ] `manual_takeover_required` 的人工接管表达。
3. [ ] `assistantReplyMachineState` 当前字段较少，仍不足以稳定支持：
   - [ ] clarification prompt 引用；
   - [ ] route 决策摘要；
   - [ ] task 状态摘要；
   - [ ] 失败解释所需的结构化上下文。
4. [ ] 当前仅有 `missing_fields` 的 reply 资产样例，说明 reply 资产尚未成为主链，只是“存在一份样例文件”。

### 2.3 根因到文件的映射
1. [ ] `internal/server/assistant_reply_nlg.go`
   - [ ] 同时承担 stage 判定、fallback 组装、模型 prompt 输入、快照持久化，多职责耦合；
   - [ ] 缺少“reply input builder / guidance lookup / realizer / fallback policy”的显式分层。
2. [ ] `internal/server/assistant_knowledge_runtime.go`
   - [ ] 已能编译 `reply_guidance` 资产，但缺少 `findReplyGuidance(...)` 这类 runtime 消费接口；
   - [ ] 对 `reply_kind`、`template_id`、error code 绑定关系的治理仍偏最小化。
3. [ ] `internal/server/assistant_knowledge/reply_guidance/*.json`
   - [ ] 覆盖场景太少，尚不足以支撑统一表达主链。
4. [ ] `internal/server/assistant_api.go` / `internal/server/assistant_phase_snapshot.go`
   - [ ] 已持久化 `ReplyNLG / CommitReply / ReplyGuidanceVersion`，但还没有由单一 realizer 保证这些快照来自统一表达策略。

## 3. 目标与非目标

### 3.1 核心目标
1. [ ] 建立 `Reply Guidance Pack` 正式资产模型，覆盖 Assistant 用户可见反馈的最小核心场景。
2. [ ] 建立 `Reply Realizer` 运行时边界：输入只接受结构化 machine state / resolver facts / guidance pack，输出受控用户文本与审计信息。
3. [ ] 把澄清、缺字段、候选解释、确认摘要、成功/失败回执、等待、人工接管统一收口到知识主链，而不是继续扩散到 helper 字符串。
4. [ ] 确保 reply 文本与 turn/task/DTO 事实严格一致，不得伪造成功、伪造可提交状态、伪造候选或隐藏失败原因。
5. [ ] 为 `zh/en` 两种 locale 提供稳定、可回放、可审计的资产化表达路径。

### 3.2 非目标
1. [ ] 不在本计划中改写 `242` 的 route 决策或 `243` 的 clarification builder；reply 只消费其结构化输出。
2. [ ] 不在本计划中引入新的外部回复协议、额外 UI 主链或前端本地语言生成器。
3. [ ] 不允许 `Reply Guidance Pack` 定义 `phase / missing_fields / candidate_list / commit_result / task_status` 真相；这些事实必须来自 turn/task/DTO/Resolver。
4. [ ] 不在本计划中把 reply 变成新的“执行控制器”；reply 不能改写 turn state、不能推进 confirm/commit、不能代替 task 状态机。
5. [ ] 不通过“扩大 fallback 常量集合”来伪装完成知识化；fallback 只能作为受控兜底，不得重新成为主入口。

## 4. 与 240E / 241 / 242 / 243 / 244 / 246 的边界冻结
1. [ ] 与 `240E` 的关系：`245` 只实现 reply 表达侧的知识主链，不新增知识类型、不改变主源矩阵；若要改资产类别或审计口径，必须先回写 `240E`。
2. [ ] 与 `241` 的关系：`245` 必须复用 `ReplyGuidanceVersion`、`knowledge_snapshot_digest`、Resolver 与快照口径，不得另起一套 reply 版本字段。
3. [ ] 与 `242` 的关系：`245` 只能消费 `route_kind / intent_id / reason_codes / clarification_required / knowledge_snapshot_digest` 等 route 结果，不得自己重新判断是否为动作请求。
4. [ ] 与 `243` 的关系：`245` 必须把 `Clarification` 作为唯一澄清事实源；澄清文案只可表达 `243` 已裁决出的 `kind / reason_codes / prompt_template_id / current_round / exit_to`，不得自己造第二套追问逻辑。
5. [ ] 与 `244` 的关系：`244` 负责理解层资产与 prompt 清退，`245` 负责用户可见表达统一；`245` 必须遵守 `244` 对资产引用、版本冻结与散点清退的约束。
6. [ ] 与 `246` 的关系：`245` 必须后置；若 `243/244` 尚未冻结澄清与理解资产边界，不得提前进行大规模 reply 主链改写。

## 5. Reply 资产契约（冻结）

### 5.1 `Reply Guidance Pack` 字段冻结
1. [ ] 资产字段冻结为：
   - [ ] `asset_type = reply_guidance_pack`
   - [ ] `reply_kind`
   - [ ] `knowledge_version`
   - [ ] `locale`
   - [ ] `guidance_templates[]`
   - [ ] `tone_constraints[]`
   - [ ] `negative_examples[]`
   - [ ] `error_codes[]`
   - [ ] `source_refs[]`
2. [ ] `guidance_templates[]` 元素结构冻结为：
   - [ ] `template_id`
   - [ ] `text`
3. [ ] 当前资产模型不新增 `priority / condition DSL / free-form metadata` 等字段；若未来需要扩展，必须先回写主计划。

### 5.2 `reply_kind` 范围冻结
1. [ ] 首批正式 `reply_kind` 至少覆盖：
   - [ ] `clarification_required`
   - [ ] `missing_fields`
   - [ ] `candidate_list`
   - [ ] `candidate_confirm`
   - [ ] `confirm_summary`
   - [ ] `commit_success`
   - [ ] `commit_failed`
   - [ ] `task_waiting`
   - [ ] `manual_takeover`
   - [ ] `non_business_route`
2. [ ] `draft` 可以保留为过渡期 stage，但不建议作为长期 reply 主语义；长期应由更明确的 `confirm_summary / task_waiting / non_business_route` 等 kind 替代。
3. [ ] `reply_kind` 只描述用户应看到的反馈类型，不描述内部阶段推进规则。

### 5.3 `guidance_templates[]` 语义冻结
1. [ ] 单个 pack 内 `template_id` 必须唯一。
2. [ ] `text` 允许使用受控模板变量，但变量集合必须来自 `Reply Realizer` 注册白名单，禁止运行时自由插值。
3. [ ] 模板文本不得：
   - [ ] 透传技术错误码、trace/request id、schema/runtime/provider 内部信息；
   - [ ] 承诺未发生的提交结果；
   - [ ] 表示“我已帮你执行成功”而事实尚未成功；
   - [ ] 诱导用户绕过必填字段或候选确认。

### 5.4 `tone_constraints[]` 与 `negative_examples[]`
1. [ ] `tone_constraints[]` 仅用于限制语气风格，如“明确、简洁、可执行”，不得编码业务判断。
2. [ ] `negative_examples[]` 只用于约束不应出现的表达，不得把它写成另一套逻辑分支。
3. [ ] `negative_examples[]` 至少应覆盖：
   - [ ] 技术术语直出；
   - [ ] 模糊承诺；
   - [ ] 要求用户“自己猜/随便填”；
   - [ ] 与当前事实冲突的示例。

### 5.5 `error_codes[]` 与 `source_refs[]`
1. [ ] `error_codes[]` 仅允许引用仓内已知、可解释的受控错误码；不得塞入技术底层异常字符串。
2. [ ] 同一 `reply_kind` 可以按 locale 拥有多份资产，但其语义必须一致。
3. [ ] `source_refs[]` 必须全部指向仓内有效路径，至少一个引用应直达：
   - [ ] `240E/243/244/245/246` 之一；或
   - [ ] 对应实现文件；或
   - [ ] 对应错误码/契约来源。

## 6. 运行时模型与 Realizer 边界（冻结）

### 6.1 Reply Realizer 的职责
1. [ ] `Reply Realizer` 的唯一职责是：**把结构化事实转成受控用户表达**。
2. [ ] 它不做：
   - [ ] route 决策；
   - [ ] clarification 裁决；
   - [ ] dry-run 校验真相计算；
   - [ ] task 状态推进；
   - [ ] API 错误码映射。
3. [ ] 它要做：
   - [ ] 选择 `reply_kind`；
   - [ ] 装配 reply 输入骨架；
   - [ ] 查找 `Reply Guidance Pack`；
   - [ ] 渲染模板或构造模型提示；
   - [ ] 执行受控 fallback；
   - [ ] 输出用户文本与审计快照。

### 6.2 建议数据结构（冻结）
1. [ ] 建议新增或收敛为以下结构：
```go
type assistantReplyRealizerInput struct {
    Stage                  string
    Kind                   string
    Locale                 string
    Outcome                string
    ErrorCode              string
    ErrorMessage           string
    RouteDecision          assistantIntentRouteDecision
    Clarification          *assistantClarificationDecision
    Machine                assistantReplyMachineState
    ReplyGuidanceVersion   string
    KnowledgeSnapshotDigest string
    ResolverContractVersion string
}

type assistantReplyGuidanceSelection struct {
    ReplyKind     string
    TemplateID    string
    TemplateText  string
    Locale        string
    KnowledgeVersion string
}

type assistantReplyRealizerOutput struct {
    Text             string
    Kind             string
    Stage            string
    ReplySource      string
    UsedFallback     bool
    TemplateID       string
    ReplyGuidanceVersion string
}
```
2. [ ] 结构名可按实现调整，但职责必须分离为：输入骨架、资产选择结果、最终输出结果三层。
3. [ ] `assistantRenderReplyResponse` 可以继续作为 API 出口 DTO，但不应再直接承载所有 realizer 中间语义。

### 6.3 `assistantReplyMachineState` 扩面方向
1. [ ] 在不重算事实的前提下，允许把以下只读字段纳入 machine state：
   - [ ] `route_kind / intent_id`；
   - [ ] `clarification_kind / prompt_template_id / current_round`；
   - [ ] `missing_fields[]`；
   - [ ] `candidate_count / candidates / resolved_candidate_id`；
   - [ ] `commit_result`；
   - [ ] `task_status / task_last_error_code`；
   - [ ] `pending_draft_summary / commit_reply` 的只读投影。
2. [ ] 所有字段都必须来自 turn/task/DTO/Resolver，不得在 reply 层自行演算新真相。

### 6.4 模板变量白名单（冻结）
1. [ ] reply 模板变量必须注册到白名单，首批建议允许：
   - [ ] `{missing_fields}`
   - [ ] `{candidate_list}`
   - [ ] `{candidate_count}`
   - [ ] `{selected_candidate}`
   - [ ] `{summary}`
   - [ ] `{effective_date}`
   - [ ] `{entity_name}`
   - [ ] `{parent_ref_text}`
   - [ ] `{error_explanation}`
   - [ ] `{next_action}`
   - [ ] `{task_status}`
2. [ ] 禁止模板直接访问任意 JSON 路径、任意 map key 或模型自由推断字段。
3. [ ] 如需新增变量，必须同步更新测试与执行记录，不得在模型 prompt 中偷偷注入新字段。

## 7. Reply 场景与阶段映射（冻结）
1. [ ] `245` 不要求立刻废弃现有 `stage` 字段，但要求建立 `stage -> reply_kind` 的稳定映射表。
2. [ ] 首批映射建议冻结为：
   - [ ] `await_clarification` / `clarification_required = true` → `clarification_required`
   - [ ] `await_missing_fields` → `missing_fields`
   - [ ] `await_candidate_confirm` 且未选中 → `candidate_list`
   - [ ] `await_candidate_confirm` 且已选中待确认 → `candidate_confirm`
   - [ ] `await_commit_confirm` → `confirm_summary`
   - [ ] `commit_result` 或 `commit_reply.success` → `commit_success`
   - [ ] `commit_failed` 或 `error_code != ''` → `commit_failed`
   - [ ] task `queued/running` → `task_waiting`
   - [ ] task `manual_takeover_required` → `manual_takeover`
   - [ ] `route_kind != business_action` → `non_business_route`
3. [ ] 若 stage 与结构化事实冲突，必须以结构化事实优先，并记录受控错误；不得凭文本内容猜测真实状态。

## 8. 直接实施范围与文件落点

### 8.1 必改文件（首期）
1. [ ] `docs/dev-plans/245-assistant-reply-guidance-pack-and-reply-realizer-plan.md`
   - [ ] 作为 reply 主链实施契约；
   - [ ] 后续范围变更必须先回写本文件。
2. [ ] `internal/server/assistant_reply_nlg.go`
   - [ ] 从“大函数 + fallback 拼文案”重构为：输入装配、guidance 查找、realizer、fallback policy、持久化五段式；
   - [ ] 减少硬编码用户文案，只保留受控 fallback。
3. [ ] `internal/server/assistant_knowledge_runtime.go`
   - [ ] 新增或补齐 `findReplyGuidance(...)`、template 查找与 locale fallback 接口；
   - [ ] 强化 `reply_guidance` 资产校验。
4. [ ] `internal/server/assistant_knowledge/reply_guidance/*.json`
   - [ ] 新增首批 `reply_kind` 资产样例；
   - [ ] 补齐 `zh/en` 双语样例。
5. [ ] `internal/server/assistant_api.go`
   - [ ] 保持 `reply` API 作为入口，但不得继续新增 reply 文案 helper；
   - [ ] 若需要补 reply 审计字段，从 realizer 输出投影回写。
6. [ ] `internal/server/assistant_phase_snapshot.go`
   - [ ] 保证 `assistantSetReplySnapshot(...)` 与 `commit_reply` 的快照一致性；
   - [ ] 不让 snapshot 路径反向重算 reply 事实。

### 8.2 视实现命中的协同文件
1. [ ] `internal/server/assistant_reply_model_gateway.go`
   - [ ] 若继续经模型生成文本，需改为消费 realizer prompt，而不是直接消费粗粒度 fallback text。
2. [ ] `internal/server/assistant_intent_router.go`
   - [ ] 仅在需要投影 `route_kind / intent_id` 到 reply input 时接线；不得在 reply 侧重做 route 决策。
3. [ ] `internal/server/assistant_task_store.go`
   - [ ] 若要支持 `task_waiting / manual_takeover`，需确保 task 状态摘要可只读进入 reply input。
4. [ ] `internal/server/assistant_runtime_status.go`
   - [ ] 如存在运行态摘要 helper，仅允许输出只读状态文本片段，不得替代 realizer。

### 8.3 新增测试/记录文件（建议）
1. [ ] `internal/server/assistant_reply_realizer_test.go`
   - [ ] 专门承载 `Reply Realizer` 的纯函数测试；
   - [ ] 避免所有测试都耦合到整条 NLG pipeline。
2. [ ] `docs/dev-records/dev-plan-245-execution-log.md`
   - [ ] 记录迁移的 helper 文案、资产样例清单、未清退项与原因。

## 9. 实施顺序（可直接开工）

### 9.1 PR-245-01：reply 资产盘点与场景矩阵
1. [ ] 盘点当前用户可见 reply 来源：
   - [ ] `assistantReplyFallbackText(...)` 中的所有硬编码文案；
   - [ ] `assistant_api.go` 中与 reply 相关的 explain/summary 辅助文本；
   - [ ] `assistant_phase_snapshot.go` 中会被用户看到的 commit/reply 快照；
   - [ ] `assistant_reply_model_gateway.go` 的提示输入边界。
2. [ ] 输出迁移矩阵：`现有文案位置 -> 目标 reply_kind -> 目标资产/保留理由`。
3. [ ] 验收：
   - [ ] 能回答“哪些回复已资产化，哪些仍依赖 fallback，为什么”；
   - [ ] 为后续资产样例与清退顺序给出明确清单。

### 9.2 PR-245-02：Reply Guidance Pack 契约加固
1. [ ] 在 `assistant_knowledge_runtime.go` 中强化 reply 资产校验：
   - [ ] `reply_kind` 非空且受控；
   - [ ] `guidance_templates[]` 中 `template_id` 唯一；
   - [ ] `tone_constraints[] / negative_examples[]` 清洗去空；
   - [ ] `error_codes[]` 全部属于已知错误码；
   - [ ] `source_refs[]` 全部有效。
2. [ ] 新增 `findReplyGuidance(replyKind, locale)` 或等价接口，支持 locale fallback。
3. [ ] 验收：
   - [ ] reply 资产不再只是“能被读取”，而是“可被稳定选择与审计”。

### 9.3 PR-245-03：Reply Realizer 最小闭环
1. [ ] 在 `assistant_reply_nlg.go` 或新文件中引入 `assistantBuildReplyRealizerInput(...)` 与 `assistantRealizeReply(...)` 纯函数。
2. [ ] 优先覆盖以下场景：
   - [ ] `missing_fields`
   - [ ] `commit_failed`
   - [ ] `non_business_route`
3. [ ] 受控 fallback 规则冻结：
   - [ ] 仅当资产缺失、模板变量缺失、模型返回空文本或含技术信号时触发；
   - [ ] fallback 文案必须最小、通用、可审计；
   - [ ] fallback 不得掩盖真实失败，也不得产出空字符串。
4. [ ] 验收：
   - [ ] reply 文本不再优先从 `assistantReplyFallbackText(...)` 直接拼出；
   - [ ] 技术错误信号会被 realizer/fallback policy 收敛。

### 9.4 PR-245-04：扩展到澄清 / 候选 / 确认摘要
1. [ ] 接入 `243` 的结构化 clarification：
   - [ ] `clarification_required`
   - [ ] `candidate_list`
   - [ ] `candidate_confirm`
2. [ ] 接入确认摘要：
   - [ ] 在事实已到 `await_commit_confirm` 时统一生成 `confirm_summary`；
   - [ ] 不允许由本地 helper 单独输出“计划已生成，等待确认后可提交”等文案。
3. [ ] 验收：
   - [ ] 澄清、候选、确认三类表达都经统一 realizer；
   - [ ] 与 `243` 主链语义一致，不重复造状态。

### 9.5 PR-245-05：扩展到成功回执 / 任务等待 / 人工接管
1. [ ] 接入 `commit_success`：
   - [ ] 从 `commit_result / commit_reply` 只读投影构造成功回执；
   - [ ] 不凭模型自由补全未持久化事实。
2. [ ] 接入 `task_waiting` 与 `manual_takeover`：
   - [ ] 读取 task 状态 `queued / running / manual_takeover_required`；
   - [ ] 统一为用户可理解、可执行的下一步提示。
3. [ ] 验收：
   - [ ] 用户可见回执、等待、人工接管都不再依赖散点 helper；
   - [ ] 审计快照能解释文本来源与资产版本。

### 9.6 PR-245-06：清退散点 reply 文案与封板证据
1. [ ] 清理首批可删除硬编码：
   - [ ] `assistantReplyFallbackText(...)` 中与已资产化场景重复的文案；
   - [ ] `assistant_api.go` 中与确认摘要/缺字段/候选说明重复的文本；
   - [ ] `assistant_reply_nlg.go` 中仅为旧路径保留的文本分支。
2. [ ] 对暂不能清退的项，必须：
   - [ ] 记录在执行日志；
   - [ ] 说明保留原因与后续归属；
   - [ ] 明确不是主入口。
3. [ ] 验收：
   - [ ] 用户可见反馈主链已由 `Reply Guidance Pack + Reply Realizer` 承担；
   - [ ] fallback 降为受控边角逻辑。

## 10. 测试矩阵（冻结）

### 10.1 资产编译成功路径
1. [ ] `TestAssistantKnowledgeRuntime_FindReplyGuidance_LocaleFallback`
2. [ ] `TestAssistantCompileKnowledgeRuntime_ReplyGuidanceVersionStable`
3. [ ] `TestAssistantCompileKnowledgeRuntime_ReplyGuidanceTemplateRefsValid`

### 10.2 资产编译失败路径
1. [ ] `TestAssistantCompileKnowledgeRuntime_RejectsDuplicateReplyGuidanceTemplateID`
2. [ ] `TestAssistantCompileKnowledgeRuntime_RejectsUnknownReplyGuidanceErrorCode`
3. [ ] `TestAssistantCompileKnowledgeRuntime_RejectsInvalidReplyGuidanceSourceRefs`
4. [ ] `TestAssistantCompileKnowledgeRuntime_RejectsEmptyReplyKind`

### 10.3 Realizer 纯函数路径
1. [ ] `TestAssistantBuildReplyRealizerInput_MissingFields`
2. [ ] `TestAssistantBuildReplyRealizerInput_ClarificationRequired`
3. [ ] `TestAssistantBuildReplyRealizerInput_TaskWaiting`
4. [ ] `TestAssistantRealizeReply_UsesGuidanceTemplate`
5. [ ] `TestAssistantRealizeReply_FallbackWhenGuidanceMissing`
6. [ ] `TestAssistantRealizeReply_SanitizesTechnicalSignals`
7. [ ] `TestAssistantRealizeReply_DoesNotContradictCommitResult`

### 10.4 Pipeline / API 路径
1. [ ] `TestAssistantRenderTurnReply_MissingFieldsUsesReplyGuidance`
2. [ ] `TestAssistantRenderTurnReply_CommitFailedUsesReplyGuidance`
3. [ ] `TestAssistantRenderTurnReply_CandidateListUsesReplyGuidance`
4. [ ] `TestAssistantRenderTurnReply_ConfirmSummaryUsesReplyGuidance`
5. [ ] `TestAssistantRenderTurnReply_ManualTakeoverUsesReplyGuidance`
6. [ ] `TestAssistantRenderTurnReply_PersistsReplySnapshotWithGuidanceVersion`

### 10.5 自然语言回归（首批必备）
1. [ ] 缺字段场景：用户看到的补全提示明确、可执行，不出现技术错误码。
2. [ ] 澄清场景：用户看到的追问与 `243` 的 `prompt_template_id / reason_codes` 一致，不凭 reply 自己追问。
3. [ ] 候选场景：用户看到候选列表/确认提示时，不会误以为系统已自动选择候选。
4. [ ] 成功回执场景：用户能看到已持久化事实摘要，但不会看到未提交字段或未确认动作。
5. [ ] 失败回执场景：用户能理解下一步怎么做，但不会看到 `assistant_*`、`trace_id` 等技术信号。
6. [ ] `zh/en` 至少各覆盖一个 `missing_fields` 与一个 `commit_failed` 样例。

## 11. 验收标准
1. [ ] `Reply Guidance Pack` 不再只是样例文件，而成为 reply 表达的正式资产主源。
2. [ ] 仓内存在明确的 `Reply Realizer` 边界，reply 输入、资产选择、输出渲染不再混在单个 helper 中。
3. [ ] 用户可见的澄清、缺字段、候选解释、确认摘要、成功/失败回执、等待、人工接管至少首批场景已统一进入知识主链。
4. [ ] reply 文本与结构化 turn/task/DTO 事实一致，不会暗中推进 phase、确认或提交。
5. [ ] `reply_guidance_version / knowledge_snapshot_digest` 能随资产变化稳定审计。
6. [ ] 技术错误信号不会大面积直接出现在用户可见文本中。
7. [ ] 能用代码、资产、测试、执行记录四类证据证明“用户可见反馈已主要由知识主链生成，而不是由散点 helper 拼接”。

## 12. 停止线（Fail-Closed）
1. [ ] 若 `245` 最终只是补几份 reply JSON，而 `assistant_reply_nlg.go` 仍主要依赖硬编码 fallback 拼文案，本计划失败。
2. [ ] 若 reply 仍能改写事实状态、隐式推进 confirm/commit 或生成与 DTO 不一致的信息，本计划失败。
3. [ ] 若技术错误码、provider/runtime/schema 术语仍大面积直出到用户文本，本计划失败。
4. [ ] 若 `245` 为了方便而重新把澄清/缺字段/候选/成功回执拆回多个 helper，各自维护不同表达逻辑，本计划失败。
5. [ ] 若 fallback 继续成为主入口，而非资产缺失时的受控兜底，本计划失败。
6. [ ] 若 `245` 反向侵入 `242/243` 去重算 route/clarification 真相，本计划失败。

## 13. 门禁与本地验证入口
1. [ ] 文档与实现触发器以 `AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 为准。
2. [ ] 本计划进入代码实现后，至少命中：
   - [ ] `go fmt ./... && go vet ./... && make check lint && make test`
   - [ ] `make check no-legacy`
   - [ ] `make check error-message`
   - [ ] `make check doc`
3. [ ] 若命中 reply 资产、Assistant 路由/错误码、schema/sqlc 或生成物，按 SSOT 补跑对应门禁；本文不复制脚本细节。

## 14. 交付物
1. [ ] 本计划文档：`docs/dev-plans/245-assistant-reply-guidance-pack-and-reply-realizer-plan.md`
2. [ ] 执行记录：`docs/dev-records/dev-plan-245-execution-log.md`
3. [ ] 代码交付物：
   - [ ] reply guidance 资产扩面；
   - [ ] reply guidance runtime 查找接口；
   - [ ] reply realizer 输入骨架与渲染器；
   - [ ] reply 文案迁移清单与清退记录；
   - [ ] 回归测试与封板证据。

## 15. 关联文档
- `docs/dev-plans/240e-assistant-internal-knowledge-pack-and-readonly-resolver-plan.md`
- `docs/dev-plans/241-assistant-knowledge-pack-runtime-minimal-implementation-plan.md`
- `docs/dev-plans/242-assistant-intent-router-runtime-minimal-plan.md`
- `docs/dev-plans/243-assistant-clarification-policy-and-slot-repair-plan.md`
- `docs/dev-plans/244-assistant-interpretation-pack-and-intent-route-catalog-compiler-plan.md`
- `docs/dev-plans/246-assistant-understand-route-clarify-roadmap.md`
