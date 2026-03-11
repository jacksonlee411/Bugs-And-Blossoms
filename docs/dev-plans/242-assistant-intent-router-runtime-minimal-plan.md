# DEV-PLAN-242：Assistant Intent Router 运行时最小落地计划

**状态**: 规划中（2026-03-11 CST；承接 `240E`，目标是把“是否进入业务动作链”从动作抽取副作用提升为正式 `route` 决策）

## 1. 背景与问题定义
1. [ ] 当前 Assistant 在 `resolveIntent` 后基本直接依赖 `intent.action` 进入动作注册表；若 `action` 未注册，则很容易落成 `assistant_intent_unsupported`。
2. [ ] 当前缺少 `240E` 所要求的 `Intent Router`，无法先判断“这是不是业务动作、置信度如何、是否应先澄清”，导致自然语言输入一旦没命中既有抽取口径，就会过早失败。
3. [ ] 本计划要解决的不是“再加一批正则同义词”，而是建立正式的 `route` 决策层，让模型理解、知识目录与运行时裁决之间形成清晰主链。

## 2. 目标与非目标
1. [ ] 在 `plan` 之前新增 `Intent Router` 运行时节点，产出结构化 route 决策：`route_kind / intent_id / candidate_action_ids / confidence_band / clarification_required / reason_codes`。
2. [ ] 让 `business_action` 与 `knowledge_qa/chitchat/uncertain` 成为一等公民，不再都挤进 `action` 解析路径。
3. [ ] 保持执行主链不变：`confirm / commit / task / adapter / DTO` 仍以后端既有事实为准。
4. [ ] 不在本计划中实现完整的澄清追问策略；多轮追问与轮次上限由 `243` 承接。
5. [ ] 不在本计划中重写 `reply` 主链；用户可见表达统一由 `245` 承接。

## 3. 输入输出契约（最小冻结）
1. [ ] 输入至少包括：
   - [ ] 用户原始输入；
   - [ ] `241` 产出的知识版本快照；
   - [ ] `Intent Route Catalog` 与最小 `Interpretation Pack`；
   - [ ] 会话快照的最小上下文（当前 phase、上一轮缺字段、候选状态）。
2. [ ] 输出结构冻结为：
   - [ ] `route_kind`：`business_action / knowledge_qa / chitchat / uncertain`；
   - [ ] `intent_id`；
   - [ ] `candidate_action_ids[]`；
   - [ ] `confidence_band`；
   - [ ] `clarification_required`；
   - [ ] `reason_codes[]`。
3. [ ] `Intent Router` 不得：
   - [ ] 直接推进正式 `phase`；
   - [ ] 直接生成 commit payload；
   - [ ] 覆盖既有 DTO 字段；
   - [ ] 绕过 `ActionSpec` 与 gate 进入执行主链。

## 4. 运行时接线边界
1. [ ] `route_kind=business_action` 且 `clarification_required=false` 时，才允许进入既有动作编排链。
2. [ ] `route_kind=business_action` 但置信度不足时，必须输出“待澄清”的 route 决策，由 `243` 负责追问。
3. [ ] `route_kind=knowledge_qa/chitchat/uncertain` 时，不得进入 `confirm/commit`。
4. [ ] 若路由目录缺失、route 结果非法、知识版本快照缺失，则 fail-closed，不得默默回退到旧的自由猜测路径。

## 5. 与现有实现的兼容策略
1. [ ] 过渡期允许保留现有 `resolveIntent -> assistantIntentSpec` 输出，但其角色下沉为 `business_action` 路由分支中的一个输入事实，而不是唯一裁决者。
2. [ ] 当前本地抽取器 `assistantExtractIntent` 只能作为 route 辅助信号，不得继续直接决定“是否支持某动作”。
3. [ ] 旧的 `unsupported` 结果必须被细分：
   - [ ] route 无法判定；
   - [ ] route 判定为非动作；
   - [ ] route 判定为动作但需要澄清；
   - [ ] route 判定为动作但 action 未注册。

## 6. 分批实施
1. [ ] PR-242-01：定义 `Intent Router` Go 契约、DTO 与审计字段。
2. [ ] PR-242-02：接入 `Intent Route Catalog` 最小运行时读取与 `route_kind` 判定。
3. [ ] PR-242-03：把 `createTurn` / `createTurnPG` 前置到 `route` 决策，并阻断 `knowledge_qa/chitchat/uncertain` 误入动作链。
4. [ ] PR-242-04：补齐 route 失败、route 待澄清、route 非动作三类受控错误码与 API 语义。

## 7. 测试与覆盖率
1. [ ] 覆盖至少包含：
   - [ ] `business_action` 正确进入动作链；
   - [ ] `knowledge_qa/chitchat/uncertain` 不进入动作链；
   - [ ] 低置信度 route 会标记 `clarification_required=true`；
   - [ ] 缺知识目录/非法 route 结果 fail-closed。
2. [ ] 新增回归样例应覆盖自然表达变体，而不是只覆盖“新建/之下/YYYY-MM-DD”黄金句式。

## 8. 停止线（Fail-Closed）
1. [ ] 若 `Intent Router` 仍只是现有 `action` 字段的薄包装，本计划失败。
2. [ ] 若 `knowledge_qa/chitchat/uncertain` 仍可直接进入 `confirm/commit`，本计划失败。
3. [ ] 若 `route` 结果未被审计记录、无法关联 `request / trace / turn`，本计划失败。
4. [ ] 若为兼容旧逻辑而新增第二条“无 route 也能执行”的旁路，本计划失败。

## 9. 交付物
1. [ ] 本计划文档：`docs/dev-plans/242-assistant-intent-router-runtime-minimal-plan.md`
2. [ ] 执行记录：`docs/dev-records/dev-plan-242-execution-log.md`
3. [ ] 代码交付物：`Intent Router` 契约、运行时接线、错误码与回归测试。

## 10. 关联文档
- `docs/dev-plans/240e-assistant-internal-knowledge-pack-and-readonly-resolver-plan.md`
- `docs/dev-plans/241-assistant-knowledge-pack-runtime-minimal-implementation-plan.md`
- `docs/dev-plans/243-assistant-clarification-policy-and-slot-repair-plan.md`
- `docs/dev-plans/246-assistant-understand-route-clarify-roadmap.md`
