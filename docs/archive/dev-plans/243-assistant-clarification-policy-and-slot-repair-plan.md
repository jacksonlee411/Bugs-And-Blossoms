# DEV-PLAN-243：Assistant 澄清策略与槽位补全回路实施计划

> 归档说明（2026-04-12）：本文件已自 `docs/dev-plans/` 迁入 `docs/archive/dev-plans/`，仅保留为历史参考，不再作为现行 SSOT。

**状态**: 已实施（2026-03-11 CST；核心主链、持久化、门禁与 Assistant 回归已落地，收口记录见 `docs/archive/dev-records/dev-plan-243-execution-log.md`）

## 1. 计划定位（与 246 对齐）
1. [ ] `243` 是 `DEV-PLAN-246` 的**阶段 C**，只能在 `242` 的 `route decision` 最小 runtime 封板后推进。
2. [ ] `243` 的唯一目标是把“需要追问/需要补槽/需要消歧”收敛成**一个结构化澄清主链**，避免继续散落在 `assistantDryRunValidationExplain`、候选 helper 与局部 merge 逻辑中。
3. [ ] `243` 不负责：
   - [ ] 重做 `Intent Router`（由 `242` 承接）；
   - [ ] 扩充理解知识资产治理（由 `244` 承接）；
   - [ ] 统一最终用户可见自然语言表达（由 `245` 承接）。
4. [ ] `243` 的完成标准不是“提示文案更好看”，而是：**当信息不完整或语义不确定时，系统能进入受控澄清、能在澄清成功后恢复业务动作、能在失败时 fail-closed**。

## 2. 问题定义（现状收敛）
1. [ ] 当前系统已有三类局部能力，但没有统一策略层：
   - [ ] `await_missing_fields`：由 `assistantBuildDryRun` + `assistantTurnMissingFields` 驱动；
   - [ ] `await_candidate_pick / await_candidate_confirm`：由候选解析与 `candidate_confirmation_required` 驱动；
   - [ ] `assistantMergeIntentWithPendingTurn`：只能在少量缺字段场景恢复 `create_orgunit`，无法覆盖 route 低置信度、意图消歧、格式确认等场景。
2. [ ] 在 `242` 完成后，系统将具备 `route_kind / clarification_required / reason_codes / confidence_band`，但若没有 `243`，低置信度输入仍会停在“知道要澄清，却不知道怎么澄清”。
3. [ ] 当前最容易退化的失败模式：
   - [ ] 直接回 `unsupported`；
   - [ ] 继续靠本地 helper 文案硬解释；
   - [ ] 缺字段与意图消歧分裂成两套逻辑；
   - [ ] 澄清轮次无上限，或者多轮后偷偷硬猜动作。

## 3. 范围冻结

### 3.1 本计划包含
1. [ ] 为 `business_action + clarification_required=true` 或“动作已识别但仍缺执行前置信息”的 turn 建立统一 `Clarification Decision`。
2. [ ] 将以下五类场景统一进同一策略模型：
   - [ ] `missing_slots`（缺字段补全）；
   - [ ] `candidate_pick`（多候选选择）；
   - [ ] `candidate_confirm`（候选确认）；
   - [ ] `intent_disambiguation`（多意图/低置信度消歧）；
   - [ ] `format_confirmation`（日期/编码等格式语义确认）。
3. [ ] 为澄清回路定义：
   - [ ] 进入条件；
   - [ ] 轮次上限；
   - [ ] 单轮问题预算；
   - [ ] 成功恢复语义；
   - [ ] 失败退出语义；
   - [ ] confirm/commit 阻断规则。

### 3.2 本计划不包含
1. [ ] `knowledge_qa / chitchat` 的表达统一；它们由 `242` 正式分流、由 `245` 做表达收口。
2. [ ] 多语言自然表达风格与措辞统一；本计划只定义 `prompt_template_id + 结构化变量`，最终 realizer 由 `245` 承接。
3. [ ] 新的正式业务 FSM；`243` 只能在 `route` 与 `plan/confirm` 之间建立受控过渡，不得发明新的提交主链。

