# DEV-PLAN-100B 执行日志

**状态**：已实施（2026-02-13；2026-02-16 文档补录）

**关联文档**：
- `docs/dev-plans/100b-org-metadata-wide-table-phase1-schema-and-metadata-skeleton.md`

## 已完成事项

- Phase 1 数据库骨架已落地（Schema + 迁移）：
  - 新增 `orgunit.tenant_field_configs`（映射唯一约束、`field_key` 形状约束、`PLAIN/DICT/ENTITY` 形状校验、`disabled_on` 规则）。
  - 新增 `orgunit.tenant_field_config_events`（`(tenant_uuid, request_code)` 幂等唯一约束、审计事件载荷约束）。
  - 为两张新表启用并强制 RLS（`ENABLE/FORCE RLS` + `tenant_isolation` policy）。
- 写入口与防线已落地（One Door for metadata writes）：
  - Guard trigger：禁止非 `orgunit_kernel` 角色绕过入口直接 DML。
  - Immutable trigger：启用后禁止修改 `field_key/physical_col/value_type/data_source_type/data_source_config/enabled_on`。
  - Kernel 函数：`orgunit.enable_tenant_field_config(...)` / `orgunit.disable_tenant_field_config(...)`（含幂等冲突 `ORG_REQUEST_ID_CONFLICT`）。
- `orgunit.org_unit_versions` 已增加 MVP 扩展槽位：
  - `ext_str_01..05`、`ext_int_01`、`ext_uuid_01`、`ext_bool_01`、`ext_date_01`、`ext_labels_snapshot`。
  - 预置最小索引（`tenant_uuid + ext_str_*` partial index）。
- 权限收口迁移已落地：
  - `orgunit_kernel` ownership / SECURITY DEFINER / 角色授权与 revoke 对齐。

## 代码与迁移落点（补录核验）

- Schema：
  - `modules/orgunit/infrastructure/persistence/schema/00016_orgunit_field_configs_schema.sql`
  - `modules/orgunit/infrastructure/persistence/schema/00017_orgunit_field_configs_kernel_privileges.sql`
- Goose 迁移：
  - `migrations/orgunit/20260213103451_orgunit_field_configs_engine.sql`
- 防回归测试：
  - `internal/server/orgunit_field_configs_schema_test.go`

## 补录说明

- 本日志为执行证据补录，目的是把已入库实现与 `DEV-PLAN-100B` 的勾选状态对齐，避免“代码已完成但计划文档仍草拟中”的漂移。
- 2026-02-16 文档补录后已执行 `make check doc`，结果：`[doc] OK`。
