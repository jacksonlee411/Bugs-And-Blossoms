# DEV-PLAN-080 执行日志

**状态**: 实施中（截至 2026-02-10）

**关联文档**:
- `docs/dev-plans/080-orgunit-audit-chain-consolidation.md`

## 已完成事项
- 2026-02-10：080C 后续收口：审计快照 presence 表级约束改为严格模式 + 写链路 INSERT 即写齐。
  - 移除过渡放宽（`before/after` 同时空、`rescind_outcome IS NULL`）并保留单一谓词权威：`is_org_event_snapshot_presence_valid(...)`。
  - 新增 `org_events_effective_for_replay(...)` 与 `rebuild_org_unit_versions_for_org_with_pending_event(...)`，以 pending 输入复用单一重建算法。
  - `submit_org_event`（含 allocator）、`submit_org_event_rescind`、`submit_org_rescind`、`submit_org_event_correction`、`submit_org_status_correction` 全部改为预分配 `id` + 单条 INSERT 写齐 `before_snapshot/after_snapshot/rescind_outcome`。
  - 新增迁移：`migrations/orgunit/20260210203000_orgunit_snapshot_insert_complete.sql`，并回填历史 `rescind_outcome`（`after_snapshot IS NULL -> ABSENT`，否则 `PRESENT`）。
  - 补齐防回归测试：`internal/server/orgunit_audit_snapshot_schema_test.go`（禁止后置 UPDATE、presence 严格语义、080C 迁移关键令牌校验）。
- 2026-02-09：Phase1：收敛 OrgUnit 审计快照字段与发起人元数据。
  - `orgunit.org_events` 增加 `initiator_name/initiator_employee_id`。
  - 增加约束：CORRECT/RESCIND 必须包含 `payload.target_event_uuid`。
  - `org_events_tenant_tx_time_idx` 收敛为 `(tenant_uuid, tx_time DESC, id DESC)`。
  - 写链路透传 `InitiatorUUID`（UI -> handler -> service -> kernel）。
- 2026-02-09：Phase2：OrgUnit Details 增加「变更日志」页签（全量审计链可视化）。
  - 左栏：`YYYY-MM-DD hh:mm` + `姓名(工号)`；支持“加载更多”。
  - 右栏：event_type + 摘要 + diff + 原始 JSON 折叠。
  - CORRECT/RESCIND 显示 `target_event_uuid/target_effective_date`，支持跳转目标事件。
  - 时间展示：固定 `UTC+08:00`。
- 2026-02-09：Phase3：补齐测试覆盖与门禁修复。
  - 修复 SQL/迁移中的引号与 schema 引用问题。
  - 补齐 handler/store/render helpers 的审计链测试。
  - 补齐 `orgunit_write_service` 分支覆盖。

## 关键修复记录（失败路径 -> 修复）
- `make test` 覆盖率门禁失败（< 100%）
  - 修复：补齐 `internal/server/orgunit_nodes.go` 审计链相关逻辑与渲染分支测试；补齐 `modules/orgunit/services/orgunit_write_service.go` 的分支覆盖。
- `make orgunit plan` 失败（atlas checksum mismatch）
  - 修复：运行 `./scripts/db/run_atlas.sh migrate hash --dir "file://migrations/orgunit" --dir-format goose` 更新 `migrations/orgunit/atlas.sum`。
- `make orgunit plan` 失败（`iam.principals` 不存在）
  - 修复：对 `iam.principals` 的依赖增加防御：
    - `fill_org_event_audit_snapshot()` 仅在 `to_regclass('iam.principals')` 存在时进行联表查询。
    - privileges/schema grant 使用 `to_regnamespace('iam')/to_regclass('iam.principals')` 保护。

## 本地验证（门禁对齐）
- Go：`go fmt ./... && go vet ./... && make check lint && make test`
- Routing：`make check routing`
- Docs：`make check doc`
- sqlc：`make sqlc-generate`（并确保无额外漂移）
- Atlas/Goose（orgunit）：`make orgunit plan && make orgunit lint && make orgunit migrate up`

## 提交记录
- `8d39962`：DEV-PLAN-080 Phase1: 审计快照字段与发起人元数据收敛
- `da5facf`：DEV-PLAN-080 Phase2: OrgUnit 变更日志页签与审计链可视化
- `93ceb75`：DEV-PLAN-080 Phase3: 审计链测试补齐与门禁修复

## 080C 本地验证证据（2026-02-10）
- `make orgunit plan`：通过（migrations 与 schema 无漂移）。
- `make orgunit lint`：通过（atlas migrate validate）。
- `./scripts/db/migrate.sh orgunit up`：通过（含 `orgunit-smoke`）。
- `make sqlc-generate`：通过（schema 导出 + sqlc 生成）。
- `go test ./internal/server -count=1`：通过（新增 080C 防回归测试）。
- `make e2e`：通过（5/5 Playwright 套件）。
- `make preflight`：通过（含 no-legacy/doc/fmt/lint/test/routing/e2e）。
