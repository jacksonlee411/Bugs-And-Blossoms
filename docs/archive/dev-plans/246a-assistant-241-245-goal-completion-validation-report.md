# DEV-PLAN-246A：基于 240E/246 的 241-245 目标完成验证报告

> 归档说明（2026-04-12）：本文件已自 `docs/dev-plans/` 迁入 `docs/archive/dev-plans/`，仅保留为历史参考，不再作为现行 SSOT。

**状态**: 已完成（2026-03-12 08:36 CST）

## 1. 报告目的
1. [X] 基于 `240E`（知识治理主契约）与 `246`（理解-分流-澄清-表达路线图），对 `241~245` 的目标完成情况做一次统一核验。
2. [X] 输出“实现与目标的符合程度 + 差异项 + 收口建议”，作为 `246` 阶段封板与后续治理输入。

## 2. 验证范围与方法
1. [X] 契约对照：`240E`、`246`、`241~245` 计划文档。
2. [X] 实现对照：`internal/server` Assistant 主链代码、知识资产 JSON、数据库迁移与 schema。
3. [X] 证据对照：`242/243/244` 执行日志与当前仓内可复跑测试结果。
4. [X] 本轮验证命令（2026-03-12 CST）：
   - [X] `go test ./internal/server -run 'TestAssistantCompileKnowledgeRuntime_|TestAssistantKnowledgeRuntime_|TestAssistantIntentRouter|TestAssistantClarification|TestAssistantBuildReplyRealizerInput|TestAssistantRealizeReply|TestAssistantRenderTurnReply'`（通过）
   - [X] `go test ./internal/server`（通过）

## 3. 总体结论
1. [X] `246` 阶段 B/C/D（`242/243/244`）已具备“代码 + 测试 + 执行日志”闭环。
2. [X] `246` 阶段 A/E（`241/245`）在“代码实现层”已明显落地，但“计划状态与执行记录层”未同步封板，存在文档治理差异。
3. [X] 对照 `240E` 主契约，主源矩阵与版本审计主线已基本成立；但部分治理条目仍停留在最小实现或兼容形态（非阻断、需收口）。

## 4. 241-245 分项核验矩阵

| 子计划 | 目标完成判定 | 与目标符合度 | 主要证据 | 差异/缺口 |
| --- | --- | --- | --- | --- |
| 241 | 已完成 | 高 | `assistant_knowledge_runtime.go` 已实现四类资产模型、编译校验、`plan_context_v1`、`knowledge_snapshot_digest` 与版本字段；资产文件已覆盖动作+非动作最小样例；`docs/archive/dev-records/dev-plan-241-execution-log.md` 已补齐 | 无阻断差异 |
| 242 | 已完成 | 高 | 计划状态“已完成”；`assistant_intent_router.go` 与 `route_decision_json` 持久化接线；`docs/archive/dev-records/dev-plan-242-execution-log.md` 已记录实施与测试 | 执行日志明确写明“本轮未跑仓库级 make preflight” |
| 243 | 已完成（功能） | 高 | `assistant_clarification_policy.go` 落地 Clarification SoT、轮次/退出语义、gate 阻断；执行日志与迁移已存在 | 执行日志记录 `make test` 受全仓 100% 覆盖率门禁影响未通过（非功能回归） |
| 244 | 已完成 | 高 | `assistantCompileInterpretationAssets(...)` 与 `assistantCompileIntentRouteCatalog(...)` 已形成编译治理；`route.uncertain` 等资产与专门测试已补齐；执行日志完备 | 执行日志记录 `make test` 与 `make check no-legacy` 受仓内既有项影响未闭环 |
| 245 | 进行中（核心已落地） | 中高 | `assistant_reply_nlg.go` 已有 `assistantBuildReplyRealizerInput(...)`、`assistantRealizeReply(...)`、`findReplyGuidance(...)` 路径；`reply_guidance/*.json` 已覆盖 10 类 `reply_kind` 且 `zh/en` 成对；`docs/archive/dev-records/dev-plan-245-execution-log.md` 已补齐 | 仍需继续收口兼容 helper（如 `assistantReplyStage/assistantReplyKind`）并补齐最终封板门禁证据 |

## 5. 对 246 里程碑的核验
1. [X] M1（241 封板前置能力）：
   - 代码层已满足“知识快照 + 最小 Resolver + `plan_context_v1`”前提，`242` 实际依赖可用。
   - 2026-03-12 已补齐执行日志并回写状态。
2. [X] M2（242）：
   - `knowledge_qa/chitchat/uncertain` 已由 route decision 与 gate 阻断动作链误入。
3. [X] M3（243）：
   - 澄清轮次、退出语义、恢复口径与 fail-closed 阻断已落地。
4. [X] M4（244）：
   - 理解层资产已具备编译、交叉引用校验与漂移阻断能力。
5. [X] M5（245）：
   - 用户可见表达主链已明显向 `Reply Guidance Pack + Realizer` 收敛。
   - 文档状态与执行日志已补齐；兼容分类 helper 仍部分存在。

## 6. 240E 对齐度分析（实现 vs 契约）
1. [X] 已对齐项：
   - 四类知识资产模型与目录结构已形成。
   - `knowledge_snapshot_digest / route_catalog_version / resolver_contract_version / context_template_version / reply_guidance_version` 已进入 plan/turn/task 审计链。
   - 非动作分流与澄清/回复主链均已进入正式运行路径，且 fail-closed 约束明显。
2. [X] 未完全收口项：
   - `241/245` 的文档层缺口（状态回写、执行日志）已于 2026-03-12 补齐。
   - reply 兼容分类 helper 尚未完全退化到最小边角（仍在部分解码路径参与归一）。

## 7. 结论与建议（246A）
1. [X] 结论：`241~245` 的“实现完成度”整体高，`242~244` 可判定为已完成；`241/245` 当前属于“实现先行、文档封板滞后”。
2. [X] 建议收口项 A（文档治理）：
   - 已新增并回填 `dev-plan-241-execution-log.md`、`dev-plan-245-execution-log.md`。
   - 已将 `241/245` 状态回写为与事实一致（`241=已完成`，`245=进行中`）。
3. [ ] 建议收口项 B（245 语义单点化）：
   - 继续收敛 `assistantReplyStage/assistantReplyKind` 在主路径中的语义权重，保持 `ResolvedReplyKind` 单入口原则。
4. [ ] 建议收口项 C（封板证据）：
   - 以 `make preflight` 与相关门禁结果补齐最终封板证据，避免“功能完成但门禁证据不足”。

## 8. 关联文档
- `docs/archive/dev-plans/240e-assistant-internal-knowledge-pack-and-readonly-resolver-plan.md`
- `docs/archive/dev-plans/246-assistant-understand-route-clarify-roadmap.md`
- `docs/archive/dev-plans/241-assistant-knowledge-pack-runtime-minimal-implementation-plan.md`
- `docs/archive/dev-plans/242-assistant-intent-router-runtime-minimal-plan.md`
- `docs/archive/dev-plans/243-assistant-clarification-policy-and-slot-repair-plan.md`
- `docs/archive/dev-plans/244-assistant-interpretation-pack-and-intent-route-catalog-compiler-plan.md`
- `docs/archive/dev-plans/245-assistant-reply-guidance-pack-and-reply-realizer-plan.md`
- `docs/archive/dev-records/dev-plan-242-execution-log.md`
- `docs/archive/dev-records/dev-plan-243-execution-log.md`
- `docs/archive/dev-records/dev-plan-244-execution-log.md`
