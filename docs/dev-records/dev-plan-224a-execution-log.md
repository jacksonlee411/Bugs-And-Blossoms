# DEV-PLAN-224A 执行日志

## 2026-03-03（UTC）

- 2026-03-03 04:10 UTC：后端完成真实 OpenAI/Codex adapter 接入与 runtime endpoint 约束（生产仅 `https://`，非生产允许 `builtin://` / `simulate://`）。
- 2026-03-03 04:12 UTC：后端新增会话列表接口 `GET /internal/assistant/conversations`，实现分页、游标签名与稳定排序（`updated_at DESC, conversation_id DESC`）。
- 2026-03-03 04:15 UTC：同步路由治理与能力映射（allowlist / capability-route-map / route registry），并补齐鉴权覆盖测试。
- 2026-03-03 04:18 UTC：新增错误码 `assistant_conversation_cursor_invalid`、`assistant_conversation_list_failed`，并完成后端 known map 与前端提示映射收敛。
- 2026-03-03 04:22 UTC：前端 `/app/assistant` 重构为多轮工作台（会话列表、时间线、右侧操作区、任务状态联动、最近活跃会话恢复）。
- 2026-03-03 04:24 UTC：前端 API SDK 补齐会话列表读取能力；E2E mock 同步支持 `GET /internal/assistant/conversations`。

## 本地验证记录

| 时间（UTC） | 命令 | 结果 |
| --- | --- | --- |
| 2026-03-03 04:43 UTC | `pnpm -C apps/web test -- src/api/assistant.test.ts src/pages/assistant/AssistantPage.test.tsx` | 通过 |
| 2026-03-03 04:44 UTC | `go test ./internal/server -run Assistant -count=1` | 通过 |
| 2026-03-03 04:44 UTC | `make check routing` | 通过 |
| 2026-03-03 04:44 UTC | `make check capability-route-map` | 通过 |
| 2026-03-03 04:45 UTC | `make check error-message` | 通过 |
| 2026-03-03 04:46 UTC | `make authz-pack && make authz-test && make authz-lint` | 通过 |
| 2026-03-03 04:48 UTC | `make preflight` | 未通过（`make test` 阶段命中覆盖率策略：total 99.30% < 100.00%） |
| 2026-03-03 04:55 UTC | `go test ./internal/server -run Assistant -count=1 && make check routing && make check capability-route-map && make check error-message` | 通过 |

## 备注

- 本次未新增数据库表与迁移。
- `make preflight` 失败点已定位为覆盖率门禁，已保留失败证据与可复现命令。