### 3.3 首期动作范围
1. [ ] 首期业务动作：`org.orgunit_create` / `create_orgunit`。
2. [ ] 首期必须覆盖的自然语言变体：
   - [ ] “建立一个组织，挂在鲜花组织下面，名字以后再说”；
   - [ ] “在鲜花组织下面建个部门，下个月一号生效”；
   - [ ] “给 FLOWER-A 新建运营部，日期写明天”；
   - [ ] “新建组织还是移动组织我还没想好，先帮我看看”。

## 4. 前置条件（M2 之后才能开工）
1. [ ] `242` 已提供正式 `route_decision`，至少包含：
   - [ ] `route_kind`；
   - [ ] `intent_id`；
   - [ ] `candidate_action_ids[]`；
   - [ ] `clarification_required`；
   - [ ] `reason_codes[]`；
   - [ ] `confidence_band`；
   - [ ] `route_catalog_version`；
   - [ ] `knowledge_snapshot_digest`。
2. [ ] `241` 已提供最小知识快照与 Resolver 运行时，使 `243` 可以读取：
   - [ ] `required_slots[]`；
   - [ ] route catalog 中声明的 `clarification_template_id`；
   - [ ] 解释性字段展示或错误解释骨架。
3. [ ] 若任一 turn 缺少 `route_decision` 或缺少知识版本字段，`243` 必须 fail-closed，而不是回退到旧的 `plan_only` 或本地硬猜逻辑。
4. [ ] `241/242` 必须已明确以下口径，`243` 只允许消费，不允许重定义：
   - [ ] 执行前必填/校验真相来自正式执行主链与 `Contract Resolver`；
   - [ ] `route catalog.required_slots[]` 只用于澄清顺序、提示模板和追问预算，不得单独构成第二套执行真相；
   - [ ] `knowledge_snapshot_digest` 对应的知识快照中必须可复算本次澄清所依赖的 `route/resolver/template` 版本集合。

## 5. 运行时模型（冻结）

### 5.1 新增结构化决策 DTO
1. [ ] 新增 `assistantClarificationDecision`（命名冻结，避免继续散落在零散字段中），最小字段如下：
   - [ ] `clarification_kind`；
   - [ ] `status`（`open / resolved / exhausted / aborted`）；
   - [ ] `prompt_template_id`；
   - [ ] `required_slots[]`；
   - [ ] `missing_slots[]`；
   - [ ] `candidate_action_ids[]`；
   - [ ] `candidate_ids[]`；
   - [ ] `reason_codes[]`；
   - [ ] `max_rounds`；
   - [ ] `current_round`；
   - [ ] `exit_to`（`business_action_resume / uncertain / manual_hint`）；
   - [ ] `await_phase`；
   - [ ] `knowledge_snapshot_digest`；
   - [ ] `route_catalog_version`。
2. [ ] `assistantClarificationDecision` 是**turn 级审计事实**，不承担业务真相，不得声明正式 `commit` 条件。
3. [ ] `assistantTurn` 增加 `Clarification *assistantClarificationDecision`，并与 `route_decision` 并列存储，不与 `intent_json` 混装。
4. [ ] 版本口径冻结：
   - [ ] `knowledge_snapshot_digest` 为澄清主版本锚点，必须可追溯到当次消费的 `route_catalog / resolver_contract / context_template` 版本集合；
   - [ ] `route_catalog_version` 作为便于审阅的显式字段保留；
   - [ ] 若后续实现发现仅凭 `knowledge_snapshot_digest` 无法稳定复核澄清输入，必须先回写本计划，再扩充 DTO 版本字段。
5. [ ] 术语关系冻结：
   - [ ] `clarification_template_id` 是 `Intent Route Catalog` 中的**静态引用键**；
   - [ ] `prompt_template_id` 是 `assistantClarificationDecision` 中落地到 turn 审计事实的**已解析模板 ID**；
   - [ ] 首期若无额外重写规则，二者可以取同一值；但运行时与审计层统一使用 `prompt_template_id`，避免 DTO 与资产层再次分叉命名。

### 5.2 phase 语义冻结
1. [ ] 保留现有 phase：
   - [ ] `await_missing_fields`；
   - [ ] `await_candidate_pick`；
   - [ ] `await_candidate_confirm`；
   - [ ] `await_commit_confirm`。
2. [ ] 新增唯一通用 phase：`await_clarification`，仅用于：
   - [ ] `intent_disambiguation`；
   - [ ] `format_confirmation`。
