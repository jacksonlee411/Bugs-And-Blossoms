# DEV-PLAN-246：Assistant 理解—分流—澄清—表达实施路线图

> 归档说明（2026-04-12）：本文件已自 `docs/dev-plans/` 迁入 `docs/archive/dev-plans/`，仅保留为历史参考，不再作为现行 SSOT。

**状态**: 规划中（2026-03-11 CST；承接 `240E/241`，用于冻结 `241~245` 的实施顺序、依赖关系与里程碑停止线）

## 1. 路线图目的
1. [ ] 把 `241~245` 从“并列计划集合”收敛成有先后顺序的实施路线，避免再次把不同层级的能力混到同一计划中。
2. [ ] 明确哪些是地基类计划、哪些是 runtime route 能力、哪些是用户可见表达收口，确保每一步都能证明增量价值。
3. [ ] 冻结本轮优先级：先解决“不要直接 unsupported”，再解决“如何追问”，最后统一理解资产与回复表达。

## 2. 总体顺序（冻结）
1. [ ] **阶段 A：完成 `241` 地基能力**
   - [ ] 知识资产 schema/编译器最小实现；
   - [ ] `knowledge_snapshot_digest` 等快照字段；
   - [ ] 最小 Resolver；
   - [ ] `plan_context_v1` 接线。
2. [ ] **阶段 B：实施 `242` Intent Router Runtime**
   - [ ] 先把“是否进入业务动作链”独立出来；
   - [ ] 让非动作输入与低置信度输入有正式出口。
3. [ ] **阶段 C：实施 `243` Clarification Policy**
   - [ ] 把 route 待澄清、缺字段补全、候选确认统一为正式回路；
   - [ ] 定义轮次上限与失败退出。
4. [ ] **阶段 D：实施 `244` 理解知识资产治理扩面**
   - [ ] 补全 `Interpretation Pack + Intent Route Catalog` 的资产样例、编译规则与治理证据；
   - [ ] 清退残余散点 prompt/规则依赖。
5. [ ] **阶段 E：实施 `245` Reply Guidance 与表达统一**
   - [ ] 把澄清、缺字段、确认、回执等用户可见表达统一收口到知识主链。

## 3. 依赖关系
1. [ ] `242` 依赖 `241` 提供：知识快照、最小 Resolver、最小 route 资产读取能力。
2. [ ] `243` 依赖 `242` 提供：`route_kind / clarification_required / reason_codes / candidate_action_ids / route_catalog_version / knowledge_snapshot_digest`。
3. [ ] `244` 与 `242/243` 可部分并行，但以不阻塞 `242/243` 最小 runtime 为前提；其主要作用是把理解层知识治理做扎实。
4. [ ] `245` 依赖 `241` 的快照与 Resolver、`243` 的澄清结构化输出，以及 `244` 的理解资产引用规范。

## 4. 每阶段核心验收问题
1. [ ] `241` 完成后：是否已具备受控知识资产、快照与 `plan` 期知识接线，而不再继续新增硬编码知识入口？
2. [ ] `242` 完成后：面对自然表达，系统是否至少能先判定“动作 / 非动作 / 待澄清”，而不是直接 unsupported？
3. [ ] `243` 完成后：当信息不完整或语义不确定时，系统是否会追问、会止损，并且由单一 `Clarification` 主源统一恢复语义，而不是硬猜或报错？
4. [ ] `244` 完成后：理解层知识是否已脱离散乱 prompt/规则，具备可编译、可审计、可版本冻结能力？
5. [ ] `245` 完成后：用户是否能在澄清、补全、确认、成功/失败回执中感受到统一且受控的 Assistant 表达？

## 5. 里程碑与停止线
1. [ ] **M1（241 封板）**：未完成知识快照与最小 Resolver，不得推进 `242` runtime 主接线。
2. [ ] **M2（242 封板）**：`knowledge_qa/chitchat/uncertain` 仍可误入动作链，则不得推进 `243`。
3. [ ] **M3（243 封板）**：澄清轮次、退出语义、失败提示、单一澄清主源与恢复后重跑主链口径未冻结，不得推进大规模 reply 改造。
4. [ ] **M4（244 封板）**：理解知识资产仍大面积散落在代码常量与 prompt 中，不得宣称“理解层完成治理”。
5. [ ] **M5（245 封板）**：用户可见反馈仍主要由本地 helper 拼接，则不得宣称知识主链完成闭环。

## 6. 并行策略
1. [ ] `241` 与 `244` 可做有限并行：`241` 先打最小 schema/快照骨架，`244` 再扩资产治理。
2. [ ] `242` 与 `244` 可在接口冻结后并行推进，但 `242` 不得等待 `244` 全量完成才启动。
3. [ ] `245` 原则上后置，避免在 route/clarification 尚未稳定时过早改写用户可见表达。

## 7. 测试与证据策略
1. [ ] 每阶段至少新增一类“自然语言变体”回归，不得只新增黄金句式样例。
2. [ ] 路线图推进以“后端主链 + 审计快照 + 用户可见反馈”三类证据同时成立为准。
3. [ ] 任一阶段若通过新增 fallback 双链路掩盖问题，应视为偏离路线图。

## 8. 交付物
1. [ ] 本路线图文档：`docs/archive/dev-plans/246-assistant-understand-route-clarify-roadmap.md`
2. [ ] 关联计划：`241/242/243/244/245`
3. [ ] 后续执行记录按各子计划分别沉淀到 `docs/dev-records/`。

## 9. 关联文档
- `docs/archive/dev-plans/240e-assistant-internal-knowledge-pack-and-readonly-resolver-plan.md`
- `docs/archive/dev-plans/241-assistant-knowledge-pack-runtime-minimal-implementation-plan.md`
- `docs/archive/dev-plans/242-assistant-intent-router-runtime-minimal-plan.md`
- `docs/archive/dev-plans/243-assistant-clarification-policy-and-slot-repair-plan.md`
- `docs/archive/dev-plans/244-assistant-interpretation-pack-and-intent-route-catalog-compiler-plan.md`
- `docs/archive/dev-plans/245-assistant-reply-guidance-pack-and-reply-realizer-plan.md`
