# DEV-PLAN-242 执行日志

**状态**: 已完成（2026-03-11 CST；`assistantIntentRouteDecision` 已作为 `242` 的运行时主链落地，`clarification_required` 边界已按 `243` 评审收紧为 route 级信号，`knowledge_snapshot_digest` 已并入可复核版本集合；本轮已完成 `internal/server` 全量测试，仓库级 `make preflight` 未在本轮执行）

## 1. 执行范围（与 242 对齐）
1. [X] 在 `resolveIntent` 与 `plan/confirm/commit` 之间落地独立 `assistantIntentRouteDecision` 运行时节点。
2. [X] 为 turn 增加 `route_decision_json` 持久化读写，并让 phase / pending draft / confirm / commit 显式消费 route decision。
3. [X] 为 `knowledge_qa / chitchat / uncertain / business_action + clarification_required=true` 建立 fail-closed 行为矩阵与错误码映射。
4. [X] 把 `knowledge_snapshot_digest / route_catalog_version / resolver_contract_version` 固化进 turn 审计链与 task contract snapshot。
5. [X] 按 `243` 评审回灌收紧边界：`route_decision.clarification_required` 仅保留 route 级建议语义，不提前膨胀为 turn 级 `Clarification` 事实。

## 2. 243 评审回灌（本轮新增约束）
1. [X] 冻结语义：`route_decision.clarification_required` 只表示“route 层建议进入澄清”，只服务于动作链 gate、`pending_draft_summary` 抑制与 `idle` 派生；它不是 turn 级 open clarification 主源。
2. [X] 冻结边界：turn 级澄清真相继续留给 `243` 的 `assistantClarificationDecision`，`242` 不新增第二套 clarification state / phase / 事实列。
3. [X] 冻结可复核要求：`knowledge_snapshot_digest` 不再只是运行时附带字段，而是必须可对齐 `route_catalog_version / resolver_contract_version / context_template_version / reply_guidance_version` 的版本集合。

## 3. 实施记录
1. [X] 2026-03-11：确认 `242` 已有主链接线雏形，包括 `assistantIntentRouteDecision` DTO、`route_decision_json` schema/持久化、`assistantEvaluateActionGate(...)` route gate 接入、phase/pending draft 派生。
2. [X] 2026-03-11：将 route builder 接入 create-turn 正式链路，由 `assistantBuildIntentRouteDecisionFn(...)` 产出 route decision，再投影回 `intent.RouteKind/Action` 兼容字段，关闭“只靠 `intent.route_kind` 判定是否可提交”的旧路径。
3. [X] 2026-03-11：将 `assistantTurnRouteClarificationRequired(...)` 收紧并更名为 `assistantTurnHasRouteClarificationSignal(...)`，明确其仅代表 route 级信号，不再暗示 turn 级 clarification SoT。
4. [X] 2026-03-11：新增 `assistantTurnRouteAuditVersionsConsistent(...)`，统一校验 `turn.route_decision` 与 `turn.plan` 的版本集合一致性；create-turn、turn 持久化、task submit、task snapshot replay 均在进入后续主链前执行 fail-closed。
5. [X] 2026-03-11：将 `knowledge_snapshot_digest` 的 hash 输入显式补上 `route_catalog_version / resolver_contract_version / context_template_version / reply_guidance_version`，确保后续 `243/245` 可按 digest 复核本次 route 所消费的版本集合。
6. [X] 2026-03-11：create-turn route 错误映射测试由旧 `knowledgeErr` 注入改为 stub `assistantBuildIntentRouteDecisionFn(...)`，避免把 route builder 错误误测成 knowledge runtime 装载错误。
7. [X] 2026-03-11：补齐/更新 route 信号边界、一致性校验、digest 版本集合的单测与覆盖测试；`internal/server` 全量测试通过。