3. [ ] phase 与 kind 的映射冻结如下：

| clarification_kind | await_phase |
| --- | --- |
| `missing_slots` | `await_missing_fields` |
| `candidate_pick` | `await_candidate_pick` |
| `candidate_confirm` | `await_candidate_confirm` |
| `intent_disambiguation` | `await_clarification` |
| `format_confirmation` | `await_clarification` |

4. [ ] `state` 在澄清期间保持 `validated`；只有正式确认后才允许进入 `confirmed`。
5. [ ] 只要 `Clarification.status=open`，`confirm/commit` 必须被 gate 阻断。
6. [ ] 单一权威表达冻结：
   - [ ] `assistantTurn.Clarification` 是“当前是否存在 open clarification、属于哪一类、轮次推进到哪里”的唯一主源；
   - [ ] `assistant_turns.phase` 是对 `Clarification.kind/status` 的派生投影，用于现有 API/UI/状态机兼容；
   - [ ] `route_decision.clarification_required` 只表示“route 层是否建议进入澄清”，不得单独替代 turn 上的 open clarification 事实；
   - [ ] 任意时刻若 `Clarification.status=open` 与 `phase/route_decision` 投影不一致，必须视为 `assistant_clarification_runtime_invalid` 并 fail-closed。

### 5.3 持久化冻结
1. [ ] 在 `iam.assistant_turns` 增加 `clarification_json jsonb NOT NULL DEFAULT '{}'::jsonb`。
2. [ ] 增加 JSON shape check：`jsonb_typeof(clarification_json) = 'object'`。
3. [ ] `assistant_turns.phase_check` 与 `assistant_state_transitions.{from_phase,to_phase}` 增加 `await_clarification`。
4. [ ] Memory 路径与 PG 路径必须写入同一 `assistantClarificationDecision` 结构，不允许一条路径只保存在内存派生字段中。
5. [ ] 除数据库 shape check 外，应用层必须补充 semantic validator，至少校验：
   - [ ] `status=open` 时，`clarification_kind / max_rounds / current_round / exit_to / await_phase / knowledge_snapshot_digest / route_catalog_version` 不得为空；
   - [ ] `clarification_kind ↔ await_phase` 必须符合第 5.2 节映射；
   - [ ] `current_round <= max_rounds`；
   - [ ] `status=resolved/exhausted/aborted` 时不得继续作为 `confirm/commit` 的阻断主因残留。

## 6. 澄清类型判定顺序（优先级冻结）
1. [ ] 同一时刻只允许存在**一个 open clarification decision**。
2. [ ] 判定顺序冻结为：
   1. [ ] `intent_disambiguation`；
   2. [ ] `candidate_pick`；
   3. [ ] `candidate_confirm`；
   4. [ ] `format_confirmation`；
   5. [ ] `missing_slots`。
3. [ ] 解释：
   - [ ] 如果动作都还不确定，不允许先问业务字段；
   - [ ] 如果候选对象未选定，不允许先进入 commit confirm；
   - [ ] 如果日期/格式语义本身不明确，不应把它误当成普通缺字段；
   - [ ] 只有在动作、候选、格式都稳定后，才进入普通缺字段补全。

## 7. 各类澄清的进入条件（直接实施口径）

### 7.1 `intent_disambiguation`
1. [ ] 进入条件任一满足即可：
   - [ ] `route_decision.clarification_required=true`；
   - [ ] `candidate_action_ids` 数量 > 1；
   - [ ] `reason_codes` 含 `low_confidence`、`multi_intent`、`route_conflict` 等等价原因码。
2. [ ] 输出要求：
   - [ ] `candidate_action_ids[]` 必填；
   - [ ] `required_slots[]` 先只带 route catalog 声明值，不直接追问；
   - [ ] `prompt_template_id` 来自 route catalog 或 interpretation pack 中已注册 template。
3. [ ] 此阶段不得调用正式 `assistantBuildPlan` 生成可提交摘要。

### 7.2 `candidate_pick`
1. [ ] 动作已稳定，且存在多个候选对象需要选择。
2. [ ] 触发信号：
   - [ ] `candidate_confirmation_required`；
   - [ ] `ResolvedCandidateID` 为空；
   - [ ] `Candidates` 非空且数量 > 1。
