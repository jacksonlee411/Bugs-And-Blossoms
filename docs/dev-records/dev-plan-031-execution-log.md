# DEV-PLAN-031 执行日志（Execution Log）

> 目的：按里程碑记录“已完成事项 + 可复现证据入口”，避免口头对齐与漂移。

## M3-A：Upsert 可重复执行（Idempotency / Re-run）

- 状态：已完成（PR #206，待合并）
- PR：https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/206
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
