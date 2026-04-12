# DEV-PLAN-272 执行日志

> 归档说明（2026-04-12）：本记录已自 `docs/dev-records/` 迁入 `docs/archive/dev-records/`，仅保留为历史执行证据，不再作为活体入口。

## 1. 执行记录
| 时间（CST） | 执行人 | 命令 | 结果 | 备注 |
| --- | --- | --- | --- | --- |
| 2026-03-11 08:12-08:21 | Codex | `make assistant-runtime-down && DEV_INFRA_ENV_FILE=.env make dev-down && DEV_INFRA_ENV_FILE=.env make dev-up && DATABASE_URL=... make iam migrate up && DATABASE_URL=... make orgunit migrate up && make dev-kratos-stub && seed_kratosstub_identity.sh * 3 && TRUST_PROXY=1 DEV_SERVER_ENV_FILE=.env make dev-server && make dev-superadmin && make assistant-runtime-up && go test ./internal/server -run 'TestAssistantActionRegistryAndVersionTupleHelpers|TestAssistantStrictDecodeIntent|TestAssistantStrictDecodeIntentExpandedFields|TestAssistantIntentPipelineExpandedActions|TestAssistantModelGatewayResolveIntentFallbackAndValidation|TestAssistantActionInterceptor_Gates|TestAssistantTurnActionHandler_CoverageMatrix|TestAssistantPersistence_SubmitCommitTaskPG_GateRejectNoTaskWrites|TestAssistant272TurnAPI_CreateAndConfirmMatrix|TestAssistant272PrepareCommitTurn_ActionMatrix|TestAssistant272SubmitCommitTaskWorkflowAndPoll_ActionMatrix|TestAssistantIntentPipeline_UnsupportedActionUpgradeFromLocalFacts' -count=1 && TRUST_PROXY=1 pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp288-librechat-real-entry-evidence.spec.js --workers=1 --trace on && TRUST_PROXY=1 pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp290b-librechat-live-intent-action-chain.spec.js --workers=1 --trace on` | 通过 | 按 `bugs-and-blossoms-dev-login` 技能全量重启本地依赖、`kratosstub`、`server:8080`、`superadmin:8081` 与 LibreChat runtime；修正本地 `TRUST_PROXY=1` 缺失后，`/app/assistant`、`/app/assistant/librechat`、`tp288-e2e-001/002`、`tp290b-e2e-000~004` 全部再次通过，确认 272 在重启后的真实运行态无回退 |
| 2026-03-11 07:17-07:20 | Codex | `make preflight` | 通过 | 全仓预检完成：`fmt/vet/lint/test/routing/doc` 与默认 `31` 条 Playwright E2E 全部通过；`tp288/tp288b/tp290/tp290b` 证据资产已自动刷新到本轮最新时间戳 |
| 2026-03-11 07:11-07:13 | Codex | `TRUST_PROXY=1 pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp288-librechat-real-entry-evidence.spec.js --workers=1 --trace on` + `TRUST_PROXY=1 pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp290b-librechat-live-intent-action-chain.spec.js --workers=1 --trace on` | 通过 | 按 `271-S5` 新鲜度规则完成 `288/290B` 统一重跑：`tp288-e2e-001/002` 全通过；`tp290b-e2e-000~004` 全通过，`runtime_admission_gate`、Case2~4 与 baseline 全部转为 `passed` |
| 2026-03-11 07:04-07:05 | Codex | `make iam migrate up` + `go test ./internal/server -run 'TestAssistantActionInterceptor_Gates|TestAssistantTurnActionHandler_CoverageMatrix|TestAssistantPersistence_SubmitCommitTaskPG_GateRejectNoTaskWrites|TestAssistant272TurnAPI_CreateAndConfirmMatrix|TestAssistant272PrepareCommitTurn_ActionMatrix|TestAssistant272SubmitCommitTaskWorkflowAndPoll_ActionMatrix|TestAssistantIntentPipeline_UnsupportedActionUpgradeFromLocalFacts' -count=1` | 通过 | 修复 live 阻断根因：恢复 `assistant` phase snapshot schema（`current_phase`、turn phase 快照、`from_phase/to_phase`）并补“未知 action 由本地显式事实升级为 `create_orgunit`”回归；`assistant_conversation_create_failed` 与 `assistant_intent_unsupported` 两类 live 阻断均已关闭 |
| 2026-03-10 12:10-12:32 | Codex | `go test ./internal/server -run 'TestAssistantActionInterceptor_Gates|TestAssistantTurnActionHandler_CoverageMatrix|TestAssistantPersistence_SubmitCommitTaskPG_GateRejectNoTaskWrites|TestAssistant272TurnAPI_CreateAndConfirmMatrix|TestAssistant272PrepareCommitTurn_ActionMatrix|TestAssistant272SubmitCommitTaskWorkflowAndPoll_ActionMatrix' -count=1` + `go test ./internal/server -count=1` | 通过 | 完成 `PR-272-04/05` 后端主证据收口：七动作 gate 矩阵、HTTP/error/reason 映射、PG 拒绝路径、API 成功矩阵，以及 `:commit -> receipt -> task poll -> conversation refresh` 终态一致性全部通过 |
| 2026-03-10 03:42-04:03 | Codex | `make check doc` + `go fmt ./...` + `go vet ./...` + `make check lint` + `make test` | 通过 | 全仓门禁恢复全绿：文档校验、`go vet`、cleanarch、全量测试与 `100.00%` 覆盖率全部通过；为消除覆盖率回退，补充了 `assistant_272_coverage_test.go` 等精确分支测试 |
| 2026-03-10 03:30-03:41 | Codex | `gofmt -w internal/server/assistant_api.go internal/server/assistant_intent_pipeline.go internal/server/assistant_action_interceptor.go internal/server/assistant_action_registry.go internal/server/assistant_action_registry_test.go internal/server/assistant_intent_pipeline_test.go internal/server/assistant_model_gateway.go internal/server/assistant_model_gateway_more_test.go internal/server/assistant_phase_snapshot.go` + `go test ./internal/server -run 'TestAssistantActionRegistryAndVersionTupleHelpers|TestAssistantStrictDecodeIntent|TestAssistantStrictDecodeIntentExpandedFields|TestAssistantIntentPipelineExpandedActions|TestAssistantModelGatewayResolveIntentFallbackAndValidation' -count=1` | 通过 | 完成 `PR-272-01/02/03` 最小闭环：七动作默认注册、七动作 commit adapter 接线、intent DTO/strict decode/normalize/compile/validation 扩容，以及首批单测回归 |

