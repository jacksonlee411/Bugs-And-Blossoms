# DEV-PLAN-080 执行日志

**状态**: 实施中（截至 2026-02-09）

**关联文档**:
- `docs/dev-plans/080-orgunit-audit-chain-consolidation.md`

## 已完成事项
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
