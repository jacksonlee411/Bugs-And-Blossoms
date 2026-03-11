# DEV-PLAN-243：Assistant 澄清策略与槽位补全回路计划

**状态**: 规划中（2026-03-11 CST；承接 `240E/242`，目标是把“低置信度或缺信息时该如何追问”从零散提示收敛为正式策略）

## 1. 背景与问题定义
1. [ ] 当前系统只有“动作已识别后的缺字段补全”，缺少“动作未完全识别前的澄清回路”。
2. [ ] 低置信度、多意图、自然表达变体、本地规则抽取失败时，当前系统更容易报 unsupported，而不是主动问清楚。
3. [ ] 本计划要把 `240E` 的 `Clarification Policy` 落到运行时，使 Assistant 在必要时追问、在不确定时止损、在澄清成功后平滑回到既有执行主链。

## 2. 目标与非目标
1. [ ] 为 `business_action` 但 `clarification_required=true` 的输入提供受控追问策略。
2. [ ] 把“缺字段补全”“候选确认”“多意图消歧”“日期/组织名歧义确认”统一进同一澄清回路。
3. [ ] 为澄清回路定义轮次上限、失败退出语义与人工提示语义。
4. [ ] 不在本计划中重新定义正式业务 FSM；澄清只是 `route` 层与 `plan` 层之间的受控过渡。
5. [ ] 不在本计划中统一最终自然语言风格；受控表达由 `245` 承接。

## 3. 澄清策略模型（冻结）
1. [ ] 输入：
   - [ ] `Intent Router` 输出；
   - [ ] 结构化缺槽位信息；
   - [ ] 候选对象信息；
   - [ ] 错误码与历史澄清上下文。
2. [ ] 输出：
   - [ ] `clarification_kind`；
   - [ ] `required_slots[]`；
   - [ ] `prompt_template_id`；
   - [ ] `max_rounds`；
   - [ ] `exit_to`（`business_action_resume / uncertain / manual_hint`）。
3. [ ] 最小澄清类型：
   - [ ] 缺字段补全；
   - [ ] 候选确认；
   - [ ] 意图消歧；
   - [ ] 格式确认（例如日期语义）。

## 4. 与当前补槽机制的整合
1. [ ] 现有 `await_missing_fields / await_candidate_pick / await_candidate_confirm` 机制继续保留，但要升级为澄清策略的正式承接点，而不是单纯由硬编码 helper 驱动。
2. [ ] `assistantMergeIntentWithPendingTurn` 应视为“澄清成功后的局部恢复工具”，不是澄清策略本身。
3. [ ] 现有缺字段解释、候选说明与错误解释必须为澄清策略输出结构化输入，而不是独立演化。

## 5. 轮次、退出与失败语义
1. [ ] 必须定义单轮澄清可问的问题数上限，避免一次性抛出过多问题。
2. [ ] 必须定义连续澄清轮次上限，超过上限后只能回到 `uncertain` 或给出人工提示。
3. [ ] 用户多次给出无关输入、冲突输入或仍无法补齐关键槽位时，不得硬猜动作。
4. [ ] 若澄清后仍无法满足最小执行前置条件，不得偷偷退回旧的 `plan_only -> create_orgunit` 升级旁路。

## 6. 运行时接线建议
1. [ ] PR-243-01：定义 `Clarification Policy` DTO、状态字段与审计记录。
2. [ ] PR-243-02：将 `Intent Router` 的 `clarification_required` 输出接入 turn 创建阶段。
3. [ ] PR-243-03：统一缺字段/候选/意图消歧三类澄清入口。
4. [ ] PR-243-04：实现轮次上限、失败退出与人工提示。

## 7. 测试与覆盖率
1. [ ] 覆盖至少包含：
   - [ ] 语义明确但缺字段时会追问而非 unsupported；
   - [ ] 多候选组织会进入候选确认；
   - [ ] 多意图输入会进入消歧而非硬猜；
   - [ ] 澄清轮次耗尽后回到 `uncertain` 或人工提示。
2. [ ] 至少新增一条“建立 + 下面 + 中文日期”这类自然语言回归，证明系统会追问或补槽，而不是直接失败。

## 8. 停止线（Fail-Closed）
1. [ ] 若实现只是继续扩写 `assistantDryRunValidationExplain` 文案，而未建立结构化澄清策略，本计划失败。
2. [ ] 若澄清回路可直接推进 `commit`，绕过正式确认链，本计划失败。
3. [ ] 若超过澄清轮次上限后仍继续硬猜动作，本计划失败。

## 9. 交付物
1. [ ] 本计划文档：`docs/dev-plans/243-assistant-clarification-policy-and-slot-repair-plan.md`
2. [ ] 执行记录：`docs/dev-records/dev-plan-243-execution-log.md`
3. [ ] 代码交付物：澄清 DTO、状态推进、错误码与回归测试。

## 10. 关联文档
- `docs/dev-plans/242-assistant-intent-router-runtime-minimal-plan.md`
- `docs/dev-plans/244-assistant-interpretation-pack-and-intent-route-catalog-compiler-plan.md`
- `docs/dev-plans/245-assistant-reply-guidance-pack-and-reply-realizer-plan.md`
- `docs/dev-plans/246-assistant-understand-route-clarify-roadmap.md`
