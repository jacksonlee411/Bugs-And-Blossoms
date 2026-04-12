# DEV-PLAN-360A 执行日志：compat session API cutover

**状态**: 已记录并提交（2026-04-13 07:23 CST；Phase 2 的 compat session API 硬切已完成并提交到 `bb5a8568`，cleanup PR 与 runtime fail-closed/error-code 收口待继续）

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

## 5. 提交记录

1. [X] 本轮代码与文档已提交：`bb5a8568`
2. [X] 提交信息：`feat: cut over dev-plan-360a compat session api`

## 6. 结论与后续

1. [X] `DEV-PLAN-360A Phase 2` 的首个 cutover 批次已完成：旧会话 compat API 已从“继续提供会话语义”切到“统一 retired by design”。
2. [X] `375M4` 中“compat session API 硬切”子目标已完成，但 `375M4` 仍未整体封账。
3. [ ] 后续仍需完成 cleanup PR，删除 compat handler 分支与路由绑定。
4. [ ] 后续仍需收口 successor runtime fail-closed/error-code 语义，包括 `assistant_runtime_unavailable / assistant_gate_unavailable`。
5. [ ] `375M3 / DEV-PLAN-370A` 仍可并行推进，但不得改写 `350` 已冻结的 `business_action` contract。
