# DEV-PLAN-100C 执行日志

**状态**：已实施（2026-02-13）

**关联文档**：
- `docs/dev-plans/100c-org-metadata-wide-table-phase2-kernel-projection-extension-one-door.md`

## 已完成事项

- 2026-02-13：OrgUnit Kernel/Projection 扩展字段（`payload.ext`）写入/回放闭环（保持 One Door）
  - 新增 deep-merge helper：`orgunit.merge_org_event_payload_with_correction(...)`
    - `orgunit.org_events_effective` / `orgunit.org_events_effective_for_replay(...)` 对 `ext/ext_labels_snapshot` 执行 deep-merge，避免 correction 浅合并导致丢键。
    - `ext` 中 `null` 清空语义下，同步移除 `ext_labels_snapshot` 对应 key。
  - 新增统一校验 + 投射 helper：`orgunit.apply_org_event_ext_payload(...)` + allow-matrix `orgunit.is_org_ext_payload_allowed_for_event(...)`
    - fail-closed：仅允许配置于 `orgunit.tenant_field_configs` 的字段写入；并校验 `field_key/physical_col/value_type/enabled_on/disabled_on` 与 label 快照一致性。
    - 稳定错误码：`ORG_EXT_PAYLOAD_INVALID_SHAPE`、`ORG_EXT_PAYLOAD_NOT_ALLOWED_FOR_EVENT`、`ORG_EXT_FIELD_NOT_CONFIGURED`、`ORG_EXT_FIELD_NOT_ENABLED_AS_OF`、`ORG_EXT_FIELD_TYPE_MISMATCH`、`ORG_EXT_LABEL_SNAPSHOT_REQUIRED`、`ORG_EXT_LABEL_SNAPSHOT_NOT_ALLOWED`。
  - 在线写链路接入：`orgunit.submit_org_event(...)` 调用 `apply_org_event_ext_payload(...)`，保证在线写入与 replay 同口径。
  - 回放链路接入：`rebuild_org_unit_versions_for_org_with_pending_event(...)` 事件应用后调用 `apply_org_event_ext_payload(...)`，确保 correction/rescind/replay 不丢扩展字段。
  - 版本切分复制：`split_org_unit_version_at(...)` 及 move 触发 split 的插入分支复制全部 ext 槽位列 + `ext_labels_snapshot`，避免 split 置空扩展值。
  - 迁移落地：新增 `migrations/orgunit/20260213120000_orgunit_ext_payload_kernel_projection.sql`，并更新 `migrations/orgunit/atlas.sum`。
  - 防回归：新增 schema token 测试 `internal/server/orgunit_ext_payload_schema_test.go`（断言 helper/错误码/关键调用点存在）。

## 本地验证（门禁对齐）

- DB（orgunit）：`make orgunit plan && make orgunit lint && make orgunit migrate up`
- sqlc：`make sqlc-generate`
- Go：`go fmt ./... && go vet ./... && make check lint && make test`

## 补充 sanity（手工）

- 在事务内调用：
  - `enable_tenant_field_config(short_name -> ext_str_01, PLAIN)` + `enable_tenant_field_config(org_type -> ext_str_02, DICT)`
  - `submit_org_event(CREATE)` 携带 `payload.ext + ext_labels_snapshot`，验证 `org_unit_versions.ext_str_01/ext_str_02/ext_labels_snapshot` 投射正确
  - `submit_org_event(RENAME)` 携带 `payload.ext.org_type=null`，验证 `ext_str_02` 清空且 `ext_labels_snapshot` key 被移除
