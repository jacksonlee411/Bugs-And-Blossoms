# DEV-PLAN-341：Assistant 主线演进与 340/350 问题关联调查报告

**状态**: 已完成（2026-04-12 09:36 CST）

## 1. 背景
1. [X] `DEV-PLAN-340` 已明确提出一个关键疑问：Assistant 是否在 OrgUnit 正式维护链路之外，又长出了一套重复维护实现。
2. [X] `DEV-PLAN-350` 又进一步把问题提升为统一策略模型层面：即使不存在第二写入口，也必须避免 Assistant 成为第二个策略解释器。
3. [X] 为避免后续整改只盯住局部文件，需要把 `DEV-PLAN-220~293` 这条 Assistant/LibreChat 主线重新串起来，回答三个问题：
   - [X] 这条主线原本试图解决什么；
   - [X] `340/350` 指出的问题是在什么阶段被埋下并放大的；
   - [X] 当前应如何理解“Assistant 的正式边界”。

## 2. 调查范围
1. [X] 计划文档：`DEV-PLAN-220~293` 中与 Assistant/LibreChat 主链直接相关的文档。
2. [X] 重点交叉文档：
   - [X] `DEV-PLAN-220/223/240/260/266/267/268/271/272/280/293/340/350`
3. [X] 代码抽样核对：
   - [X] `internal/server/assistant_action_registry.go`
   - [X] `internal/server/assistant_create_policy_precheck.go`
   - [X] `modules/orgunit/services/orgunit_write_service.go`

## 3. 已确认的主线演进

### 3.1 第一阶段：冻结基础原则，而不是冻结 UI 形态
1. [X] `DEV-PLAN-220` 在当前口径下已退居“历史总纲”，但仍保留三个长期有效原则：
   - [X] 聊天式交互方向成立；
   - [X] 业务裁决边界留在本仓；
   - [X] 会话必须事务化、可回放、可审计。
2. [X] 这说明后续所有演进，无论是 LibreChat 承载、前端重构还是语义核切换，都没有改变“业务真相必须留在本仓”的底层原则。

### 3.2 第二阶段：先把业务真相从前端和页面桥接中收回
1. [X] `DEV-PLAN-223` 把 `conversation/turn/request/trace + phase + 状态转移审计` 冻结为唯一业务事实源。
2. [X] `DEV-PLAN-260` 冻结了真实业务对话闭环的 DTO 与 FSM，要求所有关键业务步骤通过对话完成，且业务 FSM 以后端为 SSOT。
3. [X] `DEV-PLAN-266` 明确自己只负责“单通道 + 气泡内回写”，不能单独代表业务闭环达成。
4. [X] `DEV-PLAN-280/284` 则进一步把前端降权为只消费 DTO，禁止前端 helper/adapter 重算业务语义。
5. [X] 结论：`220~223 + 260 + 266 + 280` 这一段，主要解决的是“UI 不是业务真相源”的问题。

### 3.3 第三阶段：把编排主链从硬编码流程收口为事务型 Assistant 骨架
1. [X] `DEV-PLAN-240` 及其 `240A~240D` 把 Assistant 收敛为事务型后端编排主链，引入：
   - [X] `ActionRegistry`
   - [X] `CommitAdapter`
   - [X] 风险门与统一拦截
   - [X] 异步 receipt / task / poll 主链
2. [X] `DEV-PLAN-271` 将其与 `223/260/280` 编成单一推进主链，并在 `240F + 285` 完成封板。
3. [X] 这一阶段的收益是：Assistant 已不再是页面外挂逻辑，而是仓内正式的事务编排服务。

### 3.4 第四阶段：把“理解权”从本地多层状态机收回到单一语义核
1. [X] `DEV-PLAN-241~245` 先完成知识、route、clarification、reply 的最小运行时闭环。
2. [X] 但 `DEV-PLAN-267` 复盘指出：系统在工程上更可控、更可审计，却在产品上更僵硬、更保守、更容易误导用户。
3. [X] `DEV-PLAN-268` 因此改成“外部大模型单一语义核 + 本地最小执行边界”，删除本地多层平行判断权。
4. [X] `DEV-PLAN-293` 继续把边界说得更严：模型输出只能是 `proposal`，任何业务真值只能来自 authoritative gate 之后的 turn state。
5. [X] 结论：`267/268/293` 解决的是“模型和本地 runtime 谁拥有语义真相”的问题。

### 3.5 第五阶段：动作扩面让策略重复解释问题暴露出来
1. [X] `DEV-PLAN-272` 将 OrgUnit 七动作全部纳入正式运行态。
2. [X] 为了让多动作在 `createTurn/dry-run` 阶段就给出稳定可解释反馈，Assistant 开始在运行时提前处理：
   - [X] 创建字段策略；
   - [X] SetID 上下文解析；
   - [X] `FIELD_REQUIRED_VALUE_MISSING` / `PATCH_FIELD_NOT_ALLOWED` 等前置错误。
3. [X] 这一步保证了用户体验与对话闭环，但也把部分领域前置裁决提前吸进了 Assistant runtime。
4. [X] 结论：`272` 不是“做错了”，而是把“为了完成对话体验而在编排层补做策略解释”的结构性代价显性化了。

## 4. 对 340/350 的问题定性

