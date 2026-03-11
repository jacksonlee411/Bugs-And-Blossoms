# DEV-PLAN-244：Assistant Interpretation Pack 与 Intent Route Catalog 编译治理计划

**状态**: 规划中（2026-03-11 CST；承接 `240E`，目标是把理解层知识从散乱 prompt/规则收敛成可编译、可审计的资产）

## 1. 背景与问题定义
1. [ ] `240E` 已规定 `Interpretation Pack` 与 `Intent Route Catalog` 是理解层主源，但当前运行时仍缺少对应资产、编译器与版本治理。
2. [ ] 若没有正式资产，后续 `242/243` 仍会被迫依赖 prompt 拼装、本地规则和测试样例堆砌，长期仍会退化回“规则机”。

## 2. 目标与非目标
1. [ ] 定义并落地 `Interpretation Pack` 与 `Intent Route Catalog` 的 schema、Go 结构体、语义校验与 digest 规则。
2. [ ] 支持自然语言理解所需的最小知识元素：`intent_classes / clarification_prompts / negative_examples / required_slots / min_confidence / source_refs`。
3. [ ] 为 `242/243` 提供可直接消费的稳定工件，不再让它们直接依赖 dev-plan 原文或散乱 helper。
4. [ ] 不在本计划中直接接入 runtime route 裁决；运行时消费由 `242/243` 承接。
5. [ ] 不在本计划中承担 `Action View Pack` 与 `Reply Guidance Pack` 的完整扩面实现；该部分分别由 `241/245` 承接。

## 3. 资产模型（最小冻结）
1. [ ] `Interpretation Pack` 至少包含：
   - [ ] `pack_id`
   - [ ] `knowledge_version`
   - [ ] `locale`
   - [ ] `intent_classes[]`
   - [ ] `clarification_prompts[]`
   - [ ] `negative_examples[]`
   - [ ] `source_refs[]`
2. [ ] `Intent Route Catalog` 至少包含：
   - [ ] `intent_id`
   - [ ] `route_kind`
   - [ ] `action_id`
   - [ ] `required_slots[]`
   - [ ] `min_confidence`
   - [ ] `clarification_template_id`
   - [ ] `route_catalog_version`
3. [ ] 两类资产均不得定义正式执行真相字段，如 `phase`、`commit 条件`、`required_fields` 真相值。

## 4. 编译与校验范围
1. [ ] schema 校验：字段存在性、类型、枚举、locale、版本字段。
2. [ ] 语义校验：
   - [ ] `intent_id` 唯一；
   - [ ] `route_kind` 合法；
   - [ ] `action_id` 若存在则必须已注册；
   - [ ] `clarification_template_id` 必须存在；
   - [ ] `source_refs[]` 必须有效且至少一个；
   - [ ] `min_confidence` 在允许区间内。
3. [ ] 编译输出：
   - [ ] 可被 runtime 直接查询的索引；
   - [ ] 稳定 digest；
   - [ ] `route_catalog_version` 与 `knowledge_snapshot_digest` 口径。

## 5. 首期样例范围
1. [ ] 至少覆盖一个 `business_action:create_orgunit` 样例。
2. [ ] 至少覆盖一个 `knowledge_qa` 或 `chitchat` 样例。
3. [ ] 至少覆盖一个“需要澄清后才能继续”的自然表达样例。

## 6. 交付切片
1. [ ] PR-244-01：schema/Go 结构体/digest 计算。
2. [ ] PR-244-02：语义编译器与 fail-closed 校验。
3. [ ] PR-244-03：最小资产样例与索引读取接口。
4. [ ] PR-244-04：与 `241/242/243` 的版本口径对齐。

## 7. 测试与覆盖率
1. [ ] 非法 `intent_id / action_id / route_kind / template_id` 必须阻断。
2. [ ] `source_refs[]` 缺失或指向非法仓内位置必须阻断。
3. [ ] digest 与版本口径必须稳定可复算。
4. [ ] 资产变更必须能被 `241` 的知识快照识别。

## 8. 停止线（Fail-Closed）
1. [ ] 若继续把理解知识主要放在代码常量或 prompt 字符串中，而不是资产工件中，本计划失败。
2. [ ] 若资产可声明与执行主链冲突的真相字段，本计划失败。
3. [ ] 若编译器对非法引用选择静默降级而非阻断，本计划失败。

## 9. 交付物
1. [ ] 本计划文档：`docs/dev-plans/244-assistant-interpretation-pack-and-intent-route-catalog-compiler-plan.md`
2. [ ] 执行记录：`docs/dev-records/dev-plan-244-execution-log.md`
3. [ ] 代码交付物：schema、编译器、样例资产、测试。

## 10. 关联文档
- `docs/dev-plans/240e-assistant-internal-knowledge-pack-and-readonly-resolver-plan.md`
- `docs/dev-plans/241-assistant-knowledge-pack-runtime-minimal-implementation-plan.md`
- `docs/dev-plans/242-assistant-intent-router-runtime-minimal-plan.md`
- `docs/dev-plans/243-assistant-clarification-policy-and-slot-repair-plan.md`
- `docs/dev-plans/246-assistant-understand-route-clarify-roadmap.md`