3. [ ] 输出要求：
   - [ ] `candidate_ids[]` 与 `Candidates` 对齐；
   - [ ] `await_phase=await_candidate_pick`；
   - [ ] `reason_codes` 至少包含 `candidate_pick_required`。

### 7.3 `candidate_confirm`
1. [ ] 候选已收敛到单个目标，但仍需用户显式确认。
2. [ ] 触发信号：
   - [ ] `candidate_confirmation_required`；
   - [ ] `SelectedCandidateID` 或 `ResolvedCandidateID` 非空。
3. [ ] 输出要求：
   - [ ] `candidate_ids[]` 仅保留当前待确认候选；
   - [ ] `await_phase=await_candidate_confirm`；
   - [ ] `reason_codes` 至少包含 `candidate_confirm_required`。

### 7.4 `format_confirmation`
1. [ ] 动作已稳定，但关键字段存在“值已给出、语义未冻结”的场景。
2. [ ] 首期只冻结日期格式确认：
   - [ ] `invalid_effective_date_format`；
   - [ ] `invalid_target_effective_date_format`；
   - [ ] 中文相对日期（如“明天”“下个月一号”）尚未被标准化时。
3. [ ] 输出要求：
   - [ ] `missing_slots[]` 只包含对应日期字段；
   - [ ] `reason_codes` 至少包含 `date_format_confirmation_required`；
   - [ ] `await_phase=await_clarification`。

### 7.5 `missing_slots`
1. [ ] 动作已稳定、候选已稳定、格式已稳定，但执行前仍有必填槽位缺失。
2. [ ] 缺槽位来源冻结为：
   - [ ] **执行真相来源**：正式执行主链 / `Contract Resolver` / `assistantIntentValidationErrors` / `assistantTurnMissingFields` 的归一化结果；
   - [ ] **追问顺序来源**：route catalog 的 `required_slots[]`。
3. [ ] 优先级冻结：
   - [ ] 是否“真的缺字段、缺哪些字段”只由执行真相来源裁决；
   - [ ] route catalog 只能对已判定为缺失的字段进行排序、裁剪单轮追问顺序与选择模板；
   - [ ] 若 route catalog 与执行真相来源冲突，以执行真相来源为准，并记录 reason code/审计痕迹；不得为迎合 catalog 而改写执行必填集。
4. [ ] 输出要求：
   - [ ] `required_slots[]` 保持 route catalog 顺序；
   - [ ] `missing_slots[]` 只保留当前仍缺的字段；
   - [ ] `await_phase=await_missing_fields`。

## 8. 轮次、问题预算与退出语义（冻结）

### 8.1 轮次上限
1. [ ] `intent_disambiguation.max_rounds = 2`。
2. [ ] `candidate_pick.max_rounds = 2`。
3. [ ] `candidate_confirm.max_rounds = 1`。
4. [ ] `format_confirmation.max_rounds = 2`。
5. [ ] `missing_slots.max_rounds = 3`。
6. [ ] 到达上限后必须关闭当前 clarification，并且只能：
   - [ ] `exit_to=uncertain`；或
   - [ ] `exit_to=manual_hint`。

### 8.2 单轮问题预算
1. [ ] 单轮只允许输出**一个 clarification decision**。
2. [ ] `missing_slots` 单轮最多追问 **1 个 slot**，按 `required_slots[]` 顺序推进；不得一次抛出长串字段清单。
3. [ ] `candidate_pick` 单轮最多展示 **3 个候选**；超出部分只保留结构化数据，不在本计划中扩写文案。
4. [ ] `intent_disambiguation` 单轮最多给出 **2 个动作候选**。

### 8.3 进展判断（是否算“回答了问题”）
1. [ ] 仅当以下任一条件满足，才视为澄清有进展：
   - [ ] `missing_slots` 数量减少；
   - [ ] `SelectedCandidateID` 或 `ResolvedCandidateID` 被填充；
   - [ ] `candidate_action_ids[]` 收敛为单一动作；
   - [ ] 非标准日期被归一化为 `YYYY-MM-DD`；
   - [ ] `clarification_required` 从 `true` 变为 `false`。
