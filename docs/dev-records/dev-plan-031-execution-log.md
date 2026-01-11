# DEV-PLAN-031 执行日志（Execution Log）

> 目的：按里程碑记录“已完成事项 + 可复现证据入口”，避免口头对齐与漂移。

## M3-A：Upsert 可重复执行（Idempotency / Re-run）

- 状态：已合并（PR #206）
- PR：https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/206
- 合并提交：`ca0e057`
- 变更摘要：
  - `internal/server/staffing.go`：`UpsertPrimaryAssignmentForPerson` 支持同一 `effective_date` 的重复提交可重试：
    - 首次提交使用确定性 `event_id`（payload-independent）。
    - 若同一 `effective_date` 已存在事件：复用既有 `(event_id, event_type, request_id, initiator_id)`，走 Kernel 幂等分支；参数不一致则稳定报错 `STAFFING_IDEMPOTENCY_REUSED`（fail-closed）。
  - `internal/server/assignment_db_integration_test.go`：新增 DB 集成测试覆盖：
    - RLS fail-closed（缺失 `app.current_tenant` 必须报错）
    - rerun 不新增事件（同一 assignment + effective_date 仍只有 1 条 event）
    - 同日不同 payload 必须 `STAFFING_IDEMPOTENCY_REUSED`
- 本地验证（按 `AGENTS.md`）：
  - `go fmt ./...`
  - `go vet ./...`
  - `make check lint`
  - `make test`

## M3-B：Terminate / Deactivate（`status=inactive`）

- 状态：已合并（PR #207）
- PR：https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/207
- 合并提交：`54b7f98`
- 变更摘要：
  - `internal/server/staffing_handlers.go`：
    - `POST /org/api/assignments` 支持可选 `status` 字段（`active|inactive`），非法值 400。
    - `/org/assignments` 表单新增 `status` 下拉，并可提交 `status=inactive`。
  - `internal/server/staffing.go`：`UpsertPrimaryAssignmentForPerson` 写入 payload 时可选包含 `status`。
  - `internal/server/staffing_test.go`：补齐 status 分支覆盖（含 UI/Internal API 的 invalid status 负例）。
- 本地验证（按 `AGENTS.md`）：
  - `go fmt ./...`
  - `go vet ./...`
  - `make check lint`
  - `make test`

## M3-C：质量补齐（Assignment 不变量的可复现负例）

- 状态：已完成（待 PR 合并）
- 变更摘要：
  - `cmd/dbtool/main.go`（`staffing-smoke`）新增负例断言：
    - one-per-day：同一 `(tenant_id, assignment_id, effective_date)` 用不同 `event_id` 再次提交，要求能定位到 `assignment_events_one_per_day_unique`（或稳定码 `STAFFING_ASSIGNMENT_ONE_PER_DAY`）。
    - 引用校验：active assignment 引用不存在的 `position_id`，必须 `STAFFING_POSITION_NOT_FOUND_AS_OF`。
- 本地验证（按 `AGENTS.md`）：
  - `go fmt ./...`
  - `go vet ./...`
  - `make check lint`
  - `make test`
