# DEV-PLAN-232 执行日志

> 归档说明（2026-04-12）：本记录已自 `docs/dev-records/` 迁入 `docs/archive/dev-records/`，仅保留为历史执行证据，不再作为活体入口。

## 2026-03-03（UTC）

- 2026-03-03 12:37 UTC：新增运行基线资产目录 `deploy/librechat/`，包含 `docker-compose.upstream.yaml`、`docker-compose.overlay.yaml`、`.env.example`、`versions.lock.yaml`、`healthcheck.sh`、`README.md`。
- 2026-03-03 12:37 UTC：新增脚本 `scripts/librechat/up.sh`、`scripts/librechat/down.sh`、`scripts/librechat/status.sh`、`scripts/librechat/clean.sh`，并通过 `Makefile` 暴露 `assistant-runtime-up/down/status/clean` 统一入口。
- 2026-03-03 12:40 UTC：新增诊断接口 `GET /internal/assistant/runtime-status`（读取 `versions.lock.yaml` + `runtime-status.json` 聚合输出 `healthy|degraded|unavailable`），并接入路由与 capability map。
- 2026-03-03 12:44 UTC：`/assistant-ui` 代理默认上游改为 `LIBRECHAT_UPSTREAM` 缺省回退 `http://127.0.0.1:${LIBRECHAT_PORT:-3080}`，避免缺参直接不可用。
- 2026-03-03 12:48 UTC：前端 `apps/web/src/pages/assistant/AssistantPage.tsx` 接入 runtime status 展示（status/code/upstream/checked_at/services 摘要），补齐 API 与页面单测。
- 2026-03-03 12:52 UTC：执行 `make assistant-runtime-status`（负测），在容器未启动场景返回 `status=unavailable` 并落盘 `runtime-status.json`，服务原因为 `container_not_running`。
- 2026-03-03 12:54 UTC：执行 `go test ./internal/server` 通过。
- 2026-03-03 12:55 UTC：执行 `make check routing capability-route-map assistant-config-single-source doc` 通过。
- 2026-03-03 12:56 UTC：执行 `corepack pnpm -C apps/web exec vitest run src/api/assistant.test.ts src/pages/assistant/AssistantPage.test.tsx` 通过（16/16）。
- 2026-03-03 12:57 UTC：执行 `make e2e`，在 `kratosstub` 监听 `127.0.0.1:4433` 阶段失败（`bind: address already in use`）；随后通过 `ss -ltnp` 确认本机已有 `kratosstub` 占用该端口，判定为环境阻塞。
- 2026-03-03 12:58 UTC：执行 `make generate && make css`，UI 产物重建并同步到 `internal/server/assets/web`。

## 备注

- 本次实施未引入 DB migration / 新建表。
- 新增 `deploy/librechat/.gitignore`，忽略运行时产物 `runtime-status.json` 与本地敏感配置 `.env`。
- CI 冷启动证据将随后续流水线执行结果补充到同文件（本次提交以本地门禁和单测为准）。