2. [ ] 用户输入若未带来上述任一变化，则记为 `clarification_no_progress`，并推进 `current_round`。
3. [ ] 用户连续两次无关输入、冲突输入或“还是你决定吧”这类推责输入，必须走 `manual_hint`，不得硬猜。

### 8.4 退出语义
1. [ ] `business_action_resume`：
   - [ ] 动作已唯一；
   - [ ] 候选已唯一并确认；
   - [ ] 关键格式已标准化；
   - [ ] 当前轮缺槽位已消除到可继续 `plan/dry_run`。
2. [ ] `uncertain`：
   - [ ] 多轮后仍无法唯一确定动作；
   - [ ] 用户持续输入与当前澄清目标无关；
   - [ ] route 与局部补槽结果相互冲突。
3. [ ] `manual_hint`：
   - [ ] 业务上需要人工明确判断；
   - [ ] 连续冲突/推责/上下文丢失导致继续追问无意义；
   - [ ] 轮次耗尽且无法安全恢复业务动作。
4. [ ] 退出后禁止保留“半恢复、半待确认”的中间态；必须显式落到：
   - [ ] `business_action_resume`：关闭 open clarification，重新进入正式主链判定；
   - [ ] `uncertain`：关闭 open clarification，不得生成可提交 plan；
   - [ ] `manual_hint`：关闭 open clarification，不得继续自动推进行动。

## 9. 与现有实现的整合策略

### 9.1 现有逻辑的角色重定义
1. [ ] `assistantMergeIntentWithPendingTurn` 继续保留，但角色冻结为：**缺字段澄清成功后的局部恢复工具**。
2. [ ] `assistantBuildDryRun` 继续负责执行前校验，但不再直接决定“下一句该问什么”。
3. [ ] `assistantDryRunValidationExplain` 在 `243` 中只能作为过渡期 fallback，不得继续扩张为策略主入口。
4. [ ] `assistantTurnMissingFields / assistantIntentValidationErrors / candidate_confirmation_required` 都应变成 `assistantBuildClarificationDecision` 的输入，而不是各自演化。
5. [ ] 过渡 fallback 退场条件冻结：
   - [ ] 一旦 `missing_slots / candidate_pick / candidate_confirm / intent_disambiguation / format_confirmation` 五类场景都已由 `assistantBuildClarificationDecision` 覆盖，`assistantDryRunValidationExplain` 不得再参与“下一问”裁决；
   - [ ] 若实现阶段仍新增依赖 `assistantDryRunValidationExplain` 的澄清分支，应视为偏离本计划并先回写文档；
   - [ ] 执行记录中必须明确标注 fallback 仍保留的原因与删除时点，避免形成 legacy 双链路。

### 9.2 新的统一 builder
1. [ ] 新增 `assistantBuildClarificationDecision(...)`（命名冻结），输入至少包含：
   - [ ] `route_decision`；
   - [ ] `assistantIntentSpec`；
   - [ ] `assistantDryRunResult`；
   - [ ] `assistantCandidate[]`；
   - [ ] `assistantTurn` 历史澄清上下文；
   - [ ] 知识版本信息。
2. [ ] 输出：
   - [ ] `nil`：无需澄清，可继续正常 `plan/confirm`；
   - [ ] 非 `nil`：进入 `await_*` 或 `await_clarification`，并阻断 confirm/commit。
3. [ ] `assistantRefreshTurnDerivedFields` 与 `assistantTurnPhase` 必须读取 `Clarification.status/open kind`，而不是只看 `missing_fields` 和 `candidate_confirmation_required`。

### 9.3 新的恢复入口
1. [ ] 新增 `assistantResumeFromClarification(...)`（命名建议，可在实现时微调，但职责不可变）：
   - [ ] 针对 `missing_slots`：合并字段；
   - [ ] 针对 `candidate_pick`：解析候选主键/编码/名称；
   - [ ] 针对 `candidate_confirm`：解析确认/拒绝；
   - [ ] 针对 `intent_disambiguation`：收敛动作；
   - [ ] 针对 `format_confirmation`：标准化日期。
