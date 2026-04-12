# DEV-PLAN-245 执行日志：Assistant Reply Guidance Pack 与 Reply Realizer

> 归档说明（2026-04-12）：本记录已自 `docs/dev-records/` 迁入 `docs/archive/dev-records/`，仅保留为历史执行证据，不再作为活体入口。

> 对应计划：`docs/archive/dev-plans/245-assistant-reply-guidance-pack-and-reply-realizer-plan.md`

## 1. 执行时间

- 首次实施窗口：2026-03-11 ~ 2026-03-12（CST）
- 本次证据补齐：2026-03-12（CST）
- 最终封板与主线合并：2026-03-12（CST）
- 执行范围：`internal/server` reply realizer 主链、reply guidance 资产、NLG 接线与回归测试

## 2. 本次落地内容（对齐 245 目标）

1. Reply Guidance Pack 资产化落地（`zh/en` 双语）。
- 路径：`internal/server/assistant_knowledge/reply_guidance/*.json`
- 已覆盖首批 `reply_kind`：
  - `clarification_required`
  - `missing_fields`
  - `candidate_list`
  - `candidate_confirm`
  - `confirm_summary`
  - `commit_success`
  - `commit_failed`
  - `task_waiting`
  - `manual_takeover`
  - `non_business_route`

2. Reply 运行时边界与 Realizer 主链落地。
- 文件：`internal/server/assistant_reply_nlg.go`
- 已实现：
  - `assistantBuildReplyRealizerInput(...)`
  - `assistantResolveReplyGuidanceKind(...)`
  - `assistantSelectReplyGuidance(...)`
  - `assistantRealizeReply(...)`
  - 受控模板变量白名单与受控 fallback。
- reply 输出已显式携带 `reply_guidance_version`、`knowledge_snapshot_digest`、`resolver_contract_version` 关联信息。

3. Reply Guidance 编译与选择治理落地。
- 文件：`internal/server/assistant_knowledge_runtime.go`
- 已实现：
  - `findReplyGuidance(replyKind, locale, errorCode)` 选择路径；
  - `reply_guidance_pack` 语义校验（`reply_kind`、`knowledge_version`、`source_refs`、`error_codes`、模板数量与歧义阻断）；
  - locale fallback 与 error_code 优先匹配。

4. Reply pipeline / 模型网关接线对齐。
- `renderTurnReply(...)` 已先经 realizer 产出受控 fallback text，再进入 model gateway。
- 模型返回文本仍会经过用户可见文本净化，防止技术信号直出。

## 3. 验证结果

1. 2026-03-12（CST）执行：
- `go test ./internal/server -run 'TestAssistantCompileKnowledgeRuntime_|TestAssistantKnowledgeRuntime_|TestAssistantIntentRouter|TestAssistantClarification|TestAssistantBuildReplyRealizerInput|TestAssistantRealizeReply|TestAssistantRenderTurnReply'`：通过。
- `go test ./internal/server`：通过。

2. 文档门禁：
- `make check doc`：通过。

3. 封板门禁与合并证据（2026-03-12 CST）：
- `go fmt ./... && go vet ./... && make check lint`：通过。
- `make test`（100% coverage policy）：通过。
- PR：`#483`（`feat(assistant): 完成 DEV-PLAN-245 Reply Guidance Realizer 实施`）已在 GitHub Actions 全绿后合并到 `main`（merge commit: `0e337799e13b1911a541a7207aec6a2384a82189`）。

## 4. 当前结论

- DEV-PLAN-245 的核心主链（Reply Guidance Pack + Reply Realizer + pipeline 接线）已落地并可运行。
- 以“资产扩面 + realizer 接线 + 测试门禁 + CI 跑绿 + 主线合并”为证据，`DEV-PLAN-245` 已满足封板条件并转为已完成状态。
- 当前保留的少量兼容 helper 不再是主入口，后续仅作为演进优化项处理，不阻断本计划完成判定。

## 5. 关联文件

- `docs/archive/dev-plans/245-assistant-reply-guidance-pack-and-reply-realizer-plan.md`
- `internal/server/assistant_reply_nlg.go`
- `internal/server/assistant_knowledge_runtime.go`
- `internal/server/assistant_reply_model_gateway.go`
- `internal/server/assistant_reply_realizer_test.go`
- `internal/server/assistant_reply_nlg_test.go`
- `internal/server/assistant_knowledge/reply_guidance/clarification_required.zh.json`
- `internal/server/assistant_knowledge/reply_guidance/commit_failed.en.json`
