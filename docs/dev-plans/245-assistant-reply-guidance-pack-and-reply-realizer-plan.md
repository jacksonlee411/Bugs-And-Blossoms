# DEV-PLAN-245：Assistant Reply Guidance Pack 与 Reply Realizer 计划

**状态**: 规划中（2026-03-11 CST；承接 `240E`，目标是把澄清/补全/确认/结果回执的用户可见表达统一到知识主链）

## 1. 背景与问题定义
1. [ ] 当前 reply 渲染链已存在，但 fallback 文案和大量用户可见提示仍主要由代码 helper 硬编码拼接。
2. [ ] 缺少统一的 `Reply Guidance Pack` 与 `Reply Realizer` 时，澄清提问、缺字段提示、失败解释会继续在不同 helper 中分叉，难以与理解层和执行层保持一致。

## 2. 目标与非目标
1. [ ] 建立 `Reply Guidance Pack` 资产与 `Reply Realizer` 运行时，实现“结构化骨架 + 受控自然语言表达”。
2. [ ] 覆盖最小 reply 场景：
   - [ ] 澄清提问；
   - [ ] 缺字段提示；
   - [ ] 候选解释；
   - [ ] 确认摘要；
   - [ ] 提交成功；
   - [ ] 提交失败；
   - [ ] 任务等待；
   - [ ] 人工接管。
3. [ ] 保持事实状态仍由 turn/task/DTO 决定，reply 只负责表达与解释。
4. [ ] 不在本计划中重新定义 route 或 clarification 决策；这部分分别由 `242/243` 提供结构化输入。

## 3. 运行时模型（最小冻结）
1. [ ] `Reply Guidance Pack` 至少包含：
   - [ ] `reply_kind`
   - [ ] `knowledge_version`
   - [ ] `locale`
   - [ ] `guidance_templates[]`
   - [ ] `tone_constraints[]`
   - [ ] `negative_examples[]`
   - [ ] `source_refs[]`
2. [ ] `Reply Realizer` 输入至少包括：
   - [ ] stage/kind/outcome；
   - [ ] 结构化机器状态；
   - [ ] 错误码与解释事实；
   - [ ] 候选信息与缺字段信息；
   - [ ] `reply_guidance_version`。
3. [ ] `Reply Realizer` 输出必须：
   - [ ] 不暴露技术信号；
   - [ ] 不伪造业务事实；
   - [ ] 可回放、可审计；
   - [ ] 支持 `zh/en`。

## 4. 与现有实现的迁移策略
1. [ ] 现有 `assistant_reply_nlg.go` 可作为迁移承接点，但其 helper 文案应逐步下沉为资产消费结果。
2. [ ] 迁移顺序建议：
   - [ ] 先替换缺字段提示与失败解释；
   - [ ] 再替换候选解释与确认摘要；
   - [ ] 最后替换成功回执与任务等待。
3. [ ] 过渡期允许保留 fallback，但 fallback 只能作为资产缺失时的短期兜底，并须可审计。

## 5. 分批实施
1. [ ] PR-245-01：定义 `Reply Guidance Pack` schema、样例与版本口径。
2. [ ] PR-245-02：把 `error_catalog_resolver`、缺字段与候选解释接入 reply 输入骨架。
3. [ ] PR-245-03：引入 `Reply Realizer`，优先覆盖失败解释与缺字段提示。
4. [ ] PR-245-04：扩展到澄清提问、候选确认、确认摘要与提交结果。

## 6. 测试与覆盖率
1. [ ] 技术错误码不得直接透传给用户。
2. [ ] reply 文本必须与结构化 machine state 一致，不得生成与事实冲突的表达。
3. [ ] `zh/en` 至少各覆盖一个缺字段与一个失败解释样例。
4. [ ] 资产缺失、版本不匹配、resolver 失败时必须走受控 fallback，而不是返回空文本。

## 7. 停止线（Fail-Closed）
1. [ ] 若 reply 仍主要由页面本地 helper 或散落代码常量拼接，本计划失败。
2. [ ] 若 reply 可改写事实状态、暗中推进 phase 或构造未被 DTO 支持的信息，本计划失败。
3. [ ] 若技术错误信号仍能大面积出现在用户可见文本中，本计划失败。

## 8. 交付物
1. [ ] 本计划文档：`docs/dev-plans/245-assistant-reply-guidance-pack-and-reply-realizer-plan.md`
2. [ ] 执行记录：`docs/dev-records/dev-plan-245-execution-log.md`
3. [ ] 代码交付物：reply 资产、realizer、迁移接线与回归测试。

## 9. 关联文档
- `docs/dev-plans/241-assistant-knowledge-pack-runtime-minimal-implementation-plan.md`
- `docs/dev-plans/243-assistant-clarification-policy-and-slot-repair-plan.md`
- `docs/dev-plans/246-assistant-understand-route-clarify-roadmap.md`