2. [ ] 该恢复入口必须返回“是否有进展”的布尔结果，供 `current_round` 与 `exit_to` 决策使用。
3. [ ] 恢复语义冻结：
   - [ ] `assistantResumeFromClarification(...)` 只能生成“补充输入/解析结果”，不得直接把 turn 标记为可提交；
   - [ ] 任一澄清成功后，必须重新执行 `route_decision -> contract/dry_run validation -> clarification rebuild -> phase derive` 主链；
   - [ ] 只有在重跑主链后 `Clarification=nil` 或 `status!=open`，且正式 `confirm/commit` 条件满足时，才能恢复到正常 `plan/confirm` 链；
   - [ ] 若恢复结果与原 `route_decision` 冲突，必须重新以最新 route 为准，不得做局部打补丁式放行。

## 10. 直接实施切片（按 PR 可落地）

### 10.1 PR-243-01：DTO、phase 与持久化骨架
1. [ ] 代码触点：
   - [ ] `internal/server/assistant_api.go`：新增 `assistantClarificationDecision` 与 `assistantTurn.Clarification`；
   - [ ] `internal/server/assistant_phase_snapshot.go`：新增 `assistantPhaseAwaitClarification` 与 phase 派生逻辑；
   - [ ] `internal/server/assistant_persistence.go`：读写 `clarification_json`；
   - [ ] `internal/sqlc/schema.sql`：新增列与 check constraint。
2. [ ] 验收：
   - [ ] memory/PG 路径均能 round-trip `clarification_json`；
   - [ ] `await_clarification` 可被正确持久化与回放。

### 10.2 PR-243-02：create-turn 阶段构建统一澄清决策
1. [ ] 代码触点：
   - [ ] `internal/server/assistant_api.go`；
   - [ ] `internal/server/assistant_create_policy_precheck.go`；
   - [ ] `internal/server/assistant_intent_pipeline.go`。
2. [ ] 实现内容：
   - [ ] 在 `route_decision` 之后、`confirm/commit` 之前构建 `Clarification Decision`；
   - [ ] 优先级严格按第 6 节执行；
   - [ ] open clarification 不生成“等待确认提交”的误导性摘要。
3. [ ] 验收：
   - [ ] 低置信度 business_action 进入 `await_clarification`；
   - [ ] 缺字段进入 `await_missing_fields`；
   - [ ] 多候选进入 `await_candidate_pick`。

### 10.3 PR-243-03：pending turn 恢复与轮次推进
1. [ ] 代码触点：
   - [ ] `internal/server/assistant_api.go`；
   - [ ] `internal/server/assistant_intent_pipeline.go`；
   - [ ] `internal/server/assistant_phase_snapshot.go`。
2. [ ] 实现内容：
   - [ ] 用统一恢复入口替代“仅缺字段可 merge”的局部能力；
   - [ ] 计算 `clarification_no_progress`；
   - [ ] 当恢复成功时，不是直接跳过校验，而是清理 open clarification 后重新跑正式主链，再决定是否进入正常 `plan` 链。
3. [ ] 验收：
   - [ ] 同一会话下连续澄清能逐轮推进；
   - [ ] 恢复成功后 phase 从 `await_*` 回到 `await_commit_confirm` 或正常可执行状态；
   - [ ] 无进展时轮次会增加。

### 10.4 PR-243-04：confirm/commit gate 与错误码冻结
1. [ ] 代码触点：
   - [ ] `internal/server/assistant_action_interceptor.go`；
   - [ ] `internal/server/assistant_api.go`；
   - [ ] 错误码映射与 API 响应测试文件。
2. [ ] 新增/冻结最小错误码：
   - [ ] `assistant_clarification_required`；
   - [ ] `assistant_clarification_rounds_exhausted`；
   - [ ] `assistant_manual_hint_required`；
   - [ ] `assistant_clarification_runtime_invalid`。
3. [ ] 对外 reason code 最小集合：
   - [ ] `intent_disambiguation_required`；
   - [ ] `candidate_pick_required`；
   - [ ] `candidate_confirm_required`；
   - [ ] `date_format_confirmation_required`；
   - [ ] `missing_required_slot`；
   - [ ] `clarification_no_progress`；
   - [ ] `clarification_rounds_exhausted`。
4. [ ] 运行时一致性要求：
   - [ ] 若出现 `Clarification` / `phase` / `route_decision` 不一致，统一落到 `assistant_clarification_runtime_invalid`，不得继续隐式放行。
5. [ ] 验收：
   - [ ] open clarification 时 `confirm/commit` 明确阻断；
   - [ ] 不再把澄清场景压扁成 `assistant_unsupported_intent`。

