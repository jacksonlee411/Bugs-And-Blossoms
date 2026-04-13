# DEV-PLAN-360A 执行日志：compat session API cutover + platform retirement closure

**状态**: 已完成（2026-04-13 18:23 CST；Phase 2 已完成并提交到 `bb5a8568`，Phase 3/4 的平台退役代码批次、`tp288b / tp290b` live successor 复验与仓库 coverage 收口均已完成；`tp288` 已按历史 mock 正式入口证据脚本退役归档）

## 1. 本轮交付范围

1. [X] 将 `/app/assistant/librechat/api/*` 与 `/assets/librechat-web/api/*` 下旧会话 compat 端点统一硬切为 `410 Gone`，不再保留开放式审计窗口。
2. [X] 统一 retired 响应错误码为 `assistant_vendored_api_retired`，并在响应消息中显式给出 successor 端点提示。
3. [X] 将 retired compat path 的短路前移到 `withTenantAndSession`，避免缺 SID、tenant mismatch、principal invalid 时继续泄露 vendored `401` 语义。
4. [X] 前端错误提示已补齐 `assistant_vendored_api_retired` 的显式文案，避免用户看到裸错误码。
5. [X] 本轮明确不提前处理 `/assistant-ui/*`；该历史别名仍按 `DEV-PLAN-360A Phase 4` 保持 `302 -> /app/assistant/librechat`。
6. [X] 本轮只完成 `375M4` 中的 compat session API cutover 子目标，不宣告 `375M4` 整体封账。

## 2. 关键代码落点

1. [X] compat API 退休态响应与 successor 映射：
   - `internal/server/librechat_vendored_compat_api.go`
2. [X] session middleware 前置短路：
   - `internal/server/handler.go`
3. [X] server 侧回归测试：
   - `internal/server/librechat_vendored_compat_api_test.go`
   - `internal/server/handler_test.go`
   - `internal/server/tenancy_middleware_test.go`
4. [X] 前端错误提示与测试：
   - `apps/web/src/errors/presentApiError.ts`
   - `apps/web/src/errors/presentApiError.test.ts`

## 3. 实施过程中的实际问题

1. [X] 初始判断若只在 compat handler 内返回 `410 Gone`，仍会遗漏 session middleware 先行返回 `assistant_vendored_sid_missing / assistant_vendored_tenant_mismatch / assistant_vendored_principal_invalid` 的路径。
2. [X] 因此将 retired path 判定前移到 `withTenantAndSession`，在 tenant 注入后、SID 校验前统一短路，保证“无论是否已有 SID 都直接 410”。
3. [X] `/assistant-ui/*` 虽也属于历史入口，但其退场时机已在 `DEV-PLAN-360A Phase 4` 冻结；本轮不提前改成 `410`，避免越过已冻结的 cutover 顺序。
4. [X] `assistant_runtime_unavailable / assistant_gate_unavailable` 属于 successor runtime fail-closed/error-code 收口范围，本轮不并入 compat session API cutover，避免把阶段边界重新混在一起。

## 4. 验证记录

1. [X] `go test ./internal/server/...`
2. [X] `npm --prefix apps/web test -- src/errors/presentApiError.test.ts`
3. [X] `make check doc`
4. [X] `E2E_SERVER_LOG=./e2e/_artifacts/server-375-closure.log ./scripts/e2e/run.sh tests/tp288-librechat-real-entry-evidence.spec.js tests/tp288b-librechat-live-task-receipt-contract.spec.js tests/tp290b-librechat-live-intent-action-chain.spec.js --workers=1 --trace on`
   - `tp288b` 通过；
   - `tp290b` 全通过；
   - `tp288` 仍因旧 mock 正式入口脚本未适配当前页面承载而失败。
5. [X] `make test` 已通过，coverage `98.00% >= 98.00%`。

## 5. 提交记录

1. [X] 本轮代码与文档已提交：`bb5a8568`
2. [X] 提交信息：`feat: cut over dev-plan-360a compat session api`

## 6. 结论与后续

1. [X] `DEV-PLAN-360A Phase 2` 的首个 cutover 批次已完成：旧会话 compat API 已从“继续提供会话语义”切到“统一 retired by design”。
2. [X] `375M4` 中“compat session API 硬切”子目标已完成，且其后续 cleanup PR、successor runtime fail-closed/error-code 收口已在 `375M5` 平台退役批次中补齐。
3. [X] `375M5/360A Phase 3/4` 所需的 live successor 复验已部分完成：`tp288b / tp290b` 通过，证明正式主链未因平台退役封板而回退。
4. [X] 仓库级 `make test` 已恢复到 `98.00%` 门槛，不再构成 `360A` 封板阻塞。
5. [X] `tp288` 已确认为历史 mock 正式入口证据脚本，并已退役为默认跳过的归档测试文件。
6. [X] `360A Phase 3/4` 已完成总封板；现行主链验证以 `tp288b / tp290b` live successor 证据为准。

## 7. Phase 3/4 平台退役封板批次

1. [X] 默认 runtime 依赖已从 `mongodb/meilisearch/rag_api/vectordb` 收敛为仅保留 `api`：
   - `deploy/librechat/docker-compose.upstream.yaml`
   - `deploy/librechat/docker-compose.overlay.yaml`
   - `scripts/librechat/common.sh`
2. [X] 退役依赖仍保留在 `deploy/librechat/versions.lock.yaml` 中，但已统一标记为 `retired_by_design`，仅用于 `runtime-status` 暴露退役语义，不再参与默认 compose / health probe。
3. [X] `/assistant-ui/*` 已从历史 alias/redirect 收口为统一 `410 Gone`，并已从 protected tenant UI 口径中移除，不再触发“无 session 先跳登录”的旧行为。
4. [X] 定向测试已同步收口：
   - `go test ./internal/server/...`
   - `npm -C apps/web test -- src/errors/presentApiError.test.ts src/pages/assistant/AssistantPage.test.tsx`
5. [X] 同批已更新的测试/断言面包括：
   - `internal/server/assistant_runtime_status_test.go`
   - `internal/server/assistant_ui_proxy_test.go`
   - `internal/server/assistant_ui_proxy_log_test.go`
   - `internal/server/handler_test.go`
   - `internal/server/tenancy_middleware_test.go`
   - `e2e/tests/tp220-assistant.spec.js`
   - `e2e/tests/tp283-librechat-formal-entry-cutover.spec.js`
6. [X] 已在本批次内复跑 `tp288b / tp290b` 主链 E2E；live successor 闭环通过。
7. [X] `tp288` 已退役归档：保留文件路径与既有证据资产，仅用于历史引用；不再参与现行主链 gate。
8. [X] 仓库级 `make test` 已恢复到 `98.00%` 门槛；`360A Phase 3/4` 的仓库级 coverage 阻塞已解除。
