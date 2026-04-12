# DEV-PLAN-236 执行记录（旧入口退役与单主源封板）

> 归档说明（2026-04-12）：本记录已自 `docs/dev-records/` 迁入 `docs/archive/dev-records/`，仅保留为历史执行证据，不再作为活体入口。

## 1. 记录信息
- 计划：`docs/archive/dev-plans/236-librechat-legacy-endpoint-retirement-and-single-source-closure-plan.md`
- 记录时间：2026-03-03 23:47 UTC
- 记录人：Codex

## 2. 实施结果
- 已完成阶段 C 目标态落地：删除 `POST /internal/assistant/model-providers:apply` 路由、后端接线、capability 映射与前端调用。
- `GET /internal/assistant/model-providers`、`GET /internal/assistant/models`、`POST /internal/assistant/model-providers:validate` 保持只读/校验语义。
- 前端模型页面收敛为“只读展示 + 校验”，不再提供 `Apply` 写入口。

## 3. 代码与契约变更证据
- 后端路由与处理：
  - `internal/server/handler.go`
  - `internal/server/assistant_model_providers_api.go`
- 路由与能力映射：
  - `config/routing/allowlist.yaml`
  - `internal/server/capability_route_registry.go`
  - `config/capability/route-capability-map.v1.json`
- 前端入口收敛：
  - `apps/web/src/api/assistant.ts`
  - `apps/web/src/pages/assistant/AssistantModelProvidersPage.tsx`

## 4. 验证命令与结果
- `go test ./internal/server` ✅
- `pnpm --dir apps/web test src/api/assistant.test.ts` ✅
- `pnpm --dir apps/web typecheck` ✅
- `go vet ./...` ✅
- `make check lint` ✅
- `make test` ✅
- `make check routing` ✅
- `make check capability-route-map` ✅
- `make check no-legacy` ✅
- `make check assistant-config-single-source` ✅
- `make check error-message` ✅
- `make check assistant-domain-allowlist` ✅
- `make check doc` ✅
- `make check tr` ✅（placeholder no-op）
- `make generate && make css` ✅（前端静态资源已更新）

## 5. E2E 记录
- 首次执行 `make e2e` 失败：`assistant_runtime_config_invalid`（`dev-server` 优先读取 `.env.local`，其中 `ASSISTANT_MODEL_CONFIG_JSON` 为非合法 JSON）。
- 修复后执行通过：  
  `E2E_BASE_URL=http://localhost:18080 E2E_SUPERADMIN_BASE_URL=http://localhost:18081 KRATOS_PUBLIC_URL=http://127.0.0.1:14433 E2E_KRATOS_ADMIN_URL=http://127.0.0.1:14434 make e2e` ✅（13 passed）。
- 修复点：
  - `Makefile` 增加 `DEV_SERVER_ENV_FILE/DEV_SUPERADMIN_ENV_FILE` 与端口覆盖变量，避免 E2E 被本地 `.env.local` 污染。
  - `scripts/e2e/run.sh` 显式向 `dev-server/dev-superadmin` 传递 E2E 专用 env file 与端口，并为 kratos stub 绑定对应端口。
- 结论：E2E 问题已定位并修复。

## 6. 结论
- 236 的核心实施（旧入口物理下线 + 单主源封板）已在代码层完成。
- E2E 已在隔离配置下验证通过，236 当前实施范围内门禁闭环完成。