### 4.1 未形成第二写入口
1. [X] 代码与文档都表明，Assistant 的 commit adapter 统一复用 `OrgUnitWriteService`。
2. [X] `create/add_version/insert_version/correct/disable/enable/move/rename` 最终都落到同一正式写服务，而不是 Assistant 自己写库。
3. [X] 因此，当前问题不能定性为“第二 DB Kernel”或“第二写入口”。

### 4.2 已形成第二套偏 OrgUnit 专属的运行时编排与预检结构
1. [X] Assistant 已拥有自己的动作注册、required slots、候选确认、dry-run、confirm gate、commit gate、plan 编译与错误回填体系。
2. [X] 这套体系本身并不违背 One Door；它属于事务编排层，是 `240/260/272` 演进的自然结果。
3. [X] 真正的问题在于：这套编排层已经开始重复理解 OrgUnit 策略，而不只是组织对话和提交。

### 4.3 已形成“第二策略解释器”的倾向
1. [X] `assistant_create_policy_precheck.go` 会直接解析 org 上下文、获取 `ResolvedSetID`、调用字段决议查询，并直接回填 `FIELD_REQUIRED_VALUE_MISSING` / `PATCH_FIELD_NOT_ALLOWED`。
2. [X] 与此同时，`modules/orgunit/services/orgunit_write_service.go` 已在正式写链路内通过 `applyCreatePolicyDefaults(...)` 承担同一组创建字段策略默认值、允许性与必填裁决。
3. [X] 因而当前真正命中的问题是：
   - [X] Assistant 不是第二个写服务；
   - [X] 但 Assistant 已经部分成为第二个策略解释器。

## 5. 根因归纳
1. [X] `220~293` 的主线，依次解决了三个层面的“真相收权”：
   - [X] 业务真相不能留在前端；
   - [X] 语义真相不能由本地多层状态机重复判断；
   - [X] 但策略真相尚未完全从 Assistant runtime 中收回。
2. [X] `272` 扩动作之后，Assistant 需要在 dry-run 阶段给出更早、更可解释的失败原因；而仓内当时缺少一个可复用的正式预检投影边界，于是 Assistant 直接读取策略底层信息补足用户体验。
3. [X] `293` 已开始修正“runtime proposal 过早成为 turn 真值”的问题，但它主要处理的是 semantic truth，不直接解决 policy truth。
4. [X] 因此，`340/350` 实际是在前面两轮收权之后，开始第三轮收权：把“领域策略解释权”也收回到统一策略主链。

## 6. 本次调查的正式结论
1. [X] `DEV-PLAN-340` 的判断成立：Assistant 当前不是第二写入口，但已经出现领域前置裁决重复。
2. [X] `DEV-PLAN-350` 的方向也成立：后续收敛目标不应是“给 Assistant 再加一个专用预检服务”，而应是让 Assistant 成为统一策略模型的消费方。
3. [X] 对 `DEV-PLAN-220~293` 的整体评价应更新为：
   - [X] 它们已经把 Assistant 做成了正式的事务型对话系统；
   - [X] 它们已经解决了 UI 真相和大部分语义真相边界；
   - [X] 但尚未完全解决策略真相边界。
4. [X] 因而 `340/350` 不是对前序主线的否定，而是前序主线的下一次结构性纠偏。

## 7. 对后续计划的输入
1. [X] `340` 应继续聚焦“重复维护点在哪里、哪些必须退回正式边界”。
2. [X] `350` 应继续作为正式实施母法，冻结：
   - [X] `ActionSchema`
   - [X] `PolicyContext`
   - [X] `PrecheckProjection`
   - [X] `Readonly Tool Registry`
   - [X] Assistant snapshot 中的策略版本与 digest
3. [X] 若后续整改只是在 Assistant 外面再包一层 façade，但底层仍由 Assistant 自行解释 `SetID Strategy Registry`、字段策略或 Mutation Policy，则不能视为完成收敛。

## 8. 交付物
1. [X] 调查文档：`docs/dev-plans/341-assistant-mainline-evolution-and-340-350-correlation-investigation.md`
2. [X] 调查结论可直接作为以下计划的输入引用：
   - [X] `DEV-PLAN-340`
   - [X] `DEV-PLAN-350`

## 9. 关联事实源
1. [X] `docs/dev-plans/220-chat-assistant-upgrade-implementation-plan.md`
2. [X] `docs/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
3. [X] `docs/dev-plans/240-assistant-org-transaction-orchestration-modernization-plan.md`
4. [X] `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
5. [X] `docs/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
6. [X] `docs/dev-plans/267-assistant-dialog-rigidity-retrospective-and-architecture-correction-plan.md`
7. [X] `docs/dev-plans/268-assistant-external-llm-semantic-core-and-runtime-thinning-implementation-plan.md`
8. [X] `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
9. [X] `docs/dev-plans/272-assistant-orgunit-seven-actions-expansion-plan.md`
10. [X] `docs/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md`
11. [X] `docs/dev-plans/293-assistant-runtime-proposal-authoritative-gate-minimal-refactor-plan.md`
12. [X] `docs/dev-plans/340-assistant-orgunit-duplicate-maintenance-investigation-and-convergence-plan.md`
13. [X] `docs/dev-plans/350-assistant-tooling-alignment-with-unified-policy-model-plan.md`
14. [X] `internal/server/assistant_action_registry.go`
15. [X] `internal/server/assistant_create_policy_precheck.go`
16. [X] `modules/orgunit/services/orgunit_write_service.go`
