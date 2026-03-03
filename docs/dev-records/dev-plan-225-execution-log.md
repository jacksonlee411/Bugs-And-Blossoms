# DEV-PLAN-225 执行日志

## 变更范围

- 后端：新增 Assistant Tasks 三张表迁移、任务仓储/派发/状态机、Tasks API（submit/get/cancel）、路由挂载与路径解析。
- 治理：补齐 routing allowlist、capability route map、server route registry、错误码目录与前端错误映射。
- 前端：新增 tasks API SDK、AssistantPage 任务提交/轮询/取消与关键字段展示、对应单测。

## 执行记录

| 时间（UTC） | 命令 | 结果 |
| --- | --- | --- |
| 2026-03-03 01:12 UTC | `go test ./internal/server -run Assistant -count=1` | 通过 |
| 2026-03-03 01:15 UTC | `pnpm -C apps/web test -- src/api/assistant.test.ts src/pages/assistant/AssistantPage.test.tsx` | 通过（21 files / 84 tests） |
| 2026-03-03 01:15 UTC | `make check routing` | 通过 |
| 2026-03-03 01:15 UTC | `make check capability-route-map` | 通过 |
| 2026-03-03 01:15 UTC | `make authz-pack && make authz-test && make authz-lint` | 通过 |
| 2026-03-03 01:15 UTC | `make check error-message` | 通过 |
| 2026-03-03 01:20 UTC | `make iam plan && make iam lint && make iam migrate up` | 通过 |
| 2026-03-03 01:22 UTC | `make preflight` | 通过（含 `make e2e`，13/13 passed） |
| 2026-03-03 01:39 UTC | `make check doc` | 通过 |

## Stopline 复核

- 未发现同 `(tenant, conversation, turn, request_id)` 产生多条任务的实现路径。
- 已实现 dispatch 超时/重试耗尽后转 `manual_takeover_required` 并写 `dead_lettered` 事件。
- 异步链路未直接触发业务写库提交，保持 One Door 边界。
- 224 快照漂移在执行前强校验，不一致返回 fail-closed（`ai_plan_contract_version_mismatch` / `ai_plan_determinism_violation`）。
