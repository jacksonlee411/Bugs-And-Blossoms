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

- 状态：已合并（PR #208）
- PR：https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/208
- 合并提交：`221e9cc`
- 变更摘要：
  - `cmd/dbtool/main.go`（`staffing-smoke`）新增负例断言：
    - one-per-day：同一 `(tenant_id, assignment_id, effective_date)` 用不同 `event_id` 再次提交，要求能定位到 `assignment_events_one_per_day_unique`（或稳定码 `STAFFING_ASSIGNMENT_ONE_PER_DAY`）。
    - 引用校验：active assignment 引用不存在的 `position_id`，必须 `STAFFING_POSITION_NOT_FOUND_AS_OF`。
- 本地验证（按 `AGENTS.md`）：
  - `go fmt ./...`
  - `go vet ./...`
  - `make check lint`
  - `make test`

## M4：Correct / Rescind（Delete-slice + Stitch）

- 状态：已合并（PR #211）
- PR：https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/211
- 合并提交：`a6658bb`
- 变更摘要（对齐 `docs/dev-plans/031-greenfield-assignment-job-data.md` §9.4，保持 append-only，不引入 effseq，不改变同日唯一约束）：
  - DB（Schema SSOT）：
    - `modules/staffing/infrastructure/persistence/schema/00002_staffing_tables.sql`：新增两张 append-only 表：
      - `staffing.assignment_event_corrections`（replacement_payload 替换解释）
      - `staffing.assignment_event_rescinds`（replay 过滤忽略，实现 delete-slice）
    - 两表均启用 RLS（tenant isolation，fail-closed）。
  - DB（Engine SSOT）：
    - `modules/staffing/infrastructure/persistence/schema/00003_staffing_engine.sql`：
      - `replay_assignment_versions(...)` 扩展：`LEFT JOIN` corrections/rescinds、过滤 rescinded、payload=COALESCE(replacement_payload,payload)，并用过滤后序列 `lead(...)` 计算 `next_effective`（stitch）。
      - 新增 Kernel 单一写入口：
        - `staffing.submit_assignment_event_correction(...)`
        - `staffing.submit_assignment_event_rescind(...)`
  - DB（Goose migrations / Atlas）：
    - `migrations/staffing/20260112030000_staffing_assignment_correct_rescind_m4.sql`：创建两张新表。
    - `migrations/staffing/20260112030001_staffing_assignment_correct_rescind_m4_engine.sql`：RLS policy + replay 扩展 + 2 个 submit_* 函数（含 Down，使用 `-- +goose StatementBegin/End` 包裹函数体）。
    - `migrations/staffing/atlas.sum`：已更新（`atlas migrate hash`）。
  - Internal API / Routing / Authz：
    - 新增 internal endpoints：
      - `POST /org/api/assignment-events:correct`
      - `POST /org/api/assignment-events:rescind`
    - `config/routing/allowlist.yaml`：加入 allowlist（route_class=internal_api）。
    - `internal/server/authz_middleware.go`：映射到 `ObjectStaffingAssignments + ActionAdmin`。
  - UI（最小可见入口）：
    - `internal/server/staffing_handlers.go`：`/org/assignments` timeline 每行新增 “Correct/Rescind” 操作入口（表单/按钮），保持只展示 effective_date 的合同不变。
  - 证据入口（dbtool）：
    - `cmd/dbtool/main.go`（`staffing-smoke`）新增 M4 断言：Correct 生效、Rescind stitch、生效限制/错误码等（并同步 payroll 断言期望值）。
  - 测试：
    - `internal/server/assignment_db_integration_test.go`：新增/补齐 Correct/Rescind DB 集成测试与清理顺序（避免 FK 约束失败）。
    - `internal/server/staffing_test.go` / `internal/server/*_test.go`：补齐 handler/store 分支覆盖，`make test` 通过 100% coverage gate。
- 本地验证（按 `AGENTS.md`）：
  - `go fmt ./... && go vet ./...`
  - `make check lint && make check routing && make test`
  - `make staffing plan && make staffing lint && make staffing migrate up`

## 9.5（可选）：Go 分层落位（Assignments Facade/Handler）

- 状态：已合并（PR #213）
- PR：https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/213
- 合并提交：`ba96366`
- 变更摘要（不改 DB 合同与路由，`internal/server` 仅做 wiring）：
  - `modules/staffing/domain/ports/assignment_store.go` / `modules/staffing/domain/types/assignment.go`：定义 Assignments port 与最小类型。
  - `modules/staffing/services/assignments_facade.go`：落位 Assignments Facade（应用层）。
  - `modules/staffing/infrastructure/persistence/assignment_pg_store.go`：落位 PG store（含 deterministic event id + payload canonicalization）。
  - `modules/staffing/presentation/controllers/assignments_api.go`：落位 Internal API handlers（`/org/api/assignments` + `assignment-events:correct|rescind`）。
  - `internal/server/staffing.go`：`Assignment`/`AssignmentStore` 别名对齐 modules，并将 PG store 的 Assignments 方法改为调用 modules/staffing（wiring）。
  - `internal/server/staffing_handlers.go`：Assignements 相关 API handlers 改为委托 modules/staffing controllers（wiring）。
  - `pkg/httperr/httperr.go`：抽出 BadRequest error（供 internal/server 与 modules 共享），保持错误分类口径一致。
  - 测试：补齐 modules/staffing 的分支覆盖，`make test` 通过 100% coverage gate。
- 本地验证（按 `AGENTS.md`）：
  - `go fmt ./... && go vet ./...`
  - `make check lint && make check routing && make test`