### 10.5 PR-243-05：回归测试与证据
1. [ ] 代码触点：
   - [ ] `internal/server/assistant_phase_snapshot_test.go`；
   - [ ] `internal/server/assistant_action_interceptor_test.go`；
   - [ ] `internal/server/assistant_api_gap_test.go`；
   - [ ] `internal/server/assistant_persistence_coverage_test.go`；
   - [ ] 视实现新增 `internal/server/assistant_clarification_policy_test.go`。
2. [ ] 验收：
   - [ ] 覆盖 memory/PG 双路径；
   - [ ] 覆盖 create/continue/confirm/commit 四条入口；
   - [ ] 覆盖 “route catalog 与执行真相来源冲突时以执行真相为准” 的回归；
   - [ ] 覆盖 `Clarification/phase/route_decision` 不一致时 fail-closed 的回归；
   - [ ] 至少一条自然语言回归证明系统会追问，而不是 unsupported。

## 11. 推荐测试矩阵（建议直接采用）

### 11.1 unit / policy
1. [ ] `TestAssistantBuildClarificationDecision_IntentDisambiguationWinsOverMissingSlots`
2. [ ] `TestAssistantBuildClarificationDecision_CandidatePickFromValidationErrors`
3. [ ] `TestAssistantBuildClarificationDecision_FormatConfirmationForRelativeDate`
4. [ ] `TestAssistantBuildClarificationDecision_MissingSlotsUsesRouteCatalogOrder`
5. [ ] `TestAssistantBuildClarificationDecision_ReturnsNilWhenResumeReady`
6. [ ] `TestAssistantBuildClarificationDecision_ExecutionTruthWinsOverRouteCatalog`

### 11.2 phase / derived fields
1. [ ] `TestAssistantTurnPhase_OpenIntentDisambiguationUsesAwaitClarification`
2. [ ] `TestAssistantTurnPhase_OpenFormatConfirmationUsesAwaitClarification`
3. [ ] `TestAssistantTurnPhase_OpenMissingSlotsUsesAwaitMissingFields`
4. [ ] `TestAssistantTurnPhase_ResolvedClarificationRestoresCommitConfirm`
5. [ ] `TestAssistantTurnPhase_RuntimeInvalidWhenClarificationAndPhaseDiverge`

### 11.3 API / action gate
1. [ ] `TestAssistantCreateTurn_LowConfidenceBusinessActionReturnsClarification`
2. [ ] `TestAssistantCreateTurn_MultiCandidateReturnsCandidatePick`
3. [ ] `TestAssistantConfirmTurn_OpenClarificationBlocked`
4. [ ] `TestAssistantCommitTurn_OpenClarificationBlocked`
5. [ ] `TestAssistantContinueClarification_NoProgressIncrementsRound`
6. [ ] `TestAssistantContinueClarification_RoundsExhaustedFallsBackToUncertain`

### 11.4 persistence
1. [ ] `TestAssistantUpsertTurnTx_PersistsClarificationJSON`
2. [ ] `TestAssistantLoadConversationTx_RestoresClarificationJSON`
3. [ ] `TestAssistantTransitions_PersistsAwaitClarificationPhase`
4. [ ] `TestAssistantClarificationSemanticValidator_RejectsIncompleteOpenDecision`

### 11.5 自然语言回归（首批必备）
1. [ ] “建立一个组织，挂在鲜花组织下面，名字以后再说” → `missing_slots`。
2. [ ] “在鲜花组织下面建个部门，下个月一号生效” → `format_confirmation` 或成功标准化后继续。
3. [ ] “新建还是移动组织我还没想好，先帮我看看” → `intent_disambiguation`。
4. [ ] “在鲜花组织下面新建运营部” 且存在多个鲜花组织候选 → `candidate_pick`。