## 4. 命令与结果
1. [X] `gofmt -w internal/server/assistant_intent_router.go internal/server/assistant_api.go internal/server/assistant_task_store.go internal/server/assistant_persistence.go internal/server/assistant_knowledge_runtime.go internal/server/assistant_intent_router_test.go internal/server/assistant_240d_additional_coverage_test.go internal/server/assistant_knowledge_runtime_more_test.go internal/server/assistant_api_gap_test.go` | 通过 | 收敛本轮 `242` 相关 Go 文件格式。
2. [X] `go test ./internal/server -run 'TestAssistantIntentRouter|TestAssistantKnowledgeRuntime_SnapshotDigestCarriesVersionSet|TestAssistant240DHelperCoverage|TestAssistantTaskSnapshotCompatible_KnowledgeFields'` | 通过 | 验证 route 信号边界、digest 版本集合与 task snapshot 兼容性。
3. [X] `go test ./internal/server -run 'TestAssistantActionInterceptor_Gates|TestAssistantTurnRequiresIntentClarification|TestAssistantTaskValidateSnapshotAgainstTurn|TestAssistantIdempotencyTaskReceiptRestoreCoverage|TestAssistantAPI|TestAssistantPersistence'` | 通过 | 验证 gate、API、持久化与任务快照路径未回归。
4. [X] `go test ./internal/server` | 首轮失败后修复再通过 | 首轮仅 `TestAssistantRouteHandlerMappings/create_turn_route_error_mappings` 因测试注入点仍沿用旧 `knowledgeErr` 失败；补充 route builder hook 后复跑通过。

## 5. 当前结论
- `assistantIntentRouteDecision` 已成为 `242` 范围内“是否允许进入动作链”的运行时主源；`intent.RouteKind` 仅保留兼容投影用途。
- `route_decision.clarification_required` 已被明确降级为 route 级建议信号；它可以阻断 `confirm/commit`，但不再承担 turn 级 clarification 真相。
- `knowledge_snapshot_digest` 已被提升为版本集合的一部分，而非普通附带字段；turn 与 task replay 均会对 route-plan 版本漂移 fail-closed。
- `knowledge_qa / chitchat / uncertain / business_action + clarification_required=true` create-turn 仍允许成功创建 turn，但不会再被 `phase`/`pending_draft_summary`/confirm CTA 误读成“可提交”。
- 现阶段 `242` 与 `243` 的分工已清晰：`242` 负责 route decision 与动作链门禁，`243` 负责真正的多轮 clarification policy 与 turn 级 SoT。

## 6. 关联文件
- `docs/dev-plans/242-assistant-intent-router-runtime-minimal-plan.md`
- `docs/dev-plans/243-assistant-clarification-policy-and-slot-repair-plan.md`
- `internal/server/assistant_intent_router.go`
- `internal/server/assistant_api.go`
- `internal/server/assistant_action_interceptor.go`
- `internal/server/assistant_phase_snapshot.go`
- `internal/server/assistant_task_store.go`
- `internal/server/assistant_persistence.go`
- `internal/server/assistant_knowledge_runtime.go`
- `internal/server/assistant_intent_router_test.go`
- `internal/server/assistant_api_gap_test.go`
- `internal/server/assistant_240d_additional_coverage_test.go`
- `internal/server/assistant_knowledge_runtime_more_test.go`
- `modules/iam/infrastructure/persistence/schema/00009_iam_assistant_conversations.sql`
- `migrations/iam/20260311120000_iam_assistant_route_decision.sql`

## 7. 后续衔接
1. [ ] `243` 上线时，需把 turn 级 clarification SoT 正式收敛到 `assistantClarificationDecision`，并移除任何可能把 route 信号误读为 turn 事实的消费点。
2. [ ] `245` 上线时，需继续确保 reply/NLG 只消费 route / clarification 的正式输出，不回退为从 `plan.title/summary` 猜测运行时状态。
3. [ ] 本日志若后续再命中 `242/243/245` 相关运行时行为改写，应按同一文档追加条目，不另起平行执行日志。