## 2. 当前结论
- `assistantActionRegistry` 已不再是 `create_orgunit` 单动作骨架，七动作均已显式注册 `CommitAdapterKey`。
- `assistantCommitAdapterRegistry` 已接入 `add/insert/correct/disable/enable/move/rename` 七类适配器，仍保持 fail-closed。
- `assistantIntentSpec`、strict decode、OpenAI payload normalize、compile/dry-run/validation 已扩容到七动作最小载荷。
- `prepareCommitTurn(...)` 已修复“非候选动作也被强制要求 `ResolvedCandidateID`”的问题；只有真正需要候选确认的动作才在 commit 前强制校验 candidate，避免 `rename_orgunit`、`disable_orgunit`、`enable_orgunit` 以及不改父组织的版本类动作被误拦截。
- `iam` phase snapshot schema 已恢复并加固：`assistant_conversations.current_phase`、`assistant_turns.phase/pending_draft_summary/missing_fields/candidate_options/selected_candidate_id/commit_reply/error_code`、`assistant_state_transitions.from_phase/to_phase` 与约束重新回到运行态，关闭了 cleanup migration 与代码读写口径漂移。
- 对真实模型返回的未知 action，当前已允许在“用户原文可被本地规则稳定判定为 `create_orgunit`”时做动作级升级；保留 real provider/model 元信息，同时避免无谓落入 `assistant_intent_unsupported`。
- `PR-272-04` 已完成：`required_checks` 动作级最小化、interceptor 主源收敛，以及 confirm/commit 阶段 `reason_code` / `turn.error_code` 对齐均已有测试矩阵覆盖。
- `PR-272-05` 已完成：七动作 API/PG 成功与关键拒绝样例、`:commit -> receipt -> task poll -> conversation refresh` 终态一致性，以及 `288/290B` live 新鲜证据均已补齐；`make preflight`、重启后本地 live 复核、PR 跑绿与合并同步均已完成。
- `PR #481` 已于 2026-03-11 合并；本地 `wt-dev-main` 已同步到 merge commit，272 当前无剩余开发任务。

## 3. 下一步实施策略（2026-03-10 评估回写）
- 顺序冻结：先完成 `PR-272-04` 的服务端确定性回归，再推进 `PR-272-05` 的后端主证据，最后统一刷新 `288 + 290B` live 证据。
- `PR-272-04` 的首要任务是补齐三类测试：`internal/server/assistant_action_interceptor_test.go`、`internal/server/assistant_api_coverage_test.go`、`internal/server/assistant_persistence_gap_test.go`。
- `PR-272-05` 的首要任务是按七动作补齐 API/PG 成功与拒绝样例，并补 `:commit -> receipt -> task poll -> conversation refresh` 的终态一致性断言。
- 证据策略冻结：由于 `272` 继续触达运行时 gate / 错误码语义 / fail-closed 主链，`288/290B` 历史证据在本轮实现稳定前仅可视为阶段参考，不得直接用于 `271-S5/285` 封板判定。
- 当前完成度判断更新为：七动作正式链路、后端主证据、`288/290B` live 新鲜证据、`make preflight` 与重启后 live 复核均已收口，且已完成提交流程与主线同步。

## 4. 关联文件
- `docs/archive/dev-plans/272-assistant-orgunit-seven-actions-expansion-plan.md`
- `internal/server/assistant_action_registry.go`
- `internal/server/assistant_action_interceptor.go`
- `internal/server/assistant_intent_pipeline.go`
- `internal/server/assistant_api.go`
- `internal/server/assistant_model_gateway.go`
- `internal/server/assistant_phase_snapshot.go`
- `internal/server/assistant_persistence.go`
- `internal/server/assistant_action_registry_test.go`
- `internal/server/assistant_intent_pipeline_test.go`
- `internal/server/assistant_model_gateway_more_test.go`
- `internal/server/assistant_action_interceptor_test.go`
- `internal/server/assistant_api_coverage_test.go`
- `internal/server/assistant_persistence_gap_test.go`
- `internal/server/assistant_272_api_matrix_test.go`
- `internal/server/assistant_272_task_lifecycle_test.go`
