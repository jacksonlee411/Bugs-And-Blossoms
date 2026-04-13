# DEV-PLAN-240D 执行日志

> 归档说明（2026-04-12）：本记录已自 `docs/dev-records/` 迁入 `docs/archive/dev-records/`，仅保留为历史执行证据，不再作为活体入口。

- 执行日期：2026-03-09 11:00 CST
- 范围：`PR-240D-03/04` 正式 PG-backed cutover、正式入口 `receipt -> poll -> refresh/cancel` 接入、`manual_takeover_required` 可见性、以及 cutover 后 `288/290/291` 立刻重跑。

## 代码与测试
- `go test ./internal/server`：通过。
- `cd third_party/librechat-web/source/client && npm run test:ci -- --runInBand src/assistant-formal/__tests__/runtime.test.ts src/components/Chat/Messages/Content/__tests__/AssistantFormalMessage.test.tsx`：通过。
- `cd /home/lee/Projects/Bugs-And-Blossoms/apps/web && pnpm test presentApiError.test.ts`：通过。
- `cd /home/lee/Projects/Bugs-And-Blossoms/apps/web && pnpm exec tsc --noEmit`：通过。
- `cd third_party/librechat-web/source/client && npm run typecheck`：失败，原因是 vendored LibreChat workspace 现存依赖缺失（`librechat-data-provider` 等）导致的全量 typecheck 噪音；与本轮变更无直接新增失败。

## 288 / 290 重跑
- `make assistant-runtime-up && make assistant-runtime-status`：通过。
- `pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp288-librechat-real-entry-evidence.spec.js --workers=1`：`tp288-e2e-001/002` 通过。
- `pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp290-librechat-real-case-matrix.spec.js --workers=1`：`tp290-e2e-001~004` 通过。

## 291 重跑
- `make librechat-web-verify`：通过。
- `make librechat-web-build`：通过。
- `go test ./internal/server -run 'TestLibreChatWebUI|TestLibreChatVendoredCompatAPI'`：通过。
- `make check routing`：通过。
- `make check no-legacy`：通过。

## 结论
- 正式 PG-backed `:commit` 已切换到 `202 + receipt`。
- 正式入口已消费 `receipt -> poll -> refresh/cancel`，并对 `manual_takeover_required` 提供可见提示。
- `288/290/291` 已在本轮 cutover 后按 `271-S5` 时序要求重跑通过。
- 无 PG 的同步兼容 seam 已删除；正式入口与 handler 只保留任务受理主链，测试改为显式使用 test helper 覆盖纯业务提交语义。
