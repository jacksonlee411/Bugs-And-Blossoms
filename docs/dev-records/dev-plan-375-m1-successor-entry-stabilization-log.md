# DEV-PLAN-375 M1 执行记录：Successor 执行面稳定

**状态**: 已完成（2026-04-12 19:40 CST）

## 1. 本轮交付范围

1. [X] 正式入口继续固定为 `/app/assistant/librechat`，并将 vendored LibreChat Web UI 收口为聊天 UI 壳。
2. [X] 正式入口只消费 `/internal/assistant/ui-bootstrap` 与 `/internal/assistant/session*` successor 合同，不再消费旧 `/config`、`/endpoints`、`/models`、`/user`、`/roles/*` 与 `/auth/refresh`。
3. [X] vendored Web UI 中的 Agents / MCP / Search / Memory / Code Interpreter 相关导航、路由与可见入口已降权或移除。
4. [X] `AssistantRuntimeStatusResponse` 与运行态页已补齐 `agents_ui_enabled / memory_enabled / web_search_enabled / file_search_enabled / code_interpreter_enabled / artifacts_enabled / runtime_cutover_mode`，并将 `retired_by_design` 解释为设计性退役而非异常态。
5. [X] `tp283` smoke 已扩展 successor DTO、UI 降权、静态前缀会话边界、`/assistant-ui/*` 仍为 `302` alias、旧 bootstrap compat 端点删除态断言。

## 2. 关键代码落点

1. [X] runtime / successor API：
   - `internal/server/assistant_formal_entry_api.go`
   - `internal/server/assistant_runtime_status.go`
   - `internal/server/assistant_domain_policy.go`
2. [X] apps/web successor 消费与运行态展示：
   - `apps/web/src/api/assistant.ts`
   - `apps/web/src/pages/assistant/AssistantPage.tsx`
3. [X] vendored LibreChat Web UI 降权与 successor 接线：
   - `third_party/librechat-web/source/client/src/routes/Root.tsx`
   - `third_party/librechat-web/source/client/src/routes/index.tsx`
   - `third_party/librechat-web/source/client/src/hooks/Nav/useSideNavLinks.ts`
   - `third_party/librechat-web/source/packages/data-provider/src/data-service.ts`
   - `third_party/librechat-web/source/packages/data-provider/src/api-endpoints.ts`
   - `third_party/librechat-web/source/packages/data-provider/src/request.ts`
4. [X] smoke / contract tests：
   - `e2e/tests/tp283-librechat-formal-entry-cutover.spec.js`
   - `third_party/librechat-web/source/packages/data-provider/specs/assistant-formal-cutover.spec.ts`
   - `internal/server/assistant_formal_entry_api_test.go`
   - `internal/server/assistant_runtime_status_test.go`

## 3. 实施过程中的实际问题

1. [X] 2026-04-12 在 fresh 隔离 E2E 环境中首次发现正式入口前端运行时错误：`cn is not defined`。
2. [X] 根因定位到 `third_party/librechat-web/source/client/src/components/Chat/Input/ToolsDropdown.tsx` 在本轮降权重构后仍使用 `cn(...)`，但 import 被误删。
3. [X] 修复方式：补回 `cn` import，随后执行 `make librechat-web-build` 重建 `internal/server/assets/librechat-web/*` 静态产物，并重新跑隔离 `tp283` 确认恢复。

## 4. 验证记录

1. [X] `go test ./internal/server/...`
2. [X] `pnpm --dir apps/web test -- --run AssistantPage.test.tsx LibreChatPage.test.tsx api/assistant.test.ts`
3. [X] `cd third_party/librechat-web/source/packages/data-provider && npm run test:ci -- specs/assistant-formal-cutover.spec.ts --runInBand`
4. [X] `E2E_BASE_URL=http://localhost:18080 E2E_SUPERADMIN_BASE_URL=http://localhost:18081 KRATOS_PUBLIC_URL=http://127.0.0.1:14433 E2E_KRATOS_ADMIN_URL=http://127.0.0.1:14434 scripts/e2e/run.sh tests/tp283-librechat-formal-entry-cutover.spec.js --workers=1`

## 5. 结论与后续

1. [X] `375M1` 对应的 `360 Phase 0/1` 与 `360A Phase 0/1` 已具备封账证据。
2. [ ] 下一步按 `DEV-PLAN-375` 进入并行泳道：
   - `375M2 / DEV-PLAN-350A`
   - `375M3 / DEV-PLAN-370A`
