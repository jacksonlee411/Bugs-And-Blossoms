# DEV-PLAN-436 Readiness

## 目标

- 按 `DEV-PLAN-436` 硬删除历史 `assistant` / `LibreChat` / 旧 `CubeBox` 运行面。
- 不保留 compat window、redirect alias、410 Gone、retired semantics 等过渡语义。
- 将仓库级唯一反回流门禁收敛为 `make check chat-surface-clean`。

## 本轮执行摘要

- 物理删除历史运行时代码、前端页面、路由适配、脚本、部署覆盖层、vendored web 资产与旧测试资产。
- 移除后端历史路由、facade/service 装配、capability route binding、错误映射与前端导航入口。
- 将 `Makefile`、workflow、`AGENTS.md`、`DEV-PLAN-012` 收敛到 `chat-surface-clean`。
- 删除 `config/errors/catalog.yaml` 中 `assistant_*` / `cubebox_*` / `librechat_*` 历史错误码条目。
- 删除 `migrations/iam/atlas.sum` 与 `internal/sqlc/schema.sql` 中历史对话面对象汇总残留。

## 关键证据

- 旧门禁替换：
  - `Makefile`
  - `.github/workflows/quality-gates.yml`
  - `scripts/ci/check-chat-surface-clean.sh`
- 活体入口删除：
  - `internal/server/handler.go`
  - `internal/server/capability_route_registry.go`
  - `apps/web/src/router/index.tsx`
  - `apps/web/src/navigation/config.tsx`
- 错误与契约收口：
  - `config/errors/catalog.yaml`
  - `internal/routing/responder.go`
  - `apps/web/src/errors/presentApiError.ts`
  - `apps/web/src/errors/presentApiError.test.ts`
- 汇总清场：
  - `migrations/iam/atlas.sum`
  - `internal/sqlc/schema.sql`

## 验证计划

- 最小残留扫描：确认仓内不再有旧运行面路径、兼容语义、旧门禁名残留。
- 门禁验证：执行 `make check chat-surface-clean`。
- 代码验证：补跑 `go fmt ./...`、`go vet ./...`、`make check lint`，如环境允许再补 `make css`。

## 当前风险

- 本轮之前已存在大规模删除与跨目录变更，工作树仍然很脏，需基于最终扫描结果继续查漏。
- `schema.sql` 为汇总文件，删除历史块后仍需通过扫描确认无 `assistant` / `cubebox` DDL 残留。
- 尚未完成全量构建/测试验证，需在后续执行阶段补齐。