## 12. 验收标准
1. [ ] 仓库内存在正式 `assistantClarificationDecision` DTO，且 memory/PG 路径使用同一结构。
2. [ ] `assistant_turns` 可审计到 `clarification_kind / reason_codes / current_round / exit_to / prompt_template_id`。
3. [ ] 低置信度或多意图 business_action 不再直接走 `unsupported`，而是进入 `await_clarification`。
4. [ ] 缺字段、候选选择、候选确认不再各自维护独立下一步策略，而是统一由 `assistantBuildClarificationDecision` 裁决。
5. [ ] open clarification 时，API/UI 契约层不会展示“等待确认提交”的误导性 CTA。
6. [ ] 澄清成功后可恢复既有 `plan/confirm` 主链；澄清失败后只能进入 `uncertain` 或 `manual_hint`。
7. [ ] 至少一条中文自然表达回归证明系统会追问、会止损、不会硬猜。
8. [ ] `Clarification` 成为 turn 级澄清事实唯一主源，`phase` 与 `route_decision` 仅作受控投影，不再并列演化成第二套状态真相。
9. [ ] `missing_slots` 的“是否缺失”与“缺哪些字段”只由执行真相来源裁决，route catalog 只负责排序与模板引用。
10. [ ] 任一澄清恢复成功后，系统都会重新跑正式主链，而不是依赖局部 merge 直接放行到可提交态。

## 13. 停止线（Fail-Closed）
1. [ ] 若实现结果只是继续扩写 `assistantDryRunValidationExplain` 或页面本地文案，而没有独立 `assistantClarificationDecision`，本计划失败。
2. [ ] 若 `clarification` 仍与 `missing_fields / candidate_confirmation_required / route_low_confidence` 三套逻辑分头演化，本计划失败。
3. [ ] 若 `confirm/commit` 在 open clarification 下仍可推进，本计划失败。
4. [ ] 若超过轮次上限后仍继续硬猜动作、继续生成可提交 plan，或偷偷回落旧 `plan_only -> create_orgunit` 旁路，本计划失败。
5. [ ] 若 Memory 路径与 PG 路径的 clarification 行为不一致，本计划失败。
6. [ ] 若新增 phase / 错误码 / reason code 最终被压扁回 `assistant_unsupported_intent`，本计划失败。
7. [ ] 若 `Clarification`、`phase`、`route_decision.clarification_required` 三者都能各自独立决定是否可提交，而未冻结唯一主源，本计划失败。
8. [ ] 若 route catalog 借由 `required_slots[]` 实际重定义了执行必填集，形成第二套执行真相，本计划失败。
9. [ ] 若澄清恢复成功后未重新跑正式主链，而是通过局部 merge/patch 直接进入可确认或可提交态，本计划失败。
10. [ ] 若 `assistantDryRunValidationExplain` 在统一 builder 落地后仍持续扩张为策略主入口，本计划失败。

## 14. 门禁与本地验证入口
1. [ ] 文档与实现触发器以 `AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 为准。
2. [ ] 本计划进入代码实现后，至少命中：
   - [ ] `go fmt ./... && go vet ./... && make check lint && make test`
   - [ ] `make check no-legacy`
   - [ ] `make check error-message`
   - [ ] `make check doc`
3. [ ] 若命中 schema/sqlc/Assistant route 相关变更，还需按 SSOT 补跑对应检查；本文不复制脚本细节。

## 15. 交付物
1. [ ] 本计划文档：`docs/archive/dev-plans/243-assistant-clarification-policy-and-slot-repair-plan.md`
2. [ ] 执行记录：`docs/archive/dev-records/dev-plan-243-execution-log.md`
3. [ ] 代码交付物：
   - [ ] `assistantClarificationDecision` DTO；
   - [ ] `clarification_json` 持久化；
   - [ ] `await_clarification` phase 与统一 builder；
   - [ ] 恢复入口与轮次控制；
   - [ ] confirm/commit gate；
   - [ ] 回归测试与证据。

## 16. 关联文档
- `docs/archive/dev-plans/240e-assistant-internal-knowledge-pack-and-readonly-resolver-plan.md`
- `docs/archive/dev-plans/241-assistant-knowledge-pack-runtime-minimal-implementation-plan.md`
- `docs/archive/dev-plans/242-assistant-intent-router-runtime-minimal-plan.md`
- `docs/archive/dev-plans/244-assistant-interpretation-pack-and-intent-route-catalog-compiler-plan.md`
- `docs/archive/dev-plans/245-assistant-reply-guidance-pack-and-reply-realizer-plan.md`
- `docs/archive/dev-plans/246-assistant-understand-route-clarify-roadmap.md`
